package document

import (
	"encoding/json"
	"fmt"
	"iter"

	"github.com/lin-string/go-playa/contentdata"
)

// MarkedContent is one BMC/BDC section in content-stream order. Ops contains
// direct operators in the section; nested sections are represented in Children.
type MarkedContent struct {
	data       contentdata.MarkedContent
	opContexts []markedContentOpContext
	children   []MarkedContent
	node       *markedNode
}

func contentOpDataBorrowed(ops []ContentOp) []contentdata.ContentOp {
	if ops == nil {
		return nil
	}
	out := make([]contentdata.ContentOp, len(ops))
	for i, op := range ops {
		out[i] = op.data
	}
	return out
}

// markedContentOpContext keeps interpreter-only operation state out of
// contentdata while preserving the document-level meaning of yielded ops.
type markedContentOpContext struct {
	resources       Dict
	formBoundary    uint8
	propertyName    Name
	hasPropertyName bool
}

func markedContentOpContextsBorrowed(ops []ContentOp) []markedContentOpContext {
	if ops == nil {
		return nil
	}
	out := make([]markedContentOpContext, len(ops))
	for i, op := range ops {
		out[i] = markedContentOpContext{
			resources:       op.resources,
			formBoundary:    op.formBoundary,
			propertyName:    op.propertyName,
			hasPropertyName: op.hasPropertyName,
		}
	}
	return out
}

func markedContentOpContextsCopy(values []markedContentOpContext) []markedContentOpContext {
	if values == nil {
		return nil
	}
	out := make([]markedContentOpContext, len(values))
	for i, value := range values {
		out[i] = value
		out[i].resources = cloneDict(value.resources)
	}
	return out
}

func contentOpFromData(op contentdata.ContentOp, context *markedContentOpContext, cloneContext bool) ContentOp {
	out := ContentOp{data: op}
	if context == nil {
		return out
	}
	out.resources = context.resources
	if cloneContext {
		out.resources = cloneDict(out.resources)
	}
	out.formBoundary = context.formBoundary
	out.propertyName = context.propertyName
	out.hasPropertyName = context.hasPropertyName
	return out
}

func contentOpsFromData(ops []contentdata.ContentOp, contexts []markedContentOpContext, cloneContexts bool) []ContentOp {
	if ops == nil {
		return nil
	}
	out := make([]ContentOp, len(ops))
	for i, op := range ops {
		if i < len(contexts) {
			out[i] = contentOpFromData(op, &contexts[i], cloneContexts)
		} else {
			out[i] = contentOpFromData(op, nil, false)
		}
	}
	return out
}

func newMarkedContentValue(tag string, page Ref, hasPage bool, mcid int, hasMCID bool, actualText string, properties Dict, ops []ContentOp) MarkedContent {
	return MarkedContent{
		data:       contentdata.NewMarkedContentBorrowed(tag, page, hasPage, mcid, hasMCID, actualText, properties, contentOpDataBorrowed(ops)),
		opContexts: markedContentOpContextsBorrowed(ops),
	}
}

// Tag returns the marked-content tag name.
func (m MarkedContent) Tag() string { return m.data.Tag() }

// Page returns the owning page reference, when present.
func (m MarkedContent) Page() Ref { return m.data.Page() }

// HasPage reports whether Page identifies an owning page.
func (m MarkedContent) HasPage() bool { return m.data.HasPage() }

// MCID returns the marked-content identifier, when present.
func (m MarkedContent) MCID() int { return m.data.MCID() }

// HasMCID reports whether MCID is present.
func (m MarkedContent) HasMCID() bool { return m.data.HasMCID() }

// ActualText returns the decoded replacement text, when present.
func (m MarkedContent) ActualText() string { return m.data.ActualText() }

// PageObject resolves the page associated with a page-level marked-content
// section. Standalone ExtractMarkedContent values have no page association.
func (m MarkedContent) PageObject(d *Document) (Page, error) {
	if d == nil {
		return Page{}, errNilDocument
	}
	if !m.HasPage() {
		return Page{}, ErrPageNotFound
	}
	return d.PageByRef(m.Page())
}

