package document

import "testing"

func TestStructureTreeResolvesIndirectElementFields(t *testing.T) {
	d := &Document{objects: map[Ref]Object{}, trailer: Dict{}}
	d.objects[Ref{Object: 2}] = Dict{
		Name("Type"): Name("StructTreeRoot"),
		Name("K"):    Array{Ref{Object: 3}},
	}
	d.objects[Ref{Object: 3}] = Dict{
		Name("Type"):       Name("StructElem"),
		Name("S"):          Ref{Object: 4},
		Name("T"):          Ref{Object: 5},
		Name("Alt"):        Ref{Object: 6},
		Name("ActualText"): Ref{Object: 7},
		Name("Pg"):         Ref{Object: 8},
		Name("K"):          Number(12),
	}
	d.objects[Ref{Object: 4}] = Name("P")
	d.objects[Ref{Object: 5}] = String("title")
	d.objects[Ref{Object: 6}] = String("alternative")
	d.objects[Ref{Object: 7}] = String("actual")
	d.objects[Ref{Object: 8}] = Ref{Object: 9}
	d.objects[Ref{Object: 9}] = Dict{Name("Type"): Name("Page")}
	d.trailer[Name("Root")] = Ref{Object: 1}
	d.objects[Ref{Object: 1}] = Dict{Name("StructTreeRoot"): Ref{Object: 2}}

	got := structureTree(t, d)
	if len(got) != 1 || got[0].Role() != "P" || got[0].Title() != "title" || got[0].AlternateDescription() != "alternative" || got[0].ActualText() != "actual" || !got[0].HasPage() || got[0].MCID() != 12 {
		t.Fatalf("structure = %#v", got)
	}
}

func TestStructureTreeFollowsMultiLevelIndirectStructureNodes(t *testing.T) {
	d := &Document{objects: map[Ref]Object{}, trailer: Dict{Name("Root"): Ref{Object: 1}}}
	d.objects[Ref{Object: 1}] = Dict{Name("StructTreeRoot"): Ref{Object: 2}}
	d.objects[Ref{Object: 2}] = Dict{Name("K"): Ref{Object: 3}}
	d.objects[Ref{Object: 3}] = Array{Ref{Object: 4}}
	d.objects[Ref{Object: 4}] = Ref{Object: 5}
	d.objects[Ref{Object: 5}] = Dict{Name("S"): Ref{Object: 6}, Name("T"): Ref{Object: 7}}
	d.objects[Ref{Object: 6}] = Name("P")
	d.objects[Ref{Object: 7}] = String("title")

	got, err := d.StructureTree()
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Role() != "P" || got[0].Title() != "title" {
		t.Fatalf("structure = %#v", got)
	}
}

func TestStructureTreeFollowsMultiLevelIndirectKidsArray(t *testing.T) {
	d := &Document{objects: map[Ref]Object{}, trailer: Dict{Name("Root"): Ref{Object: 1}}}
	d.objects[Ref{Object: 1}] = Dict{Name("StructTreeRoot"): Ref{Object: 2}}
	d.objects[Ref{Object: 2}] = Dict{Name("K"): Ref{Object: 3}}
	d.objects[Ref{Object: 3}] = Ref{Object: 4}
	d.objects[Ref{Object: 4}] = Array{Ref{Object: 5}}
	d.objects[Ref{Object: 5}] = Dict{Name("S"): Name("P"), Name("T"): String("title")}

	got, err := d.StructureTree()
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Role() != "P" || got[0].Title() != "title" {
		t.Fatalf("indirect structure kids = %#v", got)
	}
}

func TestStructureContentsAndAttributesFollowIndirectValues(t *testing.T) {
	d := &Document{objects: map[Ref]Object{}, trailer: Dict{Name("Root"): Ref{Object: 1}}}
	d.objects[Ref{Object: 1}] = Dict{Name("StructTreeRoot"): Ref{Object: 2}}
	d.objects[Ref{Object: 2}] = Dict{Name("K"): Ref{Object: 3}}
	d.objects[Ref{Object: 3}] = Dict{Name("S"): Name("P"), Name("K"): Ref{Object: 4}, Name("Lang"): Ref{Object: 9}, Name("E"): Ref{Object: 10}, Name("C"): Ref{Object: 11}, Name("A"): Ref{Object: 12}}
	d.objects[Ref{Object: 4}] = Array{Ref{Object: 5}}
	d.objects[Ref{Object: 5}] = Dict{Name("Type"): Ref{Object: 6}, Name("MCID"): Ref{Object: 7}, Name("Pg"): Ref{Object: 8}}
	d.objects[Ref{Object: 6}] = Name("MCR")
	d.objects[Ref{Object: 7}] = Number(3)
	d.objects[Ref{Object: 8}] = Ref{Object: 13}
	d.objects[Ref{Object: 9}] = String("en-US")
	d.objects[Ref{Object: 10}] = String("abbr")
	d.objects[Ref{Object: 11}] = Name("Class")
	d.objects[Ref{Object: 12}] = Dict{Name("O"): Name("Layout")}
	d.objects[Ref{Object: 13}] = Dict{Name("Type"): Name("Page")}

	root, err := firstStructureElement(d.StructureTreeSeq())
	if err != nil {
		t.Fatal(err)
	}
	if root.Language() != "en-US" || root.AbbreviationExpansion() != "abbr" || root.ClassName() != "Class" || root.AttributesCopy()[Name("O")] != Name("Layout") {
		t.Fatalf("structure metadata = %#v", root)
	}
	var contents []StructureContent
	for content, contentErr := range root.ContentsSeq() {
		if contentErr != nil {
			t.Fatal(contentErr)
		}
		contents = append(contents, content)
	}
	if len(contents) != 1 || !contents[0].HasMCID() || contents[0].MCID() != 3 || !contents[0].HasPage() {
		t.Fatalf("structure contents = %#v", contents)
	}
}

