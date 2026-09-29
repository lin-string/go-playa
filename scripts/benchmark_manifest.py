#!/usr/bin/env python3
"""Strict, offline benchmark contracts; only the explicit pull action downloads.

Pending candidates carry null admission fields. They are accepted only for
manifest review and dry-run planning, never for downloading or benchmarking.
"""
import argparse
import hashlib
import importlib.util
import json
from pathlib import Path, PurePosixPath
import re
import sys
import urllib.parse

DOCUMENT_TYPES = ('born-digital-text', 'born-digital-graphics', 'image-only-scan',
                  'ocr-scan', 'mixed')
FEATURES = ('text', 'images', 'vectors', 'tables', 'formulas', 'outline', 'rtl',
            'vertical-text', 'ocr', 'scanned-pages', 'native-pages', 'fonts')
TASKS = ('open-pages', 'objects', 'text-glyphs', 'layout-items', 'image-digests',
         'text-glyph-jsonl')
# Extend this explicit topology contract when adding another sequential task.
SEQUENTIAL_TASKS = ('objects',)
TASK_FEATURES = {'text-glyphs': 'text', 'layout-items': 'text',
                 'text-glyph-jsonl': 'text', 'image-digests': 'images'}
PAGE_TIERS = ('under-50', '50-199', '200-499', '500-999', '1000-plus')
SIZE_TIERS = ('under-10-mib', '10-49-mib', '50-199-mib', '200-mib-plus')
ID_PATTERN = r'[a-z0-9]+(?:-[a-z0-9]+)*'
LANGUAGE_PATTERN = r'[a-z]{2,3}(?:-[A-Z][a-z]{3})?(?:-(?:[A-Z]{2}|[0-9]{3}))?(?:-[a-z0-9]{5,8})*'


def _unique_object(pairs):
    result = {}
    for key, value in pairs:
        if key in result:
            raise ValueError(f'duplicate JSON key: {key}')
        result[key] = value
    return result


def _read(path):
    try:
        data = json.loads(Path(path).read_text(encoding='utf-8'),
                          object_pairs_hook=_unique_object)
        if not isinstance(data, dict) or type(data.get('schema_version')) is not int or data['schema_version'] != 1:
            raise ValueError('schema_version must be integer 1')
        return data
    except (OSError, ValueError) as error:
        raise ValueError(f'manifest {path}: {error}') from error


def _fields(value, required, optional=()):
    if not isinstance(value, dict):
        raise ValueError('expected an object')
    missing = set(required) - value.keys()
    unknown = value.keys() - set(required) - set(optional)
    if missing or unknown:
        raise ValueError(f'missing fields {sorted(missing)}; unknown fields {sorted(unknown)}')


def _string(value, field):
    if not isinstance(value, str) or not value.strip() or value != value.strip():
        raise ValueError(f'{field} must be a nonempty trimmed string')


def _integer(value, field):
    if type(value) is not int or value <= 0:
        raise ValueError(f'{field} must be a positive integer')


def require_https(url):
    _string(url, 'url')
    if any(ord(char) <= 32 or ord(char) == 127 for char in url):
        raise ValueError('HTTPS URL contains whitespace/control characters')
    parsed = urllib.parse.urlsplit(url)
    if (parsed.scheme != 'https' or not parsed.hostname or parsed.username is not None
            or parsed.password is not None or parsed.fragment):
        raise ValueError('expected an HTTPS URL without credentials or fragments')
    try:
        parsed.port
    except ValueError as error:
        raise ValueError('invalid HTTPS port') from error


def _list(value, field, allowed=None, pattern=None):
    if not isinstance(value, list) or not value:
        raise ValueError(f'{field} must be a nonempty list')
    for item in value:
        _string(item, field)
        if allowed is not None and item not in allowed:
            raise ValueError(f'unknown {field}: {item}')
        if pattern is not None and not re.fullmatch(pattern, item):
            raise ValueError(f'invalid {field}: {item}')
    if len(set(value)) != len(value):
        raise ValueError(f'duplicate {field}')


def page_tier(pages):
    return PAGE_TIERS[0 if pages < 50 else 1 if pages < 200 else 2 if pages < 500 else 3 if pages < 1000 else 4]


def size_tier(size):
    mib = size / (1024 * 1024)
    return SIZE_TIERS[0 if mib < 10 else 1 if mib < 50 else 2 if mib < 200 else 3]


