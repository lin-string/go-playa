package contentdata

import (
	"reflect"
	"testing"

	"github.com/lin-string/go-playa/geometry"
	"github.com/lin-string/go-playa/pdftypes/primitives"
)

func TestGraphicsStateOwnsMutableValues(t *testing.T) {
	dash := []float64{1, 2}
	blendModes := []string{"Multiply", "Screen"}
	mask := primitives.Dict{primitives.Name("Type"): primitives.Name("Mask")}
	state := NewGraphicsState(GraphicsStateValues{
		CTM: geometry.Matrix{1, 0, 0, 1, 10, 20}, Dash: dash, BlendModes: blendModes,
		StrokeColor: geometry.NewColor("DeviceRGB", []float64{.1, .2, .3}, "", 3),
		FillColor:   geometry.NewColor("Pattern", []float64{.5}, "P1", 1),
		SoftMask:    mask, HasSoftMask: true,
	})
	dash[0] = 9
	blendModes[0] = "Normal"
	mask[primitives.Name("Type")] = primitives.Name("Changed")

	if got := state.DashCopy(); !reflect.DeepEqual(got, []float64{1, 2}) {
		t.Fatalf("DashCopy() = %#v", got)
	}
	if got := state.BlendModesCopy(); !reflect.DeepEqual(got, []string{"Multiply", "Screen"}) {
		t.Fatalf("BlendModesCopy() = %#v", got)
	}
	if got := state.StrokeColor().ValuesCopy(); !reflect.DeepEqual(got, []float64{.1, .2, .3}) {
		t.Fatalf("StrokeColor() = %#v", got)
	}
	got, ok := state.SoftMaskCopy()
	if !ok || got.(primitives.Dict)[primitives.Name("Type")] != primitives.Name("Mask") {
		t.Fatalf("SoftMaskCopy() = %#v, %v", got, ok)
	}

	clone := state.Finalize()
	if got := clone.DashCopy(); !reflect.DeepEqual(got, []float64{1, 2}) {
		t.Fatalf("Finalize DashCopy() = %#v", got)
	}
}

func TestGraphicsStateDefaultIsStable(t *testing.T) {
	state := DefaultGraphicsState()
	if state.LineWidth() != 1 || state.MiterLimit() != 10 || state.Alpha() != 1 || !state.Knockout() {
		t.Fatalf("unexpected defaults: %#v", state)
	}
}

func TestGraphicsStateWithCTMSharesImmutableState(t *testing.T) {
	state := NewGraphicsState(GraphicsStateValues{
		CTM: geometry.Matrix{1, 0, 0, 1, 10, 20}, Dash: []float64{1, 2},
		BlendMode: "Multiply", ClipDepth: 3,
	})
	derived := state.WithCTM(geometry.Matrix{2, 0, 0, 2, 30, 40})
	if derived.CTM() != (geometry.Matrix{2, 0, 0, 2, 30, 40}) ||
		!reflect.DeepEqual(derived.DashCopy(), []float64{1, 2}) ||
		derived.BlendMode() != "Multiply" || derived.ClipDepth() != 3 {
		t.Fatalf("derived graphics state lost shared values: %#v", derived)
	}
	clone := derived.Finalize()
	if clone.CTM() != derived.CTM() || !reflect.DeepEqual(clone.DashCopy(), derived.DashCopy()) {
		t.Fatalf("finalized derived state changed values: %#v", clone)
	}
}
