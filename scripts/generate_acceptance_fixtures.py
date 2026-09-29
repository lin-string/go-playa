#!/usr/bin/env python3
"""Generate small, readable PDFs for cross-feature acceptance checks."""

import hashlib
import struct
import sys
from pathlib import Path


ROOT = Path(__file__).resolve().parents[1]
OUT = ROOT / "testdata" / "files"


def stream(dictionary, data):
    return (b"<< " + dictionary + f" /Length {len(data)} >>\nstream\n".encode()
            + data + b"\nendstream")


PASSWORD_PADDING = bytes((
    0x28, 0xbf, 0x4e, 0x5e, 0x4e, 0x75, 0x8a, 0x41,
    0x64, 0x00, 0x4e, 0x56, 0xff, 0xfa, 0x01, 0x08,
    0x2e, 0x2e, 0x00, 0xb6, 0xd0, 0x68, 0x3e, 0x80,
    0x2f, 0x0c, 0xa9, 0xfe, 0x64, 0x53, 0x69, 0x7a,
))


def rc4(key, data):
    state = list(range(256))
    j = 0
    for i in range(256):
        j = (j + state[i] + key[i % len(key)]) & 255
        state[i], state[j] = state[j], state[i]
    out = bytearray()
    i = j = 0
    for value in data:
        i = (i + 1) & 255
        j = (j + state[i]) & 255
        state[i], state[j] = state[j], state[i]
        out.append(value ^ state[(state[i] + state[j]) & 255])
    return bytes(out)


def padded_password(value):
    return (value + PASSWORD_PADDING)[:32]


def vertical_text_pdf():
    objects = {
        1: b"<< /Type /Catalog /Pages 2 0 R >>",
        2: b"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
        3: (b"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 595 842] "
            b"/Resources << /Font << /F1 4 0 R >> >> /Contents 5 0 R >>"),
        4: (b"<< /Type /Font /Subtype /Type0 /BaseFont /ResearchVertical "
            b"/Encoding /Identity-V /DescendantFonts [6 0 R] /ToUnicode 7 0 R >>"),
        6: (b"<< /Type /Font /Subtype /CIDFontType2 /BaseFont /ResearchVertical "
            b"/CIDSystemInfo << /Registry (Adobe) /Ordering (Identity) /Supplement 0 >> "
            b"/DW 1000 /FontDescriptor 8 0 R >>"),
        8: (b"<< /Type /FontDescriptor /FontName /ResearchVertical /Flags 4 "
            b"/FontBBox [0 -200 1000 900] /ItalicAngle 0 /Ascent 880 "
            b"/Descent -120 /CapHeight 700 /StemV 80 >>"),
        7: stream(
            b"/Type /CMap /Subtype /CMap",
            b"/CIDInit /ProcSet findresource begin\n"
            b"12 dict begin\nbegincmap\n"
            b"/CIDSystemInfo << /Registry (Adobe) /Ordering (UCS) /Supplement 0 >> def\n"
            b"/CMapName /Adobe-Identity-UCS def\n/CMapType 2 def\n"
            b"1 begincodespacerange\n<0000> <ffff>\nendcodespacerange\n"
            b"2 beginbfchar\n<0001> <0041>\n<0002> <0042>\nendbfchar\n"
            b"endcmap\nend\nend\n",
        ),
        5: stream(
            b"",
            b"BT /F1 18 Tf 72 740 Td <00010002> Tj ET",
        ),
    }
    data = bytearray(b"%PDF-1.5\n")
    offsets = {}
    for number in sorted(objects):
        offsets[number] = len(data)
        data += f"{number} 0 obj\n".encode() + objects[number] + b"\nendobj\n"
    xref = len(data)
    maximum = max(objects) + 1
    data += f"xref\n0 {maximum}\n0000000000 65535 f \n".encode()
    for number in range(1, maximum):
        if number in offsets:
            data += f"{offsets[number]:010d} 00000 n \n".encode()
        else:
            data += b"0000000000 00000 f \n"
    data += (f"trailer\n<< /Size {maximum} /Root 1 0 R >>\nstartxref\n{xref}\n"
             "%%EOF\n").encode()
    return data


def adjacent_glyphs_pdf():
    """Exercise neighboring OCR-style text runs without duplicate glyphs."""
    return write_pdf({
        1: b"<< /Type /Catalog /Pages 2 0 R >>",
        2: b"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
        3: (b"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 595 842] "
            b"/Resources << /Font << /F1 4 0 R >> >> /Contents 5 0 R >>"),
        4: b"<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>",
        5: stream(b"", b"BT /F1 10 Tf 72 740 Td (A) Tj (B) Tj 0 -20 Td (C) Tj ET"),
    })


