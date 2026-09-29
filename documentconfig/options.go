// Package documentconfig owns dependency-free document-open configuration.
package documentconfig

import (
	"github.com/lin-string/go-playa/cacheconfig"
	"github.com/lin-string/go-playa/coordinates"
)

// Options controls document-wide interpretation choices.
type Options struct {
	Space    coordinates.Space
	Password string
	// PasswordSet distinguishes an explicitly supplied empty password from
	// an omitted password.
	PasswordSet bool
	Cache       cacheconfig.Options
	CacheSet    bool
}

// OpenOption configures a document opened through Open or OpenBytes.
type OpenOption func(*Options)
