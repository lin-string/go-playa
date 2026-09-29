package document

//go:generate python3 ../scripts/generate_cff_resources.py --package fontdata --output-dir ../fontdata

import (
	"encoding/binary"
	"math"
	"strconv"

	"github.com/lin-string/go-playa/fontdata"
	"github.com/lin-string/go-playa/geometry"
)

func glyphIDByName(names []string, wanted string) (int, bool) {
	for gid, name := range names {
		if name == wanted {
			return gid, true
		}
	}
	return 0, false
}

type cffMetadata struct {
	bbox              [4]float64
	hasBBox           bool
	matrix            geometry.Matrix
	hasMatrix         bool
	charset           int
	encoding          int
	charStrings       int
	charstringsData   [][]byte
	privateOffset     int
	privateSize       int
	subrsOffset       int
	fdArray           int
	fdSelect          int
	nominalWidth      float64
	defaultWidth      float64
	hasNominalWidth   bool
	hasDefaultWidth   bool
	isCID             bool
	cff2              bool
	variationStore    int
	variation         *cffVariationStore
	defaultWidthVar   *cffBlendValue
	nominalWidthVar   *cffBlendValue
	variationIndex    int
	variationIndexSet bool
	blendSeen         bool
	pendingBlend      []*cffBlendValue
	localSubrs        [][]byte
	globalSubrs       [][]byte
	glyphText         map[int]string
	glyphNames        []string
	codeNames         map[byte]string
	cidToGID          map[int]int
	glyphWidths       map[int]float64
}

type cffBlendValue struct {
	base    float64
	deltas  []float64
	regions []int
}

func cffWidthScale(matrix geometry.Matrix) (float64, bool) {
	scale := 1000 * matrix[0]
	if math.IsNaN(scale) || math.IsInf(scale, 0) {
		return 0, false
	}
	if scale == 0 {
		scale = 1
	}
	return scale, true
}

func cffScaledWidth(width, scale float64) (float64, bool) {
	value := width * scale
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return 0, false
	}
	return value, true
}

func cffFiniteAdd(left, right float64) (float64, bool) {
	value := left + right
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return 0, false
	}
	return value, true
}

func cffFiniteValues(values ...float64) bool {
	for _, value := range values {
		if math.IsNaN(value) || math.IsInf(value, 0) {
			return false
		}
	}
	return true
}

func cloneCFFBlendValue(value *cffBlendValue) *cffBlendValue {
	if value == nil {
		return nil
	}
	return &cffBlendValue{base: value.base, deltas: cloneCFFFloats(value.deltas), regions: cloneCFFInts(value.regions)}
}

func cloneCFFFloats(value []float64) []float64 {
	if value == nil {
		return nil
	}
	return append(make([]float64, 0, len(value)), value...)
}

func cloneCFFInts(value []int) []int {
	if value == nil {
		return nil
	}
	return append(make([]int, 0, len(value)), value...)
}

func (value *cffBlendValue) evaluate(store *cffVariationStore, coords []float64) float64 {
	if value == nil {
		return 0
	}
	base := value.base
	if math.IsNaN(base) || math.IsInf(base, 0) {
		base = 0
	}
	result := base
	if store == nil {
		return result
	}
	scalars := store.regionScalars(coords)
	for index, region := range value.regions {
		if index < len(value.deltas) && region >= 0 && region < len(scalars) {
			delta := value.deltas[index]
			scalar := scalars[region]
			if math.IsNaN(delta) || math.IsInf(delta, 0) || math.IsNaN(scalar) || math.IsInf(scalar, 0) {
				continue
			}
			result += delta * scalar
		}
	}
	if math.IsNaN(result) || math.IsInf(result, 0) {
		return base
	}
	return result
}

type cffVariationRegion struct {
	start []float64
	peak  []float64
	end   []float64
}

type cffVariationData struct {
	regions []int
	rows    [][]float64
}

type cffVariationStore struct {
	regions []cffVariationRegion
	data    []cffVariationData
}

func (s *cffVariationStore) DataCount() int {
	return len(s.data)
}

func (s *cffVariationStore) Item(index int) ([]int, [][]float64, bool) {
	if index < 0 || index >= len(s.data) {
		return nil, nil, false
	}
	item := s.data[index]
	return item.regions, item.rows, true
}

func (s *cffVariationStore) RegionScalars(coords []float64) []float64 {
	return s.regionScalars(coords)
}

func cffNonNegativeInt(value float64) (int, bool) {
	return fontdata.ParseCFFNonNegativeInt(value)
}

func cffInteger(value float64) (int, bool) {
	return fontdata.ParseCFFInteger(value)
}

func cffSubroutineIndex(value float64, count int) (int, bool) {
	return fontdata.ParseCFFSubroutineIndex(value, count)
}

func cloneCFFVariationStore(store *cffVariationStore) *cffVariationStore {
	if store == nil {
		return nil
	}
	clone := &cffVariationStore{}
	if store.regions != nil {
		clone.regions = make([]cffVariationRegion, len(store.regions))
		for index, region := range store.regions {
			clone.regions[index] = cffVariationRegion{start: cloneCFFFloats(region.start), peak: cloneCFFFloats(region.peak), end: cloneCFFFloats(region.end)}
		}
	}
	if store.data != nil {
		clone.data = make([]cffVariationData, len(store.data))
		for index, item := range store.data {
			clone.data[index].regions = cloneCFFInts(item.regions)
			if item.rows != nil {
				clone.data[index].rows = make([][]float64, len(item.rows))
				for rowIndex, row := range item.rows {
					clone.data[index].rows[rowIndex] = cloneCFFFloats(row)
				}
			}
		}
	}
	return clone
}

func (s *cffVariationStore) regionScalars(coords []float64) []float64 {
	if s == nil {
		return nil
	}
	out := make([]float64, len(s.regions))
	for index, region := range s.regions {
		if len(region.start) != len(region.peak) || len(region.end) != len(region.peak) {
			continue
		}
		scalar := 1.0
		valid := true
		for axis := 0; axis < len(region.peak); axis++ {
			coord := 0.0
			if axis < len(coords) {
				coord = coords[axis]
			}
			start, peak, end := region.start[axis], region.peak[axis], region.end[axis]
			if math.IsNaN(coord) || math.IsInf(coord, 0) || math.IsNaN(start) || math.IsInf(start, 0) || math.IsNaN(peak) || math.IsInf(peak, 0) || math.IsNaN(end) || math.IsInf(end, 0) {
				valid = false
				break
			}
			if coord < start || coord > end {
				scalar = 0
				break
			}
			if coord < peak {
				if peak == start {
					continue
				}
				scalar *= (coord - start) / (peak - start)
			} else if coord > peak {
				if end == peak {
					continue
				}
				scalar *= (end - coord) / (end - peak)
			}
		}
		if valid {
			out[index] = scalar
		}
	}
	return out
}

