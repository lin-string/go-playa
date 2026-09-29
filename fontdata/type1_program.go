package fontdata

import (
	"bytes"

	"github.com/lin-string/go-playa/pdftypes/primitives"
)

// Type1Program is an owned, read-only view of the encrypted program sections
// extracted from an embedded Type 1 font.
type Type1Program struct {
	charstrings map[string][]byte
	subrs       [][]byte
}

func cloneType1Bytes(value []byte) []byte {
	if len(value) == 0 {
		return nil
	}
	return primitives.CloneBytes(value)
}

// ParseType1Program extracts /CharStrings and /Subrs from a decrypted Type 1
// program. CharString bodies are decrypted with the program's /lenIV value;
// malformed or oversized entries are ignored without allocating from their
// untrusted lengths.
func ParseType1Program(data []byte) *Type1Program {
	program := &Type1Program{charstrings: map[string][]byte{}}
	lenIV := parseType1LenIV(data)
	program.subrs = parseType1Subrs(data, lenIV)
	start := bytes.Index(data, []byte("/CharStrings"))
	if start < 0 {
		return program
	}
	data = data[start+len("/CharStrings"):]
	for at := 0; at < len(data); {
		slash := bytes.IndexByte(data[at:], '/')
		if slash < 0 {
			break
		}
		slash += at
		end := slash + 1
		for end < len(data) && data[end] > ' ' {
			end++
		}
		name := string(data[slash+1 : end])
		if name == "CharStrings" || name == "Subrs" {
			at = end
			continue
		}
		rd := bytes.Index(data[end:], []byte("RD"))
		if rd < 0 || rd > 128 {
			at = end
			continue
		}
		rd += end
		length := parseType1Length(data[end:rd])
		if length < 0 {
			at = rd + 2
			continue
		}
		body := rd + 2
		if body < len(data) && (data[body] == ' ' || data[body] == '\n' || data[body] == '\r') {
			body++
		}
		if body < 0 || body > len(data) || length > len(data)-body {
			break
		}
		program.charstrings[name] = decryptType1Charstring(data[body:body+length], lenIV)
		at = body + length
	}
	return program
}

// CharString returns an owned copy of the named decrypted CharString.
func (program *Type1Program) CharString(name string) []byte {
	return cloneType1Bytes(program.charstrings[name])
}

// CharStringsCopy returns all named decrypted CharStrings as an owned map.
func (program *Type1Program) CharStringsCopy() map[string][]byte {
	result := make(map[string][]byte, len(program.charstrings))
	for name, data := range program.charstrings {
		result[name] = cloneType1Bytes(data)
	}
	return result
}

// SubrsCopy returns all decrypted local Subroutines as owned byte slices.
func (program *Type1Program) SubrsCopy() [][]byte {
	if program.subrs == nil {
		return nil
	}
	result := make([][]byte, len(program.subrs))
	for index, data := range program.subrs {
		result[index] = cloneType1Bytes(data)
	}
	return result
}

func parseType1LenIV(data []byte) int {
	marker := bytes.Index(data, []byte("/lenIV"))
	if marker < 0 {
		return 4
	}
	value, _, ok := parseType1Integer(data, marker+len("/lenIV"))
	if !ok {
		return 4
	}
	return value
}

func parseType1Subrs(data []byte, lenIV int) [][]byte {
	marker := bytes.Index(data, []byte("/Subrs"))
	if marker < 0 {
		return nil
	}
	count, at, ok := parseType1Integer(data, marker+len("/Subrs"))
	if !ok || count < 0 || count > 65536 {
		return nil
	}
	result := make([][]byte, count)
	for at < len(data) {
		dup := bytes.Index(data[at:], []byte("dup"))
		if dup < 0 {
			break
		}
		at += dup + len("dup")
		index, next, ok := parseType1Integer(data, at)
		if !ok {
			continue
		}
		length, next, ok := parseType1Integer(data, next)
		if !ok || index < 0 || index >= len(result) || length < 0 {
			continue
		}
		rd := bytes.Index(data[next:], []byte("RD"))
		if rd < 0 || rd > 128 {
			continue
		}
		rd += next
		body := rd + 2
		if body < len(data) && data[body] <= ' ' {
			body++
		}
		if body < 0 || body > len(data) || length > len(data)-body {
			break
		}
		result[index] = decryptType1Charstring(data[body:body+length], lenIV)
		at = body + length
	}
	return result
}

func parseType1Integer(data []byte, at int) (int, int, bool) {
	for at < len(data) && data[at] <= ' ' {
		at++
	}
	start := at
	sign := 1
	if at < len(data) && data[at] == '-' {
		sign, at = -1, at+1
	}
	digits := at
	for at < len(data) && data[at] >= '0' && data[at] <= '9' {
		at++
	}
	if at == digits {
		return 0, start, false
	}
	maxInt := uint(^uint(0) >> 1)
	limit := maxInt
	if sign < 0 {
		limit++
	}
	value := uint(0)
	for _, digit := range data[digits:at] {
		n := uint(digit - '0')
		if value > (limit-n)/10 {
			return 0, start, false
		}
		value = value*10 + n
	}
	if sign < 0 {
		if value == maxInt+1 {
			return -int(maxInt) - 1, at, true
		}
		return -int(value), at, true
	}
	return int(value), at, true
}

func parseType1Length(data []byte) int {
	end := len(data)
	for end > 0 && data[end-1] <= ' ' {
		end--
	}
	start := end
	for start > 0 && data[start-1] >= '0' && data[start-1] <= '9' {
		start--
	}
	if start == end {
		return -1
	}
	maxInt := uint(^uint(0) >> 1)
	n := uint(0)
	for _, value := range data[start:end] {
		digit := uint(value - '0')
		if n > (maxInt-digit)/10 {
			return -1
		}
		n = n*10 + digit
	}
	return int(n)
}

func decryptType1Charstring(data []byte, lenIV int) []byte {
	if lenIV < 0 {
		return cloneType1Bytes(data)
	}
	decrypted := decryptType1WithSeed(data, 4330)
	if len(decrypted) <= lenIV {
		return nil
	}
	return cloneType1Bytes(decrypted[lenIV:])
}

func decryptType1WithSeed(data []byte, seed uint32) []byte {
	const c1, c2 = uint32(52845), uint32(22719)
	r := seed
	decrypted := make([]byte, len(data))
	for index, cipher := range data {
		decrypted[index] = cipher ^ byte(r>>8)
		r = ((uint32(cipher) + r) * c1) + c2
		r &= 0xffff
	}
	return decrypted
}