def macexpert_encoding_pdf():
    """Exercise every non-control MacExpert slot with a synthetic Type 1 font."""
    content = bytearray(b"BT /F1 10 Tf 14 TL 72 760 Td\n")
    for start in range(32, 256, 16):
        content += b"<" + bytes(range(start, start + 16)).hex().encode() + b"> Tj T*\n"
    content += b"ET"
    return write_pdf({
        1: b"<< /Type /Catalog /Pages 2 0 R >>",
        2: b"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
        3: (b"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 595 842] "
            b"/Resources << /Font << /F1 4 0 R >> >> /Contents 5 0 R >>"),
        4: (b"<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica "
            b"/Encoding /MacExpertEncoding >>"),
        5: stream(b"", bytes(content)),
    })


def tagged_text_pdf():
    objects = {
        1: (b"<< /Type /Catalog /Pages 2 0 R /StructTreeRoot 10 0 R "
            b"/MarkInfo << /Marked true >> >>"),
        2: b"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
        3: (b"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 595 842] "
            b"/StructParents 0 /Resources << /Font << /F1 4 0 R >> >> "
            b"/Contents 5 0 R >>"),
        4: b"<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>",
        5: stream(b"", b"/P << /MCID 0 >> BDC BT /F1 18 Tf 72 740 Td (AB) Tj ET EMC"),
        10: (b"<< /Type /StructTreeRoot /K 11 0 R /ParentTree 12 0 R >>"),
        11: (b"<< /Type /StructElem /S /P /P 10 0 R /Pg 3 0 R "
             b"/K 13 0 R /T (Financial summary) >>"),
        12: b"<< /Nums [0 [11 0 R]] >>",
        13: b"<< /Type /MCR /Pg 3 0 R /MCID 0 >>",
    }
    data = bytearray(b"%PDF-1.5\n")
    offsets = {}
    for number in sorted(objects):
        offsets[number] = len(data)
        data += f"{number} 0 obj\n".encode() + objects[number] + b"\nendobj\n"
    xref = len(data)
    maximum = max(objects) + 1
    data += f"xref\n0 {maximum}\n0000000000 65535 f \n".encode()
    for number in range(1, maximum):
        if number in offsets:
            data += f"{offsets[number]:010d} 00000 n \n".encode()
        else:
            data += b"0000000000 00000 f \n"
    data += (f"trailer\n<< /Size {maximum} /Root 1 0 R >>\nstartxref\n{xref}\n"
             "%%EOF\n").encode()
    return data


def image_pdf():
    objects = {
        1: b"<< /Type /Catalog /Pages 2 0 R >>",
        2: b"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
        3: (b"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 595 842] "
            b"/Resources << /XObject << /Im1 4 0 R >> >> /Contents 5 0 R >>"),
        4: stream(
            b"/Type /XObject /Subtype /Image /Width 2 /Height 1 "
            b"/ColorSpace /DeviceRGB /BitsPerComponent 8",
            bytes((255, 0, 0, 0, 255, 0)),
        ),
        5: stream(b"", b"q 200 0 0 100 72 600 cm /Im1 Do Q"),
    }
    data = bytearray(b"%PDF-1.5\n")
    offsets = {}
    for number in sorted(objects):
        offsets[number] = len(data)
        data += f"{number} 0 obj\n".encode() + objects[number] + b"\nendobj\n"
    xref = len(data)
    maximum = max(objects) + 1
    data += f"xref\n0 {maximum}\n0000000000 65535 f \n".encode()
    for number in range(1, maximum):
        if number in offsets:
            data += f"{offsets[number]:010d} 00000 n \n".encode()
        else:
            data += b"0000000000 00000 f \n"
    data += (f"trailer\n<< /Size {maximum} /Root 1 0 R >>\nstartxref\n{xref}\n"
             "%%EOF\n").encode()
    return data


def inline_image_pdf():
    content = (
        b"BT /F1 12 Tf 72 760 Td (Quarterly report image appendix) Tj ET\n"
        b"q 24 0 0 24 72 700 cm BI /W 1 /H 1 /BPC 8 /CS /G /F /AHx ID FF>EI Q\n"
        b"q 24 0 0 24 120 700 cm BI /W 1 /H 1 /BPC 8 /CS /G /F /A85 ID z~>EI Q"
    )
    return write_pdf({
        1: b"<< /Type /Catalog /Pages 2 0 R >>",
        2: b"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
        3: (b"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 595 842] "
            b"/Resources << /Font << /F1 4 0 R >> >> /Contents 5 0 R >>"),
        4: b"<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>",
        5: stream(b"", content),
    })


