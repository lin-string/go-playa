#!/usr/bin/env python3
"""Generate the pinned Playa compatibility snapshot used by the Go driver."""

from __future__ import annotations

import argparse
from dataclasses import dataclass
import gc
import heapq
import hashlib
import importlib.metadata
import importlib.util
import json
import math
import os
import shlex
import subprocess
import sys
import tempfile
from pathlib import Path
from typing import Any

try:
    import tomllib
except ModuleNotFoundError:  # pragma: no cover - Python 3.10 compatibility
    import tomli as tomllib

SCHEMA_VERSION = "go-playa.compat/v6"
# Increment when snapshot serialization or projection semantics change.
CACHE_VERSION = 41
COORDINATE_SPACES = ("page", "screen", "default")

_LAYOUT_MINER: Any | None = None


def load_layout_miner() -> Any:
    """Load Playa's pinned Python miner source for deterministic layout ties.

    The wheel also contains a mypyc extension whose group-textbox heap uses
    ``id(obj)`` as its final ordering key. Those addresses depend on allocator
    history, so the same page can acquire a different, but geometrically
    equivalent, group tree when projected alone or after earlier pages. The
    source module lets the oracle replace that key with stable per-page ids
    while retaining Playa's implementation for every layout operation.
    """
    global _LAYOUT_MINER
    if _LAYOUT_MINER is not None:
        return _LAYOUT_MINER

    import playa

    source = Path(playa.__file__).with_name("miner.py")
    spec = importlib.util.spec_from_file_location("playa._go_playa_compat_miner", source)
    if spec is None or spec.loader is None:
        raise RuntimeError(f"cannot load Playa miner source from {source}")
    module = importlib.util.module_from_spec(spec)
    sys.modules[spec.name] = module
    spec.loader.exec_module(module)
    _LAYOUT_MINER = module
    return module


def stable_layout_id(miner: Any) -> Any:
    """Return a per-page replacement for Playa miner's allocator-based id."""
    real_id = id
    box_ids: dict[int, int] = {}
    group_ids: dict[int, int] = {}

    def layout_id(value: Any) -> int:
        identity = real_id(value)
        if isinstance(value, miner.LTTextGroup):
            if identity not in group_ids:
                group_ids[identity] = len(group_ids)
            return group_ids[identity]
        if isinstance(value, miner.LTTextBox):
            if identity not in box_ids:
                # Groups sort before boxes, matching the stable tie ids in the
                # Go implementation. The large base leaves ample room for all
                # groups on a page without depending on the final box count.
                box_ids[identity] = (1 << 60) + len(box_ids)
            return box_ids[identity]
        return identity

    return layout_id


def stable_layout_distance(value: float) -> float:
    """Normalize heap distances to the comparison's geometric precision."""
    return round(value, 6)


class StableLayoutHeap:
    """Normalize Playa group heap distances before allocator-independent ties."""

    @staticmethod
    def heapify(values: list[Any]) -> None:
        for index, item in enumerate(values):
            values[index] = (item[0], stable_layout_distance(item[1]), *item[2:])
        heapq.heapify(values)

    @staticmethod
    def heappush(values: list[Any], item: Any) -> None:
        heapq.heappush(values, (item[0], stable_layout_distance(item[1]), *item[2:]))

    @staticmethod
    def heappop(values: list[Any]) -> Any:
        return heapq.heappop(values)


@dataclass(frozen=True)
class Section:
    name: str
    status: str


@dataclass(frozen=True)
class Manifest:
    sections: tuple[Section, ...]


def load_manifest(path: Path) -> Manifest:
    data = tomllib.loads(path.read_text())
    sections = tuple(Section(name=item["name"], status=item["status"]) for item in data["section"])
    if len({section.name for section in sections}) != len(sections):
        raise ValueError("compatibility manifest has duplicate section names")
    if any(section.status not in {"required", "pending"} for section in sections):
        raise ValueError("compatibility manifest has an invalid section status")
    return Manifest(sections=sections)


def select_sections(manifest: Manifest, selected: list[str]) -> list[str]:
    known = {section.name for section in manifest.sections}
    unknown = [name for name in selected if name not in known]
    if unknown:
        raise ValueError(f"unknown compatibility section: {unknown[0]}")
    if selected:
        return selected
    return [section.name for section in manifest.sections if section.status == "required"]


def pending_sections(manifest: Manifest) -> list[str]:
    return [section.name for section in manifest.sections if section.status == "pending"]


def load_upstream(path: Path) -> dict[str, Any]:
    return tomllib.loads(path.read_text())["playa"]


def require_upstream_version(upstream: dict[str, Any]) -> None:
    installed = importlib.metadata.version(upstream["package"])
    if installed != upstream["version"]:
        raise RuntimeError(f"{upstream['package']} version is {installed}, want {upstream['version']}")


def project_sections(snapshot: dict[str, Any], sections: list[str]) -> dict[str, Any]:
    result: dict[str, Any] = {"schema_version": snapshot["schema_version"]}
    if "document" in sections:
        result["page_count"] = snapshot["page_count"]
        result["page_labels"] = snapshot["page_labels"]
        for field in ("pdf_version", "is_tagged", "is_printable", "is_modifiable", "is_extractable"):
            result[field] = snapshot[field]
        for field in ("info", "catalog", "names", "trailer", "open_action"):
            result[field] = snapshot[field]
    if "document.objects" in sections:
        result["objects"] = snapshot.get("objects", [])
    if "document.mapping" in sections:
        result["mapping"] = snapshot.get("mapping", [])
    if "document.tokens" in sections:
        result["tokens"] = snapshot.get("tokens", [])
    if "document.buffer" in sections:
        result["buffer"] = snapshot["buffer"]
    if "document.xrefs" in sections:
        result["xrefs"] = snapshot["xrefs"]
    if "destinations" in sections:
        result["destinations"] = snapshot["destinations"]
    if "outline" in sections:
        result["outline"] = snapshot["outline"]
    if "forms" in sections:
        result["need_appearances"] = snapshot["need_appearances"]
        result["forms"] = snapshot["forms"]
    if "structure" in sections:
        result["structure"] = snapshot["structure"]
    if "fonts" in sections:
        result["document_fonts"] = snapshot.get("document_fonts", [])

    include_page_fields = "pages" in sections
    include_text = "content.text" in sections
    include_flatten = "content.flatten" in sections
    include_interp = "content.interp" in sections
    include_streams = "content.streams" in sections
    include_tokens = "content.tokens" in sections
    include_xobjects = "content.xobjects" in sections
    include_paths = "content.paths" in sections
    include_images = "content.images" in sections
    include_fonts = "fonts" in sections
    include_tags = "content.tags" in sections
    include_structure = "content.structure" in sections
    include_marked = "content.marked" in sections
    include_annotations = "annotations" in sections
    if include_page_fields or include_text or include_flatten or include_interp or include_streams or include_tokens or include_xobjects or include_paths or include_images or include_fonts or include_tags or include_structure or include_marked or include_annotations:
        pages: list[dict[str, Any]] = []
        for page in snapshot["pages"]:
            item: dict[str, Any] = {"index": page["index"]}
            if include_page_fields:
                for field in ("label", "width", "height", "rotation", "parent_key"):
                    item[field] = page[field]
            if include_text:
                item["text"] = page["text"]
            if include_flatten:
                item["flatten"] = page["flatten"]
            if include_interp:
                item["interp"] = page["interp"]
            if include_streams:
                item["streams"] = page["streams"]
            if include_tokens:
                item["tokens"] = page["tokens"]
            if include_xobjects:
                item["xobjects"] = page["xobjects"]
            if include_paths:
                item["paths"] = page["paths"]
            if include_images:
                item["images"] = page["images"]
            if include_fonts:
                item["fonts"] = page["fonts"]
            if include_tags:
                item["tags"] = page["tags"]
            if include_structure:
                item["structure"] = page["structure"]
            if include_marked:
                item["marked"] = page["marked"]
            if include_annotations:
                item["annotations"] = page["annotations"]
            pages.append(item)
        result["pages"] = pages
    return result


def as_list(value: Any, length: int) -> list[float]:
    if value is None:
        return [0.0] * length
    return [float(item) for item in value]


def normalize_fontname(value: Any) -> Any:
    """Subset prefixes are arbitrary and are not part of font semantics."""
    if isinstance(value, str) and len(value) > 7 and value[6] == "+" and value[:6].isupper():
        return value[7:]
    return value


def destination_projection(destination: Any | None) -> dict[str, Any]:
    if destination is None:
        return {"page_index": -1, "view": "", "params": []}
    display = getattr(destination, "display", None)
    return {
        "page_index": int(getattr(destination, "page_idx", -1)),
        "view": getattr(display, "name", "") if display is not None else "",
        "params": [object_projection(item) for item in getattr(destination, "coords", [])],
    }


