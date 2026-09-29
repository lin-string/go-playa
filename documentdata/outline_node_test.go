package documentdata_test

import (
	"testing"

	"github.com/lin-string/go-playa/documentdata"
	"github.com/lin-string/go-playa/pdftypes/primitives"
)

func TestOutlineNodeOwnsDependencyFreeState(t *testing.T) {
	action := documentdata.NewAction("URI", "https://example.test", "", "", "", primitives.Dict{
		primitives.Name("S"): primitives.Name("URI"),
	})
	target := documentdata.NewDestination(
		primitives.Ref{Object: 7}, true, 0, false, "FitH",
		[]primitives.Object{primitives.Number(12)},
	)
	node := documentdata.NewOutlineNode(documentdata.OutlineNodeSpec{
		Title:       "Chapter 1",
		Parent:      primitives.Ref{Object: 2},
		HasParent:   true,
		Dest:        primitives.Array{primitives.Name("Fit")},
		Target:      &target,
		Action:      primitives.Dict{primitives.Name("S"): primitives.Name("URI")},
		ActionValue: &action,
		ActionKind:  "URI",
		ElementRef:  primitives.Ref{Object: 9},
		HasElement:  true,
		Count:       -1,
		HasCount:    true,
	})
	if node.Title() != "Chapter 1" || !node.HasParent() || node.Parent() != (primitives.Ref{Object: 2}) {
		t.Fatalf("outline identity = %#v", node)
	}
	if !node.HasElement() || node.ElementRef() != (primitives.Ref{Object: 9}) || !node.HasCount() || node.Count() != -1 {
		t.Fatalf("outline relations = %#v", node)
	}
	if got := node.ActionCopy(); got[primitives.Name("S")] != primitives.Name("URI") {
		t.Fatalf("action copy = %#v", got)
	}
	if got := node.TargetCopy(); got == nil || got.View() != "FitH" {
		t.Fatalf("target copy = %#v", got)
	}
	copy := node.Finalize()
	if copy.Title() != node.Title() || copy.ActionValueCopy() == nil {
		t.Fatalf("finalized outline = %#v", copy)
	}
}
