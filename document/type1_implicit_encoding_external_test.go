package document_test

import (
	"testing"

	"github.com/lin-string/go-playa/document"
	"github.com/lin-string/go-playa/pdftypes"
)

func TestType1ImplicitEncodingReplacesBaseAndFallsBackOnlyWhenEmpty(t *testing.T) {
	for _, tc := range []struct{ name, program, want string }{
		{"partial meaningful encoding", "dup 66 /B put\n", "B"},
		{"unknown only uses StandardEncoding", "dup 115 /radicalBigg put\n", " ABs"},
		{"unknown alongside meaningful stays empty", "dup 66 /B put\ndup 115 /radicalBigg put\n", "B"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, finalizeFirst := range []bool{false, true} {
				spec := document.Dict{"Subtype": document.Name("Type1"), "BaseFont": document.Name("EmbeddedSubset"), "FontDescriptor": document.Dict{"FontFile": pdftypes.NewStream(document.Dict{"Length1": document.Number(len(tc.program))}, []byte(tc.program))}}
				font, err := (&document.Document{}).GetFontWithError(0, spec)
				if err != nil {
					t.Fatal(err)
				}
				if finalizeFirst {
					font = font.Finalize()
				}
				for repeat := 0; repeat < 2; repeat++ {
					var text string
					count := 0
					for glyph, err := range font.DecodeGlyphsSeqWithError([]byte(" ABs")) {
						if err != nil {
							t.Fatal(err)
						}
						text += glyph.Text()
						count++
					}
					if text != tc.want || count != 4 {
						t.Errorf("decode=%q glyphs=%d want%q/4 finalizeFirst=%v", text, count, tc.want, finalizeFirst)
					}
				}
				owned := font.Finalize()
				if got := owned.Decode([]byte(" ABs")); got != tc.want {
					t.Errorf("owned decode=%q want%q", got, tc.want)
				}
			}
		})
	}
}

func TestType1ImplicitDifferencesFollowEmbeddedBase(t *testing.T) {
	program := []byte("/Encoding StandardEncoding def\n")
	differences := document.Array{document.Number(66), document.Name("B")}
	spec := document.Dict{"Subtype": document.Name("Type1"), "BaseFont": document.Name("EmbeddedDifferences"),
		"Encoding":       document.Dict{"Differences": differences},
		"FontDescriptor": document.Dict{"FontFile": pdftypes.NewStream(document.Dict{"Length1": document.Number(len(program))}, program)}}
	font, err := (&document.Document{}).GetFontWithError(0, spec)
	if err != nil {
		t.Fatal(err)
	}
	differences[1] = document.Name("A")
	if got := font.Decode([]byte(" ABs")); got != "B" {
		t.Errorf("lazy implicit differences decode=%q want B", got)
	}
	for _, f := range []*document.Font{font, font.Finalize()} {
		if got := f.Decode([]byte(" ABs")); got != "B" {
			t.Errorf("implicit differences decode=%q want B", got)
		}
	}
}
