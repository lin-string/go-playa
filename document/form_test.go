package document

import (
	"encoding/json"
	"math"
	"strings"
	"sync"
	"testing"

	"github.com/lin-string/go-playa/cacheconfig"
	"github.com/lin-string/go-playa/documentdata"
)

func newTestFormField(spec documentdata.FormFieldSpec) FormField {
	return FormField{data: documentdata.NewFormField(spec)}
}

func TestFormFieldsBuildsInheritedFieldTreeAndWidgets(t *testing.T) {
	d := &Document{
		trailer: Dict{Name("Root"): Ref{Object: 1, Generation: 0}},
		objects: map[Ref]Object{
			{Object: 1, Generation: 0}: Dict{Name("AcroForm"): Ref{Object: 2, Generation: 0}},
			{Object: 2, Generation: 0}: Dict{Name("Fields"): Array{Ref{Object: 3, Generation: 0}}, Name("NeedAppearances"): Bool(true)},
			{Object: 3, Generation: 0}: Dict{Name("T"): String([]byte("person")), Name("FT"): Name("Tx"), Name("Ff"): Number(4096), Name("Kids"): Array{Ref{Object: 4, Generation: 0}}},
			{Object: 4, Generation: 0}: Dict{Name("Subtype"): Name("Widget"), Name("T"): String([]byte("name")), Name("Rect"): Array{Number(10), Number(20), Number(110), Number(40)}, Name("V"): String([]byte("Alice"))},
		},
	}
	forms, err := d.CollectFormFields()
	if err != nil {
		t.Fatal(err)
	}
	if len(forms) != 1 {
		t.Fatalf("form roots = %#v", forms)
	}
	root := forms[0]
	if root.Name() != "person" || root.FullName() != "person" || root.FieldType() != "Tx" || root.Flags() != 4096 || len(root.kids) != 1 {
		t.Fatalf("root field = %#v", root)
	}
	widget := root.kids[0]
	if !widget.IsWidget() || widget.FullName() != "person.name" || widget.Value() != "Alice" || widget.Rect() != [4]float64{10, 20, 110, 40} {
		t.Fatalf("widget = %#v", widget)
	}
}

func TestFormFieldsFollowMultiLevelIndirectFieldChains(t *testing.T) {
	d := &Document{trailer: Dict{Name("Root"): Ref{Object: 1}}, objects: map[Ref]Object{}}
	d.objects[Ref{Object: 1}] = Ref{Object: 2}
	d.objects[Ref{Object: 2}] = Dict{Name("AcroForm"): Ref{Object: 3}}
	d.objects[Ref{Object: 3}] = Dict{Name("Fields"): Ref{Object: 4}}
	d.objects[Ref{Object: 4}] = Array{Ref{Object: 5}}
	d.objects[Ref{Object: 5}] = Ref{Object: 6}
	d.objects[Ref{Object: 6}] = Dict{
		Name("T"):    Ref{Object: 7},
		Name("FT"):   Ref{Object: 8},
		Name("Kids"): Ref{Object: 9},
	}
	d.objects[Ref{Object: 7}] = String("person")
	d.objects[Ref{Object: 8}] = Name("Tx")
	d.objects[Ref{Object: 9}] = Array{Ref{Object: 10}}
	d.objects[Ref{Object: 10}] = Dict{Name("Subtype"): Ref{Object: 11}, Name("T"): String("name"), Name("V"): Ref{Object: 12}}
	d.objects[Ref{Object: 11}] = Name("Widget")
	d.objects[Ref{Object: 12}] = String("Alice")

	fields, err := d.CollectFormFields()
	if err != nil {
		t.Fatal(err)
	}
	if len(fields) != 1 || fields[0].Name() != "person" || len(fields[0].kids) != 1 || fields[0].kids[0].Value() != "Alice" {
		t.Fatalf("form fields = %#v", fields)
	}
}

func TestFormFieldIteratorsFollowMultiLevelIndirectParentAndKids(t *testing.T) {
	d := &Document{trailer: Dict{Name("Root"): Ref{Object: 1}}, objects: map[Ref]Object{}}
	d.objects[Ref{Object: 1}] = Dict{Name("AcroForm"): Ref{Object: 2}}
	d.objects[Ref{Object: 2}] = Dict{Name("Fields"): Array{Ref{Object: 3}}}
	d.objects[Ref{Object: 3}] = Dict{Name("T"): String("root"), Name("Kids"): Ref{Object: 4}}
	d.objects[Ref{Object: 4}] = Array{Ref{Object: 5}}
	d.objects[Ref{Object: 5}] = Ref{Object: 6}
	d.objects[Ref{Object: 6}] = Dict{Name("T"): String("child"), Name("Parent"): Ref{Object: 7}}
	d.objects[Ref{Object: 7}] = Ref{Object: 3}

	root, err := firstFormField(d)
	if err != nil {
		t.Fatal(err)
	}
	var child FormField
	for value, childErr := range root.KidsSeq() {
		if childErr != nil {
			t.Fatal(childErr)
		}
		child = value
		break
	}
	if child.Name() != "child" {
		t.Fatalf("child = %#v", child)
	}
	parent, err := child.ParentFieldWithError(d)
	if err != nil || parent == nil || parent.Name() != "root" {
		t.Fatalf("parent = %#v, err = %v", parent, err)
	}
}

