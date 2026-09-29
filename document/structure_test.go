package document

import (
	"encoding/json"
	"fmt"
	"iter"
	"math"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/lin-string/go-playa/cacheconfig"
	"github.com/lin-string/go-playa/structureconfig"
	"github.com/lin-string/go-playa/structuredata"
)

func structureTree(t *testing.T, d *Document) []StructElement {
	t.Helper()
	got, err := d.StructureTree()
	if err != nil {
		t.Fatal(err)
	}
	return got
}

func TestStructureContentStreamCopyHandlesAbsentStream(t *testing.T) {
	if got := (StructureContent{}).StreamCopy(); got.DataBorrowed() != nil || got.DictBorrowed() != nil {
		t.Fatalf("absent stream copy = %#v", got)
	}
}

func TestStructureItemsExposeBorrowedPayloads(t *testing.T) {
	elementValue := newStructElementValue(structuredata.ElementSpec{Role: "P"})
	element := &elementValue
	item := newStructureElementItem(element)
	borrowedElement, ok := item.ElementBorrowed()
	if !ok || borrowedElement != element {
		t.Fatal("structure item did not expose its borrowed element")
	}

	content := newStructureContent(structuredata.ContentSpec{Kind: structureconfig.MarkedContent, MCID: 7, HasMCID: true})
	item = newStructureItem(structureconfig.ItemMarkedContent, content)
	borrowedContent, ok := item.ContentBorrowed()
	if !ok || borrowedContent.MCID() != 7 || !borrowedContent.HasMCID() {
		t.Fatalf("borrowed structure content = %#v, ok=%v", borrowedContent, ok)
	}

	entry := PageStructureEntry{index: 2, element: element, elements: []StructElement{*element}}
	if borrowed := entry.ElementBorrowed(); borrowed != element {
		t.Fatal("page structure entry did not expose its borrowed element")
	}
	count := 0
	for value := range entry.ElementsSeq() {
		count++
		if value.Role() != "P" {
			t.Fatalf("borrowed entry element role = %q", value.Role())
		}
	}
	if count != 1 {
		t.Fatalf("borrowed entry elements = %d, want 1", count)
	}
}

func TestStructElementItemsSeqPreservesMixedKOrder(t *testing.T) {
	element := StructElement{
		document: &Document{},
		childStart: Array{
			Dict{Name("S"): Name("P")},
			Number(7),
			Dict{Name("Type"): Name("MCR"), Name("MCID"): Number(3)},
		},
		childRoleMap: map[string]string{},
	}

	var kinds []structureconfig.ItemKind
	var childRole string
	var contentMCID int
	for item, err := range element.ItemsSeq() {
		if err != nil {
			t.Fatal(err)
		}
		kinds = append(kinds, item.Kind())
		if child, ok := item.ElementCopy(); ok {
			childRole = child.Role()
		}
		if content, ok := item.ContentCopy(); ok && content.HasMCID() {
			contentMCID = content.MCID()
		}
	}
	want := []structureconfig.ItemKind{
		structureconfig.ItemElement,
		structureconfig.ItemMarkedContent,
		structureconfig.ItemMarkedContent,
	}
	if !reflect.DeepEqual(kinds, want) || childRole != "P" || contentMCID != 3 {
		t.Fatalf("mixed structure items = %#v, childRole=%q, contentMCID=%d", kinds, childRole, contentMCID)
	}

	snapshot, err := element.FinalizeWithError()
	if err != nil {
		t.Fatal(err)
	}
	var snapshotKinds []structureconfig.ItemKind
	for item, err := range snapshot.ItemsSeq() {
		if err != nil {
			t.Fatal(err)
		}
		snapshotKinds = append(snapshotKinds, item.Kind())
	}
	if !reflect.DeepEqual(snapshotKinds, want) {
		t.Fatalf("finalized mixed structure items = %#v", snapshotKinds)
	}
}

func TestStructureEmpty(t *testing.T) {
	d := &Document{trailer: Dict{}, objects: map[Ref]Object{}}
	if structureTree(t, d) != nil {
		t.Fatal("unexpected tree")
	}
}

func TestStructureTreeRecognizesMarkedContentReference(t *testing.T) {
	d := &Document{
		trailer: Dict{Name("Root"): Ref{Object: 1, Generation: 0}},
		objects: map[Ref]Object{
			{Object: 1, Generation: 0}: Dict{Name("StructTreeRoot"): Ref{Object: 2, Generation: 0}},
			{Object: 2, Generation: 0}: Dict{Name("K"): Ref{Object: 3, Generation: 0}},
			{Object: 3, Generation: 0}: Dict{Name("Type"): Name("MCR"), Name("MCID"): Number(12)},
		},
	}
	got := structureTree(t, d)
	if len(got) != 1 || got[0].Type() != "MCR" || !got[0].HasMCID() || got[0].MCID() != 12 {
		t.Fatalf("unexpected MCR node: %#v", got)
	}
}

func TestPageStructureFollowsMultiLevelIndirectParentTreeValues(t *testing.T) {
	d := &Document{trailer: Dict{Name("Root"): Ref{Object: 1}}, objects: map[Ref]Object{
		{Object: 1}: Dict{Name("StructTreeRoot"): Ref{Object: 2}},
		{Object: 2}: Dict{Name("ParentTree"): Ref{Object: 3}, Name("K"): Array{Ref{Object: 6}}},
		{Object: 3}: Ref{Object: 4},
		{Object: 4}: Dict{Name("Nums"): Ref{Object: 5}},
		{Object: 5}: Array{Number(7), Array{Ref{Object: 6}}},
		{Object: 6}: Ref{Object: 7},
		{Object: 7}: Dict{Name("S"): Ref{Object: 8}, Name("T"): Ref{Object: 9}},
		{Object: 8}: Name("P"), {Object: 9}: String("paragraph"),
	}}
	page := Page{ref: Ref{Object: 20}, dict: Dict{Name("StructParents"): Ref{Object: 10}}}
	d.objects[Ref{Object: 10}] = Number(7)
	structure, err := page.Structure(d)
	if err != nil || len(structure.elements) != 1 || structure.elements[0].Role() != "P" || structure.elements[0].Title() != "paragraph" {
		t.Fatalf("indirect parent tree structure = %#v, err=%v", structure, err)
	}
}

func TestStructureTreeDoesNotRevisitSharedIndirectNodes(t *testing.T) {
	d := &Document{trailer: Dict{Name("Root"): Ref{Object: 1}}, objects: map[Ref]Object{
		{Object: 1}: Dict{Name("StructTreeRoot"): Ref{Object: 2}},
		{Object: 2}: Dict{Name("K"): Array{Ref{Object: 3}, Ref{Object: 5}}},
		{Object: 3}: Ref{Object: 4},
		{Object: 4}: Dict{Name("S"): Name("P"), Name("T"): String("shared")},
		{Object: 5}: Ref{Object: 4},
	}}
	var elements []StructElement
	for element, err := range d.StructureTreeSeq() {
		if err != nil {
			t.Fatal(err)
		}
		elements = append(elements, element)
	}
	if len(elements) != 1 || elements[0].Title() != "shared" {
		t.Fatalf("shared structure nodes = %#v", elements)
	}
}

func TestStructureTreeDecodesPDFTextTitle(t *testing.T) {
	d := &Document{
		trailer: Dict{Name("Root"): Ref{Object: 1, Generation: 0}},
		objects: map[Ref]Object{
			{Object: 1, Generation: 0}: Dict{Name("StructTreeRoot"): Ref{Object: 2, Generation: 0}},
			{Object: 2, Generation: 0}: Dict{Name("K"): Ref{Object: 3, Generation: 0}},
			{Object: 3, Generation: 0}: Dict{Name("S"): Name("P"), Name("T"): String{0xfe, 0xff, 0x4e, 0x2d}},
		},
	}
	got := structureTree(t, d)
	if len(got) != 1 || got[0].Title() != "中" {
		t.Fatalf("structure title = %#v", got)
	}
}

func TestStructureElementExposesPlayaDescriptionProperties(t *testing.T) {
	d := &Document{
		trailer: Dict{Name("Root"): Ref{Object: 1, Generation: 0}},
		objects: map[Ref]Object{
			{Object: 1, Generation: 0}: Dict{Name("StructTreeRoot"): Ref{Object: 2, Generation: 0}},
			{Object: 2, Generation: 0}: Dict{Name("K"): Ref{Object: 3, Generation: 0}},
			{Object: 3, Generation: 0}: Dict{
				Name("S"):          Name("Span"),
				Name("Alt"):        String("alternate"),
				Name("E"):          String("expanded"),
				Name("ActualText"): String("actual"),
			},
		},
	}
	elements := structureTree(t, d)
	if len(elements) != 1 {
		t.Fatalf("structure elements = %#v, want one element", elements)
	}
	element := elements[0]
	if got := element.AlternateDescription(); got != "alternate" {
		t.Fatalf("alternate description = %q, want %q", got, "alternate")
	}
	if got := element.AbbreviationExpansion(); got != "expanded" {
		t.Fatalf("abbreviation expansion = %q, want %q", got, "expanded")
	}
}

func TestStructureTreeAppliesRoleMap(t *testing.T) {
	d := &Document{
		trailer: Dict{Name("Root"): Ref{Object: 1, Generation: 0}},
		objects: map[Ref]Object{
			{Object: 1, Generation: 0}: Dict{Name("StructTreeRoot"): Ref{Object: 2, Generation: 0}},
			{Object: 2, Generation: 0}: Dict{Name("RoleMap"): Dict{Name("MyParagraph"): Name("P")}, Name("K"): Ref{Object: 3, Generation: 0}},
			{Object: 3, Generation: 0}: Dict{Name("S"): Name("MyParagraph")},
		},
	}
	got := structureTree(t, d)
	if len(got) != 1 || got[0].Role() != "P" || got[0].RawRole() != "MyParagraph" || got[0].StructureType() != "MyParagraph" {
		t.Fatalf("role-mapped structure = %#v", got)
	}
}

func TestStructureTreeReadsNormalizedBBox(t *testing.T) {
	d := &Document{
		trailer: Dict{Name("Root"): Ref{Object: 1, Generation: 0}},
		objects: map[Ref]Object{
			{Object: 1, Generation: 0}: Dict{Name("StructTreeRoot"): Ref{Object: 2, Generation: 0}},
			{Object: 2, Generation: 0}: Dict{Name("K"): Ref{Object: 3, Generation: 0}},
			{Object: 3, Generation: 0}: Dict{Name("S"): Name("Figure"), Name("BBox"): Array{Number(20), Number(40), Number(5), Number(10)}},
		},
	}
	got := structureTree(t, d)
	if len(got) != 1 || !got[0].HasBBox() || got[0].BBox() != [4]float64{5, 10, 20, 40} {
		t.Fatalf("structure bbox = %#v", got)
	}
}

func TestStructureTreeIgnoresMalformedBBoxValues(t *testing.T) {
	d := &Document{
		trailer: Dict{Name("Root"): Ref{Object: 1, Generation: 0}},
		objects: map[Ref]Object{
			{Object: 1, Generation: 0}: Dict{Name("StructTreeRoot"): Ref{Object: 2, Generation: 0}},
			{Object: 2, Generation: 0}: Dict{Name("K"): Ref{Object: 3, Generation: 0}},
			{Object: 3, Generation: 0}: Dict{Name("S"): Name("Figure"), Name("BBox"): Array{Number(20), String("invalid"), Number(5), Number(10)}},
		},
	}
	got := structureTree(t, d)
	if len(got) != 1 || got[0].HasBBox() {
		t.Fatalf("malformed structure bbox = %#v", got)
	}
}

func TestStructureRectRejectsNonFiniteValues(t *testing.T) {
	for _, value := range []Number{Number(math.NaN()), Number(math.Inf(1))} {
		if rect, ok := structureRect(&Document{}, Array{value, Number(0), Number(1), Number(1)}); ok {
			t.Fatalf("structure rectangle accepted non-finite value %v: %v", value, rect)
		}
	}
}

