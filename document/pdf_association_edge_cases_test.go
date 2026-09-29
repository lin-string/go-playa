package document_test

import (
	"bytes"
	"reflect"
	"testing"

	"github.com/lin-string/go-playa/document"
)

func TestPDFAssociationDefaultRGBResources(t *testing.T) {
	for _, id := range []string{"default-rgb-explicit", "default-rgb-inherited"} {
		t.Run(id, func(t *testing.T) {
			doc := openPDFAssociationFixture(t, id)
			defer closePDFAssociationFixture(t, doc)
			pages, err := doc.CollectPages()
			if err != nil {
				t.Fatal(err)
			}
			resources, err := pages[0].ResourcesWithError(doc)
			if err != nil {
				t.Fatal(err)
			}
			spaces, ok := resources[document.Name("ColorSpace")].(document.Dict)
			if !ok {
				t.Fatalf("ColorSpace resources = %#v", resources[document.Name("ColorSpace")])
			}
			resolved, err := doc.ResolveObject(spaces[document.Name("DefaultRGB")])
			if err != nil {
				t.Fatal(err)
			}
			spec, ok := resolved.(document.Array)
			if !ok || len(spec) != 2 || spec[0] != document.Name("CalRGB") {
				t.Fatalf("DefaultRGB = %#v, want CalRGB array", spaces[document.Name("DefaultRGB")])
			}
		})
	}
}

func TestPDFAssociationIndexedColorIndicesAreNormalized(t *testing.T) {
	doc := openPDFAssociationFixture(t, "indexed-color-out-of-range")
	defer closePDFAssociationFixture(t, doc)
	pages, err := doc.CollectPages()
	if err != nil {
		t.Fatal(err)
	}
	var indices []float64
	for path, err := range pages[0].Paths(doc) {
		if err != nil {
			t.Fatal(err)
		}
		color := path.GState().FillColor()
		if color.Space() != "Cs1" {
			continue
		}
		values := color.ValuesCopy()
		if len(values) != 1 {
			t.Fatalf("Indexed fill color = %#v, want one component", values)
		}
		indices = append(indices, values[0])
	}
	want := []float64{0, 0, 1, 2, 3, 4, 5, 6, 7, 7, 7}
	if !reflect.DeepEqual(indices, want) {
		t.Fatalf("Indexed fill indices = %v, want %v", indices, want)
	}
}

func TestPDFAssociationNegativeAndZeroFontSizes(t *testing.T) {
	doc := openPDFAssociationFixture(t, "negative-font-size")
	defer closePDFAssociationFixture(t, doc)
	pages, err := doc.CollectPages()
	if err != nil {
		t.Fatal(err)
	}
	var fontSizes []float64
	var sizes []float64
	for text, err := range pages[0].Texts(doc) {
		if err != nil {
			t.Fatal(err)
		}
		fontSizes = append(fontSizes, text.FontSize())
		sizes = append(sizes, text.Size())
	}
	wantFontSizes := []float64{20, -20, 0, 20, -20, 0, 20, -20, 0, 20, -20, 0}
	if !reflect.DeepEqual(fontSizes, wantFontSizes) {
		t.Fatalf("font sizes = %v, want %v", fontSizes, wantFontSizes)
	}
	wantSizes := []float64{20, 20, 0, 40, 40, 0, 20, 20, 0, 40, 40, 0}
	if !reflect.DeepEqual(sizes, wantSizes) {
		t.Fatalf("device-space sizes = %v, want %v", sizes, wantSizes)
	}
}

