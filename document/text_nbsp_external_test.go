package document_test

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/lin-string/go-playa/document"
)

func TestPublicTextPreservesExplicitNonbreakingSpace(t *testing.T) {
	for _, tc := range []struct{ name, mapping, encoding, source, want string }{
		{"ToUnicode", "/ToUnicode 6 0 R", "/WinAnsiEncoding", "41", "\u00a0"},
		{"Differences", "", "<< /BaseEncoding /WinAnsiEncoding /Differences [65 /nbspace] >>", "41", "\u00a0"},
		{"WinAnsi", "", "/WinAnsiEncoding", "a0", " "},
		{"MacRoman", "", "/MacRomanEncoding", "ca", " "},
	} {
		t.Run(tc.name, func(t *testing.T) {
			content := "BT /F1 12 Tf <" + tc.source + "> Tj ET"
			cmap := "begincmap 1 begincodespacerange <00> <ff> endcodespacerange 1 beginbfchar <41> <00a0> endbfchar endcmap"
			objects := []string{
				"<< /Type /Catalog /Pages 2 0 R >>",
				"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
				"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 100 100] /Resources << /Font << /F1 5 0 R >> >> /Contents 4 0 R >>",
				fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", len(content), content),
				"<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica /Encoding " + tc.encoding + " " + tc.mapping + " >>",
				fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", len(cmap), cmap),
			}
			var pdf bytes.Buffer
			pdf.WriteString("%PDF-1.7\n")
			offsets := make([]int, len(objects)+1)
			for i, obj := range objects {
				offsets[i+1] = pdf.Len()
				fmt.Fprintf(&pdf, "%d 0 obj\n%s\nendobj\n", i+1, obj)
			}
			xref := pdf.Len()
			fmt.Fprintf(&pdf, "xref\n0 %d\n0000000000 65535 f \n", len(offsets))
			for _, offset := range offsets[1:] {
				fmt.Fprintf(&pdf, "%010d 00000 n \n", offset)
			}
			fmt.Fprintf(&pdf, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(offsets), xref)
			d, err := document.OpenBytes(pdf.Bytes())
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = d.Close() }()
			page, err := d.PageAt(0)
			if err != nil {
				t.Fatal(err)
			}
			for pass := 0; pass < 2; pass++ {
				var text, glyphs string
				for item, err := range page.Texts(d) {
					if err != nil {
						t.Fatal(err)
					}
					text += item.Chars()
				}
				for item, err := range page.Glyphs(d) {
					if err != nil {
						t.Fatal(err)
					}
					glyphs += item.Chars()
				}
				if text != tc.want || glyphs != tc.want {
					t.Fatalf("pass %d: text=%q glyphs=%q, want %q", pass, text, glyphs, tc.want)
				}
			}
		})
	}
}
