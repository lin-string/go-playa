package document

import "iter"

// Len returns the Mapping length exposed by Playa. For opened PDFs this is the
// trailer /Size value, which is deliberately not the number of in-use mapping
// keys: incremental xref revisions can make Keys contain duplicate numbers.
func (d *Document) Len() int {
	d.cacheMu.RLock()
	if d.contentXRefRecovered {
		size := d.recoveredTrailerSize
		d.cacheMu.RUnlock()
		return size
	}
	d.cacheMu.RUnlock()
	if raw, present := d.trailer[Name("Size")]; present {
		if value, ok := d.resolveIndirectChain(raw); ok {
			if size, ok := IntValue(value); ok && size >= 0 {
				return size
			}
		}
	}
	if tables, err := d.XRefs(); err == nil {
		size := 0
		for _, table := range tables {
			size += table.EntryCount()
		}
		if size > 0 {
			return size
		}
	}
	return d.ObjectCount()
}

// Get returns the newest generation of an indirect object number as an
// independent snapshot. It is the lenient counterpart of GetWithError.
func (d *Document) Get(objectNumber int) (Object, bool) {
	value, err := d.GetWithError(objectNumber)
	return value, err == nil
}

// GetWithError returns the newest generation of an indirect object number as
// an independent snapshot. Missing numbers return ErrObjectNotFound.
func (d *Document) GetWithError(objectNumber int) (Object, error) {
	_, value, err := d.LookupWithError(objectNumber)
	return value, err
}

// Keys yields Mapping keys in Playa's XRef revision order. Incremental PDFs
// can therefore yield the same object number more than once. The sequence is
// repeatable and does not resolve object values.
func (d *Document) Keys() iter.Seq2[int, error] {
	return func(yield func(int, error) bool) {
		refs, err := d.mappingReferences()
		if err != nil {
			yield(0, err)
			return
		}
		for _, ref := range refs {
			if !yield(ref.Object, nil) {
				return
			}
		}
	}
}

// Values yields source mapping values in the same order as Keys. Values are
// borrowed and are resolved only as they are requested by the consumer.
func (d *Document) Values() iter.Seq2[Object, error] {
	return func(yield func(Object, error) bool) {
		refs, err := d.mappingReferences()
		if err != nil {
			yield(nil, err)
			return
		}
		for _, ref := range refs {
			value, err := d.resolveRefRaw(ref)
			if err != nil {
				yield(nil, err)
				return
			}
			if !yield(value, nil) {
				return
			}
		}
	}
}

// Items yields current indirect objects in the same order as Keys. Duplicate
// object numbers from incremental XRef revisions are retained. IndirectObject
// values are borrowed; ValueCopy or Finalize can be used when an item must
// outlive the iteration.
func (d *Document) Items() iter.Seq2[IndirectObject, error] {
	return func(yield func(IndirectObject, error) bool) {
		refs, err := d.mappingReferences()
		if err != nil {
			yield(IndirectObject{}, err)
			return
		}
		for _, ref := range refs {
			value, err := d.resolveRefRaw(ref)
			if err != nil {
				yield(IndirectObject{}, err)
				return
			}
			if !yield(newIndirectObject(ref, value), nil) {
				return
			}
		}
	}
}

// mappingReferences follows Playa's Mapping iteration contract. Opened PDFs
// retain every in-use object number from every XRef revision in newest-first
// revision order, while each key resolves to the current object value. Map-only
// synthetic Documents retain the deterministic newest-generation fallback.
func (d *Document) mappingReferences() ([]Ref, error) {
	tables, err := d.XRefs()
	if err == nil {
		current := currentMappingRefsFromTables(tables)
		refs := make([]Ref, 0)
		for _, table := range tables {
			entries := table.VisibleEntriesCopy()
			if entries == nil {
				entries = table.EntriesCopy()
			}
			for _, entry := range entries {
				if ref, ok := current[entry.Object()]; ok {
					refs = append(refs, ref)
				}
			}
		}
		if len(refs) != 0 {
			return refs, nil
		}
	}
	refs := d.objectReferences()
	if len(refs) == 0 {
		return nil, nil
	}
	mapping := make([]Ref, 0, len(refs))
	for _, ref := range refs {
		if len(mapping) > 0 && mapping[len(mapping)-1].Object == ref.Object {
			mapping[len(mapping)-1] = ref
			continue
		}
		mapping = append(mapping, ref)
	}
	return mapping, nil
}

func currentMappingRefsFromTables(tables []XRefTable) map[int]Ref {
	refs := make(map[int]Ref)
	for _, table := range tables {
		for entry := range table.EntriesSeq() {
			if _, found := refs[entry.Object()]; found {
				continue
			}
			refs[entry.Object()] = Ref{Object: entry.Object(), Generation: entry.Generation()}
		}
	}
	return refs
}

func (d *Document) currentMappingRefs() map[int]Ref {
	d.cacheMu.RLock()
	defer d.cacheMu.RUnlock()
	refs := make(map[int]Ref, len(d.xrefs)+len(d.objects))
	for ref, entry := range d.xrefs {
		// playaHidden only controls whether a revision's XRef iterator
		// exposes the entry. The entry is still valid for current-value lookup;
		// a later hidden generation must not erase an older revision's mapping
		// key when Mapping walks the revision sequence.
		if entry.isFree() {
			continue
		}
		current, found := refs[ref.Object]
		if !found || ref.Generation > current.Generation {
			refs[ref.Object] = ref
		}
	}
	for ref := range d.objects {
		current, found := refs[ref.Object]
		if !found || ref.Generation > current.Generation {
			refs[ref.Object] = ref
		}
	}
	return refs
}
