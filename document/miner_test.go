package document

import (
	"bytes"
	"encoding/json"
	"iter"
	"math"
	"math/rand"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"unsafe"

	"github.com/lin-string/go-playa/contentdata"
	"github.com/lin-string/go-playa/geometry"
	"github.com/lin-string/go-playa/internal/testfixture"
	"github.com/lin-string/go-playa/layout"
)

var analyzeLayoutAllocationSink LayoutResult

func TestAnalyzeLayoutAvoidsTransientGlyphCopies(t *testing.T) {
	skipAllocationCheckUnderRace(t)
	const glyphCount = 1024
	glyphs := make([]GlyphObject, glyphCount)
	for index := range glyphs {
		x := float64(index)
		glyphs[index] = newTestGlyph(contentdata.GlyphSpec{
			Text: "A", Origin: [2]float64{x, 10}, Displacement: [2]float64{1, 0},
			BBox: [4]float64{x, 8, x + 1, 18},
		}, nil, nil, geometry.Matrix{})
	}
	objects := []TextObject{{glyphs: glyphs}}
	result := testing.Benchmark(func(b *testing.B) {
		for range b.N {
			analyzeLayoutAllocationSink = AnalyzeLayout(objects, layout.Options{DisableBoxesFlow: true})
		}
	})
	limit := int64(4 * glyphCount * int(unsafe.Sizeof(GlyphObject{})))
	if allocated := result.AllocedBytesPerOp(); allocated > limit {
		t.Fatalf("AnalyzeLayout allocated %d bytes for %d glyphs; want at most %d", allocated, glyphCount, limit)
	}
}

func TestAnalyzeLayoutGlyphSetsMatchesTextObjects(t *testing.T) {
	objects := []TextObject{
		{glyphs: []GlyphObject{
			newTestGlyph(contentdata.GlyphSpec{
				Text: "A", Origin: [2]float64{0, 10}, Displacement: [2]float64{1, 0},
				BBox: [4]float64{0, 8, 1, 18},
			}, nil, nil, geometry.Matrix{}),
			newTestGlyph(contentdata.GlyphSpec{
				Text: "B", Origin: [2]float64{2, 10}, Displacement: [2]float64{1, 0},
				BBox: [4]float64{2, 8, 3, 18},
			}, nil, nil, geometry.Matrix{}),
		}},
		{glyphs: []GlyphObject{
			newTestGlyph(contentdata.GlyphSpec{
				Text: "C", Origin: [2]float64{0, 30}, Displacement: [2]float64{1, 0},
				BBox: [4]float64{0, 28, 1, 38},
			}, nil, nil, geometry.Matrix{}),
		}},
	}
	glyphSets := [][]GlyphObject{objects[0].glyphs, objects[1].glyphs}
	want, err := json.Marshal(analyzeLayout(objects, layout.Options{}, nil))
	if err != nil {
		t.Fatalf("marshal object layout: %v", err)
	}
	got, err := json.Marshal(analyzeLayoutGlyphSets(glyphSets, layout.Options{}, nil))
	if err != nil {
		t.Fatalf("marshal glyph-set layout: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("glyph-set layout = %s, want %s", got, want)
	}
}

func TestTextObjectStateLayoutValueRetainsGlyphOwnershipContext(t *testing.T) {
	glyph := newTestGlyph(contentdata.GlyphSpec{Text: "A"}, nil, nil, geometry.Matrix{})
	font := &Font{name: "LayoutFont"}
	state := textObjectState{
		font: font, glyphs: []GlyphObject{glyph},
		parentKey: 27, hasParentKey: true,
	}

	got := state.layoutValue()
	if got.font != font || len(got.glyphs) != 1 || got.glyphs[0].Text() != "A" {
		t.Fatalf("layout value lost glyph state: %#v", got)
	}
	if got.glyphs[0].parent == nil || got.glyphs[0].parent.key != 27 {
		t.Fatalf("layout glyph parent = %#v, want key 27", got.glyphs[0].parent)
	}
	if got.Text() != "" {
		t.Fatalf("layout-only TextObject materialized text data %q", got.Text())
	}
}

func TestTextBoxGroupingReleasesSupersededGroups(t *testing.T) {
	skipAllocationCheckUnderRace(t)
	const lineCount = 512
	lines := make([]TextLine, lineCount)
	for index := range lines {
		lines[index] = TextLine{data: layout.NewComponent(layout.ComponentSpec{
			Text: "line", BBox: [4]float64{0, 0, 100, 10},
		})}
	}
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	boxes := textBoxesFromLines(lines, 0.5, 0.5, false)
	runtime.ReadMemStats(&after)
	if len(boxes) != 1 || len(boxes[0].lines) != lineCount {
		t.Fatalf("grouped boxes = %d/%d, want 1/%d", len(boxes), len(boxes[0].lines), lineCount)
	}
	if allocated := after.TotalAlloc - before.TotalAlloc; allocated > 8<<20 {
		t.Fatalf("grouping %d overlapping lines allocated %d bytes", lineCount, allocated)
	}
}

func TestLayoutTextGroupsAvoidsPerPairAllocations(t *testing.T) {
	skipAllocationCheckUnderRace(t)
	const boxCount = 256
	boxes := make([]TextBox, boxCount)
	for index := range boxes {
		x := float64(index % 16 * 20)
		y := float64(index / 16 * 20)
		boxes[index] = newTestTextBox("box", [4]float64{x, y, x + 10, y + 10}, false, -1)
	}
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	groups := layoutTextGroups(boxes, 0.5)
	runtime.ReadMemStats(&after)
	if len(groups) != 1 {
		t.Fatalf("layout groups = %d, want 1", len(groups))
	}
	if allocated := after.TotalAlloc - before.TotalAlloc; allocated > 2<<20 {
		t.Fatalf("grouping %d text boxes allocated %d bytes", boxCount, allocated)
	}
}

func TestSplitPathSubpathsAvoidsQuadraticCopies(t *testing.T) {
	skipAllocationCheckUnderRace(t)
	const subpathCount = 1024
	segments := make([]geometry.PathSegment, 0, subpathCount*2)
	for index := range subpathCount {
		x := float64(index)
		segments = append(segments,
			geometry.NewPathSegment("m", [2]float64{x, 0}),
			geometry.NewPathSegment("l", [2]float64{x + 1, 1}),
		)
	}
	path := newPathObject(contentdata.PathSpec{RawSegments: segments, Segments: segments})
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	parts := splitPathSubpaths(path)
	runtime.ReadMemStats(&after)
	if len(parts) != subpathCount {
		t.Fatalf("split paths = %d, want %d", len(parts), subpathCount)
	}
	if allocated := after.TotalAlloc - before.TotalAlloc; allocated > 32<<20 {
		t.Fatalf("splitting %d subpaths allocated %d bytes", subpathCount, allocated)
	}
}

func TestLayoutItemStorageAvoidsPerSubpathPayloadAllocations(t *testing.T) {
	skipAllocationCheckUnderRace(t)
	const subpathCount = 8192
	segments := make([]geometry.PathSegment, 0, subpathCount*2)
	for index := range subpathCount {
		x := float64(index)
		segments = append(segments,
			geometry.NewPathSegment("m", [2]float64{x, 0}),
			geometry.NewPathSegment("l", [2]float64{x + 1, 1}),
		)
	}
	path := newPathObject(contentdata.PathSpec{RawSegments: segments, Segments: segments})
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	var storage layoutItemStorage
	storage.appendPathSubpaths(path)
	runtime.ReadMemStats(&after)
	if len(storage.order) != subpathCount || len(storage.paths) != subpathCount || len(storage.contents) != 0 {
		t.Fatalf("stored subpaths = %d/%d/%d, want %d/%d/0", len(storage.order), len(storage.paths), len(storage.contents), subpathCount, subpathCount)
	}
	if allocated := after.TotalAlloc - before.TotalAlloc; allocated > 4<<20 {
		t.Fatalf("storing %d subpaths allocated %d bytes", subpathCount, allocated)
	}
}

func TestAnalyzeLayoutGroupsTextLinesInContentOrder(t *testing.T) {
	objects := []TextObject{{glyphs: []GlyphObject{
		newTestGlyph(contentdata.GlyphSpec{Text: "A", Origin: [2]float64{10, 100.5}, BBox: [4]float64{10, 98.5, 15, 108.5}}, nil, nil, geometry.Matrix{}),
		newTestGlyph(contentdata.GlyphSpec{Text: "B", Origin: [2]float64{20, 100}, BBox: [4]float64{20, 98, 25, 108}}, nil, nil, geometry.Matrix{}),
		newTestGlyph(contentdata.GlyphSpec{Text: "C", Origin: [2]float64{10, 80}, BBox: [4]float64{10, 78, 15, 88}}, nil, nil, geometry.Matrix{}),
	}}}
	got := AnalyzeLayout(objects, layout.Options{LineMargin: 2})
	lines := got.LinesCopy()
	paragraphs := got.ParagraphsCopy()
	if len(lines) != 2 || len(paragraphs) != 1 || lines[0].Text() != "A B" || len(lines[0].WordsCopy()) != 2 || lines[0].WordsCopy()[0].Text() != "A" || lines[0].WordsCopy()[1].Text() != "B" || lines[1].Text() != "C" || lines[0].BBox() != [4]float64{10, 98, 25, 108.5} {
		t.Fatalf("layout = %#v", got)
	}
}

func TestPageLayoutDoesNotDuplicateAdjacentGlyphs(t *testing.T) {
	d, err := Open(testfixture.Path(t, "acceptance_adjacent_glyphs.pdf"), WithCoordinateSpace(CoordinateSpacePage))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = d.Close() }()
	p, err := d.PageAt(0)
	if err != nil {
		t.Fatal(err)
	}
	result, err := p.Layout(d, DefaultLayoutOptions())
	if err != nil {
		t.Fatal(err)
	}
	lines := result.LinesCopy()
	if len(lines) != 2 || lines[0].Text() != "AB" || lines[1].Text() != "C" {
		t.Fatalf("adjacent text-run layout = %#v, want AB/C", makeLayoutResultSignature(result))
	}
	if len(lines[0].GlyphsCopy()) != 2 || len(lines[1].GlyphsCopy()) != 1 {
		t.Fatalf("adjacent runs duplicated glyphs: line glyph counts = %d/%d, want 2/1", len(lines[0].GlyphsCopy()), len(lines[1].GlyphsCopy()))
	}
}

func TestPageLayoutStableAfterTransientCacheRelease(t *testing.T) {
	sequential, err := Open(testfixture.Path(t, "riscv-unprivileged.pdf"), WithCoordinateSpace(CoordinateSpacePage))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = sequential.Close() }()

	sequential.ReleaseTransientCaches()
	next, stop := iter.Pull2(sequential.Pages())
	defer stop()
	var sequentialPage Page
	for index := 0; index <= 24; index++ {
		page, pageErr, ok := next()
		if pageErr != nil {
			t.Fatal(pageErr)
		}
		if !ok {
			t.Fatalf("sequential page walk ended at page %d", index)
		}
		sequentialPage = page
	}
	sequentialResult, err := sequentialPage.Layout(sequential, DefaultLayoutOptions())
	if err != nil {
		t.Fatal(err)
	}
	if sequentialPage.Index() != 24 {
		t.Fatalf("sequential page lookup returned index %d, want 24", sequentialPage.Index())
	}
	sequentialSignature := makeLayoutResultSignature(sequentialResult)

	independent, err := Open(testfixture.Path(t, "riscv-unprivileged.pdf"), WithCoordinateSpace(CoordinateSpacePage))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = independent.Close() }()
	page, err := independent.PageAt(24)
	if err != nil {
		t.Fatal(err)
	}
	result, err := page.Layout(independent, DefaultLayoutOptions())
	if err != nil {
		t.Fatal(err)
	}
	independentSignature := makeLayoutResultSignature(result)
	if !reflect.DeepEqual(sequentialSignature, independentSignature) {
		for index := 0; index < len(sequentialSignature.Lines) && index < len(independentSignature.Lines); index++ {
			if !reflect.DeepEqual(sequentialSignature.Lines[index], independentSignature.Lines[index]) {
				t.Fatalf("page 24 line %d changed after sequential page lookup: sequential=%#v independent=%#v", index, sequentialSignature.Lines[index], independentSignature.Lines[index])
			}
		}
		for index := 0; index < len(sequentialSignature.TextBoxes) && index < len(independentSignature.TextBoxes); index++ {
			if !reflect.DeepEqual(sequentialSignature.TextBoxes[index], independentSignature.TextBoxes[index]) {
				t.Fatalf("page 24 textbox %d changed after sequential page lookup: sequential=%#v independent=%#v", index, sequentialSignature.TextBoxes[index], independentSignature.TextBoxes[index])
			}
		}
		for index := 0; index < len(sequentialSignature.TextGroups) && index < len(independentSignature.TextGroups); index++ {
			if !reflect.DeepEqual(sequentialSignature.TextGroups[index], independentSignature.TextGroups[index]) {
				t.Fatalf("page 24 group %d changed after sequential page lookup: sequential=%#v independent=%#v", index, sequentialSignature.TextGroups[index], independentSignature.TextGroups[index])
			}
		}
		t.Fatalf("page 24 layout changed after sequential page lookup: sequential=%#v independent=%#v", sequentialSignature, independentSignature)
	}
	if len(result.LinesCopy()) < 40 || len(result.TextBoxesCopy()) < 40 {
		t.Fatalf("public manual page 24 has unexpectedly little layout content: %#v", independentSignature)
	}
}

