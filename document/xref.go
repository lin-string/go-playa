package document

import (
	"bytes"
	"fmt"
	"math"
	"sort"
	"strconv"

	"github.com/lin-string/go-playa/documentdata"
)

type xrefEntry struct {
	// offset is the file offset for ordinary entries and the object-stream
	// number for compressed entries. Sharing the slot avoids retaining three
	// machine words for mutually exclusive representations in every xref.
	offset int
	index  int
	flags  uint8
}

const (
	xrefEntryFree uint8 = 1 << iota
	xrefEntryCompressed
	xrefEntryPlayaHidden
	xrefEntryPlayaIterVisible
)

func newXRefEntry(offset int, free, playaHidden, playaIterVisible bool) xrefEntry {
	var flags uint8
	if free {
		flags |= xrefEntryFree
	}
	if playaHidden {
		flags |= xrefEntryPlayaHidden
	}
	if playaIterVisible {
		flags |= xrefEntryPlayaIterVisible
	}
	return xrefEntry{offset: offset, flags: flags}
}

func newCompressedXRefEntry(objectStream, objectIndex int, playaHidden, playaIterVisible bool) xrefEntry {
	entry := newXRefEntry(objectStream, false, playaHidden, playaIterVisible)
	entry.index = objectIndex
	entry.flags |= xrefEntryCompressed
	return entry
}

func (e xrefEntry) isFree() bool             { return e.flags&xrefEntryFree != 0 }
func (e xrefEntry) isCompressed() bool       { return e.flags&xrefEntryCompressed != 0 }
func (e xrefEntry) isPlayaHidden() bool      { return e.flags&xrefEntryPlayaHidden != 0 }
func (e xrefEntry) isPlayaIterVisible() bool { return e.flags&xrefEntryPlayaIterVisible != 0 }
func (e xrefEntry) objectIndex() int         { return e.index }
func (e xrefEntry) objectStream() int {
	if e.isCompressed() {
		return e.offset
	}
	return 0
}
func (e xrefEntry) fileOffset() int {
	if e.isCompressed() {
		return 0
	}
	return e.offset
}

type xrefEntryFilter func(Ref, xrefEntry) bool

func skipXRefEntry(Ref, xrefEntry) bool { return false }

// XRefEntry is the dependency-free cross-reference value model.
type XRefEntry = documentdata.XRefEntry

// XRefTable is an immutable snapshot of one cross-reference revision.
type XRefTable = documentdata.XRefTable

func (d *Document) parseXRef() (map[Ref]xrefEntry, Dict, error) {
	i := bytes.LastIndex(d.data, []byte("startxref"))
	if i < 0 {
		return nil, nil, withParseContext(fmt.Errorf("playa: startxref not found"), i, "xref")
	}
	if i > 0 && d.data[i-1] != '\n' && d.data[i-1] != '\r' {
		return nil, nil, withParseContext(fmt.Errorf("playa: malformed startxref marker"), i, "xref")
	}
	markerEnd := i + len("startxref")
	if markerEnd >= len(d.data) || !xrefSpace(d.data[markerEnd]) {
		return nil, nil, withParseContext(fmt.Errorf("playa: malformed startxref marker"), i, "xref")
	}
	r := bytes.TrimSpace(d.data[markerEnd:])
	lineEnd := bytes.IndexAny(r, "\r\n")
	var tail []byte
	if lineEnd >= 0 {
		tail = bytes.TrimSpace(r[lineEnd:])
		r = r[:lineEnd]
	}
	if len(tail) > 0 {
		if !bytes.HasPrefix(tail, []byte("%%EOF")) || len(bytes.TrimSpace(tail[len("%%EOF"):])) != 0 {
			return nil, nil, withParseContext(fmt.Errorf("playa: trailing startxref data"), i, "xref")
		}
	}
	off, err := strconv.Atoi(string(bytes.TrimSpace(r)))
	if err != nil {
		return nil, nil, withParseContext(err, i, "xref")
	}
	return d.parseXRefAt(off)
}

func (d *Document) parseXRefAt(off int) (map[Ref]xrefEntry, Dict, error) {
	return d.parseXRefAtSeen(off, map[int]bool{})
}

func (d *Document) parseXRefLocalAt(off int) (map[Ref]xrefEntry, Dict, string, error) {
	return d.parseXRefAtMode(off, map[int]bool{}, false, false)
}

