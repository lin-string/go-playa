package document

import (
	"testing"

	"github.com/lin-string/go-playa/contentdata"
	"github.com/lin-string/go-playa/documentdata"
	"github.com/lin-string/go-playa/geometry"
)

func TestModelIteratorsRequireFinalizeForMutableChildren(t *testing.T) {
	outline := OutlineNode{children: []OutlineNode{newOutlineValue(documentdata.OutlineNodeSpec{Title: "original"})}, childrenReady: true}
	for child, err := range outline.ChildrenSeq() {
		if err != nil {
			t.Fatal(err)
		}
		snapshot := child.Finalize()
		snapshot.data = documentdata.NewOutlineNode(documentdata.OutlineNodeSpec{Title: "changed"})
	}
	if outline.children[0].Title() != "original" {
		t.Fatal("outline children iterator exposed source values")
	}

	field := FormField{kids: []FormField{{data: documentdata.NewFormField(documentdata.FormFieldSpec{Name: "original"})}}, kidsReady: true}
	for child, err := range field.KidsSeq() {
		if err != nil {
			t.Fatal(err)
		}
		snapshot := child.Finalize()
		if snapshot.Name() != "original" {
			t.Fatal("form child snapshot lost its name")
		}
	}
	if field.kids[0].Name() != "original" {
		t.Fatal("form children iterator exposed source values")
	}

	marked := MarkedContent{children: []MarkedContent{newMarkedContentValue("", Ref{}, false, 0, false, "", Dict{Name("Value"): String("original")}, nil)}}
	for child, err := range marked.ChildrenSeq() {
		if err != nil {
			t.Fatal(err)
		}
		snapshot := child.Finalize()
		snapshotProperties := snapshot.PropertiesCopy()
		snapshotProperties[Name("Value")] = String("changed")
	}
	if string(marked.children[0].PropertiesCopy()[Name("Value")].(String)) != "original" {
		t.Fatal("marked children iterator exposed source values")
	}
}

func TestGlyphAndPathIteratorsDoNotShareSlices(t *testing.T) {
	text := TextObject{glyphs: []GlyphObject{newTestGlyph(contentdata.GlyphSpec{Code: []byte{1}, GState: newGraphicsState().publicValue()}, nil, nil, geometry.Matrix{})}}
	for glyph, err := range text.GlyphsSeq() {
		if err != nil {
			t.Fatal(err)
		}
		snapshot := glyph.Finalize()
		codes := snapshot.Codes()
		codes[0] = 2
	}
	if text.glyphs[0].Codes()[0] != 1 {
		t.Fatal("glyph iterator exposed source code")
	}

	path := newPathObject(contentdata.PathSpec{Segments: []geometry.PathSegment{geometry.NewPathSegment("m", [2]float64{1, 2})}})
	for segment, err := range path.SegmentsSeq() {
		if err != nil {
			t.Fatal(err)
		}
		snapshot := segment.Finalize()
		points := snapshot.PointsCopy()
		points[0][0] = 3
	}
	if path.SegmentsCopy()[0].PointsCopy()[0][0] != 1 {
		t.Fatal("path iterator exposed source points")
	}
}
