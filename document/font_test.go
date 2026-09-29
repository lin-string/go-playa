package document

import (
	"encoding/binary"
	"errors"
	"io"
	"math"
	"os"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/lin-string/go-playa/cacheconfig"
	"github.com/lin-string/go-playa/fontdata"
	"github.com/lin-string/go-playa/geometry"
	"golang.org/x/image/font/gofont/goregular"
)

func TestEmbeddedFontFileErrorPreservesCategoryAndDecodeCause(t *testing.T) {
	font := &Font{trueTypeData: []byte{1, 'A'}, trueTypeFilters: []string{"RunLengthDecode"}}
	data, extension, present, err := font.EmbeddedFontFileWithError()
	if data != nil || extension != ".ttf" || !present {
		t.Fatalf("embedded font result = %q, %q, %v; want nil, .ttf, true", data, extension, present)
	}
	if !errors.Is(err, errEmbeddedFontUnavailable) || !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Fatalf("embedded font error = %v; want category and underlying unexpected EOF", err)
	}
}

func TestFontPathCacheBudgetsFollowDocumentOptions(t *testing.T) {
	d := &Document{
		cacheOptionsConfigured: true,
		cacheOptions: cacheconfig.Options{
			CFFPathBytes: 1, TrueTypePathBytes: 2, Type1PathBytes: 3, Type3Bytes: 4,
		},
	}
	f := &Font{document: d}
	if got := f.cffPathCacheBudget(); got != 1 {
		t.Fatalf("CFF path cache budget = %d, want 1", got)
	}
	if got := f.trueTypePathCacheBudget(); got != 2 {
		t.Fatalf("TrueType path cache budget = %d, want 2", got)
	}
	if got := f.type1PathCacheBudget(); got != 3 {
		t.Fatalf("Type1 path cache budget = %d, want 3", got)
	}
	if got := f.type3CacheBudget(); got != 4 {
		t.Fatalf("Type3 cache budget = %d, want 4", got)
	}
}

func TestTrueTypeGlyphPathsUseEmbeddedOutline(t *testing.T) {
	font := &Font{
		fontType:          "TrueType",
		encoding:          map[byte]rune{'A': 'A'},
		trueTypeData:      append([]byte(nil), goregular.TTF...),
		fontMatrix:        geometry.Matrix{0.001, 0, 0, 0.001, 0, 0},
		trueTypePathCache: map[int][]ContentOp{},
	}
	glyph := newTestGlyphWithCode(font, []byte{'A'}, 0, identity())
	paths := 0
	for path, err := range glyph.PathsSeq() {
		if err != nil {
			t.Fatalf("TrueType glyph path error: %v", err)
		}
		if len(path.SegmentsCopy()) == 0 {
			t.Fatal("TrueType glyph path has no segments")
		}
		paths++
	}
	if paths == 0 {
		t.Fatal("TrueType glyph produced no paths")
	}
}

func TestFontsSharingEmbeddedTrueTypeReferenceReuseParsedProgram(t *testing.T) {
	programRef := Ref{Object: 20}
	d := &Document{fontCache: map[Ref]*Font{}}
	first := &Font{
		document: d, fontType: "TrueType", trueTypeRef: programRef,
		trueTypeData: append([]byte(nil), goregular.TTF...),
	}
	second := &Font{
		document: d, fontType: "TrueType", trueTypeRef: programRef,
		trueTypeData: append([]byte(nil), goregular.TTF...),
	}
	d.storeFont(Ref{Object: 1}, first)
	d.storeFont(Ref{Object: 2}, second)
	if _, err := first.ensureTrueTypeWithError(); err != nil {
		t.Fatal(err)
	}
	if _, err := second.ensureTrueTypeWithError(); err != nil {
		t.Fatal(err)
	}
	if first.trueTypeFont == nil || second.trueTypeFont != first.trueTypeFont {
		t.Fatalf("shared TrueType parse = first %p second %p", first.trueTypeFont, second.trueTypeFont)
	}
	for glyphID, text := range second.glyphIDToUnicode {
		second.glyphIDToUnicode[glyphID] = "changed"
		if first.glyphIDToUnicode[glyphID] != text {
			t.Fatalf("shared TrueType glyph map was mutable across fonts: first[%d] = %q, want %q", glyphID, first.glyphIDToUnicode[glyphID], text)
		}
		break
	}
	if len(d.trueTypePrograms) != 1 {
		t.Fatalf("document TrueType program cache = %#v", d.trueTypePrograms)
	}
}

func TestTrueTypeGlyphPathsLazilyResolveCIDCMap(t *testing.T) {
	base := &Font{
		fontType:     "TrueType",
		encoding:     map[byte]rune{'A': 'A'},
		trueTypeData: append([]byte(nil), goregular.TTF...),
	}
	glyphID := base.glyphIDForCode([]byte{'A'}, 0)
	if glyphID == 0 {
		t.Fatal("failed to resolve the TrueType glyph ID for A")
	}

	font := &Font{
		fontType:          "TrueType",
		cid:               true,
		trueTypeData:      append([]byte(nil), goregular.TTF...),
		cmapData:          []byte("1 begincodespacerange\n<0102> <0102>\nendcodespacerange\n1 begincidchar\n<0102> 65\nendcidchar"),
		cidToGID:          map[int]int{65: glyphID},
		trueTypePathCache: map[int][]ContentOp{},
	}
	if font.cmapParsed {
		t.Fatal("CMap parsed during font setup")
	}
	if got := font.glyphIDForCode([]byte{0x01, 0x02}, 0); got != glyphID {
		t.Fatalf("lazy CID glyph ID = %d, want %d", got, glyphID)
	}
	ops, ok, err := font.trueTypePathOps([]byte{0x01, 0x02})
	if err != nil {
		t.Fatalf("TrueType CID glyph path error: %v", err)
	}
	if !ok || len(ops) == 0 {
		t.Fatal("TrueType CID glyph produced no path")
	}
	if !font.cmapParsed || font.cmapData != nil {
		t.Fatalf("TrueType CID path did not finish lazy CMap parsing: parsed=%v data=%v", font.cmapParsed, font.cmapData != nil)
	}
}

func TestTrueTypeGlyphPathsUseSavedGIDWithoutUnicodeMapping(t *testing.T) {
	font := &Font{
		fontType:     "TrueType",
		trueTypeData: append([]byte(nil), goregular.TTF...),
	}
	base := &Font{
		fontType:     "TrueType",
		encoding:     map[byte]rune{'A': 'A'},
		trueTypeData: append([]byte(nil), goregular.TTF...),
	}
	glyphID := base.glyphIDForCode([]byte{'A'}, 0)
	if glyphID == 0 {
		t.Fatal("failed to resolve the TrueType glyph ID for A")
	}
	glyph := newTestGlyphWithCode(font, []byte{0x7f}, glyphID, identity())
	paths := 0
	for path, err := range glyph.PathsSeq() {
		if err != nil {
			t.Fatalf("TrueType saved-GID glyph path error: %v", err)
		}
		if len(path.SegmentsCopy()) == 0 {
			t.Fatal("TrueType saved-GID glyph path has no segments")
		}
		paths++
	}
	if paths == 0 {
		t.Fatal("TrueType saved-GID glyph produced no paths")
	}
}

func TestTrueTypeGlyphPathsReportMalformedCIDCMap(t *testing.T) {
	font := &Font{
		fontType:     "TrueType",
		cid:          true,
		trueTypeData: append([]byte(nil), goregular.TTF...),
		cmapData:     []byte("1 begincidchar\n<00>\nendcidchar"),
	}
	if _, ok, err := font.trueTypePathOps([]byte{0}); err == nil || ok {
		t.Fatalf("malformed CID CMap path result = ok:%v err:%v", ok, err)
	}
}

func TestGlyphIDForCodeUsesEmbeddedTrueTypeCMap(t *testing.T) {
	font := &Font{
		fontType:     "TrueType",
		encoding:     map[byte]rune{'A': 'A'},
		trueTypeData: append([]byte(nil), goregular.TTF...),
	}
	if got := font.glyphIDForCode([]byte{'A'}, 0); got == 0 {
		t.Fatal("glyphIDForCode returned the .notdef glyph for A")
	}
}

func TestDecodedGlyphBytesCopyDoesNotExposeSource(t *testing.T) {
	glyph := fontdata.BorrowedDecodedGlyph("", []byte{1, 2}, 0)
	copy := glyph.BytesCopy()
	copy[0] = 9
	if got := glyph.BytesCopy(); len(got) != 2 || got[0] != 1 {
		t.Fatal("decoded glyph bytes copy aliases source")
	}
}

func TestDecodedGlyphSnapshotsPreserveEmptyBytes(t *testing.T) {
	glyph := fontdata.BorrowedDecodedGlyph("", make([]byte, 0), 0)
	if glyph.BytesCopy() == nil || glyph.Finalize().BytesCopy() == nil {
		t.Fatal("empty decoded glyph bytes became nil")
	}
}

func TestEmptyEmbeddedFontResourcesReachLazyParsers(t *testing.T) {
	font := NewSimpleFont("empty-resources")
	font.cmapData = make([]byte, 0)
	font.toUnicodeData = make([]byte, 0)
	font.cidToGIDData = make([]byte, 0)
	font.trueTypeData = make([]byte, 0)
	font.cffData = make([]byte, 0)
	font.type1Data = make([]byte, 0)

	font.ensureCMap()
	font.ensureToUnicode()
	font.ensureCIDToGID()
	font.ensureTrueType()
	font.ensureCFF()
	font.ensureType1Encoding()
	if !font.cmapParsed || !font.toUnicodeParsed || !font.cidToGIDParsed || !font.trueTypeParsed || !font.cffParsed || !font.type1Parsed {
		t.Fatalf("empty embedded resources were not marked parsed: cmap=%v tounicode=%v cidtogid=%v truetype=%v cff=%v type1=%v", font.cmapParsed, font.toUnicodeParsed, font.cidToGIDParsed, font.trueTypeParsed, font.cffParsed, font.type1Parsed)
	}
	if font.cffErr == nil {
		t.Fatal("empty embedded CFF was accepted without a parse error")
	}
}

func TestEmbeddedFontFileSnapshotsLazyProgramWithoutRetainingIt(t *testing.T) {
	font := &Font{type1Data: []byte("type1-program")}
	data, extension, ok, err := font.EmbeddedFontFileWithError()
	if err != nil || !ok || extension != ".pfa" || string(data) != "type1-program" {
		t.Fatalf("embedded Type1 export = %q, %q, %v, %v", data, extension, ok, err)
	}
	data[0] = 'X'
	if string(font.type1Data) != "type1-program" {
		t.Fatalf("embedded export exposed source buffer: %q", font.type1Data)
	}
	font.ensureType1Encoding()
	if _, _, ok := font.EmbeddedFontFile(); ok {
		t.Fatal("embedded export remained available after lazy parser released input")
	}
}

func TestEmbeddedFontFileDecodesFilteredProgram(t *testing.T) {
	font := &Font{trueTypeData: []byte("68656c6c6f>"), trueTypeFilters: []string{"ASCIIHexDecode"}}
	data, extension, ok, err := font.EmbeddedFontFileWithError()
	if err != nil || !ok || extension != ".ttf" || string(data) != "hello" {
		t.Fatalf("filtered embedded export = %q, %q, %v, %v", data, extension, ok, err)
	}
}

func TestWriteFontFileMatchesPlayaOutputContract(t *testing.T) {
	dir := t.TempDir()
	font := &Font{name: "Demo Font/Subset+", type1Data: []byte("type1-program")}
	path, err := font.WriteFontFile(dir)
	if err != nil {
		t.Fatal(err)
	}
	if want := dir + "/DemoFontSubset+.pfa"; path != want {
		t.Fatalf("font output path = %q, want %q", path, want)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "type1-program" {
		t.Fatalf("font output = %q, want %q", data, "type1-program")
	}
}

func TestWriteFontFileReturnsEmptyPathWhenProgramIsAbsent(t *testing.T) {
	path, err := (&Font{name: "NoProgram"}).WriteFontFile(t.TempDir())
	if err != nil || path != "" {
		t.Fatalf("absent font output = %q, %v; want empty path and nil error", path, err)
	}
}

func TestFontMapClonesPreserveAbsentMaps(t *testing.T) {
	tests := []struct {
		name string
		copy func() any
	}{
		{"byte-rune", func() any { return cloneByteRuneMap(nil) }},
		{"byte-string", func() any { return cloneByteStringMap(nil) }},
		{"byte-float", func() any { return cloneByteFloatMap(nil) }},
		{"rune-float", func() any { return cloneRuneFloatMap(nil) }},
		{"int-float", func() any { return cloneIntFloatMap(nil) }},
		{"int-position", func() any { return cloneIntPositionMap(nil) }},
		{"uint16-string", func() any { return cloneUint16StringMap(nil) }},
		{"string", func() any { return cloneStringMap(nil) }},
		{"int-string", func() any { return cloneIntStringMap(nil) }},
		{"rune-int", func() any { return cloneRuneIntMap(nil) }},
		{"int-int", func() any { return cloneIntIntMap(nil) }},
		{"stream", func() any { return cloneStreamMap(nil) }},
		{"string-float", func() any { return cloneStringFloatMap(nil) }},
		{"string-bbox", func() any { return cloneStringBBoxMap(nil) }},
		{"content-ops", func() any { return cloneContentOpMap(nil) }},
		{"string-bool", func() any { return cloneStringBoolMap(nil) }},
		{"string-error", func() any { return cloneStringErrorMap(nil) }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := test.copy()
			if got == nil || (reflect.ValueOf(got).Kind() == reflect.Map && reflect.ValueOf(got).IsNil()) {
				return
			}
			t.Fatalf("cloned absent map = %#v, want nil", got)
		})
	}
}

