package document

import (
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/lin-string/go-playa/cacheconfig"
	"github.com/lin-string/go-playa/documentdata"
)

func TestOutlineGoToIndirectDestinationMatchesPlaya(t *testing.T) {
	d := &Document{objects: map[Ref]Object{
		{Object: 10}: Array{Ref{Object: 20}, Name("Fit")},
	}}
	node := OutlineNode{
		document: d,
		data: documentdata.NewOutlineNode(documentdata.OutlineNodeSpec{
			ActionKind: "GoTo",
			Action:     Dict{Name("D"): Ref{Object: 10}},
		}),
	}
	if target := node.TargetCopy(); target != nil {
		t.Fatalf("outline indirect action destination = %#v, want nil", target)
	}
}

func TestOutlinePathClonePreservesEmptyAllocatedMap(t *testing.T) {
	source := map[Ref]bool{}
	clone := cloneOutlinePath(source)
	if clone == nil {
		t.Fatal("empty allocated outline path was collapsed to nil")
	}
}

func TestOutlineEmpty(t *testing.T) {
	d, _ := OpenBytes([]byte("%PDF-1.7\n1 0 obj << /Type /Catalog >> endobj"))
	got, err := d.CollectOutline()
	if err != nil {
		t.Fatal(err)
	}
	if got != nil {
		t.Fatal(got)
	}
}

func TestConcurrentOutlineCollection(t *testing.T) {
	d := &Document{
		trailer: Dict{Name("Root"): Ref{Object: 1}},
		objects: map[Ref]Object{
			{Object: 1}: Dict{Name("Outlines"): Ref{Object: 2}},
			{Object: 2}: Dict{Name("First"): Ref{Object: 3}},
			{Object: 3}: Dict{Name("Title"): String("chapter")},
		},
	}
	const readers = 16
	var wg sync.WaitGroup
	errs := make(chan error, readers)
	start := make(chan struct{})
	for i := 0; i < readers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			outline, err := d.CollectOutline()
			if err != nil {
				errs <- err
				return
			}
			if len(outline) != 1 || outline[0].Title() != "chapter" {
				errs <- fmt.Errorf("unexpected outline: %#v", outline)
			}
		}()
	}
	close(start)
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatal(err)
	}
}

func TestOutlinePreservesCountAndAction(t *testing.T) {
	d := &Document{
		trailer: Dict{Name("Root"): Ref{Object: 1, Generation: 0}},
		objects: map[Ref]Object{
			{Object: 1, Generation: 0}: Dict{Name("Outlines"): Ref{Object: 2, Generation: 0}},
			{Object: 2, Generation: 0}: Dict{Name("First"): Ref{Object: 3, Generation: 0}},
			{Object: 3, Generation: 0}: Dict{Name("Title"): String([]byte("Chapter")), Name("Count"): Number(2), Name("A"): Ref{Object: 4, Generation: 0}},
			{Object: 4, Generation: 0}: Dict{Name("S"): Name("URI"), Name("URI"): String([]byte("https://example.com"))},
		},
	}
	got, err := d.CollectOutline()
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Title() != "Chapter" || !got[0].HasCount() || got[0].Count() != 2 || got[0].ActionCopy()[Name("S")] != Name("URI") {
		t.Fatalf("unexpected outline: %#v", got)
	}
}

func TestOutlineFollowsMultiLevelIndirectNodeValues(t *testing.T) {
	d := &Document{trailer: Dict{Name("Root"): Ref{Object: 1}}, objects: map[Ref]Object{}}
	d.objects[Ref{Object: 1}] = Dict{Name("Outlines"): Ref{Object: 2}}
	d.objects[Ref{Object: 2}] = Dict{Name("First"): Ref{Object: 3}}
	d.objects[Ref{Object: 3}] = Ref{Object: 4}
	d.objects[Ref{Object: 4}] = Dict{
		Name("Title"): Ref{Object: 5},
		Name("Count"): Ref{Object: 6},
		Name("A"):     Ref{Object: 7},
		Name("Next"):  Ref{Object: 8},
	}
	d.objects[Ref{Object: 5}] = String("Chapter")
	d.objects[Ref{Object: 6}] = Number(2)
	d.objects[Ref{Object: 7}] = Ref{Object: 9}
	d.objects[Ref{Object: 8}] = Ref{Object: 10}
	d.objects[Ref{Object: 9}] = Dict{Name("S"): Ref{Object: 11}, Name("URI"): String("https://example.com")}
	d.objects[Ref{Object: 10}] = Dict{Name("Title"): String("Next")}
	d.objects[Ref{Object: 11}] = Name("URI")

	outline, err := d.CollectOutline()
	if err != nil {
		t.Fatal(err)
	}
	if len(outline) != 2 || outline[0].Title() != "Chapter" || !outline[0].HasCount() || outline[0].ActionKind() != "URI" || outline[1].Title() != "Next" {
		t.Fatalf("outline = %#v", outline)
	}
}

