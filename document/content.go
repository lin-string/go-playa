package document

import (
	"errors"
	"fmt"
	"iter"
	"strings"

	"github.com/lin-string/go-playa/contentdata"
)

type ContentOp struct {
	data            contentdata.ContentOp
	resources       Dict
	formBoundary    uint8
	parentKey       int
	hasParentKey    bool
	propertyName    Name
	hasPropertyName bool
}

var errContentStreamIndirectReference = errors.New("playa: indirect object reference is not permitted in a content stream")

func newContentOpBorrowed(operator string, operands []Object, offset int) ContentOp {
	return ContentOp{data: contentdata.NewContentOpBorrowed(operator, operands, offset)}
}

func newContentOpWithContext(operator string, operands []Object, offset int, resources Dict, formBoundary uint8, propertyName Name, hasPropertyName bool) ContentOp {
	return ContentOp{
		data:            contentdata.NewContentOpBorrowed(operator, operands, offset),
		resources:       resources,
		formBoundary:    formBoundary,
		propertyName:    propertyName,
		hasPropertyName: hasPropertyName,
	}
}

func newContentOpWithResources(operator string, operands []Object, resources Dict) ContentOp {
	return newContentOpWithContext(operator, operands, 0, resources, 0, "", false)
}

func (op ContentOp) operatorValue() string { return op.data.Operator() }

func (op ContentOp) operandsValue() []Object { return op.data.OperandsBorrowed() }

func (op ContentOp) offsetValue() int { return op.data.Offset() }

func (op *ContentOp) setOperands(operands []Object) {
	if op == nil {
		return
	}
	op.data = contentdata.NewContentOpBorrowed(op.data.Operator(), operands, op.data.Offset())
}

// Operator returns the PDF content operator name.
func (op ContentOp) Operator() string { return op.data.Operator() }

// Offset returns the byte offset of the operator in its source stream.
func (op ContentOp) Offset() int { return op.data.Offset() }

// OperandsCopy returns independent operands for this content operation.
func (op ContentOp) OperandsCopy() []Object {
	return op.data.OperandsCopy()
}

// OperandsSeq lazily yields borrowed operands in source order. The yielded
// values are valid for read-only inspection during the iteration; use
// OperandsCopy or Finalize when an owned snapshot is required.
func (op ContentOp) OperandsSeq() iter.Seq[Object] {
	return op.data.OperandsSeq()
}

// Finalize returns an independent snapshot of the content operation.
func (op ContentOp) Finalize() ContentOp {
	op.data = op.data.Finalize()
	op.resources = cloneDict(op.resources)
	return op
}

func (op ContentOp) MarshalJSON() ([]byte, error) {
	return op.data.MarshalJSON()
}

const (
	formBegin uint8 = 1
	formEnd   uint8 = 2
)

// ParseContent follows PDF content-stream rules: operands accumulate until a
// keyword is encountered, at which point one operation is emitted. Inline
// images are handled as a dedicated operation so image data is not mistaken
// for PDF operators.
func ParseContent(data []byte) ([]ContentOp, error) {
	p := NewObjectParser(data)
	out := []ContentOp{}
	operands := []Object{}
	for {
		t, e := p.NextToken()
		if e != nil {
			if recoverTrailingContentParseError(e) {
				return out, nil
			}
			return nil, e
		}
		if t.Kind() == TokenEOF {
			// Playa's non-strict content parser discards operands left at the
			// end of a stream. Real-world producers occasionally leave such
			// recovery garbage behind; it must not make otherwise valid text
			// extraction fail.
			return out, nil
		}
		if t.Kind() == TokenKeyword && t.Text() != "{" {
			if t.Text() == "BI" {
				// Inline-image parameters continue until ID. The bytes after the
				// mandatory whitespace are opaque and must not be lexed.
				params, e := inlineImageParamsFromParser(p)
				if e != nil {
					if recoverTrailingContentParseError(e) {
						return out, nil
					}
					return nil, e
				}
				data := p.Lexer().DataBorrowed()
				start := inlineImageDataStart(data, p.Lexer().Pos(), params)
				end, resume := inlineImageEnd(data, start, params)
				if end < 0 {
					return nil, fmt.Errorf("playa: unterminated inline image")
				}
				imageData := cloneObjectBytes(p.Lexer().DataBorrowed()[start:end])
				// findInlineImageEnd returns the delimiter immediately before
				// the E in EI. Resume after both marker bytes, leaving any
				// following whitespace for the normal lexer.
				p.Lexer().SetPos(resume)
				out = append(out, newContentOpBorrowed("BI", []Object{newStreamWithDecodeDict(params, imageData, effectiveInlineImageParams(params))}, t.Offset()))
				operands = nil
				continue
			}
			out = append(out, newContentOpBorrowed(t.Text(), operands, t.Offset()))
			operands = nil
			continue
		}
		if t.Kind() == TokenArrayEnd || t.Kind() == TokenDictEnd {
			// Non-strict Playa parsing logs and skips unmatched closing
			// delimiters in content streams.
			continue
		}
		o, e := p.ParseToken(t)
		if e != nil {
			if recoverTrailingContentParseError(e) {
				return out, nil
			}
			return nil, e
		}
		if contentObjectHasIndirectReference(o) {
			return nil, errContentStreamIndirectReference
		}
		operands = append(operands, o)
	}
}