func TestFontNestedByteSnapshotsPreserveEmptySlices(t *testing.T) {
	bytes := cloneByteSlices([][]byte{make([]byte, 0)})
	if bytes == nil || bytes[0] == nil {
		t.Fatal("empty nested font bytes were not preserved")
	}
	codespaces := cloneCodeSpaces([]CodeSpace{testCodeSpace(make([]byte, 0), make([]byte, 0))})
	if codespaces == nil || codespaces[0].LowCopy() == nil || codespaces[0].HighCopy() == nil {
		t.Fatal("empty font codespace bytes were not preserved")
	}
}

func TestCIDMetricAccessorsHaveErrorAwareVariants(t *testing.T) {
	font := NewSimpleFont("CID")
	font.cid = true
	font.cidWidths[7] = 600
	font.verticalWidths[7] = 800
	font.verticalPositions[7] = [2]float64{250, 770}

	if got, ok, err := font.CIDWidthWithError(7); err != nil || !ok || got != 600 {
		t.Fatalf("CID width = %v, %v, %v", got, ok, err)
	}
	if got, ok, err := font.VerticalWidthWithError(7); err != nil || !ok || got != 800 {
		t.Fatalf("vertical width = %v, %v, %v", got, ok, err)
	}
	if got, ok, err := font.VerticalPositionWithError(7); err != nil || !ok || got != [2]float64{250, 770} {
		t.Fatalf("vertical position = %v, %v, %v", got, ok, err)
	}
}

func TestFontResourcesCopyDoesNotExposeSource(t *testing.T) {
	font := &Font{resources: Dict{Name("Meta"): Dict{Name("Value"): String("original")}}}
	copy := font.ResourcesCopy()
	copy[Name("Meta")].(Dict)[Name("Value")] = String("changed")
	if got := font.resources[Name("Meta")].(Dict)[Name("Value")].(String); string(got) != "original" {
		t.Fatalf("font resources copy aliases source: %q", got)
	}
}

func TestFontVariationCoordinatesAreClampedOnOwnedSnapshot(t *testing.T) {
	font := NewSimpleFont("Variable")
	snapshot := font.WithVariationCoordinates([]float64{-2, 0.5, 2})
	got := snapshot.VariationCoordinatesCopy()
	want := []float64{-1, 0.5, 1}
	for index := range want {
		if got[index] != want[index] {
			t.Fatalf("variation coordinates = %#v, want %#v", got, want)
		}
	}
	coords := []float64{2}
	second := font.WithVariationCoordinates(coords)
	coords[0] = -2
	if got := second.VariationCoordinatesCopy()[0]; got != 1 {
		t.Fatalf("variation snapshot changed through input slice: %v", got)
	}
	if got := font.WithVariationCoordinates([]float64{math.NaN()}).VariationCoordinatesCopy()[0]; got != 0 {
		t.Fatalf("NaN variation coordinate = %v, want 0", got)
	}
}

func TestFontVariationSnapshotsPreserveEmptyCoordinates(t *testing.T) {
	font := NewSimpleFont("Variable")
	font.variationCoords = make([]float64, 0)
	snapshot := font.Finalize()
	if snapshot == nil || snapshot.variationCoords == nil || snapshot.VariationCoordinatesCopy() == nil {
		t.Fatal("empty variation coordinates were not preserved")
	}
}

func TestFontSnapshotsPreserveEmptyEmbeddedData(t *testing.T) {
	font := &Font{
		toUnicodeData: make([]byte, 0), toUnicodeFilters: make([]string, 0),
		toUnicodeParsed: true,
		cmapData:        make([]byte, 0), cmapFilters: make([]string, 0),
		cmapParsed:   true,
		cidToGIDData: make([]byte, 0), cidToGIDFilters: make([]string, 0),
		cidToGIDParsed: true,
		trueTypeData:   make([]byte, 0), trueTypeFilters: make([]string, 0),
		trueTypeParsed: true,
		cffData:        make([]byte, 0), cffFilters: make([]string, 0),
		cffParsed: true,
		type1Data: make([]byte, 0), type1Filters: make([]string, 0),
		type1Parsed: true,
	}
	snapshot := font.Finalize()
	if snapshot == nil || snapshot.toUnicodeData == nil || snapshot.toUnicodeFilters == nil ||
		snapshot.cmapData == nil || snapshot.cmapFilters == nil ||
		snapshot.cidToGIDData == nil || snapshot.cidToGIDFilters == nil ||
		snapshot.trueTypeData == nil || snapshot.trueTypeFilters == nil ||
		snapshot.cffData == nil || snapshot.cffFilters == nil ||
		snapshot.type1Data == nil || snapshot.type1Filters == nil {
		t.Fatalf("empty embedded font data was not preserved: %#v", snapshot)
	}
}

func TestFontVariationCoordinatesRecomputeCFF2PrivateWidth(t *testing.T) {
	font := &Font{
		cff2:           true,
		fontMatrix:     geometry.Matrix{0.001, 0, 0, 0.001, 0, 0},
		defaultWidth:   500,
		widths:         map[byte]float64{},
		glyphIDWidths:  map[int]float64{},
		cffGlyphIDs:    map[byte]int{65: 0},
		cffCharstrings: [][]byte{{139, 4}},
		cffVariationStore: &cffVariationStore{
			regions: []cffVariationRegion{{start: []float64{0}, peak: []float64{1}, end: []float64{1}}},
			data:    []cffVariationData{{regions: []int{0}, rows: [][]float64{{100}}}},
		},
		cffDefaultWidthVar: &cffBlendValue{base: 500, deltas: []float64{100}, regions: []int{0}},
	}
	varied := font.WithVariationCoordinates([]float64{1})
	if varied.defaultWidth != 600 {
		t.Fatalf("varied CFF2 default width = %v, want 600", varied.defaultWidth)
	}
	if got := varied.widths[65]; got != 600 {
		t.Fatalf("varied CFF2 width = %v, want 600", got)
	}
	if got := font.widths[65]; got != 0 {
		t.Fatalf("source font was mutated: width = %v", got)
	}
}

func TestFontVariationCoordinatesRecomputeCFF2CharStringWidthBlend(t *testing.T) {
	font := &Font{
		cff2:           true,
		fontMatrix:     geometry.Matrix{0.001, 0, 0, 0.001, 0, 0},
		defaultWidth:   500,
		widths:         map[byte]float64{},
		glyphIDWidths:  map[int]float64{},
		cffGlyphIDs:    map[byte]int{65: 0},
		cffCharstrings: [][]byte{{248, 136, 239, 140, 16, 139, 139, 21}},
		cffVariationStore: &cffVariationStore{
			regions: []cffVariationRegion{{start: []float64{0}, peak: []float64{1}, end: []float64{1}}},
			data:    []cffVariationData{{regions: []int{0}, rows: [][]float64{{100}}}},
		},
	}
	varied := font.WithVariationCoordinates([]float64{1})
	if got := varied.widths[65]; got != 600 {
		t.Fatalf("varied CFF2 CharString width = %v, want 600", got)
	}
	if got := font.widths[65]; got != 0 {
		t.Fatalf("source font was mutated: width = %v", got)
	}
}

func TestFontVariationCoordinatesRejectsOverflowingCFFScale(t *testing.T) {
	font := &Font{
		cff2:           true,
		fontMatrix:     geometry.Matrix{math.MaxFloat64, 0, 0, 0.001, 0, 0},
		defaultWidth:   500,
		widths:         map[byte]float64{},
		glyphIDWidths:  map[int]float64{},
		cffGlyphIDs:    map[byte]int{65: 0},
		cffCharstrings: [][]byte{{139, 4}},
	}
	varied := font.WithVariationCoordinates([]float64{0})
	if math.IsNaN(varied.defaultWidth) || math.IsInf(varied.defaultWidth, 0) {
		t.Fatalf("overflowing CFF scale produced non-finite default width: %v", varied.defaultWidth)
	}
	if width := varied.widths[65]; math.IsNaN(width) || math.IsInf(width, 0) {
		t.Fatalf("overflowing CFF scale produced non-finite glyph width: %v", width)
	}
}

func TestTrueTypeCMapRejectsOverflowingTableBounds(t *testing.T) {
	data := make([]byte, 28)
	binary.BigEndian.PutUint16(data[4:6], 1)
	copy(data[12:16], []byte("cmap"))
	binary.BigEndian.PutUint32(data[20:24], uint32(len(data)))
	binary.BigEndian.PutUint32(data[24:28], ^uint32(0))
	defer func() {
		if recovered := recover(); recovered != nil {
			t.Fatalf("malformed TrueType cmap caused panic: %v", recovered)
		}
	}()
	if got := fontdata.ParseTrueTypeCMapGlyphs(data); got != nil {
		t.Fatalf("malformed TrueType cmap returned %v", got)
	}
}

func TestTrueTypeCMapRejectsOverflowingGroupCounts(t *testing.T) {
	format10 := make([]byte, 20)
	binary.BigEndian.PutUint16(format10[0:2], 10)
	binary.BigEndian.PutUint32(format10[4:8], uint32(len(format10)))
	binary.BigEndian.PutUint32(format10[16:20], ^uint32(0))
	if got := fontdata.ParseTrueTypeCMapFormat10(format10); got != nil {
		t.Fatalf("format 10 accepted overflowing count: %v", got)
	}

	format12 := make([]byte, 16)
	binary.BigEndian.PutUint16(format12[0:2], 12)
	binary.BigEndian.PutUint32(format12[4:8], uint32(len(format12)))
	binary.BigEndian.PutUint32(format12[12:16], ^uint32(0))
	if got := fontdata.ParseTrueTypeCMapFormat12(format12); got != nil {
		t.Fatalf("format 12 accepted overflowing group count: %v", got)
	}
}

func TestTrueTypeCMapFormat4RejectsShortLength(t *testing.T) {
	data := make([]byte, 16)
	binary.BigEndian.PutUint16(data[0:2], 4)
	binary.BigEndian.PutUint16(data[2:4], 4)
	defer func() {
		if recovered := recover(); recovered != nil {
			t.Fatalf("short format 4 cmap caused panic: %v", recovered)
		}
	}()
	if got := fontdata.ParseTrueTypeCMapFormat4(data); got != nil {
		t.Fatalf("short format 4 cmap returned %v", got)
	}
}

