package fontdata

import "encoding/binary"

// ParseCFFEncoding decodes a CFF encoding table into code-to-glyph-name
// mappings. Offset 0 and 1 select the predefined Standard and Expert tables.
func ParseCFFEncoding(data []byte, offset int, glyphNames []string, stringsIndex [][]byte) map[byte]string {
	if len(glyphNames) <= 1 {
		return nil
	}
	result := map[byte]string{}
	addGlyph := func(code, gid int) {
		if code >= 0 && code <= 255 && gid > 0 && gid < len(glyphNames) && glyphNames[gid] != "" {
			result[byte(code)] = glyphNames[gid]
		}
	}
	if offset == 0 || offset == 1 {
		encoding := CFFStandardEncoding[:]
		if offset == 1 {
			encoding = CFFExpertEncoding[:]
		}
		for code, sid := range encoding {
			if name, ok := CFFSIDName(sid, stringsIndex); ok {
				for gid := 1; gid < len(glyphNames); gid++ {
					if glyphNames[gid] == name {
						addGlyph(code, gid)
						break
					}
				}
			}
		}
		return result
	}
	if offset < 0 || offset >= len(data) {
		return nil
	}
	at := offset
	format := data[at]
	at++
	supplement := format&0x80 != 0
	format &= 0x7f
	switch format {
	case 0:
		if at >= len(data) {
			return nil
		}
		count := int(data[at])
		at++
		if count > len(glyphNames)-1 || at+count > len(data) {
			return nil
		}
		for gid := 1; gid <= count; gid++ {
			addGlyph(int(data[at]), gid)
			at++
		}
	case 1:
		if at >= len(data) {
			return nil
		}
		ranges := int(data[at])
		at++
		gid := 1
		for rangeIndex := 0; rangeIndex < ranges; rangeIndex++ {
			if at+1 >= len(data) {
				return nil
			}
			first, count := int(data[at]), int(data[at+1])
			at += 2
			if count+1 > len(glyphNames)-gid {
				return nil
			}
			for code := first; code <= first+count; code++ {
				addGlyph(code, gid)
				gid++
			}
		}
	default:
		return nil
	}
	if supplement {
		if at >= len(data) {
			return nil
		}
		count := int(data[at])
		at++
		if at+count*3 > len(data) {
			return nil
		}
		for i := 0; i < count; i++ {
			code := data[at]
			sid := int(binary.BigEndian.Uint16(data[at+1 : at+3]))
			at += 3
			if name, ok := CFFSIDName(sid, stringsIndex); ok {
				result[code] = name
			}
		}
	}
	return result
}
