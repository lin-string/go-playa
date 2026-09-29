package document

import (
	"fmt"
	"testing"
)

func TestOpenBytesReadsIncrementalPageRevision(t *testing.T) {
	data := []byte("%PDF-1.4\n")
	offsets := make([]int, 4)
	appendObject := func(number int, body string) {
		offsets[number] = len(data)
		data = append(data, fmt.Sprintf("%d 0 obj\n%s\nendobj\n", number, body)...)
	}
	appendObject(1, "<< /Type /Catalog /Pages 2 0 R >>")
	appendObject(2, "<< /Type /Pages /Kids [3 0 R] /Count 1 >>")
	appendObject(3, "<< /Type /Page /Parent 2 0 R /MediaBox [0 0 595 842] >>")
	firstXRef := len(data)
	data = append(data, []byte("xref\n0 4\n0000000000 65535 f \n")...)
	for number := 1; number < len(offsets); number++ {
		data = append(data, []byte(fmt.Sprintf("%010d 00000 n \n", offsets[number]))...)
	}
	data = append(data, []byte(fmt.Sprintf("trailer\n<< /Size 4 /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", firstXRef))...)

	updatedPage := len(data)
	data = append(data, []byte("3 0 obj\n<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] >>\nendobj\n")...)
	data = append(data, []byte(fmt.Sprintf("xref\n3 1\n%010d 00000 n \ntrailer\n<< /Size 4 /Prev %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", updatedPage, firstXRef, updatedPage+len("3 0 obj\n<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] >>\nendobj\n")))...)

	d, err := OpenBytes(data)
	if err != nil {
		t.Fatal(err)
	}
	if got := d.RevisionCount(); got != 2 {
		t.Fatalf("revision count = %d, want 2", got)
	}
	if history := d.XRefHistory(); len(history) != 1 || history[0] != firstXRef {
		t.Fatalf("xref history = %v, want [%d]", history, firstXRef)
	}
	page, err := d.PageAt(0)
	if err != nil {
		t.Fatal(err)
	}
	if got := page.MediaBox(d); got != [4]float64{0, 0, 612, 792} {
		t.Fatalf("updated page box = %v", got)
	}
}

func TestOpenBytesResolvesIndirectIncrementalPrev(t *testing.T) {
	data := []byte("%PDF-1.4\n")
	offsets := make([]int, 4)
	appendObject := func(number int, body string) {
		offsets[number] = len(data)
		data = append(data, fmt.Sprintf("%d 0 obj\n%s\nendobj\n", number, body)...)
	}
	appendObject(1, "<< /Type /Catalog /Pages 2 0 R >>")
	appendObject(2, "<< /Type /Pages /Kids [3 0 R] /Count 1 >>")
	appendObject(3, "<< /Type /Page /Parent 2 0 R /MediaBox [0 0 595 842] >>")
	firstXRef := len(data)
	data = append(data, []byte("xref\n0 4\n0000000000 65535 f \n")...)
	for number := 1; number < len(offsets); number++ {
		data = append(data, []byte(fmt.Sprintf("%010d 00000 n \n", offsets[number]))...)
	}
	data = append(data, []byte(fmt.Sprintf("trailer\n<< /Size 4 /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", firstXRef))...)

	updatedPage := len(data)
	data = append(data, []byte("3 0 obj\n<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] >>\nendobj\n")...)
	prevObject := len(data)
	data = append(data, []byte(fmt.Sprintf("5 0 obj\n%d\nendobj\n", firstXRef))...)
	secondXRef := len(data)
	data = append(data, []byte(fmt.Sprintf("xref\n3 1\n%010d 00000 n \n5 1\n%010d 00000 n \ntrailer\n<< /Size 6 /Prev 5 0 R >>\nstartxref\n%d\n%%%%EOF\n", updatedPage, prevObject, secondXRef))...)

	d, err := OpenBytes(data)
	if err != nil {
		t.Fatal(err)
	}
	if got := d.RevisionCount(); got != 2 {
		t.Fatalf("revision count = %d, want 2", got)
	}
	page, err := d.PageAt(0)
	if err != nil {
		t.Fatal(err)
	}
	if got := page.MediaBox(d); got != [4]float64{0, 0, 612, 792} {
		t.Fatalf("indirect Prev page box = %v", got)
	}
}

func TestOpenBytesReadsMultipleIncrementalPageRevisions(t *testing.T) {
	data := []byte("%PDF-1.4\n")
	objects := make([]int, 4)
	appendObject := func(number int, body string) {
		objects[number] = len(data)
		data = append(data, fmt.Sprintf("%d 0 obj\n%s\nendobj\n", number, body)...)
	}
	appendObject(1, "<< /Type /Catalog /Pages 2 0 R >>")
	appendObject(2, "<< /Type /Pages /Kids [3 0 R] /Count 1 >>")
	appendObject(3, "<< /Type /Page /Parent 2 0 R /MediaBox [0 0 595 842] >>")
	firstXRef := len(data)
	data = append(data, []byte("xref\n0 4\n0000000000 65535 f \n")...)
	for number := 1; number < len(objects); number++ {
		data = append(data, []byte(fmt.Sprintf("%010d 00000 n \n", objects[number]))...)
	}
	data = append(data, []byte(fmt.Sprintf("trailer\n<< /Size 4 /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", firstXRef))...)

	appendPageRevision := func(box string, previous int) int {
		page := len(data)
		body := fmt.Sprintf("3 0 obj\n<< /Type /Page /Parent 2 0 R /MediaBox %s >>\nendobj\n", box)
		data = append(data, []byte(body)...)
		xref := len(data)
		data = append(data, []byte(fmt.Sprintf("xref\n3 1\n%010d 00000 n \ntrailer\n<< /Size 4 /Prev %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", page, previous, xref))...)
		return xref
	}
	secondXRef := appendPageRevision("[0 0 612 792]", firstXRef)
	appendPageRevision("[0 0 720 720]", secondXRef)

	d, err := OpenBytes(data)
	if err != nil {
		t.Fatal(err)
	}
	if got := d.RevisionCount(); got != 3 {
		t.Fatalf("revision count = %d, want 3", got)
	}
	if history := d.XRefHistory(); len(history) != 2 || history[0] != secondXRef || history[1] != firstXRef {
		t.Fatalf("xref history = %v, want [%d %d]", history, secondXRef, firstXRef)
	}
	page, err := d.PageAt(0)
	if err != nil {
		t.Fatal(err)
	}
	if got := page.MediaBox(d); got != [4]float64{0, 0, 720, 720} {
		t.Fatalf("latest page box = %v", got)
	}
}

