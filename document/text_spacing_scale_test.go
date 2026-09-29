package document

import "testing"

func TestTextSpacingUsesHorizontalScale(t *testing.T) {
	ops, err := ParseContent([]byte("BT /F1 10 Tf 50 Tz 2 Tc 3 Tw (A B) Tj ET"))
	if err != nil {
		t.Fatal(err)
	}
	got := InterpretText(ops)
	if len(got) != 1 || len(got[0].glyphs) != 3 {
		t.Fatalf("text = %#v", got)
	}
	// Default glyph width is 2.5 points at this font size, then Tc is
	// scaled to 1 point; the space also receives scaled word spacing.
	if got[0].glyphs[1].Origin()[0] != 3.5 || got[0].glyphs[2].Origin()[0] != 8.5 {
		t.Fatalf("glyph origins = %#v", got[0].glyphs)
	}
}
