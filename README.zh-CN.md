# go-playa

[English](README.md) | 简体中文

`go-playa` 是面向高吞吐、惰性文档分析的 Go 原生 PDF 分析器，使用 Go 1.25，
并通过可审计的测试契约对齐 [Playa](https://github.com/dhdaines/playa)。

## 为什么选择 go-playa

- **高吞吐分析。** 惰性读取、共享资源缓存和有界页面并发支持 CPU 密集型 PDF
  任务。性能比较以文档、任务、worker 拓扑和机器为边界，见
  [benchmark 协议](docs/benchmark.zh-CN.md)。
- **Go 原生集成。** 库部署不需要 Python 或跨语言序列化桥接层；领域类型可直接
  出现在应用 API 中。
- **惰性与显式资源控制。** 可重复迭代支持提前停止和延迟错误；缓存预算、
  `Finalize`/`Copy` 和 `Close` 让内存与所有权决策可见。
- **可审计兼容性。** 固定 Playa oracle 的 required 投影、异常输入测试和确定性
  资源生成定义对象、页面、文本、字形、图像及结构的验收边界。

计划内迁移已经完成。[`compat/manifest.toml`](compat/manifest.toml) 中的每个
section 都是 required，检入的生成 PDF 与清单固定的公开 PDF corpus 共同定义发布验收边界。新的生产器特有
文件仍可能暴露缺陷，但这属于兼容性加固，不代表迁移阶段尚未完成。

## 性能证据

[语料](bench/corpus.json)包含九份公开 PDF，覆盖原生文本、图形密集型、
纯扫描、带 OCR 扫描和混合文档，以及英文、中文、阿拉伯文、日文、韩文、法文，
页数从 119 到 1290 页。[矩阵](bench/matrix.json)在适用条件下以 workers 1/2/4/8
比较六类任务，计数、规范摘要和有效 worker 拓扑通过门禁后才计算比值。

2026-09-29 的四个完全通过门禁的全语料单 worker 任务聚合中，go-playa 达到
Playa 吞吐的 **3.36×–5.56×**。Reference 的 77 个单元（770 个样本）中 73 个
具备倍率资格；workers 1/2/4/8 scaling 的 89 个单元（890 个样本）中 81 个具备
资格。OpenIntro `text-glyph-jsonl` 与 RISC-V `image-digests` 因规范输出不同被
阻断，其计时不用于跨库倍率。

参见[完整日期矩阵](docs/benchmark-results-2026-09-29.zh-CN.md)与
[benchmark 协议](docs/benchmark.zh-CN.md)。这些是交互式桌面运行，并非实验室
隔离测量。吞吐量与内存取决于具体任务；速度比较不能直接推导出更低峰值 RSS。

## 安装并打开文档

```bash
go get github.com/lin-string/go-playa
```

```go
package main

import (
	"fmt"

	"github.com/lin-string/go-playa"
)

func main() {
	doc, err := playa.Open("example.pdf")
	if err != nil {
		panic(err)
	}
	defer doc.Close()

	fmt.Println(doc.PageCount())

	for page, err := range doc.Pages() {
		if err != nil {
			panic(err)
		}
		text, err := page.ExtractText(doc, playa.DefaultTextExtractionOptions())
		if err != nil {
			panic(err)
		}
		fmt.Print(text)
	}
}
```

`OpenBytes` 解析内存中的 PDF。`Open` 和 `OpenBytes` 接受 `WithPassword`、
`WithCoordinateSpace`、`WithCacheOptions` 等函数式选项。只调整某一项预算时，
请先复制默认缓存配置；将某项设为零会禁止保留对应缓存，但不会改变解析结果：

```go
cache := playa.DefaultCacheOptions()
cache.PageBytes = 0

doc, err := playa.Open("example.pdf", playa.WithCacheOptions(cache))
```

## CID-aware 字形渲染

go-playa 可以恢复并绘制单个 PDF glyph 实际使用的嵌入轮廓。它适用于 CID 字体
诊断、OCR 数据集生成、字体检查，以及排查仅靠提取后的 Unicode 文本无法确定页面
实际绘制字形的文档。

轮廓提取遵循 PDF 自身的映射和放置状态：

- Type 0/CIDFontType2 字体遵循 encoding CMap 和显式 `CIDToGIDMap` stream；
  identity mapping 保留直接 CID-to-GID 行为。
- CID CFF/CFF2 字体支持 FDSelect 专用 local subroutine 和 CFF2 variation store
  计算。Type1、Type3 CharProc 和嵌入 TrueType 轮廓使用相同的公开 glyph path 模型。
- `GlyphObject.PathsSeq()` 惰性产生 device-space path。`glyphrender` 包可写出独立
  SVG 或有界内存 PNG；`WriteJSONL` 和 `ExportGlyph` 会流式输出 CID、GID、字体、
  geometry 及选定的渲染结果。
- 渲染要求字体包含可执行的嵌入轮廓。没有轮廓时，path sequence 为空，SVG/PNG
  渲染返回 `glyphrender.ErrNoOutline`；go-playa 不会静默替换为系统字体。

借用的 glyph 如果要在页面迭代外保留或渲染，应先 finalize：

```go
import (
	"errors"
	"io"

	playa "github.com/lin-string/go-playa"
	"github.com/lin-string/go-playa/glyphrender"
)

func renderFirstGlyph(output io.Writer, doc *playa.Document, page playa.Page) error {
	for glyph, err := range page.Glyphs(doc) {
		if err != nil {
			return err
		}
		glyph = glyph.Finalize()
		err = glyphrender.RenderSVG(output, glyph, glyphrender.SVGOptions{Scale: 2})
		if errors.Is(err, glyphrender.ErrNoOutline) {
			continue
		}
		return err
	}
	return glyphrender.ErrNoOutline
}
```

映射优先级、支持的嵌入程序和 CID collection 数据见
[字体与字符映射来源](docs/mapping-sources.zh-CN.md)。

## 对齐基线

oracle 的规范身份（包、版本、tag 和源代码 commit）只定义在
[`compat/upstream.toml`](compat/upstream.toml) 中。README 不复制具体版本号。
两个兼容驱动都会读取该文件，缓存键包含其身份，测试还会要求 Python 依赖 pin
与之相符。

[`compat/manifest.toml`](compat/manifest.toml) 把每个比较 section 映射到 Playa
和 Go 的公开 API。[`docs/compatibility.zh-CN.md`](docs/compatibility.zh-CN.md)
说明投影、corpus、容差、恢复案例和缓存格式。

```bash
make public-fixtures-pull
make compat
make compat-local
make compat-release
COMPAT_NO_CACHE=1 make compat-one \
  COMPAT_PDF=testdata/files/form_simple.pdf
```

“对齐”表示 required 投影在检入的生成语料和固定的公开语料上相符，同时满足 Go 侧对顺序、惰性、
所有权、异常输入和生命周期的测试；它不表示复制 Playa 的实现细节或 Python
运行时基础设施。

兼容性默认采用严格判定。只有经过审核确认 go-playa 遵循适用的 PDF 语义、且差异
源于 Playa 已确认的缺陷时，结果才可以有意偏离固定的 Playa oracle。这类差异不是
宽泛的 xfail：每项都必须关联上游 issue，并精确绑定 fixture 和实际观测到的差异；
新增、变化、重叠或消失的差异都会令测试失败并要求复审。普通 corpus 的完整审核
清单见 [`compat/known_differences.json`](compat/known_differences.json)，PDF
Association 用例见
[`internal/testfixture/pdf_association_fixtures.json`](internal/testfixture/pdf_association_fixtures.json)
中的 `differences` 和 `upstream_issues`；完整校验规则见
[兼容性协议](docs/compatibility.zh-CN.md)。

## Playa 到 Go 的 API 映射

公开领域名词和行为遵循 Playa，控制流和命名遵循 Go。完整符号台账见
[`docs/api-audit.zh-CN.md`](docs/api-audit.zh-CN.md)。

| Playa 约定 | Go 约定 | 示例 |
| --- | --- | --- |
| `snake_case` 公开名称 | 导出的 Go 名称 | `Page.extract_text` → `Page.ExtractText` |
| 只读属性 | 访问器方法 | `Element.alternate_description` → `StructElement.AlternateDescription()` |
| generator 或惰性迭代器 | 可重复的 `iter.Seq` 或 `iter.Seq2` | `Document.pages` → `Document.Pages()` |
| 推进时可能失败的迭代器 | `iter.Seq2[T, error]` | `Page.tokens` → `Page.Tokens(doc)` |
| 关键字参数 | options 值或函数式选项 | `space=` → `WithCoordinateSpace(...)` |
| mapping 协议 | 具名 mapping 方法 | `Document.get/keys/values/items` → `Get/Keys/Values/Items` |
| 索引访问 | 领域专用查找 | 页索引 → `Document.PageAt(index)` |
| 上下文管理器 | 显式生命周期 | `with open(...)` → `Open` 加 `defer Close()` |
| `finalize()` 或需保留的模型 | 显式所有权操作 | `value.Finalize()`、`Copy()` 或 `...Copy()` |

不会仅为兼容旧版 go-playa 拼写而保留重复名称。每个公开符号都必须对应 Playa
符号或有文档记录的 Go 适配。根 facade 和领域 facade 可以重导出同一个当前
类型，例如根入口 `playa.Document` 与公开引擎 `document.Document`；这些导出
属于同一个 API 模型，不构成兼容别名层。

### 有意保留的接口差异

- 立即执行的操作返回 `(T, error)`；只有推进数据源时才能发现的错误由
  `iter.Seq2` 产生，并终止该次遍历。
- `Page` 是小型值，因此依赖文档的页面操作显式接收所属 `*Document`。Python
  Playa 可以通过对象引用携带这类上下文。
- Go options 取代 Python 关键字参数。零值行为和默认构造器由各选项包记录。
- 高频内容序列中的值可能是借用的只读视图。需要跨迭代、goroutine 或缓存释放
  保存时，应先调用 `Finalize`、`Copy` 或具名 copy 访问器。
- `Close` 和 `ReleaseTransientCaches` 是显式、互斥的生命周期操作。调用前必须
  完成所有文档读取。
- Go API 额外提供有界页面并发、`user` 坐标空间和独立字形渲染；当固定的 Playa
  oracle 没有对应能力时，这些功能由 Go 侧测试单独覆盖。
- Python 打包、多进程实现细节、notebook 和文档站点工具不在移植范围内。

字体和字符映射采用 Adobe 与 Unicode 规范，以及持续维护的 fontTools 或 Apache
PDFBox 数据，而不会把历史 Playa 表视作最终权威。优先级和有意差异见
[`docs/mapping-sources.zh-CN.md`](docs/mapping-sources.zh-CN.md)。

## 公开包

打开文档时导入根包；当领域类型出现在你自己的 API 中时，导入对应领域包。

| 包 | 职责 |
| --- | --- |
| `playa` | 精简的 `Open`/`OpenBytes` facade 和常用入口类型 |
| `document` | 文档解析、对象查找、页面、惰性缓存、生命周期和跨领域协调 |
| `page` | 面向页面的内容、注释、表单、资源和结构模型 |
| `content` | 文本、字形、路径、图像、marked content、资源选择和布局 facade |
| `font`、`image` | 面向文档的字体/CMap，以及图像/颜色空间 |
| `outline`、`structure` | 导航和 tagged PDF 语义模型 |
| `pdftypes`、`parser` | PDF 原始值、词法解析、对象解析和流过滤器 |
| `geometry`、`coordinates` | 矩阵、矩形、路径、颜色和坐标空间策略 |
| `documentdata`、`contentdata`、`fontdata`、`imagedata`、`structuredata` | 无依赖的自有值模型和真源派生数据 |
| `cacheconfig`、`contentconfig`、`documentconfig`、`parserconfig`、`structureconfig`、`textconfig`、`layout` | 无依赖配置值 |
| `glyphrender` | Go 特有的有界 SVG/PNG 和 JSONL 字形导出 |

根包与依赖文档的领域 facade 导入 `document`；`document` 导入 `parser` 以及
相应的值、几何和配置包。这些底层包不会仅为命名一个无依赖值而导入 `document`。

## 惰性与所有权

与 Playa generator 对应的 API 始终保持惰性。每次遍历都是全新的，保留源顺序，
消费者停止时立即停止，也不会暴露从未请求的条目所产生的错误。`CollectPages`、
`CollectObjects` 等收集 helper 是明确的物化点，不会取代序列 API。

高频序列产生的值可能是借用的只读视图，其生命周期在遍历推进时结束。需要跨越
迭代或缓存释放保留，或者传给另一个 goroutine 时，应先调用对应的 `Finalize`、
`Copy`、`ValueCopy` 或具名 copy 操作。

## 并发保证

- 多个 goroutine 可以共享同一个已打开的 `Document` 执行只读操作。共享索引和
  资源缓存有同步保护；解释器状态属于各自的遍历。
- `ForEachPageConcurrent(ctx, callback, options...)` 提供有界自适应页面调度。
  不传选项时自动选择 worker 数上限，增长预算为每个潜在 worker 十页。
  调度始终受 `GOMAXPROCS` 和页数限制；`WithMaxPageWorkers` 和
  `WithPagesPerWorker` 分别配置上限和增长预算。
- 不保证回调完成顺序。首个回调错误或被恢复的回调 panic 会取消剩余工作，并由
  并发页面 API 返回。context 取消会停止后续调度；调用会等待正在运行的回调
  结束后再返回。
- 回调中的借用值仍然是只读且遍历期有效的。另一个 goroutine 需要保留该值时，
  必须先复制或 finalize。
- `Close` 是幂等的。`Close` 和 `ReleaseTransientCaches` 是互斥的生命周期操作，
  必须等待所有读取和页面回调结束后再调用。

省略选项即可使用自适应默认值，也可以显式配置：

```go
ctx := context.Background()
err = doc.ForEachPageConcurrent(ctx, func(page playa.Page) error {
	_, err := page.ExtractText(doc, playa.DefaultTextExtractionOptions())
	return err
}, playa.WithMaxPageWorkers(4), playa.WithPagesPerWorker(10))
if err != nil {
	panic(err)
}
```

文档所有者继续像打开文档的示例那样 defer `Close`，因此它只会在并发调用返回后
执行。不要从页面回调中调用 `Close` 或 `ReleaseTransientCaches`。`playa` 和
`document` 都导出封闭的 `PageConcurrencyOption` 类型及其构造器。非正数上限
保留自动选择；非正数页面预算使用十页。nil 选项会被忽略，后面的选项优先。
`ForEachPageLayoutConcurrent(ctx, layoutOptions, callback, options...)` 使用
相同选项；需要在回调返回后保留布局结果时，先 finalize 或复制。
缓存和所有权示意见[领域模型指南](docs/core-domain-model.zh-CN.md)。

## 命令行工具

`playa` 命令提供从 Playa 迁移而来的常用文档检查模式：

```bash
go run ./cmd/playa -text example.pdf
go run ./cmd/playa -outline example.pdf
go run ./cmd/playa -structure example.pdf
go run ./cmd/playa -images example.pdf
```

运行 `go run ./cmd/playa -help` 查看完整参数。

## 许可证与项目独立性

go-playa 采用 MIT 许可证；[LICENSE](LICENSE) 同时保留了本实现、Playa 和
pdfminer.six 的版权及许可证声明。随项目分发的其他第三方材料仍受
[NOTICE](NOTICE) 和 [THIRD_PARTY_LICENSES](THIRD_PARTY_LICENSES) 所列各自条款约束。

go-playa 是独立的 Go 实现，并非 Playa 官方项目。兼容性 oracle 的包名、版本、
标签与源代码提交仍仅由 [compat/upstream.toml](compat/upstream.toml) 定义。

## 开发

请先阅读[贡献指南](.github/CONTRIBUTING.md)和[安全政策](.github/SECURITY.md)。
政策与契约文档的中英文内容如有差异，以英文版为准。

最低语言和标准库基线为 Go 1.25。CI 使用 [`.go-version`](.go-version) 固定的
已更新安全补丁的工具链。

```bash
pre-commit install
make verify
```

常用定向门禁：

```bash
make test
make test-race
make api-audit
make vuln-check
make module-archive-check
make module-source-test
make public-fixtures-pull
make public-fixtures-check
make compat-public
make compat-local
make pdfa-fixtures-pull
make corpus-test
make compat-fonts
make compat-recovery
make resources-check
make bench-compare COMPAT_PDFS=testdata/files/form_simple.pdf
```

`make verify` 检查格式、module tidiness、`go vet`、lint、普通测试、race 测试和
公开 API 审计。异常输入、恢复或跨领域变更使用 `make corpus-test`。资源表必须
通过真源同步器和生成器更新，不能直接编辑生成的 Go 文件。

`make vuln-check` 是单独运行、需要网络的发布门禁。它根据当前漏洞数据库检查 Go
代码及锁定的 Python 兼容环境；CI 同样执行这些检查。

`testdata/files` 只保留普通 Git 管理的小型生成 PDF fixture。大型公开 PDF
需要显式运行 `make public-fixtures-pull`，下载到 `.compat-cache/public-corpus`；清单
固定真源、许可证、大小与 SHA-256，PDF 不随 Go module 分发。
`make public-fixtures-check` 离线校验本地文件。普通 `go test ./...` 和 `make
verify` 不会下载这些文件。`make module-archive-check` 检查源码归档体积，`make
module-source-test` 验证公开源码模式；二者均包含在 `make verify` 中。
可选的大型 fixture 缺失时，相关测试会明确跳过；要运行这部分覆盖，应像 CI 一样
先执行 pull 和 check，再执行 `make verify`。

详细维护约定：

- [`docs/engineering.zh-CN.md`](docs/engineering.zh-CN.md)：API、测试、错误、缓存和依赖规则。
- [`docs/migration.zh-CN.md`](docs/migration.zh-CN.md)：已完成迁移的范围和持续生效的行为契约。
- [`docs/compatibility.zh-CN.md`](docs/compatibility.zh-CN.md)：固定 oracle 和 corpus 工作流。
- [`docs/benchmark.zh-CN.md`](docs/benchmark.zh-CN.md)：固定语料、任务/worker 矩阵、正确性门禁和下一轮流程。
- [`.github/pull_request_template.md`](.github/pull_request_template.md)：评审和验证清单（英文）。

兼容缓存和 benchmark 快照是本地产物，不会提交到仓库。