func TestOpenBytesAppliesIncrementalFreeGeneration(t *testing.T) {
	data := []byte("%PDF-1.4\n")
	offsets := make([]int, 4)
	appendObject := func(number int, body string) {
		offsets[number] = len(data)
		data = append(data, fmt.Sprintf("%d 0 obj\n%s\nendobj\n", number, body)...)
	}
	appendObject(1, "<< /Type /Catalog /Pages 2 0 R >>")
	appendObject(2, "<< /Type /Pages /Kids [3 0 R] /Count 1 >>")
	appendObject(3, "<< /Type /Page /Parent 2 0 R /MediaBox [0 0 595 842] >>")
	firstXRef := len(data)
	data = append(data, []byte("xref\n0 4\n0000000000 65535 f \n")...)
	for number := 1; number < len(offsets); number++ {
		data = append(data, []byte(fmt.Sprintf("%010d 00000 n \n", offsets[number]))...)
	}
	data = append(data, []byte(fmt.Sprintf("trailer\n<< /Size 4 /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", firstXRef))...)

	secondXRef := len(data)
	data = append(data, []byte(fmt.Sprintf("xref\n3 1\n0000000000 00001 f \ntrailer\n<< /Size 4 /Prev %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", firstXRef, secondXRef))...)

	d, err := OpenBytes(data)
	if err != nil {
		t.Fatal(err)
	}
	if got := d.RevisionCount(); got != 2 {
		t.Fatalf("revision count = %d, want 2", got)
	}
	if ref, value, ok := d.Lookup(3); !ok || ref != (Ref{Object: 3}) {
		t.Fatalf("Lookup did not fall through the free generation: ref=%v value=%T ok=%v", ref, value, ok)
	}
	page, err := d.PageAt(0)
	if err != nil || page.MediaBox(d) != [4]float64{0, 0, 595, 842} {
		t.Fatalf("PageAt free-generation fallback = %#v, err=%v", page, err)
	}
	found := false
	for object, err := range d.Items() {
		if err != nil {
			t.Fatal(err)
		}
		if object.Ref().Object == 3 {
			found = true
			if _, ok := object.ValueCopy().(Dict); !ok {
				t.Fatalf("mapping fallback object type = %T, want Dict", object.ValueCopy())
			}
		}
	}
	if !found {
		t.Fatal("mapping omitted the older visible generation after a free update")
	}
}

func TestPageContentRebuildsStaleXRefEntry(t *testing.T) {
	data := []byte("%PDF-1.4\n")
	offsets := make([]int, 9)
	appendObject := func(number int, body string) {
		offsets[number] = len(data)
		data = append(data, fmt.Sprintf("%d 0 obj\n%s\nendobj\n", number, body)...)
	}
	appendObject(1, "<< /Type /Catalog /Pages 2 0 R >>")
	appendObject(2, "<< /Type /Pages /Kids [3 0 R] /Count 1 >>")
	appendObject(3, "<< /Type /Page /Parent 2 0 R /MediaBox [0 0 10 10] /Contents 4 0 R >>")
	appendObject(4, "<< /Length 1 >>\nstream\nq\nendstream")
	appendObject(5, "<< /Length 1 >>\nstream\n(\nendstream")
	offsets[4] = len(data)
	data = append(data, []byte("4 2 obj\n<< /Length 1 >>\nstream\nQ\nendstream\nendobj\n")...)
	appendObject(6, "42")
	appendObject(8, "<< /Type /ObjStm /N 1 /First 4 /Length 6 >>\nstream\n7 0 42\nendstream")
	xref := len(data)
	data = append(data, []byte("xref\n0 9\n0000000000 65535 f \n")...)
	for number := 1; number < len(offsets); number++ {
		if number == 7 {
			data = append(data, []byte("0000000000 00000 f \n")...)
			continue
		}
		offset := offsets[number]
		if number == 4 || number == 6 {
			offset = offsets[3]
		}
		data = append(data, fmt.Sprintf("%010d 00000 n \n", offset)...)
	}
	data = append(data, fmt.Sprintf("trailer\n<< /Size 9 /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", xref)...)

	d, err := OpenBytes(data)
	if err != nil {
		t.Fatal(err)
	}
	page, err := d.PageAt(0)
	if err != nil {
		t.Fatal(err)
	}
	var before []Ref
	for ref, err := range d.ObjectRefs() {
		if err != nil {
			t.Fatal(err)
		}
		before = append(before, ref)
	}
	if !containsRef(before, Ref{Object: 4}) {
		t.Fatalf("pre-recovery refs = %v, want stale 4 0 R", before)
	}
	if d.WasRecovered() {
		t.Fatal("document reported recovery before resolving stale content")
	}
	for attempt := 0; attempt < 2; attempt++ {
		content, err := page.Content(d)
		if err != nil || string(content) != "Q\n" {
			t.Fatalf("Content attempt %d = %q, err=%v, want latest recovered Q", attempt+1, content, err)
		}
	}
	if !d.WasRecovered() {
		t.Fatal("document did not report lazy xref fallback recovery")
	}
	if d.Len() != 8 || d.Trailer()[Name("Size")] != Number(8) {
		t.Fatalf("fallback size = Len %d, trailer %#v, want 8", d.Len(), d.Trailer()[Name("Size")])
	}
	var after []Ref
	for ref, err := range d.ObjectRefs() {
		if err != nil {
			t.Fatal(err)
		}
		after = append(after, ref)
	}
	if containsRef(after, Ref{Object: 4}) || !containsRef(after, Ref{Object: 4, Generation: 2}) {
		t.Fatalf("post-recovery refs = %v, want only 4 2 R", after)
	}
	found := false
	foundSecondStale := false
	for item, err := range d.Items() {
		if err != nil {
			t.Fatal(err)
		}
		if item.Ref() == (Ref{Object: 4, Generation: 2}) {
			found = true
		}
		if item.Ref() == (Ref{Object: 6}) {
			foundSecondStale = true
		}
	}
	if !found {
		t.Fatal("post-recovery Items omitted 4 2 R")
	}
	if !foundSecondStale {
		t.Fatal("global fallback omitted independently stale object 6")
	}
	if value, ok := d.Get(7); !ok || value != Number(42) {
		t.Fatalf("recovered ObjStm member 7 = %#v, ok=%v", value, ok)
	}
	for _, err := range d.Values() {
		if err != nil {
			t.Fatal(err)
		}
	}
	if err := d.Close(); err != nil {
		t.Fatal(err)
	}
	if d.Len() != 0 || d.Trailer() != nil || d.WasRecovered() {
		t.Fatalf("closed recovered document retained state: Len=%d trailer=%#v recovered=%v", d.Len(), d.Trailer(), d.WasRecovered())
	}
}

