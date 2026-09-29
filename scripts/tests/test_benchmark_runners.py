"""Offline sample-protocol tests; the oracle dependency is isolated at open()."""
from __future__ import annotations

import contextlib
import importlib.util
import io
import hashlib
import json
import os
import runpy
from pathlib import Path
import subprocess
import shlex
import struct
import sys
import tempfile
import types
import unittest
from unittest.mock import patch

ROOT = Path(__file__).resolve().parents[2]
spec = importlib.util.spec_from_file_location("benchmark_playa", ROOT / "bench/run_playa.py")
runner = importlib.util.module_from_spec(spec)
with patch.dict(sys.modules, {"playa": types.ModuleType("playa")}):
    spec.loader.exec_module(runner)


class Pages:
    def __len__(self):
        return 1

    def map(self, callback):
        yield callback(types.SimpleNamespace(page_idx=0, texts=[], images=[]))

    def __iter__(self):
        yield types.SimpleNamespace(page_idx=0, texts=[], images=[])


class Document:
    def __init__(self, clock):
        self.pages = Pages()
        self.objects = iter([object(), object()])
        self.clock = clock

    def __enter__(self):
        return self

    def __exit__(self, *_):
        self.clock[0] += 7


class SampleProtocolTests(unittest.TestCase):

    def test_canonical_float_precision_absorbs_cross_runtime_noise(self):
        self.assertEqual(runner.canonical_json(450.545474), "450.55")
        self.assertEqual(runner.canonical_json(450.545476), "450.55")

    def test_canonical_float_precision_absorbs_ocr_boundary_noise(self):
        self.assertEqual(runner.canonical_json(181.20049), "181.20")
        self.assertEqual(runner.canonical_json(181.20051), "181.20")

    def test_canonical_float_precision_stabilizes_midpoint_noise(self):
        self.assertEqual(runner.canonical_json(71.52499999999999), "71.53")
        self.assertEqual(runner.canonical_json(71.52500000000001), "71.53")
        self.assertEqual(runner.canonical_json(-71.52499999999999), "-71.53")
        self.assertEqual(runner.canonical_json(-71.52500000000001), "-71.53")

    def test_page_projection_digest_uses_delimiter_free_canonical_objects(self):
        digest = hashlib.sha256()
        runner.update_page_projection_digest(digest, b'{"page":0}')
        runner.update_page_projection_digest(digest, b'{"page":1}')
        self.assertEqual(digest.digest(), hashlib.sha256(b'{"page":0}{"page":1}').digest())
        self.assertNotEqual(digest.digest(), hashlib.sha256(b'{"page":0}\n{"page":1}\n').digest())

    def test_callback_rss_follows_projection_serialization(self):
        events = []
        page = types.SimpleNamespace(page_idx=0, texts=[], images=[])
        def encode(value):
            events.append('serialize')
            return '{}'
        def sample():
            events.append('sample')
            return 11, 100
        with patch.object(runner, 'canonical_json', encode), patch.object(runner, 'process_memory_sample', sample):
            runner.project_page(page)
        self.assertEqual(events, ['serialize', 'sample'], 'callback lower bound must include projection encoding, while still excluding later IPC')

    def test_multiprocess_rss_is_lower_bound_not_publishable_peak(self):
        for task in ('open-pages', 'text-glyph-jsonl'):
            with self.subTest(task=task):
                doc = Document([0])
                def mapped(callback):
                    with patch.object(runner, 'process_memory_sample', return_value=(11, 100)):
                        yield callback(types.SimpleNamespace(page_idx=0, texts=[], images=[]))
                    with patch.object(runner, 'process_memory_sample', return_value=(12, 200)):
                        yield callback(types.SimpleNamespace(page_idx=1, texts=[], images=[]))
                    with patch.object(runner, 'process_memory_sample', return_value=(11, 150)):
                        yield callback(types.SimpleNamespace(page_idx=2, texts=[], images=[]))
                doc.pages = types.SimpleNamespace(map=mapped)
                output = io.StringIO()
                with patch.object(runner.playa, 'open', return_value=doc, create=True), patch.object(runner, 'process_memory_sample', return_value=(99, 400)), contextlib.redirect_stdout(output):
                    if task == 'text-glyph-jsonl':
                        runner.run_text_glyph_jsonl(['fixture.pdf'], 2)
                    else:
                        runner.run_count_task(['fixture.pdf'], 2, task)
                record = json.loads(output.getvalue())
                self.assertEqual(record['peak_rss_bytes'], 0, 'worker IPC/teardown peaks are unmeasured, so complete RSS must be unavailable')
                self.assertEqual(record.get('rss_scope'), 'parent-lifetime-and-worker-callback-lower-bound')
                self.assertEqual(record.get('parent_peak_rss_bytes'), 400)
                self.assertEqual(record.get('callback_worker_peak_rss_sum_bytes'), 350)
                self.assertEqual(record['effective_workers'], 2, 'observed child callbacks prove configured pool dispatch regardless of page-count inference')
                self.assertEqual(record['observed_workers'], 2)

    def test_effective_worker_topology_uses_dispatch_evidence_not_page_threshold(self):
        doc = Document([0])
        doc.pages = types.SimpleNamespace(map=lambda callback: (
            callback(types.SimpleNamespace(page_idx=index, texts=[], images=[])) for index in range(21)))
        output = io.StringIO()
        with patch.object(runner.playa, 'open', return_value=doc, create=True), patch.object(runner.os, 'cpu_count', return_value=8), contextlib.redirect_stdout(output):
            runner.run_count_task(['fixture.pdf'], 2, 'open-pages')
        record = json.loads(output.getvalue())
        self.assertEqual(record['effective_workers'], 1, 'parent-only callback participation must not be described as a native process pool')
        self.assertEqual(record['observed_workers'], 1)

    def test_sequential_rss_is_post_close_process_peak(self):
        for task in ('open-pages', 'objects', 'text-glyph-jsonl'):
            with self.subTest(task=task):
                clock = [0]
                doc = Document(clock)
                output = io.StringIO()
                def sample():
                    return 99, 100 + clock[0]
                with patch.object(runner.playa, 'open', return_value=doc, create=True), patch.object(runner, 'process_memory_sample', sample), contextlib.redirect_stdout(output):
                    if task == 'text-glyph-jsonl':
                        runner.run_text_glyph_jsonl(['fixture.pdf'], 1)
                    else:
                        runner.run_count_task(['fixture.pdf'], 1, task)
                record = json.loads(output.getvalue())
                self.assertEqual(record['peak_rss_bytes'], 107, 'final process RSS sampling must follow document close')
                self.assertEqual(record.get('rss_scope'), 'single-process-lifetime-peak')
                self.assertNotIn('callback_worker_peak_rss_sum_bytes', record)

    def test_image_digest_frames_content_and_positions_in_page_order(self):
        pages = [types.SimpleNamespace(page_idx=index, texts=[], images=[types.SimpleNamespace(buffer=data)])
                 for index, data in ((0, b'abc'), (1, b'xyz'))]
        expected = hashlib.sha256(b'playa-image-digests-v1\0')
        for page in pages:
            data = page.images[0].buffer
            expected.update(struct.pack('>QQQ', page.page_idx, 0, len(data)) + hashlib.sha256(data).digest())
        outputs = []
        for order in (pages, list(reversed(pages))):
            doc = Document([0])
            doc.pages = types.SimpleNamespace(map=lambda callback: map(callback, order))
            output = io.StringIO()
            with patch.object(runner.playa, 'open', return_value=doc, create=True), contextlib.redirect_stdout(output):
                runner.run_count_task(['fixture.pdf'], 2, 'image-digests')
            record = json.loads(output.getvalue())
            self.assertEqual(record.get('sha256'), expected.hexdigest())
            self.assertEqual(record['images'], 2)
            outputs.append(record['sha256'])
        self.assertEqual(outputs[0], outputs[1])

    def test_make_sequential_forces_one_worker(self):
        completed = subprocess.run(["make", "--no-print-directory", "-n", "bench-sequential", "BENCH_WORKERS=8", "COMPAT_PDFS=fixture.pdf"], cwd=ROOT, capture_output=True, text=True, check=True)
        command = next(line for line in completed.stdout.splitlines() if "./bench/run.sh" in line)
        assignments = dict(token.split("=", 1) for token in shlex.split(command) if token.startswith(("BENCH_", "GOMAXPROCS=")))
        self.assertEqual(assignments.get("BENCH_WORKERS"), "1", "Make recipe must override ambient or requested benchmark workers")

    def test_make_compare_runs_sequential_and_concurrent_once(self):
        completed = subprocess.run(["make", "--no-print-directory", "-n", "bench-compare", "COMPAT_PDFS=fixture.pdf"], cwd=ROOT, capture_output=True, text=True, check=True)
        commands = [line for line in completed.stdout.splitlines() if "./bench/run.sh" in line]
        self.assertEqual(len(commands), 2, "bench-compare must not add a duplicate concurrent comparison")

    def test_default_python_cli_emits_single_worker_jsonl(self):
        playa = types.ModuleType("playa")
        playa.open = lambda *args, **kwargs: Document([0])
        output = io.StringIO()
        with patch.dict(sys.modules, {"playa": playa}), patch.object(sys, "argv", ["run_playa.py", "fixture.pdf"]), contextlib.redirect_stdout(output):
            runpy.run_path(str(ROOT / "bench/run_playa.py"), run_name="__main__")
        self.assertTrue(output.getvalue().startswith("{"), "default CLI stdout must be JSON")
        record = json.loads(output.getvalue())
        self.assertEqual(record["task"], "text-glyph-jsonl")
        self.assertEqual(record["requested_workers"], 1)

    def test_one_open_and_close_in_timing_with_distinct_worker_fields(self):
        for task in ("open-pages", "objects", "text-glyph-jsonl"):
            with self.subTest(task=task):
                clock = [0]
                calls = []

                def open_document(path, **kwargs):
                    calls.append(kwargs)
                    clock[0] += 5
                    return Document(clock)

                output = io.StringIO()
                with patch.object(runner.playa, "open", open_document, create=True), patch.object(runner.time, "perf_counter_ns", lambda: clock[0]), contextlib.redirect_stdout(output):
                    if task == "text-glyph-jsonl":
                        runner.run_text_glyph_jsonl(["fixture.pdf"], 4)
                    else:
                        runner.run_count_task(["fixture.pdf"], 4, task)
                record = json.loads(output.getvalue())
                self.assertEqual(len(calls), 1, "sample must not reopen for a page-count prepass")
                self.assertEqual(record["elapsed_ns"], 12)
                self.assertEqual(record.get("requested_workers"), 4)
                self.assertEqual(record.get("effective_workers"), 1)
                self.assertEqual(record.get("observed_workers"), 1)
                self.assertEqual(record["pages"], 1)

    def test_shell_stdout_is_jsonl_and_default_is_single_worker(self):
        with tempfile.TemporaryDirectory() as directory:
            directory = Path(directory)
            go = directory / "go"
            uv = directory / "uv"
            # Substitute external programs only: exercise the real shell orchestration.
            go.write_text("#!/bin/sh\nif [ \"$1\" = build ]; then\ncp \"$FAKE_SAMPLE\" \"$3\"\nfi\n")
            uv.write_text("#!/bin/sh\nexec \"$FAKE_SAMPLE\" \"$@\"\n")
            sample = directory / "sample"
            sample.write_text(f"#!{sys.executable}\nimport json, sys\nprint(json.dumps({{'argv': sys.argv[1:]}}))\n")
            for path in (go, uv, sample):
                path.chmod(0o755)
            environment = {**os.environ, "PATH": str(directory) + ":" + os.environ["PATH"], "FAKE_SAMPLE": str(sample), "TMPDIR": str(directory), "BENCH_REPEATS": "1", "GOMAXPROCS": "8"}
            environment.pop("BENCH_TASK", None)
            environment.pop("BENCH_WORKERS", None)
            completed = subprocess.run(["sh", str(ROOT / "bench/run.sh"), "fixture.pdf"], cwd=ROOT, env=environment, capture_output=True, text=True, check=True)
            lines = completed.stdout.splitlines()
            self.assertEqual(len(lines), 2, "human progress must go to stderr")
            arguments = [json.loads(line)["argv"] for line in lines]
            self.assertEqual(arguments[0][arguments[0].index("-workers") + 1], "1")
            self.assertEqual(arguments[1][arguments[1].index("--workers") + 1], "1")
            self.assertIn("benchmark", completed.stderr)
            environment["BENCH_REPEATS"] = "2"
            completed = subprocess.run(["sh", str(ROOT / "bench/run.sh"), "fixture.pdf"], cwd=ROOT, env=environment, capture_output=True, text=True, check=True)
            labels = [line.split(" task=")[0] for line in completed.stderr.splitlines()[1:]]
            self.assertEqual(labels, ["go[1]", "playa[1]", "playa[2]", "go[2]"], "repeats alternate implementation order")


if __name__ == "__main__":
    unittest.main()