func (d *Document) parseXRefAtSeen(off int, seen map[int]bool) (entries map[Ref]xrefEntry, trailer Dict, err error) {
	entries, trailer, _, err = d.parseXRefAtMode(off, seen, true, true)
	return entries, trailer, err
}

func (d *Document) parseXRefAtMode(off int, seen map[int]bool, mergePrevious, mergeHybrid bool, filters ...xrefEntryFilter) (entries map[Ref]xrefEntry, trailer Dict, kind string, err error) {
	var filter xrefEntryFilter
	if len(filters) > 0 {
		filter = filters[0]
	}
	defer func() {
		if err != nil {
			err = withParseContext(err, off, "xref")
		}
	}()
	if off < 0 || off >= len(d.data) {
		return nil, nil, "", fmt.Errorf("playa: invalid xref offset")
	}
	if seen[off] {
		return nil, nil, "", fmt.Errorf("playa: xref Prev cycle at offset %d", off)
	}
	seen[off] = true
	if !bytes.HasPrefix(bytes.TrimSpace(d.data[off:]), []byte("xref")) {
		entries, trailer, err := d.parseXRefStreamFiltered(off, filter)
		if err != nil {
			return nil, nil, "stream", err
		}
		if !mergePrevious {
			return entries, trailer, "stream", nil
		}
		entries, trailer, err = d.mergePreviousXRefSeen(entries, trailer, seen, filter)
		return entries, trailer, "stream", err
	}
	x := bytes.TrimSpace(d.data[off:])
	x = x[len("xref"):]
	entries = map[Ref]xrefEntry{}
	var ranges [][2]int
	for {
		x = bytes.TrimSpace(x)
		if bytes.HasPrefix(x, []byte("trailer")) {
			x = x[len("trailer"):]
			break
		}
		line, rest, ok := xrefLine(x)
		if !ok {
			return nil, nil, "table", fmt.Errorf("playa: malformed xref")
		}
		first, count, ok := parseXRefSubsection(line)
		if !ok {
			return nil, nil, "table", fmt.Errorf("playa: malformed xref subsection")
		}
		if first < 0 || count < 0 {
			return nil, nil, "table", fmt.Errorf("playa: malformed xref subsection")
		}
		if count > 0 && first > int(^uint(0)>>1)-(count-1) {
			return nil, nil, "table", fmt.Errorf("playa: xref subsection object number overflow")
		}
		end := first + count
		position := sort.Search(len(ranges), func(index int) bool { return ranges[index][0] >= first })
		if count > 0 {
			if position > 0 && ranges[position-1][1] > first {
				return nil, nil, "table", fmt.Errorf("playa: duplicate xref object %d", first)
			}
			if position < len(ranges) && ranges[position][0] < end {
				return nil, nil, "table", fmt.Errorf("playa: duplicate xref object %d", ranges[position][0])
			}
			ranges = append(ranges, [2]int{})
			copy(ranges[position+1:], ranges[position:])
			ranges[position] = [2]int{first, end}
		}
		x = rest
		if filter == nil && len(entries) == 0 && count > 0 {
			hint := count
			if sourceLimit := len(x) / 16; hint > sourceLimit {
				hint = sourceLimit
			}
			if hint > 0 {
				entries = make(map[Ref]xrefEntry, hint)
			}
		}
		for n := 0; n < count; n++ {
			object := first + n
			x = bytes.TrimLeft(x, " \t\r\n")
			line, rest, ok = xrefLine(x)
			if !ok {
				return nil, nil, "table", fmt.Errorf("playa: malformed xref entry")
			}
			o, g, free, ok := parseXRefEntry(line)
			if !ok {
				return nil, nil, "table", fmt.Errorf("playa: malformed xref entry")
			}
			if o < 0 || g < 0 {
				return nil, nil, "table", fmt.Errorf("playa: malformed xref entry")
			}
			ref := Ref{Object: object, Generation: g}
			entry := newXRefEntry(o, free, false, !free)
			if filter == nil || filter(ref, entry) {
				entries[ref] = entry
			}
			x = rest
		}
	}
	p := NewObjectParser(bytes.TrimSpace(x))
	o, e := p.Parse()
	if e != nil {
		return nil, nil, "table", e
	}
	trailing, e := p.NextToken()
	if e != nil {
		return nil, nil, "table", e
	}
	if trailing.Kind() == TokenKeyword && trailing.Text() == "xref" {
		next, nextErr := p.NextToken()
		if nextErr != nil || next.Kind() != TokenNumber {
			return nil, nil, "table", fmt.Errorf("playa: malformed trailing xref marker")
		}
	} else if trailing.Kind() == TokenKeyword && trailing.Text() == "startxref" {
		next, nextErr := p.NextToken()
		if nextErr != nil || next.Kind() != TokenNumber {
			return nil, nil, "table", fmt.Errorf("playa: malformed trailing startxref marker")
		}
	} else if trailing.Kind() != TokenEOF {
		return nil, nil, "table", fmt.Errorf("playa: trailing trailer data")
	}
	tr, ok := o.(Dict)
	if !ok {
		return nil, nil, "table", fmt.Errorf("playa: trailer is not dictionary")
	}
	sizeValue, _ := d.resolveIndirectChain(tr[Name("Size")])
	size, sizeOK := IntValue(sizeValue)
	if !sizeOK || size < 0 {
		return nil, nil, "table", fmt.Errorf("playa: invalid xref trailer Size")
	}
	for ref := range entries {
		if ref.Object < 0 || ref.Object >= size {
			return nil, nil, "table", fmt.Errorf("playa: xref object %d exceeds trailer Size", ref.Object)
		}
	}
	if mergeHybrid {
		if n, present, err := d.xrefStreamOffset(tr, entries); err != nil {
			return nil, nil, "table", err
		} else if present {
			extra, _, e := d.parseXRefStreamFiltered(n, filter)
			if e != nil {
				return nil, nil, "table", e
			}
			for ref, entry := range entries {
				extra[ref] = entry
			}
			entries = extra
		}
	}
	if !mergePrevious {
		return entries, tr, "table", nil
	}
	entries, tr, err = d.mergePreviousXRefSeen(entries, tr, seen, filter)
	return entries, tr, "table", err
}

