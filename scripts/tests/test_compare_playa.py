import importlib.util
import json
import sys
import unittest
from pathlib import Path


ROOT = Path(__file__).resolve().parents[2]
SPEC = importlib.util.spec_from_file_location("compare_playa", ROOT / "scripts" / "compare_playa.py")
assert SPEC is not None and SPEC.loader is not None
compare_playa = importlib.util.module_from_spec(SPEC)
sys.modules[SPEC.name] = compare_playa
SPEC.loader.exec_module(compare_playa)


class ManifestTest(unittest.TestCase):
    def test_default_selection_uses_all_required_sections(self) -> None:
        manifest = compare_playa.load_manifest(ROOT / "compat" / "manifest.toml")
        self.assertEqual(
            compare_playa.select_sections(manifest, []),
            [section.name for section in manifest.sections if section.status == "required"],
        )

    def test_selected_section_must_exist(self) -> None:
        manifest = compare_playa.load_manifest(ROOT / "compat" / "manifest.toml")
        with self.assertRaises(ValueError):
            compare_playa.select_sections(manifest, ["missing"])

    def test_section_projection_keeps_only_selected_fields(self) -> None:
        snapshot = {
            "schema_version": "go-playa.compat/v1",
            "page_count": 1,
            "page_labels": ["1"],
            "pages": [{"index": 0, "label": "1", "width": 200, "height": 100, "rotation": 0, "text": [{"chars": "A"}]}],
        }
        self.assertEqual(
            compare_playa.project_sections(snapshot, ["content.text"]),
            {"schema_version": "go-playa.compat/v1", "pages": [{"index": 0, "text": [{"chars": "A"}]}]},
        )

    def test_document_projection_includes_public_document_flags(self) -> None:
        snapshot = {
            "schema_version": "go-playa.compat/v1",
            "page_count": 1,
            "page_labels": ["1"],
            "pdf_version": "1.7",
            "is_tagged": True,
            "is_printable": True,
            "is_modifiable": False,
            "is_extractable": True,
            "info": {},
            "catalog": {},
            "names": {},
            "trailer": {},
            "open_action": None,
        }
        self.assertEqual(
            compare_playa.project_sections(snapshot, ["document"]),
            snapshot,
        )

    def test_graphics_state_projection_reports_clipping_presence(self) -> None:
        class ClippedState:
            clipping_path = []

        self.assertFalse(compare_playa.graphics_state_projection(object())["has_clip"])
        self.assertTrue(compare_playa.graphics_state_projection(ClippedState())["has_clip"])

    def test_object_section_is_projected_independently(self) -> None:
        snapshot = {
            "schema_version": "go-playa.compat/v1",
            "objects": [{"object": 1, "generation": 0, "value": {"Type": "Catalog"}}],
        }
        self.assertEqual(compare_playa.project_sections(snapshot, ["document.objects"]), snapshot)

    def test_mapping_section_is_projected_independently(self) -> None:
        snapshot = {
            "schema_version": "go-playa.compat/v1",
            "mapping": [{"object": 1, "value": {"Type": "Catalog"}}],
        }
        self.assertEqual(compare_playa.project_sections(snapshot, ["document.mapping"]), snapshot)

    def test_content_structure_is_a_page_section(self) -> None:
        self.assertTrue(compare_playa.has_page_sections(["content.structure"]))

    def test_structure_projection_ignores_unused_page_slots(self) -> None:
        self.assertEqual(compare_playa.structure_projection([None]), [])

    def test_cache_key_changes_with_space_and_upstream_commit(self) -> None:
        pdf = ROOT / "testdata" / "files" / "form_simple.pdf"
        upstream = {"package": "example-playa", "version": "9.8.7", "tag": "v9.8.7", "commit": "commit-a"}
        first, metadata = compare_playa.cache_key(pdf, [], "page", upstream)
        second, _ = compare_playa.cache_key(pdf, [], "screen", upstream)
        upstream["commit"] = "commit-b"
        third, _ = compare_playa.cache_key(pdf, [], "page", upstream)
        self.assertNotEqual(first, second)
        self.assertNotEqual(first, third)
        self.assertEqual(metadata["space"], "page")

    def test_python_dependency_pin_matches_canonical_upstream(self) -> None:
        upstream = compare_playa.load_upstream(ROOT / "compat" / "upstream.toml")
        pyproject = compare_playa.tomllib.loads((ROOT / "compat" / "pyproject.toml").read_text())
        dependencies = pyproject["project"]["dependencies"]
        self.assertIn(f"{upstream['package']}[crypto]=={upstream['version']}", dependencies)

    def test_xref_projection_skips_stale_missing_entries(self) -> None:
        class XRefTable(dict):
            trailer = {}

            def __iter__(self):
                yield 4624

            def __getitem__(self, key):
                raise KeyError(key)

        class Document:
            xrefs = [XRefTable()]

        self.assertEqual(
            compare_playa.xref_projection(Document()),
            [{"kind": "table", "entries": [], "trailer": {}}],
        )

    def test_mapping_projection_skips_stale_missing_entries(self) -> None:
        class Document:
            def __iter__(self):
                return iter([1, 2, 3])

            def __getitem__(self, key):
                if key == 2:
                    raise KeyError(key)
                return {"Object": key}

            def items(self):
                for key in self:
                    yield key, self[key]

        self.assertEqual(
            compare_playa.document_mapping_projection(Document()),
            [
                {"object": 1, "value": {"Object": 1}},
                {"object": 3, "value": {"Object": 3}},
            ],
        )