func TestStructureContentsSeqFollowsMultiLevelIndirectKids(t *testing.T) {
	d := &Document{objects: map[Ref]Object{}, trailer: Dict{Name("Root"): Ref{Object: 1}}}
	d.objects[Ref{Object: 1}] = Dict{Name("StructTreeRoot"): Ref{Object: 2}}
	d.objects[Ref{Object: 2}] = Dict{Name("K"): Ref{Object: 3}}
	d.objects[Ref{Object: 3}] = Dict{Name("S"): Name("P"), Name("K"): Ref{Object: 4}}
	d.objects[Ref{Object: 4}] = Ref{Object: 5}
	d.objects[Ref{Object: 5}] = Array{Ref{Object: 6}}
	d.objects[Ref{Object: 6}] = Dict{Name("Type"): Name("MCR"), Name("MCID"): Number(9)}

	root, err := firstStructureElement(d.StructureTreeSeq())
	if err != nil {
		t.Fatal(err)
	}
	var contents []StructureContent
	for content, contentErr := range root.ContentsSeq() {
		if contentErr != nil {
			t.Fatal(contentErr)
		}
		contents = append(contents, content)
	}
	if len(contents) != 1 || !contents[0].HasMCID() || contents[0].MCID() != 9 {
		t.Fatalf("indirect structure contents = %#v", contents)
	}
}

func TestStructureExportsMultiLevelIndirectRelationRefs(t *testing.T) {
	d := &Document{objects: map[Ref]Object{}, trailer: Dict{Name("Root"): Ref{Object: 1}}}
	d.objects[Ref{Object: 1}] = Dict{Name("StructTreeRoot"): Ref{Object: 2}}
	d.objects[Ref{Object: 2}] = Dict{Name("K"): Ref{Object: 3}}
	d.objects[Ref{Object: 3}] = Dict{Name("S"): Name("Figure"), Name("Pg"): Ref{Object: 4}, Name("P"): Ref{Object: 6}, Name("K"): Ref{Object: 8}}
	d.objects[Ref{Object: 4}] = Ref{Object: 5}
	d.objects[Ref{Object: 5}] = Dict{Name("Type"): Name("Page")}
	d.objects[Ref{Object: 6}] = Ref{Object: 7}
	d.objects[Ref{Object: 7}] = Dict{Name("Type"): Name("StructTreeRoot")}
	d.objects[Ref{Object: 8}] = Array{Ref{Object: 9}}
	d.objects[Ref{Object: 9}] = Dict{Name("Type"): Name("OBJR"), Name("Pg"): Ref{Object: 10}, Name("Obj"): Ref{Object: 12}}
	d.objects[Ref{Object: 10}] = Ref{Object: 11}
	d.objects[Ref{Object: 11}] = Dict{Name("Type"): Name("Page")}
	d.objects[Ref{Object: 12}] = Ref{Object: 13}
	d.objects[Ref{Object: 13}] = Dict{Name("Type"): Name("Annot")}

	root, err := firstStructureElement(d.StructureTreeSeq())
	if err != nil {
		t.Fatal(err)
	}
	if !root.HasPage() || root.Page() != (Ref{Object: 5}) || !root.HasParent() || root.Parent() != (Ref{Object: 7}) {
		t.Fatalf("structure element relations = %#v", root)
	}
	var content StructureContent
	for value, contentErr := range root.ContentsSeq() {
		if contentErr != nil {
			t.Fatal(contentErr)
		}
		content = value
		break
	}
	if !content.HasPage() || content.Page() != (Ref{Object: 11}) || !content.HasObject() || content.ObjectRef() != (Ref{Object: 13}) {
		t.Fatalf("structure content relations = %#v", content)
	}
}
