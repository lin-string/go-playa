package document

import (
	"bytes"
	"errors"
	"strconv"
	"testing"
)

func TestParseXRefStreamRejectsOddIndex(t *testing.T) {
	d := &Document{data: []byte("\n1 0 obj\n<< /Type /XRef /W [1 1 1] /Index [0] /Length 0 >>\nstream\n\nendstream\nendobj")}
	if _, _, err := d.parseXRefStream(1); err == nil {
		t.Fatal("odd xref Index was accepted")
	} else {
		var parseErr *ParseError
		if !errors.As(err, &parseErr) || parseErr.Operation() != "xref stream" {
			t.Fatalf("xref stream error = %v", err)
		}
	}
}

func TestParseXRefStreamRetainsFreeEntries(t *testing.T) {
	// W=[1 1 1], one type-0 entry for object 7, next free-object pointer 0,
	// generation 1. The entry is intentionally retained for history merging.
	d := &Document{data: []byte("\n1 0 obj\n<< /Type /XRef /W [1 1 1] /Index [7 1] /Size 8 /Length 3 >>\nstream\n\x00\x00\x01\nendstream\nendobj")}
	entries, _, err := d.parseXRefStream(1)
	if err != nil {
		t.Fatal(err)
	}
	entry, ok := entries[Ref{Object: 7, Generation: 1}]
	if !ok || !entry.isFree() {
		t.Fatalf("free xref entry = %#v, present=%v", entry, ok)
	}
}

func TestParseXRefStreamResolvesIndirectLength(t *testing.T) {
	d := &Document{
		data:    []byte("\n1 0 obj\n<< /Type /XRef /W [1 1 1] /Size 1 /Length 3 0 R >>\nstream\n\x00\x00\x01\nendstream\nendobj"),
		objects: map[Ref]Object{{Object: 3}: Number(3)},
	}
	entries, _, err := d.parseXRefStream(1)
	if err != nil {
		t.Fatal(err)
	}
	entry, ok := entries[Ref{Object: 0, Generation: 1}]
	if !ok || !entry.isFree() {
		t.Fatalf("indirect-length xref entry = %#v, present=%v", entry, ok)
	}
}

func TestParseXRefStreamRejectsIntegerOverflow(t *testing.T) {
	d := &Document{data: []byte("\n1 0 obj\n<< /Type /XRef /W [1 8 1] /Index [7 1] /Size 8 /Length 10 >>\nstream\n\x01\xff\xff\xff\xff\xff\xff\xff\xff\x00\nendstream\nendobj")}
	if _, _, err := d.parseXRefStream(1); err == nil {
		t.Fatal("xref stream integer overflow was accepted")
	}
}

func TestParseXRefStreamRejectsUnknownEntryType(t *testing.T) {
	d := &Document{data: []byte("\n1 0 obj\n<< /Type /XRef /W [1 1 1] /Index [7 1] /Size 8 /Length 3 >>\nstream\n\x03\x00\x00\nendstream\nendobj")}
	if _, _, err := d.parseXRefStream(1); err == nil {
		t.Fatal("unknown xref stream entry type was silently ignored")
	}
}

func TestParseXRefStreamRejectsZeroObjectStreamNumber(t *testing.T) {
	d := &Document{data: []byte("\n1 0 obj\n<< /Type /XRef /W [1 1 1] /Index [7 1] /Size 8 /Length 3 >>\nstream\n\x02\x00\x00\nendstream\nendobj")}
	if _, _, err := d.parseXRefStream(1); err == nil {
		t.Fatal("zero xref object stream number was accepted")
	}
}

func TestParseXRefStreamRejectsTrailingDecodedRecords(t *testing.T) {
	d := &Document{data: []byte("\n1 0 obj\n<< /Type /XRef /W [1 1 1] /Index [7 1] /Size 8 /Length 4 >>\nstream\n\x01\x00\x00\x00\nendstream\nendobj")}
	if _, _, err := d.parseXRefStream(1); err == nil {
		t.Fatal("trailing xref stream bytes were accepted")
	}
}

func TestParseXRefStreamRejectsUnsortedIndexRanges(t *testing.T) {
	for _, index := range []string{"[7 1 7 1]", "[8 1 7 1]"} {
		t.Run(index, func(t *testing.T) {
			pdf := []byte("\n1 0 obj\n<< /Type /XRef /W [1 1 1] /Index " + index + " /Size 9 /Length 6 >>\nstream\n\x01\x00\x00\x01\x00\x00\nendstream\nendobj")
			if _, _, err := (&Document{data: pdf}).parseXRefStream(1); err == nil {
				t.Fatalf("unsorted xref Index %s was accepted", index)
			}
		})
	}
}

func TestParseXRefStreamRejectsInvalidFieldWidth(t *testing.T) {
	d := &Document{data: []byte("\n1 0 obj\n<< /Type /XRef /W [1 9 1] /Size 1 /Length 0 >>\nstream\n\nendstream\nendobj")}
	if _, _, err := d.parseXRefStream(1); err == nil {
		t.Fatal("oversized xref field width was accepted")
	}
}

func TestParseXRefStreamRejectsExtraFieldWidths(t *testing.T) {
	d := &Document{data: []byte("\n1 0 obj\n<< /Type /XRef /W [1 1 1 1] /Index [7 1] /Length 3 >>\nstream\n\x01\x00\x00\nendstream\nendobj")}
	if _, _, err := d.parseXRefStream(1); err == nil {
		t.Fatal("xref stream with extra field widths was accepted")
	}
}

func TestParseXRefStreamRejectsZeroWidthRecords(t *testing.T) {
	d := &Document{data: []byte("\n1 0 obj\n<< /Type /XRef /W [0 0 0] /Index [0 1000000] /Length 0 >>\nstream\n\nendstream\nendobj")}
	if _, _, err := d.parseXRefStream(1); err == nil {
		t.Fatal("zero-width xref records were accepted")
	}
}

func TestParseXRefStreamRejectsObjectNumberOverflow(t *testing.T) {
	maxInt := int(^uint(0) >> 1)
	first := maxInt - 1024
	raw := bytes.Repeat([]byte{1, 0, 0}, 2048)
	pdf := append([]byte("\n1 0 obj\n<< /Type /XRef /W [1 1 1] /Index ["+strconv.Itoa(first)+" 2048] /Length 6144 >>\nstream\n"), raw...)
	pdf = append(pdf, []byte("\nendstream\nendobj")...)
	if _, _, err := (&Document{data: pdf}).parseXRefStream(1); err == nil {
		t.Fatal("xref object-number overflow was accepted")
	}
}
