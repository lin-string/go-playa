# 映射来源与兼容性

[English](mapping-sources.md) | 简体中文

go-playa 不把 Playa 的历史查找表视为字体或字符映射的规范真源。Playa 是重要的
兼容参考，但其表可能反映固定 oracle release 当时的版本和平台假设。

## 真源层级

实现按以下权威顺序使用真源：

1. PDF 提供的 `ToUnicode`、嵌入字体 `cmap`、CID 度量和显式 `Differences` 条目。
2. 对应领域的规范 Adobe 表：
   - Adobe Glyph List（`glyphlist.txt`），用于 glyph name 到 Unicode 映射。
   - ITC Zapf Dingbats Glyph List，用于 Zapf Dingbats 名称。
   - Adobe CMap Resources 和字符集合，用于 byte-to-CID 映射、codespace、竖排
     变体和 CID 数据。迁移后的 loader 嵌入 vendored Adobe CMap Resources
     快照中的完整 CMap 资源集，包括 Identity、历史东亚编码、UCS2/UTF8/UTF16/
     UTF32 map、UniJISPro、JIS X 0213，以及 Japan1、GB1、CNS1、Korea1、
     Adobe-KR、Adobe-Manga1 字符集合。具名 `usecmap` 继承使用隔离缓存副本惰性
     解析。
3. MacExpert、平台名称和有文档记录的 private-use 映射所需的上下文专用历史表。
   Symbol 编码由同步的 Core 14 `Symbol.afm` 生成，并保留少量有记录的 Symbol
   专用 glyph-name override。
4. Playa 的表和行为；没有更权威来源时作为兼容 fallback。

`ToUnicode` 仍是文本提取的首选来源，因为它嵌入 PDF 并描述该文档预期的映射。
glyph list 是 simple font 和 `Differences` 的 fallback，不能取代 CID CMap、宽度
或字体程序字符映射。

## AGL 与 AGLFN

AGL 和 AGLFN 相关但不可互换。AGL 把历史 glyph name 映射到 Unicode，包括历史
presentation-form 和 private-use 分配。AGLFN 是面向新字体的推荐名称子集，会
有意省略其 AGL 映射并非语义首选 Unicode 值的名称。因此 PDF 提取首先使用 AGL；
AGLFN 可用于校验和未来命名 heuristic，不能替代完整表。

## 生成数据

同步的 Adobe Glyph List（AGL）与 ITC Zapf 真源存放在
`scripts/data/glyphlist.txt` 和 `scripts/data/zapfdingbats.txt`。从 Adobe 刷新
真源并重新生成 Go map：

```text
go generate ./document ./fontdata
```

真源同步器是 `scripts/sync_glyphlist_sources.py`，可通过 `--source-dir` 使用
离线 Adobe checkout。解析器/生成器是 `scripts/generate_glyphlist.py`，生成结果
为 `fontdata/glyphlist_generated.go`。更新真源时必须评审映射差异，并为发生变化
的历史名称添加兼容测试。

生成器写入确定性 Go 表前会校验重复名称和 Unicode scalar。生成的 ITC 表是
Zapf 权威来源；只有八个明确记录的历史名称保留在
`fontdata.GlyphTextWithLegacyAliases` 中，以兼容旧 PDF/Playa 行为。

预定义 CMap 资源从当前 Adobe `cmap-resources` 仓库同步到
`fontdata/cmapdata/`。运行 `go generate ./document ./fontdata` 刷新；同步脚本也
支持通过 `--source-dir` 使用离线 Adobe checkout。嵌入集合是完整上游
`*/CMap/*` 资源集，而非人工选择的列表。runtime 名称注册表从该嵌入目录派生；
fontdata 测试会加载每个同步资源，避免上游新 CMap 被静默遗漏。
`font.LoadPredefinedCMap` 向需要在页面字体字典之外解析具名 CMap 的调用方暴露
同一个 loader。

`font.LoadPredefinedUnicodeMap` 暴露 CID 字体 fallback 使用的对应不可变 CID 到
Unicode 视图。它从相同 Adobe UTF-32 CMap 惰性建立，按集合和 writing mode
缓存，并同时接受完整 Adobe registry 名（如 `Adobe-Japan1`）与 PDF `Ordering`
值（`Japan1`）。返回的 map 不暴露 backing store；需要自有快照时使用
`MappingCopy` 或 `Finalize`。

对于具有已识别 Adobe `CIDSystemInfo` ordering 的 Type0 descendant，loader 还会
从匹配 UTF32 CMap 构建反向 CID-to-Unicode fallback，包括 supplementary Unicode
字符及 `KR`、`Manga1` 集合。该 fallback 的优先级明确低于 PDF `ToUnicode` 和
嵌入字体字符映射。

CIDFontType2 资源也遵循显式 `CIDToGIDMap` stream。通过该表查找嵌入 TrueType
glyph 映射；PDF `Identity` 形式和缺省形式继续使用直接 CID-to-GID 行为。

