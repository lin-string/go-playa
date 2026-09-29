package document

import (
	"fmt"
	"iter"
	"math"
	"sort"

	"github.com/lin-string/go-playa/documentdata"
)

const nameTreeCacheLimit = 32 << 20
const destinationCacheLimit = 32 << 20
const destinationsRootErrorCacheLimit = 64 << 10
const nameTreeErrorCacheLimit = 64 << 10

// DestinationEntry is one named destination in document order.
type DestinationEntry struct {
	data documentdata.DestinationEntry
}

// Name returns the named destination key.
func (e DestinationEntry) Name() string { return e.data.Name() }

// ValueCopy returns an independent copy of the named destination value.
func (e DestinationEntry) ValueCopy() Object { return e.data.ValueCopy() }

// Finalize returns an independent snapshot of the named destination entry.
func (e DestinationEntry) Finalize() DestinationEntry {
	return DestinationEntry{data: e.data.Finalize()}
}

// Destination resolves this named entry to the normalized destination model.
func (e DestinationEntry) Destination(d *Document) *Destination {
	destination, _ := e.DestinationWithError(d)
	return destination
}

// DestinationWithError resolves this named entry and reports malformed
// destination data without changing the borrowed entry semantics.
func (e DestinationEntry) DestinationWithError(d *Document) (*Destination, error) {
	if d == nil {
		return nil, errNilDocument
	}
	return d.ResolveDestinationWithError(e.ValueCopy())
}

func newDestinationEntry(name string, value Object) DestinationEntry {
	return DestinationEntry{data: documentdata.NewDestinationEntry(name, value)}
}

// NameTreeEntry is one entry from a catalog /Names name tree.
type NameTreeEntry = documentdata.NameTreeEntry

func newNameTreeEntry(name string, value Object) NameTreeEntry {
	return documentdata.NewNameTreeEntry(name, value)
}

func nameTreeObjectSize(value Object, depth int) int {
	if depth <= 0 {
		return 16
	}
	size := 16
	switch value := value.(type) {
	case String:
		size = addCacheSize(size, len(value))
	case Name:
		size = addCacheSize(size, len(value))
	case Keyword:
		size = addCacheSize(size, len(value))
	case Array:
		for _, item := range value {
			size = addCacheSize(size, nameTreeObjectSize(item, depth-1))
		}
	case InvalidArray:
		for _, item := range value {
			size = addCacheSize(size, nameTreeObjectSize(item, depth-1))
		}
	case Dict:
		for key, item := range value {
			size = addCacheSize(size, len(key))
			size = addCacheSize(size, nameTreeObjectSize(item, depth-1))
		}
	case Stream:
		size = addCacheSize(size, len(value.DataBorrowed()))
		size = addCacheSize(size, nameTreeObjectSize(value.DictBorrowed(), depth-1))
	}
	return size
}

func addCacheSize(size, addition int) int {
	if addition < 0 || size > math.MaxInt-addition {
		return math.MaxInt
	}
	return size + addition
}

