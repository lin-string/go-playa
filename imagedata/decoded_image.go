package imagedata

import (
	"encoding/json"

	"github.com/lin-string/go-playa/pdftypes/primitives"
)

// DecodedImage is an owned or borrowed decoded image sample value. Pixel
// bytes from a borrowed value are stabilized with Finalize or PixCopy.
type DecodedImage struct {
	width, height int
	components    int
	bits          int
	pix           []byte
	colorSpace    string
}

// NewDecodedImage creates an owned decoded-image snapshot.
func NewDecodedImage(width, height, components, bits int, pix []byte, colorSpace string) DecodedImage {
	return BorrowedDecodedImage(width, height, components, bits, pix, colorSpace).Finalize()
}

// BorrowedDecodedImage creates a decoded-image value over pix without copying
// it. The caller must call Finalize before retaining the value beyond the
// lifetime of the supplied pixel buffer.
func BorrowedDecodedImage(width, height, components, bits int, pix []byte, colorSpace string) DecodedImage {
	return DecodedImage{width: width, height: height, components: components, bits: bits, pix: pix, colorSpace: colorSpace}
}

// Width reports the decoded image width in samples.
func (image DecodedImage) Width() int { return image.width }

// Height reports the decoded image height in samples.
func (image DecodedImage) Height() int { return image.height }

// Components reports the number of decoded color components per sample.
func (image DecodedImage) Components() int { return image.components }

// Bits reports the bits per decoded component.
func (image DecodedImage) Bits() int { return image.bits }

// ColorSpace reports the decoded image color space name.
func (image DecodedImage) ColorSpace() string { return image.colorSpace }

// PixCopy returns an independent copy of decoded pixel bytes.
func (image DecodedImage) PixCopy() []byte { return primitives.CloneBytes(image.pix) }

// Finalize returns an independent snapshot of the decoded image.
func (image DecodedImage) Finalize() DecodedImage {
	image.pix = primitives.CloneBytes(image.pix)
	return image
}

func (image DecodedImage) MarshalJSON() ([]byte, error) {
	type projection struct {
		Width      int    `json:"Width"`
		Height     int    `json:"Height"`
		Components int    `json:"Components"`
		Bits       int    `json:"Bits"`
		Pix        []byte `json:"Pix"`
		ColorSpace string `json:"ColorSpace"`
	}
	return json.Marshal(projection{
		Width: image.width, Height: image.height, Components: image.components,
		Bits: image.bits, Pix: image.pix, ColorSpace: image.colorSpace,
	})
}
