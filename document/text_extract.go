package document

import (
	"math"
	"sort"
	"strings"
	"unicode"

	"github.com/lin-string/go-playa/contentconfig"
	"github.com/lin-string/go-playa/coordinates"
	"github.com/lin-string/go-playa/structuredata"
	"github.com/lin-string/go-playa/textconfig"
)

// DefaultTextExtractionOptions returns the full-page extraction policy.
func DefaultTextExtractionOptions() textconfig.Options { return textconfig.DefaultOptions() }

func snapshotTextExtractionOptions(opts textconfig.Options) textconfig.Options {
	return textconfig.Snapshot(opts)
}

// ExtractText selects tagged or untagged extraction according to the document
// MarkInfo metadata.
func (p Page) ExtractText(d *Document, opts textconfig.Options) (string, error) {
	opts = snapshotTextExtractionOptions(opts)
	if d == nil {
		return "", errNilDocument
	}
	if d.IsTagged() {
		return p.ExtractTextTagged(d, opts)
	}
	return p.ExtractTextUntagged(d, opts)
}

// ExtractTextUntagged extracts text using glyph displacement and line
// position heuristics for pages without logical marked content.
func (p Page) ExtractTextUntagged(d *Document, opts textconfig.Options) (string, error) {
	opts = snapshotTextExtractionOptions(opts)
	if d == nil {
		return "", errNilDocument
	}
	var lines []string
	var current strings.Builder
	var haveCurrent bool
	var lastObjectText string
	var previousOrigin [2]float64
	var previousEnd float64
	var previousGlyphCount int
	var havePrevious bool
	for object, err := range p.texts(d, true, contentconfig.Options{Filter: FilterText}) {
		if err != nil {
			return "", err
		}
		origin := object.Origin()
		vertical := object.Vertical()
		if havePrevious && textNextLine(d, origin, previousOrigin, vertical) {
			if haveCurrent {
				lines = append(lines, current.String())
				current.Reset()
				haveCurrent = false
			}
		}
		objectOffset := glyphOffsetForTextObject(vertical, origin)
		if haveCurrent && textGapExceedsThreshold(textGap(objectOffset, previousEnd, vertical), object.Len() == 1 && previousGlyphCount > 1) && !strings.HasSuffix(lastObjectText, " ") {
			current.WriteByte(' ')
			lastObjectText = " "
		}
		objectText := strings.Builder{}
		objectEnd := objectOffset
		var haveGlyph bool
		var lastGlyphText string
		for _, glyph := range object.glyphs {
			origin := glyph.Origin()
			displacement := glyph.Displacement()
			glyphOffset := origin[0]
			glyphAdvance := displacement[0]
			if vertical {
				glyphOffset = origin[1]
				glyphAdvance = displacement[1]
			}
			if haveGlyph && objectEnd != 0 && textGapExceedsThreshold(textGap(glyphOffset, objectEnd, vertical), false) && lastGlyphText != " " {
				objectText.WriteByte(' ')
			}
			objectText.WriteString(glyph.Text())
			haveGlyph, lastGlyphText = true, glyph.Text()
			objectEnd = glyphOffset + glyphAdvance
		}
		if opts.BBox == nil || textObjectCrossesBBox(object, *opts.BBox) {
			current.WriteString(objectText.String())
			lastObjectText = objectText.String()
			haveCurrent = true
		}
		previousEnd = objectEnd
		previousGlyphCount = object.Len()
		previousOrigin, havePrevious = origin, true
	}
	if haveCurrent {
		lines = append(lines, current.String())
	}
	return strings.Join(lines, "\n"), nil
}

func textGapExceedsThreshold(gap float64, accumulatedBoundary bool) bool {
	// Matrix multiplication can move an exact 0.5 gap just below the
	// threshold, while Playa evaluates the same PDF on the other side. Only
	// an accumulated multi-glyph boundary needs the correction; a direct
	// single-glyph boundary must retain the strict Playa comparison at 0.5.
	return gap > 0.5 || (accumulatedBoundary && gap > 0.5-1e-9)
}

func textGap(offset, previousEnd float64, vertical bool) float64 {
	// PDF page coordinates retain the sign of the glyph displacement used by
	// Playa's spacing heuristic for both writing directions.
	return offset - previousEnd
}

