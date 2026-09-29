package document

import (
	"encoding/json"
	"fmt"
	"iter"
	"strconv"
	"strings"
	"sync"

	"github.com/lin-string/go-playa/documentdata"
)

const formTopCacheLimit = 32 << 20

// FormField is one node in the AcroForm field tree. A field can be a logical
// parent, a terminal field, a widget annotation, or both a terminal field and
// a widget. Inherited PDF field attributes are materialized on every node.
type FormField struct {
	data          documentdata.FormField
	kids          []FormField
	document      *Document
	kidObjects    []Object
	kidParent     Dict
	kidParentName string
	fieldRef      Ref
	hasFieldRef   bool
	formPath      map[Ref]bool
	kidsReady     bool
	lazyState     *formFieldLazyState
}

// formFieldLazyState is shared by value copies of a borrowed field.
type formFieldLazyState struct {
	mu    sync.Mutex
	kids  []FormField
	ready bool
	err   error
}

func cloneFormField(field FormField) FormField {
	field.data = field.data.Finalize()
	field.lazyState = nil
	if field.kidParent != nil {
		field.kidParent = cloneGraphicsObject(field.kidParent).(Dict)
	}
	if field.kids != nil {
		kids := field.kids
		field.kids = make([]FormField, len(kids))
		for i, child := range kids {
			field.kids[i] = cloneFormField(child)
		}
	}
	if field.kidObjects != nil {
		objects := field.kidObjects
		field.kidObjects = make([]Object, len(objects))
		for i, object := range objects {
			field.kidObjects[i] = cloneGraphicsObject(object)
		}
	}
	field.formPath = cloneFormPath(field.formPath)
	return field
}

func formFieldCacheSize(field FormField, depth int) int {
	if depth <= 0 {
		return 512
	}
	size := 512
	for _, addition := range []int{len(field.Name()), len(field.FullName()), len(field.FieldType()), len(field.Value()), len(field.DefaultValue()), len(field.DefaultAppearance())} {
		size = addCacheSize(size, addition)
	}
	for _, value := range field.ValuesCopy() {
		size = addCacheSize(size, addCacheSize(len(value), 16))
	}
	for _, value := range field.DefaultValuesCopy() {
		size = addCacheSize(size, addCacheSize(len(value), 16))
	}
	for _, value := range field.OptionsCopy() {
		size = addCacheSize(size, addCacheSize(len(value), 16))
	}
	for _, value := range field.OptionValuesCopy() {
		size = addCacheSize(size, addCacheSize(len(value), 16))
	}
	size = addCacheSize(size, cacheMulSize(len(field.SelectedCopy()), 8))
	size = addCacheSize(size, nameTreeObjectSize(field.DictCopy(), depth-1))
	size = addCacheSize(size, nameTreeObjectSize(field.kidParent, depth-1))
	size = addCacheSize(size, cacheMulSize(len(field.kidObjects), 16))
	for _, child := range field.kids {
		size = addCacheSize(size, formFieldCacheSize(child, depth-1))
	}
	return size
}

func (d *Document) cachedFormField(ref Ref) (FormField, bool) {
	d.cacheMu.RLock()
	field, ok := d.formTopCache[ref]
	d.cacheMu.RUnlock()
	return field, ok
}

func (d *Document) cacheFormField(ref Ref, field FormField) FormField {
	size := formFieldCacheSize(field, 8)
	d.cacheMu.Lock()
	defer d.cacheMu.Unlock()
	if cached, exists := d.formTopCache[ref]; exists {
		return cached
	}
	limit := d.cacheLimits().FormFieldBytes
	if !cacheFits(d.formTopCacheBytes, size, limit) {
		return field
	}
	if d.formTopCache == nil {
		d.formTopCache = map[Ref]FormField{}
	}
	if d.formTopCacheSizes == nil {
		d.formTopCacheSizes = map[Ref]int{}
	}
	d.formTopCache[ref] = field
	d.formTopCacheSizes[ref] = size
	d.formTopCacheBytes += size
	return field
}

