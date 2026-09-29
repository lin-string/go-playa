package document

import (
	"fmt"
	"iter"
	"sort"
	"strconv"

	"github.com/lin-string/go-playa/documentdata"
)

const pageLabelCacheLimit = 32 << 20
const pageLabelsValuesLimit = 32 << 20

// PageLabelSpec is the dependency-free page-label value model. Document owns
// only the number-tree traversal and bounded caches around it.
type PageLabelSpec = documentdata.PageLabelSpec

type pageLabelItem struct {
	page int
	spec PageLabelSpec
}

func clonePageLabelValues(value []string) []string {
	if value == nil {
		return nil
	}
	return append(make([]string, 0, len(value)), value...)
}

func pageLabelItemsCacheSize(items []pageLabelItem) int {
	size := 64
	for _, item := range items {
		itemSize := addCacheSize(64, len(item.spec.Style()))
		itemSize = addCacheSize(itemSize, len(item.spec.Prefix()))
		size = addCacheSize(size, itemSize)
	}
	return size
}

// Label returns the human-facing label for this page. Number is zero-based,
// matching the Page slice position used by the Go API.
func (p Page) Label(d *Document) string {
	label, _ := p.LabelWithError(d)
	return label
}

// LabelWithError returns the human-facing label for this page and reports a
// malformed page-label tree.
func (p Page) LabelWithError(d *Document) (string, error) {
	if d == nil {
		return "", errNilDocument
	}
	if p.number <= 0 {
		return "", nil
	}
	items, err := d.pageLabelItemsWithError()
	if err != nil {
		return "", err
	}
	// Label ranges use zero-based page indexes. With no explicit range, the
	// default range starts at page zero and formats the one-based page number.
	current := pageLabelItem{page: 0, spec: documentdata.NewPageLabelSpec("D", "", 1)}
	for _, item := range items {
		if item.page > p.number-1 {
			break
		}
		current = item
	}
	return current.spec.Format(p.number - 1 - current.page), nil
}

// PageLabel returns the label for a zero-based page index.
func (d *Document) PageLabel(index int) string {
	label, _ := d.PageLabelWithError(index)
	return label
}

// PageLabelWithError returns the label for a zero-based page index and
// reports a malformed page-label tree.
func (d *Document) PageLabelWithError(index int) (string, error) {
	if index < 0 {
		return "", nil
	}
	current := 0
	label := ""
	for value, err := range d.PageLabelsSeq() {
		if err != nil {
			return "", err
		}
		if current == index {
			label = value
			break
		}
		current++
	}
	return label, nil
}

func (d *Document) PageLabels() ([]string, error) {
	d.pageLabelsValuesMu.Lock()
	if d.pageLabelsValuesReady {
		values := clonePageLabelValues(d.pageLabelsValues)
		err := d.pageLabelsValuesErr
		d.pageLabelsValuesMu.Unlock()
		return values, err
	}
	d.pageLabelsValuesMu.Unlock()
	items := d.pageLabelItems()
	d.cacheMu.RLock()
	itemsErr := d.pageLabelsErr
	d.cacheMu.RUnlock()
	if itemsErr != nil {
		d.pageLabelsValuesMu.Lock()
		d.pageLabelsValuesReady = true
		d.pageLabelsValuesErr = itemsErr
		d.pageLabelsValuesMu.Unlock()
		return nil, itemsErr
	}
	itemIndex := 0
	values := make([]string, 0)
	valuesBytes := 0
	for index, err := range d.Pages() {
		if err != nil {
			d.pageLabelsValuesMu.Lock()
			d.pageLabelsValuesReady = true
			d.pageLabelsValuesErr = err
			d.pageLabelsValuesMu.Unlock()
			return nil, err
		}
		for itemIndex+1 < len(items) && items[itemIndex+1].page <= index.number-1 {
			itemIndex++
		}
		value := strconv.Itoa(index.number)
		if len(items) > 0 && items[itemIndex].page <= index.number-1 {
			spec := items[itemIndex].spec
			value = spec.Format(index.number - 1 - items[itemIndex].page)
		}
		values = append(values, value)
		valuesBytes = addCacheSize(valuesBytes, addCacheSize(len(value), 16))
	}
	d.pageLabelsValuesMu.Lock()
	defer d.pageLabelsValuesMu.Unlock()
	if d.pageLabelsValuesReady {
		return clonePageLabelValues(d.pageLabelsValues), d.pageLabelsValuesErr
	}
	if valuesBytes <= d.cacheLimits().PageLabelsValuesBytes {
		d.pageLabelsValues = values
		d.pageLabelsValuesBytes = valuesBytes
		d.pageLabelsValuesReady = true
	}
	return clonePageLabelValues(values), nil
}

