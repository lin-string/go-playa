package document

import (
	"testing"

	"github.com/lin-string/go-playa/documentdata"
	"github.com/lin-string/go-playa/structuredata"
)

func TestFinalizeClearsLazyTreeOwnership(t *testing.T) {
	outline := OutlineNode{
		document:      &Document{},
		childStart:    Dict{Name("Title"): String("unused")},
		childrenReady: true,
		children: []OutlineNode{newOutlineValue(documentdata.OutlineNodeSpec{
			Action: Dict{Name("Meta"): Dict{Name("Value"): String("original")}},
		})},
	}
	outlineSnapshot := outline.Finalize()
	if outlineSnapshot.document != nil || outlineSnapshot.childStart != nil || !outlineSnapshot.childrenReady {
		t.Fatalf("outline snapshot retained lazy state: %#v", outlineSnapshot)
	}
	snapshotAction := outlineSnapshot.children[0].ActionCopy()
	snapshotAction[Name("Meta")].(Dict)[Name("Value")] = String("changed")
	if string(outline.children[0].ActionCopy()[Name("Meta")].(Dict)[Name("Value")].(String)) != "original" {
		t.Fatal("outline snapshot exposed nested action")
	}

	field := FormField{
		document:   &Document{},
		kidObjects: []Object{Dict{Name("T"): String("unused")}},
		data:       documentdata.NewFormField(documentdata.FormFieldSpec{Dict: Dict{Name("Meta"): Dict{Name("Value"): String("original")}}}),
		kidsReady:  true,
		kids:       []FormField{{data: documentdata.NewFormField(documentdata.FormFieldSpec{Name: "child"})}},
	}
	fieldSnapshot := field.Finalize()
	if fieldSnapshot.document != nil || fieldSnapshot.kidObjects != nil || !fieldSnapshot.kidsReady {
		t.Fatalf("form snapshot retained lazy state: %#v", fieldSnapshot)
	}
	fieldSnapshotDict := fieldSnapshot.DictCopy()
	fieldSnapshotDict[Name("Meta")].(Dict)[Name("Value")] = String("changed")
	if string(field.DictCopy()[Name("Meta")].(Dict)[Name("Value")].(String)) != "original" {
		t.Fatal("form snapshot exposed nested dictionary")
	}

	element := newStructElementValue(structuredata.ElementSpec{
		Dict: Dict{Name("Meta"): Dict{Name("Value"): String("original")}},
	})
	element.document = &Document{}
	element.childStart = Dict{Name("K"): Number(1)}
	element.children = []StructElement{newStructElementValue(structuredata.ElementSpec{Role: "child"})}
	element.childrenReady = true
	element.contentsReady = true
	elementSnapshot := element.Finalize()
	if elementSnapshot.document != nil || elementSnapshot.childStart != nil || !elementSnapshot.childrenReady || !elementSnapshot.contentsReady {
		t.Fatalf("structure snapshot retained lazy state: %#v", elementSnapshot)
	}
	snapshotDict := elementSnapshot.DictCopy()
	snapshotDict[Name("Meta")].(Dict)[Name("Value")] = String("changed")
	originalDict := element.DictCopy()
	if string(originalDict[Name("Meta")].(Dict)[Name("Value")].(String)) != "original" {
		t.Fatal("structure snapshot exposed nested dictionary")
	}
	indexCopy := (StructureIndex{roots: []StructElement{element}}).RootsCopy()
	if len(indexCopy) != 1 || indexCopy[0].document != nil || indexCopy[0].childStart != nil {
		t.Fatalf("structure copy retained lazy state: %#v", indexCopy)
	}
	pageCopy := (PageStructure{elements: []StructElement{element}}).ElementsCopy()
	if len(pageCopy) != 1 || pageCopy[0].document != nil || pageCopy[0].childStart != nil {
		t.Fatalf("page structure copy retained lazy state: %#v", pageCopy)
	}

	outlineChildren, err := (OutlineNode{
		document:      &Document{},
		childrenReady: true,
		children:      []OutlineNode{{document: &Document{}, childStart: Dict{Name("Title"): String("unused")}}},
	}).ChildrenCopy()
	if err != nil {
		t.Fatal(err)
	}
	if len(outlineChildren) != 1 || outlineChildren[0].document != nil || outlineChildren[0].childStart != nil {
		t.Fatalf("outline copy retained lazy state: %#v", outlineChildren)
	}

	marked := MarkedContent{node: &markedNode{}}
	markedCopy := (MarkedContentIndex{roots: []MarkedContent{marked}, byMCID: map[int][]MarkedContent{1: {marked}}}).RootsCopy()
	if len(markedCopy) != 1 || markedCopy[0].node != nil {
		t.Fatalf("marked-content copy retained parser node: %#v", markedCopy)
	}
}