def path_subpaths_pdf():
    content = (
        b"BT /F1 12 Tf 72 740 Td (Quarterly report figures) Tj ET\n"
        b"q 1 0 0 1 72 600 cm "
        b"0 0 100 30 re 150 0 100 30 re 300 0 20 20 re S Q"
    )
    return write_pdf({
        1: b"<< /Type /Catalog /Pages 2 0 R >>",
        2: b"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
        3: (b"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 595 842] "
            b"/Resources << /Font << /F1 4 0 R >> >> /Contents 5 0 R >>"),
        4: b"<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>",
        5: stream(b"", content),
    })


def form_pdf():
    objects = {
        1: b"<< /Type /Catalog /Pages 2 0 R /AcroForm 8 0 R >>",
        2: b"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
        3: (b"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 595 842] "
            b"/Annots [7 0 R] /Resources << /Font << /F1 4 0 R >> >> "
            b"/Contents 5 0 R >>"),
        4: b"<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>",
        5: stream(b"", b"BT /F1 12 Tf 72 740 Td (Annual report selection) Tj ET"),
        6: (b"<< /T (report_type) /FT /Ch /Opt [(Annual) (Quarterly)] "
            b"/V (Annual) /Kids [7 0 R] >>"),
        7: (b"<< /Type /Annot /Subtype /Widget /Parent 6 0 R /P 3 0 R "
            b"/Rect [72 680 220 710] /F 4 >>"),
        8: b"<< /Fields [6 0 R] /NeedAppearances true >>",
    }
    data = bytearray(b"%PDF-1.5\n")
    offsets = {}
    for number in sorted(objects):
        offsets[number] = len(data)
        data += f"{number} 0 obj\n".encode() + objects[number] + b"\nendobj\n"
    xref = len(data)
    maximum = max(objects) + 1
    data += f"xref\n0 {maximum}\n0000000000 65535 f \n".encode()
    for number in range(1, maximum):
        if number in offsets:
            data += f"{offsets[number]:010d} 00000 n \n".encode()
        else:
            data += b"0000000000 00000 f \n"
    data += (f"trailer\n<< /Size {maximum} /Root 1 0 R >>\nstartxref\n{xref}\n"
             "%%EOF\n").encode()
    return data


def encrypted_r2_pdf():
    user_password = padded_password(b"secret")
    owner_password = padded_password(b"owner")
    owner_key = hashlib.md5(owner_password).digest()[:5]
    owner_entry = rc4(owner_key, user_password)
    permissions = -4
    file_id = b"acceptance-r2-id"
    file_key = hashlib.md5(
        user_password + owner_entry + struct.pack("<i", permissions) + file_id
    ).digest()[:5]
    # Revision 2 stores exactly the 32-byte RC4 result in /U. The extra
    # 16-byte suffix belongs to the longer revision 3/4 representation.
    user_entry = rc4(file_key, PASSWORD_PADDING)

    def object_key(number):
        seed = file_key + struct.pack("<I", number)[:3] + b"\x00\x00"
        return hashlib.md5(seed).digest()[:10]

    content = b"BT /F1 12 Tf 72 740 Td (Encrypted report) Tj ET"
    encrypted_content = rc4(object_key(5), content)
    objects = {
        1: b"<< /Type /Catalog /Pages 2 0 R >>",
        2: b"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
        3: (b"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 595 842] "
            b"/Resources << /Font << /F1 4 0 R >> >> /Contents 5 0 R >>"),
        4: b"<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>",
        5: stream(b"", encrypted_content),
        9: (b"<< /Filter /Standard /V 1 /R 2 /O <" + owner_entry.hex().encode()
            + b"> /U <" + user_entry.hex().encode() + b"> /P -4 >>"),
    }
    data = bytearray(b"%PDF-1.4\n")
    offsets = {}
    for number in sorted(objects):
        offsets[number] = len(data)
        data += f"{number} 0 obj\n".encode() + objects[number] + b"\nendobj\n"
    xref = len(data)
    maximum = max(objects) + 1
    data += f"xref\n0 {maximum}\n0000000000 65535 f \n".encode()
    for number in range(1, maximum):
        if number in offsets:
            data += f"{offsets[number]:010d} 00000 n \n".encode()
        else:
            data += b"0000000000 00000 f \n"
    data += (f"trailer\n<< /Size {maximum} /Root 1 0 R /Encrypt 9 0 R "
             f"/ID [<{file_id.hex()}> <{file_id.hex()}>] >>\n"
             f"startxref\n{xref}\n%%EOF\n").encode()
    return data