func glyphOffsetForTextObject(vertical bool, origin [2]float64) float64 {
	if vertical {
		return origin[1]
	}
	return origin[0]
}

type taggedTextSection struct {
	advancesOrigin               bool
	precedingSuppressedOrigin    [2]float64
	hasPrecedingSuppressedOrigin bool
	value                        string
	structureActualKey           string
	hasStructureActual           bool
	origin                       [2]float64
	vertical                     bool
	mcid                         int
	hasMCID                      bool
	parentKey                    int
	hasParentKey                 bool
	insideForm                   bool
	selected                     bool
	suppressed                   bool
}

type taggedPageRanks struct {
	byMCID    map[int]int
	ambiguous bool
}

// ExtractTextTagged extracts logical text from marked content, honoring
// ActualText and suppressing Artifact sections.
func (p Page) ExtractTextTagged(d *Document, opts textconfig.Options) (string, error) {
	opts = snapshotTextExtractionOptions(opts)
	if d == nil {
		return "", errNilDocument
	}
	type structureActualTextResult struct {
		text string
		key  string
		ok   bool
	}
	type structureActualTextCacheKey struct {
		mcid         int
		parentKey    int
		hasParentKey bool
	}
	structureActualTextCache := map[structureActualTextCacheKey]structureActualTextResult{}
	structureParentResolver := newStructureActualTextParentResolver(d)
	var rawSection []TextObject
	var sections []taggedTextSection
	finalizeSection := func() {
		if len(rawSection) == 0 {
			return
		}
		last := rawSection[len(rawSection)-1]
		result := taggedTextSection{
			advancesOrigin: true,
			origin:         last.Origin(),
			vertical:       last.Vertical(),
			mcid:           last.MCID(),
			hasMCID:        last.HasMCID(),
			parentKey:      last.parentKey,
			hasParentKey:   last.hasParentKey,
			insideForm:     last.insideForm,
			selected:       true,
		}
		// Tagged extraction groups by the immediate marked-content section.
		// TextObject.MCID intentionally exposes the nearest enclosing MCID instead.
		if len(last.markedStack) > 0 {
			marked := last.markedStack[len(last.markedStack)-1]
			result.mcid, result.hasMCID = marked.MCID(), marked.HasMCID()
		}
		if last.MarkedTag() == "" || last.MarkedTag() == "Artifact" {
			result.suppressed = true
			sections = append(sections, result)
			rawSection = rawSection[:0]
			return
		}
		if opts.BBox != nil {
			result.selected = false
			for _, object := range rawSection {
				if textObjectCrossesBBox(object, *opts.BBox) {
					result.selected = true
					break
				}
			}
		}
		actualText, hasActualText := rawSection[0].MarkedPropertiesCopy()[Name("ActualText")]
		// Playa consumes marked ActualText without retaining a text object for
		// its line-state calculation. It neither starts a line nor advances origin.
		result.advancesOrigin = !hasActualText
		if result.selected {
			if hasActualText {
				if actual, ok := actualText.(String); ok {
					result.value = decodePDFText(actual)
				}
			} else if last.HasMCID() {
				cacheKey := structureActualTextCacheKey{mcid: last.MCID(), parentKey: last.parentKey, hasParentKey: last.hasParentKey}
				actualResult, cached := structureActualTextCache[cacheKey]
				if !cached {
					actualResult.text, actualResult.key, actualResult.ok = p.structureActualText(d, last, structureParentResolver)
					structureActualTextCache[cacheKey] = actualResult
				}
				if actual, key, ok := actualResult.text, actualResult.key, actualResult.ok; ok {
					result.value = actual
					result.structureActualKey = key
					result.hasStructureActual = true
					hasActualText = true
				}
			}
			if !hasActualText {
				var text strings.Builder
				for _, object := range rawSection {
					if opts.BBox != nil && !textObjectCrossesBBox(object, *opts.BBox) {
						continue
					}
					value := object.Text()
					if last.MarkedTag() == "ReversedChars" {
						value = reverseRunes(value)
					}
					if value == "" {
						continue
					}
					text.WriteString(value)
				}
				result.value = text.String()
			}
			result.value = strings.ReplaceAll(result.value, "\u00ad", "")
		}
		sections = append(sections, result)
		rawSection = rawSection[:0]
	}
	for object, err := range p.texts(d, true, contentconfig.Options{Filter: FilterText}) {
		if err != nil {
			return "", err
		}
		if len(object.glyphs) == 0 {
			continue
		}
		if len(rawSection) == 0 || !sameTaggedTextSection(rawSection[0], object) {
			finalizeSection()
		}
		rawSection = append(rawSection, object)
	}
	finalizeSection()
	if ranks, ok := p.logicalTaggedSectionRanks(d, sections); ok {
		ordered := sections[:0]
		var suppressedOrigin [2]float64
		var haveSuppressedOrigin bool
		for _, section := range sections {
			if section.suppressed {
				suppressedOrigin = section.origin
				haveSuppressedOrigin = true
				continue
			}
			// Keep a suppressed section's line-state effect with the following
			// visible section even when the structure tree reorders that section.
			section.precedingSuppressedOrigin = suppressedOrigin
			section.hasPrecedingSuppressedOrigin = haveSuppressedOrigin
			haveSuppressedOrigin = false
			ordered = append(ordered, section)
		}
		sections = ordered
		sort.SliceStable(sections, func(i, j int) bool {
			return ranks[sections[i].mcid] < ranks[sections[j].mcid]
		})
	}

	var out strings.Builder
	var current strings.Builder
	var previousOrigin [2]float64
	var previousMCID struct {
		value int
		has   bool
	}
	var havePrevious bool
	emittedStructureActualText := map[string]bool{}
	for _, section := range sections {
		if section.suppressed {
			// Preserve Playa's line-state behavior for documents whose logical
			// order cannot be established safely.
			previousOrigin = section.origin
			havePrevious = true
			continue
		}
		if section.hasPrecedingSuppressedOrigin {
			previousOrigin = section.precedingSuppressedOrigin
			havePrevious = true
		}
		if section.advancesOrigin && havePrevious && (section.hasMCID != previousMCID.has || (section.hasMCID && section.mcid != previousMCID.value)) && textNextLine(d, section.origin, previousOrigin, section.vertical) {
			if current.Len() > 0 {
				if wrapped := wrapTaggedText(current.String()); wrapped != "" {
					out.WriteString(wrapped)
					out.WriteByte('\n')
				}
				current.Reset()
			}
		}
		previousMCID = struct {
			value int
			has   bool
		}{section.mcid, section.hasMCID}
		if section.advancesOrigin {
			havePrevious = true
			previousOrigin = section.origin
		}
		if !section.selected {
			continue
		}
		if section.hasStructureActual {
			if emittedStructureActualText[section.structureActualKey] {
				continue
			}
			emittedStructureActualText[section.structureActualKey] = true
		}
		current.WriteString(section.value)
	}
	out.WriteString(wrapTaggedText(current.String()))
	return strings.TrimRight(out.String(), "\n"), nil
}

