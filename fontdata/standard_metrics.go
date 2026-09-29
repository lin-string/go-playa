package fontdata

// LookupStandardFontMetric returns the Core14 metric for name.
//
// The returned value is a read-only view of generated resource data. Use
// WidthsCopy or Finalize when an owned snapshot is required.
func LookupStandardFontMetric(name string) (StandardFontMetric, bool) {
	metric, ok := standardFontMetrics[name]
	return metric, ok
}

func (m StandardFontMetric) Ascent() float64 {
	return m.ascent
}

func (m StandardFontMetric) Descent() float64 {
	return m.descent
}

// HasAscent reports whether the source AFM defines an Ascender field.
func (m StandardFontMetric) HasAscent() bool {
	return m.hasAscent
}

// HasDescent reports whether the source AFM defines a Descender field.
func (m StandardFontMetric) HasDescent() bool {
	return m.hasDescent
}

// CapHeight returns the Core 14 descriptor cap height in glyph-space units.
func (m StandardFontMetric) CapHeight() float64 {
	return m.capHeight
}

// Flags returns the Core 14 descriptor flags used by Playa.
func (m StandardFontMetric) Flags() int {
	return m.flags
}

func (m StandardFontMetric) BBox() [4]float64 {
	return m.bbox
}

func (m StandardFontMetric) ItalicAngle() float64 {
	return m.italicAngle
}

func (m StandardFontMetric) Width(codepoint rune) (float64, bool) {
	width, ok := m.widths[codepoint]
	return width, ok
}

// WidthsCopy returns an owned copy of the encoded glyph widths.
func (m StandardFontMetric) WidthsCopy() map[rune]float64 {
	if m.widths == nil {
		return nil
	}
	widths := make(map[rune]float64, len(m.widths))
	for codepoint, width := range m.widths {
		widths[codepoint] = width
	}
	return widths
}

// Finalize returns an owned metric snapshot independent of generated data.
func (m StandardFontMetric) Finalize() StandardFontMetric {
	m.widths = m.WidthsCopy()
	return m
}

// LookupZapfDingbatsWidth returns the AFM width for a ZapfDingbats code.
func LookupZapfDingbatsWidth(code byte) (float64, bool) {
	width, ok := zapfDingbatsWidths[code]
	return width, ok
}