func TestStructureTreeIgnoresBBoxWithExtraValues(t *testing.T) {
	d := &Document{
		trailer: Dict{Name("Root"): Ref{Object: 1}},
		objects: map[Ref]Object{
			{Object: 1}: Dict{Name("StructTreeRoot"): Ref{Object: 2}},
			{Object: 2}: Dict{Name("K"): Array{Ref{Object: 3}}},
			{Object: 3}: Dict{
				Name("S"):    Name("Figure"),
				Name("BBox"): Array{Number(0), Number(0), Number(10), Number(10), Number(20)},
			},
		},
	}
	got := structureTree(t, d)
	if len(got) != 1 || got[0].HasBBox() {
		t.Fatalf("malformed structure BBox was accepted: %#v", got)
	}
}

func TestStructureElementBBoxValueUsesExplicitBBox(t *testing.T) {
	e := newStructElementValue(structuredata.ElementSpec{BBox: [4]float64{1, 2, 3, 4}, HasBBox: true})
	got, err := e.BBoxValue(&Document{})
	if err != nil || got != e.BBox() {
		t.Fatalf("bbox = %v, err = %v", got, err)
	}
}

func TestStructureElementTextRejectsNilDocument(t *testing.T) {
	if _, err := (StructElement{}).Text(nil); err != errNilDocument {
		t.Fatalf("text error = %v", err)
	}
}

func TestStructureElementResolvesPageAndParent(t *testing.T) {
	d := &Document{
		trailer: Dict{Name("Root"): Ref{Object: 1}},
		objects: map[Ref]Object{
			{Object: 1}:  Dict{Name("Pages"): Ref{Object: 10}, Name("StructTreeRoot"): Ref{Object: 2}},
			{Object: 2}:  Dict{Name("K"): Ref{Object: 3}},
			{Object: 3}:  Dict{Name("Type"): Name("StructElem"), Name("S"): Name("P"), Name("P"): Ref{Object: 4}, Name("Pg"): Ref{Object: 8}},
			{Object: 4}:  Dict{Name("Type"): Name("StructElem"), Name("S"): Name("Document")},
			{Object: 10}: Dict{Name("Type"): Name("Pages"), Name("Kids"): Array{Ref{Object: 8}}, Name("Count"): Number(1)},
			{Object: 8}:  Dict{Name("Type"): Name("Page"), Name("Parent"): Ref{Object: 10}},
		},
	}
	elements := structureTree(t, d)
	if len(elements) != 1 {
		t.Fatalf("structure elements = %#v", elements)
	}
	page, err := elements[0].PageObject(d)
	if err != nil || page.ref != (Ref{Object: 8}) {
		t.Fatalf("page = %#v, err = %v", page, err)
	}
	parent := elements[0].ParentElement(d)
	if parent == nil || parent.Role() != "Document" {
		t.Fatalf("parent = %#v", parent)
	}
}

func TestStructureElementPageObjectRejectsMissingDocument(t *testing.T) {
	if _, err := (StructElement{}).PageObject(nil); err != errNilDocument {
		t.Fatalf("PageObject(nil) error = %v", err)
	}
}

func TestStructureContentResolvesPageAndObject(t *testing.T) {
	d := &Document{
		trailer: Dict{Name("Root"): Ref{Object: 1}},
		objects: map[Ref]Object{
			{Object: 1}: Dict{Name("Pages"): Ref{Object: 2}},
			{Object: 2}: Dict{Name("Type"): Name("Pages"), Name("Kids"): Array{Ref{Object: 3}}, Name("Count"): Number(1)},
			{Object: 3}: Dict{Name("Type"): Name("Page"), Name("Parent"): Ref{Object: 2}},
			{Object: 4}: String("object"),
		},
	}
	content := newStructureContent(structuredata.ContentSpec{Page: Ref{Object: 3}, HasPage: true, ObjectRef: Ref{Object: 4}, HasObject: true})
	page, err := content.PageObject(d)
	if err != nil || page.ref != (Ref{Object: 3}) {
		t.Fatalf("page = %#v, err = %v", page, err)
	}
	object, ok := content.Object(d)
	value, isString := object.(String)
	if !ok || !isString || string(value) != "object" {
		t.Fatalf("object = %#v, ok = %v", object, ok)
	}
}

func TestStructureContentObjectWithErrorReportsUnresolvedReference(t *testing.T) {
	content := newStructureContent(structuredata.ContentSpec{ObjectRef: Ref{Object: 99}, HasObject: true})
	if object, ok, err := content.ObjectWithError(&Document{objects: map[Ref]Object{}}); err == nil || ok || object != nil {
		t.Fatalf("unresolved structure object = %#v, ok=%v, err=%v", object, ok, err)
	}
}

func TestStructElementObjectWithErrorReportsUnresolvedReference(t *testing.T) {
	element := newStructElementValue(structuredata.ElementSpec{HasObject: true, ObjectRef: Ref{Object: 99}})
	if object, ok, err := element.ObjectWithError(&Document{objects: map[Ref]Object{}}); err == nil || ok || object != nil {
		t.Fatalf("object = %#v, ok = %v, err = %v", object, ok, err)
	}
	if _, _, err := (StructElement{}).ObjectWithError(nil); err != errNilDocument {
		t.Fatalf("ObjectWithError(nil) error = %v, want %v", err, errNilDocument)
	}
}

func TestStructElementParentWithErrorReportsUnresolvedReference(t *testing.T) {
	element := newStructElementValue(structuredata.ElementSpec{HasParent: true, Parent: Ref{Object: 99}})
	if parent, err := element.ParentElementWithError(&Document{objects: map[Ref]Object{}}); err == nil || parent != nil {
		t.Fatalf("parent = %#v, err = %v", parent, err)
	}
	if _, err := (StructElement{}).ParentElementWithError(nil); err != errNilDocument {
		t.Fatalf("ParentElementWithError(nil) error = %v, want %v", err, errNilDocument)
	}
}

func TestStructElementParentWithErrorReportsMalformedParentContent(t *testing.T) {
	d := &Document{objects: map[Ref]Object{
		{Object: 1}: Dict{Name("Type"): Name("StructElem"), Name("K"): Dict{
			Name("Type"): Name("MCR"), Name("MCID"): Ref{Object: 99},
		}},
	}}
	element := newStructElementValue(structuredata.ElementSpec{HasParent: true, Parent: Ref{Object: 1}})
	if parent, err := element.ParentElementWithError(d); err == nil || parent != nil {
		t.Fatalf("parent = %#v, err = %v", parent, err)
	}
}

func TestStructureContentObjectBBoxUsesObjectRectangle(t *testing.T) {
	d := &Document{
		space:   CoordinateSpaceDefault,
		trailer: Dict{Name("Root"): Ref{Object: 1}},
		objects: map[Ref]Object{
			{Object: 1}: Dict{Name("Pages"): Ref{Object: 2}},
			{Object: 2}: Dict{Name("Type"): Name("Pages"), Name("Kids"): Array{Ref{Object: 3}}, Name("Count"): Number(1)},
			{Object: 3}: Dict{Name("Type"): Name("Page"), Name("Parent"): Ref{Object: 2}, Name("MediaBox"): Array{Number(0), Number(0), Number(100), Number(100)}},
			{Object: 4}: Dict{Name("Rect"): Array{Number(1), Number(2), Number(3), Number(4)}},
		},
	}
	content := newStructureContent(structuredata.ContentSpec{Kind: StructureObject, Page: Ref{Object: 3}, HasPage: true, ObjectRef: Ref{Object: 4}, HasObject: true})
	if got, err := content.BBoxValue(d); err != nil || got != [4]float64{1, 2, 3, 4} {
		t.Fatalf("object bbox = %v, err = %v", got, err)
	}
}

func TestStructureContentObjectRejectsBBoxWithExtraValues(t *testing.T) {
	if got, ok := structureRect(&Document{}, Array{Number(1), Number(2), Number(3), Number(4), Number(5)}); ok {
		t.Fatalf("malformed structure object bbox was accepted: %v", got)
	}
}

func TestStructureContentObjectBBoxUsesStreamDictionary(t *testing.T) {
	d := &Document{
		space:   CoordinateSpaceDefault,
		trailer: Dict{Name("Root"): Ref{Object: 1}},
		objects: map[Ref]Object{
			{Object: 1}: Dict{Name("Pages"): Ref{Object: 2}},
			{Object: 2}: Dict{Name("Type"): Name("Pages"), Name("Kids"): Array{Ref{Object: 3}}, Name("Count"): Number(1)},
			{Object: 3}: Dict{Name("Type"): Name("Page"), Name("Parent"): Ref{Object: 2}, Name("MediaBox"): Array{Number(0), Number(0), Number(100), Number(100)}},
			{Object: 4}: newStream(Dict{Name("BBox"): Array{Number(5), Number(6), Number(7), Number(8)}}, []byte("q")),
		},
	}
	content := newStructureContent(structuredata.ContentSpec{Kind: StructureObject, Page: Ref{Object: 3}, HasPage: true, ObjectRef: Ref{Object: 4}, HasObject: true})
	if got, err := content.BBoxValue(d); err != nil || got != [4]float64{5, 6, 7, 8} {
		t.Fatalf("stream object bbox = %v, err = %v", got, err)
	}
}

func TestStructureContentObjectDoesNotExposeResolvedDictionary(t *testing.T) {
	d := &Document{objects: map[Ref]Object{
		{Object: 4}: Dict{Name("Value"): String("original")},
	}}
	content := newStructureContent(structuredata.ContentSpec{ObjectRef: Ref{Object: 4}, HasObject: true})
	first, ok := content.Object(d)
	if !ok {
		t.Fatal("expected structure object")
	}
	first.(Dict)[Name("Value")] = String("changed")
	second, ok := content.Object(d)
	if !ok {
		t.Fatal("expected cached structure object")
	}
	value, _ := second.(Dict)[Name("Value")].(String)
	if string(value) != "original" {
		t.Fatalf("structure object was exposed: %#v", second)
	}
}

func TestStructureContentLazilyResolvesObjectsTextAndBBox(t *testing.T) {
	d := &Document{
		trailer: Dict{Name("Root"): Ref{Object: 1}},
		objects: map[Ref]Object{
			{Object: 1}: Dict{Name("Pages"): Ref{Object: 2}},
			{Object: 2}: Dict{Name("Type"): Name("Pages"), Name("Kids"): Array{Ref{Object: 3}}, Name("Count"): Number(1)},
			{Object: 3}: Dict{Name("Type"): Name("Page"), Name("Parent"): Ref{Object: 2}, Name("Contents"): newStream(nil, []byte("/P << /MCID 7 /ActualText (actual) >> BDC BT /F1 10 Tf (painted) Tj ET EMC"))},
		},
	}
	content := newStructureContent(structuredata.ContentSpec{Kind: StructureMarkedContent, MCID: 7, HasMCID: true, Page: Ref{Object: 3}, HasPage: true})
	count := 0
	for object, err := range content.ContentSeq(d) {
		if err != nil {
			t.Fatal(err)
		}
		count++
		if object.text == nil {
			t.Fatalf("content object = %#v", object)
		}
	}
	if count != 1 {
		t.Fatalf("content object count = %d, want 1", count)
	}
	if got, err := content.Text(d); err != nil || got != "actual" {
		t.Fatalf("content text = %q, err = %v", got, err)
	}
	if got, err := content.BBoxValue(d); err != nil || got == [4]float64{} {
		t.Fatalf("content bbox = %v, err = %v", got, err)
	}
}

func TestStructureContentSequenceRejectsNilDocument(t *testing.T) {
	content := newStructureContent(structuredata.ContentSpec{Kind: StructureMarkedContent, HasPage: true, HasMCID: true})
	for _, err := range content.ContentSeq(nil) {
		if err != errNilDocument {
			t.Fatalf("ContentSeq(nil) error = %v, want %v", err, errNilDocument)
		}
		return
	}
	t.Fatal("ContentSeq(nil) produced no error")
}

