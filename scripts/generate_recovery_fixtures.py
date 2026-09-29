#!/usr/bin/env python3
"""Generate small A4 PDFs covering common xref and object-stream layouts."""

import sys
from pathlib import Path


ROOT = Path(__file__).resolve().parents[1]
OUT = ROOT / "testdata" / "files"


def classic_xref(objects, root=1, previous=None):
    data = bytearray(b"%PDF-1.4\n")
    offsets = {}
    for number, body in sorted(objects.items()):
        offsets[number] = len(data)
        data += f"{number} 0 obj\n".encode() + body + b"\nendobj\n"
    xref = len(data)
    maximum = max(objects) + 1
    data += f"xref\n0 {maximum}\n0000000000 65535 f \n".encode()
    for number in range(1, maximum):
        if number in offsets:
            data += f"{offsets[number]:010d} 00000 n \n".encode()
        else:
            data += b"0000000000 00000 f \n"
    trailer = f"/Size {maximum} /Root {root} 0 R"
    if previous is not None:
        trailer += f" /Prev {previous}"
    data += f"trailer\n<< {trailer} >>\nstartxref\n{xref}\n%%EOF\n".encode()
    return data, xref


def xref_record(kind, value, generation=0):
    return bytes((kind, (value >> 24) & 0xFF, (value >> 16) & 0xFF,
                  (value >> 8) & 0xFF, value & 0xFF,
                  (generation >> 8) & 0xFF, generation & 0xFF))


def mixed_xref(stream_first):
    objects = {
        1: b"<< /Type /Catalog /Pages 2 0 R >>",
        2: b"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
        3: b"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 595 842] >>",
    }
    if not stream_first:
        data, previous = classic_xref(objects)
        page = len(data)
        data += b"3 0 obj\n<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] >>\nendobj\n"
        stream = len(data)
        raw = xref_record(1, page) + xref_record(1, stream)
        data += (f"4 0 obj\n<< /Type /XRef /Size 5 /Root 1 0 R /Prev {previous} "
                 f"/Index [3 2] /W [1 4 2] /Length {len(raw)} >>\nstream\n").encode()
        data += raw + f"\nendstream\nendobj\nstartxref\n{stream}\n%%EOF\n".encode()
        return data

    # Rebuild the base with a stream xref so the appended revision starts from it.
    data = bytearray(b"%PDF-1.5\n")
    offsets = {}
    for number, body in sorted(objects.items()):
        offsets[number] = len(data)
        data += f"{number} 0 obj\n".encode() + body + b"\nendobj\n"
    stream = len(data)
    raw = (xref_record(0, 0, 65535) + xref_record(1, offsets[1]) +
           xref_record(1, offsets[2]) + xref_record(1, offsets[3]) +
           xref_record(1, stream))
    data += (f"4 0 obj\n<< /Type /XRef /Size 5 /Root 1 0 R /W [1 4 2] "
             f"/Length {len(raw)} >>\nstream\n").encode()
    data += raw + f"\nendstream\nendobj\nstartxref\n{stream}\n%%EOF\n".encode()
    page = len(data)
    data += b"3 0 obj\n<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] >>\nendobj\n"
    xref = len(data)
    data += f"xref\n3 1\n{page:010d} 00000 n \ntrailer\n<< /Size 5 /Root 1 0 R /Prev {stream} >>\nstartxref\n{xref}\n%%EOF\n".encode()
    return data


def object_stream_pdf():
    data = bytearray(b"%PDF-1.5\n")
    objects = {
        1: b"<< /Type /Catalog /Pages 2 0 R >>",
        2: b"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
    }
    offsets = {}
    for number, body in sorted(objects.items()):
        offsets[number] = len(data)
        data += f"{number} 0 obj\n".encode() + body + b"\nendobj\n"
    page_body = b"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 595 842] >>"
    object_data = b"3 0 " + page_body
    stream = len(data)
    data += (f"4 0 obj\n<< /Type /ObjStm /N 1 /First 4 /Length {len(object_data)} >>\n"
             "stream\n").encode()
    data += object_data + b"\nendstream\nendobj\n"
    xref = len(data)
    raw = (xref_record(0, 0, 65535) + xref_record(1, offsets[1]) +
           xref_record(1, offsets[2]) + xref_record(2, 4, 0) +
           xref_record(1, stream) + xref_record(1, xref))
    data += (f"5 0 obj\n<< /Type /XRef /Size 6 /Root 1 0 R /W [1 4 2] "
             f"/Length {len(raw)} >>\nstream\n").encode()
    data += raw + f"\nendstream\nendobj\nstartxref\n{xref}\n%%EOF\n".encode()
    return data


def malformed_xref_stream_pdf():
    data = bytearray(b"%PDF-1.5\n")
    objects = {
        1: b"<< /Type /Catalog /Pages 2 0 R >>",
        2: b"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
        3: b"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 595 842] >>",
    }
    offsets = {}
    for number, body in sorted(objects.items()):
        offsets[number] = len(data)
        data += f"{number} 0 obj\n".encode() + body + b"\nendobj\n"
    xref = len(data)
    raw = (xref_record(0, 0, 65535) + xref_record(1, offsets[1]) +
           xref_record(1, offsets[2]) + xref_record(1, offsets[3]) +
           xref_record(1, xref) + b"\x00")
    data += (f"4 0 obj\n<< /Type /XRef /Size 5 /Root 1 0 R /W [1 4 2] "
             f"/Length {len(raw)} >>\nstream\n").encode()
    data += raw + f"\nendstream\nendobj\nstartxref\n{xref}\n%%EOF\n".encode()
    return data


def malformed_object_stream_pdf():
    return object_stream_pdf().replace(b"/First 4", b"/First 99", 1)


def damaged_scan_pdf():
    return (b"%PDF-1.4\n"
            b"1 0 obj\n<< /Type /Catalog /Pages 2 0 R >>\nendobj\n"
            b"2 0 obj\n<< /Type /Pages /Kids [3 0 R] /Count 1 >>\nendobj\n"
            b"3 0 obj\n<< /Type /Page /Parent 2 0 R /MediaBox [0 0 595 842] >>\n"
            b"endobj\ntrailing damaged data\n")


def damaged_text_pdf():
    content = b"BT /F1 12 Tf 72 720 Td (Recovered text) Tj ET"
    return (b"%PDF-1.4\n"
            b"1 0 obj\n<< /Type /Catalog /Pages 2 0 R >>\nendobj\n"
            b"2 0 obj\n<< /Type /Pages /Kids [3 0 R] /Count 1 >>\nendobj\n"
            b"3 0 obj\n<< /Type /Page /Parent 2 0 R /MediaBox [0 0 595 842] "
            b"/Resources << /Font << /F1 5 0 R >> >> /Contents 4 0 R >>\nendobj\n"
            + f"4 0 obj\n<< /Length {len(content)} >>\nstream\n".encode()
            + content + b"\nendstream\nendobj\n"
            b"5 0 obj\n<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>\nendobj\n"
            b"trailer\n<< /Size 6 /Root 1 0 R >>\n"
            b"startxref\nnot-an-offset\n%%EOF\n")


def damaged_object_stream_pdf():
    page = b"3 0 << /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] >>"
    return (b"%PDF-1.5\n"
            b"1 0 obj\n<< /Type /Catalog /Pages 2 0 R >>\nendobj\n"
            b"2 0 obj\n<< /Type /Pages /Kids [3 0 R] /Count 1 >>\nendobj\n"
            + f"4 0 obj\n<< /Type /ObjStm /N 1 /First 4 /Length {len(page)} >>\n"
            "stream\n".encode()
            + page + b"\nendstream\nendobj\n"
            b"trailer\n<< /Size 5 /Root 1 0 R >>\n"
            b"startxref\nnot-an-offset\n%%EOF\n")


def incremental_object_stream_pdf():
    objects = {
        1: b"<< /Type /Catalog /Pages 2 0 R >>",
        2: b"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
        3: b"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 595 842] >>",
    }
    data, previous = classic_xref(objects)
    page = b"3 0 << /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] >>"
    object_stream = len(data)
    data += (f"4 0 obj\n<< /Type /ObjStm /N 1 /First 4 /Length {len(page)} >>\n"
             "stream\n").encode()
    data += page + b"\nendstream\nendobj\n"
    xref_stream = len(data)
    raw = (xref_record(2, 4, 0) + xref_record(1, object_stream) +
           xref_record(1, xref_stream))
    data += (f"5 0 obj\n<< /Type /XRef /Size 6 /Root 1 0 R /Prev {previous} "
             f"/Index [3 3] /W [1 4 2] /Length {len(raw)} >>\nstream\n").encode()
    data += raw + f"\nendstream\nendobj\nstartxref\n{xref_stream}\n%%EOF\n".encode()
    return data


def main():
    OUT.mkdir(parents=True, exist_ok=True)
    outputs = {
        "recovery_classic_to_xref_stream.pdf": mixed_xref(False),
        "recovery_xref_stream_to_classic.pdf": mixed_xref(True),
        "recovery_object_stream.pdf": object_stream_pdf(),
        "recovery_malformed_xref_stream.pdf": malformed_xref_stream_pdf(),
        "recovery_malformed_object_stream.pdf": malformed_object_stream_pdf(),
        "recovery_damaged_scan.pdf": damaged_scan_pdf(),
        "recovery_damaged_text.pdf": damaged_text_pdf(),
        "recovery_damaged_object_stream.pdf": damaged_object_stream_pdf(),
        "recovery_incremental_object_stream.pdf": incremental_object_stream_pdf(),
    }
    check = "--check" in sys.argv[1:]
    for name, data in outputs.items():
        path = OUT / name
        if check:
            if not path.exists() or path.read_bytes() != data:
                raise SystemExit(f"recovery fixture is out of date: {path}")
        else:
            path.write_bytes(data)


if __name__ == "__main__":
    main()
