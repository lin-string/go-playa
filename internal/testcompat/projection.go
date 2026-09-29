// Package testcompat provides the repository-only Playa comparison projection.
package testcompat

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"iter"
	"math/big"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	core "github.com/lin-string/go-playa/document"
	"github.com/lin-string/go-playa/geometry"
	"github.com/lin-string/go-playa/layout"
	pdfparser "github.com/lin-string/go-playa/parser"
	"github.com/lin-string/go-playa/parserconfig"
	publictypes "github.com/lin-string/go-playa/pdftypes"
	"github.com/lin-string/go-playa/pdftypes/primitives"
	"github.com/lin-string/go-playa/textconfig"
)

const SchemaVersion = "go-playa.compat/v6"

type Options struct {
	Pages    []int
	Sections []string
}

type Report struct {
	SchemaVersion   string             `json:"schema_version"`
	PageCount       int                `json:"page_count"`
	PageLabels      []string           `json:"page_labels"`
	PDFVersion      string             `json:"pdf_version"`
	IsTagged        bool               `json:"is_tagged"`
	IsPrintable     bool               `json:"is_printable"`
	IsModifiable    bool               `json:"is_modifiable"`
	IsExtractable   bool               `json:"is_extractable"`
	Info            map[string]any     `json:"info"`
	Catalog         map[string]any     `json:"catalog"`
	Names           map[string]any     `json:"names"`
	Trailer         map[string]any     `json:"trailer"`
	OpenAction      *Action            `json:"open_action"`
	Destinations    []NamedDestination `json:"destinations"`
	Outline         []Outline          `json:"outline"`
	Structure       []StructureElement `json:"structure"`
	NeedAppearances bool               `json:"need_appearances"`
	Forms           []Form             `json:"forms"`
	DocumentFonts   []Font             `json:"document_fonts"`
	Pages           []Page             `json:"pages"`
	Objects         []ObjectRecord     `json:"objects,omitempty"`
	Mapping         []MappingRecord    `json:"mapping,omitempty"`
	Tokens          []TokenRecord      `json:"tokens,omitempty"`
	Buffer          BufferRecord       `json:"buffer,omitempty"`
	XRefs           []XRefRecord       `json:"xrefs,omitempty"`
}

type ObjectRecord struct {
	Object     int           `json:"object"`
	Generation int           `json:"generation"`
	Value      interface{}   `json:"value,omitempty"`
	Stream     *StreamRecord `json:"stream,omitempty"`
}

// MappingRecord is one entry from Playa's Mapping-style Document view. It
// intentionally omits generation because the Python mapping key is only the
// object number; repeated object numbers can occur across XRef revisions and
// each value resolves through the current object index.
type MappingRecord struct {
	Object int           `json:"object"`
	Value  interface{}   `json:"value,omitempty"`
	Stream *StreamRecord `json:"stream,omitempty"`
}

type StreamRecord struct {
	Dict   map[string]interface{} `json:"dict"`
	Length int                    `json:"length"`
	SHA256 string                 `json:"sha256"`
}

type PageStreamRecord struct {
	Length int    `json:"length"`
	SHA256 string `json:"sha256"`
}

type XObjectRecord struct {
	Name        string                 `json:"name"`
	BBox        [4]float64             `json:"bbox"`
	Resources   map[string]interface{} `json:"resources"`
	Group       interface{}            `json:"group"`
	Fonts       []Font                 `json:"fonts"`
	Structure   []StructureElement     `json:"structure"`
	PageIndex   int                    `json:"page_index"`
	CTM         [6]float64             `json:"ctm"`
	State       GraphicsStateRecord    `json:"state"`
	MarkedStack []MarkedContext        `json:"marked_stack"`
	Length      int                    `json:"length"`
	SHA256      string                 `json:"sha256"`
	Tokens      []TokenRecord          `json:"tokens"`
	Contents    []interface{}          `json:"contents"`
}

type TokenRecord struct {
	Kind  string      `json:"kind"`
	Value interface{} `json:"value,omitempty"`
}

type BufferRecord struct {
	Length int    `json:"length"`
	SHA256 string `json:"sha256"`
}

type XRefRecord struct {
	Kind    string            `json:"kind"`
	Offset  int               `json:"offset,omitempty"`
	Entries []XRefEntryRecord `json:"entries"`
	Trailer map[string]any    `json:"trailer"`
}

type XRefEntryRecord struct {
	Object         int  `json:"object"`
	Generation     int  `json:"generation"`
	Offset         int  `json:"offset"`
	InObjectStream bool `json:"in_object_stream,omitempty"`
	ObjectStream   *int `json:"object_stream,omitempty"`
	ObjectIndex    *int `json:"object_index,omitempty"`
}

type NamedDestination struct {
	Name        string      `json:"name"`
	Destination Destination `json:"destination"`
}

type Destination struct {
	PageIndex int           `json:"page_index"`
	View      string        `json:"view"`
	Params    []interface{} `json:"params"`
}

type Outline struct {
	Title       string      `json:"title"`
	Destination Destination `json:"destination"`
	ActionKind  string      `json:"action_kind"`
	Action      *Action     `json:"action"`
	Count       int         `json:"count"`
	HasCount    bool        `json:"has_count"`
	Children    []Outline   `json:"children"`
}

// Action is the compatibility projection of Playa's normalized action model.
// Raw retains the source action dictionary while the scalar fields exercise
// the Go convenience accessors and Next exercises the lazy action chain.
type Action struct {
	Kind        string                 `json:"kind"`
	URI         string                 `json:"uri"`
	File        string                 `json:"file"`
	Name        string                 `json:"name"`
	Script      string                 `json:"script"`
	Raw         map[string]interface{} `json:"raw"`
	Destination *Destination           `json:"destination"`
	Next        []Action               `json:"next"`
}

type Form struct {
	Name              string     `json:"name"`
	FullName          string     `json:"full_name"`
	FieldType         string     `json:"field_type"`
	Flags             int        `json:"flags"`
	HasFlags          bool       `json:"has_flags"`
	Value             string     `json:"value"`
	DefaultValue      string     `json:"default_value"`
	DefaultAppearance string     `json:"default_appearance"`
	Options           []string   `json:"options"`
	Selected          []int      `json:"selected"`
	Rect              [4]float64 `json:"rect"`
	HasRect           bool       `json:"has_rect"`
	IsWidget          bool       `json:"is_widget"`
	Kids              []Form     `json:"kids"`
}

type Page struct {
	Index               int                `json:"index"`
	Label               string             `json:"label"`
	Width               float64            `json:"width"`
	Height              float64            `json:"height"`
	Rotation            int                `json:"rotation"`
	ParentKey           *int               `json:"parent_key"`
	Text                []Text             `json:"text"`
	Paths               []Path             `json:"paths"`
	Images              []Image            `json:"images"`
	Fonts               []Font             `json:"fonts"`
	Tags                []Tag              `json:"tags"`
	Structure           []StructureElement `json:"structure,omitempty"`
	Marked              []MarkedSection    `json:"marked"`
	Annotations         []Annotation       `json:"annotations"`
	Flatten             []ContentRecord    `json:"flatten,omitempty"`
	Interp              []ContentRecord    `json:"interp,omitempty"`
	Streams             []PageStreamRecord `json:"streams,omitempty"`
	Tokens              []TokenRecord      `json:"tokens,omitempty"`
	XObjects            []XObjectRecord    `json:"xobjects,omitempty"`
	Contents            []interface{}      `json:"contents,omitempty"`
	ExtractText         string             `json:"extract_text,omitempty"`
	ExtractTextTagged   string             `json:"extract_text_tagged,omitempty"`
	ExtractTextUntagged string             `json:"extract_text_untagged,omitempty"`
	Glyphs              []Glyph            `json:"glyphs,omitempty"`
	Layout              *LayoutRecord      `json:"layout,omitempty"`
}

type LayoutNode struct {
	Kind     string       `json:"kind"`
	Text     string       `json:"text"`
	BBox     [4]float64   `json:"bbox"`
	Vertical bool         `json:"vertical"`
	Index    *int         `json:"index,omitempty"`
	Children []LayoutNode `json:"children,omitempty"`
}

type LayoutRecord struct {
	Lines      []LayoutNode       `json:"lines"`
	TextBoxes  []LayoutNode       `json:"text_boxes"`
	TextGroups []LayoutNode       `json:"text_groups"`
	Items      []LayoutItemRecord `json:"items"`
}

type LayoutItemRecord struct {
	Kind    string      `json:"kind"`
	BBox    [4]float64  `json:"bbox"`
	HasBBox bool        `json:"has_bbox"`
	TextBox *LayoutNode `json:"text_box,omitempty"`
}

type ContentRecord struct {
	Kind        string              `json:"kind"`
	BBox        [4]float64          `json:"bbox"`
	HasBBox     bool                `json:"has_bbox"`
	Matrix      [6]float64          `json:"matrix"`
	HasMatrix   bool                `json:"has_matrix"`
	PageIndex   int                 `json:"page_index"`
	MCID        int                 `json:"mcid"`
	HasMCID     bool                `json:"has_mcid"`
	MarkedStack []MarkedContext     `json:"marked_stack"`
	State       GraphicsStateRecord `json:"state"`
	Parent      *StructureElement   `json:"parent"`
	ChildCount  int                 `json:"child_count"`
	Text        string              `json:"text,omitempty"`
	GlyphCount  int                 `json:"glyph_count,omitempty"`
}

type ColorRecord struct {
	Space      string    `json:"space"`
	Values     []float64 `json:"values"`
	Pattern    string    `json:"pattern"`
	Components int       `json:"components"`
}

type GraphicsStateRecord struct {
	LineWidth        float64     `json:"line_width"`
	LineCap          int         `json:"line_cap"`
	LineJoin         int         `json:"line_join"`
	MiterLimit       float64     `json:"miter_limit"`
	Dash             []float64   `json:"dash"`
	DashPhase        float64     `json:"dash_phase"`
	Intent           string      `json:"intent"`
	StrokeAdjustment bool        `json:"stroke_adjustment"`
	BlendMode        string      `json:"blend_mode"`
	StrokeAlpha      float64     `json:"stroke_alpha"`
	FillAlpha        float64     `json:"fill_alpha"`
	AlphaSource      bool        `json:"alpha_source"`
	BlackPointComp   string      `json:"black_point_comp"`
	Flatness         float64     `json:"flatness"`
	StrokeColor      ColorRecord `json:"stroke_color"`
	FillColor        ColorRecord `json:"fill_color"`
	HasFont          bool        `json:"has_font"`
	FontSize         float64     `json:"font_size"`
	CharacterSpacing float64     `json:"character_spacing"`
	WordSpacing      float64     `json:"word_spacing"`
	Scaling          float64     `json:"scaling"`
	Leading          float64     `json:"leading"`
	RenderMode       int         `json:"render_mode"`
	Rise             float64     `json:"rise"`
	Knockout         bool        `json:"knockout"`
	HasClip          bool        `json:"has_clip"`
}

type Path struct {
	RawSegments      []PathSegment          `json:"raw_segments"`
	Segments         []PathSegment          `json:"segments"`
	Stroke           bool                   `json:"stroke"`
	Fill             bool                   `json:"fill"`
	EvenOdd          bool                   `json:"evenodd"`
	BBox             [4]float64             `json:"bbox"`
	PageIndex        int                    `json:"page_index"`
	CTM              [6]float64             `json:"ctm"`
	State            GraphicsStateRecord    `json:"state"`
	MarkedTag        string                 `json:"marked_tag"`
	MarkedProperties map[string]interface{} `json:"marked_properties"`
	MarkedStack      []MarkedContext        `json:"marked_stack"`
}

type PathSegment struct {
	Operator string       `json:"operator"`
	Points   [][2]float64 `json:"points"`
}

type Image struct {
	Name             string                 `json:"name"`
	Width            int                    `json:"width"`
	Height           int                    `json:"height"`
	Bits             int                    `json:"bits"`
	ImageMask        bool                   `json:"image_mask"`
	ColorSpace       string                 `json:"color_space"`
	Components       int                    `json:"components"`
	Filters          []string               `json:"filters"`
	StreamLength     int                    `json:"stream_length"`
	StreamSHA256     string                 `json:"stream_sha256"`
	BBox             [4]float64             `json:"bbox"`
	PageIndex        int                    `json:"page_index"`
	CTM              [6]float64             `json:"ctm"`
	State            GraphicsStateRecord    `json:"state"`
	MarkedTag        string                 `json:"marked_tag"`
	MarkedProperties map[string]interface{} `json:"marked_properties"`
	MarkedStack      []MarkedContext        `json:"marked_stack"`
}

type Font struct {
	Name         string        `json:"name"`
	FontName     string        `json:"fontname"`
	BaseFont     string        `json:"basefont"`
	CIDCoding    string        `json:"cidcoding"`
	Flags        int           `json:"flags"`
	Ascent       float64       `json:"ascent"`
	Descent      float64       `json:"descent"`
	Leading      float64       `json:"leading"`
	ItalicAngle  float64       `json:"italic_angle"`
	DefaultWidth float64       `json:"default_width"`
	Matrix       [6]float64    `json:"matrix"`
	Vertical     bool          `json:"vertical"`
	Multibyte    bool          `json:"multibyte"`
	BBox         [4]float64    `json:"bbox"`
	Metrics      []FontMetric  `json:"metrics"`
	Decode       []FontDecode  `json:"decode"`
	ToUnicode    []FontUnicode `json:"to_unicode"`
}

type FontMetric struct {
	CID      int        `json:"cid"`
	HDisp    float64    `json:"hdisp"`
	VDisp    float64    `json:"vdisp"`
	Position [2]float64 `json:"position"`
	BBox     [4]float64 `json:"bbox"`
}

type FontDecode struct {
	Code   string        `json:"code"`
	Glyphs []GlyphDecode `json:"glyphs"`
}

type GlyphDecode struct {
	CID  int    `json:"cid"`
	Text string `json:"text"`
}

type FontUnicode struct {
	Code   string `json:"code"`
	Value  string `json:"value"`
	Mapped bool   `json:"mapped"`
}

type Tag struct {
	Tag              string                 `json:"tag"`
	MCID             int                    `json:"mcid"`
	HasMCID          bool                   `json:"has_mcid"`
	ActualText       string                 `json:"actual_text"`
	Properties       map[string]interface{} `json:"properties"`
	MarkedTag        string                 `json:"marked_tag"`
	MarkedProperties map[string]interface{} `json:"marked_properties"`
	MarkedStack      []MarkedContext        `json:"marked_stack"`
	PageIndex        int                    `json:"page_index"`
	CTM              [6]float64             `json:"ctm"`
	State            GraphicsStateRecord    `json:"state"`
}

type MarkedContext struct {
	Tag        string                 `json:"tag"`
	MCID       int                    `json:"mcid"`
	HasMCID    bool                   `json:"has_mcid"`
	ActualText string                 `json:"actual_text"`
	Properties map[string]interface{} `json:"properties"`
}

type MarkedSection struct {
	MCID  int      `json:"mcid"`
	Texts []string `json:"texts"`
}

type StructureElement struct {
	Type                 string                 `json:"type"`
	Role                 string                 `json:"role"`
	PageIndex            int                    `json:"page_index"`
	Title                string                 `json:"title"`
	Language             string                 `json:"language"`
	AlternateDescription string                 `json:"alternate_description"`
	ActualText           string                 `json:"actual_text"`
	Abbreviation         string                 `json:"abbreviation"`
	ClassName            string                 `json:"class_name"`
	Attributes           map[string]interface{} `json:"attributes"`
	Contents             []StructureContent     `json:"contents"`
	Children             []StructureElement     `json:"children"`
}

type StructureContent struct {
	Kind    string `json:"kind"`
	MCID    int    `json:"mcid"`
	HasMCID bool   `json:"has_mcid"`
}

type Annotation struct {
	Type       string                 `json:"type"`
	Rect       [4]float64             `json:"rect"`
	BBox       [4]float64             `json:"bbox"`
	PageIndex  int                    `json:"page_index"`
	Contents   string                 `json:"contents"`
	Name       string                 `json:"name"`
	Modified   string                 `json:"modified"`
	Parent     *StructureElement      `json:"parent"`
	Action     *Action                `json:"action"`
	Properties map[string]interface{} `json:"properties"`
}

type Text struct {
	Chars         string              `json:"chars"`
	FontName      string              `json:"font_name"`
	FontSize      float64             `json:"font_size"`
	Size          float64             `json:"size"`
	FontBase      string              `json:"font_base"`
	TextFont      string              `json:"text_font"`
	Matrix        [6]float64          `json:"matrix"`
	TextMatrix    [6]float64          `json:"text_matrix"`
	LineMatrix    [6]float64          `json:"line_matrix"`
	ScalingMatrix [6]float64          `json:"scaling_matrix"`
	Origin        [2]float64          `json:"origin"`
	Displacement  [2]float64          `json:"displacement"`
	Rotation      float64             `json:"rotation"`
	BBox          [4]float64          `json:"bbox"`
	Vertical      bool                `json:"vertical"`
	MCID          int                 `json:"mcid"`
	HasMCID       bool                `json:"has_mcid"`
	PageIndex     int                 `json:"page_index"`
	CTM           [6]float64          `json:"ctm"`
	State         GraphicsStateRecord `json:"state"`
	MarkedStack   []MarkedContext     `json:"marked_stack"`
	Args          []any               `json:"args"`
	Glyphs        []Glyph             `json:"glyphs"`
}

