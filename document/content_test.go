package document

import (
	"bytes"
	"encoding/json"
	"math"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"unsafe"

	"github.com/lin-string/go-playa/cacheconfig"
	"github.com/lin-string/go-playa/contentconfig"
	"github.com/lin-string/go-playa/contentdata"
	"github.com/lin-string/go-playa/geometry"
)

func TestZeroXObjectFinalizeDoesNotPanic(t *testing.T) {
	if got := (XObjectObject{}).Finalize(); got.data.StreamBorrowed().DataBorrowed() != nil || got.data.StreamBorrowed().DictBorrowed() != nil {
		t.Fatalf("zero XObject stream = %#v", got.data.StreamBorrowed())
	}
}

func TestGlyphObjectKeepsSharedContextOutOfPerGlyphStorage(t *testing.T) {
	const maxGlyphObjectBytes = 224
	if size := unsafe.Sizeof(GlyphObject{}); size > maxGlyphObjectBytes {
		t.Fatalf("GlyphObject size = %d bytes; want at most %d so shared graphics state is not stored per glyph", size, maxGlyphObjectBytes)
	}
}

func TestInterpretTextBuildsLongTextWithoutQuadraticAllocation(t *testing.T) {
	skipAllocationCheckUnderRace(t)
	const glyphCount = 8192
	ops, err := ParseContent([]byte("BT (" + strings.Repeat("A", glyphCount) + ") Tj ET"))
	if err != nil {
		t.Fatal(err)
	}
	runtime.GC()
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	texts := InterpretText(ops)
	runtime.ReadMemStats(&after)
	if len(texts) != 1 || texts[0].Len() != glyphCount || len(texts[0].Text()) != glyphCount {
		t.Fatalf("long text result = %d texts, %d glyphs, %d bytes", len(texts), texts[0].Len(), len(texts[0].Text()))
	}
	const allocationLimit = 8 << 20
	if allocated := after.TotalAlloc - before.TotalAlloc; allocated > allocationLimit {
		t.Fatalf("interpreting %d glyphs allocated %d bytes, want at most %d", glyphCount, allocated, allocationLimit)
	}
}

func TestInterpretTextBuildsFragmentedTJWithBoundedAllocation(t *testing.T) {
	skipAllocationCheckUnderRace(t)
	const glyphCount = 4096
	ops, err := ParseContent([]byte("BT [" + strings.Repeat("(A) ", glyphCount) + "] TJ ET"))
	if err != nil {
		t.Fatal(err)
	}
	runtime.GC()
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	texts := InterpretText(ops)
	runtime.ReadMemStats(&after)
	if len(texts) != 1 {
		t.Fatalf("fragmented TJ result = %d texts, want 1", len(texts))
	}
	if texts[0].Len() != glyphCount || texts[0].Text() != strings.Repeat("A", glyphCount) {
		t.Fatalf("fragmented TJ result = %d glyphs, want %d", texts[0].Len(), glyphCount)
	}
	const allocationLimit = 2 << 20
	if allocated := after.TotalAlloc - before.TotalAlloc; allocated > allocationLimit {
		t.Fatalf("interpreting %d fragmented TJ glyphs allocated %d bytes, want at most %d", glyphCount, allocated, allocationLimit)
	}
}

func TestPlayaGlyphMatrixOriginPreservesComposedTranslationRounding(t *testing.T) {
	render := geometry.Matrix{
		0, -0.11999999951999998,
		0.11999999951999998, 0,
		824.8399966966402, 329.11999867951994,
	}
	x, y, ok := playaGlyphMatrixOrigin(render, 567.554596644, 0, 0)
	if !ok {
		t.Fatal("finite Playa glyph origin was rejected")
	}
	if x != 824.8399966966402 || y != 261.01344735466614 {
		t.Fatalf("glyph matrix origin = (%0.17g, %0.17g), want pinned Playa rounding", x, y)
	}
}

func TestXObjectDecodedBufferUsesStreamFilters(t *testing.T) {
	x := testXObjectStream(newStream(Dict{Name("Filter"): Name("ASCIIHexDecode")}, []byte("68656c6c6f>")))
	decoded, err := x.DecodedBufferWithError()
	if err != nil || string(decoded) != "hello" {
		t.Fatalf("decoded XObject stream = %q, err=%v", decoded, err)
	}
	decoded[0] = 'X'
	if got := string(x.Buffer()); got != "68656c6c6f>" {
		t.Fatalf("raw XObject stream changed after decoded access: %q", got)
	}
	if got := string(x.DecodedBuffer()); got != "hello" {
		t.Fatalf("compatibility decoded XObject stream = %q", got)
	}
}

func TestTextAndGlyphSnapshotsPreserveEmptySlices(t *testing.T) {
	text := newTestText(contentdata.TextSpec{
		StrokeColor: make([]float64, 0), NonStrokeColor: make([]float64, 0),
	}, nil, []GlyphObject{newTestGlyph(contentdata.GlyphSpec{Code: make([]byte, 0)}, nil, nil, geometry.Matrix{})})
	glyph := newTestGlyph(contentdata.GlyphSpec{MarkedStack: []MarkedContentContext{newMarkedContentContext("Span", Dict{Name("Role"): String("code")}, "", 0, false, nil).data}}, nil, nil, geometry.Matrix{})
	stack := glyph.MarkedStackCopy()
	if len(stack) != 1 || stack[0].Tag() != "Span" {
		t.Fatalf("glyph marked stack = %#v", stack)
	}
	stackProperties := stack[0].PropertiesCopy()
	stackProperties[Name("Role")] = String("changed")
	if !reflect.DeepEqual(glyph.MarkedStackCopy()[0].PropertiesCopy()[Name("Role")], String("code")) {
		t.Fatal("glyph marked stack copy shares properties")
	}
	snapshot := text.Finalize()
	if snapshot.StrokeColorCopy() == nil || snapshot.NonStrokeColorCopy() == nil || snapshot.glyphs[0].Codes() == nil {
		t.Fatalf("empty text slices were not preserved: %#v", snapshot)
	}
	if glyph := newTestGlyph(contentdata.GlyphSpec{Code: make([]byte, 0)}, nil, nil, geometry.Matrix{}).Finalize(); glyph.Codes() == nil {
		t.Fatal("empty glyph code was not preserved")
	}
	if codes := newTestGlyph(contentdata.GlyphSpec{Code: make([]byte, 0)}, nil, nil, geometry.Matrix{}).Codes(); codes == nil {
		t.Fatal("empty glyph Codes result was collapsed to nil")
	}
	if color := text.StrokeColorCopy(); color == nil {
		t.Fatal("empty stroke color copy was collapsed to nil")
	}
	if color := text.NonStrokeColorCopy(); color == nil {
		t.Fatal("empty non-stroke color copy was collapsed to nil")
	}
}

func TestContentOpOperandsCopyPreservesAbsentOperands(t *testing.T) {
	if operands := (ContentOp{}).OperandsCopy(); operands != nil {
		t.Fatalf("absent operands copy = %#v, want nil", operands)
	}
	if operands := newContentOpBorrowed("", make([]Object, 0), 0).OperandsCopy(); operands == nil {
		t.Fatal("allocated empty operands copy became nil")
	}
}

func TestXObjectResourcesWithErrorReportsMalformedIndirectResource(t *testing.T) {
	d := &Document{objects: map[Ref]Object{{Object: 1}: Number(7)}}
	x := testXObjectStream(newStream(Dict{Name("Resources"): Ref{Object: 1}}, nil))
	if resources, err := x.ResourcesWithError(d); err == nil || resources != nil {
		t.Fatalf("malformed XObject resources = %#v, err=%v", resources, err)
	}
}

func TestXObjectResourcesWithErrorCachesTerminalErrors(t *testing.T) {
	ref := Ref{Object: 42}
	d := &Document{}
	x := testXObjectRefStream(ref, newStream(Dict{Name("Resources"): Number(1)}, nil))

	_, firstErr := x.ResourcesWithError(d)
	_, secondErr := x.ResourcesWithError(d)
	if firstErr == nil || secondErr == nil {
		t.Fatalf("resource errors = %v, %v; want both non-nil", firstErr, secondErr)
	}
	if firstErr != secondErr {
		t.Fatalf("resource error was not cached: first=%p second=%p", firstErr, secondErr)
	}
	if got := d.xobjectResourceErrors[ref]; got != firstErr {
		t.Fatalf("cached XObject resource error = %v, want %v", got, firstErr)
	}
}

func TestXObjectResourcesWithErrorHonorsErrorCacheBudget(t *testing.T) {
	ref := Ref{Object: 42}
	d := &Document{cacheOptionsConfigured: true, cacheOptions: cacheconfig.Options{XObjectResourceErrorBytes: 0}}
	x := testXObjectRefStream(ref, newStream(Dict{Name("Resources"): Number(1)}, nil))
	_, firstErr := x.ResourcesWithError(d)
	_, secondErr := x.ResourcesWithError(d)
	if firstErr == nil || secondErr == nil || firstErr == secondErr {
		t.Fatalf("resource errors = %v/%v, want uncached independent errors", firstErr, secondErr)
	}
	if len(d.xobjectResourceErrors) != 0 {
		t.Fatalf("XObject resource error cache retained entry with zero budget: %#v", d.xobjectResourceErrors)
	}
}

func TestXObjectResourcesWithErrorReportsUnresolvedEntryInSnapshot(t *testing.T) {
	d := &Document{}
	x := testXObject(contentdata.XObjectSpec{Resources: Dict{
		Name("Font"): Dict{Name("F1"): Ref{Object: 99}},
	}})
	if resources, err := x.ResourcesWithError(d); err == nil || resources != nil {
		t.Fatalf("unresolved snapshot resource = %#v, err=%v", resources, err)
	}
}

func TestXObjectResourcesWithErrorRejectsNilDocument(t *testing.T) {
	x := testXObject(contentdata.XObjectSpec{Resources: Dict{}})
	if resources, err := x.ResourcesWithError(nil); err != errNilDocument || resources != nil {
		t.Fatalf("nil document resources = %#v, err=%v", resources, err)
	}
}

func TestXObjectResourcesWithErrorValidatesStreamResources(t *testing.T) {
	d := &Document{}
	x := testXObjectStream(newStream(Dict{
		Name("Resources"): Dict{Name("Font"): Dict{Name("F1"): Ref{Object: 99}}},
	}, nil))
	if resources, err := x.ResourcesWithError(d); err == nil || resources != nil {
		t.Fatalf("malformed stream resources = %#v, err=%v", resources, err)
	}
}

func TestXObjectResourcesCacheSuccessfulResolution(t *testing.T) {
	d := &Document{objects: map[Ref]Object{{Object: 1}: Dict{Name("Font"): Dict{Name("F1"): Dict{}}}}}
	x := testXObjectRefStream(Ref{Object: 7}, newStream(Dict{Name("Resources"): Ref{Object: 1}}, nil))
	first, err := x.ResourcesWithError(d)
	if err != nil || first == nil {
		t.Fatalf("first XObject resources = %#v, err=%v", first, err)
	}
	d.objects[Ref{Object: 1}] = Dict{Name("Font"): Dict{Name("F2"): Dict{}}}
	second, err := x.ResourcesWithError(d)
	if err != nil || second[Name("Font")].(Dict)[Name("F1")] == nil {
		t.Fatalf("cached XObject resources = %#v, err=%v", second, err)
	}

	uncached := &Document{cacheOptions: cacheconfig.Options{}, cacheOptionsConfigured: true, objects: map[Ref]Object{{Object: 1}: Dict{Name("Font"): Dict{}}}}
	uncachedX := testXObjectRefStream(Ref{Object: 7}, newStream(Dict{Name("Resources"): Ref{Object: 1}}, nil))
	if _, err := uncachedX.ResourcesWithError(uncached); err != nil {
		t.Fatal(err)
	}
	if len(uncached.xobjectResourceCache) != 0 {
		t.Fatalf("zero-budget XObject resources entered cache: %#v", uncached.xobjectResourceCache)
	}
}

func TestXObjectFlattenReusesValidatedResources(t *testing.T) {
	form := newStream(Dict{Name("Subtype"): Name("Form"), Name("Resources"): Ref{Object: 1}}, []byte("q Q"))
	d := &Document{objects: map[Ref]Object{
		{Object: 1}: Dict{},
		{Object: 7}: form,
	}}
	x := testXObjectRefStream(Ref{Object: 7}, form)
	if _, err := x.ResourcesWithError(d); err != nil {
		t.Fatal(err)
	}
	d.objects[Ref{Object: 1}] = Ref{Object: 99}
	for _, err := range x.Flatten(d, contentconfig.Options{Filter: FilterAll}) {
		if err != nil {
			t.Fatalf("XObject Flatten did not reuse validated resources: %v", err)
		}
	}
}

func TestXObjectContentsReusesValidatedResources(t *testing.T) {
	form := newStream(Dict{Name("Subtype"): Name("Form"), Name("Resources"): Ref{Object: 1}}, []byte("q Q"))
	d := &Document{objects: map[Ref]Object{
		{Object: 1}: Dict{},
		{Object: 7}: form,
	}}
	x := testXObjectRefStream(Ref{Object: 7}, form)
	if _, err := x.ResourcesWithError(d); err != nil {
		t.Fatal(err)
	}
	d.objects[Ref{Object: 1}] = Ref{Object: 99}
	for _, err := range x.Contents(d) {
		if err != nil {
			t.Fatalf("XObject Contents did not reuse validated resources: %v", err)
		}
	}
}

