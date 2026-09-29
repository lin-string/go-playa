package pdftypes

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"iter"
	"sort"

	"github.com/lin-string/go-playa/parser"
	"github.com/lin-string/go-playa/pdftypes/primitives"
)

const decodedFilterExpansionLimit = 256 << 20

// StreamResolver resolves indirect PDF objects used by stream metadata. The
// document package implements this interface without making pdftypes depend
// on document-level lookup and encryption state.
type StreamResolver interface {
	ResolveObject(Object) (Object, error)
}

// Stream is the dependency-free PDF stream value. Its dictionary and encoded
// bytes are private; callers receive copies through the ordinary accessors or
// use the explicitly borrowed views while traversing a document.
type Stream struct {
	dict       Dict
	decodeDict Dict
	data       []byte
	ref        Ref
	hasRef     bool
}

// NewStream constructs an owned stream by copying its dictionary and bytes.
func NewStream(dict Dict, data []byte) Stream {
	return NewStreamOwned(cloneDict(dict), primitives.CloneBytes(data))
}

// NewStreamOwned constructs a stream from storage whose ownership is
// transferred to the returned value. It is intended for parser adapters.
func NewStreamOwned(dict Dict, data []byte, decodeDict ...Dict) Stream {
	stream := Stream{dict: dict, data: data}
	if len(decodeDict) > 0 {
		stream.decodeDict = decodeDict[0]
	}
	return stream
}

// ClonePDFObject lets document-independent object snapshots retain streams
// without importing the document implementation.
func (s Stream) ClonePDFObject() Object {
	return NewStreamOwned(cloneDict(s.dict), primitives.CloneBytes(s.data), cloneDict(s.decodeDict)).WithRefValue(s.ref, s.hasRef)
}

// WithRef returns a stream carrying an indirect object identity.
func (s Stream) WithRef(ref Ref) Stream { return s.WithRefValue(ref, true) }

func (s Stream) WithRefValue(ref Ref, present bool) Stream {
	s.ref, s.hasRef = ref, present
	return s
}

// DictBorrowed returns the parser-owned dictionary view. It is intended for
// document internals; callers that retain the value should use DictCopy.
func (s Stream) DictBorrowed() Dict { return s.dict }

// DataBorrowed returns the parser-owned encoded bytes. It is intended for
// document internals during one traversal; callers that retain the value
// should use RawData or Buffer.
func (s Stream) DataBorrowed() []byte { return s.data }

// Buffer returns an independent copy of the encoded stream bytes.
func (s Stream) Buffer() []byte { return primitives.CloneBytes(s.data) }

// RawData is the Playa ContentStream.rawdata equivalent.
func (s Stream) RawData() []byte { return s.Buffer() }

// BufferDigest returns the raw stream length and SHA-256 without copying the
// payload.
func (s Stream) BufferDigest() (int, string) {
	digest := sha256.Sum256(s.data)
	return len(s.data), hex.EncodeToString(digest[:])
}

// DecodedBuffer returns an independent, leniently decoded stream copy.
func (s Stream) DecodedBuffer() []byte {
	filters, parms := parser.StreamFilters(s.decodingDictionary())
	data, _ := parser.DecodeFiltersLenientLimited(s.data, filters, parms, decodedFilterExpansionLimit)
	return primitives.CloneBytes(data)
}

// Decode is the lenient Playa ContentStream.decode equivalent.
func (s Stream) Decode() []byte { return s.DecodedBuffer() }

// DecodedBufferWithError decodes direct stream filter metadata strictly.
func (s Stream) DecodedBufferWithError() ([]byte, error) {
	return s.DecodedBufferWithResolverWithError(nil)
}

// DecodedBufferWithResolver returns a lenient decoded stream copy after
// resolving indirect filter metadata through resolve.
func (s Stream) DecodedBufferWithResolver(resolve func(Object) Object) []byte {
	if resolve == nil {
		return s.DecodedBuffer()
	}
	filters, parms := parser.StreamFiltersWithResolver(s.decodingDictionary(), resolve)
	data, _ := parser.DecodeFiltersLenientLimited(s.data, filters, parms, decodedFilterExpansionLimit)
	return primitives.CloneBytes(data)
}

