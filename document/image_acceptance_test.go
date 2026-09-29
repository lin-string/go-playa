package document

import (
	"bytes"
	"os"
	"testing"

	"github.com/lin-string/go-playa/internal/testfixture"
)

func TestInlineImageAcceptanceFixtureMatchesPageImageContract(t *testing.T) {
	data, err := os.ReadFile(testfixture.Path(t, "acceptance_inline_image.pdf"))
	if err != nil {
		t.Fatal(err)
	}
	d, err := OpenBytes(data)
	if err != nil {
		t.Fatal(err)
	}
	page, err := d.PageAt(0)
	if err != nil {
		t.Fatal(err)
	}
	images, err := d.PageImages(page)
	if err != nil {
		t.Fatal(err)
	}
	want := []struct {
		bbox [4]float64
		data []byte
	}{
		{bbox: [4]float64{72, 118, 96, 142}, data: []byte{0xff}},
		{bbox: [4]float64{120, 118, 144, 142}, data: []byte{0, 0, 0, 0}},
	}
	if len(images) != len(want) {
		t.Fatalf("inline images = %d, want %d", len(images), len(want))
	}
	for i, image := range images {
		if !image.Inline() || image.Width() != 1 || image.Height() != 1 || image.BPC() != 8 || image.Components() != 1 || image.ColorSpace() != "DeviceGray" {
			t.Fatalf("inline image %d metadata = inline:%v size:%dx%d bpc:%d components:%d color:%q", i, image.Inline(), image.Width(), image.Height(), image.BPC(), image.Components(), image.ColorSpace())
		}
		if image.BBox() != want[i].bbox {
			t.Fatalf("inline image %d bbox = %v, want %v", i, image.BBox(), want[i].bbox)
		}
		decoded, err := image.DecodedStreamBufferWithError()
		if err != nil {
			t.Fatalf("inline image %d decoded stream = %v; raw=%x filters=%v", i, err, image.Buffer(), image.FiltersCopy())
		}
		if !bytes.Equal(decoded, want[i].data) {
			t.Fatalf("inline image %d decoded stream = %x, want %x", i, decoded, want[i].data)
		}
	}
}

func TestRGBImageAcceptanceFixture(t *testing.T) {
	data, err := os.ReadFile(testfixture.Path(t, "acceptance_rgb_image.pdf"))
	if err != nil {
		t.Fatal(err)
	}
	d, err := OpenBytes(data)
	if err != nil {
		t.Fatal(err)
	}
	page, err := d.PageAt(0)
	if err != nil {
		t.Fatal(err)
	}
	var images []ImageObject
	for image, imageErr := range page.Images(d) {
		if imageErr != nil {
			t.Fatal(imageErr)
		}
		images = append(images, image)
	}
	if len(images) != 1 || images[0].name != "Im1" || images[0].width != 2 || images[0].height != 1 || images[0].colorSpace != "DeviceRGB" {
		t.Fatalf("RGB image = %#v", images)
	}
	samples, components, err := images[0].SamplesWithError()
	if err != nil || components != 3 || string(samples) != string([]byte{255, 0, 0, 0, 255, 0}) {
		t.Fatalf("RGB samples = %v components=%d err=%v", samples, components, err)
	}
}