def cjk_text_pdf():
    objects = {
        1: b"<< /Type /Catalog /Pages 2 0 R >>",
        2: b"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
        3: (b"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 595 842] "
            b"/Resources << /Font << /F1 4 0 R >> >> /Contents 5 0 R >>"),
        4: (b"<< /Type /Font /Subtype /Type0 /BaseFont /ResearchCJK "
            b"/Encoding /Identity-H /DescendantFonts [6 0 R] /ToUnicode 7 0 R >>"),
        5: stream(b"", b"BT /F1 18 Tf 72 740 Td <00010002> Tj ET"),
        6: (b"<< /Type /Font /Subtype /CIDFontType2 /BaseFont /ResearchCJK "
            b"/CIDSystemInfo << /Registry (Adobe) /Ordering (Identity) /Supplement 0 >> "
            b"/DW 1000 /FontDescriptor 8 0 R >>"),
        7: stream(
            b"/Type /CMap /Subtype /CMap",
            b"/CIDInit /ProcSet findresource begin\n"
            b"12 dict begin\nbegincmap\n"
            b"/CIDSystemInfo << /Registry (Adobe) /Ordering (UCS) /Supplement 0 >> def\n"
            b"/CMapName /Adobe-Identity-UCS def\n/CMapType 2 def\n"
            b"1 begincodespacerange\n<0000> <ffff>\nendcodespacerange\n"
            b"2 beginbfchar\n<0001> <4e2d>\n<0002> <56fd>\nendbfchar\n"
            b"endcmap\nend\nend\n",
        ),
        8: (b"<< /Type /FontDescriptor /FontName /ResearchCJK /Flags 4 "
            b"/FontBBox [0 -200 1000 900] /ItalicAngle 0 /Ascent 880 "
            b"/Descent -120 /CapHeight 700 /StemV 80 >>"),
    }
    data = bytearray(b"%PDF-1.5\n")
    offsets = {}
    for number in sorted(objects):
        offsets[number] = len(data)
        data += f"{number} 0 obj\n".encode() + objects[number] + b"\nendobj\n"
    xref = len(data)
    maximum = max(objects) + 1
    data += f"xref\n0 {maximum}\n0000000000 65535 f \n".encode()
    for number in range(1, maximum):
        if number in offsets:
            data += f"{offsets[number]:010d} 00000 n \n".encode()
        else:
            data += b"0000000000 00000 f \n"
    data += (f"trailer\n<< /Size {maximum} /Root 1 0 R >>\nstartxref\n{xref}\n"
             "%%EOF\n").encode()
    return data