// XRefs returns independent snapshots of the document's xref revisions in
// Playa order: newest revision first, with a hybrid xref stream immediately
// after its companion table and older revisions following /Prev.
func (d *Document) XRefs() ([]XRefTable, error) {
	d.cacheMu.RLock()
	if d.contentXRefRecovered {
		entries := currentRecoveredXRefEntries(d.xrefs)
		trailer := cloneDict(d.trailer)
		if trailer == nil {
			trailer = Dict{}
		}
		trailer[Name("Size")] = Number(d.recoveredTrailerSize)
		d.cacheMu.RUnlock()
		return []XRefTable{documentdata.NewXRefTable("fallback", 0, snapshotXRefEntries(entries), nil, len(entries), trailer)}, nil
	}
	d.cacheMu.RUnlock()
	offset, ok := d.latestXRefOffset()
	if !ok {
		return []XRefTable{documentdata.NewXRefTable("fallback", 0, snapshotXRefEntries(d.xrefs), nil, len(d.xrefs), cloneDict(d.trailer))}, nil
	}
	var tables []XRefTable
	seen := map[int]bool{}
	for offset > 0 && !seen[offset] {
		seen[offset] = true
		entries, trailer, kind, err := d.parseXRefLocalAt(offset)
		if err != nil {
			return nil, err
		}
		tables = append(tables, documentdata.NewXRefTable(kind, offset, snapshotXRefEntries(entries), snapshotXRefIterEntries(entries), len(entries), cloneDict(trailer)))
		if kind == "table" {
			hybrid, present, err := d.xrefStreamOffset(trailer, entries)
			if err != nil {
				return nil, err
			}
			if present && !seen[hybrid] {
				seen[hybrid] = true
				extra, extraTrailer, extraKind, err := d.parseXRefLocalAt(hybrid)
				if err != nil {
					return nil, err
				}
				tables = append(tables, documentdata.NewXRefTable(extraKind, hybrid, snapshotXRefEntries(extra), snapshotXRefIterEntries(extra), len(extra), cloneDict(extraTrailer)))
			}
		}
		previous, present, err := d.previousXRefOffset(trailer, entries)
		if err != nil {
			return nil, err
		}
		if !present {
			break
		}
		offset = previous
	}
	d.rememberVisibleXRefEntries(tables)
	return tables, nil
}

