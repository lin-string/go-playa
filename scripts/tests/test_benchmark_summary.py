"""Correctness gates and reproducible benchmark aggregation."""
import copy
import hashlib
import importlib.util
import json
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest
from unittest.mock import patch

ROOT = Path(__file__).resolve().parents[2]


def module():
    path = ROOT / 'bench/summarize.py'
    if not path.is_file():
        raise AssertionError('benchmark summary support is missing')
    spec = importlib.util.spec_from_file_location('benchmark_summarize', path)
    value = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(value)
    return value


class SummaryTests(unittest.TestCase):
    def setUp(self):
        self.cells = [dict(fixture='book', source='local', task='text-glyph-jsonl', workers=worker, repeats=5)
                      for worker in (1, 2)]
        self.environment = dict(schema_version=1, run_id='test', preset='smoke', corpus_digest='a' * 64,
                                matrix_digest='b' * 64, cells=self.cells,
                                fixtures={'book': dict(id='book', pages=100, sha256='c' * 64,
                                  document_type='born-digital-text', languages=['en'], page_tier='50-199')})
        self.environment['implementation'] = dict(git_commit=None, git_dirty=True,
            source_sha256='f' * 64, source_file_count=2, source_identity_algorithm='sha256-path-mode-content-v1',
            binary=dict(path='/benchmark', sha256='0' * 64, bytes=100, origin='supplied-binary', build_command=None))
        identity_digest = hashlib.sha256(json.dumps(self.environment['implementation'], sort_keys=True, separators=(',', ':')).encode()).hexdigest()
        self.records = []
        for cell in self.cells:
            for repeat in range(1, 6):
                for order, library in enumerate(('go-playa', 'playa') if repeat % 2 else ('playa', 'go-playa'), 1):
                    self.records.append(dict(schema_version=1, run_id='test',
                        corpus_digest='a' * 64, matrix_digest='b' * 64, fixture='book', fixture_sha256='c' * 64,
                        task=cell['task'], requested_workers=cell['workers'], effective_workers=cell['workers'],
                        observed_workers=1, repeat=repeat, order=order, sample_index=len(self.records) + 1,
                        library=library, pdf='/book.pdf', pages=100, texts=20, glyphs=40, sha256='d' * 64,
                        elapsed_ns=repeat * 100 if library == 'go-playa' else repeat * 200,
                        peak_rss_bytes=repeat * 1000))
                    self.records[-1].update(implementation_digest=identity_digest,
                                           implementation_source_sha256='f' * 64, benchmark_binary_sha256='0' * 64)
                    if library == 'playa':
                        if cell['workers'] == 1:
                            self.records[-1]['rss_scope'] = 'single-process-lifetime-peak'
                        else:
                            self.records[-1].update(rss_scope='parent-lifetime-and-worker-callback-lower-bound',
                                                   peak_rss_bytes=0, parent_peak_rss_bytes=repeat * 1000,
                                                   callback_worker_peak_rss_sum_bytes=repeat * 500)

    def summarize(self, records=None, environment=None):
        return module().summarize(records if records is not None else self.records,
                                  environment or self.environment, 'a' * 64, 'b' * 64)

    def test_medians_percentiles_rss_throughput_and_speedup(self):
        summary = self.summarize()
        cell = summary['cells'][0]
        self.assertEqual(cell['speedup'], 2)
        self.assertEqual(cell['go']['median_elapsed_ns'], 300)
        self.assertEqual(cell['go']['p25_elapsed_ns'], 200)
        self.assertEqual(cell['go']['p75_elapsed_ns'], 400)
        self.assertEqual(cell['go']['median_peak_rss_bytes'], 3000)
        self.assertAlmostEqual(cell['go']['pages_per_second'], 100 * 1e9 / 300)
        self.assertEqual(cell['correctness'], 'canonical-digest-and-counts')

    def test_implementation_identity_is_required_consistent_and_exposed(self):
        summarizer = module()
        summary = self.summarize()
        self.assertEqual(summary['implementation'], self.environment['implementation'])
        output = summarizer.render_markdown(summary)
        self.assertIn('Source SHA-256', output)
        self.assertIn('Benchmark binary SHA-256', output)
        for corruption in ('missing', 'source', 'binary', 'dirty', 'raw'):
            env = copy.deepcopy(self.environment)
            records = copy.deepcopy(self.records)
            if corruption == 'missing':
                del env['implementation']
            elif corruption == 'source':
                env['implementation']['source_sha256'] = 'bad'
            elif corruption == 'binary':
                env['implementation']['binary']['sha256'] = 'bad'
            elif corruption == 'dirty':
                env['implementation']['git_dirty'] = 'yes'
            else:
                records[0]['benchmark_binary_sha256'] = '1' * 64
            with self.subTest(corruption=corruption), self.assertRaises(ValueError):
                self.summarize(records, env)

    def test_evidence_views_are_task_worker_scoped_and_deterministic(self):
        summary = self.summarize()
        views = summary['views']
        row = views['single_worker_tasks'][0]
        self.assertEqual(row['task'], 'text-glyph-jsonl')
        self.assertEqual(row['fixtures'], ['book'])
        self.assertEqual(row['speedup'], 2)
        self.assertEqual(row['go']['sum_cell_median_elapsed_ns'], 300)
        self.assertEqual([r['workers'] for r in views['scaling'][0]['rows']], [1, 2, 4, 8])
        self.assertEqual(views['scaling'][0]['rows'][2]['status'], 'not-measured')
        for key in ('by_document_type', 'by_language', 'by_page_tier'):
            self.assertEqual(len(views[key]), 2)
            self.assertEqual({r['workers'] for r in views[key]}, {1, 2})
        output = module().render_markdown(summary)
        for title in ('Single-worker task comparisons', 'Workers 1/2/4/8 scaling', 'By document type', 'By language', 'By page tier', 'Complete per-cell matrix'):
            self.assertIn(title, output)
        self.assertEqual(output, module().render_markdown(copy.deepcopy(summary)))

    def test_evidence_aggregation_preserves_blocked_cells_and_overlapping_languages(self):
        summarizer = module()
        cells = copy.deepcopy(self.summarize()['cells'])
        second = copy.deepcopy(cells[0])
        second.update(fixture='other', pages=200, languages=['en', 'fr'], speedup=None)
        second['go']['median_elapsed_ns'] = 600
        second['playa']['median_elapsed_ns'] = 1800
        cells.append(second)
        views = summarizer.evidence_views(cells)
        row = views['single_worker_tasks'][0]
        self.assertEqual(row['fixtures'], ['book', 'other'])
        self.assertEqual(row['blocked_cells'], 1)
        self.assertIsNone(row['speedup'])
        self.assertEqual(row['go']['sum_cell_median_elapsed_ns'], 900)
        self.assertAlmostEqual(row['go']['pages_per_second'], 300 * 1e9 / 900)
        french = next(row for row in views['by_language'] if row['value'] == 'fr')
        self.assertEqual(french['fixtures'], ['other'])
        english = next(row for row in views['by_language'] if row['value'] == 'en' and row['workers'] == 1)
        self.assertEqual(english['fixtures'], ['book', 'other'])

    def test_parallel_callback_lower_bound_is_not_formal_rss(self):
        summary = self.summarize()
        cell = summary['cells'][1]
        self.assertFalse(cell['playa']['rss_available'])
        self.assertEqual(cell['playa']['median_parent_peak_rss_bytes'], 3000)
        self.assertEqual(cell['playa']['median_callback_worker_peak_rss_sum_bytes'], 1500)
        records = copy.deepcopy(self.records)
        for record in records:
            if record['library'] == 'playa' and record['requested_workers'] == 2:
                record['peak_rss_bytes'] = 12345
        with self.assertRaises(ValueError):
            self.summarize(records)

    def test_task_specific_counts_and_digest_mismatches_reject(self):
        for task, field in (('open-pages', 'pages'), ('objects', 'objects'),
                            ('text-glyphs', 'glyphs'), ('text-glyphs', 'texts'),
                            ('layout-items', 'layout_lines'), ('layout-items', 'layout_items'),
                            ('image-digests', 'images'), ('text-glyph-jsonl', 'sha256')):
            environment = copy.deepcopy(self.environment)
            records = copy.deepcopy(self.records)
            if task == 'objects':
                environment['cells'] = environment['cells'][:1]
                records = records[:10]
            for cell in environment['cells']:
                cell['task'] = task
            for record in records:
                record['task'] = task
                if field != 'sha256':
                    record[field] = 1 if field != 'pages' else 100
            records[1][field] = 'e' * 64 if field == 'sha256' else records[1][field] + 1
            with self.subTest(task=task, field=field), self.assertRaises(ValueError):
                self.summarize(records, environment)

    def test_missing_digest_and_invalid_measurements_reject(self):
        for field, value in (('sha256', None), ('elapsed_ns', 0), ('elapsed_ns', True),
                              ('peak_rss_bytes', -1), ('effective_workers', 0), ('glyphs', False)):
            records = copy.deepcopy(self.records)
            records[0][field] = value
            with self.subTest(field=field), self.assertRaises(ValueError):
                self.summarize(records)

    def test_required_count_fields_cannot_be_silently_missing(self):
        summarizer = module()
        for task, fields in summarizer.COUNT_FIELDS.items():
            for field in fields:
                for invalid in ('missing', True, -1, 0.0):
                    records = copy.deepcopy(self.records)
                    environment = copy.deepcopy(self.environment)
                    if task == 'objects':
                        environment['cells'] = environment['cells'][:1]
                        records = records[:10]
                    for cell in environment['cells']:
                        cell['task'] = task
                    for record in records:
                        record['task'] = task
                        for required in fields:
                            record[required] = 100 if required == 'pages' else 1
                        if invalid == 'missing':
                            del record[field]
                        else:
                            record[field] = invalid
                    with self.subTest(task=task, field=field, invalid=invalid), self.assertRaises(ValueError):
                        self.summarize(records, environment)

    def test_image_digest_is_required_and_gates_all_signatures(self):
        environment = copy.deepcopy(self.environment)
        for cell in environment['cells']:
            cell['task'] = 'image-digests'
        baseline = copy.deepcopy(self.records)
        for record in baseline:
            record.update(task='image-digests', images=2)
        self.assertEqual(self.summarize(baseline, environment)['cells'][0]['correctness'], 'canonical-image-digest-and-counts')
        for corruption in ('missing', 'library', 'repeat', 'workers'):
            records = copy.deepcopy(baseline)
            if corruption == 'missing':
                for record in records:
                    del record['sha256']
            elif corruption == 'library':
                for record in records:
                    if record['library'] == 'playa':
                        record['sha256'] = 'e' * 64
            elif corruption == 'repeat':
                records[2]['sha256'] = 'e' * 64
            else:
                for record in records[10:]:
                    record['sha256'] = 'e' * 64
            if corruption == 'library':
                with self.subTest(corruption=corruption):
                    summary = self.summarize(records, environment)
                    self.assertEqual(summary['summary_status'], 'partial-correctness-mismatch')
                    self.assertTrue(all(cell['speedup'] is None for cell in summary['cells']))
            else:
                with self.subTest(corruption=corruption), self.assertRaises(ValueError):
                    self.summarize(records, environment)

    def test_atomic_publication_failures_preserve_existing_files(self):
        summarizer = module()
        summary = self.summarize()
        for operation in ('os.fsync', 'os.replace'):
            with self.subTest(operation=operation), tempfile.TemporaryDirectory() as temp:
                directory = Path(temp)
                (directory / 'summary.json').write_text('original JSON')
                (directory / 'summary.md').write_text('original Markdown')
                with patch(operation, side_effect=OSError('injected failure')):
                    with self.assertRaises(OSError):
                        summarizer.write_summary(directory, summary)
                self.assertEqual((directory / 'summary.json').read_text(), 'original JSON')
                self.assertEqual((directory / 'summary.md').read_text(), 'original Markdown')
                self.assertEqual(sorted(p.name for p in directory.iterdir()), ['summary.json', 'summary.md'])

    def test_stale_environment_or_raw_digests_reject(self):
        for field in ('corpus_digest', 'matrix_digest'):
            env = copy.deepcopy(self.environment)
            env[field] = 'e' * 64
            with self.assertRaisesRegex(ValueError, 'stale'):
                self.summarize(environment=env)
            records = copy.deepcopy(self.records)
            records[0][field] = 'e' * 64
            with self.assertRaises(ValueError):
                self.summarize(records)

    def test_incomplete_duplicate_extra_reordered_samples_reject(self):
        for records in (self.records[:-1], self.records + [self.records[0]],
                        [self.records[1], self.records[0]] + self.records[2:]):
            with self.assertRaises(ValueError):
                self.summarize(records)

    def test_cross_repeat_and_worker_signature_must_stay_identical(self):
        for indices in ([2], list(range(10, 20))):
            records = copy.deepcopy(self.records)
            for index in indices:
                records[index]['glyphs'] += 1
            with self.assertRaises(ValueError):
                self.summarize(records)

    def test_mismatched_effective_topology_blocks_speedup(self):
        records = copy.deepcopy(self.records)
        for record in records:
            if record['library'] == 'go-playa' and record['requested_workers'] == 2:
                record['effective_workers'] = 1
        cell = self.summarize(records)['cells'][1]
        self.assertIsNone(cell['speedup'])
        self.assertEqual(cell['speedup_blocked_reason'], 'effective-worker-mismatch')

    def test_stable_cross_implementation_mismatch_is_published_as_blocked(self):
        records = copy.deepcopy(self.records)
        for record in records:
            if record['library'] == 'playa':
                record['sha256'] = 'e' * 64
        summary = self.summarize(records)
        self.assertEqual(summary['summary_status'], 'partial-correctness-mismatch')
        self.assertEqual(summary['qualified_cell_count'], 0)
        self.assertEqual(summary['blocked_cell_count'], 2)
        for cell in summary['cells']:
            self.assertEqual(cell['correctness_status'], 'mismatch')
            self.assertIsNone(cell['speedup'])
            self.assertEqual(cell['speedup_blocked_reason'], 'task-specific-correctness-mismatch')
            self.assertIsNone(cell['counts'])
            self.assertIsNone(cell['sha256'])
            self.assertEqual(cell['task_signatures']['go-playa']['sha256'], 'd' * 64)
            self.assertEqual(cell['task_signatures']['playa']['sha256'], 'e' * 64)
            self.assertEqual(cell['mismatch_fields'], ['sha256'])
        single = summary['views']['single_worker_tasks']
        self.assertEqual(len(single), 1)
        self.assertIsNone(single[0]['speedup'])
        self.assertEqual(single[0]['blocked_reasons'], ['task-specific-correctness-mismatch'])
        measured = [row for row in summary['views']['scaling'][0]['rows'] if row['status'] == 'measured']
        for row in measured:
            self.assertIsNone(row['speedup'])
            self.assertIsNone(row['go_scale_vs_one'])
            self.assertIsNone(row['playa_scale_vs_one'])
            self.assertEqual(row['blocked_reason'], 'task-specific-correctness-mismatch')
        markdown = module().render_markdown(summary)
        self.assertIn('partial-correctness-mismatch', markdown)
        self.assertIn('task-specific-correctness-mismatch', markdown)
        self.assertNotIn('2.00x', markdown)

    def test_stable_count_only_cross_implementation_mismatch_is_published_as_blocked(self):
        environment = copy.deepcopy(self.environment)
        records = copy.deepcopy(self.records)
        for cell in environment['cells']:
            cell['task'] = 'text-glyphs'
        for record in records:
            record['task'] = 'text-glyphs'
            if record['library'] == 'playa':
                record['glyphs'] = 41
        summary = self.summarize(records, environment)
        self.assertEqual(summary['summary_status'], 'partial-correctness-mismatch')
        self.assertEqual(summary['qualified_cell_count'], 0)
        self.assertEqual(summary['blocked_cell_count'], 2)
        for cell in summary['cells']:
            self.assertEqual(cell['correctness_status'], 'mismatch')
            self.assertEqual(cell['mismatch_fields'], ['glyphs'])
            self.assertIsNone(cell['speedup'])
            self.assertEqual(cell['task_signatures']['go-playa']['counts']['glyphs'], 40)
            self.assertEqual(cell['task_signatures']['playa']['counts']['glyphs'], 41)

    def test_markdown_is_deterministic_and_preserves_dimension_and_rss_scope(self):
        summarizer = module()
        summary = self.summarize()
        output = summarizer.render_markdown(summary)
        self.assertEqual(output, summarizer.render_markdown(copy.deepcopy(summary)))
        for text in ('book', 'born-digital-text', 'en', '50-199', 'Size tier', 'glyphs=40', 'P25', 'P75', 'count', 'RSS', 'worker callback'):
            self.assertIn(text, output)

    def test_summary_cli_validates_saved_matrix_filters_and_fixture_pins(self):
        summarizer = module()
        corpus_path, matrix_path = ROOT / 'bench/corpus.json', ROOT / 'bench/matrix.json'
        matrix = summarizer.manifest.load_matrix(matrix_path)
        fixture = next(f for f in matrix['local_fixtures'] if f['id'] == 'smoke-text')
        cell = next(c for c in summarizer.manifest.expand_preset([], matrix, 'smoke')
                    if c['fixture'] == 'smoke-text' and c['task'] == 'text-glyph-jsonl' and c['workers'] == 1)
        environment = copy.deepcopy(self.environment)
        environment.update(corpus_digest=summarizer.manifest.manifest_digest(corpus_path),
                           matrix_digest=summarizer.manifest.manifest_digest(matrix_path), cells=[cell],
                           fixtures={'smoke-text': fixture},
                           filters={'fixtures': ['smoke-text'], 'tasks': ['text-glyph-jsonl'], 'workers': [1]})
        records = copy.deepcopy(self.records[:2])
        for record in records:
            record.update(fixture='smoke-text', fixture_sha256=fixture['sha256'], pages=fixture['pages'],
                          corpus_digest=environment['corpus_digest'], matrix_digest=environment['matrix_digest'])
        for corruption in (None, 'partial', 'stale', 'cells', 'fixture', 'missing-sample'):
            env = copy.deepcopy(environment)
            raw = copy.deepcopy(records)
            if corruption == 'stale':
                env['corpus_digest'] = 'e' * 64
            elif corruption == 'partial':
                next(record for record in raw if record['library'] == 'playa')['sha256'] = 'e' * 64
            elif corruption == 'cells':
                env['cells'][0]['repeats'] = 2
            elif corruption == 'fixture':
                env['fixtures']['smoke-text']['pages'] += 1
            elif corruption == 'missing-sample':
                raw.pop()
            with self.subTest(corruption=corruption), tempfile.TemporaryDirectory() as temp:
                directory = Path(temp)
                (directory / 'environment.json').write_text(json.dumps(env))
                (directory / 'raw.jsonl').write_text(''.join(json.dumps(r) + '\n' for r in raw))
                result = subprocess.run([sys.executable, str(ROOT / 'bench/summarize.py'), '--directory', temp],
                                        capture_output=True, text=True)
                expected_code = 0 if corruption is None else 2 if corruption == 'partial' else 1
                self.assertEqual(result.returncode, expected_code, result.stderr)
                published = corruption in (None, 'partial')
                self.assertEqual((directory / 'summary.json').exists(), published)
                self.assertEqual((directory / 'summary.md').exists(), published)
                if corruption == 'partial':
                    self.assertEqual(json.loads((directory / 'summary.json').read_text())['summary_status'],
                                     'partial-correctness-mismatch')


if __name__ == '__main__':
    unittest.main()
