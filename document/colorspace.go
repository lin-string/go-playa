package document

import (
	"encoding/json"
	"math"

	"github.com/lin-string/go-playa/imagedata"
)

// ImageColorSpace is the structured form of an image's /ColorSpace entry.
// Name remains available on ImageObject for compatibility with existing
// callers, while this value preserves bases, colorants, ICC component count,
// and Indexed metadata.
type ImageColorSpace struct {
	data              imagedata.ColorSpace
	lookup            []byte
	lookupData        []byte
	lookupFilters     []string
	lookupFilterParms []Dict
	lookupCache       *imageDecodeCache
}

func newImageColorSpace(name string, components int) ImageColorSpace {
	return ImageColorSpace{data: imagedata.NewColorSpace(name, components)}
}

// Name returns the normalized PDF color-space name.
func (c ImageColorSpace) Name() string { return c.data.Name() }

// Components returns the number of color components.
func (c ImageColorSpace) Components() int { return c.data.Components() }

// High returns the Indexed color-space high value.
func (c ImageColorSpace) High() int { return c.data.High() }

// ProfileN returns the ICC profile component count when available.
func (c ImageColorSpace) ProfileN() int { return c.data.ProfileN() }

// SpecCopy returns the raw PDF color-space specification when available.
func (c ImageColorSpace) SpecCopy() Object { return c.data.SpecCopy() }

func cloneImageStrings(value []string) []string {
	if value == nil {
		return nil
	}
	return append(make([]string, 0, len(value)), value...)
}

// BaseCopy returns an independent base color-space description.
func (c ImageColorSpace) BaseCopy() (ImageColorSpace, bool) {
	base, ok := c.data.BaseCopy()
	if !ok {
		return ImageColorSpace{}, false
	}
	return ImageColorSpace{data: base}, true
}

// ColorantsCopy returns an independent copy of Separation or DeviceN names.
func (c ImageColorSpace) ColorantsCopy() []string { return c.data.ColorantsCopy() }

// LookupCopy returns an independent copy of Indexed color lookup bytes.
func (c ImageColorSpace) LookupCopy() []byte { return cloneObjectBytes(c.lookupBytes()) }

// LookupWithError returns an independent copy of Indexed color lookup bytes
// and preserves lazy filter-decoding errors.
func (c ImageColorSpace) LookupWithError() ([]byte, error) {
	data, err := c.lookupDecoded()
	return cloneObjectBytes(data), err
}

// Finalize returns an independent snapshot of the image color-space model.
func (c ImageColorSpace) Finalize() ImageColorSpace { return cloneImageColorSpace(c) }

// FinalizeWithError returns an independent color-space snapshot and reports
// deferred lookup-filter or nested base color-space failures.
func (c ImageColorSpace) FinalizeWithError() (ImageColorSpace, error) {
	if _, err := c.lookupDecoded(); err != nil {
		return ImageColorSpace{}, err
	}
	clone := cloneImageColorSpace(c)
	return clone, nil
}

func (c ImageColorSpace) MarshalJSON() ([]byte, error) {
	lookup := c.lookupBytes()
	type projection struct {
		Name       string           `json:"Name"`
		Components int              `json:"Components"`
		Spec       Object           `json:"Spec"`
		Base       *ImageColorSpace `json:"Base"`
		Colorants  []string         `json:"Colorants"`
		High       int              `json:"High"`
		Lookup     []byte           `json:"Lookup"`
		ProfileN   int              `json:"ProfileN"`
	}
	base, hasBase := c.BaseCopy()
	var basePtr *ImageColorSpace
	if hasBase {
		basePtr = &base
	}
	return json.Marshal(projection{
		Name: c.Name(), Components: c.Components(), Spec: c.SpecCopy(), Base: basePtr,
		Colorants: c.ColorantsCopy(), High: c.High(), Lookup: lookup, ProfileN: c.ProfileN(),
	})
}

