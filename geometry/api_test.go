package geometry_test

import (
	"math"
	"testing"

	"github.com/lin-string/go-playa/geometry"
)

func TestPublicGeometryValuesPreserveOwnershipAndFiniteGuards(t *testing.T) {
	matrix := geometry.Matrix{2, 0, 0, 3, 10, 20}
	if got := matrix.Mul(geometry.Matrix{1, 0, 0, 1, 5, 7}); got != (geometry.Matrix{2, 0, 0, 3, 20, 41}) {
		t.Fatalf("matrix multiplication = %v", got)
	}
	if _, ok := matrix.MulFinite(geometry.Matrix{math.MaxFloat64, 0, 0, 1, 0, 0}); ok {
		t.Fatal("MulFinite accepted an overflowing result")
	}

	color := geometry.NewColor("DeviceRGB", []float64{0.1, 0.2, 0.3}, "", 3)
	if color.Space() != "DeviceRGB" || color.Pattern() != "" || color.Components() != 3 {
		t.Fatalf("color metadata = space:%q pattern:%q components:%d", color.Space(), color.Pattern(), color.Components())
	}
	pattern := color.WithPattern("P1")
	if pattern.Space() != "DeviceRGB" || pattern.Pattern() != "P1" || color.Pattern() != "" {
		t.Fatalf("pattern copy = %#v, original pattern=%q", pattern, color.Pattern())
	}
	values := color.ValuesCopy()
	values[0] = 1
	if color.ValuesCopy()[0] != 0.1 {
		t.Fatal("color values are not owned")
	}

	segment := geometry.NewPathSegment("m", [2]float64{1, 2})
	points := segment.PointsCopy()
	points[0][0] = 9
	if segment.PointsCopy()[0][0] != 1 {
		t.Fatal("path points are not owned")
	}
}

func TestBBoxRelationsMatchPlayaLTComponentSemantics(t *testing.T) {
	left := bboxFixture{bbox: geometry.Rect{0, 0, 10, 10}}
	right := bboxFixture{bbox: geometry.Rect{12, 2, 20, 8}}
	overlap := bboxFixture{bbox: geometry.Rect{8, 4, 16, 12}}

	if !geometry.IsHoverlap(left.BBox(), overlap) || geometry.IsHoverlap(left.BBox(), right) {
		t.Fatal("horizontal overlap relation is incorrect")
	}
	if got := geometry.HDistance(left.BBox(), right); got != 2 {
		t.Fatalf("horizontal distance = %v, want 2", got)
	}
	if got := geometry.Hoverlap(left.BBox(), overlap); got != 2 {
		t.Fatalf("horizontal overlap distance = %v, want 2", got)
	}
	if !geometry.IsVOverlap(left.BBox(), overlap) || geometry.IsVOverlap(left.BBox(), bboxFixture{bbox: geometry.Rect{2, 12, 8, 20}}) {
		t.Fatal("vertical overlap relation is incorrect")
	}
	if got := geometry.VDistance(left.BBox(), bboxFixture{bbox: geometry.Rect{2, 12, 8, 20}}); got != 2 {
		t.Fatalf("vertical distance = %v, want 2", got)
	}
	if got := geometry.VOverlap(left.BBox(), overlap); got != 6 {
		t.Fatalf("vertical overlap distance = %v, want 6", got)
	}
	if !geometry.IsEmpty(geometry.Rect{0, 0, 0, 1}) || geometry.IsEmpty(geometry.Rect{0, 0, 1, 1}) {
		t.Fatal("empty rectangle semantics are incorrect")
	}
}

type bboxFixture struct{ bbox geometry.Rect }

func (b bboxFixture) BBox() geometry.Rect { return b.bbox }
