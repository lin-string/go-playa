# go-playa

English | [简体中文](README.zh-CN.md)

`go-playa` is a native Go 1.25 PDF analyzer designed for high-throughput,
lazy document analysis with auditable compatibility against
[Playa](https://github.com/dhdaines/playa).

## Why go-playa

- **High-throughput analysis.** Lazy reads, shared resource caches, and bounded
  page concurrency support CPU-heavy PDF workloads. Comparisons are scoped to
  a document, task, worker topology, and machine; see the
  [benchmark protocol](docs/benchmark.md).
- **Native Go integration.** Deploy the library without Python or a
  cross-language serialization bridge; use its domain types in application APIs.
- **Lazy, explicit resource control.** Repeatable iterators support early stop
  and deferred errors. Cache budgets, `Finalize`/`Copy`, and `Close` expose
  memory and ownership decisions.
- **Auditable compatibility.** Required projections against a pinned Playa
  oracle, malformed-input tests, and deterministic resource generation define
  the acceptance boundary for objects, pages, text, glyphs, images, and structure.

The tracked migration is complete. Every section in
[`compat/manifest.toml`](compat/manifest.toml) is required. The checked-in and
pinned public PDF corpora define the release acceptance boundary. New
producer-specific files may still expose defects, but they are compatibility hardening rather than an open
migration phase.

## Performance evidence

The [corpus](bench/corpus.json) contains nine public PDFs spanning
native text, graphics, image-only scans, OCR scans, and mixed documents in
English, Chinese, Arabic, Japanese, Korean, and French, from 119 to 1290 pages.
The [matrix](bench/matrix.json) compares six tasks with workers 1/2/4/8 where
applicable and gates ratios on matching counts, canonical digests, and effective
worker topology.

On 2026-09-29, four fully qualified all-corpus single-worker task aggregates
measured **3.36×–5.56×** Playa's throughput. The reference run qualified 73 of
77 cells (770 samples), and the workers 1/2/4/8 scaling run qualified 81 of 89
cells (890 samples). OpenIntro `text-glyph-jsonl` and RISC-V `image-digests`
were blocked because their canonical outputs differed; their timings are not
used as cross-library ratios.

See the [complete dated matrices](docs/benchmark-results-2026-09-29.md) and the
[benchmark protocol](docs/benchmark.md). These were interactive desktop runs,
not lab-isolated measurements. Throughput and memory depend on the workload; a
speed comparison does not imply lower peak RSS.

## Install and open a document

```bash
go get github.com/lin-string/go-playa
```

```go
package main

import (
	"fmt"

	"github.com/lin-string/go-playa"
)

func main() {
	doc, err := playa.Open("example.pdf")
	if err != nil {
		panic(err)
	}
	defer doc.Close()

	fmt.Println(doc.PageCount())

	for page, err := range doc.Pages() {
		if err != nil {
			panic(err)
		}
		text, err := page.ExtractText(doc, playa.DefaultTextExtractionOptions())
		if err != nil {
			panic(err)
		}
		fmt.Print(text)
	}
}
```

`OpenBytes` parses an in-memory PDF. `Open` and `OpenBytes` accept functional
options such as `WithPassword`, `WithCoordinateSpace`, and `WithCacheOptions`.
Copy the default cache options before changing one budget; zero disables
retention for that cache without changing parsing results:

```go
cache := playa.DefaultCacheOptions()
cache.PageBytes = 0

doc, err := playa.Open("example.pdf", playa.WithCacheOptions(cache))
```

## CID-aware glyph rendering

go-playa can recover and draw the actual embedded outline for an individual
PDF glyph. This is useful for CID-font diagnostics, OCR dataset generation,
font inspection, and debugging documents where extracted Unicode text alone
does not identify the shape that was painted.

The outline path follows the PDF's own mapping and placement state:

- Type 0/CIDFontType2 fonts honor encoding CMaps and explicit `CIDToGIDMap`
  streams; identity mappings retain direct CID-to-GID behavior.
- CID CFF/CFF2 fonts support FDSelect-specific local subroutines and CFF2
  variation-store evaluation. Type1, Type3 CharProcs, and embedded TrueType
  outlines use the same public glyph path model.
- `GlyphObject.PathsSeq()` yields lazy device-space paths. The `glyphrender`
  package writes standalone SVG or bounded-memory PNG, while `WriteJSONL` and
  `ExportGlyph` stream CID, GID, font, geometry, and selected rendered outputs.
- Rendering requires an executable embedded outline. A font without one
  produces an empty path sequence and `glyphrender.ErrNoOutline` from SVG/PNG
  rendering; go-playa does not silently substitute a system font.

Finalize a borrowed glyph before retaining or rendering it outside its page
iteration:

```go
import (
	"errors"
	"io"

	playa "github.com/lin-string/go-playa"
	"github.com/lin-string/go-playa/glyphrender"
)

func renderFirstGlyph(output io.Writer, doc *playa.Document, page playa.Page) error {
	for glyph, err := range page.Glyphs(doc) {
		if err != nil {
			return err
		}
		glyph = glyph.Finalize()
		err = glyphrender.RenderSVG(output, glyph, glyphrender.SVGOptions{Scale: 2})
		if errors.Is(err, glyphrender.ErrNoOutline) {
			continue
		}
		return err
	}
	return glyphrender.ErrNoOutline
}
```

See [font and character mapping sources](docs/mapping-sources.md) for mapping
precedence, supported embedded programs, and CID collection data.

## Alignment baseline

The canonical oracle identity—package, version, tag, and source commit—is
[`compat/upstream.toml`](compat/upstream.toml). README deliberately does not
copy its version number. Both compatibility drivers read that file, cache keys
include its identity, and tests require the Python dependency pin to match it.

[`compat/manifest.toml`](compat/manifest.toml) maps each compared section to its
Playa and Go public APIs. [`docs/compatibility.md`](docs/compatibility.md)
describes the projection, corpus, tolerances, recovery cases, and cache format.

```bash
make public-fixtures-pull
make compat
make compat-local
make compat-release
COMPAT_NO_CACHE=1 make compat-one \
  COMPAT_PDF=testdata/files/form_simple.pdf
```

Alignment means that required projections match on the checked-in and pinned
public corpora while
also satisfying Go-side tests for ordering, laziness, ownership, malformed
input, and lifecycle behavior. It does not mean that implementation details or
Python runtime infrastructure are copied.

Compatibility is strict by default. A result may intentionally differ from the
pinned Playa oracle only when review establishes that go-playa follows the
applicable PDF semantics and the mismatch is caused by a confirmed Playa
defect. Such a deviation is not a blanket xfail: it must be tied to an upstream
issue and pinned to the exact fixture and observed difference so that any new,
changed, overlapping, or disappearing mismatch fails for review. The audited
lists are [`compat/known_differences.json`](compat/known_differences.json) for
ordinary corpus comparisons and the `differences` and `upstream_issues` entries
in [`internal/testfixture/pdf_association_fixtures.json`](internal/testfixture/pdf_association_fixtures.json)
for PDF Association cases. See the [compatibility protocol](docs/compatibility.md)
for the validation rules.

## Playa-to-Go API mapping

Public domain nouns and behavior follow Playa; control flow and naming follow
Go. The detailed symbol ledger is [`docs/api-audit.md`](docs/api-audit.md).

| Playa convention | Go convention | Example |
| --- | --- | --- |
| `snake_case` public name | exported Go name | `Page.extract_text` → `Page.ExtractText` |
| read-only property | accessor method | `Element.alternate_description` → `StructElement.AlternateDescription()` |
| generator or lazy iterator | repeatable `iter.Seq` or `iter.Seq2` | `Document.pages` → `Document.Pages()` |
| iterator that may fail while advancing | `iter.Seq2[T, error]` | `Page.tokens` → `Page.Tokens(doc)` |
| keyword argument | options value or functional option | `space=` → `WithCoordinateSpace(...)` |
| mapping protocol | named mapping methods | `Document.get/keys/values/items` → `Get/Keys/Values/Items` |
| indexed access | domain-specific lookup | page index → `Document.PageAt(index)` |
| context manager | explicit lifecycle | `with open(...)` → `Open` plus `defer Close()` |
| `finalize()` or retained model | explicit ownership operation | `value.Finalize()`, `Copy()`, or `...Copy()` |

Names are not kept in duplicate merely for compatibility with an earlier
go-playa spelling. A public symbol must identify a Playa counterpart or a
documented Go adaptation. Root and domain facades may re-export the same
current type—for example, the root `playa.Document` entry point and the public
`document.Document` engine—but these exports form one API model, not a
compatibility-alias layer.

### Intentional interface differences

- Eager operations return `(T, error)`. Errors discovered only while advancing
  a source are yielded by `iter.Seq2` and stop that traversal.
- A `Page` is a small value, so document-backed page operations receive the
  owning `*Document` explicitly. Python Playa can carry this context through
  object references.
- Go options replace Python keyword arguments. Zero-value behavior and default
  constructors are documented by each option package.
- Values on high-frequency content sequences may be borrowed read-only views.
  Call `Finalize`, `Copy`, or a named copy accessor before retaining them across
  iteration, goroutines, or cache release.
- `Close` and `ReleaseTransientCaches` are explicit, exclusive lifecycle
  operations. Finish all document reads before calling either one.
- The Go API adds bounded page concurrency, the `user` coordinate space, and
  standalone glyph rendering. These additions are tested separately when the
  pinned Playa oracle has no equivalent.
- Python packaging, multiprocessing implementation details, notebooks, and
  documentation-site tooling are outside the port.

Font and character mappings use Adobe and Unicode specifications and maintained
fontTools or Apache PDFBox data instead of treating historical Playa tables as
the final authority. Precedence and intentional differences are documented in
[`docs/mapping-sources.md`](docs/mapping-sources.md).

## Public packages

Import the root package for opening documents. Import a domain package when its
types appear in your own API.

| Package | Responsibility |
| --- | --- |
| `playa` | compact `Open`/`OpenBytes` facade and common entry types |
| `document` | document parsing, object lookup, pages, lazy caches, lifecycle, and cross-domain coordination |
| `page` | page-facing content, annotations, forms, resources, and structure models |
| `content` | text, glyph, path, image, marked-content, resource-selection, and layout facades |
| `font`, `image` | document-facing fonts/CMaps and images/color spaces |
| `outline`, `structure` | navigation and tagged-PDF semantic models |
| `pdftypes`, `parser` | PDF primitive values, lexical parsing, object parsing, and stream filters |
| `geometry`, `coordinates` | matrices, rectangles, paths, colors, and coordinate-space policy |
| `documentdata`, `contentdata`, `fontdata`, `imagedata`, `structuredata` | dependency-free owned value models and source-derived data |
| `cacheconfig`, `contentconfig`, `documentconfig`, `parserconfig`, `structureconfig`, `textconfig`, `layout` | dependency-free configuration values |
| `glyphrender` | Go-specific bounded SVG/PNG and JSONL glyph export |

The root and document-facing domain facades depend on `document`; `document`
depends on `parser` and the owning value, geometry, and configuration packages.
Those lower-level packages do not import `document` merely to name a
dependency-free value.

## Laziness and ownership

An API corresponding to a Playa generator remains lazy. Every traversal is
fresh, preserves source order, stops as soon as the consumer stops, and does
not surface an error from an item that was never requested. Collecting helpers
such as `CollectPages` and `CollectObjects` are explicit materialization points,
not replacements for the sequence APIs.

Values yielded by high-frequency sequences can be borrowed, read-only views
whose lifetime ends when the traversal advances. Before retaining one after
iteration or cache release, or passing it to another goroutine, use its
`Finalize`, `Copy`, `ValueCopy`, or named copy operation.

## Concurrency guarantees

- Read-only operations may share one open `Document` across goroutines. Shared
  indexes and resource caches are synchronized; interpreter state is local to
  each traversal.
- `ForEachPageConcurrent(ctx, callback, options...)` provides bounded adaptive
  page scheduling. With no options, the maximum worker count is automatic and
  the growth budget is ten pages per potential worker. Scheduling stays bounded
  by `GOMAXPROCS` and page count; `WithMaxPageWorkers` and `WithPagesPerWorker`
  configure the maximum and growth budget.
- Callback completion order is unspecified. The first callback error or
  recovered callback panic cancels remaining work and is returned by the
  concurrent page API. Context cancellation stops further scheduling; the call
  waits for running callbacks to finish before returning.
- Borrowed values remain read-only and traversal-scoped inside callbacks. Copy
  or finalize a value before another goroutine retains it.
- `Close` is idempotent. `Close` and `ReleaseTransientCaches` are exclusive
  lifecycle operations and must wait until all reads and page callbacks finish.

Omit options to use the adaptive defaults, or configure them explicitly:

```go
ctx := context.Background()
err = doc.ForEachPageConcurrent(ctx, func(page playa.Page) error {
	_, err := page.ExtractText(doc, playa.DefaultTextExtractionOptions())
	return err
}, playa.WithMaxPageWorkers(4), playa.WithPagesPerWorker(10))
if err != nil {
	panic(err)
}
```

The document owner keeps `Close` deferred, as in the opening example, so it
runs only after the concurrent call returns. Do not call `Close` or
`ReleaseTransientCaches` from a page callback. Both `playa` and `document`
export the sealed `PageConcurrencyOption` type and its constructors. A
non-positive maximum keeps automatic selection; a non-positive page budget
uses ten. Nil options are ignored, and later options take precedence.
`ForEachPageLayoutConcurrent(ctx, layoutOptions, callback, options...)` uses
the same options; finalize or copy layout results before retaining them after
the callback returns. See the
[domain model guide](docs/core-domain-model.md) for cache and ownership
diagrams.

## Command-line tool

The `playa` command exposes the useful document inspection modes migrated from
Playa:

```bash
go run ./cmd/playa -text example.pdf
go run ./cmd/playa -outline example.pdf
go run ./cmd/playa -structure example.pdf
go run ./cmd/playa -images example.pdf
```

Run `go run ./cmd/playa -help` for the complete flag set.

## License and independence

go-playa is MIT-licensed; [LICENSE](LICENSE) includes the copyright and license
notices for this implementation and the retained Playa and pdfminer.six
notices. Other bundled third-party materials remain under their respective
terms in [NOTICE](NOTICE) and [THIRD_PARTY_LICENSES](THIRD_PARTY_LICENSES).

go-playa is an independent Go implementation and is not an official Playa project.
The compatibility oracle's package, version, tag, and source commit
remain defined only by [compat/upstream.toml](compat/upstream.toml).

## Development

See the [contribution guide](.github/CONTRIBUTING.md) and
[security policy](.github/SECURITY.md). English policy and contract documents
are canonical when translations differ.

Go 1.25 is the minimum language and standard-library baseline. CI uses the
patched toolchain pinned in [`.go-version`](.go-version).

```bash
pre-commit install
make verify
```

Common focused gates:

```bash
make test
make test-race
make api-audit
make vuln-check
make module-archive-check
make module-source-test
make public-fixtures-pull
make public-fixtures-check
make compat-public
make compat-local
make pdfa-fixtures-pull
make corpus-test
make compat-fonts
make compat-recovery
make resources-check
make bench-compare COMPAT_PDFS=testdata/files/form_simple.pdf
```

`make verify` checks formatting, module tidiness, `go vet`, lint, normal tests,
race tests, and the public API audit. Use `make corpus-test` for malformed-input,
recovery, or cross-domain changes. Resource tables must be refreshed through
their source synchronizers and generators, never by editing generated Go files.

`make vuln-check` is a separate network-dependent release gate. It checks Go
code and the locked Python compatibility environment against current advisory
databases; CI runs these checks as well.

Small generated PDF fixtures under `testdata/files` are ordinary Git files.
Large public PDFs are downloaded separately into `.compat-cache/public-corpus`
after an explicit `make public-fixtures-pull`; the manifest pins their sources,
licenses, sizes, and SHA-256 digests. The PDFs are not bundled into the Go
module. `make public-fixtures-check` verifies the local copies without network
access. Ordinary `go test ./...` and `make verify` do not download them.
Tests using an absent optional large fixture report a skip; run the pull and
check commands before `make verify` for that additional coverage, as CI does.
`make module-archive-check` guards the source archive size, while
`make module-source-test` verifies the public source-only path; both are
included in `make verify`.

Detailed maintenance contracts:

- [`docs/engineering.md`](docs/engineering.md): API, testing, error, cache, and dependency rules.
- [`docs/migration.md`](docs/migration.md): completed migration scope and retained behavioral contracts.
- [`docs/compatibility.md`](docs/compatibility.md): pinned oracle and corpus workflow.
- [`docs/benchmark.md`](docs/benchmark.md): pinned corpus, task/worker matrix, correctness gates, and next-run procedure.
- [`.github/pull_request_template.md`](.github/pull_request_template.md): review and verification checklist.

Compatibility caches and benchmark snapshots are local artifacts and are not
committed.
