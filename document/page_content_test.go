package document

import (
	"bytes"
	"errors"
	"iter"
	"math"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/lin-string/go-playa/cacheconfig"
	"github.com/lin-string/go-playa/contentconfig"
	"github.com/lin-string/go-playa/contentdata"
	"github.com/lin-string/go-playa/coordinates"
	"github.com/lin-string/go-playa/documentdata"
	"github.com/lin-string/go-playa/geometry"
	"github.com/lin-string/go-playa/internal/testfixture"
	"github.com/lin-string/go-playa/structuredata"
	"github.com/lin-string/go-playa/textconfig"
)

func TestContentRootErrorCacheHonorsBudget(t *testing.T) {
	d := &Document{cacheOptionsConfigured: true, cacheOptions: cacheconfig.Options{ContentRootErrorBytes: 0}}
	ref := Ref{Object: 7}
	first := d.cacheContentRootError(ref, errors.New("contents failed"))
	second := d.cacheContentRootError(ref, errors.New("contents failed"))
	if first == nil || second == nil || first == second {
		t.Fatalf("content root errors = %v/%v, want uncached independent errors", first, second)
	}
	if len(d.contentRootErrors) != 0 {
		t.Fatalf("content root error cache retained entry with zero budget: %#v", d.contentRootErrors)
	}
}

func TestDecodedStreamCacheHasMemoryBudget(t *testing.T) {
	ref := Ref{Object: 99}
	d := &Document{decodedStreamCache: map[Ref][]byte{}}
	data := bytes.Repeat([]byte{'x'}, decodedStreamCacheLimit+1)
	if _, err := decodeContentStream(d, ref, newStream(nil, data)); err != nil {
		t.Fatal(err)
	}
	if len(d.decodedStreamCache) != 0 || d.decodedStreamCacheBytes != 0 {
		t.Fatalf("oversized stream was cached: entries=%d bytes=%d", len(d.decodedStreamCache), d.decodedStreamCacheBytes)
	}
}

func TestDecodeContentStreamRecoversCorruptLZWPrefix(t *testing.T) {
	ref := Ref{Object: 17}
	d := &Document{decodedStreamCache: map[Ref][]byte{}}
	stream := newStream(Dict{Name("Filter"): Name("LZWDecode")}, []byte{0x80, 0x10, 0x65, 0x80})
	decoded, err := decodeContentStream(d, ref, stream)
	if err != nil || string(decoded) != "A" {
		t.Fatalf("decoded content stream = %q, err=%v", decoded, err)
	}
}

func TestDecodedStreamCacheRejectsAccountingOverflow(t *testing.T) {
	ref := Ref{Object: 199}
	d := &Document{
		decodedStreamCache:      map[Ref][]byte{},
		decodedStreamCacheBytes: math.MaxInt - 1,
	}
	if _, err := decodeContentStream(d, ref, newStream(nil, []byte("BT ET"))); err != nil {
		t.Fatal(err)
	}
	if _, ok := d.decodedStreamCache[ref]; ok {
		t.Fatal("decoded stream cache accepted an overflowing byte counter")
	}
	if d.decodedStreamCacheBytes != math.MaxInt-1 {
		t.Fatalf("decoded stream cache byte counter changed: got=%d", d.decodedStreamCacheBytes)
	}
}

func TestDecodedStreamCacheCoalescesConcurrentDecodes(t *testing.T) {
	ref := Ref{Object: 100}
	d := &Document{decodedStreamCache: map[Ref][]byte{}}
	started := make(chan struct{})
	release := make(chan struct{})
	var mu sync.Mutex
	var startOnce sync.Once
	decodes := 0
	decode := func([]byte, []string, []Dict) ([]byte, error) {
		mu.Lock()
		decodes++
		mu.Unlock()
		startOnce.Do(func() { close(started) })
		<-release
		return []byte("decoded"), nil
	}

	var wg sync.WaitGroup
	results := make(chan string, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			data, err := decodeContentStreamWithDecoder(d, ref, newStream(nil, []byte("raw")), decode)
			if err != nil {
				t.Errorf("decode content stream: %v", err)
				return
			}
			results <- string(data)
		}()
	}
	<-started
	close(release)
	wg.Wait()
	close(results)
	for result := range results {
		if result != "decoded" {
			t.Fatalf("decoded stream = %q, want decoded", result)
		}
	}
	mu.Lock()
	defer mu.Unlock()
	if decodes != 1 {
		t.Fatalf("decode count = %d, want 1", decodes)
	}
}

func TestDecodedStreamCacheReusesMultiLevelIndirectReference(t *testing.T) {
	d := &Document{
		objects:            map[Ref]Object{{Object: 1}: Ref{Object: 2}, {Object: 2}: newStream(nil, []byte("raw"))},
		decodedStreamCache: map[Ref][]byte{},
	}
	decodes := 0
	decode := func([]byte, []string, []Dict) ([]byte, error) {
		decodes++
		return []byte("decoded"), nil
	}
	for _, value := range []Object{Ref{Object: 1}, Ref{Object: 2}} {
		if _, err := decodeContentStreamWithDecoder(d, value, newStream(nil, []byte("raw")), decode); err != nil {
			t.Fatal(err)
		}
	}
	if decodes != 1 {
		t.Fatalf("decode count = %d, want 1", decodes)
	}
}

func TestReleaseTransientCachesSynchronizesDecodedStream(t *testing.T) {
	ref := Ref{Object: 102}
	d := &Document{data: []byte("source"), decodedStreamCache: map[Ref][]byte{}}
	started := make(chan struct{})
	release := make(chan struct{})
	decode := func([]byte, []string, []Dict) ([]byte, error) {
		close(started)
		<-release
		return []byte("decoded"), nil
	}
	result := make(chan error, 1)
	go func() {
		_, err := decodeContentStreamWithDecoder(d, ref, newStream(nil, []byte("raw")), decode)
		result <- err
	}()
	<-started
	released := make(chan struct{})
	go func() {
		d.ReleaseTransientCaches()
		close(released)
	}()
	select {
	case <-released:
		t.Fatal("cache release completed while stream decoding was in flight")
	case <-time.After(10 * time.Millisecond):
	}
	close(release)
	if err := <-result; err != nil {
		t.Fatal(err)
	}
	select {
	case <-released:
	case <-time.After(time.Second):
		t.Fatal("cache release did not complete after stream decoding")
	}
}

func TestDecodedStreamCacheReusesDeterministicDecodeErrors(t *testing.T) {
	ref := Ref{Object: 101}
	d := &Document{decodedStreamCache: map[Ref][]byte{}}
	decodeErr := errors.New("decode failed")
	var mu sync.Mutex
	decodes := 0
	decode := func([]byte, []string, []Dict) ([]byte, error) {
		mu.Lock()
		decodes++
		mu.Unlock()
		return nil, decodeErr
	}
	for i := 0; i < 2; i++ {
		_, err := decodeContentStreamWithDecoder(d, ref, newStream(nil, []byte("raw")), decode)
		if err != decodeErr {
			t.Fatalf("decode error = %v, want %v", err, decodeErr)
		}
	}
	mu.Lock()
	defer mu.Unlock()
	if decodes != 1 {
		t.Fatalf("decode count = %d, want 1", decodes)
	}
}

func TestPageContentSequenceAPISurface(t *testing.T) {
	page := Page{}
	sequences := []any{
		page.Interp(nil, contentconfig.Options{}),
		page.Flatten(nil, contentconfig.Options{}),
		page.Texts(nil),
		page.Glyphs(nil),
		page.Paths(nil),
		page.Images(nil),
		page.XObjects(nil),
		page.FontsSeq(nil),
		page.MarkedContent(nil),
		page.Tags(nil),
		page.StructureSeq(nil),
		OutlineNode{}.ChildrenSeq(),
		StructElement{}.ChildrenSeq(),
		FormField{}.KidsSeq(),
		MarkedContent{}.ChildrenSeq(),
		MarkedContent{}.OpsSeq(),
	}
	for _, sequence := range sequences {
		if sequence == nil {
			t.Fatal("page content sequence is nil")
		}
	}
}

func TestContentIteratorDefersPagePropertiesUntilMarkedContent(t *testing.T) {
	d := &Document{pagePropCache: map[Ref]Dict{}}
	p := Page{ref: Ref{Object: 1}, dict: Dict{
		Name("Contents"):  newStream(nil, []byte("1 0 m")),
		Name("Resources"): Dict{Name("Properties"): Dict{Name("P1"): Dict{Name("MCID"): Number(1)}}},
	}}
	it := newContentOpIterator(d, p)
	if _, err, ok := it.next(); err != nil || !ok {
		t.Fatalf("first content op = err=%v ok=%v", err, ok)
	}
	if len(d.pagePropCache) != 0 {
		t.Fatalf("page properties resolved before marked content: %#v", d.pagePropCache)
	}
	for {
		_, err, ok := it.next()
		if err != nil || !ok {
			break
		}
	}
	if len(d.pagePropCache) != 0 {
		t.Fatalf("page properties resolved without marked content: %#v", d.pagePropCache)
	}
}

func TestPageTagsDefersPagePropertiesWithoutNamedDP(t *testing.T) {
	d := &Document{pagePropCache: map[Ref]Dict{}}
	p := Page{ref: Ref{Object: 1}, dict: Dict{
		Name("Contents"):  newStream(nil, []byte("/Artifact MP")),
		Name("Resources"): Dict{Name("Properties"): Dict{Name("P1"): Dict{Name("MCID"): Number(1)}}},
	}}
	for tag, err := range d.PageTagsSeq(p) {
		if err != nil || tag.Name() != "Artifact" {
			t.Fatalf("tag = %#v, err=%v", tag, err)
		}
	}
	if len(d.pagePropCache) != 0 {
		t.Fatalf("page properties resolved without named DP: %#v", d.pagePropCache)
	}
}

func TestPageContentReusesDecodedIndirectStream(t *testing.T) {
	ref := Ref{Object: 7}
	d := &Document{
		objects:            map[Ref]Object{ref: newStream(Dict{}, []byte("BT ET"))},
		decodedStreamCache: map[Ref][]byte{},
	}
	p := Page{dict: Dict{Name("Contents"): ref}}
	for i := 0; i < 2; i++ {
		if _, err := p.Content(d); err != nil {
			t.Fatal(err)
		}
	}
	for token, err := range p.Tokens(d) {
		if err != nil {
			t.Fatal(err)
		}
		if token.Kind() == TokenEOF {
			t.Fatal("page token sequence yielded EOF")
		}
	}
	if len(d.decodedStreamCache) != 1 {
		t.Fatalf("decoded stream cache size = %d, want 1", len(d.decodedStreamCache))
	}
	if got := string(d.decodedStreamCache[ref]); got != "BT ET" {
		t.Fatalf("decoded stream cache = %q, want original decoded content", got)
	}
}

func TestPageInterpreterReusesIndirectSingleStream(t *testing.T) {
	ref := Ref{Object: 8}
	d := &Document{
		objects:            map[Ref]Object{ref: newStream(Dict{}, []byte("BT ET"))},
		decodedStreamCache: map[Ref][]byte{},
	}
	p := Page{dict: Dict{Name("Contents"): ref}}
	for i := 0; i < 2; i++ {
		for _, err := range p.Contents(d) {
			if err != nil {
				t.Fatal(err)
			}
		}
	}
	if len(d.decodedStreamCache) != 1 {
		t.Fatalf("decoded stream cache size = %d, want 1", len(d.decodedStreamCache))
	}
}

func TestPageTextsPreserveOwningPage(t *testing.T) {
	d := &Document{}
	page := Page{ref: Ref{Object: 9}, dict: Dict{Name("Contents"): newStream(nil, []byte("BT /F1 10 Tf (A) Tj ET"))}}
	for text, err := range page.Texts(d) {
		if err != nil {
			t.Fatal(err)
		}
		if !text.HasPage() || text.Page() != page.ref {
			t.Fatalf("text page = %#v", text)
		}
		if len(text.glyphs) > 0 && (!text.glyphs[0].HasPage() || text.glyphs[0].Page() != page.ref) {
			t.Fatalf("glyph page = %#v", text.glyphs[0])
		}
		return
	}
	t.Fatal("no text object")
}

func TestPageGlyphsPreserveOwningPage(t *testing.T) {
	d := &Document{}
	page := Page{
		ref: Ref{Object: 11},
		dict: Dict{
			Name("Contents"): newStream(nil, []byte("BT /F1 10 Tf (A) Tj ET")),
			Name("Resources"): Dict{Name("Font"): Dict{Name("F1"): Dict{
				Name("Subtype"): Name("Type1"), Name("BaseFont"): Name("Helvetica"),
			}}},
		},
	}
	for glyph, err := range page.Glyphs(d) {
		if err != nil {
			t.Fatal(err)
		}
		if !glyph.HasPage() || glyph.Page() != page.ref {
			t.Fatalf("glyph page = %#v", glyph)
		}
		return
	}
	t.Fatal("no glyph object")
}

func TestPagePathsPreserveOwningPage(t *testing.T) {
	d := &Document{}
	page := Page{ref: Ref{Object: 10}, dict: Dict{Name("Contents"): newStream(nil, []byte("0 0 m 1 1 l S"))}}
	for path, err := range page.Paths(d) {
		if err != nil {
			t.Fatal(err)
		}
		if !path.HasPage() || path.Page() != page.ref {
			t.Fatalf("path page = %#v", path)
		}
		return
	}
	t.Fatal("no path object")
}

func TestPageFlattenSequenceIsRepeatableAndStopsEarly(t *testing.T) {
	d := &Document{}
	page := Page{dict: Dict{Name("Contents"): newStream(nil, []byte(
		"BT /F1 10 Tf (A) Tj ET BT /F1 10 Tf (B) Tj ET",
	))}}
	first := 0
	for object, err := range page.Flatten(d, contentconfig.Options{Filter: FilterText}) {
		if err != nil {
			t.Fatal(err)
		}
		if object.Kind() != ContentText {
			t.Fatalf("first object kind = %q", object.Kind())
		}
		first++
		break
	}
	second := 0
	for object, err := range page.Flatten(d, contentconfig.Options{Filter: FilterText}) {
		if err != nil {
			t.Fatal(err)
		}
		if object.Kind() != ContentText {
			t.Fatalf("object kind = %q", object.Kind())
		}
		second++
	}
	if first != 1 || second != 2 {
		t.Fatalf("flatten counts = first %d, second %d", first, second)
	}
}

func TestPageTextsSequenceIsRepeatableAndRecoversLaterEOF(t *testing.T) {
	d := &Document{}
	page := Page{dict: Dict{Name("Contents"): Array{
		newStream(nil, []byte("BT /F1 10 Tf (A) Tj ET")),
		newStream(nil, []byte("(")),
	}}}
	first := 0
	for text, err := range page.Texts(d) {
		if err != nil {
			t.Fatal(err)
		}
		if text.Text() != "A" {
			t.Fatalf("first text = %q", text.Text())
		}
		first++
		break
	}
	second, deferredErr := 0, false
	for text, err := range page.Texts(d) {
		if err != nil {
			deferredErr = true
			break
		}
		if text.Text() != "A" {
			t.Fatalf("repeated text = %q", text.Text())
		}
		second++
	}
	if first != 1 || second != 1 || deferredErr {
		t.Fatalf("text sequence = first %d, second %d, deferred error %v", first, second, deferredErr)
	}
}

func TestPageTextsSequenceReportsLazyFontErrors(t *testing.T) {
	pageRef := Ref{Object: 31}
	font := NewSimpleFont("CID")
	font.cid = true
	font.cmapData = []byte("1 begincidchar\n<00>\nendcidchar")
	d := &Document{pageFontCache: map[Ref]map[string]*Font{pageRef: {"F1": font}}}
	page := Page{ref: pageRef, dict: Dict{
		Name("Contents"): newStream(nil, []byte("BT /F1 10 Tf (A) Tj ET")),
	}}
	for _, err := range page.Texts(d) {
		if err == nil {
			t.Fatal("malformed lazy font produced text without error")
		}
		return
	}
	t.Fatal("malformed lazy font produced no error")
}

func TestPageTextsSequenceReportsLazyType3CharProcErrors(t *testing.T) {
	pageRef := Ref{Object: 32}
	font := NewSimpleFont("Type3")
	font.type3 = true
	font.glyphNames[65] = "A"
	font.charProcs["A"] = newStream(nil, []byte("500 0 0 0 (bad) 100 d1"))
	d := &Document{pageFontCache: map[Ref]map[string]*Font{pageRef: {"F1": font}}}
	font.document = d
	page := Page{ref: pageRef, dict: Dict{
		Name("Contents"): newStream(nil, []byte("BT /F1 10 Tf (A) Tj ET")),
	}}
	for _, err := range page.Texts(d) {
		if err == nil {
			t.Fatal("malformed Type3 CharProc produced text without error")
		}
		return
	}
	t.Fatal("malformed Type3 CharProc produced no error")
}

func TestPageGlyphsSequenceIsRepeatableAndRecoversLaterEOF(t *testing.T) {
	d := &Document{}
	page := Page{dict: Dict{Name("Contents"): Array{
		newStream(nil, []byte("BT /F1 10 Tf (A) Tj ET")),
		newStream(nil, []byte("(")),
	}}}
	first := 0
	for glyph, err := range page.Glyphs(d) {
		if err != nil {
			t.Fatal(err)
		}
		if glyph.Text() != "A" {
			t.Fatalf("first glyph = %q", glyph.Text())
		}
		first++
		break
	}
	second, deferredErr := 0, false
	for glyph, err := range page.Glyphs(d) {
		if err != nil {
			deferredErr = true
			break
		}
		if glyph.Text() != "A" {
			t.Fatalf("repeated glyph = %q", glyph.Text())
		}
		second++
	}
	if first != 1 || second != 1 || deferredErr {
		t.Fatalf("glyph sequence = first %d, second %d, deferred error %v", first, second, deferredErr)
	}
}

