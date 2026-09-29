package contentdata

import (
	"encoding/json"
	"iter"

	"github.com/lin-string/go-playa/pdftypes/primitives"
)

// ContentOp is the dependency-free value portion of one PDF content-stream
// operation. Resource lookup and interpreter-only context remain document-owned.
type ContentOp struct {
	operator string
	operands []primitives.Object
	offset   int
}

// NewContentOp constructs an owned content operation.
func NewContentOp(operator string, operands []primitives.Object, offset int) ContentOp {
	return ContentOp{operator: operator, operands: cloneObjects(operands), offset: offset}
}

// NewContentOpBorrowed constructs a low-allocation operation over operands
// owned by the caller. Use OperandsCopy or Finalize before retaining it past
// the source iteration.
func NewContentOpBorrowed(operator string, operands []primitives.Object, offset int) ContentOp {
	return ContentOp{operator: operator, operands: operands, offset: offset}
}

// Operator returns the PDF content operator name.
func (o ContentOp) Operator() string { return o.operator }

// Offset returns the byte offset of the operator in its source stream.
func (o ContentOp) Offset() int { return o.offset }

// OperandsBorrowed returns the underlying operand slice for low-allocation
// internal adapters. The slice is valid only for the source value lifetime.
func (o ContentOp) OperandsBorrowed() []primitives.Object { return o.operands }

// OperandsCopy returns independent operands for this content operation.
func (o ContentOp) OperandsCopy() []primitives.Object { return cloneObjects(o.operands) }

// OperandsSeq lazily yields borrowed operands in source order.
func (o ContentOp) OperandsSeq() iter.Seq[primitives.Object] {
	return func(yield func(primitives.Object) bool) {
		for _, operand := range o.operands {
			if !yield(operand) {
				return
			}
		}
	}
}

// Finalize returns an independent snapshot of the content operation.
func (o ContentOp) Finalize() ContentOp {
	o.operands = cloneObjects(o.operands)
	return o
}

func (o ContentOp) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Operator string
		Offset   int
		Operands []primitives.Object `json:"Operands"`
	}{o.operator, o.offset, o.operands})
}

func cloneObjects(values []primitives.Object) []primitives.Object {
	if values == nil {
		return nil
	}
	out := make([]primitives.Object, len(values))
	for i, value := range values {
		out[i] = cloneObject(value)
	}
	return out
}