def _classification(fixture):
    if fixture['document_type'] not in DOCUMENT_TYPES:
        raise ValueError('unknown document_type')
    _list(fixture['languages'], 'languages', pattern=LANGUAGE_PATTERN)
    _list(fixture['features'], 'features', allowed=FEATURES)
    features = set(fixture['features'])
    if fixture['document_type'] == 'image-only-scan' and (
            'text' in features or not {'images', 'scanned-pages'} <= features):
        raise ValueError('image-only-scan class requires scanned images and no text feature')
    if fixture['document_type'] == 'ocr-scan' and not {'text', 'images', 'ocr', 'scanned-pages'} <= features:
        raise ValueError('ocr-scan class requires OCR text and scanned image features')


def load_corpus(path, allow_pending=False):
    data = _read(path)
    _fields(data, ('schema_version', 'fixtures'))
    fixtures = data['fixtures']
    if not isinstance(fixtures, list) or not fixtures:
        raise ValueError('fixtures must be a nonempty list')
    seen = set()
    required = ('id', 'title', 'url', 'source_url', 'license', 'license_url',
                'attribution', 'document_type', 'languages', 'producer_family',
                'features', 'status', 'sha256', 'bytes', 'pages', 'pdf_version',
                'encrypted', 'text_layer', 'page_tier', 'size_tier')
    pin_fields = ('sha256', 'bytes', 'pages', 'pdf_version', 'encrypted', 'text_layer')
    for fixture in fixtures:
        _fields(fixture, required, ('notes', 'pin_notes', 'source_pages', 'source_bytes'))
        for field in ('id', 'title', 'license', 'attribution', 'producer_family'):
            _string(fixture[field], field)
        if not re.fullmatch(ID_PATTERN, fixture['id']) or fixture['id'] in seen:
            raise ValueError('unsafe or duplicate fixture id')
        seen.add(fixture['id'])
        for field in ('url', 'source_url', 'license_url'):
            require_https(fixture[field])
        for field in ('notes', 'pin_notes'):
            if field in fixture:
                _string(fixture[field], field)
        for field in ('source_pages', 'source_bytes'):
            if field in fixture:
                _integer(fixture[field], field)
        _classification(fixture)
        if fixture['page_tier'] not in PAGE_TIERS or fixture['size_tier'] not in SIZE_TIERS:
            raise ValueError('unknown page_tier or size_tier')
        if fixture['status'] == 'pin-pending':
            if not allow_pending:
                raise ValueError(f"{fixture['id']}: pin-pending; complete admission before execution")
            _string(fixture.get('pin_notes'), 'pin_notes')
            for field in ('source_pages', 'source_bytes'):
                _integer(fixture.get(field), field)
            if any(fixture[field] is not None for field in pin_fields):
                raise ValueError('pin-pending admission fields must be null; use source_pages/source_bytes for reported metadata')
            if ('source_pages' in fixture and fixture['page_tier'] != page_tier(fixture['source_pages'])) or (
                    'source_bytes' in fixture and fixture['size_tier'] != size_tier(fixture['source_bytes'])):
                raise ValueError('reported page/size metadata disagree with tier')
            continue
        if fixture['status'] != 'pinned':
            raise ValueError('status must be pinned or pin-pending')
        if not isinstance(fixture['sha256'], str) or not re.fullmatch(r'[0-9a-f]{64}', fixture['sha256']):
            raise ValueError('sha256 must have 64 lowercase hexadecimal digits')
        for field in ('bytes', 'pages'):
            _integer(fixture[field], field)
        if not isinstance(fixture['pdf_version'], str) or not re.fullmatch(r'(?:1\.[0-7]|2\.0)', fixture['pdf_version']):
            raise ValueError('invalid PDF version')
        if type(fixture['encrypted']) is not bool:
            raise ValueError('encrypted must be boolean')
        if fixture['text_layer'] not in ('none', 'native', 'ocr', 'mixed'):
            raise ValueError('unknown text_layer')
        features = set(fixture['features'])
        if ('text' in features) != (fixture['text_layer'] != 'none'):
            raise ValueError('text feature disagrees with text_layer')
        if fixture['text_layer'] in ('ocr', 'mixed') and 'ocr' not in features:
            raise ValueError('OCR text_layer requires ocr feature')
        if fixture['document_type'] == 'image-only-scan' and (
                'text' in features or not {'images', 'scanned-pages'} <= features):
            raise ValueError('image-only-scan class requires scanned images and no text layer')
        if fixture['document_type'] == 'ocr-scan' and (
                not {'text', 'images', 'ocr', 'scanned-pages'} <= features
                or fixture['text_layer'] not in ('ocr', 'mixed')):
            raise ValueError('ocr-scan class requires OCR text layer and scanned images')
        if fixture['page_tier'] != page_tier(fixture['pages']) or fixture['size_tier'] != size_tier(fixture['bytes']):
            raise ValueError('page_tier/size_tier disagree with pins')
    return fixtures


