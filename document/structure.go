package document

import (
	"encoding/json"
	"errors"
	"fmt"
	"iter"
	"strings"
	"sync"

	"github.com/lin-string/go-playa/contentconfig"
	"github.com/lin-string/go-playa/structureconfig"
	"github.com/lin-string/go-playa/structuredata"
)

const parentTreeCacheLimit = 32 << 20
const parentTreeErrorCacheLimit = 4096
const pageStructureCacheLimit = 32 << 20
const pageStructureErrorCacheLimit = 4096

type StructElement struct {
	data            structuredata.Element
	actualTextSet   bool
	sourceRef       Ref
	hasSourceRef    bool
	children        []StructElement
	contents        []StructureContent
	orderedContents []StructureContent
	items           []StructureItem
	itemsReady      bool
	document        *Document
	childStart      Object
	childRoleMap    map[string]string
	childPage       Ref
	childHasPage    bool
	structurePath   map[Ref]bool
	childrenReady   bool
	contentsReady   bool
	lazyState       *structElementLazyState
}

func newStructElementValue(spec structuredata.ElementSpec) StructElement {
	rawActualText, actualTextPresent := spec.Dict[Name("ActualText")]
	_, directTextString := rawActualText.(String)
	return StructElement{data: structuredata.NewElement(spec), actualTextSet: spec.ActualText != "" || (actualTextPresent && directTextString)}
}

// structElementLazyState is shared by value copies of a borrowed element.
// The public API intentionally returns StructElement values, so the state
// pointer is what preserves lazy parsing and its terminal error across copies.
type structElementLazyState struct {
	mu              sync.Mutex
	children        []StructElement
	contents        []StructureContent
	orderedContents []StructureContent
	items           []StructureItem
	ready           bool
	err             error
}

// Type returns the raw structure element type from the PDF object.
func (e StructElement) Type() string { return e.data.Type() }

// StructureType returns the normalized structure type used for role matching.
func (e StructElement) StructureType() string { return e.data.StructureType() }

// Role returns the normalized structure role.
func (e StructElement) Role() string { return e.data.Role() }

// RawRole returns the role name before RoleMap normalization.
func (e StructElement) RawRole() string { return e.data.RawRole() }

// Title returns the optional structure title.
func (e StructElement) Title() string { return e.data.Title() }

// Language returns the optional language metadata.
func (e StructElement) Language() string { return e.data.Language() }

// AlternateDescription returns the alternate description from the structure
// element's /Alt entry. It is the Go spelling of Playa's
// Element.alternate_description property.
func (e StructElement) AlternateDescription() string { return e.data.AlternateDescription() }

// ActualText returns the replacement text for this structure element.
func (e StructElement) ActualText() string { return e.data.ActualText() }

func (e StructElement) actualTextValue() (string, bool) {
	return e.ActualText(), e.actualTextSet
}

func (e StructElement) actualTextKey() string {
	if e.hasSourceRef {
		return "ref:" + e.sourceRef.String()
	}
	data, _ := json.Marshal(e.data.DictCopy())
	return "dict:" + string(data)
}

// AbbreviationExpansion returns the expansion from the structure element's
// /E entry. It is the Go spelling of Playa's
// Element.abbreviation_expansion property.
func (e StructElement) AbbreviationExpansion() string { return e.data.AbbreviationExpansion() }

// ClassName returns the normalized class name metadata.
func (e StructElement) ClassName() string { return e.data.ClassName() }

// Page returns the element's optional owning page reference.
func (e StructElement) Page() Ref { return e.data.Page() }

// HasPage reports whether Page is present.
func (e StructElement) HasPage() bool { return e.data.HasPage() }

// Parent returns the element's optional parent structure reference.
func (e StructElement) Parent() Ref { return e.data.Parent() }

// HasParent reports whether Parent is present.
func (e StructElement) HasParent() bool { return e.data.HasParent() }

// MCID returns the marked-content identifier.
func (e StructElement) MCID() int { return e.data.MCID() }

// HasMCID reports whether MCID is present.
func (e StructElement) HasMCID() bool { return e.data.HasMCID() }

// IsMCR reports whether the element is a marked-content reference/object.
func (e StructElement) IsMCR() bool { return e.data.IsMCR() }

// ObjectRef returns the optional PDF object reference.
func (e StructElement) ObjectRef() Ref { return e.data.ObjectRef() }

// HasObject reports whether ObjectRef is present.
func (e StructElement) HasObject() bool { return e.data.HasObject() }

// BBox returns the normalized explicit structure bounding box.
func (e StructElement) BBox() [4]float64 { return e.data.BBox() }

// HasBBox reports whether BBox is present.
func (e StructElement) HasBBox() bool { return e.data.HasBBox() }

const (
	StructureMarkedContent = structureconfig.MarkedContent
	StructureObject        = structureconfig.Object
)

// StructureContent is a marked-content or object-reference child of a
// structure element. It mirrors Playa's ContentItem and ContentObject split
// while keeping the raw PDF dictionary available for callers that need it.
type StructureContent struct {
	data structuredata.Content
}

func newStructureContent(spec structuredata.ContentSpec) StructureContent {
	return StructureContent{data: structuredata.NewContent(spec)}
}

// StructureItem is one direct value from a structure element's /K entry.
// It is either a nested structure element or a marked-content/object
// reference. The value is borrowed while yielded by ItemsSeq; call one of
// the copy methods when it must outlive the traversal.
type StructureItem struct {
	data    structuredata.Item
	element *StructElement
}

func newStructureItem(kind structureconfig.ItemKind, content StructureContent) StructureItem {
	return StructureItem{data: structuredata.NewItem(structuredata.ItemSpec{Kind: kind, Content: content.data})}
}

func newStructureElementItem(element *StructElement) StructureItem {
	return StructureItem{data: structuredata.NewItem(structuredata.ItemSpec{Kind: structureconfig.ItemElement}), element: element}
}

// Kind identifies the variant carried by this item.
func (i StructureItem) Kind() structureconfig.ItemKind { return i.data.Kind() }

// ElementBorrowed returns the nested structure element without copying it.
// The pointer is borrowed from ItemsSeq; use ElementCopy or Finalize when it
// must outlive the current traversal.
func (i StructureItem) ElementBorrowed() (*StructElement, bool) {
	if i.Kind() != structureconfig.ItemElement || i.element == nil {
		return nil, false
	}
	return i.element, true
}

// ContentBorrowed returns a shallow value view of the marked-content or
// object-reference payload. Use ContentCopy or Finalize for independent
// ownership.
func (i StructureItem) ContentBorrowed() (StructureContent, bool) {
	if i.Kind() != structureconfig.ItemMarkedContent && i.Kind() != structureconfig.ItemObject {
		return StructureContent{}, false
	}
	content, ok := i.data.ContentBorrowed()
	if !ok {
		return StructureContent{}, false
	}
	return StructureContent{data: content}, true
}

// ElementCopy returns an independent nested structure element, when present.
func (i StructureItem) ElementCopy() (*StructElement, bool) {
	if i.Kind() != structureconfig.ItemElement || i.element == nil {
		return nil, false
	}
	clone := i.element.Finalize()
	return &clone, true
}

// ContentCopy returns an independent marked-content/object reference, when present.
func (i StructureItem) ContentCopy() (StructureContent, bool) {
	if i.Kind() != structureconfig.ItemMarkedContent && i.Kind() != structureconfig.ItemObject {
		return StructureContent{}, false
	}
	content, ok := i.data.ContentCopy()
	if !ok {
		return StructureContent{}, false
	}
	return StructureContent{data: content}, true
}

// Finalize returns an independent stable item snapshot.
func (i StructureItem) Finalize() StructureItem {
	clone, _ := i.FinalizeWithError()
	return clone
}

// FinalizeWithError returns an independent item and reports deferred child
// materialization failures from nested structure elements.
func (i StructureItem) FinalizeWithError() (StructureItem, error) {
	clone := i
	clone.data = i.data.Finalize()
	if i.element != nil {
		element, err := i.element.FinalizeWithError()
		if err != nil {
			return StructureItem{}, err
		}
		clone.element = &element
	}
	return clone, nil
}

// MarshalJSON preserves the variant and its read-only projections.
func (i StructureItem) MarshalJSON() ([]byte, error) {
	var element *StructElement
	if i.element != nil {
		copy := i.element.Finalize()
		element = &copy
	}
	content, _ := i.ContentCopy()
	return json.Marshal(struct {
		Kind    structureconfig.ItemKind
		Element *StructElement
		Content StructureContent
	}{Kind: i.Kind(), Element: element, Content: content})
}

// Kind returns whether this item references marked content or a PDF object.
func (c StructureContent) Kind() structureconfig.ContentKind { return c.data.Kind() }

// MCID returns the marked-content identifier, when present.
func (c StructureContent) MCID() int { return c.data.MCID() }

// HasMCID reports whether MCID is present.
func (c StructureContent) HasMCID() bool { return c.data.HasMCID() }

// Page returns the owning page reference, when present.
func (c StructureContent) Page() Ref { return c.data.Page() }

// HasPage reports whether Page identifies an owning page.
func (c StructureContent) HasPage() bool { return c.data.HasPage() }

// HasStream reports whether the item carries a marked-content stream.
func (c StructureContent) HasStream() bool { return c.data.HasStream() }

// ObjectRef returns the referenced PDF object, when present.
func (c StructureContent) ObjectRef() Ref { return c.data.ObjectRef() }

// HasObject reports whether ObjectRef identifies a PDF object.
func (c StructureContent) HasObject() bool { return c.data.HasObject() }

// ContentSeq lazily yields page content objects belonging to this marked
// content item. Object-reference items do not contain page content objects.
func (c StructureContent) ContentSeq(d *Document) iter.Seq2[ContentObject, error] {
	return func(yield func(ContentObject, error) bool) {
		if d == nil {
			yield(ContentObject{}, errNilDocument)
			return
		}
		if !c.HasPage() || !c.HasMCID() || c.Kind() != StructureMarkedContent {
			return
		}
		page, err := c.PageObject(d)
		if err != nil {
			yield(ContentObject{}, err)
			return
		}
		for object, err := range page.Flatten(d, contentconfig.Options{Filter: FilterAll}) {
			if err != nil {
				if !yield(ContentObject{}, err) {
					return
				}
				return
			}
			if mcid, ok := object.MCIDValue(); !ok || mcid != c.MCID() {
				continue
			}
			if !yield(object, nil) {
				return
			}
		}
	}
}

// Text returns Unicode text contained in this marked content item. An
// ActualText value on the nearest marked section replaces painted text.
func (c StructureContent) Text(d *Document) (string, error) {
	var out strings.Builder
	for object, err := range c.ContentSeq(d) {
		if err != nil {
			return "", err
		}
		if object.text == nil {
			continue
		}
		value := object.text.Chars()
		if context := object.MarkedContext(); context != nil {
			if context.HasProperty(Name("ActualText")) {
				value = context.ActualText()
			}
		}
		out.WriteString(strings.ReplaceAll(value, "\u00ad", ""))
	}
	return out.String(), nil
}

// BBoxValue returns the smallest device-space box containing this content
// item's page objects.
func (c StructureContent) BBoxValue(d *Document) ([4]float64, error) {
	if c.Kind() == StructureObject && c.HasObject() && d != nil {
		page, err := c.PageObject(d)
		if err != nil {
			return [4]float64{}, err
		}
		object, _, err := c.ObjectWithError(d)
		if err != nil {
			return [4]float64{}, err
		}
		var properties Dict
		switch object := object.(type) {
		case Dict:
			properties = object
		case Stream:
			properties = object.DictBorrowed()
		}
		for _, key := range []Name{Name("BBox"), Name("Rect")} {
			if box, ok := structureRect(d, properties[key]); ok {
				transformed, ok := transformBBox(page.MatrixIn(d, page.coordinateSpace(d)), box)
				if ok {
					return transformed, nil
				}
				return [4]float64{}, nil
			}
		}
		return [4]float64{}, nil
	}
	var box [4]float64
	found := false
	for object, err := range c.ContentSeq(d) {
		if err != nil {
			return [4]float64{}, err
		}
		if value, ok := object.BBoxValue(); ok {
			box, found = unionBBox(box, found, value)
		}
	}
	return box, nil
}

