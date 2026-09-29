package document

import (
	"math"
	"strings"
	"sync"
	"testing"

	"github.com/lin-string/go-playa/cacheconfig"
	"github.com/lin-string/go-playa/documentdata"
)

func newTestAnnotation(spec documentdata.AnnotationSpec) Annotation {
	return Annotation{data: documentdata.NewAnnotation(spec)}
}

func TestAnnotationDictionaryCopiesDoNotExposeSource(t *testing.T) {
	annotation := newTestAnnotation(documentdata.AnnotationSpec{
		Dict:       Dict{Name("Meta"): Dict{Name("Value"): String("original")}},
		Action:     Dict{Name("S"): Name("URI")},
		Appearance: Dict{Name("N"): newStream(nil, []byte("q"))},
	})
	for _, copy := range []Dict{annotation.DictCopy(), annotation.ActionCopy(), annotation.AppearanceCopy()} {
		if copy == nil {
			t.Fatal("annotation copy is nil")
		}
	}
	copy := annotation.DictCopy()
	copy[Name("Meta")].(Dict)[Name("Value")] = String("changed")
	if got := annotation.DictCopy()[Name("Meta")].(Dict)[Name("Value")].(String); string(got) != "original" {
		t.Fatalf("annotation dictionary copy aliases source: %q", got)
	}
}

func TestAnnotationSnapshotsPreserveEmptySlices(t *testing.T) {
	annotation := newTestAnnotation(documentdata.AnnotationSpec{
		QuadPoints: make([][2]float64, 0),
		Color:      make([]float64, 0),
	})
	snapshot := annotation.Finalize()
	if snapshot.QuadPointsCopy() == nil || snapshot.ColorCopy() == nil {
		t.Fatalf("empty annotation slices were not preserved: %#v", snapshot)
	}
	if annotation.QuadPointsCopy() == nil || annotation.ColorCopy() == nil {
		t.Fatal("empty annotation copies became nil")
	}
}

func TestAnnotationActionValueCopyReportsMalformedNext(t *testing.T) {
	action := newAction("Named")
	action.document = &Document{}
	action.nextRaw = Number(1)
	annotation := Annotation{actionValue: action}
	if copy, err := annotation.ActionValueCopyWithError(); err == nil || copy != nil {
		t.Fatalf("malformed annotation action copy = %#v, err=%v", copy, err)
	}
	if snapshot, err := annotation.FinalizeWithError(); err == nil || snapshot.Subtype() != "" {
		t.Fatalf("malformed annotation finalize = %#v, err=%v", snapshot, err)
	}
}

func TestAnnotationsEmpty(t *testing.T) {
	p := Page{dict: Dict{}}
	d := &Document{objects: map[Ref]Object{}}
	got, err := d.CollectAnnotations(p)
	if err != nil {
		t.Fatal(err)
	}
	if got != nil {
		t.Fatal(got)
	}
}

func TestAnnotationsReportMalformedRoot(t *testing.T) {
	d := &Document{}
	p := Page{dict: Dict{Name("Annots"): Number(1)}}

	if _, err := d.CollectAnnotations(p); err == nil {
		t.Fatal("malformed Annots root was silently treated as empty")
	}
}

func TestAnnotationsReportMalformedEntries(t *testing.T) {
	d := &Document{}
	p := Page{dict: Dict{Name("Annots"): Array{Dict{Name("Rect"): Array{Number(0), Number(0), Number(1), Number(1)}}}}}
	if _, err := d.CollectAnnotations(p); err == nil {
		t.Fatal("malformed annotation was silently skipped")
	}
}

func TestAnnotationsRequireRectangle(t *testing.T) {
	d := &Document{}
	p := Page{dict: Dict{Name("Annots"): Array{Dict{Name("Subtype"): Name("Text")}}}}
	if _, err := d.CollectAnnotations(p); err == nil {
		t.Fatal("annotation without Rect was accepted")
	}
}

func TestAnnotationRelationsFollowMultiLevelIndirectErrorReferences(t *testing.T) {
	d := &Document{objects: map[Ref]Object{
		{Object: 1}: Ref{Object: 2},
		{Object: 2}: Dict{Name("Subtype"): Name("Link"), Name("Rect"): Array{Number(0)}},
	}}
	annotation := newTestAnnotation(documentdata.AnnotationSpec{InReplyTo: Ref{Object: 1}, HasInReplyTo: true})
	if _, err := annotation.InReplyToAnnotationWithError(d); err == nil || err.Error() != "playa: malformed referenced annotation" {
		t.Fatalf("multi-level malformed relation error = %v", err)
	}
}

