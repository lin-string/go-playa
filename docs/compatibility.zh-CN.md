# 兼容性 oracle

[English](compatibility.md) | 简体中文

`cmd/playa-compat --compare` 是权威的端到端兼容检查。它用 Go 实现打开每个指定
PDF，从 `.compat-cache` 读取固定 Playa snapshot，再在 Go 中比较投影。snapshot
以一个 header 加每页一条 JSONL record 保存，因此 Python driver 和 Go comparator
都不必在内存中保留完整文档投影。缓存 miss 时会调用
`scripts/compare_playa.py --snapshot-only --snapshot-jsonl`；旧版完整 JSON 缓存会
逐页转换为压缩 JSONL 流，包括大型缓存，且不会把完整 snapshot 载入内存。投影
schema 变化会让旧缓存键失效。

缓存 miss 时，Go driver 为每个比较 section group 启动一个 Playa 进程，把完整
JSONL snapshot 流式写入临时文件。writer 在每页序列化后释放页面本地投影状态，
所以进程可以复用而不必为每个页面批次重新打开 PDF，同时输出内存受当前页 record
限制。这样能保留单一比较流和原始页顺序，且两个进程都不保留完整文档投影。

对于大于 8 MiB 的 PDF，默认 required-section 比较还会拆成独立 section group：
文档/对象元数据、结构、页面/文本提取、字形、布局、marked-content/注释，以及
重型内容/资源。高容量页面投影有意隔离，避免单个 Playa JSONL snapshot 把文本、
字形和布局输出合并成无界临时文件。每组拥有独立 Go `Document` 和 Playa 进程，
命令最后为 PDF 输出一个结果。定向 `--section` 检查和较小 PDF 仍是单 pass。

Playa snapshot writer 遍历页面时也遵循请求的 section 列表：不会触碰未选择的
文本、路径、图像、布局、stream 和内容 iterator。这对定向兼容运行很重要，因为
单个投影不应物化无关页面缓存。

页面结构投影会省略未使用的 ParentTree slot，与 Go 模型的已填充条目序列及 Playa
的 `None` 页面结构 slot 保持一致。真源 package/tag/commit 和坐标空间记录在
[`compat/upstream.toml`](../compat/upstream.toml)；`compat/uv.lock` 使 Python
环境可复现。

## 契约

v1 投影按 section 原地扩展。`compat/manifest.toml` 把每个 section 映射到对应
公开 API，并标为 `required` 或 `pending`。默认命令比较所有 `required` section。
`--section` 只用于诊断选择；只要存在 pending section，`--release` 就会失败。

## 开放许可大型文档语料库

[`compat/public_corpus.json`](../compat/public_corpus.json) 固定三份大型公开 PDF：
RISC-V Unprivileged Architecture、GNU Emacs Manual 和 OpenIntro Statistics。
清单记录来源、许可证、精确字节数和 SHA-256。PDF 二进制文件单独下载到
`.compat-cache/public-corpus`，不随 Go module 分发。出版方 URL 即使更新，摘要
变化也会导致校验失败，必须重新审核内容与许可证后才能更新清单。

```bash
make public-fixtures-pull
make public-fixtures-check
make compat-public
```

`compat-public` 在 `page`、`screen`、`default` 三个坐标空间对完整文档的全部
required section 运行比较。`PUBLIC_CORPUS_MANIFEST` 和 `PUBLIC_FIXTURE_DIR`
可以为公开语料选择其他清单和本地目录；设置 `GO_PLAYA_PUBLIC_FIXTURE_DIR` 可让 Go
package 测试和 Make 目标使用同一个替代目录。每个公开 PDF 路径都作为独立参数传入，
目录含空格也会正确保留。校验与比较不会自动下载缺失文件。未记录的差异仍然是
失败，不会因为 PDF 来自公开语料而被忽略。

`COMPAT_*` 选项继续控制 worker、内存、缓存与计时。运行
`make public-corpus-test` 可执行 helper 的离线回归测试；它也包含在
`make verify` 中。

