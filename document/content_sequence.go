package document

import (
	"encoding/json"
	"errors"
	"fmt"
	"iter"
	"strings"
	"sync"
)

// ErrContentMCIDOutOfRange reports an index outside a content sequence's
// known MCID range.
var ErrContentMCIDOutOfRange = errors.New("playa: marked-content ID out of range")

// ContentSection is the content-object collection associated with one MCID.
// Objects are borrowed from the sequence; use ObjectsCopy or Finalize before
// retaining an independent snapshot.
type ContentSection struct {
	mcid    int
	hasMCID bool
	objects []ContentObject
}

func contentObjectsSeq(values []ContentObject) iter.Seq2[ContentObject, error] {
	return func(yield func(ContentObject, error) bool) {
		for _, value := range values {
			if !yield(value, nil) {
				return
			}
		}
	}
}

func (o ContentObject) textPayload() (*TextObject, bool) {
	if o.kind != ContentText || o.text == nil {
		return nil, false
	}
	return o.text, true
}

// isMarkedContentObject reports whether an object belongs to Playa's
// content-object stream. FilterAll also exposes resource-selection events for
// the Go API, but Playa's marked_content sequence does not include those
// events and they must not split adjacent objects with the same MCID.
func (o ContentObject) isMarkedContentObject() bool {
	switch o.kind {
	case ContentText, ContentPath, ContentImage, ContentTag, ContentXObject:
		return true
	default:
		return false
	}
}

// MCID returns the section's marked-content identifier.
func (s ContentSection) MCID() int { return s.mcid }

// HasMCID reports whether the section corresponds to an explicitly present
// marked-content section. An absent slot returned by ContentSequence.At has
// no MCID even though it is addressable within the sequence range.
func (s ContentSection) HasMCID() bool { return s.hasMCID }

// Len returns the number of interpreted content objects in the section.
func (s ContentSection) Len() int { return len(s.objects) }

// ObjectsSeq yields borrowed content objects in section order.
func (s ContentSection) ObjectsSeq() iter.Seq2[ContentObject, error] {
	return contentObjectsSeq(s.objects)
}

// ObjectsCopy returns independent content-object snapshots.
func (s ContentSection) ObjectsCopy() []ContentObject {
	if s.objects == nil {
		return nil
	}
	out := make([]ContentObject, len(s.objects))
	for i, object := range s.objects {
		out[i] = object.Finalize()
	}
	return out
}

// ObjectsCopyWithError returns independent snapshots and reports deferred
// resource or font-program failures.
func (s ContentSection) ObjectsCopyWithError() ([]ContentObject, error) {
	if s.objects == nil {
		return nil, nil
	}
	out := make([]ContentObject, len(s.objects))
	for i, object := range s.objects {
		finalized, err := object.FinalizeWithError()
		if err != nil {
			return nil, err
		}
		out[i] = finalized
	}
	return out, nil
}

// TextsSeq yields Unicode text projected from text objects in section order.
// Texts whose immediately enclosing marked context has no MCID are omitted.
// ActualText replaces the object's decoded text when it is present, and soft
// hyphens are omitted to match Playa's ContentSection.texts.
func (s ContentSection) TextsSeq() iter.Seq[string] {
	return func(yield func(string) bool) {
		for _, object := range s.objects {
			text, ok := contentSectionText(object)
			if !ok {
				continue
			}
			if !yield(text) {
				return
			}
		}
	}
}

// TextsCopy returns the section's projected text strings.
func (s ContentSection) TextsCopy() []string {
	texts := s.texts()
	if texts == nil {
		return nil
	}
	return append([]string(nil), texts...)
}

func (s ContentSection) texts() []string {
	var texts []string
	for _, object := range s.objects {
		value, ok := contentSectionText(object)
		if !ok {
			continue
		}
		texts = append(texts, value)
	}
	return texts
}

func contentSectionText(object ContentObject) (string, bool) {
	text, ok := object.textPayload()
	if !ok || !text.HasMCID() {
		return "", false
	}
	// TextObject.MCID identifies the nearest structural ancestor, while
	// Playa's ContentSection.texts selects the immediate marked context.
	if stack := text.markedStack; len(stack) > 0 && !stack[len(stack)-1].HasMCID() {
		return "", false
	}
	value := text.Text()
	if _, hasActualText := text.MarkedPropertiesCopy()[Name("ActualText")]; hasActualText {
		value = text.ActualText()
	}
	return strings.ReplaceAll(value, "\u00ad", ""), true
}

