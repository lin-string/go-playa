package structuredata_test

import (
	"testing"

	"github.com/lin-string/go-playa/pdftypes/primitives"
	"github.com/lin-string/go-playa/structuredata"
)

func TestElementOwnsDependencyFreeState(t *testing.T) {
	element := structuredata.NewElement(structuredata.ElementSpec{
		Type:          "StructElem",
		StructureType: "P",
		Role:          "P",
		RawRole:       "P",
		Title:         "paragraph",
		Language:      "en-US",
		Alt:           "alternate",
		ActualText:    "actual",
		Abbreviation:  "abbr",
		ClassName:     "class",
		Page:          primitives.Ref{Object: 2},
		HasPage:       true,
		Parent:        primitives.Ref{Object: 3},
		HasParent:     true,
		MCID:          7,
		HasMCID:       true,
		IsMCR:         true,
		ObjectRef:     primitives.Ref{Object: 4},
		HasObject:     true,
		BBox:          [4]float64{1, 2, 3, 4},
		HasBBox:       true,
		Attributes:    primitives.Dict{primitives.Name("Lang"): primitives.String("en-US")},
		Dict:          primitives.Dict{primitives.Name("T"): primitives.String("paragraph")},
		Object:        primitives.Dict{primitives.Name("Type"): primitives.Name("OBJR")},
	})
	if element.Type() != "StructElem" || element.Role() != "P" || element.Title() != "paragraph" || element.Language() != "en-US" || element.AlternateDescription() != "alternate" || element.ActualText() != "actual" || element.AbbreviationExpansion() != "abbr" || element.ClassName() != "class" {
		t.Fatalf("element text metadata = %#v", element)
	}
	if !element.HasPage() || element.Page() != (primitives.Ref{Object: 2}) || !element.HasParent() || element.Parent() != (primitives.Ref{Object: 3}) || !element.HasMCID() || element.MCID() != 7 || !element.IsMCR() || !element.HasObject() || !element.HasBBox() {
		t.Fatalf("element relations = %#v", element)
	}
	attribute, _ := element.AttributesCopy()[primitives.Name("Lang")].(primitives.String)
	dictValue, _ := element.DictCopy()[primitives.Name("T")].(primitives.String)
	if string(attribute) != "en-US" || string(dictValue) != "paragraph" || element.ObjectCopy().(primitives.Dict)[primitives.Name("Type")] != primitives.Name("OBJR") {
		t.Fatalf("element objects = %#v", element)
	}
	if element.Finalize().Role() != "P" {
		t.Fatal("element finalize lost role")
	}
}
