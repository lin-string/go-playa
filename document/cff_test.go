package document

import (
	"math"
	"testing"

	"github.com/lin-string/go-playa/fontdata"
	"github.com/lin-string/go-playa/geometry"
)

func TestCFFVariationStoreClonePreservesAbsentSlices(t *testing.T) {
	clone := cloneCFFVariationStore(&cffVariationStore{})
	if clone == nil || clone.regions != nil || clone.data != nil {
		t.Fatalf("zero variation store clone = %#v, want nil slices", clone)
	}
	source := &cffVariationStore{
		regions: []cffVariationRegion{{start: []float64{}, peak: []float64{}, end: []float64{}}},
		data:    []cffVariationData{{regions: []int{}, rows: [][]float64{{}}}},
	}
	clone = cloneCFFVariationStore(source)
	if clone.regions == nil || clone.regions[0].start == nil || clone.regions[0].peak == nil || clone.regions[0].end == nil ||
		clone.data == nil || clone.data[0].regions == nil || clone.data[0].rows == nil || clone.data[0].rows[0] == nil {
		t.Fatalf("empty allocated variation slices were collapsed: %#v", clone)
	}
}

func TestCFF2WidthRejectsVariationOperatorsWithoutOptionalStore(t *testing.T) {
	for _, data := range [][]byte{
		{139, 15},      // VSIndex 0 requires a variation store.
		{139, 140, 16}, // A blend requires a variation store.
	} {
		if _, ok := parseCFF2CharStringWidthWithGlobals(data, 0, 500, nil, nil, nil, nil, 0); ok {
			t.Fatalf("accepted variation operator without a store: %v", data)
		}
	}
}

func TestParseCFFMetadata(t *testing.T) {
	data := []byte{1, 0, 4, 4}
	data = appendCFFIndex(data, []byte("Test"))
	top := []byte{139, 139, 239, 247, 92, 5, 139, 140, 140, 139, 139, 139, 12, 7}
	data = appendCFFIndex(data, top)
	data = appendCFFIndex(data)
	data = appendCFFIndex(data)
	metadata, ok := parseCFFMetadata(data)
	if !ok || !metadata.hasBBox || metadata.bbox != [4]float64{0, 0, 100, 200} {
		t.Fatalf("CFF bbox = %#v, ok=%v", metadata, ok)
	}
	if !metadata.hasMatrix || metadata.matrix != (geometry.Matrix{0, 1, 1, 0, 0, 0}) {
		t.Fatalf("CFF matrix = %#v", metadata)
	}
}

func TestImplicitCFFEncodingReplacesDefaultEncoding(t *testing.T) {
	f := NewSimpleFont("Type1")
	f.applyEncoding("StandardEncoding")
	f.cffImplicitEncoding = true
	applyCFFCodeNames(f, map[byte]string{65: "privateGlyph"}, nil)

	if _, ok := f.encoding[65]; ok {
		t.Fatal("implicit CFF encoding retained the default Unicode mapping")
	}
	if _, ok := f.glyphTexts[65]; ok {
		t.Fatal("implicit CFF encoding retained the default glyph text")
	}
	if f.glyphNames[65] != "privateGlyph" || !f.strictEncoding {
		t.Fatalf("implicit CFF encoding state = names %#v strict=%v", f.glyphNames, f.strictEncoding)
	}
}

func TestParseCFF2Metadata(t *testing.T) {
	top := []byte{139, 139, 239, 247, 92, 5, 139, 140, 140, 139, 139, 139, 12, 7}
	data := []byte{2, 0, 5, byte(len(top) >> 8), byte(len(top))}
	data = append(data, top...)
	data = appendCFFIndex(data)
	metadata, ok := parseCFFMetadata(data)
	if !ok || !metadata.hasBBox || metadata.bbox != [4]float64{0, 0, 100, 200} {
		t.Fatalf("CFF2 bbox = %#v, ok=%v", metadata, ok)
	}
	if !metadata.hasMatrix || metadata.matrix != (geometry.Matrix{0, 1, 1, 0, 0, 0}) {
		t.Fatalf("CFF2 matrix = %#v", metadata)
	}
}

func TestCFF2Index(t *testing.T) {
	data := []byte{0, 0, 0, 2, 1, 1, 2, 3, 'A', 'B'}
	items, end, ok := fontdata.ParseCFF2Index(data, 0)
	if !ok || end != len(data) || len(items) != 2 || string(items[0]) != "A" || string(items[1]) != "B" {
		t.Fatalf("CFF2 INDEX = %#v, end=%d, ok=%v", items, end, ok)
	}
}

func TestCFFIndexesRejectOverflowingOffsets(t *testing.T) {
	maxInt := int(^uint(0) >> 1)
	if _, _, ok := fontdata.ParseCFFIndex([]byte{0, 0}, maxInt); ok {
		t.Fatal("CFF INDEX accepted an overflowing offset")
	}
	if _, _, ok := fontdata.ParseCFF2Index([]byte{0, 0, 0, 0}, maxInt); ok {
		t.Fatal("CFF2 INDEX accepted an overflowing offset")
	}
}

func TestParseCFFPrivateDictRejectsOverflowingBounds(t *testing.T) {
	metadata := cffMetadata{privateOffset: int(^uint(0) >> 1), privateSize: 2}
	defer func() {
		if recovered := recover(); recovered != nil {
			t.Fatalf("CFF private DICT caused panic: %v", recovered)
		}
	}()
	parseCFFPrivateDict([]byte{0, 0}, &metadata)
}

func TestParseCFF2PrivateDictReadsLocalSubrs(t *testing.T) {
	data := []byte{141, 19, 0, 0, 0, 1, 1, 1, 2, 14}
	metadata := cffMetadata{cff2: true, privateSize: 2}
	parseCFFPrivateDict(data, &metadata)
	if len(metadata.localSubrs) != 1 || string(metadata.localSubrs[0]) != string([]byte{14}) {
		t.Fatalf("CFF2 local subrs = %#v", metadata.localSubrs)
	}
}

func TestParseCFF2PrivateDictReadsVariationIndex(t *testing.T) {
	metadata := cffMetadata{cff2: true, privateSize: 2}
	parseCFFPrivateDict([]byte{140, 22}, &metadata)
	if metadata.variationIndex != 1 || !metadata.variationIndexSet {
		t.Fatalf("CFF2 private vsindex = %d, want 1", metadata.variationIndex)
	}
}

func TestParseCFF2PrivateDictRejectsEscapedVariationIndex(t *testing.T) {
	metadata := cffMetadata{cff2: true, privateSize: 3}
	parseCFFPrivateDict([]byte{140, 12, 22}, &metadata)
	if metadata.variationIndexSet {
		t.Fatalf("CFF2 accepted escaped private vsindex: %#v", metadata)
	}
}

func TestParseCFFDictRejectsFractionalOffsets(t *testing.T) {
	for _, dict := range [][]byte{
		{30, 0x1a, 0x5f, 15},
		{30, 0x1a, 0x5f, 16},
		{30, 0x1a, 0x5f, 17},
		{30, 0x1a, 0x5f, 12, 36},
		{30, 0x1a, 0x5f, 12, 37},
		{30, 0x1a, 0x5f, 24},
	} {
		metadata := cffMetadata{cff2: true}
		parseCFFDict(dict, &metadata)
		if metadata.charset != 0 || metadata.encoding != 0 || metadata.charStrings != 0 || metadata.fdArray != 0 || metadata.fdSelect != 0 || metadata.variationStore != 0 {
			t.Fatalf("CFF DICT accepted fractional offset: %#v", metadata)
		}
	}
	metadata := cffMetadata{privateSize: 4}
	parseCFFPrivateDict([]byte{30, 0x1a, 0x5f, 19}, &metadata)
	if metadata.subrsOffset != 0 {
		t.Fatalf("CFF Private DICT accepted fractional Subrs offset: %#v", metadata)
	}
}

func TestParseCFF2PrivateDictEvaluatesNativeBlendAtNeutralDesign(t *testing.T) {
	metadata := cffMetadata{
		cff2: true,
		variation: &cffVariationStore{
			regions: []cffVariationRegion{{start: []float64{0}, peak: []float64{1}, end: []float64{1}}},
			data:    []cffVariationData{{regions: []int{0}, rows: [][]float64{{100}}}},
		},
		privateSize: 8,
	}
	// 0 vsindex, then base=500, delta=100, n=1, native CFF2 blend,
	// followed by defaultWidthX.
	parseCFFPrivateDict([]byte{139, 22, 248, 136, 239, 140, 23, 20}, &metadata)
	if !metadata.hasDefaultWidth || metadata.defaultWidth != 500 {
		t.Fatalf("default width = %v, set=%v", metadata.defaultWidth, metadata.hasDefaultWidth)
	}
	values, ok := cffBlendDictOperandsAt([]float64{500, 100, 1}, metadata.variation, 0, []float64{1})
	if !ok || len(values) != 1 || values[0] != 600 {
		t.Fatalf("non-neutral private blend = %#v, ok=%v", values, ok)
	}
}