func parseCFF2VariationStore(data []byte, offset int) *cffVariationStore {
	source := fontdata.ParseCFF2VariationStore(data, offset)
	if source == nil {
		return nil
	}
	store := &cffVariationStore{}
	if count := source.RegionCount(); count > 0 {
		store.regions = make([]cffVariationRegion, count)
		for index := range store.regions {
			start, peak, end, ok := source.Region(index)
			if !ok {
				return nil
			}
			store.regions[index] = cffVariationRegion{start: start, peak: peak, end: end}
		}
	}
	if count := source.DataCount(); count > 0 {
		store.data = make([]cffVariationData, count)
		for index := range store.data {
			regions, rows, ok := source.Item(index)
			if !ok {
				return nil
			}
			store.data[index] = cffVariationData{regions: regions, rows: rows}
		}
	}
	return store
}

func parseCFFMetadata(data []byte) (cffMetadata, bool) {
	if len(data) < 4 {
		return cffMetadata{}, false
	}
	if data[0] == 2 {
		return parseCFF2Metadata(data)
	}
	if data[0] != 1 || int(data[2]) < 4 || int(data[2]) > len(data) {
		return cffMetadata{}, false
	}
	nameIndex, next, ok := fontdata.ParseCFFIndex(data, int(data[2]))
	if !ok || len(nameIndex) == 0 {
		return cffMetadata{}, false
	}
	topIndex, next, ok := fontdata.ParseCFFIndex(data, next)
	if !ok || len(topIndex) == 0 {
		return cffMetadata{}, false
	}
	stringIndex, next, ok := fontdata.ParseCFFIndex(data, next)
	if !ok {
		return cffMetadata{}, false
	}
	globalSubrs, _, ok := fontdata.ParseCFFIndex(data, next)
	if !ok {
		return cffMetadata{}, false
	}
	metadata := cffMetadata{}
	validGlyphData := true
	metadata.globalSubrs = globalSubrs
	parseCFFDict(topIndex[0], &metadata)
	if metadata.charStrings > 0 {
		if glyphs, _, ok := fontdata.ParseCFFIndex(data, metadata.charStrings); ok {
			metadata.charstringsData = glyphs
			parseCFFPrivateDict(data, &metadata)
			if metadata.isCID {
				metadata.cidToGID = parseCFFCharsetCIDs(data, metadata.charset, len(glyphs))
				if len(glyphs) > 1 && metadata.cidToGID == nil {
					validGlyphData = false
				}
				metadata.glyphWidths = parseCFFCIDWidths(data, &metadata)
			} else {
				metadata.glyphNames = parseCFFCharsetNames(data, metadata.charset, len(glyphs), stringIndex)
				if len(glyphs) > 1 && metadata.glyphNames == nil {
					validGlyphData = false
				}
				metadata.glyphText = mapCFFCharsetText(metadata.glyphNames)
				metadata.codeNames = parseCFFEncoding(data, metadata.encoding, metadata.glyphNames, stringIndex)
				if metadata.encoding > 1 && len(glyphs) > 1 && metadata.codeNames == nil {
					validGlyphData = false
				}
			}
		}
	}
	if !validGlyphData {
		return metadata, false
	}
	validGlyphData = metadata.cff2
	if !metadata.cff2 {
		validGlyphData = len(metadata.charstringsData) == 1
		if metadata.isCID {
			validGlyphData = validGlyphData || len(metadata.cidToGID) > 0
		} else {
			validGlyphData = validGlyphData || len(metadata.glyphNames) > 0
		}
	}
	return metadata, len(metadata.charstringsData) > 0 && validGlyphData || metadata.hasBBox || metadata.hasMatrix || len(metadata.glyphText) > 0 || len(metadata.cidToGID) > 0
}

// CFF2 removes the CFF1 Name, String, and Encoding indexes. Its header stores
// the Top DICT length directly, while the Top DICT retains the shared geometry
// operators used by PDF font fallback.
func parseCFF2Metadata(data []byte) (cffMetadata, bool) {
	if len(data) < 5 || data[2] < 5 {
		return cffMetadata{}, false
	}
	hdrSize := int(data[2])
	topLength := int(binary.BigEndian.Uint16(data[3:5]))
	if hdrSize > len(data) || topLength < 0 || hdrSize+topLength > len(data) {
		return cffMetadata{}, false
	}
	metadata := cffMetadata{}
	metadata.cff2 = true
	parseCFFDict(data[hdrSize:hdrSize+topLength], &metadata)
	if metadata.variationStore > 0 {
		metadata.variation = parseCFF2VariationStore(data, metadata.variationStore)
	}
	metadata.globalSubrs, _, _ = fontdata.ParseCFF2Index(data, hdrSize+topLength)
	if metadata.charStrings > 0 {
		if glyphs, _, ok := fontdata.ParseCFF2Index(data, metadata.charStrings); ok {
			metadata.charstringsData = glyphs
			metadata.isCID = metadata.fdArray > 0 && metadata.fdSelect > 0
			if metadata.isCID {
				metadata.glyphWidths = parseCFFCIDWidths(data, &metadata)
			} else {
				parseCFFPrivateDict(data, &metadata)
				metadata.glyphWidths = parseCFFGlyphWidths(&metadata)
			}
		}
	}
	return metadata, len(metadata.charstringsData) > 0 || metadata.hasBBox || metadata.hasMatrix || len(metadata.glyphWidths) > 0
}

func parseCFFDict(data []byte, metadata *cffMetadata) {
	entries, _ := fontdata.ParseCFFDict(data, metadata.cff2)
	for _, entry := range entries {
		operands := entry.OperandsCopy()
		switch entry.Operator() {
		case 5:
			if len(operands) == 4 {
				copy(metadata.bbox[:], operands)
				if metadata.bbox[0] > metadata.bbox[2] {
					metadata.bbox[0], metadata.bbox[2] = metadata.bbox[2], metadata.bbox[0]
				}
				if metadata.bbox[1] > metadata.bbox[3] {
					metadata.bbox[1], metadata.bbox[3] = metadata.bbox[3], metadata.bbox[1]
				}
				metadata.hasBBox = true
			}
		case 1207:
			if len(operands) == 6 {
				copy(metadata.matrix[:], operands)
				metadata.hasMatrix = true
			}
		case 15:
			if len(operands) == 1 {
				if value, ok := cffNonNegativeInt(operands[0]); ok {
					metadata.charset = value
				}
			}
		case 16:
			if len(operands) == 1 {
				if value, ok := cffNonNegativeInt(operands[0]); ok {
					metadata.encoding = value
				}
			}
		case 17:
			if len(operands) == 1 {
				if value, ok := cffNonNegativeInt(operands[0]); ok {
					metadata.charStrings = value
				}
			}
		case 18:
			if len(operands) == 2 {
				privateSize, sizeOK := cffNonNegativeInt(operands[0])
				privateOffset, offsetOK := cffNonNegativeInt(operands[1])
				if sizeOK && offsetOK {
					metadata.privateSize = privateSize
					metadata.privateOffset = privateOffset
				}
			}
		case 1236:
			if len(operands) == 1 {
				if value, ok := cffNonNegativeInt(operands[0]); ok {
					metadata.fdArray = value
				}
			}
		case 1237:
			if len(operands) == 1 {
				if value, ok := cffNonNegativeInt(operands[0]); ok {
					metadata.fdSelect = value
				}
			}
		case 1230:
			metadata.isCID = len(operands) == 3
		case 24:
			if len(operands) == 1 {
				if value, ok := cffNonNegativeInt(operands[0]); ok {
					metadata.variationStore = value
				}
			}
		}
	}
}

