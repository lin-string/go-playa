# Benchmark 结果 — 2026-09-29

[English](benchmark-results-2026-09-29.md) | 简体中文

本页结果遵循 [benchmark 协议](benchmark.zh-CN.md)，采用**部分汇总、失败关闭**：
reference 共 77 个单元、770 个新进程样本，其中 73 个
单元具备跨库倍率资格；scaling 共 89 个单元、890 个样本，其中 81 个具备资格。
下表列出所有被阻断单元；任务正确性签名不一致时不计算倍率。

## 范围与环境

- 日期为 2026-09-29。Reference run ID 为
  `20260929T030834Z-2ec420a0`；scaling run ID 为
  `20260929T034536Z-586dc351`。
- Apple M2 Pro、arm64、10 个逻辑 CPU、16 GiB RAM、macOS 26.0.1；
  Go 1.25.6、Python 3.10.14，以及由
  [`compat/upstream.toml`](../compat/upstream.toml)固定契约解析出的 Playa 1.1.0。
- 这是有其他应用活动的**交互式桌面**运行，并非实验室隔离机器。结果是项目的
  当日证据和优化参考，不能泛化为与硬件无关的普遍结论。
- 两个 preset 都为每个实现执行五次、每个样本使用新进程，并采用 AB/BA 顺序和
  open-through-close 计时。Reference 请求 workers 1 和 4；scaling 使用
  workers 1/2/4/8。
- 被执行源码 SHA-256：
  `fbe3e2a25c667ec6f88a01f0d7c6f1b7505298e13863513ea4fc76d1235139e2`；
  benchmark 二进制 SHA-256：
  `59756d147ced354d2e9aa6adf852e7260533c347a03faa9aa79ef7db091a397a`。
  checkout 为 dirty 且 unborn，因此不声称对应某个 commit。
- 文本/字形投影几何使用 0.01 PDF 用户单位网格和显式中点规则规范化；它吸收跨
  运行时浮点噪声，同时仍把文本、字形顺序、字符串、计数和几何纳入摘要。图像
  摘要使用协议定义的 decoded-stream framing。

## 核心结果

在四个完全通过门禁的全语料、单 worker 任务聚合中，go-playa 相对 Playa 的吞吐
倍率为 **3.36× 到 5.56×**。另外两个聚合各包含一个稳定的跨实现摘要差异，因此
明确阻断。

| 任务 | 文档数 | Go 中位数之和（秒） | Playa 中位数之和（秒） | 倍率 | 阻断单元 |
| --- | ---: | ---: | ---: | ---: | ---: |
| `open-pages` | 9 | 0.256 | 1.357 | 5.31× | 0 |
| `objects` | 9 | 0.500 | 2.780 | 5.56× | 0 |
| `text-glyphs` | 6 | 8.047 | 27.345 | 3.40× | 0 |
| `layout-items` | 6 | 13.624 | 45.842 | 3.36× | 0 |
| `image-digests` | 7 | 4.028 | 28.397 | 阻断 | 1 |
| `text-glyph-jsonl` | 6 | 23.970 | 141.022 | 阻断 | 1 |

这些数值是逐文档中位数的描述性求和，不是单独计时的组合 workload。高 worker
下的简单任务主要反映 Playa 进程池启动和 IPC 成本，不能把其中很大的倍率泛化到
CPU 密集型提取。

## 正确性阻断

差异在每个实现内部的所有重复和 worker 数之间保持稳定，因此作为真实差异报告，
而不是当作噪声丢弃。

- `openintro-statistics/text-glyph-jsonl`：双方均得到 465 页、53,935 个文本对象、
  905,086 个字形，但 go-playa 的规范摘要为
  `7c198a043bbf14d76cbf2d9f7789e9036dd610e1ddd3e951c788418306811302`，
  Playa 为
  `1b8854a69312c79c29fd8f501adff033fbb2f1b3cadf8af28a80f9b7e730cf81`。
  检查发现 go-playa 将一批数学/列表符号映射为 `●`，固定 Playa oracle 输出空串。
- `riscv-unprivileged/image-digests`：双方均得到 696 页和 13 个图像，但 decoded
  stream 摘要分别为
  `be9a8a3bddb3a8ce1800cc426e0c86262deef59b0d3acb0ea26b09bb54957463`
  与 `886a0aed9953f700320a6e9c944d839d8e2b6c4e37b8be6e7e4bb97f3850326a`。

这些文档/任务组合的全部跨库倍率和同实现 scaling 倍率均被阻断；计时只保留为
诊断数据。

