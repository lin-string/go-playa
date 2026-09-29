package document

import (
	"strings"
	"testing"

	"github.com/lin-string/go-playa/documentdata"
)

func TestPageLabelsWalksNumberTreeKids(t *testing.T) {
	d := &Document{
		trailer: Dict{Name("Root"): Ref{Object: 1, Generation: 0}},
		objects: map[Ref]Object{
			{Object: 1, Generation: 0}:  Dict{Name("PageLabels"): Ref{Object: 2, Generation: 0}},
			{Object: 2, Generation: 0}:  Dict{Name("Kids"): Array{Ref{Object: 3, Generation: 0}, Ref{Object: 4, Generation: 0}}},
			{Object: 3, Generation: 0}:  Dict{Name("Nums"): Array{Number(0), Dict{Name("S"): Name("r"), Name("St"): Number(3)}}},
			{Object: 4, Generation: 0}:  Dict{Name("Nums"): Array{Number(1), Dict{Name("P"): String([]byte("P-")), Name("S"): Name("D")}}},
			{Object: 10, Generation: 0}: Dict{Name("Type"): Name("Page")},
			{Object: 11, Generation: 0}: Dict{Name("Type"): Name("Page")},
		},
	}
	got, err := d.PageLabels()
	if err != nil || len(got) != 2 || got[0] != "iii" || got[1] != "P-1" {
		t.Fatalf("labels = %#v, err = %v", got, err)
	}
}

func TestPageLabelsFollowMultiLevelIndirectTreeValues(t *testing.T) {
	d := &Document{trailer: Dict{Name("Root"): Ref{Object: 1}}, objects: map[Ref]Object{}}
	d.objects[Ref{Object: 1}] = Dict{Name("PageLabels"): Ref{Object: 2}, Name("Pages"): Ref{Object: 10}}
	d.objects[Ref{Object: 2}] = Dict{Name("Kids"): Ref{Object: 3}}
	d.objects[Ref{Object: 3}] = Array{Ref{Object: 4}}
	d.objects[Ref{Object: 4}] = Ref{Object: 5}
	d.objects[Ref{Object: 5}] = Dict{Name("Nums"): Ref{Object: 6}}
	d.objects[Ref{Object: 6}] = Array{Number(0), Ref{Object: 7}}
	d.objects[Ref{Object: 7}] = Dict{Name("P"): Ref{Object: 8}, Name("S"): Ref{Object: 9}}
	d.objects[Ref{Object: 8}] = String("P-")
	d.objects[Ref{Object: 9}] = Name("D")
	d.objects[Ref{Object: 10}] = Dict{Name("Type"): Name("Pages"), Name("Kids"): Array{Ref{Object: 11}}}
	d.objects[Ref{Object: 11}] = Dict{Name("Type"): Name("Page")}

	labels, err := d.PageLabels()
	if err != nil || len(labels) != 1 || labels[0] != "P-1" {
		t.Fatalf("labels = %#v, err = %v", labels, err)
	}
}

func TestPageLabelsMissingZeroUsesFirstRuleFromPageZero(t *testing.T) {
	d := &Document{
		trailer: Dict{Name("Root"): Ref{Object: 1}},
		objects: map[Ref]Object{
			{Object: 1}: Dict{Name("PageLabels"): Ref{Object: 5}, Name("Pages"): Ref{Object: 2}},
			{Object: 2}: Dict{Name("Type"): Name("Pages"), Name("Kids"): Array{Ref{Object: 3}, Ref{Object: 4}, Ref{Object: 6}}, Name("Count"): Number(3)},
			{Object: 3}: Dict{Name("Type"): Name("Page"), Name("Parent"): Ref{Object: 2}},
			{Object: 4}: Dict{Name("Type"): Name("Page"), Name("Parent"): Ref{Object: 2}},
			{Object: 5}: Dict{Name("Nums"): Array{Number(2), Dict{Name("S"): Name("D"), Name("St"): Number(7)}}},
			{Object: 6}: Dict{Name("Type"): Name("Page"), Name("Parent"): Ref{Object: 2}},
		},
	}
	labels, err := d.PageLabels()
	if err != nil || len(labels) != 3 || labels[0] != "7" || labels[1] != "8" || labels[2] != "9" {
		t.Fatalf("labels = %#v, err = %v, want first rule from page zero", labels, err)
	}
	var lazy []string
	for label, err := range d.PageLabelsSeq() {
		if err != nil {
			t.Fatal(err)
		}
		lazy = append(lazy, label)
	}
	if len(lazy) != 3 || lazy[0] != "7" || lazy[1] != "8" || lazy[2] != "9" {
		t.Fatalf("lazy labels = %#v, want first rule from page zero", lazy)
	}
}

