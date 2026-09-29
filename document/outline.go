package document

import (
	"encoding/json"
	"fmt"
	"iter"
	"sync"

	"github.com/lin-string/go-playa/documentdata"
)

const outlineCacheLimit = 32 << 20

type OutlineNode struct {
	data          documentdata.OutlineNode
	target        *Destination
	actionValue   *Action
	children      []OutlineNode
	document      *Document
	childStart    Object
	outlineRef    Ref
	hasOutlineRef bool
	outlinePath   map[Ref]bool
	childrenReady bool
	lazyState     *outlineLazyState
}

func newOutlineValue(spec documentdata.OutlineNodeSpec) OutlineNode {
	return OutlineNode{data: documentdata.NewOutlineNode(spec)}
}

// Title returns the decoded outline title.
func (n OutlineNode) Title() string { return n.data.Title() }

// Parent returns the outline parent reference, when present.
func (n OutlineNode) Parent() Ref { return n.data.Parent() }

// HasParent reports whether Parent identifies an outline parent.
func (n OutlineNode) HasParent() bool { return n.data.HasParent() }

// ActionKind returns the normalized outline action kind.
func (n OutlineNode) ActionKind() string { return n.data.ActionKind() }

// ElementRef returns the associated structure-element reference, when present.
func (n OutlineNode) ElementRef() Ref { return n.data.ElementRef() }

// HasElement reports whether ElementRef identifies a structure element.
func (n OutlineNode) HasElement() bool { return n.data.HasElement() }

// Count returns the outline child-count hint, when present.
func (n OutlineNode) Count() int { return n.data.Count() }

// HasCount reports whether Count was present in the source outline node.
func (n OutlineNode) HasCount() bool { return n.data.HasCount() }

// outlineLazyState is shared by value copies of a borrowed outline node.
// It also records terminal traversal errors so malformed nodes are not
// repeatedly walked by independent callers.
type outlineLazyState struct {
	mu       sync.Mutex
	children []OutlineNode
	ready    bool
	err      error
}

func outlineActionCacheSize(action *Action, depth int) int {
	if action == nil {
		return 0
	}
	if depth <= 0 {
		return 256
	}
	size := 256 + len(action.Kind()) + len(action.URI()) + len(action.File()) + len(action.Name()) + len(action.Script())
	size = addCacheSize(size, nameTreeObjectSize(action.RawCopy(), depth-1))
	for _, next := range action.next {
		size = addCacheSize(size, outlineActionCacheSize(next, depth-1))
	}
	return size
}

func outlineNodeCacheSize(node OutlineNode, depth int) int {
	if depth <= 0 {
		return 512
	}
	size := 512 + len(node.Title()) + len(node.ActionKind())
	size = addCacheSize(size, nameTreeObjectSize(node.data.DestCopy(), depth-1))
	size = addCacheSize(size, nameTreeObjectSize(node.data.ActionCopy(), depth-1))
	size = addCacheSize(size, outlineActionCacheSize(node.actionValue, depth-1))
	if node.target != nil {
		params := node.target.ParamsCopy()
		size = addCacheSize(size, 128+len(node.target.View())+len(params)*16)
		for _, param := range params {
			size = addCacheSize(size, nameTreeObjectSize(param, depth-2))
		}
	}
	for _, child := range node.children {
		size = addCacheSize(size, outlineNodeCacheSize(child, depth-1))
	}
	return size
}

func (d *Document) cachedOutlineNode(ref Ref) (OutlineNode, bool) {
	d.cacheMu.RLock()
	node, ok := d.outlineCache[ref]
	d.cacheMu.RUnlock()
	return node, ok
}

func (d *Document) cacheOutlineNode(ref Ref, node OutlineNode) OutlineNode {
	size := outlineNodeCacheSize(node, 8)
	d.cacheMu.Lock()
	defer d.cacheMu.Unlock()
	if cached, exists := d.outlineCache[ref]; exists {
		return cached
	}
	limit := d.cacheLimits().OutlineBytes
	if !cacheFits(d.outlineCacheBytes, size, limit) {
		return node
	}
	if d.outlineCache == nil {
		d.outlineCache = map[Ref]OutlineNode{}
	}
	if d.outlineCacheSizes == nil {
		d.outlineCacheSizes = map[Ref]int{}
	}
	d.outlineCache[ref] = node
	d.outlineCacheSizes[ref] = size
	d.outlineCacheBytes += size
	return node
}

