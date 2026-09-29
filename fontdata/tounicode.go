package fontdata

import (
	"bufio"
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"iter"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"
)

// ErrToUnicodeRangeOverflow reports a valid source range that cannot be
// materialized without unsafe memory growth or integer overflow.
var ErrToUnicodeRangeOverflow = errors.New("playa: ToUnicode range overflow")

const (
	maxToUnicodeRange        = 1 << 20
	maxToUnicodePhysicalLine = 4 << 20
)

// ToUnicodeMap is the owned representation of a PDF ToUnicode CMap. Mapping
// keys retain their original source-code width, while Codespaces controls how
// a byte stream is split during Decode. The map is immutable after parsing.
type ToUnicodeMap struct {
	codespaces []CodeSpace
	mappings   map[string]string
}

// ToUnicodeCodeLengths returns distinct source-code widths in the same
// ascending order used by Playa's ToUnicodeMap decoder. This is intentionally
// separate from CodeLengths, whose descending order belongs to the generic
// longest-match CMap decoder.
func ToUnicodeCodeLengths(spaces []CodeSpace) []int {
	seen := [5]bool{}
	for _, space := range spaces {
		if len(space.low) == len(space.high) && len(space.low) > 0 && len(space.low) < len(seen) {
			seen[len(space.low)] = true
		}
	}
	out := make([]int, 0, len(seen)-1)
	for length := 1; length < len(seen); length++ {
		if seen[length] {
			out = append(out, length)
		}
	}
	return out
}

// ParseToUnicodeMap parses mappings and codespace ranges into the same
// codespace-aware model used by the Playa ToUnicodeMap implementation.
func ParseToUnicodeMap(data []byte) (*ToUnicodeMap, error) {
	mappings, err := ParseToUnicodeCodes(data)
	if err != nil {
		return nil, err
	}
	codespaces, err := parseToUnicodeCodeSpaces(data)
	if err != nil {
		return nil, fmt.Errorf("playa: parse ToUnicode codespaces: %w", err)
	}
	return &ToUnicodeMap{
		codespaces: cloneCodeSpaces(codespaces),
		mappings:   mappings,
	}, nil
}

// CodespacesCopy returns an owned snapshot of the source-code ranges.
func (m *ToUnicodeMap) CodespacesCopy() []CodeSpace {
	return cloneCodeSpaces(m.codespaces)
}

// Lookup returns the exact mapping for one complete source code.
func (m *ToUnicodeMap) Lookup(code []byte) (string, bool) {
	value, ok := m.mappings[string(code)]
	return value, ok
}

// MappingsCopy returns an owned copy of all source-code mappings. It is used
// by domain adapters that still expose Playa's historical uint16 projection.
func (m *ToUnicodeMap) MappingsCopy() map[string]string {
	out := make(map[string]string, len(m.mappings))
	for code, value := range m.mappings {
		out[code] = value
	}
	return out
}

// Decode decodes a source byte stream according to its codespace ranges.
// Undefined codes use Playa's numeric source-code fallback.
func (m *ToUnicodeMap) Decode(data []byte) string {
	var out strings.Builder
	for value := range m.DecodeSeq(data) {
		out.WriteString(value)
	}
	return out.String()
}

// DecodeSeq lazily decodes source bytes according to the map's codespaces.
// Each traversal starts from the beginning, and yielded strings do not retain
// mutable parser state.
func (m *ToUnicodeMap) DecodeSeq(data []byte) iter.Seq[string] {
	return func(yield func(string) bool) {
		// Playa treats a ToUnicode map without codespace declarations as a
		// byte-wise fallback and does not consult its mapping dictionary.
		if len(m.codespaces) == 0 {
			for _, value := range data {
				if !yield(string(rune(value))) {
					return
				}
			}
			return
		}
		lengths := ToUnicodeCodeLengths(m.codespaces)
		if len(lengths) == 0 {
			lengths = []int{1}
		}
		for offset := 0; offset < len(data); {
			matched := false
			for _, length := range lengths {
				if length > len(data)-offset {
					continue
				}
				code := data[offset : offset+length]
				if len(m.codespaces) > 0 && !CodeInSpaces(code, m.codespaces) {
					continue
				}
				if value, ok := m.Lookup(code); ok {
					if !yield(value) {
						return
					}
				} else {
					if !yield(string(rune(codeNumber(code)))) {
						return
					}
				}
				offset += length
				matched = true
				break
			}
			if matched {
				continue
			}
			if !yield(string(rune(data[offset]))) {
				return
			}
			offset++
		}
	}
}

