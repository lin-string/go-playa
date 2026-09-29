# Mapping Sources and Compatibility

English | [简体中文](mapping-sources.zh-CN.md)

go-playa does not treat Playa's historical lookup tables as the normative
source for font or character mappings. Playa is an important compatibility
reference, but its tables can reflect the version and platform assumptions of
the pinned oracle release.

## Source hierarchy

The implementation uses these sources in descending authority:

1. PDF-provided `ToUnicode`, embedded font `cmap`, CID metrics, and explicit
   `Differences` entries.
2. The normative Adobe tables for the relevant domain:
   - Adobe Glyph List (`glyphlist.txt`) for glyph-name to Unicode mappings.
   - ITC Zapf Dingbats Glyph List for Zapf Dingbats names.
   - Adobe CMap Resources and character collections for byte-to-CID mappings,
     code spaces, vertical variants, and CID data. The migrated loader embeds
     the complete CMap resource set from the vendored Adobe CMap Resources
     snapshot, including Identity, legacy East Asian encodings, UCS2/UTF8/
     UTF16/UTF32 maps, UniJISPro, JIS X 0213, and Adobe character collections
     for Japan1, GB1, CNS1, Korea1, Adobe-KR, and Adobe-Manga1. Named
     `usecmap` inheritance is resolved lazily with isolated cached copies.
3. Context-specific historical tables for MacExpert, platform aliases, and
   documented private-use mappings. The Symbol encoding is generated from the
   synchronized Core 14 `Symbol.afm`, with a small set of documented
   Symbol-specific glyph-name overrides.
4. Playa's table and behavior, used as a compatibility fallback when no more
   authoritative source applies.

`ToUnicode` remains the preferred source for extracted text because it is
embedded in the PDF and describes that document's intended mapping. Glyph
lists are the fallback for simple fonts and `Differences`; they do not replace
CID CMaps, widths, or font-program character maps.

## AGL versus AGLFN

AGL and AGLFN are related but not interchangeable. AGL maps legacy glyph names
to Unicode values, including historical presentation-form and private-use
assignments. AGLFN is a recommended-name subset for new fonts and intentionally
omits names whose AGL mapping is not the semantically preferred Unicode value.
PDF extraction therefore uses AGL first; AGLFN is useful for validation and
future naming heuristics, not as a replacement table.

## Generated data

The synchronized Adobe Glyph List (AGL) and ITC Zapf sources are stored at
`scripts/data/glyphlist.txt` and `scripts/data/zapfdingbats.txt`. Refresh them
from Adobe and regenerate the Go maps with:

```text
go generate ./document ./fontdata
```

The source synchronizer is `scripts/sync_glyphlist_sources.py`; it accepts
`--source-dir` for an offline local Adobe checkout. The parser/generator is
`scripts/generate_glyphlist.py`, and the generated output is
`fontdata/glyphlist_generated.go`. Updating the source requires reviewing
mapping changes and adding a compatibility test for any changed legacy alias.

The generator validates duplicate names and Unicode scalar values before
writing deterministic Go tables. The generated ITC table is the authoritative
Zapf source; only eight explicitly documented historical aliases remain in
`fontdata.GlyphTextWithLegacyAliases` for compatibility with older PDF/Playa
behavior.

The predefined CMap resources are synchronized under `fontdata/cmapdata/`
from the current Adobe `cmap-resources` repository. Run
`go generate ./document ./fontdata` to refresh them; the synchronization script also
accepts `--source-dir` for an offline local Adobe checkout. The embedded set is
the complete upstream `*/CMap/*` resource set, not a manually selected list. The
runtime name registry is derived from that embedded directory, and the fontdata
tests load every synchronized resource so a newly added upstream CMap cannot be
silently left unreachable.
`font.LoadPredefinedCMap` exposes the same loader for callers that need to
resolve a named CMap outside a page font dictionary.

`font.LoadPredefinedUnicodeMap` exposes the corresponding immutable CID to
Unicode view used by CID font fallback. It is built lazily from those same
Adobe UTF-32 CMaps, cached by collection and writing mode, and accepts both
the full Adobe registry name (for example `Adobe-Japan1`) and the PDF
`Ordering` value (`Japan1`). The returned map never exposes its backing store;
use `MappingCopy` or `Finalize` when an owned snapshot is required.

For Type0 descendants with a recognized Adobe `CIDSystemInfo` ordering, the
loader also builds a reverse CID-to-Unicode fallback from the matching UTF32
CMap, including supplementary Unicode characters and the `KR` and `Manga1`
collections. This fallback is deliberately below PDF `ToUnicode` and
embedded-font character maps in precedence.

CIDFontType2 resources also honor an explicit `CIDToGIDMap` stream. Embedded
TrueType glyph mappings are looked up through that table, while the PDF
`Identity` form and omitted form retain the direct CID-to-GID behavior.

