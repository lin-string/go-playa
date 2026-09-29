# 公开 API 审计

[English](api-audit.md) | 简体中文

本审计把当前 Go 公开 surface 映射到
[`compat/upstream.toml`](../compat/upstream.toml) 固定的 Playa oracle，是 README
中精简命名规则背后的详细符号台账。

## 分类

| Go 包/符号 | 分类 | 方向 |
|---|---|---|
| `playa.Open`、`OpenBytes`、`Document`、`Page` | Playa 模型加 Go 适配 | 保留；`Close` 和返回错误取代 Python context manager/exception 控制流。 |
| `playa.PageList`、`DeviceSpace`、`Point`、`Rect`、`ObjRef`、`ContentStream` | Playa 根 facade 重导出 | 暴露可重复惰性页面序列、坐标空间值、PDF geometry tuple、间接引用值和内容流值，不重新引入可变文档所有权。 |
| `playa.ContentStream` 元数据和 mapping helper | Playa `ContentStream.attrs`、`rawdata`、`buffer`、`filters`、`get_filters`、`get_any`、`width`、`height`、`bits`、`colorspace`、`objid`、`genno`、`keys`、`values`、`items` | `pdftypes.Stream` 保留原始/解码 buffer，以及 `AttrsCopy`、`GetFilters`、可变参数 `GetAny`/`GetAnyDefault`、`Width`、`Height`、`Bits`、`ColorSpace`/`ColorSpaceSpec`、`Ref`、`ObjectID`、`Generation`、`Keys`、`Values`、`Items`；返回容器均为独立副本，直接 stream 不报告间接身份。 |
| `parser.Lexer.Data` | Go 词法解析器源 buffer 访问器 | 默认返回防御性副本；文档 inline-image 路径使用显式命名的 `DataBorrowed` 视图，在普通访问器不暴露可变存储的前提下避免热路径额外分配。 |
| `playa.Resolve`、`playa.ResolveAll` | Playa `resolve`/`resolve_all` | 使用显式 Go 引用查找 callback；`Resolve` 带可选默认值和 cycle guard 跟随有界引用链，`ResolveAll` 保留递归复制语义。 |
| `playa.GraphicsState`、`playa.ColorSpace` | Playa 根 `GraphicState`、`ColorSpace` | `GraphicsState` 是自有状态值的惯用 Go 拼写；`ColorSpace` 复用 `imagedata.ColorSpace`，在 PDF 对象边界暴露带零填充和 pattern-name 处理的 `MakeColor`。Indexed color 还会把有限分量按 0.5 向上规则舍入至最近整数，并钳制到 `0..High`。 |
| `playa.AsObject` | Playa `asobj` | 把 PDF primitive graph 转成 JSON-friendly Go 值，不暴露可变解析器存储；byte string 可表示时保持 ASCII，否则使用 `base64:`。 |
| `Document` 对象/页面/元数据/导航方法 | Playa 文档模型 | 以对应的惰性 property 和 sequence 保留。 |
| `Document.ObjectCount`、`Document.ObjectRefs`、`Document.Objects` | Playa `len(document)`、文档 object-ID 迭代和源顺序对象值 | 作为惰性索引支持的 Go 方法保留；引用只读且可重复，对象值通过 `Objects` 保持借用；加密公开值遵循 Playa 解密语义，包括普通 stream 和安全字典 byte string。 |
| `Document.Encryption`、`EncryptionInfo` | Playa `Document.encryption` | 通过 `EncryptionWithError` 返回自有 file-ID 和加密字典快照；密码 key、cipher 状态和解析器 `decipher` callback 留在文档内部。 |
| `Document.XRefs`、`XRefTable`、`XRefEntry` | Playa `Document.xrefs`、`XRefPos` | 保留按 revision 排序的只读快照；hybrid stream 是独立条目，压缩对象位置保留 stream ID/index 而不保留解析器 buffer。 |
| `documentdata.IndirectObject` | `Document.Objects`、`Document.Items` 的 Go 值模型 | 持有与文档无关的引用/值 pair 和递归 primitive 快照逻辑；`document` 保留对象查找、顺序、缓存和文档自有 `Stream` clone。`ValueCopy` 与 `Finalize` 是显式所有权边界。 |
| `documentdata.NameTreeEntry` | `Document.NameTreeSeq` 的 Go 值模型 | 持有与文档无关的 name/value pair 和递归 primitive 快照；`document` 保留 name-tree 遍历、校验、顺序和有界缓存协调。`ValueCopy` 与 `Finalize` 是显式所有权边界。 |
| `documentdata.DestinationEntry` | `Document.DestinationsSeq` 的 Go 值模型 | 持有与文档无关的 name/value pair 和递归 primitive 快照；`document` 保留命名目的地优先级、遍历、缓存协调和页面/坐标空间解析。`ValueCopy` 与 `Finalize` 是显式所有权边界。 |
| `documentdata.Destination` | 规范化 PDF destination array 的 Go 值模型 | 持有 page-ref/page-index、view name、递归复制的 view parameter 和 `NCoords` metadata；`document.Destination` 保留依赖 Document 的页面查找、坐标变换和 geometry helper。`ParamsCopy`、`ParamCopy`、`Finalize` 是显式所有权边界。 |
| `documentdata.Action` | 规范化 action metadata 的 Go 值模型 | 持有 kind、URI/file/name/script scalar 和递归复制的原始 action 字典；`document.Action` 保留依赖 Document 的目的地解析、惰性 `/Next` 遍历和延迟错误。`RawCopy`、`Finalize` 是显式所有权边界。 |
| `imagedata.ColorSpace`、`document.ImageColorSpace` | PDF 图像 color-space metadata 的 Go 值模型 | `imagedata.ColorSpace` 持有无依赖 name、component、Indexed、ICC、colorant 和嵌套 base 值；`document.ImageColorSpace` 保留文档 lookup byte、filter parameter、惰性解码错误和有界解码缓存。`ColorantsCopy`、`BaseCopy`、`Finalize` 保持显式所有权边界。 |
| `ColorSpace.spec` | Playa 原始 color-space specification 字段 | `imagedata.ColorSpace.SpecCopy` 和 `document.ImageColorSpace.SpecCopy` 保留独立 primitive specification 快照，不暴露解析器自有 array/dict。 |
| `contentdata.PropertiesObject`、`document.PropertiesObject` | 具名 BDC/DP Properties 选择的 Go 值模型 | `contentdata.PropertiesObject` 持有名称、operator、page reference 和递归复制的 primitive 字典；`document.PropertiesObject` 重导出同一当前类型，文档保留资源查找、内容顺序遍历和异常资源错误。`DictCopy`、`Finalize` 是显式所有权边界。 |
| `contentdata.MarkedContentContext`、`document.MarkedContentContext` | 外围 marked-content stack entry 的 Go 值模型 | `contentdata.MarkedContentContext` 持有 tag、properties、ActualText 和 MCID；`document.MarkedContentContext` 重导出同一当前类型，私有解释器 wrapper 保留 grouping identity 和 marked-stack 编排。`PropertiesCopy`、`Finalize` 是显式所有权边界。 |
| `contentdata.ContentOp`、`document.ContentOp` | 单个页面内容操作的 Go 值模型 | `contentdata.ContentOp` 持有 operator、源 offset、operand、借用 operand 迭代和 finalized copy；`document.ContentOp` 只保留解释使用的资源字典、Form 边界 marker 和 Properties-selection context。 |
| `contentdata.MarkedContent`、`document.MarkedContent` | 单个 BMC/BDC section 的 Go 值模型 | `contentdata.MarkedContent` 持有 tag、page/MCID metadata、ActualText、properties 和 ContentOp；`document.MarkedContent` 保留惰性 child、ParentTree/page 解析和源顺序遍历。`PropertiesCopy`、`OpsCopy`、`Finalize` 是显式所有权边界。 |
| `contentdata.ExtGState`、`document.ExtGStateObject` | Playa 资源支持的 graphics-state 选择 | `contentdata` 持有所选资源名、页面引用、不可变 GraphicsState 和 primitive 字典；`document` 保留 ExtGState 资源查找、operator 遍历和异常资源错误。`GState`、`DictCopy`、`Finalize` 是显式所有权边界。 |
| `contentdata.Pattern`/`Shading`、`document.PatternObject`/`ShadingObject` | Playa pattern/shading 资源选择 | `contentdata` 持有选择 metadata、不可变 GraphicsState 和 primitive 资源字典；`document` 保留资源查找、stream 所有权和内容顺序遍历。stream copy 和 `Finalize` 是显式文档侧所有权边界。 |
| `contentdata.ColorSpaceSelection`、`document.ColorSpaceObject` | Playa 具名内容 color-space 选择 | `contentdata` 持有所选名称、page/stroke metadata 和 primitive 资源 spec；`document` 保留 `ImageColorSpace` 描述、惰性 Indexed lookup/filter 错误、资源解析和内容顺序遍历。`Info`、`SpecCopy`、`FinalizeWithError` 保持分层。 |
| `contentdata.GraphicsState`、`document.GraphicsState` | Playa 内容 graphics-state 快照 | `contentdata.GraphicsState` 通过只读访问器持有私有 scalar、color、dash、blend-mode、soft-mask、halftone；`document` 只保留可变解释器状态和资源解析。解释后的 Indexed color 暴露已舍入并按 `hival` 钳制的有效索引，而 `ContentOp` 保留原始操作数。公开 `GState`/`GraphicsStateCopy` 值及 graphics-state operation helper 跨越显式自有快照边界。 |
| `contentdata.DashPattern`、`GraphicsState.Dash` | Playa `DashPattern`、`GraphicState.dash` | 用 `Len`/`At`、`ValuesCopy`、`Finalize` 把 dash length 和 phase 保留为私有只读值；保留低分配 `DashCopy`/`DashPhase` 访问器和 JSON 投影。 |
| `document.Page` 页面内容与资源结果类型 | Go 文档入口 facade 重导出 | 导出 `document.PageAt` 返回的页面模型名称，使调用方无需导入实现包即可使用页面遍历、资源选择和结构结果。 |
| `ErrNilDocument` | 带显式文档参数的 API 所需稳定错误的 Go 适配 | 从根和领域 facade 导出，为值模型方法的显式文档参数保持共享 sentinel identity。指针接收器是必需输入，不提供 nil 接收器兜底语义。 |
| `Document.ForEachPageConcurrent`、`ForEachPageLayoutConcurrent`、`PageConcurrencyOption`、`WithMaxPageWorkers`、`WithPagesPerWorker` | 有界进程内页面调度的 Go 适配 | 两个方法都接收尾部封闭的函数式选项；类型和构造器由 `document` 和根 facade 导出。保留自动 worker 数上限、十页增长预算、惰性页面生成、`GOMAXPROCS`/页数限制、取消、首个错误传播、panic 恢复、无序回调完成和回调期所有权。 |
| content/image/font/page/outline/structure facade 中的 `ParseError` | 共享 typed malformed-PDF error | 导出 parser/document 使用的同一类型，使领域调用方无需导入无关 facade 即可使用 `errors.As`。 |
| `Page.ExtractText*` | Playa 页面文本提取 helper | 保留为带显式提取 options 和 error 的 Go 方法。 |
| `Page.ParentKey` | Playa `Page.parent_key` 和 `/StructParents` ParentTree 关联 | 保留为错误感知页面级值；兼容投影区分缺失 key 与显式零。 |
| `Page.StructureSeq`、`Page.Structure` | Playa `Page.structure` 和 ParentTree 页面 mapping | 保留惰性 slot 遍历及显式页面结构快照；兼容投影比较页面关联结构 element、内容 kind、attribute 和 child。 |
| `Annotation.BBox`、`Annotation.PageObject`、`Annotation.Parent`、`Annotation.Modified`、`Annotation.DictCopy` | Playa annotation bbox、所属页、外围结构 element、修改日期和原始 `props` | 保留所选空间 bbox、显式页面解析、ParentTree 结构关联、规范化日期和自有字典快照；兼容投影比较 bbox、page index、parent structure、日期和递归解析的 property。 |
| `TextObject.Size`、`TextObject.FontBase`、`TextObject.TextFont` | Playa 文本对象字体 property | 保留为 eager capture 的派生值；`Size` 使用 device-space glyph size，`FontBase` 去除 subset prefix。 |
| `TextObject.Matrix`、`TextObject.TextMatrix`、`TextObject.ScalingMatrix` | Playa 文本对象 rendering matrix | 按相同组合顺序保留捕获的 PDF text-space 与 device-space transform。 |
| `TextObject.LineMatrix` | Playa text-line 状态 | 保留对象开始时捕获的 text-line matrix。 |
| `TextObject.Args` | Playa text-show argument | 按源顺序保留复制的 `String`/`Number` operand，包括开头和内部 `TJ` adjustment。 |
| `TextObject.Chars` | Playa 解码字符文本 | 与 Go `Text()` 访问器并存，保留解码 Unicode 字符流。 |
| `TextObject.GState`、`TextObject.MarkedStackCopy`、`TextObject.PageObject` | Playa 文本对象 graphics state、marked-content stack 和页面归属 | 保留值快照和显式 copy/resolve 方法；兼容投影比较稳定 state、外围 stack 和零基 page index，不暴露可变 backing storage。 |
| `Page.ExtractText`、`Page.ExtractTextTagged`、`Page.ExtractTextUntagged` | Playa 同名文本提取方法 | 保留自动 tagged/untagged 选择和两个显式模式，使用显式 Go options/error；兼容投影比较三种结果。 |
| `Page.SetInitialCTM` | Playa `Page.set_initial_ctm` | 保留可变页面级 device-space/rotation override；`SetInitialCTMWithError` 显式解析页面 geometry，内容 iterator 复用所得 CTM 而不 eager reparse。 |
| `Page.Glyphs` | Playa `Page.glyphs` | 保留可重复惰性 glyph sequence；兼容投影比较直接 flatten glyph 顺序与完整 glyph 投影。 |
| `TextObject.Font` | Playa 已解析文本字体 | 保留缓存 `*Font`，但不纳入兼容序列化。 |
| `Document.Fonts`、`FontResource.Metadata`、`document.FontMetadata` | Playa `Document.fonts` 和页面字体 metadata 投影 | 保留带后字体覆盖规则的惰性文档/页面字体 mapping；兼容投影比较两种字体 record。 |
| `Document.GetFont` | Playa `Document.get_font` | 保留从字典独立惰性构建字体、对象号缓存 identity，以及 missing/unknown spec 的 Playa dummy-font fallback；`GetFontWithError` 暴露 Go 诊断。 |
| `Font.hdisp`、`vdisp`、`position`、`char_bbox` | Playa 字体字符度量 | 保留水平/垂直 displacement、竖排 position 和标准字符 bbox 计算；兼容投影比较页面与文档字体上的有界 CID probe。 |
| `Font.IsMultibyte` | Playa `Font.multibyte` | 以只读 Go predicate 暴露 class-level flag；固定 Playa release 对所有字体 subclass 都保持 false，因此不从 Go CID classifier 推导。 |
| `Font.WriteFontFile` | Playa `Font.write_fontfile` | 依 Playa `.pfa`/`.ttf`/`.cff` 选择和清理字体名约定写入惰性嵌入程序的有界快照；无程序时返回空路径。 |
| `Font.decode` | Playa source-code 到 CID/Unicode 解码 | 保留带 source-code 所有权和 Unicode fallback 的惰性 decoded-glyph sequence；兼容投影比较有界单/多字节 probe。 |
| `Font.Name`、`Font.BaseFont`、`Font.CIDCoding` | Playa `fontname`、`basefont`、`CIDFont.cidcoding` identity | 保留独立只读 identity；Type 0 font 独立保留根 `/BaseFont` 与 descendant descriptor name，CID coding 规范化 `/CIDSystemInfo` registry/order。 |
| `fontdata.LookupStandardFontMetric`、`StandardFontMetric`、`LookupZapfDingbatsWidth` | Playa Core14 AFM metric 和 ZapfDingbats width | 生成 AFM 资源保持 package-private；暴露 scalar/per-codepoint read 和显式 `WidthsCopy`/`Finalize`，不返回可变全局 map。 |
| `fontdata.BuiltinEncoding`、`fontdata.PredefinedCIDUnicodeMapName` | Playa 内置编码表和 CID Unicode ordering registry | 生成编码表/CID ordering map 保持 package-private；返回自有编码副本或 scalar 资源 lookup，调用方不能修改全局资源状态。 |
| `font.UnicodeMap`、`font.LoadPredefinedUnicodeMap` | Playa `cmapdb.UnicodeMap`、`get_unicode_map` | 保留为由同步 Adobe UTF-32 CMap 支持的缓存不可变 CID-to-Unicode 模型；接受完整 Adobe registry name 和 PDF ordering 名，并提供显式 copy/finalize 边界。 |
| `font.ParseEncodingCMap` | Playa `cmapdb.parse_encoding` | 嵌入字体 Encoding CMap parser 与通用/预定义 CMap 分开保留；保持 Playa 升序 codespace-width 解码顺序和 CID-zero fallback。 |
| `fontdata.ParseCIDToGIDMap` | Playa 嵌入 `CIDToGIDMap` stream 解码 | `fontdata` 持有无依赖双字节 CID-to-GID table parser；`document` 只保留惰性解码资源所有权。 |
| `Font.CIDToGIDCopy` | Playa `CIDFont.cid2gid` | 保留惰性自有 CID-to-GID 快照；复制 Go map 防止调用方修改字体缓存，错误感知形式保留延迟嵌入字体失败。 |
| `fontdata.ParseTrueTypeHorizontalMetrics` | Playa 嵌入 TrueType `head`/`hhea`/`maxp`/`hmtx` width fallback | `fontdata` 持有无依赖 sfnt horizontal-metrics parser；`document` 保留 PDF 优先级和惰性 stream 所有权。 |
| `fontdata.ParseCFFIndex`、`fontdata.ParseCFF2Index` | 嵌入 CFF 程序使用的 CFF1/CFF2 INDEX 解析 | `fontdata` 持有有界借用 slice INDEX 解析；`document` 保留字体资源 metadata、惰性 outline 和 variation-store 所有权。 |
| `fontdata.ParseCFFFDSelect` | CFF1/CFF2 FDSelect glyph-to-FD mapping | `fontdata` 持有有界 format 0/3/4 FDSelect 解码；`document` 保留 FDArray 校验和惰性字体所有权。 |
| `fontdata.ParseCFFNumber`、`fontdata.ParseCFFOperator`、`fontdata.ParseCFFCharStringOperator` | CFF/CFF2 数字和 operator token 解码 | `fontdata` 持有无依赖 scalar/operator byte 解码；`document` 保留 DICT 语义、variation state 和 outline 解释。 |
| `fontdata.ParseCFFDict`、`CFFDictEntry` | CFF/CFF2 DICT record 扫描 | `fontdata` 持有有界 number-stack/operator-record 解码；document 保留 Top/Private DICT 语义应用和跨 operator CFF2 blend state。 |
| `fontdata.ParseCFF2VariationStore` 和 `CFF2VariationStore` 访问器 | CFF2 Item Variation Store 解码与 region-scalar 求值 | `fontdata` 持有有界不可变 VariationStore 解析和 copy-based view；`document` 保留 PDF 字体所有权和 CharString blend state。 |
| `fontdata.ParseCFFInteger`、`ParseCFFNonNegativeInt`、`ParseCFFSubroutineIndex`、`CFFSubroutineBias` | CFF 数值域校验与 Type 2 Subr bias/index 计算 | `fontdata` 持有 finite/integral/bounded CFF operand 校验；`document` 只保留 CharString 语义的本地 adapter。 |
| `fontdata.ParseCFFCharStringWidth` | CFF1 Type 2 CharString width-prefix 提取 | `fontdata` 持有无依赖 width detection、local/global Subr 遍历和数值校验；document 保留 CFF2 variation-aware width state 和 PDF 字体所有权。 |
| `fontdata.ParseCFF2CharStringWidth`、`CFF2VariationStoreReader` | CFF2 Type 2 CharString width-prefix 提取 | `fontdata` 持有 `vsindex`、`blend`、local/global Subr 遍历、variation scalar 应用和有界 width 解析；document 只保留 variation-store 资源和字体缓存所有权。 |
| `fontdata.ParseType1Program`、`Type1Program` | 无依赖 Type 1 程序提取 | `fontdata` 持有有界 `/CharStrings`、`/Subrs`、`lenIV` 提取和 copy-based program view；document 只保留惰性 stream 解码、字体缓存所有权和内容模型转换。 |
| `fontdata.ParseType1CharString`、`ParseType1CharStringWithSeac`、`Type1PathOp` | 无依赖 Type 1 CharString outline 解释 | `fontdata` 持有有界 Subr/seac/flex 解释和不可变 path-operation view；document 只保留到内容操作模型的转换和惰性字体缓存。 |
| `fontdata.BuiltinEncoding` | Playa `encodingdb.ENCODINGS`、`get_encoding` 内置表 | `fontdata` 持有 Standard、MacRoman、WinAnsi、MacExpert、Symbol、ZapfDingbats 表副本；表由检入权威资源生成，调用方不能修改后续 lookup。 |
| `fontdata.GlyphText`、`GlyphTextWithLegacyAliases` | Playa `encodingdb.name2unicode` glyph-name 解析 | `GlyphText` 保持规范 Adobe/ITC AGL 语法；八个历史 PDF/Playa 映射名称只放入显式兼容 helper，使文档解析共用一个生成资源所有者而不改变规范公开 lookup。 |
| `Font.applyEncoding`、`Font.applyDifferences` | 内部 PDF 字体构建步骤 | 保持不导出；这些 mutator 在解析字体时组装编码状态，不属于 Playa 只读公开字体模型。如果任一名称出现在公开 `font.Font` facade，API 审计必须失败。 |
| `GlyphObject.Font`、`GlyphObject.Size`、`GlyphObject.FontBase`、`GlyphObject.TextFont`、`GlyphObject.Matrix` | Playa glyph 字体/rendering property | 保留已解析字体 metadata 和每 glyph device-space rendering matrix。 |
| `GlyphObject.Chars` | Playa 解码 glyph 字符文本 | 使用与上游 property 相同的名称保留解码 glyph 字符。 |
| `GlyphObject.GState`、`GlyphObject.MarkedStackCopy`、`GlyphObject.PageObject` | Playa glyph graphics state、marked-content stack 和页面归属 | 保留值快照和显式 copy/resolve 方法；glyph sequence 继承 text object 的外围 marked-content context 和 page identity。 |
| `GlyphObject.PathsSeq()` | Playa glyph-outline 迭代 | 在 glyph rendering 坐标中惰性解释 Type3 CharProc 和嵌入 CFF/Type2 glyph path。 |
| `GlyphObject.ContentSeq`、`GlyphObject.Len` | Playa `GlyphObject.__iter__` 和继承的 `Sized.__len__` | 惰性解释任意 Type3 CharProc child，而不只 path；返回直接 child 数且不保留遍历。`content.glyphs` 投影比较该 cardinality。 |
| `TextObject.Origin`、`TextObject.Displacement` | Playa 文本对象 geometry | 保留 eager capture 的 device-space geometry；`Displacement` 包含 `TJ` adjustment。 |
| `PathObject.SegmentsSeq` | Playa path segment 迭代 | 保留 device-space segment 的可重复 Go sequence。 |
| `PathObject.GState`、`PathObject.MarkedStackCopy`、`PathObject.PageObject` | Playa path graphics state、marked-content stack 和页面归属 | 保留自有 state/stack 快照和显式页面解析；兼容投影比较 CTM、state、stack 和零基 page index。 |
| `ImageObject.GState`、`ImageObject.MarkedStackCopy`、`ImageObject.PageObject` | Playa image graphics state、marked-content stack 和页面归属 | 保留自有 state/stack 快照和显式页面解析；兼容投影比较 CTM、state、stack 和零基 page index。 |
| `TagObject.GState`、`TagObject.MarkedStackCopy`、`TagObject.PageObject` | Playa marked-content point graphics state、marked-content stack 和页面归属 | 保留自有 state/stack 快照和显式页面解析；兼容投影比较 CTM、state、stack 和零基 page index。 |
| `Page.Flatten`、`XObjectObject.Flatten` | Playa 递归 Form XObject flatten | 保留带 cycle guard、filter、operator restriction 和显式借用对象 finalization 的惰性递归遍历。 |
| `Page.Interp`、`XObjectObject.Interp` | Playa 惰性解释器 sequence | 保留直接内容顺序、Form XObject node、operator restriction 和延迟错误。 |
| `Page.Streams` | Playa `Page.streams` | 保留惰性内容 stream 遍历；兼容 digest 使用解码 byte 而不保留第二份解码副本。 |
| `Page.Tokens` | Playa `Page.tokens` | 保留带延迟 parse error 的惰性页面内容词法 token 遍历。 |
| `Page.XObjects`、`XObjectObject.Tokens` | Playa `Page.xobjects`、`XObjectObject.tokens` | 保留递归惰性 Form 遍历、有界 stream 所有权和嵌套词法 token 迭代。 |
| `Page.ExtGStates`、`ColorSpaces`、`Patterns`、`Shadings`、`Properties` 及 Form 对应项 | Playa 惰性解释期间的资源选择 | 保留带 page/Form 资源继承、自有快照和 graphics-state 投影中已解析 PDF color-space 名的源顺序惰性选择；兼容投影覆盖组合资源 acceptance fixture。 |
| `Page.Contents`、`XObjectObject.Contents` | Playa `Page.contents`、`XObjectObject.contents` | 保留 operation-level 惰性序列；页面内容 stream 解码像 Playa 一样使用有界非严格 Flate/LZW 恢复，显式 stream error 访问器仍严格。兼容投影把每个 operation flatten 为源 operand 加 operator keyword。 |
| `Page.MarkedContentSequence`、`XObjectObject.MarkedContentSequence`、`ContentSequence`、`ContentSection` | Playa `Page.marked_content`、`XObjectObject.marked_content`、`ContentSequence`、`ContentSection` | 保留惰性 MCID-indexed section、空 MCID slot、页面顺序迭代、section 文本投影和显式自有快照；`content.marked` 兼容投影直接覆盖该路径。 |
| `TextObject.Len`、`PathObject.Len`、`ImageObject.Len`、`TagObject.Len`、`XObjectObject.Len`、`ContentObject.Len` | Playa `Sized`/`ContentObject.__len__` 和具体内容 iterator | 保留 leaf 零计数、glyph 数和惰性 Form/generic child 数；Form 解析错误会返回，不被纯数字 API 隐藏。 |
| `XObjectObject.ResourcesCopy`、`XObjectObject.GroupCopy`、`XObjectObject.FontsSeq`、`XObjectObject.StructureSeq`、`XObjectObject.GState`、`XObjectObject.MarkedStackCopy`、`XObjectObject.PageObject` | Playa Form XObject 资源、transparency group、字体、结构、graphics state、marked-content stack 和页面归属 | 保留自有快照和显式页面解析；兼容投影递归解析资源值，并比较资源、group、字体、结构、CTM、state、stack 和零基 page index。 |
| `ContentObject.GraphicsStateCopy`、`MatrixValue`、`MarkedStackCopy`、`Parent` | Playa `ContentObject.gstate`、`ctm`、`mcstack`、`parent` | 通过显式自有 Go 访问器保留基础对象 state 和 ParentTree 关联；兼容投影比较稳定 graphics-state、matrix、marked-stack 和 parent，不保留 Python/Go 字体 identity。 |
| `ContentObject` typed payload 访问器 | 页面迭代时的 Playa 具体内容对象 payload | 保留借用 `TextBorrowed`、`PathBorrowed`、`ImageBorrowed`、`TagBorrowed`、`XObjectBorrowed`、resource 和 properties view；typed `Copy`/`Finalize` 仍是显式所有权边界，`TextObject.ValidateWithError` 保留延迟字体错误而不深复制。 |
| `StructureItem`、`PageStructureEntry` 遍历访问器 | Playa 混合结构 `/K` 值和 ParentTree 页面 slot | 通过 `ElementBorrowed`、`ContentBorrowed` 和 `ElementsSeq` 保留借用嵌套 element/content；copy/finalize 方法是显式所有权边界。 |
| `StructElement.AlternateDescription`、`AbbreviationExpansion` | Playa `Element.alternate_description`、`Element.abbreviation_expansion` | 以惯用 Go 名称暴露 `/Alt`、`/E` scalar property，不保留重复短拼写。 |
| `Page.Layout` 和布局结果 sequence | Playa `miner.extract_page(Page, LAParams)` | 保留惰性 line、textbox、text-group 和混合 page-item 遍历；兼容投影比较每项 bbox 前，在以 `m` 开始的 subpath 处拆分 painted path，并规范化 Playa 末尾 line separator。 |
| `LayoutResult.ItemsSeq`、`LayoutItem` | Playa `LTPage` child 与分析后 textbox | 保留 textbox、image、path、Form XObject 的借用可重复混合遍历；typed copy/finalize 提供所有权，同时保留现有文本投影。 |
| `TextGroup.ChildrenSeq`、`TextGroupChild` | Playa 递归 `LTTextGroup` child | 惰性保留嵌套 group/textbox 顺序；`BoxBorrowed`/`GroupBorrowed` 暴露只读值视图，`BoxesSeq` 是显式 flatten convenience view，`Copy`/`Finalize` 持有完整树。 |
| `ImageObject.Buffer`、`ImageObject.DecodedStreamBufferWithError`、`ImageObject.DecodedStreamBufferDigestWithError`、`ImageObject.Get`、`ImageObject.Has` | Playa image stream 访问 | 保留显式 raw/filtered-stream 访问器和无需 sample 解码的字典查找；兼容 `DecodedStreamBuffer` 使用有界非严格 Flate/LZW 恢复，error-aware/digest 访问器仍严格。严格 DCT 访问会拒绝缺少必需 JPEG SOI 标记的流，而不会按格式嗅探多态字节；兼容路径仍暴露与 Playa 一致的不透明字节。兼容投影使用 digest 访问器比较 filtered-stream 长度与 SHA-256，避免第二份 payload copy。 |
| `Action`、`Document.OpenAction`、`Document.ResolveAction` | Playa outline/annotation action metadata | 保留为不执行 action 的 Go 规范化；`/Next` 链到 `NextSeq`、`NextCopy` 或 `Finalize` 才惰性解析；outline `GoTo` target 复用间接/命名目的地 resolver。 |
| `Action.NextSeq` | Playa 惰性 `/Next` action 遍历 | 保留为可重复 `iter.Seq2`；只在消费者推进时解析并缓存各 array entry，`NextCopy`/`Finalize` 是显式物化点。 |
| `Destination.Top`、`Left`、`Pos`、`BBox`、`Zoom` | Playa destination view geometry | 保留 device-space geometry，包括 XYZ/FitH/FitV 的 anchor-based bound 和显式 FitR rect；命名目的地遵循 Playa `/Dests` 优先于 `/Names/Dests`。 |
| `Page.Label`、`Document.PageLabel`、`PageLabelsSeq` | Playa page-label iterator | 保留惰性 number-tree 遍历和有界物化；保留 style、prefix、start、malformed-tree error 和 Playa 缺失 zero first-rule 恢复。 |
| `documentdata.PageLabelSpec` | Playa page-label rule 值 | `documentdata` 持有无依赖 style/prefix/start 值和标签格式化；document/page/root facade 重导出当前类型，document 持有树遍历和缓存。 |
| `documentdata.XRefEntry` | Playa xref-entry 值 | `documentdata` 持有不可变 xref scalar metadata；document 保留 revision table、parser index、trailer 值和缓存协调。 |
| `documentdata.XRefTable`、`document.XRefTable` | Playa xref revision 快照 | 持有 revision kind/offset、scalar entry、Mapping-visible entry、entry count 和复制 trailer；parser index、revision 遍历和缓存协调留在 `document`。`EntriesSeq`、`VisibleEntriesCopy`、`TrailerCopy`、`Finalize` 定义只读所有权边界。 |
| `documentdata.Annotation`、`document.Annotation` | Playa annotation 值和 `Page.annotations` 结果 | `documentdata` 持有 annotation scalar metadata、primitive 字典、destination 值、geometry、color 及 copy/finalize 行为；`document.Annotation` 保留惰性规范化 action、页面查找、ParentTree 解析和 annotation 缓存。 |
| `documentdata.FormField`、`document.FormField` | Playa AcroForm field 值和字段树结果 | `documentdata` 持有继承 scalar metadata、choice value、widget geometry 和 primitive 字典副本；`document.FormField` 保留惰性 child 遍历、field-reference 链、page/parent 解析和有界 form cache。`KidsSeq`、`KidsCopy`、`FinalizeWithError` 是显式惰性/物化所有权边界。 |
| `content` 文本、glyph、path、image、tag、marked-content、graphics-state、layout 模型 | Playa 内容模型 | 保留；generator 形态输出使用 `iter.Seq`/`iter.Seq2`。 |
| `content.FontResource`、`content.FontMetadata`、`content.PageStructure`、`content.PageStructureEntry` | XObject 字体/结构序列的 Go facade 重导出 | 重导出当前结果类型，使 `XObjectObject.FontsSeq` 和 `Structure`/`StructureSeq` 具有可用领域名称。 |
| `ContentKind` 和 `ContentText`/`ContentXObject` 风格 constant | Playa 内容对象 kind discriminator | 从 root、`document`、`page`、`content` facade 导出，使调用方无需字符串比较即可分类 `Interp`/`Flatten` 结果。 |
| `TokenKind` 和 `TokenEOF`/`TokenLiteral` 风格 constant | Playa 词法 token discriminator | 从 root、`document`、`page`、`content`、`parser` facade 导出，保持共享 constant identity。 |
| `page` 页码标签、annotation 和 form 模型 | Playa 页面/文档模型 | 保留；上游把访问放在 `Page` 时也如此；本地 annotation `GoTo` destination 使用与 outline destination 相同的间接 resolver。 |
| `page.Action`、`page.Destination`、`page.StructElement` | annotation、导航和 ParentTree 结果的 Go facade 重导出 | 重导出页面 annotation、image、tag 和内容对象返回的当前跨领域值。 |
| root/document `StructureContentKind`、structure `ContentKind` 及 `StructureMarkedContent`/`StructureObject` constant | Playa tagged-content item discriminator | 导出领域适用的具名类型和规范 constant，同一包中不保留重复名称。 |
| `font`、`outline`、`structure` 模型 | Playa 模型 | 保留在其公开领域包。 |
| `content`、`outline`、`structure`、`image` 中的跨领域 `Page`/`StructElement` 结果 | 导航、structure-parent、image-placement API 的 Go facade 重导出 | 重导出当前领域名称，避免结果暴露实现包。 |
| `StructElement.Object` | 原始 `/Obj` 结构 node 引用的 Go 适配 | 保留为惰性原始 PDF 对象 resolver；返回复制对象，因为 Playa 另行暴露 typed structure content object。 |
| `StructElement.PageOrderSeq` | Playa `Element.contents.page_order` | 保留按页面分组的 sequence；每个被引用页面只解释一次，以排序 marked-content MCID，并把 OBJR item 放在该页最前。 |
| `pdftypes`、`parser` | Playa 低级 object/content-stream surface 加 Go parser 适配 | 保持公开：PDF primitive、lexer/object parsing、diagnostic 和 filter decoding 是领域包使用的文档化低级 API。 |
| `image.DecodeFilters`、`image.UnpackData` | 上游 image-decoding 能力 | 稳定 image-domain helper；在 `image` facade 中保留严格 filter-chain decoding 和 packed-sample expansion。严格 DCT decoding 要求 JPEG SOI 标记，避免把格式不匹配的多态数据当成有效 DCT stream。 |
| `parser.DecodeFiltersLenientLimited` | Playa `ContentStream.decode(strict=False)` 恢复桥 | 保持 parser-level helper 有界且显式；为宽松 stream 访问保留已解码 Flate/LZW prefix 和不透明 DCT bytes，严格/error-aware 访问器则保留 decode 与 DCT validation error。unknown filter、异常 parameter、predictor failure 和 output-limit violation仍返回错误。 |
| `Compatibility*` | 仓库 oracle adapter | 禁止出现在公开包和 `document`；只允许位于 `internal/testcompat` 和 `cmd/playa-compat`。 |