class CacheReleaseTest(unittest.TestCase):
    def test_release_page_caches_preserves_shared_page_dictionaries(self) -> None:
        from types import SimpleNamespace

        resources = {"Font": {"F1": object()}}
        attrs = {"Resources": resources, "Type": "Page"}
        contents = [object()]
        page = SimpleNamespace(attrs=attrs, resources=resources, _contents=contents)
        next_page = SimpleNamespace(resources=resources)

        compare_playa.release_page_caches(object(), page)

        self.assertIn("F1", next_page.resources["Font"])
        self.assertEqual(attrs["Type"], "Page")
        self.assertIs(attrs["Resources"], resources)
        self.assertEqual(len(contents), 1)
        self.assertEqual(page.attrs, {})
        self.assertEqual(page.resources, {})
        self.assertEqual(page._contents, [])
        self.assertIsNot(page.attrs, attrs)
        self.assertIsNot(page.resources, resources)

    def test_release_document_caches_clears_parse_caches(self) -> None:
        class Document:
            _cached_objs = {1: object()}
            _parsed_objs = {2: object()}
            _cached_fonts = {3: object()}
            _cached_inline_images = {4: object()}

        document = Document()
        compare_playa.release_document_caches(document)
        self.assertEqual(document._cached_objs, {})
        self.assertEqual(document._parsed_objs, {})
        self.assertEqual(document._cached_fonts, {})
        self.assertEqual(document._cached_inline_images, {})

    def test_release_page_caches_clears_document_parse_caches(self) -> None:
        class Document:
            _cached_objs = {1: object()}
            _parsed_objs = {2: object()}
            _cached_fonts = {3: object()}
            _cached_inline_images = {4: object()}

        document = Document()
        compare_playa.release_page_caches(document)
        self.assertEqual(document._cached_objs, {})
        self.assertEqual(document._parsed_objs, {})
        self.assertEqual(document._cached_fonts, {})
        self.assertEqual(document._cached_inline_images, {})

    def test_release_page_caches_drops_page_local_views(self) -> None:
        class Page:
            _structmap = object()
            _marked_contents = object()
            _fontmap = object()
            _textmap = object()

        page = Page()
        compare_playa.release_page_caches(object(), page)
        self.assertIsNone(page._structmap)
        self.assertIsNone(page._marked_contents)
        self.assertIsNone(page._fontmap)
        self.assertIsNone(page._textmap)

    def test_release_page_caches_collects_page_cycles(self) -> None:
        from unittest.mock import patch

        with patch.object(compare_playa.gc, "collect", return_value=3) as collect:
            compare_playa.release_page_caches(object())
        collect.assert_called_once_with()

    def test_release_snapshot_caches_drops_global_projection_views(self) -> None:
        class Document:
            _outline = object()
            _destinations = object()
            _structure = object()
            _fontmap = object()

        document = Document()
        compare_playa.release_snapshot_caches(document, ["document"])
        self.assertIsNone(document._outline)
        self.assertIsNone(document._destinations)
        self.assertIsNone(document._structure)
        self.assertIsNone(document._fontmap)