`make compat-local` 对 `COMPAT_LOCAL_PDFS` 中全部符合兼容条件的检入小型 PDF
运行 release 比较，覆盖所有配置的坐标空间，不需要下载出版方 PDF。
`make compat` 和 `make compat-release` 默认同时包含这组本地语料和公开清单中的
全部 PDF。显式设置 `COMPAT_PDFS` 时只选择指定文件，不会再追加公开语料。
兼容性比较仅作为本地门禁：使用 `compat-local` 覆盖完整检入语料，使用
`compat-public` 或 `compat` 覆盖公开文档。

严格比较仍可能发现有独立证据支持的差异：Go 字体清单包含嵌套资源字典中的字体，
保留实际 clipping state，并采用 Adobe AFM 与 ITC Zapf Dingbats 映射。固定 oracle
的原生 PNG Average predictor 也可能在除法前溢出；分类前应使用独立解码器核对
压缩源。这些类别不会自动豁免，实际失败仍需要源码证据，`compat-public` 对未记录
差异返回失败。

## PDF Association 一致性语料库

`internal/testfixture/pdf_association_fixtures.json` 是公开一致性语料库的严格清单。
它通过完整 commit 固定 PDF Association 原始仓库的 HTTPS 直接 clone，记录每个所选
PDF 的 SHA-256、许可证和证据链接，并且只声明与该文件目标行为有关的兼容 section。
本 module 不复制或分发这些上游 PDF 二进制文件。

固定 commit 中的每个受 Git 跟踪的 PDF 都必须恰好登记一次：要么作为选中的 fixture，
要么作为经过审计的 exclusion。exclusion 同样固定文件摘要和证据链接，并说明该文件
为何没有独立、确定的测试预期，例如它只是说明文档、与另一个已选 fixture 逐字节
相同，或上游明确允许测试结果不确定。`make pdfa-fixtures-check` 会拒绝任何未登记的
PDF、不存在的 manifest 路径或摘要变化。

准备、离线校验、运行行为测试并与固定 oracle 比较：

```bash
make pdfa-fixtures-pull
make pdfa-fixtures-check
make pdfa-corpus-test
make compat-pdfa
```

默认 checkout 位于同级目录 `../pdf-association-fixtures`；可用
`GO_PLAYA_PDFA_FIXTURE_DIR` 或 `PDFA_FIXTURE_DIR` 改变位置。checkout 不存在时，
普通 `go test` 会明确跳过并提示 pull 命令。`make corpus-test`、
`make pdfa-corpus-test` 和 `make compat-pdfa` 是显式本地门禁：它们会校验仓库身份、
干净且固定的 HEAD 以及文件 digest，缺少任何源或 fixture 都会失败。设置
`GO_PLAYA_FETCH_PDFA_FIXTURES=1` 可让单个 PDF Association fixture 测试显式拉取它声明的源。

当前选择的用例覆盖 PDF 2.0 字符串和增量版本、生产器与解析器边界情况、未知过滤器
的降级处理，以及路径图形状态语义，包括 Indexed color 规范化、负 dash phase、
退化与闭合虚线拐角、line cap 与 join 状态、透明度组和 ColorBurn/ColorDodge 混合
模式；还覆盖注释 action 树及 appearance、嵌套字体资源、内容流
语法、严格区分 stream 与 dictionary 对象类型、通过 ToUnicode 与 `ActualText` 提取
Unicode 12.1 文本、Unicode 3.2 密码规范化，以及严格拒绝将 UTF-16LE 视为 PDF
文本字符串编码。用例还包括成对的通过/失败无障碍示例，覆盖
RoleMap、Unicode、结构与标记内容顺序、标题、跨页列表、Artifact 和结构元素
`ActualText`，以及分栏阅读顺序和侧栏在逻辑内容顺序中的位置。

加密的公开 fixture 可以在清单中声明其公开测试密码。兼容环境安装 Playa 的
`crypto` extra，使固定 oracle 能在本地比较中实际处理这些文件。文档明确要求某个
密码失败的 fixture 会单独记录该预期，并且只参加 corpus 行为测试，不进入兼容投影。

PDF Association 的用例说明是正确性依据；固定 Playa release 仍是兼容性依据。
只有 Go 实现失败时直接修复并恢复精确兼容。如果两个实现都违反一致性预期，则把
Go 行为修到文档预期，并在同一清单中记录剩余 Playa 结果。这些记录不是宽泛 xfail：
fixture ID 和 SHA-256、坐标空间、完整 section 列表，以及每一条差异或 Playa/Go
投影终止异常都必须精确匹配。未登记差异会失败；已登记差异消失时也会作为过期记录失败。

