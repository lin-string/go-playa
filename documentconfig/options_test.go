package documentconfig_test

import (
	"testing"

	"github.com/lin-string/go-playa/documentconfig"
)

func TestPublicDocumentOptionsDistinguishExplicitPassword(t *testing.T) {
	var option documentconfig.OpenOption = func(options *documentconfig.Options) {
		options.PasswordSet = true
	}
	options := documentconfig.Options{}
	option(&options)
	if !options.PasswordSet {
		t.Fatal("OpenOption did not update public document options")
	}
}
