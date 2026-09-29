#!/usr/bin/env python3
"""Synchronize embedded CMaps from Adobe's current public repository."""

from __future__ import annotations

import argparse
import io
import shutil
import subprocess
import tarfile
import tempfile
from pathlib import Path
from urllib.request import Request, urlopen


UPSTREAM_TARBALL = (
    "https://codeload.github.com/adobe-type-tools/cmap-resources/"
    "tar.gz/refs/heads/master"
)
ROOT = Path(__file__).resolve().parents[1]


def source_files(source_dir: Path) -> dict[str, bytes]:
    result = {}
    for path in source_dir.glob("**/CMap/*"):
        if path.is_file():
            result[path.name] = path.read_bytes()
    return result


def download_files() -> dict[str, bytes]:
    request = Request(
        UPSTREAM_TARBALL,
        headers={"User-Agent": "go-playa-cmap-resource-sync"},
    )
    try:
        with urlopen(request, timeout=120) as response:
            payload = response.read()
    except OSError:
        # A named output lets curl discard an incomplete body before retrying.
        with tempfile.TemporaryDirectory(prefix="go-playa-cmap-download-") as directory:
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
        for member in archive.getmembers():
            if "/CMap/" not in member.name or not member.isfile():
                continue
            name = Path(member.name).name
            extracted = archive.extractfile(member)
            if extracted is None:
                raise ValueError(f"unable to read upstream CMap {name}")
            result[name] = extracted.read()
    return result


def sync(files: dict[str, bytes], output_dir: Path) -> None:
    if not files:
        raise ValueError("upstream CMap source contains no CMap files")
    output_dir.mkdir(parents=True, exist_ok=True)
    with tempfile.TemporaryDirectory(prefix="go-playa-cmap-") as staging:
        staging_dir = Path(staging)
        for name, data in files.items():
            (staging_dir / f"{name}.cmap").write_bytes(data)
        expected = {f"{name}.cmap" for name in files}
        for existing in output_dir.glob("*.cmap"):
            if existing.name not in expected:
                existing.unlink()
        for source in staging_dir.glob("*.cmap"):
            shutil.copyfile(source, output_dir / source.name)


def check(files: dict[str, bytes], output_dir: Path) -> None:
    mismatches = []
    expected = {f"{name}.cmap" for name in files}
    for name, data in files.items():
        path = output_dir / f"{name}.cmap"
        if not path.is_file():
            mismatches.append(f"missing {path}")
        elif path.read_bytes() != data:
            mismatches.append(f"out-of-date {path}")
    for path in output_dir.glob("*.cmap"):
        if path.name not in expected:
            mismatches.append(f"obsolete {path}")
    if mismatches:
        raise SystemExit("Adobe CMap source drift:\n" + "\n".join(mismatches))


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument(
        "--source-dir",
        type=Path,
        help="local cmap-resources checkout; otherwise fetch Adobe master",
    )
    parser.add_argument("--output-dir", type=Path, default=ROOT / "fontdata/cmapdata")
    parser.add_argument("--check", action="store_true", help="fail if checked-in files differ from upstream")
    args = parser.parse_args()
    files = source_files(args.source_dir) if args.source_dir else download_files()
    if args.check:
        check(files, args.output_dir)
        print(f"checked {len(files)} Adobe CMap resources")
    else:
        sync(files, args.output_dir)
        print(f"synced {len(files)} Adobe CMap resources to {args.output_dir}")


if __name__ == "__main__":
    main()
