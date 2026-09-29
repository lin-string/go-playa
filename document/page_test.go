package document

import (
	"errors"
	"iter"
	"math"
	"strings"
	"testing"

	"github.com/lin-string/go-playa/cacheconfig"
	"github.com/lin-string/go-playa/geometry"
)

func TestZeroPageAccessorsAcceptNilDocument(t *testing.T) {
	page := Page{number: 1}
	if got := page.Label(nil); got != "" {
		t.Fatalf("zero page label = %q, want empty", got)
	}
	if got := page.BBox(nil); got != [4]float64{0, 0, 612, 792} {
		t.Fatalf("zero page bbox = %v, want US Letter", got)
	}
	if got := page.Resources(nil); got != nil {
		t.Fatalf("zero page resources = %#v, want nil", got)
	}
	if got := page.UserUnit(nil); got != 1 {
		t.Fatalf("zero page user unit = %v, want 1", got)
	}
	if got := page.Rotation(nil); got != 0 {
		t.Fatalf("zero page rotation = %d, want 0", got)
	}
	if _, err := page.Content(nil); err != errNilDocument {
		t.Fatalf("zero page content error = %v, want %v", err, errNilDocument)
	}
}

func TestPageResourcesWithErrorReportsMalformedIndirectResource(t *testing.T) {
	d := &Document{objects: map[Ref]Object{{Object: 1}: Number(7)}}
	p := Page{dict: Dict{Name("Resources"): Ref{Object: 1}}}
	if resources, err := p.ResourcesWithError(d); err == nil || resources != nil {
		t.Fatalf("malformed resources = %#v, err=%v", resources, err)
	}
	if got := p.Resources(d); got != nil {
		t.Fatalf("lenient malformed resources = %#v, want nil", got)
	}
}

func TestPageResourcesWithErrorReportsUnresolvedEntry(t *testing.T) {
	d := &Document{}
	p := Page{dict: Dict{Name("Resources"): Dict{
		Name("Font"): Dict{Name("F1"): Ref{Object: 99}},
	}}}
	if resources, err := p.ResourcesWithError(d); err == nil || resources != nil {
		t.Fatalf("unresolved resource entry = %#v, err=%v", resources, err)
	}
}

func TestPageResourcesWithErrorCachesTerminalErrors(t *testing.T) {
	ref := Ref{Object: 42}
	d := &Document{}
	p := Page{ref: ref, dict: Dict{Name("Resources"): Number(1)}}

	_, firstErr := p.ResourcesWithError(d)
	_, secondErr := p.ResourcesWithError(d)
	if firstErr == nil || secondErr == nil {
		t.Fatalf("resource errors = %v, %v; want both non-nil", firstErr, secondErr)
	}
	if firstErr != secondErr {
		t.Fatalf("resource error was not cached: first=%p second=%p", firstErr, secondErr)
	}
	if got := d.pageResourceErrors[ref]; got != firstErr {
		t.Fatalf("cached resource error = %v, want %v", got, firstErr)
	}
}

func TestPageResourcesWithErrorHonorsErrorCacheBudget(t *testing.T) {
	ref := Ref{Object: 42}
	d := &Document{cacheOptionsConfigured: true, cacheOptions: cacheconfig.Options{PageResourceErrorBytes: 0}}
	p := Page{ref: ref, dict: Dict{Name("Resources"): Number(1)}}
	_, firstErr := p.ResourcesWithError(d)
	_, secondErr := p.ResourcesWithError(d)
	if firstErr == nil || secondErr == nil || firstErr == secondErr {
		t.Fatalf("resource errors = %v/%v, want uncached independent errors", firstErr, secondErr)
	}
	if len(d.pageResourceErrors) != 0 {
		t.Fatalf("page resource error cache retained entry with zero budget: %#v", d.pageResourceErrors)
	}
}

func TestPageContentAccessorsSkipUnresolvedContents(t *testing.T) {
	d := &Document{objects: map[Ref]Object{}}
	p := Page{dict: Dict{Name("Contents"): Ref{Object: 99}}}
	if content, err := p.Content(d); err != nil || content != nil {
		t.Fatalf("unresolved Content = %q, err=%v", content, err)
	}
	for stream, err := range p.Streams(d) {
		t.Fatalf("unresolved Streams item = %#v, err=%v, want none", stream, err)
	}
	for token, err := range p.Tokens(d) {
		t.Fatalf("unresolved Tokens item = %#v, err=%v, want none", token, err)
	}
}

func TestPageContentSkipsUnresolvedArrayContentItem(t *testing.T) {
	d := &Document{objects: map[Ref]Object{}}
	p := Page{dict: Dict{Name("Contents"): Array{Ref{Object: 99}}}}
	if content, err := p.Content(d); err != nil || content != nil {
		t.Fatalf("unresolved array content = %q, err=%v", content, err)
	}
}

func TestPageContentAccessorsRejectNonStreamContents(t *testing.T) {
	d := &Document{objects: map[Ref]Object{}}
	p := Page{dict: Dict{Name("Contents"): Dict{Name("Length"): Number(0)}}}
	if content, err := p.Content(d); err == nil || content != nil {
		t.Fatalf("Content = %q, err=%v; want nil and a type error", content, err)
	}
	for _, iterator := range []struct {
		name string
		run  func() error
	}{
		{name: "Streams", run: func() error {
			for _, err := range p.Streams(d) {
				return err
			}
			return nil
		}},
		{name: "Tokens", run: func() error {
			_, err := p.CollectTokens(d)
			return err
		}},
		{name: "Contents", run: func() error {
			_, err := p.ContentOps(d)
			return err
		}},
	} {
		t.Run(iterator.name, func(t *testing.T) {
			if err := iterator.run(); err == nil {
				t.Fatal("malformed Contents produced no error")
			}
		})
	}
}

func TestPageContentAndResourcesFollowMultiLevelIndirectValues(t *testing.T) {
	d := &Document{objects: map[Ref]Object{
		{Object: 1}: Ref{Object: 2},
		{Object: 2}: newStream(nil, []byte("BT (hello) Tj ET")),
		{Object: 3}: Ref{Object: 4},
		{Object: 4}: Dict{Name("ProcSet"): Array{Name("PDF")}},
	}}
	p := Page{dict: Dict{
		Name("Contents"):  Ref{Object: 1},
		Name("Resources"): Ref{Object: 3},
	}}

	content, err := p.Content(d)
	if err != nil {
		t.Fatalf("Content() error = %v", err)
	}
	if string(content) != "BT (hello) Tj ET\n" {
		t.Fatalf("Content() = %q", content)
	}
	resources := p.Resources(d)
	if resources == nil || resources[Name("ProcSet")] == nil {
		t.Fatalf("Resources() = %#v", resources)
	}
	var streams []Stream
	for stream, err := range p.Streams(d) {
		if err != nil {
			t.Fatalf("Streams() error = %v", err)
		}
		streams = append(streams, stream)
	}
	if len(streams) != 1 || string(streams[0].DataBorrowed()) != "BT (hello) Tj ET" {
		t.Fatalf("Streams() = %#v", streams)
	}
	if ref, ok := streams[0].Ref(); !ok || ref != (Ref{Object: 2}) {
		t.Fatalf("stream Ref() = %#v, %v, want final indirect stream 2 0 R", ref, ok)
	}
}

