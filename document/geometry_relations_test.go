package document

import (
	"testing"

	"github.com/lin-string/go-playa/contentdata"
	"github.com/lin-string/go-playa/geometry"
)

func TestBBoxRelationsMatchPlayaComponentSemantics(t *testing.T) {
	left := newTestGlyph(contentdata.GlyphSpec{BBox: [4]float64{0, 10, 10, 20}}, nil, nil, geometry.Matrix{})
	touching := newTestTextLine("", [4]float64{10, 12, 20, 18}, false)
	separate := newPathObject(contentdata.PathSpec{BBox: [4]float64{25, 30, 35, 40}})
	contained := ImageObject{bbox: [4]float64{2, 12, 8, 18}}

	if !left.IsHoverlap(touching) || left.HDistance(touching) != 0 || left.Hoverlap(touching) != 0 {
		t.Fatalf("touching horizontal bounds = overlap=%v distance=%v length=%v", left.IsHoverlap(touching), left.HDistance(touching), left.Hoverlap(touching))
	}
	if left.IsHoverlap(separate) || left.HDistance(separate) != 15 || left.Hoverlap(separate) != 0 {
		t.Fatalf("separate horizontal bounds = overlap=%v distance=%v length=%v", left.IsHoverlap(separate), left.HDistance(separate), left.Hoverlap(separate))
	}
	if !left.IsHoverlap(contained) || left.HDistance(contained) != 0 || left.Hoverlap(contained) != 8 {
		t.Fatalf("contained horizontal bounds = overlap=%v distance=%v length=%v", left.IsHoverlap(contained), left.HDistance(contained), left.Hoverlap(contained))
	}

	if !left.IsVOverlap(touching) || left.VDistance(touching) != 0 || left.VOverlap(touching) != 8 {
		t.Fatalf("vertical overlap = overlap=%v distance=%v length=%v", left.IsVOverlap(touching), left.VDistance(touching), left.VOverlap(touching))
	}
	if left.IsVOverlap(separate) || left.VDistance(separate) != 10 || left.VOverlap(separate) != 0 {
		t.Fatalf("separate vertical bounds = overlap=%v distance=%v length=%v", left.IsVOverlap(separate), left.VDistance(separate), left.VOverlap(separate))
	}
}

func TestBBoxRelationsAreAvailableAcrossPublicContentTypes(t *testing.T) {
	var _ BBoxProvider = GlyphObject{}
	var _ BBoxProvider = TextObject{}
	var _ BBoxProvider = PathObject{}
	var _ BBoxProvider = ImageObject{}
	var _ BBoxProvider = XObjectObject{}
	var _ BBoxProvider = TextWord{}
	var _ BBoxProvider = TextLine{}
	var _ BBoxProvider = TextParagraph{}
	var _ BBoxProvider = TextBox{}
	var _ BBoxProvider = TextGroup{}

	var _ = (GlyphObject{}).IsHoverlap
	var _ = (GlyphObject{}).HDistance
	var _ = (GlyphObject{}).Hoverlap
	var _ = (GlyphObject{}).IsVOverlap
	var _ = (GlyphObject{}).VDistance
	var _ = (GlyphObject{}).VOverlap
	var _ = (TextLine{}).IsHoverlap
}
