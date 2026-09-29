package document

import "testing"

func TestContentParentWithErrorRejectsNilDocumentConsistently(t *testing.T) {
	tests := map[string]func() error{
		"annotation": func() error {
			_, err := (Annotation{}).ParentWithError(nil)
			return err
		},
		"form xobject": func() error {
			_, err := (XObjectObject{}).ParentWithError(nil)
			return err
		},
		"image": func() error {
			_, err := (ImageObject{}).ParentWithError(nil)
			return err
		},
		"glyph": func() error {
			_, err := (GlyphObject{}).ParentWithError(nil)
			return err
		},
		"text": func() error {
			_, err := (TextObject{}).ParentWithError(nil)
			return err
		},
		"path": func() error {
			_, err := (PathObject{}).ParentWithError(nil)
			return err
		},
		"marked content": func() error {
			_, err := (MarkedContent{}).ParentWithError(nil)
			return err
		},
		"tag": func() error {
			_, err := (TagObject{}).ParentWithError(nil)
			return err
		},
		"content object": func() error {
			_, err := (ContentObject{}).ParentWithError(nil)
			return err
		},
		"annotation reply": func() error {
			_, err := (Annotation{}).InReplyToAnnotationWithError(nil)
			return err
		},
		"annotation popup": func() error {
			_, err := (Annotation{}).PopupAnnotationWithError(nil)
			return err
		},
		"form field parent": func() error {
			_, err := (FormField{}).ParentFieldWithError(nil)
			return err
		},
		"outline parent": func() error {
			_, err := (OutlineNode{}).ParentNodeWithError(nil)
			return err
		},
		"structure object": func() error {
			_, _, err := (StructureContent{}).ObjectWithError(nil)
			return err
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			if err := test(); err != errNilDocument {
				t.Fatalf("ParentWithError(nil) error = %v, want %v", err, errNilDocument)
			}
		})
	}
}