def action_text(value: Any) -> str:
    from playa.parser import PSLiteral
    from playa.pdftypes import resolve1
    from playa.utils import decode_text

    value = resolve1(value)
    if isinstance(value, bytes):
        return decode_text(value)
    if isinstance(value, PSLiteral):
        return value.name
    if isinstance(value, str):
        return value
    return ""


def action_file(value: Any) -> str:
    from playa.pdftypes import dict_value, resolve1

    value = resolve1(value)
    if isinstance(value, (bytes, str)):
        return action_text(value)
    try:
        props = dict_value(value)
    except TypeError:
        return ""
    return action_text(props.get("UF") or props.get("F"))


def action_script(value: Any) -> str:
    from playa.pdftypes import resolve1

    value = resolve1(value)
    text = action_text(value)
    if text:
        return text
    raw = getattr(value, "buffer", None)
    return action_text(raw)


def action_projection(action: Any, depth: int = 0) -> dict[str, Any] | None:
    """Project Playa's normalized action fields and its lazy /Next source."""
    if action is None or depth >= 64:
        return None
    from playa.outline import Action, Destination
    from playa.pdftypes import resolve1

    props = getattr(action, "props", {})
    kind = literal_name(props.get("S"))
    result: dict[str, Any] = {
        "kind": kind,
        "uri": action_text(props.get("URI")) if kind == "URI" else "",
        "file": action_file(props.get("F")) if kind in {"GoToR", "GoToE", "Launch"} else "",
        "name": action_text(props.get("N")) if kind == "Named" else "",
        "script": action_script(props.get("JS")) if kind == "JavaScript" else "",
        "raw": raw_object_projection(props),
        "destination": None,
        "next": [],
    }
    if kind in {"GoTo", "GoToR", "GoToE"}:
        raw_destination = resolve1(props.get("D"))
        try:
            result["destination"] = destination_projection(
                Destination.from_dest(raw_destination, action.doc)
            )
        except (KeyError, TypeError, ValueError):
            pass

    raw_next = resolve1(props.get("Next"))
    if isinstance(raw_next, list):
        next_items = raw_next
    elif raw_next is None:
        next_items = []
    else:
        next_items = [raw_next]
    for item in next_items:
        try:
            child = Action.from_dict(item, action.doc)
        except (KeyError, TypeError, ValueError):
            continue
        projected = action_projection(child, depth + 1)
        if projected is not None:
            result["next"].append(projected)
    return result


def annotation_action_projection(annotation: Any) -> dict[str, Any] | None:
    from playa.outline import Action
    from playa.pdftypes import resolve1

    try:
        raw = resolve1(annotation.props.get("A"))
        if raw is None:
            return None
        return action_projection(Action.from_dict(raw, annotation.page.doc))
    except (KeyError, TypeError, ValueError):
        return None


def open_action_projection(document: Any) -> dict[str, Any] | None:
    """Project the catalog /OpenAction through Playa's normalized Action API."""
    from playa.outline import ACTION_GOTO, Action
    from playa.pdftypes import resolve1

    try:
        raw = resolve1(document.catalog.get("OpenAction"))
        if raw is None:
            return None
        direct_destination = isinstance(raw, list)
        if direct_destination:
            raw = {"S": ACTION_GOTO, "D": raw}
        result = action_projection(Action.from_dict(raw, document))
        if result is not None and direct_destination:
            # A direct destination array is normalized to a temporary GoTo
            # action, but it has no source action dictionary to expose.
            result["raw"] = {}
        return result
    except (KeyError, TypeError, ValueError):
        return None


def outline_projection(items: Any) -> list[dict[str, Any]]:
    out: list[dict[str, Any]] = []
    for item in items or []:
        action = item.action
        action_type = getattr(action.type, "name", "") if action is not None else ""
        props = getattr(item, "props", {})
        node = {
            "title": item.title,
            "destination": destination_projection(item.destination),
            "action_kind": action_type,
            "action": action_projection(action),
            "count": int(props.get("Count", 0)),
            "has_count": "Count" in props,
            "children": outline_projection(list(item)),
        }
        out.append(node)
    return out


def structure_content_projection(item: Any) -> dict[str, Any]:
    from playa.structure import ContentItem

    if isinstance(item, ContentItem):
        return {"kind": "marked_content", "mcid": int(item.mcid), "has_mcid": True}
    return {"kind": "object", "mcid": 0, "has_mcid": False}


def structure_projection(items: Any) -> list[dict[str, Any]]:
    from playa.structure import Element

    out = []
    for item in items or []:
        if item is None:
            continue
        page = item.page
        out.append(
            {
                "type": item.type,
                "role": item.role,
                "page_index": int(page.page_idx) if page is not None else -1,
                "title": item.title or "",
                "language": item.language or "",
                "alternate_description": item.alternate_description or "",
                "actual_text": item.actual_text or "",
                "abbreviation": item.abbreviation_expansion or "",
                "class_name": item.class_name or "",
                "attributes": raw_object_projection(item.attributes or {}),
                "contents": [structure_content_projection(child) for child in item.contents],
                "children": structure_projection([child for child in item if isinstance(child, Element)]),
            }
        )
    return out


def _write_json_member(output: Any, name: str, value: Any, first: bool) -> bool:
    if not first:
        output.write(",")
    json.dump(name, output, ensure_ascii=False, separators=(",", ":"))
    output.write(":")
    json.dump(value, output, ensure_ascii=False, separators=(",", ":"))
    return False


def _write_json_key(output: Any, name: str, first: bool) -> bool:
    if not first:
        output.write(",")
    json.dump(name, output, ensure_ascii=False, separators=(",", ":"))
    output.write(":")
    return False


def write_structure_projection(items: Any, output: Any) -> None:
    """Write the structure projection without retaining the whole tree."""
    from playa.structure import Element

    output.write("[")
    first_item = True
    for item in items or []:
        if item is None:
            continue
        if not first_item:
            output.write(",")
        first_item = False
        page = item.page
        output.write("{")
        first = True
        for name, value in (
            ("type", item.type),
            ("role", item.role),
            ("page_index", int(page.page_idx) if page is not None else -1),
            ("title", item.title or ""),
            ("language", item.language or ""),
            ("alternate_description", item.alternate_description or ""),
            ("actual_text", item.actual_text or ""),
            ("abbreviation", item.abbreviation_expansion or ""),
            ("class_name", item.class_name or ""),
            ("attributes", raw_object_projection(item.attributes or {})),
        ):
            first = _write_json_member(output, name, value, first)
        first = _write_json_key(output, "contents", first)
        output.write("[")
        first_content = True
        for child in item.contents or []:
            if not first_content:
                output.write(",")
            first_content = False
            json.dump(structure_content_projection(child), output, ensure_ascii=False, separators=(",", ":"))
        output.write("]")
        first = _write_json_key(output, "children", first)
        write_structure_projection([child for child in item if isinstance(child, Element)], output)
        output.write("}")
    output.write("]")


def form_text(value: Any) -> str:
    from playa.parser import PSLiteral
    from playa.pdftypes import resolve1
    from playa.utils import decode_text

    value = resolve1(value)
    if isinstance(value, bytes):
        return decode_text(value)
    if isinstance(value, PSLiteral):
        return value.name
    if isinstance(value, (int, float)):
        return ("%f" % float(value)).rstrip("0").rstrip(".")
    return ""


def form_rect(value: Any) -> tuple[list[float], bool]:
    from playa.pdftypes import resolve1

    value = resolve1(value)
    if not isinstance(value, list) or len(value) < 4:
        return [0.0, 0.0, 0.0, 0.0], False
    rect = [float(resolve1(item)) for item in value[:4]]
    if rect[0] > rect[2]:
        rect[0], rect[2] = rect[2], rect[0]
    if rect[1] > rect[3]:
        rect[1], rect[3] = rect[3], rect[1]
    return rect, True


