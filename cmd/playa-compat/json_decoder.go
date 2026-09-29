package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"unicode/utf16"
	"unicode/utf8"
)

// jsonTokenDecoder is the subset of encoding/json.Decoder used by the
// compatibility comparator. Keeping the interface permits focused tests to
// continue using the standard decoder.
type jsonTokenDecoder interface {
	Token() (json.Token, error)
	More() bool
	Decode(any) error
	UseNumber()
	DisallowUnknownFields()
}

// compareJSONDecoder avoids encoding/json.Decoder.Token's per-value scanner
// error allocation. Playa snapshots are produced by our pinned oracle, so the
// comparator only needs a small validating boundary scanner; values that must
// be materialized are still decoded by encoding/json.
type compareJSONDecoder struct {
	reader                *bufio.Reader
	useNumber             bool
	disallowUnknownFields bool
	err                   error
	scratch               []byte
}

func newCompareJSONDecoder(reader io.Reader) *compareJSONDecoder {
	return &compareJSONDecoder{reader: bufio.NewReaderSize(reader, 64<<10)}
}

func (decoder *compareJSONDecoder) UseNumber() { decoder.useNumber = true }

func (decoder *compareJSONDecoder) DisallowUnknownFields() {
	decoder.disallowUnknownFields = true
}

func (decoder *compareJSONDecoder) More() bool {
	if decoder.err != nil {
		return false
	}
	if err := decoder.skipWhitespace(); err != nil {
		decoder.err = err
		return false
	}
	value, err := decoder.peekByte()
	if err != nil {
		decoder.err = err
		return false
	}
	if value == ',' {
		_, _ = decoder.reader.ReadByte()
		if err := decoder.skipWhitespace(); err != nil {
			decoder.err = err
			return false
		}
		value, err = decoder.peekByte()
		if err != nil {
			decoder.err = err
			return false
		}
	}
	return value != ']' && value != '}'
}

func (decoder *compareJSONDecoder) Token() (json.Token, error) {
	if decoder.err != nil {
		return nil, decoder.err
	}
	if err := decoder.skipValueSeparator(); err != nil {
		decoder.err = err
		return nil, err
	}
	first, err := decoder.reader.ReadByte()
	if err != nil {
		decoder.err = err
		return nil, err
	}
	switch first {
	case '{', '}', '[', ']':
		return json.Delim(first), nil
	case '"':
		value, err := decoder.readStringToken()
		if err != nil {
			decoder.err = err
			return nil, err
		}
		return value, nil
	default:
		raw, err := decoder.readScalarToken(first)
		if err != nil {
			decoder.err = err
			return nil, err
		}
		switch string(raw) {
		case "true":
			return true, nil
		case "false":
			return false, nil
		case "null":
			return nil, nil
		}
		if decoder.useNumber {
			return json.Number(string(raw)), nil
		}
		value, err := strconv.ParseFloat(string(raw), 64)
		if err != nil {
			decoder.err = err
			return nil, err
		}
		return value, nil
	}
}

func (decoder *compareJSONDecoder) Decode(target any) error {
	if decoder.err != nil {
		return decoder.err
	}
	if err := decoder.skipValueSeparator(); err != nil {
		decoder.err = err
		return err
	}
	raw, err := decoder.readRawValue()
	if err != nil {
		decoder.err = err
		return err
	}
	if message, ok := target.(*json.RawMessage); ok {
		*message = append((*message)[:0], raw...)
		return nil
	}
	if decoder.useNumber || decoder.disallowUnknownFields {
		standard := json.NewDecoder(bytes.NewReader(raw))
		if decoder.useNumber {
			standard.UseNumber()
		}
		if decoder.disallowUnknownFields {
			standard.DisallowUnknownFields()
		}
		err = standard.Decode(target)
	} else {
		err = json.Unmarshal(raw, target)
	}
	if err != nil {
		decoder.err = err
	}
	return err
}

func (decoder *compareJSONDecoder) skipValueSeparator() error {
	for {
		if err := decoder.skipWhitespace(); err != nil {
			return err
		}
		value, err := decoder.peekByte()
		if err != nil {
			return err
		}
		if value != ',' && value != ':' {
			return nil
		}
		_, _ = decoder.reader.ReadByte()
	}
}

func (decoder *compareJSONDecoder) skipWhitespace() error {
	for {
		value, err := decoder.peekByte()
		if err != nil {
			return err
		}
		switch value {
		case ' ', '\t', '\r', '\n':
			_, _ = decoder.reader.ReadByte()
		default:
			return nil
		}
	}
}

func (decoder *compareJSONDecoder) peekByte() (byte, error) {
	value, err := decoder.reader.Peek(1)
	if err != nil {
		return 0, err
	}
	return value[0], nil
}

func (decoder *compareJSONDecoder) readString(first byte) ([]byte, error) {
	raw := []byte{first}
	escaped := false
	for {
		value, err := decoder.reader.ReadByte()
		if err != nil {
			return nil, err
		}
		raw = append(raw, value)
		if escaped {
			escaped = false
			continue
		}
		if value == '\\' {
			escaped = true
			continue
		}
		if value == '"' {
			return raw, nil
		}
	}
}