func TestOutlineReusesCacheAcrossIndirectReferenceChains(t *testing.T) {
	d := &Document{trailer: Dict{Name("Root"): Ref{Object: 1}}, objects: map[Ref]Object{
		{Object: 1}: Dict{Name("Outlines"): Ref{Object: 2}},
		{Object: 2}: Dict{Name("First"): Ref{Object: 3}},
		{Object: 3}: Ref{Object: 4},
		{Object: 4}: Dict{Name("Title"): String("shared"), Name("Next"): Ref{Object: 5}},
		{Object: 5}: Ref{Object: 4},
	}}
	var nodes []OutlineNode
	for node, err := range d.Outline() {
		if err != nil {
			t.Fatal(err)
		}
		nodes = append(nodes, node)
	}
	if len(nodes) != 1 || nodes[0].Title() != "shared" {
		t.Fatalf("shared outline nodes = %#v", nodes)
	}
	if len(d.outlineCache) != 1 {
		t.Fatalf("outline cache size = %d, want 1", len(d.outlineCache))
	}
}

func TestOutlineFollowsMultiLevelIndirectRelationRefs(t *testing.T) {
	d := &Document{trailer: Dict{Name("Root"): Ref{Object: 1}}, objects: map[Ref]Object{
		{Object: 1}: Dict{Name("Outlines"): Ref{Object: 2}},
		{Object: 2}: Dict{Name("First"): Ref{Object: 3}},
		{Object: 3}: Dict{Name("Title"): String("Child"), Name("Parent"): Ref{Object: 4}, Name("SE"): Ref{Object: 6}},
		{Object: 4}: Ref{Object: 5},
		{Object: 5}: Ref{Object: 2},
		{Object: 6}: Ref{Object: 7},
		{Object: 7}: Dict{Name("Type"): Name("StructElem")},
	}}
	outlines, err := d.CollectOutline()
	if err != nil {
		t.Fatal(err)
	}
	if len(outlines) != 1 || !outlines[0].HasParent() || outlines[0].Parent() != (Ref{Object: 2}) || !outlines[0].HasElement() || outlines[0].ElementRef() != (Ref{Object: 7}) {
		t.Fatalf("indirect outline relations = %#v", outlines)
	}
}

func TestOutlineGoToTargetIgnoresIndirectActionDestination(t *testing.T) {
	d := &Document{trailer: Dict{Name("Root"): Ref{Object: 1}}, objects: map[Ref]Object{
		{Object: 1}: Dict{Name("Outlines"): Ref{Object: 2}},
		{Object: 2}: Dict{Name("First"): Ref{Object: 3}},
		{Object: 3}: Dict{Name("Title"): String("chapter"), Name("A"): Ref{Object: 4}},
		{Object: 4}: Dict{Name("S"): Name("GoTo"), Name("D"): Ref{Object: 5}},
		{Object: 5}: Array{Ref{Object: 9}, Name("Fit")},
	}}

	nodes, err := d.CollectOutline()
	if err != nil || len(nodes) != 1 {
		t.Fatalf("outline = %#v, err = %v", nodes, err)
	}
	target, err := nodes[0].TargetCopyWithError()
	if err != nil || target != nil {
		t.Fatalf("outline target = %#v, err = %v", target, err)
	}
}

func TestOutlineGoToTargetWithErrorIgnoresMalformedIndirectDestination(t *testing.T) {
	d := &Document{trailer: Dict{Name("Root"): Ref{Object: 1}}, objects: map[Ref]Object{
		{Object: 1}: Dict{Name("Outlines"): Ref{Object: 2}},
		{Object: 2}: Dict{Name("First"): Ref{Object: 3}},
		{Object: 3}: Dict{Name("Title"): String("chapter"), Name("A"): Ref{Object: 4}},
		{Object: 4}: Dict{Name("S"): Name("GoTo"), Name("D"): Ref{Object: 99}},
	}}

	nodes, err := d.CollectOutline()
	if err != nil || len(nodes) != 1 {
		t.Fatalf("outline = %#v, err = %v", nodes, err)
	}
	if target, err := nodes[0].TargetCopyWithError(); err != nil || target != nil {
		t.Fatalf("malformed outline target = %#v, err = %v", target, err)
	}
}

