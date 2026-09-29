package coordinates_test

import (
	"testing"

	"github.com/lin-string/go-playa/coordinates"
)

func TestPublicCoordinateSpacesAreStableValues(t *testing.T) {
	spaces := []coordinates.Space{coordinates.Page, coordinates.Screen, coordinates.Default, coordinates.User}
	want := []string{"page", "screen", "default", "user"}
	for index, space := range spaces {
		if string(space) != want[index] {
			t.Fatalf("space[%d] = %q, want %q", index, space, want[index])
		}
	}
}
