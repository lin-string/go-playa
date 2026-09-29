package testcompat

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"testing"

	core "github.com/lin-string/go-playa/document"
)

func TestProjectImagesUsesEmptyArraysForMissingFilters(t *testing.T) {
	images := projectImages([]core.ImageObject{{}}, -1)
	if len(images) != 1 {
		t.Fatalf("images = %#v", images)
	}
	encoded, err := json.Marshal(images[0])
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(encoded, []byte(`"filters":[]`)) {
		t.Fatalf("image projection = %s", encoded)
	}
}

func TestProjectImageIncludesFilteredStreamDigest(t *testing.T) {
	image := core.ImageObject{}
	// This test intentionally uses the public projection path. The image is
	// zero-valued here; the stream fields must still be present and stable.
	projected := ImageSnapshot(image, 0)
	digest := sha256.Sum256(nil)
	if projected.StreamLength != 0 || projected.StreamSHA256 != hex.EncodeToString(digest[:]) {
		t.Fatalf("image stream projection = length:%d sha256:%q", projected.StreamLength, projected.StreamSHA256)
	}
	encoded, err := json.Marshal(projected)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(encoded, []byte(`"stream_length":0`)) || !bytes.Contains(encoded, []byte(`"stream_sha256":"`)) {
		t.Fatalf("image projection omitted stream digest: %s", encoded)
	}
}