func containsRef(refs []Ref, want Ref) bool {
	for _, ref := range refs {
		if ref == want {
			return true
		}
	}
	return false
}

func TestRecoveredObjectStreamXRefsBoundsHugeN(t *testing.T) {
	d := &Document{data: []byte("8 0 obj\n<< /Type /ObjStm /N 4611686018427387904 /Length 6 >>\nstream\n7 0 42\nendstream\nendobj")}
	candidates := map[int]recoveredXRefObject{8: {ref: Ref{Object: 8}, offset: 0}}
	if entries := d.recoveredObjectStreamXRefs(candidates); len(entries) != 1 {
		t.Fatalf("huge-N recovered entries = %#v, want one bounded header member", entries)
	}
}

func TestPageContentAcceptsXRefGenerationMismatch(t *testing.T) {
	data := []byte("%PDF-1.4\n")
	offsets := make([]int, 5)
	appendObject := func(number, generation int, body string) {
		offsets[number] = len(data)
		data = append(data, fmt.Sprintf("%d %d obj\n%s\nendobj\n", number, generation, body)...)
	}
	appendObject(1, 0, "<< /Type /Catalog /Pages 2 0 R >>")
	appendObject(2, 0, "<< /Type /Pages /Kids [3 0 R] /Count 1 >>")
	appendObject(3, 0, "<< /Type /Page /Parent 2 0 R /MediaBox [0 0 10 10] /Contents 4 0 R >>")
	appendObject(4, 2, "<< /Length 1 >>\nstream\nq\nendstream")
	xref := len(data)
	data = append(data, []byte("xref\n0 5\n0000000000 65535 f \n")...)
	for number := 1; number < len(offsets); number++ {
		data = append(data, fmt.Sprintf("%010d 00000 n \n", offsets[number])...)
	}
	data = append(data, fmt.Sprintf("trailer\n<< /Size 5 /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", xref)...)
	d, err := OpenBytes(data)
	if err != nil {
		t.Fatal(err)
	}
	page, err := d.PageAt(0)
	if err != nil {
		t.Fatal(err)
	}
	content, err := page.Content(d)
	if err != nil || string(content) != "q\n" {
		t.Fatalf("Content = %q, err=%v", content, err)
	}
	ref, _, ok := d.Lookup(4)
	if !ok || ref != (Ref{Object: 4}) {
		t.Fatalf("Lookup ref = %v, ok=%v, want xref generation 0", ref, ok)
	}
}

func TestOpenBytesReadsXRefStreamIncrementalRevisionAfterClassicXRef(t *testing.T) {
	data := []byte("%PDF-1.4\n")
	offsets := make([]int, 4)
	appendObject := func(number int, body string) {
		offsets[number] = len(data)
		data = append(data, fmt.Sprintf("%d 0 obj\n%s\nendobj\n", number, body)...)
	}
	appendObject(1, "<< /Type /Catalog /Pages 2 0 R >>")
	appendObject(2, "<< /Type /Pages /Kids [3 0 R] /Count 1 >>")
	appendObject(3, "<< /Type /Page /Parent 2 0 R /MediaBox [0 0 595 842] >>")
	firstXRef := len(data)
	data = append(data, []byte("xref\n0 4\n0000000000 65535 f \n")...)
	for number := 1; number < len(offsets); number++ {
		data = append(data, []byte(fmt.Sprintf("%010d 00000 n \n", offsets[number]))...)
	}
	data = append(data, []byte(fmt.Sprintf("trailer\n<< /Size 4 /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", firstXRef))...)

	updatedPage := len(data)
	data = append(data, []byte("3 0 obj\n<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] >>\nendobj\n")...)
	xrefStreamOffset := len(data)
	record := func(kind, value, generation int) []byte {
		return []byte{byte(kind), byte(value >> 24), byte(value >> 16), byte(value >> 8), byte(value), byte(generation >> 8), byte(generation)}
	}
	raw := append(record(1, updatedPage, 0), record(1, xrefStreamOffset, 0)...)
	data = append(data, fmt.Sprintf("4 0 obj\n<< /Type /XRef /Size 5 /Root 1 0 R /Prev %d /Index [3 2] /W [1 4 2] /Length %d >>\nstream\n", firstXRef, len(raw))...)
	data = append(data, raw...)
	data = append(data, []byte(fmt.Sprintf("\nendstream\nendobj\nstartxref\n%d\n%%%%EOF\n", xrefStreamOffset))...)

	d, err := OpenBytes(data)
	if err != nil {
		t.Fatal(err)
	}
	if got := d.RevisionCount(); got != 2 {
		t.Fatalf("revision count = %d, want 2", got)
	}
	page, err := d.PageAt(0)
	if err != nil {
		t.Fatal(err)
	}
	if got := page.MediaBox(d); got != [4]float64{0, 0, 612, 792} {
		t.Fatalf("mixed incremental page box = %v", got)
	}
}

