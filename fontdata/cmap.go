package fontdata

import (
	"encoding/hex"
	"fmt"
	"iter"
	"strconv"
	"strings"

	"github.com/lin-string/go-playa/pdftypes/primitives"
)

type CodeSpace struct {
	low  []byte
	high []byte
}

// NewCodeSpace creates an owned codespace snapshot for internal adapters.
func NewCodeSpace(low, high []byte) CodeSpace {
	return CodeSpace{low: primitives.CloneBytes(low), high: primitives.CloneBytes(high)}
}

type Code struct {
	bytes []byte
	cid   int
}

// NewCode creates an owned decoded-code value for internal adapters.
func NewCode(value []byte, cid int) Code {
	return Code{bytes: primitives.CloneBytes(value), cid: cid}
}

type CMap struct {
	codespaces []CodeSpace
	mapping    map[string]int
	vertical   bool
	useCMap    string
	// shortestCodeSpace selects Playa's EncodingCMap order. Embedded font
	// encodings try the shortest declared code width first; predefined CMaps
	// retain the generic longest-width traversal.
	shortestCodeSpace bool
	// missingCIDZero is set for parsed PDF encoding CMaps. Playa's
	// EncodingCMap maps both unmapped in-range codes and invalid codes to
	// CID 0; identity maps use the source code as their CID instead.
	missingCIDZero bool
}

func cloneCMap(cmap *CMap) *CMap {
	if cmap == nil {
		return nil
	}
	clone := &CMap{
		vertical:          cmap.vertical,
		useCMap:           cmap.useCMap,
		shortestCodeSpace: cmap.shortestCodeSpace,
		missingCIDZero:    cmap.missingCIDZero,
	}
	if cmap.mapping != nil {
		clone.mapping = make(map[string]int, len(cmap.mapping))
		for key, value := range cmap.mapping {
			clone.mapping[key] = value
		}
	}
	if cmap.codespaces != nil {
		clone.codespaces = make([]CodeSpace, len(cmap.codespaces))
		for i, space := range cmap.codespaces {
			clone.codespaces[i] = CodeSpace{low: primitives.CloneBytes(space.low), high: primitives.CloneBytes(space.high)}
		}
	}
	return clone
}

func codeNumber(code []byte) int {
	n := 0
	for _, b := range code {
		n = n<<8 | int(b)
	}
	return n
}

func cmapRangeCount(lo, hi []byte) (int, bool) {
	if len(lo) == 0 || len(lo) != len(hi) {
		return 0, false
	}
	var low, high uint64
	for _, value := range lo {
		low = low<<8 | uint64(value)
	}
	for _, value := range hi {
		high = high<<8 | uint64(value)
	}
	if high < low || high-low >= uint64(^uint(0)>>1) {
		return 0, false
	}
	return int(high-low) + 1, true
}

func (c *CMap) Vertical() bool { return c.vertical }
func (c *CMap) UseCMap() string {
	return c.useCMap
}

func (s CodeSpace) LowCopy() []byte  { return primitives.CloneBytes(s.low) }
func (s CodeSpace) HighCopy() []byte { return primitives.CloneBytes(s.high) }
func (c Code) BytesCopy() []byte     { return primitives.CloneBytes(c.bytes) }

// CID reports the character identifier decoded from the source code.
func (c Code) CID() int { return c.cid }

// Finalize returns an independent snapshot of this code-space range.
func (s CodeSpace) Finalize() CodeSpace {
	return CodeSpace{low: s.LowCopy(), high: s.HighCopy()}
}

// Finalize returns an independent snapshot of this decoded code.
func (c Code) Finalize() Code {
	return Code{bytes: c.BytesCopy(), cid: c.cid}
}

// Finalize returns an independent snapshot of the CMap.
func (c *CMap) Finalize() *CMap { return cloneCMap(c) }
func (c *CMap) MappingCopy() map[string]int {
	if c.mapping == nil {
		return nil
	}
	out := make(map[string]int, len(c.mapping))
	for key, value := range c.mapping {
		out[key] = value
	}
	return out
}

func (c *CMap) mappingValue(code []byte) (int, bool) {
	if c == nil {
		return 0, false
	}
	value, ok := c.mapping[string(code)]
	return value, ok
}