// PageObject resolves the content item's optional /Pg reference to its page.
func (c StructureContent) PageObject(d *Document) (Page, error) {
	if d == nil {
		return Page{}, errNilDocument
	}
	if !c.HasPage() {
		return Page{}, ErrPageNotFound
	}
	return d.PageByRef(c.Page())
}

// Object resolves the content item's optional /Obj reference.
func (c StructureContent) Object(d *Document) (Object, bool) {
	object, ok, _ := c.ObjectWithError(d)
	return object, ok
}

// ObjectWithError resolves the optional /Obj reference and reports malformed
// or unresolved indirect objects without changing the compatibility Object API.
func (c StructureContent) ObjectWithError(d *Document) (Object, bool, error) {
	if d == nil {
		return nil, false, errNilDocument
	}
	if !c.HasObject() {
		return nil, false, nil
	}
	object, resolved := d.resolveIndirectChain(c.ObjectRef())
	if !resolved {
		return nil, false, fmt.Errorf("playa: structure content object could not be resolved")
	}
	return cloneGraphicsObject(object), true, nil
}

// StructureIndex keeps the hierarchical structure tree and an MCID lookup
// table for consumers that need to join structure semantics with page content.
type StructureIndex struct {
	roots  []StructElement
	byMCID map[int][]StructElement
}

// RootsCopy returns independent root structure elements.
func (s StructureIndex) RootsCopy() []StructElement { return finalizeStructElements(s.roots) }

// RootsCopyWithError returns independent root elements and reports deferred
// structure-child or content failures.
func (s StructureIndex) RootsCopyWithError() ([]StructElement, error) {
	return finalizeStructElementsWithError(s.roots)
}

// ByMCIDCopy returns an independent MCID-to-elements index.
func (s StructureIndex) ByMCIDCopy() map[int][]StructElement {
	if s.byMCID == nil {
		return nil
	}
	out := make(map[int][]StructElement, len(s.byMCID))
	for mcid, elements := range s.byMCID {
		out[mcid] = finalizeStructElements(elements)
	}
	return out
}

// ByMCIDCopyWithError returns an independent MCID index and reports deferred
// structure-child or content failures.
func (s StructureIndex) ByMCIDCopyWithError() (map[int][]StructElement, error) {
	if s.byMCID == nil {
		return nil, nil
	}
	out := make(map[int][]StructElement, len(s.byMCID))
	for mcid, elements := range s.byMCID {
		copy, err := finalizeStructElementsWithError(elements)
		if err != nil {
			return nil, err
		}
		out[mcid] = copy
	}
	return out, nil
}

// Finalize returns an independent snapshot of the structure index.
func (s StructureIndex) Finalize() StructureIndex {
	return StructureIndex{roots: finalizeStructElements(s.roots), byMCID: s.ByMCIDCopy()}
}

// FinalizeWithError returns an independent structure index and reports
// deferred structure-child or content failures.
func (s StructureIndex) FinalizeWithError() (StructureIndex, error) {
	roots, err := s.RootsCopyWithError()
	if err != nil {
		return StructureIndex{}, err
	}
	byMCID, err := s.ByMCIDCopyWithError()
	if err != nil {
		return StructureIndex{}, err
	}
	return StructureIndex{roots: roots, byMCID: byMCID}, nil
}

// PageStructureEntry associates a page MCID slot with its logical structure
// element. Element is nil for unused ParentTree slots.
type PageStructureEntry struct {
	index    int
	element  *StructElement
	elements []StructElement
}

// Index returns the zero-based ParentTree slot index.
func (e PageStructureEntry) Index() int { return e.index }

// ElementBorrowed returns the primary structure element without copying it.
// The pointer is borrowed from the ParentTree traversal; use ElementCopy or
// Finalize when it must outlive the current entry view.
func (e PageStructureEntry) ElementBorrowed() *StructElement { return e.element }

// ElementsSeq yields borrowed structure elements in this ParentTree slot.
// Recoverable multi-element slots preserve every element in source order.
func (e PageStructureEntry) ElementsSeq() iter.Seq[StructElement] {
	return func(yield func(StructElement) bool) {
		if len(e.elements) > 0 {
			for _, element := range e.elements {
				if !yield(element) {
					return
				}
			}
			return
		}
		if e.element != nil {
			yield(*e.element)
		}
	}
}

// ElementCopy returns an independent primary structure element, when present.
func (e PageStructureEntry) ElementCopy() *StructElement {
	if e.element == nil {
		return nil
	}
	clone := e.element.Finalize()
	return &clone
}

// ElementCopyWithError returns an independent primary element and reports
// deferred structure-child or content failures.
func (e PageStructureEntry) ElementCopyWithError() (*StructElement, error) {
	if e.element == nil {
		return nil, nil
	}
	clone, err := e.element.FinalizeWithError()
	if err != nil {
		return nil, err
	}
	return &clone, nil
}

// ElementsCopy returns independent structure elements for this ParentTree slot.
func (e PageStructureEntry) ElementsCopy() []StructElement { return finalizeStructElements(e.elements) }

// ElementsCopyWithError returns independent elements and reports deferred
// structure-child or content failures.
func (e PageStructureEntry) ElementsCopyWithError() ([]StructElement, error) {
	return finalizeStructElementsWithError(e.elements)
}

// Finalize returns an independent snapshot of this ParentTree slot.
func (e PageStructureEntry) Finalize() PageStructureEntry {
	clone := PageStructureEntry{index: e.index, elements: finalizeStructElements(e.elements)}
	if e.element != nil {
		element := e.element.Finalize()
		clone.element = &element
	}
	return clone
}

// FinalizeWithError returns an independent page ParentTree entry and reports
// deferred structure-child or content failures.
func (e PageStructureEntry) FinalizeWithError() (PageStructureEntry, error) {
	elements, err := e.ElementsCopyWithError()
	if err != nil {
		return PageStructureEntry{}, err
	}
	element, err := e.ElementCopyWithError()
	if err != nil {
		return PageStructureEntry{}, err
	}
	return PageStructureEntry{index: e.index, element: element, elements: elements}, nil
}

// PageStructure is the logical structure view associated with one page.
// Elements follows ParentTree slot order and ByMCID provides direct access to
// the structure elements associated with a page MCID.
type PageStructure struct {
	// slots preserves the ParentTree sequence, including empty slots. A slot
	// can contain more than one direct element for malformed-but-recoverable
	// ParentTree arrays; the public view keeps all of them without flattening
	// the sequence index.
	slots    [][]StructElement
	elements []StructElement
	byMCID   map[int][]StructElement
}

func clonePageStructure(value PageStructure) PageStructure {
	clone := PageStructure{elements: finalizeStructElements(value.elements)}
	if value.slots != nil {
		clone.slots = make([][]StructElement, len(value.slots))
		for i, slot := range value.slots {
			clone.slots[i] = finalizeStructElements(slot)
		}
	}
	if value.byMCID == nil {
		return clone
	}
	clone.byMCID = make(map[int][]StructElement, len(value.byMCID))
	for mcid, elements := range value.byMCID {
		clone.byMCID[mcid] = finalizeStructElements(elements)
	}
	return clone
}

func structureContentCacheSize(content StructureContent, depth int) int {
	if depth <= 0 {
		return 128
	}
	size := addCacheSize(128, nameTreeObjectSize(content.DictCopy(), depth-1))
	if content.HasStream() {
		size = addCacheSize(size, nameTreeObjectSize(content.StreamCopy(), depth-1))
	}
	return size
}

func structElementCacheSize(element StructElement, depth int) int {
	if depth <= 0 {
		return 512
	}
	size := 512 + len(element.Type()) + len(element.StructureType()) + len(element.Role()) + len(element.RawRole()) +
		len(element.Title()) + len(element.Language()) + len(element.AlternateDescription()) + len(element.ActualText()) +
		len(element.AbbreviationExpansion()) + len(element.ClassName())
	size = addCacheSize(size, nameTreeObjectSize(element.AttributesCopy(), depth-1))
	size = addCacheSize(size, nameTreeObjectSize(element.DictCopy(), depth-1))
	size = addCacheSize(size, nameTreeObjectSize(element.ObjectCopy(), depth-1))
	size = addCacheSize(size, cacheMulSize(len(element.items), 16))
	for role, mapped := range element.childRoleMap {
		size = addCacheSize(size, len(role)+len(mapped)+32)
	}
	for _, child := range element.children {
		size = addCacheSize(size, structElementCacheSize(child, depth-1))
	}
	for _, content := range element.contents {
		size = addCacheSize(size, structureContentCacheSize(content, depth-1))
	}
	for _, content := range element.orderedContents {
		size = addCacheSize(size, structureContentCacheSize(content, depth-1))
	}
	return size
}

func pageStructureCacheSize(structure PageStructure) int {
	size := 128
	size = addCacheSize(size, cacheMulSize(len(structure.slots), 16))
	size = addCacheSize(size, cacheMulSize(len(structure.elements), 16))
	size = addCacheSize(size, cacheMulSize(len(structure.byMCID), 32))
	for _, element := range structure.elements {
		size = addCacheSize(size, structElementCacheSize(element, 8))
	}
	// The MCID index contains value copies of the element headers and keeps
	// their backing slices reachable, so count it conservatively as retained
	// structure state as well.
	for _, elements := range structure.byMCID {
		size = addCacheSize(size, 16)
		for _, element := range elements {
			size = addCacheSize(size, structElementCacheSize(element, 8))
		}
	}
	return size
}

// ElementsCopy returns independent page structure elements in ParentTree order.
func (s PageStructure) ElementsCopy() []StructElement { return finalizeStructElements(s.elements) }

// ElementsCopyWithError returns independent page structure elements and
// reports deferred structure-child or content failures.
func (s PageStructure) ElementsCopyWithError() ([]StructElement, error) {
	return finalizeStructElementsWithError(s.elements)
}

// ByMCIDCopy returns an independent MCID-to-elements index.
func (s PageStructure) ByMCIDCopy() map[int][]StructElement {
	if s.byMCID == nil {
		return nil
	}
	out := make(map[int][]StructElement, len(s.byMCID))
	for mcid, elements := range s.byMCID {
		out[mcid] = finalizeStructElements(elements)
	}
	return out
}

// ByMCIDCopyWithError returns an independent MCID index and reports deferred
// structure-child or content failures.
func (s PageStructure) ByMCIDCopyWithError() (map[int][]StructElement, error) {
	if s.byMCID == nil {
		return nil, nil
	}
	out := make(map[int][]StructElement, len(s.byMCID))
	for mcid, elements := range s.byMCID {
		copy, err := finalizeStructElementsWithError(elements)
		if err != nil {
			return nil, err
		}
		out[mcid] = copy
	}
	return out, nil
}

// Finalize returns an independent snapshot of the page structure.
func (s PageStructure) Finalize() PageStructure { return clonePageStructure(s) }

// FinalizeWithError returns an independent page structure and reports
// deferred structure-child or content failures.
func (s PageStructure) FinalizeWithError() (PageStructure, error) {
	elements, err := s.ElementsCopyWithError()
	if err != nil {
		return PageStructure{}, err
	}
	byMCID, err := s.ByMCIDCopyWithError()
	if err != nil {
		return PageStructure{}, err
	}
	var slots [][]StructElement
	if s.slots != nil {
		slots = make([][]StructElement, len(s.slots))
		for i, slot := range s.slots {
			copy, err := finalizeStructElementsWithError(slot)
			if err != nil {
				return PageStructure{}, err
			}
			slots[i] = copy
		}
	}
	return PageStructure{slots: slots, elements: elements, byMCID: byMCID}, nil
}

