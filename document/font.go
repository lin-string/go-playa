package document

//go:generate python3 ../scripts/sync_glyphlist_sources.py --output-dir ../scripts/data
//go:generate python3 ../scripts/generate_glyphlist.py --input ../scripts/data/glyphlist.txt --zapf-input ../scripts/data/zapfdingbats.txt --package fontdata --output ../fontdata/glyphlist_generated.go
//go:generate python3 ../scripts/sync_core14_afm.py --output-dir ../scripts/data/core14
//go:generate python3 ../scripts/generate_core14_metrics.py --input-dir ../scripts/data/core14 --glyphlist ../scripts/data/glyphlist.txt --zapf-glyphlist ../scripts/data/zapfdingbats.txt --package fontdata --output ../fontdata/standard_metrics_generated.go
//go:generate python3 ../scripts/generate_symbol_encoding.py --package fontdata --output ../fontdata/symbol_encoding_generated.go
//go:generate python3 ../scripts/generate_zapfdingbats_encoding.py --package fontdata --output ../fontdata/zapfdingbats_encoding_generated.go
//go:generate python3 ../scripts/generate_macroman_encoding.py --package fontdata --output ../fontdata/macroman_encoding_generated.go
//go:generate python3 ../scripts/generate_winansi_encoding.py --package fontdata --output ../fontdata/winansi_encoding_generated.go
//go:generate python3 ../scripts/generate_macexpert_encoding.py
//go:generate gofmt -w ../fontdata/glyphlist_generated.go
//go:generate gofmt -w ../fontdata/standard_metrics_generated.go
//go:generate gofmt -w ../fontdata/symbol_encoding_generated.go
//go:generate gofmt -w ../fontdata/zapfdingbats_encoding_generated.go
//go:generate gofmt -w ../fontdata/macroman_encoding_generated.go
//go:generate gofmt -w ../fontdata/winansi_encoding_generated.go

import (
	"errors"
	"fmt"
	"iter"
	"math"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"unicode"
	"unicode/utf8"
	"unsafe"

	"github.com/lin-string/go-playa/fontdata"
	"github.com/lin-string/go-playa/geometry"
	"golang.org/x/image/font/sfnt"
	"golang.org/x/image/math/fixed"
)

const type3CacheLimit = 64 << 10

const (
	toUnicodeTextCacheLimit      = 4096
	toUnicodeTextCacheEntryLimit = 64
	maxToUnicodePhysicalLine     = 4 << 20
)

var toUnicodeTextCache = struct {
	sync.RWMutex
	values map[string]string
}{values: make(map[string]string, toUnicodeTextCacheLimit)}

var errEmbeddedFontUnavailable = errors.New("playa: embedded font program is unavailable")

type Font struct {
	lazyMu              *sync.Mutex
	widthState          unsafe.Pointer
	name                string
	baseName            string
	cidCoding           string
	encoding            map[byte]rune
	simpleEncodingName  string
	glyphTexts          map[byte]string
	glyphNames          map[byte]string
	widths              map[byte]float64
	hasPDFWidths        bool
	standardWidths      map[rune]float64
	cidWidths           map[int]float64
	verticalWidths      map[int]float64
	verticalPositions   map[int][2]float64
	defaultWidth        float64
	defaultVWidth       float64
	defaultVPosition    [2]float64
	toUnicode           map[uint16]string
	toUnicodeSpaces     []fontdata.CodeSpace
	codeMap             map[string]string
	toUnicodeData       []byte
	toUnicodeFilters    []string
	toUnicodeParms      []Dict
	toUnicodeParsed     bool
	toUnicodeErr        error
	glyphIDToUnicode    map[int]string
	glyphUnicodeToID    map[rune]int
	cidToGID            map[int]int
	trueTypeData        []byte
	trueTypeRef         Ref
	trueTypeFilters     []string
	trueTypeParms       []Dict
	trueTypeParsed      bool
	trueTypeErr         error
	trueTypeFont        *sfnt.Font
	trueTypeProgram     *trueTypeProgramState
	trueTypePathCache   map[int][]ContentOp
	trueTypePathBytes   int
	cidToGIDData        []byte
	cidToGIDFilters     []string
	cidToGIDParms       []Dict
	cidToGIDParsed      bool
	cidToGIDErr         error
	glyphIDWidths       map[int]float64
	cidToUnicode        map[int]string
	cmap                *fontdata.CMap
	cmapData            []byte
	cmapFilters         []string
	cmapFilterParms     []Dict
	cmapUse             Object
	cmapParsed          bool
	cmapErr             error
	subtype             string
	type3               bool
	fontMatrix          geometry.Matrix
	vertical            bool
	cid                 bool
	ascent              float64
	descent             float64
	leading             float64
	fontBBox            [4]float64
	hasFontBBox         bool
	flags               int
	hasFlags            bool
	capHeight           float64
	italicAngle         float64
	stemV               float64
	charProcs           map[string]Stream
	charProcOps         map[string][]ContentOp
	type3Parsed         map[string]bool
	type3Err            map[string]error
	type3CacheBytes     int
	type3OverflowErr    error
	resources           Dict
	charWidths          map[string]float64
	charBBoxes          map[string][4]float64
	cffCharstrings      [][]byte
	cffData             []byte
	cffFilters          []string
	cffParms            []Dict
	cffParsed           bool
	cffErr              error
	cffLocalSubrs       [][]byte
	cffGlobalSubrs      [][]byte
	cffGlyphIDs         map[byte]int
	cffFDByGlyph        map[int]int
	cffFDLocalSubrs     map[int][][]byte
	cffFDVariationIndex map[int]int
	cffPathCache        map[string][]ContentOp
	cffPathCacheBytes   int
	cff2                bool
	cffVariationStore   *cffVariationStore
	cffVariationIndex   int
	cffDefaultWidthVar  *cffBlendValue
	cffNominalWidthVar  *cffBlendValue
	variationCoords     []float64
	cffImplicitEncoding bool
	cffDifferences      map[byte]string
	type1Data           []byte
	type1Filters        []string
	type1Parms          []Dict
	type1Length1        int
	type1Parsed         bool
	type1Differences    map[byte]string
	type1Err            error
	type1Charstrings    map[string][]byte
	type1Subrs          [][]byte
	type1PathCache      map[string][]ContentOp
	type1PathCacheBytes int
	fontType            string
	strictEncoding      bool
	document            *Document
}

// trueTypeProgramState is immutable after construction. Multiple PDF font
// dictionaries may select different encodings and widths while referencing
// the same embedded program, so the expensive program parse can be shared.
type trueTypeProgramState struct {
	font             *sfnt.Font
	glyphIDToUnicode map[int]string
	glyphUnicodeToID map[rune]int
	glyphIDWidths    map[int]float64
	err              error
}

// fontWidthState is immutable after publication. It contains the fields used
// by non-Type3 WidthCode lookups so repeated glyph extraction can avoid the
// font's lazy-parse mutex entirely.
type fontWidthState struct {
	name             string
	fontType         string
	cid              bool
	cmap             *fontdata.CMap
	cidToGID         map[int]int
	cidWidths        map[int]float64
	cidToUnicode     map[int]string
	standardWidths   map[rune]float64
	codeMap          map[string]string
	toUnicode        map[uint16]string
	glyphTexts       map[byte]string
	encoding         map[byte]rune
	widths           map[byte]float64
	cffGlyphIDs      map[byte]int
	glyphIDToUnicode map[int]string
	glyphUnicodeToID map[rune]int
	glyphIDWidths    map[int]float64
	defaultWidth     float64
	fontMatrixX      float64
}

type fontDecodeSnapshot struct {
	cid              bool
	type3            bool
	cmap             *fontdata.CMap
	hasToUnicode     bool
	codeMap          map[string]string
	toUnicode        map[uint16]string
	toUnicodeSpaces  []fontdata.CodeSpace
	glyphIDToUnicode map[int]string
	cidToUnicode     map[int]string
	glyphTexts       map[byte]string
	encoding         map[byte]rune
	glyphNames       map[byte]string
	strictEncoding   bool
}

func (f *Font) decodeSnapshot() fontDecodeSnapshot {
	f.lazyMutex().Lock()
	defer f.lazyMutex().Unlock()
	return fontDecodeSnapshot{
		cid:              f.cid,
		type3:            f.type3,
		cmap:             f.cmap,
		hasToUnicode:     f.toUnicodeParsed,
		codeMap:          f.codeMap,
		toUnicode:        f.toUnicode,
		toUnicodeSpaces:  f.toUnicodeSpaces,
		glyphIDToUnicode: f.glyphIDToUnicode,
		cidToUnicode:     f.cidToUnicode,
		glyphTexts:       f.glyphTexts,
		encoding:         f.encoding,
		glyphNames:       f.glyphNames,
		strictEncoding:   f.strictEncoding,
	}
}

func (f *Font) widthStateLoad() *fontWidthState {
	if f == nil {
		return nil
	}
	return (*fontWidthState)(atomic.LoadPointer(&f.widthState))
}

func (f *Font) publishWidthStateLocked() *fontWidthState {
	if f.type3 {
		return nil
	}
	state := &fontWidthState{
		name:             f.name,
		fontType:         f.fontType,
		cid:              f.cid,
		cmap:             f.cmap,
		cidToGID:         f.cidToGID,
		cidWidths:        f.cidWidths,
		cidToUnicode:     f.cidToUnicode,
		standardWidths:   f.standardWidths,
		codeMap:          f.codeMap,
		toUnicode:        f.toUnicode,
		glyphTexts:       f.glyphTexts,
		encoding:         f.encoding,
		widths:           f.widths,
		cffGlyphIDs:      f.cffGlyphIDs,
		glyphIDToUnicode: f.glyphIDToUnicode,
		glyphUnicodeToID: f.glyphUnicodeToID,
		glyphIDWidths:    f.glyphIDWidths,
		defaultWidth:     f.defaultWidth,
		fontMatrixX:      f.fontMatrix[0],
	}
	atomic.StorePointer(&f.widthState, unsafe.Pointer(state))
	return state
}

// interpreterWidthState returns the immutable font tables used repeatedly by
// glyph extraction. The caller has already completed lazy font parsing through
// DecodeGlyphsSeqWithError, so publishing the snapshot requires only one lock
// per text string instead of several locks per glyph.
func (f *Font) interpreterWidthState() *fontWidthState {
	if f == nil || f.type3 {
		return nil
	}
	if state := f.widthStateLoad(); state != nil {
		return state
	}
	f.lazyMutex().Lock()
	defer f.lazyMutex().Unlock()
	if state := f.widthStateLoad(); state != nil {
		return state
	}
	return f.publishWidthStateLocked()
}

// encodedStandardWidth uses the font Encoding (including Differences), never
// ToUnicode, which can map a space glyph to unrelated extracted text.
func encodedStandardWidth(code byte, encoding map[byte]rune, glyphTexts map[byte]string, widths map[rune]float64) (float64, bool) {
	r, ok := encoding[code]
	if text, present := glyphTexts[code]; present {
		var size int
		r, size = utf8.DecodeRuneInString(text)
		ok = size > 0 && size == len(text)
	}
	if !ok {
		return 0, false
	}
	width, found := widths[r]
	return width, found
}

func (state *fontWidthState) hDisp(cid int) float64 {
	width := state.defaultWidth
	if state.cid {
		if value, ok := state.cidWidths[cid]; ok {
			width = value
		}
	} else if value, ok := state.widths[byte(cid)]; ok {
		width = value
	} else if state.standardWidths != nil && state.fontType != "TrueType" && !isSubsetFont(state.name) {
		if value, ok := encodedStandardWidth(byte(cid), state.encoding, state.glyphTexts, state.standardWidths); ok {
			width = value
		}
	}
	return state.fontMatrixX * width
}

