package testcompat_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/lin-string/go-playa/internal/testcompat"
)

func TestLayoutProjectionNormalizesTrailingNewlines(t *testing.T) {
	content := "BT /F1 10 Tf 10 50 Td <410101> Tj ET"
	cmap := "1 begincodespacerange <00> <ff> endcodespacerange 2 beginbfchar <41> <0041> <01> <000a> endbfchar"
	doc, err := openPagePDF(testPDF(
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 200 100] /Resources << /Font << /F1 5 0 R >> >> /Contents 4 0 R >>",
		fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", len(content), content),
		"<< /Type /Font /Subtype /Type1 /BaseFont /Times-Roman /Encoding /WinAnsiEncoding /ToUnicode 6 0 R >>",
		fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", len(cmap), cmap),
	))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := doc.Close(); err != nil {
			t.Error(err)
		}
	})
	snapshot, err := testcompat.Snapshot(doc, testcompat.Options{Pages: []int{0}, Sections: []string{"layout", "content.text"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Pages) != 1 || snapshot.Pages[0].Layout == nil {
		t.Fatal("missing layout")
	}
	layout := snapshot.Pages[0].Layout
	if len(layout.Lines) != 1 {
		t.Fatalf("lines = %d, want 1", len(layout.Lines))
	}
	if got := layout.Lines[0].Text; got != "A" {
		t.Errorf("normalized line = %q, want A", got)
	}
	var check func(testcompat.LayoutNode)
	check = func(node testcompat.LayoutNode) {
		if strings.HasSuffix(node.Text, "\n") {
			t.Errorf("%s ends in newline: %q", node.Kind, node.Text)
		}
		for _, child := range node.Children {
			check(child)
		}
	}
	for _, box := range layout.TextBoxes {
		check(box)
	}
	for _, group := range layout.TextGroups {
		check(group)
	}
	// Normalization belongs to the layout projection, not extracted characters.
	if got := snapshot.Pages[0].Text[0].Chars; got != "A\n\n" {
		t.Errorf("raw text = %q, want preserved newlines", got)
	}
}