func cloneStructElements(elements []StructElement) []StructElement {
	if elements == nil {
		return nil
	}
	clone := make([]StructElement, len(elements))
	for i, element := range elements {
		clone[i] = element
		clone[i].lazyState = nil
		clone[i].data = element.data.Finalize()
		clone[i].children = cloneStructElements(element.children)
		clone[i].contents = cloneStructureContents(element.contents)
		clone[i].orderedContents = cloneStructureContents(element.orderedContents)
		clone[i].items = cloneStructureItems(element.items)
		if element.childRoleMap != nil {
			clone[i].childRoleMap = make(map[string]string, len(element.childRoleMap))
			for role, mapped := range element.childRoleMap {
				clone[i].childRoleMap[role] = mapped
			}
		}
		clone[i].structurePath = cloneStructurePath(element.structurePath)
	}
	return clone
}

func cloneStructureItems(items []StructureItem) []StructureItem {
	if items == nil {
		return nil
	}
	clone := make([]StructureItem, len(items))
	for i, item := range items {
		clone[i] = item
		clone[i].data = item.data.Finalize()
		if item.element != nil {
			element := cloneStructElement(*item.element)
			clone[i].element = &element
		}
	}
	return clone
}

func finalizeStructElementsWithError(elements []StructElement) ([]StructElement, error) {
	if elements == nil {
		return nil, nil
	}
	clone := make([]StructElement, len(elements))
	for i, element := range elements {
		value, err := element.FinalizeWithError()
		if err != nil {
			return nil, err
		}
		clone[i] = value
	}
	return clone, nil
}

func cloneStructElement(element StructElement) StructElement {
	return cloneStructElements([]StructElement{element})[0]
}

func cloneStructurePath(source map[Ref]bool) map[Ref]bool {
	if source == nil {
		return nil
	}
	path := make(map[Ref]bool, len(source))
	for ref, present := range source {
		path[ref] = present
	}
	return path
}

func finalizeStructElements(elements []StructElement) []StructElement {
	if elements == nil {
		return nil
	}
	clone := make([]StructElement, len(elements))
	for i, element := range elements {
		clone[i] = finalizeStructElement(element)
	}
	return clone
}

// ChildrenCopy materializes lazy descendants and returns an independent tree.
func (e StructElement) ChildrenCopy() ([]StructElement, error) {
	if err := e.materializeChildrenErr(); err != nil {
		return nil, err
	}
	return finalizeStructElements(e.children), nil
}

// ContentsCopy materializes structure content items and returns independent values.
func (e StructElement) ContentsCopy() ([]StructureContent, error) {
	if err := e.materializeChildrenErr(); err != nil {
		return nil, err
	}
	return cloneStructureContents(e.contents), nil
}

// AttributesCopy returns an independent copy of structure attributes.
func (e StructElement) AttributesCopy() Dict { return e.data.AttributesCopy() }

// DictCopy returns an independent copy of the source structure dictionary.
func (e StructElement) DictCopy() Dict { return e.data.DictCopy() }

// ObjectCopy returns an independent copy of an optional structure object.
func (e StructElement) ObjectCopy() Object { return e.data.ObjectCopy() }

// DictCopy returns an independent copy of the source structure-content dictionary.
func (c StructureContent) DictCopy() Dict { return c.data.DictCopy() }

// StreamCopy returns an independent copy of the marked-content stream.
func (c StructureContent) StreamCopy() Stream {
	return c.data.StreamCopy()
}

func (e StructElement) MarshalJSON() ([]byte, error) {
	if err := e.materializeChildrenErr(); err != nil {
		return nil, err
	}
	return json.Marshal(&struct {
		Type          string             `json:"Type"`
		StructureType string             `json:"StructureType"`
		Role          string             `json:"Role"`
		RawRole       string             `json:"RawRole"`
		Title         string             `json:"Title"`
		Language      string             `json:"Language"`
		Alt           string             `json:"Alt"`
		ActualText    string             `json:"ActualText"`
		Abbreviation  string             `json:"Abbreviation"`
		ClassName     string             `json:"ClassName"`
		Page          Ref                `json:"Page"`
		HasPage       bool               `json:"HasPage"`
		Parent        Ref                `json:"Parent"`
		HasParent     bool               `json:"HasParent"`
		MCID          int                `json:"MCID"`
		HasMCID       bool               `json:"HasMCID"`
		IsMCR         bool               `json:"IsMCR"`
		ObjectRef     Ref                `json:"ObjectRef"`
		HasObject     bool               `json:"HasObject"`
		BBox          [4]float64         `json:"BBox"`
		HasBBox       bool               `json:"HasBBox"`
		Attributes    Dict               `json:"Attributes"`
		Dict          Dict               `json:"Dict"`
		Object        Object             `json:"Object"`
		Children      []StructElement    `json:"Children"`
		Contents      []StructureContent `json:"Contents"`
	}{
		Type: e.Type(), StructureType: e.StructureType(), Role: e.Role(), RawRole: e.RawRole(),
		Title: e.Title(), Language: e.Language(), Alt: e.AlternateDescription(), ActualText: e.ActualText(),
		Abbreviation: e.AbbreviationExpansion(), ClassName: e.ClassName(), Page: e.Page(), HasPage: e.HasPage(),
		Parent: e.Parent(), HasParent: e.HasParent(), MCID: e.MCID(), HasMCID: e.HasMCID(),
		IsMCR: e.IsMCR(), ObjectRef: e.ObjectRef(), HasObject: e.HasObject(), BBox: e.BBox(),
		HasBBox: e.HasBBox(), Attributes: e.AttributesCopy(), Dict: e.DictCopy(), Object: e.ObjectCopy(),
		Children: e.children, Contents: e.contents,
	})
}

func cloneStructureContents(contents []StructureContent) []StructureContent {
	if contents == nil {
		return nil
	}
	clone := make([]StructureContent, len(contents))
	for i, content := range contents {
		clone[i] = StructureContent{data: content.data.Finalize()}
	}
	return clone
}

// Finalize returns an independent snapshot of this structure content item.
func (c StructureContent) Finalize() StructureContent {
	clone := cloneStructureContents([]StructureContent{c})
	return clone[0]
}

func (c StructureContent) MarshalJSON() ([]byte, error) {
	type projection struct {
		Kind      structureconfig.ContentKind
		MCID      int
		HasMCID   bool
		Page      Ref
		HasPage   bool
		Stream    Stream
		HasStream bool
		ObjectRef Ref
		HasObject bool
		Dict      Dict
	}
	return json.Marshal(projection{Kind: c.Kind(), MCID: c.MCID(), HasMCID: c.HasMCID(), Page: c.Page(), HasPage: c.HasPage(), Stream: c.StreamCopy(), HasStream: c.HasStream(), ObjectRef: c.ObjectRef(), HasObject: c.HasObject(), Dict: c.DictCopy()})
}

// Structure builds the page's ParentTree structure view.
func (p Page) Structure(d *Document) (PageStructure, error) {
	structure := PageStructure{byMCID: map[int][]StructElement{}}
	if d == nil {
		return structure, errNilDocument
	}
	if p.ref != (Ref{}) {
		d.cacheMu.RLock()
		if err, ok := d.pageStructureErrors[p.ref]; ok {
			d.cacheMu.RUnlock()
			return PageStructure{}, err
		}
		ready := d.pageStructureReady[p.ref]
		cached := d.pageStructureCache[p.ref]
		d.cacheMu.RUnlock()
		if ready {
			return cached, nil
		}
	}
	cacheError := func(err error) (PageStructure, error) {
		if p.ref == (Ref{}) {
			return PageStructure{}, err
		}
		d.cacheMu.Lock()
		defer d.cacheMu.Unlock()
		if cached, ok := d.pageStructureErrors[p.ref]; ok {
			return PageStructure{}, cached
		}
		if len(err.Error()) <= d.cacheLimits().PageStructureErrorBytes && cacheFits(d.pageStructureErrorBytes, len(err.Error()), d.cacheLimits().PageStructureErrorBytes) {
			if d.pageStructureErrors == nil {
				d.pageStructureErrors = map[Ref]error{}
			}
			d.pageStructureErrors[p.ref] = err
			d.pageStructureErrorBytes += len(err.Error())
		}
		return PageStructure{}, err
	}
	for entry, err := range p.StructureSeq(d) {
		if err != nil {
			return cacheError(err)
		}
		elements := entry.elements
		if len(elements) == 0 && entry.element != nil {
			elements = []StructElement{*entry.element}
		}
		structure.slots = append(structure.slots, elements)
		if len(elements) == 0 {
			continue
		}
		structure.elements = append(structure.elements, elements...)
		structure.byMCID[entry.index] = append(structure.byMCID[entry.index], elements...)
	}
	if p.ref != (Ref{}) {
		d.cacheMu.Lock()
		defer d.cacheMu.Unlock()
		if d.pageStructureReady[p.ref] {
			return d.pageStructureCache[p.ref], nil
		}
		cacheBytes := pageStructureCacheSize(structure)
		limit := d.cacheLimits().PageStructureBytes
		if cacheFits(d.pageStructureCacheBytes, cacheBytes, limit) {
			if d.pageStructureCache == nil {
				d.pageStructureCache = map[Ref]PageStructure{}
			}
			if d.pageStructureReady == nil {
				d.pageStructureReady = map[Ref]bool{}
			}
			d.pageStructureCache[p.ref] = structure
			d.pageStructureCacheBytes += cacheBytes
			d.pageStructureReady[p.ref] = true
		}
	}
	return structure, nil
}

// Len returns the number of ParentTree slots, including empty slots.
func (s PageStructure) Len() int { return len(s.slots) }

// At returns the elements in one zero-based ParentTree slot. An empty or
// out-of-range slot returns nil. The returned elements are independent
// snapshots, matching Go's value-oriented replacement for Playa indexing.
func (s PageStructure) At(index int) []StructElement {
	if index < 0 || index >= len(s.slots) {
		return nil
	}
	return finalizeStructElements(s.slots[index])
}

// ByMCID returns the structure elements associated with a page MCID.
func (s PageStructure) ByMCID(mcid int) []StructElement {
	return finalizeStructElements(s.byMCID[mcid])
}

// FindAllSeq yields borrowed page structure elements in ParentTree order. Call
// StructElement.Finalize when an independent snapshot is required.
func (s PageStructure) FindAllSeq(role string) iter.Seq2[StructElement, error] {
	return func(yield func(StructElement, error) bool) {
		for _, element := range s.elements {
			if role != "" && element.Role() != role {
				continue
			}
			if !yield(element, nil) {
				return
			}
		}
	}
}

// Find returns the first page structure element matching role.
func (s PageStructure) Find(role string) (StructElement, bool) {
	for element, err := range s.FindAllSeq(role) {
		if err == nil {
			return element.Finalize(), true
		}
	}
	return StructElement{}, false
}