func TestFormFieldFollowsMultiLevelIndirectPageAndParentRefs(t *testing.T) {
	d := &Document{trailer: Dict{Name("Root"): Ref{Object: 1}}, objects: map[Ref]Object{
		{Object: 1}: Dict{Name("AcroForm"): Ref{Object: 2}, Name("Pages"): Ref{Object: 8}},
		{Object: 2}: Dict{Name("Fields"): Array{Ref{Object: 3}}},
		{Object: 3}: Dict{Name("T"): String("root"), Name("Kids"): Array{Ref{Object: 4}}},
		{Object: 4}: Dict{Name("Subtype"): Name("Widget"), Name("T"): String("child"), Name("Parent"): Ref{Object: 5}, Name("P"): Ref{Object: 7}},
		{Object: 5}: Ref{Object: 6},
		{Object: 6}: Ref{Object: 3},
		{Object: 7}: Ref{Object: 9},
		{Object: 8}: Dict{Name("Type"): Name("Pages"), Name("Kids"): Array{Ref{Object: 9}}},
		{Object: 9}: Dict{Name("Type"): Name("Page")},
	}}
	fields, err := d.CollectFormFields()
	if err != nil {
		t.Fatal(err)
	}
	if len(fields) != 1 || len(fields[0].kids) != 1 {
		t.Fatalf("form tree = %#v", fields)
	}
	child := fields[0].kids[0]
	if !child.HasParent() || child.Parent() != (Ref{Object: 3}) || !child.HasPage() || child.Page() != (Ref{Object: 9}) {
		t.Fatalf("indirect form relations = %#v", child)
	}
}

func TestFormFieldRejectsMalformedRect(t *testing.T) {
	d := &Document{
		trailer: Dict{Name("Root"): Ref{Object: 1}},
		objects: map[Ref]Object{
			{Object: 1}: Dict{Name("AcroForm"): Ref{Object: 2}},
			{Object: 2}: Dict{Name("Fields"): Array{Ref{Object: 3}}},
			{Object: 3}: Dict{
				Name("Subtype"): Name("Widget"),
				Name("Rect"):    Array{Number(0), Number(0), Number(10), Number(10), Number(20)},
			},
		},
	}
	fields, err := d.CollectFormFields()
	if err != nil {
		t.Fatal(err)
	}
	if len(fields) != 1 || fields[0].HasRect() {
		t.Fatalf("malformed form rectangle was accepted: %#v", fields)
	}
}

func TestFormFieldFollowsMultiLevelIndirectRectangleValues(t *testing.T) {
	d := &Document{trailer: Dict{Name("Root"): Ref{Object: 1}}, objects: map[Ref]Object{
		{Object: 1}: Dict{Name("AcroForm"): Ref{Object: 2}},
		{Object: 2}: Dict{Name("Fields"): Array{Ref{Object: 3}}},
		{Object: 3}: Dict{Name("Subtype"): Name("Widget"), Name("Rect"): Ref{Object: 4}},
		{Object: 4}: Ref{Object: 5},
		{Object: 5}: Array{Ref{Object: 6}, Ref{Object: 7}, Ref{Object: 8}, Ref{Object: 9}},
		{Object: 6}: Number(10), {Object: 7}: Number(20),
		{Object: 8}: Ref{Object: 10}, {Object: 9}: Number(40),
		{Object: 10}: Number(110),
	}}
	fields, err := d.CollectFormFields()
	if err != nil || len(fields) != 1 || !fields[0].HasRect() || fields[0].Rect() != [4]float64{10, 20, 110, 40} {
		t.Fatalf("indirect form rectangle = %#v, err=%v", fields, err)
	}
}

func TestFormFieldRejectsNonFiniteRect(t *testing.T) {
	for _, value := range []Number{Number(math.NaN()), Number(math.Inf(1))} {
		d := &Document{
			trailer: Dict{Name("Root"): Ref{Object: 1}},
			objects: map[Ref]Object{
				{Object: 1}: Dict{Name("AcroForm"): Ref{Object: 2}},
				{Object: 2}: Dict{Name("Fields"): Array{Ref{Object: 3}}},
				{Object: 3}: Dict{
					Name("Subtype"): Name("Widget"),
					Name("Rect"):    Array{value, Number(0), Number(10), Number(10)},
				},
			},
		}
		fields, err := d.CollectFormFields()
		if err != nil {
			t.Fatal(err)
		}
		if len(fields) != 1 || fields[0].HasRect() {
			t.Fatalf("non-finite form rectangle %v was accepted: %#v", value, fields)
		}
	}
}

func TestConcurrentFormFieldResolution(t *testing.T) {
	d := &Document{
		trailer: Dict{Name("Root"): Ref{Object: 1}},
		objects: map[Ref]Object{
			{Object: 1}: Dict{Name("AcroForm"): Ref{Object: 2}},
			{Object: 2}: Dict{Name("Fields"): Array{Ref{Object: 3}}},
			{Object: 3}: Dict{Name("T"): String("person"), Name("FT"): Name("Tx")},
		},
	}
	const readers = 16
	var wg sync.WaitGroup
	errs := make(chan error, readers)
	start := make(chan struct{})
	for i := 0; i < readers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			fields, err := d.CollectFormFields()
			if err != nil {
				errs <- err
				return
			}
			if len(fields) != 1 || fields[0].Name() != "person" {
				errs <- errNilDocument
			}
		}()
	}
	close(start)
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatal(err)
	}
}

func TestFormFieldResolvesParent(t *testing.T) {
	d := &Document{
		trailer: Dict{Name("Root"): Ref{Object: 1}},
		objects: map[Ref]Object{
			{Object: 1}: Dict{Name("AcroForm"): Ref{Object: 2}},
			{Object: 2}: Dict{Name("Fields"): Array{Ref{Object: 3}}},
			{Object: 3}: Dict{Name("T"): String([]byte("person")), Name("Kids"): Array{Ref{Object: 4}}},
			{Object: 4}: Dict{Name("Parent"): Ref{Object: 3}, Name("T"): String([]byte("name")), Name("Subtype"): Name("Widget")},
		},
	}
	fields, err := d.CollectFormFields()
	if err != nil || len(fields) != 1 {
		t.Fatalf("fields = %#v, err = %v", fields, err)
	}
	kids := fields[0].kids
	if len(kids) != 1 || !kids[0].HasParent() || kids[0].Parent() != (Ref{Object: 3}) {
		t.Fatalf("kids = %#v", kids)
	}
	parent := kids[0].ParentField(d)
	if parent == nil || parent.Name() != "person" {
		t.Fatalf("parent = %#v", parent)
	}
}