def wide_tounicode_pdf(malformed_codespace=False):
    """Build a normal-looking A4 page with wide and unmapped CID codes."""
    cmap = (
        b"/CIDInit /ProcSet findresource begin\n"
        b"12 dict begin\nbegincmap\n"
        b"/CMapName /ResearchWide-Identity def\n/CMapType 1 def\n"
        b"2 begincodespacerange\n"
        b"<0100> <01ff>\n<00010000> <0001ffff>\n"
        b"endcodespacerange\n"
        b"3 begincidchar\n"
        b"<0101> 10\n<0103> 13\n<00010001> 11\n<00010002> 12\n"
        b"endcidchar\nendcmap\nend\nend\n"
    )
    codespaces = (
        b"<0100> <0001ffff>\n"
        if malformed_codespace
        else b"<0100> <01ff>\n<00010000> <0001ffff>\n"
    )
    tounicode = (
        b"/CIDInit /ProcSet findresource begin\n"
        b"12 dict begin\nbegincmap\n"
        b"/CIDSystemInfo << /Registry (Adobe) /Ordering (UCS) /Supplement 0 >> def\n"
        b"/CMapName /ResearchWide-UCS def\n/CMapType 2 def\n"
        b"2 begincodespacerange\n"
        + codespaces
        + b"endcodespacerange\n"
        b"3 beginbfchar\n"
        b"<0101> <0041>\n<00010001> <D83DDE00>\n<00010002> <4E2D>\n"
        b"endbfchar\n"
        b"1 beginbfrange\n<0103> <0103> 66\nendbfrange\n"
        b"endcmap\nend\nend\n"
    )
    content = (
        b"BT /F1 16 Tf 72 760 Td "
        b"(Annual report: wide-code font compatibility) Tj "
        b"/F2 16 Tf 72 730 Td <01010001000100010002000100030103> Tj ET"
    )
    return write_pdf({
        1: b"<< /Type /Catalog /Pages 2 0 R >>",
        2: b"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
        3: (b"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 595 842] "
            b"/Resources << /Font << /F1 10 0 R /F2 4 0 R >> >> /Contents 5 0 R >>"),
        4: (b"<< /Type /Font /Subtype /Type0 /BaseFont /ResearchWide "
            b"/Encoding 7 0 R /DescendantFonts [6 0 R] /ToUnicode 8 0 R >>"),
        5: stream(b"", content),
        6: (b"<< /Type /Font /Subtype /CIDFontType2 /BaseFont /ResearchWide "
            b"/CIDSystemInfo << /Registry (Adobe) /Ordering (Identity) /Supplement 0 >> "
            b"/DW 1000 /FontDescriptor 9 0 R >>"),
        7: stream(b"/Type /CMap /CMapType 1", cmap),
        8: stream(b"/Type /CMap /Subtype /CMap", tounicode),
        9: (b"<< /Type /FontDescriptor /FontName /ResearchWide /Flags 4 "
            b"/FontBBox [0 -200 1000 900] /ItalicAngle 0 /Ascent 880 "
            b"/Descent -120 /CapHeight 700 /StemV 80 >>"),
        10: b"<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>",
    })


def malformed_tounicode_pdf():
    """Build the wide-code fixture with a recoverable malformed codespace."""
    return wide_tounicode_pdf(malformed_codespace=True)


def write_pdf(objects, version=b"%PDF-1.5\n"):
    data = bytearray(version)
    offsets = {}
    for number in sorted(objects):
        offsets[number] = len(data)
        data += f"{number} 0 obj\n".encode() + objects[number] + b"\nendobj\n"
    xref = len(data)
    maximum = max(objects) + 1
    data += f"xref\n0 {maximum}\n0000000000 65535 f \n".encode()
    for number in range(1, maximum):
        if number in offsets:
            data += f"{offsets[number]:010d} 00000 n \n".encode()
        else:
            data += b"0000000000 00000 f \n"
    data += (f"trailer\n<< /Size {maximum} /Root 1 0 R >>\nstartxref\n{xref}\n"
             "%%EOF\n").encode()
    return data


def navigation_semantics_pdf():
    metadata = b"<x:xmpmeta><dc:title>Quarterly report</dc:title></x:xmpmeta>"
    objects = {
        1: (b"<< /Type /Catalog /Pages 2 0 R /Names 11 0 R "
            b"/Outlines 13 0 R /OpenAction [3 0 R /FitH 760] /PageLabels 15 0 R "
            b"/Metadata 16 0 R >>"),
        2: b"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
        3: (b"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 595 842] "
            b"/Annots [17 0 R 20 0 R 21 0 R 22 0 R] "
            b"/Resources << /Font << /F1 4 0 R >> >> "
            b"/Contents 5 0 R >>"),
        4: b"<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>",
        5: stream(b"", b"BT /F1 18 Tf 72 740 Td (Quarterly report) Tj ET"),
        11: b"<< /Dests 12 0 R >>",
        12: b"<< /Names [(chapter) [3 0 R /FitH 760]] >>",
        13: b"<< /Type /Outlines /First 18 0 R /Last 18 0 R /Count 1 >>",
        14: b"<< /Type /Action /S /GoTo /D [3 0 R /FitH 760] >>",
        15: b"<< /Nums [0 19 0 R] >>",
        16: stream(b"/Type /Metadata /Subtype /XML", metadata),
        17: (b"<< /Type /Annot /Subtype /Link /Rect [72 680 220 700] "
             b"/Border [0 0 0] /A << /S /URI /URI "
             b"(https://example.com/report) >> >>"),
        20: (b"<< /Type /Annot /Subtype /Text /Rect [240 680 260 700] "
             b"/Contents (Review note) /NM (note-1) /M (D:20240826090000Z) "
             b"/C [1 0 0] /Border [0 0 2] /Popup 21 0 R /IRT 22 0 R >>"),
        21: (b"<< /Type /Annot /Subtype /Popup /Rect [260 680 420 760] "
             b"/Parent 20 0 R /Open true >>"),
        22: (b"<< /Type /Annot /Subtype /Text /Rect [240 640 260 660] "
             b"/Contents (Parent note) /NM (parent-note) >>"),
        18: (b"<< /Title (Chapter 1) /Parent 13 0 R "
             b"/A << /S /URI /URI (https://outline.example) /Next ["
             b"<< /S /Named /N /NextPage >> "
             b"<< /S /Launch /F (report.pdf) >> "
             b"<< /S /JavaScript /JS (app.alert) >> "
             b"<< /S /GoTo /D [3 0 R /Fit] >>] >> >>"),
        19: b"<< /S /D /P (Section ) /St 3 >>",
    }
    return write_pdf(objects)


