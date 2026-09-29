# Compatibility oracle

English | [简体中文](compatibility.zh-CN.md)

`cmd/playa-compat --compare` is the authoritative end-to-end compatibility check.
It opens each supplied PDF with the Go implementation, reads the pinned Playa
snapshot from `.compat-cache`, and compares the projections in Go. Snapshots
are stored as a header followed by one JSONL record per page, so neither the
Python driver nor the Go comparator needs to retain the complete document
projection in memory. On a cache miss it invokes
`scripts/compare_playa.py --snapshot-only --snapshot-jsonl`; legacy complete-JSON
caches are converted to compressed JSONL streams page by page, including large
caches, without loading the complete snapshot into memory. Projection schema
changes invalidate prior cache keys. On cache misses, the Go driver starts one Playa process for each
comparison section group and streams the complete JSONL snapshot into a
temporary file. The writer releases page-local projection state after each
serialized page, so the process can be reused without reopening the PDF for
every page batch while the output remains bounded by the current page record.
This preserves one comparison stream and the original page order without
retaining a complete document projection in either process.
For PDFs larger than 8 MiB, the default required-section comparison is further
split into independent section groups: document/object metadata, structure,
page/text extraction, glyphs, layout, marked-content/annotations, and heavy
content/resources. The high-volume page projections are intentionally isolated
so a single Playa JSONL snapshot cannot combine text, glyph, and layout output
into an unbounded temporary file. Each group owns a separate Go `Document` and
Playa process, then the command emits one final result for the PDF. Focused
`--section` checks and smaller PDFs remain single-pass.
The Playa snapshot writer also honors the requested section list while walking
each page: unselected text, path, image, layout, stream, and content iterators
are not touched. This is important for focused compatibility runs because a
single projection should not materialize unrelated page caches.
Unused ParentTree slots are omitted from the page-structure projection, matching
the Go model's populated-entry sequence and Playa's `None` page-structure slots.
The source package/tag/commit and coordinate space are recorded in
[`compat/upstream.toml`](../compat/upstream.toml); `compat/uv.lock` makes the
Python environment reproducible.

## Contract

The v1 projection is extended in place by sections. `compat/manifest.toml`
maps each section to the corresponding public APIs and marks it `required` or
`pending`. The default command compares every `required` section. A section can
be selected only for diagnosis with `--section`; `--release` fails while any
section remains pending.

## Openly licensed large-document corpus

`compat/public_corpus.json` pins three additional large publisher PDFs with
their byte sizes, SHA-256 digests, attribution and license evidence: RISC-V
Unprivileged Architecture, GNU Emacs Manual and OpenIntro Statistics. The PDF
binaries live in `.compat-cache/public-corpus` and are not distributed in the
Go module. Some publisher URLs can change; a changed download must fail digest
verification until its content and license have been explicitly reviewed.

```bash
make public-fixtures-pull
make public-fixtures-check
make compat-public
```

`compat-public` compares all pages and required sections in page, screen and
default space against `compat/upstream.toml`. It reuses the existing `COMPAT_*`
options for workers, memory, cache and timing. `PUBLIC_CORPUS_MANIFEST` and
`PUBLIC_FIXTURE_DIR` select an alternate manifest and local directory for the
public corpus. Set `GO_PLAYA_PUBLIC_FIXTURE_DIR` to use the same alternate
directory from Go package tests and Make targets. Each public PDF path is passed
as a separate argument, including when its directory contains spaces. Checking
or comparing never downloads missing documents; use the explicit pull command.
Unsupported differences are failures to investigate and fix, not automatic
exceptions. Run `make public-corpus-test` for the helper's offline regression
suite; this suite also participates in `make verify`.