func TestPageContentDecodesArrayStreamsInOrder(t *testing.T) {
	d := &Document{objects: map[Ref]Object{
		{Object: 1}: newStream(nil, []byte("BT")),
		{Object: 2}: newStream(nil, []byte("ET")),
	}}
	p := Page{dict: Dict{Name("Contents"): Array{Ref{Object: 1}, Ref{Object: 2}}}}
	content, err := p.Content(d)
	if err != nil {
		t.Fatalf("Content() error = %v", err)
	}
	if string(content) != "BT\nET\n" {
		t.Fatalf("Content() = %q, want %q", content, "BT\nET\n")
	}
}

func TestPageGeometryAndContentsFollowNestedIndirectValues(t *testing.T) {
	d := &Document{objects: map[Ref]Object{
		{Object: 1}: Ref{Object: 2},
		{Object: 2}: Array{Ref{Object: 3}, Ref{Object: 4}, Ref{Object: 5}, Ref{Object: 6}},
		{Object: 3}: Number(0), {Object: 4}: Number(0),
		{Object: 5}: Ref{Object: 7}, {Object: 6}: Number(200),
		{Object: 7}:  Number(300),
		{Object: 8}:  Ref{Object: 9},
		{Object: 9}:  Number(2),
		{Object: 10}: Ref{Object: 11},
		{Object: 11}: newStream(nil, []byte("BT (nested) Tj ET")),
	}}
	p := Page{dict: Dict{
		Name("MediaBox"): Ref{Object: 1},
		Name("UserUnit"): Ref{Object: 8},
		Name("Contents"): Ref{Object: 10},
	}}
	if got := p.MediaBox(d); got != [4]float64{0, 0, 300, 200} {
		t.Fatalf("nested media box = %v", got)
	}
	if got := p.UserUnit(d); got != 2 {
		t.Fatalf("nested user unit = %v", got)
	}
	content, err := p.Content(d)
	if err != nil || string(content) != "BT (nested) Tj ET\n" {
		t.Fatalf("nested content = %q, err=%v", content, err)
	}
}

func TestPageModelNormalizesBBoxAndReadsInheritedProperties(t *testing.T) {
	d := &Document{objects: map[Ref]Object{}}
	p := Page{dict: Dict{
		Name("MediaBox"):  Array{Number(200), Number(300), Number(0), Number(-100)},
		Name("UserUnit"):  Number(2),
		Name("Resources"): Dict{Name("ProcSet"): Array{Name("PDF")}},
	}}
	if got := p.BBox(d); got != [4]float64{0, -100, 200, 300} {
		t.Fatalf("bbox = %v", got)
	}
	if p.UserUnit(d) != 2 || p.Resources(d) == nil {
		t.Fatalf("page properties not exposed")
	}
}

func TestPageMatrixTranslatesAndRotatesPageBox(t *testing.T) {
	d := &Document{objects: map[Ref]Object{}}
	p := Page{dict: Dict{Name("MediaBox"): Array{Number(10), Number(20), Number(110), Number(220)}, Name("Rotate"): Number(90)}}
	m := p.Matrix(d)
	if m != (geometry.Matrix{0, -1, 1, 0, -20, 110}) {
		t.Fatalf("page matrix = %v", m)
	}
	x, y := m.Point(10, 20)
	if x != 0 || y != 100 {
		t.Fatalf("page matrix origin = %v,%v", x, y)
	}
}

func TestPageMatrixAppliesUserUnit(t *testing.T) {
	d := &Document{objects: map[Ref]Object{}}
	p := Page{dict: Dict{Name("MediaBox"): Array{Number(0), Number(0), Number(100), Number(200)}, Name("UserUnit"): Number(2)}}
	if got := p.Matrix(d); got != (geometry.Matrix{2, 0, 0, 2, 0, 0}) {
		t.Fatalf("user unit matrix = %v", got)
	}
}

func TestPageRotationFollowsMultiLevelIndirectValue(t *testing.T) {
	d := &Document{objects: map[Ref]Object{
		{Object: 1}: Ref{Object: 2}, {Object: 2}: Ref{Object: 3}, {Object: 3}: Number(-90),
	}}
	p := Page{dict: Dict{Name("Rotate"): Ref{Object: 1}}}
	if got := p.Rotation(d); got != 270 {
		t.Fatalf("indirect page rotation = %d, want 270", got)
	}
}

func TestScanPagesFollowsMultiLevelIndirectType(t *testing.T) {
	d := &Document{objects: map[Ref]Object{
		{Object: 1}: Dict{Name("Type"): Ref{Object: 2}},
		{Object: 2}: Ref{Object: 3},
		{Object: 3}: Name("Page"),
	}}
	var pages []Page
	for page, err := range d.scanPagesSeq(false) {
		if err != nil {
			t.Fatal(err)
		}
		pages = append(pages, page)
	}
	if len(pages) != 1 || pages[0].ref != (Ref{Object: 1}) {
		t.Fatalf("scanned indirect page type = %#v", pages)
	}
}

func TestPageUserUnitRejectsNonFiniteValues(t *testing.T) {
	p := Page{dict: Dict{Name("UserUnit"): Number(math.Inf(1))}}
	if got := p.UserUnit(&Document{}); got != 1 {
		t.Fatalf("infinite UserUnit = %v, want default 1", got)
	}
}

func TestPageUserUnitWithErrorReportsMalformedIndirectValue(t *testing.T) {
	p := Page{dict: Dict{Name("UserUnit"): Ref{Object: 9}}}
	if _, err := p.UserUnitWithError(&Document{}); err == nil {
		t.Fatal("expected unresolved UserUnit error")
	}
	p = Page{dict: Dict{Name("UserUnit"): Number(0)}}
	if _, err := p.UserUnitWithError(&Document{}); err == nil {
		t.Fatal("expected invalid UserUnit error")
	}
}

