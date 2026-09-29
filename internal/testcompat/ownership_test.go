package testcompat

import "testing"

func TestCloneCompatBytesPreservesProjectionSemantics(t *testing.T) {
	if cloneCompatBytes(nil) != nil {
		t.Fatal("nil input became non-nil")
	}
	if cloneCompatBytes(make([]byte, 0)) != nil {
		t.Fatal("non-nil empty input must retain the projection's historical nil normalization")
	}
	source := []byte("owned")
	clone := cloneCompatBytes(source)
	clone[0] = 'O'
	if string(source) != "owned" {
		t.Fatal("clone aliases its input")
	}
}
