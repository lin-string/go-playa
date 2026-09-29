//go:build !race

package document

import "testing"

func skipAllocationCheckUnderRace(t *testing.T) {
	t.Helper()
}
