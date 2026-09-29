package image_test

import (
	"errors"
	"testing"

	"github.com/lin-string/go-playa/image"
	"github.com/lin-string/go-playa/pdftypes"
)

func TestPublicImageSurface(t *testing.T) {
	if image.ErrPageNotFound == nil {
		t.Fatal("ErrPageNotFound is nil")
	}
	if image.ErrNilDocument == nil {
		t.Fatal("ErrNilDocument is nil")
	}
	var _ *image.ParseError
	var _ image.Page
	var _ image.StructElement
	var _ image.ColorSpace
	var _ image.Object
	var _ image.Ref
	var _ image.Dict
	var _ image.Array
	var _ image.Stream
	var _ image.Null
	var _ image.Bool
	var _ image.Number
	var _ image.Name
	var _ image.String
	var _ image.Keyword
	var object image.ImageObject
	_ = (image.DecodedImage{}).Width
	_ = (image.DecodedImage{}).Height
	_ = (image.DecodedImage{}).Components
	_ = (image.DecodedImage{}).Bits
	_ = (image.DecodedImage{}).ColorSpace
	_ = (image.DecodedImage{}).PixCopy
	_ = (image.DecodedImage{}).Finalize
	_ = (image.ImageColorSpace{}).BaseCopy
	_ = (image.ImageColorSpace{}).ColorantsCopy
	_ = (image.ImageColorSpace{}).LookupCopy
	_ = (image.ImageColorSpace{}).LookupWithError
	_ = (image.ImageColorSpace{}).SpecCopy
	_ = (image.ImageColorSpace{}).Finalize
	_ = (image.ImageColorSpace{}).FinalizeWithError
	_ = object.Buffer
	_ = object.DecodedStreamBuffer
	_ = object.DecodedStreamBufferWithError
	_ = object.Get
	_ = object.Has
	_ = object.Samples
	_ = object.SamplesWithError
	_ = object.Finalize
	_ = object.FinalizeWithError
	_ = object.IndexedLookupWithError
	_ = object.ColorSpaceInfoCopyWithError
	_ = object.DecodeSample
	_ = object.ParentKey
	_ = object.HasParentKey
	_ = object.Parent
	_, _ = image.DecodeFilters(nil, nil, nil)
	if got := image.UnpackData([]byte{0x1b}, 2, 4, 1, 1); len(got) != 4 || got[0] != 0 || got[1] != 1 || got[2] != 2 || got[3] != 3 {
		t.Fatalf("UnpackData = %v, want [0 1 2 3]", got)
	}
	_, _ = object.Get(pdftypes.Name("Width"))
}

func TestImageDecodeFiltersExposesParseError(t *testing.T) {
	_, err := image.DecodeFilters(nil, []string{"UnknownDecode"}, nil)
	var parseErr *image.ParseError
	if !errors.As(err, &parseErr) {
		t.Fatalf("decode error = %T, want ParseError", err)
	}
	if parseErr.Operation() != "filter UnknownDecode" {
		t.Fatalf("parse error operation = %q", parseErr.Operation())
	}
}