func TestPageLayoutDefaultSpacePreservesPlayaVerticalColumnOrder(t *testing.T) {
	d, err := Open(testfixture.Path(t, "acceptance_vertical_cid.pdf"), WithCoordinateSpace(CoordinateSpaceDefault))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = d.Close() }()
	page, err := d.PageAt(0)
	if err != nil {
		t.Fatal(err)
	}
	result, err := page.Layout(d, DefaultLayoutOptions())
	if err != nil {
		t.Fatal(err)
	}
	lines := result.LinesCopy()
	if len(lines) != 2 {
		t.Fatalf("layout lines = %d, want 2", len(lines))
	}
	var got strings.Builder
	for _, line := range lines {
		got.WriteString(line.Text())
	}
	if got.String() != "AB" {
		t.Fatalf("default-space vertical column = %q, want AB", got.String())
	}
}

func TestPageLayoutStableAfterEarlierPageLayouts(t *testing.T) {
	sequential, err := Open(testfixture.Path(t, "riscv-unprivileged.pdf"), WithCoordinateSpace(CoordinateSpacePage))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = sequential.Close() }()
	sequential.ReleaseTransientCaches()

	var sequentialResult LayoutResult
	for page, pageErr := range sequential.Pages() {
		if pageErr != nil {
			t.Fatal(pageErr)
		}
		if page.Index() > 24 {
			break
		}
		sequentialResult, err = page.Layout(sequential, DefaultLayoutOptions())
		if err != nil {
			t.Fatalf("layout page %d: %v", page.Index(), err)
		}
	}

	independent, err := Open(testfixture.Path(t, "riscv-unprivileged.pdf"), WithCoordinateSpace(CoordinateSpacePage))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = independent.Close() }()
	page, err := independent.PageAt(24)
	if err != nil {
		t.Fatal(err)
	}
	independentResult, err := page.Layout(independent, DefaultLayoutOptions())
	if err != nil {
		t.Fatal(err)
	}

	sequentialSignature := makeLayoutResultSignature(sequentialResult)
	independentSignature := makeLayoutResultSignature(independentResult)
	if !reflect.DeepEqual(sequentialSignature, independentSignature) {
		t.Fatalf("page 24 layout changed after laying out earlier pages: sequential=%#v independent=%#v", sequentialSignature.TextGroups, independentSignature.TextGroups)
	}
}

func hasNormalizedGroupBoundary(group TextGroup, prefix, included, excluded string) bool {
	normalized := strings.Join(strings.Fields(group.Text()), " ")
	if strings.HasPrefix(normalized, prefix) && strings.Contains(normalized, included) && !strings.Contains(normalized, excluded) {
		return true
	}
	for child := range group.ChildrenSeq() {
		if nested, ok := child.GroupBorrowed(); ok && hasNormalizedGroupBoundary(nested, prefix, included, excluded) {
			return true
		}
	}
	return false
}

type layoutNodeSignature struct {
	Kind     string
	Text     string
	BBox     [4]float64
	Vertical bool
	Index    int
	Children []layoutNodeSignature
}

type layoutResultSignature struct {
	Lines      []layoutNodeSignature
	TextBoxes  []layoutNodeSignature
	TextGroups []layoutNodeSignature
}

func makeLayoutResultSignature(result LayoutResult) layoutResultSignature {
	signature := layoutResultSignature{}
	for line := range result.LinesSeq() {
		signature.Lines = append(signature.Lines, layoutLineSignature(line))
	}
	for box := range result.TextBoxesSeq() {
		signature.TextBoxes = append(signature.TextBoxes, layoutBoxSignature(box))
	}
	for group := range result.TextGroupsSeq() {
		signature.TextGroups = append(signature.TextGroups, layoutGroupSignature(group))
	}
	return signature
}

func layoutLineSignature(line TextLine) layoutNodeSignature {
	return layoutNodeSignature{Kind: "line", Text: line.Text(), BBox: line.BBox(), Vertical: line.Vertical()}
}

func layoutBoxSignature(box TextBox) layoutNodeSignature {
	node := layoutNodeSignature{Kind: "textbox", Text: box.Text(), BBox: box.BBox(), Vertical: box.Vertical(), Index: box.Index()}
	for line := range box.LinesSeq() {
		node.Children = append(node.Children, layoutLineSignature(line))
	}
	return node
}

func layoutGroupSignature(group TextGroup) layoutNodeSignature {
	node := layoutNodeSignature{Kind: "textgroup", Text: group.Text(), BBox: group.BBox(), Vertical: group.Vertical()}
	for child := range group.ChildrenSeq() {
		if box, ok := child.BoxBorrowed(); ok {
			node.Children = append(node.Children, layoutBoxSignature(box))
			continue
		}
		if nested, ok := child.GroupBorrowed(); ok {
			node.Children = append(node.Children, layoutGroupSignature(nested))
		}
	}
	return node
}

func hasTOCGroupShape(group TextGroup) bool {
	children := borrowedTextGroupChildren(group)
	if len(children) == 2 {
		body, bodyOK := children[0].GroupBorrowed()
		footer, footerOK := children[1].BoxBorrowed()
		if bodyOK && footerOK && strings.TrimSpace(footer.Text()) == "[page 1]" {
			bodyChildren := borrowedTextGroupChildren(body)
			if len(bodyChildren) == 2 {
				header, headerOK := bodyChildren[0].BoxBorrowed()
				remainder, remainderOK := bodyChildren[1].GroupBorrowed()
				if headerOK && remainderOK && strings.HasPrefix(header.Text(), "Example organization") {
					remainderChildren := borrowedTextGroupChildren(remainder)
					if len(remainderChildren) == 2 {
						title, titleOK := remainderChildren[0].BoxBorrowed()
						sections, sectionsOK := remainderChildren[1].GroupBorrowed()
						return titleOK && sectionsOK && strings.TrimSpace(title.Text()) == "Contents" &&
							hasNormalizedGroupBoundary(sections, "Introduction", "11. Conclusion", "[page 1]")
					}
				}
			}
		}
	}
	return false
}

func borrowedTextGroupChildren(group TextGroup) []TextGroupChild {
	children := make([]TextGroupChild, 0, 2)
	for child := range group.ChildrenSeq() {
		children = append(children, child)
	}
	return children
}

func hasPageNumberGroupShape(group TextGroup) bool {
	children := borrowedTextGroupChildren(group)
	if len(children) == 2 {
		first, firstOK := children[0].GroupBorrowed()
		last, lastOK := children[1].BoxBorrowed()
		if firstOK && lastOK &&
			strings.Join(strings.Fields(first.Text()), " ") == "1 2 3 4 5 6 7 8" &&
			strings.TrimSpace(last.Text()) == "9" {
			firstChildren := borrowedTextGroupChildren(first)
			if len(firstChildren) == 2 {
				prefix, prefixOK := firstChildren[0].GroupBorrowed()
				eighth, eighthOK := firstChildren[1].BoxBorrowed()
				return prefixOK && eighthOK && strings.Join(strings.Fields(prefix.Text()), " ") == "1 2 3 4 5 6 7" && strings.TrimSpace(eighth.Text()) == "8"
			}
		}
	}
	return false
}

func TestAnalyzeLayoutUsesPlayaMinerCharacterBBox(t *testing.T) {
	font := &Font{
		fontMatrix: geometry.Matrix{0.001, 0, 0, 0.001, 0, 0},
		descent:    -207,
		widths:     map[byte]float64{'A': 667},
	}
	glyph := newTestGlyph(contentdata.GlyphSpec{
		Text: "A", CID: 'A', FontSize: 12, Matrix: geometry.Matrix{12, 0, 0, 12, 72, 760},
		Origin: [2]float64{72, 760}, BBox: [4]float64{72, 757.516, 80.004, 768.616},
	}, font, nil, geometry.Matrix{})

	lines := AnalyzeLayout([]TextObject{{glyphs: []GlyphObject{glyph}}}, layout.Options{}).LinesCopy()
	if len(lines) != 1 || lines[0].BBox() != [4]float64{72, 757.516, 80.004, 769.516} {
		t.Fatalf("layout bbox = %#v, want Playa miner bbox", lines)
	}
	if glyph.BBox() != [4]float64{72, 757.516, 80.004, 768.616} {
		t.Fatalf("source glyph bbox was changed = %#v", glyph.BBox())
	}
}

func TestAnalyzeLayoutSplitsWordsOnSpaces(t *testing.T) {
	objects := []TextObject{{glyphs: []GlyphObject{
		newTestGlyph(contentdata.GlyphSpec{Text: "A", Origin: [2]float64{0, 10}, Displacement: [2]float64{5, 0}, FontSize: 10, BBox: [4]float64{0, 0, 5, 10}}, nil, nil, geometry.Matrix{}),
		newTestGlyph(contentdata.GlyphSpec{Text: " ", Origin: [2]float64{5, 10}, Displacement: [2]float64{3, 0}, FontSize: 10, BBox: [4]float64{5, 0, 8, 10}}, nil, nil, geometry.Matrix{}),
		newTestGlyph(contentdata.GlyphSpec{Text: "B", Origin: [2]float64{8, 10}, Displacement: [2]float64{5, 0}, FontSize: 10, BBox: [4]float64{8, 0, 13, 10}}, nil, nil, geometry.Matrix{}),
	}}}
	got := AnalyzeLayout(objects, layout.Options{})
	lines := got.LinesCopy()
	if len(lines) != 1 || lines[0].Text() != "A B" || len(lines[0].WordsCopy()) != 2 || lines[0].WordsCopy()[1].Text() != "B" {
		t.Fatalf("words = %#v", got)
	}
}

func TestAnalyzeLayoutKeepsWhitespaceOnlyLinesOutsideContainers(t *testing.T) {
	objects := []TextObject{{glyphs: []GlyphObject{
		newTestGlyph(contentdata.GlyphSpec{Text: "A", Origin: [2]float64{0, 20}, BBox: [4]float64{0, 10, 5, 20}}, nil, nil, geometry.Matrix{}),
		newTestGlyph(contentdata.GlyphSpec{Text: " ", Origin: [2]float64{0, 0}, BBox: [4]float64{0, -10, 5, 0}}, nil, nil, geometry.Matrix{}),
	}}}
	result := AnalyzeLayout(objects, layout.Options{})
	lines := result.LinesCopy()
	if len(lines) != 2 || lines[0].Text() != "A" || lines[1].Text() != " " {
		t.Fatalf("whitespace-only line was not preserved at page level = %#v", lines)
	}
	if boxes := result.TextBoxesCopy(); len(boxes) != 1 || boxes[0].Text() != "A" {
		t.Fatalf("whitespace-only line leaked into text boxes = %#v", boxes)
	}
}

func TestAnalyzeLayoutKeepsUnmappedGlyphWithPlayaCIDText(t *testing.T) {
	objects := []TextObject{{glyphs: []GlyphObject{newTestGlyph(contentdata.GlyphSpec{
		CID: 42, Unmapped: true, Origin: [2]float64{10, 20}, BBox: [4]float64{10, 10, 16, 20},
	}, nil, nil, geometry.Matrix{})}}}
	lines := AnalyzeLayout(objects, layout.Options{}).LinesCopy()
	if len(lines) != 1 || lines[0].Text() != "(cid:42)" || len(lines[0].GlyphsCopy()) != 1 {
		t.Fatalf("unmapped glyph layout = %#v", lines)
	}
}

func TestAnalyzeLayoutRetainsExplicitEmptyGlyphGeometry(t *testing.T) {
	objects := []TextObject{{glyphs: []GlyphObject{newTestGlyph(contentdata.GlyphSpec{
		Origin: [2]float64{10, 20}, BBox: [4]float64{10, 10, 16, 20},
	}, nil, nil, geometry.Matrix{})}}}
	lines := AnalyzeLayout(objects, layout.Options{}).LinesCopy()
	if len(lines) != 1 || lines[0].Text() != "" || len(lines[0].GlyphsCopy()) != 1 {
		t.Fatalf("empty glyph layout = %#v", lines)
	}
}

