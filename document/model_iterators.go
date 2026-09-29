package document

import (
	"fmt"
	"iter"
	"sort"
	"strings"

	"github.com/lin-string/go-playa/contentconfig"
	"github.com/lin-string/go-playa/contentdata"
	"github.com/lin-string/go-playa/geometry"
	"github.com/lin-string/go-playa/structureconfig"
)

// ChildrenSeq exposes borrowed outline children without requiring callers to
// range over a materialized slice. Call Finalize when an independent snapshot
// is required.
func (n OutlineNode) ChildrenSeq() iter.Seq2[OutlineNode, error] {
	if n.childrenReady {
		return outlineNodesSeq(n.children)
	}
	if n.document != nil && n.hasOutlineRef {
		if cached, ok := n.document.cachedOutlineNode(n.outlineRef); ok && cached.childrenReady {
			return outlineNodesSeq(cached.children)
		}
	}
	if n.document != nil && n.childStart != nil {
		return func(yield func(OutlineNode, error) bool) {
			seen := cloneOutlinePath(n.outlinePath)
			if seen == nil {
				seen = map[Ref]bool{}
			}
			n.document.outlineSiblingsSeq(n.childStart, seen, func(node OutlineNode, err error) bool {
				return yield(node, err)
			})
		}
	}
	return outlineNodesSeq(n.children)
}

// ChildrenSeq exposes borrowed structure-tree children using the same
// iterator contract as the document-level traversal. Call Finalize when a
// stable independent snapshot is required.
func (e StructElement) ChildrenSeq() iter.Seq2[StructElement, error] {
	if e.childrenReady {
		return structElementsSeq(e.children)
	}
	if e.document != nil && e.childStart != nil {
		seen := cloneStructurePath(e.structurePath)
		if seen == nil {
			seen = map[Ref]bool{}
		}
		return e.document.structElementChildrenSeq(e.childStart, seen, e.childRoleMap)
	}
	return structElementsSeq(e.children)
}

// ItemsSeq exposes the direct mixed /K traversal used by Playa's Element
// iterator. It preserves source order across nested elements, MCID values,
// marked-content references, and object references. The yielded values are
// borrowed; use StructureItem.Finalize when they must outlive the walk.
func (e StructElement) ItemsSeq() iter.Seq2[StructureItem, error] {
	if e.itemsReady {
		return structureItemsSeq(e.items)
	}
	if e.document != nil && e.childStart != nil {
		seen := cloneStructurePath(e.structurePath)
		if seen == nil {
			seen = map[Ref]bool{}
		}
		return e.document.structElementItemsSeq(e.childStart, e.childPage, e.childHasPage, seen, e.childRoleMap)
	}
	if e.items != nil {
		return structureItemsSeq(e.items)
	}
	return structureItemsSeq(structureItemsFromParts(e.children, e.contents))
}

// ItemsCopy returns independent mixed /K items, omitting deferred errors.
func (e StructElement) ItemsCopy() []StructureItem {
	items, _ := e.ItemsCopyWithError()
	return items
}

// ItemsCopyWithError materializes independent mixed /K items and reports
// malformed or deferred structure entries.
func (e StructElement) ItemsCopyWithError() ([]StructureItem, error) {
	var out []StructureItem
	for item, err := range e.ItemsSeq() {
		if err != nil {
			return nil, err
		}
		copy, err := item.FinalizeWithError()
		if err != nil {
			return nil, err
		}
		out = append(out, copy)
	}
	return out, nil
}

// ContentsSeq exposes all borrowed marked-content and object-reference
// descendants in the depth-first K order used by Playa's Element.contents.
// Call Finalize when an independent snapshot is required.
func (e StructElement) ContentsSeq() iter.Seq2[StructureContent, error] {
	if e.contentsReady {
		return structureContentsSeq(e.orderedContents)
	}
	if e.document != nil && e.childStart != nil {
		seen := cloneStructurePath(e.structurePath)
		if seen == nil {
			seen = map[Ref]bool{}
		}
		return e.document.structElementContentsSeq(e.childStart, e.Page(), e.HasPage(), seen)
	}
	if len(e.orderedContents) > 0 {
		return structureContentsSeq(e.orderedContents)
	}
	return structureContentsSeq(e.contents)
}

