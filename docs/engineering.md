# Engineering conventions

English | [简体中文](engineering.zh-CN.md)

## Toolchain and checks

- Go 1.25 is the minimum language and standard-library baseline. Preserve every Playa generator/iterator as an independently iterable `iter.Seq[T]` or `iter.Seq2[T, error]`; laziness is public semantics, not an optional optimization.
- Makefile Go commands default to `GOTOOLCHAIN=local`. This prevents a
  verification run from silently downloading another toolchain into the shared
  module cache; override it explicitly only when testing a different installed
  toolchain.
- Run `make verify` before every commit. It checks formatting, module tidiness
  without rewriting modules, engineering contracts, `go vet`, `golangci-lint`,
  normal tests, and race tests.
- Go build and module caches are global to the user, not owned by this
  repository. Use `make go-cache-status` to inspect them and
  `make go-cache-clean` to remove build/test artifacts. Module cleanup is
  intentionally separate as `make go-module-cache-clean`, because it also
  removes dependencies used by unrelated repositories.
- `testdata/files` contains small generated PDF fixtures stored as ordinary Git
  blobs. Larger publisher PDFs are listed with source, license, byte size, and
  SHA-256 in [`compat/public_corpus.json`](../compat/public_corpus.json). Run
  `make public-fixtures-pull` explicitly to download them into the ignored
  `.compat-cache/public-corpus` directory; `make public-fixtures-check` verifies
  them offline. The PDFs are not bundled into the source archive or Go module.
  A missing checked-in fixture fails its test. Ordinary package tests and
  `make verify` do not download large PDFs; tests using an unavailable optional
  public fixture report a skip. CI explicitly pulls and checks the public corpus
  before `make verify`. `make module-archive-check` guards
  the public source size, and `make module-source-test` verifies tests in a
  source-only tree; both are included in `make verify`.
- Public PDF Association fixtures are cloned directly from their original
  repositories into `../pdf-association-fixtures` by default, or the directory
  selected by `GO_PLAYA_PDFA_FIXTURE_DIR`. `make pdfa-fixtures-pull` checks out
  the manifest-pinned commits; `make pdfa-fixtures-check` is offline and
  verifies origin URLs, clean HEADs, and every selected file digest. Ordinary
  package tests skip with a pull hint when this optional checkout is absent,
  while `make pdfa-corpus-test`, `make corpus-test`, `make compat-pdfa`, and CI
  require it and fail hard.
- `make verify` also runs `make api-audit`, which prevents repository-only
  compatibility adapter types from leaking into public packages.
- Run `make vuln-check` before a release or after dependency changes. This
  separate network-dependent gate checks Go code and the locked Python
  compatibility environment against current advisory databases. CI uses the
  patched Go toolchain pinned in [`.go-version`](../.go-version).
- Run `pre-commit install` once per clone. Its single hook runs `make verify`,
  the same non-mutating source checks used by CI, and is intentionally not
  bypassed. Fix reported formatting explicitly with `make fmt` and module
  changes with `go mod tidy`; verification does not rewrite source files.
- `make engineering-check` scans non-generated production Go files, including
  untracked sources. It rejects nil comparisons of `context.Context`
  parameters, required function parameters of exported APIs, and receivers of
  exported pointer-receiver methods, as well as package-level blank-identifier
  function/method references that hide dead code. Diagnostics use
  `path:line:rule`; parse failures also fail the gate. Generated sources, tests,
  hidden directories, and vendored dependencies are outside this source-policy
  scan. `make verify` includes this gate, so both pre-commit and CI enforce it.
  Named parameter types are resolved only through declarations that can coexist
  with the API's file under Go OS/architecture filename suffixes and build tags.
  This checks all supported source variants without importing a different
  platform's callback contract into a pointer parameter.
- Run `make resources-check` when changing generated font/CMap data or before
  opening a resource change PR. This check contacts the declared public source
  repositories where synchronization requires network access, verifies pinned
  local inputs, compares checked-in inputs without writing them, regenerates
  derived Go tables in a temporary directory, and fails on source or generated
  drift.