func TestAnalyzeLayoutWordMarginUsesGlyphBounds(t *testing.T) {
	objects := []TextObject{{glyphs: []GlyphObject{
		newTestGlyph(contentdata.GlyphSpec{Text: "A", Origin: [2]float64{0, 10}, Displacement: [2]float64{2, 0}, FontSize: 100, BBox: [4]float64{0, 0, 2, 2}}, nil, nil, geometry.Matrix{}),
		newTestGlyph(contentdata.GlyphSpec{Text: "B", Origin: [2]float64{7, 10}, Displacement: [2]float64{2, 0}, FontSize: 100, BBox: [4]float64{7, 0, 9, 2}}, nil, nil, geometry.Matrix{}),
	}}}
	lines := AnalyzeLayout(objects, layout.Options{CharMargin: 3, WordMargin: 1}).LinesCopy()
	if len(lines) != 1 || lines[0].Text() != "A B" {
		t.Fatalf("word margin used font size instead of glyph bounds = %#v", lines)
	}
}

func TestAnalyzeLayoutWordMarginUsesPreviousGlyphBoundary(t *testing.T) {
	objects := []TextObject{{glyphs: []GlyphObject{
		// The text matrix says the glyphs are four units apart, but their
		// painted bboxes overlap. Playa uses the painted boundary here.
		newTestGlyph(contentdata.GlyphSpec{Text: "A", Origin: [2]float64{0, 10}, Displacement: [2]float64{1, 0}, BBox: [4]float64{0, 0, 10, 10}}, nil, nil, geometry.Matrix{}),
		newTestGlyph(contentdata.GlyphSpec{Text: "B", Origin: [2]float64{5, 10}, Displacement: [2]float64{1, 0}, BBox: [4]float64{9, 0, 10, 10}}, nil, nil, geometry.Matrix{}),
	}}}
	lines := AnalyzeLayout(objects, layout.Options{}).LinesCopy()
	if len(lines) != 1 || lines[0].Text() != "AB" || len(lines[0].WordsCopy()) != 1 {
		t.Fatalf("word margin used text-matrix gap instead of bbox boundary = %#v", lines)
	}
}

func TestLayoutGlyphWordGapRoundsPlayaMarginBeforeBoundaryComparison(t *testing.T) {
	const (
		previousBoundary = 231.47474207999997
		nextBoundary     = 232.57874207999998
		wordWidth        = 11.039999999999992
	)

	t.Run("horizontal", func(t *testing.T) {
		previous := newTestGlyph(contentdata.GlyphSpec{BBox: [4]float64{0, 0, previousBoundary, 1}}, nil, nil, geometry.Matrix{})
		next := newTestGlyph(contentdata.GlyphSpec{BBox: [4]float64{nextBoundary, 0, nextBoundary + 1, wordWidth}}, nil, nil, geometry.Matrix{})
		if !layoutGlyphHasWordGap(previous, next, false, 0.1) {
			t.Fatal("horizontal word margin was fused with the boundary comparison")
		}
	})

	t.Run("vertical", func(t *testing.T) {
		previous := newTestGlyph(contentdata.GlyphSpec{BBox: [4]float64{0, -previousBoundary, 1, 0}}, nil, nil, geometry.Matrix{})
		next := newTestGlyph(contentdata.GlyphSpec{BBox: [4]float64{0, -nextBoundary - 1, wordWidth, -nextBoundary}}, nil, nil, geometry.Matrix{})
		if !layoutGlyphHasWordGap(previous, next, true, 0.1) {
			t.Fatal("vertical word margin was fused with the boundary comparison")
		}
	})
}

func TestAnalyzeLayoutAcceptsPlayaMarginNames(t *testing.T) {
	objects := []TextObject{{glyphs: []GlyphObject{
		newTestGlyph(contentdata.GlyphSpec{Text: "A", Origin: [2]float64{0, 10}, Displacement: [2]float64{5, 0}, FontSize: 10, BBox: [4]float64{0, 0, 5, 10}}, nil, nil, geometry.Matrix{}),
		newTestGlyph(contentdata.GlyphSpec{Text: "B", Origin: [2]float64{11, 10}, Displacement: [2]float64{5, 0}, FontSize: 10, BBox: [4]float64{11, 0, 16, 10}}, nil, nil, geometry.Matrix{}),
	}}}
	got := AnalyzeLayout(objects, layout.Options{WordMargin: 1}).LinesCopy()
	if len(got) != 1 || got[0].Text() != "AB" {
		t.Fatalf("word margin alias = %#v", got)
	}

	paragraphs := AnalyzeLayout([]TextObject{{glyphs: []GlyphObject{
		newTestGlyph(contentdata.GlyphSpec{Text: "A", Origin: [2]float64{0, 100}, FontSize: 10, BBox: [4]float64{0, 90, 5, 100}}, nil, nil, geometry.Matrix{}),
		newTestGlyph(contentdata.GlyphSpec{Text: "B", Origin: [2]float64{0, 80}, FontSize: 10, BBox: [4]float64{0, 70, 5, 80}}, nil, nil, geometry.Matrix{}),
	}}}, layout.Options{LineMargin: 0.1}).ParagraphsCopy()
	if len(paragraphs) != 2 {
		t.Fatalf("line margin alias = %#v", paragraphs)
	}
}

func TestAnalyzeLayoutInsertsVirtualSpaceForWordGap(t *testing.T) {
	objects := []TextObject{{glyphs: []GlyphObject{
		newTestGlyph(contentdata.GlyphSpec{Text: "A", Origin: [2]float64{0, 10}, Displacement: [2]float64{5, 0}, FontSize: 10, BBox: [4]float64{0, 0, 5, 10}}, nil, nil, geometry.Matrix{}),
		newTestGlyph(contentdata.GlyphSpec{Text: "B", Origin: [2]float64{11, 10}, Displacement: [2]float64{5, 0}, FontSize: 10, BBox: [4]float64{11, 0, 16, 10}}, nil, nil, geometry.Matrix{}),
	}}}
	got := AnalyzeLayout(objects, layout.Options{})
	lines := got.LinesCopy()
	if len(lines) != 1 || lines[0].Text() != "A B" || len(lines[0].WordsCopy()) != 2 {
		t.Fatalf("virtual word space = %#v", lines)
	}
}

func TestAnalyzeLayoutAddsPlayaVirtualSpaceAfterExplicitWhitespace(t *testing.T) {
	objects := []TextObject{{glyphs: []GlyphObject{
		newTestGlyph(contentdata.GlyphSpec{Text: "A", BBox: [4]float64{0, 0, 5, 10}}, nil, nil, geometry.Matrix{}),
		newTestGlyph(contentdata.GlyphSpec{Text: " ", BBox: [4]float64{5, 0, 10, 10}}, nil, nil, geometry.Matrix{}),
		newTestGlyph(contentdata.GlyphSpec{Text: "B", BBox: [4]float64{30, 0, 35, 10}}, nil, nil, geometry.Matrix{}),
	}}}
	got := AnalyzeLayout(objects, layout.Options{CharMargin: 10}).LinesCopy()
	if len(got) != 1 || got[0].Text() != "A  B" {
		t.Fatalf("explicit plus virtual word space = %#v, want %q", got, "A  B")
	}
}

func TestAnalyzeLayoutDoesNotMergeIndependentColumns(t *testing.T) {
	objects := []TextObject{{glyphs: []GlyphObject{
		newTestGlyph(contentdata.GlyphSpec{Text: "A", Origin: [2]float64{0, 100}, FontSize: 10, BBox: [4]float64{0, 90, 5, 100}}, nil, nil, geometry.Matrix{}),
		newTestGlyph(contentdata.GlyphSpec{Text: "B", Origin: [2]float64{30, 100}, FontSize: 10, BBox: [4]float64{30, 90, 35, 100}}, nil, nil, geometry.Matrix{}),
	}}}
	got := AnalyzeLayout(objects, layout.Options{})
	if paragraphs := got.ParagraphsCopy(); len(paragraphs) != 2 {
		t.Fatalf("independent columns were merged = %#v", paragraphs)
	}
}

func TestAnalyzeLayoutUsesBoxesFlowForColumnOrder(t *testing.T) {
	objects := []TextObject{{glyphs: []GlyphObject{
		newTestGlyph(contentdata.GlyphSpec{Text: "B", Origin: [2]float64{30, 100}, FontSize: 10, BBox: [4]float64{30, 90, 35, 100}}, nil, nil, geometry.Matrix{}),
		newTestGlyph(contentdata.GlyphSpec{Text: "A", Origin: [2]float64{0, 100}, FontSize: 10, BBox: [4]float64{0, 90, 5, 100}}, nil, nil, geometry.Matrix{}),
	}}}
	got := AnalyzeLayout(objects, layout.Options{}).LinesCopy()
	if len(got) != 2 || got[0].Text() != "A" || got[1].Text() != "B" {
		t.Fatalf("column reading order = %#v", got)
	}
}

func TestAnalyzeLayoutFallsBackForNonFiniteBoxesFlow(t *testing.T) {
	objects := []TextObject{{glyphs: []GlyphObject{
		newTestGlyph(contentdata.GlyphSpec{Text: "B", Origin: [2]float64{30, 100}, FontSize: 10, BBox: [4]float64{30, 90, 35, 100}}, nil, nil, geometry.Matrix{}),
		newTestGlyph(contentdata.GlyphSpec{Text: "A", Origin: [2]float64{0, 80}, FontSize: 10, BBox: [4]float64{0, 70, 5, 80}}, nil, nil, geometry.Matrix{}),
	}}}
	flow := math.NaN()
	got := AnalyzeLayout(objects, layout.Options{BoxesFlow: &flow}).TextBoxesCopy()
	wantFlow := 0.5
	want := AnalyzeLayout(objects, layout.Options{BoxesFlow: &wantFlow}).TextBoxesCopy()
	if len(got) != len(want) || len(got) != 2 || got[0].Text() != want[0].Text() || got[1].Text() != want[1].Text() {
		t.Fatalf("non-finite boxes_flow changed default order: got=%#v want=%#v", got, want)
	}
}

func TestAnalyzeLayoutFallsBackForNonFiniteThresholds(t *testing.T) {
	objects := []TextObject{{glyphs: []GlyphObject{
		newTestGlyph(contentdata.GlyphSpec{Text: "A", Origin: [2]float64{0, 100}, Displacement: [2]float64{5, 0}, FontSize: 10, BBox: [4]float64{0, 90, 5, 100}}, nil, nil, geometry.Matrix{}),
		newTestGlyph(contentdata.GlyphSpec{Text: "B", Origin: [2]float64{20, 80}, Displacement: [2]float64{5, 0}, FontSize: 10, BBox: [4]float64{20, 70, 25, 80}}, nil, nil, geometry.Matrix{}),
	}}}
	want, err := json.Marshal(AnalyzeLayout(objects, layout.Options{}))
	if err != nil {
		t.Fatal(err)
	}
	bad := layout.Options{
		WordMargin:  math.NaN(),
		LineOverlap: math.Inf(1),
		CharMargin:  math.NaN(),
		LineMargin:  math.Inf(1),
	}
	got, err := json.Marshal(AnalyzeLayout(objects, bad))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(want) {
		t.Fatalf("non-finite thresholds changed layout: got=%s want=%s", got, want)
	}
}

func TestAnalyzeLayoutHonorsFiniteNegativeMargins(t *testing.T) {
	objects := []TextObject{{glyphs: []GlyphObject{
		newTestGlyph(contentdata.GlyphSpec{Text: "A", Origin: [2]float64{0, 10}, BBox: [4]float64{0, 0, 5, 10}}, nil, nil, geometry.Matrix{}),
		newTestGlyph(contentdata.GlyphSpec{Text: "B", Origin: [2]float64{5, 10}, BBox: [4]float64{5, 0, 10, 10}}, nil, nil, geometry.Matrix{}),
	}}}
	lines := AnalyzeLayout(objects, layout.Options{CharMargin: -1}).LinesCopy()
	if len(lines) != 2 {
		t.Fatalf("negative char margin was replaced by the default: %#v", lines)
	}
}

func TestAnalyzeLayoutUsesPlayaContainmentOverlap(t *testing.T) {
	objects := []TextObject{{glyphs: []GlyphObject{
		newTestGlyph(contentdata.GlyphSpec{Text: "A", Origin: [2]float64{0, 10}, BBox: [4]float64{0, 0, 10, 10}}, nil, nil, geometry.Matrix{}),
		newTestGlyph(contentdata.GlyphSpec{Text: "B", Origin: [2]float64{10, 5}, BBox: [4]float64{10, 4.9, 11, 5}}, nil, nil, geometry.Matrix{}),
	}}}
	lines := AnalyzeLayout(objects, layout.Options{LineOverlap: 2}).LinesCopy()
	if len(lines) != 1 || lines[0].Text() != "AB" {
		t.Fatalf("containment overlap was not computed like Playa: %#v", lines)
	}
}