普通本地兼容语料通过
[`compat/known_differences.json`](../compat/known_differences.json) 执行同一策略。
每条记录都由 PDF SHA-256、坐标空间和精确 section 分组（以及显式声明的精确替代
分组）限定，并且必须引用 PDF Association 清单或本语料清单的 `upstream_issues` 中
已经确认为 `existing` 或 `submitted` 的 Playa issue。语料 issue 条目记录核查的源码
提交与日期、复现、预期结果和正式 URL，并明确列出其负责的全部差异记录。路径模式只用于按
已记录根因划分差异；真正放行还要求完整有序差异集合的数量和 SHA-256 完全一致。
对象键按规范顺序比较，因此该摘要不受 Go map 随机迭代影响。比较器保留全部差异，
而不是在诊断前缀后停止。值变化、页面差异增删、顺序变化、
无关路径、fixture 变化或上游修复都会令记录失败并要求复查，不会被静默当作 xfail。

指定上游 issue group 或其关联的差异 ID，可生成只供审阅、不会发布的中性 issue
正文；也可查看持久登记状态：

```bash
make pdfa-issue-draft PDFA_DIFFERENCE=unknown-filter-linearization-error
make pdfa-issue-status
```

草稿从 [`compat/upstream.toml`](../compat/upstream.toml) 读取受影响的 Playa package、
release、tag 和源码 commit，引用原始测试文件和预期，且不提本实现。fixture 清单
中的 issue group 会记录关联的全部差异 ID、最后核查的上游 commit 和日期，以及
`candidate`、`existing`、`submitted` 三种状态之一。`existing` 或 `submitted`
记录必须包含 GitHub issue URL，并会阻止草稿命令再次生成该 issue。由同一个上游
缺陷造成的多个差异共享一个 group 和 URL。本地验证会离线检查登记表，但每次实际提交
之前仍须实时搜索上游 open/closed issue。当前
`UnknownFilter-PageContentStream.pdf` 的字节在 stream 字典应为 `>>` 处只有一个
`>`；因此其行为测试要求显式报告解析失败，而不是静默接受为空内容。

当前 required 投影除 catalog 元数据外，还包含规范化的文档 open action，因此
`/OpenAction` 通过公开 `Document.OpenAction` 和 `Action` 访问器比较，而不只作为
原始 catalog 条目比较。

- 文档页数、页码标签、PDF 版本，以及 Playa permission/tag flag
  （`is_tagged`、`is_printable`、`is_modifiable`、`is_extractable`）；
- 文档 `info`、`catalog`、`names` 和 `trailer` 字典，把间接引用保留为稳定对象号
  marker；
- 按 Playa 源顺序排列的文档间接对象，保留重复物理 revision，并把压缩对象流
  member 插在所属 stream 之后。字典值按结构比较；stream byte 按长度和 SHA-256
  比较，同时只保留一个对象。重复 record 按 `(object, generation)` bucket 匹配，
  使 comparator 无需依赖缓存时序即可报告值差异；
- Mapping 风格文档视图（`len`、`get`、`keys`、`values`、`items`），保留 Playa
  的 trailer-size `len`、XRef revision 顺序、增量 revision 的重复对象号，并用
  流式 JSONL record 避免大型对象 map 扩大 metadata header；
- 按顺序的文档源 token，把 boolean 规范化为 Playa 数字 token，并把字符串 byte
  保留为十六进制值；
- 文档源 buffer 长度和 SHA-256 digest，避免 Go 投影再次分配完整 buffer；
- 按 Playa 顺序的 xref revision，包括传统表、xref stream、hybrid revision、
  active entry、压缩对象位置和 trailer；
- 所选页面 index、label、width、height 和 rotation；
- flatten 后的文本对象（`chars`、bbox）；
- tagged 与 untagged 页面默认文本提取结果；
- 直接页面字形序列顺序和字形 geometry/font metadata；
- 递归 Form XObject 中 flatten 后的解释内容对象，包括具体 kind、所属页面 index、
  bbox、CTM、marked-content stack、graphics state、MCID context、解码文本和
  glyph 数；