func TestPageLabelsDoNotRevisitSharedIndirectNodes(t *testing.T) {
	d := &Document{trailer: Dict{Name("Root"): Ref{Object: 1}}, objects: map[Ref]Object{
		{Object: 1}:  Dict{Name("PageLabels"): Ref{Object: 2}, Name("Pages"): Ref{Object: 10}},
		{Object: 2}:  Dict{Name("Kids"): Array{Ref{Object: 3}, Ref{Object: 5}}},
		{Object: 3}:  Ref{Object: 4},
		{Object: 4}:  Dict{Name("Nums"): Array{Number(0), Dict{Name("P"): String("shared-")}}},
		{Object: 5}:  Ref{Object: 4},
		{Object: 10}: Dict{Name("Type"): Name("Pages"), Name("Kids"): Array{Ref{Object: 11}}},
		{Object: 11}: Dict{Name("Type"): Name("Page")},
	}}
	labels, err := d.PageLabels()
	if err != nil || len(labels) != 1 || labels[0] != "shared-" {
		t.Fatalf("shared page-label nodes = %#v, err = %v", labels, err)
	}
}

func TestPageLabelSpecFormatsOffsets(t *testing.T) {
	if got := documentdata.NewPageLabelSpec("D", "p-", 3).Format(2); got != "p-5" {
		t.Fatalf("decimal label = %q", got)
	}
	if got := documentdata.NewPageLabelSpec("R", "", 4).Format(0); got != "IV" {
		t.Fatalf("roman label = %q", got)
	}
	if got := documentdata.NewPageLabelSpec("a", "").Format(0); got != "a" {
		t.Fatalf("default start label = %q", got)
	}
	if got := documentdata.NewPageLabelSpec("D", "").Format(-1); got != "" {
		t.Fatalf("negative offset label = %q", got)
	}
}

func TestPageLabelsRejectsNumberTreeCycle(t *testing.T) {
	d := &Document{
		trailer: Dict{Name("Root"): Ref{Object: 1, Generation: 0}},
		objects: map[Ref]Object{
			{Object: 1, Generation: 0}:  Dict{Name("PageLabels"): Ref{Object: 2, Generation: 0}},
			{Object: 2, Generation: 0}:  Dict{Name("Kids"): Array{Ref{Object: 2, Generation: 0}}},
			{Object: 10, Generation: 0}: Dict{Name("Type"): Name("Page")},
		},
	}
	got, err := d.PageLabels()
	if err != nil || len(got) != 1 || got[0] != "1" {
		t.Fatalf("labels from cyclic tree = %#v, err = %v", got, err)
	}
}

func TestPageLabelsSequenceReportsMalformedNumberTree(t *testing.T) {
	tests := []struct {
		name string
		tree Dict
		want string
	}{
		{name: "odd Nums", tree: Dict{Name("Nums"): Array{Number(0)}}, want: "unmatched key"},
		{name: "non-integer key", tree: Dict{Name("Nums"): Array{String("zero"), Dict{}}}, want: "key 0 is not an integer"},
		{name: "non-dictionary value", tree: Dict{Name("Nums"): Array{Number(0), Number(1)}}, want: "value 0 is not a dictionary"},
		{name: "non-array Kids", tree: Dict{Name("Kids"): Number(1)}, want: "Kids is not an array"},
		{name: "non-dictionary child", tree: Dict{Name("Kids"): Array{Number(1)}}, want: "child is not a dictionary"},
		{name: "mixed node", tree: Dict{Name("Nums"): Array{Number(0), Dict{}}, Name("Kids"): Array{}}, want: "cannot contain both Nums and Kids"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			d := &Document{
				trailer: Dict{Name("Root"): Ref{Object: 1, Generation: 0}},
				objects: map[Ref]Object{
					{Object: 1, Generation: 0}: Dict{Name("PageLabels"): test.tree, Name("Pages"): Ref{Object: 2, Generation: 0}},
					{Object: 2, Generation: 0}: Dict{Name("Type"): Name("Pages"), Name("Kids"): Array{Ref{Object: 3, Generation: 0}}, Name("Count"): Number(1)},
					{Object: 3, Generation: 0}: Dict{Name("Type"): Name("Page"), Name("Parent"): Ref{Object: 2, Generation: 0}},
				},
			}
			for _, err := range d.PageLabelsSeq() {
				if err == nil {
					t.Fatal("malformed PageLabels tree produced a value")
				}
				if !strings.Contains(err.Error(), test.want) {
					t.Fatalf("error = %v, want substring %q", err, test.want)
				}
				return
			}
			t.Fatal("malformed PageLabels tree produced no error")
		})
	}
}