func (d *Document) updateOutlineCache(ref Ref, node OutlineNode) {
	d.cacheMu.Lock()
	defer d.cacheMu.Unlock()
	oldSize, exists := d.outlineCacheSizes[ref]
	if !exists {
		return
	}
	newSize := outlineNodeCacheSize(node, 8)
	limit := d.cacheLimits().OutlineBytes
	if oldSize > d.outlineCacheBytes || !cacheFits(d.outlineCacheBytes-oldSize, newSize, limit) {
		delete(d.outlineCache, ref)
		delete(d.outlineCacheSizes, ref)
		if oldSize >= d.outlineCacheBytes {
			d.outlineCacheBytes = 0
		} else {
			d.outlineCacheBytes -= oldSize
		}
		return
	}
	d.outlineCache[ref] = node
	d.outlineCacheSizes[ref] = newSize
	d.outlineCacheBytes += newSize - oldSize
}

func cloneOutlineNodes(nodes []OutlineNode) []OutlineNode {
	if nodes == nil {
		return nil
	}
	clone := make([]OutlineNode, len(nodes))
	for i, node := range nodes {
		clone[i] = cloneOutlineNode(node)
	}
	return clone
}

func cloneOutlineNode(node OutlineNode) OutlineNode {
	node.data = node.data.Finalize()
	node.lazyState = nil
	node.target = cloneDestination(node.target)
	node.actionValue = finalizeAction(node.actionValue)
	node.children = cloneOutlineNodes(node.children)
	if node.outlinePath != nil {
		path := make(map[Ref]bool, len(node.outlinePath))
		for ref, present := range node.outlinePath {
			path[ref] = present
		}
		node.outlinePath = path
	}
	return node
}

func cloneOutlineNodeWithError(node OutlineNode) (OutlineNode, error) {
	clone := cloneOutlineNode(node)
	if node.actionValue != nil {
		action, err := node.actionValue.FinalizeWithError()
		if err != nil {
			return OutlineNode{}, err
		}
		clone.actionValue = &action
	}
	for i, child := range node.children {
		copy, err := cloneOutlineNodeWithError(child)
		if err != nil {
			return OutlineNode{}, err
		}
		clone.children[i] = copy
	}
	return clone, nil
}

// ChildrenCopy materializes lazy descendants and returns an independent tree.
func (n OutlineNode) ChildrenCopy() ([]OutlineNode, error) {
	if err := n.materializeChildrenErr(); err != nil {
		return nil, err
	}
	if n.children == nil {
		return nil, nil
	}
	clone := make([]OutlineNode, len(n.children))
	for i, child := range n.children {
		copy, err := child.FinalizeWithError()
		if err != nil {
			return nil, err
		}
		clone[i] = copy
	}
	return clone, nil
}

// ActionCopy returns an independent copy of the original outline action dictionary.
func (n OutlineNode) ActionCopy() Dict { return n.data.ActionCopy() }

// TargetCopy returns an independent outline destination.
func (n OutlineNode) TargetCopy() *Destination {
	if n.target != nil {
		return cloneDestination(n.target)
	}
	target := n.data.TargetCopy()
	if target == nil {
		return nil
	}
	return &Destination{data: *target}
}

// TargetCopyWithError returns an independent outline destination and reports
// malformed destination data while preserving lazy child traversal.
func (n OutlineNode) TargetCopyWithError() (*Destination, error) {
	if n.document == nil {
		return n.TargetCopy(), nil
	}
	value := n.data.DestCopy()
	if value == nil && n.ActionKind() == "GoTo" {
		action := n.data.ActionCopy()
		raw, present := action[Name("D")]
		if !present {
			return cloneDestination(n.target), nil
		}
		if _, indirect := raw.(Ref); indirect {
			return cloneDestination(n.target), nil
		}
		value = raw
	}
	if value == nil {
		return n.TargetCopy(), nil
	}
	target, err := n.document.ResolveDestinationWithError(value)
	if err != nil {
		return nil, err
	}
	return cloneDestination(target), nil
}

