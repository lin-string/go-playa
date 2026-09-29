package document

import "testing"

func TestPagePatternsFollowsPatternColorSelections(t *testing.T) {
	pattern := Dict{Name("PatternType"): Number(1), Name("PaintType"): Number(1)}
	d := &Document{}
	p := Page{dict: Dict{
		Name("Contents"):  newStream(nil, []byte("/Pattern cs /P1 scn 0.5 scn /P1 scn")),
		Name("Resources"): Dict{Name("Pattern"): Dict{Name("P1"): pattern}},
	}}
	var names []string
	for value, err := range p.Patterns(d) {
		if err != nil {
			t.Fatal(err)
		}
		names = append(names, value.Name())
	}
	if len(names) != 2 || names[0] != "P1" || names[1] != "P1" {
		t.Fatalf("pattern selections = %#v", names)
	}
}

func TestXObjectPatternsUseFormResourcesAndSnapshots(t *testing.T) {
	d := &Document{}
	stream := newStream(Dict{Name("Resources"): Dict{Name("Pattern"): Dict{Name("Local"): newStream(Dict{Name("PatternType"): Number(1)}, []byte("tile"))}}}, []byte("/Pattern cs /Local scn"))
	x := testXObjectStream(stream)
	for value, err := range x.Patterns(d) {
		if err != nil {
			t.Fatal(err)
		}
		copy := value.DictCopy()
		copy[Name("Marker")] = String("changed")
		if _, ok := value.DictCopy()[Name("Marker")]; ok {
			t.Fatal("pattern dictionary copy shares source")
		}
		streamCopy, ok := value.StreamCopy()
		if !ok || string(streamCopy.Buffer()) != "tile" {
			t.Fatalf("pattern stream snapshot = %#v, %v", streamCopy, ok)
		}
		return
	}
	t.Fatal("form pattern was not yielded")
}

func TestPagePatternsReportsMalformedResource(t *testing.T) {
	d := &Document{}
	p := Page{dict: Dict{
		Name("Contents"):  newStream(nil, []byte("/Pattern cs /Bad scn")),
		Name("Resources"): Dict{Name("Pattern"): Dict{Name("Bad"): Number(1)}},
	}}
	for _, err := range p.Patterns(d) {
		if err == nil {
			t.Fatal("malformed pattern resource produced no error")
		}
		return
	}
	t.Fatal("malformed pattern resource produced no sequence error")
}

func TestPagePatternsReportsUnresolvedResourceReferences(t *testing.T) {
	tests := []struct {
		name string
		page Page
		want string
	}{
		{name: "resources", page: Page{dict: Dict{Name("Contents"): newStream(nil, []byte("/Pattern cs /P scn")), Name("Resources"): Ref{Object: 99}}}, want: "playa: Pattern resources could not be resolved"},
		{name: "entry", page: Page{dict: Dict{Name("Contents"): newStream(nil, []byte("/Pattern cs /P scn")), Name("Resources"): Dict{Name("Pattern"): Dict{Name("P"): Ref{Object: 99}}}}}, want: "playa: pattern resource \"P\" could not be resolved"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			for _, err := range test.page.Patterns(&Document{}) {
				if err == nil || err.Error() != test.want {
					t.Fatalf("error = %v, want %q", err, test.want)
				}
				return
			}
			t.Fatal("unresolved Pattern reference produced no error")
		})
	}
}
