from __future__ import annotations

import hashlib
import argparse
from functools import partial
import json
import math
import os
import struct
import sys
import time
try:
    import resource
except ImportError:  # pragma: no cover - unavailable on Windows
    resource = None  # type: ignore[assignment]

import playa

# Match the Go projection while absorbing OCR geometry differences introduced
# by independent floating-point implementations.
BENCHMARK_FLOAT_DIGITS = 2
BENCHMARK_FLOAT_TIE_EPSILON = 1e-9


def peak_rss_bytes() -> int:
    """Return this process's lifetime peak RSS using the Go runner's units."""
    if resource is None:
        return 0
    value = int(resource.getrusage(resource.RUSAGE_SELF).ru_maxrss)
    if value <= 0:
        return 0
    if sys.platform.startswith("linux"):
        return value * 1024
    return value


def process_memory_sample() -> tuple[int, int]:
    return os.getpid(), peak_rss_bytes()


def record_process_peak(process_peaks: dict[int, int], pid: int, rss: int) -> None:
    process_peaks[pid] = max(process_peaks.get(pid, 0), rss)


def completed_process_measurement(process_peaks: dict[int, int], selected_workers: int = 1) -> dict[str, object]:
    """Sample the parent after close; worker callback samples remain lower bounds.

    The public page-map interface returns before worker IPC serialization and
    exposes no per-worker exit rusage. Never publish those incomplete samples as
    complete process peaks or use them for cross-library memory comparisons.
    """
    parent_pid, parent_rss = process_memory_sample()
    worker_peaks = {pid: rss for pid, rss in process_peaks.items() if pid != parent_pid}
    topology = {
        "effective_workers": selected_workers if worker_peaks else 1,
        "observed_workers": max(1, len(process_peaks)),
    }
    if not worker_peaks:
        return {**topology, "peak_rss_bytes": parent_rss, "rss_scope": "single-process-lifetime-peak"}
    return {
        **topology,
        "peak_rss_bytes": 0,
        "rss_scope": "parent-lifetime-and-worker-callback-lower-bound",
        "parent_peak_rss_bytes": parent_rss,
        "callback_worker_peak_rss_sum_bytes": sum(worker_peaks.values()),
    }


def canonical_json(value: object) -> str:
    """Serialize benchmark projections with the Go runner's float contract."""
    if isinstance(value, bool) or value is None:
        return json.dumps(value, separators=(",", ":"))
    if isinstance(value, int):
        return str(value)
    if isinstance(value, float):
        if not math.isfinite(value):
            raise ValueError(f"non-finite benchmark float: {value}")
        if abs(value) < 0.5 * 10 ** -BENCHMARK_FLOAT_DIGITS:
            value = 0.0
        scale = 10 ** BENCHMARK_FLOAT_DIGITS
        magnitude = math.floor(
            abs(value) * scale + 0.5 + BENCHMARK_FLOAT_TIE_EPSILON
        )
        value = math.copysign(magnitude / scale, value)
        return f"{value:.{BENCHMARK_FLOAT_DIGITS}f}"
    if isinstance(value, str):
        return json.dumps(value, ensure_ascii=False, separators=(",", ":"))
    if isinstance(value, list):
        return "[" + ",".join(canonical_json(item) for item in value) + "]"
    if isinstance(value, dict):
        return "{" + ",".join(
            canonical_json(str(key)) + ":" + canonical_json(item)
            for key, item in value.items()
        ) + "}"
    raise TypeError(f"unsupported benchmark JSON value: {type(value)!r}")


def project_page(page: object) -> tuple[bytes, int, int, int, int]:
    page_output = {"page": page.page_idx, "texts": []}  # type: ignore[attr-defined]
    texts = glyphs = 0
    for text in page.texts:  # type: ignore[attr-defined]
        glyph_output = [
            {
                "text": glyph.text or "",
                "origin": list(glyph.origin),
                "displacement": list(glyph.displacement),
                "bbox": list(glyph.bbox),
            }
            for glyph in text
        ]
        page_output["texts"].append(
            {
                "text": text.chars,
                "bbox": list(text.bbox),
                "glyphs": glyph_output,
            }
        )
        texts += 1
        glyphs += len(glyph_output)
    encoded = canonical_json(page_output).encode()
    pid, rss = process_memory_sample()
    return encoded, texts, glyphs, pid, rss


def update_page_projection_digest(digest: object, data: bytes) -> None:
    """Hash one canonical page object with no inter-record delimiter."""
    digest.update(data)  # type: ignore[attr-defined]


def count_page(page: object, task: str) -> tuple[int, int, int, int, int, int, int, int, int, int, bytes]:
    texts = glyphs = layout_lines = layout_items = images = 0
    image_frames = bytearray()
    if task == "text-glyphs":
        for text in page.texts:  # type: ignore[attr-defined]
            texts += 1
            glyphs += sum(1 for _ in text)
    elif task == "layout-items":
        import playa.miner as miner

        analyzed = miner.extract_page(page, miner.LAParams())
        seen: set[int] = set()

        def collect_lines(value: object) -> None:
            nonlocal layout_lines
            if isinstance(value, miner.LTTextLine):
                identity = id(value)
                if identity not in seen:
                    seen.add(identity)
                    layout_lines += 1
                return
            if isinstance(value, (miner.LTTextBox, miner.LTTextGroup)):
                for child in value:
                    collect_lines(child)

        for item in analyzed:
            collect_lines(item)
            if isinstance(item, (miner.LTTextBox, miner.LTFigure, miner.LTCurve, miner.LTImage)):
                layout_items += 1
    elif task == "image-digests":
        for image_index, image in enumerate(page.images):  # type: ignore[attr-defined]
            data = image.buffer
            image_frames.extend(struct.pack('>QQQ', page.page_idx, image_index, len(data)))  # type: ignore[attr-defined]
            image_frames.extend(hashlib.sha256(data).digest())
            images += 1
    encoded_frames = bytes(image_frames)
    pid, rss = process_memory_sample()
    return 1, texts, glyphs, layout_lines, layout_items, images, 0, pid, rss, page.page_idx, encoded_frames  # type: ignore[attr-defined]