func TestAnnotationsCacheTerminalErrors(t *testing.T) {
	ref := Ref{Object: 6}
	d := &Document{objects: map[Ref]Object{
		ref: Dict{Name("Subtype"): String("invalid")},
	}}
	p := Page{dict: Dict{Name("Annots"): Array{ref}}}
	_, firstErr := d.CollectAnnotations(p)
	if firstErr == nil {
		t.Fatal("malformed annotation produced no error")
	}
	d.objects[ref] = Dict{Name("Subtype"): Name("Text")}
	_, secondErr := d.CollectAnnotations(p)
	if secondErr == nil || secondErr.Error() != firstErr.Error() {
		t.Fatalf("cached annotation error = %v, want %v", secondErr, firstErr)
	}
}

func TestAnnotationErrorCachesHonorBudgets(t *testing.T) {
	itemRef := Ref{Object: 6}
	d := &Document{
		cacheOptionsConfigured: true,
		cacheOptions:           cacheconfig.Options{AnnotationErrorBytes: 0, AnnotationRootErrorBytes: 0},
		objects:                map[Ref]Object{itemRef: Dict{Name("Subtype"): String("invalid")}},
	}
	p := Page{ref: Ref{Object: 7}, dict: Dict{Name("Annots"): Array{itemRef}}}
	_, firstItemErr := d.CollectAnnotations(p)
	_, secondItemErr := d.CollectAnnotations(p)
	if firstItemErr == nil || secondItemErr == nil || firstItemErr == secondItemErr {
		t.Fatalf("annotation item errors = %v/%v, want uncached independent errors", firstItemErr, secondItemErr)
	}
	rootPage := Page{ref: Ref{Object: 8}, dict: Dict{Name("Annots"): Number(1)}}
	_, firstRootErr := d.CollectAnnotations(rootPage)
	_, secondRootErr := d.CollectAnnotations(rootPage)
	if firstRootErr == nil || secondRootErr == nil || firstRootErr == secondRootErr {
		t.Fatalf("annotation root errors = %v/%v, want uncached independent errors", firstRootErr, secondRootErr)
	}
	if len(d.annotationErrors) != 0 || len(d.annotationRootErrors) != 0 {
		t.Fatalf("annotation error caches retained entries with zero budget: item=%#v root=%#v", d.annotationErrors, d.annotationRootErrors)
	}
}

func TestAnnotationsRejectRectanglesWithWrongLength(t *testing.T) {
	d := &Document{}
	p := Page{dict: Dict{Name("Annots"): Array{Dict{
		Name("Subtype"): Name("Link"),
		Name("Rect"):    Array{Number(0), Number(0), Number(1), Number(1), Number(2)},
	}}}}
	if _, err := d.CollectAnnotations(p); err == nil {
		t.Fatal("annotation rectangle with five values was accepted")
	}
}

func TestAnnotationsRejectNonFiniteRectangles(t *testing.T) {
	for _, value := range []Number{Number(math.NaN()), Number(math.Inf(1))} {
		d := &Document{}
		p := Page{dict: Dict{Name("Annots"): Array{Dict{
			Name("Subtype"): Name("Link"),
			Name("Rect"):    Array{value, Number(0), Number(1), Number(1)},
		}}}}
		if _, err := d.CollectAnnotations(p); err == nil {
			t.Fatalf("annotation accepted non-finite rectangle value %v", value)
		}
	}
}

func TestAnnotationsRejectNonFiniteQuadPoints(t *testing.T) {
	q := Array{Number(0), Number(0), Number(1), Number(0), Number(1), Number(1), Number(math.NaN()), Number(1)}
	p := Page{dict: Dict{Name("Annots"): Array{Dict{
		Name("Subtype"):    Name("Highlight"),
		Name("QuadPoints"): q,
	}}}}
	if _, err := (&Document{}).CollectAnnotations(p); err == nil {
		t.Fatal("annotation accepted non-finite QuadPoints")
	}
}