def type3_glyph_pdf():
    """Build a normal A4 page whose Type3 CharProc has path and text children."""
    objects = {
        1: b"<< /Type /Catalog /Pages 2 0 R >>",
        2: b"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
        3: (b"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 595 842] "
            b"/Resources << /Font << /F1 4 0 R /F2 7 0 R >> >> /Contents 5 0 R >>"),
        4: (b"<< /Type /Font /Subtype /Type3 /Name /ReportType3 "
            b"/FontBBox [0 0 600 700] /FontMatrix [0.001 0 0 0.001 0 0] "
            b"/CharProcs << /A 6 0 R >> "
            b"/Encoding << /Type /Encoding /Differences [65 /A] >> "
            b"/FirstChar 65 /LastChar 65 /Widths [600] "
            b"/Resources << /Font << /F1 7 0 R >> >> >>"),
        5: stream(
            b"",
            b"BT /F2 18 Tf 72 760 Td (Quarterly financial review) Tj ET "
            b"BT /F1 72 Tf 72 680 Td (A) Tj ET",
        ),
        6: stream(
            b"",
            b"600 0 0 0 600 700 d1 "
            b"100 0 m 300 700 l 500 0 l 420 0 l 370 180 l 230 180 l 180 0 l h f "
            b"BT /F1 8 Tf 260 260 Td (X) Tj ET",
        ),
        7: b"<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>",
    }
    return write_pdf(objects)


def page_labels_pdf():
    objects = {
        1: (b"<< /Type /Catalog /Pages 2 0 R /PageLabels 10 0 R "
            b"/OpenAction [3 0 R /Fit] >>"),
        2: b"<< /Type /Pages /Kids [3 0 R 4 0 R 5 0 R 6 0 R] /Count 4 >>",
        3: (b"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 595 842] "
            b"/Resources << /Font << /F1 8 0 R >> >> /Contents 7 0 R >>"),
        4: (b"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 595 842] "
            b"/Resources << /Font << /F1 8 0 R >> >> /Contents 7 0 R >>"),
        5: (b"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 595 842] "
            b"/Resources << /Font << /F1 8 0 R >> >> /Contents 7 0 R >>"),
        6: (b"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 595 842] "
            b"/Resources << /Font << /F1 8 0 R >> >> /Contents 7 0 R >>"),
        7: stream(b"", b"BT /F1 18 Tf 72 740 Td (Annual report page) Tj ET"),
        8: b"<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>",
        10: b"<< /Nums [0 11 0 R 1 12 0 R 2 13 0 R 3 14 0 R] >>",
        11: b"<< /S /D /P (Page ) /St 1 >>",
        12: b"<< /S /r /P (Appendix ) /St 3 >>",
        13: b"<< /S /A /P (Annex ) /St 2 >>",
        14: b"<< /S /a /P (Part ) /St 4 >>",
    }
    return write_pdf(objects)


def form_xobject_pdf():
    objects = {
        1: b"<< /Type /Catalog /Pages 2 0 R >>",
        2: b"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
        3: (b"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 595 842] "
            b"/Resources << /XObject << /Fm1 4 0 R >> >> /Contents 5 0 R >>"),
        4: stream(
            b"/Type /XObject /Subtype /Form /FormType 1 "
            b"/BBox [0 0 300 200] "
            b"/Resources << /Font << /F1 6 0 R >> >>",
            b"q BT /F1 14 Tf 20 150 Td (Form summary) Tj ET Q",
        ),
        5: stream(b"", b"q 1 0 0 1 72 560 cm /Fm1 Do Q"),
        6: b"<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>",
    }
    return write_pdf(objects)