func TestOpenBytesReadsClassicIncrementalRevisionAfterXRefStream(t *testing.T) {
	data := []byte("%PDF-1.5\n")
	offsets := make([]int, 5)
	appendObject := func(number int, body string) {
		offsets[number] = len(data)
		data = append(data, fmt.Sprintf("%d 0 obj\n%s\nendobj\n", number, body)...)
	}
	appendObject(1, "<< /Type /Catalog /Pages 2 0 R >>")
	appendObject(2, "<< /Type /Pages /Kids [3 0 R] /Count 1 >>")
	appendObject(3, "<< /Type /Page /Parent 2 0 R /MediaBox [0 0 595 842] >>")
	xrefStreamOffset := len(data)
	record := func(kind, value, generation int) []byte {
		return []byte{byte(kind), byte(value >> 24), byte(value >> 16), byte(value >> 8), byte(value), byte(generation >> 8), byte(generation)}
	}
	raw := append(record(0, 0, 65535), record(1, offsets[1], 0)...)
	raw = append(raw, record(1, offsets[2], 0)...)
	raw = append(raw, record(1, offsets[3], 0)...)
	raw = append(raw, record(1, xrefStreamOffset, 0)...)
	data = append(data, fmt.Sprintf("4 0 obj\n<< /Type /XRef /Size 5 /Root 1 0 R /W [1 4 2] /Length %d >>\nstream\n", len(raw))...)
	data = append(data, raw...)
	data = append(data, []byte(fmt.Sprintf("\nendstream\nendobj\nstartxref\n%d\n%%%%EOF\n", xrefStreamOffset))...)

	updatedPage := len(data)
	data = append(data, []byte("3 0 obj\n<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] >>\nendobj\n")...)
	classicXRef := len(data)
	data = append(data, []byte(fmt.Sprintf("xref\n3 1\n%010d 00000 n \ntrailer\n<< /Size 5 /Root 1 0 R /Prev %d >>\nstartxref\n%d\n%%%%EOF\n", updatedPage, xrefStreamOffset, classicXRef))...)

	d, err := OpenBytes(data)
	if err != nil {
		t.Fatal(err)
	}
	if got := d.RevisionCount(); got != 2 {
		t.Fatalf("revision count = %d, want 2", got)
	}
	page, err := d.PageAt(0)
	if err != nil {
		t.Fatal(err)
	}
	if got := page.MediaBox(d); got != [4]float64{0, 0, 612, 792} {
		t.Fatalf("reverse mixed incremental page box = %v", got)
	}
}

func TestOpenBytesResolvesIndirectPrevFromXRefStream(t *testing.T) {
	data := []byte("%PDF-1.5\n")
	offsets := make([]int, 4)
	appendObject := func(number int, body string) {
		offsets[number] = len(data)
		data = append(data, fmt.Sprintf("%d 0 obj\n%s\nendobj\n", number, body)...)
	}
	appendObject(1, "<< /Type /Catalog /Pages 2 0 R >>")
	appendObject(2, "<< /Type /Pages /Kids [3 0 R] /Count 1 >>")
	appendObject(3, "<< /Type /Page /Parent 2 0 R /MediaBox [0 0 595 842] >>")
	firstXRef := len(data)
	data = append(data, []byte("xref\n0 4\n0000000000 65535 f \n")...)
	for number := 1; number < len(offsets); number++ {
		data = append(data, []byte(fmt.Sprintf("%010d 00000 n \n", offsets[number]))...)
	}
	data = append(data, []byte(fmt.Sprintf("trailer\n<< /Size 4 /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", firstXRef))...)

	updatedPage := len(data)
	data = append(data, []byte("3 0 obj\n<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] >>\nendobj\n")...)
	prevObject := len(data)
	data = append(data, []byte(fmt.Sprintf("5 0 obj\n%d\nendobj\n", firstXRef))...)
	xrefStreamOffset := len(data)
	record := func(kind, value, generation int) []byte {
		return []byte{byte(kind), byte(value >> 24), byte(value >> 16), byte(value >> 8), byte(value), byte(generation >> 8), byte(generation)}
	}
	raw := append(record(1, updatedPage, 0), record(1, xrefStreamOffset, 0)...)
	raw = append(raw, record(1, prevObject, 0)...)
	data = append(data, fmt.Sprintf("4 0 obj\n<< /Type /XRef /Size 6 /Root 1 0 R /Prev 5 0 R /Index [3 3] /W [1 4 2] /Length %d >>\nstream\n", len(raw))...)
	data = append(data, raw...)
	data = append(data, []byte(fmt.Sprintf("\nendstream\nendobj\nstartxref\n%d\n%%%%EOF\n", xrefStreamOffset))...)

	d, err := OpenBytes(data)
	if err != nil {
		t.Fatal(err)
	}
	if got := d.RevisionCount(); got != 2 {
		t.Fatalf("revision count = %d, want 2", got)
	}
	page, err := d.PageAt(0)
	if err != nil {
		t.Fatal(err)
	}
	if got := page.MediaBox(d); got != [4]float64{0, 0, 612, 792} {
		t.Fatalf("indirect xref-stream Prev page box = %v", got)
	}
}