func TestOutlineExposesActionKindAndResolvedDestination(t *testing.T) {
	d := &Document{
		trailer: Dict{Name("Root"): Ref{Object: 1, Generation: 0}},
		objects: map[Ref]Object{
			{Object: 1, Generation: 0}: Dict{Name("Outlines"): Ref{Object: 2, Generation: 0}},
			{Object: 2, Generation: 0}: Dict{Name("First"): Ref{Object: 3, Generation: 0}},
			{Object: 3, Generation: 0}: Dict{Name("Title"): String([]byte("Chapter")), Name("A"): Dict{Name("S"): Name("GoTo"), Name("D"): Array{Ref{Object: 9, Generation: 0}, Name("Fit")}}},
		},
	}
	got, err := d.CollectOutline()
	if err != nil {
		t.Fatal(err)
	}
	target := got[0].TargetCopy()
	page, hasPage := target.PageRef()
	if len(got) != 1 || got[0].ActionKind() != "GoTo" || target == nil || !hasPage || page != (Ref{Object: 9, Generation: 0}) || target.View() != "Fit" {
		t.Fatalf("outline = %#v", got)
	}
}

func TestOutlineTargetCopyWithErrorReportsMalformedDestination(t *testing.T) {
	node := newOutlineValue(documentdata.OutlineNodeSpec{Dest: Array{Number(1), Name("FitH")}})
	node.document = &Document{}
	if target, err := node.TargetCopyWithError(); err == nil || target != nil {
		t.Fatalf("outline target = %#v, err = %v", target, err)
	}
	if target := node.TargetCopy(); target != nil {
		t.Fatalf("compatibility outline target = %#v", target)
	}
}

func TestOutlineActionValueCopyReportsMalformedNext(t *testing.T) {
	action := newAction("Named")
	action.document = &Document{}
	action.nextRaw = Number(1)
	node := OutlineNode{actionValue: action}
	if copy, err := node.ActionValueCopyWithError(); err == nil || copy != nil {
		t.Fatalf("malformed outline action copy = %#v, err=%v", copy, err)
	}
	if snapshot, err := node.FinalizeWithError(); err == nil || snapshot.Title() != "" {
		t.Fatalf("malformed outline finalize = %#v, err=%v", snapshot, err)
	}
}

func TestOutlineDoesNotResolveDestinationFromNonGoToAction(t *testing.T) {
	d := &Document{
		trailer: Dict{Name("Root"): Ref{Object: 1, Generation: 0}},
		objects: map[Ref]Object{
			{Object: 1, Generation: 0}: Dict{Name("Outlines"): Ref{Object: 2, Generation: 0}},
			{Object: 2, Generation: 0}: Dict{Name("First"): Ref{Object: 3, Generation: 0}},
			{Object: 3, Generation: 0}: Dict{Name("Title"): String([]byte("Chapter")), Name("A"): Dict{Name("S"): Name("URI"), Name("D"): Array{Number(1), Name("Fit")}}},
		},
	}
	got, err := d.CollectOutline()
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].ActionKind() != "URI" || got[0].target != nil {
		t.Fatalf("outline = %#v", got)
	}
}

func TestOutlineDoesNotResolveIndirectActionDestination(t *testing.T) {
	d := &Document{
		trailer: Dict{Name("Root"): Ref{Object: 1, Generation: 0}},
		objects: map[Ref]Object{
			{Object: 1, Generation: 0}: Dict{Name("Outlines"): Ref{Object: 2, Generation: 0}},
			{Object: 2, Generation: 0}: Dict{Name("First"): Ref{Object: 3, Generation: 0}},
			{Object: 3, Generation: 0}: Dict{Name("Title"): String([]byte("Chapter")), Name("A"): Dict{Name("S"): Name("GoTo"), Name("D"): Ref{Object: 4, Generation: 0}}},
			{Object: 4, Generation: 0}: Array{Number(1), Name("Fit")},
		},
	}
	got, err := d.CollectOutline()
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].ActionKind() != "GoTo" || got[0].target != nil {
		t.Fatalf("outline = %#v", got)
	}
}

