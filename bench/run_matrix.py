#!/usr/bin/env python3
"""Plan or execute pinned benchmark cells in fresh, alternating processes."""
import argparse
from datetime import datetime, timezone
import hashlib
import importlib.util
import json
import os
from pathlib import Path
import platform
import re
import subprocess
import stat
import sys
import uuid

ROOT = Path(__file__).resolve().parents[1]
spec = importlib.util.spec_from_file_location('benchmark_summary', ROOT / 'bench/summarize.py')
summary = importlib.util.module_from_spec(spec)
spec.loader.exec_module(summary)
manifest = summary.manifest


def select_cells(corpus, matrix, preset, fixtures=(), tasks=(), workers=(), allow_pending=False):
    cells = manifest.expand_preset(corpus, matrix, preset, allow_pending=allow_pending)
    for values, field in ((fixtures, 'fixture'), (tasks, 'task'), (workers, 'workers')):
        if set(values) - {cell[field] for cell in cells}:
            raise ValueError(f'unknown or inapplicable {field} filters')
    selected = [cell for cell in cells if (not fixtures or cell['fixture'] in fixtures)
                and (not tasks or cell['task'] in tasks) and (not workers or cell['workers'] in workers)]
    if not selected:
        raise ValueError('filters select no applicable cells')
    return selected


def fixture_path(fixture, source, root, corpus_directory):
    path = (root / fixture['path']) if source == 'local' else corpus_directory / (fixture['id'] + '.pdf')
    resolved = path.resolve()
    boundary = (root / 'testdata/files').resolve() if source == 'local' else corpus_directory.resolve()
    try:
        resolved.relative_to(boundary)
    except ValueError as error:
        raise ValueError('fixture path escapes its input directory') from error
    return resolved


def oracle_command(root):
    # Frozen and offline prevents the measurement launcher from resolving or downloading dependencies.
    return ['uv', 'run', '--offline', '--frozen', '--project', str(root / 'compat'), 'python']


def plan_samples(cells, fixtures, root, corpus_directory, binary):
    samples = []
    for cell in cells:
        path = fixture_path(fixtures[cell['fixture']], cell['source'], root, corpus_directory)
        for repeat in range(1, cell['repeats'] + 1):
            for order, library in enumerate(('go-playa', 'playa') if repeat % 2 else ('playa', 'go-playa'), 1):
                command = ([str(binary), '-task', cell['task'], '-workers', str(cell['workers']), '-jsonl', str(path)]
                           if library == 'go-playa' else oracle_command(root) +
                           [str(root / 'bench/run_playa.py'), '--task', cell['task'], '--workers', str(cell['workers']), '--jsonl', str(path)])
                samples.append(dict(fixture=cell['fixture'], task=cell['task'], requested_workers=cell['workers'],
                                    repeat=repeat, order=order, library=library, command=command))
    return samples


def parse_sample_stdout(output):
    lines = output.splitlines()
    if len(lines) != 1 or not lines[0].strip():
        raise ValueError('sample stdout must contain exactly one JSON object line')
    value = summary.strict_json(lines[0])
    if not isinstance(value, dict):
        raise ValueError('sample stdout must be a JSON object')
    return value


def execute(command, root, env=None):
    result = subprocess.run(command, cwd=root, env=env, capture_output=True, text=True, check=False)
    if result.stderr:
        print(result.stderr, end='' if result.stderr.endswith('\n') else '\n', file=sys.stderr)
    if result.returncode:
        raise ValueError(f'process exited {result.returncode}: {command[0]}')
    return result.stdout


def parse_pages(path, root):
    # This untimed admission probe uses the same pinned parser, not lexical /Page counts.
    code = 'import json,sys,playa\nwith playa.open(sys.argv[1],max_workers=1) as doc:\n print(json.dumps({"pages":len(doc.pages)}))'
    value = parse_sample_stdout(execute(oracle_command(root) + ['-c', code, str(path)], root))
    return value.get('pages')


