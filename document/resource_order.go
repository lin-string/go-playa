package document

import "bytes"

const (
	fontOrderPathNone uint8 = iota
	fontOrderPathTop
	fontOrderPathFont
	fontOrderPathResourcesFont
)

func (d *Document) fontResourceOrderSource(node Object, resourcesValue Object) (Ref, uint8) {
	resources, ok := d.resolveIndirectChain(resourcesValue)
	resourcesDict, dictOK := resources.(Dict)
	if !ok || !dictOK {
		return Ref{}, fontOrderPathNone
	}
	if fontRef, ok := d.finalIndirectRef(resourcesDict[Name("Font")]); ok {
		return fontRef, fontOrderPathTop
	}
	if resourcesRef, ok := d.finalIndirectRef(resourcesValue); ok {
		return resourcesRef, fontOrderPathFont
	}
	if nodeRef, ok := d.finalIndirectRef(node); ok {
		return nodeRef, fontOrderPathResourcesFont
	}
	return Ref{}, fontOrderPathNone
}

func (d *Document) sourceFontResourceNames(page Page, fonts Dict) ([]string, bool) {
	var path []Name
	switch page.fontOrderPath {
	case fontOrderPathTop:
	case fontOrderPathFont:
		path = []Name{Name("Font")}
	case fontOrderPathResourcesFont:
		path = []Name{Name("Resources"), Name("Font")}
	default:
		return nil, false
	}
	names, ok := d.sourceDictionaryNames(page.fontOrderRef, path)
	if !ok || len(names) != len(fonts) {
		return nil, false
	}
	for _, name := range names {
		if _, present := fonts[Name(name)]; !present {
			return nil, false
		}
	}
	return names, true
}

func (d *Document) sourceDictionaryNames(ref Ref, path []Name) ([]string, bool) {
	if d == nil || ref == (Ref{}) || len(d.data) == 0 {
		return nil, false
	}
	d.cacheMu.RLock()
	entry, ok := d.xrefs[ref]
	if !ok {
		entry, ok = d.xrefHistory[ref]
	}
	d.cacheMu.RUnlock()
	if !ok || entry.isFree() || entry.isCompressed() || entry.fileOffset() <= 0 {
		return nil, false
	}
	objectOffset := xrefObjectOffset(d.data, entry.fileOffset())
	body, ok := indirectBody(d.data, objectOffset, d.resolveRaw)
	if !ok {
		return nil, false
	}
	header := bytes.Index(body, []byte("obj"))
	if header < 0 {
		return nil, false
	}
	p := NewObjectParser(bytes.TrimSpace(body[header+len("obj"):]))
	token, err := p.NextToken()
	if err != nil {
		return nil, false
	}
	return sourceDictionaryNamesAt(p, token, path)
}

func sourceDictionaryNamesAt(p *ObjectParser, token Token, path []Name) ([]string, bool) {
	if token.Kind() != TokenDictStart {
		return nil, false
	}
	if len(path) == 0 {
		var names []string
		seen := map[string]bool{}
		for {
			key, err := p.NextToken()
			if err != nil || key.Kind() == TokenEOF {
				return nil, false
			}
			if key.Kind() == TokenDictEnd {
				return names, true
			}
			if key.Kind() != TokenName {
				return nil, false
			}
			value, err := p.NextToken()
			if err != nil {
				return nil, false
			}
			if _, err := p.ParseToken(value); err != nil {
				return nil, false
			}
			if !seen[key.Text()] {
				seen[key.Text()] = true
				names = append(names, key.Text())
			}
		}
	}
	for {
		key, err := p.NextToken()
		if err != nil || key.Kind() == TokenEOF || key.Kind() == TokenDictEnd {
			return nil, false
		}
		if key.Kind() != TokenName {
			return nil, false
		}
		value, err := p.NextToken()
		if err != nil {
			return nil, false
		}
		if Name(key.Text()) == path[0] {
			return sourceDictionaryNamesAt(p, value, path[1:])
		}
		if _, err := p.ParseToken(value); err != nil {
			return nil, false
		}
	}
}
