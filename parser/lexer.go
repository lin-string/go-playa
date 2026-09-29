package parser

import (
	"encoding/json"
	"fmt"
	"strconv"
	"unicode/utf8"

	"github.com/lin-string/go-playa/parserconfig"
	pdftypes "github.com/lin-string/go-playa/pdftypes/primitives"
)

const (
	TokenEOF        = parserconfig.TokenEOF
	TokenNumber     = parserconfig.TokenNumber
	TokenName       = parserconfig.TokenName
	TokenLiteral    = parserconfig.TokenLiteral
	TokenString     = parserconfig.TokenString
	TokenHexString  = parserconfig.TokenHexString
	TokenArrayStart = parserconfig.TokenArrayStart
	TokenArrayEnd   = parserconfig.TokenArrayEnd
	TokenDictStart  = parserconfig.TokenDictStart
	TokenDictEnd    = parserconfig.TokenDictEnd
	TokenKeyword    = parserconfig.TokenKeyword
)

type Token struct {
	kind       parserconfig.TokenKind
	raw        []byte
	value      []byte
	text       string
	number     float64
	numberText string
	offset     int
}

// Kind reports the lexical category of the token.
func (t Token) Kind() parserconfig.TokenKind { return t.kind }

// Text reports the decoded textual token value.
func (t Token) Text() string { return t.text }

// Number reports the numeric token value.
func (t Token) Number() float64 { return t.number }

// NumberText returns the original numeric spelling when the token came from
// the raw document lexer. It is empty for synthetic and structured-parser
// tokens whose numeric value is represented only as a float64.
func (t Token) NumberText() string { return t.numberText }

// Offset reports the byte offset at which the token starts.
func (t Token) Offset() int { return t.offset }

// NewToken constructs a standalone token value for parser-facing integrations
// and tests. Lexer-produced tokens retain their raw/value buffers internally.
func NewToken(kind parserconfig.TokenKind, text string) Token {
	return Token{kind: kind, text: text}
}

// RawCopy returns an independent copy of the original token bytes.
func (t Token) RawCopy() []byte { return pdftypes.CloneBytes(t.raw) }

// ValueCopy returns an independent copy of decoded string bytes.
func (t Token) ValueCopy() []byte { return pdftypes.CloneBytes(t.value) }

// Finalize returns an independent snapshot of the token.
func (t Token) Finalize() Token {
	t.raw = t.RawCopy()
	t.value = pdftypes.CloneBytes(t.value)
	return t
}

// MarshalJSON preserves the historical public token snapshot shape while raw
// and decoded byte buffers remain implementation details.
func (t Token) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Kind   parserconfig.TokenKind `json:"Kind"`
		Text   string                 `json:"Text"`
		Number float64                `json:"Number"`
		Offset int                    `json:"Offset"`
	}{Kind: t.kind, Text: t.text, Number: t.number, Offset: t.offset})
}

// Lexer implements the lexical rules used by Playa's parser. It deliberately
// preserves raw bytes and Offset so higher layers can retain malformed-but-recoverable
// PDF objects and produce useful diagnostics.
type Lexer struct {
	data         []byte
	pos          int
	preserveNull bool
	sourceMode   bool
}

func NewLexer(data []byte) *Lexer { return &Lexer{data: data} }
func NewSourceLexer(data []byte) *Lexer {
	return &Lexer{data: data, preserveNull: true, sourceMode: true}
}
func (l *Lexer) Pos() int { return l.pos }

// Data returns an independent copy of the source buffer.
func (l *Lexer) Data() []byte { return pdftypes.CloneBytes(l.data) }

// DataBorrowed returns the source buffer without copying. The returned slice
// is valid only while the lexer owns the buffer and must not be retained or
// mutated by callers.
func (l *Lexer) DataBorrowed() []byte { return l.data }

