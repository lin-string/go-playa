package documentdata_test

import (
	"testing"

	"github.com/lin-string/go-playa/documentdata"
	"github.com/lin-string/go-playa/pdftypes/primitives"
)

func TestActionOwnsRawValueSnapshot(t *testing.T) {
	raw := primitives.Dict{
		primitives.Name("Meta"): primitives.Dict{
			primitives.Name("Value"): primitives.String("original"),
		},
	}
	action := documentdata.NewAction("URI", "https://example.test", "", "", "", raw)
	raw[primitives.Name("Meta")].(primitives.Dict)[primitives.Name("Value")] = primitives.String("source changed")

	if action.Kind() != "URI" || action.URI() != "https://example.test" {
		t.Fatalf("action metadata = %q/%q", action.Kind(), action.URI())
	}
	copy := action.RawCopy()
	copy[primitives.Name("Meta")].(primitives.Dict)[primitives.Name("Value")] = primitives.String("copy changed")
	got := action.RawCopy()[primitives.Name("Meta")].(primitives.Dict)[primitives.Name("Value")].(primitives.String)
	if string(got) != "original" {
		t.Fatalf("action raw value was not isolated: %q", got)
	}
	if snapshot := action.Finalize(); snapshot.Kind() != action.Kind() {
		t.Fatalf("finalized action kind = %q", snapshot.Kind())
	}
}