func TestAnalyzeLayoutNegativeLineOverlapStillRequiresOverlap(t *testing.T) {
	objects := []TextObject{{glyphs: []GlyphObject{
		newTestGlyph(contentdata.GlyphSpec{Text: "A", Origin: [2]float64{0, 10}, BBox: [4]float64{0, 0, 5, 10}}, nil, nil, geometry.Matrix{}),
		newTestGlyph(contentdata.GlyphSpec{Text: "B", Origin: [2]float64{5, 11}, BBox: [4]float64{5, 11, 10, 21}}, nil, nil, geometry.Matrix{}),
	}}}
	lines := AnalyzeLayout(objects, layout.Options{LineOverlap: -1}).LinesCopy()
	if len(lines) != 2 {
		t.Fatalf("non-overlapping glyphs merged with negative line overlap: %#v", lines)
	}
}

func TestAnalyzeLayoutNegativeLineOverlapAcceptsTouchingGlyphs(t *testing.T) {
	objects := []TextObject{{glyphs: []GlyphObject{
		newTestGlyph(contentdata.GlyphSpec{Text: "A", Origin: [2]float64{0, 10}, BBox: [4]float64{0, 0, 5, 10}}, nil, nil, geometry.Matrix{}),
		newTestGlyph(contentdata.GlyphSpec{Text: "B", Origin: [2]float64{5, 20}, BBox: [4]float64{5, 10, 10, 20}}, nil, nil, geometry.Matrix{}),
	}}}
	lines := AnalyzeLayout(objects, layout.Options{LineOverlap: -1}).LinesCopy()
	if len(lines) != 1 || lines[0].Text() != "AB" {
		t.Fatalf("touching glyphs were split with negative line overlap: %#v", lines)
	}
}

func TestAnalyzeLayoutNegativeLineOverlapStillRequiresVerticalOverlap(t *testing.T) {
	objects := []TextObject{{glyphs: []GlyphObject{
		newTestGlyph(contentdata.GlyphSpec{Text: "A", Origin: [2]float64{10, 20}, BBox: [4]float64{0, 15, 10, 20}}, nil, nil, geometry.Matrix{}),
		newTestGlyph(contentdata.GlyphSpec{Text: "B", Origin: [2]float64{11, 9}, BBox: [4]float64{11, 4, 21, 9}}, nil, nil, geometry.Matrix{}),
	}}}
	lines := AnalyzeLayout(objects, layout.Options{DetectVertical: true, LineOverlap: -1}).LinesCopy()
	if len(lines) != 2 {
		t.Fatalf("non-overlapping vertical glyphs merged with negative line overlap: %#v", lines)
	}
}

func TestAnalyzeLayoutSkipsNonFiniteGlyphGeometry(t *testing.T) {
	objects := []TextObject{{glyphs: []GlyphObject{
		newTestGlyph(contentdata.GlyphSpec{Text: "A", Origin: [2]float64{0, 10}, Displacement: [2]float64{5, 0}, BBox: [4]float64{0, 0, 5, 10}}, nil, nil, geometry.Matrix{}),
		newTestGlyph(contentdata.GlyphSpec{Text: "?", Origin: [2]float64{math.NaN(), 10}, BBox: [4]float64{math.NaN(), 0, 5, 10}}, nil, nil, geometry.Matrix{}),
		newTestGlyph(contentdata.GlyphSpec{Text: "B", Origin: [2]float64{6, 10}, Displacement: [2]float64{5, 0}, BBox: [4]float64{6, 0, 11, 10}}, nil, nil, geometry.Matrix{}),
	}}}
	lines := AnalyzeLayout(objects, layout.Options{}).LinesCopy()
	if len(lines) != 1 || lines[0].Text() != "AB" {
		t.Fatalf("non-finite glyph geometry leaked into layout = %#v", lines)
	}
}

func TestAnalyzeLayoutUsesBoxesFlowForTextBoxOrder(t *testing.T) {
	objects := []TextObject{{glyphs: []GlyphObject{
		newTestGlyph(contentdata.GlyphSpec{Text: "A", Origin: [2]float64{0, 80}, FontSize: 10, BBox: [4]float64{0, 70, 5, 80}}, nil, nil, geometry.Matrix{}),
		newTestGlyph(contentdata.GlyphSpec{Text: "B", Origin: [2]float64{30, 100}, FontSize: 10, BBox: [4]float64{30, 90, 35, 100}}, nil, nil, geometry.Matrix{}),
	}}}
	flow := -1.0
	boxes := AnalyzeLayout(objects, layout.Options{BoxesFlow: &flow}).TextBoxesCopy()
	if len(boxes) != 2 || boxes[0].Text() != "A" || boxes[1].Text() != "B" || boxes[0].Index() != 0 || boxes[1].Index() != 1 {
		t.Fatalf("left-to-right textbox order = %#v", boxes)
	}
	flow = 1
	boxes = AnalyzeLayout(objects, layout.Options{BoxesFlow: &flow}).TextBoxesCopy()
	if len(boxes) != 2 || boxes[0].Text() != "B" || boxes[1].Text() != "A" || boxes[0].Index() != 0 || boxes[1].Index() != 1 {
		t.Fatalf("top-to-bottom textbox order = %#v", boxes)
	}
}

func TestAnalyzeLayoutCanDisableBoxesFlow(t *testing.T) {
	objects := []TextObject{{glyphs: []GlyphObject{
		newTestGlyph(contentdata.GlyphSpec{Text: "B", Origin: [2]float64{30, 100}, FontSize: 10, BBox: [4]float64{30, 90, 35, 100}}, nil, nil, geometry.Matrix{}),
		newTestGlyph(contentdata.GlyphSpec{Text: "A", Origin: [2]float64{0, 80}, FontSize: 10, BBox: [4]float64{0, 70, 5, 80}}, nil, nil, geometry.Matrix{}),
	}}}
	result := AnalyzeLayout(objects, layout.Options{DisableBoxesFlow: true})
	boxes := result.TextBoxesCopy()
	if len(boxes) != 2 || boxes[0].Text() != "B" || boxes[1].Text() != "A" {
		t.Fatalf("boxes_flow=None positional order = %#v", boxes)
	}
	if boxes[0].Index() != -1 || boxes[1].Index() != -1 {
		t.Fatalf("boxes_flow=None assigned reading indexes = %d, %d; want -1, -1", boxes[0].Index(), boxes[1].Index())
	}
	if groups := result.TextGroupsCopy(); groups != nil {
		t.Fatalf("boxes_flow=None produced text groups = %#v", groups)
	}
}

func TestAnalyzeLayoutDisablesBoxesFlowForLineOrder(t *testing.T) {
	objects := []TextObject{{glyphs: []GlyphObject{
		newTestGlyph(contentdata.GlyphSpec{Text: "A", Origin: [2]float64{100, 100}, FontSize: 10, BBox: [4]float64{100, 90, 105, 100}}, nil, nil, geometry.Matrix{}),
		newTestGlyph(contentdata.GlyphSpec{Text: "B", Origin: [2]float64{0, 90}, FontSize: 10, BBox: [4]float64{0, 80, 5, 90}}, nil, nil, geometry.Matrix{}),
	}}}
	result := AnalyzeLayout(objects, layout.Options{DisableBoxesFlow: true})
	lines := result.LinesCopy()
	boxes := result.TextBoxesCopy()
	if len(lines) != 2 || len(boxes) != 2 || lines[0].Text() != boxes[0].Text() || lines[1].Text() != boxes[1].Text() {
		t.Fatalf("boxes_flow=None line order = %#v, boxes = %#v", lines, boxes)
	}
	if lines[0].Text() != "A" || lines[1].Text() != "B" {
		t.Fatalf("boxes_flow=None positional line order = %#v", lines)
	}
}

func TestAnalyzeLayoutGroupsTextBoxesByDistance(t *testing.T) {
	objects := []TextObject{{glyphs: []GlyphObject{
		newTestGlyph(contentdata.GlyphSpec{Text: "A", Origin: [2]float64{0, 100}, FontSize: 10, BBox: [4]float64{0, 90, 5, 100}}, nil, nil, geometry.Matrix{}),
		newTestGlyph(contentdata.GlyphSpec{Text: "B", Origin: [2]float64{0, 80}, FontSize: 10, BBox: [4]float64{0, 70, 5, 80}}, nil, nil, geometry.Matrix{}),
	}}}
	groups := AnalyzeLayout(objects, layout.Options{}).TextGroupsCopy()
	if len(groups) != 1 || groups[0].Text() != "A\nB" || len(groups[0].BoxesCopy()) != 2 {
		t.Fatalf("text groups = %#v", groups)
	}
}

func TestTextGroupsPreserveNestedPlayaHierarchy(t *testing.T) {
	boxes := []TextBox{
		newTestTextBox("A", [4]float64{0, 0, 10, 10}, false, -1),
		newTestTextBox("B", [4]float64{11, 0, 21, 10}, false, -1),
		newTestTextBox("C", [4]float64{22, 0, 32, 10}, false, -1),
	}
	groups := layoutTextGroups(boxes, 0.5)
	if len(groups) != 1 {
		t.Fatalf("text groups = %#v, want one root", groups)
	}
	children := groups[0].ChildrenCopy()
	if len(children) != 2 || !children[0].IsGroup() || !children[1].IsBox() {
		t.Fatalf("root children = %#v, want nested group followed by textbox", children)
	}
	nested, ok := children[0].GroupCopy()
	if !ok {
		t.Fatalf("first child is not a group: %#v", children[0])
	}
	if got := nested.Text(); got != "A\nB" {
		t.Fatalf("nested group text = %q, want A\\nB", got)
	}
}

func TestTextGroupsPreservePlayaDistanceOrdering(t *testing.T) {
	boxes := []TextBox{
		newTestTextBox("A", [4]float64{89.9, 703.408, 505.58000000000004, 715.408}, false, -1),
		newTestTextBox("B", [4]float64{89.9, 680.068, 505.52, 692.068}, false, -1),
		newTestTextBox("C", [4]float64{89.9, 656.728, 505.58000000000004, 668.728}, false, -1),
	}

	groups := layoutTextGroups(boxes, 0.5)
	if len(groups) != 1 {
		t.Fatalf("text groups = %#v, want one root", groups)
	}
	children := groups[0].ChildrenCopy()
	if len(children) != 2 || !children[0].IsGroup() || !children[1].IsBox() {
		t.Fatalf("root children = %#v, want A+B group followed by C", children)
	}
	nested, ok := children[0].GroupCopy()
	if !ok || nested.Text() != "A\nB" {
		t.Fatalf("nested group = %#v, want A\\nB", nested)
	}
}

func TestTextGroupsNormalizeSubToleranceDistanceOrdering(t *testing.T) {
	boxes := []TextBox{
		newTestTextBox("1416", [4]float64{449.5, 499.82800000000003, 476.5, 511.82800000000003}, false, -1),
		newTestTextBox("1765", [4]float64{449.5, 479.488, 476.5, 491.488}, false, -1),
		newTestTextBox("1806", [4]float64{449.5, 459.148, 476.5, 471.148}, false, -1),
	}

	groups := layoutTextGroups(boxes, 0.5)
	if len(groups) != 1 {
		t.Fatalf("text groups = %#v, want one root", groups)
	}
	children := groups[0].ChildrenCopy()
	if len(children) != 2 || !children[0].IsGroup() || !children[1].IsBox() {
		t.Fatalf("root children = %#v, want normalized 1416+1765 group followed by 1806", children)
	}
	nested, ok := children[0].GroupCopy()
	if !ok || nested.Text() != "1416\n1765" {
		t.Fatalf("nested group = %#v, want 1416\\n1765", nested)
	}
}

func TestTextGroupDistanceMatchesPlayaRoundedHeapBoundary(t *testing.T) {
	left := newTestTextGroup("lower rows", [4]float64{162.38, 521.632, 505.485, 595.7995}, false)
	right := newTestTextGroup("upper row", [4]float64{167.66, 623.992, 505.485, 635.2795}, false)
	if got := normalizeLayoutHeapDistance(textGroupDistance(left, right)); got != 9732.585713 {
		t.Fatalf("normalized group distance = %.6f, want Playa %.6f", got, 9732.585713)
	}
}

func TestNormalizeLayoutHeapDistanceCommonCaseUsesFastPath(t *testing.T) {
	got, ok := normalizeLayoutHeapDistanceFast(1234.5678912)
	if !ok || got != 1234.567891 {
		t.Fatalf("fast layout distance normalization = %v, %v; want 1234.567891, true", got, ok)
	}
}