type Glyph struct {
	Text         string              `json:"text"`
	Chars        string              `json:"chars"`
	CID          int                 `json:"cid"`
	FontName     string              `json:"font_name"`
	FontSize     float64             `json:"font_size"`
	Size         float64             `json:"size"`
	FontBase     string              `json:"font_base"`
	TextFont     string              `json:"text_font"`
	Matrix       [6]float64          `json:"matrix"`
	Origin       [2]float64          `json:"origin"`
	Displacement [2]float64          `json:"displacement"`
	BBox         [4]float64          `json:"bbox"`
	Vertical     bool                `json:"vertical"`
	MCID         int                 `json:"mcid"`
	HasMCID      bool                `json:"has_mcid"`
	PageIndex    int                 `json:"page_index"`
	CTM          [6]float64          `json:"ctm"`
	State        GraphicsStateRecord `json:"state"`
	MarkedStack  []MarkedContext     `json:"marked_stack"`
	ChildCount   int                 `json:"child_count"`
}

// ProjectTextArgs converts PDF show arguments to stable raw-byte values.
func ProjectTextArgs(args []core.Object) []any {
	if args == nil {
		return nil
	}
	out := make([]any, 0, len(args))
	for _, arg := range args {
		switch value := arg.(type) {
		case core.String:
			out = append(out, hex.EncodeToString([]byte(value)))
		case core.Number:
			out = append(out, float64(value))
		default:
			out = append(out, fmt.Sprint(value))
		}
	}
	return out
}

// GlyphSnapshot projects one public glyph object without forcing callers to
// materialize a containing text object.
func GlyphSnapshot(document *core.Document, glyph core.GlyphObject, pageIndex int) (Glyph, error) {
	childCount, err := glyph.Len(document)
	if err != nil {
		return Glyph{}, err
	}
	return Glyph{
		Text: glyph.Text(), Chars: glyph.Chars(), CID: glyph.CID(),
		FontName: glyph.FontName(), FontSize: glyph.FontSize(), Size: glyph.Size(),
		FontBase: glyph.FontBase(), TextFont: glyph.TextFont(), Matrix: glyph.Matrix(),
		Origin: glyph.Origin(), Displacement: glyph.Displacement(), BBox: glyph.BBox(),
		Vertical: glyph.Vertical(), MCID: contextMCID(glyph.MCID(), glyph.HasMCID()), HasMCID: glyph.HasMCID(),
		PageIndex: pageIndex, CTM: glyph.GState().CTM(),
		State: projectGraphicsState(glyph.GState()), MarkedStack: projectMarkedStack(glyph.MarkedStackCopy()), ChildCount: childCount,
	}, nil
}

// TextSnapshot projects one public text object and leaves its glyph sequence
// empty so callers can append it incrementally while preserving streaming.
func TextSnapshot(text core.TextObject, pageIndex int) Text {
	return Text{
		Chars: text.Chars(), FontName: text.FontName(), FontSize: text.Size(),
		Size: text.Size(), FontBase: text.FontBase(), TextFont: text.TextFont(),
		Matrix: text.Matrix(), TextMatrix: text.TextMatrix(), LineMatrix: text.LineMatrix(),
		ScalingMatrix: text.ScalingMatrix(), Origin: text.Origin(), Displacement: text.Displacement(),
		// Playa 1.1.0 has no TextObject.rotation attribute; the Python
		// compatibility projection therefore observes its getattr(..., 0)
		// fallback. Keep Go's richer Rotation API out of the shared schema.
		Rotation: 0, BBox: text.BBox(), Vertical: text.Vertical(),
		MCID: contextMCID(text.MCID(), text.HasMCID()), HasMCID: text.HasMCID(), PageIndex: pageIndex,
		CTM: text.GState().CTM(), State: projectGraphicsState(text.GState()),
		MarkedStack: projectMarkedStack(text.MarkedStackCopy()),
		Args:        ProjectTextArgs(text.ArgsCopy()), Glyphs: make([]Glyph, 0),
	}
}

func Snapshot(document *core.Document, options Options) (Report, error) {
	if document == nil {
		return Report{}, fmt.Errorf("playa: compatibility snapshot: nil document")
	}
	sections := options.Sections
	if len(sections) == 0 {
		sections = allSnapshotSections()
	}
	pages, err := document.CollectPages()
	if err != nil {
		return Report{}, fmt.Errorf("playa: compatibility snapshot pages: %w", err)
	}
	indices := options.Pages
	if len(indices) == 0 {
		indices = make([]int, len(pages))
		for index := range pages {
			indices[index] = index
		}
	}

	pageLabels, err := document.PageLabels()
	if err != nil {
		return Report{}, fmt.Errorf("playa: compatibility snapshot page labels: %w", err)
	}
	destinations, err := projectDestinations(document, pages)
	if err != nil {
		return Report{}, fmt.Errorf("playa: compatibility snapshot destinations: %w", err)
	}
	report := Report{
		SchemaVersion:   SchemaVersion,
		PageCount:       len(pages),
		PageLabels:      append([]string(nil), pageLabels...),
		PDFVersion:      document.PDFVersion(),
		IsTagged:        document.IsTagged(),
		IsPrintable:     document.IsPrintable(),
		IsModifiable:    document.IsModifiable(),
		IsExtractable:   document.IsExtractable(),
		Info:            playaDocumentInfo(),
		Catalog:         projectDocumentDict(document.Catalog()),
		Names:           projectDocumentDict(document.Names()),
		Trailer:         playaTrailerProjection(document),
		Destinations:    destinations,
		NeedAppearances: document.NeedAppearances(),
		Pages:           make([]Page, 0, len(indices)),
	}
	pageIndex := pageIndexByRef(pages)
	openAction, err := document.OpenActionWithError()
	if err != nil {
		return Report{}, fmt.Errorf("playa: compatibility snapshot open action: %w", err)
	}
	report.OpenAction = projectAction(openAction, pageIndex)
	pageSections := sections
	projector := NewPageProjector(document)
	if hasSection(sections, "structure") {
		report.Structure = projectStructure(document.StructureTreeSeq(), pageIndex)
	}
	if hasSection(sections, "outline") {
		report.Outline = projectOutline(document.Outline(), pageIndex)
	}
	if hasSection(sections, "forms") {
		report.Forms = projectForms(document.FormFields())
	}
	if hasSection(sections, "fonts") {
		fonts, fontErr := document.Fonts()
		if fontErr != nil {
			return Report{}, fmt.Errorf("playa: compatibility snapshot document fonts: %w", fontErr)
		}
		names := make([]string, 0, len(fonts))
		for name := range fonts {
			names = append(names, name)
		}
		sort.Strings(names)
		report.DocumentFonts = make([]Font, 0, len(names))
		for _, name := range names {
			if font := fonts[name]; font != nil {
				report.DocumentFonts = append(report.DocumentFonts, projectFontValue(name, font))
			}
		}
	}
	if hasSection(sections, "document.objects") {
		objects, objectErr := document.CollectObjects()
		if objectErr != nil {
			return Report{}, fmt.Errorf("playa: compatibility snapshot objects: %w", objectErr)
		}
		report.Objects = make([]ObjectRecord, 0, len(objects))
		for _, object := range objects {
			snapshot, snapshotErr := ObjectSnapshot(document, object)
			if snapshotErr != nil {
				return Report{}, fmt.Errorf("playa: compatibility snapshot object %d: %w", object.Ref().Object, snapshotErr)
			}
			report.Objects = append(report.Objects, snapshot)
		}
	}
	if hasSection(sections, "document.mapping") {
		mapping, mappingErr := collectMapping(document)
		if mappingErr != nil {
			return Report{}, fmt.Errorf("playa: compatibility snapshot mapping: %w", mappingErr)
		}
		report.Mapping = mapping
	}
	if hasSection(sections, "document.tokens") {
		tokens, tokenErr := document.CollectTokens()
		if tokenErr != nil {
			return Report{}, fmt.Errorf("playa: compatibility snapshot tokens: %w", tokenErr)
		}
		report.Tokens = make([]TokenRecord, 0, len(tokens))
		for _, token := range tokens {
			report.Tokens = append(report.Tokens, TokenSnapshot(token))
		}
	}
	if hasSection(sections, "document.buffer") {
		length, digest := document.BufferDigest()
		report.Buffer = BufferRecord{Length: length, SHA256: digest}
	}
	if hasSection(sections, "document.xrefs") {
		xrefs, xrefErr := document.XRefs()
		if xrefErr != nil {
			return Report{}, fmt.Errorf("playa: compatibility snapshot xrefs: %w", xrefErr)
		}
		report.XRefs = XRefSnapshots(xrefs)
	}
	for _, index := range indices {
		if index < 0 || index >= len(pages) {
			return Report{}, fmt.Errorf("playa: compatibility snapshot: page index %d out of range", index)
		}
		item, pageErr := projector.PageSnapshotSections(pages[index], index, pageSections)
		if pageErr != nil {
			return Report{}, pageErr
		}
		report.Pages = append(report.Pages, item)
	}
	return report, nil
}

// SnapshotMetadata projects document-level sections without materializing any
// page. It is used by streaming comparators whose page records are processed
// separately.
func SnapshotMetadata(document *core.Document, sections []string) (Report, error) {
	if document == nil {
		return Report{}, fmt.Errorf("playa: compatibility snapshot metadata: nil document")
	}
	report := Report{SchemaVersion: SchemaVersion}
	pageCount := document.PageCount()
	var pageIndex map[core.Ref]int
	needsPageIndex := false
	for _, section := range sections {
		if section == "document" || section == "destinations" || section == "outline" || section == "structure" {
			needsPageIndex = true
			break
		}
	}
	if needsPageIndex {
		var err error
		pageIndex, err = pageIndexByDocument(document, pageCount)
		if err != nil {
			return Report{}, fmt.Errorf("playa: compatibility snapshot metadata pages: %w", err)
		}
	}
	for _, section := range sections {
		switch section {
		case "document":
			report.PageCount = pageCount
			report.PDFVersion = document.PDFVersion()
			report.IsTagged = document.IsTagged()
			report.IsPrintable = document.IsPrintable()
			report.IsModifiable = document.IsModifiable()
			report.IsExtractable = document.IsExtractable()
			report.Info = playaDocumentInfo()
			report.Catalog = projectDocumentDict(document.Catalog())
			report.Names = projectDocumentDict(document.Names())
			report.Trailer = playaTrailerProjection(document)
			action, err := document.OpenActionWithError()
			if err != nil {
				return Report{}, fmt.Errorf("playa: compatibility snapshot open action: %w", err)
			}
			report.OpenAction = projectAction(action, pageIndex)
			pageLabels, err := document.PageLabels()
			if err != nil {
				return Report{}, fmt.Errorf("playa: compatibility snapshot page labels: %w", err)
			}
			report.PageLabels = append([]string(nil), pageLabels...)
		case "destinations":
			var err error
			report.Destinations, err = projectDestinationsByIndex(document, pageIndex)
			if err != nil {
				return Report{}, fmt.Errorf("playa: compatibility snapshot destinations: %w", err)
			}
		case "forms":
			report.NeedAppearances = document.NeedAppearances()
			report.Forms = projectForms(document.FormFields())
		case "outline":
			report.Outline = projectOutline(document.Outline(), pageIndex)
		case "structure":
			report.Structure = projectStructure(document.StructureTreeSeq(), pageIndex)
		case "document.objects":
			objects, err := document.CollectObjects()
			if err != nil {
				return Report{}, fmt.Errorf("playa: compatibility snapshot objects: %w", err)
			}
			report.Objects = make([]ObjectRecord, 0, len(objects))
			for _, object := range objects {
				snapshot, snapshotErr := ObjectSnapshot(document, object)
				if snapshotErr != nil {
					return Report{}, fmt.Errorf("playa: compatibility snapshot object %d: %w", object.Ref().Object, snapshotErr)
				}
				report.Objects = append(report.Objects, snapshot)
			}
		case "document.mapping":
			mapping, err := collectMapping(document)
			if err != nil {
				return Report{}, fmt.Errorf("playa: compatibility snapshot mapping: %w", err)
			}
			report.Mapping = mapping
		case "document.tokens":
			tokens, err := document.CollectTokens()
			if err != nil {
				return Report{}, fmt.Errorf("playa: compatibility snapshot tokens: %w", err)
			}
			report.Tokens = make([]TokenRecord, 0, len(tokens))
			for _, token := range tokens {
				report.Tokens = append(report.Tokens, TokenSnapshot(token))
			}
		case "document.buffer":
			length, digest := document.BufferDigest()
			report.Buffer = BufferRecord{Length: length, SHA256: digest}
		case "document.xrefs":
			xrefs, err := document.XRefs()
			if err != nil {
				return Report{}, fmt.Errorf("playa: compatibility snapshot xrefs: %w", err)
			}
			report.XRefs = XRefSnapshots(xrefs)
		case "fonts":
			fonts, err := document.Fonts()
			if err != nil {
				return Report{}, fmt.Errorf("playa: compatibility snapshot document fonts: %w", err)
			}
			names := make([]string, 0, len(fonts))
			for name := range fonts {
				names = append(names, name)
			}
			sort.Strings(names)
			report.DocumentFonts = make([]Font, 0, len(names))
			for _, name := range names {
				if font := fonts[name]; font != nil {
					report.DocumentFonts = append(report.DocumentFonts, projectFontValue(name, font))
				}
			}
		}
	}
	return report, nil
}

func XRefSnapshots(tables []core.XRefTable) []XRefRecord {
	if tables == nil {
		return nil
	}
	out := make([]XRefRecord, 0, len(tables))
	for _, table := range tables {
		entries := table.EntriesCopy()
		record := XRefRecord{Kind: table.Kind(), Trailer: projectDocumentDict(table.TrailerCopy()), Entries: make([]XRefEntryRecord, 0, len(entries))}
		for _, entry := range entries {
			item := XRefEntryRecord{Object: entry.Object(), Generation: entry.Generation(), Offset: entry.Offset(), InObjectStream: entry.InObjectStream()}
			if entry.InObjectStream() {
				stream, index := entry.ObjectStream(), entry.ObjectIndex()
				item.ObjectStream = &stream
				item.ObjectIndex = &index
			}
			record.Entries = append(record.Entries, item)
		}
		out = append(out, record)
	}
	return out
}

func hasSection(sections []string, want string) bool {
	for _, section := range sections {
		if section == want {
			return true
		}
	}
	return false
}

// WriteStructureJSON writes the structure projection incrementally. It keeps
// only one element and its current child/content traversal live at a time.
func WriteStructureJSON(document *core.Document, writer io.Writer) error {
	if document == nil {
		return fmt.Errorf("playa: compatibility structure: nil document")
	}
	pageCount := document.PageCount()
	pageIndex, err := pageIndexByDocument(document, pageCount)
	if err != nil {
		return err
	}
	// The page index only retains refs and integer positions. Drop the page
	// dictionaries used to build it before traversing the structure tree.
	document.ReleaseTransientCaches()
	if _, err := io.WriteString(writer, "["); err != nil {
		return err
	}
	first := true
	for element, elementErr := range document.StructureTreeSeq() {
		if elementErr != nil {
			continue
		}
		if !first {
			if _, err := io.WriteString(writer, ","); err != nil {
				return err
			}
		}
		first = false
		if err := writeStructureElement(writer, element, pageIndex); err != nil {
			return err
		}
	}
	_, err = io.WriteString(writer, "]")
	return err
}

func writeStructureElement(writer io.Writer, element core.StructElement, pageIndex map[core.Ref]int) error {
	page := -1
	if element.HasPage() {
		if index, ok := pageIndex[element.Page()]; ok {
			page = index
		}
	}
	fields := []struct {
		name  string
		value any
	}{
		{"type", element.StructureType()}, {"role", element.Role()}, {"page_index", page},
		{"title", element.Title()}, {"language", element.Language()},
		{"alternate_description", element.AlternateDescription()}, {"actual_text", element.ActualText()},
		{"abbreviation", element.AbbreviationExpansion()}, {"class_name", element.ClassName()},
		{"attributes", projectDict(element.AttributesCopy())},
	}
	if _, err := io.WriteString(writer, "{"); err != nil {
		return err
	}
	for index, field := range fields {
		if index > 0 {
			if _, err := io.WriteString(writer, ","); err != nil {
				return err
			}
		}
		if err := writeJSONField(writer, field.name, field.value); err != nil {
			return err
		}
	}
	if _, err := io.WriteString(writer, ",\"contents\":"); err != nil {
		return err
	}
	if err := writeStructureContents(writer, element.ContentsSeq()); err != nil {
		return err
	}
	if _, err := io.WriteString(writer, ",\"children\":"); err != nil {
		return err
	}
	if err := writeStructureChildren(writer, element.ChildrenSeq(), pageIndex); err != nil {
		return err
	}
	_, err := io.WriteString(writer, "}")
	return err
}

func writeStructureContents(writer io.Writer, seq iter.Seq2[core.StructureContent, error]) error {
	if _, err := io.WriteString(writer, "["); err != nil {
		return err
	}
	first := true
	for content, contentErr := range seq {
		if contentErr != nil {
			continue
		}
		if !first {
			if _, err := io.WriteString(writer, ","); err != nil {
				return err
			}
		}
		first = false
		if err := writeStructureContent(writer, content); err != nil {
			return err
		}
	}
	_, err := io.WriteString(writer, "]")
	return err
}

func writeStructureContent(writer io.Writer, content core.StructureContent) error {
	if _, err := io.WriteString(writer, `{"kind":`); err != nil {
		return err
	}
	if err := writeJSONValue(writer, string(content.Kind())); err != nil {
		return err
	}
	if _, err := io.WriteString(writer, `,"mcid":`); err != nil {
		return err
	}
	if err := writeJSONValue(writer, contentMCID(content)); err != nil {
		return err
	}
	if _, err := io.WriteString(writer, `,"has_mcid":`); err != nil {
		return err
	}
	if err := writeJSONValue(writer, content.HasMCID()); err != nil {
		return err
	}
	_, err := io.WriteString(writer, "}")
	return err
}

func writeStructureChildren(writer io.Writer, seq iter.Seq2[core.StructElement, error], pageIndex map[core.Ref]int) error {
	if _, err := io.WriteString(writer, "["); err != nil {
		return err
	}
	first := true
	for child, childErr := range seq {
		if childErr != nil {
			continue
		}
		if !first {
			if _, err := io.WriteString(writer, ","); err != nil {
				return err
			}
		}
		first = false
		if err := writeStructureElement(writer, child, pageIndex); err != nil {
			return err
		}
	}
	_, err := io.WriteString(writer, "]")
	return err
}