// Text returns the Unicode text associated with this element's marked-content
// descendants. ActualText replaces the painted text when it is present.
func (e StructElement) Text(d *Document) (string, error) {
	if d == nil {
		return "", errNilDocument
	}
	if actualText, present := e.actualTextValue(); present {
		return strings.ReplaceAll(actualText, "\u00ad", ""), nil
	}
	var out strings.Builder
	textCache := map[Ref][]TextObject{}
	markedCache := map[Ref]MarkedContentIndex{}
	for _, content := range e.orderedContentsOrDirect() {
		if !content.HasMCID() {
			continue
		}
		page, err, ok := structureContentPage(d, content, e)
		if err != nil {
			return "", err
		}
		if !ok {
			continue
		}
		var texts []TextObject
		if page.ref != (Ref{}) {
			var cached bool
			texts, cached = textCache[page.ref]
			if !cached {
				texts, err = d.PageTextExpanded(page)
				if err != nil {
					return "", err
				}
				textCache[page.ref] = texts
			}
		} else {
			texts, err = d.PageTextExpanded(page)
			if err != nil {
				return "", err
			}
		}
		actualText := ""
		if page.ref != (Ref{}) {
			index, cached := markedCache[page.ref]
			if !cached {
				index, err = d.PageMarkedContentIndex(page)
				if err != nil {
					return "", err
				}
				markedCache[page.ref] = index
			}
			for _, marked := range index.byMCID[content.MCID()] {
				if marked.ActualText() != "" {
					actualText = marked.ActualText()
				}
				break
			}
		} else {
			for marked, err := range d.PageMarkedContentByMCIDSeq(page, content.MCID()) {
				if err != nil {
					return "", err
				}
				if marked.ActualText() != "" {
					actualText = marked.ActualText()
				}
				break
			}
		}
		for _, text := range texts {
			if !text.HasMCID() || text.MCID() != content.MCID() {
				continue
			}
			value := text.Text()
			if actualText != "" {
				value = actualText
			}
			out.WriteString(strings.ReplaceAll(value, "\u00ad", ""))
		}
	}
	return out.String(), nil
}

// PageObject resolves the element's /Pg reference to its owning page.
// The raw Page reference remains available for callers that do not need
// document traversal.
func (e StructElement) PageObject(d *Document) (Page, error) {
	if d == nil {
		return Page{}, errNilDocument
	}
	if !e.HasPage() {
		return Page{}, ErrPageNotFound
	}
	return d.PageByRef(e.Page())
}

// Object resolves an OBJ structure element's /Obj reference without
// materializing a page content interpretation. The returned PDF object is an
// independent read-only snapshot; callers that need a typed content model can
// resolve it through the owning page APIs.
func (e StructElement) Object(d *Document) (Object, bool) {
	object, ok, _ := e.ObjectWithError(d)
	return object, ok
}

// ObjectWithError resolves an OBJ structure element's /Obj reference and
// reports malformed or unresolved indirect objects without changing the
// compatibility Object API.
func (e StructElement) ObjectWithError(d *Document) (Object, bool, error) {
	if d == nil {
		return nil, false, errNilDocument
	}
	if !e.HasObject() {
		return nil, false, nil
	}
	object, resolved := d.resolveIndirectChain(e.ObjectRef())
	if !resolved {
		return nil, false, fmt.Errorf("playa: structure element object could not be resolved")
	}
	return cloneGraphicsObject(object), true, nil
}

// ParentElement resolves the element's /P reference when it points to
// another structure element. A StructTreeRoot parent has no corresponding
// element value and returns nil. Call Finalize when an independent materialized
// snapshot is required.
func (e StructElement) ParentElement(d *Document) *StructElement {
	parent, _ := e.ParentElementWithError(d)
	return parent
}

// ParentElementWithError resolves the element's /P reference and reports
// malformed or unresolved parent objects without changing ParentElement's
// compatibility behavior.
func (e StructElement) ParentElementWithError(d *Document) (*StructElement, error) {
	if d == nil {
		return nil, errNilDocument
	}
	if !e.HasParent() {
		return nil, nil
	}
	parentValue, resolved := d.resolveIndirectChain(e.Parent())
	if !resolved {
		return nil, fmt.Errorf("playa: structure element parent could not be resolved")
	}
	parent, ok := parentValue.(Dict)
	if !ok {
		return nil, fmt.Errorf("playa: structure element parent is not a dictionary")
	}
	typeValue, typeResolved := d.resolveIndirectChain(parent[Name("Type")])
	if _, present := parent[Name("Type")]; present && !typeResolved {
		return nil, fmt.Errorf("playa: structure element parent Type could not be resolved")
	}
	typ, _ := typeValue.(Name)
	if typ != Name("StructElem") && typ != Name("") {
		return nil, nil
	}
	roleMap := d.structureRoleMap()
	seen := map[Ref]bool{}
	for _, err := range d.structElementChildrenSeq(e.Parent(), seen, roleMap) {
		if err != nil {
			return nil, err
		}
	}
	for _, err := range d.structElementContentsSeq(e.Parent(), Ref{}, false, map[Ref]bool{}) {
		if err != nil {
			return nil, err
		}
	}
	elements := d.structKidsLazy(e.Parent(), map[Ref]bool{}, roleMap)
	if len(elements) == 0 {
		return nil, nil
	}
	element := elements[0]
	return &element, nil
}

// BBoxValue returns an explicit structure BBox or the union of descendant
// page-content object boxes when the PDF does not provide one.
func (e StructElement) BBoxValue(d *Document) ([4]float64, error) {
	if d == nil {
		return [4]float64{}, errNilDocument
	}
	if e.HasBBox() {
		return e.BBox(), nil
	}
	var box [4]float64
	found := false
	objectCache := map[Ref][]ContentObject{}
	for _, content := range e.orderedContentsOrDirect() {
		if !content.HasMCID() {
			continue
		}
		page, err, ok := structureContentPage(d, content, e)
		if err != nil {
			return [4]float64{}, err
		}
		if !ok {
			continue
		}
		var objects []ContentObject
		if page.ref != (Ref{}) {
			var cached bool
			objects, cached = objectCache[page.ref]
			if !cached {
				for object, interpErr := range page.Interp(d, contentconfig.Options{Filter: FilterAll}) {
					if interpErr != nil {
						return [4]float64{}, interpErr
					}
					objects = append(objects, object)
				}
				objectCache[page.ref] = objects
			}
		} else {
			for object, interpErr := range page.Interp(d, contentconfig.Options{Filter: FilterAll}) {
				if interpErr != nil {
					return [4]float64{}, interpErr
				}
				objects = append(objects, object)
			}
		}
		for _, object := range objects {
			var objectBox [4]float64
			valid := false
			if object.text != nil && object.text.HasMCID() && object.text.MCID() == content.MCID() {
				objectBox, valid = object.text.BBox(), object.text.Len() > 0
			}
			if object.path != nil && object.path.HasMCID() && object.path.MCID() == content.MCID() {
				objectBox, valid = object.path.BBox(), true
			}
			if object.image != nil && object.image.hasMCID && object.image.mcid == content.MCID() {
				objectBox, valid = object.image.bbox, true
			}
			if valid {
				box, found = unionBBox(box, found, objectBox)
			}
		}
	}
	return box, nil
}

func (e StructElement) orderedContentsOrDirect() []StructureContent {
	if len(e.orderedContents) > 0 {
		return e.orderedContents
	}
	return e.contents
}

func structureContentPage(d *Document, content StructureContent, element StructElement) (Page, error, bool) {
	ref, ok := content.Page(), content.HasPage()
	if !ok {
		ref, ok = element.Page(), element.HasPage()
	}
	if !ok {
		return Page{}, nil, false
	}
	page, err := d.PageByRef(ref)
	if errors.Is(err, ErrPageNotFound) {
		return Page{}, nil, false
	}
	if err != nil {
		return Page{}, err, false
	}
	return page, nil, true
}

func unionBBox(current [4]float64, found bool, next [4]float64) ([4]float64, bool) {
	if !found {
		return next, true
	}
	if next[0] < current[0] {
		current[0] = next[0]
	}
	if next[1] < current[1] {
		current[1] = next[1]
	}
	if next[2] > current[2] {
		current[2] = next[2]
	}
	if next[3] > current[3] {
		current[3] = next[3]
	}
	return current, true
}

func (p Page) structureSeq(d *Document) iter.Seq2[PageStructureEntry, error] {
	return func(yield func(PageStructureEntry, error) bool) {
		key, present, keyErr := p.ParentKeyWithError(d)
		if keyErr != nil {
			yield(PageStructureEntry{}, keyErr)
			return
		}
		if !present {
			return
		}
		value, ok, err := d.parentTreeValueChecked(key)
		if err != nil {
			yield(PageStructureEntry{}, err)
			return
		}
		if !ok {
			return
		}
		resolvedParents, parentsResolved := d.resolveIndirectChain(value)
		if !parentsResolved {
			yield(PageStructureEntry{}, fmt.Errorf("playa: ParentTree value could not be resolved"))
			return
		}
		parents, ok := resolvedParents.(Array)
		if !ok {
			yield(PageStructureEntry{}, fmt.Errorf("playa: ParentTree value is not an array"))
			return
		}
		roleMap, roleMapErr := d.structureRoleMapChecked()
		if roleMapErr != nil {
			yield(PageStructureEntry{}, roleMapErr)
			return
		}
		for index, parent := range parents {
			entry := PageStructureEntry{index: index}
			resolvedParent, parentResolved := d.resolveIndirectChain(parent)
			if !parentResolved {
				yield(PageStructureEntry{}, fmt.Errorf("playa: ParentTree value could not be resolved"))
				return
			}
			if _, isNull := resolvedParent.(Null); !isNull {
				// The same structure element may legitimately occupy multiple
				// ParentTree slots. Keep cycle detection local to one slot so
				// PageStructure preserves Playa's repeated sequence entries.
				for element, err := range d.parentTreeElementsSeqWithSeen(parent, map[Ref]bool{}, roleMap) {
					if err != nil {
						yield(PageStructureEntry{}, err)
						return
					}
					entry.elements = append(entry.elements, element)
				}
				if len(entry.elements) > 0 {
					entry.element = &entry.elements[0]
				}
			}
			if !yield(entry, nil) {
				return
			}
		}
	}
}

func (d *Document) structureRoleMap() map[string]string {
	roles, _ := d.structureRoleMapChecked()
	return roles
}

func (d *Document) structureRoleMapChecked() (map[string]string, error) {
	d.cacheMu.RLock()
	if d.structureRoleMapReady {
		roles := d.structureRoleMapCache
		err := d.structureRoleMapErr
		d.cacheMu.RUnlock()
		return roles, err
	}
	d.cacheMu.RUnlock()
	cacheError := func(err error) (map[string]string, error) {
		d.cacheMu.Lock()
		defer d.cacheMu.Unlock()
		if d.structureRoleMapReady {
			return d.structureRoleMapCache, d.structureRoleMapErr
		}
		d.structureRoleMapErr = err
		d.structureRoleMapReady = true
		return nil, err
	}
	rootValue, ok := d.resolveIndirectChain(d.trailer[Name("Root")])
	if !ok {
		return cacheError(fmt.Errorf("playa: catalog root is not a dictionary"))
	}
	root, ok := rootValue.(Dict)
	if !ok {
		return cacheError(fmt.Errorf("playa: catalog root is not a dictionary"))
	}
	structureValue, ok := d.resolveIndirectChain(root[Name("StructTreeRoot")])
	if !ok {
		return cacheError(fmt.Errorf("playa: StructTreeRoot is not a dictionary"))
	}
	structure, ok := structureValue.(Dict)
	if !ok {
		return cacheError(fmt.Errorf("playa: StructTreeRoot is not a dictionary"))
	}
	roles := map[string]string{}
	if raw, present := structure[Name("RoleMap")]; present {
		roleMapValue, _ := d.resolveIndirectChain(raw)
		roleMap, ok := roleMapValue.(Dict)
		if !ok {
			return cacheError(fmt.Errorf("playa: RoleMap is not a dictionary"))
		}
		for key, value := range roleMap {
			roleValue, _ := d.resolveIndirectChain(value)
			if role, ok := roleValue.(Name); ok {
				roles[string(key)] = string(role)
			} else {
				return cacheError(fmt.Errorf("playa: RoleMap entry %q is not a name", key))
			}
		}
	}
	d.cacheMu.Lock()
	defer d.cacheMu.Unlock()
	if d.structureRoleMapReady {
		return d.structureRoleMapCache, d.structureRoleMapErr
	}
	d.structureRoleMapCache = roles
	d.structureRoleMapReady = true
	return d.structureRoleMapCache, nil
}

func (d *Document) parentTreeValue(key int) (Object, bool) {
	value, ok, _ := d.parentTreeValueChecked(key)
	return value, ok
}

