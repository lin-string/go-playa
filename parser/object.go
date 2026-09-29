package parser

import (
	"fmt"

	pdftypes "github.com/lin-string/go-playa/pdftypes/primitives"
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
type Ref = pdftypes.Ref
type Keyword = pdftypes.Keyword

type ObjectParser struct {
	lexer         *Lexer
	buffered      [4]Token
	bufferedCount uint8
	overflow      []Token
}

func NewObjectParser(data []byte) *ObjectParser { return &ObjectParser{lexer: NewLexer(data)} }
func ParseObject(data []byte) (Object, error)   { return NewObjectParser(data).Parse() }

// NextToken, ParseToken, and Lexer expose the parser integration surface used
// by content-stream interpretation while keeping buffering private.
func (p *ObjectParser) NextToken() (Token, error)          { return p.next() }
func (p *ObjectParser) PushToken(t Token)                  { p.push(t) }
func (p *ObjectParser) ParseToken(t Token) (Object, error) { return p.parseToken(t) }
func (p *ObjectParser) Lexer() *Lexer                      { return p.lexer }

func (p *ObjectParser) PendingTokens() []Token {
	out := make([]Token, 0, int(p.bufferedCount)+len(p.overflow))
	out = append(out, p.buffered[:p.bufferedCount]...)
	out = append(out, p.overflow...)
	return out
}

func (p *ObjectParser) next() (Token, error) {
	if len(p.overflow) > 0 {
		last := len(p.overflow) - 1
		t := p.overflow[last]
		p.overflow = p.overflow[:last]
		return t, nil
	}
	if p.bufferedCount > 0 {
		p.bufferedCount--
		return p.buffered[p.bufferedCount], nil
	}
	t, err := p.lexer.Next()
	return t, Wrap(err, p.lexer.Pos(), "lex")
}

func (p *ObjectParser) push(t Token) {
	if p.bufferedCount < uint8(len(p.buffered)) {
		p.buffered[p.bufferedCount] = t
		p.bufferedCount++
		return
	}
	p.overflow = append(p.overflow, t)
}

func (p *ObjectParser) Parse() (Object, error) {
	t, err := p.next()
	if err != nil {
		return nil, err
	}
	return p.parseToken(t)
}

func (p *ObjectParser) parseToken(t Token) (Object, error) {
	switch t.Kind() {
	case TokenNumber:
		if t.Number() == float64(int(t.Number())) {
			t2, err := p.next()
			if err != nil {
				return nil, err
			}
			if t2.Kind() == TokenNumber && t2.Number() == float64(int(t2.Number())) {
				t3, err := p.next()
				if err != nil {
					return nil, err
				}
				if t3.Kind() == TokenKeyword && t3.Text() == "R" {
					return Ref{Object: int(t.Number()), Generation: int(t2.Number())}, nil
				}
				p.push(t3)
			}
			p.push(t2)
		}
		return Number(t.Number()), nil
	case TokenName:
		return Name(t.Text()), nil
	case TokenString, TokenHexString:
		if value := t.ValueCopy(); value != nil {
			return String(value), nil
		}
		return String([]byte(t.Text())), nil
	case TokenKeyword:
		switch t.Text() {
		case "null":
			return Null{}, nil
		case "true":
			return Bool(true), nil
		case "false":
			return Bool(false), nil
		case "{":
			return p.procedure()
		}
		return Keyword(t.Text()), nil
	case TokenArrayStart:
		return p.array()
	case TokenDictStart:
		return p.dict()
	default:
		return nil, Wrap(fmt.Errorf("unexpected token kind %d %q", t.Kind(), t.Text()), t.Offset(), "object")
	}
}

// procedure parses the PostScript-style procedure containers accepted by
// Playa's non-strict object parser. PDF does not assign procedures a distinct
// public object type, so retain their recursively parsed values as an Array.
func (p *ObjectParser) procedure() (Object, error) {
	out := Array{}
	for {
		t, err := p.next()
		if err != nil {
			return nil, err
		}
		if t.Kind() == TokenEOF {
			return nil, Wrap(fmt.Errorf("unterminated procedure"), t.Offset(), "object")
		}
		if t.Kind() == TokenKeyword && t.Text() == "}" {
			return out, nil
		}
		value, err := p.parseToken(t)
		if err != nil {
			return nil, err
		}
		out = append(out, value)
	}
}

func (p *ObjectParser) array() (Object, error) {
	out := Array{}
	for {
		t, err := p.next()
		if err != nil {
			return nil, err
		}
		if t.Kind() == TokenEOF {
			return nil, Wrap(fmt.Errorf("unterminated array"), t.Offset(), "object")
		}
		if t.Kind() == TokenArrayEnd {
			return out, nil
		}
		value, err := p.parseToken(t)
		if err != nil {
			return nil, err
		}
		out = append(out, value)
	}
}

func (p *ObjectParser) dict() (Object, error) {
	out := Dict{}
	for {
		t, err := p.next()
		if err != nil {
			return nil, err
		}
		if t.Kind() == TokenEOF {
			return nil, Wrap(fmt.Errorf("unterminated dictionary"), t.Offset(), "object")
		}
		if t.Kind() == TokenDictEnd {
			return out, nil
		}
		if t.Kind() != TokenName {
			return nil, Wrap(fmt.Errorf("dictionary key is not a name"), t.Offset(), "object")
		}
		valueToken, err := p.next()
		if err != nil {
			return nil, err
		}
		if valueToken.Kind() == TokenEOF {
			return nil, Wrap(fmt.Errorf("dictionary value is missing"), valueToken.Offset(), "object")
		}
		p.push(valueToken)
		value, err := p.Parse()
		if err != nil {
			return nil, err
		}
		out[Name(t.Text())] = value
	}
}

func NumberValue(o Object) (float64, bool) { return pdftypes.NumberValue(o) }

func FiniteNumberValue(o Object) (float64, bool) {
	return pdftypes.FiniteNumberValue(o)
}

func IntValue(o Object) (int, bool) {
	return pdftypes.IntValue(o)
}

func NameValue(o Object) (string, bool) { return pdftypes.NameValue(o) }
func RefValue(o Object) (Ref, bool)     { return pdftypes.RefValue(o) }