func TestStructureContentsSequenceReportsUnresolvedFields(t *testing.T) {
	d := &Document{}
	for _, err := range d.structElementContentsSeq(Dict{Name("Type"): Ref{Object: 99}}, Ref{}, false, map[Ref]bool{}) {
		if err == nil || !strings.Contains(err.Error(), "structure content Type could not be resolved") {
			t.Fatalf("unresolved structure content field error = %v", err)
		}
		return
	}
	t.Fatal("unresolved structure content field produced no error")
}

func TestStructureChildrenSequenceReportsUnresolvedType(t *testing.T) {
	d := &Document{}
	for _, err := range d.structElementChildrenSeq(Dict{Name("Type"): Ref{Object: 99}}, map[Ref]bool{}, nil) {
		if err == nil || !strings.Contains(err.Error(), "structure child Type could not be resolved") {
			t.Fatalf("unresolved structure child type error = %v", err)
		}
		return
	}
	t.Fatal("unresolved structure child type produced no error")
}

func TestStructElementFinalizeReportsMalformedContentField(t *testing.T) {
	element := StructElement{
		document:   &Document{},
		childStart: Dict{Name("Type"): Name("MCR"), Name("MCID"): Ref{Object: 99}},
	}
	if snapshot, err := element.FinalizeWithError(); err == nil || snapshot.document != nil {
		t.Fatalf("FinalizeWithError = %#v, err = %v", snapshot, err)
	}
}

func TestStructureElementPageOrderPutsObjectBeforeMarkedContent(t *testing.T) {
	d := &Document{
		trailer: Dict{Name("Root"): Ref{Object: 1}},
		objects: map[Ref]Object{
			{Object: 1}: Dict{Name("Pages"): Ref{Object: 2}},
			{Object: 2}: Dict{Name("Type"): Name("Pages"), Name("Kids"): Array{Ref{Object: 3}}, Name("Count"): Number(1)},
			{Object: 3}: Dict{Name("Type"): Name("Page"), Name("Parent"): Ref{Object: 2}, Name("Contents"): newStream(nil, []byte("/P << /MCID 7 >> BDC 0 0 m 1 1 l S EMC"))},
			{Object: 4}: Dict{Name("Rect"): Array{Number(1), Number(2), Number(3), Number(4)}},
		},
	}
	element := newStructElementValue(structuredata.ElementSpec{Page: Ref{Object: 3}, HasPage: true})
	element.contentsReady = true
	element.orderedContents = []StructureContent{
		newStructureContent(structuredata.ContentSpec{Kind: StructureMarkedContent, MCID: 7, HasMCID: true, Page: Ref{Object: 3}, HasPage: true}),
		newStructureContent(structuredata.ContentSpec{Kind: StructureObject, ObjectRef: Ref{Object: 4}, HasObject: true, Page: Ref{Object: 3}, HasPage: true}),
	}
	var kinds []structureconfig.ContentKind
	for content, err := range element.PageOrderSeq(d) {
		if err != nil {
			t.Fatal(err)
		}
		kinds = append(kinds, content.Kind())
	}
	if !reflect.DeepEqual(kinds, []structureconfig.ContentKind{StructureObject, StructureMarkedContent}) {
		t.Fatalf("page order kinds = %#v", kinds)
	}
}

func TestStructureContentIncludesMarkedContentInFormXObjects(t *testing.T) {
	d := &Document{
		trailer: Dict{Name("Root"): Ref{Object: 1}},
		objects: map[Ref]Object{
			{Object: 1}: Dict{Name("Pages"): Ref{Object: 2}},
			{Object: 2}: Dict{Name("Type"): Name("Pages"), Name("Kids"): Array{Ref{Object: 3}}, Name("Count"): Number(1)},
			{Object: 3}: Dict{
				Name("Type"):      Name("Page"),
				Name("Parent"):    Ref{Object: 2},
				Name("Resources"): Dict{Name("XObject"): Dict{Name("Fm1"): Ref{Object: 4}}},
				Name("Contents"):  newStream(nil, []byte("/Fm1 Do")),
			},
			{Object: 4}: newStream(Dict{Name("Subtype"): Name("Form"), Name("Resources"): Dict{Name("Font"): Dict{Name("F1"): Ref{Object: 5}}}}, []byte("/P << /MCID 7 >> BDC BT /F1 10 Tf (nested) Tj ET EMC")),
			{Object: 5}: Dict{Name("Type"): Name("Font"), Name("Subtype"): Name("Type1"), Name("BaseFont"): Name("Helvetica")},
		},
	}
	content := newStructureContent(structuredata.ContentSpec{Kind: StructureMarkedContent, MCID: 7, HasMCID: true, Page: Ref{Object: 3}, HasPage: true})
	var text string
	count := 0
	for object, err := range content.ContentSeq(d) {
		if err != nil {
			t.Fatal(err)
		}
		if object.text != nil {
			text += object.text.Text()
			count++
		}
	}
	if count != 1 || text != "nested" {
		t.Fatalf("nested structure content = count %d, text %q", count, text)
	}
}

func TestStructureContentTextHonorsEmptyActualText(t *testing.T) {
	d := &Document{trailer: Dict{Name("Root"): Ref{Object: 1}}, objects: map[Ref]Object{
		{Object: 1}: Dict{Name("Pages"): Ref{Object: 2}},
		{Object: 2}: Dict{Name("Type"): Name("Pages"), Name("Kids"): Array{Ref{Object: 3}}},
		{Object: 3}: Dict{Name("Type"): Name("Page"), Name("Parent"): Ref{Object: 2}, Name("Contents"): newStream(nil, []byte(
			"/P << /MCID 7 /ActualText () >> BDC BT /F1 10 Tf (painted) Tj ET EMC",
		))},
	}}
	content := newStructureContent(structuredata.ContentSpec{Kind: StructureMarkedContent, Page: Ref{Object: 3}, HasPage: true, MCID: 7, HasMCID: true})
	text, err := content.Text(d)
	if err != nil {
		t.Fatal(err)
	}
	if text != "" {
		t.Fatalf("empty ActualText fell back to painted text: %q", text)
	}
}

func TestStructureTreeReadsAccessibilityAndPageFields(t *testing.T) {
	d := &Document{
		trailer: Dict{Name("Root"): Ref{Object: 1, Generation: 0}},
		objects: map[Ref]Object{
			{Object: 1, Generation: 0}: Dict{Name("StructTreeRoot"): Ref{Object: 2, Generation: 0}},
			{Object: 2, Generation: 0}: Dict{Name("K"): Ref{Object: 3, Generation: 0}},
			{Object: 3, Generation: 0}: Dict{Name("S"): Name("Figure"), Name("Alt"): String([]byte("diagram")), Name("ActualText"): String([]byte{0xfe, 0xff, 0x66, 0xff, 0x4e, 0xe3, 0x65, 0x87, 0x67, 0x2c}), Name("Pg"): Ref{Object: 9, Generation: 0}},
			{Object: 9, Generation: 0}: Dict{Name("Type"): Name("Page")},
		},
	}
	got := structureTree(t, d)
	if len(got) != 1 || got[0].AlternateDescription() != "diagram" || got[0].ActualText() != "替代文本" || !got[0].HasPage() || got[0].Page() != (Ref{Object: 9, Generation: 0}) {
		t.Fatalf("structure accessibility fields = %#v", got)
	}
}

func TestStructureTreeReadsParentReference(t *testing.T) {
	d := &Document{
		trailer: Dict{Name("Root"): Ref{Object: 1, Generation: 0}},
		objects: map[Ref]Object{
			{Object: 1, Generation: 0}: Dict{Name("StructTreeRoot"): Ref{Object: 2, Generation: 0}},
			{Object: 2, Generation: 0}: Dict{Name("K"): Ref{Object: 3, Generation: 0}},
			{Object: 3, Generation: 0}: Dict{Name("S"): Name("Document"), Name("K"): Ref{Object: 4, Generation: 0}},
			{Object: 4, Generation: 0}: Dict{Name("S"): Name("P"), Name("P"): Ref{Object: 3, Generation: 0}},
		},
	}
	got := structureTree(t, d)
	if len(got) != 1 || len(got[0].children) != 1 || !got[0].children[0].HasParent() || got[0].children[0].Parent() != (Ref{Object: 3, Generation: 0}) {
		t.Fatalf("structure parent = %#v", got)
	}
}

func TestStructureTreeReadsElementMetadata(t *testing.T) {
	d := &Document{
		trailer: Dict{Name("Root"): Ref{Object: 1, Generation: 0}},
		objects: map[Ref]Object{
			{Object: 1, Generation: 0}: Dict{Name("StructTreeRoot"): Ref{Object: 2, Generation: 0}},
			{Object: 2, Generation: 0}: Dict{Name("K"): Ref{Object: 3, Generation: 0}},
			{Object: 3, Generation: 0}: Dict{
				Name("S"):    Name("P"),
				Name("Lang"): String("en-US"),
				Name("E"):    String("abbr"),
				Name("C"):    Array{Name("old"), Number(1), Name("new"), Number(2)},
				Name("A"):    Array{newStream(Dict{Name("O"): Name("Layout")}, nil), Number(3)},
			},
		},
	}
	got := structureTree(t, d)
	if len(got) != 1 || got[0].Language() != "en-US" || got[0].AbbreviationExpansion() != "abbr" || got[0].ClassName() != "new" || got[0].AttributesCopy()[Name("O")] != Name("Layout") {
		t.Fatalf("structure metadata = %#v", got)
	}
}

func TestStructureTreeReadsMultipleUnversionedAttributeObjects(t *testing.T) {
	d := &Document{
		trailer: Dict{Name("Root"): Ref{Object: 1, Generation: 0}},
		objects: map[Ref]Object{
			{Object: 1, Generation: 0}: Dict{Name("StructTreeRoot"): Ref{Object: 2, Generation: 0}},
			{Object: 2, Generation: 0}: Dict{Name("K"): Ref{Object: 3, Generation: 0}},
			{Object: 3, Generation: 0}: Dict{
				Name("S"): Name("L"),
				Name("A"): Array{
					Dict{Name("O"): Name("List"), Name("ListNumbering"): Name("LowerRoman")},
					Number(1),
					newStream(Dict{Name("O"): Name("Layout"), Name("Placement"): Name("Inline")}, nil),
				},
			},
		},
	}
	got := structureTree(t, d)
	if len(got) != 1 {
		t.Fatalf("structure element count = %d, want 1", len(got))
	}
	attributes := got[0].AttributesCopy()
	if attributes[Name("O")] != Name("Layout") || attributes[Name("ListNumbering")] != Name("LowerRoman") || attributes[Name("Placement")] != Name("Inline") {
		t.Fatalf("structure attributes = %#v, want later Layout owner with LowerRoman numbering and Inline placement", attributes)
	}
}

func TestParentTreeDoesNotRetainOversizedValues(t *testing.T) {
	d := &Document{
		trailer: Dict{Name("Root"): Ref{Object: 1, Generation: 0}},
		objects: map[Ref]Object{
			{Object: 1, Generation: 0}: Dict{Name("StructTreeRoot"): Ref{Object: 2, Generation: 0}},
			{Object: 2, Generation: 0}: Dict{Name("ParentTree"): Ref{Object: 3, Generation: 0}},
			{Object: 3, Generation: 0}: Dict{Name("Nums"): Array{Number(0), String(make([]byte, parentTreeCacheLimit+1))}},
		},
	}
	value, ok, err := d.parentTreeValueChecked(0)
	if err != nil || !ok || value == nil {
		t.Fatalf("parent-tree value = %#v, ok=%v, err=%v", value, ok, err)
	}
	if len(d.parentTreeCache) != 0 || d.parentTreeCacheBytes != 0 {
		t.Fatalf("oversized parent-tree value entered cache: entries=%d bytes=%d", len(d.parentTreeCache), d.parentTreeCacheBytes)
	}
}

