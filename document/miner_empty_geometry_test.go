package document

import (
	"testing"

	"github.com/lin-string/go-playa/contentdata"
	"github.com/lin-string/go-playa/geometry"
	"github.com/lin-string/go-playa/layout"
)

func TestLayoutKeepsZeroWidthTextOutsideBoxes(t *testing.T) {
	objects := []TextObject{{glyphs: []GlyphObject{
		newTestGlyph(contentdata.GlyphSpec{Text: "\u0338", Origin: [2]float64{10, 20}, BBox: [4]float64{10, 10, 10, 20}}, nil, nil, geometry.Matrix{}),
		newTestGlyph(contentdata.GlyphSpec{Text: "A", Origin: [2]float64{30, 20}, BBox: [4]float64{30, 10, 36, 20}}, nil, nil, geometry.Matrix{}),
	}}}
	result := AnalyzeLayout(objects, layout.Options{})
	boxes := result.TextBoxesCopy()
	if len(boxes) != 1 || boxes[0].Text() != "A" {
		t.Fatalf("boxes=%v, want only A", boxes)
	}
	lines := result.LinesCopy()
	if len(lines) != 2 || lines[0].Text() != "A" || lines[1].Text() != "\u0338" {
		t.Fatalf("lines=%v, want A then zero-width overlay", lines)
	}
}

func TestLayoutExcludesWhitespaceMadeFromEmptyGlyphs(t *testing.T) {
	objects := []TextObject{{glyphs: []GlyphObject{
		newTestGlyph(contentdata.GlyphSpec{Origin: [2]float64{10, 20}, BBox: [4]float64{10, 10, 16, 20}}, nil, nil, geometry.Matrix{}),
		newTestGlyph(contentdata.GlyphSpec{Origin: [2]float64{18, 20}, BBox: [4]float64{18, 10, 24, 20}}, nil, nil, geometry.Matrix{}),
	}}}
	result := AnalyzeLayout(objects, layout.Options{})
	if lines := result.LinesCopy(); len(lines) != 1 || lines[0].Text() != " " {
		t.Fatalf("lines=%v, want one whitespace line", lines)
	}
	if boxes := result.TextBoxesCopy(); len(boxes) != 0 {
		t.Fatalf("boxes=%v, want whitespace excluded", boxes)
	}
}