func (d *Document) describeImageColorSpace(o Object) ImageColorSpace {
	o, _ = d.resolveIndirectChain(o)
	if name, ok := o.(Name); ok {
		name = Name(imageColorSpaceName(string(name)))
		return ImageColorSpace{data: imagedata.NewColorSpace(string(name), colorSpaceComponents(string(name))).WithSpec(o)}
	}
	a, ok := o.(Array)
	if !ok || len(a) == 0 {
		return ImageColorSpace{}
	}
	nameValue, _ := d.resolveIndirectChain(a[0])
	name, _ := nameValue.(Name)
	info := ImageColorSpace{data: imagedata.NewColorSpace(imageColorSpaceName(string(name)), 0).WithSpec(o)}
	if !validColorSpaceArrayLength(info.Name(), len(a)) {
		return ImageColorSpace{}
	}
	switch info.Name() {
	case "CalGray", "CalRGB", "Lab":
		if !d.validCalibratedColorSpace(info.Name(), a[1]) {
			return ImageColorSpace{}
		}
		// Calibrated spaces are fully validated above, but unlike device
		// spaces their component count is not populated by the generic name
		// fallback. Preserve the count so content color-space selections can
		// be described and validated like their Playa counterparts.
		info.data = info.data.WithComponents(colorSpaceComponents(info.Name()))
	case "ICCBased":
		if len(a) <= 1 {
			return ImageColorSpace{}
		}
		profileValue, _ := d.resolveIndirectChain(a[1])
		profile, ok := profileValue.(Stream)
		if !ok {
			return ImageColorSpace{}
		}
		profileN := d.iccProfileComponents(profile)
		if profileN <= 0 {
			return ImageColorSpace{}
		}
		info.data = info.data.WithProfileN(profileN).WithComponents(profileN)
	case "Indexed":
		if len(a) > 1 {
			base := d.describeImageColorSpace(a[1])
			if base.Components() <= 0 {
				return ImageColorSpace{}
			}
			info.data = info.data.WithBase(base.data).WithComponents(1)
		}
		if len(a) > 2 {
			var highOK bool
			high, highOK := indexedHighValue(d, a[2])
			if !highOK {
				return ImageColorSpace{}
			}
			info.data = info.data.WithHigh(high)
		}
		if len(a) > 3 {
			info.lookup, info.lookupData, info.lookupFilters, info.lookupFilterParms = d.resolveLookup(a[3])
			baseComponents := 0
			if base, ok := info.BaseCopy(); ok {
				baseComponents = base.Components()
			}
			if len(info.lookupFilters) > 0 && indexedLookupCacheAllowed(info.High(), baseComponents, d.cacheLimits().ImageBytes) {
				info.lookupCache = &imageDecodeCache{limit: d.cacheLimits().ImageBytes}
			}
		}
	case "Separation":
		if len(a) <= 2 {
			return ImageColorSpace{}
		}
		colorantValue, _ := d.resolveIndirectChain(a[1])
		switch colorant := colorantValue.(type) {
		case Name:
			if colorant == "" {
				return ImageColorSpace{}
			}
			info.data = info.data.WithColorants([]string{string(colorant)})
		case Array:
			var namesOK bool
			colorants, namesOK := d.colorantNames(colorant)
			if !namesOK {
				return ImageColorSpace{}
			}
			info.data = info.data.WithColorants(colorants)
		default:
			return ImageColorSpace{}
		}
		base := d.describeImageColorSpace(a[2])
		if base.Components() <= 0 {
			return ImageColorSpace{}
		}
		info.data = info.data.WithBase(base.data).WithComponents(1)
	case "DeviceN":
		if len(a) <= 1 {
			return ImageColorSpace{}
		}
		var namesOK bool
		colorants, namesOK := d.colorantNames(a[1])
		if !namesOK {
			return ImageColorSpace{}
		}
		info.data = info.data.WithColorants(colorants)
		if len(a) == 4 {
			base := d.describeImageColorSpace(a[2])
			if base.Components() <= 0 {
				return ImageColorSpace{}
			}
			info.data = info.data.WithBase(base.data)
		}
		info.data = info.data.WithComponents(len(colorants))
	case "Pattern":
		info.data = info.data.WithComponents(1)
		if len(a) > 1 {
			base := d.describeImageColorSpace(a[1])
			if base.Components() <= 0 || base.Name() == "Pattern" {
				return ImageColorSpace{}
			}
			info.data = info.data.WithBase(base.data).WithComponents(1 + base.Components())
		}
	default:
		info.data = info.data.WithComponents(colorSpaceComponents(info.Name()))
	}
	return info
}