def count_object_task(path: str, workers: int = 1) -> dict[str, object]:
    start = time.perf_counter_ns()
    with playa.open(path, space="page", max_workers=1) as doc:
        objects = sum(1 for _ in doc.objects)
        pages = len(doc.pages)
    elapsed = time.perf_counter_ns() - start
    return {
        "library": "playa",
        "pdf": path,
        "task": "objects",
        "requested_workers": workers,
        "pages": pages,
        "texts": 0,
        "glyphs": 0,
        "objects": objects,
        "elapsed_ns": elapsed,
        **completed_process_measurement({}),
    }


def normalize_workers(requested: int) -> int:
    if requested <= 0:
        requested = os.cpu_count() or 1
    cpu_limit = os.cpu_count() or 1
    return max(1, min(requested, cpu_limit))


def run_text_glyph_jsonl(paths: list[str], workers: int) -> None:
    for path in paths:
        digest = hashlib.sha256()
        pages = texts = glyphs = 0
        selected_workers = normalize_workers(workers)
        process_peaks: dict[int, int] = {}
        start = time.perf_counter_ns()
        with playa.open(path, space="page", max_workers=selected_workers) as doc:
            for data, page_texts, page_glyphs, pid, rss in doc.pages.map(project_page):
                update_page_projection_digest(digest, data)
                texts += page_texts
                glyphs += page_glyphs
                pages += 1
                record_process_peak(process_peaks, pid, rss)
        elapsed = time.perf_counter_ns() - start
        result = {
            "library": "playa",
            "pdf": path,
            "task": "text-glyph-jsonl",
            "requested_workers": workers,
            "pages": pages,
            "texts": texts,
            "glyphs": glyphs,
            "elapsed_ns": elapsed,
            **completed_process_measurement(process_peaks, selected_workers),
            "sha256": digest.hexdigest(),
        }
        print(json.dumps(result, separators=(",", ":")))


def run_count_task(paths: list[str], workers: int, task: str) -> None:
    for path in paths:
        if task == "objects":
            print(json.dumps(count_object_task(path, workers), separators=(",", ":")))
            continue
        pages = texts = glyphs = layout_lines = layout_items = images = objects = 0
        selected_workers = normalize_workers(workers)
        process_peaks: dict[int, int] = {}
        image_pages: dict[int, bytes] = {}
        image_digest = hashlib.sha256(b'playa-image-digests-v1\0') if task == 'image-digests' else None
        start = time.perf_counter_ns()
        with playa.open(path, space="page", max_workers=selected_workers) as doc:
            counter = partial(count_page, task=task)
            for page_count, page_texts, page_glyphs, page_layout_lines, page_layout_items, page_images, page_objects, pid, rss, page_index, image_frames in doc.pages.map(counter):
                pages += page_count
                texts += page_texts
                glyphs += page_glyphs
                layout_lines += page_layout_lines
                layout_items += page_layout_items
                images += page_images
                objects += page_objects
                record_process_peak(process_peaks, pid, rss)
                if image_digest is not None:
                    if page_index in image_pages or page_index < 0:
                        raise ValueError('invalid or duplicate image page index')
                    image_pages[page_index] = image_frames
            if image_digest is not None:
                for page_index in range(pages):
                    if page_index not in image_pages:
                        raise ValueError('image page result gap')
                    image_digest.update(image_pages[page_index])
        elapsed = time.perf_counter_ns() - start
        result = {
            "library": "playa",
            "pdf": path,
            "task": task,
            "requested_workers": workers,
            "pages": pages,
            "texts": texts,
            "glyphs": glyphs,
            "layout_lines": layout_lines,
            "layout_items": layout_items,
            "images": images,
            "objects": objects,
            "elapsed_ns": elapsed,
            **completed_process_measurement(process_peaks, selected_workers),
        }
        if image_digest is not None:
            result['sha256'] = image_digest.hexdigest()
        print(json.dumps(result, separators=(",", ":")))


if __name__ == "__main__":
    parser = argparse.ArgumentParser()
    parser.add_argument("--task", default="text-glyph-jsonl", choices=["open-pages", "text-glyphs", "text-glyph-jsonl", "layout-items", "image-digests", "objects"])
    parser.add_argument("--workers", type=int, default=1)
    parser.add_argument("--jsonl", action="store_true")
    parser.add_argument("paths", nargs="+")
    args = parser.parse_args()
    if args.task == "text-glyph-jsonl":
        run_text_glyph_jsonl(args.paths, args.workers)
    elif args.task in {"open-pages", "text-glyphs", "layout-items", "image-digests", "objects"}:
        run_count_task(args.paths, args.workers, args.task)
