package parser

import (
	"testing"

	"github.com/lin-string/go-playa/parserconfig"
)

func TestTokenRawCopyDoesNotExposeLexerBuffer(t *testing.T) {
	lexer := NewLexer([]byte("12"))
	token, err := lexer.Next()
	if err != nil {
		t.Fatal(err)
	}
	raw := token.RawCopy()
	raw[0] = '9'
	if string(token.raw) != "12" {
		t.Fatalf("token raw copy aliases source: %q", token.raw)
	}
}

func TestLexerDataReturnsDefensiveCopy(t *testing.T) {
	lexer := NewLexer([]byte("12"))
	data := lexer.Data()
	data[0] = '9'
	if got := string(lexer.Data()); got != "12" {
		t.Fatalf("Lexer.Data aliases lexer storage: %q", got)
	}
}

func TestLexerDataBorrowedSharesLexerStorage(t *testing.T) {
	lexer := NewLexer([]byte("12"))
	data := lexer.DataBorrowed()
	data[0] = '9'
	if got := string(lexer.DataBorrowed()); got != "92" {
		t.Fatalf("Lexer.DataBorrowed does not expose borrowed storage: %q", got)
	}
}

func TestTokenSnapshotsPreserveEmptyBytes(t *testing.T) {
	token := Token{raw: make([]byte, 0), value: make([]byte, 0)}
	if token.RawCopy() == nil {
		t.Fatal("empty token raw bytes became nil")
	}
	if snapshot := token.Finalize(); snapshot.raw == nil || snapshot.value == nil {
		t.Fatalf("empty token bytes were not preserved: %#v", snapshot)
	}
}

func TestDecodeNameUsesPDFDocEncoding(t *testing.T) {
	if got := decodeName([]byte{0xba, 0xda, 0xcc, 0xe5}); got != "ºÚÌå" {
		t.Fatalf("decoded name = %q", got)
	}
}

func TestDecodeNamePreservesUTF8(t *testing.T) {
	if got := decodeName([]byte("ABCDEE+宋体")); got != "ABCDEE+宋体" {
		t.Fatalf("decoded name = %q", got)
	}
}

func TestLexerPDFTokens(t *testing.T) {
	l := NewLexer([]byte("%x\n/Name#20value [12 -3.5 (hi\\n)] << /K <4142> >>"))
	want := []parserconfig.TokenKind{TokenName, TokenArrayStart, TokenNumber, TokenNumber, TokenString, TokenArrayEnd, TokenDictStart, TokenName, TokenHexString, TokenDictEnd}
	for i, w := range want {
		got, e := l.Next()
		if e != nil || got.Kind() != w {
			t.Fatalf("%d: got %#v err=%v", i, got, e)
		}
	}
}

func TestLexerInvalidHexOpeningFallsBackToKeywords(t *testing.T) {
	l := NewLexer([]byte("<x:xmpmeta>"))
	for _, want := range []string{"<", "x:xmpmeta", ">"} {
		token, err := l.Next()
		if err != nil || token.Kind() != TokenKeyword || token.Text() != want {
			t.Fatalf("token = %#v, err=%v, want keyword %q", token, err, want)
		}
	}
}

func TestLexerClosingParenProgressesAsKeyword(t *testing.T) {
	l := NewLexer([]byte(") next"))
	first, err := l.Next()
	if err != nil || first.Kind() != TokenKeyword || first.Text() != ")" {
		t.Fatalf("closing paren token = %#v, err=%v", first, err)
	}
	second, err := l.Next()
	if err != nil || second.Kind() != TokenKeyword || second.Text() != "next" {
		t.Fatalf("token after closing paren = %#v, err=%v", second, err)
	}
}

func TestSourceLexerPreservesBinaryStreamBytes(t *testing.T) {
	l := NewSourceLexer([]byte{0xff, 0})
	first, err := l.Next()
	if err != nil || first.Kind() != TokenKeyword || first.Text() != "ÿ" {
		t.Fatalf("first binary token = %#v, err=%v", first, err)
	}
	second, err := l.Next()
	if err != nil || second.Kind() != TokenKeyword || second.Text() != "\x00" {
		t.Fatalf("second binary token = %#v, err=%v", second, err)
	}
}