func indexedHighValue(d *Document, o Object) (int, bool) {
	resolved, _ := d.resolveIndirectChain(o)
	high, ok := IntValue(resolved)
	return high, ok && high >= 0 && high <= 255
}

func (d *Document) validSeparationColorant(o Object) bool {
	resolved, _ := d.resolveIndirectChain(o)
	switch colorant := resolved.(type) {
	case Name:
		return colorant != ""
	case Array:
		_, ok := d.colorantNames(colorant)
		return ok
	default:
		return false
	}
}

func (d *Document) validCalibratedColorSpace(name string, o Object) bool {
	resolved, _ := d.resolveIndirectChain(o)
	dict, ok := resolved.(Dict)
	if !ok {
		return false
	}
	whitePointValue, _ := d.resolveIndirectChain(dict[Name("WhitePoint")])
	whitePoint, ok := whitePointValue.(Array)
	if !ok || len(whitePoint) != 3 {
		return false
	}
	values := make([]float64, len(whitePoint))
	for i, item := range whitePoint {
		resolved, _ := d.resolveIndirectChain(item)
		value, ok := NumberValue(resolved)
		if !ok || math.IsNaN(value) || math.IsInf(value, 0) {
			return false
		}
		values[i] = value
	}
	if values[0] <= 0 || values[1] != 1 || values[2] <= 0 {
		return false
	}
	if value, present := dict[Name("BlackPoint")]; present && !d.validNonNegativeNumberArray(value, 3) {
		return false
	}
	switch name {
	case "CalGray":
		if value, present := dict[Name("Gamma")]; present {
			resolved, _ := d.resolveIndirectChain(value)
			gamma, ok := NumberValue(resolved)
			if !ok || math.IsNaN(gamma) || math.IsInf(gamma, 0) || gamma <= 0 {
				return false
			}
		}
	case "CalRGB":
		if value, present := dict[Name("Gamma")]; present && !d.validPositiveNumberArray(value, 3) {
			return false
		}
		if value, present := dict[Name("Matrix")]; present && !d.validFiniteNumberArray(value, 9) {
			return false
		}
	case "Lab":
		if value, present := dict[Name("Range")]; present && !d.validOrderedRange(value) {
			return false
		}
	}
	return true
}

func (d *Document) validFiniteNumberArray(o Object, length int) bool {
	_, ok := d.numericArrayValues(o, length)
	return ok
}

func (d *Document) validNonNegativeNumberArray(o Object, length int) bool {
	values, ok := d.numericArrayValues(o, length)
	if !ok {
		return false
	}
	for _, value := range values {
		if value < 0 {
			return false
		}
	}
	return true
}

func (d *Document) validPositiveNumberArray(o Object, length int) bool {
	values, ok := d.numericArrayValues(o, length)
	if !ok {
		return false
	}
	for _, value := range values {
		if value <= 0 {
			return false
		}
	}
	return true
}

func (d *Document) validOrderedRange(o Object) bool {
	values, ok := d.numericArrayValues(o, 4)
	return ok && values[0] <= values[1] && values[2] <= values[3]
}

func (d *Document) numericArrayValues(o Object, length int) ([]float64, bool) {
	resolved, _ := d.resolveIndirectChain(o)
	values, ok := resolved.(Array)
	if !ok || len(values) != length {
		return nil, false
	}
	out := make([]float64, len(values))
	for i, item := range values {
		itemValue, _ := d.resolveIndirectChain(item)
		value, ok := NumberValue(itemValue)
		if !ok || math.IsNaN(value) || math.IsInf(value, 0) {
			return nil, false
		}
		out[i] = value
	}
	return out, true
}

