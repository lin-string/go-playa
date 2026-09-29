package fontdata

import "encoding/binary"

// ParseTrueTypeCMapGlyphs returns the Unicode text associated with each
// non-zero glyph ID in an embedded TrueType cmap table. The parser is bounded
// by the table lengths and rejects malformed or overlapping groups.
func ParseTrueTypeCMapGlyphs(data []byte) map[int]string {
	if len(data) < 12 {
		return nil
	}
	tables := int(binary.BigEndian.Uint16(data[4:6]))
	var cmap []byte
	for i := 0; i < tables; i++ {
		entry := 12 + i*16
		if entry+16 > len(data) {
			return nil
		}
		if string(data[entry:entry+4]) != "cmap" {
			continue
		}
		offset := int(binary.BigEndian.Uint32(data[entry+8 : entry+12]))
		length := int(binary.BigEndian.Uint32(data[entry+12 : entry+16]))
		if offset > len(data) || length > len(data)-offset {
			return nil
		}
		cmap = data[offset : offset+length]
		break
	}
	if len(cmap) < 4 {
		return nil
	}
	count := int(binary.BigEndian.Uint16(cmap[2:4]))
	if count > (len(cmap)-4)/8 {
		return nil
	}
	best, bestRank := -1, -1
	var bestFormat uint16
	for i := 0; i < count; i++ {
		entry := 4 + i*8
		platform := binary.BigEndian.Uint16(cmap[entry : entry+2])
		encoding := binary.BigEndian.Uint16(cmap[entry+2 : entry+4])
		offset := int(binary.BigEndian.Uint32(cmap[entry+4 : entry+8]))
		if offset > len(cmap) || len(cmap)-offset < 2 {
			continue
		}
		format := binary.BigEndian.Uint16(cmap[offset : offset+2])
		rank := trueTypeCMapFormatRank(format)
		if rank < 0 {
			continue
		}
		if platform == 3 && encoding == 1 && rank < 3 {
			rank = 3
		} else if platform == 3 && rank < 2 {
			rank = 2
		} else if platform == 0 && rank < 1 {
			rank = 1
		}
		if rank > bestRank {
			best, bestRank, bestFormat = offset, rank, format
		}
	}
	if best < 0 {
		return nil
	}
	data = cmap[best:]
	switch bestFormat {
	case 0:
		return parseTrueTypeCMapFormat0(data)
	case 2:
		return parseTrueTypeCMapFormat2(data)
	case 4:
		return parseTrueTypeCMapFormat4(data)
	case 6:
		return parseTrueTypeCMapFormat6(data)
	case 8:
		return parseTrueTypeCMapFormat8(data)
	case 10:
		return parseTrueTypeCMapFormat10(data)
	case 12:
		return parseTrueTypeCMapFormat12(data)
	case 13:
		return parseTrueTypeCMapFormat13(data)
	default:
		return nil
	}
}

// The format-specific entry points are kept for focused parser tests and
// diagnostics; normal callers should use ParseTrueTypeCMapGlyphs.
func ParseTrueTypeCMapFormat0(data []byte) map[int]string  { return parseTrueTypeCMapFormat0(data) }
func ParseTrueTypeCMapFormat2(data []byte) map[int]string  { return parseTrueTypeCMapFormat2(data) }
func ParseTrueTypeCMapFormat4(data []byte) map[int]string  { return parseTrueTypeCMapFormat4(data) }
func ParseTrueTypeCMapFormat6(data []byte) map[int]string  { return parseTrueTypeCMapFormat6(data) }
func ParseTrueTypeCMapFormat8(data []byte) map[int]string  { return parseTrueTypeCMapFormat8(data) }
func ParseTrueTypeCMapFormat10(data []byte) map[int]string { return parseTrueTypeCMapFormat10(data) }
func ParseTrueTypeCMapFormat12(data []byte) map[int]string { return parseTrueTypeCMapFormat12(data) }
func ParseTrueTypeCMapFormat13(data []byte) map[int]string { return parseTrueTypeCMapFormat13(data) }

func trueTypeCMapFormatRank(format uint16) int {
	switch format {
	// Playa's TrueType fallback consumes the legacy format 0/2/4
	// subtables and lets those mappings win when a font also advertises
	// modern format 8/10/12/13 subtables. Keep modern formats as a
	// fallback for fonts that have no legacy Unicode cmap.
	case 4:
		return 100
	case 2:
		return 90
	case 6:
		return 30
	case 0:
		return 80
	case 12:
		return 70
	case 13:
		return 60
	case 10:
		return 50
	case 8:
		return 40
	default:
		return -1
	}
}

