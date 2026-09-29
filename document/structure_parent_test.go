package document

import (
	"testing"

	"github.com/lin-string/go-playa/contentdata"
)

func TestParentTreeElementResolvesContentParent(t *testing.T) {
	d := &Document{
		trailer: Dict{Name("Root"): Ref{Object: 1}},
		objects: map[Ref]Object{
			{Object: 1}: Dict{Name("StructTreeRoot"): Ref{Object: 2}},
			{Object: 2}: Dict{
				Name("ParentTree"): Dict{
					Name("Nums"): Array{
						Number(4), Dict{Name("S"): Name("P"), Name("T"): String("body")},
					},
				},
			},
		},
	}
	parent := d.parentTreeElement(4)
	if parent == nil || parent.Role() != "P" || parent.Title() != "body" {
		t.Fatalf("parent = %#v", parent)
	}
	if _, ok := d.parentTreeValue(4); !ok {
		t.Fatal("cached parent-tree value is missing")
	}
	if _, ok := d.parentTreeValue(99); ok {
		t.Fatal("unexpected parent-tree value")
	}
	if _, ok := d.parentTreeValue(99); ok || !d.parentTreeMissing[99] {
		t.Fatal("missing parent-tree lookup was not cached")
	}
	snapshot := parent.Finalize()
	snapshotDict := snapshot.DictCopy()
	snapshotDict[Name("T")] = String("changed")
	second := d.parentTreeElement(4)
	if second == nil || string(second.DictCopy()[Name("T")].(String)) != "body" {
		t.Fatalf("parent-tree element exposed source dictionary: %#v", second)
	}
}

func TestPageContentParentResolvesMCIDFromParentTree(t *testing.T) {
	d := &Document{
		trailer: Dict{Name("Root"): Ref{Object: 1}},
		objects: map[Ref]Object{
			{Object: 1}: Dict{Name("Pages"): Ref{Object: 2}, Name("StructTreeRoot"): Ref{Object: 3}},
			{Object: 2}: Dict{Name("Type"): Name("Pages"), Name("Kids"): Array{Ref{Object: 9}}},
			{Object: 3}: Dict{Name("ParentTree"): Dict{Name("Nums"): Array{
				Number(7), Array{Dict{Name("Type"): Name("StructElem"), Name("S"): Name("P")}},
			}}},
			{Object: 9}: Dict{Name("Type"): Name("Page"), Name("Parent"): Ref{Object: 2}, Name("StructParents"): Number(7)},
		},
	}
	parent := d.pageContentParent(Ref{Object: 9}, 0)
	if parent == nil || parent.Role() != "P" {
		t.Fatalf("content parent = %#v", parent)
	}
	if got := newTestText(contentdata.TextSpec{Page: Ref{Object: 9}, HasPage: true, MCID: 0, HasMCID: true}, nil, nil).Parent(d); got == nil || got.Role() != "P" {
		t.Fatalf("text parent = %#v", got)
	}
	image := ImageObject{page: Ref{Object: 9}, hasPage: true, mcid: 0, hasMCID: true}
	if got := image.Parent(d); got == nil || got.Role() != "P" {
		t.Fatalf("image parent = %#v", got)
	}
}

func TestImageParentUsesNearestMarkedStackMCID(t *testing.T) {
	d := &Document{
		trailer: Dict{Name("Root"): Ref{Object: 1}},
		objects: map[Ref]Object{
			{Object: 1}: Dict{Name("Pages"): Ref{Object: 2}, Name("StructTreeRoot"): Ref{Object: 3}},
			{Object: 2}: Dict{Name("Type"): Name("Pages"), Name("Kids"): Array{Ref{Object: 9}}},
			{Object: 3}: Dict{Name("ParentTree"): Dict{Name("Nums"): Array{
				Number(7), Array{Dict{Name("Type"): Name("StructElem"), Name("S"): Name("Figure")}},
			}}},
			{Object: 9}: Dict{Name("Type"): Name("Page"), Name("Parent"): Ref{Object: 2}, Name("StructParents"): Number(7)},
		},
	}
	image := ImageObject{
		page: Ref{Object: 9}, hasPage: true,
		markedStack: []markedContentContext{
			newMarkedContentContext("Figure", nil, "", 0, true, nil),
			newMarkedContentContext("Span", nil, "", -1, false, nil),
		},
	}
	parent := image.Parent(d)
	if parent == nil || parent.Role() != "Figure" {
		t.Fatalf("image parent = %#v, want outer Figure MCID parent", parent)
	}
}

func TestPageContentParentReportsMalformedStructParents(t *testing.T) {
	d := &Document{
		trailer: Dict{Name("Root"): Ref{Object: 1}},
		objects: map[Ref]Object{
			{Object: 1}: Dict{Name("Pages"): Ref{Object: 2}},
			{Object: 2}: Dict{Name("Type"): Name("Pages"), Name("Kids"): Array{Ref{Object: 9}}},
			{Object: 9}: Dict{Name("Type"): Name("Page"), Name("Parent"): Ref{Object: 2}, Name("StructParents"): Ref{Object: 99}},
		},
	}
	if _, err := d.pageContentParentWithError(Ref{Object: 9}, 0); err == nil {
		t.Fatal("expected malformed StructParents error")
	}
}
