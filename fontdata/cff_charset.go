package fontdata

import "encoding/binary"

// ParseCFFCharsetNames decodes a name-keyed CFF charset into GID-indexed
// glyph names. The predefined charset offsets are 0, 1, and 2.
func ParseCFFCharsetNames(data []byte, offset, glyphCount int, stringsIndex [][]byte) []string {
	if glyphCount <= 1 || offset < 0 {
		return nil
	}
	if offset <= 2 {
		names := CFFPredefinedCharset(offset)
		if len(names) < glyphCount {
			return nil
		}
		return names[:glyphCount]
	}
	if offset >= len(data) {
		return nil
	}
	format := data[offset]
	at := offset + 1
	sids := make([]int, glyphCount)
	sids[0] = 0
	readSID := func() (int, bool) {
		if at+2 > len(data) {
			return 0, false
		}
		sid := int(binary.BigEndian.Uint16(data[at:]))
		at += 2
		return sid, true
	}
	switch format {
	case 0:
		for gid := 1; gid < glyphCount; gid++ {
			sid, ok := readSID()
			if !ok {
				return nil
			}
			sids[gid] = sid
		}
	case 1, 2:
		gid := 1
		for gid < glyphCount {
			first, ok := readSID()
			if !ok || at >= len(data) {
				return nil
			}
			count := int(data[at])
			at++
			if format == 2 {
				if at+1 >= len(data) {
					return nil
				}
				count = int(binary.BigEndian.Uint16(data[at:]))
				at += 2
			}
			if count+1 > glyphCount-gid {
				return nil
			}
			for index := 0; index <= count && gid < glyphCount; index++ {
				sids[gid] = first + index
				gid++
			}
		}
	default:
		return nil
	}
	result := make([]string, glyphCount)
	for gid, sid := range sids {
		if name, ok := CFFSIDName(sid, stringsIndex); ok {
			result[gid] = name
		}
	}
	return result
}

// CFFPredefinedCharset returns one of the generated Adobe charset tables.
func CFFPredefinedCharset(id int) []string {
	switch id {
	case 0:
		return CFFISOAdobeCharset[:]
	case 1:
		return CFFExpertCharset[:]
	case 2:
		return CFFExpertSubsetCharset[:]
	default:
		return nil
	}
}

// CFFSIDName resolves a standard or custom CFF String ID.
func CFFSIDName(sid int, stringsIndex [][]byte) (string, bool) {
	if sid >= 0 && sid < len(CFFStandardStrings) {
		return CFFStandardStrings[sid], true
	}
	custom := sid - len(CFFStandardStrings)
	if custom < 0 || custom >= len(stringsIndex) {
		return "", false
	}
	return string(stringsIndex[custom]), true
}