class AnnotationProjectionTest(unittest.TestCase):
    def test_annotation_without_struct_parent_does_not_resolve_page_structure(self) -> None:
        class Annotation:
            props = {}

            @property
            def parent(self):
                raise AssertionError("parent must not be resolved without StructParent")

        self.assertIsNone(compare_playa.annotation_parent_projection(Annotation()))

    def test_object_projection_reuses_repeated_indirect_references(self) -> None:
        from playa import pdftypes

        reference = pdftypes.ObjRef(None, 7)
        original = pdftypes.resolve1
        calls = 0

        def resolve(value):
            nonlocal calls
            if value is reference:
                calls += 1
                return {"Name": b"cached"}
            return original(value)

        pdftypes.resolve1 = resolve
        try:
            projected = compare_playa.object_projection([reference, reference], {})
        finally:
            pdftypes.resolve1 = original
        self.assertEqual(projected, [{"Name": "cached"}, {"Name": "cached"}])
        self.assertEqual(calls, 1)

    def test_raw_object_projection_preserves_indirect_references(self) -> None:
        from playa import pdftypes

        reference = pdftypes.ObjRef(None, 7)
        self.assertEqual(
            compare_playa.object_projection([reference], resolve_references=False),
            [{"ref": 7}],
        )

    def test_release_snapshot_caches_keeps_structure_for_page_structure_sections(self) -> None:
        class Document:
            _outline = object()
            _destinations = object()
            _structure = object()
            _fontmap = object()

        document = Document()
        compare_playa.release_snapshot_caches(document, ["annotations"])
        self.assertIsNone(document._outline)
        self.assertIsNone(document._destinations)
        self.assertIsNotNone(document._structure)
        self.assertIsNone(document._fontmap)


class ResourceGraphProjectionTest(unittest.TestCase):
    def test_shared_cycles_keep_each_node_once_and_hash_decoded_stream(self) -> None:
        import hashlib
        from unittest.mock import patch
        class Reference:
            def __init__(self, objid):
                self.objid = objid

            def resolve(self):
                calls.append(self.objid)
                return values[self.objid]

        refs = {key: Reference(key) for key in (7, 8, 9)}

        class Stream:
            attrs = {"Back": refs[7]}
            reads = 0
            payload = b"decoded\x00resource"

            @property
            def buffer(self):
                self.reads += 1
                return self.payload

        stream = Stream()
        values = {
            7: {"Children": [refs[8]] * 64, "Payload": refs[9]},
            8: {"Parent": refs[7], "Bytes": b"\x00\xff"},
            9: stream,
        }
        calls = []

        root = {"Z": refs[7], "A": refs[8], "Again": refs[7]}
        with patch("playa.pdftypes.ObjRef", Reference):
            actual = compare_playa.resource_graph_projection(root)
            repeat = compare_playa.resource_graph_projection(dict(reversed(list(root.items()))))
            stream.payload = b"changed"
            changed = compare_playa.resource_graph_projection(root)
        self.assertEqual(actual, repeat)
        self.assertLess(len(json.dumps(actual)), 4096)
        self.assertEqual([node["object"] for node in actual["objects"]], [7, 8, 9])
        self.assertEqual(actual["root"]["Again"], {"ref": 7})
        self.assertEqual(actual["objects"][1]["value"]["Parent"], {"ref": 7})
        self.assertEqual(actual["objects"][1]["value"]["Bytes"], "00ff")
        self.assertEqual(actual["objects"][2]["value"], {
            "dict": {"Back": {"ref": 7}},
            "length": len(b"decoded\x00resource"),
            "sha256": hashlib.sha256(b"decoded\x00resource").hexdigest(),
        })
        self.assertNotEqual(actual["objects"][2]["value"]["sha256"], changed["objects"][2]["value"]["sha256"])
        self.assertEqual(sorted(calls), [7, 7, 7, 8, 8, 8, 9, 9, 9])
        self.assertEqual(stream.reads, 3)

    def test_reference_alias_cycle_preserves_both_nodes(self) -> None:
        from unittest.mock import patch

        class Reference:
            def __init__(self, objid):
                self.objid = objid

            def resolve(self):
                return references[2 if self.objid == 1 else 1]

        references = {key: Reference(key) for key in (1, 2)}
        with patch("playa.pdftypes.ObjRef", Reference):
            actual = compare_playa.resource_graph_projection({"Alias": references[1]})
        self.assertEqual(actual["objects"], [
            {"object": 1, "value": {"ref": 2}},
            {"object": 2, "value": {"ref": 1}},
        ])

    def test_empty_and_absent_resources(self) -> None:
        self.assertIsNone(compare_playa.resource_graph_projection(None))
        self.assertEqual(compare_playa.resource_graph_projection({}), {"root": {}, "objects": []})


class PageIterationTest(unittest.TestCase):
    def test_page_section_selection_is_explicit(self) -> None:
        self.assertEqual(
            compare_playa.page_section_flags(["content.flatten", "pages"]),
            {
                "pages": True,
                "text": False,
                "flatten": True,
                "interp": False,
                "streams": False,
                "tokens": False,
                "xobjects": False,
                "paths": False,
                "images": False,
                "fonts": False,
                "tags": False,
                "structure": False,
                "marked": False,
                "annotations": False,
                "extract_text": False,
                "extract_text_tagged": False,
                "extract_text_untagged": False,
                "glyphs": False,
                "layout": False,
                "contents": False,
            },
        )

    def test_default_page_iteration_does_not_materialize_page_list(self) -> None:
        class Pages:
            def __iter__(self):
                yield "first"
                yield "second"

            def __getitem__(self, index):
                raise AssertionError(f"unexpected indexed access: {index}")

        class Document:
            pages = Pages()

        self.assertEqual(
            list(compare_playa.iter_snapshot_pages(Document(), [])),
            [(0, "first"), (1, "second")],
        )