func TestPageExtractTextUntagged(t *testing.T) {
	d := &Document{}
	page := Page{dict: Dict{Name("Contents"): newStream(nil, []byte(
		"BT /F1 10 Tf 1 0 0 1 10 100 Tm (AB) Tj -4 0 Td (C) Tj ET",
	))}}
	text, err := page.ExtractTextUntagged(d, textconfig.Options{})
	if err != nil || text != "ABC" {
		t.Fatalf("untagged text = %q, err = %v", text, err)
	}
}

func TestPageExtractTextMatchesPublicManualSpacing(t *testing.T) {
	d, err := Open(testfixture.Path(t, "riscv-unprivileged.pdf"), WithCoordinateSpace(CoordinateSpacePage))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = d.Close() }()
	page, err := d.PageAt(0)
	if err != nil {
		t.Fatal(err)
	}
	const want = "The RISC-V Instruction Set\nManual, Volume I\nUnprivileged Architecture\nVersion 20260120: Official Release"
	got, err := page.ExtractText(d, textconfig.Options{})
	if err != nil || got != want {
		t.Fatalf("public manual extraction = %q, err=%v; want %q", got, err, want)
	}
	got, err = page.ExtractTextUntagged(d, textconfig.Options{})
	if err != nil || got != want {
		t.Fatalf("public manual untagged extraction = %q, err=%v; want %q", got, err, want)
	}
}

func TestPageExtractTextUntaggedInsertsTJGapInsideTextObject(t *testing.T) {
	d := &Document{}
	page := Page{dict: Dict{Name("Contents"): newStream(nil, []byte(
		"BT /F1 10 Tf 1 0 0 1 10 100 Tm [(A) -100 (B)] TJ ET",
	))}}
	text, err := page.ExtractTextUntagged(d, textconfig.Options{})
	if err != nil || text != "A B" {
		t.Fatalf("untagged TJ text = %q, err = %v; want A B", text, err)
	}
}

func TestPageExtractTextUntaggedBBoxIgnoresFilteredObjectsForSpacing(t *testing.T) {
	d := &Document{}
	page := Page{dict: Dict{Name("Contents"): newStream(nil, []byte(
		"BT /F1 10 Tf 1 0 0 1 10 100 Tm (A) Tj 1 0 0 1 1000 100 Tm (X) Tj 1 0 0 1 30 100 Tm (B) Tj ET",
	))}}
	text, err := page.ExtractTextUntagged(d, textconfig.Options{BBox: &[4]float64{0, 90, 100, 110}})
	if err != nil || text != "A B" {
		t.Fatalf("untagged bbox text = %q, err = %v; want A B", text, err)
	}
}

func TestPageExtractTextUntaggedBBoxRetainsFilteredLineState(t *testing.T) {
	d := &Document{}
	page := Page{dict: Dict{Name("Contents"): newStream(nil, []byte(
		"BT /F1 10 Tf 1 0 0 1 10 100 Tm (A) Tj 1 0 0 1 10 200 Tm (X) Tj 1 0 0 1 20 100 Tm (B) Tj ET",
	))}}
	text, err := page.ExtractTextUntagged(d, textconfig.Options{BBox: &[4]float64{0, 90, 100, 110}})
	if err != nil || text != "A\nB" {
		t.Fatalf("untagged bbox text state = %q, err = %v; want A\\nB", text, err)
	}
}

func TestTextObjectBBoxFilteringUsesTextAdvance(t *testing.T) {
	object := newTestText(contentdata.TextSpec{
		Origin: [2]float64{10, 100}, Displacement: [2]float64{20, 0},
		BBox: [4]float64{100, 100, 110, 110},
	}, nil, nil)
	if !textObjectCrossesBBox(object, [4]float64{15, 99, 16, 101}) {
		t.Fatalf("text advance was not considered by bbox filtering")
	}
}

func TestTextObjectBBoxFilteringUsesLastGlyphEnd(t *testing.T) {
	object := newTestText(contentdata.TextSpec{
		Origin: [2]float64{10, 100}, Displacement: [2]float64{20, 0},
		BBox: [4]float64{100, 100, 110, 110},
	}, nil, []GlyphObject{newTestGlyphWithGeometry([2]float64{10, 100}, [2]float64{2, 0})})
	if textObjectCrossesBBox(object, [4]float64{15, 99, 16, 101}) {
		t.Fatal("trailing text adjustment was treated as painted glyph extent")
	}
}

func TestPageExtractTextTaggedUsesActualTextAndSkipsArtifacts(t *testing.T) {
	d := &Document{}
	page := Page{dict: Dict{Name("Contents"): newStream(nil, []byte(
		"/Artifact BMC BT /F1 10 Tf (X) Tj ET EMC /Span << /MCID 1 /ActualText (AB) >> BDC BT /F1 10 Tf (X) Tj ET EMC",
	))}}
	text, err := page.ExtractTextTagged(d, textconfig.Options{})
	if err != nil || text != "AB" {
		t.Fatalf("tagged text = %q, err = %v", text, err)
	}
}

func TestPageExtractTextTaggedKeepsRotatedBaselineObjectsTogether(t *testing.T) {
	d := &Document{}
	page := Page{dict: Dict{Name("Contents"): newStream(nil, []byte(
		"/P << /MCID 1 >> BDC BT /F1 10 Tf .866 -0.5 0.5 .866 10 100 Tm (A) Tj ET BT /F1 10 Tf .866 -0.5 0.5 .866 20 94.2265 Tm (B) Tj ET EMC",
	))}}
	text, err := page.ExtractTextTagged(d, textconfig.Options{})
	if err != nil || text != "AB" {
		t.Fatalf("tagged rotated-baseline text = %q, err = %v; want AB", text, err)
	}
}

func TestTaggedTextObjectStartsNewLineKeepsDirectionIndependentOfBaseline(t *testing.T) {
	tests := []struct {
		name     string
		space    coordinates.Space
		previous TextObject
		current  TextObject
	}{
		{
			name:     "page mirrored horizontal",
			space:    CoordinateSpacePage,
			previous: taggedLineTestText([2]float64{10, 100}, [2]float64{-5, 0}, geometry.Matrix{-10, 0, 0, 10, 0, 0}, geometry.Matrix{-1, 0, 0, 1, 0, 0}, false),
			current:  taggedLineTestText([2]float64{10, 80}, [2]float64{-5, 0}, geometry.Matrix{-10, 0, 0, 10, 0, 0}, geometry.Matrix{-1, 0, 0, 1, 0, 0}, false),
		},
		{
			name:     "screen mirrored horizontal",
			space:    CoordinateSpaceScreen,
			previous: taggedLineTestText([2]float64{10, 100}, [2]float64{-5, 0}, geometry.Matrix{-10, 0, 0, -10, 0, 0}, geometry.Matrix{-1, 0, 0, 1, 0, 0}, false),
			current:  taggedLineTestText([2]float64{10, 120}, [2]float64{-5, 0}, geometry.Matrix{-10, 0, 0, -10, 0, 0}, geometry.Matrix{-1, 0, 0, 1, 0, 0}, false),
		},
		{
			name:     "page reversed vertical",
			space:    CoordinateSpacePage,
			previous: taggedLineTestText([2]float64{100, 10}, [2]float64{0, 5}, geometry.Matrix{-10, 0, 0, 10, 0, 0}, geometry.Matrix{-1, 0, 0, 1, 0, 0}, true),
			current:  taggedLineTestText([2]float64{80, 10}, [2]float64{0, 5}, geometry.Matrix{-10, 0, 0, 10, 0, 0}, geometry.Matrix{-1, 0, 0, 1, 0, 0}, true),
		},
		{
			name:     "screen reversed vertical",
			space:    CoordinateSpaceScreen,
			previous: taggedLineTestText([2]float64{100, 10}, [2]float64{0, 5}, geometry.Matrix{-10, 0, 0, -10, 0, 0}, geometry.Matrix{-1, 0, 0, 1, 0, 0}, true),
			current:  taggedLineTestText([2]float64{80, 10}, [2]float64{0, 5}, geometry.Matrix{-10, 0, 0, -10, 0, 0}, geometry.Matrix{-1, 0, 0, 1, 0, 0}, true),
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if !taggedTextObjectStartsNewLine(&Document{space: test.space}, test.current, test.previous) {
				t.Fatal("same-direction line advance was not separated")
			}
		})
	}
}

func TestPageExtractTextTaggedConcatenatesReversedCharsWithinSection(t *testing.T) {
	d := &Document{}
	page := Page{dict: Dict{Name("Contents"): newStream(nil, []byte(
		"/ReversedChars << /MCID 1 >> BDC BT /F1 10 Tf -1 0 0 1 100 100 Tm (AB) Tj ET BT /F1 10 Tf -1 0 0 1 100 80 Tm (CD) Tj ET EMC",
	))}}
	text, err := page.ExtractTextTagged(d, textconfig.Options{})
	if err != nil || text != "BADC" {
		t.Fatalf("tagged ReversedChars multiline text = %q, err = %v; want BADC", text, err)
	}
}

func TestTaggedTextBaselineFallsBackForDegenerateValues(t *testing.T) {
	zero := taggedLineTestText([2]float64{}, [2]float64{}, geometry.Matrix{}, geometry.Matrix{}, false)
	nonfinite := taggedLineTestText([2]float64{}, [2]float64{math.NaN(), 0}, geometry.Matrix{math.Inf(1), 0}, geometry.Matrix{0, math.Inf(1)}, false)
	if _, _, ok := taggedTextBaseline(zero, nonfinite); ok {
		t.Fatal("degenerate baseline unexpectedly normalized")
	}
}

func TestTaggedTextObjectStartsNewLineUsesLocalCrossAxisExtentForRotatedText(t *testing.T) {
	previous := taggedLineTestText(
		[2]float64{0, 0}, [2]float64{50, -50},
		geometry.Matrix{7.0710678118654755, -7.0710678118654755, 7.0710678118654755, 7.0710678118654755},
		geometry.Matrix{0.7071067811865476, -0.7071067811865476, 0.7071067811865476, 0.7071067811865476}, false,
	)
	current := taggedLineTestText(
		[2]float64{-4.242640687119286, -4.242640687119286}, [2]float64{50, -50},
		geometry.Matrix{7.0710678118654755, -7.0710678118654755, 7.0710678118654755, 7.0710678118654755},
		geometry.Matrix{0.7071067811865476, -0.7071067811865476, 0.7071067811865476, 0.7071067811865476}, false,
	)
	previous.data = contentdata.NewTextBorrowed(contentdata.TextSpec{
		Origin: previous.Origin(), Displacement: previous.Displacement(), Matrix: previous.Matrix(), TextMatrix: previous.TextMatrix(),
		BBox: [4]float64{0, 0, 100, 100},
	})
	current.data = contentdata.NewTextBorrowed(contentdata.TextSpec{
		Origin: current.Origin(), Displacement: current.Displacement(), Matrix: current.Matrix(), TextMatrix: current.TextMatrix(),
		BBox: [4]float64{0, 0, 100, 100},
	})
	if !taggedTextObjectStartsNewLine(&Document{space: CoordinateSpacePage}, current, previous) {
		t.Fatal("rotated long-text line advance was hidden by the axis-aligned bounding box")
	}
}

func taggedLineTestText(origin, displacement [2]float64, matrix, textMatrix geometry.Matrix, vertical bool) TextObject {
	return newTestText(contentdata.TextSpec{
		Origin: origin, Displacement: displacement, Matrix: matrix, TextMatrix: textMatrix,
		BBox: [4]float64{0, 0, 10, 10}, Vertical: vertical,
	}, nil, nil)
}

func TestPageExtractTextTaggedUsesStructureElementActualText(t *testing.T) {
	tests := []struct {
		name       string
		actualText Object
		want       string
	}{
		{name: "replacement", actualText: String("replacement"), want: "replacement"},
		{name: "empty replacement", actualText: String(""), want: ""},
		{name: "malformed replacement", actualText: Number(123), want: "wrong"},
		{name: "unresolved replacement", actualText: Ref{Object: 99}, want: "wrong"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			d := structureActualTextTestDocument(test.actualText)
			page, err := d.PageByRef(Ref{Object: 9})
			if err != nil {
				t.Fatal(err)
			}
			text, err := page.ExtractTextTagged(d, textconfig.Options{})
			if err != nil || text != test.want {
				t.Fatalf("tagged structure ActualText = %q, err = %v; want %q", text, err, test.want)
			}
			structure, err := page.Structure(d)
			if err != nil {
				t.Fatal(err)
			}
			elements := structure.ByMCID(1)
			if len(elements) != 1 {
				t.Fatalf("structure elements for MCID 1 = %#v", elements)
			}
			text, err = elements[0].Text(d)
			if err != nil || text != test.want {
				t.Fatalf("structure element ActualText = %q, err = %v; want %q", text, err, test.want)
			}
		})
	}
}

func structureActualTextTestDocument(actualText Object) *Document {
	d := &Document{
		trailer: Dict{Name("Root"): Ref{Object: 1}},
		objects: map[Ref]Object{
			{Object: 1}: Dict{Name("Pages"): Ref{Object: 2}, Name("StructTreeRoot"): Ref{Object: 3}},
			{Object: 2}: Dict{Name("Type"): Name("Pages"), Name("Kids"): Array{Ref{Object: 9}}, Name("Count"): Number(1)},
			{Object: 3}: Dict{Name("ParentTree"): Dict{Name("Nums"): Array{
				Number(7), Array{Null{}, Dict{
					Name("Type"):       Name("StructElem"),
					Name("S"):          Name("Span"),
					Name("Pg"):         Ref{Object: 9},
					Name("K"):          Number(1),
					Name("ActualText"): actualText,
				}},
			}}},
			{Object: 9}: Dict{
				Name("Type"):          Name("Page"),
				Name("Parent"):        Ref{Object: 2},
				Name("StructParents"): Number(7),
				Name("Contents"): newStream(nil, []byte(
					"/Span << /MCID 1 >> BDC BT /F1 10 Tf (wrong) Tj ET EMC",
				)),
			},
		},
	}
	return d
}

func TestPageExtractTextTaggedUsesAncestorActualTextOnce(t *testing.T) {
	d := &Document{
		trailer: Dict{Name("Root"): Ref{Object: 1}},
		objects: map[Ref]Object{
			{Object: 1}: Dict{Name("Pages"): Ref{Object: 2}, Name("StructTreeRoot"): Ref{Object: 3}},
			{Object: 2}: Dict{Name("Type"): Name("Pages"), Name("Kids"): Array{Ref{Object: 9}}, Name("Count"): Number(1)},
			{Object: 3}: Dict{
				Name("Type"): Name("StructTreeRoot"),
				Name("K"):    Ref{Object: 6},
				Name("ParentTree"): Dict{Name("Nums"): Array{
					Number(7), Array{Ref{Object: 7}, Ref{Object: 8}},
				}},
			},
			{Object: 6}: Dict{
				Name("Type"):       Name("StructElem"),
				Name("S"):          Name("P"),
				Name("P"):          Ref{Object: 3},
				Name("Pg"):         Ref{Object: 9},
				Name("K"):          Array{Ref{Object: 7}, Ref{Object: 8}},
				Name("ActualText"): String("replacement"),
			},
			{Object: 7}: Dict{Name("Type"): Name("StructElem"), Name("S"): Name("Span"), Name("P"): Ref{Object: 6}, Name("Pg"): Ref{Object: 9}, Name("K"): Number(0)},
			{Object: 8}: Dict{Name("Type"): Name("StructElem"), Name("S"): Name("Span"), Name("P"): Ref{Object: 6}, Name("Pg"): Ref{Object: 9}, Name("K"): Number(1)},
			{Object: 9}: Dict{
				Name("Type"):          Name("Page"),
				Name("Parent"):        Ref{Object: 2},
				Name("StructParents"): Number(7),
				Name("Contents"): newStream(nil, []byte(
					"/Span << /MCID 0 >> BDC BT /F1 10 Tf (wrong one) Tj ET EMC /Span << /MCID 1 >> BDC BT /F1 10 Tf (wrong two) Tj ET EMC",
				)),
			},
		},
	}
	page, err := d.PageByRef(Ref{Object: 9})
	if err != nil {
		t.Fatal(err)
	}
	text, err := page.ExtractTextTagged(d, textconfig.Options{})
	if err != nil || text != "replacement" {
		t.Fatalf("ancestor structure ActualText = %q, err = %v; want one replacement", text, err)
	}
}

func TestStructureActualTextParentDoesNotMaterializeChildren(t *testing.T) {
	d := &Document{objects: map[Ref]Object{
		{Object: 6}: Dict{
			Name("Type"):       Name("StructElem"),
			Name("P"):          Ref{Object: 3},
			Name("K"):          Array{Number(0), Number(1), Number(2)},
			Name("ActualText"): String("replacement"),
		},
	}}
	child := newStructElementValue(structuredata.ElementSpec{Parent: Ref{Object: 6}, HasParent: true})

	parent := structureActualTextParent(d, child)
	if parent == nil {
		t.Fatal("structure ActualText parent was not resolved")
	}
	if actual, present := parent.actualTextValue(); actual != "replacement" || !present {
		t.Fatalf("parent ActualText = %q, present = %v; want replacement, true", actual, present)
	}
	if len(parent.contents) != 0 || parent.childStart != nil || parent.lazyState != nil {
		t.Fatalf("metadata-only parent materialized structure children: %#v", parent)
	}
	if len(parent.DictCopy()) != 0 {
		t.Fatalf("metadata-only parent retained the complete structure dictionary: %#v", parent.DictCopy())
	}
}

