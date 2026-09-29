package document

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/lin-string/go-playa/internal/testfixture"
	"github.com/lin-string/go-playa/textconfig"
)

func TestRecoveryPDFFixturesResolvePages(t *testing.T) {
	tests := []struct {
		path          string
		wantPage      bool
		wantRecovered bool
		wantText      string
		wantWidth     float64
		wantRevisions int
	}{
		{path: testfixture.Path(t, "recovery_classic_to_xref_stream.pdf"), wantPage: true, wantRecovered: false},
		{path: testfixture.Path(t, "recovery_xref_stream_to_classic.pdf"), wantPage: true, wantRecovered: false},
		{path: testfixture.Path(t, "recovery_object_stream.pdf"), wantPage: true, wantRecovered: false},
		{path: testfixture.Path(t, "recovery_damaged_scan.pdf"), wantPage: true, wantRecovered: true},
		// A malformed xref stream falls back to damaged-object scanning because
		// the page object is still recoverable outside the xref table.
		{path: testfixture.Path(t, "recovery_malformed_xref_stream.pdf"), wantPage: true, wantRecovered: true},
		// An oversized /First remains recoverable when the indexed object payload
		// is structurally present, matching Playa's tolerant object-stream parse.
		{path: testfixture.Path(t, "recovery_malformed_object_stream.pdf"), wantPage: true, wantRecovered: false},
		{path: testfixture.Path(t, "recovery_damaged_text.pdf"), wantPage: true, wantRecovered: true, wantText: "Recovered text"},
		{path: testfixture.Path(t, "recovery_damaged_object_stream.pdf"), wantPage: true, wantRecovered: true},
		{path: testfixture.Path(t, "recovery_incremental_object_stream.pdf"), wantPage: true, wantRecovered: false, wantWidth: 612, wantRevisions: 2},
	}
	for _, test := range tests {
		t.Run(filepath.Base(test.path), func(t *testing.T) {
			data, err := os.ReadFile(test.path)
			if err != nil {
				t.Fatal(err)
			}
			d, err := OpenBytes(data)
			if err != nil {
				if test.wantPage {
					t.Fatal(err)
				}
				return
			}
			if got := d.WasRecovered(); got != test.wantRecovered {
				t.Fatalf("WasRecovered() = %v, want %v", got, test.wantRecovered)
			}
			if test.wantRevisions != 0 {
				if got := d.RevisionCount(); got != test.wantRevisions {
					t.Fatalf("RevisionCount() = %d, want %d", got, test.wantRevisions)
				}
			}
			if test.wantRecovered && d.RecoveryError() == nil {
				t.Fatal("recovered fixture has no original xref error")
			}
			d.ReleaseTransientCaches()
			if got := d.WasRecovered(); got != test.wantRecovered {
				t.Fatalf("WasRecovered() after cache release = %v, want %v", got, test.wantRecovered)
			}
			if test.wantRecovered && (len(d.xrefs) == 0 || len(d.objects) != 0) {
				t.Fatalf("recovered cache release did not retain rebuildable xrefs: xrefs=%d objects=%d", len(d.xrefs), len(d.objects))
			}
			page, err := d.PageAt(0)
			if !test.wantPage {
				if err == nil {
					t.Fatal("malformed recovery fixture unexpectedly resolved a page")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got := page.MediaBox(d); got != [4]float64{0, 0, 612, 792} && got != [4]float64{0, 0, 595, 842} {
				t.Fatalf("page box = %v", got)
			}
			if test.wantWidth != 0 {
				if got := page.MediaBox(d); got[2] != test.wantWidth {
					t.Fatalf("page width = %v, want %.0f", got[2], test.wantWidth)
				}
			}
			if test.wantText != "" {
				text, err := page.ExtractText(d, textconfig.Options{})
				if err != nil {
					t.Fatalf("ExtractText() error = %v", err)
				}
				if text != test.wantText {
					t.Fatalf("ExtractText() = %q, want %q", text, test.wantText)
				}
			}
		})
	}
}

func TestRecoveryPDFFixturesRemainRepeatableAfterCacheRelease(t *testing.T) {
	paths := []string{
		testfixture.Path(t, "recovery_classic_to_xref_stream.pdf"),
		testfixture.Path(t, "recovery_xref_stream_to_classic.pdf"),
		testfixture.Path(t, "recovery_object_stream.pdf"),
		testfixture.Path(t, "recovery_damaged_scan.pdf"),
		testfixture.Path(t, "recovery_malformed_xref_stream.pdf"),
		testfixture.Path(t, "recovery_malformed_object_stream.pdf"),
		testfixture.Path(t, "recovery_damaged_text.pdf"),
		testfixture.Path(t, "recovery_damaged_object_stream.pdf"),
		testfixture.Path(t, "recovery_incremental_object_stream.pdf"),
	}
	for _, path := range paths {
		t.Run(filepath.Base(path), func(t *testing.T) {
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			d, err := OpenBytes(data)
			if err != nil {
				t.Fatal(err)
			}
			if got := d.PageCount(); got != 1 {
				t.Fatalf("PageCount() = %d, want 1", got)
			}
			before, err := d.PageAt(0)
			if err != nil {
				t.Fatal(err)
			}
			wantBox := before.MediaBox(d)

			d.ReleaseTransientCaches()

			after, err := d.PageAt(0)
			if err != nil {
				t.Fatal(err)
			}
			if got := after.MediaBox(d); got != wantBox {
				t.Fatalf("MediaBox() after cache release = %v, want %v", got, wantBox)
			}
			if got := d.PageCount(); got != 1 {
				t.Fatalf("PageCount() after cache release = %d, want 1", got)
			}
		})
	}
}