func TestFormFieldParentWithErrorReportsMalformedReference(t *testing.T) {
	d := &Document{objects: map[Ref]Object{{Object: 9}: String("invalid")}}
	field := newTestFormField(documentdata.FormFieldSpec{Parent: Ref{Object: 9}, HasParent: true})
	if parent, err := field.ParentFieldWithError(d); err == nil || parent != nil {
		t.Fatalf("parent = %#v, err = %v", parent, err)
	}
	if parent := field.ParentField(d); parent != nil {
		t.Fatalf("compatibility parent = %#v", parent)
	}
}

func TestFormFieldParentPreservesAncestorName(t *testing.T) {
	d := &Document{
		trailer: Dict{Name("Root"): Ref{Object: 1}},
		objects: map[Ref]Object{
			{Object: 1}: Dict{Name("AcroForm"): Ref{Object: 2}},
			{Object: 2}: Dict{Name("Fields"): Array{Ref{Object: 3}}},
			{Object: 3}: Dict{Name("T"): String("person"), Name("Kids"): Array{Ref{Object: 4}}},
			{Object: 4}: Dict{Name("T"): String("address"), Name("Kids"): Array{Ref{Object: 5}}, Name("Parent"): Ref{Object: 3}},
			{Object: 5}: Dict{Name("T"): String("city"), Name("Parent"): Ref{Object: 4}},
		},
	}
	fields, err := d.CollectFormFields()
	if err != nil || len(fields) != 1 || len(fields[0].kids) != 1 || len(fields[0].kids[0].kids) != 1 {
		t.Fatalf("nested fields = %#v, err = %v", fields, err)
	}
	parent := fields[0].kids[0].kids[0].ParentField(d)
	if parent == nil || parent.Name() != "address" || parent.FullName() != "person.address" {
		t.Fatalf("nested parent = %#v", parent)
	}
}

func TestFormFieldResolvesWidgetPage(t *testing.T) {
	d := &Document{
		trailer: Dict{Name("Root"): Ref{Object: 1}},
		objects: map[Ref]Object{
			{Object: 1}: Dict{Name("Pages"): Ref{Object: 2}, Name("AcroForm"): Ref{Object: 5}},
			{Object: 2}: Dict{Name("Type"): Name("Pages"), Name("Kids"): Array{Ref{Object: 3}}},
			{Object: 3}: Dict{Name("Type"): Name("Page"), Name("Parent"): Ref{Object: 2}},
			{Object: 5}: Dict{Name("Fields"): Array{Ref{Object: 6}}},
			{Object: 6}: Dict{Name("T"): String("widget"), Name("Subtype"): Name("Widget"), Name("P"): Ref{Object: 3}},
		},
	}
	fields, err := d.CollectFormFields()
	if err != nil || len(fields) != 1 || !fields[0].HasPage() || fields[0].Page() != (Ref{Object: 3}) {
		t.Fatalf("fields = %#v, err = %v", fields, err)
	}
	page, err := fields[0].PageObject(d)
	if err != nil || page.ref != (Ref{Object: 3}) {
		t.Fatalf("field page = %#v, err = %v", page, err)
	}
}

func TestFormFieldsReadsChoiceOptionsAndSelectedIndices(t *testing.T) {
	d := &Document{
		trailer: Dict{Name("Root"): Ref{Object: 1, Generation: 0}},
		objects: map[Ref]Object{
			{Object: 1, Generation: 0}: Dict{Name("AcroForm"): Ref{Object: 2, Generation: 0}},
			{Object: 2, Generation: 0}: Dict{Name("Fields"): Array{Ref{Object: 3, Generation: 0}}},
			{Object: 3, Generation: 0}: Dict{Name("T"): String([]byte("choice")), Name("FT"): Name("Ch"), Name("Opt"): Array{String([]byte("One")), Array{String([]byte("two-export")), String([]byte{0xfe, 0xff, 0x4e, 0x8c})}}, Name("I"): Array{Number(1)}},
		},
	}
	got, err := d.CollectFormFields()
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("choice field = %#v", got)
	}
	options, optionValues, selected := got[0].OptionsCopy(), got[0].OptionValuesCopy(), got[0].SelectedCopy()
	if len(options) != 2 || options[0] != "One" || options[1] != "二" || len(optionValues) != 2 || optionValues[0] != "One" || optionValues[1] != "two-export" || len(selected) != 1 || selected[0] != 1 {
		t.Fatalf("choice field = %#v", got)
	}
}

func TestFormFieldsIgnoreMalformedChoiceOptionPairs(t *testing.T) {
	d := &Document{
		trailer: Dict{Name("Root"): Ref{Object: 1}},
		objects: map[Ref]Object{
			{Object: 1}: Dict{Name("AcroForm"): Ref{Object: 2}},
			{Object: 2}: Dict{Name("Fields"): Array{Ref{Object: 3}}},
			{Object: 3}: Dict{
				Name("T"): Name("choice"), Name("FT"): Name("Ch"),
				Name("Opt"): Array{
					Array{String("export"), String("display"), String("extra")},
					String("valid"),
				},
			},
		},
	}
	fields, err := d.CollectFormFields()
	if err != nil {
		t.Fatal(err)
	}
	if len(fields) != 1 {
		t.Fatalf("malformed choice option pair was accepted: %#v", fields)
	}
	options := fields[0].OptionsCopy()
	if len(options) != 1 || options[0] != "valid" {
		t.Fatalf("malformed choice option pair was accepted: %#v", fields)
	}
}

