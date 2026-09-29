package document

import (
	"fmt"
	"iter"
	"sort"
	"strings"

	"github.com/lin-string/go-playa/fontdata"
	"github.com/lin-string/go-playa/geometry"
)

// FontResource is one lazily resolved font entry from page resources.
type FontResource struct {
	name string
	font *Font
}

// Name returns the resource key used by the page resource dictionary.
func (r FontResource) Name() string { return r.name }

// FontMetadata remains a core alias for internal compatibility. The owning
// value model lives in fontdata so public font values have one domain owner.
type FontMetadata = fontdata.Metadata

// Metadata returns the resolved font's scalar metadata without copying its
// lazy caches or parsed font programs.
func (r FontResource) Metadata() (FontMetadata, bool) {
	if r.font == nil {
		return FontMetadata{}, false
	}
	return fontdata.NewMetadata(r.font.name, r.font.flags, r.font.ascent, r.font.descent, r.font.leading, r.font.capHeight, r.font.stemV, r.font.fontMatrix, r.font.hasFlags, r.font.hasFontBBox, r.font.italicAngle, r.font.defaultWidth, r.font.vertical, r.font.fontBBox), true
}

// FontCopy returns an independent snapshot of the resolved font.
func (r FontResource) FontCopy() *Font {
	if r.font == nil {
		return nil
	}
	return r.font.Finalize()
}

// FontCopyWithError returns an independent font snapshot and reports deferred
// font-program or character-procedure failures.
func (r FontResource) FontCopyWithError() (*Font, error) {
	if r.font == nil {
		return nil, nil
	}
	return r.font.FinalizeWithError()
}

// Finalize returns an independent snapshot of the resolved font resource.
func (r FontResource) Finalize() FontResource {
	return FontResource{name: r.name, font: r.FontCopy()}
}

// FinalizeWithError returns an independent font resource snapshot and reports
// deferred font-program or character-procedure failures.
func (r FontResource) FinalizeWithError() (FontResource, error) {
	font, err := r.FontCopyWithError()
	if err != nil {
		return FontResource{}, err
	}
	return FontResource{name: r.name, font: font}, nil
}