func (d *Document) updateFormFieldCache(ref Ref, field FormField) {
	d.cacheMu.Lock()
	defer d.cacheMu.Unlock()
	oldSize, exists := d.formTopCacheSizes[ref]
	if !exists {
		return
	}
	newSize := formFieldCacheSize(field, 8)
	limit := d.cacheLimits().FormFieldBytes
	if oldSize > d.formTopCacheBytes || !cacheFits(d.formTopCacheBytes-oldSize, newSize, limit) {
		delete(d.formTopCache, ref)
		delete(d.formTopCacheSizes, ref)
		if oldSize >= d.formTopCacheBytes {
			d.formTopCacheBytes = 0
		} else {
			d.formTopCacheBytes -= oldSize
		}
		return
	}
	d.formTopCache[ref] = field
	d.formTopCacheSizes[ref] = newSize
	d.formTopCacheBytes += newSize - oldSize
}

func (f FormField) ValuesCopy() []string        { return f.data.ValuesCopy() }
func (f FormField) DefaultValuesCopy() []string { return f.data.DefaultValuesCopy() }
func (f FormField) OptionsCopy() []string       { return f.data.OptionsCopy() }
func (f FormField) OptionValuesCopy() []string  { return f.data.OptionValuesCopy() }
func (f FormField) SelectedCopy() []int         { return f.data.SelectedCopy() }

// Name returns the field's local name.
func (f FormField) Name() string { return f.data.Name() }

// FullName returns the field's fully qualified name.
func (f FormField) FullName() string { return f.data.FullName() }

// Page returns the page reference from the field's /P entry.
func (f FormField) Page() Ref {
	page, _ := f.data.Page()
	return page
}

// HasPage reports whether Page is present.
func (f FormField) HasPage() bool { return f.data.HasPage() }

// Parent returns the field's /Parent reference.
func (f FormField) Parent() Ref {
	parent, _ := f.data.Parent()
	return parent
}

// HasParent reports whether Parent is present.
func (f FormField) HasParent() bool { return f.data.HasParent() }

// FieldType returns the inherited PDF field type, such as "Tx" or "Ch".
func (f FormField) FieldType() string { return f.data.FieldType() }

// Flags returns the inherited field flags.
func (f FormField) Flags() int { return f.data.Flags() }

// HasFlags reports whether Flags is present.
func (f FormField) HasFlags() bool { return f.data.HasFlags() }

// Value returns the scalar field value. Multi-valued fields expose ValuesCopy.
func (f FormField) Value() string { return f.data.Value() }

// DefaultValue returns the scalar default value.
func (f FormField) DefaultValue() string { return f.data.DefaultValue() }

// DefaultAppearance returns the inherited default appearance string.
func (f FormField) DefaultAppearance() string { return f.data.DefaultAppearance() }

// Rect returns the normalized widget rectangle.
func (f FormField) Rect() [4]float64 { return f.data.Rect() }

// HasRect reports whether Rect is present.
func (f FormField) HasRect() bool { return f.data.HasRect() }

// IsWidget reports whether this field is a widget annotation.
func (f FormField) IsWidget() bool { return f.data.IsWidget() }

// DictCopy returns an independent copy of the source form-field dictionary.
func (f FormField) DictCopy() Dict { return f.data.DictCopy() }

// KidsCopy materializes lazy descendants and returns an independent tree.
func (f FormField) KidsCopy() ([]FormField, error) {
	if err := f.materializeKidsErr(); err != nil {
		return nil, err
	}
	if f.kids == nil {
		return nil, nil
	}
	clone := make([]FormField, len(f.kids))
	for i, child := range f.kids {
		clone[i] = finalizeFormField(child)
	}
	return clone, nil
}