func TestPageUserUnitCachesByPageReference(t *testing.T) {
	d := &Document{objects: map[Ref]Object{{Object: 9}: Number(2)}}
	p := Page{ref: Ref{Object: 7}, dict: Dict{Name("UserUnit"): Ref{Object: 9}}}
	first, err := p.UserUnitWithError(d)
	if err != nil || first != 2 {
		t.Fatalf("first UserUnit = %v, err=%v", first, err)
	}
	d.objects[Ref{Object: 9}] = Number(3)
	second, err := p.UserUnitWithError(d)
	if err != nil || second != first {
		t.Fatalf("cached UserUnit = %v, err=%v, want %v", second, err, first)
	}

	bad := &Document{}
	badPage := Page{ref: Ref{Object: 8}, dict: Dict{Name("UserUnit"): Ref{Object: 99}}}
	_, firstErr := badPage.UserUnitWithError(bad)
	_, secondErr := badPage.UserUnitWithError(bad)
	if firstErr == nil || secondErr != firstErr {
		t.Fatalf("cached UserUnit error = %v/%v", firstErr, secondErr)
	}
	bad.ReleaseTransientCaches()
	if _, err := badPage.UserUnitWithError(bad); err == nil || err == firstErr {
		t.Fatalf("UserUnit error survived transient-cache release: %v", err)
	}
}

func TestPageRotationWithErrorReportsMalformedIndirectValue(t *testing.T) {
	p := Page{dict: Dict{Name("Rotate"): Ref{Object: 9}}}
	if _, err := p.RotationWithError(&Document{}); err == nil {
		t.Fatal("expected unresolved Rotate error")
	}
	p = Page{dict: Dict{Name("Rotate"): String("90")}}
	if _, err := p.RotationWithError(&Document{}); err == nil {
		t.Fatal("expected invalid Rotate error")
	}
}

func TestPageRotationCachesByPageReference(t *testing.T) {
	d := &Document{objects: map[Ref]Object{{Object: 9}: Number(90)}}
	p := Page{ref: Ref{Object: 7}, dict: Dict{Name("Rotate"): Ref{Object: 9}}}
	first, err := p.RotationWithError(d)
	if err != nil || first != 90 {
		t.Fatalf("first Rotate = %v, err=%v", first, err)
	}
	d.objects[Ref{Object: 9}] = Number(180)
	second, err := p.RotationWithError(d)
	if err != nil || second != first {
		t.Fatalf("cached Rotate = %v, err=%v, want %v", second, err, first)
	}

	bad := &Document{}
	badPage := Page{ref: Ref{Object: 8}, dict: Dict{Name("Rotate"): Ref{Object: 99}}}
	_, firstErr := badPage.RotationWithError(bad)
	_, secondErr := badPage.RotationWithError(bad)
	if firstErr == nil || secondErr != firstErr {
		t.Fatalf("cached Rotate error = %v/%v", firstErr, secondErr)
	}
	bad.ReleaseTransientCaches()
	if _, err := badPage.RotationWithError(bad); err == nil || err == firstErr {
		t.Fatalf("Rotate error survived transient-cache release: %v", err)
	}
}

func TestPageGeometryErrorCachesHonorBudgets(t *testing.T) {
	d := &Document{cacheOptionsConfigured: true, cacheOptions: cacheconfig.Options{PageUserUnitErrorBytes: 0, PageRotationErrorBytes: 0}}
	userUnitPage := Page{ref: Ref{Object: 7}, dict: Dict{Name("UserUnit"): Ref{Object: 99}}}
	_, firstUserErr := userUnitPage.UserUnitWithError(d)
	_, secondUserErr := userUnitPage.UserUnitWithError(d)
	if firstUserErr == nil || secondUserErr == nil || firstUserErr == secondUserErr {
		t.Fatalf("UserUnit errors = %v/%v, want uncached independent errors", firstUserErr, secondUserErr)
	}
	rotationPage := Page{ref: Ref{Object: 8}, dict: Dict{Name("Rotate"): Ref{Object: 99}}}
	_, firstRotationErr := rotationPage.RotationWithError(d)
	_, secondRotationErr := rotationPage.RotationWithError(d)
	if firstRotationErr == nil || secondRotationErr == nil || firstRotationErr == secondRotationErr {
		t.Fatalf("Rotate errors = %v/%v, want uncached independent errors", firstRotationErr, secondRotationErr)
	}
	if len(d.pageUserUnitErrors) != 0 || len(d.pageRotationErrors) != 0 {
		t.Fatalf("geometry error caches retained entries with zero budget: user=%#v rotation=%#v", d.pageUserUnitErrors, d.pageRotationErrors)
	}
}

func TestPageParentKeyWithErrorReportsMalformedIndirectValue(t *testing.T) {
	p := Page{dict: Dict{Name("StructParents"): Ref{Object: 9}}}
	if _, _, err := p.ParentKeyWithError(&Document{}); err == nil {
		t.Fatal("expected unresolved StructParents error")
	}
	p = Page{dict: Dict{Name("StructParents"): Number(-1)}}
	if _, _, err := p.ParentKeyWithError(&Document{}); err == nil {
		t.Fatal("expected invalid StructParents error")
	}
}

func TestPageScreenMatrixRejectsOverflowingScale(t *testing.T) {
	d := &Document{objects: map[Ref]Object{}}
	p := Page{dict: Dict{
		Name("MediaBox"): Array{Number(0), Number(0), Number(math.MaxFloat64), Number(math.MaxFloat64)},
		Name("UserUnit"): Number(2),
	}}
	m := p.MatrixIn(d, CoordinateSpaceScreen)
	for _, value := range m {
		if math.IsInf(value, 0) || math.IsNaN(value) {
			t.Fatalf("screen matrix contains non-finite value: %v", m)
		}
	}
}

func TestPageSizeRejectsOverflowingUserUnitProduct(t *testing.T) {
	d := &Document{objects: map[Ref]Object{}}
	p := Page{dict: Dict{
		Name("MediaBox"): Array{Number(0), Number(0), Number(math.MaxFloat64), Number(1)},
		Name("UserUnit"): Number(2),
	}}
	if width, height := p.Size(d); width != 0 || height != 2 {
		t.Fatalf("overflowing page size = %v,%v, want 0,2", width, height)
	}
}

func TestPageSpaceSizeAndMatrixUseMediaBoxWhenCropBoxExists(t *testing.T) {
	d := &Document{objects: map[Ref]Object{}}
	p := Page{dict: Dict{
		Name("MediaBox"): Array{Number(0), Number(0), Number(600.48), Number(845.64)},
		Name("CropBox"):  Array{Number(2.52), Number(1.8), Number(597.96), Number(844.2)},
	}}
	w, h := p.Size(d)
	if w != 600.48 || h != 845.64 {
		t.Fatalf("page size = %v,%v", w, h)
	}
	if got := p.Matrix(d); got != (geometry.Matrix{1, 0, 0, 1, 0, 0}) {
		t.Fatalf("page matrix = %v", got)
	}
}