func TestPageLabelsMaterializationDoesNotPublishPartialMalformedTree(t *testing.T) {
	d := &Document{
		trailer: Dict{Name("Root"): Ref{Object: 1}},
		objects: map[Ref]Object{
			{Object: 1}: Dict{Name("PageLabels"): Dict{Name("Nums"): Array{
				Number(0), Dict{Name("S"): Name("r")}, Number(1),
			}}, Name("Pages"): Ref{Object: 2}},
			{Object: 2}: Dict{Name("Type"): Name("Pages"), Name("Kids"): Array{Ref{Object: 3}}, Name("Count"): Number(1)},
			{Object: 3}: Dict{Name("Type"): Name("Page"), Name("Parent"): Ref{Object: 2}},
		},
	}
	if got, err := d.PageLabels(); err == nil || got != nil {
		t.Fatalf("labels from malformed tree = %#v, err = %v", got, err)
	}
	for _, err := range d.pageLabelItemsSeq() {
		if err == nil {
			t.Fatal("cached malformed PageLabels tree produced no error")
		}
		return
	}
	t.Fatal("cached malformed PageLabels tree produced no error")
}

func TestPageLabelsSequenceReportsMalformedCatalogRoot(t *testing.T) {
	d := &Document{trailer: Dict{Name("Root"): Number(1)}}
	for _, err := range d.pageLabelItemsSeq() {
		if err == nil {
			t.Fatal("malformed catalog root produced a label")
		}
		return
	}
	t.Fatal("malformed catalog root produced no error")
}

func TestPageLabelsSequenceReportsMalformedPageLabelsRoot(t *testing.T) {
	d := &Document{trailer: Dict{Name("Root"): Dict{Name("PageLabels"): Number(1)}}}
	for _, err := range d.pageLabelItemsSeq() {
		if err == nil {
			t.Fatal("malformed PageLabels root produced a label")
		}
		break
	}
	if !d.pageLabelsReady || d.pageLabelsErr == nil {
		t.Fatalf("malformed PageLabels root was not cached: ready=%v err=%v", d.pageLabelsReady, d.pageLabelsErr)
	}
	for _, err := range d.pageLabelItemsSeq() {
		if err == nil || err != d.pageLabelsErr {
			t.Fatalf("cached malformed PageLabels root error = %v, want %v", err, d.pageLabelsErr)
		}
		return
	}
	t.Fatal("cached malformed PageLabels root produced no error")
}

func TestPageLabelsSequenceReportsUnresolvedEntries(t *testing.T) {
	d := &Document{
		trailer: Dict{Name("Root"): Ref{Object: 1}},
		objects: map[Ref]Object{
			{Object: 1}: Dict{Name("PageLabels"): Ref{Object: 2}},
			{Object: 2}: Dict{Name("Nums"): Ref{Object: 99}},
		},
	}
	for _, err := range d.pageLabelItemsSeq() {
		if err == nil || err.Error() != "playa: PageLabels Nums could not be resolved" {
			t.Fatalf("unresolved Nums error = %v", err)
		}
		return
	}
	t.Fatal("unresolved Nums produced no error")
}

func TestPageLabelsRejectsUnsortedOrNegativeNumsKeys(t *testing.T) {
	for name, nums := range map[string]Array{
		"unsorted": {Number(2), Dict{}, Number(1), Dict{}},
		"negative": {Number(-1), Dict{}},
	} {
		t.Run(name, func(t *testing.T) {
			d := &Document{trailer: Dict{Name("Root"): Dict{Name("PageLabels"): Dict{Name("Nums"): nums}}}}
			if labels, err := d.PageLabels(); err == nil || labels != nil {
				t.Fatalf("page labels = %#v, err=%v", labels, err)
			}
		})
	}
}