func TestFormFieldsIgnoreInvalidChoiceSelectionIndices(t *testing.T) {
	d := &Document{
		trailer: Dict{Name("Root"): Ref{Object: 1}},
		objects: map[Ref]Object{
			{Object: 1}: Dict{Name("AcroForm"): Ref{Object: 2}},
			{Object: 2}: Dict{Name("Fields"): Array{Ref{Object: 3}}},
			{Object: 3}: Dict{
				Name("T"): Name("choice"), Name("FT"): Name("Ch"),
				Name("Opt"): Array{String("only")},
				Name("I"):   Array{Number(-1), Number(5), Number(0)},
			},
		},
	}
	fields, err := d.CollectFormFields()
	if err != nil {
		t.Fatal(err)
	}
	if len(fields) != 1 {
		t.Fatalf("invalid choice selection indices were accepted: %#v", fields)
	}
	selected := fields[0].SelectedCopy()
	if len(selected) != 1 || selected[0] != 0 {
		t.Fatalf("invalid choice selection indices were accepted: %#v", fields)
	}
}

func TestFormFieldsPreserveMultiValueChoices(t *testing.T) {
	d := &Document{
		trailer: Dict{Name("Root"): Ref{Object: 1, Generation: 0}},
		objects: map[Ref]Object{
			{Object: 1, Generation: 0}: Dict{Name("AcroForm"): Ref{Object: 2, Generation: 0}},
			{Object: 2, Generation: 0}: Dict{Name("Fields"): Array{Ref{Object: 3, Generation: 0}}},
			{Object: 3, Generation: 0}: Dict{
				Name("T"):  String("languages"),
				Name("FT"): Name("Ch"),
				Name("V"):  Array{String("Go"), String([]byte{0xfe, 0xff, 0x4e, 0x8c})},
				Name("DV"): Array{String("English"), String([]byte{0xfe, 0xff, 0x4e, 0x2d, 0x65, 0x87})},
			},
		},
	}
	got, err := d.CollectFormFields()
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("fields = %#v", got)
	}
	field := got[0]
	values := field.ValuesCopy()
	if field.Value() != "" || len(values) != 2 || values[0] != "Go" || values[1] != "二" {
		t.Fatalf("multi-value field = %#v", field)
	}
	defaultValues := field.DefaultValuesCopy()
	if field.DefaultValue() != "" || len(defaultValues) != 2 || defaultValues[0] != "English" || defaultValues[1] != "中文" {
		t.Fatalf("multi-value defaults = %#v", field)
	}
}

func TestFormFieldsPreserveSingleValueArrayShape(t *testing.T) {
	d := &Document{
		trailer: Dict{Name("Root"): Ref{Object: 1, Generation: 0}},
		objects: map[Ref]Object{
			{Object: 1, Generation: 0}: Dict{Name("AcroForm"): Ref{Object: 2, Generation: 0}},
			{Object: 2, Generation: 0}: Dict{Name("Fields"): Array{Ref{Object: 3, Generation: 0}}},
			{Object: 3, Generation: 0}: Dict{Name("T"): String("choice"), Name("FT"): Name("Ch"), Name("V"): Array{String("one")}, Name("DV"): Array{String("default")}},
		},
	}
	got, err := d.CollectFormFields()
	if err != nil || len(got) != 1 {
		t.Fatalf("fields = %#v, err = %v", got, err)
	}
	field := got[0]
	values, defaultValues := field.ValuesCopy(), field.DefaultValuesCopy()
	if field.Value() != "" || len(values) != 1 || values[0] != "one" || field.DefaultValue() != "" || len(defaultValues) != 1 || defaultValues[0] != "default" {
		t.Fatalf("single-value arrays = %#v", field)
	}
}

func TestFormFieldCacheCopiesNestedKids(t *testing.T) {
	d := &Document{
		trailer: Dict{Name("Root"): Ref{Object: 1}},
		objects: map[Ref]Object{
			{Object: 1}: Dict{Name("AcroForm"): Ref{Object: 2}},
			{Object: 2}: Dict{Name("Fields"): Array{Ref{Object: 3}}},
			{Object: 3}: Dict{Name("T"): String("root"), Name("Kids"): Array{Ref{Object: 4}}},
			{Object: 4}: Dict{Name("T"): String("child")},
		},
	}
	first, err := d.CollectFormFields()
	if err != nil || len(first) != 1 || len(first[0].kids) != 1 {
		t.Fatalf("first form fields = %#v, err = %v", first, err)
	}
	snapshot := first[0].Finalize()
	if snapshot.kids[0].Name() != "child" {
		t.Fatalf("finalized child = %#v", snapshot.kids[0])
	}
	second, err := d.CollectFormFields()
	if err != nil || second[0].kids[0].Name() != "child" {
		t.Fatalf("form cache was exposed = %#v, err = %v", second, err)
	}
}

func TestFormFieldFinalizeDoesNotShareCachedDictionaries(t *testing.T) {
	d := &Document{
		trailer: Dict{Name("Root"): Ref{Object: 1}},
		objects: map[Ref]Object{
			{Object: 1}: Dict{Name("AcroForm"): Ref{Object: 2}},
			{Object: 2}: Dict{Name("Fields"): Array{Ref{Object: 3}}},
		},
	}
	d.formTopCache = map[Ref]FormField{{Object: 3}: newTestFormField(documentdata.FormFieldSpec{
		Name: "field", Dict: Dict{Name("TU"): String("original")},
	})}

	first := []FormField{}
	for field, err := range d.FormFields() {
		if err != nil {
			t.Fatal(err)
		}
		first = append(first, field)
	}
	if len(first) != 1 {
		t.Fatalf("fields = %#v", first)
	}
	snapshot := first[0].Finalize()
	snapshotDict := snapshot.DictCopy()
	snapshotDict[Name("TU")] = String("changed")
	second := []FormField{}
	for field, err := range d.FormFields() {
		if err != nil {
			t.Fatal(err)
		}
		second = append(second, field)
	}
	value, _ := second[0].DictCopy()[Name("TU")].(String)
	if string(value) != "original" {
		t.Fatalf("form dictionary cache was exposed: %#v", second[0].DictCopy())
	}
}

