package textconfig_test

import (
	"testing"

	"github.com/lin-string/go-playa/textconfig"
)

func TestPublicSnapshotOwnsBoundingBox(t *testing.T) {
	bbox := [4]float64{1, 2, 3, 4}
	copy := textconfig.Snapshot(textconfig.Options{BBox: &bbox})
	bbox[0] = 99
	if copy.BBox == nil || copy.BBox[0] != 1 {
		t.Fatalf("Snapshot shared BBox storage: %#v", copy.BBox)
	}
}