func (d *Document) pageFontsDirect(p Page) (map[string]*Font, error) {
	d.fontLifecycle.RLock()
	defer d.fontLifecycle.RUnlock()
	out := map[string]*Font{}
	resolveFont := func(value Object) Object {
		resolved, _ := d.resolveIndirectChain(value)
		return resolved
	}
	res, err := d.pageFontResources(p)
	if err != nil {
		return nil, err
	}
	if res == nil {
		return out, nil
	}
	raw, present := res[Name("Font")]
	if !present {
		return out, nil
	}
	resolvedFonts, _ := d.resolveIndirectChain(raw)
	fonts, ok := resolvedFonts.(Dict)
	if !ok {
		return nil, fmt.Errorf("playa: Font resources are not a dictionary")
	}
	for key, value := range fonts {
		ref, hasRef := value.(Ref)
		if hasRef {
			if canonical, ok := d.finalIndirectRef(value); ok {
				ref = canonical
			}
			if cachedErr, ok := d.cachedFontError(ref); ok {
				return nil, cachedErr
			}
			for {
				cached, build, owner := d.acquireFontBuild(ref)
				if cached != nil {
					out[string(key)] = cached
					break
				}
				if owner {
					break
				}
				<-build.done
				if build.err != nil {
					return nil, build.err
				}
				if build.font != nil {
					out[string(key)] = build.font
					break
				}
			}
			if cached, ok := d.cachedFont(ref); ok {
				out[string(key)] = cached
				continue
			}
			if out[string(key)] != nil {
				continue
			}
		}
		resolvedSpec, _ := d.resolveIndirectChain(value)
		spec, ok := resolvedSpec.(Dict)
		if !ok {
			err := fmt.Errorf("playa: font resource %q is not a dictionary", key)
			if hasRef {
				d.finishFontBuild(ref, nil, err)
			}
			return nil, err
		}
		f := NewSimpleFont(string(key))
		f.document = d
		// PDF font defaults use 1000 units; NewSimpleFont keeps a smaller
		// fallback for callers constructing an unbound test font.
		f.defaultWidth = 1000
		if n, ok := resolvedFontName(resolveFont(spec[Name("BaseFont")])); ok {
			f.name = n
			f.baseName = n
			applyStandardFontMetrics(f)
		}
		if st, ok := resolveFont(spec[Name("Subtype")]).(Name); ok {
			f.fontType = string(st)
			f.subtype = string(st)
			f.type3 = f.subtype == "Type3"
			f.cid = f.subtype == "Type0" || f.subtype == "CIDFontType0" || f.subtype == "CIDFontType2"
			if f.cid {
				f.cidCoding = "unknown-unknown"
				if info, ok := resolveFont(spec[Name("CIDSystemInfo")]).(Dict); ok {
					f.cidCoding = cidCodingFromDict(resolveFont, info)
				}
			}
			if !f.cid {
				f.subtype = ""
			}
		}
		builtInType1 := false
		if f.fontType == "Type1" || f.fontType == "MMType1" {
			if builtInName, ok := playaBuiltInType1FontName(f.name); ok {
				f.name = builtInName
				f.baseName = builtInName
				applyPlayaBuiltInType1Metrics(f)
				builtInType1 = true
			}
		}
		if f.type3 {
			hasExplicitName := f.name != string(key)
			if descriptor, ok := resolveFont(spec[Name("FontDescriptor")]).(Dict); ok {
				if name, ok := resolvedFontName(resolveFont(descriptor[Name("FontName")])); ok {
					f.name = name
					f.baseName = name
					hasExplicitName = true
				}
			}
			if !hasExplicitName {
				if name, ok := resolvedFontName(resolveFont(spec[Name("Name")])); ok {
					f.name = name
					f.baseName = name
					hasExplicitName = true
				}
			}
			if !hasExplicitName {
				f.name = "unknown"
				f.baseName = "unknown"
			}
			f.resources, _ = resolveFont(spec[Name("Resources")]).(Dict)
			if matrix, ok := resolveFont(spec[Name("FontMatrix")]).(Array); ok && len(matrix) == 6 {
				valid := true
				for i := range f.fontMatrix {
					f.fontMatrix[i], valid = finiteNumberValue(resolveFont(matrix[i]))
					if !valid {
						break
					}
				}
				if !valid {
					f.fontMatrix = geometry.Matrix{0.001, 0, 0, 0.001, 0, 0}
				}
			}
			if bbox, ok := resolveFont(spec[Name("FontBBox")]).(Array); ok && len(bbox) == 4 {
				var values [4]float64
				valid := true
				for i := range values {
					values[i], valid = finiteNumberValue(resolveFont(bbox[i]))
					if !valid {
						break
					}
				}
				if valid {
					if values[1] > values[3] {
						values[1], values[3] = values[3], values[1]
					}
					f.fontBBox, f.hasFontBBox = values, true
					f.descent, f.ascent = values[1], values[3]
				}
			}
			if charProcs, ok := resolveFont(spec[Name("CharProcs")]).(Dict); ok {
				for glyph, value := range charProcs {
					if proc, ok := resolveFont(value).(Stream); ok {
						f.charProcs[string(glyph)] = proc
					}
				}
			}
		}
		if f.cid {
			switch enc := resolveFont(spec[Name("Encoding")]).(type) {
			case Stream:
				encDict := enc.DictBorrowed()
				filters, parms := streamFiltersWithResolver(encDict, resolveFont)
				f.cmapData = cloneObjectBytes(enc.DataBorrowed())
				f.cmapFilters = cloneFontStrings(filters)
				f.cmapFilterParms = cloneFilterParms(parms)
				f.cmapUse = resolveFont(encDict[Name("UseCMap")])
			case Name:
				if predefined, ok := identityPredefinedCMap(string(enc)); ok {
					// Predefined CMaps are immutable shared tables. Direct font
					// encodings never merge into them, so sharing avoids copying
					// large East Asian maps for every page and text object.
					f.cmap = predefined
				} else if predefined, err := loadPredefinedCMap(string(enc)); err == nil {
					f.cmap = predefined
				}
				f.vertical = (f.cmap != nil && f.cmap.Vertical()) || strings.HasSuffix(string(enc), "-V")
			}
		}
		if !f.cid && resolveFont(spec[Name("Encoding")]) == nil {
			f.cffImplicitEncoding = f.fontType == "Type1" || f.fontType == "MMType1"
			applyImplicitSimpleEncoding(d, f, spec)
		}
		if desc, ok := resolveFont(spec[Name("FontDescriptor")]).(Dict); ok && !builtInType1 {
			if !f.type3 && !f.cid {
				if name, ok := resolvedFontName(resolveFont(desc[Name("FontName")])); ok {
					f.name = name
				}
			}
			setFontMetricsResolved(d, f, desc)
			resolveFontBBox(d, f, desc)
			applyTrueTypeCMap(d, f, desc)
			applyCFFMetadata(d, f, desc)
			if resolveFont(spec[Name("Encoding")]) == nil {
				applyType1Encoding(d, f, desc)
			}
		}
		if first, ok := IntValue(resolveFont(spec[Name("FirstChar")])); ok && !builtInType1 {
			if widths, ok := resolveFont(spec[Name("Widths")]).(Array); ok {
				f.hasPDFWidths = true
				for i, value := range widths {
					if width, err := ParseWidth(resolveFont(value)); err == nil {
						if first+i >= 0 && first+i <= 255 {
							f.widths[byte(first+i)] = width
						}
					}
				}
			}
		}
		if enc := resolveFont(spec[Name("Encoding")]); enc != nil {
			if name, ok := enc.(Name); ok {
				f.applyEncoding(string(name))
			}
			if dif, ok := enc.(Dict); ok {
				if base, ok := resolveFont(dif[Name("BaseEncoding")]).(Name); ok {
					f.applyEncoding(string(base))
				} else {
					// Playa derives the implicit base when a dictionary omits
					// BaseEncoding, then applies Differences on top of it.
					f.cffImplicitEncoding = f.fontType == "Type1" || f.fontType == "MMType1"
					applyImplicitSimpleEncoding(d, f, spec)
					if f.cffImplicitEncoding && !builtInType1 {
						if desc, ok := resolveFont(spec[Name("FontDescriptor")]).(Dict); ok {
							applyType1Encoding(d, f, desc)
						}
					}
				}
				if a, ok := resolveFont(dif[Name("Differences")]).(Array); ok {
					resolvedDifferences := make(Array, len(a))
					for i, value := range a {
						resolvedDifferences[i] = resolveFont(value)
					}
					f.applyDifferences(resolvedDifferences)
				}
			}
		}
		if dw, ok := spec[Name("DW")]; ok && !builtInType1 {
			if x, e := ParseWidth(resolveFont(dw)); e == nil {
				f.defaultWidth = x
			}
		}
		isType0 := f.fontType == "Type0"
		// Playa treats a ToUnicode entry that aliases Encoding as an explicit
		// identity request. Such PDFs commonly reuse the encoding CMap object;
		// parsing that object as a Unicode map would turn CIDs into code points.
		toUnicodeAliasesEncoding := isType0 && fontObjectsAlias(d, spec[Name("ToUnicode")], spec[Name("Encoding")])
		if !toUnicodeAliasesEncoding {
			if tu := resolveFont(spec[Name("ToUnicode")]); tu != nil {
				if s, ok := tu.(Stream); ok {
					sDict := s.DictBorrowed()
					filters, parms := streamFiltersWithResolver(sDict, resolveFont)
					f.toUnicodeData = cloneObjectBytes(s.DataBorrowed())
					f.toUnicodeFilters = cloneFontStrings(filters)
					f.toUnicodeParms = cloneFilterParms(parms)
				}
			}
		}
		if isType0 {
			if ds, ok := resolveFont(spec[Name("DescendantFonts")]).(Array); ok && len(ds) > 0 {
				if child, ok := resolveFont(ds[0]).(Dict); ok {
					if name, ok := resolvedFontName(resolveFont(child[Name("BaseFont")])); ok {
						f.baseName = name
					}
					if info, ok := resolveFont(child[Name("CIDSystemInfo")]).(Dict); ok {
						f.cidCoding = cidCodingFromDict(resolveFont, info)
						if ordering, ok := resolvedFontName(resolveFont(info[Name("Ordering")])); ok {
							f.cidToUnicode = predefinedCIDUnicode(ordering, f.vertical)
						}
					}
					if subtype, ok := resolveFont(child[Name("Subtype")]).(Name); ok {
						f.subtype = string(subtype)
					}
					if cidToGID := resolveFont(child[Name("CIDToGIDMap")]); cidToGID != nil {
						if name, ok := cidToGID.(Name); !ok || name != Name("Identity") {
							if stream, ok := cidToGID.(Stream); ok {
								streamDict := stream.DictBorrowed()
								filters, parms := streamFiltersWithResolver(streamDict, resolveFont)
								f.cidToGIDData = cloneObjectBytes(stream.DataBorrowed())
								f.cidToGIDFilters = cloneFontStrings(filters)
								f.cidToGIDParms = cloneFilterParms(parms)
							}
						}
					}
					f.defaultWidth = 1000
					// PDF's default vertical metrics apply when DW2 is absent:
					// the position vector is (DW/2, 880) and the vertical
					// displacement is -1000 glyph-space units.
					f.defaultVPosition = [2]float64{f.defaultWidth / 2, 880}
					f.defaultVWidth = -1000
					hasDW2 := false
					if dw, ok := child[Name("DW")]; ok {
						if x, e := ParseWidth(resolveFont(dw)); e == nil {
							f.defaultWidth = x
						}
					}
					if w, ok := resolveFont(child[Name("W")]).(Array); ok {
						parseCIDWidths(d, w, f)
					}
					if dw2, ok := resolveFont(child[Name("DW2")]).(Array); ok && len(dw2) == 2 {
						position, positionOK := finiteNumberValue(resolveFont(dw2[0]))
						width, widthOK := finiteNumberValue(resolveFont(dw2[1]))
						if positionOK && widthOK {
							hasDW2 = true
							f.defaultVPosition[1] = position
							f.defaultVWidth = width
							f.defaultVPosition[0] = f.defaultWidth / 2
						}
					}
					if !hasDW2 {
						f.defaultVPosition[0] = f.defaultWidth / 2
					}
					if w2, ok := resolveFont(child[Name("W2")]).(Array); ok {
						parseCIDVerticalWidths(d, w2, f)
					}
					if desc, ok := resolveFont(child[Name("FontDescriptor")]).(Dict); ok {
						// Playa constructs a Type0 font from the merged descendant
						// dictionary, so its public fontname comes from the
						// descendant descriptor rather than the root Encoding suffix.
						if name, ok := resolvedFontName(resolveFont(desc[Name("FontName")])); ok {
							f.name = name
						}
						setFontMetricsResolved(d, f, desc)
						resolveFontBBox(d, f, desc)
						applyTrueTypeCMap(d, f, desc)
						applyCFFMetadata(d, f, desc)
					}
				}
			}
		}
		out[string(key)] = f
		if hasRef {
			d.finishFontBuild(ref, f, nil)
		}
	}
	return out, nil
}

// GetFont resolves a Playa-style standalone font specification. The object
// number is an optional cache identity; a zero number keeps the result local
// to this call. The returned font is read-only through the public API and may
// be reused by subsequent calls with the same object number.
func (d *Document) GetFont(objectID int, spec Dict) *Font {
	font, _ := d.GetFontWithError(objectID, spec)
	return font
}