func TestPageFlattenReusesCachedFormResources(t *testing.T) {
	form := newStream(Dict{Name("Subtype"): Name("Form"), Name("Resources"): Ref{Object: 2}}, []byte("q Q"))
	d := &Document{objects: map[Ref]Object{
		{Object: 1}: Dict{Name("Fm"): Ref{Object: 7}},
		{Object: 2}: Dict{},
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
	for _, err := range page.Flatten(d, contentconfig.Options{Filter: FilterAll}) {
		if err != nil {
			t.Fatalf("Page Flatten did not reuse cached Form resources: %v", err)
		}
	}
}

func TestResourceContentObjectsHaveStableProjectionSemantics(t *testing.T) {
	objects := []ContentObject{
		{kind: ContentExtGState, extgstate: func() *ExtGStateObject {
			value := newTestExtGState(contentdata.ExtGStateSpec{Name: "GS", Dict: Dict{Name("LW"): Number(2)}})
			return &value
		}()},
		{kind: ContentColorSpace, colorspace: func() *ColorSpaceObject {
			value := ColorSpaceObject{data: contentdata.NewColorSpaceSelection(contentdata.ColorSpaceSelectionSpec{Name: "CS", Spec: Dict{Name("Marker"): String("original")}}), info: newImageColorSpace("DeviceRGB", 3)}
			return &value
		}()},
		{kind: ContentPattern, pattern: func() *PatternObject {
			value := PatternObject{data: contentdata.NewPattern(contentdata.PatternSpec{Name: "P", Dict: Dict{Name("Marker"): String("original")}})}
			return &value
		}()},
		{kind: ContentShading, shading: func() *ShadingObject {
			value := ShadingObject{data: contentdata.NewShading(contentdata.ShadingSpec{Name: "S", Dict: Dict{Name("Marker"): String("original")}})}
			return &value
		}()},
		{kind: ContentProperties, properties: func() *PropertiesObject {
			value := newPropertiesObject("Props", "", Ref{}, false, Dict{Name("Marker"): String("original")})
			return &value
		}()},
	}
	for _, object := range objects {
		if object.ObjectType() != string(object.Kind()) {
			t.Fatalf("object type = %q, want %q", object.ObjectType(), object.Kind())
		}
		if bbox, ok := object.BBoxValue(); ok || bbox != [4]float64{} {
			t.Fatalf("resource bbox = %#v, %v", bbox, ok)
		}
		snapshot := object.Finalize()
		encoded, err := json.Marshal(snapshot)
		if err != nil {
			t.Fatalf("marshal %s: %v", object.Kind(), err)
		}
		if !bytes.Contains(encoded, []byte(`"Kind"`)) {
			t.Fatalf("resource JSON omitted kind: %s", encoded)
		}
	}
	copy := objects[0].ExtGStateCopy()
	copyDict := copy.DictCopy()
	copyDict[Name("LW")] = Number(9)
	if objects[0].ExtGStateCopy().DictCopy()[Name("LW")] != Number(2) {
		t.Fatal("resource Finalize shares ExtGState dictionary")
	}
}

func TestContentObjectExposesOwnedBaseState(t *testing.T) {
	state := newGraphicsState()
	state.ctm = geometry.Matrix{2, 0, 0, 3, 10, 20}
	markedStack := []markedContentContext{newMarkedContentContext("P", Dict{Name("Role"): String("body")}, "", 0, false, nil)}
	text := newTestTextWithMarkedStack(contentdata.TextSpec{GState: state.publicValue()}, markedStack, nil, nil)
	object := ContentObject{kind: ContentText, text: &text}

	stateCopy := object.GraphicsStateCopy()
	matrixCopy := stateCopy.CTM()
	matrixCopy[0] = 99
	if text.GState().CTM()[0] != 2 {
		t.Fatalf("graphics state copy changed source: %v", text.GState().CTM())
	}
	matrix, ok := object.MatrixValue()
	if !ok || matrix != text.GState().CTM() {
		t.Fatalf("matrix = %v, %v; want %v, true", matrix, ok, text.GState().CTM())
	}
	stack := object.MarkedStackCopy()
	if len(stack) != 1 || stack[0].Tag() != "P" {
		t.Fatalf("marked stack = %#v", stack)
	}
	stackProperties := stack[0].PropertiesCopy()
	stackProperties[Name("Role")] = String("changed")
	if !reflect.DeepEqual(text.markedStack[0].PropertiesCopy()[Name("Role")], String("body")) {
		t.Fatal("marked stack copy shares properties")
	}
	if matrix, ok := (ContentObject{kind: ContentExtGState}).MatrixValue(); ok || matrix != (geometry.Matrix{}) {
		t.Fatalf("resource matrix = %v, %v; want zero, false", matrix, ok)
	}
}

func TestNumbersIgnoresNonFiniteOperands(t *testing.T) {
	got := numbers([]Object{Number(1), Number(math.Inf(1)), Number(math.NaN()), Number(2)})
	if !reflect.DeepEqual(got, []float64{1, 2}) {
		t.Fatalf("finite numbers = %v, want [1 2]", got)
	}
}

func TestInterpretSequencesAreRepeatableAndStopEarly(t *testing.T) {
	ops, err := ParseContent([]byte("BT 10 Tf (A) Tj (B) Tj ET 0 0 m 1 0 l S 2 0 m 3 0 l S"))
	if err != nil {
		t.Fatal(err)
	}
	var firstText []string
	for text := range InterpretTextSeq(ops) {
		firstText = append(firstText, text.Text())
		break
	}
	if len(firstText) != 1 || firstText[0] != "A" {
		t.Fatalf("early text sequence = %#v", firstText)
	}
	var allText []string
	for text := range InterpretTextSeq(ops) {
		allText = append(allText, text.Text())
	}
	if len(allText) != 2 || allText[0] != "A" || allText[1] != "B" {
		t.Fatalf("repeatable text sequence = %#v", allText)
	}
	var allPaths int
	for range InterpretPathsSeq(ops) {
		allPaths++
	}
	if allPaths != 2 {
		t.Fatalf("repeatable path sequence count = %d", allPaths)
	}
}

func TestContentOpOperandsSeqIsBorrowedAndInterruptible(t *testing.T) {
	op := newContentOpBorrowed("", []Object{Number(1), String("two")}, 0)
	var got []Object
	for operand := range op.OperandsSeq() {
		got = append(got, operand)
		break
	}
	if len(got) != 1 || got[0] != Number(1) {
		t.Fatalf("operands sequence = %#v, want first operand only", got)
	}
}

func TestDecodedStreamErrorCacheIsBounded(t *testing.T) {
	d := &Document{decodedStreamErrors: map[Ref]error{}}
	for i := 0; i < 4096; i++ {
		ref := Ref{Object: i + 1}
		_, err := decodeContentStream(d, ref, newStream(Dict{Name("Filter"): Name("UnknownDecode")}, nil))
		if err == nil {
			t.Fatal("unknown stream filter was accepted")
		}
	}
	if d.decodedStreamErrorBytes > decodedStreamErrorLimit {
		t.Fatalf("decoded stream error cache bytes = %d, want <= %d", d.decodedStreamErrorBytes, decodedStreamErrorLimit)
	}
	if len(d.decodedStreamErrors) >= 4096 {
		t.Fatalf("decoded stream error cache retained every failure: %d", len(d.decodedStreamErrors))
	}
}

func TestTextObjectArgsSeqIsBorrowedAndInterruptible(t *testing.T) {
	text := newTestText(contentdata.TextSpec{Args: []Object{String("A"), Number(2)}}, nil, nil)
	var got []Object
	for arg := range text.ArgsSeq() {
		got = append(got, arg)
		break
	}
	if len(got) != 1 || string(got[0].(String)) != "A" {
		t.Fatalf("text args sequence = %#v, want first argument only", got)
	}
}

func TestContentObjectCloneDoesNotShareTextAndPathFields(t *testing.T) {
	textValue := newTestTextWithMarkedStack(contentdata.TextSpec{
		Args:             []Object{Dict{Name("Value"): String("original")}},
		MarkedProperties: Dict{Name("Value"): String("original")},
	}, []markedContentContext{newMarkedContentContext("", Dict{Name("Value"): String("original")}, "", 0, false, nil)},
		&Font{encoding: map[byte]rune{1: 'A'}},
		[]GlyphObject{newTestGlyph(contentdata.GlyphSpec{Code: []byte{1}, GState: newGraphicsState().publicValue()}, nil, nil, geometry.Matrix{})})
	text := &textValue
	pathValue := newPathObject(contentdata.PathSpec{
		Segments:         []geometry.PathSegment{geometry.NewPathSegment("m", [2]float64{1, 2})},
		MarkedProperties: Dict{Name("Value"): String("original")},
		MarkedStack:      []MarkedContentContext{newMarkedContentContext("", Dict{Name("Value"): String("original")}, "", 0, false, nil).data},
	})
	path := &pathValue
	clone := ContentObject{text: text, path: path}.Finalize()
	clone.text.data.ArgsBorrowed()[0].(Dict)[Name("Value")] = String("changed")
	cloneGlyphCode := clone.text.glyphs[0].Codes()
	cloneGlyphCode[0] = 9
	clone.text.font.encoding[1] = 'B'
	setTestTextData(clone.text, func(spec *contentdata.TextSpec) { spec.MarkedProperties[Name("Value")] = String("changed") })
	cloneTextMarkedProperties := clone.text.markedStack[0].PropertiesCopy()
	cloneTextMarkedProperties[Name("Value")] = String("changed")
	clonePoints := clone.path.SegmentsCopy()[0].PointsCopy()
	clonePoints[0][0] = 9
	clonePathProperties := clone.path.MarkedPropertiesCopy()
	clonePathProperties[Name("Value")] = String("changed")
	clonePathMarkedProperties := clone.path.MarkedStackCopy()[0].PropertiesCopy()
	clonePathMarkedProperties[Name("Value")] = String("changed")
	if string(text.ArgsCopy()[0].(Dict)[Name("Value")].(String)) != "original" || text.glyphs[0].Codes()[0] != 1 || text.font.encoding[1] != 'A' || string(text.MarkedPropertiesCopy()[Name("Value")].(String)) != "original" || string(text.MarkedStackCopy()[0].PropertiesCopy()[Name("Value")].(String)) != "original" {
		t.Fatalf("text payload was exposed: %#v", text)
	}
	if path.SegmentsCopy()[0].PointsCopy()[0][0] != 1 || string(path.MarkedPropertiesCopy()[Name("Value")].(String)) != "original" || string(path.MarkedStackCopy()[0].PropertiesCopy()[Name("Value")].(String)) != "original" {
		t.Fatalf("path payload was exposed: %#v", path)
	}
}

func TestXObjectConvenienceSequencesPreservePageOwnership(t *testing.T) {
	d := &Document{}
	x := testXObject(contentdata.XObjectSpec{Page: Ref{Object: 7}, HasPage: true, Stream: newStream(nil, []byte("BT /F1 10 Tf (A) Tj ET 0 0 m 1 1 l S"))})
	for text, err := range x.Texts(d) {
		if err != nil {
			t.Fatal(err)
		}
		if !text.HasPage() || text.Page() != x.Page() {
			t.Fatalf("xobject text page = %#v", text)
		}
		break
	}
	for path, err := range x.Paths(d) {
		if err != nil {
			t.Fatal(err)
		}
		if !path.HasPage() || path.Page() != x.Page() {
			t.Fatalf("xobject path page = %#v", path)
		}
		return
	}
	t.Fatal("no xobject path")
}

func TestXObjectStructureSequenceUsesParentKey(t *testing.T) {
	d := &Document{
		trailer: Dict{Name("Root"): Ref{Object: 1}},
		objects: map[Ref]Object{
			{Object: 1}: Dict{Name("StructTreeRoot"): Ref{Object: 2}},
			{Object: 2}: Dict{Name("ParentTree"): Dict{Name("Nums"): Array{
				Number(4), Array{Dict{Name("S"): Name("Figure")}},
			}}},
		},
	}
	x := testXObject(contentdata.XObjectSpec{ParentKey: 4, HasParentKey: true})
	for entry, err := range x.StructureSeq(d) {
		if err != nil {
			t.Fatal(err)
		}
		if entry.Index() != 0 || entry.element == nil || entry.element.Role() != "Figure" {
			t.Fatalf("xobject structure entry = %#v", entry)
		}
		return
	}
	t.Fatal("no xobject structure entry")
}

func TestXObjectStructureIgnoresSingularStructParent(t *testing.T) {
	d := &Document{
		trailer: Dict{Name("Root"): Ref{Object: 1}},
		objects: map[Ref]Object{
			{Object: 1}: Dict{Name("StructTreeRoot"): Ref{Object: 2}},
			{Object: 2}: Dict{Name("ParentTree"): Dict{Name("Nums"): Array{
				Number(4), Array{Dict{Name("S"): Name("Figure")}},
			}}},
		},
	}
	x := testXObject(contentdata.XObjectSpec{
		ParentKey: 4, HasParentKey: true,
		Stream: newStream(Dict{Name("StructParent"): Number(4)}, nil),
	})
	structure, err := x.Structure(d)
	if err != nil {
		t.Fatal(err)
	}
	if structure.Len() != 0 {
		t.Fatalf("singular StructParent produced %d structure slots", structure.Len())
	}
	for entry, err := range x.StructureSeq(d) {
		if err != nil {
			t.Fatal(err)
		}
		t.Fatalf("singular StructParent produced structure entry %#v", entry)
	}
}

func TestXObjectStructureDoesNotReuseOwningPageCache(t *testing.T) {
	d := &Document{
		trailer: Dict{Name("Root"): Ref{Object: 1}},
		objects: map[Ref]Object{
			{Object: 1}: Dict{Name("StructTreeRoot"): Ref{Object: 2}},
			{Object: 2}: Dict{Name("ParentTree"): Dict{Name("Nums"): Array{
				Number(4), Array{Dict{Name("S"): Name("PagePart")}},
				Number(5), Array{Dict{Name("S"): Name("FormPart")}},
			}}},
		},
	}
	page := Page{ref: Ref{Object: 9}, dict: Dict{Name("StructParents"): Number(4)}}
	if _, err := page.Structure(d); err != nil {
		t.Fatal(err)
	}
	x := testXObject(contentdata.XObjectSpec{
		Page: Ref{Object: 9}, HasPage: true, ParentKey: 5, HasParentKey: true,
		Stream: newStream(Dict{Name("StructParents"): Number(5)}, nil),
	})
	structure, err := x.Structure(d)
	if err != nil {
		t.Fatal(err)
	}
	elements, err := structure.ElementsCopyWithError()
	if err != nil || len(elements) != 1 || elements[0].Role() != "FormPart" {
		t.Fatalf("xobject structure = %#v, err=%v", elements, err)
	}
}

func TestXObjectStructureBuildsParentTreeView(t *testing.T) {
	d := &Document{
		trailer: Dict{Name("Root"): Ref{Object: 1}},
		objects: map[Ref]Object{
			{Object: 1}: Dict{Name("StructTreeRoot"): Ref{Object: 2}},
			{Object: 2}: Dict{Name("ParentTree"): Dict{Name("Nums"): Array{
				Number(4), Array{Dict{Name("S"): Name("Figure")}},
			}}},
		},
	}
	x := testXObject(contentdata.XObjectSpec{ParentKey: 4, HasParentKey: true})
	structure, err := x.Structure(d)
	if err != nil || len(structure.elements) != 1 || structure.elements[0].Role() != "Figure" {
		t.Fatalf("xobject structure = %#v, err=%v", structure, err)
	}
}

func TestXObjectMarkedContentLookupUsesDirectContent(t *testing.T) {
	d := &Document{}
	x := testXObject(contentdata.XObjectSpec{Page: Ref{Object: 8}, HasPage: true, Stream: newStream(nil, []byte(
		"/Span << /MCID 2 >> BDC (A) Tj EMC",
	))})
	items, err := x.MarkedContentIndex(d)
	if err != nil {
		t.Fatal(err)
	}
	if len(items.roots) != 1 || len(items.byMCID[2]) != 1 || !items.roots[0].HasPage() || items.roots[0].Page() != x.Page() {
		t.Fatalf("xobject marked content = %#v", items)
	}
	count := 0
	for item, err := range x.MarkedContentByMCIDSeq(d, 2) {
		if err != nil {
			t.Fatal(err)
		}
		if item.MCID() != 2 {
			t.Fatalf("xobject MCID item = %#v", item)
		}
		count++
	}
	if count != 1 {
		t.Fatalf("xobject MCID count = %d", count)
	}
}

func TestContentObjectDelegatesPageAndParentLookups(t *testing.T) {
	text := newTestText(contentdata.TextSpec{Page: Ref{Object: 9}, HasPage: true}, nil, nil)
	object := ContentObject{kind: ContentText, text: &text}
	if _, err := object.PageObject(nil); err != errNilDocument {
		t.Fatalf("content page error = %v", err)
	}
	if object.Parent(&Document{}) != nil {
		t.Fatal("content object unexpectedly has parent")
	}
	if _, err := (ContentObject{}).PageObject(&Document{}); err != ErrPageNotFound {
		t.Fatalf("empty content page error = %v", err)
	}
}

func TestContentObjectExposesCommonContentSemantics(t *testing.T) {
	textValue := newTestTextWithMarkedStack(contentdata.TextSpec{
		BBox: [4]float64{1, 2, 3, 4}, MCID: 2, HasMCID: true,
	}, []markedContentContext{newMarkedContentContext("Artifact", nil, "", 0, false, nil), newMarkedContentContext("P", nil, "", 7, true, nil)}, nil, nil)
	object := ContentObject{kind: ContentText, text: &textValue}
	if object.ObjectType() != "text" {
		t.Fatalf("object type = %q", object.ObjectType())
	}
	if bbox, ok := object.BBoxValue(); !ok || bbox != [4]float64{1, 2, 3, 4} {
		t.Fatalf("bbox = %v, present=%v", bbox, ok)
	}
	context := object.MarkedContext()
	if context == nil || context.Tag() != "P" {
		t.Fatalf("marked context = %#v", context)
	}
	if mcid, ok := object.MCIDValue(); !ok || mcid != 7 {
		t.Fatalf("mcid = %d, present=%v", mcid, ok)
	}
}

func TestContentParentsAcceptNilDocument(t *testing.T) {
	page := Ref{Object: 1}
	if newTestText(contentdata.TextSpec{Page: page, HasPage: true, MCID: 1, HasMCID: true}, nil, nil).Parent(nil) != nil {
		t.Fatal("text parent resolved with nil document")
	}
	if newTestGlyph(contentdata.GlyphSpec{Page: page, HasPage: true, MCID: 1, HasMCID: true}, nil, nil, geometry.Matrix{}).Parent(nil) != nil {
		t.Fatal("glyph parent resolved with nil document")
	}
	if newPathObject(contentdata.PathSpec{Page: page, HasPage: true, MCID: 1, HasMCID: true}).Parent(nil) != nil {
		t.Fatal("path parent resolved with nil document")
	}
	if newMarkedContentValue("", page, true, 1, true, "", nil, nil).Parent(nil) != nil {
		t.Fatal("marked-content parent resolved with nil document")
	}
	if newTagObject(contentdata.TagSpec{Page: page, HasPage: true, MCID: 1, HasMCID: true}).Parent(nil) != nil {
		t.Fatal("tag parent resolved with nil document")
	}
}

func TestTextObjectGlyphsSequenceIsRepeatableAndStopsEarly(t *testing.T) {
	font := &Font{encoding: map[byte]rune{1: 'A'}}
	text := TextObject{glyphs: []GlyphObject{newTestGlyph(contentdata.GlyphSpec{Text: "A"}, font, nil, geometry.Matrix{}), newTestGlyph(contentdata.GlyphSpec{Text: "B"}, nil, nil, geometry.Matrix{})}}
	var first []string
	for glyph, err := range text.GlyphsSeq() {
		if err != nil {
			t.Fatal(err)
		}
		first = append(first, glyph.Text())
		break
	}
	if len(first) != 1 || first[0] != "A" {
		t.Fatalf("early glyphs = %#v", first)
	}
	for glyph, err := range text.GlyphsSeq() {
		if err != nil {
			t.Fatal(err)
		}
		snapshot := glyph.Finalize()
		snapshot.font.encoding[1] = 'B'
		break
	}
	if font.encoding[1] != 'A' {
		t.Fatal("glyph sequence exposed the font cache")
	}
	var second []string
	for glyph, err := range text.GlyphsSeq() {
		if err != nil {
			t.Fatal(err)
		}
		second = append(second, glyph.Text())
	}
	if len(second) != 2 || second[0] != "A" || second[1] != "B" {
		t.Fatalf("repeated glyphs = %#v", second)
	}
}

func TestInterpretTextObjectExposesOriginAndDisplacement(t *testing.T) {
	ops, err := ParseContent([]byte("BT /F1 10 Tf 1 0 0 1 100 200 Tm [(A) 500 (B)] TJ ET"))
	if err != nil {
		t.Fatal(err)
	}
	got := InterpretText(ops)
	if len(got) != 1 {
		t.Fatalf("text objects = %#v", got)
	}
	if got[0].Origin() != [2]float64{100, 200} {
		t.Fatalf("origin = %v, want [100 200]", got[0].Origin())
	}
	if got[0].Displacement() != [2]float64{5, 0} {
		t.Fatalf("displacement = %v, want TJ-adjusted [5 0]", got[0].Displacement())
	}
}

func TestInterpretTextObjectExposesFontDerivedProperties(t *testing.T) {
	ops, err := ParseContent([]byte("BT /F1 12 Tf (A) Tj ET"))
	if err != nil {
		t.Fatal(err)
	}
	font := NewSimpleFont("ABCDEF+Helvetica")
	got := InterpretTextWithFonts(ops, map[string]*Font{"F1": font})
	if len(got) != 1 {
		t.Fatalf("text objects = %#v", got)
	}
	if got[0].Size() != 12 {
		t.Fatalf("size = %v, want 12", got[0].Size())
	}
	if got[0].FontBase() != "Helvetica" {
		t.Fatalf("font base = %q, want Helvetica", got[0].FontBase())
	}
	if got[0].TextFont() != "Helvetica 12" {
		t.Fatalf("text font = %q, want Helvetica 12", got[0].TextFont())
	}
}

func TestInterpretTextFlushesBeforeApplyingNextGraphicsOperator(t *testing.T) {
	ops, err := ParseContent([]byte("BT /F1 10 Tf (A) Tj 1 j (B) Tj ET"))
	if err != nil {
		t.Fatal(err)
	}
	texts := InterpretTextWithFonts(ops, map[string]*Font{"F1": NewSimpleFont("Helvetica")})
	if len(texts) != 2 {
		t.Fatalf("text objects = %#v, want one object on each side of j", texts)
	}
	if got := texts[0].GState().LineJoin(); got != 0 {
		t.Fatalf("text before j has line join %d, want default 0", got)
	}
	if got := texts[1].GState().LineJoin(); got != 1 {
		t.Fatalf("text after j has line join %d, want 1", got)
	}
}

func TestInterpretTextKeepsTextOriginBeforeLeadingTJAdjustment(t *testing.T) {
	ops, err := ParseContent([]byte("BT /F1 12 Tf [-1000 (A)] TJ ET"))
	if err != nil {
		t.Fatal(err)
	}
	texts := InterpretTextWithFonts(ops, map[string]*Font{"F1": NewSimpleFont("Helvetica")})
	if len(texts) != 1 {
		t.Fatalf("text objects = %#v", texts)
	}
	text := texts[0]
	if got := text.TextMatrix()[4]; got != 0 {
		t.Fatalf("text matrix x = %v, want unadjusted origin 0", got)
	}
	glyphs := text.GlyphsCopy()
	if len(glyphs) != 1 || glyphs[0].Matrix()[4] != 12 {
		t.Fatalf("glyphs = %#v, want glyph x=12 after TJ adjustment", glyphs)
	}
	if got := text.Displacement()[0]; got <= 12 {
		t.Fatalf("text displacement x = %v, want full displacement from original origin", got)
	}
}

func TestInterpretTextObjectExposesRenderingMatrices(t *testing.T) {
	ops, err := ParseContent([]byte("BT /F1 10 Tf 50 Tz 2 Ts 1 0 0 1 100 200 Tm (A) Tj ET"))
	if err != nil {
		t.Fatal(err)
	}
	got := InterpretText(ops)
	if len(got) != 1 {
		t.Fatalf("text objects = %#v", got)
	}
	if got[0].TextMatrix() != (geometry.Matrix{1, 0, 0, 1, 100, 200}) {
		t.Fatalf("text matrix = %v", got[0].TextMatrix())
	}
	if got[0].ScalingMatrix() != (geometry.Matrix{5, 0, 0, 10, 0, 2}) {
		t.Fatalf("scaling matrix = %v", got[0].ScalingMatrix())
	}
	if got[0].Matrix() != (geometry.Matrix{5, 0, 0, 10, 100, 202}) {
		t.Fatalf("rendering matrix = %v", got[0].Matrix())
	}
}

func TestTextFontNameUsesPythonTiesToEvenRounding(t *testing.T) {
	if got := textFontName("Times New Roman", 10.5); got != "Times New Roman 10" {
		t.Fatalf("text font at 10.5 = %q, want Python-compatible ties-to-even result", got)
	}
	if got := textFontName("Times New Roman", 11.5); got != "Times New Roman 12" {
		t.Fatalf("text font at 11.5 = %q, want Python-compatible ties-to-even result", got)
	}
}

func TestInterpretTextObjectExposesLineMatrix(t *testing.T) {
	ops, err := ParseContent([]byte("BT 1 0 0 1 10 20 Tm 5 6 Td /F1 10 Tf (A) Tj ET"))
	if err != nil {
		t.Fatal(err)
	}
	got := InterpretText(ops)
	if len(got) != 1 {
		t.Fatalf("text objects = %#v", got)
	}
	if got[0].LineMatrix() != (geometry.Matrix{1, 0, 0, 1, 15, 26}) {
		t.Fatalf("line matrix = %v", got[0].LineMatrix())
	}
}

func TestInterpretTextRejectsNonFiniteTextMatrix(t *testing.T) {
	ops := []ContentOp{
		newContentOpBorrowed("BT", nil, 0),
		newContentOpBorrowed("Tm", []Object{Number(1), Number(0), Number(0), Number(1), Number(10), Number(20)}, 0),
		newContentOpBorrowed("Tm", []Object{Number(1), Number(math.NaN()), Number(0), Number(1), Number(90), Number(100)}, 0),
		newContentOpBorrowed("Tj", []Object{String("A")}, 0),
		newContentOpBorrowed("ET", nil, 0),
	}
	got := InterpretText(ops)
	if len(got) != 1 {
		t.Fatalf("text objects = %#v", got)
	}
	if got[0].Origin() != [2]float64{10, 20} {
		t.Fatalf("non-finite Tm changed text origin = %v, want [10 20]", got[0].Origin())
	}
}

func TestInterpretTextRejectsOverflowingCTMProduct(t *testing.T) {
	ops := []ContentOp{
		newContentOpBorrowed("BT", nil, 0),
		newContentOpBorrowed("cm", []Object{
			Number(math.MaxFloat64), Number(0), Number(0), Number(1), Number(0), Number(0),
		}, 0),
		newContentOpBorrowed("cm", []Object{
			Number(math.MaxFloat64), Number(0), Number(0), Number(1), Number(0), Number(0),
		}, 0),
		newContentOpBorrowed("Tm", []Object{Number(1), Number(0), Number(0), Number(1), Number(10), Number(20)}, 0),
		newContentOpBorrowed("Tj", []Object{String("A")}, 0),
		newContentOpBorrowed("ET", nil, 0),
	}
	got := InterpretText(ops)
	if len(got) != 0 {
		t.Fatalf("overflowing CTM reached text output: %#v", got)
	}
}

func TestInterpretTextRejectsOverflowingGlyphAdvance(t *testing.T) {
	ops := []ContentOp{
		newContentOpBorrowed("BT", nil, 0),
		newContentOpBorrowed("Tf", []Object{Name("F1"), Number(math.MaxFloat64)}, 0),
		newContentOpBorrowed("Tm", []Object{Number(1), Number(0), Number(0), Number(1), Number(10), Number(20)}, 0),
		newContentOpBorrowed("Tj", []Object{String("AA")}, 0),
		newContentOpBorrowed("ET", nil, 0),
	}
	got := InterpretText(ops)
	if len(got) != 1 || math.IsInf(got[0].Displacement()[0], 0) || math.IsNaN(got[0].Displacement()[0]) {
		t.Fatalf("overflowing glyph advance reached text output: %#v", got)
	}
}

func TestInterpretTextWithFontsRejectsNonFiniteGlyphGeometry(t *testing.T) {
	font := NewSimpleFont("F1")
	font.widths[65] = math.MaxFloat64
	ops := []ContentOp{
		newContentOpBorrowed("BT", nil, 0),
		newContentOpBorrowed("Tf", []Object{Name("F1"), Number(math.MaxFloat64)}, 0),
		newContentOpBorrowed("Tj", []Object{String("A")}, 0),
		newContentOpBorrowed("ET", nil, 0),
	}
	if got := InterpretTextWithFonts(ops, map[string]*Font{"F1": font}); len(got) != 0 {
		t.Fatalf("non-finite glyph geometry was published: %#v", got)
	}
}

func TestAppendFontStringRejectsOverflowingTextOrigin(t *testing.T) {
	font := NewSimpleFont("F1")
	var object textObjectState
	_, err := appendFontStringWithError(
		&object,
		geometry.Matrix{1, 0, 0, 1, math.MaxFloat64, 0},
		0, 0,
		[]byte{'A'}, "F1", 1,
		func(string) *Font { return font },
		0, 1, 0, 0,
		geometry.Matrix{2, 0, 0, 1, 0, 0}, false,
		map[textFontNameKey]string{}, new([]DecodedGlyph), nil,
	)
	if err == nil {
		t.Fatal("overflowing text origin was accepted")
	}
}

func TestValidateTextGeometryRejectsNonFiniteAggregateValues(t *testing.T) {
	text := textObjectState{
		origin:       [2]float64{math.Inf(1), 0},
		displacement: [2]float64{0, 0},
		bbox:         [4]float64{0, 0, 1, 1},
		matrix:       identity(),
	}
	if err := validateTextGeometry(text); err == nil {
		t.Fatal("non-finite aggregate text geometry was accepted")
	}
}

func TestValidateTextGeometryRejectsNonFiniteDerivedGlyphMatrix(t *testing.T) {
	text := textObjectState{
		matrix: identity(),
		glyphs: []GlyphObject{newTestGlyph(contentdata.GlyphSpec{Matrix: identity()}, nil, nil, geometry.Matrix{math.Inf(1), 0, 0, 1, 0, 0})},
	}
	if err := validateTextGeometry(text); err == nil {
		t.Fatal("non-finite derived glyph matrix was accepted")
	}
}

func TestTextRenderingMatrixRejectsOverflowingProducts(t *testing.T) {
	_, ok := textRenderingMatrix(
		geometry.Matrix{math.MaxFloat64, 0, 0, 1, 0, 0},
		geometry.Matrix{math.MaxFloat64, 0, 0, 1, 0, 0},
		geometry.Matrix{math.MaxFloat64, 0, 0, math.MaxFloat64, 0, 0},
	)
	if ok {
		t.Fatal("overflowing text rendering matrix was accepted")
	}
}

func TestInterpretTextObjectPreservesTextArgs(t *testing.T) {
	ops, err := ParseContent([]byte("BT /F1 10 Tf [(A) 500 (B)] TJ ET"))
	if err != nil {
		t.Fatal(err)
	}
	got := InterpretText(ops)
	if len(got) != 1 || len(got[0].ArgsCopy()) != 3 {
		t.Fatalf("text args = %#v", got)
	}
	args := got[0].ArgsCopy()
	if string(args[0].(String)) != "A" || args[1] != Number(500) || string(args[2].(String)) != "B" {
		t.Fatalf("text args = %#v, want A, 500, B", args)
	}
}

func TestInterpretTextObjectPreservesLeadingTJAdjustment(t *testing.T) {
	ops, err := ParseContent([]byte("BT /F1 10 Tf [-500 (A)] TJ ET"))
	if err != nil {
		t.Fatal(err)
	}
	got := InterpretText(ops)
	if len(got) != 1 || len(got[0].ArgsCopy()) != 2 {
		t.Fatalf("text args = %#v", got)
	}
	args := got[0].ArgsCopy()
	if args[0] != Number(-500) || string(args[1].(String)) != "A" {
		t.Fatalf("text args = %#v, want -500, A", args)
	}
}

func TestInterpretTextObjectExposesCharsAlias(t *testing.T) {
	ops, err := ParseContent([]byte("BT /F1 10 Tf (A) Tj ET"))
	if err != nil {
		t.Fatal(err)
	}
	got := InterpretText(ops)
	if len(got) != 1 || got[0].Chars() != "A" {
		t.Fatalf("chars = %#v, want A", got)
	}
}

func TestInterpretTextObjectRetainsResolvedFont(t *testing.T) {
	ops, err := ParseContent([]byte("BT /F1 10 Tf (A) Tj ET"))
	if err != nil {
		t.Fatal(err)
	}
	font := NewSimpleFont("Helvetica")
	got := InterpretTextWithFonts(ops, map[string]*Font{"F1": font})
	if len(got) != 1 || got[0].font != font {
		t.Fatalf("text font = %p, want %p", got[0].font, font)
	}
}

func TestInterpretGlyphExposesFontPropertiesAndMatrix(t *testing.T) {
	ops, err := ParseContent([]byte("BT /F1 10 Tf 50 Tz 1 0 0 1 100 200 Tm (A) Tj ET"))
	if err != nil {
		t.Fatal(err)
	}
	font := NewSimpleFont("ABCDEF+Helvetica")
	got := InterpretTextWithFonts(ops, map[string]*Font{"F1": font})
	if len(got) != 1 || len(got[0].glyphs) != 1 {
		t.Fatalf("text objects = %#v", got)
	}
	glyph := got[0].glyphs[0]
	if glyph.font != font || glyph.Size() != 10 || glyph.FontBase() != "Helvetica" || glyph.TextFont() != "Helvetica 10" {
		t.Fatalf("glyph font properties = %#v", glyph)
	}
	if glyph.Matrix() != (geometry.Matrix{5, 0, 0, 10, 100, 200}) {
		t.Fatalf("glyph matrix = %v", glyph.Matrix())
	}
}

func TestInterpretGlyphExposesCharsAlias(t *testing.T) {
	ops, err := ParseContent([]byte("BT /F1 10 Tf (A) Tj ET"))
	if err != nil {
		t.Fatal(err)
	}
	got := InterpretText(ops)
	if len(got) != 1 || len(got[0].glyphs) != 1 || got[0].glyphs[0].Chars() != "A" {
		t.Fatalf("glyph chars = %#v, want A", got)
	}
}

func TestParseContent(t *testing.T) {
	ops, e := ParseContent([]byte("q 1 0 0 1 10 20 cm /F1 12 Tf (hello) Tj Q"))
	if e != nil {
		t.Fatal(e)
	}
	if len(ops) != 5 || ops[1].operatorValue() != "cm" || ops[3].operatorValue() != "Tj" {
		t.Fatalf("%#v", ops)
	}
}

func TestParseContentRejectsIndirectReferences(t *testing.T) {
	for _, content := range []string{
		"7 0 R newoperator",
		"/Span << /Self 7 0 R >> BDC",
		"BI /ACME_Private << /Self 7 0 R >> /W 1 /H 1 /BPC 8 /CS /G /L 1 ID X EI",
	} {
		if _, err := ParseContent([]byte(content)); err == nil {
			t.Fatalf("ParseContent(%q) accepted an indirect object reference", content)
		}
	}
}

func TestParseContentRecoversPlayaInlineImageParameters(t *testing.T) {
	ops, err := ParseContent([]byte("BI 1 null ID X EI BI /A {1 null} /L 1 ID Y EI Q"))
	if err != nil {
		t.Fatal(err)
	}
	if len(ops) != 3 || ops[0].Operator() != "BI" || ops[1].Operator() != "BI" || ops[2].Operator() != "Q" {
		t.Fatalf("operations = %#v", ops)
	}
	first, ok := ops[0].OperandsCopy()[0].(Stream)
	if !ok || len(first.DictCopy()) != 0 || string(first.DataBorrowed()) != "X" {
		t.Fatalf("first inline image = %#v", ops[0].OperandsCopy())
	}
	second, ok := ops[1].OperandsCopy()[0].(Stream)
	if !ok || string(second.DataBorrowed()) != "Y" {
		t.Fatalf("second inline image = %#v", ops[1].OperandsCopy())
	}
	want := Dict{Name("A"): Array{Number(1), Null{}}, Name("L"): Number(1)}
	if got := second.DictCopy(); !reflect.DeepEqual(got, want) {
		t.Fatalf("second inline params = %#v, want %#v", got, want)
	}
}

func TestParseContentParsesTopLevelProcedureOperand(t *testing.T) {
	ops, err := ParseContent([]byte("{1 null {2}} q"))
	if err != nil {
		t.Fatal(err)
	}
	want := Array{Number(1), Null{}, Array{Number(2)}}
	if len(ops) != 1 || ops[0].Operator() != "q" || !reflect.DeepEqual(ops[0].OperandsCopy(), []Object{want}) {
		t.Fatalf("procedure content = %#v, want q with %#v", ops, want)
	}
}

func TestParseContentDiscardsUnterminatedTopLevelProcedure(t *testing.T) {
	ops, err := ParseContent([]byte("q {1"))
	if err != nil {
		t.Fatal(err)
	}
	if len(ops) != 1 || ops[0].Operator() != "q" {
		t.Fatalf("unterminated procedure content = %#v, want q", ops)
	}
}

func TestParseContentIgnoresTrailingOperands(t *testing.T) {
	ops, err := ParseContent([]byte("BT /F1 10 Tf (text) Tj ET 1 2 3"))
	if err != nil {
		t.Fatal(err)
	}
	if len(ops) != 4 || ops[len(ops)-1].operatorValue() != "ET" {
		t.Fatalf("operations = %#v", ops)
	}
}

func TestParseContentSkipsUnmatchedClosingDelimiters(t *testing.T) {
	ops, err := ParseContent([]byte("BT (text) Tj ET ] >>"))
	if err != nil {
		t.Fatal(err)
	}
	if len(ops) != 3 || ops[len(ops)-1].operatorValue() != "ET" {
		t.Fatalf("operations = %#v", ops)
	}
}

func TestParseContentRecoversUnclosedArray(t *testing.T) {
	ops, err := ParseContent([]byte("[(text)] TJ [1 2"))
	if err != nil {
		t.Fatal(err)
	}
	if len(ops) != 1 || ops[0].operatorValue() != "TJ" {
		t.Fatalf("operations = %#v", ops)
	}
}

func TestParseContentTreatsUnterminatedTrailingStringAsEOF(t *testing.T) {
	ops, err := ParseContent([]byte("q ("))
	if err != nil {
		t.Fatal(err)
	}
	if len(ops) != 1 || ops[0].Operator() != "q" {
		t.Fatalf("operations = %#v, want trailing string discarded after q", ops)
	}
}

func TestParseContentTreatsUnterminatedInlineParametersAsEOF(t *testing.T) {
	for _, data := range []string{"q BI /A (", "q BI /A {1"} {
		ops, err := ParseContent([]byte(data))
		if err != nil {
			t.Fatalf("ParseContent(%q): %v", data, err)
		}
		if len(ops) != 1 || ops[0].Operator() != "q" {
			t.Fatalf("ParseContent(%q) = %#v, want the completed q operation", data, ops)
		}
	}
}

func TestInterpretTextUsesFontMapping(t *testing.T) {
	ops, _ := ParseContent([]byte("BT /F1 10 Tf <0102> Tj ET"))
	f := NewSimpleFont("F1")
	f.toUnicode = map[uint16]string{1: "A", 2: "中"}
	got := InterpretTextWithFonts(ops, map[string]*Font{"F1": f})
	if len(got) != 1 || got[0].Text() != "A中" {
		t.Fatalf("%#v", got)
	}
}

func TestInterpretTextCarriesGraphicsState(t *testing.T) {
	ops, _ := ParseContent([]byte("q 2 w 0.1 0.2 0.3 RG BT /F1 10 Tf (A) Tj ET Q"))
	got := InterpretText(ops)
	if len(got) != 1 || got[0].GState().LineWidth() != 2 || got[0].GState().StrokeColor().Space() != "DeviceRGB" {
		t.Fatalf("text graphics state = %#v", got)
	}
}

func TestInterpretTextCarriesTransparencyState(t *testing.T) {
	ops, err := ParseContent([]byte("0.25 CA 0.5 ca /Multiply BM BT (x) Tj ET"))
	if err != nil {
		t.Fatal(err)
	}
	got := InterpretText(ops)
	if len(got) != 1 || got[0].GState().StrokeAlpha() != 0.25 || got[0].GState().FillAlpha() != 0.5 || got[0].GState().BlendMode() != "Multiply" {
		t.Fatalf("text transparency state = %#v", got)
	}
}

func TestInterpretTextCarriesMarkedActualText(t *testing.T) {
	ops, err := ParseContent([]byte("/Span << /MCID 3 /ActualText <FEFF0041> >> BDC BT (x) Tj ET EMC"))
	if err != nil {
		t.Fatal(err)
	}
	got := InterpretText(ops)
	if len(got) != 1 || got[0].MCID() != 3 || got[0].ActualText() != "A" {
		t.Fatalf("text marked metadata = %#v", got)
	}
}

func TestPageTextResolvesNamedMarkedProperties(t *testing.T) {
	d := &Document{}
	p := Page{dict: Dict{
		Name("Resources"): Dict{Name("Properties"): Dict{Name("P1"): Dict{Name("MCID"): Number(7), Name("ActualText"): String([]byte{0xfe, 0xff, 0x00, 0x41})}}},
		Name("Contents"):  newStream(nil, []byte("/Span /P1 BDC BT (x) Tj ET EMC")),
	}}
	got, err := d.PageText(p)
	if err != nil || len(got) != 1 || got[0].MCID() != 7 || got[0].ActualText() != "A" {
		t.Fatalf("named marked properties = %#v, err=%v", got, err)
	}
}

func TestPageTextResolvesFormMarkedProperties(t *testing.T) {
	form := newStream(Dict{
		Name("Subtype"): Name("Form"),
		Name("Resources"): Dict{Name("Properties"): Dict{
			Name("P1"): Dict{Name("MCID"): Number(8), Name("ActualText"): String("form")},
		}},
	}, []byte("/Span /P1 BDC BT (x) Tj ET EMC"))
	d := &Document{}
	p := Page{dict: Dict{
		Name("Resources"): Dict{Name("XObject"): Dict{Name("Fm"): form}},
		Name("Contents"):  newStream(nil, []byte("/Fm Do")),
	}}
	var got []TextObject
	var err error
	for text, itemErr := range p.Texts(d) {
		if itemErr != nil {
			err = itemErr
			break
		}
		got = append(got, text)
	}
	if err != nil || len(got) != 1 || got[0].MCID() != 8 || got[0].ActualText() != "form" || got[0].MarkedPropertiesCopy()[Name("MCID")] != Number(8) {
		t.Fatalf("form marked properties = %#v, err=%v", got, err)
	}
}

func TestPageXObjectsFollowMultiLevelIndirectResources(t *testing.T) {
	d := &Document{objects: map[Ref]Object{
		{Object: 1}: newStream(nil, []byte("/Fm Do")),
		{Object: 2}: Ref{Object: 3},
		{Object: 3}: Dict{Name("XObject"): Ref{Object: 4}},
		{Object: 4}: Dict{Name("Fm"): Ref{Object: 5}},
		{Object: 5}: Ref{Object: 6},
		{Object: 6}: newStream(Dict{Name("Subtype"): Name("Form")}, []byte("q Q")),
	}}
	p := Page{ref: Ref{Object: 20}, dict: Dict{
		Name("Contents"):  Ref{Object: 1},
		Name("Resources"): Ref{Object: 2},
	}}

	var objects []XObjectObject
	for object, err := range p.XObjects(d) {
		if err != nil {
			t.Fatalf("XObjects() error = %v", err)
		}
		objects = append(objects, object)
	}
	if len(objects) != 1 || objects[0].Name() != "Fm" || objects[0].Ref() != (Ref{Object: 5}) {
		t.Fatalf("XObjects() = %#v", objects)
	}
}

func TestSelectedResourcesFollowMultiLevelIndirectChains(t *testing.T) {
	d := &Document{objects: map[Ref]Object{
		{Object: 1}: Ref{Object: 2},
		{Object: 2}: Dict{
			Name("ColorSpace"): Ref{Object: 3},
			Name("ExtGState"):  Ref{Object: 4},
			Name("Pattern"):    Ref{Object: 5},
			Name("Shading"):    Ref{Object: 6},
			Name("Properties"): Ref{Object: 7},
		},
		{Object: 3}:  Ref{Object: 8},
		{Object: 4}:  Ref{Object: 9},
		{Object: 5}:  Ref{Object: 10},
		{Object: 6}:  Ref{Object: 11},
		{Object: 7}:  Ref{Object: 12},
		{Object: 8}:  Dict{Name("CS"): Ref{Object: 13}},
		{Object: 9}:  Dict{Name("GS"): Ref{Object: 14}},
		{Object: 10}: Dict{Name("P"): Ref{Object: 15}},
		{Object: 11}: Dict{Name("S"): Ref{Object: 16}},
		{Object: 12}: Dict{Name("P"): Ref{Object: 17}},
		{Object: 13}: Name("DeviceGray"),
		{Object: 14}: Dict{Name("LW"): Number(2)},
		{Object: 15}: Dict{Name("PatternType"): Number(1), Name("PaintType"): Number(1)},
		{Object: 16}: Dict{Name("ShadingType"): Number(2), Name("ColorSpace"): Name("DeviceGray")},
		{Object: 17}: Dict{Name("MCID"): Number(4)},
	}}
	p := Page{dict: Dict{
		Name("Contents"):  newStream(nil, []byte("/CS cs /GS gs /Pattern cs /P scn /S sh /Tag /P BDC EMC")),
		Name("Resources"): Ref{Object: 1},
	}}

	var colors []ColorSpaceObject
	for value, err := range p.ColorSpaces(d) {
		if err != nil {
			t.Fatalf("ColorSpaces() error = %v", err)
		}
		colors = append(colors, value)
	}
	var states []ExtGStateObject
	for value, err := range p.ExtGStates(d) {
		if err != nil {
			t.Fatalf("ExtGStates() error = %v", err)
		}
		states = append(states, value)
	}
	var patterns []PatternObject
	for value, err := range p.Patterns(d) {
		if err != nil {
			t.Fatalf("Patterns() error = %v", err)
		}
		patterns = append(patterns, value)
	}
	var shadings []ShadingObject
	for value, err := range p.Shadings(d) {
		if err != nil {
			t.Fatalf("Shadings() error = %v", err)
		}
		shadings = append(shadings, value)
	}
	var properties []PropertiesObject
	for value, err := range p.Properties(d) {
		if err != nil {
			t.Fatalf("Properties() error = %v", err)
		}
		properties = append(properties, value)
	}
	if len(colors) != 2 || len(states) != 1 || len(patterns) != 1 || len(shadings) != 1 || len(properties) != 1 {
		t.Fatalf("selected resources = colors:%d states:%d patterns:%d shadings:%d properties:%d", len(colors), len(states), len(patterns), len(shadings), len(properties))
	}
}

func TestPageTextExpandedResolvesFormMarkedProperties(t *testing.T) {
	form := newStream(Dict{
		Name("Subtype"): Name("Form"),
		Name("Resources"): Dict{
			Name("Font"):       Dict{Name("F"): Dict{Name("Subtype"): Name("Type1"), Name("BaseFont"): Name("Helvetica")}},
			Name("Properties"): Dict{Name("P1"): Dict{Name("ActualText"): String("form")}},
		},
	}, []byte("/Span /P1 BDC BT /F 10 Tf (painted) Tj ET EMC"))
	d := &Document{}
	p := Page{dict: Dict{
		Name("Contents"):  newStream(nil, []byte("/Fm Do")),
		Name("Resources"): Dict{Name("XObject"): Dict{Name("Fm"): form}},
	}}

	texts, err := d.PageTextExpanded(p)
	if err != nil {
		t.Fatal(err)
	}
	if len(texts) != 1 || texts[0].ActualText() != "form" {
		t.Fatalf("expanded form text = %#v", texts)
	}
}

func TestPageTextExpandedAppliesFormExternalGraphicsState(t *testing.T) {
	form := newStream(Dict{
		Name("Subtype"): Name("Form"),
		Name("Resources"): Dict{
			Name("Font"):      Dict{Name("F"): Dict{Name("Subtype"): Name("Type1"), Name("BaseFont"): Name("Helvetica")}},
			Name("ExtGState"): Dict{Name("GS1"): Dict{Name("LW"): Number(2), Name("CA"): Number(0.4), Name("BM"): Name("Multiply")}},
		},
	}, []byte("/GS1 gs BT /F 10 Tf (painted) Tj ET"))
	d := &Document{}
	p := Page{dict: Dict{
		Name("Contents"):  newStream(nil, []byte("/Fm Do")),
		Name("Resources"): Dict{Name("XObject"): Dict{Name("Fm"): form}},
	}}

	texts, err := d.PageTextExpanded(p)
	if err != nil {
		t.Fatal(err)
	}
	if len(texts) != 1 || texts[0].GState().LineWidth() != 2 || texts[0].GState().BlendMode() != "Multiply" {
		t.Fatalf("expanded form graphics state = %#v", texts)
	}
}

func TestPageTextExpandedRestoresGraphicsStateAfterForm(t *testing.T) {
	form := newStream(Dict{Name("Subtype"): Name("Form")}, []byte("2 w BT (inside) Tj ET"))
	d := &Document{}
	p := Page{dict: Dict{
		Name("Contents"):  newStream(nil, []byte("/Fm Do BT (outside) Tj ET")),
		Name("Resources"): Dict{Name("XObject"): Dict{Name("Fm"): form}},
	}}
	texts, err := d.PageTextExpanded(p)
	if err != nil {
		t.Fatal(err)
	}
	if len(texts) != 2 || texts[0].GState().LineWidth() != 2 || texts[1].GState().LineWidth() != 1 {
		t.Fatalf("form graphics state scope = %#v", texts)
	}
}

func TestPageTextAppliesPageExternalGraphicsState(t *testing.T) {
	d := &Document{}
	p := Page{dict: Dict{
		Name("Contents"): newStream(nil, []byte("/GS1 gs BT /F 10 Tf (painted) Tj ET")),
		Name("Resources"): Dict{
			Name("Font"):      Dict{Name("F"): Dict{Name("Subtype"): Name("Type1"), Name("BaseFont"): Name("Helvetica")}},
			Name("ExtGState"): Dict{Name("GS1"): Dict{Name("LW"): Number(2), Name("BM"): Name("Multiply")}},
		},
	}}

	texts, err := d.PageText(p)
	if err != nil {
		t.Fatal(err)
	}
	if len(texts) != 1 || texts[0].GState().LineWidth() != 2 || texts[0].GState().BlendMode() != "Multiply" {
		t.Fatalf("page graphics state = %#v", texts)
	}
}

func TestPageTextFollowsMultiLevelIndirectResources(t *testing.T) {
	d := &Document{objects: map[Ref]Object{
		{Object: 1}: Ref{Object: 2},
		{Object: 2}: Dict{
			Name("Font"): Dict{Name("F"): Dict{
				Name("Subtype"): Name("Type1"), Name("BaseFont"): Name("Helvetica"),
			}},
			Name("ExtGState"): Dict{Name("GS"): Dict{Name("LW"): Number(3)}},
		},
	}}
	p := Page{dict: Dict{
		Name("Contents"):  newStream(nil, []byte("/GS gs BT /F 10 Tf (indirect) Tj ET")),
		Name("Resources"): Ref{Object: 1},
	}}
	texts, err := d.PageText(p)
	if err != nil {
		t.Fatal(err)
	}
	if len(texts) != 1 || texts[0].Text() != "indirect" || texts[0].GState().LineWidth() != 3 {
		t.Fatalf("indirect page resources text = %#v", texts)
	}
}

func TestPageTextPadsResourceColorSpaceComponents(t *testing.T) {
	d := &Document{}
	p := Page{dict: Dict{
		Name("Contents"): newStream(nil, []byte("/DN cs 0.5 scn BT (x) Tj ET")),
		Name("Resources"): Dict{Name("ColorSpace"): Dict{
			Name("DN"): Array{Name("DeviceN"), Array{Name("C1"), Name("C2")}, Name("DeviceRGB"), Null{}},
		}},
	}}
	texts, err := d.PageText(p)
	if err != nil {
		t.Fatal(err)
	}
	if len(texts) != 1 || !reflect.DeepEqual(texts[0].GState().FillColor().ValuesCopy(), []float64{0.5, 0}) {
		t.Fatalf("resource text color = %#v", texts)
	}
}

func TestBatchPageTextEntrypointsPreservePageOwnership(t *testing.T) {
	d := &Document{}
	p := Page{ref: Ref{Object: 7}, dict: Dict{Name("Contents"): newStream(nil, []byte("BT (x) Tj ET"))}}

	plain, err := d.PageText(p)
	if err != nil {
		t.Fatal(err)
	}
	expanded, err := d.PageTextExpanded(p)
	if err != nil {
		t.Fatal(err)
	}
	if len(plain) != 1 || !plain[0].HasPage() || plain[0].Page() != p.ref {
		t.Fatalf("PageText ownership = %#v", plain)
	}
	if len(expanded) != 1 || !expanded[0].HasPage() || expanded[0].Page() != p.ref {
		t.Fatalf("PageTextExpanded ownership = %#v", expanded)
	}
}

func TestInterpretTextPreservesNamedColorSpaces(t *testing.T) {
	ops, err := ParseContent([]byte("/CS1 CS 0.2 0.3 SCN BT (x) Tj ET"))
	if err != nil {
		t.Fatal(err)
	}
	got := InterpretText(ops)
	if len(got) != 1 || got[0].GState().StrokeColor().Space() != "CS1" || len(got[0].GState().StrokeColor().ValuesCopy()) != 2 {
		t.Fatalf("text named color space = %#v", got)
	}
}

func TestInterpretTextPadsMissingDeviceColorComponents(t *testing.T) {
	ops, err := ParseContent([]byte("0.25 0 0 rg BT (x) Tj ET"))
	if err != nil {
		t.Fatal(err)
	}
	texts := InterpretText(ops)
	if len(texts) != 1 || !reflect.DeepEqual(texts[0].NonStrokeColorCopy(), []float64{0.25, 0, 0}) {
		t.Fatalf("text color = %#v", texts)
	}
}

func TestInterpretTextTruncatesExtraDeviceColorComponents(t *testing.T) {
	ops, err := ParseContent([]byte("0.1 0.2 0.3 0.4 rg BT (x) Tj ET"))
	if err != nil {
		t.Fatal(err)
	}
	texts := InterpretText(ops)
	if len(texts) != 1 || !reflect.DeepEqual(texts[0].NonStrokeColorCopy(), []float64{0.1, 0.2, 0.3}) {
		t.Fatalf("text color = %#v", texts)
	}
}

func TestInterpretTextIgnoresNonFiniteDeviceColorUpdate(t *testing.T) {
	ops := []ContentOp{
		newContentOpBorrowed("rg", []Object{Number(0.1), Number(0.2), Number(0.3)}, 0),
		newContentOpBorrowed("rg", []Object{Number(math.NaN()), Number(0.8), Number(0.9)}, 0),
		newContentOpBorrowed("BT", nil, 0),
		newContentOpBorrowed("Tj", []Object{String("x")}, 0),
		newContentOpBorrowed("ET", nil, 0),
	}
	got := InterpretText(ops)
	if len(got) != 1 || !reflect.DeepEqual(got[0].NonStrokeColorCopy(), []float64{0.1, 0.2, 0.3}) {
		t.Fatalf("non-finite device color changed state = %#v", got)
	}
}

func TestInterpretTextPreservesPatternAndBlendModeArray(t *testing.T) {
	ops, err := ParseContent([]byte("/Pattern cs /P1 scn [/Multiply /Screen] BM BT (x) Tj ET"))
	if err != nil {
		t.Fatal(err)
	}
	got := InterpretText(ops)
	if len(got) != 1 || got[0].GState().FillColor().Pattern() != "P1" || got[0].GState().BlendMode() != "Multiply" || len(got[0].GState().BlendModesCopy()) != 2 || got[0].GState().BlendModesCopy()[1] != "Screen" {
		t.Fatalf("text pattern/composite state = %#v", got)
	}
}

func TestInterpretTextIgnoresMalformedBlendModeArrayAtomically(t *testing.T) {
	ops, err := ParseContent([]byte("/Multiply BM [/Screen 1] BM BT (x) Tj ET"))
	if err != nil {
		t.Fatal(err)
	}
	got := InterpretText(ops)
	if len(got) != 1 || got[0].GState().BlendMode() != "Multiply" || len(got[0].GState().BlendModesCopy()) != 0 {
		t.Fatalf("malformed blend-mode array changed text state = %#v", got)
	}
}

func TestInterpretTextPreservesGlyphCodeAndCID(t *testing.T) {
	ops, _ := ParseContent([]byte("BT (A) Tj ET"))
	got := InterpretText(ops)
	if len(got) != 1 || len(got[0].glyphs) != 1 || string(got[0].glyphs[0].Codes()) != "A" || got[0].glyphs[0].CID() != 65 {
		t.Fatalf("glyph source identity = %#v", got)
	}
}

func TestInterpretTextAssignsPageContextDuringConstruction(t *testing.T) {
	ops, err := ParseContent([]byte("/Span <</MCID 3>> BDC BT (AB) Tj ET EMC"))
	if err != nil {
		t.Fatal(err)
	}
	index := 0
	var got []TextObject
	page := Ref{Object: 7}
	interpretTextWithFontResolverNextPage(nil, page, func() (ContentOp, bool) {
		if index >= len(ops) {
			return ContentOp{}, false
		}
		op := ops[index]
		index++
		return op, true
	}, nil, true, nil, func(text TextObject) bool {
		got = append(got, text)
		return true
	})
	if len(got) != 1 || got[0].Page() != page || got[0].MCID() != 3 || len(got[0].glyphs) != 2 {
		t.Fatalf("page-aware text = %#v", got)
	}
	for _, glyph := range got[0].glyphs {
		if glyph.Page() != page || glyph.MCID() != 3 || !glyph.HasMCID() {
			t.Fatalf("page-aware glyph = %#v", glyph)
		}
	}
	if got[0].glyphs[0].data.ContextBorrowed() != got[0].glyphs[1].data.ContextBorrowed() {
		t.Fatal("glyphs from one text object do not share context")
	}
}

func TestInterpretTextCopiesGraphicsStateToGlyphs(t *testing.T) {
	ops, _ := ParseContent([]byte("3 Tr BT (A) Tj ET"))
	got := InterpretText(ops)
	if len(got) != 1 || len(got[0].glyphs) != 1 || got[0].glyphs[0].GState().RenderMode() != 3 {
		t.Fatalf("glyph graphics state = %#v", got)
	}
}

func TestInterpretTextKeepsPreviousRenderStateForInvalidTr(t *testing.T) {
	ops := []ContentOp{
		newContentOpBorrowed("Tr", []Object{Number(3)}, 0),
		newContentOpBorrowed("Tr", []Object{Number(math.NaN())}, 0),
		newContentOpBorrowed("BT", nil, 0),
		newContentOpBorrowed("Tj", []Object{String("A")}, 0),
		newContentOpBorrowed("ET", nil, 0),
	}
	got := InterpretText(ops)
	if len(got) != 1 || got[0].GState().RenderMode() != 3 {
		t.Fatalf("invalid Tr changed render mode = %#v", got)
	}
}

func TestInterpretTextKeepsValidatedTransparencyAndRenderState(t *testing.T) {
	ops, err := ParseContent([]byte("0.25 CA 0.5 ca 3 Tr 2 CA -1 ca 8 Tr BT (x) Tj ET"))
	if err != nil {
		t.Fatal(err)
	}
	got := InterpretText(ops)
	if len(got) != 1 || got[0].GState().StrokeAlpha() != 0.25 || got[0].GState().FillAlpha() != 0.5 || got[0].GState().RenderMode() != 3 {
		t.Fatalf("invalid transparency/render state changed text snapshot = %#v", got)
	}
}

func TestInterpretTextAppliesQuoteSpacingAtomically(t *testing.T) {
	ops := []ContentOp{
		newContentOpBorrowed("Tc", []Object{Number(2)}, 0),
		newContentOpBorrowed("Tw", []Object{Number(3)}, 0),
		newContentOpBorrowed("BT", nil, 0),
		newContentOpBorrowed("\"", []Object{Number(math.NaN()), Number(4), String("A")}, 0),
		newContentOpBorrowed("ET", nil, 0),
	}
	got := InterpretText(ops)
	if len(got) != 1 {
		t.Fatalf("text objects = %#v", got)
	}
	state := got[0].GState()
	if state.WordSpacing() != 3 || state.CharacterSpacing() != 2 {
		t.Fatalf("quote spacing was partially updated = %#v", state)
	}
}

func TestInterpretTextCarriesTextState(t *testing.T) {
	ops, _ := ParseContent([]byte("2 Tc 3 Tw 80 Tz 14 TL BT (A) Tj ET"))
	got := InterpretText(ops)
	if len(got) != 1 || got[0].GState().CharacterSpacing() != 2 || got[0].GState().WordSpacing() != 3 || got[0].GState().HorizontalScale() != 0.8 || got[0].GState().Leading() != 14 {
		t.Fatalf("text state = %#v", got)
	}
}

func TestInterpretTextKeepsPreviousStateForNonFiniteOperands(t *testing.T) {
	ops := []ContentOp{
		newContentOpBorrowed("BT", nil, 0),
		newContentOpBorrowed("Tf", []Object{Name("F1"), Number(12)}, 0),
		newContentOpBorrowed("Tc", []Object{Number(2)}, 0),
		newContentOpBorrowed("Tw", []Object{Number(3)}, 0),
		newContentOpBorrowed("Tz", []Object{Number(80)}, 0),
		newContentOpBorrowed("Tf", []Object{Name("F2"), Number(math.Inf(1))}, 0),
		newContentOpBorrowed("Tc", []Object{Number(math.NaN())}, 0),
		newContentOpBorrowed("Tw", []Object{Number(math.Inf(-1))}, 0),
		newContentOpBorrowed("Tz", []Object{Number(math.NaN())}, 0),
		newContentOpBorrowed("Tj", []Object{String("A")}, 0),
		newContentOpBorrowed("ET", nil, 0),
	}
	got := InterpretText(ops)
	if len(got) != 1 {
		t.Fatalf("text objects = %#v", got)
	}
	state := got[0].GState()
	if state.FontName() != "F2" || state.FontSize() != 12 || state.CharacterSpacing() != 2 || state.WordSpacing() != 3 || state.HorizontalScale() != 0.8 {
		t.Fatalf("non-finite text operands changed state = %#v", state)
	}
}

func TestInterpretTextCarriesFontStateToGlyphs(t *testing.T) {
	ops, _ := ParseContent([]byte("BT /FEmbedded 13 Tf (A) Tj ET"))
	got := InterpretText(ops)
	if len(got) != 1 || len(got[0].glyphs) != 1 || got[0].glyphs[0].GState().FontName() != "FEmbedded" || got[0].glyphs[0].GState().FontSize() != 13 {
		t.Fatalf("glyph font state = %#v", got)
	}
}

func TestInterpretTextAppliesExternalGraphicsState(t *testing.T) {
	d := &Document{}
	fontRef := Ref{Object: 12, Generation: 0}
	resources := Dict{
		Name("Font"): Dict{Name("F1"): fontRef},
		Name("ExtGState"): Dict{Name("GS1"): Dict{
			Name("Font"): Array{fontRef, Number(14)},
			Name("LW"):   Number(2), Name("CA"): Number(0.4), Name("BM"): Name("Multiply"),
		}},
	}
	ops := []ContentOp{
		newContentOpWithResources("gs", []Object{Name("GS1")}, resources),
		newContentOpBorrowed("BT", nil, 0), newContentOpBorrowed("Tj", []Object{String("A")}, 0), newContentOpBorrowed("ET", nil, 0),
	}
	var got []TextObject
	interpretTextWithFontResolverNext(d, func() (ContentOp, bool) {
		if len(ops) == 0 {
			return ContentOp{}, false
		}
		op := ops[0]
		ops = ops[1:]
		return op, true
	}, func(name string) *Font { return NewSimpleFont(name) }, true, nil, func(text TextObject) bool {
		got = append(got, text)
		return true
	})
	if len(got) != 1 || got[0].FontName() != "F1" || got[0].FontSize() != 14 || got[0].GState().LineWidth() != 2 || got[0].GState().StrokeAlpha() != 0.4 || got[0].GState().BlendMode() != "Multiply" {
		t.Fatalf("external text graphics state = %#v", got)
	}
}

func TestInterpretTextRestoresFontStateAcrossQ(t *testing.T) {
	ops, _ := ParseContent([]byte("/F2 10 Tf q BT /F1 10 Tf (A) Tj ET Q BT (B) Tj ET"))
	got := InterpretText(ops)
	if len(got) != 2 || got[0].FontName() != "F1" || got[1].FontName() != "F2" {
		t.Fatalf("font restore = %#v", got)
	}
}

func TestInterpretTextSkipsEmptyShowObjects(t *testing.T) {
	ops, _ := ParseContent([]byte("BT () Tj (A) Tj ET"))
	got := InterpretText(ops)
	if len(got) != 1 || got[0].Text() != "A" {
		t.Fatalf("text objects = %#v, want one A object", got)
	}
}

func TestInterpretTextCarriesMarkedContentAssociation(t *testing.T) {
	ops, _ := ParseContent([]byte("/P BMC /Span << /MCID 7 >> BDC BT /F1 10 Tf (A) Tj ET EMC EMC"))
	got := InterpretText(ops)
	if len(got) != 1 || !got[0].HasMCID() || got[0].MCID() != 7 || got[0].MarkedTag() != "Span" || got[0].MarkedPropertiesCopy()[Name("MCID")] != Number(7) || len(got[0].markedStack) != 2 || got[0].markedStack[0].Tag() != "P" || got[0].markedStack[1].Tag() != "Span" {
		t.Fatalf("text marked-content association = %#v", got)
	}
}

func TestContentObjectMCIDSkipsNestedMarkedSectionsWithoutMCID(t *testing.T) {
	d := &Document{}
	p := Page{dict: Dict{
		Name("Resources"): Dict{Name("Font"): Dict{Name("F1"): Dict{
			Name("Subtype"):  Name("Type1"),
			Name("BaseFont"): Name("Helvetica"),
		}}},
		Name("Contents"): newStream(nil, []byte(
			"/P << /MCID 7 >> BDC /Span << /Role /emphasis >> BDC "+
				"BT /F1 10 Tf (x) Tj ET EMC EMC",
		)),
	}}

	for object, err := range p.Interp(d, DefaultContentOptions()) {
		if err != nil {
			t.Fatal(err)
		}
		if object.kind != ContentText {
			continue
		}
		if mcid, ok := object.MCIDValue(); !ok || mcid != 7 {
			t.Fatalf("nested marked-content MCID = %d, %v; want 7, true", mcid, ok)
		}
		return
	}
	t.Fatal("nested marked-content fixture produced no text object")
}

func TestInterpretTextAdvancesMatrix(t *testing.T) {
	ops, _ := ParseContent([]byte("BT /F1 10 Tf (A) Tj (B) Tj ET"))
	got := InterpretText(ops)
	if len(got) != 2 || len(got[0].glyphs) != 1 || len(got[1].glyphs) != 1 || got[1].glyphs[0].Origin()[0] <= got[0].glyphs[0].Origin()[0] {
		t.Fatalf("%#v", got)
	}
}

func TestInterpretTextAppliesTextMatrixLinearPartToGlyphAdvances(t *testing.T) {
	ops, _ := ParseContent([]byte("BT /F1 10 Tf 1 -0.1 0 1 100 200 Tm (AB) Tj ET"))
	got := InterpretTextWithFonts(ops, map[string]*Font{"F1": NewSimpleFont("F1")})
	if len(got) != 1 || len(got[0].glyphs) != 2 {
		t.Fatalf("unexpected text objects: %#v", got)
	}
	if got[0].glyphs[1].Origin() != [2]float64{105, 199.5} {
		t.Fatalf("second glyph origin = %v, want [105 199.5]", got[0].glyphs[1].Origin())
	}
	if got[0].glyphs[1].Displacement() != [2]float64{5, -0.5} {
		t.Fatalf("second glyph displacement = %v, want [5 -0.5]", got[0].glyphs[1].Displacement())
	}
}

func TestInterpretTextAdvancesFromFontDisplacementBeforeScaling(t *testing.T) {
	ops, err := ParseContent([]byte("BT /F1 11.7 Tf 1 0 0 1 0 0 Tm (AA) Tj ET"))
	if err != nil {
		t.Fatal(err)
	}
	font := NewSimpleFont("F1")
	font.widths['A'] = 3
	got := InterpretTextWithFonts(ops, map[string]*Font{"F1": font})
	if len(got) != 1 || len(got[0].glyphs) != 2 {
		t.Fatalf("text objects = %#v", got)
	}
	if got := got[0].glyphs[1].Origin()[0]; got != 0.0351 {
		t.Fatalf("second glyph origin = %.17g, want %.17g", got, 0.0351)
	}
}

func TestInterpretTextTJPreservesLocalGlyphOffset(t *testing.T) {
	ops, err := ParseContent([]byte("BT /F1 12 Tf 1 0 0 1 89.9 0 Tm [(AAAA) 120 (AAAAAA) 120 (AAAAAAAAAAAAAAAAA) 120 (AAAAAAAA)] TJ ET"))
	if err != nil {
		t.Fatal(err)
	}
	font := NewSimpleFont("F1")
	font.widths['A'] = 1000
	got := InterpretTextWithFonts(ops, map[string]*Font{"F1": font})
	if len(got) != 1 || len(got[0].glyphs) != 35 {
		t.Fatalf("text objects = %#v", got)
	}
	if origin := got[0].glyphs[27].Origin()[0]; origin != 409.58000000000004 {
		t.Fatalf("glyph after TJ adjustment = %.17g, want %.17g", origin, 409.58000000000004)
	}
}

func TestInterpretTextTJMatchesPlayaBoundaryRounding(t *testing.T) {
	content := "BT /F1 11.04 Tf 1 0 0 1 172.46 691.06 Tm " +
		"[(entit) 10 (y) -3 ( ) -99 (is ) -98 (the ) -100 (ap) 4 (p) 3 (o) -5 (in) 5 (te) -3 (d) 3 ( ) -88 (m) -4 (an) 4 (ag) 15 (er ) -100 (o) -5 (r ) -98 (p) 3 (ro) -3 (p) 3 (e) 9 (rty) -3 ( ) -88 (m) -4 (an) 4 (ag) 4 (er)] TJ ET"
	ops, err := ParseContent([]byte(content))
	if err != nil {
		t.Fatal(err)
	}
	font := NewSimpleFont("F1")
	for char, width := range map[byte]float64{
		'e': 498, 'n': 525, 't': 335, 'i': 230, 'y': 453, ' ': 226,
		's': 391, 'h': 525, 'a': 479, 'p': 525, 'o': 527, 'd': 525,
		'm': 799, 'g': 471, 'r': 349,
	} {
		font.widths[char] = width
	}
	got := InterpretTextWithFonts(ops, map[string]*Font{"F1": font})
	if len(got) != 1 || len(got[0].glyphs) != 51 {
		t.Fatalf("text objects = %#v", got)
	}
	if origin := got[0].glyphs[32].Origin()[0]; origin != 323.81839999999994 {
		t.Fatalf("glyph after manager-space boundary = %.17g, want %.17g", origin, 323.81839999999994)
	}
}

func TestInterpretTextAccumulatesSkewedGlyphOffsetsInTextSpace(t *testing.T) {
	ops, err := ParseContent([]byte("BT /F1 11.7 Tf 0.1 0.9 -0.8 0.2 89.9 7.3 Tm (AAA) Tj ET"))
	if err != nil {
		t.Fatal(err)
	}
	font := NewSimpleFont("F1")
	font.widths['A'] = 3
	got := InterpretTextWithFonts(ops, map[string]*Font{"F1": font})
	if len(got) != 1 || len(got[0].glyphs) != 3 {
		t.Fatalf("text objects = %#v", got)
	}
	step := font.HDisp('A') * 11.7
	wantX, wantY := (geometry.Matrix{0.1, 0.9, -0.8, 0.2, 89.9, 7.3}).Point(2*step, 0)
	if origin := got[0].glyphs[2].Origin(); origin != [2]float64{wantX, wantY} {
		t.Fatalf("third skewed glyph origin = %.17g, %.17g, want %.17g, %.17g", origin[0], origin[1], wantX, wantY)
	}
}

func TestInterpretTextAccumulatesRotatedVerticalGlyphOffsetsInTextSpace(t *testing.T) {
	ops, err := ParseContent([]byte("BT /F1 11.7 Tf 0.1 0.9 -0.7032711038139268 0.2 -961.6893795133581 7.3 Tm (AAA) Tj ET"))
	if err != nil {
		t.Fatal(err)
	}
	font := NewSimpleFont("F1")
	font.vertical = true
	font.verticalWidths['A'] = -30.206609632876535
	got := InterpretTextWithFonts(ops, map[string]*Font{"F1": font})
	if len(got) != 1 || len(got[0].glyphs) != 3 {
		t.Fatalf("text objects = %#v", got)
	}
	step := font.VDisp('A') * 11.7
	wantX, wantY := (geometry.Matrix{0.1, 0.9, -0.7032711038139268, 0.2, -961.6893795133581, 7.3}).Point(0, 2*step)
	if origin := got[0].glyphs[2].Origin(); origin != [2]float64{wantX, wantY} {
		t.Fatalf("third rotated vertical glyph origin = %.17g, %.17g, want %.17g, %.17g", origin[0], origin[1], wantX, wantY)
	}
}

func TestInterpretTextKeepsSkewedGlyphOffsetAcrossOperators(t *testing.T) {
	ops, err := ParseContent([]byte("BT /F1 11.7 Tf 0.1 0.9 -0.8 0.2 89.9 7.3 Tm (A) Tj (A) Tj (A) Tj ET"))
	if err != nil {
		t.Fatal(err)
	}
	font := NewSimpleFont("F1")
	font.widths['A'] = 3
	got := InterpretTextWithFonts(ops, map[string]*Font{"F1": font})
	if len(got) != 3 || len(got[2].glyphs) != 1 {
		t.Fatalf("text objects = %#v", got)
	}
	step := math.FMA(font.HDisp('A'), 11.7, 0)
	wantX, wantY := (geometry.Matrix{0.1, 0.9, -0.8, 0.2, 89.9, 7.3}).Point(2*step, 0)
	if origin := got[2].glyphs[0].Origin(); origin != [2]float64{wantX, wantY} {
		t.Fatalf("third skewed text-object origin = %.17g, %.17g, want %.17g, %.17g", origin[0], origin[1], wantX, wantY)
	}
}

func TestInterpretTextKeepsRotatedVerticalGlyphOffsetAcrossOperators(t *testing.T) {
	ops, err := ParseContent([]byte("BT /F1 11.7 Tf 0.1 0.9 -0.7032711038139268 0.2 -961.6893795133581 7.3 Tm (A) Tj (A) Tj (A) Tj ET"))
	if err != nil {
		t.Fatal(err)
	}
	font := NewSimpleFont("F1")
	font.vertical = true
	font.verticalWidths['A'] = -30.206609632876535
	got := InterpretTextWithFonts(ops, map[string]*Font{"F1": font})
	if len(got) != 3 || len(got[2].glyphs) != 1 {
		t.Fatalf("text objects = %#v", got)
	}
	step := math.FMA(font.VDisp('A'), 11.7, 0)
	wantX, wantY := (geometry.Matrix{0.1, 0.9, -0.7032711038139268, 0.2, -961.6893795133581, 7.3}).Point(0, 2*step)
	if origin := got[2].glyphs[0].Origin(); origin != [2]float64{wantX, wantY} {
		t.Fatalf("third rotated vertical text-object origin = %.17g, %.17g, want %.17g, %.17g", origin[0], origin[1], wantX, wantY)
	}
}

func TestInterpretTextKeepsGlyphOffsetAcrossTjAndTJ(t *testing.T) {
	ops, err := ParseContent([]byte("BT /F1 11.7 Tf 0.1 0.9 -0.8 0.2 89.9 7.3 Tm (A) Tj [(A)] TJ (A) Tj ET"))
	if err != nil {
		t.Fatal(err)
	}
	font := NewSimpleFont("F1")
	font.widths['A'] = 3
	got := InterpretTextWithFonts(ops, map[string]*Font{"F1": font})
	if len(got) != 3 {
		t.Fatalf("text objects = %#v", got)
	}
	step := math.FMA(font.HDisp('A'), 11.7, 0)
	base := geometry.Matrix{0.1, 0.9, -0.8, 0.2, 89.9, 7.3}
	for i, object := range got {
		wantX, wantY := base.Point(float64(i)*step, 0)
		if origin := object.Origin(); origin != [2]float64{wantX, wantY} {
			t.Fatalf("text object %d origin = %.17g, %.17g, want %.17g, %.17g", i, origin[0], origin[1], wantX, wantY)
		}
	}
}

func TestInterpretTextLineOperatorsResetGlyphOffset(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    [2]float64
	}{
		{name: "BT", content: "BT /F1 10 Tf (A) Tj BT (A) Tj ET", want: [2]float64{0, 0}},
		{name: "Tm", content: "BT /F1 10 Tf (A) Tj 1 0 0 1 20 30 Tm (A) Tj ET", want: [2]float64{20, 30}},
		{name: "Td", content: "BT /F1 10 Tf 1 0 0 1 20 30 Tm (A) Tj 5 6 Td (A) Tj ET", want: [2]float64{25, 36}},
		{name: "TD", content: "BT /F1 10 Tf 1 0 0 1 20 30 Tm (A) Tj 5 6 TD (A) Tj ET", want: [2]float64{25, 36}},
		{name: "T*", content: "BT /F1 10 Tf 12 TL 1 0 0 1 20 30 Tm (A) Tj T* (A) Tj ET", want: [2]float64{20, 18}},
	}
	font := NewSimpleFont("F1")
	font.widths['A'] = 500
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ops, err := ParseContent([]byte(test.content))
			if err != nil {
				t.Fatal(err)
			}
			got := InterpretTextWithFonts(ops, map[string]*Font{"F1": font})
			if len(got) != 2 {
				t.Fatalf("text objects = %#v", got)
			}
			if origin := got[1].Origin(); origin != test.want {
				t.Fatalf("origin after %s = %v, want %v", test.name, origin, test.want)
			}
		})
	}
}