func TestAnnotationsExposeActionFlagsAndSourceRectangle(t *testing.T) {
	p := Page{dict: Dict{Name("Annots"): Array{Dict{
		Name("Subtype"):  Name("Link"),
		Name("Rect"):     Array{Number(20), Number(30), Number(10), Number(5)},
		Name("Contents"): String([]byte("note")),
		Name("NM"):       String([]byte("a1")),
		Name("F"):        Number(4),
		Name("A"):        Dict{Name("S"): Name("URI"), Name("URI"): String([]byte("https://example.com"))},
	}}}}
	d := &Document{objects: map[Ref]Object{}}
	got, err := d.CollectAnnotations(p)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Rect() != [4]float64{20, 30, 10, 5} || got[0].Contents() != "note" || got[0].Name() != "a1" || !got[0].HasFlags() || got[0].Flags() != 4 || got[0].URI() != "https://example.com" {
		t.Fatalf("unexpected annotation: %#v", got)
	}
}

func TestAnnotationsExposeGeometryAppearanceAndActionKind(t *testing.T) {
	d := &Document{}
	p := Page{dict: Dict{Name("Annots"): Array{Dict{
		Name("Subtype"):    Name("Link"),
		Name("Rect"):       Array{Number(20), Number(40), Number(10), Number(30)},
		Name("QuadPoints"): Array{Number(10), Number(30), Number(20), Number(30), Number(10), Number(40), Number(20), Number(40)},
		Name("Border"):     Array{Number(0), Number(0), Number(2)},
		Name("C"):          Array{Number(1), Number(0.5), Number(0)},
		Name("AP"):         Dict{Name("N"): Stream{}},
		Name("A"):          Dict{Name("S"): Name("URI"), Name("URI"): String([]byte("https://example.com"))},
	}}}}
	got, err := d.CollectAnnotations(p)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Rect() != [4]float64{20, 40, 10, 30} || got[0].ActionKind() != "URI" || got[0].URI() != "https://example.com" {
		t.Fatalf("annotation = %#v", got)
	}
	if len(got[0].QuadPointsCopy()) != 4 || got[0].Border() != [3]float64{0, 0, 2} || len(got[0].ColorCopy()) != 3 || got[0].AppearanceCopy() == nil {
		t.Fatalf("annotation display metadata = %#v", got[0])
	}
}

func TestAnnotationsSequenceIsOrderedRepeatableAndStopsEarly(t *testing.T) {
	p := Page{dict: Dict{Name("Annots"): Array{
		Dict{Name("Subtype"): Name("Text"), Name("Rect"): Array{Number(0), Number(0), Number(1), Number(1)}, Name("NM"): String([]byte("first"))},
		Dict{Name("Subtype"): Name("Link"), Name("Rect"): Array{Number(0), Number(0), Number(1), Number(1)}, Name("NM"): String([]byte("second"))},
	}}}
	d := &Document{objects: map[Ref]Object{}}

	first := []string{}
	for annotation, err := range d.Annotations(p) {
		if err != nil {
			t.Fatal(err)
		}
		first = append(first, annotation.Name())
		break
	}
	if len(first) != 1 || first[0] != "first" {
		t.Fatalf("early annotations = %v, want [first]", first)
	}

	second, err := d.CollectAnnotations(p)
	if err != nil {
		t.Fatal(err)
	}
	if len(second) != 2 || second[0].Name() != "first" || second[1].Name() != "second" {
		t.Fatalf("second traversal = %#v", second)
	}
}

func TestPageCollectAnnotationsUsesPageSequence(t *testing.T) {
	d := &Document{}
	p := Page{dict: Dict{Name("Annots"): Array{
		Dict{Name("Subtype"): Name("Text"), Name("Rect"): Array{Number(0), Number(0), Number(1), Number(1)}},
		Dict{Name("Subtype"): Name("Link"), Name("Rect"): Array{Number(0), Number(0), Number(1), Number(1)}},
	}}}
	annotations, err := p.CollectAnnotations(d)
	if err != nil {
		t.Fatal(err)
	}
	if len(annotations) != 2 || annotations[0].Subtype() != "Text" || annotations[1].Subtype() != "Link" {
		t.Fatalf("page annotations = %#v", annotations)
	}
}

