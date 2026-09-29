package document

import (
	"fmt"

	"github.com/lin-string/go-playa/documentdata"
)

const metadataXMLCacheLimit = 32 << 20
const metadataCacheLimit = 32 << 20

type Metadata = documentdata.Metadata

func cloneMetadata(source Metadata) Metadata { return documentdata.CloneMetadata(source) }

func (d *Document) Metadata() Metadata {
	metadata, err := d.MetadataWithError()
	if err != nil {
		return Metadata{}
	}
	return metadata
}

// MetadataWithError returns decoded common Info fields and reports a
// malformed explicit trailer /Info value. Unsupported individual values
// retain the compatibility behavior of being omitted.
func (d *Document) MetadataWithError() (Metadata, error) {
	d.metadataMu.Lock()
	defer d.metadataMu.Unlock()
	if d.metadataReady && d.metadataErr != nil {
		return nil, d.metadataErr
	}
	if d.metadataReady && d.metadataCacheable {
		return documentdata.CloneMetadata(d.metadataCache), nil
	}
	cacheError := func(err error) (Metadata, error) {
		d.metadataReady = true
		d.metadataCacheable = true
		d.metadataCache = nil
		d.metadataErr = err
		return nil, err
	}
	out := Metadata{}
	bytes := 0
	rawInfo, present := d.trailer[Name("Info")]
	if present {
		infoValue, ok := d.resolveIndirectChain(rawInfo)
		if !ok {
			return cacheError(fmt.Errorf("playa: document Info is not a dictionary"))
		}
		v, ok := infoValue.(Dict)
		if !ok {
			return cacheError(fmt.Errorf("playa: document Info is not a dictionary"))
		}
		for k, x := range v {
			value, valid := "", false
			resolved, resolvedOK := d.resolveIndirectChain(x)
			if !resolvedOK {
				return cacheError(fmt.Errorf("playa: document Info field %q could not be resolved", k))
			}
			switch z := resolved.(type) {
			case String:
				value, valid = decodePDFText(z), true
			case Name:
				value, valid = string(z), true
			case Number:
				if finite, ok := finiteNumberValue(z); ok {
					value, valid = documentdata.FormatNumber(finite), true
				}
			}
			if valid {
				bytes = addCacheSize(bytes, addCacheSize(len(k), len(value)))
				out[string(k)] = value
			}
		}
	}
	d.metadataReady = true
	d.metadataCacheable = bytes <= d.cacheLimits().MetadataBytes
	d.metadataErr = nil
	if d.metadataCacheable {
		d.metadataCache = out
	} else {
		d.metadataCache = nil
	}
	return documentdata.CloneMetadata(out), nil
}

// MetadataXML returns the decoded XMP metadata stream from the document
// catalog. A PDF without catalog metadata returns a nil byte slice and nil
// error.
func (d *Document) MetadataXML() ([]byte, error) {
	d.metadataXMLMu.Lock()
	defer d.metadataXMLMu.Unlock()
	if d.metadataXMLReady && d.metadataXMLCacheable {
		data, err := cloneObjectBytes(d.metadataXMLCache), d.metadataXMLErr
		return data, err
	}
	// Keep the per-document lock through decoding so concurrent callers share
	// one lazy parse instead of repeating the same stream work.
	data, err := d.decodeMetadataXML()
	d.metadataXMLReady = true
	if err != nil {
		d.metadataXMLErr = err
		d.metadataXMLCacheable = true
		return nil, err
	}
	if len(data) <= d.cacheLimits().MetadataXMLBytes {
		d.metadataXMLCache = cloneObjectBytes(data)
		d.metadataXMLCacheable = true
		return cloneObjectBytes(data), nil
	}
	// Keep oversized XMP out of the document cache; the current caller still
	// receives its result, while a later call decodes it again on demand.
	d.metadataXMLCache = nil
	d.metadataXMLCacheable = false
	return data, nil
}

func (d *Document) decodeMetadataXML() ([]byte, error) {
	catalog, err := d.catalogWithError()
	if err != nil {
		return nil, err
	}
	if catalog == nil {
		return nil, nil
	}
	rawMetadata, present := catalog[Name("Metadata")]
	if !present {
		return nil, nil
	}
	streamValue, ok := d.resolveIndirectChain(rawMetadata)
	if !ok {
		return nil, fmt.Errorf("playa: catalog Metadata could not be resolved")
	}
	stream, ok := streamValue.(Stream)
	if !ok {
		return nil, fmt.Errorf("playa: catalog Metadata is not a stream")
	}
	streamDict := stream.DictBorrowed()
	// Type and Subtype are required for a conforming metadata stream when
	// present. Keep accepting omitted hints for compatibility with older
	// PDFs, but never decode an explicitly non-XML stream as XMP.
	if typ, present := streamDict[Name("Type")]; present {
		value, resolved := d.resolveIndirectChain(typ)
		if value, ok := value.(Name); !resolved || !ok || value != Name("Metadata") {
			return nil, nil
		}
	}
	if subtype, present := streamDict[Name("Subtype")]; present {
		value, resolved := d.resolveIndirectChain(subtype)
		if value, ok := value.(Name); !resolved || !ok || value != Name("XML") {
			return nil, nil
		}
	}
	filters, parms := streamFiltersWithResolver(streamDict, d.resolveRaw)
	data, err := decodeFiltersLimited(stream.DataBorrowed(), filters, parms, decodedFilterExpansionLimit)
	if err != nil {
		return nil, fmt.Errorf("playa: metadata XML: %w", err)
	}
	return data, nil
}
