package fontdata

// CFFDictEntry is one completed CFF/CFF2 DICT operator and its operands.
// The operand slice is private so a parsed DICT can be reused safely by font
// resource code.
type CFFDictEntry struct {
	operator int
	operands []float64
}

// Operator returns the normalized CFF operator number. Escaped operators use
// the 1200-plus-byte form shared by ParseCFFOperator.
func (entry CFFDictEntry) Operator() int { return entry.operator }

// OperandsCopy returns an independent copy of the operator operands.
func (entry CFFDictEntry) OperandsCopy() []float64 {
	if entry.operands == nil {
		return nil
	}
	return append([]float64(nil), entry.operands...)
}

// ParseCFFDict parses completed CFF/CFF2 DICT records. If the input ends in
// a malformed operator or an unterminated operand list, completed records
// before that point are returned with ok=false. This permits document-level
// recovery to retain the same prefix semantics as Playa.
func ParseCFFDict(data []byte, cff2 bool) ([]CFFDictEntry, bool) {
	entries := make([]CFFDictEntry, 0, 8)
	operands := make([]float64, 0, 8)
	for at := 0; at < len(data); {
		if value, next, ok := ParseCFFNumber(data, at); ok {
			operands = append(operands, value)
			if len(operands) > 513 {
				return entries, false
			}
			at = next
			continue
		}
		op, next, ok := ParseCFFOperator(data, at, cff2)
		if !ok {
			return entries, false
		}
		entries = append(entries, CFFDictEntry{operator: op, operands: append([]float64(nil), operands...)})
		operands = operands[:0]
		at = next
	}
	if len(operands) != 0 {
		return entries, false
	}
	return entries, true
}
