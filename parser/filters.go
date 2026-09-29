package parser

import (
	"bytes"
	"compress/flate"
	"compress/zlib"
	"encoding/ascii85"
	"errors"
	"fmt"
	"io"

	"golang.org/x/image/ccitt"
)

const decodedFilterExpansionLimit = 256 << 20

var errDecodedFilterOutputLimit = errors.New("decoded filter output exceeds limit")

// DecodeFilters applies the PDF stream filter chain in the same order as the
// /Filter array. Predictor handling is kept explicit for the image/text
// decoders that will be added with the content interpreter.
func DecodeFilters(data []byte, filters []string, parms []Dict) ([]byte, error) {
	return DecodeFiltersLimited(data, filters, parms, 0)
}

// DecodeFiltersLimited applies a hard decoded-output limit to a filter chain.
// A zero limit preserves DecodeFilters' historical unbounded behavior.
func DecodeFiltersLimited(data []byte, filters []string, parms []Dict, limit int) ([]byte, error) {
	return decodeFiltersLimited(data, filters, parms, limit, false)
}

// DecodeFiltersLenientLimited applies a hard decoded-output limit while
// retaining the prefix produced before malformed Flate/LZW data fails. This
// is the non-strict ContentStream.decode behavior used by Playa. Unknown
// filters, malformed parameters, predictor failures, and output-limit
// violations remain errors.
func DecodeFiltersLenientLimited(data []byte, filters []string, parms []Dict, limit int) ([]byte, error) {
	return decodeFiltersLimited(data, filters, parms, limit, true)
}

func decodeFiltersLimited(data []byte, filters []string, parms []Dict, limit int, lenientFilters bool) ([]byte, error) {
	var err error
	for i, name := range filters {
		var p Dict
		if i < len(parms) {
			p = parms[i]
		}
		switch name {
		case "", "Identity":
		case "FlateDecode", "Fl":
			encoded := data
			data, err = flateDecodeLimited(encoded, limit)
			if err != nil && lenientFilters {
				if recovered, recoverErr := flateDecodeCorruptedLimited(encoded, limit); recoverErr == nil {
					data, err = recovered, nil
				}
			}
		case "ASCIIHexDecode", "AHx":
			data, err = asciiHexDecodeLimited(data, limit)
		case "ASCII85Decode", "A85":
			data, err = ascii85DecodeLimited(data, limit)
		case "RunLengthDecode", "RL":
			data, err = runLengthDecodeLimited(data, limit)
		case "LZWDecode", "LZW":
			ec := 1
			if p != nil {
				if x, ok := IntValue(p[Name("EarlyChange")]); ok {
					if x != 0 && x != 1 {
						return nil, WithContext(fmt.Errorf("playa: invalid LZW EarlyChange %d", x), -1, "filter "+name)
					}
					ec = x
				}
			}
			data, err = lzwDecodeLimited(data, ec, limit)
			if err != nil && lenientFilters && data != nil && !errors.Is(err, errDecodedFilterOutputLimit) {
				err = nil
			}
		case "CCITTFaxDecode", "CCF":
			data, err = ccittFaxDecode(data, p)
		case "DCTDecode", "DCT":
			if !lenientFilters && (len(data) < 2 || data[0] != 0xff || data[1] != 0xd8) {
				err = fmt.Errorf("invalid JPEG data: missing SOI marker")
			}
			// DCT data remains self-contained after the PDF filter is
			// validated. Playa exposes those bytes unchanged to image writers.
		case "JPXDecode", "JBIG2Decode":
			// These are already self-contained image encodings. Playa exposes
			// their stream bytes unchanged to the image writer as well.
			// CCITT remains a separate bitmap decoder and is not silently treated
			// as raw data.
		default:
			return nil, WithContext(fmt.Errorf("playa: unsupported stream filter %q", name), -1, "filter "+name)
		}
		if err != nil {
			return nil, WithContext(fmt.Errorf("playa: filter %s: %w", name, err), -1, "filter "+name)
		}
		if name == "FlateDecode" || name == "Fl" || name == "LZWDecode" || name == "LZW" {
			if p != nil {
				var pe error
				if data, pe = decodePredictorLimited(data, p, limit); pe != nil {
					return nil, WithContext(pe, -1, "filter "+name+" predictor")
				}
			}
		}
		if limit > 0 && len(data) > limit {
			return nil, WithContext(fmt.Errorf("playa: decoded filter output exceeds %d bytes", limit), -1, "filter "+name)
		}
		_ = p
	}
	return data, nil
}