- Run `make compat` for extraction, page-model, or public-domain behavior
  changes. Use `make compat-one` while iterating, but include the full
  compatibility result before requesting review. CI runs the configured spaces
  and corpus independently of local caches.
- Run `make corpus-test` for malformed-input, recovery, or cross-domain
  acceptance changes; use `make compat-fonts` and `make compat-recovery` for
  their Playa oracle projections.
- The focused corpus targets run generator drift checks before tests:
  `python3 scripts/generate_security_fixtures.py --check`,
  `python3 scripts/generate_recovery_fixtures.py --check`, and
  `python3 scripts/generate_acceptance_fixtures.py --check`. Use the matching
  generator without `--check` to regenerate checked-in PDFs intentionally.
- Encrypted fixtures must be checked with `COMPAT_PASSWORD=... make compat-one`
  or the equivalent `playa-compat --password` flag. Passwords are hashed in
  cache metadata and are never written as plaintext cache fields.

## Alignment maintenance and pull requests

- Every public behavior change must identify the corresponding Playa module,
  class, property, method, or generator in the PR description and update the
  compatibility manifest or migration contract when their scope changes.
- Public API changes require a focused external API test and, when behavior is
  observable in the oracle, a compatibility projection field or an explicit
  documented intentional difference.
- Generator-shaped behavior requires ordering, repeatability, early-stop, and
  deferred-error tests. Lazy values borrowed from interpreter state must expose
  an explicit `Finalize`/`Copy` ownership path where retention is supported.
- Resource changes must be produced by the source synchronizer and generator;
  generated Go files are review artifacts, not hand-edit targets. Run
  `go generate ./document ./fontdata` from the repository root; generator defaults
  must resolve to the owning public `fontdata` or `parser`
  package rather than an unrelated implementation package. Include the
  source URL/commit and license impact in the PR.
- A PR is ready only when `make verify`, the relevant `make compat` command,
  and `make resources-check` (when applicable) pass. The PR must state any
  command that could not run and why.
- Keep `go.mod` dependency-light. Prefer the Go standard library for PDF parsing and command-line tooling.

## Public API

- Keep the repository root small: `playa.go`, root-level integration tests,
  and cross-domain compatibility glue only. New domain code belongs in its
  public package (`content`, `font`, `page`, and so on) or in `internal/`.
- Public packages mirror Playa's domain modules. A user may import
  `github.com/lin-string/go-playa/content` when they need a content type;
  the root `playa` package remains the convenient `Open` entry point.
- `document` is the canonical owner of document-wide parsing, lazy caches,
  lifecycle, and cross-domain coordination. New dependency-free behavior
  belongs in its owning public domain package; document-aware orchestration
  remains in `document`.
- Dependency-free geometric values belong in the public `geometry` package.
  The public `document` engine imports that package directly; new public APIs
  must also import `geometry` directly.
- Dependency-free PDF scalar, array, dictionary, and indirect-reference values
  belong in the public `pdftypes/primitives` package; lexical parsing, object parsing, PDF text
  decoding, parser diagnostics, and stream filter decoding belong in the
  public `parser` package, including indirect filter metadata normalization.
  The public `document` engine depends on these owning packages and retains
  document-aware operations such as filtered `Stream` access. Dependency-free
  packed image sample expansion belongs in the public `imagedata` package;
  dependency-free BDC/DP Properties, marked-content context, marked-content
  section, and content operation values belong in `contentdata`.
  Byte ownership copies should use the shared `primitives.CloneBytes` helper.
  Dependency-free document value models such as `Metadata` belong in the
  public `documentdata` package. Document parsing and cache lifecycle code
  remains in the document engine.
- Dependency-free option and discriminator values belong in the public
  `cacheconfig`, `contentconfig`, `documentconfig`,
  `parserconfig`, `structureconfig`, and `textconfig` packages. The matching
  internal configuration paths have been removed; do not add legacy-name
  compatibility exports.
- Document-aware page scheduling options belong in `document`: the sealed
  `PageConcurrencyOption` type and `WithMaxPageWorkers`/`WithPagesPerWorker`
  constructors are re-exported by the root facade.