func TestOutlineSequenceIsOrderedRepeatableAndStopsEarly(t *testing.T) {
	d := &Document{
		trailer: Dict{Name("Root"): Ref{Object: 1, Generation: 0}},
		objects: map[Ref]Object{
			{Object: 1, Generation: 0}: Dict{Name("Outlines"): Ref{Object: 2, Generation: 0}},
			{Object: 2, Generation: 0}: Dict{Name("First"): Ref{Object: 3, Generation: 0}},
			{Object: 3, Generation: 0}: Dict{Name("Title"): String([]byte("first")), Name("Next"): Ref{Object: 4, Generation: 0}},
			{Object: 4, Generation: 0}: Dict{Name("Title"): String([]byte("second"))},
		},
	}

	first := []string{}
	for node, err := range d.Outline() {
		if err != nil {
			t.Fatal(err)
		}
		first = append(first, node.Title())
		break
	}
	if len(first) != 1 || first[0] != "first" {
		t.Fatalf("early outline = %v, want [first]", first)
	}

	second, err := d.CollectOutline()
	if err != nil {
		t.Fatal(err)
	}
	if len(second) != 2 || second[0].Title() != "first" || second[1].Title() != "second" {
		t.Fatalf("second traversal = %#v", second)
	}
	if len(d.outlineCache) != 2 {
		t.Fatalf("outline cache size = %d, want 2", len(d.outlineCache))
	}
}

func TestOutlineSequenceReportsMalformedNodes(t *testing.T) {
	tests := []struct {
		name    string
		outline Dict
		objects map[Ref]Object
		want    string
	}{
		{
			name:    "invalid first",
			outline: Dict{Name("First"): Ref{Object: 3, Generation: 0}},
			objects: map[Ref]Object{{Object: 3, Generation: 0}: Number(1)},
			want:    "outline node is not a dictionary",
		},
		{
			name:    "invalid next",
			outline: Dict{Name("First"): Ref{Object: 3, Generation: 0}},
			objects: map[Ref]Object{{Object: 3, Generation: 0}: Dict{Name("Next"): Ref{Object: 4, Generation: 0}}, {Object: 4, Generation: 0}: Number(1)},
			want:    "outline node is not a dictionary",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			objects := map[Ref]Object{
				{Object: 1, Generation: 0}: Dict{Name("Outlines"): Ref{Object: 2, Generation: 0}},
				{Object: 2, Generation: 0}: test.outline,
			}
			for ref, object := range test.objects {
				objects[ref] = object
			}
			d := &Document{trailer: Dict{Name("Root"): Ref{Object: 1, Generation: 0}}, objects: objects}
			for _, err := range d.Outline() {
				if err == nil {
					continue
				}
				if !strings.Contains(err.Error(), test.want) {
					t.Fatalf("error = %v, want substring %q", err, test.want)
				}
				return
			}
			t.Fatal("malformed outline produced no error")
		})
	}
}

func TestOutlineSequenceReportsMalformedCatalogRoot(t *testing.T) {
	d := &Document{trailer: Dict{Name("Root"): Dict{Name("Outlines"): Number(1)}}}
	for _, err := range d.Outline() {
		if err == nil {
			t.Fatal("malformed Outlines root produced a node")
		}
		return
	}
	t.Fatal("malformed Outlines root produced no error")
}

func TestOutlineCachesTerminalRootErrors(t *testing.T) {
	d := &Document{trailer: Dict{Name("Root"): Dict{Name("Outlines"): Number(1)}}}
	var firstErr, secondErr error
	for _, err := range d.Outline() {
		firstErr = err
		break
	}
	for _, err := range d.Outline() {
		secondErr = err
		break
	}
	if firstErr == nil || secondErr == nil {
		t.Fatalf("Outline errors = %v, %v; want both non-nil", firstErr, secondErr)
	}
	if firstErr != secondErr {
		t.Fatalf("Outline error was not cached: first=%p second=%p", firstErr, secondErr)
	}
	if d.outlineRootErr != firstErr || !d.outlineRootErrReady {
		t.Fatalf("cached Outline state = ready=%v err=%v", d.outlineRootErrReady, d.outlineRootErr)
	}
}

func TestOutlineSequenceReportsMalformedFirstEntry(t *testing.T) {
	d := &Document{trailer: Dict{Name("Root"): Dict{Name("Outlines"): Dict{Name("First"): Number(1)}}}}
	for _, err := range d.Outline() {
		if err == nil {
			t.Fatal("malformed outline First produced a node")
		}
		return
	}
	t.Fatal("malformed outline First produced no error")
}

