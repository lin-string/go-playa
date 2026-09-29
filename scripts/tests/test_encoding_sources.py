"""Check expert encoding sources and Python resource generator entry points."""

from __future__ import annotations

import importlib.util
import http.server
import io
import shlex
import subprocess
import threading
import tarfile
import unittest
from pathlib import Path
from unittest.mock import patch


ROOT = Path(__file__).resolve().parents[2]


def load_script(name: str):
    path = ROOT / "scripts" / name
    spec = importlib.util.spec_from_file_location(path.stem, path)
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


class ExpertEncodingSourceTest(unittest.TestCase):
    def test_cff_fetch_discards_partial_response_before_curl_retry(self) -> None:
        cff = load_script("generate_cff_resources.py")
        source = b"cffStandardStrings = ['.notdef']\n"
        self.check_retry(cff, source, lambda url: cff.read_source("cff.py", None, url), source.decode())

    def test_resource_fetches_discard_partial_response_before_curl_retry(self) -> None:
        for script in ("sync_core14_afm.py", "sync_macexpert_encoding.py"):
            with self.subTest(script=script):
                module = load_script(script)
                source = b"complete upstream source body\n"
                self.check_retry(module, source, module.fetch, source)

    def test_archive_fetches_discard_partial_response_before_curl_retry(self) -> None:
        for script, names in (
            ("sync_glyphlist_sources.py", ("glyphlist.txt", "zapfdingbats.txt")),
            ("sync_cmap_resources.py", ("CMap/Test-H",)),
        ):
            with self.subTest(script=script):
                module = load_script(script)
                payload = io.BytesIO()
                source = b"complete upstream source body\n"
                with tarfile.open(fileobj=payload, mode="w:gz") as archive:
                    for name in names:
                        info = tarfile.TarInfo(f"upstream/{name}")
                        info.size = len(source)
                        archive.addfile(info, io.BytesIO(source))
                def fetch(url):
                    with patch.object(module, "UPSTREAM_TARBALL", url):
                        return module.download_files()
                self.check_retry(module, payload.getvalue(), fetch, {Path(name).name: source for name in names})

    def test_normalized_afm_prominently_notes_modification(self) -> None:
        module = load_script("sync_core14_afm.py")
        source = b"StartFontMetrics 4.1\r\nComment Copyright Adobe.  \r\nEndFontMetrics\r\n"
        normalized = module.normalize(source)
        self.assertIn(b"Comment Modified by go-playa: normalized line endings and trailing whitespace.\n", normalized)
        self.assertIn(b"Comment Copyright Adobe.\n", normalized)
        self.assertEqual(normalized, module.normalize(normalized), "synchronization must remain idempotent")

    def check_retry(self, module, source, fetch, expected) -> None:

        class Handler(http.server.BaseHTTPRequestHandler):
            attempts = 0

            def do_GET(self) -> None:
                type(self).attempts += 1
                self.send_response(200)
                self.send_header("Content-Length", str(len(source)))
                self.end_headers()
                self.wfile.write(source[:8] if self.attempts == 1 else source)
                self.close_connection = True

            def log_message(self, *_args) -> None:
                pass

        with http.server.HTTPServer(("127.0.0.1", 0), Handler) as server:
            worker = threading.Thread(target=server.serve_forever, daemon=True)
            worker.start()
            try:
                with patch.object(module, "urlopen", side_effect=OSError("force curl fallback")):
                    result = fetch(f"http://127.0.0.1:{server.server_port}")
                self.assertEqual(result, expected, "retry must discard the incomplete first body")
                self.assertEqual(Handler.attempts, 2)
            finally:
                server.shutdown()
                worker.join()

    def test_python_go_generate_scripts_resolve_from_package_directory(self) -> None:
        result = subprocess.run(
            ["git", "ls-files", "-z", "--cached", "--others", "--exclude-standard", "--", "*.go"],
            cwd=ROOT,
            check=True,
            capture_output=True,
        )
        checked = 0
        for filename in sorted(set(result.stdout.decode().split("\0")) - {""}):
            source = ROOT / filename
            for line_number, line in enumerate(source.read_text(encoding="utf-8").splitlines(), 1):
                if not line.startswith(("//go:generate ", "//go:generate\t")):
                    continue
                command = shlex.split(line.removeprefix("//go:generate"))
                if not command or command[0] not in {"python", "python3"}:
                    continue
                checked += 1
                with self.subTest(source=filename, line=line_number):
                    self.assertGreaterEqual(len(command), 2, "Python generator must name a script")
                    script = (source.parent / command[1]).resolve()
                    self.assertTrue(
                        script.is_file(),
                        f"{command[1]} does not resolve from {source.parent.relative_to(ROOT)}",
                    )
        self.assertGreater(checked, 0, "expected Python go:generate directives")

    def test_cff_generator_writes_to_owning_fontdata_package(self) -> None:
        source = ROOT / "document/cff.go"
        directive = next(line for line in source.read_text().splitlines()
                         if line.startswith("//go:generate python3 "))
        command = shlex.split(directive.removeprefix("//go:generate"))
        output = command[command.index("--output-dir") + 1]
        self.assertEqual((source.parent / output).resolve(), ROOT / "fontdata")

    def test_generated_headers_preserve_pdfbox_modification_notices(self) -> None:
        for filename in ("cff_encoding_generated.go", "macexpert_encoding_generated.go"):
            with self.subTest(filename=filename):
                source = (ROOT / "fontdata" / filename).read_text(encoding="utf-8")
                header = source.split("var ", 1)[0]
                self.assertIn("Code generated by scripts/", header)
                self.assertIn("DO NOT EDIT.", header)
                self.assertIn("Modified/translated from Apache PDFBox material", header)
                for notice in ("NOTICE", "THIRD_PARTY_LICENSES", "scripts/data/pdfbox/LICENSE.txt"):
                    with self.subTest(notice=notice):
                        self.assertIn(notice, header)

    def test_pdf_macexpert_and_cff_expert_are_distinct(self) -> None:
        mac = load_script("generate_macexpert_encoding.py")
        cff = load_script("generate_cff_resources.py")

        mac_names = mac.read_encoding(ROOT / "scripts/data/pdfbox/MacExpertEncoding.java")
        cff_sids = cff.read_expert_encoding(ROOT / "scripts/data/pdfbox/CFFExpertEncoding.java")

        self.assertEqual(len(mac_names), 256)
        self.assertEqual(len(cff_sids), 256)
        self.assertEqual(mac_names[35], "centoldstyle")
        self.assertEqual(mac_names[60], ".notdef")
        self.assertEqual(cff_sids[35], 0)
        self.assertEqual(cff_sids[60], 249)


if __name__ == "__main__":
    unittest.main()
