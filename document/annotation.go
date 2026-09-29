package document

import (
	"encoding/json"
	"fmt"
	"iter"

	"github.com/lin-string/go-playa/documentdata"
)

const annotationCacheLimit = 32 << 20
const annotationRootErrorCacheLimit = 64 << 10
const annotationErrorCacheLimit = 64 << 10

type Annotation struct {
	data        documentdata.Annotation
	actionValue *Action
}

// Subtype returns the annotation subtype.
func (a Annotation) Subtype() string { return a.data.Subtype() }

// Page returns the owning page reference, when present.
func (a Annotation) Page() Ref {
	page, _ := a.data.Page()
	return page
}

// HasPage reports whether Page identifies an owning page.
func (a Annotation) HasPage() bool { return a.data.HasPage() }

// InReplyTo returns the referenced parent annotation, when present.
func (a Annotation) InReplyTo() Ref {
	ref, _ := a.data.InReplyTo()
	return ref
}

// HasInReplyTo reports whether InReplyTo is present.
func (a Annotation) HasInReplyTo() bool { return a.data.HasInReplyTo() }

// Popup returns the referenced popup annotation, when present.
func (a Annotation) Popup() Ref {
	ref, _ := a.data.Popup()
	return ref
}

// HasPopup reports whether Popup is present.
func (a Annotation) HasPopup() bool { return a.data.HasPopup() }

// Rect returns the source annotation rectangle in default user space.
func (a Annotation) Rect() [4]float64 { return a.data.Rect() }

// Contents returns the decoded annotation contents.
func (a Annotation) Contents() string { return a.data.Contents() }

// URI returns the decoded URI action target.
func (a Annotation) URI() string { return a.data.URI() }

// ActionKind returns the normalized action subtype.
func (a Annotation) ActionKind() string { return a.data.ActionKind() }

// Border returns the annotation border components.
func (a Annotation) Border() [3]float64 { return a.data.Border() }

// HasBorder reports whether a valid border was present.
func (a Annotation) HasBorder() bool { return a.data.HasBorder() }

// Name returns the annotation name.
func (a Annotation) Name() string { return a.data.Name() }

// Modified returns the decoded modification timestamp.
func (a Annotation) Modified() string { return a.data.Modified() }

// ParentKey returns the structure ParentTree key, when present.
func (a Annotation) ParentKey() int {
	key, _ := a.data.ParentKey()
	return key
}

// HasParentKey reports whether ParentKey is present.
func (a Annotation) HasParentKey() bool { return a.data.HasParentKey() }

// Flags returns the annotation flags, when present.
func (a Annotation) Flags() int {
	flags, _ := a.data.Flags()
	return flags
}

// HasFlags reports whether Flags is present.
func (a Annotation) HasFlags() bool { return a.data.HasFlags() }

func annotationCacheSize(annotation Annotation) int {
	dict := annotation.data.DictCopy()
	action := annotation.data.ActionCopy()
	appearance := annotation.data.AppearanceCopy()
	dest := annotation.data.DestCopy()
	quadPoints := annotation.data.QuadPointsCopy()
	color := annotation.data.ColorCopy()
	size := 512
	for _, addition := range []int{len(annotation.Subtype()), len(annotation.Contents()), len(annotation.URI()), len(annotation.ActionKind()), len(annotation.Name()), len(annotation.Modified()), nameTreeObjectSize(dict, 8), nameTreeObjectSize(action, 8), nameTreeObjectSize(appearance, 8), nameTreeObjectSize(dest, 8), outlineActionCacheSize(annotation.actionValue, 8), cacheMulSize(len(quadPoints), 16), cacheMulSize(len(color), 8)} {
		size = addCacheSize(size, addition)
	}
	return size
}

func cloneAnnotation(annotation Annotation) Annotation {
	annotation.data = annotation.data.Finalize()
	annotation.actionValue = finalizeAction(annotation.actionValue)
	return annotation
}

func cloneAnnotationWithError(annotation Annotation) (Annotation, error) {
	clone := cloneAnnotation(annotation)
	if annotation.actionValue != nil {
		action, err := annotation.actionValue.FinalizeWithError()
		if err != nil {
			return Annotation{}, err
		}
		clone.actionValue = &action
	}
	return clone, nil
}