func TestFormPathClonePreservesEmptyAllocatedMap(t *testing.T) {
	source := map[Ref]bool{}
	clone := cloneFormPath(source)
	if clone == nil {
		t.Fatal("empty allocated form path was collapsed to nil")
	}
}

func TestFormFieldSnapshotsPreserveEmptySlices(t *testing.T) {
	field := newTestFormField(documentdata.FormFieldSpec{
		Values: make([]string, 0), DefaultValues: make([]string, 0),
		Options: make([]string, 0), OptionValues: make([]string, 0), Selected: make([]int, 0),
	})
	snapshot := field.Finalize()
	if snapshot.ValuesCopy() == nil || snapshot.DefaultValuesCopy() == nil || snapshot.OptionsCopy() == nil || snapshot.OptionValuesCopy() == nil || snapshot.SelectedCopy() == nil {
		t.Fatalf("empty form slices were not preserved: %#v", snapshot)
	}
	if field.ValuesCopy() == nil || field.DefaultValuesCopy() == nil || field.OptionsCopy() == nil || field.OptionValuesCopy() == nil || field.SelectedCopy() == nil {
		t.Fatal("empty form copies became nil")
	}
}

func TestFormFieldKidsCopyReturnsFinalizedChildren(t *testing.T) {
	d := &Document{}
	field := FormField{kids: []FormField{{
		data:        documentdata.NewFormField(documentdata.FormFieldSpec{Name: "child"}),
		document:    d,
		fieldRef:    Ref{Object: 7},
		hasFieldRef: true,
		kidParent:   Dict{Name("T"): String("parent")},
	}}}

	kids, err := field.KidsCopy()
	if err != nil {
		t.Fatal(err)
	}
	if len(kids) != 1 {
		t.Fatalf("kids = %#v", kids)
	}
	child := kids[0]
	if child.document != nil || child.hasFieldRef || child.kidParent != nil || child.kidsReady != true {
		t.Fatalf("KidsCopy returned borrowed child: %#v", child)
	}
}

func TestFormFieldKidsCopyReportsMalformedChild(t *testing.T) {
	field := FormField{document: &Document{}, kidObjects: []Object{Number(1)}}
	if kids, err := field.KidsCopy(); err == nil || kids != nil {
		t.Fatalf("malformed form children = %#v, err = %v", kids, err)
	}
	if _, err := field.MarshalJSON(); err == nil {
		t.Fatal("malformed form JSON produced no error")
	}
	if snapshot, err := field.FinalizeWithError(); err == nil || snapshot.Name() != "" {
		t.Fatalf("malformed form finalize = %#v, err = %v", snapshot, err)
	}
}

func TestFormFieldLazyKidsAreReusedAcrossValueCopies(t *testing.T) {
	d := &Document{objects: map[Ref]Object{{Object: 1, Generation: 0}: Dict{Name("T"): String("child")}}}
	field := FormField{document: d, kidObjects: []Object{Ref{Object: 1, Generation: 0}}, lazyState: &formFieldLazyState{}}
	kids, err := field.KidsCopy()
	if err != nil || len(kids) != 1 || kids[0].Name() != "child" {
		t.Fatalf("first lazy kids = %#v, err=%v", kids, err)
	}
	d.objects[Ref{Object: 1, Generation: 0}] = Number(1)
	kids, err = field.KidsCopy()
	if err != nil || len(kids) != 1 || kids[0].Name() != "child" {
		t.Fatalf("cached lazy kids = %#v, err=%v", kids, err)
	}
}

func TestFormFieldLazyKidsCacheTerminalErrors(t *testing.T) {
	d := &Document{objects: map[Ref]Object{{Object: 1, Generation: 0}: Number(1)}}
	field := FormField{document: d, kidObjects: []Object{Ref{Object: 1, Generation: 0}}, lazyState: &formFieldLazyState{}}
	if kids, err := field.KidsCopy(); err == nil || kids != nil {
		t.Fatalf("first malformed kids = %#v, err=%v", kids, err)
	}
	d.objects[Ref{Object: 1, Generation: 0}] = Dict{Name("T"): String("repaired")}
	if kids, err := field.KidsCopy(); err == nil || kids != nil {
		t.Fatalf("cached malformed kids = %#v, err=%v", kids, err)
	}
}

func TestFormFieldFinalizeWithErrorPropagatesNestedChildFailure(t *testing.T) {
	child := FormField{document: &Document{}, kidObjects: []Object{Number(1)}}
	parent := FormField{kids: []FormField{child}, kidsReady: true}
	if snapshot, err := parent.FinalizeWithError(); err == nil || snapshot.Name() != "" {
		t.Fatalf("nested form child finalize = %#v, err=%v", snapshot, err)
	}
}

func TestFormFieldJSONIncludesMaterializedKids(t *testing.T) {
	field := FormField{
		data: documentdata.NewFormField(documentdata.FormFieldSpec{
			Name: "field", FullName: "root.field", Page: Ref{Object: 7}, HasPage: true,
			FieldType: "Tx", Flags: 4096, HasFlags: true, Value: "value",
			DefaultValue: "default", DefaultAppearance: "/F1 10 Tf", Rect: [4]float64{1, 2, 3, 4},
			HasRect: true, IsWidget: true,
		}),
		kids: []FormField{{data: documentdata.NewFormField(documentdata.FormFieldSpec{Name: "child"})}},
	}
	data, err := json.Marshal(field)
	if err != nil {
		t.Fatal(err)
	}
	got := string(data)
	for _, want := range []string{`"Name":"field"`, `"FullName":"root.field"`, `"FieldType":"Tx"`, `"Value":"value"`, `"HasPage":true`, `"HasRect":true`, `"IsWidget":true`, `"Name":"child"`} {
		if !strings.Contains(got, want) {
			t.Fatalf("form JSON omitted %s: %s", want, got)
		}
	}
	if !strings.Contains(got, `"Rect":[1,2,3,4]`) {
		t.Fatalf("form JSON omitted widget rectangle: %s", got)
	}
}