// PageOrderSeq yields structure content in Playa's page-order view. Content
// objects are placed before marked-content items on the same page; marked
// content follows its first appearance in the page content stream.
func (e StructElement) PageOrderSeq(d *Document) iter.Seq2[StructureContent, error] {
	return func(yield func(StructureContent, error) bool) {
		if d == nil {
			yield(StructureContent{}, errNilDocument)
			return
		}
		var contents []StructureContent
		for content, err := range e.ContentsSeq() {
			if err != nil {
				yield(StructureContent{}, err)
				return
			}
			contents = append(contents, content)
		}
		type orderedContent struct {
			content StructureContent
			page    int
			order   int
			index   int
		}
		ordered := make([]orderedContent, 0, len(contents))
		pageOrders := map[Ref]map[int]int{}
		for index, content := range contents {
			page, err, hasPage := structureContentPage(d, content, e)
			if err != nil {
				yield(StructureContent{}, err)
				return
			}
			if !hasPage {
				ordered = append(ordered, orderedContent{content: content, page: int(^uint(0) >> 1), order: index, index: index})
				continue
			}
			orders, cached := pageOrders[page.ref]
			if !cached {
				orders = map[int]int{}
				position := 0
				for object, interpErr := range page.Flatten(d, contentconfig.Options{Filter: FilterAll}) {
					if interpErr != nil {
						yield(StructureContent{}, interpErr)
						return
					}
					if mcid, ok := object.MCIDValue(); ok {
						if _, exists := orders[mcid]; !exists {
							orders[mcid] = position
						}
					}
					position++
				}
				pageOrders[page.ref] = orders
			}
			order := index
			if content.Kind() == StructureObject {
				order = -1
			} else if content.HasMCID() {
				if position, ok := orders[content.MCID()]; ok {
					order = position
				}
			}
			ordered = append(ordered, orderedContent{content: content, page: page.number, order: order, index: index})
		}
		sort.SliceStable(ordered, func(i, j int) bool {
			if ordered[i].page != ordered[j].page {
				return ordered[i].page < ordered[j].page
			}
			if ordered[i].order != ordered[j].order {
				return ordered[i].order < ordered[j].order
			}
			return ordered[i].index < ordered[j].index
		})
		for _, item := range ordered {
			if !yield(item.content, nil) {
				return
			}
		}
	}
}

// FindAllSeq walks descendant structure elements depth-first. An empty role
// matches every descendant; otherwise matching uses the normalized Role field.
func (e StructElement) FindAllSeq(role string) iter.Seq2[StructElement, error] {
	return func(yield func(StructElement, error) bool) {
		var walk func(StructElement) bool
		walk = func(node StructElement) bool {
			if role == "" || node.Role() == role {
				if !yield(node, nil) {
					return false
				}
			}
			for child, err := range node.ChildrenSeq() {
				if err != nil {
					return yield(StructElement{}, err)
				}
				if !walk(child) {
					return false
				}
			}
			return true
		}
		for child, err := range e.ChildrenSeq() {
			if err != nil {
				yield(StructElement{}, err)
				return
			}
			if !walk(child) {
				return
			}
		}
	}
}

// Find returns the first descendant matching role. An empty role returns the
// first descendant.
func (e StructElement) Find(role string) (StructElement, bool) {
	for node, err := range e.FindAllSeq(role) {
		if err == nil {
			return node, true
		}
	}
	return StructElement{}, false
}