// MappingValue returns one mapping without exposing the backing map.
func MappingValue(c *CMap, code []byte) (int, bool) {
	return c.mappingValue(code)
}

func (c *CMap) cacheBytes() int {
	if c == nil {
		return 0
	}
	size := 256 + len(c.useCMap)
	for key := range c.mapping {
		size += len(key) + 32
	}
	return size
}

// CacheBytes estimates the retained mapping footprint for bounded caches.
func CacheBytes(c *CMap) int {
	return c.cacheBytes()
}
func (c *CMap) CodespacesCopy() []CodeSpace {
	if c.codespaces == nil {
		return nil
	}
	out := make([]CodeSpace, len(c.codespaces))
	for i, space := range c.codespaces {
		out[i] = CodeSpace{low: primitives.CloneBytes(space.low), high: primitives.CloneBytes(space.high)}
	}
	return out
}

// MergeUseCMap applies an upstream CMap as fallback while preserving local
// mappings and codespaces. This models PDF's /UseCMap inheritance without
// aliasing any of the mutable slices or maps.
func (c *CMap) mergeUseCMap(base *CMap) {
	if c == nil || base == nil {
		return
	}
	mapping := map[string]int{}
	for key, value := range base.mapping {
		mapping[key] = value
	}
	for key, value := range c.mapping {
		mapping[key] = value
	}
	c.mapping = mapping
	codespaces := make([]CodeSpace, 0, len(base.codespaces)+len(c.codespaces))
	for _, space := range base.codespaces {
		codespaces = append(codespaces, CodeSpace{low: primitives.CloneBytes(space.low), high: primitives.CloneBytes(space.high)})
	}
	for _, space := range c.codespaces {
		codespaces = append(codespaces, CodeSpace{low: primitives.CloneBytes(space.low), high: primitives.CloneBytes(space.high)})
	}
	c.codespaces = codespaces
	c.vertical = c.vertical || base.vertical
	c.shortestCodeSpace = c.shortestCodeSpace || base.shortestCodeSpace
	c.missingCIDZero = c.missingCIDZero || base.missingCIDZero
}

// MergeUseCMap applies PDF /UseCMap inheritance without exposing the mutable
// implementation fields to packages outside fontdata.
func MergeUseCMap(c, base *CMap) {
	c.mergeUseCMap(base)
}

// NewIdentityCMap creates the built-in one- or two-byte identity maps used by
// PDF fonts. It is an internal resource constructor, not a public facade API.
func NewIdentityCMap(vertical bool, byteWidth int) *CMap {
	if byteWidth < 1 || byteWidth > 4 {
		return nil
	}
	low := make([]byte, byteWidth)
	high := make([]byte, byteWidth)
	for i := range high {
		high[i] = 0xff
	}
	return &CMap{
		codespaces: []CodeSpace{{low: low, high: high}},
		mapping:    map[string]int{},
		vertical:   vertical,
	}
}

// NewCMap creates an owned CMap from already-owned internal values. It is
// used by migration tests and core adapters that construct synthetic maps.
func NewCMap(codespaces []CodeSpace, mapping map[string]int, vertical bool, useCMap string) *CMap {
	c := &CMap{vertical: vertical, useCMap: useCMap}
	if codespaces != nil {
		c.codespaces = make([]CodeSpace, len(codespaces))
		for i, space := range codespaces {
			c.codespaces[i] = space.Finalize()
		}
	}
	if mapping != nil {
		c.mapping = make(map[string]int, len(mapping))
		for key, value := range mapping {
			c.mapping[key] = value
		}
	}
	return c
}

