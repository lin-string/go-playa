package document

import "testing"

func TestPagesDoNotInheritLastModifiedFromPagesNode(t *testing.T) {
	d := &Document{
		trailer: Dict{Name("Root"): Ref{Object: 1}},
		objects: map[Ref]Object{
			{Object: 1}: Dict{Name("Pages"): Ref{Object: 2}},
			{Object: 2}: Dict{
				Name("Type"):         Name("Pages"),
				Name("LastModified"): String("D:20240102030405Z"),
				Name("Kids"):         Array{Ref{Object: 3}},
			},
			{Object: 3}: Dict{Name("Type"): Name("Page")},
		},
	}
	for page, err := range d.Pages() {
		if err != nil {
			t.Fatal(err)
		}
		if page.lastModified != "" {
			t.Fatalf("last modified inherited from /Pages = %q", page.lastModified)
		}
		return
	}
	t.Fatal("no page yielded")
}

func TestPagesDoNotFollowInheritedIndirectLastModified(t *testing.T) {
	d := &Document{
		trailer: Dict{Name("Root"): Ref{Object: 1}},
		objects: map[Ref]Object{
			{Object: 1}: Dict{Name("Pages"): Ref{Object: 2}},
			{Object: 2}: Dict{
				Name("Type"):         Name("Pages"),
				Name("LastModified"): Ref{Object: 4},
				Name("Kids"):         Array{Ref{Object: 3}},
			},
			{Object: 3}: Dict{Name("Type"): Name("Page")},
			{Object: 4}: Ref{Object: 5},
			{Object: 5}: String("D:20250102030405Z"),
		},
	}
	for page, err := range d.Pages() {
		if err != nil {
			t.Fatal(err)
		}
		if page.lastModified != "" {
			t.Fatalf("indirect last modified inherited from /Pages = %q", page.lastModified)
		}
		return
	}
	t.Fatal("no page yielded")
}
