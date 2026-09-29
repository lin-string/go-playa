package fontdata_test

import (
	"testing"

	"github.com/lin-string/go-playa/fontdata"
)

func TestParseCFFDictReturnsImmutableOperatorRecords(t *testing.T) {
	data := []byte{139, 140, 12, 9, 141, 142, 5}
	entries, ok := fontdata.ParseCFFDict(data, false)
	if !ok || len(entries) != 2 {
		t.Fatalf("CFF DICT entries = %#v, ok=%v", entries, ok)
	}
	if entries[0].Operator() != 1209 || entries[1].Operator() != 5 {
		t.Fatalf("CFF DICT operators = %d, %d", entries[0].Operator(), entries[1].Operator())
	}
	operands := entries[0].OperandsCopy()
	if len(operands) != 2 || operands[0] != 0 || operands[1] != 1 {
		t.Fatalf("CFF DICT operands = %#v", operands)
	}
	operands[0] = 99
	if got := entries[0].OperandsCopy()[0]; got != 0 {
		t.Fatalf("CFF DICT operands were not copied: %v", got)
	}
}

func TestParseCFFDictReturnsCompletedRecordsBeforeMalformedTail(t *testing.T) {
	entries, ok := fontdata.ParseCFFDict([]byte{139, 5, 28, 0}, false)
	if ok || len(entries) != 1 || entries[0].Operator() != 5 {
		t.Fatalf("malformed CFF DICT = %#v, ok=%v", entries, ok)
	}
}
