#!/usr/bin/env python3
"""Generate normal-looking A4 PDFs containing isolated malformed objects."""

import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
OUT = ROOT / "testdata" / "files"


def stream(dictionary, data):
    return (b"<< " + dictionary + f" /Length {len(data)} >>\nstream\n".encode()
            + data + b"\nendstream")


def make_pdf(kind):
    objects = {}
    objects[1] = b"<< /Type /Catalog /Pages 2 0 R >>"
    objects[2] = b"<< /Type /Pages /Kids [3 0 R] /Count 1 >>"
    text = b"BT /F1 11 Tf 40 760 Td (Annual report excerpt: revenue grew steadily while operating costs declined.) Tj ET"
    resources = b"/Font << /F1 4 0 R >>"
    objects[5] = stream(b"", text)
    objects[4] = b"<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>"

    if kind == "metadata_filter":
        objects[1] = b"<< /Type /Catalog /Pages 2 0 R /Metadata 6 0 R >>"
        objects[6] = stream(b"/Type /Metadata /Subtype /XML /Filter /FlateDecode", b"<x:xmpmeta/>")

    if kind == "metadata_cycle":
        objects[1] = b"<< /Type /Catalog /Pages 2 0 R /Metadata 6 0 R >>"
        objects[6] = b"7 0 R"
        objects[7] = b"6 0 R"

    if kind == "metadata_encrypt_cycle":
        objects[1] = b"<< /Type /Catalog /Pages 2 0 R /Metadata 6 0 R >>"
        objects[6] = stream(b"/Type /Metadata /Subtype /XML", b"<x:xmpmeta/>")
        objects[8] = b"9 0 R"
        objects[9] = b"8 0 R"
        objects[10] = (b"<< /Filter /Standard /V 2 /R 3 /Length 128 "
                       b"/O (01234567890123456789012345678901) "
                       b"/U (01234567890123456789012345678901) /P -4 "
                       b"/EncryptMetadata 8 0 R >>")

    if kind == "metadata_encrypt_type":
        objects[1] = b"<< /Type /Catalog /Pages 2 0 R /Metadata 6 0 R >>"
        objects[6] = stream(b"/Type /Metadata /Subtype /XML", b"<x:xmpmeta/>")
        objects[8] = b"1"
        objects[10] = (b"<< /Filter /Standard /V 2 /R 3 /Length 128 "
                       b"/O (01234567890123456789012345678901) "
                       b"/U (01234567890123456789012345678901) /P -4 "
                       b"/EncryptMetadata 8 0 R >>")

    if kind in {"cff_index", "truetype_cmap", "cid_width", "tounicode", "cmap_range"}:
        text = b"BT /F1 11 Tf 40 760 Td <0001> Tj ET"
        objects[5] = stream(b"", text)
        objects[4] = b"<< /Type /Font /Subtype /Type0 /BaseFont /ResearchFont /Encoding /Identity-H /DescendantFonts [6 0 R]"
        if kind in {"tounicode", "cmap_range"}:
            objects[4] += b" /ToUnicode 7 0 R"
        objects[4] += b" >>"
        descendant = b"<< /Type /Font /Subtype /CIDFontType2 /BaseFont /ResearchFont /CIDSystemInfo << /Registry (Adobe) /Ordering (Identity) /Supplement 0 >> /DW 1000"
        if kind == "cid_width":
            descendant += b" /W [1 1152921504606846976 500]"
        if kind == "cff_index":
            descendant = descendant.replace(b"CIDFontType2", b"CIDFontType0") + b" /FontDescriptor 10 0 R"
        if kind == "truetype_cmap":
            descendant += b" /FontDescriptor 10 0 R"
        objects[6] = descendant + b" >>"
        if kind in {"tounicode", "cmap_range"}:
            cmap = b"/CIDInit /ProcSet findresource begin\n12 dict begin\nbegincmap\n1 begincodespacerange\n<0000> <ffff>\nendcodespacerange\n1 begincidrange\n<00000000> <ffffffff> 0\nendcidrange\nendcmap\nend\nend\n"
            objects[7] = stream(b"", cmap)
        if kind == "cff_index":
            cff = bytes([1, 0, 4, 4, 0, 1, 1, 4, 0, 1, 0xff, 0xff, 0xff, 0xff])
            objects[9] = stream(b"/Subtype /Type1C", cff)
            objects[10] = b"<< /Type /FontDescriptor /FontName /ResearchFont /FontFile3 9 0 R >>"
        if kind == "truetype_cmap":
            sfnt = bytearray(b"\x00\x01\x00\x00\x00\x01\x00\x00\x00\x00\x00\x00")
            sfnt += b"cmap" + b"\x00\x00\x00\x1c" + b"\xff\xff\xff\xff"
            objects[9] = stream(b"/Length1 28", bytes(sfnt))
            objects[10] = b"<< /Type /FontDescriptor /FontName /ResearchFont /FontFile2 9 0 R >>"
    elif kind == "image_geometry":
        resources = b"/Font << /F1 4 0 R >> /XObject << /Im1 8 0 R >>"
        objects[5] = stream(b"", text + b" q 1 0 0 1 40 500 cm /Im1 Do Q")
        objects[8] = stream(b"/Subtype /Image /Width 1152921504606846976 /Height 1152921504606846976 /BPC 4 /ColorSpace /DeviceGray", b"\x00")
    elif kind == "icc_components":
        resources = b"/Font << /F1 4 0 R >> /XObject << /Im1 8 0 R >>"
        objects[5] = stream(b"", text + b" q 1 0 0 1 40 500 cm /Im1 Do Q")
        profile = bytearray(20)
        profile[16:20] = b"RGB "
        objects[9] = stream(b"/N 1152921504606846976", bytes(profile))
        objects[8] = stream(b"/Subtype /Image /Width 1 /Height 1 /BPC 8 /ColorSpace [/ICCBased 9 0 R]", b"\x00")
    elif kind == "stream_length":
        objects[5] = b"<< /Length 1152921504606846976 >>\nstream\n" + text + b"\nendstream"
    elif kind == "object_stream":
        objects[9] = stream(b"/N 1 /First 4", b"1 1152921504606846976 <<>>")

    objects[3] = (b"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 595 842] "
                  + b"/Resources << " + resources + b" >> /Contents 5 0 R >>")

    body = bytearray(b"%PDF-1.4\n")
    max_number = max(objects)
    offsets = [0] * (max_number + 1)
    for number in sorted(objects):
        offsets[number] = len(body)
        body += f"{number} 0 obj\n".encode() + objects[number] + b"\nendobj\n"
    xref = len(body)
    body += f"xref\n0 {len(offsets)}\n".encode()
    body += b"0000000000 65535 f \n"
    for offset in offsets[1:]:
        if offset == 0:
            body += b"0000000000 00000 f \n"
        else:
            body += f"{offset:010d} 00000 n \n".encode()
    trailer = f"/Size {len(offsets)} /Root 1 0 R"
    if kind in {"metadata_encrypt_cycle", "metadata_encrypt_type"}:
        trailer += " /Encrypt 10 0 R /ID [(file-id)]"
    body += f"trailer\n<< {trailer} >>\nstartxref\n{xref}\n%%EOF\n".encode()
    return bytes(body)


names = ("stream_length", "object_stream", "cff_index", "image_geometry",
         "icc_components", "truetype_cmap", "cid_width", "tounicode",
         "cmap_range", "metadata_filter", "metadata_cycle",
         "metadata_encrypt_cycle", "metadata_encrypt_type")
arguments = sys.argv[1:]
check = "--check" in arguments
selected = [argument for argument in arguments if argument != "--check"]
if not selected:
    selected = list(names)
unknown = sorted(set(selected) - set(names))
if unknown:
    raise SystemExit(f"unknown security fixture(s): {', '.join(unknown)}")

for name in selected:
    output_name = "security_object_stream_offset_overflow" if name == "object_stream" else f"security_{name}_overflow"
    path = OUT / f"{output_name}.pdf"
    data = make_pdf(name)
    if check:
        if not path.exists() or path.read_bytes() != data:
            raise SystemExit(f"security fixture is out of date: {path}")
    else:
        path.write_bytes(data)