func writeJSONField(writer io.Writer, name string, value any) error {
	key, err := json.Marshal(name)
	if err != nil {
		return err
	}
	if _, err := writer.Write(key); err != nil {
		return err
	}
	if _, err := io.WriteString(writer, ":"); err != nil {
		return err
	}
	return writeJSONValue(writer, value)
}

func writeJSONValue(writer io.Writer, value any) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	_, err = writer.Write(data)
	return err
}

func contentMCID(content core.StructureContent) int {
	if !content.HasMCID() {
		return 0
	}
	return content.MCID()
}

// SnapshotJSONL writes the compatibility projection without retaining all
// pages in memory. The first record contains document-level metadata and each
// following record contains one projected page.
func SnapshotJSONL(document *core.Document, options Options, writer io.Writer) error {
	if document == nil {
		return fmt.Errorf("playa: compatibility snapshot: nil document")
	}
	pageCount := document.PageCount()
	pageIndex, err := pageIndexByDocument(document, pageCount)
	if err != nil {
		return fmt.Errorf("playa: compatibility snapshot pages: %w", err)
	}
	indices := options.Pages
	if len(indices) == 0 {
		indices = make([]int, pageCount)
		for index := range indices {
			indices[index] = index
		}
	}
	pageSections := options.Sections
	if len(pageSections) == 0 {
		pageSections = allPageSections()
	}
	projector := NewPageProjector(document)
	pageLabels, err := document.PageLabels()
	if err != nil {
		return fmt.Errorf("playa: compatibility snapshot page labels: %w", err)
	}
	destinations, err := projectDestinationsByIndex(document, pageIndex)
	if err != nil {
		return fmt.Errorf("playa: compatibility snapshot destinations: %w", err)
	}
	report := Report{
		SchemaVersion:   SchemaVersion,
		PDFVersion:      document.PDFVersion(),
		IsTagged:        document.IsTagged(),
		IsPrintable:     document.IsPrintable(),
		IsModifiable:    document.IsModifiable(),
		IsExtractable:   document.IsExtractable(),
		PageCount:       pageCount,
		PageLabels:      append([]string(nil), pageLabels...),
		Info:            playaDocumentInfo(),
		Catalog:         projectDocumentDict(document.Catalog()),
		Names:           projectDocumentDict(document.Names()),
		Trailer:         playaTrailerProjection(document),
		OpenAction:      nil,
		Destinations:    destinations,
		NeedAppearances: document.NeedAppearances(),
	}
	action, err := document.OpenActionWithError()
	if err != nil {
		return fmt.Errorf("playa: compatibility snapshot open action: %w", err)
	}
	report.OpenAction = projectAction(action, pageIndex)
	if hasSection(options.Sections, "document.buffer") {
		length, digest := document.BufferDigest()
		report.Buffer = BufferRecord{Length: length, SHA256: digest}
	}
	if hasSection(options.Sections, "document.xrefs") {
		xrefs, xrefErr := document.XRefs()
		if xrefErr != nil {
			return fmt.Errorf("playa: compatibility snapshot xrefs: %w", xrefErr)
		}
		report.XRefs = XRefSnapshots(xrefs)
	}
	report.Structure = projectStructure(document.StructureTreeSeq(), pageIndex)
	report.Outline = projectOutline(document.Outline(), pageIndex)
	report.Forms = projectForms(document.FormFields())
	// Header fields own their projected values; the source-side caches can be
	// released before the first page is interpreted.
	document.ReleaseTransientCaches()
	encoder := json.NewEncoder(writer)
	if err := encoder.Encode(struct {
		Kind     string `json:"kind"`
		Snapshot Report `json:"snapshot"`
	}{Kind: "header", Snapshot: report}); err != nil {
		return fmt.Errorf("playa: encode compatibility snapshot header: %w", err)
	}
	if hasSection(options.Sections, "document.mapping") {
		if _, err := WriteMappingJSONL(document, writer); err != nil {
			return fmt.Errorf("playa: encode compatibility mapping: %w", err)
		}
	}
	if hasSection(options.Sections, "document.objects") {
		if _, err := WriteObjectsJSONL(document, writer); err != nil {
			return fmt.Errorf("playa: encode compatibility objects: %w", err)
		}
	}
	if hasSection(options.Sections, "document.tokens") {
		if _, err := WriteTokensJSONL(document, writer); err != nil {
			return fmt.Errorf("playa: encode compatibility tokens: %w", err)
		}
	}
	for _, index := range indices {
		if index < 0 || index >= pageCount {
			return fmt.Errorf("playa: compatibility snapshot: page index %d out of range", index)
		}
		page, pageErr := document.PageAt(index)
		if pageErr != nil {
			return fmt.Errorf("playa: compatibility snapshot page %d: %w", index, pageErr)
		}
		projection, projectionErr := projector.PageSnapshotSections(page, index, pageSections)
		if projectionErr != nil {
			return projectionErr
		}
		if err := encoder.Encode(struct {
			Kind string `json:"kind"`
			Page Page   `json:"page"`
		}{Kind: "page", Page: projection}); err != nil {
			return fmt.Errorf("playa: encode compatibility page %d: %w", index, err)
		}
		document.ReleaseTransientCaches()
	}
	return nil
}

func allPageSections() []string {
	return []string{"pages", "content.text", "content.extract_text", "content.extract_text.tagged", "content.extract_text.untagged", "content.glyphs", "content.flatten", "content.interp", "content.streams", "content.tokens", "content.xobjects", "content.contents", "layout", "content.paths", "content.images", "fonts", "content.tags", "content.marked", "content.structure", "annotations"}
}

func allSnapshotSections() []string {
	return append([]string{"document", "document.objects", "document.mapping", "document.tokens", "document.buffer", "document.xrefs", "destinations", "outline", "forms", "structure"}, allPageSections()...)
}

// PageSnapshot projects one page and owns only the objects needed for that page.
func PageSnapshot(document *core.Document, page core.Page, index int) (Page, error) {
	return PageSnapshotSections(document, page, index, allPageSections())
}

// PageProjector retains only small lexical indexes shared by page projections
// for one document. It avoids rebuilding xref metadata for every requested
// page or section while never copying the complete PDF buffer.
type PageProjector struct {
	document *core.Document
	source   *playaSourceResolver
}

func NewPageProjector(document *core.Document) *PageProjector {
	return &PageProjector{document: document, source: &playaSourceResolver{document: document}}
}

func (projector *PageProjector) PageSnapshotSections(page core.Page, index int, sections []string) (Page, error) {
	return pageSnapshotSections(projector.document, page, index, sections, projector.source)
}

// PageSnapshotSections projects only the page domains requested by sections.
// This keeps compatibility checks lazy: selecting text must not decode paths,
// images, fonts, tags, or annotations from the same page.
func PageSnapshotSections(document *core.Document, page core.Page, index int, sections []string) (Page, error) {
	return NewPageProjector(document).PageSnapshotSections(page, index, sections)
}

func pageSnapshotSections(document *core.Document, page core.Page, index int, sections []string, source *playaSourceResolver) (Page, error) {
	has := make(map[string]bool, len(sections))
	for _, section := range sections {
		has[section] = true
	}
	needText := has["content.text"]
	needGlyphs := has["content.glyphs"]
	needFlatten := has["content.flatten"]
	needInterp := has["content.interp"]
	needStreams := has["content.streams"]
	needTokens := has["content.tokens"]
	needXObjects := has["content.xobjects"]
	needContents := has["content.contents"]
	needExtractText := has["content.extract_text"]
	needExtractTextTagged := has["content.extract_text.tagged"]
	needExtractTextUntagged := has["content.extract_text.untagged"]
	needStructure := has["content.structure"]
	needLayout := has["layout"]
	var width, height float64
	if has["pages"] {
		width, height = page.Size(document)
	}
	// Consume the page's lazy text iterator instead of materializing the
	// complete expanded operator stream. The projection only needs text and
	// glyph values, while retaining all expanded ContentOp values makes large
	// documents grow substantially across the page loop.
	texts := make([]Text, 0)
	glyphs := make([]Glyph, 0)
	if needText || needGlyphs {
		for text, textErr := range page.Texts(document) {
			if textErr != nil {
				return Page{}, fmt.Errorf("playa: compatibility snapshot page %d text: %w", index, textErr)
			}
			compatible := TextSnapshot(text, index)
			if needText || needGlyphs {
				for glyph, glyphErr := range text.GlyphsSeq() {
					if glyphErr != nil {
						return Page{}, fmt.Errorf("playa: compatibility snapshot page %d glyphs: %w", index, glyphErr)
					}
					projectedGlyph, glyphSnapshotErr := GlyphSnapshot(document, glyph, index)
					if glyphSnapshotErr != nil {
						return Page{}, fmt.Errorf("playa: compatibility snapshot page %d glyph children: %w", index, glyphSnapshotErr)
					}
					if needText {
						compatible.Glyphs = append(compatible.Glyphs, projectedGlyph)
					}
					if needGlyphs {
						glyphs = append(glyphs, projectedGlyph)
					}
				}
			}
			if needText {
				texts = append(texts, compatible)
			}
		}
	}
	annotations := []core.Annotation{}
	if has["annotations"] {
		var err error
		annotations, err = document.CollectAnnotations(page)
		if err != nil {
			return Page{}, fmt.Errorf("playa: compatibility snapshot page %d annotations: %w", index, err)
		}
	}
	paths := make([]Path, 0)
	if has["content.paths"] {
		for path, pathErr := range document.PagePathsSeq(page) {
			if pathErr != nil {
				return Page{}, fmt.Errorf("playa: compatibility snapshot page %d paths: %w", index, pathErr)
			}
			paths = append(paths, projectPath(path, index))
		}
	}
	tags := make([]core.TagObject, 0)
	if has["content.tags"] {
		for tag, tagErr := range document.PageTagsSeq(page) {
			if tagErr != nil {
				return Page{}, fmt.Errorf("playa: compatibility snapshot page %d tags: %w", index, tagErr)
			}
			// Playa's page.tags projection excludes point tags without an MCID.
			if tag.HasMCID() || tag.MarkedTag() != "P" {
				tags = append(tags, tag)
			}
		}
	}
	structure := make([]StructureElement, 0)
	if needStructure {
		var structureErr error
		structure, structureErr = pageStructureProjection(document, page, index)
		if structureErr != nil {
			return Page{}, fmt.Errorf("playa: compatibility snapshot page %d structure: %w", index, structureErr)
		}
	}
	images := make([]core.ImageObject, 0)
	if has["content.images"] {
		for image, imageErr := range document.PageImagesSeq(page) {
			if imageErr != nil {
				return Page{}, fmt.Errorf("playa: compatibility snapshot page %d images: %w", index, imageErr)
			}
			images = append(images, image)
		}
	}
	fonts := make([]core.FontResource, 0)
	if has["fonts"] {
		for font, fontErr := range document.PageFontSeq(page) {
			if fontErr != nil {
				return Page{}, fmt.Errorf("playa: compatibility snapshot page %d fonts: %w", index, fontErr)
			}
			fonts = append(fonts, font)
		}
	}
	marked := []MarkedSection{}
	if has["content.marked"] {
		var err error
		marked, err = projectMarkedSectionsFromPage(document, page)
		if err != nil {
			return Page{}, fmt.Errorf("playa: compatibility snapshot page %d marked content: %w", index, err)
		}
	}
	flatten := make([]ContentRecord, 0)
	if needFlatten {
		for object, objectErr := range page.Flatten(document, core.DefaultContentOptions()) {
			if objectErr != nil {
				return Page{}, fmt.Errorf("playa: compatibility snapshot page %d flatten: %w", index, objectErr)
			}
			record, recordErr := ContentSnapshot(document, object)
			if recordErr != nil {
				return Page{}, fmt.Errorf("playa: compatibility snapshot page %d flatten object: %w", index, recordErr)
			}
			flatten = append(flatten, record)
		}
	}
	interp := make([]ContentRecord, 0)
	if needInterp {
		for object, objectErr := range page.Interp(document, core.DefaultContentOptions()) {
			if objectErr != nil {
				return Page{}, fmt.Errorf("playa: compatibility snapshot page %d interp: %w", index, objectErr)
			}
			record, recordErr := ContentSnapshot(document, object)
			if recordErr != nil {
				return Page{}, fmt.Errorf("playa: compatibility snapshot page %d interp object: %w", index, recordErr)
			}
			interp = append(interp, record)
		}
	}
	streams := make([]PageStreamRecord, 0)
	if needStreams {
		for stream, streamErr := range page.Streams(document) {
			if streamErr != nil {
				return Page{}, fmt.Errorf("playa: compatibility snapshot page %d streams: %w", index, streamErr)
			}
			length, digest, digestErr := stream.DecodedBufferDigestWithDocument(document)
			if digestErr != nil {
				return Page{}, fmt.Errorf("playa: compatibility snapshot page %d stream digest: %w", index, digestErr)
			}
			streams = append(streams, PageStreamRecord{Length: length, SHA256: digest})
		}
	}
	tokens := make([]TokenRecord, 0)
	if needTokens {
		for token, tokenErr := range page.Tokens(document) {
			if tokenErr != nil {
				return Page{}, fmt.Errorf("playa: compatibility snapshot page %d tokens: %w", index, tokenErr)
			}
			tokens = append(tokens, TokenSnapshot(token))
		}
	}
	xobjects := make([]XObjectRecord, 0)
	if needXObjects {
		structurePages := map[core.Ref]int{page.Ref(): index}
		for object, objectErr := range page.XObjects(document) {
			if objectErr != nil {
				return Page{}, fmt.Errorf("playa: compatibility snapshot page %d xobjects: %w", index, objectErr)
			}
			length, digest, digestErr := object.StreamCopy().DecodedBufferDigestWithDocument(document)
			if digestErr != nil {
				return Page{}, fmt.Errorf("playa: compatibility snapshot page %d xobject digest: %w", index, digestErr)
			}
			resources, resourcesErr := object.ResourcesWithError(document)
			if resourcesErr != nil {
				return Page{}, fmt.Errorf("playa: compatibility snapshot page %d xobject resources: %w", index, resourcesErr)
			}
			var resourceProjection map[string]interface{}
			if resources != nil {
				var resourceProjectionErr error
				resourceProjection, resourceProjectionErr = projectResourceGraph(document, resources)
				if resourceProjectionErr != nil {
					return Page{}, fmt.Errorf("playa: compatibility snapshot page %d xobject resource values: %w", index, resourceProjectionErr)
				}
			}
			groupProjection, groupProjectionErr := projectResolvedObject(document, object.GroupCopy())
			if groupProjectionErr != nil {
				return Page{}, fmt.Errorf("playa: compatibility snapshot page %d xobject group: %w", index, groupProjectionErr)
			}
			structure, structureErr := object.Structure(document)
			if structureErr != nil {
				return Page{}, fmt.Errorf("playa: compatibility snapshot page %d xobject structure: %w", index, structureErr)
			}
			elements, elementsErr := structure.ElementsCopyWithError()
			if elementsErr != nil {
				return Page{}, fmt.Errorf("playa: compatibility snapshot page %d xobject structure elements: %w", index, elementsErr)
			}
			collectStructurePageIndices(document, func(yield func(core.StructElement, error) bool) {
				for _, element := range elements {
					if !yield(element, nil) {
						return
					}
				}
			}, structurePages)
			structureProjection := projectStructure(func(yield func(core.StructElement, error) bool) {
				for _, element := range elements {
					if !yield(element, nil) {
						return
					}
				}
			}, structurePages)
			decoded, decodedErr := object.DecodedBufferWithDocumentWithError(document)
			if decodedErr != nil {
				return Page{}, fmt.Errorf("playa: compatibility snapshot page %d xobject decoded contents: %w", index, decodedErr)
			}
			tokens := playaBoundaryTokensSnapshot(decoded)
			contents := make([]interface{}, 0)
			if needContents || needXObjects {
				contents, decodedErr = playaBoundaryBufferSnapshot(document, decoded, source)
				if decodedErr != nil {
					return Page{}, fmt.Errorf("playa: compatibility snapshot page %d xobject contents: %w", index, decodedErr)
				}
			}
			xobjects = append(xobjects, XObjectRecord{
				Name: object.Name(), BBox: object.BBox(), Resources: resourceProjection, Group: groupProjection,
				Fonts: make([]Font, 0), Structure: structureProjection,
				PageIndex: index, CTM: object.GState().CTM().Mul(object.Matrix()),
				State: projectGraphicsState(object.GState()), MarkedStack: projectMarkedStack(object.MarkedStackCopy()),
				Length: length, SHA256: digest, Tokens: tokens, Contents: contents,
			})
			for font, fontErr := range object.FontsSeq(document) {
				if fontErr != nil {
					return Page{}, fmt.Errorf("playa: compatibility snapshot page %d xobject fonts: %w", index, fontErr)
				}
				if projected, ok := projectFont(font); ok {
					xobjects[len(xobjects)-1].Fonts = append(xobjects[len(xobjects)-1].Fonts, projected)
				}
			}
		}
	}
	contents := make([]interface{}, 0)
	if needContents {
		boundaryContents, recoveredBoundary, boundaryErr := playaBoundaryContentsSnapshot(document, page, source)
		if boundaryErr != nil {
			return Page{}, fmt.Errorf("playa: compatibility snapshot page %d contents boundary recovery: %w", index, boundaryErr)
		}
		if recoveredBoundary {
			contents = boundaryContents
		} else {
			for op, opErr := range page.Contents(document) {
				if opErr != nil {
					return Page{}, fmt.Errorf("playa: compatibility snapshot page %d contents: %w", index, opErr)
				}
				contents = append(contents, ContentOpSnapshot(op)...)
			}
		}
	}
	extractedText := ""
	if needExtractText {
		var extractErr error
		extractedText, extractErr = page.ExtractText(document, textconfig.DefaultOptions())
		if extractErr != nil {
			return Page{}, fmt.Errorf("playa: compatibility snapshot page %d extract text: %w", index, extractErr)
		}
	}
	extractedTextTagged := ""
	if needExtractTextTagged {
		var extractErr error
		extractedTextTagged, extractErr = page.ExtractTextTagged(document, textconfig.DefaultOptions())
		if extractErr != nil {
			return Page{}, fmt.Errorf("playa: compatibility snapshot page %d tagged extract text: %w", index, extractErr)
		}
	}
	extractedTextUntagged := ""
	if needExtractTextUntagged {
		var extractErr error
		extractedTextUntagged, extractErr = page.ExtractTextUntagged(document, textconfig.DefaultOptions())
		if extractErr != nil {
			return Page{}, fmt.Errorf("playa: compatibility snapshot page %d untagged extract text: %w", index, extractErr)
		}
	}
	var layoutResult *LayoutRecord
	if needLayout {
		result, layoutErr := page.Layout(document, layout.DefaultOptions())
		if layoutErr != nil {
			return Page{}, fmt.Errorf("playa: compatibility snapshot page %d layout: %w", index, layoutErr)
		}
		projected := LayoutRecord{Lines: make([]LayoutNode, 0), TextBoxes: make([]LayoutNode, 0), TextGroups: make([]LayoutNode, 0), Items: make([]LayoutItemRecord, 0)}
		for line := range result.LinesSeq() {
			projected.Lines = append(projected.Lines, layoutLineSnapshot(line))
		}
		for box := range result.TextBoxesSeq() {
			projected.TextBoxes = append(projected.TextBoxes, layoutBoxSnapshot(box))
		}
		for group := range result.TextGroupsSeq() {
			projected.TextGroups = append(projected.TextGroups, layoutGroupSnapshot(group))
		}
		for item := range result.ItemsSeq() {
			itemSnapshot, itemErr := layoutItemSnapshot(item)
			if itemErr != nil {
				return Page{}, fmt.Errorf("playa: compatibility snapshot page %d layout item: %w", index, itemErr)
			}
			projected.Items = append(projected.Items, itemSnapshot)
		}
		layoutResult = &projected
	}
	label := ""
	rotation := 0
	var parentKey *int
	if has["pages"] {
		label = page.Label(document)
		rotation = page.Rotation(document)
		if key, present := page.ParentKey(document); present {
			parentKey = &key
		}
	}
	item := Page{Index: index, Label: label, Width: width, Height: height, Rotation: rotation, ParentKey: parentKey, Text: texts, Paths: paths, Images: projectImages(images, index), Fonts: projectFonts(fonts), Tags: projectTags(tags, index), Structure: structure, Marked: marked, Annotations: make([]Annotation, 0, len(annotations)), Flatten: flatten, Interp: interp, Streams: streams, Tokens: tokens, XObjects: xobjects, Contents: contents, ExtractText: extractedText, ExtractTextTagged: extractedTextTagged, ExtractTextUntagged: extractedTextUntagged, Glyphs: glyphs, Layout: layoutResult}
	for _, annotation := range annotations {
		projected, projectionErr := AnnotationSnapshotWithDocument(document, annotation, page.Ref(), index)
		if projectionErr != nil {
			return Page{}, fmt.Errorf("playa: compatibility snapshot page %d annotation: %w", index, projectionErr)
		}
		item.Annotations = append(item.Annotations, projected)
	}
	return item, nil
}

