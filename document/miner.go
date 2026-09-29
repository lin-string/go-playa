package document

import (
	"encoding/json"
	"fmt"
	"iter"
	"math"
	"slices"
	"sort"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/lin-string/go-playa/contentconfig"
	"github.com/lin-string/go-playa/geometry"
	"github.com/lin-string/go-playa/layout"
)

// DefaultLayoutOptions returns the Playa-compatible default mining thresholds.
func DefaultLayoutOptions() layout.Options {
	return layout.DefaultOptions()
}

type TextWord struct {
	data   layout.Component
	glyphs []GlyphObject
}

func (w TextWord) Text() string     { return w.data.Text() }
func (w TextWord) BBox() [4]float64 { return w.data.BBox() }
func (w TextWord) X0() float64      { return w.data.X0() }
func (w TextWord) Y0() float64      { return w.data.Y0() }
func (w TextWord) X1() float64      { return w.data.X1() }
func (w TextWord) Y1() float64      { return w.data.Y1() }
func (w TextWord) Width() float64   { return w.data.Width() }
func (w TextWord) Height() float64  { return w.data.Height() }
func (w TextWord) IsEmpty() bool    { return w.data.IsEmpty() }
func (w TextWord) IsHoverlap(other BBoxProvider) bool {
	return w.data.IsHoverlap(other)
}
func (w TextWord) HDistance(other BBoxProvider) float64 {
	return w.data.HDistance(other)
}
func (w TextWord) Hoverlap(other BBoxProvider) float64 {
	return w.data.Hoverlap(other)
}
func (w TextWord) IsVOverlap(other BBoxProvider) bool {
	return w.data.IsVOverlap(other)
}
func (w TextWord) VDistance(other BBoxProvider) float64 {
	return w.data.VDistance(other)
}
func (w TextWord) VOverlap(other BBoxProvider) float64 {
	return w.data.VOverlap(other)
}

// GlyphsSeq yields borrowed glyphs in word order. Call Finalize before
// retaining a glyph beyond the next sequence step.
func (w TextWord) GlyphsSeq() iter.Seq[GlyphObject] { return glyphObjectsSeq(w.glyphs) }

func (w TextWord) GlyphsCopy() []GlyphObject { return cloneGlyphObjects(w.glyphs) }

// GlyphsCopyWithError returns independent glyph snapshots and reports
// deferred font-program or character-procedure failures.
func (w TextWord) GlyphsCopyWithError() ([]GlyphObject, error) {
	return cloneGlyphObjectsWithError(w.glyphs)
}

// FinalizeWithError returns an independent word snapshot and reports deferred
// font-program or character-procedure failures.
func (w TextWord) FinalizeWithError() (TextWord, error) {
	words, err := cloneTextWordsWithError([]TextWord{w})
	if err != nil {
		return TextWord{}, err
	}
	return words[0], nil
}

// Finalize returns an independent snapshot of the word.
func (w TextWord) Finalize() TextWord { return cloneTextWords([]TextWord{w})[0] }

func (w TextWord) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Text   string
		Glyphs []GlyphObject
		BBox   [4]float64
	}{w.Text(), cloneBorrowedGlyphObjects(w.glyphs), w.BBox()})
}

type TextLine struct {
	data   layout.Component
	glyphs []GlyphObject
	words  []TextWord
}

func (l TextLine) Text() string     { return l.data.Text() }
func (l TextLine) BBox() [4]float64 { return l.data.BBox() }
func (l TextLine) X0() float64      { return l.data.X0() }
func (l TextLine) Y0() float64      { return l.data.Y0() }
func (l TextLine) X1() float64      { return l.data.X1() }
func (l TextLine) Y1() float64      { return l.data.Y1() }
func (l TextLine) Width() float64   { return l.data.Width() }
func (l TextLine) Height() float64  { return l.data.Height() }
func (l TextLine) IsEmpty() bool    { return layout.TextLineIsEmpty(l.Text(), l.BBox()) }
func (l TextLine) IsHoverlap(other BBoxProvider) bool {
	return l.data.IsHoverlap(other)
}
func (l TextLine) HDistance(other BBoxProvider) float64 {
	return l.data.HDistance(other)
}
func (l TextLine) Hoverlap(other BBoxProvider) float64 {
	return l.data.Hoverlap(other)
}
func (l TextLine) IsVOverlap(other BBoxProvider) bool {
	return l.data.IsVOverlap(other)
}
func (l TextLine) VDistance(other BBoxProvider) float64 {
	return l.data.VDistance(other)
}
func (l TextLine) VOverlap(other BBoxProvider) float64 {
	return l.data.VOverlap(other)
}
func (l TextLine) Vertical() bool { return l.data.Vertical() }

// WritingMode returns Playa's layout writing-mode name.
func (l TextLine) WritingMode() string { return l.data.WritingMode() }

// GlyphsSeq yields borrowed glyphs in line order.
func (l TextLine) GlyphsSeq() iter.Seq[GlyphObject] { return glyphObjectsSeq(l.glyphs) }

// WordsSeq yields borrowed words in line order.
func (l TextLine) WordsSeq() iter.Seq[TextWord] { return textWordsSeq(l.words) }

func (l TextLine) GlyphsCopy() []GlyphObject { return cloneGlyphObjects(l.glyphs) }

// GlyphsCopyWithError returns independent glyph snapshots and reports
// deferred font-program or character-procedure failures.
func (l TextLine) GlyphsCopyWithError() ([]GlyphObject, error) {
	return cloneGlyphObjectsWithError(l.glyphs)
}

func (l TextLine) WordsCopy() []TextWord { return cloneTextWords(l.words) }

// WordsCopyWithError returns independent word snapshots and reports deferred
// font-program or character-procedure failures.
func (l TextLine) WordsCopyWithError() ([]TextWord, error) {
	return cloneTextWordsWithError(l.words)
}

// FinalizeWithError returns an independent line snapshot and reports deferred
// font-program or character-procedure failures.
func (l TextLine) FinalizeWithError() (TextLine, error) {
	lines, err := cloneTextLinesWithError([]TextLine{l})
	if err != nil {
		return TextLine{}, err
	}
	return lines[0], nil
}

// Finalize returns an independent snapshot of the line.
func (l TextLine) Finalize() TextLine { return cloneTextLines([]TextLine{l})[0] }

func (l TextLine) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Text     string
		Glyphs   []GlyphObject
		Words    []TextWord
		BBox     [4]float64
		Vertical bool
	}{l.Text(), cloneBorrowedGlyphObjects(l.glyphs), l.words, l.BBox(), l.Vertical()})
}

type TextParagraph struct {
	data  layout.Component
	lines []TextLine
}

func (p TextParagraph) Text() string     { return p.data.Text() }
func (p TextParagraph) BBox() [4]float64 { return p.data.BBox() }
func (p TextParagraph) X0() float64      { return p.data.X0() }
func (p TextParagraph) Y0() float64      { return p.data.Y0() }
func (p TextParagraph) X1() float64      { return p.data.X1() }
func (p TextParagraph) Y1() float64      { return p.data.Y1() }
func (p TextParagraph) Width() float64   { return p.data.Width() }
func (p TextParagraph) Height() float64  { return p.data.Height() }
func (p TextParagraph) IsEmpty() bool    { return p.data.IsEmpty() }
func (p TextParagraph) IsHoverlap(other BBoxProvider) bool {
	return p.data.IsHoverlap(other)
}
func (p TextParagraph) HDistance(other BBoxProvider) float64 {
	return p.data.HDistance(other)
}
func (p TextParagraph) Hoverlap(other BBoxProvider) float64 {
	return p.data.Hoverlap(other)
}
func (p TextParagraph) IsVOverlap(other BBoxProvider) bool {
	return p.data.IsVOverlap(other)
}
func (p TextParagraph) VDistance(other BBoxProvider) float64 {
	return p.data.VDistance(other)
}
func (p TextParagraph) VOverlap(other BBoxProvider) float64 {
	return p.data.VOverlap(other)
}
func (p TextParagraph) Vertical() bool { return p.data.Vertical() }

func (p TextParagraph) WritingMode() string { return p.data.WritingMode() }

// LinesSeq yields borrowed lines in paragraph order.
func (p TextParagraph) LinesSeq() iter.Seq[TextLine] { return textLinesSeq(p.lines) }

func (p TextParagraph) LinesCopy() []TextLine { return cloneTextLines(p.lines) }

// LinesCopyWithError returns independent line snapshots and reports deferred
// font-program or character-procedure failures.
func (p TextParagraph) LinesCopyWithError() ([]TextLine, error) {
	return cloneTextLinesWithError(p.lines)
}

// FinalizeWithError returns an independent paragraph snapshot and reports
// deferred font-program or character-procedure failures.
func (p TextParagraph) FinalizeWithError() (TextParagraph, error) {
	paragraphs, err := cloneTextParagraphsWithError([]TextParagraph{p})
	if err != nil {
		return TextParagraph{}, err
	}
	return paragraphs[0], nil
}

// Finalize returns an independent snapshot of the paragraph.
func (p TextParagraph) Finalize() TextParagraph { return cloneTextParagraphs([]TextParagraph{p})[0] }

func (p TextParagraph) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Text     string
		Lines    []TextLine
		BBox     [4]float64
		Vertical bool
	}{p.Text(), p.lines, p.BBox(), p.Vertical()})
}

// TextBox is the layout container corresponding to Playa's LTTextBox.
// Lines are borrowed by LinesSeq and copied by LinesCopy/Finalize.
type TextBox struct {
	data  layout.Component
	lines []TextLine
}

func (b TextBox) Text() string     { return b.data.Text() }
func (b TextBox) BBox() [4]float64 { return b.data.BBox() }
func (b TextBox) X0() float64      { return b.data.X0() }
func (b TextBox) Y0() float64      { return b.data.Y0() }
func (b TextBox) X1() float64      { return b.data.X1() }
func (b TextBox) Y1() float64      { return b.data.Y1() }
func (b TextBox) Width() float64   { return b.data.Width() }
func (b TextBox) Height() float64  { return b.data.Height() }
func (b TextBox) IsEmpty() bool    { return b.data.IsEmpty() }
func (b TextBox) IsHoverlap(other BBoxProvider) bool {
	return b.data.IsHoverlap(other)
}
func (b TextBox) HDistance(other BBoxProvider) float64 {
	return b.data.HDistance(other)
}
func (b TextBox) Hoverlap(other BBoxProvider) float64 {
	return b.data.Hoverlap(other)
}
func (b TextBox) IsVOverlap(other BBoxProvider) bool {
	return b.data.IsVOverlap(other)
}
func (b TextBox) VDistance(other BBoxProvider) float64 {
	return b.data.VDistance(other)
}
func (b TextBox) VOverlap(other BBoxProvider) float64 {
	return b.data.VOverlap(other)
}
func (b TextBox) Vertical() bool { return b.data.Vertical() }
func (b TextBox) Index() int     { return b.data.Index() }

func (b TextBox) WritingMode() string { return b.data.WritingMode() }

// LinesSeq yields borrowed lines in textbox reading order.
func (b TextBox) LinesSeq() iter.Seq[TextLine] { return textLinesSeq(b.lines) }

func (b TextBox) LinesCopy() []TextLine { return cloneTextLines(b.lines) }

// LinesCopyWithError returns independent line snapshots and reports deferred
// font-program or character-procedure failures.
func (b TextBox) LinesCopyWithError() ([]TextLine, error) {
	return cloneTextLinesWithError(b.lines)
}

// FinalizeWithError returns an independent textbox snapshot and reports
// deferred font-program or character-procedure failures.
func (b TextBox) FinalizeWithError() (TextBox, error) {
	boxes, err := cloneTextBoxesWithError([]TextBox{b})
	if err != nil {
		return TextBox{}, err
	}
	return boxes[0], nil
}

