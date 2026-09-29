package imagedata_test

import (
	"testing"

	"github.com/lin-string/go-playa/imagedata"
)

func TestPublicDecodedImageSnapshots(t *testing.T) {
	_ = (imagedata.ColorSpace{}).SpecCopy
	_ = (imagedata.ColorSpace{}).WithSpec
	image := imagedata.NewDecodedImage(2, 3, 4, 8, []byte{1, 2}, "DeviceRGB")
	if image.Width() != 2 || image.Height() != 3 || image.Components() != 4 || image.Bits() != 8 || image.ColorSpace() != "DeviceRGB" {
		t.Fatalf("decoded image metadata = %#v", image)
	}
	pix := image.PixCopy()
	pix[0] = 9
	if got := image.PixCopy(); len(got) != 2 || got[0] != 1 {
		t.Fatalf("decoded image pixels = %#v", got)
	}
	if finalized := image.Finalize(); finalized.ColorSpace() != "DeviceRGB" || finalized.PixCopy()[1] != 2 {
		t.Fatalf("decoded image finalization = %#v", finalized)
	}
}
