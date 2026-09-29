package document

import "testing"

func TestMarkedContentClonePreservesAbsentOperands(t *testing.T) {
	cloned := cloneMarkedContent(newMarkedContentValue("", Ref{}, false, 0, false, "", nil, []ContentOp{newContentOpBorrowed("q", nil, 0)}))
	if cloned.OpsCopy()[0].operandsValue() != nil {
		t.Fatalf("cloned operands = %#v, want nil", cloned.OpsCopy()[0].operandsValue())
	}
}

func TestMarkedContentValuesPreservesAbsentRoots(t *testing.T) {
	if got := markedContentValues(nil); got != nil {
		t.Fatalf("marked-content roots = %#v, want nil", got)
	}
}

func TestMarkedStackClonePreservesEmptyStack(t *testing.T) {
	stack := make([]markedContentContext, 0)
	if clone := cloneMarkedStack(stack); clone == nil {
		t.Fatal("empty marked-content stack was collapsed to nil")
	}
}

func TestMarkedContentSnapshotsPreserveEmptyOps(t *testing.T) {
	content := newMarkedContentValue("", Ref{}, false, 0, false, "", nil, make([]ContentOp, 0))
	snapshot := content.Finalize()
	if snapshot.OpsCopy() == nil || content.OpsCopy() == nil {
		t.Fatal("empty marked-content operations became nil")
	}
}

func TestMarkedContentOpsPreserveDocumentContext(t *testing.T) {
	resources := Dict{Name("Properties"): Dict{Name("P"): String("original")}}
	operation := newContentOpWithContext("BDC", []Object{Name("Span"), Name("P")}, 7, resources, formBegin, Name("P"), true)
	content := newMarkedContentValue("Span", Ref{}, false, 0, false, "", nil, []ContentOp{operation})

	var borrowed ContentOp
	for op, err := range content.OpsSeq() {
		if err != nil {
			t.Fatal(err)
		}
		borrowed = op
		break
	}
	if borrowed.formBoundary != formBegin || borrowed.propertyName != Name("P") || !borrowed.hasPropertyName {
		t.Fatalf("OpsSeq lost operation context: %#v", borrowed)
	}
	if got, ok := borrowed.resources[Name("Properties")].(Dict)[Name("P")].(String); !ok || string(got) != "original" {
		t.Fatalf("OpsSeq lost operation resources: %#v", borrowed.resources)
	}

	copyOps := content.OpsCopy()
	copyOps[0].resources[Name("Properties")].(Dict)[Name("P")] = String("changed")
	if got := resources[Name("Properties")].(Dict)[Name("P")].(String); string(got) != "original" {
		t.Fatalf("OpsCopy exposed operation resources: %q", got)
	}
}

func TestExtractMarkedContentBuildsNestedTree(t *testing.T) {
	ops, err := ParseContent([]byte("/P BMC 1 0 m /Span << /MCID 7 /ActualText <FEFF0041> >> BDC (x) Tj EMC 2 0 l EMC"))
	if err != nil {
		t.Fatal(err)
	}
	got := ExtractMarkedContent(ops)
	if len(got) != 1 || got[0].Tag() != "P" || len(got[0].children) != 1 {
		t.Fatalf("unexpected tree: %#v", got)
	}
	child := got[0].children[0]
	childOps := child.OpsCopy()
	if child.Tag() != "Span" || !child.HasMCID() || child.MCID() != 7 || child.ActualText() != "A" || len(childOps) != 1 || childOps[0].operatorValue() != "Tj" {
		t.Fatalf("unexpected child: %#v", child)
	}
	parentOps := got[0].OpsCopy()
	if len(parentOps) != 2 || parentOps[0].operatorValue() != "m" || parentOps[1].operatorValue() != "l" {
		t.Fatalf("unexpected parent ops: %#v", got[0].OpsCopy())
	}
}

func TestExtractMarkedContentIgnoresOperatorsOutsideSections(t *testing.T) {
	ops, _ := ParseContent([]byte("1 0 m /Artifact BMC 2 0 l EMC 3 0 l"))
	got := ExtractMarkedContent(ops)
	gotOps := got[0].OpsCopy()
	if len(got) != 1 || len(gotOps) != 1 || gotOps[0].operatorValue() != "l" {
		t.Fatalf("unexpected sections: %#v", got)
	}
}

