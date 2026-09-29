#!/usr/bin/env python3
"""Synchronize the two distinct expert encodings from pinned Apache PDFBox."""

from __future__ import annotations

import argparse
import hashlib
import subprocess
import tempfile
from pathlib import Path
from urllib.request import Request, urlopen


ROOT = Path(__file__).resolve().parents[1]
PDFBOX_COMMIT = "5ae91127c3316db8663da843dce746022f22df91"
PDFBOX_RAW = f"https://raw.githubusercontent.com/apache/pdfbox/{PDFBOX_COMMIT}"
SOURCES = {
    "LICENSE.txt": (
        "LICENSE.txt",
        "1301d8415a4868d82aeeec594849cf7679f1ead4636a9603dc46875f5713157e",
    ),
    "NOTICE.txt": (
        "NOTICE.txt",
        "40741b4ab76d77ba4fbc5e8759277169fb0ce281859d273075de6fd3a3588458",
    ),
    "MacExpertEncoding.java": (
        "pdfbox/src/main/java/org/apache/pdfbox/pdmodel/font/encoding/MacExpertEncoding.java",
        "c837b6b1bd6add722ec4b9cc5d3998e0dfb6c1feb2c92465cac5947f8ba6fc6f",
    ),
    "CFFExpertEncoding.java": (
        "fontbox/src/main/java/org/apache/fontbox/cff/CFFExpertEncoding.java",
        "e6d9d3816e2e5098b99b69dcbb190e7d12cbedd571fd9720c454948c542e2da9",
    ),
}


def fetch(url: str) -> bytes:
    request = Request(url, headers={"User-Agent": "go-playa-resource-sync"})
    try:
        with urlopen(request, timeout=30) as response:
            return response.read()
    except OSError:
        # A named output lets curl discard an incomplete body before retrying.
        with tempfile.TemporaryDirectory(prefix="go-playa-pdfbox-") as directory:
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


def verify_hash(data: bytes, expected: str, name: str) -> None:
    actual = hashlib.sha256(data).hexdigest()
    if actual != expected:
        raise SystemExit(f"{name}: SHA-256 mismatch: got {actual}, want {expected}")


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--source-dir", type=Path, help="local Apache PDFBox checkout")
    parser.add_argument("--output-dir", type=Path, default=ROOT / "scripts/data")
    parser.add_argument("--check", action="store_true", help="check checked-in files and pinned hashes")
    args = parser.parse_args()
    output = args.output_dir / "pdfbox"
    for name, (path, digest) in SOURCES.items():
        target = output / name
        if args.check:
            if not target.is_file():
                raise SystemExit(f"missing Apache PDFBox source: {target}")
            data = target.read_bytes()
        elif args.source_dir:
            data = (args.source_dir / path).read_bytes()
        else:
            data = fetch(f"{PDFBOX_RAW}/{path}")
        verify_hash(data, digest, name)
        if not args.check:
            output.mkdir(parents=True, exist_ok=True)
            target.write_bytes(data)
    print(f"{'checked' if args.check else 'synchronized'} Apache PDFBox expert encoding sources at {PDFBOX_COMMIT}")


if __name__ == "__main__":
    main()