def form_projection(document: Any) -> tuple[bool, list[dict[str, Any]]]:
    from playa.parser import PSLiteral
    from playa.pdftypes import dict_value, list_value, resolve1

    try:
        acro = dict_value(document.catalog.get("AcroForm", {}))
    except TypeError:
        return False, []
    need = bool(resolve1(acro.get("NeedAppearances", False)))

    def merge(parent: dict[str, Any], child: dict[str, Any]) -> dict[str, Any]:
        merged = dict(parent)
        merged.update(child)
        return merged

    def walk(raw: Any, parent: dict[str, Any], parent_name: str, seen: set[int]) -> dict[str, Any] | None:
        ref_id = getattr(raw, "objid", None)
        if ref_id is not None:
            if ref_id in seen:
                return None
            seen.add(ref_id)
        value = resolve1(raw)
        if not isinstance(value, dict):
            return None
        merged = merge(parent, value)
        name = form_text(value.get("T"))
        full_name = name
        if parent_name:
            full_name = f"{parent_name}.{name}" if name else parent_name
        field_type = resolve1(merged.get("FT"))
        subtype = resolve1(value.get("Subtype"))
        rect, has_rect = form_rect(value.get("Rect"))
        options: list[str] = []
        raw_options = resolve1(merged.get("Opt"))
        if isinstance(raw_options, list):
            for option in raw_options:
                option = resolve1(option)
                if isinstance(option, list) and len(option) >= 2:
                    options.append(form_text(option[1]))
                else:
                    options.append(form_text(option))
        selected: list[int] = []
        raw_selected = resolve1(merged.get("I"))
        if isinstance(raw_selected, list):
            selected = [int(resolve1(item)) for item in raw_selected if isinstance(resolve1(item), int)]
        elif isinstance(raw_selected, int):
            selected = [raw_selected]
        out = {
            "name": name,
            "full_name": full_name,
            "field_type": field_type.name if isinstance(field_type, PSLiteral) else "",
            "flags": int(resolve1(merged.get("Ff", 0))),
            "has_flags": "Ff" in merged,
            "value": form_text(merged.get("V")),
            "default_value": form_text(merged.get("DV")),
            "default_appearance": form_text(merged.get("DA")),
            "options": options,
            "selected": selected,
            "rect": rect,
            "has_rect": has_rect,
            "is_widget": isinstance(subtype, PSLiteral) and subtype.name == "Widget",
            "kids": [],
        }
        kids = resolve1(value.get("Kids"))
        if isinstance(kids, list):
            for child in kids:
                nested = walk(child, merged, full_name, seen)
                if nested is not None:
                    out["kids"].append(nested)
        if ref_id is not None:
            seen.remove(ref_id)
        return out

    fields: list[dict[str, Any]] = []
    try:
        raw_fields = list_value(acro.get("Fields", []))
    except TypeError:
        raw_fields = []
    for field in raw_fields:
        projected = walk(field, {}, "", set())
        if projected is not None:
            fields.append(projected)
    return need, fields


def object_projection(
    value: Any,
    reference_cache: dict[int, Any] | None = None,
    *,
    resolve_references: bool = True,
) -> Any:
    from playa.parser import PSLiteral
    from playa.pdftypes import ObjRef, resolve1
    from playa.utils import decode_text

    reference_cache = reference_cache if reference_cache is not None else {}

    def project(value: Any, active_refs: set[int]) -> Any:
        if isinstance(value, ObjRef):
            ref_id = int(value.objid)
            if not resolve_references:
                return {"ref": ref_id}
            if ref_id in active_refs:
                return {"ref": ref_id}
            if ref_id in reference_cache:
                return reference_cache[ref_id]
            resolved = resolve1(value)
            projected = project(resolved, active_refs | {ref_id})
            reference_cache[ref_id] = projected
            return projected
        value = resolve1(value)
        if value is None or isinstance(value, (bool, int, float)):
            return value
        if isinstance(value, PSLiteral):
            return value.name
        if isinstance(value, bytes):
            return decode_text(value)
        if hasattr(value, "attrs") and hasattr(value, "buffer"):
            return {
                "dict": project(value.attrs, active_refs),
                "data": decode_text(value.buffer),
            }
        if isinstance(value, list):
            return [project(item, active_refs) for item in value]
        if isinstance(value, dict):
            return {
                key.name if isinstance(key, PSLiteral) else str(key): project(item, active_refs)
                for key, item in value.items()
            }
        return str(value)

    return project(value, set())


def resource_graph_projection(value: Any) -> dict[str, Any] | None:
    """Keep the complete reachable resource graph without expanding shared nodes.

    Resource stream bytes are compared by decoded length and SHA-256, like
    document objects, so shared Form resources cannot multiply raw stream data.
    """
    from playa.parser import PSLiteral
    from playa.pdftypes import ObjRef

    if value is None:
        return None
    pending: dict[int, Any] = {}
    queue: list[int] = []

    def project(item: Any) -> Any:
        if isinstance(item, ObjRef):
            key = int(item.objid)
            if key not in pending:
                pending[key] = item
                queue.append(key)
            return {"ref": key}
        if item is None or isinstance(item, (bool, int, float)):
            return item
        if isinstance(item, PSLiteral):
            return item.name
        if isinstance(item, bytes):
            return item.hex()
        attrs = getattr(item, "attrs", None)
        if isinstance(attrs, dict):
            raw = item.buffer
            return {
                "dict": project(attrs),
                "length": len(raw),
                "sha256": hashlib.sha256(raw).hexdigest(),
            }
        if isinstance(item, list):
            return [project(child) for child in item]
        if isinstance(item, dict):
            return {
                key.name if isinstance(key, PSLiteral) else str(key): project(child)
                for key, child in item.items()
            }
        return str(item)

    root = project(value)
    objects = []
    for key in queue:
        objects.append({"object": key, "value": project(pending[key].resolve())})
    objects.sort(key=lambda node: node["object"])
    return {"root": root, "objects": objects}


def document_object_projection(value: Any) -> Any:
    """Project document dictionaries without expanding indirect references."""
    from playa.parser import PSLiteral
    from playa.pdftypes import ObjRef

    if isinstance(value, ObjRef):
        return {"ref": int(value.objid)}
    if value is None or isinstance(value, (bool, int, float)):
        return value
    if isinstance(value, PSLiteral):
        return value.name
    if isinstance(value, bytes):
        return value.hex()
    if isinstance(value, list):
        return [document_object_projection(item) for item in value]
    if isinstance(value, dict):
        return {
            key.name if isinstance(key, PSLiteral) else str(key): document_object_projection(item)
            for key, item in value.items()
        }
    return str(value)


def document_dict_projection(value: Any) -> dict[str, Any]:
    if not isinstance(value, dict):
        return {}
    return document_object_projection(value)


def indirect_object_projection(item: Any) -> dict[str, Any]:
    value = item.obj
    record = {"object": int(item.objid), "generation": int(item.genno)}
    rawdata = getattr(value, "buffer", None)
    attrs = getattr(value, "attrs", None)
    if rawdata is not None and isinstance(attrs, dict):
        record["stream"] = {
            "dict": document_dict_projection(attrs),
            "length": len(rawdata),
            "sha256": hashlib.sha256(rawdata).hexdigest(),
        }
    else:
        record["value"] = document_object_projection(value)
    return record


def document_mapping_entry_projection(key: Any, value: Any) -> dict[str, Any]:
    """Project one Mapping entry without expanding indirect references."""
    record = {"object": int(key)}
    rawdata = getattr(value, "buffer", None)
    attrs = getattr(value, "attrs", None)
    if rawdata is not None and isinstance(attrs, dict):
        record["stream"] = {
            "dict": document_dict_projection(attrs),
            "length": len(rawdata),
            "sha256": hashlib.sha256(rawdata).hexdigest(),
        }
    else:
        record["value"] = document_object_projection(value)
    return record


def document_mapping_items(document: Any) -> Any:
    """Yield mapping entries while ignoring stale damaged-xref keys."""
    for key in document:
        try:
            value = document[key]
        except KeyError:
            # A malformed or rebuilt xref can retain a key whose object body
            # is absent. The Go mapping view and xref projection both skip it.
            continue
        yield key, value


def document_mapping_projection(document: Any) -> list[dict[str, Any]]:
    return [
        document_mapping_entry_projection(key, value)
        for key, value in document_mapping_items(document)
    ]


def token_projection(token: Any) -> dict[str, Any]:
    from playa.parser import PSLiteral, PSKeyword

    if isinstance(token, bool):
        return {"kind": "number", "value": int(token)}
    if isinstance(token, (int, float)):
        return {"kind": "number", "value": token}
    if isinstance(token, PSLiteral):
        return {"kind": "name", "value": token.name}
    if isinstance(token, PSKeyword):
        value = token.name
        if isinstance(value, bytes):
            value = value.decode("latin1")
        delimiter_kinds = {"[": "array_start", "]": "array_end", "<<": "dict_start", ">>": "dict_end"}
        if value in delimiter_kinds:
            return {"kind": delimiter_kinds[value]}
        return {"kind": "keyword", "value": value}
    if isinstance(token, bytes):
        return {"kind": "string", "value": token.hex()}
    return {"kind": type(token).__name__.lower(), "value": str(token)}