func parseCFFCIDWidths(data []byte, metadata *cffMetadata) map[int]float64 {
	if metadata.fdArray <= 0 || metadata.fdSelect <= 0 || len(metadata.charstringsData) == 0 {
		return nil
	}
	var fds [][]byte
	var ok bool
	if metadata.cff2 {
		fds, _, ok = fontdata.ParseCFF2Index(data, metadata.fdArray)
	} else {
		fds, _, ok = fontdata.ParseCFFIndex(data, metadata.fdArray)
	}
	if !ok {
		return nil
	}
	fdByGlyph := fontdata.ParseCFFFDSelect(data, metadata.fdSelect, len(metadata.charstringsData))
	if !cffFDSelectFitsArray(fdByGlyph, len(fds)) {
		return nil
	}
	private := make([]cffMetadata, len(fds))
	for i, fd := range fds {
		private[i].cff2 = metadata.cff2
		private[i].variation = metadata.variation
		parseCFFDict(fd, &private[i])
		parseCFFPrivateDict(data, &private[i])
	}
	widths := make(map[int]float64, len(fdByGlyph))
	for gid, fd := range fdByGlyph {
		if fd < 0 || fd >= len(private) {
			continue
		}
		entry := private[fd]
		defaultWidth := 0.0
		if entry.hasDefaultWidth {
			defaultWidth = entry.defaultWidth
		}
		var width float64
		var ok bool
		if metadata.cff2 {
			width, ok = parseCFF2CharStringWidthWithGlobals(metadata.charstringsData[gid], entry.nominalWidth, defaultWidth, entry.localSubrs, metadata.globalSubrs, metadata.variation, nil, entry.variationIndex)
		} else {
			width, ok = parseCFFCharStringWidthWithGlobals(metadata.charstringsData[gid], entry.nominalWidth, defaultWidth, entry.localSubrs, metadata.globalSubrs)
		}
		if ok {
			widths[gid] = width
		}
	}
	return widths
}

func parseCFFGlyphWidths(metadata *cffMetadata) map[int]float64 {
	if len(metadata.charstringsData) == 0 {
		return nil
	}
	widths := make(map[int]float64, len(metadata.charstringsData))
	nominalWidth := metadata.nominalWidth
	defaultWidth := 0.0
	if metadata.hasDefaultWidth {
		defaultWidth = metadata.defaultWidth
	}
	if metadata.cff2 {
		if metadata.nominalWidthVar != nil {
			nominalWidth = metadata.nominalWidthVar.evaluate(metadata.variation, nil)
		}
		if metadata.defaultWidthVar != nil {
			defaultWidth = metadata.defaultWidthVar.evaluate(metadata.variation, nil)
		}
	}
	for glyphID, charString := range metadata.charstringsData {
		var width float64
		var ok bool
		if metadata.cff2 {
			width, ok = parseCFF2CharStringWidthWithGlobals(charString, nominalWidth, defaultWidth, metadata.localSubrs, metadata.globalSubrs, metadata.variation, nil, metadata.variationIndex)
		} else {
			width, ok = parseCFFCharStringWidthWithGlobals(charString, nominalWidth, defaultWidth, metadata.localSubrs, metadata.globalSubrs)
		}
		if ok {
			widths[glyphID] = width
		}
	}
	return widths
}

func cffFDSelectFitsArray(fdByGlyph map[int]int, fdCount int) bool {
	if len(fdByGlyph) == 0 || fdCount <= 0 {
		return false
	}
	for _, fd := range fdByGlyph {
		if fd < 0 || fd >= fdCount {
			return false
		}
	}
	return true
}

func parseCFFPrivateDict(data []byte, metadata *cffMetadata) {
	if metadata.privateSize <= 0 || metadata.privateOffset < 0 || metadata.privateOffset > len(data) || metadata.privateSize > len(data)-metadata.privateOffset {
		return
	}
	private := data[metadata.privateOffset : metadata.privateOffset+metadata.privateSize]
	entries, _ := fontdata.ParseCFFDict(private, metadata.cff2)
	var operands []float64
	carryOperands := false
	for _, entry := range entries {
		rawOperands := entry.OperandsCopy()
		if !carryOperands {
			operands = rawOperands
		} else {
			operands = append(operands, rawOperands...)
		}
		op := entry.Operator()
		switch op {
		case 20:
			if len(operands) == 1 {
				metadata.defaultWidth = operands[0]
				metadata.hasDefaultWidth = true
				if len(metadata.pendingBlend) == 1 {
					metadata.defaultWidthVar = cloneCFFBlendValue(metadata.pendingBlend[0])
				}
			}
		case 21:
			if len(operands) == 1 {
				metadata.nominalWidth = operands[0]
				metadata.hasNominalWidth = true
				if len(metadata.pendingBlend) == 1 {
					metadata.nominalWidthVar = cloneCFFBlendValue(metadata.pendingBlend[0])
				}
			}
		case 19:
			if len(operands) == 1 {
				if value, ok := cffNonNegativeInt(operands[0]); ok {
					metadata.subrsOffset = value
				}
			}
		case 1222:
			if !metadata.cff2 && len(operands) == 1 {
				if value, ok := cffNonNegativeInt(operands[0]); ok {
					metadata.variationIndex = value
					metadata.variationIndexSet = true
				}
			}
		case 22:
			if metadata.cff2 && !metadata.variationIndexSet && !metadata.blendSeen && len(operands) == 1 {
				if value, ok := cffNonNegativeInt(operands[0]); ok {
					metadata.variationIndex = value
					metadata.variationIndexSet = true
				}
			}
		case 23:
			if metadata.cff2 {
				var ok bool
				operands, metadata.pendingBlend, ok = cffBlendDictValuesAt(operands, metadata.variation, metadata.variationIndex, nil)
				if !ok {
					operands = nil
					metadata.pendingBlend = nil
				} else {
					metadata.blendSeen = true
				}
			}
		}
		if op != 23 {
			metadata.pendingBlend = nil
			carryOperands = false
		} else {
			carryOperands = metadata.pendingBlend != nil
		}
	}
	if metadata.subrsOffset <= 0 {
		return
	}
	if metadata.subrsOffset > len(data)-metadata.privateOffset {
		return
	}
	offset := metadata.privateOffset + metadata.subrsOffset
	if metadata.cff2 {
		metadata.localSubrs, _, _ = fontdata.ParseCFF2Index(data, offset)
	} else {
		metadata.localSubrs, _, _ = fontdata.ParseCFFIndex(data, offset)
	}
}

