# Playa 迁移契约

[English](migration.md) | 简体中文

## 状态与验收边界

计划内迁移已经完成。对于 [`compat/manifest.toml`](../compat/manifest.toml)
中的每个 section，Playa 用户都能找到对应的 Go 包、类型、方法或序列；每个
section 都是发布兼容门禁的 required 项。

oracle 的包、版本、tag 和 commit 只定义在
[`compat/upstream.toml`](../compat/upstream.toml) 中。检入的生成 PDF 与清单固定的公开 PDF corpus
共同定义验收边界。新增文件可能暴露缺陷或扩展支持的生产器集合，但不会让已完成的台账
重新变成活动迁移阶段。

## 范围

go-playa 实现 Playa 中可复用的 PDF 行为：

- 词法和对象解析、xref 表与流、对象流、增量更新、恢复、加密和流过滤器；
- 文档 mapping、页面、内容流和 token；
- 图形状态、文本、字形、路径、图像、Form XObject、资源、marked content 和
  布局分析；
- 字体、编码、CMap、ToUnicode、嵌入字体程序、度量和轮廓；
- 元数据、名称、目的地、动作、大纲、注释、AcroForm、页码标签和 tagged PDF
  结构；
- 面向内容、大纲、结构、文本、文本对象、图像和字体的实用检查 CLI 模式。

Python 打包、多进程实现细节、notebook、文档站点工具和 benchmark harness
内部实现不在移植范围内。Go benchmark 和兼容命令是仓库工具，不属于 Playa
API surface。

go-playa 还提供有文档记录的 Go 扩展：有界页面并发、`user` 坐标空间、可配置的
有界缓存和独立的有界字形渲染。当固定 oracle 没有公开对应项时，它们由 Go 侧
测试覆盖。

## 接口映射

保留 Playa 的领域名词与可观察行为，同时把控制流适配到 Go：

- Python `snake_case` 名称变为符合 Go 惯例的导出名称。
- 只读属性变为访问器方法。
- 立即发生的失败返回 `(T, error)`。
- generator 变为全新、可重复的 `iter.Seq[T]` 或 `iter.Seq2[T, error]`。
- Python 关键字参数变为 options 值或函数式选项。
- mapping 和索引协议变为具名的 `Get`、`Lookup`、`Keys`、`Values`、`Items`
  或领域专用查找方法。
- 上下文管理器与 finalization 行为变为显式的 `Close`、`Copy`、`Finalize` 或
  具名 copy 访问器。
- 不依赖文档的 `Page` 在需要对象解析或共享缓存时显式接收 `*Document`。

不要仅为兼容旧版 go-playa API 而保留第二种公开拼写。根 facade 和领域 facade
可以把同一个当前类型重导出到其文档化领域；这属于同一个 API 模型，不构成兼容
别名层。每个导出符号都必须有 Playa 对应项或必要的 Go 适配，并记录在
[`api-audit.zh-CN.md`](api-audit.zh-CN.md) 中。

## 公开包归属

根 `playa` 包是精简的打开入口。`document` 是公开文档引擎，负责解析、惰性缓存、
生命周期、页面/对象查找、资源解析和跨领域协调。

面向领域的 facade 位于 `page`、`content`、`font`、`image`、`outline` 和
`structure`。无依赖值与算法归其自然所有者：

| 所有者 | 值和行为 |
| --- | --- |
| `pdftypes` / `pdftypes/primitives` | PDF 标量、数组、字典、流和引用值 |
| `parser` | lexer、对象解析器、文本解码、诊断和流过滤器 |
| `geometry`、`coordinates` | 矩阵、矩形、路径、颜色和坐标空间 |
| `documentdata` | 元数据、加密元数据、页码标签、xref、间接对象、目的地、动作、大纲、注释和表单值 |
| `contentdata` | 内容操作、marked content、图形状态快照、字形/路径/tag/XObject/资源选择值 |
| `fontdata` | CMap、解码字形、字体元数据、真源派生映射数据和字体程序解析 helper |
| `imagedata` | 解码图像、颜色空间标量值和打包样本展开 |
| `structuredata` | 无依赖的 tagged PDF element、content 和 item 值 |
| 配置包 | cache、content、document、parser、structure、text 和 layout options |

根包与依赖文档的领域 facade 导入 `document`；`document` 导入 `parser` 以及
相应的值、几何和配置包。无依赖领域包不能仅为命名某个值而导入 `document`。

## 迭代契约

惰性属于公开行为。与 Playa generator 对应的序列：

1. 保留上游源顺序；
2. 每次 `range` 都开始全新遍历；
3. 不为消费者未请求的条目工作；
4. 消费者退出时立即停止；
5. 推进错误只产生一次，随后停止。

