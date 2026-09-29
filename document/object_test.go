package document

import (
	"bytes"
	"compress/zlib"
	"encoding/json"
	"errors"
	"math"
	"reflect"
	"runtime"
	"testing"

	"github.com/lin-string/go-playa/cacheconfig"
	"github.com/lin-string/go-playa/documentdata"
	"github.com/lin-string/go-playa/internal/testfixture"
)

func TestParseObjectReturnsTypedParseError(t *testing.T) {
	_, err := ParseObject([]byte("<< /Value (unterminated"))
	if err == nil {
		t.Fatal("expected parse error")
	}
	var parseErr *ParseError
	if !errors.As(err, &parseErr) {
		t.Fatalf("error type = %T, want ParseError", err)
	}
	if parseErr.Offset() != 10 || parseErr.Operation() != "lex" {
		t.Fatalf("parse error = %#v, want offset 10 and lex operation", parseErr)
	}
}

func TestIntValueRejectsNonFiniteAndOutOfRangeNumbers(t *testing.T) {
	for _, value := range []Number{Number(1.5), Number(1e100), Number(math.NaN()), Number(math.Inf(1))} {
		if _, ok := IntValue(value); ok {
			t.Fatalf("IntValue accepted %v", value)
		}
	}
	if _, ok := IntValue(Number(0)); !ok {
		t.Fatal("IntValue rejected zero")
	}
}

func TestScanObjectsSkipsOverflowingObjectNumbers(t *testing.T) {
	d := &Document{data: []byte("%PDF-1.7\n999999999999999999999 0 obj\n42\nendobj\n1 0 obj\n43\nendobj")}
	if err := d.scanObjects(); err != nil {
		t.Fatal(err)
	}
	if _, ok := d.objects[Ref{}]; ok {
		t.Fatal("overflowing scanned object was converted to object 0")
	}
	if got, ok := d.objects[Ref{Object: 1}]; !ok || got != Number(43) {
		t.Fatalf("valid scanned object = %#v, %v", got, ok)
	}
}

func TestParseScannedObjectHeaderValidatesNumericFields(t *testing.T) {
	id, generation, ok := parseScannedObjectHeader([]byte("12 3 obj \n"))
	if !ok || id != 12 || generation != 3 {
		t.Fatalf("scanned object header = %d %d %v, want 12 3 true", id, generation, ok)
	}
	if _, _, ok := parseScannedObjectHeader([]byte("999999999999999999999 0 obj\n")); ok {
		t.Fatal("scanned object header accepted an overflowing object number")
	}
	if _, _, ok := parseScannedObjectHeader([]byte("1 999999999999999999999 obj\n")); ok {
		t.Fatal("scanned object header accepted an overflowing generation")
	}
}

func TestScanObjectsIgnoresObjectHeadersInsideComments(t *testing.T) {
	d := &Document{data: []byte("%PDF-1.7\n% 2 0 obj\n99\nendobj\n1 0 obj\n43\nendobj")}
	if err := d.scanObjects(); err != nil {
		t.Fatal(err)
	}
	if _, ok := d.objects[Ref{Object: 2}]; ok {
		t.Fatal("object header inside a comment was recovered")
	}
	if got, ok := d.objects[Ref{Object: 1}]; !ok || got != Number(43) {
		t.Fatalf("valid scanned object = %#v, %v", got, ok)
	}
}

func TestScanObjectsIgnoresObjectHeadersInsideStrings(t *testing.T) {
	d := &Document{data: []byte("%PDF-1.7\n1 0 obj\n(2 0 obj\n99\nendobj)\nendobj\n3 0 obj\n43\nendobj")}
	if err := d.scanObjects(); err != nil {
		t.Fatal(err)
	}
	if _, ok := d.objects[Ref{Object: 2}]; ok {
		t.Fatal("object header inside a string was recovered")
	}
	if got, ok := d.objects[Ref{Object: 3}]; !ok || got != Number(43) {
		t.Fatalf("valid scanned object = %#v, %v", got, ok)
	}
}

func TestScanObjectsResolvesIndirectStreamLength(t *testing.T) {
	d := &Document{data: []byte("%PDF-1.7\n" +
		"1 0 obj\n<< /Length 2 0 R >>\nstream\nAendstream\nendobj\n" +
		"2 0 obj\n1\nendobj\n")}
	if err := d.scanObjects(); err != nil {
		t.Fatal(err)
	}
	stream, ok := d.objects[Ref{Object: 1}].(Stream)
	if !ok || string(stream.DataBorrowed()) != "A" {
		t.Fatalf("recovered indirect-length stream = %#v, ok=%v", stream, ok)
	}
}

func TestScanObjectsIgnoresStandaloneStringObjectHeaders(t *testing.T) {
	d := &Document{data: []byte("%PDF-1.7\n(2 0 obj\n99\nendobj)\n3 0 obj\n43\nendobj")}
	if err := d.scanObjects(); err != nil {
		t.Fatal(err)
	}
	if _, ok := d.objects[Ref{Object: 2}]; ok {
		t.Fatal("standalone string object header was recovered")
	}
	if got, ok := d.objects[Ref{Object: 3}]; !ok || got != Number(43) {
		t.Fatalf("valid scanned object = %#v, %v", got, ok)
	}
}

func TestScanObjectsIgnoresTrailerInsideStream(t *testing.T) {
	d := &Document{data: []byte("%PDF-1.7\n1 0 obj\n<< /Length 31 >>\nstream\ntrailer << /Root 9 0 R >>\nendstream\nendobj")}
	if err := d.scanObjects(); err != nil {
		t.Fatal(err)
	}
	if d.trailer != nil {
		t.Fatalf("trailer inside stream was recovered: %#v", d.trailer)
	}
}