func parseTrueTypeCMapFormat0(data []byte) map[int]string {
	if len(data) < 262 || binary.BigEndian.Uint16(data[0:2]) != 0 {
		return nil
	}
	length := int(binary.BigEndian.Uint16(data[2:4]))
	if length < 262 || length > len(data) {
		return nil
	}
	out := map[int]string{}
	for code, glyph := range data[6:length][:256] {
		if glyph != 0 {
			out[int(glyph)] = string(rune(code))
		}
	}
	return out
}

func parseTrueTypeCMapFormat6(data []byte) map[int]string {
	if len(data) < 10 || binary.BigEndian.Uint16(data[0:2]) != 6 {
		return nil
	}
	length := int(binary.BigEndian.Uint16(data[2:4]))
	if length < 10 || length > len(data) {
		return nil
	}
	first := binary.BigEndian.Uint16(data[6:8])
	count := int(binary.BigEndian.Uint16(data[8:10]))
	if 10+count*2 > length {
		return nil
	}
	out := map[int]string{}
	for i := 0; i < count; i++ {
		glyph := binary.BigEndian.Uint16(data[10+i*2 : 12+i*2])
		if glyph != 0 && uint32(first)+uint32(i) <= 0x10ffff {
			out[int(glyph)] = string(rune(uint32(first) + uint32(i)))
		}
	}
	return out
}

func parseTrueTypeCMapFormat10(data []byte) map[int]string {
	if len(data) < 20 || binary.BigEndian.Uint16(data[0:2]) != 10 {
		return nil
	}
	length := int(binary.BigEndian.Uint32(data[4:8]))
	if length < 20 || length > len(data) {
		return nil
	}
	start := binary.BigEndian.Uint32(data[12:16])
	count := int(binary.BigEndian.Uint32(data[16:20]))
	if count > (length-20)/2 {
		return nil
	}
	out := map[int]string{}
	for i := 0; i < count; i++ {
		code := start + uint32(i)
		glyph := binary.BigEndian.Uint16(data[20+i*2 : 22+i*2])
		if glyph != 0 && code <= 0x10ffff {
			out[int(glyph)] = string(rune(code))
		}
	}
	return out
}

func parseTrueTypeCMapFormat8(data []byte) map[int]string {
	if len(data) < 8204 || binary.BigEndian.Uint16(data[0:2]) != 8 {
		return nil
	}
	length := int(binary.BigEndian.Uint32(data[4:8]))
	if length < 8204 || length > len(data) {
		return nil
	}
	groups := int(binary.BigEndian.Uint32(data[8200:8204]))
	if groups > (length-8204)/12 {
		return nil
	}
	out := map[int]string{}
	var previousEnd uint32
	for i := 0; i < groups; i++ {
		offset := 8204 + i*12
		start := binary.BigEndian.Uint32(data[offset : offset+4])
		end := binary.BigEndian.Uint32(data[offset+4 : offset+8])
		glyph := binary.BigEndian.Uint32(data[offset+8 : offset+12])
		if end < start || end > 0x10ffff || (i > 0 && start <= previousEnd) {
			return nil
		}
		previousEnd = end
		span := trueTypeCMapMappedSpan(start, end, glyph)
		for index := uint32(0); index < span; index++ {
			out[int(glyph+index)] = string(rune(start + index))
		}
	}
	return out
}

func parseTrueTypeCMapFormat2(data []byte) map[int]string {
	if len(data) < 518 || binary.BigEndian.Uint16(data[0:2]) != 2 {
		return nil
	}
	length := int(binary.BigEndian.Uint16(data[2:4]))
	if length < 518 || length > len(data) {
		return nil
	}
	maxKey := 0
	for i := 0; i < 256; i++ {
		key := int(binary.BigEndian.Uint16(data[6+i*2 : 8+i*2]))
		if key%8 == 0 && key/8 > maxKey {
			maxKey = key / 8
		}
	}
	if 518+(maxKey+1)*8 > length {
		return nil
	}
	out := map[int]string{}
	for i := 0; i <= maxKey; i++ {
		sub := 518 + i*8
		first := binary.BigEndian.Uint16(data[sub : sub+2])
		count := int(binary.BigEndian.Uint16(data[sub+2 : sub+4]))
		delta := int16(binary.BigEndian.Uint16(data[sub+4 : sub+6]))
		rangeOffset := int(binary.BigEndian.Uint16(data[sub+6 : sub+8]))
		if count == 0 {
			continue
		}
		glyphBase := sub + 6 + rangeOffset
		if glyphBase < 0 || glyphBase+count*2 > length {
			return nil
		}
		high := 0
		if i > 0 {
			high = int(binary.BigEndian.Uint16(data[6+i*2:8+i*2])) / 8
		}
		for j := 0; j < count; j++ {
			glyph := binary.BigEndian.Uint16(data[glyphBase+j*2 : glyphBase+j*2+2])
			if glyph == 0 {
				continue
			}
			glyph = uint16(int(glyph) + int(delta))
			code := uint32(first) + uint32(j)
			if i > 0 {
				code |= uint32(high) << 8
			}
			if code <= 0x10ffff {
				out[int(glyph)] = string(rune(code))
			}
		}
	}
	return out
}

