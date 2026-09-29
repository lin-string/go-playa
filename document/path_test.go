package document

import (
	"encoding/json"
	"math"
	"runtime"
	"strings"
	"testing"

	"github.com/lin-string/go-playa/contentdata"
	"github.com/lin-string/go-playa/geometry"
)

func TestInterpretPathsSharesStableContext(t *testing.T) {
	skipAllocationCheckUnderRace(t)
	const pathCount = 4096
	ops, err := ParseContent([]byte(strings.Repeat("0 0 1 1 re f ", pathCount)))
	if err != nil {
		t.Fatal(err)
	}
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	paths := InterpretPaths(ops)
	runtime.ReadMemStats(&after)
	if len(paths) != pathCount {
		t.Fatalf("interpreted paths = %d, want %d", len(paths), pathCount)
	}
	if allocated := after.TotalAlloc - before.TotalAlloc; allocated > 7<<20 {
		t.Fatalf("interpreting %d paths allocated %d bytes", pathCount, allocated)
	}
}

func TestPathObjectJSONIncludesPrivateMetadata(t *testing.T) {
	path := newPathObject(contentdata.PathSpec{
		Page: Ref{Object: 7}, HasPage: true, Stroke: true, Fill: true, EvenOdd: true,
		Clip: true, ClipEvenOdd: true, BBox: [4]float64{1, 2, 3, 4}, MarkedTag: "Figure",
		ActualText: "path", MCID: 3, HasMCID: true,
	})
	data, err := json.Marshal(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"HasPage":true`, `"Stroke":true`, `"Fill":true`, `"EvenOdd":true`, `"Clip":true`, `"ClipEvenOdd":true`, `"BBox":[1,2,3,4]`, `"marked_tag":"Figure"`, `"actual_text":"path"`, `"has_mcid":true`} {
		if !strings.Contains(string(data), want) {
			t.Fatalf("path JSON omitted %s: %s", want, data)
		}
	}
}

func TestPathObjectSegmentsSequenceIsRepeatableAndStopsEarly(t *testing.T) {
	path := newPathObject(contentdata.PathSpec{Segments: []geometry.PathSegment{geometry.NewPathSegment("m"), geometry.NewPathSegment("l")}})
	var first []string
	for segment, err := range path.SegmentsSeq() {
		if err != nil {
			t.Fatal(err)
		}
		first = append(first, segment.Operator())
		break
	}
	if len(first) != 1 || first[0] != "m" {
		t.Fatalf("early segments = %#v", first)
	}
	var second []string
	for segment, err := range path.SegmentsSeq() {
		if err != nil {
			t.Fatal(err)
		}
		second = append(second, segment.Operator())
	}
	if len(second) != 2 || second[0] != "m" || second[1] != "l" {
		t.Fatalf("repeated segments = %#v", second)
	}
}

func TestPathSnapshotsPreserveEmptyPointSlices(t *testing.T) {
	segment := geometry.NewPathSegment("m")
	if segment.PointsCopy() == nil || segment.Finalize().PointsCopy() == nil {
		t.Fatal("empty segment points became nil")
	}
	path := newPathObject(contentdata.PathSpec{
		RawSegments: []geometry.PathSegment{segment}, Segments: []geometry.PathSegment{segment},
	})
	snapshot := path.Finalize()
	if snapshot.RawSegmentsCopy()[0].PointsCopy() == nil || snapshot.SegmentsCopy()[0].PointsCopy() == nil {
		t.Fatalf("empty path points were not preserved: %#v", snapshot)
	}
}

func TestInterpretPathsBuildsSegmentsAndPaintFlags(t *testing.T) {
	ops, err := ParseContent([]byte("q 2 0 0 3 10 20 cm 0 0 m 10 0 l 10 10 l h B* Q"))
	if err != nil {
		t.Fatal(err)
	}
	got := InterpretPaths(ops)
	if len(got) != 1 {
		t.Fatalf("got %d paths: %#v", len(got), got)
	}
	p := got[0]
	if !p.Fill() || !p.Stroke() || !p.EvenOdd() || len(p.RawSegmentsCopy()) != 4 {
		t.Fatalf("unexpected path flags/segments: %#v", p)
	}
	if p.RawSegmentsCopy()[0].Operator() != "m" || p.RawSegmentsCopy()[3].Operator() != "h" {
		t.Fatalf("unexpected segments: %#v", p.RawSegmentsCopy())
	}
	if p.SegmentsCopy()[0].PointsCopy()[0] != [2]float64{10, 20} || p.SegmentsCopy()[2].PointsCopy()[0] != [2]float64{30, 50} {
		t.Fatalf("unexpected device segments: %#v", p.SegmentsCopy())
	}
	// User-space bounds [0,0]-[10,10] transformed by the CTM.
	if p.BBox() != [4]float64{10, 20, 30, 50} {
		t.Fatalf("bbox = %v", p.BBox())
	}
}

func TestInterpretPathsIgnoresNonFiniteGeometry(t *testing.T) {
	ops := []ContentOp{
		newContentOpBorrowed("cm", []Object{Number(math.NaN()), Number(0), Number(0), Number(1), Number(0), Number(0)}, 0),
		newContentOpBorrowed("m", []Object{Number(0), Number(0)}, 0),
		newContentOpBorrowed("l", []Object{Number(1), Number(0)}, 0),
		newContentOpBorrowed("S", nil, 0),
	}
	paths := InterpretPaths(ops)
	if len(paths) != 1 || paths[0].BBox() != [4]float64{0, 0, 1, 0} {
		t.Fatalf("non-finite geometry was applied: %#v", paths)
	}
}

func TestInterpretPathsRejectsOverflowingTransformedGeometry(t *testing.T) {
	ops := []ContentOp{
		newContentOpBorrowed("cm", []Object{Number(1e308), Number(0), Number(0), Number(1), Number(0), Number(0)}, 0),
		newContentOpBorrowed("m", []Object{Number(1e308), Number(0)}, 0),
		newContentOpBorrowed("l", []Object{Number(1e308), Number(1)}, 0),
		newContentOpBorrowed("S", nil, 0),
	}
	if paths := InterpretPaths(ops); len(paths) != 0 {
		t.Fatalf("overflowing transformed path was emitted: %#v", paths)
	}
}

func TestInterpretPathsIgnoresOverflowingCTMProduct(t *testing.T) {
	ops := []ContentOp{
		newContentOpBorrowed("cm", []Object{Number(math.MaxFloat64), Number(0), Number(0), Number(1), Number(0), Number(0)}, 0),
		newContentOpBorrowed("cm", []Object{Number(math.MaxFloat64), Number(0), Number(0), Number(1), Number(0), Number(0)}, 0),
		newContentOpBorrowed("m", []Object{Number(0), Number(0)}, 0),
		newContentOpBorrowed("l", []Object{Number(1), Number(0)}, 0),
		newContentOpBorrowed("S", nil, 0),
	}
	if paths := InterpretPaths(ops); len(paths) != 1 || paths[0].GState().CTM()[0] != math.MaxFloat64 {
		t.Fatalf("overflowing path CTM changed output: %#v", paths)
	}
}

func TestInterpretPathsCarriesGraphicsState(t *testing.T) {
	ops, _ := ParseContent([]byte("2 w 0.2 0.3 0.4 RG 0 0 m 10 0 l S"))
	got := InterpretPaths(ops)
	if len(got) != 1 || got[0].GState().LineWidth() != 2 || got[0].GState().StrokeColor().Space() != "DeviceRGB" {
		t.Fatalf("path graphics state = %#v", got)
	}
}

func TestInterpretPathsDoesNotReuseGraphicsStateAcrossRestoredBranches(t *testing.T) {
	ops, err := ParseContent([]byte(
		"q 0 g [1.44] 0 d 0 0 1 1 re f Q " +
			"1 g [0.48] 0 d 2 0 1 1 re f",
	))
	if err != nil {
		t.Fatal(err)
	}
	paths := InterpretPaths(ops)
	if len(paths) != 2 {
		t.Fatalf("paths = %d, want 2", len(paths))
	}
	if got := paths[1].GState().FillColor().ValuesCopy(); len(got) != 1 || got[0] != 1 {
		t.Fatalf("restored-branch fill color = %#v, want [1]", got)
	}
	if got := paths[1].GState().DashCopy(); len(got) != 1 || got[0] != 0.48 {
		t.Fatalf("restored-branch dash = %#v, want [0.48]", got)
	}
}

func TestInterpretPathsMarksClippingPaths(t *testing.T) {
	ops, err := ParseContent([]byte("0 0 10 10 re W S"))
	if err != nil {
		t.Fatal(err)
	}
	got := InterpretPaths(ops)
	if len(got) != 1 || !got[0].Clip() || got[0].ClipEvenOdd() || got[0].GState().ClipDepth() != 1 {
		t.Fatalf("clipping path = %#v", got)
	}
	ops, _ = ParseContent([]byte("0 0 10 10 re W S 1 1 m 9 9 l S"))
	got = InterpretPaths(ops)
	if len(got) != 2 || got[1].GState().ClipDepth() != 1 {
		t.Fatalf("clip state was not persistent = %#v", got)
	}
	ops, _ = ParseContent([]byte("0 0 10 10 re W* S"))
	got = InterpretPaths(ops)
	if len(got) != 1 || !got[0].Clip() || !got[0].ClipEvenOdd() {
		t.Fatalf("even-odd clipping path = %#v", got)
	}
	ops, _ = ParseContent([]byte("0 0 10 10 re W n 1 1 m 9 9 l S"))
	got = InterpretPaths(ops)
	if len(got) != 1 || got[0].GState().ClipDepth() != 1 || got[0].GState().ClipEvenOdd() {
		t.Fatalf("persistent clipping state = %#v", got)
	}
}

func TestInterpretPathsAppliesTextClippingAtEndText(t *testing.T) {
	ops, err := ParseContent([]byte(
		"BT 7 Tr (one) Tj ET 0 0 1 1 re f " +
			"BT 5 Tr (two) Tj 4 Tr (three) Tj ET 1 0 1 1 re f " +
			"BT 6 Tr (four) Tj ET 2 0 1 1 re f",
	))
	if err != nil {
		t.Fatal(err)
	}
	got := InterpretPaths(ops)
	if len(got) != 3 {
		t.Fatalf("paths = %d, want 3", len(got))
	}
	for index, path := range got {
		if depth := path.GState().ClipDepth(); depth != index+1 {
			t.Fatalf("path %d clip depth = %d, want %d", index, depth, index+1)
		}
	}
}

func TestInterpretPathsDoesNotApplyEmptyTextClipping(t *testing.T) {
	ops, err := ParseContent([]byte("BT 7 Tr () Tj [] TJ ET 0 0 1 1 re f"))
	if err != nil {
		t.Fatal(err)
	}
	got := InterpretPaths(ops)
	if len(got) != 1 || got[0].GState().ClipDepth() != 0 {
		t.Fatalf("empty text clipping changed path graphics state = %#v", got)
	}
}

func TestInterpretPathsRecognizesEveryTextClippingShowOperator(t *testing.T) {
	tests := []struct {
		name string
		show string
	}{
		{name: "Tj", show: "(one) Tj"},
		{name: "TJ", show: "[(one) 10 (two)] TJ"},
		{name: "quote", show: "(one) '"},
		{name: "double quote", show: "0 0 (one) \""},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ops, err := ParseContent([]byte("BT 7 Tr " + test.show + " ET 0 0 1 1 re f"))
			if err != nil {
				t.Fatal(err)
			}
			got := InterpretPaths(ops)
			if len(got) != 1 || got[0].GState().ClipDepth() != 1 {
				t.Fatalf("text clipping for %q = %#v", test.show, got)
			}
		})
	}
}

func TestInterpretPathsCarriesMarkedContentAssociation(t *testing.T) {
	ops, _ := ParseContent([]byte("/Figure << /MCID 3 >> BDC 0 0 m 10 0 l S EMC"))
	got := InterpretPaths(ops)
	if len(got) != 1 || !got[0].HasMCID() || got[0].MCID() != 3 || got[0].MarkedTag() != "Figure" {
		t.Fatalf("path marked-content association = %#v", got)
	}
}

func TestInterpretPathsSupportsBezierShorthandAndRect(t *testing.T) {
	ops, err := ParseContent([]byte("0 0 m 10 10 20 0 30 0 c 40 10 50 0 v 60 10 70 0 y 5 5 10 10 re f"))
	if err != nil {
		t.Fatal(err)
	}
	got := InterpretPaths(ops)
	if len(got) != 1 || !got[0].Fill() || len(got[0].RawSegmentsCopy()) != 9 {
		t.Fatalf("unexpected paths: %#v", got)
	}
	if got[0].RawSegmentsCopy()[1].Operator() != "c" || got[0].RawSegmentsCopy()[2].Operator() != "v" || got[0].RawSegmentsCopy()[3].Operator() != "y" || got[0].RawSegmentsCopy()[4].Operator() != "m" {
		t.Fatalf("unexpected operators: %#v", got[0].RawSegmentsCopy())
	}
}

func TestInterpretPathsUsesBezierControlPointsForBBox(t *testing.T) {
	ops, err := ParseContent([]byte("0 0 m 100 100 100 -100 0 0 c S"))
	if err != nil {
		t.Fatal(err)
	}
	got := InterpretPaths(ops)
	if len(got) != 1 || got[0].BBox() != [4]float64{0, -100, 100, 100} {
		t.Fatalf("bezier bbox = %v", got[0].BBox())
	}
}

func TestPagePathsExpandedIncludesFormMatrix(t *testing.T) {
	form := newStream(Dict{
		Name("Subtype"): Name("Form"),
		Name("Matrix"):  Array{Number(2), Number(0), Number(0), Number(2), Number(10), Number(20)},
	}, []byte("0 0 m 3 4 l S"))
	d := &Document{objects: map[Ref]Object{}}
	p := Page{dict: Dict{
		Name("Contents"):  newStream(nil, []byte("/Fm Do")),
		Name("Resources"): Dict{Name("XObject"): Dict{Name("Fm"): form}},
	}}
	got, err := d.PagePathsExpanded(p)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].BBox() != [4]float64{10, 20, 16, 28} {
		t.Fatalf("unexpected expanded path: %#v", got)
	}
}

func TestPagePathsExpandedUsesFormResourceColorSpace(t *testing.T) {
	form := newStream(Dict{
		Name("Subtype"):   Name("Form"),
		Name("Resources"): Dict{Name("ColorSpace"): Dict{Name("C1"): Name("DeviceRGB")}},
	}, []byte("/C1 cs 0.1 0.2 0.3 scn 0 0 m 1 1 l S"))
	d := &Document{}
	p := Page{dict: Dict{
		Name("Contents"):  newStream(nil, []byte("/Fm Do")),
		Name("Resources"): Dict{Name("XObject"): Dict{Name("Fm"): form}},
	}}

	paths, err := d.PagePathsExpanded(p)
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) != 1 {
		t.Fatalf("expanded paths = %#v", paths)
	}
	if paths[0].GState().FillColor().Space() != "DeviceRGB" {
		t.Fatalf("form fill color space = %q, want predefined DeviceRGB", paths[0].GState().FillColor().Space())
	}
}

func TestPagePathsSequenceDefersLaterStreamErrors(t *testing.T) {
	d := &Document{}
	p := Page{dict: Dict{Name("Contents"): Array{
		newStream(nil, []byte("0 0 m 1 1 l S")),
		newStream(nil, []byte("(")),
	}}}
	count := 0
	for path, err := range d.PagePathsSeq(p) {
		if err != nil {
			t.Fatal(err)
		}
		if len(path.RawSegmentsCopy()) == 0 {
			t.Fatalf("path = %#v", path)
		}
		count++
		break
	}
	if count != 1 {
		t.Fatalf("paths = %d", count)
	}
}

func TestPagePathsSequenceIsRepeatableAndStopsEarly(t *testing.T) {
	d := &Document{}
	p := Page{dict: Dict{Name("Contents"): newStream(nil, []byte(
		"0 0 m 1 1 l S 2 2 m 3 3 l S",
	))}}
	first := 0
	for path, err := range p.Paths(d) {
		if err != nil {
			t.Fatal(err)
		}
		if len(path.RawSegmentsCopy()) == 0 {
			t.Fatalf("first path = %#v", path)
		}
		first++
		break
	}
	second := 0
	for path, err := range p.Paths(d) {
		if err != nil {
			t.Fatal(err)
		}
		if len(path.RawSegmentsCopy()) == 0 {
			t.Fatalf("repeated path = %#v", path)
		}
		second++
	}
	if first != 1 || second != 2 {
		t.Fatalf("path sequence = first %d, second %d", first, second)
	}
}

func TestPagePathsToleratesIncompleteDictionaryAtStreamBoundary(t *testing.T) {
	d := &Document{}
	p := Page{dict: Dict{Name("Contents"): Array{
		newStream(nil, []byte("/Span << /MCID")),
		newStream(nil, []byte(" 40 >> BDC 0 0 m 1 1 l S")),
	}}}
	paths, err := d.PagePaths(p)
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) != 1 || paths[0].BBox() != [4]float64{0, 0, 1, 1} {
		t.Fatalf("paths = %#v", paths)
	}
}

func TestPagePathsToleratesIncompleteArrayAtStreamBoundary(t *testing.T) {
	d := &Document{}
	p := Page{dict: Dict{Name("Contents"): Array{
		newStream(nil, []byte("[ 0")),
		newStream(nil, []byte(" ] 0 0 m 1 1 l S")),
	}}}
	paths, err := d.PagePaths(p)
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) != 1 || paths[0].BBox() != [4]float64{0, 0, 1, 1} {
		t.Fatalf("paths = %#v", paths)
	}
}