- PDF object implementations use the exported `PDFObject` marker so an owning
  domain package can define a primitive without depending on `document`.
- Preserve Playa's domain nouns and semantics: `Document`, `Page`, content objects, fonts, destinations, annotations, and structure elements.
- Adapt control flow to Go. Return `(T, error)` for eager operations; for a source that may fail while advancing, return `iter.Seq2[T, error]`. Use options rather than Python keyword arguments.
- `Open` and `OpenBytes` take `...OpenOption` directly. Keep option
  constructors small and composable, and use `DefaultCacheOptions()` as the
	baseline when changing only selected cache budgets. A zero budget means
	"do not retain" and must not change observable parsing results, including
	bounded terminal-error caches such as `ActionErrorBytes` and
	`DestinationErrorBytes`, `ObjectErrorBytes`, and
	`ObjectStreamErrorBytes`, plus the font, resource, page-geometry, page-box,
	and content-root error-budget fields, as well as annotation and structure
	error budgets.
	NameTree and destination-root error budgets are configurable as well.
	Font outline and Type3 CharProc cache budgets are configurable through the
	same document options.
	All bounded-cache admission checks must use the overflow-safe `cacheFits`
	contract; do not reimplement budget checks with manual `limit-current`
	subtraction.
- A public sequence preserves upstream ordering, supports a fresh traversal on each `range`, does no work for items the consumer does not request, and stops immediately when the consumer breaks. Never replace an upstream generator with `[]T`.
- An aggregation without an upstream Playa counterpart belongs in an adapter application, never in a public library package. `internal/testcompat` may collect only for the repository's compatibility oracle; no package may re-export its types or functions.
- A successfully opened `Document` has a `Close() error` lifecycle method. Callers should use `defer doc.Close()` immediately after `Open` succeeds; `Close` must be safe to call more than once.
- Do not expose mutable parser internals as a convenience shortcut. Values from
  high-frequency lazy content sequences may be borrowed views of interpreter
  state; callers must not retain or mutate them across iteration unless they
  first call the model's `Finalize` method. Cache-boundary APIs must still
  isolate returned values.
- Every public behavior added to the library needs a corresponding compatibility-projection field or an explicit rationale in the migration document.

## Errors

- This is a reusable library, not a service: do not import application-specific business error codes.
- Define stable sentinel errors for detectable categories and wrap causes with `%w`. Use typed errors when callers need location, object reference, or parser operation details.
- Never log and return the same error. Libraries return contextual errors; commands print one terminal error and return a non-zero status.
- Never replace a parse failure with an empty slice, zero value, or silently skipped PDF object. Recovery must be explicit and observable.

## Required inputs and nil semantics

- A `context.Context` parameter is required: callers pass a real context,
  including `context.Background()` when no cancellation is needed. Do not
  compare it with nil, supply a background fallback, or translate nil into a
  library error.
- Function parameters on exported APIs are required unless their optional nil
  behavior is explicitly documented. Do not preflight-check required callbacks
  for nil. Required-input violations are programmer errors, not recoverable
  PDF failures; use ordinary dereference/invocation semantics.
- Exported methods with pointer receivers require a real receiver. Do not add
  nil-receiver success results, sentinels, or no-op fallbacks, including inside
  returned iterator closures. Valid zero-value structs and optional pointer
  fields keep their documented behavior.
- Nil functional options remain optional. Page-concurrency options ignore nil
  and apply later options last. Optional resolver/filter parameters in private
  helpers may retain their deliberate nil semantics.
- The checker has exact public callback exceptions for
  `pdftypes.Stream.DecodedBufferWithResolver`,
  `DecodedBufferWithResolverWithError`, and `DecodedBufferDigestWithResolver`:
  a nil resolver means direct filter metadata. The `resolve` callback of
  `fontdata.ParseType1CharStringWithSeac` is optional when seac resolution is
  unavailable. Adding a public optional callback requires documenting its nil
  behavior and an exact checker exception with regression coverage. Tests
  validate every exception against its current file, API, receiver, and named
  function parameter, so stale entries fail `make verify`; naming a callback
  `resolve` does not exempt it.

