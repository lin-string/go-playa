package document

import (
	"testing"

	"github.com/lin-string/go-playa/contentdata"
)

func TestResourceMetadataAccessorsReturnReadOnlyValues(t *testing.T) {
	state := newGraphicsState()
	state.lineWidth = 2
	page := Ref{Object: 7}

	shading := ShadingObject{data: contentdata.NewShading(contentdata.ShadingSpec{Name: "S1", Page: page, HasPage: true, GState: state.publicValue()})}
	if shading.Name() != "S1" || shading.Page() != page || !shading.HasPage() || shading.GState().LineWidth() != 2 {
		t.Fatalf("shading metadata = %#v", shading)
	}
	shadingState := shading.GState()
	_ = shadingState.Finalize()
	if shading.GState().LineWidth() != 2 {
		t.Fatal("shading GState accessor aliases the source")
	}

	pattern := PatternObject{data: contentdata.NewPattern(contentdata.PatternSpec{Name: "P1", Page: page, HasPage: true, Stroke: true, GState: state.publicValue()})}
	if pattern.Name() != "P1" || pattern.Page() != page || !pattern.HasPage() || !pattern.Stroke() || pattern.GState().LineWidth() != 2 {
		t.Fatalf("pattern metadata = %#v", pattern)
	}

	extgstate := newTestExtGState(contentdata.ExtGStateSpec{Name: "GS1", Page: page, HasPage: true, GState: state.publicValue()})
	if extgstate.Name() != "GS1" || extgstate.Page() != page || !extgstate.HasPage() || extgstate.GState().LineWidth() != 2 {
		t.Fatalf("ExtGState metadata = %#v", extgstate)
	}

	properties := newPropertiesObject("Props", "BDC", page, true, nil)
	if properties.Name() != "Props" || properties.Operator() != "BDC" || properties.Page() != page || !properties.HasPage() {
		t.Fatalf("properties metadata = %#v", properties)
	}
}
