package contentdata

import (
	"testing"

	"github.com/lin-string/go-playa/pdftypes/primitives"
)

func TestPatternAndShadingOwnDependencyFreeSnapshots(t *testing.T) {
	dict := primitives.Dict{primitives.Name("PatternType"): primitives.Number(1)}
	pattern := NewPattern(PatternSpec{
		Name: "P1", Page: primitives.Ref{Object: 3}, HasPage: true,
		Stroke: true, GState: DefaultGraphicsState(), Dict: dict,
	})
	shading := NewShading(ShadingSpec{
		Name: "S1", Page: primitives.Ref{Object: 3}, HasPage: true,
		GState: DefaultGraphicsState(), Dict: dict,
	})
	dict[primitives.Name("PatternType")] = primitives.Number(2)

	if pattern.Name() != "P1" || !pattern.Stroke() || pattern.Page().Object != 3 {
		t.Fatalf("pattern metadata = %#v", pattern)
	}
	if shading.Name() != "S1" || shading.Page().Object != 3 {
		t.Fatalf("shading metadata = %#v", shading)
	}
	if got, _ := pattern.DictCopy()[primitives.Name("PatternType")].(primitives.Number); got != 1 {
		t.Fatalf("pattern DictCopy() = %v", got)
	}
	if got, _ := shading.DictCopy()[primitives.Name("PatternType")].(primitives.Number); got != 1 {
		t.Fatalf("shading DictCopy() = %v", got)
	}
}