func (f FormField) MarshalJSON() ([]byte, error) {
	if err := f.materializeKidsErr(); err != nil {
		return nil, err
	}
	page, _ := f.data.Page()
	parent, _ := f.data.Parent()
	return json.Marshal(&struct {
		Name              string      `json:"Name"`
		FullName          string      `json:"FullName"`
		Page              Ref         `json:"Page"`
		HasPage           bool        `json:"HasPage"`
		Parent            Ref         `json:"Parent"`
		HasParent         bool        `json:"HasParent"`
		FieldType         string      `json:"FieldType"`
		Flags             int         `json:"Flags"`
		HasFlags          bool        `json:"HasFlags"`
		Value             string      `json:"Value"`
		DefaultValue      string      `json:"DefaultValue"`
		DefaultAppearance string      `json:"DefaultAppearance"`
		Rect              [4]float64  `json:"Rect"`
		HasRect           bool        `json:"HasRect"`
		IsWidget          bool        `json:"IsWidget"`
		Dict              Dict        `json:"Dict"`
		Values            []string    `json:"Values"`
		DefaultValues     []string    `json:"DefaultValues"`
		Options           []string    `json:"Options"`
		OptionValues      []string    `json:"OptionValues"`
		Selected          []int       `json:"Selected"`
		Kids              []FormField `json:"Kids"`
	}{
		Name: f.Name(), FullName: f.FullName(), Page: page, HasPage: f.HasPage(),
		Parent: parent, HasParent: f.HasParent(), FieldType: f.FieldType(),
		Flags: f.Flags(), HasFlags: f.HasFlags(), Value: f.Value(),
		DefaultValue: f.DefaultValue(), DefaultAppearance: f.DefaultAppearance(),
		Rect: f.Rect(), HasRect: f.HasRect(), IsWidget: f.IsWidget(),
		Dict: f.DictCopy(), Values: f.ValuesCopy(), DefaultValues: f.DefaultValuesCopy(),
		Options: f.OptionsCopy(), OptionValues: f.OptionValuesCopy(), Selected: f.SelectedCopy(), Kids: f.kids,
	})
}

// PageObject resolves the page associated with a widget field's /P entry.
func (f FormField) PageObject(d *Document) (Page, error) {
	if d == nil {
		return Page{}, errNilDocument
	}
	if !f.HasPage() {
		return Page{}, ErrPageNotFound
	}
	return d.PageByRef(f.Page())
}

// NeedAppearances reports whether the document asks a consumer to synthesize
// appearances for form fields.
func (d *Document) NeedAppearances() bool {
	catalogValue, _ := d.resolveIndirectChain(d.trailer[Name("Root")])
	catalog, _ := catalogValue.(Dict)
	acroValue, _ := d.resolveIndirectChain(catalog[Name("AcroForm")])
	acro, _ := acroValue.(Dict)
	value, _ := d.resolveIndirectChain(acro[Name("NeedAppearances")])
	b, _ := value.(Bool)
	return bool(b)
}

// FormFields returns the top-level AcroForm fields in document order.
//
// Malformed field entries are reported through the sequence error channel. A
// reference cycle is still skipped after the cycle guard identifies it. The
// sequence can be traversed more than once and stops as soon as the consumer
// stops yielding.
func (d *Document) FormFields() iter.Seq2[FormField, error] {
	return func(yield func(FormField, error) bool) {
		d.cacheMu.RLock()
		if d.formFieldsErrReady {
			err := d.formFieldsErr
			d.cacheMu.RUnlock()
			yield(FormField{}, err)
			return
		}
		d.cacheMu.RUnlock()
		cacheError := func(err error) {
			d.cacheMu.Lock()
			if d.formFieldsErrReady {
				err = d.formFieldsErr
			} else {
				d.formFieldsErr = err
				d.formFieldsErrReady = true
			}
			d.cacheMu.Unlock()
			yield(FormField{}, err)
		}
		rawCatalog, present := d.trailer[Name("Root")]
		if !present {
			return
		}
		catalogValue, ok := d.resolveIndirectChain(rawCatalog)
		if !ok {
			cacheError(fmt.Errorf("playa: catalog root is not a dictionary"))
			return
		}
		catalog, ok := catalogValue.(Dict)
		if !ok {
			cacheError(fmt.Errorf("playa: catalog root is not a dictionary"))
			return
		}
		rawAcro, present := catalog[Name("AcroForm")]
		if !present {
			return
		}
		acroValue, ok := d.resolveIndirectChain(rawAcro)
		if !ok {
			cacheError(fmt.Errorf("playa: AcroForm is not a dictionary"))
			return
		}
		acro, ok := acroValue.(Dict)
		if !ok {
			cacheError(fmt.Errorf("playa: AcroForm is not a dictionary"))
			return
		}
		rawFields, present := acro[Name("Fields")]
		if !present {
			return
		}
		fieldsValue, fieldsResolved := d.resolveIndirectChain(rawFields)
		if !fieldsResolved {
			cacheError(fmt.Errorf("playa: AcroForm Fields could not be resolved"))
			return
		}
		fields, ok := fieldsValue.(Array)
		if !ok {
			cacheError(fmt.Errorf("playa: AcroForm Fields is not an array"))
			return
		}
		if len(fields) == 0 {
			return
		}
		seen := map[Ref]bool{}
		for _, item := range fields {
			cacheRef, hasCacheRef := d.finalIndirectRef(item)
			if !hasCacheRef {
				cacheRef, hasCacheRef = item.(Ref)
			}
			cached := false
			if hasCacheRef {
				d.cacheMu.RLock()
				_, cached = d.formTopCache[cacheRef]
				d.cacheMu.RUnlock()
			}
			if !cached {
				resolved, resolvedOK := d.resolveIndirectChain(item)
				if !resolvedOK {
					yield(FormField{}, fmt.Errorf("playa: form field is not a dictionary"))
					return
				}
				if _, valid := resolved.(Dict); !valid {
					yield(FormField{}, fmt.Errorf("playa: form field is not a dictionary"))
					return
				}
			}
			var field FormField
			var ok bool
			if hasCacheRef {
				field, ok = d.cachedFormField(cacheRef)
				if !ok {
					field, ok = d.formField(item, nil, "", seen)
					if ok {
						field = d.cacheFormField(cacheRef, field)
					}
				}
			} else {
				field, ok = d.formField(item, nil, "", seen)
			}
			if !ok {
				continue
			}
			if !yield(field, nil) {
				return
			}
		}
	}
}

