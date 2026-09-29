# Benchmark Protocol

English | [简体中文](benchmark.zh-CN.md)

## Published results

The reference and scaling results were recorded on 2026-09-29. See the
[complete dated matrices](benchmark-results-2026-09-29.md). The reference run
qualified 73 of 77 cells for ratios; scaling qualified 81 of 89. The remaining
cells are visible but blocked by stable
task-specific correctness mismatches. Checked-in smoke fixtures still verify
orchestration and correctness only; their tiny-input timings are not performance
evidence.

Performance comparisons must name their run date, machine, input, task, and
worker topology. The dated run used an interactive desktop rather than a
lab-isolated machine. Faster execution does not establish lower memory use.
The oracle package, version, tag, and source commit are defined only in
[`compat/upstream.toml`](../compat/upstream.toml); the environment artifact
records that contract and the installed oracle identity for each real run.

## Corpus and admission

[`bench/corpus.json`](../bench/corpus.json) is independent of the compatibility
corpus. Its nine pinned documents total 4754 pages and 720048399 bytes
(686.69 MiB), covering five document classes, six languages, page tiers
50-199, 200-499, 500-999, and 1000-plus, and all four declared size tiers.
Every current input is unencrypted. Only the Chinese manuscript exceeds 200 MiB.

| Fixture | Pages | Bytes | Type / text layer | Languages | Page tier / size tier | PDF / producer |
| --- | ---: | ---: | --- | --- | --- | --- |
| `riscv-unprivileged` | 696 | 4580174 | born-digital-graphics / native | en | 500-999 / under-10-mib | 1.4 / Asciidoctor PDF / Prawn |
| `gnu-emacs` | 804 | 3084965 | born-digital-text / native | en | 500-999 / under-10-mib | 1.7 / TeX / pdfTeX |
| `openintro-statistics` | 465 | 20947445 | mixed / native | en | 200-499 / 10-49-mib | 1.5 / macOS Quartz PDFContext / appended LaTeX |
| `shuying-siku-quanshu` | 460 | 528780070 | image-only-scan / none | zh-Hant | 200-499 / 200-mib-plus | 1.5 / PDFPatcher / iTextSharp |
| `muqaddimah-1900` | 596 | 87637786 | image-only-scan / none | ar | 500-999 / 50-199-mib | 1.6 / Internet Archive / LuraDocument PDF v2.65 |
| `iroha-jiruisho-japanese` | 119 | 37933566 | image-only-scan / none | ja | 50-199 / 10-49-mib | 1.5 / Adobe Acrobat 6.0 Image Conversion |
| `korean-school-reader` | 154 | 28525153 | ocr-scan / ocr | ko, zh-Hant | 50-199 / 10-49-mib | 1.6 / Adobe Acrobat 9 Image Conversion |
| `murat-coeur-fervent` | 170 | 4376195 | ocr-scan / ocr | fr | 50-199 / under-10-mib | 1.7 / ocrmypdf / Tesseract / pikepdf |
| `gnu-libc-manual` | 1290 | 4183045 | born-digital-text / native | en | 1000-plus / under-10-mib | 1.5 / Texinfo / TeX |

Arabic RTL and Chinese/Japanese vertical text in image-only scans describe
visible page content; those files do not exercise extracted RTL or vertical
glyph semantics. The Korean reader has OCR text on 149 of 154 pages; the French
scan has OCR on 133 of 170. OpenIntro is mixed native text, raster/vector
graphics, and heterogeneous producer content; it is not classified as scanned.
The corpus covers these concrete inputs, not every producer or PDF variation.

Admission requires an authoritative source page, explicit license or
public-domain basis, attribution, HTTPS download URL, exact bytes, SHA-256,
parsed page count, PDF version, encryption status, text layer, fonts, producer
family, and feature classification. Full pins and inspection notes live in the
manifest. Moving publisher URLs are allowed only with exact hash/size guards;
a changed download fails instead of silently updating the evidence.
The six added inputs were inspected and opened with Go and pinned Playa during
admission; untimed first/middle/last-page checks are admission evidence, not a
full-document equivalence claim.