func TestFormFieldFinalizeDoesNotShareKidObjects(t *testing.T) {
	field := FormField{kidObjects: []Object{
		Dict{Name("Meta"): Array{Dict{Name("Value"): String("original")}}},
	}}

	copy := cloneFormField(field)
	cloned, ok := copy.kidObjects[0].(Dict)
	if !ok {
		t.Fatalf("cloned kid object = %#v", copy.kidObjects[0])
	}
	meta, ok := cloned[Name("Meta")].(Array)
	if !ok || len(meta) != 1 {
		t.Fatalf("cloned kid metadata = %#v", cloned[Name("Meta")])
	}
	entry, ok := meta[0].(Dict)
	if !ok {
		t.Fatalf("cloned kid metadata entry = %#v", meta[0])
	}
	entry[Name("Value")] = String("changed")
	original := field.kidObjects[0].(Dict)[Name("Meta")].(Array)[0].(Dict)
	value := original[Name("Value")].(String)
	if string(value) != "original" {
		t.Fatalf("form kid object copy shares nested values: %q", value)
	}
}

func TestFormFieldLazyKidsBorrowDocumentArray(t *testing.T) {
	kids := Array{Ref{Object: 2}, Ref{Object: 3}}
	d := &Document{}
	field, ok := d.formField(Dict{Name("Kids"): kids}, nil, "", map[Ref]bool{})
	if !ok || len(field.kidObjects) != len(kids) {
		t.Fatalf("form field = %#v, ok=%v", field, ok)
	}
	if &field.kidObjects[0] != &kids[0] {
		t.Fatal("lazy form kids copied the document-owned array")
	}
}

func TestFormFieldCacheEvictionClampsInconsistentByteAccounting(t *testing.T) {
	ref := Ref{Object: 7}
	d := &Document{
		formTopCache:           map[Ref]FormField{ref: newTestFormField(documentdata.FormFieldSpec{Name: "cached"})},
		formTopCacheSizes:      map[Ref]int{ref: 128},
		formTopCacheBytes:      64,
		cacheOptions:           cacheconfig.Options{FormFieldBytes: 128},
		cacheOptionsConfigured: true,
	}
	d.updateFormFieldCache(ref, FormField{})
	if d.formTopCacheBytes != 0 {
		t.Fatalf("form cache bytes after inconsistent eviction = %d, want 0", d.formTopCacheBytes)
	}
}

func TestFormFieldsSequenceIsOrderedRepeatableAndStopsEarly(t *testing.T) {
	d := &Document{
		trailer: Dict{Name("Root"): Ref{Object: 1, Generation: 0}},
		objects: map[Ref]Object{
			{Object: 1, Generation: 0}: Dict{Name("AcroForm"): Ref{Object: 2, Generation: 0}},
			{Object: 2, Generation: 0}: Dict{Name("Fields"): Array{Ref{Object: 3, Generation: 0}, Ref{Object: 4, Generation: 0}}},
			{Object: 3, Generation: 0}: Dict{Name("T"): String([]byte("first"))},
			{Object: 4, Generation: 0}: Dict{Name("T"): String([]byte("second"))},
		},
	}

	first := []string{}
	for field, err := range d.FormFields() {
		if err != nil {
			t.Fatal(err)
		}
		first = append(first, field.Name())
		break
	}
	if len(first) != 1 || first[0] != "first" {
		t.Fatalf("early fields = %v, want [first]", first)
	}

	second, err := d.CollectFormFields()
	if err != nil {
		t.Fatal(err)
	}
	if len(second) != 2 || second[0].Name() != "first" || second[1].Name() != "second" {
		t.Fatalf("second traversal = %#v", second)
	}
	if len(d.formTopCache) != 2 {
		t.Fatalf("top-level form cache size = %d, want 2", len(d.formTopCache))
	}
}

func TestFormFieldsReuseCacheAcrossIndirectReferenceChains(t *testing.T) {
	d := &Document{trailer: Dict{Name("Root"): Ref{Object: 1}}, objects: map[Ref]Object{
		{Object: 1}: Dict{Name("AcroForm"): Ref{Object: 2}},
		{Object: 2}: Dict{Name("Fields"): Array{Ref{Object: 3}, Ref{Object: 4}}},
		{Object: 3}: Ref{Object: 4},
		{Object: 4}: Dict{Name("T"): String("shared")},
	}}
	fields, err := d.CollectFormFields()
	if err != nil || len(fields) != 2 || fields[0].Name() != "shared" || fields[1].Name() != "shared" {
		t.Fatalf("form fields = %#v, err = %v", fields, err)
	}
	if len(d.formTopCache) != 1 {
		t.Fatalf("top-level form cache size = %d, want 1", len(d.formTopCache))
	}
}

func TestFormFieldDoesNotRetainOversizedCacheValues(t *testing.T) {
	d := &Document{
		trailer: Dict{Name("Root"): Ref{Object: 1, Generation: 0}},
		objects: map[Ref]Object{
			{Object: 1, Generation: 0}: Dict{Name("AcroForm"): Ref{Object: 2, Generation: 0}},
			{Object: 2, Generation: 0}: Dict{Name("Fields"): Array{Ref{Object: 3, Generation: 0}}},
			{Object: 3, Generation: 0}: Dict{Name("T"): String(strings.Repeat("x", formTopCacheLimit+1))},
		},
	}
	fields, err := d.CollectFormFields()
	if err != nil || len(fields) != 1 || len(fields[0].Name()) != formTopCacheLimit+1 {
		t.Fatalf("form fields = %d, err = %v", len(fields), err)
	}
	if len(d.formTopCache) != 0 || d.formTopCacheBytes != 0 {
		t.Fatalf("oversized form field entered cache: entries=%d bytes=%d", len(d.formTopCache), d.formTopCacheBytes)
	}
}

