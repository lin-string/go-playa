from __future__ import annotations

import re
import json
import os
import shutil
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path
from urllib.parse import unquote


ROOT = Path(__file__).resolve().parents[2]

PROJECT_MIT_LICENSE = """MIT License

Copyright (c) 2026 lin-string <linshijun.string@gmail.com>

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
"""

PLAYA_LICENSE = """Copyright (c) 2024 David Huggins-Daines <dhd@ecolingui.ca>

Permission is hereby granted, free of charge, to any person
obtaining a copy of this software and associated documentation
files (the "Software"), to deal in the Software without
restriction, including without limitation the rights to use,
copy, modify, merge, publish, distribute, sublicense, and/or
sell copies of the Software, and to permit persons to whom the
Software is furnished to do so, subject to the following
conditions:

The above copyright notice and this permission notice shall be
included in all copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY
KIND, EXPRESS OR IMPLIED, INCLUDING BUT NOT LIMITED TO THE
WARRANTIES OF MERCHANTABILITY, FITNESS FOR A PARTICULAR
PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE AUTHORS OR
COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR
OTHERWISE, ARISING FROM, OUT OF OR IN CONNECTION WITH THE
SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.

Contains code from pdfminer.six, which is:

Copyright (c) 2004-2016  Yusuke Shinyama <yusuke at shinyama dot jp>

Permission is hereby granted, free of charge, to any person
obtaining a copy of this software and associated documentation
files (the "Software"), to deal in the Software without
restriction, including without limitation the rights to use,
copy, modify, merge, publish, distribute, sublicense, and/or
sell copies of the Software, and to permit persons to whom the
Software is furnished to do so, subject to the following
conditions:

The above copyright notice and this permission notice shall be
included in all copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY
KIND, EXPRESS OR IMPLIED, INCLUDING BUT NOT LIMITED TO THE
WARRANTIES OF MERCHANTABILITY, FITNESS FOR A PARTICULAR
PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE AUTHORS OR
COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR
OTHERWISE, ARISING FROM, OUT OF OR IN CONNECTION WITH THE
SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.
"""

COMBINED_LICENSE = (
    PROJECT_MIT_LICENSE
    + "\nPortions derived from Playa retain the following license notice:\n\n"
    + PLAYA_LICENSE
)

DEPENDABOT_CONFIG = """version: 2
updates:
  - package-ecosystem: gomod
    directory: /
    schedule:
      interval: weekly
    open-pull-requests-limit: 5

  - package-ecosystem: github-actions
    directory: /
    schedule:
      interval: weekly
    open-pull-requests-limit: 5

  - package-ecosystem: pip
    directory: /compat
    schedule:
      interval: weekly
    open-pull-requests-limit: 5
"""

def repository_files() -> list[Path]:
    # Include publishable untracked files so the contract also works before
    # the initial commit. Git's ignore rules keep local caches out of the scan.
    result = subprocess.run(
        ["git", "ls-files", "-z", "--cached", "--others", "--exclude-standard"],
        cwd=ROOT,
        check=True,
        capture_output=True,
    )
    return sorted({ROOT / name.decode() for name in result.stdout.split(b"\0") if name})


def public_markdown_files() -> list[Path]:
    files: list[Path] = []
    for path in repository_files():
        if path.suffix.lower() != ".md" or not path.is_file():
            continue
        relative = path.relative_to(ROOT)
        if any(part in {".git", ".compat-cache", ".venv", "__pycache__"} for part in relative.parts):
            continue
        if relative.parts[:2] == ("docs", "superpowers"):
            continue
        if relative.parts[0] == ".superpowers":
            continue
        files.append(path)
    return sorted(files)


def public_text_files() -> list[Path]:
    files: list[Path] = []
    for path in repository_files():
        if not path.is_file():
            continue
        relative = path.relative_to(ROOT)
        if any(part in {".git", ".compat-cache", ".venv", "__pycache__"} for part in relative.parts):
            continue
        if relative.parts[:2] == ("docs", "superpowers"):
            continue
        if relative.parts[0] == ".superpowers":
            continue
        data = path.read_bytes()
        if b"\0" in data:
            continue
        try:
            data.decode("utf-8")
        except UnicodeDecodeError:
            continue
        files.append(path)
    return sorted(files)


