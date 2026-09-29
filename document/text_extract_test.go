package document

import (
	"sync"
	"testing"

	"github.com/lin-string/go-playa/textconfig"
)

func TestTextExtractionOptionsSnapshotCopiesBBox(t *testing.T) {
	bbox := [4]float64{1, 2, 3, 4}
	snapshot := snapshotTextExtractionOptions(textconfig.Options{BBox: &bbox})
	bbox[0] = 99
	if snapshot.BBox == nil || snapshot.BBox[0] != 1 {
		t.Fatalf("text extraction bbox was not snapshotted: %#v", snapshot.BBox)
	}
}

func TestLogicalTaggedSectionRanksRequiresUnambiguousPageSections(t *testing.T) {
	pageRef := Ref{Object: 9}
	page := Page{ref: pageRef, dict: Dict{Name("StructParents"): Number(4)}}
	documentWithElements := func(elements ...Object) *Document {
		return &Document{
			trailer: Dict{Name("Root"): Ref{Object: 1}},
			objects: map[Ref]Object{
				{Object: 1}: Dict{Name("StructTreeRoot"): Ref{Object: 2}},
				{Object: 2}: Dict{Name("K"): Array(elements)},
			},
		}
	}
	markedContent := func(mcid int, ref Ref) Dict {
		return Dict{Name("Type"): Name("MCR"), Name("Pg"): ref, Name("MCID"): Number(mcid)}
	}
	element := func(contents ...Object) Dict {
		return Dict{Name("S"): Name("P"), Name("K"): Array(contents)}
	}

	t.Run("multiple ordered contents remain sortable", func(t *testing.T) {
		doc := documentWithElements(element(markedContent(1, pageRef), markedContent(0, pageRef)))
		ranks, ok := page.logicalTaggedSectionRanks(doc, []taggedTextSection{
			{mcid: 0, hasMCID: true},
			{mcid: 1, hasMCID: true},
		})
		if !ok || ranks[1] != 0 || ranks[0] != 1 {
			t.Fatalf("logical ranks = %v, ok=%v; want 1:0, 0:1", ranks, ok)
		}
	})

	t.Run("duplicate extracted MCID falls back", func(t *testing.T) {
		doc := documentWithElements(
			element(markedContent(1, pageRef)),
			element(markedContent(0, pageRef)),
		)
		if ranks, ok := page.logicalTaggedSectionRanks(doc, []taggedTextSection{
			{mcid: 0, hasMCID: true},
			{mcid: 1, hasMCID: true},
			{mcid: 0, hasMCID: true},
		}); ok {
			t.Fatalf("duplicate extracted MCID unexpectedly sortable: %v", ranks)
		}
	})

	t.Run("Form content falls back", func(t *testing.T) {
		doc := documentWithElements(
			element(markedContent(1, pageRef)),
			element(markedContent(0, pageRef)),
		)
		if ranks, ok := page.logicalTaggedSectionRanks(doc, []taggedTextSection{
			{mcid: 0, hasMCID: true},
			{mcid: 1, hasMCID: true, insideForm: true},
		}); ok {
			t.Fatalf("Form content unexpectedly sortable with page contents: %v", ranks)
		}
	})

	t.Run("other-page contents do not block sorting", func(t *testing.T) {
		otherRef := Ref{Object: 10}
		doc := &Document{
			trailer: Dict{Name("Root"): Ref{Object: 1}},
			objects: map[Ref]Object{
				{Object: 1}: Dict{Name("StructTreeRoot"): Ref{Object: 2}},
				{Object: 2}: Dict{Name("K"): Array{
					Dict{Name("S"): Name("P"), Name("K"): markedContent(1, pageRef)},
					Dict{Name("S"): Name("P"), Name("K"): Array{
						markedContent(3, otherRef), markedContent(4, otherRef),
					}},
					Dict{Name("S"): Name("P"), Name("K"): markedContent(0, pageRef)},
				}},
			},
		}
		ranks, ok := page.logicalTaggedSectionRanks(doc, []taggedTextSection{
			{mcid: 0, hasMCID: true},
			{mcid: 1, hasMCID: true},
		})
		if !ok || ranks[1] != 0 || ranks[0] != 1 {
			t.Fatalf("logical ranks = %v, ok=%v; want current-page 1:0, 0:1", ranks, ok)
		}
	})
}