// PageLabelsSeq lazily generates one human-facing label per page.
func (d *Document) PageLabelsSeq() iter.Seq2[string, error] {
	return func(yield func(string, error) bool) {
		var items []pageLabelItem
		var nextItem func() (pageLabelItem, error, bool)
		var stop func()
		d.cacheMu.RLock()
		ready := d.pageLabelsReady
		cachedItems := d.pageLabelCache
		cachedErr := d.pageLabelsErr
		d.cacheMu.RUnlock()
		if ready {
			if cachedErr != nil {
				yield("", cachedErr)
				return
			}
			items = cachedItems
		} else {
			nextItem, stop = iter.Pull2(d.pageLabelItemsSeq())
			defer stop()
		}
		itemIndex := 0
		firstRuleSeen := false
		var pending *pageLabelItem
		var advanceErr error
		current := pageLabelItem{page: -1, spec: documentdata.NewPageLabelSpec("D", "", 1)}
		advance := func(page int) {
			for {
				var item pageLabelItem
				var ok bool
				if pending != nil {
					item, pending = *pending, nil
					ok = true
				} else if ready {
					if itemIndex >= len(items) {
						return
					}
					item = items[itemIndex]
					itemIndex++
					ok = true
				} else {
					var err error
					item, err, ok = nextItem()
					if err != nil {
						advanceErr = err
						return
					}
					if !ok {
						return
					}
				}
				if !firstRuleSeen {
					firstRuleSeen = true
					if item.page > 0 {
						// Playa tolerates a missing zero rule by applying the
						// first listed rule from page zero onward.
						item.page = 0
					}
				}
				if item.page > page {
					pending = &item
					return
				}
				current = item
			}
		}
		for index, err := range d.Pages() {
			if err != nil {
				yield("", err)
				return
			}
			advance(index.number - 1)
			if advanceErr != nil {
				yield("", advanceErr)
				return
			}
			value := strconv.Itoa(index.number)
			if current.page >= 0 {
				spec := current.spec
				value = spec.Format(index.number - 1 - current.page)
			}
			if !yield(value, nil) {
				return
			}
		}
	}
}

func (d *Document) pageLabelItemsSeq() iter.Seq2[pageLabelItem, error] {
	return func(yield func(pageLabelItem, error) bool) {
		d.cacheMu.RLock()
		if d.pageLabelsReady {
			items := d.pageLabelCache
			err := d.pageLabelsErr
			d.cacheMu.RUnlock()
			if err != nil {
				yield(pageLabelItem{}, err)
				return
			}
			for _, item := range items {
				if !yield(item, nil) {
					return
				}
			}
			return
		}
		d.cacheMu.RUnlock()
		cacheRootError := func(err error) error {
			d.cacheMu.Lock()
			defer d.cacheMu.Unlock()
			if d.pageLabelsReady {
				return d.pageLabelsErr
			}
			d.pageLabelCache = nil
			d.pageLabelCacheBytes = 0
			d.pageLabelsErr = err
			d.pageLabelsReady = true
			return err
		}
		root, ok := d.trailer[Name("Root")]
		if !ok {
			return
		}
		catalogValue, ok := d.resolveIndirectChain(root)
		if !ok {
			yield(pageLabelItem{}, cacheRootError(fmt.Errorf("playa: catalog root is not a dictionary")))
			return
		}
		cat, ok := catalogValue.(Dict)
		if !ok {
			yield(pageLabelItem{}, cacheRootError(fmt.Errorf("playa: catalog root is not a dictionary")))
			return
		}
		labelTree, present := cat[Name("PageLabels")]
		if !present {
			return
		}
		treeValue, resolved := d.resolveIndirectChain(labelTree)
		tree, ok := treeValue.(Dict)
		if !resolved {
			ok = false
		}
		if !ok {
			yield(pageLabelItem{}, cacheRootError(fmt.Errorf("playa: PageLabels root is not a dictionary")))
			return
		}
		d.walkPageLabelTreeSeq(tree, map[Ref]bool{}, nil, yield)
	}
}

