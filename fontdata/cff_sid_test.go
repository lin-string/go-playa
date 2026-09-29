package fontdata

import "testing"

func TestCFFSIDNameResolvesStandardAndCustomStrings(t *testing.T) {
	if got, ok := CFFSIDName(1, nil); !ok || got != "space" {
		t.Fatalf("standard CFF SID = %q, %v", got, ok)
	}
	if got, ok := CFFSIDName(len(CFFStandardStrings), [][]byte{[]byte("custom")}); !ok || got != "custom" {
		t.Fatalf("custom CFF SID = %q, %v", got, ok)
	}
	if _, ok := CFFSIDName(len(CFFStandardStrings)+1, nil); ok {
		t.Fatal("missing custom CFF SID was accepted")
	}
}