## Reference 矩阵

每格为 `Go 中位数 ms / Playa 中位数 ms / Playa÷Go`。`阻断` 表示任务正确性
门禁未通过，不发布倍率。`objects` 是顺序任务，因此没有 workers=4 单元。

| 文档 | 页数 | 类型 | 任务 | w1：Go/Playa ms/倍率 | w4：Go/Playa ms/倍率 |
| --- | ---: | --- | --- | ---: | ---: |
| `gnu-emacs` | 804 | born-digital-text | `layout-items` | 3489.26 / 9071.20 / 2.60× | 1297.93 / 3164.84 / 2.44× |
| `gnu-emacs` | 804 | born-digital-text | `objects` | 121.32 / 728.14 / 6.00× | — |
| `gnu-emacs` | 804 | born-digital-text | `open-pages` | 70.21 / 359.26 / 5.12× | 57.51 / 906.91 / 15.77× |
| `gnu-emacs` | 804 | born-digital-text | `text-glyph-jsonl` | 6444.14 / 37863.99 / 5.88× | 1771.71 / 10678.80 / 6.03× |
| `gnu-emacs` | 804 | born-digital-text | `text-glyphs` | 1865.66 / 5283.85 / 2.83× | 764.33 / 2136.84 / 2.80× |
| `gnu-libc-manual` | 1290 | born-digital-text | `layout-items` | 4594.24 / 13034.99 / 2.84× | 1774.64 / 4323.61 / 2.44× |
| `gnu-libc-manual` | 1290 | born-digital-text | `objects` | 175.38 / 1016.89 / 5.80× | — |
| `gnu-libc-manual` | 1290 | born-digital-text | `open-pages` | 94.43 / 523.62 / 5.55× | 80.61 / 1214.64 / 15.07× |
| `gnu-libc-manual` | 1290 | born-digital-text | `text-glyph-jsonl` | 8914.49 / 51894.48 / 5.82× | 2439.77 / 14565.50 / 5.97× |
| `gnu-libc-manual` | 1290 | born-digital-text | `text-glyphs` | 2617.38 / 7337.37 / 2.80× | 1117.77 / 2870.75 / 2.57× |
| `iroha-jiruisho-japanese` | 119 | image-only-scan | `image-digests` | 25.10 / 46.46 / 1.85× | 10.25 / 193.06 / 18.84× |
| `iroha-jiruisho-japanese` | 119 | image-only-scan | `objects` | 7.07 / 35.59 / 5.03× | — |
| `iroha-jiruisho-japanese` | 119 | image-only-scan | `open-pages` | 1.83 / 11.74 / 6.41× | 2.29 / 183.67 / 80.10× |
| `korean-school-reader` | 154 | ocr-scan | `image-digests` | 35.12 / 88.36 / 2.52× | 18.10 / 407.28 / 22.50× |
| `korean-school-reader` | 154 | ocr-scan | `layout-items` | 20.80 / 77.78 / 3.74× | 17.48 / 394.79 / 22.58× |
| `korean-school-reader` | 154 | ocr-scan | `objects` | 13.90 / 62.38 / 4.49× | — |
| `korean-school-reader` | 154 | ocr-scan | `open-pages` | 2.84 / 16.85 / 5.92× | 3.50 / 386.39 / 110.46× |
| `korean-school-reader` | 154 | ocr-scan | `text-glyph-jsonl` | 34.00 / 83.81 / 2.47× | 23.86 / 388.89 / 16.30× |
| `korean-school-reader` | 154 | ocr-scan | `text-glyphs` | 32.76 / 75.69 / 2.31× | 24.44 / 390.95 / 15.99× |
| `muqaddimah-1900` | 596 | image-only-scan | `image-digests` | 55.69 / 153.95 / 2.76× | 36.96 / 486.29 / 13.16× |
| `muqaddimah-1900` | 596 | image-only-scan | `objects` | 31.57 / 139.32 / 4.41× | — |
| `muqaddimah-1900` | 596 | image-only-scan | `open-pages` | 9.94 / 53.76 / 5.41× | 11.05 / 488.75 / 44.24× |
| `murat-coeur-fervent` | 170 | ocr-scan | `image-digests` | 1534.43 / 16563.19 / 10.79× | 457.79 / 4544.81 / 9.93× |
| `murat-coeur-fervent` | 170 | ocr-scan | `layout-items` | 168.67 / 490.52 / 2.91× | 78.56 / 509.26 / 6.48× |
| `murat-coeur-fervent` | 170 | ocr-scan | `objects` | 4.93 / 27.92 / 5.66× | — |
| `murat-coeur-fervent` | 170 | ocr-scan | `open-pages` | 2.12 / 16.00 / 7.56× | 2.60 / 390.89 / 150.34× |
| `murat-coeur-fervent` | 170 | ocr-scan | `text-glyph-jsonl` | 302.21 / 1740.53 / 5.76× | 102.18 / 841.41 / 8.23× |
| `murat-coeur-fervent` | 170 | ocr-scan | `text-glyphs` | 114.01 / 326.65 / 2.87× | 46.12 / 459.37 / 9.96× |
| `openintro-statistics` | 465 | mixed | `image-digests` | 1286.34 / 6966.70 / 5.42× | 453.06 / 2633.22 / 5.81× |
| `openintro-statistics` | 465 | mixed | `layout-items` | 1795.81 / 11982.24 / 6.67× | 598.03 / 4098.97 / 6.85× |
| `openintro-statistics` | 465 | mixed | `objects` | 59.43 / 365.89 / 6.16× | — |
| `openintro-statistics` | 465 | mixed | `open-pages` | 52.82 / 266.67 / 5.05× | 41.76 / 737.76 / 17.67× |
| `openintro-statistics` | 465 | mixed | `text-glyph-jsonl` | 3851.73 / 22124.10 / 阻断 | 1045.08 / 6137.27 / 阻断 |
| `openintro-statistics` | 465 | mixed | `text-glyphs` | 1909.60 / 8255.15 / 4.32× | 617.27 / 2908.61 / 4.71× |
| `riscv-unprivileged` | 696 | born-digital-graphics | `image-digests` | 802.39 / 4191.49 / 阻断 | 204.34 / 1622.15 / 阻断 |
| `riscv-unprivileged` | 696 | born-digital-graphics | `layout-items` | 3554.81 / 11185.23 / 3.15× | 944.49 / 3255.79 / 3.45× |
| `riscv-unprivileged` | 696 | born-digital-graphics | `objects` | 74.84 / 290.61 / 3.88× | — |
| `riscv-unprivileged` | 696 | born-digital-graphics | `open-pages` | 18.64 / 82.91 / 4.45× | 19.51 / 581.51 / 29.81× |
| `riscv-unprivileged` | 696 | born-digital-graphics | `text-glyph-jsonl` | 4423.47 / 27314.72 / 6.17× | 1183.03 / 7201.33 / 6.09× |
| `riscv-unprivileged` | 696 | born-digital-graphics | `text-glyphs` | 1507.10 / 6066.60 / 4.03× | 419.57 / 2126.83 / 5.07× |
| `shuying-siku-quanshu` | 460 | image-only-scan | `image-digests` | 288.47 / 387.15 / 1.34× | 95.47 / 506.13 / 5.30× |
| `shuying-siku-quanshu` | 460 | image-only-scan | `objects` | 11.76 / 112.87 / 9.59× | — |
| `shuying-siku-quanshu` | 460 | image-only-scan | `open-pages` | 2.79 / 26.00 / 9.32× | 3.68 / 426.82 / 115.97× |