func (l *Lexer) SetPos(pos int) { l.pos = pos }
func (l *Lexer) Next() (Token, error) {
	l.skip()
	start := l.pos
	if l.pos >= len(l.data) {
		return Token{kind: TokenEOF, offset: start}, nil
	}
	if l.preserveNull && l.data[l.pos] == 0 {
		l.pos++
		return Token{kind: TokenKeyword, raw: l.data[start:l.pos], text: "\x00", offset: start}, nil
	}
	if l.pos+1 < len(l.data) && l.data[l.pos] == '<' && l.data[l.pos+1] == '<' {
		l.pos += 2
		return Token{kind: TokenDictStart, raw: l.data[start:l.pos], offset: start}, nil
	}
	if l.pos+1 < len(l.data) && l.data[l.pos] == '>' && l.data[l.pos+1] == '>' {
		l.pos += 2
		return Token{kind: TokenDictEnd, raw: l.data[start:l.pos], offset: start}, nil
	}
	if l.data[l.pos] == '>' {
		l.pos++
		return Token{kind: TokenKeyword, raw: l.data[start:l.pos], text: ">", offset: start}, nil
	}
	if l.data[l.pos] == ')' {
		l.pos++
		return Token{kind: TokenKeyword, raw: l.data[start:l.pos], text: ")", offset: start}, nil
	}
	switch l.data[l.pos] {
	case '[':
		l.pos++
		return Token{kind: TokenArrayStart, raw: l.data[start:l.pos], offset: start}, nil
	case ']':
		l.pos++
		return Token{kind: TokenArrayEnd, raw: l.data[start:l.pos], offset: start}, nil
	case '{', '}':
		l.pos++
		return Token{kind: TokenKeyword, raw: l.data[start:l.pos], text: string(l.data[start:l.pos]), offset: start}, nil
	case '/':
		return l.name(start)
	case '(':
		return l.string(start)
	case '<':
		if l.validHexString(start) {
			return l.hex(start)
		}
		l.pos++
		return Token{kind: TokenKeyword, raw: l.data[start:l.pos], text: "<", offset: start}, nil
	}
	if l.sourceMode {
		if end, ok := l.sourceNumberEnd(start); ok {
			l.pos = end
			raw := l.data[start:l.pos]
			text := string(raw)
			number, err := strconv.ParseFloat(text, 64)
			if err != nil {
				return Token{}, NewParseError(err, start, "lex")
			}
			return Token{kind: TokenNumber, raw: raw, text: text, number: number, numberText: text, offset: start}, nil
		}
		if isASCIILetter(l.data[l.pos]) {
			l.takeSourceKeyword()
		} else {
			l.pos++
		}
		raw := l.data[start:l.pos]
		return Token{kind: TokenKeyword, raw: raw, text: latin1Text(raw), offset: start}, nil
	}
	l.takeWord()
	raw := l.data[start:l.pos]
	if looksLikeNumber(raw) {
		text := string(raw)
		if n, e := strconv.ParseFloat(text, 64); e == nil {
			return Token{kind: TokenNumber, raw: raw, text: text, number: n, offset: start}, nil
		}
	}
	return Token{kind: TokenKeyword, raw: raw, text: keywordText(raw), offset: start}, nil
}

func (l *Lexer) validHexString(start int) bool {
	if start >= len(l.data) || l.data[start] != '<' {
		return false
	}
	for index := start + 1; index < len(l.data); index++ {
		value := l.data[index]
		if value == '>' {
			return true
		}
		if isHexWhitespace(value) {
			continue
		}
		if _, ok := hexNibble(value); !ok {
			return false
		}
	}
	return false
}