func TestStructureActualTextParentResolverCachesSharedAncestorMetadata(t *testing.T) {
	d := &Document{objects: map[Ref]Object{
		{Object: 6}: Dict{Name("Type"): Name("StructElem"), Name("ActualText"): String("replacement")},
	}}
	child := newStructElementValue(structuredata.ElementSpec{Parent: Ref{Object: 6}, HasParent: true})
	resolver := newStructureActualTextParentResolver(d)

	first := resolver.parent(child)
	second := resolver.parent(child)
	if first == nil || second == nil {
		t.Fatal("structure ActualText ancestor was not resolved")
	}
	if first != second {
		t.Fatal("shared structure ActualText ancestor metadata was resolved more than once")
	}
}

func TestPageExtractTextTaggedSeparatesPageAndFormParentTreeContexts(t *testing.T) {
	formOne := newStream(Dict{
		Name("Subtype"):       Name("Form"),
		Name("StructParents"): Number(8),
	}, []byte("/Span << /MCID 0 >> BDC BT (wrong form one) Tj ET EMC"))
	formTwo := newStream(Dict{
		Name("Subtype"):       Name("Form"),
		Name("StructParents"): Number(9),
	}, []byte("/Span << /MCID 0 >> BDC BT (wrong form two) Tj ET EMC"))
	d := &Document{
		trailer: Dict{Name("Root"): Ref{Object: 1}},
		objects: map[Ref]Object{
			{Object: 1}: Dict{Name("Pages"): Ref{Object: 2}, Name("StructTreeRoot"): Ref{Object: 3}},
			{Object: 2}: Dict{Name("Type"): Name("Pages"), Name("Kids"): Array{Ref{Object: 10}}, Name("Count"): Number(1)},
			{Object: 3}: Dict{Name("ParentTree"): Dict{Name("Nums"): Array{
				Number(7), Array{Dict{Name("Type"): Name("StructElem"), Name("S"): Name("Span"), Name("ActualText"): String("PAGE")}},
				Number(8), Array{Dict{Name("Type"): Name("StructElem"), Name("S"): Name("Span"), Name("ActualText"): String("FORM1")}},
				Number(9), Array{Dict{Name("Type"): Name("StructElem"), Name("S"): Name("Span"), Name("ActualText"): String("FORM2")}},
			}}},
			{Object: 10}: Dict{
				Name("Type"):          Name("Page"),
				Name("Parent"):        Ref{Object: 2},
				Name("StructParents"): Number(7),
				Name("Contents"): newStream(nil, []byte(
					"/Span << /MCID 0 >> BDC BT (wrong page) Tj ET EMC /Fm1 Do /Fm2 Do",
				)),
				Name("Resources"): Dict{Name("XObject"): Dict{Name("Fm1"): formOne, Name("Fm2"): formTwo}},
			},
		},
	}
	page, err := d.PageByRef(Ref{Object: 10})
	if err != nil {
		t.Fatal(err)
	}
	text, err := page.ExtractTextTagged(d, textconfig.Options{})
	if err != nil || text != "PAGEFORM1FORM2" {
		t.Fatalf("page and Form structure ActualText = %q, err = %v; want PAGEFORM1FORM2", text, err)
	}
}

func TestPageExtractTextTaggedAppliesBBoxToActualTextSections(t *testing.T) {
	d := &Document{}
	page := Page{dict: Dict{Name("Contents"): newStream(nil, []byte(
		"/Span << /MCID 1 /ActualText (AB) >> BDC BT /F1 10 Tf 1 0 0 1 10 100 Tm (X) Tj ET EMC",
	))}}
	text, err := page.ExtractTextTagged(d, textconfig.Options{BBox: &[4]float64{500, 500, 600, 600}})
	if err != nil || text != "" {
		t.Fatalf("tagged ActualText outside bbox = %q, err = %v; want empty", text, err)
	}
}

func TestPageExtractTextTaggedGroupsMarkedSectionObjects(t *testing.T) {
	d := &Document{}
	page := Page{dict: Dict{Name("Contents"): newStream(nil, []byte(
		"/Artifact BMC BT /F1 10 Tf (X) Tj ET EMC /Span << /MCID 1 /ActualText (AB) >> BDC BT /F1 10 Tf (X) Tj (Y) Tj ET EMC",
	))}}
	text, err := page.ExtractTextTagged(d, textconfig.Options{})
	if err != nil || text != "AB" {
		t.Fatalf("grouped ActualText = %q, err = %v", text, err)
	}
}

func TestPageExtractTextTaggedConcatenatesLinesWithinMarkedSection(t *testing.T) {
	d := &Document{}
	page := Page{dict: Dict{Name("Contents"): newStream(nil, []byte(
		"/P << /MCID 0 >> BDC BT /F1 10 Tf 1 0 0 1 10 100 Tm (first) Tj 1 0 0 1 10 80 Tm (second) Tj ET EMC",
	))}}
	text, err := page.ExtractTextTagged(d, textconfig.Options{})
	if err != nil || text != "firstsecond" {
		t.Fatalf("tagged multiline section = %q, err = %v; want firstsecond", text, err)
	}
}

func TestPageExtractTextTaggedDoesNotSeparateBaselineShiftWithinMarkedSection(t *testing.T) {
	d := &Document{}
	page := Page{dict: Dict{Name("Contents"): newStream(nil, []byte(
		"/P << /MCID 0 >> BDC BT /F1 10 Tf 1 0 0 1 10 100 Tm (H) Tj 1 0 0 1 20 98 Tm (2) Tj ET EMC",
	))}}
	text, err := page.ExtractTextTagged(d, textconfig.Options{})
	if err != nil || text != "H2" {
		t.Fatalf("tagged baseline shift = %q, err = %v; want H2", text, err)
	}
}

func TestPageExtractTextTaggedJoinsSoftHyphenContinuationWithinMarkedSection(t *testing.T) {
	d := &Document{}
	page := Page{dict: Dict{Name("Contents"): newStream(nil, []byte(
		"/P << /MCID 0 >> BDC BT /F1 10 Tf 1 0 0 1 10 100 Tm (hyphen\\255) Tj 1 0 0 1 10 80 Tm (ation) Tj ET EMC",
	))}}
	text, err := page.ExtractTextTagged(d, textconfig.Options{})
	if err != nil || text != "hyphenation" {
		t.Fatalf("tagged soft-hyphen continuation = %q, err = %v; want hyphenation", text, err)
	}
}

func TestPageExtractTextTaggedDoesNotMergeDistinctEquivalentSections(t *testing.T) {
	d := &Document{}
	page := Page{dict: Dict{Name("Contents"): newStream(nil, []byte(
		"/Span << /MCID 1 /ActualText (X) >> BDC BT /F1 10 Tf 1 0 0 1 10 100 Tm (A) Tj ET EMC /Span << /MCID 1 /ActualText (X) >> BDC BT /F1 10 Tf 1 0 0 1 10 80 Tm (B) Tj ET EMC",
	))}}
	text, err := page.ExtractTextTagged(d, textconfig.Options{})
	if err != nil || text != "XX" {
		t.Fatalf("tagged text for equivalent sections = %q, err = %v; want XX", text, err)
	}
}

func TestPageExtractTextTaggedUpdatesLogicalLineStateAfterEachSection(t *testing.T) {
	d := &Document{}
	page := Page{dict: Dict{Name("Contents"): newStream(nil, []byte(
		"/Span << /MCID 1 >> BDC BT /F1 10 Tf 1 0 0 1 10 100 Tm (A) Tj ET EMC /Span << /MCID 2 >> BDC BT /F1 10 Tf 1 0 0 1 10 80 Tm (B) Tj ET EMC",
	))}}
	text, err := page.ExtractTextTagged(d, textconfig.Options{})
	if err != nil || text != "A\nB" {
		t.Fatalf("tagged text line state = %q, err = %v; want A\\nB", text, err)
	}
}

func TestPageExtractTextTaggedArtifactUpdatesLogicalLineState(t *testing.T) {
	d := &Document{}
	page := Page{dict: Dict{Name("Contents"): newStream(nil, []byte(
		"/Span << /MCID 1 >> BDC BT /F1 10 Tf 1 0 0 1 10 100 Tm (A) Tj ET EMC /Artifact BMC BT /F1 10 Tf 1 0 0 1 10 200 Tm (X) Tj ET EMC /Span << /MCID 2 >> BDC BT /F1 10 Tf 1 0 0 1 20 100 Tm (B) Tj ET EMC",
	))}}
	text, err := page.ExtractTextTagged(d, textconfig.Options{})
	if err != nil || text != "A\nB" {
		t.Fatalf("tagged text around artifact = %q, err = %v; want A\\nB", text, err)
	}
}

func TestPageExtractTextTaggedIncludesSectionsWithoutMCID(t *testing.T) {
	d := &Document{}
	page := Page{dict: Dict{Name("Contents"): newStream(nil, []byte(
		"/Span BMC BT /F1 10 Tf (A) Tj (B) Tj ET EMC",
	))}}
	text, err := page.ExtractTextTagged(d, textconfig.Options{})
	if err != nil || text != "AB" {
		t.Fatalf("untagged-ID tagged section = %q, err = %v", text, err)
	}
}

func TestWrapTaggedTextUsesPlayaDefaultWidth(t *testing.T) {
	wrapped := wrapTaggedText(string(bytes.Repeat([]byte{'A'}, 71)))
	if wrapped != string(bytes.Repeat([]byte{'A'}, 70))+"\nA" {
		t.Fatalf("wrapped tagged text = %q", wrapped)
	}
	if got := wrapTaggedText(" "); got != "" {
		t.Fatalf("whitespace-only tagged text = %q, want empty", got)
	}
	if got := wrapTaggedText("A "); got != "A" {
		t.Fatalf("trailing whitespace in tagged text = %q, want A", got)
	}
	longWordAfterFullLine := strings.Repeat("x", 69) + " " + strings.Repeat("y", 71)
	longWordAfterFullLineWant := strings.Repeat("x", 69) + " \n" + strings.Repeat("y", 70) + "\ny"
	if got := wrapTaggedText(longWordAfterFullLine); got != longWordAfterFullLineWant {
		t.Fatalf("separator before long word = %q, want %q", got, longWordAfterFullLineWant)
	}
	value := "目  录" + strings.Repeat(".", 70) + " 2"
	want := "目  录" + strings.Repeat(".", 66) + "\n.... 2"
	if got := wrapTaggedText(value); got != want {
		t.Fatalf("long punctuation run = %q, want %q", got, want)
	}
	hyphenated := "有机-" + strings.Repeat("无", 70)
	hyphenatedWant := "有机-\n" + strings.Repeat("无", 70)
	if got := wrapTaggedText(hyphenated); got != hyphenatedWant {
		t.Fatalf("hyphenated word = %q, want %q", got, hyphenatedWant)
	}
	numericHyphen := strings.Repeat("前", 68) + "1-6后"
	numericHyphenWant := strings.Repeat("前", 68) + "1-\n6后"
	if got := wrapTaggedText(numericHyphen); got != numericHyphenWant {
		t.Fatalf("numeric hyphenated word = %q, want %q", got, numericHyphenWant)
	}
	shortHyphen := strings.Repeat("前", 65) + " 0701-6689877"
	shortHyphenWant := strings.Repeat("前", 65) + "\n0701-6689877"
	if got := wrapTaggedText(shortHyphen); got != shortHyphenWant {
		t.Fatalf("short hyphenated word = %q, want %q", got, shortHyphenWant)
	}
	shortLetterHyphen := strings.Repeat("前", 65) + " Lac-Tunis"
	shortLetterHyphenWant := strings.Repeat("前", 65) + " Lac-\nTunis"
	if got := wrapTaggedText(shortLetterHyphen); got != shortLetterHyphenWant {
		t.Fatalf("short letter hyphenated word = %q, want %q", got, shortLetterHyphenWant)
	}
	if got := wrapTaggedText("Please advise if we are to combine FAF-B2-08 and                 FAF-B1-09 as per the remarks"); got != "Please advise if we are to combine FAF-B2-08 and\nFAF-B1-09 as per the remarks" {
		t.Fatalf("mixed alphanumeric hyphenated word = %q", got)
	}
	nonBreaking := strings.Repeat("前", 63) + "\u00a0and\u00a0shall"
	nonBreakingWant := strings.Repeat("前", 63) + "\u00a0and\u00a0sh\nall"
	if got := wrapTaggedText(nonBreaking); got != nonBreakingWant {
		t.Fatalf("non-breaking space word = %q, want %q", got, nonBreakingWant)
	}
	negative := "-" + strings.Repeat("1", 70)
	negativeWant := "-" + strings.Repeat("1", 69) + "\n1"
	if got := wrapTaggedText(negative); got != negativeWant {
		t.Fatalf("negative number = %q, want %q", got, negativeWant)
	}
}

func TestPageContentReusesDecodedIndirectStreams(t *testing.T) {
	d := &Document{
		objects: map[Ref]Object{
			{Object: 7}: newStream(nil, []byte("1 0 m")),
		},
	}
	p := Page{dict: Dict{Name("Contents"): Array{Ref{Object: 7}, Ref{Object: 7}}}}
	var count int
	for _, err := range p.Contents(d) {
		if err != nil {
			t.Fatal(err)
		}
		count++
	}
	if count != 2 || len(d.decodedStreamCache) != 1 {
		t.Fatalf("content streams=%d cache=%d", count, len(d.decodedStreamCache))
	}
}

func TestFormExpansionReusesDecodedIndirectStreams(t *testing.T) {
	d := &Document{objects: map[Ref]Object{
		{Object: 7}: newStream(Dict{Name("Subtype"): Name("Form")}, []byte("1 0 m")),
	}}
	p := Page{dict: Dict{
		Name("Contents"):  newStream(nil, []byte("/Fm Do /Fm Do")),
		Name("Resources"): Dict{Name("XObject"): Dict{Name("Fm"): Ref{Object: 7}}},
	}}
	ops, err := d.PageContentOps(p)
	if err != nil {
		t.Fatal(err)
	}
	if len(ops) == 0 || len(d.decodedStreamCache) != 1 {
		t.Fatalf("expanded ops=%d decoded stream cache=%d", len(ops), len(d.decodedStreamCache))
	}
}

func TestPageContentOpsReusesCachedFormResources(t *testing.T) {
	form := newStream(Dict{Name("Subtype"): Name("Form"), Name("Resources"): Ref{Object: 2}}, []byte("q Q"))
	d := &Document{objects: map[Ref]Object{
		{Object: 1}: Dict{Name("Fm"): Ref{Object: 7}},
		{Object: 2}: Dict{Name("Marker"): String("cached")},
		{Object: 7}: form,
	}}
	x := testXObjectRefStream(Ref{Object: 7}, form)
	if _, err := x.ResourcesWithError(d); err != nil {
		t.Fatal(err)
	}
	d.objects[Ref{Object: 7}] = newStream(Dict{Name("Subtype"): Name("Form"), Name("Resources"): Ref{Object: 99}}, []byte("q Q"))
	page := Page{dict: Dict{
		Name("Contents"):  newStream(nil, []byte("/Fm Do")),
		Name("Resources"): Dict{Name("XObject"): Ref{Object: 1}},
	}}
	ops, err := d.PageContentOps(page)
	if err != nil {
		t.Fatal(err)
	}
	for _, op := range ops {
		if marker, ok := op.resources[Name("Marker")].(String); ok && string(marker) == "cached" {
			return
		}
	}
	t.Fatalf("expanded Form operations did not retain cached resources: %#v", ops)
}

func TestPageContentOpsAcceptsPropertyStreams(t *testing.T) {
	d := &Document{}
	p := Page{dict: Dict{
		Name("Contents"):  newStream(nil, []byte("/Span /P1 DP")),
		Name("Resources"): Dict{Name("Properties"): Dict{Name("P1"): newStream(Dict{Name("MCID"): Number(1)}, nil)}},
	}}
	if _, err := d.PageContentOps(p); err != nil {
		t.Fatalf("property stream rejected: %v", err)
	}
}

func TestPageContentOpsReusesValidatedPageResources(t *testing.T) {
	d := &Document{objects: map[Ref]Object{
		{Object: 1}: Dict{Name("XObject"): Dict{}},
	}}
	p := Page{ref: Ref{Object: 7}, dict: Dict{
		Name("Contents"):  newStream(nil, []byte("q Q")),
		Name("Resources"): Ref{Object: 1},
	}}
	if _, err := p.ResourcesWithError(d); err != nil {
		t.Fatal(err)
	}
	d.objects[Ref{Object: 1}] = Ref{Object: 99}
	if _, err := d.PageContentOps(p); err != nil {
		t.Fatalf("PageContentOps did not reuse validated resources: %v", err)
	}
}

func TestPageContentOpsReportsUnresolvedResourceReferences(t *testing.T) {
	tests := []struct {
		name string
		page Page
		want string
	}{
		{name: "resources", page: Page{dict: Dict{Name("Contents"): newStream(nil, []byte("q Q")), Name("Resources"): Ref{Object: 99}}}, want: "playa: Resources could not be resolved"},
		{name: "font", page: Page{dict: Dict{Name("Contents"): newStream(nil, []byte("q Q")), Name("Resources"): Dict{Name("Font"): Ref{Object: 99}}}}, want: "playa: Font resources could not be resolved"},
		{name: "xobject entry", page: Page{dict: Dict{Name("Contents"): newStream(nil, []byte("q Q")), Name("Resources"): Dict{Name("XObject"): Dict{Name("Fm"): Ref{Object: 99}}}}}, want: "playa: XObject resource \"Fm\" could not be resolved"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := (&Document{}).PageContentOps(test.page); err == nil || err.Error() != test.want {
				t.Fatalf("error = %v, want %q", err, test.want)
			}
		})
	}
}