func TestNormalizeLayoutHeapDistanceMatchesDecimalReference(t *testing.T) {
	reference := func(value float64) float64 {
		if math.IsNaN(value) || math.IsInf(value, 0) || math.Abs(value) > math.MaxFloat64/layoutHeapDistanceScale {
			return value
		}
		if math.Abs(value) >= 1<<46 {
			return value
		}
		normalized, err := strconv.ParseFloat(strconv.FormatFloat(value, 'f', 6, 64), 64)
		if err != nil {
			return value
		}
		return normalized
	}
	check := func(value float64) {
		t.Helper()
		got, want := normalizeLayoutHeapDistance(value), reference(value)
		if math.Float64bits(got) != math.Float64bits(want) {
			t.Fatalf("normalizeLayoutHeapDistance(%0.17g) = %0.17g (%016x), want %0.17g (%016x)", value, got, math.Float64bits(got), want, math.Float64bits(want))
		}
	}
	for _, value := range []float64{0, math.Copysign(0, -1), math.NaN(), math.Inf(-1), math.Inf(1), 9732.5857125000002, 1 << 33, -(1 << 33), 1 << 46, -(1 << 46)} {
		check(value)
	}
	for _, integer := range []float64{0, 1, 2, 999_999, 1 << 32, (1 << 52) - 2} {
		for _, sign := range []float64{-1, 1} {
			half := sign * (integer + 0.5) / layoutHeapDistanceScale
			check(math.Nextafter(half, math.Inf(-1)))
			check(half)
			check(math.Nextafter(half, math.Inf(1)))
		}
	}
	random := rand.New(rand.NewSource(1))
	for _, scale := range []float64{1e-12, 1e-6, 1, 1e3, 1e6, 1e9, 1 << 33, 1 << 46, 1e100} {
		for range 10_000 {
			check((random.Float64()*2 - 1) * scale)
		}
	}
}

func TestTextGroupsPreservePlayaTOCGroupShape(t *testing.T) {
	boxes := []TextBox{
		newTestTextBox("Example organization    Engineering report", [4]float64{90.68, 782.3760000000001, 506.97, 792.051}, false, -1),
		newTestTextBox("Contents", [4]float64{276.58, 738.5203200000001, 322.25500000000005, 753.5488200000001}, false, -1),
		newTestTextBox("Introduction", [4]float64{90.02000000000004, 704.508, 510.885, 717.4080000000001}, false, -1),
		newTestTextBox("Section 1    Project overview", [4]float64{90.01999999999998, 681.088, 510.885, 693.988}, false, -1),
		newTestTextBox("1. Organization", [4]float64{114.01999999999998, 657.688, 510.885, 670.588}, false, -1),
		newTestTextBox("2. Contributors", [4]float64{114.01999999999998, 634.288, 510.885, 647.188}, false, -1),
		newTestTextBox("3. Background", [4]float64{114.01999999999998, 610.888, 510.885, 623.788}, false, -1),
		newTestTextBox("4. Dependencies", [4]float64{114.01999999999998, 587.488, 510.885, 600.388}, false, -1),
		newTestTextBox("5. Review procedure", [4]float64{114.01999999999998, 564.088, 510.885, 576.988}, false, -1),
		newTestTextBox("Section 2    Design requirements", [4]float64{90.01999999999998, 540.688, 510.885, 553.588}, false, -1),
		newTestTextBox("Section 3    Validation", [4]float64{90.01999999999998, 517.288, 510.885, 530.188}, false, -1),
		newTestTextBox("1. Results", [4]float64{114.01999999999998, 493.88800000000003, 510.885, 506.788}, false, -1),
		newTestTextBox("2. Reproducibility", [4]float64{114.01999999999998, 470.488, 510.885, 483.38800000000003}, false, -1),
		newTestTextBox("3. Scope", [4]float64{114.01999999999998, 447.988, 505.34, 459.988}, false, -1),
		newTestTextBox("Evidence and verification procedure", [4]float64{114.01999999999998, 423.688, 510.885, 436.588}, false, -1),
		newTestTextBox("4. Inputs", [4]float64{114.01999999999998, 400.268, 510.885, 413.16799999999995}, false, -1),
		newTestTextBox("5. Outputs", [4]float64{114.01999999999998, 376.868, 510.885, 389.76800000000003}, false, -1),
		newTestTextBox("6. Limitations", [4]float64{114.01999999999998, 353.468, 510.885, 366.368}, false, -1),
		newTestTextBox("7. Future work", [4]float64{114.01999999999998, 330.06800000000004, 510.885, 342.968}, false, -1),
		newTestTextBox("8. Dependencies", [4]float64{114.01999999999998, 306.668, 510.885, 319.56800000000004}, false, -1),
		newTestTextBox("9. Resource usage", [4]float64{114.01999999999998, 283.26800000000003, 510.885, 296.168}, false, -1),
		newTestTextBox("10. Performance", [4]float64{114.01999999999998, 259.868, 510.885, 272.76800000000003}, false, -1),
		newTestTextBox("11. Conclusion", [4]float64{114.01999999999998, 236.46799999999996, 510.885, 249.36799999999997}, false, -1),
		newTestTextBox("[page 1]", [4]float64{284.2, 49.775999999999996, 313.45, 58.775999999999996}, false, -1),
	}
	groups := layoutTextGroups(boxes, 0.5)
	if len(groups) != 1 || !hasTOCGroupShape(groups[0]) {
		t.Fatalf("TOC groups = %#v, want Playa section hierarchy", groups)
	}
}

func TestTextGroupsPreservePlayaPageNumberGroupShape(t *testing.T) {
	boxes := make([]TextBox, 0, 9)
	for index, y := range []float64{599.812, 576.352, 552.8919999999999, 529.492, 506.03200000000004, 482.572, 459.112, 435.65200000000004, 412.172} {
		boxes = append(boxes, newTestTextBox(strconv.Itoa(index+1), [4]float64{120.98, y, 128.825, y + 10.5}, false, -1))
	}
	groups := layoutTextGroups(boxes, 0.5)
	if len(groups) != 1 || !hasPageNumberGroupShape(groups[0]) {
		t.Fatalf("page-number groups = %#v, want Playa serial-number hierarchy", groups)
	}
}

func TestTextGroupReadingOrderDrivesFlatTextboxProjection(t *testing.T) {
	boxes := []TextBox{
		{data: newTestLayoutComponent("A", [4]float64{0, 90, 10, 100}, false, -1), lines: []TextLine{newTestTextLine("A", [4]float64{}, false)}},
		{data: newTestLayoutComponent("B", [4]float64{30, 70, 40, 80}, false, -1), lines: []TextLine{newTestTextLine("B", [4]float64{}, false)}},
		{data: newTestLayoutComponent("C", [4]float64{60, 50, 70, 60}, false, -1), lines: []TextLine{newTestTextLine("C", [4]float64{}, false)}},
	}
	groups := layoutTextGroups(boxes, 0.5)
	assignTextBoxIndexes(groups)
	ordered := orderedTextBoxes(groups)
	if len(ordered) != 3 {
		t.Fatalf("ordered textboxes = %#v, want three boxes", ordered)
	}
	for i, want := range []string{"A", "B", "C"} {
		if ordered[i].Text() != want || ordered[i].Index() != i {
			t.Fatalf("ordered textbox[%d] = %q(index=%d), want %q(index=%d)", i, ordered[i].Text(), ordered[i].Index(), want, i)
		}
	}
	lines := orderedTextLines(ordered)
	if len(lines) != 3 || lines[0].Text() != "A" || lines[1].Text() != "B" || lines[2].Text() != "C" {
		t.Fatalf("ordered lines = %#v, want textbox reading order", lines)
	}
}

func TestTextGroupsBlockedUsesIntersectingObjects(t *testing.T) {
	groups := []TextGroup{
		newTestTextGroup("", [4]float64{0, 0, 10, 10}, false),
		newTestTextGroup("", [4]float64{20, 20, 30, 30}, false),
		// This box crosses the candidate rectangle's boundary. Playa's
		// Plane.find still reports it as an intervening object.
		newTestTextGroup("", [4]float64{-5, 15, 25, 35}, false),
	}
	if !textGroupsBlocked(groups, 0, 1) {
		t.Fatalf("partially intersecting textbox was ignored")
	}
}

func TestTextGroupsBlockedExcludesBoundaryTouchingObjects(t *testing.T) {
	groups := []TextGroup{
		newTestTextGroup("", [4]float64{0, 0, 10, 10}, false),
		newTestTextGroup("", [4]float64{20, 20, 30, 30}, false),
		newTestTextGroup("", [4]float64{30, 5, 40, 15}, false),
	}
	if textGroupsBlocked(groups, 0, 1) {
		t.Fatal("boundary-touching textbox was treated as an obstacle")
	}
	nodes := make([]*layoutGroupNode, len(groups))
	for i, group := range groups {
		nodes[i] = &layoutGroupNode{group: group, active: true}
	}
	if textGroupsBlockedNodes(nodes, 0, 1) {
		t.Fatal("boundary-touching active textbox was treated as an obstacle")
	}
}

func TestLayoutTextLinesNeighborUsesSourceLineTolerance(t *testing.T) {
	short := newTestTextLine("", [4]float64{0, 0, 10, 10}, false)
	long := newTestTextLine("", [4]float64{0, 12.2, 10, 24.4}, false)

	// Playa's LTTextLine.find_neighbors computes d from the line being
	// queried, rather than widening it to the larger of both line heights.
	// With ratio .2, the taller line accepts the pair (d=2.44), while the
	// shorter line does not (d=2).
	if layoutTextLinesNeighbor(short, long, 0.2) {
		t.Fatal("short source line used the neighbor's larger tolerance")
	}
	if !layoutTextLinesNeighbor(long, short, 0.2) {
		t.Fatal("long source line did not use its own tolerance")
	}
}

func TestLayoutTextLinesNeighborRejectsDistantAlignedLines(t *testing.T) {
	first := newTestTextLine("", [4]float64{0, 0, 10, 10}, false)
	second := newTestTextLine("", [4]float64{0, 100, 10, 110}, false)
	if layoutTextLinesNeighbor(first, second, 0.2) {
		t.Fatal("distant aligned lines were treated as neighbors")
	}
}

func TestLayoutTextLinesNeighborRejectsExactPlaneBoundary(t *testing.T) {
	for _, test := range []struct {
		name   string
		first  TextLine
		second TextLine
	}{
		{
			name:   "horizontal",
			first:  newTestTextLine("", [4]float64{0, 0, 10, 10}, false),
			second: newTestTextLine("", [4]float64{0, 12, 10, 22}, false),
		},
		{
			name:   "vertical",
			first:  newTestTextLine("", [4]float64{0, 0, 10, 10}, true),
			second: newTestTextLine("", [4]float64{12, 0, 22, 10}, true),
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			if layoutTextLinesNeighbor(test.first, test.second, 0.2) {
				t.Fatal("line exactly on Playa Plane boundary was treated as a neighbor")
			}
		})
	}
}

func TestLayoutTextLinesNeighborUsesCenterDistance(t *testing.T) {
	horizontal := []TextLine{
		newTestTextLine("A", [4]float64{0, 0, 10, 10}, false),
		newTestTextLine("B", [4]float64{5, 10.5, 7.5, 20.5}, false),
	}
	if !layoutTextLinesNeighbor(horizontal[0], horizontal[1], 0.2) {
		t.Fatal("horizontal center alignment used twice the Playa tolerance")
	}

	vertical := []TextLine{
		newTestTextLine("A", [4]float64{0, 0, 10, 10}, true),
		newTestTextLine("B", [4]float64{10.5, 5, 20.5, 7.5}, true),
	}
	if !layoutTextLinesNeighbor(vertical[0], vertical[1], 0.2) {
		t.Fatal("vertical center alignment used twice the Playa tolerance")
	}
}

func TestLayoutTextLinesNeighborRejectsDistantVerticalLines(t *testing.T) {
	first := newTestTextLine("", [4]float64{0, 0, 10, 10}, true)
	second := newTestTextLine("", [4]float64{100, 0, 110, 10}, true)
	if layoutTextLinesNeighbor(first, second, 0.2) {
		t.Fatal("distant vertical lines were treated as neighbors")
	}
}

func TestLayoutTextLinePlaneBoundsOversizedFiniteGrid(t *testing.T) {
	const maxGridBuckets = 1 << 12
	span := float64(maxGridBuckets+1) * layoutPlaneGridSize
	lines := []TextLine{
		newTestTextLine("wide", [4]float64{0, 0, span, 10}, false),
		newTestTextLine("normal", [4]float64{0, 0, 10, 10}, false),
	}
	for _, test := range []struct {
		name   string
		bounds *[4]float64
	}{
		{name: "unbounded"},
		{name: "bounded", bounds: &[4]float64{0, 0, span, 100}},
	} {
		t.Run(test.name, func(t *testing.T) {
			plane := newLayoutTextLinePlaneWithin(lines, test.bounds)
			if len(plane.grid) > maxGridBuckets {
				t.Fatalf("oversized finite bbox materialized %d grid buckets", len(plane.grid))
			}
			neighbors := plane.neighbors(0, 0.5)
			if len(neighbors) != 2 || neighbors[0] != 0 || neighbors[1] != 1 {
				t.Fatalf("oversized fallback neighbors = %v, want source order [0 1]", neighbors)
			}
		})
	}
}