func TestPageMatrixInSupportsCoordinateSpaces(t *testing.T) {

	d := &Document{}
	p := Page{dict: Dict{
		Name("MediaBox"): Array{Number(10), Number(20), Number(110), Number(220)},
	}}

	wantPage := geometry.Matrix{1, 0, 0, 1, -10, -20}
	if got := p.MatrixIn(d, CoordinateSpacePage); got != wantPage {
		t.Fatalf("page matrix = %v, want %v", got, wantPage)
	}
	wantScreen := geometry.Matrix{1, 0, 0, -1, -10, 220}
	if got := p.MatrixIn(d, CoordinateSpaceScreen); got != wantScreen {
		t.Fatalf("screen matrix = %v, want %v", got, wantScreen)
	}
	if got := p.MatrixIn(d, CoordinateSpaceDefault); got != (geometry.Matrix{1, 0, 0, 1, 0, 0}) {
		t.Fatalf("default matrix = %v, want identity", got)
	}
	if got := p.MatrixIn(d, CoordinateSpaceUser); got != (geometry.Matrix{1, 0, 0, 1, 0, 0}) {
		t.Fatalf("user matrix = %v, want identity", got)
	}
}

func TestPageMatrixUsesPageCoordinateSpaceWhenConfigured(t *testing.T) {
	d := &Document{}
	p := Page{
		space: CoordinateSpaceScreen,
		dict:  Dict{Name("MediaBox"): Array{Number(10), Number(20), Number(110), Number(220)}},
	}
	want := geometry.Matrix{1, 0, 0, -1, -10, 220}
	if got := p.Matrix(d); got != want {
		t.Fatalf("page matrix = %v, want selected screen matrix %v", got, want)
	}
}

func TestPageMatrixInScreenAppliesRotationAndUserUnit(t *testing.T) {
	d := &Document{}
	p := Page{dict: Dict{
		Name("MediaBox"): Array{Number(10), Number(20), Number(110), Number(220)},
		Name("Rotate"):   Number(90),
		Name("UserUnit"): Number(2),
	}}
	if got, want := p.MatrixIn(d, CoordinateSpaceScreen), (geometry.Matrix{0, 2, 2, 0, -40, -20}); got != want {
		t.Fatalf("screen matrix = %v, want %v", got, want)
	}
}

func TestPageSetInitialCTMMatchesPlaya(t *testing.T) {
	d := &Document{}
	p := Page{dict: Dict{
		Name("MediaBox"): Array{Number(10), Number(20), Number(110), Number(220)},
		Name("Contents"): newStream(nil, []byte("0 0 m 10 0 l S")),
	}}

	if got := p.SetInitialCTM(d, CoordinateSpacePage, 90); got != (geometry.Matrix{0, -1, 1, 0, -20, 110}) {
		t.Fatalf("page initial CTM = %v", got)
	}
	if p.Space() != CoordinateSpacePage {
		t.Fatalf("page space = %q, want page", p.Space())
	}
	if got := p.Rotation(d); got != 90 {
		t.Fatalf("page rotation = %d, want 90", got)
	}

	if got := p.SetInitialCTM(d, CoordinateSpaceScreen, 90); got != (geometry.Matrix{0, 1, 1, 0, -20, -10}) {
		t.Fatalf("screen initial CTM = %v", got)
	}
	if got := p.Matrix(d); got != (geometry.Matrix{0, 1, 1, 0, -20, -10}) {
		t.Fatalf("page matrix after SetInitialCTM = %v", got)
	}
	var first PathObject
	var firstErr error
	for path, err := range p.Paths(d) {
		first, firstErr = path, err
		break
	}
	if firstErr != nil || first.GState().CTM() != (geometry.Matrix{0, 1, 1, 0, -20, -10}) {
		t.Fatalf("content initial CTM = %v, err=%v", first.GState().CTM(), firstErr)
	}
	if got := p.SetInitialCTM(d, CoordinateSpaceDefault, 90); got != (geometry.Matrix{1, 0, 0, 1, 0, 0}) {
		t.Fatalf("default initial CTM = %v", got)
	}
}

func TestPageSetInitialCTMWithErrorReportsMalformedMediaBox(t *testing.T) {
	p := Page{dict: Dict{Name("MediaBox"): Array{Number(0), Number(0), Name("bad"), Number(10)}}}
	if _, err := p.SetInitialCTMWithError(&Document{}, CoordinateSpacePage, 0); err == nil {
		t.Fatal("expected malformed MediaBox error")
	}
}

func TestPageSizeUsesMediaBoxAndUserUnitWithoutRotation(t *testing.T) {
	d := &Document{objects: map[Ref]Object{}}
	p := Page{dict: Dict{Name("MediaBox"): Array{Number(0), Number(0), Number(100), Number(200)}, Name("Rotate"): Number(90), Name("UserUnit"): Number(2)}}
	w, h := p.Size(d)
	if w != 200 || h != 400 {
		t.Fatalf("page size = %v,%v", w, h)
	}
}

func TestPageBoxesWithErrorDistinguishMissingAndMalformedValues(t *testing.T) {
	d := &Document{}
	p := Page{}
	if got, err := p.MediaBoxWithError(d); err != nil || got != [4]float64{0, 0, 612, 792} {
		t.Fatalf("missing MediaBox = %v, err=%v", got, err)
	}
	p = Page{dict: Dict{Name("MediaBox"): Ref{Object: 9}}}
	if _, err := p.MediaBoxWithError(d); err == nil {
		t.Fatal("expected unresolved MediaBox error")
	}
	p = Page{dict: Dict{Name("CropBox"): Array{Number(0), Number(0), Number(10)}}}
	if _, err := p.CropBoxWithError(d); err == nil {
		t.Fatal("expected malformed CropBox error")
	}
	if _, err := p.BBoxWithError(d); err == nil {
		t.Fatal("expected malformed BBox error")
	}
}

func TestPageSizeWithErrorReportsMalformedGeometry(t *testing.T) {
	d := &Document{}
	p := Page{dict: Dict{Name("MediaBox"): Ref{Object: 9}}}
	if _, _, err := p.SizeWithError(d); err == nil {
		t.Fatal("expected unresolved MediaBox error from SizeWithError")
	}
	p = Page{dict: Dict{Name("UserUnit"): Number(0)}}
	if _, _, err := p.SizeWithError(d); err == nil {
		t.Fatal("expected invalid UserUnit error from SizeWithError")
	}
}

