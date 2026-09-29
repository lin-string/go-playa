package fontdata

import "embed"

// CMapFiles contains the synchronized Adobe predefined CMap resources.
// The predefined loader, inheritance resolver, and immutable cache live in
// this same resource domain so public font callers do not depend on the PDF
// document engine for CMap access.
//
//go:embed cmapdata/*.cmap
var CMapFiles embed.FS