// ActionValueCopy returns an independent normalized outline action.
func (n OutlineNode) ActionValueCopy() *Action {
	if n.actionValue != nil {
		return finalizeAction(n.actionValue)
	}
	value := n.data.ActionValueCopy()
	if value == nil {
		return nil
	}
	return &Action{data: *value}
}

// ActionValueCopyWithError returns an independent outline action and reports
// malformed lazy action chains.
func (n OutlineNode) ActionValueCopyWithError() (*Action, error) {
	if n.actionValue == nil {
		return n.ActionValueCopy(), nil
	}
	action, err := n.actionValue.FinalizeWithError()
	if err != nil {
		return nil, err
	}
	return &action, nil
}

// DestCopy returns an independent copy of the raw outline destination.
func (n OutlineNode) DestCopy() Object { return n.data.DestCopy() }

func (n OutlineNode) MarshalJSON() ([]byte, error) {
	if err := n.materializeChildrenErr(); err != nil {
		return nil, err
	}
	type projection struct {
		Title       string
		Parent      Ref
		HasParent   bool
		ActionKind  string
		ElementRef  Ref
		HasElement  bool
		Count       int
		HasCount    bool
		Action      Dict          `json:"Action"`
		Target      *Destination  `json:"Target"`
		ActionValue *Action       `json:"ActionValue"`
		Dest        Object        `json:"Dest"`
		Children    []OutlineNode `json:"Children"`
	}
	return json.Marshal(projection{
		Title: n.Title(), Parent: n.Parent(), HasParent: n.HasParent(), ActionKind: n.ActionKind(),
		ElementRef: n.ElementRef(), HasElement: n.HasElement(), Count: n.Count(), HasCount: n.HasCount(),
		Action: n.ActionCopy(), Target: n.TargetCopy(), ActionValue: n.actionValue, Dest: n.DestCopy(), Children: n.children,
	})
}

// Element resolves the structure element associated with this outline node's
// optional /SE reference.
func (n OutlineNode) Element(d *Document) *StructElement {
	element, _ := n.ElementWithError(d)
	return element
}

// ElementWithError resolves the outline node's optional /SE reference and
// reports malformed or unresolved structure data without changing Element's
// compatibility behavior.
func (n OutlineNode) ElementWithError(d *Document) (*StructElement, error) {
	if d == nil {
		return nil, errNilDocument
	}
	if !n.HasElement() {
		return nil, nil
	}
	for element, err := range d.parentTreeElementsSeq(n.ElementRef()) {
		if err != nil {
			return nil, err
		}
		return &element, nil
	}
	return nil, nil
}

// ParentNode resolves the outline node's /Parent link. The catalog outline
// dictionary is not an outline node and therefore returns nil. Call Finalize
// when an independent materialized snapshot is required.
func (n OutlineNode) ParentNode(d *Document) *OutlineNode {
	parent, _ := n.ParentNodeWithError(d)
	return parent
}

// ParentNodeWithError resolves /Parent and reports malformed parent data.
func (n OutlineNode) ParentNodeWithError(d *Document) (*OutlineNode, error) {
	if d == nil {
		return nil, errNilDocument
	}
	if !n.HasParent() {
		return nil, nil
	}
	parentValue, parentResolved := d.resolveIndirectChain(n.Parent())
	if !parentResolved {
		return nil, fmt.Errorf("playa: outline parent could not be resolved")
	}
	parent, ok := parentValue.(Dict)
	if !ok {
		return nil, fmt.Errorf("playa: outline parent is not a dictionary")
	}
	if _, hasTitle := parent[Name("Title")]; !hasTitle {
		return nil, nil
	}
	value := d.outlineNode(parent, map[Ref]bool{})
	return &value, nil
}

// Outline returns borrowed top-level outline nodes in document order. Child
// outlines remain lazy. Call Finalize when an independent snapshot is needed.
func (d *Document) Outline() iter.Seq2[OutlineNode, error] {
	return func(yield func(OutlineNode, error) bool) {
		d.cacheMu.RLock()
		if d.outlineRootErrReady {
			err := d.outlineRootErr
			d.cacheMu.RUnlock()
			yield(OutlineNode{}, err)
			return
		}
		d.cacheMu.RUnlock()
		first, ok, err := d.outlineFirst()
		if err != nil {
			d.cacheMu.Lock()
			if d.outlineRootErrReady {
				err = d.outlineRootErr
			} else {
				d.outlineRootErr = err
				d.outlineRootErrReady = true
			}
			d.cacheMu.Unlock()
			yield(OutlineNode{}, err)
			return
		}
		if !ok {
			return
		}
		d.outlineSiblingsSeq(first, map[Ref]bool{}, yield)
	}
}

