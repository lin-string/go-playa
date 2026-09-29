package fontdata

import "encoding/binary"

// ParseCFFCharsetCIDs decodes a CID-keyed CFF charset into CID-to-GID
// mappings. The charset format byte is at offset; predefined name-keyed
// charsets are intentionally not accepted.
func ParseCFFCharsetCIDs(data []byte, offset, glyphCount int) map[int]int {
	if glyphCount <= 1 || offset < 3 || offset >= len(data) {
		return nil
	}
	at := offset + 1
	result := make(map[int]int, glyphCount-1)
	readUint16 := func() (int, bool) {
		if at+2 > len(data) {
			return 0, false
		}
		value := int(binary.BigEndian.Uint16(data[at:]))
		at += 2
		return value, true
	}
	format := data[offset]
	gid := 1
	for gid < glyphCount {
		first, ok := readUint16()
		if !ok {
			return nil
		}
		var count int
		switch format {
		case 0:
			count = 0
		case 1:
			if at >= len(data) {
				return nil
			}
			count = int(data[at])
			at++
		case 2:
			var valid bool
			count, valid = readUint16()
			if !valid {
				return nil
			}
		default:
			return nil
		}
		if count+1 > glyphCount-gid {
			return nil
		}
		for index := 0; index <= count && gid < glyphCount; index++ {
			result[first+index] = gid
			gid++
		}
	}
	return result
}