// Parent resolves the structure element associated with this marked-content section's MCID.
func (m MarkedContent) Parent(d *Document) *StructElement {
	parent, _ := m.ParentWithError(d)
	return parent
}

// ParentWithError resolves this marked-content section's ParentTree element
// and reports malformed page structure data.
func (m MarkedContent) ParentWithError(d *Document) (*StructElement, error) {
	if d == nil {
		return nil, errNilDocument
	}
	if !m.HasMCID() || !m.HasPage() {
		return nil, nil
	}
	return d.pageContentParentWithError(m.Page(), m.MCID())
}

// MarkedContentIndex provides both the original nesting and O(1)-style lookup
// of all marked sections carrying a given MCID on a page.
type MarkedContentIndex struct {
	roots  []MarkedContent
	byMCID map[int][]MarkedContent
}

// RootsCopy returns independent root marked-content sections.
func (m MarkedContentIndex) RootsCopy() []MarkedContent { return finalizeMarkedContents(m.roots) }

// ByMCIDCopy returns an independent MCID-to-sections index.
func (m MarkedContentIndex) ByMCIDCopy() map[int][]MarkedContent {
	if m.byMCID == nil {
		return nil
	}
	out := make(map[int][]MarkedContent, len(m.byMCID))
	for mcid, items := range m.byMCID {
		out[mcid] = finalizeMarkedContents(items)
	}
	return out
}

// Finalize returns an independent snapshot of the marked-content index.
func (m MarkedContentIndex) Finalize() MarkedContentIndex {
	return MarkedContentIndex{roots: finalizeMarkedContents(m.roots), byMCID: m.ByMCIDCopy()}
}

func finalizeMarkedContents(items []MarkedContent) []MarkedContent {
	if items == nil {
		return nil
	}
	out := make([]MarkedContent, len(items))
	for i, item := range items {
		out[i] = item.Finalize()
	}
	return out
}

type markedNode struct {
	tag           string
	page          Ref
	hasPage       bool
	mcid          int
	hasMCID       bool
	actualText    string
	properties    Dict
	ops           []ContentOp
	ChildrenNodes []*markedNode
}

func (m MarkedContent) PropertiesCopy() Dict { return m.data.PropertiesCopy() }

func (m MarkedContent) OpsCopy() []ContentOp {
	return contentOpsFromData(m.data.OpsCopy(), m.opContexts, true)
}

// ChildrenCopy materializes lazy descendants and returns an independent tree.
func (m MarkedContent) ChildrenCopy() []MarkedContent {
	m.materializeChildren()
	if m.children == nil {
		return nil
	}
	clone := make([]MarkedContent, len(m.children))
	for i, child := range m.children {
		clone[i] = child.Finalize()
	}
	return clone
}

func (m MarkedContent) MarshalJSON() ([]byte, error) {
	m.materializeChildren()
	type projection struct {
		Tag        string
		Page       Ref
		HasPage    bool
		MCID       int
		HasMCID    bool
		ActualText string
		Properties Dict            `json:"Properties"`
		Ops        []ContentOp     `json:"Ops"`
		Children   []MarkedContent `json:"Children"`
	}
	return json.Marshal(projection{
		Tag: m.Tag(), Page: m.Page(), HasPage: m.HasPage(), MCID: m.MCID(), HasMCID: m.HasMCID(),
		ActualText: m.ActualText(), Properties: m.PropertiesCopy(), Ops: m.OpsCopy(), Children: m.children,
	})
}

// ExtractMarkedContent builds the marked-content tree without interpreting
// drawing operators. It preserves direct operator order and parses the
// standard MCID/ActualText properties used by structure extraction.
func ExtractMarkedContent(ops []ContentOp) []MarkedContent {
	return extractMarkedContent(ops, nil)
}