def content_value_projection(value: Any) -> Any:
    """Project one value from Playa's lazy content-object sequence."""
    from playa.parser import PSLiteral, PSKeyword
    from playa.pdftypes import ContentStream

    if isinstance(value, (bool, int, float, PSLiteral, PSKeyword, bytes)):
        return token_projection(value)
    if isinstance(value, list):
        return [content_value_projection(item) for item in value]
    if isinstance(value, dict):
        return {
            key.name if isinstance(key, PSLiteral) else str(key): content_value_projection(item)
            for key, item in value.items()
        }
    if isinstance(value, ContentStream):
        try:
            length = len(value.buffer)
        except Exception:
            length = len(value.rawdata)
        return {
            "kind": "stream",
            "length": length,
            "attrs": {
                key: content_value_projection(item)
                for key, item in value.attrs.items()
            },
        }
    return str(value)


def literal_name(item: Any) -> str:
    return getattr(item, "name", "") or ""


def color_projection(color: Any, color_space: Any) -> dict[str, Any]:
    return {
        "space": getattr(color_space, "name", "") or "",
        "values": [float(item) for item in (getattr(color, "values", None) or ())],
        "pattern": getattr(color, "pattern", None) or "",
        "components": int(getattr(color_space, "ncomponents", 0) or 0),
    }


def graphics_state_projection(state: Any) -> dict[str, Any]:
    return {
        "line_width": float(getattr(state, "linewidth", 0) or 0),
        "line_cap": int(getattr(state, "linecap", 0) or 0),
        "line_join": int(getattr(state, "linejoin", 0) or 0),
        "miter_limit": float(getattr(state, "miterlimit", 0) or 0),
        "dash": [float(item) for item in (getattr(getattr(state, "dash", None), "dash", None) or ())],
        "dash_phase": float(getattr(getattr(state, "dash", None), "phase", 0) or 0),
        "intent": literal_name(getattr(state, "intent", None)),
        "stroke_adjustment": bool(getattr(state, "stroke_adjustment", False)),
        "blend_mode": literal_name(getattr(state, "blend_mode", None)),
        "stroke_alpha": float(getattr(state, "salpha", 0) or 0),
        "fill_alpha": float(getattr(state, "nalpha", 0) or 0),
        "alpha_source": bool(getattr(state, "alpha_source", False)),
        "black_point_comp": literal_name(getattr(state, "black_pt_comp", None)),
        "flatness": float(getattr(state, "flatness", 0) or 0),
        "stroke_color": color_projection(getattr(state, "scolor", None), getattr(state, "scs", None)),
        "fill_color": color_projection(getattr(state, "ncolor", None), getattr(state, "ncs", None)),
        "has_font": getattr(state, "font", None) is not None,
        "font_size": float(getattr(state, "fontsize", 0) or 0),
        "character_spacing": float(getattr(state, "charspace", 0) or 0),
        "word_spacing": float(getattr(state, "wordspace", 0) or 0),
        "scaling": float(getattr(state, "scaling", 0) or 0),
        "leading": float(getattr(state, "leading", 0) or 0),
        "render_mode": int(getattr(state, "render_mode", 0) or 0),
        "rise": float(getattr(state, "rise", 0) or 0),
        "knockout": bool(getattr(state, "knockout", False)),
        "has_clip": getattr(state, "clipping_path", None) is not None,
    }


def marked_stack_projection(value: Any) -> list[dict[str, Any]]:
    return [
        {
            "tag": context.tag,
            "mcid": int(context.mcid or 0),
            "has_mcid": context.mcid is not None,
            "actual_text": raw_object_projection(context.props.get("ActualText", b"")) if "ActualText" in context.props else "",
            "properties": raw_object_projection(context.props),
        }
        for context in (getattr(value, "mcstack", None) or ())
    ]


def glyph_projection(glyph: Any) -> dict[str, Any]:
    page = getattr(glyph, "page", None)
    return {
        "text": glyph.text or "",
        "chars": getattr(glyph, "chars", "") or "",
        "cid": int(getattr(glyph, "cid", 0) or 0),
        "font_name": getattr(glyph, "fontname", "") or "",
        "font_size": float(getattr(glyph, "fontsize", None) or getattr(glyph, "size", 0) or 0),
        "size": float(getattr(glyph, "size", 0) or 0),
        "font_base": getattr(glyph, "fontbase", "") or "",
        "text_font": getattr(glyph, "textfont", "") or "",
        "matrix": as_list(getattr(glyph, "matrix", None), 6),
        "origin": as_list(glyph.origin, 2),
        "displacement": as_list(glyph.displacement, 2),
        "bbox": as_list(glyph.bbox, 4),
        "vertical": bool(getattr(getattr(glyph, "font", None), "vertical", False)),
        "mcid": int(getattr(glyph, "mcid", 0) or 0),
        "has_mcid": getattr(glyph, "mcid", None) is not None,
        "page_index": int(getattr(page, "page_idx", -1)) if page is not None else -1,
        "ctm": as_list(getattr(glyph, "ctm", None), 6),
        "state": graphics_state_projection(getattr(glyph, "gstate", None)),
        "marked_stack": marked_stack_projection(glyph),
        "child_count": len(glyph),
    }


def annotation_parent_projection(annotation: Any) -> dict[str, Any] | None:
    """Project an annotation parent without resolving an absent StructParent.

    Playa returns ``None`` for annotations without this optional key, but its
    property implementation attempts the page structure lookup before
    discovering that fact. Avoid rebuilding a large parent tree for ordinary
    link annotations that have no structural parent at all.
    """
    props = getattr(annotation, "props", None)
    if not isinstance(props, dict) or "StructParent" not in props:
        return None
    parent = annotation.parent
    return structure_projection([parent])[0] if parent is not None else None


def raw_object_projection(value: Any) -> Any:
    """Project a public raw property dictionary without following ObjRefs."""
    return object_projection(value, resolve_references=False)


def font_projection(name: str, font: Any) -> dict[str, Any]:
    metrics = []
    for cid in (0, 1, 32, 65, 255):
        metrics.append(
            {
                "cid": cid,
                "hdisp": float(font.hdisp(cid)),
                "vdisp": float(font.vdisp(cid)),
                "position": as_list(font.position(cid), 2),
                "bbox": as_list(font.char_bbox(cid), 4),
            }
        )
    decode = []
    codes = (b"\x00\x01",) if type(font).__name__ == "CIDFont" else (b" ", b"A")
    for code in codes:
        decode.append(
            {
                "code": code.hex(),
                "glyphs": [
                    {"cid": int(cid), "text": text}
                    for cid, text in font.decode(code)
                ],
            }
        )
    to_unicode = []
    cmap = getattr(font, "tounicode", None)
    mappings = getattr(cmap, "bytes2unicode", {}) if cmap is not None else {}
    for code in codes:
        value = mappings.get(code)
        to_unicode.append(
            {
                "code": code.hex(),
                "value": value or "",
                "mapped": value is not None,
            }
        )
    return {
        "name": name,
        "fontname": normalize_fontname(getattr(font, "fontname", "")),
        "basefont": normalize_fontname(getattr(font, "basefont", "")),
        "cidcoding": getattr(font, "cidcoding", "") or "",
        "flags": int(getattr(font, "flags", 0)),
        "ascent": float(getattr(font, "ascent", 0)),
        "descent": float(getattr(font, "descent", 0)),
        "leading": float(getattr(font, "leading", 0)),
        "italic_angle": float(getattr(font, "italic_angle", 0)),
        "default_width": float(getattr(font, "default_width", 0)),
        "matrix": as_list(getattr(font, "matrix", None), 6),
        "vertical": bool(getattr(font, "vertical", False)),
        "multibyte": False,
        "bbox": as_list(getattr(font, "bbox", None), 4),
        "metrics": metrics,
        "decode": decode,
        "to_unicode": to_unicode,
    }


def layout_node_projection(value: Any, kind: str | None = None) -> dict[str, Any]:
    name = type(value).__name__
    if kind is None:
        if name.startswith("LTTextLine"):
            kind = "line"
        elif name.startswith("LTTextBox"):
            kind = "textbox"
        elif name.startswith("LTTextGroup"):
            kind = "textgroup"
        else:
            kind = name.lower()
    record: dict[str, Any] = {
        "kind": kind,
        "text": value.get_text().rstrip("\n") if hasattr(value, "get_text") else "",
        "bbox": as_list(getattr(value, "bbox", None), 4),
        "vertical": name.endswith("Vertical") or name.endswith("TBRL"),
    }
    if kind == "textbox":
        record["index"] = int(getattr(value, "index", -1))
    if kind in {"textbox", "textgroup"}:
        record["children"] = [layout_node_projection(item) for item in value]
    return record


