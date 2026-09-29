package document

import "testing"

func TestObjectStreamRejectsNegativeOffsets(t *testing.T) {
	d := &Document{data: []byte("\n10 0 obj\n<< /Type /ObjStm /N 1 /First 4 /Length 8 >>\nstream\n1 -1\n<<>>\nendstream\nendobj"), objects: map[Ref]Object{}, expandedObjStms: map[int]bool{}}
	err := d.loadObjectStream(map[Ref]xrefEntry{
		{Object: 10}: {offset: 1},
		{Object: 1}:  newCompressedXRefEntry(10, 0, false, true),
	}, 10)
	if err == nil {
		t.Fatal("negative object stream offset was accepted")
	}
}

func TestObjectStreamRejectsEqualOffsets(t *testing.T) {
	d := &Document{data: []byte("\n10 0 obj\n<< /Type /ObjStm /N 2 /First 8 /Length 12 >>\nstream\n1 0 2 0\n<<>><<>>\nendstream\nendobj"), objects: map[Ref]Object{}, expandedObjStms: map[int]bool{}}
	if err := d.loadObjectStream(map[Ref]xrefEntry{
		{Object: 10}: {offset: 1},
		{Object: 1}:  newCompressedXRefEntry(10, 0, false, true),
		{Object: 2}:  newCompressedXRefEntry(10, 0, false, true),
	}, 10); err == nil {
		t.Fatal("equal object stream offsets were accepted")
	}
}

func TestObjectStreamRejectsDuplicateObjectNumbers(t *testing.T) {
	d := &Document{data: []byte("\n10 0 obj\n<< /Type /ObjStm /N 2 /First 8 /Length 12 >>\nstream\n1 0 1 2\n<<>><<>>\nendstream\nendobj"), objects: map[Ref]Object{}, expandedObjStms: map[int]bool{}}
	if err := d.loadObjectStream(map[Ref]xrefEntry{
		{Object: 10}: {offset: 1},
		{Object: 1}:  newCompressedXRefEntry(10, 0, false, true),
	}, 10); err == nil {
		t.Fatal("duplicate object stream object numbers were accepted")
	}
}

func TestObjectStreamValidatesUnreferencedOffsets(t *testing.T) {
	d := &Document{data: []byte("\n10 0 obj\n<< /Type /ObjStm /N 1 /First 5 /Length 9 >>\nstream\n1 99\n<<>>\nendstream\nendobj"), objects: map[Ref]Object{}, expandedObjStms: map[int]bool{}}
	if err := d.loadObjectStream(map[Ref]xrefEntry{{Object: 10}: {offset: 1}}, 10); err == nil {
		t.Fatal("invalid unreferenced object-stream offset was accepted")
	}
}

func TestObjectStreamRejectsXRefIndexMismatch(t *testing.T) {
	d := &Document{data: []byte("\n10 0 obj\n<< /Type /ObjStm /N 2 /First 8 /Length 12 >>\nstream\n1 0 2 2\n<<>><<>>\nendstream\nendobj"), objects: map[Ref]Object{}, expandedObjStms: map[int]bool{}}
	if err := d.loadObjectStream(map[Ref]xrefEntry{
		{Object: 10}: {offset: 1},
		{Object: 1}:  newCompressedXRefEntry(10, 1, false, true),
		{Object: 2}:  newCompressedXRefEntry(10, 1, false, true),
	}, 10); err == nil {
		t.Fatal("object stream xref index mismatch was accepted")
	}
}