func ccittFaxDecode(data []byte, parms Dict) ([]byte, error) {
	k := 0
	damagedRowsBeforeError := 0
	if value, present := parms[Name("K")]; present {
		parsed, ok := IntValue(value)
		if !ok {
			return nil, fmt.Errorf("playa: invalid CCITTFaxDecode K")
		}
		k = parsed
	}
	format := ccitt.Group4
	if k == 0 {
		format = ccitt.Group3
	}

	columns := 1728
	if value, present := parms[Name("Columns")]; present {
		parsed, ok := IntValue(value)
		if !ok || parsed <= 0 {
			return nil, fmt.Errorf("playa: invalid CCITTFaxDecode Columns")
		}
		columns = parsed
	}
	rows := ccitt.AutoDetectHeight
	if value, present := parms[Name("Rows")]; present {
		parsed, ok := IntValue(value)
		if !ok || parsed < 0 {
			return nil, fmt.Errorf("playa: invalid CCITTFaxDecode Rows")
		}
		if parsed > 0 {
			rows = parsed
		}
	}
	options := ccitt.Options{}
	if value, present := parms[Name("EncodedByteAlign")]; present {
		parsed, ok := value.(Bool)
		if !ok {
			return nil, fmt.Errorf("playa: invalid CCITTFaxDecode EncodedByteAlign")
		}
		options.Align = bool(parsed)
	}
	if value, present := parms[Name("BlackIs1")]; present {
		parsed, ok := value.(Bool)
		if !ok {
			return nil, fmt.Errorf("playa: invalid CCITTFaxDecode BlackIs1")
		}
		options.Invert = bool(parsed)
	}
	if value, present := parms[Name("EndOfLine")]; present {
		_, ok := value.(Bool)
		if !ok {
			return nil, fmt.Errorf("playa: invalid CCITTFaxDecode EndOfLine")
		}
	}
	if value, present := parms[Name("EndOfBlock")]; present {
		parsed, ok := value.(Bool)
		if !ok {
			return nil, fmt.Errorf("playa: invalid CCITTFaxDecode EndOfBlock")
		}
		if !bool(parsed) && rows == ccitt.AutoDetectHeight {
			return nil, fmt.Errorf("playa: CCITTFaxDecode requires Rows when EndOfBlock is false")
		}
	}
	if value, present := parms[Name("DamagedRowsBeforeError")]; present {
		parsed, ok := IntValue(value)
		if !ok || parsed < 0 {
			return nil, fmt.Errorf("playa: invalid CCITTFaxDecode DamagedRowsBeforeError")
		}
		damagedRowsBeforeError = parsed
	}

	const maxDecodedBytes = 256 << 20
	maxInt := int(^uint(0) >> 1)
	if columns > maxDecodedBytes*8 || columns > maxInt-7 {
		return nil, fmt.Errorf("playa: CCITTFaxDecode output exceeds %d bytes", maxDecodedBytes)
	}
	rowBytes := (columns + 7) / 8
	if rowBytes <= 0 || rowBytes > maxDecodedBytes || (rows != ccitt.AutoDetectHeight && rows > maxDecodedBytes/rowBytes) {
		return nil, fmt.Errorf("playa: CCITTFaxDecode output exceeds %d bytes", maxDecodedBytes)
	}
	limit := maxDecodedBytes
	if rows != ccitt.AutoDetectHeight {
		limit = rowBytes * rows
	}
	if k > 0 {
		decoded, err := ccittMixedFaxDecode(data, columns, rows, options.Align, options.Invert)
		if err != nil {
			if damagedRowsBeforeError > 0 {
				if len(decoded) > limit {
					return nil, fmt.Errorf("playa: CCITTFaxDecode output exceeds %d bytes", limit)
				}
				return decoded, nil
			}
			return nil, err
		}
		if len(decoded) > limit {
			return nil, fmt.Errorf("playa: CCITTFaxDecode output exceeds %d bytes", limit)
		}
		return decoded, nil
	}
	reader := ccitt.NewReader(bytes.NewReader(data), ccitt.MSB, format, columns, rows, &options)
	decoded, err := io.ReadAll(io.LimitReader(reader, int64(limit)+1))
	if err != nil {
		if damagedRowsBeforeError > 0 {
			if len(decoded) > limit {
				return nil, fmt.Errorf("playa: CCITTFaxDecode output exceeds %d bytes", limit)
			}
			return decoded, nil
		}
		return nil, err
	}
	if len(decoded) > limit {
		return nil, fmt.Errorf("playa: CCITTFaxDecode output exceeds %d bytes", limit)
	}
	return decoded, nil
}