func TestParentTreeCacheDoesNotDoubleCountKeys(t *testing.T) {
	d := &Document{}
	d.parentTreeCache = map[int]Object{}
	value := Dict{Name("T"): String("paragraph")}
	d.cacheParentTreeValue(3, value)
	firstBytes := d.parentTreeCacheBytes
	d.cacheParentTreeValue(3, Dict{Name("T"): String("changed")})
	if d.parentTreeCacheBytes != firstBytes {
		t.Fatalf("repeated parent-tree key changed cache bytes: first=%d second=%d", firstBytes, d.parentTreeCacheBytes)
	}
	if got := string(d.parentTreeCache[3].(Dict)[Name("T")].(String)); got != "paragraph" {
		t.Fatalf("repeated parent-tree key replaced cached value: got %v", got)
	}
}

func TestPageStructureDoesNotRetainOversizedValues(t *testing.T) {
	d := &Document{
		trailer: Dict{Name("Root"): Ref{Object: 1, Generation: 0}},
		objects: map[Ref]Object{
			{Object: 1, Generation: 0}: Dict{Name("StructTreeRoot"): Ref{Object: 2, Generation: 0}},
			{Object: 2, Generation: 0}: Dict{Name("ParentTree"): Ref{Object: 3, Generation: 0}},
			{Object: 3, Generation: 0}: Dict{Name("Nums"): Array{Number(7), Array{Ref{Object: 4, Generation: 0}}}},
			{Object: 4, Generation: 0}: Dict{Name("S"): Name("P"), Name("T"): String(strings.Repeat("x", pageStructureCacheLimit+1))},
		},
	}
	page := Page{ref: Ref{Object: 9, Generation: 0}, dict: Dict{Name("StructParents"): Number(7)}}
	structure, err := page.Structure(d)
	if err != nil || len(structure.elements) != 1 {
		t.Fatalf("page structure = %#v, err = %v", structure, err)
	}
	if len(d.pageStructureCache) != 0 || d.pageStructureCacheBytes != 0 || d.pageStructureReady[page.ref] {
		t.Fatalf("oversized page structure entered cache: entries=%d bytes=%d ready=%v", len(d.pageStructureCache), d.pageStructureCacheBytes, d.pageStructureReady[page.ref])
	}
}

func TestStructureTreeIndexFindsNestedMCIDs(t *testing.T) {
	d := &Document{
		trailer: Dict{Name("Root"): Ref{Object: 1, Generation: 0}},
		objects: map[Ref]Object{
			{Object: 1, Generation: 0}: Dict{Name("StructTreeRoot"): Ref{Object: 2, Generation: 0}},
			{Object: 2, Generation: 0}: Dict{Name("K"): Ref{Object: 3, Generation: 0}},
			{Object: 3, Generation: 0}: Dict{Name("S"): Name("Document"), Name("K"): Array{Ref{Object: 4, Generation: 0}}},
			{Object: 4, Generation: 0}: Dict{Name("S"): Name("P"), Name("K"): Number(5)},
		},
	}
	index, err := d.StructureTreeIndex()
	if err != nil {
		t.Fatal(err)
	}
	if len(index.roots) != 1 || len(index.byMCID[5]) != 1 || index.byMCID[5][0].Role() != "P" {
		t.Fatalf("structure index = %#v", index)
	}
}

func TestStructureTreeRecognizesMCRAndOBJNodes(t *testing.T) {
	d := &Document{
		trailer: Dict{Name("Root"): Ref{Object: 1, Generation: 0}},
		objects: map[Ref]Object{
			{Object: 1, Generation: 0}: Dict{Name("StructTreeRoot"): Ref{Object: 2, Generation: 0}},
			{Object: 2, Generation: 0}: Dict{Name("K"): Array{Ref{Object: 3, Generation: 0}, Ref{Object: 4, Generation: 0}}},
			{Object: 3, Generation: 0}: Dict{Name("Type"): Name("MCR"), Name("MCID"): Number(2)},
			{Object: 4, Generation: 0}: Dict{Name("Type"): Name("OBJ"), Name("Obj"): Ref{Object: 8, Generation: 0}},
			{Object: 8, Generation: 0}: Dict{Name("Type"): Name("Annot"), Name("Contents"): String("linked")},
		},
	}
	got := structureTree(t, d)
	if len(got) != 2 || !got[0].IsMCR() || !got[0].HasMCID() || got[0].MCID() != 2 || !got[1].HasObject() || got[1].ObjectRef().Object != 8 {
		t.Fatalf("structure references = %#v", got)
	}
	object, ok := got[1].Object(d)
	if !ok || !reflect.DeepEqual(object, Dict{Name("Type"): Name("Annot"), Name("Contents"): String("linked")}) {
		t.Fatalf("structure object = %#v, ok=%v", object, ok)
	}
	object.(Dict)[Name("Contents")] = String("mutated")
	resolved, _ := got[1].Object(d)
	if string(resolved.(Dict)[Name("Contents")].(String)) != "linked" {
		t.Fatalf("structure object was not isolated: %#v", resolved)
	}
}

func TestStructureElementSeparatesContentChildren(t *testing.T) {
	d := &Document{
		trailer: Dict{Name("Root"): Ref{Object: 1, Generation: 0}},
		objects: map[Ref]Object{
			{Object: 1, Generation: 0}: Dict{Name("StructTreeRoot"): Ref{Object: 2, Generation: 0}},
			{Object: 2, Generation: 0}: Dict{Name("K"): Ref{Object: 3, Generation: 0}},
			{Object: 3, Generation: 0}: Dict{Name("S"): Name("P"), Name("K"): Array{
				Number(4),
				Dict{Name("Type"): Name("MCR"), Name("MCID"): Number(5), Name("Pg"): Ref{Object: 9, Generation: 0}},
				Dict{Name("Type"): Name("OBJR"), Name("Obj"): Ref{Object: 10, Generation: 0}},
			}},
		},
	}
	got := structureTree(t, d)
	if len(got) != 1 || len(got[0].contents) != 3 || got[0].contents[0].MCID() != 4 || got[0].contents[1].MCID() != 5 || !got[0].contents[2].HasObject() {
		t.Fatalf("structure content children = %#v", got)
	}
}

func TestStructureContentsSeqPreservesDepthFirstKOrder(t *testing.T) {
	d := &Document{
		trailer: Dict{Name("Root"): Ref{Object: 1, Generation: 0}},
		objects: map[Ref]Object{
			{Object: 1, Generation: 0}: Dict{Name("StructTreeRoot"): Ref{Object: 2, Generation: 0}},
			{Object: 2, Generation: 0}: Dict{Name("K"): Ref{Object: 3, Generation: 0}},
			{Object: 3, Generation: 0}: Dict{Name("S"): Name("Document"), Name("K"): Array{
				Number(1), Dict{Name("S"): Name("P"), Name("K"): Number(2)}, Number(3),
			}},
		},
	}
	root := structureTree(t, d)[0]
	var got []int
	for content, err := range root.ContentsSeq() {
		if err != nil {
			t.Fatal(err)
		}
		got = append(got, content.MCID())
	}
	if want := []int{1, 2, 3}; !reflect.DeepEqual(got, want) {
		t.Fatalf("content order = %v, want %v", got, want)
	}
}

func TestLazyStructureContentsSeqIncludesDescendants(t *testing.T) {
	d := &Document{
		trailer: Dict{Name("Root"): Ref{Object: 1, Generation: 0}},
		objects: map[Ref]Object{
			{Object: 1, Generation: 0}: Dict{Name("StructTreeRoot"): Ref{Object: 2, Generation: 0}},
			{Object: 2, Generation: 0}: Dict{Name("K"): Ref{Object: 3, Generation: 0}},
			{Object: 3, Generation: 0}: Dict{Name("S"): Name("Document"), Name("K"): Array{
				Number(1), Ref{Object: 4, Generation: 0},
			}},
			{Object: 4, Generation: 0}: Dict{Name("S"): Name("P"), Name("K"): Number(2)},
		},
	}
	var root StructElement
	for element, err := range d.StructureTreeSeq() {
		if err != nil {
			t.Fatal(err)
		}
		root = element
		break
	}
	var got []int
	for content, err := range root.ContentsSeq() {
		if err != nil {
			t.Fatal(err)
		}
		got = append(got, content.MCID())
	}
	if len(root.children) != 0 {
		t.Fatalf("lazy contents materialized children: %#v", root.children)
	}
	if want := []int{1, 2}; !reflect.DeepEqual(got, want) {
		t.Fatalf("lazy structure contents = %v, want %v", got, want)
	}
}

func TestStructureTreeSequenceIsRepeatableAndStopsEarly(t *testing.T) {
	d := &Document{
		trailer: Dict{Name("Root"): Ref{Object: 1, Generation: 0}},
		objects: map[Ref]Object{
			{Object: 1, Generation: 0}: Dict{Name("StructTreeRoot"): Ref{Object: 2, Generation: 0}},
			{Object: 2, Generation: 0}: Dict{Name("K"): Array{Ref{Object: 3, Generation: 0}, Ref{Object: 4, Generation: 0}}},
			{Object: 3, Generation: 0}: Dict{Name("S"): Name("P"), Name("T"): String("first")},
			{Object: 4, Generation: 0}: Dict{Name("S"): Name("P"), Name("T"): String("second")},
		},
	}

	count := 0
	for element, err := range d.StructureTreeSeq() {
		if err != nil {
			t.Fatal(err)
		}
		if element.Title() != "first" {
			t.Fatalf("first structure element = %#v", element)
		}
		count++
		break
	}
	var titles []string
	for element, err := range d.StructureTreeSeq() {
		if err != nil {
			t.Fatal(err)
		}
		titles = append(titles, element.Title())
	}
	if count != 1 || len(titles) != 2 || titles[1] != "second" {
		t.Fatalf("sequence = %d, %#v", count, titles)
	}
}

func TestStructureTreeFinalizeDoesNotShareElementDictionaries(t *testing.T) {
	d := &Document{
		trailer: Dict{Name("Root"): Ref{Object: 1}},
		objects: map[Ref]Object{
			{Object: 1}: Dict{Name("StructTreeRoot"): Ref{Object: 2}},
			{Object: 2}: Dict{Name("K"): Ref{Object: 3}},
			{Object: 3}: Dict{Name("S"): Name("P"), Name("T"): String("original")},
		},
	}
	first, err := firstStructureElement(d.StructureTreeSeq())
	if err != nil {
		t.Fatal(err)
	}
	firstDict := first.Finalize().DictCopy()
	firstDict[Name("T")] = String("changed")
	second, err := firstStructureElement(d.StructureTreeSeq())
	if err != nil {
		t.Fatal(err)
	}
	secondDict := second.DictCopy()
	value, _ := secondDict[Name("T")].(String)
	if string(value) != "original" {
		t.Fatalf("structure tree dictionary was exposed: %#v", secondDict)
	}
}

func TestStructureTreeIndexDoesNotShareRootElements(t *testing.T) {
	d := &Document{
		trailer: Dict{Name("Root"): Ref{Object: 1}},
		objects: map[Ref]Object{
			{Object: 1}: Dict{Name("StructTreeRoot"): Ref{Object: 2}},
			{Object: 2}: Dict{Name("K"): Ref{Object: 3}},
			{Object: 3}: Dict{Name("S"): Name("P"), Name("T"): String("original"), Name("K"): Number(4)},
		},
	}
	index, err := d.StructureTreeIndex()
	if err != nil {
		t.Fatal(err)
	}
	if len(index.roots) != 1 || len(index.byMCID[4]) != 1 {
		t.Fatalf("structure tree index = %#v", index)
	}
	snapshot := index.Finalize()
	snapshotDict := snapshot.roots[0].DictCopy()
	snapshotDict[Name("T")] = String("changed")
	indexDict := index.byMCID[4][0].DictCopy()
	value, _ := indexDict[Name("T")].(String)
	if string(value) != "original" {
		t.Fatalf("structure index shared root element: %#v", indexDict)
	}
}