// GetFontWithError is the diagnostic form of GetFont. It mirrors Playa's
// Document.get_font while accepting the Go-owned dictionary model.
func (d *Document) GetFontWithError(objectID int, spec Dict) (*Font, error) {
	if objectID > 0 {
		ref := Ref{Object: objectID}
		if cached, ok := d.cachedFont(ref); ok {
			return cached, nil
		}
	}
	if spec == nil {
		return newDummyFont(), nil
	}
	subtypeValue, subtypeResolved := d.resolveIndirectChain(spec[Name("Subtype")])
	if !subtypeResolved {
		subtypeValue = nil
	}
	if subtype, ok := subtypeValue.(Name); !ok || !knownFontSubtype(string(subtype)) {
		font := newDummyFont()
		if objectID > 0 {
			d.storeFont(Ref{Object: objectID}, font)
		}
		return font, nil
	}

	// pageFontsDirect already contains the full lazy font construction path,
	// including indirect descriptors, CMaps, embedded programs, and metrics.
	// Reuse it instead of maintaining a second standalone parser.
	resources := Dict{Name("Font"): Dict{Name("F"): spec}}
	fonts, err := d.pageFontsDirect(Page{dict: Dict{Name("Resources"): resources}})
	if err != nil {
		return nil, err
	}
	font := fonts["F"]
	if font == nil {
		font = newDummyFont()
	}
	if objectID > 0 {
		d.storeFont(Ref{Object: objectID}, font)
	}
	return font, nil
}

func knownFontSubtype(subtype string) bool {
	switch subtype {
	case "Type1", "MMType1", "TrueType", "Type3", "Type0":
		return true
	default:
		return false
	}
}

func newDummyFont() *Font {
	font := NewSimpleFont("unknown")
	font.baseName = "unknown"
	font.defaultWidth = 1000
	font.encoding = make(map[byte]rune, 256)
	font.glyphTexts = make(map[byte]string, 256)
	for code := 0; code <= 255; code++ {
		value := byte(code)
		font.encoding[value] = rune(value)
		font.glyphTexts[value] = string(rune(value))
	}
	return font
}

// pageFontResources validates the resource roots needed by page font
// expansion. It deliberately does not resolve font programs or XObject
// streams, keeping the public mapping lazy while rejecting explicit shape
// errors instead of silently treating them as empty resources.
func (d *Document) pageFontResources(p Page) (Dict, error) {
	var resources Dict
	if p.hasCachedResources(d) {
		resources, _ = d.cachedPageResource(p.ref)
	} else {
		raw, present := p.dict[Name("Resources")]
		if !present {
			return nil, nil
		}
		resolvedResources, resolvedOK := d.resolveIndirectChain(raw)
		if !resolvedOK {
			return nil, fmt.Errorf("playa: Resources could not be resolved")
		}
		var ok bool
		resources, ok = resolvedResources.(Dict)
		if !ok {
			return nil, fmt.Errorf("playa: Resources is not a dictionary")
		}
	}
	if raw, present := resources[Name("Font")]; present {
		resolvedFonts, resolvedOK := d.resolveIndirectChain(raw)
		if !resolvedOK {
			return nil, fmt.Errorf("playa: Font resources could not be resolved")
		}
		fonts, ok := resolvedFonts.(Dict)
		if !ok {
			return nil, fmt.Errorf("playa: Font resources are not a dictionary")
		}
		for name, value := range fonts {
			ref, hasRef := value.(Ref)
			if hasRef {
				if canonical, ok := d.finalIndirectRef(ref); ok {
					ref = canonical
				}
				if cachedErr, ok := d.cachedFontError(ref); ok {
					return nil, cachedErr
				}
			}
			resolved, resolvedOK := d.resolveIndirectChain(value)
			if !resolvedOK {
				err := fmt.Errorf("playa: font resource %q could not be resolved", name)
				return nil, d.storeFontError(ref, err)
			}
			if _, ok := resolved.(Dict); !ok {
				err := fmt.Errorf("playa: font resource %q is not a dictionary", name)
				return nil, d.storeFontError(ref, err)
			}
		}
	}
	if raw, present := resources[Name("XObject")]; present {
		resolved, _ := d.resolveIndirectChain(raw)
		if _, ok := resolved.(Dict); !ok {
			return nil, fmt.Errorf("playa: XObject resources are not a dictionary")
		}
	}
	return resources, nil
}

func resolvedFontName(value Object) (string, bool) {
	switch value := value.(type) {
	case Name:
		return string(value), true
	case String:
		return decodePDFText(value), true
	default:
		return "", false
	}
}

func cidCodingFromDict(resolve func(Object) Object, info Dict) string {
	registry := "unknown"
	ordering := "unknown"
	if value, ok := resolvedFontName(resolve(info[Name("Registry")])); ok && strings.TrimSpace(value) != "" {
		registry = strings.TrimSpace(value)
	}
	if value, ok := resolvedFontName(resolve(info[Name("Ordering")])); ok && strings.TrimSpace(value) != "" {
		ordering = strings.TrimSpace(value)
	}
	return registry + "-" + ordering
}

func fontObjectsAlias(d *Document, left, right Object) bool {
	leftRef, leftOK := left.(Ref)
	rightRef, rightOK := right.(Ref)
	if !leftOK || !rightOK {
		return false
	}
	if canonical, ok := d.finalIndirectRef(leftRef); ok {
		leftRef = canonical
	}
	if canonical, ok := d.finalIndirectRef(rightRef); ok {
		rightRef = canonical
	}
	return leftRef == rightRef
}

// applyImplicitSimpleEncoding selects the encoding used by a simple font when
// the PDF omits an encoding name or omits BaseEncoding from an encoding
// dictionary. Type3 fonts deliberately have no implicit mapping.
func applyImplicitSimpleEncoding(d *Document, f *Font, spec Dict) {
	if f.type3 {
		f.encoding = map[byte]rune{}
		f.glyphTexts = map[byte]string{}
		f.strictEncoding = true
		if _, hasEncoding := spec[Name("Encoding")]; !hasEncoding {
			// Playa keeps the Type3 encoding table empty when the required
			// Encoding entry is absent, but its SimpleFont fallback still
			// decodes through StandardEncoding. Keep the table empty for
			// metadata queries and store only the decode projection here.
			for i := byte(32); i < 127; i++ {
				f.glyphTexts[i] = string(rune(i))
			}
			if standard, ok := fontdata.BuiltinEncoding("StandardEncoding"); ok {
				for code, value := range standard {
					f.glyphTexts[code] = string(value)
				}
			}
		}
		return
	}
	if f.fontType == "TrueType" {
		descriptorValue, _ := d.resolveIndirectChain(spec[Name("FontDescriptor")])
		descriptor, _ := descriptorValue.(Dict)
		flagsValue, _ := d.resolveIndirectChain(descriptor[Name("Flags")])
		flags, _ := IntValue(flagsValue)
		if flags&32 == 0 {
			f.encoding = map[byte]rune{}
			f.glyphTexts = map[byte]string{}
			f.strictEncoding = true
			return
		}
	}
	encodingName := "StandardEncoding"
	switch f.name {
	case "Symbol":
		encodingName = "Symbol"
	case "ZapfDingbats":
		encodingName = "ZapfDingbats"
	}
	f.applyEncoding(encodingName)
}

func (d *Document) loadUseCMap(value Object) *fontdata.CMap {
	return d.loadUseCMapWithStack(value, map[string]bool{})
}