func TestPageInterpSkipsUnresolvedContents(t *testing.T) {
	d := &Document{objects: map[Ref]Object{}}
	p := Page{dict: Dict{Name("Contents"): Ref{Object: 99}}}
	for object, err := range p.Interp(d, contentconfig.Options{Filter: FilterText}) {
		t.Fatalf("unresolved Interp contents = %#v, err=%v, want no items", object, err)
	}
}

func TestPageContentDoesNotCacheSkippedUnresolvedContentsRoot(t *testing.T) {
	d := &Document{objects: map[Ref]Object{}}
	p := Page{ref: Ref{Object: 7}, dict: Dict{Name("Contents"): Ref{Object: 99}}}
	first, firstErr := p.Content(d)
	second, secondErr := p.Content(d)
	if firstErr != nil || secondErr != nil || first != nil || second != nil {
		t.Fatalf("Content roots = %q/%q, errors %v/%v, want skipped", first, second, firstErr, secondErr)
	}
	for object, err := range p.Interp(d, contentconfig.Options{}) {
		t.Fatalf("iterator unresolved root = %#v, %v, want no items", object, err)
	}
	for stream, err := range p.Streams(d) {
		t.Fatalf("stream unresolved root = %#v, %v, want no items", stream, err)
	}
	for token, err := range p.Tokens(d) {
		t.Fatalf("token unresolved root = %#v, %v, want no items", token, err)
	}
	if len(d.contentRootErrors) != 0 {
		t.Fatalf("skipped Contents root cached errors: %#v", d.contentRootErrors)
	}
}

func TestPageContentAPIsSkipMalformedContentsReferences(t *testing.T) {
	ref := Ref{Object: 99}
	d := &Document{
		data:    []byte("%PDF-1.4\n"),
		objects: map[Ref]Object{},
		xrefs:   map[Ref]xrefEntry{ref: {offset: 1 << 20}},
	}
	p := Page{dict: Dict{Name("Contents"): ref}}
	if content, err := p.Content(d); err != nil || content != nil {
		t.Fatalf("Content = %q, err=%v, want skipped malformed reference", content, err)
	}
	for stream, err := range p.Streams(d) {
		t.Fatalf("Streams yielded %#v, err=%v, want no items", stream, err)
	}
	for token, err := range p.Tokens(d) {
		t.Fatalf("Tokens yielded %#v, err=%v, want no items", token, err)
	}
	for op, err := range p.Contents(d) {
		t.Fatalf("Contents yielded %#v, err=%v, want no items", op, err)
	}
}

func TestPageContentReferencesUseNewestObjectGeneration(t *testing.T) {
	d := &Document{objects: map[Ref]Object{{Object: 4}: newStream(nil, []byte("q"))}}
	p := Page{dict: Dict{Name("Contents"): Ref{Object: 4, Generation: 99}}}
	content, err := p.Content(d)
	if err != nil || string(content) != "q\n" {
		t.Fatalf("Content = %q, err=%v", content, err)
	}
	var streams int
	for _, err := range p.Streams(d) {
		if err != nil {
			t.Fatal(err)
		}
		streams++
	}
	if streams != 1 {
		t.Fatalf("Streams count = %d, want 1", streams)
	}
	tokens, err := p.CollectTokens(d)
	if err != nil || len(tokens) != 1 || tokens[0].Text() != "q" {
		t.Fatalf("Tokens = %#v, err=%v", tokens, err)
	}
	ops, err := p.ContentOps(d)
	if err != nil || len(ops) != 1 || ops[0].Operator() != "q" {
		t.Fatalf("Contents = %#v, err=%v", ops, err)
	}
}

func TestPageContentPropagatesObjectStreamDecodeErrors(t *testing.T) {
	data := []byte("\n10 0 obj\n<< /Type /ObjStm /N 1 /First 5 /Length 5 /Filter /UnknownFilter >>\nstream\n99 0 \nendstream\nendobj")
	ref := Ref{Object: 99}
	d := &Document{
		data:               data,
		objects:            map[Ref]Object{},
		expandedObjStms:    map[int]bool{},
		xrefs:              map[Ref]xrefEntry{ref: newCompressedXRefEntry(10, 0, false, true), {Object: 10}: {offset: 1}},
		objectStreamErrors: map[int]error{},
	}
	p := Page{dict: Dict{Name("Contents"): ref}}
	if _, err := p.Content(d); err == nil {
		t.Fatal("Content skipped an object-stream filter error")
	}
}

func TestXObjectTokensTreatUnterminatedStringAsEOF(t *testing.T) {
	x := testXObjectStream(newStream(nil, []byte("q (")))
	tokens, err := x.CollectTokens(&Document{})
	if err != nil {
		t.Fatal(err)
	}
	if len(tokens) != 1 || tokens[0].Kind() != TokenKeyword || tokens[0].Text() != "q" {
		t.Fatalf("XObject tokens = %#v, want q", tokens)
	}
}

func TestPageContentsContinueAfterUnterminatedInlineParameters(t *testing.T) {
	p := Page{dict: Dict{Name("Contents"): Array{
		newStream(nil, []byte("q BI /A (")),
		newStream(nil, []byte("Q")),
	}}}
	var operators []string
	for op, err := range p.Contents(&Document{}) {
		if err != nil {
			t.Fatal(err)
		}
		operators = append(operators, op.Operator())
	}
	if !reflect.DeepEqual(operators, []string{"q", "Q"}) {
		t.Fatalf("operators = %#v, want q/Q across the recovered stream boundary", operators)
	}
}

func TestPageInterpReportsUnresolvedResources(t *testing.T) {
	d := &Document{objects: map[Ref]Object{}}
	p := Page{dict: Dict{
		Name("Contents"):  newStream(nil, []byte("q Q")),
		Name("Resources"): Ref{Object: 99},
	}}
	for object, err := range p.Interp(d, contentconfig.Options{Filter: FilterAll}) {
		if err == nil || object.Kind() != "" || err.Error() != "playa: Resources could not be resolved" {
			t.Fatalf("unresolved Interp resources = %#v, err=%v", object, err)
		}
		return
	}
	t.Fatal("unresolved Interp resources produced no error")
}

func TestPageTextsReportsUnresolvedProperties(t *testing.T) {
	d := &Document{objects: map[Ref]Object{}}
	p := Page{dict: Dict{
		Name("Contents"):  newStream(nil, []byte("BT /Span /P1 BDC (text) Tj EMC ET")),
		Name("Resources"): Dict{Name("Properties"): Ref{Object: 99}},
	}}
	for _, err := range p.Texts(d) {
		if err == nil || err.Error() != "playa: Properties resources could not be resolved" {
			t.Fatalf("unresolved Properties in text = %v", err)
		}
		return
	}
	t.Fatal("unresolved Properties produced no text error")
}

func TestPageTextsReportsUnresolvedExtGState(t *testing.T) {
	d := &Document{}
	p := Page{dict: Dict{
		Name("Contents"):  newStream(nil, []byte("/GS gs BT (text) Tj ET")),
		Name("Resources"): Dict{Name("ExtGState"): Ref{Object: 99}},
	}}
	for _, err := range p.Texts(d) {
		if err == nil || err.Error() != "playa: ExtGState resources could not be resolved" {
			t.Fatalf("unresolved ExtGState in text = %v", err)
		}
		return
	}
	t.Fatal("unresolved ExtGState produced no text error")
}

func TestPageTextsReportsUnresolvedColorSpace(t *testing.T) {
	d := &Document{}
	p := Page{dict: Dict{
		Name("Contents"):  newStream(nil, []byte("/CS1 cs BT (text) Tj ET")),
		Name("Resources"): Dict{Name("ColorSpace"): Dict{Name("CS1"): Ref{Object: 99}}},
	}}
	for _, err := range p.Texts(d) {
		if err == nil || err.Error() != "playa: ColorSpace resource \"CS1\" could not be resolved" {
			t.Fatalf("unresolved ColorSpace in text = %v", err)
		}
		return
	}
	t.Fatal("unresolved ColorSpace produced no text error")
}

func TestPageTextsReportsUnresolvedShading(t *testing.T) {
	d := &Document{}
	p := Page{dict: Dict{
		Name("Contents"):  newStream(nil, []byte("/Sh sh BT (text) Tj ET")),
		Name("Resources"): Dict{Name("Shading"): Dict{Name("Sh"): Ref{Object: 99}}},
	}}
	for _, err := range p.Texts(d) {
		if err == nil || err.Error() != "playa: shading resource \"Sh\" could not be resolved" {
			t.Fatalf("unresolved Shading in text = %v", err)
		}
		return
	}
	t.Fatal("unresolved Shading produced no text error")
}

func TestContentIteratorReportsUnresolvedFormReferences(t *testing.T) {
	tests := []struct {
		name      string
		resources Dict
		want      string
	}{
		{name: "xobject", resources: Dict{Name("XObject"): Ref{Object: 99}}, want: "playa: XObject resources could not be resolved"},
		{name: "entry", resources: Dict{Name("XObject"): Dict{Name("Fm"): Ref{Object: 99}}}, want: "playa: XObject resource \"Fm\" could not be resolved"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			it := newContentOpIterator(&Document{}, Page{}, true)
			_, _, err := it.formFrame(contentFrame{ctx: Dict{Name("Resources"): test.resources}}, newContentOpBorrowed("Do", []Object{Name("Fm")}, 0))
			if err == nil || err.Error() != test.want {
				t.Fatalf("error = %v, want %q", err, test.want)
			}
		})
	}
}

func TestPageFlattenReportsUnresolvedFormResources(t *testing.T) {
	d := &Document{}
	form := newStream(Dict{
		Name("Subtype"):   Name("Form"),
		Name("Resources"): Ref{Object: 99},
	}, []byte("BT (form) Tj ET"))
	p := Page{dict: Dict{
		Name("Contents"):  newStream(nil, []byte("/Fm Do")),
		Name("Resources"): Dict{Name("XObject"): Dict{Name("Fm"): form}},
	}}
	for object, err := range p.Flatten(d, contentconfig.Options{Filter: FilterText}) {
		if err == nil || object.Kind() != "" {
			t.Fatalf("unresolved Form resources = %#v, err=%v", object, err)
		}
		return
	}
	t.Fatal("unresolved Form resources produced no error")
}

func TestPageContentsSequenceIsRepeatableAndRecoversLaterEOF(t *testing.T) {
	d := &Document{}
	p := Page{dict: Dict{Name("Contents"): Array{
		newStream(nil, []byte("1 0 m")),
		newStream(nil, []byte("(")),
	}}}
	first := 0
	for op, err := range p.Contents(d) {
		if err != nil {
			t.Fatal(err)
		}
		if op.operatorValue() != "m" {
			t.Fatalf("first content op = %#v", op)
		}
		first++
		break
	}
	second, deferredErr := 0, false
	for op, err := range p.Contents(d) {
		if err != nil {
			deferredErr = true
			break
		}
		if op.operatorValue() != "m" {
			t.Fatalf("repeated content op = %#v", op)
		}
		second++
	}
	if first != 1 || second != 1 || deferredErr {
		t.Fatalf("content sequence = first %d, second %d, deferred error %v", first, second, deferredErr)
	}
}

func TestPageContentsParsesTopLevelProcedureOperand(t *testing.T) {
	d := &Document{}
	p := Page{dict: Dict{Name("Contents"): newStream(nil, []byte("{1 null {2}} q"))}}
	var got []ContentOp
	for op, err := range p.Contents(d) {
		if err != nil {
			t.Fatal(err)
		}
		got = append(got, op.Finalize())
	}
	want := Array{Number(1), Null{}, Array{Number(2)}}
	if len(got) != 1 || got[0].Operator() != "q" || !reflect.DeepEqual(got[0].OperandsCopy(), []Object{want}) {
		t.Fatalf("page procedure content = %#v, want q with %#v", got, want)
	}
}

func TestPageContentAPIsSkipNonStreamAndDanglingContentsItems(t *testing.T) {
	d := &Document{objects: map[Ref]Object{}}
	p := Page{dict: Dict{Name("Contents"): Array{
		newStream(nil, []byte("q")), Null{}, Number(42), Ref{Object: 99}, newStream(nil, []byte("Q")),
	}}}
	content, err := p.Content(d)
	if err != nil || string(content) != "q\nQ\n" {
		t.Fatalf("Content() = %q, %v, want q/Q streams", content, err)
	}
	var streams []string
	for stream, err := range p.Streams(d) {
		if err != nil {
			t.Fatal(err)
		}
		streams = append(streams, string(stream.DataBorrowed()))
	}
	if !reflect.DeepEqual(streams, []string{"q", "Q"}) {
		t.Fatalf("Streams() = %#v", streams)
	}
	var tokens []string
	for token, err := range p.Tokens(d) {
		if err != nil {
			t.Fatal(err)
		}
		tokens = append(tokens, token.Text())
	}
	if !reflect.DeepEqual(tokens, []string{"q", "Q"}) {
		t.Fatalf("Tokens() = %#v", tokens)
	}
	var operators []string
	for op, err := range p.Contents(d) {
		if err != nil {
			t.Fatal(err)
		}
		operators = append(operators, op.Operator())
	}
	if !reflect.DeepEqual(operators, []string{"q", "Q"}) {
		t.Fatalf("Contents() = %#v", operators)
	}
	first := ""
	for stream, err := range p.Streams(d) {
		if err != nil {
			t.Fatal(err)
		}
		first = string(stream.DataBorrowed())
		break
	}
	if first != "q" {
		t.Fatalf("early Streams() = %q, want q", first)
	}
}

func TestLazyContentsIgnoreMalformedFormMatrix(t *testing.T) {
	d := &Document{objects: map[Ref]Object{}}
	p := Page{dict: Dict{
		Name("Contents"):  newStream(nil, []byte("/Fm Do")),
		Name("Resources"): Dict{Name("XObject"): Dict{Name("Fm"): Ref{Object: 1}}},
	}}
	d.objects[Ref{Object: 1}] = newStream(Dict{
		Name("Subtype"):   Name("Form"),
		Name("Matrix"):    Array{Number(2), Number(0), Number(0), Number(2), Number(10), Number(20), Number(30)},
		Name("Resources"): Dict{},
	}, []byte("q Q"))
	for op, err := range p.Contents(d) {
		if err != nil {
			t.Fatal(err)
		}
		if op.operatorValue() == "cm" {
			t.Fatalf("lazy content iterator emitted malformed form matrix: %#v", op)
		}
	}
}

func TestPageSemanticSequencesRejectNilDocument(t *testing.T) {
	page := Page{}
	for _, sequence := range []iter.Seq2[PageStructureEntry, error]{page.StructureSeq(nil)} {
		for _, err := range sequence {
			if err != errNilDocument {
				t.Fatalf("structure sequence error = %v", err)
			}
			break
		}
	}
	for _, sequence := range []iter.Seq2[MarkedContent, error]{page.MarkedContent(nil)} {
		for _, err := range sequence {
			if err != errNilDocument {
				t.Fatalf("marked-content sequence error = %v", err)
			}
			break
		}
	}
	for _, sequence := range []iter.Seq2[TagObject, error]{page.Tags(nil)} {
		for _, err := range sequence {
			if err != errNilDocument {
				t.Fatalf("tag sequence error = %v", err)
			}
			break
		}
	}
	if _, err := page.MarkedContentIndex(nil); err != errNilDocument {
		t.Fatalf("marked-content index error = %v", err)
	}
}

func TestPageTagsSequencePreservesOrderAndStopsEarly(t *testing.T) {
	d := &Document{}
	page := Page{dict: Dict{Name("Contents"): newStream(nil, []byte(
		"/A MP /B << /MCID 2 >> DP /C MP",
	))}}
	var first []string
	for tag, err := range page.Tags(d) {
		if err != nil {
			t.Fatal(err)
		}
		first = append(first, tag.Name())
		break
	}
	var second []string
	for tag, err := range page.Tags(d) {
		if err != nil {
			t.Fatal(err)
		}
		second = append(second, tag.Name())
	}
	if len(first) != 1 || len(second) != 3 || second[0] != "A" || second[2] != "C" {
		t.Fatalf("tags = %#v, %#v", first, second)
	}
}

func TestNestedModelSequencesAreRepeatableAndStopEarly(t *testing.T) {
	node := OutlineNode{children: []OutlineNode{
		newOutlineValue(documentdata.OutlineNodeSpec{Title: "a"}),
		newOutlineValue(documentdata.OutlineNodeSpec{Title: "b"}),
	}}
	var first []string
	for child, err := range node.ChildrenSeq() {
		if err != nil {
			t.Fatal(err)
		}
		first = append(first, child.Title())
		break
	}
	var second []string
	for child, err := range node.ChildrenSeq() {
		if err != nil {
			t.Fatal(err)
		}
		second = append(second, child.Title())
	}
	if len(first) != 1 || len(second) != 2 || second[1] != "b" {
		t.Fatalf("children = %#v, %#v", first, second)
	}
}

func TestPageStreamsAndTokensPreserveContentOrder(t *testing.T) {
	d := &Document{}
	page := Page{dict: Dict{Name("Contents"): Array{
		newStream(nil, []byte("/F1")),
		newStream(nil, []byte("12 Tf")),
	}}}

	var streams []Stream
	for stream, err := range page.Streams(d) {
		if err != nil {
			t.Fatal(err)
		}
		streams = append(streams, stream)
	}
	if len(streams) != 2 || string(streams[0].DataBorrowed()) != "/F1" || string(streams[1].DataBorrowed()) != "12 Tf" {
		t.Fatalf("streams = %#v", streams)
	}

	var tokens []Token
	for token, err := range page.Tokens(d) {
		if err != nil {
			t.Fatal(err)
		}
		tokens = append(tokens, token)
	}
	if len(tokens) != 3 || tokens[0].Kind() != TokenName || tokens[1].Kind() != TokenNumber || tokens[2].Text() != "Tf" {
		t.Fatalf("tokens = %#v", tokens)
	}
}

