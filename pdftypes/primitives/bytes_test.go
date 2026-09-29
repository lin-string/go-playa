package primitives

import "testing"

func TestCloneBytesPreservesOwnershipAndEmptyValues(t *testing.T) {
	value := []byte("source")
	copyValue := CloneBytes(value)
	copyValue[0] = 'S'
	if string(value) != "source" || string(copyValue) != "Source" {
		t.Fatalf("CloneBytes shared source storage: source=%q copy=%q", value, copyValue)
	}
	if empty := CloneBytes([]byte{}); empty == nil {
		t.Fatal("CloneBytes converted an empty non-nil slice to nil")
	}
}
