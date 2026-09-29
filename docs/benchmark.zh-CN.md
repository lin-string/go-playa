# Benchmark 协议

[English](benchmark.md) | 简体中文

## 已发布结果

Reference 与 scaling 结果已于 2026-09-29 记录，见
[完整日期矩阵](benchmark-results-2026-09-29.zh-CN.md)。Reference 的 77 个单元
中有 73 个具备倍率资格；scaling 的 89 个单元中有 81 个具备资格。其余单元仍
展示，但因稳定的任务正确性签名差异而被阻断。检入的 smoke fixture 仍只用于
验证编排与正确性，其小型输入耗时不能作为性能证据。

性能比较必须注明运行日期、机器、输入、任务和 worker 拓扑。该日期结果来自
交互式桌面而非实验室隔离机器。更快执行不能推导出更低内存。oracle 的包、版本、tag 和源码 commit 只定义在
[`compat/upstream.toml`](../compat/upstream.toml)；每次真实运行的环境产物会
记录该契约及实际安装的 oracle 身份。

## 语料与准入

[`bench/corpus.json`](../bench/corpus.json)独立于兼容性语料。九份固定文档
共 4754 页、720048399 字节（686.69 MiB），覆盖五种文档类型、六种语言，
50-199、200-499、500-999、1000-plus 四种页数层级，以及全部四种大小层级。
当前全部输入均未加密，只有中文手稿超过 200 MiB。

| Fixture | 页数 | 字节数 | 类型 / 文本层 | 语言 | 页数层级 / 大小层级 | PDF / 生产器 |
| --- | ---: | ---: | --- | --- | --- | --- |
| `riscv-unprivileged` | 696 | 4580174 | born-digital-graphics / native | en | 500-999 / under-10-mib | 1.4 / Asciidoctor PDF / Prawn |
| `gnu-emacs` | 804 | 3084965 | born-digital-text / native | en | 500-999 / under-10-mib | 1.7 / TeX / pdfTeX |
| `openintro-statistics` | 465 | 20947445 | mixed / native | en | 200-499 / 10-49-mib | 1.5 / macOS Quartz PDFContext / appended LaTeX |
| `shuying-siku-quanshu` | 460 | 528780070 | image-only-scan / none | zh-Hant | 200-499 / 200-mib-plus | 1.5 / PDFPatcher / iTextSharp |
| `muqaddimah-1900` | 596 | 87637786 | image-only-scan / none | ar | 500-999 / 50-199-mib | 1.6 / Internet Archive / LuraDocument PDF v2.65 |
| `iroha-jiruisho-japanese` | 119 | 37933566 | image-only-scan / none | ja | 50-199 / 10-49-mib | 1.5 / Adobe Acrobat 6.0 Image Conversion |
| `korean-school-reader` | 154 | 28525153 | ocr-scan / ocr | ko, zh-Hant | 50-199 / 10-49-mib | 1.6 / Adobe Acrobat 9 Image Conversion |
| `murat-coeur-fervent` | 170 | 4376195 | ocr-scan / ocr | fr | 50-199 / under-10-mib | 1.7 / ocrmypdf / Tesseract / pikepdf |
| `gnu-libc-manual` | 1290 | 4183045 | born-digital-text / native | en | 1000-plus / under-10-mib | 1.5 / Texinfo / TeX |

纯扫描件中的阿拉伯文 RTL、中文/日文竖排描述可见页面内容，不能覆盖提取后的
RTL 或竖排字形语义。韩文读本在 154 页中有 149 页 OCR 文本，法文扫描件在
170 页中有 133 页 OCR 文本。OpenIntro 混合原生文本、栅格/矢量图形及不同
生产器的内容，没有归类为扫描件。语料覆盖这些具体输入，不能代表所有生产器
或 PDF 变体。

