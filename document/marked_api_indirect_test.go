package document

import "testing"

func TestPageMarkedContentResolvesIndirectProperties(t *testing.T) {
	d := &Document{objects: map[Ref]Object{
		{Object: 1}: Number(4),
		{Object: 2}: String([]byte{0xfe, 0xff, 0x00, 0x42}),
	}}
	p := Page{dict: Dict{
		Name("Resources"): Dict{Name("Properties"): Dict{Name("P1"): Dict{
			Name("MCID"):       Ref{Object: 1},
			Name("ActualText"): Ref{Object: 2},
		}}},
		Name("Contents"): newStream(nil, []byte("/Span /P1 BDC (x) Tj EMC")),
	}}
	got, err := d.PageMarkedContent(p)
	if err != nil || len(got) != 1 || got[0].MCID() != 4 || got[0].ActualText() != "B" {
		t.Fatalf("marked=%#v err=%v", got, err)
	}
}

func TestPageMarkedContentStopsBeforeMalformedLaterStream(t *testing.T) {
	d := &Document{}
	p := Page{dict: Dict{Name("Contents"): Array{
		newStream(nil, []byte("/Span BMC EMC")),
		newStream(nil, []byte("(")),
	}}}

	count := 0
	for item, err := range p.MarkedContent(d) {
		if err != nil {
			t.Fatal(err)
		}
		if item.Tag() != "Span" {
			t.Fatalf("marked content = %#v", item)
		}
		count++
		break
	}
	if count != 1 {
		t.Fatalf("consumed %d marked sections, want 1", count)
	}
}

func TestPageMarkedContentSequenceIsRepeatableAndStopsEarly(t *testing.T) {
	d := &Document{}
	p := Page{dict: Dict{Name("Contents"): newStream(nil, []byte(
		"/First BMC 1 0 m EMC /Second BMC 2 0 l EMC",
	))}}
	first := 0
	for item, err := range p.MarkedContent(d) {
		if err != nil {
			t.Fatal(err)
		}
		if item.Tag() != "First" {
			t.Fatalf("first marked content = %#v", item)
		}
		first++
		break
	}
	second := 0
	for item, err := range p.MarkedContent(d) {
		if err != nil {
			t.Fatal(err)
		}
		if item.Tag() != "First" && item.Tag() != "Second" {
			t.Fatalf("repeated marked content = %#v", item)
		}
		second++
	}
	if first != 1 || second != 2 {
		t.Fatalf("marked-content sequence counts = first %d, second %d", first, second)
	}
}
