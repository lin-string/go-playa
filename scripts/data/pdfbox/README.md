# Apache PDFBox expert encoding inputs

These unmodified source files are synchronized from Apache PDFBox commit
`5ae91127c3316db8663da843dce746022f22df91`. The upstream Apache-2.0 headers,
`LICENSE.txt`, and `NOTICE.txt` are preserved. The generated Go tables are
translations of the encoding data; they are not Java runtime dependencies.

| Local file | Upstream path | SHA-256 |
| --- | --- | --- |
| `MacExpertEncoding.java` | `pdfbox/src/main/java/org/apache/pdfbox/pdmodel/font/encoding/MacExpertEncoding.java` | `c837b6b1bd6add722ec4b9cc5d3998e0dfb6c1feb2c92465cac5947f8ba6fc6f` |
| `CFFExpertEncoding.java` | `fontbox/src/main/java/org/apache/fontbox/cff/CFFExpertEncoding.java` | `e6d9d3816e2e5098b99b69dcbb190e7d12cbedd571fd9720c454948c542e2da9` |
| `LICENSE.txt` | `LICENSE.txt` | `1301d8415a4868d82aeeec594849cf7679f1ead4636a9603dc46875f5713157e` |
| `NOTICE.txt` | `NOTICE.txt` | `40741b4ab76d77ba4fbc5e8759277169fb0ce281859d273075de6fd3a3588458` |

Source repository: <https://github.com/apache/pdfbox/tree/5ae91127c3316db8663da843dce746022f22df91>

`MacExpertEncoding` is the PDF encoding described in ISO 32000-1 Annex D.3.
CFF `ExpertEncoding` is the separate predefined CFF code-to-SID mapping in
Adobe Technical Note #5176 Appendix B. For example, PDF MacExpert code 35 is
`centoldstyle`, while CFF Expert code 35 is undefined. The CFF mapping was
checked against all 256 code/SID entries in the official specification:
<https://adobe-type-tools.github.io/font-tech-notes/pdfs/5176.CFF.pdf>.

Refresh both sources and regenerate the tables with:

```sh
python3 scripts/sync_macexpert_encoding.py
python3 scripts/generate_macexpert_encoding.py
python3 scripts/generate_cff_resources.py
make resources-check
```

The synchronizer also accepts `--source-dir` for a local PDFBox checkout.
Every input must match its pinned SHA-256; changing the upstream revision
requires an explicit update to the synchronizer and this provenance record.
`--check` verifies the checked-in inputs without network access. Preserve
the original file bytes, including line endings, when synchronizing.