func TestCollectOutlineReportsMalformedNestedChild(t *testing.T) {
	d := &Document{
		trailer: Dict{Name("Root"): Ref{Object: 1}},
		objects: map[Ref]Object{
			{Object: 1}: Dict{Name("Outlines"): Ref{Object: 2}},
			{Object: 2}: Dict{Name("First"): Ref{Object: 3}},
			{Object: 3}: Dict{Name("Title"): String("root"), Name("First"): Ref{Object: 4}},
			{Object: 4}: Number(1),
		},
	}
	if got, err := d.CollectOutline(); err == nil || got != nil {
		t.Fatalf("nested malformed outline = %#v, err = %v", got, err)
	}
}

func TestOutlineChildrenStopAtAncestorCycle(t *testing.T) {
	d := &Document{
		trailer: Dict{Name("Root"): Ref{Object: 1}},
		objects: map[Ref]Object{
			{Object: 1}: Dict{Name("Outlines"): Ref{Object: 2}},
			{Object: 2}: Dict{Name("First"): Ref{Object: 3}},
			{Object: 3}: Dict{Name("Title"): String("root"), Name("First"): Ref{Object: 4}},
			{Object: 4}: Dict{Name("Title"): String("child"), Name("First"): Ref{Object: 3}},
		},
	}

	nodes, err := d.CollectOutline()
	if err != nil {
		t.Fatal(err)
	}
	if len(nodes) != 1 {
		t.Fatalf("top-level outline count = %d, want 1", len(nodes))
	}
	children, err := nodes[0].ChildrenCopy()
	if err != nil {
		t.Fatal(err)
	}
	if len(children) != 1 || children[0].Title() != "child" {
		t.Fatalf("first-level children = %#v", children)
	}
	grandchildren, err := children[0].ChildrenCopy()
	if err != nil {
		t.Fatal(err)
	}
	if len(grandchildren) != 0 {
		t.Fatalf("ancestor cycle was not stopped: %#v", grandchildren)
	}
}

func TestOutlineChildrenCopyReportsMalformedNestedChild(t *testing.T) {
	d := &Document{
		trailer: Dict{Name("Root"): Ref{Object: 1}},
		objects: map[Ref]Object{
			{Object: 1}: Dict{Name("Outlines"): Ref{Object: 2}},
			{Object: 2}: Dict{Name("First"): Ref{Object: 3}},
			{Object: 3}: Dict{Name("Title"): String("root"), Name("First"): Ref{Object: 4}},
			{Object: 4}: Number(1),
		},
	}
	root, err := firstOutline(d)
	if err != nil {
		t.Fatal(err)
	}
	if children, err := root.ChildrenCopy(); err == nil || children != nil {
		t.Fatalf("malformed outline children = %#v, err = %v", children, err)
	}
	if _, err := root.MarshalJSON(); err == nil {
		t.Fatal("malformed outline JSON produced no error")
	}
	if snapshot, err := root.FinalizeWithError(); err == nil || snapshot.Title() != "" {
		t.Fatalf("malformed outline finalize = %#v, err = %v", snapshot, err)
	}
}

func TestOutlineChildrenAreLazyUntilChildrenSequenceIsConsumed(t *testing.T) {
	d := &Document{
		trailer: Dict{Name("Root"): Ref{Object: 1, Generation: 0}},
		objects: map[Ref]Object{
			{Object: 1, Generation: 0}: Dict{Name("Outlines"): Ref{Object: 2, Generation: 0}},
			{Object: 2, Generation: 0}: Dict{Name("First"): Ref{Object: 3, Generation: 0}},
			{Object: 3, Generation: 0}: Dict{Name("Title"): String("root"), Name("First"): Ref{Object: 4, Generation: 0}},
			{Object: 4, Generation: 0}: Dict{Name("Title"): String("child")},
		},
	}
	node, err := firstOutline(d)
	if err != nil {
		t.Fatal(err)
	}
	if len(node.children) != 0 {
		t.Fatalf("children were eagerly materialized: %#v", node.children)
	}
	var titles []string
	for child, err := range node.ChildrenSeq() {
		if err != nil {
			t.Fatal(err)
		}
		titles = append(titles, child.Title())
	}
	if len(titles) != 1 || titles[0] != "child" {
		t.Fatalf("lazy children = %#v", titles)
	}
	node.materializeChildren()
	node.materializeChildren()
	if len(node.children) != 1 || node.children[0].Title() != "child" {
		t.Fatalf("repeated materialization = %#v", node.children)
	}
}