// parseCFFCharStringWidth reads the optional Type2 width prefix. Width is
// present when the first operator receives one extra operand; all other
// CharString drawing instructions are irrelevant to width extraction.
func parseCFFCharStringWidth(data []byte, nominal, defaultWidth float64) (float64, bool) {
	return fontdata.ParseCFFCharStringWidth(data, nominal, defaultWidth, nil, nil)
}

func parseCFFCharStringWidthWithSubrs(data []byte, nominal, defaultWidth float64, subrs [][]byte) (float64, bool) {
	return fontdata.ParseCFFCharStringWidth(data, nominal, defaultWidth, subrs, nil)
}

func parseCFFCharStringWidthWithGlobals(data []byte, nominal, defaultWidth float64, subrs, globals [][]byte) (float64, bool) {
	return fontdata.ParseCFFCharStringWidth(data, nominal, defaultWidth, subrs, globals)
}

func parseCFF2CharStringWidthWithGlobals(data []byte, nominal, defaultWidth float64, subrs, globals [][]byte, variation *cffVariationStore, coords []float64, variationIndex int) (float64, bool) {
	var reader fontdata.CFF2VariationStoreReader
	if variation != nil {
		reader = variation
	}
	return fontdata.ParseCFF2CharStringWidth(data, nominal, defaultWidth, subrs, globals, reader, coords, variationIndex)
}

// cffCharStringOps converts the drawing subset of a Type2 CharString into
// the existing path interpreter's operations. Width and hint operators are
// consumed but do not produce geometry.
func cffCharStringOps(data []byte, local, global [][]byte) ([]ContentOp, bool) {
	state := cffPathState{local: local, global: global}
	return state.finish(data)
}

func cff2CharStringOps(data []byte, local, global [][]byte, variation *cffVariationStore, coords []float64, variationIndex int) ([]ContentOp, bool) {
	state := cffPathState{local: local, global: global, cff2: true, variation: variation, coords: coords, vsindex: variationIndex}
	return state.finish(data)
}

func (s *cffPathState) finish(data []byte) ([]ContentOp, bool) {
	if !s.run(data, 0) {
		return nil, false
	}
	if len(s.operands) != 0 {
		return nil, false
	}
	if s.drawn {
		s.ops = append(s.ops, newContentOpBorrowed("h", nil, 0), newContentOpBorrowed("f", nil, 0))
	}
	return s.ops, true
}

type cffPathState struct {
	ops                   []ContentOp
	operands              []float64
	randomState           uint32
	x, y                  float64
	stems                 int
	drawn                 bool
	invalid               bool
	local                 [][]byte
	global                [][]byte
	transient             map[int]float64
	cff2                  bool
	variation             *cffVariationStore
	coords                []float64
	variationScalarsCache []float64
	vsindex               int
	vsindexSeen           bool
	variationBlendSeen    bool
	widthSeen             bool
	charstrings           [][]byte
	glyphIDs              map[byte]int
}

func (s *cffPathState) variationScalars() []float64 {
	if s.variationScalarsCache == nil && s.variation != nil {
		s.variationScalarsCache = s.variation.regionScalars(s.coords)
	}
	return s.variationScalarsCache
}

func (f *Font) cffCharStringOps(code []byte) ([]ContentOp, bool) {
	ops, ok, _ := f.cffCharStringOpsWithGIDError(code, 0)
	return ops, ok
}

func (f *Font) cffCharStringOpsWithGIDError(code []byte, gidHint int) ([]ContentOp, bool, error) {
	if f == nil || len(code) == 0 {
		return nil, false, nil
	}
	f.lazyMutex().Lock()
	f.ensureCFFLocked()
	if f.cffErr != nil {
		f.lazyMutex().Unlock()
		return nil, false, f.cffErr
	}
	if len(f.cffCharstrings) == 0 {
		f.lazyMutex().Unlock()
		return nil, false, nil
	}
	key := strconv.Itoa(len(code)) + ":" + string(code)
	if f.cff2 {
		key += "|"
		for _, coord := range f.variationCoords {
			key += strconv.FormatFloat(coord, 'g', -1, 64) + ";"
		}
	}
	if gidHint > 0 {
		key += "#gid=" + strconv.Itoa(gidHint)
	}
	if cached, ok := f.cffPathCache[key]; ok {
		ops := cloneContentOps(cached)
		f.lazyMutex().Unlock()
		return ops, true, nil
	}
	if gidHint <= 0 && f.cid {
		f.ensureCMapLocked()
		if f.cmapErr != nil {
			f.lazyMutex().Unlock()
			return nil, false, f.cmapErr
		}
	}
	gid := 0
	if gidHint > 0 {
		gid = gidHint
	} else if f.cid {
		f.ensureCIDToGIDLocked()
		cid := codeNumber(code)
		if f.cmap != nil {
			if mapped, ok := fontdata.MappingValue(f.cmap, code); ok {
				cid = mapped
			}
		}
		gid = cid
		if mapped, ok := f.cidToGID[cid]; ok {
			gid = mapped
		}
	} else {
		var ok bool
		gid, ok = f.cffGlyphIDs[code[len(code)-1]]
		if !ok {
			f.lazyMutex().Unlock()
			return nil, false, nil
		}
	}
	if gid < 0 || gid >= len(f.cffCharstrings) {
		f.lazyMutex().Unlock()
		return nil, false, nil
	}
	local := f.cffLocalSubrs
	variationIndex := f.cffVariationIndex
	if fd, ok := f.cffFDByGlyph[gid]; ok {
		local = f.cffFDLocalSubrs[fd]
		if index, exists := f.cffFDVariationIndex[fd]; exists {
			variationIndex = index
		}
	}
	charString := f.cffCharstrings[gid]
	global := f.cffGlobalSubrs
	variation := f.cffVariationStore
	coords := cloneFontFloats(f.variationCoords)
	cff2 := f.cff2
	charstrings := f.cffCharstrings
	glyphIDs := f.cffGlyphIDs
	f.lazyMutex().Unlock()

	var (
		ops []ContentOp
		ok  bool
	)
	if cff2 {
		ops, ok = cff2CharStringOps(charString, local, global, variation, coords, variationIndex)
	} else {
		state := cffPathState{local: local, global: global, charstrings: charstrings, glyphIDs: glyphIDs}
		ops, ok = state.finish(charString)
	}
	if !ok {
		return nil, false, nil
	}

	f.lazyMutex().Lock()
	defer f.lazyMutex().Unlock()
	if f.cffPathCache == nil {
		f.cffPathCache = map[string][]ContentOp{}
	}
	entrySize := addCacheSize(len(key)+64, contentOpsCacheSize(ops))
	if cacheFits(f.cffPathCacheBytes, entrySize, f.cffPathCacheBudget()) {
		f.cffPathCache[key] = cloneContentOps(ops)
		f.cffPathCacheBytes += entrySize
	}
	return cloneContentOps(ops), true, nil
}