// Finalize returns an independent snapshot of the textbox.
func (b TextBox) Finalize() TextBox { return cloneTextBoxes([]TextBox{b})[0] }

func (b TextBox) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Text     string
		Lines    []TextLine
		BBox     [4]float64
		Vertical bool
		Index    int
	}{b.Text(), b.lines, b.BBox(), b.Vertical(), b.Index()})
}

// TextGroupChild is one node in Playa's recursive LTTextGroup tree.
// Exactly one of BoxCopy and GroupCopy succeeds.
type TextGroupChild struct {
	box   *TextBox
	group *TextGroup
}

func (c TextGroupChild) IsBox() bool   { return c.box != nil }
func (c TextGroupChild) IsGroup() bool { return c.group != nil }

// BoxBorrowed returns the textbox view held by this child without copying its
// lines or glyphs. The returned value is borrowed from the parent group; use
// BoxCopy or Finalize when retaining an independent snapshot.
func (c TextGroupChild) BoxBorrowed() (TextBox, bool) {
	if c.box == nil {
		return TextBox{}, false
	}
	return *c.box, true
}

// GroupBorrowed returns the nested group view held by this child without
// copying its descendants. The returned value is borrowed from the parent
// group; use GroupCopy or Finalize when retaining an independent snapshot.
func (c TextGroupChild) GroupBorrowed() (TextGroup, bool) {
	if c.group == nil {
		return TextGroup{}, false
	}
	return *c.group, true
}

func (c TextGroupChild) BoxCopy() (TextBox, bool) {
	if c.box == nil {
		return TextBox{}, false
	}
	return cloneTextBoxes([]TextBox{*c.box})[0], true
}

func (c TextGroupChild) GroupCopy() (TextGroup, bool) {
	if c.group == nil {
		return TextGroup{}, false
	}
	return cloneTextGroups([]TextGroup{*c.group})[0], true
}

func (c TextGroupChild) Finalize() TextGroupChild {
	if c.box != nil {
		box := cloneTextBoxes([]TextBox{*c.box})[0]
		return TextGroupChild{box: &box}
	}
	if c.group != nil {
		group := cloneTextGroups([]TextGroup{*c.group})[0]
		return TextGroupChild{group: &group}
	}
	return TextGroupChild{}
}

func (c TextGroupChild) MarshalJSON() ([]byte, error) {
	if c.box != nil {
		return json.Marshal(struct {
			Kind string  `json:"kind"`
			Box  TextBox `json:"box"`
		}{"textbox", *c.box})
	}
	if c.group != nil {
		return json.Marshal(struct {
			Kind  string    `json:"kind"`
			Group TextGroup `json:"group"`
		}{"textgroup", *c.group})
	}
	return json.Marshal(struct {
		Kind string `json:"kind"`
	}{"unknown"})
}

// TextGroup is a reading-order group corresponding to Playa's recursive
// LTTextGroup. Children are borrowed by ChildrenSeq and copied by
// ChildrenCopy/Finalize. BoxesSeq remains a flattened convenience view.
type TextGroup struct {
	data     layout.Component
	boxes    []TextBox
	children []TextGroupChild
}

func (g TextGroup) Text() string     { return g.data.Text() }
func (g TextGroup) BBox() [4]float64 { return g.data.BBox() }
func (g TextGroup) X0() float64      { return g.data.X0() }
func (g TextGroup) Y0() float64      { return g.data.Y0() }
func (g TextGroup) X1() float64      { return g.data.X1() }
func (g TextGroup) Y1() float64      { return g.data.Y1() }
func (g TextGroup) Width() float64   { return g.data.Width() }
func (g TextGroup) Height() float64  { return g.data.Height() }
func (g TextGroup) IsEmpty() bool    { return g.data.IsEmpty() }
func (g TextGroup) IsHoverlap(other BBoxProvider) bool {
	return g.data.IsHoverlap(other)
}
func (g TextGroup) HDistance(other BBoxProvider) float64 {
	return g.data.HDistance(other)
}
func (g TextGroup) Hoverlap(other BBoxProvider) float64 {
	return g.data.Hoverlap(other)
}
func (g TextGroup) IsVOverlap(other BBoxProvider) bool {
	return g.data.IsVOverlap(other)
}
func (g TextGroup) VDistance(other BBoxProvider) float64 {
	return g.data.VDistance(other)
}
func (g TextGroup) VOverlap(other BBoxProvider) float64 {
	return g.data.VOverlap(other)
}
func (g TextGroup) Vertical() bool { return g.data.Vertical() }

func (g TextGroup) WritingMode() string { return g.data.WritingMode() }

func (g TextGroup) ChildrenSeq() iter.Seq[TextGroupChild] {
	return func(yield func(TextGroupChild) bool) {
		if len(g.children) > 0 {
			for _, child := range g.children {
				if !yield(child) {
					return
				}
			}
			return
		}
		for i := range g.boxes {
			if !yield(TextGroupChild{box: &g.boxes[i]}) {
				return
			}
		}
	}
}

func (g TextGroup) ChildrenCopy() []TextGroupChild {
	if len(g.children) == 0 {
		if g.boxes == nil {
			return nil
		}
		out := make([]TextGroupChild, len(g.boxes))
		for i := range g.boxes {
			box := cloneTextBoxes([]TextBox{g.boxes[i]})[0]
			out[i] = TextGroupChild{box: &box}
		}
		return out
	}
	out := make([]TextGroupChild, len(g.children))
	for i, child := range g.children {
		out[i] = child.Finalize()
	}
	return out
}

// ChildrenCopyWithError returns an independent recursive group snapshot and
// reports deferred font-program or character-procedure failures.
func (g TextGroup) ChildrenCopyWithError() ([]TextGroupChild, error) {
	var children []TextGroupChild
	for child := range g.ChildrenSeq() {
		children = append(children, child)
	}
	if children == nil {
		return nil, nil
	}
	out := make([]TextGroupChild, len(children))
	for i, child := range children {
		finalized, err := finalizeTextGroupChildWithError(child)
		if err != nil {
			return nil, err
		}
		out[i] = finalized
	}
	return out, nil
}

func (g TextGroup) BoxesSeq() iter.Seq[TextBox] {
	return func(yield func(TextBox) bool) {
		for child := range g.ChildrenSeq() {
			if child.box != nil {
				if !yield(*child.box) {
					return
				}
				continue
			}
			if child.group != nil {
				for box := range child.group.BoxesSeq() {
					if !yield(box) {
						return
					}
				}
			}
		}
	}
}

func (g TextGroup) BoxesCopy() []TextBox {
	var out []TextBox
	for box := range g.BoxesSeq() {
		out = append(out, cloneTextBoxes([]TextBox{box})[0])
	}
	return out
}

// BoxesCopyWithError returns independent textbox snapshots and reports
// deferred font-program or character-procedure failures.
func (g TextGroup) BoxesCopyWithError() ([]TextBox, error) {
	var out []TextBox
	for child := range g.ChildrenSeq() {
		if child.box != nil {
			boxes, err := cloneTextBoxesWithError([]TextBox{*child.box})
			if err != nil {
				return nil, err
			}
			out = append(out, boxes[0])
			continue
		}
		if child.group != nil {
			boxes, err := child.group.BoxesCopyWithError()
			if err != nil {
				return nil, err
			}
			out = append(out, boxes...)
		}
	}
	return out, nil
}

// FinalizeWithError returns an independent text-group snapshot and reports
// deferred font-program or character-procedure failures.
func (g TextGroup) FinalizeWithError() (TextGroup, error) {
	groups, err := cloneTextGroupsWithError([]TextGroup{g})
	if err != nil {
		return TextGroup{}, err
	}
	return groups[0], nil
}

// Finalize returns an independent snapshot of the text group.
func (g TextGroup) Finalize() TextGroup { return cloneTextGroups([]TextGroup{g})[0] }

func (g TextGroup) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Text     string
		Boxes    []TextBox
		Children []TextGroupChild
		BBox     [4]float64
		Vertical bool
	}{g.Text(), g.BoxesCopy(), g.ChildrenCopy(), g.BBox(), g.Vertical()})
}

type LayoutResult struct {
	lines        []TextLine
	paragraphs   []TextParagraph
	textBoxes    []TextBox
	textGroups   []TextGroup
	items        []LayoutItem
	contentItems []ContentObject
	pathItems    []PathObject
}

// LayoutItemKind identifies a child of Playa's analyzed page layout. Text
// boxes are the analyzed text projection; the other kinds retain drawable
// page content that Playa exposes alongside text in LTPage.
type LayoutItemKind string

const (
	LayoutTextBox LayoutItemKind = "text_box"
	LayoutImage   LayoutItemKind = "image"
	LayoutPath    LayoutItemKind = "path"
	LayoutXObject LayoutItemKind = "xobject"
)

// LayoutItem is a borrowed child of a LayoutResult. Use the typed copy
// methods or Finalize before retaining it after the result's lifetime.
type LayoutItem struct {
	kind    LayoutItemKind
	textBox *TextBox
	content *ContentObject
	path    *PathObject
}

// Kind returns the concrete analyzed-layout child kind.
func (i LayoutItem) Kind() LayoutItemKind { return i.kind }

// TextBoxBorrowed returns the text-box payload without copying its lines or
// glyphs. The pointer is borrowed from the layout result; use TextBoxCopy or
// Finalize before retaining it beyond the result's lifetime.
func (i LayoutItem) TextBoxBorrowed() *TextBox { return i.textBox }

// ContentBorrowed returns the drawable payload without copying it. The
// pointer is borrowed from the layout result; use ContentCopy or Finalize
// before retaining it beyond the result's lifetime.
func (i LayoutItem) ContentBorrowed() *ContentObject {
	if i.content != nil {
		return i.content
	}
	if i.path != nil {
		return &ContentObject{kind: ContentPath, path: i.path}
	}
	return nil
}

// TextBoxCopy returns an independent text-box payload when this is a text-box
// item, or nil for a drawable content item.
func (i LayoutItem) TextBoxCopy() *TextBox {
	if i.textBox == nil {
		return nil
	}
	box := cloneTextBoxes([]TextBox{*i.textBox})[0]
	return &box
}

// TextBoxCopyWithError returns an independent text-box payload and reports
// deferred font or glyph failures.
func (i LayoutItem) TextBoxCopyWithError() (*TextBox, error) {
	if i.textBox == nil {
		return nil, nil
	}
	boxes, err := cloneTextBoxesWithError([]TextBox{*i.textBox})
	if err != nil {
		return nil, err
	}
	return &boxes[0], nil
}

// ContentCopy returns an independent drawable content payload when this is a
// non-text layout item, or nil for a text-box item.
func (i LayoutItem) ContentCopy() *ContentObject {
	content := i.ContentBorrowed()
	if content == nil {
		return nil
	}
	snapshot := content.Finalize()
	return &snapshot
}

// ContentCopyWithError returns an independent drawable content payload and
// reports deferred resource failures.
func (i LayoutItem) ContentCopyWithError() (*ContentObject, error) {
	content := i.ContentBorrowed()
	if content == nil {
		return nil, nil
	}
	snapshot, err := content.FinalizeWithError()
	if err != nil {
		return nil, err
	}
	return &snapshot, nil
}

// Finalize returns an independent layout-item snapshot.
func (i LayoutItem) Finalize() LayoutItem {
	if i.textBox != nil {
		return LayoutItem{kind: i.kind, textBox: i.TextBoxCopy()}
	}
	if i.content != nil || i.path != nil {
		return LayoutItem{kind: i.kind, content: i.ContentCopy()}
	}
	return LayoutItem{kind: i.kind}
}

