package document_test

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/lin-string/go-playa/document"
	"github.com/lin-string/go-playa/textconfig"
)

func TestTaggedTextMatchesOracleLineState(t *testing.T) {
	for _, tc := range []struct{ name, content, want string }{
		{"nested ActualText without MCID", "/P << /MCID 0 >> BDC BT /F1 10 Tf 1 0 0 1 10 100 Tm (A) Tj ET /Span << /ActualText (X) >> BDC BT /F1 10 Tf 1 0 0 1 10 80 Tm (hidden) Tj ET EMC BT /F1 10 Tf 1 0 0 1 10 90 Tm (B) Tj ET EMC", "AX\nB"},
		{"same section", "/P << /MCID 0 >> BDC BT /F1 10 Tf 1 0 0 1 10 100 Tm (first) Tj 1 0 0 1 10 80 Tm (second) Tj ET EMC", "firstsecond"},
		{"actual text preserves previous origin", "/P << /MCID 0 >> BDC BT /F1 10 Tf 1 0 0 1 10 100 Tm (A) Tj ET EMC /Span << /MCID 1 /ActualText (X) >> BDC BT /F1 10 Tf 1 0 0 1 10 80 Tm (hidden) Tj ET EMC /P << /MCID 2 >> BDC BT /F1 10 Tf 1 0 0 1 10 90 Tm (B) Tj ET EMC", "AX\nB"},
		{"artifact retains line state with structure", "/P << /MCID 0 >> BDC BT /F1 10 Tf 1 0 0 1 10 100 Tm (A) Tj ET EMC /Artifact BMC BT /F1 10 Tf 1 0 0 1 10 80 Tm (dots) Tj ET EMC /P << /MCID 1 >> BDC BT /F1 10 Tf 1 0 0 1 20 80 Tm (B) Tj ET EMC", "AB"},
		{"reversed section", "/ReversedChars << /MCID 0 >> BDC BT /F1 10 Tf 1 0 0 1 10 100 Tm (AB) Tj 1 0 0 1 10 80 Tm (CD) Tj ET EMC", "BADC"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d, err := document.OpenBytes(taggedPolicyPDF(tc.content, "0 1 2"))
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = d.Close() }()
			for repeat := 0; repeat < 2; repeat++ {
				page, err := d.PageAt(0)
				if err != nil {
					t.Fatal(err)
				}
				got, err := page.ExtractTextTagged(d, textconfig.Options{})
				if err != nil || got != tc.want {
					t.Fatalf("repeat %d tagged=%q err=%v want %q", repeat, got, err, tc.want)
				}
				got, err = page.ExtractText(d, textconfig.Options{})
				if err != nil || got != tc.want {
					t.Fatalf("automatic=%q err=%v want %q", got, err, tc.want)
				}
				d.ReleaseTransientCaches()
			}
		})
	}
}
func taggedPolicyPDF(content, order string) []byte {
	objects := []string{
		"<< /Type /Catalog /Pages 2 0 R /MarkInfo << /Marked true >> /StructTreeRoot 6 0 R >>",
		"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 200 200] /StructParents 0 /Resources << /Font << /F1 5 0 R >> >> /Contents 4 0 R >>",
		fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", len(content), content),
		"<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>",
		"<< /Type /StructTreeRoot /K [7 0 R] >>",
		fmt.Sprintf("<< /Type /StructElem /S /P /P 6 0 R /Pg 3 0 R /K [%s] >>", order),
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
	for _, off := range offsets[1:] {
		fmt.Fprintf(&pdf, "%010d 00000 n \n", off)
	}
	fmt.Fprintf(&pdf, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(offsets), xref)
	return pdf.Bytes()
}

func TestTaggedTextLogicalReorderingRetainsArtifactOrigin(t *testing.T) {
	// Structure reading order remains the documented Go adaptation. The artifact
	// before B still establishes B's incoming line state when B moves after C.
	content := "/P << /MCID 0 >> BDC BT /F1 10 Tf 1 0 0 1 10 100 Tm (A) Tj ET EMC /Artifact BMC BT /F1 10 Tf 1 0 0 1 10 80 Tm (dots) Tj ET EMC /P << /MCID 1 >> BDC BT /F1 10 Tf 1 0 0 1 20 80 Tm (B) Tj ET EMC /P << /MCID 2 >> BDC BT /F1 10 Tf 1 0 0 1 10 140 Tm (C) Tj ET EMC"
	d, err := document.OpenBytes(taggedPolicyPDF(content, "2 1 0"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = d.Close() }()
	for repeat := 0; repeat < 2; repeat++ {
		page, err := d.PageAt(0)
		if err != nil {
			t.Fatal(err)
		}
		text, err := page.ExtractTextTagged(d, textconfig.Options{})
		if err != nil || text != "CBA" {
			t.Fatalf("text=%q err=%v want CBA", text, err)
		}
		d.ReleaseTransientCaches()
	}
}