func keywordText(raw []byte) string {
	switch len(raw) {
	case 1:
		switch raw[0] {
		case 'q':
			return "q"
		case 'Q':
			return "Q"
		case 'w':
			return "w"
		case 'J':
			return "J"
		case 'j':
			return "j"
		case 'M':
			return "M"
		case 'd':
			return "d"
		case 'i':
			return "i"
		case 'm':
			return "m"
		case 'l':
			return "l"
		case 'c':
			return "c"
		case 'v':
			return "v"
		case 'y':
			return "y"
		case 'h':
			return "h"
		case 'S':
			return "S"
		case 's':
			return "s"
		case 'F':
			return "F"
		case 'f':
			return "f"
		case 'B':
			return "B"
		case 'b':
			return "b"
		case 'n':
			return "n"
		case 'W':
			return "W"
		case 'G':
			return "G"
		case 'g':
			return "g"
		case 'K':
			return "K"
		case 'k':
			return "k"
		case 'R':
			return "R"
		case 'I':
			return "I"
		}
	case 2:
		switch {
		case raw[0] == 'c' && raw[1] == 'm':
			return "cm"
		case raw[0] == 'r' && raw[1] == 'i':
			return "ri"
		case raw[0] == 'g' && raw[1] == 's':
			return "gs"
		case raw[0] == 'f' && raw[1] == '*':
			return "f*"
		case raw[0] == 'B' && raw[1] == '*':
			return "B*"
		case raw[0] == 'b' && raw[1] == '*':
			return "b*"
		case raw[0] == 'W' && raw[1] == '*':
			return "W*"
		case raw[0] == 'B' && raw[1] == 'T':
			return "BT"
		case raw[0] == 'E' && raw[1] == 'T':
			return "ET"
		case raw[0] == 'T' && raw[1] == 'c':
			return "Tc"
		case raw[0] == 'T' && raw[1] == 'w':
			return "Tw"
		case raw[0] == 'T' && raw[1] == 'z':
			return "Tz"
		case raw[0] == 'T' && raw[1] == 'L':
			return "TL"
		case raw[0] == 'T' && raw[1] == 'f':
			return "Tf"
		case raw[0] == 'T' && raw[1] == 'r':
			return "Tr"
		case raw[0] == 'T' && raw[1] == 's':
			return "Ts"
		case raw[0] == 'T' && raw[1] == 'd':
			return "Td"
		case raw[0] == 'T' && raw[1] == 'D':
			return "TD"
		case raw[0] == 'T' && raw[1] == 'm':
			return "Tm"
		case raw[0] == 'T' && raw[1] == '*':
			return "T*"
		case raw[0] == 'T' && raw[1] == 'j':
			return "Tj"
		case raw[0] == 'T' && raw[1] == 'J':
			return "TJ"
		case raw[0] == 'D' && raw[1] == 'o':
			return "Do"
		case raw[0] == 'M' && raw[1] == 'P':
			return "MP"
		case raw[0] == 'D' && raw[1] == 'P':
			return "DP"
		case raw[0] == 'B' && raw[1] == 'I':
			return "BI"
		case raw[0] == 'I' && raw[1] == 'D':
			return "ID"
		case raw[0] == 'E' && raw[1] == 'I':
			return "EI"
		case raw[0] == 'C' && raw[1] == 'S':
			return "CS"
		case raw[0] == 'c' && raw[1] == 's':
			return "cs"
		case raw[0] == 'S' && raw[1] == 'C':
			return "SC"
		case raw[0] == 's' && raw[1] == 'c':
			return "sc"
		case raw[0] == 'R' && raw[1] == 'G':
			return "RG"
		case raw[0] == 'r' && raw[1] == 'g':
			return "rg"
		case raw[0] == 'r' && raw[1] == 'e':
			return "re"
		case raw[0] == 'C' && raw[1] == 'A':
			return "CA"
		case raw[0] == 'c' && raw[1] == 'a':
			return "ca"
		case raw[0] == 'B' && raw[1] == 'M':
			return "BM"
		case raw[0] == 'S' && raw[1] == 'A':
			return "SA"
		case raw[0] == 'T' && raw[1] == 'K':
			return "TK"
		}
	case 3:
		switch {
		case raw[0] == 'B' && raw[1] == 'M' && raw[2] == 'C':
			return "BMC"
		case raw[0] == 'B' && raw[1] == 'D' && raw[2] == 'C':
			return "BDC"
		case raw[0] == 'E' && raw[1] == 'M' && raw[2] == 'C':
			return "EMC"
		case raw[0] == 'S' && raw[1] == 'C' && raw[2] == 'N':
			return "SCN"
		case raw[0] == 's' && raw[1] == 'c' && raw[2] == 'n':
			return "scn"
		case raw[0] == 'A' && raw[1] == 'I' && raw[2] == 'S':
			return "AIS"
		}
	case 4:
		if raw[0] == 'n' && raw[1] == 'u' && raw[2] == 'l' && raw[3] == 'l' {
			return "null"
		}
		if raw[0] == 't' && raw[1] == 'r' && raw[2] == 'u' && raw[3] == 'e' {
			return "true"
		}
	case 5:
		if raw[0] == 'S' && raw[1] == 'M' && raw[2] == 'a' && raw[3] == 's' && raw[4] == 'k' {
			return "SMask"
		}
	}
	if !utf8.Valid(raw) {
		return decodePDFText(raw)
	}
	return string(raw)
}
func (l *Lexer) skip() {
	for l.pos < len(l.data) {
		switch l.data[l.pos] {
		case 0, 9, 10, 11, 12, 13, 32:
			if l.preserveNull && l.data[l.pos] == 0 {
				return
			}
			l.pos++
		case '%':
			for l.pos < len(l.data) && l.data[l.pos] != '\n' && l.data[l.pos] != '\r' {
				l.pos++
			}
		default:
			return
		}
	}
}
func delim(c byte) bool {
	switch c {
	case 0, 9, 10, 12, 13, 32, '[', ']', '(', ')', '<', '>', '/', '%', '{', '}':
		return true
	default:
		return false
	}
}

