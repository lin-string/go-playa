package fontdata_test

import (
	"testing"

	"github.com/lin-string/go-playa/fontdata"
)

func TestToUnicodeCodespaceDoesNotDefineLowerBoundMapping(t *testing.T) {
	for _, separator := range []string{"\n", " "} {
		data := []byte("begincmap\n1 begincodespacerange" + separator + "<0000> <FFFF>" + separator + "endcodespacerange" + separator + "1 beginbfchar" + separator + "<0001> <0041>" + separator + "endbfchar\nendcmap")
		m, err := fontdata.ParseToUnicodeMap(data)
		if err != nil {
			t.Fatal(err)
		}
		if got, ok := m.Lookup([]byte{0, 0}); ok {
			t.Fatalf("codespace lower bound became mapping %q", got)
		}
		if got := m.Decode([]byte{0, 0, 0, 1}); got != "\x00A" {
			t.Fatalf("undefined code fallback = %q", got)
		}
		strict, err := fontdata.ParseToUnicodeCodesStrict(data)
		if err != nil {
			t.Fatal(err)
		}
		if len(strict) != 1 {
			t.Fatalf("strict mapping count = %d", len(strict))
		}
	}
}