For embedded TrueType and TrueType-outline OpenType fonts, `head`, `hhea`,
and `hmtx` are read lazily from `FontFile2` or `FontFile3 /Subtype /OpenType`
to supply cmap mappings and scaled glyph advance widths. Explicit PDF widths
remain authoritative. CFF1, Type1C, and CFF2 `FontFile3` programs now provide
their embedded geometry metadata; CharString execution and outline extraction
are implemented for Type2 drawing programs, including CID FDSelect-specific
local subroutines, CFF2 variation-store evaluation, and lazy path extraction.

CFF standard SID strings, predefined charsets, and StandardEncoding are
generated directly from the public fontTools source. CFF ExpertEncoding uses
the distinct `CFFExpertEncoding.java` table from the pinned Apache PDFBox
source under `scripts/data/pdfbox/`, checked independently against Adobe
Technical Note #5176 Appendix B. The generator preserves the upstream numeric
CFF SIDs; it does not substitute MacExpertEncoding or use a Playa table.

Run `go generate ./document ./fontdata` to refresh the generated Go tables. For
offline generation, pass `--fonttools-source-dir` for a local fontTools checkout
and `--expert-source` for the CFF ExpertEncoding Java input; its default is the
checked-in PDFBox input.

The CFF-generated StandardEncoding supplies every defined PDF StandardEncoding
slot at runtime. Undefined slots are intentionally absent; they must not be
filled from MacRoman-like historical tables or decoded as replacement text.

Core 14 standard font metrics are generated from the 14 Adobe AFM files. The
original Adobe download endpoint is no longer available, so the checked-in
inputs are synchronized from the public `tc-font-core14-afms` mirror, whose
README identifies the files as the Adobe Core 14 AFMs. The AFM format and field
semantics follow Adobe Technical Note #5004. This provenance is deliberately
documented as a mirror rather than presented as a current Adobe repository;
the synchronizer can be switched to an official Adobe endpoint if one is
published again.

Run `go generate ./document ./fontdata` to refresh the AFMs and regenerate
`fontdata/standard_metrics_generated.go`. The synchronizer accepts
`--source-dir` for an offline checkout of the AFM source.

The built-in Symbol code-to-Unicode mapping is generated from
`scripts/data/core14/Symbol.afm` and `scripts/data/glyphlist.txt` by
`scripts/generate_symbol_encoding.py`. Symbol-specific names whose meaning is
defined by the Symbol font rather than general AGL are kept as explicit
overrides in that generator. The generated table is refreshed by
`go generate ./document ./fontdata`.

The built-in ZapfDingbats code-to-Unicode mapping is generated from
`scripts/data/core14/ZapfDingbats.afm` and `scripts/data/zapfdingbats.txt` by
`scripts/generate_zapfdingbats_encoding.py`, with the AFM space slot retained
explicitly because it is not part of the ITC glyph-name list.

The high-byte MacRoman mapping is generated from the standard Unicode/CPython
`mac_roman` codec by `scripts/generate_macroman_encoding.py`. The generated
table is limited to the PDF MacRoman high-byte range and is refreshed by
`go generate ./document ./fontdata`.

The high-byte WinAnsi mapping is generated from the standard Windows-1252
codec by `scripts/generate_winansi_encoding.py`; codec-undefined slots remain
absent from the generated table, matching PDF undefined-slot behavior.

MacExpertEncoding is generated from Apache PDFBox's separate
`MacExpertEncoding.java` table. Both ExpertEncoding inputs are synchronized by
`scripts/sync_macexpert_encoding.py` from PDFBox commit
`5ae91127c3316db8663da843dce746022f22df91`, with SHA-256 values pinned in the
synchronizer. The Java inputs retain their Apache-2.0 source headers under
`scripts/data/pdfbox/`; their license and attribution are recorded in `NOTICE`.
`make resources-check` verifies the pinned input digests offline, then checks
that the generated Go tables match those inputs. The synchronizer's
`--source-dir` accepts a local PDFBox checkout.

`scripts/generate_macexpert_encoding.py` writes
`fontdata/macexpert_encoding_generated.go`. MacExpert remains a glyph-name
mapping so AGL, private-use, and composite-name resolution share the same
precedence-aware path. CFF ExpertEncoding instead maps character codes to CFF
SIDs; sharing the source project does not make the two encodings interchangeable.

## Private and platform mappings

Private-use mappings are never installed as unconditional global Unicode
truth. The same code point or glyph name can have different meanings in
Symbol, MacExpert, Zapf Dingbats, Apple, and vendor-specific fonts. Such tables
must carry their scope and only participate after PDF-provided mappings and
standard glyph-name resolution have been considered.

When a private mapping is added, record its source, applicable font or
encoding, Unicode interpretation, and precedence in the implementation or a
focused test. Historical Playa-derived mappings are retained only when they
improve compatibility without overriding a document-provided or normative
mapping.

## Difference from Playa

This source-first policy is an intentional behavior difference from using a
historical Playa table as the complete mapping database. It may produce a more
complete result for newer glyph names while preserving Playa-compatible output
for scoped legacy and private-font cases. Compatibility tests must distinguish
an actual semantic regression from an intentional correction to an outdated
table.
