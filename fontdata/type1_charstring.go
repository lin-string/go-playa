package fontdata

import (
	"encoding/binary"
	"math"
)

// Type1PathOp is a dependency-free outline operation produced from a Type 1
// CharString. Operands are coordinates in the CharString's font space.
type Type1PathOp struct {
	operator string
	operands []float64
}

// Operator returns the PDF-style path operator name.
func (op Type1PathOp) Operator() string { return op.operator }

// OperandsCopy returns an owned copy of the operation operands.
func (op Type1PathOp) OperandsCopy() []float64 { return append([]float64(nil), op.operands...) }

// ParseType1CharString converts the drawing subset of a Type 1 CharString
// into path operations. The input must already be decrypted and lenIV
// stripped. Adobe Type 1's ten-level Subr recursion limit is enforced.
func ParseType1CharString(data []byte, subrs [][]byte) ([]Type1PathOp, bool) {
	return ParseType1CharStringWithSeac(data, subrs, nil)
}

// ParseType1CharStringWithSeac is ParseType1CharString with a callback for
// resolving the base and accent CharStrings used by the seac operator.
func ParseType1CharStringWithSeac(data []byte, subrs [][]byte, resolve func(byte) []byte) ([]Type1PathOp, bool) {
	state := type1CharStringState{subrs: subrs, seac: resolve}
	if !state.run(data, 0) || len(state.operands) != 0 || !state.finite || state.flexActive || state.flexSetPoint || state.otherSubrResults != 0 {
		return nil, false
	}
	if state.drawn {
		state.ops = append(state.ops, Type1PathOp{operator: "h"}, Type1PathOp{operator: "f"})
	}
	return state.ops, true
}

const type1CharStringMaxDepth = 10

type type1CharStringState struct {
	ops              []Type1PathOp
	operands         []float64
	subrs            [][]byte
	x, y             float64
	drawn            bool
	finite           bool
	seac             func(byte) []byte
	flexActive       bool
	flexPoints       [][2]float64
	flexSetPoint     bool
	otherSubrResults int
}

func (s *type1CharStringState) run(data []byte, depth int) bool {
	if depth > type1CharStringMaxDepth {
		return false
	}
	s.finite = true
	for at := 0; at < len(data); {
		value := data[at]
		at++
		if value >= 32 {
			number, next, ok := type1CharStringNumber(data, at-1)
			if !ok {
				return false
			}
			s.operands = append(s.operands, number)
			at = next
			continue
		}
		op := int(value)
		if op == 12 {
			if at >= len(data) {
				return false
			}
			op = 1200 + int(data[at])
			at++
		}
		if !s.operator(op, depth) {
			return false
		}
		if op == 14 {
			return depth == 0
		}
		if op == 11 {
			return depth > 0
		}
	}
	return false
}

