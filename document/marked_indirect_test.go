package document

import "testing"

func TestPageTextResolvesIndirectMarkedProperties(t *testing.T) {
	d := &Document{objects: map[Ref]Object{
		{Object: 1}: Number(9),
		{Object: 2}: String([]byte{0xfe, 0xff, 0x00, 0x41}),
	}}
	p := Page{dict: Dict{
		Name("Resources"): Dict{Name("Properties"): Dict{Name("P1"): Dict{
			Name("MCID"):       Ref{Object: 1},
			Name("ActualText"): Ref{Object: 2},
		}}},
		Name("Contents"): newStream(nil, []byte("/Span /P1 BDC BT (x) Tj ET EMC")),
	}}
	got, err := d.PageText(p)
	if err != nil || len(got) != 1 || got[0].MCID() != 9 || got[0].ActualText() != "A" {
		t.Fatalf("text=%#v err=%v", got, err)
	}
}