func TestPageLabelsRejectsMalformedLimits(t *testing.T) {
	d := &Document{trailer: Dict{Name("Root"): Dict{Name("PageLabels"): Dict{Name("Limits"): Array{Number(2), Number(1)}}}}}
	if labels, err := d.PageLabels(); err == nil || labels != nil {
		t.Fatalf("malformed page-label limits = %#v, err=%v", labels, err)
	}
}

func TestPageLabelsRejectsChildLimitsOutsideParent(t *testing.T) {
	d := &Document{trailer: Dict{Name("Root"): Dict{Name("PageLabels"): Dict{
		Name("Limits"): Array{Number(0), Number(4)},
		Name("Kids"):   Array{Dict{Name("Limits"): Array{Number(5), Number(6)}}},
	}}}}
	if labels, err := d.PageLabels(); err == nil || labels != nil {
		t.Fatalf("out-of-range page-label limits = %#v, err=%v", labels, err)
	}
}

func TestPageLabelsSequenceReportsUnresolvedRuleFields(t *testing.T) {
	d := &Document{
		trailer: Dict{Name("Root"): Ref{Object: 1}},
		objects: map[Ref]Object{
			{Object: 1}: Dict{Name("PageLabels"): Ref{Object: 2}},
			{Object: 2}: Dict{Name("Nums"): Array{Number(0), Dict{Name("P"): Ref{Object: 99}}}},
		},
	}
	for _, err := range d.pageLabelItemsSeq() {
		if err == nil || err.Error() != "playa: PageLabels prefix could not be resolved" {
			t.Fatalf("unresolved prefix error = %v", err)
		}
		return
	}
	t.Fatal("unresolved prefix produced no error")
}

func TestPageLabelWithErrorRejectsNilDocument(t *testing.T) {
	if _, err := (Page{number: 1}).LabelWithError(nil); err == nil {
		t.Fatal("expected nil document error")
	}
}

func TestPageLabelConvenienceMethodsReportMalformedTree(t *testing.T) {
	d := &Document{
		trailer: Dict{Name("Root"): Ref{Object: 1}},
		objects: map[Ref]Object{
			{Object: 1}: Dict{Name("PageLabels"): Number(1), Name("Pages"): Ref{Object: 2}},
			{Object: 2}: Dict{Name("Type"): Name("Pages"), Name("Kids"): Array{Ref{Object: 3}}, Name("Count"): Number(1)},
			{Object: 3}: Dict{Name("Type"): Name("Page"), Name("Parent"): Ref{Object: 2}},
		},
	}
	if label, err := d.PageLabelWithError(0); err == nil || label != "" {
		t.Fatalf("document label = %q, err = %v", label, err)
	}
	if label, err := (Page{number: 1}).LabelWithError(d); err == nil || label != "" {
		t.Fatalf("page label = %q, err = %v", label, err)
	}
	if label := d.PageLabel(0); label != "" || (Page{number: 1}).Label(d) != "" {
		t.Fatal("compatibility label methods returned a label for malformed tree")
	}
}

func TestPageLabelsClampInvalidStartNumber(t *testing.T) {
	d := &Document{
		trailer: Dict{Name("Root"): Ref{Object: 1}},
		objects: map[Ref]Object{
			{Object: 1}: Dict{Name("Pages"): Ref{Object: 2}, Name("PageLabels"): Ref{Object: 5}},
			{Object: 2}: Dict{Name("Type"): Name("Pages"), Name("Kids"): Array{Ref{Object: 3}}, Name("Count"): Number(1)},
			{Object: 3}: Dict{Name("Type"): Name("Page"), Name("Parent"): Ref{Object: 2}},
			{Object: 5}: Dict{Name("Nums"): Array{Number(0), Dict{Name("S"): Name("r"), Name("St"): Number(0)}}},
		},
	}
	if got, err := d.PageLabels(); err != nil || len(got) != 1 || got[0] != "i" {
		t.Fatalf("materialized labels = %#v, err = %v", got, err)
	}
	for label, err := range d.PageLabelsSeq() {
		if err != nil || label != "i" {
			t.Fatalf("lazy label = %q, err = %v", label, err)
		}
		break
	}
}