// QuadPointsCopy returns independent annotation quadrilateral points.
func (a Annotation) QuadPointsCopy() [][2]float64 {
	return a.data.QuadPointsCopy()
}

// ColorCopy returns independent annotation color components.
func (a Annotation) ColorCopy() []float64 { return a.data.ColorCopy() }

// DictCopy returns an independent copy of the source annotation dictionary.
func (a Annotation) DictCopy() Dict { return a.data.DictCopy() }

// ActionCopy returns an independent copy of the annotation action dictionary.
func (a Annotation) ActionCopy() Dict { return a.data.ActionCopy() }

// ActionValueCopy returns an independent normalized annotation action.
func (a Annotation) ActionValueCopy() *Action { return finalizeAction(a.actionValue) }

// ActionValueCopyWithError returns an independent annotation action and
// reports malformed lazy action chains.
func (a Annotation) ActionValueCopyWithError() (*Action, error) {
	if a.actionValue == nil {
		return nil, nil
	}
	action, err := a.actionValue.FinalizeWithError()
	if err != nil {
		return nil, err
	}
	return &action, nil
}

// DestCopy returns an independent copy of the raw annotation destination.
func (a Annotation) DestCopy() Object { return a.data.DestCopy() }

// AppearanceCopy returns an independent copy of the annotation appearance dictionary.
func (a Annotation) AppearanceCopy() Dict { return a.data.AppearanceCopy() }

func (a Annotation) MarshalJSON() ([]byte, error) {
	type projection struct {
		Subtype      string
		Page         Ref
		HasPage      bool
		InReplyTo    Ref
		HasInReplyTo bool
		Popup        Ref
		HasPopup     bool
		Rect         [4]float64
		Contents     string
		Dict         Dict `json:"Dict"`
		URI          string
		Action       Dict    `json:"Action"`
		ActionValue  *Action `json:"ActionValue"`
		Dest         Object  `json:"Dest"`
		ActionKind   string
		Appearance   Dict         `json:"Appearance"`
		QuadPoints   [][2]float64 `json:"QuadPoints"`
		Border       [3]float64
		HasBorder    bool
		Color        []float64 `json:"Color"`
		Name         string
		Modified     string
		ParentKey    int
		HasParentKey bool
		Flags        int
		HasFlags     bool
	}
	page, hasPage := a.data.Page()
	inReplyTo, hasInReplyTo := a.data.InReplyTo()
	popup, hasPopup := a.data.Popup()
	parentKey, hasParentKey := a.data.ParentKey()
	flags, hasFlags := a.data.Flags()
	return json.Marshal(projection{
		Subtype: a.Subtype(), Page: page, HasPage: hasPage, InReplyTo: inReplyTo,
		HasInReplyTo: hasInReplyTo, Popup: popup, HasPopup: hasPopup, Rect: a.Rect(),
		Contents: a.Contents(), Dict: a.DictCopy(), URI: a.URI(), Action: a.ActionCopy(),
		ActionValue: a.actionValue, Dest: a.DestCopy(), ActionKind: a.ActionKind(),
		Appearance: a.AppearanceCopy(), QuadPoints: a.QuadPointsCopy(), Border: a.Border(),
		HasBorder: a.HasBorder(), Color: a.ColorCopy(), Name: a.Name(), Modified: a.Modified(),
		ParentKey: parentKey, HasParentKey: hasParentKey, Flags: flags, HasFlags: hasFlags,
	})
}

// Finalize returns an independent snapshot of this annotation.
func (a Annotation) FinalizeWithError() (Annotation, error) {
	return cloneAnnotationWithError(a)
}

func (a Annotation) Finalize() Annotation {
	clone, _ := a.FinalizeWithError()
	return clone
}

// PageObject resolves the annotation's owning page.
func (a Annotation) PageObject(d *Document) (Page, error) {
	if d == nil {
		return Page{}, errNilDocument
	}
	if !a.HasPage() {
		return Page{}, ErrPageNotFound
	}
	return d.PageByRef(a.Page())
}

