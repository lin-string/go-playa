package document

import (
	"os"
	"testing"

	"github.com/lin-string/go-playa/internal/testfixture"
)

func TestMaliciousCMapPDFDoesNotExpandHugeRange(t *testing.T) {
	data, err := os.ReadFile(testfixture.Path(t, "malicious_cmap.pdf"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if recovered := recover(); recovered != nil {
			t.Fatalf("malicious CMap PDF caused panic: %v", recovered)
		}
	}()
	d, err := OpenBytes(data)
	if err != nil {
		t.Fatal(err)
	}
	var pages []Page
	for page, pageErr := range d.Pages() {
		if pageErr != nil {
			t.Fatal(pageErr)
		}
		pages = append(pages, page)
	}
	if len(pages) != 1 {
		t.Fatalf("malicious CMap pages = %d", len(pages))
	}
	if _, err := d.PageText(pages[0]); err != nil {
		t.Fatalf("malicious CMap page text = %v", err)
	}
}
