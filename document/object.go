package document

import (
	pdfparser "github.com/lin-string/go-playa/parser"
	"github.com/lin-string/go-playa/parserconfig"
	publictypes "github.com/lin-string/go-playa/pdftypes"
	pdftypes "github.com/lin-string/go-playa/pdftypes/primitives"
)

type Token = pdfparser.Token
type Lexer = pdfparser.Lexer

// NewToken constructs a standalone lexical token value for integrations that
// need to feed a token snapshot into compatibility or diagnostic code.
func NewToken(kind parserconfig.TokenKind, text string) Token {
	return pdfparser.NewToken(kind, text)
}

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

type Object = pdftypes.Object
type Null = pdftypes.Null
type Bool = pdftypes.Bool
type Number = pdftypes.Number
type Name = pdftypes.Name
type String = pdftypes.String
type Array = pdftypes.Array
type InvalidArray = pdftypes.InvalidArray
type Dict = pdftypes.Dict
type Keyword = pdftypes.Keyword

type Ref = pdftypes.Ref

type Stream = publictypes.Stream

func newStream(dict Dict, data []byte) Stream {
	return publictypes.NewStreamOwned(dict, data)
}

func newStreamWithDecodeDict(dict Dict, data []byte, decodeDict Dict) Stream {
	return publictypes.NewStreamOwned(dict, data, decodeDict)
}

func streamWithRef(stream Stream, ref Ref) Stream { return stream.WithRef(ref) }

func cloneObjectBytes(value []byte) []byte {
	return pdftypes.CloneBytes(value)
}

func cloneStreamBytes(value []byte) []byte { return cloneObjectBytes(value) }

func NewLexer(data []byte) *Lexer       { return pdfparser.NewLexer(data) }
func NewSourceLexer(data []byte) *Lexer { return pdfparser.NewSourceLexer(data) }

type ObjectParser = pdfparser.ObjectParser

func NewObjectParser(data []byte) *ObjectParser { return pdfparser.NewObjectParser(data) }

func ParseObject(data []byte) (Object, error) {
	return pdfparser.ParseObject(data)
}

func ResolveAll(o Object, resolve func(Ref) (Object, bool)) Object {
	return publictypes.ResolveAll(o, resolve)
}
func NumberValue(o Object) (float64, bool)       { return pdfparser.NumberValue(o) }
func finiteNumberValue(o Object) (float64, bool) { return pdfparser.FiniteNumberValue(o) }
func IntValue(o Object) (int, bool)              { return pdfparser.IntValue(o) }
func NameValue(o Object) (string, bool)          { return pdfparser.NameValue(o) }
func RefValue(o Object) (Ref, bool)              { return pdfparser.RefValue(o) }
