package fontdata

import (
	"encoding/hex"
	"testing"
)

func TestParseType1EncodingReadsAssignments(t *testing.T) {
	data := []byte("%!PS-AdobeFont-1.0\n/Encoding 256 array\ndup 65 /Aacute put\ndup 66 /custom put\nreadonly def\n")
	got := ParseType1Encoding(data)
	if got[65] != "Aacute" || got[66] != "custom" {
		t.Fatalf("Type1 encoding = %#v", got)
	}
}

func TestParseType1EncodingRejectsOutOfRangeCodes(t *testing.T) {
	data := []byte("dup 1e100 /tooLarge put\ndup 65 /A put\n")
	got := ParseType1Encoding(data)
	if len(got) != 1 || got[65] != "A" {
		t.Fatalf("Type1 encoding accepted an out-of-range code: %#v", got)
	}
}

func TestParseType1EexecEncodingReadsHexPayload(t *testing.T) {
	cleartext := []byte("%!PS-AdobeFont-1.0\neexec\n")
	plain := append([]byte{0, 0, 0, 0}, []byte("dup 66 /B put\n")...)
	hexPayload := []byte(hex.EncodeToString(encryptType1EexecForTest(plain)))
	data := append(append([]byte(nil), cleartext...), hexPayload...)
	got := ParseType1EexecEncoding(data, len(cleartext))
	if got[66] != "B" {
		t.Fatalf("hex eexec Type1 encoding = %#v", got)
	}
}

func encryptType1EexecForTest(plain []byte) []byte {
	const c1, c2 = uint32(52845), uint32(22719)
	r := uint32(55665)
	out := make([]byte, len(plain))
	for i, value := range plain {
		cipher := uint32(value) ^ (r >> 8)
		out[i] = byte(cipher)
		r = (cipher+r)*c1 + c2
		r &= 0xffff
	}
	return out
}