func (s *type1CharStringState) operator(op, depth int) bool {
	args := s.operands
	s.operands = nil
	if !s.finiteArgs(args) {
		return false
	}
	line := func(dx, dy float64) {
		s.x += dx
		s.y += dy
		s.ops = append(s.ops, type1PathOp("l", s.x, s.y))
		s.drawn = true
	}
	curve := func(dx1, dy1, dx2, dy2, dx3, dy3 float64) {
		c1x, c1y := s.x+dx1, s.y+dy1
		c2x, c2y := c1x+dx2, c1y+dy2
		s.x, s.y = c2x+dx3, c2y+dy3
		if math.IsNaN(s.x) || math.IsInf(s.x, 0) || math.IsNaN(s.y) || math.IsInf(s.y, 0) {
			s.finite = false
			return
		}
		s.ops = append(s.ops, Type1PathOp{operator: "c", operands: []float64{c1x, c1y, c2x, c2y, s.x, s.y}})
		s.drawn = true
	}
	move := func(dx, dy float64) {
		s.x += dx
		s.y += dy
		if s.flexActive {
			return
		}
		s.ops = append(s.ops, type1PathOp("m", s.x, s.y))
		s.drawn = true
	}
	switch op {
	case 1, 3, 10, 13:
		// Stem hints and hsbw only affect metrics, not the outline here.
		if op == 10 {
			if len(args) != 1 || len(s.subrs) == 0 {
				return len(args) == 1 && len(s.subrs) == 0
			}
			index, ok := type1Integer(args[0])
			if !ok || index < 0 || index >= len(s.subrs) {
				return false
			}
			return s.run(s.subrs[index], depth+1)
		}
		if op == 13 && len(args) != 2 || op != 13 && len(args)%2 != 0 {
			return false
		}
	case 1200: // dotsection
		if len(args) != 0 {
			return false
		}
	case 1201, 1202: // vstem3 and hstem3
		if len(args) != 6 {
			return false
		}
	case 4:
		if len(args) != 1 {
			return false
		}
		move(0, args[0])
	case 5:
		if len(args) < 2 || len(args)%2 != 0 {
			return false
		}
		for i := 0; i < len(args); i += 2 {
			line(args[i], args[i+1])
		}
	case 6:
		if len(args) == 0 {
			return false
		}
		for i, value := range args {
			if i%2 == 0 {
				line(value, 0)
			} else {
				line(0, value)
			}
		}
	case 7:
		if len(args) == 0 {
			return false
		}
		for i, value := range args {
			if i%2 == 0 {
				line(0, value)
			} else {
				line(value, 0)
			}
		}
	case 8:
		if len(args) == 0 || len(args)%6 != 0 {
			return false
		}
		for i := 0; i < len(args); i += 6 {
			curve(args[i], args[i+1], args[i+2], args[i+3], args[i+4], args[i+5])
		}
	case 11:
		return depth > 0
	case 14:
		return len(args) == 0
	case 21:
		if len(args) != 2 {
			return false
		}
		move(args[0], args[1])
	case 22:
		if len(args) != 1 {
			return false
		}
		move(args[0], 0)
	case 1207: // sbw
		return len(args) == 4
	case 1212: // div
		if len(args) < 2 || args[len(args)-1] == 0 {
			return false
		}
		s.operands = append(args[:len(args)-2], args[len(args)-2]/args[len(args)-1])
	case 1233: // setcurrentpoint
		if len(args) == 0 && s.flexSetPoint {
			s.flexSetPoint = false
			return true
		}
		if len(args) != 2 {
			return false
		}
		s.x, s.y = args[0], args[1]
	case 1216: // callothersubr
		if len(args) < 2 {
			return false
		}
		argCount, argOK := type1Integer(args[len(args)-2])
		subrNumber, subrOK := type1Integer(args[len(args)-1])
		if !argOK || !subrOK || argCount < 0 || argCount != len(args)-2 {
			return false
		}
		switch subrNumber {
		case 0: // finish Flex
			if argCount != 3 || !s.flexActive || len(s.flexPoints) != 7 {
				return false
			}
			p := s.flexPoints
			s.ops = append(s.ops,
				Type1PathOp{operator: "c", operands: []float64{p[1][0], p[1][1], p[2][0], p[2][1], p[3][0], p[3][1]}},
				Type1PathOp{operator: "c", operands: []float64{p[4][0], p[4][1], p[5][0], p[5][1], p[6][0], p[6][1]}},
			)
			s.flexActive = false
			s.flexPoints = nil
			s.flexSetPoint = true
			s.otherSubrResults = 2
			s.drawn = true
		case 1: // start Flex
			if argCount != 0 || s.flexActive {
				return false
			}
			s.flexActive = true
			s.flexPoints = [][2]float64{{s.x, s.y}}
		case 2: // add current point to Flex
			if argCount != 0 || !s.flexActive || len(s.flexPoints) >= 7 {
				return false
			}
			s.flexPoints = append(s.flexPoints, [2]float64{s.x, s.y})
		case 3: // hint replacement
			if argCount != 1 {
				return false
			}
			s.otherSubrResults = 1
		case 12, 13: // Counter Control hints
		default:
			return false
		}
	case 1217: // pop
		if len(args) != 0 || s.otherSubrResults == 0 {
			return false
		}
		s.otherSubrResults--
	case 1234: // hflex
		if len(args) != 7 {
			return false
		}
		curve(args[0], 0, args[1], args[2], args[3], 0)
		curve(args[4], 0, args[5], -args[2], args[6], 0)
	case 1235: // flex
		if len(args) != 13 {
			return false
		}
		curve(args[0], args[1], args[2], args[3], args[4], args[5])
		curve(args[6], args[7], args[8], args[9], args[10], args[11])
	case 1236: // hflex1
		if len(args) != 9 {
			return false
		}
		curve(args[0], args[1], args[2], args[3], args[4], 0)
		curve(args[5], 0, args[6], args[7], args[8], -(args[1] + args[3] + args[7]))
	case 1237: // flex1
		if len(args) != 11 {
			return false
		}
		curve(args[0], args[1], args[2], args[3], args[4], args[5])
		dx := args[0] + args[2] + args[4] + args[6] + args[8]
		dy := args[1] + args[3] + args[5] + args[7] + args[9]
		if math.Abs(dx) > math.Abs(dy) {
			curve(args[6], args[7], args[8], args[9], args[10], -dy)
		} else {
			curve(args[6], args[7], args[8], args[9], -dx, args[10])
		}
	case 1206: // seac
		if len(args) != 5 || s.seac == nil || args[3] < 0 || args[3] > 255 || args[4] < 0 || args[4] > 255 || args[3] != math.Trunc(args[3]) || args[4] != math.Trunc(args[4]) {
			return false
		}
		base, accent := s.seac(byte(args[3])), s.seac(byte(args[4]))
		if len(base) == 0 || len(accent) == 0 {
			return false
		}
		baseState := type1CharStringState{subrs: s.subrs, finite: true}
		accentState := type1CharStringState{subrs: s.subrs, finite: true}
		if !baseState.run(base, 0) || !accentState.run(accent, 0) || len(baseState.operands) != 0 || len(accentState.operands) != 0 || baseState.flexActive || accentState.flexActive || baseState.flexSetPoint || accentState.flexSetPoint || baseState.otherSubrResults != 0 || accentState.otherSubrResults != 0 {
			return false
		}
		accentOps, ok := translateType1Ops(accentState.ops, args[1]-args[0], args[2])
		if !ok {
			return false
		}
		s.ops = append(s.ops, baseState.ops...)
		s.ops = append(s.ops, accentOps...)
		s.drawn = baseState.drawn || accentState.drawn
	default:
		return false
	}
	return true
}

