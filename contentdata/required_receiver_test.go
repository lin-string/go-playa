package contentdata

import (
	"reflect"
	"testing"
)

func TestGlyphContextReceiverIsRequired(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("nil GlyphContext receiver did not trigger a programmer error")
		}
	}()
	_ = (*GlyphContext)(nil).SpecBorrowed()
}

func TestZeroGlyphKeepsOptionalContext(t *testing.T) {
	var glyph Glyph
	if spec := glyph.SpecBorrowed(); !reflect.DeepEqual(spec, GlyphSpec{}) {
		t.Fatalf("zero glyph specification = %#v", spec)
	}
	_ = glyph.Finalize()
}