func lzwDecodeLimited(data []byte, early, limit int) ([]byte, error) {
	dict := make([][]byte, 4096)
	reset := func() {
		for i := range dict {
			dict[i] = nil
		}
		for i := 0; i < 256; i++ {
			dict[i] = []byte{byte(i)}
		}
	}
	reset()
	size := 9
	next := 258
	clear := 256
	eoi := 257
	bit := 0
	read := func(n int) (int, bool) {
		if bit+n > len(data)*8 {
			return 0, false
		}
		v := 0
		for i := 0; i < n; i++ {
			v = (v << 1) | int((data[(bit+i)/8]>>(7-uint((bit+i)%8)))&1)
		}
		bit += n
		return v, true
	}
	out := []byte{}
	var prev []byte
	for {
		code, ok := read(size)
		if !ok {
			return out, nil
		}
		if code == clear {
			reset()
			size = 9
			next = 258
			prev = nil
			continue
		}
		if code == eoi {
			return out, nil
		}
		var entry []byte
		if code < next && dict[code] != nil {
			entry = dict[code]
		} else if code == next && prev != nil {
			entry = append(append([]byte(nil), prev...), prev[0])
		} else {
			return out, fmt.Errorf("playa: invalid LZW code")
		}
		if limit > 0 && len(entry) > limit-len(out) {
			return nil, fmt.Errorf("%w: %d bytes", errDecodedFilterOutputLimit, limit)
		}
		out = append(out, entry...)
		if prev != nil {
			dict[next] = append(append([]byte(nil), prev...), entry[0])
			next++
			if size < 12 && next+early == (1<<size) {
				size++
			}
		}
		prev = entry
	}
}

func decodePredictor(data []byte, p Dict) ([]byte, error) {
	return decodePredictorLimited(data, p, 0)
}