func ParseCMap(data []byte) (*CMap, error) {
	c := &CMap{mapping: map[string]int{}, missingCIDZero: true}
	reader := cmapTokenReader{data: data}
	section := ""
	parsedRecord := false
	for {
		token, ok := reader.next()
		if !ok {
			break
		}
		if token == "/WMode" {
			value, valueOK := reader.next()
			definition, definitionOK := reader.next()
			if definitionOK && definition == "def" {
				if number, err := strconv.Atoi(value); valueOK && err == nil {
					// Playa's CMapBase considers every non-zero WMode
					// value vertical, including malformed-but-numeric values,
					// but only after a complete `def` declaration.
					c.vertical = number != 0
				}
			} else if definitionOK {
				reader.unread(definition)
			}
			continue
		}
		if strings.HasPrefix(token, "/") {
			if next, ok := reader.next(); ok {
				if next == "usecmap" {
					c.useCMap = strings.TrimPrefix(token, "/")
					continue
				}
				reader.unread(next)
			}
			continue
		}
		switch token {
		case "begincodespacerange", "begincidchar", "begincidrange":
			section = token
			continue
		case "endcodespacerange", "endcidchar", "endcidrange":
			if section == "" {
				return nil, fmt.Errorf("playa: unexpected CMap section end %s", token)
			}
			if section != "begin"+token[3:] {
				return nil, fmt.Errorf("playa: mismatched CMap section end %s for %s", token, section)
			}
			section = ""
			continue
		}
		if section == "" {
			continue
		}
		width := 2
		if section == "begincidrange" {
			width = 3
		}
		var operands [3]string
		operands[0] = token
		for index := 1; index < width; index++ {
			operand, ok := reader.next()
			if !ok {
				if reader.err != nil {
					return nil, reader.err
				}
				// Playa groups section operands with choplist, so a trailing
				// incomplete tuple is ignored at EOF.
				if !parsedRecord {
					return nil, fmt.Errorf("playa: truncated CMap %s record", section)
				}
				return c, nil
			}
			if isCMapSectionBoundary(operand) {
				// A section boundary closes the current section and discards
				// an incomplete tuple, just like Playa's stack-based parser.
				if strings.HasPrefix(operand, "end") {
					if section != "begin"+operand[3:] {
						return nil, fmt.Errorf("playa: mismatched CMap section end %s for %s", operand, section)
					}
					section = ""
					if !parsedRecord {
						return nil, fmt.Errorf("playa: truncated CMap %s record", section)
					}
					break
				}
				section = ""
				reader.unread(operand)
				break
			}
			operands[index] = operand
		}
		if section == "" {
			continue
		}
		switch section {
		case "begincodespacerange":
			lo, err1 := cmapHex(operands[0])
			hi, err2 := cmapHex(operands[1])
			if err1 != nil || err2 != nil || len(lo) != len(hi) || compareCode(lo, hi) > 0 {
				return nil, fmt.Errorf("playa: invalid CMap codespace")
			}
			c.codespaces = append(c.codespaces, CodeSpace{low: lo, high: hi})
			parsedRecord = true
		case "begincidchar":
			code, err := cmapHex(operands[0])
			cid, err2 := strconv.Atoi(operands[1])
			if err != nil || err2 != nil || cid < 0 {
				return nil, fmt.Errorf("playa: invalid CMap cidchar")
			}
			c.mapping[string(code)] = cid
			parsedRecord = true
		case "begincidrange":
			lo, err1 := cmapHex(operands[0])
			hi, err2 := cmapHex(operands[1])
			start, err3 := strconv.Atoi(operands[2])
			if err1 != nil || err2 != nil || err3 != nil || start < 0 || len(lo) != len(hi) {
				return nil, fmt.Errorf("playa: invalid CMap cidrange")
			}
			count, ok := cmapRangeCount(lo, hi)
			if !ok && compareCode(lo, hi) > 0 {
				return nil, fmt.Errorf("playa: invalid CMap cidrange")
			}
			if ok && count <= maxCMapRange && (count == 0 || start <= int(^uint(0)>>1)-(count-1)) {
				for delta := 0; delta < count; delta++ {
					c.mapping[string(incrementCode(lo, delta))] = start + delta
				}
			} else if ok {
				return nil, fmt.Errorf("playa: CMap cidrange overflows")
			}
			parsedRecord = true
		}
	}
	if reader.err != nil {
		return nil, reader.err
	}
	return c, nil
}

func isCMapSectionBoundary(token string) bool {
	switch token {
	case "begincodespacerange", "begincidchar", "begincidrange",
		"endcodespacerange", "endcidchar", "endcidrange":
		return true
	default:
		return false
	}
}

