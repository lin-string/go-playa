package document

import "testing"

func TestCFFImplicitEncodingOmitsUnencodedASCII(t *testing.T) {
	f := NewSimpleFont("Test")
	f.cffImplicitEncoding = true
	prior := f.decodeSnapshot()
	applyCFFCodeNames(f, map[byte]string{76: "L"}, []string{".notdef", "L"})
	if prior.encoding[65] != 'A' {
		t.Fatal("implicit encoding replacement mutated a published decode snapshot")
	}
	for repeat := 0; repeat < 2; repeat++ {
		if got := f.Decode([]byte{32, 65, 76}); got != "L" {
			t.Fatalf("implicit CFF decoding = %q, want L", got)
		}
	}
}

func TestCFFExplicitEncodingPreservesASCII(t *testing.T) {
	f := NewSimpleFont("Test")
	applyCFFCodeNames(f, map[byte]string{76: "L"}, []string{".notdef", "L"})
	if got := f.Decode([]byte{32, 65, 76}); got != " AL" {
		t.Fatalf("explicit CFF decoding = %q", got)
	}
}