// (d *Document) expandContentPropertiesWithResources resolves property-list
// names against the resource dictionary attached to each operation. Form
// XObjects may define a different Properties resource than their page.
func (d *Document) expandContentPropertiesWithResources(ops []ContentOp, fallback Dict) []ContentOp {
	out := append([]ContentOp(nil), ops...)
	changed := false
	for i, op := range out {
		if op.operatorValue() != "BDC" || len(op.operandsValue()) < 2 {
			continue
		}
		name, ok := op.operandsValue()[1].(Name)
		if !ok {
			continue
		}
		property := Object(fallback[name])
		if op.resources != nil {
			propertiesValue, _ := d.resolveIndirectChain(op.resources[Name("Properties")])
			if raw, ok := propertiesValue.(Dict); ok {
				if value, exists := raw[name]; exists {
					property, _ = d.resolveIndirectChain(value)
				}
			}
		}
		resolved, ok := markedPropertiesDict(property)
		if !ok {
			continue
		}
		resolved = resolveTagProperties(d, resolved)
		out[i].setOperands(append([]Object(nil), op.operandsValue()...))
		out[i].operandsValue()[1] = resolved
		changed = true
	}
	if !changed {
		return ops
	}
	return out
}

func markedPropertiesDict(value Object) (Dict, bool) {
	switch value := value.(type) {
	case Dict:
		return value, true
	case Stream:
		return value.DictBorrowed(), true
	default:
		return nil, false
	}
}

func extractMarkedContent(ops []ContentOp, propertyLists Dict) []MarkedContent {
	roots := []*markedNode{}
	stack := []*markedNode{}
	for _, op := range ops {
		switch op.operatorValue() {
		case "BMC", "BDC":
			if op.operatorValue() == "BDC" && !validMarkedContentData(op, func() Dict { return propertyLists }) {
				continue
			}
			n := &markedNode{}
			if len(op.operandsValue()) > 0 {
				if tag, ok := op.operandsValue()[0].(Name); ok {
					n.tag = string(tag)
				}
			}
			if op.operatorValue() == "BDC" && len(op.operandsValue()) > 1 {
				switch p := op.operandsValue()[1].(type) {
				case Dict:
					n.properties = p
				case Name:
					if propertyLists != nil {
						if resolved, ok := markedPropertiesDict(propertyLists[p]); ok {
							n.properties = resolved
						}
					}
				}
			}
			if n.properties != nil {
				if id, ok := IntValue(n.properties[Name("MCID")]); ok {
					n.mcid, n.hasMCID = id, true
				}
				if text, ok := n.properties[Name("ActualText")].(String); ok {
					n.actualText = decodePDFText(text)
				}
			}
			if len(stack) == 0 {
				roots = append(roots, n)
			} else {
				stack[len(stack)-1].ChildrenNodes = append(stack[len(stack)-1].ChildrenNodes, n)
			}
			stack = append(stack, n)
		case "EMC":
			if len(stack) > 0 {
				stack = stack[:len(stack)-1]
			}
		default:
			if len(stack) > 0 {
				stack[len(stack)-1].ops = append(stack[len(stack)-1].ops, op)
			}
		}
	}
	return markedContentValues(roots)
}

// The public tree uses values; convert the internal pointer tree recursively.
func markedContentValues(nodes []*markedNode) []MarkedContent {
	if nodes == nil {
		return nil
	}
	out := make([]MarkedContent, 0, len(nodes))
	for _, n := range nodes {
		v := newMarkedContentValue(n.tag, n.page, n.hasPage, n.mcid, n.hasMCID, n.actualText, n.properties, n.ops).Finalize()
		v.children = markedContentValues(n.ChildrenNodes)
		out = append(out, v)
	}
	return out
}

func cloneMarkedContent(value MarkedContent) MarkedContent {
	value.data = value.data.Finalize()
	value.opContexts = markedContentOpContextsCopy(value.opContexts)
	return value
}

// Finalize materializes lazy children and returns an independent snapshot.
func (m MarkedContent) Finalize() MarkedContent {
	m.materializeChildren()
	clone := cloneMarkedContent(m)
	clone.node = nil
	for i := range clone.children {
		clone.children[i] = clone.children[i].Finalize()
	}
	return clone
}