func TestPageStreamsSequenceIsRepeatableAndSkipsLaterNonStream(t *testing.T) {
	d := &Document{}
	page := Page{dict: Dict{Name("Contents"): Array{
		newStream(nil, []byte("first")),
		Number(7),
	}}}
	first := 0
	for stream, err := range page.Streams(d) {
		if err != nil {
			t.Fatal(err)
		}
		if string(stream.DataBorrowed()) != "first" {
			t.Fatalf("first stream = %q", stream.DataBorrowed())
		}
		first++
		break
	}
	second, deferredErr := 0, false
	for stream, err := range page.Streams(d) {
		if err != nil {
			deferredErr = true
			break
		}
		if string(stream.DataBorrowed()) != "first" {
			t.Fatalf("repeated stream = %q", stream.DataBorrowed())
		}
		second++
	}
	if first != 1 || second != 1 || deferredErr {
		t.Fatalf("stream sequence = first %d, second %d, deferred error %v", first, second, deferredErr)
	}
}

func TestPageTokensSequenceIsRepeatableAndRecoversLaterEOF(t *testing.T) {
	d := &Document{}
	page := Page{dict: Dict{Name("Contents"): Array{
		newStream(nil, []byte("/A /B")),
		newStream(nil, []byte("(")),
	}}}
	first := 0
	for token, err := range page.Tokens(d) {
		if err != nil {
			t.Fatal(err)
		}
		if token.Text() != "A" {
			t.Fatalf("first token = %#v", token)
		}
		first++
		break
	}
	second, deferredErr := 0, false
	for token, err := range page.Tokens(d) {
		if err != nil {
			deferredErr = true
			break
		}
		if token.Text() != "A" && token.Text() != "B" {
			t.Fatalf("repeated token = %#v", token)
		}
		second++
	}
	if first != 1 || second != 2 || deferredErr {
		t.Fatalf("token sequence = first %d, second %d, deferred error %v", first, second, deferredErr)
	}
}

func TestPageTokensContinueAfterUnterminatedStringStream(t *testing.T) {
	d := &Document{}
	page := Page{dict: Dict{Name("Contents"): Array{
		newStream(nil, []byte("/A (")),
		newStream(nil, []byte("/B")),
	}}}
	var got []string
	for token, err := range page.Tokens(d) {
		if err != nil {
			t.Fatal(err)
		}
		got = append(got, token.Text())
	}
	if !reflect.DeepEqual(got, []string{"A", "B"}) {
		t.Fatalf("tokens after unterminated string = %#v", got)
	}
}

func TestPageTokensStopBeforeLaterStream(t *testing.T) {
	d := &Document{}
	page := Page{dict: Dict{Name("Contents"): Array{
		newStream(nil, []byte("/F1")),
		newStream(nil, []byte("(")),
	}}}

	count := 0
	for token, err := range page.Tokens(d) {
		if err != nil {
			t.Fatal(err)
		}
		if token.Kind() != TokenName {
			t.Fatalf("token = %#v", token)
		}
		count++
		break
	}
	if count != 1 {
		t.Fatalf("consumed %d tokens, want 1", count)
	}
}

func TestPageInterpFiltersDirectTextObjects(t *testing.T) {
	d := &Document{}
	page := Page{dict: Dict{Name("Contents"): newStream(nil, []byte("BT (A) Tj ET 0 0 m 1 1 l S"))}}

	var got []string
	for object, err := range page.Interp(d, contentconfig.Options{Filter: FilterText}) {
		if err != nil {
			t.Fatal(err)
		}
		if object.Kind() != ContentText || object.text == nil {
			t.Fatalf("object = %#v", object)
		}
		got = append(got, object.text.Text())
	}
	if len(got) != 1 || got[0] != "A" {
		t.Fatalf("text objects = %#v", got)
	}
}

func TestPageInterpFilterAllStopsWithoutBlockingWorkers(t *testing.T) {
	d := &Document{}
	page := Page{dict: Dict{Name("Contents"): newStream(nil, []byte(
		"BT (A) Tj ET 0 0 m 1 1 l S /Im Do",
	))}}

	done := make(chan struct{})
	go func() {
		for _, err := range page.Interp(d, contentconfig.Options{Filter: FilterAll}) {
			if err != nil {
				return
			}
			break
		}
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("FilterAll did not stop after the consumer returned")
	}

	// A second traversal must still be able to start after the first one was
	// cancelled; this also catches workers left waiting on stale channels.
	count := 0
	for object, err := range page.Interp(d, contentconfig.Options{Filter: FilterAll}) {
		if err != nil {
			t.Fatal(err)
		}
		count++
		if object.Kind() == ContentText {
			break
		}
	}
	if count == 0 {
		t.Fatal("second FilterAll traversal produced no objects")
	}
}

func TestAllInterpreterRoutesOnlyStateRelevantOperators(t *testing.T) {
	tests := []struct {
		name     string
		worker   int
		operator string
		want     bool
	}{
		{name: "text show skips path", worker: 1, operator: "Tj", want: false},
		{name: "text show skips image", worker: 2, operator: "Tj", want: false},
		{name: "text show skips tag", worker: 3, operator: "Tj", want: false},
		{name: "path paint to path", worker: 1, operator: "S", want: true},
		{name: "image draw to image", worker: 2, operator: "Do", want: true},
		{name: "marked point to tag", worker: 3, operator: "MP", want: true},
		{name: "graphics state reaches every worker", worker: 3, operator: "gs", want: true},
		{name: "marked content reaches every worker", worker: 2, operator: "BDC", want: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			op := newContentOpBorrowed(test.operator, nil, 0)
			if got := allWorkerAccepts(test.worker, op); got != test.want {
				t.Fatalf("allWorkerAccepts(%d, %q) = %t, want %t", test.worker, test.operator, got, test.want)
			}
		})
	}
}

func TestPageFlattenIncludesFormTextOnce(t *testing.T) {
	form := newStream(Dict{Name("Subtype"): Name("Form")}, []byte("BT (F) Tj ET"))
	d := &Document{}
	page := Page{dict: Dict{
		Name("Contents"):  newStream(nil, []byte("BT (P) Tj ET /Fm Do")),
		Name("Resources"): Dict{Name("XObject"): Dict{Name("Fm"): form}},
	}}

	var got []string
	for object, err := range page.Flatten(d, contentconfig.Options{Filter: FilterText}) {
		if err != nil {
			t.Fatal(err)
		}
		got = append(got, object.text.Text())
	}
	if len(got) != 2 || got[0] != "P" || got[1] != "F" {
		t.Fatalf("flattened text = %#v", got)
	}
}

func TestPageFlattenLazilyExpandsFormFonts(t *testing.T) {
	font := Dict{Name("Subtype"): Name("Type1"), Name("BaseFont"): Name("Helvetica")}
	form := newStream(Dict{Name("Subtype"): Name("Form"), Name("Resources"): Dict{Name("Font"): Dict{Name("F"): font}}}, []byte("BT /F 10 Tf (F) Tj ET"))
	d := &Document{}
	page := Page{dict: Dict{
		Name("Contents"):  newStream(nil, []byte("/Fm Do")),
		Name("Resources"): Dict{Name("XObject"): Dict{Name("Fm"): form}},
	}}

	var got []string
	for text, err := range page.Texts(d) {
		if err != nil {
			t.Fatal(err)
		}
		got = append(got, text.Text())
	}
	if len(got) != 1 || got[0] != "F" {
		t.Fatalf("flattened form text = %#v", got)
	}
}

func TestFormObjectInheritsParentResourcesWhenResourcesAreOmitted(t *testing.T) {
	form := newStream(Dict{Name("Subtype"): Name("Form")}, []byte("BT /F 10 Tf (F) Tj ET"))
	font := Dict{Name("Subtype"): Name("Type1"), Name("BaseFont"): Name("Helvetica")}
	d := &Document{}
	page := Page{ref: Ref{Object: 1}, dict: Dict{
		Name("Contents"):  newStream(nil, []byte("/Fm Do")),
		Name("Resources"): Dict{Name("Font"): Dict{Name("F"): font}, Name("XObject"): Dict{Name("Fm"): form}},
	}}
	var objects []XObjectObject
	for object, err := range page.XObjects(d) {
		if err != nil {
			t.Fatal(err)
		}
		objects = append(objects, object)
	}
	if len(objects) != 1 {
		t.Fatalf("inherited form objects = %#v", objects)
	}
	resources, err := objects[0].ResourcesWithError(d)
	if err != nil {
		t.Fatal(err)
	}
	if resources != nil {
		t.Fatalf("inherited form public resources = %#v, want nil", resources)
	}
	fonts, err := objects[0].Fonts(d)
	if err != nil {
		t.Fatal(err)
	}
	if len(fonts) != 0 {
		t.Fatalf("inherited form public fonts = %#v, want empty", fonts)
	}
	var texts []string
	for text, err := range objects[0].Texts(d) {
		if err != nil {
			t.Fatal(err)
		}
		texts = append(texts, text.Text())
	}
	if len(texts) != 1 || texts[0] != "F" {
		t.Fatalf("inherited form text = %#v", texts)
	}
}

func TestFormObjectCapturesInvocationGraphicsState(t *testing.T) {
	form := newStream(Dict{Name("Subtype"): Name("Form")}, []byte("BT (F) Tj ET"))
	d := &Document{}
	page := Page{ref: Ref{Object: 1}, dict: Dict{
		Name("Contents"):  newStream(nil, []byte("q 2 0 0 2 0 0 cm /Fm Do Q")),
		Name("Resources"): Dict{Name("XObject"): Dict{Name("Fm"): form}},
	}}
	var objects []XObjectObject
	for object, err := range page.XObjects(d) {
		if err != nil {
			t.Fatal(err)
		}
		objects = append(objects, object)
	}
	if len(objects) != 1 || objects[0].GState().CTM()[0] != 2 || objects[0].GState().CTM()[3] != 2 {
		t.Fatalf("form invocation graphics state = %#v", objects)
	}
	for text, err := range objects[0].Texts(d) {
		if err != nil {
			t.Fatal(err)
		}
		if text.GState().CTM()[0] != 2 || text.GState().CTM()[3] != 2 {
			t.Fatalf("form text initial graphics state = %#v", text.GState())
		}
	}
}

func TestNestedFormObjectInvocationGraphicsStateIncludesAncestorMatrices(t *testing.T) {
	child := newStream(Dict{Name("Subtype"): Name("Form"), Name("Matrix"): Array{Number(1), Number(0), Number(0), Number(1), Number(17), Number(19)}}, []byte("q Q"))
	parent := newStream(Dict{
		Name("Subtype"):   Name("Form"),
		Name("Matrix"):    Array{Number(2), Number(0), Number(0), Number(3), Number(5), Number(7)},
		Name("Resources"): Dict{Name("XObject"): Dict{Name("Child"): Ref{Object: 2}}},
	}, []byte("4 0 0 5 11 13 cm /Child Do"))
	document := &Document{objects: map[Ref]Object{{Object: 1}: parent, {Object: 2}: child}}
	page := Page{dict: Dict{
		Name("Contents"):  newStream(nil, []byte("1 2 0 1 10 20 cm /Parent Do")),
		Name("Resources"): Dict{Name("XObject"): Dict{Name("Parent"): Ref{Object: 1}}},
	}}
	var objects []XObjectObject
	for object, err := range page.XObjects(document) {
		if err != nil {
			t.Fatal(err)
		}
		objects = append(objects, object)
	}
	if len(objects) != 2 {
		t.Fatalf("xobjects = %#v, want parent and child", objects)
	}
	pageMatrix := geometry.Matrix{1, 2, 0, 1, 10, 20}
	parentMatrix := geometry.Matrix{2, 0, 0, 3, 5, 7}
	insideMatrix := geometry.Matrix{4, 0, 0, 5, 11, 13}
	want := pageMatrix.Mul(parentMatrix).Mul(insideMatrix)
	if got := objects[1].GState().CTM(); got != want {
		t.Fatalf("nested invocation ctm = %v, want ancestor-aware ctm %v", got, want)
	}
}

func TestFormObjectCarriesInheritedExternalGraphicsState(t *testing.T) {
	form := newStream(Dict{Name("Subtype"): Name("Form")}, []byte("q Q"))
	page := Page{ref: Ref{Object: 1}, dict: Dict{
		Name("Contents"): newStream(nil, []byte("q /GS gs /Fm Do Q")),
		Name("Resources"): Dict{
			Name("ExtGState"): Dict{Name("GS"): Dict{
				Name("LW"): Number(2),
				Name("D"):  Array{Array{Number(3), Number(1)}, Number(0)},
				Name("BM"): Name("Multiply"),
				Name("CA"): Number(0.5),
				Name("ca"): Number(0.5),
			}},
			Name("XObject"): Dict{Name("Fm"): form},
		},
	}}

	var objects []XObjectObject
	for object, err := range page.XObjects(&Document{}) {
		if err != nil {
			t.Fatal(err)
		}
		objects = append(objects, object)
	}
	if len(objects) != 1 {
		t.Fatalf("xobjects = %#v", objects)
	}
	state := objects[0].GState()
	if state.LineWidth() != 2 || state.BlendMode() != "Multiply" || state.StrokeAlpha() != 0.5 || state.FillAlpha() != 0.5 {
		t.Fatalf("inherited external graphics state = %#v", state)
	}
	if dash := state.DashCopy(); len(dash) != 2 || dash[0] != 3 || dash[1] != 1 {
		t.Fatalf("inherited dash pattern = %#v", dash)
	}
}

func TestPageInterpFormObjectCapturesInvocationGraphicsState(t *testing.T) {
	form := newStream(Dict{Name("Subtype"): Name("Form")}, nil)
	d := &Document{}
	page := Page{ref: Ref{Object: 1}, dict: Dict{
		Name("Contents"):  newStream(nil, []byte("q 4 0 0 4 0 0 cm /Fm Do Q")),
		Name("Resources"): Dict{Name("XObject"): Dict{Name("Fm"): form}},
	}}
	var objects []XObjectObject
	for object, err := range page.Interp(d, contentconfig.Options{Filter: FilterXObject}) {
		if err != nil {
			t.Fatal(err)
		}
		objects = append(objects, *object.xobject)
	}
	if len(objects) != 1 || objects[0].GState().CTM()[0] != 4 || objects[0].GState().CTM()[3] != 4 {
		t.Fatalf("streamed form graphics state = %#v", objects)
	}
}

func TestPageTextsStopBeforeMalformedLaterStream(t *testing.T) {
	d := &Document{}
	page := Page{dict: Dict{Name("Contents"): Array{
		newStream(nil, []byte("BT (A) Tj ET")),
		newStream(nil, []byte("(")),
	}}}

	count := 0
	for text, err := range page.Texts(d) {
		if err != nil {
			t.Fatal(err)
		}
		if text.Text() != "A" {
			t.Fatalf("text = %#v", text)
		}
		count++
		break
	}
	if count != 1 {
		t.Fatalf("consumed %d text objects, want 1", count)
	}
}

func TestPageTextsPreservesOperandsAcrossStreams(t *testing.T) {
	d := &Document{}
	page := Page{dict: Dict{Name("Contents"): Array{
		newStream(nil, []byte("BT (A) ")),
		newStream(nil, []byte("Tj ET")),
	}}}
	var got []string
	for text, err := range page.Texts(d) {
		if err != nil {
			t.Fatal(err)
		}
		got = append(got, text.Text())
	}
	if len(got) != 1 || got[0] != "A" {
		t.Fatalf("texts = %#v", got)
	}
}

func TestContentIteratorCachesInlineImagesByPageOffset(t *testing.T) {
	d := &Document{}
	p := Page{ref: Ref{Object: 7}, dict: Dict{Name("Contents"): newStream(nil, []byte("BI /W 1 /H 1 /BPC 8 /CS /G ID x EI Q"))}}
	first := newContentOpIterator(d, p)
	if _, err, ok := first.next(); err != nil || !ok {
		t.Fatalf("first inline image: op=%v err=%v", ok, err)
	}
	second := newContentOpIterator(d, p)
	if _, err, ok := second.next(); err != nil || !ok {
		t.Fatalf("cached inline image: op=%v err=%v", ok, err)
	}
	if op, err, ok := second.next(); err != nil || !ok || op.Operator() != "Q" {
		t.Fatalf("cached inline image did not advance past EI: op=%q ok=%v err=%v", op.Operator(), ok, err)
	}
	if len(d.inlineImageCache) != 1 {
		t.Fatalf("inline image cache size = %d, want 1", len(d.inlineImageCache))
	}
}

func TestInlineImageCacheHasMemoryBudget(t *testing.T) {
	d := &Document{}
	d.cacheInlineImage(inlineImageKey{page: Ref{Object: 1}}, newStream(nil, bytes.Repeat([]byte{1}, inlineImageCacheLimit+1)))
	if len(d.inlineImageCache) != 0 || d.inlineImageCacheBytes != 0 {
		t.Fatalf("oversized inline image was cached: entries=%d bytes=%d", len(d.inlineImageCache), d.inlineImageCacheBytes)
	}
}

func TestInlineImageCacheDoesNotDoubleCountDuplicateKeys(t *testing.T) {
	d := &Document{}
	key := inlineImageKey{page: Ref{Object: 1}, stream: 2, offset: 3}
	first := newStream(nil, bytes.Repeat([]byte{1}, 11))
	second := newStream(nil, bytes.Repeat([]byte{2}, 17))
	d.cacheInlineImage(key, first)
	d.cacheInlineImage(key, second)
	if d.inlineImageCacheBytes != len(first.DataBorrowed()) {
		t.Fatalf("duplicate inline image key changed byte accounting: got %d, want %d", d.inlineImageCacheBytes, len(first.DataBorrowed()))
	}
	if got := string(d.inlineImageCache[key].DataBorrowed()); got != string(first.DataBorrowed()) {
		t.Fatalf("duplicate inline image replaced cached value: got %q, want %q", got, first.DataBorrowed())
	}
}