func parseTrueTypeCMapFormat13(data []byte) map[int]string {
	if len(data) < 16 || binary.BigEndian.Uint16(data[0:2]) != 13 {
		return nil
	}
	length := int(binary.BigEndian.Uint32(data[4:8]))
	if length < 16 || length > len(data) {
		return nil
	}
	groups := int(binary.BigEndian.Uint32(data[12:16]))
	if groups > (length-16)/12 {
		return nil
	}
	out := map[int]string{}
	var previousEnd uint32
	for i := 0; i < groups; i++ {
		offset := 16 + i*12
		start := binary.BigEndian.Uint32(data[offset : offset+4])
		end := binary.BigEndian.Uint32(data[offset+4 : offset+8])
		glyph := binary.BigEndian.Uint32(data[offset+8 : offset+12])
		if start > end || end > 0x10ffff || glyph > 0xffff || (i > 0 && start <= previousEnd) {
			return nil
		}
		previousEnd = end
		out[int(glyph)] = string(rune(start))
	}
	return out
}

func parseTrueTypeCMapFormat12(data []byte) map[int]string {
	if len(data) < 16 || binary.BigEndian.Uint16(data[0:2]) != 12 {
		return nil
	}
	length := int(binary.BigEndian.Uint32(data[4:8]))
	if length < 16 || length > len(data) {
		return nil
	}
	groups := int(binary.BigEndian.Uint32(data[12:16]))
	if groups > (length-16)/12 {
		return nil
	}
	out := map[int]string{}
	var previousEnd uint32
	for i := 0; i < groups; i++ {
		offset := 16 + i*12
		start := binary.BigEndian.Uint32(data[offset : offset+4])
		end := binary.BigEndian.Uint32(data[offset+4 : offset+8])
		glyph := binary.BigEndian.Uint32(data[offset+8 : offset+12])
		if end < start || end > 0x10ffff || (i > 0 && start <= previousEnd) {
			return nil
		}
		previousEnd = end
		span := trueTypeCMapMappedSpan(start, end, glyph)
		for index := uint32(0); index < span; index++ {
			out[int(glyph+index)] = string(rune(start + index))
		}
	}
	return out
}

func parseTrueTypeCMapFormat4(data []byte) map[int]string {
	if len(data) < 16 || binary.BigEndian.Uint16(data[0:2]) != 4 {
		return nil
	}
	length := int(binary.BigEndian.Uint16(data[2:4]))
	if length < 16 || length > len(data) {
		return nil
	}
	segCount := int(binary.BigEndian.Uint16(data[6:8]) / 2)
	endOffset := 14
	startOffset := endOffset + segCount*2 + 2
	deltaOffset := startOffset + segCount*2
	rangeOffset := deltaOffset + segCount*2
	if rangeOffset+segCount*2 > length {
		return nil
	}
	out := map[int]string{}
	for i := 0; i < segCount; i++ {
		endCode := binary.BigEndian.Uint16(data[endOffset+i*2 : endOffset+i*2+2])
		startCode := binary.BigEndian.Uint16(data[startOffset+i*2 : startOffset+i*2+2])
		delta := int16(binary.BigEndian.Uint16(data[deltaOffset+i*2 : deltaOffset+i*2+2]))
		rangeValueOffset := rangeOffset + i*2
		rangeValue := binary.BigEndian.Uint16(data[rangeValueOffset : rangeValueOffset+2])
		if startCode == 0xffff && endCode == 0xffff {
			continue
		}
		for code := startCode; code <= endCode; code++ {
			glyphID := uint16(0)
			if rangeValue == 0 {
				glyphID = uint16(int(code) + int(delta))
			} else {
				glyphOffset := rangeValueOffset + int(rangeValue) + int(code-startCode)*2
				if glyphOffset+2 > length {
					return nil
				}
				raw := binary.BigEndian.Uint16(data[glyphOffset : glyphOffset+2])
				if raw != 0 {
					glyphID = uint16(int(raw) + int(delta))
				}
			}
			if glyphID != 0 {
				out[int(glyphID)] = string(rune(code))
			}
			if code == 0xffff {
				break
			}
		}
	}
	return out
}

func trueTypeCMapMappedSpan(start, end, glyph uint32) uint32 {
	if end < start || glyph > 0xffff {
		return 0
	}
	span := uint64(end-start) + 1
	maxSpan := uint64(0x10000) - uint64(glyph)
	if span > maxSpan {
		return uint32(maxSpan)
	}
	return uint32(span)
}