`make compat-local` runs the release comparison for every eligible checked-in
small PDF in `COMPAT_LOCAL_PDFS`, in all configured coordinate spaces. It does
not require the publisher PDF download. `make compat` and `make compat-release`
include both that local corpus and all PDFs in the public manifest by default.
Setting `COMPAT_PDFS` explicitly selects only the supplied files and does not
append the public corpus. Compatibility comparisons are local gates: use
`compat-local` for the complete checked-in corpus and `compat-public` or
`compat` for the public documents.

Strict comparisons can still report independently supported differences:
the Go inventory includes fonts in nested resource dictionaries, preserves
actual clipping state, and uses the Adobe AFM and ITC Zapf Dingbats mappings.
The pinned native PNG Average predictor can also overflow before division;
compare the compressed source with an independent decoder before classifying
such a mismatch. These categories are not automatic waivers: each actual
failure still needs source evidence, and `compat-public` returns failure for
unrecorded differences.

## PDF Association conformance corpus

`internal/testfixture/pdf_association_fixtures.json` is the strict manifest for
the public conformance corpus. It pins direct HTTPS clones of the original PDF
Association repositories by full commit, records every selected PDF's SHA-256,
license and evidence URL, and declares only the compatibility sections relevant
to that file's stated behavior. No upstream PDF binary is copied into this
module.

Every tracked PDF at each pinned source commit must appear exactly once as a
selected fixture or an audited exclusion. Exclusions also pin the file digest
and evidence URL and explain why the file has no distinct deterministic test
expectation, for example because it is documentation, a byte-identical copy of
another selected fixture, or an upstream case whose documented result is
explicitly indeterminate. `make pdfa-fixtures-check` rejects any unlisted PDF,
missing manifest path, or digest change.

Prepare and verify the corpus, run its behavior tests, and compare it with the
pinned oracle using:

```bash
make pdfa-fixtures-pull
make pdfa-fixtures-check
make pdfa-corpus-test
make compat-pdfa
```

The default checkout is the sibling `../pdf-association-fixtures`; set
`GO_PLAYA_PDFA_FIXTURE_DIR` or `PDFA_FIXTURE_DIR` to use another location.
Ordinary `go test` reports a skip with the pull command when the checkout is
absent. `make corpus-test`, `make pdfa-corpus-test`, and `make compat-pdfa` are
explicit local gates: they verify repository identity, clean pinned HEADs, and file
digests and fail if any source or fixture is unavailable. Setting
`GO_PLAYA_FETCH_PDFA_FIXTURES=1` lets an individual PDF Association fixture test fetch its declared
source explicitly.

The selected cases cover PDF 2.0 strings and revisions, producer and parser
edge cases, graceful handling of unsupported filters, path graphics-state
semantics including Indexed color normalization, negative dash phases,
degenerate and closed dashed corners, line-cap and join state, transparency
groups, and ColorBurn/ColorDodge blend modes, annotation action trees and
appearances, nested font resources,
content-stream grammar, strict stream-versus-dictionary object kinds, Unicode
12.1 text extraction through ToUnicode and `ActualText`, Unicode 3.2 password
normalization, and strict rejection of UTF-16LE as a PDF text-string encoding.
They also include paired passing and failing accessibility examples for role
mapping, Unicode, structure and marked-content order, headings, cross-page
lists, artifacts, structure-element `ActualText`, column reading order, and
sidebar placement in logical content order.

Encrypted public fixtures may declare their published test password in the
manifest. The compatibility environment installs Playa's `crypto` extra so
the pinned oracle exercises those files in local comparisons. A fixture whose
documented password must fail records that expectation explicitly and remains
outside compatibility projection, while still participating in corpus tests.

The PDF Association test description is the correctness authority; the pinned
Playa release remains the compatibility authority. A failure unique to the Go
implementation is fixed and must regain exact compatibility. If both
implementations violate the conformance expectation, the Go behavior is fixed
to the documented expectation and the remaining Playa result is recorded in
the same manifest. Such records are not broad xfails: fixture ID and SHA-256,
coordinate space, complete section list, and every difference line or terminal
Playa or Go projection exception must match exactly. An unrecorded difference
fails, and a recorded difference that disappears also fails as stale.

