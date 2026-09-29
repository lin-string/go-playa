package document

import "testing"

func TestInvisibleRenderModeExcludesGlyphsFromVisualBBox(t *testing.T) {
	ops, err := ParseContent([]byte("3 Tr BT /F1 10 Tf (hidden) Tj ET"))
	if err != nil {
		t.Fatal(err)
	}
	got := InterpretText(ops)
	if visual, hasVisual := got[0].VisualBBoxCopy(); len(got) != 1 || len(got[0].glyphs) != 6 || hasVisual || visual != ([4]float64{}) || !got[0].Invisible() {
		t.Fatalf("invisible text = %#v", got)
	}
	for _, glyph := range got[0].glyphs {
		if !glyph.Invisible() {
			t.Fatalf("glyph was not marked invisible: %#v", got[0].glyphs)
		}
	}
}