func currentRecoveredXRefEntries(entries map[Ref]xrefEntry) map[Ref]xrefEntry {
	refs := make(map[int]Ref)
	for ref, entry := range entries {
		if entry.isFree() {
			continue
		}
		current, ok := refs[ref.Object]
		if !ok || ref.Generation > current.Generation {
			refs[ref.Object] = ref
		}
	}
	current := make(map[Ref]xrefEntry, len(refs))
	for _, ref := range refs {
		current[ref] = entries[ref]
	}
	return current
}

// rememberVisibleXRefEntries keeps the scalar positions exposed by the
// revision snapshots available for exact historical resolution. The merged
// d.xrefs index intentionally retains only the newest declaration for normal
// lookup, but Playa's Mapping iterator can expose an older in-use generation
// after a newer revision marks the same object free.
func (d *Document) rememberVisibleXRefEntries(tables []XRefTable) {
	d.cacheMu.Lock()
	defer d.cacheMu.Unlock()
	added := false
	if d.xrefHistory == nil {
		d.xrefHistory = make(map[Ref]xrefEntry)
	}
	for _, table := range tables {
		for _, entry := range table.EntriesCopy() {
			ref := Ref{Object: entry.Object(), Generation: entry.Generation()}
			if _, present := d.xrefs[ref]; present {
				continue
			}
			if _, present := d.xrefHistory[ref]; present {
				continue
			}
			if entry.InObjectStream() {
				d.xrefHistory[ref] = newCompressedXRefEntry(entry.ObjectStream(), entry.ObjectIndex(), false, true)
			} else {
				d.xrefHistory[ref] = newXRefEntry(entry.Offset(), entry.Free(), false, true)
			}
			added = true
			if cachedErr, present := d.objectErrors[ref]; present {
				delete(d.objectErrors, ref)
				d.objectErrorBytes -= len(cachedErr.Error())
				if d.objectErrorBytes < 0 {
					d.objectErrorBytes = 0
				}
			}
		}
	}
	if added {
		d.resetLookupIndexLocked()
	}
}

func snapshotXRefEntries(entries map[Ref]xrefEntry) []XRefEntry {
	refs := make([]Ref, 0, len(entries))
	for ref := range entries {
		refs = append(refs, ref)
	}
	sort.Slice(refs, func(i, j int) bool {
		if refs[i].Object != refs[j].Object {
			return refs[i].Object < refs[j].Object
		}
		return refs[i].Generation < refs[j].Generation
	})
	out := make([]XRefEntry, 0, len(refs))
	for _, ref := range refs {
		entry := entries[ref]
		if entry.isFree() || entry.isPlayaHidden() {
			continue
		}
		out = append(out, documentdata.NewXRefEntry(ref.Object, ref.Generation, entry.fileOffset(), entry.isFree(), entry.objectStream(), entry.objectIndex(), entry.isCompressed()))
	}
	return out
}

func snapshotXRefIterEntries(entries map[Ref]xrefEntry) []XRefEntry {
	refs := make([]Ref, 0, len(entries))
	for ref, entry := range entries {
		if entry.isPlayaIterVisible() {
			refs = append(refs, ref)
		}
	}
	sort.Slice(refs, func(i, j int) bool {
		if refs[i].Object != refs[j].Object {
			return refs[i].Object < refs[j].Object
		}
		return refs[i].Generation < refs[j].Generation
	})
	out := make([]XRefEntry, 0, len(refs))
	for _, ref := range refs {
		entry := entries[ref]
		out = append(out, documentdata.NewXRefEntry(ref.Object, ref.Generation, entry.fileOffset(), entry.isFree(), entry.objectStream(), entry.objectIndex(), entry.isCompressed()))
	}
	return out
}

func (d *Document) latestXRefOffset() (int, bool) {
	i := bytes.LastIndex(d.data, []byte("startxref"))
	if i < 0 || (i > 0 && d.data[i-1] != '\n' && d.data[i-1] != '\r') {
		return 0, false
	}
	r := bytes.TrimSpace(d.data[i+len("startxref"):])
	lineEnd := bytes.IndexAny(r, "\r\n")
	if lineEnd >= 0 {
		r = r[:lineEnd]
	}
	offset, err := strconv.Atoi(string(bytes.TrimSpace(r)))
	return offset, err == nil && offset > 0
}