func (state *fontWidthState) glyphID(code []byte, cid int) int {
	if len(code) == 0 {
		return 0
	}
	if state.cid {
		if state.cmap != nil {
			if mapped, ok := fontdata.MappingValue(state.cmap, code); ok {
				cid = mapped
			}
		}
		if glyphID, ok := state.cidToGID[cid]; ok {
			return glyphID
		}
		return cid
	}
	codeByte := code[len(code)-1]
	if glyphID, ok := state.cffGlyphIDs[codeByte]; ok {
		return glyphID
	}
	if r, ok := state.encoding[codeByte]; ok {
		if glyphID, found := state.glyphUnicodeToID[r]; found {
			return glyphID
		}
		if glyphID, found := state.glyphIDForUnicode(r); found {
			return glyphID
		}
	}
	return 0
}

func (state *fontWidthState) hasUnicodeMapping(code []byte, cid int) bool {
	if text, ok := state.codeMap[string(code)]; ok && text != "" {
		return true
	}
	if text, ok := state.toUnicode[uint16(codeNumber(code))]; ok && text != "" {
		return true
	}
	glyphID := cid
	if mapped, ok := state.cidToGID[cid]; ok {
		glyphID = mapped
	}
	if text, ok := state.glyphIDToUnicode[glyphID]; ok && text != "" && !isPrivateUseText(text) {
		return true
	}
	text, ok := state.cidToUnicode[cid]
	return ok && text != ""
}

func (state *fontWidthState) widthCode(code []byte) float64 {
	if state.cid {
		cid := codeNumber(code)
		if state.cmap != nil {
			if mapped, ok := fontdata.MappingValue(state.cmap, code); ok {
				cid = mapped
			}
		}
		if width, ok := state.cidWidths[cid]; ok {
			return width
		}
		if width, ok := state.standardWidthForCode(code); ok {
			return width
		}
	}
	if len(code) == 0 {
		return state.defaultWidth
	}
	codeByte := code[len(code)-1]
	if state.fontType == "TrueType" {
		if width, ok := state.widths[codeByte]; ok {
			return width
		}
	}
	if state.standardWidths != nil && state.fontType != "TrueType" && !isSubsetFont(state.name) {
		if runeValue, ok := state.encoding[codeByte]; ok {
			if width, found := state.standardWidths[runeValue]; found {
				return width
			}
		}
	}
	if state.fontType == "TrueType" {
		if runeValue, ok := state.encoding[codeByte]; ok {
			if glyphID, found := state.glyphUnicodeToID[runeValue]; found {
				if width, found := state.glyphIDWidths[glyphID]; found {
					return width
				}
			} else if glyphID, found := state.glyphIDForUnicode(runeValue); found {
				if width, found := state.glyphIDWidths[glyphID]; found {
					return width
				}
			}
		}
	}
	if width, ok := state.widths[codeByte]; ok {
		return width
	}
	return state.defaultWidth
}

func (state *fontWidthState) glyphIDForUnicode(r rune) (int, bool) {
	for glyphID, text := range state.glyphIDToUnicode {
		if utf8.RuneCountInString(text) == 1 {
			mapped, _ := utf8.DecodeRuneInString(text)
			if mapped == r {
				return glyphID, true
			}
		}
	}
	return 0, false
}

func (state *fontWidthState) standardWidthForCode(code []byte) (float64, bool) {
	if len(code) == 0 || state.standardWidths == nil {
		return 0, false
	}
	if text, ok := state.codeMap[string(code)]; ok {
		if runes := []rune(text); len(runes) == 1 {
			width, found := state.standardWidths[runes[0]]
			return width, found
		}
	}
	if mapped, found := state.toUnicode[uint16(codeNumber(code))]; found {
		if runes := []rune(mapped); len(runes) == 1 {
			width, ok := state.standardWidths[runes[0]]
			return width, ok
		}
	}
	return 0, false
}

var fontMutexInitMu sync.Mutex

func (font *Font) lazyMutex() *sync.Mutex {
	if pointer := atomic.LoadPointer((*unsafe.Pointer)(unsafe.Pointer(&font.lazyMu))); pointer != nil {
		return (*sync.Mutex)(pointer)
	}
	fontMutexInitMu.Lock()
	defer fontMutexInitMu.Unlock()
	if pointer := atomic.LoadPointer((*unsafe.Pointer)(unsafe.Pointer(&font.lazyMu))); pointer != nil {
		return (*sync.Mutex)(pointer)
	}
	mutex := &sync.Mutex{}
	atomic.StorePointer((*unsafe.Pointer)(unsafe.Pointer(&font.lazyMu)), unsafe.Pointer(mutex))
	return mutex
}

func cloneFont(font *Font) *Font {
	if font == nil {
		return nil
	}
	font.lazyMutex().Lock()
	defer font.lazyMutex().Unlock()
	if font.type3 {
		for name := range font.charProcs {
			_ = font.ensureType3CharProcLocked(name)
		}
	}
	font.ensureCMapLocked()
	font.ensureToUnicodeLocked()
	font.ensureCIDToGIDLocked()
	font.ensureTrueTypeLocked()
	font.ensureCFFLocked()
	font.ensureType1EncodingLocked()
	clone := *font
	clone.lazyMu = &sync.Mutex{}
	clone.widthState = nil
	clone.encoding = cloneByteRuneMap(font.encoding)
	clone.glyphTexts = cloneByteStringMap(font.glyphTexts)
	clone.glyphNames = cloneByteStringMap(font.glyphNames)
	clone.widths = cloneByteFloatMap(font.widths)
	clone.standardWidths = cloneRuneFloatMap(font.standardWidths)
	clone.cidWidths = cloneIntFloatMap(font.cidWidths)
	clone.verticalWidths = cloneIntFloatMap(font.verticalWidths)
	clone.verticalPositions = cloneIntPositionMap(font.verticalPositions)
	clone.toUnicode = cloneUint16StringMap(font.toUnicode)
	clone.toUnicodeSpaces = cloneCodeSpaces(font.toUnicodeSpaces)
	clone.codeMap = cloneStringMap(font.codeMap)
	clone.toUnicodeData = cloneObjectBytes(font.toUnicodeData)
	clone.toUnicodeFilters = cloneFontStrings(font.toUnicodeFilters)
	clone.toUnicodeParms = cloneFilterParms(font.toUnicodeParms)
	clone.toUnicodeErr = font.toUnicodeErr
	clone.glyphIDToUnicode = cloneIntStringMap(font.glyphIDToUnicode)
	clone.glyphUnicodeToID = cloneRuneIntMap(font.glyphUnicodeToID)
	clone.cidToGID = cloneIntIntMap(font.cidToGID)
	clone.glyphIDWidths = cloneIntFloatMap(font.glyphIDWidths)
	clone.cidToUnicode = cloneIntStringMap(font.cidToUnicode)
	clone.cmap = cloneCMap(font.cmap)
	clone.charProcs = cloneStreamMap(font.charProcs)
	clone.resources = cloneDict(font.resources)
	clone.charWidths = cloneStringFloatMap(font.charWidths)
	clone.charBBoxes = cloneStringBBoxMap(font.charBBoxes)
	clone.cffCharstrings = cloneByteSlices(font.cffCharstrings)
	clone.cffLocalSubrs = cloneByteSlices(font.cffLocalSubrs)
	clone.cffGlobalSubrs = cloneByteSlices(font.cffGlobalSubrs)
	clone.cffGlyphIDs = cloneByteIntMap(font.cffGlyphIDs)
	clone.cffDifferences = cloneByteStringMap(font.cffDifferences)
	clone.type1Differences = cloneByteStringMap(font.type1Differences)
	clone.cffFDByGlyph = cloneIntIntMap(font.cffFDByGlyph)
	clone.cffFDLocalSubrs = cloneIntByteSlicesMap(font.cffFDLocalSubrs)
	clone.cffFDVariationIndex = cloneIntIntMap(font.cffFDVariationIndex)
	clone.cffPathCache = cloneContentOpMap(font.cffPathCache)
	clone.cffPathCacheBytes = font.cffPathCacheBytes
	clone.cffVariationIndex = font.cffVariationIndex
	clone.cffDefaultWidthVar = cloneCFFBlendValue(font.cffDefaultWidthVar)
	clone.cffNominalWidthVar = cloneCFFBlendValue(font.cffNominalWidthVar)
	clone.variationCoords = cloneFontFloats(font.variationCoords)
	clone.cffVariationStore = cloneCFFVariationStore(font.cffVariationStore)
	clone.charProcOps = cloneContentOpMap(font.charProcOps)
	clone.type3Parsed = cloneStringBoolMap(font.type3Parsed)
	clone.type3Err = cloneStringErrorMap(font.type3Err)
	clone.type3CacheBytes = font.type3CacheBytes
	clone.type3OverflowErr = font.type3OverflowErr
	clone.cmapData = cloneObjectBytes(font.cmapData)
	clone.cmapFilters = cloneFontStrings(font.cmapFilters)
	clone.cmapFilterParms = cloneFilterParms(font.cmapFilterParms)
	clone.cmapUse = cloneGraphicsObject(font.cmapUse)
	clone.cmapErr = font.cmapErr
	clone.cidToGIDData = cloneObjectBytes(font.cidToGIDData)
	clone.cidToGIDFilters = cloneFontStrings(font.cidToGIDFilters)
	clone.cidToGIDParms = cloneFilterParms(font.cidToGIDParms)
	clone.cidToGIDErr = font.cidToGIDErr
	clone.trueTypeData = cloneObjectBytes(font.trueTypeData)
	clone.trueTypeProgram = nil
	clone.trueTypeFilters = cloneFontStrings(font.trueTypeFilters)
	clone.trueTypeParms = cloneFilterParms(font.trueTypeParms)
	clone.trueTypeErr = font.trueTypeErr
	clone.trueTypePathCache = cloneContentOpIntMap(font.trueTypePathCache)
	clone.trueTypePathBytes = font.trueTypePathBytes
	clone.cffData = cloneObjectBytes(font.cffData)
	clone.cffFilters = cloneFontStrings(font.cffFilters)
	clone.cffParms = cloneFilterParms(font.cffParms)
	clone.cffErr = font.cffErr
	clone.type1Data = cloneObjectBytes(font.type1Data)
	clone.type1Filters = cloneFontStrings(font.type1Filters)
	clone.type1Parms = cloneFilterParms(font.type1Parms)
	clone.type1Err = font.type1Err
	clone.type1Charstrings = cloneStringByteMap(font.type1Charstrings)
	clone.type1Subrs = cloneByteSlices(font.type1Subrs)
	clone.type1PathCache = cloneContentOpMap(font.type1PathCache)
	clone.type1PathCacheBytes = font.type1PathCacheBytes
	return &clone
}

func cloneFontFloats(value []float64) []float64 {
	if value == nil {
		return nil
	}
	return append(make([]float64, 0, len(value)), value...)
}

func cloneFontStrings(value []string) []string {
	if value == nil {
		return nil
	}
	return append(make([]string, 0, len(value)), value...)
}

// Finalize returns an independent snapshot of this font.
func (font *Font) Finalize() *Font {
	clone := cloneFont(font)
	if clone != nil {
		clone.document = nil
	}
	return clone
}

// FinalizeWithError materializes an independent font snapshot and reports any
// deferred resource or Type3 CharProc parsing error encountered while doing
// so. Finalize remains the compatibility accessor that suppresses errors.
func (font *Font) FinalizeWithError() (*Font, error) {
	clone := cloneFont(font)
	if clone == nil {
		return nil, nil
	}
	if err := clone.fontLazyError(); err != nil {
		return nil, err
	}
	clone.document = nil
	return clone, nil
}

// ValidateWithError resolves the font's deferred resources in place and
// reports the first terminal error without allocating an independent font
// snapshot. It is intended for borrowed content projections that need error
// propagation but must not pay Finalize's deep-copy cost.
func (font *Font) ValidateWithError() error {
	font.lazyMutex().Lock()
	defer font.lazyMutex().Unlock()
	if font.type3 {
		for name := range font.charProcs {
			_ = font.ensureType3CharProcLocked(name)
		}
	}
	font.ensureCMapLocked()
	font.ensureToUnicodeLocked()
	font.ensureCIDToGIDLocked()
	font.ensureTrueTypeLocked()
	font.ensureCFFLocked()
	font.ensureType1EncodingLocked()
	return font.fontLazyError()
}