func TestPageXObjectsIncludesNestedFormsInOrder(t *testing.T) {
	inner := newStream(Dict{Name("Subtype"): Name("Form")}, []byte("BT (I) Tj ET"))
	d := &Document{objects: map[Ref]Object{
		{Object: 1}: newStream(Dict{
			Name("Subtype"):   Name("Form"),
			Name("Resources"): Dict{Name("XObject"): Dict{Name("Inner"): Ref{Object: 2}}},
		}, []byte("/Inner Do")),
		{Object: 2}: inner,
	}}
	page := Page{dict: Dict{
		Name("Contents"):  newStream(nil, []byte("/Outer Do")),
		Name("Resources"): Dict{Name("XObject"): Dict{Name("Outer"): Ref{Object: 1}}},
	}, ref: Ref{Object: 9}}

	var got []string
	for object, err := range page.XObjects(d) {
		if err != nil {
			t.Fatal(err)
		}
		if !object.HasPage() || object.Page() != page.ref {
			t.Fatalf("xobject page = %#v", object)
		}
		got = append(got, object.Path())
	}
	if len(got) != 2 || got[0] != "Outer" || got[1] != "Outer/Inner" {
		t.Fatalf("xobjects = %#v", got)
	}
}

func TestPageXObjectsReportsMalformedResourceRoot(t *testing.T) {
	p := Page{dict: Dict{Name("Resources"): Dict{Name("XObject"): Number(1)}}}
	for _, err := range p.XObjects(&Document{}) {
		if err == nil {
			t.Fatal("malformed XObject root produced an object")
		}
		return
	}
	t.Fatal("malformed XObject root produced no error")
}

func TestPageXObjectsReportsMalformedResourceEntry(t *testing.T) {
	p := Page{dict: Dict{Name("Resources"): Dict{Name("XObject"): Dict{Name("Fm"): Number(1)}}}}
	for _, err := range p.XObjects(&Document{}) {
		if err == nil {
			t.Fatal("malformed XObject entry produced an object")
		}
		return
	}
	t.Fatal("malformed XObject entry produced no error")
}

func TestPageXObjectsReportsUnresolvedResourceEntry(t *testing.T) {
	d := &Document{objects: map[Ref]Object{{Object: 1}: Dict{Name("XObject"): Dict{Name("Fm"): Ref{Object: 99}}}}}
	p := Page{dict: Dict{Name("Resources"): Ref{Object: 1}, Name("Contents"): newStream(nil, []byte("/Fm Do"))}}
	for object, err := range p.XObjects(d) {
		if err == nil || object.Name() != "" {
			t.Fatalf("unresolved XObject = %#v, err=%v", object, err)
		}
		return
	}
	t.Fatal("unresolved XObject produced no error")
}

func TestPageXObjectsReportsUnresolvedSubtype(t *testing.T) {
	d := &Document{objects: map[Ref]Object{{Object: 1}: newStream(Dict{Name("Subtype"): Ref{Object: 99}}, []byte("q Q"))}}
	p := Page{dict: Dict{Name("Resources"): Dict{Name("XObject"): Dict{Name("Fm"): Ref{Object: 1}}}, Name("Contents"): newStream(nil, []byte("/Fm Do"))}}
	for object, err := range p.XObjects(d) {
		if err == nil || object.Name() != "" {
			t.Fatalf("unresolved subtype XObject = %#v, err=%v", object, err)
		}
		return
	}
	t.Fatal("unresolved subtype produced no error")
}

func TestPageFilterXObjectReportsMalformedResourceRoot(t *testing.T) {
	p := Page{dict: Dict{
		Name("Contents"):  newStream(nil, []byte("/Fm Do")),
		Name("Resources"): Dict{Name("XObject"): Number(1)},
	}}
	for _, err := range p.Interp(&Document{}, contentconfig.Options{Filter: FilterXObject}) {
		if err == nil {
			t.Fatal("FilterXObject silently accepted malformed XObject root")
		}
		return
	}
	t.Fatal("FilterXObject produced no error")
}

func TestPageFilterXObjectReportsUnresolvedResourceRoot(t *testing.T) {
	d := &Document{}
	p := Page{dict: Dict{
		Name("Contents"):  newStream(nil, []byte("/Fm Do")),
		Name("Resources"): Dict{Name("XObject"): Ref{Object: 99}},
	}}
	for object, err := range p.Interp(d, contentconfig.Options{Filter: FilterXObject}) {
		if err == nil || object.Kind() != "" {
			t.Fatalf("unresolved XObject root = %#v, err=%v", object, err)
		}
		return
	}
	t.Fatal("unresolved XObject root produced no error")
}

func TestPageFilterXObjectReportsUnresolvedSubtype(t *testing.T) {
	d := &Document{objects: map[Ref]Object{{Object: 1}: newStream(Dict{Name("Subtype"): Ref{Object: 99}}, []byte("q Q"))}}
	p := Page{dict: Dict{
		Name("Contents"):  newStream(nil, []byte("/Fm Do")),
		Name("Resources"): Dict{Name("XObject"): Dict{Name("Fm"): Ref{Object: 1}}},
	}}
	for object, err := range p.Interp(d, contentconfig.Options{Filter: FilterXObject}) {
		if err == nil || object.Kind() != "" {
			t.Fatalf("unresolved XObject subtype = %#v, err=%v", object, err)
		}
		return
	}
	t.Fatal("unresolved XObject subtype produced no error")
}

func TestPageXObjectsResolveResourceColorSpaceInGraphicsState(t *testing.T) {
	form := newStream(Dict{Name("Subtype"): Name("Form")}, []byte("q Q"))
	d := &Document{}
	page := Page{dict: Dict{
		Name("Contents"): newStream(nil, []byte("/CS1 CS /Fm Do")),
		Name("Resources"): Dict{
			Name("ColorSpace"): Dict{Name("CS1"): Name("DeviceRGB")},
			Name("XObject"):    Dict{Name("Fm"): form},
		},
	}}
	var objects []XObjectObject
	for object, err := range page.XObjects(d) {
		if err != nil {
			t.Fatal(err)
		}
		objects = append(objects, object)
	}
	if len(objects) != 1 || objects[0].GState().StrokeColor().Space() != "DeviceRGB" {
		t.Fatalf("xobject color space = %#v, want predefined DeviceRGB", objects)
	}
}

func TestXObjectExposesContentSequences(t *testing.T) {
	form := testXObjectStream(newStream(nil, []byte("BT (inside) Tj ET")))
	d := &Document{}
	var ops []string
	for op, err := range form.Contents(d) {
		if err != nil {
			t.Fatal(err)
		}
		ops = append(ops, op.operatorValue())
	}
	if len(ops) != 3 || ops[0] != "BT" || ops[1] != "Tj" || ops[2] != "ET" {
		t.Fatalf("xobject contents = %#v", ops)
	}
	var texts []string
	for object, err := range form.Interp(d, contentconfig.Options{Filter: FilterText}) {
		if err != nil {
			t.Fatal(err)
		}
		texts = append(texts, object.text.Text())
	}
	if len(texts) != 1 || texts[0] != "inside" {
		t.Fatalf("xobject text = %#v", texts)
	}
	var first []string
	for token, err := range form.Tokens(d) {
		if err != nil {
			t.Fatal(err)
		}
		first = append(first, token.Text())
		break
	}
	if len(first) != 1 || first[0] != "BT" {
		t.Fatalf("early xobject tokens = %#v", first)
	}
	var second []string
	for token, err := range form.Tokens(d) {
		if err != nil {
			t.Fatal(err)
		}
		second = append(second, token.Text())
	}
	if len(second) != 4 || second[0] != "BT" || second[len(second)-1] != "ET" {
		t.Fatalf("repeated xobject tokens = %#v", second)
	}
}

func TestXObjectContentsReportsUnresolvedReference(t *testing.T) {
	x := testXObjectRefStream(Ref{Object: 99}, newStream(nil, []byte("q Q")))
	for _, err := range x.Contents(&Document{objects: map[Ref]Object{}}) {
		if err == nil || err.Error() != "playa: XObject reference could not be resolved" {
			t.Fatalf("unresolved XObject reference error = %v", err)
		}
		return
	}
	t.Fatal("unresolved XObject reference produced no error")
}

func TestXObjectTextsReportsUnresolvedReference(t *testing.T) {
	x := testXObjectRefStream(Ref{Object: 99}, newStream(nil, []byte("BT (fallback) Tj ET")))
	for _, err := range x.Texts(&Document{objects: map[Ref]Object{}}) {
		if err == nil || err.Error() != "playa: XObject reference could not be resolved" {
			t.Fatalf("unresolved XObject text error = %v", err)
		}
		return
	}
	t.Fatal("unresolved XObject text reference produced no error")
}

func TestXObjectContentUsesIndirectStreamCache(t *testing.T) {
	ref := Ref{Object: 12}
	stream := newStream(nil, []byte("q"))
	d := &Document{objects: map[Ref]Object{ref: stream}, decodedStreamCache: map[Ref][]byte{}}
	form := testXObjectRefStream(ref, stream)
	for op, err := range form.Contents(d) {
		if err != nil {
			t.Fatal(err)
		}
		if op.operatorValue() != "q" {
			t.Fatalf("form operator = %q", op.operatorValue())
		}
	}
	for token, err := range form.Tokens(d) {
		if err != nil {
			t.Fatal(err)
		}
		if token.Text() != "q" {
			t.Fatalf("form token = %q", token.Text())
		}
	}
	if len(d.decodedStreamCache) != 1 {
		t.Fatalf("decoded stream cache size = %d, want 1", len(d.decodedStreamCache))
	}
}

func TestXObjectContentFollowsMultiLevelIndirectStreamAndResources(t *testing.T) {
	streamRef := Ref{Object: 12}
	resourcesRef := Ref{Object: 20}
	d := &Document{objects: map[Ref]Object{
		{Object: 10}: streamRef,
		streamRef:    newStream(Dict{Name("Resources"): Ref{Object: 21}}, []byte("q")),
		{Object: 21}: resourcesRef,
		resourcesRef: Dict{Name("ProcSet"): Array{Name("PDF")}},
	}}
	form := testXObjectRefStream(Ref{Object: 10}, newStream(nil, []byte("broken")))
	var operators []string
	for op, err := range form.Contents(d) {
		if err != nil {
			t.Fatal(err)
		}
		operators = append(operators, op.operatorValue())
	}
	if len(operators) != 1 || operators[0] != "q" {
		t.Fatalf("indirect xobject contents = %#v, want q", operators)
	}
	if got := form.pageView(d).Resources(d); got == nil || got[Name("ProcSet")] == nil {
		t.Fatalf("indirect xobject resources = %#v", got)
	}
}

func TestDetachedXObjectFallsBackToEmbeddedStream(t *testing.T) {
	form := testXObjectRefStream(Ref{Object: 99}, newStream(nil, []byte("q")))
	for op, err := range form.Contents(&Document{}) {
		if err != nil {
			t.Fatal(err)
		}
		if op.operatorValue() != "q" {
			t.Fatalf("detached form operator = %q", op.operatorValue())
		}
	}
	for _, err := range form.Tokens(nil) {
		if err != errNilDocument {
			t.Fatalf("nil document error = %v", err)
		}
		break
	}
}

func TestXObjectContentSequencesAreRepeatableAndStopEarly(t *testing.T) {
	form := testXObjectStream(newStream(nil, []byte("q 1 0 m Q")))
	d := &Document{}
	firstOp := 0
	for op, err := range form.Contents(d) {
		if err != nil {
			t.Fatal(err)
		}
		if op.operatorValue() != "q" {
			t.Fatalf("first xobject op = %#v", op)
		}
		firstOp++
		break
	}
	secondOps := 0
	for op, err := range form.Contents(d) {
		if err != nil {
			t.Fatal(err)
		}
		if op.operatorValue() != "q" && op.operatorValue() != "m" && op.operatorValue() != "Q" {
			t.Fatalf("repeated xobject op = %#v", op)
		}
		secondOps++
	}
	if firstOp != 1 || secondOps != 3 {
		t.Fatalf("xobject content sequence counts = first %d, second %d", firstOp, secondOps)
	}

	firstToken := 0
	for token, err := range form.Tokens(d) {
		if err != nil {
			t.Fatal(err)
		}
		if token.Text() != "q" {
			t.Fatalf("first xobject token = %#v", token)
		}
		firstToken++
		break
	}
	secondTokens := 0
	for token, err := range form.Tokens(d) {
		if err != nil {
			t.Fatal(err)
		}
		if token.Text() == "" {
			t.Fatalf("empty repeated xobject token = %#v", token)
		}
		secondTokens++
	}
	if firstToken != 1 || secondTokens != 5 {
		t.Fatalf("xobject token sequence counts = first %d, second %d", firstToken, secondTokens)
	}
}

func TestXObjectInterpretationAppliesFormMatrix(t *testing.T) {
	form := testXObject(contentdata.XObjectSpec{Matrix: geometry.Matrix{2, 0, 0, 2, 10, 20}, Stream: newStream(nil, []byte("0 0 m 1 0 l S"))})
	d := &Document{}
	var paths []PathObject
	for object, err := range form.Interp(d, contentconfig.Options{Filter: FilterPath}) {
		if err != nil {
			t.Fatal(err)
		}
		paths = append(paths, *object.path)
	}
	if len(paths) != 1 || paths[0].BBox() != [4]float64{10, 20, 12, 20} {
		t.Fatalf("xobject path bbox = %#v", paths)
	}
}

func TestPageXObjectBBoxIncludesInvocationCTM(t *testing.T) {
	form := newStream(Dict{
		Name("Subtype"): Name("Form"),
		Name("BBox"):    Array{Number(0), Number(0), Number(2), Number(3)},
	}, []byte("q Q"))
	p := Page{dict: Dict{
		Name("Contents"):  newStream(nil, []byte("1 0 0 1 300 300 cm /Fm Do")),
		Name("Resources"): Dict{Name("XObject"): Dict{Name("Fm"): form}},
	}}
	var got [4]float64
	count := 0
	for object, err := range p.Interp(&Document{}, contentconfig.Options{Filter: FilterXObject}) {
		if err != nil {
			t.Fatal(err)
		}
		got = object.XObjectBorrowed().BBox()
		count++
	}
	if count != 1 || got != [4]float64{300, 300, 302, 303} {
		t.Fatalf("invoked form bbox = %v, count=%d", got, count)
	}
}

func TestPageXObjectsCarryMarkedContext(t *testing.T) {
	form := newStream(Dict{Name("Subtype"): Name("Form")}, []byte("q Q"))
	page := Page{dict: Dict{
		Name("Resources"): Dict{Name("XObject"): Dict{Name("Fm"): form}},
		Name("Contents"):  newStream(nil, []byte("/P BMC /Span << /MCID 3 >> BDC /Fm Do EMC EMC")),
	}}
	d := &Document{}
	var objects []XObjectObject
	for object, err := range page.XObjects(d) {
		if err != nil {
			t.Fatal(err)
		}
		objects = append(objects, object)
	}
	if len(objects) != 1 || objects[0].MarkedTag() != "Span" || len(objects[0].markedStack) != 2 || objects[0].markedStack[0].Tag() != "P" || objects[0].markedStack[1].Tag() != "Span" {
		t.Fatalf("xobject marked context = %#v", objects)
	}
}

func TestPageNestedXObjectsStartWithFormLocalMarkedContext(t *testing.T) {
	innerRef := Ref{Object: 1}
	outerRef := Ref{Object: 2}
	d := &Document{objects: map[Ref]Object{
		innerRef: newStream(Dict{Name("Subtype"): Name("Form")}, []byte("q Q")),
		outerRef: newStream(Dict{
			Name("Subtype"):   Name("Form"),
			Name("Resources"): Dict{Name("XObject"): Dict{Name("Inner"): innerRef}},
		}, []byte("/Local BMC /Inner Do EMC")),
	}}
	page := Page{dict: Dict{
		Name("Resources"): Dict{Name("XObject"): Dict{Name("Outer"): outerRef}},
		Name("Contents"):  newStream(nil, []byte("/Page BMC /Outer Do EMC")),
	}}
	var objects []XObjectObject
	for object, err := range page.XObjects(d) {
		if err != nil {
			t.Fatal(err)
		}
		objects = append(objects, object)
	}
	if len(objects) != 2 {
		t.Fatalf("xobjects = %#v, want outer and inner forms", objects)
	}
	if stack := objects[0].MarkedStackCopy(); len(stack) != 1 || stack[0].Tag() != "Page" {
		t.Fatalf("outer marked stack = %#v, want page invocation context", stack)
	}
	if stack := objects[1].MarkedStackCopy(); len(stack) != 1 || stack[0].Tag() != "Local" {
		t.Fatalf("inner marked stack = %#v, want only form-local context", stack)
	}
}

func TestPageXObjectsResetTransparencyGroupState(t *testing.T) {
	form := newStream(Dict{
		Name("Subtype"): Name("Form"),
		Name("Group"):   Dict{Name("S"): Name("Transparency")},
	}, []byte("q Q"))
	page := Page{dict: Dict{
		Name("Resources"): Dict{Name("XObject"): Dict{Name("Fm"): form}},
		Name("Contents"):  newStream(nil, []byte("0.4 ca /Multiply BM /Fm Do")),
	}}
	d := &Document{}
	var objects []XObjectObject
	for object, err := range page.XObjects(d) {
		if err != nil {
			t.Fatal(err)
		}
		objects = append(objects, object)
	}
	if len(objects) != 1 {
		t.Fatalf("xobjects = %#v", objects)
	}
	got := objects[0].GState()
	if got.BlendMode() != "Normal" || got.StrokeAlpha() != 1 || got.FillAlpha() != 1 || got.Alpha() != 1 || got.HasSoftMask() {
		t.Fatalf("transparency group state = %#v", got)
	}
}