func TestSourceLexerUsesPlayaKeywordBoundaries(t *testing.T) {
	l := NewSourceLexer([]byte("x\x9c\xad\x8f\xbb\x02\x01 12abc"))
	want := []struct {
		kind parserconfig.TokenKind
		text string
		num  float64
	}{
		{kind: TokenKeyword, text: "x\u009c\u00ad\u008f\u00bb\x02\x01"},
		{kind: TokenNumber, text: "12", num: 12},
		{kind: TokenKeyword, text: "abc"},
	}
	for i, expected := range want {
		got, err := l.Next()
		if err != nil || got.Kind() != expected.kind || got.Text() != expected.text || got.Number() != expected.num {
			t.Fatalf("token %d = %#v, err=%v, want kind=%v text=%q number=%v", i, got, err, expected.kind, expected.text, expected.num)
		}
	}
}

func TestSourceLexerUsesUTF8OrLatin1ForNames(t *testing.T) {
	l := NewSourceLexer([]byte("/\xff /\xe4"))
	first, err := l.Next()
	if err != nil || first.Kind() != TokenName || first.Text() != "\u00ff" {
		t.Fatalf("first source name = %#v, err=%v", first, err)
	}
	second, err := l.Next()
	if err != nil || second.Kind() != TokenName || second.Text() != "\u00e4" {
		t.Fatalf("second source name = %#v, err=%v", second, err)
	}
}

func TestSourceLexerNormalizesStringLineEndings(t *testing.T) {
	l := NewSourceLexer([]byte("(a\rb\r\nc\n)"))
	token, err := l.Next()
	if err != nil || token.Kind() != TokenString || string(token.ValueCopy()) != "a\nb\nc\n" {
		t.Fatalf("source string = %#v, err=%v", token, err)
	}
}

func TestSourceLexerTreatsCRAsCommentEnd(t *testing.T) {
	l := NewSourceLexer([]byte("% comment\r12"))
	token, err := l.Next()
	if err != nil || token.Kind() != TokenNumber || token.Number() != 12 {
		t.Fatalf("token after CR comment = %#v, err=%v", token, err)
	}
}

func TestLexerNumberPrecheckPreservesKeywords(t *testing.T) {
	for _, value := range []string{"0", "-3.5", ".25", "+1e2"} {
		if !looksLikeNumber([]byte(value)) {
			t.Fatalf("looksLikeNumber(%q) = false", value)
		}
	}
	for _, value := range []string{"BT", "1abc", "+", "-"} {
		if looksLikeNumber([]byte(value)) {
			t.Fatalf("looksLikeNumber(%q) = true", value)
		}
	}
	l := NewLexer([]byte("BT 1abc"))
	first, err := l.Next()
	if err != nil || first.Kind() != TokenKeyword || first.Text() != "BT" {
		t.Fatalf("first token = %#v, err=%v", first, err)
	}
	second, err := l.Next()
	if err != nil || second.Kind() != TokenKeyword || second.Text() != "1abc" {
		t.Fatalf("second token = %#v, err=%v", second, err)
	}
}

func TestLexerPDFStringEscapes(t *testing.T) {
	l := NewLexer([]byte("(a\\101\\(b\\)\\\\c\\b\\f\\\n d)"))
	tok, e := l.Next()
	if e != nil || tok.Text() != "aA(b)\\c\b\f d" {
		t.Fatalf("%q %v", tok.Text(), e)
	}
}

func TestLexerPDFStringIgnoresOctalOverflowLikePlaya(t *testing.T) {
	l := NewLexer([]byte("(\\466A)"))
	tok, err := l.Next()
	if err != nil {
		t.Fatal(err)
	}
	if got := string(tok.ValueCopy()); got != "A" {
		t.Fatalf("octal overflow value = %q, want %q", got, "A")
	}
}
