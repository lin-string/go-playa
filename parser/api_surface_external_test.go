package parser_test

import (
	"testing"

	"github.com/lin-string/go-playa/parser"
)

func TestPublicParserSurface(t *testing.T) {
	_ = (*parser.ParseError)(nil).Offset
	_ = (*parser.ParseError)(nil).Operation
	_ = (*parser.ParseError)(nil).ObjectRef
	_ = (*parser.ParseError)(nil).HasObject
	_ = (*parser.ParseError)(nil).Cause
	var _ parser.Object
	var _ parser.Ref
	var _ parser.Dict
	var _ parser.Array
	var _ parser.Null
	var _ parser.Bool
	var _ parser.Number
	var _ parser.Name
	var _ parser.String
	var _ parser.Keyword
	lexer := parser.NewLexer([]byte("12 /Name"))
	_ = lexer.Data
	_ = lexer.DataBorrowed
	_ = (parser.Token{}).RawCopy
	_ = (parser.Token{}).Finalize
	_ = parser.DecodeFiltersLenientLimited
	if _, err := lexer.Next(); err != nil {
		t.Fatal(err)
	}
	if _, err := parser.NewObjectParser([]byte("[1 2]")).Parse(); err != nil {
		t.Fatal(err)
	}
	if _, err := parser.ParseObject([]byte("<< /Width 10 >>")); err != nil {
		t.Fatal(err)
	}
}