// Finalize returns an independent section snapshot.
func (s ContentSection) Finalize() ContentSection {
	clone := s
	clone.objects = s.ObjectsCopy()
	return clone
}

// FinalizeWithError returns an independent section snapshot and reports
// deferred resource or font-program failures.
func (s ContentSection) FinalizeWithError() (ContentSection, error) {
	objects, err := s.ObjectsCopyWithError()
	if err != nil {
		return ContentSection{}, err
	}
	clone := s
	clone.objects = objects
	return clone, nil
}

func (s ContentSection) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		MCID    int             `json:"mcid"`
		HasMCID bool            `json:"has_mcid"`
		Objects []ContentObject `json:"objects"`
		Texts   []string        `json:"texts"`
	}{s.mcid, s.hasMCID, s.ObjectsCopy(), s.TextsCopy()})
}

type contentSequenceState struct {
	once      sync.Once
	load      func() (iter.Seq2[ContentObject, error], []int)
	sections  map[int]ContentSection
	pageOrder []int
	maxMCID   int
	err       error
}

// ContentSequence is the MCID-indexed content projection exposed by Playa's
// Page.marked_content and XObjectObject.marked_content. It does not interpret
// content until one of its accessors is used.
type ContentSequence struct {
	state *contentSequenceState
}

func newContentSequence(load func() (iter.Seq2[ContentObject, error], []int)) ContentSequence {
	return ContentSequence{state: &contentSequenceState{load: load}}
}

func (s ContentSequence) materialize() *contentSequenceState {
	if s.state == nil {
		return &contentSequenceState{sections: map[int]ContentSection{}}
	}
	s.state.once.Do(func() {
		if s.state.load == nil {
			if s.state.sections == nil {
				s.state.sections = map[int]ContentSection{}
			}
			return
		}
		s.state.sections = make(map[int]ContentSection)
		s.state.maxMCID = 0
		sequence, knownIDs := s.state.load()
		if sequence == nil {
			s.state.err = errNilDocument
			return
		}
		seenOrder := make(map[int]bool)
		var currentID int
		currentHasID := false
		var current []ContentObject
		flush := func() {
			if !currentHasID {
				return
			}
			section := ContentSection{mcid: currentID, hasMCID: true, objects: current}
			// itertools.groupby in Playa replaces duplicate IDs while preserving
			// the original dictionary insertion order.
			if !seenOrder[currentID] {
				s.state.pageOrder = append(s.state.pageOrder, currentID)
				seenOrder[currentID] = true
			}
			s.state.sections[currentID] = section
			if currentID > s.state.maxMCID {
				s.state.maxMCID = currentID
			}
			current = nil
			currentHasID = false
		}
		for object, err := range sequence {
			if err != nil {
				s.state.err = err
				return
			}
			if !object.isMarkedContentObject() {
				continue
			}
			mcid, hasMCID := object.MCIDValue()
			if !hasMCID {
				flush()
				continue
			}
			if !currentHasID || currentID != mcid {
				flush()
				currentID, currentHasID = mcid, true
			}
			// ContentSequence owns a borrowed view while it is being iterated.
			// Keep the interpreter object here and defer deep copying of fonts,
			// glyphs, and resources to ObjectsCopy/Finalize. Finalizing every
			// object during materialization duplicates large CMaps once per text
			// object and defeats the lazy borrowed-iterator contract.
			current = append(current, object)
		}
		flush()
		// Playa records empty marked-content starts separately from emitted
		// objects. Add those missing sections after content-order entries, as
		// its insertion-ordered mapping does.
		for _, mcid := range knownIDs {
			if mcid > s.state.maxMCID {
				s.state.maxMCID = mcid
			}
			if _, exists := s.state.sections[mcid]; exists {
				continue
			}
			s.state.sections[mcid] = ContentSection{mcid: mcid, hasMCID: true}
			s.state.pageOrder = append(s.state.pageOrder, mcid)
		}
	})
	return s.state
}

// Len returns the number of addressable MCID slots. The zero-length content
// sequence follows Playa's max-MCID-plus-one convention and therefore has a
// minimum length of one.
func (s ContentSequence) Len() (int, error) {
	state := s.materialize()
	if state.err != nil {
		return 0, state.err
	}
	return state.maxMCID + 1, nil
}