func (p Page) logicalTaggedSectionRanks(d *Document, sections []taggedTextSection) (map[int]int, bool) {
	if p.ref == (Ref{}) || len(sections) < 2 {
		return nil, false
	}
	parentKey, hasParentKey, err := p.ParentKeyWithError(d)
	if err != nil || !hasParentKey {
		return nil, false
	}
	seenSections := make(map[int]bool, len(sections))
	for _, section := range sections {
		if section.suppressed {
			continue
		}
		if section.insideForm || !section.hasMCID || (section.hasParentKey && section.parentKey != parentKey) || seenSections[section.mcid] {
			return nil, false
		}
		seenSections[section.mcid] = true
	}
	ranks, sortable, err := d.logicalTaggedPageRanks(p.ref)
	if err != nil || !sortable {
		return nil, false
	}
	for _, section := range sections {
		if section.suppressed {
			continue
		}
		if _, ok := ranks[section.mcid]; !ok {
			return nil, false
		}
	}
	return ranks, true
}

// logicalTaggedPageRanks builds the document's marked-content reading order
// once, without materializing StructElement child trees. The cache retains only
// page references and integer ranks, so borrowed structure values cannot escape
// the traversal that produced them.
func (d *Document) logicalTaggedPageRanks(page Ref) (map[int]int, bool, error) {
	if d == nil {
		return nil, false, errNilDocument
	}
	if page == (Ref{}) {
		return nil, false, nil
	}
	d.taggedRanksMu.Lock()
	defer d.taggedRanksMu.Unlock()
	if !d.taggedRanksReady {
		pageRanks := make(map[Ref]taggedPageRanks)
		var buildErr error
		for root, err := range d.StructureTreeSeq() {
			if err != nil {
				buildErr = err
				break
			}
			for content, err := range root.ContentsSeq() {
				if err != nil {
					buildErr = err
					break
				}
				if content.Kind() != StructureMarkedContent || !content.HasMCID() || content.HasStream() || !content.HasPage() {
					continue
				}
				ref := content.Page()
				ranks := pageRanks[ref]
				if ranks.byMCID == nil {
					ranks.byMCID = make(map[int]int)
				}
				mcid := content.MCID()
				if _, duplicate := ranks.byMCID[mcid]; duplicate {
					ranks.ambiguous = true
				} else {
					ranks.byMCID[mcid] = len(ranks.byMCID)
				}
				pageRanks[ref] = ranks
			}
			if buildErr != nil {
				break
			}
		}
		d.taggedRanksReady = true
		d.taggedRanksErr = buildErr
		if buildErr == nil {
			d.taggedRanks = pageRanks
		}
	}
	if d.taggedRanksErr != nil {
		return nil, false, d.taggedRanksErr
	}
	ranks, ok := d.taggedRanks[page]
	return ranks.byMCID, ok && !ranks.ambiguous, nil
}