The ordinary local compatibility corpus applies the same policy through
[`compat/known_differences.json`](../compat/known_differences.json). Each record
is scoped by the PDF SHA-256, coordinate spaces, and an exact section group
(plus any explicitly declared exact alternate group), and
must reference a confirmed `existing` or `submitted` Playa issue from the
PDF Association manifest or the corpus manifest's own `upstream_issues` list.
Corpus issue entries include the checked source commit and date, reproduction,
expected result, and confirmed URL; each lists exactly the records it owns. The path pattern only partitions differences by the
documented root cause; acceptance also requires the exact number and the
SHA-256 of the complete ordered difference set. Object keys are compared in a
canonical order so this digest is reproducible across Go map iterations.
Comparison therefore retains
all differences instead of stopping after a diagnostic prefix. A changed
value, added or removed page difference, order change, unrelated path, changed
fixture, or upstream fix makes the record fail for review rather than silently
passing as an xfail.

Generate a reviewable, neutral issue body without publishing it by naming an
upstream issue group or one of its recorded differences, and inspect the
persistent registry status with:

```bash
make pdfa-issue-draft PDFA_DIFFERENCE=unknown-filter-linearization-error
make pdfa-issue-status
```

The draft reads the affected Playa package, release, tag, and source commit
from [`compat/upstream.toml`](../compat/upstream.toml), cites the original test
file and expectation, and does not name this implementation. Issue groups in
the fixture manifest record all associated difference IDs, the last upstream
commit and date checked, and one of three states: `candidate`, `existing`, or
`submitted`. An `existing` or `submitted` record includes the GitHub issue URL
and prevents the draft command from generating the issue again. Multiple
differences caused by one upstream defect share one issue group and URL. Local
verification validates this registry without live GitHub access; a live search
of open and closed upstream issues is still required immediately before any
submission. The current
`UnknownFilter-PageContentStream.pdf` bytes contain a single `>` where the
stream dictionary requires `>>`; its behavior check therefore requires an
explicit parse failure rather than silently accepting empty content.

The current required projection contains normalized document open actions in
addition to catalog metadata, so `/OpenAction` is compared through the public
`Document.OpenAction` and `Action` accessors rather than only as a raw catalog
entry.

- document page count, page labels, PDF version, and Playa permission/tag flags
  (`is_tagged`, `is_printable`, `is_modifiable`, and `is_extractable`);
- document `info`, `catalog`, `names`, and `trailer` dictionaries, preserving
  indirect references as stable object-number markers;
- document indirect objects in Playa source order, retaining duplicate physical
  revisions and inserting compressed-object-stream members immediately after
  their containing stream. Dictionary values are compared structurally and
  stream bytes by length and SHA-256 while retaining only one object at a time;
  duplicate records are matched by `(object, generation)` buckets so the
  comparator can report value differences without depending on cache timing;
- the Mapping-style document view (`len`, `get`, `keys`, `values`, and `items`),
  with Playa's trailer-size `len`, XRef-revision ordering, repeated object
  numbers for incremental revisions, and streaming JSONL records so large
  object maps do not enlarge the metadata header;
- document source tokens in order, normalizing booleans to Playa's numeric
  token representation and preserving string bytes as hexadecimal values;
- document source buffer length and SHA-256 digest, avoiding a second complete
  buffer allocation in the Go projection;
- xref revisions in Playa order, including classic tables, xref streams,
  hybrid revisions, active entries, compressed-object positions, and trailers;
- selected page index, label, width, height, and rotation;
- flattened text objects (`chars`, bbox);
- default page text extraction output from tagged and untagged pages;
- direct page glyph sequence order and glyph geometry/font metadata;
- flattened interpreted content objects in recursive Form XObjects, including
  concrete kind, owning page index, bbox, CTM, marked-content stack, graphics
  state, MCID context, decoded text, and glyph count;
