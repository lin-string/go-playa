package outline_test

import (
	"testing"

	"github.com/lin-string/go-playa/outline"
)

func TestPublicOutlineSurface(t *testing.T) {
	if outline.ErrPageNotFound == nil {
		t.Fatal("ErrPageNotFound is nil")
	}
	if outline.ErrNilDocument == nil {
		t.Fatal("ErrNilDocument is nil")
	}
	var _ *outline.ParseError
	var _ outline.Object
	var _ outline.Ref
	var _ outline.Dict
	var _ outline.Array
	var _ outline.Null
	var _ outline.Bool
	var _ outline.Number
	var _ outline.Name
	var _ outline.String
	var _ outline.Keyword
	var _ outline.Node
	var _ outline.Page
	var _ outline.StructElement
	assertSeq2[outline.Node]((outline.Node{}).ChildrenSeq())
	childrenCopy, childrenCopyErr := (outline.Node{}).ChildrenCopy()
	_, _ = childrenCopy, childrenCopyErr
	_ = (outline.Node{}).ActionCopy
	_ = (outline.Node{}).TargetCopy
	_ = (outline.Node{}).TargetCopyWithError
	_ = (outline.Node{}).ActionValueCopy
	_ = (outline.Node{}).ActionValueCopyWithError
	_ = (outline.Node{}).DestCopy
	_ = (outline.Node{}).ParentNode
	_ = (outline.Node{}).ParentNodeWithError
	_ = (outline.Node{}).Element
	_ = (outline.Node{}).ElementWithError
	_ = (outline.Node{}).Finalize
	_ = (outline.Node{}).FinalizeWithError
	_ = (outline.Destination{}).Top
	_ = (outline.Destination{}).TopWithError
	_ = (outline.Destination{}).Left
	_ = (outline.Destination{}).LeftWithError
	_ = (outline.Destination{}).Pos
	_ = (outline.Destination{}).PosWithError
	_ = (outline.Destination{}).BBox
	_ = (outline.Destination{}).BBoxWithError
	_ = (outline.Destination{}).Zoom
	_ = (outline.Destination{}).ZoomWithError
	_ = (outline.Destination{}).Finalize
	_ = (outline.Destination{}).ParamsCopy
	var action outline.Action
	_ = action.NextCopy
	_ = action.NextSeq
	_ = (outline.Action{}).DestinationCopy
	_ = (*outline.Action)(nil).FinalizeWithError
	_ = testing.Short
}

func assertSeq2[T any](seq func(func(T, error) bool)) {}
