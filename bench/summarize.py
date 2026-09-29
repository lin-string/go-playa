#!/usr/bin/env python3
"""Validate immutable benchmark samples before producing performance evidence."""
import argparse
import hashlib
import importlib.util
import json
import os
from pathlib import Path
import re
import statistics
import sys
import tempfile

ROOT = Path(__file__).resolve().parents[1]
spec = importlib.util.spec_from_file_location('benchmark_manifest', ROOT / 'scripts/benchmark_manifest.py')
manifest = importlib.util.module_from_spec(spec)
spec.loader.exec_module(manifest)

COUNT_FIELDS = {'open-pages': ('pages',), 'objects': ('pages', 'objects'),
                'text-glyphs': ('pages', 'texts', 'glyphs'),
                'layout-items': ('pages', 'layout_lines', 'layout_items'),
                'image-digests': ('pages', 'images'),
                'text-glyph-jsonl': ('pages', 'texts', 'glyphs')}
DIGEST_TASKS = ('text-glyph-jsonl', 'image-digests')


def file_sha256(path):
    digest = hashlib.sha256()
    with Path(path).open('rb') as stream:
        for block in iter(lambda: stream.read(1024 * 1024), b''):
            digest.update(block)
    return digest.hexdigest()


def implementation_digest(implementation):
    return hashlib.sha256(json.dumps(implementation, sort_keys=True, separators=(',', ':'), allow_nan=False).encode()).hexdigest()


def validate_implementation(implementation):
    manifest._fields(implementation, ('git_commit', 'git_dirty', 'source_sha256', 'source_file_count',
                                     'source_identity_algorithm', 'binary'))
    if implementation['git_commit'] is not None and (not isinstance(implementation['git_commit'], str) or not re.fullmatch(r'(?:[0-9a-f]{40}|[0-9a-f]{64})', implementation['git_commit'])):
        raise ValueError('invalid implementation Git commit')
    if implementation['git_dirty'] is not None and type(implementation['git_dirty']) is not bool:
        raise ValueError('implementation Git dirty state must be boolean or unavailable')
    if implementation['source_identity_algorithm'] != 'sha256-path-mode-content-v1':
        raise ValueError('unsupported implementation source identity algorithm')
    positive_integer(implementation['source_file_count'], 'source_file_count')
    binary = implementation['binary']
    manifest._fields(binary, ('path', 'sha256', 'bytes', 'origin', 'build_command'))
    for digest in (implementation['source_sha256'], binary['sha256']):
        if not isinstance(digest, str) or not re.fullmatch(r'[0-9a-f]{64}', digest):
            raise ValueError('invalid implementation source/binary SHA-256')
    if not isinstance(binary['path'], str) or not Path(binary['path']).is_absolute():
        raise ValueError('implementation binary path must be absolute')
    positive_integer(binary['bytes'], 'binary bytes')
    if binary['origin'] not in ('built-from-current-source', 'supplied-binary'):
        raise ValueError('invalid implementation binary origin')
    if binary['origin'] == 'supplied-binary' and binary['build_command'] is not None:
        raise ValueError('supplied binary cannot claim a local build command')
    if binary['origin'] == 'built-from-current-source' and binary['build_command'] != ['go', 'build', '-o', binary['path'], './bench/go']:
        raise ValueError('invalid implementation binary build provenance')


def strict_json(text):
    def constant(value):
        raise ValueError(f'non-finite JSON number: {value}')
    return json.loads(text, object_pairs_hook=manifest._unique_object, parse_constant=constant)


def positive_integer(value, field, minimum=1):
    if type(value) is not int or value < minimum:
        raise ValueError(f'{field} must be an integer >= {minimum}')


def signature(record):
    counts = tuple(record[field] for field in COUNT_FIELDS[record['task']])
    return counts + ((record['sha256'],) if record['task'] in DIGEST_TASKS else ())


def signature_payload(task, value):
    count_names = COUNT_FIELDS[task]
    return {
        'counts': dict(zip(count_names, value[:len(count_names)])),
        'sha256': value[-1] if task in DIGEST_TASKS else None,
    }