func TestAnalyzeLayoutOversizedFiniteGeometryUsesFallback(t *testing.T) {
	objects := []TextObject{{glyphs: []GlyphObject{
		newTestGlyph(contentdata.GlyphSpec{Text: "wide", BBox: [4]float64{0, 0, 1e18, 10}}, nil, nil, geometry.Matrix{}),
		newTestGlyph(contentdata.GlyphSpec{Text: "normal", BBox: [4]float64{0, 20, 10, 30}}, nil, nil, geometry.Matrix{}),
	}}}
	boxes := AnalyzeLayout(objects, layout.Options{}).TextBoxesCopy()
	if len(boxes) != 2 {
		t.Fatalf("oversized finite geometry text boxes = %d, want 2", len(boxes))
	}
	seen := map[string]bool{}
	for _, box := range boxes {
		seen[box.Text()] = true
	}
	if !seen["wide"] || !seen["normal"] {
		t.Fatalf("oversized fallback text boxes = %#v", seen)
	}
}

func TestLayoutTextLinePlaneBoundsClipOutsideAndTouchingLines(t *testing.T) {
	baseLines := []TextLine{
		newTestTextLine("inside", [4]float64{10, 10, 20, 20}, false),
		newTestTextLine("touching", [4]float64{10, -10, 20, 0}, false),
		newTestTextLine("outside", [4]float64{30, -20, 40, -10}, false),
		newTestTextLine("crossing", [4]float64{50, -5, 60, 5}, false),
	}
	contains := func(values []int, target int) bool {
		for _, value := range values {
			if value == target {
				return true
			}
		}
		return false
	}
	for _, test := range []struct {
		name          string
		bounds        [4]float64
		forceFallback bool
	}{
		{name: "grid", bounds: [4]float64{0, 0, 100, 100}},
		{name: "fallback", bounds: [4]float64{0, 0, 1e18, 100}, forceFallback: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			lines := append([]TextLine(nil), baseLines...)
			if test.forceFallback {
				lines = append(lines, newTestTextLine("oversized", [4]float64{0, 50, 1e18, 60}, false))
			}
			plane := newLayoutTextLinePlaneWithin(lines, &test.bounds)
			if plane.fallback != test.forceFallback {
				t.Fatalf("Plane fallback = %v, want %v", plane.fallback, test.forceFallback)
			}
			if contains(plane.neighbors(1, 0.5), 1) {
				t.Fatal("line touching the lower Plane boundary was indexed")
			}
			if neighbors := plane.neighbors(2, 0.5); len(neighbors) != 0 {
				t.Fatalf("line outside the Plane found neighbors: %v", neighbors)
			}
			if !contains(plane.neighbors(3, 0.5), 3) {
				t.Fatal("line crossing the lower Plane boundary was not indexed")
			}
		})
	}
}

func TestPageLayoutPlaneBoundsTransformMediaBox(t *testing.T) {
	for _, test := range []struct {
		name   string
		space  CoordinateSpace
		rotate Number
		want   [4]float64
	}{
		{name: "page-90", space: CoordinateSpacePage, rotate: 90, want: [4]float64{0, 0, 400, 200}},
		{name: "screen-270", space: CoordinateSpaceScreen, rotate: 270, want: [4]float64{0, 0, 400, 200}},
		{name: "default-90", space: CoordinateSpaceDefault, rotate: 90, want: [4]float64{0, 0, 100, 200}},
	} {
		t.Run(test.name, func(t *testing.T) {
			d := &Document{space: test.space}
			page := Page{space: test.space, dict: Dict{
				Name("MediaBox"): Array{Number(10), Number(20), Number(110), Number(220)},
				Name("UserUnit"): Number(2),
				Name("Rotate"):   test.rotate,
			}}
			if got := page.layoutPlaneBounds(d); got != test.want {
				t.Fatalf("layout Plane bounds = %#v, want %#v", got, test.want)
			}
		})
	}
}

func TestTextBoxesGroupReverseDirectionalNeighbors(t *testing.T) {
	short := newTestTextLine("short", [4]float64{0, 0, 10, 10}, false)
	long := newTestTextLine("long", [4]float64{0, 12.2, 10, 24.4}, false)
	boxes := textBoxesFromLines([]TextLine{short, long}, 0.2, 0.5, false)
	if len(boxes) != 1 || boxes[0].Text() != "long\nshort" {
		t.Fatalf("reverse directional neighbors were not grouped: %#v", boxes)
	}
}

func TestTextBoxesPreservePlayaNeighborInsertionOrder(t *testing.T) {
	lines := []TextLine{
		newTestTextLine("A", [4]float64{-5, 0, 5, 10}, false),
		newTestTextLine("B", [4]float64{-3.5, 0, 6.5, 10}, false),
		newTestTextLine("C", [4]float64{-2, 0, 8, 10}, false),
	}
	boxes := textBoxesFromLines(lines, 0.2, 0.5, false)
	if len(boxes) != 1 || boxes[0].Text() != "C\nB\nA" {
		t.Fatalf("textbox member order = %#v, want Playa order C/B/A", boxes)
	}
}

func TestLayoutGroupPairHeapPrefersUnblockedTie(t *testing.T) {
	nodes := []*layoutGroupNode{{tieID: 0}}
	h := layoutGroupPairHeap{nodes: &nodes, values: []layoutGroupPair{
		{distance: 1, left: layoutGroupPairBlocked},
		{distance: 1},
	}}
	h.init()
	first := h.pop()
	if first.isBlocked() {
		t.Fatal("blocked pair won an equal-distance heap tie")
	}
}

func TestLayoutGroupPairHeapPrioritizesUnblockedPairsBeforeDistance(t *testing.T) {
	nodes := []*layoutGroupNode{{tieID: 0}}
	h := layoutGroupPairHeap{nodes: &nodes, values: []layoutGroupPair{
		{distance: 1, left: layoutGroupPairBlocked},
		{distance: 2},
	}}
	h.init()
	first := h.pop()
	if first.isBlocked() {
		t.Fatalf("blocked pair with a shorter distance won over an unblocked pair: %#v", first)
	}
}

func TestLayoutGroupPairHeapOrderIsTransitive(t *testing.T) {
	box := TextBox{}
	nodes := []*layoutGroupNode{
		{tieID: 3},
		{tieID: 1},
		{tieID: 2, box: &box},
		{tieID: 4},
	}
	h := layoutGroupPairHeap{nodes: &nodes, values: []layoutGroupPair{
		{distance: 1, left: 0, right: 3},
		{distance: 1, left: 1, right: 3},
		{distance: 1, left: 2, right: 3},
	}}
	for i := range h.values {
		for j := range h.values {
			for k := range h.values {
				if h.Less(i, j) && h.Less(j, k) && !h.Less(i, k) {
					t.Fatalf("heap order is not transitive: %d < %d and %d < %d, but not %d < %d", i, j, j, k, i, k)
				}
			}
		}
	}
}

func TestLayoutGroupPairHeapCompactsOnlyInactivePairs(t *testing.T) {
	const nodeCount = 24
	nodes := make([]*layoutGroupNode, nodeCount)
	for index := range nodes {
		nodes[index] = &layoutGroupNode{active: index%2 == 0, tieID: index}
	}
	heap := layoutGroupPairHeap{nodes: &nodes}
	for left := 0; left < len(nodes); left++ {
		for right := left + 1; right < len(nodes); right++ {
			heap.values = append(heap.values, newLayoutGroupPair(left, right, float64(left*nodeCount+right)))
		}
	}
	heap.init()
	heap.compactInactive(nodeCount / 2)
	want := (nodeCount / 2) * (nodeCount/2 - 1) / 2
	if heap.Len() != want {
		t.Fatalf("compacted heap length = %d, want %d", heap.Len(), want)
	}
	for heap.Len() > 0 {
		pair := heap.pop()
		if !nodes[pair.leftID()].active || !nodes[pair.rightID()].active {
			t.Fatalf("compacted heap retained inactive pair %#v", pair)
		}
	}
}

func TestTextGroupSortUsesGroupWritingModeForMixedBoxes(t *testing.T) {
	horizontal := newTestTextBox("horizontal", [4]float64{0, 100, 10, 110}, false, -1)
	vertical := newTestTextBox("vertical", [4]float64{95, 0, 105, 10}, true, -1)
	group := textGroupFromBoxes([]TextBox{horizontal, vertical}, 0.5)
	boxes := group.BoxesCopy()
	if !group.Vertical() || len(boxes) != 2 || !boxes[0].Vertical() || boxes[1].Vertical() {
		t.Fatalf("mixed text-group order = %#v", boxes)
	}
}

func TestAnalyzeLayoutSerializesContainerProjection(t *testing.T) {
	result := AnalyzeLayout([]TextObject{{glyphs: []GlyphObject{
		newTestGlyph(contentdata.GlyphSpec{Text: "A", Origin: [2]float64{0, 100}, BBox: [4]float64{0, 90, 5, 100}}, nil, nil, geometry.Matrix{}),
	}}}, layout.Options{})
	data, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{`"Lines"`, `"Paragraphs"`, `"TextBoxes"`, `"TextGroups"`, `"Index":0`} {
		if !bytes.Contains(data, []byte(key)) {
			t.Fatalf("layout JSON missing %s: %s", key, data)
		}
	}
}

func TestAnalyzeLayoutGroupsInterleavedLinesByColumn(t *testing.T) {
	objects := []TextObject{{glyphs: []GlyphObject{
		newTestGlyph(contentdata.GlyphSpec{Text: "A1", Origin: [2]float64{0, 100}, FontSize: 10, BBox: [4]float64{0, 90, 10, 100}}, nil, nil, geometry.Matrix{}),
		newTestGlyph(contentdata.GlyphSpec{Text: "B1", Origin: [2]float64{50, 100}, FontSize: 10, BBox: [4]float64{50, 90, 60, 100}}, nil, nil, geometry.Matrix{}),
		newTestGlyph(contentdata.GlyphSpec{Text: "A2", Origin: [2]float64{0, 85}, FontSize: 10, BBox: [4]float64{0, 75, 10, 85}}, nil, nil, geometry.Matrix{}),
		newTestGlyph(contentdata.GlyphSpec{Text: "B2", Origin: [2]float64{50, 85}, FontSize: 10, BBox: [4]float64{50, 75, 60, 85}}, nil, nil, geometry.Matrix{}),
	}}}
	paragraphs := AnalyzeLayout(objects, layout.Options{}).ParagraphsCopy()
	if len(paragraphs) != 2 || paragraphs[0].Text() != "A1\nA2" || paragraphs[1].Text() != "B1\nB2" {
		t.Fatalf("interleaved column paragraphs = %#v", paragraphs)
	}
}

func TestTextGroupsBlockedNodesUsesStrictPlaneIntersection(t *testing.T) {
	nodes := []*layoutGroupNode{
		{group: newTestTextGroup("", [4]float64{0, 0, 10, 10}, false), active: true},
		{group: newTestTextGroup("", [4]float64{20, 0, 30, 10}, false), active: true},
		{group: newTestTextGroup("", [4]float64{-10, 2, 0, 8}, false), active: true},
	}
	if textGroupsBlockedNodes(nodes, 0, 1) {
		t.Fatal("a group touching the candidate boundary was treated as an obstacle")
	}
}

func TestTextGroupsBlockedNodesIgnoresObjectsOutsideBoundedPlane(t *testing.T) {
	nodes := []*layoutGroupNode{
		{group: newTestTextGroup("left", [4]float64{10, 10, 20, 20}, false), active: true},
		{group: newTestTextGroup("right", [4]float64{120, 10, 130, 20}, false), active: true},
		{group: newTestTextGroup("outside", [4]float64{105, 10, 110, 20}, false), active: true},
	}
	if !textGroupsBlockedNodes(nodes, 0, 1) {
		t.Fatal("unbounded group scan ignored the intervening object")
	}
	bounds := [4]float64{0, 0, 100, 100}
	if textGroupsBlockedNodesWithin(nodes, 0, 1, &bounds) {
		t.Fatal("object outside the bounded Playa Plane blocked a group merge")
	}
}