根包只是精简 facade，不能为方便而重导出每个内部内容类型；领域类型属于其具名包。

Context、必需回调和指针接收器必须非 nil。传入 nil 属于编程错误，库不承诺
sentinel 错误或兜底结果。并发页面 worker 保留现有的回调 panic 恢复机制；nil
回调在实际调用时通过该机制报告失败。函数式页面并发选项仍是可选输入，nil
选项会被忽略。可选模型字段，以及文档明确允许缺省的 resolver/filter 参数，
继续保留各自的 nil 语义。

## 审计命令

清理完成后，以下检查必须保持无输出：

```bash
! go doc github.com/lin-string/go-playa | rg 'Compatibility|ExtractOptions|PageObjects'
! go doc github.com/lin-string/go-playa/content | rg 'ExtractOptions|PageObjects'
! go doc github.com/lin-string/go-playa/page | rg 'PageResult'
! go doc github.com/lin-string/go-playa/font.Font.ApplyEncoding >/dev/null 2>&1
! go doc github.com/lin-string/go-playa/font.Font.ApplyDifferences >/dev/null 2>&1
! go doc github.com/lin-string/go-playa.GraphicState >/dev/null 2>&1
! go doc github.com/lin-string/go-playa/layout.Options.LineTolerance >/dev/null 2>&1
! go doc github.com/lin-string/go-playa/layout.Options.WordGap >/dev/null 2>&1
! go doc github.com/lin-string/go-playa/layout.Options.ParagraphGap >/dev/null 2>&1
! go doc github.com/lin-string/go-playa/structure.MarkedContent >/dev/null 2>&1
! go doc github.com/lin-string/go-playa/structure.Object >/dev/null 2>&1
! go doc github.com/lin-string/go-playa/outline.OutlineNode >/dev/null 2>&1
! go doc github.com/lin-string/go-playa/structure.StructElement >/dev/null 2>&1
! go doc github.com/lin-string/go-playa/structure.StructureIndex >/dev/null 2>&1
! go doc github.com/lin-string/go-playa/structure.StructureContent >/dev/null 2>&1
! go doc github.com/lin-string/go-playa/structure.StructureContentKind >/dev/null 2>&1
! go doc github.com/lin-string/go-playa/document.StructElement.Alt >/dev/null 2>&1
! go doc github.com/lin-string/go-playa/document.StructElement.Abbreviation >/dev/null 2>&1
! python3 scripts/generate_cff_resources.py --help | rg '^[[:space:]]+--source-dir([[:space:]]|$)'
```

新增导出符号前，在本文记录其上游对应项或必要 Go 适配，并添加 API/兼容测试。
