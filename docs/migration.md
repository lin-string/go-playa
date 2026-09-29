# Playa migration contract

English | [简体中文](migration.zh-CN.md)

## Status and acceptance boundary

The tracked migration is complete. A Playa user can identify the corresponding
Go package, type, method, or sequence for every section in
[`compat/manifest.toml`](../compat/manifest.toml), and every listed section is
required by the release compatibility gate.

The oracle package, version, tag, and commit are defined only in
[`compat/upstream.toml`](../compat/upstream.toml). The checked-in and pinned
public PDF corpora define the acceptance boundary. Additional files may expose
defects or extend the supported producer set, but they do not make the completed ledger an active
migration phase again.

## Scope

go-playa implements Playa's reusable PDF behavior:

- lexical and object parsing, xref tables and streams, object streams,
  incremental updates, recovery, encryption, and stream filters;
- document mappings, pages, content streams and tokens;
- graphics state, text, glyphs, paths, images, Form XObjects, resources,
  marked content, and layout analysis;
- fonts, encodings, CMaps, ToUnicode, embedded font programs, metrics, and
  outlines;
- metadata, names, destinations, actions, outlines, annotations, AcroForm,
  page labels, and tagged-PDF structure;
- useful inspection CLI modes for content, outline, structure, text, text
  objects, images, and fonts.

Python packaging, multiprocessing implementation details, notebooks,
documentation-site tooling, and benchmark harness internals are outside the
port. The Go benchmark and compatibility commands are repository tools, not
Playa API surface.

go-playa also provides documented Go additions: bounded page concurrency, a
`user` coordinate space, configurable bounded caches, and standalone bounded
glyph rendering. They have Go-side tests when the pinned oracle has no public
counterpart.

## Interface mapping

Preserve Playa's domain nouns and observable behavior while adapting control
flow to Go:

- Python `snake_case` names become idiomatic exported Go names.
- Read-only properties become accessor methods.
- Eager failures return `(T, error)`.
- Generators become fresh, repeatable `iter.Seq[T]` or
  `iter.Seq2[T, error]` values.
- Python keyword arguments become options values or functional options.
- Mapping and indexing protocols become named `Get`, `Lookup`, `Keys`,
  `Values`, `Items`, or domain-specific lookup methods.
- Context-manager and finalization behavior becomes explicit `Close`, `Copy`,
  `Finalize`, or named copy accessors.
- A document-independent `Page` receives `*Document` explicitly for operations
  that need object resolution or shared caches.

Do not retain a second public spelling solely for compatibility with an older
go-playa API. Root and domain facades may re-export one current type into its
documented domain; that is one API model, not a compatibility-alias layer.
Every exported symbol must have a Playa counterpart or a necessary Go
adaptation recorded in
[`api-audit.md`](api-audit.md).

## Public package ownership

The root `playa` package is the compact opening facade. `document` is the
public document engine and owns parsing, lazy caches, lifecycle, page/object
lookup, resource resolution, and cross-domain coordination.

Domain-facing facades live in `page`, `content`, `font`, `image`, `outline`,
and `structure`. Dependency-free values and algorithms live with their owners:

| Owner | Values and behavior |
| --- | --- |
| `pdftypes` / `pdftypes/primitives` | PDF scalar, array, dictionary, stream, and reference values |
| `parser` | lexer, object parser, text decoding, diagnostics, and stream filters |
| `geometry`, `coordinates` | matrices, rectangles, paths, colors, and coordinate spaces |
| `documentdata` | metadata, encryption metadata, page-label, xref, indirect-object, destination, action, outline, annotation, and form values |
| `contentdata` | content operations, marked content, graphics-state snapshots, glyph/path/tag/XObject/resource-selection values |
| `fontdata` | CMaps, decoded glyphs, font metadata, source-derived mapping data, and font-program parsing helpers |
| `imagedata` | decoded images, color-space scalar values, and packed sample expansion |
| `structuredata` | dependency-free tagged-PDF element, content, and item values |
| configuration packages | cache, content, document, parser, structure, text, and layout options |

The root and document-facing domain facades depend on `document`; `document`
depends on `parser` and the owning value, geometry, and configuration packages.
A dependency-free domain package must not import `document` merely to name a
value.

## Iteration contract

Laziness is public behavior. A sequence corresponding to a Playa generator:

1. preserves upstream source order;
2. starts a fresh traversal on every range;
3. performs no work for an item the consumer does not request;
4. stops immediately when the consumer breaks;
5. yields an advancing error once and then stops.

Document pages, objects, tokens, annotations, forms, outline nodes, structure
nodes, and page content/resources use this contract. Collecting helpers such as
`CollectPages`, `CollectObjects`, `CollectAnnotations`, `CollectFormFields`,
`CollectOutline`, and `CollectTokens` are explicit materialization points.
They do not replace the primary sequence APIs.

Tests for generator-shaped behavior cover order, repeatability, early stop,
and a malformed later item whose error must remain deferred.

## Ownership and stability

Go has no read-only slice, map, or pointer type. Public parser and model state
therefore uses these boundaries:

- Scalar state is exposed through accessor methods.
- Cache-returned mutable data is isolated from internal state.
- High-frequency lazy sequences may yield borrowed read-only views.
- `Finalize`, `Copy`, `ValueCopy`, or a named `...Copy` method creates an owned
  value when retention is supported.
- JSON projection is explicit and does not depend on exported mutable fields.

A borrowed value must not be retained or mutated after advancing its sequence,
crossing a cache-release boundary, or returning from a page callback unless it
has first been finalized or copied.

## Lifecycle and concurrency

Call `defer doc.Close()` immediately after a successful `Open` or `OpenBytes`.
`Close` is idempotent.

Read-only operations on one open document may run concurrently. Document
caches are synchronized; page interpreters and temporary output state are
page-local. `Document.ForEachPageConcurrent(ctx, callback, options...)` and
`Document.ForEachPageLayoutConcurrent(ctx, layoutOptions, callback, options...)`
provide bounded adaptive page scheduling through sealed `PageConcurrencyOption`
values. `WithMaxPageWorkers` and `WithPagesPerWorker` configure the automatic
worker maximum and ten-page growth budget defaults; non-positive values retain
those defaults. The type and constructors are exported by `document` and the
root facade. Scheduling respects `GOMAXPROCS` and page count, produces pages
lazily, and does not guarantee callback completion order. Context cancellation
or the first callback error or recovered panic stops remaining work; calls wait
for running callbacks to finish. Finalize or copy borrowed values and layout
results before retaining them after the callback returns.

`Close` and `ReleaseTransientCaches` are exclusive lifecycle operations. They
must not overlap document reads, page workers, or borrowed-value use.

## Errors, recovery, and security

The library returns stable sentinel or typed errors and wraps causes with
`%w`. It does not log an error that it also returns, and it never converts a
parse failure into an unexplained empty result.

Recovery is explicit and tested against the pinned oracle where the oracle can
open the input. Go-only damaged-input recovery and security limits remain in
the local corpus when the oracle cannot safely provide a reference result.
Malformed-input work must preserve bounded allocation, lazy error delivery,
and cache accounting.

Some parser recovery deliberately follows the pinned oracle rather than the
strictest PDF interpretation, including object-number lookup and selected
content-stream recovery. Intentional differences that protect bounded resource
use remain Go-side security behavior and are recorded in tests or
[`compatibility.md`](compatibility.md).

## Fonts and mapping sources

Document-provided ToUnicode, embedded font maps, CID metrics, and explicit
encoding differences take precedence. Adobe and Unicode specifications, and
fontTools and Apache PDFBox data, supply generated fallback mappings. Historical Playa
tables are compatibility references, not automatically the normative source.

All generated font, CMap, glyph-list, AFM, and CFF data must be refreshed
through the synchronizers and generators. Provenance, precedence, and
intentional differences are documented in
[`mapping-sources.md`](mapping-sources.md).

## Change workflow

For a new or changed public behavior:

1. Identify the Playa module, class, property, method, or generator and its
   owning Go package.
2. Add a failing behavior and external API test, including laziness and
   ownership checks where applicable.
3. Implement the smallest compatible behavior without eager materialization or
   mutable parser-state exposure.
4. Update `compat/manifest.toml`, this contract, or `api-audit.md` when the
   accepted public scope or mapping changes.
5. Run `make verify` and the relevant oracle/corpus/resource gates.

Use `make compat` for public extraction or projection behavior,
`make corpus-test` for malformed/recovery/cross-domain behavior,
`make compat-fonts` or `make compat-recovery` for their focused oracle corpora,
and `make resources-check` for generated resource changes.

The former checkbox ledger is retained only as the completion record in
[`migration-todo.md`](migration-todo.md).