func TestAnnotationsReuseIndirectParseCache(t *testing.T) {
	ref := Ref{Object: 6}
	d := &Document{objects: map[Ref]Object{
		ref: Dict{Name("Subtype"): Name("Link"), Name("Rect"): Array{Number(0), Number(0), Number(1), Number(1)}, Name("NM"): String("cached")},
	}}
	p := Page{dict: Dict{Name("Annots"): Array{ref}}}
	for i := 0; i < 2; i++ {
		annotations, err := d.CollectAnnotations(p)
		if err != nil || len(annotations) != 1 || annotations[0].Name() != "cached" {
			t.Fatalf("annotations = %#v, err = %v", annotations, err)
		}
	}
	if len(d.annotationCache) != 1 {
		t.Fatalf("annotation cache size = %d, want 1", len(d.annotationCache))
	}
}

func TestAnnotationsReuseCacheAcrossIndirectReferenceChains(t *testing.T) {
	d := &Document{objects: map[Ref]Object{
		{Object: 1}: Ref{Object: 2},
		{Object: 2}: Dict{Name("Subtype"): Name("Link"), Name("Rect"): Array{Number(0), Number(0), Number(1), Number(1)}, Name("NM"): String("shared")},
	}}
	p := Page{dict: Dict{Name("Annots"): Array{Ref{Object: 1}, Ref{Object: 2}}}}
	annotations, err := d.CollectAnnotations(p)
	if err != nil || len(annotations) != 2 || annotations[0].Name() != "shared" || annotations[1].Name() != "shared" {
		t.Fatalf("annotations = %#v, err = %v", annotations, err)
	}
	if len(d.annotationCache) != 1 {
		t.Fatalf("annotation cache size = %d, want 1", len(d.annotationCache))
	}
}

func TestAnnotationDoesNotRetainOversizedCacheValues(t *testing.T) {
	ref := Ref{Object: 6}
	d := &Document{objects: map[Ref]Object{
		ref: Dict{Name("Subtype"): Name("Text"), Name("Rect"): Array{Number(0), Number(0), Number(1), Number(1)}, Name("Contents"): String(strings.Repeat("x", annotationCacheLimit+1))},
	}}
	p := Page{dict: Dict{Name("Annots"): Array{ref}}}
	annotations, err := d.CollectAnnotations(p)
	if err != nil || len(annotations) != 1 || len(annotations[0].Contents()) != annotationCacheLimit+1 {
		t.Fatalf("annotations = %d, err = %v", len(annotations), err)
	}
	if len(d.annotationCache) != 0 || d.annotationCacheBytes != 0 {
		t.Fatalf("oversized annotation entered cache: entries=%d bytes=%d", len(d.annotationCache), d.annotationCacheBytes)
	}
}

func TestAnnotationErrorsHaveBoundedCache(t *testing.T) {
	d := &Document{}
	const attempts = 4096
	for i := 1; i <= attempts; i++ {
		if _, err := d.annotationSequenceItem(Ref{Object: i}); err == nil {
			t.Fatalf("annotation %d unexpectedly resolved", i)
		}
	}
	if len(d.annotationErrors) >= attempts {
		t.Fatalf("annotation error cache retained every failure: entries=%d", len(d.annotationErrors))
	}
}

