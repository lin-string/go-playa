package document

import (
	"os"
	"testing"

	"github.com/lin-string/go-playa/internal/testfixture"
)

func TestTaggedTextAcceptanceFixture(t *testing.T) {
	data, err := os.ReadFile(testfixture.Path(t, "acceptance_tagged_text.pdf"))
	if err != nil {
		t.Fatal(err)
	}
	d, err := OpenBytes(data)
	if err != nil {
		t.Fatal(err)
	}
	page, err := d.PageAt(0)
	if err != nil {
		t.Fatal(err)
	}
	tree, err := d.StructureTree()
	if err != nil {
		t.Fatal(err)
	}
	if len(tree) != 1 || tree[0].Role() != "P" || tree[0].Title() != "Financial summary" {
		t.Fatalf("tagged structure = %#v", tree)
	}
	texts, err := d.PageText(page)
	if err != nil || len(texts) != 1 || texts[0].Text() != "AB" || !texts[0].HasMCID() || texts[0].MCID() != 0 {
		t.Fatalf("tagged text = %#v, err=%v", texts, err)
	}
	parent := texts[0].Parent(d)
	if parent == nil || parent.Role() != "P" || parent.Title() != "Financial summary" {
		t.Fatalf("tagged text parent = %#v", parent)
	}
}