func TestPageStructureQueriesRequireFinalizeForElements(t *testing.T) {
	root := newStructElementValue(structuredata.ElementSpec{Role: "P", Dict: Dict{Name("T"): String("original")}})
	indexed := newStructElementValue(structuredata.ElementSpec{Role: "P", Dict: Dict{Name("T"): String("original")}})
	structure := PageStructure{
		elements: []StructElement{root},
	}
	structure.byMCID = make(map[int][]StructElement)
	structure.byMCID[4] = []StructElement{indexed}
	at := structure.ByMCID(4)
	atDict := at[0].DictCopy()
	atDict[Name("T")] = String("changed")
	found, ok := structure.Find("P")
	if !ok {
		t.Fatal("expected structure element")
	}
	foundDict := found.DictCopy()
	foundDict[Name("T")] = String("changed")
	for element, err := range structure.FindAllSeq("P") {
		if err != nil {
			t.Fatal(err)
		}
		snapshotDict := element.Finalize().DictCopy()
		snapshotDict[Name("T")] = String("changed")
	}
	rootDict := structure.elements[0].DictCopy()
	indexedDict := structure.byMCID[4][0].DictCopy()
	if string(rootDict[Name("T")].(String)) != "original" || string(indexedDict[Name("T")].(String)) != "original" {
		t.Fatal("page structure query exposed source elements")
	}
}

func TestPageStructureFinalizePreservesAbsentMCIDIndex(t *testing.T) {
	snapshot := (PageStructure{}).Finalize()
	if snapshot.byMCID != nil {
		t.Fatalf("zero PageStructure finalized with an allocated MCID index: %#v", snapshot.byMCID)
	}
}

func TestPageStructureMatchesPlayaParentTreeSequence(t *testing.T) {
	p := newStructElementValue(structuredata.ElementSpec{Role: "P"})
	figure := newStructElementValue(structuredata.ElementSpec{Role: "Figure"})
	caption := newStructElementValue(structuredata.ElementSpec{Role: "Caption"})
	structure := PageStructure{slots: [][]StructElement{
		nil,
		{p},
		{figure, caption},
	}}

	if got := structure.Len(); got != 3 {
		t.Fatalf("PageStructure.Len() = %d, want 3", got)
	}
	if got := structure.At(0); got != nil {
		t.Fatalf("empty ParentTree slot = %#v, want nil", got)
	}
	if got := structure.At(1); len(got) != 1 || got[0].Role() != "P" {
		t.Fatalf("single ParentTree slot = %#v", got)
	}
	if got := structure.At(2); len(got) != 2 || got[0].Role() != "Figure" || got[1].Role() != "Caption" {
		t.Fatalf("multi-element ParentTree slot = %#v", got)
	}
	if got := structure.At(3); got != nil {
		t.Fatalf("out-of-range ParentTree slot = %#v, want nil", got)
	}
}

func TestStructurePathClonePreservesEmptyAllocatedMap(t *testing.T) {
	source := map[Ref]bool{}
	clone := cloneStructurePath(source)
	if clone == nil {
		t.Fatal("empty allocated structure path was collapsed to nil")
	}
}

func TestStructElementFinalizeDoesNotShareChildren(t *testing.T) {
	child := newStructElementValue(structuredata.ElementSpec{Role: "Span", Dict: Dict{Name("T"): String("original")}})
	parent := StructElement{children: []StructElement{child}, childrenReady: true, contents: []StructureContent{newStructureContent(structuredata.ContentSpec{Dict: Dict{Name("MCID"): Number(1)}})}, contentsReady: true}
	for element, err := range parent.ChildrenSeq() {
		if err != nil {
			t.Fatal(err)
		}
		copy := element.Finalize().DictCopy()
		copy[Name("T")] = String("changed")
	}
	for content, err := range parent.ContentsSeq() {
		if err != nil {
			t.Fatal(err)
		}
		contentCopy := content.Finalize().DictCopy()
		contentCopy[Name("MCID")] = Number(2)
	}
	if value, _ := parent.children[0].DictCopy()[Name("T")].(String); string(value) != "original" || parent.contents[0].DictCopy()[Name("MCID")] != Number(1) {
		t.Fatal("structure element lazy query exposed source values")
	}
}

func TestStructElementJSONIncludesPrivateMetadata(t *testing.T) {
	element := newStructElementValue(structuredata.ElementSpec{
		Type: "StructElem", StructureType: "P", Role: "P", RawRole: "Paragraph",
		Title: "title", Language: "en", Alt: "alt", ActualText: "actual",
		Abbreviation: "abbr", ClassName: "class", Page: Ref{Object: 7}, HasPage: true,
		Parent: Ref{Object: 8}, HasParent: true, MCID: 3, HasMCID: true,
		IsMCR: true, ObjectRef: Ref{Object: 9}, HasObject: true,
		BBox: [4]float64{1, 2, 3, 4}, HasBBox: true,
	})
	data, err := json.Marshal(element)
	if err != nil {
		t.Fatal(err)
	}
	got := string(data)
	for _, want := range []string{`"Type":"StructElem"`, `"StructureType":"P"`, `"Role":"P"`, `"RawRole":"Paragraph"`, `"Title":"title"`, `"HasPage":true`, `"HasParent":true`, `"HasMCID":true`, `"IsMCR":true`, `"HasObject":true`, `"HasBBox":true`, `"BBox":[1,2,3,4]`} {
		if !strings.Contains(got, want) {
			t.Fatalf("structure JSON omitted %s: %s", want, got)
		}
	}
}

func firstStructureElement(sequence iter.Seq2[StructElement, error]) (StructElement, error) {
	for element, err := range sequence {
		return element, err
	}
	return StructElement{}, nil
}

func TestStructureTreeSequenceDefersNestedChildren(t *testing.T) {
	d := &Document{
		trailer: Dict{Name("Root"): Ref{Object: 1, Generation: 0}},
		objects: map[Ref]Object{
			{Object: 1, Generation: 0}: Dict{Name("StructTreeRoot"): Ref{Object: 2, Generation: 0}},
			{Object: 2, Generation: 0}: Dict{Name("K"): Ref{Object: 3, Generation: 0}},
			{Object: 3, Generation: 0}: Dict{Name("S"): Name("Document"), Name("K"): Ref{Object: 4, Generation: 0}},
			{Object: 4, Generation: 0}: Dict{Name("S"): Name("Sect"), Name("K"): Ref{Object: 5, Generation: 0}},
			{Object: 5, Generation: 0}: Dict{Name("S"): Name("P"), Name("T"): String("paragraph")},
		},
	}
	var root StructElement
	for element, err := range d.StructureTreeSeq() {
		if err != nil {
			t.Fatal(err)
		}
		root = element
		break
	}
	if len(root.children) != 0 {
		t.Fatalf("nested children were materialized: %#v", root.children)
	}
	var children []StructElement
	for child, err := range root.ChildrenSeq() {
		if err != nil {
			t.Fatal(err)
		}
		children = append(children, child)
	}
	if len(children) != 1 || children[0].Role() != "Sect" {
		t.Fatalf("lazy children = %#v", children)
	}
	var roles []string
	for element, err := range root.FindAllSeq("") {
		if err != nil {
			t.Fatal(err)
		}
		roles = append(roles, element.Role())
	}
	if len(roles) != 2 || roles[0] != "Sect" || roles[1] != "P" {
		t.Fatalf("lazy descendant roles = %#v", roles)
	}
}

func TestStructureLazyChildrenAreReusedAcrossElementCopies(t *testing.T) {
	d := &Document{
		trailer: Dict{Name("Root"): Ref{Object: 1, Generation: 0}},
		objects: map[Ref]Object{
			{Object: 1, Generation: 0}: Dict{Name("StructTreeRoot"): Ref{Object: 2, Generation: 0}},
			{Object: 2, Generation: 0}: Dict{Name("K"): Ref{Object: 3, Generation: 0}},
			{Object: 3, Generation: 0}: Dict{Name("S"): Name("Document"), Name("K"): Ref{Object: 4, Generation: 0}},
			{Object: 4, Generation: 0}: Dict{Name("S"): Name("P"), Name("T"): String("paragraph")},
		},
	}
	root, err := firstStructureElement(d.StructureTreeSeq())
	if err != nil {
		t.Fatal(err)
	}
	children, err := root.ChildrenCopy()
	if err != nil || len(children) != 1 || children[0].Role() != "P" {
		t.Fatalf("first lazy children = %#v, err=%v", children, err)
	}
	d.objects[Ref{Object: 4, Generation: 0}] = Number(1)
	children, err = root.ChildrenCopy()
	if err != nil || len(children) != 1 || children[0].Role() != "P" {
		t.Fatalf("cached lazy children = %#v, err=%v", children, err)
	}
}

func TestLazyStructureChildrenStopAtAncestorCycle(t *testing.T) {
	d := &Document{
		trailer: Dict{Name("Root"): Ref{Object: 1}},
		objects: map[Ref]Object{
			{Object: 1}: Dict{Name("StructTreeRoot"): Ref{Object: 2}},
			{Object: 2}: Dict{Name("K"): Ref{Object: 3}},
			{Object: 3}: Dict{Name("S"): Name("Document"), Name("K"): Ref{Object: 4}},
			{Object: 4}: Dict{Name("S"): Name("Sect"), Name("K"): Ref{Object: 3}},
		},
	}

	root, err := firstStructureElement(d.StructureTreeSeq())
	if err != nil {
		t.Fatal(err)
	}
	var child StructElement
	count := 0
	for value, childErr := range root.ChildrenSeq() {
		if childErr != nil {
			t.Fatal(childErr)
		}
		child = value
		count++
	}
	if count != 1 || child.Role() != "Sect" {
		t.Fatalf("first-level children = %d, %#v", count, child)
	}
	grandchildren := 0
	for _, childErr := range child.ChildrenSeq() {
		if childErr != nil {
			t.Fatal(childErr)
		}
		grandchildren++
	}
	if grandchildren != 0 {
		t.Fatalf("ancestor cycle was not stopped: %d grandchildren", grandchildren)
	}
}

func TestStructureTreeSequenceReportsMalformedRoot(t *testing.T) {
	d := &Document{
		trailer: Dict{Name("Root"): Ref{Object: 1, Generation: 0}},
		objects: map[Ref]Object{
			{Object: 1, Generation: 0}: Dict{Name("StructTreeRoot"): Ref{Object: 2, Generation: 0}},
			{Object: 2, Generation: 0}: Dict{Name("K"): String("invalid")},
		},
	}
	for _, err := range d.StructureTreeSeq() {
		if err == nil {
			t.Fatal("malformed structure root produced a value")
		}
		if !strings.Contains(err.Error(), "structure tree root is not a dictionary") {
			t.Fatalf("error = %v", err)
		}
		break
	}
	if !d.structureRootErrReady || d.structureRootErr == nil {
		t.Fatalf("structure root error was not cached: ready=%v err=%v", d.structureRootErrReady, d.structureRootErr)
	}
	for _, err := range d.StructureTreeSeq() {
		if err == nil || err != d.structureRootErr {
			t.Fatalf("cached structure root error = %v, want %v", err, d.structureRootErr)
		}
		return
	}
	t.Fatal("cached malformed structure root produced no error")
}

func TestStructureTreeSequenceReportsUnresolvedKReference(t *testing.T) {
	d := &Document{
		trailer: Dict{Name("Root"): Dict{Name("StructTreeRoot"): Dict{Name("K"): Ref{Object: 99}}}},
	}
	for _, err := range d.StructureTreeSeq() {
		if err == nil || err.Error() != "playa: structure tree root could not be resolved" {
			t.Fatalf("unresolved structure K error = %v", err)
		}
		return
	}
	t.Fatal("unresolved structure K reference produced no error")
}