func (d *Document) loadUseCMapWithStack(value Object, stack map[string]bool) *fontdata.CMap {
	switch use := value.(type) {
	case Name:
		name := string(use)
		if stack[name] {
			return nil
		}
		stack[name] = true
		defer delete(stack, name)
		cmap, err := loadPredefinedCMap(name)
		if err == nil {
			// Never attach the shared predefined instance to a document-owned
			// CMap graph. Callers may merge inherited mappings into the result.
			return cmap.Finalize()
		}
	case Stream:
		useDict := use.DictBorrowed()
		filters, parms := streamFiltersWithResolver(useDict, d.resolveIndirectValue)
		data := use.DataBorrowed()
		if len(filters) > 0 {
			decoded, err := decodeFiltersLimited(data, filters, parms, decodedFilterExpansionLimit)
			if err != nil {
				return nil
			}
			data = decoded
		}
		cmap, err := fontdata.ParseCMap(data)
		if err == nil {
			if cmap.UseCMap() != "" {
				base := d.loadUseCMapWithStack(Name(cmap.UseCMap()), stack)
				fontdata.MergeUseCMap(cmap, base)
			}
			return cmap
		}
	}
	return nil
}

// pageFontByName resolves only the font named by a Tf operator. Form fonts
// use the same slash-separated prefix as expandOps and contentOpIterator.
func (d *Document) pageFontByName(p Page, name string) *Font {
	if p.hasPageCacheKey() {
		if font, ok := d.cachedPageFont(p.ref, name); ok {
			return font
		}
	}
	var resources Dict
	if cached, ok := d.cachedPageResource(p.ref); p.hasPageCacheKey() && ok {
		resources = cached
	} else {
		resourcesValue, _ := d.resolveIndirectChain(p.dict[Name("Resources")])
		resources, _ = resourcesValue.(Dict)
	}
	parts := strings.Split(name, "/")
	if len(parts) == 0 {
		return nil
	}
	for i := 0; i < len(parts)-1; i++ {
		xobjectsValue, _ := d.resolveIndirectChain(resources[Name("XObject")])
		xobjects, _ := xobjectsValue.(Dict)
		formValue, _ := d.resolveIndirectChain(xobjects[Name(parts[i])])
		form, _ := formValue.(Stream)
		formDict := form.DictBorrowed()
		subtypeValue, _ := d.resolveIndirectChain(formDict[Name("Subtype")])
		if subtypeValue != Name("Form") {
			return nil
		}
		nextValue, _ := d.resolveIndirectChain(formDict[Name("Resources")])
		if next, ok := nextValue.(Dict); ok {
			resources = next
		}
	}
	fontsValue, _ := d.resolveIndirectChain(resources[Name("Font")])
	fonts, _ := fontsValue.(Dict)
	value, ok := fonts[Name(parts[len(parts)-1])]
	if !ok {
		return nil
	}
	ref, hasRef := value.(Ref)
	if hasRef {
		if canonical, ok := d.finalIndirectRef(value); ok {
			ref = canonical
		}
		if font, ok := d.cachedFont(ref); ok {
			return font
		}
	}
	single := Dict{Name("Resources"): Dict{Name("Font"): Dict{Name(parts[len(parts)-1]): value}}}
	resolved, err := d.pageFontsDirect(Page{dict: single})
	if err != nil {
		return nil
	}
	font := resolved[parts[len(parts)-1]]
	if font != nil && hasRef {
		d.storeFont(ref, font)
	}
	if font != nil && p.hasPageCacheKey() {
		d.storePageFont(p.ref, name, font)
	}
	return font
}

func applyTrueTypeCMap(d *Document, f *Font, desc Dict) {
	resolveFont := func(value Object) Object {
		resolved, _ := d.resolveIndirectChain(value)
		return resolved
	}
	programValue := desc[Name("FontFile2")]
	stream, ok := resolveFont(programValue).(Stream)
	if !ok {
		programValue = desc[Name("FontFile3")]
		stream, ok = resolveFont(programValue).(Stream)
		if !ok {
			return
		}
		streamDict := stream.DictBorrowed()
		if subtype, ok := resolveFont(streamDict[Name("Subtype")]).(Name); !ok || subtype != Name("OpenType") {
			return
		}
	}
	if ref, ok := d.finalIndirectRef(programValue); ok {
		f.trueTypeRef = ref
	} else if ref, ok := stream.Ref(); ok {
		f.trueTypeRef = ref
	}
	streamDict := stream.DictBorrowed()
	filters, parms := streamFiltersWithResolver(streamDict, resolveFont)
	f.trueTypeData = cloneObjectBytes(stream.DataBorrowed())
	f.trueTypeFilters = cloneFontStrings(filters)
	f.trueTypeParms = cloneFilterParms(parms)
}

func applyCFFMetadata(d *Document, f *Font, desc Dict) {
	resolveFont := func(value Object) Object {
		resolved, _ := d.resolveIndirectChain(value)
		return resolved
	}
	stream, ok := resolveFont(desc[Name("FontFile3")]).(Stream)
	if !ok {
		return
	}
	streamDict := stream.DictBorrowed()
	filters, parms := streamFiltersWithResolver(streamDict, resolveFont)
	f.cffData = cloneObjectBytes(stream.DataBorrowed())
	f.cffFilters = cloneFontStrings(filters)
	f.cffParms = cloneFilterParms(parms)
}

func applyCFFMetadataData(f *Font, data []byte) bool {
	metadata, ok := parseCFFMetadata(data)
	if !ok {
		return false
	}
	f.cff2 = metadata.cff2
	if metadata.cff2 && metadata.variationStore > 0 {
		f.cffVariationStore = parseCFF2VariationStore(data, metadata.variationStore)
	}
	if metadata.cff2 {
		f.cffVariationIndex = metadata.variationIndex
		f.cffDefaultWidthVar = cloneCFFBlendValue(metadata.defaultWidthVar)
		f.cffNominalWidthVar = cloneCFFBlendValue(metadata.nominalWidthVar)
	}
	if len(metadata.cidToGID) > 0 {
		f.cidToGID = cloneIntIntMap(f.cidToGID)
		if f.cidToGID == nil {
			f.cidToGID = map[int]int{}
		}
		for cid, glyphID := range metadata.cidToGID {
			if _, exists := f.cidToGID[cid]; !exists {
				f.cidToGID[cid] = glyphID
			}
		}
	}
	if len(metadata.glyphText) > 0 {
		f.glyphIDToUnicode = cloneIntStringMap(f.glyphIDToUnicode)
		if f.glyphIDToUnicode == nil {
			f.glyphIDToUnicode = map[int]string{}
		}
		for glyphID, text := range metadata.glyphText {
			f.glyphIDToUnicode[glyphID] = text
		}
	}
	// CID layout follows PDF W/DW metrics. The embedded CFF glyph widths are
	// retained by the parser for diagnostics but must not become a resident
	// Font cache because the public CID width precedence deliberately ignores
	// them.
	if len(metadata.glyphWidths) > 0 && !metadata.isCID {
		f.glyphIDWidths = cloneIntFloatMap(f.glyphIDWidths)
		if f.glyphIDWidths == nil {
			f.glyphIDWidths = map[int]float64{}
		}
		if scale, ok := cffWidthScale(f.fontMatrix); ok {
			for glyphID, width := range metadata.glyphWidths {
				if _, exists := f.glyphIDWidths[glyphID]; !exists {
					if value, ok := cffScaledWidth(width, scale); ok {
						f.glyphIDWidths[glyphID] = value
					}
				}
			}
		}
	}
	applyCFFCodeNames(f, metadata.codeNames, metadata.glyphNames)
	if f.cffImplicitEncoding {
		f.strictEncoding = true
	}
	if len(metadata.charstringsData) > 0 && !metadata.isCID {
		// cffIndex returns slices into the decoded, immutable font buffer. Keep
		// those borrowed slices here; Font.Finalize performs the owned copy.
		f.cffCharstrings = metadata.charstringsData
		f.cffLocalSubrs = metadata.localSubrs
		f.cffGlobalSubrs = metadata.globalSubrs
	}
	if len(metadata.charstringsData) > 0 && metadata.isCID {
		f.cffCharstrings = metadata.charstringsData
		f.cffGlobalSubrs = metadata.globalSubrs
		f.cffFDByGlyph, f.cffFDLocalSubrs, f.cffFDVariationIndex = parseCFFCIDSubroutines(data, &metadata)
	}
	if !f.hasFontBBox && metadata.hasBBox {
		f.fontBBox, f.hasFontBBox = metadata.bbox, true
		if f.ascent == 0 && f.descent == 0 {
			f.descent, f.ascent = metadata.bbox[1], metadata.bbox[3]
		}
	}
	if metadata.hasMatrix && f.fontMatrix == (geometry.Matrix{0.001, 0, 0, 0.001, 0, 0}) {
		f.fontMatrix = metadata.matrix
	}
	// An explicit PDF Widths table is authoritative even outside its range:
	// those character codes use MissingWidth, not embedded program metrics.
	if !f.hasPDFWidths && len(metadata.charstringsData) > 0 && len(metadata.glyphNames) > 0 {
		f.widths = cloneByteFloatMap(f.widths)
		if f.widths == nil {
			f.widths = map[byte]float64{}
		}
		if scale, ok := cffWidthScale(f.fontMatrix); ok {
			defaultWidth := metadata.defaultWidth
			if !metadata.hasDefaultWidth {
				defaultWidth = f.defaultWidth / scale
			}
			nominalWidth := metadata.nominalWidth
			for code, gid := range f.cffGlyphIDs {
				if _, exists := f.widths[code]; exists {
					continue
				}
				if gid <= 0 || gid >= len(metadata.charstringsData) {
					continue
				}
				var width float64
				var ok bool
				if metadata.cff2 {
					width, ok = parseCFF2CharStringWidthWithGlobals(metadata.charstringsData[gid], nominalWidth, defaultWidth, metadata.localSubrs, metadata.globalSubrs, metadata.variation, nil, metadata.variationIndex)
				} else {
					width, ok = parseCFFCharStringWidthWithGlobals(metadata.charstringsData[gid], nominalWidth, defaultWidth, metadata.localSubrs, metadata.globalSubrs)
				}
				if ok {
					if value, ok := cffScaledWidth(width, scale); ok {
						f.widths[code] = value
					}
				}
			}
		}
	}
	return true
}

