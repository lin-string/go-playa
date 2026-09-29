package parser

import "testing"

func TestDecodePDFTextUTF16(t *testing.T) {
	if got := decodePDFText([]byte{0xfe, 0xff, 0x4e, 0x2d}); got != "中" {
		t.Fatalf("%q", got)
	}
}

func TestDecodePDFTextUsesPlayaPDFDocEncodingControlRange(t *testing.T) {
	input := []byte{0x16, 0x17, 0x18, 0x19, 0x1a, 0x1b, 0x1c, 0x1d, 0x1e, 0x1f}
	want := "\u0017\u0017\u02d8\u02c7\u02c6\u02d9\u02dd\u02db\u02da\u02dc"
	if got := DecodePDFText(input); got != want {
		t.Fatalf("PDFDocEncoding = %q, want %q", got, want)
	}
}
func TestDecodePDFDocEncoding(t *testing.T) {
	if got := decodePDFText([]byte{0x80, 0x82, 0xa0, 0x9c}); got != "•‡€œ" {
		t.Fatalf("%q", got)
	}
}

func TestDecodePDFDocEncodingExtendedSlots(t *testing.T) {
	if got := decodePDFText([]byte{0x16, 0x17, 0x18, 0x1e, 0x1f}); got != "\u0017\u0017˘˚˜" {
		t.Fatalf("PDFDocEncoding extended slots = %q", got)
	}
}