func (p Page) structureActualText(d *Document, object TextObject, parents *structureActualTextParentResolver) (string, string, bool) {
	if parents == nil {
		parents = newStructureActualTextParentResolver(d)
	}
	var element *StructElement
	if object.Page() != (Ref{}) || object.hasParentKey {
		element, _ = d.contentParentWithContext(object.Page(), object.MCID(), object.HasMCID(), object.parentKey, object.hasParentKey)
	} else {
		structure, err := p.Structure(d)
		if err != nil {
			return "", "", false
		}
		elements := structure.ByMCID(object.MCID())
		if len(elements) > 0 {
			element = &elements[0]
		}
	}
	var actualText, key string
	found := false
	seen := map[Ref]bool{}
	for depth := 0; element != nil && depth < 64; depth++ {
		if actual, present := element.actualTextValue(); present {
			actualText, key, found = actual, element.actualTextKey(), true
		}
		if !element.HasParent() || seen[element.Parent()] {
			break
		}
		seen[element.Parent()] = true
		parent := parents.parent(*element)
		if parent == nil {
			break
		}
		element = parent
	}
	return actualText, key, found
}

type structureActualTextParentResolver struct {
	d       *Document
	parents map[Ref]*StructElement
}

func newStructureActualTextParentResolver(d *Document) *structureActualTextParentResolver {
	return &structureActualTextParentResolver{d: d, parents: map[Ref]*StructElement{}}
}

func (r *structureActualTextParentResolver) parent(element StructElement) *StructElement {
	if r == nil || r.d == nil || !element.HasParent() {
		return nil
	}
	ref := element.Parent()
	if parent, cached := r.parents[ref]; cached {
		return parent
	}
	parent := structureActualTextParent(r.d, element)
	r.parents[ref] = parent
	return parent
}

// structureActualTextParent follows only the metadata needed for replacement
// lookup. ParentElementWithError deliberately validates the parent's complete
// child tree; doing that once per MCID makes tagged extraction quadratic on
// large documents.
func structureActualTextParent(d *Document, element StructElement) *StructElement {
	parentRef := element.Parent()
	parentValue, resolved := d.resolveIndirectChain(parentRef)
	if !resolved {
		return nil
	}
	parent, ok := parentValue.(Dict)
	if !ok {
		return nil
	}
	typeValue, typeResolved := d.resolveIndirectChain(parent[Name("Type")])
	if _, present := parent[Name("Type")]; present && !typeResolved {
		return nil
	}
	typ, _ := typeValue.(Name)
	if typ != Name("StructElem") && typ != Name("") {
		return nil
	}
	spec := structuredata.ElementSpec{Type: string(typ), Object: parentRef}
	actualValue, actualResolved := d.resolveIndirectChain(parent[Name("ActualText")])
	actualTextSet := false
	if actual, ok := actualValue.(String); ok {
		spec.ActualText = decodePDFText(actual)
		actualTextSet = actualResolved
	}
	if ancestor, ok := d.finalIndirectRef(parent[Name("P")]); ok {
		spec.Parent, spec.HasParent = ancestor, true
	}
	result := newStructElementValue(spec)
	result.actualTextSet = actualTextSet
	result.sourceRef, result.hasSourceRef = parentRef, true
	return &result
}