// Layout text follows the same trailing-newline normalization as
// layout_node_projection in the Python oracle adapter. Raw glyph/text
// projections retain their original characters.
func layoutLineSnapshot(line core.TextLine) LayoutNode {
	return LayoutNode{Kind: "line", Text: strings.TrimRight(line.Text(), "\n"), BBox: line.BBox(), Vertical: line.Vertical()}
}

func layoutBoxSnapshot(box core.TextBox) LayoutNode {
	children := make([]LayoutNode, 0)
	for line := range box.LinesSeq() {
		children = append(children, layoutLineSnapshot(line))
	}
	index := box.Index()
	return LayoutNode{Kind: "textbox", Text: strings.TrimRight(box.Text(), "\n"), BBox: box.BBox(), Vertical: box.Vertical(), Index: &index, Children: children}
}

func layoutGroupSnapshot(group core.TextGroup) LayoutNode {
	children := make([]LayoutNode, 0)
	for child := range group.ChildrenSeq() {
		if box, ok := child.BoxBorrowed(); ok {
			children = append(children, layoutBoxSnapshot(box))
			continue
		}
		if nested, ok := child.GroupBorrowed(); ok {
			children = append(children, layoutGroupSnapshot(nested))
		}
	}
	return LayoutNode{Kind: "textgroup", Text: strings.TrimRight(group.Text(), "\n"), BBox: group.BBox(), Vertical: group.Vertical(), Children: children}
}

func layoutItemSnapshot(item core.LayoutItem) (LayoutItemRecord, error) {
	record := LayoutItemRecord{Kind: string(item.Kind())}
	if item.Kind() == core.LayoutTextBox {
		box := item.TextBoxBorrowed()
		if box == nil {
			return LayoutItemRecord{}, fmt.Errorf("text-box layout item has no text box payload")
		}
		record.BBox = box.BBox()
		record.HasBBox = true
		node := layoutBoxSnapshot(*box)
		record.TextBox = &node
		return record, nil
	}
	content := item.ContentBorrowed()
	if content == nil {
		return LayoutItemRecord{}, fmt.Errorf("%s layout item has no content payload", item.Kind())
	}
	record.BBox, record.HasBBox = content.BBoxValue()
	return record, nil
}

func ContentSnapshot(document *core.Document, object core.ContentObject) (ContentRecord, error) {
	bbox, hasBBox := object.BBoxValue()
	matrix, hasMatrix := object.MatrixValue()
	mcid, hasMCID := object.MCIDValue()
	childCount, err := object.Len(document)
	if err != nil {
		return ContentRecord{}, err
	}
	pageIndex := -1
	var pageRef core.Ref
	if page, pageErr := object.PageObject(document); pageErr == nil {
		pageIndex = page.Index()
		pageRef = page.Ref()
	}
	parent, err := object.ParentWithError(document)
	if err != nil {
		return ContentRecord{}, err
	}
	var parentProjection *StructureElement
	if parent != nil {
		parentPages := map[core.Ref]int{pageRef: pageIndex}
		parentSeq := func(yield func(core.StructElement, error) bool) {
			yield(*parent, nil)
		}
		collectStructurePageIndices(document, parentSeq, parentPages)
		projected := projectStructure(parentSeq, parentPages)
		if len(projected) > 0 {
			parentProjection = &projected[0]
		}
	}
	record := ContentRecord{Kind: object.ObjectType(), BBox: bbox, HasBBox: hasBBox, Matrix: matrix, HasMatrix: hasMatrix, PageIndex: pageIndex, MCID: contextMCID(mcid, hasMCID), HasMCID: hasMCID, MarkedStack: projectMarkedStack(object.MarkedStackCopy()), State: projectGraphicsState(object.GraphicsStateCopy()), Parent: parentProjection, ChildCount: childCount}
	if object.ObjectType() == "text" {
		text := object.TextBorrowed()
		if text != nil {
			if err := text.ValidateWithError(); err != nil {
				return ContentRecord{}, err
			}
			record.Text = text.Chars()
			record.GlyphCount = text.Len()
		}
	}
	return record, nil
}

func projectColor(color geometry.Color) ColorRecord {
	return ColorRecord{Space: color.Space(), Values: color.ValuesCopy(), Pattern: color.Pattern(), Components: color.Components()}
}

func projectGraphicsState(state core.GraphicsState) GraphicsStateRecord {
	blendMode := state.BlendMode()
	if blendMode == "" {
		blendMode = "Normal"
	}
	blackPointComp := state.BlackPointComp()
	if blackPointComp == "" {
		blackPointComp = "Default"
	}
	return GraphicsStateRecord{
		LineWidth: state.LineWidth(), LineCap: state.LineCap(), LineJoin: state.LineJoin(),
		MiterLimit: state.MiterLimit(), Dash: append([]float64{}, state.DashCopy()...), DashPhase: state.DashPhase(),
		Intent: state.Intent(), StrokeAdjustment: state.StrokeAdjustment(), BlendMode: blendMode,
		StrokeAlpha: state.StrokeAlpha(), FillAlpha: state.FillAlpha(), AlphaSource: state.AlphaSource(),
		BlackPointComp: blackPointComp, Flatness: state.Flatness(),
		StrokeColor: projectColor(state.StrokeColor()), FillColor: projectColor(state.FillColor()),
		HasFont: state.FontName() != "" || state.FontSize() != 0, FontSize: state.FontSize(),
		CharacterSpacing: state.CharacterSpacing(), WordSpacing: state.WordSpacing(),
		Scaling: state.HorizontalScale() * 100, Leading: state.Leading(),
		RenderMode: state.RenderMode(), Rise: state.CharacterRise(), Knockout: state.Knockout(),
		HasClip: state.ClipDepth() > 0,
	}
}

func projectMarkedStack(stack []core.MarkedContentContext) []MarkedContext {
	out := make([]MarkedContext, 0, len(stack))
	for _, context := range stack {
		out = append(out, MarkedContext{
			Tag: context.Tag(), MCID: contextMCID(context.MCID(), context.HasMCID()), HasMCID: context.HasMCID(),
			ActualText: context.ActualText(), Properties: projectDict(context.PropertiesCopy()),
		})
	}
	return out
}

// ContentOpSnapshot flattens the operation-level Go API into the source
// object sequence exposed by Playa's Page.contents and XObject.contents.
// Operands remain lexical values; the operator itself is emitted as the
// trailing keyword for each operation.
func ContentOpSnapshot(op core.ContentOp) []interface{} {
	// Playa's ContentParser returns an InlineImage value for BI...ID...EI,
	// rather than exposing the BI keyword as an operation. The interpreter
	// still needs the explicit BI operation internally, so normalize it only
	// at this compatibility boundary.
	if op.Operator() == "BI" {
		operands := op.OperandsCopy()
		if len(operands) == 1 {
			if _, ok := operands[0].(core.Stream); ok {
				return []interface{}{contentValueSnapshot(operands[0])}
			}
		}
	}
	out := make([]interface{}, 0)
	for operand := range op.OperandsSeq() {
		out = append(out, contentValueSnapshot(operand))
	}
	out = append(out, TokenRecord{Kind: "keyword", Value: op.Operator()})
	return out
}

func contentValueSnapshot(object core.Object) interface{} {
	switch value := object.(type) {
	case nil, core.Null:
		return TokenRecord{Kind: "keyword", Value: "null"}
	case core.Bool:
		if value {
			return TokenRecord{Kind: "number", Value: 1}
		}
		return TokenRecord{Kind: "number", Value: 0}
	case core.Number:
		return TokenRecord{Kind: "number", Value: float64(value)}
	case core.Name:
		return TokenRecord{Kind: "name", Value: string(value)}
	case core.Keyword:
		return TokenSnapshot(core.NewToken(core.TokenKeyword, string(value)))
	case core.String:
		return TokenRecord{Kind: "string", Value: hex.EncodeToString([]byte(value))}
	case core.Array:
		out := make([]interface{}, len(value))
		for index, item := range value {
			out[index] = contentValueSnapshot(item)
		}
		return out
	case core.InvalidArray:
		out := make([]interface{}, len(value))
		for index, item := range value {
			out[index] = contentValueSnapshot(item)
		}
		return out
	case core.Dict:
		out := make(map[string]interface{}, len(value))
		for key, item := range value {
			out[string(key)] = contentValueSnapshot(item)
		}
		return out
	case core.Stream:
		attrs := make(map[string]interface{}, len(value.DictBorrowed()))
		for key, item := range value.DictBorrowed() {
			attrs[string(key)] = contentValueSnapshot(item)
		}
		return map[string]interface{}{
			"kind":   "stream",
			"length": len(value.DecodedBuffer()),
			"attrs":  attrs,
		}
	case core.Ref:
		return value.String()
	default:
		return fmt.Sprint(value)
	}
}

type playaBoundaryValue struct {
	object      core.Object
	delimiter   parserconfig.TokenKind
	none        bool
	numberText  string
	integerText string
	reference   bool
	refID       string
	sequence    []playaBoundaryValue
	isSequence  bool
	dict        map[string]playaBoundaryValue
	isDict      bool
	inline      []interface{}
	isInline    bool
	inlineStart bool
	procStart   bool
}

type playaBoundaryToken struct {
	kind        parserconfig.TokenKind
	text        string
	number      float64
	numberText  string
	integerText string
	value       []byte
}

func (token playaBoundaryToken) Kind() parserconfig.TokenKind { return token.kind }
func (token playaBoundaryToken) Text() string                 { return token.text }
func (token playaBoundaryToken) Number() float64              { return token.number }
func (token playaBoundaryToken) NumberText() string           { return token.numberText }
func (token playaBoundaryToken) IntegerText() string          { return token.integerText }
func (token playaBoundaryToken) ValueCopy() []byte            { return cloneCompatBytes(token.value) }

func cloneCompatBytes(value []byte) []byte {
	if len(value) == 0 {
		return nil
	}
	return primitives.CloneBytes(value)
}

type playaBoundaryParser struct {
	document       *core.Document
	source         *playaSourceResolver
	stack          []playaBoundaryValue
	pendingTop     bool
	recoveryNeeded bool
	rejectRefs     bool
}

type playaSourceResolver struct {
	document      *core.Document
	loaded        bool
	entries       map[string]core.XRefEntry
	values        map[string]playaBoundaryValue
	checked       map[string]bool
	objectStreams map[int]playaCompressedObjectStream
	recovered     bool
	err           error
}

type playaCompressedObjectStream struct {
	values []playaBoundaryValue
	n      int
	ok     bool
}

type playaInlineState uint8

const (
	playaInlineEmitted playaInlineState = iota
	playaInlinePending
	playaInlineExhausted
)

// playaBoundaryContentsSnapshot projects the pinned ObjectParser semantics for
// both ordinary and malformed content. In particular, when a compound object
// is split across streams, ObjectParser's local `top` marker is lost while its
// stack survives, so the unfinished stack is yielded in reverse order before
// parsing the next stream. The public Page.Contents API intentionally keeps
// spec-correct concatenated operator parsing; all oracle-specific lexical and
// recovery behavior therefore remains at this projection boundary.
func playaBoundaryContentsSnapshot(document *core.Document, page core.Page, source *playaSourceResolver) ([]interface{}, bool, error) {
	projected, _, err := scanPlayaBoundaryContents(document, page, source, true)
	return projected, true, err
}

func scanPlayaBoundaryContents(document *core.Document, page core.Page, source *playaSourceResolver, capture bool) ([]interface{}, bool, error) {
	parser := playaBoundaryParser{document: document, source: source, rejectRefs: true}
	projected := make([]interface{}, 0)
	needed := false
	streamIndex := 0
	for stream, streamErr := range playaPageContentStreams(document, page) {
		if streamErr != nil {
			return nil, false, streamErr
		}
		if streamIndex > 0 && (len(parser.stack) > 0 || parser.pendingTop) {
			needed = true
			for len(parser.stack) > 0 {
				last := len(parser.stack) - 1
				if capture {
					projected = appendPlayaBoundaryValue(projected, parser.stack[last])
				}
				parser.stack = parser.stack[:last]
			}
			parser.pendingTop = false
		}
		decoded, decodeErr := stream.DecodedBufferWithDocumentWithError(document)
		if decodeErr != nil {
			return nil, false, decodeErr
		}
		lexer := core.NewLexer(decoded)
		for {
			value, eof, err := parser.next(lexer, capture)
			if err != nil {
				return nil, false, err
			}
			if eof {
				break
			}
			if capture {
				projected = appendPlayaBoundaryValue(projected, value)
			}
		}
		streamIndex++
	}
	if len(parser.stack) > 0 || parser.pendingTop {
		// Pinned Playa discards a compound object left open at final EOF.
		needed = true
	}
	needed = needed || parser.recoveryNeeded
	return projected, needed, nil
}

// playaPageContentStreams mirrors ContentParser's stream_value loop: a page
// Contents array may contain null or another non-stream object, which Playa
// skips while continuing with later streams.
func playaPageContentStreams(document *core.Document, page core.Page) iter.Seq2[core.Stream, error] {
	return func(yield func(core.Stream, error) bool) {
		for stream, err := range page.Streams(document) {
			if !yield(stream, err) || err != nil {
				return
			}
		}
	}
}

func playaBoundaryBufferSnapshot(document *core.Document, decoded []byte, source *playaSourceResolver) ([]interface{}, error) {
	parser := playaBoundaryParser{document: document, source: source}
	projected := make([]interface{}, 0)
	lexer := core.NewLexer(decoded)
	for {
		value, eof, err := parser.next(lexer, true)
		if err != nil {
			return nil, err
		}
		if eof {
			return projected, nil
		}
		projected = appendPlayaBoundaryValue(projected, value)
	}
}

func playaBoundaryTokensSnapshot(decoded []byte) []TokenRecord {
	lexer := core.NewLexer(decoded)
	tokens := make([]TokenRecord, 0)
	for {
		token, err := nextPlayaBoundaryToken(lexer)
		if err != nil || token.Kind() == core.TokenEOF {
			return tokens
		}
		tokens = append(tokens, playaBoundaryTokenSnapshot(token))
	}
}

func playaBoundaryTokenSnapshot(token playaBoundaryToken) TokenRecord {
	record := TokenRecord{Kind: tokenKindName(token.Kind())}
	switch token.Kind() {
	case core.TokenNumber:
		record.Value = json.Number(canonicalNumberText(token.NumberText()))
	case core.TokenName, core.TokenKeyword:
		if token.Kind() == core.TokenKeyword && (token.Text() == "true" || token.Text() == "false") {
			record.Kind = "number"
			if token.Text() == "true" {
				record.Value = 1
			} else {
				record.Value = 0
			}
		} else {
			record.Value = token.Text()
		}
	case core.TokenString, core.TokenHexString:
		record.Kind = "string"
		record.Value = hex.EncodeToString(token.ValueCopy())
	}
	return record
}

