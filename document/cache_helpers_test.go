package document

import (
	"math"
	"testing"
)

func TestCacheFitsRejectsOverflowAndInvalidCounters(t *testing.T) {
	tests := []struct {
		name                     string
		current, addition, limit int
		want                     bool
	}{
		{name: "within budget", current: 4, addition: 6, limit: 10, want: true},
		{name: "at budget", current: 4, addition: 6, limit: 10, want: true},
		{name: "over budget", current: 5, addition: 6, limit: 10, want: false},
		{name: "counter overflow", current: math.MaxInt - 1, addition: 2, limit: math.MaxInt, want: false},
		{name: "negative current", current: -1, addition: 1, limit: 10, want: false},
		{name: "negative addition", current: 1, addition: -1, limit: 10, want: false},
		{name: "negative limit", current: 0, addition: 0, limit: -1, want: false},
		{name: "inconsistent current", current: 11, addition: 0, limit: 10, want: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := cacheFits(test.current, test.addition, test.limit); got != test.want {
				t.Fatalf("cacheFits(%d, %d, %d) = %v, want %v", test.current, test.addition, test.limit, got, test.want)
			}
		})
	}
}

func TestAddCacheSizeSaturatesOnOverflow(t *testing.T) {
	if got := addCacheSize(math.MaxInt-1, 2); got != math.MaxInt {
		t.Fatalf("addCacheSize overflow = %d, want %d", got, math.MaxInt)
	}
	if got := addCacheSize(1, -1); got != math.MaxInt {
		t.Fatalf("addCacheSize negative addition = %d, want %d", got, math.MaxInt)
	}
}

func TestCacheMulSizeSaturatesOnOverflow(t *testing.T) {
	if got := cacheMulSize(math.MaxInt, 2); got != math.MaxInt {
		t.Fatalf("cacheMulSize overflow = %d, want %d", got, math.MaxInt)
	}
	if got := cacheMulSize(3, 4); got != 12 {
		t.Fatalf("cacheMulSize(3, 4) = %d, want 12", got)
	}
}
