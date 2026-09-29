package document

import "testing"

func TestType3WidthCodeUsesCharProcWidth(t *testing.T) {
	f := NewSimpleFont("Type3")
	f.type3 = true
	f.glyphNames[65] = "A"
	f.charWidths["A"] = 742
	if got := f.WidthCode([]byte{65}); got != 742 {
		t.Fatalf("Type3 width = %v, want 742", got)
	}
}

func TestType3WidthCodeFallsBackWhenCharProcWidthMissing(t *testing.T) {
	f := NewSimpleFont("Type3")
	f.type3 = true
	f.glyphNames[65] = "A"
	f.widths[65] = 610
	if got := f.WidthCode([]byte{65}); got != 610 {
		t.Fatalf("Type3 fallback width = %v, want 610", got)
	}
}