func TestOpenBytesResolvesIndirectPrevFromCompressedObject(t *testing.T) {
	data := []byte("%PDF-1.5\n")
	offsets := make([]int, 4)
	appendObject := func(number int, body string) {
		offsets[number] = len(data)
		data = append(data, fmt.Sprintf("%d 0 obj\n%s\nendobj\n", number, body)...)
	}
	appendObject(1, "<< /Type /Catalog /Pages 2 0 R >>")
	appendObject(2, "<< /Type /Pages /Kids [3 0 R] /Count 1 >>")
	appendObject(3, "<< /Type /Page /Parent 2 0 R /MediaBox [0 0 595 842] >>")
	firstXRef := len(data)
	data = append(data, []byte("xref\n0 4\n0000000000 65535 f \n")...)
	for number := 1; number < len(offsets); number++ {
		data = append(data, []byte(fmt.Sprintf("%010d 00000 n \n", offsets[number]))...)
	}
	data = append(data, []byte(fmt.Sprintf("trailer\n<< /Size 4 /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", firstXRef))...)

	updatedPage := len(data)
	data = append(data, []byte("3 0 obj\n<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] >>\nendobj\n")...)
	objectStreamData := fmt.Sprintf("5 0 %d", firstXRef)
	objectStreamOffset := len(data)
	data = append(data, []byte(fmt.Sprintf("6 0 obj\n<< /Type /ObjStm /N 1 /First 4 /Length %d >>\nstream\n%s\nendstream\nendobj\n", len(objectStreamData), objectStreamData))...)
	xrefStreamOffset := len(data)
	record := func(kind, value, generation int) []byte {
		return []byte{byte(kind), byte(value >> 24), byte(value >> 16), byte(value >> 8), byte(value), byte(generation >> 8), byte(generation)}
	}
	compressed := func(objectStream, index int) []byte {
		return []byte{2, byte(objectStream >> 24), byte(objectStream >> 16), byte(objectStream >> 8), byte(objectStream), byte(index >> 8), byte(index)}
	}
	raw := append(record(1, updatedPage, 0), record(1, xrefStreamOffset, 0)...)
	raw = append(raw, compressed(6, 0)...)
	raw = append(raw, record(1, objectStreamOffset, 0)...)
	data = append(data, fmt.Sprintf("4 0 obj\n<< /Type /XRef /Size 7 /Root 1 0 R /Prev 5 0 R /Index [3 4] /W [1 4 2] /Length %d >>\nstream\n", len(raw))...)
	data = append(data, raw...)
	data = append(data, []byte(fmt.Sprintf("\nendstream\nendobj\nstartxref\n%d\n%%%%EOF\n", xrefStreamOffset))...)

	d, err := OpenBytes(data)
	if err != nil {
		t.Fatal(err)
	}
	if got := d.RevisionCount(); got != 2 {
		t.Fatalf("revision count = %d, want 2", got)
	}
	page, err := d.PageAt(0)
	if err != nil {
		t.Fatal(err)
	}
	if got := page.MediaBox(d); got != [4]float64{0, 0, 612, 792} {
		t.Fatalf("compressed indirect Prev page box = %v", got)
	}
}

func TestOpenBytesReadsXRefStreamPDF(t *testing.T) {
	data := []byte("%PDF-1.5\n")
	offsets := make([]int, 5)
	appendObject := func(number int, body string) {
		offsets[number] = len(data)
		data = append(data, fmt.Sprintf("%d 0 obj\n%s\nendobj\n", number, body)...)
	}
	appendObject(1, "<< /Type /Catalog /Pages 2 0 R >>")
	appendObject(2, "<< /Type /Pages /Kids [3 0 R] /Count 1 >>")
	appendObject(3, "<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] >>")
	xrefOffset := len(data)
	record := func(kind, value, generation int) []byte {
		return []byte{byte(kind), byte(value >> 24), byte(value >> 16), byte(value >> 8), byte(value), byte(generation >> 8), byte(generation)}
	}
	raw := append(record(0, 0, 65535), record(1, offsets[1], 0)...)
	raw = append(raw, record(1, offsets[2], 0)...)
	raw = append(raw, record(1, offsets[3], 0)...)
	raw = append(raw, record(1, xrefOffset, 0)...)
	data = append(data, fmt.Sprintf("4 0 obj\n<< /Type /XRef /Size 5 /Root 1 0 R /W [1 4 2] /Length %d >>\nstream\n", len(raw))...)
	data = append(data, raw...)
	data = append(data, []byte(fmt.Sprintf("\nendstream\nendobj\nstartxref\n%d\n%%%%EOF\n", xrefOffset))...)

	d, err := OpenBytes(data)
	if err != nil {
		t.Fatal(err)
	}
	page, err := d.PageAt(0)
	if err != nil {
		t.Fatal(err)
	}
	if got := page.MediaBox(d); got != [4]float64{0, 0, 612, 792} {
		t.Fatalf("xref-stream page box = %v", got)
	}
}

func TestOpenBytesResolvesIndirectXRefStreamFields(t *testing.T) {
	data := []byte("%PDF-1.5\n")
	offsets := make([]int, 4)
	appendObject := func(number int, body string) {
		offsets[number] = len(data)
		data = append(data, fmt.Sprintf("%d 0 obj\n%s\nendobj\n", number, body)...)
	}
	appendObject(1, "<< /Type /Catalog /Pages 2 0 R >>")
	appendObject(2, "<< /Type /Pages /Kids [3 0 R] /Count 1 >>")
	appendObject(3, "<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] >>")
	wOffset := len(data)
	data = append(data, []byte("5 0 obj\n[1 4 2]\nendobj\n")...)
	data = append(data, []byte("9 0 obj\n9\nendobj\n")...)
	xrefStreamOffset := len(data)
	record := func(kind, value, generation int) []byte {
		return []byte{byte(kind), byte(value >> 24), byte(value >> 16), byte(value >> 8), byte(value), byte(generation >> 8), byte(generation)}
	}
	raw := append(record(0, 0, 65535), record(1, offsets[1], 0)...)
	raw = append(raw, record(1, offsets[2], 0)...)
	raw = append(raw, record(1, offsets[3], 0)...)
	raw = append(raw, record(1, xrefStreamOffset, 0)...)
	raw = append(raw, record(1, wOffset, 0)...)
	data = append(data, fmt.Sprintf("4 0 obj\n<< /Type /XRef /Size 9 0 R /W 5 0 R /Index [0 6] /Length %d >>\nstream\n", len(raw))...)
	data = append(data, raw...)
	data = append(data, []byte(fmt.Sprintf("\nendstream\nendobj\nstartxref\n%d\n%%%%EOF\n", xrefStreamOffset))...)

	d, err := OpenBytes(data)
	if err != nil {
		t.Fatal(err)
	}
	if d.WasRecovered() {
		t.Fatalf("indirect xref-stream fields unexpectedly used recovery: %v", d.RecoveryError())
	}
	page, err := d.PageAt(0)
	if err != nil {
		t.Fatal(err)
	}
	if got := page.MediaBox(d); got != [4]float64{0, 0, 612, 792} {
		t.Fatalf("indirect xref-stream fields page box = %v", got)
	}
}

