#!/usr/bin/env python3
"""Synchronize Adobe glyph-list source files from the current upstream."""

from __future__ import annotations

import argparse
import io
import subprocess
import tarfile
import tempfile
from pathlib import Path
from urllib.request import Request, urlopen


UPSTREAM_TARBALL = (
    "https://codeload.github.com/adobe-type-tools/agl-aglfn/"
    "tar.gz/refs/heads/master"
)
SOURCE_FILES = ("glyphlist.txt", "zapfdingbats.txt")
ROOT = Path(__file__).resolve().parents[1]


def local_files(source_dir: Path) -> dict[str, bytes]:
    return {
        name: (source_dir / name).read_bytes()
        for name in SOURCE_FILES
    }


def download_files() -> dict[str, bytes]:
    request = Request(
        UPSTREAM_TARBALL,
        headers={"User-Agent": "go-playa-glyphlist-sync"},
    )
    try:
        with urlopen(request, timeout=120) as response:
            payload = response.read()
    except OSError:
        # A named output lets curl discard an incomplete body before retrying.
        with tempfile.TemporaryDirectory(prefix="go-playa-glyphlist-") as directory:
            output = Path(directory) / "source.tar.gz"
            subprocess.run(
                [
                    "curl", "--fail", "--silent", "--show-error", "--location",
                    "--retry", "5", "--retry-delay", "2", "--retry-all-errors",
                    "--retry-max-time", "90", "--connect-timeout", "15", "--max-time", "60",
                    "--output", str(output), UPSTREAM_TARBALL,
                ],
                check=True,
                timeout=150,
            )
            payload = output.read_bytes()
    result = {}
    with tarfile.open(fileobj=io.BytesIO(payload), mode="r:gz") as archive:
        for name in SOURCE_FILES:
            member = next(
                (item for item in archive.getmembers() if item.name.endswith(f"/{name}")),
                None,
            )
            if member is None or not member.isfile():
                raise ValueError(f"upstream archive is missing {name}")
            extracted = archive.extractfile(member)
            if extracted is None:
                raise ValueError(f"unable to read upstream {name}")
            result[name] = extracted.read()
    return result


def sync(files: dict[str, bytes], output_dir: Path) -> None:
    output_dir.mkdir(parents=True, exist_ok=True)
    for name, data in files.items():
        (output_dir / name).write_bytes(data)


def check(files: dict[str, bytes], output_dir: Path) -> None:
    mismatches = []
    for name, data in files.items():
        path = output_dir / name
        if not path.is_file():
            mismatches.append(f"missing {path}")
        elif path.read_bytes() != data:
            mismatches.append(f"out-of-date {path}")
    if mismatches:
        raise SystemExit("glyph-list source drift:\n" + "\n".join(mismatches))


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument(
        "--source-dir",
        type=Path,
        help="local agl-aglfn checkout; otherwise fetch Adobe master",
    )
    parser.add_argument("--output-dir", type=Path, default=ROOT / "scripts/data")
    parser.add_argument("--check", action="store_true", help="fail if checked-in files differ from upstream")
    args = parser.parse_args()
    files = local_files(args.source_dir) if args.source_dir else download_files()
    if args.check:
        check(files, args.output_dir)
        print(f"checked {len(files)} Adobe glyph-list sources")
    else:
        sync(files, args.output_dir)
        print(f"synced {len(files)} Adobe glyph-list sources to {args.output_dir}")


if __name__ == "__main__":
    main()