func taggedOrderTestDocument(pageRef Ref, order ...int) *Document {
	doc := &Document{
		trailer: Dict{Name("Root"): Ref{Object: 1}},
		objects: map[Ref]Object{
			{Object: 1}: Dict{Name("StructTreeRoot"): Ref{Object: 2}},
			{Object: 2}: Dict{Name("K"): Ref{Object: 3}},
		},
	}
	setTaggedOrderTestDocument(doc, pageRef, order...)
	return doc
}

func setTaggedOrderTestDocument(doc *Document, pageRef Ref, order ...int) {
	contents := make(Array, len(order))
	for i, mcid := range order {
		contents[i] = Dict{
			Name("Type"): Name("MCR"),
			Name("Pg"):   pageRef,
			Name("MCID"): Number(mcid),
		}
	}
	doc.objects[Ref{Object: 3}] = Dict{Name("S"): Name("P"), Name("K"): contents}
}

func TestLogicalTaggedSectionRanksRetainsDocumentOrderAcrossCacheRelease(t *testing.T) {
	pageRef := Ref{Object: 9}
	doc := taggedOrderTestDocument(pageRef, 1, 0)
	// Keep the synthetic source objects across ReleaseTransientCaches so the
	// test can prove that the derived rank index itself is invalidated.
	doc.scannedObjects = true
	page := Page{ref: pageRef, dict: Dict{Name("StructParents"): Number(4)}}
	sections := []taggedTextSection{{mcid: 0, hasMCID: true}, {mcid: 1, hasMCID: true}}

	first, ok := page.logicalTaggedSectionRanks(doc, sections)
	if !ok || first[1] != 0 || first[0] != 1 {
		t.Fatalf("first ranks = %v, ok=%v; want 1:0, 0:1", first, ok)
	}
	setTaggedOrderTestDocument(doc, pageRef, 0, 1)
	second, ok := page.logicalTaggedSectionRanks(doc, sections)
	if !ok || second[1] != 0 || second[0] != 1 {
		t.Fatalf("cached ranks = %v, ok=%v; want 1:0, 0:1", second, ok)
	}
	doc.ReleaseTransientCaches()
	third, ok := page.logicalTaggedSectionRanks(doc, sections)
	if !ok || third[1] != 0 || third[0] != 1 {
		t.Fatalf("retained ranks = %v, ok=%v; want 1:0, 0:1", third, ok)
	}
}

func TestLogicalTaggedSectionRanksKeepsAmbiguityPageLocal(t *testing.T) {
	firstPage := Ref{Object: 9}
	secondPage := Ref{Object: 10}
	doc := &Document{
		trailer: Dict{Name("Root"): Ref{Object: 1}},
		objects: map[Ref]Object{
			{Object: 1}: Dict{Name("StructTreeRoot"): Ref{Object: 2}},
			{Object: 2}: Dict{Name("K"): Dict{Name("S"): Name("P"), Name("K"): Array{
				Dict{Name("Type"): Name("MCR"), Name("Pg"): firstPage, Name("MCID"): Number(0)},
				Dict{Name("Type"): Name("MCR"), Name("Pg"): firstPage, Name("MCID"): Number(0)},
				Dict{Name("Type"): Name("MCR"), Name("Pg"): secondPage, Name("MCID"): Number(1)},
				Dict{Name("Type"): Name("MCR"), Name("Pg"): secondPage, Name("MCID"): Number(0)},
			}}},
		},
	}
	sections := []taggedTextSection{{mcid: 0, hasMCID: true}, {mcid: 1, hasMCID: true}}
	if ranks, ok := (Page{ref: firstPage, dict: Dict{Name("StructParents"): Number(1)}}).logicalTaggedSectionRanks(doc, sections); ok {
		t.Fatalf("duplicate first-page MCID unexpectedly sortable: %v", ranks)
	}
	ranks, ok := (Page{ref: secondPage, dict: Dict{Name("StructParents"): Number(2)}}).logicalTaggedSectionRanks(doc, sections)
	if !ok || ranks[1] != 0 || ranks[0] != 1 {
		t.Fatalf("second-page ranks = %v, ok=%v; want 1:0, 0:1", ranks, ok)
	}
}

