package document

import (
	"testing"

	"github.com/lin-string/go-playa/contentconfig"
)

func TestPagePropertiesFollowsNamedBDCSelections(t *testing.T) {
	d := &Document{}
	p := Page{dict: Dict{
		Name("Contents"):  newStream(nil, []byte("/Tag /P BDC EMC /Tag /P DP")),
		Name("Resources"): Dict{Name("Properties"): Dict{Name("P"): Dict{Name("MCID"): Number(7), Name("Role"): Name("P")}}},
	}}
	var values []PropertiesObject
	for value, err := range p.Properties(d) {
		if err != nil {
			t.Fatal(err)
		}
		values = append(values, value)
	}
	if len(values) != 2 || values[0].Name() != "P" || values[0].Operator() != "BDC" || values[1].Operator() != "DP" {
		t.Fatalf("property selections = %#v", values)
	}
	if values[0].DictCopy()[Name("MCID")] != Number(7) {
		t.Fatalf("property dictionary = %#v", values[0].DictCopy())
	}
}

func TestXObjectPropertiesUseFormResources(t *testing.T) {
	d := &Document{}
	x := testXObjectStream(newStream(Dict{Name("Resources"): Dict{Name("Properties"): Dict{Name("Local"): Dict{Name("MCID"): Number(3)}}}}, []byte("/Tag /Local BDC EMC")))
	for value, err := range x.Properties(d) {
		if err != nil {
			t.Fatal(err)
		}
		if value.Name() != "Local" || value.DictCopy()[Name("MCID")] != Number(3) {
			t.Fatalf("form property = %#v", value)
		}
		return
	}
	t.Fatal("form property was not yielded")
}

func TestPagePropertiesReportsMalformedResource(t *testing.T) {
	d := &Document{}
	p := Page{dict: Dict{
		Name("Contents"):  newStream(nil, []byte("/Tag /Bad BDC EMC")),
		Name("Resources"): Dict{Name("Properties"): Dict{Name("Bad"): Number(1)}},
	}}
	for _, err := range p.Properties(d) {
		if err == nil {
			t.Fatal("malformed Properties resource produced no error")
		}
		return
	}
	t.Fatal("malformed Properties resource produced no sequence error")
}

func TestPagePropertiesReportsUnresolvedReferences(t *testing.T) {
	tests := []struct {
		name string
		page Page
		want string
	}{
		{
			name: "resources",
			page: Page{dict: Dict{
				Name("Contents"):  newStream(nil, []byte("/Tag /P BDC EMC")),
				Name("Resources"): Dict{Name("Properties"): Ref{Object: 99}},
			}},
			want: "playa: Properties resources could not be resolved",
		},
		{
			name: "entry",
			page: Page{dict: Dict{
				Name("Contents"):  newStream(nil, []byte("/Tag /P BDC EMC")),
				Name("Resources"): Dict{Name("Properties"): Dict{Name("P"): Ref{Object: 99}}},
			}},
			want: "playa: property resource \"P\" could not be resolved",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			for object, err := range test.page.Properties(&Document{}) {
				if err == nil || object.Name() != "" || err.Error() != test.want {
					t.Fatalf("object = %#v, error = %v, want %q", object, err, test.want)
				}
				return
			}
			t.Fatal("unresolved Properties reference produced no sequence error")
		})
	}
}

func TestPageInterpReportsUnresolvedPropertiesReferences(t *testing.T) {
	tests := []struct {
		name string
		page Page
		want string
	}{
		{name: "resources", page: Page{dict: Dict{Name("Contents"): newStream(nil, []byte("/Tag /P BDC EMC")), Name("Resources"): Dict{Name("Properties"): Ref{Object: 99}}}}, want: "playa: Properties resources could not be resolved"},
		{name: "entry", page: Page{dict: Dict{Name("Contents"): newStream(nil, []byte("/Tag /P BDC EMC")), Name("Resources"): Dict{Name("Properties"): Dict{Name("P"): Ref{Object: 99}}}}}, want: "playa: property resource \"P\" could not be resolved"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			for object, err := range test.page.Interp(&Document{}, contentconfig.Options{Filter: FilterAll}) {
				if err == nil || object.Kind() != "" || err.Error() != test.want {
					t.Fatalf("object = %#v, error = %v, want %q", object, err, test.want)
				}
				return
			}
			t.Fatal("unresolved Properties reference produced no error")
		})
	}
}