// CollectOutline materializes Outline for adapters that need a complete list.
// Returned nodes retain their document association; call Finalize on a node
// when an independent snapshot is required.
func (d *Document) CollectOutline() ([]OutlineNode, error) {
	var out []OutlineNode
	for node, err := range d.Outline() {
		if err != nil {
			return nil, err
		}
		if err := node.materializeChildrenErr(); err != nil {
			return nil, err
		}
		out = append(out, node)
	}
	return out, nil
}

func (n *OutlineNode) materializeChildren() {
	_ = n.materializeChildrenErr()
}

func (n *OutlineNode) materializeChildrenErr() error {
	if n == nil {
		return nil
	}
	if n.lazyState != nil {
		state := n.lazyState
		state.mu.Lock()
		defer state.mu.Unlock()
		if state.ready {
			n.children = state.children
			n.childrenReady = true
			return state.err
		}
		if n.document == nil || n.childStart == nil {
			state.children = n.children
			state.ready = true
			n.childrenReady = true
			return nil
		}
		children := make([]OutlineNode, 0)
		for child, err := range n.ChildrenSeq() {
			if err != nil {
				state.ready, state.err = true, err
				n.childrenReady = true
				return err
			}
			if err := child.materializeChildrenErr(); err != nil {
				state.ready, state.err = true, err
				n.childrenReady = true
				return err
			}
			children = append(children, child)
		}
		state.children = children
		state.ready = true
		n.children = children
		n.childrenReady = true
		if n.hasOutlineRef {
			n.document.updateOutlineCache(n.outlineRef, *n)
		}
		return nil
	}
	if n.childrenReady {
		return nil
	}
	if n.document == nil || n.childStart == nil {
		n.childrenReady = true
		return nil
	}
	children := make([]OutlineNode, 0)
	for child, err := range n.ChildrenSeq() {
		if err != nil {
			return err
		}
		if err := child.materializeChildrenErr(); err != nil {
			return err
		}
		children = append(children, child)
	}
	n.children = children
	n.childrenReady = true
	if n.hasOutlineRef && n.document != nil {
		n.document.updateOutlineCache(n.outlineRef, *n)
	}
	return nil
}

// Finalize materializes lazy children and returns an independent outline snapshot.
func (n OutlineNode) FinalizeWithError() (OutlineNode, error) {
	if err := n.materializeChildrenErr(); err != nil {
		return OutlineNode{}, err
	}
	clone, err := cloneOutlineNodeWithError(n)
	if err != nil {
		return OutlineNode{}, err
	}
	clone.document = nil
	clone.childStart = nil
	clone.outlineRef = Ref{}
	clone.hasOutlineRef = false
	clone.outlinePath = nil
	clone.childrenReady = true
	return clone, nil
}

func (n OutlineNode) Finalize() OutlineNode {
	clone, _ := n.FinalizeWithError()
	return clone
}

func (d *Document) outlineFirst() (Object, bool, error) {
	root, ok := d.trailer[Name("Root")]
	if !ok {
		return nil, false, nil
	}
	catalogValue, ok := d.resolveIndirectChain(root)
	if !ok {
		return nil, false, fmt.Errorf("playa: catalog root is not a dictionary")
	}
	cat, ok := catalogValue.(Dict)
	if !ok {
		return nil, false, fmt.Errorf("playa: catalog root is not a dictionary")
	}
	raw, present := cat[Name("Outlines")]
	if !present {
		return nil, false, nil
	}
	outlinesValue, resolved := d.resolveIndirectChain(raw)
	o, ok := outlinesValue.(Dict)
	if !resolved {
		ok = false
	}
	if !ok {
		return nil, false, fmt.Errorf("playa: catalog Outlines is not a dictionary")
	}
	first, ok := o[Name("First")]
	if !ok {
		return nil, false, nil
	}
	return first, true, nil
}

