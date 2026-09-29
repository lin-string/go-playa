package parser

import (
	"bytes"
	"compress/zlib"
	"encoding/ascii85"
	"encoding/base64"
	"errors"
	"testing"
)

func TestDecodeFiltersLimitedRejectsExpandedOutput(t *testing.T) {
	var encoded bytes.Buffer
	writer := zlib.NewWriter(&encoded)
	if _, err := writer.Write(bytes.Repeat([]byte{'x'}, 4096)); err != nil {
		t.Fatalf("compress filter input: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close compressed filter input: %v", err)
	}
	if _, err := DecodeFiltersLimited(encoded.Bytes(), []string{"FlateDecode"}, nil, 1024); err == nil {
		t.Fatal("limited filter decode accepted output beyond its byte budget")
	}
}

func TestDecodePredictorLimitedRejectsExpandedRows(t *testing.T) {
	data := bytes.Repeat([]byte{0, 'x'}, 1024)
	if _, err := decodePredictorLimited(data, Dict{
		Name("Predictor"):        Number(10),
		Name("Columns"):          Number(1),
		Name("Colors"):           Number(1),
		Name("BitsPerComponent"): Number(8),
	}, 512); err == nil {
		t.Fatal("limited predictor accepted output beyond its byte budget")
	}
}

func TestDecodeFiltersLimitedRejectsASCIIHexBeforeLargeOutput(t *testing.T) {
	if _, err := DecodeFiltersLimited(bytes.Repeat([]byte{'A'}, 2048), []string{"ASCIIHexDecode"}, nil, 512); err == nil {
		t.Fatal("limited ASCIIHex decode accepted output beyond its byte budget")
	}
}

func TestASCII85LimitedDecodesStandardPayload(t *testing.T) {
	want := []byte("Hello world!")
	encoded := make([]byte, ascii85.MaxEncodedLen(len(want)))
	encoded = encoded[:ascii85.Encode(encoded, want)]
	encoded = append(encoded, '~', '>')
	got, err := ascii85DecodeLimited(encoded, 64)
	if err != nil || !bytes.Equal(got, want) {
		t.Fatalf("ASCII85 decoded = %q, err=%v", got, err)
	}
}

func TestASCII85LimitedDecodesZeroShorthand(t *testing.T) {
	got, err := ascii85DecodeLimited([]byte("z"), 64)
	if err != nil || !bytes.Equal(got, []byte{0, 0, 0, 0}) {
		t.Fatalf("ASCII85 zero shorthand = %x, err=%v", got, err)
	}
	got, err = ascii85DecodeLimited([]byte("zz"), 64)
	if err != nil || !bytes.Equal(got, []byte{0, 0, 0, 0, 0, 0, 0, 0}) {
		t.Fatalf("ASCII85 repeated zero shorthand = %x, err=%v", got, err)
	}
}

func TestASCII85LimitedRejectsZeroShorthandOverBudget(t *testing.T) {
	if _, err := ascii85DecodeLimited([]byte("z"), 3); err == nil {
		t.Fatal("ASCII85 zero shorthand exceeded its output budget without an error")
	}
}

func TestASCII85LimitedUsesDecodedSizeForBudget(t *testing.T) {
	got, err := ascii85DecodeLimited([]byte("!!!!"), 3)
	if err != nil || !bytes.Equal(got, []byte{0, 0, 0}) {
		t.Fatalf("ASCII85 decoded-size budget = %x, err=%v", got, err)
	}
}

func TestDecodeFiltersPreservesOpaqueImageEncodings(t *testing.T) {
	raw := []byte{0xff, 0xd8, 0xff, 0xd9}
	for _, filter := range []string{"DCTDecode", "JPXDecode", "JBIG2Decode"} {
		got, err := DecodeFilters(raw, []string{filter}, nil)
		if err != nil || string(got) != string(raw) {
			t.Fatalf("filter %s: got=%v err=%v", filter, got, err)
		}
	}
}

func TestDecodeFiltersStrictDCTRequiresJPEGSOI(t *testing.T) {
	raw := []byte("\x89PNG\r\n\x1a\nnot JPEG")
	for _, filter := range []string{"DCTDecode", "DCT"} {
		if _, err := DecodeFilters(raw, []string{filter}, nil); err == nil {
			t.Fatalf("strict %s decoding accepted a stream without a JPEG SOI marker", filter)
		}
		got, err := DecodeFiltersLenientLimited(raw, []string{filter}, nil, 1024)
		if err != nil || !bytes.Equal(got, raw) {
			t.Fatalf("lenient %s bytes = %x, err=%v; want original stream", filter, got, err)
		}
	}
}

