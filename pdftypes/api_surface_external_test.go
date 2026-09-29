package pdftypes_test

import (
	"errors"
	"testing"

	"github.com/lin-string/go-playa/pdftypes"
)

type failingStreamResolver struct{ cause error }

func (resolver failingStreamResolver) ResolveObject(pdftypes.Object) (pdftypes.Object, error) {
	return nil, resolver.cause
}

func TestStrictStreamDocumentDecodersPreserveResolverCause(t *testing.T) {
	cause := errors.New("unavailable metadata object")
	for _, dict := range []pdftypes.Dict{
		{pdftypes.Name("Filter"): pdftypes.Ref{Object: 7}},
		{pdftypes.Name("Filter"): pdftypes.Array{pdftypes.Ref{Object: 7}}},
		{pdftypes.Name("Filter"): pdftypes.Name("ASCIIHexDecode"), pdftypes.Name("DecodeParms"): pdftypes.Ref{Object: 8}},
		{pdftypes.Name("Filter"): pdftypes.Name("ASCIIHexDecode"), pdftypes.Name("DecodeParms"): pdftypes.Array{pdftypes.Ref{Object: 8}}},
	} {
		stream := pdftypes.NewStream(dict, []byte("4142>"))
		if data, err := stream.DecodedBufferWithDocumentWithError(failingStreamResolver{cause}); !errors.Is(err, cause) || data != nil {
			t.Errorf("strict buffer = %q, %v; want nil and resolver cause", data, err)
		}
		if length, digest, err := stream.DecodedBufferDigestWithDocument(failingStreamResolver{cause}); !errors.Is(err, cause) || length != 0 || digest != "" {
			t.Errorf("strict digest = %d, %q, %v; want zero outputs and resolver cause", length, digest, err)
		}
	}
}

func TestStreamCopiesPreserveNilAndEmptyByteStorage(t *testing.T) {
	for _, data := range [][]byte{nil, {}} {
		stream := pdftypes.NewStream(nil, data)
		for _, snapshot := range []pdftypes.Stream{stream, stream.Finalize()} {
			if got := snapshot.RawData(); (got == nil) != (data == nil) {
				t.Fatalf("RawData nil = %v, want %v", got == nil, data == nil)
			}
			decoded, err := snapshot.DecodedBufferWithDocumentWithError(nil)
			if err != nil || (decoded == nil) != (data == nil) {
				t.Fatalf("decoded = %#v, %v; want matching nil storage", decoded, err)
			}
		}
	}
}

func TestPublicPrimitiveObjectSurface(t *testing.T) {
	var _ pdftypes.Object = pdftypes.InvalidArray{}
	_ = pdftypes.AsObject
	_ = (pdftypes.Stream{}).AttrsCopy
	_ = (pdftypes.Stream{}).RawData
	_ = (pdftypes.Stream{}).DecodedBuffer
	_ = (pdftypes.Stream{}).Decode
	_ = (pdftypes.Stream{}).FiltersCopy
	_ = (pdftypes.Stream{}).FilterParamsCopy
	_ = (pdftypes.Stream{}).GetFilters
	_ = (pdftypes.Stream{}).GetAny
	_ = (pdftypes.Stream{}).GetAnyDefault
	_ = (pdftypes.Stream{}).Width
	_ = (pdftypes.Stream{}).Height
	_ = (pdftypes.Stream{}).Bits
	_ = (pdftypes.Stream{}).ColorSpaceSpec
	_ = (pdftypes.Stream{}).ColorSpace
	_ = (pdftypes.Stream{}).Ref
	_ = (pdftypes.Stream{}).ObjectID
	_ = (pdftypes.Stream{}).Generation
	_ = (pdftypes.Stream{}).Keys
	_ = (pdftypes.Stream{}).Values
	_ = (pdftypes.Stream{}).Items
	if got, ok := pdftypes.NameValue(pdftypes.Name("Width")); !ok || got != "Width" {
		t.Fatalf("NameValue() = %q, %v", got, ok)
	}
}

func TestStreamOwnsPrimitiveStorageAndResolvesFiltersWithoutDocumentImport(t *testing.T) {
	dict := pdftypes.Dict{pdftypes.Name("Filter"): pdftypes.Name("ASCIIHexDecode")}
	data := []byte("4142>")
	stream := pdftypes.NewStream(dict, data)
	dict[pdftypes.Name("Filter")] = pdftypes.Name("Unknown")
	data[0] = 'x'

	if got := string(stream.RawData()); got != "4142>" {
		t.Fatalf("RawData() = %q, want original bytes", got)
	}
	decoded, err := stream.DecodedBufferWithError()
	if err != nil || string(decoded) != "AB" {
		t.Fatalf("DecodedBufferWithError() = %q, %v, want AB", decoded, err)
	}

	resolved := pdftypes.NewStream(
		pdftypes.Dict{pdftypes.Name("Filter"): pdftypes.Ref{Object: 7}},
		[]byte("4142>"),
	)
	decoded, err = resolved.DecodedBufferWithResolverWithError(func(value pdftypes.Object) pdftypes.Object {
		if value == (pdftypes.Ref{Object: 7}) {
			return pdftypes.Name("ASCIIHexDecode")
		}
		return value
	})
	if err != nil || string(decoded) != "AB" {
		t.Fatalf("resolver-backed decode = %q, %v, want AB", decoded, err)
	}
}