func (d *Document) parentTreeValueChecked(key int) (Object, bool, error) {
	d.cacheMu.RLock()
	if err, ok := d.parentTreeErrors[key]; ok {
		d.cacheMu.RUnlock()
		return nil, false, err
	}
	if value, ok := d.parentTreeCache[key]; ok {
		d.cacheMu.RUnlock()
		return value, true, nil
	}
	if d.parentTreeMissing[key] {
		d.cacheMu.RUnlock()
		return nil, false, nil
	}
	d.cacheMu.RUnlock()
	rawRoot, present := d.trailer[Name("Root")]
	if !present {
		return nil, false, nil
	}
	rootValue, ok := d.resolveIndirectChain(rawRoot)
	if !ok {
		return nil, false, d.cacheParentTreeError(key, fmt.Errorf("playa: catalog root is not a dictionary"))
	}
	root, ok := rootValue.(Dict)
	if !ok {
		return nil, false, d.cacheParentTreeError(key, fmt.Errorf("playa: catalog root is not a dictionary"))
	}
	rawStructure, present := root[Name("StructTreeRoot")]
	if !present {
		return nil, false, nil
	}
	structureValue, ok := d.resolveIndirectChain(rawStructure)
	if !ok {
		return nil, false, d.cacheParentTreeError(key, fmt.Errorf("playa: StructTreeRoot is not a dictionary"))
	}
	structure, ok := structureValue.(Dict)
	if !ok {
		return nil, false, d.cacheParentTreeError(key, fmt.Errorf("playa: StructTreeRoot is not a dictionary"))
	}
	tree, present := structure[Name("ParentTree")]
	if !present {
		return nil, false, nil
	}
	value, ok, err := d.lookupNumberTreeChecked(tree, key, map[Ref]bool{}, nil)
	d.cacheMu.Lock()
	if d.parentTreeCache == nil {
		d.parentTreeCache = map[int]Object{}
		d.parentTreeMissing = map[int]bool{}
	}
	if err != nil {
		d.cacheMu.Unlock()
		return nil, false, d.cacheParentTreeError(key, err)
	}
	if ok {
		d.cacheParentTreeValue(key, value)
	} else {
		d.parentTreeMissing[key] = true
	}
	d.cacheMu.Unlock()
	return value, ok, nil
}

func (d *Document) cacheParentTreeError(key int, err error) error {
	d.cacheMu.Lock()
	defer d.cacheMu.Unlock()
	if cached, exists := d.parentTreeErrors[key]; exists {
		return cached
	}
	if err == nil || len(err.Error()) > d.cacheLimits().ParentTreeErrorBytes || !cacheFits(d.parentTreeErrorBytes, len(err.Error()), d.cacheLimits().ParentTreeErrorBytes) {
		return err
	}
	if d.parentTreeErrors == nil {
		d.parentTreeErrors = map[int]error{}
	}
	d.parentTreeErrors[key] = err
	d.parentTreeErrorBytes += len(err.Error())
	return err
}

func (d *Document) cacheParentTreeValue(key int, value Object) {
	valueBytes := nameTreeObjectSize(value, 8)
	limit := d.cacheLimits().ParentTreeBytes
	if valueBytes > limit {
		return
	}
	if _, exists := d.parentTreeCache[key]; exists {
		return
	}
	if !cacheFits(d.parentTreeCacheBytes, valueBytes, limit) {
		return
	}
	d.parentTreeCache[key] = value
	d.parentTreeCacheBytes += valueBytes
}

func (d *Document) parentTreeElement(key int) *StructElement {
	element, _ := d.parentTreeElementWithError(key)
	return element
}

func (d *Document) parentTreeElementWithError(key int) (*StructElement, error) {
	value, ok, err := d.parentTreeValueChecked(key)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, nil
	}
	for element, err := range d.parentTreeElementsSeq(value) {
		if err != nil {
			return nil, err
		}
		return &element, nil
	}
	return nil, nil
}

// parentTreeContentElementWithError resolves one MCID slot from a container's
// StructParents entry. A plural ParentTree value is an array; unlike a singular
// StructParent it does not make the first array element the container's parent.
func (d *Document) parentTreeContentElementWithError(key, mcid int) (*StructElement, error) {
	if d == nil || mcid < 0 {
		return nil, nil
	}
	value, ok, err := d.parentTreeValueChecked(key)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, nil
	}
	resolvedSlots, slotsResolved := d.resolveIndirectChain(value)
	if !slotsResolved {
		return nil, fmt.Errorf("playa: ParentTree value could not be resolved")
	}
	slots, ok := resolvedSlots.(Array)
	if !ok || mcid >= len(slots) {
		return nil, nil
	}
	for element, elementErr := range d.parentTreeElementsSeq(slots[mcid]) {
		if elementErr != nil {
			return nil, elementErr
		}
		return &element, nil
	}
	return nil, nil
}

func (d *Document) contentParentWithContext(page Ref, mcid int, hasMCID bool, parentKey int, hasParentKey bool) (*StructElement, error) {
	if d == nil {
		return nil, errNilDocument
	}
	if !hasMCID {
		return nil, nil
	}
	if hasParentKey {
		return d.parentTreeContentElementWithError(parentKey, mcid)
	}
	return d.pageContentParentWithError(page, mcid)
}

// pageContentParent resolves a content object's MCID through the page's
// StructParents ParentTree entry. ParentTree slots are arrays, with the MCID
// selecting the slot and the slot's structure element being the parent.
func (d *Document) pageContentParent(page Ref, mcid int) *StructElement {
	parent, _ := d.pageContentParentWithError(page, mcid)
	return parent
}

func (d *Document) pageContentParentWithError(page Ref, mcid int) (*StructElement, error) {
	if d == nil || page == (Ref{}) || mcid < 0 {
		return nil, nil
	}
	p, err := d.PageByRef(page)
	if err != nil {
		return nil, err
	}
	key, present, err := p.ParentKeyWithError(d)
	if err != nil {
		return nil, err
	}
	if !present {
		return nil, nil
	}
	return d.parentTreeContentElementWithError(key, mcid)
}

// parentTreeElementsSeq yields borrowed direct structure elements stored in a
// ParentTree value. ParentTree arrays may be nested by producers, but their
// entries are not structure-tree containers and must not expand descendants.
func (d *Document) parentTreeElementsSeq(o Object) iter.Seq2[StructElement, error] {
	return d.parentTreeElementsSeqWithSeen(o, map[Ref]bool{}, d.structureRoleMap())
}

func (d *Document) parentTreeElementsSeqWithSeen(o Object, seen map[Ref]bool, roleMap map[string]string) iter.Seq2[StructElement, error] {
	return func(yield func(StructElement, error) bool) {
		var walk func(Object) bool
		walk = func(value Object) bool {
			resolved, resolvedOK := d.resolveIndirectChain(value)
			if !resolvedOK {
				return yield(StructElement{}, fmt.Errorf("playa: structure child could not be resolved"))
			}
			if items, ok := resolved.(Array); ok {
				for _, item := range items {
					if !walk(item) {
						return false
					}
				}
				return true
			}
			if _, ok := resolved.(Null); ok {
				return true
			}
			for _, element := range d.structKidsLazy(value, seen, roleMap) {
				if !yield(element, nil) {
					return false
				}
			}
			return true
		}
		walk(o)
	}
}