func TestParseXRefStreamRejectsNonXRefType(t *testing.T) {
	raw := []byte{0, 0, 255, 1, 0, 0}
	data := []byte(fmt.Sprintf("5 0 obj\n<< /Type /NotXRef /Size 2 /W [1 1 1] /Length %d >>\nstream\n", len(raw)))
	data = append(data, raw...)
	data = append(data, []byte("\nendstream\nendobj\n")...)
	d := &Document{data: data}
	if _, _, err := d.parseXRefStream(0); err == nil {
		t.Fatal("xref stream with non-XRef Type was accepted")
	}
}

func TestOpenBytesReadsHybridXRefPDF(t *testing.T) {
	data := []byte("%PDF-1.5\n")
	offsets := make([]int, 5)
	appendObject := func(number int, body string) {
		offsets[number] = len(data)
		data = append(data, fmt.Sprintf("%d 0 obj\n%s\nendobj\n", number, body)...)
	}
	appendObject(1, "<< /Type /Catalog /Pages 2 0 R >>")
	appendObject(2, "<< /Type /Pages /Kids [3 0 R] /Count 1 >>")
	appendObject(3, "<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] >>")
	xrefOffset := len(data)
	data = append(data, []byte("xref\n0 3\n0000000000 65535 f \n")...)
	for number := 1; number <= 2; number++ {
		data = append(data, []byte(fmt.Sprintf("%010d 00000 n \n", offsets[number]))...)
	}
	xrefStreamOffset := len(data)
	record := func(kind, value, generation int) []byte {
		return []byte{byte(kind), byte(value >> 24), byte(value >> 16), byte(value >> 8), byte(value), byte(generation >> 8), byte(generation)}
	}
	raw := append(record(1, offsets[3], 0), record(1, xrefStreamOffset, 0)...)
	data = append(data, fmt.Sprintf("4 0 obj\n<< /Type /XRef /Size 5 /Root 1 0 R /W [1 4 2] /Index [3 2] /Length %d >>\nstream\n", len(raw))...)
	data = append(data, raw...)
	data = append(data, []byte(fmt.Sprintf("\nendstream\nendobj\ntrailer\n<< /Size 5 /Root 1 0 R /XRefStm %d >>\nstartxref\n%d\n%%%%EOF\n", xrefStreamOffset, xrefOffset))...)

	d, err := OpenBytes(data)
	if err != nil {
		t.Fatal(err)
	}
	page, err := d.PageAt(0)
	if err != nil {
		t.Fatal(err)
	}
	if got := page.MediaBox(d); got != [4]float64{0, 0, 612, 792} {
		t.Fatalf("hybrid xref page box = %v", got)
	}
}

func TestOpenBytesResolvesIndirectHybridXRefStream(t *testing.T) {
	data := []byte("%PDF-1.5\n")
	offsets := make([]int, 5)
	appendObject := func(number int, body string) {
		offsets[number] = len(data)
		data = append(data, fmt.Sprintf("%d 0 obj\n%s\nendobj\n", number, body)...)
	}
	appendObject(1, "<< /Type /Catalog /Pages 2 0 R >>")
	appendObject(2, "<< /Type /Pages /Kids [3 0 R] /Count 1 >>")
	appendObject(3, "<< /Type /Page /Parent 2 0 R /MediaBox [0 0 595 842] >>")
	xrefStreamOffset := len(data)
	record := func(kind, value, generation int) []byte {
		return []byte{byte(kind), byte(value >> 24), byte(value >> 16), byte(value >> 8), byte(value), byte(generation >> 8), byte(generation)}
	}
	raw := append(record(1, offsets[3], 0), record(1, xrefStreamOffset, 0)...)
	data = append(data, fmt.Sprintf("4 0 obj\n<< /Type /XRef /Size 6 /Root 1 0 R /W [1 4 2] /Index [3 2] /Length %d >>\nstream\n", len(raw))...)
	data = append(data, raw...)
	data = append(data, []byte("\nendstream\nendobj\n")...)
	offsetObject := len(data)
	data = append(data, []byte(fmt.Sprintf("5 0 obj\n%d\nendobj\n", xrefStreamOffset))...)
	xrefOffset := len(data)
	data = append(data, []byte("xref\n0 3\n0000000000 65535 f \n")...)
	for number := 1; number <= 2; number++ {
		data = append(data, []byte(fmt.Sprintf("%010d 00000 n \n", offsets[number]))...)
	}
	data = append(data, []byte(fmt.Sprintf("4 2\n%010d 00000 n \n%010d 00000 n \n", xrefStreamOffset, offsetObject))...)
	data = append(data, []byte(fmt.Sprintf("trailer\n<< /Size 6 /Root 1 0 R /XRefStm 5 0 R >>\nstartxref\n%d\n%%%%EOF\n", xrefOffset))...)

	d, err := OpenBytes(data)
	if err != nil {
		t.Fatal(err)
	}
	if d.WasRecovered() {
		t.Fatalf("indirect hybrid xref-stream unexpectedly used recovery: %v", d.RecoveryError())
	}
	page, err := d.PageAt(0)
	if err != nil {
		t.Fatal(err)
	}
	if got := page.MediaBox(d); got != [4]float64{0, 0, 595, 842} {
		t.Fatalf("indirect hybrid xref-stream page box = %v", got)
	}
	if _, err := d.ResolveRef(Ref{Object: 3}); err != nil {
		t.Fatalf("indirect hybrid xref-stream did not publish page entry: %v", err)
	}
}