- 直接解释内容对象，在递归 flatten 前保留 Form XObject node；
- 按源顺序排列的页面内容 stream，以解码长度和 SHA-256 digest 表示；
- 按源顺序排列的页面内容词法 token；
- 按资源名排列的递归 Form XObject，包括资源、transparency group、字体、结构、
  device-space bbox、解码 stream digest、页面归属、CTM、graphics state、外围
  marked-content stack 和嵌套词法 token。Form 资源字典使用完整引用图：根保留引用，
  每个可达间接对象按对象号顺序恰好出现一次，stream 保留字典及解码长度和 SHA-256。
  二进制字符串用十六进制编码，避免重复展开共享资源，同时比较每个可达节点和流；
- 按源顺序排列的页面和 Form XObject 内容对象，投影为词法 operand 加 operator
  keyword；
- 默认 Playa `LAParams` 分析产生的布局 line、textbox、分层 text group 和混合
  `LTPage` child，包括文本、bbox、书写方向、读取 index，以及有序图像/路径/Form
  XObject item；
- glyph 文本、origin、displacement 和 bbox；
- 默认、显式 tagged、显式 untagged 页面文本提取；
- 路径段、painting flag、device-space bbox、页面归属、CTM、graphics state 和
  marked-content context；
- 图像尺寸、bit depth、color space、filter、bbox、页面归属、CTM、graphics
  state 和 marked-content context；
- 页面字体名称、度量、flag、width、书写方向和 bbox；
- 有界 ToUnicode source-code probe，包括 source code 是否有显式映射及其 Unicode
  值；
- 文档级字体 mapping，保留 Playa 后页覆盖冲突的行为；
- 结构 element、role、页面关联、attribute、内容引用和 child；
- 页面级 ParentTree 结构视图，包括 slot 关联 element、内容 kind、attribute 和
  child；
- 按 MCID 分组的 marked-content section 及其聚合文本；
- marked-content point 的 properties、页面归属、CTM、graphics state 和外围
  marked-content stack；
- 页面 annotation，包括所选空间 bbox、页面归属、ParentTree 结构关联、规范化
  修改日期、规范化 action 和递归解析的 annotation property；
- 页面 ParentTree key，保留 key 缺失与显式零的区别；
- 文本、路径、图像、tag 和 Form XObject 的内容对象 ParentTree 关联；
- 水平/垂直 displacement、position 和标准字符 bbox 的有界字体度量 probe；
- 比较 source code、CID 和 Unicode 文本的有界字体解码 probe；
- 命名目的地、大纲 node 和规范化 outline action（包括 URI/file/name/script
  字段、原始字典和 `/Next` 链）；
- 直接 destination-array `/OpenAction` 值，以及十进制、Roman、字母标签之间的
  page-label 规则切换；
- AcroForm `NeedAppearances`、字段值、flag、option、所选 index、widget rect 和
  字段树 child。

字体投影通过公开只读 `Font.IsMultibyte` predicate，把 Playa 的 `multibyte`
兼容字段保持为 oracle 定义的 `false`。不能从 Go 内部 CID 字体 classifier 推导：
Playa 没有把该 classifier 投影到这个字段，CJK 和 vertical-CID fixture 必须保留
相同结果。

运行 `make compat-fonts` 执行定向字体 corpus 检查。它在配置的坐标空间中覆盖
CJK、vertical-CID 和 tagged-text fixture；使用 `COMPAT_WORKERS` 限制并发任务。

运行 `make compat-recovery` 检查两个实现都能打开的恢复 fixture。damaged-text 和
damaged-object-stream fixture 有意测试超出 Playa parser 的 Go-only 恢复，由 Go
恢复测试覆盖，不参与 Playa oracle 比较。

页面级迭代契约与投影 adapter 相互独立。使用 `Page.Texts` 或 `Page.Glyphs` 进行
惰性页面遍历；只有为 oracle 收集完整 snapshot 时才使用 `internal/testcompat`。
Go 方法通过 `iter.Seq2` 产生错误，并在消费者返回 `false` 时停止，与 Playa
generator 行为一致。

