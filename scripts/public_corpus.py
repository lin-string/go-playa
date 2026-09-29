#!/usr/bin/env python3
"""Fetch or verify manifest-pinned, openly licensed PDF compatibility fixtures.

Only ``pull`` uses the network. ``compare -- COMMAND ARGS...`` verifies the
entire corpus before invoking COMMAND with one ``--pdf PATH`` per fixture.
``run -- COMMAND ARGS...`` instead appends verified paths as positional arguments.
"""
import argparse
import hashlib
import json
import os
from pathlib import Path
import re
import subprocess
import sys
import tempfile
import urllib.parse
import urllib.request

CHUNK_SIZE = 1024 * 1024


def require_https(url):
    parsed = urllib.parse.urlsplit(url)
    if parsed.scheme != 'https' or not parsed.hostname or parsed.username or parsed.password:
        raise ValueError('expected an HTTPS URL without credentials')


def load_manifest(path):
    try:
        data = json.loads(path.read_text(encoding='utf-8'))
        if not isinstance(data, dict) or type(data.get('schema_version')) is not int or data['schema_version'] != 1:
            raise ValueError('schema_version must be 1')
        fixtures = data.get('fixtures')
        if not isinstance(fixtures, list) or not fixtures:
            raise ValueError('fixtures must be a nonempty list')
        seen = set()
        for fixture in fixtures:
            if not isinstance(fixture, dict):
                raise ValueError('each fixture must be an object')
            for field in ('id', 'title', 'url', 'sha256', 'license', 'license_url', 'source_url'):
                if not isinstance(fixture.get(field), str) or not fixture[field].strip():
                    raise ValueError(f'{field} must be a nonempty string')
            identifier = fixture['id']
            if not re.fullmatch(r'[a-z0-9]+(?:-[a-z0-9]+)*', identifier) or identifier in seen:
                raise ValueError(f'unsafe or duplicate fixture id: {identifier!r}')
            seen.add(identifier)
            if not re.fullmatch(r'[0-9a-f]{64}', fixture['sha256']):
                raise ValueError(f'{identifier}: sha256 must have 64 lowercase hexadecimal digits')
            for field in ('bytes', 'pages'):
                if type(fixture.get(field)) is not int or fixture[field] <= 0:
                    raise ValueError(f'{identifier}: {field} must be a positive integer')
            for field in ('url', 'license_url', 'source_url'):
                require_https(fixture[field])
        return fixtures
    except (OSError, ValueError) as error:
        raise ValueError(f'manifest {path}: {error}') from error


def verify_stream(stream, fixture, output=None):
    digest = hashlib.sha256()
    size = 0
    signature = b''
    while True:
        chunk = stream.read(CHUNK_SIZE)
        if not chunk:
            break
        size += len(chunk)
        if size > fixture['bytes']:
            raise ValueError(f"{fixture['id']}: size mismatch (exceeds {fixture['bytes']} bytes)")
        signature = (signature + chunk[:5])[:5] if len(signature) < 5 else signature
        digest.update(chunk)
        if output is not None:
            output.write(chunk)
    if not signature.startswith(b'%PDF-'):
        raise ValueError(f"{fixture['id']}: invalid PDF signature")
    if size != fixture['bytes']:
        raise ValueError(f"{fixture['id']}: size mismatch: got {size}, expected {fixture['bytes']}")
    if digest.hexdigest() != fixture['sha256']:
        raise ValueError(f"{fixture['id']}: SHA256 mismatch")


def verify(fixture, directory):
    path = directory / (fixture['id'] + '.pdf')
    try:
        with path.open('rb') as stream:
            verify_stream(stream, fixture)
    except FileNotFoundError as error:
        raise ValueError(f"missing public PDF {path}; run 'make public-fixtures-pull'") from error
    return path.resolve()


class HTTPSRedirectHandler(urllib.request.HTTPRedirectHandler):
    def redirect_request(self, req, fp, code, msg, headers, newurl):
        require_https(newurl)
        return super().redirect_request(req, fp, code, msg, headers, newurl)


def open_https(url):
    require_https(url)
    opener = urllib.request.build_opener(HTTPSRedirectHandler())
    return opener.open(urllib.request.Request(url, headers={'User-Agent': 'go-playa-public-corpus/1'}), timeout=60)


def pull(fixture, directory):
    directory.mkdir(parents=True, exist_ok=True)
    destination = directory / (fixture['id'] + '.pdf')
    if os.path.lexists(destination):
        verify(fixture, directory)
        return
    temporary = None
    try:
        with tempfile.NamedTemporaryFile(prefix=fixture['id'] + '.', suffix='.part',
                                         dir=directory, delete=False) as output:
            temporary = Path(output.name)
            with open_https(fixture['url']) as response:
                verify_stream(response, fixture, output)
        # A same-directory hard link installs the fully verified file atomically
        # without overwriting a destination created by another pull process.
        try:
            os.link(temporary, destination)
        except FileExistsError:
            verify(fixture, directory)
    finally:
        if temporary is not None:
            temporary.unlink(missing_ok=True)


def main(argv=None):
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--manifest', type=Path, default=Path('compat/public_corpus.json'))
    parser.add_argument('--directory', type=Path, default=Path('.compat-cache/public-corpus'))
    parser.add_argument('action', choices=('pull', 'check', 'paths', 'compare', 'run'))
    parser.add_argument('command', nargs=argparse.REMAINDER)
    args = parser.parse_args(argv)
    command = args.command[1:] if args.command[:1] == ['--'] else args.command
    if args.action in ('compare', 'run') and not command:
        parser.error(f'{args.action} requires -- COMMAND ARGS...')
    if args.action not in ('compare', 'run') and command:
        parser.error('unexpected command arguments')
    try:
        fixtures = load_manifest(args.manifest)
        if args.action == 'pull':
            for fixture in fixtures:
                pull(fixture, args.directory)
        paths = [verify(fixture, args.directory) for fixture in fixtures]
        if args.action == 'paths':
            for path in paths:
                print(path)
        elif args.action == 'compare':
            return subprocess.run(command + [arg for path in paths for arg in ('--pdf', str(path))]).returncode
        elif args.action == 'run':
            return subprocess.run(command + [str(path) for path in paths]).returncode
        else:
            print(f'Verified {len(paths)} public PDF fixtures.')
        return 0
    except (OSError, ValueError) as error:
        print(f'public-corpus: {error}', file=sys.stderr)
        return 1


if __name__ == '__main__':
    sys.exit(main())
