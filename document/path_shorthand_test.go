package document

import (
	"reflect"
	"testing"
)

func TestPathPreservesShorthandCubicSegments(t *testing.T) {
	ops, err := ParseContent([]byte("2 0 0 3 10 20 cm 1 2 m 3 4 5 6 v 7 8 9 10 y 11 12 13 14 15 16 c S"))
	if err != nil {
		t.Fatal(err)
	}
	for run := 0; run < 2; run++ {
		paths := InterpretPaths(ops)
		if len(paths) != 1 {
			t.Fatalf("paths=%d", len(paths))
		}
		raw, device := paths[0].Finalize().RawSegmentsCopy(), paths[0].SegmentsCopy()
		for i, op := range []string{"m", "v", "y", "c"} {
			if raw[i].Operator() != op || device[i].Operator() != op {
				t.Fatalf("segment%d=%s/%s want%s", i, raw[i].Operator(), device[i].Operator(), op)
			}
		}
		if got := raw[1].PointsCopy(); !reflect.DeepEqual(got, [][2]float64{{3, 4}, {5, 6}}) {
			t.Fatalf("v points=%v", got)
		}
		if got := device[2].PointsCopy(); !reflect.DeepEqual(got, [][2]float64{{24, 44}, {28, 50}}) {
			t.Fatalf("device y points=%v", got)
		}
		if got := paths[0].BBox(); got != [4]float64{12, 26, 40, 68} {
			t.Fatalf("bbox=%v", got)
		}
	}
}
