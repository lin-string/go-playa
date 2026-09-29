package parser

import "testing"

func TestStreamFiltersResolveIndirectFilterAndParameters(t *testing.T) {
	objects := map[Ref]Object{
		{Object: 1}: Name("FlateDecode"),
		{Object: 2}: Dict{Name("Predictor"): Number(12)},
	}
	filters, parameters := StreamFiltersWithResolver(Dict{
		Name("Filter"):      Ref{Object: 1},
		Name("DecodeParms"): Ref{Object: 2},
	}, func(value Object) Object {
		if ref, ok := value.(Ref); ok {
			return objects[ref]
		}
		return value
	})
	if len(filters) != 1 || filters[0] != "FlateDecode" || len(parameters) != 1 || parameters[0][Name("Predictor")] != Number(12) {
		t.Fatalf("filters=%v parameters=%v", filters, parameters)
	}
}