func (decoder *compareJSONDecoder) readStringToken() (string, error) {
	decoder.scratch = decoder.scratch[:0]
	for {
		value, err := decoder.reader.ReadByte()
		if err != nil {
			return "", err
		}
		switch value {
		case '"':
			return string(decoder.scratch), nil
		case '\\':
			escape, err := decoder.reader.ReadByte()
			if err != nil {
				return "", err
			}
			switch escape {
			case '"', '\\', '/':
				decoder.scratch = append(decoder.scratch, escape)
			case 'b':
				decoder.scratch = append(decoder.scratch, '\b')
			case 'f':
				decoder.scratch = append(decoder.scratch, '\f')
			case 'n':
				decoder.scratch = append(decoder.scratch, '\n')
			case 'r':
				decoder.scratch = append(decoder.scratch, '\r')
			case 't':
				decoder.scratch = append(decoder.scratch, '\t')
			case 'u':
				runeValue, err := decoder.readUnicodeEscape()
				if err != nil {
					return "", err
				}
				decoder.scratch = utf8.AppendRune(decoder.scratch, runeValue)
			default:
				return "", fmt.Errorf("invalid JSON string escape %q", escape)
			}
		default:
			if value < 0x20 {
				return "", fmt.Errorf("invalid control character in JSON string")
			}
			decoder.scratch = append(decoder.scratch, value)
		}
	}
}

func (decoder *compareJSONDecoder) readUnicodeEscape() (rune, error) {
	first, err := decoder.readHexRune()
	if err != nil {
		return 0, err
	}
	if first < 0xD800 || first > 0xDFFF {
		return first, nil
	}
	if first > 0xDBFF {
		return utf8.RuneError, nil
	}
	sequence, err := decoder.reader.Peek(6)
	if err != nil || len(sequence) != 6 || sequence[0] != '\\' || sequence[1] != 'u' {
		return utf8.RuneError, nil
	}
	second, valid := jsonHexRune(sequence[2:])
	if !valid {
		_, _ = decoder.reader.Discard(2)
		_, err := decoder.readHexRune()
		return 0, err
	}
	if second < 0xDC00 || second > 0xDFFF {
		// encoding/json emits RuneError for the unmatched high surrogate, then
		// decodes the following escape independently.
		return utf8.RuneError, nil
	}
	_, _ = decoder.reader.Discard(6)
	return utf16.DecodeRune(first, second), nil
}

func jsonHexRune(value []byte) (rune, bool) {
	if len(value) != 4 {
		return 0, false
	}
	var result rune
	for _, digit := range value {
		result <<= 4
		switch {
		case digit >= '0' && digit <= '9':
			result += rune(digit - '0')
		case digit >= 'a' && digit <= 'f':
			result += rune(digit-'a') + 10
		case digit >= 'A' && digit <= 'F':
			result += rune(digit-'A') + 10
		default:
			return 0, false
		}
	}
	return result, true
}

func (decoder *compareJSONDecoder) readHexRune() (rune, error) {
	var value rune
	for range 4 {
		digit, err := decoder.reader.ReadByte()
		if err != nil {
			return 0, err
		}
		value <<= 4
		switch {
		case digit >= '0' && digit <= '9':
			value += rune(digit - '0')
		case digit >= 'a' && digit <= 'f':
			value += rune(digit-'a') + 10
		case digit >= 'A' && digit <= 'F':
			value += rune(digit-'A') + 10
		default:
			return 0, fmt.Errorf("invalid hexadecimal digit %q in JSON string", digit)
		}
	}
	return value, nil
}

func (decoder *compareJSONDecoder) readScalarToken(first byte) ([]byte, error) {
	decoder.scratch = append(decoder.scratch[:0], first)
	for {
		value, err := decoder.peekByte()
		if err == io.EOF {
			return decoder.scratch, nil
		}
		if err != nil {
			return nil, err
		}
		if value == ',' || value == ']' || value == '}' || value == ' ' || value == '\t' || value == '\r' || value == '\n' {
			return decoder.scratch, nil
		}
		value, _ = decoder.reader.ReadByte()
		decoder.scratch = append(decoder.scratch, value)
	}
}

func (decoder *compareJSONDecoder) readScalar(first byte) ([]byte, error) {
	raw := []byte{first}
	for {
		value, err := decoder.peekByte()
		if err == io.EOF {
			return raw, nil
		}
		if err != nil {
			return nil, err
		}
		if value == ',' || value == ']' || value == '}' || value == ' ' || value == '\t' || value == '\r' || value == '\n' {
			return raw, nil
		}
		value, _ = decoder.reader.ReadByte()
		raw = append(raw, value)
	}
}

func (decoder *compareJSONDecoder) readRawValue() ([]byte, error) {
	first, err := decoder.reader.ReadByte()
	if err != nil {
		return nil, err
	}
	if first == '"' {
		return decoder.readString(first)
	}
	if first != '{' && first != '[' {
		return decoder.readScalar(first)
	}
	raw := []byte{first}
	stack := []byte{first}
	inString := false
	escaped := false
	for len(stack) > 0 {
		value, err := decoder.reader.ReadByte()
		if err != nil {
			return nil, err
		}
		raw = append(raw, value)
		if inString {
			if escaped {
				escaped = false
				continue
			}
			switch value {
			case '\\':
				escaped = true
			case '"':
				inString = false
			}
			continue
		}
		switch value {
		case '"':
			inString = true
		case '{', '[':
			stack = append(stack, value)
		case '}', ']':
			open := stack[len(stack)-1]
			if (open == '{' && value != '}') || (open == '[' && value != ']') {
				return nil, fmt.Errorf("mismatched JSON delimiter %q", value)
			}
			stack = stack[:len(stack)-1]
		}
	}
	return raw, nil
}
