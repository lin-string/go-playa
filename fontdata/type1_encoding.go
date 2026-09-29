package fontdata

import (
	"bytes"
	"encoding/hex"

	pdfparser "github.com/lin-string/go-playa/parser"
)

// ParseType1Encoding reads cleartext Type 1 Encoding assignments. It only
// accepts the well-formed `code /name put` pattern and ignores PostScript.
func ParseType1Encoding(data []byte) map[byte]string {
	lexer := pdfparser.NewLexer(data)
	result := map[byte]string{}
	var previous [2]pdfparser.Token
	for {
		token, err := lexer.Next()
		if err != nil || token.Kind() == pdfparser.TokenEOF {
			return result
		}
		if token.Kind() == pdfparser.TokenKeyword && token.Text() == "put" {
			if previous[0].Kind() == pdfparser.TokenNumber && previous[1].Kind() == pdfparser.TokenName && previous[1].Text() != "" {
				code := int(previous[0].Number())
				if float64(code) == previous[0].Number() && code >= 0 && code <= 255 {
					result[byte(code)] = previous[1].Text()
				}
			}
		}
		previous[0], previous[1] = previous[1], token
	}
}

// ParseType1EexecEncoding decodes the encrypted Encoding portion after the
// cleartext eexec marker. length1 is the cleartext boundary in data.
func ParseType1EexecEncoding(data []byte, length1 int) map[byte]string {
	if length1 < 0 || length1 >= len(data) || !bytes.Contains(data[:length1], []byte("eexec")) {
		return nil
	}
	payload := data[length1:]
	if decoded, ok := DecodeType1HexEexec(payload); ok {
		payload = decoded
	}
	if len(payload) <= 4 {
		return nil
	}
	return ParseType1Encoding(DecryptType1Eexec(payload)[4:])
}

// DecodeType1HexEexec decodes a hexadecimal eexec payload.
func DecodeType1HexEexec(data []byte) ([]byte, bool) {
	digits := make([]byte, 0, len(data))
	for _, value := range data {
		switch {
		case value == ' ' || value == '\t' || value == '\r' || value == '\n' || value == '\f':
			continue
		case (value >= '0' && value <= '9') || (value >= 'a' && value <= 'f') || (value >= 'A' && value <= 'F'):
			digits = append(digits, value)
		default:
			if len(digits) < 8 || len(digits)%2 != 0 {
				return nil, false
			}
			decoded, err := hex.DecodeString(string(digits))
			return decoded, err == nil
		}
	}
	if len(digits) < 8 || len(digits)%2 != 0 {
		return nil, false
	}
	decoded, err := hex.DecodeString(string(digits))
	return decoded, err == nil
}

// DecryptType1Eexec decrypts the Type 1 eexec cipher payload.
func DecryptType1Eexec(data []byte) []byte {
	const c1, c2 = uint32(52845), uint32(22719)
	r := uint32(55665)
	decrypted := make([]byte, len(data))
	for index, cipher := range data {
		decrypted[index] = cipher ^ byte(r>>8)
		r = ((uint32(cipher) + r) * c1) + c2
		r &= 0xffff
	}
	return decrypted
}