func (d *Document) walkPageLabelTreeSeq(tree Dict, seen map[Ref]bool, parentBounds *[2]int, yield func(pageLabelItem, error) bool) bool {
	_, hasNums := tree[Name("Nums")]
	_, hasKids := tree[Name("Kids")]
	if hasNums && hasKids {
		return yield(pageLabelItem{}, fmt.Errorf("playa: PageLabels node cannot contain both Nums and Kids"))
	}
	bounds, err := d.parsePageLabelLimits(tree)
	if err != nil {
		return yield(pageLabelItem{}, err)
	}
	if bounds != nil && parentBounds != nil && (bounds[0] < parentBounds[0] || bounds[1] > parentBounds[1]) {
		return yield(pageLabelItem{}, fmt.Errorf("playa: PageLabels child Limits are outside parent"))
	}
	if raw, present := tree[Name("Nums")]; present {
		resolvedNums, resolved := d.resolveIndirectChain(raw)
		if !resolved {
			return yield(pageLabelItem{}, fmt.Errorf("playa: PageLabels Nums could not be resolved"))
		}
		nums, ok := resolvedNums.(Array)
		if !ok {
			return yield(pageLabelItem{}, fmt.Errorf("playa: PageLabels Nums is not an array"))
		}
		if len(nums)%2 != 0 {
			return yield(pageLabelItem{}, fmt.Errorf("playa: PageLabels Nums has an unmatched key"))
		}
		var previousPage = -1
		for i := 0; i+1 < len(nums); i += 2 {
			pageValue, resolved := d.resolveIndirectChain(nums[i])
			if !resolved {
				return yield(pageLabelItem{}, fmt.Errorf("playa: PageLabels key %d could not be resolved", i/2))
			}
			page, ok := IntValue(pageValue)
			if !ok {
				return yield(pageLabelItem{}, fmt.Errorf("playa: PageLabels key %d is not an integer", i/2))
			}
			if page < 0 {
				return yield(pageLabelItem{}, fmt.Errorf("playa: PageLabels key %d is negative", i/2))
			}
			if i > 0 && page <= previousPage {
				return yield(pageLabelItem{}, fmt.Errorf("playa: PageLabels keys are not sorted"))
			}
			if bounds != nil && (page < bounds[0] || page > bounds[1]) {
				return yield(pageLabelItem{}, fmt.Errorf("playa: PageLabels key %d is outside Limits", i/2))
			}
			previousPage = page
			dictValue, resolved := d.resolveIndirectChain(nums[i+1])
			if !resolved {
				return yield(pageLabelItem{}, fmt.Errorf("playa: PageLabels value %d could not be resolved", i/2))
			}
			dict, ok := dictValue.(Dict)
			if !ok {
				return yield(pageLabelItem{}, fmt.Errorf("playa: PageLabels value %d is not a dictionary", i/2))
			}
			style := ""
			prefix := ""
			start := 1
			prefixValue, prefixResolved := d.resolveIndirectChain(dict[Name("P")])
			if _, present := dict[Name("P")]; present && !prefixResolved {
				return yield(pageLabelItem{}, fmt.Errorf("playa: PageLabels prefix could not be resolved"))
			}
			if value, ok := prefixValue.(String); ok {
				prefix = decodePDFText(value)
			}
			styleValue, styleResolved := d.resolveIndirectChain(dict[Name("S")])
			if _, present := dict[Name("S")]; present && !styleResolved {
				return yield(pageLabelItem{}, fmt.Errorf("playa: PageLabels style could not be resolved"))
			}
			if value, ok := styleValue.(Name); ok {
				style = string(value)
			}
			if value, ok := dict[Name("St")]; ok {
				startValue, startResolved := d.resolveIndirectChain(value)
				if !startResolved {
					return yield(pageLabelItem{}, fmt.Errorf("playa: PageLabels start could not be resolved"))
				}
				if parsedStart, ok := IntValue(startValue); ok && parsedStart > 0 {
					start = parsedStart
				}
			}
			spec := documentdata.NewPageLabelSpec(style, prefix, start)
			if !yield(pageLabelItem{page: page, spec: spec}, nil) {
				return false
			}
		}
	}
	if raw, present := tree[Name("Kids")]; present {
		resolvedKids, resolved := d.resolveIndirectChain(raw)
		if !resolved {
			return yield(pageLabelItem{}, fmt.Errorf("playa: PageLabels Kids could not be resolved"))
		}
		kids, ok := resolvedKids.(Array)
		if !ok {
			return yield(pageLabelItem{}, fmt.Errorf("playa: PageLabels Kids is not an array"))
		}
		var previousChildBounds *[2]int
		for _, kid := range kids {
			if ref, ok := d.finalIndirectRef(kid); ok {
				if seen[ref] {
					continue
				}
				seen[ref] = true
			}
			childValue, resolved := d.resolveIndirectChain(kid)
			if !resolved {
				return yield(pageLabelItem{}, fmt.Errorf("playa: PageLabels child could not be resolved"))
			}
			child, ok := childValue.(Dict)
			if !ok {
				return yield(pageLabelItem{}, fmt.Errorf("playa: PageLabels child is not a dictionary"))
			}
			childBounds, err := d.parsePageLabelLimits(child)
			if err != nil {
				return yield(pageLabelItem{}, err)
			}
			if childBounds != nil && previousChildBounds != nil {
				if childBounds[0] < previousChildBounds[0] {
					return yield(pageLabelItem{}, fmt.Errorf("playa: PageLabels sibling Limits are not sorted"))
				}
				if childBounds[0] <= previousChildBounds[1] {
					return yield(pageLabelItem{}, fmt.Errorf("playa: PageLabels sibling Limits overlap"))
				}
			}
			if childBounds != nil {
				copyBounds := *childBounds
				previousChildBounds = &copyBounds
			}
			if !d.walkPageLabelTreeSeq(child, seen, bounds, yield) {
				return false
			}
		}
	}
	return true
}

