package fontdata_test

import (
	"testing"

	"github.com/lin-string/go-playa/fontdata"
)

func TestRequiredFontDataReceivers(t *testing.T) {
	for name, run := range map[string]func(){
		"CMap":                func() { _ = (*fontdata.CMap)(nil).Vertical() },
		"ToUnicodeMap":        func() { _, _ = (*fontdata.ToUnicodeMap)(nil).Lookup([]byte{65}) },
		"UnicodeMap":          func() { _ = (*fontdata.UnicodeMap)(nil).Name() },
		"CFF2VariationStore":  func() { _ = (*fontdata.CFF2VariationStore)(nil).RegionCount() },
		"Type1Program":        func() { _ = (*fontdata.Type1Program)(nil).CharString("A") },
		"DecodeCMap callback": func() { fontdata.DecodeCMap(fontdata.NewIdentityCMap(false, 2), []byte{0, 65}, nil) },
	} {
		t.Run(name, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Fatal("nil required input did not trigger a programmer error")
				}
			}()
			run()
		})
	}
}