func xrefLine(data []byte) (line, rest []byte, ok bool) {
	for index, value := range data {
		switch value {
		case '\n':
			return data[:index], data[index+1:], true
		case '\r':
			next := index + 1
			if next < len(data) && data[next] == '\n' {
				next++
			}
			return data[:index], data[next:], true
		}
	}
	return nil, nil, false
}

func xrefSpace(c byte) bool {
	switch c {
	case 0, 9, 10, 12, 13, 32:
		return true
	default:
		return false
	}
}

func parseXRefNumber(line []byte, offset int) (value, next int, ok bool) {
	for offset < len(line) && xrefSpace(line[offset]) {
		offset++
	}
	if offset >= len(line) || line[offset] < '0' || line[offset] > '9' {
		return 0, offset, false
	}
	for offset < len(line) && line[offset] >= '0' && line[offset] <= '9' {
		digit := int(line[offset] - '0')
		if value > (int(^uint(0)>>1)-digit)/10 {
			return 0, offset, false
		}
		value = value*10 + digit
		offset++
	}
	return value, offset, true
}

func parseXRefSubsection(line []byte) (first, count int, ok bool) {
	var offset int
	first, offset, ok = parseXRefNumber(line, 0)
	if !ok {
		return 0, 0, false
	}
	count, offset, ok = parseXRefNumber(line, offset)
	if !ok {
		return 0, 0, false
	}
	for offset < len(line) && xrefSpace(line[offset]) {
		offset++
	}
	return first, count, offset == len(line)
}

func parseXRefEntry(line []byte) (offsetValue, generation int, free, ok bool) {
	var offset int
	offsetValue, offset, ok = parseXRefNumber(line, 0)
	if !ok {
		return 0, 0, false, false
	}
	generation, offset, ok = parseXRefNumber(line, offset)
	if !ok {
		return 0, 0, false, false
	}
	separator := offset
	for offset < len(line) && xrefSpace(line[offset]) {
		offset++
	}
	if offset == separator || offset >= len(line) || (line[offset] != 'n' && line[offset] != 'f') {
		return 0, 0, false, false
	}
	state := line[offset] == 'f'
	for offset++; offset < len(line) && xrefSpace(line[offset]); offset++ {
	}
	if offset != len(line) {
		return 0, 0, false, false
	}
	return offsetValue, generation, state, true
}

func (d *Document) xrefStreamOffset(trailer Dict, indexes ...map[Ref]xrefEntry) (int, bool, error) {
	if trailer == nil {
		return 0, false, nil
	}
	raw, present := trailer[Name("XRefStm")]
	if !present {
		return 0, false, nil
	}
	var entries map[Ref]xrefEntry
	if len(indexes) > 0 {
		entries = indexes[0]
	}
	resolved, resolvedOK := d.resolveXRefValue(raw, entries)
	if !resolvedOK {
		return 0, false, fmt.Errorf("playa: invalid xref stream offset")
	}
	offset, ok := IntValue(resolved)
	if !ok || offset <= 0 {
		return 0, false, fmt.Errorf("playa: invalid xref stream offset")
	}
	return offset, true, nil
}

func (d *Document) mergePreviousXRefSeen(entries map[Ref]xrefEntry, trailer Dict, seen map[int]bool, filter xrefEntryFilter) (map[Ref]xrefEntry, Dict, error) {
	prev, present, err := d.previousXRefOffset(trailer, entries)
	if err != nil {
		return nil, nil, err
	}
	if present {
		previousFilter := filter
		if previousFilter == nil && d.xrefIndexCoversEveryLiveObject(entries, trailer) {
			previousFilter = skipXRefEntry
		}
		older, olderTrailer, _, err := d.parseXRefAtMode(prev, seen, true, true, previousFilter)
		if err != nil {
			return nil, nil, err
		}
		// Reuse the older map, which is commonly a complete xref, and overlay
		// the sparse newer revision. Growing the sparse map to the full older
		// size retains evacuated Go map buckets for the document lifetime.
		for ref, entry := range entries {
			older[ref] = entry
		}
		entries = older
		// Incremental trailers may omit unchanged document-level entries.
		// Preserve the newest value when both revisions declare a key, while
		// inheriting Root, ID, Encrypt, and other unchanged entries from the
		// previous revision.
		for key, value := range olderTrailer {
			if _, exists := trailer[key]; !exists {
				trailer[key] = value
			}
		}
	}
	return entries, trailer, nil
}