func translateType1Ops(ops []Type1PathOp, dx, dy float64) ([]Type1PathOp, bool) {
	result := make([]Type1PathOp, 0, len(ops))
	for _, source := range ops {
		values := append([]float64(nil), source.operands...)
		if source.operator == "m" || source.operator == "l" || source.operator == "c" {
			if len(values)%2 != 0 {
				return nil, false
			}
			for i := range values {
				if i%2 == 0 {
					values[i] += dx
				} else {
					values[i] += dy
				}
			}
		}
		result = append(result, Type1PathOp{operator: source.operator, operands: values})
	}
	return result, true
}

func (s *type1CharStringState) finiteArgs(args []float64) bool {
	for _, value := range args {
		if math.IsNaN(value) || math.IsInf(value, 0) {
			s.finite = false
			return false
		}
	}
	return true
}

func type1Integer(value float64) (int, bool) {
	if math.IsNaN(value) || math.IsInf(value, 0) || value != math.Trunc(value) || value < math.MinInt32 || value > math.MaxInt32 {
		return 0, false
	}
	return int(value), true
}

func type1PathOp(operator string, x, y float64) Type1PathOp {
	return Type1PathOp{operator: operator, operands: []float64{x, y}}
}

func type1CharStringNumber(data []byte, at int) (float64, int, bool) {
	if at >= len(data) {
		return 0, at, false
	}
	b := data[at]
	switch {
	case b >= 32 && b <= 246:
		return float64(int(b) - 139), at + 1, true
	case b >= 247 && b <= 250:
		if at+1 >= len(data) {
			return 0, at, false
		}
		return float64((int(b)-247)*256 + int(data[at+1]) + 108), at + 2, true
	case b >= 251 && b <= 254:
		if at+1 >= len(data) {
			return 0, at, false
		}
		return float64(-(int(b)-251)*256 - int(data[at+1]) - 108), at + 2, true
	case b == 255:
		if at+4 >= len(data) {
			return 0, at, false
		}
		return float64(int32(binary.BigEndian.Uint32(data[at+1:at+5]))) / 65536, at + 5, true
	default:
		return 0, at, false
	}
}