def signature_mismatch_fields(task, left, right):
    fields = list(COUNT_FIELDS[task]) + (['sha256'] if task in DIGEST_TASKS else [])
    return [field for field, left_value, right_value in zip(fields, left, right) if left_value != right_value]


def validate_record(record, fixture):
    if not isinstance(record, dict) or record.get('library') not in ('go-playa', 'playa'):
        raise ValueError('sample library must be go-playa or playa')
    if record.get('task') not in COUNT_FIELDS:
        raise ValueError('unknown sample task')
    for field in ('requested_workers', 'effective_workers', 'observed_workers', 'pages', 'elapsed_ns'):
        positive_integer(record.get(field), field)
    for field in ('texts', 'glyphs'):
        positive_integer(record.get(field), field, minimum=0)
    for field in COUNT_FIELDS[record['task']]:
        if field not in record:
            raise ValueError(f'missing required task count: {field}')
        positive_integer(record[field], field, minimum=0)
    for field in ('texts', 'glyphs', 'objects', 'layout_lines', 'layout_items', 'images', 'alloc_bytes', 'peak_rss_bytes', 'parent_peak_rss_bytes', 'callback_worker_peak_rss_sum_bytes'):
        if field in record:
            positive_integer(record[field], field, minimum=0)
    if record['effective_workers'] > record['requested_workers'] or record['observed_workers'] > record['effective_workers']:
        raise ValueError('sample worker topology exceeds configured limits')
    if record['pages'] != fixture['pages']:
        raise ValueError(f"{fixture['id']}: parsed page count disagrees with pin")
    if not isinstance(record.get('pdf'), str) or not record['pdf']:
        raise ValueError('missing sample PDF path')
    if record['task'] in DIGEST_TASKS and (
            not isinstance(record.get('sha256'), str) or not re.fullmatch(r'[0-9a-f]{64}', record['sha256'])):
        raise ValueError('canonical task digest is missing or invalid')
    if record['library'] == 'playa':
        scope = record.get('rss_scope')
        peak = record.get('peak_rss_bytes', 0)
        if scope == 'parent-lifetime-and-worker-callback-lower-bound':
            if peak != 0 or 'parent_peak_rss_bytes' not in record or 'callback_worker_peak_rss_sum_bytes' not in record:
                raise ValueError('parallel Playa callback lower bounds cannot be formal peak RSS')
        elif scope == 'single-process-lifetime-peak':
            if record['effective_workers'] != 1 or record.get('parent_peak_rss_bytes', peak) != peak or record.get('callback_worker_peak_rss_sum_bytes', 0) != 0:
                raise ValueError('single-process Playa RSS scope disagrees with worker/diagnostic fields')
        else:
            raise ValueError('missing or unknown Playa RSS scope')


def percentile(values, proportion):
    """Linear interpolation at (n-1)*p, including the one-sample smoke case."""
    values = sorted(values)
    position = (len(values) - 1) * proportion
    lower = int(position)
    upper = min(lower + 1, len(values) - 1)
    return values[lower] + (values[upper] - values[lower]) * (position - lower)


def aggregate(records):
    elapsed = [r['elapsed_ns'] for r in records]
    median = statistics.median(elapsed)
    effective = {r['effective_workers'] for r in records}
    if len(effective) != 1:
        raise ValueError('effective workers changed across repeats')
    rss = [r.get('peak_rss_bytes', 0) for r in records]
    return dict(samples=len(records), median_elapsed_ns=median,
                p25_elapsed_ns=percentile(elapsed, .25), p75_elapsed_ns=percentile(elapsed, .75),
                median_peak_rss_bytes=statistics.median(rss), rss_available=all(value > 0 for value in rss),
                rss_scopes=sorted({r.get('rss_scope', 'single-process-lifetime-peak') for r in records}),
                median_parent_peak_rss_bytes=statistics.median([r.get('parent_peak_rss_bytes', 0) for r in records]),
                median_callback_worker_peak_rss_sum_bytes=statistics.median([r.get('callback_worker_peak_rss_sum_bytes', 0) for r in records]),
                pages_per_second=records[0]['pages'] * 1e9 / median,
                effective_workers=effective.pop(),
                observed_workers=sorted({r['observed_workers'] for r in records}))