func TestOpenBytesReadsObjectFromXRefStream(t *testing.T) {
	data := []byte("%PDF-1.5\n")
	offsets := make([]int, 5)
	appendObject := func(number int, body string) {
		offsets[number] = len(data)
		data = append(data, fmt.Sprintf("%d 0 obj\n%s\nendobj\n", number, body)...)
	}
	appendObject(1, "<< /Type /Catalog /Pages 2 0 R >>")
	appendObject(2, "<< /Type /Pages /Kids [3 0 R] /Count 1 >>")
	objectBody := "<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] >>"
	objectStreamData := "3 0 " + objectBody
	xrefOffset := len(data)
	offsets[4] = xrefOffset
	data = append(data, fmt.Sprintf("4 0 obj\n<< /Type /ObjStm /N 1 /First %d /Length %d >>\nstream\n%s\nendstream\nendobj\n", len("3 0 "), len(objectStreamData), objectStreamData)...)
	record := func(kind, value, generation int) []byte {
		return []byte{byte(kind), byte(value >> 24), byte(value >> 16), byte(value >> 8), byte(value), byte(generation >> 8), byte(generation)}
	}
	xrefStreamOffset := len(data)
	raw := append(record(0, 0, 65535), record(1, offsets[1], 0)...)
	raw = append(raw, record(1, offsets[2], 0)...)
	raw = append(raw, record(2, 4, 0)...)
	raw = append(raw, record(1, offsets[4], 0)...)
	raw = append(raw, record(1, xrefStreamOffset, 0)...)
	data = append(data, fmt.Sprintf("5 0 obj\n<< /Type /XRef /Size 6 /Root 1 0 R /W [1 4 2] /Length %d >>\nstream\n", len(raw))...)
	data = append(data, raw...)
	data = append(data, []byte(fmt.Sprintf("\nendstream\nendobj\nstartxref\n%d\n%%%%EOF\n", xrefStreamOffset))...)

	d, err := OpenBytes(data)
	if err != nil {
		t.Fatal(err)
	}
	page, err := d.PageAt(0)
	if err != nil {
		t.Fatal(err)
	}
	if got := page.MediaBox(d); got != [4]float64{0, 0, 612, 792} {
		t.Fatalf("object-stream page box = %v", got)
	}
}

func TestOpenBytesFallsBackToObjectScanWithoutXRef(t *testing.T) {
	data := []byte("%PDF-1.4\n" +
		"1 0 obj\n<< /Type /Catalog /Pages 2 0 R >>\nendobj\n" +
		"2 0 obj\n<< /Type /Pages /Kids [3 0 R] /Count 1 >>\nendobj\n" +
		"3 0 obj\n<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] >>\nendobj\n" +
		"garbage after objects\n")
	d, err := OpenBytes(data)
	if err != nil {
		t.Fatal(err)
	}
	page, err := d.PageAt(0)
	if err != nil {
		t.Fatal(err)
	}
	if got := page.MediaBox(d); got != [4]float64{0, 0, 612, 792} {
		t.Fatalf("scanned page box = %v", got)
	}
}

func TestOpenBytesRecoversCompressedObjectsDuringObjectScan(t *testing.T) {
	data := []byte("%PDF-1.5\n" +
		"1 0 obj\n<< /Type /Catalog /Pages 2 0 R >>\nendobj\n" +
		"2 0 obj\n<< /Type /Pages /Kids [3 0 R] /Count 1 >>\nendobj\n")
	pageBody := "<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] >>"
	objectStreamData := "3 0 " + pageBody
	data = append(data, []byte(fmt.Sprintf(
		"4 0 obj\n<< /Type /ObjStm /N 1 /First %d /Length %d >>\nstream\n%s\nendstream\nendobj\n",
		len("3 0 "), len(objectStreamData), objectStreamData))...)
	data = append(data, []byte("trailer\n<< /Size 5 /Root 1 0 R >>\nstartxref\nnot-an-offset\n%%EOF\n")...)

	d, err := OpenBytes(data)
	if err != nil {
		t.Fatal(err)
	}
	if !d.WasRecovered() {
		t.Fatal("object scan recovery was not reported")
	}
	page, err := d.PageAt(0)
	if err != nil {
		t.Fatal(err)
	}
	if got := page.MediaBox(d); got != [4]float64{0, 0, 612, 792} {
		t.Fatalf("recovered object-stream page box = %v", got)
	}
}

func TestOpenBytesRecoveryKeepsFirstDuplicateCompressedObject(t *testing.T) {
	data := []byte("%PDF-1.5\n" +
		"1 0 obj\n<< /Type /Catalog /Pages 2 0 R >>\nendobj\n" +
		"2 0 obj\n<< /Type /Pages /Kids [3 0 R] /Count 1 >>\nendobj\n")
	appendObjectStream := func(object int, box string) {
		body := "3 0 << /Type /Page /Parent 2 0 R /MediaBox " + box + " >>"
		data = append(data, []byte(fmt.Sprintf(
			"%d 0 obj\n<< /Type /ObjStm /N 1 /First 4 /Length %d >>\nstream\n%s\nendstream\nendobj\n",
			object, len(body), body))...)
	}
	appendObjectStream(4, "[0 0 612 792]")
	appendObjectStream(5, "[0 0 720 720]")
	data = append(data, []byte("trailer\n<< /Size 6 /Root 1 0 R >>\nstartxref\nnot-an-offset\n%%EOF\n")...)

	d, err := OpenBytes(data)
	if err != nil {
		t.Fatal(err)
	}
	page, err := d.PageAt(0)
	if err != nil {
		t.Fatal(err)
	}
	if got := page.MediaBox(d); got != [4]float64{0, 0, 612, 792} {
		t.Fatalf("duplicate compressed object selected non-first declaration: %v", got)
	}
}