文档页面、对象、token、注释、表单、大纲节点、结构节点以及页面内容/资源都遵循
该契约。`CollectPages`、`CollectObjects`、`CollectAnnotations`、
`CollectFormFields`、`CollectOutline`、`CollectTokens` 等收集 helper 是明确
物化点，不取代主要序列 API。

generator 形态的行为测试覆盖顺序、可重复遍历、提前停止，以及一个必须延迟到
推进时才暴露错误的后续异常条目。

## 所有权与稳定性

Go 没有只读 slice、map 或 pointer 类型，因此公开解析器和模型状态使用以下边界：

- 标量状态通过访问器方法暴露。
- 缓存返回的可变数据与内部状态隔离。
- 高频惰性序列可以产生借用的只读视图。
- 需要保留时，`Finalize`、`Copy`、`ValueCopy` 或具名 `...Copy` 方法创建自有值。
- JSON 投影是显式的，不依赖导出的可变字段。

推进序列、跨越缓存释放边界或离开页面回调后，借用值不能继续保留或修改，除非已
先完成 finalize 或 copy。

## 生命周期与并发

`Open` 或 `OpenBytes` 成功后立即调用 `defer doc.Close()`。`Close` 是幂等的。

同一个已打开文档上的只读操作可以并发执行。文档缓存有同步保护；页面解释器与
临时输出状态局限于页面。`Document.ForEachPageConcurrent(ctx, callback, options...)`
和 `Document.ForEachPageLayoutConcurrent(ctx, layoutOptions, callback, options...)`
通过封闭的 `PageConcurrencyOption` 值提供有界自适应页面调度。
`WithMaxPageWorkers` 和 `WithPagesPerWorker` 分别配置默认自动选择的 worker 数
上限和十页增长预算；非正数值保留对应默认值。类型和构造器由 `document` 和
根 facade 导出。调度受 `GOMAXPROCS` 和页数限制，页面惰性生成，回调完成顺序
不保证固定。context 取消或首个回调错误、被恢复的 panic 会停止剩余工作；
调用会等待正在运行的回调结束。需要在回调返回后保留借用值和布局结果时，先
finalize 或复制。

`Close` 和 `ReleaseTransientCaches` 是互斥的生命周期操作，不能与文档读取、
页面 worker 或借用值使用重叠。

## 错误、恢复与安全

库返回稳定 sentinel 或 typed error，并用 `%w` 包装原因。它不会一边记录错误
一边返回同一错误，也不会把解析失败转换成无法解释的空结果。

oracle 能打开输入时，恢复行为会与固定 oracle 对照测试。oracle 无法安全提供
参考结果时，仅 Go 支持的损坏输入恢复与安全限制保留在本地 corpus 中。异常输入
工作必须维持有界分配、惰性错误交付和缓存计数。

部分解析器恢复有意遵循固定 oracle，而不是最严格的 PDF 解释，包括只按对象号
查找以及部分内容流恢复。用于保护资源上界的有意差异仍属于 Go 侧安全行为，并
记录在测试或 [`compatibility.zh-CN.md`](compatibility.zh-CN.md) 中。

## 字体与映射来源

文档提供的 ToUnicode、嵌入字体映射、CID 度量和显式编码 Differences 具有更高
优先级。Adobe 与 Unicode 规范，以及 fontTools 和 Apache PDFBox 数据，为生成的
fallback 映射提供来源。历史 Playa 表是兼容参考，不会自动成为规范真源。

所有生成的字体、CMap、glyph list、AFM 和 CFF 数据都必须通过同步器与生成器
刷新。来源、优先级和有意差异记录在
[`mapping-sources.zh-CN.md`](mapping-sources.zh-CN.md) 中。

## 变更工作流

新增或修改公开行为时：

1. 确定 Playa module、class、property、method 或 generator，以及对应 Go 包。
2. 添加先失败的行为测试和外部 API 测试；需要时覆盖惰性和所有权。
3. 实现最小兼容行为，不做 eager materialization，也不暴露可变解析器状态。
4. 接受的公开范围或映射发生变化时，更新 `compat/manifest.toml`、本契约或
   `api-audit.md`。
5. 运行 `make verify` 以及相关 oracle/corpus/resource 门禁。

公开提取或投影行为使用 `make compat`；异常/恢复/跨领域行为使用
`make corpus-test`；字体或恢复对照使用 `make compat-fonts` 或
`make compat-recovery`；生成资源变更使用 `make resources-check`。

原 checkbox 台账只作为完成记录保留在
[`migration-todo.zh-CN.md`](migration-todo.zh-CN.md)。