// wrapTaggedText mirrors the default width used by Python's textwrap.wrap in
// Playa's tagged extractor while keeping long unspaced runs intact in chunks.
func wrapTaggedText(value string) string {
	const width = 70
	if strings.TrimSpace(value) == "" {
		return ""
	}
	runes := taggedWrapWhitespace(value)
	chunks := taggedWrapChunks(runes)
	lines := make([]string, 0, (len(runes)+width-1)/width)
	for len(chunks) > 0 {
		line := make([]rune, 0, width)
		lineLength := 0
		preserveTrailingWhitespace := false
		if len(lines) > 0 && isTaggedWhitespaceChunk(chunks[0]) {
			chunks = chunks[1:]
		}
		for len(chunks) > 0 {
			chunk := chunks[0]
			if lineLength+len(chunk) <= width {
				line = append(line, chunk...)
				lineLength += len(chunk)
				chunks = chunks[1:]
				continue
			}
			spaceLeft := width - lineLength
			if spaceLeft == 0 && len(chunk) > width {
				// Python's _handle_long_word appends an empty chunk when
				// a full line is followed by an overlong word. The later
				// whitespace check therefore keeps a separator in this
				// specific case.
				preserveTrailingWhitespace = true
			} else if spaceLeft > 0 && len(chunk) > width {
				cut := taggedHyphenCut(chunk, spaceLeft)
				if cut == 0 {
					cut = spaceLeft
				}
				line = append(line, chunk[:cut]...)
				chunks[0] = chunk[cut:]
			}
			break
		}
		for !preserveTrailingWhitespace && len(line) > 0 && isTaggedTrimWhitespace(line[len(line)-1]) {
			line = line[:len(line)-1]
		}
		if len(line) > 0 {
			lines = append(lines, string(line))
		}
	}
	return strings.Join(lines, "\n")
}

// Python textwrap expands tabs at eight-character stops before translating
// ASCII whitespace. Only carriage returns and newlines reset the tab column.
func taggedWrapWhitespace(value string) []rune {
	if !strings.ContainsAny(value, "\t\n\r\v\f") {
		return []rune(value)
	}
	result := make([]rune, 0, len(value))
	column := 0
	for _, r := range value {
		switch r {
		case '\t':
			spaces := 8 - column%8
			for i := 0; i < spaces; i++ {
				result = append(result, ' ')
			}
			column += spaces
		case '\r', '\n':
			result = append(result, ' ')
			column = 0
		case '\v', '\f':
			result = append(result, ' ')
			column++
		default:
			result = append(result, r)
			column++
		}
	}
	return result
}

func taggedWrapChunks(value []rune) [][]rune {
	chunks := make([][]rune, 0, len(value)/2+1)
	for len(value) > 0 {
		whitespace := isTaggedWhitespace(value[0])
		end := 1
		for end < len(value) && isTaggedWhitespace(value[end]) == whitespace {
			end++
		}
		if whitespace {
			chunks = append(chunks, value[:end])
		} else {
			start := 0
			for index, r := range value[:end] {
				if r != '-' || index < 2 || index+2 >= end || !unicode.IsLetter(value[index-2]) || !unicode.IsLetter(value[index-1]) || !unicode.IsLetter(value[index+1]) || !unicode.IsLetter(value[index+2]) {
					continue
				}
				chunks = append(chunks, value[start:index+1])
				start = index + 1
			}
			if start < end {
				chunks = append(chunks, value[start:end])
			}
		}
		value = value[end:]
	}
	return chunks
}

func taggedHyphenCut(value []rune, width int) int {
	if width > len(value) {
		width = len(value)
	}
	for index := width - 1; index > 0; index-- {
		if value[index] != '-' {
			continue
		}
		for _, prefix := range value[:index] {
			if prefix != '-' {
				return index + 1
			}
		}
	}
	return 0
}