// BBox returns the annotation rectangle transformed into the document's
// selected device space. Rect remains the original default user-space box.
func (a Annotation) BBox(d *Document) [4]float64 {
	page, err := a.PageObject(d)
	if err != nil {
		return a.Rect()
	}
	rect := a.Rect()
	matrix := page.MatrixIn(d, page.coordinateSpace(d))
	points := [4][2]float64{
		{rect[0], rect[1]},
		{rect[2], rect[1]},
		{rect[2], rect[3]},
		{rect[0], rect[3]},
	}
	firstX, firstY, ok := matrix.PointFinite(points[0][0], points[0][1])
	if !ok {
		return rect
	}
	box := [4]float64{firstX, firstY, firstX, firstY}
	for _, point := range points[1:] {
		x, y, ok := matrix.PointFinite(point[0], point[1])
		if !ok {
			return rect
		}
		if x < box[0] {
			box[0] = x
		}
		if y < box[1] {
			box[1] = y
		}
		if x > box[2] {
			box[2] = x
		}
		if y > box[3] {
			box[3] = y
		}
	}
	return box
}

// Destination resolves this annotation's direct destination or local GoTo
// action without executing any action.
func (a Annotation) Destination(d *Document) *Destination {
	destination, _ := a.DestinationWithError(d)
	return destination
}

// DestinationWithError resolves this annotation's direct destination or
// local GoTo action and reports malformed destination data.
func (a Annotation) DestinationWithError(d *Document) (*Destination, error) {
	if d == nil {
		return nil, errNilDocument
	}
	dest := a.DestCopy()
	if dest != nil {
		destination, err := d.ResolveDestinationWithError(dest)
		if err != nil {
			return nil, err
		}
		if destination != nil {
			return destination, nil
		}
	}
	if a.actionValue != nil && a.actionValue.Kind() == "GoTo" {
		action := a.ActionCopy()
		if action != nil {
			if value, present := action[Name("D")]; present {
				destination, err := d.ResolveDestinationWithError(value)
				if err != nil {
					return nil, err
				}
				if destination != nil {
					return destination, nil
				}
			}
		}
		return a.actionValue.destination, nil
	}
	return nil, nil
}

// InReplyToAnnotation resolves the annotation referenced by /IRT.
func (a Annotation) InReplyToAnnotation(d *Document) *Annotation {
	annotation, _ := a.InReplyToAnnotationWithError(d)
	return annotation
}

// InReplyToAnnotationWithError resolves /IRT and reports a malformed
// referenced annotation.
func (a Annotation) InReplyToAnnotationWithError(d *Document) (*Annotation, error) {
	if d == nil {
		return nil, errNilDocument
	}
	if !a.HasInReplyTo() {
		return nil, nil
	}
	annotation, err := d.annotationWithError(a.InReplyTo())
	if err != nil {
		return nil, err
	}
	return &annotation, nil
}

// PopupAnnotation resolves the annotation's optional /Popup reference.
func (a Annotation) PopupAnnotation(d *Document) *Annotation {
	annotation, _ := a.PopupAnnotationWithError(d)
	return annotation
}

// PopupAnnotationWithError resolves /Popup and reports a malformed
// referenced annotation.
func (a Annotation) PopupAnnotationWithError(d *Document) (*Annotation, error) {
	if d == nil {
		return nil, errNilDocument
	}
	if !a.HasPopup() {
		return nil, nil
	}
	popup, err := d.annotationWithError(a.Popup())
	if err != nil {
		return nil, err
	}
	return &popup, nil
}

// Parent resolves the structure element associated with this annotation.
func (a Annotation) Parent(d *Document) *StructElement {
	parent, _ := a.ParentWithError(d)
	return parent
}

// ParentWithError resolves the annotation's page ParentTree slot and reports
// malformed page or ParentTree data.
func (a Annotation) ParentWithError(d *Document) (*StructElement, error) {
	if d == nil {
		return nil, errNilDocument
	}
	if !a.HasParentKey() {
		return nil, nil
	}
	if !a.HasPage() || a.Page() == (Ref{}) {
		return nil, fmt.Errorf("playa: annotation page could not be resolved")
	}
	return d.pageContentParentWithError(a.Page(), a.ParentKey())
}