// FinalizeWithError returns an independent layout-item snapshot and reports
// deferred resource or glyph failures.
func (i LayoutItem) FinalizeWithError() (LayoutItem, error) {
	if i.textBox != nil {
		box, err := i.TextBoxCopyWithError()
		if err != nil {
			return LayoutItem{}, err
		}
		return LayoutItem{kind: i.kind, textBox: box}, nil
	}
	if i.content != nil || i.path != nil {
		content, err := i.ContentCopyWithError()
		if err != nil {
			return LayoutItem{}, err
		}
		return LayoutItem{kind: i.kind, content: content}, nil
	}
	return LayoutItem{kind: i.kind}, nil
}

func (i LayoutItem) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Kind    LayoutItemKind `json:"Kind"`
		TextBox *TextBox       `json:"TextBox,omitempty"`
		Content *ContentObject `json:"Content,omitempty"`
	}{Kind: i.kind, TextBox: i.textBox, Content: i.ContentBorrowed()})
}

// Layout extracts the page's text objects and runs the Playa-compatible text
// mining pipeline. Text extraction errors are returned before any layout
// result is published.
func (p Page) Layout(d *Document, opts layout.Options) (LayoutResult, error) {
	if d == nil {
		return LayoutResult{}, errNilDocument
	}
	var glyphSets [][]GlyphObject
	var itemStorage layoutItemStorage
	var err error
	if opts.AllTexts {
		var objects []TextObject
		objects, err = d.PageTextExpanded(p)
		if err == nil {
			glyphSets = textObjectGlyphSets(objects)
			itemStorage, err = p.layoutItems(d)
		}
	} else {
		glyphSets, itemStorage, err = p.layoutGlyphSetsAndItems(d)
	}
	if err != nil {
		return LayoutResult{}, err
	}
	pageBounds := p.layoutPlaneBounds(d)
	result := analyzeLayoutGlyphSets(glyphSets, opts, &pageBounds)
	result.contentItems = itemStorage.contents
	result.pathItems = itemStorage.paths
	result.items = make([]LayoutItem, 0, len(result.textBoxes)+len(itemStorage.order))
	for index := range result.textBoxes {
		result.items = append(result.items, LayoutItem{kind: LayoutTextBox, textBox: &result.textBoxes[index]})
	}
	for _, stored := range itemStorage.order {
		if stored.path {
			result.items = append(result.items, LayoutItem{kind: LayoutPath, path: &result.pathItems[stored.index]})
			continue
		}
		kind, ok := layoutItemKindForContent(result.contentItems[stored.index].Kind())
		if ok {
			result.items = append(result.items, LayoutItem{kind: kind, content: &result.contentItems[stored.index]})
		}
	}
	return result, nil
}

// layoutPlaneBounds mirrors miner.extract_page's LTPage mediabox: transform
// the MediaBox corners into the selected device space, then anchor the plane
// at the origin with the resulting absolute dimensions.
func (p Page) layoutPlaneBounds(d *Document) [4]float64 {
	mediaBox := p.MediaBox(d)
	matrix := p.Matrix(d)
	x0, y0 := matrix.Point(mediaBox[0], mediaBox[1])
	x1, y1 := matrix.Point(mediaBox[2], mediaBox[3])
	return [4]float64{0, 0, absFloat(x0 - x1), absFloat(y0 - y1)}
}

// layoutItems preserves the non-text LTPage children that Playa leaves next
// to analyzed text boxes. Resource-selection and marked-content point objects
// are interpreter details, not miner layout children, so they are omitted.
func (p Page) layoutItems(d *Document) (layoutItemStorage, error) {
	_, items, err := p.layoutGlyphSetsAndItems(d)
	return items, err
}

// layoutGlyphSetsAndItems performs the interpreter pass shared by default text
// mining and non-text LTPage children. Playa feeds one interpreter result to
// the layout analyzer; reparsing the page once for text and again for paths,
// images, and figures creates avoidable page-sized allocation spikes.
// Retaining only glyph slice headers avoids copying the much larger borrowed
// TextObject when the analyzer does not observe any of its other fields.
func (p Page) layoutGlyphSetsAndItems(d *Document) ([][]GlyphObject, layoutItemStorage, error) {
	glyphSets := [][]GlyphObject{}
	var items layoutItemStorage
	var streamErr error
	p.streamingLayoutContentObjects(d, func(object ContentObject, err error) bool {
		if err != nil {
			streamErr = err
			return false
		}
		kind := LayoutItemKind("")
		switch object.Kind() {
		case ContentText:
			if object.text != nil {
				glyphSets = append(glyphSets, object.text.glyphs)
			}
			return true
		case ContentImage:
			kind = LayoutImage
		case ContentPath:
			kind = LayoutPath
		case ContentXObject:
			kind = LayoutXObject
		default:
			return true
		}
		content := object
		if kind == LayoutPath && content.path != nil {
			items.appendPathSubpaths(*content.path)
			return true
		}
		items.appendContent(content)
		return true
	})
	if streamErr != nil {
		return nil, layoutItemStorage{}, streamErr
	}
	return glyphSets, items, nil
}

type layoutItemStorage struct {
	contents []ContentObject
	paths    []PathObject
	order    []layoutStoredItem
}

type layoutStoredItem struct {
	index uint32
	path  bool
}

func (storage *layoutItemStorage) appendContent(content ContentObject) {
	storage.order = append(storage.order, layoutStoredItem{index: uint32(len(storage.contents))})
	storage.contents = append(storage.contents, content)
}

func (storage *layoutItemStorage) appendPathSubpaths(source PathObject) {
	pathStart := len(storage.paths)
	storage.paths = appendSplitPathSubpaths(storage.paths, source)
	count := len(storage.paths) - pathStart
	storage.order = slices.Grow(storage.order, count)
	for index := pathStart; index < len(storage.paths); index++ {
		storage.order = append(storage.order, layoutStoredItem{index: uint32(index), path: true})
	}
}

func layoutItemKindForContent(kind contentconfig.Kind) (LayoutItemKind, bool) {
	switch kind {
	case ContentImage:
		return LayoutImage, true
	case ContentPath:
		return LayoutPath, true
	case ContentXObject:
		return LayoutXObject, true
	default:
		return "", false
	}
}

// splitPathSubpaths mirrors Playa miner.subpaths. A painted PDF path can
// contain multiple move-to initiated subpaths, but Playa exposes each one as
// a separate LTLine/LTRect/LTCurve child of the analyzed page.
func splitPathSubpaths(source PathObject) []PathObject {
	return appendSplitPathSubpaths(nil, source)
}

func appendSplitPathSubpaths(paths []PathObject, source PathObject) []PathObject {
	// Layout items are borrowed until LayoutResult.Finalize. Copy the source
	// path once, then let its subpaths share those owned backing arrays instead
	// of deep-cloning the complete painted path for every move-to segment.
	spec := pathSpec(source)
	rawParts := splitPathSegmentList(spec.RawSegments)
	deviceParts := splitPathSegmentList(spec.Segments)
	count := len(rawParts)
	if len(deviceParts) > count {
		count = len(deviceParts)
	}
	if count == 0 {
		return nil
	}
	paths = slices.Grow(paths, count)
	for index := 0; index < count; index++ {
		var raw, device []geometry.PathSegment
		if index < len(rawParts) {
			raw = rawParts[index]
		}
		if index < len(deviceParts) {
			device = deviceParts[index]
		}
		paths = append(paths, PathObject{data: source.data.WithGeometryBorrowed(raw, device, pathSegmentBBox(device))})
	}
	return paths
}

func splitPathSegmentList(segments []geometry.PathSegment) [][]geometry.PathSegment {
	if len(segments) == 0 {
		return nil
	}
	parts := make([][]geometry.PathSegment, 0, 1)
	start := 0
	for index := 1; index < len(segments); index++ {
		if segments[index].Operator() != "m" {
			continue
		}
		parts = append(parts, segments[start:index])
		start = index
	}
	parts = append(parts, segments[start:])
	return parts
}

func pathSegmentBBox(segments []geometry.PathSegment) [4]float64 {
	var bbox [4]float64
	first := true
	var subpathStart [2]float64
	for _, segment := range segments {
		points := segment.PointsCopy()
		var point [2]float64
		havePoint := false
		if segment.Operator() == "h" {
			point, havePoint = subpathStart, !first
		} else if len(points) > 0 {
			point, havePoint = points[len(points)-1], true
		}
		if !havePoint {
			continue
		}
		if segment.Operator() == "m" {
			subpathStart = point
		}
		if first {
			bbox = [4]float64{point[0], point[1], point[0], point[1]}
			first = false
			continue
		}
		bbox[0] = min(bbox[0], point[0])
		bbox[1] = min(bbox[1], point[1])
		bbox[2] = max(bbox[2], point[0])
		bbox[3] = max(bbox[3], point[1])
	}
	return bbox
}

// LinesSeq yields borrowed lines in layout order.
func (r LayoutResult) LinesSeq() iter.Seq[TextLine] { return textLinesSeq(r.lines) }

// ParagraphsSeq yields borrowed paragraphs in layout order.
func (r LayoutResult) ParagraphsSeq() iter.Seq[TextParagraph] {
	return textParagraphsSeq(r.paragraphs)
}

// TextBoxesSeq yields borrowed textboxes in Playa reading order.
func (r LayoutResult) TextBoxesSeq() iter.Seq[TextBox] { return textBoxesSeq(r.textBoxes) }

// TextGroupsSeq yields borrowed text groups in reading order.
func (r LayoutResult) TextGroupsSeq() iter.Seq[TextGroup] { return textGroupsSeq(r.textGroups) }

// ItemsSeq yields borrowed text-box and drawable children in Playa's analyzed
// page order. Use typed copy methods or Finalize before retaining an item.
func (r LayoutResult) ItemsSeq() iter.Seq[LayoutItem] {
	return func(yield func(LayoutItem) bool) {
		for _, item := range r.items {
			if !yield(item) {
				return
			}
		}
	}
}

func (r LayoutResult) LinesCopy() []TextLine { return cloneTextLines(r.lines) }

func (r LayoutResult) ParagraphsCopy() []TextParagraph { return cloneTextParagraphs(r.paragraphs) }

func (r LayoutResult) TextBoxesCopy() []TextBox { return cloneTextBoxes(r.textBoxes) }

func (r LayoutResult) TextGroupsCopy() []TextGroup { return cloneTextGroups(r.textGroups) }

// ItemsCopy returns independent layout-item snapshots.
func (r LayoutResult) ItemsCopy() []LayoutItem {
	if r.items == nil {
		return nil
	}
	out := make([]LayoutItem, len(r.items))
	for index, item := range r.items {
		out[index] = item.Finalize()
	}
	return out
}

// LinesCopyWithError returns independent line snapshots and reports deferred
// font-program or character-procedure failures.
func (r LayoutResult) LinesCopyWithError() ([]TextLine, error) {
	return cloneTextLinesWithError(r.lines)
}

// ParagraphsCopyWithError returns independent paragraph snapshots and reports
// deferred font-program or character-procedure failures.
func (r LayoutResult) ParagraphsCopyWithError() ([]TextParagraph, error) {
	return cloneTextParagraphsWithError(r.paragraphs)
}

// TextBoxesCopyWithError returns independent textbox snapshots and reports
// deferred font-program or character-procedure failures.
func (r LayoutResult) TextBoxesCopyWithError() ([]TextBox, error) {
	return cloneTextBoxesWithError(r.textBoxes)
}

// TextGroupsCopyWithError returns independent text-group snapshots and reports
// deferred font-program or character-procedure failures.
func (r LayoutResult) TextGroupsCopyWithError() ([]TextGroup, error) {
	return cloneTextGroupsWithError(r.textGroups)
}

// ItemsCopyWithError returns independent layout-item snapshots and reports
// deferred resource or glyph failures.
func (r LayoutResult) ItemsCopyWithError() ([]LayoutItem, error) {
	if r.items == nil {
		return nil, nil
	}
	out := make([]LayoutItem, len(r.items))
	for index, item := range r.items {
		finalized, err := item.FinalizeWithError()
		if err != nil {
			return nil, err
		}
		out[index] = finalized
	}
	return out, nil
}