class ReleaseMetadataTests(unittest.TestCase):
    def test_readmes_lead_with_qualified_performance_and_early_example(self) -> None:
        aligned_sections = (
            ("Why go-playa", "为什么选择 go-playa"),
            ("Performance evidence", "性能证据"),
            ("Install and open a document", "安装并打开文档"),
            ("CID-aware glyph rendering", "CID-aware 字形渲染"),
            ("Alignment baseline", "对齐基线"),
            ("Playa-to-Go API mapping", "Playa 到 Go 的 API 映射"),
            ("Public packages", "公开包"),
            ("Laziness and ownership", "惰性与所有权"),
            ("Concurrency guarantees", "并发保证"),
            ("Command-line tool", "命令行工具"),
            ("License and independence", "许可证与项目独立性"),
            ("Development", "开发"),
        )
        translations = (
            ("README.md", "Why go-playa", "Performance evidence", "Install and open a document", "CID-aware glyph rendering", "High-throughput", "Native Go", "lazy", "auditable", "docs/benchmark.md", "docs/benchmark-results-2026-09-29.md"),
            ("README.zh-CN.md", "为什么选择 go-playa", "性能证据", "安装并打开文档", "CID-aware 字形渲染", "高吞吐", "Go 原生", "惰性", "可审计", "docs/benchmark.zh-CN.md", "docs/benchmark-results-2026-09-29.zh-CN.md"),
        )
        for column, (filename, why, evidence, install, cid, performance, native, lazy, audit, protocol, next_run) in enumerate(translations):
            with self.subTest(document=filename):
                text = (ROOT / filename).read_text()
                intro = text.split("## ", 1)[0]
                for phrase in (performance.lower(), native.lower(), lazy, audit):
                    self.assertIn(phrase, intro.lower())
                first_reason = re.search(r"(?m)^- (.*)$", text).group(1)
                self.assertIn(performance.lower(), first_reason.lower())
                headings = re.findall(r"(?m)^## (.+)$", text)
                self.assertEqual(headings[:4], [why, evidence, install, cid])
                self.assertEqual(headings, [section[column] for section in aligned_sections])
                self.assertIn(protocol, text[:text.index("## " + install)])
                self.assertIn("package main", text[text.index("## " + install):text.index("## " + cid)])
                for phrase in ("bench/corpus.json", "bench/matrix.json", next_run):
                    self.assertIn(phrase, text)
                self.assertNotRegex(text, r"2\.9\s*[–-]\s*4\.1|1\.45 s|5\.88 s|4\.1[x×]|2\.9[x×]")
                self.assertNotRegex(text.lower(), r"always faster|universally faster|guaranteed throughput|guaranteed lower memory|始终更快|普遍更快|保证更低内存")
                for phrase in ("2026-09-29", "3.36", "5.56", "73", "77", "81", "89"):
                    self.assertIn(phrase, text)
                boundaries = (("interactive desktop", "blocked", "does not imply lower peak rss"),
                              ("交互式桌面", "阻断", "不能直接推导出更低峰值 rss"))
                for phrase in boundaries[column]:
                    self.assertIn(phrase, text.lower())
                self.assertNotRegex(text.lower(), r"2026-09-27|superseded|first current-protocol|旧的.*快照|已被替代|首份")

    def test_benchmark_protocols_share_structure_dimensions_and_boundaries(self) -> None:
        headings = (
            ("Published results", "Corpus and admission", "Matrix presets", "Tasks and applicability", "Run commands", "Timing and worker topology", "Correctness gates", "Artifacts and statistics"),
            ("已发布结果", "语料与准入", "矩阵 preset", "任务与适用条件", "运行命令", "计时与 worker 拓扑", "正确性门禁", "产物与统计"),
        )
        corpus = json.loads((ROOT / "bench/corpus.json").read_text())["fixtures"]
        common = ("../bench/corpus.json", "../bench/matrix.json", "../compat/upstream.toml",
                  "28", "770", "890", "77", "89", "14", "4754", "720048399",
                  "make bench-smoke", "make bench-reference", "make bench-scaling", "make bench-dry-run", "make bench-summarize",
                  "benchmark_manifest.py pull", "benchmark_manifest.py check", "BENCH_MATRIX_ARGS", "BENCH_OUTPUT",
                  "raw.jsonl", "environment.json", "summary.json", "summary.md", "elapsed_ns", "peak_rss_bytes", "alloc_bytes",
                  "requested_workers", "effective_workers", "observed_workers", "effective-worker-mismatch",
                  "SEQUENTIAL_TASKS", "source_sha256", "git_commit", "git_dirty", "source_file_count",
                  "benchmark_binary_sha256", "implementation_digest", "supplied-binary",
                  "single_worker_tasks", "by_document_type", "by_language", "by_page_tier",
                  "make bench-corpus-pull", "make bench-corpus-check", "make bench-matrix-check", "make bench-contract-test bench-contract-check",
                  "playa-image-digests-v1", "56", "uint64", "SHA-256", "P25", "P75", "(n-1)*p",
                  "count-equivalence", "canonical-image-digest-and-counts", "canonical-digest-and-counts")
        for filename, expected in zip(("docs/benchmark.md", "docs/benchmark.zh-CN.md"), headings):
            with self.subTest(document=filename):
                text = (ROOT / filename).read_text()
                self.assertEqual(tuple(re.findall(r"(?m)^## (.+)$", text)), expected)
                for phrase in common:
                    self.assertIn(phrase, text)
                for fixture in corpus:
                    for value in (fixture["id"], str(fixture["pages"]), str(fixture["bytes"]), fixture["document_type"], fixture["producer_family"], fixture["source_url"], fixture["license_url"]):
                        self.assertIn(value, text)
                    for language in fixture["languages"]:
                        self.assertIn(language, text)
                self.assertNotRegex(text.lower(), r"always faster|universally faster|guaranteed lower memory|始终更快|普遍更快|保证更低内存")
                self.assertNotRegex(text.lower(), r"2026-09-27|superseded|first current-protocol|historical evidence|task identifier is historical|旧协议|旧计时|历史证据|已被替代|首份|历史名称")

    def test_dated_benchmark_results_are_complete_qualified_and_bilingual(self) -> None:
        common = (
            "2026-09-29", "77", "770", "73", "89", "890", "81", "0.01",
            "20260929T030834Z-2ec420a0", "20260929T034536Z-586dc351",
            "fbe3e2a25c667ec6f88a01f0d7c6f1b7505298e13863513ea4fc76d1235139e2",
            "59756d147ced354d2e9aa6adf852e7260533c347a03faa9aa79ef7db091a397a",
            "openintro-statistics", "riscv-unprivileged", "text-glyph-jsonl", "image-digests",
            "workers 1/2/4/8", "3.36", "5.56", "34", "43",
        )
        boundaries = (
            ("docs/benchmark-results-2026-09-29.md", "interactive desktop", "blocked", "No cross-library memory ratio"),
            ("docs/benchmark-results-2026-09-29.zh-CN.md", "交互式桌面", "阻断", "禁止跨库内存比值"),
        )
        fixture_ids = [fixture["id"] for fixture in json.loads((ROOT / "bench/corpus.json").read_text())["fixtures"]]
        for filename, load_note, blocked, memory in boundaries:
            with self.subTest(document=filename):
                text = (ROOT / filename).read_text()
                for phrase in (*common, load_note, blocked, memory, *fixture_ids):
                    self.assertIn(phrase, text)
                self.assertGreaterEqual(text.count("| `"), 50)
                self.assertNotRegex(text.lower(), r"2026-09-27|superseded|first dated|current .*protocol.*首份|历史数据|历史证据|已被替代|首份带日期")

    def test_notice_attributes_every_benchmark_only_document(self) -> None:
        notice = (ROOT / "NOTICE").read_text()
        self.assertIn("Benchmark-only public PDFs", notice)
        section = notice.split("Benchmark-only public PDFs", 1)[-1]
        self.assertIn("not distributed with the Go module", section)
        self.assertIn("bench/corpus.json", section)
        for fixture in json.loads((ROOT / "bench/corpus.json").read_text())["fixtures"]:
            with self.subTest(fixture=fixture["id"]):
                for value in (fixture["id"], fixture["source_url"], fixture["license"], fixture["license_url"], fixture["attribution"]):
                    self.assertIn(value, section)

    def test_benchmark_rss_claims_exclude_worker_callback_lower_bounds(self) -> None:
        for filename, end_heading, required in (
            ("docs/benchmark.md", "Correctness gates", ("single-process-lifetime-peak", "parent-lifetime-and-worker-callback-lower-bound", "callback_worker_peak_rss_sum_bytes", "parent_peak_rss_bytes", "peak_rss_bytes=0", "IPC", "not a complete lifetime peak", "No cross-library memory ratio")),
            ("docs/benchmark.zh-CN.md", "正确性门禁", ("single-process-lifetime-peak", "parent-lifetime-and-worker-callback-lower-bound", "callback_worker_peak_rss_sum_bytes", "parent_peak_rss_bytes", "peak_rss_bytes=0", "IPC", "不是完整生命周期峰值", "禁止跨库内存比值")),
        ):
            with self.subTest(document=filename):
                text = (ROOT / filename).read_text().split("## " + end_heading, 1)[0]
                for phrase in required:
                    self.assertIn(phrase, text)
                self.assertNotIn("Playa multi-process RSS sums observed per-process\nlifetime peaks", text)
                self.assertNotIn("Playa 多进程 RSS 求和各已观测进程的生命周期峰值", text)

    def test_ci_is_one_basic_job_and_full_verification_stays_local(self) -> None:
        makefile = (ROOT / "Makefile").read_text()
        verify = re.search(r"(?m)^verify:\s*([^\n]*)$", makefile)
        self.assertIsNotNone(verify)
        self.assertIn("test-race", verify.group(1).split())
        self.assertIn("public-corpus-test", verify.group(1).split())
        self.assertIn("bench-contract-test", verify.group(1).split())
        self.assertRegex(makefile, r"(?m)^engineering-check:\s*\n\t\$\(GO\) run ./cmd/playa-engineering-check(?:\s|$)")
        basic = re.search(r"(?m)^ci-basic:\s*([^\n]*)$", makefile)
        self.assertIsNotNone(basic)
        self.assertEqual(
            basic.group(1).split(),
            ["release-metadata-test", "check-fmt", "tidy-check", "engineering-check",
             "vet", "lint", "test", "api-audit"],
        )
        workflow = (ROOT / ".github/workflows/ci.yml").read_text()
        self.assertEqual(
            re.findall(r"(?m)^  ([A-Za-z0-9_-]+):\s*$", workflow.split("jobs:\n", 1)[1]),
            ["basic"],
        )
        self.assertRegex(workflow, r"(?m)^\s*- run: make ci-basic\s*$")
        self.assertRegex(
            workflow,
            r"(?m)^\s*- uses: golangci/golangci-lint-action@0a35821d5c230e903fcfe077583637dea1b27b47\s+# v9\.0\.0\s*$",
        )
        self.assertRegex(workflow, r"(?m)^\s+install-only:\s+true\s*$")
        forbidden = (
            "setup-python", "setup-uv", "actions/cache", "make verify",
            "public-fixtures", "pdfa-fixtures", "corpus-test", "compat-",
            "resources-check", "bench-contract", "test-race", "vuln-",
        )
        for value in forbidden:
            with self.subTest(forbidden=value):
                self.assertNotIn(value, workflow)
        documentation = {
            "README.md": "GitHub Actions runs `make ci-basic`",
            "README.zh-CN.md": "GitHub Actions 运行 `make ci-basic`",
            ".github/CONTRIBUTING.md": "GitHub Actions runs only `make ci-basic`",
            "docs/engineering.md": "Online CI runs only `make ci-basic`",
            "docs/engineering.zh-CN.md": "在线 CI 只运行 `make ci-basic`",
            "docs/compatibility.md": "Compatibility comparisons are local gates",
            "docs/compatibility.zh-CN.md": "兼容性比较仅作为本地门禁",
            "docs/benchmark.md": "Benchmark execution and its contract suite remain local gates",
            "docs/benchmark.zh-CN.md": "benchmark 执行及其合同测试只作为本地门禁",
        }
        for filename, required in documentation.items():
            with self.subTest(document=filename):
                self.assertIn(required, (ROOT / filename).read_text())

    def test_pre_commit_uses_one_non_mutating_verify_gate(self) -> None:
        config = (ROOT / ".pre-commit-config.yaml").read_text()
        entries = re.findall(r"(?m)^\s*entry:\s*(.*?)\s*$", config)
        self.assertEqual(entries, ["make verify"])
        self.assertRegex(config, r"(?m)^\s*pass_filenames: false\s*$")
        self.assertRegex(config, r"(?m)^\s*always_run: true\s*$")
        makefile = (ROOT / "Makefile").read_text()
        self.assertRegex(makefile, r"(?m)^tidy-check:\s*\n\t\$\(GO\) mod tidy -diff\s*$")

    def run_archive_probe(self, failure: str) -> subprocess.CompletedProcess[str]:
        with tempfile.TemporaryDirectory(prefix="go-playa-archive-probe-") as directory:
            root = Path(directory)
            source = root / "source"
            source.mkdir()
            (source / "Makefile").write_bytes((ROOT / "Makefile").read_bytes())
            (source / "go.mod").write_text("module example.test/archive\n\ngo 1.25.0\n")
            if failure == "nested-module":
                nested = source / "testdata/files"
                nested.mkdir(parents=True)
                (nested / "go.mod").write_text("module example.test/nested\n")
            tools = root / "tools"
            tools.mkdir()
            env = {**os.environ, "PATH": str(tools) + os.pathsep + os.environ["PATH"]}
            if failure == "lfs":
                (source / "a.pdf").write_text("%PDF-1.7\n")
                (source / "z.pdf").write_text("%PDF-1.7\n")
                (source / ".gitattributes").write_text("a.pdf filter=lfs\n")
                env.update({
                    "GIT_CONFIG_COUNT": "2",
                    "GIT_CONFIG_KEY_0": "filter.lfs.clean", "GIT_CONFIG_VALUE_0": "cat",
                    "GIT_CONFIG_KEY_1": "filter.lfs.required", "GIT_CONFIG_VALUE_1": "false",
                })
            if failure in {"copy", "git-preparation", "attributes", "compressed-size"}:
                tool = {"copy": "rsync", "git-preparation": "git", "attributes": "git", "compressed-size": "wc"}[failure]
                real = shutil.which(tool)
                self.assertIsNotNone(real)
                body = f"#!{sys.executable}\nimport os, sys\n"
                if failure == "copy":
                    body += "sys.stderr.write('injected copy failure\\n')\nsys.exit(23)\n"
                elif failure == "git-preparation":
                    body += "if 'init' in sys.argv: sys.exit(23)\n"
                elif failure == "attributes":
                    body += "if 'check-attr' in sys.argv: sys.exit(23)\n"
                else:
                    # Model a >500 MiB compressed archive without allocating one.
                    body += "data = sys.stdin.buffer.read()\nprint(524288001 if data.startswith(b'PK') else len(data))\nsys.exit(0)\n"
                body += f"os.execv({real!r}, [{real!r}, *sys.argv[1:]])\n"
                wrapper = tools / tool
                wrapper.write_text(body)
                wrapper.chmod(0o755)
            return subprocess.run(
                ["make", "module-archive-check"], cwd=source, env=env,
                capture_output=True, text=True,
            )

    def test_archive_gate_accepts_a_valid_source_candidate(self) -> None:
        result = self.run_archive_probe("none")
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)

    def test_archive_gate_propagates_validation_and_preparation_failures(self) -> None:
        for failure in ("nested-module", "compressed-size", "copy", "git-preparation", "attributes", "lfs"):
            with self.subTest(failure=failure):
                result = self.run_archive_probe(failure)
                self.assertNotEqual(result.returncode, 0, f"{failure} was silently accepted:\n{result.stdout}{result.stderr}")

    def test_project_license_is_standard_mit(self) -> None:
        self.assertEqual(
            (ROOT / "LICENSE").read_text(),
            COMBINED_LICENSE,
            "LICENSE must retain the project, Playa, and inherited pdfminer.six MIT notices",
        )

    def test_required_public_project_files_exist(self) -> None:
        required = (
            "NOTICE",
            "THIRD_PARTY_LICENSES",
            ".github/SECURITY.md",
            ".github/CONTRIBUTING.md",
            ".github/dependabot.yml",
        )
        missing = [path for path in required if not (ROOT / path).is_file()]
        self.assertEqual(missing, [], f"missing public project files: {missing}")

    def test_duplicate_internal_core_is_absent(self) -> None:
        stale: list[str] = []
        old_package = "github.com/lin-string/go-playa/" + "internal/core"
        for path in public_text_files():
            if old_package in path.read_text():
                stale.append(str(path.relative_to(ROOT)))
        self.assertEqual(
            ((ROOT / "internal/core").exists(), stale),
            (False, []),
            f"internal/core must be absent; stale package references: {stale}",
        )

    def test_legacy_page_concurrency_api_is_absent(self) -> None:
        common_declarations = (
            r"(?m)^type PageConcurrencyOptions\b",
            r"(?m)^func DefaultPageConcurrencyOptions\(",
        )
        packages = (
            ("github.com/lin-string/go-playa", common_declarations),
            ("github.com/lin-string/go-playa/document", (
                r"(?m)^func \([^)]*\) ForEachPageConcurrentWithOptions\(",
                *common_declarations,
            )),
        )
        for package, declarations in packages:
            result = subprocess.run(
                ["go", "doc", "-all", package],
                cwd=ROOT, capture_output=True, text=True,
            )
            self.assertEqual(result.returncode, 0,
                             f"{package} API inspection must succeed:\n"
                             + result.stdout + result.stderr)
            for declaration in declarations:
                with self.subTest(package=package, declaration=declaration):
                    match = re.search(declaration, result.stdout)
                    self.assertIsNone(
                        match,
                        "legacy public page concurrency declaration must be absent",
                    )
        with self.subTest(package="github.com/lin-string/go-playa/concurrency"):
            self.assertFalse((ROOT / "concurrency").exists(),
                             "public concurrency package must be absent")

    def test_github_actions_use_immutable_revisions(self) -> None:
        failures: list[str] = []
        pattern = re.compile(
            r"^\s*(?:-\s+)?uses:\s+['\"]?([^\s@'\"]+)@([0-9a-fA-F]{40})"
            r"['\"]?\s+#\s+v\d+(?:\.\d+){0,2}"
            r"(?:[-+][\w.-]+)?(?:\s+.*)?$"
        )
        for workflow in sorted((ROOT / ".github/workflows").glob("*.y*ml")):
            for line_number, line in enumerate(workflow.read_text().splitlines(), 1):
                active = line.split("#", 1)[0]
                # Reject flow-style action mappings explicitly rather than
                # silently skipping them in this line-oriented policy check.
                flow_uses = re.search(r"[{,]\s*['\"]?uses['\"]?\s*:", active)
                block_uses = re.match(r"^\s*(?:-\s+)?uses:", line)
                if flow_uses or (block_uses and not pattern.match(line)):
                    failures.append(f"{workflow.relative_to(ROOT)}:{line_number}: {line.strip()}")
        self.assertEqual(
            failures,
            [],
            "GitHub Actions must use full commit SHAs with tag comments:\n"
            + "\n".join(failures),
        )

    def test_ci_uses_the_patched_go_toolchain(self) -> None:
        workflow = (ROOT / ".github/workflows/ci.yml").read_text()
        steps = re.split(r"(?m)^\s*-\s+", workflow)
        go_steps = [step for step in steps if "actions/setup-go@" in step]
        self.assertTrue(go_steps, "CI must configure a Go toolchain")
        for step in go_steps:
            active = "\n".join(line.split("#", 1)[0] for line in step.splitlines())
            self.assertRegex(
                active,
                r"(?m)^\s*go-version-file:\s*['\"]?\.go-version['\"]?\s*$",
                "Every CI Go job must select the patched toolchain from .go-version",
            )
            self.assertNotRegex(
                active,
                r"(?m)^\s*go-version:",
                "An explicit go-version overrides the shared .go-version file",
            )
        self.assertRegex(
            (ROOT / ".go-version").read_text().strip(),
            r"^\d+\.\d+\.\d+$",
            ".go-version must pin an exact patch version",
        )

    def test_release_artifacts_are_ignored(self) -> None:
        lines = {
            line.strip()
            for line in (ROOT / ".gitignore").read_text().splitlines()
            if line.strip() and not line.lstrip().startswith("#")
        }
        required = {
            ".compat-cache/",
            ".venv/",
            "__pycache__/",
            "*.pyc",
            ".DS_Store",
            ".idea/",
            ".vscode/",
            "*.swp",
            "*.tmp",
            "*.bak",
            "*~",
            "*.pprof",
            "*.mprof",
        }
        self.assertEqual(sorted(required - lines), [], "missing release ignore patterns")

    def test_public_markdown_relative_links_resolve(self) -> None:
        missing: list[str] = []
        link_pattern = re.compile(
            r"!?\[[^\]]*\]\(\s*(<[^>]+>|[^\s)]+)(?:\s+['\"].*?['\"])?\s*\)"
            r"|^\s{0,3}\[[^\]]+\]:\s*(<[^>]+>|\S+)"
        )
        for document in public_markdown_files():
            for line_number, line in enumerate(document.read_text(errors="replace").splitlines(), 1):
                for match in link_pattern.finditer(line):
                    raw_target = (match.group(1) or match.group(2)).strip()
                    if raw_target.startswith("<") and raw_target.endswith(">"):
                        raw_target = raw_target[1:-1]
                    target = unquote(raw_target.split("#", 1)[0])
                    if not target or re.match(r"^[a-z][a-z0-9+.-]*:", target, re.I):
                        continue
                    if target.startswith("/"):
                        continue
                    resolved = (document.parent / target).resolve()
                    if not resolved.exists():
                        missing.append(
                            f"{document.relative_to(ROOT)}:{line_number}: {raw_target}"
                        )
        self.assertEqual(missing, [], "broken relative Markdown links:\n" + "\n".join(missing))

    def test_readme_states_license_and_independence_boundaries(self) -> None:
        readme = (ROOT / "README.md").read_text().lower()
        section = re.search(r"^## license(?: and [^\n]+)?\s*$\n(.*?)(?=^## |\Z)",
                            readme, re.M | re.S)
        self.assertIsNotNone(section, "README must have a License section")
        license_section = section.group(1)
        required = (
            "independent go implementation",
            "not an official playa project",
            "playa and pdfminer.six",
            "third_party_licenses",
            "notice",
            "compat/upstream.toml",
        )
        missing = [phrase for phrase in required if phrase not in license_section]
        self.assertEqual(missing, [], f"README is missing release boundary statements: {missing}")
        self.assertRegex(license_section, r"go-playa is mit-licensed",
                         "README must identify the project's MIT license")
        self.assertRegex(license_section, r"other bundled third-party[^.]*terms",
                         "README must distinguish bundled third-party terms")

    def test_third_party_components_are_attributed(self) -> None:
        notice_path = ROOT / "NOTICE"
        licenses_path = ROOT / "THIRD_PARTY_LICENSES"
        notice = notice_path.read_text().lower() if notice_path.is_file() else ""
        licenses = licenses_path.read_text().lower() if licenses_path.is_file() else ""
        in_notice = (
            "playa",
            "pdfminer.six",
            "pdfbox",
            "adobe glyph list",
            "zapf dingbats",
            "adobe cmap",
            "fonttools",
            "core 14",
            "golang.org/x/image",
            "golang.org/x/text",
            "golang.org/x/sys",
            "public pdf corpora",
        )
        in_licenses = (
            "david huggins-daines",
            "yusuke shinyama",
            "apache license",
            "www.pdfbox.org",
            "adobe",
            "just van rossum",
            "core 14 afm",
            "golang.org/x/image",
            "golang.org/x/text",
            "golang.org/x/sys",
        )
        missing_notice = [name for name in in_notice if name not in notice]
        missing_licenses = [name for name in in_licenses if name not in licenses]
        self.assertEqual(missing_notice, [], f"NOTICE is missing components: {missing_notice}")
        self.assertEqual(
            missing_licenses,
            [],
            f"THIRD_PARTY_LICENSES is missing terms: {missing_licenses}",
        )

    def test_dependabot_configuration_is_exact(self) -> None:
        path = ROOT / ".github/dependabot.yml"
        actual = path.read_text() if path.is_file() else ""
        self.assertEqual(actual, DEPENDABOT_CONFIG,
                         "Dependabot must match the approved three-ecosystem weekly policy")


if __name__ == "__main__":
    unittest.main()