func applyCFFCodeNames(f *Font, codeNames map[byte]string, glyphNames []string) {
	if f == nil || f.cid {
		return
	}
	// CFF2 has no charset; retain glyph selection supplied by the OpenType cmap.
	if !f.cffImplicitEncoding && len(glyphNames) == 0 {
		return
	}

	names := cloneByteStringMap(f.glyphNames)
	if names == nil {
		names = map[byte]string{}
	}
	if f.cffImplicitEncoding {
		// Select the embedded base before applying the PDF Differences. Replacing
		// these tables keeps previously published decode snapshots immutable.
		names = cloneByteStringMap(codeNames)
		if names == nil {
			names = map[byte]string{}
		}
		for code, name := range f.cffDifferences {
			names[code] = name
		}
		f.encoding = map[byte]rune{}
		f.glyphTexts = map[byte]string{}
		f.strictEncoding = true
		for code, name := range names {
			if text, ok := glyphText(name); ok {
				f.glyphTexts[code] = text
			}
		}
	} else {
		// An explicit PDF base encoding remains authoritative. Match its Unicode
		// values to the charset only to select outlines, never to replace decoding.
		nameByText := make(map[string]string, len(glyphNames))
		for _, name := range glyphNames {
			if text, ok := glyphText(name); ok {
				prior, exists := nameByText[text]
				if !exists || (strings.Contains(prior, ".") && !strings.Contains(name, ".")) {
					nameByText[text] = name
				}
			}
		}
		canonicalByText := make(map[string]string, len(fontdata.CFFStandardStrings))
		for _, name := range fontdata.CFFStandardStrings {
			if text, ok := glyphText(name); ok {
				if _, exists := canonicalByText[text]; !exists {
					canonicalByText[text] = name
				}
			}
		}
		for code, value := range f.encoding {
			if _, present := names[code]; present {
				continue
			}
			// Preserve the actual name when the base encoding publishes one.
			// Otherwise prefer the existing canonical CFF names before Unicode aliases.
			switch f.simpleEncodingName {
			case "StandardEncoding":
				if name, ok := fontdata.CFFSIDName(fontdata.CFFStandardEncoding[code], nil); ok {
					names[code] = name
				}
			case "MacExpertEncoding":
				names[code] = fontdata.MacExpertEncoding[code]
			default:
				if name, ok := canonicalByText[string(value)]; ok {
					names[code] = name
				} else if name, ok := nameByText[string(value)]; ok {
					names[code] = name
				}
			}
		}
	}
	f.glyphNames = names
	f.cffGlyphIDs = make(map[byte]int, len(names))
	for code, name := range names {
		// A missing selected name is .notdef, not a different Unicode-equivalent
		// glyph found later by the generic glyph-ID fallback.
		gid, _ := glyphIDByName(glyphNames, name)
		f.cffGlyphIDs[code] = gid
	}
	f.cffDifferences = nil
}

func parseType3CharProc(d *Document, f *Font, name string, proc Stream) {
	_ = parseType3CharProcWithError(d, f, name, proc)
}

func parseType3CharProcWithError(d *Document, f *Font, name string, proc Stream) error {
	procDict := proc.DictBorrowed()
	filters, parms := streamFiltersWithResolver(procDict, d.resolveIndirectValue)
	data := proc.DataBorrowed()
	if len(filters) > 0 {
		decoded, err := decodeFiltersLimited(data, filters, parms, decodedFilterExpansionLimit)
		if err != nil {
			return fmt.Errorf("playa: decode Type3 CharProc %q: %w", name, err)
		}
		data = decoded
	}
	ops, err := ParseContent(data)
	if err != nil {
		return fmt.Errorf("playa: parse Type3 CharProc %q: %w", name, err)
	}
	if len(ops) == 0 {
		return fmt.Errorf("playa: empty Type3 CharProc %q", name)
	}
	if f.charProcOps == nil {
		f.charProcOps = map[string][]ContentOp{}
	}
	f.charProcOps[name] = append([]ContentOp(nil), ops...)
	first := ops[0]
	if first.operatorValue() == "d0" && len(first.operandsValue()) == 2 {
		if width, ok := finiteNumberValue(first.operandsValue()[0]); ok {
			f.charWidths[name] = width
			return nil
		}
		return fmt.Errorf("playa: invalid Type3 CharProc %q d0 width", name)
	}
	if first.operatorValue() == "d0" {
		return fmt.Errorf("playa: invalid Type3 CharProc %q d0 operands", name)
	}
	if first.operatorValue() != "d1" {
		return nil
	}
	if len(first.operandsValue()) != 6 {
		return fmt.Errorf("playa: invalid Type3 CharProc %q d1 operands", name)
	}
	width, ok := finiteNumberValue(first.operandsValue()[0])
	if !ok {
		return fmt.Errorf("playa: invalid Type3 CharProc %q d1 width", name)
	}
	var bbox [4]float64
	for i := range bbox {
		var ok bool
		bbox[i], ok = finiteNumberValue(first.operandsValue()[i+2])
		if !ok {
			return fmt.Errorf("playa: invalid Type3 CharProc %q d1 bbox", name)
		}
	}
	if bbox[0] > bbox[2] {
		bbox[0], bbox[2] = bbox[2], bbox[0]
	}
	if bbox[1] > bbox[3] {
		bbox[1], bbox[3] = bbox[3], bbox[1]
	}
	f.charWidths[name] = width
	f.charBBoxes[name] = bbox
	return nil
}

