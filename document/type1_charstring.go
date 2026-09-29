package document

import "github.com/lin-string/go-playa/fontdata"

// Keep the historical local constant for document tests; the interpreter
// itself now lives in fontdata and has no dependency on PDF document state.
const type1CharStringMaxDepth = 10

func type1CharStringOps(data []byte, subrs [][]byte) ([]ContentOp, bool) {
	return type1CharStringOpsWithSeac(data, subrs, nil)
}

func type1CharStringOpsWithSeac(data []byte, subrs [][]byte, seac func(byte) []byte) ([]ContentOp, bool) {
	ops, ok := fontdata.ParseType1CharStringWithSeac(data, subrs, seac)
	if !ok {
		return nil, false
	}
	result := make([]ContentOp, 0, len(ops))
	for _, source := range ops {
		values := source.OperandsCopy()
		operands := make([]Object, len(values))
		for i, value := range values {
			operands[i] = Number(value)
		}
		result = append(result, newContentOpBorrowed(source.Operator(), operands, 0))
	}
	return result, true
}
