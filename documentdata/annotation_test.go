package documentdata

import (
	"testing"

	"github.com/lin-string/go-playa/pdftypes/primitives"
)

func TestAnnotationOwnsDependencyFreeSnapshot(t *testing.T) {
	dict := primitives.Dict{primitives.Name("Contents"): primitives.String("original")}
	action := primitives.Dict{primitives.Name("S"): primitives.Name("URI")}
	appearance := primitives.Dict{primitives.Name("N"): primitives.Name("Appearance")}
	quadPoints := [][2]float64{{1, 2}}
	color := []float64{0.1, 0.2, 0.3}
	value := NewAnnotation(AnnotationSpec{
		Subtype: "Link", Dict: dict, Action: action, Appearance: appearance,
		QuadPoints: quadPoints, Color: color, Rect: [4]float64{1, 2, 3, 4},
	})

	dict[primitives.Name("Contents")] = primitives.String("changed")
	action[primitives.Name("S")] = primitives.Name("JavaScript")
	appearance[primitives.Name("N")] = primitives.Name("changed")
	quadPoints[0][0] = 99
	color[0] = 99

	if value.Subtype() != "Link" || value.Rect() != [4]float64{1, 2, 3, 4} {
		t.Fatalf("scalar annotation metadata changed: %#v", value)
	}
	if got, _ := value.DictCopy()[primitives.Name("Contents")].(primitives.String); string(got) != "original" {
		t.Fatalf("DictCopy() contents = %q, want original", got)
	}
	if got, _ := value.ActionCopy()[primitives.Name("S")].(primitives.Name); got != primitives.Name("URI") {
		t.Fatalf("ActionCopy() subtype = %q, want URI", got)
	}
	if got := value.QuadPointsCopy()[0][0]; got != 1 {
		t.Fatalf("QuadPointsCopy() x = %v, want 1", got)
	}
	if got := value.ColorCopy()[0]; got != 0.1 {
		t.Fatalf("ColorCopy() first component = %v, want 0.1", got)
	}

	page := value.WithPage(primitives.Ref{Object: 7}, true)
	if got, ok := page.Page(); !ok || got.Object != 7 || !page.HasPage() {
		t.Fatalf("WithPage() = %v, %v", got, ok)
	}
	if value.HasPage() {
		t.Fatal("WithPage() mutated the source value")
	}
	finalized := value.Finalize()
	if finalized.Subtype() != "Link" || finalized.Rect() != value.Rect() {
		t.Fatalf("Finalize() = %#v", finalized)
	}
}