// Annotations returns page annotations in array order.
//
// A missing Annots entry is empty; an explicitly malformed root or entry is
// reported through the sequence. The sequence can be traversed more than once
// and stops as soon as the consumer stops yielding.
func (d *Document) Annotations(p Page) iter.Seq2[Annotation, error] {
	return func(yield func(Annotation, error) bool) {
		pageRef, hasPageRef := p.ref, p.ref != (Ref{})
		if hasPageRef {
			d.cacheMu.RLock()
			cachedErr, cached := d.annotationRootErrors[pageRef]
			d.cacheMu.RUnlock()
			if cached {
				yield(Annotation{}, cachedErr)
				return
			}
		}
		cacheRootError := func(err error) error {
			if hasPageRef && len(err.Error()) <= d.cacheLimits().AnnotationRootErrorBytes {
				d.cacheMu.Lock()
				if d.annotationRootErrors == nil {
					d.annotationRootErrors = map[Ref]error{}
				}
				if cachedErr, cached := d.annotationRootErrors[pageRef]; cached {
					err = cachedErr
				} else if cacheFits(d.annotationRootErrorBytes, len(err.Error()), d.cacheLimits().AnnotationRootErrorBytes) {
					d.annotationRootErrors[pageRef] = err
					d.annotationRootErrorBytes += len(err.Error())
				}
				d.cacheMu.Unlock()
			}
			return err
		}
		raw, present := p.dict[Name("Annots")]
		if !present {
			return
		}
		resolved, ok := d.resolveIndirectChain(raw)
		if !ok {
			yield(Annotation{}, cacheRootError(fmt.Errorf("playa: annotations root could not be resolved")))
			return
		}
		a, ok := resolved.(Array)
		if !ok {
			yield(Annotation{}, cacheRootError(fmt.Errorf("playa: annotations root is not an array")))
			return
		}
		for _, x := range a {
			annotation, err := d.annotationSequenceItem(x)
			if err != nil {
				yield(Annotation{}, err)
				return
			}
			annotation.data = annotation.data.WithPage(p.ref, p.ref != (Ref{}))
			if !yield(annotation, nil) {
				return
			}
		}
	}
}

func (d *Document) annotationSequenceItem(o Object) (Annotation, error) {
	ref, hasRef := d.finalIndirectRef(o)
	if hasRef {
		d.cacheMu.RLock()
		cachedErr, cached := d.annotationErrors[ref]
		d.cacheMu.RUnlock()
		if cached {
			return Annotation{}, cachedErr
		}
	}
	cacheError := func(err error) (Annotation, error) {
		if hasRef && len(err.Error()) <= d.cacheLimits().AnnotationErrorBytes {
			d.cacheMu.Lock()
			defer d.cacheMu.Unlock()
			if d.annotationErrors == nil {
				d.annotationErrors = map[Ref]error{}
			}
			if _, exists := d.annotationErrors[ref]; !exists && cacheFits(d.annotationErrorBytes, len(err.Error()), d.cacheLimits().AnnotationErrorBytes) {
				d.annotationErrors[ref] = err
				d.annotationErrorBytes += len(err.Error())
			}
		}
		return Annotation{}, err
	}
	resolvedObject, ok := d.resolveIndirectChain(o)
	if !ok {
		return cacheError(fmt.Errorf("playa: annotation could not be resolved"))
	}
	v, ok := resolvedObject.(Dict)
	if !ok {
		return cacheError(fmt.Errorf("playa: annotation is not a dictionary"))
	}
	subtypeValue, _ := d.resolveIndirectChain(v[Name("Subtype")])
	if _, ok := subtypeValue.(Name); !ok {
		return cacheError(fmt.Errorf("playa: annotation has invalid Subtype"))
	}
	rawRect, present := v[Name("Rect")]
	if !present {
		return cacheError(fmt.Errorf("playa: annotation has invalid Rect"))
	}
	resolvedRect, _ := d.resolveIndirectChain(rawRect)
	r, ok := resolvedRect.(Array)
	if !ok || len(r) != 4 {
		return cacheError(fmt.Errorf("playa: annotation has invalid Rect"))
	}
	for i := 0; i < 4; i++ {
		item, _ := d.resolveIndirectChain(r[i])
		if _, ok := finiteNumberValue(item); !ok {
			return cacheError(fmt.Errorf("playa: annotation has invalid Rect"))
		}
	}
	annotation, ok := d.annotation(o)
	if !ok {
		return cacheError(fmt.Errorf("playa: annotation could not be resolved"))
	}
	return annotation, nil
}