| Fixture | Source and license basis | Attribution |
| --- | --- | --- |
| `riscv-unprivileged` | [Source](https://docs.riscv.org/reference/isa/v20260120/unpriv/unpriv-index.html); [CC-BY-4.0](https://creativecommons.org/licenses/by/4.0/) | RISC-V International and credited contributors; preserve page 20 author/license notices. |
| `gnu-emacs` | [Source](https://www.gnu.org/software/emacs/manual/); [GFDL-1.3-or-later with invariant sections and cover texts](https://www.gnu.org/licenses/fdl-1.3.html) | Free Software Foundation, 1985-2026; preserve embedded GFDL, GNU Manifesto, Distribution, GPL and cover notices. |
| `openintro-statistics` | [Source](https://www.openintro.org/book/os/); [CC-BY-SA-3.0](https://github.com/OpenIntroStat/openintro-statistics/blob/master/LICENSE.md) | David Diez, Mine Cetinkaya-Rundel, Christopher D. Barr / OpenIntro, 2019; retain original author/image credits and ShareAlike notices. |
| `shuying-siku-quanshu` | [Source](https://commons.wikimedia.org/wiki/File:%E6%9B%B8%E5%BD%B1%E5%9B%9B%E5%BA%AB%E5%85%A8%E6%9B%B8%E6%9C%AC.pdf); [CC0-1.0](https://creativecommons.org/publicdomain/zero/1.0/) | Zhou Lianggong; Siku Quanshu manuscript; Chinese University of Hong Kong Library / Shuge; Commons upload by Ghren. |
| `muqaddimah-1900` | [Source](https://commons.wikimedia.org/wiki/File:%D9%85%D9%82%D8%AF%D9%85%D8%A9_%D8%A7%D8%A8%D9%86_%D8%AE%D9%84%D8%AF%D9%88%D9%86_(%D8%A7%D9%84%D9%85%D8%B7%D8%A8%D8%B9%D8%A9_%D8%A7%D9%84%D8%A3%D8%AF%D8%A8%D9%8A%D8%A9%D8%8C_1900).pdf); [Public domain (PD-Lebanon / PD-old-100; pre-1931 publication)](https://creativecommons.org/publicdomain/mark/1.0/) | Ibn Khaldun (1332-1406), al-Matbaah al-adabiyah 1900 edition; digitization/source credited on Commons. |
| `iroha-jiruisho-japanese` | [Source](https://commons.wikimedia.org/wiki/File:WUL-ho02_00596_%E8%89%B2%E8%91%89%E5%AD%97%E9%A1%9E%E6%8A%84_1.pdf); [Public domain (PD-old-70; 1827 manuscript of a medieval work)](https://creativecommons.org/publicdomain/mark/1.0/) | Tachibana Tadakane; Mitsutomi's 1827 Kyoto manuscript; Waseda University Library ho02/ho02_00596; retain original collection notices. |
| `korean-school-reader` | [Source](https://commons.wikimedia.org/wiki/File:%E5%9C%8B%E6%B0%91%E5%B0%8F%E5%AD%B8%E8%AE%80%E6%9C%AC.pdf); [Public domain (PD-Art / PD-old-70)](https://creativecommons.org/publicdomain/mark/1.0/) | Joseon Ministry of Education editorial bureau (1895); National Library of Korea; Commons upload by AstrobluePeter (2016). |
| `murat-coeur-fervent` | [Source](https://commons.wikimedia.org/wiki/File:Murat_-_D%E2%80%99un_c%C5%93ur_fervent%2C_1908.pdf); [Public domain (PD-US-expired / PD-old-80; author died 1940)](https://creativecommons.org/publicdomain/mark/1.0/) | Amelie Murat (1882-1940), E. Sansot et Cie, Paris 1908; LeDeuxiemeTexte collection; OCR/crop by Commons contributor Cunegonde1 (2024). |
| `gnu-libc-manual` | [Source](https://sourceware.org/glibc/manual/latest/html_node/index.html); [GFDL-1.3-or-later with invariant sections and cover texts](https://www.gnu.org/licenses/fdl-1.3.html) | Free Software Foundation and GNU C Library manual contributors. |

Downloaded PDFs remain in `.compat-cache/benchmark-corpus`, outside the
distributed Go module. Retain original embedded author/image/collection notices
and the GNU manuals' invariant sections and cover texts if redistributing
those documents. The project MIT license does not cover these PDFs. Source and
license attribution is indexed in [NOTICE](../NOTICE); the manifest links to
full license terms rather than duplicating them. Large scans are not added to
`make compat`.

## Matrix presets

[`bench/matrix.json`](../bench/matrix.json) defines deterministic cells over
fixture × applicable task × requested workers. A sample is one implementation
in one fresh process; each repeat runs both implementations.

| Preset | Inputs | Requested workers | Repeats per implementation | Cells | Fresh samples |
| --- | --- | --- | ---: | ---: | ---: |
| `smoke` | Two checked-in, one-page text/image fixtures | 1, 2 | 1 | 14 | 28 |
| `reference` | All nine public PDFs | 1, 4 | 5 | 77 | 770 |
| `scaling` | One representative of each document class | 1, 2, 4, 8 | 5 | 89 | 890 |

Sequential tasks are explicitly listed in `SEQUENTIAL_TASKS` in
[`scripts/benchmark_manifest.py`](../scripts/benchmark_manifest.py).
Currently `objects` expands once per fixture with requested_workers=1, even
when a preset lists only larger worker counts. Page tasks use the listed
workers. Extend this topology contract for any new sequential task; sequential
rows are excluded from scaling views.

Scaling selects GNU Emacs, RISC-V, OpenIntro, Muqaddimah, and the Korean reader.
Expansion rejects missing reference coverage, missing/duplicate scaling
classes, duplicate cells, unsafe paths, and executable pending candidates.
Fixtures sort lexically, tasks follow the declared task order, and workers
ascend. Task applicability comes from feature tags, not filenames. Counts
above describe the current full presets; explicit filters produce smaller runs.

## Tasks and applicability

| Task | Applicable inputs | Required matching counts | Additional digest |
| --- | --- | --- | --- |
| `open-pages` | All | pages | None |
| `objects` | All | pages, objects | None |
| `text-glyphs` | `text` feature | pages, texts, glyphs | None |
| `layout-items` | `text` feature | pages, layout_lines, layout_items | None |
| `image-digests` | `images` feature | pages, images | Canonical decoded-image stream digest |
| `text-glyph-jsonl` | `text` feature | pages, texts, glyphs | Canonical text/glyph page-object projection digest |

Count-only tasks exclude JSON serialization of their extraction result.
`text-glyph-jsonl` includes serialization and hashing of its canonical
projection and is an end-to-end workload. Despite the `jsonl` suffix, the
digest input is canonical page JSON objects concatenated in page order with no
newline, length prefix, or other inter-record delimiter. It is not JSON Lines
framing. `image-digests` includes image
decoding and hashing, not rendered-pixel comparison. All sample runners emit
one JSON result line, independently of the measured task.

## Run commands

Run commands from the repository root. Preparation and offline planning:

```bash
make bench-contract-test bench-contract-check
make bench-matrix-check
make bench-dry-run BENCH_PRESET=reference
make bench-dry-run BENCH_PRESET=scaling
```

Dry-run prints cells, sample commands, and manifest digests. It builds nothing,
launches no sample process, parses no PDF, and creates no run directory.
Download and install dependencies explicitly before the next real run:

```bash
uv sync --frozen --project compat
make bench-corpus-pull
make bench-corpus-check
```

`make bench-corpus-pull` delegates to `python3 scripts/benchmark_manifest.py pull`;
`make bench-corpus-check` delegates to `python3 scripts/benchmark_manifest.py check`.
`pull` downloads and verifies exact bytes/SHA-256; `check` verifies existing
local files offline. Manifest checks validate metadata and local fixture
hashes. Before a real run, the launcher additionally opens each selected PDF
with the pinned oracle in an untimed preflight to verify its parsed page count.
Oracle sample launches use `uv run --offline --frozen --project compat` so
measurement does not resolve or download dependencies.
Benchmark execution and its contract suite remain local gates. `make verify`
runs the benchmark Python contract tests and offline corpus/matrix metadata
checks; these inspect only manifests and checked-in fixture identities and do
not download benchmark inputs or execute a benchmark.

The following commands reproduce the procedure. Choose a new empty directory
for every run:

```bash
make bench-smoke BENCH_OUTPUT=.compat-cache/benchmarks/next-smoke
make bench-reference BENCH_OUTPUT=.compat-cache/benchmarks/next-reference
make bench-scaling BENCH_OUTPUT=.compat-cache/benchmarks/next-scaling
make bench-summarize BENCH_OUTPUT=.compat-cache/benchmarks/next-reference
```

Optional explicit selection, still subject to applicability and full gates:

```bash
make bench-dry-run BENCH_PRESET=reference \
  BENCH_MATRIX_ARGS="--fixture gnu-emacs --task text-glyphs --worker 1"
python3 bench/run_matrix.py --preset reference \
  --fixture gnu-emacs --task text-glyphs --worker 1 \
  --output .compat-cache/benchmarks/next-filtered
python3 bench/summarize.py --directory .compat-cache/benchmarks/next-filtered
```

Filters `--fixture`, `--task`, and `--worker` are repeatable.
`--corpus-directory` selects the download directory; `--go-binary` reuses a
prebuilt sample binary. Avoid other machine workloads, record filesystem-cache
conditions, and use consistent conditions throughout a run. The protocol does
not automatically warm or flush filesystem caches. Review complete validated
tables and generated evidence views before copying any dated result into README.

## Timing and worker topology

Every Go/Playa sample has a fresh process and opens its PDF exactly once.
`elapsed_ns` begins immediately before opening the document and ends after
close completes, including parsing, extraction, worker setup/teardown, and
task serialization/hashing where applicable. Builds, dependency installation,
preflight, environment capture, result printing, and post-close RSS/allocation
bookkeeping are outside that boundary. Timers are monotonic. Odd repeats run
Go then Playa; even repeats run Playa then Go.

`requested_workers` is the explicit command limit.
`effective_workers` is the native scheduled limit after CPU/page constraints.
`observed_workers` is maximum simultaneous callbacks in Go, but distinct
callback process IDs in Playa; these observations have different meanings.
Go uses goroutines over one shared document, sets `GOMAXPROCS` to the request,
and caps effective page workers at ceil(pages/10). Playa CPU-bounds the request
and follows its pinned native page loading/pool thresholds. Its result uses
actual callback PID evidence: foreign callback PIDs confirm the configured pool
limit; callbacks confined to the parent report effective_workers=1. This avoids
inferring pool participation from a post-run page count. `objects` requests one
worker and always has effective/observed workers 1. All workers finish before
document close.

A differing effective topology retains timing rows but blocks the ratio with
`speedup: null` and `effective-worker-mismatch`. Sequential matrix cells
explicitly request 1 regardless of ambient Make worker settings.
`bench-sequential` also forces `BENCH_WORKERS=1`; use the matrix presets for
publishable evidence.

`peak_rss_bytes` is a lifetime process peak only when complete process scope is
available. Go reports one process peak. Playa reports its post-close parent
peak with `rss_scope: single-process-lifetime-peak` only when all callbacks ran
in the parent. That single-process case supports a direct process comparison.
Zero means unavailable, not zero memory.

When callbacks run in worker processes, Playa reports `peak_rss_bytes=0` and
`rss_scope: parent-lifetime-and-worker-callback-lower-bound`. The separate
`parent_peak_rss_bytes` is sampled after close; `callback_worker_peak_rss_sum_bytes`
sums the largest callback-time observation for each participating worker.
Callbacks sample after task encoding but before return/IPC serialization;
they can miss later IPC, cleanup, or unobserved-worker peaks. This diagnostic
sum is not a complete lifetime peak and is not simultaneous aggregate memory.
The public page-map API exposes no individual worker exit rusage. Worker
callback sampling is inside task timing; parent post-close sampling is outside.
No cross-library memory ratio or lower-memory claim is permitted from these
incomparable multi-process scopes. Summaries mark complete Playa RSS unavailable
and keep lower-bound diagnostics separately labeled.

Go `alloc_bytes` is cumulative allocation during the sample, not peak memory.
Cache retention can increase RSS and benefit later reads; use profiles to
investigate, and qualify each measured memory result.

## Correctness gates

No speedup is published unless byte/hash identity, parsed page count, complete
sample ordering and count, exact run/fixture/manifest identities, and matching
task signatures pass. Required count fields must be explicit nonnegative
integers; missing fields are not interpreted as zero. Duplicate JSON keys,
non-finite values, extra stdout, malformed worker topology, and nonpositive
timing fail. Repeat signatures must be stable within each implementation and
across workers. Insufficient samples, implementation-internal signature drift,
or stale corpus/matrix digests reject summary generation.

A stable Go/Playa cross-implementation signature mismatch produces a
`partial-correctness-mismatch` summary instead of discarding every other cell.
The affected fixture/task is retained with both signatures,
`task-specific-correctness-mismatch`, and null cross-library ratios for every
worker count. Aggregates containing it are also blocked, as are both
same-implementation scaling ratios. The summary files are written, but the CLI
returns nonzero so automation cannot treat a partial run as fully qualified.
Counts alone establish `count-equivalence`, not complete semantic equivalence.

`text-glyph-jsonl` additionally compares the SHA-256 of the ordered,
delimiter-free concatenation of canonical page JSON objects
(`canonical-digest-and-counts`). Geometry uses a 0.01 PDF
user-unit decimal grid and an explicit midpoint rule so independent runtimes do
not split on floating-point ULP noise. `image-digests` uses
`canonical-image-digest-and-counts`: the aggregate SHA-256 starts with
`playa-image-digests-v1` followed by NUL. Each image contributes one 56-byte
frame: zero-based page index, zero-based image index on that page, and decoded
stream length as three big-endian uint64 values, followed by 32 raw SHA-256
bytes of the decoded image stream. Frames follow page/image order, regardless
of worker completion order. Only compact frames are retained, not image data.
The digest validates this decoded-stream projection, not rendering or complete
image semantics. Missing or differing digests fail repeats, worker comparisons,
and cross-library comparisons.

## Artifacts and statistics

Each new `.compat-cache/benchmarks/<run-id>/` contains:

| Artifact | Contents |
| --- | --- |
| `raw.jsonl` | Immutable samples with input/run hashes, implementation_source_sha256, benchmark_binary_sha256, implementation_digest, workers, counts/digests, elapsed_ns, RSS scope/diagnostics, and Go alloc_bytes |
| `environment.json` | Machine/runtime/oracle data, implementation identity, timing/RSS scope, date, filters, cells, and exact fixture metadata |
| `summary.json` | Validated implementation identity, per-cell aggregates, and deterministic evidence views |
| `summary.md` | Implementation provenance, single-worker/scaling/dimension views, and the complete qualified matrix |

The implementation identity records `git_commit` when available, `git_dirty`,
`source_sha256`, `source_file_count`, and `sha256-path-mode-content-v1`.
The source fingerprint frames relative path, file kind/executable bit, byte
length, and content hash for sorted non-ignored repository files. Tracked
deletions and symlink targets also affect it. Caches, benchmark outputs, the
chosen binary, and explicitly selected output/corpus directories are excluded.
The fingerprint remains meaningful for dirty checkouts and unborn repositories;
Git metadata is nullable when unavailable.

Binary provenance records the exact absolute executed path, bytes, SHA-256,
origin and local build command. `--go-binary` records `supplied-binary` and no
build command; its checkout fingerprint is context, not a claim about the
binary's source. Raw records bind the complete identity through
`implementation_digest`; summaries validate those bindings and expose them.
If the recorded executable still exists, summarization verifies its bytes/hash.
Archived evidence can be summarized without retaining the executable.

The launcher refuses nonempty output directories and exclusively creates
raw/environment files. Summarization preserves them and revalidates current
manifests, selected filters, saved inputs, sample ordering, and all signatures.
Each summary file is flushed, fsynced, and atomically replaced; the pair is not
a single transaction. No raw benchmark output, downloaded PDF, or generated
cache is committed.

Per implementation and cell, report elapsed median, P25/P75, median lifetime
peak RSS, and pages/s = pages × 1e9 / median elapsed_ns. Quantiles linearly
interpolate sorted samples at (n-1)*p. Speedup is Playa median elapsed divided
by Go median elapsed, only for equal effective workers. Five repeats give a
small-sample distribution, not a confidence interval. A preset row must not
be generalized to unmeasured PDFs, tasks, or machines; smoke has one repeat and
does not support a throughput claim.

Top-level summaries expose `summary_status`, `qualified_cell_count`,
`blocked_cell_count`, and `correctness_mismatch_cell_count`. Each cell records
`correctness_status`, both `task_signatures`, `mismatch_fields`, and explicit
`speedup_blocked_reasons`; matching-only `counts` and `sha256` are null on a
cross-implementation mismatch.

Generated JSON `views` contains `single_worker_tasks`, `scaling`,
`by_document_type`, `by_language`, and `by_page_tier`, with matching Markdown
tables. Task comparisons use requested_workers=1. Scaling lists workers 1/2/4/8
for each page-task/fixture, labels unmeasured settings, and compares each
implementation against its own one-worker median. Missing baselines leave
scaling unavailable; effective topology remains visible in the complete table.

Dimension aggregates never combine different tasks or requested worker counts.
Within each such group they sum per-fixture median elapsed times and pages;
throughput is total pages divided by the summed medians. The ratio of summed
Playa/Go medians is emitted only if every constituent topology matches. These
are descriptive aggregates, not separately measured combined workloads or
means of cell speedups. Fixture membership, blocked cell counts and correctness
scope are explicit. Multi-language documents contribute in full to each language
group, so language groups overlap. Views derive no cross-library memory ratio
or universal performance claim and retain all per-cell count/digest qualifications.
