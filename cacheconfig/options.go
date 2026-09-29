// Package cacheconfig owns document cache budget values.
package cacheconfig

// Options controls retained parsed-object caches. A zero value is valid and
// disables cache retention when supplied through a document open option.
type Options struct {
	ObjectBytes                int
	ObjectErrorBytes           int
	ObjectStreamErrorBytes     int
	ImageBytes                 int
	InlineImageBytes           int
	DecodedStreamBytes         int
	DecodedErrorBytes          int
	FontBytes                  int
	FontErrorBytes             int
	PageFontBytes              int
	PagePropertyBytes          int
	PagePropertyErrorBytes     int
	PageResourceBytes          int
	PageResourceErrorBytes     int
	XObjectResourceBytes       int
	XObjectResourceErrorBytes  int
	PageUserUnitBytes          int
	PageUserUnitErrorBytes     int
	PageRotationBytes          int
	PageRotationErrorBytes     int
	PageBoxBytes               int
	ContentRootErrorBytes      int
	CFFPathBytes               int
	TrueTypePathBytes          int
	Type1PathBytes             int
	Type3Bytes                 int
	NameTreeBytes              int
	NameTreeErrorBytes         int
	DestinationBytes           int
	DestinationsRootErrorBytes int
	DestinationErrorBytes      int
	ActionScriptBytes          int
	ActionBytes                int
	ActionErrorBytes           int
	MetadataBytes              int
	MetadataXMLBytes           int
	ParentTreeBytes            int
	ParentTreeErrorBytes       int
	PageStructureBytes         int
	PageStructureErrorBytes    int
	AnnotationBytes            int
	AnnotationErrorBytes       int
	AnnotationRootErrorBytes   int
	OutlineBytes               int
	FormFieldBytes             int
	PageBytes                  int
	PagesBytes                 int
	PageLabelRuleBytes         int
	PageLabelsValuesBytes      int
	LookupBytes                int
	MissingPageBytes           int
}
