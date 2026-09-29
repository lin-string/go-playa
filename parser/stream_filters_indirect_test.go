package parser

import "testing"

func TestStreamFiltersWithResolver(t *testing.T) {
	objects := map[Ref]Object{
		{Object: 1}: Name("FlateDecode"),
		{Object: 2}: Dict{Name("Predictor"): Number(12)},
	}
	resolve := func(o Object) Object {
		if ref, ok := o.(Ref); ok {
			return objects[ref]
		}
		return o
	}
	filters, parms := StreamFiltersWithResolver(Dict{
		Name("Filter"):      Ref{Object: 1},
		Name("DecodeParms"): Ref{Object: 2},
	}, resolve)
	if len(filters) != 1 || filters[0] != "FlateDecode" || len(parms) != 1 || parms[0][Name("Predictor")] != Number(12) {
		t.Fatalf("filters=%v parms=%v", filters, parms)
	}
}

func TestStreamFiltersRejectMalformedEntriesOnDecode(t *testing.T) {
	objects := map[Ref]Object{{Object: 1}: Array{Name("FlateDecode"), Number(7)}}
	resolve := func(o Object) Object {
		if ref, ok := o.(Ref); ok {
			return objects[ref]
		}
		return o
	}
	filters, _ := StreamFiltersWithResolver(Dict{Name("Filter"): Ref{Object: 1}}, resolve)
	if _, err := DecodeFilters(nil, filters, nil); err == nil {
		t.Fatalf("malformed filter array was silently ignored: %v", filters)
	}
}

func TestStreamFiltersRejectMalformedDecodeParmsOnDecode(t *testing.T) {
	cases := []Dict{
		{Name("Filter"): Name("FlateDecode"), Name("DecodeParms"): Array{Dict{}, Number(1)}},
		{Name("Filter"): Name("FlateDecode"), Name("DecodeParms"): Array{}},
		{Name("DecodeParms"): Dict{}},
	}
	for _, dict := range cases {
		filters, parms := StreamFilters(dict)
		if _, err := DecodeFilters(nil, filters, parms); err == nil {
			t.Fatalf("malformed DecodeParms was silently ignored: filters=%v parms=%v", filters, parms)
		}
	}
}

func TestStreamFiltersAcceptNullDecodeParmsEntries(t *testing.T) {
	filters, parms := StreamFilters(Dict{
		Name("Filter"):      Array{Name("FlateDecode"), Name("Identity")},
		Name("DecodeParms"): Array{Null{}, Dict{}},
	})
	if len(filters) != 2 || len(parms) != 2 || parms[0] != nil || parms[1] == nil {
		t.Fatalf("null DecodeParms entry = filters:%v parms:%v", filters, parms)
	}
}
