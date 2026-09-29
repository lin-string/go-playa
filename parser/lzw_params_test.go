package parser

import "testing"

func TestDecodeFiltersRejectsInvalidLWZEarlyChange(t *testing.T) {
	if _, err := DecodeFilters(nil, []string{"LZWDecode"}, []Dict{{Name("EarlyChange"): Number(2)}}); err == nil {
		t.Fatal("invalid EarlyChange was accepted")
	}
}