// EmbeddedFontFile returns a copy of the still-lazy embedded font program and
// its Playa-compatible filename extension. The result is unavailable after
// the corresponding lazy parser has consumed and released its input buffer.
// Call EmbeddedFontFileWithError when the distinction between an absent
// program and a malformed filtered stream matters.
func (font *Font) EmbeddedFontFile() ([]byte, string, bool) {
	data, extension, ok, _ := font.EmbeddedFontFileWithError()
	return data, extension, ok
}

// EmbeddedFontFileWithError snapshots the embedded font program without
// retaining it in the Font. The returned extension is one of ".pfa", ".ttf",
// or ".cff", matching Playa's write_fontfile selection order.
func (font *Font) EmbeddedFontFileWithError() ([]byte, string, bool, error) {
	font.lazyMutex().Lock()
	defer font.lazyMutex().Unlock()

	var data []byte
	var filters []string
	var parms []Dict
	var extension string
	switch {
	case font.type1Data != nil:
		data, filters, parms, extension = font.type1Data, font.type1Filters, font.type1Parms, ".pfa"
	case font.trueTypeData != nil:
		data, filters, parms, extension = font.trueTypeData, font.trueTypeFilters, font.trueTypeParms, ".ttf"
	case font.cffData != nil:
		data, filters, parms, extension = font.cffData, font.cffFilters, font.cffParms, ".cff"
	default:
		return nil, "", false, nil
	}
	if len(filters) > 0 {
		decoded, err := decodeFiltersLimited(data, filters, parms, decodedFilterExpansionLimit)
		if err != nil {
			return nil, extension, true, fmt.Errorf("%w: %w", errEmbeddedFontUnavailable, err)
		}
		data = decoded
	}
	return cloneObjectBytes(data), extension, true, nil
}

// WriteFontFile writes the still-lazy embedded font program to dir and
// returns its path. The filename follows Playa's write_fontfile convention:
// non-word characters other than '+' are removed from the resolved font name,
// and the extension identifies the embedded program kind. An absent program
// returns an empty path and nil error.
func (font *Font) WriteFontFile(dir string) (string, error) {
	data, extension, ok, err := font.EmbeddedFontFileWithError()
	if err != nil || !ok {
		return "", err
	}
	name := sanitizeFontFileName(font.Name())
	path := filepath.Join(dir, name+extension)
	if err := os.WriteFile(path, data, 0o666); err != nil {
		return "", fmt.Errorf("playa: write embedded font file: %w", err)
	}
	return path, nil
}

func sanitizeFontFileName(name string) string {
	var out strings.Builder
	for _, r := range name {
		if r == '+' || r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r) {
			out.WriteRune(r)
		}
	}
	return out.String()
}

func (font *Font) fontLazyError() error {
	for _, err := range []error{font.cmapErr, font.toUnicodeErr, font.cidToGIDErr, font.trueTypeErr, font.cffErr, font.type1Err} {
		if err != nil {
			return err
		}
	}
	if font.type3OverflowErr != nil {
		return font.type3OverflowErr
	}
	for _, err := range font.type3Err {
		if err != nil {
			return err
		}
	}
	return nil
}

// WithVariationCoordinates returns an owned font snapshot evaluated at the
// supplied normalized design-space coordinates. Coordinates use the OpenType
// range [-1, 1]; omitted axes use the default coordinate 0.
func (font *Font) WithVariationCoordinates(coords []float64) *Font {
	clone := font.Finalize()
	if clone == nil {
		return nil
	}
	clone.variationCoords = make([]float64, len(coords))
	for index, coord := range coords {
		if math.IsNaN(coord) {
			coord = 0
		} else if coord < -1 {
			coord = -1
		} else if coord > 1 {
			coord = 1
		}
		clone.variationCoords[index] = coord
	}
	clone.cffPathCache = nil
	clone.cffPathCacheBytes = 0
	clone.recomputeCFFWidths()
	return clone
}

// VariationCoordinatesCopy returns the normalized coordinates used by CFF2
// blend evaluation.
func (font *Font) VariationCoordinatesCopy() []float64 {
	font.lazyMutex().Lock()
	defer font.lazyMutex().Unlock()
	return cloneFontFloats(font.variationCoords)
}

// ResourcesCopy returns an independent copy of Type3 font resources.
func (font *Font) ResourcesCopy() Dict {
	font.lazyMutex().Lock()
	defer font.lazyMutex().Unlock()
	return cloneDict(font.resources)
}

// EncodingValue returns the Unicode value assigned to a single-byte code.
func (font *Font) EncodingValue(code byte) (rune, bool) {
	value, ok, _ := font.EncodingValueWithError(code)
	return value, ok
}

// EncodingValueWithError returns a single-byte encoding value and preserves
// lazy Type1/CFF parsing errors.
func (font *Font) EncodingValueWithError(code byte) (rune, bool, error) {
	if _, err := font.ensureType1EncodingWithError(); err != nil {
		return 0, false, err
	}
	if _, err := font.ensureCFFWithError(); err != nil {
		return 0, false, err
	}
	font.lazyMutex().Lock()
	defer font.lazyMutex().Unlock()
	value, ok := font.encoding[code]
	return value, ok, nil
}

// GlyphName returns the glyph name assigned to a single-byte code.
func (font *Font) GlyphName(code byte) (string, bool) {
	value, ok, _ := font.GlyphNameWithError(code)
	return value, ok
}

// GlyphNameWithError returns a glyph name and preserves lazy CFF parsing
// errors.
func (font *Font) GlyphNameWithError(code byte) (string, bool, error) {
	if _, err := font.ensureType1EncodingWithError(); err != nil {
		return "", false, err
	}
	if _, err := font.ensureCFFWithError(); err != nil {
		return "", false, err
	}
	font.lazyMutex().Lock()
	defer font.lazyMutex().Unlock()
	value, ok := font.glyphNames[code]
	return value, ok, nil
}

// WidthValue returns the explicitly assigned width for a single-byte code.
func (font *Font) WidthValue(code byte) (float64, bool) {
	value, ok, _ := font.WidthValueWithError(code)
	return value, ok
}

// WidthValueWithError returns an explicitly assigned width and preserves
// lazy TrueType/CFF parsing errors.
func (font *Font) WidthValueWithError(code byte) (float64, bool, error) {
	if _, err := font.ensureTrueTypeWithError(); err != nil {
		return 0, false, err
	}
	if _, err := font.ensureCFFWithError(); err != nil {
		return 0, false, err
	}
	font.lazyMutex().Lock()
	defer font.lazyMutex().Unlock()
	value, ok := font.widths[code]
	return value, ok, nil
}

// CIDWidth returns the width assigned to a CID.
func (font *Font) CIDWidth(cid int) (float64, bool) {
	value, ok, _ := font.CIDWidthWithError(cid)
	return value, ok
}

// CIDWidthWithError returns the width assigned to a CID and preserves errors
// from deferred embedded-font parsing.
func (font *Font) CIDWidthWithError(cid int) (float64, bool, error) {
	if _, err := font.ensureTrueTypeWithError(); err != nil {
		return 0, false, err
	}
	if _, err := font.ensureCFFWithError(); err != nil {
		return 0, false, err
	}
	font.lazyMutex().Lock()
	defer font.lazyMutex().Unlock()
	value, ok := font.cidWidths[cid]
	return value, ok, nil
}

// CIDToGIDCopy returns an independent snapshot of the font's CID-to-GID
// mapping. The mapping is parsed lazily from CIDToGIDMap or an embedded CFF
// charset and the returned map never aliases the font cache.
func (font *Font) CIDToGIDCopy() map[int]int {
	mapping, _ := font.CIDToGIDCopyWithError()
	return mapping
}

// CIDToGIDCopyWithError is the error-aware CID-to-GID snapshot accessor.
func (font *Font) CIDToGIDCopyWithError() (map[int]int, error) {
	if _, err := font.ensureCIDToGIDWithError(); err != nil {
		return nil, err
	}
	if _, err := font.ensureCFFWithError(); err != nil {
		return nil, err
	}
	font.lazyMutex().Lock()
	defer font.lazyMutex().Unlock()
	return cloneIntIntMap(font.cidToGID), nil
}

// VerticalWidth returns the vertical width assigned to a CID.
func (font *Font) VerticalWidth(cid int) (float64, bool) {
	value, ok, _ := font.VerticalWidthWithError(cid)
	return value, ok
}

// VerticalWidthWithError returns the vertical width assigned to a CID and
// preserves errors from deferred embedded-font parsing.
func (font *Font) VerticalWidthWithError(cid int) (float64, bool, error) {
	if _, err := font.ensureTrueTypeWithError(); err != nil {
		return 0, false, err
	}
	if _, err := font.ensureCFFWithError(); err != nil {
		return 0, false, err
	}
	font.lazyMutex().Lock()
	defer font.lazyMutex().Unlock()
	value, ok := font.verticalWidths[cid]
	return value, ok, nil
}

// VerticalPosition returns the vertical origin assigned to a CID.
func (font *Font) VerticalPosition(cid int) ([2]float64, bool) {
	value, ok, _ := font.VerticalPositionWithError(cid)
	return value, ok
}

// VerticalPositionWithError returns the vertical origin assigned to a CID and
// preserves errors from deferred embedded-font parsing.
func (font *Font) VerticalPositionWithError(cid int) ([2]float64, bool, error) {
	if _, err := font.ensureTrueTypeWithError(); err != nil {
		return [2]float64{}, false, err
	}
	if _, err := font.ensureCFFWithError(); err != nil {
		return [2]float64{}, false, err
	}
	font.lazyMutex().Lock()
	defer font.lazyMutex().Unlock()
	value, ok := font.verticalPositions[cid]
	return value, ok, nil
}

// ToUnicodeValue returns the Unicode mapping assigned to a two-byte code.
func (font *Font) ToUnicodeValue(code uint16) (string, bool) {
	value, ok, _ := font.ToUnicodeValueWithError(code)
	return value, ok
}

// ToUnicodeValueWithError returns a ToUnicode mapping and preserves its
// deferred parse or decode error.
func (font *Font) ToUnicodeValueWithError(code uint16) (string, bool, error) {
	if _, err := font.ensureToUnicodeWithError(); err != nil {
		return "", false, err
	}
	font.lazyMutex().Lock()
	defer font.lazyMutex().Unlock()
	value, ok := font.toUnicode[code]
	return value, ok, nil
}

// ToUnicodeCodeValue returns the Unicode mapping assigned to the complete
// source-code byte sequence. Unlike ToUnicodeValue, it preserves three- and
// four-byte source codes used by some CMaps.
func (font *Font) ToUnicodeCodeValue(code []byte) (string, bool) {
	value, ok, _ := font.ToUnicodeCodeValueWithError(code)
	return value, ok
}

// ToUnicodeCodeValueWithError returns a source-code Unicode mapping and
// preserves deferred ToUnicode parsing or stream-decoding errors.
func (font *Font) ToUnicodeCodeValueWithError(code []byte) (string, bool, error) {
	if _, err := font.ensureToUnicodeWithError(); err != nil {
		return "", false, err
	}
	font.lazyMutex().Lock()
	defer font.lazyMutex().Unlock()
	value, ok := font.codeMap[string(code)]
	return value, ok, nil
}

// CMapSnapshot returns an independent CMap snapshot, when present.
func (font *Font) CMapSnapshot() *fontdata.CMap {
	cmap, _ := font.CMapSnapshotWithError()
	return cmap
}

// CMapSnapshotWithError returns an independent CMap snapshot and preserves
// deferred embedded-CMap errors.
func (font *Font) CMapSnapshotWithError() (*fontdata.CMap, error) {
	if _, err := font.ensureCMapWithError(); err != nil {
		return nil, err
	}
	font.lazyMutex().Lock()
	defer font.lazyMutex().Unlock()
	return cloneCMap(font.cmap), nil
}

func (font *Font) ensureCMap() {
	_, _ = font.ensureCMapWithError()
}

func (font *Font) ensureCMapWithError() (bool, error) {
	if font == nil {
		return false, nil
	}
	font.lazyMutex().Lock()
	defer font.lazyMutex().Unlock()
	font.ensureCMapLocked()
	return font.cmapParsed, font.cmapErr
}