def resource_semantics_pdf():
    """Build a normal A4 page exercising page and Form resource selection."""
    content = (
        b"BT /F1 12 Tf 72 760 Td (Quarterly resource review) Tj ET\n"
        b"q /GS1 gs /CS1 cs 0.1 0.3 0.5 scn "
        b"72 650 180 80 re f\n"
        b"/Pattern cs /P1 scn 72 520 180 80 re f\n"
        b"/Sh1 sh\n"
        b"/Span /MC1 BDC BT /F1 10 Tf 72 420 Td (Revenue region) Tj ET EMC\n"
        b"q 1 0 0 1 300 300 cm /Fm1 Do Q Q"
    )
    form_content = (
        b"q /FGS gs /Span /FM1 BDC "
        b"BT /F1 10 Tf 10 80 Td (Form resource note) Tj ET EMC Q"
    )
    objects = {
        1: b"<< /Type /Catalog /Pages 2 0 R >>",
        2: b"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
        3: (
            b"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 595 842] "
            b"/Resources << /Font << /F1 11 0 R >> "
            b"/XObject << /Fm1 4 0 R >> "
            b"/ExtGState << /GS1 6 0 R >> "
            b"/ColorSpace << /CS1 /DeviceRGB >> "
            b"/Pattern << /P1 7 0 R >> "
            b"/Shading << /Sh1 8 0 R >> "
            b"/Properties << /MC1 10 0 R >> >> /Contents 5 0 R >>"
        ),
        4: stream(
            b"/Type /XObject /Subtype /Form /FormType 1 /BBox [0 0 180 100] "
            b"/Resources << /Font << /F1 11 0 R >> "
            b"/ExtGState << /FGS 12 0 R >> "
            b"/Properties << /FM1 13 0 R >> >>",
            form_content,
        ),
        5: stream(b"", content),
        6: b"<< /LW 2 /CA 0.5 /ca 0.5 /BM /Multiply /D [[3 1] 0] >>",
        7: stream(
            b"/Type /Pattern /PatternType 1 /PaintType 1 /TilingType 1 "
            b"/BBox [0 0 10 10] /XStep 10 /YStep 10 /Resources << >>",
            b"0 0 m 10 10 l S",
        ),
        8: b"<< /ShadingType 2 /ColorSpace /DeviceRGB /Coords [72 400 250 400] /Function 9 0 R /Extend [true true] >>",
        9: b"<< /FunctionType 2 /Domain [0 1] /C0 [1 0 0] /C1 [0 0 1] /N 1 >>",
        10: b"<< /Lang (en-US) /ActualText (Revenue region) >>",
        11: b"<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>",
        12: b"<< /LW 1.5 /CA 0.7 /ca 0.7 >>",
        13: b"<< /Lang (en-US) /ActualText (Form resource) >>",
    }
    return write_pdf(objects)


