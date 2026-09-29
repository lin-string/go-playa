package fontdata_test

import (
	"testing"

	"github.com/lin-string/go-playa/fontdata"
)

func TestParseCFFNumberForms(t *testing.T) {
	tests := []struct {
		name string
		data []byte
		want float64
		end  int
	}{
		{name: "single", data: []byte{139}, want: 0, end: 1},
		{name: "positive", data: []byte{247, 0}, want: 108, end: 2},
		{name: "negative", data: []byte{251, 0}, want: -108, end: 2},
		{name: "shortint", data: []byte{28, 0xff, 0xfe}, want: -2, end: 3},
		{name: "longint", data: []byte{29, 0xff, 0xff, 0xff, 0xfe}, want: -2, end: 5},
		{name: "negative decimal", data: []byte{30, 0xe1, 0x8a, 0x5f}, want: -18.5, end: 4},
		{name: "positive exponent", data: []byte{30, 0x1b, 0x2f}, want: 100, end: 3},
		{name: "negative exponent", data: []byte{30, 0x1c, 0x2f}, want: 0.01, end: 3},
		{name: "real", data: []byte{30, 0x12, 0x3f}, want: 123, end: 3},
		{name: "fixed", data: []byte{255, 0, 1, 0, 0}, want: 1, end: 5},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, end, ok := fontdata.ParseCFFNumber(tt.data, 0)
			if !ok || got != tt.want || end != tt.end {
				t.Fatalf("fontdata.ParseCFFNumber(%v) = %v, %d, %v; want %v, %d, true", tt.data, got, end, ok, tt.want, tt.end)
			}
		})
	}
}

func TestParseCFFOperators(t *testing.T) {
	if op, end, ok := fontdata.ParseCFFOperator([]byte{12, 7}, 0, false); !ok || op != 1207 || end != 2 {
		t.Fatalf("CFF escaped operator = %d, %d, %v", op, end, ok)
	}
	if op, end, ok := fontdata.ParseCFFOperator([]byte{22}, 0, true); !ok || op != 22 || end != 1 {
		t.Fatalf("CFF2 dict operator = %d, %d, %v", op, end, ok)
	}
	if op, end, ok := fontdata.ParseCFFCharStringOperator([]byte{16}, 0, true); !ok || op != 16 || end != 1 {
		t.Fatalf("CFF2 charstring operator = %d, %d, %v", op, end, ok)
	}
	if _, _, ok := fontdata.ParseCFFNumber([]byte{28, 0}, 0); ok {
		t.Fatal("truncated CFF number accepted")
	}
}

func TestParseCFFNumberRejectsReservedRealNibble(t *testing.T) {
	if value, _, ok := fontdata.ParseCFFNumber([]byte{30, 0x1d, 0x2f}, 0); ok {
		t.Fatalf("reserved CFF real nibble accepted as %v", value)
	}
}