当前没有剩余 pending 兼容 section。

`content.text` 投影比较固定 oracle 中有直接对应项的字段，包括 text/glyph code、
CID、字体身份与大小、text/rendering matrix、origin、displacement、bbox、书写方向、
页面归属、graphics state 和 marked-content 关联。graphics state 投影会规范化
Playa 隐式 `Normal` blend mode 和 `Default` black-point compensation 默认值。
Go 特有的 `Invisible` 和 `Unmapped` glyph flag 不进入 Playa 投影，因为固定
oracle 没有对应字段；它们由 Go API 和 glyph-rendering 测试覆盖。

浮点数使用 `1e-6` 绝对容差。字符串、数组长度、对象顺序和其他所有值必须精确
相同。Playa oracle 比较接受 `page`、`screen`、`default` 坐标空间。Go API 还
暴露 `user` 空间，但固定 Playa release 不接受 `space="user"`，因此由 Go 侧
geometry 测试覆盖，而不进入 Python oracle。比较默认使用历史基线 `page`；重复
`--space` 可在一次调用中比较多个受支持空间。

固定 oracle 的分层 textbox heap 使用 `id(obj)` 作为等距 pair 的最终 key。wheel
中的 mypyc 对象地址取决于 allocator 历史，因此单独投影一页与在前面页面后投影
同一页，可能产生不同但 geometry 等价的树。oracle 加载固定 wheel 的 `miner.py`
源码进行布局分析，只把该 allocator key 替换为稳定的每页 textbox/group 创建
ID；Go miner 使用相同 tie order。不 flatten 或省略任何 group node：完整层级、
child、文本、bbox 和 index 仍精确比较。考虑这些稳定 ID 前，会把 heap distance
key 规范化到 comparator 的 `1e-6` geometry 精度。这样能防止小于容差的 Go/
Python 浮点差异选择不同等距树，同时保留每个投影坐标的原始精度。

兼容 oracle 对已经解析的页面 stream 做 tokenization。这样可为 `Contents` 数组
含间接引用的有效页面保留预期 `Page.tokens` 行为；否则固定 oracle 的私有 token
遍历会尝试从未解析 `ObjRef` 读取 `buffer` 并终止。

页面内容遍历遵循 Playa `stream_value` 恢复：`Contents` 数组中的 `null`、scalar
和 dangling reference 条目会被跳过，但不影响后续 stream。对象解析也保留 Playa
PostScript 风格 `{...}` procedure container；最终内容流 EOF 处未终止的 literal
string 或 procedure 只丢弃未完成对象。这些规则一致应用于公开 `Content`、
`Streams`、`Tokens`、`Contents` 路径和兼容投影。

tagged 提取直接拼接每个 marked-content section 内的文本对象，不因基线变化插入
空格。行状态的 MCID 来自紧邻的 marked-content context；公开 TextObject MCID
仍标识最近的有编号 context。marked-content `ActualText` 替换文本时不推进行原点
状态，与固定 oracle 一致。被抑制的内容仍参与行原点状态；既有结构阅读顺序适配重排
section 时，传入状态与随后可见的 section 一起移动。结构元素 `ActualText` 处理
保持不变。

Playa snapshot 默认缓存在 `.compat-cache/`。缓存键包含 PDF 内容 digest、固定
Playa package/version/tag/commit、schema version、所选页面和坐标空间。使用
`--no-cache` 强制重新运行 Playa，或用 `--cache-dir` 选择其他缓存目录。缓存文件
是本地产物，Git 会忽略它们。