func (d *Document) PageMarkedContent(p Page) ([]MarkedContent, error) {
	var out []MarkedContent
	for item, err := range d.markedContentSeq(p) {
		if err != nil {
			return nil, err
		}
		item.materializeChildren()
		out = append(out, item)
	}
	return out, nil
}

// PageMarkedContentByMCIDSeq yields marked-content sections carrying mcid in
// source and nesting order, stopping as soon as the consumer stops.
func (d *Document) PageMarkedContentByMCIDSeq(p Page, mcid int) iter.Seq2[MarkedContent, error] {
	return func(yield func(MarkedContent, error) bool) {
		for item, err := range d.markedContentSeq(p) {
			if err != nil {
				yield(MarkedContent{}, err)
				return
			}
			if !yieldMarkedContentMCID(item, mcid, yield) {
				return
			}
		}
	}
}

func yieldMarkedContentMCID(item MarkedContent, mcid int, yield func(MarkedContent, error) bool) bool {
	if item.HasMCID() && item.MCID() == mcid && !yield(item, nil) {
		return false
	}
	for child, err := range item.ChildrenSeq() {
		if err != nil {
			return false
		}
		if !yieldMarkedContentMCID(child, mcid, yield) {
			return false
		}
	}
	return true
}

// markedContentSeq yields a root section as soon as its EMC is reached. It
// keeps the currently open nesting stack, so later streams are not decoded
// until the consumer asks for another root section.
func (d *Document) markedContentSeq(p Page) iter.Seq2[MarkedContent, error] {
	return func(yield func(MarkedContent, error) bool) {
		iterator := newContentOpIterator(d, p, true)
		var properties Dict
		var propertiesErr error
		propertiesReady := false
		propertiesFor := func() Dict {
			if !propertiesReady {
				properties, propertiesErr = d.pagePropertiesChecked(p)
				propertiesReady = true
			}
			return properties
		}
		stack := []*markedNode{}
		for {
			op, err, ok := iterator.next()
			if err != nil {
				yield(MarkedContent{}, err)
				return
			}
			if !ok {
				for len(stack) > 0 {
					last := len(stack) - 1
					node := stack[last]
					stack = stack[:last]
					value := markedContentValue(node)
					if len(stack) > 0 {
						stack[len(stack)-1].ChildrenNodes = append(stack[len(stack)-1].ChildrenNodes, node)
						continue
					}
					if markedNodeEmpty(node) {
						continue
					}
					if !yield(value, nil) {
						return
					}
				}
				return
			}
			if op.formBoundary != 0 {
				continue
			}
			switch op.operatorValue() {
			case "BMC", "BDC":
				if op.operatorValue() == "BDC" {
					propertiesFor()
					if propertiesErr != nil {
						yield(MarkedContent{}, propertiesErr)
						return
					}
				}
				node, nodeErr := markedNodeForOp(d, op, propertiesFor, p.ref)
				if nodeErr != nil {
					yield(MarkedContent{}, nodeErr)
					return
				}
				if node == nil {
					continue
				}
				stack = append(stack, node)
			case "EMC":
				if len(stack) == 0 {
					continue
				}
				last := len(stack) - 1
				node := stack[last]
				stack = stack[:last]
				if len(stack) > 0 {
					stack[len(stack)-1].ChildrenNodes = append(stack[len(stack)-1].ChildrenNodes, node)
					continue
				}
				if markedNodeEmpty(node) {
					continue
				}
				if !yield(markedContentValue(node), nil) {
					return
				}
			default:
				if len(stack) > 0 {
					stack[len(stack)-1].ops = append(stack[len(stack)-1].ops, op)
				}
			}
		}
	}
}

func markedNodeEmpty(node *markedNode) bool {
	return node.hasMCID && len(node.ops) == 0 && len(node.ChildrenNodes) == 0
}