// NameTreeSeq lazily walks the named tree at catalog /Names/<name> and yields
// borrowed entries. Call Finalize when an independent snapshot is required.
func (d *Document) NameTreeSeq(name string) iter.Seq2[NameTreeEntry, error] {
	return func(yield func(NameTreeEntry, error) bool) {
		d.cacheMu.RLock()
		ready := d.nameTreeReady[name]
		cachedEntries := d.nameTreeCache[name]
		cachedErr, hasCachedErr := d.nameTreeErrors[name]
		d.cacheMu.RUnlock()
		if hasCachedErr {
			yield(NameTreeEntry{}, cachedErr)
			return
		}
		if ready {
			for _, entry := range cachedEntries {
				if !yield(entry, nil) {
					return
				}
			}
			return
		}
		cacheError := func(err error) bool {
			d.cacheMu.Lock()
			errorBytes := len(err.Error())
			if !d.nameTreeReady[name] && errorBytes <= d.cacheLimits().NameTreeErrorBytes && cacheFits(d.nameTreeErrorBytes, errorBytes, d.cacheLimits().NameTreeErrorBytes) {
				if d.nameTreeErrors == nil {
					d.nameTreeErrors = map[string]error{}
				}
				if _, exists := d.nameTreeErrors[name]; !exists {
					d.nameTreeErrors[name] = err
					d.nameTreeErrorBytes += errorBytes
				}
			}
			d.cacheMu.Unlock()
			return yield(NameTreeEntry{}, err)
		}
		rawCatalog, present := d.trailer[Name("Root")]
		if !present {
			return
		}
		catalogValue, ok := d.resolveIndirectChain(rawCatalog)
		if !ok {
			cacheError(fmt.Errorf("playa: catalog root is not a dictionary"))
			return
		}
		catalog, ok := catalogValue.(Dict)
		if !ok {
			cacheError(fmt.Errorf("playa: catalog root is not a dictionary"))
			return
		}
		rawNames, present := catalog[Name("Names")]
		if !present {
			return
		}
		namesValue, ok := d.resolveIndirectChain(rawNames)
		if !ok {
			cacheError(fmt.Errorf("playa: catalog Names is not a dictionary"))
			return
		}
		names, ok := namesValue.(Dict)
		if !ok {
			cacheError(fmt.Errorf("playa: catalog Names is not a dictionary"))
			return
		}
		root := names[Name(name)]
		treeValue, resolved := d.resolveIndirectChain(root)
		tree, ok := treeValue.(Dict)
		if !resolved {
			ok = false
		}
		if !ok {
			if root != nil {
				cacheError(fmt.Errorf("playa: name tree %q root is not a dictionary", name))
			}
			return
		}
		seen := map[Ref]bool{}
		if ref, ok := d.finalIndirectRef(root); ok {
			seen[ref] = true
		}
		cached := make([]NameTreeEntry, 0)
		cachedBytes := 0
		cacheable := true
		complete := d.nameTreeSeq(tree, seen, nil, func(entry NameTreeEntry, err error) bool {
			if err != nil {
				return cacheError(err)
			}
			if cacheable {
				value := entry.ValueCopy()
				entryBytes := addCacheSize(len(entry.Name()), nameTreeObjectSize(value, 8))
				cachedBytes = addCacheSize(cachedBytes, entryBytes)
				if cachedBytes > d.cacheLimits().NameTreeBytes {
					cacheable = false
					cached = nil
				} else {
					cached = append(cached, documentdata.NewNameTreeEntry(entry.Name(), value))
				}
			}
			return yield(entry, nil)
		})
		if complete && cacheable {
			d.cacheMu.Lock()
			defer d.cacheMu.Unlock()
			if d.nameTreeReady[name] {
				return
			}
			if d.nameTreeCache == nil {
				d.nameTreeCache = map[string][]NameTreeEntry{}
			}
			if d.nameTreeReady == nil {
				d.nameTreeReady = map[string]bool{}
			}
			d.nameTreeCache[name] = cached
			d.nameTreeReady[name] = true
		}
	}
}

// DestinationsSeq lazily walks catalog and name-tree destinations, yielding
// borrowed entries. Direct dictionary keys are sorted for repeatable
// traversal; name-tree arrays retain their source order. A consumer may stop
// without resolving later entries. Repeated names yield the first value, as
// in Playa's Destinations.items, while retaining every name-tree entry.
func (d *Document) DestinationsSeq() iter.Seq2[DestinationEntry, error] {
	return func(yield func(DestinationEntry, error) bool) {
		d.cacheMu.RLock()
		if d.destinationsRootErrReady {
			err := d.destinationsRootErr
			d.cacheMu.RUnlock()
			yield(DestinationEntry{}, err)
			return
		}
		d.cacheMu.RUnlock()
		cacheRootError := func(err error) error {
			if len(err.Error()) > d.cacheLimits().DestinationsRootErrorBytes {
				return err
			}
			d.cacheMu.Lock()
			defer d.cacheMu.Unlock()
			if d.destinationsRootErrReady {
				return d.destinationsRootErr
			}
			d.destinationsRootErr = err
			d.destinationsRootErrReady = true
			return err
		}
		firstValues := make(map[string]Object)
		emit := func(entry DestinationEntry) bool {
			name := entry.Name()
			value, exists := firstValues[name]
			if !exists {
				value = entry.ValueCopy()
				firstValues[name] = value
			}
			d.cacheDestination(name, value)
			return yield(newDestinationEntry(name, value), nil)
		}
		catalog, err := d.catalogWithError()
		if err != nil {
			yield(DestinationEntry{}, err)
			return
		}
		if catalog == nil {
			return
		}
		if raw, present := catalog[Name("Dests")]; present {
			directValue, resolved := d.resolveIndirectChain(raw)
			if !resolved {
				yield(DestinationEntry{}, cacheRootError(fmt.Errorf("playa: catalog Dests could not be resolved")))
				return
			}
			direct, ok := directValue.(Dict)
			if !ok {
				yield(DestinationEntry{}, cacheRootError(fmt.Errorf("playa: catalog Dests is not a dictionary")))
				return
			}
			if !d.destinationDictSeq(direct, func(entry DestinationEntry, _ error) bool {
				return emit(entry)
			}) {
				return
			}
			// Playa treats the legacy catalog dictionary and the modern
			// name-tree form as alternatives. A present /Dests entry wins;
			// do not append a second view from /Names/Dests.
			return
		}
		for entry, err := range d.NameTreeSeq("Dests") {
			if err != nil {
				if !yield(DestinationEntry{}, err) {
					return
				}
				return
			}
			if !emit(newDestinationEntry(entry.Name(), entry.ValueCopy())) {
				return
			}
		}
	}
}

