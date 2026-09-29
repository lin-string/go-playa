package document_test

import (
	"errors"
	"testing"

	"github.com/lin-string/go-playa/document"
	"github.com/lin-string/go-playa/internal/testfixture"
)

func TestPDFAssociationUnicodePasswordNormalization(t *testing.T) {
	tests := []struct {
		name     string
		fixture  string
		password string
		wantOpen bool
		wantText string
	}{
		{name: "corrigendum 4 source character", fixture: "unicode-password-c4-correct", password: "Password当!", wantOpen: true, wantText: "U+4EE4 U+548C 令和\nU+32FF ㋿"},
		{name: "corrigendum 4 Unicode 3.2 normalized", fixture: "unicode-password-c4-correct", password: "Password弳!", wantOpen: true, wantText: "U+4EE4 U+548C 令和\nU+32FF ㋿"},
		{name: "corrigendum 4 rejects modern normalization", fixture: "unicode-password-c4-wrong", password: "Password当!"},
		{name: "corrigendum 4 modern normalized literal", fixture: "unicode-password-c4-wrong", password: "Password当!", wantOpen: true, wantText: "U+4EE4 U+548C 令和\nU+32FF ㋿"},
		{name: "corrigendum 5 rejects one normalization pass", fixture: "unicode-password-c5-once", password: "가̣̀"},
		{name: "corrigendum 5 rejects unnormalized source", fixture: "unicode-password-c5-twice", password: "ᄀ̀ᅡ̣"},
		{name: "corrigendum 5 accepts one-pass source", fixture: "unicode-password-c5-twice", password: "가̣̀", wantOpen: true, wantText: "U+4EE4 U+458C 令和\nU+32FF ㋿"},
		{name: "corrigendum 5 accepts normalized source", fixture: "unicode-password-c5-twice", password: "가̣̀", wantOpen: true, wantText: "U+4EE4 U+458C 令和\nU+32FF ㋿"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			path := testfixture.PDFAPath(t, test.fixture)
			doc, err := document.Open(path, document.WithPassword(test.password))
			if !test.wantOpen {
				if !errors.Is(err, document.ErrInvalidPassword) {
					if doc != nil {
						_ = doc.Close()
					}
					t.Fatalf("open error = %v, want %v", err, document.ErrInvalidPassword)
				}
				return
			}
			if err != nil {
				t.Fatalf("open with Unicode password: %v", err)
			}
			defer closePDFAssociationFixture(t, doc)
			pages, err := doc.CollectPages()
			if err != nil {
				t.Fatal(err)
			}
			if len(pages) != 1 {
				t.Fatalf("page count = %d, want 1", len(pages))
			}
			text, err := pages[0].ExtractTextUntagged(doc, document.DefaultTextExtractionOptions())
			if err != nil {
				t.Fatalf("extract decrypted text: %v", err)
			}
			if text != test.wantText {
				t.Fatalf("decrypted text = %q, want %q", text, test.wantText)
			}
		})
	}
}
