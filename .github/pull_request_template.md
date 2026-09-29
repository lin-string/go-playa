## Alignment scope

- Playa module/API counterpart or documented Go adaptation:
- Go package and public symbols affected:
- Compatibility manifest section(s), if any:

## Behavior and ownership

- [ ] Iterator/generator ordering and repeatability are covered.
- [ ] Early termination and deferred errors are covered where applicable.
- [ ] Borrowed values have an explicit `Finalize`/`Copy` path when retention is supported.
- [ ] Any intentional difference from Playa is documented.

## Resource provenance

- [ ] No generated font/CMap table was hand-edited.
- [ ] Source URL/repository, revision, and license impact are documented for resource changes.
- [ ] `make resources-check` passes when resource or generated data changed.

## Verification

- [ ] `make verify`
- [ ] `make api-audit` (included in `make verify`)
- [ ] `make vuln-check` for dependency changes or release preparation
- [ ] `make corpus-test` when touching malformed-input, recovery, or cross-domain behavior
- [ ] `make compat-pdfa` when PDF Association behavior or oracle-visible parsing changes
- [ ] `make public-fixtures-pull public-fixtures-check` before public large-PDF checks
- [ ] `make compat` or focused `make compat-one` plus the full compatibility run
- [ ] Relevant benchmark or memory check, if performance-sensitive

Notes, limitations, and exact commands:
