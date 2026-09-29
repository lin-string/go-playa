package document

import "testing"

func TestOutlineResolvesIndirectTitle(t *testing.T) {
	d := &Document{objects: map[Ref]Object{}, trailer: Dict{}}
	d.trailer[Name("Root")] = Ref{Object: 1}
	d.objects[Ref{Object: 1}] = Dict{Name("Outlines"): Ref{Object: 2}}
	d.objects[Ref{Object: 2}] = Dict{Name("First"): Ref{Object: 3}}
	d.objects[Ref{Object: 3}] = Dict{Name("Title"): Ref{Object: 4}}
	d.objects[Ref{Object: 4}] = String("chapter")

	got, err := d.CollectOutline()
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Title() != "chapter" {
		t.Fatalf("outline = %#v", got)
	}
}
