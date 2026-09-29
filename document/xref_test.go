package document

import (
	"bytes"
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"testing"
	"unsafe"

	"github.com/lin-string/go-playa/internal/testfixture"
)

func TestXRefEntryStaysCompact(t *testing.T) {
	if got := unsafe.Sizeof(xrefEntry{}); got > 24 {
		t.Fatalf("xrefEntry size = %d bytes, want at most 24", got)
	}
}

func TestXRefsExposeReadOnlyRevisionTables(t *testing.T) {
	d, err := Open(testfixture.Path(t, "form_simple.pdf"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = d.Close() }()
	tables, err := d.XRefs()
	if err != nil {
		t.Fatal(err)
	}
	if len(tables) != 1 || tables[0].Kind() != "table" {
		t.Fatalf("xref tables = %#v, want one table", tables)
	}
	entries := tables[0].EntriesCopy()
	if len(entries) != 7 || entries[0].Object() != 1 {
		t.Fatalf("xref entries = %#v", entries)
	}
	entries[0] = XRefEntry{}
	again, err := d.XRefs()
	if err != nil {
		t.Fatal(err)
	}
	if again[0].EntriesCopy()[0].Object() != 1 {
		t.Fatalf("xref snapshot was mutable: %#v", again[0].EntriesCopy())
	}
}

func TestXRefStreamSnapshotMatchesPlayaRangeIteration(t *testing.T) {
	// Playa's XRefStream iterator resets its byte offset for every /Index
	// range. Preserve that observable iteration result in the public revision
	// snapshot while keeping the parser's complete index for object lookup.
	raw := []byte{1, 0, 0, 0, 0, 1, 1, 0, 0, 1, 0, 0, 1, 0, 0}
	data := []byte(fmt.Sprintf("4 0 obj\n<< /Type /XRef /Size 23 /W [1 1 1] /Index [10 2 20 3] /Length %d >>\nstream\n", len(raw)))
	data = append(data, raw...)
	data = append(data, []byte("\nendstream\nendobj\n")...)
	d := &Document{data: data}
	entries, _, err := d.parseXRefStream(0)
	if err != nil {
		t.Fatal(err)
	}
	got := snapshotXRefEntries(entries)
	objects := make([]int, 0, len(got))
	for _, entry := range got {
		objects = append(objects, entry.Object())
	}
	want := []int{10, 20, 22}
	if !reflect.DeepEqual(objects, want) {
		t.Fatalf("visible xref objects = %v, want %v", objects, want)
	}
}

func TestXRefStreamMappingKeepsStaleIteratorKeysSeparate(t *testing.T) {
	raw := []byte{
		1, 1, 0,
		0, 0, 1,
		0, 0, 1,
		1, 2, 0,
	}
	data := []byte(fmt.Sprintf("4 0 obj\n<< /Type /XRef /Size 22 /W [1 1 1] /Index [10 2 20 2] /Length %d >>\nstream\n", len(raw)))
	data = append(data, raw...)
	data = append(data, []byte("\nendstream\nendobj\n")...)
	d := &Document{data: data}
	entries, _, err := d.parseXRefStream(0)
	if err != nil {
		t.Fatal(err)
	}
	visible := snapshotXRefEntries(entries)
	iterated := snapshotXRefIterEntries(entries)
	if got := []int{visible[0].Object()}; !reflect.DeepEqual(got, []int{10}) {
		t.Fatalf("public xref objects = %v, want [10]", got)
	}
	got := make([]int, 0, len(iterated))
	for _, entry := range iterated {
		got = append(got, entry.Object())
	}
	if !reflect.DeepEqual(got, []int{10, 20}) {
		t.Fatalf("mapping iterator objects = %v, want [10 20]", got)
	}
	if !iterated[1].Free() {
		t.Fatal("stale mapping iterator key lost its free marker")
	}
}

func TestParseXRefRejectsPrevCycle(t *testing.T) {
	d := &Document{data: []byte("%PDF-1.4\nxref\n0 1\n0000000000 65535 f \ntrailer\n<< /Size 1 /Prev 9 >>")}
	if _, _, err := d.parseXRefAt(9); err == nil {
		t.Fatal("expected xref Prev cycle error")
	}
}

func TestParseXRefAcceptsCROnlyLineEndings(t *testing.T) {
	d := &Document{data: []byte("xref\r0 1\r0000000000 65535 f \rtrailer\r<< /Size 1 >>\r")}
	entries, trailer, err := d.parseXRefAt(0)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || !entries[Ref{Generation: 65535}].isFree() {
		t.Fatalf("xref entries = %#v", entries)
	}
	if size, ok := IntValue(trailer[Name("Size")]); !ok || size != 1 {
		t.Fatalf("xref trailer = %#v", trailer)
	}
}

func TestParseXRefRejectsEmbeddedStartXRefMarker(t *testing.T) {
	d := &Document{data: []byte("xref\n0 1\n0000000000 65535 f \ntrailer\n<< /Size 1 >>\nnotstartxref\n0\n")}
	if _, _, err := d.parseXRef(); err == nil {
		t.Fatal("embedded startxref substring was accepted")
	}
}

func TestParseXRefReturnsTypedContext(t *testing.T) {
	d := &Document{data: []byte("%PDF-1.4")}
	_, _, err := d.parseXRefAt(-1)
	var parseErr *ParseError
	if !errors.As(err, &parseErr) {
		t.Fatalf("error type = %T, want ParseError", err)
	}
	if parseErr.Offset() != -1 || parseErr.Operation() != "xref" {
		t.Fatalf("xref parse error = %#v", parseErr)
	}
}

func TestMergeXRefEntriesRetainsOlderGeneration(t *testing.T) {
	newer := map[Ref]xrefEntry{
		{Object: 7, Generation: 1}: newXRefEntry(0, true, false, false),
	}
	older := map[Ref]xrefEntry{
		{Object: 7, Generation: 0}: newXRefEntry(123, false, false, true),
		{Object: 8, Generation: 0}: newXRefEntry(456, false, false, true),
	}

	mergeXRefEntries(newer, older)
	if _, ok := newer[Ref{Object: 7, Generation: 0}]; !ok {
		t.Fatal("older in-use generation was discarded")
	}
	if _, ok := newer[Ref{Object: 8, Generation: 0}]; !ok {
		t.Fatal("unchanged older object was not retained")
	}
}

func TestParseXRefInheritsUnchangedTrailerEntries(t *testing.T) {
	old := "xref\n0 1\n0000000000 65535 f \ntrailer\n<< /Size 2 /Root 1 0 R /ID [(stable)] >>\n"
	data := append(bytes.Repeat([]byte{' '}, 20), []byte(old)...)
	newOffset := len(data)
	data = append(data, []byte("xref\n0 1\n0000000000 65535 f \ntrailer\n<< /Size 2 /Prev 20 >>\n")...)

	d := &Document{data: data}
	_, trailer, err := d.parseXRefAt(newOffset)
	if err != nil {
		t.Fatal(err)
	}
	if trailer[Name("Root")] != (Ref{Object: 1}) {
		t.Fatalf("inherited trailer root = %#v", trailer[Name("Root")])
	}
	if _, ok := trailer[Name("ID")]; !ok {
		t.Fatal("inherited trailer ID is missing")
	}
}

func TestParseXRefRejectsMalformedHybridXRefStream(t *testing.T) {
	data := []byte("xref\n0 1\n0000000000 65535 f \ntrailer\n<< /Size 1 /XRefStm 100 >>\n")
	if len(data) > 100 {
		t.Fatal("test xref unexpectedly exceeds stream offset")
	}
	data = append(data, bytes.Repeat([]byte{' '}, 100-len(data))...)
	data = append(data, []byte("not an indirect object")...)

	d := &Document{data: data}
	if _, _, err := d.parseXRefAt(0); err == nil {
		t.Fatal("malformed hybrid xref stream was silently ignored")
	}
}

func TestXRefStreamOffsetResolvesIndirectReference(t *testing.T) {
	d := &Document{objects: map[Ref]Object{{Object: 3}: Number(77)}}
	trailer := Dict{Name("XRefStm"): Ref{Object: 3}}
	if got, ok, err := d.xrefStreamOffset(trailer); err != nil || !ok || got != 77 {
		t.Fatalf("indirect XRefStm offset = %d, %v, %v; want 77, true, nil", got, ok, err)
	}
}

func TestXRefStreamFieldsFollowMultiLevelIndirectValues(t *testing.T) {
	d := &Document{objects: map[Ref]Object{
		{Object: 1}: Ref{Object: 2}, {Object: 2}: Number(77),
		{Object: 3}: Ref{Object: 4}, {Object: 4}: Array{Number(0), Number(1), Number(1)},
		{Object: 5}: Ref{Object: 6}, {Object: 6}: Number(1),
	}}
	if offset, ok, err := d.xrefStreamOffset(Dict{Name("XRefStm"): Ref{Object: 1}}); err != nil || !ok || offset != 77 {
		t.Fatalf("nested xref stream offset = %d, %v, %v", offset, ok, err)
	}
	if got, ok, err := d.previousXRefOffset(Dict{Name("Prev"): Ref{Object: 5}}); err != nil || !ok || got != 1 {
		t.Fatalf("nested previous xref offset = %d, %v, %v", got, ok, err)
	}
}

func TestParseXRefRejectsMalformedRevisionOffsets(t *testing.T) {
	for _, key := range []string{"Prev", "XRefStm"} {
		t.Run(key, func(t *testing.T) {
			data := []byte("xref\n0 1\n0000000000 65535 f \ntrailer\n<< /Size 1 /" + key + " 1.5 >>")
			d := &Document{data: data}
			if _, _, err := d.parseXRefAt(0); err == nil {
				t.Fatalf("malformed %s offset was accepted", key)
			}
		})
	}
}

func TestValidateXRefStreamIndex(t *testing.T) {
	for name, index := range map[string][]int{
		"unsorted":     {4, 1, 2, 1},
		"outside size": {4, 2},
		"negative":     {-1, 1},
	} {
		t.Run(name, func(t *testing.T) {
			if err := validateXRefStreamIndex(5, index); err == nil {
				t.Fatalf("index %v was accepted", index)
			}
		})
	}
	if err := validateXRefStreamIndex(5, []int{0, 5}); err != nil {
		t.Fatalf("valid full index rejected: %v", err)
	}
}

func TestParseXRefRejectsNonNumericEntry(t *testing.T) {
	d := &Document{data: []byte("xref\n0 1\nnot-a-number 00000 n\ntrailer\n<< /Size 1 >>")}
	if _, _, err := d.parseXRefAt(0); err == nil {
		t.Fatal("non-numeric xref entry was accepted")
	}
}

func TestParseXRefRejectsUnknownEntryState(t *testing.T) {
	d := &Document{data: []byte("xref\n0 1\n0000000000 65535 x\ntrailer\n<< /Size 1 >>")}
	if _, _, err := d.parseXRefAt(0); err == nil {
		t.Fatal("unknown xref entry state was accepted")
	}
}

func TestParseXRefRejectsUnseparatedEntryState(t *testing.T) {
	d := &Document{data: []byte("xref\n0 1\n0000000000 65535n\ntrailer\n<< /Size 1 >>")}
	if _, _, err := d.parseXRefAt(0); err == nil {
		t.Fatal("unseparated xref entry state was accepted")
	}
}

func TestParseXRefRejectsTrailingEntryGarbage(t *testing.T) {
	d := &Document{data: []byte("xref\n0 1\n0000000000 65535 f garbage\ntrailer\n<< /Size 1 >>")}
	if _, _, err := d.parseXRefAt(0); err == nil {
		t.Fatal("trailing xref entry garbage was accepted")
	}
}

func TestParseXRefRejectsTrailingTrailerData(t *testing.T) {
	d := &Document{data: []byte("xref\n0 1\n0000000000 65535 f \ntrailer\n<< /Size 1 >> garbage")}
	if _, _, err := d.parseXRefAt(0); err == nil {
		t.Fatal("trailing trailer data was accepted")
	}
}

func TestParseXRefRejectsTrailingStartXRefData(t *testing.T) {
	d := &Document{data: []byte("startxref\n0\n12345\n")}
	if _, _, err := d.parseXRef(); err == nil {
		t.Fatal("trailing startxref data was accepted")
	}
}

func TestParseXRefAcceptsEOFAfterStartXRef(t *testing.T) {
	d := &Document{data: []byte("xref\n0 1\n0000000000 65535 f \ntrailer\n<< /Size 1 >>\nstartxref\n0\n%%EOF\n")}
	if _, _, err := d.parseXRef(); err != nil {
		t.Fatalf("standard EOF after startxref rejected: %v", err)
	}
}

func TestParseXRefRejectsMalformedTrailingXRefMarker(t *testing.T) {
	d := &Document{data: []byte("xref\n0 1\n0000000000 65535 f \ntrailer\n<< /Size 1 >>\nxref garbage")}
	if _, _, err := d.parseXRefAt(0); err == nil {
		t.Fatal("malformed trailing xref marker was accepted")
	}
}

func TestParseXRefRejectsNumberOverflow(t *testing.T) {
	d := &Document{data: []byte("xref\n0 1\n999999999999999999999999999999 00000 n\ntrailer\n<< /Size 1 >>")}
	if _, _, err := d.parseXRefAt(0); err == nil {
		t.Fatal("overflowing xref number was accepted")
	}
}

func TestParseXRefRejectsMalformedTrailerSize(t *testing.T) {
	for _, value := range []string{"1.5", "-1"} {
		t.Run(value, func(t *testing.T) {
			d := &Document{data: []byte("xref\n0 1\n0000000000 65535 f \ntrailer\n<< /Size " + value + " >>")}
			if _, _, err := d.parseXRefAt(0); err == nil {
				t.Fatalf("malformed trailer Size %s was accepted", value)
			}
		})
	}
}

func TestParseXRefRejectsObjectsOutsideTrailerSize(t *testing.T) {
	d := &Document{data: []byte("xref\n1 1\n0000000000 00000 n \ntrailer\n<< /Size 1 >>")}
	if _, _, err := d.parseXRefAt(0); err == nil {
		t.Fatal("xref object outside trailer Size was accepted")
	}
}

func TestParseXRefRejectsSubsectionObjectNumberOverflow(t *testing.T) {
	maxInt := int(^uint(0) >> 1)
	d := &Document{data: []byte("xref\n" + strconv.Itoa(maxInt) + " 2\n")}
	if _, _, err := d.parseXRefAt(0); err == nil {
		t.Fatal("overflowing xref subsection was accepted")
	}
}

func TestParseXRefRejectsDuplicateObjectAcrossSubsections(t *testing.T) {
	d := &Document{data: []byte("xref\n0 2\n0000000000 65535 f\n0000000010 00000 n\n1 1\n0000000020 00000 n\ntrailer\n<< /Size 2 >>")}
	if _, _, err := d.parseXRefAt(0); err == nil {
		t.Fatal("duplicate xref object was accepted")
	}
}

func TestLookupFallsBackPastFreeNewestGeneration(t *testing.T) {
	d := &Document{
		xrefs: map[Ref]xrefEntry{
			{Object: 7, Generation: 1}: newXRefEntry(0, true, false, false),
			{Object: 7, Generation: 0}: newXRefEntry(1, false, false, true),
		},
		objects: map[Ref]Object{{Object: 7, Generation: 0}: Number(70)},
	}
	wantRef := Ref{Object: 7, Generation: 0}
	if ref, value, ok := d.Lookup(7); !ok || ref != wantRef || value != Number(70) {
		t.Fatalf("free object lookup = (%v, %#v, %v), want older in-use generation", ref, value, ok)
	}
	if ref, value, ok := d.Lookup(7); !ok || ref != wantRef || value != Number(70) {
		t.Fatalf("cached free object lookup = (%v, %#v, %v), want older in-use generation", ref, value, ok)
	}
}
