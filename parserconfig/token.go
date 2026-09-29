// Package parserconfig owns dependency-free parser token values.
package parserconfig

// TokenKind identifies a lexical PDF token.
type TokenKind uint8

const (
	TokenEOF TokenKind = iota
	TokenNumber
	TokenName
	TokenLiteral
	TokenString
	TokenHexString
	TokenArrayStart
	TokenArrayEnd
	TokenDictStart
	TokenDictEnd
	TokenKeyword
)
