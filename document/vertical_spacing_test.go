package document

import "testing"

func TestVerticalTextAppliesCharacterAndWordSpacing(t *testing.T) {
	ops, err := ParseContent([]byte("2 Tc 3 Tw BT /F1 10 Tf (A B) Tj ET"))
	if err != nil {
		t.Fatal(err)
	}
	f := NewSimpleFont("Vertical")
	f.vertical = true
	got := InterpretTextWithFonts(ops, map[string]*Font{"F1": f})
	if len(got) != 1 || len(got[0].glyphs) != 3 {
		t.Fatalf("text = %#v", got)
	}
	if got[0].glyphs[1].Origin()[1] != -8 || got[0].glyphs[2].Origin()[1] != -13 {
		t.Fatalf("vertical origins = %#v", got[0].glyphs)
	}
}
