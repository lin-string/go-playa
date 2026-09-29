package fontdata_test

import (
	"testing"

	"github.com/lin-string/go-playa/fontdata"
)

func TestParseCFFCharStringWidthUsesDefaultAndExplicitWidth(t *testing.T) {
	if got, ok := fontdata.ParseCFFCharStringWidth([]byte{139, 139, 21, 14}, 100, 500, nil, nil); !ok || got != 500 {
		t.Fatalf("default width = %v, %v", got, ok)
	}
	if got, ok := fontdata.ParseCFFCharStringWidth([]byte{149, 139, 139, 21, 14}, 100, 500, nil, nil); !ok || got != 110 {
		t.Fatalf("explicit width = %v, %v", got, ok)
	}
}

func TestParseCFFCharStringWidthFollowsLocalSubr(t *testing.T) {
	local := [][]byte{{139, 139, 21, 11}}
	data := []byte{139, 10, 14}
	if got, ok := fontdata.ParseCFFCharStringWidth(data, 100, 500, local, nil); !ok || got != 500 {
		t.Fatalf("subroutine width = %v, %v", got, ok)
	}
}

func TestParseCFF2CharStringWidthEvaluatesBlendAndVSIndex(t *testing.T) {
	store := testCFF2VariationStore{
		regions: []testCFF2Region{{start: []float64{0}, peak: []float64{1}, end: []float64{1}}},
		items: []testCFF2Item{
			{regions: []int{0}, rows: [][]float64{{100}}},
			{regions: []int{0}, rows: [][]float64{{200}}},
		},
	}
	// Select item 1, then blend 500 + 200 at the positive variation limit.
	data := []byte{140, 15, 248, 136, 247, 92, 140, 16, 139, 22}
	width, ok := fontdata.ParseCFF2CharStringWidth(data, 0, 0, nil, nil, store, []float64{1}, 0)
	if !ok || width != 700 {
		t.Fatalf("CFF2 blended width = %v, %v; want 700, true", width, ok)
	}
}

func TestParseCFF2CharStringWidthRejectsMultipleVSIndexOperators(t *testing.T) {
	store := testCFF2VariationStore{
		regions: []testCFF2Region{{start: []float64{0}, peak: []float64{1}, end: []float64{1}}},
		items:   []testCFF2Item{{}, {}},
	}
	data := []byte{139, 15, 140, 15, 139, 22}
	if _, ok := fontdata.ParseCFF2CharStringWidth(data, 0, 500, nil, nil, store, []float64{1}, 0); ok {
		t.Fatal("CFF2 width parser accepted multiple vsindex operators")
	}
}

type testCFF2VariationStore struct {
	regions []testCFF2Region
	items   []testCFF2Item
}

type testCFF2Region struct {
	start []float64
	peak  []float64
	end   []float64
}

type testCFF2Item struct {
	regions []int
	rows    [][]float64
}

func (store testCFF2VariationStore) DataCount() int { return len(store.items) }

func (store testCFF2VariationStore) Item(index int) ([]int, [][]float64, bool) {
	if index < 0 || index >= len(store.items) {
		return nil, nil, false
	}
	item := store.items[index]
	return append([]int(nil), item.regions...), item.rows, true
}

func (store testCFF2VariationStore) RegionScalars(coords []float64) []float64 {
	result := make([]float64, len(store.regions))
	for index, region := range store.regions {
		coord := 0.0
		if len(coords) > 0 {
			coord = coords[0]
		}
		if coord < region.start[0] || coord > region.end[0] {
			continue
		}
		if coord <= region.peak[0] {
			result[index] = (coord - region.start[0]) / (region.peak[0] - region.start[0])
		} else {
			result[index] = (region.end[0] - coord) / (region.end[0] - region.peak[0])
		}
	}
	return result
}
