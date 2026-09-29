package cacheconfig_test

import (
	"testing"

	"github.com/lin-string/go-playa/cacheconfig"
)

func TestPublicCacheOptionsZeroValueDisablesRetention(t *testing.T) {
	var options cacheconfig.Options
	if options.ObjectBytes != 0 || options.DecodedStreamBytes != 0 || options.PageBytes != 0 {
		t.Fatalf("zero cache options retained data: %#v", options)
	}
}