func TestFormFieldKidsAreLazyUntilKidsSequenceIsConsumed(t *testing.T) {
	d := &Document{
		trailer: Dict{Name("Root"): Ref{Object: 1, Generation: 0}},
		objects: map[Ref]Object{
			{Object: 1, Generation: 0}: Dict{Name("AcroForm"): Ref{Object: 2, Generation: 0}},
			{Object: 2, Generation: 0}: Dict{Name("Fields"): Array{Ref{Object: 3, Generation: 0}}},
			{Object: 3, Generation: 0}: Dict{Name("T"): String("root"), Name("Kids"): Array{Ref{Object: 4, Generation: 0}}},
			{Object: 4, Generation: 0}: Dict{Name("T"): String("child")},
		},
	}
	field, err := firstFormField(d)
	if err != nil {
		t.Fatal(err)
	}
	if len(field.kids) != 0 {
		t.Fatalf("kids were eagerly materialized: %#v", field.kids)
	}
	var names []string
	for child, err := range field.KidsSeq() {
		if err != nil {
			t.Fatal(err)
		}
		names = append(names, child.Name())
	}
	if len(names) != 1 || names[0] != "child" {
		t.Fatalf("lazy kids = %#v", names)
	}
	field.materializeKids()
	field.materializeKids()
	if len(field.kids) != 1 || field.kids[0].Name() != "child" {
		t.Fatalf("repeated materialization = %#v", field.kids)
	}
	delete(d.objects, Ref{Object: 4, Generation: 0})
	var cached []string
	for child, err := range field.KidsSeq() {
		if err != nil {
			t.Fatal(err)
		}
		cached = append(cached, child.Name())
	}
	if len(cached) != 1 || cached[0] != "child" {
		t.Fatalf("cached kids = %#v", cached)
	}
	fields, err := d.CollectFormFields()
	if err != nil || len(fields) != 1 || len(fields[0].kids) != 1 {
		t.Fatalf("cached top-level field = %#v, err = %v", fields, err)
	}
}

func TestFormFieldKidsStopAtAncestorCycle(t *testing.T) {
	d := &Document{
		trailer: Dict{Name("Root"): Ref{Object: 1}},
		objects: map[Ref]Object{
			{Object: 1}: Dict{Name("AcroForm"): Ref{Object: 2}},
			{Object: 2}: Dict{Name("Fields"): Array{Ref{Object: 3}}},
			{Object: 3}: Dict{Name("T"): String("root"), Name("Kids"): Array{Ref{Object: 4}}},
			{Object: 4}: Dict{Name("T"): String("child"), Name("Kids"): Array{Ref{Object: 3}}},
		},
	}

	field, err := firstFormField(d)
	if err != nil {
		t.Fatal(err)
	}
	var child FormField
	count := 0
	for value, childErr := range field.KidsSeq() {
		if childErr != nil {
			t.Fatal(childErr)
		}
		child = value
		count++
	}
	if count != 1 || child.Name() != "child" {
		t.Fatalf("first-level kids = %d, %#v", count, child)
	}
	grandchildren := 0
	for _, childErr := range child.KidsSeq() {
		if childErr != nil {
			t.Fatal(childErr)
		}
		grandchildren++
	}
	if grandchildren != 0 {
		t.Fatalf("ancestor cycle was not stopped: %d grandchildren", grandchildren)
	}
}

func TestFormFieldKidsStopAtIndirectAncestorCycle(t *testing.T) {
	d := &Document{trailer: Dict{Name("Root"): Ref{Object: 1}}, objects: map[Ref]Object{
		{Object: 1}: Dict{Name("AcroForm"): Ref{Object: 2}},
		{Object: 2}: Dict{Name("Fields"): Array{Ref{Object: 3}}},
		{Object: 3}: Dict{Name("T"): String("root"), Name("Kids"): Array{Ref{Object: 4}}},
		{Object: 4}: Dict{Name("T"): String("child"), Name("Kids"): Array{Ref{Object: 5}}},
		{Object: 5}: Ref{Object: 6},
		{Object: 6}: Ref{Object: 3},
	}}
	field, err := firstFormField(d)
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, childErr := range field.KidsSeq() {
		if childErr != nil {
			t.Fatal(childErr)
		}
		count++
	}
	if count != 1 {
		t.Fatalf("indirect cyclic kids = %d, want 1", count)
	}
}

func TestFormFieldMaterializationStopsCircularReferences(t *testing.T) {
	d := &Document{
		trailer: Dict{Name("Root"): Ref{Object: 1}},
		objects: map[Ref]Object{
			{Object: 1}: Dict{Name("AcroForm"): Ref{Object: 2}},
			{Object: 2}: Dict{Name("Fields"): Array{Ref{Object: 3}}},
			{Object: 3}: Dict{Name("T"): String("A"), Name("Kids"): Array{Ref{Object: 4}}},
			{Object: 4}: Dict{Name("T"): String("B"), Name("Kids"): Array{Ref{Object: 3}}},
		},
	}
	fields, err := d.CollectFormFields()
	if err != nil || len(fields) != 1 || len(fields[0].kids) != 1 || len(fields[0].kids[0].kids) != 0 {
		t.Fatalf("circular form tree = %#v, err = %v", fields, err)
	}
}

func TestFormFieldsReportMalformedEntries(t *testing.T) {
	d := &Document{
		trailer: Dict{Name("Root"): Ref{Object: 1, Generation: 0}},
		objects: map[Ref]Object{
			{Object: 1, Generation: 0}: Dict{Name("AcroForm"): Ref{Object: 2, Generation: 0}},
			{Object: 2, Generation: 0}: Dict{Name("Fields"): Array{Number(1)}},
		},
	}
	for _, err := range d.FormFields() {
		if err == nil {
			t.Fatal("malformed form field produced a value")
		}
		if !strings.Contains(err.Error(), "form field is not a dictionary") {
			t.Fatalf("error = %v", err)
		}
		return
	}
	t.Fatal("malformed form field produced no error")
}

