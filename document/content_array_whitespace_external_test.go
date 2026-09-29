package document_test

import (
	"bytes"
	"fmt"
	"reflect"
	"testing"

	"github.com/lin-string/go-playa/contentconfig"
	"github.com/lin-string/go-playa/document"
)

func TestPageTextArraysAcceptLeadingWhitespaceInForms(t *testing.T) {
	for _, prefix := range []string{"\n", "\r", "\r\n", "%comment\n"} {
		t.Run(fmt.Sprintf("%q", prefix), func(t *testing.T) {
			d, err := document.OpenBytes(contentArrayWhitespacePDF(prefix))
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = d.Close() }()
			for repeat := 0; repeat < 2; repeat++ {
				page, err := d.PageAt(0)
				if err != nil {
					t.Fatal(err)
				}
				for text, err := range page.Texts(d) {
					if err != nil {
						t.Fatal(err)
					}
					if text.Text() != "AB" {
						t.Fatalf("first text=%q, want AB", text.Text())
					}
					break
				}
				var retained []document.TextObject
				for text, err := range page.Texts(d) {
					if err != nil {
						t.Fatal(err)
					}
					retained = append(retained, text.Finalize())
				}
				if len(retained) != 2 || retained[0].Text() != "AB" || retained[1].Text() != "C" {
					t.Fatalf("text objects=%v, want AB,C", retained)
				}
				var flattened []string
				for object, err := range page.Flatten(d, contentconfig.Options{Filter: contentconfig.FilterText}) {
					if err != nil {
						t.Fatal(err)
					}
					if text := object.TextBorrowed(); text != nil {
						flattened = append(flattened, text.Text())
					}
				}
				if !reflect.DeepEqual(flattened, []string{"AB", "C"}) {
					t.Fatalf("flattened=%v, want AB,C", flattened)
				}
				d.ReleaseTransientCaches()
				if retained[0].Text() != "AB" || retained[0].Len() != 2 {
					t.Fatal("retained text changed after cache release")
				}
			}
		})
	}
}

func contentArrayWhitespacePDF(prefix string) []byte {
	content := "BT /F1 10 Tf 1 0 0 1 10 100 Tm [" + prefix + "(A) -100 (B)] TJ ET BT /F1 10 Tf 1 0 0 1 10 80 Tm (C) Tj ET"
	objects := []string{
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 200 200] /Resources << /XObject << /Fm 5 0 R >> >> /Contents 4 0 R >>",
		"<< /Length 6 >>\nstream\n/Fm Do\nendstream",
		fmt.Sprintf("<< /Type /XObject /Subtype /Form /BBox [0 0 200 200] /Resources << /Font << /F1 6 0 R >> >> /Length %d >>\nstream\n%s\nendstream", len(content), content),
		"<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>",
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