func markedNodeForOp(d *Document, op ContentOp, properties func() Dict, page Ref) (*markedNode, error) {
	if op.operatorValue() == "BDC" {
		valid, err := validMarkedContentDataForDocument(d, op, properties)
		if err != nil {
			return nil, err
		}
		if !valid {
			return nil, nil
		}
	}
	node := &markedNode{}
	node.page, node.hasPage = page, page != (Ref{})
	if len(op.operandsValue()) > 0 {
		if tag, ok := op.operandsValue()[0].(Name); ok {
			node.tag = string(tag)
		}
	}
	if op.operatorValue() == "BDC" && len(op.operandsValue()) > 1 {
		switch value := op.operandsValue()[1].(type) {
		case Dict:
			node.properties = resolveTagProperties(d, value)
		case Name:
			if props, ok, err := markedPropertiesForOp(d, op, properties, value); err == nil && ok {
				node.properties = props
			} else if err != nil {
				return nil, err
			}
		}
	}
	if node.properties != nil {
		if id, ok := IntValue(node.properties[Name("MCID")]); ok {
			node.mcid, node.hasMCID = id, true
		}
		if text, ok := node.properties[Name("ActualText")].(String); ok {
			node.actualText = decodePDFText(text)
		}
	}
	return node, nil
}

func validMarkedContentData(op ContentOp, properties func() Dict) bool {
	if len(op.operandsValue()) < 2 {
		return false
	}
	if _, ok := op.operandsValue()[1].(Dict); ok {
		return true
	}
	name, ok := op.operandsValue()[1].(Name)
	if !ok {
		return false
	}
	_, ok = markedPropertiesDict(properties()[name])
	return ok
}

func validMarkedContentDataForDocument(d *Document, op ContentOp, properties func() Dict) (bool, error) {
	if len(op.operandsValue()) < 2 {
		return false, nil
	}
	if _, ok := op.operandsValue()[1].(Dict); ok {
		return true, nil
	}
	name, ok := op.operandsValue()[1].(Name)
	if !ok {
		return false, nil
	}
	_, ok, err := markedPropertiesForOp(d, op, properties, name)
	return ok, err
}

func markedPropertiesForOp(d *Document, op ContentOp, fallback func() Dict, name Name) (Dict, bool, error) {
	if op.resources != nil {
		rawProperties, present := op.resources[Name("Properties")]
		propertiesValue, resolved := d.resolveIndirectChain(rawProperties)
		if present && !resolved {
			return nil, false, fmt.Errorf("playa: Properties resources could not be resolved")
		}
		resources, hasProperties := propertiesValue.(Dict)
		if !hasProperties {
			return nil, false, nil
		}
		value, ok := resources[name]
		if !ok {
			return nil, false, nil
		}
		resolvedValue, resolvedOK := d.resolveIndirectChain(value)
		if !resolvedOK {
			return nil, false, fmt.Errorf("playa: Properties resource %q could not be resolved", name)
		}
		dict, ok := markedPropertiesDict(resolvedValue)
		return dict, ok, nil
	}
	dict, ok := markedPropertiesDict(fallback()[name])
	return dict, ok, nil
}

func markedContentValue(node *markedNode) MarkedContent {
	value := newMarkedContentValue(node.tag, node.page, node.hasPage, node.mcid, node.hasMCID, node.actualText, node.properties, node.ops)
	value.node = node
	return value
}

func (m *MarkedContent) materializeChildren() {
	if m == nil || m.node == nil {
		return
	}
	m.children = markedContentValues(m.node.ChildrenNodes)
}

// PageMarkedContentIndex is the association entry point for structure-aware
// consumers. It keeps marked sections in source order and indexes nested
// sections without discarding their properties or ActualText.
func (d *Document) PageMarkedContentIndex(p Page) (MarkedContentIndex, error) {
	roots, err := d.PageMarkedContent(p)
	if err != nil {
		return MarkedContentIndex{}, err
	}
	index := MarkedContentIndex{roots: roots, byMCID: map[int][]MarkedContent{}}
	var walk func([]MarkedContent)
	walk = func(nodes []MarkedContent) {
		for _, node := range nodes {
			if node.HasMCID() {
				index.byMCID[node.MCID()] = append(index.byMCID[node.MCID()], node)
			}
			walk(node.children)
		}
	}
	walk(roots)
	return index, nil
}