func TestFormFieldsReportMalformedCatalogRoot(t *testing.T) {
	d := &Document{trailer: Dict{Name("Root"): Number(1)}}
	for _, err := range d.FormFields() {
		if err == nil {
			t.Fatal("malformed catalog root produced a form field")
		}
		return
	}
	t.Fatal("malformed catalog root produced no error")
}

func TestFormFieldsCachesTerminalRootErrors(t *testing.T) {
	d := &Document{trailer: Dict{Name("Root"): Number(1)}}
	var firstErr, secondErr error
	for _, err := range d.FormFields() {
		firstErr = err
		break
	}
	for _, err := range d.FormFields() {
		secondErr = err
		break
	}
	if firstErr == nil || secondErr == nil {
		t.Fatalf("FormFields errors = %v, %v; want both non-nil", firstErr, secondErr)
	}
	if firstErr != secondErr {
		t.Fatalf("FormFields error was not cached: first=%p second=%p", firstErr, secondErr)
	}
	if d.formFieldsErr != firstErr || !d.formFieldsErrReady {
		t.Fatalf("cached FormFields state = ready=%v err=%v", d.formFieldsErrReady, d.formFieldsErr)
	}
}

func TestFormFieldsReportMalformedAcroFormRoots(t *testing.T) {
	tests := []struct {
		name string
		root Object
	}{
		{name: "acroform", root: Dict{Name("AcroForm"): Number(1)}},
		{name: "fields", root: Dict{Name("AcroForm"): Dict{Name("Fields"): Number(1)}}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			d := &Document{trailer: Dict{Name("Root"): test.root}}
			for _, err := range d.FormFields() {
				if err == nil {
					t.Fatal("malformed AcroForm root produced a field")
				}
				return
			}
			t.Fatal("malformed AcroForm root produced no error")
		})
	}
}

func TestFormFieldsReportUnresolvedFieldsReference(t *testing.T) {
	d := &Document{trailer: Dict{Name("Root"): Dict{
		Name("AcroForm"): Dict{Name("Fields"): Ref{Object: 99}},
	}}}
	for _, err := range d.FormFields() {
		if err == nil || err.Error() != "playa: AcroForm Fields could not be resolved" {
			t.Fatalf("unresolved Fields error = %v", err)
		}
		return
	}
	t.Fatal("unresolved Fields reference produced no error")
}

func TestFormFieldKidsReportMalformedEntries(t *testing.T) {
	field := FormField{document: &Document{}, kidObjects: []Object{Number(1)}}
	for _, err := range field.KidsSeq() {
		if err == nil {
			t.Fatal("malformed form child produced a value")
		}
		if !strings.Contains(err.Error(), "form field child is not a dictionary") {
			t.Fatalf("error = %v", err)
		}
		return
	}
	t.Fatal("malformed form child produced no error")
}

func TestFormFieldKidsReportUnresolvedEntries(t *testing.T) {
	d := &Document{objects: map[Ref]Object{}}
	field := FormField{document: d, kidObjects: []Object{Ref{Object: 99}}}
	for _, err := range field.KidsSeq() {
		if err == nil || err.Error() != "playa: form field child could not be resolved" {
			t.Fatalf("unresolved form child error = %v", err)
		}
		return
	}
	t.Fatal("unresolved form child reference produced no error")
}

func TestFormFieldParentReportsUnresolvedReference(t *testing.T) {
	d := &Document{objects: map[Ref]Object{}}
	field := newTestFormField(documentdata.FormFieldSpec{Parent: Ref{Object: 99}, HasParent: true})
	if parent, err := field.ParentFieldWithError(d); err == nil || parent != nil || err.Error() != "playa: form field parent could not be resolved" {
		t.Fatalf("unresolved form parent = %#v, err=%v", parent, err)
	}
}

func TestCollectFormFieldsReportsMalformedNestedChild(t *testing.T) {
	d := &Document{
		trailer: Dict{Name("Root"): Ref{Object: 1}},
		objects: map[Ref]Object{
			{Object: 1}: Dict{Name("AcroForm"): Ref{Object: 2}},
			{Object: 2}: Dict{Name("Fields"): Array{Ref{Object: 3}}},
			{Object: 3}: Dict{Name("T"): String("root"), Name("Kids"): Array{Number(1)}},
		},
	}
	if got, err := d.CollectFormFields(); err == nil || got != nil {
		t.Fatalf("nested malformed form field = %#v, err = %v", got, err)
	}
}

func TestFormKidsSequenceUsesCompletedCanonicalCache(t *testing.T) {
	d := &Document{
		trailer: Dict{Name("Root"): Ref{Object: 1, Generation: 0}},
		objects: map[Ref]Object{
			{Object: 1, Generation: 0}: Dict{Name("AcroForm"): Ref{Object: 2, Generation: 0}},
			{Object: 2, Generation: 0}: Dict{Name("Fields"): Array{Ref{Object: 3, Generation: 0}}},
			{Object: 3, Generation: 0}: Dict{Name("T"): String("root"), Name("Kids"): Array{Ref{Object: 4, Generation: 0}}},
			{Object: 4, Generation: 0}: Dict{Name("T"): String("child")},
		},
	}
	field, err := firstFormField(d)
	if err != nil {
		t.Fatal(err)
	}
	materialized := field
	materialized.materializeKids()
	if !materialized.kidsReady {
		t.Fatal("form kids were not materialized")
	}
	field.kids = nil
	field.kidsReady = false
	delete(d.objects, Ref{Object: 4, Generation: 0})
	var names []string
	for child, err := range field.KidsSeq() {
		if err != nil {
			t.Fatal(err)
		}
		names = append(names, child.Name())
	}
	if len(names) != 1 || names[0] != "child" {
		t.Fatalf("canonical form kids = %v", names)
	}
}

func firstFormField(d *Document) (FormField, error) {
	for field, err := range d.FormFields() {
		return field, err
	}
	return FormField{}, nil
}