func TestPageMatrixWithErrorReportsMalformedGeometry(t *testing.T) {
	d := &Document{}
	p := Page{dict: Dict{Name("Rotate"): Ref{Object: 9}}}
	if _, err := p.MatrixInWithError(d, CoordinateSpacePage); err == nil {
		t.Fatal("expected unresolved Rotate error from MatrixInWithError")
	}
	p = Page{dict: Dict{Name("MediaBox"): Array{Number(0), Number(0), Number(10)}}}
	if _, err := p.MatrixInWithError(d, CoordinateSpaceScreen); err == nil {
		t.Fatal("expected malformed MediaBox error from MatrixInWithError")
	}
}

func TestPageCountUsesPageTreeNotAllPageLikeObjects(t *testing.T) {
	d := &Document{
		trailer: Dict{Name("Root"): Ref{Object: 1, Generation: 0}},
		objects: map[Ref]Object{
			{Object: 1, Generation: 0}: Dict{Name("Pages"): Ref{Object: 2, Generation: 0}},
			{Object: 2, Generation: 0}: Dict{Name("Type"): Name("Pages"), Name("Kids"): Array{Ref{Object: 3, Generation: 0}}},
			{Object: 3, Generation: 0}: Dict{Name("Type"): Name("Page")},
			{Object: 9, Generation: 0}: Dict{Name("Type"): Name("Page")},
		},
	}
	if d.PageCount() != 1 {
		t.Fatalf("page count = %d", d.PageCount())
	}
}

func TestPageCountReportsUnresolvedPageTreeType(t *testing.T) {
	d := &Document{
		trailer: Dict{Name("Root"): Ref{Object: 1}},
		objects: map[Ref]Object{
			{Object: 1}: Dict{Name("Pages"): Ref{Object: 2}},
			{Object: 2}: Dict{Name("Type"): Ref{Object: 99}, Name("Kids"): Array{}},
		},
	}
	if _, err := d.countPages(); err == nil || err.Error() != "playa: page node Type could not be resolved" {
		t.Fatalf("page count error = %v", err)
	}
}

func TestPagesReverseReportsUnresolvedPageTreeType(t *testing.T) {
	d := &Document{
		trailer: Dict{Name("Root"): Ref{Object: 1}},
		objects: map[Ref]Object{
			{Object: 1}: Dict{Name("Pages"): Ref{Object: 2}},
			{Object: 2}: Dict{Name("Type"): Ref{Object: 99}, Name("Kids"): Array{}},
		},
	}
	for _, err := range d.pagesReverseSeq() {
		if err == nil || err.Error() != "playa: page node Type could not be resolved" {
			t.Fatalf("reverse page error = %v", err)
		}
		return
	}
	t.Fatal("reverse page traversal produced no error")
}

func TestPageTreeFollowsMultiLevelIndirectNodes(t *testing.T) {
	d := &Document{
		trailer: Dict{Name("Root"): Ref{Object: 1}},
		objects: map[Ref]Object{
			{Object: 1}: Dict{Name("Pages"): Ref{Object: 2}},
			{Object: 2}: Ref{Object: 3},
			{Object: 3}: Dict{Name("Type"): Ref{Object: 4}, Name("Kids"): Ref{Object: 5}},
			{Object: 4}: Name("Pages"),
			{Object: 5}: Ref{Object: 6},
			{Object: 6}: Array{Ref{Object: 7}},
			{Object: 7}: Ref{Object: 8},
			{Object: 8}: Dict{Name("Type"): Ref{Object: 9}, Name("Parent"): Ref{Object: 3}},
			{Object: 9}: Name("Page"),
		},
	}
	if got := d.PageCount(); got != 1 {
		t.Fatalf("indirect page count = %d, want 1", got)
	}
	page, err := d.PageAt(0)
	if err != nil || page.ref != (Ref{Object: 7}) {
		t.Fatalf("indirect page = %#v, err = %v", page, err)
	}
}

func TestPageTreeDoesNotInheritNonInheritableAttributes(t *testing.T) {
	d := &Document{
		trailer: Dict{Name("Root"): Ref{Object: 1}},
		objects: map[Ref]Object{
			{Object: 1}: Dict{Name("Pages"): Ref{Object: 2}},
			{Object: 2}: Dict{
				Name("Type"):     Name("Pages"),
				Name("MediaBox"): Array{Number(0), Number(0), Number(100), Number(100)},
				Name("Contents"): newStream(nil, []byte("1 0 m 2 2 l S")),
				Name("Annots"):   Array{Dict{Name("Subtype"): Name("Link"), Name("Rect"): Array{Number(0), Number(0), Number(1), Number(1)}}},
				Name("Kids"):     Array{Ref{Object: 3}},
			},
			{Object: 3}: Dict{Name("Type"): Name("Page")},
		},
	}

	page, err := d.PageAt(0)
	if err != nil {
		t.Fatal(err)
	}
	if content, err := page.Content(d); err != nil || len(content) != 0 {
		t.Fatalf("inherited page contents = %q, err=%v", content, err)
	}
	annotations := 0
	for _, err := range d.Annotations(page) {
		if err != nil {
			t.Fatal(err)
		}
		annotations++
	}
	if annotations != 0 {
		t.Fatalf("inherited page annotations = %d, want 0", annotations)
	}
}

func TestPageByLabelStopsAtMatchingPage(t *testing.T) {
	d := &Document{
		trailer: Dict{Name("Root"): Ref{Object: 1}},
		objects: map[Ref]Object{
			{Object: 1}: Dict{Name("Pages"): Ref{Object: 2}, Name("PageLabels"): Dict{Name("Nums"): Array{Number(0), Dict{Name("P"): String("front ")}, Number(1), Dict{Name("P"): String("body ")}}}},
			{Object: 2}: Dict{Name("Type"): Name("Pages"), Name("Kids"): Array{Ref{Object: 3}, Ref{Object: 4}}, Name("Count"): Number(2)},
			{Object: 3}: Dict{Name("Type"): Name("Page"), Name("Parent"): Ref{Object: 2}},
			{Object: 4}: Dict{Name("Type"): Name("Page"), Name("Parent"): Ref{Object: 2}},
		},
	}
	page, err := d.PageByLabel("body ")
	if err != nil || page.number != 2 || page.ref != (Ref{Object: 4}) {
		t.Fatalf("page = %#v, err = %v", page, err)
	}
}