func decodePredictorLimited(data []byte, p Dict, limit int) ([]byte, error) {
	if limit > 0 && len(data) > limit {
		return nil, fmt.Errorf("playa: decoded filter output exceeds %d bytes", limit)
	}
	pred := 1
	if raw, present := p[Name("Predictor")]; present {
		value, ok := IntValue(raw)
		if !ok {
			return nil, fmt.Errorf("playa: invalid predictor value")
		}
		pred = value
	}
	if pred <= 1 {
		return data, nil
	}
	if pred != 2 && (pred < 10 || pred > 15) {
		return nil, fmt.Errorf("playa: unsupported predictor %d", pred)
	}
	cols, err := predictorPositiveInt(p, "Columns", 1)
	if err != nil {
		return nil, err
	}
	colors, err := predictorPositiveInt(p, "Colors", 1)
	if err != nil {
		return nil, err
	}
	bpc, err := predictorPositiveInt(p, "BitsPerComponent", 8)
	if err != nil {
		return nil, err
	}
	if bpc != 1 && bpc != 2 && bpc != 4 && bpc != 8 && bpc != 16 {
		return nil, fmt.Errorf("playa: invalid predictor BitsPerComponent %d", bpc)
	}
	maxInt := int(^uint(0) >> 1)
	if colors > maxInt/bpc {
		return nil, fmt.Errorf("playa: predictor component width overflow")
	}
	componentBits := colors * bpc
	if cols > maxInt/componentBits {
		return nil, fmt.Errorf("playa: predictor row width overflow")
	}
	rowBits := cols * componentBits
	if rowBits > maxInt-7 {
		return nil, fmt.Errorf("playa: predictor row width overflow")
	}
	bpp := (componentBits + 7) / 8
	row := (rowBits + 7) / 8
	if bpp <= 0 || row <= 0 {
		return nil, fmt.Errorf("playa: invalid predictor row parameters")
	}
	if len(data) > 0 && row > len(data) {
		return nil, fmt.Errorf("playa: truncated predictor row")
	}
	if pred == 2 {
		return tiffPredict(data, row, bpp), nil
	}
	if pred < 10 {
		return data, nil
	}
	stride := row
	out := make([]byte, 0, len(data))
	var prev []byte
	for i := 0; i+stride < len(data)+1; {
		if i >= len(data) {
			break
		}
		filter := int(data[i])
		i++
		if i+stride > len(data) {
			return nil, fmt.Errorf("playa: truncated PNG predictor")
		}
		cur := append([]byte(nil), data[i:i+stride]...)
		i += stride
		for x := 0; x < len(cur); x++ {
			left := byte(0)
			if x >= bpp {
				left = cur[x-bpp]
			}
			up := byte(0)
			if prev != nil {
				up = prev[x]
			}
			ul := byte(0)
			if prev != nil && x >= bpp {
				ul = prev[x-bpp]
			}
			switch filter {
			case 0:
				// PNG filter None: keep the row bytes unchanged.
			case 1:
				cur[x] += left
			case 2:
				cur[x] += up
			case 3:
				cur[x] += byte((int(left) + int(up)) / 2)
			case 4:
				cur[x] += paeth(left, up, ul)
			default:
				return nil, fmt.Errorf("playa: unsupported PNG predictor filter %d", filter)
			}
		}
		out = append(out, cur...)
		prev = cur
	}
	return out, nil
}

func predictorPositiveInt(p Dict, key string, fallback int) (int, error) {
	raw, present := p[Name(key)]
	if !present {
		return fallback, nil
	}
	value, ok := IntValue(raw)
	if !ok || value <= 0 {
		return 0, fmt.Errorf("playa: invalid predictor %s", key)
	}
	return value, nil
}
func tiffPredict(data []byte, row, bpp int) []byte {
	out := append([]byte(nil), data...)
	for i := 0; i < len(out); i++ {
		if i%row >= bpp {
			out[i] += out[i-bpp]
		}
	}
	return out
}
func paeth(a, b, c byte) byte {
	p := int(a) + int(b) - int(c)
	pa := p - int(a)
	if pa < 0 {
		pa = -pa
	}
	pb := p - int(b)
	if pb < 0 {
		pb = -pb
	}
	pc := p - int(c)
	if pc < 0 {
		pc = -pc
	}
	if pa <= pb && pa <= pc {
		return a
	}
	if pb <= pc {
		return b
	}
	return c
}
func flateDecodeLimited(b []byte, limit int) ([]byte, error) {
	r, e := zlib.NewReader(bytesReader(b))
	if e != nil {
		r2 := flate.NewReader(bytesReader(b))
		defer func() { _ = r2.Close() }()
		return readFilterOutputHint(r2, limit, flateOutputCapacityHint(len(b), limit))
	}
	defer func() { _ = r.Close() }()
	return readFilterOutputHint(r, limit, flateOutputCapacityHint(len(b), limit))
}