每个成功的比较 section group 还会输出一行机器可读的 `TIMING` JSON，记录其 Go
phase。`--timing-mode=auto` 在本地跟踪历史；只要设置了 `CI` 或
`GITHUB_ACTIONS` 就自动切换为仅报告，Make 对应变量为 `COMPAT_TIMING_MODE`。
本地历史保存在 `.compat-cache/timings/`，被 Git 忽略，并按 GOOS/GOARCH、CPU、
逻辑 CPU 数、`GOMAXPROCS`、Go 版本、固定 oracle commit、兼容缓存版本、worker
数和缓存策略隔离。workload 则按 PDF digest、所选页面、坐标空间、section 集合和
密码 digest 分别建基线。原子 lock-directory 会在并发本地进程之间串行化每次
history 事务；锁不会按时间被强行抢占。如果进程持锁时崩溃，之后的运行会告警并
保持 history 不变，直到删除该 `.lock` 目录或本机 timing 目录。只有成功、单
worker、cache hit 且未启用 CPU/heap profile 的运行
可以更新基线。至少有三个历史正常样本后，当前耗时必须同时比中位数慢 35% 以上且
多出 500 ms 以上才会告警。告警不会令兼容测试失败，告警样本不会进入基线，正常
样本最多保留五个。`report` 不读取也不写入历史，`off` 不输出耗时；删除
`.compat-cache/timings/` 只会重置本机性能历史。损坏或来自更新版本的历史会被提示
并原样保留，不影响正确性结果。

缓存格式升级后，当前缓存键无法访问旧版 JSONL entry。使用以下命令评审并只删除
header 可解码且 `cache_version` 低于当前 driver 的 entry：

```bash
make compat-cache-prune
```

该命令保留当前与未来版本、异常文件，以及兼容 driver 仍可能转换的旧 `.json`
缓存。

进行基于 Make 的定向诊断时，`compat` 和 `compat-one` 通过 `COMPAT_SECTIONS`
接受重复 section 名；改变投影语义时设置 `COMPAT_NO_CACHE=1`：

```bash
COMPAT_SECTIONS=content.text COMPAT_NO_CACHE=1 \
  make compat-one COMPAT_PDF=testdata/files/acceptance_cjk_cid.pdf
COMPAT_SECTIONS=annotations make compat \
  COMPAT_PDFS=testdata/files/acceptance_navigation_semantics.pdf
```

仅值投影无法证明惰性。对于每个对应上游 generator 的公开 API，都要添加 Go API
测试，验证源顺序、全新第二次遍历、消费者提前停止和后续条目的延迟错误。保留一份
后页/后对象无效的 PDF fixture：只消费前一条必须成功，继续消费到无效条目必须
产生匹配错误。这些测试与 Python-to-Go 投影比较共同组成兼容验收。

## 运行方式

```bash
make compat
go run ./cmd/playa-compat --compare --section content.text --pdf testdata/files/form_simple.pdf
make compat-release
```

`make compat` 把每个 PDF/坐标空间组合都提交给同一个 `playa-compat` 进程。共享
调度器同时消除 PDF 与坐标空间之间的串行长尾：自动 worker 上限为
`GOMAXPROCS`；独立内存预算的硬上限为检测到的物理内存的 60%，且该上限外始终
至少留出 2 GiB，并按 64 MiB 向下取整。无法检测物理内存时，硬上限回退到
3072 MiB。每次派发任务前，调度器还会重新采样系统可用内存，以及 Go runtime
已经提交且尚未归还的内存；有效容量取硬上限与
`可用内存 + runtime 已提交内存 - 2 GiB` 中的较小值。这样在保守任务估算高于
实际占用时可以安全补入工作，而其他程序增加内存占用时会暂停派发，不侵占系统
保留量。PDF 的文件映射不会被加回，因为操作系统的可用内存估算可能已经把它们
视为可回收页。只要估算值能放进剩余容量，较小任务就可以越过暂时受阻的大任务，避免
CPU 槽位空闲。Go 比较阶段和外部 Python oracle 阶段各自可以并行，但绝不重叠，
以免叠加两类峰值工作集。Make 变量为零表示自动选择（也是默认值）；也可分别
显式设置上限，例如 `COMPAT_WORKERS=6 COMPAT_MEMORY_LIMIT_MIB=6144 make compat`。
显式内存值不会关闭实时系统保留量检查。

并发是第二层优化，不用于掩盖单任务低效。比较器会先保证每个投影有界并消除重复
工作：大型 typed array 直接与逐个借用的 JSON value 对照，不再额外解码一份期望
对象；文档缓存按既有 byte budget 跨页保留共享字体、decoded stream 和已解析资源；
完整 transient state 只在 group 和 phase 边界释放。只有这些单任务成本已经受控后，
调度器才并行派发彼此独立的任务。