def load_matrix(path):
    data = _read(path)
    _fields(data, ('schema_version', 'local_fixtures', 'presets'))
    if not isinstance(data['local_fixtures'], list) or not data['local_fixtures']:
        raise ValueError('local_fixtures must be nonempty')
    seen = set()
    for fixture in data['local_fixtures']:
        _fields(fixture, ('id', 'path', 'pages', 'bytes', 'sha256', 'document_type', 'languages', 'features'))
        if not isinstance(fixture['id'], str) or not re.fullmatch(ID_PATTERN, fixture['id']) or fixture['id'] in seen:
            raise ValueError('unsafe or duplicate local fixture id')
        seen.add(fixture['id'])
        _string(fixture['path'], 'path')
        parts = PurePosixPath(fixture['path']).parts
        if (parts[:2] != ('testdata', 'files') or '..' in parts
                or '\\' in fixture['path'] or not fixture['path'].endswith('.pdf')):
            raise ValueError('local fixture path must remain under testdata/files and end in .pdf')
        _integer(fixture['pages'], 'pages')
        _integer(fixture['bytes'], 'bytes')
        if not isinstance(fixture['sha256'], str) or not re.fullmatch(r'[0-9a-f]{64}', fixture['sha256']):
            raise ValueError('local sha256 must have 64 lowercase hexadecimal digits')
        _classification(fixture)
    presets = data['presets']
    _fields(presets, ('smoke', 'reference', 'scaling'))
    for name, preset in presets.items():
        _fields(preset, ('source', 'tasks', 'workers', 'repeats', 'selectors'))
        if preset['source'] != ('local' if name == 'smoke' else 'public'):
            raise ValueError('smoke uses local fixtures; reference/scaling use public fixtures')
        _integer(preset['repeats'], 'repeats')
        _list(preset['tasks'], 'tasks', allowed=TASKS)
        workers = preset['workers']
        if not isinstance(workers, list) or not workers:
            raise ValueError('workers must be a nonempty list')
        for worker in workers:
            _integer(worker, 'workers')
        if len(set(workers)) != len(workers):
            raise ValueError('duplicate workers')
        selectors = preset['selectors']
        _fields(selectors, (), ('ids', 'document_types', 'languages', 'features_all',
                               'page_tiers', 'size_tiers'))
        for field, values in selectors.items():
            allowed = {'document_types': DOCUMENT_TYPES, 'features_all': FEATURES,
                       'page_tiers': PAGE_TIERS, 'size_tiers': SIZE_TIERS}.get(field)
            pattern = {'ids': ID_PATTERN, 'languages': LANGUAGE_PATTERN}.get(field)
            _list(values, field, allowed=allowed, pattern=pattern)
    return data