func TestOpenBytesRecoveryKeepsFirstDuplicateObject(t *testing.T) {
	data := []byte("%PDF-1.4\n" +
		"1 0 obj\n<< /Type /Catalog /Pages 2 0 R >>\nendobj\n" +
		"2 0 obj\n<< /Type /Pages /Kids [3 0 R] /Count 1 >>\nendobj\n" +
		"3 0 obj\n<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] >>\nendobj\n" +
		"3 0 obj\n<< /Type /Page /Parent 2 0 R /MediaBox [0 0 720 720] >>\nendobj\n" +
		"trailer\n<< /Size 4 /Root 1 0 R >>\nstartxref\nnot-an-offset\n%%EOF\n")

	d, err := OpenBytes(data)
	if err != nil {
		t.Fatal(err)
	}
	page, err := d.PageAt(0)
	if err != nil {
		t.Fatal(err)
	}
	if got := page.MediaBox(d); got != [4]float64{0, 0, 612, 792} {
		t.Fatalf("duplicate object selected non-first declaration: %v", got)
	}
}

func TestOpenBytesReportsXRefRecoveryCause(t *testing.T) {
	data := []byte("%PDF-1.4\n" +
		"1 0 obj\n<< /Type /Catalog /Pages 2 0 R >>\nendobj\n" +
		"2 0 obj\n<< /Type /Pages /Kids [3 0 R] /Count 1 >>\nendobj\n" +
		"3 0 obj\n<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] >>\nendobj\n" +
		"trailer\n<< /Root 1 0 R >>\n%%EOF\n")
	d, err := OpenBytes(data)
	if err != nil {
		t.Fatal(err)
	}
	if !d.WasRecovered() {
		t.Fatal("object scan recovery was not reported")
	}
	if recoveryErr := d.RecoveryError(); recoveryErr == nil {
		t.Fatal("missing original xref recovery error")
	}
}

func TestOpenBytesWithPasswordReadsEncryptedPDF(t *testing.T) {
	password := "secret"
	owner := []byte("owner-entry-with-32-byte-padding----")
	id := []byte("file-id")
	permissions := -4
	key := encryptionKey([]byte(password), owner, permissions, id)
	user, err := userEntryRevision(key, id, 2)
	if err != nil {
		t.Fatal(err)
	}
	objects := []string{
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] >>",
		"<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>",
		fmt.Sprintf("<< /Filter /Standard /V 1 /R 2 /Length 40 /O <%x> /U <%x> /P %d >>", owner, user, permissions),
	}
	data := []byte("%PDF-1.3\n")
	offsets := make([]int, len(objects)+1)
	for number, object := range objects {
		number++
		offsets[number] = len(data)
		data = append(data, fmt.Sprintf("%d 0 obj\n%s\nendobj\n", number, object)...)
	}
	xref := len(data)
	data = append(data, []byte(fmt.Sprintf("xref\n0 %d\n0000000000 65535 f \n", len(offsets)))...)
	for _, offset := range offsets[1:] {
		data = append(data, []byte(fmt.Sprintf("%010d 00000 n \n", offset))...)
	}
	data = append(data, []byte(fmt.Sprintf("trailer\n<< /Size %d /Root 1 0 R /Encrypt 5 0 R /ID [(file-id)] >>\nstartxref\n%d\n%%%%EOF\n", len(offsets), xref))...)

	if _, err := OpenBytes(data); err != ErrPasswordRequired {
		t.Fatalf("missing password error = %v, want %v", err, ErrPasswordRequired)
	}
	if _, err := OpenBytes(data, WithPassword("wrong")); err != ErrInvalidPassword {
		t.Fatalf("wrong password error = %v, want %v", err, ErrInvalidPassword)
	}
	d, err := OpenBytes(data, WithPassword(password))
	if err != nil {
		t.Fatal(err)
	}
	page, err := d.PageAt(0)
	if err != nil {
		t.Fatal(err)
	}
	if got := page.MediaBox(d); got != [4]float64{0, 0, 612, 792} {
		t.Fatalf("encrypted page box = %v", got)
	}
}

func TestOpenBytesWithExplicitEmptyPassword(t *testing.T) {
	owner := []byte("owner-entry-with-32-byte-padding----")
	id := []byte("empty-password-file")
	permissions := -4
	key := encryptionKey(nil, owner, permissions, id)
	user, err := userEntryRevision(key, id, 2)
	if err != nil {
		t.Fatal(err)
	}
	objects := []string{
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] >>",
		fmt.Sprintf("<< /Filter /Standard /V 1 /R 2 /O <%x> /U <%x> /P %d >>", owner, user, permissions),
	}
	data := []byte("%PDF-1.3\n")
	offsets := make([]int, len(objects)+1)
	for number, object := range objects {
		number++
		offsets[number] = len(data)
		data = append(data, fmt.Sprintf("%d 0 obj\n%s\nendobj\n", number, object)...)
	}
	xref := len(data)
	data = append(data, []byte(fmt.Sprintf("xref\n0 %d\n0000000000 65535 f \n", len(offsets)))...)
	for _, offset := range offsets[1:] {
		data = append(data, []byte(fmt.Sprintf("%010d 00000 n \n", offset))...)
	}
	data = append(data, []byte(fmt.Sprintf("trailer\n<< /Size %d /Root 1 0 R /Encrypt 4 0 R /ID [(empty-password-file)] >>\nstartxref\n%d\n%%%%EOF\n", len(offsets), xref))...)

	if _, err := OpenBytes(data); err != ErrPasswordRequired {
		t.Fatalf("unset password error = %v, want %v", err, ErrPasswordRequired)
	}
	if _, err := OpenBytes(data, WithPassword("")); err != nil {
		t.Fatalf("explicit empty password = %v", err)
	}
}
