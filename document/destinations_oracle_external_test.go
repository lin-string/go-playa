package document_test

import (
	"bytes"
	"fmt"
	"reflect"
	"testing"

	"github.com/lin-string/go-playa/document"
)

func TestDuplicateDestinationsRetainFirstValue(t *testing.T) {
	d, err := document.OpenBytes(duplicateDestinationsPDF())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = d.Close() }()
	for repeat := 0; repeat < 2; repeat++ {
		var raw []document.Object
		for entry, err := range d.NameTreeSeq("Dests") {
			if err != nil {
				t.Fatal(err)
			}
			raw = append(raw, entry.ValueCopy().(document.Array)[2])
		}
		if !reflect.DeepEqual(raw, []document.Object{document.Number(100), document.Number(200)}) {
			t.Fatalf("raw name tree = %v", raw)
		}
		var entries []document.DestinationEntry
		for entry, err := range d.DestinationsSeq() {
			if err != nil {
				t.Fatal(err)
			}
			entries = append(entries, entry.Finalize())
		}
		if len(entries) != 2 {
			t.Fatalf("entries = %d, want 2", len(entries))
		}
		for i, entry := range entries {
			dest, err := entry.DestinationWithError(d)
			if err != nil {
				t.Fatal(err)
			}
			if entry.Name() != "chapter" || dest == nil || !reflect.DeepEqual(dest.ParamsCopy(), []document.Object{document.Number(100)}) {
				t.Fatalf("entry %d = %q destination=%v, want first definition at 100", i, entry.Name(), dest)
			}
		}
		values, err := d.Destinations()
		if err != nil {
			t.Fatal(err)
		}
		if len(values) != 1 || values["chapter"].(document.Array)[2] != document.Number(100) {
			t.Fatalf("materialized destinations = %v, want first definition", values)
		}
		values["chapter"].(document.Array)[2] = document.Number(999)
		resolved, err := d.ResolveDestinationWithError(document.String("chapter"))
		if err != nil || resolved == nil || !reflect.DeepEqual(resolved.ParamsCopy(), []document.Object{document.Number(100)}) {
			t.Fatalf("named lookup = %v, err=%v, want first definition", resolved, err)
		}
		d.ReleaseTransientCaches()
		for i, entry := range entries {
			if entry.ValueCopy().(document.Array)[2] != document.Number(100) {
				t.Fatalf("retained entry %d changed after cache release", i)
			}
		}
	}
}

func duplicateDestinationsPDF() []byte {
	objects := []string{
		"<< /Type /Catalog /Pages 2 0 R /Names << /Dests << /Names [(chapter) [3 0 R /FitH 100] (chapter) [3 0 R /FitH 200]] >> >> >>",
		"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 300 300] >>",
	}
	var pdf bytes.Buffer
	pdf.WriteString("%PDF-1.7\n")
	offsets := make([]int, len(objects)+1)
	for i, object := range objects {
		offsets[i+1] = pdf.Len()
		fmt.Fprintf(&pdf, "%d 0 obj\n%s\nendobj\n", i+1, object)
	}
	xref := pdf.Len()
	fmt.Fprintf(&pdf, "xref\n0 %d\n0000000000 65535 f \n", len(offsets))
	for _, offset := range offsets[1:] {
		fmt.Fprintf(&pdf, "%010d 00000 n \n", offset)
	}
	fmt.Fprintf(&pdf, "trailer\n<< /Root 1 0 R /Size %d >>\nstartxref\n%d\n%%%%EOF\n", len(offsets), xref)
	return pdf.Bytes()
}