func looksLikeNumber(raw []byte) bool {
	if len(raw) == 0 {
		return false
	}
	start := 0
	if raw[0] == '+' || raw[0] == '-' {
		start = 1
	}
	if start == len(raw) {
		return false
	}
	digit := false
	for _, c := range raw[start:] {
		switch {
		case c >= '0' && c <= '9':
			digit = true
		case c == '.', c == 'e', c == 'E', c == '+', c == '-':
		default:
			return false
		}
	}
	return digit
}
func (l *Lexer) takeWord() {
	for l.pos < len(l.data) && !delim(l.data[l.pos]) {
		l.pos++
	}
}
func (l *Lexer) name(start int) (Token, error) {
	l.pos++
	if l.sourceMode {
		for l.pos < len(l.data) {
			if l.data[l.pos] == '#' {
				if l.pos+2 >= len(l.data) || !isHexByte(l.data[l.pos+1], l.data[l.pos+2]) {
					break
				}
				l.pos += 3
				continue
			}
			if sourceDelimiter(l.data[l.pos]) {
				break
			}
			l.pos++
		}
	} else {
		for l.pos < len(l.data) && !delim(l.data[l.pos]) {
			l.pos++
		}
	}
	raw := l.data[start:l.pos]
	text := decodeName(raw[1:])
	if l.sourceMode {
		text = decodeNameSource(raw[1:])
	}
	return Token{kind: TokenName, raw: raw, text: text, offset: start}, nil
}
func decodeName(b []byte) string {
	out := make([]byte, 0, len(b))
	for i := 0; i < len(b); i++ {
		if b[i] == '#' && i+2 < len(b) {
			if v, e := strconv.ParseUint(string(b[i+1:i+3]), 16, 8); e == nil {
				out = append(out, byte(v))
				i += 2
				continue
			}
		}
		out = append(out, b[i])
	}
	if utf8.Valid(out) {
		return string(out)
	}
	return decodePDFText(out)
}
func (l *Lexer) string(start int) (Token, error) {
	l.pos++
	depth := 1
	out := make([]byte, 0)
	for l.pos < len(l.data) {
		c := l.data[l.pos]
		l.pos++
		if c == '\\' {
			if l.pos >= len(l.data) {
				break
			}
			n := l.data[l.pos]
			l.pos++
			switch n {
			case 'n':
				out = append(out, '\n')
			case 'r':
				out = append(out, '\r')
			case 't':
				out = append(out, '\t')
			case 'b':
				out = append(out, '\b')
			case 'f':
				out = append(out, '\f')
			case '\n':
				continue
			case '\r':
				if l.pos < len(l.data) && l.data[l.pos] == '\n' {
					l.pos++
				}
				continue
			case '0', '1', '2', '3', '4', '5', '6', '7':
				v := int(n - '0')
				for j := 0; j < 2 && l.pos < len(l.data); j++ {
					q := l.data[l.pos]
					if q < '0' || q > '7' {
						break
					}
					v = v*8 + int(q-'0')
					l.pos++
				}
				if v < 256 {
					out = append(out, byte(v))
				}
			default:
				out = append(out, n)
			}
			continue
		}
		if c == '(' {
			depth++
		}
		if l.sourceMode && (c == '\r' || c == '\n') {
			if c == '\r' && l.pos < len(l.data) && l.data[l.pos] == '\n' {
				l.pos++
			}
			out = append(out, '\n')
			continue
		}
		if c == ')' {
			depth--
			if depth == 0 {
				return Token{kind: TokenString, raw: l.data[start:l.pos], value: out, text: string(out), offset: start}, nil
			}
		}
		out = append(out, c)
	}
	return Token{}, NewParseError(fmt.Errorf("unterminated PDF string"), start, "lex")
}
func (l *Lexer) hex(start int) (Token, error) {
	l.pos++
	for l.pos < len(l.data) && l.data[l.pos] != '>' {
		l.pos++
	}
	if l.pos >= len(l.data) {
		return Token{}, NewParseError(fmt.Errorf("unterminated hex string"), start, "lex")
	}
	l.pos++
	data := l.data[start+1 : l.pos-1]
	digits := 0
	for _, c := range data {
		if !isHexWhitespace(c) {
			digits++
		}
	}
	buf := make([]byte, (digits+1)/2)
	index := 0
	var high byte
	haveHigh := false
	for _, c := range data {
		if isHexWhitespace(c) {
			continue
		}
		value, ok := hexNibble(c)
		if !ok {
			return Token{}, NewParseError(fmt.Errorf("invalid hex string"), start, "lex")
		}
		if !haveHigh {
			high, haveHigh = value, true
			continue
		}
		buf[index] = high<<4 | value
		index++
		haveHigh = false
	}
	if haveHigh {
		buf[index] = high << 4
	}
	return Token{kind: TokenHexString, raw: l.data[start:l.pos], value: buf, text: string(buf), offset: start}, nil
}