func identityCMap(vertical bool) *fontdata.CMap {
	return fontdata.NewIdentityCMap(vertical, 2)
}

// PageFonts returns fonts declared directly by the page resource dictionary.
// Form XObject fonts are resolved separately by flattening interpreters.
func (d *Document) PageFonts(p Page) (map[string]*Font, error) {
	if p.hasPageCacheKey() {
		if fonts, ok := d.cachedPageFonts(p.ref); ok {
			return cloneFontMap(fonts), nil
		}
	}
	out := map[string]*Font{}
	for entry, err := range d.PageFontSeq(p) {
		if err != nil {
			return nil, err
		}
		out[entry.name] = entry.font
	}
	if p.hasPageCacheKey() {
		d.storePageFonts(p.ref, out)
	}
	return cloneFontMap(out), nil
}

func cloneFontMap(fonts map[string]*Font) map[string]*Font {
	if fonts == nil {
		return nil
	}
	clone := make(map[string]*Font, len(fonts))
	for name, font := range fonts {
		clone[name] = font.Finalize()
	}
	return clone
}

// PageFontSeq yields page resource fonts in stable resource order.
// Font dictionaries are resolved only when their entry is requested.
func (d *Document) PageFontSeq(p Page) iter.Seq2[FontResource, error] {
	return d.pageFontSeq(p, false)
}

func (d *Document) pageFontSeq(p Page, reverse bool) iter.Seq2[FontResource, error] {
	if d == nil {
		return func(yield func(FontResource, error) bool) {
			yield(FontResource{}, errNilDocument)
		}
	}
	return func(yield func(FontResource, error) bool) {
		resources := Dict{}
		if cached, ok := d.cachedPageResource(p.ref); p.hasPageCacheKey() && ok {
			resources = cached
		} else if raw, present := p.dict[Name("Resources")]; present {
			resolved, resolvedOK := d.resolveIndirectChain(raw)
			if !resolvedOK {
				yield(FontResource{}, fmt.Errorf("playa: Resources could not be resolved"))
				return
			}
			var ok bool
			resources, ok = resolved.(Dict)
			if !ok {
				yield(FontResource{}, fmt.Errorf("playa: Resources is not a dictionary"))
				return
			}
		}
		fonts := Dict{}
		if raw, present := resources[Name("Font")]; present {
			resolved, resolvedOK := d.resolveIndirectChain(raw)
			if !resolvedOK {
				yield(FontResource{}, fmt.Errorf("playa: Font resources could not be resolved"))
				return
			}
			var ok bool
			fonts, ok = resolved.(Dict)
			if !ok {
				yield(FontResource{}, fmt.Errorf("playa: Font resources are not a dictionary"))
				return
			}
		}
		names := []string{}
		for name := range fonts {
			names = append(names, string(name))
		}
		sort.Strings(names)
		if reverse {
			if sourceNames, ok := d.sourceFontResourceNames(p, fonts); ok {
				names = sourceNames
			}
		}
		visit := func(name string) bool {
			resolved, resolvedOK := d.resolveIndirectChain(fonts[Name(name)])
			if !resolvedOK {
				yield(FontResource{}, fmt.Errorf("playa: font resource %q could not be resolved", name))
				return false
			}
			if _, ok := resolved.(Dict); !ok {
				yield(FontResource{}, fmt.Errorf("playa: font resource %q is not a dictionary", name))
				return false
			}
			var font *Font
			if p.hasPageCacheKey() {
				if cached, ok := d.cachedPageFont(p.ref, name); ok {
					font = cached
				}
			}
			if font == nil {
				font = d.pageFontByName(p, name)
			}
			if font == nil {
				return true
			}
			if p.hasPageCacheKey() {
				d.storePageFont(p.ref, name, font)
			}
			if !yield(FontResource{name: name, font: font}, nil) {
				return false
			}
			return true
		}
		if reverse {
			for index := len(names) - 1; index >= 0; index-- {
				if !visit(names[index]) {
					return
				}
			}
			return
		}
		for _, name := range names {
			if !visit(name) {
				return
			}
		}
	}
}

// Fonts returns the document-level font mapping keyed by each font's PDF
// name. When names collide, the font from the later page wins, matching
// Playa's document font mapping.
func (d *Document) Fonts() (map[string]*Font, error) {
	out := map[string]*Font{}
	for entry, err := range d.FontsSeq() {
		if err != nil {
			return nil, err
		}
		out[entry.name] = entry.font.Finalize()
	}
	return out, nil
}

// FontsSeq traverses the document-level font mapping in Playa's lazy order.
// Pages are visited from the end so a later page takes precedence when two
// resources resolve to the same embedded font name.
func (d *Document) FontsSeq() iter.Seq2[FontResource, error] {
	return func(yield func(FontResource, error) bool) {
		seen := map[string]bool{}
		visited := map[Ref]bool{}
		for page, err := range d.pagesReverseSeq() {
			if err != nil {
				yield(FontResource{}, err)
				return
			}
			for entry, err := range d.pageFontSeq(page, true) {
				if err != nil {
					yield(FontResource{}, err)
					return
				}
				font := entry.font
				if font == nil || font.name == "" || seen[font.name] {
					continue
				}
				seen[font.name] = true
				if !yield(FontResource{name: font.name, font: font}, nil) {
					return
				}
				if font.type3 && !d.walkNestedFontResources(font.resources, seen, visited, yield) {
					return
				}
			}
			resources, err := page.ResourcesWithError(d)
			if err != nil {
				yield(FontResource{}, err)
				return
			}
			if !d.walkNestedFontResources(resources, seen, visited, yield) {
				return
			}
		}
	}
}

func (d *Document) walkNestedFontResources(resources Dict, seen map[string]bool, visited map[Ref]bool, yield func(FontResource, error) bool) bool {
	if resources == nil {
		return true
	}
	fontsValue, resolved := d.resolveIndirectChain(resources[Name("Font")])
	if resources[Name("Font")] != nil && !resolved {
		return yield(FontResource{}, fmt.Errorf("playa: nested Font resources could not be resolved"))
	}
	fonts, _ := fontsValue.(Dict)
	fontNames := make([]string, 0, len(fonts))
	for name := range fonts {
		fontNames = append(fontNames, string(name))
	}
	sort.Sort(sort.Reverse(sort.StringSlice(fontNames)))
	for _, resourceName := range fontNames {
		raw := fonts[Name(resourceName)]
		if ref, ok := d.finalIndirectRef(raw); ok {
			if visited[ref] {
				continue
			}
			visited[ref] = true
		}
		resolvedFont, ok := d.resolveIndirectChain(raw)
		if !ok {
			return yield(FontResource{}, fmt.Errorf("playa: nested font resource %q could not be resolved", resourceName))
		}
		if _, ok := resolvedFont.(Dict); !ok {
			return yield(FontResource{}, fmt.Errorf("playa: nested font resource %q is not a dictionary", resourceName))
		}
		font := d.pageFontByName(Page{dict: Dict{Name("Resources"): resources}}, resourceName)
		if font == nil {
			continue
		}
		if !seen[font.name] {
			seen[font.name] = true
			if !yield(FontResource{name: font.name, font: font}, nil) {
				return false
			}
			if font.type3 && !d.walkNestedFontResources(font.resources, seen, visited, yield) {
				return false
			}
		}
	}
	for _, category := range []Name{Name("Pattern"), Name("XObject")} {
		objectsValue, resolved := d.resolveIndirectChain(resources[category])
		if resources[category] != nil && !resolved {
			return yield(FontResource{}, fmt.Errorf("playa: nested %s resources could not be resolved", category))
		}
		objects, _ := objectsValue.(Dict)
		names := make([]string, 0, len(objects))
		for name := range objects {
			names = append(names, string(name))
		}
		sort.Strings(names)
		for _, name := range names {
			raw := objects[Name(name)]
			if ref, ok := d.finalIndirectRef(raw); ok {
				if visited[ref] {
					continue
				}
				visited[ref] = true
			}
			value, ok := d.resolveIndirectChain(raw)
			if !ok {
				return yield(FontResource{}, fmt.Errorf("playa: nested %s resource %q could not be resolved", category, name))
			}
			var dict Dict
			switch value := value.(type) {
			case Dict:
				dict = value
			case Stream:
				dict = value.DictBorrowed()
			default:
				continue
			}
			nestedValue, resolved := d.resolveIndirectChain(dict[Name("Resources")])
			if dict[Name("Resources")] != nil && !resolved {
				return yield(FontResource{}, fmt.Errorf("playa: nested %s resource %q Resources could not be resolved", category, name))
			}
			if nested, ok := nestedValue.(Dict); ok && !d.walkNestedFontResources(nested, seen, visited, yield) {
				return false
			}
		}
	}
	return true
}