func TestPageAndDocumentExposeIndividualLabels(t *testing.T) {
	d := &Document{
		trailer: Dict{Name("Root"): Ref{Object: 1, Generation: 0}},
		objects: map[Ref]Object{
			{Object: 1, Generation: 0}: Dict{Name("PageLabels"): Ref{Object: 2, Generation: 0}},
			{Object: 2, Generation: 0}: Dict{Name("Nums"): Array{Number(0), Dict{Name("S"): Name("R"), Name("St"): Number(4)}}},
			{Object: 3, Generation: 0}: Dict{Name("Type"): Name("Page")},
		},
	}
	page := Page{number: 1}
	if page.Label(d) != "IV" || d.PageLabel(0) != "IV" || d.PageLabel(1) != "" {
		t.Fatalf("individual labels = %q, %q, %q", page.Label(d), d.PageLabel(0), d.PageLabel(1))
	}
}

func TestPageLabelDefaultsToOneBasedPageNumber(t *testing.T) {
	d := &Document{}
	if got := (Page{number: 1}).Label(d); got != "1" {
		t.Fatalf("default first page label = %q", got)
	}
	if got := (Page{number: 3}).Label(d); got != "3" {
		t.Fatalf("default third page label = %q", got)
	}
}

func TestPageLabelsSequenceIsRepeatableAndStopsEarly(t *testing.T) {
	d := &Document{
		trailer: Dict{Name("Root"): Ref{Object: 1}},
		objects: map[Ref]Object{
			{Object: 1}: Dict{Name("PageLabels"): Ref{Object: 2}},
			{Object: 2}: Dict{Name("Nums"): Array{Number(0), Dict{Name("S"): Name("D"), Name("P"): String([]byte("P-"))}}},
			{Object: 3}: Dict{Name("Type"): Name("Pages"), Name("Kids"): Array{Ref{Object: 4}, Ref{Object: 5}, Ref{Object: 6}}},
			{Object: 4}: Dict{Name("Type"): Name("Page")},
			{Object: 5}: Dict{Name("Type"): Name("Page")},
			{Object: 6}: Dict{Name("Type"): Name("Page")},
		},
	}
	// The fallback page scanner is used when the catalog has no /Pages entry.
	first := []string{}
	for label, err := range d.PageLabelsSeq() {
		if err != nil {
			t.Fatal(err)
		}
		first = append(first, label)
		if len(first) == 1 {
			break
		}
	}
	if len(first) != 1 || first[0] != "P-1" {
		t.Fatalf("early labels = %#v", first)
	}
	var all []string
	for label, err := range d.PageLabelsSeq() {
		if err != nil {
			t.Fatal(err)
		}
		all = append(all, label)
	}
	if len(all) != 3 || all[2] != "P-3" {
		t.Fatalf("all labels = %#v", all)
	}
}

func TestPageLabelsReuseNumberTreeCache(t *testing.T) {
	d := &Document{
		trailer: Dict{Name("Root"): Ref{Object: 1}},
		objects: map[Ref]Object{
			{Object: 1}: Dict{Name("PageLabels"): Ref{Object: 2}},
			{Object: 2}: Dict{Name("Nums"): Array{Number(0), Dict{Name("S"): Name("D")}}},
		},
	}
	_, _ = d.PageLabels()
	first := d.pageLabelCache
	_, _ = d.PageLabels()
	if !d.pageLabelsReady || len(first) != 1 || len(d.pageLabelCache) != 1 {
		t.Fatalf("page label cache = %#v, ready=%v", d.pageLabelCache, d.pageLabelsReady)
	}
	if len(first) > 0 && &first[0] != &d.pageLabelCache[0] {
		t.Fatal("page label cache was rebuilt")
	}
}