// CollectFormFields materializes FormFields for adapters that need a complete
// list. Returned fields retain their document association; call Finalize on a
// field when an independent snapshot is required.
func (d *Document) CollectFormFields() ([]FormField, error) {
	var out []FormField
	for field, err := range d.FormFields() {
		if err != nil {
			return nil, err
		}
		if err := field.materializeKidsErr(); err != nil {
			return nil, err
		}
		out = append(out, field)
	}
	return out, nil
}

func (f *FormField) materializeKids() {
	_ = f.materializeKidsErr()
}

func (f *FormField) materializeKidsErr() error {
	return f.materializeKidsErrSeen(map[Ref]bool{})
}

func (f *FormField) materializeKidsErrSeen(seen map[Ref]bool) error {
	if f == nil {
		return nil
	}
	if f.lazyState != nil {
		state := f.lazyState
		state.mu.Lock()
		defer state.mu.Unlock()
		if state.ready {
			f.kids = state.kids
			f.kidsReady = true
			return state.err
		}
		if f.hasFieldRef {
			if seen[f.fieldRef] {
				state.ready = true
				f.kidsReady = true
				return nil
			}
			seen[f.fieldRef] = true
			defer delete(seen, f.fieldRef)
		}
		if f.document == nil || len(f.kidObjects) == 0 {
			state.kids = f.kids
			state.ready = true
			f.kidsReady = true
			return nil
		}
		kids := make([]FormField, 0, len(f.kidObjects))
		for _, item := range f.kidObjects {
			child, ok := f.document.formField(item, f.kidParent, f.kidParentName, seen)
			if !ok {
				resolved, _ := f.document.resolveIndirectChain(item)
				if _, valid := resolved.(Dict); !valid {
					err := fmt.Errorf("playa: form field child is not a dictionary")
					state.ready, state.err = true, err
					f.kidsReady = true
					return err
				}
				continue
			}
			if err := child.materializeKidsErrSeen(seen); err != nil {
				state.ready, state.err = true, err
				f.kidsReady = true
				return err
			}
			kids = append(kids, child)
		}
		state.kids = kids
		state.ready = true
		f.kids = kids
		f.kidsReady = true
		if f.hasFieldRef && f.document != nil {
			f.document.updateFormFieldCache(f.fieldRef, *f)
		}
		return nil
	}
	if f.kidsReady {
		return nil
	}
	if f.hasFieldRef {
		if seen[f.fieldRef] {
			f.kidsReady = true
			return nil
		}
		seen[f.fieldRef] = true
		defer delete(seen, f.fieldRef)
	}
	if f.document == nil || len(f.kidObjects) == 0 {
		f.kidsReady = true
		return nil
	}
	f.kids = nil
	for _, item := range f.kidObjects {
		child, ok := f.document.formField(item, f.kidParent, f.kidParentName, seen)
		if !ok {
			resolved, _ := f.document.resolveIndirectChain(item)
			if _, valid := resolved.(Dict); !valid {
				return fmt.Errorf("playa: form field child is not a dictionary")
			}
			continue
		}
		if err := child.materializeKidsErrSeen(seen); err != nil {
			return err
		}
		f.kids = append(f.kids, child)
	}
	f.kidsReady = true
	if f.hasFieldRef && f.document != nil {
		f.document.updateFormFieldCache(f.fieldRef, *f)
	}
	return nil
}

