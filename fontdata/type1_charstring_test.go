package fontdata_test

import (
	"testing"

	"github.com/lin-string/go-playa/fontdata"
)

func TestParseType1CharStringExposesPathOperations(t *testing.T) {
	ops, ok := fontdata.ParseType1CharString(
		[]byte{139, 139, 21, 239, 139, 5, 139, 239, 5, 14},
		nil,
	)
	if !ok || len(ops) != 5 {
		t.Fatalf("ParseType1CharString() = %#v, %v", ops, ok)
	}
	if ops[0].Operator() != "m" || len(ops[0].OperandsCopy()) != 2 {
		t.Fatalf("first operation = %#v", ops[0])
	}
	operands := ops[0].OperandsCopy()
	operands[0] = 999
	if got := ops[0].OperandsCopy()[0]; got != 0 {
		t.Fatalf("OperandsCopy leaked mutable storage: %v", got)
	}
	if ops[1].Operator() != "l" || ops[2].Operator() != "l" || ops[3].Operator() != "h" || ops[4].Operator() != "f" {
		t.Fatalf("operations = %#v", ops)
	}
}

func TestParseType1CharStringRejectsInvalidProgram(t *testing.T) {
	if _, ok := fontdata.ParseType1CharString([]byte{139, 12, 6}, nil); ok {
		t.Fatal("invalid escaped operator was accepted")
	}
}
