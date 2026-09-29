package playa_test

import (
	"fmt"
	"strings"
	"testing"

	playa "github.com/lin-string/go-playa"
)

func TestREADMEQuickStart(t *testing.T) {
	const content = "BT /F1 12 Tf 20 50 Td (Hello, go-playa!) Tj ET\n"
	objects := []string{
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 200 100] /Resources << /Font << /F1 4 0 R >> >> /Contents 5 0 R >>",
		"<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>",
		fmt.Sprintf("<< /Length %d >>\nstream\n%sendstream", len(content), content),
	}
	var pdf strings.Builder
	pdf.WriteString("%PDF-1.4\n")
	offsets := make([]int, len(objects))
	for i, object := range objects {
		offsets[i] = pdf.Len()
		fmt.Fprintf(&pdf, "%d 0 obj\n%s\nendobj\n", i+1, object)
	}
	xref := pdf.Len()
	fmt.Fprintf(&pdf, "xref\n0 %d\n0000000000 65535 f \n", len(objects)+1)
	for _, offset := range offsets {
		fmt.Fprintf(&pdf, "%010d 00000 n \n", offset)
	}
	fmt.Fprintf(&pdf, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(objects)+1, xref)

	doc, err := playa.OpenBytes([]byte(pdf.String()))
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := doc.Close(); err != nil {
			t.Error(err)
		}
	}()
	if got := doc.PageCount(); got != 1 {
		t.Fatalf("PageCount() = %d, want 1", got)
	}
	pages := 0
	for page, err := range doc.Pages() {
		if err != nil {
			t.Fatal(err)
		}
		pages++
		text, err := page.ExtractText(doc, playa.DefaultTextExtractionOptions())
		if err != nil {
			t.Fatal(err)
		}
		if got := strings.TrimSpace(text); got != "Hello, go-playa!" {
			t.Fatalf("ExtractText() = %q, want %q", got, "Hello, go-playa!")
		}
	}
	if pages != 1 {
		t.Fatalf("Pages() yielded %d pages, want 1", pages)
	}
}