func TestPageLabelsDoNotRetainOversizedRuleCache(t *testing.T) {
	d := &Document{
		trailer: Dict{Name("Root"): Ref{Object: 1}},
		objects: map[Ref]Object{
			{Object: 1}: Dict{Name("PageLabels"): Dict{Name("Nums"): Array{
				Number(0), Dict{Name("P"): String(strings.Repeat("x", pageLabelCacheLimit+1))},
			}}},
		},
	}
	items := d.pageLabelItems()
	if len(items) != 1 || len(items[0].spec.Prefix()) != pageLabelCacheLimit+1 {
		t.Fatalf("page label items = %d", len(items))
	}
	if d.pageLabelsReady || d.pageLabelCache != nil || d.pageLabelCacheBytes != 0 {
		t.Fatalf("oversized page labels entered cache: ready=%v items=%d bytes=%d", d.pageLabelsReady, len(d.pageLabelCache), d.pageLabelCacheBytes)
	}
}

func TestPageLabelsDoNotRetainOversizedValues(t *testing.T) {
	d := &Document{
		trailer: Dict{Name("Root"): Ref{Object: 1}},
		objects: map[Ref]Object{
			{Object: 1}: Dict{Name("Pages"): Ref{Object: 2}, Name("PageLabels"): Dict{Name("Nums"): Array{
				Number(0), Dict{Name("P"): String(strings.Repeat("x", pageLabelsValuesLimit+1))},
			}}},
			{Object: 2}: Dict{Name("Type"): Name("Pages"), Name("Kids"): Array{Ref{Object: 3}}},
			{Object: 3}: Dict{Name("Type"): Name("Page")},
		},
	}
	values, err := d.PageLabels()
	if err != nil {
		t.Fatal(err)
	}
	if len(values) != 1 || len(values[0]) != pageLabelsValuesLimit+1 {
		t.Fatalf("page labels = %d, value length = %d", len(values), len(values[0]))
	}
	if d.pageLabelsValuesReady || d.pageLabelsValues != nil || d.pageLabelsValuesBytes != 0 {
		t.Fatalf("oversized page labels entered cache: ready=%v values=%d bytes=%d", d.pageLabelsValuesReady, len(d.pageLabelsValues), d.pageLabelsValuesBytes)
	}
}

func TestPageLabelsReturnsIndependentCachedSlices(t *testing.T) {
	d := &Document{
		trailer: Dict{Name("Root"): Ref{Object: 1}},
		objects: map[Ref]Object{
			{Object: 1}: Dict{Name("Pages"): Ref{Object: 2}},
			{Object: 2}: Dict{Name("Type"): Name("Pages"), Name("Kids"): Array{Ref{Object: 3}}},
			{Object: 3}: Dict{Name("Type"): Name("Page")},
		},
	}
	first, err := d.PageLabels()
	if err != nil {
		t.Fatal(err)
	}
	first[0] = "mutated"
	second, err := d.PageLabels()
	if err != nil {
		t.Fatal(err)
	}
	if len(second) != 1 || second[0] != "1" {
		t.Fatalf("page label cache was exposed: %#v", second)
	}
}

func TestPageLabelsPreservesEmptyCachedValues(t *testing.T) {
	d := &Document{
		pageLabelsValuesReady: true,
		pageLabelsValues:      make([]string, 0),
	}
	values, err := d.PageLabels()
	if err != nil || values == nil {
		t.Fatalf("cached empty page labels = %#v, err=%v", values, err)
	}
}

func TestPageLabelsSequenceDefersNumberTreeCache(t *testing.T) {
	d := &Document{
		trailer: Dict{Name("Root"): Ref{Object: 1}},
		objects: map[Ref]Object{
			{Object: 1}: Dict{Name("Pages"): Ref{Object: 2}, Name("PageLabels"): Ref{Object: 5}},
			{Object: 2}: Dict{Name("Type"): Name("Pages"), Name("Kids"): Array{Ref{Object: 3}, Ref{Object: 4}}},
			{Object: 3}: Dict{Name("Type"): Name("Page"), Name("Parent"): Ref{Object: 2}},
			{Object: 4}: Dict{Name("Type"): Name("Page"), Name("Parent"): Ref{Object: 2}},
			{Object: 5}: Dict{Name("Nums"): Array{Number(0), Dict{Name("P"): String("front ")}, Number(1), Dict{Name("P"): String("body ")}}},
		},
	}
	for label, err := range d.PageLabelsSeq() {
		if err != nil || label != "front " {
			t.Fatalf("first label=%q err=%v", label, err)
		}
		break
	}
	if d.pageLabelsReady {
		t.Fatal("lazy label sequence populated materialized cache")
	}
}