func TestDecodeFiltersDecodesCCITTFaxGroup4(t *testing.T) {
	encoded, err := base64.StdEncoding.DecodeString("O1pwQMjnCO57SPGEEkwiPMpOgqV+Ekk2HT96DZXW6NgLaelbKyyq0CI6CsL/hBMqoLx9AguDBGF1VlcErK19giOgS74YUQmVpuIaI9d2KeHYUHxYRdWUNj8KyhrOxtfO0gjKEGnnZcE0CMLnYYdlJx87gHZVRULk0C4jyshWUYypgnyWghEhspJkWByJgnwRCCoX2cSIbAfiVAN75VQb/IWDGlkRZ8f99BF1/ukF6Q8JbShlahWUgKy7BYIuflDWljgAgAg=")
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := DecodeFilters(encoded, []string{"CCITTFaxDecode"}, []Dict{{
		Name("K"):       Number(-1),
		Name("Columns"): Number(153),
		Name("Rows"):    Number(55),
	}})
	if err != nil {
		t.Fatalf("CCITT Group 4 decode: %v", err)
	}
	if len(decoded) != 20*55 {
		t.Fatalf("CCITT Group 4 decoded length = %d, want %d", len(decoded), 20*55)
	}
}

func TestDecodeFiltersDecodesCCITTFaxGroup3WithEndOfLine(t *testing.T) {
	encoded, err := base64.StdEncoding.DecodeString("AB2tMwADslCDmMAB2q79ofhDAAdqfHyHnScAHaksXb7pOADsK8XN4YADsFdrnGMAB2CDhdXpwAdltideuScAHZZdus/VYAOytDrHB0VYAOytDrGOZCqAB2Uw6wIQqgAdlCdDqbhIVQAOxfH3XV5C1AB2L4/BDp2GtQAdlCH84OpaAA7KE6Ix4PDAB2Uwb1QYAOytaKtnAB2VoSTqgAOyxDboADssQ3oAB2Woa0AA7G0hDWgAHaQSiEU0gAHZcEQhihekAA7DDl0nP0gAHcA4GqlCmkAAmgUWQEkAArIUERhIABUwRSRhIABLQQSHUkiQACLA4mCapIkAAiYQVBetJEgAEUxINgNJEgAEyBuB5NIABVQblGTSAAQsGAvVogAELEWdWiAAdqWrRAAO1LV6AAdqWoTo4AO1LUJ0cAHamgiHjgA7U0DWoAO1NCOoAHanp+sAHaqtcYADskSIuADsldygAHZKnNIADslToaAB2SodDYAOyWGwAQAQAQAQAQAQ")
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := DecodeFilters(encoded, []string{"CCITTFaxDecode"}, []Dict{{
		Name("K"):         Number(0),
		Name("Columns"):   Number(153),
		Name("Rows"):      Number(55),
		Name("EndOfLine"): Bool(true),
	}})
	if err != nil {
		t.Fatalf("CCITT Group 3 decode: %v", err)
	}
	if len(decoded) != 20*55 {
		t.Fatalf("CCITT Group 3 decoded length = %d, want %d", len(decoded), 20*55)
	}
	decoded, err = DecodeFilters(encoded, []string{"CCITTFaxDecode"}, []Dict{{
		Name("K"):       Number(0),
		Name("Columns"): Number(153),
		Name("Rows"):    Number(55),
	}})
	if err != nil || len(decoded) != 20*55 {
		t.Fatalf("CCITT Group 3 stream with implicit EOL = %d bytes, err=%v", len(decoded), err)
	}
}

func TestDecodeFiltersDecodesCCITTFaxMixedMode(t *testing.T) {
	encoded, err := base64.StdEncoding.DecodeString("ABWwAeA=")
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := DecodeFilters(encoded, []string{"CCITTFaxDecode"}, []Dict{{
		Name("K"):       Number(1),
		Name("Columns"): Number(8),
		Name("Rows"):    Number(2),
	}})
	if err != nil {
		t.Fatalf("CCITT mixed-mode decode: %v", err)
	}
	if !bytes.Equal(decoded, []byte{0xf0, 0xf0}) {
		t.Fatalf("CCITT mixed-mode decoded = %x, want f0f0", decoded)
	}
	decoded, err = DecodeFilters(encoded, []string{"CCITTFaxDecode"}, []Dict{{
		Name("K"):        Number(1),
		Name("Columns"):  Number(8),
		Name("Rows"):     Number(2),
		Name("BlackIs1"): Bool(true),
	}})
	if err != nil || !bytes.Equal(decoded, []byte{0x0f, 0x0f}) {
		t.Fatalf("CCITT mixed-mode BlackIs1 decoded = %x, err=%v", decoded, err)
	}
	decoded, err = DecodeFilters(encoded, []string{"CCITTFaxDecode"}, []Dict{{
		Name("K"):       Number(1),
		Name("Columns"): Number(8),
	}})
	if err != nil || !bytes.Equal(decoded, []byte{0xf0, 0xf0}) {
		t.Fatalf("CCITT mixed-mode auto-height decoded = %x, err=%v", decoded, err)
	}
}