func (d *Document) cacheDestination(name string, value Object) {
	entryBytes := addCacheSize(len(name), nameTreeObjectSize(value, 8))
	d.cacheMu.Lock()
	defer d.cacheMu.Unlock()
	if _, exists := d.destinationCache[name]; exists {
		return
	}
	limit := d.cacheLimits().DestinationBytes
	if entryBytes == math.MaxInt || !cacheFits(d.destinationCacheBytes, entryBytes, limit) {
		return
	}
	if d.destinationCache == nil {
		d.destinationCache = map[string]Object{}
	}
	d.destinationCache[name] = value
	d.destinationCacheBytes += entryBytes
}

func (d *Document) destinationDictSeq(dict Dict, yield func(DestinationEntry, error) bool) bool {
	names := make([]string, 0, len(dict))
	for name := range dict {
		names = append(names, string(name))
	}
	sort.Strings(names)
	for _, name := range names {
		if !yield(newDestinationEntry(name, dict[Name(name)]), nil) {
			return false
		}
	}
	return true
}

func (d *Document) nameTreeSeq(tree Dict, seen map[Ref]bool, parentBounds *[2]string, yield func(NameTreeEntry, error) bool) bool {
	_, hasNames := tree[Name("Names")]
	_, hasKids := tree[Name("Kids")]
	if hasNames && hasKids {
		return yield(NameTreeEntry{}, fmt.Errorf("playa: name tree node cannot contain both Names and Kids"))
	}
	bounds, err := d.parseNameTreeLimits(tree)
	if err != nil {
		return yield(NameTreeEntry{}, err)
	}
	if bounds != nil && parentBounds != nil && (bounds[0] < parentBounds[0] || bounds[1] > parentBounds[1]) {
		return yield(NameTreeEntry{}, fmt.Errorf("playa: name tree child Limits are outside parent"))
	}
	if raw, present := tree[Name("Names")]; present {
		resolvedArray, resolved := d.resolveIndirectChain(raw)
		if !resolved {
			return yield(NameTreeEntry{}, fmt.Errorf("playa: name tree Names could not be resolved"))
		}
		a, ok := resolvedArray.(Array)
		if !ok {
			return yield(NameTreeEntry{}, fmt.Errorf("playa: name tree Names is not an array"))
		}
		if len(a)%2 != 0 {
			return yield(NameTreeEntry{}, fmt.Errorf("playa: name tree Names has an unmatched key"))
		}
		previous := ""
		for i := 0; i < len(a); i += 2 {
			nameValue, resolved := d.resolveIndirectChain(a[i])
			if !resolved {
				return yield(NameTreeEntry{}, fmt.Errorf("playa: name tree key %d could not be resolved", i/2))
			}
			nameValueString, ok := nameValue.(String)
			if !ok {
				return yield(NameTreeEntry{}, fmt.Errorf("playa: name tree key %d is not a string", i/2))
			}
			name := decodePDFText(nameValueString)
			if previous != "" && name < previous {
				return yield(NameTreeEntry{}, fmt.Errorf("playa: name tree Names are not sorted"))
			}
			if bounds != nil && (name < bounds[0] || name > bounds[1]) {
				return yield(NameTreeEntry{}, fmt.Errorf("playa: name tree name is outside Limits"))
			}
			previous = name
			if !yield(newNameTreeEntry(name, a[i+1]), nil) {
				return false
			}
		}
	}
	if raw, present := tree[Name("Kids")]; present {
		resolvedKids, resolved := d.resolveIndirectChain(raw)
		if !resolved {
			return yield(NameTreeEntry{}, fmt.Errorf("playa: name tree Kids could not be resolved"))
		}
		kids, ok := resolvedKids.(Array)
		if !ok {
			return yield(NameTreeEntry{}, fmt.Errorf("playa: name tree Kids is not an array"))
		}
		var previousChildBounds *[2]string
		for _, kid := range kids {
			if ref, ok := d.finalIndirectRef(kid); ok {
				if seen[ref] {
					continue
				}
				seen[ref] = true
			}
			childValue, resolved := d.resolveIndirectChain(kid)
			if !resolved {
				return yield(NameTreeEntry{}, fmt.Errorf("playa: name tree child could not be resolved"))
			}
			child, ok := childValue.(Dict)
			if !ok {
				return yield(NameTreeEntry{}, fmt.Errorf("playa: name tree child is not a dictionary"))
			}
			childBounds, err := d.parseNameTreeLimits(child)
			if err != nil {
				return yield(NameTreeEntry{}, err)
			}
			if childBounds != nil && previousChildBounds != nil {
				if childBounds[0] < previousChildBounds[0] {
					return yield(NameTreeEntry{}, fmt.Errorf("playa: name tree sibling Limits are not sorted"))
				}
				if childBounds[0] <= previousChildBounds[1] {
					return yield(NameTreeEntry{}, fmt.Errorf("playa: name tree sibling Limits overlap"))
				}
			}
			if childBounds != nil {
				boundsCopy := *childBounds
				previousChildBounds = &boundsCopy
			}
			if !d.nameTreeSeq(child, seen, bounds, yield) {
				return false
			}
		}
	}
	return true
}

