package parserconfig_test

import (
	"testing"

	"github.com/lin-string/go-playa/parserconfig"
)

func TestPublicTokenKindsRemainStable(t *testing.T) {
	if parserconfig.TokenEOF != 0 || parserconfig.TokenKeyword != 10 {
		t.Fatalf("token kinds changed: eof=%d keyword=%d", parserconfig.TokenEOF, parserconfig.TokenKeyword)
	}
}