func (font *Font) ensureCMapLocked() {
	if font == nil || font.cmapParsed || font.cmapData == nil {
		return
	}
	font.cmapParsed = true
	data := font.cmapData
	defer func() {
		font.cmapData = nil
		font.cmapFilters = nil
		font.cmapFilterParms = nil
	}()
	if len(font.cmapFilters) > 0 {
		decoded, err := decodeFiltersLimited(data, font.cmapFilters, font.cmapFilterParms, decodedFilterExpansionLimit)
		if err != nil {
			font.cmapErr = fmt.Errorf("playa: decode embedded CMap: %w", err)
			return
		}
		data = decoded
	}
	cmap, err := fontdata.ParseEncodingCMap(data)
	if err != nil {
		font.cmapErr = fmt.Errorf("playa: parse embedded CMap: %w", err)
		return
	}
	if cmap.UseCMap() != "" && font.document != nil {
		fontdata.MergeUseCMap(cmap, font.document.loadUseCMap(Name(cmap.UseCMap())))
	}
	if font.cmapUse != nil && font.document != nil {
		fontdata.MergeUseCMap(cmap, font.document.loadUseCMap(font.cmapUse))
	}
	font.cmap = cmap
	font.vertical = cmap.Vertical()
}

func (font *Font) ensureToUnicode() {
	_, _ = font.ensureToUnicodeWithError()
}

func (font *Font) ensureToUnicodeWithError() (bool, error) {
	if font == nil {
		return false, nil
	}
	font.lazyMutex().Lock()
	defer font.lazyMutex().Unlock()
	font.ensureToUnicodeLocked()
	return font.toUnicodeParsed, font.toUnicodeErr
}

func (font *Font) ensureToUnicodeLocked() {
	if font == nil || font.toUnicodeParsed || font.toUnicodeData == nil {
		return
	}
	font.toUnicodeParsed = true
	data := font.toUnicodeData
	defer func() {
		font.toUnicodeData = nil
		font.toUnicodeFilters = nil
		font.toUnicodeParms = nil
	}()
	if len(font.toUnicodeFilters) > 0 {
		decoded, err := decodeFiltersLimited(data, font.toUnicodeFilters, font.toUnicodeParms, decodedFilterExpansionLimit)
		if err != nil {
			font.toUnicodeErr = fmt.Errorf("playa: decode embedded ToUnicode: %w", err)
			return
		}
		data = decoded
	}
	toUnicodeMap, err := fontdata.ParseToUnicodeMap(data)
	if err != nil {
		font.toUnicodeErr = fmt.Errorf("playa: parse embedded ToUnicode: %w", err)
		return
	}
	font.codeMap = toUnicodeMap.MappingsCopy()
	font.toUnicode = make(map[uint16]string, len(font.codeMap))
	for source, value := range font.codeMap {
		if len(source) == 0 || len(source) > 2 {
			continue
		}
		var code uint16
		for _, b := range []byte(source) {
			code = code<<8 | uint16(b)
		}
		font.toUnicode[code] = value
	}
	if _, err := fontdata.ParseToUnicodeCodesStrict(data); err != nil {
		if errors.Is(err, fontdata.ErrToUnicodeRangeOverflow) {
			markMalformedToUnicodeFont(font)
			return
		}
		font.toUnicodeErr = fmt.Errorf("playa: parse embedded ToUnicode codes: %w", err)
		return
	}
	font.toUnicodeSpaces = cloneCodeSpaces(toUnicodeMap.CodespacesCopy())
	if !font.cid {
		font.toUnicodeSpaces = []fontdata.CodeSpace{fontdata.NewCodeSpace([]byte{0}, []byte{255})}
	}
}

// markMalformedToUnicodeFont matches Playa's recovery for a valid PDF font
// whose ToUnicode range cannot be represented safely. The font remains usable
// for layout, but text is emitted one source byte at a time and its name is
// intentionally unknown rather than pretending the malformed mapping worked.
func markMalformedToUnicodeFont(font *Font) {
	font.name = "unknown"
	font.baseName = "unknown"
	font.cidCoding = ""
	font.cid = false
	font.vertical = false
	font.strictEncoding = false
	font.encoding = make(map[byte]rune, 256)
	for value := 0; value <= 0xff; value++ {
		font.encoding[byte(value)] = rune(value)
	}
	font.glyphTexts = map[byte]string{}
	font.glyphNames = map[byte]string{}
	font.toUnicode = map[uint16]string{}
	font.codeMap = map[string]string{}
	font.toUnicodeSpaces = []fontdata.CodeSpace{fontdata.NewCodeSpace([]byte{0}, []byte{255})}
}

func (font *Font) ensureCIDToGID() {
	_, _ = font.ensureCIDToGIDWithError()
}

func (font *Font) ensureCIDToGIDWithError() (bool, error) {
	if font == nil {
		return false, nil
	}
	font.lazyMutex().Lock()
	defer font.lazyMutex().Unlock()
	font.ensureCIDToGIDLocked()
	return font.cidToGIDParsed, font.cidToGIDErr
}

func (font *Font) ensureCIDToGIDLocked() {
	if font == nil || font.cidToGIDParsed || font.cidToGIDData == nil {
		return
	}
	font.cidToGIDParsed = true
	data := font.cidToGIDData
	defer func() {
		font.cidToGIDData = nil
		font.cidToGIDFilters = nil
		font.cidToGIDParms = nil
	}()
	if len(font.cidToGIDFilters) > 0 {
		decoded, err := decodeFiltersLimited(data, font.cidToGIDFilters, font.cidToGIDParms, decodedFilterExpansionLimit)
		if err != nil {
			font.cidToGIDErr = fmt.Errorf("playa: decode embedded CIDToGIDMap: %w", err)
			return
		}
		data = decoded
	}
	font.cidToGID = fontdata.ParseCIDToGIDMap(data)
	font.cidToGIDData = nil
	font.cidToGIDFilters = nil
	font.cidToGIDParms = nil
}

func (font *Font) ensureTrueType() {
	_, _ = font.ensureTrueTypeWithError()
}

func (font *Font) ensureTrueTypeWithError() (bool, error) {
	if font == nil {
		return false, nil
	}
	font.lazyMutex().Lock()
	defer font.lazyMutex().Unlock()
	font.ensureTrueTypeLocked()
	return font.trueTypeParsed, font.trueTypeErr
}

func (font *Font) ensureTrueTypeLocked() {
	if font == nil || font.trueTypeParsed || font.trueTypeData == nil {
		return
	}
	font.trueTypeParsed = true
	data := font.trueTypeData
	defer func() {
		font.trueTypeData = nil
		font.trueTypeFilters = nil
		font.trueTypeParms = nil
	}()
	if font.document != nil && font.trueTypeRef != (Ref{}) {
		if program := font.document.cachedTrueTypeProgram(font.trueTypeRef); program != nil {
			font.applyTrueTypeProgramLocked(program)
			return
		}
	}
	program := &trueTypeProgramState{}
	if len(font.trueTypeFilters) > 0 {
		decoded, err := decodeFiltersLimited(data, font.trueTypeFilters, font.trueTypeParms, decodedFilterExpansionLimit)
		if err != nil {
			program.err = fmt.Errorf("playa: decode embedded TrueType: %w", err)
			font.storeAndApplyTrueTypeProgramLocked(program)
			return
		}
		data = decoded
	}
	parsed, _ := sfnt.Parse(data)
	if parsed != nil {
		program.font = parsed
	}
	if glyphs := fontdata.ParseTrueTypeCMapGlyphs(data); len(glyphs) > 0 {
		program.glyphIDToUnicode = glyphs
		program.glyphUnicodeToID = make(map[rune]int, len(glyphs))
		for glyphID, text := range glyphs {
			if utf8.RuneCountInString(text) == 1 {
				r, _ := utf8.DecodeRuneInString(text)
				program.glyphUnicodeToID[r] = glyphID
			}
		}
	}
	if widths := fontdata.ParseTrueTypeHorizontalMetrics(data); len(widths) > 0 {
		program.glyphIDWidths = widths
	}
	font.storeAndApplyTrueTypeProgramLocked(program)
}

func (font *Font) storeAndApplyTrueTypeProgramLocked(program *trueTypeProgramState) {
	if font.document != nil && font.trueTypeRef != (Ref{}) {
		program = font.document.storeTrueTypeProgram(font.trueTypeRef, font, program)
	}
	font.applyTrueTypeProgramLocked(program)
}

func (font *Font) applyTrueTypeProgramLocked(program *trueTypeProgramState) {
	if program == nil {
		return
	}
	font.trueTypeProgram = program
	font.trueTypeFont = program.font
	// The parsed sfnt is immutable and safe to share. These maps remain part of
	// an individual PDF Font's mutable lazy state (OpenType/CFF metadata may
	// augment them later), so each dictionary needs its own copy.
	font.glyphIDToUnicode = cloneIntStringMap(program.glyphIDToUnicode)
	font.glyphUnicodeToID = cloneRuneIntMap(program.glyphUnicodeToID)
	font.glyphIDWidths = cloneIntFloatMap(program.glyphIDWidths)
	font.trueTypeErr = program.err
}

func (font *Font) trueTypePathOps(code []byte) ([]ContentOp, bool, error) {
	return font.trueTypePathOpsWithGID(code, 0)
}

func (font *Font) trueTypePathOpsWithGID(code []byte, gidHint int) ([]ContentOp, bool, error) {
	if font == nil || len(code) == 0 {
		return nil, false, nil
	}
	font.lazyMutex().Lock()
	defer font.lazyMutex().Unlock()
	font.ensureTrueTypeLocked()
	if font.trueTypeErr != nil {
		return nil, false, font.trueTypeErr
	}
	if font.trueTypeFont == nil {
		return nil, false, nil
	}
	if gidHint <= 0 && font.cid {
		font.ensureCMapLocked()
		if font.cmapErr != nil {
			return nil, false, font.cmapErr
		}
	}
	glyphID := 0
	if gidHint > 0 {
		glyphID = gidHint
	} else if font.cid {
		font.ensureCIDToGIDLocked()
		cid := codeNumber(code)
		if font.cmap != nil {
			if mapped, ok := fontdata.MappingValue(font.cmap, code); ok {
				cid = mapped
			}
		}
		if mapped, ok := font.cidToGID[cid]; ok {
			glyphID = mapped
		}
	} else {
		runeValue, ok := font.encoding[code[len(code)-1]]
		if !ok {
			return nil, false, nil
		}
		if mapped, ok := font.glyphUnicodeToID[runeValue]; ok {
			glyphID = mapped
		} else if mapped, ok := font.glyphIDForUnicodeLocked(runeValue); ok {
			glyphID = mapped
		}
	}
	if cached, ok := font.trueTypePathCache[glyphID]; ok {
		return cloneContentOps(cached), len(cached) > 0, nil
	}
	var buffer sfnt.Buffer
	segments, err := font.trueTypeFont.LoadGlyph(&buffer, sfnt.GlyphIndex(glyphID), fixed.I(1000), nil)
	if err != nil {
		return nil, false, fmt.Errorf("playa: load TrueType glyph %d: %w", glyphID, err)
	}
	ops := trueTypeSegmentsToOps(segments, 1)
	if font.trueTypePathCache == nil {
		font.trueTypePathCache = make(map[int][]ContentOp)
	}
	entry := cloneContentOps(ops)
	entrySize := contentOpsCacheSize(entry)
	if cacheFits(font.trueTypePathBytes, entrySize, font.trueTypePathCacheBudget()) {
		font.trueTypePathCache[glyphID] = entry
		font.trueTypePathBytes += entrySize
	}
	return ops, len(ops) > 0, nil
}

