package fontdata

import (
	"math"
	"testing"

	pdftypes "github.com/lin-string/go-playa/pdftypes/primitives"
)

func TestParseWidthAcceptsOnlyFiniteNumbers(t *testing.T) {
	if got, err := ParseWidth(pdftypes.Number(600)); err != nil || got != 600 {
		t.Fatalf("finite width = %v, %v", got, err)
	}
	for _, value := range []pdftypes.Object{pdftypes.Name("600"), pdftypes.Number(math.Inf(1)), pdftypes.Number(math.NaN())} {
		if _, err := ParseWidth(value); err == nil {
			t.Fatalf("ParseWidth(%#v) accepted malformed width", value)
		}
	}
}