func TestScanObjectsIgnoresEmbeddedTrailerWordAfterObjects(t *testing.T) {
	d := &Document{data: []byte("%PDF-1.7\n1 0 obj\n42\nendobj\nnotrailer << /Root 9 0 R >>")}
	if err := d.scanObjects(); err != nil {
		t.Fatal(err)
	}
	if d.trailer != nil {
		t.Fatalf("embedded trailer word was recovered: %#v", d.trailer)
	}
}

func TestScanObjectsRequiresTrailerDictionary(t *testing.T) {
	d := &Document{data: []byte("%PDF-1.7\n1 0 obj\n42\nendobj\ntrailer 42")}
	if err := d.scanObjects(); err != nil {
		t.Fatal(err)
	}
	if d.trailer != nil {
		t.Fatalf("non-dictionary trailer was recovered: %#v", d.trailer)
	}
}

func TestScanObjectsRejectsTrailingTrailerDictionaryData(t *testing.T) {
	d := &Document{data: []byte("%PDF-1.7\n1 0 obj\n42\nendobj\ntrailer << /Size 1 >> garbage")}
	if err := d.scanObjects(); err != nil {
		t.Fatal(err)
	}
	if d.trailer != nil {
		t.Fatalf("trailer with trailing data was recovered: %#v", d.trailer)
	}
}

func TestScanObjectsIgnoresTrailerInsideStringAndComment(t *testing.T) {
	for _, data := range []string{
		"%PDF-1.7\n1 0 obj\n42\nendobj\n(trailer << /Root 9 0 R >>)",
		"%PDF-1.7\n1 0 obj\n42\nendobj\n% trailer << /Root 9 0 R >>",
		"%PDF-1.7\n1 0 obj\n42\nendobj\n<trailer << /Root 9 0 R >>>",
	} {
		t.Run(data, func(t *testing.T) {
			d := &Document{data: []byte(data)}
			if err := d.scanObjects(); err != nil {
				t.Fatal(err)
			}
			if d.trailer != nil {
				t.Fatalf("hidden trailer was recovered: %#v", d.trailer)
			}
		})
	}
}

func TestScanObjectsHandlesEscapedNestedStringsAndDictionaries(t *testing.T) {
	d := &Document{data: []byte("%PDF-1.7\n1 0 obj\n42\nendobj\n(trai\\)ler (nested (trailer)))\ntrailer << /Root << /Name (trailer) >> >>")}
	if err := d.scanObjects(); err != nil {
		t.Fatal(err)
	}
	if d.trailer == nil {
		t.Fatal("valid trailer after escaped and nested strings was not recovered")
	}
}

func TestScanObjectsDoesNotTruncateTrailerAtHiddenStartxref(t *testing.T) {
	d := &Document{data: []byte("%PDF-1.7\n1 0 obj\n42\nendobj\ntrailer << /Root 1 0 R /Note (contains startxref) >>\nstartxref\n0")}
	if err := d.scanObjects(); err != nil {
		t.Fatal(err)
	}
	root, ok := d.trailer[Name("Root")].(Ref)
	if d.trailer == nil || !ok || root != (Ref{Object: 1}) {
		t.Fatalf("trailer = %#v, want Root 1 0 R", d.trailer)
	}
}

func TestParseObjectRejectsUnterminatedContainers(t *testing.T) {
	for _, input := range []string{"[1 2", "<< /Key 1"} {
		_, err := ParseObject([]byte(input))
		if err == nil {
			t.Fatalf("input %q was accepted", input)
		}
		var parseErr *ParseError
		if !errors.As(err, &parseErr) || parseErr.Operation() != "object" {
			t.Fatalf("input %q error = %v", input, err)
		}
	}
}

func TestParseObjectBodyRejectsTrailingData(t *testing.T) {
	if _, err := parseObjectBody([]byte("<<>> garbage")); err == nil {
		t.Fatal("trailing object data was accepted")
	}
	if _, err := parseObjectBody([]byte("<< /Length 1 >> garbage\nstream\nAendstream")); err == nil {
		t.Fatal("trailing stream prefix data was accepted")
	}
}

func TestParseObjectBodyDoesNotTreatStringAsStream(t *testing.T) {
	object, err := parseObjectBody([]byte("<< /Note (stream value) >>"))
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := object.(Dict); !ok {
		t.Fatalf("object = %#v, want dictionary", object)
	}
}

func TestIndirectBodyStopsBeforeStreamInFollowingObject(t *testing.T) {
	data := []byte("1 0 obj\n<< /Length 1 /Kind /Dictionary >>\nendobj\n" +
		"2 0 obj\n<< /Length 1 >>\nstream\nX\nendstream\nendobj\n")
	body, ok := indirectBody(data, 0)
	if !ok {
		t.Fatal("dictionary body was not found")
	}
	if bytes.Contains(body, []byte("2 0 obj")) {
		t.Fatalf("dictionary body consumed the following stream object: %q", body)
	}
	header := bytes.Index(body, []byte("obj"))
	object, err := parseObjectBody(bytes.TrimSpace(body[header+len("obj"):]))
	if err != nil {
		t.Fatalf("parse dictionary body: %v", err)
	}
	if _, ok := object.(Dict); !ok {
		t.Fatalf("object = %T, want dictionary", object)
	}
}

