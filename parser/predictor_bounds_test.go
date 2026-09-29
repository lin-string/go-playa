package parser

import "testing"

func TestDecodePredictorRejectsUnknownPNGFilter(t *testing.T) {
	_, err := decodePredictor([]byte{9, 0}, Dict{
		Name("Predictor"):        Number(10),
		Name("Columns"):          Number(1),
		Name("Colors"):           Number(1),
		Name("BitsPerComponent"): Number(8),
	})
	if err == nil {
		t.Fatal("unknown PNG predictor filter was accepted")
	}
}

func TestDecodePredictorRejectsUnsupportedPredictor(t *testing.T) {
	if _, err := decodePredictor(nil, Dict{Name("Predictor"): Number(3)}); err == nil {
		t.Fatal("unsupported predictor was accepted")
	}
}

func TestDecodePredictorRejectsMalformedExplicitParameters(t *testing.T) {
	for _, key := range []Name{Name("Columns"), Name("Colors"), Name("BitsPerComponent")} {
		p := Dict{Name("Predictor"): Number(10), key: Number(0)}
		if _, err := decodePredictor(nil, p); err == nil {
			t.Fatalf("invalid predictor parameter %s was accepted", key)
		}
	}
	for _, bits := range []int{3, 32} {
		if _, err := decodePredictor(nil, Dict{Name("Predictor"): Number(10), Name("BitsPerComponent"): Number(bits)}); err == nil {
			t.Fatalf("invalid predictor bit depth %d was accepted", bits)
		}
	}
}