// FinalizeWithError returns an independent layout snapshot and reports
// deferred font-program or character-procedure failures.
func (r LayoutResult) FinalizeWithError() (LayoutResult, error) {
	lines, err := r.LinesCopyWithError()
	if err != nil {
		return LayoutResult{}, err
	}
	paragraphs, err := r.ParagraphsCopyWithError()
	if err != nil {
		return LayoutResult{}, err
	}
	boxes, err := r.TextBoxesCopyWithError()
	if err != nil {
		return LayoutResult{}, err
	}
	groups, err := r.TextGroupsCopyWithError()
	if err != nil {
		return LayoutResult{}, err
	}
	items, err := r.ItemsCopyWithError()
	if err != nil {
		return LayoutResult{}, err
	}
	return LayoutResult{lines: lines, paragraphs: paragraphs, textBoxes: boxes, textGroups: groups, items: items}, nil
}

// Finalize returns an independent snapshot of the layout result.
func (r LayoutResult) Finalize() LayoutResult {
	return LayoutResult{lines: r.LinesCopy(), paragraphs: r.ParagraphsCopy(), textBoxes: r.TextBoxesCopy(), textGroups: r.TextGroupsCopy(), items: r.ItemsCopy()}
}

func (r LayoutResult) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Lines      []TextLine
		Paragraphs []TextParagraph
		TextBoxes  []TextBox
		TextGroups []TextGroup
		Items      []LayoutItem `json:",omitempty"`
	}{r.lines, r.paragraphs, r.textBoxes, r.textGroups, r.ItemsCopy()})
}

func cloneGlyphObjects(values []GlyphObject) []GlyphObject {
	if values == nil {
		return nil
	}
	out := make([]GlyphObject, len(values))
	for i, value := range values {
		out[i] = finalizeGlyphObject(value)
	}
	return out
}

func cloneGlyphObjectsWithError(values []GlyphObject) ([]GlyphObject, error) {
	if values == nil {
		return nil, nil
	}
	out := make([]GlyphObject, len(values))
	for i, value := range values {
		glyph, err := finalizeGlyphObjectWithError(value)
		if err != nil {
			return nil, err
		}
		out[i] = glyph
	}
	return out, nil
}

func glyphObjectsSeq(values []GlyphObject) iter.Seq[GlyphObject] {
	return func(yield func(GlyphObject) bool) {
		for _, value := range values {
			if !yield(value) {
				return
			}
		}
	}
}

func textWordsSeq(values []TextWord) iter.Seq[TextWord] {
	return func(yield func(TextWord) bool) {
		for _, value := range values {
			if !yield(value) {
				return
			}
		}
	}
}

func textLinesSeq(values []TextLine) iter.Seq[TextLine] {
	return func(yield func(TextLine) bool) {
		for _, value := range values {
			if !yield(value) {
				return
			}
		}
	}
}

func textParagraphsSeq(values []TextParagraph) iter.Seq[TextParagraph] {
	return func(yield func(TextParagraph) bool) {
		for _, value := range values {
			if !yield(value) {
				return
			}
		}
	}
}

func textBoxesSeq(values []TextBox) iter.Seq[TextBox] {
	return func(yield func(TextBox) bool) {
		for _, value := range values {
			if !yield(value) {
				return
			}
		}
	}
}

func textGroupsSeq(values []TextGroup) iter.Seq[TextGroup] {
	return func(yield func(TextGroup) bool) {
		for _, value := range values {
			if !yield(value) {
				return
			}
		}
	}
}

func cloneTextWords(values []TextWord) []TextWord {
	if values == nil {
		return nil
	}
	out := make([]TextWord, len(values))
	for i, value := range values {
		out[i] = value
		out[i].glyphs = cloneGlyphObjects(value.glyphs)
	}
	return out
}

func cloneTextWordsWithError(values []TextWord) ([]TextWord, error) {
	if values == nil {
		return nil, nil
	}
	out := make([]TextWord, len(values))
	for i, value := range values {
		out[i] = value
		glyphs, err := cloneGlyphObjectsWithError(value.glyphs)
		if err != nil {
			return nil, err
		}
		out[i].glyphs = glyphs
	}
	return out, nil
}

func cloneTextLines(values []TextLine) []TextLine {
	if values == nil {
		return nil
	}
	out := make([]TextLine, len(values))
	for i, value := range values {
		out[i] = value
		out[i].glyphs = cloneGlyphObjects(value.glyphs)
		out[i].words = cloneTextWords(value.words)
	}
	return out
}

func cloneTextLinesWithError(values []TextLine) ([]TextLine, error) {
	if values == nil {
		return nil, nil
	}
	out := make([]TextLine, len(values))
	for i, value := range values {
		out[i] = value
		glyphs, err := cloneGlyphObjectsWithError(value.glyphs)
		if err != nil {
			return nil, err
		}
		words, err := cloneTextWordsWithError(value.words)
		if err != nil {
			return nil, err
		}
		out[i].glyphs, out[i].words = glyphs, words
	}
	return out, nil
}

func cloneTextParagraphs(values []TextParagraph) []TextParagraph {
	if values == nil {
		return nil
	}
	out := make([]TextParagraph, len(values))
	for i, value := range values {
		out[i] = value
		out[i].lines = cloneTextLines(value.lines)
	}
	return out
}

func cloneTextParagraphsWithError(values []TextParagraph) ([]TextParagraph, error) {
	if values == nil {
		return nil, nil
	}
	out := make([]TextParagraph, len(values))
	for i, value := range values {
		out[i] = value
		lines, err := cloneTextLinesWithError(value.lines)
		if err != nil {
			return nil, err
		}
		out[i].lines = lines
	}
	return out, nil
}

func cloneTextBoxes(values []TextBox) []TextBox {
	if values == nil {
		return nil
	}
	out := make([]TextBox, len(values))
	for i, value := range values {
		out[i] = value
		out[i].lines = cloneTextLines(value.lines)
	}
	return out
}

func cloneTextBoxesWithError(values []TextBox) ([]TextBox, error) {
	if values == nil {
		return nil, nil
	}
	out := make([]TextBox, len(values))
	for i, value := range values {
		out[i] = value
		lines, err := cloneTextLinesWithError(value.lines)
		if err != nil {
			return nil, err
		}
		out[i].lines = lines
	}
	return out, nil
}

func cloneTextGroups(values []TextGroup) []TextGroup {
	if values == nil {
		return nil
	}
	out := make([]TextGroup, len(values))
	for i, value := range values {
		out[i] = value
		if value.boxes != nil {
			out[i].boxes = cloneTextBoxes(value.boxes)
		}
		if value.children != nil {
			out[i].children = make([]TextGroupChild, len(value.children))
			for j, child := range value.children {
				out[i].children[j] = child.Finalize()
			}
		}
	}
	return out
}

func cloneTextGroupsWithError(values []TextGroup) ([]TextGroup, error) {
	if values == nil {
		return nil, nil
	}
	out := make([]TextGroup, len(values))
	for i, value := range values {
		out[i] = value
		if value.boxes != nil {
			boxes, err := cloneTextBoxesWithError(value.boxes)
			if err != nil {
				return nil, err
			}
			out[i].boxes = boxes
		}
		if value.children != nil {
			out[i].children = make([]TextGroupChild, len(value.children))
			for j, child := range value.children {
				finalized, err := finalizeTextGroupChildWithError(child)
				if err != nil {
					return nil, err
				}
				out[i].children[j] = finalized
			}
		}
	}
	return out, nil
}

func finalizeTextGroupChildWithError(child TextGroupChild) (TextGroupChild, error) {
	if child.box != nil {
		boxes, err := cloneTextBoxesWithError([]TextBox{*child.box})
		if err != nil {
			return TextGroupChild{}, err
		}
		return TextGroupChild{box: &boxes[0]}, nil
	}
	if child.group != nil {
		groups, err := cloneTextGroupsWithError([]TextGroup{*child.group})
		if err != nil {
			return TextGroupChild{}, err
		}
		return TextGroupChild{group: &groups[0]}, nil
	}
	return TextGroupChild{}, nil
}

// AnalyzeLayout performs the first layout pass: glyphs are grouped by a
// nearby baseline, ordered in reading direction, and assigned a union bbox.
// Word/paragraph inference is intentionally a later pass.
func AnalyzeLayout(objects []TextObject, opts layout.Options) LayoutResult {
	return analyzeLayout(objects, opts, nil)
}

func analyzeLayout(objects []TextObject, opts layout.Options, planeBounds *[4]float64) LayoutResult {
	return analyzeLayoutGlyphSets(textObjectGlyphSets(objects), opts, planeBounds)
}

func textObjectGlyphSets(objects []TextObject) [][]GlyphObject {
	glyphSets := make([][]GlyphObject, len(objects))
	for index := range objects {
		glyphSets[index] = objects[index].glyphs
	}
	return glyphSets
}

