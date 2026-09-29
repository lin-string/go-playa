package document

import (
	"os"
	"testing"

	"github.com/lin-string/go-playa/internal/testfixture"
)

func TestCJKCIDAcceptanceFixture(t *testing.T) {
	data, err := os.ReadFile(testfixture.Path(t, "acceptance_cjk_cid.pdf"))
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
	texts, err := d.PageText(page)
	if err != nil || len(texts) != 1 || texts[0].Text() != "中国" {
		t.Fatalf("CJK text = %#v, err=%v", texts, err)
	}
	if glyphs := texts[0].GlyphsCopy(); len(glyphs) != 2 || glyphs[0].Text() != "中" || glyphs[1].Text() != "国" {
		t.Fatalf("CJK glyphs = %#v", glyphs)
	}
}