func TestParseCFF2PrivateDictDoesNotCarryBlendAcrossOperators(t *testing.T) {
	metadata := cffMetadata{
		cff2: true,
		variation: &cffVariationStore{
			regions: []cffVariationRegion{{start: []float64{0}, peak: []float64{1}, end: []float64{1}}},
			data:    []cffVariationData{{regions: []int{0}, rows: [][]float64{{100}}}},
		},
		privateSize: 13,
	}
	// A blend is followed by BlueScale before defaultWidthX. The blend must
	// not be attached to the later width field.
	parseCFFPrivateDict([]byte{139, 22, 248, 136, 239, 140, 23, 139, 12, 9, 248, 236, 20}, &metadata)
	if !metadata.hasDefaultWidth || metadata.defaultWidth != 600 {
		t.Fatalf("default width = %v, set=%v", metadata.defaultWidth, metadata.hasDefaultWidth)
	}
	if metadata.defaultWidthVar != nil {
		t.Fatal("CFF2 blend crossed an unrelated private DICT operator")
	}
}

func TestParseCFF2PrivateDictRestrictsVariationIndexOrdering(t *testing.T) {
	metadata := cffMetadata{cff2: true, privateSize: 4}
	parseCFFPrivateDict([]byte{139, 22, 140, 22}, &metadata)
	if metadata.variationIndex != 0 || !metadata.variationIndexSet {
		t.Fatalf("private DICT changed duplicate vsindex: %#v", metadata)
	}

	metadata = cffMetadata{
		cff2: true,
		variation: &cffVariationStore{
			regions: []cffVariationRegion{{start: []float64{0}, peak: []float64{1}, end: []float64{1}}},
			data:    []cffVariationData{{regions: []int{0}, rows: [][]float64{{100}}}},
		},
		privateSize: 9,
	}
	// Once a blend has been consumed, a later vsindex must not change the
	// variation store used by the rest of this PrivateDICT.
	parseCFFPrivateDict([]byte{139, 22, 248, 136, 239, 140, 23, 140, 22}, &metadata)
	if metadata.variationIndex != 0 || !metadata.variationIndexSet {
		t.Fatalf("private DICT accepted vsindex after blend: %#v", metadata)
	}
}

func TestParseCFF2CIDWidths(t *testing.T) {
	// Header + Top DICT. The one-byte CFF numbers point to the structures
	// appended below: CharStrings at 17, FDArray at 29, FDSelect at 39.
	top := []byte{156, 17, 168, 12, 36, 178, 12, 37}
	data := []byte{2, 0, 5, 0, byte(len(top))}
	data = append(data, top...)
	data = append(data, 0, 0, 0, 0) // empty GlobalSubrs INDEX
	data = append(data, 0, 0, 0, 2, 1, 1, 2, 5, 14, 140, 141, 22)
	data = append(data, 0, 0, 0, 1, 1, 1, 4, 145, 193, 18)
	data = append(data, 4, 0, 0, 0, 1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 2)
	data = append(data, 248, 136, 21, 248, 236, 20)
	metadata, ok := parseCFF2Metadata(data)
	if !ok || metadata.glyphWidths[0] != 600 || metadata.glyphWidths[1] != 501 {
		t.Fatalf("CFF2 CID metadata = %#v, ok=%v", metadata, ok)
	}
}

func TestParseCFF2NonCIDWidths(t *testing.T) {
	// Header + Top DICT. CharStrings is at 14 and Private is at 26.
	top := []byte{153, 17, 141, 165, 18}
	data := []byte{2, 0, 5, 0, byte(len(top))}
	data = append(data, top...)
	data = append(data, 0, 0, 0, 0) // empty GlobalSubrs INDEX
	data = append(data, 0, 0, 0, 2, 1, 1, 2, 4, 14, 32, 10)
	data = append(data, 0)
	data = append(data, 141, 19, 0, 0, 0, 1, 1, 1, 4, 140, 140, 22)
	metadata, ok := parseCFF2Metadata(data)
	if !ok || metadata.glyphWidths[0] != 0 || metadata.glyphWidths[1] != 1 {
		t.Fatalf("CFF2 non-CID metadata = %#v, ok=%v", metadata, ok)
	}
}

func TestParseCFFCustomCharsetMapsGlyphText(t *testing.T) {
	data := []byte{0, 0, 0, 0, 1, 135}
	got := parseCFFCharset(data, 3, 2, [][]byte{[]byte("A")})
	if got[1] != "A" {
		t.Fatalf("CFF charset glyph text = %#v", got)
	}
}

// CFF1 stores String INDEX before Global Subr INDEX, even when both are nonempty.
func TestParseCFFMetadataSeparatesStringsAndGlobalSubroutines(t *testing.T) {
	data := []byte{1, 0, 4, 4}
	data = appendCFFIndex(data, []byte("Test"))
	topItemStart := len(data) + 5
	data = appendCFFIndex(data, []byte{0, 17, 0, 15, 0, 16})
	data = appendCFFIndex(data, []byte("uni03A9"))
	// Width 500, horizontal move 0, return.
	data = appendCFFIndex(data, []byte{248, 136, 139, 22, 11})
	charsetOffset := len(data)
	sid := len(fontdata.CFFStandardStrings)
	data = append(data, 0, byte(sid>>8), byte(sid))
	encodingOffset := len(data)
	data = append(data, 0, 1, 65)
	charStringsOffset := len(data)
	data = appendCFFIndex(data, []byte{14}, []byte{32, 29, 14})
	data[topItemStart] = byte(139 + charStringsOffset)
	data[topItemStart+2] = byte(139 + charsetOffset)
	data[topItemStart+4] = byte(139 + encodingOffset)
	metadata, ok := parseCFFMetadata(data)
	if !ok || metadata.glyphText[1] != "Ω" || metadata.codeNames[65] != "uni03A9" {
		t.Fatalf("custom CFF string/encoding = %#v, ok=%v", metadata, ok)
	}
	widths := parseCFFGlyphWidths(&metadata)
	if widths[1] != 500 {
		t.Fatalf("global subroutine width = %v, want 500", widths[1])
	}
	font := NewSimpleFont("Test")
	font.cffData = data
	font.cffImplicitEncoding = true
	for repeat := 0; repeat < 2; repeat++ {
		count := 0
		for glyph, err := range font.DecodeGlyphsSeqWithError([]byte{65}) {
			if err != nil {
				t.Fatal(err)
			}
			if glyph.Text() != "Ω" {
				t.Fatalf("decoded custom glyph = %#v", glyph)
			}
			count++
		}
		if count != 1 {
			t.Fatalf("decoded glyph count = %d", count)
		}
	}
}

func TestParseCFFMetadataAcceptsValidCustomGlyphWithoutKnownText(t *testing.T) {
	data := []byte{1, 0, 4, 4}
	data = appendCFFIndex(data, []byte("Test"))
	topItemStart := len(data) + 5
	data = appendCFFIndex(data, []byte{0, 17, 0, 15})
	data = appendCFFIndex(data, []byte("privateGlyph"))
	data = appendCFFIndex(data)
	charsetOffset := len(data)
	sid := len(fontdata.CFFStandardStrings)
	data = append(data, 0, byte(sid>>8), byte(sid))
	charStringsOffset := len(data)
	data = appendCFFIndex(data, []byte{14}, []byte{14})
	data[topItemStart] = byte(139 + charStringsOffset)
	data[topItemStart+2] = byte(139 + charsetOffset)
	metadata, ok := parseCFFMetadata(data)
	if !ok || len(metadata.charstringsData) != 2 || metadata.glyphText[1] != "" {
		t.Fatalf("valid custom CFF metadata = %#v, ok=%v", metadata, ok)
	}
	data[charsetOffset] = 9
	if _, ok := parseCFFMetadata(data); ok {
		t.Fatal("CFF with malformed charset was accepted because CharStrings were present")
	}
}

func TestParseCFFMetadataRejectsMalformedCharsetDespiteFontBBox(t *testing.T) {
	data := []byte{1, 0, 4, 4}
	data = appendCFFIndex(data, []byte("Test"))
	topItemStart := len(data) + 5
	data = appendCFFIndex(data, []byte{139, 139, 239, 247, 92, 5, 0, 17, 0, 15})
	data = appendCFFIndex(data, []byte("privateGlyph"))
	data = appendCFFIndex(data)
	charsetOffset := len(data)
	data = append(data, 9)
	charStringsOffset := len(data)
	data = appendCFFIndex(data, []byte{14}, []byte{14})
	data[topItemStart+6] = byte(139 + charStringsOffset)
	data[topItemStart+8] = byte(139 + charsetOffset)
	if _, ok := parseCFFMetadata(data); ok {
		t.Fatal("CFF with malformed charset and FontBBox was accepted")
	}
}

func TestParseCFFCIDCharsetFormats(t *testing.T) {
	for _, test := range []struct {
		name string
		data []byte
		want map[int]int
	}{
		{name: "format0", data: []byte{0, 0, 100, 0, 200}, want: map[int]int{100: 1, 200: 2}},
		{name: "format1", data: []byte{1, 0, 100, 1, 0, 200, 0}, want: map[int]int{100: 1, 101: 2, 200: 3}},
		{name: "format2", data: []byte{2, 0, 100, 0, 1, 0, 200, 0, 0}, want: map[int]int{100: 1, 101: 2, 200: 3}},
	} {
		t.Run(test.name, func(t *testing.T) {
			data := append([]byte{0, 0, 0}, test.data...)
			got := parseCFFCharsetCIDs(data, 3, len(test.want)+1)
			if len(got) != len(test.want) {
				t.Fatalf("CID charset = %#v, want %#v", got, test.want)
			}
			for cid, gid := range test.want {
				if got[cid] != gid {
					t.Fatalf("CID charset = %#v, want %#v", got, test.want)
				}
			}
		})
	}
	if got := parseCFFCharsetCIDs([]byte{1, 0, 100, 2}, 0, 3); got != nil {
		t.Fatalf("CID charset accepted an oversized range: %#v", got)
	}
}