func flateOutputCapacityHint(encodedLength, limit int) int {
	const maxInitialCapacity = 8 << 20
	if encodedLength <= 0 {
		return 0
	}
	capacity := encodedLength
	if encodedLength <= int(^uint(0)>>1)/4 {
		capacity *= 4
	}
	if capacity > maxInitialCapacity {
		capacity = maxInitialCapacity
	}
	if limit > 0 && capacity > limit {
		capacity = limit
	}
	return capacity
}

func flateDecodeCorruptedLimited(b []byte, limit int) ([]byte, error) {
	r, err := zlib.NewReader(bytesReader(b))
	if err == nil {
		defer func() { _ = r.Close() }()
		return readFilterOutputCorrupted(r, limit)
	}
	raw := flate.NewReader(bytesReader(b))
	defer func() { _ = raw.Close() }()
	return readFilterOutputCorrupted(raw, limit)
}

func readFilterOutputCorrupted(reader io.Reader, limit int) ([]byte, error) {
	if limit <= 0 {
		data, _ := io.ReadAll(reader)
		return data, nil
	}
	data, err := io.ReadAll(io.LimitReader(reader, int64(limit)+1))
	if len(data) > limit {
		return nil, fmt.Errorf("playa: decoded filter output exceeds %d bytes", limit)
	}
	if err != nil {
		// Playa's non-strict Flate path deliberately keeps all bytes emitted
		// before zlib reports corruption or an incomplete trailer.
		return data, nil
	}
	return data, nil
}

func readFilterOutputHint(reader io.Reader, limit, capacity int) ([]byte, error) {
	var output bytes.Buffer
	if capacity > 0 {
		output.Grow(capacity)
	}
	if limit <= 0 {
		_, err := output.ReadFrom(reader)
		return output.Bytes(), err
	}
	readLimit := int64(limit)
	if limit < int(^uint(0)>>1) {
		readLimit++
	}
	_, err := output.ReadFrom(io.LimitReader(reader, readLimit))
	if err != nil {
		return nil, err
	}
	data := output.Bytes()
	if len(data) > limit {
		return nil, fmt.Errorf("playa: decoded filter output exceeds %d bytes", limit)
	}
	return data, nil
}

type byteReader struct {
	b []byte
	i int
}

func (r *byteReader) Read(p []byte) (int, error) {
	if r.i == len(r.b) {
		return 0, io.EOF
	}
	n := copy(p, r.b[r.i:])
	r.i += n
	return n, nil
}
func bytesReader(b []byte) io.Reader { return &byteReader{b: b} }
func asciiHexDecode(b []byte) ([]byte, error) {
	return asciiHexDecodeLimited(b, 0)
}

func asciiHexDecodeLimited(b []byte, limit int) ([]byte, error) {
	capacity := len(b) / 2
	if limit > 0 && capacity > limit {
		capacity = limit
	}
	out := make([]byte, 0, capacity)
	var nibble byte
	hasNibble := false
	for _, c := range b {
		if c == '>' {
			break
		}
		if c == 0 || c == ' ' || c == '\n' || c == '\r' || c == '\t' || c == '\f' {
			continue
		}
		value, ok := asciiHexNibble(c)
		if !ok {
			return nil, fmt.Errorf("encoding/hex: invalid byte: %q", c)
		}
		if !hasNibble {
			nibble = value
			hasNibble = true
			continue
		}
		if limit > 0 && len(out) == limit {
			return nil, fmt.Errorf("playa: decoded filter output exceeds %d bytes", limit)
		}
		out = append(out, byte(nibble<<4|value))
		hasNibble = false
	}
	if hasNibble {
		if limit > 0 && len(out) == limit {
			return nil, fmt.Errorf("playa: decoded filter output exceeds %d bytes", limit)
		}
		out = append(out, byte(nibble<<4))
	}
	return out, nil
}