func (d *Document) outlineSiblingsSeq(start Object, seen map[Ref]bool, yield func(OutlineNode, error) bool) bool {
	cur := start
	for cur != nil {
		r, has := d.finalIndirectRef(cur)
		if has {
			if seen[r] {
				break
			}
			seen[r] = true
		}
		resolved, resolvedOK := d.resolveIndirectChain(cur)
		if !resolvedOK {
			return yield(OutlineNode{}, fmt.Errorf("playa: outline child could not be resolved"))
		}
		v, ok := resolved.(Dict)
		if !ok {
			return yield(OutlineNode{}, fmt.Errorf("playa: outline node is not a dictionary"))
		}
		node, cached := OutlineNode{}, false
		if has {
			node, cached = d.cachedOutlineNode(r)
		}
		if !cached {
			node = d.outlineNode(v, seen)
			if has {
				node.outlineRef = r
				node.hasOutlineRef = true
				node = d.cacheOutlineNode(r, node)
			}
		} else if node.outlinePath == nil {
			// Older cache entries, or nodes created from a direct dictionary,
			// still need their own ref in the traversal path.
			node.outlinePath = cloneOutlinePath(seen)
		}
		if !yield(node, nil) {
			return false
		}
		next, ok := v[Name("Next")]
		if !ok {
			break
		}
		cur = next
	}
	return true
}

func cloneOutlinePath(source map[Ref]bool) map[Ref]bool {
	if source == nil {
		return nil
	}
	path := make(map[Ref]bool, len(source))
	for ref, present := range source {
		path[ref] = present
	}
	return path
}

func (d *Document) outlineNode(v Dict, seen map[Ref]bool) OutlineNode {
	title := ""
	titleValue, _ := d.resolveIndirectChain(v[Name("Title")])
	if s, ok := titleValue.(String); ok {
		title = decodePDFText(s)
	}
	n := OutlineNode{target: d.ResolveDestination(v[Name("Dest")]), actionValue: d.ResolveAction(v[Name("A")]), document: d, childStart: v[Name("First")], outlinePath: cloneOutlinePath(seen)}
	if n.childStart != nil {
		n.lazyState = &outlineLazyState{}
	}
	var elementRef Ref
	var hasElement bool
	if element, ok := d.finalIndirectRef(v[Name("SE")]); ok {
		elementRef, hasElement = element, true
	}
	var parentRef Ref
	var hasParent bool
	if parent, ok := d.finalIndirectRef(v[Name("Parent")]); ok {
		parentRef, hasParent = parent, true
	}
	actionKind := ""
	actionValue, _ := d.resolveIndirectChain(v[Name("A")])
	if action, ok := actionValue.(Dict); ok {
		actionKindValue, _ := d.resolveIndirectChain(action[Name("S")])
		if kind, ok := actionKindValue.(Name); ok {
			actionKind = string(kind)
		}
		if n.target == nil && actionKind == "GoTo" {
			// Playa passes an outline action's D value directly to its
			// destination factory. An indirect D is therefore rejected, while
			// a direct array or name is still interpreted as a destination.
			if raw, present := action[Name("D")]; present {
				if _, indirect := raw.(Ref); !indirect {
					n.target = d.ResolveDestination(raw)
				}
			}
		}
	}
	countValue, _ := d.resolveIndirectChain(v[Name("Count")])
	count := 0
	hasCount := false
	if parsedCount, ok := IntValue(countValue); ok {
		count, hasCount = parsedCount, true
	}
	var targetData *documentdata.Destination
	if n.target != nil {
		value := n.target.data.Finalize()
		targetData = &value
	}
	var actionData *documentdata.Action
	if n.actionValue != nil {
		value := n.actionValue.data.Finalize()
		actionData = &value
	}
	n.data = documentdata.NewOutlineNode(documentdata.OutlineNodeSpec{
		Title: title, Parent: parentRef, HasParent: hasParent,
		Dest: v[Name("Dest")], Target: targetData, Action: func() Dict {
			if action, ok := actionValue.(Dict); ok {
				return action
			}
			return nil
		}(), ActionValue: actionData, ActionKind: actionKind,
		ElementRef: elementRef, HasElement: hasElement, Count: count, HasCount: hasCount,
	})
	return n
}