- direct interpreted content objects, retaining Form XObject nodes before
  recursive flattening;
- page content streams in source order by decoded length and SHA-256 digest;
- page content lexical tokens in source order;
- recursive Form XObjects by resource name, resources, transparency group,
  fonts, structure, device-space bbox, decoded stream digest, page ownership,
  CTM, graphics state, enclosing marked-content stack, and nested lexical
  tokens. Form resource dictionaries use a complete reference graph: the root
  preserves references, each reachable indirect object appears once in object-ID
  order, and streams retain their dictionaries plus decoded length and SHA-256.
  Binary strings are hex encoded. This avoids repeatedly expanding shared Form
  resources while still comparing every reachable node and stream payload;
- page and Form XObject content objects in source order, projected as lexical
  operands followed by their operator keyword;
- layout lines, text boxes, hierarchical text groups, and mixed `LTPage`
  children from the default Playa `LAParams` analysis, including text, bbox,
  writing direction, reading indexes, and ordered image/path/Form XObject
  items;
- glyph text, origin, displacement, and bbox;
- default, explicitly tagged, and explicitly untagged page text extraction;
- path segments, painting flags, device-space bbox, page ownership, CTM,
  graphics state, and marked-content context;
- image dimensions, bit depth, color space, filters, bbox, page ownership, CTM,
  graphics state, and marked-content context;
- page font names, metrics, flags, widths, writing direction, and bboxes;
- bounded ToUnicode source-code probes, including whether a source code has an
  explicit mapping and its mapped Unicode value;
- document-level font mapping with Playa's later-page-wins collision behavior;
- structure elements, roles, page association, attributes, content references, and children;
- page-level ParentTree structure views, including slot-associated elements,
  content kinds, attributes, and children;
- marked-content sections grouped by MCID with their aggregated text;
- marked-content points with their properties, page ownership, CTM, graphics
  state, and enclosing marked-content stack;
- page annotations, including selected-space bbox, page ownership, ParentTree
  structure association, normalized modification dates, normalized actions,
  and recursively resolved annotation properties;
- page ParentTree keys, preserving the distinction between an absent key and
  an explicit zero;
- content-object ParentTree associations for text, paths, images, tags, and
  Form XObjects;
- bounded font metric probes for horizontal/vertical displacement, position,
  and standard character bounding boxes;
- bounded font decoding probes comparing source code, CID, and Unicode text;
- named destinations, outline nodes, and normalized outline actions (including
  URI/file/name/script fields, raw dictionaries, and `/Next` chains);
- direct destination-array `/OpenAction` values and page-label rule switches
  across decimal, Roman, and alphabetic labels;
- AcroForm `NeedAppearances`, field values, flags, options, selected indices,
  widget rectangles, and field-tree children.

The font projection keeps Playa's `multibyte` compatibility field at its
oracle-defined `false` value through the public read-only `Font.IsMultibyte`
predicate. It must not be inferred from Go's internal CID font classifier:
Playa does not project that classifier into this field, and the CJK and
vertical-CID fixtures are required to retain the same result.

Run `make compat-fonts` for the focused font corpus check. It covers the CJK,
vertical-CID, and tagged-text fixtures across the configured coordinate spaces;
use `COMPAT_WORKERS` to bound concurrent jobs.

Run `make compat-recovery` for the recovery fixtures that both implementations
can open. The damaged-text and damaged-object-stream fixtures intentionally
exercise Go-only recovery beyond Playa's parser and are covered by Go recovery
tests rather than the Playa oracle comparison.

The page-level iteration contract is separate from the projection adapter.
Use `Page.Texts` or `Page.Glyphs` for lazy page traversal; use
`internal/testcompat` only when collecting a complete snapshot for the oracle.
The Go methods yield errors through `iter.Seq2` and stop when the consumer
returns `false`, matching Playa's generator behavior.

There are no remaining pending compatibility sections.