func contentObjectHasIndirectReference(object Object) bool {
	switch value := object.(type) {
	case Ref:
		return true
	case Array:
		for _, item := range value {
			if contentObjectHasIndirectReference(item) {
				return true
			}
		}
	case InvalidArray:
		for _, item := range value {
			if contentObjectHasIndirectReference(item) {
				return true
			}
		}
	case Dict:
		for _, item := range value {
			if contentObjectHasIndirectReference(item) {
				return true
			}
		}
	case Stream:
		return contentObjectHasIndirectReference(value.DictBorrowed())
	}
	return false
}

func recoverTrailingContentParseError(err error) bool {
	var parseErr *ParseError
	if !errors.As(err, &parseErr) || parseErr.Cause() == nil {
		return false
	}
	message := parseErr.Cause().Error()
	return strings.Contains(message, "unterminated array") || strings.Contains(message, "unterminated dictionary") || strings.Contains(message, "unterminated procedure") || strings.Contains(message, "unterminated PDF string")
}

func findInlineImageEnd(b []byte, start int) int {
	for i := start + 1; i+1 < len(b); i++ {
		if b[i] == 'E' && b[i+1] == 'I' && isContentWhitespace(b[i-1]) && (i+2 == len(b) || isContentWhitespace(b[i+2])) {
			return i - 1
		}
	}
	return -1
}

// inlineImageEnd prefers the explicit /Length (or /L) when it is present,
// but validates that the following bytes are the required EI marker. This
// avoids treating marker-like bytes inside compressed image data as the end.
func inlineImageEnd(b []byte, start int, params Dict) (end, resume int) {
	filter := inlineImageFinalFilter(params)
	lengthObject := params[Name("L")]
	if lengthObject == nil {
		lengthObject = params[Name("Length")]
	}
	if length, ok := IntValue(lengthObject); ok && length >= 0 && start <= len(b) && length <= len(b)-start {
		marker := start + length
		for marker < len(b) && isContentWhitespace(b[marker]) {
			marker++
		}
		if marker+2 <= len(b) && b[marker] == 'E' && b[marker+1] == 'I' && (marker+2 == len(b) || isContentWhitespace(b[marker+2])) {
			return start + length, marker + 2
		}
	}
	if filter == "ASCIIHexDecode" || filter == "AHx" {
		for i := start; i+1 < len(b); i++ {
			if b[i] == 'E' && b[i+1] == 'I' {
				return i, i + 2
			}
		}
	}
	if filter == "ASCII85Decode" || filter == "A85" {
		for i := start; i+1 < len(b); i++ {
			if b[i] != '~' {
				continue
			}
			marker := i + 1
			for marker < len(b) && isContentWhitespace(b[marker]) {
				marker++
			}
			if marker >= len(b) || b[marker] != '>' {
				continue
			}
			marker++
			for marker < len(b) && isContentWhitespace(b[marker]) {
				marker++
			}
			if marker+1 < len(b) && b[marker] == 'E' && b[marker+1] == 'I' {
				return i, marker + 2
			}
		}
	}
	end = findInlineImageEnd(b, start)
	if end < 0 {
		return -1, -1
	}
	return end, end + 3
}

func inlineImageFinalFilter(params Dict) string {
	filter := params[Name("F")]
	if filter == nil {
		filter = params[Name("Filter")]
	}
	if filters, ok := filter.(Array); ok {
		if len(filters) == 0 {
			return ""
		}
		filter = filters[0]
	}
	name, _ := filter.(Name)
	return string(name)
}

func inlineImageDataStart(data []byte, start int, params Dict) int {
	filter := inlineImageFinalFilter(params)
	for start < len(data) {
		if filter == "ASCIIHexDecode" || filter == "AHx" || filter == "ASCII85Decode" || filter == "A85" {
			if !isContentWhitespace(data[start]) {
				break
			}
		} else if data[start] != ' ' && data[start] != '\n' && data[start] != '\r' && data[start] != '\t' {
			break
		}
		start++
	}
	return start
}

func isContentWhitespace(c byte) bool {
	return c == ' ' || c == '\n' || c == '\r' || c == '\t' || c == '\f' || c == 0
}