func isHexWhitespace(c byte) bool {
	return c == ' ' || c == '\n' || c == '\r' || c == '\t' || c == '\v'
}

func (l *Lexer) sourceNumberEnd(start int) (int, bool) {
	pos := start
	if pos < len(l.data) && (l.data[pos] == '+' || l.data[pos] == '-') {
		pos++
	}
	if pos >= len(l.data) {
		return start, false
	}
	if l.data[pos] == '.' {
		if pos+1 >= len(l.data) || !isASCIIDigit(l.data[pos+1]) {
			return start, false
		}
		pos += 2
		for pos < len(l.data) && isASCIIDigit(l.data[pos]) {
			pos++
		}
		return pos, true
	}
	if !isASCIIDigit(l.data[pos]) {
		return start, false
	}
	for pos < len(l.data) && isASCIIDigit(l.data[pos]) {
		pos++
	}
	if pos < len(l.data) && l.data[pos] == '.' {
		pos++
		for pos < len(l.data) && isASCIIDigit(l.data[pos]) {
			pos++
		}
	}
	return pos, true
}

func (l *Lexer) takeSourceKeyword() {
	for l.pos < len(l.data) && !sourceDelimiter(l.data[l.pos]) {
		l.pos++
	}
}

func isASCIILetter(c byte) bool {
	return c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z'
}

func isASCIIDigit(c byte) bool {
	return c >= '0' && c <= '9'
}

func sourceDelimiter(c byte) bool {
	switch c {
	case '#', '%', '[', ']', '(', ')', '<', '>', '/', '{', '}', ' ', '\t', '\n', '\v', '\f', '\r':
		return true
	default:
		return false
	}
}

func isHexByte(high, low byte) bool {
	_, highOK := hexNibble(high)
	_, lowOK := hexNibble(low)
	return highOK && lowOK
}

func decodeNameSource(raw []byte) string {
	decoded := make([]byte, 0, len(raw))
	for index := 0; index < len(raw); index++ {
		if raw[index] == '#' && index+2 < len(raw) && isHexByte(raw[index+1], raw[index+2]) {
			high, _ := hexNibble(raw[index+1])
			low, _ := hexNibble(raw[index+2])
			decoded = append(decoded, high<<4|low)
			index += 2
			continue
		}
		decoded = append(decoded, raw[index])
	}
	if utf8.Valid(decoded) {
		return string(decoded)
	}
	return latin1Text(decoded)
}

func latin1Text(raw []byte) string {
	var out []rune
	for _, value := range raw {
		out = append(out, rune(value))
	}
	return string(out)
}

func hexNibble(c byte) (byte, bool) {
	switch {
	case c >= '0' && c <= '9':
		return c - '0', true
	case c >= 'a' && c <= 'f':
		return c - 'a' + 10, true
	case c >= 'A' && c <= 'F':
		return c - 'A' + 10, true
	default:
		return 0, false
	}
}