func trueTypeSegmentsToOps(segments sfnt.Segments, scale float64) []ContentOp {
	ops := make([]ContentOp, 0, len(segments)+1)
	var current [2]float64
	point := func(value fixed.Point26_6) [2]float64 {
		return [2]float64{float64(value.X) / 64 * scale, -float64(value.Y) / 64 * scale}
	}
	for _, segment := range segments {
		switch segment.Op {
		case sfnt.SegmentOpMoveTo:
			p := point(segment.Args[0])
			current = p
			ops = append(ops, newContentOpBorrowed("m", []Object{Number(p[0]), Number(p[1])}, 0))
		case sfnt.SegmentOpLineTo:
			p := point(segment.Args[0])
			current = p
			ops = append(ops, newContentOpBorrowed("l", []Object{Number(p[0]), Number(p[1])}, 0))
		case sfnt.SegmentOpQuadTo:
			control, end := point(segment.Args[0]), point(segment.Args[1])
			c1 := [2]float64{current[0] + (control[0]-current[0])*2/3, current[1] + (control[1]-current[1])*2/3}
			c2 := [2]float64{end[0] + (control[0]-end[0])*2/3, end[1] + (control[1]-end[1])*2/3}
			current = end
			ops = append(ops, newContentOpBorrowed("c", []Object{Number(c1[0]), Number(c1[1]), Number(c2[0]), Number(c2[1]), Number(end[0]), Number(end[1])}, 0))
		case sfnt.SegmentOpCubeTo:
			control1, control2, end := point(segment.Args[0]), point(segment.Args[1]), point(segment.Args[2])
			current = end
			ops = append(ops, newContentOpBorrowed("c", []Object{Number(control1[0]), Number(control1[1]), Number(control2[0]), Number(control2[1]), Number(end[0]), Number(end[1])}, 0))
		}
	}
	return ops
}

func (font *Font) ensureCFF() {
	_, _ = font.ensureCFFWithError()
}

func (font *Font) ensureCFFWithError() (bool, error) {
	if font == nil {
		return false, nil
	}
	font.lazyMutex().Lock()
	defer font.lazyMutex().Unlock()
	font.ensureCFFLocked()
	return font.cffParsed, font.cffErr
}

func (font *Font) ensureCFFLocked() {
	if font == nil || font.cffParsed || font.cffData == nil {
		return
	}
	font.cffParsed = true
	data := font.cffData
	defer func() {
		font.cffData = nil
		font.cffFilters = nil
		font.cffParms = nil
	}()
	if len(font.cffFilters) > 0 {
		decoded, err := decodeFiltersLimited(data, font.cffFilters, font.cffParms, decodedFilterExpansionLimit)
		if err != nil {
			font.cffErr = fmt.Errorf("playa: decode embedded CFF: %w", err)
			return
		}
		data = decoded
	}
	atomic.StorePointer(&font.widthState, nil)
	if !applyCFFMetadataData(font, data) {
		font.cffErr = fmt.Errorf("playa: invalid embedded CFF data")
	}
}

// CharProc returns an independent Type3 character procedure stream.
func (font *Font) CharProc(name string) (Stream, bool) {
	value, ok := font.charProcSnapshot(name)
	return value, ok
}

func (font *Font) charProcSnapshot(name string) (Stream, bool) {
	font.lazyMutex().Lock()
	defer font.lazyMutex().Unlock()
	value, ok := font.charProcs[name]
	if !ok {
		return Stream{}, false
	}
	return cloneGraphicsObject(value).(Stream), true
}

func (font *Font) type3CharProcSnapshot(code []byte) (string, Stream, Dict, bool) {
	if font == nil || len(code) == 0 {
		return "", Stream{}, nil, false
	}
	font.lazyMutex().Lock()
	defer font.lazyMutex().Unlock()
	name, ok := font.glyphNames[code[len(code)-1]]
	if !ok {
		return "", Stream{}, nil, false
	}
	proc, ok := font.charProcs[name]
	if !ok {
		return name, Stream{}, nil, false
	}
	return name, cloneGraphicsObject(proc).(Stream), cloneDict(font.resources), true
}

func (font *Font) type3GlyphOpsSnapshot(code []byte) ([]ContentOp, bool) {
	if font == nil || len(code) == 0 {
		return nil, false
	}
	font.lazyMutex().Lock()
	defer font.lazyMutex().Unlock()
	name, ok := font.glyphNames[code[len(code)-1]]
	if !ok {
		return nil, false
	}
	_ = font.ensureType3CharProcLocked(name)
	return cloneContentOps(font.charProcOps[name]), true
}

func (font *Font) ensureType3CharProcWithError(name string) ([]ContentOp, error) {
	if font == nil {
		return nil, nil
	}
	font.lazyMutex().Lock()
	defer font.lazyMutex().Unlock()
	err := font.ensureType3CharProcLocked(name)
	return font.charProcOps[name], err
}

func (font *Font) ensureType3CharProcLocked(name string) error {
	if font == nil || !font.type3 {
		return nil
	}
	if font.type3Parsed[name] {
		return font.type3Err[name]
	}
	if font.type3Parsed == nil {
		font.type3Parsed = make(map[string]bool)
	}
	if font.type3Err == nil {
		font.type3Err = make(map[string]error)
	}
	font.type3Parsed[name] = true
	entryBytes := len(name) + 32
	cacheLimit := font.type3CacheBudget()
	cacheEntry := cacheFits(font.type3CacheBytes, entryBytes, cacheLimit)
	if cacheEntry {
		font.type3CacheBytes += entryBytes
	} else {
		delete(font.type3Parsed, name)
	}
	if proc, ok := font.charProcs[name]; ok && font.document != nil {
		if err := parseType3CharProcWithError(font.document, font, name, proc); err != nil {
			if cacheEntry {
				errorBytes := len(err.Error())
				if cacheFits(font.type3CacheBytes, errorBytes, cacheLimit) {
					font.type3Err[name] = err
					font.type3CacheBytes += errorBytes
				} else {
					delete(font.type3Parsed, name)
					font.type3CacheBytes -= entryBytes
					font.type3OverflowErr = err
				}
			} else {
				font.type3OverflowErr = err
			}
			return err
		}
	}
	return nil
}

func cloneByteRuneMap(source map[byte]rune) map[byte]rune {
	if source == nil {
		return nil
	}
	clone := make(map[byte]rune, len(source))
	for key, value := range source {
		clone[key] = value
	}
	return clone
}

func cloneByteStringMap(source map[byte]string) map[byte]string {
	if source == nil {
		return nil
	}
	clone := make(map[byte]string, len(source))
	for key, value := range source {
		clone[key] = value
	}
	return clone
}

func cloneByteFloatMap(source map[byte]float64) map[byte]float64 {
	if source == nil {
		return nil
	}
	clone := make(map[byte]float64, len(source))
	for key, value := range source {
		clone[key] = value
	}
	return clone
}

func cloneByteIntMap(source map[byte]int) map[byte]int {
	if source == nil {
		return nil
	}
	out := make(map[byte]int, len(source))
	for key, value := range source {
		out[key] = value
	}
	return out
}

func cloneByteSlices(source [][]byte) [][]byte {
	if source == nil {
		return nil
	}
	out := make([][]byte, len(source))
	for i, value := range source {
		out[i] = cloneObjectBytes(value)
	}
	return out
}

func cloneIntByteSlicesMap(source map[int][][]byte) map[int][][]byte {
	if source == nil {
		return nil
	}
	out := make(map[int][][]byte, len(source))
	for key, value := range source {
		out[key] = cloneByteSlices(value)
	}
	return out
}

func cloneRuneFloatMap(source map[rune]float64) map[rune]float64 {
	if source == nil {
		return nil
	}
	clone := make(map[rune]float64, len(source))
	for key, value := range source {
		clone[key] = value
	}
	return clone
}

func cloneIntFloatMap(source map[int]float64) map[int]float64 {
	if source == nil {
		return nil
	}
	clone := make(map[int]float64, len(source))
	for key, value := range source {
		clone[key] = value
	}
	return clone
}

func cloneIntPositionMap(source map[int][2]float64) map[int][2]float64 {
	if source == nil {
		return nil
	}
	clone := make(map[int][2]float64, len(source))
	for key, value := range source {
		clone[key] = value
	}
	return clone
}

func cloneUint16StringMap(source map[uint16]string) map[uint16]string {
	if source == nil {
		return nil
	}
	clone := make(map[uint16]string, len(source))
	for key, value := range source {
		clone[key] = value
	}
	return clone
}

func cloneStringMap(source map[string]string) map[string]string {
	if source == nil {
		return nil
	}
	clone := make(map[string]string, len(source))
	for key, value := range source {
		clone[key] = value
	}
	return clone
}

func cloneCodeSpaces(source []fontdata.CodeSpace) []fontdata.CodeSpace {
	if source == nil {
		return nil
	}
	out := make([]fontdata.CodeSpace, len(source))
	for i, space := range source {
		out[i] = space.Finalize()
	}
	return out
}

func cloneIntStringMap(source map[int]string) map[int]string {
	if source == nil {
		return nil
	}
	clone := make(map[int]string, len(source))
	for key, value := range source {
		clone[key] = value
	}
	return clone
}

func cloneRuneIntMap(source map[rune]int) map[rune]int {
	if source == nil {
		return nil
	}
	clone := make(map[rune]int, len(source))
	for key, value := range source {
		clone[key] = value
	}
	return clone
}

func cloneIntIntMap(source map[int]int) map[int]int {
	if source == nil {
		return nil
	}
	clone := make(map[int]int, len(source))
	for key, value := range source {
		clone[key] = value
	}
	return clone
}

func cloneStreamMap(source map[string]Stream) map[string]Stream {
	if source == nil {
		return nil
	}
	clone := make(map[string]Stream, len(source))
	for key, value := range source {
		clone[key] = cloneGraphicsObject(value).(Stream)
	}
	return clone
}

func cloneStringFloatMap(source map[string]float64) map[string]float64 {
	if source == nil {
		return nil
	}
	clone := make(map[string]float64, len(source))
	for key, value := range source {
		clone[key] = value
	}
	return clone
}

func cloneStringBBoxMap(source map[string][4]float64) map[string][4]float64 {
	if source == nil {
		return nil
	}
	clone := make(map[string][4]float64, len(source))
	for key, value := range source {
		clone[key] = value
	}
	return clone
}

func cloneContentOpMap(source map[string][]ContentOp) map[string][]ContentOp {
	if source == nil {
		return nil
	}
	clone := make(map[string][]ContentOp, len(source))
	for key, ops := range source {
		clone[key] = cloneContentOps(ops)
	}
	return clone
}

func cloneContentOpIntMap(source map[int][]ContentOp) map[int][]ContentOp {
	if source == nil {
		return nil
	}
	clone := make(map[int][]ContentOp, len(source))
	for key, ops := range source {
		clone[key] = cloneContentOps(ops)
	}
	return clone
}

func cloneStringBoolMap(source map[string]bool) map[string]bool {
	if source == nil {
		return nil
	}
	clone := make(map[string]bool, len(source))
	for key, value := range source {
		clone[key] = value
	}
	return clone
}

func cloneStringErrorMap(source map[string]error) map[string]error {
	if source == nil {
		return nil
	}
	clone := make(map[string]error, len(source))
	for key, value := range source {
		clone[key] = value
	}
	return clone
}

func NewSimpleFont(name string) *Font {
	f := &Font{lazyMu: &sync.Mutex{}, name: name, encoding: map[byte]rune{}, glyphTexts: map[byte]string{}, glyphNames: map[byte]string{}, widths: map[byte]float64{}, cidWidths: map[int]float64{}, verticalWidths: map[int]float64{}, verticalPositions: map[int][2]float64{}, toUnicode: map[uint16]string{}, codeMap: map[string]string{}, charProcs: map[string]Stream{}, charProcOps: map[string][]ContentOp{}, type3Parsed: map[string]bool{}, type3Err: map[string]error{}, charWidths: map[string]float64{}, charBBoxes: map[string][4]float64{}, cffGlyphIDs: map[byte]int{}, defaultWidth: 500, defaultVWidth: -1000, ascent: 880, descent: -120, fontMatrix: geometry.Matrix{0.001, 0, 0, 0.001, 0, 0}}
	for i := byte(32); i < 127; i++ {
		f.encoding[i] = rune(i)
	}
	return f
}

// Name returns the resolved font name.
func (f *Font) Name() string {
	return f.name
}