func TestParseCFFCharsetRejectsOversizedRange(t *testing.T) {
	// The range describes three glyphs, but only two glyph slots remain after
	// .notdef.
	data := []byte{0, 0, 0, 1, 0, 0, 3}
	if got := parseCFFCharsetNames(data, 3, 3, nil); got != nil {
		t.Fatalf("CFF charset accepted an oversized range: %#v", got)
	}
}

func TestParseCFFStandardCharsetMapsGlyphText(t *testing.T) {
	data := []byte{0, 0, 0, 0, 0, 33}
	got := parseCFFCharset(data, 3, 2, nil)
	if got[1] != "@" {
		t.Fatalf("CFF standard charset glyph text = %#v", got)
	}
}

func TestParseCFFMetadataMapsPredefinedCharset(t *testing.T) {
	prefix := []byte{1, 0, 4, 4}
	prefix = appendCFFIndex(prefix, []byte("Test"))
	prefix = appendCFFIndex(prefix, []byte{139, 17})
	prefix = appendCFFIndex(prefix)
	prefix = appendCFFIndex(prefix)
	charStringsOffset := len(prefix)
	if charStringsOffset > 107 {
		t.Fatalf("synthetic CFF fixture offset too large: %d", charStringsOffset)
	}
	data := []byte{1, 0, 4, 4}
	data = appendCFFIndex(data, []byte("Test"))
	data = appendCFFIndex(data, []byte{byte(139 + charStringsOffset), 17})
	data = appendCFFIndex(data)
	data = appendCFFIndex(data)
	data = appendCFFIndex(data, []byte{14}, []byte{14})
	metadata, ok := parseCFFMetadata(data)
	if !ok || metadata.glyphText[1] != " " {
		t.Fatalf("predefined CFF charset metadata = %#v, ok=%v", metadata, ok)
	}
}

func TestParseCFFEncodingMapsPredefinedAndSupplementalCodes(t *testing.T) {
	names := []string{".notdef", "space", "A"}
	standard := parseCFFEncoding(nil, 0, names, nil)
	if standard[65] != "A" {
		t.Fatalf("CFF StandardEncoding code 65 = %q", standard[65])
	}

	custom := []byte{0, 0, 0, 0, 1, 65}
	if got := parseCFFEncoding(custom, 3, names, nil); got[65] != "space" {
		t.Fatalf("CFF custom encoding code 65 = %q", got[65])
	}

	supplemental := []byte{0, 0, 0, 0x80, 0, 1, 66, 0, 34}
	if got := parseCFFEncoding(supplemental, 3, names, nil); got[66] != "A" {
		t.Fatalf("CFF supplemental encoding code 66 = %q", got[66])
	}
	if got := parseCFFEncoding([]byte{0, 0, 0, 0, 3, 65, 66, 67}, 3, names, nil); got != nil {
		t.Fatalf("CFF format 0 encoding accepted oversized glyph count: %#v", got)
	}
	if got := parseCFFEncoding([]byte{0, 0, 0, 1, 1, 65, 3}, 3, names, nil); got != nil {
		t.Fatalf("CFF format 1 encoding accepted oversized range: %#v", got)
	}
	if got := parseCFFEncoding([]byte{0, 0, 0, 0x80, 0}, 3, names, nil); got != nil {
		t.Fatalf("CFF supplemental encoding accepted a missing supplement count: %#v", got)
	}
}