func TestPageFilterAllXObjectsCarryMarkedContext(t *testing.T) {
	form := newStream(Dict{Name("Subtype"): Name("Form")}, []byte("q Q"))
	page := Page{dict: Dict{
		Name("Resources"): Dict{Name("XObject"): Dict{Name("Fm"): form}},
		Name("Contents"):  newStream(nil, []byte("/P BMC /Span << /MCID 3 >> BDC /Fm Do EMC EMC")),
	}}
	d := &Document{}
	var objects []XObjectObject
	for object, err := range page.Interp(d, contentconfig.Options{Filter: FilterAll}) {
		if err != nil {
			t.Fatal(err)
		}
		if object.xobject != nil {
			objects = append(objects, *object.xobject)
		}
	}
	if len(objects) != 1 || objects[0].MarkedTag() != "Span" || len(objects[0].markedStack) != 2 {
		t.Fatalf("filter-all xobject marked context = %#v", objects)
	}
}

func TestPageXObjectsDefersLaterMalformedContent(t *testing.T) {
	form := newStream(Dict{Name("Subtype"): Name("Form")}, []byte("q Q"))
	page := Page{dict: Dict{
		Name("Resources"): Dict{Name("XObject"): Dict{Name("Fm"): form}},
		Name("Contents"): Array{
			newStream(nil, []byte("/Fm Do")),
			newStream(nil, []byte("[ unterminated")),
		},
	}}
	d := &Document{}
	count := 0
	for object, err := range page.XObjects(d) {
		if err != nil {
			t.Fatal(err)
		}
		if object.Name() != "Fm" {
			t.Fatalf("xobject = %#v", object)
		}
		count++
		break
	}
	if count != 1 {
		t.Fatalf("xobject count = %d", count)
	}
}

func TestPageXObjectsSequenceIsRepeatableAndStopsEarly(t *testing.T) {
	form := newStream(Dict{Name("Subtype"): Name("Form")}, []byte("q Q"))
	page := Page{dict: Dict{
		Name("Resources"): Dict{Name("XObject"): Dict{Name("First"): form, Name("Second"): form}},
		Name("Contents"):  newStream(nil, []byte("/First Do /Second Do")),
	}}
	d := &Document{}
	first := 0
	for object, err := range page.XObjects(d) {
		if err != nil {
			t.Fatal(err)
		}
		if object.Name() != "First" {
			t.Fatalf("first xobject = %#v", object)
		}
		first++
		break
	}
	second := 0
	for object, err := range page.XObjects(d) {
		if err != nil {
			t.Fatal(err)
		}
		if object.Name() != "First" && object.Name() != "Second" {
			t.Fatalf("repeated xobject = %#v", object)
		}
		second++
	}
	if first != 1 || second != 2 {
		t.Fatalf("xobject sequence counts = first %d, second %d", first, second)
	}
}

func TestPageXObjectsReusesValidatedPageResources(t *testing.T) {
	form := newStream(Dict{Name("Subtype"): Name("Form")}, []byte("q Q"))
	d := &Document{objects: map[Ref]Object{
		{Object: 1}: Dict{Name("XObject"): Ref{Object: 2}},
		{Object: 2}: Dict{Name("Fm"): Ref{Object: 3}},
		{Object: 3}: form,
	}}
	page := Page{ref: Ref{Object: 9}, dict: Dict{Name("Resources"): Ref{Object: 1}, Name("Contents"): newStream(nil, []byte("/Fm Do"))}}
	if _, err := page.ResourcesWithError(d); err != nil {
		t.Fatal(err)
	}
	d.objects[Ref{Object: 1}] = Dict{}
	object, err := firstXObject(page.XObjects(d))
	if err != nil || object.Name() != "Fm" {
		t.Fatalf("XObjects did not reuse validated resources: %#v, err=%v", object, err)
	}
}

func TestPageXObjectsReusesCachedFormResources(t *testing.T) {
	form := newStream(Dict{Name("Subtype"): Name("Form"), Name("Resources"): Ref{Object: 2}}, []byte("q Q"))
	d := &Document{objects: map[Ref]Object{
		{Object: 1}: Dict{Name("Fm"): Ref{Object: 7}},
		{Object: 2}: Dict{Name("Marker"): String("cached")},
		{Object: 7}: form,
	}}
	x := testXObjectRefStream(Ref{Object: 7}, form)
	if _, err := x.ResourcesWithError(d); err != nil {
		t.Fatal(err)
	}
	d.objects[Ref{Object: 7}] = newStream(Dict{Name("Subtype"): Name("Form"), Name("Resources"): Ref{Object: 99}}, []byte("q Q"))
	page := Page{dict: Dict{
		Name("Contents"):  newStream(nil, []byte("/Fm Do")),
		Name("Resources"): Dict{Name("XObject"): Ref{Object: 1}},
	}}
	for object, err := range page.XObjects(d) {
		if err != nil || object.Name() != "Fm" {
			t.Fatalf("XObjects did not reuse cached Form resources: %#v, err=%v", object, err)
		}
		return
	}
	t.Fatal("XObjects produced no Form")
}

func TestPageFilterXObjectReusesValidatedPageResources(t *testing.T) {
	form := newStream(Dict{Name("Subtype"): Name("Form")}, []byte("q Q"))
	d := &Document{objects: map[Ref]Object{
		{Object: 1}: Dict{Name("XObject"): Ref{Object: 2}},
		{Object: 2}: Dict{Name("Fm"): Ref{Object: 3}},
		{Object: 3}: form,
	}}
	page := Page{ref: Ref{Object: 9}, dict: Dict{Name("Resources"): Ref{Object: 1}, Name("Contents"): newStream(nil, []byte("/Fm Do"))}}
	if _, err := page.ResourcesWithError(d); err != nil {
		t.Fatal(err)
	}
	d.objects[Ref{Object: 1}] = Dict{}
	for object, err := range page.Interp(d, contentconfig.Options{Filter: FilterXObject}) {
		if err != nil || object.Kind() != ContentXObject {
			t.Fatalf("FilterXObject did not reuse validated resources: %#v, err=%v", object, err)
		}
		return
	}
	t.Fatal("FilterXObject produced no object")
}

func TestPageXObjectsDoNotShareStreamDictionaries(t *testing.T) {
	form := newStream(Dict{Name("Subtype"): Name("Form"), Name("Marker"): String("original")}, []byte("q Q"))
	page := Page{dict: Dict{
		Name("Resources"): Dict{Name("XObject"): Dict{Name("Form"): form}},
		Name("Contents"):  newStream(nil, []byte("/Form Do")),
	}}
	d := &Document{}
	first, err := firstXObject(page.XObjects(d))
	if err != nil {
		t.Fatal(err)
	}
	snapshot := first.Finalize()
	snapshot.data.StreamBorrowed().DictBorrowed()[Name("Marker")] = String("changed")
	second, err := firstXObject(page.XObjects(d))
	if err != nil {
		t.Fatal(err)
	}
	value, _ := second.data.StreamBorrowed().DictBorrowed()[Name("Marker")].(String)
	if string(value) != "original" {
		t.Fatalf("xobject dictionary was exposed: %#v", second.data.StreamBorrowed().DictBorrowed())
	}
}

func TestFlattenDoesNotShareXObjectDictionaries(t *testing.T) {
	form := newStream(Dict{Name("Subtype"): Name("Form"), Name("Marker"): String("original")}, []byte("q Q"))
	page := Page{dict: Dict{
		Name("Resources"): Dict{Name("XObject"): Dict{Name("Form"): form}},
		Name("Contents"):  newStream(nil, []byte("/Form Do")),
	}}
	d := &Document{}
	first, err := firstContentObject(page.Interp(d, contentconfig.Options{Filter: FilterXObject}))
	if err != nil || first.xobject == nil {
		t.Fatalf("first flatten object = %#v, err = %v", first, err)
	}
	snapshot := first.Finalize()
	snapshot.xobject.data.StreamBorrowed().DictBorrowed()[Name("Marker")] = String("changed")
	second, err := firstContentObject(page.Interp(d, contentconfig.Options{Filter: FilterXObject}))
	if err != nil || second.xobject == nil {
		t.Fatalf("second flatten object = %#v, err = %v", second, err)
	}
	value, _ := second.xobject.data.StreamBorrowed().DictBorrowed()[Name("Marker")].(String)
	if string(value) != "original" {
		t.Fatalf("flatten XObject dictionary was exposed: %#v", second.xobject.data.StreamBorrowed().DictBorrowed())
	}
}

func firstContentObject(sequence iter.Seq2[ContentObject, error]) (ContentObject, error) {
	for object, err := range sequence {
		return object, err
	}
	return ContentObject{}, nil
}

func firstXObject(sequence iter.Seq2[XObjectObject, error]) (XObjectObject, error) {
	for object, err := range sequence {
		return object, err
	}
	return XObjectObject{}, nil
}

func TestPageFlattenStopsCircularFormExpansion(t *testing.T) {
	d := &Document{objects: map[Ref]Object{}}
	d.objects[Ref{Object: 1}] = newStream(Dict{
		Name("Subtype"):   Name("Form"),
		Name("Resources"): Dict{Name("XObject"): Dict{Name("Loop"): Ref{Object: 1}}},
	}, []byte("/Loop Do"))
	page := Page{dict: Dict{
		Name("Contents"):  newStream(nil, []byte("/Loop Do")),
		Name("Resources"): Dict{Name("XObject"): Dict{Name("Loop"): Ref{Object: 1}}},
	}}

	count := 0
	for _, err := range page.Flatten(d, contentconfig.Options{Filter: FilterText}) {
		if err != nil {
			t.Fatal(err)
		}
		count++
		if count > 1 {
			t.Fatal("circular Form XObject was not stopped")
		}
	}
}

func TestPageXObjectsSkipsCircularFormInvocation(t *testing.T) {
	d := &Document{objects: map[Ref]Object{}}
	d.objects[Ref{Object: 1}] = newStream(Dict{
		Name("Subtype"):   Name("Form"),
		Name("Resources"): Dict{Name("XObject"): Dict{Name("Loop"): Ref{Object: 1}}},
	}, []byte("/Loop Do"))
	page := Page{dict: Dict{
		Name("Contents"):  newStream(nil, []byte("/Loop Do")),
		Name("Resources"): Dict{Name("XObject"): Dict{Name("Loop"): Ref{Object: 1}}},
	}}
	count := 0
	for object, err := range page.XObjects(d) {
		if err != nil {
			t.Fatal(err)
		}
		if object.Name() != "Loop" {
			t.Fatalf("unexpected XObject = %#v", object)
		}
		count++
	}
	if count != 1 {
		t.Fatalf("circular XObject count = %d, want 1", count)
	}
}

func TestPageXObjectsCanonicalizesIndirectFormCycleReferences(t *testing.T) {
	d := &Document{objects: map[Ref]Object{
		{Object: 1}: Ref{Object: 2},
		{Object: 2}: newStream(Dict{
			Name("Subtype"):   Name("Form"),
			Name("Resources"): Dict{Name("XObject"): Dict{Name("Loop"): Ref{Object: 2}}},
		}, []byte("/Loop Do")),
	}}
	page := Page{dict: Dict{
		Name("Contents"):  newStream(nil, []byte("/Outer Do")),
		Name("Resources"): Dict{Name("XObject"): Dict{Name("Outer"): Ref{Object: 1}}},
	}}

	count := 0
	for object, err := range page.XObjects(d) {
		if err != nil {
			t.Fatal(err)
		}
		if object.Name() != "Outer" && object.Name() != "Loop" {
			t.Fatalf("unexpected XObject = %#v", object)
		}
		count++
	}
	if count != 1 {
		t.Fatalf("aliased circular XObject count = %d, want 1", count)
	}
}

func TestPageXObjectsSkipsCircularDirectFormInvocation(t *testing.T) {
	xobjects := Dict{}
	form := newStream(Dict{
		Name("Subtype"):   Name("Form"),
		Name("Resources"): Dict{Name("XObject"): xobjects},
	}, []byte("/Loop Do"))
	xobjects[Name("Loop")] = form
	page := Page{dict: Dict{
		Name("Contents"):  newStream(nil, []byte("/Loop Do")),
		Name("Resources"): Dict{Name("XObject"): xobjects},
	}}
	count := 0
	for object, err := range page.XObjects(&Document{}) {
		if err != nil {
			t.Fatal(err)
		}
		if object.Name() != "Loop" {
			t.Fatalf("unexpected direct XObject = %#v", object)
		}
		count++
	}
	if count != 1 {
		t.Fatalf("direct circular XObject count = %d, want 1", count)
	}
}

func TestPageInterpPreservesTextAndPathOrder(t *testing.T) {
	d := &Document{}
	page := Page{dict: Dict{Name("Contents"): newStream(nil, []byte(
		"BT (A) Tj ET 0 0 m 1 1 l S BT (B) Tj ET",
	))}}

	var kinds []contentconfig.Kind
	for object, err := range page.Interp(d, contentconfig.Options{}) {
		if err != nil {
			t.Fatal(err)
		}
		kinds = append(kinds, object.Kind())
	}
	want := []contentconfig.Kind{ContentText, ContentPath, ContentText}
	if len(kinds) != len(want) {
		t.Fatalf("kinds = %#v, want %#v", kinds, want)
	}
	for i := range want {
		if kinds[i] != want[i] {
			t.Fatalf("kinds = %#v, want %#v", kinds, want)
		}
	}
}

func TestPageInterpStreamsAllContentKinds(t *testing.T) {
	form := newStream(Dict{Name("Subtype"): Name("Form")}, []byte("0 0 m 1 1 l S"))
	image := newStream(Dict{Name("Subtype"): Name("Image"), Name("Width"): Number(1), Name("Height"): Number(1), Name("BPC"): Number(8), Name("CS"): Name("G")}, []byte{7})
	d := &Document{}
	p := Page{dict: Dict{
		Name("Contents"):  newStream(nil, []byte("BT (A) Tj ET /Fm Do /Im Do")),
		Name("Resources"): Dict{Name("XObject"): Dict{Name("Fm"): form, Name("Im"): image}},
	}}
	var kinds []contentconfig.Kind
	for object, err := range p.Interp(d, contentconfig.Options{}) {
		if err != nil {
			t.Fatal(err)
		}
		kinds = append(kinds, object.Kind())
	}
	want := []contentconfig.Kind{ContentText, ContentXObject, ContentImage}
	if len(kinds) != len(want) {
		t.Fatalf("kinds = %#v, want %#v", kinds, want)
	}
	for i := range want {
		if kinds[i] != want[i] {
			t.Fatalf("kinds = %#v, want %#v", kinds, want)
		}
	}
}

func TestPageFilterAllOmitsResourceSelectionsLikePlaya(t *testing.T) {
	d := &Document{}
	p := Page{dict: Dict{
		Name("Contents"): newStream(nil, []byte("/GS gs /CS cs /Pattern cs /P scn /S sh /Tag /Props BDC EMC")),
		Name("Resources"): Dict{
			Name("ExtGState"):  Dict{Name("GS"): Dict{Name("LW"): Number(2)}},
			Name("ColorSpace"): Dict{Name("CS"): Name("DeviceRGB")},
			Name("Pattern"):    Dict{Name("P"): Dict{Name("PatternType"): Number(1)}},
			Name("Shading"):    Dict{Name("S"): Dict{Name("ShadingType"): Number(2)}},
			Name("Properties"): Dict{Name("Props"): Dict{Name("MCID"): Number(1)}},
		},
	}}
	var kinds []contentconfig.Kind
	for object, err := range p.Interp(d, contentconfig.Options{Filter: FilterAll}) {
		if err != nil {
			t.Fatal(err)
		}
		kinds = append(kinds, object.Kind())
	}
	// Playa's LazyInterpreter applies resource operators to graphics state but
	// never yields resource selections as generic ContentObjects. The dedicated
	// page resource iterators cover those values separately.
	want := []contentconfig.Kind{}
	if len(kinds) != len(want) {
		t.Fatalf("resource kinds = %#v, want %#v", kinds, want)
	}
	for i := range want {
		if kinds[i] != want[i] {
			t.Fatalf("resource kinds = %#v, want %#v", kinds, want)
		}
	}
}

func TestPageInterpFilterXObjectYieldsOnlyDirectForms(t *testing.T) {
	inner := newStream(Dict{Name("Subtype"): Name("Form")}, []byte("0 0 m 1 1 l S"))
	outer := newStream(Dict{
		Name("Subtype"):   Name("Form"),
		Name("Resources"): Dict{Name("XObject"): Dict{Name("Inner"): inner}},
	}, []byte("/Inner Do"))
	p := Page{dict: Dict{
		Name("Contents"):  newStream(nil, []byte("/Outer Do")),
		Name("Resources"): Dict{Name("XObject"): Dict{Name("Outer"): outer}},
	}}
	direct := 0
	for object, err := range p.Interp(&Document{}, contentconfig.Options{Filter: FilterXObject}) {
		if err != nil {
			t.Fatal(err)
		}
		if object.Kind() != ContentXObject {
			t.Fatalf("filtered object kind = %v", object.Kind())
		}
		direct++
	}
	if direct != 1 {
		t.Fatalf("direct XObject count = %d, want 1", direct)
	}
}

func TestPageInterpFilterImageYieldsOnlyRenderedImages(t *testing.T) {
	image := func() Stream {
		return newStream(Dict{
			Name("Subtype"): Name("Image"), Name("Width"): Number(1),
			Name("Height"): Number(1), Name("BPC"): Number(8), Name("CS"): Name("G"),
		}, []byte{7})
	}
	p := Page{dict: Dict{
		Name("Contents"):  newStream(nil, []byte("/Used Do")),
		Name("Resources"): Dict{Name("XObject"): Dict{Name("Used"): image(), Name("Unused"): image()}},
	}}
	count := 0
	for object, err := range p.Interp(&Document{}, contentconfig.Options{Filter: FilterImage}) {
		if err != nil {
			t.Fatal(err)
		}
		if object.Kind() != ContentImage {
			t.Fatalf("filtered object kind = %v", object.Kind())
		}
		count++
	}
	if count != 1 {
		t.Fatalf("rendered image count = %d, want 1", count)
	}
}

