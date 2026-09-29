package documentdata

import (
	"encoding/json"

	"github.com/lin-string/go-playa/pdftypes/primitives"
)

// OutlineNode is the document-independent value portion of one PDF outline
// item. Child traversal and document-bound page/structure resolution remain
// owned by document.OutlineNode.
type OutlineNode struct {
	title       string
	parent      primitives.Ref
	hasParent   bool
	dest        primitives.Object
	target      *Destination
	action      primitives.Dict
	actionValue *Action
	actionKind  string
	elementRef  primitives.Ref
	hasElement  bool
	count       int
	hasCount    bool
}

// OutlineNodeSpec supplies the dependency-free fields used to construct an
// owned outline value. The constructor copies all recursive PDF values.
type OutlineNodeSpec struct {
	Title       string
	Parent      primitives.Ref
	HasParent   bool
	Dest        primitives.Object
	Target      *Destination
	Action      primitives.Dict
	ActionValue *Action
	ActionKind  string
	ElementRef  primitives.Ref
	HasElement  bool
	Count       int
	HasCount    bool
}

// NewOutlineNode constructs an owned dependency-free outline value.
func NewOutlineNode(spec OutlineNodeSpec) OutlineNode {
	return OutlineNode{
		title:       spec.Title,
		parent:      spec.Parent,
		hasParent:   spec.HasParent,
		dest:        cloneObject(spec.Dest),
		target:      cloneDestination(spec.Target),
		action:      cloneDict(spec.Action),
		actionValue: cloneAction(spec.ActionValue),
		actionKind:  spec.ActionKind,
		elementRef:  spec.ElementRef,
		hasElement:  spec.HasElement,
		count:       spec.Count,
		hasCount:    spec.HasCount,
	}
}

func (n OutlineNode) Title() string              { return n.title }
func (n OutlineNode) Parent() primitives.Ref     { return n.parent }
func (n OutlineNode) HasParent() bool            { return n.hasParent }
func (n OutlineNode) ActionKind() string         { return n.actionKind }
func (n OutlineNode) ElementRef() primitives.Ref { return n.elementRef }
func (n OutlineNode) HasElement() bool           { return n.hasElement }
func (n OutlineNode) Count() int                 { return n.count }
func (n OutlineNode) HasCount() bool             { return n.hasCount }

// ActionCopy returns an independent copy of the original outline action
// dictionary.
func (n OutlineNode) ActionCopy() primitives.Dict { return cloneDict(n.action) }

// TargetCopy returns an independent normalized outline destination.
func (n OutlineNode) TargetCopy() *Destination { return cloneDestination(n.target) }

// ActionValueCopy returns an independent normalized outline action.
func (n OutlineNode) ActionValueCopy() *Action { return cloneAction(n.actionValue) }

// DestCopy returns an independent copy of the raw outline destination.
func (n OutlineNode) DestCopy() primitives.Object { return cloneObject(n.dest) }

// Finalize returns an independent outline value snapshot.
func (n OutlineNode) Finalize() OutlineNode {
	return NewOutlineNode(OutlineNodeSpec{
		Title: n.title, Parent: n.parent, HasParent: n.hasParent,
		Dest: n.dest, Target: n.target, Action: n.action,
		ActionValue: n.actionValue, ActionKind: n.actionKind,
		ElementRef: n.elementRef, HasElement: n.hasElement,
		Count: n.count, HasCount: n.hasCount,
	})
}

func (n OutlineNode) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Title       string
		Parent      primitives.Ref
		HasParent   bool
		ActionKind  string
		ElementRef  primitives.Ref
		HasElement  bool
		Count       int
		HasCount    bool
		Action      primitives.Dict   `json:"Action"`
		Target      *Destination      `json:"Target"`
		ActionValue *Action           `json:"ActionValue"`
		Dest        primitives.Object `json:"Dest"`
	}{
		Title: n.title, Parent: n.parent, HasParent: n.hasParent,
		ActionKind: n.actionKind, ElementRef: n.elementRef,
		HasElement: n.hasElement, Count: n.count, HasCount: n.hasCount,
		Action: n.action, Target: n.target, ActionValue: n.actionValue,
		Dest: n.dest,
	})
}

func cloneDestination(value *Destination) *Destination {
	if value == nil {
		return nil
	}
	clone := value.Finalize()
	return &clone
}

func cloneAction(value *Action) *Action {
	if value == nil {
		return nil
	}
	clone := value.Finalize()
	return &clone
}
