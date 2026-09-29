package fontdata

import (
	"encoding/json"
	"testing"

	"github.com/lin-string/go-playa/geometry"
)

func TestMetadataIsAnIndependentValueModel(t *testing.T) {
	got := NewMetadata("F1", 4, 800, -200, 0, 0, 0, geometry.Matrix{1, 0, 0, 1, 0, 0}, true, true, 0, 0, false, [4]float64{-10, -20, 1000, 900})
	if got.Name() != "F1" || !got.HasFlags() || !got.HasBBox() || got.BBox()[2] != 1000 {
		t.Fatalf("metadata = %#v", got)
	}
}

func TestMetadataJSONKeepsPublicProjection(t *testing.T) {
	metadata := NewMetadata("F1", 4, 800, -200, 24, 700, 80, geometry.Matrix{1, 0, 0, 1, 0, 0}, true, true, -12, 1000, false, [4]float64{-10, -20, 1000, 900})
	data, err := json.Marshal(metadata)
	if err != nil {
		t.Fatal(err)
	}
	var projection map[string]any
	if err := json.Unmarshal(data, &projection); err != nil {
		t.Fatal(err)
	}
	if projection["Name"] != "F1" || projection["Flags"] != float64(4) || projection["HasBBox"] != true {
		t.Fatalf("metadata JSON projection = %s", data)
	}
}
