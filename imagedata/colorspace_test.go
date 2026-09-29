package imagedata_test

import (
	"reflect"
	"testing"

	"github.com/lin-string/go-playa/imagedata"
	"github.com/lin-string/go-playa/pdftypes/primitives"
)

func TestIndexedColorSpaceMakeColorNormalizesIndex(t *testing.T) {
	space := imagedata.NewColorSpace("Indexed", 1).WithHigh(7)
	var got []float64
	for _, value := range []float64{-17, 0, 0.49, 0.5, 1, 6.49, 6.5, 7, 17} {
		color := space.MakeColor(primitives.Number(value))
		components := color.ValuesCopy()
		if len(components) != 1 {
			t.Fatalf("MakeColor(%v) = %#v, want one component", value, components)
		}
		got = append(got, components[0])
	}
	want := []float64{0, 0, 0, 1, 1, 6, 7, 7, 7}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Indexed colors = %v, want %v", got, want)
	}
}

func TestColorSpaceOwnsScalarMetadata(t *testing.T) {
	base := imagedata.NewColorSpace("DeviceRGB", 3)
	space := imagedata.NewColorSpace("Indexed", 1).
		WithBase(base).
		WithColorants([]string{"SpotA", "SpotB"}).
		WithHigh(255).
		WithProfileN(3).
		WithSpec(primitives.Array{primitives.Name("Indexed"), primitives.Name("DeviceRGB"), primitives.Number(255)})

	colorants := space.ColorantsCopy()
	colorants[0] = "changed"
	if space.Name() != "Indexed" || space.Components() != 1 || space.High() != 255 || space.ProfileN() != 3 {
		t.Fatalf("color-space metadata = %#v", space)
	}
	if got := space.ColorantsCopy(); len(got) != 2 || got[0] != "SpotA" {
		t.Fatalf("colorants were not isolated: %#v", got)
	}
	if got, ok := space.BaseCopy(); !ok || got.Name() != "DeviceRGB" {
		t.Fatalf("base color space = %#v, %v", got, ok)
	}
	spec, ok := space.SpecCopy().(primitives.Array)
	if !ok || len(spec) != 3 || spec[0] != primitives.Name("Indexed") {
		t.Fatalf("color-space spec = %#v", space.SpecCopy())
	}
	spec[0] = primitives.Name("changed")
	if got := space.SpecCopy().(primitives.Array)[0]; got != primitives.Name("Indexed") {
		t.Fatalf("color-space spec was not owned: %#v", got)
	}
	if snapshot := space.Finalize(); snapshot.Name() != space.Name() {
		t.Fatalf("finalized color space = %#v", snapshot)
	}
}