func TestPDFAssociationNegativeDashPhasesRemainObservable(t *testing.T) {
	doc := openPDFAssociationFixture(t, "negative-dash-phase")
	defer closePDFAssociationFixture(t, doc)
	pages, err := doc.CollectPages()
	if err != nil {
		t.Fatal(err)
	}
	type dashState struct {
		array []float64
		phase float64
	}
	var states []dashState
	for path, err := range pages[0].Paths(doc) {
		if err != nil {
			t.Fatal(err)
		}
		if dash := path.GState().DashCopy(); len(dash) != 0 {
			states = append(states, dashState{array: dash, phase: path.GState().DashPhase()})
		}
	}
	want := []dashState{
		{array: []float64{10, 10}, phase: 0},
		{array: []float64{10, 10}, phase: -1},
		{array: []float64{20, 0, 0, 10, 10}, phase: 0},
		{array: []float64{20, 0, 0, 10, 10}, phase: -1},
		{array: []float64{20, 0, 0, 10, 10}, phase: -2},
		{array: []float64{20, 0, 0, 10, 10}, phase: -3},
		{array: []float64{20, 0, 0, 10, 10}, phase: -4},
		{array: []float64{20, 0, 0, 10, 10}, phase: -5},
		{array: []float64{20, 0, 0, 10, 10}, phase: -6},
		{array: []float64{20, 0, 0, 10, 10}, phase: -7},
	}
	if !reflect.DeepEqual(states, want) {
		t.Fatalf("dash states = %v, want %v", states, want)
	}
}

func TestPDFAssociationCutHereNegativeDashPhaseRemainsObservable(t *testing.T) {
	doc := openPDFAssociationFixture(t, "negative-dash-phase-cut-here")
	defer closePDFAssociationFixture(t, doc)
	pages, err := doc.CollectPages()
	if err != nil {
		t.Fatal(err)
	}
	var got []struct {
		array []float64
		phase float64
		cap   int
		width float64
	}
	for path, err := range pages[0].Paths(doc) {
		if err != nil {
			t.Fatal(err)
		}
		if dash := path.GState().DashCopy(); len(dash) != 0 {
			got = append(got, struct {
				array []float64
				phase float64
				cap   int
				width float64
			}{dash, path.GState().DashPhase(), path.GState().LineCap(), path.GState().LineWidth()})
		}
	}
	if len(got) != 1 || !reflect.DeepEqual(got[0].array, []float64{10, 5, 60, 50}) ||
		got[0].phase != -20 || got[0].cap != 0 || got[0].width != 6 {
		t.Fatalf("dashed cut line = %#v", got)
	}
}

func TestPDFAssociationDashCornerFixturesPreserveStrokeState(t *testing.T) {
	tests := []struct {
		id       string
		dash     []float64
		width    float64
		caps     []int
		join     int
		closedBy string
	}{
		{id: "degenerate-dashing", dash: []float64{10, 10}, width: 5, caps: []int{0, 0, 1, 1, 2, 2, 2, 2}, join: 1, closedBy: "h"},
		{id: "dashing-end-before-bend", dash: []float64{20, 20}, width: 10, caps: []int{0, 1, 2, 0, 1, 2}},
		{id: "dashing-end-before-bend-closed", dash: []float64{10, 20}, width: 10, caps: []int{1}, closedBy: "h"},
	}
	for _, test := range tests {
		t.Run(test.id, func(t *testing.T) {
			doc := openPDFAssociationFixture(t, test.id)
			defer closePDFAssociationFixture(t, doc)
			pages, err := doc.CollectPages()
			if err != nil {
				t.Fatal(err)
			}
			var caps []int
			for path, err := range pages[0].Paths(doc) {
				if err != nil {
					t.Fatal(err)
				}
				state := path.GState()
				if state.LineWidth() != test.width || !reflect.DeepEqual(state.DashCopy(), test.dash) || state.DashPhase() != 0 {
					continue
				}
				if state.LineJoin() != test.join {
					t.Fatalf("dashed path line join = %d, want %d", state.LineJoin(), test.join)
				}
				segments := path.RawSegmentsCopy()
				if test.closedBy != "" {
					found := false
					for _, segment := range segments {
						if segment.Operator() == test.closedBy {
							found = true
							break
						}
					}
					if !found {
						t.Fatalf("dashed path segments do not contain %q: %#v", test.closedBy, segments)
					}
				}
				caps = append(caps, state.LineCap())
			}
			if !reflect.DeepEqual(caps, test.caps) {
				t.Fatalf("dashed path line caps = %v, want %v", caps, test.caps)
			}
		})
	}
}

