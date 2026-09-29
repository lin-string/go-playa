package document

import (
	"testing"

	"github.com/lin-string/go-playa/contentdata"
)

func newTestExtGState(spec contentdata.ExtGStateSpec) ExtGStateObject {
	return ExtGStateObject{data: contentdata.NewExtGState(spec)}
}

func TestPageExtGStatesFollowsGsSelectionsAndSnapshots(t *testing.T) {
	state := Dict{Name("LW"): Number(2), Name("BM"): Name("Multiply")}
	d := &Document{}
	p := Page{dict: Dict{
		Name("Contents"):  newStream(nil, []byte("/GS1 gs /GS1 gs")),
		Name("Resources"): Dict{Name("ExtGState"): Dict{Name("GS1"): state}},
	}}
	count := 0
	for value, err := range p.ExtGStates(d) {
		if err != nil {
			t.Fatal(err)
		}
		count++
		if value.Name() != "GS1" || value.GState().LineWidth() != 2 || value.GState().BlendMode() != "Multiply" {
			t.Fatalf("extgstate = %#v", value)
		}
		copy := value.DictCopy()
		copy[Name("Marker")] = String("changed")
		if _, ok := value.DictCopy()[Name("Marker")]; ok {
			t.Fatal("ExtGState dictionary copy shares source")
		}
	}
	if count != 2 {
		t.Fatalf("ExtGState selections = %d, want 2", count)
	}
}

func TestXObjectExtGStatesUseFormResources(t *testing.T) {
	d := &Document{}
	x := testXObjectStream(newStream(Dict{Name("Resources"): Dict{Name("ExtGState"): Dict{Name("Local"): Dict{Name("CA"): Number(0.4)}}}}, []byte("/Local gs")))
	for value, err := range x.ExtGStates(d) {
		if err != nil {
			t.Fatal(err)
		}
		if value.Name() != "Local" || value.GState().StrokeAlpha() != 0.4 {
			t.Fatalf("form ExtGState = %#v", value)
		}
		return
	}
	t.Fatal("form ExtGState was not yielded")
}

func TestPageExtGStatesReportsMalformedResource(t *testing.T) {
	d := &Document{}
	p := Page{dict: Dict{
		Name("Contents"):  newStream(nil, []byte("/Bad gs")),
		Name("Resources"): Dict{Name("ExtGState"): Dict{Name("Bad"): Number(1)}},
	}}
	for _, err := range p.ExtGStates(d) {
		if err == nil {
			t.Fatal("malformed ExtGState produced no error")
		}
		return
	}
	t.Fatal("malformed ExtGState produced no sequence error")
}

func TestPageExtGStatesReportsUnresolvedResourceReferences(t *testing.T) {
	tests := []struct {
		name string
		page Page
		want string
	}{
		{name: "resources", page: Page{dict: Dict{Name("Contents"): newStream(nil, []byte("/GS gs")), Name("Resources"): Ref{Object: 99}}}, want: "playa: ExtGState resources could not be resolved"},
		{name: "entry", page: Page{dict: Dict{Name("Contents"): newStream(nil, []byte("/GS gs")), Name("Resources"): Dict{Name("ExtGState"): Dict{Name("GS"): Ref{Object: 99}}}}}, want: "playa: ExtGState resource \"GS\" could not be resolved"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			for _, err := range test.page.ExtGStates(&Document{}) {
				if err == nil || err.Error() != test.want {
					t.Fatalf("error = %v, want %q", err, test.want)
				}
				return
			}
			t.Fatal("unresolved ExtGState reference produced no error")
		})
	}
}
