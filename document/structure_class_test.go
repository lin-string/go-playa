package document

import "testing"

func TestStructureClassNameOptionalRevisions(t *testing.T) {
	for _, tc := range []struct {
		name  string
		value Object
		want  string
	}{
		{"single", Name("first"), "first"},
		{"single-array", Array{Name("first")}, "first"},
		{"unversioned", Array{Name("first"), Name("second")}, "first"},
		{"explicit-revisions", Array{Name("old"), Number(1), Name("new"), Number(2)}, "new"},
		{"mixed", Array{Name("first"), Name("new"), Number(2), Name("last")}, "new"},
		{"revision-tie", Array{Name("first"), Number(2), Name("second"), Number(2)}, "first"},
		{"indirect", Array{Ref{Object: 4}, Ref{Object: 5}, Ref{Object: 6}}, "new"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d := &Document{
				trailer: Dict{Name("Root"): Ref{Object: 1}},
				objects: map[Ref]Object{
					{Object: 1}: Dict{Name("StructTreeRoot"): Ref{Object: 2}},
					{Object: 2}: Dict{Name("K"): Ref{Object: 3}},
					{Object: 3}: Dict{Name("S"): Name("P"), Name("C"): tc.value},
					{Object: 4}: Name("new"),
					{Object: 5}: Number(3),
					{Object: 6}: Name("last"),
				},
			}
			seq := d.StructureTreeSeq()
			for attempt := 0; attempt < 2; attempt++ {
				count := 0
				for element, err := range seq {
					if err != nil {
						t.Fatal(err)
					}
					count++
					if got := element.ClassName(); got != tc.want {
						t.Fatalf("ClassName = %q, want %q", got, tc.want)
					}
				}
				if count != 1 {
					t.Fatalf("structure elements = %d, want 1", count)
				}
			}
		})
	}
}
