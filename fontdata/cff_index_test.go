package fontdata

import "testing"

func TestParseCFF2Index(t *testing.T) {
	data := []byte{0, 0, 0, 2, 1, 1, 2, 3, 'A', 'B'}
	items, end, ok := ParseCFF2Index(data, 0)
	if !ok || end != len(data) || len(items) != 2 || string(items[0]) != "A" || string(items[1]) != "B" {
		t.Fatalf("CFF2 INDEX = %#v, end=%d, ok=%v", items, end, ok)
	}
}

func TestParseCFFIndexesRejectOverflowingOffsets(t *testing.T) {
	maxInt := int(^uint(0) >> 1)
	if _, _, ok := ParseCFFIndex([]byte{0, 0}, maxInt); ok {
		t.Fatal("CFF INDEX accepted an overflowing offset")
	}
	if _, _, ok := ParseCFF2Index([]byte{0, 0, 0, 0}, maxInt); ok {
		t.Fatal("CFF2 INDEX accepted an overflowing offset")
	}
}