func TestLogicalTaggedPageRanksCachesTraversalErrorUntilRelease(t *testing.T) {
	pageRef := Ref{Object: 9}
	doc := taggedOrderTestDocument(pageRef, 1, 0)
	doc.scannedObjects = true
	doc.objects[Ref{Object: 3}] = Dict{Name("S"): Name("P"), Name("K"): String("invalid")}

	if ranks, ok, err := doc.logicalTaggedPageRanks(pageRef); err == nil || ok || ranks != nil {
		t.Fatalf("malformed ranks = %v, ok=%v, err=%v; want cached error", ranks, ok, err)
	}
	setTaggedOrderTestDocument(doc, pageRef, 1, 0)
	if ranks, ok, err := doc.logicalTaggedPageRanks(pageRef); err == nil || ok || ranks != nil {
		t.Fatalf("mutated ranks = %v, ok=%v, err=%v; want original cached error", ranks, ok, err)
	}
	doc.ReleaseTransientCaches()
	ranks, ok, err := doc.logicalTaggedPageRanks(pageRef)
	if err != nil || !ok || ranks[1] != 0 || ranks[0] != 1 {
		t.Fatalf("released ranks = %v, ok=%v, err=%v; want 1:0, 0:1", ranks, ok, err)
	}
}

func TestLogicalTaggedPageRanksConcurrentReaders(t *testing.T) {
	pageRef := Ref{Object: 9}
	doc := taggedOrderTestDocument(pageRef, 2, 1, 0)
	const readers = 32
	start := make(chan struct{})
	errors := make(chan string, readers)
	var wait sync.WaitGroup
	for range readers {
		wait.Add(1)
		go func() {
			defer wait.Done()
			<-start
			ranks, ok, err := doc.logicalTaggedPageRanks(pageRef)
			if err != nil || !ok || len(ranks) != 3 || ranks[2] != 0 || ranks[1] != 1 || ranks[0] != 2 {
				errors <- "incomplete tagged rank index"
			}
		}()
	}
	close(start)
	wait.Wait()
	close(errors)
	for err := range errors {
		t.Fatal(err)
	}
}

func BenchmarkLogicalTaggedSectionRanksCached(b *testing.B) {
	pageRef := Ref{Object: 9}
	const sectionCount = 1000
	order := make([]int, sectionCount)
	sections := make([]taggedTextSection, sectionCount)
	for index := range sectionCount {
		order[index] = sectionCount - index - 1
		sections[index] = taggedTextSection{mcid: index, hasMCID: true}
	}
	doc := taggedOrderTestDocument(pageRef, order...)
	page := Page{ref: pageRef, dict: Dict{Name("StructParents"): Number(4)}}
	if _, ok := page.logicalTaggedSectionRanks(doc, sections); !ok {
		b.Fatal("failed to warm tagged rank index")
	}
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		if _, ok := page.logicalTaggedSectionRanks(doc, sections); !ok {
			b.Fatal("cached tagged ranks unavailable")
		}
	}
}

func TestInterpretTextTracksFormScope(t *testing.T) {
	ops, err := ParseContent([]byte("/P << /MCID 1 >> BDC BT (A) Tj ET EMC"))
	if err != nil {
		t.Fatal(err)
	}
	ops = append([]ContentOp{{formBoundary: formBegin}}, ops...)
	ops = append(ops, ContentOp{formBoundary: formEnd})
	texts := InterpretText(ops)
	if len(texts) != 1 || !texts[0].insideForm {
		t.Fatalf("Form text scope = %#v; want one inside-Form object", texts)
	}
}