func TestConcurrentAnnotationResolution(t *testing.T) {
	ref := Ref{Object: 6}
	d := &Document{objects: map[Ref]Object{
		ref: Dict{Name("Subtype"): Name("Link"), Name("Rect"): Array{Number(0), Number(0), Number(1), Number(1)}, Name("NM"): String("cached")},
	}}
	p := Page{dict: Dict{Name("Annots"): Array{ref}}}
	const readers = 16
	var wg sync.WaitGroup
	errs := make(chan error, readers)
	start := make(chan struct{})
	for i := 0; i < readers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			annotations, err := d.CollectAnnotations(p)
			if err != nil {
				errs <- err
				return
			}
			if len(annotations) != 1 || annotations[0].Name() != "cached" {
				errs <- errNilDocument
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

func TestAnnotationCacheCopiesSliceFields(t *testing.T) {
	ref := Ref{Object: 6}
	d := &Document{objects: map[Ref]Object{
		ref: Dict{
			Name("Subtype"):    Name("Link"),
			Name("QuadPoints"): Array{Number(1), Number(2), Number(1), Number(2), Number(1), Number(2), Number(1), Number(2)},
			Name("C"):          Array{Number(0.1), Number(0.2), Number(0.3)},
		},
	}}
	first, ok := d.annotation(ref)
	if !ok || len(first.QuadPointsCopy()) != 4 || len(first.ColorCopy()) != 3 {
		t.Fatalf("first annotation = %#v", first)
	}
	snapshot := first.Finalize()
	quadPoints := snapshot.QuadPointsCopy()
	color := snapshot.ColorCopy()
	quadPoints[0][0] = 99
	color[0] = 99
	second, ok := d.annotation(ref)
	if !ok || second.QuadPointsCopy()[0][0] != 1 || second.ColorCopy()[0] != 0.1 {
		t.Fatalf("annotation cache was exposed = %#v", second)
	}
}

func TestAnnotationIgnoresMalformedColorArrays(t *testing.T) {
	d := &Document{}
	for _, color := range []Array{
		{Number(0), Number(1)},
		{Number(0), Number(1), Number(0), Number(1), Number(0)},
		{Number(0), String("invalid"), Number(1)},
	} {
		annotation, ok := d.annotation(Dict{Name("Subtype"): Name("Text"), Name("C"): color})
		if !ok {
			t.Fatalf("malformed color rejected the annotation: %#v", color)
		}
		if got := annotation.ColorCopy(); got != nil {
			t.Fatalf("malformed color was retained: %#v", got)
		}
	}
}

func TestAnnotationAcceptsStandardColorComponentCounts(t *testing.T) {
	d := &Document{}
	for _, color := range []Array{{Number(0.5)}, {Number(0.1), Number(0.2), Number(0.3)}, {Number(0.1), Number(0.2), Number(0.3), Number(0.4)}} {
		annotation, ok := d.annotation(Dict{Name("Subtype"): Name("Text"), Name("C"): color})
		if !ok || len(annotation.ColorCopy()) != len(color) {
			t.Fatalf("standard color was not retained: %#v", color)
		}
	}
}

func TestAnnotationCacheDoesNotShareDictionaries(t *testing.T) {
	ref := Ref{Object: 6}
	d := &Document{objects: map[Ref]Object{
		ref: Dict{
			Name("Subtype"): Name("Link"),
			Name("AP"):      Dict{Name("N"): newStream(nil, []byte("appearance"))},
		},
	}}
	first, ok := d.annotation(ref)
	if !ok {
		t.Fatal("expected annotation")
	}
	snapshot := first.Finalize()
	dict := snapshot.DictCopy()
	dict[Name("Subtype")] = Name("Changed")
	appearance := snapshot.AppearanceCopy()
	appearance[Name("N")] = Null{}
	stream := appearance[Name("N")].(Null)
	_ = stream
	second, ok := d.annotation(ref)
	if !ok {
		t.Fatal("expected cached annotation")
	}
	if _, ok := second.DictCopy()[Name("Subtype")].(Name); !ok {
		t.Fatalf("annotation dictionary cache was exposed: %#v", second.DictCopy())
	}
	if _, ok := second.AppearanceCopy()[Name("N")].(Stream); !ok {
		t.Fatalf("appearance dictionary cache was exposed: %#v", second.AppearanceCopy())
	}
}

func TestAnnotationPreservesModificationAndStructureParent(t *testing.T) {
	d := &Document{}
	annotation, ok := d.annotation(Dict{
		Name("Subtype"):      Name("Link"),
		Name("M"):            String("D:20240102030405Z"),
		Name("StructParent"): Number(7),
	})
	if !ok {
		t.Fatal("annotation was not parsed")
	}
	if annotation.Modified() != "D:20240102030405Z" {
		t.Fatalf("modified = %q", annotation.Modified())
	}
	if annotation.ParentKey() != 7 || !annotation.HasParentKey() {
		t.Fatalf("parent key = %d, present=%v", annotation.ParentKey(), annotation.HasParentKey())
	}
}

func TestAnnotationRejectsNegativeStructureParent(t *testing.T) {
	d := &Document{}
	annotation, ok := d.annotation(Dict{Name("Subtype"): Name("Text"), Name("StructParent"): Number(-1)})
	if !ok || annotation.HasParentKey() {
		t.Fatalf("negative StructParent accepted: %#v", annotation)
	}
}

func TestAnnotationParentWithErrorReportsMalformedParentTree(t *testing.T) {
	d := &Document{trailer: Dict{Name("Root"): Dict{Name("StructTreeRoot"): Number(1)}}}
	annotation := newTestAnnotation(documentdata.AnnotationSpec{ParentKey: 7, HasParentKey: true})
	if parent, err := annotation.ParentWithError(d); err == nil || parent != nil {
		t.Fatalf("annotation parent = %#v, err = %v", parent, err)
	}
	if parent := annotation.Parent(d); parent != nil {
		t.Fatalf("compatibility annotation parent = %#v", parent)
	}
}

func TestAnnotationParentUsesPageParentTreeSlot(t *testing.T) {
	d := &Document{
		trailer: Dict{Name("Root"): Ref{Object: 1}},
		objects: map[Ref]Object{
			{Object: 1}: Dict{Name("Pages"): Ref{Object: 2}, Name("StructTreeRoot"): Ref{Object: 3}},
			{Object: 2}: Dict{Name("Type"): Name("Pages"), Name("Kids"): Array{Ref{Object: 9}}},
			{Object: 3}: Dict{Name("ParentTree"): Dict{Name("Nums"): Array{
				Number(7), Array{Null{}, Dict{Name("S"): Name("Span"), Name("ActualText"): String("body")}},
			}}},
			{Object: 9}: Dict{Name("Type"): Name("Page"), Name("Parent"): Ref{Object: 2}, Name("StructParents"): Number(7)},
		},
	}
	annotation := newTestAnnotation(documentdata.AnnotationSpec{Page: Ref{Object: 9}, HasPage: true, ParentKey: 1, HasParentKey: true})
	parent, err := annotation.ParentWithError(d)
	if err != nil || parent == nil || parent.Role() != "Span" || parent.ActualText() != "body" {
		t.Fatalf("annotation parent = %#v, err = %v", parent, err)
	}
}

func TestAnnotationRejectsMalformedRectDirectly(t *testing.T) {
	d := &Document{}
	if _, ok := d.annotation(Dict{Name("Subtype"): Name("Text"), Name("Rect"): Array{Number(0), String("invalid"), Number(1), Number(1)}}); ok {
		t.Fatal("direct annotation parser accepted malformed Rect")
	}
}

func TestAnnotationRejectsRectWithExtraValuesDirectly(t *testing.T) {
	d := &Document{}
	if _, ok := d.annotation(Dict{Name("Subtype"): Name("Text"), Name("Rect"): Array{Number(0), Number(0), Number(1), Number(1), Number(2)}}); ok {
		t.Fatal("direct annotation parser accepted Rect with extra values")
	}
}

func TestAnnotationRejectsBorderWithExtraValues(t *testing.T) {
	d := &Document{}
	if _, ok := d.annotation(Dict{
		Name("Subtype"): Name("Text"),
		Name("Border"):  Array{Number(0), Number(0), Number(1), Number(2), Number(3)},
	}); ok {
		t.Fatal("direct annotation parser accepted Border with extra values")
	}
}

func TestAnnotationRejectsMalformedBorderDashValue(t *testing.T) {
	d := &Document{}
	if _, ok := d.annotation(Dict{
		Name("Subtype"): Name("Text"),
		Name("Border"):  Array{Number(0), Number(0), Number(1), Number(2)},
	}); ok {
		t.Fatal("direct annotation parser accepted non-array Border dash value")
	}
}

func TestAnnotationRejectsMalformedQuadPoints(t *testing.T) {
	d := &Document{}
	if _, ok := d.annotation(Dict{
		Name("Subtype"):    Name("Highlight"),
		Name("QuadPoints"): Array{Number(0), Number(0), Number(1), Number(0), Number(1), Number(1)},
	}); ok {
		t.Fatal("direct annotation parser accepted incomplete QuadPoints")
	}
}

func TestAnnotationResolvesDestination(t *testing.T) {
	d := &Document{}
	annotation := newTestAnnotation(documentdata.AnnotationSpec{Dest: Array{Ref{Object: 9}, Name("Fit")}})
	d.objects = map[Ref]Object{{Object: 9}: Dict{Name("Type"): Name("Page")}}
	d.trailer = Dict{Name("Root"): Ref{Object: 1}}
	d.objects[Ref{Object: 1}] = Dict{Name("Pages"): Ref{Object: 2}}
	d.objects[Ref{Object: 2}] = Dict{Name("Type"): Name("Pages"), Name("Kids"): Array{Ref{Object: 9}}}
	got := annotation.Destination(d)
	if got == nil {
		t.Fatalf("annotation destination = %#v", got)
	}
	page, hasPage := got.PageRef()
	if !hasPage || page != (Ref{Object: 9}) || got.View() != "Fit" {
		t.Fatalf("annotation destination = %#v", got)
	}
}

func TestAnnotationDestinationWithErrorReportsMalformedValue(t *testing.T) {
	d := &Document{}
	annotation := newTestAnnotation(documentdata.AnnotationSpec{Dest: Array{Number(1), Name("FitH")}})
	if destination, err := annotation.DestinationWithError(d); err == nil || destination != nil {
		t.Fatalf("annotation destination = %#v, err = %v", destination, err)
	}
	if destination := annotation.Destination(d); destination != nil {
		t.Fatalf("compatibility annotation destination = %#v", destination)
	}
}

func TestAnnotationGoToDestinationFollowsIndirectActionTarget(t *testing.T) {
	d := &Document{objects: map[Ref]Object{
		{Object: 1}: Array{Ref{Object: 9}, Name("Fit")},
	}}
	annotation := newTestAnnotation(documentdata.AnnotationSpec{
		Action: Dict{Name("D"): Ref{Object: 1}}, ActionKind: "GoTo",
	})
	annotation.actionValue = newAction("GoTo")
	target, err := annotation.DestinationWithError(d)
	if err != nil || target == nil {
		t.Fatalf("annotation destination = %#v, err = %v", target, err)
	}
	page, ok := target.PageRef()
	if !ok || page.Object != 9 || target.View() != "Fit" {
		t.Fatalf("annotation destination = %#v, want page 9 Fit", target)
	}
}

func TestAnnotationGoToDestinationWithErrorReportsIndirectFailure(t *testing.T) {
	d := &Document{}
	annotation := newTestAnnotation(documentdata.AnnotationSpec{
		Action: Dict{Name("D"): Ref{Object: 99}}, ActionKind: "GoTo",
	})
	annotation.actionValue = newAction("GoTo")
	if target, err := annotation.DestinationWithError(d); err == nil || target != nil {
		t.Fatalf("annotation destination = %#v, err = %v", target, err)
	}
}

func TestAnnotationDestinationWithErrorRejectsNilDocument(t *testing.T) {
	if destination, err := (Annotation{}).DestinationWithError(nil); err != errNilDocument || destination != nil {
		t.Fatalf("destination = %#v, err = %v", destination, err)
	}
}

func TestAnnotationResolvesOwningPage(t *testing.T) {
	d := &Document{
		trailer: Dict{Name("Root"): Ref{Object: 1}},
		objects: map[Ref]Object{
			{Object: 1}: Dict{Name("Pages"): Ref{Object: 2}},
			{Object: 2}: Dict{Name("Type"): Name("Pages"), Name("Kids"): Array{Ref{Object: 3}}, Name("Count"): Number(1)},
			{Object: 3}: Dict{Name("Type"): Name("Page"), Name("Parent"): Ref{Object: 2}, Name("Annots"): Array{Ref{Object: 4}}},
			{Object: 4}: Dict{Name("Subtype"): Name("Link"), Name("Rect"): Array{Number(0), Number(0), Number(10), Number(10)}},
		},
	}
	page, err := d.PageAt(0)
	if err != nil {
		t.Fatal(err)
	}
	annotation, err := d.CollectAnnotations(page)
	if err != nil || len(annotation) != 1 {
		t.Fatalf("annotations = %#v, err = %v", annotation, err)
	}
	if !annotation[0].HasPage() || annotation[0].Page() != (Ref{Object: 3}) {
		t.Fatalf("annotation page = %#v", annotation[0])
	}
	owned, err := annotation[0].PageObject(d)
	if err != nil || owned.ref != (Ref{Object: 3}) {
		t.Fatalf("page = %#v, err = %v", owned, err)
	}
}

func TestAnnotationBBoxUsesSelectedCoordinateSpace(t *testing.T) {
	d := &Document{
		space:   CoordinateSpacePage,
		trailer: Dict{Name("Root"): Ref{Object: 1}},
		objects: map[Ref]Object{
			{Object: 1}: Dict{Name("Pages"): Ref{Object: 2}},
			{Object: 2}: Dict{Name("Type"): Name("Pages"), Name("Kids"): Array{Ref{Object: 3}}, Name("Count"): Number(1)},
			{Object: 3}: Dict{Name("Type"): Name("Page"), Name("Parent"): Ref{Object: 2}, Name("MediaBox"): Array{Number(10), Number(20), Number(110), Number(220)}},
		},
	}
	page := Page{
		ref:  Ref{Object: 3},
		dict: Dict{Name("MediaBox"): Array{Number(10), Number(20), Number(110), Number(220)}},
	}
	annotation := newTestAnnotation(documentdata.AnnotationSpec{Page: page.ref, HasPage: true, Rect: [4]float64{20, 30, 40, 50}})
	if got := annotation.BBox(d); got != [4]float64{10, 10, 30, 30} {
		t.Fatalf("page-space annotation bbox = %v", got)
	}
	d.space = CoordinateSpaceScreen
	if got := annotation.BBox(d); got != [4]float64{10, 170, 30, 190} {
		t.Fatalf("screen-space annotation bbox = %v", got)
	}
}

func TestAnnotationBBoxFallsBackWhenTransformOverflows(t *testing.T) {
	d := &Document{space: CoordinateSpacePage}
	page := Page{dict: Dict{Name("MediaBox"): Array{Number(0), Number(0), Number(math.MaxFloat64), Number(math.MaxFloat64)}, Name("UserUnit"): Number(2)}}
	d.pageCache = map[Ref]Page{page.ref: page}
	annotation := newTestAnnotation(documentdata.AnnotationSpec{Page: page.ref, HasPage: true, Rect: [4]float64{0, 0, math.MaxFloat64, 1}})
	if got := annotation.BBox(d); got != annotation.Rect() {
		t.Fatalf("overflowing annotation bbox = %v, want fallback %v", got, annotation.Rect())
	}
}

func TestAnnotationResolvesInReplyTo(t *testing.T) {
	d := &Document{objects: map[Ref]Object{
		{Object: 4}: Dict{Name("Subtype"): Name("Text"), Name("Contents"): String([]byte("original"))},
		{Object: 5}: Dict{Name("Subtype"): Name("Text"), Name("IRT"): Ref{Object: 4}},
	}}
	annotation, ok := d.annotation(Ref{Object: 5})
	if !ok || !annotation.HasInReplyTo() || annotation.InReplyTo() != (Ref{Object: 4}) {
		t.Fatalf("annotation = %#v", annotation)
	}
	original := annotation.InReplyToAnnotation(d)
	if original == nil || original.Contents() != "original" {
		t.Fatalf("original = %#v", original)
	}
}

func TestAnnotationResolvesPopup(t *testing.T) {
	d := &Document{objects: map[Ref]Object{
		{Object: 6}: Dict{Name("Subtype"): Name("Popup"), Name("Contents"): String([]byte("popup"))},
		{Object: 7}: Dict{Name("Subtype"): Name("Text"), Name("Popup"): Ref{Object: 6}},
	}}
	annotation, ok := d.annotation(Ref{Object: 7})
	if !ok || !annotation.HasPopup() || annotation.Popup() != (Ref{Object: 6}) {
		t.Fatalf("annotation = %#v", annotation)
	}
	popup := annotation.PopupAnnotation(d)
	if popup == nil || popup.Subtype() != "Popup" || popup.Contents() != "popup" {
		t.Fatalf("popup = %#v", popup)
	}
	if newTestAnnotation(documentdata.AnnotationSpec{Popup: Ref{Object: 99}, HasPopup: true}).PopupAnnotation(d) != nil {
		t.Fatal("invalid popup reference should be ignored")
	}
}

func TestAnnotationReferenceWithErrorReportsMalformedTarget(t *testing.T) {
	d := &Document{objects: map[Ref]Object{{Object: 8}: String("invalid")}}
	annotation := newTestAnnotation(documentdata.AnnotationSpec{InReplyTo: Ref{Object: 8}, HasInReplyTo: true, Popup: Ref{Object: 8}, HasPopup: true})
	if target, err := annotation.InReplyToAnnotationWithError(d); err == nil || target != nil {
		t.Fatalf("IRT target = %#v, err = %v", target, err)
	}
	if target, err := annotation.PopupAnnotationWithError(d); err == nil || target != nil {
		t.Fatalf("Popup target = %#v, err = %v", target, err)
	}
	if annotation.InReplyToAnnotation(d) != nil || annotation.PopupAnnotation(d) != nil {
		t.Fatal("compatibility annotation references returned malformed targets")
	}
}