func TestPDFAssociationBlendModesAndTransparencyGroupsRemainObservable(t *testing.T) {
	for _, test := range []struct {
		id   string
		mode string
	}{
		{id: "color-burn-blend-mode", mode: "ColorBurn"},
		{id: "color-dodge-blend-mode", mode: "ColorDodge"},
	} {
		t.Run(test.id, func(t *testing.T) {
			doc := openPDFAssociationFixture(t, test.id)
			defer closePDFAssociationFixture(t, doc)
			pages, err := doc.CollectPages()
			if err != nil {
				t.Fatal(err)
			}
			states := 0
			for state, err := range pages[0].ExtGStates(doc) {
				if err != nil {
					t.Fatal(err)
				}
				if state.GState().BlendMode() != test.mode {
					continue
				}
				states++
			}
			if states != 1 {
				t.Fatalf("%s external graphics states = %d, want 1", test.mode, states)
			}
			forms := 0
			for object, err := range pages[0].XObjects(doc) {
				if err != nil {
					t.Fatal(err)
				}
				forms++
				group := object.GroupCopy()
				if group[document.Name("S")] != document.Name("Transparency") || group[document.Name("CS")] != document.Name("DeviceRGB") {
					t.Fatalf("%s group = %#v", test.mode, group)
				}
				if object.GState().BlendMode() != "Normal" || object.GState().StrokeAlpha() != 1 || object.GState().FillAlpha() != 1 {
					t.Fatalf("%s transparency group initial state = %#v", test.mode, object.GState())
				}
			}
			if forms != 1 {
				t.Fatalf("%s transparency groups = %d, want 1", test.mode, forms)
			}
		})
	}
}

func TestPDFAssociationDegeneratePathsRetainLineCaps(t *testing.T) {
	doc := openPDFAssociationFixture(t, "degenerate-line-caps")
	defer closePDFAssociationFixture(t, doc)
	pages, err := doc.CollectPages()
	if err != nil {
		t.Fatal(err)
	}
	var caps []int
	checkedUserUnit := false
	for path, err := range pages[0].Paths(doc) {
		if err != nil {
			t.Fatal(err)
		}
		if !checkedUserUnit {
			rawSegments := path.RawSegmentsCopy()
			segments := path.SegmentsCopy()
			if len(rawSegments) == 0 || len(segments) == 0 || len(rawSegments[0].PointsCopy()) == 0 || len(segments[0].PointsCopy()) == 0 {
				t.Fatal("first path has no comparable segment point")
			}
			raw := rawSegments[0].PointsCopy()[0]
			device := segments[0].PointsCopy()[0]
			if raw[0] != 10 || device[0] != 100 {
				t.Fatalf("first path X coordinates = raw %v, device %v; want 10, 100 for UserUnit 10", raw[0], device[0])
			}
			checkedUserUnit = true
		}
		if path.GState().LineWidth() == 5 {
			caps = append(caps, path.GState().LineCap())
		}
	}
	want := []int{0, 0, 0, 1, 1, 1, 2, 2, 2}
	if !reflect.DeepEqual(caps, want) {
		t.Fatalf("degenerate-path line caps = %v, want %v", caps, want)
	}
}

func TestPDFAssociationLargeMiterLimitsRemainObservable(t *testing.T) {
	doc := openPDFAssociationFixture(t, "large-miter-limit")
	defer closePDFAssociationFixture(t, doc)
	pages, err := doc.CollectPages()
	if err != nil {
		t.Fatal(err)
	}
	var limits []float64
	for path, err := range pages[0].Paths(doc) {
		if err != nil {
			t.Fatal(err)
		}
		if path.GState().LineWidth() == 10 {
			limits = append(limits, path.GState().MiterLimit())
		}
	}
	want := []float64{333, 333, 333.3276, 333.3276}
	if !reflect.DeepEqual(limits, want) {
		t.Fatalf("large miter limits = %v, want %v", limits, want)
	}
}

