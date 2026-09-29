package document

import (
	"testing"

	"github.com/lin-string/go-playa/contentconfig"
)

func TestPageColorSpacesFollowsNamedSelections(t *testing.T) {
	d := &Document{}
	p := Page{dict: Dict{
		Name("Contents"):  newStream(nil, []byte("/CS1 cs /CS1 CS")),
		Name("Resources"): Dict{Name("ColorSpace"): Dict{Name("CS1"): Name("DeviceRGB")}},
	}}
	var values []ColorSpaceObject
	for value, err := range p.ColorSpaces(d) {
		if err != nil {
			t.Fatal(err)
		}
		values = append(values, value)
	}
	if len(values) != 2 || values[0].Name() != "CS1" || values[0].Stroke() || !values[1].Stroke() {
		t.Fatalf("color space selections = %#v", values)
	}
	for _, value := range values {
		if value.Info().Name() != "DeviceRGB" || value.Info().Components() != 3 {
			t.Fatalf("color space info = %#v", value.Info())
		}
	}
}

func TestPageColorSpacesReusesValidatedPageResources(t *testing.T) {
	d := &Document{objects: map[Ref]Object{
		{Object: 1}: Dict{Name("ColorSpace"): Ref{Object: 2}},
		{Object: 2}: Dict{Name("CS1"): Name("DeviceRGB")},
	}}
	p := Page{ref: Ref{Object: 9}, dict: Dict{Name("Contents"): newStream(nil, []byte("/CS1 cs")), Name("Resources"): Ref{Object: 1}}}
	if _, err := p.ResourcesWithError(d); err != nil {
		t.Fatal(err)
	}
	d.objects[Ref{Object: 1}] = Ref{Object: 99}
	for value, err := range p.ColorSpaces(d) {
		if err != nil || value.Name() != "CS1" || value.Info().Name() != "DeviceRGB" {
			t.Fatalf("ColorSpaces did not reuse validated resources: %#v, err=%v", value, err)
		}
		return
	}
	t.Fatal("ColorSpaces produced no selection")
}

func TestXObjectColorSpacesUseFormResources(t *testing.T) {
	d := &Document{}
	x := testXObjectStream(newStream(Dict{Name("Resources"): Dict{Name("ColorSpace"): Dict{Name("Local"): Name("DeviceGray")}}}, []byte("/Local cs")))
	for value, err := range x.ColorSpaces(d) {
		if err != nil {
			t.Fatal(err)
		}
		if value.Name() != "Local" || value.Info().Name() != "DeviceGray" || value.Info().Components() != 1 {
			t.Fatalf("form color space = %#v", value)
		}
		return
	}
	t.Fatal("form color space was not yielded")
}

func TestXObjectColorSpacesReportsUnresolvedReference(t *testing.T) {
	x := testXObjectRefStream(Ref{Object: 99}, newStream(nil, []byte("/CS cs")))
	for _, err := range x.ColorSpaces(&Document{objects: map[Ref]Object{}}) {
		if err == nil || err.Error() != "playa: XObject reference could not be resolved" {
			t.Fatalf("unresolved XObject color space error = %v", err)
		}
		return
	}
	t.Fatal("unresolved XObject color space reference produced no error")
}

func TestPageColorSpacesReportsMalformedResource(t *testing.T) {
	d := &Document{}
	p := Page{dict: Dict{
		Name("Contents"):  newStream(nil, []byte("/Bad cs")),
		Name("Resources"): Dict{Name("ColorSpace"): Dict{Name("Bad"): Number(1)}},
	}}
	for _, err := range p.ColorSpaces(d) {
		if err == nil {
			t.Fatal("malformed ColorSpace produced no error")
		}
		return
	}
	t.Fatal("malformed ColorSpace produced no sequence error")
}

func TestPageColorSpacesReportsUnresolvedResourceReferences(t *testing.T) {
	tests := []struct {
		name string
		page Page
		want string
	}{
		{name: "resources", page: Page{dict: Dict{Name("Contents"): newStream(nil, []byte("/CS1 CS")), Name("Resources"): Ref{Object: 99}}}, want: "playa: ColorSpace resources could not be resolved"},
		{name: "entry", page: Page{dict: Dict{Name("Contents"): newStream(nil, []byte("/CS1 CS")), Name("Resources"): Dict{Name("ColorSpace"): Dict{Name("CS1"): Ref{Object: 99}}}}}, want: "playa: ColorSpace resource \"CS1\" could not be resolved"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			for _, err := range test.page.ColorSpaces(&Document{}) {
				if err == nil || err.Error() != test.want {
					t.Fatalf("error = %v, want %q", err, test.want)
				}
				return
			}
			t.Fatal("unresolved ColorSpace reference produced no error")
		})
	}
}

func TestPageInterpReportsUnresolvedColorSpaceReferences(t *testing.T) {
	tests := []struct {
		name string
		page Page
		want string
	}{
		{name: "resources", page: Page{dict: Dict{Name("Contents"): newStream(nil, []byte("/CS1 CS")), Name("Resources"): Dict{Name("ColorSpace"): Ref{Object: 99}}}}, want: "playa: ColorSpace resources could not be resolved"},
		{name: "entry", page: Page{dict: Dict{Name("Contents"): newStream(nil, []byte("/CS1 CS")), Name("Resources"): Dict{Name("ColorSpace"): Dict{Name("CS1"): Ref{Object: 99}}}}}, want: "playa: ColorSpace resource \"CS1\" could not be resolved"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			for object, err := range test.page.Interp(&Document{}, contentconfig.Options{Filter: FilterAll}) {
				if err == nil || object.Kind() != "" || err.Error() != test.want {
					t.Fatalf("object = %#v, error = %v, want %q", object, err, test.want)
				}
				return
			}
			t.Fatal("unresolved ColorSpace reference produced no error")
		})
	}
}