func isTaggedWhitespaceChunk(value []rune) bool {
	if len(value) == 0 {
		return false
	}
	for _, r := range value {
		if !isTaggedTrimWhitespace(r) {
			return false
		}
	}
	return true
}

func isTaggedWhitespace(value rune) bool {
	return value == ' ' || value == '\t' || value == '\n' || value == '\r' || value == '\f' || value == '\v'
}

func isTaggedTrimWhitespace(value rune) bool {
	return isTaggedWhitespace(value)
}

func sameTaggedTextSection(first, next TextObject) bool {
	if first.MarkedTag() != next.MarkedTag() || first.HasMCID() != next.HasMCID() || (first.HasMCID() && first.MCID() != next.MCID()) || first.ActualText() != next.ActualText() || len(first.markedStack) != len(next.markedStack) {
		return false
	}
	for i := range first.markedStack {
		left, right := first.markedStack[i], next.markedStack[i]
		if left.Tag() != right.Tag() || left.HasMCID() != right.HasMCID() || (left.HasMCID() && left.MCID() != right.MCID()) || left.ActualText() != right.ActualText() {
			return false
		}
	}
	if len(first.markedStack) > 0 {
		left := first.markedStack[len(first.markedStack)-1]
		right := next.markedStack[len(next.markedStack)-1]
		if left.identity != nil || right.identity != nil {
			return left.identity == right.identity
		}
	}
	return true
}

func textObjectCrossesBBox(object TextObject, bbox [4]float64) bool {
	start := object.Origin()
	displacement := object.Displacement()
	end := [2]float64{start[0] + displacement[0], start[1] + displacement[1]}
	if object.Len() > 0 {
		glyphs := object.glyphs
		last := glyphs[len(glyphs)-1]
		origin := last.Origin()
		displacement := last.Displacement()
		if object.Vertical() {
			end[1] = origin[1] + displacement[1]
		} else {
			end[0] = origin[0] + displacement[0]
		}
	}
	segment := [4]float64{min(start[0], end[0]), min(start[1], end[1]), max(start[0], end[0]), max(start[1], end[1])}
	return max(segment[0], bbox[0]) < min(segment[2], bbox[2]) && max(segment[1], bbox[1]) <= min(segment[3], bbox[3])
}

func textNextLine(d *Document, origin, previous [2]float64, vertical bool) bool {
	if vertical {
		return origin[0]-previous[0] < 0
	}
	delta := origin[1] - previous[1]
	if d.CoordinateSpace() == coordinates.Screen {
		delta = -delta
	}
	return delta < 0
}

func taggedTextObjectStartsNewLine(d *Document, current, previous TextObject) bool {
	if normalX, normalY, localExtent, ok := taggedTextLineNormal(d, current, previous); ok {
		origin, previousOrigin := current.Origin(), previous.Origin()
		// The baseline-normal component distinguishes a genuine line change
		// from movement along a rotated or skewed text baseline. Its direction
		// comes from the independent text line axis, rather than the baseline's
		// sign, so mirrored and reversed writing directions retain their line
		// separation behavior.
		advance := (origin[0]-previousOrigin[0])*normalX + (origin[1]-previousOrigin[1])*normalY
		extent := max(current.Height(), previous.Height())
		if current.Vertical() {
			extent = max(current.Width(), previous.Width())
		}
		if localExtent > 0 {
			extent = min(extent, localExtent)
		}
		return advance > extent/2
	}
	// Preserve the axis-aligned fallback for malformed or degenerate text
	// matrices whose baseline cannot be recovered.
	origin, previousOrigin := current.Origin(), previous.Origin()
	advance := previousOrigin[1] - origin[1]
	extent := max(current.Height(), previous.Height())
	if current.Vertical() {
		advance = previousOrigin[0] - origin[0]
		extent = max(current.Width(), previous.Width())
	} else if d.CoordinateSpace() == coordinates.Screen {
		advance = -advance
	}
	return advance > extent/2
}