func TestExtractMarkedContentResolvesPropertyList(t *testing.T) {
	ops, _ := ParseContent([]byte("/Span /P1 BDC EMC"))
	got := extractMarkedContent(ops, Dict{Name("P1"): Dict{Name("MCID"): Number(9)}})
	if len(got) != 1 || !got[0].HasMCID() || got[0].MCID() != 9 {
		t.Fatalf("unexpected property-list section: %#v", got)
	}
}

func TestExtractMarkedContentResolvesPropertyStream(t *testing.T) {
	ops, err := ParseContent([]byte("/P /P1 BDC EMC"))
	if err != nil {
		t.Fatal(err)
	}
	got := extractMarkedContent(ops, Dict{Name("P1"): newStream(Dict{Name("MCID"): Number(10)}, nil)})
	if len(got) != 1 || !got[0].HasMCID() || got[0].MCID() != 10 {
		t.Fatalf("property stream = %#v", got)
	}
}

func TestPageMarkedContentIndexFindsNestedMCIDs(t *testing.T) {
	d := &Document{}
	p := Page{dict: Dict{Name("Contents"): newStream(nil, []byte("/P << /MCID 1 >> BDC /Span << /MCID 2 >> BDC EMC EMC"))}}
	index, err := d.PageMarkedContentIndex(p)
	if err != nil || len(index.roots) != 1 || len(index.byMCID[1]) != 1 || len(index.byMCID[2]) != 1 || index.byMCID[2][0].Tag() != "Span" {
		t.Fatalf("marked index = %#v, err=%v", index, err)
	}
}

func TestPageMarkedContentPreservesOwningPage(t *testing.T) {
	d := &Document{
		trailer: Dict{Name("Root"): Ref{Object: 1}},
		objects: map[Ref]Object{
			{Object: 1}: Dict{Name("Pages"): Ref{Object: 2}},
			{Object: 2}: Dict{Name("Type"): Name("Pages"), Name("Kids"): Array{Ref{Object: 3}}, Name("Count"): Number(1)},
			{Object: 3}: Dict{Name("Type"): Name("Page"), Name("Parent"): Ref{Object: 2}, Name("Contents"): newStream(nil, []byte("/P BMC (x) Tj EMC"))},
		},
	}
	p, err := d.PageAt(0)
	if err != nil {
		t.Fatal(err)
	}
	items, err := d.PageMarkedContent(p)
	if err != nil || len(items) != 1 || !items[0].HasPage() || items[0].Page() != p.ref {
		t.Fatalf("marked content = %#v, err = %v", items, err)
	}
	owned, err := items[0].PageObject(d)
	if err != nil || owned.ref != p.ref {
		t.Fatalf("page = %#v, err = %v", owned, err)
	}
}

func TestPageMarkedContentIncludesFormXObjects(t *testing.T) {
	d := &Document{
		objects: map[Ref]Object{
			{Object: 4}: newStream(Dict{Name("Subtype"): Name("Form")}, []byte("/Span << /MCID 9 >> BDC 1 0 m EMC")),
		},
	}
	p := Page{
		ref:  Ref{Object: 3},
		dict: Dict{Name("Resources"): Dict{Name("XObject"): Dict{Name("Fm1"): Ref{Object: 4}}}, Name("Contents"): newStream(nil, []byte("/Fm1 Do"))},
	}
	items, err := d.PageMarkedContent(p)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].Tag() != "Span" || !items[0].HasMCID() || items[0].MCID() != 9 {
		t.Fatalf("form marked content = %#v", items)
	}
}

func TestPageMarkedContentSkipsEmptySections(t *testing.T) {
	d := &Document{}
	p := Page{dict: Dict{Name("Contents"): newStream(nil, []byte(
		"/P << /MCID 1 >> BDC EMC /P << /MCID 2 >> BDC (x) Tj EMC",
	))}}
	got, err := d.PageMarkedContent(p)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].MCID() != 2 || len(got[0].OpsCopy()) != 1 {
		t.Fatalf("marked content = %#v", got)
	}
}

func TestPageMarkedContentByMCIDIsLazyAndNested(t *testing.T) {
	p := Page{dict: Dict{Name("Contents"): Array{
		newStream(nil, []byte("/P << /MCID 1 >> BDC /Span << /MCID 2 >> BDC EMC EMC")),
		newStream(nil, []byte("[ malformed")),
	}}}
	d := &Document{}
	var got []string
	for item, err := range p.MarkedContentByMCIDSeq(d, 2) {
		if err != nil {
			t.Fatal(err)
		}
		got = append(got, item.Tag())
		break
	}
	if len(got) != 1 || got[0] != "Span" {
		t.Fatalf("mcid sections = %#v", got)
	}
}

