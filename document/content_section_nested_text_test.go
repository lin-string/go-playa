package document

import (
	"reflect"
	"testing"
)

func TestContentSectionTextsRequireImmediateMarkedID(t *testing.T) {
	d := &Document{}
	p := Page{dict: Dict{
		Name("Resources"): Dict{Name("Font"): Dict{Name("F1"): Dict{Name("Subtype"): Name("Type1"), Name("BaseFont"): Name("Helvetica")}}},
		Name("Contents"):  newStream(nil, []byte("/P << /MCID 7 >> BDC BT /F1 10 Tf (before) Tj ET /Span << /ActualText (replacement) >> BDC BT /F1 10 Tf (nested) Tj (more) Tj ET EMC BT /F1 10 Tf (after) Tj ET EMC")),
	}}
	seq := p.MarkedContentSequence(d)
	section, err := seq.At(7)
	if err != nil {
		t.Fatal(err)
	}
	if section.Len() != 4 {
		t.Fatalf("objects=%d, want all four objects retained", section.Len())
	}
	for o, err := range section.ObjectsSeq() {
		if err != nil {
			t.Fatal(err)
		}
		text, ok := o.textPayload()
		if !ok || !text.HasMCID() || text.MCID() != 7 {
			t.Fatal("nearest text MCID lost")
		}
	}
	for run := 0; run < 2; run++ {
		got := []string{}
		for s := range section.TextsSeq() {
			got = append(got, s)
		}
		if !reflect.DeepEqual(got, []string{"before", "after"}) {
			t.Fatalf("texts=%q, want direct marked text only", got)
		}
		if !reflect.DeepEqual(section.TextsCopy(), got) {
			t.Fatal("copied and iterated texts differ")
		}
	}
}