func finalizeFormField(field FormField) FormField {
	field.materializeKids()
	clone := cloneFormField(field)
	clone.document = nil
	clone.kidObjects = nil
	clone.kidParent = nil
	clone.fieldRef = Ref{}
	clone.hasFieldRef = false
	clone.formPath = nil
	clone.kidsReady = true
	for i := range clone.kids {
		clone.kids[i] = finalizeFormField(clone.kids[i])
	}
	return clone
}

func finalizeFormFieldWithError(field FormField) (FormField, error) {
	if err := field.materializeKidsErr(); err != nil {
		return FormField{}, err
	}
	clone := cloneFormField(field)
	clone.document = nil
	clone.kidObjects = nil
	clone.kidParent = nil
	clone.fieldRef = Ref{}
	clone.hasFieldRef = false
	clone.formPath = nil
	clone.kidsReady = true
	for i, child := range field.kids {
		value, err := finalizeFormFieldWithError(child)
		if err != nil {
			return FormField{}, err
		}
		clone.kids[i] = value
	}
	return clone, nil
}

// Finalize materializes lazy children and returns an independent form snapshot.
func (f FormField) FinalizeWithError() (FormField, error) {
	return finalizeFormFieldWithError(f)
}

func (f FormField) Finalize() FormField {
	clone, _ := f.FinalizeWithError()
	return clone
}

func (d *Document) formField(item Object, parent Dict, parentName string, seen map[Ref]bool) (FormField, bool) {
	if ref, ok := d.finalIndirectRef(item); ok {
		if seen[ref] {
			return FormField{}, false
		}
		seen[ref] = true
		defer delete(seen, ref)
	}
	resolvedItem, _ := d.resolveIndirectChain(item)
	v, ok := resolvedItem.(Dict)
	if !ok {
		return FormField{}, false
	}
	merged := mergeDict(parent, v)
	name := ""
	nameValue, _ := d.resolveIndirectChain(v[Name("T")])
	if raw, ok := nameValue.(String); ok {
		name = decodePDFText(raw)
	} else if raw, ok := nameValue.(Name); ok {
		name = string(raw)
	}
	full := name
	if parentName != "" {
		if full != "" {
			full = parentName + "." + full
		} else {
			full = parentName
		}
	}
	spec := documentdata.FormFieldSpec{Name: name, FullName: full, Dict: v}
	field := FormField{formPath: cloneFormPath(seen)}
	if ref, ok := d.finalIndirectRef(item); ok {
		field.fieldRef, field.hasFieldRef = ref, true
	}
	if page, ok := d.finalIndirectRef(merged[Name("P")]); ok {
		spec.Page, spec.HasPage = page, true
	}
	if parentRef, ok := d.finalIndirectRef(v[Name("Parent")]); ok {
		spec.Parent, spec.HasParent = parentRef, true
	}
	fieldTypeValue, _ := d.resolveIndirectChain(merged[Name("FT")])
	if ft, ok := fieldTypeValue.(Name); ok {
		spec.FieldType = string(ft)
	}
	flagsValue, _ := d.resolveIndirectChain(merged[Name("Ff")])
	if flags, ok := IntValue(flagsValue); ok {
		spec.Flags, spec.HasFlags = flags, true
	}
	valueObject, _ := d.resolveIndirectChain(merged[Name("V")])
	if values, ok := d.formTexts(valueObject); ok {
		if _, isArray := valueObject.(Array); isArray {
			spec.Values = values
		} else if len(values) == 1 {
			spec.Value = values[0]
		} else {
			spec.Values = values
		}
	}
	defaultValueObject, _ := d.resolveIndirectChain(merged[Name("DV")])
	if values, ok := d.formTexts(defaultValueObject); ok {
		if _, isArray := defaultValueObject.(Array); isArray {
			spec.DefaultValues = values
		} else if len(values) == 1 {
			spec.DefaultValue = values[0]
		} else {
			spec.DefaultValues = values
		}
	}
	if da, ok := d.formText(merged[Name("DA")]); ok {
		spec.DefaultAppearance = da
	}
	optionsValue, _ := d.resolveIndirectChain(merged[Name("Opt")])
	if options, ok := optionsValue.(Array); ok {
		for _, option := range options {
			pairValue, _ := d.resolveIndirectChain(option)
			if pair, ok := pairValue.(Array); ok && len(pair) == 2 {
				export, exportOK := d.formText(pair[0])
				display, displayOK := d.formText(pair[1])
				if displayOK {
					spec.Options = append(spec.Options, display)
					if !exportOK {
						export = display
					}
					spec.OptionValues = append(spec.OptionValues, export)
				}
				continue
			}
			if value, ok := d.formText(option); ok {
				spec.Options = append(spec.Options, value)
				spec.OptionValues = append(spec.OptionValues, value)
			}
		}
	}
	selectedValue, _ := d.resolveIndirectChain(merged[Name("I")])
	if selected, ok := selectedValue.(Array); ok {
		for _, value := range selected {
			resolved, _ := d.resolveIndirectChain(value)
			if index, ok := IntValue(resolved); ok {
				if index >= 0 && index < len(spec.Options) {
					spec.Selected = append(spec.Selected, index)
				}
			}
		}
	} else if index, ok := IntValue(selectedValue); ok {
		if index >= 0 && index < len(spec.Options) {
			spec.Selected = []int{index}
		}
	}
	subtypeValue, _ := d.resolveIndirectChain(v[Name("Subtype")])
	if subtype, ok := subtypeValue.(Name); ok && subtype == Name("Widget") {
		spec.IsWidget = true
		if rect, ok := d.formRect(v[Name("Rect")]); ok {
			spec.Rect, spec.HasRect = rect, true
		}
	}
	kidsValue, _ := d.resolveIndirectChain(v[Name("Kids")])
	if kids, ok := kidsValue.(Array); ok {
		field.document = d
		field.kidObjects = kids
		field.kidParent = merged
		field.kidParentName = full
		field.lazyState = &formFieldLazyState{}
	}
	field.data = documentdata.NewFormField(spec)
	return field, true
}