func TestInterpretTextAppliesVerticalSpacingBeforeScaling(t *testing.T) {
	ops, err := ParseContent([]byte("BT /F1 40.7057950468858 Tf -4.087587045609464 Tc -3.5817409808724676 Tw 1 0 0 1 0 0 Tm ( A) Tj ET"))
	if err != nil {
		t.Fatal(err)
	}
	font := NewSimpleFont("F1")
	font.vertical = true
	font.verticalWidths[' '] = -1409.366381434621
	got := InterpretTextWithFonts(ops, map[string]*Font{"F1": font})
	if len(got) != 1 || len(got[0].glyphs) != 2 {
		t.Fatalf("text objects = %#v", got)
	}
	testSize := 40.7057950468858
	testCharSpace := -4.087587045609464
	testWordSpace := -3.5817409808724676
	displacement := font.VDisp(' ') + testCharSpace/testSize + testWordSpace/testSize
	step := math.FMA(displacement, testSize, 0)
	wantX, wantY := (geometry.Matrix{1, 0, 0, 1, 0, 0}).Point(0, step)
	if origin := got[0].glyphs[1].Origin(); origin != [2]float64{wantX, wantY} {
		t.Fatalf("vertical spacing origin = %.17g, %.17g, want %.17g, %.17g", origin[0], origin[1], wantX, wantY)
	}
}

