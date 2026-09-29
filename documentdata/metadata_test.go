package documentdata_test

import (
	"testing"

	"github.com/lin-string/go-playa/documentdata"
)

func TestPublicMetadataOwnsClonesAndUsesCompactNumbers(t *testing.T) {
	source := documentdata.Metadata{"Title": "original"}
	clone := documentdata.CloneMetadata(source)
	clone["Title"] = "changed"
	if source["Title"] != "original" {
		t.Fatalf("metadata clone shared source map: %#v", source)
	}
	if got := documentdata.FormatNumber(12.5); got != "12.5" {
		t.Fatalf("FormatNumber(12.5) = %q", got)
	}
}
