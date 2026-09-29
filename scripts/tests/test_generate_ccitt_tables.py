import importlib.util
import json
import subprocess
import sys
import unittest
from pathlib import Path
from unittest.mock import patch


ROOT = Path(__file__).resolve().parents[2]
SPEC = importlib.util.spec_from_file_location(
    "generate_ccitt_tables", ROOT / "scripts" / "generate_ccitt_tables.py"
)
assert SPEC is not None and SPEC.loader is not None
generate_ccitt_tables = importlib.util.module_from_spec(SPEC)
sys.modules[SPEC.name] = generate_ccitt_tables
SPEC.loader.exec_module(generate_ccitt_tables)


class SourcePathTest(unittest.TestCase):
    @patch("subprocess.run")
    def test_source_path_uses_module_directory_reported_by_go(self, run) -> None:
        module_dir = Path("/cache/golang.org/x/image@selected")
        run.return_value = subprocess.CompletedProcess(
            args=[],
            returncode=0,
            stdout=json.dumps({"Dir": str(module_dir)}),
        )

        self.assertEqual(
            generate_ccitt_tables.source_path(),
            module_dir / "ccitt" / "table.go",
        )
        run.assert_called_once_with(
            ["go", "mod", "download", "-json", "golang.org/x/image"],
            cwd=ROOT,
            check=True,
            capture_output=True,
            text=True,
        )


if __name__ == "__main__":
    unittest.main()