func cloneFormPath(source map[Ref]bool) map[Ref]bool {
	if source == nil {
		return nil
	}
	path := make(map[Ref]bool, len(source))
	for ref, present := range source {
		path[ref] = present
	}
	return path
}

func (d *Document) formText(o Object) (string, bool) {
	o, _ = d.resolveIndirectChain(o)
	switch v := o.(type) {
	case String:
		return decodePDFText(v), true
	case Name:
		return string(v), true
	case Number:
		return numberString(float64(v)), true
	default:
		return "", false
	}
}

func (d *Document) formTexts(o Object) ([]string, bool) {
	o, _ = d.resolveIndirectChain(o)
	if values, ok := o.(Array); ok {
		out := make([]string, 0, len(values))
		for _, value := range values {
			text, ok := d.formText(value)
			if !ok {
				return nil, false
			}
			out = append(out, text)
		}
		return out, true
	}
	value, ok := d.formText(o)
	if !ok {
		return nil, false
	}
	return []string{value}, true
}

func numberString(v float64) string {
	return strings.TrimRight(strings.TrimRight(formatFloat(v), "0"), ".")
}

func formatFloat(v float64) string {
	// Keep form values compact while preserving ordinary decimal values.
	return strconv.FormatFloat(v, 'f', -1, 64)
}

func (d *Document) formRect(o Object) ([4]float64, bool) {
	var rect [4]float64
	resolved, _ := d.resolveIndirectChain(o)
	a, ok := resolved.(Array)
	if !ok || len(a) != 4 {
		return rect, false
	}
	for i := 0; i < 4; i++ {
		var valid bool
		item, _ := d.resolveIndirectChain(a[i])
		rect[i], valid = finiteNumberValue(item)
		if !valid {
			return [4]float64{}, false
		}
	}
	if rect[0] > rect[2] {
		rect[0], rect[2] = rect[2], rect[0]
	}
	if rect[1] > rect[3] {
		rect[1], rect[3] = rect[3], rect[1]
	}
	return rect, true
}
