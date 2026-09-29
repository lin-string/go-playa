package document_test

import (
	"reflect"
	"testing"

	"github.com/lin-string/go-playa/document"
	"github.com/lin-string/go-playa/internal/testfixture"
	"github.com/lin-string/go-playa/page"
)

func TestPublicDocumentSequencesAndNavigation(t *testing.T) {
	if document.ErrNilDocument == nil {
		t.Fatal("ErrNilDocument is nil")
	}
	d := &document.Document{}
	var p page.Page
	var _ *document.ParseError
	var _ document.Annotation
	var _ document.FormField
	var _ document.ContentFilter
	var _ document.ContentKind
	var _ document.ContentOptions
	var _ document.ContentObject
	var _ document.GraphicsState
	var _ document.DashPattern
	var _ document.Color
	var _ document.PathSegment
	var _ document.XObjectObject
	var _ document.ShadingObject
	var _ document.PatternObject
	var _ document.ExtGStateObject
	var _ document.ColorSpaceObject
	var _ document.PropertiesObject
	var _ document.Font
	var _ document.FontResource
	var _ document.FontMetadata
	var _ document.ImageObject
	var _ document.DecodedImage
	var _ document.ImageColorSpace
	var _ document.EncryptionInfo
	var _ document.GlyphObject
	var _ document.PathObject
	var _ document.TagObject
	var _ document.ContentOp
	var _ document.MarkedContent
	var _ document.MarkedContentIndex
	var _ document.MarkedContentContext
	var _ document.OutlineNode
	var _ document.PageLabelSpec
	var _ document.PageStructureEntry
	var _ document.PageStructure
	var _ document.StructElement
	var _ document.StructureContent
	var _ document.StructureItem
	var _ document.StructureIndex
	var _ document.Matrix
	var _ document.Token
	var _ document.TokenKind
	_ = document.FilterAll
	_ = document.FilterText
	_ = document.ContentText
	_ = document.ContentXObject
	_ = document.DefaultContentOptions
	_ = document.DefaultGraphicsState
	_ = (*document.Font).ValidateWithError
	var _ document.Stream
	var _ document.Object
	var _ document.Ref
	var _ document.Dict
	var _ document.Array
	var _ document.Null
	var _ document.Bool
	var _ document.Number
	var _ document.Name
	var _ document.String
	var _ document.Keyword
	var _ document.TextWord
	var _ document.TextLine
	var _ document.TextParagraph
	var _ document.TextObject
	var _ document.TextBox
	var _ document.TextGroup
	var _ document.TextGroupChild
	var _ document.BBoxProvider
	var _ document.LayoutItem
	var _ document.LayoutItemKind
	_ = document.LayoutTextBox
	_ = document.LayoutImage
	_ = document.LayoutPath
	_ = document.LayoutXObject
	_ = (document.LayoutItem{}).Kind
	_ = (document.LayoutItem{}).TextBoxBorrowed
	_ = (document.LayoutItem{}).ContentBorrowed
	_ = (document.LayoutItem{}).TextBoxCopy
	_ = (document.LayoutItem{}).TextBoxCopyWithError
	_ = (document.LayoutItem{}).ContentCopy
	_ = (document.LayoutItem{}).ContentCopyWithError
	_ = (document.LayoutItem{}).Finalize
	_ = (document.LayoutItem{}).FinalizeWithError
	_ = (document.ContentObject{}).TextBorrowed
	_ = (document.ContentObject{}).PathBorrowed
	_ = (document.ContentObject{}).ImageBorrowed
	_ = (document.ContentObject{}).TagBorrowed
	_ = (document.ContentObject{}).XObjectBorrowed
	_ = (document.ContentObject{}).ExtGStateBorrowed
	_ = (document.ContentObject{}).ColorSpaceBorrowed
	_ = (document.ContentObject{}).PatternBorrowed
	_ = (document.ContentObject{}).ShadingBorrowed
	_ = (document.ContentObject{}).PropertiesBorrowed
	_ = (document.ImageColorSpace{}).SpecCopy
	_ = (*document.Document).Encryption
	_ = (*document.Document).EncryptionWithError
	_ = (document.GlyphObject{}).ValidateWithError
	_ = (document.GlyphObject{}).X0
	_ = (document.GlyphObject{}).Y0
	_ = (document.GlyphObject{}).X1
	_ = (document.GlyphObject{}).Y1
	_ = (document.GlyphObject{}).Width
	_ = (document.GlyphObject{}).Height
	_ = (document.GlyphObject{}).IsEmpty
	_ = (document.GlyphObject{}).IsHoverlap
	_ = (document.GlyphObject{}).HDistance
	_ = (document.GlyphObject{}).Hoverlap
	_ = (document.GlyphObject{}).IsVOverlap
	_ = (document.GlyphObject{}).VDistance
	_ = (document.GlyphObject{}).VOverlap
	_ = (document.TextObject{}).ValidateWithError
	_ = (document.TextObject{}).X0
	_ = (document.TextObject{}).Y0
	_ = (document.TextObject{}).X1
	_ = (document.TextObject{}).Y1
	_ = (document.TextObject{}).Width
	_ = (document.TextObject{}).Height
	_ = (document.TextObject{}).IsEmpty
	_ = (document.TextObject{}).IsHoverlap
	_ = (document.TextObject{}).HDistance
	_ = (document.TextObject{}).Hoverlap
	_ = (document.TextObject{}).IsVOverlap
	_ = (document.TextObject{}).VDistance
	_ = (document.TextObject{}).VOverlap
	_ = (document.PathObject{}).X0
	_ = (document.PathObject{}).Y0
	_ = (document.PathObject{}).X1
	_ = (document.PathObject{}).Y1
	_ = (document.PathObject{}).Width
	_ = (document.PathObject{}).Height
	_ = (document.PathObject{}).IsEmpty
	_ = (document.PathObject{}).IsHoverlap
	_ = (document.PathObject{}).HDistance
	_ = (document.PathObject{}).Hoverlap
	_ = (document.PathObject{}).IsVOverlap
	_ = (document.PathObject{}).VDistance
	_ = (document.PathObject{}).VOverlap
	_ = (document.ImageObject{}).BBoxX0
	_ = (document.ImageObject{}).BBoxY0
	_ = (document.ImageObject{}).BBoxX1
	_ = (document.ImageObject{}).BBoxY1
	_ = (document.ImageObject{}).BBoxWidth
	_ = (document.ImageObject{}).BBoxHeight
	_ = (document.ImageObject{}).IsEmpty
	_ = (document.ImageObject{}).IsHoverlap
	_ = (document.ImageObject{}).HDistance
	_ = (document.ImageObject{}).Hoverlap
	_ = (document.ImageObject{}).IsVOverlap
	_ = (document.ImageObject{}).VDistance
	_ = (document.ImageObject{}).VOverlap
	_ = (document.XObjectObject{}).X0
	_ = (document.XObjectObject{}).Y0
	_ = (document.XObjectObject{}).X1
	_ = (document.XObjectObject{}).Y1
	_ = (document.XObjectObject{}).Width
	_ = (document.XObjectObject{}).Height
	_ = (document.XObjectObject{}).IsEmpty
	_ = (document.XObjectObject{}).IsHoverlap
	_ = (document.XObjectObject{}).HDistance
	_ = (document.XObjectObject{}).Hoverlap
	_ = (document.XObjectObject{}).IsVOverlap
	_ = (document.XObjectObject{}).VDistance
	_ = (document.XObjectObject{}).VOverlap
	_ = (document.TextWord{}).X0
	_ = (document.TextWord{}).Y0
	_ = (document.TextWord{}).X1
	_ = (document.TextWord{}).Y1
	_ = (document.TextWord{}).Width
	_ = (document.TextWord{}).Height
	_ = (document.TextWord{}).IsEmpty
	_ = (document.TextWord{}).IsHoverlap
	_ = (document.TextWord{}).HDistance
	_ = (document.TextWord{}).Hoverlap
	_ = (document.TextWord{}).IsVOverlap
	_ = (document.TextWord{}).VDistance
	_ = (document.TextWord{}).VOverlap
	_ = (document.TextLine{}).X0
	_ = (document.TextLine{}).Y0
	_ = (document.TextLine{}).X1
	_ = (document.TextLine{}).Y1
	_ = (document.TextLine{}).Width
	_ = (document.TextLine{}).Height
	_ = (document.TextLine{}).IsEmpty
	_ = (document.TextLine{}).IsHoverlap
	_ = (document.TextLine{}).HDistance
	_ = (document.TextLine{}).Hoverlap
	_ = (document.TextLine{}).IsVOverlap
	_ = (document.TextLine{}).VDistance
	_ = (document.TextLine{}).VOverlap
	_ = (document.TextParagraph{}).X0
	_ = (document.TextParagraph{}).Y0
	_ = (document.TextParagraph{}).X1
	_ = (document.TextParagraph{}).Y1
	_ = (document.TextParagraph{}).Width
	_ = (document.TextParagraph{}).Height
	_ = (document.TextParagraph{}).IsEmpty
	_ = (document.TextParagraph{}).IsHoverlap
	_ = (document.TextParagraph{}).HDistance
	_ = (document.TextParagraph{}).Hoverlap
	_ = (document.TextParagraph{}).IsVOverlap
	_ = (document.TextParagraph{}).VDistance
	_ = (document.TextParagraph{}).VOverlap
	_ = (document.TextBox{}).X0
	_ = (document.TextBox{}).Y0
	_ = (document.TextBox{}).X1
	_ = (document.TextBox{}).Y1
	_ = (document.TextBox{}).Width
	_ = (document.TextBox{}).Height
	_ = (document.TextBox{}).IsEmpty
	_ = (document.TextBox{}).IsHoverlap
	_ = (document.TextBox{}).HDistance
	_ = (document.TextBox{}).Hoverlap
	_ = (document.TextBox{}).IsVOverlap
	_ = (document.TextBox{}).VDistance
	_ = (document.TextBox{}).VOverlap
	_ = (document.TextGroup{}).X0
	_ = (document.TextGroup{}).Y0
	_ = (document.TextGroup{}).X1
	_ = (document.TextGroup{}).Y1
	_ = (document.TextGroup{}).Width
	_ = (document.TextGroup{}).Height
	_ = (document.TextGroup{}).IsEmpty
	_ = (document.TextGroup{}).IsHoverlap
	_ = (document.TextGroup{}).HDistance
	_ = (document.TextGroup{}).Hoverlap
	_ = (document.TextGroup{}).IsVOverlap
	_ = (document.TextGroup{}).VDistance
	_ = (document.TextGroup{}).VOverlap
	_ = (document.LayoutResult{}).ItemsSeq
	_ = (document.LayoutResult{}).ItemsCopy
	_ = (document.LayoutResult{}).ItemsCopyWithError
	_ = (document.TextGroupChild{}).BoxBorrowed
	_ = (document.TextGroupChild{}).GroupBorrowed
	_ = (document.TextGroupChild{}).BoxCopy
	_ = (document.TextGroupChild{}).GroupCopy
	_ = (document.PageLabelSpec{}).Style
	_ = (document.PageLabelSpec{}).Prefix
	_ = (document.PageLabelSpec{}).Start
	_ = (document.OutlineNode{}).Title
	_ = (document.OutlineNode{}).Parent
	_ = (document.OutlineNode{}).HasParent
	_ = (document.OutlineNode{}).ActionKind
	_ = (document.OutlineNode{}).ElementRef
	_ = (document.OutlineNode{}).HasElement
	_ = (document.OutlineNode{}).Count
	_ = (document.OutlineNode{}).HasCount
	_ = (document.StructureContent{}).Kind
	_ = (document.StructureContent{}).MCID
	_ = (document.StructureContent{}).HasMCID
	_ = (document.StructureContent{}).Page
	_ = (document.StructureContent{}).HasPage
	_ = (document.StructureContent{}).HasStream
	_ = (document.StructureContent{}).ObjectRef
	_ = (document.StructureContent{}).HasObject
	_ = (document.StructureItem{}).Kind
	_ = (document.StructureItem{}).ElementCopy
	_ = (document.StructureItem{}).ContentCopy
	_ = (document.StructureItem{}).ElementBorrowed
	_ = (document.StructureItem{}).ContentBorrowed
	_ = (document.PageStructureEntry{}).ElementBorrowed
	_ = (document.PageStructureEntry{}).ElementsSeq
	_ = (document.StructureItem{}).Finalize
	_ = (document.StructureItem{}).FinalizeWithError
	_ = (document.StructElement{}).Type
	_ = (document.StructElement{}).StructureType
	_ = (document.StructElement{}).Role
	_ = (document.StructElement{}).RawRole
	_ = (document.StructElement{}).Title
	_ = (document.StructElement{}).Language
	_ = (document.StructElement{}).AlternateDescription
	_ = (document.StructElement{}).ActualText
	_ = (document.StructElement{}).AbbreviationExpansion
	_ = (document.StructElement{}).ClassName
	_ = (document.StructElement{}).Page
	_ = (document.StructElement{}).HasPage
	_ = (document.StructElement{}).Parent
	_ = (document.StructElement{}).HasParent
	_ = (document.StructElement{}).MCID
	_ = (document.StructElement{}).HasMCID
	_ = (document.StructElement{}).IsMCR
	_ = (document.StructElement{}).ObjectRef
	_ = (document.StructElement{}).HasObject
	_ = (document.StructElement{}).BBox
	_ = (document.StructElement{}).HasBBox
	_ = (document.PageStructureEntry{}).Index
	_ = (document.PageStructure{}).Len
	_ = (document.PageStructure{}).At
	_ = (document.PageStructure{}).ByMCID
	_, _ = d.Images(p)
	pageLabels := (*document.Document).PageLabels
	_ = pageLabels
	getFont := (*document.Document).GetFont
	getFontWithError := (*document.Document).GetFontWithError
	getObject := (*document.Document).Get
	getObjectWithError := (*document.Document).GetWithError
	length := (*document.Document).Len
	keys := (*document.Document).Keys
	values := (*document.Document).Values
	items := (*document.Document).Items
	_ = getFont
	_ = getFontWithError
	_ = getObject
	_ = getObjectWithError
	_ = length
	_ = keys
	_ = values
	_ = items
	_ = (*document.Font).BaseFont
	_ = (*document.Font).CIDCoding
	_ = (*document.Document).PageLabelWithError
	destinations := (*document.Document).Destinations
	_ = destinations
	structureTree := (*document.Document).StructureTree
	_ = structureTree
	structureTreeIndex := (*document.Document).StructureTreeIndex
	_ = structureTreeIndex
	assertSeq2[document.IndirectObject](d.Objects())
	_ = (document.IndirectObject{}).ValueCopy
	_ = (document.IndirectObject{}).Finalize
	_ = (document.IndirectObject{}).Ref
	_ = (document.XRefEntry{}).Object
	_ = (document.XRefEntry{}).Generation
	_ = (document.XRefEntry{}).Offset
	_ = (document.XRefEntry{}).Free
	_ = (document.XRefEntry{}).ObjectStream
	_ = (document.XRefEntry{}).ObjectIndex
	_ = (document.XRefEntry{}).InObjectStream
	_ = (document.XRefEntry{}).Finalize
	_ = (document.XRefTable{}).Kind
	_ = (document.XRefTable{}).Offset
	_ = (document.XRefTable{}).EntriesSeq
	_ = (document.XRefTable{}).EntriesCopy
	_ = (document.XRefTable{}).TrailerCopy
	_ = (document.XRefTable{}).Finalize
	assertSeq2[page.Page](d.Pages())
	assertSeq2[document.DestinationEntry](d.DestinationsSeq())
	_ = (document.DestinationEntry{}).Destination
	_ = (document.DestinationEntry{}).DestinationWithError
	_ = (document.DestinationEntry{}).ValueCopy
	_ = (document.DestinationEntry{}).Finalize
	_ = (document.DestinationEntry{}).Name
	_ = (document.Token{}).Kind
	_ = (document.Token{}).Text
	_ = (document.Token{}).Number
	_ = (document.Token{}).Offset
	_ = (document.Token{}).RawCopy
	_ = (document.Token{}).ValueCopy
	_ = (document.Token{}).Finalize
	assertSeq2[document.NameTreeEntry](d.NameTreeSeq("JavaScript"))
	_ = (document.NameTreeEntry{}).ValueCopy
	_ = (document.NameTreeEntry{}).Finalize
	_ = (document.NameTreeEntry{}).Name
	_ = d.Metadata
	_ = d.MetadataWithError
	_ = d.MetadataXML
	_ = d.Info
	_ = d.InfoWithError
	_ = d.Catalog
	_ = d.CatalogWithError
	_ = d.Names
	_ = d.NamesWithError
	_ = d.IsPrintable
	_ = d.IsModifiable
	_ = d.IsExtractable
	_ = d.WasRecovered
	_ = d.RecoveryError
	_ = d.OpenAction
	_ = d.OpenActionWithError
	_ = d.ResolveAction
	_ = d.ResolveActionWithError
	_ = d.ResolveDestination
	_ = d.ResolveDestinationWithError
	_ = (document.Destination{}).PageObject
	_ = (document.Destination{}).NCoords
	_ = (document.Destination{}).Finalize
	_ = (*document.Action)(nil).Finalize
	_ = (*document.Action)(nil).FinalizeWithError
	_ = (document.Action{}).RawCopy
	var action document.Action
	_ = action.NextSeq
	assertSeq2[*document.Action](action.NextSeq())
	_ = action.NextCopy
	_ = (document.Action{}).PageObject
	_ = (page.FormField{}).ParentField
	_ = testing.Short
}

