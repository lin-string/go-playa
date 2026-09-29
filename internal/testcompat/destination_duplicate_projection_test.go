package testcompat_test

import (
	"reflect"
	"testing"

	"github.com/lin-string/go-playa/internal/testcompat"
)

func TestDestinationProjectionRetainsDuplicateNames(t *testing.T) {
	doc, err := openPagePDF(testPDF(
		"<< /Type /Catalog /Pages 2 0 R /Names << /Dests << /Names [(chapter) [3 0 R /FitH 100] (chapter) [3 0 R /FitH 200]] >> >> >>",
		"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 300 300] >>",
	))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = doc.Close() }()
	report, err := testcompat.SnapshotMetadata(doc, []string{"destinations"})
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Destinations) != 2 {
		t.Fatalf("destination count = %d, want 2", len(report.Destinations))
	}
	for i, entry := range report.Destinations {
		if entry.Name != "chapter" || entry.Destination.PageIndex != 0 || entry.Destination.View != "FitH" || !reflect.DeepEqual(entry.Destination.Params, []any{float64(100)}) {
			t.Fatalf("destination %d = %#v, want first chapter definition", i, entry)
		}
	}
}