// CollectAnnotations materializes Annotations for adapters that need a complete list.
func (d *Document) CollectAnnotations(p Page) ([]Annotation, error) {
	var out []Annotation
	for annotation, err := range d.Annotations(p) {
		if err != nil {
			return nil, err
		}
		out = append(out, annotation)
	}
	return out, nil
}

func (d *Document) annotation(o Object) (Annotation, bool) {
	ref, hasRef := d.finalIndirectRef(o)
	if hasRef {
		d.cacheMu.RLock()
		if annotation, ok := d.annotationCache[ref]; ok {
			d.cacheMu.RUnlock()
			return annotation, true
		}
		d.cacheMu.RUnlock()
	}
	resolvedObject, _ := d.resolveIndirectChain(o)
	v, ok := resolvedObject.(Dict)
	if !ok {
		return Annotation{}, false
	}
	subtypeValue, _ := d.resolveIndirectChain(v[Name("Subtype")])
	n, _ := subtypeValue.(Name)
	var rect [4]float64
	if rawRect, present := v[Name("Rect")]; present {
		resolvedRect, _ := d.resolveIndirectChain(rawRect)
		r, ok := resolvedRect.(Array)
		if !ok || len(r) != 4 {
			return Annotation{}, false
		}
		for i := 0; i < 4; i++ {
			item, _ := d.resolveIndirectChain(r[i])
			number, valid := finiteNumberValue(item)
			if !valid {
				return Annotation{}, false
			}
			rect[i] = number
		}
	}
	c := ""
	contentsValue, _ := d.resolveIndirectChain(v[Name("Contents")])
	if s, ok := contentsValue.(String); ok {
		c = decodePDFText(s)
	}
	uri := ""
	var action Dict
	aValue, _ := d.resolveIndirectChain(v[Name("A")])
	if a, ok := aValue.(Dict); ok {
		action = a
		uriValue, _ := d.resolveIndirectChain(a[Name("URI")])
		if u, ok := uriValue.(String); ok {
			uri = decodePDFText(u)
		}
	}
	spec := documentdata.AnnotationSpec{
		Subtype: string(n), Rect: rect, Contents: c, Dict: v, URI: uri,
		Dest: v[Name("Dest")], Action: action,
	}
	actionKindValue, _ := d.resolveIndirectChain(action[Name("S")])
	if kind, ok := actionKindValue.(Name); ok {
		spec.ActionKind = string(kind)
	}
	apValue, _ := d.resolveIndirectChain(v[Name("AP")])
	if ap, ok := apValue.(Dict); ok {
		spec.Appearance = ap
	}
	if rawQuadPoints, present := v[Name("QuadPoints")]; present {
		resolvedQuadPoints, _ := d.resolveIndirectChain(rawQuadPoints)
		q, ok := resolvedQuadPoints.(Array)
		if !ok || len(q) == 0 || len(q)%8 != 0 {
			return Annotation{}, false
		}
		for i := 0; i < len(q); i += 2 {
			xValue, _ := d.resolveIndirectChain(q[i])
			yValue, _ := d.resolveIndirectChain(q[i+1])
			x, okx := finiteNumberValue(xValue)
			y, oky := finiteNumberValue(yValue)
			if !okx || !oky {
				return Annotation{}, false
			}
			spec.QuadPoints = append(spec.QuadPoints, [2]float64{x, y})
		}
	}
	borderValue, _ := d.resolveIndirectChain(v[Name("Border")])
	if border, ok := borderValue.(Array); ok {
		if len(border) < 3 || len(border) > 4 {
			return Annotation{}, false
		}
		if len(border) == 4 {
			dashValue, _ := d.resolveIndirectChain(border[3])
			dash, ok := dashValue.(Array)
			if !ok {
				return Annotation{}, false
			}
			for _, value := range dash {
				resolved, _ := d.resolveIndirectChain(value)
				if _, ok := finiteNumberValue(resolved); !ok {
					return Annotation{}, false
				}
			}
		}
		valid := true
		for i := 0; i < 3; i++ {
			resolved, _ := d.resolveIndirectChain(border[i])
			spec.Border[i], valid = finiteNumberValue(resolved)
			if !valid {
				break
			}
		}
		spec.HasBorder = valid
	}
	colorValue, _ := d.resolveIndirectChain(v[Name("C")])
	if color, ok := colorValue.(Array); ok {
		if len(color) == 1 || len(color) == 3 || len(color) == 4 {
			components := make([]float64, len(color))
			valid := true
			for i, value := range color {
				resolved, _ := d.resolveIndirectChain(value)
				components[i], valid = finiteNumberValue(resolved)
				if !valid {
					break
				}
			}
			if valid {
				spec.Color = components
			}
		}
	}
	nameValue, _ := d.resolveIndirectChain(v[Name("NM")])
	if name, ok := nameValue.(String); ok {
		spec.Name = decodePDFText(name)
	}
	modifiedValue, _ := d.resolveIndirectChain(v[Name("M")])
	if modified, ok := modifiedValue.(String); ok {
		spec.Modified = decodePDFText(modified)
	}
	parentValue, _ := d.resolveIndirectChain(v[Name("StructParent")])
	if parent, ok := IntValue(parentValue); ok && parent >= 0 {
		spec.ParentKey, spec.HasParentKey = parent, true
	}
	if reply, ok := d.finalIndirectRef(v[Name("IRT")]); ok {
		spec.InReplyTo, spec.HasInReplyTo = reply, true
	}
	if popup, ok := d.finalIndirectRef(v[Name("Popup")]); ok {
		spec.Popup, spec.HasPopup = popup, true
	}
	flagsValue, _ := d.resolveIndirectChain(v[Name("F")])
	if flags, ok := IntValue(flagsValue); ok {
		spec.Flags, spec.HasFlags = flags, true
	}
	ann := Annotation{data: documentdata.NewAnnotation(spec), actionValue: d.ResolveAction(v[Name("A")])}
	if hasRef {
		d.cacheMu.Lock()
		defer d.cacheMu.Unlock()
		if annotation, ok := d.annotationCache[ref]; ok {
			return annotation, true
		}
		if d.annotationCache == nil {
			d.annotationCache = map[Ref]Annotation{}
		}
		cacheBytes := annotationCacheSize(ann)
		limit := d.cacheLimits().AnnotationBytes
		if cacheFits(d.annotationCacheBytes, cacheBytes, limit) {
			if d.annotationCacheSizes == nil {
				d.annotationCacheSizes = map[Ref]int{}
			}
			d.annotationCache[ref] = ann
			d.annotationCacheSizes[ref] = cacheBytes
			d.annotationCacheBytes += cacheBytes
		}
	}
	return ann, true
}

func (d *Document) finalIndirectRef(value Object) (Ref, bool) {
	seen := map[Ref]bool{}
	var last Ref
	for {
		ref, ok := value.(Ref)
		if !ok {
			return last, last != (Ref{})
		}
		if seen[ref] {
			return Ref{}, false
		}
		seen[ref] = true
		last = ref
		resolved, err := d.resolveRefRaw(ref)
		if err != nil {
			return last, true
		}
		value = resolved
	}
}

func (d *Document) annotationWithError(o Object) (Annotation, error) {
	annotation, ok := d.annotation(o)
	if ok {
		return annotation, nil
	}
	if _, valid := d.resolveIndirectChain(o); !valid {
		return Annotation{}, fmt.Errorf("playa: referenced annotation is not a dictionary")
	}
	return Annotation{}, fmt.Errorf("playa: malformed referenced annotation")
}
