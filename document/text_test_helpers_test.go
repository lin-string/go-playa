package document

import "github.com/lin-string/go-playa/contentdata"

func newTestText(spec contentdata.TextSpec, font *Font, glyphs []GlyphObject) TextObject {
	return TextObject{data: contentdata.NewTextBorrowed(spec), font: font, glyphs: glyphs}
}

func newTestTextWithMarkedStack(spec contentdata.TextSpec, stack []markedContentContext, font *Font, glyphs []GlyphObject) TextObject {
	dataStack := make([]contentdata.MarkedContentContext, len(stack))
	for i := range stack {
		dataStack[i] = stack[i].data
	}
	spec.MarkedStack = dataStack
	return TextObject{data: contentdata.NewTextBorrowed(spec), font: font, glyphs: glyphs, markedStack: stack}
}

func setTestTextData(text *TextObject, update func(*contentdata.TextSpec)) {
	if text == nil {
		return
	}
	visualBBox, hasVisualBBox := text.VisualBBoxCopy()
	var visualBBoxPtr *[4]float64
	if hasVisualBBox {
		visualBBoxPtr = &visualBBox
	}
	spec := contentdata.TextSpec{
		Text: text.Text(), Page: text.Page(), HasPage: text.HasPage(), Chars: text.Chars(),
		FontName: text.FontName(), FontSize: text.FontSize(), Size: text.Size(),
		FontBase: text.FontBase(), TextFont: text.TextFont(), Args: text.data.ArgsBorrowed(),
		Matrix: text.Matrix(), TextMatrix: text.TextMatrix(), LineMatrix: text.LineMatrix(),
		ScalingMatrix: text.ScalingMatrix(), Origin: text.Origin(), Displacement: text.Displacement(),
		Rotation: text.Rotation(), BBox: text.BBox(), VisualBBox: visualBBoxPtr,
		LineWidth: text.LineWidth(), StrokeColor: text.data.StrokeColorCopy(),
		NonStrokeColor: text.data.NonStrokeColorCopy(), Invisible: text.Invisible(),
		Vertical: text.Vertical(), Unmapped: text.Unmapped(), GState: text.GState(),
		MarkedTag: text.MarkedTag(), MarkedProperties: text.data.MarkedPropertiesCopy(),
		MarkedStack: text.MarkedStackCopy(), ActualText: text.ActualText(),
		MCID: text.MCID(), HasMCID: text.HasMCID(),
	}
	update(&spec)
	text.data = contentdata.NewTextBorrowed(spec)
}