func (d *Document) lookupNumberTreeChecked(o Object, key int, seen map[Ref]bool, parentBounds *[2]int) (Object, bool, error) {
	if ref, ok := d.finalIndirectRef(o); ok {
		if seen[ref] {
			return nil, false, nil
		}
		seen[ref] = true
		defer delete(seen, ref)
	}
	treeValue, resolved := d.resolveIndirectChain(o)
	if !resolved {
		return nil, false, fmt.Errorf("playa: number tree root could not be resolved")
	}
	tree, ok := treeValue.(Dict)
	if !ok {
		if o == nil {
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("playa: number tree node is not a dictionary")
	}
	_, hasNums := tree[Name("Nums")]
	_, hasKids := tree[Name("Kids")]
	if hasNums && hasKids {
		return nil, false, fmt.Errorf("playa: number tree node cannot contain both Nums and Kids")
	}
	bounds, err := d.parseNumberTreeLimits(tree)
	if err != nil {
		return nil, false, err
	}
	if bounds != nil && parentBounds != nil && (bounds[0] < parentBounds[0] || bounds[1] > parentBounds[1]) {
		return nil, false, fmt.Errorf("playa: number tree child Limits are outside parent")
	}
	if raw, present := tree[Name("Nums")]; present {
		resolvedNums, resolved := d.resolveIndirectChain(raw)
		if !resolved {
			return nil, false, fmt.Errorf("playa: number tree Nums could not be resolved")
		}
		nums, ok := resolvedNums.(Array)
		if !ok {
			return nil, false, fmt.Errorf("playa: number tree Nums is not an array")
		}
		if len(nums)%2 != 0 {
			return nil, false, fmt.Errorf("playa: number tree Nums has an unmatched key")
		}
		previousKey := -1
		for i := 0; i+1 < len(nums); i += 2 {
			keyValue, resolved := d.resolveIndirectChain(nums[i])
			if !resolved {
				return nil, false, fmt.Errorf("playa: number tree key %d could not be resolved", i/2)
			}
			item, valid := IntValue(keyValue)
			if !valid {
				return nil, false, fmt.Errorf("playa: number tree key %d is not an integer", i/2)
			}
			if item < 0 {
				return nil, false, fmt.Errorf("playa: number tree key %d is negative", i/2)
			}
			if i > 0 && item <= previousKey {
				return nil, false, fmt.Errorf("playa: number tree keys are not sorted")
			}
			if bounds != nil && (item < bounds[0] || item > bounds[1]) {
				return nil, false, fmt.Errorf("playa: number tree key %d is outside Limits", i/2)
			}
			previousKey = item
			if item == key {
				return nums[i+1], true, nil
			}
		}
	}
	if raw, present := tree[Name("Kids")]; present {
		resolvedKids, resolved := d.resolveIndirectChain(raw)
		if !resolved {
			return nil, false, fmt.Errorf("playa: number tree Kids could not be resolved")
		}
		kids, ok := resolvedKids.(Array)
		if !ok {
			return nil, false, fmt.Errorf("playa: number tree Kids is not an array")
		}
		var previousChildBounds *[2]int
		for _, kid := range kids {
			childValue, resolved := d.resolveIndirectChain(kid)
			if !resolved {
				return nil, false, fmt.Errorf("playa: number tree child could not be resolved")
			}
			child, ok := childValue.(Dict)
			if !ok {
				return nil, false, fmt.Errorf("playa: number tree child is not a dictionary")
			}
			childBounds, err := d.parseNumberTreeLimits(child)
			if err != nil {
				return nil, false, err
			}
			if childBounds != nil && previousChildBounds != nil {
				if childBounds[0] < previousChildBounds[0] {
					return nil, false, fmt.Errorf("playa: number tree sibling Limits are not sorted")
				}
				if childBounds[0] <= previousChildBounds[1] {
					return nil, false, fmt.Errorf("playa: number tree sibling Limits overlap")
				}
			}
			if childBounds != nil {
				copyBounds := *childBounds
				previousChildBounds = &copyBounds
			}
			if value, found, err := d.lookupNumberTreeChecked(child, key, seen, bounds); err != nil {
				return nil, false, err
			} else if found {
				return value, true, nil
			}
		}
	}
	return nil, false, nil
}

func (d *Document) parseNumberTreeLimits(tree Dict) (*[2]int, error) {
	raw, present := tree[Name("Limits")]
	if !present {
		return nil, nil
	}
	value, resolved := d.resolveIndirectChain(raw)
	if !resolved {
		return nil, fmt.Errorf("playa: number tree Limits could not be resolved")
	}
	limits, ok := value.(Array)
	if !ok || len(limits) != 2 {
		return nil, fmt.Errorf("playa: number tree Limits must contain two integers")
	}
	parsed := [2]int{}
	for i, item := range limits {
		resolvedItem, resolved := d.resolveIndirectChain(item)
		if !resolved {
			return nil, fmt.Errorf("playa: number tree Limit %d could not be resolved", i)
		}
		integer, ok := IntValue(resolvedItem)
		if !ok || integer < 0 {
			return nil, fmt.Errorf("playa: number tree Limits must contain non-negative integers")
		}
		parsed[i] = integer
	}
	if parsed[0] > parsed[1] {
		return nil, fmt.Errorf("playa: number tree Limits are out of order")
	}
	return &parsed, nil
}

// StructureTree materializes top-level structure elements into a slice.
// Returned elements retain their document association; call Finalize on an
// element when an independent snapshot is required.
func (d *Document) StructureTree() ([]StructElement, error) {
	var out []StructElement
	for element, err := range d.StructureTreeSeq() {
		if err != nil {
			return nil, err
		}
		if err := element.materializeChildrenErr(); err != nil {
			return nil, err
		}
		out = append(out, element)
	}
	return out, nil
}

// StructureTreeSeq traverses top-level structure elements on demand. Yielded
// elements borrow document-owned read-only state; call Finalize when an
// independent snapshot is required.
func (d *Document) StructureTreeSeq() iter.Seq2[StructElement, error] {
	return func(yield func(StructElement, error) bool) {
		d.cacheMu.RLock()
		if d.structureRootErrReady {
			err := d.structureRootErr
			d.cacheMu.RUnlock()
			yield(StructElement{}, err)
			return
		}
		d.cacheMu.RUnlock()
		cacheRootError := func(err error) error {
			d.cacheMu.Lock()
			defer d.cacheMu.Unlock()
			if d.structureRootErrReady {
				return d.structureRootErr
			}
			d.structureRootErr = err
			d.structureRootErrReady = true
			return err
		}
		root, ok := d.trailer[Name("Root")]
		if !ok {
			return
		}
		catalogValue, ok := d.resolveIndirectChain(root)
		if !ok {
			yield(StructElement{}, cacheRootError(fmt.Errorf("playa: catalog is not dictionary")))
			return
		}
		cat, ok := catalogValue.(Dict)
		if !ok {
			yield(StructElement{}, cacheRootError(fmt.Errorf("playa: catalog is not dictionary")))
			return
		}
		rawStructure, present := cat[Name("StructTreeRoot")]
		if !present {
			return
		}
		structureValue, ok := d.resolveIndirectChain(rawStructure)
		if !ok {
			yield(StructElement{}, cacheRootError(fmt.Errorf("playa: structure tree root is not a dictionary")))
			return
		}
		sr, ok := structureValue.(Dict)
		if !ok {
			yield(StructElement{}, cacheRootError(fmt.Errorf("playa: structure tree root is not a dictionary")))
			return
		}
		k, ok := sr[Name("K")]
		if !ok {
			return
		}
		roleMap, roleMapErr := d.structureRoleMapChecked()
		if roleMapErr != nil {
			yield(StructElement{}, roleMapErr)
			return
		}
		seen := map[Ref]bool{}
		resolvedKids, kidsResolved := d.resolveIndirectChain(k)
		if !kidsResolved {
			yield(StructElement{}, cacheRootError(fmt.Errorf("playa: structure tree root could not be resolved")))
			return
		}
		if items, ok := resolvedKids.(Array); ok {
			for _, item := range items {
				resolved, _ := d.resolveIndirectChain(item)
				if _, valid := resolved.(Dict); !valid {
					if _, isNested := resolved.(Array); !isNested {
						yield(StructElement{}, fmt.Errorf("playa: structure tree child is not a dictionary"))
						return
					}
				}
				for _, element := range d.structKidsLazy(item, seen, roleMap) {
					if !yield(element, nil) {
						return
					}
				}
			}
			return
		}
		if _, valid := resolvedKids.(Dict); !valid {
			yield(StructElement{}, cacheRootError(fmt.Errorf("playa: structure tree root is not a dictionary")))
			return
		}
		for _, element := range d.structKidsLazy(k, seen, roleMap) {
			if !yield(element, nil) {
				return
			}
		}
	}
}

// StructureTreeIndex returns the structure tree plus all elements carrying an
// MCID. The lookup is intentionally document-wide; callers can use HasPage
// and Page to narrow a match when the source PDF provides /Pg references.
func (d *Document) StructureTreeIndex() (StructureIndex, error) {
	roots, err := d.StructureTree()
	if err != nil {
		return StructureIndex{}, err
	}
	index := StructureIndex{roots: roots, byMCID: map[int][]StructElement{}}
	var walk func([]StructElement)
	walk = func(nodes []StructElement) {
		for _, node := range nodes {
			if node.HasMCID() && len(node.contents) == 0 {
				index.byMCID[node.MCID()] = append(index.byMCID[node.MCID()], node)
			}
			for _, content := range node.contents {
				if content.HasMCID() {
					index.byMCID[content.MCID()] = append(index.byMCID[content.MCID()], node)
				}
			}
			walk(node.children)
		}
	}
	walk(roots)
	return index, nil
}

func (d *Document) structKidsLazy(o Object, seen map[Ref]bool, roleMap map[string]string) []StructElement {
	return d.structKidsMode(o, seen, roleMap, true)
}

// structElementChildrenSeq traverses only the child entries requested by the
// consumer. In particular, it does not first flatten /K into a slice, which
// matters for large structure trees and for callers that stop early.
func (d *Document) structElementChildrenSeq(o Object, seen map[Ref]bool, roleMap map[string]string) iter.Seq2[StructElement, error] {
	return func(yield func(StructElement, error) bool) {
		var walk func(Object) bool
		walk = func(value Object) bool {
			resolved, resolvedOK := d.resolveIndirectChain(value)
			if !resolvedOK {
				return yield(StructElement{}, fmt.Errorf("playa: structure child could not be resolved"))
			}
			if items, ok := resolved.(Array); ok {
				for _, item := range items {
					if !walk(item) {
						return false
					}
				}
				return true
			}
			if kid, ok := resolved.(Dict); ok {
				typeValue := kid[Name("Type")]
				if _, present := kid[Name("Type")]; present {
					var typeResolved bool
					typeValue, typeResolved = d.resolveIndirectChain(typeValue)
					if !typeResolved {
						return yield(StructElement{}, fmt.Errorf("playa: structure child Type could not be resolved"))
					}
				}
				typ, _ := typeValue.(Name)
				if typ == Name("MCR") || typ == Name("OBJR") {
					return true
				}
			}
			if _, isDict := resolved.(Dict); !isDict {
				if _, isMCID := IntValue(resolved); !isMCID {
					return yield(StructElement{}, fmt.Errorf("playa: structure child is not a dictionary"))
				}
			}
			for _, child := range d.structKidsLazy(value, seen, roleMap) {
				if !yield(child, nil) {
					return false
				}
			}
			return true
		}
		walk(o)
	}
}

// structElementContentsSeq walks only marked-content and object-reference
// entries in a structure element's /K tree. It intentionally avoids building
// intermediate StructElement values, so ContentsSeq remains lazy even for
// very large tagged documents.
func (d *Document) structElementContentsSeq(o Object, page Ref, hasPage bool, seen map[Ref]bool) iter.Seq2[StructureContent, error] {
	return func(yield func(StructureContent, error) bool) {
		resolveField := func(dict Dict, key Name) (Object, Object, bool, error) {
			raw, present := dict[key]
			if !present {
				return nil, nil, false, nil
			}
			resolved, ok := d.resolveIndirectChain(raw)
			if !ok {
				return raw, nil, true, fmt.Errorf("playa: structure content %s could not be resolved", key)
			}
			return raw, resolved, true, nil
		}
		var walk func(Object, Ref, bool) bool
		walk = func(value Object, currentPage Ref, currentHasPage bool) bool {
			if ref, ok := d.finalIndirectRef(value); ok {
				if seen[ref] {
					return true
				}
				seen[ref] = true
				defer delete(seen, ref)
			}
			resolved, resolvedOK := d.resolveIndirectChain(value)
			if !resolvedOK {
				return yield(StructureContent{}, fmt.Errorf("playa: structure content could not be resolved"))
			}
			if items, ok := resolved.(Array); ok {
				for _, item := range items {
					if !walk(item, currentPage, currentHasPage) {
						return false
					}
				}
				return true
			}
			if mcid, ok := IntValue(resolved); ok {
				return yield(newStructureContent(structuredata.ContentSpec{
					Kind: StructureMarkedContent, MCID: mcid, HasMCID: true,
					Page: currentPage, HasPage: currentHasPage,
				}), nil)
			}
			kid, ok := resolved.(Dict)
			if !ok {
				return yield(StructureContent{}, fmt.Errorf("playa: structure content item is not a dictionary"))
			}
			_, typeValue, _, err := resolveField(kid, Name("Type"))
			if err != nil {
				return yield(StructureContent{}, err)
			}
			typ, _ := typeValue.(Name)
			switch typ {
			case Name("MCR"), Name("OBJR"):
				contentKind := StructureMarkedContent
				var contentMCID int
				var contentHasMCID bool
				contentPage, contentHasPage := currentPage, currentHasPage
				var contentObjectRef Ref
				var contentHasObject bool
				var contentStream Stream
				var contentHasStream bool
				if typ == Name("OBJR") {
					contentKind = StructureObject
				}
				_, mcidValue, _, err := resolveField(kid, Name("MCID"))
				if err != nil {
					return yield(StructureContent{}, err)
				}
				if mcid, ok := IntValue(mcidValue); ok {
					contentMCID, contentHasMCID = mcid, true
				}
				pageRaw := kid[Name("Pg")]
				if ref, ok := d.finalIndirectRef(pageRaw); ok {
					contentPage, contentHasPage = ref, true
				}
				objectRaw := kid[Name("Obj")]
				if ref, ok := d.finalIndirectRef(objectRaw); ok {
					contentObjectRef, contentHasObject = ref, true
				}
				_, streamValue, _, err := resolveField(kid, Name("Stm"))
				if err != nil {
					return yield(StructureContent{}, err)
				}
				if stream, ok := streamValue.(Stream); ok {
					contentStream, contentHasStream = stream, true
				}
				content := newStructureContent(structuredata.ContentSpec{
					Kind: contentKind, MCID: contentMCID, HasMCID: contentHasMCID,
					Page: contentPage, HasPage: contentHasPage, Stream: contentStream,
					HasStream: contentHasStream, ObjectRef: contentObjectRef,
					HasObject: contentHasObject, Dict: kid,
				})
				return yield(content, nil)
			}
			childPage, childHasPage := currentPage, currentHasPage
			pageRaw := kid[Name("Pg")]
			if ref, ok := d.finalIndirectRef(pageRaw); ok {
				childPage, childHasPage = ref, true
			}
			if child, ok := kid[Name("K")]; ok {
				return walk(child, childPage, childHasPage)
			}
			_, mcidValue, _, err := resolveField(kid, Name("MCID"))
			if err != nil {
				return yield(StructureContent{}, err)
			}
			if mcid, ok := IntValue(mcidValue); ok {
				return yield(newStructureContent(structuredata.ContentSpec{
					Kind: StructureMarkedContent, MCID: mcid, HasMCID: true,
					Page: childPage, HasPage: childHasPage,
				}), nil)
			}
			objectRaw := kid[Name("Obj")]
			if object, ok := d.finalIndirectRef(objectRaw); ok {
				return yield(newStructureContent(structuredata.ContentSpec{
					Kind: StructureObject, ObjectRef: object, HasObject: true,
					Page: childPage, HasPage: childHasPage,
				}), nil)
			}
			return true
		}
		walk(o, page, hasPage)
	}
}

// structElementItemsSeq walks the direct /K entries without materializing a
// parallel child/content projection. This is the Go equivalent of Playa's
// mixed Element iterator and is intentionally source-order preserving.
func (d *Document) structElementItemsSeq(o Object, page Ref, hasPage bool, seen map[Ref]bool, roleMap map[string]string) iter.Seq2[StructureItem, error] {
	return func(yield func(StructureItem, error) bool) {
		var walk func(Object) bool
		walk = func(value Object) bool {
			resolved, ok := d.resolveIndirectChain(value)
			if !ok {
				return yield(StructureItem{}, fmt.Errorf("playa: structure item could not be resolved"))
			}
			if items, ok := resolved.(Array); ok {
				for _, item := range items {
					if !walk(item) {
						return false
					}
				}
				return true
			}
			if mcid, ok := IntValue(resolved); ok {
				return yield(newStructureItem(structureconfig.ItemMarkedContent, newStructureContent(structuredata.ContentSpec{Kind: structureconfig.MarkedContent, MCID: mcid, HasMCID: true, Page: page, HasPage: hasPage})), nil)
			}
			kid, ok := resolved.(Dict)
			if !ok {
				return yield(StructureItem{}, fmt.Errorf("playa: structure item is not a dictionary"))
			}
			typeValue, typeResolved := d.resolveIndirectChain(kid[Name("Type")])
			if _, present := kid[Name("Type")]; present && !typeResolved {
				return yield(StructureItem{}, fmt.Errorf("playa: structure item Type could not be resolved"))
			}
			typ, _ := typeValue.(Name)
			if typ == Name("MCR") || typ == Name("OBJR") {
				contentKind := structureconfig.MarkedContent
				contentMCID, contentHasMCID := 0, false
				contentPage, contentHasPage := page, hasPage
				var contentObjectRef Ref
				var contentHasObject bool
				var contentStream Stream
				var contentHasStream bool
				itemKind := structureconfig.ItemMarkedContent
				if typ == Name("OBJR") {
					contentKind = structureconfig.Object
					itemKind = structureconfig.ItemObject
				}
				if raw, present := kid[Name("MCID")]; present {
					value, resolved := d.resolveIndirectChain(raw)
					if !resolved {
						return yield(StructureItem{}, fmt.Errorf("playa: structure item MCID could not be resolved"))
					}
					if mcid, ok := IntValue(value); ok {
						contentMCID, contentHasMCID = mcid, true
					}
				}
				if ref, ok := d.finalIndirectRef(kid[Name("Pg")]); ok {
					contentPage, contentHasPage = ref, true
				}
				if ref, ok := d.finalIndirectRef(kid[Name("Obj")]); ok {
					contentObjectRef, contentHasObject = ref, true
				}
				if raw, present := kid[Name("Stm")]; present {
					value, resolved := d.resolveIndirectChain(raw)
					if !resolved {
						return yield(StructureItem{}, fmt.Errorf("playa: structure item stream could not be resolved"))
					}
					if stream, ok := value.(Stream); ok {
						contentStream, contentHasStream = stream, true
					}
				}
				content := newStructureContent(structuredata.ContentSpec{
					Kind: contentKind, MCID: contentMCID, HasMCID: contentHasMCID,
					Page: contentPage, HasPage: contentHasPage, Stream: contentStream,
					HasStream: contentHasStream, ObjectRef: contentObjectRef,
					HasObject: contentHasObject, Dict: kid,
				})
				return yield(newStructureItem(itemKind, content), nil)
			}

			children := d.structKidsMode(value, cloneStructurePath(seen), roleMap, true)
			for i := range children {
				if !yield(newStructureElementItem(&children[i]), nil) {
					return false
				}
			}
			return true
		}
		walk(o)
	}
}

func (d *Document) structKids(o Object, seen map[Ref]bool, roleMap map[string]string) []StructElement {
	return d.structKidsMode(o, seen, roleMap, false)
}

func (d *Document) structKidsMode(o Object, seen map[Ref]bool, roleMap map[string]string, lazy bool) []StructElement {
	out := []StructElement{}
	resolvedObject, _ := d.resolveIndirectChain(o)
	if a, ok := resolvedObject.(Array); ok {
		for _, x := range a {
			out = append(out, d.structKidsMode(x, seen, roleMap, lazy)...)
		}
		return out
	}
	r, ok := d.finalIndirectRef(o)
	sourceRef, hasSourceRef := r, ok
	if ok {
		if seen[r] {
			return nil
		}
		seen[r] = true
	}
	v, ok := resolvedObject.(Dict)
	if !ok {
		return nil
	}
	roleValue, _ := d.resolveIndirectChain(v[Name("S")])
	role, _ := roleValue.(Name)
	typeValue, _ := d.resolveIndirectChain(v[Name("Type")])
	typ, _ := typeValue.(Name)
	title := ""
	titleValue, _ := d.resolveIndirectChain(v[Name("T")])
	if x, ok := titleValue.(String); ok {
		title = decodePDFText(x)
	}
	rawRole := string(role)
	resolvedRole := rawRole
	for i := 0; i < 16; i++ {
		next, ok := roleMap[resolvedRole]
		if !ok || next == resolvedRole {
			break
		}
		resolvedRole = next
	}
	spec := structuredata.ElementSpec{Type: string(typ), StructureType: rawRole, Role: resolvedRole, RawRole: rawRole, Title: title, Dict: v, Object: o, MCID: -1}
	if typ == Name("MCR") {
		spec.IsMCR = true
	}
	if typ == Name("OBJ") {
		if object, ok := d.finalIndirectRef(v[Name("Obj")]); ok {
			spec.ObjectRef, spec.HasObject = object, true
		}
	}
	altValue, _ := d.resolveIndirectChain(v[Name("Alt")])
	if alt, ok := altValue.(String); ok {
		spec.Alt = decodePDFText(alt)
	}
	actualValue, actualResolved := d.resolveIndirectChain(v[Name("ActualText")])
	actualTextSet := false
	if actual, ok := actualValue.(String); ok {
		spec.ActualText = decodePDFText(actual)
		actualTextSet = actualResolved
	}
	languageValue, _ := d.resolveIndirectChain(v[Name("Lang")])
	if language, ok := structureString(languageValue); ok {
		spec.Language = language
	}
	abbreviationValue, _ := d.resolveIndirectChain(v[Name("E")])
	if abbreviation, ok := structureString(abbreviationValue); ok {
		spec.Abbreviation = abbreviation
	}
	classValue, _ := d.resolveIndirectChain(v[Name("C")])
	if className, ok := structureClassName(d, classValue); ok {
		spec.ClassName = className
	}
	attributesValue, _ := d.resolveIndirectChain(v[Name("A")])
	if attributes, ok := structureAttributes(d, attributesValue); ok {
		spec.Attributes = attributes
	}
	if page, ok := d.finalIndirectRef(v[Name("Pg")]); ok {
		spec.Page, spec.HasPage = page, true
	}
	if parent, ok := d.finalIndirectRef(v[Name("P")]); ok {
		spec.Parent, spec.HasParent = parent, true
	}
	bboxValue, _ := d.resolveIndirectChain(v[Name("BBox")])
	if bbox, ok := bboxValue.(Array); ok && len(bbox) == 4 {
		valid := true
		for i := 0; i < 4; i++ {
			bboxItem, _ := d.resolveIndirectChain(bbox[i])
			spec.BBox[i], valid = finiteNumberValue(bboxItem)
			if !valid {
				break
			}
		}
		if valid {
			if spec.BBox[0] > spec.BBox[2] {
				spec.BBox[0], spec.BBox[2] = spec.BBox[2], spec.BBox[0]
			}
			if spec.BBox[1] > spec.BBox[3] {
				spec.BBox[1], spec.BBox[3] = spec.BBox[3], spec.BBox[1]
			}
			spec.HasBBox = true
		}
	}
	mcidValue, _ := d.resolveIndirectChain(v[Name("MCID")])
	if n, ok := IntValue(mcidValue); ok {
		spec.MCID = n
		spec.HasMCID = true
	}
	if x, ok := v[Name("K")]; ok {
		resolvedK, _ := d.resolveIndirectChain(x)
		if n, ok := IntValue(resolvedK); ok {
			spec.MCID = n
			spec.HasMCID = true
		}
	}
	e := newStructElementValue(spec)
	e.actualTextSet = actualTextSet
	e.sourceRef, e.hasSourceRef = sourceRef, hasSourceRef
	e.document, e.structurePath = d, cloneStructurePath(seen)
	if x, ok := v[Name("K")]; ok {
		if lazy {
			e.childStart, e.childRoleMap, e.childPage, e.childHasPage = x, roleMap, e.Page(), e.HasPage()
			e.contents, e.orderedContents = d.structElementDirectContents(x, e.Page(), e.HasPage())
			e.lazyState = &structElementLazyState{}
		} else {
			e.children, e.contents, e.orderedContents = d.structElementKids(x, seen, roleMap, e.Page(), e.HasPage())
			e.childrenReady = true
			e.contentsReady = true
		}
	}
	out = append(out, e)
	return out
}

func (e *StructElement) materializeChildren() {
	_ = e.materializeChildrenErr()
}

func (e *StructElement) materializeChildrenErr() error {
	if e == nil {
		return nil
	}
	if e.lazyState != nil {
		state := e.lazyState
		state.mu.Lock()
		defer state.mu.Unlock()
		if state.ready {
			e.children, e.contents, e.orderedContents, e.items = state.children, state.contents, state.orderedContents, state.items
			e.childrenReady, e.contentsReady, e.itemsReady = true, true, true
			return state.err
		}
		if e.document == nil || e.childStart == nil {
			state.ready = true
			state.children, state.contents, state.orderedContents, state.items = e.children, e.contents, e.orderedContents, e.items
			if state.items == nil {
				state.items = structureItemsFromParts(state.children, state.contents)
			}
			e.childrenReady, e.contentsReady, e.itemsReady = true, true, true
			return nil
		}
		for _, err := range e.document.structElementChildrenSeq(e.childStart, map[Ref]bool{}, e.childRoleMap) {
			if err != nil {
				state.ready, state.err = true, err
				e.childrenReady, e.contentsReady = true, true
				return err
			}
		}
		for _, err := range e.document.structElementContentsSeq(e.childStart, e.childPage, e.childHasPage, cloneStructurePath(e.structurePath)) {
			if err != nil {
				state.ready, state.err = true, err
				e.childrenReady, e.contentsReady = true, true
				return err
			}
		}
		children, contents, ordered := e.document.structElementKids(e.childStart, map[Ref]bool{}, e.childRoleMap, e.childPage, e.childHasPage)
		var items []StructureItem
		for item, err := range e.document.structElementItemsSeq(e.childStart, e.childPage, e.childHasPage, cloneStructurePath(e.structurePath), e.childRoleMap) {
			if err != nil {
				state.ready, state.err = true, err
				e.childrenReady, e.contentsReady, e.itemsReady = true, true, true
				return err
			}
			items = append(items, item)
		}
		state.children, state.contents, state.orderedContents, state.items = children, contents, ordered, items
		state.ready = true
		e.children, e.contents, e.orderedContents, e.items = children, contents, ordered, items
		e.childrenReady, e.contentsReady, e.itemsReady = true, true, true
		return nil
	}
	if e.childrenReady {
		return nil
	}
	if e.document == nil || e.childStart == nil {
		e.childrenReady = true
		e.contentsReady = true
		if e.items == nil {
			e.items = structureItemsFromParts(e.children, e.contents)
		}
		e.itemsReady = true
		return nil
	}
	for _, err := range e.document.structElementChildrenSeq(e.childStart, map[Ref]bool{}, e.childRoleMap) {
		if err != nil {
			return err
		}
	}
	for _, err := range e.document.structElementContentsSeq(e.childStart, e.childPage, e.childHasPage, cloneStructurePath(e.structurePath)) {
		if err != nil {
			return err
		}
	}
	// Build children, direct content, and flattened content order together.
	// Walking ChildrenSeq first and then calling structElementKids again
	// duplicated the full structure traversal for every node.
	children, contents, ordered := e.document.structElementKids(e.childStart, map[Ref]bool{}, e.childRoleMap, e.childPage, e.childHasPage)
	var items []StructureItem
	for item, err := range e.document.structElementItemsSeq(e.childStart, e.childPage, e.childHasPage, cloneStructurePath(e.structurePath), e.childRoleMap) {
		if err != nil {
			return err
		}
		items = append(items, item)
	}
	e.children = children
	e.contents = contents
	e.orderedContents = ordered
	e.items = items
	e.childrenReady = true
	e.contentsReady = true
	e.itemsReady = true
	return nil
}

func finalizeStructElement(element StructElement) StructElement {
	element.materializeChildren()
	clone := cloneStructElement(element)
	clone.document = nil
	clone.childStart = nil
	clone.childRoleMap = nil
	clone.childPage = Ref{}
	clone.childHasPage = false
	clone.structurePath = nil
	clone.childrenReady = true
	clone.contentsReady = true
	for i := range clone.children {
		clone.children[i] = finalizeStructElement(clone.children[i])
	}
	for i := range clone.contents {
		clone.contents[i] = clone.contents[i].Finalize()
	}
	for i := range clone.orderedContents {
		clone.orderedContents[i] = clone.orderedContents[i].Finalize()
	}
	for i := range clone.items {
		clone.items[i] = clone.items[i].Finalize()
	}
	return clone
}

func finalizeStructElementWithError(element StructElement) (StructElement, error) {
	if err := element.materializeChildrenErr(); err != nil {
		return StructElement{}, err
	}
	clone := cloneStructElement(element)
	clone.document = nil
	clone.childStart = nil
	clone.childRoleMap = nil
	clone.childPage = Ref{}
	clone.childHasPage = false
	clone.structurePath = nil
	clone.childrenReady = true
	clone.contentsReady = true
	for i, child := range element.children {
		value, err := finalizeStructElementWithError(child)
		if err != nil {
			return StructElement{}, err
		}
		clone.children[i] = value
	}
	for i := range clone.contents {
		clone.contents[i] = clone.contents[i].Finalize()
	}
	for i := range clone.orderedContents {
		clone.orderedContents[i] = clone.orderedContents[i].Finalize()
	}
	for i, item := range element.items {
		value, err := item.FinalizeWithError()
		if err != nil {
			return StructElement{}, err
		}
		clone.items[i] = value
	}
	return clone, nil
}

// Finalize materializes lazy children and returns an independent structure snapshot.
func (e StructElement) FinalizeWithError() (StructElement, error) {
	return finalizeStructElementWithError(e)
}

func (e StructElement) Finalize() StructElement {
	clone, _ := e.FinalizeWithError()
	return clone
}

func (d *Document) structElementDirectContents(o Object, page Ref, hasPage bool) ([]StructureContent, []StructureContent) {
	resolvedItems, _ := d.resolveIndirectChain(o)
	items, ok := resolvedItems.(Array)
	if !ok {
		items = Array{o}
	}
	var contents []StructureContent
	for _, item := range items {
		resolved, _ := d.resolveIndirectChain(item)
		if mcid, ok := IntValue(resolved); ok {
			contents = append(contents, newStructureContent(structuredata.ContentSpec{
				Kind: StructureMarkedContent, MCID: mcid, HasMCID: true,
				Page: page, HasPage: hasPage,
			}))
			continue
		}
		kid, ok := resolved.(Dict)
		if !ok {
			continue
		}
		typeValue, _ := d.resolveIndirectChain(kid[Name("Type")])
		typ, _ := typeValue.(Name)
		if typ != Name("MCR") && typ != Name("OBJR") {
			continue
		}
		contentKind := StructureMarkedContent
		contentMCID, contentHasMCID := 0, false
		contentPage, contentHasPage := page, hasPage
		var contentObjectRef Ref
		var contentHasObject bool
		var contentStream Stream
		var contentHasStream bool
		if typ == Name("OBJR") {
			contentKind = StructureObject
		}
		mcidValue, _ := d.resolveIndirectChain(kid[Name("MCID")])
		if value, ok := IntValue(mcidValue); ok {
			contentMCID, contentHasMCID = value, true
		}
		if value, ok := d.finalIndirectRef(kid[Name("Pg")]); ok {
			contentPage, contentHasPage = value, true
		}
		if value, ok := d.finalIndirectRef(kid[Name("Obj")]); ok {
			contentObjectRef, contentHasObject = value, true
		}
		streamValue, _ := d.resolveIndirectChain(kid[Name("Stm")])
		if stream, ok := streamValue.(Stream); ok {
			contentStream, contentHasStream = stream, true
		}
		content := newStructureContent(structuredata.ContentSpec{
			Kind: contentKind, MCID: contentMCID, HasMCID: contentHasMCID,
			Page: contentPage, HasPage: contentHasPage, Stream: contentStream,
			HasStream: contentHasStream, ObjectRef: contentObjectRef,
			HasObject: contentHasObject, Dict: kid,
		})
		contents = append(contents, content)
	}
	// Direct K entries have the same logical order for both views. The slices
	// are owned by the lazy structure element and only traversed after
	// materialization, so sharing the backing array avoids a second allocation.
	return contents, contents
}

func (d *Document) structElementKids(o Object, seen map[Ref]bool, roleMap map[string]string, page Ref, hasPage bool) ([]StructElement, []StructureContent, []StructureContent) {
	resolvedItems, _ := d.resolveIndirectChain(o)
	items, ok := resolvedItems.(Array)
	if !ok {
		items = Array{o}
	}
	var children []StructElement
	var contents []StructureContent
	var orderedContents []StructureContent
	for _, item := range items {
		resolved, _ := d.resolveIndirectChain(item)
		if mcid, ok := IntValue(resolved); ok {
			content := newStructureContent(structuredata.ContentSpec{
				Kind: StructureMarkedContent, MCID: mcid, HasMCID: true,
				Page: page, HasPage: hasPage,
			})
			contents = append(contents, content)
			orderedContents = append(orderedContents, content)
			continue
		}
		kidValue, _ := d.resolveIndirectChain(resolved)
		kid, ok := kidValue.(Dict)
		if !ok {
			continue
		}
		typeValue, _ := d.resolveIndirectChain(kid[Name("Type")])
		typ, _ := typeValue.(Name)
		switch typ {
		case Name("MCR"):
			contentKind := StructureMarkedContent
			contentMCID, contentHasMCID := 0, false
			contentPage, contentHasPage := page, hasPage
			var contentStream Stream
			var contentHasStream bool
			mcidValue, _ := d.resolveIndirectChain(kid[Name("MCID")])
			if value, ok := IntValue(mcidValue); ok {
				contentMCID, contentHasMCID = value, true
			}
			if value, ok := d.finalIndirectRef(kid[Name("Pg")]); ok {
				contentPage, contentHasPage = value, true
			}
			streamValue, _ := d.resolveIndirectChain(kid[Name("Stm")])
			if stream, ok := streamValue.(Stream); ok {
				contentStream, contentHasStream = stream, true
			}
			content := newStructureContent(structuredata.ContentSpec{
				Kind: contentKind, MCID: contentMCID, HasMCID: contentHasMCID,
				Page: contentPage, HasPage: contentHasPage, Stream: contentStream,
				HasStream: contentHasStream, Dict: kid,
			})
			contents = append(contents, content)
			orderedContents = append(orderedContents, content)
		case Name("OBJR"):
			contentPage, contentHasPage := page, hasPage
			var contentObjectRef Ref
			var contentHasObject bool
			if value, ok := d.finalIndirectRef(kid[Name("Pg")]); ok {
				contentPage, contentHasPage = value, true
			}
			if value, ok := d.finalIndirectRef(kid[Name("Obj")]); ok {
				contentObjectRef, contentHasObject = value, true
			}
			content := newStructureContent(structuredata.ContentSpec{
				Kind: StructureObject, Page: contentPage, HasPage: contentHasPage,
				ObjectRef: contentObjectRef, HasObject: contentHasObject, Dict: kid,
			})
			contents = append(contents, content)
			orderedContents = append(orderedContents, content)
		default:
			nested := d.structKids(item, seen, roleMap)
			children = append(children, nested...)
			for _, child := range nested {
				if len(child.orderedContents) > 0 {
					orderedContents = append(orderedContents, child.orderedContents...)
				} else if child.IsMCR() && child.HasMCID() {
					orderedContents = append(orderedContents, newStructureContent(structuredata.ContentSpec{Kind: StructureMarkedContent, MCID: child.MCID(), HasMCID: true, Page: child.Page(), HasPage: child.HasPage()}))
				} else if child.HasObject() {
					orderedContents = append(orderedContents, newStructureContent(structuredata.ContentSpec{Kind: StructureObject, ObjectRef: child.ObjectRef(), HasObject: true, Page: child.Page(), HasPage: child.HasPage()}))
				}
			}
		}
	}
	return children, contents, orderedContents
}

func structureString(value Object) (string, bool) {
	switch value := value.(type) {
	case String:
		return decodePDFText(value), true
	case Name:
		return string(value), true
	default:
		return "", false
	}
}

func structureRect(d *Document, value Object) ([4]float64, bool) {
	resolved, _ := d.resolveIndirectChain(value)
	items, ok := resolved.(Array)
	if !ok || len(items) != 4 {
		return [4]float64{}, false
	}
	var rect [4]float64
	for i := range rect {
		var valid bool
		item, _ := d.resolveIndirectChain(items[i])
		rect[i], valid = finiteNumberValue(item)
		if !valid {
			return [4]float64{}, false
		}
	}
	if rect[0] > rect[2] {
		rect[0], rect[2] = rect[2], rect[0]
	}
	if rect[1] > rect[3] {
		rect[1], rect[3] = rect[3], rect[1]
	}
	return rect, true
}

func structureClassName(d *Document, value Object) (string, bool) {
	value, _ = d.resolveIndirectChain(value)
	if name, ok := value.(Name); ok {
		return string(name), true
	}
	items, ok := value.(Array)
	if !ok {
		return "", false
	}
	var selected string
	latest, found := 0, false
	for i := 0; i < len(items); i++ {
		classValue, _ := d.resolveIndirectChain(items[i])
		name, ok := classValue.(Name)
		if !ok {
			continue
		}
		// ISO 32000-1 permits each class name to have an optional revision.
		// Preserve Playa's singular projection: first name at the latest revision.
		revision := 0
		if i+1 < len(items) {
			revisionValue, _ := d.resolveIndirectChain(items[i+1])
			if value, ok := IntValue(revisionValue); ok {
				revision = value
				i++
			}
		}
		if !found || revision > latest {
			selected, latest, found = string(name), revision, true
		}
	}
	return selected, found
}

func structureAttributes(d *Document, value Object) (Dict, bool) {
	value, _ = d.resolveIndirectChain(value)
	if attributes, ok := structureAttributeDict(value); ok {
		return attributes, true
	}
	items, ok := value.(Array)
	if !ok {
		return nil, false
	}
	var attributes Dict
	for i := 0; i < len(items); i++ {
		attributeValue, _ := d.resolveIndirectChain(items[i])
		attributeObject, ok := structureAttributeDict(attributeValue)
		if !ok {
			continue
		}
		if attributes == nil {
			attributes = Dict{}
		}
		for key, value := range attributeObject {
			attributes[key] = value
		}
		// A revision number may follow an attribute object, but is optional.
		// Consume it when present without mistaking the next unversioned
		// attribute object for a revision.
		if i+1 < len(items) {
			revisionValue, _ := d.resolveIndirectChain(items[i+1])
			if _, ok := IntValue(revisionValue); ok {
				i++
			}
		}
	}
	return attributes, attributes != nil
}

func structureAttributeDict(value Object) (Dict, bool) {
	switch value := value.(type) {
	case Dict:
		return value, true
	case Stream:
		return value.DictBorrowed(), true
	default:
		return nil, false
	}
}