func TestPageCountCachesSuccessfulPageTreeTraversal(t *testing.T) {
	d := &Document{
		trailer: Dict{Name("Root"): Ref{Object: 1}},
		objects: map[Ref]Object{
			{Object: 1}: Dict{Name("Pages"): Ref{Object: 2}},
			{Object: 2}: Dict{Name("Type"): Name("Pages"), Name("Kids"): Array{Ref{Object: 3}}, Name("Count"): Number(1)},
			{Object: 3}: Dict{Name("Type"): Name("Page"), Name("Parent"): Ref{Object: 2}},
		},
	}
	first, second := d.PageCount(), d.PageCount()
	if first != 1 || second != 1 || !d.pageCountReady || d.pageCountCache != 1 {
		t.Fatalf("page count cache = %d, ready=%v", d.pageCountCache, d.pageCountReady)
	}
	if len(d.pageCache) != 0 || d.pagesReady {
		t.Fatalf("page count materialized page views: pages=%d ready=%v", len(d.pageCache), d.pagesReady)
	}
}

func TestLookupAndMissingPageCachesAreBounded(t *testing.T) {
	d := &Document{lookupCache: map[int]Ref{}, lookupFound: map[int]bool{}, pageMissing: map[Ref]bool{}, lookupCacheBytes: lookupCacheLimit, pageMissingBytes: pageMissingCacheLimit}
	d.cacheLookup(1, Ref{Object: 1}, true)
	d.cacheMissingPage(Ref{Object: 1})
	if len(d.lookupFound) != 0 || len(d.lookupCache) != 0 || len(d.pageMissing) != 0 {
		t.Fatalf("oversized query caches accepted entries: lookup=%d/%d missing=%d", len(d.lookupFound), len(d.lookupCache), len(d.pageMissing))
	}
}

func TestPagesCachesOnlyAfterCompleteTraversal(t *testing.T) {
	d := &Document{
		trailer: Dict{Name("Root"): Ref{Object: 1}},
		objects: map[Ref]Object{
			{Object: 1}: Dict{Name("Pages"): Ref{Object: 2}},
			{Object: 2}: Dict{Name("Type"): Name("Pages"), Name("Kids"): Array{Ref{Object: 3}, Ref{Object: 4}}},
			{Object: 3}: Dict{Name("Type"): Name("Page")},
			{Object: 4}: Dict{Name("Type"): Name("Page")},
		},
	}
	for page, err := range d.Pages() {
		if err != nil {
			t.Fatal(err)
		}
		if page.number == 1 {
			break
		}
	}
	if d.pagesReady {
		t.Fatal("partial page traversal was cached")
	}
	pages, err := d.CollectPages()
	if err != nil || len(pages) != 2 {
		t.Fatalf("pages = %#v, err=%v", pages, err)
	}
	if !d.pagesReady || len(d.pagesCache) != 2 {
		t.Fatalf("page cache = ready=%v pages=%d", d.pagesReady, len(d.pagesCache))
	}
}

func TestPagesDoNotRetainOversizedPageValues(t *testing.T) {
	d := &Document{
		trailer: Dict{Name("Root"): Ref{Object: 1}},
		objects: map[Ref]Object{
			{Object: 1}: Dict{Name("Pages"): Ref{Object: 2}},
			{Object: 2}: Dict{Name("Type"): Name("Pages"), Name("Kids"): Array{Ref{Object: 3}}},
			{Object: 3}: Dict{Name("Type"): Name("Page"), Name("Large"): String(strings.Repeat("x", pagesCacheLimit+1))},
		},
	}
	pages, err := d.CollectPages()
	if err != nil || len(pages) != 1 || len(pages[0].dict[Name("Large")].(String)) != pagesCacheLimit+1 {
		t.Fatalf("pages = %d, err = %v", len(pages), err)
	}
	if len(d.pageCache) != 0 || d.pageCacheBytes != 0 || d.pagesCache != nil || d.pagesCacheBytes != 0 || d.pagesReady {
		t.Fatalf("oversized page entered cache: page=%d/%d list=%d/%d ready=%v", len(d.pageCache), d.pageCacheBytes, len(d.pagesCache), d.pagesCacheBytes, d.pagesReady)
	}
}

func TestPageByRefCachesMissingReferences(t *testing.T) {
	d := &Document{objects: map[Ref]Object{
		{Object: 1, Generation: 0}: Dict{Name("Type"): Name("Page")},
	}}
	missing := Ref{Object: 99}
	if _, err := d.PageByRef(missing); err != ErrPageNotFound {
		t.Fatalf("first missing page error = %v", err)
	}
	if _, err := d.PageByRef(missing); err != ErrPageNotFound {
		t.Fatalf("cached missing page error = %v", err)
	}
	if !d.pageMissing[missing] {
		t.Fatal("missing page reference was not cached")
	}
}

func TestPageLookupDoesNotExposeCachedDictionary(t *testing.T) {
	ref := Ref{Object: 3}
	d := &Document{
		pageCache: map[Ref]Page{ref: {ref: ref, dict: Dict{Name("Type"): Name("Page"), Name("Label"): String("original")}}},
	}
	first, err := d.PageByRef(ref)
	if err != nil {
		t.Fatal(err)
	}
	snapshot := first.Finalize()
	snapshot.dict[Name("Label")] = String("changed")
	second, err := d.PageByRef(ref)
	if err != nil {
		t.Fatal(err)
	}
	value, _ := second.dict[Name("Label")].(String)
	if string(value) != "original" {
		t.Fatalf("page cache was exposed: %#v", second.dict)
	}
}

func TestPageDictionaryAccessorsCopyValues(t *testing.T) {
	p := Page{dict: Dict{Name("Nested"): Dict{Name("Value"): String("original")}}}
	copy := p.DictCopy()
	copy[Name("Nested")].(Dict)[Name("Value")] = String("changed")
	if got := p.dict[Name("Nested")].(Dict)[Name("Value")].(String); string(got) != "original" {
		t.Fatalf("page dictionary copy exposed nested value: %q", got)
	}
	value, ok := p.Get(Name("Nested"))
	if !ok {
		t.Fatal("page dictionary entry missing")
	}
	value.(Dict)[Name("Value")] = String("changed")
	if got := p.dict[Name("Nested")].(Dict)[Name("Value")].(String); string(got) != "original" {
		t.Fatalf("page Get exposed nested value: %q", got)
	}
}

