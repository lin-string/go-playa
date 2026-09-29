"""Offline contracts for the independent benchmark corpus and matrix."""
import copy
import hashlib
import importlib.util
import json
from pathlib import Path
import tempfile
import subprocess
import sys
import unittest

ROOT = Path(__file__).resolve().parents[2]
SCRIPT = ROOT / 'scripts/benchmark_manifest.py'


class BenchmarkManifestTest(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.path = Path(self.temp.name) / 'manifest.json'
        self.fixture = dict(id='sample-book', title='Sample Book',
                            url='https://example.org/sample.pdf',
                            source_url='https://example.org/book',
                            license='CC-BY-4.0',
                            license_url='https://creativecommons.org/licenses/by/4.0/',
                            attribution='Example authors', document_type='born-digital-text',
                            languages=['en', 'zh-Hans'], producer_family='TeX',
                            features=['text', 'images'], status='pinned',
                            sha256='a' * 64, bytes=1000, pages=100,
                            pdf_version='1.7', encrypted=False, text_layer='native',
                            page_tier='50-199', size_tier='under-10-mib')
        self.matrix = dict(schema_version=1, local_fixtures=[dict(
            id='local-text', path='testdata/files/form_simple.pdf', pages=1,
            bytes=771, sha256='65fb0ba4a419861c53c3d4e6953e78c425232d03f990478bb1d2fa37ade60214',
            document_type='born-digital-text', languages=['en'], features=['text'])],
            presets={'smoke': dict(source='local', repeats=1, workers=[1, 2],
                       tasks=['open-pages', 'text-glyphs'], selectors={}),
                     'reference': dict(source='public', repeats=5, workers=[1, 4],
                       tasks=['open-pages', 'objects', 'text-glyphs', 'layout-items',
                              'image-digests', 'text-glyph-jsonl'], selectors={}),
                     'scaling': dict(source='public', repeats=5, workers=[1, 2, 4, 8],
                       tasks=['open-pages'], selectors={'ids': ['sample-book']})})

    def module(self):
        self.assertTrue(SCRIPT.is_file(), 'benchmark manifest support is missing')
        spec = importlib.util.spec_from_file_location('benchmark_manifest', SCRIPT)
        module = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(module)
        return module

    def write(self, value):
        self.path.write_text(json.dumps(value), encoding='utf-8')
        return self.path

    def corpus(self, fixtures=None, **kwargs):
        return self.module().load_corpus(self.write(dict(schema_version=1,
                    fixtures=fixtures if fixtures is not None else [self.fixture])), **kwargs)

    def test_valid_corpus_preserves_metadata(self):
        self.assertEqual(self.corpus(), [self.fixture])

    def test_required_metadata(self):
        for field in self.fixture:
            with self.subTest(field=field):
                fixture = dict(self.fixture)
                del fixture[field]
                with self.assertRaises(ValueError):
                    self.corpus([fixture])

    def test_rejects_malformed_values(self):
        invalid = [('id', '../bad'), ('id', 'CAPS'), ('title', ' '),
                   ('bytes', True), ('pages', 0), ('sha256', 'A' * 64),
                   ('document_type', 'text'), ('languages', ['en_XX']),
                   ('languages', ['en', 'en']), ('languages', []),
                   ('features', ['unrecognized']), ('features', ['text', 'text']),
                   ('producer_family', ''), ('status', 'ready'),
                   ('encrypted', 0), ('pdf_version', '2'),
                   ('text_layer', 'maybe'), ('page_tier', '500-999'),
                   ('size_tier', '200-mib-plus')]
        for field, value in invalid:
            with self.subTest(field=field, value=value), self.assertRaises(ValueError):
                self.corpus([dict(self.fixture, **{field: value})])

    def test_unsafe_urls(self):
        for field in ('url', 'source_url', 'license_url'):
            for url in ('http://example.org/f.pdf', 'https:///f.pdf',
                        'https://user:pass@example.org/f.pdf',
                        'https://example.org/f.pdf#part', 'https://example.org/\nfile'):
                with self.subTest(field=field, url=url), self.assertRaises(ValueError):
                    self.corpus([dict(self.fixture, **{field: url})])

    def test_schema_duplicate_ids_and_unknown_fields(self):
        module = self.module()
        for value in (dict(schema_version=True, fixtures=[self.fixture]),
                      dict(schema_version=1, fixtures=[]),
                      dict(schema_version=1, fixtures=[self.fixture, self.fixture]),
                      dict(schema_version=1, fixtures=[dict(self.fixture, typo='oops')])):
            with self.subTest(value=value), self.assertRaises(ValueError):
                module.load_corpus(self.write(value))

    def test_duplicate_json_keys_rejected(self):
        self.path.write_text('{"schema_version":1,"schema_version":1,"fixtures":[]}')
        with self.assertRaisesRegex(ValueError, 'duplicate'):
            self.module().load_corpus(self.path)

    def test_pending_pins_are_explicit_and_not_executable(self):
        fixture = dict(self.fixture, status='pin-pending',
                       pin_notes='Await exact file admission',
                       sha256=None, bytes=None, pages=None, pdf_version=None,
                       encrypted=None, text_layer=None, source_pages=100, source_bytes=1000)
        with self.assertRaisesRegex(ValueError, 'pin-pending'):
            self.corpus([fixture])
        self.assertEqual(self.corpus([fixture], allow_pending=True), [fixture])
        with self.assertRaises(ValueError):
            self.corpus([dict(fixture, pin_notes='')], allow_pending=True)

    def test_pending_reported_dimensions_must_match_declared_tiers(self):
        fixture = dict(self.fixture, status='pin-pending', pin_notes='Await admission',
                       sha256=None, bytes=None, pages=None, pdf_version=None,
                       encrypted=None, text_layer=None, source_pages=1000, source_bytes=1000)
        with self.assertRaisesRegex(ValueError, 'tier'):
            self.corpus([fixture], allow_pending=True)

    def pending(self):
        return dict(self.fixture, status='pin-pending', pin_notes='Await admission',
                    sha256=None, bytes=None, pages=None, pdf_version=None,
                    encrypted=None, text_layer=None, source_pages=100, source_bytes=1000)

    def test_pending_requires_both_reported_dimensions(self):
        for field in ('source_pages', 'source_bytes'):
            fixture = self.pending()
            del fixture[field]
            with self.subTest(field=field), self.assertRaises(ValueError):
                self.corpus([fixture], allow_pending=True)

    def test_pending_classification_is_validated(self):
        for changes in (dict(document_type='image-only-scan'),
                        dict(document_type='ocr-scan')):
            with self.subTest(changes=changes), self.assertRaises(ValueError):
                self.corpus([dict(self.pending(), **changes)], allow_pending=True)

    def test_executable_expansion_refuses_selected_pending_fixtures(self):
        module = self.module()
        matrix = module.load_matrix(self.write(self.matrix))
        with self.assertRaisesRegex(ValueError, 'pin-pending'):
            module.expand_preset([self.pending()], matrix, 'reference')
        self.assertTrue(module.expand_preset([self.pending()], matrix, 'reference', allow_pending=True))

    def test_local_fixture_contract_detects_missing_or_changed_files(self):
        module = self.module()
        matrix = module.load_matrix(ROOT / 'bench/matrix.json')
        page_counts = {fixture['id']: 1 for fixture in matrix['local_fixtures']}
        module.verify_local_fixtures(matrix, ROOT, page_counts=page_counts)
        with self.assertRaises(ValueError):
            module.verify_local_fixtures(matrix, Path(self.temp.name))
        changed = copy.deepcopy(matrix)
        changed['local_fixtures'][0]['pages'] += 1
        with self.assertRaises(ValueError):
            module.verify_local_fixtures(changed, ROOT, page_counts=page_counts)
        with self.assertRaises(ValueError):
            module.verify_local_fixtures(matrix, ROOT,
                page_counts={fixture['id']: True for fixture in matrix['local_fixtures']})

    def test_text_layer_features_and_document_class_must_agree(self):
        for changes in (dict(text_layer='none'), dict(document_type='image-only-scan'),
                        dict(document_type='ocr-scan'), dict(text_layer='ocr')):
            with self.subTest(changes=changes), self.assertRaisesRegex(ValueError, 'feature|class|layer'):
                self.corpus([dict(self.fixture, **changes)])

    def test_pending_cli_execution_fails_before_any_download(self):
        directory = Path(self.temp.name) / 'downloaded'
        self.write(dict(schema_version=1, fixtures=[self.pending()]))
        for action in ('pull', 'check'):
            result = subprocess.run([sys.executable, str(SCRIPT), '--corpus', str(self.path),
                                     '--directory', str(directory), action],
                                    cwd=ROOT, text=True, capture_output=True)
            self.assertEqual(result.returncode, 1)
            self.assertIn('pin-pending', result.stderr)
            self.assertEqual(result.stdout, '')
            self.assertFalse(directory.exists())

    def test_manifest_digest_hashes_exact_bytes(self):
        path = self.write(self.matrix)
        self.assertEqual(self.module().manifest_digest(path), hashlib.sha256(path.read_bytes()).hexdigest())

    def test_matrix_rejects_duplicate_workers_and_unknown_tasks(self):
        for key, value in [('workers', [1, 1]), ('workers', [True]),
                           ('workers', [0]), ('workers', []),
                           ('tasks', ['bogus']), ('tasks', ['objects', 'objects']),
                           ('repeats', True), ('repeats', 0), ('source', 'compat')]:
            data = copy.deepcopy(self.matrix)
            data['presets']['reference'][key] = value
            with self.subTest(key=key, value=value), self.assertRaises(ValueError):
                self.module().load_matrix(self.write(data))

    def test_matrix_rejects_unsafe_local_paths_and_duplicate_ids(self):
        for path in ('../a.pdf', '/tmp/a.pdf', '.compat-cache/a.pdf',
                     'testdata/files/../../a.pdf', 'testdata/files/a.txt'):
            data = copy.deepcopy(self.matrix)
            data['local_fixtures'][0]['path'] = path
            with self.subTest(path=path), self.assertRaises(ValueError):
                self.module().load_matrix(self.write(data))
        data = copy.deepcopy(self.matrix)
        data['local_fixtures'].append(data['local_fixtures'][0])
        with self.assertRaises(ValueError):
            self.module().load_matrix(self.write(data))

    def test_matrix_rejects_unknown_selector_fields_and_values(self):
        for selectors in ({'typo': ['text']}, {'document_types': ['unknown']},
                          {'features_all': ['unknown']}, {'languages': ['en_bad']},
                          {'ids': []}):
            data = copy.deepcopy(self.matrix)
            data['presets']['reference']['selectors'] = selectors
            with self.subTest(selectors=selectors), self.assertRaises(ValueError):
                self.module().load_matrix(self.write(data))

    def test_expansion_applies_features_and_is_deterministic(self):
        module = self.module()
        matrix = module.load_matrix(self.write(self.matrix))
        scan = dict(self.fixture, id='scan-book', document_type='image-only-scan',
                    features=['images'], text_layer='none')
        cells = module.expand_preset([scan, self.fixture], matrix, 'reference')
        self.assertEqual(cells, module.expand_preset([self.fixture, scan], matrix, 'reference'))
        self.assertEqual(len(cells), 16)
        self.assertEqual(cells[0], dict(fixture='sample-book', source='public',
                         task='open-pages', workers=1, repeats=5))
        self.assertFalse(any(cell['task'].startswith('text-')
                             for cell in cells if cell['fixture'] == 'scan-book'))

    def test_selectors_use_metadata_and_reject_unresolved_ids(self):
        module = self.module()
        data = copy.deepcopy(self.matrix)
        data['presets']['reference']['selectors'] = dict(
            document_types=['born-digital-text'], languages=['zh-Hans'],
            features_all=['text'], page_tiers=['50-199'], size_tiers=['under-10-mib'])
        matrix = module.load_matrix(self.write(data))
        self.assertEqual(len(module.expand_preset([self.fixture], matrix, 'reference')), 11)
        with self.assertRaisesRegex(ValueError, 'unknown.*fixture|unresolved'):
            module.expand_preset([], matrix, 'scaling')

    def test_smoke_uses_only_local_fixtures(self):
        module = self.module()
        matrix = module.load_matrix(self.write(self.matrix))
        cells = module.expand_preset([self.fixture], matrix, 'smoke')
        self.assertEqual(len(cells), 4)
        self.assertEqual({cell['fixture'] for cell in cells}, {'local-text'})
        self.assertEqual({cell['source'] for cell in cells}, {'local'})

    def test_sequential_tasks_expand_once_and_force_one_requested_worker(self):
        module = self.module()
        matrix = module.load_matrix(ROOT / 'bench/matrix.json')
        corpus = module.load_corpus(ROOT / 'bench/corpus.json')
        for preset, expected in (('smoke', 14), ('reference', 77), ('scaling', 89)):
            cells = module.expand_preset(corpus, matrix, preset)
            self.assertEqual(len(cells), expected)
            sequential = [cell for cell in cells if cell['task'] == 'objects']
            self.assertEqual({cell['workers'] for cell in sequential}, {1})
            self.assertEqual(len(sequential), len({cell['fixture'] for cell in sequential}))
        matrix['presets']['reference']['workers'] = [4, 8]
        cells = module.expand_preset(corpus, matrix, 'reference')
        self.assertEqual({cell['workers'] for cell in cells if cell['task'] == 'objects'}, {1})

    def test_repository_corpus_covers_declared_dimensions(self):
        module = self.module()
        corpus = module.load_corpus(ROOT / 'bench/corpus.json')
        matrix = module.load_matrix(ROOT / 'bench/matrix.json')
        self.assertGreaterEqual(len(corpus), 9)
        self.assertEqual({item['document_type'] for item in corpus}, set(module.DOCUMENT_TYPES))
        self.assertTrue({'50-199', '200-499', '500-999', '1000-plus'} <=
                        {item['page_tier'] for item in corpus})
        self.assertIn('200-mib-plus', {item['size_tier'] for item in corpus})
        languages = {language for item in corpus for language in item['languages']}
        self.assertTrue({'en', 'zh-Hant', 'ar', 'ja', 'ko', 'fr'} <= languages)
        for preset in ('smoke', 'reference', 'scaling'):
            self.assertTrue(module.expand_preset(corpus, matrix, preset))

    def test_reference_requires_all_pinned_fixtures_without_selector_gaps(self):
        module = self.module()
        corpus = module.load_corpus(ROOT / 'bench/corpus.json')
        matrix = module.load_matrix(ROOT / 'bench/matrix.json')
        cells = module.expand_preset(corpus, matrix, 'reference')
        self.assertEqual({cell['fixture'] for cell in cells}, {fixture['id'] for fixture in corpus})
        limited = copy.deepcopy(matrix)
        limited['presets']['reference']['selectors'] = {'ids': [corpus[0]['id']]}
        with self.assertRaisesRegex(ValueError, 'reference.*all|reference.*coverage'):
            module.expand_preset(corpus, limited, 'reference')

    def test_scaling_requires_exactly_one_pinned_representative_of_each_class(self):
        module = self.module()
        corpus = module.load_corpus(ROOT / 'bench/corpus.json')
        matrix = module.load_matrix(ROOT / 'bench/matrix.json')
        cells = module.expand_preset(corpus, matrix, 'scaling')
        selected = {cell['fixture'] for cell in cells}
        classes = [fixture['document_type'] for fixture in corpus if fixture['id'] in selected]
        self.assertCountEqual(classes, module.DOCUMENT_TYPES)
        self.assertTrue(all(fixture['status'] == 'pinned' for fixture in corpus if fixture['id'] in selected))
        for identifiers in ([corpus[0]['id']],
                            matrix['presets']['scaling']['selectors']['ids'] + ['gnu-libc-manual']):
            limited = copy.deepcopy(matrix)
            limited['presets']['scaling']['selectors'] = {'ids': identifiers}
            with self.subTest(identifiers=identifiers), self.assertRaisesRegex(ValueError, 'scaling.*class|scaling.*representative'):
                module.expand_preset(corpus, limited, 'scaling')


if __name__ == '__main__':
    unittest.main()