func TestInterpretTextZeroSizeSuppressesVerticalSpacing(t *testing.T) {
	ops, err := ParseContent([]byte("BT /F1 0 Tf 2 Tc 3 Tw 0.1 0.9 -0.8 0.2 89.9 7.3 Tm ( A) Tj ET"))
	if err != nil {
		t.Fatal(err)
	}
	font := NewSimpleFont("F1")
	font.vertical = true
	got := InterpretTextWithFonts(ops, map[string]*Font{"F1": font})
	if len(got) != 1 || len(got[0].glyphs) != 2 {
		t.Fatalf("text objects = %#v", got)
	}
	if got[0].glyphs[1].Origin() != got[0].glyphs[0].Origin() {
		t.Fatalf("zero-size vertical spacing advanced from %v to %v", got[0].glyphs[0].Origin(), got[0].glyphs[1].Origin())
	}
}

func TestInterpretTextObjectBBoxIncludesTJAdjustmentOrigin(t *testing.T) {
	ops, _ := ParseContent([]byte("BT /F1 12 Tf 1 0 0 1 100 200 Tm [-4760 (A)] TJ ET"))
	f := NewSimpleFont("F1")
	f.ascent, f.descent = 1000, 0
	got := InterpretTextWithFonts(ops, map[string]*Font{"F1": f})
	if len(got) != 1 || len(got[0].glyphs) != 1 {
		t.Fatalf("%#v", got)
	}
	if got[0].glyphs[0].BBox() != [4]float64{157.12, 200, 163.12, 212} {
		t.Fatalf("glyph bbox = %v", got[0].glyphs[0].BBox())
	}
	if got[0].BBox() != [4]float64{100, 200, 163.12, 212} {
		t.Fatalf("text bbox = %v, want [100 200 163.12 212]", got[0].BBox())
	}
}