The `content.text` projection compares fields that have direct pinned-oracle
counterparts, including text/glyph codes, CID, font identity and size, text
and rendering matrices, origins, displacement, bbox, writing direction,
page ownership, graphics state, and marked-content association. The graphics
state projection normalizes Playa's implicit `Normal` blend mode and `Default`
black-point compensation defaults. Go-specific `Invisible` and `Unmapped`
glyph flags remain outside the Playa projection because the pinned oracle does not
expose equivalent fields; they are covered by Go API and glyph-rendering
tests.

Float values use an absolute tolerance of `1e-6`. Strings, array lengths, object order, and all other values must match exactly. The Playa oracle comparison accepts `page`, `screen`, and `default` coordinate spaces. The Go API also exposes `user` space, but the pinned Playa release does not accept `space="user"`, so it is covered by Go-side geometry tests rather than the Python oracle. The comparison defaults to `page` for the historical baseline; repeat `--space` to compare several supported spaces in one invocation.

The pinned oracle's hierarchical textbox heap uses `id(obj)` as the final key for
equal-distance pairs. The wheel's mypyc object addresses depend on allocator
history, so projecting one page alone and projecting it after earlier pages can
otherwise produce different, geometrically equivalent trees. The oracle loads
the pinned wheel's `miner.py` source for layout analysis and replaces only that
allocator key with stable per-page textbox/group creation ids; the Go miner uses
the same tie order. No group nodes are flattened or omitted: the complete
hierarchy, children, text, bboxes, and indexes are still compared exactly.
Heap distance keys are normalized to the comparator's `1e-6` geometric
precision before those stable ids are considered. This prevents sub-tolerance
Go/Python floating-point differences from selecting different equal-distance
trees while leaving every projected coordinate at its original precision.

The compatibility oracle tokenizes resolved page streams. This preserves the
intended `Page.tokens` behavior for valid pages whose `Contents` array contains
indirect references; the pinned oracle's private token walk otherwise attempts to
read `buffer` from those unresolved `ObjRef` values and aborts.

Page content traversal follows Playa's `stream_value` recovery: `null`, scalar,
and dangling-reference entries in a `Contents` array are skipped without
preventing later streams from being read. Object parsing also retains Playa's
PostScript-style `{...}` procedure containers, and an unterminated literal
string or procedure at final content-stream EOF discards only the unfinished
object. These rules apply consistently to the public `Content`, `Streams`,
`Tokens`, and `Contents` paths and to the compatibility projection.

Tagged extraction concatenates text objects within each marked-content section
without inserting spaces from baseline changes. Its line-state MCID comes from
the immediate marked-content context; public TextObject MCIDs still identify
the nearest enclosing numbered context. Marked-content `ActualText`
replaces text without advancing line-origin state, matching the pinned oracle.
Suppressed content still contributes line-origin state; when the existing
structure reading-order adaptation reorders sections, that incoming state stays
with its following visible section. Structure-element `ActualText` handling is
unchanged.

Playa snapshots are cached under `.compat-cache/` by default. The cache key includes the PDF content digest, pinned Playa package/version/tag/commit, schema version, selected pages, and coordinate space. Use `--no-cache` to force a fresh Playa run or `--cache-dir` to select another cache location. Cache files are local artifacts and are ignored by Git.