func TestPageStreamsDoNotShareSourceData(t *testing.T) {
	d := &Document{}
	page := Page{dict: Dict{Name("Contents"): newStream(Dict{Name("Marker"): String("original")}, []byte("q"))}}
	first, err := firstPageStream(page.Streams(d))
	if err != nil {
		t.Fatal(err)
	}
	snapshot := first.Finalize()
	snapshot.DataBorrowed()[0] = 'x'
	snapshot.DictBorrowed()[Name("Marker")] = String("changed")
	second, err := firstPageStream(page.Streams(d))
	if err != nil {
		t.Fatal(err)
	}
	if string(second.DataBorrowed()) != "q" || string(second.DictBorrowed()[Name("Marker")].(String)) != "original" {
		t.Fatalf("page stream exposed source data: %#v", second)
	}
}

func firstPageStream(sequence iter.Seq2[Stream, error]) (Stream, error) {
	for stream, err := range sequence {
		return stream, err
	}
	return Stream{}, nil
}

func TestPageExposesTokensAndUnexpandedContentOps(t *testing.T) {
	d := &Document{}
	p := Page{dict: Dict{Name("Contents"): newStream(nil, []byte("1 2 m (A) Tj (B) Tj"))}}
	tokens, err := p.CollectTokens(d)
	if err != nil || len(tokens) != 7 || tokens[0].Number() != 1 || tokens[2].Text() != "m" {
		t.Fatalf("page tokens = %#v, err=%v", tokens, err)
	}
	ops, err := p.ContentOps(d)
	if err != nil || len(ops) != 3 || ops[0].operatorValue() != "m" || string(ops[1].operandsValue()[0].(String)) != "A" || string(ops[2].operandsValue()[0].(String)) != "B" {
		t.Fatalf("page ops = %#v, err=%v", ops, err)
	}
}

func TestPagesRejectsPageTreeCycle(t *testing.T) {
	d := &Document{
		trailer: Dict{Name("Root"): Ref{Object: 1, Generation: 0}},
		objects: map[Ref]Object{
			{Object: 1, Generation: 0}: Dict{Name("Pages"): Ref{Object: 2, Generation: 0}},
			{Object: 2, Generation: 0}: Dict{Name("Type"): Name("Pages"), Name("Kids"): Array{Ref{Object: 2, Generation: 0}}},
		},
	}
	var gotErr error
	for _, err := range d.Pages() {
		gotErr = err
		break
	}
	if gotErr == nil {
		t.Fatal("expected page tree cycle error")
	}
}

func TestPagesSequenceIsOrderedRepeatableAndLazy(t *testing.T) {
	d := &Document{
		trailer: Dict{Name("Root"): Ref{Object: 1, Generation: 0}},
		objects: map[Ref]Object{
			{Object: 1, Generation: 0}: Dict{Name("Pages"): Ref{Object: 2, Generation: 0}},
			{Object: 2, Generation: 0}: Dict{Name("Type"): Name("Pages"), Name("Kids"): Array{Ref{Object: 3, Generation: 0}, Ref{Object: 9, Generation: 0}}},
			{Object: 3, Generation: 0}: Dict{Name("Type"): Name("Page"), Name("N"): Number(1)},
			{Object: 9, Generation: 0}: Name("not a page node"),
		},
	}

	first := []int{}
	for page, err := range d.Pages() {
		if err != nil {
			t.Fatalf("early page iteration observed later error: %v", err)
		}
		if page.number != 1 || page.index != 0 {
			t.Fatalf("first page indexes = number %d, index %d; want 1, 0", page.number, page.index)
		}
		first = append(first, page.number)
		break
	}
	if len(first) != 1 || first[0] != 1 {
		t.Fatalf("first traversal = %v, want [1]", first)
	}

	second := []int{}
	secondIndexes := []int{}
	var gotErr error
	for page, err := range d.Pages() {
		if err != nil {
			gotErr = err
			break
		}
		second = append(second, page.number)
		secondIndexes = append(secondIndexes, page.index)
	}
	if gotErr == nil {
		t.Fatal("expected deferred later-page error")
	}
	if len(second) != 1 || second[0] != 1 {
		t.Fatalf("second traversal before error = %v, want [1]", second)
	}
	if len(secondIndexes) != 1 || secondIndexes[0] != 0 {
		t.Fatalf("second traversal indexes before error = %v, want [0]", secondIndexes)
	}
}

func TestScannedPagesExposeZeroBasedIndexAlongsideDisplayNumber(t *testing.T) {
	d := &Document{
		objects: map[Ref]Object{
			{Object: 3, Generation: 0}: Dict{Name("Type"): Name("Page")},
			{Object: 9, Generation: 0}: Dict{Name("Type"): Name("Page")},
		},
	}
	var pages []Page
	for page, err := range d.scanPagesSeq(false) {
		if err != nil {
			t.Fatal(err)
		}
		pages = append(pages, page)
	}
	if len(pages) != 2 || pages[0].number != 1 || pages[0].index != 0 || pages[1].number != 2 || pages[1].index != 1 {
		t.Fatalf("scanned page indexes = %#v", pages)
	}
}

func TestResolveRefReturnsMissingObjectError(t *testing.T) {
	d := &Document{objects: map[Ref]Object{{Object: 1, Generation: 0}: Number(3)}}
	if value, err := d.ResolveRef(Ref{Object: 1, Generation: 0}); err != nil || value != Number(3) {
		t.Fatalf("resolved=%v err=%v", value, err)
	}
	if _, err := d.ResolveRef(Ref{Object: 2, Generation: 0}); err == nil {
		t.Fatal("expected missing object error")
	}
}

func TestScanPagesUsesStableObjectOrder(t *testing.T) {
	d := &Document{objects: map[Ref]Object{
		{Object: 20, Generation: 0}: Dict{Name("Type"): Name("Page"), Name("N"): Number(20)},
		{Object: 10, Generation: 0}: Dict{Name("Type"): Name("Page"), Name("N"): Number(10)},
	}}
	pages := d.scanPages()
	if len(pages) != 2 || pages[0].dict[Name("N")] != Number(10) || pages[1].dict[Name("N")] != Number(20) {
		t.Fatalf("scanned pages = %#v", pages)
	}
}

func TestScanPagesReportsUnresolvedPageType(t *testing.T) {
	d := &Document{objects: map[Ref]Object{
		{Object: 10, Generation: 0}: Dict{Name("Type"): Ref{Object: 99}},
	}}
	for _, err := range d.scanPagesSeq(false) {
		if err == nil || err.Error() != "playa: page Type could not be resolved" {
			t.Fatalf("unresolved page Type error = %v", err)
		}
		return
	}
	t.Fatal("unresolved page Type produced no error")
}

