package document_test

import (
	"math"
	"testing"

	"github.com/lin-string/go-playa/document"
	"github.com/lin-string/go-playa/pdftypes"
)

func cffWithSpaceWidth() []byte {
	index := func(dst []byte, items ...[]byte) []byte {
		if len(items) == 0 {
			return append(dst, 0, 0)
		}
		dst = append(dst, 0, byte(len(items)), 1, 1)
		offset := 1
		for _, item := range items {
			offset += len(item)
			dst = append(dst, byte(offset))
		}
		for _, item := range items {
			dst = append(dst, item...)
		}
		return dst
	}
	prefix := func(offset byte) []byte {
		data := index([]byte{1, 0, 4, 4}, []byte("Test"))
		data = index(data, []byte{offset, 17})
		return index(index(data))
	}
	data := prefix(byte(139 + len(prefix(139))))
	// ISOAdobe charset glyph 1 is space; its explicit Type2 width is 240.
	return index(data, []byte{14}, []byte{247, 132, 14})
}

func TestCFFWidthsRespectPDFWidthTable(t *testing.T) {
	for _, tc := range []struct {
		name   string
		widths document.Array
		want   float64
	}{
		{"narrow PDF table", document.Array{document.Number(700)}, .9},
		{"empty PDF table", document.Array{}, .9},
		{"no PDF table keeps embedded fallback", nil, .24},
	} {
		t.Run(tc.name, func(t *testing.T) {
			spec := document.Dict{
				"Subtype": document.Name("Type1"), "BaseFont": document.Name("SubsetFont"),
				"Encoding": document.Name("WinAnsiEncoding"), "FirstChar": document.Number(65),
				"FontDescriptor": document.Dict{
					"MissingWidth": document.Number(900),
					"FontFile3":    pdftypes.NewStream(document.Dict{"Subtype": document.Name("Type1C")}, cffWithSpaceWidth()),
				},
			}
			if tc.widths != nil {
				spec["Widths"] = tc.widths
			}
			d := &document.Document{}
			f, err := d.GetFontWithError(0, spec)
			if err != nil {
				t.Fatal(err)
			}
			if got, err := f.HDispWithError(32); err != nil || got != tc.want {
				t.Errorf("first lazy HDisp = %v, %v; want %v", got, err, tc.want)
			}
			if width, present, err := f.WidthValueWithError(32); err != nil || present != (tc.widths == nil) || (present && width != 240) {
				t.Errorf("space WidthValue = %v, %v, %v", width, present, err)
			}
			owned := f.Finalize()
			for _, font := range []*document.Font{f, owned} {
				for repeat := 0; repeat < 2; repeat++ {
					if got, err := font.HDispWithError(32); err != nil || got != tc.want {
						t.Errorf("space HDisp = %v, %v; want %v", got, err, tc.want)
					}
					if got, err := font.WidthCodeWithError([]byte{32}); err != nil || math.Abs(got-1000*tc.want) > 1e-12 {
						t.Errorf("space WidthCode = %v, %v; want %v", got, err, 1000*tc.want)
					}
				}
				if len(tc.widths) > 0 {
					if got, err := font.HDispWithError(65); err != nil || math.Abs(got-.7) > 1e-12 {
						t.Errorf("explicit A HDisp = %v, %v; want .7", got, err)
					}
				}
			}
		})
	}
}