Every successful comparison section group also emits one machine-readable
`TIMING` JSON line for its Go phase. `--timing-mode=auto` tracks history
locally and switches to report-only whenever `CI` or `GITHUB_ACTIONS` is set;
the Make equivalent is `COMPAT_TIMING_MODE`. Local history lives under
`.compat-cache/timings/`, is ignored by Git, and is keyed by GOOS/GOARCH, CPU,
logical CPU count, `GOMAXPROCS`, Go version, pinned oracle commit,
compatibility cache version, worker count, and cache policy. Workloads are
independently keyed by PDF digest, selected pages, coordinate space, section
set, and password digest. A portable lock directory serializes each history
transaction across concurrent local processes. Locks are never stolen based on
age; if a process crashes while holding one, later runs warn and leave history
untouched until that `.lock` directory or the local timing directory is
removed.
Only successful single-worker cache-hit runs without CPU or heap profiling may
update a baseline. Once three prior non-warning samples exist, a run warns
only when it is both more than 35% and more than 500 ms slower than their
median. Warnings do not fail compatibility, warned observations never become
baseline samples, and at most five good samples are retained. `report` never
reads or writes history, `off` suppresses timing output, and deleting
`.compat-cache/timings/` resets only the local performance history. Corrupt or
newer-version history is reported and left untouched rather than affecting a
correctness result.

When the cache format is bumped, older JSONL entries are no longer reachable
from the current cache key. Review and remove only entries whose header can be
decoded and whose `cache_version` is older than the current driver with:

```bash
make compat-cache-prune
```

The command preserves current and future versions, malformed files, and legacy
`.json` caches that may still be convertible by the compatibility driver.

For focused Make-based diagnosis, `compat` and `compat-one` accept repeated
section names through `COMPAT_SECTIONS`; set `COMPAT_NO_CACHE=1` when changing
projection semantics:

```bash
COMPAT_SECTIONS=content.text COMPAT_NO_CACHE=1 \
  make compat-one COMPAT_PDF=testdata/files/acceptance_cjk_cid.pdf
COMPAT_SECTIONS=annotations make compat \
  COMPAT_PDFS=testdata/files/acceptance_navigation_semantics.pdf
```

Value projection alone cannot prove laziness. For every public API corresponding
to an upstream generator, add Go API tests that verify source order, a fresh
second traversal, consumer early-stop, and deferred delivery of a later-item
error. Keep a PDF fixture whose later page/object is invalid: consuming only
the earlier item must succeed, while consuming through the invalid item must
yield the matching error. These tests are part of compatibility acceptance,
alongside the Python-to-Go projection comparison.

## Running it

```bash
make compat
go run ./cmd/playa-compat --compare --section content.text --pdf testdata/files/form_simple.pdf
make compat-release
```

`make compat` submits every PDF/coordinate-space pair to one `playa-compat`
process. A shared scheduler removes both PDF and coordinate-space serial tails:
the automatic worker limit is `GOMAXPROCS`, while the independent memory budget
has a hard upper bound of 60% of detected physical RAM and always leaves at
least 2 GiB outside that bound, rounded down to 64 MiB. If physical-memory
detection is unavailable, the hard bound falls back to 3072 MiB. Before every
dispatch the scheduler also samples available system memory and Go runtime
memory that is committed and not yet released. The effective capacity is the
smaller of the hard bound and `available + runtime-committed - 2 GiB`: it can
safely admit more work when conservative task estimates exceed actual use, but
pauses new work when other applications consume the reserve. File-backed PDF
mappings are deliberately not added back because the OS availability estimate
may already count them as reclaimable. Tasks whose estimates fit the remaining capacity may pass
a blocked larger task, so CPU slots are not left idle. Go comparison phases and
external Python oracle phases are separately parallel but never overlap, which
avoids stacking both peak working sets. Set either Make variable to zero for
automatic selection (the default), or tune their upper bounds independently
with, for example, `COMPAT_WORKERS=6 COMPAT_MEMORY_LIMIT_MIB=6144 make compat`.
An explicit memory value does not disable the live system-reserve check.

Concurrency is the second optimization layer, not a substitute for efficient
single-job execution. The comparator first keeps each projection bounded and
avoids repeated work: large typed arrays are compared directly against one
borrowed JSON value at a time instead of decoding a second expected object;
document caches retain shared fonts, decoded streams, and parsed resources
across pages under their existing byte budgets; complete transient state is
released only at group and phase boundaries. Only after those costs are
bounded does the scheduler admit independent jobs in parallel.

