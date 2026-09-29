package font_test

import (
	"testing"

	"github.com/lin-string/go-playa/font"
)

func TestPublicFontSurface(t *testing.T) {
	var _ *font.ParseError
	var _ font.Object
	var _ font.Ref
	var _ font.Dict
	var _ font.Array
	var _ font.Stream
	var _ font.Null
	var _ font.Bool
	var _ font.Number
	var _ font.Name
	var _ font.String
	var _ font.Keyword
	var _ font.FontMetadata
	_ = (*font.Font).Name
	_ = (*font.Font).BaseFont
	_ = (*font.Font).CIDCoding
	_ = (font.FontMetadata{}).Name
	_ = (font.FontMetadata{}).Flags
	_ = (font.FontMetadata{}).Ascent
	_ = (font.FontMetadata{}).Descent
	_ = (font.FontMetadata{}).Leading
	_ = (font.FontMetadata{}).CapHeight
	_ = (font.FontMetadata{}).StemV
	_ = (font.FontMetadata{}).FontMatrix
	_ = (font.FontMetadata{}).HasFlags
	_ = (font.FontMetadata{}).HasBBox
	_ = (font.FontMetadata{}).ItalicAngle
	_ = (font.FontMetadata{}).DefaultWidth
	_ = (font.FontMetadata{}).Vertical
	_ = (font.FontMetadata{}).BBox
	_ = (font.FontMetadata{}).Finalize
	var _ font.Matrix
	_ = font.ParseToUnicodeCodes
	_ = font.ParseToUnicodeMap
	_ = font.ParseEncodingCMap
	_ = (*font.ToUnicodeMap)(nil).Lookup
	_ = (*font.ToUnicodeMap)(nil).CodespacesCopy
	_ = (*font.ToUnicodeMap)(nil).MappingsCopy
	_ = (*font.ToUnicodeMap)(nil).Decode
	_ = (*font.ToUnicodeMap)(nil).DecodeSeq
	_ = font.ParseWidth
	_ = (font.CodeSpace{}).LowCopy
	_ = (font.CodeSpace{}).HighCopy
	_ = (font.CodeSpace{}).Finalize
	_ = (font.Code{}).BytesCopy
	_ = (font.Code{}).CID
	_ = (font.Code{}).Finalize
	_ = (font.DecodedGlyph{}).Text
	_ = (font.DecodedGlyph{}).CID
	_ = (font.DecodedGlyph{}).BytesCopy
	_ = (font.DecodedGlyph{}).Finalize
	_ = (*font.CMap)(nil).MappingCopy
	_ = (*font.CMap)(nil).CodespacesCopy
	_ = (*font.CMap)(nil).Finalize
	_ = (*font.CMap)(nil).DecodeSeq
	_ = (*font.UnicodeMap)(nil).Name
	_ = (*font.UnicodeMap)(nil).Vertical
	_ = (*font.UnicodeMap)(nil).Lookup
	_ = (*font.UnicodeMap)(nil).Unicode
	_ = (*font.UnicodeMap)(nil).MappingCopy
	_ = (*font.UnicodeMap)(nil).Finalize
	_ = font.LoadPredefinedUnicodeMap
	f := font.NewSimple("Helvetica")
	_ = f.DecodeGlyphs
	_ = f.DecodeGlyphsSeq
	assertSeq2[font.DecodedGlyph](f.DecodeGlyphsSeqWithError(nil))
	_ = f.WidthCode
	_ = f.WidthCodeWithError
	_ = f.Width
	_ = f.WidthWithError
	_ = f.GlyphBBox
	_ = f.HDisp
	_ = f.HDispWithError
	_ = f.VDisp
	_ = f.VDispWithError
	_ = f.Position
	_ = f.VPosition
	_ = f.VPositionWithError
	_ = f.CharBBox
	_ = f.CharBBoxWithError
	_ = f.Finalize
	_ = f.FinalizeWithError
	_ = f.EmbeddedFontFile
	_ = f.EmbeddedFontFileWithError
	_ = f.WriteFontFile
	_ = f.ResourcesCopy
	_ = f.EncodingValue
	_ = f.EncodingValueWithError
	_ = f.GlyphName
	_ = f.GlyphNameWithError
	_ = f.WidthValue
	_ = f.WidthValueWithError
	_ = f.CIDWidth
	_ = f.CIDWidthWithError
	_ = f.CIDToGIDCopy
	_ = f.CIDToGIDCopyWithError
	_ = f.VerticalWidth
	_ = f.VerticalWidthWithError
	_ = f.VerticalPosition
	_ = f.VerticalPositionWithError
	_ = f.ToUnicodeValue
	_ = f.ToUnicodeValueWithError
	_ = f.ToUnicodeCodeValue
	_ = f.ToUnicodeCodeValueWithError
	_ = f.CMapSnapshot
	_ = f.CMapSnapshotWithError
	_ = f.CharProc
	_ = f.WithVariationCoordinates
	_ = f.VariationCoordinatesCopy
	_ = f.Name
	_ = f.Subtype
	_ = f.IsType3
	_ = f.IsVertical
	_ = f.IsCID
	_ = f.IsMultibyte
	_ = f.FontMatrix
	_ = f.DefaultWidth
	_ = f.DefaultVWidth
	_ = f.DefaultVPosition
	_ = f.Ascent
	_ = f.Descent
	_ = f.Leading
	_ = f.FontBBox
	_ = f.Flags
	_ = f.CapHeight
	_ = f.ItalicAngle
	_ = f.StemV
	if f == nil {
		t.Fatal("NewSimple returned nil font")
	}
}

