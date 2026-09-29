package fontdata

import "testing"

func TestParseCFF2VariationStoreExposesReadOnlyViews(t *testing.T) {
	data := []byte{
		0, 1, 0, 0, 0, 22, 0, 1, 0, 0, 0, 12,
		0, 1, 0, 1, 0, 1, 0, 0, 0xff, 0xce,
		0, 1, 0, 1, 0, 0, 0x40, 0, 0x40, 0,
	}
	store := ParseCFF2VariationStore(data, 0)
	if store == nil || store.RegionCount() != 1 || store.DataCount() != 1 {
		t.Fatalf("variation store = %#v", store)
	}
	if got := store.RegionScalars([]float64{0.5})[0]; got != 0.5 {
		t.Fatalf("region scalar = %v", got)
	}
	regions, rows, ok := store.Item(0)
	if !ok || len(regions) != 1 || len(rows) != 1 || len(rows[0]) != 1 || rows[0][0] != -50 {
		t.Fatalf("variation item = %#v, %#v, %v", regions, rows, ok)
	}
	regions[0] = 99
	rows[0][0] = 99
	regionsAgain, rowsAgain, ok := store.Item(0)
	if !ok || regionsAgain[0] != 0 || rowsAgain[0][0] != -50 {
		t.Fatalf("variation item was mutable through a view: %#v, %#v", regionsAgain, rowsAgain)
	}
}

func TestParseCFF2VariationStoreRejectsMalformedData(t *testing.T) {
	if store := ParseCFF2VariationStore([]byte{0, 1, 0, 0}, 0); store != nil {
		t.Fatalf("truncated variation store = %#v", store)
	}
}
