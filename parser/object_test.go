package parser

import (
	"errors"
	"reflect"
	"testing"
)

func TestObjectParserBuildsIndirectReferences(t *testing.T) {
	value, err := ParseObject([]byte("[1 0 R << /Value (ok) >>]"))
	if err != nil {
		t.Fatal(err)
	}
	array, ok := value.(Array)
	if !ok || len(array) != 2 {
		t.Fatalf("parsed object = %#v", value)
	}
	ref, ok := array[0].(Ref)
	if !ok || ref.Object != 1 || ref.Generation != 0 {
		t.Fatalf("reference = %#v", array[0])
	}
}

func TestObjectParserWrapsMalformedContainers(t *testing.T) {
	_, err := ParseObject([]byte("<< /Value (unterminated"))
	var parseErr *ParseError
	if !errors.As(err, &parseErr) || parseErr.Operation() != "lex" {
		t.Fatalf("error = %v, want lexical ParseError", err)
	}
}

func TestObjectParserParsesProcedureContainers(t *testing.T) {
	value, err := ParseObject([]byte("{1 null {2}}"))
	if err != nil {
		t.Fatal(err)
	}
	want := Array{Number(1), Null{}, Array{Number(2)}}
	if !reflect.DeepEqual(value, want) {
		t.Fatalf("procedure = %#v, want %#v", value, want)
	}
}