def summarize(records, environment, corpus_digest, matrix_digest):
    for field, expected in (('corpus_digest', corpus_digest), ('matrix_digest', matrix_digest)):
        if environment.get(field) != expected:
            raise ValueError(f'stale {field}; run was produced against a different manifest')
    if type(environment.get('schema_version')) is not int or environment['schema_version'] != 1:
        raise ValueError('unsupported environment schema')
    implementation = environment.get('implementation')
    validate_implementation(implementation)
    cells = environment.get('cells')
    fixtures = environment.get('fixtures')
    if not isinstance(cells, list) or not cells or not isinstance(fixtures, dict):
        raise ValueError('missing selected matrix or fixture identities')
    expected = []
    seen_cells = set()
    for cell in cells:
        manifest._fields(cell, ('fixture', 'source', 'task', 'workers', 'repeats'))
        key = (cell['fixture'], cell['task'], cell['workers'])
        if key in seen_cells or cell['task'] not in COUNT_FIELDS or cell['fixture'] not in fixtures or cell['source'] not in ('local', 'public'):
            raise ValueError('invalid or duplicate matrix cell')
        seen_cells.add(key)
        for field in ('workers', 'repeats'):
            positive_integer(cell[field], field)
        if cell['task'] in manifest.SEQUENTIAL_TASKS and cell['workers'] != 1:
            raise ValueError('sequential matrix tasks must request exactly one worker')
        for repeat in range(1, cell['repeats'] + 1):
            for order, library in enumerate(('go-playa', 'playa') if repeat % 2 else ('playa', 'go-playa'), 1):
                expected.append((cell, repeat, order, library))
    if len(records) != len(expected):
        raise ValueError(f'incomplete or extra samples: expected {len(expected)}, received {len(records)}')
    groups = {}
    signatures = {}
    paths = {}
    for index, (record, (cell, repeat, order, library)) in enumerate(zip(records, expected), 1):
        fixture = fixtures[cell['fixture']]
        validate_record(record, fixture)
        metadata = dict(schema_version=1, run_id=environment['run_id'], corpus_digest=corpus_digest,
                        matrix_digest=matrix_digest, fixture=cell['fixture'], fixture_sha256=fixture['sha256'],
                        task=cell['task'], requested_workers=cell['workers'], repeat=repeat,
                        order=order, sample_index=index, library=library,
                        implementation_digest=implementation_digest(implementation),
                        implementation_source_sha256=implementation['source_sha256'],
                        benchmark_binary_sha256=implementation['binary']['sha256'])
        if any(type(record.get(field)) is not type(value) or record.get(field) != value for field, value in metadata.items()):
            raise ValueError(f'sample {index} identity, ordering, or manifest metadata mismatch')
        path_key = cell['fixture']
        if path_key in paths and paths[path_key] != record['pdf']:
            raise ValueError('input path changed within run')
        paths[path_key] = record['pdf']
        # The same task signature must hold across both repeat and worker axes.
        signature_key = (cell['fixture'], cell['task'], library)
        current = signature(record)
        if signature_key in signatures and signatures[signature_key] != current:
            raise ValueError(f'{signature_key}: correctness signature changed across repeats/workers')
        signatures[signature_key] = current
        groups.setdefault((cell['fixture'], cell['task'], cell['workers'], library), []).append(record)
    result = []
    for cell in cells:
        fixture_id, task, workers = cell['fixture'], cell['task'], cell['workers']
        go_signature = signatures[(fixture_id, task, 'go-playa')]
        playa_signature = signatures[(fixture_id, task, 'playa')]
        correctness_matches = go_signature == playa_signature
        go, playa = (aggregate(groups[(fixture_id, task, workers, library)]) for library in ('go-playa', 'playa'))
        topology_matches = go['effective_workers'] == playa['effective_workers']
        blocked_reasons = []
        if not correctness_matches:
            blocked_reasons.append('task-specific-correctness-mismatch')
        if not topology_matches:
            blocked_reasons.append('effective-worker-mismatch')
        fixture = fixtures[fixture_id]
        result.append(dict(cell, pages=fixture['pages'], bytes=fixture.get('bytes'),
                      document_type=fixture['document_type'], languages=fixture['languages'],
                      page_tier=fixture.get('page_tier', manifest.page_tier(fixture['pages'])),
                      size_tier=fixture.get('size_tier', manifest.size_tier(fixture.get('bytes', 0))),
                      correctness=('canonical-image-digest-and-counts' if task == 'image-digests' else
                                   'canonical-digest-and-counts' if task == 'text-glyph-jsonl' else 'count-equivalence'),
                      correctness_status='matched' if correctness_matches else 'mismatch',
                      task_signatures={
                          'go-playa': signature_payload(task, go_signature),
                          'playa': signature_payload(task, playa_signature),
                      },
                      mismatch_fields=signature_mismatch_fields(task, go_signature, playa_signature),
                      counts=signature_payload(task, go_signature)['counts'] if correctness_matches else None,
                      sha256=go_signature[-1] if task in DIGEST_TASKS and correctness_matches else None,
                      go=go, playa=playa,
                      speedup=playa['median_elapsed_ns'] / go['median_elapsed_ns'] if not blocked_reasons else None,
                      speedup_blocked_reason=blocked_reasons[0] if blocked_reasons else None,
                      speedup_blocked_reasons=blocked_reasons))
    correctness_mismatches = sum(cell['correctness_status'] == 'mismatch' for cell in result)
    blocked = sum(cell['speedup'] is None for cell in result)
    return dict(schema_version=1, run_id=environment['run_id'], preset=environment['preset'],
                corpus_digest=corpus_digest, matrix_digest=matrix_digest,
                implementation=implementation, sample_count=len(records), cell_count=len(result),
                summary_status='partial-correctness-mismatch' if correctness_mismatches else 'complete',
                correctness_mismatch_cell_count=correctness_mismatches,
                qualified_cell_count=len(result) - blocked, blocked_cell_count=blocked,
                cells=result, views=evidence_views(result))


