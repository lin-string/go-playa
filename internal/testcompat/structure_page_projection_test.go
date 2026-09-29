package testcompat_test

import (
	"testing"

	"github.com/lin-string/go-playa/internal/testcompat"
)

func TestSnapshotResolvesNestedStructurePagesOutsideSelectedPages(t *testing.T) {
	doc, err := openPagePDF(testPDF(
		"<< /Type /Catalog /Pages 2 0 R /StructTreeRoot 5 0 R >>",
		"<< /Type /Pages /Kids [3 0 R 4 0 R] /Count 2 >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 100 100] /StructParents 0 >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 100 100] >>",
		"<< /Type /StructTreeRoot /ParentTree 6 0 R >>",
		"<< /Nums [0 [7 0 R]] >>",
		"<< /Type /StructElem /S /P /Pg 3 0 R /K [8 0 R] >>",
		"<< /Type /StructElem /S /Span /P 7 0 R /Pg 4 0 R >>",
	))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = doc.Close() }()
	page, err := doc.PageAt(0)
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := testcompat.PageSnapshotSections(doc, page, 0, []string{"content.structure"})
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Structure) != 1 || len(snapshot.Structure[0].Children) != 1 {
		t.Fatalf("structure = %#v, want one nested element", snapshot.Structure)
	}
	if got := snapshot.Structure[0].Children[0].PageIndex; got != 1 {
		t.Fatalf("structure child page = %d, want 1", got)
	}
}