func (parser *playaBoundaryParser) next(lexer *core.Lexer, capture bool) (playaBoundaryValue, bool, error) {
	top := false
	for {
		if len(parser.stack) > 0 && !top {
			last := len(parser.stack) - 1
			value := parser.stack[last]
			parser.stack = parser.stack[:last]
			return value, false, nil
		}
		token, err := nextPlayaBoundaryToken(lexer)
		if err != nil {
			return playaBoundaryValue{}, false, err
		}
		if token.Kind() == core.TokenEOF {
			parser.pendingTop = top
			return playaBoundaryValue{}, true, nil
		}
		switch token.Kind() {
		case core.TokenArrayStart, core.TokenDictStart:
			if !top {
				top = true
			}
			parser.stack = append(parser.stack, playaBoundaryValue{delimiter: token.Kind()})
		case core.TokenArrayEnd:
			values, matched := parser.popTo(core.TokenArrayStart)
			value := playaBoundaryValue{none: !matched}
			if matched {
				value = playaBoundaryValue{sequence: values, isSequence: true}
			}
			if matched && len(parser.stack) == 0 && top {
				return value, false, nil
			}
			parser.stack = append(parser.stack, value)
		case core.TokenDictEnd:
			values, matched := parser.popTo(core.TokenDictStart)
			for _, item := range values {
				if parser.rejectRefs && playaBoundaryValueHasReference(item) {
					return playaBoundaryValue{}, false, fmt.Errorf("playa: indirect object reference is not permitted in a content stream")
				}
			}
			value := playaBoundaryValue{none: true}
			if matched && len(values)%2 == 0 {
				dict := make(map[string]playaBoundaryValue, len(values)/2)
				valid := true
				for index := 0; index < len(values); index += 2 {
					if values[index+1].none {
						continue
					}
					key, ok := values[index].object.(core.Name)
					if !ok {
						valid = false
						break
					}
					dict[string(key)] = values[index+1]
				}
				if valid {
					value = playaBoundaryValue{dict: dict, isDict: true}
				}
			}
			if matched && len(parser.stack) == 0 && top {
				return value, false, nil
			}
			parser.stack = append(parser.stack, value)
		default:
			if token.Kind() == core.TokenKeyword && token.Text() == "{" {
				if !top {
					top = true
				}
				parser.stack = append(parser.stack, playaBoundaryValue{procStart: true})
				continue
			}
			if token.Kind() == core.TokenKeyword && token.Text() == "}" {
				values, matched := parser.popToProcedure()
				value := playaBoundaryValue{none: !matched}
				if matched {
					value = playaBoundaryValue{sequence: values, isSequence: true}
				}
				if matched && len(parser.stack) == 0 && top {
					return value, false, nil
				}
				parser.stack = append(parser.stack, value)
				continue
			}
			if token.Kind() == core.TokenKeyword && token.Text() == "BI" {
				if top {
					return playaBoundaryValue{}, false, fmt.Errorf("playa: inline image inside compound content object")
				}
				top = true
				parser.stack = append(parser.stack, playaBoundaryValue{inlineStart: true})
				continue
			}
			if token.Kind() == core.TokenKeyword && token.Text() == "ID" {
				values, matched := parser.popToInline()
				if !matched {
					return playaBoundaryValue{}, false, fmt.Errorf("playa: unmatched inline image ID without BI")
				}
				params, paramsErr := playaBoundaryInlineDictionary(values)
				if paramsErr != nil {
					return playaBoundaryValue{}, false, paramsErr
				}
				inline, state, inlineErr := playaBoundaryInlineSnapshot(parser, lexer, params, capture)
				if inlineErr != nil {
					return playaBoundaryValue{}, false, inlineErr
				}
				switch state {
				case playaInlinePending:
					top = true
					continue
				case playaInlineExhausted:
					parser.recoveryNeeded = true
					parser.pendingTop = false
					return playaBoundaryValue{}, true, nil
				}
				return playaBoundaryValue{inline: inline, isInline: true}, false, nil
			}
			value := playaBoundaryTokenValue(token)
			if top && token.Kind() == core.TokenKeyword && token.Text() == "R" {
				value, err = playaBoundaryReference(&parser.stack)
				if err != nil {
					return playaBoundaryValue{}, false, err
				}
			}
			parser.stack = append(parser.stack, value)
		}
	}
}

func playaBoundaryValueHasReference(value playaBoundaryValue) bool {
	if value.reference {
		return true
	}
	for _, item := range value.sequence {
		if playaBoundaryValueHasReference(item) {
			return true
		}
	}
	for _, item := range value.dict {
		if playaBoundaryValueHasReference(item) {
			return true
		}
	}
	return false
}

func (parser *playaBoundaryParser) popTo(delimiter parserconfig.TokenKind) ([]playaBoundaryValue, bool) {
	values := make([]playaBoundaryValue, 0)
	for len(parser.stack) > 0 {
		last := len(parser.stack) - 1
		value := parser.stack[last]
		parser.stack = parser.stack[:last]
		if value.delimiter == delimiter {
			for left, right := 0, len(values)-1; left < right; left, right = left+1, right-1 {
				values[left], values[right] = values[right], values[left]
			}
			return values, true
		}
		values = append(values, value)
	}
	return nil, false
}

func (parser *playaBoundaryParser) popToProcedure() ([]playaBoundaryValue, bool) {
	values := make([]playaBoundaryValue, 0)
	for len(parser.stack) > 0 {
		last := len(parser.stack) - 1
		value := parser.stack[last]
		parser.stack = parser.stack[:last]
		if value.procStart {
			for left, right := 0, len(values)-1; left < right; left, right = left+1, right-1 {
				values[left], values[right] = values[right], values[left]
			}
			return values, true
		}
		values = append(values, value)
	}
	return nil, false
}

func (parser *playaBoundaryParser) popToInline() ([]playaBoundaryValue, bool) {
	values := make([]playaBoundaryValue, 0)
	for len(parser.stack) > 0 {
		last := len(parser.stack) - 1
		value := parser.stack[last]
		parser.stack = parser.stack[:last]
		if value.inlineStart {
			for left, right := 0, len(values)-1; left < right; left, right = left+1, right-1 {
				values[left], values[right] = values[right], values[left]
			}
			return values, true
		}
		values = append(values, value)
	}
	return nil, false
}

func nextPlayaBoundaryToken(lexer *core.Lexer) (playaBoundaryToken, error) {
	data := lexer.DataBorrowed()
	position := lexer.Pos()
	for position < len(data) {
		if playaBoundaryRegexWhitespace(data[position]) {
			position++
			continue
		}
		if data[position] == '%' {
			end := position + 1
			for end < len(data) && data[end] != '\r' && data[end] != '\n' {
				end++
			}
			if end == len(data) {
				lexer.SetPos(position + 1)
				return playaBoundaryToken{kind: core.TokenKeyword, text: "%"}, nil
			}
			position = end + 1
			continue
		}
		break
	}
	lexer.SetPos(position)
	if position >= len(data) {
		return playaBoundaryToken{kind: core.TokenEOF}, nil
	}
	value := data[position]
	switch value {
	case '[':
		lexer.SetPos(position + 1)
		return playaBoundaryToken{kind: core.TokenArrayStart}, nil
	case ']':
		lexer.SetPos(position + 1)
		return playaBoundaryToken{kind: core.TokenArrayEnd}, nil
	case '{', '}':
		lexer.SetPos(position + 1)
		return playaBoundaryToken{kind: core.TokenKeyword, text: string(value)}, nil
	case '/':
		end := position + 1
		for end < len(data) {
			if data[end] == '#' && end+2 < len(data) && playaBoundaryHex(data[end+1]) && playaBoundaryHex(data[end+2]) {
				end += 3
				continue
			}
			if playaBoundaryNotLiteral(data[end]) {
				break
			}
			end++
		}
		lexer.SetPos(end)
		return playaBoundaryToken{kind: core.TokenName, text: playaBoundaryName(data[position+1 : end])}, nil
	case '(':
		value, end, ok := playaBoundaryString(data, position)
		if !ok {
			lexer.SetPos(len(data))
			return playaBoundaryToken{kind: core.TokenEOF}, nil
		}
		lexer.SetPos(end)
		return playaBoundaryToken{kind: core.TokenString, value: value}, nil
	case '<':
		if position+1 < len(data) && data[position+1] == '<' {
			lexer.SetPos(position + 2)
			return playaBoundaryToken{kind: core.TokenDictStart}, nil
		}
		value, end, ok := playaBoundaryHexString(data, position)
		if ok {
			lexer.SetPos(end)
			return playaBoundaryToken{kind: core.TokenHexString, value: value}, nil
		}
		lexer.SetPos(position + 1)
		return playaBoundaryToken{kind: core.TokenKeyword, text: "<"}, nil
	case '>':
		if position+1 < len(data) && data[position+1] == '>' {
			lexer.SetPos(position + 2)
			return playaBoundaryToken{kind: core.TokenDictEnd}, nil
		}
		lexer.SetPos(position + 1)
		return playaBoundaryToken{kind: core.TokenKeyword, text: ">"}, nil
	}
	if end, ok := playaBoundaryNumberEnd(data, position); ok {
		raw := string(data[position:end])
		number, _ := strconv.ParseFloat(raw, 64)
		token := playaBoundaryToken{kind: core.TokenNumber, text: raw, number: number, numberText: raw}
		if !strings.Contains(raw, ".") {
			token.integerText = raw
		}
		lexer.SetPos(end)
		return token, nil
	}
	if playaBoundaryASCIILetter(value) {
		end := position + 1
		for end < len(data) && !playaBoundaryNotKeyword(data[end]) {
			end++
		}
		lexer.SetPos(end)
		return playaBoundaryToken{kind: core.TokenKeyword, text: playaBoundaryLatin1(data[position:end])}, nil
	}
	lexer.SetPos(position + 1)
	return playaBoundaryToken{kind: core.TokenKeyword, text: playaBoundaryLatin1(data[position : position+1])}, nil
}

func playaBoundaryName(raw []byte) string {
	decoded := make([]byte, 0, len(raw))
	for index := 0; index < len(raw); index++ {
		if raw[index] == '#' && index+2 < len(raw) && playaBoundaryHex(raw[index+1]) && playaBoundaryHex(raw[index+2]) {
			decoded = append(decoded, playaBoundaryHexValue(raw[index+1])<<4|playaBoundaryHexValue(raw[index+2]))
			index += 2
			continue
		}
		decoded = append(decoded, raw[index])
	}
	if utf8.Valid(decoded) {
		return string(decoded)
	}
	return playaBoundaryLatin1(decoded)
}

func playaBoundaryLatin1(raw []byte) string {
	runes := make([]rune, len(raw))
	for index, value := range raw {
		runes[index] = rune(value)
	}
	return string(runes)
}

func playaBoundaryString(data []byte, start int) ([]byte, int, bool) {
	value := make([]byte, 0)
	depth := 1
	for position := start + 1; position < len(data); {
		current := data[position]
		switch current {
		case '(':
			depth++
			value = append(value, current)
			position++
		case ')':
			depth--
			position++
			if depth == 0 {
				return value, position, true
			}
			value = append(value, current)
		case '\r':
			value = append(value, '\n')
			position++
			if position < len(data) && data[position] == '\n' {
				position++
			}
		case '\n':
			value = append(value, '\n')
			position++
		case '\\':
			position++
			if position >= len(data) {
				return nil, len(data), false
			}
			escaped := data[position]
			if escaped >= '0' && escaped <= '7' {
				number := 0
				count := 0
				for position < len(data) && count < 3 && data[position] >= '0' && data[position] <= '7' {
					number = number*8 + int(data[position]-'0')
					position++
					count++
				}
				if number < 256 {
					value = append(value, byte(number))
				}
				continue
			}
			if escaped == '\r' || escaped == '\n' {
				position++
				if escaped == '\r' && position < len(data) && data[position] == '\n' {
					position++
				}
				continue
			}
			switch escaped {
			case 'b':
				value = append(value, '\b')
			case 't':
				value = append(value, '\t')
			case 'n':
				value = append(value, '\n')
			case 'f':
				value = append(value, '\f')
			case 'r':
				value = append(value, '\r')
			default:
				value = append(value, escaped)
			}
			position++
		default:
			value = append(value, current)
			position++
		}
	}
	return nil, len(data), false
}

func playaBoundaryHexString(data []byte, start int) ([]byte, int, bool) {
	digits := make([]byte, 0)
	for position := start + 1; position < len(data); position++ {
		if data[position] == '>' {
			if len(digits)%2 != 0 {
				digits = append(digits, '0')
			}
			decoded := make([]byte, len(digits)/2)
			for index := range decoded {
				decoded[index] = playaBoundaryHexValue(digits[index*2])<<4 | playaBoundaryHexValue(digits[index*2+1])
			}
			return decoded, position + 1, true
		}
		if playaBoundaryRegexWhitespace(data[position]) {
			continue
		}
		if !playaBoundaryHex(data[position]) {
			return nil, start + 1, false
		}
		digits = append(digits, data[position])
	}
	return nil, start + 1, false
}

func playaBoundaryHexValue(value byte) byte {
	switch {
	case value >= '0' && value <= '9':
		return value - '0'
	case value >= 'A' && value <= 'F':
		return value - 'A' + 10
	default:
		return value - 'a' + 10
	}
}

func playaBoundaryNumberEnd(data []byte, start int) (int, bool) {
	position := start
	if position < len(data) && (data[position] == '+' || data[position] == '-') {
		position++
	}
	if position < len(data) && data[position] == '.' {
		position++
		firstDigit := position
		for position < len(data) && playaBoundaryDigit(data[position]) {
			position++
		}
		return position, position > firstDigit
	}
	firstDigit := position
	for position < len(data) && playaBoundaryDigit(data[position]) {
		position++
	}
	if position == firstDigit {
		return start, false
	}
	if position < len(data) && data[position] == '.' {
		position++
		for position < len(data) && playaBoundaryDigit(data[position]) {
			position++
		}
	}
	return position, true
}

func playaBoundaryDigit(value byte) bool { return value >= '0' && value <= '9' }

func playaBoundaryASCIILetter(value byte) bool {
	return value >= 'A' && value <= 'Z' || value >= 'a' && value <= 'z'
}

func playaBoundaryNotLiteral(value byte) bool {
	return value == '#' || playaBoundaryNotKeyword(value)
}

func playaBoundaryNotKeyword(value byte) bool {
	return playaBoundaryRegexWhitespace(value) || strings.ContainsRune("#/%[]()<>{}", rune(value))
}

func playaBoundaryHex(value byte) bool {
	return playaBoundaryDigit(value) || value >= 'A' && value <= 'F' || value >= 'a' && value <= 'f'
}

func playaBoundaryReference(stack *[]playaBoundaryValue) (playaBoundaryValue, error) {
	if len(*stack) < 2 {
		return playaBoundaryValue{}, fmt.Errorf("playa: indirect reference stack underflow")
	}
	values := *stack
	object := values[len(values)-2]
	*stack = values[:len(values)-2]
	objectID, ok := playaBoundaryBigInteger(object)
	if !ok || objectID.Sign() == 0 {
		return playaBoundaryValue{none: true}, nil
	}
	return playaBoundaryValue{reference: true, refID: objectID.String()}, nil
}

func playaBoundaryBigInteger(value playaBoundaryValue) (*big.Int, bool) {
	if value.integerText != "" {
		number, ok := new(big.Int).SetString(value.integerText, 10)
		return number, ok
	}
	if boolean, ok := value.object.(core.Bool); ok {
		if boolean {
			return big.NewInt(1), true
		}
		return big.NewInt(0), true
	}
	return nil, false
}

func (parser *playaBoundaryParser) integer(value playaBoundaryValue) (int, bool) {
	seen := make(map[string]bool)
	for value.reference {
		if parser.document == nil || seen[value.refID] {
			return 0, false
		}
		seen[value.refID] = true
		var resolved playaBoundaryValue
		ok := false
		if parser.source != nil {
			resolved, ok = parser.source.objectValue(value.refID)
		}
		if !ok {
			objectID, parsed := new(big.Int).SetString(value.refID, 10)
			if !parsed || !objectID.IsInt64() {
				return 0, false
			}
			integer := int(objectID.Int64())
			if int64(integer) != objectID.Int64() {
				return 0, false
			}
			object, found := parser.document.Get(integer)
			if !found {
				return 0, false
			}
			switch object := object.(type) {
			case core.Ref:
				value = playaBoundaryValue{reference: true, refID: strconv.Itoa(object.Object)}
				continue
			case core.Bool:
				value = playaBoundaryValue{object: object}
			case core.Number:
				// A Number reached only when no source token is available. Its
				// original int/float identity is unknown, so do not guess.
				return 0, false
			default:
				return 0, false
			}
		} else {
			value = resolved
		}
	}
	number, ok := playaBoundaryBigInteger(value)
	if !ok || !number.IsInt64() {
		return 0, false
	}
	integer := int(number.Int64())
	if int64(integer) != number.Int64() {
		return 0, false
	}
	return integer, true
}

func (resolver *playaSourceResolver) objectValue(objectID string) (playaBoundaryValue, bool) {
	if resolver == nil || resolver.document == nil {
		return playaBoundaryValue{}, false
	}
	resolver.ensureLoaded()
	if resolver.checked[objectID] {
		value, ok := resolver.values[objectID]
		return value, ok
	}
	resolver.checked[objectID] = true
	if resolver.err != nil {
		return playaBoundaryValue{}, false
	}
	entry, ok := resolver.entries[objectID]
	if !ok {
		return playaBoundaryValue{}, false
	}
	var value playaBoundaryValue
	if entry.InObjectStream() {
		value, ok = resolver.compressedObjectValue(entry, objectID)
	} else {
		value, ok = resolver.sourceObjectValue(entry, objectID)
	}
	if ok {
		resolver.values[objectID] = value
	}
	return value, ok
}