func TestObjectParser(t *testing.T) {
	p := NewObjectParser([]byte("<< /Type /Page /MediaBox [0 0 612 792] /Parent 2 0 R >>"))
	o, e := p.Parse()
	if e != nil {
		t.Fatal(e)
	}
	d, ok := o.(Dict)
	if !ok || d[Name("Type")] != Name("Page") {
		t.Fatalf("%#v", o)
	}
	if _, ok := d[Name("Parent")].(Ref); !ok {
		t.Fatalf("parent=%#v", d[Name("Parent")])
	}
}

func TestParseObjectAndResolveAll(t *testing.T) {
	o, err := ParseObject([]byte("[1 0 R << /Value 2 0 R >>]"))
	if err != nil {
		t.Fatal(err)
	}
	objects := map[Ref]Object{
		{Object: 1, Generation: 0}: Number(7),
		{Object: 2, Generation: 0}: Dict{Name("Value"): Number(9)},
	}
	resolved := ResolveAll(o, func(ref Ref) (Object, bool) { value, ok := objects[ref]; return value, ok })
	a := resolved.(Array)
	if a[0] != Number(7) || a[1].(Dict)[Name("Value")].(Dict)[Name("Value")] != Number(9) {
		t.Fatalf("resolved object: %#v", resolved)
	}
}

func TestResolveAllDoesNotShareStreamData(t *testing.T) {
	stream := newStream(nil, []byte("original"))
	resolved := ResolveAll(stream, func(Ref) (Object, bool) { return nil, false }).(Stream)
	resolved.DataBorrowed()[0] = 'c'
	if string(stream.DataBorrowed()) != "original" {
		t.Fatalf("resolved stream aliases source data: %q", stream.DataBorrowed())
	}
}

func TestResolveAllDoesNotShareDirectStringData(t *testing.T) {
	original := String("original")
	resolved := ResolveAll(Array{original}, func(Ref) (Object, bool) { return nil, false }).(Array)
	resolved[0].(String)[0] = 'c'
	if string(original) != "original" {
		t.Fatalf("ResolveAll shared direct string data: %q", original)
	}
}

func TestResolveAllPreservesEmptyStringBytes(t *testing.T) {
	original := String(make([]byte, 0))
	resolved := ResolveAll(original, func(Ref) (Object, bool) { return nil, false }).(String)
	if resolved == nil {
		t.Fatal("empty PDF string bytes became nil")
	}
}

func TestStreamAccessorsDoNotShareMutableValues(t *testing.T) {
	stream := newStream(Dict{Name("Nested"): Dict{Name("Value"): String("original")}}, []byte("abc"))

	buffer := stream.Buffer()
	buffer[0] = 'x'
	if string(stream.DataBorrowed()) != "abc" {
		t.Fatalf("stream buffer aliases source data: %q", stream.DataBorrowed())
	}

	dict := stream.DictCopy()
	dict[Name("Nested")].(Dict)[Name("Value")] = String("changed")
	if got := stream.DictBorrowed()[Name("Nested")].(Dict)[Name("Value")].(String); !bytes.Equal(got, String("original")) {
		t.Fatalf("stream dictionary aliases source value: %#v", got)
	}

	value, ok := stream.Get(Name("Nested"))
	if !ok {
		t.Fatal("stream Get did not find nested value")
	}
	value.(Dict)[Name("Value")] = String("changed again")
	if got := stream.DictBorrowed()[Name("Nested")].(Dict)[Name("Value")].(String); !bytes.Equal(got, String("original")) {
		t.Fatalf("stream Get aliases source value: %#v", got)
	}
	if !stream.Has(Name("Nested")) || stream.Has(Name("Missing")) {
		t.Fatal("stream Has returned an incorrect result")
	}
}

func TestStreamDecodedBufferAppliesFiltersWithoutExposingSource(t *testing.T) {
	stream := newStream(Dict{Name("Filter"): Name("ASCIIHexDecode")}, []byte("68656c6c6f>"))
	decoded, err := stream.DecodedBufferWithError()
	if err != nil || string(decoded) != "hello" {
		t.Fatalf("decoded stream = %q, err = %v", decoded, err)
	}
	decoded[0] = 'X'
	if string(stream.DataBorrowed()) != "68656c6c6f>" {
		t.Fatalf("decoded stream exposed source bytes: %q", stream.DataBorrowed())
	}
}

func TestStreamDecodedBufferReportsMalformedFilter(t *testing.T) {
	stream := newStream(Dict{Name("Filter"): Name("UnknownDecode")}, []byte("data"))
	if _, err := stream.DecodedBufferWithError(); err == nil {
		t.Fatal("decoded stream accepted an unsupported filter")
	}
	if stream.DecodedBuffer() != nil {
		t.Fatal("compatibility decoded accessor returned data after filter failure")
	}
}

func TestStreamDecodeRecoversTruncatedFlatePrefix(t *testing.T) {
	stream := newStream(Dict{Name("Filter"): Name("FlateDecode")}, []byte{
		0x78, 0x9c, 0xed, 0xc5, 0xb1, 0x09, 0x00, 0x20, 0x0c, 0x00,
		0xb0, 0x8b, 0x7a, 0x96, 0x05, 0xb7, 0xa2, 0x14, 0x3c, 0x5f,
		0x7f, 0x70, 0x4d, 0x96, 0xd4, 0x1a, 0x39, 0x4f, 0x94, 0x24,
		0x49, 0x92, 0x24, 0x49, 0x92, 0x24, 0x49, 0x92, 0x24, 0x49,
	})
	recovered := stream.Decode()
	if len(recovered) == 0 || !bytes.HasPrefix(recovered, []byte("prefix-")) {
		t.Fatalf("recovered Flate prefix = %q, want non-empty prefix", recovered)
	}
	if _, err := stream.DecodedBufferWithError(); err == nil {
		t.Fatal("strict decoded accessor accepted truncated Flate data")
	}
}

