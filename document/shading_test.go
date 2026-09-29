package document

import "testing"

func TestPageShadingsFollowsContentOrderAndIsRepeatable(t *testing.T) {
	shading := Dict{Name("ShadingType"): Number(2), Name("ColorSpace"): Name("DeviceRGB")}
	d := &Document{}
	p := Page{dict: Dict{
		Name("Contents"):  newStream(nil, []byte("/S1 sh /S2 sh")),
		Name("Resources"): Dict{Name("Shading"): Dict{Name("S1"): shading, Name("S2"): newStream(shading, []byte("samples"))}},
	}}
	for pass := 0; pass < 2; pass++ {
		var names []string
		for value, err := range p.Shadings(d) {
			if err != nil {
				t.Fatal(err)
			}
			names = append(names, value.Name())
		}
		if len(names) != 2 || names[0] != "S1" || names[1] != "S2" {
			t.Fatalf("shading names pass %d = %#v", pass, names)
		}
	}
	for value, err := range p.Shadings(d) {
		if err != nil {
			t.Fatal(err)
		}
		final := value.Finalize()
		copy := final.DictCopy()
		copy[Name("Marker")] = String("changed")
		if _, ok := value.DictCopy()[Name("Marker")]; ok {
			t.Fatal("shading dictionary copy shares source")
		}
		if value.hasStream {
			stream, ok := final.StreamCopy()
			if !ok || string(stream.Buffer()) != "samples" {
				t.Fatalf("stream snapshot = %#v, %v", stream, ok)
			}
		}
		break
	}
}

func TestPageShadingsReportsMalformedResource(t *testing.T) {
	d := &Document{}
	p := Page{dict: Dict{
		Name("Contents"):  newStream(nil, []byte("/Missing sh")),
		Name("Resources"): Dict{Name("Shading"): Dict{Name("Missing"): Number(1)}},
	}}
	for _, err := range p.Shadings(d) {
		if err == nil {
			t.Fatal("malformed shading resource produced no error")
		}
		return
	}
	t.Fatal("malformed shading resource produced no sequence error")
}

func TestPageShadingsReportsUnresolvedResourceReferences(t *testing.T) {
	tests := []struct {
		name string
		page Page
		want string
	}{
		{name: "resources", page: Page{dict: Dict{Name("Contents"): newStream(nil, []byte("/S sh")), Name("Resources"): Ref{Object: 99}}}, want: "playa: Shading resources could not be resolved"},
		{name: "entry", page: Page{dict: Dict{Name("Contents"): newStream(nil, []byte("/S sh")), Name("Resources"): Dict{Name("Shading"): Dict{Name("S"): Ref{Object: 99}}}}}, want: "playa: shading resource \"S\" could not be resolved"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			for _, err := range test.page.Shadings(&Document{}) {
				if err == nil || err.Error() != test.want {
					t.Fatalf("error = %v, want %q", err, test.want)
				}
				return
			}
			t.Fatal("unresolved Shading reference produced no error")
		})
	}
}

func TestXObjectShadingsUsesFormResources(t *testing.T) {
	d := &Document{}
	form := newStream(Dict{Name("Resources"): Dict{Name("Shading"): Dict{Name("Local"): Dict{Name("ShadingType"): Number(2)}}}}, []byte("/Local sh"))
	x := testXObjectStream(form)
	var names []string
	for shading, err := range x.Shadings(d) {
		if err != nil {
			t.Fatal(err)
		}
		names = append(names, shading.Name())
	}
	if len(names) != 1 || names[0] != "Local" {
		t.Fatalf("form shading names = %#v", names)
	}
}