// ParseEncodingCMap parses a PDF font Encoding CMap. Playa's EncodingCMap
// tries declared code-space widths in ascending order, unlike predefined
// CMaps whose trie traversal prefers a complete longer code.
func ParseEncodingCMap(data []byte) (*CMap, error) {
	cmap, err := ParseCMap(data)
	if err != nil {
		return nil, err
	}
	cmap.shortestCodeSpace = true
	// Playa's parse_encoding explicitly discards an embedded usecmap
	// directive; only generic/predefined CMap loading resolves inheritance.
	cmap.useCMap = ""
	return cmap, nil
}

// ParseCodeSpaces extracts and validates only codespace ranges. ToUnicode
// streams may contain mapping ranges that are intentionally too large to
// materialize; those mappings must not prevent their valid codespaces from
// being used for bounded decoding.
func ParseCodeSpaces(data []byte) ([]CodeSpace, error) {
	return parseCodeSpaces(data, false)
}

func parseToUnicodeCodeSpaces(data []byte) ([]CodeSpace, error) {
	return parseCodeSpaces(data, true)
}

func parseCodeSpaces(data []byte, tolerateMalformed bool) ([]CodeSpace, error) {
	reader := cmapTokenReader{data: data}
	section := ""
	spaces := []CodeSpace{}
	for {
		token, ok := reader.next()
		if !ok {
			break
		}
		switch token {
		case "endcmap":
			return spaces, nil
		case "begincodespacerange":
			section = token
			continue
		case "endcodespacerange":
			if section != "begincodespacerange" {
				return nil, fmt.Errorf("playa: unexpected CMap section end %s", token)
			}
			section = ""
			continue
		}
		if section != "begincodespacerange" {
			continue
		}
		high, ok := reader.next()
		if !ok {
			if reader.err != nil {
				return nil, reader.err
			}
			// Playa's choplist drops an incomplete trailing codespace.
			if len(spaces) == 0 && !tolerateMalformed {
				return nil, fmt.Errorf("playa: truncated CMap codespace record")
			}
			return spaces, nil
		}
		if isCMapSectionBoundary(high) {
			// Do not interpret a section boundary as the second operand of
			// an incomplete codespace record.
			if strings.HasPrefix(high, "end") {
				if section != "begincodespacerange" {
					return nil, fmt.Errorf("playa: unexpected CMap section end %s", high)
				}
				section = ""
				if len(spaces) == 0 && !tolerateMalformed {
					return nil, fmt.Errorf("playa: truncated CMap codespace record")
				}
				continue
			}
			section = ""
			reader.unread(high)
			continue
		}
		lowBytes, lowErr := cmapHex(token)
		highBytes, highErr := cmapHex(high)
		if lowErr != nil || highErr != nil {
			return nil, fmt.Errorf("playa: invalid CMap codespace")
		}
		if len(lowBytes) != len(highBytes) || compareCode(lowBytes, highBytes) > 0 {
			if tolerateMalformed {
				continue
			}
			return nil, fmt.Errorf("playa: invalid CMap codespace")
		}
		spaces = append(spaces, NewCodeSpace(lowBytes, highBytes))
	}
	if reader.err != nil {
		return nil, reader.err
	}
	return spaces, nil
}

const maxCMapRange = 1 << 20
const maxCMapTokenSize = 4 << 20

type cmapTokenReader struct {
	data       []byte
	pos        int
	pending    string
	hasPending bool
	err        error
}

