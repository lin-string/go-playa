package contentdata

import (
	"fmt"
	"strings"
)

// DashPattern is the PDF stroke dash pattern and phase.
//
// It mirrors Playa's DashPattern value while keeping its slice private. Use
// ValuesCopy or Finalize when an owned snapshot is required.
type DashPattern struct {
	dash  []float64
	phase float64
}

// NewDashPattern constructs an owned dash pattern.
func NewDashPattern(dash []float64, phase float64) DashPattern {
	return DashPattern{dash: cloneFloats(dash), phase: phase}
}

// ValuesCopy returns the dash lengths in order.
func (d DashPattern) ValuesCopy() []float64 { return cloneFloats(d.dash) }

// Phase returns the starting dash phase.
func (d DashPattern) Phase() float64 { return d.phase }

// Len returns the number of dash and gap lengths.
func (d DashPattern) Len() int { return len(d.dash) }

// At returns one dash or gap length by index.
func (d DashPattern) At(index int) (float64, bool) {
	if index < 0 || index >= len(d.dash) {
		return 0, false
	}
	return d.dash[index], true
}

// Finalize returns an independent dash-pattern snapshot.
func (d DashPattern) Finalize() DashPattern {
	d.dash = cloneFloats(d.dash)
	return d
}

// String follows Playa's compact dash-pattern display.
func (d DashPattern) String() string {
	if len(d.dash) == 0 {
		return ""
	}
	values := make([]string, len(d.dash))
	for index, value := range d.dash {
		values[index] = fmt.Sprint(value)
	}
	return "(" + strings.Join(values, ", ") + ") " + fmt.Sprint(d.phase)
}
