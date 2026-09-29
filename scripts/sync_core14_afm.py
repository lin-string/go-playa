#!/usr/bin/env python3
"""Fetch the Adobe Core14 AFM source files used for standard font metrics."""

from __future__ import annotations

import argparse
import io
import subprocess
import tempfile
import tarfile
from pathlib import Path
from urllib.request import Request, urlopen


SOURCE_ARCHIVE = "https://codeload.github.com/tecnickcom/tc-font-core14-afms/tar.gz/refs/heads/main"
ROOT = Path(__file__).resolve().parents[1]
FONT_NAMES = (
    "Courier",
    "Courier-Bold",
    "Courier-BoldOblique",
    "Courier-Oblique",
    "Helvetica",
    "Helvetica-Bold",
    "Helvetica-BoldOblique",
    "Helvetica-Oblique",
    "Symbol",
    "Times-Bold",
    "Times-BoldItalic",
    "Times-Italic",
    "Times-Roman",
    "ZapfDingbats",
)


def normalize(data: bytes) -> bytes:
    """Keep source semantics while making checked-in AFM files Git-clean."""
    lines = [line.rstrip() for line in data.replace(b"\r\n", b"\n").splitlines()]
    notice = b"Comment Modified by go-playa: normalized line endings and trailing whitespace."
    if notice not in lines:
        lines.insert(1, notice)
    return b"\n".join(lines) + b"\n"


def fetch(url: str) -> bytes:
    request = Request(url, headers={"User-Agent": "go-playa-core14-afm-sync"})
    try:
        with urlopen(request, timeout=60) as response:
            return response.read()
    except OSError:
        # A named output lets curl discard an incomplete body before retrying.
        with tempfile.TemporaryDirectory(prefix="go-playa-core14-") as directory:
            output = Path(directory) / "download"
            subprocess.run(
                [
                    "curl", "--fail", "--silent", "--show-error", "--location",
                    "--retry", "5", "--retry-delay", "2", "--retry-all-errors",
                    "--retry-max-time", "90", "--connect-timeout", "15", "--max-time", "30",
                    "--output", str(output), url,
                ],
                check=True,
                timeout=120,
            )
            return output.read_bytes()


def archive_files() -> dict[str, bytes]:
    payload = fetch(SOURCE_ARCHIVE)
    result = {}
    with tarfile.open(fileobj=io.BytesIO(payload), mode="r:gz") as archive:
        for name in FONT_NAMES:
            member = next(
                (item for item in archive.getmembers() if item.name.endswith(f"/{name}.afm")),
                None,
            )
            if member is None or not member.isfile():
                raise ValueError(f"upstream archive is missing {name}.afm")
            extracted = archive.extractfile(member)
            if extracted is None:
                raise ValueError(f"unable to read upstream {name}.afm")
            result[f"{name}.afm"] = extracted.read()
    return result


def files(source_dir: Path | None) -> dict[str, bytes]:
    if source_dir:
        return {
            f"{name}.afm": normalize((source_dir / f"{name}.afm").read_bytes())
            for name in FONT_NAMES
        }
    return {name: normalize(data) for name, data in archive_files().items()}


def check(values: dict[str, bytes], output_dir: Path) -> None:
    mismatches = []
    for name, data in values.items():
        path = output_dir / name
        if not path.is_file():
            mismatches.append(f"missing {path}")
        elif path.read_bytes() != data:
            mismatches.append(f"out-of-date {path}")
    if mismatches:
        raise SystemExit("Core14 AFM source drift:\n" + "\n".join(mismatches))


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--source-dir", type=Path, help="local Core14 AFM checkout")
    parser.add_argument("--output-dir", type=Path, default=ROOT / "scripts/data/core14")
    parser.add_argument("--check", action="store_true", help="fail if checked-in files differ from upstream")
    args = parser.parse_args()
    values = files(args.source_dir)
    if args.check:
        check(values, args.output_dir)
        print(f"checked {len(values)} Adobe Core14 AFM files")
    else:
        args.output_dir.mkdir(parents=True, exist_ok=True)
        for name, data in values.items():
            (args.output_dir / name).write_bytes(data)
        print(f"synced {len(values)} Adobe Core14 AFM files to {args.output_dir}")


if __name__ == "__main__":
    main()