func (r *cmapTokenReader) next() (string, bool) {
	if r.err != nil {
		return "", false
	}
	if r.hasPending {
		r.hasPending = false
		return r.pending, true
	}
	for r.pos < len(r.data) {
		switch r.data[r.pos] {
		case ' ', '\t', '\r', '\n', '\f':
			r.pos++
		case '%':
			for r.pos < len(r.data) && r.data[r.pos] != '\n' {
				r.pos++
			}
		default:
			goto token
		}
	}
	return "", false

token:
	start := r.pos
	switch r.data[r.pos] {
	case '<':
		r.pos++
		for r.pos < len(r.data) && r.data[r.pos] != '>' {
			r.pos++
		}
		if r.pos >= len(r.data) {
			r.err = fmt.Errorf("playa: unterminated CMap hex token")
			return "", false
		}
		r.pos++
		if r.pos-start > maxCMapTokenSize {
			r.err = fmt.Errorf("playa: CMap token exceeds %d bytes", maxCMapTokenSize)
			return "", false
		}
	case '[', ']':
		r.pos++
	default:
		for r.pos < len(r.data) {
			switch r.data[r.pos] {
			case ' ', '\t', '\r', '\n', '\f', '<', '[', ']':
				if r.pos-start > maxCMapTokenSize {
					r.err = fmt.Errorf("playa: CMap token exceeds %d bytes", maxCMapTokenSize)
					return "", false
				}
				return string(r.data[start:r.pos]), true
			default:
				r.pos++
			}
		}
		if r.pos-start > maxCMapTokenSize {
			r.err = fmt.Errorf("playa: CMap token exceeds %d bytes", maxCMapTokenSize)
			return "", false
		}
	}
	return string(r.data[start:r.pos]), true
}

func (r *cmapTokenReader) unread(token string) {
	r.pending = token
	r.hasPending = true
}

func cmapHex(s string) ([]byte, error) {
	s = strings.Trim(s, "<>")
	if len(s)%2 != 0 {
		s += "0"
	}
	b, err := hex.DecodeString(s)
	if err != nil || len(b) == 0 || len(b) > 4 {
		if err == nil {
			err = fmt.Errorf("invalid CMap code length")
		}
	}
	return b, err
}

func incrementCode(src []byte, delta int) []byte {
	out := append([]byte(nil), src...)
	for i := len(out) - 1; i >= 0 && delta > 0; i-- {
		n := int(out[i]) + (delta & 0xff)
		out[i] = byte(n)
		delta = (delta >> 8) + n/256
	}
	return out
}

type cmapDecoder struct {
	cmap         *CMap
	data         []byte
	pos          int
	lengthBuffer [8]int
	lengthCount  int
	extraLengths []int
}

func newCMapDecoder(c *CMap, data []byte) cmapDecoder {
	d := cmapDecoder{cmap: c, data: data}
	for _, space := range c.codespaces {
		length := len(space.low)
		if length == 0 || len(space.high) != length {
			continue
		}
		seen := false
		for i := 0; i < d.lengthCount; i++ {
			if d.lengthAt(i) == length {
				seen = true
				break
			}
		}
		if seen {
			continue
		}
		if d.lengthCount == len(d.lengthBuffer) {
			if d.extraLengths == nil {
				d.extraLengths = append([]int(nil), d.lengthBuffer[:]...)
			}
			d.extraLengths = append(d.extraLengths, length)
			insertAt := len(d.extraLengths) - 1
			for insertAt > 0 && cmapLengthBefore(d.cmap, d.extraLengths[insertAt-1], length) {
				d.extraLengths[insertAt] = d.extraLengths[insertAt-1]
				insertAt--
			}
			d.extraLengths[insertAt] = length
			d.lengthCount++
			continue
		}
		insertAt := d.lengthCount
		for insertAt > 0 && cmapLengthBefore(d.cmap, d.lengthBuffer[insertAt-1], length) {
			d.lengthBuffer[insertAt] = d.lengthBuffer[insertAt-1]
			insertAt--
		}
		d.lengthBuffer[insertAt] = length
		d.lengthCount++
	}
	return d
}

func cmapLengthBefore(cmap *CMap, previous, next int) bool {
	if cmap != nil && cmap.shortestCodeSpace {
		return previous > next
	}
	return previous < next
}

// DecodeCMap streams decoded source codes and CIDs without exposing the
// decoder's mutable cursor to callers.
// yield is required and must be non-nil.
func DecodeCMap(c *CMap, data []byte, yield func([]byte, int) bool) bool {
	if c == nil {
		return false
	}
	decoder := newCMapDecoder(c, data)
	for {
		code, cid, ok := decoder.next()
		if !ok {
			return true
		}
		if !yield(code, cid) {
			return false
		}
	}
}

func (d *cmapDecoder) lengthAt(index int) int {
	if d.extraLengths != nil {
		return d.extraLengths[index]
	}
	return d.lengthBuffer[index]
}