def evidence_views(cells):
    """Sum per-fixture medians only within one task and requested-worker setting."""
    def combine(selected, task, workers, dimension=None, value=None):
        pages = sum(cell['pages'] for cell in selected)
        go_ns, playa_ns = (sum(cell[library]['median_elapsed_ns'] for cell in selected) for library in ('go', 'playa'))
        blocked = sum(cell['speedup'] is None for cell in selected)
        blocked_reasons = sorted({reason for cell in selected for reason in cell.get('speedup_blocked_reasons', [])})
        row = dict(task=task, workers=workers, cell_count=len(selected), fixtures=sorted(cell['fixture'] for cell in selected),
                   total_pages=pages, correctness=sorted({cell['correctness'] for cell in selected}), blocked_cells=blocked,
                   blocked_reasons=blocked_reasons,
                   go=dict(sum_cell_median_elapsed_ns=go_ns, pages_per_second=pages * 1e9 / go_ns),
                   playa=dict(sum_cell_median_elapsed_ns=playa_ns, pages_per_second=pages * 1e9 / playa_ns),
                   speedup=playa_ns / go_ns if not blocked else None)
        if dimension is not None:
            row.update(dimension=dimension, value=value)
        return row
    def grouped(dimension):
        groups = {}
        for cell in cells:
            values = cell['languages'] if dimension == 'language' else [cell[dimension]]
            for value in values:
                groups.setdefault((value, cell['task'], cell['workers']), []).append(cell)
        return [combine(groups[key], key[1], key[2], dimension, key[0]) for key in sorted(groups)]
    single = []
    for task in manifest.TASKS:
        selected = [cell for cell in cells if cell['task'] == task and cell['workers'] == 1]
        if selected:
            single.append(combine(selected, task, 1))
    scaling = []
    pairs = sorted({(cell['fixture'], cell['task']) for cell in cells if cell['task'] not in manifest.SEQUENTIAL_TASKS})
    for fixture, task in pairs:
        selected = {cell['workers']: cell for cell in cells if cell['fixture'] == fixture and cell['task'] == task}
        baseline = selected.get(1)
        rows = []
        for workers in sorted({1, 2, 4, 8} | selected.keys()):
            cell = selected.get(workers)
            row = dict(workers=workers, status='measured' if cell else 'not-measured')
            if cell:
                correctness_scaling_matches = (baseline is not None and baseline['correctness_status'] == 'matched'
                                               and cell['correctness_status'] == 'matched')
                row.update(go=cell['go'], playa=cell['playa'], speedup=cell['speedup'], correctness=cell['correctness'],
                           correctness_status=cell['correctness_status'], blocked_reason=cell['speedup_blocked_reason'],
                           go_scale_vs_one=baseline['go']['median_elapsed_ns'] / cell['go']['median_elapsed_ns'] if correctness_scaling_matches else None,
                           playa_scale_vs_one=baseline['playa']['median_elapsed_ns'] / cell['playa']['median_elapsed_ns'] if correctness_scaling_matches else None)
            rows.append(row)
        scaling.append(dict(fixture=fixture, task=task, rows=rows))
    return dict(single_worker_tasks=single, scaling=scaling,
                by_document_type=grouped('document_type'), by_language=grouped('language'), by_page_tier=grouped('page_tier'),
                aggregation_semantics='Within each task/requested-worker group, sum fixture median elapsed times and pages; aggregate speedup is the ratio of those sums only when every constituent correctness signature and topology matches. Multi-language fixtures contribute in full to each language view; language groups overlap. Counts and canonical digests qualify only their task projections. No memory ratios or universal performance claims are derived.')