func TestStructElementSequencesReportMalformedChildren(t *testing.T) {
	d := &Document{
		trailer: Dict{Name("Root"): Ref{Object: 1, Generation: 0}},
		objects: map[Ref]Object{
			{Object: 1, Generation: 0}: Dict{Name("StructTreeRoot"): Ref{Object: 2, Generation: 0}},
			{Object: 2, Generation: 0}: Dict{Name("K"): Ref{Object: 3, Generation: 0}},
			{Object: 3, Generation: 0}: Dict{Name("S"): Name("P"), Name("K"): String("invalid")},
		},
	}
	root, err := firstStructureElement(d.StructureTreeSeq())
	if err != nil {
		t.Fatal(err)
	}
	childError := false
	for _, err := range root.ChildrenSeq() {
		if err == nil {
			t.Fatal("malformed structure child produced a value")
		}
		if !strings.Contains(err.Error(), "structure child is not a dictionary") {
			t.Fatalf("children error = %v", err)
		}
		childError = true
		break
	}
	if !childError {
		t.Fatal("malformed structure child produced no error")
	}
	contentError := false
	for _, err := range root.ContentsSeq() {
		if err == nil {
			t.Fatal("malformed structure content produced a value")
		}
		if !strings.Contains(err.Error(), "structure content item is not a dictionary") {
			t.Fatalf("contents error = %v", err)
		}
		contentError = true
		break
	}
	if !contentError {
		t.Fatal("malformed structure content produced no error")
	}
	if children, err := root.ChildrenCopy(); err == nil || children != nil {
		t.Fatalf("malformed structure children copy = %#v, err=%v", children, err)
	}
	if contents, err := root.ContentsCopy(); err == nil || contents != nil {
		t.Fatalf("malformed structure contents copy = %#v, err=%v", contents, err)
	}
	if _, err := root.MarshalJSON(); err == nil {
		t.Fatal("malformed structure JSON produced no error")
	}
	if snapshot, err := root.FinalizeWithError(); err == nil || snapshot.Role() != "" {
		t.Fatalf("malformed structure finalize = %#v, err = %v", snapshot, err)
	}
}

func TestStructElementSequencesReportUnresolvedChildren(t *testing.T) {
	d := &Document{objects: map[Ref]Object{}}
	element := StructElement{document: d, childStart: Ref{Object: 99}}
	for _, err := range element.ChildrenSeq() {
		if err == nil || err.Error() != "playa: structure child could not be resolved" {
			t.Fatalf("unresolved structure child error = %v", err)
		}
		return
	}
	t.Fatal("unresolved structure child produced no error")
}

func TestStructElementContentsSequenceReportsUnresolvedChildren(t *testing.T) {
	d := &Document{objects: map[Ref]Object{}}
	element := StructElement{document: d, childStart: Ref{Object: 99}}
	for _, err := range element.ContentsSeq() {
		if err == nil || err.Error() != "playa: structure content could not be resolved" {
			t.Fatalf("unresolved structure content error = %v", err)
		}
		return
	}
	t.Fatal("unresolved structure content produced no error")
}

func TestStructureAggregatesFinalizeWithErrorPropagatesNestedFailures(t *testing.T) {
	d := &Document{}
	element := StructElement{document: d, childStart: String("invalid")}
	entry := PageStructureEntry{index: 1, element: &element, elements: []StructElement{element}}
	structure := PageStructure{elements: []StructElement{element}, byMCID: map[int][]StructElement{1: {element}}}
	index := StructureIndex{roots: []StructElement{element}, byMCID: map[int][]StructElement{1: {element}}}

	if snapshot, err := entry.FinalizeWithError(); err == nil || snapshot.element != nil {
		t.Fatalf("entry finalize = %#v, err=%v", snapshot, err)
	}
	if snapshot, err := structure.FinalizeWithError(); err == nil || snapshot.elements != nil {
		t.Fatalf("page structure finalize = %#v, err=%v", snapshot, err)
	}
	if snapshot, err := index.FinalizeWithError(); err == nil || snapshot.roots != nil {
		t.Fatalf("structure index finalize = %#v, err=%v", snapshot, err)
	}
}

func TestStructElementFinalizeWithErrorPropagatesNestedChildFailure(t *testing.T) {
	child := StructElement{document: &Document{}, childStart: String("invalid")}
	parent := StructElement{children: []StructElement{child}, childrenReady: true}
	if snapshot, err := parent.FinalizeWithError(); err == nil || snapshot.StructureType() != "" {
		t.Fatalf("nested child finalize = %#v, err=%v", snapshot, err)
	}
}

func TestStructureTreeRejectsMalformedNestedChildren(t *testing.T) {
	d := &Document{
		trailer: Dict{Name("Root"): Ref{Object: 1}},
		objects: map[Ref]Object{
			{Object: 1}: Dict{Name("StructTreeRoot"): Ref{Object: 2}},
			{Object: 2}: Dict{Name("K"): Ref{Object: 3}},
			{Object: 3}: Dict{Name("S"): Name("P"), Name("K"): String("invalid")},
		},
	}
	got, err := d.StructureTree()
	if err == nil || got != nil {
		t.Fatalf("structure tree from malformed nested child = %#v, err=%v", got, err)
	}
	if index, err := d.StructureTreeIndex(); err == nil || index.roots != nil {
		t.Fatalf("structure index from malformed nested child = %#v, err=%v", index, err)
	}
}

func TestStructureMaterializationIsReusable(t *testing.T) {
	d := &Document{
		trailer: Dict{Name("Root"): Ref{Object: 1, Generation: 0}},
		objects: map[Ref]Object{
			{Object: 1, Generation: 0}: Dict{Name("StructTreeRoot"): Ref{Object: 2, Generation: 0}},
			{Object: 2, Generation: 0}: Dict{Name("K"): Ref{Object: 3, Generation: 0}},
			{Object: 3, Generation: 0}: Dict{Name("S"): Name("Document"), Name("K"): Ref{Object: 4, Generation: 0}},
			{Object: 4, Generation: 0}: Dict{Name("S"): Name("P"), Name("T"): String("paragraph")},
		},
	}
	root := structureTree(t, d)[0]
	if len(root.children) != 1 {
		t.Fatalf("materialized children = %#v", root.children)
	}
	delete(d.objects, Ref{Object: 4, Generation: 0})
	var roles []string
	for child, err := range root.ChildrenSeq() {
		if err != nil {
			t.Fatal(err)
		}
		roles = append(roles, child.Role())
	}
	if len(roles) != 1 || roles[0] != "P" {
		t.Fatalf("cached structure children = %#v", roles)
	}
}

func TestStructureElementChildrenExcludeMarkedContentReferences(t *testing.T) {
	d := &Document{
		trailer: Dict{Name("Root"): Ref{Object: 1, Generation: 0}},
		objects: map[Ref]Object{
			{Object: 1, Generation: 0}: Dict{Name("StructTreeRoot"): Ref{Object: 2, Generation: 0}},
			{Object: 2, Generation: 0}: Dict{Name("K"): Ref{Object: 3, Generation: 0}},
			{Object: 3, Generation: 0}: Dict{Name("S"): Name("P"), Name("K"): Array{
				Dict{Name("Type"): Name("MCR"), Name("MCID"): Number(4)},
				Dict{Name("S"): Name("Span"), Name("T"): String("child")},
			}},
		},
	}
	var root StructElement
	for element, err := range d.StructureTreeSeq() {
		if err != nil {
			t.Fatal(err)
		}
		root = element
		break
	}
	var got []StructElement
	for child, err := range root.ChildrenSeq() {
		if err != nil {
			t.Fatal(err)
		}
		got = append(got, child)
	}
	if len(got) != 1 || got[0].Role() != "Span" {
		t.Fatalf("lazy children = %#v", got)
	}
}

func TestStructureElementFindsDescendantsDepthFirst(t *testing.T) {
	d := &Document{
		trailer: Dict{Name("Root"): Ref{Object: 1, Generation: 0}},
		objects: map[Ref]Object{
			{Object: 1, Generation: 0}: Dict{Name("StructTreeRoot"): Ref{Object: 2, Generation: 0}},
			{Object: 2, Generation: 0}: Dict{Name("K"): Ref{Object: 3, Generation: 0}},
			{Object: 3, Generation: 0}: Dict{Name("S"): Name("Document"), Name("K"): Array{Ref{Object: 4, Generation: 0}, Ref{Object: 5, Generation: 0}}},
			{Object: 4, Generation: 0}: Dict{Name("S"): Name("Sect"), Name("K"): Ref{Object: 6, Generation: 0}},
			{Object: 5, Generation: 0}: Dict{Name("S"): Name("P")},
			{Object: 6, Generation: 0}: Dict{Name("S"): Name("P")},
		},
	}
	root := structureTree(t, d)[0]
	var roles []string
	for node, err := range root.FindAllSeq("") {
		if err != nil {
			t.Fatal(err)
		}
		roles = append(roles, node.Role())
	}
	if len(roles) != 3 || roles[0] != "Sect" || roles[1] != "P" || roles[2] != "P" {
		t.Fatalf("descendant order = %#v", roles)
	}
	if node, ok := root.Find("P"); !ok || node.Role() != "P" {
		t.Fatalf("first matching descendant = %#v, %v", node, ok)
	}
}

func TestPageStructureSequenceMapsParentTreeSlots(t *testing.T) {
	d := &Document{
		trailer: Dict{Name("Root"): Ref{Object: 1, Generation: 0}},
		objects: map[Ref]Object{
			{Object: 1, Generation: 0}: Dict{Name("StructTreeRoot"): Ref{Object: 2, Generation: 0}},
			{Object: 2, Generation: 0}: Dict{Name("ParentTree"): Ref{Object: 5, Generation: 0}},
			{Object: 3, Generation: 0}: Dict{Name("S"): Name("P"), Name("T"): String("first")},
			{Object: 4, Generation: 0}: Dict{Name("S"): Name("Figure"), Name("T"): String("third")},
			{Object: 5, Generation: 0}: Dict{Name("Nums"): Array{Number(7), Array{Ref{Object: 3, Generation: 0}, Null{}, Ref{Object: 4, Generation: 0}}}},
		},
	}
	page := Page{dict: Dict{Name("StructParents"): Number(7)}}
	var entries []PageStructureEntry
	for entry, err := range page.StructureSeq(d) {
		if err != nil {
			t.Fatal(err)
		}
		entries = append(entries, entry)
	}
	if len(entries) != 3 || entries[0].element == nil || entries[0].element.Title() != "first" || entries[1].element != nil || entries[2].element == nil || entries[2].element.Role() != "Figure" {
		t.Fatalf("page structure = %#v", entries)
	}
}

func TestPageStructureSequenceRepeatsSharedElementAcrossParentTreeSlots(t *testing.T) {
	d := &Document{
		trailer: Dict{Name("Root"): Ref{Object: 1}},
		objects: map[Ref]Object{
			{Object: 1}: Dict{Name("StructTreeRoot"): Ref{Object: 2}},
			{Object: 2}: Dict{Name("ParentTree"): Ref{Object: 5}},
			{Object: 3}: Dict{Name("S"): Name("P"), Name("T"): String("shared")},
			{Object: 5}: Dict{Name("Nums"): Array{Number(7), Array{Ref{Object: 3}, Ref{Object: 3}, Null{}}}},
		},
	}

	page := Page{dict: Dict{Name("StructParents"): Number(7)}}
	var entries []PageStructureEntry
	for entry, err := range page.StructureSeq(d) {
		if err != nil {
			t.Fatal(err)
		}
		entries = append(entries, entry)
	}
	if len(entries) != 3 || entries[0].element == nil || entries[1].element == nil || entries[2].element != nil {
		t.Fatalf("shared parent tree entries = %#v", entries)
	}
	if entries[0].element.Title() != "shared" || entries[1].element.Title() != "shared" {
		t.Fatalf("shared parent tree elements = %#v", entries)
	}
}

