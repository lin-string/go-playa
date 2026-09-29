package document

import (
	"os"
	"testing"
)

func TestRealPDFSmoke(t *testing.T) {
	path := os.Getenv("GOPLAYA_TEST_PDF")
	if path == "" {
		t.Skip("GOPLAYA_TEST_PDF not set")
	}
	d, e := Open(path)
	if e != nil {
		t.Fatal(e)
	}
	pages, e := d.CollectPages()
	if e != nil {
		t.Fatal(e)
	}
	if len(pages) == 0 {
		t.Fatal("no pages")
	}
	_, e = d.PageText(pages[0])
	if e != nil {
		t.Fatal(e)
	}
}
