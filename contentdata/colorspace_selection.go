package contentdata

import "github.com/lin-string/go-playa/pdftypes/primitives"

// ColorSpaceSelection is the document-independent value portion of a named
// content color-space selection. Color-space description and lazy lookup data
// remain in document because they depend on parser and cache state.
type ColorSpaceSelection struct {
	name    string
	page    primitives.Ref
	hasPage bool
	stroke  bool
	spec    primitives.Object
}

type ColorSpaceSelectionSpec struct {
	Name    string
	Page    primitives.Ref
	HasPage bool
	Stroke  bool
	Spec    primitives.Object
}

func NewColorSpaceSelection(spec ColorSpaceSelectionSpec) ColorSpaceSelection {
	return ColorSpaceSelection{
		name: spec.Name, page: spec.Page, hasPage: spec.HasPage,
		stroke: spec.Stroke, spec: cloneObject(spec.Spec),
	}
}

func (c ColorSpaceSelection) Name() string                { return c.name }
func (c ColorSpaceSelection) Page() primitives.Ref        { return c.page }
func (c ColorSpaceSelection) HasPage() bool               { return c.hasPage }
func (c ColorSpaceSelection) Stroke() bool                { return c.stroke }
func (c ColorSpaceSelection) SpecCopy() primitives.Object { return cloneObject(c.spec) }
func (c ColorSpaceSelection) Finalize() ColorSpaceSelection {
	return NewColorSpaceSelection(ColorSpaceSelectionSpec{
		Name: c.name, Page: c.page, HasPage: c.hasPage,
		Stroke: c.stroke, Spec: c.spec,
	})
}