func analyzeLayoutGlyphSets(glyphSets [][]GlyphObject, opts layout.Options, planeBounds *[4]float64) LayoutResult {
	wordGap := layout.FiniteThreshold(opts.WordMargin, 0.1)
	lineOverlap := layout.FiniteThreshold(opts.LineOverlap, 0.5)
	charMargin := layout.FiniteThreshold(opts.CharMargin, 2)
	paragraphGap := layout.FiniteThreshold(opts.LineMargin, 0.5)
	boxesFlow := layout.BoxesFlow(opts.BoxesFlow)
	type minedLine struct {
		vertical    bool
		baseline    float64
		start       int
		end         int
		sourceIndex int
	}
	glyphCount := 0
	for _, glyphs := range glyphSets {
		glyphCount += len(glyphs)
	}
	layoutGlyphs := make([]GlyphObject, 0, glyphCount)
	lines := []minedLine{}
	appendLine := func(line minedLine) {
		line.sourceIndex = len(lines)
		lines = append(lines, line)
	}
	var previous GlyphObject
	previousIndex := -1
	havePrevious := false
	current := -1
	flushLine := func() {
		if current < 0 {
			return
		}
		current = -1
	}
	processGlyph := func(glyph GlyphObject) {
		glyph = layoutGlyphSnapshot(glyph)
		if glyph.Unmapped() && glyph.Text() == "" {
			// Playa's LTChar preserves an unmapped CID in layout output
			// even though the public glyph text remains absent.
			glyph.setGlyphText(fmt.Sprintf("(cid:%d)", glyph.CID()))
		}
		if !layoutGlyphGeometryFinite(glyph) {
			return
		}
		layoutGlyphs = append(layoutGlyphs, glyph)
		glyphIndex := len(layoutGlyphs) - 1
		if !havePrevious {
			previous = glyph
			previousIndex = glyphIndex
			havePrevious = true
			return
		}
		halign := layoutGlyphsAlign(previous, glyph, false, lineOverlap, charMargin)
		valign := opts.DetectVertical && layoutGlyphsAlign(previous, glyph, true, lineOverlap, charMargin)
		if current >= 0 {
			if (lines[current].vertical && valign) || (!lines[current].vertical && halign) {
				lines[current].end = glyphIndex + 1
				previous = glyph
				previousIndex = glyphIndex
				return
			}
			flushLine()
			previous = glyph
			previousIndex = glyphIndex
			return
		} else if valign && !halign {
			appendLine(minedLine{vertical: true, baseline: glyph.Origin()[0], start: previousIndex, end: glyphIndex + 1})
			current = len(lines) - 1
			previous = glyph
			previousIndex = glyphIndex
			return
		} else if halign && !valign {
			appendLine(minedLine{baseline: previous.Origin()[1], start: previousIndex, end: glyphIndex + 1})
			current = len(lines) - 1
			previous = glyph
			previousIndex = glyphIndex
			return
		}
		appendLine(minedLine{baseline: previous.Origin()[1], start: previousIndex, end: previousIndex + 1})
		previous = glyph
		previousIndex = glyphIndex
	}
	for _, glyphs := range glyphSets {
		for _, glyph := range glyphs {
			processGlyph(glyph)
		}
	}
	if havePrevious {
		if current < 0 {
			appendLine(minedLine{baseline: previous.Origin()[1], start: previousIndex, end: previousIndex + 1})
		}
	}
	sort.SliceStable(lines, func(i, j int) bool {
		if lines[i].vertical != lines[j].vertical {
			return !lines[i].vertical
		}
		if lines[i].vertical {
			return layoutLineFlowScore(layoutGlyphs[lines[i].start], true, boxesFlow) <
				layoutLineFlowScore(layoutGlyphs[lines[j].start], true, boxesFlow)
		}
		return layoutLineFlowScore(layoutGlyphs[lines[i].start], false, boxesFlow) <
			layoutLineFlowScore(layoutGlyphs[lines[j].start], false, boxesFlow)
	})
	sourceLineObjects := make([]TextLine, len(lines))
	out := LayoutResult{lines: make([]TextLine, 0, len(lines))}
	for _, line := range lines {
		lineGlyphs := layoutGlyphs[line.start:line.end]
		var text strings.Builder
		var previous GlyphObject
		havePrevious := false
		first := true
		var bbox [4]float64
		for _, glyph := range lineGlyphs {
			if havePrevious && layoutGlyphHasWordGap(previous, glyph, line.vertical, wordGap) {
				text.WriteByte(' ')
			}
			text.WriteString(glyph.Text())
			previous = glyph
			havePrevious = true
			if first {
				bbox = glyph.BBox()
				first = false
			} else {
				glyphBBox := glyph.BBox()
				bbox[0], bbox[1] = min(bbox[0], glyphBBox[0]), min(bbox[1], glyphBBox[1])
				bbox[2], bbox[3] = max(bbox[2], glyphBBox[2]), max(bbox[3], glyphBBox[3])
			}
		}
		words := []TextWord{}
		wordStart := -1
		flushWord := func(end int) {
			if wordStart < 0 || wordStart >= end {
				return
			}
			wordGlyphs := lineGlyphs[wordStart:end]
			word := TextWord{glyphs: wordGlyphs}
			var wordText strings.Builder
			var wordBBox [4]float64
			firstGlyph := true
			for _, glyph := range wordGlyphs {
				wordText.WriteString(glyph.Text())
				if firstGlyph {
					wordBBox = glyph.BBox()
					firstGlyph = false
				} else {
					glyphBBox := glyph.BBox()
					wordBBox[0], wordBBox[1] = min(wordBBox[0], glyphBBox[0]), min(wordBBox[1], glyphBBox[1])
					wordBBox[2], wordBBox[3] = max(wordBBox[2], glyphBBox[2]), max(wordBBox[3], glyphBBox[3])
				}
			}
			word.data = layout.NewComponent(layout.ComponentSpec{Text: wordText.String(), BBox: wordBBox})
			words = append(words, word)
			wordStart = -1
		}
		for index, glyph := range lineGlyphs {
			firstRune, _ := utf8.DecodeRuneInString(glyph.Text())
			if strings.TrimSpace(glyph.Text()) == "" || unicode.IsSpace(firstRune) {
				flushWord(index)
				continue
			}
			if wordStart >= 0 {
				previous := lineGlyphs[index-1]
				if layoutGlyphHasWordGap(previous, glyph, line.vertical, wordGap) {
					flushWord(index)
				}
			}
			if wordStart < 0 {
				wordStart = index
			}
		}
		flushWord(len(lineGlyphs))
		lineObject := TextLine{
			data:   layout.NewComponent(layout.ComponentSpec{Text: text.String(), BBox: bbox, Vertical: line.vertical}),
			glyphs: lineGlyphs,
			words:  words,
		}
		out.lines = append(out.lines, lineObject)
		sourceLineObjects[line.sourceIndex] = lineObject
	}
	// Playa keeps empty/whitespace-only LTTextLine objects at page level but
	// excludes them from text boxes and groups. Keep both projections while
	// preserving their source order for the page-level line sequence.
	nonEmptyLines := make([]TextLine, 0, len(out.lines))
	for _, line := range out.lines {
		bbox := line.BBox()
		if bbox[2] > bbox[0] && bbox[3] > bbox[1] && (line.Text() == "" || strings.TrimSpace(line.Text()) != "") {
			nonEmptyLines = append(nonEmptyLines, line)
		}
	}
	out.paragraphs = layoutParagraphs(nonEmptyLines, paragraphGap)
	sourceNonEmptyLines := make([]TextLine, 0, len(sourceLineObjects))
	sourceEmptyLines := make([]TextLine, 0)
	for _, line := range sourceLineObjects {
		bbox := line.BBox()
		if bbox[2] > bbox[0] && bbox[3] > bbox[1] && (line.Text() == "" || strings.TrimSpace(line.Text()) != "") {
			sourceNonEmptyLines = append(sourceNonEmptyLines, line)
		} else {
			sourceEmptyLines = append(sourceEmptyLines, line)
		}
	}
	out.textBoxes = textBoxesFromLinesWithin(sourceNonEmptyLines, paragraphGap, boxesFlow, opts.DisableBoxesFlow, planeBounds)
	if !opts.DisableBoxesFlow {
		out.textGroups = layoutTextGroupsWithin(out.textBoxes, boxesFlow, planeBounds)
		assignTextBoxIndexes(out.textGroups)
		out.textBoxes = orderedTextBoxes(out.textGroups)
		out.lines = append(orderedTextLines(out.textBoxes), sourceEmptyLines...)
	} else {
		// Playa's boxes_flow=None path sorts textboxes by their positional
		// reading order. Keep the line projection in that same order instead
		// of retaining the earlier glyph-stream order.
		out.lines = append(orderedTextLines(out.textBoxes), sourceEmptyLines...)
	}
	return out
}

// layoutGlyphSnapshot reproduces Playa miner's pdfminer-compatible character
// box. GlyphObject.BBox is the actual font/content bbox; layout deliberately
// uses the wider standard character box used by LTChar for grouping.
func layoutGlyphSnapshot(glyph GlyphObject) GlyphObject {
	if glyph.font == nil {
		return glyph
	}
	font := glyph.font
	var textbox [4]float64
	if font.vertical {
		vdisp := font.VDisp(glyph.CID())
		position := font.Position(glyph.CID())
		textbox = [4]float64{-position[0], position[1] + vdisp, -position[0] + 1, position[1]}
	} else {
		width := font.HDisp(glyph.CID())
		descent := font.descent * font.fontMatrix[3]
		textbox = [4]float64{0, descent, width, descent + 1}
	}
	if bbox, ok := transformFontBBox(glyph.Matrix(), textbox); ok {
		glyph.setGlyphBBox(bbox)
	}
	return glyph
}

func layoutGlyphGeometryFinite(glyph GlyphObject) bool {
	origin := glyph.Origin()
	displacement := glyph.Displacement()
	bbox := glyph.BBox()
	values := []float64{
		origin[0], origin[1], displacement[0], displacement[1],
		bbox[0], bbox[1], bbox[2], bbox[3], glyph.FontSize(),
	}
	for _, value := range values {
		if math.IsNaN(value) || math.IsInf(value, 0) {
			return false
		}
	}
	return true
}

// textBoxesFromLines follows Playa's line-neighbor grouping independently of
// the higher-level paragraph projection. This keeps the LTTextBox equivalent
// faithful when a Go caller requests both projections.
func textBoxesFromLines(lines []TextLine, lineMargin, boxesFlow float64, disableBoxesFlow bool) []TextBox {
	return textBoxesFromLinesWithin(lines, lineMargin, boxesFlow, disableBoxesFlow, nil)
}

func textBoxesFromLinesWithin(lines []TextLine, lineMargin, boxesFlow float64, disableBoxesFlow bool, planeBounds *[4]float64) []TextBox {
	if len(lines) == 0 {
		return nil
	}
	// Playa's group_textlines does not compute connected components. It walks
	// Plane.find's 50-point grid and replaces each touched line-to-box mapping
	// with a newly assembled box. That detail affects the stable order of lines
	// when several neighboring lines overlap the same candidate.
	groups := make([][]int, 0, len(lines))
	lineGroup := make([]int, len(lines))
	for i := range lineGroup {
		lineGroup[i] = -1
	}
	seenMembers := make([]uint32, len(lines))
	var seenGeneration uint32
	recycled := make([][]int, 0, 2)
	takeMembers := func() []int {
		if len(recycled) == 0 {
			return make([]int, 0, 1)
		}
		best := 0
		for index := 1; index < len(recycled); index++ {
			if cap(recycled[index]) > cap(recycled[best]) {
				best = index
			}
		}
		members := recycled[best][:0]
		recycled[best] = recycled[len(recycled)-1]
		recycled = recycled[:len(recycled)-1]
		return members
	}
	recycleMembers := func(members []int) {
		members = members[:0]
		if len(recycled) < cap(recycled) {
			recycled = append(recycled, members)
			return
		}
		smallest := 0
		for index := 1; index < len(recycled); index++ {
			if cap(recycled[index]) < cap(recycled[smallest]) {
				smallest = index
			}
		}
		if len(recycled) > 0 && cap(members) > cap(recycled[smallest]) {
			recycled[smallest] = members
		}
	}
	plane := newLayoutTextLinePlaneWithin(lines, planeBounds)
	for i := range lines {
		seenGeneration++
		if seenGeneration == 0 {
			clear(seenMembers)
			seenGeneration++
		}
		members := takeMembers()
		appendMember := func(member int) {
			if seenMembers[member] == seenGeneration {
				return
			}
			seenMembers[member] = seenGeneration
			members = append(members, member)
		}
		appendMember(i)
		// Playa expands an existing line-to-box mapping only when that line
		// is reached in the grid-ordered neighbor walk, which matters for
		// equal-height lines whose later box sort is stable.
		for _, j := range plane.neighbors(i, lineMargin) {
			appendMember(j)
			if groupID := lineGroup[j]; groupID >= 0 {
				previous := groups[groupID]
				if previous == nil {
					continue
				}
				groups[groupID] = nil
				for _, member := range previous {
					appendMember(member)
				}
				recycleMembers(previous)
			}
		}
		groupID := len(groups)
		groups = append(groups, members)
		for _, member := range members {
			lineGroup[member] = groupID
		}
	}

	out := make([]TextBox, 0, len(lines))
	seenGroups := make(map[int]struct{}, len(lines))
	for _, groupID := range lineGroup {
		if _, seen := seenGroups[groupID]; seen {
			continue
		}
		seenGroups[groupID] = struct{}{}
		indexes := groups[groupID]
		members := make([]TextLine, 0, len(indexes))
		for _, index := range indexes {
			members = append(members, lines[index])
		}
		vertical := members[0].Vertical()
		sort.SliceStable(members, func(i, j int) bool {
			if vertical {
				return members[i].BBox()[2] > members[j].BBox()[2]
			}
			return members[i].BBox()[3] > members[j].BBox()[3]
		})
		box := TextBox{
			data:  layout.NewComponent(layout.ComponentSpec{Vertical: vertical, Index: -1}),
			lines: members,
		}
		var boxBBox [4]float64
		var builder strings.Builder
		for i, line := range members {
			if i > 0 {
				builder.WriteByte('\n')
			}
			builder.WriteString(line.Text())
			if i == 0 {
				boxBBox = line.BBox()
				continue
			}
			lineBBox := line.BBox()
			boxBBox[0], boxBBox[1] = min(boxBBox[0], lineBBox[0]), min(boxBBox[1], lineBBox[1])
			boxBBox[2], boxBBox[3] = max(boxBBox[2], lineBBox[2]), max(boxBBox[3], lineBBox[3])
		}
		box.data = box.data.WithBBox(boxBBox).WithText(builder.String())
		out = append(out, box)
	}
	if disableBoxesFlow {
		sort.SliceStable(out, func(i, j int) bool {
			if out[i].Vertical() != out[j].Vertical() {
				return out[i].Vertical()
			}
			if out[i].Vertical() {
				if out[i].BBox()[2] != out[j].BBox()[2] {
					return out[i].BBox()[2] > out[j].BBox()[2]
				}
				return out[i].BBox()[1] > out[j].BBox()[1]
			}
			if out[i].BBox()[3] != out[j].BBox()[3] {
				return out[i].BBox()[3] > out[j].BBox()[3]
			}
			return out[i].BBox()[0] < out[j].BBox()[0]
		})
	}
	return out
}

