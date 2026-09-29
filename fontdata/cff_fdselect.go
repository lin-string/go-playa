package fontdata

import "encoding/binary"

// ParseCFFFDSelect parses a CFF FDSelect table into a glyph-ID to font-dict
// index map. The table may use CFF1 format 0 or 3, or CFF2 format 4.
// Malformed or incomplete tables return nil.
func ParseCFFFDSelect(data []byte, offset, glyphCount int) map[int]int {
	if offset < 0 || offset >= len(data) || glyphCount <= 0 {
		return nil
	}
	at := offset + 1
	// Avoid preallocating from an untrusted glyph count before the table has
	// passed its format-specific bounds checks.
	result := make(map[int]int)
	switch data[offset] {
	case 0:
		if glyphCount > len(data)-at {
			return nil
		}
		for gid := 0; gid < glyphCount; gid++ {
			result[gid] = int(data[at+gid])
		}
	case 3:
		if len(data)-at < 4 {
			return nil
		}
		ranges := int(binary.BigEndian.Uint16(data[at:]))
		at += 2
		first := int(binary.BigEndian.Uint16(data[at:]))
		at += 2
		for i := 0; i < ranges; i++ {
			if len(data)-at < 3 {
				return nil
			}
			fd := int(data[at])
			at++
			next := int(binary.BigEndian.Uint16(data[at:]))
			at += 2
			if next < first || first >= glyphCount {
				return nil
			}
			if next > glyphCount {
				next = glyphCount
			}
			for gid := first; gid < next; gid++ {
				result[gid] = fd
			}
			first = next
		}
		if first != glyphCount {
			return nil
		}
	case 4:
		if len(data)-at < 8 {
			return nil
		}
		ranges := int(binary.BigEndian.Uint32(data[at:]))
		at += 4
		first := int(binary.BigEndian.Uint32(data[at:]))
		at += 4
		for i := 0; i < ranges; i++ {
			if len(data)-at < 6 {
				return nil
			}
			fd := int(binary.BigEndian.Uint16(data[at:]))
			at += 2
			nextValue := binary.BigEndian.Uint32(data[at:])
			at += 4
			if uint64(nextValue) > uint64(glyphCount) || nextValue < uint32(first) || first >= glyphCount {
				return nil
			}
			next := int(nextValue)
			for gid := first; gid < next; gid++ {
				result[gid] = fd
			}
			first = next
		}
		if first != glyphCount {
			return nil
		}
	default:
		return nil
	}
	return result
}