func TestPublicFontMetadataIsReadOnly(t *testing.T) {
	f := font.NewSimple("Helvetica")
	if f.Name() != "Helvetica" || f.IsCID() || f.IsType3() || f.IsVertical() || f.IsMultibyte() {
		t.Fatalf("font identity = name:%q cid:%v type3:%v vertical:%v multibyte:%v", f.Name(), f.IsCID(), f.IsType3(), f.IsVertical(), f.IsMultibyte())
	}
	if got := f.DefaultWidth(); got != 500 {
		t.Fatalf("default width = %v, want 500", got)
	}
	if bbox, ok := f.FontBBox(); ok || bbox != [4]float64{} {
		t.Fatalf("font bbox = %v, %v; want absent zero bbox", bbox, ok)
	}
}

func TestToUnicodeMapFacadePreservesWideCodesAndCodespaces(t *testing.T) {
	toUnicode, err := font.ParseToUnicodeMap([]byte(`1 begincodespacerange
<0000> <ffff>
endcodespacerange
2 beginbfchar
<0041> <0041>
<00010001> <4e2d>
endbfchar`))
	if err != nil {
		t.Fatal(err)
	}
	if value, ok := toUnicode.Lookup([]byte{0, 0x41}); !ok || value != "A" {
		t.Fatalf("Lookup = %q, %v", value, ok)
	}
	if value, ok := toUnicode.Lookup([]byte{0, 1, 0, 1}); !ok || value != "中" {
		t.Fatalf("wide Lookup = %q, %v", value, ok)
	}
	if len(toUnicode.CodespacesCopy()) != 1 {
		t.Fatal("ToUnicodeMap did not expose codespaces")
	}
}

func assertSeq2[T any](seq func(func(T, error) bool)) {}

func TestParseToUnicodeCodesPreservesWideSourceCodes(t *testing.T) {
	data := []byte("1 beginbfchar\n<00010001> <0041>\nendbfchar")
	mapping, err := font.ParseToUnicodeCodes(data)
	if err != nil {
		t.Fatal(err)
	}
	if got := mapping[string([]byte{0, 1, 0, 1})]; got != "A" {
		t.Fatalf("wide ToUnicode mapping = %q, all mappings = %#v", got, mapping)
	}
}

func TestPublicFontLoadsCachedPredefinedCMap(t *testing.T) {
	first, err := font.LoadPredefinedCMap("Identity-V")
	if err != nil {
		t.Fatal(err)
	}
	second, err := font.LoadPredefinedCMap("Identity-V")
	if err != nil {
		t.Fatal(err)
	}
	if first != second || !first.Vertical() {
		t.Fatalf("predefined CMap cache = first %p, second %p, vertical=%v", first, second, first.Vertical())
	}
	decoded := first.Decode([]byte{0x01, 0x02})
	if len(decoded) != 1 || decoded[0].CID() != 0x0102 {
		t.Fatalf("Identity-V decode = %#v", decoded)
	}
}

func TestPublicFontLoadsCachedPredefinedUnicodeMap(t *testing.T) {
	horizontal, err := font.LoadPredefinedUnicodeMap("Adobe-Japan1", false)
	if err != nil {
		t.Fatal(err)
	}
	if horizontal.Name() != "Adobe-Japan1" || horizontal.Vertical() {
		t.Fatalf("horizontal UnicodeMap identity = name:%q vertical:%v", horizontal.Name(), horizontal.Vertical())
	}
	if value, ok := horizontal.Lookup(2980); !ok || value != "中" {
		t.Fatalf("horizontal CID 2980 = %q, %v", value, ok)
	}
	if horizontal.Unicode(2980) != "中" || horizontal.Unicode(-1) != "" {
		t.Fatalf("Unicode lookup = %q and %q", horizontal.Unicode(2980), horizontal.Unicode(-1))
	}

	vertical, err := font.LoadPredefinedUnicodeMap("Japan1", true)
	if err != nil {
		t.Fatal(err)
	}
	if vertical.Name() != "Adobe-Japan1" || !vertical.Vertical() {
		t.Fatalf("vertical UnicodeMap identity = name:%q vertical:%v", vertical.Name(), vertical.Vertical())
	}
	if value, ok := vertical.Lookup(736); !ok || value != "↑" {
		t.Fatalf("vertical CID 736 = %q, %v", value, ok)
	}

	again, err := font.LoadPredefinedUnicodeMap("Adobe-Japan1", false)
	if err != nil {
		t.Fatal(err)
	}
	if again != horizontal {
		t.Fatalf("predefined UnicodeMap cache returned %p then %p", horizontal, again)
	}

	mapping := horizontal.MappingCopy()
	mapping[2980] = "mutated"
	if got := horizontal.Unicode(2980); got != "中" {
		t.Fatalf("MappingCopy exposed backing map: %q", got)
	}
	snapshot := horizontal.Finalize()
	if snapshot == horizontal || snapshot.Unicode(2980) != "中" {
		t.Fatalf("UnicodeMap Finalize = %p, %q", snapshot, snapshot.Unicode(2980))
	}
}

func TestPublicFontRejectsUnknownPredefinedUnicodeMap(t *testing.T) {
	if got, err := font.LoadPredefinedUnicodeMap("unknown", false); err == nil || got != nil {
		t.Fatalf("unknown UnicodeMap = %v, err=%v", got, err)
	}
}