// KidsSeq exposes borrowed form-field descendants in field-tree order. Call
// Finalize when an independent snapshot is required.
func (f FormField) KidsSeq() iter.Seq2[FormField, error] {
	if f.kidsReady {
		return formFieldsSeq(f.kids)
	}
	if f.document != nil && f.hasFieldRef {
		if cached, ok := f.document.cachedFormField(f.fieldRef); ok && cached.kidsReady {
			return formFieldsSeq(cached.kids)
		}
	}
	if f.document != nil && len(f.kidObjects) > 0 {
		return func(yield func(FormField, error) bool) {
			seen := cloneFormPath(f.formPath)
			if seen == nil {
				seen = map[Ref]bool{}
			}
			for _, item := range f.kidObjects {
				resolved, resolvedOK := f.document.resolveIndirectChain(item)
				if !resolvedOK {
					yield(FormField{}, fmt.Errorf("playa: form field child could not be resolved"))
					return
				}
				if _, valid := resolved.(Dict); !valid {
					yield(FormField{}, fmt.Errorf("playa: form field child is not a dictionary"))
					return
				}
				child, ok := f.document.formField(item, f.kidParent, f.kidParentName, seen)
				if !ok {
					continue
				}
				if !yield(child, nil) {
					return
				}
			}
		}
	}
	return formFieldsSeq(f.kids)
}

// ParentField resolves the field's /Parent reference. The returned value is
// a standalone lazy view and does not materialize the parent's children. Call
// Finalize when an independent materialized snapshot is required.
func (f FormField) ParentField(d *Document) *FormField {
	parent, _ := f.ParentFieldWithError(d)
	return parent
}

// ParentFieldWithError resolves the field's /Parent reference and reports a
// malformed referenced field.
func (f FormField) ParentFieldWithError(d *Document) (*FormField, error) {
	if d == nil {
		return nil, errNilDocument
	}
	if !f.HasParent() {
		return nil, nil
	}
	parentName := f.FullName()
	if separator := strings.LastIndexByte(parentName, '.'); separator >= 0 {
		parentName = parentName[:separator]
	} else {
		parentName = ""
	}
	parentValue, parentResolved := d.resolveIndirectChain(f.Parent())
	if !parentResolved {
		return nil, fmt.Errorf("playa: form field parent could not be resolved")
	}
	parentDict, ok := parentValue.(Dict)
	if !ok {
		return nil, fmt.Errorf("playa: form field parent is not a dictionary")
	}
	{
		fieldName := ""
		nameValue, _ := d.resolveIndirectChain(parentDict[Name("T")])
		if raw, ok := nameValue.(String); ok {
			fieldName = decodePDFText(raw)
		} else if raw, ok := nameValue.(Name); ok {
			fieldName = string(raw)
		}
		if parentName == fieldName {
			parentName = ""
		} else if fieldName != "" {
			parentName = strings.TrimSuffix(parentName, "."+fieldName)
		}
	}
	parent, ok := d.formField(f.Parent(), nil, parentName, map[Ref]bool{})
	if !ok {
		return nil, fmt.Errorf("playa: malformed form field parent")
	}
	return &parent, nil
}

// ChildrenSeq exposes nested marked-content sections in source order.
func (m MarkedContent) ChildrenSeq() iter.Seq2[MarkedContent, error] {
	if m.node != nil {
		return func(yield func(MarkedContent, error) bool) {
			for _, child := range m.node.ChildrenNodes {
				if !yield(markedContentValue(child), nil) {
					return
				}
			}
		}
	}
	return markedContentsSeq(m.children)
}

// OpsSeq exposes operators belonging directly to a marked-content section.
func (m MarkedContent) OpsSeq() iter.Seq2[ContentOp, error] {
	return func(yield func(ContentOp, error) bool) {
		ops := m.data.OpsBorrowed()
		for i, op := range ops {
			var context *markedContentOpContext
			if i < len(m.opContexts) {
				context = &m.opContexts[i]
			}
			if !yield(contentOpFromData(op, context, false), nil) {
				return
			}
		}
	}
}

// GlyphsSeq exposes a text object's glyphs through the same repeatable
// sequence contract as page-level content traversal.
func (t TextObject) GlyphsSeq() iter.Seq2[GlyphObject, error] {
	glyphs := t.glyphs
	return func(yield func(GlyphObject, error) bool) {
		for _, glyph := range glyphs {
			if !yield(glyph, nil) {
				return
			}
		}
	}
}