def expand_preset(corpus, matrix, preset, allow_pending=False):
    if preset not in matrix['presets']:
        raise ValueError(f'unknown preset: {preset}')
    definition = matrix['presets'][preset]
    fixtures = corpus if definition['source'] == 'public' else matrix['local_fixtures']
    selectors = definition['selectors']
    identifiers = {fixture['id'] for fixture in fixtures}
    if set(selectors.get('ids', [])) - identifiers:
        raise ValueError('unknown fixture ids in selectors')
    cells = []
    for fixture in sorted(fixtures, key=lambda item: item['id']):
        if any((field in selectors and fixture[attribute] not in selectors[field])
               for field, attribute in (('ids', 'id'), ('document_types', 'document_type'),
                                        ('page_tiers', 'page_tier'), ('size_tiers', 'size_tier'))):
            continue
        if 'languages' in selectors and not set(selectors['languages']).intersection(fixture['languages']):
            continue
        if not set(selectors.get('features_all', [])).issubset(fixture['features']):
            continue
        if definition['source'] == 'public' and fixture['status'] != 'pinned' and not allow_pending:
            raise ValueError(f"{fixture['id']}: pin-pending; executable expansion requires pinned fixtures")
        for task in TASKS:
            if task not in definition['tasks'] or (task in TASK_FEATURES and TASK_FEATURES[task] not in fixture['features']):
                continue
            task_workers = (1,) if task in SEQUENTIAL_TASKS else sorted(definition['workers'])
            for workers in task_workers:
                cells.append(dict(fixture=fixture['id'], source=definition['source'],
                                  task=task, workers=workers, repeats=definition['repeats']))
    if not cells:
        raise ValueError(f'preset {preset} selects no applicable cells')
    selected = {cell['fixture'] for cell in cells}
    if preset == 'reference' and selected != identifiers:
        raise ValueError('reference coverage must include all public fixtures')
    if preset == 'scaling':
        classes = [fixture['document_type'] for fixture in fixtures if fixture['id'] in selected]
        if sorted(classes) != sorted(DOCUMENT_TYPES):
            raise ValueError('scaling must select exactly one representative per document class')
    return cells


def manifest_digest(path):
    """Hash exact manifest bytes; formatting changes intentionally stale a run."""
    return hashlib.sha256(Path(path).read_bytes()).hexdigest()


def verify_local_fixtures(matrix, root, page_counts=None):
    """Verify small checked-in input identities and optional parsed page counts.

    Each page pin is admitted together with the exact file hash. Actual parsed
    page counts, supplied by the sample runner, must additionally agree.
    """
    root = Path(root).resolve()
    for fixture in matrix['local_fixtures']:
        path = (root / fixture['path']).resolve()
        try:
            path.relative_to(root / 'testdata/files')
        except ValueError as error:
            raise ValueError('local fixture path escapes testdata/files') from error
        try:
            with path.open('rb') as stream:
                _public_driver().verify_stream(stream, fixture)
        except OSError as error:
            raise ValueError(f'missing local fixture {path}') from error
        if page_counts is not None:
            pages = page_counts.get(fixture['id'])
            if type(pages) is not int or pages != fixture['pages']:
                raise ValueError(f"{fixture['id']}: parsed page count disagrees with pin")


def _public_driver():
    spec = importlib.util.spec_from_file_location('benchmark_public_corpus', Path(__file__).with_name('public_corpus.py'))
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


def verify_fixture(fixture, directory):
    """Check bytes and SHA-256; the sample runner also checks parsed page count."""
    if fixture['status'] != 'pinned':
        raise ValueError(f"{fixture['id']}: pin-pending")
    return _public_driver().verify(fixture, Path(directory))


def main(argv=None):
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--corpus', '--manifest', type=Path, default=Path('bench/corpus.json'))
    parser.add_argument('--matrix', type=Path, default=Path('bench/matrix.json'))
    parser.add_argument('--directory', type=Path, default=Path('.compat-cache/benchmark-corpus'))
    parser.add_argument('action', choices=('corpus-check', 'matrix-check', 'pull', 'check'))
    args = parser.parse_args(argv)
    try:
        corpus = load_corpus(args.corpus, allow_pending=args.action in ('corpus-check', 'matrix-check'))
        if args.action == 'matrix-check':
            matrix = load_matrix(args.matrix)
            for preset in matrix['presets']:
                print(f'{preset}: {len(expand_preset(corpus, matrix, preset, allow_pending=True))} cells')
            verify_local_fixtures(matrix, Path(__file__).resolve().parents[1])
        elif args.action == 'corpus-check':
            pending = sum(fixture['status'] == 'pin-pending' for fixture in corpus)
            print(f'Validated {len(corpus)} benchmark documents ({pending} pin-pending).')
        else:
            driver = _public_driver()
            if args.action == 'pull':
                for fixture in corpus:
                    driver.pull(fixture, args.directory)
            for fixture in corpus:
                verify_fixture(fixture, args.directory)
            print(f'Verified {len(corpus)} benchmark PDF files (bytes and SHA-256).')
        return 0
    except (OSError, ValueError) as error:
        print(f'benchmark-manifest: {error}', file=sys.stderr)
        return 1


if __name__ == '__main__':
    sys.exit(main())
