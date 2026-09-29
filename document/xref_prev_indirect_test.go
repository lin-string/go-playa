package document

import (
	"bytes"
	"testing"
)

func TestPreviousXRefOffsetResolvesIndirectPrev(t *testing.T) {
	d := &Document{
		objects: map[Ref]Object{{Object: 1}: Number(123)},
		trailer: Dict{Name("Prev"): Ref{Object: 1}},
	}
	if got, ok := d.PreviousXRefOffset(); !ok || got != 123 {
		t.Fatalf("previous xref offset = %d, %v", got, ok)
	}
}

func TestXRefHistoryStartsFromIndirectPrev(t *testing.T) {
	d := &Document{
		objects: map[Ref]Object{{Object: 1}: Number(20)},
		trailer: Dict{Name("Prev"): Ref{Object: 1}},
	}
	if got := d.XRefHistory(); len(got) != 1 || got[0] != 20 {
		t.Fatalf("xref history = %v", got)
	}
}

func TestXRefHistoryFollowsIndirectPreviousRevisionChain(t *testing.T) {
	first := []byte("xref\n0 1\n0000000000 65535 f \ntrailer\n<< /Size 1 /Prev 2 0 R >>\n")
	second := []byte("xref\n0 1\n0000000000 65535 f \ntrailer\n<< /Size 1 >>\n")
	data := append(bytes.Repeat([]byte{' '}, 20), first...)
	data = append(data, bytes.Repeat([]byte{' '}, 90-len(data))...)
	data = append(data, second...)
	d := &Document{
		data:    data,
		objects: map[Ref]Object{{Object: 1}: Number(20), {Object: 2}: Number(90)},
		trailer: Dict{Name("Prev"): Ref{Object: 1}},
	}
	got := d.XRefHistory()
	if len(got) != 2 || got[0] != 20 || got[1] != 90 {
		t.Fatalf("xref history = %v, want [20 90]", got)
	}
}