func TestInterpretTextObjectBBoxIgnoresTrailingTJAdjustment(t *testing.T) {
	ops, _ := ParseContent([]byte("BT /F1 12 Tf 1 0 0 1 100 200 Tm [(A) -500] TJ ET"))
	f := NewSimpleFont("F1")
	f.ascent, f.descent = 1000, 0
	got := InterpretTextWithFonts(ops, map[string]*Font{"F1": f})
	if len(got) != 1 || got[0].BBox() != got[0].glyphs[0].BBox() {
		t.Fatalf("text bbox = %v glyph bbox = %v", got[0].BBox(), got[0].glyphs[0].BBox())
	}
}

func TestInterpretTextObjectBBoxIgnoresInternalOverlapAdjustmentOrigin(t *testing.T) {
	ops, _ := ParseContent([]byte("BT /F1 10 Tf 1 0 0 1 100 200 Tm [(W) 500 (i)] TJ ET"))
	f := NewSimpleFont("F1")
	f.ascent, f.descent = 1000, 0
	f.widths['W'] = 800
	f.widths['i'] = 200
	got := InterpretTextWithFonts(ops, map[string]*Font{"F1": f})
	if len(got) != 1 || got[0].BBox()[2] != 105 {
		t.Fatalf("text bbox = %v, want right edge at final glyph", got)
	}
}