func TestStreamDecodeRecoversCorruptLZWPrefix(t *testing.T) {
	stream := newStream(Dict{Name("Filter"): Name("LZWDecode")}, []byte{0x80, 0x10, 0x65, 0x80})
	if recovered := stream.Decode(); string(recovered) != "A" {
		t.Fatalf("recovered LZW prefix = %q, want %q", recovered, "A")
	}
	if _, err := stream.DecodedBufferWithError(); err == nil {
		t.Fatal("strict decoded accessor accepted corrupt LZW data")
	}
}

func TestStreamDecodedBufferWithDocumentResolvesIndirectFilters(t *testing.T) {
	stream := newStream(Dict{Name("Filter"): Ref{Object: 1}}, []byte("68656c6c6f>"))
	d := &Document{objects: map[Ref]Object{{Object: 1}: Name("ASCIIHexDecode")}}
	decoded, err := stream.DecodedBufferWithDocumentWithError(d)
	if err != nil || string(decoded) != "hello" {
		t.Fatalf("document-aware decoded stream = %q, err = %v", decoded, err)
	}
}

func TestStreamSnapshotsPreserveEmptyData(t *testing.T) {
	stream := newStream(nil, make([]byte, 0))
	if stream.Buffer() == nil {
		t.Fatal("empty stream buffer became nil")
	}
	if snapshot := stream.Finalize(); snapshot.DataBorrowed() == nil {
		t.Fatal("empty stream data was not preserved")
	}
}

func TestParseObjectBodyPreservesEmptyStreamData(t *testing.T) {
	object, err := parseObjectBody([]byte("<< /Length 0 >>\nstream\n\nendstream"))
	if err != nil {
		t.Fatal(err)
	}
	stream, ok := object.(Stream)
	if !ok || stream.DataBorrowed() == nil {
		t.Fatalf("empty parsed stream data became nil: %#v", object)
	}
}

func TestStreamJSONPreservesPublicProjection(t *testing.T) {
	encoded, err := json.Marshal(newStream(Dict{Name("Marker"): String("value")}, []byte("abc")))
	if err != nil {
		t.Fatal(err)
	}
	if string(encoded) != `{"Dict":{"Marker":"dmFsdWU="},"Data":"YWJj"}` {
		t.Fatalf("stream JSON = %s", encoded)
	}
}

func TestStreamContentMetadataUsesReadOnlyAbbreviatedFields(t *testing.T) {
	stream := newStream(Dict{
		Name("Width"):            Number(11),
		Name("W"):                Number(12),
		Name("Height"):           Number(21),
		Name("H"):                Number(22),
		Name("BitsPerComponent"): Number(7),
		Name("BPC"):              Number(8),
		Name("ColorSpace"):       Name("DeviceRGB"),
		Name("CS"):               Name("DeviceGray"),
		Name("Filter"):           Name("FlateDecode"),
		Name("DecodeParms"):      Dict{Name("Predictor"): Number(12)},
	}, []byte("raw"))

	if stream.Width() != 12 || stream.Height() != 22 || stream.Bits() != 8 {
		t.Fatalf("abbreviated image metadata = %d/%d/%d, want 12/22/8", stream.Width(), stream.Height(), stream.Bits())
	}
	if got, ok := stream.ColorSpaceSpec().(Name); !ok || got != Name("DeviceGray") {
		t.Fatalf("ColorSpaceSpec() = %#v, want DeviceGray", stream.ColorSpaceSpec())
	}
	if got, ok := stream.GetAny(Name("Width")); !ok || got != Number(12) {
		t.Fatalf("GetAny(Width) = %#v, %v, want abbreviated W", got, ok)
	}
	if got, ok := stream.GetAny(Name("Missing"), Name("Filter")); !ok || got != Name("FlateDecode") {
		t.Fatalf("GetAny(missing, Filter) = %#v, %v, want FlateDecode", got, ok)
	}
	if got := stream.GetAnyDefault(Name("fallback"), Name("Missing")); got != Name("fallback") {
		t.Fatalf("GetAnyDefault(missing) = %#v, want fallback", got)
	}
	filters, parms := stream.GetFilters()
	if !reflect.DeepEqual(filters, []string{"FlateDecode"}) || len(parms) != 1 || parms[0][Name("Predictor")] != Number(12) {
		t.Fatalf("GetFilters() = %#v/%#v", filters, parms)
	}

	attrs := stream.AttrsCopy()
	attrs[Name("Width")] = Number(99)
	if stream.Width() != 12 {
		t.Fatal("AttrsCopy exposed stream dictionary storage")
	}
	raw := stream.RawData()
	raw[0] = 'X'
	if string(stream.RawData()) != "raw" {
		t.Fatal("RawData exposed stream bytes")
	}
}

func TestStreamReferenceMetadataIsExplicit(t *testing.T) {
	stream := streamWithRef(newStream(nil, []byte("raw")), Ref{Object: 7, Generation: 2})
	ref, ok := stream.Ref()
	if !ok || ref != (Ref{Object: 7, Generation: 2}) {
		t.Fatalf("Ref() = %#v, %v", ref, ok)
	}
	if objectID, ok := stream.ObjectID(); !ok || objectID != 7 {
		t.Fatalf("ObjectID() = %d, %v", objectID, ok)
	}
	if generation, ok := stream.Generation(); !ok || generation != 2 {
		t.Fatalf("Generation() = %d, %v", generation, ok)
	}
	if objectID, ok := (Stream{}).ObjectID(); ok || objectID != 0 {
		t.Fatalf("zero stream ObjectID() = %d, %v", objectID, ok)
	}
	if generation, ok := (Stream{}).Generation(); ok || generation != 0 {
		t.Fatalf("zero stream Generation() = %d, %v", generation, ok)
	}
	if _, ok := (Stream{}).Ref(); ok {
		t.Fatal("zero stream unexpectedly has an object reference")
	}
}

