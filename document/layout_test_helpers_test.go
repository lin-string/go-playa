package document

import "github.com/lin-string/go-playa/layout"

func newTestLayoutComponent(text string, bbox [4]float64, vertical bool, index int) layout.Component {
	return layout.NewComponent(layout.ComponentSpec{
		Text:     text,
		BBox:     bbox,
		Vertical: vertical,
		Index:    index,
	})
}

func newTestTextWord(text string, bbox [4]float64) TextWord {
	return TextWord{data: newTestLayoutComponent(text, bbox, false, -1)}
}

func newTestTextLine(text string, bbox [4]float64, vertical bool) TextLine {
	return TextLine{data: newTestLayoutComponent(text, bbox, vertical, -1)}
}

func newTestTextParagraph(text string, bbox [4]float64, vertical bool) TextParagraph {
	return TextParagraph{data: newTestLayoutComponent(text, bbox, vertical, -1)}
}

func newTestTextBox(text string, bbox [4]float64, vertical bool, index int) TextBox {
	return TextBox{data: newTestLayoutComponent(text, bbox, vertical, index)}
}

func newTestTextGroup(text string, bbox [4]float64, vertical bool) TextGroup {
	return TextGroup{data: newTestLayoutComponent(text, bbox, vertical, -1)}
}