class LayoutProjectionTest(unittest.TestCase):
    def test_stable_layout_distances_use_comparison_precision(self) -> None:
        self.assertEqual(
            compare_playa.stable_layout_distance(3624.2033079706200),
            compare_playa.stable_layout_distance(3624.2033079706266),
        )
        self.assertNotEqual(
            compare_playa.stable_layout_distance(3624.203307),
            compare_playa.stable_layout_distance(3624.203309),
        )

    def test_stable_layout_ids_do_not_depend_on_allocator_addresses(self) -> None:
        class TextBox:
            pass

        class TextGroup:
            pass

        class Miner:
            LTTextBox = TextBox
            LTTextGroup = TextGroup

        layout_id = compare_playa.stable_layout_id(Miner)
        first_box, second_box = TextBox(), TextBox()
        first_group, second_group = TextGroup(), TextGroup()

        self.assertEqual(layout_id(first_box), 1 << 60)
        self.assertEqual(layout_id(second_box), (1 << 60) + 1)
        self.assertEqual(layout_id(first_group), 0)
        self.assertEqual(layout_id(second_group), 1)
        self.assertEqual(layout_id(first_box), 1 << 60)

    def test_page_tokens_use_resolved_streams(self) -> None:
        from unittest.mock import patch

        class Stream:
            buffer = b"stream"

        class Page:
            streams = [Stream()]

        class Token:
            pass

        with patch("playa.parser.Lexer", return_value=[(0, Token())]) as lexer:
            self.assertEqual(len(list(compare_playa.iter_page_tokens(Page()))), 1)
            lexer.assert_called_once_with(b"stream")


class JSONLWriterTest(unittest.TestCase):
    def test_write_jsonl_record_writes_one_decodable_line(self) -> None:
        from io import StringIO

        output = StringIO()
        compare_playa.write_jsonl_record({"kind": "header", "value": "值"}, output)
        self.assertEqual(json.loads(output.getvalue()), {"kind": "header", "value": "值"})
        self.assertTrue(output.getvalue().endswith("\n"))

    def test_snapshot_jsonl_streams_records_without_record_temp_files(self) -> None:
        from io import StringIO
        from unittest.mock import patch

        def snapshot(*args, **kwargs):
            kwargs["header_output"].write('{"kind":"header","snapshot":{}}\n')
            compare_playa.write_jsonl_record({"kind": "object", "object": {"id": 1}}, kwargs["object_sink"])
            compare_playa.write_jsonl_record({"kind": "token", "token": {"name": "BT"}}, kwargs["token_sink"])
            kwargs["page_sink"]({"index": 0})
            return {}

        output = StringIO()
        with patch.object(compare_playa, "playa_snapshot", side_effect=snapshot), patch.object(
            compare_playa.tempfile, "NamedTemporaryFile", side_effect=AssertionError("record temp file")
        ):
            compare_playa.write_snapshot_jsonl(
                ROOT / "testdata" / "files" / "form_simple.pdf",
                [0],
                "page",
                output,
                sections=["document.objects", "document.tokens", "pages"],
            )

        self.assertEqual(
            [json.loads(line)["kind"] for line in output.getvalue().splitlines()],
            ["header", "object", "token", "page"],
        )

    def test_write_structure_projection_streams_nested_nodes(self) -> None:
        from io import StringIO
        from unittest.mock import patch

        class ContentItem:
            mcid = 7

        class Element:
            def __init__(self, child=None):
                self.type = "P"
                self.role = "P"
                self.page = None
                self.title = None
                self.language = None
                self.alternate_description = None
                self.actual_text = None
                self.abbreviation_expansion = None
                self.class_name = None
                self.attributes = {}
                self.contents = [ContentItem()]
                self.children = [child] if child is not None else []

            def __iter__(self):
                return iter(self.children)

        output = StringIO()
        with patch("playa.structure.Element", Element), patch("playa.structure.ContentItem", ContentItem):
            compare_playa.write_structure_projection([Element(Element())], output)
        self.assertEqual(
            json.loads(output.getvalue())[0]["contents"],
            [{"kind": "marked_content", "mcid": 7, "has_mcid": True}],
        )
        self.assertEqual(len(json.loads(output.getvalue())[0]["children"]), 1)


if __name__ == "__main__":
    unittest.main()