// PathsSeq exposes Type3 CharProc, embedded CFF, or TrueType glyph paths in
// device space. Fonts without an executable glyph outline produce an empty
// sequence.
func (g GlyphObject) PathsSeq() iter.Seq2[PathObject, error] {
	return func(yield func(PathObject, error) bool) {
		code := g.Codes()
		if g.font != nil {
			if _, err := g.font.ensureCFFWithError(); err != nil {
				yield(PathObject{}, err)
				return
			}
			if _, err := g.font.ensureType1EncodingWithError(); err != nil {
				yield(PathObject{}, err)
				return
			}
			if err := g.font.ensureType3GlyphWithError(code); err != nil {
				yield(PathObject{}, err)
				return
			}
		}
		if g.font != nil && g.font.fontType == "Type1" && len(code) > 0 {
			if ops, ok, err := g.font.type1GlyphPathOpsWithError(code); err != nil {
				if !yield(PathObject{}, err) {
					return
				}
				return
			} else if ok {
				pathOps := append([]ContentOp{newContentOpBorrowed("cm", matrixObjects(g.outlineMatrix()), 0)}, ops...)
				for path, pathErr := range valuesSeq(InterpretPaths(pathOps)) {
					if pathErr != nil || !yield(path, pathErr) {
						return
					}
				}
				return
			}
		}
		if g.font != nil && !g.font.type3 && len(g.font.cffCharstrings) == 0 && len(code) > 0 {
			ops, ok, err := g.font.trueTypePathOpsWithGID(code, g.GID())
			if err != nil {
				yield(PathObject{}, err)
				return
			}
			if ok {
				pathOps := []ContentOp{newContentOpBorrowed("cm", matrixObjects(g.outlineMatrix()), 0)}
				pathOps = append(pathOps, ops...)
				pathOps = append(pathOps, newContentOpBorrowed("f", nil, 0))
				for path, pathErr := range valuesSeq(InterpretPaths(pathOps)) {
					if pathErr != nil || !yield(path, pathErr) {
						return
					}
				}
				return
			}
		}
		hasCFF := g.font != nil && len(g.font.cffCharstrings) > 0
		hasType3 := g.font != nil && g.font.type3
		if len(g.outlineOpsBorrowed()) == 0 && (g.font == nil || len(code) == 0 || (!hasCFF && !hasType3)) {
			return
		}
		if g.font != nil && g.font.document != nil && len(code) > 0 {
			if name, proc, resources, ok := g.font.type3CharProcSnapshot(code); ok {
				_, err := g.font.ensureType3CharProcWithError(name)
				if err != nil {
					yield(PathObject{}, err)
					return
				}
				form := newXObjectObject(contentdata.XObjectSpec{
					Stream: proc, Matrix: g.outlineMatrix(), Resources: resources,
				})
				for object, err := range form.Flatten(g.font.document, contentconfig.Options{Filter: FilterPath}) {
					if err != nil {
						if !yield(PathObject{}, err) {
							return
						}
						return
					}
					if object.path != nil && !yield(*object.path, nil) {
						return
					}
				}
				return
			}
		}
		ops := []ContentOp{newContentOpBorrowed("cm", matrixObjects(g.outlineMatrix()), 0)}
		ops = append(ops, g.outlineOpsBorrowed()...)
		for path, err := range valuesSeq(InterpretPaths(ops)) {
			if err != nil {
				if !yield(PathObject{}, err) {
					return
				}
				return
			}
			if !yield(path, nil) {
				return
			}
		}
		if g.font != nil && len(code) > 0 {
			if ops, ok, err := g.font.cffCharStringOpsWithGIDError(code, g.GID()); err != nil {
				if !yield(PathObject{}, err) {
					return
				}
				return
			} else if ok {
				pathOps := []ContentOp{newContentOpBorrowed("cm", matrixObjects(g.outlineMatrix()), 0)}
				pathOps = append(pathOps, ops...)
				for path, err := range valuesSeq(InterpretPaths(pathOps)) {
					if err != nil || !yield(path, err) {
						return
					}
				}
			}
		}
	}
}

