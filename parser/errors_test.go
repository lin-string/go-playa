package parser

import (
	"errors"
	"testing"

	pdftypes "github.com/lin-string/go-playa/pdftypes/primitives"
)

func TestParseErrorContextPreservesObjectReference(t *testing.T) {
	err := WithObjectContext(errors.New("bad object"), pdftypes.Ref{Object: 7, Generation: 2}, 19, "object")
	var parseErr *ParseError
	if !errors.As(err, &parseErr) || !parseErr.HasObject() {
		t.Fatalf("error = %v, want ParseError with object context", err)
	}
	if got := err.Error(); got != "playa: parse error during object at offset 19 for object 7 2 R: bad object" {
		t.Fatalf("ParseError.Error() = %q", got)
	}
}

func TestParseErrorReceiverIsRequired(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("nil ParseError receiver did not trigger a programmer error")
		}
	}()
	_ = (*ParseError)(nil).Offset()
}