func TestStreamMappingIteratorsAreStableAndOwned(t *testing.T) {
	stream := newStream(Dict{
		Name("Z"): Dict{Name("Value"): String("original")},
		Name("A"): Number(1),
	}, nil)
	var keys []Name
	for key := range stream.Keys() {
		keys = append(keys, key)
	}
	if len(keys) != 2 || keys[0] != Name("A") || keys[1] != Name("Z") {
		t.Fatalf("stream keys = %#v", keys)
	}
	valueCount := 0
	for value := range stream.Values() {
		valueCount++
		if dict, ok := value.(Dict); ok {
			dict[Name("Value")] = String("changed")
		}
	}
	if valueCount != 2 {
		t.Fatalf("stream values = %d, want 2", valueCount)
	}
	gotValue, ok := stream.Get(Name("Z"))
	if !ok {
		t.Fatal("stream mapping omitted Z")
	}
	if got := gotValue.(Dict)[Name("Value")]; string(got.(String)) != "original" {
		t.Fatalf("stream value was not copied: %#v", got)
	}
	itemCount := 0
	for key := range stream.Items() {
		itemCount++
		if itemCount == 1 && key != Name("A") {
			t.Fatalf("stream items started with %q", key)
		}
		if itemCount == 1 {
			break
		}
	}
	if itemCount != 1 {
		t.Fatalf("stream items early stop count = %d", itemCount)
	}
}

func TestStreamBufferDigestAvoidsPayloadCopy(t *testing.T) {
	stream := newStream(nil, []byte("abc"))
	length, digest := stream.BufferDigest()
	if length != 3 || digest != "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad" {
		t.Fatalf("stream digest = %d/%q", length, digest)
	}
}

func TestStreamDecodedBufferDigestUsesDecodedContents(t *testing.T) {
	var encoded bytes.Buffer
	writer := zlib.NewWriter(&encoded)
	if _, err := writer.Write([]byte("abc")); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	stream := newStream(Dict{Name("Filter"): Name("FlateDecode")}, encoded.Bytes())
	length, digest, err := stream.DecodedBufferDigestWithDocument(nil)
	if err != nil {
		t.Fatal(err)
	}
	if length != 3 || digest != "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad" {
		t.Fatalf("decoded stream digest = %d/%q", length, digest)
	}
}