// xrefIndexCoversEveryLiveObject reports whether older revisions cannot
// change object-number lookup. Object zero is the required free-list head;
// any other free current entry must still fall through to older revisions.
func (d *Document) xrefIndexCoversEveryLiveObject(entries map[Ref]xrefEntry, trailer Dict) bool {
	value, ok := d.resolveXRefValue(trailer[Name("Size")], entries)
	if !ok {
		return false
	}
	size, ok := IntValue(value)
	if !ok || size <= 0 || len(entries) != size {
		return false
	}
	seen := make([]byte, (size+7)/8)
	for ref, entry := range entries {
		if ref.Object < 0 || ref.Object >= size || entry.isFree() && ref.Object != 0 {
			return false
		}
		index, mask := ref.Object/8, byte(1<<uint(ref.Object%8))
		if seen[index]&mask != 0 {
			return false
		}
		seen[index] |= mask
	}
	return true
}

func (d *Document) previousXRefOffset(trailer Dict, indexes ...map[Ref]xrefEntry) (int, bool, error) {
	if trailer == nil {
		return 0, false, nil
	}
	raw, present := trailer[Name("Prev")]
	if !present {
		return 0, false, nil
	}
	var entries map[Ref]xrefEntry
	if len(indexes) > 0 {
		entries = indexes[0]
	}
	resolved, resolvedOK := d.resolveXRefValue(raw, entries)
	if !resolvedOK {
		return 0, false, fmt.Errorf("playa: invalid previous xref offset")
	}
	offset, ok := IntValue(resolved)
	if !ok || offset <= 0 {
		return 0, false, fmt.Errorf("playa: invalid previous xref offset")
	}
	return offset, true, nil
}

// resolveXRefValue resolves trailer values while an xref revision is still
// being assembled. In that phase d.xrefs is not published yet, so ordinary
// indirect resolution cannot see an indirect /Prev object declared by the
// current revision.
func (d *Document) resolveXRefValue(value Object, entries map[Ref]xrefEntry) (Object, bool) {
	seen := map[Ref]bool{}
	for {
		ref, ok := value.(Ref)
		if !ok {
			return value, true
		}
		if seen[ref] {
			return nil, false
		}
		seen[ref] = true
		d.cacheMu.RLock()
		_, objectPresent := d.objects[ref]
		xrefsPublished := d.xrefs != nil
		d.cacheMu.RUnlock()
		if objectPresent || xrefsPublished {
			if resolved, resolvedOK := d.resolveIndirectChain(ref); resolvedOK {
				value = resolved
				continue
			}
		}
		entry, found := entries[ref]
		if !found {
			if resolved, resolvedOK := d.resolveScannedXRefObject(ref); resolvedOK {
				value = resolved
				continue
			}
			return nil, false
		}
		if entry.isFree() {
			return nil, false
		}
		if entry.objectStream() > 0 {
			if err := d.loadObjectStream(entries, entry.objectStream()); err != nil {
				return nil, false
			}
			d.cacheMu.RLock()
			resolved, resolvedOK := d.objects[ref]
			d.cacheMu.RUnlock()
			if !resolvedOK {
				return nil, false
			}
			value = resolved
			continue
		}
		if entry.offset <= 0 {
			return nil, false
		}
		body, found := indirectBody(d.data, entry.offset, d.resolveRaw)
		if !found {
			return nil, false
		}
		header := bytes.Index(body, []byte("obj"))
		if header < 0 {
			return nil, false
		}
		resolved, err := parseObjectBodyBorrowedWithResolver(bytes.TrimSpace(body[header+len("obj"):]), d.resolveRaw)
		if err != nil {
			return nil, false
		}
		value = resolved
	}
}

// resolveScannedXRefObject resolves one indirect helper object while an xref
// stream is still being parsed. Such helper objects are not yet represented
// in d.xrefs, so scan only object headers until the requested reference is
// found instead of materializing the whole document.
func (d *Document) resolveScannedXRefObject(ref Ref) (Object, bool) {
	for offset := 0; offset < len(d.data); {
		match := objRE.FindIndex(d.data[offset:])
		if match == nil {
			return nil, false
		}
		start := offset + match[0]
		headerEnd := offset + match[1]
		if objectHeaderHidden(d.data, start) {
			offset = headerEnd
			continue
		}
		object, generation, ok := parseScannedObjectHeader(d.data[start:headerEnd])
		if !ok {
			offset = headerEnd
			continue
		}
		body, ok := indirectBody(d.data, start)
		if !ok {
			offset = headerEnd
			continue
		}
		offset = start + len(body)
		if object != ref.Object || generation != ref.Generation {
			continue
		}
		header := bytes.Index(body, []byte("obj"))
		if header < 0 {
			return nil, false
		}
		value, err := parseObjectBodyBorrowedWithResolver(bytes.TrimSpace(body[header+len("obj"):]), d.resolveRaw)
		return value, err == nil
	}
	return nil, false
}

