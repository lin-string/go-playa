package contentconfig_test

import (
	"testing"

	"github.com/lin-string/go-playa/contentconfig"
)

func TestPublicContentOptionsDefaultToAllKinds(t *testing.T) {
	if got := contentconfig.DefaultOptions().Filter; got != contentconfig.FilterAll {
		t.Fatalf("default content filter = %v, want FilterAll", got)
	}
}