func (d *Document) parsePageLabelLimits(tree Dict) (*[2]int, error) {
	raw, present := tree[Name("Limits")]
	if !present {
		return nil, nil
	}
	value, resolved := d.resolveIndirectChain(raw)
	if !resolved {
		return nil, fmt.Errorf("playa: PageLabels Limits could not be resolved")
	}
	limits, ok := value.(Array)
	if !ok || len(limits) != 2 {
		return nil, fmt.Errorf("playa: PageLabels Limits must contain two integers")
	}
	parsed := [2]int{}
	for i, item := range limits {
		resolvedItem, resolved := d.resolveIndirectChain(item)
		if !resolved {
			return nil, fmt.Errorf("playa: PageLabels Limit %d could not be resolved", i)
		}
		integer, ok := IntValue(resolvedItem)
		if !ok || integer < 0 {
			return nil, fmt.Errorf("playa: PageLabels Limits must contain non-negative integers")
		}
		parsed[i] = integer
	}
	if parsed[0] > parsed[1] {
		return nil, fmt.Errorf("playa: PageLabels Limits are out of order")
	}
	return &parsed, nil
}

func (d *Document) pageLabelItems() []pageLabelItem {
	items, _ := d.pageLabelItemsWithError()
	return items
}

func (d *Document) pageLabelItemsWithError() ([]pageLabelItem, error) {
	d.cacheMu.RLock()
	if d.pageLabelsReady {
		items := d.pageLabelCache
		err := d.pageLabelsErr
		d.cacheMu.RUnlock()
		return items, err
	}
	d.cacheMu.RUnlock()
	items := []pageLabelItem{}
	var itemsErr error
	for item, err := range d.pageLabelItemsSeq() {
		if err != nil {
			items = nil
			itemsErr = err
			break
		}
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].page < items[j].page })
	if len(items) > 0 && items[0].page > 0 {
		items[0].page = 0
	}
	return d.publishPageLabelItems(items, itemsErr), itemsErr
}

func (d *Document) publishPageLabelItems(items []pageLabelItem, itemsErr error) []pageLabelItem {
	d.cacheMu.Lock()
	defer d.cacheMu.Unlock()
	if d.pageLabelsReady {
		return d.pageLabelCache
	}
	cacheBytes := pageLabelItemsCacheSize(items)
	if itemsErr == nil && cacheBytes > d.cacheLimits().PageLabelRuleBytes {
		return items
	}
	d.pageLabelCache, d.pageLabelsErr, d.pageLabelsReady = items, itemsErr, true
	d.pageLabelCacheBytes = cacheBytes
	return d.pageLabelCache
}
