package document

import (
	"math"
	"testing"

	"github.com/lin-string/go-playa/geometry"
)

func TestExpandOpsResolvesFormMatrixNumbers(t *testing.T) {
	d := &Document{objects: map[Ref]Object{}}
	d.objects[Ref{Object: 1}] = newStream(Dict{
		Name("Subtype"):   Name("Form"),
		Name("Matrix"):    Array{Ref{Object: 2}, Number(0), Number(0), Ref{Object: 3}, Number(10), Number(20)},
		Name("Resources"): Dict{},
	}, []byte("q Q"))
	d.objects[Ref{Object: 2}] = Number(1)
	d.objects[Ref{Object: 3}] = Number(1)
	ops := []ContentOp{newContentOpBorrowed("Do", []Object{Name("Fm")}, 0)}
	ctx := Dict{Name("Resources"): Dict{Name("XObject"): Dict{Name("Fm"): Ref{Object: 1}}}}
	got := d.expandOps(ops, ctx, 0, "")
	if len(got) < 2 || !hasMatrixOp(got, Number(1), Number(1)) {
		t.Fatalf("expanded form ops = %#v", got)
	}
}

func TestResolveFormMatrixFollowsMultiLevelIndirectValues(t *testing.T) {
	d := &Document{objects: map[Ref]Object{
		{Object: 1}: Ref{Object: 2},
		{Object: 2}: Array{Ref{Object: 3}, Number(0), Number(0), Ref{Object: 4}, Number(10), Ref{Object: 5}},
		{Object: 3}: Ref{Object: 6},
		{Object: 4}: Number(2),
		{Object: 5}: Ref{Object: 7},
		{Object: 6}: Number(1),
		{Object: 7}: Number(20),
	}}
	want := geometry.Matrix{1, 0, 0, 2, 10, 20}
	if got := resolveFormMatrix(d, Ref{Object: 1}); got != want {
		t.Fatalf("indirect form matrix = %v, want %v", got, want)
	}
	if !validFormMatrix(d, Ref{Object: 1}) {
		t.Fatal("multi-level indirect form matrix was rejected")
	}
}

func TestResolveFormMatrixRejectsMalformedValues(t *testing.T) {
	d := &Document{}
	got := resolveFormMatrix(d, Array{Number(1), String("invalid"), Number(0), Number(1), Number(0), Number(0)})
	if got != identity() {
		t.Fatalf("malformed form matrix = %v, want identity", got)
	}
}

func TestResolveFormMatrixRejectsExtraValues(t *testing.T) {
	d := &Document{}
	got := resolveFormMatrix(d, Array{Number(2), Number(0), Number(0), Number(2), Number(10), Number(20), Number(30)})
	if got != identity() {
		t.Fatalf("form matrix with extra values = %v, want identity", got)
	}
}

func TestResolveFormMatrixRejectsNonFiniteValues(t *testing.T) {
	for _, value := range []Number{Number(math.NaN()), Number(math.Inf(1))} {
		got := resolveFormMatrix(&Document{}, Array{value, Number(0), Number(0), Number(1), Number(0), Number(0)})
		if got != identity() {
			t.Fatalf("form matrix with non-finite value %v = %v, want identity", value, got)
		}
	}
}

func TestExpandOpsIgnoresMalformedFormMatrix(t *testing.T) {
	d := &Document{objects: map[Ref]Object{
		{Object: 1}: newStream(Dict{
			Name("Subtype"):   Name("Form"),
			Name("Matrix"):    Array{Number(1), Number(0), Number(0), Number(1), Number(0), Number(0), Number(20)},
			Name("Resources"): Dict{},
		}, []byte("q Q")),
	}}
	ops := d.expandOps(
		[]ContentOp{newContentOpBorrowed("Do", []Object{Name("Fm")}, 0)},
		Dict{Name("Resources"): Dict{Name("XObject"): Dict{Name("Fm"): Ref{Object: 1}}}},
		0,
		"",
	)
	if hasMatrixOp(ops, Number(1), Number(1)) {
		t.Fatalf("malformed form matrix was emitted as cm: %#v", ops)
	}
}

func hasMatrixOp(ops []ContentOp, a, d Object) bool {
	for _, op := range ops {
		if op.operatorValue() == "cm" && len(op.operandsValue()) >= 4 && op.operandsValue()[0] == a && op.operandsValue()[3] == d {
			return true
		}
	}
	return false
}