对于嵌入 TrueType 和 TrueType-outline OpenType 字体，会从 `FontFile2` 或
`FontFile3 /Subtype /OpenType` 惰性读取 `head`、`hhea` 和 `hmtx`，提供 cmap
映射和缩放 glyph advance width。显式 PDF width 仍为权威来源。CFF1、Type1C 和
CFF2 `FontFile3` 程序现在提供嵌入 geometry metadata；Type2 drawing program 已
实现 CharString 执行和 outline 提取，包括 CID FDSelect 专用 local subroutine、
CFF2 variation-store 求值和惰性路径提取。

CFF standard SID string、预定义 charset 和 StandardEncoding 直接从公开
fontTools 真源生成。CFF ExpertEncoding 使用 `scripts/data/pdfbox/` 中固定的
Apache PDFBox `CFFExpertEncoding.java` 表，并独立与 Adobe Technical Note
#5176 Appendix B 核对。生成器保留上游的数值 CFF SID，不使用 MacExpertEncoding
替代，也不以 Playa 表为输入。

运行 `go generate ./document ./fontdata` 刷新生成 Go 表。离线生成时，通过
`--fonttools-source-dir` 指向本地 fontTools checkout，`--expert-source` 选择 CFF
ExpertEncoding Java 输入；默认值是检入的 PDFBox 输入。

CFF 生成的 StandardEncoding 在运行时提供所有已定义的 PDF StandardEncoding
slot。未定义 slot 有意保持缺失，不能从类似 MacRoman 的历史表填充，也不能解码
成 replacement text。

Core 14 标准字体度量由 14 个 Adobe AFM 文件生成。原 Adobe 下载 endpoint 已不
可用，因此检入输入从公开 `tc-font-core14-afms` mirror 同步；其 README 标识这些
文件为 Adobe Core 14 AFM。AFM 格式与字段语义遵循 Adobe Technical Note #5004。
这里有意把来源记录为 mirror，而不是宣称它是当前 Adobe 仓库；如果未来发布官方
Adobe endpoint，可以切换同步器。

运行 `go generate ./document ./fontdata` 刷新 AFM 并重新生成
`fontdata/standard_metrics_generated.go`。同步器支持用 `--source-dir` 指向离线
AFM 真源 checkout。

内置 Symbol code-to-Unicode 映射由 `scripts/data/core14/Symbol.afm` 和
`scripts/data/glyphlist.txt` 通过 `scripts/generate_symbol_encoding.py` 生成。
其含义由 Symbol 字体而非通用 AGL 定义的名称，在生成器中保留为显式 override。
运行 `go generate ./document ./fontdata` 刷新生成表。

内置 ZapfDingbats code-to-Unicode 映射由
`scripts/data/core14/ZapfDingbats.afm` 和 `scripts/data/zapfdingbats.txt` 通过
`scripts/generate_zapfdingbats_encoding.py` 生成；AFM 中的 space slot 会显式
保留，因为它不在 ITC glyph-name list 中。

高字节 MacRoman 映射由标准 Unicode/CPython `mac_roman` codec 通过
`scripts/generate_macroman_encoding.py` 生成。生成表只包含 PDF MacRoman
高字节范围，并由 `go generate ./document ./fontdata` 刷新。

高字节 WinAnsi 映射由标准 Windows-1252 codec 通过
`scripts/generate_winansi_encoding.py` 生成；codec 未定义的 slot 在生成表中
保持缺失，与 PDF 未定义 slot 行为一致。

MacExpertEncoding 从 Apache PDFBox 独立的 `MacExpertEncoding.java` 表生成。
`scripts/sync_macexpert_encoding.py` 从 PDFBox commit
`5ae91127c3316db8663da843dce746022f22df91` 同步两张 ExpertEncoding 输入，并在脚本
中固定 SHA-256。Java 输入保存在 `scripts/data/pdfbox/`，保留 Apache-2.0 原始
header；许可证与署名记录在 `NOTICE`。`make resources-check` 离线校验固定输入
摘要，再检查生成 Go 表是否匹配。同步器的 `--source-dir` 接受本地 PDFBox checkout。

`scripts/generate_macexpert_encoding.py` 输出
`fontdata/macexpert_encoding_generated.go`。MacExpert 以 glyph name 保存映射，使
AGL、private-use 和 composite-name 解析共用同一条有优先级的路径。CFF
ExpertEncoding 则把字符码映射到 CFF SID；两张表来自同一项目，不代表编码可互换。

## 私有与平台映射

private-use 映射绝不会作为无条件的全局 Unicode 真值安装。同一 code point 或
glyph name 在 Symbol、MacExpert、Zapf Dingbats、Apple 和供应商专用字体中可能
含义不同。这些表必须带有作用域，并且只能在考虑 PDF 提供的映射和标准 glyph-name
解析后参与。

添加私有映射时，在实现或定向测试中记录真源、适用字体或编码、Unicode 解释和
优先级。只有不覆盖文档提供映射或规范映射时，才保留改善旧 PDF/Playa 行为的
历史映射名称。

## 与 Playa 的差异

这套 source-first 策略与把历史 Playa 表作为完整映射数据库有意不同。它可能为
较新的 glyph name 提供更完整结果，同时为作用域明确的 legacy/private-font 案例
保留 Playa 兼容输出。兼容测试必须区分真实语义回归与对过时表的有意修正。