func TestInterpretTextTJAdjustmentStaysWithinTextObject(t *testing.T) {
	ops, _ := ParseContent([]byte("BT /F1 10 Tf 1 0 0 1 100 200 Tm [(A) -200 (B)] TJ ET"))
	got := InterpretTextWithFonts(ops, map[string]*Font{"F1": NewSimpleFont("F1")})
	if len(got) != 1 || got[0].Text() != "AB" {
		t.Fatalf("text objects = %#v, want one AB object", got)
	}
	if got[0].glyphs[1].Origin()[0] <= got[0].glyphs[0].BBox()[2] {
		t.Fatalf("second glyph origin = %v, want after first glyph", got[0].glyphs[1].Origin())
	}
}

func TestInterpretTextAcceptsTJArrayLeadingWhitespace(t *testing.T) {
	for _, prefix := range []string{" \n", "\r", "\r\n", "%comment\n"} {
		t.Run(strconv.Quote(prefix), func(t *testing.T) {
			ops, err := ParseContent([]byte("BT /F1 10 Tf 1 0 0 1 100 200 Tm [" + prefix + "(A) -100 (B)] TJ ET"))
			if err != nil {
				t.Fatal(err)
			}
			got := InterpretTextWithFonts(ops, map[string]*Font{"F1": NewSimpleFont("F1")})
			if len(got) != 1 || got[0].Text() != "AB" {
				t.Fatalf("text objects = %#v, want one AB object", got)
			}
		})
	}
}

