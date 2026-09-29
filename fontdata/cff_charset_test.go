package fontdata

import "testing"

func TestParseCFFCharsetNamesParsesCustomAndPredefinedCharsets(t *testing.T) {
	custom := []byte{0, 0, 0, 0, 1, 135}
	if got := ParseCFFCharsetNames(custom, 3, 2, [][]byte{[]byte("A")}); got[1] != "A" {
		t.Fatalf("custom CFF charset = %#v", got)
	}

	predefined := ParseCFFCharsetNames(nil, 0, 2, nil)
	if len(predefined) != 2 || predefined[1] != "space" {
		t.Fatalf("predefined CFF charset = %#v", predefined)
	}
}

func TestParseCFFCharsetNamesRejectsMalformedRanges(t *testing.T) {
	data := []byte{0, 0, 0, 1, 0, 2}
	if got := ParseCFFCharsetNames(data, 3, 3, nil); got != nil {
		t.Fatalf("oversized CFF charset range = %#v", got)
	}
}
