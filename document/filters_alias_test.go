package document

import "testing"

func TestStreamFiltersAcceptsLegacyAliases(t *testing.T) {
	filters, parms := streamFilters(Dict{
		Name("F"):  Name("FlateDecode"),
		Name("DP"): Dict{Name("Predictor"): Number(1)},
	})
	if len(filters) != 1 || filters[0] != "FlateDecode" || len(parms) != 1 {
		t.Fatalf("filters=%v parms=%v", filters, parms)
	}
}