func (f *Font) recomputeCFFWidths() {
	if f == nil || !f.cff2 || f.cid || len(f.cffCharstrings) == 0 || len(f.cffGlyphIDs) == 0 {
		return
	}
	scale, ok := cffWidthScale(f.fontMatrix)
	if !ok {
		return
	}
	defaultWidth := f.defaultWidth / scale
	if f.cffDefaultWidthVar != nil {
		defaultWidth = f.cffDefaultWidthVar.evaluate(f.cffVariationStore, f.variationCoords)
		if value, ok := cffScaledWidth(defaultWidth, scale); ok {
			f.defaultWidth = value
		}
	}
	nominalWidth := 0.0
	if f.cffNominalWidthVar != nil {
		nominalWidth = f.cffNominalWidthVar.evaluate(f.cffVariationStore, f.variationCoords)
	}
	for code, glyphID := range f.cffGlyphIDs {
		if glyphID < 0 || glyphID >= len(f.cffCharstrings) {
			continue
		}
		var width float64
		var ok bool
		if f.cff2 {
			width, ok = parseCFF2CharStringWidthWithGlobals(f.cffCharstrings[glyphID], nominalWidth, defaultWidth, f.cffLocalSubrs, f.cffGlobalSubrs, f.cffVariationStore, f.variationCoords, f.cffVariationIndex)
		} else {
			width, ok = parseCFFCharStringWidthWithGlobals(f.cffCharstrings[glyphID], nominalWidth, defaultWidth, f.cffLocalSubrs, f.cffGlobalSubrs)
		}
		if !ok {
			continue
		}
		if value, ok := cffScaledWidth(width, scale); ok {
			f.widths[code] = value
			if f.glyphIDWidths != nil {
				f.glyphIDWidths[glyphID] = value
			}
		}
	}
}

func parseCFFCIDSubroutines(data []byte, metadata *cffMetadata) (map[int]int, map[int][][]byte, map[int]int) {
	if metadata == nil || metadata.fdArray <= 0 || metadata.fdSelect <= 0 || len(metadata.charstringsData) == 0 {
		return nil, nil, nil
	}
	var fds [][]byte
	var ok bool
	if metadata.cff2 {
		fds, _, ok = fontdata.ParseCFF2Index(data, metadata.fdArray)
	} else {
		fds, _, ok = fontdata.ParseCFFIndex(data, metadata.fdArray)
	}
	if !ok {
		return nil, nil, nil
	}
	fdByGlyph := fontdata.ParseCFFFDSelect(data, metadata.fdSelect, len(metadata.charstringsData))
	if !cffFDSelectFitsArray(fdByGlyph, len(fds)) {
		return nil, nil, nil
	}
	localByFD := make(map[int][][]byte, len(fds))
	variationByFD := make(map[int]int, len(fds))
	for fd, dict := range fds {
		private := cffMetadata{cff2: metadata.cff2}
		private.variation = metadata.variation
		parseCFFDict(dict, &private)
		parseCFFPrivateDict(data, &private)
		localByFD[fd] = cloneByteSlices(private.localSubrs)
		if metadata.cff2 {
			variationByFD[fd] = private.variationIndex
		}
	}
	return fdByGlyph, localByFD, variationByFD
}

func (s *cffPathState) run(data []byte, depth int) bool {
	depthLimit := 16
	if s.cff2 {
		depthLimit = 10
		if len(data) > 65535 {
			return false
		}
	}
	if depth > depthLimit {
		return false
	}
	for at := 0; at < len(data); {
		if value, next, ok := fontdata.ParseCFFNumber(data, at); ok {
			s.operands = append(s.operands, value)
			limit := 48
			if s.cff2 {
				limit = 513
			}
			if len(s.operands) > limit {
				return false
			}
			at = next
			continue
		}
		op, next, ok := fontdata.ParseCFFCharStringOperator(data, at, s.cff2)
		if !ok {
			return false
		}
		at = next
		if !s.operator(op, data, &at, depth) {
			return false
		}
		if !s.finite() {
			return false
		}
	}
	return true
}

func (s *cffPathState) finite() bool {
	return !s.invalid && cffFiniteValues(s.x, s.y)
}