func TestPageParentKeyRequiresNonNegativeInteger(t *testing.T) {
	d := &Document{}
	if key, ok := (Page{dict: Dict{Name("StructParents"): Number(7)}}).ParentKey(d); !ok || key != 7 {
		t.Fatalf("integer StructParents = %d, %v", key, ok)
	}
	for _, value := range []Number{7.5, -1} {
		if key, ok := (Page{dict: Dict{Name("StructParents"): value}}).ParentKey(d); ok {
			t.Fatalf("invalid StructParents %v accepted as %d", value, key)
		}
	}
}

func TestPageStructureSequenceReportsMalformedStructParents(t *testing.T) {
	page := Page{dict: Dict{Name("StructParents"): Ref{Object: 99}}}
	for _, err := range page.StructureSeq(&Document{}) {
		if err == nil || !strings.Contains(err.Error(), "StructParents could not be resolved") {
			t.Fatalf("malformed StructParents error = %v", err)
		}
		return
	}
	t.Fatal("malformed StructParents produced no error")
}

func TestPageStructureSequenceReportsMalformedParentTree(t *testing.T) {
	tests := []struct {
		name string
		tree Dict
		want string
	}{
		{name: "odd Nums", tree: Dict{Name("Nums"): Array{Number(7)}}, want: "unmatched key"},
		{name: "non-integer key", tree: Dict{Name("Nums"): Array{String("seven"), Array{}}}, want: "key 0 is not an integer"},
		{name: "non-array Kids", tree: Dict{Name("Kids"): Number(1)}, want: "Kids is not an array"},
		{name: "non-array value", tree: Dict{Name("Nums"): Array{Number(7), Dict{}}}, want: "ParentTree value is not an array"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			d := &Document{
				trailer: Dict{Name("Root"): Ref{Object: 1, Generation: 0}},
				objects: map[Ref]Object{
					{Object: 1, Generation: 0}: Dict{Name("StructTreeRoot"): Ref{Object: 2, Generation: 0}},
					{Object: 2, Generation: 0}: Dict{Name("ParentTree"): test.tree},
				},
			}
			page := Page{dict: Dict{Name("StructParents"): Number(7)}}
			for _, err := range page.StructureSeq(d) {
				if err == nil {
					t.Fatal("malformed ParentTree produced a value")
				}
				if !strings.Contains(err.Error(), test.want) {
					t.Fatalf("error = %v, want substring %q", err, test.want)
				}
				return
			}
			t.Fatal("malformed ParentTree produced no error")
		})
	}
}

func TestPageStructureSequenceRejectsInvalidParentTreeKeys(t *testing.T) {
	for name, nums := range map[string]Array{
		"negative": {Number(-1), Array{}},
		"unsorted": {Number(8), Array{}, Number(7), Array{}},
	} {
		t.Run(name, func(t *testing.T) {
			d := &Document{
				trailer: Dict{Name("Root"): Ref{Object: 1, Generation: 0}},
				objects: map[Ref]Object{
					{Object: 1, Generation: 0}: Dict{Name("StructTreeRoot"): Ref{Object: 2, Generation: 0}},
					{Object: 2, Generation: 0}: Dict{Name("ParentTree"): Dict{Name("Nums"): nums}},
				},
			}
			page := Page{dict: Dict{Name("StructParents"): Number(7)}}
			for _, err := range page.StructureSeq(d) {
				if err == nil {
					t.Fatal("invalid ParentTree key produced a value")
				}
				return
			}
			t.Fatal("invalid ParentTree key produced no error")
		})
	}
}

func TestPageStructureSequenceRejectsInvalidParentTreeLimits(t *testing.T) {
	for name, tree := range map[string]Dict{
		"reversed": {Name("Limits"): Array{Number(8), Number(7)}},
		"outside child": {
			Name("Limits"): Array{Number(0), Number(4)},
			Name("Kids"):   Array{Dict{Name("Limits"): Array{Number(5), Number(6)}}},
		},
	} {
		t.Run(name, func(t *testing.T) {
			d := &Document{
				trailer: Dict{Name("Root"): Ref{Object: 1, Generation: 0}},
				objects: map[Ref]Object{
					{Object: 1, Generation: 0}: Dict{Name("StructTreeRoot"): Ref{Object: 2, Generation: 0}},
					{Object: 2, Generation: 0}: Dict{Name("ParentTree"): tree},
				},
			}
			page := Page{dict: Dict{Name("StructParents"): Number(7)}}
			for _, err := range page.StructureSeq(d) {
				if err == nil {
					t.Fatal("invalid ParentTree limits produced a value")
				}
				return
			}
			t.Fatal("invalid ParentTree limits produced no error")
		})
	}
}

func TestPageStructureSequenceRejectsMixedParentTreeNode(t *testing.T) {
	d := &Document{
		trailer: Dict{Name("Root"): Ref{Object: 1, Generation: 0}},
		objects: map[Ref]Object{
			{Object: 1, Generation: 0}: Dict{Name("StructTreeRoot"): Ref{Object: 2, Generation: 0}},
			{Object: 2, Generation: 0}: Dict{Name("ParentTree"): Dict{
				Name("Nums"): Array{Number(7), Array{}},
				Name("Kids"): Array{Dict{Name("Nums"): Array{Number(7), Array{}}}},
			}},
		},
	}
	page := Page{dict: Dict{Name("StructParents"): Number(7)}}
	for _, err := range page.StructureSeq(d) {
		if err == nil || !strings.Contains(err.Error(), "cannot contain both Nums and Kids") {
			t.Fatalf("mixed ParentTree node error = %v", err)
		}
		return
	}
	t.Fatal("mixed ParentTree node produced no error")
}

func TestPageStructureSequenceReportsUnresolvedParentTreeSlot(t *testing.T) {
	d := &Document{
		trailer: Dict{Name("Root"): Ref{Object: 1}},
		objects: map[Ref]Object{
			{Object: 1}: Dict{Name("StructTreeRoot"): Ref{Object: 2}},
			{Object: 2}: Dict{Name("ParentTree"): Ref{Object: 3}},
			{Object: 3}: Dict{Name("Nums"): Array{Number(7), Ref{Object: 99}}},
		},
	}
	page := Page{dict: Dict{Name("StructParents"): Number(7)}}
	for _, err := range page.StructureSeq(d) {
		if err == nil || err.Error() != "playa: ParentTree value could not be resolved" {
			t.Fatalf("unresolved ParentTree slot error = %v", err)
		}
		return
	}
	t.Fatal("unresolved ParentTree slot produced no error")
}

func TestPageStructureSequenceReportsUnresolvedParentTreeRoot(t *testing.T) {
	d := &Document{
		trailer: Dict{Name("Root"): Ref{Object: 1}},
		objects: map[Ref]Object{
			{Object: 1}: Dict{Name("StructTreeRoot"): Ref{Object: 2}},
			{Object: 2}: Dict{Name("ParentTree"): Ref{Object: 99}},
		},
	}
	page := Page{dict: Dict{Name("StructParents"): Number(7)}}
	for _, err := range page.StructureSeq(d) {
		if err == nil || err.Error() != "playa: number tree root could not be resolved" {
			t.Fatalf("unresolved ParentTree root error = %v", err)
		}
		return
	}
	t.Fatal("unresolved ParentTree root produced no error")
}

func TestParentTreeCachesTerminalErrors(t *testing.T) {
	d := &Document{
		trailer: Dict{Name("Root"): Ref{Object: 1}},
		objects: map[Ref]Object{
			{Object: 1}: Dict{Name("StructTreeRoot"): Ref{Object: 2}},
			{Object: 2}: Dict{Name("ParentTree"): Ref{Object: 99}},
		},
	}

	_, _, firstErr := d.parentTreeValueChecked(7)
	_, _, secondErr := d.parentTreeValueChecked(7)
	if firstErr == nil || secondErr == nil {
		t.Fatalf("ParentTree errors = %v, %v; want both non-nil", firstErr, secondErr)
	}
	if firstErr != secondErr {
		t.Fatalf("ParentTree error was not cached: first=%p second=%p", firstErr, secondErr)
	}
	if got := d.parentTreeErrors[7]; got != firstErr {
		t.Fatalf("cached ParentTree error = %v, want %v", got, firstErr)
	}
}

func TestPageStructureCachesTerminalErrors(t *testing.T) {
	pageRef := Ref{Object: 9}
	d := &Document{
		trailer: Dict{Name("Root"): Ref{Object: 1}},
		objects: map[Ref]Object{
			{Object: 1}: Dict{Name("StructTreeRoot"): Ref{Object: 2}},
			{Object: 2}: Dict{Name("ParentTree"): Ref{Object: 99}},
		},
	}
	p := Page{ref: pageRef, dict: Dict{Name("StructParents"): Number(7)}}

	_, firstErr := p.Structure(d)
	_, secondErr := p.Structure(d)
	if firstErr == nil || secondErr == nil {
		t.Fatalf("page structure errors = %v, %v; want both non-nil", firstErr, secondErr)
	}
	if firstErr != secondErr {
		t.Fatalf("page structure error was not cached: first=%p second=%p", firstErr, secondErr)
	}
	if got := d.pageStructureErrors[pageRef]; got != firstErr {
		t.Fatalf("cached page structure error = %v, want %v", got, firstErr)
	}
}

func TestStructureErrorCachesHonorBudgets(t *testing.T) {
	d := &Document{
		cacheOptionsConfigured: true,
		cacheOptions:           cacheconfig.Options{ParentTreeErrorBytes: 0, PageStructureErrorBytes: 0},
		trailer:                Dict{Name("Root"): Ref{Object: 1}},
		objects: map[Ref]Object{
			{Object: 1}: Dict{Name("StructTreeRoot"): Ref{Object: 2}},
			{Object: 2}: Dict{Name("ParentTree"): Ref{Object: 99}},
		},
	}
	_, _, firstParentErr := d.parentTreeValueChecked(7)
	_, _, secondParentErr := d.parentTreeValueChecked(7)
	if firstParentErr == nil || secondParentErr == nil || firstParentErr == secondParentErr {
		t.Fatalf("ParentTree errors = %v/%v, want uncached independent errors", firstParentErr, secondParentErr)
	}
	p := Page{ref: Ref{Object: 9}, dict: Dict{Name("StructParents"): Number(7)}}
	_, firstPageErr := p.Structure(d)
	_, secondPageErr := p.Structure(d)
	if firstPageErr == nil || secondPageErr == nil || firstPageErr == secondPageErr {
		t.Fatalf("page structure errors = %v/%v, want uncached independent errors", firstPageErr, secondPageErr)
	}
	if len(d.parentTreeErrors) != 0 || len(d.pageStructureErrors) != 0 {
		t.Fatalf("structure error caches retained entries with zero budget: parent=%#v page=%#v", d.parentTreeErrors, d.pageStructureErrors)
	}
}

func TestStructureTreeSequenceReportsMalformedStructTreeRoot(t *testing.T) {
	d := &Document{trailer: Dict{Name("Root"): Dict{Name("StructTreeRoot"): Number(1)}}}
	for _, err := range d.StructureTreeSeq() {
		if err == nil {
			t.Fatal("malformed StructTreeRoot produced an element")
		}
		return
	}
	t.Fatal("malformed StructTreeRoot produced no error")
}

func TestPageStructureSequenceReportsMalformedStructTreeRoot(t *testing.T) {
	d := &Document{trailer: Dict{Name("Root"): Dict{Name("StructTreeRoot"): Number(1)}}}
	page := Page{dict: Dict{Name("StructParents"): Number(7)}}
	for _, err := range page.StructureSeq(d) {
		if err == nil {
			t.Fatal("malformed StructTreeRoot produced a structure entry")
		}
		return
	}
	t.Fatal("malformed StructTreeRoot produced no error")
}

func TestStructureTreeSequenceReportsMalformedRoleMap(t *testing.T) {
	d := &Document{trailer: Dict{Name("Root"): Dict{Name("StructTreeRoot"): Dict{
		Name("RoleMap"): Number(1),
		Name("K"):       Dict{Name("S"): Name("P")},
	}}}}
	for _, err := range d.StructureTreeSeq() {
		if err == nil {
			t.Fatal("malformed RoleMap produced a structure element")
		}
		return
	}
	t.Fatal("malformed RoleMap produced no error")
}

