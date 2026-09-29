package fontdata_test

import (
	"testing"

	"github.com/lin-string/go-playa/fontdata"
)

func TestMacRomanPDFEncodingCodecExceptions(t *testing.T) {
	encoding, ok := fontdata.BuiltinEncoding("MacRomanEncoding")
	if !ok {
		t.Fatal("missing MacRomanEncoding")
	}
	// ISO 32000-1 Annex D.2 notes 1 and 6: PDF keeps currency at octal
	// 333 and duplicates space at octal 312, unlike modern mac_roman.
	for code, want := range map[byte]rune{202: ' ', 219: '¤'} {
		if got := encoding[code]; got != want {
			t.Errorf("code %d = %U, want %U", code, got, want)
		}
	}
}