// mergeXRefEntries keeps every distinct generation while preserving the
// newest declaration of an exact reference. Pinned Playa falls through a
// newer free generation to an older in-use revision for object-number lookup,
// so the merged resolver index must retain that older entry.
func mergeXRefEntries(newer, older map[Ref]xrefEntry) {
	for ref, entry := range older {
		if _, shadowed := newer[ref]; shadowed {
			continue
		}
		newer[ref] = entry
	}
}

func (d *Document) parseXRefStream(off int) (entries map[Ref]xrefEntry, trailer Dict, err error) {
	return d.parseXRefStreamFiltered(off, nil)
}

func (d *Document) parseXRefStreamFiltered(off int, filter xrefEntryFilter) (entries map[Ref]xrefEntry, trailer Dict, err error) {
	defer func() {
		if err != nil {
			err = withParseContext(err, off, "xref stream")
		}
	}()
	resolveXRef := func(value Object) Object {
		resolved, _ := d.resolveXRefValue(value, nil)
		return resolved
	}
	body, ok := indirectBody(d.data, off, resolveXRef)
	if !ok {
		return nil, nil, fmt.Errorf("playa: malformed xref stream")
	}
	header := bytes.Index(body, []byte("obj"))
	if header < 0 {
		return nil, nil, fmt.Errorf("playa: malformed xref stream header")
	}
	o, e := parseObjectBodyBorrowedWithResolver(bytes.TrimSpace(body[header+len("obj"):]), resolveXRef)
	if e != nil {
		return nil, nil, e
	}
	s, ok := o.(Stream)
	if !ok {
		return nil, nil, fmt.Errorf("playa: xref is not stream")
	}
	streamDict := s.DictBorrowed()
	typeValue := resolveXRef(streamDict[Name("Type")])
	typ, ok := typeValue.(Name)
	if !ok || typ != Name("XRef") {
		return nil, nil, fmt.Errorf("playa: xref stream has invalid Type")
	}
	filters, parms := streamFiltersWithResolver(streamDict, resolveXRef)
	raw, e := decodeFiltersLimited(s.DataBorrowed(), filters, parms, decodedFilterExpansionLimit)
	if e != nil {
		return nil, nil, e
	}
	wValue := resolveXRef(streamDict[Name("W")])
	w, ok := wValue.(Array)
	if !ok || len(w) != 3 {
		return nil, nil, fmt.Errorf("playa: xref stream missing W")
	}
	widths := make([]int, 3)
	recordWidth := 0
	for i := range widths {
		fieldValue := resolveXRef(w[i])
		value, ok := IntValue(fieldValue)
		if !ok || value < 0 || value > 8 {
			return nil, nil, fmt.Errorf("playa: invalid xref stream field width")
		}
		widths[i] = value
		recordWidth += value
	}
	sizeValue := resolveXRef(streamDict[Name("Size")])
	size, sizeOK := IntValue(sizeValue)
	if !sizeOK || size < 0 {
		return nil, nil, fmt.Errorf("playa: xref stream missing Size")
	}
	idx := []int{0, size}
	if rawIndex, present := streamDict[Name("Index")]; present {
		indexValue := resolveXRef(rawIndex)
		a, ok := indexValue.(Array)
		if !ok {
			return nil, nil, fmt.Errorf("playa: invalid xref stream Index")
		}
		if len(a)%2 != 0 {
			return nil, nil, fmt.Errorf("playa: invalid xref stream Index")
		}
		idx = make([]int, 0, len(a))
		for _, v := range a {
			indexItem := resolveXRef(v)
			x, ok := IntValue(indexItem)
			if !ok || x < 0 {
				return nil, nil, fmt.Errorf("playa: invalid xref stream Index value")
			}
			idx = append(idx, x)
		}
	}
	pos := 0
	if err := validateXRefStreamIndex(size, idx); err != nil {
		return nil, nil, err
	}
	entryCount := 0
	for j := 0; j+1 < len(idx); j += 2 {
		entryCount += idx[j+1]
	}
	if filter == nil {
		entries = make(map[Ref]xrefEntry, entryCount)
	} else {
		entries = map[Ref]xrefEntry{}
	}
	for j := 0; j+1 < len(idx); j += 2 {
		first, count := idx[j], idx[j+1]
		if count > 0 && recordWidth == 0 {
			return nil, nil, fmt.Errorf("playa: xref stream has zero-width records")
		}
		for n := 0; n < count; n++ {
			playaIterVisible := !xrefStreamRecordHidden(raw, recordWidth, widths[0], n)
			playaHidden := !playaIterVisible
			vals := []uint64{0, 0, 0}
			for k, wid := range widths {
				for q := 0; q < wid; q++ {
					if pos >= len(raw) {
						return nil, nil, fmt.Errorf("playa: truncated xref stream")
					}
					vals[k] = vals[k]<<8 | uint64(raw[pos])
					pos++
				}
			}
			if vals[1] > uint64(math.MaxInt) || vals[2] > uint64(math.MaxInt) {
				return nil, nil, fmt.Errorf("playa: xref stream field exceeds platform integer range")
			}
			typ := vals[0]
			if widths[0] == 0 {
				typ = 1
			}
			switch typ {
			case 0:
				// Free entries must remain in the map so an incremental xref
				// revision shadows older in-use generations for this object.
				ref := Ref{Object: first + n, Generation: int(vals[2])}
				entry := newXRefEntry(0, true, playaHidden, playaIterVisible)
				if filter == nil || filter(ref, entry) {
					entries[ref] = entry
				}
			case 1:
				ref := Ref{Object: first + n, Generation: int(vals[2])}
				entry := newXRefEntry(int(vals[1]), false, playaHidden, playaIterVisible)
				if filter == nil || filter(ref, entry) {
					entries[ref] = entry
				}
			case 2:
				if vals[1] == 0 {
					return nil, nil, fmt.Errorf("playa: invalid xref object stream number %d", vals[1])
				}
				ref := Ref{Object: first + n, Generation: 0}
				entry := newCompressedXRefEntry(int(vals[1]), int(vals[2]), playaHidden, playaIterVisible)
				if filter == nil || filter(ref, entry) {
					entries[ref] = entry
				}
			default:
				return nil, nil, fmt.Errorf("playa: unsupported xref stream entry type %d", typ)
			}
		}
	}
	if pos != len(raw) {
		return nil, nil, fmt.Errorf("playa: xref stream has %d trailing bytes", len(raw)-pos)
	}
	return entries, streamDict, nil
}