const (
	layoutPlaneGridSize       = 50
	layoutPlaneMaxGridKeys    = 1 << 12
	layoutPlaneMaxGridEntries = 1 << 20
)

type layoutTextLinePlane struct {
	lines    []TextLine
	grid     map[[2]int][]int
	seen     []uint32
	seenGen  uint32
	scratch  []int
	bounds   [4]float64
	bounded  bool
	fallback bool
}

func newLayoutTextLinePlaneWithin(lines []TextLine, bounds *[4]float64) layoutTextLinePlane {
	plane := layoutTextLinePlane{lines: lines, grid: make(map[[2]int][]int)}
	if bounds != nil {
		plane.bounds = *bounds
		plane.bounded = true
	}
	gridEntries := 0
	for index, line := range lines {
		x0, x1, y0, y1, ok := plane.gridRange(line.BBox())
		entries := (x1 - x0) * (y1 - y0)
		if !ok || entries > layoutPlaneMaxGridEntries-gridEntries {
			// A finite but enormous bbox must not materialize every 50-point
			// cell. Source-order scanning retains the former unbounded
			// AnalyzeLayout semantics without exposing an allocation panic.
			plane.grid = nil
			plane.fallback = true
			return plane
		}
		gridEntries += entries
		for y := y0; y < y1; y++ {
			for x := x0; x < x1; x++ {
				key := [2]int{x, y}
				plane.grid[key] = append(plane.grid[key], index)
			}
		}
	}
	plane.seen = make([]uint32, len(lines))
	return plane
}

func (plane *layoutTextLinePlane) neighbors(index int, ratio float64) []int {
	line := plane.lines[index]
	bbox := line.BBox()
	if line.Vertical() {
		distance := ratio * (bbox[2] - bbox[0])
		bbox[0], bbox[2] = bbox[0]-distance, bbox[2]+distance
	} else {
		distance := ratio * (bbox[3] - bbox[1])
		bbox[1], bbox[3] = bbox[1]-distance, bbox[3]+distance
	}
	candidates := plane.candidates(bbox)
	neighbors := make([]int, 0, len(candidates))
	for _, candidate := range candidates {
		candidateBBox := plane.lines[candidate].BBox()
		if candidateBBox[2] <= bbox[0] || bbox[2] <= candidateBBox[0] ||
			candidateBBox[3] <= bbox[1] || bbox[3] <= candidateBBox[1] {
			continue
		}
		if layoutTextLinesNeighbor(line, plane.lines[candidate], ratio) {
			neighbors = append(neighbors, candidate)
		}
	}
	return neighbors
}

func (plane *layoutTextLinePlane) candidates(bbox [4]float64) []int {
	x0, x1, y0, y1, ok := plane.gridRange(bbox)
	plane.scratch = plane.scratch[:0]
	if plane.fallback || !ok {
		if plane.bounded && !layoutPlaneBBoxIntersects(bbox, plane.bounds) {
			return nil
		}
		for index := range plane.lines {
			if plane.bounded && !layoutPlaneBBoxIntersects(plane.lines[index].BBox(), plane.bounds) {
				continue
			}
			plane.scratch = append(plane.scratch, index)
		}
		return plane.scratch
	}
	plane.seenGen++
	if plane.seenGen == 0 {
		clear(plane.seen)
		plane.seenGen++
	}
	seenGen := plane.seenGen
	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			for _, candidate := range plane.grid[[2]int{x, y}] {
				if plane.seen[candidate] == seenGen {
					continue
				}
				plane.seen[candidate] = seenGen
				plane.scratch = append(plane.scratch, candidate)
			}
		}
	}
	return plane.scratch
}

func layoutPlaneBBoxIntersects(a, b [4]float64) bool {
	return a[2] > b[0] && a[0] < b[2] && a[3] > b[1] && a[1] < b[3]
}

func (plane layoutTextLinePlane) gridRange(bbox [4]float64) (int, int, int, int, bool) {
	if !plane.bounded {
		return layoutPlaneGridRange(bbox)
	}
	if !layoutPlaneBBoxIntersects(bbox, plane.bounds) {
		return 0, 0, 0, 0, true
	}
	bbox[0] = max(bbox[0], plane.bounds[0])
	bbox[1] = max(bbox[1], plane.bounds[1])
	bbox[2] = min(bbox[2], plane.bounds[2])
	bbox[3] = min(bbox[3], plane.bounds[3])
	return layoutPlaneGridRange(bbox)
}

func layoutPlaneGridRange(bbox [4]float64) (int, int, int, int, bool) {
	x1Value, y1Value := bbox[2]+layoutPlaneGridSize, bbox[3]+layoutPlaneGridSize
	x0, x0OK := layoutPlaneGridIndex(bbox[0])
	x1, x1OK := layoutPlaneGridIndex(x1Value)
	y0, y0OK := layoutPlaneGridIndex(bbox[1])
	y1, y1OK := layoutPlaneGridIndex(y1Value)
	if !x0OK || !x1OK || !y0OK || !y1OK {
		return 0, 0, 0, 0, false
	}
	widthFloat, heightFloat := float64(x1)-float64(x0), float64(y1)-float64(y0)
	if widthFloat > layoutPlaneMaxGridKeys || heightFloat > layoutPlaneMaxGridKeys {
		return 0, 0, 0, 0, false
	}
	width, height := x1-x0, y1-y0
	if width <= 0 || height <= 0 {
		return 0, 0, 0, 0, true
	}
	if width > layoutPlaneMaxGridKeys || height > layoutPlaneMaxGridKeys/width {
		return 0, 0, 0, 0, false
	}
	return x0, x1, y0, y1, true
}

func layoutPlaneGridIndex(value float64) (int, bool) {
	maxInt := int(^uint(0) >> 1)
	minInt := -maxInt - 1
	if math.IsNaN(value) || math.IsInf(value, 0) || value >= float64(maxInt) || value <= float64(minInt) {
		return 0, false
	}
	integer := int(value)
	quotient := integer / layoutPlaneGridSize
	if integer < 0 && integer%layoutPlaneGridSize != 0 {
		quotient--
	}
	return quotient, true
}

func layoutTextLinesNeighbor(a, b TextLine, ratio float64) bool {
	aBBox, bBBox := a.BBox(), b.BBox()
	if a.Vertical() != b.Vertical() {
		return false
	}
	if a.Vertical() {
		// Playa's LTTextLineVertical.find_neighbors derives d from the
		// queried line, so this relation is intentionally directional.
		tolerance := ratio * (aBBox[2] - aBBox[0])
		if absFloat((aBBox[2]-aBBox[0])-(bBBox[2]-bBBox[0])) > tolerance {
			return false
		}
		if !layoutPlaneAxisIntersects(aBBox[0], aBBox[2], bBBox[0], bBBox[2], tolerance) {
			return false
		}
		return absFloat(aBBox[1]-bBBox[1]) <= tolerance ||
			absFloat(aBBox[3]-bBBox[3]) <= tolerance ||
			absFloat((aBBox[1]+aBBox[3])/2-(bBBox[1]+bBBox[3])/2) <= tolerance
	}
	// LTTextLineHorizontal.find_neighbors likewise uses the source line's
	// height, rather than max(height(a), height(b)).
	tolerance := ratio * (aBBox[3] - aBBox[1])
	if absFloat((aBBox[3]-aBBox[1])-(bBBox[3]-bBBox[1])) > tolerance {
		return false
	}
	if !layoutPlaneAxisIntersects(aBBox[1], aBBox[3], bBBox[1], bBBox[3], tolerance) {
		return false
	}
	return absFloat(aBBox[0]-bBBox[0]) <= tolerance ||
		absFloat(aBBox[2]-bBBox[2]) <= tolerance ||
		absFloat((aBBox[0]+aBBox[2])/2-(bBBox[0]+bBBox[2])/2) <= tolerance
}

func layoutTextGroups(boxes []TextBox, boxesFlow float64) []TextGroup {
	return layoutTextGroupsWithin(boxes, boxesFlow, nil)
}

func layoutTextGroupsWithin(boxes []TextBox, boxesFlow float64, planeBounds *[4]float64) []TextGroup {
	if len(boxes) == 0 {
		return nil
	}
	nodes := make([]*layoutGroupNode, 0, len(boxes)*2-1)
	initialTieBase := len(boxes)
	for i := range boxes {
		box := boxes[i]
		nodes = append(nodes, &layoutGroupNode{group: textGroupFromBox(&box, boxesFlow), box: &box, active: true, tieID: initialTieBase + i})
	}
	initialPairCount := 0
	pairCapacity := 0
	if boxCount := len(boxes); boxCount > 1 && boxCount-1 <= int(^uint(0)>>1)/boxCount {
		initialPairCount = boxCount * (boxCount - 1) / 2
		pairCapacity = initialPairCount
		// The heap starts with every box pair. Merging with m active nodes
		// removes at least one pair and adds m-2 pairs for the new group, so
		// the maximum live count is (n-1)*(n-2)+1. Reserving that exact upper
		// bound avoids repeatedly copying the quadratic pair table while stale
		// entries are waiting to be discarded.
		maxInt := int(^uint(0) >> 1)
		if boxCount == 2 || boxCount-1 <= (maxInt-1)/(boxCount-2) {
			pairCapacity = (boxCount-1)*(boxCount-2) + 1
		}
	}
	pairs := layoutGroupPairHeap{values: make([]layoutGroupPair, 0, pairCapacity), nodes: &nodes}
	for left := 0; left < len(nodes); left++ {
		for right := left + 1; right < len(nodes); right++ {
			pairs.values = append(pairs.values, newLayoutGroupPair(left, right, textGroupDistance(nodes[left].group, nodes[right].group)))
		}
	}
	pairs.init()
	activeCount := len(nodes)
	mergedTieID := 0
	for activeCount > 1 {
		pair := pairs.pop()
		leftID, rightID := pair.leftID(), pair.rightID()
		left, right := nodes[leftID], nodes[rightID]
		if !left.active || !right.active {
			continue
		}
		if pair.isGroupPair(nodes) {
			pair = popPreferredLayoutGroupPair(pair, &pairs, nodes)
			leftID, rightID = pair.leftID(), pair.rightID()
			left, right = nodes[leftID], nodes[rightID]
		}
		if !pair.isBlocked() && textGroupsBlockedNodesWithin(nodes, leftID, rightID, planeBounds) {
			pair.setBlocked()
			pairs.push(pair)
			continue
		}
		children := []TextGroupChild{layoutGroupNodeChild(left), layoutGroupNodeChild(right)}
		merged := textGroupFromChildren(children, boxesFlow)
		left.active = false
		right.active = false
		mergedNode := &layoutGroupNode{group: merged, active: true, tieID: mergedTieID}
		mergedTieID++
		mergedID := len(nodes)
		nodes = append(nodes, mergedNode)
		activeCount--
		for otherID, other := range nodes[:mergedID] {
			if !other.active {
				continue
			}
			pairs.push(newLayoutGroupPair(mergedID, otherID, textGroupDistance(merged, other.group)))
		}
		pairs.compactInactive(activeCount)
	}
	groups := make([]TextGroup, 0, 1)
	for _, node := range nodes {
		if node.active {
			groups = append(groups, node.group)
		}
	}
	sort.SliceStable(groups, func(i, j int) bool {
		if groups[i].Vertical() != groups[j].Vertical() {
			return !groups[i].Vertical()
		}
		return layoutGroupFlowScore(groups[i], boxesFlow) < layoutGroupFlowScore(groups[j], boxesFlow)
	})
	return groups
}