// ContentSeq exposes the direct interpreter children of a Type3 glyph.
// Playa GlyphObject iteration is broader than outline extraction: a CharProc
// may emit text, images, tags, or Form XObjects as well as paths. The stream
// and resource snapshots are borrowed only for the duration of each traversal.
func (g GlyphObject) ContentSeq(d *Document) iter.Seq2[ContentObject, error] {
	return func(yield func(ContentObject, error) bool) {
		if d == nil {
			yield(ContentObject{}, errNilDocument)
			return
		}
		code := g.Codes()
		if g.font == nil || !g.font.type3 || len(code) == 0 {
			return
		}
		name, proc, resources, ok := g.font.type3CharProcSnapshot(code)
		if !ok {
			return
		}
		if _, err := g.font.ensureType3CharProcWithError(name); err != nil {
			yield(ContentObject{}, err)
			return
		}
		form := newXObjectObject(contentdata.XObjectSpec{
			Page: g.Page(), HasPage: g.HasPage(), Stream: proc,
			Matrix: g.outlineMatrix(), Resources: resources, GState: g.GState(),
		})
		for object, err := range form.Interp(d, DefaultContentOptions()) {
			if !yield(object, err) {
				return
			}
			if err != nil {
				return
			}
		}
	}
}

// Len counts the direct interpreter children of a Type3 glyph without
// retaining them after traversal, matching Playa's generic Sized contract.
func (g GlyphObject) Len(d *Document) (int, error) {
	count := 0
	for _, err := range g.ContentSeq(d) {
		if err != nil {
			return 0, err
		}
		count++
	}
	return count, nil
}

func matrixObjects(m geometry.Matrix) []Object {
	return []Object{Number(m[0]), Number(m[1]), Number(m[2]), Number(m[3]), Number(m[4]), Number(m[5])}
}

// SegmentsSeq exposes a path object's device-space segments through the same
// repeatable sequence contract as page-level content traversal.
func (p PathObject) SegmentsSeq() iter.Seq2[geometry.PathSegment, error] {
	return func(yield func(geometry.PathSegment, error) bool) {
		for _, segment := range p.SegmentsCopy() {
			if !yield(segment, nil) {
				return
			}
		}
	}
}

func valuesSeq[T any](values []T) iter.Seq2[T, error] {
	return func(yield func(T, error) bool) {
		for _, value := range values {
			if !yield(value, nil) {
				return
			}
		}
	}
}

func structElementsSeq(values []StructElement) iter.Seq2[StructElement, error] {
	return func(yield func(StructElement, error) bool) {
		for _, value := range values {
			if !yield(value, nil) {
				return
			}
		}
	}
}

func outlineNodesSeq(values []OutlineNode) iter.Seq2[OutlineNode, error] {
	return func(yield func(OutlineNode, error) bool) {
		for _, value := range values {
			if !yield(value, nil) {
				return
			}
		}
	}
}

func formFieldsSeq(values []FormField) iter.Seq2[FormField, error] {
	return func(yield func(FormField, error) bool) {
		for _, value := range values {
			if !yield(value, nil) {
				return
			}
		}
	}
}

func markedContentsSeq(values []MarkedContent) iter.Seq2[MarkedContent, error] {
	return func(yield func(MarkedContent, error) bool) {
		for _, value := range values {
			if !yield(value, nil) {
				return
			}
		}
	}
}

func structureContentsSeq(values []StructureContent) iter.Seq2[StructureContent, error] {
	return func(yield func(StructureContent, error) bool) {
		for _, value := range values {
			if !yield(value, nil) {
				return
			}
		}
	}
}

func structureItemsSeq(values []StructureItem) iter.Seq2[StructureItem, error] {
	return func(yield func(StructureItem, error) bool) {
		for _, value := range values {
			if !yield(value, nil) {
				return
			}
		}
	}
}

func structureItemsFromParts(children []StructElement, contents []StructureContent) []StructureItem {
	if len(children) == 0 && len(contents) == 0 {
		return nil
	}
	items := make([]StructureItem, 0, len(children)+len(contents))
	for i := range children {
		child := children[i]
		items = append(items, newStructureElementItem(&child))
	}
	for _, content := range contents {
		kind := structureconfig.ItemMarkedContent
		if content.Kind() == structureconfig.Object {
			kind = structureconfig.ItemObject
		}
		items = append(items, newStructureItem(kind, content))
	}
	return items
}