func (d *Document) collectFontNames(res Dict, depth int, path string, out *[]string) error {
	if depth >= 32 {
		return nil
	}
	if raw, present := res[Name("Font")]; present {
		resolvedFonts, resolvedOK := d.resolveIndirectChain(raw)
		if !resolvedOK {
			return fmt.Errorf("playa: Form Font resources at %q could not be resolved", path)
		}
		fonts, ok := resolvedFonts.(Dict)
		if !ok {
			return fmt.Errorf("playa: Form Font resources at %q are not a dictionary", path)
		}
		names := make([]string, 0, len(fonts))
		for name := range fonts {
			resolved, resolvedOK := d.resolveIndirectChain(fonts[name])
			if !resolvedOK {
				return fmt.Errorf("playa: Form font resource %q%s could not be resolved", path, name)
			}
			if _, ok := resolved.(Dict); !ok {
				return fmt.Errorf("playa: Form font resource %q%s is not a dictionary", path, name)
			}
			names = append(names, string(name))
		}
		sort.Strings(names)
		for _, name := range names {
			*out = append(*out, path+name)
		}
	}
	var xos Dict
	if raw, present := res[Name("XObject")]; present {
		var ok bool
		resolvedXObjects, resolvedOK := d.resolveIndirectChain(raw)
		if !resolvedOK {
			return fmt.Errorf("playa: Form XObject resources at %q could not be resolved", path)
		}
		xos, ok = resolvedXObjects.(Dict)
		if !ok {
			return fmt.Errorf("playa: Form XObject resources at %q are not a dictionary", path)
		}
	}
	names := make([]string, 0, len(xos))
	for name := range xos {
		names = append(names, string(name))
	}
	sort.Strings(names)
	for _, rawName := range names {
		streamValue, streamResolved := d.resolveIndirectChain(xos[Name(rawName)])
		if !streamResolved {
			return fmt.Errorf("playa: Form XObject %q could not be resolved", path+rawName)
		}
		stream, ok := streamValue.(Stream)
		if !ok {
			continue
		}
		streamDict := stream.DictBorrowed()
		subtypeValue, subtypeResolved := d.resolveIndirectChain(streamDict[Name("Subtype")])
		if streamDict[Name("Subtype")] != nil && !subtypeResolved {
			return fmt.Errorf("playa: Form XObject %q subtype could not be resolved", path+rawName)
		}
		if subtypeValue != Name("Form") {
			continue
		}
		formResources := Dict{}
		if raw, present := streamDict[Name("Resources")]; present {
			var ok bool
			resolvedResources, resolvedOK := d.resolveIndirectChain(raw)
			if !resolvedOK {
				return fmt.Errorf("playa: Form XObject %q resources could not be resolved", path+rawName)
			}
			formResources, ok = resolvedResources.(Dict)
			if !ok {
				return fmt.Errorf("playa: Form XObject %q resources are not a dictionary", path+rawName)
			}
		}
		if err := d.collectFontNames(formResources, depth+1, path+rawName+"/", out); err != nil {
			return err
		}
	}
	return nil
}

func (d *Document) pageFontsExpanded(p Page) (map[string]*Font, error) {
	resources, err := d.pageFontResources(p)
	if err != nil {
		return nil, err
	}
	// Content interpretation only reads fonts. Reuse the document-owned font
	// cache here; PageFonts remains the public snapshot boundary and continues
	// to return deep copies.
	var out map[string]*Font
	if p.hasPageCacheKey() {
		if cached, ok := d.cachedPageFonts(p.ref); ok {
			out = cached
		}
	}
	if out == nil {
		out, err = d.pageFontsDirect(p)
		if err != nil {
			return nil, err
		}
		if p.hasPageCacheKey() {
			d.storePageFonts(p.ref, out)
		}
	}
	names := []string{}
	if err := d.collectFontNames(resources, 0, "", &names); err != nil {
		return nil, err
	}
	for _, name := range names {
		if _, ok := out[name]; ok {
			continue
		}
		if font := d.pageFontByName(p, name); font != nil {
			out[name] = font
		}
	}
	return out, nil
}

func setFontMetrics(f *Font, desc Dict) {
	setFontMetricsWithResolver(f, desc, func(value Object) Object { return value })
}

func setFontMetricsResolved(d *Document, f *Font, desc Dict) {
	setFontMetricsWithResolver(f, desc, d.resolveIndirectValue)
}

func setFontMetricsWithResolver(f *Font, desc Dict, resolve func(Object) Object) {
	if n, ok := finiteNumberValue(resolve(desc[Name("Ascent")])); ok {
		f.ascent = n
	}
	if n, ok := finiteNumberValue(resolve(desc[Name("Descent")])); ok {
		f.descent = n
	}
	if n, ok := finiteNumberValue(resolve(desc[Name("Leading")])); ok {
		f.leading = n
	}
	if bbox, ok := resolve(desc[Name("FontBBox")]).(Array); ok && len(bbox) == 4 {
		var values [4]float64
		valid := true
		for i := range values {
			values[i], valid = finiteNumberValue(resolve(bbox[i]))
			if !valid {
				break
			}
		}
		if valid {
			if values[0] > values[2] {
				values[0], values[2] = values[2], values[0]
			}
			if values[1] > values[3] {
				values[1], values[3] = values[3], values[1]
			}
			f.fontBBox, f.hasFontBBox = values, true
		}
	}
	if n, ok := finiteNumberValue(resolve(desc[Name("Flags")])); ok {
		f.flags, f.hasFlags = int(n), true
	}
	if n, ok := finiteNumberValue(resolve(desc[Name("CapHeight")])); ok {
		f.capHeight = n
	}
	if n, ok := finiteNumberValue(resolve(desc[Name("ItalicAngle")])); ok {
		f.italicAngle = n
	}
	if n, ok := finiteNumberValue(resolve(desc[Name("StemV")])); ok {
		f.stemV = n
	}
	// PDF specifies a negative descent. Some producers emit the absolute
	// value instead; Playa normalizes that form before layout calculations.
	if f.descent > 0 {
		f.descent = -f.descent
	}
	if f.ascent == 0 && f.descent == 0 && f.hasFontBBox {
		f.descent = f.fontBBox[1]
		f.ascent = f.fontBBox[3]
	}
	if n, ok := finiteNumberValue(resolve(desc[Name("MissingWidth")])); ok && (f.defaultWidth == 500 || f.defaultWidth == 1000) {
		f.defaultWidth = n
	}
}