func TestScanPagesReportsInvalidPageType(t *testing.T) {
	d := &Document{objects: map[Ref]Object{
		{Object: 10, Generation: 0}: Dict{Name("Type"): Number(7)},
	}}
	for _, err := range d.scanPagesSeq(false) {
		if err == nil || err.Error() != "playa: page Type is invalid" {
			t.Fatalf("invalid page Type error = %v", err)
		}
		return
	}
	t.Fatal("invalid page Type produced no error")
}

func TestReverseScanPagesReportsUnresolvedPageType(t *testing.T) {
	d := &Document{objects: map[Ref]Object{
		{Object: 10, Generation: 0}: Dict{Name("Type"): Ref{Object: 99}},
	}}
	for _, err := range d.pagesReverseScanSeq() {
		if err == nil || err.Error() != "playa: page Type could not be resolved" {
			t.Fatalf("unresolved reverse page Type error = %v", err)
		}
		return
	}
	t.Fatal("unresolved reverse page Type produced no error")
}

func TestPageLookupByIndexLabelAndReference(t *testing.T) {
	d := &Document{
		trailer: Dict{Name("Root"): Ref{Object: 1}},
		objects: map[Ref]Object{
			{Object: 1}: Dict{Name("Pages"): Ref{Object: 2}},
			{Object: 2}: Dict{Name("Type"): Name("Pages"), Name("Kids"): Array{Ref{Object: 3}, Ref{Object: 4}}},
			{Object: 3}: Dict{Name("Type"): Name("Page"), Name("Resources"): Dict{}},
			{Object: 4}: Dict{Name("Type"): Name("Page"), Name("Resources"): Dict{}},
		},
	}
	page, err := d.PageAt(1)
	if err != nil || page.ref != (Ref{Object: 4}) {
		t.Fatalf("PageAt = %#v, err=%v", page, err)
	}
	page, err = d.PageByRef(Ref{Object: 3})
	if err != nil || page.number != 1 {
		t.Fatalf("PageByRef = %#v, err=%v", page, err)
	}
	if len(d.pageCache) != 2 {
		t.Fatalf("page cache size = %d, want 2", len(d.pageCache))
	}
	page, err = d.PageByLabel("2")
	if err != nil || page.number != 2 {
		t.Fatalf("PageByLabel = %#v, err=%v", page, err)
	}
	if _, err := d.PageAt(2); err != ErrPageNotFound {
		t.Fatalf("missing PageAt error = %v", err)
	}
}

func TestPagesReportsUnresolvedPageTreeKids(t *testing.T) {
	d := &Document{
		trailer: Dict{Name("Root"): Ref{Object: 1}},
		objects: map[Ref]Object{
			{Object: 1}: Dict{Name("Pages"): Ref{Object: 2}},
			{Object: 2}: Dict{Name("Type"): Name("Pages"), Name("Kids"): Array{Ref{Object: 99}}},
		},
	}
	for _, err := range d.Pages() {
		if err == nil || err.Error() != "playa: page node is not dictionary" {
			t.Fatalf("unresolved page tree Kids error = %v", err)
		}
		return
	}
	t.Fatal("unresolved page tree Kids produced no error")
}

func TestPagesCachesTerminalPageTreeErrors(t *testing.T) {
	d := &Document{
		trailer: Dict{Name("Root"): Ref{Object: 1}},
		objects: map[Ref]Object{
			{Object: 1}: Dict{Name("Pages"): Ref{Object: 2}},
			{Object: 2}: Dict{Name("Type"): Name("Pages"), Name("Kids"): Array{Ref{Object: 99}}},
		},
	}
	_, firstErr := d.CollectPages()
	if firstErr == nil {
		t.Fatal("malformed page tree produced no error")
	}
	d.objects[Ref{Object: 99}] = Dict{Name("Type"): Name("Page")}
	_, secondErr := d.CollectPages()
	if secondErr == nil || secondErr.Error() != firstErr.Error() {
		t.Fatalf("cached page-tree error = %v, want %v", secondErr, firstErr)
	}
}

func TestPagesReportsUnresolvedPageTreeType(t *testing.T) {
	d := &Document{
		trailer: Dict{Name("Root"): Ref{Object: 1}},
		objects: map[Ref]Object{
			{Object: 1}: Dict{Name("Pages"): Ref{Object: 2}},
			{Object: 2}: Dict{Name("Type"): Ref{Object: 99}, Name("Kids"): Array{}},
		},
	}
	for _, err := range d.Pages() {
		if err == nil || err.Error() != "playa: page node Type could not be resolved" {
			t.Fatalf("unresolved page tree Type error = %v", err)
		}
		return
	}
	t.Fatal("unresolved page tree Type produced no error")
}

func TestResolveRefAddsObjectContextToParseError(t *testing.T) {
	d := &Document{
		data:    []byte("1 0 obj << /Value (unterminated endobj"),
		xrefs:   map[Ref]xrefEntry{{Object: 1, Generation: 0}: {offset: 0}},
		objects: map[Ref]Object{},
	}
	_, err := d.ResolveRef(Ref{Object: 1, Generation: 0})
	var parseErr *ParseError
	if !errors.As(err, &parseErr) {
		t.Fatalf("error type = %T, want ParseError", err)
	}
	if !parseErr.HasObject() || parseErr.ObjectRef() != (Ref{Object: 1, Generation: 0}) {
		t.Fatalf("parse error object = %#v", parseErr)
	}
}

func TestResolveRefCachesTerminalErrors(t *testing.T) {
	ref := Ref{Object: 1}
	d := &Document{data: []byte(""), xrefs: map[Ref]xrefEntry{ref: {offset: 1}}}
	_, firstErr := d.ResolveRef(ref)
	if firstErr == nil {
		t.Fatal("invalid object produced no error")
	}
	d.objects = map[Ref]Object{ref: Number(42)}
	_, secondErr := d.ResolveRef(ref)
	if secondErr == nil || secondErr.Error() != firstErr.Error() {
		t.Fatalf("cached object error = %v, want %v", secondErr, firstErr)
	}
}

func TestResolveRefErrorCacheHasBoundedBytes(t *testing.T) {
	const attempts = 4096
	d := &Document{data: []byte{}, xrefs: map[Ref]xrefEntry{}}
	for i := 0; i < attempts; i++ {
		ref := Ref{Object: i + 1}
		d.xrefs[ref] = xrefEntry{offset: -1}
		if _, err := d.ResolveRef(ref); err == nil {
			t.Fatalf("object %d unexpectedly resolved", ref.Object)
		}
	}
	if d.objectErrorBytes == 0 || d.objectErrorBytes > objectErrorCacheByteLimit {
		t.Fatalf("object error cache bytes = %d, want a positive bounded value", d.objectErrorBytes)
	}
}