func (resolver *playaSourceResolver) activeRef(objectID string) (core.Ref, bool) {
	if resolver == nil || resolver.document == nil {
		return core.Ref{}, false
	}
	resolver.ensureLoaded()
	entry, ok := resolver.entries[objectID]
	if !ok {
		return core.Ref{}, false
	}
	objectNumber, err := strconv.Atoi(objectID)
	if err != nil {
		return core.Ref{}, false
	}
	return core.Ref{Object: objectNumber, Generation: entry.Generation()}, true
}

func (resolver *playaSourceResolver) ensureLoaded() {
	if resolver == nil || resolver.document == nil {
		return
	}
	if !resolver.loaded || resolver.recovered != resolver.document.WasRecovered() {
		resolver.load()
	}
}

func (resolver *playaSourceResolver) load() {
	for {
		recovered := resolver.document.WasRecovered()
		entries := make(map[string]core.XRefEntry)
		tables, err := resolver.document.XRefs()
		if recovered != resolver.document.WasRecovered() {
			continue
		}
		resolver.loaded = true
		resolver.recovered = recovered
		resolver.entries = entries
		resolver.values = make(map[string]playaBoundaryValue)
		resolver.checked = make(map[string]bool)
		resolver.objectStreams = make(map[int]playaCompressedObjectStream)
		resolver.err = err
		if err != nil {
			return
		}
		for _, table := range tables {
			for entry := range table.EntriesSeq() {
				key := strconv.Itoa(entry.Object())
				if _, exists := resolver.entries[key]; !exists {
					resolver.entries[key] = entry
				}
			}
		}
		return
	}
}

func (resolver *playaSourceResolver) sourceObjectValue(entry core.XRefEntry, objectID string) (playaBoundaryValue, bool) {
	next, stop := iter.Pull2(resolver.document.TokensFrom(entry.Offset()))
	defer stop()
	for {
		objectToken, err, ok := next()
		if !ok || err != nil {
			return playaBoundaryValue{}, false
		}
		if objectToken.Offset() != entry.Offset() {
			continue
		}
		if objectToken.Kind() != core.TokenNumber || objectToken.NumberText() != objectID {
			return playaBoundaryValue{}, false
		}
		generationToken, err, ok := next()
		if !ok || err != nil || generationToken.Kind() != core.TokenNumber {
			return playaBoundaryValue{}, false
		}
		headerToken, err, ok := next()
		if !ok || err != nil || headerToken.Kind() != core.TokenKeyword || headerToken.Text() != "obj" {
			return playaBoundaryValue{}, false
		}
		return playaBoundarySourceValue(next)
	}
}

func (resolver *playaSourceResolver) compressedObjectValue(entry core.XRefEntry, _ string) (playaBoundaryValue, bool) {
	streamObjects := resolver.compressedObjectStream(entry.ObjectStream())
	objectIndex := entry.ObjectIndex()
	maxInt := int(^uint(0) >> 1)
	minInt := -maxInt - 1
	if !streamObjects.ok || objectIndex < 0 || streamObjects.n > (maxInt-objectIndex)/2 || streamObjects.n < minInt/2 {
		return playaBoundaryValue{}, false
	}
	target := 2*streamObjects.n + objectIndex
	if target < 0 {
		target += len(streamObjects.values)
	}
	if target < 0 || target >= len(streamObjects.values) {
		return playaBoundaryValue{}, false
	}
	return streamObjects.values[target], true
}

func (resolver *playaSourceResolver) compressedObjectStream(objectStream int) playaCompressedObjectStream {
	if resolver.objectStreams == nil {
		resolver.objectStreams = make(map[int]playaCompressedObjectStream)
	}
	if cached, ok := resolver.objectStreams[objectStream]; ok {
		return cached
	}
	// Publish a negative sentinel before parsing so malformed self-references
	// cannot recursively decode the same object stream without bound.
	resolver.objectStreams[objectStream] = playaCompressedObjectStream{}
	object, _ := resolver.document.Get(objectStream)
	stream, ok := object.(core.Stream)
	if !ok {
		return playaCompressedObjectStream{}
	}
	decoded, err := stream.DecodedBufferWithDocumentWithError(resolver.document)
	if err != nil {
		return playaCompressedObjectStream{}
	}
	dict := stream.DictCopy()
	n, nOK := resolver.objectStreamCount(objectStream, dict[core.Name("N")])
	if !nOK {
		return playaCompressedObjectStream{}
	}
	// Pinned Playa advances through the decoded object stream as a sequence of
	// parsed objects and caches that full sequence by object-stream id. It does
	// not seek via /First or validate header ids/offsets.
	valueLexer := core.NewLexer(decoded)
	parser := playaBoundaryParser{document: resolver.document, source: resolver}
	values := make([]playaBoundaryValue, 0)
	for {
		value, eof, parseErr := parser.next(valueLexer, false)
		if parseErr != nil {
			return playaCompressedObjectStream{}
		}
		if eof {
			break
		}
		values = append(values, value)
	}
	result := playaCompressedObjectStream{values: values, n: n, ok: true}
	resolver.objectStreams[objectStream] = result
	return result
}

func (resolver *playaSourceResolver) objectStreamCount(objectStream int, value core.Object) (int, bool) {
	if _, ok := value.(core.Ref); ok {
		return resolver.integerObject(value)
	}
	if _, ok := value.(core.Number); !ok {
		// Pinned Playa catches int_value's TypeError and uses N=0.
		return 0, true
	}
	entry, ok := resolver.entries[strconv.Itoa(objectStream)]
	if !ok || entry.InObjectStream() {
		return 0, false
	}
	if integer, ok := resolver.sourceDictionaryInteger(entry, "N"); ok {
		return integer, true
	}
	// A directly parsed float, including an integral spelling such as 1.0,
	// is not a Python int and therefore makes pinned Playa use N=0.
	return 0, true
}

func (resolver *playaSourceResolver) sourceDictionaryInteger(entry core.XRefEntry, key string) (int, bool) {
	next, stop := iter.Pull2(resolver.document.TokensFrom(entry.Offset()))
	defer stop()
	for index := 0; index < 3; index++ {
		token, err, ok := next()
		if !ok || err != nil {
			return 0, false
		}
		if index < 2 && token.Kind() != core.TokenNumber || index == 2 && (token.Kind() != core.TokenKeyword || token.Text() != "obj") {
			return 0, false
		}
	}
	depth := 0
	for {
		token, err, ok := next()
		if !ok || err != nil {
			return 0, false
		}
		switch token.Kind() {
		case core.TokenDictStart:
			depth++
		case core.TokenDictEnd:
			depth--
			if depth <= 0 {
				return 0, false
			}
		case core.TokenName:
			if depth != 1 || token.Text() != key {
				continue
			}
			value, valueErr, valueOK := next()
			if !valueOK || valueErr != nil || value.Kind() != core.TokenNumber {
				return 0, false
			}
			integer, integerOK := new(big.Int).SetString(value.NumberText(), 10)
			if !integerOK || !integer.IsInt64() {
				return 0, false
			}
			result := int(integer.Int64())
			return result, int64(result) == integer.Int64()
		}
	}
}

func (resolver *playaSourceResolver) integerObject(value core.Object) (int, bool) {
	if ref, ok := value.(core.Ref); ok {
		parser := playaBoundaryParser{document: resolver.document, source: resolver}
		return parser.integer(playaBoundaryValue{reference: true, refID: strconv.Itoa(ref.Object)})
	}
	return core.IntValue(value)
}

func playaBoundarySourceScalar(next func() (core.Token, error, bool)) (playaBoundaryValue, bool) {
	valueToken, err, ok := next()
	if !ok || err != nil {
		return playaBoundaryValue{}, false
	}
	value := playaBoundaryTokenValue(playaBoundaryToken{
		kind: valueToken.Kind(), text: valueToken.Text(), number: valueToken.Number(),
		numberText: valueToken.NumberText(), value: valueToken.ValueCopy(),
	})
	value.delimiter = valueToken.Kind()
	if valueToken.Kind() == core.TokenNumber {
		if integer, ok := new(big.Int).SetString(valueToken.NumberText(), 10); ok {
			value.integerText = integer.String()
		}
	}
	return value, true
}

func playaBoundarySourceScalarUsable(value playaBoundaryValue) bool {
	return value.delimiter != core.TokenArrayStart && value.delimiter != core.TokenDictStart
}

func playaBoundarySourceValue(next func() (core.Token, error, bool)) (playaBoundaryValue, bool) {
	return playaBoundarySourceScalar(next)
}

func playaBoundaryTokenValue(token playaBoundaryToken) playaBoundaryValue {
	switch token.Kind() {
	case core.TokenNumber:
		return playaBoundaryValue{object: core.Number(token.Number()), numberText: token.NumberText(), integerText: token.IntegerText()}
	case core.TokenName:
		return playaBoundaryValue{object: core.Name(token.Text())}
	case core.TokenString, core.TokenHexString:
		return playaBoundaryValue{object: core.String(token.ValueCopy())}
	case core.TokenKeyword:
		switch token.Text() {
		case "null":
			return playaBoundaryValue{none: true}
		case "true":
			return playaBoundaryValue{object: core.Bool(true)}
		case "false":
			return playaBoundaryValue{object: core.Bool(false)}
		default:
			return playaBoundaryValue{object: core.Keyword(token.Text())}
		}
	default:
		return playaBoundaryValue{object: core.Keyword(token.Text())}
	}
}

func appendPlayaBoundaryValue(projected []interface{}, value playaBoundaryValue) []interface{} {
	if value.isInline {
		return append(projected, value.inline...)
	}
	return append(projected, playaBoundaryValueSnapshot(value))
}

func playaBoundaryInlineSnapshot(parser *playaBoundaryParser, lexer *core.Lexer, params map[string]playaBoundaryValue, capture bool) ([]interface{}, playaInlineState, error) {
	data := lexer.DataBorrowed()
	idEnd := lexer.Pos()
	start := playaBoundaryInlineDataStart(data, idEnd, params)
	lengthObject, hasLength := params["L"]
	if !hasLength {
		lengthObject, hasLength = params["Length"]
	}
	if hasLength {
		length, ok := parser.integer(lengthObject)
		if !ok {
			return nil, playaInlinePending, fmt.Errorf("playa: invalid inline image Length")
		}
		if length > len(data)-start {
			lexer.SetPos(len(data))
			return nil, playaInlineExhausted, nil
		}
		end := start + length
		if end >= len(data) {
			lexer.SetPos(len(data))
			return nil, playaInlineExhausted, nil
		}
		markerLexer := core.NewLexer(data)
		markerPosition := end
		if markerPosition < 0 {
			markerPosition = 0
		}
		markerLexer.SetPos(markerPosition)
		marker, err := nextPlayaBoundaryToken(markerLexer)
		if err != nil {
			// Playa's lexer reports an unterminated marker token as
			// StopIteration, ending this content stream without an image.
			lexer.SetPos(len(data))
			return nil, playaInlineExhausted, nil
		}
		if marker.Kind() == core.TokenEOF {
			lexer.SetPos(len(data))
			return nil, playaInlineExhausted, nil
		}
		lexer.SetPos(markerLexer.Pos())
		var imageData []byte
		sliceEnd := end
		if sliceEnd < 0 {
			if sliceEnd < -len(data) {
				sliceEnd = 0
			} else {
				sliceEnd += len(data)
			}
		}
		if sliceEnd > len(data) {
			sliceEnd = len(data)
		}
		if sliceEnd >= start {
			imageData = data[start:sliceEnd]
		}
		return playaBoundaryInlineValue(parser, params, imageData, capture), playaInlineEmitted, nil
	}
	end, resume, found := playaBoundaryInlineEnd(data, start, params)
	if found {
		lexer.SetPos(resume)
		return playaBoundaryInlineValue(parser, params, data[start:end], capture), playaInlineEmitted, nil
	}
	// Pinned Playa returns None and continues lexing immediately after ID
	// when neither the normal terminator nor its furthest-EI salvage matches.
	lexer.SetPos(idEnd)
	return nil, playaInlinePending, nil
}

func playaBoundaryInlineDictionary(values []playaBoundaryValue) (map[string]playaBoundaryValue, error) {
	params := make(map[string]playaBoundaryValue, len(values)/2)
	for index := 0; index+1 < len(values); index += 2 {
		if values[index+1].none {
			continue
		}
		key, ok := values[index].object.(core.Name)
		if !ok {
			return nil, fmt.Errorf("playa: invalid inline image dictionary")
		}
		params[string(key)] = values[index+1]
	}
	return params, nil
}

func playaBoundaryInlineValue(parser *playaBoundaryParser, params map[string]playaBoundaryValue, data []byte, capture bool) []interface{} {
	if !capture {
		return nil
	}
	attrs := make(map[string]interface{}, len(params))
	for key, value := range params {
		attrs[key] = playaBoundaryValueSnapshot(value)
	}
	streamParams := make(core.Dict, 4)
	for key, value := range params {
		switch key {
		case "F", "Filter", "DP", "DecodeParms":
			streamParams[core.Name(key)] = playaBoundaryObject(parser.document, parser.source, value)
		}
	}
	decodeParams := make(core.Dict, 2)
	if filter, ok := params["F"]; ok {
		decodeParams[core.Name("Filter")] = playaBoundaryDecodeObject(parser.document, parser.source, filter)
	} else if filter, ok := params["Filter"]; ok {
		decodeParams[core.Name("Filter")] = playaBoundaryDecodeObject(parser.document, parser.source, filter)
	}
	if parameters, ok := params["DP"]; ok {
		decodeParams[core.Name("DecodeParms")] = playaBoundaryDecodeObject(parser.document, parser.source, parameters)
	} else if parameters, ok := params["DecodeParms"]; ok {
		decodeParams[core.Name("DecodeParms")] = playaBoundaryDecodeObject(parser.document, parser.source, parameters)
	}
	stream := publictypes.NewStreamOwned(streamParams, cloneCompatBytes(data), decodeParams)
	decoded, err := stream.DecodedBufferWithDocumentWithError(parser.document)
	decodedLength := len(decoded)
	if err != nil {
		decodedLength = len(data)
	}
	return []interface{}{map[string]interface{}{
		"kind":   "stream",
		"length": decodedLength,
		"attrs":  attrs,
	}}
}

func playaBoundaryDecodeObject(document *core.Document, source *playaSourceResolver, value playaBoundaryValue) core.Object {
	object := playaBoundaryObject(document, source, value)
	return playaBoundaryDecodeNulls(object)
}

func playaBoundaryDecodeNulls(object core.Object) core.Object {
	switch value := object.(type) {
	case nil:
		return core.Null{}
	case core.Array:
		out := make(core.Array, len(value))
		for index, item := range value {
			out[index] = playaBoundaryDecodeNulls(item)
		}
		return out
	case core.Dict:
		out := make(core.Dict, len(value))
		for key, item := range value {
			out[key] = playaBoundaryDecodeNulls(item)
		}
		return out
	default:
		return object
	}
}

func playaBoundaryObject(document *core.Document, source *playaSourceResolver, value playaBoundaryValue) core.Object {
	if value.none {
		return nil
	}
	if value.reference {
		if source != nil {
			if resolved, ok := source.objectValue(value.refID); ok && playaBoundarySourceScalarUsable(resolved) {
				return playaBoundaryObject(document, source, resolved)
			}
			if ref, ok := source.activeRef(value.refID); ok {
				if document != nil {
					if _, object, err := document.LookupWithError(ref.Object); err == nil {
						return playaBoundaryNormalizeObject(document, source, object, map[int]bool{})
					}
				}
				return ref
			}
		}
		if objectID, ok := new(big.Int).SetString(value.refID, 10); ok && objectID.IsInt64() {
			integer := int(objectID.Int64())
			if int64(integer) == objectID.Int64() {
				return core.Ref{Object: integer}
			}
		}
		return core.Keyword("<ObjRef:" + value.refID + ">")
	}
	if value.isSequence {
		sequence := make(core.Array, len(value.sequence))
		for index := range value.sequence {
			sequence[index] = playaBoundaryObject(document, source, value.sequence[index])
		}
		return sequence
	}
	if value.isDict {
		dict := make(core.Dict, len(value.dict))
		for key, item := range value.dict {
			dict[core.Name(key)] = playaBoundaryObject(document, source, item)
		}
		return dict
	}
	return value.object
}

func playaBoundaryNormalizeObject(document *core.Document, source *playaSourceResolver, value core.Object, seen map[int]bool) core.Object {
	switch value := value.(type) {
	case core.Ref:
		if seen[value.Object] {
			return value
		}
		if ref, ok := source.activeRef(strconv.Itoa(value.Object)); ok {
			return ref
		}
		return core.Ref{Object: value.Object}
	case core.Array:
		out := make(core.Array, len(value))
		for index, item := range value {
			out[index] = playaBoundaryNormalizeObject(document, source, item, seen)
		}
		return out
	case core.Dict:
		out := make(core.Dict, len(value))
		for key, item := range value {
			out[key] = playaBoundaryNormalizeObject(document, source, item, seen)
		}
		return out
	default:
		return value
	}
}

func playaBoundaryInlineDataStart(data []byte, start int, params map[string]playaBoundaryValue) int {
	filter := playaBoundaryInlineFinalFilter(params)
	if filter == "ASCIIHexDecode" || filter == "AHx" || filter == "ASCII85Decode" || filter == "A85" {
		for start < len(data) && playaBoundaryRegexWhitespace(data[start]) {
			start++
		}
	} else if start < len(data) && playaBoundaryRegexWhitespace(data[start]) {
		start++
	}
	return start
}