// resolveFontBBox fills the descriptor bbox when the producer stored the
// array indirectly. The common direct-array path remains in setFontMetrics.
func resolveFontBBox(d *Document, f *Font, desc Dict) {
	if f.hasFontBBox {
		return
	}
	bboxValue, _ := d.resolveIndirectChain(desc[Name("FontBBox")])
	bbox, ok := bboxValue.(Array)
	if !ok || len(bbox) != 4 {
		return
	}
	var values [4]float64
	for i := range values {
		var valid bool
		item, _ := d.resolveIndirectChain(bbox[i])
		values[i], valid = finiteNumberValue(item)
		if !valid {
			return
		}
	}
	if values[0] > values[2] {
		values[0], values[2] = values[2], values[0]
	}
	if values[1] > values[3] {
		values[1], values[3] = values[3], values[1]
	}
	f.fontBBox, f.hasFontBBox = values, true
	if f.ascent == 0 && f.descent == 0 {
		f.descent, f.ascent = values[1], values[3]
	}
}

// parseCIDWidths follows Playa's width-array traversal: the W array itself is
// resolved by the caller, but indirect entries inside it are not dereferenced.
// This matters for compatibility with PDFs that store a width array behind an
// indirect entry; Playa treats those entries as absent and falls back to DW.
func parseCIDWidths(_ *Document, w Array, f *Font) {
	pending := map[int]float64{}
	for i := 0; i < len(w); {
		start, ok := IntValue(w[i])
		if !ok || start < 0 || start > 0xffff || i+1 >= len(w) {
			return
		}
		i++
		if vals, ok := w[i].(Array); ok {
			for j, v := range vals {
				if start+j > 0xffff {
					return
				}
				width, err := ParseWidth(v)
				if err != nil {
					return
				}
				pending[start+j] = width
			}
			i++
			continue
		}
		end, ok := IntValue(w[i])
		if !ok || end < start || end > 0xffff || i+1 >= len(w) {
			return
		}
		width, err := ParseWidth(w[i+1])
		if err != nil {
			return
		}
		for cid := start; cid <= end; cid++ {
			pending[cid] = width
		}
		i += 2
	}
	if f.cidWidths == nil {
		f.cidWidths = map[int]float64{}
	}
	for cid, width := range pending {
		f.cidWidths[cid] = width
	}
}

func parseCIDVerticalWidths(d *Document, w Array, f *Font) {
	pendingWidths := map[int]float64{}
	pendingPositions := map[int][2]float64{}
	for i := 0; i < len(w); {
		startValue, _ := d.resolveIndirectChain(w[i])
		start, ok := IntValue(startValue)
		if !ok || start < 0 || start > 0xffff || i+1 >= len(w) {
			return
		}
		i++
		valuesValue, _ := d.resolveIndirectChain(w[i])
		if values, ok := valuesValue.(Array); ok {
			if len(values)%3 != 0 {
				return
			}
			for j := 0; j < len(values); j += 3 {
				if start+j/3 > 0xffff {
					return
				}
				widthValue, _ := d.resolveIndirectChain(values[j])
				vxValue, _ := d.resolveIndirectChain(values[j+1])
				vyValue, _ := d.resolveIndirectChain(values[j+2])
				width, okw := finiteNumberValue(widthValue)
				vx, okx := finiteNumberValue(vxValue)
				vy, oky := finiteNumberValue(vyValue)
				if !okw || !okx || !oky {
					return
				}
				cid := start + j/3
				pendingWidths[cid] = width
				pendingPositions[cid] = [2]float64{vx, vy}
			}
			i++
			continue
		}
		endValue, _ := d.resolveIndirectChain(w[i])
		end, ok := IntValue(endValue)
		if !ok || end < start || end > 0xffff || i+3 >= len(w) {
			return
		}
		widthValue, _ := d.resolveIndirectChain(w[i+1])
		vxValue, _ := d.resolveIndirectChain(w[i+2])
		vyValue, _ := d.resolveIndirectChain(w[i+3])
		width, okw := finiteNumberValue(widthValue)
		vx, okx := finiteNumberValue(vxValue)
		vy, oky := finiteNumberValue(vyValue)
		if !okw || !okx || !oky {
			return
		}
		for cid := start; cid <= end; cid++ {
			pendingWidths[cid] = width
			pendingPositions[cid] = [2]float64{vx, vy}
		}
		i += 4
	}
	if f.verticalWidths == nil {
		f.verticalWidths = map[int]float64{}
	}
	if f.verticalPositions == nil {
		f.verticalPositions = map[int][2]float64{}
	}
	for cid, width := range pendingWidths {
		f.verticalWidths[cid] = width
		f.verticalPositions[cid] = pendingPositions[cid]
	}
}
func (d *Document) PageText(p Page) ([]TextObject, error) {
	b, e := p.Content(d)
	if e != nil {
		return nil, e
	}
	ops, e := ParseContent(b)
	if e != nil {
		return nil, e
	}
	fonts, e := d.pageFontsDirect(p)
	if e != nil {
		return nil, fmt.Errorf("playa: page fonts: %w", e)
	}
	ops = pageMatrixOps(p, d, ops)
	var resources Dict
	if cached, ok := d.cachedPageResource(p.ref); p.hasPageCacheKey() && ok {
		resources = cached
	} else {
		resourcesValue, _ := d.resolveIndirectChain(p.dict[Name("Resources")])
		resources, _ = resourcesValue.(Dict)
	}
	for i := range ops {
		ops[i].resources = resources
	}
	properties, err := d.pagePropertiesChecked(p)
	if err != nil {
		return nil, err
	}
	ops = d.expandContentPropertiesWithResources(ops, properties)
	return materializeTextWithDocument(d, p.ref, ops, fonts), nil
}

func (d *Document) pageProperties(p Page) Dict {
	properties, _ := d.pagePropertiesChecked(p)
	return properties
}

func (d *Document) pagePropertiesChecked(p Page) (Dict, error) {
	if p.hasPageCacheKey() {
		if err, ok := d.cachedPagePropertiesError(p.ref); ok {
			return nil, err
		}
		if properties, ok := d.cachedPageProperties(p.ref); ok {
			return properties, nil
		}
	}
	cacheError := func(err error) (Dict, error) {
		if p.hasPageCacheKey() {
			err = d.storePagePropertiesError(p.ref, err)
		}
		return nil, err
	}
	var resources Dict
	if cached, ok := d.cachedPageResource(p.ref); p.hasPageCacheKey() && ok {
		resources = cached
	} else if resRaw, present := p.dict[Name("Resources")]; present {
		resolvedResources, resourcesResolved := d.resolveIndirectChain(resRaw)
		if !resourcesResolved {
			return cacheError(fmt.Errorf("playa: Resources could not be resolved"))
		}
		var ok bool
		resources, ok = resolvedResources.(Dict)
		if !ok {
			return cacheError(fmt.Errorf("playa: Resources is not a dictionary"))
		}
	}
	if raw, present := resources[Name("Properties")]; present {
		resolvedProperties, propertiesResolved := d.resolveIndirectChain(raw)
		if !propertiesResolved {
			return cacheError(fmt.Errorf("playa: Properties resources could not be resolved"))
		}
		properties, ok := resolvedProperties.(Dict)
		if !ok {
			return cacheError(fmt.Errorf("playa: Properties resources are not a dictionary"))
		}
		for name, value := range properties {
			resolved, resolvedOK := d.resolveIndirectChain(value)
			if !resolvedOK {
				return cacheError(fmt.Errorf("playa: property resource %q could not be resolved", name))
			}
			if _, ok := markedPropertiesDict(resolved); !ok {
				return cacheError(fmt.Errorf("playa: property resource %q is not a dictionary", name))
			}
		}
	}
	propertiesValue, _ := d.resolveIndirectChain(resources[Name("Properties")])
	raw, _ := propertiesValue.(Dict)
	properties := Dict{}
	for name, value := range raw {
		resolvedValue, _ := d.resolveIndirectChain(value)
		if resolved, ok := markedPropertiesDict(resolvedValue); ok {
			properties[name] = resolveTagProperties(d, resolved)
		}
	}
	if p.hasPageCacheKey() {
		d.storePageProperties(p.ref, properties)
	}
	return properties, nil
}
