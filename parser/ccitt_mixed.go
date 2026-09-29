package parser

import (
	"fmt"
)

type ccittBitReader struct {
	data []byte
	bit  int
}

func (r *ccittBitReader) readBit() (int, error) {
	if r.bit >= len(r.data)*8 {
		return 0, fmt.Errorf("playa: truncated CCITTFaxDecode mixed-mode data")
	}
	value := (r.data[r.bit/8] >> uint(7-r.bit%8)) & 1
	r.bit++
	return int(value), nil
}

func (r *ccittBitReader) align() {
	r.bit = (r.bit + 7) &^ 7
}

func ccittDecodeTable(r *ccittBitReader, table [][2]int16) (int, error) {
	node := 1
	for {
		bit, err := r.readBit()
		if err != nil {
			return 0, err
		}
		if node <= 0 || node >= len(table) {
			return 0, fmt.Errorf("playa: invalid CCITTFaxDecode code tree node")
		}
		next := table[node][bit]
		if next == 0 {
			return 0, fmt.Errorf("playa: invalid CCITTFaxDecode code")
		}
		if next < 0 {
			return ^int(next), nil
		}
		node = int(next)
	}
}

func ccittDecodeRun(r *ccittBitReader, table [][2]int16, width int) (int, error) {
	total := 0
	for {
		run, err := ccittDecodeTable(r, table)
		if err != nil {
			return 0, err
		}
		if run < 0 || run > width-total {
			return 0, fmt.Errorf("playa: CCITTFaxDecode run length exceeds row width")
		}
		total += run
		if run < 64 {
			return total, nil
		}
	}
}

func ccittFindB(line []byte, pos int, color byte, second bool) int {
	if len(line) == 0 {
		return 0
	}
	i := pos
	if i < 0 {
		i = 0
	}
	opposite := byte(0)
	if color == 0 {
		opposite = 0xff
	}
	if pos == 0 {
		for i < len(line) && line[i] == color {
			i++
		}
	} else {
		for i < len(line) && line[i] == opposite {
			i++
		}
		for i < len(line) && line[i] == color {
			i++
		}
	}
	if second {
		for i < len(line) && line[i] == opposite {
			i++
		}
	}
	return i
}

func ccittDecodeMixedRow(r *ccittBitReader, current, previous []byte, oneDim bool) error {
	for i := range current {
		current[i] = 0xff
	}
	pos := 0
	color := byte(0xff)
	if oneDim {
		for pos < len(current) {
			run, err := ccittDecodeRun(r, ccittTableForColor(color), len(current)-pos)
			if err != nil {
				return err
			}
			for i := pos; i < pos+run; i++ {
				current[i] = color
			}
			pos += run
			if color == 0 {
				color = 0xff
			} else {
				color = 0
			}
		}
		return nil
	}
	for pos < len(current) {
		mode, err := ccittDecodeTable(r, pdfparserCCITTModeDecodeTable[:])
		if err != nil {
			return err
		}
		if mode == 9 {
			// The upstream x/image table exposes the PDF extension prefix
			// (0000001), while Playa recognizes the longer uncompressed
			// extension (0000001111) and the seven reserved variants. Consume
			// the suffix here so the prefix cannot be mistaken for a complete
			// mode.
			suffix := 0
			for i := 0; i < 3; i++ {
				bit, readErr := r.readBit()
				if readErr != nil {
					return readErr
				}
				suffix = (suffix << 1) | bit
			}
			if suffix != 7 {
				return fmt.Errorf("playa: unsupported CCITTFaxDecode extension %d", suffix+1)
			}
			for pos < len(current) {
				bit, readErr := r.readBit()
				if readErr != nil {
					return readErr
				}
				if bit == 0 {
					current[pos] = 0
				} else {
					current[pos] = 0xff
				}
				pos++
			}
			continue
		}
		switch mode {
		case 0:
			b2 := ccittFindB(previous, pos, color, true)
			if b2 < pos || b2 > len(current) {
				return fmt.Errorf("playa: invalid CCITTFaxDecode pass offset")
			}
			for i := pos; i < b2; i++ {
				current[i] = color
			}
			pos = b2
		case 1:
			first, err := ccittDecodeRun(r, ccittTableForColor(color), len(current)-pos)
			if err != nil {
				return err
			}
			for i := pos; i < pos+first; i++ {
				current[i] = color
			}
			pos += first
			if color == 0 {
				color = 0xff
			} else {
				color = 0
			}
			second, err := ccittDecodeRun(r, ccittTableForColor(color), len(current)-pos)
			if err != nil {
				return err
			}
			for i := pos; i < pos+second; i++ {
				current[i] = color
			}
			pos += second
			if color == 0 {
				color = 0xff
			} else {
				color = 0
			}
		case 2, 3, 4, 5, 6, 7, 8:
			delta := [...]int{0, 0, 0, 1, 2, 3, -1, -2, -3}[mode]
			b1 := ccittFindB(previous, pos, color, false)
			a1 := b1 + delta
			if a1 < pos || a1 > len(current) {
				return fmt.Errorf("playa: invalid CCITTFaxDecode vertical offset")
			}
			for i := pos; i < a1; i++ {
				current[i] = color
			}
			pos = a1
			if color == 0 {
				color = 0xff
			} else {
				color = 0
			}
		default:
			return fmt.Errorf("playa: unsupported CCITTFaxDecode mode %d", mode)
		}
	}
	return nil
}

func ccittTableForColor(color byte) [][2]int16 {
	if color == 0xff {
		return pdfparserCCITTWhiteDecodeTable[:]
	}
	return pdfparserCCITTBlackDecodeTable[:]
}

func ccittMixedFaxDecode(data []byte, columns, rows int, align, invert bool) ([]byte, error) {
	rowBytes := (columns + 7) / 8
	capacityRows := rows
	if capacityRows <= 0 {
		capacityRows = decodedFilterExpansionLimit / rowBytes
	}
	decoded := make([]byte, 0, rowBytes*capacityRows)
	reader := ccittBitReader{data: data}
	previous := make([]byte, columns)
	for i := range previous {
		previous[i] = 0xff
	}
	current := make([]byte, columns)
	maxRows := rows
	if maxRows <= 0 {
		maxRows = decodedFilterExpansionLimit / rowBytes
	}
	for row := 0; row < maxRows; row++ {
		rowStart := reader.bit
		validEOL := true
		for i := 0; i < 12; i++ {
			bit, err := reader.readBit()
			if err != nil || (i < 11 && bit != 0) || (i == 11 && bit != 1) {
				validEOL = false
				break
			}
		}
		if !validEOL {
			if rows <= 0 && row > 0 {
				reader.bit = rowStart
				return decoded, nil
			}
			return decoded, fmt.Errorf("playa: invalid CCITTFaxDecode mixed-mode EOL")
		}
		mode, err := reader.readBit()
		if err != nil {
			return decoded, err
		}
		if align {
			reader.align()
		}
		if err := ccittDecodeMixedRow(&reader, current, previous, mode == 0); err != nil {
			return decoded, err
		}
		for i := 0; i < columns; i += 8 {
			var value byte
			for bit := 0; bit < 8 && i+bit < columns; bit++ {
				if current[i+bit] != 0 && !invert || current[i+bit] == 0 && invert {
					value |= 0x80 >> uint(bit)
				}
			}
			decoded = append(decoded, value)
		}
		previous, current = current, previous
	}
	return decoded, nil
}