func (d *cmapDecoder) next() ([]byte, int, bool) {
	if d.pos >= len(d.data) {
		return nil, 0, false
	}
	for i := 0; i < d.lengthCount; i++ {
		n := d.lengthAt(i)
		if d.pos+n > len(d.data) || !d.cmap.inCodespace(d.data[d.pos:d.pos+n]) {
			continue
		}
		code := d.data[d.pos : d.pos+n]
		cid, ok := d.cmap.mapping[string(code)]
		if !ok {
			if d.cmap.missingCIDZero {
				cid = 0
			} else {
				cid = codeNumber(code)
			}
		}
		d.pos += n
		return code, cid, true
	}
	code := d.data[d.pos : d.pos+1]
	cid, ok := d.cmap.mapping[string(code)]
	if !ok {
		if len(d.cmap.codespaces) == 0 {
			// Base CMaps use a trie and discard bytes that do not
			// begin a mapped code. EncodingCMap inputs always carry
			// explicit codespaces and retain their invalid-code fallback.
			d.pos++
			return d.next()
		}
		if d.cmap.missingCIDZero {
			cid = 0
		} else {
			cid = int(code[0])
		}
	}
	d.pos++
	return code, cid, true
}

func (c *CMap) Decode(data []byte) []Code {
	// Returned code bytes borrow data. Call BytesCopy or Finalize when the
	// decoded codes must outlive the input buffer or the iterator step.
	decoder := newCMapDecoder(c, data)
	out := make([]Code, 0, len(data))
	for {
		code, cid, ok := decoder.next()
		if !ok {
			return out
		}
		out = append(out, Code{bytes: code, cid: cid})
	}
}

// DecodeSeq lazily decodes bytes into borrowed Code values. The sequence is
// repeatable and stops decoding as soon as yield returns false. Code.Bytes
// borrows data; call Code.BytesCopy or Code.Finalize before retaining a value
// beyond the current sequence step.
func (c *CMap) DecodeSeq(data []byte) iter.Seq[Code] {
	return func(yield func(Code) bool) {
		decoder := newCMapDecoder(c, data)
		for {
			bytes, cid, ok := decoder.next()
			if !ok || !yield(Code{bytes: bytes, cid: cid}) {
				return
			}
		}
	}
}

func (c *CMap) inCodespace(code []byte) bool {
	for _, space := range c.codespaces {
		if len(code) != len(space.low) || len(code) != len(space.high) {
			continue
		}
		if compareCode(code, space.low) >= 0 && compareCode(code, space.high) <= 0 {
			return true
		}
	}
	return false
}

func compareCode(a, b []byte) int {
	for i := range a {
		if a[i] < b[i] {
			return -1
		}
		if a[i] > b[i] {
			return 1
		}
	}
	return 0
}

// CompareCode compares equal-width CMap byte codes lexicographically.
func CompareCode(a, b []byte) int {
	return compareCode(a, b)
}

func cmapCodeLengths(spaces []CodeSpace) []int {
	seen := [5]bool{}
	for _, space := range spaces {
		if len(space.low) == len(space.high) && len(space.low) > 0 && len(space.low) < len(seen) {
			seen[len(space.low)] = true
		}
	}
	out := make([]int, 0, len(seen)-1)
	for length := len(seen) - 1; length > 0; length-- {
		if seen[length] {
			out = append(out, length)
		}
	}
	return out
}

// CodeLengths returns candidate source-code widths in descending order.
func CodeLengths(spaces []CodeSpace) []int {
	return cmapCodeLengths(spaces)
}

func cmapCodeInSpaces(code []byte, spaces []CodeSpace) bool {
	for _, space := range spaces {
		if len(code) != len(space.low) || len(code) != len(space.high) {
			continue
		}
		if compareCode(code, space.low) >= 0 && compareCode(code, space.high) <= 0 {
			return true
		}
	}
	return false
}

// CodeInSpaces reports whether a source code belongs to any codespace.
func CodeInSpaces(code []byte, spaces []CodeSpace) bool {
	return cmapCodeInSpaces(code, spaces)
}