func TestOutlineMaterializationIsReusedByCachedNodes(t *testing.T) {
	d := &Document{
		trailer: Dict{Name("Root"): Ref{Object: 1, Generation: 0}},
		objects: map[Ref]Object{
			{Object: 1, Generation: 0}: Dict{Name("Outlines"): Ref{Object: 2, Generation: 0}},
			{Object: 2, Generation: 0}: Dict{Name("First"): Ref{Object: 3, Generation: 0}},
			{Object: 3, Generation: 0}: Dict{Name("Title"): String("root"), Name("First"): Ref{Object: 4, Generation: 0}},
			{Object: 4, Generation: 0}: Dict{Name("Title"): String("child")},
		},
	}
	first, err := d.CollectOutline()
	if err != nil || len(first) != 1 || len(first[0].children) != 1 {
		t.Fatalf("first outline = %#v, err = %v", first, err)
	}
	delete(d.objects, Ref{Object: 4, Generation: 0})
	second, err := d.CollectOutline()
	if err != nil || len(second) != 1 {
		t.Fatalf("cached outline = %#v, err = %v", second, err)
	}
	var titles []string
	for child, err := range second[0].ChildrenSeq() {
		if err != nil {
			t.Fatal(err)
		}
		titles = append(titles, child.Title())
	}
	if len(titles) != 1 || titles[0] != "child" {
		t.Fatalf("cached children = %#v", titles)
	}
}

func TestOutlineChildrenSequenceUsesCompletedCanonicalCache(t *testing.T) {
	d := &Document{
		trailer: Dict{Name("Root"): Ref{Object: 1, Generation: 0}},
		objects: map[Ref]Object{
			{Object: 1, Generation: 0}: Dict{Name("Outlines"): Ref{Object: 2, Generation: 0}},
			{Object: 2, Generation: 0}: Dict{Name("First"): Ref{Object: 3, Generation: 0}},
			{Object: 3, Generation: 0}: Dict{Name("Title"): String("root"), Name("First"): Ref{Object: 4, Generation: 0}},
			{Object: 4, Generation: 0}: Dict{Name("Title"): String("child")},
		},
	}
	node, err := firstOutline(d)
	if err != nil {
		t.Fatal(err)
	}
	materialized := node
	materialized.materializeChildren()
	if !materialized.childrenReady {
		t.Fatal("outline children were not materialized")
	}
	node.children = nil
	node.childrenReady = false
	delete(d.objects, Ref{Object: 4, Generation: 0})
	var titles []string
	for child, err := range node.ChildrenSeq() {
		if err != nil {
			t.Fatal(err)
		}
		titles = append(titles, child.Title())
	}
	if len(titles) != 1 || titles[0] != "child" {
		t.Fatalf("canonical outline children = %v", titles)
	}
}

func TestOutlineLazyChildrenAreReusedAcrossNodeCopies(t *testing.T) {
	d := &Document{
		trailer: Dict{Name("Root"): Ref{Object: 1, Generation: 0}},
		objects: map[Ref]Object{
			{Object: 1, Generation: 0}: Dict{Name("Outlines"): Ref{Object: 2, Generation: 0}},
			{Object: 2, Generation: 0}: Dict{Name("First"): Ref{Object: 3, Generation: 0}},
			{Object: 3, Generation: 0}: Dict{Name("Title"): String("root"), Name("First"): Ref{Object: 4, Generation: 0}},
			{Object: 4, Generation: 0}: Dict{Name("Title"): String("child")},
		},
	}
	node := d.outlineNode(d.objects[Ref{Object: 3, Generation: 0}].(Dict), map[Ref]bool{})
	children, err := node.ChildrenCopy()
	if err != nil || len(children) != 1 || children[0].Title() != "child" {
		t.Fatalf("first lazy children = %#v, err=%v", children, err)
	}
	d.objects[Ref{Object: 4, Generation: 0}] = Number(1)
	children, err = node.ChildrenCopy()
	if err != nil || len(children) != 1 || children[0].Title() != "child" {
		t.Fatalf("cached lazy children = %#v, err=%v", children, err)
	}
}

func TestOutlineLazyChildrenCacheTerminalErrors(t *testing.T) {
	d := &Document{objects: map[Ref]Object{{Object: 4, Generation: 0}: Number(1)}}
	node := OutlineNode{document: d, childStart: Ref{Object: 4, Generation: 0}, lazyState: &outlineLazyState{}}
	if children, err := node.ChildrenCopy(); err == nil || children != nil {
		t.Fatalf("first malformed children = %#v, err=%v", children, err)
	}
	d.objects[Ref{Object: 4, Generation: 0}] = Dict{Name("Title"): String("repaired")}
	if children, err := node.ChildrenCopy(); err == nil || children != nil {
		t.Fatalf("cached malformed children = %#v, err=%v", children, err)
	}
}

