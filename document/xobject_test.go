package document

import "testing"

func TestExpandOpsReusesContentWithoutForms(t *testing.T) {
	d := &Document{}
	ops := []ContentOp{newContentOpBorrowed("BT", nil, 0), newContentOpBorrowed("ET", nil, 0)}
	got := d.expandOps(ops, Dict{}, 0, "")
	if len(got) != len(ops) || &got[0] != &ops[0] {
		t.Fatal("content without Form XObjects was copied")
	}
}

func TestExpandOpsSkipsCircularFormInvocation(t *testing.T) {
	ref := Ref{Object: 7}
	form := newStream(Dict{
		Name("Subtype"): Name("Form"),
		Name("Resources"): Dict{Name("XObject"): Dict{
			Name("A"): ref,
		}},
	}, []byte("/A Do"))
	d := &Document{objects: map[Ref]Object{ref: form}}
	ctx := Dict{Name("Resources"): Dict{Name("XObject"): Dict{Name("A"): ref}}}
	ops := []ContentOp{newContentOpBorrowed("Do", []Object{Name("A")}, 0)}
	got := d.expandOps(ops, ctx, 0, "")
	for _, op := range got {
		if op.operatorValue() == "Do" {
			t.Fatalf("circular Form invocation survived flattening: %#v", got)
		}
	}
}

func TestFormParentKeyAcceptsStructParents(t *testing.T) {
	key, ok := formParentKey(&Document{}, newStream(Dict{
		Name("StructParents"): Number(9),
	}, nil))
	if !ok || key != 9 {
		t.Fatalf("formParentKey = (%d, %v), want (9, true)", key, ok)
	}
}
