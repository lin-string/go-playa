package document

import (
	"bytes"
	"compress/zlib"
	"math"
	"sync"
	"testing"
)

func TestCloneMetadataPreservesAbsentMap(t *testing.T) {
	if got := cloneMetadata(nil); got != nil {
		t.Fatalf("cloned absent metadata = %#v, want nil", got)
	}
}

func TestMetadataIgnoresNonFiniteInfoNumbers(t *testing.T) {
	d := &Document{
		trailer: Dict{Name("Info"): Dict{
			Name("Finite"): Number(3), Name("Infinite"): Number(math.Inf(1)), Name("NaN"): Number(math.NaN()),
		}},
	}
	metadata := d.Metadata()
	if metadata["Finite"] != "3" {
		t.Fatalf("finite metadata = %#v", metadata)
	}
	if _, ok := metadata["Infinite"]; ok {
		t.Fatalf("infinite metadata was exposed: %#v", metadata)
	}
	if _, ok := metadata["NaN"]; ok {
		t.Fatalf("NaN metadata was exposed: %#v", metadata)
	}
}

func TestMetadataWithErrorReportsMalformedInfoRoot(t *testing.T) {
	d := &Document{trailer: Dict{Name("Info"): Number(1)}}
	if metadata, err := d.MetadataWithError(); err == nil || metadata != nil {
		t.Fatalf("malformed metadata = %#v, err = %v", metadata, err)
	}
	firstErr := d.metadataErr
	if firstErr == nil {
		t.Fatal("malformed metadata error was not cached")
	}
	if metadata, err := d.MetadataWithError(); err != firstErr || metadata != nil {
		t.Fatalf("cached malformed metadata = %#v, err=%v", metadata, err)
	}
	if metadata := d.Metadata(); len(metadata) != 0 {
		t.Fatalf("compatibility metadata = %#v, want empty", metadata)
	}
}

func TestMetadataWithErrorReportsUnresolvedInfoField(t *testing.T) {
	d := &Document{
		trailer: Dict{Name("Info"): Dict{Name("Title"): Ref{Object: 99}}},
	}
	if metadata, err := d.MetadataWithError(); err == nil || metadata != nil {
		t.Fatalf("unresolved Info field = %#v, err=%v", metadata, err)
	}
}

func TestMetadataDoesNotCacheOversizedInfo(t *testing.T) {
	value := string(bytes.Repeat([]byte{'x'}, metadataCacheLimit+1))
	d := &Document{trailer: Dict{Name("Info"): Dict{Name("Subject"): String(value)}}}
	first := d.Metadata()
	if first["Subject"] != value {
		t.Fatalf("oversized metadata value was changed: len=%d", len(first["Subject"]))
	}
	if d.metadataCacheable || d.metadataCache != nil {
		t.Fatalf("oversized metadata entered cache: cacheable=%v cache=%v", d.metadataCacheable, d.metadataCache)
	}
	second := d.Metadata()
	if second["Subject"] != value {
		t.Fatalf("uncached metadata value was not repeatable: len=%d", len(second["Subject"]))
	}
}

func TestDocumentInfoPreservesOriginalPDFValues(t *testing.T) {
	d := &Document{
		trailer: Dict{Name("Info"): Ref{Object: 1}},
		objects: map[Ref]Object{{Object: 1}: Dict{
			Name("Title"):  String("Report"),
			Name("Count"):  Number(3),
			Name("Custom"): Array{Number(1), Number(2)},
		}},
	}
	info := d.Info()
	if string(info[Name("Title")].(String)) != "Report" {
		t.Fatalf("info title = %#v", info[Name("Title")])
	}
	if info[Name("Count")] != Number(3) {
		t.Fatalf("info count = %#v", info[Name("Count")])
	}
	if _, ok := info[Name("Custom")].(Array); !ok {
		t.Fatalf("info custom value = %#v", info[Name("Custom")])
	}
}

func TestMetadataAndXMLFollowMultiLevelIndirectValues(t *testing.T) {
	d := &Document{
		trailer: Dict{Name("Root"): Ref{Object: 1}, Name("Info"): Ref{Object: 6}},
		objects: map[Ref]Object{
			{Object: 1}: Dict{Name("Metadata"): Ref{Object: 2}},
			{Object: 2}: Ref{Object: 3},
			{Object: 3}: newStream(Dict{Name("Type"): Ref{Object: 4}, Name("Subtype"): Ref{Object: 5}}, []byte("<xmpmeta/>")),
			{Object: 4}: Name("Metadata"),
			{Object: 5}: Name("XML"),
			{Object: 6}: Ref{Object: 7},
			{Object: 7}: Dict{Name("Title"): String("Report")},
		},
	}
	info, err := d.MetadataWithError()
	if err != nil || info["Title"] != "Report" {
		t.Fatalf("indirect metadata = %#v, err = %v", info, err)
	}
	xml, err := d.MetadataXML()
	if err != nil || string(xml) != "<xmpmeta/>" {
		t.Fatalf("indirect XML = %q, err = %v", xml, err)
	}
}