func TestOutlineCacheCopiesNestedChildren(t *testing.T) {
	d := &Document{
		trailer: Dict{Name("Root"): Ref{Object: 1}},
		objects: map[Ref]Object{
			{Object: 1}: Dict{Name("Outlines"): Ref{Object: 2}},
			{Object: 2}: Dict{Name("First"): Ref{Object: 3}},
			{Object: 3}: Dict{Name("Title"): String("root"), Name("First"): Ref{Object: 4}},
			{Object: 4}: Dict{Name("Title"): String("child")},
		},
	}
	first, err := d.CollectOutline()
	if err != nil || len(first) != 1 || len(first[0].children) != 1 {
		t.Fatalf("first outline = %#v, err = %v", first, err)
	}
	snapshot := first[0].Finalize()
	snapshot.children[0].data = documentdata.NewOutlineNode(documentdata.OutlineNodeSpec{Title: "changed"})
	second, err := d.CollectOutline()
	if err != nil || second[0].children[0].Title() != "child" {
		t.Fatalf("outline cache was exposed = %#v, err = %v", second, err)
	}
}

func TestOutlineDoesNotRetainOversizedCacheValues(t *testing.T) {
	d := &Document{
		trailer: Dict{Name("Root"): Ref{Object: 1}},
		objects: map[Ref]Object{
			{Object: 1}: Dict{Name("Outlines"): Ref{Object: 2}},
			{Object: 2}: Dict{Name("First"): Ref{Object: 3}},
			{Object: 3}: Dict{Name("Title"): String(strings.Repeat("x", outlineCacheLimit+1))},
		},
	}
	outline, err := d.CollectOutline()
	if err != nil || len(outline) != 1 || len(outline[0].Title()) != outlineCacheLimit+1 {
		t.Fatalf("outline = %d, err = %v", len(outline), err)
	}
	if len(d.outlineCache) != 0 || d.outlineCacheBytes != 0 {
		t.Fatalf("oversized outline entered cache: entries=%d bytes=%d", len(d.outlineCache), d.outlineCacheBytes)
	}
}

func TestOutlineCacheEvictionClampsInconsistentByteAccounting(t *testing.T) {
	ref := Ref{Object: 7}
	d := &Document{
		outlineCache:           map[Ref]OutlineNode{ref: newOutlineValue(documentdata.OutlineNodeSpec{Title: "cached"})},
		outlineCacheSizes:      map[Ref]int{ref: 128},
		outlineCacheBytes:      64,
		cacheOptions:           cacheconfig.Options{OutlineBytes: 128},
		cacheOptionsConfigured: true,
	}
	d.updateOutlineCache(ref, OutlineNode{})
	if d.outlineCacheBytes != 0 {
		t.Fatalf("outline cache bytes after inconsistent eviction = %d, want 0", d.outlineCacheBytes)
	}
}

func TestOutlineCloneDeepCopiesNestedChildFields(t *testing.T) {
	targetValue := documentdata.NewDestination(Ref{}, false, 0, false, "", []Object{Dict{Name("Value"): String("original")}})
	original := OutlineNode{children: []OutlineNode{newOutlineValue(documentdata.OutlineNodeSpec{
		Dest: Dict{Name("Value"): String("original")}, Target: &targetValue,
		Action: Dict{Name("Meta"): Dict{Name("Value"): String("original")}},
	})}}
	clone := cloneOutlineNode(original)
	child := &clone.children[0]
	childDest := child.DestCopy().(Dict)
	childDest[Name("Value")] = String("changed")
	childParams := child.TargetCopy().ParamsCopy()
	childParams[0].(Dict)[Name("Value")] = String("changed")
	childAction := child.ActionCopy()
	childAction[Name("Meta")].(Dict)[Name("Value")] = String("changed")
	originalChild := original.children[0]
	originalParams := originalChild.TargetCopy().ParamsCopy()
	if string(originalChild.DestCopy().(Dict)[Name("Value")].(String)) != "original" || string(originalParams[0].(Dict)[Name("Value")].(String)) != "original" || string(originalChild.ActionCopy()[Name("Meta")].(Dict)[Name("Value")].(String)) != "original" {
		t.Fatalf("nested outline fields were exposed: %#v", originalChild)
	}
}