def preflight(cells, fixtures, root, corpus_directory, page_parser=None):
    page_parser = page_parser or (lambda path: parse_pages(path, root))
    selected = {cell['fixture']: cell['source'] for cell in cells}
    for fixture_id in sorted(selected):
        fixture = fixtures[fixture_id]
        path = fixture_path(fixture, selected[fixture_id], root, corpus_directory)
        with path.open('rb') as stream:
            manifest._public_driver().verify_stream(stream, fixture)
        pages = page_parser(path)
        if type(pages) is not int or pages != fixture['pages']:
            raise ValueError(f'{fixture_id}: parsed page count disagrees with pin')


def capture_environment(root):
    ram = None
    cpu = platform.processor() or platform.machine()
    if sys.platform == 'darwin':
        cpu = execute(['sysctl', '-n', 'machdep.cpu.brand_string'], root).strip()
        ram = int(execute(['sysctl', '-n', 'hw.memsize'], root).strip())
    elif sys.platform.startswith('linux'):
        if Path('/proc/cpuinfo').exists():
            for line in Path('/proc/cpuinfo').read_text().splitlines():
                if line.startswith('model name'):
                    cpu = line.split(':', 1)[1].strip()
                    break
        if Path('/proc/meminfo').exists():
            ram = int(Path('/proc/meminfo').read_text().split('MemTotal:', 1)[1].split()[0]) * 1024
    if ram is None:
        try:
            ram = os.sysconf('SC_PHYS_PAGES') * os.sysconf('SC_PAGE_SIZE')
        except (ValueError, OSError, AttributeError):
            pass
    contract = (root / 'compat/upstream.toml').read_text(encoding='utf-8')
    package_match = re.search(r'^package\s*=\s*"([^"]+)"\s*$', contract, re.MULTILINE)
    if package_match is None:
        raise ValueError('canonical oracle contract is missing its package')
    code = 'import json,sys,importlib.metadata; print(json.dumps({"python":sys.version,"playa":importlib.metadata.version(sys.argv[1])}))'
    oracle = parse_sample_stdout(execute(oracle_command(root) + ['-c', code, package_match.group(1)], root))
    return dict(os=platform.platform(), architecture=platform.machine(), cpu=cpu,
                logical_cpus=os.cpu_count(), ram_bytes=ram,
                go=execute(['go', 'version'], root).strip(), python=oracle['python'], playa=oracle['playa'],
                oracle_sha256=hashlib.sha256((root / 'compat/upstream.toml').read_bytes()).hexdigest(),
                oracle_contract=contract,
                rss_scope='Go and single-process Playa lifetime peaks; parallel Playa formal RSS unavailable, parent lifetime peak and worker callback lower bounds are diagnostic only',
                timing_scope='immediately before document open through completed document close')