func TestAnalyzeLayoutSplitsDistantParagraphs(t *testing.T) {
	objects := []TextObject{{glyphs: []GlyphObject{
		newTestGlyph(contentdata.GlyphSpec{Text: "A", Origin: [2]float64{0, 100}, FontSize: 10, BBox: [4]float64{0, 98, 5, 108}}, nil, nil, geometry.Matrix{}),
		newTestGlyph(contentdata.GlyphSpec{Text: "B", Origin: [2]float64{0, 50}, FontSize: 10, BBox: [4]float64{0, 48, 5, 58}}, nil, nil, geometry.Matrix{}),
	}}}
	got := AnalyzeLayout(objects, layout.Options{})
	paragraphs := got.ParagraphsCopy()
	if len(paragraphs) != 2 || paragraphs[0].Text() != "A" || paragraphs[1].Text() != "B" {
		t.Fatalf("paragraphs = %#v", paragraphs)
	}
}

func TestAnalyzeLayoutHonorsIndependentParagraphGap(t *testing.T) {
	objects := []TextObject{{glyphs: []GlyphObject{
		newTestGlyph(contentdata.GlyphSpec{Text: "A", Origin: [2]float64{0, 100}, FontSize: 10, BBox: [4]float64{0, 98, 5, 108}}, nil, nil, geometry.Matrix{}),
		newTestGlyph(contentdata.GlyphSpec{Text: "B", Origin: [2]float64{0, 80}, FontSize: 10, BBox: [4]float64{0, 78, 5, 88}}, nil, nil, geometry.Matrix{}),
	}}}
	got := AnalyzeLayout(objects, layout.Options{LineMargin: 0.5})
	if paragraphs := got.ParagraphsCopy(); len(paragraphs) != 2 {
		t.Fatalf("paragraphs with explicit gap = %#v", paragraphs)
	}
}

func TestLayoutParagraphIndexMatchesQuadraticReference(t *testing.T) {
	random := rand.New(rand.NewSource(0x50415241))
	gaps := []float64{-0.25, 0, 0.1, 0.5, 1, 2}
	for iteration := 0; iteration < 300; iteration++ {
		lineCount := 1 + random.Intn(80)
		lines := make([]TextLine, lineCount)
		for index := range lines {
			x := float64(random.Intn(21)-10) * 25
			y := float64(random.Intn(21)-10) * 25
			width := float64(1 + random.Intn(80))
			height := float64(1 + random.Intn(40))
			lines[index] = TextLine{data: layout.NewComponent(layout.ComponentSpec{
				Text: strconv.Itoa(index), BBox: [4]float64{x, y, x + width, y + height}, Vertical: random.Intn(4) == 0,
			})}
		}
		if iteration%50 == 0 {
			// Exercise the source-order fallback without enumerating an enormous
			// 50-point grid.
			lines[0].data = layout.NewComponent(layout.ComponentSpec{
				Text: "0", BBox: [4]float64{-1e18, -1e18, 1e18, 1e18}, Vertical: iteration%100 == 0,
			})
		}
		gap := gaps[random.Intn(len(gaps))]
		got := paragraphMemberIndexes(layoutParagraphs(lines, gap))
		want := quadraticParagraphMemberIndexes(lines, gap)
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("iteration %d gap %v paragraph members = %#v, want %#v", iteration, gap, got, want)
		}
	}
}

func paragraphMemberIndexes(paragraphs []TextParagraph) [][]int {
	groups := make([][]int, len(paragraphs))
	for groupIndex := range paragraphs {
		for lineIndex := range paragraphs[groupIndex].lines {
			index, _ := strconv.Atoi(paragraphs[groupIndex].lines[lineIndex].data.Text())
			groups[groupIndex] = append(groups[groupIndex], index)
		}
	}
	return groups
}

func quadraticParagraphMemberIndexes(lines []TextLine, paragraphGap float64) [][]int {
	parents := make([]int, len(lines))
	for index := range parents {
		parents[index] = index
	}
	var find func(int) int
	find = func(index int) int {
		if parents[index] != index {
			parents[index] = find(parents[index])
		}
		return parents[index]
	}
	for i := 0; i < len(lines); i++ {
		for j := i + 1; j < len(lines); j++ {
			if lines[i].Vertical() != lines[j].Vertical() {
				continue
			}
			bbox := lines[i].BBox()
			height := bbox[3] - bbox[1]
			if lines[i].Vertical() {
				height = bbox[2] - bbox[0]
			}
			if height <= 0 {
				continue
			}
			tolerance := paragraphGap * height
			if layoutLineDistance(lines[i], lines[j]) > tolerance || !layoutLinesNeighbor(lines[i], lines[j], tolerance) {
				continue
			}
			left, right := find(i), find(j)
			if left != right {
				parents[right] = left
			}
		}
	}
	groups := make(map[int][]int, len(lines))
	order := make([]int, 0, len(lines))
	for index := range lines {
		root := find(index)
		if _, exists := groups[root]; !exists {
			order = append(order, root)
		}
		groups[root] = append(groups[root], index)
	}
	out := make([][]int, 0, len(order))
	for _, root := range order {
		out = append(out, groups[root])
	}
	return out
}

func TestAnalyzeLayoutSplitsFarApartGlyphsWithSameBaseline(t *testing.T) {
	objects := []TextObject{{glyphs: []GlyphObject{
		newTestGlyph(contentdata.GlyphSpec{Text: "A", Origin: [2]float64{0, 10}, BBox: [4]float64{0, 0, 5, 10}}, nil, nil, geometry.Matrix{}),
		newTestGlyph(contentdata.GlyphSpec{Text: "B", Origin: [2]float64{100, 10}, BBox: [4]float64{100, 0, 105, 10}}, nil, nil, geometry.Matrix{}),
	}}}
	got := AnalyzeLayout(objects, layout.Options{})
	if lines := got.LinesCopy(); len(lines) != 2 {
		t.Fatalf("same-baseline distant glyphs = %#v", lines)
	}
}

func TestAnalyzeLayoutDoesNotBackfillInterruptedSourceLines(t *testing.T) {
	objects := []TextObject{{glyphs: []GlyphObject{
		newTestGlyph(contentdata.GlyphSpec{Text: "A", Origin: [2]float64{0, 100}, BBox: [4]float64{0, 90, 5, 100}}, nil, nil, geometry.Matrix{}),
		newTestGlyph(contentdata.GlyphSpec{Text: "B", Origin: [2]float64{0, 80}, BBox: [4]float64{0, 70, 5, 80}}, nil, nil, geometry.Matrix{}),
		newTestGlyph(contentdata.GlyphSpec{Text: "C", Origin: [2]float64{6, 100}, BBox: [4]float64{6, 90, 11, 100}}, nil, nil, geometry.Matrix{}),
	}}}
	lines := AnalyzeLayout(objects, layout.Options{}).LinesCopy()
	if len(lines) != 3 || lines[0].Text() != "A" || lines[1].Text() != "C" || lines[2].Text() != "B" {
		t.Fatalf("interrupted source lines were backfilled = %#v", lines)
	}
	for _, line := range lines {
		if len(line.GlyphsCopy()) != 1 {
			t.Fatalf("interrupted source line was merged = %#v", lines)
		}
	}
}

func TestAnalyzeLayoutUsesStrictCharMarginBoundary(t *testing.T) {
	objects := []TextObject{{glyphs: []GlyphObject{
		newTestGlyph(contentdata.GlyphSpec{Text: "A", Origin: [2]float64{0, 10}, FontSize: 10, BBox: [4]float64{0, 0, 5, 10}}, nil, nil, geometry.Matrix{}),
		newTestGlyph(contentdata.GlyphSpec{Text: "B", Origin: [2]float64{15, 10}, FontSize: 10, BBox: [4]float64{15, 0, 20, 10}}, nil, nil, geometry.Matrix{}),
	}}}
	if lines := AnalyzeLayout(objects, layout.Options{}).LinesCopy(); len(lines) != 2 {
		t.Fatalf("exact char margin was merged = %#v", lines)
	}
}

func TestAnalyzeLayoutUsesGlyphOverlapForLineGrouping(t *testing.T) {
	objects := []TextObject{{glyphs: []GlyphObject{
		newTestGlyph(contentdata.GlyphSpec{Text: "A", Origin: [2]float64{0, 10}, FontSize: 10, BBox: [4]float64{0, 0, 5, 10}}, nil, nil, geometry.Matrix{}),
		newTestGlyph(contentdata.GlyphSpec{Text: "B", Origin: [2]float64{5, 13}, FontSize: 10, BBox: [4]float64{5, 3, 10, 13}}, nil, nil, geometry.Matrix{}),
	}}}
	got := AnalyzeLayout(objects, layout.Options{})
	if lines := got.LinesCopy(); len(lines) != 1 || lines[0].Text() != "AB" || len(lines[0].WordsCopy()) != 1 {
		t.Fatalf("overlapping glyphs were split into lines = %#v", lines)
	}
}

func TestAnalyzeLayoutUsesStrictLineOverlapBoundary(t *testing.T) {
	objects := []TextObject{{glyphs: []GlyphObject{
		newTestGlyph(contentdata.GlyphSpec{Text: "A", Origin: [2]float64{0, 10}, FontSize: 10, BBox: [4]float64{0, 0, 5, 10}}, nil, nil, geometry.Matrix{}),
		newTestGlyph(contentdata.GlyphSpec{Text: "B", Origin: [2]float64{5, 15}, FontSize: 10, BBox: [4]float64{5, 5, 10, 15}}, nil, nil, geometry.Matrix{}),
	}}}
	// Playa requires overlap to be strictly greater than the configured
	// fraction of the smaller glyph height. Equality must split the lines.
	if lines := AnalyzeLayout(objects, layout.Options{}).LinesCopy(); len(lines) != 2 {
		t.Fatalf("exact line-overlap boundary was merged = %#v", lines)
	}
}

func TestAnalyzeLayoutDetectsVerticalTextOnlyWhenEnabled(t *testing.T) {
	objects := []TextObject{{glyphs: []GlyphObject{
		newTestGlyph(contentdata.GlyphSpec{Text: "A", Vertical: true, Origin: [2]float64{10, 20}, FontSize: 10, BBox: [4]float64{5, 15, 15, 25}}, nil, nil, geometry.Matrix{}),
		newTestGlyph(contentdata.GlyphSpec{Text: "B", Vertical: true, Origin: [2]float64{10, 10}, FontSize: 10, BBox: [4]float64{5, 5, 15, 15}}, nil, nil, geometry.Matrix{}),
	}}}
	withoutVertical := AnalyzeLayout(objects, layout.Options{}).LinesCopy()
	if len(withoutVertical) != 2 || withoutVertical[0].Vertical() || withoutVertical[1].Vertical() {
		t.Fatalf("default vertical analysis = %#v", withoutVertical)
	}
	withVertical := AnalyzeLayout(objects, layout.Options{DetectVertical: true}).LinesCopy()
	if len(withVertical) != 1 || !withVertical[0].Vertical() || withVertical[0].Text() != "AB" {
		t.Fatalf("enabled vertical analysis = %#v", withVertical)
	}
	if withVertical[0].WritingMode() != "tb-rl" {
		t.Fatalf("vertical writing mode = %q", withVertical[0].WritingMode())
	}
	if withoutVertical[0].WritingMode() != "lr-tb" {
		t.Fatalf("horizontal writing mode = %q", withoutVertical[0].WritingMode())
	}
}

func TestAnalyzeLayoutPrefersHorizontalWhenBothAlignmentsMatch(t *testing.T) {
	objects := []TextObject{{glyphs: []GlyphObject{
		newTestGlyph(contentdata.GlyphSpec{Text: "A", Vertical: true, Origin: [2]float64{10, 20}, BBox: [4]float64{5, 15, 15, 25}}, nil, nil, geometry.Matrix{}),
		newTestGlyph(contentdata.GlyphSpec{Text: "B", Vertical: true, Origin: [2]float64{10, 16}, BBox: [4]float64{5, 11, 15, 21}}, nil, nil, geometry.Matrix{}),
	}}}
	lines := AnalyzeLayout(objects, layout.Options{DetectVertical: true}).LinesCopy()
	if len(lines) != 2 || lines[0].Vertical() || lines[1].Vertical() || lines[0].WritingMode() != "lr-tb" {
		t.Fatalf("ambiguous line alignment selected vertical mode = %#v", lines)
	}
}

func TestAnalyzeLayoutUsesHorizontalFallbackForSingleVerticalGlyph(t *testing.T) {
	objects := []TextObject{{glyphs: []GlyphObject{
		newTestGlyph(contentdata.GlyphSpec{Text: "A", Vertical: true, Origin: [2]float64{10, 20}, FontSize: 10, BBox: [4]float64{5, 15, 15, 25}}, nil, nil, geometry.Matrix{}),
	}}}
	lines := AnalyzeLayout(objects, layout.Options{DetectVertical: true}).LinesCopy()
	if len(lines) != 1 || lines[0].Vertical() || lines[0].WritingMode() != "lr-tb" {
		t.Fatalf("single vertical glyph fallback = %#v", lines)
	}
}