def render_markdown(summary):
    def safe(value):
        return str(value).replace('|', '\\|').replace('\n', ' ')
    lines = [f"# Benchmark matrix: {safe(summary['run_id'])}", '',
             f"Preset: {safe(summary['preset'])}; {summary['cell_count']} cells; {summary['sample_count']} fresh-process samples.", '',
             f"Summary status: `{summary['summary_status']}`; {summary['qualified_cell_count']} ratio-qualified cells; {summary['blocked_cell_count']} blocked cells; {summary['correctness_mismatch_cell_count']} correctness-mismatch cells.", '',
             f"Corpus SHA-256: `{summary['corpus_digest']}`; matrix SHA-256: `{summary['matrix_digest']}`.", '',
             f"Git commit: `{safe(summary['implementation']['git_commit'] or 'unavailable/unborn')}`; dirty: `{safe(summary['implementation']['git_dirty'])}`.", '',
             f"Source SHA-256: `{summary['implementation']['source_sha256']}` ({summary['implementation']['source_file_count']} repository files; {summary['implementation']['source_identity_algorithm']}).", '',
             f"Benchmark binary SHA-256: `{summary['implementation']['binary']['sha256']}`; path: `{safe(summary['implementation']['binary']['path'])}`; origin: `{summary['implementation']['binary']['origin']}`.", '',
             'For a supplied binary, the source identity describes checkout context, not its build source. Binary SHA-256 identifies the executed implementation.', '',
             'Count equivalence does not establish complete semantic equivalence. Text-glyph-jsonl and image-digests add canonical digest gates for their projections and decoded image streams.', '',
             'Formal RSS is a lifetime process peak for Go and single-process Playa. Parallel Playa formal RSS is unavailable: parent lifetime peak and worker callback peak sums are diagnostics only. Worker callback sums are lower bounds, not complete lifetime peaks or simultaneous aggregate memory. Zero formal RSS means unavailable. Observed workers use callback concurrency in Go and distinct callback processes in Playa.', '',
             'Quantiles use linear interpolation at (n-1)*p. Speedup is Playa median elapsed / Go median elapsed and is blocked for differing correctness signatures or effective workers. A partial summary preserves diagnostic timings but is not a fully qualified benchmark run.', '']
    views = summary['views']
    lines.extend([views['aggregation_semantics'], '', '## Single-worker task comparisons', '',
                  '| Task | Fixtures | Pages | Go summed medians (ms) | Playa summed medians (ms) | Speedup | Correctness |',
                  '| --- | --- | ---: | ---: | ---: | --- | --- |'])
    def aggregate_line(row, dimension=False):
        ratio = (f"{row['speedup']:.2f}x" if row['speedup'] is not None else
                 'blocked: ' + ','.join(row['blocked_reasons'] or ['unavailable']))
        values = ([row['value'], row['task'], row['workers']] if dimension else [row['task']]) + [','.join(row['fixtures']), row['total_pages'],
                  f"{row['go']['sum_cell_median_elapsed_ns'] / 1e6:.3f}", f"{row['playa']['sum_cell_median_elapsed_ns'] / 1e6:.3f}",
                  ratio, ','.join(row['correctness'])]
        return '| ' + ' | '.join(safe(value) for value in values) + ' |'
    lines.extend(aggregate_line(row) for row in views['single_worker_tasks'])
    lines.extend(['', '## Workers 1/2/4/8 scaling', '',
                  'Each entry shows Go / Playa scaling versus its own one-worker median and the cross-library ratio. Missing baselines give unavailable scaling; unmeasured workers are explicit. Effective and observed workers remain in the complete table.', '',
                  '| Fixture | Task | Workers 1 | Workers 2 | Workers 4 | Workers 8 |', '| --- | --- | --- | --- | --- | --- |'])
    for group in views['scaling']:
        by_worker = {row['workers']: row for row in group['rows']}
        values = [group['fixture'], group['task']]
        for workers in (1, 2, 4, 8):
            row = by_worker[workers]
            if row['status'] == 'not-measured':
                values.append('not measured')
            else:
                scale = lambda value: f'{value:.2f}x' if value is not None else 'unavailable'
                reason = f"; blocked: {row['blocked_reason']}" if row.get('blocked_reason') else ''
                values.append(f"{scale(row['go_scale_vs_one'])} / {scale(row['playa_scale_vs_one'])}; Go vs Playa {scale(row['speedup'])}{reason}")
        lines.append('| ' + ' | '.join(safe(value) for value in values) + ' |')
    for title, key in (('By document type', 'by_document_type'), ('By language', 'by_language'), ('By page tier', 'by_page_tier')):
        lines.extend(['', f'## {title}', '',
                      '| Group | Task | Requested workers | Fixtures | Pages | Go summed medians (ms) | Playa summed medians (ms) | Speedup | Correctness |',
                      '| --- | --- | ---: | --- | ---: | ---: | ---: | --- | --- |'])
        lines.extend(aggregate_line(row, dimension=True) for row in views[key])
    lines.extend(['', '## Complete per-cell matrix', '',
             '| Fixture | Type | Languages | Page tier | Size tier | Pages | Bytes | Task | Requested workers | Repeats | Go / Playa effective | Go / Playa observed | Go median / P25 / P75 (ms) | Playa median / P25 / P75 (ms) | Go / Playa pages/s | Go / Playa median RSS (MiB) | Speedup | Correctness | Go signature | Playa signature |',
             '| --- | --- | --- | --- | --- | ---: | ---: | --- | ---: | ---: | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |'])
    for cell in summary['cells']:
        go, playa = cell['go'], cell['playa']
        elapsed = lambda r: ' / '.join(f"{r[f'{field}_elapsed_ns'] / 1e6:.3f}" for field in ('median', 'p25', 'p75'))
        rss = lambda r: f"{r['median_peak_rss_bytes'] / 2**20:.2f}" if r['rss_available'] else 'unavailable'
        speed = (f"{cell['speedup']:.2f}x" if cell['speedup'] is not None else
                 'blocked: ' + ','.join(cell['speedup_blocked_reasons']))
        def render_signature(library):
            value = cell['task_signatures'][library]
            counts = ', '.join(f'{key}={item}' for key, item in value['counts'].items())
            return counts + (f", sha256={value['sha256']}" if value['sha256'] else '')
        values = [cell['fixture'], cell['document_type'], ','.join(cell['languages']), cell['page_tier'], cell['size_tier'],
                  cell['pages'], cell['bytes'], cell['task'], cell['workers'], cell['repeats'],
                  f"{go['effective_workers']} / {playa['effective_workers']}",
                  f"{','.join(map(str, go['observed_workers']))} / {','.join(map(str, playa['observed_workers']))}",
                  elapsed(go), elapsed(playa), f"{go['pages_per_second']:.2f} / {playa['pages_per_second']:.2f}",
                  f'{rss(go)} / {rss(playa)}', speed, f"{cell['correctness']} ({cell['correctness_status']})",
                  render_signature('go-playa'), render_signature('playa')]
        lines.append('| ' + ' | '.join(safe(value) for value in values) + ' |')
    return '\n'.join(lines) + '\n'


