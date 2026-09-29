# Agent Instructions

## Repository status

The planned Playa migration is complete. Current work is compatibility
maintenance, producer hardening, performance work, and normal API evolution
against the pinned oracle. Do not describe the repository as being in an
unfinished migration phase.

Use these English documents as the canonical project contracts:

- [Engineering conventions](docs/engineering.md)
- [Migration contract and completed scope](docs/migration.md)
- [Compatibility oracle and corpus workflow](docs/compatibility.md)
- [Public API audit](docs/api-audit.md)

The oracle package, version, tag, and source commit are defined only in
[`compat/upstream.toml`](compat/upstream.toml). Reference that file instead of
copying the version into documentation or code comments.

## Workspace and tool safety

- Work in the current checkout and branch unless the user explicitly requests
  a branch or worktree. Preserve all existing user changes and avoid unrelated
  edits.
- If an operation can reasonably be expected to fail under the sandbox—for
  example writing to a global directory, using Go's global caches, starting a
  GUI, downloading dependencies, accessing restricted networks, or writing
  outside the workspace—request elevated permission on the first attempt.
- Explain and obtain confirmation before destructive or externally visible
  actions such as deleting or overwriting substantial data, resetting Git
  history, force-pushing, or publishing data to an external service.
- Do not commit unless the user asks. Never commit `.compat-cache`, temporary
  snapshots, benchmark output, or local generated caches.

## Implementation contract

- Every exported symbol must map to a Playa API or a documented necessary Go
  adaptation. Keep Playa's domain nouns and observable behavior while using
  idiomatic Go naming and control flow. Do not retain legacy public spellings
  as compatibility aliases.
- Preserve generator behavior as fresh, repeatable `iter.Seq` or
  `iter.Seq2` values. Ordering, early stop, deferred errors, lazy parsing,
  cache behavior, and `Finalize`/`Copy` ownership are public semantics; never
  replace them silently with eager slices.
- Keep `Close` and `ReleaseTransientCaches` exclusive from document reads.
  Borrowed values must be finalized or copied before they outlive an
  iteration, callback, goroutine handoff, or cache-release boundary.
- Keep dependency-free values and algorithms in their owning public domain
  packages. Document-aware parsing, resolution, caching, lifecycle, and
  cross-domain coordination belong in `document`.

## Tests and required gates

- Write a failing behavior test before changing public behavior. Add focused
  external API coverage and iterator/ownership tests where applicable.
- Run `make verify` before completion. It includes formatting, module
  tidiness, `go vet`, lint, normal tests, race tests, and `make api-audit`.
- Run `make compat` for public extraction, page-model, projection, or other
  oracle-visible changes. Use `make compat-one` only while iterating; record
  the full relevant compatibility result before review.
- Run `make corpus-test` for malformed input, recovery, security, or
  cross-domain acceptance behavior. Use `make compat-fonts` and
  `make compat-recovery` for the focused Playa comparisons.
- Font, CMap, glyph-list, Core 14 AFM, CFF, and other generated resource
  changes must use their source synchronizers and generators. Never hand-edit
  generated tables; run `make resources-check` and record source provenance
  and license impact in `NOTICE`.

## Performance goals

- For representative CPU-heavy public workloads, treat roughly 5x Playa's
  throughput with lower peak memory as a directional optimization reference,
  not a universal acceptance gate or correctness requirement. Always compare
  matching semantics, inputs, fresh processes, and comparable worker topology.
- Quantify and document lower ratios or higher memory. Fixed startup cost,
  I/O or compression, native-library kernels, different concurrency models,
  and measurement noise can be valid explanations; avoidable Go-controlled
  copying, allocation, parsing, or lock contention is an optimization target,
  not an exception by itself.
- Use profiles before changing behavior or cache policy. Preserve matching
  result counts and digests, and distinguish direct one-process RSS comparisons
  from aggregate multi-worker measurements. A benchmark warning prompts
  investigation; it must not weaken correctness or compatibility checks.

Pull requests must identify the Playa counterpart or Go adaptation, describe
behavior and compatibility impact, and list exact verification commands. Use
the checklist in [`.github/pull_request_template.md`](.github/pull_request_template.md).