// BaseFont returns the PDF /BaseFont identity. For Type 0 fonts this comes
// from the descendant font, matching Playa. It may differ from Name, which
// can come from the font descriptor.
func (f *Font) BaseFont() string {
	return f.baseName
}

// CIDCoding returns the CIDSystemInfo registry-ordering identity used by a
// CID font, for example "Adobe-Japan1". Non-CID fonts return an empty string.
func (f *Font) CIDCoding() string {
	return f.cidCoding
}

// Subtype returns the PDF font subtype when one was resolved.
func (f *Font) Subtype() string {
	return f.subtype
}

// IsType3 reports whether the font uses Type3 CharProcs.
func (f *Font) IsType3() bool { return f.type3 }

// IsVertical reports whether the font writes in vertical mode.
func (f *Font) IsVertical() bool { return f.vertical }

// IsCID reports whether the font uses a CID encoding.
func (f *Font) IsCID() bool { return f.cid }

// IsMultibyte reports Playa's public multibyte font flag. Playa 1.1.0
// declares this class attribute but does not set it for any supported font
// subclass, so its observable value is false for every parsed font.
func (f *Font) IsMultibyte() bool { return false }

// FontMatrix returns the font transformation matrix.
func (f *Font) FontMatrix() geometry.Matrix {
	return f.fontMatrix
}

// DefaultWidth returns the fallback horizontal width in glyph-space units.
func (f *Font) DefaultWidth() float64 {
	return f.defaultWidth
}

// DefaultVWidth returns the fallback vertical displacement in glyph-space
// units.
func (f *Font) DefaultVWidth() float64 {
	return f.defaultVWidth
}

// DefaultVPosition returns the fallback vertical position vector.
func (f *Font) DefaultVPosition() [2]float64 {
	return f.defaultVPosition
}

// Ascent returns the font ascent in glyph-space units.
func (f *Font) Ascent() float64 {
	return f.ascent
}

// Descent returns the font descent in glyph-space units.
func (f *Font) Descent() float64 {
	return f.descent
}

// Leading returns the font leading metric.
func (f *Font) Leading() float64 {
	return f.leading
}

// FontBBox returns the descriptor bounding box and whether it was present.
func (f *Font) FontBBox() ([4]float64, bool) {
	return f.fontBBox, f.hasFontBBox
}

// Flags returns the descriptor flags and whether the PDF supplied them.
func (f *Font) Flags() (int, bool) {
	return f.flags, f.hasFlags
}

// CapHeight returns the font cap-height metric.
func (f *Font) CapHeight() float64 {
	return f.capHeight
}

// ItalicAngle returns the font italic angle.
func (f *Font) ItalicAngle() float64 {
	return f.italicAngle
}

// StemV returns the font vertical stem metric.
func (f *Font) StemV() float64 {
	return f.stemV
}

func (f *Font) VDisp(cid int) float64 {
	displacement, _ := f.VDispWithError(cid)
	return displacement
}

// VDispWithError returns vertical text-space displacement and preserves lazy
// embedded-font and Type3/CFF parsing errors.
func (f *Font) VDispWithError(cid int) (float64, error) {
	if !f.vertical {
		return 0, nil
	}
	if _, err := f.ensureTrueTypeWithError(); err != nil {
		return 0, err
	}
	if _, err := f.ensureCFFWithError(); err != nil {
		return 0, err
	}
	f.lazyMutex().Lock()
	defer f.lazyMutex().Unlock()
	if width, ok := f.verticalWidths[cid]; ok {
		return f.fontMatrix[3] * width, nil
	}
	return f.fontMatrix[3] * f.defaultVWidth, nil
}

func (f *Font) VPosition(cid int) [2]float64 {
	position, _ := f.VPositionWithError(cid)
	return position
}

// VPositionWithError returns the vertical origin vector and preserves lazy
// embedded-font and Type3/CFF parsing errors.
func (f *Font) VPositionWithError(cid int) ([2]float64, error) {
	if _, err := f.ensureTrueTypeWithError(); err != nil {
		return [2]float64{}, err
	}
	if _, err := f.ensureCFFWithError(); err != nil {
		return [2]float64{}, err
	}
	f.lazyMutex().Lock()
	defer f.lazyMutex().Unlock()
	if position, ok := f.verticalPositions[cid]; ok {
		return position, nil
	}
	return f.defaultVPosition, nil
}

// HDisp returns the horizontal displacement in text-space units.
func (f *Font) HDisp(cid int) float64 {
	displacement, _ := f.HDispWithError(cid)
	return displacement
}

// HDispWithError returns horizontal text-space displacement and preserves
// lazy embedded-font and Type3 CharProc errors.
func (f *Font) HDispWithError(cid int) (float64, error) {
	if _, err := f.ensureTrueTypeWithError(); err != nil {
		return 0, err
	}
	if _, err := f.ensureCFFWithError(); err != nil {
		return 0, err
	}
	width := f.defaultWidth
	if f.cid {
		f.lazyMutex().Lock()
		if value, ok := f.cidWidths[cid]; ok {
			width = value
		}
		f.lazyMutex().Unlock()
	} else if f.type3 {
		f.lazyMutex().Lock()
		if name, ok := f.glyphNames[byte(cid)]; ok {
			f.lazyMutex().Unlock()
			if _, err := f.ensureType3CharProcWithError(name); err != nil {
				return 0, err
			}
			f.lazyMutex().Lock()
			if value, found := f.charWidths[name]; found {
				width = value
			}
			f.lazyMutex().Unlock()
		} else {
			f.lazyMutex().Unlock()
		}
	} else {
		f.lazyMutex().Lock()
		if value, ok := f.widths[byte(cid)]; ok {
			width = value
		} else if f.standardWidths != nil && f.fontType != "TrueType" && !isSubsetFont(f.name) {
			if value, ok := encodedStandardWidth(byte(cid), f.encoding, f.glyphTexts, f.standardWidths); ok {
				width = value
			}
		}
		f.lazyMutex().Unlock()
	}
	return f.fontMatrix[0] * width, nil
}

// Position returns the glyph position vector in text-space units.
func (f *Font) Position(cid int) [2]float64 {
	if !f.vertical {
		return [2]float64{}
	}
	position := f.VPosition(cid)
	return [2]float64{f.fontMatrix[0] * position[0], f.fontMatrix[3] * position[1]}
}

// CharBBox returns the standard character bounding box in text-space units.
// It is a layout box, not the actual outline bounds of the glyph.
func (f *Font) CharBBox(cid int) [4]float64 {
	bbox, _ := f.CharBBoxWithError(cid)
	return bbox
}

// CharBBoxWithError returns the standard character bounding box and preserves
// lazy Type3 and embedded-font errors used to calculate its width.
func (f *Font) CharBBoxWithError(cid int) ([4]float64, error) {
	if f.type3 {
		f.lazyMutex().Lock()
		name, found := f.glyphNames[byte(cid)]
		f.lazyMutex().Unlock()
		if found {
			if _, err := f.ensureType3CharProcWithError(name); err != nil {
				return [4]float64{}, err
			}
			f.lazyMutex().Lock()
			bbox, found := f.charBBoxes[name]
			f.lazyMutex().Unlock()
			if found {
				transformed, ok := transformFontBBox(f.fontMatrix, bbox)
				if !ok {
					return [4]float64{}, fmt.Errorf("playa: non-finite Type3 FontMatrix transform")
				}
				return transformed, nil
			}
		}
	}
	width, err := f.HDispWithError(cid)
	if err != nil {
		return [4]float64{}, err
	}
	bbox := [4]float64{0, f.fontMatrix[3] * f.descent, width, f.fontMatrix[3] * f.ascent}
	if !f.vertical {
		return bbox, nil
	}
	position := f.Position(cid)
	return [4]float64{
		-position[0],
		-position[1] + bbox[1],
		-position[0] + bbox[2],
		-position[1] + bbox[3],
	}, nil
}

func transformFontBBox(matrix geometry.Matrix, bbox [4]float64) ([4]float64, bool) {
	// Playa's transform_bbox only needs the diagonal corners when the
	// transform preserves the rectangle's x/y extrema. It expands to all
	// four corners only for rotations or shears that reverse an axis.
	points := [][2]float64{{bbox[0], bbox[1]}, {bbox[2], bbox[3]}}
	if matrix[0]*matrix[2] < 0 || matrix[1]*matrix[3] < 0 {
		points = [][2]float64{
			{bbox[0], bbox[1]},
			{bbox[0], bbox[3]},
			{bbox[2], bbox[3]},
			{bbox[2], bbox[1]},
		}
	}
	minX, minY := 0.0, 0.0
	maxX, maxY := 0.0, 0.0
	for i, point := range points {
		x, y, ok := matrix.PointFinite(point[0], point[1])
		if !ok {
			return [4]float64{}, false
		}
		if i == 0 {
			minX, maxX, minY, maxY = x, x, y, y
			continue
		}
		minX, maxX = min(minX, x), max(maxX, x)
		minY, maxY = min(minY, y), max(maxY, y)
	}
	return [4]float64{minX, minY, maxX, maxY}, true
}

// Glyph-name Unicode mappings are owned by fontdata and generated from the
// synchronized Adobe/ITC sources. Scoped historical aliases live there too.

func glyphText(name string) (string, bool) {
	return fontdata.GlyphTextWithLegacyAliases(name)
}

func (f *Font) applyDifferences(a Array) {
	f.lazyMutex().Lock()
	defer f.lazyMutex().Unlock()
	// Keep published width snapshots immutable while replacing the tables
	// used by subsequent decoding.
	f.glyphNames = cloneByteStringMap(f.glyphNames)
	f.glyphTexts = cloneByteStringMap(f.glyphTexts)
	f.encoding = cloneByteRuneMap(f.encoding)
	atomic.StorePointer(&f.widthState, nil)
	captureCFF := f.cffImplicitEncoding && f.cffData != nil && !f.cffParsed
	captureType1 := f.type1Data != nil && !f.type1Parsed
	if captureType1 {
		f.type1Differences = cloneByteStringMap(f.type1Differences)
		if f.type1Differences == nil {
			f.type1Differences = map[byte]string{}
		}
	}
	if captureCFF {
		f.cffDifferences = cloneByteStringMap(f.cffDifferences)
		if f.cffDifferences == nil {
			f.cffDifferences = map[byte]string{}
		}
	}
	code := -1
	for _, v := range a {
		if n, ok := IntValue(v); ok {
			if n >= 0 && n <= 255 {
				code = n
			} else {
				code = -1
			}
			continue
		}
		if name, ok := v.(Name); ok && code >= 0 && code <= 255 {
			f.glyphNames[byte(code)] = string(name)
			if captureType1 {
				f.type1Differences[byte(code)] = string(name)
			}
			if captureCFF {
				f.cffDifferences[byte(code)] = string(name)
			}
			if text, ok := glyphText(string(name)); ok {
				f.glyphTexts[byte(code)] = text
				delete(f.encoding, byte(code))
			} else {
				delete(f.glyphTexts, byte(code))
				delete(f.encoding, byte(code))
			}
			code++
		}
	}
}

func (f *Font) applyEncoding(name string) {
	f.lazyMutex().Lock()
	defer f.lazyMutex().Unlock()
	f.simpleEncodingName = name
	atomic.StorePointer(&f.widthState, nil)
	// A font can be reused while its encoding is being resolved through a
	// BaseEncoding chain.  Start from a clean table so entries from an earlier
	// built-in encoding cannot leak into the replacement encoding.
	f.encoding = map[byte]rune{}
	f.glyphTexts = map[byte]string{}
	switch name {
	case "StandardEncoding":
		f.strictEncoding = false
		f.encoding, _ = fontdata.BuiltinEncoding(name)
	case "WinAnsiEncoding":
		f.strictEncoding = false
		f.encoding, _ = fontdata.BuiltinEncoding(name)
	case "MacRomanEncoding":
		f.strictEncoding = false
		f.encoding, _ = fontdata.BuiltinEncoding(name)
	case "MacExpertEncoding":
		f.strictEncoding = true
		f.encoding, _ = fontdata.BuiltinEncoding(name)
	case "Symbol":
		f.strictEncoding = true
		f.encoding, _ = fontdata.BuiltinEncoding(name)
	case "ZapfDingbats":
		f.strictEncoding = true
		f.encoding, _ = fontdata.BuiltinEncoding(name)
	default:
		// Playa keeps an unrecognized simple-font Encoding table empty,
		// but decodes it through its StandardEncoding CID fallback. Keep
		// that fallback in the decode-only glyph table so EncodingValue
		// still reports the original empty table.
		f.strictEncoding = f.type3
		for i := byte(32); i < 127; i++ {
			f.glyphTexts[i] = string(rune(i))
		}
		if standard, ok := fontdata.BuiltinEncoding("StandardEncoding"); ok {
			for code, r := range standard {
				f.glyphTexts[code] = string(r)
			}
		}
	}
}

