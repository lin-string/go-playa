#!/usr/bin/env python3
"""Generate CFF resource tables from current public upstream sources.

The generated tables are compatibility data, not handwritten implementation
logic. CFF strings, charsets, and StandardEncoding come from fontTools.
ExpertEncoding comes from the pinned Apache PDFBox source, independently
checked against Adobe Technical Note #5176 Appendix B. fontTools sources are
fetched unless a local source directory is supplied.
"""

from __future__ import annotations

import argparse
import ast
import re
import subprocess
import tempfile
from pathlib import Path
from urllib.request import Request, urlopen


FONTTOOLS_RAW = "https://raw.githubusercontent.com/fonttools/fonttools/main/Lib/fontTools"
ROOT = Path(__file__).resolve().parents[1]
EXPERT_SOURCE = ROOT / "scripts/data/pdfbox/CFFExpertEncoding.java"


def read_source(path: str, source_dir: Path | None, raw_base: str) -> str:
    if source_dir is not None:
        local = source_dir / path
        return local.read_text(encoding="utf-8")
    request = Request(f"{raw_base}/{path}", headers={"User-Agent": "go-playa-resource-generator"})
    try:
        with urlopen(request, timeout=30) as response:
            return response.read().decode("utf-8")
    except OSError:
        # curl can rewind a named output file before a retry; a stdout pipe
        # would retain any incomplete response and concatenate the next one.
        with tempfile.TemporaryDirectory(prefix="go-playa-cff-") as directory:
            output = Path(directory) / "source.py"
            subprocess.run(
                [
                    "curl", "--fail", "--silent", "--show-error", "--location",
                    "--retry", "5", "--retry-delay", "2", "--retry-all-errors",
                    "--retry-max-time", "90", "--connect-timeout", "15", "--max-time", "30",
                    "--output", str(output), request.full_url,
                ],
                check=True,
            )
            return output.read_text(encoding="utf-8")


def assignment(source: str, name: str):
    tree = ast.parse(source)
    for node in ast.walk(tree):
        if isinstance(node, ast.Assign) and any(isinstance(target, ast.Name) and target.id == name for target in node.targets):
            return ast.literal_eval(node.value)
    raise ValueError(f"missing assignment {name}")


def go_string(value: str) -> str:
    return '"' + value.replace("\\", "\\\\").replace('"', '\\"') + '"'


def go_strings(name: str, values: list[str]) -> str:
    lines = [f"var {name} = [...]string{{"]
    lines.extend(f"\t{go_string(value)}," for value in values)
    lines.append("}")
    return "\n".join(lines)


def go_ints(name: str, values: list[int]) -> str:
    lines = [f"var {name} = [...]int{{"]
    lines.extend(f"\t{value}," for value in values)
    lines.append("}")
    return "\n".join(lines)


def read_expert_encoding(path: Path) -> list[int]:
    source = path.read_text(encoding="utf-8")
    match = re.search(r"cffExpertEncodingTable\s*=\s*\{(.*?)\};", source, flags=re.DOTALL)
    if match is None:
        raise ValueError(f"{path}: missing CFF ExpertEncoding table")
    entries = [(int(code), int(sid)) for code, sid in re.findall(r"\{(\d+),\s*(\d+)\}", match.group(1))]
    if len(entries) != 256 or [code for code, _ in entries] != list(range(256)):
        raise ValueError(f"{path}: expected all 256 character codes in order")
    values = [sid for _, sid in entries]
    if any(sid < 0 or sid >= 391 for sid in values):
        raise ValueError(f"{path}: invalid CFF standard SID")
    return values


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--fonttools-source-dir", type=Path, help="local fontTools checkout")
    parser.add_argument("--expert-source", type=Path, default=EXPERT_SOURCE, help="pinned Apache PDFBox CFFExpertEncoding.java")
    parser.add_argument("--output-dir", type=Path, default=ROOT / "fontdata")
    parser.add_argument("--package", default="fontdata")
    args = parser.parse_args()

    cff = read_source("cffLib/__init__.py", args.fonttools_source_dir, FONTTOOLS_RAW)
    standard_encoding = read_source("encodings/StandardEncoding.py", args.fonttools_source_dir, FONTTOOLS_RAW)

    standard_strings = assignment(cff, "cffStandardStrings")
    iso_adobe = assignment(cff, "cffISOAdobeStrings")
    expert = assignment(cff, "cffIExpertStrings")
    expert_subset = assignment(cff, "cffExpertSubsetStrings")
    sid_by_name = {name: sid for sid, name in enumerate(standard_strings)}
    standard_names = assignment(standard_encoding, "StandardEncoding")
    standard = [sid_by_name[name] if name != ".notdef" else 0 for name in standard_names]
    expert_codes = read_expert_encoding(args.expert_source)

    if len(standard_strings) != 391 or len(iso_adobe) != 229 or len(expert) != 166 or len(expert_subset) != 87:
        raise ValueError("unexpected CFF charset table length")
    if len(standard) != 256 or len(expert_codes) != 256:
        raise ValueError("unexpected CFF encoding table length")

    header = f"// Code generated by scripts/generate_cff_resources.py; DO NOT EDIT.\n\npackage {args.package}\n\n"
    args.output_dir.mkdir(parents=True, exist_ok=True)
    (args.output_dir / "cff_standard_strings.go").write_text(
        header
        + "// Adobe CFF standard SID strings; custom strings start at SID 391.\n"
        + go_strings("CFFStandardStrings", standard_strings)
        + "\n\nconst CFFStandardStringCount = 391\n\n"
        + "var _ [CFFStandardStringCount - len(CFFStandardStrings)]struct{}\n"
        + "var _ [len(CFFStandardStrings) - CFFStandardStringCount]struct{}\n",
        encoding="utf-8",
    )
    (args.output_dir / "cff_predefined_charsets.go").write_text(
        header
        + "// Adobe CFF predefined charset sequences.\n"
        + go_strings("CFFISOAdobeCharset", iso_adobe)
        + "\n\n"
        + go_strings("CFFExpertCharset", expert)
        + "\n\n"
        + go_strings("CFFExpertSubsetCharset", expert_subset)
        + "\n",
        encoding="utf-8",
    )
    (args.output_dir / "cff_encoding_generated.go").write_text(
        header
        + "// ExpertEncoding: Modified/translated from Apache PDFBox material; Apache-2.0.\n"
        + "// See NOTICE, THIRD_PARTY_LICENSES, and scripts/data/pdfbox/LICENSE.txt.\n"
        + "// Adobe CFF predefined encoding SID sequences.\n"
        + go_ints("CFFStandardEncoding", standard)
        + "\n\n"
        + go_ints("CFFExpertEncoding", expert_codes)
        + "\n",
        encoding="utf-8",
    )


if __name__ == "__main__":
    main()