func TestParseToUnicodeCodesRejectsHugeSourceRange(t *testing.T) {
	data := []byte("1 beginbfrange\n<00000000> <ffffffff> <0041>\nendbfrange")
	codes, err := ParseToUnicodeCodes(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(codes) != 0 {
		t.Fatalf("huge ToUnicode range expanded to %d entries", len(codes))
	}
}

func TestFontToUnicode(t *testing.T) {
	f := NewSimpleFont("F1")
	f.toUnicode, _ = ParseToUnicode([]byte("<0001> <0041>\n<0002> <4E2D>"))
	if got := f.Decode([]byte{1, 2}); got != "A中" {
		t.Fatalf("%q", got)
	}
}

func TestFontToUnicodeCodeValuePreservesWideSourceCodes(t *testing.T) {
	f := NewSimpleFont("WideToUnicode")
	f.toUnicodeData = []byte("1 beginbfchar\n<00010001> <0041>\nendbfchar")
	if got, ok, err := f.ToUnicodeCodeValueWithError([]byte{0, 1, 0, 1}); err != nil || !ok || got != "A" {
		t.Fatalf("wide ToUnicode code = %q, %v, %v", got, ok, err)
	}
	if got, ok := f.ToUnicodeCodeValue([]byte{0, 1, 0, 2}); ok || got != "" {
		t.Fatalf("missing ToUnicode code = %q, %v", got, ok)
	}
}

func TestDecodeGlyphsReturnsIndependentCodeBytes(t *testing.T) {
	font := NewSimpleFont("Test")
	font.codeMap = map[string]string{"A": "A"}
	data := []byte("A")
	glyphs := font.DecodeGlyphs(data)
	if len(glyphs) != 1 || string(glyphs[0].BytesCopy()) != "A" {
		t.Fatalf("decoded glyphs = %#v", glyphs)
	}
	data[0] = 'B'
	if string(glyphs[0].BytesCopy()) != "A" {
		t.Fatalf("public DecodeGlyphs aliases input: %q", glyphs[0].BytesCopy())
	}
}

func TestDecodeGlyphsSequenceIsRepeatableAndStopsEarly(t *testing.T) {
	font := NewSimpleFont("Helvetica")
	data := []byte("AB")
	var first []DecodedGlyph
	for glyph := range font.DecodeGlyphsSeq(data) {
		first = append(first, glyph.Finalize())
		break
	}
	if len(first) != 1 || first[0].Text() != "A" || string(first[0].BytesCopy()) != "A" {
		t.Fatalf("early glyph sequence = %#v", first)
	}
	var all []string
	for glyph := range font.DecodeGlyphsSeq(data) {
		all = append(all, glyph.Text())
	}
	if len(all) != 2 || all[0] != "A" || all[1] != "B" {
		t.Fatalf("repeatable glyph sequence = %#v", all)
	}
}

func TestDecodeGlyphsSequenceWithErrorReportsMalformedCMap(t *testing.T) {
	f := NewSimpleFont("CID")
	f.cid = true
	f.cmapData = []byte("1 begincidchar\n<00>\nendcidchar")
	for _, err := range f.DecodeGlyphsSeqWithError([]byte{0}) {
		if err == nil {
			t.Fatal("malformed CMap produced a glyph without error")
		}
		return
	}
	t.Fatal("malformed CMap produced no error")
}

func TestDecodeGlyphsSequenceWithErrorIgnoresTruncatedToUnicodeLikePlaya(t *testing.T) {
	f := NewSimpleFont("ToUnicode")
	f.toUnicodeData = []byte("1 beginbfchar\n<00>\nendbfchar")
	seen := false
	for _, err := range f.DecodeGlyphsSeqWithError([]byte("A")) {
		seen = true
		if err != nil {
			t.Fatalf("truncated ToUnicode produced an error: %v", err)
		}
	}
	if !seen {
		t.Fatal("truncated ToUnicode produced no glyph")
	}
}

func TestDecodeGlyphsSequenceWithErrorReportsCIDToGIDDecodeFailure(t *testing.T) {
	f := NewSimpleFont("CID")
	f.cid = true
	f.cidToGIDData = []byte("invalid")
	f.cidToGIDFilters = []string{"UnsupportedFilter"}
	for _, err := range f.DecodeGlyphsSeqWithError([]byte{0}) {
		if err == nil {
			t.Fatal("failed CIDToGIDMap produced a glyph without error")
		}
		return
	}
	t.Fatal("failed CIDToGIDMap produced no error")
}

func TestDecodeGlyphsSequenceWithErrorReportsTrueTypeDecodeFailure(t *testing.T) {
	f := NewSimpleFont("TrueType")
	f.trueTypeData = []byte("invalid")
	f.trueTypeFilters = []string{"UnsupportedFilter"}
	for _, err := range f.DecodeGlyphsSeqWithError([]byte("A")) {
		if err == nil {
			t.Fatal("failed TrueType stream produced a glyph without error")
		}
		return
	}
	t.Fatal("failed TrueType stream produced no error")
}

func TestDecodeGlyphsSequenceWithErrorReportsType1DecodeFailure(t *testing.T) {
	f := NewSimpleFont("Type1")
	f.type1Data = []byte("invalid")
	f.type1Filters = []string{"UnsupportedFilter"}
	for _, err := range f.DecodeGlyphsSeqWithError([]byte("A")) {
		if err == nil {
			t.Fatal("failed Type1 stream produced a glyph without error")
		}
		return
	}
	t.Fatal("failed Type1 stream produced no error")
}

func TestFontScalarAccessorsWithErrorPreserveLazyFailures(t *testing.T) {
	cmap := NewSimpleFont("CID")
	cmap.cid = true
	cmap.cmapData = []byte("1 begincidchar\n<00>\nendcidchar")
	if snapshot, err := cmap.CMapSnapshotWithError(); err == nil || snapshot != nil {
		t.Fatalf("CMap snapshot error = %v, snapshot=%#v", err, snapshot)
	}

	toUnicode := NewSimpleFont("ToUnicode")
	toUnicode.toUnicodeData = []byte("1 beginbfchar\n<00>\nendbfchar")
	if value, ok, err := toUnicode.ToUnicodeValueWithError(0); err != nil || ok || value != "" {
		t.Fatalf("truncated ToUnicode = %v, value=%q, ok=%v", err, value, ok)
	}

	cff := NewSimpleFont("CFF")
	cff.cffData = []byte{1, 0, 4}
	if _, ok, err := cff.GlyphNameWithError(65); err == nil || ok {
		t.Fatalf("glyph-name error = %v, ok=%v", err, ok)
	}
	if _, ok, err := cff.WidthValueWithError(65); err == nil || ok {
		t.Fatalf("width error = %v, ok=%v", err, ok)
	}
	if width, err := cff.WidthCodeWithError([]byte{65}); err == nil || width != 0 {
		t.Fatalf("width-code error = %v, width=%v", err, width)
	}

	type3 := NewSimpleFont("Type3")
	type3.type3 = true
	type3.document = &Document{}
	type3.glyphNames[65] = "A"
	type3.charProcs["A"] = newStream(nil, []byte("500 0 0 0 (bad) 100 d1"))
	if width, err := type3.WidthCodeWithError([]byte{65}); err == nil || width != 0 {
		t.Fatalf("Type3 width-code error = %v, width=%v", err, width)
	}
	if width, err := type3.WidthWithError(65); err != nil || width != type3.defaultWidth {
		t.Fatalf("Type3 single-byte width = %v, width=%v", err, width)
	}
	if displacement, err := type3.HDispWithError(65); err == nil || displacement != 0 {
		t.Fatalf("Type3 displacement error = %v, displacement=%v", err, displacement)
	}
	if bbox, err := type3.CharBBoxWithError(65); err == nil || bbox != [4]float64{} {
		t.Fatalf("Type3 bbox error = %v, bbox=%v", err, bbox)
	}
}

func TestFontFinalizeWithErrorReportsLazyFailures(t *testing.T) {
	font := NewSimpleFont("CFF")
	font.cffData = []byte{1, 0, 4}
	if snapshot, err := font.FinalizeWithError(); err == nil || snapshot != nil {
		t.Fatalf("font finalize error = %v, snapshot=%#v", err, snapshot)
	}

	font = NewSimpleFont("Helvetica")
	snapshot, err := font.FinalizeWithError()
	if err != nil || snapshot == nil || snapshot.document != nil {
		t.Fatalf("font finalize snapshot = %#v, err=%v", snapshot, err)
	}
}

func TestFontTextSpaceMetrics(t *testing.T) {
	f := NewSimpleFont("F")
	f.widths[65] = 600
	f.ascent = 800
	f.descent = -200
	if got := f.HDisp(65); got != 0.6 {
		t.Fatalf("horizontal displacement = %v, want 0.6", got)
	}
	if got := f.Position(65); got != [2]float64{} {
		t.Fatalf("simple-font position = %v", got)
	}
	if got := f.VDisp(65); got != 0 {
		t.Fatalf("simple-font vertical displacement = %v", got)
	}
	if got := f.CharBBox(65); got != [4]float64{0, -0.2, 0.6, 0.8} {
		t.Fatalf("character bbox = %v", got)
	}
}

func TestCIDVerticalTextSpaceMetrics(t *testing.T) {
	f := NewSimpleFont("CID")
	f.cid = true
	f.vertical = true
	f.cidWidths[7] = 500
	f.verticalWidths[7] = -880
	f.verticalPositions[7] = [2]float64{250, 770}
	f.ascent = 880
	f.descent = -120
	if got := f.HDisp(7); got != 0.5 {
		t.Fatalf("CID horizontal displacement = %v, want 0.5", got)
	}
	if got := f.VDisp(7); got != -0.88 {
		t.Fatalf("CID vertical displacement = %v, want -0.88", got)
	}
	if got, err := f.VDispWithError(7); err != nil || got != -0.88 {
		t.Fatalf("CID strict vertical displacement = %v, err=%v", got, err)
	}
	if got := f.Position(7); got != [2]float64{0.25, 0.77} {
		t.Fatalf("CID position = %v, want [0.25 0.77]", got)
	}
	if got, err := f.VPositionWithError(7); err != nil || got != [2]float64{250, 770} {
		t.Fatalf("CID strict position = %v, err=%v", got, err)
	}
	got := f.CharBBox(7)
	want := [4]float64{-0.25, -0.89, 0.25, 0.11}
	for i := range got {
		if math.Abs(got[i]-want[i]) > 1e-12 {
			t.Fatalf("CID character bbox = %v, want %v", got, want)
		}
	}
}

func TestCIDVerticalMetricsWithErrorPreserveLazyFailures(t *testing.T) {
	f := NewSimpleFont("CFF vertical")
	f.cid = true
	f.vertical = true
	f.cffData = []byte{1, 0, 4}
	if displacement, err := f.VDispWithError(7); err == nil || displacement != 0 {
		t.Fatalf("vertical displacement error = %v, displacement=%v", err, displacement)
	}
	if position, err := f.VPositionWithError(7); err == nil || position != [2]float64{} {
		t.Fatalf("vertical position error = %v, position=%v", err, position)
	}
}

func TestFontToUnicodeRange(t *testing.T) {
	f := NewSimpleFont("F1")
	f.toUnicode, _ = ParseToUnicode([]byte("<0001> <0003> <0041>"))
	if got := f.Decode([]byte{1, 2, 3}); got != "ABC" {
		t.Fatalf("%q", got)
	}
}

func TestParseToUnicodeAcceptsCIDSectionsUsedByBrokenPDFs(t *testing.T) {
	data := []byte("1 begincodespacerange\n<0001> <0004>\nendcodespacerange\n2 begincidchar\n<0001> 65\n<0002> 66\nendcidchar\n1 begincidrange\n<0003> <0004> 67\nendcidrange")
	mapping, err := ParseToUnicode(data)
	if err != nil || mapping[1] != "A" || mapping[2] != "B" || mapping[3] != "C" || mapping[4] != "D" {
		t.Fatalf("CID ToUnicode mapping = %#v, err=%v", mapping, err)
	}
	codes, err := ParseToUnicodeCodes(data)
	if err != nil || codes[string([]byte{0, 1})] != "A" || codes[string([]byte{0, 4})] != "D" {
		t.Fatalf("CID ToUnicode code mapping = %#v, err=%v", codes, err)
	}
}

func TestParseToUnicodeSkipsNegativeAndOverflowingCIDRanges(t *testing.T) {
	maxInt := int(^uint(0) >> 1)
	data := []byte("2 begincidrange\n<0001> <0002> -1\n<0003> <0004> " + strconv.Itoa(maxInt) + "\nendcidrange")
	mapping, err := ParseToUnicode(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(mapping) != 0 {
		t.Fatalf("invalid CID ranges produced mappings: %#v", mapping)
	}
	codes, err := ParseToUnicodeCodes(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(codes) != 0 {
		t.Fatalf("invalid CID ranges produced code mappings: %#v", codes)
	}
}

func TestParseToUnicodeSkipsInvalidUnicodeCIDValues(t *testing.T) {
	data := []byte("2 begincidchar\n<0001> 55296\n<0002> 1114112\nendcidchar")
	mapping, err := ParseToUnicode(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(mapping) != 0 {
		t.Fatalf("invalid Unicode CID values produced mappings: %#v", mapping)
	}
}

func TestParseToUnicodeRejectsMalformedCIDCharRecords(t *testing.T) {
	for _, source := range []string{
		"1 begincidchar\n<nothex> 65\nendcidchar",
		"1 begincidchar\n<0001> not-a-cid\nendcidchar",
	} {
		if _, err := ParseToUnicode([]byte(source)); err == nil {
			t.Fatalf("malformed ToUnicode cidchar record was accepted: %q", source)
		}
		if _, err := ParseToUnicodeCodes([]byte(source)); err == nil {
			t.Fatalf("malformed ToUnicode code cidchar record was accepted: %q", source)
		}
	}
}

func TestParseToUnicodeCodesRejectsOversizedSourceCodes(t *testing.T) {
	for _, source := range []string{
		"1 beginbfchar\n<0000000001> <0041>\nendbfchar",
		"1 begincidchar\n<0000000001> 65\nendcidchar",
	} {
		if _, err := ParseToUnicodeCodes([]byte(source)); err == nil {
			t.Fatalf("oversized source code was accepted: %q", source)
		}
	}
}

func TestParseToUnicodeIgnoresTruncatedRecordsLikePlaya(t *testing.T) {
	for _, source := range []string{
		"1 beginbfchar\n<0001>\nendbfchar",
		"1 beginbfrange\n<0001> <0002>\nendbfrange",
		"1 begincidchar\n<0001>\nendcidchar",
		"1 begincidrange\n<0001> <0002>\nendcidrange",
	} {
		mapping, err := ParseToUnicode([]byte(source))
		if err != nil {
			t.Fatalf("truncated ToUnicode record failed: %q: %v", source, err)
		}
		if len(mapping) != 0 {
			t.Fatalf("truncated ToUnicode record produced mappings: %q: %#v", source, mapping)
		}
		codes, err := ParseToUnicodeCodes([]byte(source))
		if err != nil {
			t.Fatalf("truncated ToUnicode code record failed: %q: %v", source, err)
		}
		if len(codes) != 0 {
			t.Fatalf("truncated ToUnicode code record produced mappings: %q: %#v", source, codes)
		}
	}
}

func TestParseToUnicodeRejectsReversedRanges(t *testing.T) {
	for _, source := range []string{
		"1 beginbfrange\n<0002> <0001> <0041>\nendbfrange",
		"1 begincidrange\n<0002> <0001> 65\nendcidrange",
	} {
		if _, err := ParseToUnicode([]byte(source)); err == nil {
			t.Fatalf("reversed ToUnicode range was accepted: %q", source)
		}
		if _, err := ParseToUnicodeCodes([]byte(source)); err == nil {
			t.Fatalf("reversed ToUnicode code range was accepted: %q", source)
		}
	}
}

func TestParseToUnicodeAcceptsMismatchedRangeArraysLikePlaya(t *testing.T) {
	short := "1 beginbfrange\n<0001> <0002> [<0041>]\nendbfrange"
	codes, err := ParseToUnicodeCodes([]byte(short))
	if err != nil || codes[string([]byte{0, 1})] != "A" || len(codes) != 1 {
		t.Fatalf("short ToUnicode range array = %#v, err=%v", codes, err)
	}
	long := "1 beginbfrange\n<0001> <0001> [<0041> <0042>]\nendbfrange"
	codes, err = ParseToUnicodeCodes([]byte(long))
	if err != nil || codes[string([]byte{0, 1})] != "A" || len(codes) != 1 {
		t.Fatalf("long ToUnicode range array = %#v, err=%v", codes, err)
	}
}

func TestParseToUnicodeRejectsMalformedRangeArrayValues(t *testing.T) {
	source := "1 beginbfrange\n<0001> <0001> [<nothex>]\nendbfrange"
	if _, err := ParseToUnicode([]byte(source)); err == nil {
		t.Fatal("malformed ToUnicode range array value was accepted")
	}
	if _, err := ParseToUnicodeCodes([]byte(source)); err == nil {
		t.Fatal("malformed ToUnicode code range array value was accepted")
	}
}

func TestParseToUnicodeRejectsMalformedScalarRangeValues(t *testing.T) {
	for _, source := range []string{
		"1 beginbfrange\n<nothex> <0001> <0041>\nendbfrange",
		"1 beginbfrange\n<0001> <0001> <nothex>\nendbfrange",
		"1 beginbfrange\n<0001> <0001> not-a-unicode-value\nendbfrange",
		"1 begincidrange\n<0001> <nothex> 65\nendcidrange",
		"1 begincidrange\n<0001> <0001> not-a-cid\nendcidrange",
	} {
		if _, err := ParseToUnicode([]byte(source)); err == nil {
			t.Fatalf("malformed ToUnicode scalar range value was accepted: %q", source)
		}
		if _, err := ParseToUnicodeCodes([]byte(source)); err == nil {
			t.Fatalf("malformed ToUnicode code scalar range value was accepted: %q", source)
		}
	}
}

func TestParseToUnicodeRejectsUTF16RangeOverflow(t *testing.T) {
	source := "1 beginbfrange\n<0001> <0002> <FFFF>\nendbfrange"
	if _, err := ParseToUnicode([]byte(source)); err == nil {
		t.Fatal("ToUnicode accepted a UTF-16 range overflow")
	}
	if _, err := ParseToUnicodeCodes([]byte(source)); err == nil {
		t.Fatal("ToUnicode code parser accepted a UTF-16 range overflow")
	}
}

func TestParseToUnicodeRejectsOddUTF16Values(t *testing.T) {
	for _, source := range []string{
		"1 beginbfchar\n<0001> <0041FF>\nendbfchar",
		"1 beginbfrange\n<0001> <0001> <0041FF>\nendbfrange",
		"1 beginbfrange\n<0001> <0001> [<0041FF>]\nendbfrange",
	} {
		if _, err := ParseToUnicode([]byte(source)); err == nil {
			t.Fatalf("ToUnicode accepted an odd-length UTF-16 value: %q", source)
		}
		if _, err := ParseToUnicodeCodes([]byte(source)); err == nil {
			t.Fatalf("ToUnicode code parser accepted an odd-length UTF-16 value: %q", source)
		}
	}
}

func TestParseToUnicodeRejectsEmptyUTF16Values(t *testing.T) {
	for _, source := range []string{
		"1 beginbfchar\n<0001> <>\nendbfchar",
		"1 beginbfrange\n<0001> <0001> <>\nendbfrange",
		"1 beginbfrange\n<0001> <0001> [<>]\nendbfrange",
	} {
		if _, err := ParseToUnicode([]byte(source)); err == nil {
			t.Fatalf("ToUnicode accepted an empty UTF-16 value: %q", source)
		}
		if _, err := ParseToUnicodeCodes([]byte(source)); err == nil {
			t.Fatalf("ToUnicode code parser accepted an empty UTF-16 value: %q", source)
		}
	}
}

func TestParseToUnicodeRejectsMalformedBFCharRecords(t *testing.T) {
	source := "2 beginbfchar\n<0001> <0041> <nothex> <0042>\nendbfchar"
	if _, err := ParseToUnicode([]byte(source)); err == nil {
		t.Fatal("malformed ToUnicode bfchar record was accepted")
	}
	if _, err := ParseToUnicodeCodes([]byte(source)); err == nil {
		t.Fatal("malformed ToUnicode code bfchar record was accepted")
	}
}

func TestParseToUnicodeAcceptsWrappedArrayRanges(t *testing.T) {
	source := `/CIDInit /ProcSet findresource begin
begincmap
1 begincodespacerange
<0000> <FFFF>
endcodespacerange
2 beginbfrange
<00B2> <00B2> [<2014>]
<00B3> <00B6> <201C>
endbfrange
endcmap
end`
	mapping, err := ParseToUnicode([]byte(source))
	if err != nil {
		t.Fatal(err)
	}
	if mapping[0x00b2] != "—" || mapping[0x00b5] != "„" || mapping[0x00b6] != "‟" {
		t.Fatalf("wrapped ToUnicode mapping = %#v", mapping)
	}
	codes, err := ParseToUnicodeCodes([]byte(source))
	if err != nil {
		t.Fatal(err)
	}
	if codes[string([]byte{0, 0xb5})] != "„" || codes[string([]byte{0, 0xb6})] != "‟" {
		t.Fatalf("wrapped ToUnicode code mapping = %#v", codes)
	}
}

func TestUTF16SurrogatePair(t *testing.T) {
	if got := utf16Bytes([]byte{0xd8, 0x3d, 0xde, 0x00}); got != "😀" {
		t.Fatalf("%q", got)
	}
}

func TestUTF16UnpairedSurrogatesAreIgnored(t *testing.T) {
	if got := utf16Bytes([]byte{0xd8, 0x3d, 0x00, 0x41, 0xdc, 0x00}); got != "A" {
		t.Fatalf("unpaired UTF-16 surrogates = %q, want A", got)
	}
}

func TestUTF16OddTrailingByteIsIgnored(t *testing.T) {
	if got := utf16Bytes([]byte{0x00, 0x41, 0xff}); got != "A" {
		t.Fatalf("odd UTF-16 destination = %q, want A", got)
	}
}

func TestTrueTypeCMapFormat12MapsSupplementaryCharacters(t *testing.T) {
	data := make([]byte, 16+12)
	binary.BigEndian.PutUint16(data[0:2], 12)
	binary.BigEndian.PutUint32(data[4:8], uint32(len(data)))
	binary.BigEndian.PutUint32(data[12:16], 1)
	binary.BigEndian.PutUint32(data[16:20], 0x20000)
	binary.BigEndian.PutUint32(data[20:24], 0x20001)
	binary.BigEndian.PutUint32(data[24:28], 37)
	got := fontdata.ParseTrueTypeCMapFormat12(data)
	if got[37] != string(rune(0x20000)) || got[38] != string(rune(0x20001)) {
		t.Fatalf("format 12 cmap = %#v", got)
	}
}

func TestTrueTypeCMapFormat0MapsByteCharacters(t *testing.T) {
	data := make([]byte, 6+256)
	binary.BigEndian.PutUint16(data[0:2], 0)
	binary.BigEndian.PutUint16(data[2:4], uint16(len(data)))
	data[6+'A'] = 17
	got := fontdata.ParseTrueTypeCMapFormat0(data)
	if got[17] != "A" {
		t.Fatalf("format 0 cmap = %#v", got)
	}
}

func TestTrueTypeCMapFormat6MapsTrimmedCharacterRange(t *testing.T) {
	data := make([]byte, 10+2*2)
	binary.BigEndian.PutUint16(data[0:2], 6)
	binary.BigEndian.PutUint16(data[2:4], uint16(len(data)))
	binary.BigEndian.PutUint16(data[6:8], 0x20)
	binary.BigEndian.PutUint16(data[8:10], 2)
	binary.BigEndian.PutUint16(data[10:12], 31)
	binary.BigEndian.PutUint16(data[12:14], 32)
	got := fontdata.ParseTrueTypeCMapFormat6(data)
	if got[31] != " " || got[32] != "!" {
		t.Fatalf("format 6 cmap = %#v", got)
	}
}

func TestTrueTypeCMapFormat10MapsWideCharacterRange(t *testing.T) {
	data := make([]byte, 20+2*2)
	binary.BigEndian.PutUint16(data[0:2], 10)
	binary.BigEndian.PutUint32(data[4:8], uint32(len(data)))
	binary.BigEndian.PutUint32(data[12:16], 0x10000)
	binary.BigEndian.PutUint32(data[16:20], 2)
	binary.BigEndian.PutUint16(data[20:22], 41)
	binary.BigEndian.PutUint16(data[22:24], 42)
	got := fontdata.ParseTrueTypeCMapFormat10(data)
	if got[41] != string(rune(0x10000)) || got[42] != string(rune(0x10001)) {
		t.Fatalf("format 10 cmap = %#v", got)
	}
}

func TestTrueTypeCMapFormat13MapsRangeToOneGlyph(t *testing.T) {
	data := make([]byte, 16+12)
	binary.BigEndian.PutUint16(data[0:2], 13)
	binary.BigEndian.PutUint32(data[4:8], uint32(len(data)))
	binary.BigEndian.PutUint32(data[12:16], 1)
	binary.BigEndian.PutUint32(data[16:20], 0x20000)
	binary.BigEndian.PutUint32(data[20:24], 0x2ffff)
	binary.BigEndian.PutUint32(data[24:28], 73)
	got := fontdata.ParseTrueTypeCMapFormat13(data)
	if got[73] != string(rune(0x20000)) {
		t.Fatalf("format 13 cmap = %#v", got)
	}
}

func TestTrueTypeCMapFormat8MapsMixedCharacterRange(t *testing.T) {
	data := make([]byte, 8204+12)
	binary.BigEndian.PutUint16(data[0:2], 8)
	binary.BigEndian.PutUint32(data[4:8], uint32(len(data)))
	binary.BigEndian.PutUint32(data[8200:8204], 1)
	binary.BigEndian.PutUint32(data[8204:8208], 0x10000)
	binary.BigEndian.PutUint32(data[8208:8212], 0x10001)
	binary.BigEndian.PutUint32(data[8212:8216], 53)
	got := fontdata.ParseTrueTypeCMapFormat8(data)
	if got[53] != string(rune(0x10000)) || got[54] != string(rune(0x10001)) {
		t.Fatalf("format 8 cmap = %#v", got)
	}
}

func TestTrueTypeCMapFormat2MapsHighByteSubheader(t *testing.T) {
	data := make([]byte, 6+512+8+2+4)
	binary.BigEndian.PutUint16(data[0:2], 2)
	binary.BigEndian.PutUint16(data[2:4], uint16(len(data)))
	// Subheader zero handles the low-byte range for single-byte codes.
	binary.BigEndian.PutUint16(data[6+0:6+2], 0)
	// firstCode=0x41, entryCount=1, idDelta=0, idRangeOffset=6.
	binary.BigEndian.PutUint16(data[518:520], 0x41)
	binary.BigEndian.PutUint16(data[520:522], 1)
	binary.BigEndian.PutUint16(data[522:524], 0)
	binary.BigEndian.PutUint16(data[524:526], 6)
	binary.BigEndian.PutUint16(data[530:532], 17)
	got := fontdata.ParseTrueTypeCMapFormat2(data)
	if got[17] != "A" {
		t.Fatalf("format 2 cmap = %#v", got)
	}
}

func TestFontToUnicodeArrayRange(t *testing.T) {
	m, _ := ParseToUnicode([]byte("<0001> <0003> [<0041> <0042> <0043>]"))
	if m[1] != "A" || m[2] != "B" || m[3] != "C" {
		t.Fatalf("%#v", m)
	}
}
func TestCodeMapDecode(t *testing.T) {
	f := NewSimpleFont("x")
	f.codeMap[string([]byte{9, 0x84})] = "中"
	if got := f.Decode([]byte{9, 0x84}); got != "中" {
		t.Fatalf("%q", got)
	}
}

func TestDecodeHonorsToUnicodeCodespaces(t *testing.T) {
	f := NewSimpleFont("F")
	f.toUnicodeSpaces = []CodeSpace{testCodeSpace([]byte{0x00, 0x01}, []byte{0x00, 0xff})}
	f.codeMap[string([]byte{0x00, 0x01})] = "X"
	if got := f.Decode([]byte{0x00, 0x01, 0x41}); got != "XA" {
		t.Fatalf("codespace-constrained decode = %q", got)
	}
}

func TestCIDToUnicodeMissFallsBackToSourceBeforeEmbeddedGlyphMap(t *testing.T) {
	f := NewSimpleFont("CID")
	f.cid = true
	f.cmap = testCMap([]CodeSpace{testCodeSpace([]byte{0, 0}, []byte{0xff, 0xff})}, map[string]int{string([]byte{0, 1}): 1}, false)
	f.glyphIDToUnicode = map[int]string{1: "\x00"}
	f.toUnicodeData = []byte("1 begincodespacerange\n<0000> <ffff>\nendcodespacerange\n1 beginbfchar\n<0003> <0020>\nendbfchar")
	got := f.DecodeGlyphs([]byte{0, 1})
	if len(got) != 1 || got[0].Text() != string(rune(1)) {
		t.Fatalf("ToUnicode miss = %#v, want source U+0001", got)
	}
}

func TestCIDToUnicodeWithoutValidCodespacesUsesByteFallback(t *testing.T) {
	f := NewSimpleFont("CID")
	f.cid = true
	f.cmap = testCMap([]CodeSpace{testCodeSpace([]byte{0, 0}, []byte{0xff, 0xff})}, map[string]int{string([]byte{0, 1}): 1}, false)
	f.toUnicodeData = []byte("1 begincodespacerange\n<00> <0001>\nendcodespacerange\n1 beginbfchar\n<0001> <0058>\nendbfchar")
	glyphs := f.DecodeGlyphs([]byte{0, 1})
	if len(glyphs) != 1 || glyphs[0].Text() != string(rune(0)) {
		t.Fatalf("invalid ToUnicode codespace fallback = %#v, want first source byte", glyphs)
	}
}

func TestSimpleFontForcesToUnicodeToSingleByteCodespace(t *testing.T) {
	f := NewSimpleFont("F")
	f.toUnicodeData = []byte("1 begincodespacerange\n<0000> <00ff>\nendcodespacerange\n1 beginbfchar\n<0001> <0058>\nendbfchar")
	if got := f.Decode([]byte{0x00, 0x01}); got != string([]byte{0x00, 0x01}) {
		t.Fatalf("simple-font ToUnicode decode = %q, want two single-byte fallbacks", got)
	}
}

func TestDecodeGlyphsPreservesCIDCodeAndWidth(t *testing.T) {
	f := NewSimpleFont("F")
	f.cid = true
	f.toUnicode = map[uint16]string{}
	f.codeMap[string([]byte{0x00, 0x01})] = "\u4e00"
	f.cidWidths[1] = 1004
	glyphs := f.DecodeGlyphs([]byte{0x00, 0x01})
	if len(glyphs) != 1 || glyphs[0].Text() != "\u4e00" || glyphs[0].CID() != 1 {
		t.Fatalf("unexpected glyphs: %#v", glyphs)
	}
	if got := f.WidthCode(glyphs[0].BytesCopy()); got != 1004 {
		t.Fatalf("CID width = %v, want 1004", got)
	}
}

func TestWinAnsiEncoding(t *testing.T) {
	f := NewSimpleFont("F")
	f.applyEncoding("WinAnsiEncoding")
	if got := f.Decode([]byte{0x97, 0xb2, 0xb7}); got != "—²·" {
		t.Fatalf("WinAnsi decode = %q", got)
	}
	if got := f.Decode([]byte{0xa0}); got != " " {
		t.Fatalf("WinAnsi space glyph = %q", got)
	}
}

func TestGlyphTextSupportsAdobeDynamicNames(t *testing.T) {
	if got, ok := glyphText("uni20AC0308"); !ok || got != "€\u0308" {
		t.Fatalf("multi-codepoint glyph name = %q, ok=%v", got, ok)
	}
	if got, ok := glyphText("A_B"); !ok || got != "AB" {
		t.Fatalf("composite glyph name = %q, ok=%v", got, ok)
	}
	if got, ok := glyphText("u1040C"); !ok || got != "𐐌" {
		t.Fatalf("wide glyph name = %q, ok=%v", got, ok)
	}
	if got, ok := glyphText("uniD801DC0C"); ok || got != "" {
		t.Fatalf("surrogate glyph name = %q, ok=%v", got, ok)
	}
}

func TestStandardAndMacRomanEncodings(t *testing.T) {
	standard := NewSimpleFont("F")
	standard.applyEncoding("StandardEncoding")
	if got := standard.Decode([]byte{0x60, 0x7e}); got != "‘~" {
		t.Fatalf("Standard decode = %q", got)
	}
	if got := standard.Decode([]byte{0xa1, 0xa4, 0xb1, 0xb2}); got != "¡⁄–†" {
		t.Fatalf("Standard high-byte decode = %q", got)
	}

	mac := NewSimpleFont("F")
	mac.applyEncoding("MacRomanEncoding")
	if got := mac.Decode([]byte{0x80, 0x8e}); got != "Äé" {
		t.Fatalf("MacRoman decode = %q", got)
	}
}

func TestStandardEncodingLeavesUndefinedHighSlotsUnmapped(t *testing.T) {
	font := NewSimpleFont("F")
	font.applyEncoding("StandardEncoding")
	if got := font.Decode([]byte{0x80, 0x81, 0x8f, 0x90}); got != "" {
		t.Fatalf("undefined StandardEncoding slots decoded as %q", got)
	}
}

func TestMacExpertEncoding(t *testing.T) {
	// PDF MacExpert assignments; the separate CFF Expert table uses other slots.
	f := NewSimpleFont("F")
	f.applyEncoding("MacExpertEncoding")
	if got := f.Decode([]byte{33, 34, 35, 36, 37, 38, 39, 40, 41, 42, 43, 47, 48, 49}); got != "\uf721\uf6f8\uf7a2\uf724\uf6e4\uf726\uf7b4⁽⁾‥․⁄\uf730\uf731" {
		t.Fatalf("MacExpert decode = %q", got)
	}
}

func TestMacExpertEncodingMapsSupplementalSlots(t *testing.T) {
	// PDF MacExpert assignments; the separate CFF Expert table uses other slots.
	f := NewSimpleFont("F")
	f.applyEncoding("MacExpertEncoding")
	if got := f.Decode([]byte{71, 72, 73, 74, 75, 76, 77, 78, 79, 86, 87, 88, 89, 90}); got != "¼½¾⅛⅜⅝⅞⅓⅔ﬀﬁﬂﬃﬄ" {
		t.Fatalf("MacExpert decode = %q", got)
	}
}

func TestMacExpertEncodingMapsSuperiorSlots(t *testing.T) {
	// PDF MacExpert assignments; the separate CFF Expert table uses other slots.
	f := NewSimpleFont("F")
	f.applyEncoding("MacExpertEncoding")
	if got := f.Decode([]byte{226, 218, 219, 220, 221, 222, 223, 224, 161, 225, 188, 193, 170, 163, 162, 176, 164, 166, 165, 187}); got != "⁰¹²³⁴⁵⁶⁷⁸⁹₀₁₂₃₄₅₆₇₈₉" {
		t.Fatalf("MacExpert decode = %q", got)
	}
}

func TestSymbolEncoding(t *testing.T) {
	f := NewSimpleFont("Symbol")
	f.applyEncoding("Symbol")
	if got := f.Decode([]byte("ABGPa b g p")); got != "ΑΒΓΠα β γ π" {
		t.Fatalf("Symbol decode = %q", got)
	}
}

func TestBuiltInEncodingsDoNotASCIIFallbackUndefinedSlots(t *testing.T) {
	for _, name := range []string{"MacExpertEncoding", "Symbol", "ZapfDingbats"} {
		f := NewSimpleFont("F")
		f.applyEncoding(name)
		if got := f.Decode([]byte{127}); got != "" {
			t.Fatalf("%s undefined slot decoded as %q", name, got)
		}
	}

	f := NewSimpleFont("F")
	f.applyEncoding("MacExpertEncoding")
	if got := f.Decode([]byte{64, 70}); got != "" {
		t.Fatalf("MacExpert undefined slots decoded as %q", got)
	}
}

func TestApplyEncodingReplacesPreviousEncoding(t *testing.T) {
	f := NewSimpleFont("F")
	f.applyEncoding("MacExpertEncoding")
	f.applyEncoding("Symbol")
	if got := f.Decode([]byte{240}); got != "" {
		t.Fatalf("stale MacExpert slot survived Symbol encoding: %q", got)
	}

	f.applyEncoding("WinAnsiEncoding")
	if got := f.Decode([]byte{33}); got != "!" {
		t.Fatalf("WinAnsi ASCII slot = %q", got)
	}
}

func TestFontEncodingChangesInvalidatePublishedWidthState(t *testing.T) {
	font := NewSimpleFont("mutable-encoding")
	font.widths[65] = 600
	if got := font.WidthCode([]byte{65}); got != 600 {
		t.Fatalf("initial WidthCode = %v, want 600", got)
	}
	if font.widthStateLoad() == nil {
		t.Fatal("WidthCode did not publish its immutable state")
	}

	font.applyDifferences(Array{Number(65), Name("unknownGlyph")})
	if font.widthStateLoad() != nil {
		t.Fatal("applyDifferences retained stale width state")
	}
	font.applyEncoding("Symbol")
	if font.widthStateLoad() != nil {
		t.Fatal("applyEncoding retained stale width state")
	}
}

func TestSymbolEncodingMapsMathSlots(t *testing.T) {
	f := NewSimpleFont("Symbol")
	f.applyEncoding("Symbol")
	if got := f.Decode([]byte{34, 36, 39, 64, 92, 160, 165, 177, 179, 180, 185}); got != "∀∃∋≅∴€∞±≥×≠" {
		t.Fatalf("Symbol math decode = %q", got)
	}
}

func TestSymbolEncodingMapsAdditionalMathSlots(t *testing.T) {
	f := NewSimpleFont("Symbol")
	f.applyEncoding("Symbol")
	if got := f.Decode([]byte{210, 211, 212, 226, 227, 228, 243, 245}); got != "®©™®©™⌠⌡" {
		t.Fatalf("Symbol additional slots = %q", got)
	}
}

func TestSymbolEncodingIncludesGeneratedAFMSlots(t *testing.T) {
	f := NewSimpleFont("Symbol")
	f.applyEncoding("Symbol")
	if got := f.Decode([]byte{188}); got != "…" {
		t.Fatalf("generated Symbol AFM slot = %q, want ellipsis", got)
	}
}

func TestSymbolEncodingMapsStructuralSlots(t *testing.T) {
	f := NewSimpleFont("Symbol")
	f.applyEncoding("Symbol")
	if got := f.Decode([]byte{230, 239, 244, 246, 254}); got != "\uf8eb\uf8f4\uf8f5\uf8f6\uf8fe" {
		t.Fatalf("Symbol structural slots = %q", got)
	}
}

func TestSymbolGlyphNamesMapStructuralSlots(t *testing.T) {
	f := NewSimpleFont("F")
	f.applyDifferences(Array{Number(65), Name("parenlefttp"), Name("integralex"), Name("bracerightbt")})
	if got := f.Decode([]byte{65, 66, 67}); got != "\uf8eb\uf8f5\uf8fe" {
		t.Fatalf("Symbol structural glyph names = %q", got)
	}
}

func TestZapfDingbatsEncoding(t *testing.T) {
	f := NewSimpleFont("ZapfDingbats")
	f.applyEncoding("ZapfDingbats")
	if got := f.Decode([]byte{33, 34, 35, 36, 37, 38, 39, 40, 41, 42, 43, 44, 45, 46, 47, 48, 49, 50, 51, 52}); got != "✁✂✃✄☎✆✇✈✉☛☞✌✍✎✏✐✑✒✓✔" {
		t.Fatalf("ZapfDingbats decode = %q", got)
	}
}

func TestZapfDingbatsEncodingMapsDecorativeSlots(t *testing.T) {
	f := NewSimpleFont("ZapfDingbats")
	f.applyEncoding("ZapfDingbats")
	if got := f.Decode([]byte{72, 73, 74, 75, 76, 77, 78, 79, 80, 81, 82, 83, 84, 85, 86, 87, 88, 89, 90, 91, 92}); got != "★✩✪✫✬✭✮✯✰✱✲✳✴✵✶✷✸✹✺✻✼" {
		t.Fatalf("ZapfDingbats decorative decode = %q", got)
	}
}

func TestZapfDingbatsEncodingMapsFlorettesAndShapes(t *testing.T) {
	f := NewSimpleFont("ZapfDingbats")
	f.applyEncoding("ZapfDingbats")
	if got := f.Decode([]byte{93, 94, 95, 96, 97, 98, 99, 100, 101, 102, 103, 104, 105, 106, 107, 108, 109, 110, 111, 112}); got != "✽✾✿❀❁❂❃❄❅❆❇❈❉❊❋●❍■❏❐" {
		t.Fatalf("ZapfDingbats florettes decode = %q", got)
	}
}

func TestZapfDingbatsEncodingMapsOrnamentsAndBrackets(t *testing.T) {
	f := NewSimpleFont("ZapfDingbats")
	f.applyEncoding("ZapfDingbats")
	if got := f.Decode([]byte{113, 114, 115, 116, 117, 118, 119, 120, 121, 122, 123, 124, 125, 126}); got != "❑❒▲▼◆❖◗❘❙❚❛❜❝❞" {
		t.Fatalf("ZapfDingbats ornament decode = %q", got)
	}
	if got := f.Decode([]byte{128, 129, 130, 131, 132, 133, 134, 135, 136, 137, 138, 139, 140, 141}); got != "\uf8d7\uf8d8\uf8d9\uf8da\uf8db\uf8dc\uf8dd\uf8de\uf8df\uf8e0\uf8e1\uf8e2\uf8e3\uf8e4" {
		t.Fatalf("ZapfDingbats bracket decode = %q", got)
	}
}

func TestZapfDingbatsEncodingMapsHeartsAndCircledNumbers(t *testing.T) {
	f := NewSimpleFont("ZapfDingbats")
	f.applyEncoding("ZapfDingbats")
	if got := f.Decode([]byte{161, 162, 163, 164, 165, 166, 167, 168, 169, 170, 171}); got != "❡❢❣❤❥❦❧♣♦♥♠" {
		t.Fatalf("ZapfDingbats heart decode = %q", got)
	}
	if got := f.Decode([]byte{172, 173, 174, 175, 176, 177, 178, 179, 180, 181, 182, 183, 184, 185, 186, 187, 188, 189, 190, 191}); got != "①②③④⑤⑥⑦⑧⑨⑩❶❷❸❹❺❻❼❽❾❿" {
		t.Fatalf("ZapfDingbats circled number decode = %q", got)
	}
}

func TestZapfDingbatsEncodingMapsEnclosedNumbers(t *testing.T) {
	f := NewSimpleFont("ZapfDingbats")
	f.applyEncoding("ZapfDingbats")
	if got := f.Decode([]byte{192, 193, 194, 195, 196, 197, 198, 199, 200, 201, 202}); got != "➀➁➂➃➄➅➆➇➈➉➊" {
		t.Fatalf("ZapfDingbats enclosed number decode = %q", got)
	}
}

func TestZapfDingbatsEncodingMapsArrows(t *testing.T) {
	f := NewSimpleFont("ZapfDingbats")
	f.applyEncoding("ZapfDingbats")
	if got := f.Decode([]byte{203, 204, 205, 206, 207, 208, 209, 210, 211, 212, 213, 214, 215}); got != "➋➌➍➎➏➐➑➒➓➔→↔↕" {
		t.Fatalf("ZapfDingbats arrow decode = %q", got)
	}
}

func TestZapfDingbatsEncodingMapsDecorativeArrows(t *testing.T) {
	f := NewSimpleFont("ZapfDingbats")
	f.applyEncoding("ZapfDingbats")
	if got := f.Decode([]byte{216, 217, 218, 219, 220, 221, 222, 223, 224, 225, 226, 227, 228, 229, 230, 231, 232, 233, 234, 235, 236, 237, 238, 239, 241, 242, 243, 244, 245, 246, 247, 248, 249, 250, 251, 252, 253, 254}); got != "➘➙➚➛➜➝➞➟➠➡➢➣➤➥➦➧➨➩➪➫➬➭➮➯➱➲➳➴➵➶➷➸➹➺➻➼➽➾" {
		t.Fatalf("ZapfDingbats decorative arrow decode = %q", got)
	}
}

func TestDifferencesDecodeZapfDingbatsGlyphNames(t *testing.T) {
	f := NewSimpleFont("F")
	f.applyDifferences(Array{Number(65), Name("a1"), Name("a2"), Name("a202"), Name("a4"), Name("a11"), Name("a19")})
	if got := f.Decode([]byte{65, 66, 67, 68, 69, 70}); got != "✁✂✃☎☛✓" {
		t.Fatalf("Zapf Differences decode = %q", got)
	}
}

func TestDifferencesDecodeZapfDingbatsOrnamentNames(t *testing.T) {
	f := NewSimpleFont("F")
	f.applyDifferences(Array{Number(65), Name("a35"), Name("a56"), Name("a64"), Name("a71"), Name("a76"), Name("a100")})
	if got := f.Decode([]byte{65, 66, 67, 68, 69, 70}); got != "★✽❅●▲❞" {
		t.Fatalf("Zapf ornament Differences decode = %q", got)
	}
}

func TestDifferencesDecodeZapfDingbatsHeartAndNumberNames(t *testing.T) {
	f := NewSimpleFont("F")
	f.applyDifferences(Array{Number(65), Name("a101"), Name("a104"), Name("a109"), Name("a120"), Name("a130"), Name("a140")})
	if got := f.Decode([]byte{65, 66, 67, 68, 69, 70}); got != "❡❤♠①❶➀" {
		t.Fatalf("Zapf heart/number Differences decode = %q", got)
	}
}

func TestDifferencesDecodeZapfDingbatsArrowNames(t *testing.T) {
	f := NewSimpleFont("F")
	f.applyDifferences(Array{Number(65), Name("a160"), Name("a161"), Name("a163"), Name("a151"), Name("a174"), Name("a191")})
	if got := f.Decode([]byte{65, 66, 67, 68, 69, 70}); got != "➔→↔➋➤➾" {
		t.Fatalf("Zapf arrow Differences decode = %q", got)
	}
}

func TestDifferencesDecodeGlyphNameSuffixes(t *testing.T) {
	f := NewSimpleFont("F")
	f.applyDifferences(Array{Number(65), Name("A.alt"), Name("a1.alt"), Name("uni4E2D.vert")})
	if got := f.Decode([]byte{65, 66, 67}); got != "A✁中" {
		t.Fatalf("glyph-name suffix decode = %q", got)
	}
}

func TestDifferencesDecodeLigatureAliases(t *testing.T) {
	f := NewSimpleFont("F")
	f.applyDifferences(Array{Number(65), Name("f_f"), Name("f_i.alt"), Name("f_f_i"), Name("f_f_l")})
	if got := f.Decode([]byte{65, 66, 67, 68, 69}); got != "ﬀﬁﬃﬄE" {
		t.Fatalf("ligature alias decode = %q", got)
	}
}

func TestDifferencesDecodeCommonSpaceAndHyphenAliases(t *testing.T) {
	f := NewSimpleFont("F")
	f.applyDifferences(Array{Number(65), Name("nbspace"), Name("sfthyphen"), Name("nbhyphen"), Name("hyphenminus"), Name("middot")})
	if got := f.Decode([]byte{65, 66, 67, 68, 69}); got != "\u00a0\u00ad\u2011-·" {
		t.Fatalf("space/hyphen alias decode = %q", got)
	}
}

func TestDifferencesDecodeStandardLetterAndDigitNames(t *testing.T) {
	f := NewSimpleFont("F")
	f.applyDifferences(Array{Number(65), Name("zero"), Name("nine"), Name("A"), Name("z")})
	if got := f.Decode([]byte{65, 66, 67, 68, 69}); got != "09AzE" {
		t.Fatalf("standard letter/digit name decode = %q", got)
	}
}

func TestDifferencesDecodeStandardPunctuationNames(t *testing.T) {
	f := NewSimpleFont("F")
	f.applyDifferences(Array{Number(65), Name("exclamdown"), Name("questiondown"), Name("florin"), Name("ordfeminine"), Name("caron"), Name("ogonek")})
	if got := f.Decode([]byte{65, 66, 67, 68, 69, 70}); got != "¡¿ƒªˇ˛" {
		t.Fatalf("standard punctuation name decode = %q", got)
	}
}

func TestDifferencesDecodeExtendedLatinNames(t *testing.T) {
	f := NewSimpleFont("F")
	f.applyDifferences(Array{Number(65), Name("Scaron"), Name("zcaron"), Name("Yacute"), Name("dotlessj"), Name("Oslash")})
	if got := f.Decode([]byte{65, 66, 67, 68, 69, 70}); got != "ŠžÝ\uf6beØF" {
		t.Fatalf("extended Latin name decode = %q", got)
	}
}

func TestDifferencesDecodeCentralEuropeanNames(t *testing.T) {
	f := NewSimpleFont("F")
	f.applyDifferences(Array{Number(65), Name("Aogonek"), Name("eogonek"), Name("Scedilla"), Name("cacute"), Name("Uring")})
	if got := f.Decode([]byte{65, 66, 67, 68, 69, 70}); got != "ĄęŞćŮF" {
		t.Fatalf("Central European name decode = %q", got)
	}
}

func TestDifferencesDecodeCrossPlatformAliases(t *testing.T) {
	f := NewSimpleFont("F")
	f.applyDifferences(Array{Number(65), Name("applelogo"), Name("Ohm"), Name("mu1"), Name("increment")})
	if got := f.Decode([]byte{65, 66, 67, 68, 69}); got != "\uf8ff\u2126\u00b5∆E" {
		t.Fatalf("authoritative glyph-list alias decode = %q", got)
	}
}

func TestDifferencesDecodeUsesAuthoritativeAdobeDelta(t *testing.T) {
	f := NewSimpleFont("F")
	f.applyDifferences(Array{Number(65), Name("Delta"), Name("Deltagreek")})
	if got := f.Decode([]byte{65, 66}); got != "∆Δ" {
		t.Fatalf("Adobe Delta glyph-list decode = %q", got)
	}
}

func TestDifferencesDecodeGeneratedAdobeGlyphListSequence(t *testing.T) {
	f := NewSimpleFont("F")
	f.applyDifferences(Array{Number(65), Name("dalethatafpatah")})
	if got := f.Decode([]byte{65}); got != "דֲ" {
		t.Fatalf("generated AGL sequence = %q", got)
	}
}

func TestGlyphNameDecodingUsesGeneratedZapfGlyphList(t *testing.T) {
	for name, want := range map[string]string{
		"a1":   "✁",
		"a104": "❤",
		"a202": "✃",
	} {
		if got, ok := glyphText(name); !ok || got != want {
			t.Fatalf("Zapf glyph %q = %q, %v; want %q", name, got, ok, want)
		}
	}
}

func TestDifferencesDecodeCommonGlyphNames(t *testing.T) {
	f := NewSimpleFont("F")
	f.applyDifferences(Array{Number(65), Name("Aacute"), Name("ntilde"), Name("oe"), Name("Euro")})
	if got := f.Decode([]byte{65, 66, 67, 68}); got != "Áñœ€" {
		t.Fatalf("Differences decode = %q", got)
	}
}

func TestDifferencesDecodeExtendedGlyphNames(t *testing.T) {
	f := NewSimpleFont("F")
	f.applyDifferences(Array{Number(65), Name("mu"), Name("Omega"), Name("fi"), Name("uni4E2D")})
	if got := f.Decode([]byte{65, 66, 67, 68}); got != "µΩﬁ中" {
		t.Fatalf("extended Differences decode = %q", got)
	}
}

func TestTransformFontBBoxRejectsOverflowingMatrixProducts(t *testing.T) {
	box, ok := transformFontBBox(geometry.Matrix{math.MaxFloat64, 0, 0, 1, math.MaxFloat64, 0}, [4]float64{0, 0, 1, 1})
	if ok || box != [4]float64{} {
		t.Fatalf("overflowing font bbox = %v, valid=%v", box, ok)
	}
}

func TestDifferencesDecodeMacExpertGlyphNames(t *testing.T) {
	f := NewSimpleFont("F")
	f.applyDifferences(Array{Number(65), Name("onequarter"), Name("threeeighths"), Name("foursuperior"), Name("nineinferior")})
	if got := f.Decode([]byte{65, 66, 67, 68}); got != "¼⅜⁴₉" {
		t.Fatalf("MacExpert Differences decode = %q", got)
	}
}

func TestDifferencesDecodeMacExpertAccentNames(t *testing.T) {
	f := NewSimpleFont("F")
	f.applyDifferences(Array{Number(65), Name("Acutesmall"), Name("Dieresissmall"), Name("Aacutesmall"), Name("Ntildesmall"), Name("Ydieresissmall")})
	if got := f.Decode([]byte{65, 66, 67, 68, 69}); got != "\uf7b4\uf7a8\uf7e1\uf7f1\uf7ff" {
		t.Fatalf("MacExpert accent Differences decode = %q", got)
	}
}

func TestDifferencesDecodeMacExpertSpecialNames(t *testing.T) {
	f := NewSimpleFont("F")
	f.applyDifferences(Array{Number(65), Name("colonmonetary"), Name("rupiah"), Name("parenleftsuperior"), Name("hyphensuperior"), Name("figuredash"), Name("threequartersemdash")})
	if got := f.Decode([]byte{65, 66, 67, 68, 69, 70}); got != "₡\uf6dd⁽\uf6e6‒\uf6de" {
		t.Fatalf("MacExpert special Differences decode = %q", got)
	}
}

func TestDifferencesDecodeAdditionalGlyphNames(t *testing.T) {
	f := NewSimpleFont("F")
	f.applyDifferences(Array{Number(65), Name("quotedblleft"), Name("guilsinglright"), Name("summation"), Name("infinity")})
	if got := f.Decode([]byte{65, 66, 67, 68}); got != "“›∑∞" {
		t.Fatalf("additional Differences decode = %q", got)
	}
}

func TestDifferencesDecodeRemainingGreekGlyphNames(t *testing.T) {
	f := NewSimpleFont("F")
	f.applyDifferences(Array{Number(65), Name("Nu"), Name("xi"), Name("Omicron"), Name("tau"), Name("Zeta")})
	if got := f.Decode([]byte{65, 66, 67, 68, 69}); got != "ΝξΟτΖ" {
		t.Fatalf("remaining Greek Differences decode = %q", got)
	}
}

func TestDifferencesDecodeSymbolAndDingbatGlyphNames(t *testing.T) {
	f := NewSimpleFont("F")
	f.applyDifferences(Array{
		Number(65), Name("theta1"), Name("phi1"), Name("omega1"), Name("universal"),
		Name("suchthat"), Name("arrowboth"), Name("club"), Name("diamond"), Name("spade"),
	})
	if got := f.Decode([]byte{65, 66, 67, 68, 69, 70, 71, 72, 73}); got != "ϑϕϖ∀∋↔♣♦♠" {
		t.Fatalf("Symbol and Dingbats glyph decode = %q", got)
	}
}

func TestDifferencesDecodeAdditionalMathGlyphNames(t *testing.T) {
	f := NewSimpleFont("F")
	f.applyDifferences(Array{
		Number(65), Name("aleph"), Name("Ifraktur"), Name("weierstrass"), Name("emptyset"),
		Name("propersuperset"), Name("propersubset"), Name("element"), Name("notelement"),
		Name("gradient"), Name("dotmath"), Name("logicaland"), Name("logicalor"), Name("angleleft"),
	})
	if got := f.Decode([]byte{65, 66, 67, 68, 69, 70, 71, 72, 73, 74, 75, 76, 77}); got != "ℵℑ℘∅⊃⊂∈∉∇⋅∧∨〈" {
		t.Fatalf("additional math glyph decode = %q", got)
	}
}

func TestApplyDifferencesIgnoresOutOfRangeCodes(t *testing.T) {
	f := NewSimpleFont("F")
	f.applyDifferences(Array{Number(-1), Name("A"), Number(256), Name("B"), Number(65), Name("Chi")})
	if _, ok := f.glyphNames[255]; ok {
		t.Fatal("negative Differences code wrapped to 255")
	}
	if _, ok := f.glyphNames[0]; ok {
		t.Fatal("out-of-range Differences code wrapped to 0")
	}
	if got := f.Decode([]byte{65}); got != "Χ" {
		t.Fatalf("valid Differences code = %q, want Χ", got)
	}
}

func TestDecodeUnknownDifferenceGlyphAsEmpty(t *testing.T) {
	f := NewSimpleFont("F")
	f.applyDifferences(Array{Number(65), Name("unknownGlyph")})
	if got := f.Decode([]byte{65}); got != "" {
		t.Fatalf("unknown Difference glyph decode = %q, want empty", got)
	}
}

func TestSetFontMetricsPreservesDescriptorGeometry(t *testing.T) {
	f := NewSimpleFont("F1")
	setFontMetrics(f, Dict{
		Name("FontBBox"):    Array{Number(700), Number(900), Number(-100), Number(-200)},
		Name("Flags"):       Number(4),
		Name("CapHeight"):   Number(700),
		Name("ItalicAngle"): Number(-12),
		Name("StemV"):       Number(80),
	})
	if !f.hasFontBBox || f.fontBBox != [4]float64{-100, -200, 700, 900} || !f.hasFlags || f.flags != 4 || f.capHeight != 700 || f.italicAngle != -12 || f.stemV != 80 {
		t.Fatalf("font metrics = %#v", f)
	}
}

func TestType3CharProcReadsD1WidthAndBBox(t *testing.T) {
	f := NewSimpleFont("T3")
	f.type3 = true
	f.applyDifferences(Array{Number(65), Name("A")})
	parseType3CharProc(&Document{}, f, "A", newStream(nil, []byte("500 0 -10 -20 510 700 d1")))
	if f.charWidths["A"] != 500 || f.charBBoxes["A"] != [4]float64{-10, -20, 510, 700} {
		t.Fatalf("type3 metrics = %#v", f)
	}
	if bbox, ok := f.GlyphBBox([]byte{65}); !ok || bbox != [4]float64{-10, -20, 510, 700} {
		t.Fatalf("type3 glyph bbox = %v, %v", bbox, ok)
	}
	f.fontMatrix = geometry.Matrix{0, 0.002, -0.001, 0, 0, 0}
	got := f.CharBBox(65)
	want := [4]float64{-0.7, -0.02, 0.02, 1.02}
	for i := range got {
		if math.Abs(got[i]-want[i]) > 1e-12 {
			t.Fatalf("type3 text-space bbox = %v, want %v", got, want)
		}
	}
}

func TestType3CharProcRejectsMalformedMetricsOperands(t *testing.T) {
	f := NewSimpleFont("F")
	parseType3CharProc(&Document{}, f, "D0", newStream(nil, []byte("500 0 1 d0")))
	parseType3CharProc(&Document{}, f, "D1", newStream(nil, []byte("invalid 0 0 0 100 100 d1")))
	parseType3CharProc(&Document{}, f, "D1BBox", newStream(nil, []byte("500 0 0 0 invalid 100 d1")))
	parseType3CharProc(&Document{}, f, "D1Extra", newStream(nil, []byte("500 0 0 0 100 100 1 d1")))
	if _, ok := f.charWidths["D0"]; ok {
		t.Fatal("d0 with extra operands was accepted")
	}
	if _, ok := f.charBBoxes["D1"]; ok {
		t.Fatal("d1 with invalid width was accepted")
	}
	if _, ok := f.charWidths["D1BBox"]; ok {
		t.Fatal("d1 with invalid bbox published a partial width")
	}
	if _, ok := f.charBBoxes["D1BBox"]; ok {
		t.Fatal("d1 with invalid bbox was accepted")
	}
	if _, ok := f.charBBoxes["D1Extra"]; ok {
		t.Fatal("d1 with extra operands was accepted")
	}
}

func TestType3CharProcParsingIsLazy(t *testing.T) {
	f := NewSimpleFont("T3")
	f.type3 = true
	f.document = &Document{}
	f.applyDifferences(Array{Number(65), Name("A")})
	f.charProcs["A"] = newStream(nil, []byte("500 0 -10 -20 510 700 d1"))
	if len(f.charProcOps) != 0 || f.type3Parsed["A"] {
		t.Fatal("Type3 CharProc parsed during setup")
	}
	if got := f.HDisp(65); got != 0.5 {
		t.Fatalf("lazy Type3 width = %v, want 0.5", got)
	}
	got := f.CharBBox(65)
	want := [4]float64{-0.01, -0.02, 0.51, 0.7}
	for i := range got {
		if math.Abs(got[i]-want[i]) > 1e-12 {
			t.Fatalf("lazy Type3 text-space bbox = %v, want %v", got, want)
		}
	}
	if !f.type3Parsed["A"] || len(f.charProcOps["A"]) == 0 {
		t.Fatal("Type3 CharProc was not materialized on access")
	}
}

func TestType3CharProcSnapshotsAreOwnedAndConcurrent(t *testing.T) {
	font := NewSimpleFont("Type3")
	font.type3 = true
	font.charProcs["A"] = newStream(nil, []byte("500 0 0 0 100 100 d1"))

	first, ok := font.CharProc("A")
	if !ok {
		t.Fatal("Type3 CharProc was not found")
	}
	first.DataBorrowed()[0] = 'X'
	second, ok := font.CharProc("A")
	if !ok || string(second.DataBorrowed()) != "500 0 0 0 100 100 d1" {
		t.Fatalf("Type3 CharProc snapshot was not independent: %#v", second)
	}

	const readers = 16
	var wg sync.WaitGroup
	for i := 0; i < readers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, ok := font.CharProc("A"); !ok {
				t.Errorf("concurrent Type3 CharProc lookup failed")
			}
		}()
	}
	wg.Wait()
}

func TestType3CharProcPathSnapshotCopiesResources(t *testing.T) {
	font := NewSimpleFont("Type3")
	font.type3 = true
	font.glyphNames[65] = "A"
	font.charProcs["A"] = newStream(nil, []byte("500 0 0 0 100 100 d1"))
	font.resources = Dict{Name("Properties"): Dict{Name("Marker"): String("original")}}

	_, proc, resources, ok := font.type3CharProcSnapshot([]byte{65})
	if !ok {
		t.Fatal("Type3 path snapshot was not found")
	}
	proc.DataBorrowed()[0] = 'X'
	resources[Name("Properties")].(Dict)[Name("Marker")] = String("changed")
	if string(font.charProcs["A"].DataBorrowed()) != "500 0 0 0 100 100 d1" {
		t.Fatal("Type3 path snapshot aliases CharProc bytes")
	}
	if got := font.resources[Name("Properties")].(Dict)[Name("Marker")]; string(got.(String)) != "original" {
		t.Fatalf("Type3 path snapshot aliases resources: %#v", got)
	}
}

func TestType3GlyphOpsSnapshotIsIndependent(t *testing.T) {
	font := NewSimpleFont("Type3")
	font.type3 = true
	font.glyphNames[65] = "A"
	font.type3Parsed = map[string]bool{"A": true}
	font.charProcOps = map[string][]ContentOp{"A": {newContentOpBorrowed("m", []Object{Number(1)}, 0)}}

	ops, ok := font.type3GlyphOpsSnapshot([]byte{65})
	if !ok || len(ops) != 1 {
		t.Fatalf("Type3 glyph ops snapshot = %#v, ok=%v", ops, ok)
	}
	ops[0].operandsValue()[0] = Number(9)
	if got := font.charProcOps["A"][0].operandsValue()[0]; got != Number(1) {
		t.Fatalf("Type3 glyph ops snapshot aliases operands: %#v", got)
	}
}

func TestType3CharProcCacheHasBoundedBytes(t *testing.T) {
	const attempts = 4096
	font := NewSimpleFont("Type3")
	font.type3 = true
	font.document = &Document{}
	font.charProcs = make(map[string]Stream, attempts)
	for i := 0; i < attempts; i++ {
		name := "G" + strconv.Itoa(i)
		font.charProcs[name] = newStream(nil, []byte("1 2 3 d0"))
		if _, err := font.ensureType3CharProcWithError(name); err == nil {
			t.Fatalf("Type3 CharProc %q unexpectedly parsed", name)
		}
	}
	if font.type3CacheBytes == 0 || font.type3CacheBytes > type3CacheLimit {
		t.Fatalf("Type3 cache bytes = %d, want a positive bounded value", font.type3CacheBytes)
	}
}

func TestFailedLazyFontParsersReleaseInputBuffers(t *testing.T) {
	cmap := NewSimpleFont("CMap")
	cmap.cmapData = []byte("large cmap input")
	cmap.cmapFilters = []string{"UnsupportedFilter"}
	if got := cmap.CMapSnapshot(); got != nil {
		t.Fatalf("failed CMap parse returned %#v", got)
	}
	if !cmap.cmapParsed || cmap.cmapData != nil || cmap.cmapFilters != nil || cmap.cmapFilterParms != nil {
		t.Fatalf("failed CMap parse retained inputs: parsed=%v data=%d filters=%v parms=%v", cmap.cmapParsed, len(cmap.cmapData), cmap.cmapFilters, cmap.cmapFilterParms)
	}

	unicode := NewSimpleFont("ToUnicode")
	unicode.toUnicodeData = []byte("large ToUnicode input")
	unicode.toUnicodeFilters = []string{"UnsupportedFilter"}
	if _, ok := unicode.ToUnicodeValue(0x41); ok {
		t.Fatal("failed ToUnicode parse returned a mapping")
	}
	if !unicode.toUnicodeParsed || unicode.toUnicodeData != nil || unicode.toUnicodeFilters != nil || unicode.toUnicodeParms != nil {
		t.Fatalf("failed ToUnicode parse retained inputs: parsed=%v data=%d filters=%v parms=%v", unicode.toUnicodeParsed, len(unicode.toUnicodeData), unicode.toUnicodeFilters, unicode.toUnicodeParms)
	}

	font := NewSimpleFont("embedded")
	font.cidToGIDData = []byte("cid")
	font.cidToGIDFilters = []string{"UnsupportedFilter"}
	font.trueTypeData = []byte("truetype")
	font.trueTypeFilters = []string{"UnsupportedFilter"}
	font.cffData = []byte("cff")
	font.cffFilters = []string{"UnsupportedFilter"}
	font.ensureCIDToGID()
	font.ensureTrueType()
	font.ensureCFF()
	if font.cidToGIDData != nil || font.cidToGIDFilters != nil || font.trueTypeData != nil || font.trueTypeFilters != nil || font.cffData != nil || font.cffFilters != nil {
		t.Fatalf("failed embedded font parsers retained input buffers")
	}
}

func TestEmbeddedCIDCMapParsingIsLazy(t *testing.T) {
	f := NewSimpleFont("CID")
	f.cid = true
	f.document = &Document{}
	f.cmapData = []byte("1 begincodespacerange\n<00> <ff>\nendcodespacerange\n1 begincidchar\n<41> 100\nendcidchar")
	if f.cmap != nil || f.cmapParsed {
		t.Fatal("embedded CMap parsed during setup")
	}
	glyphs := f.DecodeGlyphs([]byte{0x41})
	if len(glyphs) != 1 || glyphs[0].CID() != 100 || !f.cmapParsed {
		t.Fatalf("lazy embedded CMap decode = %#v", glyphs)
	}
}

func TestToUnicodeParsingIsLazy(t *testing.T) {
	f := NewSimpleFont("CID")
	f.cid = true
	f.toUnicodeData = []byte("<0041> <4e2d>")
	if f.toUnicodeParsed || len(f.toUnicode) != 0 {
		t.Fatal("ToUnicode parsed during setup")
	}
	if got, ok := f.ToUnicodeValue(0x41); !ok || got != "中" || !f.toUnicodeParsed {
		t.Fatalf("lazy ToUnicode = %q, %v", got, ok)
	}
}

func TestDecodeCIDUsesTwoByteToUnicode(t *testing.T) {
	f := NewSimpleFont("F")
	f.cid = true
	f.toUnicode = map[uint16]string{}
	f.toUnicode[0x0a27] = "—"
	if got := f.Decode([]byte{0x0a, 0x27}); got != "—" {
		t.Fatalf("CID decode = %q", got)
	}
}

func TestDecodeCIDUsesEmbeddedCMapCodespaces(t *testing.T) {
	f := NewSimpleFont("CID")
	f.cid = true
	f.cmap, _ = ParseCMap([]byte("1 begincodespacerange\n<00> <FF>\nendcodespacerange\n1 begincidchar\n<41> 100\nendcidchar"))
	f.cidWidths[100] = 600
	got := f.DecodeGlyphs([]byte{0x41})
	if len(got) != 1 || got[0].CID() != 100 || got[0].Text() != "A" || f.WidthCode(got[0].BytesCopy()) != 600 {
		t.Fatalf("decoded glyph: %#v width=%v", got, f.WidthCode(got[0].BytesCopy()))
	}
}

func TestEmbeddedEncodingCMapUsesPlayaAscendingCodeSpaceOrder(t *testing.T) {
	f := NewSimpleFont("CID")
	f.cmapData = []byte(`2 begincodespacerange
<00> <ff>
<0000> <ffff>
endcodespacerange
2 begincidchar
<01> 11
<0102> 22
endcidchar`)
	f.ensureCMap()
	if f.cmap == nil {
		t.Fatal("embedded CMap was not parsed")
	}
	decoded := f.cmap.Decode([]byte{0x01, 0x02})
	if len(decoded) != 2 || decoded[0].CID() != 11 || decoded[1].CID() != 0 {
		t.Fatalf("embedded Encoding CMap decode = %#v, want one-byte Playa order", decoded)
	}
}

func TestParseToUnicodeCodesBFRangesArray(t *testing.T) {
	m, err := ParseToUnicodeCodes([]byte("1 beginbfrange\n<00B2> <00B2> [<2014>]\nendbfrange"))
	if err != nil || m[string([]byte{0, 0xb2})] != "—" {
		t.Fatalf("bfrange array = %#v, err=%v", m, err)
	}
}

func TestParseToUnicodeAcceptsMultipleBFCharPairsOnOneLine(t *testing.T) {
	data := []byte("2 beginbfchar\n<0001> <0041> <0002> <0042>\nendbfchar")
	m, err := ParseToUnicode(data)
	if err != nil || m[1] != "A" || m[2] != "B" || len(m) != 2 {
		t.Fatalf("bfchar mappings = %#v, err=%v", m, err)
	}
	codes, err := ParseToUnicodeCodes(data)
	if err != nil || codes[string([]byte{0, 1})] != "A" || codes[string([]byte{0, 2})] != "B" || len(codes) != 2 {
		t.Fatalf("bfchar code mappings = %#v, err=%v", codes, err)
	}
}

func TestParseToUnicodeHandlesLargePhysicalLines(t *testing.T) {
	data := []byte("1 beginbfchar\n" + strings.Repeat(" ", 1<<20) + "<0001> <0041>\nendbfchar")
	mapping, err := ParseToUnicode(data)
	if err != nil || mapping[1] != "A" {
		t.Fatalf("large-line ToUnicode mapping = %#v, err=%v", mapping, err)
	}
	codes, err := ParseToUnicodeCodes(data)
	if err != nil || codes[string([]byte{0, 1})] != "A" {
		t.Fatalf("large-line ToUnicode codes = %#v, err=%v", codes, err)
	}
}

func TestParseToUnicodeRejectsOversizedPhysicalLines(t *testing.T) {
	data := []byte("1 beginbfchar\n" + strings.Repeat(" ", maxToUnicodePhysicalLine+1) + "<0001> <0041>\nendbfchar")
	if _, err := ParseToUnicode(data); err == nil {
		t.Fatal("ToUnicode accepted an oversized physical line")
	}
	if _, err := ParseToUnicodeCodes(data); err == nil {
		t.Fatal("ToUnicode code parser accepted an oversized physical line")
	}
}

func TestParseToUnicodeAcceptsRecordsSplitAcrossLines(t *testing.T) {
	data := []byte(`3 beginbfchar
<0001>
<0041>
endbfchar
1 beginbfrange
<0002> <0003>
<0042>
endbfrange
1 begincidrange
<0004> <0005>
70
endcidrange`)
	m, err := ParseToUnicode(data)
	if err != nil || m[1] != "A" || m[2] != "B" || m[3] != "C" || m[4] != "F" || m[5] != "G" {
		t.Fatalf("split ToUnicode mappings = %#v, err=%v", m, err)
	}
	codes, err := ParseToUnicodeCodes(data)
	if err != nil || codes[string([]byte{0, 1})] != "A" || codes[string([]byte{0, 2})] != "B" || codes[string([]byte{0, 3})] != "C" || codes[string([]byte{0, 4})] != "F" || codes[string([]byte{0, 5})] != "G" {
		t.Fatalf("split ToUnicode code mappings = %#v, err=%v", codes, err)
	}
}

func TestParseToUnicodeAcceptsInlineSectionsAndComments(t *testing.T) {
	data := []byte("1 beginbfchar <0001> <0041> endbfchar % inline mapping\n")
	m, err := ParseToUnicode(data)
	if err != nil || m[1] != "A" || len(m) != 1 {
		t.Fatalf("inline bfchar mappings = %#v, err=%v", m, err)
	}
	codes, err := ParseToUnicodeCodes(data)
	if err != nil || codes[string([]byte{0, 1})] != "A" || len(codes) != 1 {
		t.Fatalf("inline bfchar code mappings = %#v, err=%v", codes, err)
	}
}

func TestParseToUnicodeAcceptsMultilineBFRangesArray(t *testing.T) {
	data := []byte("1 beginbfrange\n<0001> <0002> [\n<0041>\n<0042>\n]\nendbfrange")
	m, err := ParseToUnicode(data)
	if err != nil || m[1] != "A" || m[2] != "B" || len(m) != 2 {
		t.Fatalf("multiline bfrange mappings = %#v, err=%v", m, err)
	}
	codes, err := ParseToUnicodeCodes(data)
	if err != nil || codes[string([]byte{0, 1})] != "A" || codes[string([]byte{0, 2})] != "B" || len(codes) != 2 {
		t.Fatalf("multiline bfrange code mappings = %#v, err=%v", codes, err)
	}
}

func TestParseToUnicodeAcceptsAdjacentHexBFRanges(t *testing.T) {
	data := []byte("1 beginbfrange\n<0044><0046><0061>\nendbfrange")
	m, err := ParseToUnicode(data)
	if err != nil || m[0x44] != "a" || m[0x45] != "b" || m[0x46] != "c" || len(m) != 3 {
		t.Fatalf("adjacent hex bfrange mappings = %#v, err=%v", m, err)
	}
	codes, err := ParseToUnicodeCodes(data)
	if err != nil || codes[string([]byte{0, 0x44})] != "a" || codes[string([]byte{0, 0x45})] != "b" || codes[string([]byte{0, 0x46})] != "c" || len(codes) != 3 {
		t.Fatalf("adjacent hex bfrange code mappings = %#v, err=%v", codes, err)
	}
}

func TestParseToUnicodeCodesHandlesFourByteRangeAcrossUint16Boundary(t *testing.T) {
	data := []byte("1 beginbfrange\n<00fffffe> <01000001> <0041>\nendbfrange")
	codes, err := ParseToUnicodeCodes(data)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		string([]byte{0x00, 0xff, 0xff, 0xfe}): "A",
		string([]byte{0x00, 0xff, 0xff, 0xff}): "B",
		string([]byte{0x01, 0x00, 0x00, 0x00}): "C",
		string([]byte{0x01, 0x00, 0x00, 0x01}): "D",
	}
	for code, text := range want {
		if codes[code] != text {
			t.Fatalf("code %x = %q, want %q; all mappings = %#v", []byte(code), codes[code], text, codes)
		}
	}
}

func TestParseToUnicodeDoesNotTruncateWideSourceCodes(t *testing.T) {
	m, err := ParseToUnicode([]byte("1 beginbfchar\n<00010001> <0041>\nendbfchar"))
	if err != nil {
		t.Fatal(err)
	}
	if len(m) != 0 {
		t.Fatalf("wide source code was truncated: %#v", m)
	}
}
