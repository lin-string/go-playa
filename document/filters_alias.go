package document

import (
	pdfparser "github.com/lin-string/go-playa/parser"
	pdftypes "github.com/lin-string/go-playa/pdftypes/primitives"
)

const decodedFilterExpansionLimit = 256 << 20

func DecodeFilters(data []byte, filters []string, parms []pdftypes.Dict) ([]byte, error) {
	return pdfparser.DecodeFilters(data, filters, parms)
}

func decodeFiltersLimited(data []byte, filters []string, parms []Dict, limit int) ([]byte, error) {
	return pdfparser.DecodeFiltersLimited(data, filters, parms, limit)
}

func decodeFiltersLenientLimited(data []byte, filters []string, parms []Dict, limit int) ([]byte, error) {
	return pdfparser.DecodeFiltersLenientLimited(data, filters, parms, limit)
}

func streamFilters(d Dict) ([]string, []Dict) { return pdfparser.StreamFilters(d) }

func streamFiltersWithResolver(d Dict, resolve func(Object) Object) ([]string, []Dict) {
	return pdfparser.StreamFiltersWithResolver(d, resolve)
}