func (f *Font) Decode(data []byte) string {
	var b strings.Builder
	for _, glyph := range f.DecodeGlyphs(data) {
		b.WriteString(glyph.Text())
	}
	return b.String()
}

// DecodedGlyph is the dependency-free font-data value returned by glyph
// decoding. It remains aliased here so document-facing APIs keep their
// existing type name while ownership lives with fontdata.
type DecodedGlyph = fontdata.DecodedGlyph

func (f *Font) DecodeGlyphs(data []byte) []DecodedGlyph {
	f.prepareDecode()
	capacity := len(data)
	if f.cid && f.cmap != nil && capacity > 1 {
		capacity = (capacity + 1) / 2
	}
	out := make([]DecodedGlyph, 0, capacity)
	for glyph := range f.DecodeGlyphsSeq(data) {
		out = append(out, glyph)
	}
	for i := range out {
		out[i] = out[i].Finalize()
	}
	return out
}

// DecodeGlyphsSeq lazily decodes source bytes into borrowed glyphs. The
// sequence is repeatable and stops decoding as soon as the consumer stops
// yielding. Glyph code bytes borrow data; call Finalize before retaining a
// glyph beyond the current sequence step.
func (f *Font) DecodeGlyphsSeq(data []byte) iter.Seq[DecodedGlyph] {
	return func(yield func(DecodedGlyph) bool) {
		f.prepareDecode()
		f.decodeGlyphsEach(data, yield)
	}
}

// DecodeGlyphsSeqWithError is the error-aware variant of DecodeGlyphsSeq. It
// reports lazy embedded CMap failures instead of treating them as an absent
// character mapping.
func (f *Font) DecodeGlyphsSeqWithError(data []byte) iter.Seq2[DecodedGlyph, error] {
	return func(yield func(DecodedGlyph, error) bool) {
		if _, err := f.ensureCMapWithError(); err != nil {
			yield(DecodedGlyph{}, err)
			return
		}
		if _, err := f.ensureToUnicodeWithError(); err != nil {
			yield(DecodedGlyph{}, err)
			return
		}
		if _, err := f.ensureCIDToGIDWithError(); err != nil {
			yield(DecodedGlyph{}, err)
			return
		}
		if _, err := f.ensureTrueTypeWithError(); err != nil {
			yield(DecodedGlyph{}, err)
			return
		}
		if _, err := f.ensureCFFWithError(); err != nil {
			yield(DecodedGlyph{}, err)
			return
		}
		if _, err := f.ensureType1EncodingWithError(); err != nil {
			yield(DecodedGlyph{}, err)
			return
		}
		f.decodeGlyphsEach(data, func(glyph DecodedGlyph) bool {
			return yield(glyph, nil)
		})
	}
}

func (f *Font) prepareDecode() {
	f.ensureCMap()
	f.ensureToUnicode()
	f.ensureTrueType()
	f.ensureCFF()
	f.ensureType1Encoding()
}

func (f *Font) decodeGlyphsEach(data []byte, yield func(DecodedGlyph) bool) bool {
	snapshot := f.decodeSnapshot()
	if snapshot.cid && snapshot.cmap != nil {
		var toUnicodeDecoder *toUnicodeDecoder
		if snapshot.hasToUnicode {
			toUnicodeDecoder = newToUnicodeDecoder(data, snapshot.toUnicodeSpaces, snapshot.codeMap)
		}
		return fontdata.DecodeCMap(snapshot.cmap, data, func(codeBytes []byte, cid int) bool {
			if toUnicodeDecoder != nil {
				text, ok := toUnicodeDecoder.next()
				if !ok {
					return false
				}
				return yield(fontdata.BorrowedDecodedGlyph(text, codeBytes, cid))
			}
			// Without a parsed ToUnicode map, use the source-code mapping and
			// then the same CID/glyph fallbacks as Playa's CIDFont decoder.
			text, ok := snapshot.codeMap[string(codeBytes)]
			if !ok {
				if mapped, found := snapshot.toUnicode[uint16(codeNumber(codeBytes))]; found {
					text, ok = mapped, true
				}
			}
			if !ok {
				// Playa's identity fallback is based on the source code
				// bytes, not the CID produced by the encoding CMap. An embedded
				// glyph map is only a fallback when the font has no ToUnicode
				// map; Playa gives an existing ToUnicode map precedence, even
				// when the source code is not present in that map.
				if !snapshot.hasToUnicode {
					if mapped, found := snapshot.glyphIDToUnicode[f.glyphID(cid)]; found && !isPrivateUseText(mapped) {
						text = mapped
					} else if mapped, found := snapshot.cidToUnicode[cid]; found {
						text = mapped
					} else {
						text = string(rune(codeNumber(codeBytes)))
					}
				}
			}
			if !yield(fontdata.BorrowedDecodedGlyph(text, codeBytes, cid)) {
				return false
			}
			return true
		})
	}
	lengths := []int{4, 3, 2, 1}
	if len(snapshot.toUnicodeSpaces) > 0 {
		lengths = fontdata.ToUnicodeCodeLengths(snapshot.toUnicodeSpaces)
	}
	for i := 0; i < len(data); {
		matched := false
		for _, n := range lengths {
			if n > len(data)-i || (len(snapshot.toUnicodeSpaces) > 0 && !fontdata.CodeInSpaces(data[i:i+n], snapshot.toUnicodeSpaces)) {
				continue
			}
			code := data[i : i+n]
			if s, ok := snapshot.codeMap[string(code)]; ok {
				if !yield(fontdata.BorrowedDecodedGlyph(s, code, codeNumber(code))) {
					return false
				}
				i += n
				matched = true
				break
			}
			if len(snapshot.toUnicodeSpaces) > 0 {
				if !yield(fontdata.BorrowedDecodedGlyph(string(rune(codeNumber(code))), code, codeNumber(code))) {
					return false
				}
				i += n
				matched = true
				break
			}
		}
		if matched {
			continue
		}
		if snapshot.cid && i+1 < len(data) {
			code := uint16(data[i])<<8 | uint16(data[i+1])
			if s, ok := snapshot.toUnicode[code]; ok {
				if !yield(fontdata.BorrowedDecodedGlyph(s, data[i:i+2], int(code))) {
					return false
				}
				i += 2
				continue
			}
		}
		c := data[i]
		code := []byte{c}
		i++
		text := ""
		if s, ok := snapshot.glyphTexts[c]; ok {
			text = s
		} else if r, ok := snapshot.encoding[c]; ok {
			text = string(r)
		} else if s, ok := snapshot.toUnicode[uint16(c)]; ok {
			text = s
		} else if _, named := snapshot.glyphNames[c]; named {
			if snapshot.type3 && c >= 32 && c < 127 {
				// Type 3 glyph names describe painting procedures, not necessarily
				// Unicode semantics. Playa retains the printable source code when a
				// custom CharProc name is absent from the Adobe glyph list.
				text = string(c)
			} else {
				// An explicit Differences glyph without an Adobe glyph-list
				// mapping must not fall back to the source ASCII byte.
				text = ""
			}
		} else if !snapshot.strictEncoding && c >= 32 && c < 127 {
			text = string(c)
		} else if snapshot.strictEncoding {
			text = ""
		} else {
			// Playa's SimpleFont returns an empty string for an undefined
			// encoding slot. Replacement characters are reserved for
			// explicit Unicode decoding errors, not absent glyph names.
			text = ""
		}
		if !yield(fontdata.BorrowedDecodedGlyph(text, code, int(c))) {
			return false
		}
	}
	return true
}

type toUnicodeDecoder struct {
	data     []byte
	offset   int
	lengths  []int
	spaces   []fontdata.CodeSpace
	mappings map[string]string
}

func newToUnicodeDecoder(data []byte, spaces []fontdata.CodeSpace, mappings map[string]string) *toUnicodeDecoder {
	lengths := []int{1}
	if len(spaces) > 0 {
		lengths = fontdata.ToUnicodeCodeLengths(spaces)
	}
	return &toUnicodeDecoder{data: data, lengths: lengths, spaces: spaces, mappings: mappings}
}

func (d *toUnicodeDecoder) next() (string, bool) {
	if d == nil || d.offset >= len(d.data) {
		return "", false
	}
	if len(d.spaces) == 0 {
		value := string(rune(d.data[d.offset]))
		d.offset++
		return value, true
	}
	for _, length := range d.lengths {
		if length > len(d.data)-d.offset {
			continue
		}
		code := d.data[d.offset : d.offset+length]
		if len(d.spaces) > 0 && !fontdata.CodeInSpaces(code, d.spaces) {
			continue
		}
		d.offset += length
		if value, ok := d.mappings[string(code)]; ok {
			return value, true
		}
		return string(rune(codeNumber(code))), true
	}
	value := string(rune(d.data[d.offset]))
	d.offset++
	return value, true
}

func codeNumber(code []byte) int {
	n := 0
	for _, b := range code {
		n = n<<8 | int(b)
	}
	return n
}
func (f *Font) Width(c byte) float64 {
	width, _ := f.WidthWithError(c)
	return width
}

// WidthWithError returns a single-byte width and preserves lazy TrueType and
// CFF parsing errors.
func (f *Font) WidthWithError(c byte) (float64, error) {
	f.lazyMutex().Lock()
	defer f.lazyMutex().Unlock()
	f.ensureTrueTypeLocked()
	if f.trueTypeErr != nil {
		return 0, f.trueTypeErr
	}
	f.ensureCFFLocked()
	if f.cffErr != nil {
		return 0, f.cffErr
	}
	return f.widthLocked(c), nil
}

func (f *Font) widthLocked(c byte) float64 {
	f.ensureTrueTypeLocked()
	f.ensureCFFLocked()
	if w, ok := f.widths[c]; ok {
		return w
	}
	return f.defaultWidth
}

// GlyphBBox returns a Type3 CharProc bbox for a source code when the font's
// Encoding maps that code to a named CharProc.
func (f *Font) GlyphBBox(code []byte) ([4]float64, bool) {
	bbox, ok, _ := f.GlyphBBoxWithError(code)
	return bbox, ok
}

// GlyphBBoxWithError resolves a Type3 CharProc lazily and reports malformed
// CharProc metrics instead of conflating them with a missing bbox.
func (f *Font) GlyphBBoxWithError(code []byte) ([4]float64, bool, error) {
	if len(code) == 0 || !f.type3 {
		return [4]float64{}, false, nil
	}
	f.lazyMutex().Lock()
	name, ok := f.glyphNames[code[len(code)-1]]
	f.lazyMutex().Unlock()
	if !ok {
		return [4]float64{}, false, nil
	}
	if _, err := f.ensureType3CharProcWithError(name); err != nil {
		return [4]float64{}, false, err
	}
	f.lazyMutex().Lock()
	defer f.lazyMutex().Unlock()
	bbox, ok := f.charBBoxes[name]
	return bbox, ok, nil
}

func (f *Font) ensureType3GlyphWithError(code []byte) error {
	if f == nil || !f.type3 || len(code) == 0 {
		return nil
	}
	f.lazyMutex().Lock()
	name, ok := f.glyphNames[code[len(code)-1]]
	f.lazyMutex().Unlock()
	if !ok {
		return nil
	}
	_, err := f.ensureType3CharProcWithError(name)
	return err
}

func (f *Font) WidthCode(code []byte) float64 {
	width, _ := f.WidthCodeWithError(code)
	return width
}