func TestOutlineNodeResolvesParent(t *testing.T) {
	d := &Document{
		trailer: Dict{Name("Root"): Ref{Object: 1}},
		objects: map[Ref]Object{
			{Object: 1}: Dict{Name("Outlines"): Ref{Object: 2}},
			{Object: 2}: Dict{Name("First"): Ref{Object: 3}},
			{Object: 3}: Dict{Name("Title"): String([]byte("root")), Name("First"): Ref{Object: 4}},
			{Object: 4}: Dict{Name("Title"): String([]byte("child")), Name("Parent"): Ref{Object: 3}},
		},
	}
	root, err := d.CollectOutline()
	if err != nil || len(root) != 1 || len(root[0].children) != 1 {
		t.Fatalf("outline = %#v, err = %v", root, err)
	}
	if !root[0].children[0].HasParent() {
		t.Fatalf("child parent reference not preserved: %#v", root[0].children[0])
	}
	parent := root[0].children[0].ParentNode(d)
	if parent == nil || parent.Title() != "root" {
		t.Fatalf("parent = %#v", parent)
	}
}

func TestOutlineNodeResolvesStructureElement(t *testing.T) {
	d := &Document{trailer: Dict{Name("Root"): Ref{Object: 1}}, objects: map[Ref]Object{
		{Object: 9}: Dict{Name("Type"): Name("StructElem"), Name("S"): Name("P"), Name("T"): String("section")},
	}}
	node := newOutlineValue(documentdata.OutlineNodeSpec{ElementRef: Ref{Object: 9}, HasElement: true})
	element := node.Element(d)
	if element == nil || element.Role() != "P" || element.Title() != "section" {
		t.Fatalf("element = %#v", element)
	}
	resolved, err := node.ElementWithError(d)
	if err != nil || resolved == nil || resolved.Role() != "P" {
		t.Fatalf("ElementWithError = %#v, err = %v", resolved, err)
	}
}

func TestOutlineNodeReferencesWithErrorReportMalformedData(t *testing.T) {
	d := &Document{objects: map[Ref]Object{{Object: 9}: String("invalid")}}
	if parent, err := newOutlineValue(documentdata.OutlineNodeSpec{Parent: Ref{Object: 9}, HasParent: true}).ParentNodeWithError(d); err == nil || parent != nil {
		t.Fatalf("outline parent = %#v, err = %v", parent, err)
	}
	if element, err := newOutlineValue(documentdata.OutlineNodeSpec{ElementRef: Ref{Object: 99}, HasElement: true}).ElementWithError(d); err == nil || element != nil {
		t.Fatalf("outline element = %#v, err = %v", element, err)
	}
	if _, err := (OutlineNode{}).ElementWithError(nil); err != errNilDocument {
		t.Fatalf("ElementWithError(nil) error = %v, want %v", err, errNilDocument)
	}
}

func TestOutlineReportsUnresolvedChildReference(t *testing.T) {
	d := &Document{trailer: Dict{Name("Root"): Ref{Object: 1}}, objects: map[Ref]Object{
		{Object: 1}: Dict{Name("Outlines"): Ref{Object: 2}},
		{Object: 2}: Dict{Name("First"): Ref{Object: 3}},
		{Object: 3}: Dict{Name("Title"): String("root"), Name("First"): Ref{Object: 99}},
	}}
	root, err := firstOutline(d)
	if err != nil || root.Title() != "root" {
		t.Fatalf("outline root = %#v, err = %v", root, err)
	}
	for _, err := range root.ChildrenSeq() {
		if err == nil || err.Error() != "playa: outline child could not be resolved" {
			t.Fatalf("unresolved outline child error = %v", err)
		}
		return
	}
	t.Fatal("unresolved outline child produced no error")
}

func TestOutlineParentReportsUnresolvedReference(t *testing.T) {
	d := &Document{objects: map[Ref]Object{}}
	node := newOutlineValue(documentdata.OutlineNodeSpec{Parent: Ref{Object: 99}, HasParent: true})
	if parent, err := node.ParentNodeWithError(d); err == nil || parent != nil || err.Error() != "playa: outline parent could not be resolved" {
		t.Fatalf("unresolved outline parent = %#v, err = %v", parent, err)
	}
}

func firstOutline(d *Document) (OutlineNode, error) {
	for node, err := range d.Outline() {
		return node, err
	}
	return OutlineNode{}, nil
}