func (d *Document) parseNameTreeLimits(tree Dict) (*[2]string, error) {
	raw, present := tree[Name("Limits")]
	if !present {
		return nil, nil
	}
	limitsValue, resolved := d.resolveIndirectChain(raw)
	if !resolved {
		return nil, fmt.Errorf("playa: name tree Limits could not be resolved")
	}
	limits, ok := limitsValue.(Array)
	if !ok || len(limits) != 2 {
		return nil, fmt.Errorf("playa: name tree Limits must contain two strings")
	}
	parsed := [2]string{}
	for i, value := range limits {
		resolvedValue, resolved := d.resolveIndirectChain(value)
		if !resolved {
			return nil, fmt.Errorf("playa: name tree Limits bound %d could not be resolved", i)
		}
		name, ok := resolvedValue.(String)
		if !ok {
			return nil, fmt.Errorf("playa: name tree Limits must contain two strings")
		}
		parsed[i] = decodePDFText(name)
	}
	if parsed[0] > parsed[1] {
		return nil, fmt.Errorf("playa: name tree Limits are out of order")
	}
	return &parsed, nil
}

func (d *Document) Destinations() (map[string]Object, error) {
	d.destinationsMu.Lock()
	defer d.destinationsMu.Unlock()
	d.cacheMu.RLock()
	if d.destinationsReady {
		out := cloneDestinationMap(d.destinationsCache)
		err := d.destinationsErr
		d.cacheMu.RUnlock()
		return out, err
	}
	d.cacheMu.RUnlock()

	cache := map[string]Object{}
	cacheBytes := 0
	var destinationsErr error
	for entry, err := range d.DestinationsSeq() {
		if err != nil {
			destinationsErr = err
			break
		}
		if _, exists := cache[entry.Name()]; exists {
			continue
		}
		value := entry.ValueCopy()
		cache[entry.Name()] = value
		cacheBytes = addCacheSize(cacheBytes, addCacheSize(len(entry.Name()), nameTreeObjectSize(value, 8)))
	}
	if destinationsErr != nil {
		cache = map[string]Object{}
		cacheBytes = 0
	}
	cacheable := destinationsErr == nil && cacheBytes <= d.cacheLimits().DestinationBytes
	d.cacheMu.Lock()
	if cacheable {
		d.destinationsCache = cache
		d.destinationsCacheBytes = cacheBytes
		d.destinationsCacheable = true
		d.destinationsReady = true
		d.destinationsErr = nil
	} else if destinationsErr != nil {
		d.destinationsCache = nil
		d.destinationsCacheBytes = 0
		d.destinationsCacheable = true
		d.destinationsReady = true
		d.destinationsErr = destinationsErr
	} else {
		d.destinationsCache = nil
		d.destinationsCacheBytes = 0
		// A malformed tree must not publish an empty snapshot. Keep the
		// incremental cache available for valid entries already visited by
		// DestinationsSeq, while allowing later named lookups to traverse again.
		d.destinationsCacheable = false
		d.destinationsReady = false
		d.destinationsErr = nil
	}
	d.cacheMu.Unlock()
	if destinationsErr != nil {
		return nil, destinationsErr
	}
	return cloneDestinationMap(cache), nil
}

func cloneDestinationMap(source map[string]Object) map[string]Object {
	if source == nil {
		return nil
	}
	out := make(map[string]Object, len(source))
	for name, value := range source {
		out[name] = cloneGraphicsObject(value)
	}
	return out
}