// WidthCodeWithError returns the width for a source code and preserves errors
// from the lazy CMap, ToUnicode, embedded-font, and Type3 parsers.
func (f *Font) WidthCodeWithError(code []byte) (float64, error) {
	if state := f.widthStateLoad(); state != nil {
		return state.widthCode(code), nil
	}
	f.lazyMutex().Lock()
	defer f.lazyMutex().Unlock()
	if state := f.widthStateLoad(); state != nil {
		return state.widthCode(code), nil
	}
	f.ensureCMapLocked()
	if f.cmapErr != nil {
		return 0, f.cmapErr
	}
	f.ensureToUnicodeLocked()
	if f.toUnicodeErr != nil {
		return 0, f.toUnicodeErr
	}
	f.ensureCIDToGIDLocked()
	if f.cidToGIDErr != nil {
		return 0, f.cidToGIDErr
	}
	f.ensureTrueTypeLocked()
	if f.trueTypeErr != nil {
		return 0, f.trueTypeErr
	}
	f.ensureCFFLocked()
	if f.cffErr != nil {
		return 0, f.cffErr
	}
	f.ensureType1EncodingLocked()
	if f.type1Err != nil {
		return 0, f.type1Err
	}
	state := f.publishWidthStateLocked()
	if state != nil {
		return state.widthCode(code), nil
	}
	if f.cid {
		cid := codeNumber(code)
		if f.cmap != nil {
			if mapped, ok := fontdata.MappingValue(f.cmap, code); ok {
				cid = mapped
			}
		}
		if w, ok := f.cidWidths[cid]; ok {
			return w, nil
		}
		if w, ok := f.standardWidthForCodeLocked(code); ok {
			return w, nil
		}
	}
	if len(code) > 0 {
		if f.type3 {
			if name, ok := f.glyphNames[code[len(code)-1]]; ok {
				if err := f.ensureType3CharProcLocked(name); err != nil {
					return 0, err
				}
				if width, found := f.charWidths[name]; found {
					return width, nil
				}
			}
		}
		if f.fontType == "TrueType" {
			if width, found := f.widths[code[len(code)-1]]; found {
				return width, nil
			}
		}
		if f.standardWidths != nil && f.fontType != "TrueType" && !isSubsetFont(f.name) {
			if r, ok := f.encoding[code[len(code)-1]]; ok {
				if width, found := f.standardWidths[r]; found {
					return width, nil
				}
			}
		}
		if f.fontType == "TrueType" {
			if r, ok := f.encoding[code[len(code)-1]]; ok {
				if glyphID, ok := f.glyphUnicodeToID[r]; ok {
					if width, found := f.glyphIDWidths[glyphID]; found {
						return width, nil
					}
				} else if glyphID, ok := f.glyphIDForUnicodeLocked(r); ok {
					if width, found := f.glyphIDWidths[glyphID]; found {
						return width, nil
					}
				}
			}
		}
		return f.widthLocked(code[len(code)-1]), nil
	}
	return f.defaultWidth, nil
}

func (f *Font) glyphIDForUnicodeLocked(r rune) (int, bool) {
	for glyphID, text := range f.glyphIDToUnicode {
		if utf8.RuneCountInString(text) == 1 {
			mapped, _ := utf8.DecodeRuneInString(text)
			if mapped == r {
				return glyphID, true
			}
		}
	}
	return 0, false
}

func (f *Font) standardWidthForCodeLocked(code []byte) (float64, bool) {
	if len(code) == 0 || f.standardWidths == nil {
		return 0, false
	}
	if text, ok := f.codeMap[string(code)]; ok {
		if runes := []rune(text); len(runes) == 1 {
			width, found := f.standardWidths[runes[0]]
			return width, found
		}
	}
	if mapped, found := f.toUnicode[uint16(codeNumber(code))]; found {
		if runes := []rune(mapped); len(runes) == 1 {
			width, ok := f.standardWidths[runes[0]]
			return width, ok
		}
	}
	return 0, false
}

func isSubsetFont(name string) bool {
	return strings.Contains(name, "+")
}

func isPrivateUseText(text string) bool {
	runes := []rune(text)
	if len(runes) == 0 {
		return false
	}
	for _, r := range runes {
		if r < 0xe000 || r > 0xf8ff {
			return false
		}
	}
	return true
}

func (f *Font) hasGlyphIDUnicode(cid int) bool {
	f.ensureTrueType()
	glyphID := f.glyphID(cid)
	f.lazyMutex().Lock()
	defer f.lazyMutex().Unlock()
	text, ok := f.glyphIDToUnicode[glyphID]
	return ok && text != "" && !isPrivateUseText(text)
}

func (f *Font) glyphID(cid int) int {
	f.ensureCIDToGID()
	f.lazyMutex().Lock()
	defer f.lazyMutex().Unlock()
	if glyphID, ok := f.cidToGID[cid]; ok {
		return glyphID
	}
	return cid
}

func (f *Font) glyphIDForCode(code []byte, cid int) int {
	if f == nil || len(code) == 0 {
		return 0
	}
	if f.cid {
		f.ensureCIDToGID()
		f.ensureCMap()
		f.lazyMutex().Lock()
		if f.cmap != nil {
			if mapped, ok := fontdata.MappingValue(f.cmap, code); ok {
				cid = mapped
			}
		}
		f.lazyMutex().Unlock()
		return f.glyphID(cid)
	}
	f.ensureCFF()
	f.lazyMutex().Lock()
	glyphID, ok := f.cffGlyphIDs[code[len(code)-1]]
	f.lazyMutex().Unlock()
	if ok {
		return glyphID
	}
	f.ensureTrueType()
	f.lazyMutex().Lock()
	defer f.lazyMutex().Unlock()
	if r, ok := f.encoding[code[len(code)-1]]; ok {
		if glyphID, found := f.glyphUnicodeToID[r]; found {
			return glyphID
		}
		if glyphID, found := f.glyphIDForUnicodeLocked(r); found {
			return glyphID
		}
	}
	return 0
}

func (f *Font) hasUnicodeMapping(code []byte, cid int) bool {
	f.ensureToUnicode()
	f.lazyMutex().Lock()
	if text, ok := f.codeMap[string(code)]; ok && text != "" {
		f.lazyMutex().Unlock()
		return true
	}
	if text, ok := f.toUnicode[uint16(codeNumber(code))]; ok && text != "" {
		f.lazyMutex().Unlock()
		return true
	}
	f.lazyMutex().Unlock()
	if f.hasGlyphIDUnicode(cid) {
		return true
	}
	f.lazyMutex().Lock()
	defer f.lazyMutex().Unlock()
	text, ok := f.cidToUnicode[cid]
	return ok && text != ""
}

func standardFontName(name string) string {
	if i := strings.LastIndex(name, "+"); i >= 0 {
		name = name[i+1:]
	}
	switch name {
	case "Arial", "ArialMT":
		return "Helvetica"
	case "Arial-BoldMT", "Arial-Bold":
		return "Helvetica-Bold"
	case "TimesNewRoman":
		return "Times-Roman"
	default:
		return name
	}
}

func playaBuiltInType1FontName(name string) (string, bool) {
	if isSubsetFont(name) {
		return "", false
	}
	metricName := name
	switch name {
	case "Arial":
		metricName = "Helvetica"
	case "Arial,Italic":
		metricName = "Helvetica-Oblique"
	case "Arial,Bold":
		metricName = "Helvetica-Bold"
	case "Arial,BoldItalic":
		metricName = "Helvetica-BoldOblique"
	case "CourierNew":
		metricName = "Courier"
	case "CourierNew,Italic":
		metricName = "Courier-Oblique"
	case "CourierNew,Bold":
		metricName = "Courier-Bold"
	case "CourierNew,BoldItalic":
		metricName = "Courier-BoldOblique"
	case "TimesNewRoman":
		metricName = "Times-Roman"
	case "TimesNewRoman,Italic":
		metricName = "Times-Italic"
	case "TimesNewRoman,Bold":
		metricName = "Times-Bold"
	case "TimesNewRoman,BoldItalic":
		metricName = "Times-BoldItalic"
	}
	if _, ok := fontdata.LookupStandardFontMetric(metricName); !ok {
		return "", false
	}
	return metricName, true
}

func applyStandardFontMetrics(f *Font) {
	metric, ok := fontdata.LookupStandardFontMetric(standardFontName(f.name))
	if !ok {
		return
	}
	f.standardWidths = metric.WidthsCopy()
	if f.standardWidths == nil {
		helvetica, ok := fontdata.LookupStandardFontMetric("Helvetica")
		if ok {
			f.standardWidths = helvetica.WidthsCopy()
		}
	}
	f.ascent = metric.Ascent()
	f.descent = metric.Descent()
	f.fontBBox = metric.BBox()
	f.hasFontBBox = true
	f.italicAngle = metric.ItalicAngle()
	if strings.Contains(f.name, "Oblique") {
		f.italicAngle = -12
	} else if strings.Contains(f.name, "Italic") {
		f.italicAngle = -15
	}
	f.defaultWidth = 1000
	if f.name == "ZapfDingbats" {
		for code := 0; code <= 255; code++ {
			if width, ok := fontdata.LookupZapfDingbatsWidth(byte(code)); ok {
				f.widths[byte(code)] = width
			}
		}
	}
}

func applyPlayaBuiltInType1Metrics(f *Font) {
	metric, ok := fontdata.LookupStandardFontMetric(f.name)
	if !ok {
		return
	}
	applyStandardFontMetrics(f)
	if !metric.HasAscent() {
		f.ascent = 880
	}
	if !metric.HasDescent() {
		f.descent = -120
	}
	f.capHeight = metric.CapHeight()
	f.flags = metric.Flags()
	f.hasFlags = true
	f.leading = 0
	f.stemV = 0
	f.italicAngle = metric.ItalicAngle()
}

// ParseToUnicode parses the common bfchar/bfrange subset emitted by PDF
// generators. The parser intentionally accepts whitespace and comments between
// tokens, as required by PDF content syntax.
// ParseToUnicode delegates ToUnicode decoding to the fontdata domain owner.
func ParseToUnicode(data []byte) (map[uint16]string, error) {
	return fontdata.ParseToUnicode(data)
}

// ParseToUnicodeCodes preserves the original source-code byte sequence.
func ParseToUnicodeCodes(data []byte) (map[string]string, error) {
	return fontdata.ParseToUnicodeCodes(data)
}

func utf16Bytes(b []byte) string {
	cacheKey := ""
	if len(b) > 0 && len(b) <= toUnicodeTextCacheEntryLimit {
		cacheKey = string(b)
		toUnicodeTextCache.RLock()
		if value, ok := toUnicodeTextCache.values[cacheKey]; ok {
			toUnicodeTextCache.RUnlock()
			return value
		}
		toUnicodeTextCache.RUnlock()
	}
	if len(b)%2 != 0 {
		b = b[:len(b)-1]
	}
	r := make([]rune, 0, len(b)/2)
	for i := 0; i < len(b); i += 2 {
		u := uint16(b[i])<<8 | uint16(b[i+1])
		if u >= 0xd800 && u <= 0xdbff && i+3 < len(b) {
			v := uint16(b[i+2])<<8 | uint16(b[i+3])
			if v >= 0xdc00 && v <= 0xdfff {
				r = append(r, rune(0x10000+(uint32(u)-0xd800)*0x400+uint32(v)-0xdc00))
				i += 2
				continue
			}
			continue
		}
		if u >= 0xdc00 && u <= 0xdfff {
			continue
		}
		r = append(r, rune(u))
	}
	value := string(r)
	if cacheKey != "" {
		toUnicodeTextCache.Lock()
		if len(toUnicodeTextCache.values) >= toUnicodeTextCacheLimit {
			clear(toUnicodeTextCache.values)
		}
		toUnicodeTextCache.values[cacheKey] = value
		toUnicodeTextCache.Unlock()
	}
	return value
}

// ParseWidth remains a core forwarding function for internal callers. The
// font-domain implementation lives in fontdata.
func ParseWidth(v Object) (float64, error) { return fontdata.ParseWidth(v) }
