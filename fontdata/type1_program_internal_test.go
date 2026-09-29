package fontdata

import "testing"

func TestParseType1IntegerRejectsOverflow(t *testing.T) {
	if _, _, ok := parseType1Integer([]byte("922337203685477580792233720368"), 0); ok {
		t.Fatal("Type1 integer overflow was accepted")
	}
	if _, _, ok := parseType1Integer([]byte("-922337203685477580792233720368"), 0); ok {
		t.Fatal("negative Type1 integer overflow was accepted")
	}
}

func TestParseType1LengthRejectsOverflow(t *testing.T) {
	if got := parseType1Length([]byte("999999999999999999999999999999 RD")); got >= 0 {
		t.Fatalf("Type1 length overflow was accepted: %d", got)
	}
}