func TestInterpretTextTStarResetsToLineOrigin(t *testing.T) {
	ops, _ := ParseContent([]byte("BT /F1 10 Tf 12 TL 1 0 0 1 10 20 Tm (AB) Tj T* (C) Tj ET"))
	got := InterpretText(ops)
	if len(got) != 2 || got[1].glyphs[0].Origin() != [2]float64{10, 8} {
		t.Fatalf("unexpected second line: %#v", got)
	}
}

func TestInterpretTextRotation(t *testing.T) {
	ops, _ := ParseContent([]byte("BT 0 1 -1 0 10 20 Tm /F1 10 Tf (A) Tj ET"))
	got := InterpretText(ops)
	if len(got) != 1 || got[0].Rotation() < 89 || got[0].Rotation() > 91 {
		t.Fatalf("%#v", got)
	}
}

func TestInterpretTextNormalizesNearZeroRotationNoise(t *testing.T) {
	ops, _ := ParseContent([]byte("BT 1 -0.0001 0 1 0 0 Tm /F1 10 Tf (A) Tj ET"))
	got := InterpretText(ops)
	if len(got) != 1 || got[0].Rotation() != 0 {
		t.Fatalf("near-zero rotation = %v, want 0", got[0].Rotation())
	}
}

func TestInterpretTextUsesAllTransformedGlyphCorners(t *testing.T) {
	ops, _ := ParseContent([]byte("BT 1 0 0.05 1 0 0 Tm /F1 10 Tf (A) Tj ET"))
	f := NewSimpleFont("F1")
	f.ascent, f.descent = 880, -120
	got := InterpretTextWithFonts(ops, map[string]*Font{"F1": f})
	if len(got) != 1 || len(got[0].glyphs) != 1 {
		t.Fatalf("%#v", got)
	}
	// The oblique text matrix makes the top and bottom corners extend
	// beyond the diagonal pair; Playa includes all four corners.
	want := [4]float64{-0.06, -1.2, 5.44, 8.8}
	for i := range want {
		if got[0].glyphs[0].BBox()[i] != want[i] {
			t.Fatalf("bbox[%d] = %v, want %v (bbox=%v)", i, got[0].glyphs[0].BBox()[i], want[i], got[0].glyphs[0].BBox())
		}
	}
}

func TestInterpretTextVerticalDisplacement(t *testing.T) {
	ops, _ := ParseContent([]byte("BT /F1 10 Tf (AB) Tj ET"))
	f := NewSimpleFont("Vertical")
	f.vertical = true
	f.ascent, f.descent = 880, -120
	f.cid = false
	got := InterpretTextWithFonts(ops, map[string]*Font{"F1": f})
	if len(got) != 1 || len(got[0].glyphs) != 2 {
		t.Fatalf("unexpected text: %#v", got)
	}
	if got[0].Displacement() != [2]float64{0, -20} || got[0].glyphs[0].Displacement() != [2]float64{0, -10} || got[0].glyphs[1].Origin() != [2]float64{0, -10} {
		t.Fatalf("unexpected vertical glyphs: %#v", got[0].glyphs)
	}
}

func TestInterpretTextVerticalGlyphMatrixUsesTextOrigin(t *testing.T) {
	ops, _ := ParseContent([]byte("BT /F1 10 Tf (A) Tj ET"))
	f := NewSimpleFont("Vertical")
	f.vertical = true
	f.ascent, f.descent = 880, -120
	got := InterpretTextWithFonts(ops, map[string]*Font{"F1": f})
	if len(got) != 1 || len(got[0].glyphs) != 1 {
		t.Fatalf("unexpected vertical text: %#v", got)
	}
	if got[0].glyphs[0].Matrix()[4] != 0 || got[0].glyphs[0].Matrix()[5] != 0 {
		matrix := got[0].glyphs[0].Matrix()
		t.Fatalf("vertical glyph matrix translation = %v, want text origin", matrix[4:])
	}
}

func TestInterpretTextVerticalSizeUsesVerticalRenderingAxis(t *testing.T) {
	ops, _ := ParseContent([]byte("BT 2 0 0 1 0 0 Tm /F1 10 Tf (A) Tj ET"))
	f := NewSimpleFont("Vertical")
	f.vertical = true
	got := InterpretTextWithFonts(ops, map[string]*Font{"F1": f})
	if len(got) != 1 || len(got[0].glyphs) != 1 {
		t.Fatalf("unexpected vertical text: %#v", got)
	}
	if got[0].Size() != 20 || got[0].glyphs[0].Size() != 20 {
		t.Fatalf("vertical sizes = text %v glyph %v, want 20", got[0].Size(), got[0].glyphs[0].Size())
	}
}

func TestInterpretCIDDecodedSpaceDoesNotUseWordSpacing(t *testing.T) {
	ops, _ := ParseContent([]byte("BT /F1 10 Tf 10 Tw <0003> Tj ET"))
	f := NewSimpleFont("F1")
	f.cid = true
	f.cmap = identityCMap(false)
	f.toUnicode[3] = " "
	f.codeMap[string([]byte{0, 3})] = " "
	f.cidWidths[3] = 278
	got := InterpretTextWithFonts(ops, map[string]*Font{"F1": f})
	if len(got) != 1 || len(got[0].glyphs) != 1 {
		t.Fatalf("objects = %#v", got)
	}
	if dx := got[0].glyphs[0].Displacement()[0]; dx != 2.7800000000000002 {
		t.Fatalf("CID decoded space displacement = %v, want width without word spacing", dx)
	}
}

func TestInterpretCID32UsesWordSpacing(t *testing.T) {
	ops, _ := ParseContent([]byte("BT /F1 10 Tf 10 Tw <0020> Tj ET"))
	f := NewSimpleFont("F1")
	f.cid = true
	f.cmap = identityCMap(false)
	f.toUnicode[32] = "x"
	f.codeMap[string([]byte{0, 32})] = "x"
	f.cidWidths[32] = 278
	got := InterpretTextWithFonts(ops, map[string]*Font{"F1": f})
	if len(got) != 1 || len(got[0].glyphs) != 1 {
		t.Fatalf("objects = %#v", got)
	}
	if dx := got[0].glyphs[0].Displacement()[0]; dx != 12.780000000000001 {
		t.Fatalf("CID 32 displacement = %v, want width with word spacing", dx)
	}
}

func TestInterpretVerticalCIDDecodedSpaceDoesNotUseWordSpacing(t *testing.T) {
	ops, _ := ParseContent([]byte("BT /F1 10 Tf 10 Tw <0003> Tj ET"))
	f := NewSimpleFont("F1")
	f.cid = true
	f.vertical = true
	f.cmap = identityCMap(false)
	f.toUnicode[3] = " "
	f.codeMap[string([]byte{0, 3})] = " "
	f.verticalWidths[3] = -1000
	got := InterpretTextWithFonts(ops, map[string]*Font{"F1": f})
	if len(got) != 1 || len(got[0].glyphs) != 1 {
		t.Fatalf("objects = %#v", got)
	}
	if dy := got[0].glyphs[0].Displacement()[1]; dy != -10 {
		t.Fatalf("vertical CID decoded space displacement = %v, want width without word spacing", dy)
	}
}

func TestInterpretVerticalCID32UsesWordSpacing(t *testing.T) {
	ops, _ := ParseContent([]byte("BT /F1 10 Tf 10 Tw <0020> Tj ET"))
	f := NewSimpleFont("F1")
	f.cid = true
	f.vertical = true
	f.cmap = identityCMap(false)
	f.toUnicode[32] = "x"
	f.codeMap[string([]byte{0, 32})] = "x"
	f.verticalWidths[32] = -1000
	got := InterpretTextWithFonts(ops, map[string]*Font{"F1": f})
	if len(got) != 1 || len(got[0].glyphs) != 1 {
		t.Fatalf("objects = %#v", got)
	}
	if dy := got[0].glyphs[0].Displacement()[1]; dy != 0 {
		t.Fatalf("vertical CID 32 displacement = %v, want width with word spacing", dy)
	}
}

func TestInterpretCIDEmbeddedUnicodeIsNotMarkedUnmapped(t *testing.T) {
	ops, _ := ParseContent([]byte("BT /F1 10 Tf <0041> Tj ET"))
	f := NewSimpleFont("F1")
	f.cid = true
	f.cmap = identityCMap(false)
	f.glyphIDToUnicode = map[int]string{0x41: "A"}
	got := InterpretTextWithFonts(ops, map[string]*Font{"F1": f})
	if len(got) != 1 || got[0].Text() != "A" || got[0].Unmapped() {
		t.Fatalf("embedded Unicode text = %#v", got)
	}
}

func TestInterpretCIDEmbeddedUnicodeMarksOnlyUnmappedGlyphs(t *testing.T) {
	ops, _ := ParseContent([]byte("BT /F1 10 Tf <00410042> Tj ET"))
	f := NewSimpleFont("F1")
	f.cid = true
	f.cmap = identityCMap(false)
	f.glyphIDToUnicode = map[int]string{0x41: "A"}
	got := InterpretTextWithFonts(ops, map[string]*Font{"F1": f})
	if len(got) != 1 || len(got[0].glyphs) != 2 || got[0].glyphs[0].Unmapped() || !got[0].glyphs[1].Unmapped() || !got[0].Unmapped() {
		t.Fatalf("mixed embedded Unicode text = %#v", got)
	}
}

func TestInterpretCIDToUnicodeMarksOnlyMappedGlyphs(t *testing.T) {
	ops, _ := ParseContent([]byte("BT /F1 10 Tf <00410042> Tj ET"))
	f := NewSimpleFont("F1")
	f.cid = true
	f.cmap = identityCMap(false)
	f.toUnicode[0x41] = "A"
	got := InterpretTextWithFonts(ops, map[string]*Font{"F1": f})
	if len(got) != 1 || len(got[0].glyphs) != 2 || got[0].glyphs[0].Unmapped() || !got[0].glyphs[1].Unmapped() || !got[0].Unmapped() {
		t.Fatalf("mixed ToUnicode text = %#v", got)
	}
}

func TestInterpretVerticalCIDGlyphOriginUsesTextOrigin(t *testing.T) {
	ops, _ := ParseContent([]byte("BT /F1 10 Tf 1 0 0 1 100 200 Tm <0001> Tj ET"))
	f := NewSimpleFont("Vertical")
	f.cid = true
	f.vertical = true
	f.cmap = identityCMap(true)
	f.toUnicode[1] = "、"
	f.defaultWidth = 900
	f.defaultVWidth = -700
	f.defaultVPosition = [2]float64{450, 880}
	got := InterpretTextWithFonts(ops, map[string]*Font{"F1": f})
	if len(got) != 1 || len(got[0].glyphs) != 1 {
		t.Fatalf("unexpected vertical glyphs: %#v", got)
	}
	if got[0].glyphs[0].Origin() != [2]float64{100, 200} {
		t.Fatalf("vertical glyph origin = %v, want text origin [100 200]", got[0].glyphs[0].Origin())
	}
}