func (s *cffPathState) operator(op int, data []byte, at *int, depth int) bool {
	args := s.operands
	s.operands = nil
	for _, value := range args {
		if math.IsNaN(value) || math.IsInf(value, 0) {
			return false
		}
	}
	if s.cff2 && op >= 1200 && (op < 1234 || op > 1237) {
		return false
	}
	popWidth := func() {
		if !s.widthSeen {
			if s.cff2 {
				s.widthSeen = true
				return
			}
			if len(args)%2 == 1 {
				args = args[1:]
			}
			s.widthSeen = true
		}
	}
	point := func(x, y float64) {
		if !cffFiniteValues(x, y) {
			s.invalid = true
		}
		s.x, s.y = x, y
	}
	line := func(x, y float64) {
		if !cffFiniteValues(x, y) {
			s.invalid = true
			return
		}
		s.ops = append(s.ops, cffOp("l", x, y))
		point(x, y)
		s.drawn = true
	}
	curve := func(values []float64) {
		if len(values) != 6 {
			return
		}
		c1x, c1y := s.x+values[0], s.y+values[1]
		c2x, c2y := c1x+values[2], c1y+values[3]
		x, y := c2x+values[4], c2y+values[5]
		if !cffFiniteValues(c1x, c1y, c2x, c2y, x, y) {
			s.invalid = true
			return
		}
		s.ops = append(s.ops, cffOp("c", c1x, c1y, c2x, c2y, x, y))
		point(x, y)
		s.drawn = true
	}
	switch op {
	case 15:
		if !s.cff2 || len(args) != 1 || s.variation == nil || s.vsindexSeen || s.variationBlendSeen {
			return false
		}
		index, indexOK := cffNonNegativeInt(args[0])
		if !indexOK || index >= len(s.variation.data) {
			return false
		}
		s.vsindex = index
		s.vsindexSeen = true
	case 16:
		if !s.cff2 || len(args) == 0 || s.variation == nil || s.vsindex < 0 || s.vsindex >= len(s.variation.data) {
			return false
		}
		nValue := args[len(args)-1]
		if math.IsNaN(nValue) || math.IsInf(nValue, 0) || nValue < 1 || nValue != math.Trunc(nValue) || nValue > float64(len(args)-1) {
			return false
		}
		n := int(nValue)
		regions := 0
		var scalars []float64
		if s.variation != nil && s.vsindex >= 0 && s.vsindex < len(s.variation.data) {
			item := s.variation.data[s.vsindex]
			regions = len(item.regions)
			scalars = s.variationScalars()
		}
		if regions < 0 || n > (len(args)-1)/(regions+1) {
			return false
		}
		if len(args) != n*(regions+1)+1 {
			return false
		}
		base := args[:n]
		blended := append([]float64(nil), base...)
		if regions > 0 {
			item := s.variation.data[s.vsindex]
			for valueIndex := 0; valueIndex < n; valueIndex++ {
				for regionIndex, region := range item.regions {
					if region < 0 || region >= len(scalars) || n+valueIndex*regions+regionIndex >= len(args)-1 {
						return false
					}
					blended[valueIndex] += args[n+valueIndex*regions+regionIndex] * scalars[region]
				}
			}
		}
		for _, value := range blended {
			if math.IsNaN(value) || math.IsInf(value, 0) {
				return false
			}
		}
		s.variationBlendSeen = true
		s.operands = blended
	case 1, 3, 18, 23:
		popWidth()
		if len(args) < 2 || len(args)%2 != 0 {
			return false
		}
		s.stems += len(args) / 2
		if s.cff2 && s.stems > 96 {
			return false
		}
	case 19, 20:
		popWidth()
		if len(args) == 0 && s.stems == 0 {
			return false
		}
		if s.cff2 && len(args)%2 != 0 {
			return false
		}
		s.stems += len(args) / 2
		if s.cff2 && s.stems > 96 {
			return false
		}
		maskBytes := (s.stems + 7) / 8
		if *at+maskBytes > len(data) {
			return false
		}
		*at += maskBytes
	case 4:
		popWidth()
		if len(args) != 1 {
			return false
		}
		point(s.x, s.y+args[0])
		s.ops = append(s.ops, cffOp("m", s.x, s.y))
		s.drawn = true
	case 21:
		popWidth()
		if len(args) != 2 {
			return false
		}
		point(s.x+args[0], s.y+args[1])
		s.ops = append(s.ops, cffOp("m", s.x, s.y))
		s.drawn = true
	case 22:
		popWidth()
		if len(args) != 1 {
			return false
		}
		point(s.x+args[0], s.y)
		s.ops = append(s.ops, cffOp("m", s.x, s.y))
		s.drawn = true
	case 5:
		if len(args) < 2 || len(args)%2 != 0 {
			return false
		}
		for i := 0; i < len(args); i += 2 {
			line(s.x+args[i], s.y+args[i+1])
		}
	case 6:
		if len(args) == 0 {
			return false
		}
		for i, value := range args {
			if i%2 == 0 {
				line(s.x+value, s.y)
			} else {
				line(s.x, s.y+value)
			}
		}
	case 7:
		if len(args) == 0 {
			return false
		}
		for i, value := range args {
			if i%2 == 0 {
				line(s.x, s.y+value)
			} else {
				line(s.x+value, s.y)
			}
		}
	case 8:
		if len(args) < 6 || len(args)%6 != 0 {
			return false
		}
		for i := 0; i < len(args); i += 6 {
			curve(args[i : i+6])
		}
	case 24:
		if len(args) < 8 || (len(args)-2)%6 != 0 {
			return false
		}
		for i := 0; i < len(args)-2; i += 6 {
			curve(args[i : i+6])
		}
		line(s.x+args[len(args)-2], s.y+args[len(args)-1])
	case 25:
		if len(args) < 8 || (len(args)-6)%2 != 0 {
			return false
		}
		for i := 0; i < len(args)-6; i += 2 {
			line(s.x+args[i], s.y+args[i+1])
		}
		curve(args[len(args)-6:])
	case 26:
		if len(args) < 4 {
			return false
		}
		start := 0
		if len(args)%4 == 1 {
			line(s.x+args[0], s.y)
			start = 1
		}
		if (len(args)-start)%4 != 0 {
			return false
		}
		for i := start; i < len(args); i += 4 {
			curve([]float64{0, args[i], args[i+1], args[i+2], 0, args[i+3]})
		}
	case 27:
		if len(args) < 4 {
			return false
		}
		start := 0
		if len(args)%4 == 1 {
			line(s.x, s.y+args[0])
			start = 1
		}
		if (len(args)-start)%4 != 0 {
			return false
		}
		for i := start; i < len(args); i += 4 {
			curve([]float64{args[i], 0, args[i+1], args[i+2], args[i+3], 0})
		}
	case 30, 31:
		if len(args) < 4 {
			return false
		}
		vertical := op == 30
		extra := 0.0
		hasExtra := len(args)%4 == 1
		if hasExtra {
			extra = args[len(args)-1]
			args = args[:len(args)-1]
		}
		for len(args) >= 4 {
			values := make([]float64, 6)
			if vertical {
				values[1], values[2], values[3], values[4] = args[0], args[1], args[2], args[3]
			} else {
				values[0], values[2], values[3], values[5] = args[0], args[1], args[2], args[3]
			}
			if hasExtra && len(args) == 4 {
				if op == 30 {
					values[5] = extra
				} else {
					values[4] = extra
				}
			}
			curve(values)
			args = args[4:]
			vertical = !vertical
		}
		if len(args) != 0 {
			return false
		}
	case 10, 29:
		if len(args) != 1 {
			return false
		}
		subs := s.local
		if op == 29 {
			subs = s.global
		}
		if len(subs) == 0 {
			return true
		}
		index, ok := cffSubroutineIndex(args[0], len(subs))
		if !ok {
			return true
		}
		return s.run(subs[index], depth+1)
	case 11:
		if s.cff2 {
			return false
		}
		return true
	case 1234, 1235, 1236, 1237:
		return s.flexOperator(op, args)
	case 1210, 1211, 1212, 1224:
		if len(args) < 2 {
			return false
		}
		left, right := args[len(args)-2], args[len(args)-1]
		var value float64
		switch op {
		case 1210:
			value = left + right
		case 1211:
			value = left - right
		case 1212:
			if right == 0 {
				return false
			}
			value = left / right
		case 1224:
			value = left * right
		}
		if math.IsNaN(value) || math.IsInf(value, 0) {
			return false
		}
		s.operands = append(args[:len(args)-2], value)
	case 1209, 1214, 1226:
		if len(args) < 1 {
			return false
		}
		value := args[len(args)-1]
		if op == 1209 && value < 0 {
			value = -value
		} else if op == 1214 {
			value = -value
		} else if value < 0 {
			return false
		} else {
			value = math.Sqrt(value)
		}
		if math.IsNaN(value) || math.IsInf(value, 0) {
			return false
		}
		s.operands = append(args[:len(args)-1], value)
	case 1215:
		if s.cff2 {
			if len(args) != 1 {
				return false
			}
			break
		}
		if len(args) < 2 {
			return false
		}
		value := 0.0
		if args[len(args)-2] == args[len(args)-1] {
			value = 1
		}
		s.operands = append(args[:len(args)-2], value)
	case 1222:
		if len(args) < 4 {
			return false
		}
		value := args[len(args)-1]
		if args[len(args)-4] <= args[len(args)-3] {
			value = args[len(args)-2]
		}
		s.operands = append(args[:len(args)-4], value)
	case 1223:
		if len(args) != 0 {
			return false
		}
		// Type 2 random is implementation-seeded. Keep the seed local to
		// this CharString so glyph extraction remains reproducible and safe
		// when several pages are interpreted concurrently.
		if s.randomState == 0 {
			s.randomState = 0x9e3779b9
		}
		s.randomState ^= s.randomState << 13
		s.randomState ^= s.randomState >> 17
		s.randomState ^= s.randomState << 5
		s.operands = []float64{float64(s.randomState) / float64(uint64(1)<<32)}
	case 1218:
		if len(args) < 1 {
			return false
		}
		s.operands = append([]float64(nil), args[:len(args)-1]...)
	case 1227:
		if len(args) < 1 {
			return false
		}
		s.operands = append(args, args[len(args)-1])
	case 1228:
		if len(args) < 2 {
			return false
		}
		s.operands = append([]float64(nil), args...)
		s.operands[len(args)-1], s.operands[len(args)-2] = args[len(args)-2], args[len(args)-1]
	case 1229:
		if len(args) < 1 {
			return false
		}
		index, ok := cffNonNegativeInt(args[len(args)-1])
		if !ok || index >= len(args)-1 {
			return false
		}
		s.operands = append(args[:len(args)-1], args[len(args)-2-index])
	case 1203, 1204, 1205:
		if op == 1205 {
			if len(args) < 1 {
				return false
			}
			value := 0.0
			if args[len(args)-1] == 0 {
				value = 1
			}
			s.operands = append(args[:len(args)-1], value)
		} else {
			if len(args) < 2 {
				return false
			}
			value := 0.0
			if (op == 1203 && args[len(args)-2] != 0 && args[len(args)-1] != 0) || (op == 1204 && (args[len(args)-2] != 0 || args[len(args)-1] != 0)) {
				value = 1
			}
			s.operands = append(args[:len(args)-2], value)
		}
	case 1220:
		if len(args) < 2 {
			return false
		}
		if s.transient == nil {
			s.transient = map[int]float64{}
		}
		index, ok := cffNonNegativeInt(args[len(args)-1])
		if !ok || index >= 32 {
			return false
		}
		s.transient[index] = args[len(args)-2]
	case 1221:
		if len(args) < 1 || s.transient == nil {
			return false
		}
		index, ok := cffNonNegativeInt(args[len(args)-1])
		if !ok || index >= 32 {
			return false
		}
		value, ok := s.transient[index]
		if !ok {
			return false
		}
		s.operands = append(args[:len(args)-1], value)
	case 1230:
		if len(args) < 2 {
			return false
		}
		n, nOK := cffNonNegativeInt(args[len(args)-2])
		shift, shiftOK := cffInteger(args[len(args)-1])
		if !nOK || !shiftOK || n > len(args)-2 {
			return false
		}
		values := append([]float64(nil), args[:len(args)-2]...)
		if n > 0 {
			shift %= n
			if shift < 0 {
				shift += n
			}
			start := len(values) - n
			rotated := append([]float64(nil), values[start+shift:]...)
			rotated = append(rotated, values[start:start+shift]...)
			copy(values[start:], rotated)
		}
		s.operands = values
	case 1216: // blend: retain the default-design values for now.
		if len(args) == 0 {
			return false
		}
		nValue := args[len(args)-1]
		n, ok := cffNonNegativeInt(nValue)
		if !ok || n < 1 || n > len(args)-1 {
			return false
		}
		s.operands = append(s.operands, args[:n]...)
	case 14:
		if s.cff2 {
			return false
		}
		if len(args) == 4 {
			return s.seac(args, depth)
		}
		return len(args) == 0 || len(args) == 1 || len(args) == 5
	default:
		// Other escaped operators are hinting or arithmetic operators. They
		// do not contribute to a static outline in this first execution pass.
		if s.cff2 {
			return false
		}
		if op > 1237 {
			return false
		}
		if op >= 1200 {
			return true
		}
		return false
	}
	return true
}

