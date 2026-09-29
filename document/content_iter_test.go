package document

import "testing"

func TestContentStreamsBorrowsArrayStorage(t *testing.T) {
	source := Array{Stream{}, Stream{}}
	streams := contentStreams(source)
	if len(streams) != len(source) || &streams[0] != &source[0] {
		t.Fatal("content stream array was copied instead of borrowed")
	}
}

func TestResolvedContentStreamsPreservesIndirectStreamReference(t *testing.T) {
	ref := Ref{Object: 7}
	stream := Stream{}
	streams, err := resolvedContentStreams(ref, stream)
	if err != nil {
		t.Fatal(err)
	}
	if len(streams) != 1 || streams[0] != ref {
		t.Fatalf("indirect content stream = %#v, want %v", streams, ref)
	}
}

func TestResolvedContentStreamsBorrowsArrayStorage(t *testing.T) {
	source := Array{Stream{}, Stream{}}
	streams, err := resolvedContentStreams(nil, source)
	if err != nil {
		t.Fatal(err)
	}
	if len(streams) != len(source) || &streams[0] != &source[0] {
		t.Fatal("resolved content stream array was copied instead of borrowed")
	}
}