func TestInterpretType3UsesFontMatrix(t *testing.T) {
	ops, _ := ParseContent([]byte("BT /F1 10 Tf (A) Tj ET"))
	f := NewSimpleFont("Type3")
	f.type3 = true
	f.fontMatrix = geometry.Matrix{0.002, 0, 0, 0.002, 0, 0}
	f.ascent, f.descent = 1000, 0
	got := InterpretTextWithFonts(ops, map[string]*Font{"F1": f})
	if len(got) != 1 || got[0].glyphs[0].Displacement()[0] != 10 || got[0].glyphs[0].BBox() != [4]float64{0, 0, 10, 20} {
		t.Fatalf("unexpected Type3 geometry: %#v", got)
	}
}

func TestType3GlyphPathsSequenceUsesCharProc(t *testing.T) {
	font := NewSimpleFont("Type3")
	font.type3 = true
	font.encoding[65] = 'A'
	font.glyphNames[65] = "A"
	font.fontMatrix = geometry.Matrix{0.001, 0, 0, 0.001, 0, 0}
	parseType3CharProc(&Document{}, font, "A", newStream(nil, []byte(
		"500 0 0 0 100 100 d1 0 0 m 100 0 l S",
	)))
	ops, err := ParseContent([]byte("BT /F1 100 Tf (A) Tj ET"))
	if err != nil {
		t.Fatal(err)
	}
	texts := InterpretTextWithFonts(ops, map[string]*Font{"F1": font})
	if len(texts) != 1 || len(texts[0].glyphs) != 1 {
		t.Fatalf("type3 text = %#v", texts)
	}
	var paths []PathObject
	for path, err := range texts[0].glyphs[0].PathsSeq() {
		if err != nil {
			t.Fatal(err)
		}
		paths = append(paths, path)
	}
	if len(paths) != 1 || paths[0].BBox() != [4]float64{0, 0, 10, 0} {
		t.Fatalf("type3 glyph paths = %#v", paths)
	}
	ordinaryPaths := 0
	for _, err := range newTestGlyph(contentdata.GlyphSpec{Text: "A"}, nil, nil, geometry.Matrix{}).PathsSeq() {
		if err != nil {
			t.Fatal(err)
		}
		ordinaryPaths++
	}
	if ordinaryPaths != 0 {
		t.Fatal("ordinary glyph unexpectedly exposed Type3 paths")
	}
}

func TestType3GlyphPathsSequenceReportsLazyCharProcError(t *testing.T) {
	font := NewSimpleFont("Type3")
	font.type3 = true
	font.document = &Document{}
	font.glyphNames[65] = "A"
	font.charProcs["A"] = newStream(nil, []byte("500 0 0 0 (bad) 100 d1"))

	glyph := newTestGlyphWithCode(font, []byte{65}, 0, identity())
	var got error
	for _, err := range glyph.PathsSeq() {
		if err != nil {
			got = err
			break
		}
	}
	if got == nil || !strings.Contains(got.Error(), "Type3 CharProc") {
		t.Fatalf("lazy Type3 error = %v", got)
	}
}

func TestType3GlyphPathsSequenceReportsMalformedD1Operands(t *testing.T) {
	font := NewSimpleFont("Type3")
	font.type3 = true
	font.document = &Document{}
	font.glyphNames[65] = "A"
	font.charProcs["A"] = newStream(nil, []byte("500 0 0 0 100 100 1 d1"))

	glyph := newTestGlyphWithCode(font, []byte{65}, 0, identity())
	var got error
	for _, err := range glyph.PathsSeq() {
		if err != nil {
			got = err
			break
		}
	}
	if got == nil || !strings.Contains(got.Error(), "invalid Type3 CharProc") {
		t.Fatalf("malformed Type3 d1 error = %v", got)
	}
}

func TestType3GlyphPathsSequenceMaterializesRawCharProc(t *testing.T) {
	d := &Document{}
	font := NewSimpleFont("Type3")
	font.type3 = true
	font.document = d
	font.glyphNames[65] = "A"
	font.charProcs["A"] = newStream(nil, []byte(
		"500 0 0 0 100 100 d1 0 0 m 100 0 l S",
	))
	glyph := newTestGlyphWithCode(font, []byte{65}, 0, identity())
	var paths []PathObject
	for path, err := range glyph.PathsSeq() {
		if err != nil {
			t.Fatal(err)
		}
		paths = append(paths, path)
	}
	if len(paths) != 1 || len(paths[0].RawSegmentsCopy()) != 2 {
		t.Fatalf("raw Type3 glyph paths = %#v", paths)
	}
}

func TestType3GlyphContentSequenceExposesInterpreterChildren(t *testing.T) {
	d := &Document{}
	font := NewSimpleFont("Type3")
	font.type3 = true
	font.document = d
	font.glyphNames[65] = "A"
	font.charProcs["A"] = newStream(nil, []byte(
		"500 0 0 0 100 100 d1 0 0 m 100 0 l S",
	))
	glyph := newTestGlyphWithCode(font, []byte{65}, 0, identity())

	var objects []ContentObject
	for object, err := range glyph.ContentSeq(d) {
		if err != nil {
			t.Fatal(err)
		}
		objects = append(objects, object)
	}
	if len(objects) != 1 || objects[0].ObjectType() != "path" {
		t.Fatalf("Type3 glyph content = %#v", objects)
	}
	count, err := glyph.Len(d)
	if err != nil || count != len(objects) {
		t.Fatalf("Type3 glyph len = %d, err=%v; objects=%d", count, err, len(objects))
	}

	second := 0
	for _, err := range glyph.ContentSeq(d) {
		if err != nil {
			t.Fatal(err)
		}
		second++
	}
	if second != len(objects) {
		t.Fatalf("Type3 glyph content was not repeatable: first=%d second=%d", len(objects), second)
	}
}

func TestCFFGlyphPathsSequenceUsesEmbeddedCharString(t *testing.T) {
	font := NewSimpleFont("CFF")
	font.cffCharstrings = [][]byte{{14}, {139, 139, 21, 239, 139, 5, 14}}
	font.cffGlyphIDs[65] = 1
	font.fontMatrix = geometry.Matrix{0.001, 0, 0, 0.001, 0, 0}
	glyph := newTestGlyphWithCode(font, []byte{65}, 0, identity())
	var paths []PathObject
	for path, err := range glyph.PathsSeq() {
		if err != nil {
			t.Fatal(err)
		}
		paths = append(paths, path)
	}
	if len(paths) != 1 || !paths[0].Fill() || len(paths[0].RawSegmentsCopy()) != 3 {
		t.Fatalf("CFF glyph paths = %#v", paths)
	}
	if paths[0].RawSegmentsCopy()[0].Operator() != "m" || paths[0].RawSegmentsCopy()[1].Operator() != "l" {
		t.Fatalf("CFF glyph path segments = %#v", paths[0].RawSegmentsCopy())
	}
}

func TestCFFGlyphPathsSequenceReportsMalformedLazyProgram(t *testing.T) {
	font := NewSimpleFont("CFF")
	font.cffData = []byte{1, 0, 4}
	glyph := newTestGlyphWithCode(font, []byte{65}, 0, identity())
	for _, err := range glyph.PathsSeq() {
		if err == nil {
			t.Fatal("malformed CFF produced a path without error")
		}
		return
	}
	t.Fatal("malformed CFF produced no error")
}

func TestCFFGlyphPathsSequenceMaterializesLazyProgram(t *testing.T) {
	prefix := []byte{1, 0, 4, 4}
	prefix = appendCFFIndex(prefix, []byte("Test"))
	prefix = appendCFFIndex(prefix, []byte{0, 17})
	prefix = appendCFFIndex(prefix)
	prefix = appendCFFIndex(prefix)
	charStringsOffset := len(prefix)

	data := []byte{1, 0, 4, 4}
	data = appendCFFIndex(data, []byte("Test"))
	data = appendCFFIndex(data, []byte{byte(139 + charStringsOffset), 17})
	data = appendCFFIndex(data)
	data = appendCFFIndex(data)
	data = append(data, 0, 2, 1, 1, 8, 15)
	data = append(data, []byte{14, 14, 14, 14, 14, 14, 14}...)
	data = append(data, []byte{139, 139, 21, 239, 139, 5, 14}...)

	font := NewSimpleFont("CFF")
	font.cffData = data
	// The PDF encoding selects the charset's space glyph for code 65.
	font.applyDifferences(Array{Number(65), Name("space")})
	glyph := newTestGlyphWithCode(font, []byte{65}, 0, identity())
	var paths []PathObject
	for path, err := range glyph.PathsSeq() {
		if err != nil {
			t.Fatal(err)
		}
		paths = append(paths, path)
	}
	if !font.cffParsed || len(paths) != 1 || len(paths[0].RawSegmentsCopy()) != 3 {
		t.Fatalf("lazy CFF glyph paths = parsed:%v charstrings:%#v glyphIDs:%#v paths:%#v", font.cffParsed, font.cffCharstrings, font.cffGlyphIDs, paths)
	}
}

func TestPageType3GlyphPathsSequenceFromResources(t *testing.T) {
	font := Dict{
		Name("Subtype"):    Name("Type3"),
		Name("FontMatrix"): Array{Number(0.001), Number(0), Number(0), Number(0.001), Number(0), Number(0)},
		Name("FontBBox"):   Array{Number(0), Number(0), Number(100), Number(100)},
		Name("Encoding"): Dict{
			Name("Differences"): Array{Number(65), Name("A")},
		},
		Name("CharProcs"): Dict{Name("A"): newStream(nil, []byte(
			"500 0 0 0 100 100 d1 0 0 m 100 0 l S",
		))},
	}
	d := &Document{}
	page := Page{dict: Dict{
		Name("Resources"): Dict{Name("Font"): Dict{Name("F1"): font}},
		Name("Contents"):  newStream(nil, []byte("BT /F1 100 Tf (A) Tj ET")),
	}}
	var paths []PathObject
	for glyph, err := range page.Glyphs(d) {
		if err != nil {
			t.Fatal(err)
		}
		for path, pathErr := range glyph.PathsSeq() {
			if pathErr != nil {
				t.Fatal(pathErr)
			}
			paths = append(paths, path)
		}
	}
	if len(paths) != 1 || paths[0].BBox() != [4]float64{0, 0, 10, 0} {
		t.Fatalf("page type3 glyph paths = %#v", paths)
	}
}

func TestPageType3GlyphPathsExpandCharProcResources(t *testing.T) {
	font := Dict{
		Name("Subtype"):    Name("Type3"),
		Name("FontMatrix"): Array{Number(0.001), Number(0), Number(0), Number(0.001), Number(0), Number(0)},
		Name("FontBBox"):   Array{Number(0), Number(0), Number(100), Number(100)},
		Name("Encoding"):   Dict{Name("Differences"): Array{Number(65), Name("A")}},
		Name("Resources"):  Dict{Name("XObject"): Dict{Name("Fm"): newStream(Dict{Name("Subtype"): Name("Form")}, []byte("0 0 m 100 0 l S"))}},
		Name("CharProcs"): Dict{Name("A"): newStream(nil, []byte(
			"500 0 0 0 100 100 d1 /Fm Do",
		))},
	}
	d := &Document{}
	page := Page{dict: Dict{
		Name("Resources"): Dict{Name("Font"): Dict{Name("F1"): font}},
		Name("Contents"):  newStream(nil, []byte("BT /F1 100 Tf (A) Tj ET")),
	}}
	paths := 0
	for glyph, err := range page.Glyphs(d) {
		if err != nil {
			t.Fatal(err)
		}
		for path, pathErr := range glyph.PathsSeq() {
			if pathErr != nil {
				t.Fatal(pathErr)
			}
			if path.BBox() != [4]float64{0, 0, 10, 0} {
				t.Fatalf("nested type3 path bbox = %#v", path.BBox())
			}
			paths++
		}
	}
	if paths != 1 {
		t.Fatalf("nested type3 paths = %d", paths)
	}
}

func TestParseInlineImage(t *testing.T) {
	ops, e := ParseContent([]byte("BI /W 1 /H 1 /BPC 8 /CS /G ID \x00 EI Q"))
	if e != nil {
		t.Fatal(e)
	}
	if len(ops) < 2 || ops[0].operatorValue() != "BI" {
		t.Fatalf("%#v", ops)
	}
	s, ok := ops[0].operandsValue()[0].(Stream)
	if !ok || len(s.DataBorrowed()) != 1 {
		t.Fatalf("%#v", ops[0])
	}
}

func TestParseInlineImageRequiresDelimitedEIMarker(t *testing.T) {
	ops, err := ParseContent([]byte("BI /W 1 /H 1 /BPC 8 /CS /G ID \x00 EI\x01\x02 EI Q"))
	if err != nil {
		t.Fatal(err)
	}
	if len(ops) == 0 {
		t.Fatal("no operations")
	}
	s, ok := ops[0].operandsValue()[0].(Stream)
	if !ok || string(s.DataBorrowed()) != "\x00 EI\x01\x02" {
		t.Fatalf("inline image data = %q", s.DataBorrowed())
	}
}

func TestParseInlineImagePreservesEmptyData(t *testing.T) {
	ops, err := ParseContent([]byte("BI /W 1 /H 1 /BPC 8 /CS /G /L 0 ID  EI Q"))
	if err != nil || len(ops) == 0 {
		t.Fatalf("empty inline image parse = %#v, err=%v", ops, err)
	}
	stream, ok := ops[0].operandsValue()[0].(Stream)
	if !ok || stream.DataBorrowed() == nil {
		t.Fatalf("empty inline image data became nil: %#v", ops[0])
	}
}

func TestParseInlineImageUsesLengthBeforeEIHeuristic(t *testing.T) {
	ops, err := ParseContent([]byte("BI /W 1 /H 1 /L 8 ID abc EI x EI Q"))
	if err != nil {
		t.Fatal(err)
	}
	if len(ops) == 0 {
		t.Fatal("no operations")
	}
	s, ok := ops[0].operandsValue()[0].(Stream)
	if !ok || string(s.DataBorrowed()) != "abc EI x" {
		t.Fatalf("inline image data = %q", s.DataBorrowed())
	}
}

func TestParseInlineImageASCIIHexUsesFilterSpecificEIMarker(t *testing.T) {
	ops, err := ParseContent([]byte("BI /W 1 /H 1 /F /AHx ID ABCEI Q"))
	if err != nil || len(ops) == 0 {
		t.Fatalf("ASCIIHex inline image parse = %#v, err=%v", ops, err)
	}
	stream, ok := ops[0].operandsValue()[0].(Stream)
	if !ok || string(stream.DataBorrowed()) != "ABC" {
		t.Fatalf("ASCIIHex inline image data = %q", stream.DataBorrowed())
	}
}

func TestParseInlineImageASCII85ExcludesEncodedTerminator(t *testing.T) {
	ops, err := ParseContent([]byte("BI /W 1 /H 1 /F /A85 ID xyz ~ \n > \r\nEI Q"))
	if err != nil || len(ops) == 0 {
		t.Fatalf("ASCII85 inline image parse = %#v, err=%v", ops, err)
	}
	stream, ok := ops[0].operandsValue()[0].(Stream)
	if !ok || string(stream.DataBorrowed()) != "xyz " {
		t.Fatalf("ASCII85 inline image data = %q", stream.DataBorrowed())
	}
}

func TestParseInlineImageRejectsOverflowingLengthWithoutPanic(t *testing.T) {
	length := strconv.FormatInt(int64(^uint(0)>>1), 10)
	ops, err := ParseContent([]byte("BI /W 1 /H 1 /L " + length + " ID abc EI Q"))
	if err != nil {
		t.Fatal(err)
	}
	if len(ops) == 0 {
		t.Fatal("no operations")
	}
	s, ok := ops[0].operandsValue()[0].(Stream)
	if !ok || string(s.DataBorrowed()) != "abc" {
		t.Fatalf("inline image data = %q", s.DataBorrowed())
	}
}
