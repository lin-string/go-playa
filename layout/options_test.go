package layout

import (
	"math"
	"testing"
)

func TestDefaultOptionsReturnsIndependentBoxesFlowPointer(t *testing.T) {
	first := DefaultOptions()
	second := DefaultOptions()
	if first.WordMargin != 0.1 || first.LineMargin != 0.5 || first.LineOverlap != 0.5 || first.CharMargin != 2 || first.BoxesFlow == nil || *first.BoxesFlow != 0.5 {
		t.Fatalf("default layout options = %#v", first)
	}
	if first.BoxesFlow == second.BoxesFlow {
		t.Fatal("default boxes_flow pointers are shared")
	}
}

func TestFiniteThresholdAndBoxesFlowNormalizeInputs(t *testing.T) {
	if got := FiniteThreshold(-1, 2); got != -1 {
		t.Fatalf("FiniteThreshold() = %v, want -1", got)
	}
	if got := FiniteThreshold(math.NaN(), 2); got != 2 {
		t.Fatalf("FiniteThreshold() = %v, want 2", got)
	}
	if got := FiniteThreshold(math.Inf(1), 2); got != 2 {
		t.Fatalf("FiniteThreshold() = %v, want 2", got)
	}
	if got := BoxesFlow(nil); got != 0.5 {
		t.Fatalf("BoxesFlow(nil) = %v, want 0.5", got)
	}
	if got := BoxesFlow(floatPtr(2)); got != 1 {
		t.Fatalf("BoxesFlow(high) = %v, want 1", got)
	}
	if got := BoxesFlow(floatPtr(-2)); got != -1 {
		t.Fatalf("BoxesFlow(low) = %v, want -1", got)
	}
}

func TestWritingMode(t *testing.T) {
	if got := WritingMode(false); got != "lr-tb" {
		t.Fatalf("WritingMode(false) = %q", got)
	}
	if got := WritingMode(true); got != "tb-rl" {
		t.Fatalf("WritingMode(true) = %q", got)
	}
}

func TestComponentOwnsLayoutScalarsAndPreservesReadOnlyRelations(t *testing.T) {
	component := NewComponent(ComponentSpec{
		Text:     "line",
		BBox:     [4]float64{0, 0, 10, 20},
		Vertical: true,
		Index:    3,
	})
	if component.Text() != "line" || component.BBox() != ([4]float64{0, 0, 10, 20}) || !component.Vertical() || component.Index() != 3 {
		t.Fatalf("component metadata = %#v", component)
	}
	if component.WritingMode() != "tb-rl" || component.Width() != 10 || component.Height() != 20 {
		t.Fatalf("component geometry = mode:%q width:%v height:%v", component.WritingMode(), component.Width(), component.Height())
	}
	updated := component.WithText("updated").WithIndex(4)
	if component.Text() != "line" || component.Index() != 3 || updated.Text() != "updated" || updated.Index() != 4 {
		t.Fatalf("component With methods mutated the wrong value: original=%#v updated=%#v", component, updated)
	}
}

func floatPtr(value float64) *float64 { return &value }
