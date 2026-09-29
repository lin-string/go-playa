package document

import (
	"math"
	"testing"

	"github.com/lin-string/go-playa/contentconfig"
	"github.com/lin-string/go-playa/contentdata"
	"github.com/lin-string/go-playa/geometry"
)

func TestTagPropertiesCopyDoesNotExposeSource(t *testing.T) {
	tag := newTagObject(contentdata.TagSpec{Properties: Dict{Name("Meta"): Dict{Name("Value"): String("original")}}})
	copy := tag.PropertiesCopy()
	copy[Name("Meta")].(Dict)[Name("Value")] = String("changed")
	if got := tag.PropertiesCopy()[Name("Meta")].(Dict)[Name("Value")].(String); string(got) != "original" {
		t.Fatalf("tag properties copy aliases source: %q", got)
	}

	context := newMarkedContentContext("", Dict{Name("Meta"): String("original")}, "", 0, false, nil)
	contextCopy := context.PropertiesCopy()
	contextCopy[Name("Meta")] = String("changed")
	if got := context.PropertiesCopy()[Name("Meta")].(String); string(got) != "original" {
		t.Fatalf("marked context properties copy aliases source: %q", got)
	}
}

func TestPageTagsIgnoresNonFiniteMatrix(t *testing.T) {
	ops := []ContentOp{
		newContentOpBorrowed("cm", []Object{Number(math.Inf(1)), Number(0), Number(0), Number(1), Number(0), Number(0)}, 0),
		newContentOpBorrowed("MP", []Object{Name("Point")}, 0),
	}
	var got TagObject
	count := 0
	index := 0
	err := interpretTagsNext(&Document{}, func() (Dict, error) { return nil, nil }, Ref{}, func() (ContentOp, bool) {
		if index == len(ops) {
			return ContentOp{}, false
		}
		op := ops[index]
		index++
		return op, true
	}, nil, func(tag TagObject) bool {
		got = tag
		count++
		return true
	})
	if err != nil || count != 1 || got.GState().CTM() != identity() {
		t.Fatalf("non-finite tag matrix changed state: err=%v count=%d ctm=%v", err, count, got.GState().CTM())
	}
}

func TestPageTagsExtractsMarkedContentPoints(t *testing.T) {
	d := &Document{objects: map[Ref]Object{}}
	page := Page{dict: Dict{Name("Contents"): newStream(Dict{}, []byte("/Artifact MP /Span << /MCID 7 /ActualText (x) >> DP")), Name("Resources"): Dict{}}}
	tags, err := d.PageTags(page)
	if err != nil {
		t.Fatal(err)
	}
	if len(tags) != 2 || tags[0].Name() != "Artifact" || tags[1].Name() != "Span" || !tags[1].HasMCID() || tags[1].MCID() != 7 || tags[1].ActualText() != "x" {
		t.Fatalf("unexpected tags: %#v", tags)
	}
}

func TestPageTagsSequenceIsRepeatableAndStopsEarly(t *testing.T) {
	d := &Document{}
	page := Page{dict: Dict{Name("Contents"): newStream(nil, []byte("/A MP /B MP"))}}
	first := 0
	for tag, err := range page.Tags(d) {
		if err != nil {
			t.Fatal(err)
		}
		if tag.Name() != "A" {
			t.Fatalf("first tag = %q", tag.Name())
		}
		first++
		break
	}
	second := 0
	for tag, err := range page.Tags(d) {
		if err != nil {
			t.Fatal(err)
		}
		if tag.Name() != "A" && tag.Name() != "B" {
			t.Fatalf("repeated tag = %q", tag.Name())
		}
		second++
	}
	if first != 1 || second != 2 {
		t.Fatalf("tag sequence = first %d, second %d", first, second)
	}
}

func TestPageInterpRestrictOpsPreservesTagState(t *testing.T) {
	d := &Document{}
	p := Page{dict: Dict{Name("Contents"): newStream(nil, []byte("1 0 0 1 10 20 cm /Point MP"))}}
	for object, err := range p.Interp(d, contentconfig.Options{Filter: FilterTag, RestrictOps: []string{"MP"}}) {
		if err != nil {
			t.Fatal(err)
		}
		if object.tag == nil {
			t.Fatalf("restricted object has no tag payload: %#v", object)
		}
		if got := object.tag.GState().CTM(); got != (geometry.Matrix{1, 0, 0, 1, 10, 20}) {
			t.Fatalf("restricted tag CTM = %v, want [1 0 0 1 10 20]", got)
		}
		return
	}
	t.Fatal("restricted tag object was not emitted")
}

func TestPageTagsIncludesFormXObjects(t *testing.T) {
	d := &Document{
		objects: map[Ref]Object{
			{Object: 4}: newStream(Dict{Name("Subtype"): Name("Form")}, []byte("/Span << /MCID 3 >> BDC /Point << /MCID 4 >> DP EMC")),
		},
	}
	p := Page{
		ref:  Ref{Object: 3},
		dict: Dict{Name("Resources"): Dict{Name("XObject"): Dict{Name("Fm1"): Ref{Object: 4}}}, Name("Contents"): newStream(nil, []byte("/Fm1 Do"))},
	}
	tags, err := d.PageTags(p)
	if err != nil {
		t.Fatal(err)
	}
	if len(tags) != 1 || tags[0].Name() != "Point" || !tags[0].HasMCID() || tags[0].MCID() != 4 || tags[0].MarkedTag() != "Span" {
		t.Fatalf("form tags = %#v", tags)
	}
}