func TestPageInterpPreservesPlayaFormMatrixAndStructParentsSemantics(t *testing.T) {
	form := newStream(Dict{
		Name("Subtype"):       Name("Form"),
		Name("Matrix"):        Array{Number(2), Number(0), Number(0), Number(3), Number(5), Number(7)},
		Name("StructParents"): Number(8),
	}, []byte("BT (A) Tj ET"))
	d := &Document{}
	page := Page{dict: Dict{
		Name("Contents"):  newStream(nil, []byte("/Fm Do")),
		Name("Resources"): Dict{Name("XObject"): Dict{Name("Fm"): form}},
	}}
	count := 0
	for object, err := range page.Interp(d, DefaultContentOptions()) {
		if err != nil {
			t.Fatal(err)
		}
		if object.Kind() != ContentXObject {
			continue
		}
		count++
		matrix, ok := object.MatrixValue()
		if !ok || matrix != (geometry.Matrix{2, 0, 0, 3, 5, 7}) {
			t.Fatalf("form matrix = %#v, present=%v", matrix, ok)
		}
		parent, err := object.ParentWithError(d)
		if err != nil || parent != nil {
			t.Fatalf("form StructParents without MCID parent = %#v, err=%v; want nil", parent, err)
		}
	}
	if count != 1 {
		t.Fatalf("form count = %d, want 1", count)
	}
}

func TestPageFlattenUsesFormStructParentsForChildContent(t *testing.T) {
	form := newStream(Dict{
		Name("Subtype"):       Name("Form"),
		Name("StructParents"): Number(8),
	}, []byte("/Span << /MCID 0 >> BDC BT (A) Tj ET EMC"))
	d := &Document{
		trailer: Dict{Name("Root"): Ref{Object: 1}},
		objects: map[Ref]Object{
			{Object: 1}: Dict{Name("Pages"): Ref{Object: 2}, Name("StructTreeRoot"): Ref{Object: 3}},
			{Object: 2}: Dict{Name("Type"): Name("Pages"), Name("Kids"): Array{Ref{Object: 9}}},
			{Object: 3}: Dict{Name("ParentTree"): Dict{Name("Nums"): Array{
				Number(7), Array{Dict{Name("S"): Name("P")}},
				Number(8), Array{Dict{Name("S"): Name("LBody")}},
			}}},
			{Object: 9}: Dict{Name("Type"): Name("Page"), Name("Parent"): Ref{Object: 2}, Name("StructParents"): Number(7)},
		},
	}
	page := Page{ref: Ref{Object: 9}, dict: Dict{
		Name("Contents"):  newStream(nil, []byte("/Fm Do")),
		Name("Resources"): Dict{Name("XObject"): Dict{Name("Fm"): form}},
	}}
	for object, err := range page.Flatten(d, DefaultContentOptions()) {
		if err != nil {
			t.Fatal(err)
		}
		if object.Kind() != ContentText {
			continue
		}
		parent, parentErr := object.ParentWithError(d)
		if parentErr != nil {
			t.Fatal(parentErr)
		}
		if parent == nil || parent.Role() != "LBody" {
			t.Fatalf("flattened Form text parent = %#v, want LBody from Form StructParents", parent)
		}
		return
	}
	t.Fatal("flattened Form yielded no text")
}

func TestPageFlattenPreservesPlayaFormParentPage(t *testing.T) {
	form := newStream(Dict{Name("Subtype"): Name("Form"), Name("StructParents"): Number(8)},
		[]byte("/Span << /MCID 0 >> BDC BT (A) Tj ET EMC"))
	d := &Document{trailer: Dict{Name("Root"): Ref{Object: 1}}, objects: map[Ref]Object{
		{Object: 1}:  Dict{Name("Pages"): Ref{Object: 2}, Name("StructTreeRoot"): Ref{Object: 3}},
		{Object: 2}:  Dict{Name("Type"): Name("Pages"), Name("Kids"): Array{Ref{Object: 9}, Ref{Object: 10}}, Name("Count"): Number(2)},
		{Object: 3}:  Dict{Name("ParentTree"): Dict{Name("Nums"): Array{Number(8), Array{Ref{Object: 4}}}}},
		{Object: 4}:  Dict{Name("Type"): Name("StructElem"), Name("S"): Name("P"), Name("Pg"): Ref{Object: 10}},
		{Object: 9}:  Dict{Name("Type"): Name("Page"), Name("Parent"): Ref{Object: 2}},
		{Object: 10}: Dict{Name("Type"): Name("Page"), Name("Parent"): Ref{Object: 2}},
	}}
	page := Page{ref: Ref{Object: 9}, dict: Dict{
		Name("Contents"):  newStream(nil, []byte("/Fm Do")),
		Name("Resources"): Dict{Name("XObject"): Dict{Name("Fm"): form}},
	}}
	for object, err := range page.Flatten(d, DefaultContentOptions()) {
		if err != nil {
			t.Fatal(err)
		}
		if object.Kind() != ContentText {
			continue
		}
		parent, err := object.ParentWithError(d)
		if err != nil || parent == nil || parent.Role() != "P" {
			t.Fatalf("form text parent = %#v, err=%v; want P", parent, err)
		}
		parentPage, err := parent.PageObject(d)
		if err != nil || parentPage.Index() != 1 {
			t.Fatalf("form text parent page = %d, err=%v; want 1", parentPage.Index(), err)
		}
		return
	}
	t.Fatal("form yielded no text")
}

func TestPageFlattenPreservesPlayaTagParents(t *testing.T) {
	d := &Document{trailer: Dict{Name("Root"): Ref{Object: 1}}, objects: map[Ref]Object{
		{Object: 1}: Dict{Name("Pages"): Ref{Object: 2}, Name("StructTreeRoot"): Ref{Object: 3}},
		{Object: 2}: Dict{Name("Type"): Name("Pages"), Name("Kids"): Array{Ref{Object: 9}}},
		{Object: 3}: Dict{Name("ParentTree"): Dict{Name("Nums"): Array{Number(7), Array{
			Dict{Name("S"): Name("P")}, Dict{Name("S"): Name("LBody")},
		}}}},
		{Object: 9}: Dict{Name("Type"): Name("Page"), Name("Parent"): Ref{Object: 2}, Name("StructParents"): Number(7)},
	}}
	page := Page{ref: Ref{Object: 9}, dict: Dict{Name("Contents"): newStream(nil,
		[]byte("/Span << /MCID 0 >> DP /Span << /MCID 1 >> DP"))}}
	var roles []string
	for object, err := range page.Flatten(d, DefaultContentOptions()) {
		if err != nil {
			t.Fatal(err)
		}
		if object.TagBorrowed() == nil {
			continue
		}
		parent, err := object.ParentWithError(d)
		if err != nil || parent == nil {
			t.Fatalf("tag parent = %#v, err=%v", parent, err)
		}
		roles = append(roles, parent.Role())
	}
	if !reflect.DeepEqual(roles, []string{"P", "LBody"}) {
		t.Fatalf("tag parents = %q, want P/LBody", roles)
	}
}

func TestPageInterpFilterImageReportsUnresolvedXObject(t *testing.T) {
	d := &Document{}
	p := Page{dict: Dict{
		Name("Contents"):  newStream(nil, []byte("/Im Do")),
		Name("Resources"): Dict{Name("XObject"): Ref{Object: 99}},
	}}
	for object, err := range p.Interp(d, contentconfig.Options{Filter: FilterImage}) {
		if err == nil || object.Kind() != "" {
			t.Fatalf("unresolved image XObject = %#v, err=%v", object, err)
		}
		return
	}
	t.Fatal("unresolved image XObject produced no error")
}

func TestPageInterpFilterImageReportsUnresolvedSubtype(t *testing.T) {
	d := &Document{objects: map[Ref]Object{{Object: 1}: newStream(Dict{Name("Subtype"): Ref{Object: 99}}, []byte{1})}}
	p := Page{dict: Dict{
		Name("Contents"):  newStream(nil, []byte("/Im Do")),
		Name("Resources"): Dict{Name("XObject"): Dict{Name("Im"): Ref{Object: 1}}},
	}}
	for object, err := range p.Interp(d, contentconfig.Options{Filter: FilterImage}) {
		if err == nil || object.Kind() != "" {
			t.Fatalf("unresolved image subtype = %#v, err=%v", object, err)
		}
		return
	}
	t.Fatal("unresolved image subtype produced no error")
}

func TestPagePathsPreserveTextLeadingAcrossFlattenedContent(t *testing.T) {
	form := newStream(Dict{Name("Subtype"): Name("Form")}, []byte("0 0 m 10 10 l S"))
	page := Page{dict: Dict{
		Name("Contents"):  newStream(nil, []byte("BT 23.34 TL ET /Fm Do")),
		Name("Resources"): Dict{Name("XObject"): Dict{Name("Fm"): form}},
	}}
	d := &Document{}
	count := 0
	for path, err := range d.PagePathsSeq(page) {
		if err != nil {
			t.Fatal(err)
		}
		count++
		if got := path.GState().Leading(); got != 23.34 {
			t.Fatalf("flattened path leading = %v, want 23.34", got)
		}
	}
	if count != 1 {
		t.Fatalf("flattened path count = %d, want 1", count)
	}
}

func TestPageImagesPreserveTextLeadingAcrossFlattenedContent(t *testing.T) {
	image := newStream(Dict{Name("Subtype"): Name("Image"), Name("Width"): Number(1), Name("Height"): Number(1), Name("BitsPerComponent"): Number(8), Name("ColorSpace"): Name("DeviceGray")}, []byte{7})
	form := newStream(Dict{Name("Subtype"): Name("Form"), Name("Resources"): Dict{Name("XObject"): Dict{Name("Im"): image}}}, []byte("/Im Do"))
	page := Page{dict: Dict{
		Name("Contents"):  newStream(nil, []byte("BT 15.6 TL ET /Fm Do")),
		Name("Resources"): Dict{Name("XObject"): Dict{Name("Fm"): form}},
	}}
	d := &Document{}
	count := 0
	for image, err := range d.PageImagesSeq(page) {
		if err != nil {
			t.Fatal(err)
		}
		count++
		if got := image.GState().Leading(); got != 15.6 {
			t.Fatalf("flattened image leading = %v, want 15.6", got)
		}
	}
	if count != 1 {
		t.Fatalf("flattened image count = %d, want 1", count)
	}
}

func TestPageFontDecodePreservesIdentityCodeBytes(t *testing.T) {
	cmap := newStream(nil, []byte("1 begincodespacerange\n<00> <ff>\nendcodespacerange\n2 begincidchar\n<00> 42\n<01> 43\nendcidchar"))
	page := Page{dict: Dict{Name("Resources"): Dict{Name("Font"): Dict{Name("F1"): Dict{
		Name("Type"): Name("Font"), Name("Subtype"): Name("Type0"), Name("BaseFont"): Name("SyntheticIdentity"), Name("Encoding"): cmap,
		Name("DescendantFonts"): Array{Dict{Name("Type"): Name("Font"), Name("Subtype"): Name("CIDFontType2"), Name("BaseFont"): Name("SyntheticIdentity"),
			Name("CIDSystemInfo"): Dict{Name("Registry"): String("Adobe"), Name("Ordering"): String("Identity"), Name("Supplement"): Number(0)},
		}},
	}}}}}
	fonts, err := page.Fonts(&Document{})
	if err != nil {
		t.Fatal(err)
	}
	font := fonts["F1"]
	if font == nil || !font.IsCID() {
		t.Fatalf("synthetic identity font = %#v", fonts)
	}
	glyphs := font.DecodeGlyphs([]byte{0, 1})
	if len(glyphs) != 2 || glyphs[0].Text() != "\x00" || glyphs[0].CID() != 42 || glyphs[1].Text() != "\x01" || glyphs[1].CID() != 43 {
		t.Fatalf("identity source-code fallback = %#v, want NUL/U+0001 from source bytes rather than CID 42/43", glyphs)
	}
}

func TestPageInterpRestrictsOpsWithoutDesynchronizingConsumers(t *testing.T) {
	d := &Document{}
	page := Page{dict: Dict{Name("Contents"): newStream(nil, []byte("BT (A) Tj ET 0 0 m 1 1 l S"))}}

	var objects []ContentObject
	for object, err := range page.Interp(d, contentconfig.Options{RestrictOps: []string{"Tj"}}) {
		if err != nil {
			t.Fatal(err)
		}
		objects = append(objects, object)
	}
	if len(objects) != 1 || objects[0].Kind() != ContentText || objects[0].text == nil || objects[0].text.Text() != "A" {
		t.Fatalf("restricted objects = %#v", objects)
	}
}

func TestPageInterpRestrictOpsPreservesInterpreterState(t *testing.T) {
	d := &Document{}
	page := Page{dict: Dict{
		Name("Contents"): newStream(nil, []byte("BT /F1 10 Tf 1 0 0 1 100 200 Tm (A) Tj ET")),
		Name("Resources"): Dict{Name("Font"): Dict{Name("F1"): Dict{
			Name("Subtype"): Name("Type1"), Name("BaseFont"): Name("Helvetica"),
		}}},
	}}
	for object, err := range page.Interp(d, contentconfig.Options{RestrictOps: []string{"Tj"}}) {
		if err != nil {
			t.Fatal(err)
		}
		if object.text == nil {
			t.Fatalf("restricted object has no text payload: %#v", object)
		}
		if object.text.FontName() != "Helvetica" || object.text.Origin() != [2]float64{100, 200} {
			t.Fatalf("restricted text state = font %q origin %v", object.text.FontName(), object.text.Origin())
		}
		return
	}
	t.Fatal("restricted text object was not emitted")
}

func TestPageInterpRestrictOpsPreservesPathState(t *testing.T) {
	d := &Document{}
	page := Page{dict: Dict{Name("Contents"): newStream(nil, []byte("q 1 0 0 1 10 20 cm 0 0 m 10 0 l S Q"))}}
	for object, err := range page.Interp(d, contentconfig.Options{Filter: FilterPath, RestrictOps: []string{"S"}}) {
		if err != nil {
			t.Fatal(err)
		}
		if object.path == nil {
			t.Fatalf("restricted object has no path payload: %#v", object)
		}
		if got := object.path.BBox(); got != [4]float64{10, 20, 20, 20} {
			t.Fatalf("restricted path bbox = %v, want [10 20 20 20]", got)
		}
		return
	}
	t.Fatal("restricted path object was not emitted")
}

func TestPageInterpRestrictOpsPreservesImageState(t *testing.T) {
	d := &Document{}
	page := Page{dict: Dict{Name("Contents"): newStream(nil, []byte("q 1 0 0 1 10 20 cm BI /W 1 /H 1 /BPC 8 /CS /G ID x EI Q"))}}
	for object, err := range page.Interp(d, contentconfig.Options{Filter: FilterImage, RestrictOps: []string{"BI"}}) {
		if err != nil {
			t.Fatal(err)
		}
		if object.image == nil {
			t.Fatalf("restricted object has no image payload: %#v", object)
		}
		if got := object.image.BBox(); got != [4]float64{10, 20, 11, 21} {
			t.Fatalf("restricted image bbox = %v, want [10 20 11 21]", got)
		}
		return
	}
	t.Fatal("restricted image object was not emitted")
}

func TestPageInterpFilterAllRestrictOpsPreservesTagState(t *testing.T) {
	d := &Document{}
	page := Page{dict: Dict{Name("Contents"): newStream(nil, []byte("1 0 0 1 10 20 cm /Point MP"))}}
	for object, err := range page.Interp(d, contentconfig.Options{Filter: FilterAll, RestrictOps: []string{"MP"}}) {
		if err != nil {
			t.Fatal(err)
		}
		if object.tag == nil {
			t.Fatalf("restricted FilterAll object has no tag payload: %#v", object)
		}
		if got := object.tag.GState().CTM(); got != (geometry.Matrix{1, 0, 0, 1, 10, 20}) {
			t.Fatalf("restricted FilterAll tag CTM = %v, want [1 0 0 1 10 20]", got)
		}
		return
	}
	t.Fatal("restricted FilterAll tag object was not emitted")
}

func TestPageInterpCopiesRestrictOpsBeforeIteration(t *testing.T) {
	d := &Document{}
	page := Page{dict: Dict{Name("Contents"): newStream(nil, []byte("BT (A) Tj ET 0 0 m 1 1 l S"))}}
	restrictOps := []string{"Tj"}
	sequence := page.Interp(d, contentconfig.Options{RestrictOps: restrictOps})
	restrictOps[0] = "S"

	var objects []ContentObject
	for object, err := range sequence {
		if err != nil {
			t.Fatal(err)
		}
		objects = append(objects, object)
	}
	if len(objects) != 1 || objects[0].Kind() != ContentText {
		t.Fatalf("restrict ops were not snapshotted: %#v", objects)
	}
}

func TestPageInterpStopsBeforeMalformedLaterStream(t *testing.T) {
	d := &Document{}
	page := Page{dict: Dict{Name("Contents"): Array{
		newStream(nil, []byte("BT (A) Tj ET")),
		newStream(nil, []byte("(")),
	}}}

	count := 0
	for object, err := range page.Interp(d, contentconfig.Options{}) {
		if err != nil {
			t.Fatal(err)
		}
		if object.Kind() != ContentText {
			t.Fatalf("object = %#v", object)
		}
		count++
		break
	}
	if count != 1 {
		t.Fatalf("consumed %d objects, want 1", count)
	}
}
