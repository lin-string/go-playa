package structure_test

import (
	"testing"

	"github.com/lin-string/go-playa/structure"
)

func TestPublicStructureSurface(t *testing.T) {
	if structure.StructureMarkedContent == structure.StructureObject {
		t.Fatal("structure content kinds are not distinct")
	}
	if structure.ErrPageNotFound == nil {
		t.Fatal("ErrPageNotFound is nil")
	}
	if structure.ErrNilDocument == nil {
		t.Fatal("ErrNilDocument is nil")
	}
	var _ *structure.ParseError
	var _ structure.PDFObject
	var _ structure.Ref
	var _ structure.Dict
	var _ structure.Array
	var _ structure.Stream
	var _ structure.Null
	var _ structure.Bool
	var _ structure.Number
	var _ structure.Name
	var _ structure.String
	var _ structure.Keyword
	var _ structure.StructureItem
	var _ structure.Element
	var _ structure.Index
	var _ structure.Content
	var _ structure.ContentKind
	var _ structure.StructureItemKind
	var _ structure.Page
	_ = (structure.Index{}).RootsCopy
	_ = (structure.Index{}).RootsCopyWithError
	_ = (structure.Index{}).ByMCIDCopy
	_ = (structure.Index{}).ByMCIDCopyWithError
	_ = (structure.Index{}).Finalize
	_ = (structure.Index{}).FinalizeWithError
	var element structure.Element
	assertSeq2[structure.Element](element.ChildrenSeq())
	assertSeq2[structure.Content](element.ContentsSeq())
	assertSeq2[structure.Content](element.PageOrderSeq(nil))
	childrenCopy, childrenCopyErr := element.ChildrenCopy()
	contentsCopy, contentsCopyErr := element.ContentsCopy()
	_, _, _, _ = childrenCopy, childrenCopyErr, contentsCopy, contentsCopyErr
	_, _ = element.PageObject(nil)
	_, _ = element.Object(nil)
	_, _, _ = element.ObjectWithError(nil)
	_ = element.ParentElement(nil)
	_, _ = element.ParentElementWithError(nil)
	_ = element.Finalize
	_ = element.FinalizeWithError
	_ = element.AlternateDescription
	_ = element.AbbreviationExpansion
	var content structure.Content
	assertSeq2[structure.ContentObject](content.ContentSeq(nil))
	_ = content.Text
	_ = content.BBoxValue
	_, _ = content.PageObject(nil)
	_, _ = content.Object(nil)
	_, _, _ = content.ObjectWithError(nil)
	_ = content.DictCopy
	_ = content.StreamCopy
	_ = (structure.Element{}).AttributesCopy
	_ = (structure.Element{}).DictCopy
	_ = (structure.Element{}).ObjectCopy
	_ = content.Finalize
	_ = structure.StructureMarkedContent
	_ = structure.StructureObject
	_ = testing.Short
}

func assertSeq2[T any](seq func(func(T, error) bool)) {}