func playaBoundaryInlineEnd(data []byte, start int, params map[string]playaBoundaryValue) (int, int, bool) {
	filter := playaBoundaryInlineFinalFilter(params)
	switch filter {
	case "ASCIIHexDecode", "AHx":
		for index := start; index+1 < len(data); index++ {
			if data[index] == 'E' && data[index+1] == 'I' {
				return index, index + 2, true
			}
		}
	case "ASCII85Decode", "A85":
		if matchStart, matchEnd, ok := playaBoundaryASCII85End(data, start); ok {
			return matchStart, matchEnd, true
		}
	default:
		for index := start; index+2 < len(data); index++ {
			if playaBoundaryRegexWhitespace(data[index]) && data[index+1] == 'E' && data[index+2] == 'I' && playaBoundaryWordBoundary(data, index+3) {
				return index, index + 3, true
			}
		}
	}
	lineEnd := len(data)
	for index := start; index < len(data); index++ {
		if data[index] == '\n' {
			lineEnd = index
			break
		}
	}
	for index := lineEnd - 2; index >= start; index-- {
		if data[index] == 'E' && data[index+1] == 'I' {
			return index, index + 2, true
		}
	}
	return 0, 0, false
}

func playaBoundaryASCII85End(data []byte, start int) (int, int, bool) {
	match := playaBoundaryASCII85Pattern.FindIndex(data[start:])
	if match == nil {
		return 0, 0, false
	}
	return start + match[0], start + match[1], true
}

var playaBoundaryASCII85Pattern = regexp.MustCompile(`[ \t\n\x0b\f\r]*~[ \t\n\x0b\f\r]*>[ \t\n\x0b\f\r]*EI\b`)

func playaBoundaryWordBoundary(data []byte, index int) bool {
	return index == len(data) || !playaBoundaryWordByte(data[index])
}

func playaBoundaryWordByte(value byte) bool {
	return value >= 'a' && value <= 'z' || value >= 'A' && value <= 'Z' || value >= '0' && value <= '9' || value == '_'
}

func playaBoundaryInlineFinalFilter(params map[string]playaBoundaryValue) string {
	filter, ok := params["F"]
	if !ok {
		filter = params["Filter"]
	}
	if filter.isSequence {
		if len(filter.sequence) == 0 {
			return ""
		}
		filter = filter.sequence[0]
	}
	name, _ := filter.object.(core.Name)
	return string(name)
}

func playaBoundaryRegexWhitespace(value byte) bool {
	return value == ' ' || value == '\n' || value == '\r' || value == '\t' || value == '\f' || value == '\v'
}

func playaBoundaryValueSnapshot(value playaBoundaryValue) interface{} {
	if value.none {
		return "None"
	}
	if value.numberText != "" {
		projected := interface{}(float64(value.object.(core.Number)))
		if value.integerText != "" {
			if integer, ok := new(big.Int).SetString(value.integerText, 10); ok && integer.BitLen() > 53 {
				projected = json.Number(integer.String())
			}
		}
		return TokenRecord{Kind: "number", Value: projected}
	}
	if value.reference {
		return fmt.Sprintf("<ObjRef:%s>", value.refID)
	}
	if value.isSequence {
		out := make([]interface{}, len(value.sequence))
		for index := range value.sequence {
			out[index] = playaBoundaryValueSnapshot(value.sequence[index])
		}
		return out
	}
	if value.isDict {
		out := make(map[string]interface{}, len(value.dict))
		for key, item := range value.dict {
			out[key] = playaBoundaryValueSnapshot(item)
		}
		return out
	}
	if value.inlineStart {
		return TokenRecord{Kind: "keyword", Value: "BI"}
	}
	if value.procStart {
		return TokenRecord{Kind: "keyword", Value: "{"}
	}
	switch value.delimiter {
	case core.TokenArrayStart:
		return TokenRecord{Kind: "array_start"}
	case core.TokenDictStart:
		return TokenRecord{Kind: "dict_start"}
	}
	return contentValueSnapshot(value.object)
}

// AnnotationSnapshot projects one borrowed annotation without retaining the
// source annotation.
func AnnotationSnapshot(annotation core.Annotation) Annotation {
	return Annotation{Type: annotation.Subtype(), Rect: annotation.Rect(), Contents: annotation.Contents(), Name: annotation.Name(), Modified: annotation.Modified(), PageIndex: -1}
}

func AnnotationSnapshotWithDocument(document *core.Document, annotation core.Annotation, pageRef core.Ref, pageIndex int) (Annotation, error) {
	// Annotation properties are a borrowed raw PDF dictionary. Preserve
	// indirect references instead of recursively materializing their targets;
	// a link destination may point at the entire page tree.
	properties := projectDict(annotation.DictCopy())
	parent, err := annotation.ParentWithError(document)
	if err != nil {
		return Annotation{}, err
	}
	var parentProjection *StructureElement
	if parent != nil {
		projected := projectStructure(func(yield func(core.StructElement, error) bool) {
			yield(*parent, nil)
		}, map[core.Ref]int{pageRef: pageIndex})
		if len(projected) > 0 {
			parentProjection = &projected[0]
		}
	}
	var actionProjection *Action
	if action := annotation.ActionValueCopy(); action != nil {
		actionProjection = projectAction(action, map[core.Ref]int{pageRef: pageIndex})
		resolveActionDestinationPageIndices(actionProjection, action, document)
	}
	return Annotation{
		Type: annotation.Subtype(), Rect: annotation.Rect(), BBox: annotation.BBox(document),
		PageIndex: pageIndex, Contents: annotation.Contents(), Name: annotation.Name(), Modified: annotation.Modified(),
		Parent: parentProjection, Action: actionProjection, Properties: properties,
	}, nil
}

// resolveActionDestinationPageIndices fills annotation action destinations
// from the document page tree. Annotation projections are built one page at a
// time, so their initial ref map only contains the owning page; unlike that
// local map, Playa resolves GoTo targets against every page in the document.
func resolveActionDestinationPageIndices(projected *Action, source *core.Action, document *core.Document) {
	if projected == nil || source == nil || document == nil {
		return
	}
	if destination := source.DestinationCopy(); destination != nil && projected.Destination != nil {
		if page, err := destination.PageObject(document); err == nil {
			projected.Destination.PageIndex = page.Index()
		}
	}
	next, err := source.NextCopy()
	if err != nil {
		return
	}
	for index, child := range next {
		if index >= len(projected.Next) {
			break
		}
		resolveActionDestinationPageIndices(&projected.Next[index], child, document)
	}
}

func pageIndexByRef(pages []core.Page) map[core.Ref]int {
	out := map[core.Ref]int{}
	for index, page := range pages {
		if page.Ref() != (core.Ref{}) {
			out[page.Ref()] = index
		}
	}
	return out
}

func pageIndexByDocument(document *core.Document, pageCount int) (map[core.Ref]int, error) {
	out := make(map[core.Ref]int)
	index := 0
	for page, err := range document.Pages() {
		if err != nil {
			return nil, fmt.Errorf("page %d: %w", index, err)
		}
		if page.Ref() != (core.Ref{}) {
			out[page.Ref()] = index
		}
		index++
		if index == pageCount {
			break
		}
	}
	if index != pageCount {
		return nil, fmt.Errorf("page count changed while indexing: got %d, want %d", index, pageCount)
	}
	return out, nil
}

func projectDestinations(document *core.Document, pages []core.Page) ([]NamedDestination, error) {
	return projectDestinationsByIndex(document, pageIndexByRef(pages))
}