func TestPDFAssociationLargeMiterLimitsRemainObservableOnBezierJoins(t *testing.T) {
	doc := openPDFAssociationFixture(t, "large-miter-limit-beziers")
	defer closePDFAssociationFixture(t, doc)
	pages, err := doc.CollectPages()
	if err != nil {
		t.Fatal(err)
	}
	var limits []float64
	for path, err := range pages[0].Paths(doc) {
		if err != nil {
			t.Fatal(err)
		}
		if path.GState().LineWidth() != 10 {
			continue
		}
		segments := path.RawSegmentsCopy()
		hasCurve := false
		for _, segment := range segments {
			if segment.Operator() == "c" {
				hasCurve = true
				break
			}
		}
		if hasCurve {
			limits = append(limits, path.GState().MiterLimit())
		}
	}
	want := []float64{333, 333, 333.3276, 333.3276}
	if !reflect.DeepEqual(limits, want) {
		t.Fatalf("Bezier-join miter limits = %v, want %v", limits, want)
	}
}

func TestPDFAssociationCombinedFillStrokeIsAtomicPath(t *testing.T) {
	doc := openPDFAssociationFixture(t, "atomic-fill-stroke")
	defer closePDFAssociationFixture(t, doc)
	pages, err := doc.CollectPages()
	if err != nil {
		t.Fatal(err)
	}
	fillOnly, strokeOnly := 0, 0
	var combined [][2]float64
	for path, err := range pages[0].Paths(doc) {
		if err != nil {
			t.Fatal(err)
		}
		switch {
		case path.Fill() && path.Stroke():
			combined = append(combined, [2]float64{path.GState().StrokeAlpha(), path.GState().FillAlpha()})
		case path.Fill():
			fillOnly++
		case path.Stroke():
			strokeOnly++
		}
	}
	if fillOnly != 4 || strokeOnly != 4 {
		t.Fatalf("separate fill/stroke counts = %d/%d, want 4/4", fillOnly, strokeOnly)
	}
	wantCombined := [][2]float64{{1, 1}, {0.3, 0.5}}
	if !reflect.DeepEqual(combined, wantCombined) {
		t.Fatalf("combined path alpha pairs = %v, want %v", combined, wantCombined)
	}
}

func TestPDFAssociationSelfIntersectingFillStrokeIsAtomicPath(t *testing.T) {
	tests := []struct {
		id    string
		alpha [2]float64
	}{
		{id: "atomic-fill-stroke-self-intersecting-opaque", alpha: [2]float64{1, 1}},
		{id: "atomic-fill-stroke-self-intersecting-transparency", alpha: [2]float64{0.3, 0.5}},
	}
	for _, test := range tests {
		t.Run(test.id, func(t *testing.T) {
			doc := openPDFAssociationFixture(t, test.id)
			defer closePDFAssociationFixture(t, doc)
			pages, err := doc.CollectPages()
			if err != nil {
				t.Fatal(err)
			}
			var got []struct {
				evenOdd bool
				alpha   [2]float64
			}
			for path, err := range pages[0].Paths(doc) {
				if err != nil {
					t.Fatal(err)
				}
				if path.Fill() && path.Stroke() {
					got = append(got, struct {
						evenOdd bool
						alpha   [2]float64
					}{path.EvenOdd(), [2]float64{path.GState().StrokeAlpha(), path.GState().FillAlpha()}})
				}
			}
			if len(got) != 2 || !got[0].evenOdd || got[1].evenOdd || got[0].alpha != test.alpha || got[1].alpha != test.alpha {
				t.Fatalf("combined paths = %#v, want odd-even then nonzero with alpha %v", got, test.alpha)
			}
		})
	}
}

func TestPDFAssociationTextClippingUpdatesPathGraphicsStateAtET(t *testing.T) {
	tests := []struct {
		id         string
		clipDepths []int
	}{
		{id: "overlapping-glyph-clipping", clipDepths: []int{1}},
		{id: "text-clipping-mode-changes", clipDepths: []int{1, 1, 1}},
	}
	for _, test := range tests {
		t.Run(test.id, func(t *testing.T) {
			doc := openPDFAssociationFixture(t, test.id)
			defer closePDFAssociationFixture(t, doc)
			pages, err := doc.CollectPages()
			if err != nil {
				t.Fatal(err)
			}
			var depths []int
			for path, err := range pages[0].Paths(doc) {
				if err != nil {
					t.Fatal(err)
				}
				depths = append(depths, path.GState().ClipDepth())
			}
			if !reflect.DeepEqual(depths, test.clipDepths) {
				t.Fatalf("path clip depths = %v, want %v", depths, test.clipDepths)
			}
		})
	}
}