def layout_item_projection(value: Any, miner: Any) -> dict[str, Any]:
    LTCurve, LTFigure, LTImage, LTTextBox = miner.LTCurve, miner.LTFigure, miner.LTImage, miner.LTTextBox

    if isinstance(value, LTTextBox):
        node = layout_node_projection(value)
        return {
            "kind": "text_box",
            "bbox": node["bbox"],
            "has_bbox": getattr(value, "bbox", None) is not None,
            "text_box": node,
        }
    if isinstance(value, LTFigure):
        children = list(value)
        # Playa wraps both images and Form XObjects in LTFigure. An image
        # wrapper has exactly one LTImage child; all other figures are forms.
        kind = "image" if len(children) == 1 and isinstance(children[0], LTImage) else "xobject"
    elif isinstance(value, (LTCurve, LTImage)):
        kind = "path" if isinstance(value, LTCurve) else "image"
    else:
        kind = type(value).__name__.lower()
    return {
        "kind": kind,
        "bbox": as_list(getattr(value, "bbox", None), 4),
        "has_bbox": getattr(value, "bbox", None) is not None,
    }


def layout_projection(page: Any) -> dict[str, Any]:
    miner = load_layout_miner()
    miner.id = stable_layout_id(miner)
    miner.heapq = StableLayoutHeap
    LAParams, LTCurve, LTFigure = miner.LAParams, miner.LTCurve, miner.LTFigure
    LTImage, LTTextBox, LTTextGroup, LTTextLine = miner.LTImage, miner.LTTextBox, miner.LTTextGroup, miner.LTTextLine

    analyzed = miner.extract_page(page, LAParams())
    lines: list[Any] = []
    seen: set[int] = set()

    def collect_lines(value: Any) -> None:
        if isinstance(value, LTTextLine):
            if id(value) not in seen:
                seen.add(id(value))
                lines.append(value)
            return
        if isinstance(value, (LTTextBox, LTTextGroup)):
            for item in value:
                collect_lines(item)

    for item in analyzed:
        collect_lines(item)
    groups = getattr(analyzed, "groups", None) or []
    return {
        "lines": [layout_node_projection(item) for item in lines],
        "text_boxes": [layout_node_projection(item) for item in analyzed if isinstance(item, LTTextBox)],
        "text_groups": [layout_node_projection(item) for item in groups],
        "items": [layout_item_projection(item, miner) for item in analyzed if isinstance(item, (LTTextBox, LTFigure, LTCurve, LTImage))],
    }


def content_projection(value: Any) -> dict[str, Any]:
    bbox = getattr(value, "bbox", None)
    ctm = getattr(value, "ctm", None)
    page = getattr(value, "page", None)
    state = getattr(value, "gstate", None)
    parent = getattr(value, "parent", None)
    record: dict[str, Any] = {
        "kind": getattr(value, "object_type", type(value).__name__.lower()),
        "bbox": as_list(bbox, 4) if bbox is not None else [0.0, 0.0, 0.0, 0.0],
        "has_bbox": bbox is not None,
        "matrix": as_list(ctm, 6),
        "has_matrix": ctm is not None,
        "page_index": int(getattr(page, "page_idx", -1)) if page is not None else -1,
        "state": graphics_state_projection(state),
        "mcid": int(getattr(value, "mcid", 0) or 0),
        "has_mcid": getattr(value, "mcid", None) is not None,
        "marked_stack": marked_stack_projection(value),
        "parent": (structure_projection([parent])[0] if parent is not None else None),
        "child_count": len(value),
    }
    if record["kind"] == "text":
        record["text"] = getattr(value, "chars", "") or ""
        record["glyph_count"] = len(value)
    return record


def xref_projection(document: Any) -> list[dict[str, Any]]:
    result = []
    kind_names = {"XRefTable": "table", "XRefStream": "stream", "XRefFallback": "fallback"}
    for xref in document.xrefs:
        entries = []
        for object_id in sorted(xref):
            try:
                stream_id, position, generation = xref[object_id]
            except KeyError:
                # Damaged or lazily rebuilt xrefs can expose a stale key while
                # the backing object has already been discarded.
                continue
            entry = {
                "object": int(object_id),
                "generation": int(generation),
                "offset": int(position),
            }
            if stream_id is not None:
                entry["in_object_stream"] = True
                entry["object_stream"] = int(stream_id)
                entry["object_index"] = int(position)
                entry["offset"] = 0
            entries.append(entry)
        result.append({
            "kind": kind_names.get(type(xref).__name__, type(xref).__name__),
            "entries": entries,
            "trailer": document_dict_projection(xref.trailer),
        })
    return result


def pdf_digest(pdf_path: Path) -> str:
    digest = hashlib.sha256()
    with pdf_path.open("rb") as stream:
        for chunk in iter(lambda: stream.read(1024 * 1024), b""):
            digest.update(chunk)
    return digest.hexdigest()


def cache_key(pdf_path: Path, pages: list[int], space: str, upstream: dict[str, Any], password: str = "", sections: list[str] | None = None) -> tuple[str, dict[str, Any]]:
    metadata = {
        "cache_version": CACHE_VERSION,
        "schema_version": SCHEMA_VERSION,
        "package": upstream["package"],
        "version": upstream["version"],
        "tag": upstream["tag"],
        "commit": upstream["commit"],
        "pdf_sha256": pdf_digest(pdf_path),
        "pages": pages,
        "space": space,
        "sections": sorted(sections or []),
        "password_set": bool(password),
    }
    if password:
        metadata["password_sha256"] = hashlib.sha256(password.encode()).hexdigest()
    encoded = json.dumps(metadata, sort_keys=True, separators=(",", ":")).encode()
    return hashlib.sha256(encoded).hexdigest(), metadata


def cached_playa_snapshot(cache_dir: Path, pdf_path: Path, pages: list[int], space: str, upstream: dict[str, Any], password: str = "", sections: list[str] | None = None) -> tuple[dict[str, Any], bool]:
    key, metadata = cache_key(pdf_path, pages, space, upstream, password, sections)
    path = cache_dir / f"{key}.json"
    try:
        cached = json.loads(path.read_text())
        if cached.get("metadata") == metadata and isinstance(cached.get("snapshot"), dict):
            return cached["snapshot"], True
    except (FileNotFoundError, OSError, json.JSONDecodeError, AttributeError):
        pass

    snapshot = playa_snapshot(pdf_path, pages, space, password=password, sections=sections, include_objects="document.objects" in (sections or []), include_tokens="document.tokens" in (sections or []), include_mapping="document.mapping" in (sections or []))
    temporary: Path | None = None
    try:
        cache_dir.mkdir(parents=True, exist_ok=True)
        payload = json.dumps({"metadata": metadata, "snapshot": snapshot}, ensure_ascii=False, separators=(",", ":"))
        with tempfile.NamedTemporaryFile("w", dir=cache_dir, prefix=f".{key}.", suffix=".tmp", delete=False) as stream:
            stream.write(payload)
            temporary = Path(stream.name)
        os.replace(temporary, path)
    except OSError:
        if temporary is not None:
            temporary.unlink(missing_ok=True)
    return snapshot, False


def iter_snapshot_pages(document: Any, pages: list[int]) -> Any:
    if pages:
        for index in pages:
            yield index, document.pages[index]
        return
    for index, page in enumerate(document.pages):
        yield index, page


def page_section_flags(sections: list[str] | None) -> dict[str, bool]:
    selected = set(sections or ())
    return {
        "pages": not sections or "pages" in selected,
        "text": not sections or "content.text" in selected,
        "flatten": not sections or "content.flatten" in selected,
        "interp": not sections or "content.interp" in selected,
        "streams": not sections or "content.streams" in selected,
        "tokens": not sections or "content.tokens" in selected,
        "xobjects": not sections or "content.xobjects" in selected,
        "paths": not sections or "content.paths" in selected,
        "images": not sections or "content.images" in selected,
        "fonts": not sections or "fonts" in selected,
        "tags": not sections or "content.tags" in selected,
        "structure": not sections or "content.structure" in selected,
        "marked": not sections or "content.marked" in selected,
        "annotations": not sections or "annotations" in selected,
        "extract_text": not sections or "content.extract_text" in selected,
        "extract_text_tagged": not sections or "content.extract_text.tagged" in selected,
        "extract_text_untagged": not sections or "content.extract_text.untagged" in selected,
        "glyphs": not sections or "content.glyphs" in selected,
        "layout": not sections or "layout" in selected,
        "contents": not sections or "content.contents" in selected,
    }