Cached comparisons first expand every PDF/space/section-group key and fill all
missing Playa snapshots through the same scheduler. The subsequent comparison
phase is therefore Go-only instead of repeatedly alternating between one late
oracle cache miss and Go work. `--no-cache` keeps the direct alternating path.
At this idle phase boundary, automatic mode reports a fresh effective capacity;
dead preflight heap pages are returned to the OS before committed memory is sampled. Then
the live probe continues to raise or lower that capacity at every later
dispatch, so memory released by the oracle can immediately become comparison
parallelism without stacking the two workloads.

The memory value is a conservative scheduling estimate rather than an
OS-enforced allocation limit. A task whose own estimate exceeds the limit is
rejected instead of running outside the budget; use an explicit higher limit
only after checking system headroom. Each task still opens and closes its own
`Document`, and Playa snapshot cache writes use temporary files plus atomic
rename. Direct CLI calls may repeat `--space`; all resulting jobs share the
same `--memory-limit-mib` budget. The command prints its resolved scheduler
settings before starting comparisons.

### Comparator throughput check

The scheduler should be evaluated with reproducible public inputs. Prepare the
pinned open corpus, warm the oracle and filesystem cache, then compare the same
section in all three spaces with explicit and automatic worker limits:

```bash
make public-fixtures-pull
make public-fixtures-check
go build -o /tmp/go-playa-compat ./cmd/playa-compat
/usr/bin/time -lp /tmp/go-playa-compat --compare --workers 1 \
  --space page --space screen --space default --section layout \
  --pdf .compat-cache/public-corpus/riscv-unprivileged.pdf
/usr/bin/time -lp /tmp/go-playa-compat --compare --workers 0 \
  --space page --space screen --space default --section layout \
  --pdf .compat-cache/public-corpus/riscv-unprivileged.pdf
```

Compare counts and reported section results before comparing elapsed time and
peak RSS. `make compat` runs the configured checked-in and public corpus across
the required `page`, `screen`, and `default` spaces. The library workload benchmark is documented in
[`benchmark.md`](benchmark.md).

## Extending the projection

1. Add a failing Go test for the projected field in `internal/testcompat`.
2. Add the same field to `internal/testcompat.Snapshot` and the Playa snapshot
   projection in `scripts/compare_playa.py`.
3. Add representative PDFs that exercise it, such as
   `testdata/files/form_simple.pdf` for AcroForm coverage.
4. Run the full corpus before merging.

Never make the script ignore a mismatch merely to pass a gate. If a feature is deliberately unsupported, record it in `docs/migration.md`; it remains a failing compatibility case until implemented.

`cmd/playa-compat` and `internal/testcompat` are repository development tools.
They are not part of the go-playa library API and are distinct from the useful
end-user modes provided by `cmd/playa`.

The generated `testdata/files/acceptance_navigation_semantics.pdf` fixture is
an ordinary A4-style document that composes named destinations, an open action,
an outline, a URI annotation, page labels, and XMP metadata. Its document test and
the three-space compatibility checks ensure these lazy document-domain models
remain interoperable rather than only passing isolated unit tests.

For large documents, `cmd/playa-compat --jsonl` writes one document metadata
header followed by one page record per line instead of retaining a complete
JSON report. Combine it with `--memprofile path` to inspect the heap after the
stream has completed:

```bash
go run ./cmd/playa-compat --pdf .compat-cache/public-corpus/gnu-emacs.pdf \
  --jsonl --memprofile /tmp/playa-jsonl.heap.pprof > /tmp/playa.jsonl
```

The fixtures `testdata/files/malicious_cmap.pdf` and
`testdata/files/security_*.pdf` are intentionally excluded from the strict
Playa-output corpus. They contain oversized or malformed parser inputs and
are security regression fixtures: the Go implementation rejects or bounds
the malformed input, while the reference Playa version can produce a
different degraded projection. They remain covered by Go parser tests.
