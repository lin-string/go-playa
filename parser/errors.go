// Package parser owns dependency-free PDF parse diagnostics.
package parser

import (
	"errors"
	"fmt"

	pdftypes "github.com/lin-string/go-playa/pdftypes/primitives"
)

// ParseError describes a malformed or incomplete PDF input location.
// Its diagnostic state is immutable after construction; use the accessors to
// inspect the location and wrapped cause.
type ParseError struct {
	offset    int
	objectRef pdftypes.Ref
	hasObject bool
	operation string
	err       error
}

// Offset reports the byte offset associated with the parse failure.
func (e *ParseError) Offset() int {
	return e.offset
}

// ObjectRef reports the indirect object being resolved, when available.
func (e *ParseError) ObjectRef() pdftypes.Ref {
	return e.objectRef
}

// HasObject reports whether ObjectRef identifies the failing object.
func (e *ParseError) HasObject() bool { return e.hasObject }

// Operation reports the parser operation that detected the failure.
func (e *ParseError) Operation() string {
	return e.operation
}

// Cause returns the wrapped low-level error without exposing mutable parser
// state.
func (e *ParseError) Cause() error {
	return e.err
}

// NewParseError constructs a parse diagnostic for parser integrations.
func NewParseError(err error, offset int, operation string) *ParseError {
	return &ParseError{offset: offset, operation: operation, err: err}
}

// NewParseErrorWithObject constructs a parse diagnostic tied to an indirect
// object reference.
func NewParseErrorWithObject(err error, ref pdftypes.Ref, offset int, operation string) *ParseError {
	return &ParseError{offset: offset, operation: operation, objectRef: ref, hasObject: true, err: err}
}

func (e *ParseError) Error() string {
	prefix := "playa: parse error"
	if e.operation != "" {
		prefix += " during " + e.operation
	}
	prefix += fmt.Sprintf(" at offset %d", e.offset)
	if e.hasObject {
		prefix += " for object " + e.objectRef.String()
	}
	if e.err != nil {
		return prefix + ": " + e.err.Error()
	}
	return prefix
}

func (e *ParseError) Unwrap() error { return e.err }

func Wrap(err error, offset int, operation string) error {
	if err == nil {
		return nil
	}
	var parseErr *ParseError
	if errors.As(err, &parseErr) {
		return err
	}
	return NewParseError(err, offset, operation)
}

func WithContext(err error, offset int, operation string) error {
	if err == nil {
		return nil
	}
	return NewParseError(err, offset, operation)
}

func WithObjectContext(err error, ref pdftypes.Ref, offset int, operation string) error {
	if err == nil {
		return nil
	}
	var parseErr *ParseError
	if errors.As(err, &parseErr) {
		if parseErr.hasObject {
			return err
		}
		copy := *parseErr
		copy.objectRef, copy.hasObject = ref, true
		return &copy
	}
	return NewParseErrorWithObject(err, ref, offset, operation)
}