func TestPublicDocumentMappingViewUsesNewestGeneration(t *testing.T) {
	doc, err := document.Open(testfixture.Path(t, "form_simple.pdf"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = doc.Close() }()

	if doc.Len() != 8 {
		t.Fatalf("Len = %d, want Playa trailer size 8", doc.Len())
	}
	value, ok := doc.Get(1)
	if !ok || value == nil {
		t.Fatalf("Get(1) = %#v, %v; want a catalog", value, ok)
	}
	if _, ok := doc.Get(0); ok {
		t.Fatal("Get(0) unexpectedly found an object")
	}
	if _, err := doc.GetWithError(0); err == nil {
		t.Fatal("GetWithError(0) did not report a missing object")
	}

	var keys []int
	for key, err := range doc.Keys() {
		if err != nil {
			t.Fatal(err)
		}
		keys = append(keys, key)
	}
	if len(keys) != 7 || len(keys) == doc.Len() {
		t.Fatalf("Keys = %#v, Len = %d; want seven in-use keys and trailer size eight", keys, doc.Len())
	}
	for index, key := range keys {
		if key != index+1 {
			t.Fatalf("Keys[%d] = %d, want %d", index, key, index+1)
		}
	}

	values := 0
	for value, err := range doc.Values() {
		if err != nil {
			t.Fatal(err)
		}
		if value == nil {
			t.Fatalf("Values[%d] is nil", values)
		}
		values++
	}
	if values != len(keys) {
		t.Fatalf("Values count = %d, Keys count = %d", values, len(keys))
	}

	items := 0
	for item, err := range doc.Items() {
		if err != nil {
			t.Fatal(err)
		}
		if item.Ref().Object != keys[items] {
			t.Fatalf("Items[%d] ref = %#v, want object %d", items, item.Ref(), keys[items])
		}
		items++
	}
	if items != len(keys) {
		t.Fatalf("Items count = %d, Keys count = %d", items, len(keys))
	}
}

func TestPublicDocumentMappingPreservesPlayaXRefRevisionOrder(t *testing.T) {
	doc, err := document.Open(testfixture.Path(t, "recovery_classic_to_xref_stream.pdf"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = doc.Close() }()

	if doc.Len() != 5 {
		t.Fatalf("Len = %d, want Playa trailer size 5", doc.Len())
	}
	var keys []int
	for key, err := range doc.Keys() {
		if err != nil {
			t.Fatal(err)
		}
		keys = append(keys, key)
	}
	want := []int{3, 4, 1, 2, 3}
	if !reflect.DeepEqual(keys, want) {
		t.Fatalf("Keys = %#v, want Playa xref order %#v", keys, want)
	}
	var itemKeys []int
	for item, err := range doc.Items() {
		if err != nil {
			t.Fatal(err)
		}
		itemKeys = append(itemKeys, item.Ref().Object)
	}
	if !reflect.DeepEqual(itemKeys, want) {
		t.Fatalf("Items keys = %#v, want %#v", itemKeys, want)
	}
}

func TestPublicDocumentOpenOptions(t *testing.T) {
	defaults := document.DefaultDocumentOptions()
	if defaults.Space != document.CoordinateSpaceScreen || !defaults.CacheSet || defaults.Cache.ObjectBytes <= 0 {
		t.Fatalf("default document options = %#v", defaults)
	}
	if document.WithDocumentOptions(defaults) == nil {
		t.Fatal("WithDocumentOptions returned nil")
	}
	var custom document.OpenOption = func(options *document.DocumentOptions) {
		options.Space = document.CoordinateSpacePage
	}
	_ = custom
	if document.ErrObjectNotFound == nil {
		t.Fatal("ErrObjectNotFound is nil")
	}
	if document.ErrPageNotFound == nil {
		t.Fatal("ErrPageNotFound is nil")
	}
	cache := document.DefaultCacheOptions()
	cache.ObjectBytes = 0
	cache.PageBytes = 0
	d, err := document.OpenBytes([]byte("%PDF-1.4\n1 0 obj\n<< /Type /Catalog >>\nendobj\ntrailer\n<< /Root 1 0 R >>\n%%EOF\n"),
		document.WithCoordinateSpace(document.CoordinateSpacePage),
		document.WithCacheOptions(cache),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := d.Close(); err != nil {
			t.Fatal(err)
		}
	}()
	if d.CoordinateSpace() != document.CoordinateSpacePage {
		t.Fatalf("coordinate space = %q, want %q", d.CoordinateSpace(), document.CoordinateSpacePage)
	}
}

func TestPublicNavigationMetadataIsReadOnly(t *testing.T) {
	spec := document.PageLabelSpec{}
	if spec.Style() != "" || spec.Prefix() != "" || spec.Start() != 0 {
		t.Fatalf("zero page-label metadata = %#v", spec)
	}
	node := document.OutlineNode{}
	if node.Title() != "" || node.Parent() != (document.Ref{}) || node.HasParent() || node.ActionKind() != "" || node.ElementRef() != (document.Ref{}) || node.HasElement() || node.Count() != 0 || node.HasCount() {
		t.Fatalf("zero outline metadata = %#v", node)
	}
}

func TestPublicStructureMetadataIsReadOnly(t *testing.T) {
	content := document.StructureContent{}
	if content.Kind() != "" || content.MCID() != 0 || content.HasMCID() || content.Page() != (document.Ref{}) || content.HasPage() || content.HasStream() || content.ObjectRef() != (document.Ref{}) || content.HasObject() {
		t.Fatalf("zero structure content metadata = %#v", content)
	}
	if got := (document.PageStructureEntry{}).Index(); got != 0 {
		t.Fatalf("zero page structure index = %d", got)
	}
}

func TestPublicAnnotationsSequenceIsRepeatableAndEarlyStoppable(t *testing.T) {
	doc, err := document.Open(testfixture.Path(t, "acceptance_navigation_semantics.pdf"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = doc.Close() }()
	page, err := doc.PageAt(0)
	if err != nil {
		t.Fatal(err)
	}
	var first []string
	for annotation, err := range doc.Annotations(page) {
		if err != nil {
			t.Fatal(err)
		}
		first = append(first, annotation.ActionKind())
		break
	}
	if len(first) != 1 || first[0] != "URI" {
		t.Fatalf("first annotation sequence values = %#v", first)
	}
	var second []string
	for annotation, err := range doc.Annotations(page) {
		if err != nil {
			t.Fatal(err)
		}
		second = append(second, annotation.ActionKind())
	}
	if !reflect.DeepEqual(second, []string{"URI", "", "", ""}) {
		t.Fatalf("repeat annotation sequence values = %#v", second)
	}
}

func TestPublicFormFieldsSequenceIsRepeatableAndEarlyStoppable(t *testing.T) {
	doc, err := document.Open(testfixture.Path(t, "acceptance_form_choice.pdf"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = doc.Close() }()
	var first []string
	for field, err := range doc.FormFields() {
		if err != nil {
			t.Fatal(err)
		}
		first = append(first, field.Name())
		break
	}
	if len(first) != 1 || first[0] != "report_type" {
		t.Fatalf("first form sequence values = %#v", first)
	}
	fields, err := doc.CollectFormFields()
	if err != nil {
		t.Fatal(err)
	}
	if len(fields) != 1 || fields[0].Name() != "report_type" || fields[0].Value() != "Annual" {
		t.Fatalf("repeat form sequence values = %#v", fields)
	}
	if options := fields[0].OptionsCopy(); len(options) != 2 || options[0] != "Annual" || options[1] != "Quarterly" {
		t.Fatalf("form options = %#v", options)
	}
}

func TestPublicFormFieldMetadataIsReadOnly(t *testing.T) {
	field := page.FormField{}
	if field.Name() != "" || field.FullName() != "" || field.Page() != (page.Ref{}) || field.HasPage() || field.Parent() != (page.Ref{}) || field.HasParent() || field.FieldType() != "" || field.Flags() != 0 || field.HasFlags() || field.Value() != "" || field.DefaultValue() != "" || field.DefaultAppearance() != "" || field.Rect() != [4]float64{} || field.HasRect() || field.IsWidget() {
		t.Fatalf("zero form-field metadata = %#v", field)
	}
}

func TestPublicNavigationSequencesAreRepeatableAndEarlyStoppable(t *testing.T) {
	doc, err := document.Open(testfixture.Path(t, "acceptance_navigation_semantics.pdf"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = doc.Close() }()
	var destinationNames []string
	for entry, err := range doc.DestinationsSeq() {
		if err != nil {
			t.Fatal(err)
		}
		destinationNames = append(destinationNames, entry.Name())
		break
	}
	if len(destinationNames) != 1 || destinationNames[0] != "chapter" {
		t.Fatalf("first destination sequence values = %#v", destinationNames)
	}
	var repeatedDestinations []string
	for entry, err := range doc.DestinationsSeq() {
		if err != nil {
			t.Fatal(err)
		}
		repeatedDestinations = append(repeatedDestinations, entry.Name())
	}
	if len(repeatedDestinations) != 1 || repeatedDestinations[0] != "chapter" {
		t.Fatalf("repeat destination sequence values = %#v", repeatedDestinations)
	}
	var outlineTitles []string
	for node, err := range doc.Outline() {
		if err != nil {
			t.Fatal(err)
		}
		outlineTitles = append(outlineTitles, node.Title())
		break
	}
	if len(outlineTitles) != 1 || outlineTitles[0] != "Chapter 1" {
		t.Fatalf("first outline sequence values = %#v", outlineTitles)
	}
	var repeatedOutline []string
	for node, err := range doc.Outline() {
		if err != nil {
			t.Fatal(err)
		}
		repeatedOutline = append(repeatedOutline, node.Title())
	}
	if len(repeatedOutline) != 1 || repeatedOutline[0] != "Chapter 1" {
		t.Fatalf("repeat outline sequence values = %#v", repeatedOutline)
	}
}

func TestPublicStructureTreeSequenceIsRepeatableAndEarlyStoppable(t *testing.T) {
	doc, err := document.Open(testfixture.Path(t, "acceptance_tagged_text.pdf"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = doc.Close() }()
	var first []string
	for element, err := range doc.StructureTreeSeq() {
		if err != nil {
			t.Fatal(err)
		}
		first = append(first, element.Role()+":"+element.Title())
		break
	}
	if len(first) != 1 || first[0] != "P:Financial summary" {
		t.Fatalf("first structure sequence values = %#v", first)
	}
	var repeated []string
	for element, err := range doc.StructureTreeSeq() {
		if err != nil {
			t.Fatal(err)
		}
		repeated = append(repeated, element.Role()+":"+element.Title())
	}
	if len(repeated) != 1 || repeated[0] != "P:Financial summary" {
		t.Fatalf("repeat structure sequence values = %#v", repeated)
	}
}

func TestPublicPageLabelsSequenceIsRepeatableAndEarlyStoppable(t *testing.T) {
	doc, err := document.Open(testfixture.Path(t, "acceptance_navigation_semantics.pdf"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = doc.Close() }()
	var first []string
	for label, err := range doc.PageLabelsSeq() {
		if err != nil {
			t.Fatal(err)
		}
		first = append(first, label)
		break
	}
	if len(first) != 1 || first[0] != "Section 3" {
		t.Fatalf("first page-label sequence values = %#v", first)
	}
	var repeated []string
	for label, err := range doc.PageLabelsSeq() {
		if err != nil {
			t.Fatal(err)
		}
		repeated = append(repeated, label)
	}
	if len(repeated) != 1 || repeated[0] != "Section 3" {
		t.Fatalf("repeat page-label sequence values = %#v", repeated)
	}
}

func TestPublicMetadataXMLReturnsOwnedRepeatableBytes(t *testing.T) {
	doc, err := document.Open(testfixture.Path(t, "acceptance_navigation_semantics.pdf"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = doc.Close() }()
	first, err := doc.MetadataXML()
	if err != nil || string(first) != "<x:xmpmeta><dc:title>Quarterly report</dc:title></x:xmpmeta>" {
		t.Fatalf("first metadata XML = %q, err=%v", first, err)
	}
	first[0] = 'X'
	second, err := doc.MetadataXML()
	if err != nil || string(second) != "<x:xmpmeta><dc:title>Quarterly report</dc:title></x:xmpmeta>" {
		t.Fatalf("repeat metadata XML = %q, err=%v", second, err)
	}
}

func TestPublicPagesSequenceIsOrderedRepeatableAndEarlyStoppable(t *testing.T) {
	doc, err := document.Open(testfixture.Path(t, "acceptance_page_labels.pdf"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = doc.Close() }()
	var first []int
	for page, err := range doc.Pages() {
		if err != nil {
			t.Fatal(err)
		}
		if page.Index() != 0 {
			t.Fatalf("first page index = %d, want 0", page.Index())
		}
		first = append(first, page.Number())
		break
	}
	if len(first) != 1 || first[0] != 1 {
		t.Fatalf("first page sequence values = %#v", first)
	}
	var repeated []int
	var repeatedIndexes []int
	for page, err := range doc.Pages() {
		if err != nil {
			t.Fatal(err)
		}
		repeated = append(repeated, page.Number())
		repeatedIndexes = append(repeatedIndexes, page.Index())
	}
	if len(repeated) != 4 || repeated[0] != 1 || repeated[1] != 2 || repeated[2] != 3 || repeated[3] != 4 {
		t.Fatalf("repeat page sequence values = %#v", repeated)
	}
	if len(repeatedIndexes) != 4 || repeatedIndexes[0] != 0 || repeatedIndexes[1] != 1 || repeatedIndexes[2] != 2 || repeatedIndexes[3] != 3 {
		t.Fatalf("repeat page indexes = %#v", repeatedIndexes)
	}
}

func TestPublicObjectsSequenceIsRepeatableAndEarlyStoppable(t *testing.T) {
	doc, err := document.Open(testfixture.Path(t, "acceptance_cjk_cid.pdf"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = doc.Close() }()
	first := 0
	for object, err := range doc.Objects() {
		if err != nil {
			t.Fatal(err)
		}
		if object.Ref().Object <= 0 {
			t.Fatalf("first object reference = %#v", object.Ref())
		}
		first++
		break
	}
	if first != 1 {
		t.Fatalf("early object count = %d", first)
	}
	repeated := 0
	for object, err := range doc.Objects() {
		if err != nil {
			t.Fatal(err)
		}
		if object.Ref().Object <= 0 {
			t.Fatalf("repeat object reference = %#v", object.Ref())
		}
		repeated++
	}
	if repeated == 0 {
		t.Fatal("repeat object sequence was empty")
	}
}

func TestPublicDocumentTokensSequenceIsRepeatableAndFinalizable(t *testing.T) {
	doc, err := document.Open(testfixture.Path(t, "acceptance_cjk_cid.pdf"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = doc.Close() }()

	first := 0
	for token, err := range doc.Tokens() {
		if err != nil {
			t.Fatal(err)
		}
		if token.Offset() < 0 {
			t.Fatalf("first token offset = %d", token.Offset())
		}
		finalized := token.Finalize()
		if string(finalized.RawCopy()) != string(token.RawCopy()) {
			t.Fatal("finalized token changed its raw bytes")
		}
		first++
		break
	}
	if first != 1 {
		t.Fatalf("early token count = %d", first)
	}

	repeated := 0
	for token, err := range doc.Tokens() {
		if err != nil {
			t.Fatal(err)
		}
		if token.Offset() < 0 {
			t.Fatalf("repeat token offset = %d", token.Offset())
		}
		repeated++
	}
	if repeated == 0 {
		t.Fatal("repeat token sequence was empty")
	}
}

func assertSeq2[T any](seq func(func(T, error) bool)) {}