func (d *Document) colorantNames(o Object) ([]string, bool) {
	resolved, _ := d.resolveIndirectChain(o)
	names, ok := resolved.(Array)
	if !ok || len(names) == 0 {
		return nil, false
	}
	out := make([]string, len(names))
	for i, item := range names {
		itemValue, _ := d.resolveIndirectChain(item)
		name, ok := itemValue.(Name)
		if !ok || name == "" {
			return nil, false
		}
		out[i] = string(name)
	}
	return out, true
}

func validColorSpaceArrayLength(name string, length int) bool {
	switch name {
	case "CalGray", "CalRGB", "Lab":
		return length == 2
	case "ICCBased":
		return length == 2
	case "Separation":
		return length == 4
	case "DeviceN":
		return length == 2 || length == 4
	case "Indexed":
		return length == 4
	case "Pattern":
		return length == 1 || length == 2
	default:
		return true
	}
}

// iccProfileComponents follows the PDF/Playa fallback for ICC profiles that
// omit /N. The ICC header's color-space signature is at bytes 16 through 19.
func (d *Document) iccProfileComponents(profile Stream) int {
	profileDict := profile.DictBorrowed()
	nValue, _ := d.resolveIndirectChain(profileDict[Name("N")])
	if n, ok := IntValue(nValue); ok && n > 0 && n <= 15 {
		return n
	}
	data := profile.DataBorrowed()
	filters, parms := streamFiltersWithResolver(profileDict, d.resolveRaw)
	if len(filters) > 0 {
		decoded, err := decodeFiltersLimited(data, filters, parms, decodedFilterExpansionLimit)
		if err != nil {
			return 0
		}
		data = decoded
	}
	if len(data) < 20 {
		return 0
	}
	switch string(data[16:20]) {
	case "GRAY":
		return 1
	case "RGB ", "Lab ":
		return 3
	case "CMYK":
		return 4
	default:
		// Playa follows the ICC profile color-space signature convention for
		// arbitrary nCLR profiles and falls back to three components for other
		// valid profiles whose /N entry is omitted. Keep the fallback guarded by
		// the ICC profile signature so short arbitrary streams remain malformed.
		if data[17] == 'C' && data[18] == 'L' && data[19] == 'R' {
			if data[16] >= '1' && data[16] <= '9' {
				return int(data[16] - '0')
			}
			if data[16] >= 'A' && data[16] <= 'F' {
				return int(data[16]-'A') + 10
			}
		}
		if len(data) >= 40 && string(data[36:40]) == "acsp" {
			return 3
		}
		return 0
	}
}

func (c ImageColorSpace) lookupBytes() []byte {
	data, _ := c.lookupDecoded()
	return data
}

func (c ImageColorSpace) lookupDecoded() ([]byte, error) {
	if c.lookup != nil {
		return c.lookup, nil
	}
	if c.lookupData == nil {
		return c.lookup, nil
	}
	if len(c.lookupFilters) == 0 {
		return c.lookupData, nil
	}
	if c.lookupCache == nil {
		return decodeFiltersLimited(c.lookupData, c.lookupFilters, c.lookupFilterParms, decodedFilterExpansionLimit)
	}
	return c.lookupCache.decode(c.lookupData, c.lookupFilters, c.lookupFilterParms)
}

func (d *Document) resolveLookup(o Object) (lookup, data []byte, filters []string, parms []Dict) {
	resolved, _ := d.resolveIndirectChain(o)
	switch value := resolved.(type) {
	case String:
		return cloneObjectBytes(value), nil, nil, nil
	case Stream:
		filters, parms := streamFiltersWithResolver(value.DictBorrowed(), d.resolveRaw)
		if len(filters) == 0 {
			return cloneObjectBytes(value.DataBorrowed()), nil, nil, nil
		}
		return nil, cloneObjectBytes(value.DataBorrowed()), filters, cloneFilterParms(parms)
	}
	return nil, nil, nil, nil
}