func TestMetadataInfoFieldsFollowMultiLevelIndirectValues(t *testing.T) {
	d := &Document{trailer: Dict{Name("Info"): Ref{Object: 1}}, objects: map[Ref]Object{
		{Object: 1}: Dict{
			Name("Title"):   Ref{Object: 2},
			Name("Author"):  Ref{Object: 3},
			Name("Version"): Ref{Object: 4},
		},
		{Object: 2}: Ref{Object: 5}, {Object: 5}: String("Report"),
		{Object: 3}: Ref{Object: 6}, {Object: 6}: Name("Analyst"),
		{Object: 4}: Ref{Object: 7}, {Object: 7}: Number(2.5),
	}}
	metadata, err := d.MetadataWithError()
	if err != nil || metadata["Title"] != "Report" || metadata["Author"] != "Analyst" || metadata["Version"] != "2.5" {
		t.Fatalf("indirect Info fields = %#v, err=%v", metadata, err)
	}
}

func TestMetadataXMLCachesDecodedStreamAndReturnsCopies(t *testing.T) {
	var encoded bytes.Buffer
	writer := zlib.NewWriter(&encoded)
	_, _ = writer.Write([]byte("<xmpmeta/>"))
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	d := &Document{
		trailer: Dict{Name("Root"): Ref{Object: 1}},
		objects: map[Ref]Object{
			{Object: 1}: Dict{Name("Metadata"): Ref{Object: 2}},
			{Object: 2}: newStream(Dict{Name("Filter"): Name("FlateDecode")}, encoded.Bytes()),
		},
	}
	first, err := d.MetadataXML()
	if err != nil || string(first) != "<xmpmeta/>" {
		t.Fatalf("first metadata = %q, err = %v", first, err)
	}
	first[0] = 'X'
	second, err := d.MetadataXML()
	if err != nil || string(second) != "<xmpmeta/>" {
		t.Fatalf("cached metadata = %q, err = %v", second, err)
	}
}

func TestMetadataXMLConcurrentCallsShareLazyResult(t *testing.T) {
	d := &Document{
		trailer: Dict{Name("Root"): Ref{Object: 1}},
		objects: map[Ref]Object{
			{Object: 1}: Dict{Name("Metadata"): Ref{Object: 2}},
			{Object: 2}: newStream(Dict{Name("Type"): Name("Metadata"), Name("Subtype"): Name("XML")}, []byte("<xmpmeta/>")),
		},
	}
	const callers = 8
	results := make(chan string, callers)
	var wg sync.WaitGroup
	for i := 0; i < callers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			data, err := d.MetadataXML()
			if err != nil {
				results <- "error: " + err.Error()
				return
			}
			results <- string(data)
		}()
	}
	wg.Wait()
	close(results)
	for result := range results {
		if result != "<xmpmeta/>" {
			t.Fatalf("concurrent metadata result = %q", result)
		}
	}
}

func TestMetadataXMLIgnoresNonXMLCatalogStreams(t *testing.T) {
	d := &Document{
		trailer: Dict{Name("Root"): Ref{Object: 1}},
		objects: map[Ref]Object{
			{Object: 1}: Dict{Name("Metadata"): Ref{Object: 2}},
			{Object: 2}: newStream(Dict{Name("Type"): Name("EmbeddedFile"), Name("Subtype"): Name("application/octet-stream")}, []byte("not XMP")),
		},
	}

	got, err := d.MetadataXML()
	if err != nil {
		t.Fatal(err)
	}
	if got != nil {
		t.Fatalf("non-XML catalog stream returned metadata %q", got)
	}
}

func TestMetadataXMLReportsUnresolvedMetadataReference(t *testing.T) {
	d := &Document{
		trailer: Dict{Name("Root"): Ref{Object: 1}},
		objects: map[Ref]Object{
			{Object: 1}: Dict{Name("Metadata"): Ref{Object: 99}},
		},
	}
	if data, err := d.MetadataXML(); err == nil || data != nil {
		t.Fatalf("unresolved metadata = %q, err=%v", data, err)
	}
}

func TestMetadataXMLDoesNotCacheOversizedResults(t *testing.T) {
	data := bytes.Repeat([]byte{'x'}, metadataXMLCacheLimit+1)
	d := &Document{
		trailer: Dict{Name("Root"): Ref{Object: 1}},
		objects: map[Ref]Object{
			{Object: 1}: Dict{Name("Metadata"): Ref{Object: 2}},
			{Object: 2}: newStream(Dict{Name("Type"): Name("Metadata"), Name("Subtype"): Name("XML")}, data),
		},
	}
	got, err := d.MetadataXML()
	if err != nil || len(got) != len(data) {
		t.Fatalf("oversized metadata = len %d, err=%v", len(got), err)
	}
	if d.metadataXMLCacheable || d.metadataXMLCache != nil {
		t.Fatalf("oversized metadata entered cache: cacheable=%v cache=%d", d.metadataXMLCacheable, len(d.metadataXMLCache))
	}
}

func TestMetadataXMLPreservesEmptyCachedBytes(t *testing.T) {
	d := &Document{
		metadataXMLReady:     true,
		metadataXMLCacheable: true,
		metadataXMLCache:     make([]byte, 0),
	}
	data, err := d.MetadataXML()
	if err != nil || data == nil {
		t.Fatalf("cached empty metadata XML = %#v, err=%v", data, err)
	}
	mutated := append(data, 'x')
	if len(mutated) != 1 {
		t.Fatalf("metadata XML copy length = %d", len(mutated))
	}
	if len(d.metadataXMLCache) != 0 {
		t.Fatal("metadata XML cache was exposed through returned bytes")
	}
}