// xrefStreamRecordHidden reproduces Playa 1.1.0's XRefStream.__iter__ byte
// offset behavior. Playa resets the record index for each /Index range, while
// __getitem__ and object resolution use the correct cumulative offset.
func xrefStreamRecordHidden(raw []byte, recordWidth, firstWidth, localIndex int) bool {
	if firstWidth == 0 {
		return false
	}
	if recordWidth <= 0 || localIndex < 0 || localIndex > len(raw)/recordWidth {
		return true
	}
	start := localIndex * recordWidth
	if start+firstWidth > len(raw) {
		return true
	}
	var typ uint64
	for _, value := range raw[start : start+firstWidth] {
		typ = typ<<8 | uint64(value)
	}
	return typ != 1 && typ != 2
}

func validateXRefStreamIndex(size int, idx []int) error {
	if size < 0 || len(idx)%2 != 0 {
		return fmt.Errorf("playa: invalid xref stream Index")
	}
	previousEnd := 0
	for j := 0; j+1 < len(idx); j += 2 {
		first, count := idx[j], idx[j+1]
		if first < 0 || count < 0 {
			return fmt.Errorf("playa: invalid xref stream range")
		}
		if j > 0 && first < previousEnd {
			return fmt.Errorf("playa: xref stream Index ranges are not sorted")
		}
		if count > 0 && first > int(^uint(0)>>1)-count {
			return fmt.Errorf("playa: xref stream object number overflow")
		}
		if first > size || count > size-first {
			return fmt.Errorf("playa: xref stream Index exceeds Size")
		}
		previousEnd = first + count
	}
	return nil
}
