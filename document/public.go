// Package document owns the PDF document domain model and its lazy caches.
package document

import (
	"iter"

	"github.com/lin-string/go-playa/cacheconfig"
	"github.com/lin-string/go-playa/contentconfig"
	"github.com/lin-string/go-playa/coordinates"
	"github.com/lin-string/go-playa/documentconfig"
	"github.com/lin-string/go-playa/geometry"
	"github.com/lin-string/go-playa/layout"
	"github.com/lin-string/go-playa/parserconfig"
	"github.com/lin-string/go-playa/structureconfig"
	"github.com/lin-string/go-playa/textconfig"
)

type (
	// PageList is the repeatable lazy page sequence returned by Document.Pages.
	PageList = iter.Seq2[Page, error]

	ContentFilter         = contentconfig.Filter
	ContentKind           = contentconfig.Kind
	ContentOptions        = contentconfig.Options
	StructureContentKind  = structureconfig.ContentKind
	StructureItemKind     = structureconfig.ItemKind
	Matrix                = geometry.Matrix
	Point                 = geometry.Point
	Rect                  = geometry.Rect
	Color                 = geometry.Color
	PathSegment           = geometry.PathSegment
	CoordinateSpace       = coordinates.Space
	CacheOptions          = cacheconfig.Options
	DocumentOptions       = documentconfig.Options
	OpenOption            = documentconfig.OpenOption
	LayoutOptions         = layout.Options
	TextExtractionOptions = textconfig.Options
	TokenKind             = parserconfig.TokenKind
)