type layoutGroupNode struct {
	group  TextGroup
	box    *TextBox
	active bool
	tieID  int
}

func layoutGroupNodeChild(node *layoutGroupNode) TextGroupChild {
	if node.box != nil {
		return TextGroupChild{box: node.box}
	}
	return TextGroupChild{group: &node.group}
}

type layoutGroupPair struct {
	distance float64
	left     uint32
	right    uint32
}

const layoutGroupPairBlocked = uint32(1 << 31)

func newLayoutGroupPair(left, right int, distance float64) layoutGroupPair {
	return layoutGroupPair{left: uint32(left), right: uint32(right), distance: normalizeLayoutHeapDistance(distance)}
}

func (pair layoutGroupPair) leftID() int  { return int(pair.left &^ layoutGroupPairBlocked) }
func (pair layoutGroupPair) rightID() int { return int(pair.right) }
func (pair layoutGroupPair) isBlocked() bool {
	return pair.left&layoutGroupPairBlocked != 0
}
func (pair *layoutGroupPair) setBlocked() { pair.left |= layoutGroupPairBlocked }
func (pair layoutGroupPair) isGroupPair(nodes []*layoutGroupNode) bool {
	return nodes[pair.leftID()].box == nil && nodes[pair.rightID()].box == nil
}

const layoutHeapDistanceScale = 1_000_000.0

func normalizeLayoutHeapDistance(distance float64) float64 {
	if normalized, ok := normalizeLayoutHeapDistanceFast(distance); ok {
		return normalized
	}
	// Python's round(value, 6) rounds the original binary64 value directly.
	// Formatting preserves the rare half-boundary that scaling can erase.
	normalized, err := strconv.ParseFloat(strconv.FormatFloat(distance, 'f', 6, 64), 64)
	if err != nil {
		return distance
	}
	return normalized
}

func normalizeLayoutHeapDistanceFast(distance float64) (float64, bool) {
	if math.IsNaN(distance) || math.IsInf(distance, 0) || absFloat(distance) > math.MaxFloat64/layoutHeapDistanceScale {
		return distance, true
	}
	// At 2^33 and above the binary64 spacing is at least 2^-19. Rounding to
	// six decimal places moves the value by at most 5e-7, less than half that
	// spacing, so parsing the decimal representation returns the input exactly.
	if absFloat(distance) >= 1<<33 {
		return distance, true
	}
	scaled := distance * layoutHeapDistanceScale
	_, fraction := math.Modf(absFloat(scaled))
	spacing := math.Nextafter(absFloat(scaled), math.Inf(1)) - absFloat(scaled)
	if absFloat(fraction-0.5) <= spacing {
		// Multiplication may have rounded across a half-integer boundary. The
		// decimal formatter below is the exact, uncommon fallback for that tie.
		return 0, false
	}
	return math.RoundToEven(scaled) / layoutHeapDistanceScale, true
}

type layoutGroupPairHeap struct {
	values []layoutGroupPair
	nodes  *[]*layoutGroupNode
}

func (h layoutGroupPairHeap) Len() int { return len(h.values) }

func (h layoutGroupPairHeap) Less(i, j int) bool {
	left, right := h.values[i], h.values[j]
	if left.isBlocked() != right.isBlocked() {
		return !left.isBlocked()
	}
	if left.distance != right.distance {
		return left.distance < right.distance
	}
	nodes := *h.nodes
	leftTie, rightTie := nodes[left.leftID()].tieID, nodes[right.leftID()].tieID
	if leftTie != rightTie {
		return leftTie < rightTie
	}
	return nodes[left.rightID()].tieID < nodes[right.rightID()].tieID
}

// popPreferredLayoutGroupPair removes stale entries from an equal-distance
// bucket and applies the stable surrogate for Playa's allocator-based id tie.
// The heap itself remains a strict total order when the bucket also contains
// group/box pairs.
func popPreferredLayoutGroupPair(first layoutGroupPair, pairs *layoutGroupPairHeap, nodes []*layoutGroupNode) layoutGroupPair {
	preferred := first
	requeue := make([]layoutGroupPair, 0)
	for pairs.Len() > 0 {
		next := pairs.values[0]
		if next.isBlocked() != first.isBlocked() || next.distance != first.distance {
			break
		}
		next = pairs.pop()
		if !nodes[next.leftID()].active || !nodes[next.rightID()].active {
			continue
		}
		if next.isGroupPair(nodes) && layoutGroupPairBaseLess(next, preferred, nodes) {
			requeue = append(requeue, preferred)
			preferred = next
			continue
		}
		requeue = append(requeue, next)
	}
	for _, pair := range requeue {
		pairs.push(pair)
	}
	return preferred
}

func layoutGroupPairBaseLess(left, right layoutGroupPair, nodes []*layoutGroupNode) bool {
	leftTie, rightTie := nodes[left.leftID()].tieID, nodes[right.leftID()].tieID
	if leftTie != rightTie {
		return leftTie < rightTie
	}
	return nodes[left.rightID()].tieID < nodes[right.rightID()].tieID
}

func (h layoutGroupPairHeap) Swap(i, j int) { h.values[i], h.values[j] = h.values[j], h.values[i] }

func (h *layoutGroupPairHeap) init() {
	n := len(h.values)
	for index := n/2 - 1; index >= 0; index-- {
		h.down(index, n)
	}
}

func (h *layoutGroupPairHeap) push(value layoutGroupPair) {
	h.values = append(h.values, value)
	h.up(len(h.values) - 1)
}

func (h *layoutGroupPairHeap) pop() layoutGroupPair {
	n := len(h.values) - 1
	h.Swap(0, n)
	h.down(0, n)
	value := h.values[n]
	h.values[n] = layoutGroupPair{}
	h.values = h.values[:n]
	return value
}

// compactInactive bulk-removes pairs whose nodes were already merged. Leaving
// every stale pair in the heap makes each inevitable discard pay O(log n) for
// a sift-down; rebuilding once stale entries dominate costs O(n) and preserves
// the exact ordering of every live pair.
func (h *layoutGroupPairHeap) compactInactive(activeCount int) {
	if activeCount < 2 {
		h.values = h.values[:0]
		return
	}
	maxInt := int(^uint(0) >> 1)
	if activeCount-1 > maxInt/activeCount {
		return
	}
	livePairs := activeCount * (activeCount - 1) / 2
	// Avoid rebuilding for small heaps or a thin stale tail. The 3/2 ratio
	// keeps compactions geometrically spaced while bounding stale pop work.
	if len(h.values) < 128 || len(h.values) <= livePairs+livePairs/2 {
		return
	}
	nodes := *h.nodes
	values := h.values[:0]
	for _, pair := range h.values {
		if nodes[pair.leftID()].active && nodes[pair.rightID()].active {
			values = append(values, pair)
		}
	}
	clear(h.values[len(values):])
	h.values = values
	h.init()
}

func (h *layoutGroupPairHeap) up(index int) {
	for {
		parent := (index - 1) / 2
		if index == 0 || !h.Less(index, parent) {
			break
		}
		h.Swap(parent, index)
		index = parent
	}
}

func (h *layoutGroupPairHeap) down(index, n int) {
	for {
		left := 2*index + 1
		if left >= n || left < 0 {
			return
		}
		smallest := left
		if right := left + 1; right < n && h.Less(right, left) {
			smallest = right
		}
		if !h.Less(smallest, index) {
			return
		}
		h.Swap(index, smallest)
		index = smallest
	}
}

func textGroupFromBoxes(boxes []TextBox, boxesFlow float64) TextGroup {
	children := make([]TextGroupChild, len(boxes))
	for i := range boxes {
		box := boxes[i]
		children[i] = TextGroupChild{box: &box}
	}
	return textGroupFromChildren(children, boxesFlow)
}

func textGroupFromBox(box *TextBox, boxesFlow float64) TextGroup {
	return textGroupFromChildren([]TextGroupChild{{box: box}}, boxesFlow)
}

func textGroupFromChildren(children []TextGroupChild, boxesFlow float64) TextGroup {
	group := TextGroup{children: append([]TextGroupChild(nil), children...)}
	if len(group.children) == 0 {
		return group
	}
	groupVertical := childVertical(group.children[0])
	groupBBox := childBBox(group.children[0])
	for _, child := range group.children[1:] {
		groupVertical = groupVertical || childVertical(child)
		bbox := childBBox(child)
		groupBBox[0], groupBBox[1] = min(groupBBox[0], bbox[0]), min(groupBBox[1], bbox[1])
		groupBBox[2], groupBBox[3] = max(groupBBox[2], bbox[2]), max(groupBBox[3], bbox[3])
	}
	sort.SliceStable(group.children, func(i, j int) bool {
		return layoutChildFlowScore(group.children[i], groupVertical, boxesFlow) <
			layoutChildFlowScore(group.children[j], groupVertical, boxesFlow)
	})
	var text strings.Builder
	for i, child := range group.children {
		if i > 0 {
			text.WriteByte('\n')
		}
		text.WriteString(childText(child))
	}
	group.data = layout.NewComponent(layout.ComponentSpec{
		Text:     text.String(),
		BBox:     groupBBox,
		Vertical: groupVertical,
	})
	return group
}

func childText(child TextGroupChild) string {
	if child.box != nil {
		return child.box.Text()
	}
	if child.group != nil {
		return child.group.Text()
	}
	return ""
}

func childVertical(child TextGroupChild) bool {
	if child.box != nil {
		return child.box.Vertical()
	}
	return child.group != nil && child.group.Vertical()
}

func childBBox(child TextGroupChild) [4]float64 {
	if child.box != nil {
		return child.box.BBox()
	}
	if child.group != nil {
		return child.group.BBox()
	}
	return [4]float64{}
}

func layoutChildFlowScore(child TextGroupChild, vertical bool, boxesFlow float64) float64 {
	return layoutBoxFlowScoreForMode(TextBox{
		data: layout.NewComponent(layout.ComponentSpec{BBox: childBBox(child), Vertical: childVertical(child)}),
	}, vertical, boxesFlow)
}

func assignTextBoxIndexes(groups []TextGroup) {
	index := 0
	var assign func(TextGroup)
	assign = func(group TextGroup) {
		for _, child := range group.children {
			if child.box != nil {
				child.box.data = child.box.data.WithIndex(index)
				index++
				continue
			}
			if child.group != nil {
				assign(*child.group)
			}
		}
	}
	for _, group := range groups {
		assign(group)
	}
}

// orderedTextBoxes returns the flat LTTextBox projection in the recursive
// IndexAssigner order used by Playa. The returned textboxes borrow their line
// and glyph slices from the layout group tree; TextBoxesCopy/Finalize owns the
// deep-copy boundary for callers that need stable independent values.
func orderedTextBoxes(groups []TextGroup) []TextBox {
	var out []TextBox
	for _, group := range groups {
		for box := range group.BoxesSeq() {
			out = append(out, box)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].Index() < out[j].Index()
	})
	return out
}

// orderedTextLines returns the line projection in the same textbox traversal
// order as Playa's layout projection.
func orderedTextLines(boxes []TextBox) []TextLine {
	var out []TextLine
	for _, box := range boxes {
		for line := range box.LinesSeq() {
			out = append(out, line)
		}
	}
	return out
}

func textGroupDistance(a, b TextGroup) float64 {
	aBBox, bBBox := a.BBox(), b.BBox()
	x0, y0 := min(aBBox[0], bBBox[0]), min(aBBox[1], bBBox[1])
	x1, y1 := max(aBBox[2], bBBox[2]), max(aBBox[3], bBBox[3])
	return textGroupRectangleArea(x1-x0, y1-y0) - textGroupArea(a) - textGroupArea(b)
}

