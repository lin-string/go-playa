PDFA_FIXTURE_DIR ?= $(if $(GO_PLAYA_PDFA_FIXTURE_DIR),$(GO_PLAYA_PDFA_FIXTURE_DIR),../pdf-association-fixtures)
PUBLIC_CORPUS_MANIFEST ?= compat/public_corpus.json
PUBLIC_FIXTURE_DIR ?= $(if $(GO_PLAYA_PUBLIC_FIXTURE_DIR),$(GO_PLAYA_PUBLIC_FIXTURE_DIR),.compat-cache/public-corpus)
COMPAT_LOCAL_PDFS := $(filter-out testdata/files/malicious_cmap.pdf testdata/files/security_%.pdf testdata/files/recovery_malformed_%.pdf testdata/files/recovery_damaged_text.pdf testdata/files/recovery_damaged_object_stream.pdf testdata/files/acceptance_encrypted_%.pdf,$(wildcard testdata/files/*.pdf))
# In default mode the manifest helper appends each verified public PDF as one
# argument. An explicit COMPAT_PDFS override selects only those paths.
COMPAT_PDFS ?= $(COMPAT_LOCAL_PDFS)
COMPAT_SPACES ?= page screen default
COMPAT_BIN ?= /tmp/go-playa-compat
COMPAT_CACHE_DIR ?= .compat-cache
COMPAT_WORKERS ?= 0
COMPAT_MEMORY_LIMIT_MIB ?= 0
COMPAT_PASSWORD ?=
COMPAT_SECTIONS ?=
COMPAT_NO_CACHE ?=
COMPAT_TIMING_MODE ?= auto
COMPAT_FONT_PDFS ?= testdata/files/acceptance_cjk_cid.pdf testdata/files/acceptance_vertical_cid.pdf testdata/files/acceptance_tagged_text.pdf testdata/files/acceptance_macexpert_encoding.pdf
COMPAT_RECOVERY_PDFS ?= testdata/files/recovery_classic_to_xref_stream.pdf testdata/files/recovery_xref_stream_to_classic.pdf testdata/files/recovery_object_stream.pdf testdata/files/recovery_damaged_scan.pdf testdata/files/recovery_incremental_object_stream.pdf
BENCH_WORKERS ?= 4
BENCH_PRESET ?= reference
BENCH_OUTPUT ?=
BENCH_MATRIX_ARGS ?=
BENCH_CORPUS_MANIFEST ?= bench/corpus.json
BENCH_MATRIX_MANIFEST ?= bench/matrix.json
BENCH_CORPUS_DIR ?= .compat-cache/benchmark-corpus
GO ?= go
# Do not let a local verification run silently download another Go toolchain.
# Override this explicitly when testing against a different installed toolchain.
GOTOOLCHAIN ?= local
export GOTOOLCHAIN

# Both release copies exclude local residue independently of Git's ignore rules.
# Quote wildcard patterns so the shell leaves matching to rsync.
RSYNC_RELEASE_EXCLUDES := --exclude '.git/' --exclude '.compat-cache/' \
	--exclude '.venv/' --exclude '__pycache__/' --exclude '*.pyc' \
	--exclude '*.test' --exclude '.DS_Store' --exclude '.idea/' --exclude '.vscode/' \
	--exclude '*.swp' --exclude '*.tmp' --exclude '*.bak' --exclude '*~' \
	--exclude '*.pprof' --exclude '*.mprof' --exclude '*.out' \
	--exclude 'coverage.*' --exclude '*.coverprofile' --exclude 'profile.cov' \
	--exclude '/docs/superpowers/' --exclude '/.superpowers/'

.PHONY: fmt check-fmt lint test test-race vet tidy-check module-archive-check module-source-test pdfa-fixtures-check pdfa-fixtures-pull pdfa-corpus-test pdfa-issue-draft pdfa-issue-status public-fixtures-pull public-fixtures-check public-corpus-test compat-public api-audit verify resources-check recovery-fixtures recovery-test security-fixtures security-test acceptance-fixtures acceptance-test corpus-test compat compat-pdfa compat-fonts compat-recovery compat-one compat-release compat-cache-prune bench bench-sequential bench-concurrent bench-compare bench-profile go-cache-status go-cache-clean go-module-cache-clean
.PHONY: compat-local
.PHONY: release-metadata-test
.PHONY: engineering-check
.PHONY: vuln-go vuln-python vuln-check
.PHONY: bench-smoke bench-reference bench-scaling bench-dry-run bench-summarize
.PHONY: bench-corpus-pull bench-corpus-check bench-matrix-check bench-contract-test bench-contract-check

fmt:
	gofmt -w $$(find . -name '*.go' -type f -not -path './.git/*' -not -path './.compat-cache/*')

check-fmt:
	@test -z "$$(gofmt -l $$(find . -name '*.go' -type f -not -path './.git/*' -not -path './.compat-cache/*'))"

lint:
	golangci-lint run ./...

test:
	$(GO) test ./...

test-race:
	$(GO) test -race ./...

vet:
	$(GO) vet ./...

tidy-check:
	$(GO) mod tidy -diff

engineering-check:
	$(GO) run ./cmd/playa-engineering-check

# Fail on preparation errors and on any failed stage of the size-check pipes.
module-archive-check module-source-test: SHELL := /bin/bash

module-archive-check:
	@set -euo pipefail; tmp="$$(mktemp -d "$${TMPDIR:-/tmp}/go-playa-module-archive.XXXXXX")"; \
		trap 'rm -rf "$$tmp"' EXIT HUP INT TERM; \
		rsync -a $(RSYNC_RELEASE_EXCLUDES) ./ "$$tmp/"; \
		git init -q "$$tmp"; \
		git -C "$$tmp" add .; \
		attributes="$$(git -C "$$tmp" ls-files '*.pdf' | git -C "$$tmp" check-attr --stdin filter)"; \
		case "$$attributes" in *': lfs'|*': lfs'$$'\n'*) \
			echo "public repository PDF fixtures must not use Git LFS" >&2; \
			exit 1 ;; \
		esac; \
		test ! -e "$$tmp/testdata/files/go.mod" || \
			{ echo "testdata/files/go.mod must not recreate a nested module boundary" >&2; exit 1; }; \
		tree="$$(git -C "$$tmp" write-tree)"; \
		git -C "$$tmp" archive --format=zip --output="$$tmp/module.zip" "$$tree"; \
		archive_size="$$(wc -c <"$$tmp/module.zip" | tr -d ' ')"; \
		content_size="$$(cd "$$tmp" && git ls-files -z | xargs -0 cat | wc -c | tr -d ' ')"; \
		test "$$archive_size" -le 524288000 || \
			{ echo "Git source archive is $$archive_size bytes; Go permits at most 524288000" >&2; exit 1; }; \
		test "$$content_size" -le 524288000 || \
			{ echo "module file content is $$content_size bytes; Go permits at most 524288000" >&2; exit 1; }

module-source-test:
	@set -euo pipefail; tmp="$$(mktemp -d "$${TMPDIR:-/tmp}/go-playa-module-source.XXXXXX")"; \
		trap 'rm -rf "$$tmp"' EXIT HUP INT TERM; \
		rsync -a $(RSYNC_RELEASE_EXCLUDES) ./ "$$tmp/"; \
		cd "$$tmp" && $(GO) test ./...

pdfa-fixtures-pull:
	GO_PLAYA_PDFA_FIXTURE_DIR="$(PDFA_FIXTURE_DIR)" $(GO) run ./cmd/playa-fixtures --pdf-association

pdfa-fixtures-check:
	GO_PLAYA_PDFA_FIXTURE_DIR="$(PDFA_FIXTURE_DIR)" $(GO) run ./cmd/playa-fixtures --pdf-association --check

pdfa-corpus-test: pdfa-fixtures-check
	GO_PLAYA_PDFA_FIXTURE_DIR="$(PDFA_FIXTURE_DIR)" $(GO) test ./document -run PDFAssociation -count=1

pdfa-issue-draft:
	@test -n "$(PDFA_DIFFERENCE)" || (echo "PDFA_DIFFERENCE must name an upstream issue group or recorded difference" >&2; exit 2)
	$(GO) run ./cmd/playa-compat --issue-draft "$(PDFA_DIFFERENCE)"

pdfa-issue-status:
	$(GO) run ./cmd/playa-compat --issue-status

api-audit:
	@set -eu; api="$$(go doc -all github.com/lin-string/go-playa/document)"; \
		root_api="$$(go doc -all github.com/lin-string/go-playa)"; \
		if printf '%s\n' "$$api" | rg '^(type PageConcurrencyOptions([[:space:]]|=)|func DefaultPageConcurrencyOptions\(|func \([^)]*\) ForEachPageConcurrentWithOptions\()'; then \
			echo "legacy public page concurrency declarations must be absent" >&2; exit 1; \
		fi; \
		if printf '%s\n' "$$root_api" | rg '^(type PageConcurrencyOptions([[:space:]]|=)|func DefaultPageConcurrencyOptions\()'; then \
			echo "legacy root page concurrency declarations must be absent" >&2; exit 1; \
		fi
	@test ! -e concurrency || { echo "public concurrency package must be absent" >&2; exit 1; }
	! go doc github.com/lin-string/go-playa | rg 'Compatibility|ExtractOptions|PageObjects'
	! go doc github.com/lin-string/go-playa/content | rg 'ExtractOptions|PageObjects'
	! go doc github.com/lin-string/go-playa/page | rg 'PageResult'
	! go doc github.com/lin-string/go-playa/font.Font.ApplyEncoding >/dev/null 2>&1
	! go doc github.com/lin-string/go-playa/font.Font.ApplyDifferences >/dev/null 2>&1
	! go doc github.com/lin-string/go-playa.GraphicState >/dev/null 2>&1
	! go doc github.com/lin-string/go-playa/layout.Options.LineTolerance >/dev/null 2>&1
	! go doc github.com/lin-string/go-playa/layout.Options.WordGap >/dev/null 2>&1
	! go doc github.com/lin-string/go-playa/layout.Options.ParagraphGap >/dev/null 2>&1
	! go doc github.com/lin-string/go-playa/structure.MarkedContent >/dev/null 2>&1
	! go doc github.com/lin-string/go-playa/structure.Object >/dev/null 2>&1
	! go doc github.com/lin-string/go-playa/outline.OutlineNode >/dev/null 2>&1
	! go doc github.com/lin-string/go-playa/structure.StructElement >/dev/null 2>&1
	! go doc github.com/lin-string/go-playa/structure.StructureIndex >/dev/null 2>&1
	! go doc github.com/lin-string/go-playa/structure.StructureContent >/dev/null 2>&1
	! go doc github.com/lin-string/go-playa/structure.StructureContentKind >/dev/null 2>&1
	! go doc github.com/lin-string/go-playa/document.StructElement.Alt >/dev/null 2>&1
	! go doc github.com/lin-string/go-playa/document.StructElement.Abbreviation >/dev/null 2>&1
	! python3 scripts/generate_cff_resources.py --help | rg '^[[:space:]]+--source-dir([[:space:]]|$$)'

release-metadata-test:
	python3 -m unittest scripts/tests/test_release_metadata.py -v

# Explicit release gates: scanner bootstrap and advisory databases need network.
# govulncheck v1.7.0 supports the Go 1.25 baseline and queries the live advisory DB.
vuln-go:
	$(GO) run golang.org/x/vuln/cmd/govulncheck@v1.7.0 ./...

vuln-python:
	@set -eu; requirements="$$(mktemp "$${TMPDIR:-/tmp}/go-playa-vuln.XXXXXX")"; \
		trap 'rm -f "$$requirements"' EXIT HUP INT TERM; \
		uv export --frozen --no-dev --no-emit-project --project compat > "$$requirements"; \
		uvx --from pip-audit==2.10.1 pip-audit --requirement "$$requirements"

vuln-check: vuln-go vuln-python

verify: release-metadata-test check-fmt tidy-check engineering-check module-archive-check vet lint test module-source-test test-race api-audit public-corpus-test bench-contract-test bench-contract-check

public-fixtures-pull:
	python3 scripts/public_corpus.py --manifest "$(PUBLIC_CORPUS_MANIFEST)" --directory "$(PUBLIC_FIXTURE_DIR)" pull

public-fixtures-check:
	python3 scripts/public_corpus.py --manifest "$(PUBLIC_CORPUS_MANIFEST)" --directory "$(PUBLIC_FIXTURE_DIR)" check

public-corpus-test:
	python3 -m unittest scripts/tests/test_public_corpus.py

# Check checked-in upstream font/CMap inputs and generated Go tables without
# modifying the working tree. CI runs this against the live public sources.
resources-check:
	python3 -m unittest scripts/tests/test_generate_ccitt_tables.py scripts/tests/test_encoding_sources.py
	python3 scripts/sync_glyphlist_sources.py --check --output-dir scripts/data
	python3 scripts/sync_core14_afm.py --check --output-dir scripts/data/core14
	python3 scripts/sync_macexpert_encoding.py --check --output-dir scripts/data
	python3 scripts/sync_cmap_resources.py --check --output-dir fontdata/cmapdata
	python3 scripts/check_generated_resources.py

recovery-fixtures:
	python3 scripts/generate_recovery_fixtures.py

recovery-test:
	python3 scripts/generate_recovery_fixtures.py --check
	$(GO) test ./document -run 'TestRecoveryPDFFixtures(ResolvePages|RemainRepeatableAfterCacheRelease)' -count=1

security-fixtures:
	python3 scripts/generate_security_fixtures.py
	$(GO) run ./cmd/generate-security-fixtures

security-test:
	python3 scripts/generate_security_fixtures.py --check
	$(GO) run ./cmd/generate-security-fixtures --check
	$(GO) test ./document -run 'TestSecurityPDFFixturesDoNotPanicOrExpand|TestSecurityPDFFixtureNamesRemainStable|TestModernSecurityPDFFixturesDecryptPage|TestConfigureSecurityRejectsCyclicEncryptMetadataReference' -count=1

acceptance-fixtures:
	python3 scripts/generate_acceptance_fixtures.py

acceptance-test:
	python3 scripts/generate_acceptance_fixtures.py --check
	$(GO) test ./document -run 'Acceptance|SemanticAcceptance' -count=1

corpus-test:
	$(MAKE) security-test recovery-test acceptance-test pdfa-corpus-test

ifeq ($(origin COMPAT_PDFS),file)
compat compat-release bench bench-sequential bench-concurrent bench-compare bench-profile: public-fixtures-check
COMPAT_DRIVER = python3 scripts/public_corpus.py --manifest "$(PUBLIC_CORPUS_MANIFEST)" --directory "$(PUBLIC_FIXTURE_DIR)" compare --
BENCH_DRIVER = python3 scripts/public_corpus.py --manifest "$(PUBLIC_CORPUS_MANIFEST)" --directory "$(PUBLIC_FIXTURE_DIR)" run --
endif

compat:
	@test -n "$(COMPAT_PDFS)" || (echo "no PDF fixtures configured" >&2; exit 2)
	uv lock --check --project compat
	$(GO) build -o "$(COMPAT_BIN)" ./cmd/playa-compat
	$(COMPAT_DRIVER) "$(COMPAT_BIN)" --compare --workers "$(COMPAT_WORKERS)" --memory-limit-mib "$(COMPAT_MEMORY_LIMIT_MIB)" \
		--cache-dir "$(COMPAT_CACHE_DIR)" --timing-mode "$(COMPAT_TIMING_MODE)" \
		$(foreach space,$(COMPAT_SPACES),--space "$(space)") \
		$(if $(COMPAT_PASSWORD),--password "$(COMPAT_PASSWORD)") \
		$(foreach section,$(COMPAT_SECTIONS),--section "$(section)") \
		$(if $(COMPAT_NO_CACHE),--no-cache) \
		$(foreach pdf,$(COMPAT_PDFS),--pdf "$(pdf)")

compat-pdfa: pdfa-fixtures-check
	uv lock --check --project compat
	$(GO) build -o "$(COMPAT_BIN)" ./cmd/playa-compat
	GO_PLAYA_PDFA_FIXTURE_DIR="$(PDFA_FIXTURE_DIR)" "$(COMPAT_BIN)" --compare --pdf-association --no-cache \
		--workers "$(COMPAT_WORKERS)" --memory-limit-mib "$(COMPAT_MEMORY_LIMIT_MIB)" \
		--cache-dir "$(COMPAT_CACHE_DIR)" --timing-mode "$(COMPAT_TIMING_MODE)" \
		$(foreach space,$(COMPAT_SPACES),--space "$(space)")

# Explicit pulls keep ordinary verification offline. The helper passes each
# verified path as a separate argument, including directories containing spaces.
compat-public: public-fixtures-check
	uv lock --check --project compat
	$(GO) build -o "$(COMPAT_BIN)" ./cmd/playa-compat
	python3 scripts/public_corpus.py --manifest "$(PUBLIC_CORPUS_MANIFEST)" --directory "$(PUBLIC_FIXTURE_DIR)" compare -- \
		"$(COMPAT_BIN)" --compare $(if $(COMPAT_SECTIONS),,--release) \
		--workers "$(COMPAT_WORKERS)" --memory-limit-mib "$(COMPAT_MEMORY_LIMIT_MIB)" \
		--cache-dir "$(COMPAT_CACHE_DIR)" --timing-mode "$(COMPAT_TIMING_MODE)" \
		$(foreach space,$(COMPAT_SPACES),--space "$(space)") \
		$(if $(COMPAT_PASSWORD),--password "$(COMPAT_PASSWORD)") \
		$(foreach section,$(COMPAT_SECTIONS),--section "$(section)") \
		$(if $(COMPAT_NO_CACHE),--no-cache)

compat-fonts:
	$(MAKE) compat COMPAT_PDFS="$(COMPAT_FONT_PDFS)"

compat-local:
	$(MAKE) compat-release COMPAT_PDFS="$(COMPAT_LOCAL_PDFS)"

compat-recovery:
	$(MAKE) compat COMPAT_PDFS="$(COMPAT_RECOVERY_PDFS)"

compat-one:
	@test -n "$(COMPAT_PDF)" || (echo "COMPAT_PDF must name one PDF fixture" >&2; exit 2)
	uv lock --check --project compat
	$(GO) build -o "$(COMPAT_BIN)" ./cmd/playa-compat
	"$(COMPAT_BIN)" --compare --space "$(firstword $(COMPAT_SPACES))" \
		--workers "$(COMPAT_WORKERS)" --memory-limit-mib "$(COMPAT_MEMORY_LIMIT_MIB)" \
		--cache-dir "$(COMPAT_CACHE_DIR)" --timing-mode "$(COMPAT_TIMING_MODE)" \
		$(if $(COMPAT_PASSWORD),--password "$(COMPAT_PASSWORD)") \
		$(foreach section,$(COMPAT_SECTIONS),--section "$(section)") \
		$(if $(COMPAT_NO_CACHE),--no-cache) --pdf "$(COMPAT_PDF)"

compat-cache-prune:
	$(GO) build -o "$(COMPAT_BIN)" ./cmd/playa-compat
	"$(COMPAT_BIN)" --prune-cache --cache-dir "$(COMPAT_CACHE_DIR)"

compat-release:
	@test -n "$(COMPAT_PDFS)" || (echo "no PDF fixtures configured" >&2; exit 2)
	uv lock --check --project compat
	$(GO) build -o "$(COMPAT_BIN)" ./cmd/playa-compat
	$(COMPAT_DRIVER) "$(COMPAT_BIN)" --compare --release --workers "$(COMPAT_WORKERS)" --memory-limit-mib "$(COMPAT_MEMORY_LIMIT_MIB)" \
		--cache-dir "$(COMPAT_CACHE_DIR)" --timing-mode "$(COMPAT_TIMING_MODE)" \
		$(foreach space,$(COMPAT_SPACES),--space "$(space)") \
		$(if $(COMPAT_PASSWORD),--password "$(COMPAT_PASSWORD)") \
		$(foreach pdf,$(COMPAT_PDFS),--pdf "$(pdf)")

bench:
	BENCH_REPEATS=$(BENCH_REPEATS) BENCH_TASK=text-glyphs BENCH_WORKERS=1 $(BENCH_DRIVER) ./bench/run.sh $(COMPAT_PDFS)

bench-sequential:
	BENCH_TASK=text-glyphs BENCH_WORKERS=1 GOMAXPROCS=1 $(BENCH_DRIVER) ./bench/run.sh $(COMPAT_PDFS)

bench-concurrent:
	BENCH_TASK=text-glyphs BENCH_WORKERS=$(BENCH_WORKERS) $(BENCH_DRIVER) ./bench/run.sh $(COMPAT_PDFS)

bench-compare:
	$(MAKE) bench-sequential
	$(MAKE) bench-concurrent

bench-profile:
	$(GO) run ./bench/go -task text-glyph-jsonl -workers $(BENCH_WORKERS) -cpuprofile /tmp/go-playa-bench.pprof -memprofile /tmp/go-playa-bench.mprof $(firstword $(COMPAT_PDFS))

# New matrix runs have explicit input presets and never inherit legacy workers.
# Install the pinned oracle first; launchers use its frozen offline environment.
bench-smoke:
	python3 bench/run_matrix.py --preset smoke $(if $(BENCH_OUTPUT),--output "$(BENCH_OUTPUT)") $(BENCH_MATRIX_ARGS)

bench-reference:
	python3 bench/run_matrix.py --preset reference $(if $(BENCH_OUTPUT),--output "$(BENCH_OUTPUT)") $(BENCH_MATRIX_ARGS)

bench-scaling:
	python3 bench/run_matrix.py --preset scaling $(if $(BENCH_OUTPUT),--output "$(BENCH_OUTPUT)") $(BENCH_MATRIX_ARGS)

bench-dry-run:
	python3 bench/run_matrix.py --preset "$(BENCH_PRESET)" --dry-run $(BENCH_MATRIX_ARGS)

bench-summarize:
	@test -n "$(BENCH_OUTPUT)" || (echo "BENCH_OUTPUT must name an existing run directory" >&2; exit 2)
	python3 bench/summarize.py --directory "$(BENCH_OUTPUT)"

bench-corpus-pull:
	python3 scripts/benchmark_manifest.py --corpus "$(BENCH_CORPUS_MANIFEST)" --directory "$(BENCH_CORPUS_DIR)" pull

bench-corpus-check:
	python3 scripts/benchmark_manifest.py --corpus "$(BENCH_CORPUS_MANIFEST)" --directory "$(BENCH_CORPUS_DIR)" check

bench-matrix-check:
	python3 scripts/benchmark_manifest.py --corpus "$(BENCH_CORPUS_MANIFEST)" --matrix "$(BENCH_MATRIX_MANIFEST)" matrix-check

# These verification gates inspect manifests and checked-in inputs only.
bench-contract-check: bench-matrix-check
	python3 scripts/benchmark_manifest.py --corpus "$(BENCH_CORPUS_MANIFEST)" corpus-check

bench-contract-test:
	python3 -m unittest discover -s scripts/tests -p 'test_benchmark_*.py'

# Go's build cache is global and content-addressed; it has no supported project
# size limit. These targets make its lifecycle visible and explicit instead of
# deleting it on every test run and losing useful reuse across commands.
go-cache-status:
	@printf 'GOCACHE=%s\n' "$$($(GO) env GOCACHE)"
	@printf 'GOMODCACHE=%s\n' "$$($(GO) env GOMODCACHE)"
	@printf 'GOCACHE size: '; du -sh "$$($(GO) env GOCACHE)" 2>/dev/null | cut -f1 || true
	@printf 'GOMODCACHE size: '; du -sh "$$($(GO) env GOMODCACHE)" 2>/dev/null | cut -f1 || true

go-cache-clean:
	$(GO) clean -cache -testcache

# This is intentionally separate because GOMODCACHE is shared by every Go
# project using the same GOPATH. Run it only when those downloaded modules can
# be fetched again, or after reviewing `make go-cache-status`.
go-module-cache-clean:
	$(GO) clean -modcache