def capture_implementation(root, binary, supplied=False, exclude_paths=()):
    """Identify repository source independently of HEAD, plus the exact executable.

    Supplied binaries are identified by their own hash; the source snapshot is
    checkout context and does not assert which source built an external binary.
    """
    root, binary = root.resolve(), binary.resolve()
    def git(*args):
        try:
            return subprocess.run(['git', *args], cwd=root, capture_output=True, check=False)
        except OSError:
            return None
    head = git('rev-parse', '--verify', 'HEAD')
    commit = head.stdout.decode().strip() if head is not None and head.returncode == 0 else None
    status = git('status', '--porcelain', '--untracked-files=normal')
    dirty = bool(status.stdout) if status is not None and status.returncode == 0 else None
    listing = git('ls-files', '--cached', '--others', '--exclude-standard', '-z')
    ignored = {'.git', '.compat-cache', '.venv', '__pycache__', '.superpowers', '.idea', '.vscode'}
    if listing is not None and listing.returncode == 0:
        paths = sorted({Path(os.fsdecode(value)) for value in listing.stdout.split(b'\0') if value}, key=lambda p: p.as_posix())
    else:
        paths = []
        for directory, dirs, files in os.walk(root):
            dirs[:] = sorted(name for name in dirs if name not in ignored)
            paths.extend((Path(directory) / name).relative_to(root) for name in files)
        paths.sort(key=lambda p: p.as_posix())
    excluded = [Path(path).resolve() for path in exclude_paths]
    digest, count = hashlib.sha256(b'go-playa-source-v1\0'), 0
    for relative in paths:
        if any(part in ignored for part in relative.parts) or relative.suffix in ('.pyc', '.pprof', '.mprof', '.out', '.tmp'):
            continue
        path = root / relative
        if path.resolve() == binary or any(path.resolve() == excluded_path or excluded_path in path.resolve().parents for excluded_path in excluded):
            continue
        name = relative.as_posix().encode('utf-8', errors='surrogateescape')
        digest.update(len(name).to_bytes(8, 'big') + name)
        if path.is_symlink():
            data = os.fsencode(os.readlink(path))
            digest.update(b'L' + len(data).to_bytes(8, 'big') + hashlib.sha256(data).digest())
        elif path.is_file():
            mode = stat.S_IMODE(path.stat().st_mode)
            digest.update(b'F' + bytes([bool(mode & 0o111)]) + path.stat().st_size.to_bytes(8, 'big'))
            digest.update(bytes.fromhex(summary.file_sha256(path)))
        else:
            digest.update(b'M')  # A tracked deletion is part of a dirty source identity.
        count += 1
    if not count:
        raise ValueError('repository source identity has no files')
    return dict(git_commit=commit, git_dirty=dirty, source_sha256=digest.hexdigest(), source_file_count=count,
                source_identity_algorithm='sha256-path-mode-content-v1',
                binary=dict(path=str(binary), sha256=summary.file_sha256(binary), bytes=binary.stat().st_size,
                            origin='supplied-binary' if supplied else 'built-from-current-source',
                            build_command=None if supplied else ['go', 'build', '-o', str(binary), './bench/go']))