## Tests and determinism

- Write a failing behavior test before new production code.
- Unit tests cover malformed syntax and boundary values. Integration tests use
  real PDFs. They may explicitly skip when a declared external fixture is not
  available, but a missing checked-in fixture is always a failure.
- The compatibility oracle is the final authority for parity. For a declared
  PDF Association conformance case, its documented expectation is the
  correctness authority; a deliberate correction beyond the pinned oracle
  must use an exact, stale-checked difference record. Preserve object order
  unless the upstream API explicitly does not specify it.
- Sequence tests must cover ordering, repeated traversal, early break, and deferred errors from a later source item. The PDF corpus must include a later-item failure fixture so eager preloading cannot pass unnoticed.
- Do not add a new bulk extraction worker adapter. Page-level goroutine
  parallelism is supported by `Document.ForEachPageConcurrent`; document its
  ownership, cancellation, ordering, and lifecycle rules, and prove `-race`
  clean results plus identical sequential/concurrent benchmark digests.

## Performance acceptance

- On representative CPU-heavy public interface paths, roughly 5x Playa's
  throughput with lower peak memory is a directional optimization reference,
  not a universal acceptance gate or correctness requirement. Compare matching
  semantics and inputs in fresh processes under comparable worker topology.
- Quantify lower ratios or higher memory and explain them in the change
  documentation. Fixed startup cost, I/O or compression, native-library
  kernels, different concurrency models, and measurement noise can be valid
  explanations. Avoidable Go-controlled copying, allocation, parsing, or lock
  contention remains an optimization target rather than an exception by
  itself.
- Performance-sensitive changes must use fresh processes, preserve matching
  result counts, and cover representative large text, layout, image, and object
  workloads. Include both one-worker and bounded concurrent runs where the API
  supports concurrency; alternate warm runs so filesystem cache order does not
  bias one implementation.
- Use CPU and heap profiles to explain a regression before changing behavior or
  cache policy. Do not hide retained work with eager collection, a benchmark-
  only garbage collection call, or a less complete projection. Treat benchmark
  regression warnings as prompts for investigation, never as permission to
  weaken correctness or compatibility checks.

## Dependency direction

The root and document-facing domain facades (`content`, `font`, `page`,
`image`, `outline`, and `structure`) depend on `document`. The document engine
depends on `parser` and the owning value, geometry, and configuration packages,
not on those facades. Dependency-free packages must not import `document` or
its facades; pass a narrow context or stable identifier when needed. This
preserves direct public imports without Go import cycles.

## Data and licensing

- Keep upstream-derived tables, fixtures, and generated data traceable to an upstream commit and license in `NOTICE`.
- Do not copy surrounding Python packaging, multiprocessing, or service infrastructure unless it is needed for a public PDF domain behavior or command-line feature.

## Concurrent reads and lifecycle

After `Open` succeeds, read-only operations on one `Document` are safe to use
from multiple goroutines. Page iterators, content interpreters, and their
temporary output state are page-local; document-level lazy caches are shared
and synchronized internally. Values returned by borrowed sequences must not be
mutated or retained across iteration unless `Finalize`, `ValueCopy`, or the
relevant `Copy` method is used.

`ForEachPageConcurrent(ctx, callback, options...)` and
`ForEachPageLayoutConcurrent(ctx, layoutOptions, callback, options...)` use
the same functional options and adaptive scheduler. No options means an
automatic worker maximum and a ten-page growth budget per potential worker,
bounded by `GOMAXPROCS` and page count. `WithMaxPageWorkers` configures the
maximum; `WithPagesPerWorker` configures growth. Non-positive values retain
their respective defaults. Nil options are ignored; later options take
precedence. Pages are produced lazily, callback completion order is unspecified,
and context cancellation or the first callback error or recovered panic stops
remaining work. Calls wait for running callbacks before returning. Finalize or
copy borrowed values and layout results before they outlive their callback.

`Close` and `ReleaseTransientCaches` are exclusive lifecycle operations. They
must not run concurrently with document reads because they release or reset
document-owned state. Callers should finish all page workers before invoking
either operation.