func cloneCodeSpaces(source []CodeSpace) []CodeSpace {
	if source == nil {
		return nil
	}
	out := make([]CodeSpace, len(source))
	for i, space := range source {
		out[i] = NewCodeSpace(space.LowCopy(), space.HighCopy())
	}
	return out
}

// ParseToUnicodeCodes parses bfchar/bfrange and cidchar/cidrange mappings.
// The map key is the original source-code byte sequence, so four-byte codes
// are preserved instead of being narrowed to uint16.
func ParseToUnicodeCodes(data []byte) (map[string]string, error) {
	return parseToUnicodeCodes(data, false)
}

// ParseToUnicodeCodesStrict reports valid-but-unmaterializable ranges to the
// document parser, which can then enter its malformed-font fallback state.
func ParseToUnicodeCodesStrict(data []byte) (map[string]string, error) {
	return parseToUnicodeCodes(data, true)
}

func parseToUnicodeCodes(data []byte, rejectOversizedRange bool) (map[string]string, error) {
	out := map[string]string{}
	section, pending := "", ""
	scanner := bufio.NewScanner(bytes.NewReader(data))
	scanner.Buffer(make([]byte, 64<<10), maxToUnicodePhysicalLine)
	stopAtEndCMap := false
	for scanner.Scan() {
		if stopAtEndCMap {
			break
		}
		line := strings.TrimSpace(strings.SplitN(scanner.Text(), "%", 2)[0])
		if pending != "" {
			line, pending = pending+" "+line, ""
		}
		fields := parseCMapFields(line)
		stopAtEndCMap = false
		for i, token := range fields {
			if token == "endcmap" {
				fields = fields[:i]
				stopAtEndCMap = true
				break
			}
		}
		for i, token := range fields {
			switch token {
			case "beginbfchar":
				section, fields = "bfchar", fields[i+1:]
			case "beginbfrange":
				section, fields = "bfrange", fields[i+1:]
			case "begincidchar":
				section, fields = "cidchar", fields[i+1:]
			case "begincidrange":
				section, fields = "cidrange", fields[i+1:]
			case "beginnotdefrange":
				section, fields = "notdef", fields[i+1:]
			case "begincodespacerange":
				section = "codespace"
				// A following mapping section can share this physical line.
				// Leave its begin operator available to the scan below.
				if slices.Contains(fields[i+1:], "endcodespacerange") {
					continue
				}
				fields = fields[i+1:]
			default:
				continue
			}
			break
		}
		if strings.Contains(line, "[") && !strings.Contains(line, "]") {
			pending = line
			continue
		}
		endSection := false
		for i, token := range fields {
			if strings.HasPrefix(token, "end") && (token == "endbfchar" || token == "endbfrange" || token == "endcidchar" || token == "endcidrange" || token == "endnotdefrange" || token == "endcodespacerange") {
				fields, endSection = fields[:i], true
				break
			}
		}
		if len(fields) == 0 && endSection {
			section = ""
			continue
		}
		if len(fields) == 0 {
			if stopAtEndCMap {
				return out, nil
			}
			continue
		}
		width := 0
		switch section {
		case "bfchar", "cidchar":
			width = 2
		case "bfrange", "cidrange":
			width = 3
		}
		if section == "notdef" || section == "codespace" {
			if endSection {
				section = ""
			}
			continue
		}
		if width != 0 && len(fields)%width != 0 && !strings.Contains(line, "[") {
			complete := len(fields) - len(fields)%width
			if endSection {
				// Playa groups records with choplist and silently drops the
				// incomplete suffix when the section ends.
				fields = fields[:complete]
			} else {
				pending = strings.Join(fields[complete:], " ")
				fields = fields[:complete]
			}
		}
		if section == "bfchar" {
			for i := 0; i+1 < len(fields); i += 2 {
				if !isHexToken(fields[i]) || !isHexToken(fields[i+1]) {
					return nil, fmt.Errorf("playa: invalid ToUnicode bfchar record")
				}
				source, e1 := decodeCMapHex(fields[i])
				dest, e2 := decodeCMapHex(fields[i+1])
				if e1 != nil {
					return nil, fmt.Errorf("playa: invalid ToUnicode bfchar source: %w", e1)
				}
				if len(source) == 0 || len(source) > 4 {
					return nil, fmt.Errorf("playa: invalid ToUnicode bfchar source length")
				}
				if e2 != nil {
					return nil, fmt.Errorf("playa: invalid ToUnicode bfchar value: %w", e2)
				}
				if len(dest) == 0 || len(dest)%2 != 0 {
					return nil, fmt.Errorf("playa: invalid ToUnicode bfchar UTF-16 value")
				}
				out[string(source)] = decodeUTF16(dest)
			}
			if endSection {
				section = ""
			}
			continue
		}
		if section == "cidchar" {
			for i := 0; i+1 < len(fields); i += 2 {
				if !strings.HasPrefix(fields[i], "<") {
					return nil, fmt.Errorf("playa: invalid ToUnicode cidchar source")
				}
				source, sourceErr := decodeCMapHex(fields[i])
				value, valueErr := strconv.Atoi(fields[i+1])
				if sourceErr != nil {
					return nil, fmt.Errorf("playa: invalid ToUnicode cidchar source: %w", sourceErr)
				}
				if len(source) == 0 || len(source) > 4 {
					return nil, fmt.Errorf("playa: invalid ToUnicode cidchar source length")
				}
				if valueErr != nil {
					return nil, fmt.Errorf("playa: invalid ToUnicode cidchar value: %w", valueErr)
				}
				if utf8.ValidRune(rune(value)) {
					out[string(source)] = string(rune(value))
				}
			}
			if endSection {
				section = ""
			}
			continue
		}
		rangeSection := section
		if rangeSection == "" {
			// Keep the historical standalone ParseToUnicode projection: a
			// bare three-token record is treated as a scalar bfrange.
			rangeSection = "bfrange"
		}
		if (rangeSection == "bfrange" || rangeSection == "cidrange") && len(fields) >= 3 && isHexToken(fields[0]) && isHexToken(fields[1]) {
			if err := parseToUnicodeRangeRecords(rangeSection, fields, out, rejectOversizedRange); err != nil {
				return nil, err
			}
			if endSection {
				section = ""
			}
			continue
		}
		if len(fields) < 2 || !isHexToken(fields[0]) || !isHexToken(fields[1]) {
			if endSection {
				section = ""
			}
			continue
		}
		source, e1 := decodeCMapHex(fields[0])
		dest, e2 := decodeCMapHex(fields[1])
		if e1 == nil && e2 == nil && len(dest) > 0 && len(dest)%2 == 0 {
			out[string(source)] = decodeUTF16(dest)
		}
		if endSection {
			section = ""
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("playa: failed to scan ToUnicode data: %w", err)
	}
	return out, nil
}

// parseToUnicodeRangeRecords consumes the object-stream form of range
// records. A CMap is whitespace-delimited rather than line-delimited, so a
// single physical line may contain several scalar records or several arrays.
// The caller has already handled an incomplete non-array suffix; an
// incomplete array is intentionally left unconsumed and follows the existing
// recoverable malformed-input path.
func parseToUnicodeRangeRecords(section string, fields []string, out map[string]string, rejectOversizedRange bool) error {
	for len(fields) >= 3 {
		if !isHexToken(fields[0]) || !isHexToken(fields[1]) {
			return nil
		}
		start, end := fields[0], fields[1]
		if fields[2] != "[" {
			target := fields[2]
			if err := applyToUnicodeRangeRecord(section, start, end, []string{target}, false, out, rejectOversizedRange); err != nil {
				return err
			}
			fields = fields[3:]
			continue
		}

		close := -1
		for i := 3; i < len(fields); i++ {
			if fields[i] == "]" {
				close = i
				break
			}
		}
		if close < 0 {
			return nil
		}
		if err := applyToUnicodeRangeRecord(section, start, end, fields[3:close], true, out, rejectOversizedRange); err != nil {
			return err
		}
		fields = fields[close+1:]
	}
	return nil
}

func applyToUnicodeRangeRecord(section, startToken, endToken string, targets []string, array bool, out map[string]string, rejectOversizedRange bool) error {
	lo, err := parseCMapCodeBytes(startToken)
	if err != nil {
		return fmt.Errorf("playa: invalid ToUnicode range source: %w", err)
	}
	hi, err := parseCMapCodeBytes(endToken)
	if err != nil {
		return fmt.Errorf("playa: invalid ToUnicode range source: %w", err)
	}
	if CompareCode(lo, hi) > 0 {
		return fmt.Errorf("playa: reversed ToUnicode %s", section)
	}
	count, rangeOK := cmapRangeCount(lo, hi)
	if !rangeOK || count > maxToUnicodeRange {
		if rejectOversizedRange {
			return ErrToUnicodeRangeOverflow
		}
		return nil
	}

	if array {
		// Playa uses zip(range(...), values): a short array is usable and
		// excess values are ignored.
		if len(targets) > count {
			targets = targets[:count]
		}
		for i, target := range targets {
			dest, err := decodeCMapHex(target)
			if err != nil {
				return fmt.Errorf("playa: invalid ToUnicode %s value: %w", section, err)
			}
			if len(dest) == 0 || len(dest)%2 != 0 {
				return fmt.Errorf("playa: invalid ToUnicode %s UTF-16 value", section)
			}
			out[string(incrementBytes(lo, i))] = decodeUTF16(dest)
		}
		return nil
	}

	target := targets[0]
	if section == "bfrange" {
		if !isHexToken(target) {
			startUnicode, err := strconv.Atoi(target)
			if err != nil {
				return fmt.Errorf("playa: invalid ToUnicode bfrange value: %w", err)
			}
			if count <= 0 || startUnicode > int(^uint(0)>>1)-(count-1) {
				return fmt.Errorf("playa: ToUnicode bfrange value overflows Unicode")
			}
			lastUnicode := startUnicode + count - 1
			if !utf8.ValidRune(rune(startUnicode)) || !utf8.ValidRune(rune(lastUnicode)) {
				return fmt.Errorf("playa: invalid ToUnicode bfrange Unicode value")
			}
			for i := 0; i < count; i++ {
				out[string(incrementBytes(lo, i))] = string(rune(startUnicode + i))
			}
			return nil
		}
		dest, err := decodeCMapHex(target)
		if err != nil {
			return fmt.Errorf("playa: invalid ToUnicode range value: %w", err)
		}
		if len(dest) == 0 || len(dest)%2 != 0 {
			return fmt.Errorf("playa: invalid ToUnicode bfrange UTF-16 value")
		}
		if _, ok := incrementUTF16Checked(dest, count-1); !ok {
			return fmt.Errorf("playa: ToUnicode bfrange value overflows UTF-16")
		}
		for i := 0; i < count; i++ {
			out[string(incrementBytes(lo, i))] = decodeUTF16(incrementUTF16(dest, i))
		}
		return nil
	}

	startCID, err := strconv.Atoi(target)
	if err != nil {
		return fmt.Errorf("playa: invalid ToUnicode cidrange value: %w", err)
	}
	if !utf8.ValidRune(rune(startCID)) || startCID > int(^uint(0)>>1)-(count-1) || !utf8.ValidRune(rune(startCID+count-1)) {
		return nil
	}
	for i := 0; i < count; i++ {
		out[string(incrementBytes(lo, i))] = string(rune(startCID + i))
	}
	return nil
}

// ParseToUnicode is the uint16 compatibility projection of ParseToUnicodeCodes.
// Wide source codes remain available through ParseToUnicodeCodes and are not
// truncated into a uint16 key.
func ParseToUnicode(data []byte) (map[uint16]string, error) {
	codes, err := ParseToUnicodeCodes(data)
	if err != nil {
		return nil, err
	}
	out := make(map[uint16]string, len(codes))
	for source, value := range codes {
		if len(source) == 0 || len(source) > 2 {
			continue
		}
		var code uint16
		for _, b := range []byte(source) {
			code = code<<8 | uint16(b)
		}
		out[code] = value
	}
	return out, nil
}

func isHexToken(value string) bool { return strings.HasPrefix(value, "<") }

func parseCMapCodeBytes(value string) ([]byte, error) {
	decoded, err := decodeCMapHex(value)
	if err != nil || len(decoded) == 0 || len(decoded) > 4 {
		if err == nil {
			err = fmt.Errorf("invalid CMap code length")
		}
		return nil, err
	}
	return decoded, nil
}

func decodeCMapHex(value string) ([]byte, error) {
	digits := strings.Trim(value, "<>")
	if len(digits)%2 != 0 {
		digits += "0"
	}
	return hex.DecodeString(digits)
}

func parseCMapFields(value string) []string {
	fields := []string{}
	for i := 0; i < len(value); {
		switch value[i] {
		case ' ', '\t', '\r', '\n', '\f':
			i++
		case '<':
			j := i + 1
			for j < len(value) && value[j] != '>' {
				j++
			}
			if j < len(value) {
				j++
			}
			fields = append(fields, value[i:j])
			i = j
		case '[', ']':
			fields = append(fields, value[i:i+1])
			i++
		default:
			j := i + 1
			for j < len(value) && !strings.ContainsRune("<>[] \t\r\n\f", rune(value[j])) {
				j++
			}
			fields = append(fields, value[i:j])
			i = j
		}
	}
	return fields
}

func incrementBytes(value []byte, amount int) []byte {
	out := append([]byte(nil), value...)
	for i := len(out) - 1; i >= 0 && amount > 0; i-- {
		n := int(out[i]) + amount
		out[i] = byte(n)
		amount = n >> 8
	}
	return out
}

func incrementUTF16Checked(value []byte, amount int) ([]byte, bool) {
	out := append([]byte(nil), value...)
	for i := len(out) - 1; i >= 0 && amount > 0; i-- {
		n := int(out[i]) + amount
		out[i] = byte(n)
		amount = n >> 8
	}
	return out, amount == 0
}

func incrementUTF16(value []byte, amount int) []byte {
	out, _ := incrementUTF16Checked(value, amount)
	return out
}

func decodeUTF16(value []byte) string {
	if len(value)%2 != 0 {
		value = value[:len(value)-1]
	}
	runes := make([]rune, 0, len(value)/2)
	for i := 0; i < len(value); i += 2 {
		u := uint16(value[i])<<8 | uint16(value[i+1])
		if u >= 0xd800 && u <= 0xdbff {
			if i+3 < len(value) {
				v := uint16(value[i+2])<<8 | uint16(value[i+3])
				if v >= 0xdc00 && v <= 0xdfff {
					runes = append(runes, rune(0x10000+(uint32(u)-0xd800)*0x400+uint32(v)-0xdc00))
					i += 2
				}
			}
			continue
		}
		if u >= 0xdc00 && u <= 0xdfff {
			continue
		}
		runes = append(runes, rune(u))
	}
	return string(runes)
}