func TestDecodeFiltersDecodesCCITTFaxMixedUncompressedMode(t *testing.T) {
	// Mixed mode row: EOL, 2D row selector, uncompressed mode, then eight
	// literal pixels. The first reference row is all white, so no previous
	// row state is needed for this extension mode.
	bits := "000000000001" + "1" + "0000001111" + "10100101"
	encoded := make([]byte, (len(bits)+7)/8)
	for i, bit := range bits {
		if bit == '1' {
			encoded[i/8] |= 0x80 >> uint(i%8)
		}
	}
	decoded, err := DecodeFilters(encoded, []string{"CCITTFaxDecode"}, []Dict{{
		Name("K"):       Number(1),
		Name("Columns"): Number(8),
		Name("Rows"):    Number(1),
	}})
	if err != nil {
		t.Fatalf("CCITT mixed uncompressed decode: %v", err)
	}
	if !bytes.Equal(decoded, []byte{0xa5}) {
		t.Fatalf("CCITT mixed uncompressed decoded = %x, want a5", decoded)
	}
}

func TestDecodeFiltersRejectsUnsupportedCCITTModeAndMalformedParameters(t *testing.T) {
	for name, parms := range map[string]Dict{
		"group 3":      {Name("K"): Number(0)},
		"mixed rows":   {Name("K"): Number(1)},
		"columns":      {Name("K"): Number(-1), Name("Columns"): Number(0)},
		"rows":         {Name("K"): Number(-1), Name("Rows"): Number(-1)},
		"alignment":    {Name("K"): Number(-1), Name("EncodedByteAlign"): Name("true")},
		"black-is-1":   {Name("K"): Number(-1), Name("BlackIs1"): Number(1)},
		"end-of-line":  {Name("K"): Number(-1), Name("EndOfLine"): Bool(true)},
		"damaged-rows": {Name("K"): Number(-1), Name("DamagedRowsBeforeError"): Number(-1)},
	} {
		if _, err := DecodeFilters(nil, []string{"CCITTFaxDecode"}, []Dict{parms}); err == nil {
			t.Fatalf("malformed %s parameters were accepted", name)
		}
	}
	if _, err := DecodeFilters(nil, []string{"CCF"}, []Dict{{
		Name("K"):       Number(-1),
		Name("Columns"): Number(1 << 30),
		Name("Rows"):    Number(1),
	}}); err == nil {
		t.Fatal("oversized CCITT output was accepted")
	}
}

func TestDecodeFiltersAcceptsDamagedRowsBeforeErrorLikePlaya(t *testing.T) {
	encoded, err := base64.StdEncoding.DecodeString("ABWwAeA=")
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := DecodeFilters(encoded, []string{"CCITTFaxDecode"}, []Dict{{
		Name("K"):                      Number(1),
		Name("Columns"):                Number(8),
		Name("Rows"):                   Number(2),
		Name("DamagedRowsBeforeError"): Number(1),
	}})
	if err != nil || !bytes.Equal(decoded, []byte{0xf0, 0xf0}) {
		t.Fatalf("DamagedRowsBeforeError should not reject an otherwise valid decode: got=%x err=%v", decoded, err)
	}
}

func TestDecodeFiltersKeepsCCITTDamagedRowsBeforeErrorOutput(t *testing.T) {
	encoded, err := base64.StdEncoding.DecodeString("ABWwAeA=")
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := DecodeFilters(encoded[:len(encoded)-1], []string{"CCITTFaxDecode"}, []Dict{{
		Name("K"):                      Number(1),
		Name("Columns"):                Number(8),
		Name("Rows"):                   Number(2),
		Name("DamagedRowsBeforeError"): Number(1),
	}})
	if err != nil {
		t.Fatalf("damaged CCITT rows should preserve decoded output: %v", err)
	}
	if len(decoded) == 0 {
		t.Fatal("damaged CCITT rows discarded the valid prefix")
	}
}

func TestDecodePredictorUsesColorComponentCount(t *testing.T) {
	// One PNG-predictor row, RGB: filter None followed by three samples.
	got, err := decodePredictor([]byte{0, 10, 20, 30}, Dict{Name("Predictor"): Number(10), Name("Columns"): Number(1), Name("Colors"): Number(3), Name("BitsPerComponent"): Number(8)})
	if err != nil || string(got) != string([]byte{10, 20, 30}) {
		t.Fatalf("predictor RGB = %v, err=%v", got, err)
	}
}

func TestDecodePredictorRejectsRowWiderThanInput(t *testing.T) {
	_, err := decodePredictor([]byte{0}, Dict{
		Name("Predictor"):        Number(10),
		Name("Columns"):          Number(1 << 60),
		Name("Colors"):           Number(1),
		Name("BitsPerComponent"): Number(8),
	})
	if err == nil {
		t.Fatal("predictor row wider than input was accepted")
	}
}

func TestDecodeFiltersReturnsTypedFilterError(t *testing.T) {
	_, err := DecodeFilters(nil, []string{"UnknownDecode"}, nil)
	var parseErr *ParseError
	if !errors.As(err, &parseErr) {
		t.Fatalf("error type = %T, want ParseError", err)
	}
	if parseErr.Operation() != "filter UnknownDecode" {
		t.Fatalf("filter error = %#v", parseErr)
	}
}