## Scaling 矩阵

每个已测单元为 `Go 相对自身 w1 / Playa 相对自身 w1；Playa÷Go`。低于 1× 表示
增加 worker 后该实现反而变慢。`阻断` 会同时隐藏同实现 scaling 倍率和跨库倍率，
因为任务签名不一致。

| 文档 | 任务 | w1 | w2 | w4 | w8 |
| --- | --- | ---: | ---: | ---: | ---: |
| `gnu-emacs` | `layout-items` | 1× / 1×; 2.66× | 1.95× / 1.78×; 2.91× | 2.61× / 2.91×; 2.38× | 2.12× / 3.34×; 1.69× |
| `gnu-emacs` | `open-pages` | 1× / 1×; 5.29× | 1.24× / 0.58×; 11.30× | 1.28× / 0.43×; 15.73× | 1.25× / 0.29×; 22.67× |
| `gnu-emacs` | `text-glyph-jsonl` | 1× / 1×; 5.76× | 1.93× / 1.89×; 5.87× | 3.59× / 3.54×; 5.84× | 4.78× / 4.89×; 5.63× |
| `gnu-emacs` | `text-glyphs` | 1× / 1×; 2.95× | 1.80× / 1.67×; 3.18× | 2.37× / 2.46×; 2.85× | 2.14× / 2.56×; 2.47× |
| `korean-school-reader` | `image-digests` | 1× / 1×; 2.55× | 1.62× / 0.36×; 11.38× | 1.82× / 0.23×; 20.37× | 1.48× / 0.12×; 31.45× |
| `korean-school-reader` | `layout-items` | 1× / 1×; 3.74× | 1.39× / 0.33×; 15.82× | 1.20× / 0.20×; 22.11× | 0.98× / 0.11×; 34.44× |
| `korean-school-reader` | `open-pages` | 1× / 1×; 5.87× | 0.92× / 0.09×; 62.06× | 0.84× / 0.05×; 100.22× | 0.84× / 0.03×; 190.17× |
| `korean-school-reader` | `text-glyph-jsonl` | 1× / 1×; 2.51× | 1.36× / 0.36×; 9.52× | 1.32× / 0.22×; 14.90× | 1.19× / 0.12×; 25.25× |
| `korean-school-reader` | `text-glyphs` | 1× / 1×; 2.32× | 1.34× / 0.33×; 9.60× | 1.27× / 0.20×; 14.85× | 1.15× / 0.11×; 25.50× |
| `muqaddimah-1900` | `image-digests` | 1× / 1×; 2.90× | 1.52× / 0.49×; 8.91× | 1.43× / 0.33×; 12.71× | 1.30× / 0.18×; 20.60× |
| `muqaddimah-1900` | `open-pages` | 1× / 1×; 5.34× | 0.98× / 0.18×; 28.37× | 0.86× / 0.12×; 39.88× | 0.87× / 0.07×; 70.42× |
| `openintro-statistics` | `image-digests` | 1× / 1×; 5.33× | 1.88× / 1.73×; 5.79× | 2.79× / 2.61×; 5.71× | 2.82× / 2.58×; 5.83× |
| `openintro-statistics` | `layout-items` | 1× / 1×; 6.94× | 1.91× / 1.80×; 7.38× | 2.82× / 3.00×; 6.52× | 2.82× / 3.23×; 6.06× |
| `openintro-statistics` | `open-pages` | 1× / 1×; 5.77× | 1.17× / 0.51×; 13.10× | 1.09× / 0.37×; 16.87× | 1.04× / 0.24×; 24.70× |
| `openintro-statistics` | `text-glyph-jsonl` | 阻断 | 阻断 | 阻断 | 阻断 |
| `openintro-statistics` | `text-glyphs` | 1× / 1×; 4.36× | 1.87× / 1.76×; 4.65× | 2.94× / 2.77×; 4.63× | 3.01× / 2.87×; 4.57× |
| `riscv-unprivileged` | `image-digests` | 阻断 | 阻断 | 阻断 | 阻断 |
| `riscv-unprivileged` | `layout-items` | 1× / 1×; 3.07× | 2.02× / 1.85×; 3.36× | 3.54× / 3.25×; 3.34× | 4.52× / 3.95×; 3.51× |
| `riscv-unprivileged` | `open-pages` | 1× / 1×; 4.83× | 0.95× / 0.24×; 19.23× | 0.86× / 0.15×; 27.18× | 0.85× / 0.09×; 45.94× |
| `riscv-unprivileged` | `text-glyph-jsonl` | 1× / 1×; 5.96× | 1.91× / 1.90×; 5.97× | 3.59× / 3.58×; 5.98× | 5.32× / 4.94×; 6.43× |
| `riscv-unprivileged` | `text-glyphs` | 1× / 1×; 4.07× | 1.90× / 1.79×; 4.33× | 3.49× / 2.93×; 4.86× | 4.76× / 3.21×; 6.02× |

## 内存证据

43 个单进程单元具有可直接比较的完整生命周期 peak RSS：go-playa 较低 34 个，
Playa 较低 9 个。例外具有实际意义，例如若干大型原生文本和布局任务中 go-playa
的峰值更高，因此这些结果不支持“普遍更省内存”的主张。

多 worker Playa 运行**禁止跨库内存比值**。此时 Playa 使用多个进程，记录的父进程
峰值与 callback 时 worker 峰值之和只是下界诊断量，并非完整或同时发生的聚合 RSS。
精确范围见 benchmark 协议。

下载的 PDF 继续作为 `.compat-cache/benchmark-corpus` 下被忽略的本地语料。
raw JSONL、生成汇总和 benchmark 二进制则保留在 `.compat-cache/benchmarks`
对应运行目录中；本页是检入的精炼结果记录。
