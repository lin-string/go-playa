package document

import pdfparser "github.com/lin-string/go-playa/parser"

func decodePDFText(b []byte) string { return pdfparser.DecodePDFText(b) }
