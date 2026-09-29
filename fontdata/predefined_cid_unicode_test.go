package fontdata

import "testing"

func TestPredefinedCIDUnicodeReturnsIndependentFallbackMap(t *testing.T) {
	first := PredefinedCIDUnicode("Japan1")
	if first == nil || first[2980] == "" {
		t.Fatalf("Japan1 fallback = %#v", first)
	}
	first[2980] = "mutated"
	second := PredefinedCIDUnicode("Japan1")
	if second[2980] == "mutated" {
		t.Fatal("CID Unicode cache exposed mutable backing map")
	}
}

func TestPredefinedCIDUnicodeRejectsUnknownOrdering(t *testing.T) {
	if got := PredefinedCIDUnicode("unknown"); got != nil {
		t.Fatalf("unknown ordering = %#v, want nil", got)
	}
}