// seac expands the deprecated Type2 composite-glyph form of endchar. It is
// still emitted by older CFF producers and uses character codes to select the
// base and accent charstrings.
func (s *cffPathState) seac(args []float64, depth int) bool {
	if len(s.charstrings) == 0 || s.glyphIDs == nil {
		return false
	}
	toCode := func(value float64) (byte, bool) {
		if value < 0 || value > 255 || value != math.Trunc(value) {
			return 0, false
		}
		return byte(value), true
	}
	adx, ady := args[0], args[1]
	bchar, ok := toCode(args[2])
	if !ok {
		return false
	}
	achar, ok := toCode(args[3])
	if !ok {
		return false
	}
	base, baseOK := s.glyphIDs[bchar]
	accent, accentOK := s.glyphIDs[achar]
	if !baseOK || !accentOK || base < 0 || accent < 0 || base >= len(s.charstrings) || accent >= len(s.charstrings) {
		return false
	}
	child := func(gid int) (cffPathState, bool) {
		state := cffPathState{
			local: s.local, global: s.global,
			charstrings: s.charstrings, glyphIDs: s.glyphIDs,
		}
		if !state.run(s.charstrings[gid], depth+1) || len(state.operands) != 0 {
			return cffPathState{}, false
		}
		return state, true
	}
	baseState, ok := child(base)
	if !ok {
		return false
	}
	accentState, ok := child(accent)
	if !ok {
		return false
	}
	s.ops = append(s.ops, baseState.ops...)
	for _, op := range accentState.ops {
		translated := op.Finalize()
		switch translated.operatorValue() {
		case "m", "l":
			if len(translated.operandsValue()) == 2 {
				x, ok := NumberValue(translated.operandsValue()[0])
				if !ok {
					return false
				}
				x, ok = cffFiniteAdd(x, adx)
				if !ok {
					return false
				}
				y, ok := NumberValue(translated.operandsValue()[1])
				if !ok {
					return false
				}
				y, ok = cffFiniteAdd(y, ady)
				if !ok {
					return false
				}
				translated.operandsValue()[0] = Number(x)
				translated.operandsValue()[1] = Number(y)
			}
		case "c":
			if len(translated.operandsValue()) == 6 {
				for _, index := range []int{0, 2, 4} {
					value, ok := NumberValue(translated.operandsValue()[index])
					if !ok {
						return false
					}
					value, ok = cffFiniteAdd(value, adx)
					if !ok {
						return false
					}
					translated.operandsValue()[index] = Number(value)
				}
				for _, index := range []int{1, 3, 5} {
					value, ok := NumberValue(translated.operandsValue()[index])
					if !ok {
						return false
					}
					value, ok = cffFiniteAdd(value, ady)
					if !ok {
						return false
					}
					translated.operandsValue()[index] = Number(value)
				}
			}
		}
		s.ops = append(s.ops, translated)
	}
	s.drawn = baseState.drawn || accentState.drawn
	return true
}

