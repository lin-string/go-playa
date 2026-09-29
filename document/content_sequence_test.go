package document

import (
	"iter"
	"reflect"
	"testing"

	"github.com/lin-string/go-playa/contentdata"
	"github.com/lin-string/go-playa/internal/testfixture"
)

func TestPageMarkedContentSequenceMatchesPlayaContentSections(t *testing.T) {
	doc, err := Open(testfixture.Path(t, "acceptance_tagged_text.pdf"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = doc.Close() }()

	page, err := doc.PageAt(0)
	if err != nil {
		t.Fatal(err)
	}
	sequence := page.MarkedContentSequence(doc)
	if got, err := sequence.Len(); err != nil || got != 1 {
		t.Fatalf("content sequence length = %d, err=%v; want 1", got, err)
	}
	section, err := sequence.At(0)
	if err != nil {
		t.Fatal(err)
	}
	if !section.HasMCID() || section.MCID() != 0 {
		t.Fatalf("section MCID = %d/%v", section.MCID(), section.HasMCID())
	}
	if section.Len() == 0 {
		t.Fatal("MCID section has no content objects")
	}
	texts := section.TextsCopy()
	if len(texts) != 1 || texts[0] == "" {
		t.Fatalf("section texts = %#v", texts)
	}

	ordered, err := sequence.PageOrderCopy()
	if err != nil {
		t.Fatal(err)
	}
	if len(ordered) != 1 || ordered[0].MCID() != 0 {
		t.Fatalf("page-order sections = %#v", ordered)
	}

	// The sequence is a lazy, repeatable view. A second lookup must not depend
	// on the first section's borrowed object values.
	repeated, err := sequence.At(0)
	if err != nil || repeated.TextsCopy()[0] != texts[0] {
		t.Fatalf("repeated section = %#v, err=%v", repeated, err)
	}
}

func TestContentSequencePreservesEmptyMCIDSlots(t *testing.T) {
	doc := &Document{}
	page := Page{dict: Dict{Name("Contents"): newStream(nil, []byte(
		"/P << /MCID 1 >> BDC EMC /P << /MCID 3 >> BDC (x) Tj EMC",
	))}}
	sequence := page.MarkedContentSequence(doc)
	if got, err := sequence.Len(); err != nil || got != 4 {
		t.Fatalf("content sequence length = %d, err=%v; want 4", got, err)
	}
	for _, mcid := range []int{0, 1, 2} {
		section, err := sequence.At(mcid)
		if err != nil {
			t.Fatal(err)
		}
		if section.Len() != 0 {
			t.Fatalf("empty MCID %d has %d objects", mcid, section.Len())
		}
	}
	section, err := sequence.At(3)
	if err != nil || section.Len() == 0 {
		t.Fatalf("MCID 3 section = %#v, err=%v", section, err)
	}
}

func TestContentSequenceIgnoresResourceEventsBetweenMarkedTextObjects(t *testing.T) {
	d := &Document{}
	page := Page{dict: Dict{
		Name("Contents"): newStream(nil, []byte(
			"/P << /MCID 1 >> BDC BT (A) Tj ET /CS cs "+
				"BT (B) Tj ET EMC",
		)),
		Name("Resources"): Dict{Name("ColorSpace"): Dict{Name("CS"): Name("DeviceRGB")}},
	}}

	section, err := page.MarkedContentSequence(d).At(1)
	if err != nil {
		t.Fatal(err)
	}
	if got := section.TextsCopy(); !reflect.DeepEqual(got, []string{"A", "B"}) {
		t.Fatalf("marked text objects = %#v, want [A B]", got)
	}
}

func TestXObjectMarkedContentSequenceUsesItsOwnContentContext(t *testing.T) {
	xobject := testXObjectStream(newStream(nil, []byte(
		"/Figure << /MCID 4 >> BDC 0 0 m 10 10 l S EMC",
	)))
	sequence := xobject.MarkedContentSequence(&Document{})
	if got, err := sequence.Len(); err != nil || got != 5 {
		t.Fatalf("XObject content sequence length = %d, err=%v; want 5", got, err)
	}
	section, err := sequence.At(4)
	if err != nil || section.Len() == 0 {
		t.Fatalf("XObject MCID 4 section = %#v, err=%v", section, err)
	}
	finalized, err := sequence.FinalizeWithError()
	if err != nil {
		t.Fatal(err)
	}
	if got, err := finalized.Len(); err != nil || got != 5 {
		t.Fatalf("finalized content sequence length = %d, err=%v; want 5", got, err)
	}
	if _, err := (ContentSequence{}).At(0); err != nil {
		t.Fatalf("zero content sequence At(0) error = %v", err)
	}
	if _, err := xobject.MarkedContentSequence(nil).Len(); err != errNilDocument {
		t.Fatalf("nil XObject content sequence error = %v", err)
	}
}

func TestContentSequenceBorrowsObjectsUntilExplicitCopy(t *testing.T) {
	font := &Font{}
	text := newTestText(contentdata.TextSpec{HasMCID: true}, font, nil)
	borrowed := ContentObject{kind: ContentText, text: &text}
	sequence := newContentSequence(func() (iter.Seq2[ContentObject, error], []int) {
		return func(yield func(ContentObject, error) bool) {
			yield(borrowed, nil)
		}, []int{0}
	})

	section, err := sequence.At(0)
	if err != nil {
		t.Fatal(err)
	}
	for object, objectErr := range section.ObjectsSeq() {
		if objectErr != nil {
			t.Fatal(objectErr)
		}
		if object.text == nil || object.text.font != font {
			t.Fatal("content sequence eagerly finalized its borrowed font")
		}
		break
	}

	finalized, err := section.ObjectsCopyWithError()
	if err != nil {
		t.Fatal(err)
	}
	if len(finalized) != 1 || finalized[0].text == nil || finalized[0].text.font == font {
		t.Fatal("explicit content copy did not finalize the font")
	}
}

func TestContentSectionTextsSeqDefersProjectionUntilConsumption(t *testing.T) {
	textValue := newTestText(contentdata.TextSpec{Text: "before", HasMCID: true}, nil, nil)
	text := &textValue
	section := ContentSection{
		mcid:    1,
		hasMCID: true,
		objects: []ContentObject{{kind: ContentText, text: text}},
	}
	sequence := section.TextsSeq()
	setTestTextData(text, func(spec *contentdata.TextSpec) { spec.Text = "after" })

	var got []string
	for value := range sequence {
		got = append(got, value)
	}
	if !reflect.DeepEqual(got, []string{"after"}) {
		t.Fatalf("lazy section text projection = %#v, want [after]", got)
	}
}

func TestContentObjectLengthsMirrorPlayaSizedSemantics(t *testing.T) {
	text := TextObject{glyphs: []GlyphObject{{}, {}}}
	if got := text.Len(); got != 2 {
		t.Fatalf("text length = %d, want 2", got)
	}
	if got, err := (ContentObject{kind: ContentText, text: &text}).Len(nil); err != nil || got != 2 {
		t.Fatalf("content text length = %d, err=%v; want 2", got, err)
	}
	if got := (PathObject{}).Len(); got != 0 {
		t.Fatalf("path length = %d, want 0", got)
	}
	if got := (ImageObject{}).Len(); got != 0 {
		t.Fatalf("image length = %d, want 0", got)
	}
	if got := (TagObject{}).Len(); got != 0 {
		t.Fatalf("tag length = %d, want 0", got)
	}

	xobject := testXObjectStream(newStream(nil, []byte("0 0 m 10 10 l S")))
	got, err := xobject.Len(&Document{})
	if err != nil || got != 1 {
		t.Fatalf("XObject length = %d, err=%v; want 1", got, err)
	}
	if _, err := xobject.Len(nil); err != errNilDocument {
		t.Fatalf("nil XObject length error = %v", err)
	}
}
