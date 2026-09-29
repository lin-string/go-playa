// Package coordinates owns the document coordinate-space value model.
package coordinates

// Space identifies the device-space convention used for page geometry.
type Space string

const (
	Page    Space = "page"
	Screen  Space = "screen"
	Default Space = "default"
	User    Space = "user"
)
