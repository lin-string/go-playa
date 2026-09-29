package fontdata

import (
	"encoding/binary"
	"strconv"
)

// ParseCFFNumber parses one CFF/CFF2 DICT or CharString number at offset at.
// The returned offset points immediately after the number. Malformed or
// truncated operands return ok=false.
func ParseCFFNumber(data []byte, at int) (float64, int, bool) {
	if at < 0 || at >= len(data) {
		return 0, at, false
	}
	b := data[at]
	switch {
	case b >= 32 && b <= 246:
		return float64(int(b) - 139), at + 1, true
	case b >= 247 && b <= 250:
		if len(data)-at < 2 {
			return 0, at, false
		}
		return float64((int(b)-247)*256 + int(data[at+1]) + 108), at + 2, true
	case b >= 251 && b <= 254:
		if len(data)-at < 2 {
			return 0, at, false
		}
		return float64(-(int(b)-251)*256 - int(data[at+1]) - 108), at + 2, true
	case b == 28:
		if len(data)-at < 3 {
			return 0, at, false
		}
		return float64(int16(binary.BigEndian.Uint16(data[at+1:]))), at + 3, true
	case b == 29:
		if len(data)-at < 5 {
			return 0, at, false
		}
		return float64(int32(binary.BigEndian.Uint32(data[at+1:]))), at + 5, true
	case b == 30:
		return parseCFFReal(data, at+1)
	case b == 255:
		if len(data)-at < 5 {
			return 0, at, false
		}
		return float64(int32(binary.BigEndian.Uint32(data[at+1:]))) / 65536, at + 5, true
	default:
		return 0, at, false
	}
}

// ParseCFFOperator parses one CFF DICT operator. CFF2 additionally accepts
// the single-byte variation operators 22, 23, and 24.
func ParseCFFOperator(data []byte, at int, cff2 bool) (int, int, bool) {
	if at < 0 || at >= len(data) {
		return 0, at, false
	}
	if cff2 && (data[at] == 22 || data[at] == 23 || data[at] == 24) {
		return int(data[at]), at + 1, true
	}
	if data[at] > 21 {
		return 0, at, false
	}
	if data[at] == 12 {
		if len(data)-at < 2 {
			return 0, at, false
		}
		return 1200 + int(data[at+1]), at + 2, true
	}
	return int(data[at]), at + 1, true
}

// ParseCFFCharStringOperator parses one Type 2 CharString operator. The CFF2
// flag preserves the CFF2 handling of variation operators 15 and 16.
func ParseCFFCharStringOperator(data []byte, at int, cff2 bool) (int, int, bool) {
	if at < 0 || at >= len(data) || data[at] > 31 || data[at] == 0 {
		return 0, at, false
	}
	if cff2 && (data[at] == 15 || data[at] == 16) {
		return int(data[at]), at + 1, true
	}
	if data[at] == 12 {
		if len(data)-at < 2 {
			return 0, at, false
		}
		return 1200 + int(data[at+1]), at + 2, true
	}
	return int(data[at]), at + 1, true
}

func parseCFFReal(data []byte, at int) (float64, int, bool) {
	digits := []byte{}
	for at < len(data) {
		b := data[at]
		at++
		for _, nibble := range []byte{b >> 4, b & 15} {
			switch nibble {
			case 15:
				value, err := strconv.ParseFloat(string(digits), 64)
				return value, at, err == nil
			case 14:
				digits = append(digits, '-')
			case 11:
				digits = append(digits, 'E')
			case 12:
				digits = append(digits, 'E', '-')
			case 10:
				digits = append(digits, '.')
			case 0, 1, 2, 3, 4, 5, 6, 7, 8, 9:
				digits = append(digits, '0'+nibble)
			default:
				return 0, at, false
			}
		}
	}
	return 0, at, false
}
