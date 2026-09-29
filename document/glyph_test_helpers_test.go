package document

import (
	"github.com/lin-string/go-playa/contentdata"
	"github.com/lin-string/go-playa/geometry"
)

func newTestGlyph(spec contentdata.GlyphSpec, font *Font, type3Ops []ContentOp, type3Matrix geometry.Matrix) GlyphObject {
	return newGlyphObject(spec, font, type3Ops, type3Matrix)
}

func newTestGlyphWithCode(font *Font, code []byte, gid int, type3Matrix geometry.Matrix) GlyphObject {
	return newTestGlyph(contentdata.GlyphSpec{Code: code, GID: gid}, font, nil, type3Matrix)
}

func newTestGlyphWithFont(font *Font) GlyphObject {
	return newTestGlyph(contentdata.GlyphSpec{}, font, nil, geometry.Matrix{})
}

func newTestGlyphWithBBox(bbox [4]float64) GlyphObject {
	return newTestGlyph(contentdata.GlyphSpec{BBox: bbox}, nil, nil, geometry.Matrix{})
}

func newTestGlyphWithGeometry(origin, displacement [2]float64) GlyphObject {
	return newTestGlyph(contentdata.GlyphSpec{Origin: origin, Displacement: displacement}, nil, nil, geometry.Matrix{})
}