PAGE_SECTIONS = frozenset(
    {
        "pages", "content.text", "content.extract_text", "content.glyphs",
        "content.extract_text.tagged", "content.extract_text.untagged",
        "content.flatten", "content.interp", "content.streams", "content.tokens",
        "content.xobjects", "content.contents", "content.structure", "layout",
        "annotations", "fonts", "content.paths", "content.images", "content.tags",
        "content.marked",
    }
)


def has_page_sections(sections: list[str]) -> bool:
    return bool(PAGE_SECTIONS.intersection(sections))


def iter_page_tokens(page: Any) -> Any:
    """Tokenize resolved page streams, including indirect content arrays.

    The pinned Playa oracle's ``Page.tokens`` iterates its private ``_contents`` list
    directly.  For pages whose ``Contents`` is an array of references that
    leaves ``ObjRef`` values in that list and raises while reading ``buffer``;
    ``Page.streams`` performs the intended dereference.  Keep the oracle
    usable for those valid PDFs while preserving the public token semantics.
    """
    from playa.parser import Lexer

    for stream in page.streams:
        for _, token in Lexer(stream.buffer):
            yield token


def playa_snapshot(
    pdf_path: Path,
    pages: list[int],
    space: str,
    page_sink: Any | None = None,
    header_output: Any | None = None,
    object_sink: Any | None = None,
    token_sink: Any | None = None,
    mapping_sink: Any | None = None,
    include_objects: bool = False,
    include_tokens: bool = False,
    include_mapping: bool = False,
    password: str = "",
    sections: list[str] | None = None,
    header_only: bool = False,
) -> dict[str, Any]:
    try:
        import playa
    except ImportError as exc:
        raise RuntimeError("playa-pdf is required; run with 'uv run --with playa-pdf'") from exc

    with playa.open(pdf_path, password=password, space=space) as document:
        need_appearances, forms = form_projection(document)
        result: dict[str, Any] = {
            "schema_version": SCHEMA_VERSION,
            "buffer": {
                "length": pdf_path.stat().st_size,
                "sha256": pdf_digest(pdf_path),
            },
            "xrefs": xref_projection(document),
            "page_count": len(document.pages),
            "page_labels": [page.label or "" for page in document.pages],
            "pdf_version": document.pdf_version,
            "is_tagged": bool(document.is_tagged),
            "is_printable": bool(document.is_printable),
            "is_modifiable": bool(document.is_modifiable),
            "is_extractable": bool(document.is_extractable),
            "info": document_dict_projection(document.info),
            "catalog": document_dict_projection(document.catalog),
            "names": document_dict_projection(document.names) if "Names" in document.catalog else {},
            "trailer": document_dict_projection(document.trailer),
            "open_action": open_action_projection(document),
            "destinations": [
                {"name": name, "destination": destination_projection(destination)}
                for name, destination in sorted(document.destinations.items())
            ],
            "outline": outline_projection(document.outline),
            "need_appearances": need_appearances,
            "forms": forms,
            "pages": [],
        }
        if sections is None or "fonts" in sections:
            result["document_fonts"] = [
                font_projection(name, font) for name, font in sorted(document.fonts.items())
            ]
        if include_objects:
            result["objects"] = [indirect_object_projection(item) for item in document.objects]
        if include_tokens:
            result["tokens"] = [token_projection(token) for token in document.tokens]
        if include_mapping:
            result["mapping"] = document_mapping_projection(document)
        if header_output is None:
            result["structure"] = structure_projection(document.structure)
        else:
            write_streaming_snapshot_header(header_output, result, document.structure)
            if header_only:
                return result
            if mapping_sink is not None:
                for index, (key, value) in enumerate(document_mapping_items(document), 1):
                    write_jsonl_record({"kind": "mapping", "mapping": document_mapping_entry_projection(key, value)}, mapping_sink)
                    if index % 128 == 0:
                        release_document_caches(document)
                release_document_caches(document)
            release_snapshot_caches(document, sections)
        if object_sink is not None:
            for index, indirect_object in enumerate(document.objects, 1):
                write_jsonl_record({"kind": "object", "object": indirect_object_projection(indirect_object)}, object_sink)
                if index % 128 == 0:
                    release_document_caches(document)
            release_document_caches(document)
        if token_sink is not None:
            for token in document.tokens:
                write_jsonl_record({"kind": "token", "token": token_projection(token)}, token_sink)
        if sections is not None and not has_page_sections(sections):
            return result
        flags = page_section_flags(sections)
        for index, page in iter_snapshot_pages(document, pages):
            text = []
            if flags["text"]:
                for item in page.texts:
                    text_font = getattr(item, "textfont", "") or ""
                    text_size = float(getattr(item, "size", 0) or 0)
                    text_vertical = bool(getattr(getattr(item, "font", None), "vertical", False))
                    text_mcid = getattr(item, "mcid", None)
                    glyphs = [glyph_projection(glyph) for glyph in item]
                    text.append(
                        {
                            "chars": item.chars,
                            "font_name": getattr(item, "fontname", "") or "",
                            "font_size": float(getattr(item, "fontsize", None) or text_size),
                            "size": text_size,
                            "font_base": getattr(item, "fontbase", "") or "",
                            "text_font": text_font,
                            "matrix": as_list(getattr(item, "matrix", None), 6),
                            "text_matrix": as_list(getattr(item, "text_matrix", None), 6),
                            "line_matrix": as_list(getattr(item, "line_matrix", None), 6),
                            "scaling_matrix": as_list(getattr(item, "scaling_matrix", None), 6),
                            "origin": as_list(getattr(item, "origin", None), 2),
                            "displacement": as_list(getattr(item, "displacement", None), 2),
                            "rotation": float(getattr(item, "rotation", 0) or 0),
                            "bbox": as_list(item.bbox, 4),
                            "vertical": text_vertical,
                            "mcid": int(text_mcid or 0),
                            "has_mcid": text_mcid is not None,
                            "page_index": index,
                            "ctm": as_list(getattr(item, "ctm", None), 6),
                            "state": graphics_state_projection(getattr(item, "gstate", None)),
                            "marked_stack": marked_stack_projection(item),
                            "args": [value.hex() if isinstance(value, bytes) else float(value) for value in getattr(item, "args", [])],
                            "glyphs": glyphs,
                        }
                    )
            annotations = [
                {
                    "type": annotation.type,
                    "rect": as_list(annotation.rect, 4),
                    "bbox": as_list(annotation.bbox, 4),
                    "page_index": index,
                    "contents": annotation.contents or "",
                    "name": annotation.name or "",
                    "modified": annotation.mtime or "",
                    "parent": annotation_parent_projection(annotation),
                    "action": annotation_action_projection(annotation),
                    "properties": raw_object_projection(annotation.props),
                }
                for annotation in page.annotations
            ] if flags["annotations"] else []
            from playa.content import TagObject
            from playa.content import PathObject
            from playa.content import ImageObject
            paths = []
            for path_object in (page.paths if flags["paths"] else ()):
                if not isinstance(path_object, PathObject):
                    continue
                enclosing = path_object.mcstack[-1] if path_object.mcstack else None
                stack = [
                    {
                        "tag": context.tag,
                        "mcid": context.mcid if context.mcid is not None else 0,
                        "has_mcid": context.mcid is not None,
                        "actual_text": raw_object_projection(context.props.get("ActualText", b"")) if "ActualText" in context.props else "",
                        "properties": raw_object_projection(context.props),
                    }
                    for context in path_object.mcstack
                ]
                paths.append(
                    {
                        "raw_segments": [
                            {"operator": segment.operator, "points": [list(point) for point in segment.points]}
                            for segment in path_object.raw_segments
                        ],
                        "segments": [
                            {"operator": segment.operator, "points": [list(point) for point in segment.points]}
                            for segment in path_object.segments
                        ],
                        "stroke": path_object.stroke,
                        "fill": path_object.fill,
                        "evenodd": path_object.evenodd,
                        "bbox": as_list(path_object.bbox, 4),
                        "page_index": index,
                        "ctm": as_list(getattr(path_object, "ctm", None), 6),
                        "state": graphics_state_projection(getattr(path_object, "gstate", None)),
                        "marked_tag": enclosing.tag if enclosing is not None else "",
                        "marked_properties": raw_object_projection(enclosing.props if enclosing is not None else {}),
                        "marked_stack": stack,
                    }
                )
            images = []
            for image_object in (page.images if flags["images"] else ()):
                if not isinstance(image_object, ImageObject):
                    continue
                enclosing = image_object.mcstack[-1] if image_object.mcstack else None
                stack = [
                    {
                        "tag": context.tag,
                        "mcid": context.mcid if context.mcid is not None else 0,
                        "has_mcid": context.mcid is not None,
                        "actual_text": raw_object_projection(context.props.get("ActualText", b"")) if "ActualText" in context.props else "",
                        "properties": raw_object_projection(context.props),
                    }
                    for context in image_object.mcstack
                ]
                raw_filters = image_object.stream.get("Filter")
                if raw_filters is None:
                    filters = []
                elif isinstance(raw_filters, list):
                    filters = [object_projection(value) for value in raw_filters]
                else:
                    filters = [object_projection(raw_filters)]
                colorspace = image_object.colorspace
                images.append(
                    {
                        "name": image_object.xobjid or "",
                        "width": int(image_object.srcsize[0]),
                        "height": int(image_object.srcsize[1]),
                        "bits": int(image_object.bits),
                        "image_mask": bool(image_object.imagemask),
                        "color_space": colorspace.name if colorspace is not None else "",
                        "components": int(colorspace.ncomponents) if colorspace is not None else 0,
                        "filters": filters,
                        "stream_length": len(image_object.buffer),
                        "stream_sha256": hashlib.sha256(image_object.buffer).hexdigest(),
                        "bbox": as_list(image_object.bbox, 4),
                        "page_index": index,
                        "ctm": as_list(getattr(image_object, "ctm", None), 6),
                        "state": graphics_state_projection(getattr(image_object, "gstate", None)),
                        "marked_tag": enclosing.tag if enclosing is not None else "",
                        "marked_properties": raw_object_projection(enclosing.props if enclosing is not None else {}),
                        "marked_stack": stack,
                    }
                )
            marked = []
            for section in (page.marked_content.page_order if flags["marked"] else ()):
                objects = list(section)
                mcid = next((obj.mcid for obj in objects if obj.mcid is not None), None)
                if mcid is None:
                    continue
                marked.append({"mcid": int(mcid), "texts": list(section.texts)})
            fonts = [font_projection(name, font) for name, font in sorted(page.fonts.items())] if flags["fonts"] else []
            tags = []
            for tag in (page if flags["tags"] else ()):
                if not isinstance(tag, TagObject):
                    continue
                mcs = tag.mcs
                stack = [
                    {
                        "tag": context.tag,
                        "mcid": context.mcid if context.mcid is not None else 0,
                        "has_mcid": context.mcid is not None,
                        "actual_text": raw_object_projection(context.props.get("ActualText", b"")) if "ActualText" in context.props else "",
                        "properties": raw_object_projection(context.props),
                    }
                    for context in tag.mcstack
                ]
                enclosing = tag.mcstack[-1] if tag.mcstack else None
                tags.append(
                    {
                        "tag": mcs.tag,
                        "mcid": mcs.mcid if mcs.mcid is not None else 0,
                        "has_mcid": mcs.mcid is not None,
                        "actual_text": raw_object_projection(mcs.props.get("ActualText", b"")) if "ActualText" in mcs.props else "",
                        "properties": raw_object_projection(mcs.props),
                        "marked_tag": enclosing.tag if enclosing is not None else "",
                        "marked_properties": raw_object_projection(enclosing.props if enclosing is not None else {}),
                        "marked_stack": stack,
                        "page_index": index,
                        "ctm": as_list(getattr(tag, "ctm", None), 6),
                        "state": graphics_state_projection(getattr(tag, "gstate", None)),
                    }
                )
            page_result = {"index": index}
            if flags["pages"]:
                page_result.update({
                    "label": page.label or "",
                    "width": float(page.width),
                    "height": float(page.height),
                    "rotation": int(page.rotate),
                    "parent_key": getattr(page, "parent_key", None),
                })
            if flags["text"]:
                page_result["text"] = text
            if flags["paths"]:
                page_result["paths"] = paths
            if flags["images"]:
                page_result["images"] = images
            if flags["fonts"]:
                page_result["fonts"] = fonts
            if flags["tags"]:
                page_result["tags"] = tags
            if flags["structure"]:
                page_result["structure"] = structure_projection(page.structure)
            if flags["marked"]:
                page_result["marked"] = marked
            if flags["annotations"]:
                page_result["annotations"] = annotations
            if flags["flatten"]:
                page_result["flatten"] = [content_projection(item) for item in page.flatten()]
            if flags["interp"]:
                page_result["interp"] = [content_projection(item) for item in page.interp()]
            if flags["streams"]:
                page_result["streams"] = [{"length": len(stream.buffer), "sha256": hashlib.sha256(stream.buffer).hexdigest()} for stream in page.streams]
            if flags["tokens"]:
                page_result["tokens"] = [token_projection(token) for token in iter_page_tokens(page)]
            if flags["contents"]:
                page_result["contents"] = [content_value_projection(value) for value in page.contents]
            if flags["xobjects"]:
                page_result["xobjects"] = [
                    {
                        "name": xobject.xobjid or "",
                        "bbox": as_list(xobject.bbox, 4),
                        "resources": resource_graph_projection(xobject.resources),
                        "group": object_projection(xobject.group),
                        "fonts": [font_projection(name, font) for name, font in sorted(xobject.fonts.items())],
                        "structure": structure_projection(xobject.structure),
                        "page_index": int(getattr(getattr(xobject, "page", None), "page_idx", index)),
                        "ctm": as_list(getattr(xobject, "ctm", None), 6),
                        "state": graphics_state_projection(getattr(xobject, "gstate", None)),
                        "marked_stack": marked_stack_projection(xobject),
                        "length": len(raw),
                        "sha256": hashlib.sha256(raw).hexdigest(),
                        "tokens": [token_projection(token) for token in xobject.tokens],
                        "contents": [content_value_projection(value) for value in xobject.contents],
                    }
                    for xobject in page.xobjects
                    for raw in [xobject.buffer]
                ]
            if sections is None or "layout" in sections:
                page_result["layout"] = layout_projection(page)
            if sections is None or "content.extract_text" in sections:
                page_result["extract_text"] = page.extract_text()
            if sections is None or "content.extract_text.tagged" in sections:
                page_result["extract_text_tagged"] = page.extract_text_tagged()
            if sections is None or "content.extract_text.untagged" in sections:
                page_result["extract_text_untagged"] = page.extract_text_untagged()
            if sections is None or "content.glyphs" in sections:
                page_result["glyphs"] = [glyph_projection(glyph) for glyph in page.glyphs]
            if page_sink is None:
                result["pages"].append(page_result)
            else:
                page_sink(page_result)
            release_page_caches(document, page)
    return result


