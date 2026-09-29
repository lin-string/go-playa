package testcompat_test

import (
	"fmt"
	"testing"

	"github.com/lin-string/go-playa/internal/testcompat"
)

func TestPathParentProjectionUsesEnclosingMCID(t *testing.T) {
	content := "/P << /MCID 0 >> BDC /PlacedPDF BMC 0 0 10 10 re f EMC EMC"
	doc, err := openPagePDF(testPDF(
		"<< /Type /Catalog /Pages 2 0 R /StructTreeRoot 5 0 R >>",
		"<< /Type /Pages /Kids [3 0 R 4 0 R] /Count 2 >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 100 100] /StructParents 0 /Contents 9 0 R >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 100 100] >>",
		"<< /Type /StructTreeRoot /ParentTree 6 0 R >>",
		"<< /Nums [0 [7 0 R]] >>",
		"<< /Type /StructElem /S /P /Pg 3 0 R /K [0 8 0 R] >>",
		"<< /Type /StructElem /S /Span /P 7 0 R /Pg 4 0 R >>",
		fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", len(content), content),
	))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = doc.Close() }()
	page, err := doc.PageAt(0)
	if err != nil {
		t.Fatal(err)
	}
	for _, section := range []string{"content.flatten", "content.interp"} {
		for repeat := 0; repeat < 2; repeat++ {
			snapshot, err := testcompat.PageSnapshotSections(doc, page, 0, []string{section})
			if err != nil {
				t.Fatal(err)
			}
			records := snapshot.Flatten
			if section == "content.interp" {
				records = snapshot.Interp
			}
			found := false
			for _, record := range records {
				if record.Kind != "path" {
					continue
				}
				found = true
				if record.Parent == nil || len(record.Parent.Children) != 1 {
					t.Fatalf("%s parent=%#v", section, record.Parent)
				}
				if got := record.Parent.Children[0].PageIndex; got != 1 {
					t.Fatalf("%s parent child page=%d, want1", section, got)
				}
			}
			if !found {
				t.Fatalf("%s has no path", section)
			}
		}
	}
}
