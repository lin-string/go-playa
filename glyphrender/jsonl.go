package glyphrender

import (
	"encoding/json"
	"errors"
	"io"
	"iter"

	"github.com/lin-string/go-playa/page"
)

var ErrInvalidOutput = errors.New("glyphrender: invalid output")

// ManifestEntry is the bounded metadata projection for one glyph. Code is
// copied from the glyph before encoding so borrowed sequence storage is never
// retained by the encoder.
type ManifestEntry struct {
	Index        int        `json:"index"`
	Text         string     `json:"text"`
	Chars        string     `json:"chars"`
	Code         []byte     `json:"code,omitempty"`
	CID          int        `json:"cid"`
	GID          int        `json:"gid,omitempty"`
	FontName     string     `json:"font_name,omitempty"`
	FontSize     float64    `json:"font_size,omitempty"`
	Origin       [2]float64 `json:"origin"`
	Displacement [2]float64 `json:"displacement"`
	BBox         [4]float64 `json:"bbox"`
	Vertical     bool       `json:"vertical,omitempty"`
	Unmapped     bool       `json:"unmapped,omitempty"`
	Invisible    bool       `json:"invisible,omitempty"`
}

// WriteJSONL streams one manifest entry per glyph. The sequence may be
// borrowed and lazy; each glyph is finalized long enough to copy its code and
// then released before the next line is encoded.
func WriteJSONL(output io.Writer, glyphs iter.Seq2[page.GlyphObject, error]) error {
	if output == nil {
		return ErrInvalidOutput
	}
	encoder := json.NewEncoder(output)
	index := 0
	for glyph, err := range glyphs {
		if err != nil {
			return err
		}
		if err := encodeManifestEntry(encoder, index, glyph); err != nil {
			return err
		}
		index++
	}
	return nil
}

func encodeManifestEntry(encoder *json.Encoder, index int, glyph page.GlyphObject) error {
	return encoder.Encode(ManifestEntry{
		Index:        index,
		Text:         glyph.Text(),
		Chars:        glyph.Chars(),
		Code:         glyph.Codes(),
		CID:          glyph.CID(),
		GID:          glyph.GID(),
		FontName:     glyph.FontName(),
		FontSize:     glyph.FontSize(),
		Origin:       glyph.Origin(),
		Displacement: glyph.Displacement(),
		BBox:         glyph.BBox(),
		Vertical:     glyph.Vertical(),
		Unmapped:     glyph.Unmapped(),
		Invisible:    glyph.Invisible(),
	})
}