func TestPDFAssociationUnknownFiltersPreserveIndependentContent(t *testing.T) {
	t.Run("font stream", func(t *testing.T) {
		doc := openPDFAssociationFixture(t, "unknown-filter-font")
		defer closePDFAssociationFixture(t, doc)
		pages, err := doc.CollectPages()
		if err != nil {
			t.Fatal(err)
		}
		images := 0
		for _, err := range pages[0].Images(doc) {
			if err != nil {
				t.Fatal(err)
			}
			images++
		}
		if images != 1 {
			t.Fatalf("image count = %d, want 1", images)
		}
		for _, err := range pages[0].Texts(doc) {
			if err != nil {
				return
			}
		}
		t.Fatal("unknown embedded-font filter was silently accepted during text extraction")
	})
	t.Run("image stream", func(t *testing.T) {
		doc := openPDFAssociationFixture(t, "unknown-filter-image")
		defer closePDFAssociationFixture(t, doc)
		pages, err := doc.CollectPages()
		if err != nil {
			t.Fatal(err)
		}
		text, err := pages[0].ExtractText(doc, document.DefaultTextExtractionOptions())
		if err != nil || text != "Hello!" {
			t.Fatalf("text = %q, %v; want Hello!", text, err)
		}
		for image, err := range pages[0].Images(doc) {
			if err != nil {
				t.Fatal(err)
			}
			if _, err := image.DecodedStreamBufferWithError(); err == nil {
				t.Fatal("unknown image filter was silently accepted")
			}
			return
		}
		t.Fatal("image not found")
	})
	t.Run("form stream", func(t *testing.T) {
		doc := openPDFAssociationFixture(t, "unknown-filter-form")
		defer closePDFAssociationFixture(t, doc)
		pages, err := doc.CollectPages()
		if err != nil {
			t.Fatal(err)
		}
		operations := 0
		for _, err := range pages[0].Contents(doc) {
			if err != nil {
				t.Fatal(err)
			}
			operations++
		}
		if operations == 0 {
			t.Fatal("page content stream is empty")
		}
		for _, err := range pages[0].Texts(doc) {
			if err != nil {
				return
			}
		}
		t.Fatal("unknown Form XObject filter was silently accepted")
	})
}

func TestPDFAssociationDCTPNGPolyglotsAreRejectedByStrictDecoding(t *testing.T) {
	pngSignature := []byte("\x89PNG\r\n\x1a\n")
	for _, id := range []string{"dct-png-polyglot-wine", "dct-png-polyglot-oil"} {
		t.Run(id, func(t *testing.T) {
			doc := openPDFAssociationFixture(t, id)
			defer closePDFAssociationFixture(t, doc)
			pages, err := doc.CollectPages()
			if err != nil {
				t.Fatal(err)
			}
			if len(pages) != 1 {
				t.Fatalf("pages = %d, want 1", len(pages))
			}
			for image, err := range pages[0].Images(doc) {
				if err != nil {
					t.Fatal(err)
				}
				if image.Name() != "Ig2" {
					continue
				}
				if got := image.RawFiltersCopy(); !reflect.DeepEqual(got, []string{"DCTDecode"}) {
					t.Fatalf("Ig2 filters = %v, want DCTDecode", got)
				}
				if raw := image.Buffer(); !bytes.HasPrefix(raw, pngSignature) {
					t.Fatalf("Ig2 stream prefix = % x, want PNG signature", raw[:min(len(raw), len(pngSignature))])
				}
				if _, err := image.DecodedStreamBufferWithError(); err == nil {
					t.Fatal("PNG polyglot was accepted as a DCTDecode stream")
				}
				return
			}
			t.Fatal("Ig2 image not found")
		})
	}
}