准入要求权威来源页面、明确许可或公有领域依据、署名、HTTPS 下载地址、精确
字节数、SHA-256、解析页数、PDF 版本、加密状态、文本层、字体、生产器及功能
分类。完整 pin 和检查记录存于清单。允许生产器使用变化中的下载地址，但必须
由精确 hash/大小保护；文件变化会失败，不会静默更新证据。新增六份文档经过
检查，并由 Go 与固定 Playa 打开；不计时的首/中/末页检查属于准入证据，
不能证明全文提取等价。

| Fixture | 来源与许可依据 | 署名 |
| --- | --- | --- |
| `riscv-unprivileged` | [来源](https://docs.riscv.org/reference/isa/v20260120/unpriv/unpriv-index.html); [CC-BY-4.0](https://creativecommons.org/licenses/by/4.0/) | RISC-V International and credited contributors; preserve page 20 author/license notices. |
| `gnu-emacs` | [来源](https://www.gnu.org/software/emacs/manual/); [GFDL-1.3-or-later with invariant sections and cover texts](https://www.gnu.org/licenses/fdl-1.3.html) | Free Software Foundation, 1985-2026; preserve embedded GFDL, GNU Manifesto, Distribution, GPL and cover notices. |
| `openintro-statistics` | [来源](https://www.openintro.org/book/os/); [CC-BY-SA-3.0](https://github.com/OpenIntroStat/openintro-statistics/blob/master/LICENSE.md) | David Diez, Mine Cetinkaya-Rundel, Christopher D. Barr / OpenIntro, 2019; retain original author/image credits and ShareAlike notices. |
| `shuying-siku-quanshu` | [来源](https://commons.wikimedia.org/wiki/File:%E6%9B%B8%E5%BD%B1%E5%9B%9B%E5%BA%AB%E5%85%A8%E6%9B%B8%E6%9C%AC.pdf); [CC0-1.0](https://creativecommons.org/publicdomain/zero/1.0/) | Zhou Lianggong; Siku Quanshu manuscript; Chinese University of Hong Kong Library / Shuge; Commons upload by Ghren. |
| `muqaddimah-1900` | [来源](https://commons.wikimedia.org/wiki/File:%D9%85%D9%82%D8%AF%D9%85%D8%A9_%D8%A7%D8%A8%D9%86_%D8%AE%D9%84%D8%AF%D9%88%D9%86_(%D8%A7%D9%84%D9%85%D8%B7%D8%A8%D8%B9%D8%A9_%D8%A7%D9%84%D8%A3%D8%AF%D8%A8%D9%8A%D8%A9%D8%8C_1900).pdf); [Public domain (PD-Lebanon / PD-old-100; pre-1931 publication)](https://creativecommons.org/publicdomain/mark/1.0/) | Ibn Khaldun (1332-1406), al-Matbaah al-adabiyah 1900 edition; digitization/source credited on Commons. |
| `iroha-jiruisho-japanese` | [来源](https://commons.wikimedia.org/wiki/File:WUL-ho02_00596_%E8%89%B2%E8%91%89%E5%AD%97%E9%A1%9E%E6%8A%84_1.pdf); [Public domain (PD-old-70; 1827 manuscript of a medieval work)](https://creativecommons.org/publicdomain/mark/1.0/) | Tachibana Tadakane; Mitsutomi's 1827 Kyoto manuscript; Waseda University Library ho02/ho02_00596; retain original collection notices. |
| `korean-school-reader` | [来源](https://commons.wikimedia.org/wiki/File:%E5%9C%8B%E6%B0%91%E5%B0%8F%E5%AD%B8%E8%AE%80%E6%9C%AC.pdf); [Public domain (PD-Art / PD-old-70)](https://creativecommons.org/publicdomain/mark/1.0/) | Joseon Ministry of Education editorial bureau (1895); National Library of Korea; Commons upload by AstrobluePeter (2016). |
| `murat-coeur-fervent` | [来源](https://commons.wikimedia.org/wiki/File:Murat_-_D%E2%80%99un_c%C5%93ur_fervent%2C_1908.pdf); [Public domain (PD-US-expired / PD-old-80; author died 1940)](https://creativecommons.org/publicdomain/mark/1.0/) | Amelie Murat (1882-1940), E. Sansot et Cie, Paris 1908; LeDeuxiemeTexte collection; OCR/crop by Commons contributor Cunegonde1 (2024). |
| `gnu-libc-manual` | [来源](https://sourceware.org/glibc/manual/latest/html_node/index.html); [GFDL-1.3-or-later with invariant sections and cover texts](https://www.gnu.org/licenses/fdl-1.3.html) | Free Software Foundation and GNU C Library manual contributors. |

下载的 PDF 保留在 `.compat-cache/benchmark-corpus`，不随 Go module 分发。
再分发这些文档时应保留原始作者、图片、馆藏声明，以及 GNU 手册的不可变章节
和封面文字。项目 MIT 许可证不适用于这些 PDF。[NOTICE](../NOTICE)记录来源
及署名索引，清单链接完整许可条款，无需重复许可正文。大型扫描件不加入
`make compat`。

## 矩阵 preset

[`bench/matrix.json`](../bench/matrix.json)按 fixture × 适用任务 × 请求
worker 数定义确定性单元。一个 sample 是一种实现的一个独立新进程；每次重复
都运行两种实现。

| Preset | 输入 | 请求 workers | 每种实现的重复次数 | 单元数 | 独立新进程 sample 数 |
| --- | --- | --- | ---: | ---: | ---: |
| `smoke` | 两份检入的单页文本/图像 fixture | 1, 2 | 1 | 14 | 28 |
| `reference` | 全部九份公开 PDF | 1, 4 | 5 | 77 | 770 |
| `scaling` | 每种文档类型的一份代表 | 1, 2, 4, 8 | 5 | 89 | 890 |

顺序任务在 [`scripts/benchmark_manifest.py`](../scripts/benchmark_manifest.py)
的 `SEQUENTIAL_TASKS` 中显式声明。目前 `objects` 每份输入仅展开一次，
requested_workers=1，即使 preset 只列出更大的 worker 数。页面任务使用
preset 中的 workers。新增顺序任务时应扩展这一拓扑契约；顺序行不进入扩展性视图。

scaling 选择 GNU Emacs、RISC-V、OpenIntro、Muqaddimah 和韩文读本。展开时
拒绝 reference 覆盖缺失、scaling 类型缺失/重复、重复单元、不安全路径和执行
未完成准入的候选。fixture 按 ID 排序，任务按声明顺序，worker 数递增。
任务适用性来自功能标签，不依赖文件名。以上是当前完整 preset 数量，显式
过滤会生成较小的运行集合。

## 任务与适用条件

| 任务 | 适用输入 | 必须一致的计数 | 额外摘要 |
| --- | --- | --- | --- |
| `open-pages` | 全部 | pages | 无 |
| `objects` | 全部 | pages, objects | 无 |
| `text-glyphs` | `text` 标签 | pages, texts, glyphs | 无 |
| `layout-items` | `text` 标签 | pages, layout_lines, layout_items | 无 |
| `image-digests` | `images` 标签 | pages, images | 规范解码图像流摘要 |
| `text-glyph-jsonl` | `text` 标签 | pages, texts, glyphs | 规范文本/字形逐页 JSON 对象投影摘要 |

仅比较计数的任务不包含提取结果的 JSON 序列化。`text-glyph-jsonl` 包含规范
投影的序列化和 hash，是端到端任务。尽管名称带有 `jsonl` 后缀，摘要输入是
按页序直接拼接的规范逐页 JSON 对象，记录之间没有换行、长度前缀或其他分隔符，
因此并非 JSON Lines framing。`image-digests` 包含图像解码和 hash，
不比较渲染后的像素。所有 sample runner 都输出一行 JSON 结果，与被测任务的
输出方式相互独立。

## 运行命令

从仓库根目录运行。准备与离线规划：

```bash
make bench-contract-test bench-contract-check
make bench-matrix-check
make bench-dry-run BENCH_PRESET=reference
make bench-dry-run BENCH_PRESET=scaling
```

dry-run 输出单元、sample 命令和清单摘要，不构建、不启动 sample 进程、不解析
PDF，也不创建运行目录。下一轮真实运行前，显式下载文档并安装依赖：

```bash
uv sync --frozen --project compat
make bench-corpus-pull
make bench-corpus-check
```

`make bench-corpus-pull` 调用 `python3 scripts/benchmark_manifest.py pull`；
`make bench-corpus-check` 调用 `python3 scripts/benchmark_manifest.py check`。
`pull` 下载并验证精确字节数/SHA-256，`check` 离线验证已有文件。清单检查
验证 metadata 和本地 fixture hash。真实运行前，launcher 还会在不计时的
preflight 中使用固定 oracle 打开每份选中文档，验证解析页数。oracle sample
使用 `uv run --offline --frozen --project compat`，避免测量期间解析或下载依赖。
benchmark 执行及其合同测试只作为本地门禁。`make verify` 运行全部 benchmark
Python 合同测试及离线 corpus/matrix metadata 检查；这些检查仅验证清单和已检入
fixture 身份，不下载 benchmark 输入，也不执行 benchmark。

以下命令可复现执行流程。每次运行选用新的空目录：

```bash
make bench-smoke BENCH_OUTPUT=.compat-cache/benchmarks/next-smoke
make bench-reference BENCH_OUTPUT=.compat-cache/benchmarks/next-reference
make bench-scaling BENCH_OUTPUT=.compat-cache/benchmarks/next-scaling
make bench-summarize BENCH_OUTPUT=.compat-cache/benchmarks/next-reference
```

可选的显式过滤仍然遵守适用条件与完整门禁：

```bash
make bench-dry-run BENCH_PRESET=reference \
  BENCH_MATRIX_ARGS="--fixture gnu-emacs --task text-glyphs --worker 1"
python3 bench/run_matrix.py --preset reference \
  --fixture gnu-emacs --task text-glyphs --worker 1 \
  --output .compat-cache/benchmarks/next-filtered
python3 bench/summarize.py --directory .compat-cache/benchmarks/next-filtered
```

`--fixture`、`--task`、`--worker` 可重复传入。`--corpus-directory` 指定
下载目录，`--go-binary` 复用已经构建的 sample 程序。避免机器上其他任务干扰，
记录文件系统缓存条件，并在一次运行内保持一致。本协议不会自动预热或清空文件
系统缓存。将带日期数字写入 README 前应审核完整、通过门禁的表格与生成的证据视图。

## 计时与 worker 拓扑

每个 Go/Playa sample 都使用独立新进程，且仅打开 PDF 一次。`elapsed_ns` 从
打开文档前开始，到关闭文档完成后结束，包含解析、提取、worker 启停，以及适用
任务的序列化/hash。构建、依赖安装、preflight、环境采集、结果打印及关闭后的
RSS/分配量采集均在计时边界外。使用单调时钟；奇数次重复先 Go 后 Playa，偶数
次先 Playa 后 Go。

`requested_workers` 是显式命令上限。`effective_workers` 是 CPU/页数约束
后的原生调度上限。`observed_workers` 在 Go 中表示最大同时回调数，在 Playa
中表示参与回调的不同进程 ID 数；二者含义不同。Go 使用同一文档上的 goroutine，
把 `GOMAXPROCS` 设为请求值，并将有效页面 worker 上限限定为 ceil(pages/10)。
Playa 将请求限定在 CPU 数内，并遵循固定版本的原生页面加载/进程池阈值。
结果使用实际 callback PID 证据：出现父进程以外的回调 PID 时，effective
报告配置的进程池上限；回调全部在父进程时报告 effective_workers=1，不从
运行后的页数推断进程池是否参与。`objects` 请求一个 worker，其
effective/observed workers 始终为 1。所有 worker 完成后才关闭文档。

有效拓扑不同会保留耗时行，但比值设置为 `speedup: null`，原因是
`effective-worker-mismatch`。顺序矩阵单元显式请求 1，不受外围 Make worker
配置影响。`bench-sequential` 也强制 `BENCH_WORKERS=1`；正式性能证据
使用矩阵 preset。

`peak_rss_bytes` 只有在完整进程范围可用时才表示生命周期峰值。Go 报告一个
进程的峰值。Playa 仅在全部回调都运行于父进程时，报告关闭后采集的父进程峰值，
并标注 `rss_scope: single-process-lifetime-peak`；这种单进程情况允许直接比较。
零表示不可用，不是零内存。

当回调运行于 worker 进程时，Playa 报告 `peak_rss_bytes=0`，并标注
`rss_scope: parent-lifetime-and-worker-callback-lower-bound`。
`parent_peak_rss_bytes` 单独记录关闭后父进程峰值，
`callback_worker_peak_rss_sum_bytes` 求和各参与 worker 在回调中观测到的最大值。
回调在任务编码后采样，但此时还未返回或进行 IPC 序列化，可能遗漏后续 IPC、
清理及未观测 worker 的峰值。这一诊断和不是完整生命周期峰值，也不是同时刻
总内存。公开 page-map API 不提供各 worker 退出时的 rusage。worker 回调采样
位于任务计时内，父进程关闭后采样位于计时外。针对这些不可比的多进程范围，
禁止跨库内存比值或更低内存主张。汇总将完整 Playa RSS 标记为不可用，并单独
标注下界诊断量。

Go 的 `alloc_bytes` 是 sample 内累计分配量，不是峰值内存。缓存保留可能增加
RSS，并加速后续读取；应通过 profile 调查，按具体测量限定内存结论。

## 正确性门禁

字节/hash 身份、解析页数、完整 sample 数量/顺序、精确的运行/fixture/清单身份，
以及任务签名全部通过后才能发布 speedup。必需计数字段必须显式给出非负整数，
不能将缺失字段当成零。重复 JSON key、非有限数、额外 stdout、非法 worker 拓扑
和非正耗时都会失败。每种实现的重复签名以及不同 workers 的签名必须稳定。
sample 不足、实现内部签名漂移或过期的 corpus/matrix 摘要会拒绝汇总生成。

稳定的 Go/Playa 跨实现签名差异会生成 `partial-correctness-mismatch` 汇总，而不
丢弃其他单元。受影响的 fixture/task 保留双方签名，标记
`task-specific-correctness-mismatch`，并将全部 worker 数的跨库倍率置空。包含它
的聚合以及双方相对单 worker 的 scaling 倍率也会阻断。summary 文件仍会写出，
但 CLI 返回非零，避免自动化把部分结果当成完全通过。计数只能证明
`count-equivalence`，不能证明完整语义等价。

`text-glyph-jsonl` 还比较按页序、无分隔符拼接的规范逐页 JSON 对象的 SHA-256
（`canonical-digest-and-counts`）。几何使用 0.01 PDF 用户单位十进制网格和显式
中点规则，避免独立运行时因浮点 ULP 噪声落在边界两侧。`image-digests` 使用
`canonical-image-digest-and-counts`：聚合 SHA-256 从
`playa-image-digests-v1` 加 NUL 开始，每张图像贡献一个 56 字节 frame：
从零开始的页索引、页内图像索引和解码流长度，以三个大端 uint64 表示，随后是
解码图像流 SHA-256 的 32 个原始字节。frame 按页/图像顺序排列，不受 worker
完成顺序影响。仅保留紧凑 frame，不保留图像数据。摘要验证的是解码流投影，
不是渲染或完整图像语义。摘要缺失或不同会令重复、worker 和跨库比较全部失败。

## 产物与统计

每个新 `.compat-cache/benchmarks/<run-id>/` 包含：

| 产物 | 内容 |
| --- | --- |
| `raw.jsonl` | 不可变 sample：输入/运行身份、implementation_source_sha256、benchmark_binary_sha256、implementation_digest、workers、计数/摘要、elapsed_ns、RSS 范围/诊断量和 Go alloc_bytes |
| `environment.json` | 机器/runtime/oracle、实现身份、计时/RSS 范围、日期、过滤条件、单元及精确 fixture metadata |
| `summary.json` | 通过门禁的实现身份、逐单元聚合与确定性证据视图 |
| `summary.md` | 实现来源、单 worker/扩展性/属性视图，以及完整、限定范围的矩阵 |

实现身份记录可用时的 `git_commit`、`git_dirty`、`source_sha256`、
`source_file_count` 和 `sha256-path-mode-content-v1`。源文件指纹对排序后的
非忽略仓库文件，以相对路径、文件类型/可执行位、字节数和内容 hash 编码。
已跟踪的删除、符号链接目标同样影响指纹。缓存、benchmark 产物、被测程序以及
显式指定的输出/语料目录均被排除。工作树有改动或仓库尚无提交时，指纹仍有
明确含义；不可用的 Git metadata 可以为空。

程序来源记录精确的已执行绝对路径、字节数、SHA-256、来源及本地构建命令。
`--go-binary` 记录 `supplied-binary`，没有构建命令；工作树指纹是上下文，
不声称该程序来自此源码。raw 使用 `implementation_digest` 绑定完整身份，
汇总验证并展示这些绑定。若记录的程序仍存在，汇总还验证其字节数/hash；
归档证据可以在不保留程序的情况下重新汇总。

launcher 拒绝非空输出目录，并以独占创建方式写入 raw/environment。汇总保留
这些文件，重新验证当前清单、选中条件、已保存输入、sample 顺序和全部签名。
每个 summary 文件先 flush、fsync，再原子替换；两个文件不是同一个事务。
原始 benchmark 输出、下载 PDF 和生成缓存均不提交。

每种实现、每个单元报告耗时中位数、P25/P75、生命周期峰值 RSS 中位数，
pages/s = pages × 1e9 / median elapsed_ns。分位数在排序 sample 的 (n-1)*p
位置线性插值。Speedup = Playa 耗时中位数 / Go 耗时中位数，仅在 effective
workers 相同时计算。五次重复提供小样本分布，不是置信区间。不能把 preset
某行推广到未测 PDF、任务或机器；smoke 仅重复一次，不能支持吞吐主张。

顶层汇总公开 `summary_status`、`qualified_cell_count`、
`blocked_cell_count` 和 `correctness_mismatch_cell_count`。每个单元记录
`correctness_status`、双方 `task_signatures`、`mismatch_fields` 和明确的
`speedup_blocked_reasons`；跨实现不一致时，仅匹配才有意义的 `counts` 与
`sha256` 置空。

JSON `views` 生成 `single_worker_tasks`、`scaling`、`by_document_type`、
`by_language` 和 `by_page_tier`，Markdown 同步生成对应表格。任务比较选取
requested_workers=1；扩展性按页面任务/输入列出 workers 1/2/4/8，标注未测
设置，并将每种实现与其自己的单 worker 耗时中位数比较。没有单 worker 基线
时扩展比值不可用；有效拓扑仍在完整表中展示。

属性聚合不混合不同任务或 requested worker 数。在同一组内，对每份输入的
耗时中位数和页数求和；吞吐是总页数除以中位数之和。仅当每个成员的有效拓扑
均匹配时，才报告 Playa/Go 中位数之和的比值。这是描述性聚合，不是单独测量
的组合任务，也不是各单元 speedup 的平均值。成员输入、被阻断的单元数及
正确性范围均明确列出。多语言文档完整计入每个语言组，因此语言组互有重叠。
视图不生成跨库内存比值或普遍性能主张，并保留逐单元的计数/摘要限定。
