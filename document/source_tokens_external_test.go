package document_test

import (
	"bytes"
	"fmt"
	"iter"
	"reflect"
	"testing"

	"github.com/lin-string/go-playa/document"
)

func TestPublicContentTokensPreserveNullAndRepeat(t *testing.T) {
	content := "BI /W 1 /H 1 /IM true /BPC 1 ID\n\x00 EI"
	pageContent := content + " /Form Do"
	objects := []string{
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 10 10] /Resources << /XObject << /Form 5 0 R >> >> /Contents 4 0 R >>",
		fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", len(pageContent), pageContent),
		fmt.Sprintf("<< /Type /XObject /Subtype /Form /BBox [0 0 10 10] /Resources << >> /Length %d >>\nstream\n%s\nendstream", len(content), content),
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
	d, err := document.OpenBytes(pdf.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := d.Close(); err != nil {
			t.Error(err)
		}
	})
	page, err := d.PageAt(0)
	if err != nil {
		t.Fatal(err)
	}
	check := func(t *testing.T, seq iter.Seq2[document.Token, error], wantCount int) {
		t.Helper()
		var first document.Token
		for token, err := range seq {
			if err != nil {
				t.Fatal(err)
			}
			first = token.Finalize()
			break
		}
		var previous []string
		for attempt := 0; attempt < 2; attempt++ {
			var got []string
			for token, err := range seq {
				if err != nil {
					t.Fatal(err)
				}
				got = append(got, token.Text())
			}
			if len(got) != wantCount || got[10] != "\x00" || got[11] != "EI" {
				t.Fatalf("tokens = %q, want %d tokens with NUL then EI at indexes 10 and 11", got, wantCount)
			}
			if attempt > 0 && !reflect.DeepEqual(got, previous) {
				t.Fatalf("repeated tokens = %q, want %q", got, previous)
			}
			previous = got
		}
		if first.Text() != "BI" {
			t.Fatalf("retained first token = %q, want BI", first.Text())
		}
	}
	t.Run("page", func(t *testing.T) { check(t, page.Tokens(d), 14) })
	count := 0
	for form, err := range page.XObjects(d) {
		if err != nil {
			t.Fatal(err)
		}
		count++
		t.Run("form", func(t *testing.T) { check(t, form.Tokens(d), 12) })
	}
	if count != 1 {
		t.Fatalf("forms = %d, want 1", count)
	}
}
