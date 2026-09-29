package document_test

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/lin-string/go-playa/contentconfig"
	"github.com/lin-string/go-playa/document"
)

func TestFormFontScopeIsIndependentOfParentCache(t *testing.T) {
	for _, warm := range []bool{false, true} {
		t.Run(fmt.Sprintf("parentWarm=%v", warm), func(t *testing.T) {
			d, err := document.OpenBytes(formFontScopePDF(), document.WithCoordinateSpace(document.CoordinateSpaceDefault))
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = d.Close() }()
			for repeat := 0; repeat < 2; repeat++ {
				p, err := d.PageAt(0)
				if err != nil {
					t.Fatal(err)
				}
				if warm {
					if _, err := p.ResourcesWithError(d); err != nil {
						t.Fatal(err)
					}
					if _, err := d.PageFonts(p); err != nil {
						t.Fatal(err)
					}
				}
				var retained []document.TextObject
				count := 0
				for object, err := range p.Interp(d, contentconfig.Options{}) {
					if err != nil {
						t.Fatal(err)
					}
					if form := object.XObjectBorrowed(); form != nil {
						n, err := form.Len(d)
						if err != nil || n != 1 {
							t.Fatalf("Form.Len=%d,%v", n, err)
						}
						for text, err := range form.Texts(d) {
							if err != nil {
								t.Fatal(err)
							}
							checkScopedText(t, d, text, "FormFont", [4]float64{30, 39.5, 32, 43})
							retained = append(retained, text.Finalize())
							break
						}
					}
					if text := object.TextBorrowed(); text != nil {
						checkScopedText(t, d, *text, "PageFont", [4]float64{10, 18, 19, 28})
						retained = append(retained, text.Finalize())
						count++
					}
				}
				if count != 1 || len(retained) != 2 {
					t.Fatalf("parent texts=%d retained=%d", count, len(retained))
				}
				// A stopped public iterator must remain independently repeatable.
				for _, err := range p.Interp(d, contentconfig.Options{}) {
					if err != nil {
						t.Fatal(err)
					}
					break
				}
				var flat []document.TextObject
				for object, err := range p.Flatten(d, contentconfig.Options{Filter: contentconfig.FilterText}) {
					if err != nil {
						t.Fatal(err)
					}
					if text := object.TextBorrowed(); text != nil {
						flat = append(flat, text.Finalize())
					}
				}
				if len(flat) != 2 {
					t.Fatalf("flatten text count=%d", len(flat))
				}
				checkScopedText(t, d, flat[0], "FormFont", [4]float64{30, 39.5, 32, 43})
				checkScopedText(t, d, flat[1], "PageFont", [4]float64{10, 18, 19, 28})
				d.ReleaseTransientCaches()
				checkScopedText(t, d, retained[0], "FormFont", [4]float64{30, 39.5, 32, 43})
				checkScopedText(t, d, retained[1], "PageFont", [4]float64{10, 18, 19, 28})
			}
		})
	}
}

func checkScopedText(t *testing.T, d *document.Document, text document.TextObject, name string, bbox [4]float64) {
	t.Helper()
	if text.FontName() != name || text.BBox() != bbox {
		t.Fatalf("font=%s bbox=%v want %s %v", text.FontName(), text.BBox(), name, bbox)
	}
	p, err := text.PageObject(d)
	if err != nil || p.Index() != 0 {
		t.Fatalf("text page=%v err=%v", p, err)
	}
}

func formFontScopePDF() []byte {
	content := "/Fm Do BT /F1 10 Tf 10 20 Td (A) Tj ET"
	form := "BT /F1 10 Tf 30 40 Td (A) Tj ET"
	objects := []string{
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 200 200] /Resources << /Font << /F1 6 0 R >> /XObject << /Fm 5 0 R >> >> /Contents 4 0 R >>",
		fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", len(content), content),
		fmt.Sprintf("<< /Type /XObject /Subtype /Form /BBox [0 0 200 200] /Resources << /Font << /F1 7 0 R >> >> /Length %d >>\nstream\n%s\nendstream", len(form), form),
		"<< /Type /Font /Subtype /Type1 /BaseFont /PageFont /Encoding /WinAnsiEncoding /FirstChar 65 /Widths [900] /FontDescriptor << /Ascent 800 /Descent -200 >> >>",
		"<< /Type /Font /Subtype /Type1 /BaseFont /FormFont /Encoding /WinAnsiEncoding /FirstChar 65 /Widths [200] /FontDescriptor << /Ascent 300 /Descent -50 >> >>",
	}
	var pdf bytes.Buffer
	pdf.WriteString("%PDF-1.7\n")
	offsets := make([]int, len(objects)+1)
	for i, object := range objects {
		offsets[i+1] = pdf.Len()
		fmt.Fprintf(&pdf, "%d 0 obj\n%s\nendobj\n", i+1, object)
	}
	xref := pdf.Len()
	fmt.Fprintf(&pdf, "xref\n0 %d\n0000000000 65535 f \n", len(offsets))
	for _, offset := range offsets[1:] {
		fmt.Fprintf(&pdf, "%010d 00000 n \n", offset)
	}
	fmt.Fprintf(&pdf, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(offsets), xref)
	return pdf.Bytes()
}