func TestEncryptedIndirectStreamDigestUsesDecryptedContents(t *testing.T) {
	d, err := Open(testfixture.Path(t, "acceptance_encrypted_r2.pdf"), WithPassword("secret"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = d.Close() }()
	for object, objectErr := range d.Objects() {
		if objectErr != nil {
			t.Fatal(objectErr)
		}
		if object.Ref().Object != 5 {
			continue
		}
		stream, ok := object.ValueCopy().(Stream)
		if !ok {
			t.Fatalf("object 5 = %T, want stream", object.ValueCopy())
		}
		decoded, decodeErr := stream.DecodedBufferWithDocumentWithError(d)
		if decodeErr != nil || string(decoded) != "BT /F1 12 Tf 72 740 Td (Encrypted report) Tj ET" {
			t.Fatalf("encrypted object stream = %q, err=%v", decoded, decodeErr)
		}
		return
	}
	t.Fatal("encrypted object 5 was not found")
}

func TestRevisionCountWithoutPreviousRevision(t *testing.T) {
	d := &Document{trailer: Dict{Name("Size"): Number(1)}, objects: map[Ref]Object{}}
	if got := d.RevisionCount(); got != 1 {
		t.Fatalf("revision count = %d", got)
	}
}

func TestBufferDigestDoesNotChangeBufferContents(t *testing.T) {
	d, err := OpenBytes([]byte("%PDF-1.4\n%%EOF\n"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = d.Close() }()
	length, digest := d.BufferDigest()
	if length != len(d.Buffer()) || digest == "" {
		t.Fatalf("buffer digest = %d/%q", length, digest)
	}
}

func TestObjectsAreReturnedInStableReferenceOrder(t *testing.T) {
	d := &Document{objects: map[Ref]Object{
		{Object: 3, Generation: 0}: Number(3), {Object: 1, Generation: 2}: Number(12), {Object: 1, Generation: 0}: Number(1), {Object: 2, Generation: 0}: Number(2),
	}}
	got, err := d.CollectObjects()
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 4 || got[0].Ref() != (Ref{Object: 1, Generation: 0}) || got[1].Ref() != (Ref{Object: 1, Generation: 2}) || got[2].Ref() != (Ref{Object: 2, Generation: 0}) || got[3].Ref() != (Ref{Object: 3, Generation: 0}) {
		t.Fatalf("objects = %#v", got)
	}
}

func TestObjectsFollowPhysicalSourceOrderAndRetainDuplicateRevisions(t *testing.T) {
	d := &Document{data: []byte("%PDF-1.4\n1 0 obj\n(old)\nendobj\n1 0 obj\n(new)\nendobj\n")}
	objects, err := d.CollectObjects()
	if err != nil {
		t.Fatal(err)
	}
	if len(objects) != 2 || objects[0].Ref() != (Ref{Object: 1}) || objects[1].Ref() != (Ref{Object: 1}) {
		t.Fatalf("source objects = %#v", objects)
	}
	if string(objects[0].ValueCopy().(String)) != "old" || string(objects[1].ValueCopy().(String)) != "new" {
		t.Fatalf("source object values = %#v/%#v", objects[0].ValueCopy(), objects[1].ValueCopy())
	}
}

func TestObjectsBorrowLargeSourceStreamsUntilSnapshot(t *testing.T) {
	skipAllocationCheckUnderRace(t)
	const payloadSize = 8 << 20
	data := append([]byte("%PDF-1.4\n1 0 obj\n<< /Length 8388608 >>\nstream\n"), bytes.Repeat([]byte{'x'}, payloadSize)...)
	data = append(data, []byte("\nendstream\nendobj\n")...)
	d := &Document{data: data}
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	count := 0
	for _, err := range d.Objects() {
		if err != nil {
			t.Fatal(err)
		}
		count++
	}
	runtime.ReadMemStats(&after)
	if count != 1 {
		t.Fatalf("object count = %d, want 1", count)
	}
	if allocated := after.TotalAlloc - before.TotalAlloc; allocated >= payloadSize/2 {
		t.Fatalf("borrowed object traversal allocated %d bytes for %d-byte stream", allocated, payloadSize)
	}
}

func TestObjectCountAndRefsAreRepeatable(t *testing.T) {
	d := &Document{objects: map[Ref]Object{
		{Object: 3, Generation: 0}: Number(3),
		{Object: 1, Generation: 2}: Number(12),
		{Object: 1, Generation: 0}: Number(1),
		{Object: 2, Generation: 0}: Number(2),
	}}
	if got := d.ObjectCount(); got != 4 {
		t.Fatalf("object count = %d, want 4", got)
	}
	collect := func() []Ref {
		var refs []Ref
		for ref, err := range d.ObjectRefs() {
			if err != nil {
				t.Fatalf("object refs: %v", err)
			}
			refs = append(refs, ref)
		}
		return refs
	}
	want := []Ref{{Object: 1}, {Object: 1, Generation: 2}, {Object: 2}, {Object: 3}}
	if got := collect(); !reflect.DeepEqual(got, want) {
		t.Fatalf("object refs = %#v, want %#v", got, want)
	}
	if got := collect(); !reflect.DeepEqual(got, want) {
		t.Fatalf("second object refs = %#v, want %#v", got, want)
	}
}

func TestDocumentMappingSelectsNewestGeneration(t *testing.T) {
	d := &Document{objects: map[Ref]Object{
		{Object: 1, Generation: 0}: Number(1),
		{Object: 1, Generation: 2}: Number(12),
		{Object: 2, Generation: 0}: Number(2),
	}}

	value, ok := d.Get(1)
	if !ok || value != Number(12) {
		t.Fatalf("Get(1) = %#v, %v; want newest generation value 12", value, ok)
	}
	var keys []int
	for key, err := range d.Keys() {
		if err != nil {
			t.Fatal(err)
		}
		keys = append(keys, key)
	}
	if !reflect.DeepEqual(keys, []int{1, 2}) {
		t.Fatalf("mapping keys = %#v, want [1 2]", keys)
	}
	var values []Object
	for value, err := range d.Values() {
		if err != nil {
			t.Fatal(err)
		}
		values = append(values, value)
	}
	if !reflect.DeepEqual(values, []Object{Number(12), Number(2)}) {
		t.Fatalf("mapping values = %#v, want [12 2]", values)
	}
	var refs []Ref
	for item, err := range d.Items() {
		if err != nil {
			t.Fatal(err)
		}
		refs = append(refs, item.Ref())
	}
	wantRefs := []Ref{{Object: 1, Generation: 2}, {Object: 2}}
	if !reflect.DeepEqual(refs, wantRefs) {
		t.Fatalf("mapping item refs = %#v, want %#v", refs, wantRefs)
	}
}

func TestDocumentMappingCurrentRefRetainsHiddenGeneration(t *testing.T) {
	d := &Document{xrefs: map[Ref]xrefEntry{
		{Object: 7, Generation: 0}: newXRefEntry(42, false, true, false),
	}}

	refs := d.currentMappingRefs()
	got, ok := refs[7]
	if !ok {
		t.Fatal("hidden xref entry was dropped from current mapping refs")
	}
	if got != (Ref{Object: 7}) {
		t.Fatalf("current mapping ref = %#v, want object 7", got)
	}
}

func TestObjectsSequenceIsOrderedRepeatableAndStopsEarly(t *testing.T) {
	d := &Document{objects: map[Ref]Object{
		{Object: 3, Generation: 0}: Number(3), {Object: 1, Generation: 2}: Number(12), {Object: 1, Generation: 0}: Number(1), {Object: 2, Generation: 0}: Number(2),
	}}

	first := []Ref{}
	for object, err := range d.Objects() {
		if err != nil {
			t.Fatal(err)
		}
		first = append(first, object.Ref())
		if len(first) == 2 {
			break
		}
	}
	if len(first) != 2 || first[0] != (Ref{Object: 1, Generation: 0}) || first[1] != (Ref{Object: 1, Generation: 2}) {
		t.Fatalf("early objects = %v, want [1 0 R 1 2 R]", first)
	}

	second, err := d.CollectObjects()
	if err != nil {
		t.Fatal(err)
	}
	if len(second) != 4 || second[0].Ref() != (Ref{Object: 1, Generation: 0}) || second[3].Ref() != (Ref{Object: 3, Generation: 0}) {
		t.Fatalf("second traversal = %#v", second)
	}
	if !d.objectRefsReady || len(d.objectRefs) != 4 {
		t.Fatalf("object reference cache = ready=%v refs=%d, want ready=true refs=4", d.objectRefsReady, len(d.objectRefs))
	}
}

func TestPublicObjectSnapshotsDoNotExposeCachedContainers(t *testing.T) {
	ref := Ref{Object: 1}
	d := &Document{objects: map[Ref]Object{ref: Dict{Name("Value"): Array{String("original")}}}}
	object, ok := d.Object(ref)
	if !ok {
		t.Fatal("expected object")
	}
	object.(Dict)[Name("Value")].(Array)[0] = String("changed")
	second, ok := d.Object(ref)
	if !ok || string(second.(Dict)[Name("Value")].(Array)[0].(String)) != "original" {
		t.Fatalf("object cache was exposed: %#v", second)
	}
	for entry, err := range d.Objects() {
		if err != nil {
			t.Fatal(err)
		}
		snapshot := entry.Finalize()
		copyValue := snapshot.ValueCopy().(Dict)
		copyValue[Name("Value")].(Array)[0] = String("changed again")
	}
	third, ok := d.Object(ref)
	if !ok || string(third.(Dict)[Name("Value")].(Array)[0].(String)) != "original" {
		t.Fatalf("object sequence exposed cache: %#v", third)
	}
}

func TestIndirectObjectFinalizeClonesDocumentStreamStorage(t *testing.T) {
	stream := newStream(Dict{Name("Marker"): String("original")}, []byte("payload"))
	object := documentdata.NewIndirectObject(Ref{Object: 7}, stream)

	finalized := object.Finalize()
	copyStream := finalized.ValueCopy().(Stream)
	copyStream.DataBorrowed()[0] = 'X'
	copyStream.DictBorrowed()[Name("Marker")] = String("changed")

	if string(stream.DataBorrowed()) != "payload" || string(stream.DictBorrowed()[Name("Marker")].(String)) != "original" {
		t.Fatalf("IndirectObject shared document Stream storage: %#v", stream)
	}
}

func TestObjectWithErrorPreservesResolutionFailure(t *testing.T) {
	d := &Document{}
	if value, err := d.ObjectWithError(Ref{Object: 99}); err == nil || value != nil {
		t.Fatalf("missing object = %#v, err=%v; want an error-aware failure", value, err)
	}
	if value, ok := d.Object(Ref{Object: 99}); ok || value != nil {
		t.Fatalf("lenient missing object = %#v, %v; want nil, false", value, ok)
	}
}

func TestObjectWithErrorHonorsErrorCacheBudget(t *testing.T) {
	d := &Document{
		cacheOptionsConfigured: true,
		cacheOptions:           cacheconfig.Options{ObjectErrorBytes: 0},
	}
	_, firstErr := d.ObjectWithError(Ref{Object: 99})
	_, secondErr := d.ObjectWithError(Ref{Object: 99})
	if firstErr == nil || secondErr == nil || firstErr == secondErr {
		t.Fatalf("object errors = %v/%v, want uncached independent errors", firstErr, secondErr)
	}
	if len(d.objectErrors) != 0 {
		t.Fatalf("object error cache retained entry with zero budget: %#v", d.objectErrors)
	}
}

func TestLookupReturnsNewestObjectGeneration(t *testing.T) {
	d := &Document{objects: map[Ref]Object{{Object: 5, Generation: 0}: Number(10), {Object: 5, Generation: 3}: Number(30), {Object: 6, Generation: 0}: Number(60)}}
	ref, value, ok := d.Lookup(5)
	if !ok || ref != (Ref{Object: 5, Generation: 3}) || value != Number(30) {
		t.Fatalf("lookup = %v, %v, %v", ref, value, ok)
	}
	if _, _, ok := d.Lookup(99); ok {
		t.Fatal("missing lookup unexpectedly succeeded")
	}
	if cached, ok := d.lookupCache[5]; !ok || cached != (Ref{Object: 5, Generation: 3}) || d.lookupFound[99] {
		t.Fatalf("lookup cache = refs=%#v found=%#v", d.lookupCache, d.lookupFound)
	}
}

func TestLookupAllocationDoesNotScaleWithXRefCount(t *testing.T) {
	skipAllocationCheckUnderRace(t)
	const target = 20_000
	xrefs := make(map[Ref]xrefEntry, target)
	for objectNumber := 1; objectNumber <= target; objectNumber++ {
		xrefs[Ref{Object: objectNumber}] = xrefEntry{offset: objectNumber}
	}
	objects := make(map[Ref]Object, 11)
	for objectNumber := target - 10; objectNumber <= target; objectNumber++ {
		objects[Ref{Object: objectNumber}] = Number(objectNumber)
	}
	d := &Document{
		xrefs:       xrefs,
		objects:     objects,
		lookupCache: make(map[int]Ref),
		lookupFound: make(map[int]bool),
	}
	if _, _, err := d.lookupRawWithError(target); err != nil {
		t.Fatal(err)
	}
	next := target - 1
	allocs := testing.AllocsPerRun(3, func() {
		_, value, err := d.lookupRawWithError(next)
		if err != nil || value != Number(next) {
			t.Fatalf("lookup %d = %#v, %v", next, value, err)
		}
		next--
	})
	if allocs > 2 {
		t.Fatalf("lookup allocated %.0f objects with %d xrefs; want at most 2", allocs, target)
	}
}

func TestLookupBuildsReusableObjectNumberIndex(t *testing.T) {
	d := &Document{
		xrefs: map[Ref]xrefEntry{
			{Object: 1}:   {offset: 1},
			{Object: 50}:  {offset: 50},
			{Object: 100}: {offset: 100},
		},
		objects: map[Ref]Object{{Object: 100}: Number(42)},
	}
	if _, value, err := d.lookupRawWithError(100); err != nil || value != Number(42) {
		t.Fatalf("lookup = %#v, %v; want 42", value, err)
	}
	if ref, ok := d.lookupCache[1]; !ok || ref != (Ref{Object: 1}) {
		t.Fatalf("lookup did not retain reusable xref index: %#v", d.lookupCache)
	}
}

func TestLookupWithErrorDistinguishesMissingAndMalformedObjects(t *testing.T) {
	d := &Document{
		data:    []byte("%PDF-1.4\n7 0 obj << /Broken >> endobj\n"),
		xrefs:   map[Ref]xrefEntry{{Object: 7}: {offset: 9}},
		objects: map[Ref]Object{{Object: 5, Generation: 2}: Number(30)},
	}
	ref, value, err := d.LookupWithError(5)
	if err != nil || ref != (Ref{Object: 5, Generation: 2}) || value != Number(30) {
		t.Fatalf("resolved lookup = %v, %v, %v", ref, value, err)
	}
	if _, _, err := d.LookupWithError(99); err != ErrObjectNotFound {
		t.Fatalf("missing lookup error = %v, want ErrObjectNotFound", err)
	}
	if _, _, err := d.LookupWithError(7); err == nil || err == ErrObjectNotFound {
		t.Fatalf("malformed lookup error = %v, want a parse error", err)
	}
}

func TestDocumentExposesBufferAndTokens(t *testing.T) {
	d := &Document{data: []byte("%PDF-1.4\n/Name 12")}
	buffer := d.Buffer()
	tokens, err := d.CollectTokens()
	if err != nil || string(buffer) != string(d.data) || len(tokens) != 2 || tokens[0].Kind() != TokenName || tokens[1].Number() != 12 {
		t.Fatalf("buffer/tokens = %q, %#v, err=%v", buffer, tokens, err)
	}
	buffer[0] = 'X'
	if d.data[0] == 'X' {
		t.Fatal("buffer was not defensive")
	}
}

func TestDocumentTokensFromStartsAtSourceOffset(t *testing.T) {
	data := []byte("%PDF-1.4\nstream\n(\nendstream\n7 0 obj\n8\nendobj\n")
	offset := bytes.Index(data, []byte("7 0 obj"))
	d := &Document{data: data}
	var tokens []Token
	for token, err := range d.TokensFrom(offset) {
		if err != nil {
			t.Fatal(err)
		}
		tokens = append(tokens, token)
		if len(tokens) == 4 {
			break
		}
	}
	if len(tokens) != 4 || tokens[0].Offset() != offset || tokens[0].NumberText() != "7" || tokens[3].NumberText() != "8" {
		t.Fatalf("TokensFrom(%d) = %#v", offset, tokens)
	}
}

func TestDocumentBufferPreservesEmptyData(t *testing.T) {
	d := &Document{data: make([]byte, 0)}
	if d.Buffer() == nil {
		t.Fatal("empty document buffer became nil")
	}
}

func TestResolveRefReturnsIndependentObjectSnapshot(t *testing.T) {
	d := &Document{objects: map[Ref]Object{
		{Object: 1}: Dict{Name("Meta"): Dict{Name("Value"): String("original")}},
	}}
	value, err := d.ResolveRef(Ref{Object: 1})
	if err != nil {
		t.Fatal(err)
	}
	value.(Dict)[Name("Meta")].(Dict)[Name("Value")] = String("changed")
	again, err := d.ResolveRef(Ref{Object: 1})
	if err != nil {
		t.Fatal(err)
	}
	got := again.(Dict)[Name("Meta")].(Dict)[Name("Value")].(String)
	if string(got) != "original" {
		t.Fatalf("ResolveRef exposed cached object: %q", got)
	}
}

func TestResolveReturnsIndependentObjectSnapshot(t *testing.T) {
	d := &Document{objects: map[Ref]Object{
		{Object: 1}: Dict{Name("Meta"): Dict{Name("Value"): String("original")}},
	}}
	value := d.Resolve(Ref{Object: 1})
	value.(Dict)[Name("Meta")].(Dict)[Name("Value")] = String("changed")
	again := d.Resolve(Ref{Object: 1})
	got := again.(Dict)[Name("Meta")].(Dict)[Name("Value")].(String)
	if string(got) != "original" {
		t.Fatalf("Resolve exposed cached object: %q", got)
	}
}

func TestDocumentTokensAreRepeatableAndStopEarly(t *testing.T) {
	d := &Document{data: []byte("%PDF-1.4\n/First /Second")}
	var first []string
	for token, err := range d.Tokens() {
		if err != nil {
			t.Fatal(err)
		}
		first = append(first, token.Text())
		break
	}
	var second []string
	for token, err := range d.Tokens() {
		if err != nil {
			t.Fatal(err)
		}
		second = append(second, token.Text())
	}
	if len(first) != 1 || len(second) != 2 || second[0] != "First" || second[1] != "Second" {
		t.Fatalf("tokens = %#v, %#v", first, second)
	}
}

func TestOpenFallbackScannerUsesStreamLength(t *testing.T) {
	pdf := []byte("%PDF-1.4\n1 0 obj\n<< /Length 14 >>\nstream\nabc endobj xyz\nendstream\nendobj\n")
	d, err := OpenBytes(pdf)
	if err != nil {
		t.Fatal(err)
	}
	s, ok := d.Object(Ref{Object: 1, Generation: 0})
	stream, streamOK := s.(Stream)
	if !ok || !streamOK || string(stream.DataBorrowed()) != "abc endobj xyz" {
		t.Fatalf("scanned stream = %#v, ok=%v", s, ok)
	}
}