func TestParseCFFPredefinedCharsetsMapGlyphText(t *testing.T) {
	for _, test := range []struct {
		id   int
		name string
	}{
		{id: 0, name: "A"},
		{id: 1, name: "exclamsmall"},
		{id: 2, name: "onehalf"},
	} {
		text, ok := glyphText(test.name)
		if !ok {
			t.Fatalf("missing glyph-list mapping for %q", test.name)
		}
		names := cffPredefinedCharset(test.id)
		found := false
		for gid, name := range names {
			if name == test.name {
				mapped := parseCFFCharset(nil, test.id, gid+1, nil)
				if mapped[gid] != text {
					t.Fatalf("predefined charset %d glyph %q = %q", test.id, name, mapped[gid])
				}
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("predefined charset %d lacks glyph %q", test.id, test.name)
		}
	}
}

func TestParseCFFCharStringWidth(t *testing.T) {
	// 500 is the nominal width and the first operand is an optional width
	// before rmoveto. The encoded operands are 1, 30, and 0.
	charString := []byte{140, 169, 139, 21}
	if got, ok := parseCFFCharStringWidth(charString, 500, 600); !ok || got != 501 {
		t.Fatalf("explicit CFF width = %v, ok=%v", got, ok)
	}
	if got, ok := parseCFFCharStringWidth([]byte{139, 139, 21}, 500, 600); !ok || got != 600 {
		t.Fatalf("default CFF width = %v, ok=%v", got, ok)
	}
	if got, ok := parseCFFCharStringWidth([]byte{139, 14}, 500, 600); !ok || got != 500 {
		t.Fatalf("endchar default width = %v, ok=%v", got, ok)
	}
	if got, ok := parseCFFCharStringWidth([]byte{140, 141, 22}, 500, 600); !ok || got != 501 {
		t.Fatalf("hmoveto CFF width = %v, ok=%v", got, ok)
	}
	if got, ok := parseCFFCharStringWidth([]byte{140, 141, 142, 19}, 500, 600); !ok || got != 501 {
		t.Fatalf("hintmask CFF width = %v, ok=%v", got, ok)
	}
	if got, ok := parseCFFCharStringWidth([]byte{140, 141, 142, 20}, 500, 600); !ok || got != 501 {
		t.Fatalf("cntrmask CFF width = %v, ok=%v", got, ok)
	}
	if got, ok := parseCFFCharStringWidthWithSubrs([]byte{32, 10}, 500, 600, [][]byte{{140, 141, 22}}); !ok || got != 501 {
		t.Fatalf("local subroutine CFF width = %v, ok=%v", got, ok)
	}
	if got, ok := parseCFFCharStringWidthWithSubrs([]byte{150, 32, 10}, 500, 600, [][]byte{{140, 141, 22}}); !ok || got != 511 {
		t.Fatalf("explicit local subroutine CFF width = %v, ok=%v", got, ok)
	}
	if got, ok := parseCFFCharStringWidthWithGlobals([]byte{32, 29}, 500, 600, nil, [][]byte{{140, 141, 22}}); !ok || got != 501 {
		t.Fatalf("global subroutine CFF width = %v, ok=%v", got, ok)
	}
	if got, ok := parseCFFCharStringWidthWithGlobals([]byte{150, 32, 29}, 500, 600, nil, [][]byte{{140, 141, 22}}); !ok || got != 511 {
		t.Fatalf("explicit global subroutine CFF width = %v, ok=%v", got, ok)
	}
	if got, ok := parseCFFCharStringWidthWithSubrs([]byte{150, 32, 10}, 500, 600, nil); !ok || got != 511 {
		t.Fatalf("missing local subroutine CFF width = %v, ok=%v", got, ok)
	}
	if got, ok := parseCFFCharStringWidthWithGlobals([]byte{150, 32, 29}, 500, 600, nil, nil); !ok || got != 511 {
		t.Fatalf("missing global subroutine CFF width = %v, ok=%v", got, ok)
	}
}

func TestCFFCharStringOpsBuildsOutlineAndCallsSubroutine(t *testing.T) {
	// 0 0 rmoveto, 100 0 rlineto, then call local subroutine -107
	// (the bias for a one-entry subroutine INDEX is 107). The subroutine
	// adds a vertical line before endchar.
	data := []byte{139, 139, 21, 239, 139, 5, 32, 10, 14}
	ops, ok := cffCharStringOps(data, [][]byte{{139, 239, 5}}, nil)
	if !ok || len(ops) != 5 {
		t.Fatalf("CFF outline ops = %#v, ok=%v", ops, ok)
	}
	if ops[0].operatorValue() != "m" || ops[1].operatorValue() != "l" || ops[2].operatorValue() != "l" || ops[3].operatorValue() != "h" || ops[4].operatorValue() != "f" {
		t.Fatalf("CFF outline operators = %#v", ops)
	}
	if got, _ := NumberValue(ops[2].operandsValue()[1]); got != 100 {
		t.Fatalf("CFF subroutine endpoint = %v", got)
	}
}

func TestCFFCharStringOpsCachesOwnedPathResults(t *testing.T) {
	font := NewSimpleFont("CFF")
	font.cffCharstrings = [][]byte{{139, 139, 21, 14}}
	font.cffGlyphIDs[65] = 0

	first, ok := font.cffCharStringOps([]byte{65})
	if !ok || len(first) == 0 {
		t.Fatalf("first CFF path = %#v, ok=%v", first, ok)
	}
	first[0].operandsValue()[0] = Number(999)
	second, ok := font.cffCharStringOps([]byte{65})
	if !ok || len(second) == 0 {
		t.Fatalf("cached CFF path = %#v, ok=%v", second, ok)
	}
	if got, _ := NumberValue(second[0].operandsValue()[0]); got != 0 {
		t.Fatalf("cached CFF path shared mutable operands: %v", got)
	}
	if len(font.cffPathCache) != 1 {
		t.Fatalf("CFF path cache entries = %d, want 1", len(font.cffPathCache))
	}
}

func TestCFFCharStringOpsLazilyResolveCIDCMap(t *testing.T) {
	font := NewSimpleFont("CFF CID")
	font.cid = true
	font.cffCharstrings = [][]byte{
		{139, 139, 21, 14},
		{139, 139, 21, 239, 139, 5, 14},
	}
	font.cidToGID = map[int]int{}
	font.cidToGID[65] = 1
	font.cmapData = []byte("1 begincodespacerange\n<0102> <0102>\nendcodespacerange\n1 begincidchar\n<0102> 65\nendcidchar")
	if font.cmapParsed {
		t.Fatal("CMap parsed during font setup")
	}
	if got := font.glyphIDForCode([]byte{0x01, 0x02}, 0); got != 1 {
		t.Fatalf("lazy CID CFF glyph ID = %d, want 1", got)
	}
	ops, ok := font.cffCharStringOps([]byte{0x01, 0x02})
	if !ok || len(ops) == 0 {
		t.Fatalf("CFF CID path = %#v, ok=%v", ops, ok)
	}
	if !font.cmapParsed || font.cmapData != nil {
		t.Fatalf("CFF CID path did not finish lazy CMap parsing: parsed=%v data=%v", font.cmapParsed, font.cmapData != nil)
	}
}

func TestCFFGlyphPathsUseSavedGIDWithoutEncodingMapping(t *testing.T) {
	font := NewSimpleFont("CFF saved GID")
	font.cffCharstrings = [][]byte{
		{139, 139, 21, 14},
		{139, 139, 21, 239, 139, 5, 14},
	}
	glyph := newTestGlyphWithCode(font, []byte{0x7f}, 1, identity())
	paths := 0
	for path, err := range glyph.PathsSeq() {
		if err != nil {
			t.Fatalf("CFF saved-GID glyph path error: %v", err)
		}
		if len(path.SegmentsCopy()) == 0 {
			t.Fatal("CFF saved-GID glyph path has no segments")
		}
		paths++
	}
	if paths == 0 {
		t.Fatal("CFF saved-GID glyph produced no paths")
	}
}

func TestCFFGlyphPathsReportMalformedCIDCMap(t *testing.T) {
	font := NewSimpleFont("CFF malformed CID CMap")
	font.cid = true
	font.cffCharstrings = [][]byte{{139, 139, 21, 14}}
	font.cmapData = []byte("1 begincidchar\n<00>\nendcidchar")
	glyph := newTestGlyphWithCode(font, []byte{0}, 0, identity())
	for _, err := range glyph.PathsSeq() {
		if err == nil {
			t.Fatal("malformed CFF CID CMap produced a glyph without error")
		}
		return
	}
	t.Fatal("malformed CFF CID CMap produced no error")
}

func TestCFFCharStringOpsExpandsSeacCompositeGlyph(t *testing.T) {
	font := NewSimpleFont("CFF")
	font.cffCharstrings = [][]byte{
		{14},
		{139, 139, 21, 189, 139, 5, 14},
		{139, 139, 21, 149, 139, 5, 14},
		{149, 159, 204, 205, 14},
	}
	font.cffGlyphIDs[65] = 1
	font.cffGlyphIDs[66] = 2
	font.cffGlyphIDs[67] = 3
	ops, ok := font.cffCharStringOps([]byte{67})
	if !ok || len(ops) != 6 {
		t.Fatalf("CFF seac outline ops = %#v, ok=%v", ops, ok)
	}
	if ops[0].operatorValue() != "m" || ops[1].operatorValue() != "l" || ops[2].operatorValue() != "m" || ops[3].operatorValue() != "l" || ops[4].operatorValue() != "h" || ops[5].operatorValue() != "f" {
		t.Fatalf("CFF seac operators = %#v", ops)
	}
	x, _ := NumberValue(ops[2].operandsValue()[0])
	y, _ := NumberValue(ops[2].operandsValue()[1])
	if x != 10 || y != 20 {
		t.Fatalf("CFF seac accent origin = (%v,%v), want (10,20)", x, y)
	}
}

func TestCFFSeacRejectsNonFiniteAccentTranslation(t *testing.T) {
	state := cffPathState{
		charstrings: [][]byte{{139, 139, 21, 149, 139, 5, 14}},
		glyphIDs:    map[byte]int{65: 0, 66: 0},
	}
	if state.seac([]float64{math.Inf(1), 0, 65, 66}, 0) {
		t.Fatal("CFF seac accepted a non-finite accent translation")
	}
}

func TestCIDCFFCharStringOpsUsesCIDGIDAndFDLocalSubroutine(t *testing.T) {
	f := NewSimpleFont("CIDFont")
	f.cid = true
	f.cffCharstrings = [][]byte{
		{14},
		{139, 139, 21, 32, 10, 14},
	}
	f.cidToGID = map[int]int{42: 1}
	f.cffFDByGlyph = map[int]int{1: 0}
	f.cffFDLocalSubrs = map[int][][]byte{0: {{239, 139, 5}}}
	f.cffGlobalSubrs = nil
	ops, ok := f.cffCharStringOps([]byte{0, 42})
	if !ok || len(ops) != 4 {
		t.Fatalf("CID CFF outline ops = %#v, ok=%v", ops, ok)
	}
	if ops[0].operatorValue() != "m" || ops[1].operatorValue() != "l" || ops[2].operatorValue() != "h" || ops[3].operatorValue() != "f" {
		t.Fatalf("CID CFF outline operators = %#v", ops)
	}
	if got, _ := NumberValue(ops[1].operandsValue()[0]); got != 100 {
		t.Fatalf("CID CFF subroutine endpoint = %v", got)
	}
}

func TestCIDCFF2CharStringUsesFDVariationIndex(t *testing.T) {
	f := NewSimpleFont("CIDCFF2")
	f.cid = true
	f.cff2 = true
	f.cffCharstrings = [][]byte{{239, 247, 92, 139, 89, 141, 16, 21}}
	f.cidToGID = map[int]int{42: 0}
	f.cffFDByGlyph = map[int]int{0: 1}
	f.cffFDVariationIndex = map[int]int{1: 1}
	f.cffVariationStore = &cffVariationStore{
		regions: []cffVariationRegion{{start: []float64{0}, peak: []float64{1}, end: []float64{1}}},
		data:    []cffVariationData{{}, {regions: []int{0}, rows: [][]float64{{0, -50}}}},
	}
	f.variationCoords = []float64{0.5}
	ops, ok := f.cffCharStringOps([]byte{0, 42})
	if !ok || len(ops) == 0 {
		t.Fatalf("CID CFF2 ops = %#v, ok=%v", ops, ok)
	}
	y, _ := NumberValue(ops[0].operandsValue()[1])
	if y != 175 {
		t.Fatalf("CID CFF2 FD variation y = %v, want 175", y)
	}
}

func TestCFF2BlendPreservesDefaultDesignOperands(t *testing.T) {
	// Two default values, two deltas, and numBlends=2 followed by rmoveto.
	data := []byte{139, 140, 141, 142, 141, 12, 16, 21, 14}
	ops, ok := cffCharStringOps(data, nil, nil)
	if !ok || len(ops) != 3 {
		t.Fatalf("CFF2 blend ops = %#v, ok=%v", ops, ok)
	}
	if ops[0].operatorValue() != "m" || ops[1].operatorValue() != "h" || ops[2].operatorValue() != "f" {
		t.Fatalf("CFF2 blend operators = %#v", ops)
	}
	x, _ := NumberValue(ops[0].operandsValue()[0])
	y, _ := NumberValue(ops[0].operandsValue()[1])
	if x != 0 || y != 1 {
		t.Fatalf("CFF2 blend default point = (%v,%v)", x, y)
	}
}

func TestCFF2VariationStoreAndBlend(t *testing.T) {
	// One axis, one region active from the default to the positive extreme.
	// The item row applies a -50 delta to the second operand.
	data := []byte{
		0, 1, 0, 0, 0, 22, 0, 1, 0, 0, 0, 12,
		0, 1, 0, 1, 0, 1, 0, 0, 0xff, 0xce,
		0, 1, 0, 1, 0, 0, 0x40, 0, 0x40, 0,
	}
	store := parseCFF2VariationStore(data, 0)
	if store == nil || len(store.regions) != 1 || len(store.data) != 1 {
		t.Fatalf("variation store = %#v", store)
	}
	if got := store.regionScalars([]float64{0.5})[0]; got != 0.5 {
		t.Fatalf("region scalar = %v", got)
	}
	// 100 200, then deltas 0 -50, then n=2 blend rmoveto.
	ops, ok := cff2CharStringOps([]byte{239, 247, 92, 139, 89, 141, 16, 21}, nil, nil, store, []float64{0.5}, 0)
	if !ok || len(ops) != 3 || ops[0].operatorValue() != "m" {
		t.Fatalf("CFF2 variation ops = %#v, ok=%v", ops, ok)
	}
	x, _ := NumberValue(ops[0].operandsValue()[0])
	y, _ := NumberValue(ops[0].operandsValue()[1])
	if x != 100 || y != 175 {
		t.Fatalf("CFF2 variation point = (%v,%v)", x, y)
	}
}

func TestCFF2CharStringWidthEvaluatesBlend(t *testing.T) {
	store := &cffVariationStore{
		regions: []cffVariationRegion{{start: []float64{0}, peak: []float64{1}, end: []float64{1}}},
		data:    []cffVariationData{{regions: []int{0}, rows: [][]float64{{100}}}},
	}
	// width=500, delta=100, numBlends=1, then rmoveto(0, 0).
	data := []byte{248, 136, 239, 140, 16, 139, 139, 21}
	width, ok := parseCFF2CharStringWidthWithGlobals(data, 0, 0, nil, nil, store, []float64{1}, 0)
	if !ok || width != 600 {
		t.Fatalf("CFF2 blended CharString width = %v, %v; want 600, true", width, ok)
	}
}

func TestCFF2CharStringWidthHonorsVSIndex(t *testing.T) {
	store := &cffVariationStore{
		regions: []cffVariationRegion{{start: []float64{0}, peak: []float64{1}, end: []float64{1}}},
		data: []cffVariationData{
			{regions: []int{0}, rows: [][]float64{{100}}},
			{regions: []int{0}, rows: [][]float64{{200}}},
		},
	}
	// Select item 1, then blend 500 + 200 at the positive variation limit.
	data := []byte{140, 15, 248, 136, 247, 92, 140, 16, 139, 22}
	width, ok := parseCFF2CharStringWidthWithGlobals(data, 0, 0, nil, nil, store, []float64{1}, 0)
	if !ok || width != 700 {
		t.Fatalf("CFF2 vsindex width = %v, %v; want 700, true", width, ok)
	}
}

func TestCFF2PathStateReusesVariationScalars(t *testing.T) {
	store := &cffVariationStore{
		regions: []cffVariationRegion{{start: []float64{0}, peak: []float64{1}, end: []float64{1}}},
		data:    []cffVariationData{{regions: []int{0}, rows: [][]float64{{100}}}},
	}
	state := cffPathState{cff2: true, variation: store, coords: []float64{0.5}}
	first := state.variationScalars()
	second := state.variationScalars()
	if len(first) != 1 || first[0] != 0.5 {
		t.Fatalf("first variation scalar = %#v", first)
	}
	if len(second) != len(first) || &second[0] != &first[0] {
		t.Fatal("CFF2 path state recomputed variation scalars")
	}
}

func TestCFF2VariationCoordinatesReachGlyphPaths(t *testing.T) {
	store := &cffVariationStore{
		regions: []cffVariationRegion{{start: []float64{0}, peak: []float64{1}, end: []float64{1}}},
		data:    []cffVariationData{{regions: []int{0}, rows: [][]float64{{0, -50}}}},
	}
	font := NewSimpleFont("CFF2")
	font.cff2 = true
	font.cffCharstrings = [][]byte{{}, {239, 247, 92, 139, 89, 141, 16, 21, 239, 6}}
	font.cffGlyphIDs[65] = 1
	font.cffVariationStore = store
	font.fontMatrix = geometry.Matrix{0.001, 0, 0, 0.001, 0, 0}

	collectMove := func(glyph GlyphObject) float64 {
		for path, err := range glyph.PathsSeq() {
			if err != nil || len(path.RawSegmentsCopy()) == 0 {
				t.Fatalf("CFF2 glyph path = %#v, err=%v", path, err)
			}
			points := path.RawSegmentsCopy()[0].PointsCopy()
			if len(points) == 0 {
				t.Fatalf("CFF2 move segment has no points: %#v", path.RawSegmentsCopy()[0])
			}
			return points[0][1]
		}
		t.Fatal("CFF2 glyph produced no path")
		return 0
	}
	base := collectMove(newTestGlyphWithCode(font, []byte{65}, 0, identity()))
	font.variationCoords = []float64{0.5}
	varied := collectMove(newTestGlyphWithCode(font, []byte{65}, 0, identity()))
	if base == varied {
		t.Fatalf("CFF2 variation did not change glyph path: base=%v varied=%v", base, varied)
	}
}

func TestCFF2PrivateVariationIndexIsInheritedByCharString(t *testing.T) {
	store := &cffVariationStore{
		regions: []cffVariationRegion{{start: []float64{0}, peak: []float64{1}, end: []float64{1}}},
		data:    []cffVariationData{{}, {regions: []int{0}, rows: [][]float64{{0, -50}}}},
	}
	ops, ok := cff2CharStringOps([]byte{239, 247, 92, 139, 89, 141, 16, 21}, nil, nil, store, []float64{0.5}, 1)
	if !ok || len(ops) < 1 {
		t.Fatalf("inherited CFF2 variation ops = %#v, ok=%v", ops, ok)
	}
	y, _ := NumberValue(ops[0].operandsValue()[1])
	if y != 175 {
		t.Fatalf("inherited CFF2 variation y = %v, want 175", y)
	}
}

func TestCFF2CharStringRejectsMultipleVariationIndexSelections(t *testing.T) {
	store := &cffVariationStore{
		regions: []cffVariationRegion{{start: []float64{0}, peak: []float64{1}, end: []float64{1}}},
		data: []cffVariationData{
			{regions: []int{0}, rows: [][]float64{{10, 20}}},
			{regions: []int{0}, rows: [][]float64{{30, 40}}},
		},
	}
	// A CharString may select its ItemVariationData only once, before blend.
	data := []byte{
		139, 15,
		139, 139, 149, 159, 141, 16, 21,
		140, 15,
		139, 139, 169, 179, 141, 16, 21,
	}
	if _, ok := cff2CharStringOps(data, nil, nil, store, []float64{1}, 0); ok {
		t.Fatal("CFF2 CharString accepted multiple vsindex operators")
	}
}

func TestCFF2CharStringRejectsVariationIndexAfterBlend(t *testing.T) {
	store := &cffVariationStore{
		regions: []cffVariationRegion{{start: []float64{0}, peak: []float64{1}, end: []float64{1}}},
		data: []cffVariationData{
			{regions: []int{0}, rows: [][]float64{{10, 20}}},
			{regions: []int{0}, rows: [][]float64{{30, 40}}},
		},
	}
	// A CharString may select its ItemVariationData once, before its first
	// blend. A second selection after that blend is invalid CFF2.
	data := []byte{139, 15, 139, 139, 149, 159, 141, 16, 21, 140, 15}
	if _, ok := cff2CharStringOps(data, nil, nil, store, []float64{1}, 0); ok {
		t.Fatal("CFF2 CharString accepted vsindex after blend")
	}
}

func TestCFF2CharStringRejectsEmptyVariationIndexWithoutPanicking(t *testing.T) {
	defer func() {
		if recovered := recover(); recovered != nil {
			t.Fatalf("empty CFF2 vsindex panicked: %v", recovered)
		}
	}()
	if _, ok := cff2CharStringOps([]byte{15}, nil, nil, &cffVariationStore{}, nil, 0); ok {
		t.Fatal("empty CFF2 vsindex was accepted")
	}
}

func TestCFF2VariationRegionWithNeutralAxis(t *testing.T) {
	store := &cffVariationStore{regions: []cffVariationRegion{{
		start: []float64{-1}, peak: []float64{0}, end: []float64{1},
	}}}
	if got := store.regionScalars([]float64{0})[0]; got != 1 {
		t.Fatalf("neutral-axis scalar at default = %v", got)
	}
	if got := store.regionScalars([]float64{0.5})[0]; got != 0.5 {
		t.Fatalf("neutral-axis scalar at positive coordinate = %v", got)
	}
}

func TestCFF2BlendRequiresVariationStore(t *testing.T) {
	if _, ok := cff2CharStringOps([]byte{139, 139, 141, 16, 21}, nil, nil, nil, nil, 0); ok {
		t.Fatal("CFF2 blend accepted without a VariationStore")
	}
}

func TestCFFBlendValueKeepsBaseWithoutVariationStore(t *testing.T) {
	value := &cffBlendValue{base: 500, deltas: []float64{100}, regions: []int{0}}
	if got := value.evaluate(nil, nil); got != 500 {
		t.Fatalf("blend base without variation store = %v, want 500", got)
	}
}

func TestCFFBlendValueRejectsNonFiniteOperands(t *testing.T) {
	store := &cffVariationStore{regions: []cffVariationRegion{{start: []float64{0}, peak: []float64{1}, end: []float64{1}}}}
	for _, test := range []struct {
		value *cffBlendValue
		want  float64
	}{
		{value: &cffBlendValue{base: math.NaN(), deltas: []float64{100}, regions: []int{0}}, want: 100},
		{value: &cffBlendValue{base: 500, deltas: []float64{math.Inf(1)}, regions: []int{0}}, want: 500},
	} {
		if got := test.value.evaluate(store, []float64{1}); got != test.want {
			t.Fatalf("non-finite blend = %v, want %v", got, test.want)
		}
	}
}

func TestCFFScaledWidthRejectsOverflow(t *testing.T) {
	if value, ok := cffScaledWidth(math.MaxFloat64, 2); ok || value != 0 {
		t.Fatalf("overflowing CFF width = %v, ok=%v", value, ok)
	}
	if value, ok := cffScaledWidth(3, 2); !ok || value != 6 {
		t.Fatalf("finite CFF width = %v, ok=%v", value, ok)
	}
}

func TestCFFWidthParserRejectsOverflowingWidth(t *testing.T) {
	if width, ok := cffFiniteAdd(math.MaxFloat64, math.MaxFloat64); ok || width != 0 {
		t.Fatalf("overflowing CFF addition = %v, ok=%v", width, ok)
	}
	if width, ok := parseCFFCharStringWidth([]byte{32, 14}, 500, 600); !ok || width != 393 {
		t.Fatalf("finite CFF width = %v, ok=%v", width, ok)
	}
}

func TestCFFPathRejectsNonFiniteOperands(t *testing.T) {
	state := cffPathState{operands: []float64{math.Inf(1), 0}}
	if state.operator(5, nil, new(int), 0) {
		t.Fatal("CFF path accepted a non-finite geometry operand")
	}
}

func TestCFFPathRandomProducesDeterministicFiniteOperand(t *testing.T) {
	first := cffPathState{}
	second := cffPathState{}
	for _, state := range []*cffPathState{&first, &second} {
		if !state.operator(1223, nil, new(int), 0) {
			t.Fatal("CFF path rejected random operator")
		}
		if len(state.operands) != 1 || state.operands[0] < 0 || state.operands[0] >= 1 || math.IsNaN(state.operands[0]) {
			t.Fatalf("CFF random operand = %#v", state.operands)
		}
	}
	if first.operands[0] != second.operands[0] {
		t.Fatalf("CFF random seed is not deterministic: %v != %v", first.operands[0], second.operands[0])
	}
}

func TestCFFPathRejectsCoordinateOverflow(t *testing.T) {
	state := cffPathState{x: math.MaxFloat64, widthSeen: true, operands: []float64{math.MaxFloat64}}
	state.operator(22, nil, new(int), 0)
	if state.finite() {
		t.Fatal("CFF path accepted an overflowing coordinate")
	}
}

func TestCFFFlexRejectsCoordinateOverflow(t *testing.T) {
	state := cffPathState{x: math.MaxFloat64}
	state.flexOperator(1234, []float64{
		math.MaxFloat64, 0, 0, 0, 0, 0, 0,
	})
	if state.finite() {
		t.Fatal("CFF flex accepted an overflowing coordinate")
	}
}

func TestCFF2CharStringEnforcesImplementationLimits(t *testing.T) {
	if _, ok := cff2CharStringOps(make([]byte, 65536), nil, nil, nil, nil, 0); ok {
		t.Fatal("CFF2 accepted an overlong CharString")
	}
	deep := []byte{32, 10}
	local := [][]byte{deep}
	if _, ok := cff2CharStringOps(deep, local, nil, nil, nil, 0); ok {
		t.Fatal("CFF2 accepted excessive subroutine nesting")
	}
	operands := make([]byte, 0, 513*2+1)
	for i := 0; i < 514; i++ {
		operands = append(operands, 139)
	}
	operands = append(operands, 21)
	if _, ok := cff2CharStringOps(operands, nil, nil, nil, nil, 0); ok {
		t.Fatal("CFF2 accepted an oversized operand stack")
	}
	stems := make([]byte, 0, 97*2+1)
	for i := 0; i < 97*2; i++ {
		stems = append(stems, 139)
	}
	stems = append(stems, 1)
	if _, ok := cff2CharStringOps(stems, nil, nil, nil, nil, 0); ok {
		t.Fatal("CFF2 accepted excessive stem hints")
	}
}

func TestCFF1CharStringEnforcesOperandStackLimit(t *testing.T) {
	data := make([]byte, 0, 49)
	for i := 0; i < 49; i++ {
		data = append(data, 139)
	}
	state := cffPathState{}
	if state.run(data, 0) {
		t.Fatal("CFF1 accepted an oversized operand stack")
	}
}

func TestCFF2BlendRejectsOversizedCountBeforeAllocation(t *testing.T) {
	store := &cffVariationStore{data: []cffVariationData{{regions: []int{0}, rows: [][]float64{{1}}}}}
	if _, _, ok := cffBlendDictValuesAt([]float64{math.MaxFloat64}, store, 0, nil); ok {
		t.Fatal("CFF2 DICT blend accepted an oversized count")
	}
	operands := []byte{139, 255, 127, 255, 255, 127, 16}
	if _, ok := cff2CharStringOps(operands, nil, nil, store, nil, 0); ok {
		t.Fatal("CFF2 CharString blend accepted an oversized count")
	}
}

func TestCFF2RejectsNonCFF2EscapedOperators(t *testing.T) {
	// 12 10 is the CFF1 add operator; CFF2 reserves the escaped range except
	// for hflex/flex/hflex1/flex1.
	if _, ok := cff2CharStringOps([]byte{139, 140, 12, 10}, nil, nil, nil, nil, 0); ok {
		t.Fatal("CFF2 accepted a CFF1 escaped arithmetic operator")
	}
	if _, ok := cff2CharStringOps([]byte{12, 23}, nil, nil, nil, nil, 0); ok {
		t.Fatal("CFF2 accepted the CFF1 random operator")
	}
	state := cffPathState{cff2: true}
	if state.operator(1223, nil, new(int), 0) {
		t.Fatal("CFF2 operator state accepted the CFF1 random operator")
	}
	hflex := []byte{139, 139, 21, 139, 139, 139, 139, 139, 139, 139, 12, 34}
	if _, ok := cff2CharStringOps(hflex, nil, nil, nil, nil, 0); !ok {
		t.Fatal("CFF2 rejected the hflex escaped operator")
	}
}

func TestCFF2RejectsType2ReturnAndEndChar(t *testing.T) {
	if _, ok := cff2CharStringOps([]byte{11}, nil, nil, nil, nil, 0); ok {
		t.Fatal("CFF2 accepted Type2 return")
	}
	if _, ok := cff2CharStringOps([]byte{14}, nil, nil, nil, nil, 0); ok {
		t.Fatal("CFF2 accepted Type2 endchar")
	}
}

func TestCFF2DoesNotConsumeType2WidthOperand(t *testing.T) {
	// Three hstem operands are invalid in CFF2; the first one must not be
	// silently consumed as a Type2 width.
	if _, ok := cff2CharStringOps([]byte{139, 139, 139, 1}, nil, nil, nil, nil, 0); ok {
		t.Fatal("CFF2 consumed a Type2 width operand")
	}
}

func TestCFF2RejectsIncompleteAlternatingCurves(t *testing.T) {
	for _, op := range []byte{26, 27, 30, 31} {
		if _, ok := cff2CharStringOps([]byte{139, op}, nil, nil, nil, nil, 0); ok {
			t.Fatalf("CFF2 accepted incomplete curve operator %d", op)
		}
	}
}

func TestCFF2RejectsEmptyDrawingOperators(t *testing.T) {
	for _, op := range []byte{5, 6, 7, 8} {
		if _, ok := cff2CharStringOps([]byte{op}, nil, nil, nil, nil, 0); ok {
			t.Fatalf("CFF2 accepted empty drawing operator %d", op)
		}
	}
}

func TestCFF2RejectsEmptyStemOperators(t *testing.T) {
	for _, op := range []byte{1, 3, 18, 23} {
		if _, ok := cff2CharStringOps([]byte{op}, nil, nil, nil, nil, 0); ok {
			t.Fatalf("CFF2 accepted empty stem operator %d", op)
		}
	}
}

func TestCFF2RejectsEmptyHintMasks(t *testing.T) {
	for _, op := range []byte{19, 20} {
		if _, ok := cff2CharStringOps([]byte{op}, nil, nil, nil, nil, 0); ok {
			t.Fatalf("CFF2 accepted empty hint-mask operator %d", op)
		}
	}
}

func TestCFFRejectsEmptyHintMasks(t *testing.T) {
	for _, op := range []byte{19, 20} {
		if _, ok := cffCharStringOps([]byte{op}, nil, nil); ok {
			t.Fatalf("CFF accepted empty hint-mask operator %d", op)
		}
	}
}

func TestCFF2RejectsTrailingCharStringOperands(t *testing.T) {
	if _, ok := cff2CharStringOps([]byte{139}, nil, nil, nil, nil, 0); ok {
		t.Fatal("CFF2 accepted a CharString ending with an unconsumed operand")
	}
}

func TestCFF2BlendRejectsNegativeVariationRegion(t *testing.T) {
	store := &cffVariationStore{
		regions: []cffVariationRegion{{start: []float64{0}, peak: []float64{1}, end: []float64{1}}},
		data:    []cffVariationData{{regions: []int{-1}, rows: [][]float64{{100}}}},
	}
	if _, ok := cff2CharStringOps([]byte{139, 139, 140, 16}, nil, nil, store, []float64{1}, 0); ok {
		t.Fatal("CFF2 blend accepted a negative variation region")
	}
}

func TestCFF2VariationRegionShapeDoesNotPanic(t *testing.T) {
	store := &cffVariationStore{regions: []cffVariationRegion{{peak: []float64{1}}}}
	if scalars := store.regionScalars(nil); len(scalars) != 1 || scalars[0] != 0 {
		t.Fatalf("malformed variation region scalar = %#v", scalars)
	}
}

func TestCFF2VariationRegionIgnoresNonFiniteValues(t *testing.T) {
	store := &cffVariationStore{regions: []cffVariationRegion{{
		start: []float64{0}, peak: []float64{math.NaN()}, end: []float64{1},
	}}}
	if scalars := store.regionScalars([]float64{0.5}); len(scalars) != 1 || scalars[0] != 0 {
		t.Fatalf("non-finite variation region scalar = %#v", scalars)
	}
	value := &cffBlendValue{base: 500, deltas: []float64{100}, regions: []int{0}}
	if got := value.evaluate(store, []float64{0.5}); got != 500 {
		t.Fatalf("non-finite variation blend = %v, want base 500", got)
	}
}

func TestCFF2VariationIndexOrdering(t *testing.T) {
	store := &cffVariationStore{data: []cffVariationData{{regions: nil}, {regions: nil}}}
	if _, ok := cff2CharStringOps([]byte{140, 15, 141, 15}, nil, nil, store, nil, 0); ok {
		t.Fatal("CFF2 accepted duplicate vsindex")
	}
	if _, ok := cff2CharStringOps([]byte{139, 140, 16, 141, 15}, nil, nil, store, nil, 0); ok {
		t.Fatal("CFF2 accepted vsindex after blend")
	}
}

func TestCFF2RejectsOverflowingVariationIndex(t *testing.T) {
	store := &cffVariationStore{data: []cffVariationData{{regions: nil}}}
	state := cffPathState{cff2: true, variation: store, operands: []float64{math.MaxFloat64}}
	if state.operator(15, nil, new(int), 0) {
		t.Fatal("CFF2 accepted an overflowing variation index")
	}
}

func TestCFFRejectsOverflowingDiscreteIndices(t *testing.T) {
	if index, ok := cffSubroutineIndex(math.MaxFloat64, 1); ok || index != 0 {
		t.Fatalf("CFF accepted an overflowing subroutine index: %d, %v", index, ok)
	}
	state := cffPathState{operands: []float64{1, math.MaxFloat64}}
	if state.operator(1220, nil, new(int), 0) {
		t.Fatal("CFF accepted an overflowing transient index")
	}
	state = cffPathState{operands: []float64{1, math.MaxFloat64}}
	if state.operator(1230, nil, new(int), 0) {
		t.Fatal("CFF accepted an overflowing roll shift")
	}
}

func TestCFFRejectsOverflowingLegacyBlendCount(t *testing.T) {
	state := cffPathState{operands: []float64{1, math.MaxFloat64}}
	if state.operator(1216, nil, new(int), 0) {
		t.Fatal("CFF accepted an overflowing legacy blend count")
	}
}

func TestCFF2VariationCountsRequireIntegers(t *testing.T) {
	store := &cffVariationStore{data: []cffVariationData{{regions: nil}}}
	if _, ok := cff2CharStringOps([]byte{139, 15, 139, 139, 21}, nil, nil, store, nil, 0); !ok {
		t.Fatal("integer vsindex was rejected")
	}
	state := cffPathState{cff2: true, variation: store}
	state.operands = []float64{1, 1, 1.5}
	if state.operator(16, nil, new(int), 0) {
		t.Fatal("CFF2 accepted fractional blend count")
	}
	metadata := cffMetadata{cff2: true, privateSize: 2}
	parseCFFPrivateDict([]byte{140, 22}, &metadata)
	if metadata.variationIndex != 1 {
		t.Fatalf("integer private vsindex = %d, want 1", metadata.variationIndex)
	}
	metadata = cffMetadata{cff2: true, privateSize: 4}
	parseCFFPrivateDict([]byte{30, 0x1a, 0x5f, 22}, &metadata)
	if metadata.variationIndexSet {
		t.Fatalf("CFF2 accepted fractional private vsindex: value=%v", metadata.variationIndex)
	}
}

func TestCFF2VariationIndexMustExist(t *testing.T) {
	store := &cffVariationStore{data: []cffVariationData{{}}}
	if _, ok := cff2CharStringOps([]byte{140, 15, 139, 139, 21}, nil, nil, store, nil, 0); ok {
		t.Fatal("CFF2 accepted an out-of-range vsindex")
	}
	if _, ok := cff2CharStringOps([]byte{139, 15, 139, 139, 21}, nil, nil, nil, nil, 0); ok {
		t.Fatal("CFF2 accepted vsindex without a VariationStore")
	}
}

func TestCFF2VariationStoreRejectsReversedRegion(t *testing.T) {
	data := []byte{
		0, 1, 0, 0, 0, 22, 0, 1, 0, 0, 0, 12,
		0, 1, 0, 1, 0, 1, 0, 0, 0, 0,
		0, 1, 0, 1, 0x40, 0, 0, 0, 0, 0,
	}
	if store := parseCFF2VariationStore(data, 0); store != nil {
		t.Fatal("CFF2 accepted a reversed variation region")
	}
}

func TestCFF2VariationStoreRejectsOverlappingRegionList(t *testing.T) {
	data := []byte{
		0, 1, 0, 0, 0, 0, 0, 0,
		0, 1, 0, 0,
	}
	if store := parseCFF2VariationStore(data, 0); store != nil {
		t.Fatal("CFF2 accepted a region list overlapping the store header")
	}
}

func TestCFF2VariationStoreRejectsItemDataOverlappingHeader(t *testing.T) {
	for offset := 0; offset < 12; offset++ {
		data := []byte{
			0, 1, 0, 0, 0, 20, 0, 1,
			0, 0, 0, byte(offset),
		}
		data = append(data, make([]byte, 20)...)
		data[20], data[21] = 0, 1 // one region axis
		data[22], data[23] = 0, 0 // no regions
		if store := parseCFF2VariationStore(data, 0); store != nil {
			t.Fatalf("CFF2 accepted item data offset %d overlapping its header", offset)
		}
	}
}

func TestCFF2VariationStoreRejectsItemDataOverlappingRegionList(t *testing.T) {
	data := []byte{
		0, 1, 0, 0, 0, 20, 0, 1,
		0, 0, 0, 20,
	}
	data = append(data, make([]byte, 20)...)
	data[20], data[21] = 0, 1 // one region axis, also looks like one item row
	data[22], data[23] = 0, 0 // no regions
	if store := parseCFF2VariationStore(data, 0); store != nil {
		t.Fatal("CFF2 accepted item data overlapping its region list")
	}
}

func TestCFF2VariationStoreRejectsDuplicateItemDataOffsets(t *testing.T) {
	data := []byte{
		0, 1, 0, 0, 0, 40, 0, 2,
		0, 0, 0, 16, 0, 0, 0, 16,
	}
	data = append(data, make([]byte, 32)...)
	data[40], data[41] = 0, 1 // one region axis
	data[42], data[43] = 0, 0 // no regions
	if store := parseCFF2VariationStore(data, 0); store != nil {
		t.Fatal("CFF2 accepted duplicate ItemVariationData offsets")
	}
}

func TestCFF2VariationStoreRejectsItemDataCrossingRegionList(t *testing.T) {
	data := []byte{
		0, 1, 0, 0, 0, 24, 0, 1,
		0, 0, 0, 16,
		0, 0, 0, 0,
		0, 1, 0, 1, 0, 1, 0, 0,
		0, 1, 0, 1,
		0, 0, 0, 0, 0, 0,
	}
	if store := parseCFF2VariationStore(data, 0); store != nil {
		t.Fatal("CFF2 accepted item data crossing its region list")
	}
}

func TestCFFCharStringConsumesWidthBeforeInitialStem(t *testing.T) {
	// Width=20, one hstem pair, one hint mask byte, then a move and endchar.
	data := []byte{159, 139, 149, 19, 0, 139, 139, 21, 14}
	ops, ok := cffCharStringOps(data, nil, nil)
	if !ok || len(ops) != 3 || ops[0].operatorValue() != "m" || ops[1].operatorValue() != "h" || ops[2].operatorValue() != "f" {
		t.Fatalf("initial-stem width ops = %#v, ok=%v", ops, ok)
	}
}

func TestCFFCharStringCurvesAcceptOptionalVHCoordinate(t *testing.T) {
	// vhcurveto: dy1 dx2 dy2 dx3 dy3, followed by endchar.
	data := []byte{139, 139, 21, 149, 149, 149, 149, 149, 30, 14}
	ops, ok := cffCharStringOps(data, nil, nil)
	if !ok || len(ops) != 4 || ops[1].operatorValue() != "c" {
		t.Fatalf("vhcurveto ops = %#v, ok=%v", ops, ok)
	}
	y, _ := NumberValue(ops[1].operandsValue()[5])
	if y != 30 {
		t.Fatalf("vhcurveto endpoint y = %v", y)
	}
}

func TestCFFCharStringHFlexUsesHorizontalEndpoints(t *testing.T) {
	// Move to the origin, then hflex with dx1=10, dx2=20, dy2=5,
	// dx3=30, dx4=40, dx5=50, dx6=60.
	data := []byte{139, 139, 21, 149, 159, 144, 169, 179, 189, 199, 12, 34, 14}
	ops, ok := cffCharStringOps(data, nil, nil)
	if !ok || len(ops) != 5 || ops[1].operatorValue() != "c" || ops[2].operatorValue() != "c" {
		t.Fatalf("hflex ops = %#v, ok=%v", ops, ok)
	}
	x, _ := NumberValue(ops[2].operandsValue()[4])
	y, _ := NumberValue(ops[2].operandsValue()[5])
	if x != 210 || y != 0 {
		t.Fatalf("hflex endpoint = (%v,%v)", x, y)
	}
}

func TestCFFCharStringHFlex1CompensatesFinalVerticalDelta(t *testing.T) {
	data := []byte{139, 139, 21, 149, 141, 159, 142, 169, 179, 189, 143, 199, 12, 36, 14}
	ops, ok := cffCharStringOps(data, nil, nil)
	if !ok || len(ops) != 5 {
		t.Fatalf("hflex1 ops = %#v, ok=%v", ops, ok)
	}
	x, _ := NumberValue(ops[2].operandsValue()[4])
	y, _ := NumberValue(ops[2].operandsValue()[5])
	if x != 210 || y != 0 {
		t.Fatalf("hflex1 endpoint = (%v,%v)", x, y)
	}
}

func TestCFFCharStringFlex1ChoosesHorizontalFinalDelta(t *testing.T) {
	data := []byte{139, 139, 21, 149, 141, 159, 142, 169, 143, 179, 144, 189, 145, 199, 12, 37, 14}
	ops, ok := cffCharStringOps(data, nil, nil)
	if !ok || len(ops) != 5 {
		t.Fatalf("flex1 ops = %#v, ok=%v", ops, ok)
	}
	x, _ := NumberValue(ops[2].operandsValue()[4])
	y, _ := NumberValue(ops[2].operandsValue()[5])
	if x != 210 || y != 0 {
		t.Fatalf("flex1 endpoint = (%v,%v)", x, y)
	}
}

func TestCFFCharStringRCurveLineDoesNotDuplicateCurves(t *testing.T) {
	data := []byte{139, 139, 21, 149, 139, 149, 139, 149, 139, 149, 139, 24, 14}
	ops, ok := cffCharStringOps(data, nil, nil)
	if !ok || len(ops) != 5 || ops[1].operatorValue() != "c" || ops[2].operatorValue() != "l" {
		t.Fatalf("rcurveline ops = %#v, ok=%v", ops, ok)
	}
}

func TestCFFCharStringArithmeticFeedsGeometry(t *testing.T) {
	data := []byte{139, 139, 21, 149, 144, 12, 10, 22, 14}
	ops, ok := cffCharStringOps(data, nil, nil)
	if !ok || len(ops) != 4 || ops[1].operatorValue() != "m" {
		t.Fatalf("arithmetic geometry ops = %#v, ok=%v", ops, ok)
	}
	x, _ := NumberValue(ops[1].operandsValue()[0])
	if x != 15 {
		t.Fatalf("arithmetic hmoveto x = %v", x)
	}
}

func TestCFFCharStringArithmeticRejectsNonFiniteResults(t *testing.T) {
	for _, test := range []struct {
		op    int
		args  []float64
		label string
	}{
		{op: 1210, args: []float64{math.MaxFloat64, math.MaxFloat64}, label: "add overflow"},
		{op: 1212, args: []float64{1, 0}, label: "division by zero"},
		{op: 1226, args: []float64{-1}, label: "negative sqrt"},
	} {
		state := cffPathState{operands: test.args}
		if state.operator(test.op, nil, new(int), 0) {
			t.Fatalf("CFF accepted %s", test.label)
		}
	}
}

func TestCFFCharStringEqRemainsAvailableToCFF1(t *testing.T) {
	data := []byte{139, 139, 21, 140, 140, 12, 15, 22, 14}
	ops, ok := cffCharStringOps(data, nil, nil)
	if !ok || len(ops) != 4 {
		t.Fatalf("CFF1 eq ops = %#v, ok=%v", ops, ok)
	}
	x, _ := NumberValue(ops[1].operandsValue()[0])
	if x != 1 {
		t.Fatalf("CFF1 eq result = %v", x)
	}
}

func TestCFFCharStringRejectsEmptyBlendResult(t *testing.T) {
	if _, ok := cffCharStringOps([]byte{139, 12, 16}, nil, nil); ok {
		t.Fatal("CFF1 accepted a blend with zero results")
	}
}

func TestCFFCharStringRejectsUnknownEscapedOperator(t *testing.T) {
	if _, ok := cffCharStringOps([]byte{12, 38}, nil, nil); ok {
		t.Fatal("CFF1 accepted an unknown escaped operator")
	}
}

func TestParseCFFFDSelect(t *testing.T) {
	if got := fontdata.ParseCFFFDSelect([]byte{0, 2, 3}, 0, 2); got[0] != 2 || got[1] != 3 {
		t.Fatalf("format 0 FDSelect = %#v", got)
	}
	data := []byte{3, 0, 2, 0, 0, 1, 0, 2, 4, 0, 4}
	got := fontdata.ParseCFFFDSelect(data, 0, 4)
	if len(got) != 4 || got[0] != 1 || got[1] != 1 || got[2] != 4 || got[3] != 4 {
		t.Fatalf("format 3 FDSelect = %#v", got)
	}
	data = []byte{4, 0, 0, 0, 2, 0, 0, 0, 0, 0, 7, 0, 0, 0, 1, 0, 9, 0, 0, 0, 4}
	got = fontdata.ParseCFFFDSelect(data, 0, 4)
	if len(got) != 4 || got[0] != 7 || got[1] != 9 || got[2] != 9 || got[3] != 9 {
		t.Fatalf("format 4 FDSelect = %#v", got)
	}
	if got := fontdata.ParseCFFFDSelect([]byte{3, 0, 1, 0, 0, 0, 0}, 0, 2); got != nil {
		t.Fatalf("format 3 FDSelect accepted incomplete sentinel: %#v", got)
	}
	if got := fontdata.ParseCFFFDSelect([]byte{4, 0, 0, 0, 1, 0, 0, 0, 0, 0, 7, 0, 0, 0, 1}, 0, 2); got != nil {
		t.Fatalf("format 4 FDSelect accepted incomplete sentinel: %#v", got)
	}
}

func TestParseCFFCIDWidths(t *testing.T) {
	// Private DICT: nominalWidthX=500 and defaultWidthX=600.
	private := []byte{248, 136, 21, 248, 236, 20}
	data := []byte{0}
	privateOffset := len(data)
	data = append(data, private...)
	fdArrayOffset := len(data)
	fdDict := []byte{byte(139 + len(private)), byte(139 + privateOffset), 18}
	data = appendCFFIndex(data, fdDict)
	fdSelectOffset := len(data)
	data = append(data, 0, 0, 0)
	metadata := cffMetadata{
		fdArray:         fdArrayOffset,
		fdSelect:        fdSelectOffset,
		charstringsData: [][]byte{{14}, {140, 141, 22}},
	}
	got := parseCFFCIDWidths(data, &metadata)
	if got[0] != 600 || got[1] != 501 {
		t.Fatalf("CID CFF widths = %#v", got)
	}
}

func TestParseCFF2CIDWidthsEvaluatesCharStringBlend(t *testing.T) {
	private := []byte{139, 22, 248, 136, 239, 140, 23, 20}
	data := []byte{0}
	privateOffset := len(data)
	data = append(data, private...)
	fdArrayOffset := len(data)
	fdDict := []byte{byte(139 + len(private)), byte(139 + privateOffset), 18}
	data = appendCFF2Index(data, fdDict)
	fdSelectOffset := len(data)
	data = append(data, 0, 0, 0)
	metadata := cffMetadata{
		cff2:     true,
		fdArray:  fdArrayOffset,
		fdSelect: fdSelectOffset,
		variation: &cffVariationStore{
			regions: []cffVariationRegion{{start: []float64{0}, peak: []float64{1}, end: []float64{1}}},
			data:    []cffVariationData{{regions: []int{0}, rows: [][]float64{{100}}}},
		},
		charstringsData: [][]byte{{248, 136, 239, 140, 16, 139, 22}},
	}
	got := parseCFFCIDWidths(data, &metadata)
	if got[0] != 500 {
		t.Fatalf("CID CFF2 blended width = %#v, want 500", got)
	}
}

func TestParseCFFCIDRejectsFDSelectReferencesOutsideFDArray(t *testing.T) {
	data := []byte{0}
	fdArrayOffset := len(data)
	data = appendCFFIndex(data, []byte{14})
	fdSelectOffset := len(data)
	data = append(data, 0, 0, 1)
	metadata := cffMetadata{
		fdArray:         fdArrayOffset,
		fdSelect:        fdSelectOffset,
		charstringsData: [][]byte{{14}, {14}},
	}
	if got := parseCFFCIDWidths(data, &metadata); got != nil {
		t.Fatalf("CID widths accepted an invalid FDSelect reference: %#v", got)
	}

	fdByGlyph, localByFD, variationByFD := parseCFFCIDSubroutines(data, &metadata)
	if fdByGlyph != nil || localByFD != nil || variationByFD != nil {
		t.Fatalf("CID subroutines accepted an invalid FDSelect reference: %v %v %v", fdByGlyph, localByFD, variationByFD)
	}
}

func appendCFFIndex(data []byte, items ...[]byte) []byte {
	data = append(data, byte(len(items)>>8), byte(len(items)))
	if len(items) == 0 {
		return data
	}
	data = append(data, 1)
	offset := 1
	for _, item := range items {
		data = append(data, byte(offset))
		offset += len(item)
	}
	data = append(data, byte(offset))
	for _, item := range items {
		data = append(data, item...)
	}
	return data
}

func appendCFF2Index(data []byte, items ...[]byte) []byte {
	data = append(data, byte(len(items)>>24), byte(len(items)>>16), byte(len(items)>>8), byte(len(items)))
	if len(items) == 0 {
		return data
	}
	data = append(data, 1)
	offset := 1
	for _, item := range items {
		data = append(data, byte(offset))
		offset += len(item)
	}
	data = append(data, byte(offset))
	for _, item := range items {
		data = append(data, item...)
	}
	return data
}