func TestLayoutSequencesAreRepeatableAndStopEarly(t *testing.T) {
	result := AnalyzeLayout([]TextObject{{glyphs: []GlyphObject{
		newTestGlyph(contentdata.GlyphSpec{Text: "A", Origin: [2]float64{0, 10}, FontSize: 10, BBox: [4]float64{0, 0, 5, 10}}, nil, nil, geometry.Matrix{}),
		newTestGlyph(contentdata.GlyphSpec{Text: "B", Origin: [2]float64{6, 10}, FontSize: 10, BBox: [4]float64{6, 0, 11, 10}}, nil, nil, geometry.Matrix{}),
	}}}, layout.Options{})
	var first []string
	for line := range result.LinesSeq() {
		first = append(first, line.Text())
		break
	}
	var second []string
	for line := range result.LinesSeq() {
		second = append(second, line.Text())
	}
	if len(first) != 1 || len(second) != 1 || first[0] != second[0] {
		t.Fatalf("layout line sequences = %#v, %#v", first, second)
	}
	line := result.LinesCopy()[0]
	var glyphs []string
	for glyph := range line.GlyphsSeq() {
		glyphs = append(glyphs, glyph.Text())
	}
	if len(glyphs) != 2 || glyphs[0] != "A" || glyphs[1] != "B" {
		t.Fatalf("line glyph sequence = %#v", glyphs)
	}
}

func TestTextGroupBorrowedAccessDefersDeepCopy(t *testing.T) {
	font := &Font{}
	box := TextBox{lines: []TextLine{{glyphs: []GlyphObject{newTestGlyphWithFont(font)}}}}
	child := TextGroupChild{box: &box}

	borrowed, ok := child.BoxBorrowed()
	if !ok || len(borrowed.lines) != 1 || borrowed.lines[0].glyphs[0].font != font {
		t.Fatal("borrowed textbox access copied its font")
	}
	owned, ok := child.BoxCopy()
	if !ok || len(owned.lines) != 1 || owned.lines[0].glyphs[0].font == font {
		t.Fatal("textbox copy did not finalize its font")
	}

	group := TextGroup{children: []TextGroupChild{child}}
	groupChild := TextGroupChild{group: &group}
	borrowedGroup, ok := groupChild.GroupBorrowed()
	if !ok || len(borrowedGroup.children) != 1 || borrowedGroup.children[0].box.lines[0].glyphs[0].font != font {
		t.Fatal("borrowed text-group access copied its font")
	}
	ownedGroup, ok := groupChild.GroupCopy()
	if !ok || len(ownedGroup.children) != 1 || ownedGroup.children[0].box.lines[0].glyphs[0].font == font {
		t.Fatal("text-group copy did not finalize its font")
	}
}

func TestOrderedTextBoxesKeepsBorrowedGlyphs(t *testing.T) {
	font := &Font{}
	box := TextBox{lines: []TextLine{{glyphs: []GlyphObject{newTestGlyphWithFont(font)}}}}
	group := TextGroup{children: []TextGroupChild{{box: &box}}}

	ordered := orderedTextBoxes([]TextGroup{group})
	if len(ordered) != 1 || len(ordered[0].lines) != 1 || ordered[0].lines[0].glyphs[0].font != font {
		t.Fatal("ordered textbox projection eagerly finalized glyph fonts")
	}
}

func TestAnalyzeLayoutExposesTextBoxesWithStableOwnership(t *testing.T) {
	result := AnalyzeLayout([]TextObject{{glyphs: []GlyphObject{
		newTestGlyph(contentdata.GlyphSpec{Text: "A", Origin: [2]float64{0, 100}, BBox: [4]float64{0, 90, 5, 100}}, nil, nil, geometry.Matrix{}),
		newTestGlyph(contentdata.GlyphSpec{Text: "B", Origin: [2]float64{0, 85.1}, BBox: [4]float64{0, 75.1, 5, 85.1}}, nil, nil, geometry.Matrix{}),
	}}}, layout.Options{})
	boxes := result.TextBoxesCopy()
	if len(boxes) != 1 || boxes[0].Text() != "A\nB" || len(boxes[0].LinesCopy()) != 2 {
		t.Fatalf("text boxes = %#v", boxes)
	}
	var seen []string
	for box := range result.TextBoxesSeq() {
		seen = append(seen, box.Text())
	}
	if len(seen) != 1 || seen[0] != boxes[0].Text() {
		t.Fatalf("text box sequence = %#v", seen)
	}
	final := result.Finalize()
	if got := final.TextBoxesCopy(); len(got) != 1 || len(got[0].LinesCopy()) != 2 {
		t.Fatalf("finalized text boxes = %#v", got)
	}
}

func TestPageLayoutCombinesTextExtractionAndMining(t *testing.T) {
	d := openFixture(t, testfixture.Path(t, "acceptance_cjk_cid.pdf"))
	p, err := d.PageAt(0)
	if err != nil {
		t.Fatal(err)
	}
	result, err := p.Layout(d, layout.Options{})
	if err != nil {
		t.Fatal(err)
	}
	lines := result.LinesCopy()
	if len(lines) != 1 || lines[0].Text() != "中国" {
		t.Fatalf("page layout lines = %#v", lines)
	}
}

func TestPageLayoutAllTextsIncludesFormXObjects(t *testing.T) {
	form := newStream(Dict{
		Name("Subtype"): Name("Form"),
		Name("Resources"): Dict{
			Name("Font"): Dict{Name("F"): Dict{Name("Subtype"): Name("Type1"), Name("BaseFont"): Name("Helvetica")}},
		},
	}, []byte("BT /F 10 Tf (form) Tj ET"))
	d := &Document{}
	p := Page{dict: Dict{
		Name("Contents"):  newStream(nil, []byte("/Fm Do")),
		Name("Resources"): Dict{Name("XObject"): Dict{Name("Fm"): form}},
	}}
	if result, err := p.Layout(d, layout.Options{}); err != nil {
		t.Fatal(err)
	} else if lines := result.LinesCopy(); len(lines) != 0 {
		t.Fatalf("default layout unexpectedly included Form text: %#v", lines)
	}
	result, err := p.Layout(d, layout.Options{AllTexts: true})
	if err != nil {
		t.Fatal(err)
	}
	lines := result.LinesCopy()
	if len(lines) != 1 || lines[0].Text() != "form" {
		t.Fatalf("all_texts layout = %#v", lines)
	}
}

func TestPageLayoutItemsIncludeNonTextContentInPlayaOrder(t *testing.T) {
	d := openFixture(t, testfixture.Path(t, "acceptance_inline_image.pdf"))
	p, err := d.PageAt(0)
	if err != nil {
		t.Fatal(err)
	}
	result, err := p.Layout(d, layout.Options{})
	if err != nil {
		t.Fatal(err)
	}
	items := result.ItemsCopy()
	if len(items) != 3 {
		t.Fatalf("layout items = %#v, want one text box and two images", items)
	}
	if items[0].Kind() != LayoutTextBox || items[0].TextBoxCopy() == nil {
		t.Fatalf("first layout item = %#v, want text box", items[0])
	}
	for index, item := range items[1:] {
		if item.Kind() != LayoutImage {
			t.Fatalf("layout item %d = %#v, want image", index+1, item)
		}
		content := item.ContentCopy()
		if content == nil || content.Kind() != ContentImage || content.ImageCopy() == nil {
			t.Fatalf("layout image item %d = %#v", index+1, item)
		}
	}
}

func TestPageLayoutItemsIncludeDirectFormXObject(t *testing.T) {
	d := openFixture(t, testfixture.Path(t, "acceptance_form_xobject.pdf"))
	p, err := d.PageAt(0)
	if err != nil {
		t.Fatal(err)
	}
	result, err := p.Layout(d, layout.Options{})
	if err != nil {
		t.Fatal(err)
	}
	items := result.ItemsCopy()
	if len(items) != 1 || items[0].Kind() != LayoutXObject {
		t.Fatalf("layout items = %#v, want one Form XObject", items)
	}
	content := items[0].ContentCopy()
	if content == nil || content.Kind() != ContentXObject || content.XObjectCopy() == nil {
		t.Fatalf("layout Form XObject item = %#v", items[0])
	}
}

func TestLayoutPathSubpathsMatchPlayaItemBoundaries(t *testing.T) {
	path := newPathObject(contentdata.PathSpec{
		RawSegments: []geometry.PathSegment{
			geometry.NewPathSegment("m", [2]float64{0, 0}),
			geometry.NewPathSegment("l", [2]float64{10, 0}),
			geometry.NewPathSegment("m", [2]float64{20, 0}),
			geometry.NewPathSegment("l", [2]float64{30, 0}),
		},
		Segments: []geometry.PathSegment{
			geometry.NewPathSegment("m", [2]float64{0, 0}),
			geometry.NewPathSegment("l", [2]float64{10, 0}),
			geometry.NewPathSegment("m", [2]float64{20, 0}),
			geometry.NewPathSegment("l", [2]float64{30, 0}),
		},
		BBox: [4]float64{0, 0, 30, 0},
	})
	subpaths := splitPathSubpaths(path)
	if len(subpaths) != 2 {
		t.Fatalf("path subpaths = %#v, want two independent layout paths", subpaths)
	}
	if got := subpaths[0].SegmentsCopy(); len(got) != 2 || got[0].Operator() != "m" || got[1].Operator() != "l" {
		t.Fatalf("first subpath segments = %#v", got)
	}
	if got := subpaths[1].SegmentsCopy(); len(got) != 2 || got[0].Operator() != "m" || got[1].Operator() != "l" {
		t.Fatalf("second subpath segments = %#v", got)
	}
	if subpaths[0].BBox() != [4]float64{0, 0, 10, 0} || subpaths[1].BBox() != [4]float64{20, 0, 30, 0} {
		t.Fatalf("subpath bboxes = %#v, want independent bounds", subpaths)
	}
}

func TestPageLayoutSplitsPaintedPathSubpaths(t *testing.T) {
	d := openFixture(t, testfixture.Path(t, "acceptance_path_subpaths.pdf"))
	p, err := d.PageAt(0)
	if err != nil {
		t.Fatal(err)
	}
	result, err := p.Layout(d, layout.Options{})
	if err != nil {
		t.Fatal(err)
	}
	items := result.ItemsCopy()
	if len(items) != 4 {
		t.Fatalf("layout items = %#v, want one text box and three paths", items)
	}
	want := [][4]float64{{72, 212, 172, 242}, {222, 212, 322, 242}, {372, 222, 392, 242}}
	for index, bbox := range want {
		item := items[index+1]
		if item.Kind() != LayoutPath {
			t.Fatalf("layout item %d kind = %q, want path", index+1, item.Kind())
		}
		content := item.ContentCopy()
		path := (*PathObject)(nil)
		if content != nil {
			path = content.PathCopy()
		}
		if path == nil || path.BBox() != bbox {
			var got [4]float64
			if path != nil {
				got = path.BBox()
			}
			t.Fatalf("layout path %d bbox = %v, want bbox %v", index, got, bbox)
		}
	}
}

func TestLayoutItemsAreRepeatableAndOwned(t *testing.T) {
	d := openFixture(t, testfixture.Path(t, "acceptance_inline_image.pdf"))
	p, err := d.PageAt(0)
	if err != nil {
		t.Fatal(err)
	}
	result, err := p.Layout(d, layout.Options{})
	if err != nil {
		t.Fatal(err)
	}
	var first []LayoutItemKind
	for item := range result.ItemsSeq() {
		first = append(first, item.Kind())
		break
	}
	var second []LayoutItemKind
	for item := range result.ItemsSeq() {
		second = append(second, item.Kind())
	}
	if len(first) != 1 || len(second) != 3 || first[0] != second[0] {
		t.Fatalf("layout item sequences = %v, %v", first, second)
	}
	copyItems := result.ItemsCopy()
	if copyItems[0].TextBoxCopy() == nil {
		t.Fatal("copied text-box item has no text box")
	}
	box := copyItems[0].TextBoxCopy()
	boxText := box.Text()
	if boxes := result.TextBoxesCopy(); len(boxes) != 1 || boxes[0].Text() != boxText {
		t.Fatalf("layout text-box copy changed result = %#v", boxes)
	}
	content := copyItems[1].ContentCopy()
	if content == nil {
		t.Fatal("copied image item has no content")
	}
	image := content.ImageCopy()
	if image == nil {
		t.Fatal("copied image item has no image payload")
	}
	data := image.Buffer()
	if len(data) > 0 {
		data[0] ^= 0xff
	}
	original := result.ItemsCopy()[1].ContentCopy().ImageCopy().Buffer()
	if len(original) > 0 && data[0] == original[0] {
		t.Fatal("image copy shares mutable buffer")
	}
}

func TestPageLayoutRejectsNilDocument(t *testing.T) {
	var page Page
	if _, err := page.Layout(nil, layout.Options{}); err != errNilDocument {
		t.Fatalf("nil document layout error = %v, want %v", err, errNilDocument)
	}
}