func TestPageMarkedContentDefersPagePropertiesWithoutBDC(t *testing.T) {
	d := &Document{pagePropCache: map[Ref]Dict{}}
	p := Page{ref: Ref{Object: 1}, dict: Dict{
		Name("Contents"):  newStream(nil, []byte("/Artifact BMC EMC")),
		Name("Resources"): Dict{Name("Properties"): Dict{Name("P1"): Dict{Name("MCID"): Number(1)}}},
	}}
	if _, err := d.PageMarkedContent(p); err != nil {
		t.Fatal(err)
	}
	if len(d.pagePropCache) != 0 {
		t.Fatalf("page properties resolved without BDC: %#v", d.pagePropCache)
	}
}

func TestPageMarkedContentReportsMalformedPropertiesResource(t *testing.T) {
	p := Page{dict: Dict{
		Name("Contents"):  newStream(nil, []byte("/Span /P1 BDC (x) Tj EMC")),
		Name("Resources"): Dict{Name("Properties"): Number(1)},
	}}
	for _, err := range p.MarkedContent(&Document{}) {
		if err == nil {
			t.Fatal("malformed Properties resource produced marked content")
		}
		return
	}
	t.Fatal("malformed Properties resource produced no error")
}

func TestPageMarkedContentReportsMalformedFormPropertiesResource(t *testing.T) {
	form := newStream(Dict{
		Name("Subtype"):   Name("Form"),
		Name("Resources"): Dict{Name("Properties"): Number(1)},
	}, []byte("/Span << /MCID 1 >> BDC (x) Tj EMC"))
	p := Page{dict: Dict{
		Name("Contents"):  newStream(nil, []byte("/Fm Do")),
		Name("Resources"): Dict{Name("XObject"): Dict{Name("Fm"): form}},
	}}
	for _, err := range p.MarkedContent(&Document{}) {
		if err == nil {
			t.Fatal("malformed Form Properties resource produced marked content")
		}
		return
	}
	t.Fatal("malformed Form Properties resource produced no error")
}

func TestPageMarkedContentReportsUnresolvedFormPropertiesResource(t *testing.T) {
	form := newStream(Dict{
		Name("Subtype"):   Name("Form"),
		Name("Resources"): Dict{Name("Properties"): Ref{Object: 99}},
	}, []byte("/Span /P1 BDC (x) Tj EMC"))
	p := Page{dict: Dict{
		Name("Contents"):  newStream(nil, []byte("/Fm Do")),
		Name("Resources"): Dict{Name("XObject"): Dict{Name("Fm"): form}},
	}}
	for _, err := range p.MarkedContent(&Document{}) {
		if err == nil || err.Error() != "playa: Properties resources could not be resolved" {
			t.Fatalf("unresolved Form Properties error = %v", err)
		}
		return
	}
	t.Fatal("unresolved Form Properties resource produced no error")
}

func TestPageMarkedContentReportsUnresolvedPropertiesEntry(t *testing.T) {
	p := Page{dict: Dict{
		Name("Contents"):  newStream(nil, []byte("/Span /P1 BDC (x) Tj EMC")),
		Name("Resources"): Dict{Name("Properties"): Dict{Name("P1"): Ref{Object: 99}}},
	}}
	for _, err := range p.MarkedContent(&Document{}) {
		if err == nil || err.Error() != "playa: property resource \"P1\" could not be resolved" {
			t.Fatalf("unresolved Properties entry error = %v", err)
		}
		return
	}
	t.Fatal("unresolved Properties entry produced no error")
}

func TestMarkedContentIndexDoesNotShareRootNodes(t *testing.T) {
	page := Page{dict: Dict{Name("Contents"): newStream(nil, []byte("/P << /MCID 1 /Value (original) >> BDC /Span << /MCID 2 >> BDC EMC EMC"))}}
	d := &Document{}
	index, err := d.PageMarkedContentIndex(page)
	if err != nil || len(index.roots) != 1 || len(index.byMCID[1]) != 1 {
		t.Fatalf("marked content index = %#v, err = %v", index, err)
	}
	snapshot := index.Finalize()
	snapshotProperties := snapshot.roots[0].PropertiesCopy()
	snapshotProperties[Name("Value")] = String("changed")
	value, _ := index.byMCID[1][0].PropertiesCopy()[Name("Value")].(String)
	if string(value) != "original" {
		t.Fatalf("MCID index shared root properties: %#v", index.byMCID[1][0].PropertiesCopy())
	}
}

