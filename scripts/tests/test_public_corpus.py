"""Offline contract tests for the optional public PDF corpus."""
import hashlib
import importlib.util
import io
import json
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest
from unittest import mock

SCRIPT = Path(__file__).resolve().parents[1] / 'public_corpus.py'
ROOT = SCRIPT.parent.parent
PDF = b'%PDF-1.7\nsmall offline fixture\n%%EOF\n'


class PublicCorpusTest(unittest.TestCase):
    def test_repository_compatibility_fixtures_have_public_sources(self):
        manifest = json.loads((ROOT / 'compat' / 'public_corpus.json').read_text())
        public_names = {fixture['id'] + '.pdf' for fixture in manifest['fixtures']}
        differences = json.loads((ROOT / 'compat' / 'known_differences.json').read_text())
        for record in differences['records']:
            name = record['fixture']
            with self.subTest(fixture=name):
                if name in public_names:
                    fixture = next(item for item in manifest['fixtures'] if item['id'] + '.pdf' == name)
                    self.assertEqual(record['fixture_sha256'], fixture['sha256'])
                    continue
                self.assertTrue(name.startswith(('acceptance_', 'recovery_', 'security_')),
                                f'{name} is not a generated test fixture or public PDF')
                path = ROOT / 'testdata' / 'files' / name
                self.assertTrue(path.is_file(), f'{name} is missing from generated fixtures')
                self.assertEqual(record['fixture_sha256'], hashlib.sha256(path.read_bytes()).hexdigest())

    def test_makefile_defaults_use_manifest_driver_and_overrides_stay_local(self):
        manifest = self.module().load_manifest(ROOT / 'compat' / 'public_corpus.json')
        for target, mode in [('compat', 'compare'), ('bench', 'run')]:
            arguments = ['make', '--no-print-directory', '-n', target,
                         f'PUBLIC_CORPUS_MANIFEST={self.manifest}',
                         f'PUBLIC_FIXTURE_DIR={self.directory}']
            with self.subTest(target=target):
                result = subprocess.run(arguments, cwd=ROOT, text=True, capture_output=True)
                self.assertEqual(result.returncode, 0, result.stderr)
                driver = (f'python3 scripts/public_corpus.py --manifest "{self.manifest}" '
                          f'--directory "{self.directory}" {mode} --')
                self.assertIn(driver, result.stdout)
                # The manifest is the sole source of large PDF names; Make must
                # not split a cached path with spaces into its own word list.
                for fixture in manifest:
                    self.assertNotIn(fixture['id'] + '.pdf', result.stdout)
                result = subprocess.run(arguments + ['COMPAT_PDFS=testdata/files/form_simple.pdf'],
                                        cwd=ROOT, text=True, capture_output=True)
                self.assertEqual(result.returncode, 0, result.stderr)
                self.assertNotIn('scripts/public_corpus.py', result.stdout)
                self.assertIn('testdata/files/form_simple.pdf', result.stdout)

    def setUp(self):
        self.temp = tempfile.TemporaryDirectory(prefix='public corpus ')
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        self.directory = self.root / 'PDF files'
        self.manifest = self.root / 'manifest.json'
        self.fixture = dict(id='licensed-book', title='Licensed Book',
                            url='https://example.org/book.pdf',
                            sha256=hashlib.sha256(PDF).hexdigest(), bytes=len(PDF),
                            pages=1, license='CC-BY-4.0',
                            license_url='https://creativecommons.org/licenses/by/4.0/',
                            source_url='https://example.org/books', notes='Test fixture')
        self.write_manifest()

    def write_manifest(self, **changes):
        data = dict(schema_version=1, fixtures=[self.fixture])
        data.update(changes)
        self.manifest.write_text(json.dumps(data))

    def run_cli(self, action, *args):
        return subprocess.run([sys.executable, str(SCRIPT), '--manifest', str(self.manifest),
                               '--directory', str(self.directory), action, *args],
                              text=True, capture_output=True)

    def install(self, data=PDF):
        self.directory.mkdir(exist_ok=True)
        path = self.directory / 'licensed-book.pdf'
        path.write_bytes(data)
        return path

    def module(self):
        self.assertTrue(SCRIPT.is_file(), 'public corpus helper must exist')
        spec = importlib.util.spec_from_file_location('public_corpus', SCRIPT)
        module = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(module)
        return module

    def test_paths_verifies_and_preserves_spaces(self):
        path = self.install()
        result = self.run_cli('paths')
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(result.stdout, str(path.resolve()) + '\n')

    def test_offline_check_and_paths_reject_missing_or_corrupt_files(self):
        for action in ('check', 'paths'):
            result = self.run_cli(action)
            self.assertNotEqual(result.returncode, 0)
            self.assertIn('missing', result.stderr)
            for data in (b'not PDF', PDF + b'extra', PDF.replace(b'small', b'large')):
                self.install(data)
                result = self.run_cli(action)
                self.assertNotEqual(result.returncode, 0)
                self.assertEqual(result.stdout, '')
            (self.directory / 'licensed-book.pdf').unlink()

    def test_manifest_rejects_unsafe_or_incomplete_entries(self):
        self.install()
        for key, value in [('id', '../escape'), ('id', 'Book'), ('id', ''),
                           ('url', 'http://example.org/book.pdf'), ('url', 'https:///file'),
                           ('sha256', 'z' * 64), ('bytes', 0), ('bytes', True),
                           ('pages', -1), ('license', ''), ('title', ''),
                           ('source_url', ''), ('license_url', '')]:
            with self.subTest(key=key, value=value):
                fixture = dict(self.fixture, **{key: value})
                self.write_manifest(fixtures=[fixture])
                result = self.run_cli('check')
                self.assertNotEqual(result.returncode, 0)
                self.assertIn('manifest', result.stderr)
        for changes in (dict(schema_version=2), dict(fixtures=[]),
                        dict(fixtures=[self.fixture, self.fixture])):
            self.write_manifest(**changes)
            result = self.run_cli('check')
            self.assertNotEqual(result.returncode, 0)
            self.assertIn('manifest', result.stderr)

    def test_pull_skips_valid_and_refuses_mismatched_existing_file(self):
        module = self.module()
        path = self.install()
        with mock.patch.object(module, 'open_https', side_effect=AssertionError('no network')):
            module.pull(self.fixture, self.directory)
            path.write_bytes(b'local content')
            with self.assertRaisesRegex(ValueError, 'mismatch|signature'):
                module.pull(self.fixture, self.directory)
        self.assertEqual(path.read_bytes(), b'local content')

    def test_pull_installs_verified_download_and_cleans_failed_partials(self):
        module = self.module()
        for data in (PDF + b'oversized', PDF[:-1], PDF.replace(b'small', b'large'), b'X' * len(PDF)):
            with self.subTest(data=data), mock.patch.object(module, 'open_https', return_value=io.BytesIO(data)):
                with self.assertRaises(ValueError):
                    module.pull(self.fixture, self.directory)
                self.assertEqual(list(self.directory.iterdir()), [])
        with mock.patch.object(module, 'open_https', return_value=io.BytesIO(PDF)):
            module.pull(self.fixture, self.directory)
        self.assertEqual((self.directory / 'licensed-book.pdf').read_bytes(), PDF)
        self.assertEqual(len(list(self.directory.iterdir())), 1)

    def test_pull_cleans_interrupted_download(self):
        module = self.module()
        response = mock.MagicMock()
        response.__enter__.return_value = response
        response.read.side_effect = [PDF[:10], OSError('connection lost')]
        with mock.patch.object(module, 'open_https', return_value=response):
            with self.assertRaises(OSError):
                module.pull(self.fixture, self.directory)
        self.assertEqual(list(self.directory.iterdir()), [])

    def test_https_redirect_rejects_downgrade(self):
        module = self.module()
        with self.assertRaisesRegex(ValueError, 'HTTPS'):
            module.HTTPSRedirectHandler().redirect_request(None, None, 302, '', {},
                                                          'http://example.org/book.pdf')

    def test_compare_preserves_argument_boundaries_and_exit_status(self):
        path = self.install()
        capture = self.root / 'args.json'
        comparator = self.root / 'fake comparator.py'
        comparator.write_text('import json,sys\nfrom pathlib import Path\n'
                              'Path(sys.argv[1]).write_text(json.dumps(sys.argv[2:]))\n'
                              'sys.exit(7)\n')
        result = self.run_cli('compare', '--', sys.executable, str(comparator),
                              str(capture), '--password', 'password with spaces')
        self.assertEqual(result.returncode, 7, result.stderr)
        self.assertEqual(json.loads(capture.read_text()),
                         ['--password', 'password with spaces', '--pdf', str(path.resolve())])
        path.unlink()
        capture.unlink()
        result = self.run_cli('compare', '--', sys.executable, str(comparator), str(capture))
        self.assertNotEqual(result.returncode, 0)
        self.assertFalse(capture.exists())

    def test_run_preserves_pdf_paths_as_positional_arguments(self):
        path = self.install()
        capture = self.root / 'args.json'
        runner = self.root / 'fake runner.py'
        runner.write_text('import json,sys\nfrom pathlib import Path\n'
                          'Path(sys.argv[1]).write_text(json.dumps(sys.argv[2:]))\n'
                          'sys.exit(7)\n')
        result = self.run_cli('run', '--', sys.executable, str(runner),
                              str(capture), '--option', 'argument with spaces')
        self.assertEqual(result.returncode, 7, result.stderr)
        self.assertEqual(json.loads(capture.read_text()),
                         ['--option', 'argument with spaces', str(path.resolve())])
        path.unlink()
        capture.unlink()
        result = self.run_cli('run', '--', sys.executable, str(runner), str(capture))
        self.assertNotEqual(result.returncode, 0)
        self.assertFalse(capture.exists())


if __name__ == '__main__':
    unittest.main()