def write_jsonl_record(record: dict[str, Any], output: Any) -> None:
    json.dump(record, output, ensure_ascii=False, separators=(",", ":"))
    output.write("\n")


def write_streaming_snapshot_header(output: Any, snapshot: dict[str, Any], structure: Any) -> None:
    output.write('{"kind":"header","snapshot":{')
    first = True
    for name, value in snapshot.items():
        if name == "pages":
            continue
        first = _write_json_member(output, name, value, first)
    first = _write_json_key(output, "structure", first)
    write_structure_projection(structure, output)
    output.write("}}\n")


def write_snapshot_jsonl(pdf_path: Path, pages: list[int], space: str, output: Any, password: str = "", sections: list[str] | None = None) -> None:
    """Write one header and one record per object/page without record staging.

    ``playa_snapshot`` visits document objects and tokens before pages, so all
    record sinks can share the caller's output stream. Keeping the records in
    temporary files adds disk I/O and leaves large files behind when a worker
    is interrupted.
    """
    sections = sections or []

    def write_page(page: dict[str, Any]) -> None:
        write_jsonl_record({"kind": "page", "page": page}, output)

    object_sink = output if "document.objects" in sections else None
    token_sink = output if "document.tokens" in sections else None
    mapping_sink = output if "document.mapping" in sections else None
    playa_snapshot(
        pdf_path,
        pages,
        space,
        page_sink=write_page,
        header_output=output,
        object_sink=object_sink,
        token_sink=token_sink,
        mapping_sink=mapping_sink,
        password=password,
        sections=sections,
    )