func asciiHexNibble(value byte) (byte, bool) {
	switch {
	case value >= '0' && value <= '9':
		return value - '0', true
	case value >= 'a' && value <= 'f':
		return value - 'a' + 10, true
	case value >= 'A' && value <= 'F':
		return value - 'A' + 10, true
	default:
		return 0, false
	}
}
func ascii85DecodeLimited(b []byte, limit int) ([]byte, error) {
	if end := bytes.Index(b, []byte("~>")); end >= 0 {
		b = b[:end]
	}
	if len(b) >= 2 && b[0] == '<' && b[1] == '~' {
		b = b[2:]
	}
	upper, bounded := ascii85OutputUpperBound(b)
	if bounded && limit > 0 && upper > limit {
		return nil, fmt.Errorf("playa: decoded filter output exceeds %d bytes", limit)
	}
	estimated := (len(b)/5 + 1) * 4
	capacity := estimated
	// ASCII85 normally expands by less than one byte per input byte, but
	// the PDF shorthand `z` expands one byte to four zero bytes. Never cap
	// the destination at the encoded input length or that valid shorthand
	// would be silently decoded to an empty result.
	if capacity < len(b) {
		capacity = len(b)
	}
	if bounded {
		capacity = upper
		// encoding/ascii85.Decode needs room for one complete four-byte
		// tuple even when a flushed partial tuple emits only one to three
		// bytes. The upper-bound check above still enforces the caller's
		// decoded-output budget.
		if capacity > 0 && capacity < 4 {
			capacity = 4
		}
	}
	truncated := false
	if limit > 0 && !bounded && capacity > limit {
		capacity = limit
		truncated = true
	}
	out := make([]byte, capacity)
	n, _, e := ascii85.Decode(out, b, true)
	if e == nil && truncated && n == limit && len(b) > 0 {
		return nil, fmt.Errorf("playa: decoded filter output exceeds %d bytes", limit)
	}
	return out[:n], e
}

// ascii85OutputUpperBound returns the exact decoded size for valid ASCII85
// input, including PDF's `z` zero-tuple shorthand. Keeping this calculation
// separate prevents both silent truncation of repeated zero tuples and
// allocations beyond a caller's decoded-output budget.
func ascii85OutputUpperBound(b []byte) (int, bool) {
	maxInt := int(^uint(0) >> 1)
	out := 0
	group := 0
	add := func(value int) bool {
		if value < 0 || out > maxInt-value {
			return false
		}
		out += value
		return true
	}
	for _, value := range b {
		if value <= ' ' {
			continue
		}
		if value == 'z' {
			if group != 0 || !add(4) {
				return 0, false
			}
			continue
		}
		if value < '!' || value > 'u' {
			return 0, false
		}
		group++
		if group == 5 {
			if !add(4) {
				return 0, false
			}
			group = 0
		}
	}
	if group == 1 {
		return 0, false
	}
	if group > 1 && !add(group-1) {
		return 0, false
	}
	return out, true
}
func runLengthDecodeLimited(b []byte, limit int) ([]byte, error) {
	capacity := len(b)
	if limit > 0 && capacity > limit {
		capacity = limit
	}
	out := make([]byte, 0, capacity)
	for i := 0; i < len(b); {
		n := int(b[i])
		i++
		if n == 128 {
			return out, nil
		}
		if n <= 127 {
			if i+n+1 > len(b) {
				return nil, io.ErrUnexpectedEOF
			}
			if limit > 0 && n+1 > limit-len(out) {
				return nil, fmt.Errorf("playa: decoded filter output exceeds %d bytes", limit)
			}
			out = append(out, b[i:i+n+1]...)
			i += n + 1
		} else {
			if i >= len(b) {
				return nil, io.ErrUnexpectedEOF
			}
			repeat := 257 - n
			if limit > 0 && repeat > limit-len(out) {
				return nil, fmt.Errorf("playa: decoded filter output exceeds %d bytes", limit)
			}
			for j := 0; j < repeat; j++ {
				out = append(out, b[i])
			}
			i++
		}
	}
	return nil, io.ErrUnexpectedEOF
}