func taggedTextLineNormal(d *Document, current, previous TextObject) (float64, float64, float64, bool) {
	baselineX, baselineY, ok := taggedTextBaseline(current, previous)
	if !ok {
		return 0, 0, 0, false
	}
	axisX, axisY, ok := taggedTextLineAxis(current, previous, current.Vertical())
	if !ok {
		return 0, 0, 0, false
	}
	// Remove the baseline component so skewed matrices still measure the
	// origin delta strictly perpendicular to the text baseline.
	dot := axisX*baselineX + axisY*baselineY
	normalX, normalY := axisX-dot*baselineX, axisY-dot*baselineY
	localExtent := math.Hypot(normalX, normalY)
	if localExtent <= 1e-12 || math.IsNaN(localExtent) || math.IsInf(localExtent, 0) {
		return 0, 0, 0, false
	}
	normalX, normalY = normalX/localExtent, normalY/localExtent
	if current.Vertical() {
		// Vertical writing advances within a column along the baseline. The
		// independent first matrix axis identifies the next-column direction;
		// its text-matrix handedness keeps that direction stable when the text
		// baseline itself is reversed. Screen's device transform is not part of
		// the source text handedness.
		if taggedTextHandedness(current, previous, d) > 0 {
			normalX, normalY = -normalX, -normalY
		}
	} else {
		// Horizontal text moves to a following line opposite its independent
		// local Y axis.
		normalX, normalY = -normalX, -normalY
	}
	return normalX, normalY, localExtent, true
}

func taggedTextLineAxis(current, previous TextObject, vertical bool) (float64, float64, bool) {
	objects := [...]TextObject{current, previous}
	for _, object := range objects {
		matrix := object.Matrix()
		axisX, axisY := matrix[2], matrix[3]
		if vertical {
			axisX, axisY = matrix[0], matrix[1]
		}
		if _, _, ok := normalizeTaggedTextBaseline(axisX, axisY); ok {
			return axisX, axisY, true
		}
		matrix = object.TextMatrix()
		axisX, axisY = matrix[2], matrix[3]
		if vertical {
			axisX, axisY = matrix[0], matrix[1]
		}
		if _, _, ok := normalizeTaggedTextBaseline(axisX, axisY); ok {
			return axisX, axisY, true
		}
	}
	return 0, 0, false
}

func taggedTextHandedness(current, previous TextObject, d *Document) float64 {
	objects := [...]TextObject{current, previous}
	for _, object := range objects {
		matrix := object.TextMatrix()
		if handedness, ok := taggedTextMatrixHandedness(matrix); ok {
			return handedness
		}
		matrix = object.Matrix()
		if handedness, ok := taggedTextMatrixHandedness(matrix); ok {
			if d.CoordinateSpace() == coordinates.Screen {
				handedness = -handedness
			}
			return handedness
		}
	}
	return 1
}

func taggedTextMatrixHandedness(matrix [6]float64) (float64, bool) {
	determinant := matrix[0]*matrix[3] - matrix[1]*matrix[2]
	if math.IsNaN(determinant) || math.IsInf(determinant, 0) || math.Abs(determinant) <= 1e-12 {
		return 0, false
	}
	if determinant < 0 {
		return -1, true
	}
	return 1, true
}

func taggedTextBaseline(current, previous TextObject) (float64, float64, bool) {
	objects := [...]TextObject{current, previous}
	for _, object := range objects {
		displacement := object.Displacement()
		if x, y, ok := normalizeTaggedTextBaseline(displacement[0], displacement[1]); ok {
			return x, y, true
		}
		matrix := object.Matrix()
		if x, y, ok := normalizeTaggedTextBaseline(matrix[0], matrix[1]); ok {
			return x, y, true
		}
		matrix = object.TextMatrix()
		if x, y, ok := normalizeTaggedTextBaseline(matrix[0], matrix[1]); ok {
			return x, y, true
		}
	}
	return 0, 0, false
}

func normalizeTaggedTextBaseline(x, y float64) (float64, float64, bool) {
	length := math.Hypot(x, y)
	if !math.IsNaN(length) && !math.IsInf(length, 0) && length > 1e-12 {
		return x / length, y / length, true
	}
	return 0, 0, false
}

func reverseRunes(value string) string {
	runes := []rune(value)
	for left, right := 0, len(runes)-1; left < right; left, right = left+1, right-1 {
		runes[left], runes[right] = runes[right], runes[left]
	}
	return string(runes)
}