def visual_semantics_pdf():
    """Build an A4 report page combining color, clipping, image, and form semantics."""
    content = (
        b"BT /F1 12 Tf 72 780 Td (Visual semantics quarterly report) Tj ET\n"
        b"q /GS1 gs /CSCal cs 0.1 0.2 0.3 scn 72 650 100 60 re f\n"
        b"/CSIndexed cs 2 scn 180 650 100 60 re f\n"
        b"/CSSpot cs 0.5 scn 288 650 100 60 re f\n"
        b"/CSDeviceN cs 0.1 0.2 scn 396 650 80 60 re f\n"
        b"/CSICC cs 0.1 0.2 0.3 scn 490 650 80 60 re f\n"
        b"/Pattern cs /P1 scn 396 560 174 60 re f Q\n"
        b"q 72 500 200 100 re W n 0 0 m 272 500 l 272 600 l h S Q\n"
        b"/Sh1 sh\n"
        b"/Span /MC1 BDC BT /F1 10 Tf 72 420 Td "
        b"(Color and clipping review) Tj ET EMC\n"
        b"q 1 0 0 1 300 280 cm /Fm1 Do Q\n"
        b"q 48 0 0 48 72 300 cm BI /W 1 /H 1 /BPC 8 /CS /G /F /AHx ID FF>EI Q\n"
        b"q 48 0 0 48 140 300 cm BI /W 1 /H 1 /BPC 8 /CS /G /F /A85 ID z~>EI Q"
    )
    form_content = (
        b"q /GS2 gs BT /F1 9 Tf 10 45 Td "
        b"(Inherited form resource) Tj ET Q"
    )
    objects = {
        1: b"<< /Type /Catalog /Pages 2 0 R >>",
        2: b"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
        3: (
            b"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 595 842] "
            b"/Resources << /Font << /F1 10 0 R >> "
            b"/XObject << /Fm1 4 0 R >> "
            b"/ExtGState << /GS1 6 0 R /GS2 7 0 R >> "
            b"/ColorSpace << /CSCal 8 0 R /CSIndexed 9 0 R /CSSpot 11 0 R "
            b"/CSDeviceN 16 0 R /CSICC 17 0 R >> "
            b"/Pattern << /P1 13 0 R >> "
            b"/Shading << /Sh1 14 0 R >> "
            b"/Properties << /MC1 15 0 R >> >> /Contents 5 0 R >>"
        ),
        4: stream(
            b"/Type /XObject /Subtype /Form /FormType 1 /BBox [0 0 180 100] "
            b"/Group << /S /Transparency /CS /DeviceRGB /I true /K false >>",
            form_content,
        ),
        5: stream(b"", content),
        6: b"<< /LW 2 /CA 0.4 /ca 0.6 /BM /Multiply /D [[4 2] 0] >>",
        7: b"<< /LW 1 /CA 1 /ca 1 /BM /Screen >>",
        8: b"[ /CalRGB << /WhitePoint [1 1 1] /Gamma [2 2 2] >> ]",
        9: b"[ /Indexed /DeviceRGB 3 <FF000000FF000000FFFFFFFF> ]",
        10: b"<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>",
        11: b"[ /Separation /SpotColor /DeviceRGB 12 0 R ]",
        12: (
            b"<< /FunctionType 2 /Domain [0 1] /C0 [1 0 0] "
            b"/C1 [0 0 1] /N 1 >>"
        ),
        13: stream(
            b"/Type /Pattern /PatternType 1 /PaintType 1 /TilingType 1 "
            b"/BBox [0 0 8 8] /XStep 8 /YStep 8 /Resources << >>",
            b"0 0 m 8 8 l S",
        ),
        14: (
            b"<< /ShadingType 2 /ColorSpace /DeviceRGB "
            b"/Coords [72 400 250 400] /Function 12 0 R /Extend [true true] >>"
        ),
        15: b"<< /Lang (en-US) /ActualText (Visual region) >>",
        16: b"[ /DeviceN [ /Cyan /Magenta ] /DeviceCMYK 18 0 R ]",
        17: b"[ /ICCBased 19 0 R ]",
        18: (
            b"<< /FunctionType 2 /Domain [0 1 0 1] /C0 [0 0 0 0] "
            b"/C1 [1 1 1 1] /N 1 >>"
        ),
        19: stream(b"/N 3", b""),
    }
    return write_pdf(objects)


def main():
    OUT.mkdir(parents=True, exist_ok=True)
    outputs = {
        "acceptance_adjacent_glyphs.pdf": adjacent_glyphs_pdf(),
        "acceptance_macexpert_encoding.pdf": macexpert_encoding_pdf(),
        "acceptance_vertical_cid.pdf": vertical_text_pdf(),
        "acceptance_tagged_text.pdf": tagged_text_pdf(),
        "acceptance_rgb_image.pdf": image_pdf(),
        "acceptance_inline_image.pdf": inline_image_pdf(),
        "acceptance_path_subpaths.pdf": path_subpaths_pdf(),
        "acceptance_form_choice.pdf": form_pdf(),
        "acceptance_encrypted_r2.pdf": encrypted_r2_pdf(),
        "acceptance_cjk_cid.pdf": cjk_text_pdf(),
        "acceptance_type3_glyph.pdf": type3_glyph_pdf(),
        "acceptance_wide_tounicode.pdf": wide_tounicode_pdf(),
        "acceptance_malformed_tounicode.pdf": malformed_tounicode_pdf(),
        "acceptance_navigation_semantics.pdf": navigation_semantics_pdf(),
        "acceptance_page_labels.pdf": page_labels_pdf(),
        "acceptance_form_xobject.pdf": form_xobject_pdf(),
        "acceptance_resource_semantics.pdf": resource_semantics_pdf(),
        "acceptance_visual_semantics.pdf": visual_semantics_pdf(),
    }
    args = sys.argv[1:]
    check = "--check" in args
    selected = {arg for arg in args if not arg.startswith("-")}
    for name, data in outputs.items():
        if selected and name not in selected:
            continue
        path = OUT / name
        if check:
            if not path.exists() or path.read_bytes() != data:
                raise SystemExit(f"acceptance fixture is out of date: {path}")
        else:
            path.write_bytes(data)


if __name__ == "__main__":
    main()