def main(argv=None):
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--preset', choices=('smoke', 'reference', 'scaling'), default='smoke')
    parser.add_argument('--fixture', action='append', default=[])
    parser.add_argument('--task', action='append', default=[])
    parser.add_argument('--worker', '--workers', type=int, action='append', default=[])
    parser.add_argument('--corpus', type=Path, default=ROOT / 'bench/corpus.json')
    parser.add_argument('--matrix', type=Path, default=ROOT / 'bench/matrix.json')
    parser.add_argument('--corpus-directory', type=Path, default=ROOT / '.compat-cache/benchmark-corpus')
    parser.add_argument('--output', '--output-dir', type=Path)
    parser.add_argument('--go-binary', type=Path)
    parser.add_argument('--dry-run', action='store_true')
    args = parser.parse_args(argv)
    try:
        corpus = manifest.load_corpus(args.corpus, allow_pending=args.dry_run)
        matrix = manifest.load_matrix(args.matrix)
        cells = select_cells(corpus, matrix, args.preset, args.fixture, args.task, args.worker, args.dry_run)
        source = corpus if args.preset != 'smoke' else matrix['local_fixtures']
        fixtures = {f['id']: f for f in source if f['id'] in {cell['fixture'] for cell in cells}}
        run_id = datetime.now(timezone.utc).strftime('%Y%m%dT%H%M%SZ') + '-' + uuid.uuid4().hex[:8]
        output = (args.output or ROOT / '.compat-cache/benchmarks' / run_id).resolve()
        binary = (args.go_binary or output / 'go-playa-bench').resolve()
        samples = plan_samples(cells, fixtures, ROOT, args.corpus_directory.resolve(), binary)
        digests = dict(corpus_digest=manifest.manifest_digest(args.corpus), matrix_digest=manifest.manifest_digest(args.matrix))
        if args.dry_run:
            print(json.dumps(dict(schema_version=1, preset=args.preset, cell_count=len(cells), sample_count=len(samples),
                                  cells=cells, samples=samples, **digests), sort_keys=True, allow_nan=False))
            return 0
        if output.exists() and any(output.iterdir()):
            raise ValueError('output directory must be new or empty; existing raw samples are immutable')
        preflight(cells, fixtures, ROOT, args.corpus_directory.resolve())
        environment = dict(capture_environment(ROOT), schema_version=1, run_id=run_id, preset=args.preset,
                           started_at=datetime.now(timezone.utc).isoformat(), cells=cells, fixtures=fixtures,
                           filters=dict(fixtures=args.fixture, tasks=args.task, workers=args.worker), **digests)
        output.mkdir(parents=True, exist_ok=True)
        if not args.go_binary:
            execute(['go', 'build', '-o', str(binary), './bench/go'], ROOT)
        implementation = capture_implementation(ROOT, binary, supplied=bool(args.go_binary),
                                                exclude_paths=(output, args.corpus_directory))
        environment['implementation'] = implementation
        identity = dict(implementation_digest=summary.implementation_digest(implementation),
                        implementation_source_sha256=implementation['source_sha256'],
                        benchmark_binary_sha256=implementation['binary']['sha256'])
        # Exclusive file creation refuses to overwrite a run even under a competing launcher.
        with (output / 'environment.json').open('x', encoding='utf-8') as stream:
            stream.write(json.dumps(environment, sort_keys=True, indent=2, allow_nan=False) + '\n')
        records = []
        with (output / 'raw.jsonl').open('x', encoding='utf-8') as stream:
            for index, sample in enumerate(samples, 1):
                print(f"[{index}/{len(samples)}] {sample['fixture']} {sample['task']} workers={sample['requested_workers']} repeat={sample['repeat']} {sample['library']}", file=sys.stderr)
                env = dict(os.environ, GOMAXPROCS=str(sample['requested_workers']))
                record = parse_sample_stdout(execute(sample['command'], ROOT, env))
                for field in ('library', 'task', 'requested_workers'):
                    if type(record.get(field)) is not type(sample[field]) or record.get(field) != sample[field]:
                        raise ValueError(f'sample runner {field} disagrees with command')
                if record.get('pdf') != sample['command'][-1]:
                    raise ValueError('sample runner input path disagrees with command')
                summary.validate_record(record, fixtures[sample['fixture']])
                reserved = {'schema_version', 'run_id', 'sample_index', 'fixture', 'fixture_sha256', 'repeat', 'order', *digests, *identity}
                if reserved.intersection(record):
                    raise ValueError('sample runner attempted to supply orchestrator metadata')
                record.update(schema_version=1, run_id=run_id, sample_index=index, fixture=sample['fixture'],
                              fixture_sha256=fixtures[sample['fixture']]['sha256'], repeat=sample['repeat'], order=sample['order'], **digests, **identity)
                stream.write(json.dumps(record, sort_keys=True, separators=(',', ':'), allow_nan=False) + '\n')
                stream.flush()
                records.append(record)
        if summary.file_sha256(binary) != implementation['binary']['sha256']:
            raise ValueError('benchmark binary changed during run')
        result = summary.summarize(records, environment, **digests)
        summary.write_summary(output, result)
        print(json.dumps(dict(directory=str(output), cells=len(cells), samples=len(samples))))
        if result['correctness_mismatch_cell_count']:
            print(f"benchmark-matrix: wrote partial evidence with {result['correctness_mismatch_cell_count']} correctness-mismatch cells", file=sys.stderr)
            return 2
        return 0
    except (OSError, ValueError, KeyError, TypeError) as error:
        print(f'benchmark-matrix: {error}', file=sys.stderr)
        return 1


if __name__ == '__main__':
    sys.exit(main())