// At returns the content section for an MCID. Missing slots inside the known
// range return an empty section, matching Playa's sequence indexing behavior.
func (s ContentSequence) At(mcid int) (ContentSection, error) {
	state := s.materialize()
	if state.err != nil {
		return ContentSection{}, state.err
	}
	if mcid > state.maxMCID {
		return ContentSection{}, fmt.Errorf("%w: %d", ErrContentMCIDOutOfRange, mcid)
	}
	if section, ok := state.sections[mcid]; ok {
		return section, nil
	}
	return ContentSection{}, nil
}

// PageOrderSeq yields sections in their first content appearance order.
func (s ContentSequence) PageOrderSeq() iter.Seq2[ContentSection, error] {
	return func(yield func(ContentSection, error) bool) {
		state := s.materialize()
		if state.err != nil {
			yield(ContentSection{}, state.err)
			return
		}
		for _, mcid := range state.pageOrder {
			if !yield(state.sections[mcid], nil) {
				return
			}
		}
	}
}

// PageOrderCopy returns independent sections in content order.
func (s ContentSequence) PageOrderCopy() ([]ContentSection, error) {
	var out []ContentSection
	for section, err := range s.PageOrderSeq() {
		if err != nil {
			return nil, err
		}
		out = append(out, section.Finalize())
	}
	return out, nil
}

// Finalize returns an independent materialized sequence. Deferred load errors
// are represented by an empty sequence; callers needing the error should use
// FinalizeWithError.
func (s ContentSequence) Finalize() ContentSequence {
	clone, _ := s.FinalizeWithError()
	return clone
}

// FinalizeWithError returns an independent materialized sequence.
func (s ContentSequence) FinalizeWithError() (ContentSequence, error) {
	state := s.materialize()
	if state.err != nil {
		return ContentSequence{}, state.err
	}
	cloneState := &contentSequenceState{
		sections:  make(map[int]ContentSection, len(state.sections)),
		pageOrder: append([]int(nil), state.pageOrder...),
		maxMCID:   state.maxMCID,
	}
	for mcid, section := range state.sections {
		cloneState.sections[mcid] = section.Finalize()
	}
	return ContentSequence{state: cloneState}, nil
}

func (s ContentSequence) MarshalJSON() ([]byte, error) {
	length, err := s.Len()
	if err != nil {
		return nil, err
	}
	sections, err := s.PageOrderCopy()
	if err != nil {
		return nil, err
	}
	return json.Marshal(struct {
		Length   int              `json:"length"`
		Sections []ContentSection `json:"page_order"`
	}{length, sections})
}

func newPageContentSequence(d *Document, p Page) ContentSequence {
	return newContentSequence(func() (iter.Seq2[ContentObject, error], []int) {
		if d == nil {
			return nil, nil
		}
		known, err := d.contentSequenceMCIDs(p)
		if err != nil {
			return errorContentObjectSeq(err), nil
		}
		return p.Interp(d, DefaultContentOptions()), known
	})
}

func newXObjectContentSequence(d *Document, x XObjectObject) ContentSequence {
	return newContentSequence(func() (iter.Seq2[ContentObject, error], []int) {
		if d == nil {
			return nil, nil
		}
		view := x.pageView(d)
		known, err := d.contentSequenceMCIDs(view)
		if err != nil {
			return errorContentObjectSeq(err), nil
		}
		return x.Interp(d, DefaultContentOptions()), known
	})
}

func errorContentObjectSeq(err error) iter.Seq2[ContentObject, error] {
	return func(yield func(ContentObject, error) bool) {
		yield(ContentObject{}, err)
	}
}

// contentSequenceMCIDs records explicit BDC MCIDs, including sections with no
// emitted content objects. It walks only direct page operators, matching the
// non-flattened page interpreter; a Form XObject has its own sequence.
func (d *Document) contentSequenceMCIDs(p Page) ([]int, error) {
	iterator := newContentOpIterator(d, p, false)
	var properties Dict
	propertiesReady := false
	propertiesFor := func() Dict {
		if !propertiesReady {
			properties, _ = d.pagePropertiesChecked(p)
			propertiesReady = true
		}
		return properties
	}
	var ids []int
	seen := map[int]bool{}
	for {
		op, err, ok := iterator.next()
		if err != nil {
			return nil, err
		}
		if !ok {
			return ids, nil
		}
		if op.formBoundary != 0 || op.operatorValue() != "BDC" {
			continue
		}
		node, err := markedNodeForOp(d, op, propertiesFor, p.ref)
		if err != nil {
			return nil, err
		}
		if node == nil || !node.hasMCID || seen[node.mcid] {
			continue
		}
		seen[node.mcid] = true
		ids = append(ids, node.mcid)
	}
}