func projectDestinationsByIndex(document *core.Document, pageIndex map[core.Ref]int) ([]NamedDestination, error) {
	out := make([]NamedDestination, 0)
	for entry, err := range document.DestinationsSeq() {
		if err != nil {
			return nil, err
		}
		destination, err := entry.DestinationWithError(document)
		if err != nil {
			return nil, err
		}
		out = append(out, NamedDestination{Name: entry.Name(), Destination: projectDestination(destination, pageIndex)})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

func projectDestination(dest *core.Destination, pageIndex map[core.Ref]int) Destination {
	out := Destination{PageIndex: -1, Params: []interface{}{}}
	if dest == nil {
		return out
	}
	if page, ok := dest.PageRef(); ok {
		if index, ok := pageIndex[page]; ok {
			out.PageIndex = index
		}
	} else if index, ok := dest.PageIndex(); ok {
		out.PageIndex = index
	}
	out.View = dest.View()
	for _, param := range dest.ParamsCopy() {
		out.Params = append(out.Params, projectObject(param))
	}
	return out
}

func projectOutline(seq func(func(core.OutlineNode, error) bool), pageIndex map[core.Ref]int) []Outline {
	out := make([]Outline, 0)
	for node, err := range seq {
		if err != nil {
			continue
		}
		target := node.TargetCopy()
		var action *Action
		if value := node.ActionValueCopy(); value != nil {
			action = projectAction(value, pageIndex)
		}
		out = append(out, Outline{
			Title:       node.Title(),
			Destination: projectDestination(target, pageIndex),
			ActionKind:  node.ActionKind(),
			Action:      action,
			Count:       node.Count(),
			HasCount:    node.HasCount(),
			Children:    projectOutline(node.ChildrenSeq(), pageIndex),
		})
	}
	return out
}

func projectAction(action *core.Action, pageIndex map[core.Ref]int) *Action {
	if action == nil {
		return nil
	}
	out := &Action{
		Kind:   action.Kind(),
		URI:    action.URI(),
		File:   action.File(),
		Name:   action.Name(),
		Script: action.Script(),
		Raw:    projectDict(action.RawCopy()),
		Next:   []Action{},
	}
	if destination := action.DestinationCopy(); destination != nil {
		projected := projectDestination(destination, pageIndex)
		out.Destination = &projected
	}
	if next, err := action.NextCopy(); err == nil {
		out.Next = make([]Action, 0, len(next))
		for _, child := range next {
			if projected := projectAction(child, pageIndex); projected != nil {
				out.Next = append(out.Next, *projected)
			}
		}
	}
	return out
}

func projectStructure(seq func(func(core.StructElement, error) bool), pageIndex map[core.Ref]int) []StructureElement {
	out := make([]StructureElement, 0)
	for node, err := range seq {
		if err != nil {
			continue
		}
		contents := make([]StructureContent, 0)
		for content, err := range node.ContentsSeq() {
			if err != nil {
				continue
			}
			contents = append(contents, StructureContent{Kind: string(content.Kind()), MCID: contextMCID(content.MCID(), content.HasMCID()), HasMCID: content.HasMCID()})
		}
		page := -1
		if node.HasPage() {
			if index, ok := pageIndex[node.Page()]; ok {
				page = index
			}
		}
		out = append(out, StructureElement{
			Type: node.StructureType(), Role: node.Role(), PageIndex: page,
			Title: node.Title(), Language: node.Language(), AlternateDescription: node.AlternateDescription(),
			ActualText: node.ActualText(), Abbreviation: node.AbbreviationExpansion(), ClassName: node.ClassName(),
			Attributes: projectDict(node.AttributesCopy()), Contents: contents,
			Children: projectStructure(node.ChildrenSeq(), pageIndex),
		})
	}
	return out
}

func collectStructurePageIndices(document *core.Document, seq func(func(core.StructElement, error) bool), pageIndex map[core.Ref]int) {
	for node, err := range seq {
		if err != nil {
			continue
		}
		if node.HasPage() {
			ref := node.Page()
			if _, known := pageIndex[ref]; !known {
				pageIndex[ref] = -1
				if owningPage, pageErr := node.PageObject(document); pageErr == nil {
					pageIndex[ref] = owningPage.Index()
				}
			}
		}
		collectStructurePageIndices(document, node.ChildrenSeq(), pageIndex)
	}
}

func pageStructureProjection(document *core.Document, page core.Page, index int) ([]StructureElement, error) {
	structure, err := page.Structure(document)
	if err != nil {
		return nil, err
	}
	elements, err := structure.ElementsCopyWithError()
	if err != nil {
		return nil, err
	}
	seq := func(yield func(core.StructElement, error) bool) {
		for _, element := range elements {
			if !yield(element, nil) {
				return
			}
		}
	}
	pageIndices := map[core.Ref]int{page.Ref(): index}
	collectStructurePageIndices(document, seq, pageIndices)
	return projectStructure(seq, pageIndices), nil
}

func projectForms(seq func(func(core.FormField, error) bool)) []Form {
	out := make([]Form, 0)
	for field, err := range seq {
		if err != nil {
			continue
		}
		options := field.OptionsCopy()
		if options == nil {
			options = []string{}
		}
		selected := field.SelectedCopy()
		if selected == nil {
			selected = []int{}
		}
		out = append(out, Form{
			Name:              field.Name(),
			FullName:          field.FullName(),
			FieldType:         field.FieldType(),
			Flags:             field.Flags(),
			HasFlags:          field.HasFlags(),
			Value:             field.Value(),
			DefaultValue:      field.DefaultValue(),
			DefaultAppearance: field.DefaultAppearance(),
			Options:           options,
			Selected:          selected,
			Rect:              field.Rect(),
			HasRect:           field.HasRect(),
			IsWidget:          field.IsWidget(),
			Kids:              projectForms(field.KidsSeq()),
		})
	}
	return out
}

func projectTags(tags []core.TagObject, pageIndex int) []Tag {
	out := make([]Tag, 0, len(tags))
	for _, tag := range tags {
		out = append(out, projectTag(tag, pageIndex))
	}
	return out
}

// TagSnapshot projects one borrowed tag without retaining the source tag.
func TagSnapshot(tag core.TagObject, pageIndex int) Tag { return projectTag(tag, pageIndex) }

func projectTag(tag core.TagObject, pageIndex int) Tag {
	markedStack := tag.MarkedStackCopy()
	marked := make([]MarkedContext, 0, len(markedStack))
	for _, context := range markedStack {
		marked = append(marked, projectMarkedContext(context))
	}
	mcid := contextMCID(tag.MCID(), tag.HasMCID())
	return Tag{
		Tag: tag.Name(), MCID: mcid, HasMCID: tag.HasMCID(), ActualText: tag.ActualText(),
		Properties: projectDict(tag.PropertiesCopy()), MarkedTag: tag.MarkedTag(),
		MarkedProperties: projectDict(tag.MarkedPropertiesCopy()), MarkedStack: marked,
		PageIndex: pageIndex, CTM: tag.GState().CTM(), State: projectGraphicsState(tag.GState()),
	}
}

func projectPath(path core.PathObject, pageIndex int) Path {
	markedStack := path.MarkedStackCopy()
	marked := make([]MarkedContext, 0, len(markedStack))
	for _, context := range markedStack {
		marked = append(marked, projectMarkedContext(context))
	}
	return Path{
		RawSegments: projectPathSegments(path.RawSegmentsCopy()), Segments: projectPathSegments(path.SegmentsCopy()),
		Stroke: path.Stroke(), Fill: path.Fill(), EvenOdd: path.EvenOdd(), BBox: path.BBox(),
		PageIndex: pageIndex, CTM: path.GState().CTM(), State: projectGraphicsState(path.GState()),
		MarkedTag: path.MarkedTag(), MarkedProperties: projectDict(path.MarkedPropertiesCopy()), MarkedStack: marked,
	}
}

// PathSnapshot projects one borrowed path without retaining the source path.
// It is used by the streaming compatibility comparator for large path arrays.
func PathSnapshot(path core.PathObject, pageIndex int) Path { return projectPath(path, pageIndex) }

func projectPathSegments(segments []geometry.PathSegment) []PathSegment {
	out := make([]PathSegment, 0, len(segments))
	for _, segment := range segments {
		points := segment.PointsCopy()
		if points == nil {
			points = [][2]float64{}
		}
		out = append(out, PathSegment{Operator: segment.Operator(), Points: points})
	}
	return out
}

func projectImages(images []core.ImageObject, pageIndex int) []Image {
	out := make([]Image, 0, len(images))
	for _, image := range images {
		out = append(out, projectImage(image, pageIndex))
	}
	return out
}

// ImageSnapshot projects one borrowed image without retaining the source image.
func ImageSnapshot(image core.ImageObject, pageIndex int) Image {
	return projectImage(image, pageIndex)
}

func projectImage(image core.ImageObject, pageIndex int) Image {
	markedStack := image.MarkedStackCopy()
	marked := make([]MarkedContext, 0, len(markedStack))
	for _, context := range markedStack {
		marked = append(marked, projectMarkedContext(context))
	}
	filters := image.RawFiltersCopy()
	if filters == nil {
		filters = []string{}
	}
	streamLength, streamSHA256, streamErr := image.DecodedStreamBufferDigestWithError()
	if streamErr != nil {
		streamLength, streamSHA256 = 0, ""
	}
	return Image{
		Name: image.Name(), Width: image.Width(), Height: image.Height(), Bits: image.BPC(),
		ImageMask: image.ImageMask(), ColorSpace: image.ColorSpace(), Components: image.Components(),
		Filters: filters, StreamLength: streamLength, StreamSHA256: streamSHA256,
		BBox: image.BBox(), PageIndex: pageIndex, CTM: image.GState().CTM(),
		State:     projectGraphicsState(image.GState()),
		MarkedTag: image.MarkedTag(), MarkedProperties: projectDict(image.MarkedPropertiesCopy()), MarkedStack: marked,
	}
}

func projectFonts(fonts []core.FontResource) []Font {
	out := make([]Font, 0, len(fonts))
	for _, resource := range fonts {
		if font, ok := projectFont(resource); ok {
			out = append(out, font)
		}
	}
	return out
}

// FontSnapshot projects one borrowed font resource. The boolean reports
// whether the resource has usable metadata, matching the page projection's
// filtering behavior.
func FontSnapshot(resource core.FontResource) (Font, bool) { return projectFont(resource) }

func projectFont(resource core.FontResource) (Font, bool) {
	metadata, ok := resource.Metadata()
	if !ok {
		return Font{}, false
	}
	font := resource.FontCopy()
	return Font{
		Name: resource.Name(), FontName: compatibleFontName(metadata.Name()), BaseFont: compatibleFontName(font.BaseFont()), CIDCoding: font.CIDCoding(), Flags: metadata.Flags(),
		Ascent: metadata.Ascent(), Descent: metadata.Descent(), Leading: metadata.Leading(),
		ItalicAngle: metadata.ItalicAngle(), DefaultWidth: metadata.DefaultWidth(),
		Matrix: metadata.FontMatrix(), Vertical: metadata.Vertical(),
		BBox: metadata.BBox(), Metrics: projectFontMetrics(font), Decode: projectFontDecode(font), ToUnicode: projectFontToUnicode(font),
	}, true
}

func projectFontValue(name string, font *core.Font) Font {
	if font == nil {
		return Font{Name: name}
	}
	flags, _ := font.Flags()
	bbox, _ := font.FontBBox()
	return Font{
		Name: name, FontName: compatibleFontName(font.Name()), BaseFont: compatibleFontName(font.BaseFont()), CIDCoding: font.CIDCoding(), Flags: flags,
		Ascent: font.Ascent(), Descent: font.Descent(), Leading: font.Leading(),
		ItalicAngle: font.ItalicAngle(), DefaultWidth: font.DefaultWidth(),
		Matrix: font.FontMatrix(), Vertical: font.IsVertical(), Multibyte: font.IsMultibyte(), BBox: bbox, Metrics: projectFontMetrics(font), Decode: projectFontDecode(font), ToUnicode: projectFontToUnicode(font),
	}
}

func projectFontMetrics(font *core.Font) []FontMetric {
	metrics := make([]FontMetric, 0, 5)
	if font == nil {
		return metrics
	}
	for _, cid := range []int{0, 1, 32, 65, 255} {
		metrics = append(metrics, FontMetric{
			CID: cid, HDisp: font.HDisp(cid), VDisp: font.VDisp(cid),
			Position: font.Position(cid), BBox: font.CharBBox(cid),
		})
	}
	return metrics
}

func projectFontDecode(font *core.Font) []FontDecode {
	out := make([]FontDecode, 0)
	if font == nil {
		return out
	}
	probes := [][]byte{{32}, {65}}
	if font.IsCID() {
		probes = [][]byte{{0, 1}}
	}
	for _, code := range probes {
		glyphs := make([]GlyphDecode, 0)
		for _, glyph := range font.DecodeGlyphs(code) {
			glyphs = append(glyphs, GlyphDecode{CID: glyph.CID(), Text: glyph.Text()})
		}
		out = append(out, FontDecode{Code: hex.EncodeToString(code), Glyphs: glyphs})
	}
	return out
}

func projectFontToUnicode(font *core.Font) []FontUnicode {
	out := make([]FontUnicode, 0, 2)
	if font == nil {
		return out
	}
	probes := [][]byte{{32}, {65}}
	if font.IsCID() {
		probes = [][]byte{{0, 1}}
	}
	for _, code := range probes {
		value, mapped := font.ToUnicodeCodeValue(code)
		out = append(out, FontUnicode{Code: hex.EncodeToString(code), Value: value, Mapped: mapped})
	}
	return out
}

func contextMCID(mcid int, hasMCID bool) int {
	if !hasMCID {
		return 0
	}
	return mcid
}

func projectMarkedContext(context core.MarkedContentContext) MarkedContext {
	return MarkedContext{
		Tag: context.Tag(), MCID: contextMCID(context.MCID(), context.HasMCID()), HasMCID: context.HasMCID(),
		ActualText: context.ActualText(), Properties: projectDict(context.PropertiesCopy()),
	}
}

func compatibleFontName(name string) string {
	if len(name) > 7 && name[6] == '+' {
		for _, char := range name[:6] {
			if char < 'A' || char > 'Z' {
				return name
			}
		}
		return name[7:]
	}
	for _, suffix := range []string{"-Identity-H", "-Identity-V"} {
		if strings.HasSuffix(name, suffix) {
			return strings.TrimSuffix(name, suffix)
		}
	}
	return name
}

func projectMarkedSectionsFromPage(document *core.Document, page core.Page) ([]MarkedSection, error) {
	sequence := page.MarkedContentSequence(document)
	var out []MarkedSection
	for section, sectionErr := range sequence.PageOrderSeq() {
		if sectionErr != nil {
			return nil, sectionErr
		}
		if !section.HasMCID() {
			continue
		}
		// Playa's compatibility projection iterates page_order but omits
		// explicit MCID sections that emitted no content objects. Keep empty
		// slots in the public ContentSequence; only the JSON projection drops
		// them to match Playa's page.marked serializer.
		if section.Len() == 0 {
			continue
		}
		texts := section.TextsCopy()
		if texts == nil {
			texts = []string{}
		}
		out = append(out, MarkedSection{MCID: section.MCID(), Texts: texts})
	}
	return out, nil
}

func projectDict(value core.Dict) map[string]interface{} {
	if value == nil {
		return map[string]interface{}{}
	}
	out := make(map[string]interface{}, len(value))
	for key, item := range value {
		out[string(key)] = projectObject(item)
	}
	return out
}

// projectResourceGraph retains reference identity while projecting every
// reachable object once. Shared and cyclic resource dictionaries therefore
// require space proportional to their graph instead of their expanded paths.
func projectResourceGraph(document *core.Document, value core.Dict) (map[string]interface{}, error) {
	if value == nil {
		return nil, nil
	}
	seen := make(map[int]bool)
	queue := make([]core.Ref, 0)
	var project func(core.Object) (interface{}, error)
	project = func(object core.Object) (interface{}, error) {
		switch item := object.(type) {
		case core.Ref:
			if !seen[item.Object] {
				seen[item.Object] = true
				queue = append(queue, item)
			}
			return map[string]interface{}{"ref": item.Object}, nil
		case core.Stream:
			attrs, err := project(item.DictCopy())
			if err != nil {
				return nil, err
			}
			length, digest, err := item.DecodedBufferDigestWithDocument(document)
			if err != nil {
				return nil, err
			}
			return map[string]interface{}{"dict": attrs, "length": length, "sha256": digest}, nil
		case core.Dict:
			out := make(map[string]interface{}, len(item))
			keys := make([]string, 0, len(item))
			for key := range item {
				keys = append(keys, string(key))
			}
			sort.Strings(keys)
			for _, key := range keys {
				child, err := project(item[core.Name(key)])
				if err != nil {
					return nil, err
				}
				out[key] = child
			}
			return out, nil
		case core.Array:
			out := make([]interface{}, len(item))
			for index, child := range item {
				var err error
				out[index], err = project(child)
				if err != nil {
					return nil, err
				}
			}
			return out, nil
		case core.InvalidArray:
			return project(core.Array(item))
		default:
			return projectDocumentObject(object), nil
		}
	}
	root, err := project(value)
	if err != nil {
		return nil, err
	}
	objects := make([]interface{}, 0)
	for next := 0; next < len(queue); next++ {
		ref := queue[next]
		resolved, err := document.ResolveObject(ref)
		if err != nil {
			return nil, err
		}
		node, err := project(resolved)
		if err != nil {
			return nil, err
		}
		objects = append(objects, map[string]interface{}{"object": ref.Object, "value": node})
	}
	sort.Slice(objects, func(i, j int) bool {
		return objects[i].(map[string]interface{})["object"].(int) < objects[j].(map[string]interface{})["object"].(int)
	})
	return map[string]interface{}{"root": root, "objects": objects}, nil
}

func projectResolvedObject(document *core.Document, object core.Object) (interface{}, error) {
	return projectResolvedObjectSeen(document, object, make(map[core.Ref]bool))
}

func projectResolvedObjectSeen(document *core.Document, object core.Object, active map[core.Ref]bool) (interface{}, error) {
	if object == nil {
		return nil, nil
	}
	if ref, ok := object.(core.Ref); ok {
		if active[ref] {
			return map[string]interface{}{"ref": ref.Object}, nil
		}
		active[ref] = true
		defer delete(active, ref)
		resolved, err := document.ResolveObject(ref)
		if err != nil {
			return nil, err
		}
		return projectResolvedObjectSeen(document, resolved, active)
	}
	switch value := object.(type) {
	case core.Stream:
		streamDict, err := projectResolvedObjectSeen(document, value.DictCopy(), active)
		if err != nil {
			return nil, err
		}
		decoded, err := value.DecodedBufferWithDocumentWithError(document)
		if err != nil {
			return nil, err
		}
		return map[string]interface{}{
			"dict": streamDict,
			"data": projectObject(core.String(decoded)),
		}, nil
	case core.Dict:
		if value == nil {
			return nil, nil
		}
		out := make(map[string]interface{}, len(value))
		for key, item := range value {
			projected, err := projectResolvedObjectSeen(document, item, active)
			if err != nil {
				return nil, err
			}
			out[string(key)] = projected
		}
		return out, nil
	case core.Array:
		out := make([]interface{}, len(value))
		for index, item := range value {
			projected, err := projectResolvedObjectSeen(document, item, active)
			if err != nil {
				return nil, err
			}
			out[index] = projected
		}
		return out, nil
	case core.InvalidArray:
		out := make([]interface{}, len(value))
		for index, item := range value {
			projected, err := projectResolvedObjectSeen(document, item, active)
			if err != nil {
				return nil, err
			}
			out[index] = projected
		}
		return out, nil
	default:
		return projectObject(object), nil
	}
}

func projectDocumentDict(value core.Dict) map[string]interface{} {
	if value == nil {
		return map[string]interface{}{}
	}
	out := make(map[string]interface{}, len(value))
	for key, item := range value {
		out[string(key)] = projectDocumentObject(item)
	}
	return out
}

func playaDocumentInfo() map[string]interface{} {
	// The pinned Playa release initializes Document.info but never populates it;
	// compatibility projections must preserve that observable result even
	// though the Go domain API exposes the parsed Info dictionary separately.
	return map[string]interface{}{}
}

func playaTrailerProjection(document *core.Document) map[string]interface{} {
	trailer := projectDocumentDict(document.Trailer())
	xrefs, err := document.XRefs()
	if err != nil || len(xrefs) == 0 {
		return trailer
	}
	trailer = projectDocumentDict(xrefs[0].TrailerCopy())
	if len(trailer) == 0 && xrefs[0].Kind() == "fallback" {
		// Playa's damaged-input fallback exposes the first recovered page
		// dictionary as document.trailer when no trailer marker exists.
		for page, pageErr := range document.Pages() {
			if pageErr == nil {
				trailer = projectDocumentDict(page.DictCopy())
			}
			break
		}
	}
	size := 0
	for _, table := range xrefs {
		size += table.EntryCount()
	}
	trailer["Size"] = size
	return trailer
}

func projectDocumentObject(object core.Object) interface{} {
	switch value := object.(type) {
	case core.Ref:
		return map[string]interface{}{"ref": value.Object}
	case core.Array:
		out := make([]interface{}, len(value))
		for index, item := range value {
			out[index] = projectDocumentObject(item)
		}
		return out
	case core.InvalidArray:
		out := make([]interface{}, len(value))
		for index, item := range value {
			out[index] = projectDocumentObject(item)
		}
		return out
	case core.Dict:
		return projectDocumentDict(value)
	case core.String:
		// Document.objects exposes binary string values as bytes. Keep the
		// compatibility projection lossless instead of applying a text codec
		// to encryption keys and other arbitrary byte strings.
		return hex.EncodeToString([]byte(value))
	default:
		return projectObject(object)
	}
}

func ObjectSnapshot(document *core.Document, object core.IndirectObject) (ObjectRecord, error) {
	record := ObjectRecord{Object: object.Ref().Object, Generation: object.Ref().Generation}
	value := object.ValueCopy()
	if stream, ok := value.(core.Stream); ok {
		length, digest, err := stream.DecodedBufferDigestWithDocument(document)
		if err != nil {
			return ObjectRecord{}, err
		}
		record.Stream = &StreamRecord{
			Dict:   projectDocumentDict(stream.DictCopy()),
			Length: length,
			SHA256: digest,
		}
		return record, nil
	}
	record.Value = projectDocumentObject(value)
	return record, nil
}

// MappingSnapshot projects one entry from the XRef-ordered Document.Items.
// Mapping values retain raw indirect references, matching Playa's Mapping
// object view rather than the recursively resolved catalog projections.
func MappingSnapshot(document *core.Document, object core.IndirectObject) (MappingRecord, error) {
	record := MappingRecord{Object: object.Ref().Object}
	value := object.ValueCopy()
	if stream, ok := value.(core.Stream); ok {
		length, digest, err := stream.DecodedBufferDigestWithDocument(document)
		if err != nil {
			return MappingRecord{}, err
		}
		record.Stream = &StreamRecord{
			Dict:   projectDocumentDict(stream.DictCopy()),
			Length: length,
			SHA256: digest,
		}
		return record, nil
	}
	record.Value = projectDocumentObject(value)
	return record, nil
}

func collectMapping(document *core.Document) ([]MappingRecord, error) {
	var out []MappingRecord
	for object, err := range document.Items() {
		if err != nil {
			return nil, err
		}
		record, err := MappingSnapshot(document, object)
		if err != nil {
			return nil, err
		}
		out = append(out, record)
	}
	return out, nil
}

// WriteMappingJSONL streams the XRef-ordered Mapping view without retaining all
// object values in a compatibility snapshot.
func WriteMappingJSONL(document *core.Document, writer io.Writer) (int, error) {
	if document == nil {
		return 0, fmt.Errorf("playa: compatibility mapping: nil document")
	}
	encoder := json.NewEncoder(writer)
	count := 0
	for object, err := range document.Items() {
		if err != nil {
			return count, err
		}
		snapshot, snapshotErr := MappingSnapshot(document, object)
		if snapshotErr != nil {
			return count, snapshotErr
		}
		if err := encoder.Encode(struct {
			Kind    string        `json:"kind"`
			Mapping MappingRecord `json:"mapping"`
		}{Kind: "mapping", Mapping: snapshot}); err != nil {
			return count, err
		}
		count++
	}
	return count, nil
}

func WriteObjectsJSONL(document *core.Document, writer io.Writer) (int, error) {
	if document == nil {
		return 0, fmt.Errorf("playa: compatibility objects: nil document")
	}
	encoder := json.NewEncoder(writer)
	count := 0
	for object, err := range document.Objects() {
		if err != nil {
			return count, err
		}
		snapshot, snapshotErr := ObjectSnapshot(document, object)
		if snapshotErr != nil {
			return count, snapshotErr
		}
		if err := encoder.Encode(struct {
			Kind   string       `json:"kind"`
			Object ObjectRecord `json:"object"`
		}{Kind: "object", Object: snapshot}); err != nil {
			return count, err
		}
		count++
		document.ReleaseTransientCaches()
	}
	return count, nil
}

func TokenSnapshot(token core.Token) TokenRecord {
	record := TokenRecord{Kind: tokenKindName(token.Kind())}
	switch token.Kind() {
	case core.TokenNumber:
		if spelling := token.NumberText(); spelling != "" {
			record.Value = json.Number(canonicalNumberText(spelling))
		} else {
			record.Value = token.Number()
		}
	case core.TokenName, core.TokenKeyword:
		if token.Kind() == core.TokenKeyword && (token.Text() == "true" || token.Text() == "false") {
			record.Kind = "number"
			if token.Text() == "true" {
				record.Value = 1
			} else {
				record.Value = 0
			}
			break
		}
		record.Value = token.Text()
	case core.TokenString, core.TokenHexString:
		record.Kind = "string"
		record.Value = hex.EncodeToString(token.ValueCopy())
	}
	return record
}

func canonicalNumberText(spelling string) string {
	if !strings.ContainsAny(spelling, ".eE") {
		if integer, ok := new(big.Int).SetString(spelling, 10); ok {
			return integer.String()
		}
	}
	value, err := strconv.ParseFloat(spelling, 64)
	if err != nil {
		return spelling
	}
	formatted := strconv.FormatFloat(value, 'g', -1, 64)
	if strings.Contains(spelling, ".") && !strings.ContainsAny(formatted, ".eE") {
		formatted += ".0"
	}
	return formatted
}

func tokenKindName(kind parserconfig.TokenKind) string {
	switch kind {
	case core.TokenNumber:
		return "number"
	case core.TokenName:
		return "name"
	case core.TokenLiteral:
		return "literal"
	case core.TokenString:
		return "string"
	case core.TokenHexString:
		return "hex_string"
	case core.TokenArrayStart:
		return "array_start"
	case core.TokenArrayEnd:
		return "array_end"
	case core.TokenDictStart:
		return "dict_start"
	case core.TokenDictEnd:
		return "dict_end"
	case core.TokenKeyword:
		return "keyword"
	default:
		return "unknown"
	}
}

func WriteTokensJSONL(document *core.Document, writer io.Writer) (int, error) {
	if document == nil {
		return 0, fmt.Errorf("playa: compatibility tokens: nil document")
	}
	encoder := json.NewEncoder(writer)
	count := 0
	for token, err := range document.Tokens() {
		if err != nil {
			return count, err
		}
		if err := encoder.Encode(struct {
			Kind  string      `json:"kind"`
			Token TokenRecord `json:"token"`
		}{Kind: "token", Token: TokenSnapshot(token)}); err != nil {
			return count, err
		}
		count++
	}
	return count, nil
}

func projectObject(object core.Object) interface{} {
	switch value := object.(type) {
	case nil:
		return nil
	case core.Null:
		return nil
	case core.Bool:
		return bool(value)
	case core.Number:
		return float64(value)
	case core.Name:
		return string(value)
	case core.String:
		return pdfparser.DecodePDFText([]byte(value))
	case core.Array:
		out := make([]interface{}, len(value))
		for index, item := range value {
			out[index] = projectObject(item)
		}
		return out
	case core.InvalidArray:
		out := make([]interface{}, len(value))
		for index, item := range value {
			out[index] = projectObject(item)
		}
		return out
	case core.Dict:
		return projectDict(value)
	case core.Ref:
		return map[string]interface{}{"ref": value.Object}
	default:
		return fmt.Sprint(value)
	}
}