func TestPageTagsCarryEnclosingMarkedContent(t *testing.T) {
	page := Page{dict: Dict{Name("Contents"): newStream(nil, []byte("/P BMC /Span << /MCID 9 >> BDC /Point << /MCID 6 >> DP EMC EMC")), Name("Resources"): Dict{}}}
	d := &Document{}
	var tags []TagObject
	for object, err := range page.Interp(d, contentconfig.Options{Filter: FilterTag}) {
		if err != nil {
			t.Fatal(err)
		}
		if object.tag != nil {
			tags = append(tags, *object.tag)
		}
	}
	if len(tags) != 1 {
		t.Fatalf("tag marked context = %#v", tags)
	}
	stack := tags[0].MarkedStackCopy()
	if tags[0].Name() != "Point" || tags[0].MarkedTag() != "Span" || len(stack) != 2 || stack[0].Tag() != "P" || stack[1].Tag() != "Span" {
		t.Fatalf("tag marked context = %#v", tags)
	}
}

func TestPageTagsPreserveOwningPage(t *testing.T) {
	d := &Document{
		trailer: Dict{Name("Root"): Ref{Object: 1}},
		objects: map[Ref]Object{
			{Object: 1}: Dict{Name("Pages"): Ref{Object: 2}},
			{Object: 2}: Dict{Name("Type"): Name("Pages"), Name("Kids"): Array{Ref{Object: 3}}, Name("Count"): Number(1)},
			{Object: 3}: Dict{Name("Type"): Name("Page"), Name("Parent"): Ref{Object: 2}, Name("Contents"): newStream(nil, []byte("/Span MP"))},
		},
	}
	p, err := d.PageAt(0)
	if err != nil {
		t.Fatal(err)
	}
	tags, err := d.PageTags(p)
	if err != nil || len(tags) != 1 || !tags[0].HasPage() || tags[0].Page() != p.ref {
		t.Fatalf("tags = %#v, err = %v", tags, err)
	}
	owned, err := tags[0].PageObject(d)
	if err != nil || owned.ref != p.ref {
		t.Fatalf("page = %#v, err = %v", owned, err)
	}
}

func TestPageTagsSequenceDefersLaterStreamErrors(t *testing.T) {
	d := &Document{}
	p := Page{dict: Dict{Name("Contents"): Array{
		newStream(nil, []byte("/Artifact MP")),
		newStream(nil, []byte("(")),
	}}}
	count := 0
	for tag, err := range d.PageTagsSeq(p) {
		if err != nil {
			t.Fatal(err)
		}
		if tag.Name() != "Artifact" {
			t.Fatalf("tag = %#v", tag)
		}
		count++
		break
	}
	if count != 1 {
		t.Fatalf("tags = %d", count)
	}
}

func TestPageTagsResolveFormPropertyLists(t *testing.T) {
	d := &Document{}
	p := Page{dict: Dict{
		Name("Resources"): Dict{Name("Properties"): Dict{
			Name("P1"): Dict{Name("MCID"): Number(6), Name("ActualText"): String("point")},
		}},
		Name("Contents"): newStream(nil, []byte("/Span /P1 DP")),
	}}
	var tags []TagObject
	for object, err := range p.Interp(d, contentconfig.Options{Filter: FilterTag}) {
		if err != nil {
			t.Fatal(err)
		}
		if object.tag != nil {
			tags = append(tags, *object.tag)
		}
	}
	if len(tags) != 1 || tags[0].MCID() != 6 || tags[0].ActualText() != "point" || tags[0].PropertiesCopy()[Name("MCID")] != Number(6) {
		t.Fatalf("tag properties = %#v", tags)
	}
}

func TestPageTagsResolveNamedPropertyStreams(t *testing.T) {
	d := &Document{}
	p := Page{dict: Dict{
		Name("Resources"): Dict{Name("Properties"): Dict{
			Name("P1"): newStream(Dict{Name("MCID"): Number(8), Name("ActualText"): String("stream")}, nil),
		}},
		Name("Contents"): newStream(nil, []byte("/Span /P1 DP")),
	}}
	var tags []TagObject
	for tag, err := range d.PageTagsSeq(p) {
		if err != nil {
			t.Fatal(err)
		}
		tags = append(tags, tag)
	}
	if len(tags) != 1 || tags[0].MCID() != 8 || tags[0].ActualText() != "stream" {
		t.Fatalf("tag properties = %#v", tags)
	}
}

func TestPageTagsReportsMalformedPropertiesResource(t *testing.T) {
	p := Page{dict: Dict{
		Name("Contents"):  newStream(nil, []byte("/Span /P1 DP")),
		Name("Resources"): Dict{Name("Properties"): Number(1)},
	}}
	for _, err := range (&Document{}).PageTagsSeq(p) {
		if err == nil {
			t.Fatal("malformed Properties resource produced a tag")
		}
		return
	}
	t.Fatal("malformed Properties resource produced no error")
}

func TestPageInterpReportsMalformedPropertiesResource(t *testing.T) {
	p := Page{dict: Dict{
		Name("Contents"):  newStream(nil, []byte("/Span /P1 DP")),
		Name("Resources"): Dict{Name("Properties"): Number(1)},
	}}
	for _, err := range p.Interp(&Document{}, contentconfig.Options{Filter: FilterAll}) {
		if err != nil {
			return
		}
	}
	t.Fatal("FilterAll silently accepted malformed Properties resource")
}