func textGroupArea(group TextGroup) float64 {
	bbox := group.BBox()
	return textGroupRectangleArea(max(bbox[2]-bbox[0], 0), max(bbox[3]-bbox[1], 0))
}

// textGroupRectangleArea forces the multiplication to round before callers
// subtract other areas, matching CPython's non-contracted float operations.
func textGroupRectangleArea(width, height float64) float64 {
	return math.FMA(width, height, 0)
}

func textGroupsBlocked(groups []TextGroup, left, right int) bool {
	a, b := groups[left], groups[right]
	aBBox, bBBox := a.BBox(), b.BBox()
	x0, y0 := min(aBBox[0], bBBox[0]), min(aBBox[1], bBBox[1])
	x1, y1 := max(aBBox[2], bBBox[2]), max(aBBox[3], bBBox[3])
	for i, group := range groups {
		if i == left || i == right {
			continue
		}
		// Playa's Plane.find reports every object whose bbox intersects the
		// candidate rectangle, including objects that extend beyond it.
		bbox := group.BBox()
		if bbox[0] < x1 && x0 < bbox[2] && bbox[1] < y1 && y0 < bbox[3] {
			return true
		}
	}
	return false
}

func textGroupsBlockedNodes(nodes []*layoutGroupNode, left, right int) bool {
	return textGroupsBlockedNodesWithin(nodes, left, right, nil)
}

func textGroupsBlockedNodesWithin(nodes []*layoutGroupNode, left, right int, planeBounds *[4]float64) bool {
	a, b := nodes[left].group, nodes[right].group
	aBBox, bBBox := a.BBox(), b.BBox()
	x0, y0 := min(aBBox[0], bBBox[0]), min(aBBox[1], bBBox[1])
	x1, y1 := max(aBBox[2], bBBox[2]), max(aBBox[3], bBBox[3])
	for index, node := range nodes {
		if index == left || index == right || !node.active {
			continue
		}
		group := node.group
		bbox := group.BBox()
		// Playa keeps objects outside the page Plane in its source sequence,
		// but does not index them in the grid. Plane.find therefore cannot
		// report such an object as an obstacle to a page-bounded merge.
		if planeBounds != nil && !layoutPlaneBBoxIntersects(bbox, *planeBounds) {
			continue
		}
		if bbox[0] < x1 && x0 < bbox[2] && bbox[1] < y1 && y0 < bbox[3] {
			return true
		}
	}
	return false
}

func layoutGroupFlowScore(group TextGroup, boxesFlow float64) float64 {
	bbox := group.BBox()
	if group.Vertical() {
		return -(1+boxesFlow)*(bbox[0]+bbox[2]) - (1-boxesFlow)*bbox[3]
	}
	return (1-boxesFlow)*bbox[0] - (1+boxesFlow)*(bbox[1]+bbox[3])
}

func layoutParagraphs(lines []TextLine, paragraphGap float64) []TextParagraph {
	if len(lines) == 0 {
		return nil
	}
	parents := make([]int, len(lines))
	for i := range parents {
		parents[i] = i
	}
	var find func(int) int
	find = func(index int) int {
		if parents[index] != index {
			parents[index] = find(parents[index])
		}
		return parents[index]
	}
	union := func(left, right int) {
		left, right = find(left), find(right)
		if left != right {
			parents[right] = left
		}
	}
	type lineGeometry struct {
		bbox     [4]float64
		vertical bool
	}
	geometry := make([]lineGeometry, len(lines))
	for index := range lines {
		geometry[index] = lineGeometry{bbox: lines[index].data.BBox(), vertical: lines[index].data.Vertical()}
	}
	plane := newLayoutTextLinePlaneWithin(lines, nil)
	for i := 0; i < len(lines); i++ {
		line := geometry[i]
		height := line.bbox[3] - line.bbox[1]
		if line.vertical {
			height = line.bbox[2] - line.bbox[0]
		}
		if height <= 0 {
			continue
		}
		tolerance := paragraphGap * height
		if tolerance < 0 || math.IsNaN(tolerance) {
			continue
		}
		query := [4]float64{
			line.bbox[0] - tolerance, line.bbox[1] - tolerance,
			line.bbox[2] + tolerance, line.bbox[3] + tolerance,
		}
		for _, j := range plane.candidates(query) {
			if j <= i || line.vertical != geometry[j].vertical {
				continue
			}
			if layoutLineDistanceBounds(line.bbox, geometry[j].bbox, line.vertical) <= tolerance &&
				layoutLinesNeighborBounds(line.bbox, geometry[j].bbox, line.vertical, tolerance) {
				union(i, j)
			}
		}
	}
	groups := make(map[int][]TextLine, len(lines))
	order := make([]int, 0, len(lines))
	for index, line := range lines {
		root := find(index)
		if _, exists := groups[root]; !exists {
			order = append(order, root)
		}
		groups[root] = append(groups[root], line)
	}
	out := make([]TextParagraph, 0, len(order))
	for _, root := range order {
		members := groups[root]
		paragraphBBox := members[0].data.BBox()
		var paragraphText strings.Builder
		for index := range members {
			if index > 0 {
				paragraphText.WriteByte('\n')
			}
			paragraphText.WriteString(members[index].data.Text())
			if index == 0 {
				continue
			}
			lineBBox := members[index].data.BBox()
			paragraphBBox[0], paragraphBBox[1] = min(paragraphBBox[0], lineBBox[0]), min(paragraphBBox[1], lineBBox[1])
			paragraphBBox[2], paragraphBBox[3] = max(paragraphBBox[2], lineBBox[2]), max(paragraphBBox[3], lineBBox[3])
		}
		out = append(out, TextParagraph{
			data: layout.NewComponent(layout.ComponentSpec{
				Text: paragraphText.String(), BBox: paragraphBBox, Vertical: members[0].data.Vertical(),
			}),
			lines: members,
		})
	}
	return out
}

func layoutLineDistance(a, b TextLine) float64 {
	aBBox, bBBox := a.BBox(), b.BBox()
	return layoutLineDistanceBounds(aBBox, bBBox, a.Vertical())
}

func layoutLineDistanceBounds(aBBox, bBBox [4]float64, vertical bool) float64 {
	if vertical {
		return layoutAxisGap(aBBox[0], aBBox[2], bBBox[0], bBBox[2])
	}
	return layoutAxisGap(aBBox[1], aBBox[3], bBBox[1], bBBox[3])
}

func layoutLineFlowScore(glyph GlyphObject, vertical bool, boxesFlow float64) float64 {
	bbox := glyph.BBox()
	if vertical {
		return -(1+boxesFlow)*(bbox[0]+bbox[2]) - (1-boxesFlow)*bbox[3]
	}
	return (1-boxesFlow)*bbox[0] - (1+boxesFlow)*(bbox[1]+bbox[3])
}

func layoutBoxFlowScoreForMode(box TextBox, vertical bool, boxesFlow float64) float64 {
	bbox := box.BBox()
	if vertical {
		return -(1+boxesFlow)*(bbox[0]+bbox[2]) - (1-boxesFlow)*bbox[3]
	}
	return (1-boxesFlow)*bbox[0] - (1+boxesFlow)*(bbox[1]+bbox[3])
}

func layoutGlyphsAlign(previous, next GlyphObject, vertical bool, lineOverlap, charMargin float64) bool {
	previousBBox := previous.BBox()
	nextBBox := next.BBox()
	if vertical {
		overlap, ok := layoutPlayaAxisOverlap(previousBBox[0], previousBBox[2], nextBBox[0], nextBBox[2])
		if !ok || overlap <= min(previousBBox[2]-previousBBox[0], nextBBox[2]-nextBBox[0])*lineOverlap {
			return false
		}
		gap := layoutAxisGap(previousBBox[1], previousBBox[3], nextBBox[1], nextBBox[3])
		width := max(previousBBox[3]-previousBBox[1], nextBBox[3]-nextBBox[1])
		return gap < width*charMargin
	}
	overlap, ok := layoutPlayaAxisOverlap(previousBBox[1], previousBBox[3], nextBBox[1], nextBBox[3])
	if !ok || overlap <= min(previousBBox[3]-previousBBox[1], nextBBox[3]-nextBBox[1])*lineOverlap {
		return false
	}
	gap := layoutAxisGap(previousBBox[0], previousBBox[2], nextBBox[0], nextBBox[2])
	width := max(previousBBox[2]-previousBBox[0], nextBBox[2]-nextBBox[0])
	return gap < width*charMargin
}

// layoutPlayaAxisOverlap mirrors LTComponent.voverlap/hoverlap. Its result
// intentionally differs from geometric intersection for contained intervals:
// Playa measures the smaller distance between opposite edges.
func layoutPlayaAxisOverlap(a0, a1, b0, b1 float64) (float64, bool) {
	if b0 > a1 || a0 > b1 {
		return 0, false
	}
	return min(absFloat(a0-b1), absFloat(a1-b0)), true
}

func layoutAxisGap(a0, a1, b0, b1 float64) float64 {
	if a1 < b0 {
		return b0 - a1
	}
	if b1 < a0 {
		return a0 - b1
	}
	return 0
}

// layoutPlaneAxisIntersects mirrors Plane.find's strict intersection test for
// one expanded query axis. Equal boundaries are excluded, while overlapping
// intervals remain valid even when the expansion is zero.
func layoutPlaneAxisIntersects(a0, a1, b0, b1, expansion float64) bool {
	if expansion < 0 {
		return false
	}
	return b1 > a0-expansion && b0 < a1+expansion
}

func layoutGlyphWordWidth(glyph GlyphObject) float64 {
	bbox := glyph.BBox()
	width := max(bbox[2]-bbox[0], bbox[3]-bbox[1])
	if width <= 0 {
		return max(glyph.FontSize(), 1)
	}
	return width
}

// layoutGlyphHasWordGap mirrors Playa's LTTextLine add methods. The decision
// is based on painted glyph boundaries, rather than text-matrix advances,
// because embedded fonts can report advances that do not match their bbox.
func layoutGlyphHasWordGap(previous, next GlyphObject, vertical bool, wordMargin float64) bool {
	// Playa/Python rounds the multiplication before applying the margin to
	// the glyph boundary. The explicit conversion prevents Go from fusing
	// those operations and changing strict comparisons at a one-ULP tie.
	margin := float64(wordMargin * layoutGlyphWordWidth(next))
	previousBBox := previous.BBox()
	nextBBox := next.BBox()
	if vertical {
		return nextBBox[3]+margin < previousBBox[1]
	}
	return previousBBox[2] < nextBBox[0]-margin
}

func layoutLinesNeighbor(a, b TextLine, tolerance float64) bool {
	aBBox, bBBox := a.BBox(), b.BBox()
	return layoutLinesNeighborBounds(aBBox, bBBox, a.Vertical(), tolerance)
}

func layoutLinesNeighborBounds(aBBox, bBBox [4]float64, vertical bool, tolerance float64) bool {
	if vertical {
		if absFloat((aBBox[2]-aBBox[0])-(bBBox[2]-bBBox[0])) > tolerance {
			return false
		}
		return absFloat(aBBox[1]-bBBox[1]) <= tolerance ||
			absFloat(aBBox[3]-bBBox[3]) <= tolerance ||
			absFloat((aBBox[1]+aBBox[3])/2-(bBBox[1]+bBBox[3])/2) <= tolerance
	}
	if absFloat((aBBox[3]-aBBox[1])-(bBBox[3]-bBBox[1])) > tolerance {
		return false
	}
	return absFloat(aBBox[0]-bBBox[0]) <= tolerance ||
		absFloat(aBBox[2]-bBBox[2]) <= tolerance ||
		absFloat((aBBox[0]+aBBox[2])/2-(bBBox[0]+bBBox[2])/2) <= tolerance
}

func absFloat(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}