// DecodedBufferWithResolverWithError is the strict resolver-aware decoder.
func (s Stream) DecodedBufferWithResolverWithError(resolve func(Object) Object) ([]byte, error) {
	var filters []string
	var parms []Dict
	if resolve == nil {
		filters, parms = parser.StreamFilters(s.decodingDictionary())
	} else {
		filters, parms = parser.StreamFiltersWithResolver(s.decodingDictionary(), resolve)
	}
	decoded, err := parser.DecodeFiltersLimited(s.data, filters, parms, decodedFilterExpansionLimit)
	if err != nil {
		return nil, err
	}
	return primitives.CloneBytes(decoded), nil
}

// DecodedBufferWithDocument returns a lenient decoded stream copy after
// resolving indirect filter metadata through a document-like resolver.
func (s Stream) DecodedBufferWithDocument(d StreamResolver) []byte {
	decoded, _ := s.DecodedBufferWithDocumentWithError(d)
	return decoded
}

// DecodedBufferWithDocumentWithError is the resolver-aware counterpart to
// DecodedBufferWithError. The historical method name is retained so the
// document-facing wrappers keep a direct mapping to Playa's stream API.
func (s Stream) DecodedBufferWithDocumentWithError(d StreamResolver) ([]byte, error) {
	decoded, err := s.decodedBufferWithDocument(d)
	if err != nil {
		return nil, err
	}
	return primitives.CloneBytes(decoded), nil
}

// DecodedBufferDigestWithResolver returns the decoded length and SHA-256.
func (s Stream) DecodedBufferDigestWithResolver(resolve func(Object) Object) (int, string, error) {
	var filters []string
	var parms []Dict
	if resolve == nil {
		filters, parms = parser.StreamFilters(s.decodingDictionary())
	} else {
		filters, parms = parser.StreamFiltersWithResolver(s.decodingDictionary(), resolve)
	}
	decoded, err := parser.DecodeFiltersLimited(s.data, filters, parms, decodedFilterExpansionLimit)
	if err != nil {
		return 0, "", err
	}
	digest := sha256.Sum256(decoded)
	return len(decoded), hex.EncodeToString(digest[:]), nil
}

// DecodedBufferDigestWithDocument returns the decoded length and digest using
// a document-like resolver for indirect filter metadata.
func (s Stream) DecodedBufferDigestWithDocument(d StreamResolver) (int, string, error) {
	decoded, err := s.decodedBufferWithDocument(d)
	if err != nil {
		return 0, "", err
	}
	digest := sha256.Sum256(decoded)
	return len(decoded), hex.EncodeToString(digest[:]), nil
}

func (s Stream) decodedBufferWithDocument(d StreamResolver) ([]byte, error) {
	var filters []string
	var parms []Dict
	if d == nil {
		filters, parms = parser.StreamFilters(s.decodingDictionary())
	} else {
		var resolveErr error
		filters, parms = parser.StreamFiltersWithResolver(s.decodingDictionary(), func(value Object) Object {
			if resolveErr != nil {
				return nil
			}
			resolved, err := d.ResolveObject(value)
			if err != nil {
				resolveErr = err
				return nil
			}
			return resolved
		})
		if resolveErr != nil {
			return nil, resolveErr
		}
	}
	return parser.DecodeFiltersLimited(s.data, filters, parms, decodedFilterExpansionLimit)
}

// Finalize returns an independent stream snapshot.
func (s Stream) Finalize() Stream { return s.ClonePDFObject().(Stream) }

func (s Stream) decodingDictionary() Dict {
	if s.decodeDict != nil {
		return s.decodeDict
	}
	return s.dict
}

// DictCopy returns an independent copy of the stream dictionary.
func (s Stream) DictCopy() Dict { return cloneDict(s.dict) }

// AttrsCopy is the read-only Go counterpart of ContentStream.attrs.
func (s Stream) AttrsCopy() Dict { return s.DictCopy() }

// Get returns an independent copy of one stream dictionary value.
func (s Stream) Get(key Name) (Object, bool) {
	value, ok := s.dict[key]
	return clonePDFObject(value), ok
}

// Keys lazily yields stream attribute names in stable lexical order.
func (s Stream) Keys() iter.Seq[Name] {
	return func(yield func(Name) bool) {
		keys := streamAttributeKeys(s.dict)
		for _, key := range keys {
			if !yield(key) {
				return
			}
		}
	}
}

// Values lazily yields independent stream attribute values in key order.
func (s Stream) Values() iter.Seq[Object] {
	return func(yield func(Object) bool) {
		for key := range s.Keys() {
			if !yield(s.value(key)) {
				return
			}
		}
	}
}

// Items lazily yields independent stream attributes in key order.
func (s Stream) Items() iter.Seq2[Name, Object] {
	return func(yield func(Name, Object) bool) {
		for key := range s.Keys() {
			if !yield(key, s.value(key)) {
				return
			}
		}
	}
}

