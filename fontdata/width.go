package fontdata

import (
	"fmt"
	"math"

	pdftypes "github.com/lin-string/go-playa/pdftypes/primitives"
)

// ParseWidth validates a PDF font-width value without depending on the
// document interpreter. Non-finite numbers are rejected because they cannot
// participate in font metrics or layout calculations.
func ParseWidth(value pdftypes.Object) (float64, error) {
	number, ok := value.(pdftypes.Number)
	if !ok || math.IsNaN(float64(number)) || math.IsInf(float64(number), 0) {
		return 0, fmt.Errorf("playa: width is not number")
	}
	return float64(number), nil
}
