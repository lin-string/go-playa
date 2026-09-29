package fontdata

import "testing"

func TestParseCFFEncodingMapsPredefinedAndSupplementalCodes(t *testing.T) {
	names := []string{".notdef", "space", "A"}
	standard := ParseCFFEncoding(nil, 0, names, nil)
	if standard[65] != "A" {
		t.Fatalf("CFF StandardEncoding code 65 = %q", standard[65])
	}

	custom := []byte{0, 0, 0, 0, 1, 65}
	if got := ParseCFFEncoding(custom, 3, names, nil); got[65] != "space" {
		t.Fatalf("CFF custom encoding code 65 = %q", got[65])
	}

	supplemental := []byte{0, 0, 0, 0x80, 0, 1, 66, 0, 34}
	if got := ParseCFFEncoding(supplemental, 3, names, nil); got[66] != "A" {
		t.Fatalf("CFF supplemental encoding code 66 = %q", got[66])
	}
	for _, data := range [][]byte{
		{0, 0, 0, 0, 3, 65, 66, 67},
		{0, 0, 0, 1, 1, 65, 3},
		{0, 0, 0, 0x80, 0},
	} {
		if got := ParseCFFEncoding(data, 3, names, nil); got != nil {
			t.Fatalf("malformed CFF encoding accepted: %#v", got)
		}
	}
}