func (s *cffPathState) flexOperator(op int, args []float64) bool {
	switch op {
	case 1234: // hflex: two curves sharing their vertical control deltas.
		if len(args) != 7 {
			return false
		}
		s.curve([]float64{args[0], 0, args[1], args[2], args[3], 0})
		s.curve([]float64{args[4], 0, args[5], -args[2], args[6], 0})
	case 1235: // flex: six control pairs and a flex-depth operand.
		if len(args) != 13 {
			return false
		}
		s.curve(args[:6])
		s.curve(args[6:12])
	case 1236: // hflex1: horizontal displacement is implicit in the last point.
		if len(args) != 9 {
			return false
		}
		s.curve([]float64{args[0], args[1], args[2], args[3], args[4], 0})
		s.curve([]float64{args[5], 0, args[6], args[7], args[8], -(args[1] + args[3] + args[7])})
	case 1237: // flex1: one of the final x/y deltas is implicit.
		if len(args) != 11 {
			return false
		}
		s.curve(args[:6])
		dx := args[0] + args[2] + args[4] + args[6] + args[8]
		dy := args[1] + args[3] + args[5] + args[7] + args[9]
		if math.Abs(dx) > math.Abs(dy) {
			s.curve([]float64{args[6], args[7], args[8], args[9], args[10], -dy})
		} else {
			s.curve([]float64{args[6], args[7], args[8], args[9], -dx, args[10]})
		}
	}
	return true
}

func (s *cffPathState) curve(values []float64) {
	c1x, c1y := s.x+values[0], s.y+values[1]
	c2x, c2y := c1x+values[2], c1y+values[3]
	x, y := c2x+values[4], c2y+values[5]
	if !cffFiniteValues(c1x, c1y, c2x, c2y, x, y) {
		s.invalid = true
		return
	}
	s.ops = append(s.ops, cffOp("c", c1x, c1y, c2x, c2y, x, y))
	s.x, s.y = x, y
	s.drawn = true
}

func cffOp(operator string, values ...float64) ContentOp {
	operands := make([]Object, len(values))
	for i, value := range values {
		operands[i] = Number(value)
	}
	return newContentOpBorrowed(operator, operands, 0)
}

func parseCFFCharset(data []byte, offset, glyphCount int, stringsIndex [][]byte) map[int]string {
	return mapCFFCharsetText(parseCFFCharsetNames(data, offset, glyphCount, stringsIndex))
}

func parseCFFCharsetNames(data []byte, offset, glyphCount int, stringsIndex [][]byte) []string {
	return fontdata.ParseCFFCharsetNames(data, offset, glyphCount, stringsIndex)
}
func parseCFFCharsetCIDs(data []byte, offset, glyphCount int) map[int]int {
	return fontdata.ParseCFFCharsetCIDs(data, offset, glyphCount)
}
func mapCFFCharsetText(names []string) map[int]string {
	result := map[int]string{}
	for gid, name := range names {
		if text, ok := glyphText(name); ok {
			result[gid] = text
		}
	}
	return result
}

func parseCFFEncoding(data []byte, offset int, glyphNames []string, stringsIndex [][]byte) map[byte]string {
	return fontdata.ParseCFFEncoding(data, offset, glyphNames, stringsIndex)
}
func cffPredefinedCharset(id int) []string {
	return fontdata.CFFPredefinedCharset(id)
}

// cffBlendDictOperands evaluates a CFF2 DICT blend at the neutral design
// coordinate. DICT blends use the same ItemVariationData rows as CharString
// blends, but consume all values from the DICT operand stack at once.
func cffBlendDictOperandsAt(operands []float64, store *cffVariationStore, vsindex int, coords []float64) ([]float64, bool) {
	values, _, ok := cffBlendDictValuesAt(operands, store, vsindex, coords)
	return values, ok
}

func cffBlendDictValuesAt(operands []float64, store *cffVariationStore, vsindex int, coords []float64) ([]float64, []*cffBlendValue, bool) {
	if store == nil || vsindex < 0 || vsindex >= len(store.data) || len(operands) == 0 {
		return nil, nil, false
	}
	n := operands[len(operands)-1]
	if math.IsNaN(n) || math.IsInf(n, 0) || n < 1 || n != math.Trunc(n) || n > float64(len(operands)-1) {
		return nil, nil, false
	}
	count := int(n)
	item := store.data[vsindex]
	regions := len(item.regions)
	if regions < 0 || count > (len(operands)-1)/(regions+1) || len(operands) != count*(regions+1)+1 || count > len(item.rows) {
		return nil, nil, false
	}
	values := append([]float64(nil), operands[:count]...)
	blends := make([]*cffBlendValue, count)
	scalars := store.regionScalars(coords)
	for valueIndex := 0; valueIndex < count; valueIndex++ {
		row := item.rows[valueIndex]
		for regionIndex, region := range item.regions {
			if region < 0 || region >= len(scalars) || regionIndex >= len(row) {
				return nil, nil, false
			}
			values[valueIndex] += row[regionIndex] * scalars[region]
		}
		blends[valueIndex] = &cffBlendValue{base: values[valueIndex], deltas: append([]float64(nil), row...), regions: append([]int(nil), item.regions...)}
		blends[valueIndex].base = operands[valueIndex]
	}
	return values, blends, true
}
