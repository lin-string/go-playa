# Contributing to go-playa

The planned Playa migration is complete. Contributions now maintain
compatibility, harden producer support, improve performance, and evolve the
public API against the pinned oracle.

Read [AGENTS.md](../AGENTS.md) and these canonical English contracts before
changing code; English governs when a translation differs:

- [Engineering conventions](../docs/engineering.md)
- [Completed migration scope and behavioral contract](../docs/migration.md)
- [Compatibility oracle and corpus workflow](../docs/compatibility.md)
- [Public API audit](../docs/api-audit.md)

The oracle package, version, tag, and source commit are defined only in
[`compat/upstream.toml`](../compat/upstream.toml). Refer to that file instead of
copying its version into documentation or code comments. Report suspected
vulnerabilities through the [security policy](SECURITY.md).

## Prerequisites

Use Go 1.25 or newer, Git, Make, Bash, Python 3.10 or newer, curl, rsync, ripgrep (`rg`),
pre-commit, and golangci-lint compatible with [CI](workflows/ci.yml). Compatibility checks also
require uv and the locked environment in [`compat`](../compat/pyproject.toml).
Resource checks use curl for download fallback tests against a local HTTP server.
Make defaults to `GOTOOLCHAIN=local`; install the intended Go toolchain explicitly.
CI pins its patched Go toolchain in [`.go-version`](../.go-version).
From the repository root, install hooks once per clone:

```bash
pre-commit install
```

Do not bypass hooks. Keep dependencies light and prefer the Go standard library.
Preserve existing work, keep changes focused, and do not commit compatibility
caches, temporary snapshots, benchmark output, or other local generated caches.

## Behavior tests and public API

Start a behavior change with a focused failing test. Confirm it fails for the
intended reason, make the smallest implementation change, then rerun that test
and the relevant gates. Public API changes need focused external API coverage.
Every exported symbol must identify a Playa counterpart or a documented
necessary Go adaptation; do not retain old public spellings as compatibility
aliases.

Keep dependency-free values and algorithms in their owning public domain
packages. Document-aware parsing, resolution, caches, lifecycle, and
cross-domain coordination belong in `document`.

Preserve generators as fresh, repeatable `iter.Seq` or `iter.Seq2` values. Test
ordering, independent repeated traversal, immediate early stop, lazy parsing,
and errors deferred until the failing item is requested. Cache policy must not
change observable results. Do not replace a lazy sequence with an eager slice.

Borrowed values are read-only. Use `Finalize`, `Copy`, or the relevant named copy
accessor before retaining them across an iteration, callback, goroutine handoff,
or cache-release boundary. Respect the particular API's ownership contract.
After a successful open, arrange `defer doc.Close()`. Finish all readers and
page workers before `Close` or `ReleaseTransientCaches`; those lifecycle
operations are exclusive from document reads. Cover ownership and lifecycle
boundaries with tests, including race coverage where applicable.

## Verification

Run the baseline gate before declaring work complete or requesting review:

```bash
make verify
```

It covers formatting, module tidiness, `go vet`, lint, normal and race tests,
`make api-audit`, release metadata, and module archive/source checks. The focused
release metadata and relative Markdown link checks can also be run with:

```bash
python3 -m unittest scripts/tests/test_release_metadata.py -v
```

Before a release or after dependency changes, also run `make vuln-check`.
It separately checks Go code and the locked Python compatibility environment
against current advisory databases and requires network access. GitHub Actions runs only `make ci-basic`;
vulnerability scans remain an explicit local gate.

For extraction, page-model, projection, or other oracle-visible changes, run
the full relevant compatibility gate. `make compat-one` is for iteration only:

```bash
make compat-one COMPAT_PDF=testdata/files/form_simple.pdf
make public-fixtures-pull public-fixtures-check
make compat
```

The pull command explicitly downloads the manifest-pinned public corpus into
the ignored `.compat-cache/public-corpus` directory; it is not bundled in the Go
module. Ordinary tests and `make verify` do not download it and may skip optional
large-fixture tests if it is absent. Pull and check it before verification to
include that local coverage. Missing checked-in fixtures are failures.

For malformed-input, recovery, security, or cross-domain acceptance changes,
prepare the separately pinned PDF Association corpus and run:

```bash
make pdfa-fixtures-pull pdfa-fixtures-check
make corpus-test
```

For font or recovery changes, also use the corresponding focused oracle check:

```bash
make compat-fonts
make compat-recovery
```

Run `make compat-pdfa` when changing PDF Association behavior or oracle-visible
parsing. Document intentional oracle differences using the exact, reviewed
records required by the compatibility contract. Focused checks do not replace
the full relevant compatibility result before review.

## Generated resources and performance

Font, CMap, glyph-list, Core 14 AFM, CFF, and other generated resources must be
updated through their source synchronizers and generators. Never hand-edit
generated tables. Follow [mapping sources](../docs/mapping-sources.md), record
source URLs, revisions, and license impact in [NOTICE](../NOTICE), and preserve
the applicable terms in [THIRD_PARTY_LICENSES](../THIRD_PARTY_LICENSES). Run:

```bash
go generate ./document ./fontdata
make resources-check
```

Resource checks may contact declared upstream sources. Use the matching fixture
generators for intentional corpus updates and verify their drift checks.

Use profiles before changing behavior or cache policy for performance. Follow
the [benchmark protocol](../docs/benchmark.md), compare matching semantics and
inputs in fresh processes with comparable worker topology, and preserve result
counts and digests. Roughly 5x Playa throughput with lower peak memory is a
directional reference, not a universal gate. Quantify lower ratios or higher
memory and investigate avoidable work without weakening correctness checks.

## Pull requests

Use the [pull-request template](pull_request_template.md). Identify the Playa
counterpart or necessary Go adaptation, explain observable behavior and
compatibility impact, and list the exact verification commands and results.
Include iterator, ownership, lifecycle, resource provenance, and performance
evidence where relevant. State any command that could not run and why; a skipped
check is not a passing result.
