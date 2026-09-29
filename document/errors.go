package document

import pdfparser "github.com/lin-string/go-playa/parser"

type ParseError = pdfparser.ParseError

func wrapParseError(err error, offset int, operation string) error {
	if err == nil {
		return nil
	}
	return pdfparser.Wrap(err, offset, operation)
}

func withParseContext(err error, offset int, operation string) error {
	if err == nil {
		return nil
	}
	return pdfparser.WithContext(err, offset, operation)
}

func wrapObjectParseError(err error, ref Ref, offset int, operation string) error {
	if err == nil {
		return nil
	}
	return pdfparser.WithObjectContext(err, ref, offset, operation)
}
