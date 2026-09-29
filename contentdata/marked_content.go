package contentdata

import (
	"encoding/json"

	"github.com/lin-string/go-playa/pdftypes/primitives"
)

// MarkedContent is the dependency-free value portion of one BMC/BDC section.
// Lazy child traversal and document-bound ParentTree resolution remain outside
// this package.
type MarkedContent struct {
	tag        string
	page       primitives.Ref
	hasPage    bool
	mcid       int
	hasMCID    bool
	actualText string
	properties primitives.Dict
	ops        []ContentOp
}

// NewMarkedContent constructs an owned marked-content value.
func NewMarkedContent(tag string, page primitives.Ref, hasPage bool, mcid int, hasMCID bool, actualText string, properties primitives.Dict, ops []ContentOp) MarkedContent {
	return MarkedContent{
		tag:        tag,
		page:       page,
		hasPage:    hasPage,
		mcid:       mcid,
		hasMCID:    hasMCID,
		actualText: actualText,
		properties: cloneDict(properties),
		ops:        cloneContentOps(ops),
	}
}

// NewMarkedContentBorrowed constructs a low-allocation value over data owned
// by the caller. Use PropertiesCopy, OpsCopy, or Finalize before retaining it.
func NewMarkedContentBorrowed(tag string, page primitives.Ref, hasPage bool, mcid int, hasMCID bool, actualText string, properties primitives.Dict, ops []ContentOp) MarkedContent {
	return MarkedContent{
		tag:        tag,
		page:       page,
		hasPage:    hasPage,
		mcid:       mcid,
		hasMCID:    hasMCID,
		actualText: actualText,
		properties: properties,
		ops:        ops,
	}
}

// Tag returns the marked-content tag name.
func (m MarkedContent) Tag() string { return m.tag }

// Page returns the owning page reference, when present.
func (m MarkedContent) Page() primitives.Ref { return m.page }

// HasPage reports whether Page identifies an owning page.
func (m MarkedContent) HasPage() bool { return m.hasPage }

// MCID returns the marked-content identifier, when present.
func (m MarkedContent) MCID() int { return m.mcid }

// HasMCID reports whether MCID is present.
func (m MarkedContent) HasMCID() bool { return m.hasMCID }

// ActualText returns the decoded replacement text, when present.
func (m MarkedContent) ActualText() string { return m.actualText }

// PropertiesCopy returns independent marked-content properties.
func (m MarkedContent) PropertiesCopy() primitives.Dict { return cloneDict(m.properties) }

// OpsCopy returns independent content operations.
func (m MarkedContent) OpsCopy() []ContentOp { return cloneContentOps(m.ops) }

// OpsBorrowed returns the operation slice for low-allocation internal
// adapters. Use OpsCopy or Finalize before retaining it.
func (m MarkedContent) OpsBorrowed() []ContentOp { return m.ops }

// Finalize returns an independent snapshot of the marked-content value.
func (m MarkedContent) Finalize() MarkedContent {
	m.properties = cloneDict(m.properties)
	m.ops = cloneContentOps(m.ops)
	return m
}

func (m MarkedContent) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Tag        string
		Page       primitives.Ref
		HasPage    bool
		MCID       int
		HasMCID    bool
		ActualText string
		Properties primitives.Dict `json:"Properties"`
		Ops        []ContentOp     `json:"Ops"`
	}{m.tag, m.page, m.hasPage, m.mcid, m.hasMCID, m.actualText, m.properties, m.ops})
}

func cloneContentOps(values []ContentOp) []ContentOp {
	if values == nil {
		return nil
	}
	out := make([]ContentOp, len(values))
	for i, value := range values {
		out[i] = value.Finalize()
	}
	return out
}
