package fontdata_test

import (
	"testing"

	"github.com/lin-string/go-playa/fontdata"
)

func TestParseType1ProgramOwnsCharStringsAndSubrs(t *testing.T) {
	data := []byte("/lenIV -1 def /Subrs 1 array dup 0 1 RD ")
	data = append(data, 14)
	data = append(data, []byte(" ND /CharStrings 1 dict begin /A 1 RD ")...)
	data = append(data, 14)
	data = append(data, []byte(" ND end")...)

	program := fontdata.ParseType1Program(data)
	if program == nil {
		t.Fatal("ParseType1Program returned nil")
	}
	if got := program.CharString("A"); len(got) != 1 || got[0] != 14 {
		t.Fatalf("CharString(A) = %v", got)
	}
	subrs := program.SubrsCopy()
	if len(subrs) != 1 || len(subrs[0]) != 1 || subrs[0][0] != 14 {
		t.Fatalf("SubrsCopy() = %v", subrs)
	}

	charstring := program.CharString("A")
	charstring[0] = 0
	subrs[0][0] = 0
	if program.CharString("A")[0] != 14 || program.SubrsCopy()[0][0] != 14 {
		t.Fatal("Type1Program leaked mutable storage")
	}
}