def write_summary(directory, summary):
    artifacts = {'summary.json': json.dumps(summary, indent=2, sort_keys=True, allow_nan=False) + '\n',
                 'summary.md': render_markdown(summary)}
    for name, text in artifacts.items():
        temporary = None
        try:
            with tempfile.NamedTemporaryFile(mode='w', encoding='utf-8', dir=directory,
                                             prefix='.' + name + '.', suffix='.tmp', delete=False) as stream:
                temporary = Path(stream.name)
                stream.write(text)
                stream.flush()
                os.fsync(stream.fileno())
            os.replace(temporary, directory / name)
        finally:
            if temporary is not None:
                temporary.unlink(missing_ok=True)


def main(argv=None):
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--directory', '--output', type=Path, required=True)
    parser.add_argument('--corpus', type=Path, default=ROOT / 'bench/corpus.json')
    parser.add_argument('--matrix', type=Path, default=ROOT / 'bench/matrix.json')
    args = parser.parse_args(argv)
    try:
        environment = strict_json((args.directory / 'environment.json').read_text(encoding='utf-8'))
        validate_implementation(environment.get('implementation'))
        binary = environment['implementation']['binary']
        binary_path = Path(binary['path'])
        if binary_path.exists() and (binary_path.stat().st_size != binary['bytes'] or file_sha256(binary_path) != binary['sha256']):
            raise ValueError('benchmark binary no longer matches saved provenance')
        corpus = manifest.load_corpus(args.corpus)
        matrix = manifest.load_matrix(args.matrix)
        expanded = manifest.expand_preset(corpus, matrix, environment['preset'])
        filters = environment.get('filters', {})
        cells = [cell for cell in expanded if all(not filters.get(field) or cell[attribute] in filters[field]
                 for field, attribute in (('fixtures', 'fixture'), ('tasks', 'task'), ('workers', 'workers')))]
        if cells != environment['cells']:
            raise ValueError('selected cells disagree with current matrix and saved filters')
        known = {fixture['id']: fixture for fixture in (corpus if environment['preset'] != 'smoke' else matrix['local_fixtures'])}
        if {key: known[key] for key in sorted({cell['fixture'] for cell in cells})} != environment['fixtures']:
            raise ValueError('saved fixture identities disagree with current manifests')
        records = [strict_json(line) for line in (args.directory / 'raw.jsonl').read_text(encoding='utf-8').splitlines()]
        summary = summarize(records, environment, manifest.manifest_digest(args.corpus), manifest.manifest_digest(args.matrix))
        write_summary(args.directory, summary)
        print(json.dumps(dict(directory=str(args.directory), cells=summary['cell_count'], samples=summary['sample_count'])))
        if summary['correctness_mismatch_cell_count']:
            print(f"benchmark-summary: wrote partial evidence with {summary['correctness_mismatch_cell_count']} correctness-mismatch cells", file=sys.stderr)
            return 2
        return 0
    except (OSError, ValueError, KeyError, TypeError) as error:
        print(f'benchmark-summary: {error}', file=sys.stderr)
        return 1


if __name__ == '__main__':
    sys.exit(main())
