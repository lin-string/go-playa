package contentdata_test

import (
	"testing"

	"github.com/lin-string/go-playa/contentdata"
	"github.com/lin-string/go-playa/geometry"
	"github.com/lin-string/go-playa/pdftypes/primitives"
)

func TestGlyphOwnsDependencyFreeState(t *testing.T) {
	code := []byte{1, 2}
	glyph := contentdata.NewGlyph(contentdata.GlyphSpec{
		Text: "A", Chars: "A", Code: code, CID: 3, GID: 4, FontName: "F1",
		Matrix: geometry.Matrix{1, 0, 0, 1, 2, 3}, BBox: [4]float64{1, 2, 3, 4},
		Page: primitives.Ref{Object: 7}, HasPage: true,
	})
	code[0] = 9
	if glyph.Text() != "A" || glyph.CID() != 3 || glyph.GID() != 4 || !glyph.HasPage() || glyph.Page() != (primitives.Ref{Object: 7}) {
		t.Fatalf("glyph metadata = %#v", glyph)
	}
	if got := glyph.CodeCopy(); got[0] != 1 {
		t.Fatalf("glyph retained caller code = %v", got)
	}
	copy := glyph.CodeCopy()
	copy[0] = 8
	if glyph.CodeCopy()[0] != 1 {
		t.Fatal("glyph code copy exposed backing storage")
	}
	if glyph.Finalize().BBox() != [4]float64{1, 2, 3, 4} {
		t.Fatal("glyph finalize lost geometry")
	}
}

func TestGlyphSpecBorrowedProjectsWithoutCopying(t *testing.T) {
	code := []byte{1, 2}
	stack := []contentdata.MarkedContentContext{
		contentdata.NewMarkedContentContext("Span", primitives.Dict{primitives.Name("Lang"): primitives.String("en")}, "", 7, true),
	}
	glyph := contentdata.NewGlyphBorrowed(contentdata.GlyphSpec{
		Text: "A", Chars: "A", Code: code, CID: 3, GID: 4,
		Page: primitives.Ref{Object: 9}, HasPage: true, MCID: 7, HasMCID: true,
		BBox: [4]float64{1, 2, 3, 4}, MarkedStack: stack,
	})
	spec := glyph.SpecBorrowed()
	if spec.Text != "A" || spec.Page != (primitives.Ref{Object: 9}) || spec.MCID != 7 || spec.BBox != [4]float64{1, 2, 3, 4} || len(spec.MarkedStack) != 1 {
		t.Fatalf("borrowed spec lost glyph state: %#v", spec)
	}
	spec.Code[0] = 8
	if got := glyph.CodeCopy(); got[0] != 8 {
		t.Fatalf("borrowed spec copied code: %v", got)
	}
	owned := glyph.Finalize()
	spec.Code[0] = 9
	if got := owned.CodeCopy(); got[0] != 8 {
		t.Fatalf("finalized glyph retained borrowed code: %v", got)
	}
}

func TestGlyphsShareBorrowedContextAndFinalizeOwnsIt(t *testing.T) {
	properties := primitives.Dict{primitives.Name("Lang"): primitives.String("en")}
	stack := []contentdata.MarkedContentContext{
		contentdata.NewMarkedContentContextBorrowed("Span", properties, "", 7, true),
	}
	context := contentdata.NewGlyphContextBorrowed(contentdata.GlyphContextSpec{
		Page: primitives.Ref{Object: 9}, HasPage: true,
		MCID: 7, HasMCID: true, FontName: "F1", FontSize: 12, Size: 12,
		FontBase: "Helvetica", TextFont: "Helvetica-12", Vertical: true,
		GState: contentdata.DefaultGraphicsState(), MarkedStack: stack,
	})
	code := []byte{1}
	first := contentdata.NewGlyphBorrowedWithContext(contentdata.GlyphSpec{Text: "A", Code: code}, context)
	second := contentdata.NewGlyphBorrowedWithContext(contentdata.GlyphSpec{Text: "B", Code: []byte{2}}, context)
	if first.ContextBorrowed() != second.ContextBorrowed() {
		t.Fatal("glyphs did not retain the shared context")
	}
	if first.Page() != (primitives.Ref{Object: 9}) || first.MCID() != 7 || first.FontName() != "F1" || !first.Vertical() {
		t.Fatalf("shared context metadata = %#v", first.SpecBorrowed())
	}

	owned := first.Finalize()
	code[0] = 8
	properties[primitives.Name("Lang")] = primitives.String("fr")
	if got := owned.CodeCopy(); got[0] != 1 {
		t.Fatalf("finalized glyph retained borrowed code: %v", got)
	}
	ownedStack := owned.MarkedStackCopy()
	if got := ownedStack[0].PropertiesCopy()[primitives.Name("Lang")]; string(got.(primitives.String)) != "en" {
		t.Fatalf("finalized glyph retained borrowed marked-content properties: %v", got)
	}
}
