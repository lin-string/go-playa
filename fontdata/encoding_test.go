package fontdata_test

import (
	"testing"

	"github.com/lin-string/go-playa/fontdata"
)

func TestWinAnsiUsesPDFGlyphAssignments(t *testing.T) {
	encoding, ok := fontdata.BuiltinEncoding("WinAnsiEncoding")
	if !ok {
		t.Fatal("WinAnsiEncoding unavailable")
	}
	for code, want := range map[byte]rune{
		127: '\u2022', 129: '\u2022', 141: '\u2022', 143: '\u2022',
		144: '\u2022', 157: '\u2022', 160: ' ', 173: '-',
		128: '\u20ac', 149: '\u2022', 150: '\u2013', 255: '\u00ff',
	} {
		if got := encoding[code]; got != want {
			t.Errorf("WinAnsi[%d] = %U, want %U", code, got, want)
		}
	}
}

func TestMacExpertUsesPDFExpertSetCodeAssignments(t *testing.T) {
	// ISO 32000-1 Annex D.3 MacExpertEncoding differs from CFF ExpertEncoding.
	for code, want := range map[int]string{
		35: "centoldstyle", 60: ".notdef", 68: "Ethsmall",
		95: "hypheninferior", 129: "asuperior", 190: "AEsmall",
	} {
		if got := fontdata.MacExpertEncoding[code]; got != want {
			t.Errorf("MacExpertEncoding[%d] = %q, want %q", code, got, want)
		}
	}
	if got := fontdata.CFFExpertEncoding[35]; got != 0 {
		t.Errorf("CFF ExpertEncoding[35] = %d, want 0", got)
	}
	if got := fontdata.CFFExpertEncoding[60]; got != 249 {
		t.Errorf("CFF ExpertEncoding[60] = %d, want 249", got)
	}
}

func TestBuiltinEncodingReturnsOwnedAuthoritativeTables(t *testing.T) {
	for _, name := range []string{"StandardEncoding", "MacRomanEncoding", "WinAnsiEncoding", "MacExpertEncoding", "Symbol", "ZapfDingbats"} {
		t.Run(name, func(t *testing.T) {
			encoding, ok := fontdata.BuiltinEncoding(name)
			if !ok || len(encoding) == 0 {
				t.Fatalf("BuiltinEncoding(%q) = %#v, %v", name, encoding, ok)
			}
			original, exists := encoding[32]
			encoding[32] = '\uffff'
			second, secondOK := fontdata.BuiltinEncoding(name)
			if !secondOK || (exists && second[32] != original) || (!exists && second[32] == '\uffff') {
				t.Fatalf("BuiltinEncoding(%q) returned shared mutable state: first=%#v second=%#v", name, encoding, second)
			}
		})
	}
}