func (s Stream) value(key Name) Object {
	value, _ := s.Get(key)
	return value
}

func streamAttributeKeys(attrs Dict) []Name {
	keys := make([]Name, 0, len(attrs))
	for key := range attrs {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })
	return keys
}

// Has reports whether the stream dictionary contains key.
func (s Stream) Has(key Name) bool {
	_, ok := s.dict[key]
	return ok
}

// GetAny returns the first present stream attribute, accepting PDF
// abbreviations for the standard stream keys.
func (s Stream) GetAny(keys ...Name) (Object, bool) {
	for _, key := range keys {
		if short, ok := streamAbbreviatedKey(key); ok {
			if value, present := s.dict[short]; present {
				return clonePDFObject(value), true
			}
		}
		if value, present := s.dict[key]; present {
			return clonePDFObject(value), true
		}
	}
	return nil, false
}

// GetAnyDefault returns the first present stream attribute or a copied
// fallback.
func (s Stream) GetAnyDefault(fallback Object, keys ...Name) Object {
	if value, ok := s.GetAny(keys...); ok {
		return value
	}
	return clonePDFObject(fallback)
}

// GetFilters returns normalized filter names and copied DecodeParms values.
func (s Stream) GetFilters() ([]string, []Dict) {
	filters, parms := parser.StreamFilters(s.dict)
	if parms == nil {
		return filters, nil
	}
	out := make([]Dict, len(parms))
	for i, parm := range parms {
		out[i] = cloneDict(parm)
	}
	return filters, out
}

func (s Stream) FiltersCopy() []string {
	filters, _ := s.GetFilters()
	return filters
}

func (s Stream) FilterParamsCopy() []Dict {
	_, parms := s.GetFilters()
	return parms
}

func (s Stream) Width() int             { return streamPositiveInt(s.valueFor(Name("Width")), 1) }
func (s Stream) Height() int            { return streamPositiveInt(s.valueFor(Name("Height")), 1) }
func (s Stream) Bits() int              { return streamPositiveInt(s.valueFor(Name("BitsPerComponent")), 1) }
func (s Stream) ColorSpaceSpec() Object { return s.valueFor(Name("ColorSpace")) }
func (s Stream) ColorSpace() Object     { return s.ColorSpaceSpec() }

func (s Stream) valueFor(key Name) Object {
	value, _ := s.GetAny(key)
	return value
}

func (s Stream) Ref() (Ref, bool) { return s.ref, s.hasRef }
func (s Stream) ObjectID() (int, bool) {
	ref, ok := s.Ref()
	return ref.Object, ok
}
func (s Stream) Generation() (int, bool) {
	ref, ok := s.Ref()
	return ref.Generation, ok
}

func streamAbbreviatedKey(key Name) (Name, bool) {
	switch key {
	case Name("Width"):
		return Name("W"), true
	case Name("Height"):
		return Name("H"), true
	case Name("BitsPerComponent"):
		return Name("BPC"), true
	case Name("ColorSpace"):
		return Name("CS"), true
	case Name("Filter"):
		return Name("F"), true
	case Name("DecodeParms"):
		return Name("DP"), true
	default:
		return "", false
	}
}

func streamPositiveInt(value Object, fallback int) int {
	parsed, ok := IntValue(value)
	if !ok || parsed <= 0 {
		return fallback
	}
	return parsed
}

func (s Stream) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Dict Dict
		Data []byte
	}{Dict: s.dict, Data: s.data})
}

func (Stream) PDFObject() {}

func cloneDict(value Dict) Dict {
	if value == nil {
		return nil
	}
	out := make(Dict, len(value))
	for key, item := range value {
		out[key] = clonePDFObject(item)
	}
	return out
}

func clonePDFObject(value Object) Object {
	switch value := value.(type) {
	case nil, Null, Bool, Number, Name, Ref, Keyword:
		return value
	case String:
		return String(primitives.CloneBytes(value))
	case Array:
		if value == nil {
			return Array(nil)
		}
		out := make(Array, len(value))
		for i, item := range value {
			out[i] = clonePDFObject(item)
		}
		return out
	case InvalidArray:
		if value == nil {
			return InvalidArray(nil)
		}
		out := make(InvalidArray, len(value))
		for i, item := range value {
			out[i] = clonePDFObject(item)
		}
		return out
	case Dict:
		return cloneDict(value)
	case Stream:
		return value.ClonePDFObject()
	default:
		return value
	}
}