def release_page_caches(document: Any, page: Any | None = None) -> None:
    """Release parser objects retained only by the compatibility walk.

    Playa intentionally caches resolved indirect objects for normal interactive
    use. The compatibility command visits every page once, so retaining every
    parsed object would make peak memory proportional to document length.
    Page-local output has already been serialized when this is called. The
    optional page argument clears views cached directly on the Page as well.
    """
    release_document_caches(document)
    if page is not None:
        for name in ("_structmap", "_marked_contents", "_fontmap", "_textmap"):
            if hasattr(page, name):
                setattr(page, name, None)
        # PageList keeps Page instances for random access. Once this page's
        # projection has been serialized, release the large inherited/resource
        # dictionaries and content-stream references as well. These dictionaries
        # can be shared by other pages or resolved objects: detach references
        # without mutating their contents. Geometry, label, and page identity
        # remain available for later diagnostics.
        for name in ("attrs", "resources"):
            value = getattr(page, name, None)
            if isinstance(value, dict):
                setattr(page, name, {})
        if hasattr(page, "_contents"):
            page._contents = []
    # Playa content objects retain document/page back-references and can form
    # cycles after a page projection is serialized. Reclaim them before the
    # next page so long documents do not accumulate unreachable interpreters.
    gc.collect()


def release_document_caches(document: Any) -> None:
    """Release Playa caches that are not part of a serialized record.

    Document-level JSONL sections visit one object at a time. Keeping parsed
    object-stream members alive until the whole section finishes makes peak
    memory proportional to the PDF rather than the current record batch.
    """
    for name in ("_cached_objs", "_parsed_objs", "_cached_fonts", "_cached_inline_images"):
        cache = getattr(document, name, None)
        if cache is not None:
            cache.clear()


def release_snapshot_caches(document: Any, sections: list[str] | None = None) -> None:
    """Release global views that later page projections do not need.

    Page annotations, page structure, marked content, and rendered form
    XObjects can resolve through ``Document.structure``. Keep that lazy tree
    alive for those projections; rebuilding it for every page is both slow and
    extremely memory hungry on malformed parent trees.
    """
    for name in ("_outline", "_destinations", "_structure", "_fontmap"):
        if name == "_structure" and (
            not sections
            or set(sections).intersection(
                {"annotations", "content.structure", "content.marked", "content.xobjects"}
            )
        ):
            continue
        if hasattr(document, name):
            setattr(document, name, None)


def go_snapshot(repo_root: Path, pdf_path: Path, pages: list[int], space: str, command: str, sections: list[str] | None = None) -> dict[str, Any]:
    args = shlex.split(command) + ["--pdf", str(pdf_path)]
    if pages:
        args.extend(["--pages", ",".join(str(page) for page in pages)])
    args.extend(["--space", space])
    for section in sections or []:
        args.extend(["--section", section])
    completed = subprocess.run(args, cwd=repo_root, check=False, capture_output=True, text=True)
    if completed.returncode != 0:
        raise RuntimeError(f"go projection failed for {pdf_path}: {completed.stderr.strip()}")
    return json.loads(completed.stdout)


def compare(expected: Any, actual: Any, path: str, tolerance: float, differences: list[str]) -> None:
    if isinstance(expected, (int, float)) and not isinstance(expected, bool):
        if not isinstance(actual, (int, float)) or not math.isclose(expected, actual, abs_tol=tolerance, rel_tol=0):
            differences.append(f"{path}: playa={expected!r}, go={actual!r}")
        return
    if type(expected) is not type(actual):
        differences.append(f"{path}: playa type={type(expected).__name__}, go type={type(actual).__name__}")
        return
    if isinstance(expected, dict):
        keys = sorted(set(expected) | set(actual))
        for key in keys:
            child = f"{path}.{key}" if path else key
            if key not in expected:
                differences.append(f"{child}: unexpected Go value {actual[key]!r}")
            elif key not in actual:
                differences.append(f"{child}: missing Go value; playa={expected[key]!r}")
            else:
                compare(expected[key], actual[key], child, tolerance, differences)
        return
    if isinstance(expected, list):
        if len(expected) != len(actual):
            differences.append(f"{path}: playa length={len(expected)}, go length={len(actual)}")
        for index, (left, right) in enumerate(zip(expected, actual)):
            compare(left, right, f"{path}[{index}]", tolerance, differences)
        return
    if expected != actual:
        differences.append(f"{path}: playa={expected!r}, go={actual!r}")


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("pdf", nargs="+", type=Path, help="PDF files to compare")
    parser.add_argument("--page", dest="pages", action="append", type=int, default=[], help="zero-based page index; repeat to select several pages")
    parser.add_argument("--go", default="go run ./cmd/playa-compat", help="Go projection command")
    parser.add_argument("--space", dest="spaces", action="append", choices=COORDINATE_SPACES, default=[], help="coordinate space; repeat to compare several")
    parser.add_argument("--cache-dir", type=Path, default=None, help="Playa snapshot cache directory")
    parser.add_argument("--no-cache", action="store_true", help="disable Playa snapshot caching")
    parser.add_argument("--tolerance", type=float, default=1e-6, help="absolute float tolerance")
    parser.add_argument("--section", dest="sections", action="append", default=[], help="compatibility section to diagnose; repeat to select several")
    parser.add_argument("--release", action="store_true", help="fail when the compatibility manifest has pending sections")
    parser.add_argument("--snapshot-only", action="store_true", help="emit one Playa snapshot for the Go compatibility driver")
    parser.add_argument("--snapshot-jsonl", action="store_true", help="emit a streaming Playa snapshot, one page per JSONL record")
    parser.add_argument("--header-only", action="store_true", help="emit only the JSONL snapshot header")
    parser.add_argument("--password", default="", help="PDF password")
    args = parser.parse_args()
    if any(page < 0 for page in args.pages):
        parser.error("--page must be non-negative")

    repo_root = Path(__file__).resolve().parents[1]
    spaces = args.spaces or ["page"]
    if args.snapshot_only:
        if len(args.pdf) != 1 or len(spaces) != 1:
            parser.error("--snapshot-only requires exactly one PDF and one --space")
        try:
            require_upstream_version(load_upstream(repo_root / "compat" / "upstream.toml"))
            snapshot = None if args.snapshot_jsonl else playa_snapshot(args.pdf[0], args.pages, spaces[0], password=args.password, sections=args.sections, include_objects="document.objects" in args.sections, include_tokens="document.tokens" in args.sections, include_mapping="document.mapping" in args.sections)
        except (RuntimeError, importlib.metadata.PackageNotFoundError) as exc:
            print(f"compatibility environment error: {exc}", file=sys.stderr)
            return 2
        if args.snapshot_jsonl:
            if args.header_only:
                playa_snapshot(args.pdf[0], args.pages, spaces[0], header_output=sys.stdout, password=args.password, sections=args.sections, header_only=True)
            else:
                write_snapshot_jsonl(args.pdf[0], args.pages, spaces[0], sys.stdout, password=args.password, sections=args.sections)
        else:
            assert snapshot is not None
            print(json.dumps(snapshot, ensure_ascii=False, separators=(",", ":")))
        return 0
    cache_dir = args.cache_dir if args.cache_dir is not None else repo_root / ".compat-cache"
    manifest = load_manifest(repo_root / "compat" / "manifest.toml")
    try:
        selected_sections = select_sections(manifest, args.sections)
    except ValueError as exc:
        parser.error(str(exc))
    upstream = load_upstream(repo_root / "compat" / "upstream.toml")
    try:
        require_upstream_version(upstream)
    except (RuntimeError, importlib.metadata.PackageNotFoundError) as exc:
        print(f"compatibility environment error: {exc}", file=sys.stderr)
        return 2
    pending = pending_sections(manifest)
    if pending:
        print(f"PENDING compatibility sections: {', '.join(pending)}", file=sys.stderr)
    if args.release and pending:
        print("release compatibility check requires no pending sections", file=sys.stderr)
        return 2
    failed = False
    for pdf_path in args.pdf:
        for space in spaces:
            if args.no_cache:
                expected_snapshot = playa_snapshot(pdf_path, args.pages, space, password=args.password, sections=selected_sections, include_objects="document.objects" in selected_sections, include_tokens="document.tokens" in selected_sections, include_mapping="document.mapping" in selected_sections)
                cache_hit = False
            else:
                expected_snapshot, cache_hit = cached_playa_snapshot(cache_dir, pdf_path, args.pages, space, upstream, args.password, selected_sections)
            expected = project_sections(expected_snapshot, selected_sections)
            go_command = args.go
            if args.password:
                go_command = f"{go_command} --password {shlex.quote(args.password)}"
            actual = project_sections(go_snapshot(repo_root, pdf_path, args.pages, space, go_command, selected_sections), selected_sections)
            differences: list[str] = []
            compare(expected, actual, "", args.tolerance, differences)
            label = f"{pdf_path} [{space}]"
            if differences:
                failed = True
                print(f"FAIL {label}", file=sys.stderr)
                print("\n".join(differences), file=sys.stderr)
            else:
                suffix = " (cache hit)" if cache_hit else ""
                print(f"PASS {label}{suffix}")
    return 1 if failed else 0


if __name__ == "__main__":
    raise SystemExit(main())
