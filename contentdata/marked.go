package contentdata

import (
	"encoding/json"

	"github.com/lin-string/go-playa/pdftypes/primitives"
)

// MarkedContentContext is the dependency-free value portion of one enclosing
// marked-content section. Interpreter grouping remains document-owned.
type MarkedContentContext struct {
	tag        string
	properties primitives.Dict
	actualText string
	mcid       int
	hasMCID    bool
}

// NewMarkedContentContext constructs an owned marked-content value.
func NewMarkedContentContext(tag string, properties primitives.Dict, actualText string, mcid int, hasMCID bool) MarkedContentContext {
	return MarkedContentContext{
		tag:        tag,
		properties: cloneDict(properties),
		actualText: actualText,
		mcid:       mcid,
		hasMCID:    hasMCID,
	}
}

// NewMarkedContentContextBorrowed constructs a low-allocation value over
// properties owned by the caller. Use PropertiesCopy or Finalize before
// retaining it beyond the source iteration.
func NewMarkedContentContextBorrowed(tag string, properties primitives.Dict, actualText string, mcid int, hasMCID bool) MarkedContentContext {
	return MarkedContentContext{
		tag:        tag,
		properties: properties,
		actualText: actualText,
		mcid:       mcid,
		hasMCID:    hasMCID,
	}
}

// Tag returns the enclosing marked-content tag name.
func (m MarkedContentContext) Tag() string { return m.tag }

// ActualText returns the decoded replacement text, when present.
func (m MarkedContentContext) ActualText() string { return m.actualText }

// MCID returns the enclosing marked-content identifier, when present.
func (m MarkedContentContext) MCID() int { return m.mcid }

// HasMCID reports whether MCID is present.
func (m MarkedContentContext) HasMCID() bool { return m.hasMCID }

// HasProperty reports whether a marked-content property is present.
func (m MarkedContentContext) HasProperty(name primitives.Name) bool {
	_, ok := m.properties[name]
	return ok
}

// PropertiesCopy returns an independent copy of enclosing marked-content properties.
func (m MarkedContentContext) PropertiesCopy() primitives.Dict { return cloneDict(m.properties) }

// Finalize returns an independent snapshot of the value.
func (m MarkedContentContext) Finalize() MarkedContentContext {
	m.properties = cloneDict(m.properties)
	return m
}

func (m MarkedContentContext) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Tag        string
		Properties primitives.Dict
		ActualText string
		MCID       int
		HasMCID    bool
	}{m.tag, m.properties, m.actualText, m.mcid, m.hasMCID})
}
