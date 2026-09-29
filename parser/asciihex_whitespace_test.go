package parser

import "testing"

func TestASCIIHexDecodeAcceptsPDFWhitespace(t *testing.T) {
	got, err := asciiHexDecode([]byte("4\f1\x0030>"))
	if err != nil || string(got) != "A0" {
		t.Fatalf("decoded=%q err=%v", got, err)
	}
}
