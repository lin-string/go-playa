package fontdata

import "github.com/lin-string/go-playa/pdftypes/primitives"

// DecodedGlyph keeps source code bytes alongside their decoded text and CID.
// The code bytes may be borrowed while a decoding sequence is advancing; call
// Finalize before retaining the value beyond that sequence step.
type DecodedGlyph struct {
	text string
	code []byte
	cid  int
}

// NewDecodedGlyph creates an owned decoded-glyph value.
func NewDecodedGlyph(text string, code []byte, cid int) DecodedGlyph {
	return BorrowedDecodedGlyph(text, code, cid).Finalize()
}

// BorrowedDecodedGlyph creates a decoded-glyph view over code. The caller
// must call Finalize before retaining the result after the current iteration.
func BorrowedDecodedGlyph(text string, code []byte, cid int) DecodedGlyph {
	return DecodedGlyph{text: text, code: code, cid: cid}
}

// Text reports the Unicode text decoded from the source glyph code.
func (g DecodedGlyph) Text() string { return g.text }

// CID reports the source character identifier used for width and outline
// lookup.
func (g DecodedGlyph) CID() int { return g.cid }

// BytesCopy returns an independent copy of the source glyph code bytes.
func (g DecodedGlyph) BytesCopy() []byte { return primitives.CloneBytes(g.code) }

// BytesBorrowed returns the source code bytes without copying. The returned
// slice is only valid until the owning decode sequence advances or the caller
// otherwise changes the source buffer; use BytesCopy or Finalize to retain it.
func (g DecodedGlyph) BytesBorrowed() []byte { return g.code }

// Finalize returns an independent snapshot of the decoded glyph.
func (g DecodedGlyph) Finalize() DecodedGlyph {
	g.code = g.BytesCopy()
	return g
}
