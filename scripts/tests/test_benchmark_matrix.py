"""Offline matrix orchestration contracts; no public benchmark is launched."""
import importlib.util
import hashlib
import json
from pathlib import Path
import subprocess
import tempfile
import unittest
import sys
from unittest.mock import patch

ROOT = Path(__file__).resolve().parents[2]


def module(name):
    path = ROOT / 'bench' / (name + '.py')
    if not path.is_file():
        raise AssertionError(f'{name} benchmark support is missing')
    spec = importlib.util.spec_from_file_location(name, path)
    value = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(value)
    return value


class MatrixTests(unittest.TestCase):
    def test_fresh_commands_and_alternating_repeat_order(self):
        runner = module('run_matrix')
        cell = dict(fixture='smoke-text', source='local', task='text-glyphs', workers=1, repeats=2)
        fixture = dict(id='smoke-text', path='testdata/files/acceptance_adjacent_glyphs.pdf')
        samples = runner.plan_samples([cell], {'smoke-text': fixture}, ROOT,
                                     ROOT / '.compat-cache/benchmark-corpus', Path('/tmp/bench'))
        self.assertEqual([s['library'] for s in samples], ['go-playa', 'playa', 'playa', 'go-playa'])
        self.assertEqual([s['repeat'] for s in samples], [1, 1, 2, 2])
        for sample in samples:
            command = sample['command']
            self.assertEqual(command[-1], str(ROOT / fixture['path']))
            flag = '-workers' if sample['library'] == 'go-playa' else '--workers'
            self.assertEqual(command[command.index(flag) + 1], '1')
            self.assertEqual(sum(argument.endswith('.pdf') for argument in command), 1)
        self.assertNotIn('go run', ' '.join(samples[0]['command']))

    def test_filter_and_applicability(self):
        runner = module('run_matrix')
        corpus = runner.manifest.load_corpus(ROOT / 'bench/corpus.json')
        matrix = runner.manifest.load_matrix(ROOT / 'bench/matrix.json')
        cells = runner.select_cells(corpus, matrix, 'smoke', ['smoke-image'], ['image-digests'], [2])
        self.assertEqual(len(cells), 1)
        self.assertEqual(cells[0]['task'], 'image-digests')
        with self.assertRaises(ValueError):
            runner.select_cells(corpus, matrix, 'smoke', ['smoke-image'], ['text-glyphs'], [])
        with self.assertRaises(ValueError):
            runner.select_cells(corpus, matrix, 'smoke', ['typo'], [], [])

    def test_stdout_is_exactly_one_json_object(self):
        runner = module('run_matrix')
        self.assertEqual(runner.parse_sample_stdout('{"library":"go"}\n'), {'library': 'go'})
        for output in ('label\n{}\n', '{}\n{}\n', '[]\n', '', '{"a":1,"a":2}\n', '{"x":NaN}'):
            with self.subTest(output=output), self.assertRaises(ValueError):
                runner.parse_sample_stdout(output)

    def test_preflight_parses_selected_local_inputs_before_samples(self):
        runner = module('run_matrix')
        matrix = runner.manifest.load_matrix(ROOT / 'bench/matrix.json')
        fixtures = {f['id']: f for f in matrix['local_fixtures']}
        selected = [dict(fixture='smoke-text', source='local')]
        calls = []
        def parse(path):
            calls.append(path)
            return 1
        runner.preflight(selected, fixtures, ROOT, Path('/unused'), page_parser=parse)
        self.assertEqual(calls, [ROOT / fixtures['smoke-text']['path']])
        with self.assertRaisesRegex(ValueError, 'parsed page count'):
            runner.preflight(selected, fixtures, ROOT, Path('/unused'), page_parser=lambda path: 2)
        with self.assertRaises(ValueError):
            runner.preflight(selected, fixtures, ROOT, Path('/unused'), page_parser=lambda path: True)

    def test_dry_run_never_builds_or_runs_inputs_or_creates_output(self):
        runner = module('run_matrix')
        with tempfile.TemporaryDirectory() as temp:
            output = Path(temp) / 'must-not-exist'
            with patch.object(runner.subprocess, 'run', side_effect=AssertionError('process launched')):
                with patch('sys.stdout'):
                    self.assertEqual(runner.main(['--preset', 'reference', '--dry-run', '--output', str(output)]), 0)
            self.assertFalse(output.exists())

    def test_environment_records_machine_runtime_and_oracle_identity(self):
        runner = module('run_matrix')
        def command(argv, **kwargs):
            if argv[-1] == 'hw.memsize':
                return subprocess.CompletedProcess(argv, 0, '1000000\n', '')
            if argv[0] == 'go':
                return subprocess.CompletedProcess(argv, 0, 'go version go1.25.0 linux/amd64\n', '')
            if argv[0] == 'uv':
                return subprocess.CompletedProcess(argv, 0, json.dumps({'python': '3.12', 'playa': 'pinned'}), '')
            return subprocess.CompletedProcess(argv, 0, 'Example CPU\n', '')
        with patch.object(runner.platform, 'platform', return_value='test OS'), patch.object(runner.subprocess, 'run', side_effect=command):
            env = runner.capture_environment(ROOT)
        for key in ('os', 'architecture', 'cpu', 'ram_bytes', 'go', 'python', 'playa', 'oracle_sha256'):
            self.assertIn(key, env)
        self.assertEqual(env['playa'], 'pinned')

    def test_execution_launches_fresh_process_and_captures_only_stdout(self):
        runner = module('run_matrix')
        code = 'import os,json,sys; print("progress",file=sys.stderr); print(json.dumps({"pid":os.getpid()}))'
        with patch('sys.stderr'):
            first = runner.parse_sample_stdout(runner.execute([sys.executable, '-c', code], ROOT))
            second = runner.parse_sample_stdout(runner.execute([sys.executable, '-c', code], ROOT))
        self.assertNotEqual(first['pid'], second['pid'])

    def test_oracle_package_is_read_from_the_canonical_contract(self):
        runner = module('run_matrix')
        commands = []
        def execute(argv, root):
            commands.append(argv)
            if argv[0] == 'uv':
                return json.dumps({'python': '3.12', 'playa': 'pinned'})
            if argv[-1] == 'hw.memsize':
                return '1000000'
            return 'metadata'
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            (root / 'compat').mkdir()
            (root / 'compat/upstream.toml').write_text('[playa]\npackage = "alternate-oracle"\n')
            with patch.object(runner, 'execute', side_effect=execute):
                runner.capture_environment(root)
        self.assertIn('alternate-oracle', ' '.join(next(c for c in commands if c[0] == 'uv')))

    def test_make_matrix_targets_are_explicit_and_dry_run_safe(self):
        for target, preset in (('bench-smoke', 'smoke'), ('bench-reference', 'reference'), ('bench-scaling', 'scaling')):
            result = subprocess.run(['make', '-n', target], cwd=ROOT, capture_output=True, text=True)
            self.assertEqual(result.returncode, 0, result.stderr)
            self.assertIn(f'--preset {preset}', result.stdout)
        result = subprocess.run(['make', '-n', 'bench-dry-run', 'BENCH_PRESET=scaling'], cwd=ROOT, capture_output=True, text=True)
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertIn('--preset "scaling"', result.stdout)
        self.assertIn('--dry-run', result.stdout)

    def test_source_and_binary_identity_for_dirty_unborn_repository(self):
        runner = module('run_matrix')
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            subprocess.run(['git', 'init', '-q', str(root)], check=True, capture_output=True)
            (root / '.gitignore').write_text('.compat-cache/\n__pycache__/\n')
            (root / 'main.go').write_text('package main\n')
            binary = root / '.compat-cache/sample-binary'
            binary.parent.mkdir()
            binary.write_bytes(b'benchmark binary')
            first = runner.capture_implementation(root, binary, supplied=True)
            self.assertIsNone(first['git_commit'])
            self.assertTrue(first['git_dirty'])
            self.assertEqual(first['binary']['origin'], 'supplied-binary')
            self.assertEqual(first['binary']['path'], str(binary.resolve()))
            self.assertEqual(first['binary']['sha256'], hashlib.sha256(binary.read_bytes()).hexdigest())
            (root / '.compat-cache/raw.jsonl').write_text('generated results')
            self.assertEqual(first['source_sha256'], runner.capture_implementation(root, binary, supplied=True)['source_sha256'])
            (root / 'main.go').write_text('package changed\n')
            second = runner.capture_implementation(root, binary, supplied=True)
            self.assertNotEqual(first['source_sha256'], second['source_sha256'])
            binary.write_bytes(b'changed binary')
            third = runner.capture_implementation(root, binary, supplied=True)
            self.assertEqual(second['source_sha256'], third['source_sha256'])
            self.assertNotEqual(second['binary']['sha256'], third['binary']['sha256'])

    def test_offline_contract_gates_are_wired_to_local_verify_only(self):
        makefile = (ROOT / 'Makefile').read_text()
        verify = next(line for line in makefile.splitlines() if line.startswith('verify:'))
        self.assertIn('bench-contract-test', verify)
        self.assertIn('bench-contract-check', verify)
        for target, action in (('bench-corpus-pull', 'pull'), ('bench-corpus-check', 'check'), ('bench-matrix-check', 'matrix-check')):
            result = subprocess.run(['make', '-n', target], cwd=ROOT, capture_output=True, text=True)
            self.assertEqual(result.returncode, 0, result.stderr)
            self.assertIn('benchmark_manifest.py', result.stdout)
            self.assertIn(action, result.stdout)
        result = subprocess.run(['make', '-n', 'bench-contract-test', 'bench-contract-check'], cwd=ROOT, capture_output=True, text=True)
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertIn('test_benchmark_*.py', result.stdout)
        self.assertIn('corpus-check', result.stdout)
        self.assertIn('matrix-check', result.stdout)
        self.assertNotIn('run_matrix.py', result.stdout)
        self.assertNotIn(' pull', result.stdout)
        ci = (ROOT / '.github/workflows/ci.yml').read_text()
        self.assertNotIn('bench-contract-test', ci)
        self.assertNotIn('bench-contract-check', ci)


if __name__ == '__main__':
    unittest.main()
