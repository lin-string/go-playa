package fontdata

import "encoding/binary"

// ParseTrueTypeHorizontalMetrics returns PDF glyph-space widths from an
// embedded TrueType/OpenType hmtx table. The final advance-width entry is
// repeated for glyphs beyond numberOfHMetrics, as required by the sfnt
// hhea/hmtx layout. Malformed or incomplete tables return nil.
func ParseTrueTypeHorizontalMetrics(data []byte) map[int]float64 {
	head, ok := trueTypeTable(data, "head")
	if !ok || len(head) < 20 {
		return nil
	}
	unitsPerEm := int(binary.BigEndian.Uint16(head[18:20]))
	if unitsPerEm == 0 {
		return nil
	}
	hhea, ok := trueTypeTable(data, "hhea")
	if !ok || len(hhea) < 36 {
		return nil
	}
	numHMetrics := int(binary.BigEndian.Uint16(hhea[34:36]))
	maxp, ok := trueTypeTable(data, "maxp")
	if !ok || len(maxp) < 6 {
		return nil
	}
	numGlyphs := int(binary.BigEndian.Uint16(maxp[4:6]))
	hmtx, ok := trueTypeTable(data, "hmtx")
	if !ok || numHMetrics == 0 || numGlyphs == 0 || numHMetrics > numGlyphs {
		return nil
	}
	requiredHmtx := numHMetrics*4 + (numGlyphs-numHMetrics)*2
	if requiredHmtx > len(hmtx) {
		return nil
	}
	metrics := make(map[int]float64, numGlyphs)
	lastAdvance := uint16(0)
	for glyphID := 0; glyphID < numGlyphs; glyphID++ {
		metricIndex := glyphID
		if metricIndex >= numHMetrics {
			metricIndex = numHMetrics - 1
		}
		offset := metricIndex * 4
		if glyphID < numHMetrics {
			lastAdvance = binary.BigEndian.Uint16(hmtx[offset : offset+2])
		}
		metrics[glyphID] = float64(lastAdvance) * 1000 / float64(unitsPerEm)
	}
	return metrics
}

func trueTypeTable(data []byte, wanted string) ([]byte, bool) {
	if len(data) < 12 || (string(data[:4]) != "\x00\x01\x00\x00" && string(data[:4]) != "true") {
		return nil, false
	}
	numTables := int(binary.BigEndian.Uint16(data[4:6]))
	for i := 0; i < numTables; i++ {
		entry := 12 + i*16
		if entry+16 > len(data) {
			return nil, false
		}
		if string(data[entry:entry+4]) != wanted {
			continue
		}
		offset := int(binary.BigEndian.Uint32(data[entry+8 : entry+12]))
		length := int(binary.BigEndian.Uint32(data[entry+12 : entry+16]))
		if offset < 0 || length < 0 || offset > len(data) || length > len(data)-offset {
			return nil, false
		}
		return data[offset : offset+length], true
	}
	return nil, false
}