func TestMarkedContentIndexCopiesDoNotExposeSource(t *testing.T) {
	page := Page{dict: Dict{Name("Contents"): newStream(nil, []byte("/P << /MCID 1 /Value (original) >> BDC /Span << /MCID 2 >> BDC EMC EMC"))}}
	index, err := (&Document{}).PageMarkedContentIndex(page)
	if err != nil {
		t.Fatal(err)
	}

	roots := index.RootsCopy()
	byMCID := index.ByMCIDCopy()
	rootProperties := roots[0].PropertiesCopy()
	rootProperties[Name("Value")] = String("changed roots")
	mcidProperties := byMCID[1][0].PropertiesCopy()
	mcidProperties[Name("Value")] = String("changed index")

	rootValue, _ := index.roots[0].PropertiesCopy()[Name("Value")].(String)
	indexValue, _ := index.byMCID[1][0].PropertiesCopy()[Name("Value")].(String)
	if string(rootValue) != "original" || string(indexValue) != "original" {
		t.Fatalf("marked content index copies exposed source: roots=%q index=%q", rootValue, indexValue)
	}
}

func TestMarkedContentDoesNotSharePropertiesOrOperands(t *testing.T) {
	page := Page{dict: Dict{Name("Contents"): newStream(nil, []byte("/Span << /MCID 1 /Value (original) >> BDC (text) Tj EMC"))}}
	d := &Document{}
	first, err := d.PageMarkedContent(page)
	if err != nil || len(first) != 1 {
		t.Fatalf("marked content = %#v, err = %v", first, err)
	}
	firstProperties := first[0].PropertiesCopy()
	firstProperties[Name("Value")] = String("changed")
	firstOps := first[0].OpsCopy()
	firstOps[0].operandsValue()[0] = String("changed")
	second, err := d.PageMarkedContent(page)
	if err != nil || len(second) != 1 {
		t.Fatalf("second marked content = %#v, err = %v", second, err)
	}
	value, _ := second[0].PropertiesCopy()[Name("Value")].(String)
	if string(value) != "original" {
		t.Fatalf("marked properties were aliased: %#v", second[0].PropertiesCopy())
	}
}

func TestPageMarkedContentDefersNestedValueMaterialization(t *testing.T) {
	p := Page{dict: Dict{Name("Contents"): newStream(nil, []byte("/P BMC /Span BMC EMC EMC"))}}
	d := &Document{}
	var root MarkedContent
	for item, err := range d.markedContentSeq(p) {
		if err != nil {
			t.Fatal(err)
		}
		root = item
		break
	}
	if len(root.children) != 0 {
		t.Fatalf("nested marked content was materialized: %#v", root.children)
	}
	var children []MarkedContent
	for child, err := range root.ChildrenSeq() {
		if err != nil {
			t.Fatal(err)
		}
		children = append(children, child)
	}
	if len(children) != 1 || children[0].Tag() != "Span" {
		t.Fatalf("lazy marked children = %#v", children)
	}
}

func TestPageMarkedContentDoesNotMutateSharedPropertyResources(t *testing.T) {
	properties := Dict{Name("P1"): Ref{Object: 9, Generation: 0}}
	d := &Document{objects: map[Ref]Object{{Object: 9, Generation: 0}: Dict{Name("MCID"): Number(3)}}}
	p := Page{dict: Dict{Name("Resources"): Dict{Name("Properties"): properties}, Name("Contents"): newStream(nil, []byte("/Span /P1 BDC (x) Tj EMC"))}}
	items, err := d.PageMarkedContent(p)
	if err != nil {
		t.Fatal(err)
	}
	itemProperties := items[0].PropertiesCopy()
	itemProperties[Name("MCID")] = Number(99)
	if _, ok := properties[Name("P1")].(Ref); !ok {
		t.Fatalf("properties mutated: %#v", properties)
	}
	resolved, _ := d.ResolveRef(Ref{Object: 9})
	if value, _ := resolved.(Dict)[Name("MCID")].(Number); value != Number(3) {
		t.Fatalf("resolved property dictionary was exposed: %#v", resolved)
	}
}