func TestStructureRoleMapCachesTerminalErrors(t *testing.T) {
	d := &Document{trailer: Dict{Name("Root"): Dict{Name("StructTreeRoot"): Dict{
		Name("RoleMap"): Number(1),
	}}}}

	_, firstErr := d.structureRoleMapChecked()
	_, secondErr := d.structureRoleMapChecked()
	if firstErr == nil || secondErr == nil {
		t.Fatalf("RoleMap errors = %v, %v; want both non-nil", firstErr, secondErr)
	}
	if firstErr != secondErr {
		t.Fatalf("RoleMap error was not cached: first=%p second=%p", firstErr, secondErr)
	}
	if d.structureRoleMapErr != firstErr || !d.structureRoleMapReady {
		t.Fatalf("cached RoleMap state = ready=%v err=%v, want ready with first error", d.structureRoleMapReady, d.structureRoleMapErr)
	}
}

func TestPageStructureSequenceIsRepeatableAndStopsEarly(t *testing.T) {
	d := &Document{
		trailer: Dict{Name("Root"): Ref{Object: 1, Generation: 0}},
		objects: map[Ref]Object{
			{Object: 1, Generation: 0}: Dict{Name("StructTreeRoot"): Ref{Object: 2, Generation: 0}},
			{Object: 2, Generation: 0}: Dict{Name("ParentTree"): Ref{Object: 5, Generation: 0}},
			{Object: 3, Generation: 0}: Dict{Name("S"): Name("P"), Name("T"): String("first")},
			{Object: 4, Generation: 0}: Dict{Name("S"): Name("Figure"), Name("T"): String("second")},
			{Object: 5, Generation: 0}: Dict{Name("Nums"): Array{Number(7), Array{Ref{Object: 3, Generation: 0}, Ref{Object: 4, Generation: 0}}}},
		},
	}
	page := Page{dict: Dict{Name("StructParents"): Number(7)}}

	first := 0
	for entry, err := range page.StructureSeq(d) {
		if err != nil {
			t.Fatal(err)
		}
		if entry.element == nil || entry.element.Title() != "first" {
			t.Fatalf("first structure entry = %#v", entry)
		}
		first++
		break
	}
	second := 0
	for entry, err := range page.StructureSeq(d) {
		if err != nil {
			t.Fatal(err)
		}
		if entry.element == nil {
			t.Fatalf("repeated structure entry = %#v", entry)
		}
		second++
	}
	if first != 1 || second != 2 {
		t.Fatalf("structure sequence counts = first %d, second %d", first, second)
	}
}

func TestPageStructureIndexesMCIDsAndFindsRoles(t *testing.T) {
	d := &Document{
		trailer: Dict{Name("Root"): Ref{Object: 1, Generation: 0}},
		objects: map[Ref]Object{
			{Object: 1, Generation: 0}: Dict{Name("StructTreeRoot"): Ref{Object: 2, Generation: 0}},
			{Object: 2, Generation: 0}: Dict{Name("ParentTree"): Ref{Object: 5, Generation: 0}},
			{Object: 3, Generation: 0}: Dict{Name("S"): Name("P"), Name("T"): String("paragraph")},
			{Object: 4, Generation: 0}: Dict{Name("S"): Name("Table"), Name("T"): String("table")},
			{Object: 5, Generation: 0}: Dict{Name("Nums"): Array{Number(7), Array{Ref{Object: 3, Generation: 0}, Null{}, Ref{Object: 4, Generation: 0}}}},
		},
	}
	page := Page{dict: Dict{Name("StructParents"): Number(7)}}
	structure, err := page.Structure(d)
	if err != nil {
		t.Fatal(err)
	}
	if got := structure.ByMCID(0); len(got) != 1 || got[0].Role() != "P" {
		t.Fatalf("MCID 0 = %#v", got)
	}
	var roles []string
	for element, err := range structure.FindAllSeq("") {
		if err != nil {
			t.Fatal(err)
		}
		roles = append(roles, element.Role())
	}
	if len(roles) != 2 || roles[0] != "P" || roles[1] != "Table" {
		t.Fatalf("page structure roles = %#v", roles)
	}
	if element, ok := structure.Find("Table"); !ok || element.Title() != "table" || element.document != nil || element.childStart != nil {
		t.Fatalf("page structure find = %#v, %v", element, ok)
	}
}

func TestPageStructureCachesCompletedViews(t *testing.T) {
	d := &Document{
		trailer: Dict{Name("Root"): Ref{Object: 1}},
		objects: map[Ref]Object{
			{Object: 1}: Dict{Name("StructTreeRoot"): Ref{Object: 2}},
			{Object: 2}: Dict{Name("ParentTree"): Ref{Object: 5}},
			{Object: 3}: Dict{Name("S"): Name("P"), Name("T"): String("paragraph")},
			{Object: 5}: Dict{Name("Nums"): Array{Number(7), Array{Ref{Object: 3}}}},
		},
	}
	page := Page{ref: Ref{Object: 9}, dict: Dict{Name("StructParents"): Number(7)}}
	first, err := page.Structure(d)
	if err != nil || len(first.elements) != 1 || first.elements[0].Title() != "paragraph" {
		t.Fatalf("first page structure = %#v, err = %v", first, err)
	}
	delete(d.objects, Ref{Object: 3})
	second, err := page.Structure(d)
	if err != nil || len(second.elements) != 1 || second.elements[0].Title() != "paragraph" {
		t.Fatalf("cached page structure = %#v, err = %v", second, err)
	}
	snapshot := second.Finalize()
	if snapshot.elements[0].Title() != "paragraph" {
		t.Fatalf("finalized cached title = %q", snapshot.elements[0].Title())
	}
	third, err := page.Structure(d)
	if err != nil || third.elements[0].Title() != "paragraph" {
		t.Fatalf("page structure cache was exposed = %#v, err = %v", third, err)
	}
}

func TestConcurrentPageStructureResolution(t *testing.T) {
	d := &Document{
		trailer: Dict{Name("Root"): Ref{Object: 1}},
		objects: map[Ref]Object{
			{Object: 1}: Dict{Name("StructTreeRoot"): Ref{Object: 2}},
			{Object: 2}: Dict{Name("ParentTree"): Ref{Object: 5}},
			{Object: 3}: Dict{Name("S"): Name("P"), Name("T"): String("paragraph")},
			{Object: 5}: Dict{Name("Nums"): Array{Number(7), Array{Ref{Object: 3}}}},
		},
	}
	page := Page{ref: Ref{Object: 9}, dict: Dict{Name("StructParents"): Number(7)}}
	const readers = 16
	var wg sync.WaitGroup
	errs := make(chan error, readers)
	start := make(chan struct{})
	for i := 0; i < readers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			structure, err := page.Structure(d)
			if err != nil {
				errs <- err
				return
			}
			if len(structure.elements) != 1 || structure.elements[0].Title() != "paragraph" {
				errs <- fmt.Errorf("unexpected page structure: %#v", structure)
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

func TestPageStructureCacheCopiesNestedElements(t *testing.T) {
	d := &Document{
		pageStructureCache: map[Ref]PageStructure{
			{Object: 9}: {
				elements: []StructElement{{
					data:     structuredata.NewElement(structuredata.ElementSpec{Attributes: Dict{Name("Lang"): String("en")}}),
					children: []StructElement{newStructElementValue(structuredata.ElementSpec{Title: "child"})},
					contents: []StructureContent{newStructureContent(structuredata.ContentSpec{Dict: Dict{Name("MCID"): Number(7)}, HasStream: true, Stream: newStream(Dict{Name("Length"): Number(1)}, []byte("x"))})},
				}},
			},
		},
		pageStructureReady: map[Ref]bool{{Object: 9}: true},
	}
	page := Page{ref: Ref{Object: 9}, dict: Dict{Name("StructParents"): Number(7)}}
	first, err := page.Structure(d)
	if err != nil || len(first.elements) != 1 || len(first.elements[0].children) != 1 {
		t.Fatalf("first nested structure = %#v, err = %v", first, err)
	}
	snapshot := first.Finalize()
	if snapshot.elements[0].children[0].Title() != "child" {
		t.Fatalf("finalized nested title = %q", snapshot.elements[0].children[0].Title())
	}
	snapshotAttributes := snapshot.elements[0].AttributesCopy()
	snapshotAttributes[Name("Lang")] = String("fr")
	snapshotContentDict := snapshot.elements[0].contents[0].DictCopy()
	snapshotContentDict[Name("MCID")] = Number(9)
	snapshotContentStream := snapshot.elements[0].contents[0].StreamCopy()
	snapshotContentStream.DataBorrowed()[0] = 'y'
	second, err := page.Structure(d)
	lang, _ := second.elements[0].AttributesCopy()[Name("Lang")].(String)
	if err != nil || second.elements[0].children[0].Title() != "child" || string(lang) != "en" || second.elements[0].contents[0].DictCopy()[Name("MCID")] != Number(7) || second.elements[0].contents[0].StreamCopy().DataBorrowed()[0] != 'x' {
		t.Fatalf("nested structure cache was exposed = %#v, err = %v", second, err)
	}
}

func TestPageStructurePreservesMultipleElementsPerParentTreeSlot(t *testing.T) {
	d := &Document{
		trailer: Dict{Name("Root"): Ref{Object: 1, Generation: 0}},
		objects: map[Ref]Object{
			{Object: 1, Generation: 0}: Dict{Name("StructTreeRoot"): Ref{Object: 2, Generation: 0}},
			{Object: 2, Generation: 0}: Dict{Name("ParentTree"): Ref{Object: 5, Generation: 0}},
			{Object: 3, Generation: 0}: Dict{Name("S"): Name("P"), Name("T"): String("first")},
			{Object: 4, Generation: 0}: Dict{Name("S"): Name("Span"), Name("T"): String("second")},
			{Object: 5, Generation: 0}: Dict{Name("Nums"): Array{Number(7), Array{Array{Ref{Object: 3, Generation: 0}, Ref{Object: 4, Generation: 0}}}}},
		},
	}
	page := Page{dict: Dict{Name("StructParents"): Number(7)}}
	structure, err := page.Structure(d)
	if err != nil {
		t.Fatal(err)
	}
	if got := structure.At(0); len(got) != 2 || got[0].Title() != "first" || got[1].Title() != "second" {
		t.Fatalf("parent tree slot = %#v", got)
	}
	var entries []PageStructureEntry
	for entry, err := range page.StructureSeq(d) {
		if err != nil {
			t.Fatal(err)
		}
		entries = append(entries, entry)
	}
	if len(entries) != 1 || len(entries[0].elements) != 2 || entries[0].element.Title() != "first" {
		t.Fatalf("structure entries = %#v", entries)
	}
}

func TestPageStructureParentTreeKeepsDirectElementsOnly(t *testing.T) {
	d := &Document{
		trailer: Dict{Name("Root"): Ref{Object: 1, Generation: 0}},
		objects: map[Ref]Object{
			{Object: 1, Generation: 0}: Dict{Name("StructTreeRoot"): Ref{Object: 2, Generation: 0}},
			{Object: 2, Generation: 0}: Dict{Name("ParentTree"): Ref{Object: 5, Generation: 0}},
			{Object: 3, Generation: 0}: Dict{Name("S"): Name("P"), Name("K"): Ref{Object: 4, Generation: 0}},
			{Object: 4, Generation: 0}: Dict{Name("S"): Name("Span")},
			{Object: 5, Generation: 0}: Dict{Name("Nums"): Array{Number(7), Array{Ref{Object: 3, Generation: 0}}}},
		},
	}
	page := Page{dict: Dict{Name("StructParents"): Number(7)}}
	structure, err := page.Structure(d)
	if err != nil {
		t.Fatal(err)
	}
	if got := structure.At(0); len(got) != 1 || got[0].Role() != "P" {
		t.Fatalf("parent tree direct elements = %#v", got)
	}
}