启用缓存时，比较器会先展开全部 PDF/空间/section-group key，并通过同一个调度器
并行补齐所有缺失的 Playa snapshot。随后的比较阶段因此只运行 Go，不再因后段偶遇
单个缓存 miss 而在 oracle 与 Go 之间反复串行切换。`--no-cache` 仍保留直接交替路径。
在这个没有活动任务的阶段边界，自动模式会报告重新采样后的有效容量；之后每次
采样已提交内存前会先把 preflight 的死亡堆页归还给操作系统。之后每次派发的实时探针仍
可继续提高或降低该容量。oracle 释放出的内存可以立即转化为比较并行度，同时仍
不会叠加两类工作集。

内存值是保守的调度估算，不是操作系统强制的分配上限。单任务估算若已超过预算，
程序会拒绝执行，而不是越过预算运行；只有确认系统余量后才应显式提高限制。每个
任务仍打开并关闭自己的 `Document`，Playa snapshot 缓存写入使用临时文件加
atomic rename。直接调用 CLI 时可以重复 `--space`；产生的所有任务共享同一个
`--memory-limit-mib` 预算。命令会在比较前打印最终解析出的调度参数。

### 比较器吞吐验证

调度器应使用可复现的公开输入评估。先准备固定的公开语料，预热 oracle 与文件系统
缓存，再在三个坐标空间中用显式和自动 worker 上限比较同一个 section：

```bash
make public-fixtures-pull
make public-fixtures-check
go build -o /tmp/go-playa-compat ./cmd/playa-compat
/usr/bin/time -lp /tmp/go-playa-compat --compare --workers 1 \
  --space page --space screen --space default --section layout \
  --pdf .compat-cache/public-corpus/riscv-unprivileged.pdf
/usr/bin/time -lp /tmp/go-playa-compat --compare --workers 0 \
  --space page --space screen --space default --section layout \
  --pdf .compat-cache/public-corpus/riscv-unprivileged.pdf
```

比较耗时和峰值 RSS 之前，先核对计数与 section 结果。`make compat` 在 `page`、
`screen`、`default` 中检查配置的检入及公开语料。库的 workload benchmark 见
[`benchmark.zh-CN.md`](benchmark.zh-CN.md)。

## 扩展投影

1. 在 `internal/testcompat` 中为投影字段添加失败 Go 测试。
2. 把同一字段加入 `internal/testcompat.Snapshot` 和
   `scripts/compare_playa.py` 的 Playa snapshot 投影。
3. 添加能覆盖它的代表性 PDF，例如用于 AcroForm 的
   `testdata/files/form_simple.pdf`。
4. 合并前运行完整 corpus。

绝不能仅为通过门禁而让脚本忽略 mismatch。有意不支持的功能应记录到
[`migration.zh-CN.md`](migration.zh-CN.md)；在实现前它仍是失败兼容案例。

`cmd/playa-compat` 和 `internal/testcompat` 是仓库开发工具，不属于 go-playa 库
API；它们也不同于 `cmd/playa` 提供的实用终端用户模式。

生成的 `testdata/files/acceptance_navigation_semantics.pdf` fixture 是普通 A4
风格文档，组合了命名目的地、open action、大纲、URI annotation、页码标签和 XMP
metadata。其 document 测试和三空间兼容检查验证这些惰性文档领域模型可以互操作，而
不只是分别通过隔离单元测试。

对于大型文档，`cmd/playa-compat --jsonl` 会写一个文档 metadata header，随后
每行一条页面 record，而不是保留完整 JSON report。与 `--memprofile path` 组合
可在流结束后检查 heap：

```bash
go run ./cmd/playa-compat --pdf .compat-cache/public-corpus/gnu-emacs.pdf \
  --jsonl --memprofile /tmp/playa-jsonl.heap.pprof > /tmp/playa.jsonl
```

fixture `testdata/files/malicious_cmap.pdf` 和 `testdata/files/security_*.pdf`
有意排除在严格 Playa 输出 corpus 之外。它们包含超大或异常 parser 输入，属于
安全回归 fixture：Go 实现会拒绝或限制异常输入，而参考 Playa 版本可能产生不同
的降级投影。它们继续由 Go parser 测试覆盖。
