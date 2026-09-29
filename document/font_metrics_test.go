package document

import "testing"

func TestFontMetricsNormalizesPositiveDescent(t *testing.T) {
	f := NewSimpleFont("F")
	setFontMetrics(f, Dict{Name("Ascent"): Number(900), Name("Descent"): Number(120)})
	if f.descent != -120 {
		t.Fatalf("descent = %v, want -120", f.descent)
	}
}

func TestFontMetricsReadsLeading(t *testing.T) {
	f := NewSimpleFont("F")
	setFontMetrics(f, Dict{Name("Leading"): Number(24)})
	if f.leading != 24 {
		t.Fatalf("leading = %v, want 24", f.leading)
	}
}

func TestFontMetricsFallsBackToFontBBox(t *testing.T) {
	f := NewSimpleFont("F")
	setFontMetrics(f, Dict{Name("Ascent"): Number(0), Name("Descent"): Number(0), Name("FontBBox"): Array{Number(-10), Number(-200), Number(900), Number(700)}})
	if f.ascent != 700 || f.descent != -200 {
		t.Fatalf("metrics = ascent %v descent %v", f.ascent, f.descent)
	}
}

func TestBuiltInSymbolFontMetrics(t *testing.T) {
	f := NewSimpleFont("Symbol")
	f.defaultWidth = 1000
	applyStandardFontMetrics(f)
	if f.ascent != 1010 || f.descent != -293 || f.fontBBox != [4]float64{-180, -293, 1090, 1010} {
		t.Fatalf("Symbol geometry = ascent %v descent %v bbox %v", f.ascent, f.descent, f.fontBBox)
	}
	f.applyEncoding("Symbol")
	if got := f.HDisp('A'); got != 0.722 {
		t.Fatalf("Symbol Alpha width = %v, want 0.722", got)
	}
	if got := f.HDisp('a'); got != 0.631 {
		t.Fatalf("Symbol alpha width = %v, want 0.631", got)
	}
	if got := f.HDisp('D'); got != 0.612 {
		t.Fatalf("Symbol Delta width = %v, want 0.612", got)
	}
	if got := f.HDisp('V'); got != 0.439 {
		t.Fatalf("Symbol final sigma width = %v, want 0.439", got)
	}
	if got := f.HDisp(165); got != 0.713 {
		t.Fatalf("Symbol infinity width = %v, want 0.713", got)
	}
}

func TestBuiltInZapfDingbatsFontMetrics(t *testing.T) {
	f := NewSimpleFont("ZapfDingbats")
	f.defaultWidth = 1000
	applyStandardFontMetrics(f)
	if f.ascent != 820 || f.descent != -143 || f.fontBBox != [4]float64{-1, -143, 981, 820} {
		t.Fatalf("ZapfDingbats geometry = ascent %v descent %v bbox %v", f.ascent, f.descent, f.fontBBox)
	}
	if got := f.HDisp(33); got != 0.974 {
		t.Fatalf("ZapfDingbats first glyph width = %v, want 0.974", got)
	}
	if got := f.HDisp(34); got != 0.961 {
		t.Fatalf("ZapfDingbats second glyph width = %v, want 0.961", got)
	}
}

func TestStandardFontAliasesPreserveItalicAngle(t *testing.T) {
	for _, test := range []struct {
		name  string
		angle float64
	}{
		{name: "Helvetica-BoldOblique", angle: -12},
		{name: "Times-Italic", angle: -15},
		{name: "Times-BoldItalic", angle: -15},
	} {
		font := NewSimpleFont(test.name)
		applyStandardFontMetrics(font)
		if font.italicAngle != test.angle {
			t.Errorf("%s italic angle = %v, want %v", test.name, font.italicAngle, test.angle)
		}
	}
}

func TestBuiltInCourierFontMetrics(t *testing.T) {
	f := NewSimpleFont("Courier")
	applyStandardFontMetrics(f)
	if f.ascent != 629 || f.descent != -157 || f.fontBBox != [4]float64{-23, -250, 715, 805} {
		t.Fatalf("Courier geometry = ascent %v descent %v bbox %v", f.ascent, f.descent, f.fontBBox)
	}
	if got := f.HDisp('M'); got != 0.6 {
		t.Fatalf("Courier width = %v, want 0.6", got)
	}
}
