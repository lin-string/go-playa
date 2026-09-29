package fontdata

import (
	"math"
	"testing"
)

func TestParseCFFIntegerDomains(t *testing.T) {
	if got, ok := ParseCFFInteger(7); !ok || got != 7 {
		t.Fatalf("ParseCFFInteger(7) = %d, %v", got, ok)
	}
	if _, ok := ParseCFFInteger(math.MaxFloat64); ok {
		t.Fatal("ParseCFFInteger accepted an overflowing value")
	}
	if _, ok := ParseCFFNonNegativeInt(-1); ok {
		t.Fatal("ParseCFFNonNegativeInt accepted a negative value")
	}
}

func TestCFFSubroutineBiasAndIndex(t *testing.T) {
	if CFFSubroutineBias(1) != 107 || CFFSubroutineBias(1240) != 1131 || CFFSubroutineBias(33900) != 32768 {
		t.Fatalf("CFF subroutine bias boundaries are incorrect")
	}
	if index, ok := ParseCFFSubroutineIndex(-107, 1); !ok || index != 0 {
		t.Fatalf("CFF subroutine index = %d, %v", index, ok)
	}
}
