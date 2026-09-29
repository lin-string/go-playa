// Package primitives owns dependency-free PDF primitive values.
package primitives

import (
	"math"
	"strconv"
)

// CloneBytes returns an independent copy of borrowed PDF byte storage.
func CloneBytes(value []byte) []byte {
	if value == nil {
		return nil
	}
	return append(make([]byte, 0, len(value)), value...)
}

// Object is the common set of values that can occur in a PDF object graph.
// The marker is exported so domain packages can own concrete PDF values while
// retaining a distinct type from arbitrary Go values.
type Object interface{ PDFObject() }

type Null struct{}

func (Null) PDFObject() {}

type Bool bool

func (Bool) PDFObject() {}

type Number float64

func (Number) PDFObject() {}

type Name string

func (Name) PDFObject() {}

type String []byte

func (String) PDFObject() {}

type Array []Object

func (Array) PDFObject() {}

type InvalidArray Array

func (InvalidArray) PDFObject() {}

type Dict map[Name]Object

func (Dict) PDFObject() {}

// Ref identifies an indirect PDF object.
type Ref struct {
	Object     int
	Generation int
}

func (Ref) PDFObject() {}

func (r Ref) String() string {
	return strconv.Itoa(r.Object) + " " + strconv.Itoa(r.Generation) + " R"
}

type Keyword string

func (Keyword) PDFObject() {}

func NumberValue(value Object) (float64, bool) {
	number, ok := value.(Number)
	return float64(number), ok
}

func FiniteNumberValue(value Object) (float64, bool) {
	number, ok := NumberValue(value)
	return number, ok && !math.IsNaN(number) && !math.IsInf(number, 0)
}

func IntValue(value Object) (int, bool) {
	number, ok := NumberValue(value)
	if !ok || math.IsNaN(number) || math.IsInf(number, 0) || number != math.Trunc(number) {
		return 0, false
	}
	maxInt := int(^uint(0) >> 1)
	minInt := -maxInt - 1
	if number < float64(minInt) || number >= float64(maxInt) {
		return 0, false
	}
	return int(number), true
}

func NameValue(value Object) (string, bool) {
	name, ok := value.(Name)
	return string(name), ok
}

func RefValue(value Object) (Ref, bool) {
	ref, ok := value.(Ref)
	return ref, ok
}
