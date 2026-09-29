package imagedata_test

import (
	"testing"

	"github.com/lin-string/go-playa/imagedata"
)

func TestUnpackPreservesPackedRows(t *testing.T) {
	got := imagedata.Unpack([]byte{0x1b, 0xe4}, 2, 4, 2, 1)
	want := []byte{0, 1, 2, 3, 3, 2, 1, 0}
	if string(got) != string(want) {
		t.Fatalf("Unpack() = %v, want %v", got, want)
	}
}

func TestUnpackRejectsOverflowingGeometry(t *testing.T) {
	if got := imagedata.Unpack([]byte{0xff}, 4, int(^uint(0)>>1), 2, 2); got != nil {
		t.Fatalf("Unpack accepted overflowing geometry: %v", got)
	}
}
