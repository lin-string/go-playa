# 工程约定

[English](engineering.md) | 简体中文

## 工具链和检查

- 最低语言和标准库基线为 Go 1.25。每个 Playa generator/iterator 都必须保留为
  可独立遍历的 `iter.Seq[T]` 或 `iter.Seq2[T, error]`；惰性是公开语义，不是
  可选优化。
- Makefile 中的 Go 命令默认使用 `GOTOOLCHAIN=local`，防止验证时悄悄下载另一个
  toolchain 到共享 module cache。只有在测试另一套已安装 toolchain 时才显式覆盖。
- 每次提交前运行 `make verify`。它检查格式、module tidiness（不重写模块文件）、
  工程契约、`go vet`、`golangci-lint`、普通测试和 race 测试。
- Go build 和 module cache 是用户全局资源，不归本仓库所有。用
  `make go-cache-status` 查看，用 `make go-cache-clean` 删除 build/test 产物。
  module 清理单独使用 `make go-module-cache-clean`，因为它还会删除其他仓库使用的
  依赖。
- `testdata/files` 保存以普通 Git blob 检入的小型生成 PDF fixture。较大的出版方
  PDF 在 [`compat/public_corpus.json`](../compat/public_corpus.json) 中登记来源、
  许可证、字节数和 SHA-256。显式运行 `make public-fixtures-pull` 才会把它们下载到
  忽略目录 `.compat-cache/public-corpus`；`make public-fixtures-check` 离线校验。
  这些 PDF 不随源码归档或 Go module 分发。检入 fixture 缺失会使测试失败。普通
  package 测试与 `make verify` 不下载大型 PDF，可选公开 fixture 缺失时相关测试明确
  跳过。CI 在 `make verify` 前显式 pull 并 check 公开语料。`make module-archive-check` 保护
  公开源码体积，`make module-source-test` 验证纯公开源码测试路径；二者均包含在
  `make verify` 中。
- 公开 PDF Association fixture 默认由 `make pdfa-fixtures-pull` 从原始仓库直接
  clone 到 `../pdf-association-fixtures`，也可用 `GO_PLAYA_PDFA_FIXTURE_DIR` 指定
  目录。`make pdfa-fixtures-check` 离线校验 origin URL、干净且固定的 HEAD，以及
  每个所选文件的 digest。可选 checkout 缺失时普通 package 测试会提示 pull 并跳过；
  `make pdfa-corpus-test`、`make corpus-test`、`make compat-pdfa` 和 CI 则要求
  语料库存在并严格失败。
- `make verify` 还会运行 `make api-audit`，防止仓库专用兼容 adapter 类型泄漏到
  公开包。
- 发布前或依赖变化后运行 `make vuln-check`。这个单独运行、需要网络的门禁根据
  当前漏洞数据库检查 Go 代码及锁定的 Python 兼容环境。CI 使用
  [`.go-version`](../.go-version) 固定的已更新安全补丁的 Go 工具链。
- 每个 clone 运行一次 `pre-commit install`。唯一 hook 执行 `make verify`，与 CI
  使用同一套不修改源码的检查，不能绕过。格式问题显式使用 `make fmt` 修复，模块
  问题使用 `go mod tidy` 修复；验证不会重写源码。
- `make engineering-check` 扫描非生成的生产 Go 文件，包含尚未加入 Git 的源码。
  它禁止对 `context.Context` 参数、导出 API 的必需函数参数、导出指针 receiver
  方法的 receiver 做 nil 比较，也禁止通过包级空白标识符引用函数/方法隐藏死代码。
  诊断格式为 `path:line:rule`；解析失败同样阻断门禁。生成源码、测试、隐藏目录和
  vendor 依赖不属于该源码策略扫描。`make verify` 包含此门禁，pre-commit 与 CI
  因而都会执行。
  具名参数类型只从能与 API 文件在 Go OS/架构文件名后缀及 build tag 约束下共存的
  声明解析，检查全部源码变体，同时避免把其他平台的回调契约套到指针参数上。
- 修改生成的字体/CMap 数据或准备资源变更 PR 时运行 `make resources-check`。该检查会
  在同步需要网络时访问声明的公开真源仓库，并验证固定的本地输入，在不写入的情况下
  比较检入输入，在临时目录重新生成 Go
  表，并在真源或生成结果漂移时失败。
- 提取、页面模型或公开领域行为发生变化时运行 `make compat`。迭代期间可使用
  `make compat-one`，请求评审前必须包含完整兼容结果。CI 会独立于本地缓存运行
  配置的坐标空间和 corpus。
- 异常输入、恢复或跨领域 acceptance 变更运行 `make corpus-test`；字体和恢复的
  Playa oracle 投影分别运行 `make compat-fonts` 和 `make compat-recovery`。
- 定向 corpus 目标会先运行生成器漂移检查：
  `python3 scripts/generate_security_fixtures.py --check`、
  `python3 scripts/generate_recovery_fixtures.py --check` 和
  `python3 scripts/generate_acceptance_fixtures.py --check`。有意重新生成检入 PDF 时，
  使用对应的不带 `--check` 的生成器。
- 加密 fixture 必须用 `COMPAT_PASSWORD=... make compat-one` 或等价的
  `playa-compat --password` 参数检查。密码会在缓存元数据中哈希，绝不会以明文
  缓存字段写入。

## 对齐维护和 Pull Request

- 每项公开行为变更都必须在 PR 描述中指出对应的 Playa module、class、property、
  method 或 generator；范围变化时更新兼容清单或迁移契约。
- 公开 API 变更需要定向外部 API 测试；当行为可由 oracle 观察时，还需要兼容
  投影字段，或明确记录有意差异。
- generator 形态的行为需要顺序、可重复遍历、提前停止和延迟错误测试。解释器
  状态借出的惰性值在支持保留时必须提供显式 `Finalize`/`Copy` 所有权路径。
- 资源变更必须由真源同步器和生成器产生；生成的 Go 文件是评审产物，不能手工
  编辑。仓库根目录运行 `go generate ./document ./fontdata`；生成器默认输出必须
  指向所属公开 `fontdata` 或 `parser` 包，不能指向无关实现包。PR 中写明真源
  URL/commit 和许可证影响。
- 只有 `make verify`、相关 `make compat` 命令，以及适用时的
  `make resources-check` 均通过，PR 才准备就绪。必须说明未能运行的命令及原因。
- 保持 `go.mod` 依赖精简。PDF 解析和命令行工具优先使用 Go 标准库。

## 公开 API

- 保持仓库根包精简：只放 `playa.go`、根级集成测试和跨领域兼容 glue。新领域代码
  应放入其公开包（如 `content`、`font`、`page`）或 `internal/`。
- 公开包映射 Playa 的领域 module。用户需要内容类型时可直接导入
  `github.com/lin-string/go-playa/content`；根 `playa` 包仍是方便的 `Open` 入口。
- `document` 是文档级解析、惰性缓存、生命周期、页面/对象查找、资源解析和跨领域
  协调的规范所有者。新的无依赖行为属于相应公开领域包；依赖文档的编排保留在
  `document`。
- 无依赖几何值属于公开 `geometry` 包。公开 `document` 引擎和新公开 API 都直接导入
  `geometry`。
- 无依赖 PDF 标量、数组、字典和间接引用值属于公开 `pdftypes/primitives` 包；
  词法解析、对象解析、PDF 文本解码、解析器诊断和流过滤器解码属于公开 `parser`
  包，包括间接过滤器元数据归一化。公开 `document` 引擎依赖这些所属包，并保留过滤后
  `Stream` 访问等文档感知操作。无依赖的打包图像样本展开属于 `imagedata`；无依赖
  的 BDC/DP Properties、marked-content context、marked-content section 和内容
  操作值属于 `contentdata`。字节所有权复制应使用共享的
  `primitives.CloneBytes` helper。`Metadata` 等无依赖文档值属于公开
  `documentdata`。文档解析和缓存生命周期代码留在文档引擎中。
- 无依赖 option 和 discriminator 值属于公开 `cacheconfig`、
  `contentconfig`、`documentconfig`、`parserconfig`、`structureconfig` 和
  `textconfig` 包。对应内部配置路径已删除，不得新增旧名称兼容导出。
- 依赖文档的页面调度选项属于 `document`：封闭的 `PageConcurrencyOption`
  类型和 `WithMaxPageWorkers`/`WithPagesPerWorker` 构造器由根 facade 重导出。
- PDF 对象实现使用导出的 `PDFObject` marker，让领域包可以在不依赖 `document`
  的情况下定义原语。
- 保留 Playa 的领域名词和语义：`Document`、`Page`、内容对象、字体、目的地、
  注释和结构 element。
- 控制流适配到 Go。立即操作返回 `(T, error)`；推进时可能失败的数据源返回
  `iter.Seq2[T, error]`。使用 options 取代 Python 关键字参数。
- `Open` 和 `OpenBytes` 直接接收 `...OpenOption`。选项构造器保持小而可组合；只
  修改部分缓存预算时，以 `DefaultCacheOptions()` 为基线。零预算表示“不保留”，
  不能改变可观察解析结果，包括 `ActionErrorBytes`、`DestinationErrorBytes`、
  `ObjectErrorBytes`、`ObjectStreamErrorBytes` 等有界终止错误缓存，以及字体、
  资源、页面几何、页面框和内容根错误预算；NameTree 和目的地根错误预算同样可配；
  字体轮廓与 Type3 CharProc 缓存也通过同一文档选项配置。所有有界缓存 admission
  检查必须使用防溢出的 `cacheFits` 契约，不能自行用 `limit-current` 减法实现。
- 公开序列保留上游顺序，每次 `range` 都开始全新遍历，不处理消费者未请求的条目，
  并在消费者退出时立即停止。绝不能用 `[]T` 取代上游 generator。
- 没有上游 Playa 对应项的 aggregation 属于 adapter 应用，不能进入公开库包。
  `internal/testcompat` 只能为仓库兼容 oracle 收集，任何包都不能重导出其类型或函数。
- 成功打开的 `Document` 有 `Close() error` 生命周期方法。`Open` 成功后应立即
  `defer doc.Close()`；重复调用 `Close` 必须安全。
- 不能为方便而暴露可变解析器内部状态。高频惰性内容序列可能返回解释器状态的借用
  视图；除非先调用模型的 `Finalize`，否则调用方不能跨迭代保留或修改它们。缓存
  边界 API 仍必须隔离返回值。
- 每个新增公开行为都需要对应兼容投影字段，或在迁移文档中明确说明理由。

## 错误

- 这是可复用库，不是服务；不能导入应用专用业务错误码。
- 为可检测类别定义稳定 sentinel error，并用 `%w` 包装原因。调用方需要位置、对象
  引用或解析操作细节时使用 typed error。
- 不能记录并返回同一个错误。库返回带上下文的错误；命令只打印一个终止错误并返回
  非零状态。
- 不能把解析失败替换为空 slice、零值或静默跳过的 PDF 对象。恢复必须显式且可观察。

## 必需输入与 nil 语义

- `context.Context` 参数是必需输入。调用者传入真实 context；无需取消时传
  `context.Background()`。不得检查 nil、补默认 context，或把 nil 转换为库错误。
- 导出 API 的函数参数是必需输入，除非明确记录其可选 nil 语义。不得对必需回调
  做 nil 前置检查。违反必需输入契约属于程序错误，不是可恢复的 PDF 错误；沿用
  普通解引用或调用语义。
- 导出的指针 receiver 方法要求真实 receiver。不得加入 nil receiver 的成功
  返回、sentinel 或空操作回退，返回的 iterator 闭包内部同样适用。合法零值结构体
  和可选指针字段保留已记录的行为。
- nil 函数式选项仍然是可选输入。页面并发选项忽略 nil，后面的选项优先。私有
  helper 的可选 resolver/filter 参数可以保留明确的 nil 语义。
- checker 精确豁免 `pdftypes.Stream.DecodedBufferWithResolver`、
  `DecodedBufferWithResolverWithError`、`DecodedBufferDigestWithResolver`
  的 `resolve` 参数：nil 表示只使用直接过滤器元数据。
  `fontdata.ParseType1CharStringWithSeac` 的 `resolve` 回调在无法解析 seac 时
  可省略。新增公开可选回调需要记录 nil 行为，并为 checker 增加精确豁免及回归
  覆盖。测试会验证每个豁免当前的文件、API、receiver 和具名函数参数，陈旧项会使
  `make verify` 失败；仅命名为 `resolve` 不会获得豁免。

## 测试和确定性

- 修改公开行为前先写失败行为测试。
- 单元测试覆盖异常语法和边界值。集成测试使用真实 PDF。已声明的外部 fixture
  不可用时可以明确跳过；缺少主仓库检入的 fixture 必须失败。
- 兼容 oracle 是行为对齐的最终依据。对于已声明的 PDF Association 一致性用例，
  其文档预期是正确性依据；超出固定 oracle 的有意修正必须使用精确且会检查过期的
  差异记录。除非上游 API 明确不规定，否则保留对象顺序。
- 序列测试必须覆盖顺序、重复遍历、提前退出和后续数据源条目的延迟错误。PDF
  corpus 必须含后页/后对象损坏的 fixture，使 eager preload 无法误通过。
- 不要新增批量提取 worker adapter。页面级 goroutine 并发由
  `Document.ForEachPageConcurrent` 支持；文档必须说明其所有权、取消、顺序和
  生命周期规则，并证明 `-race` 干净及顺序/并发 benchmark digest 相同。

## 性能验收

- 对有代表性的 CPU 密集型公开接口，将吞吐约为 Playa 的 5 倍且峰值内存更低
  作为方向性优化参考，而不是普遍适用的验收硬门槛或正确性要求。比较时必须保持
  语义、输入一致，使用独立新进程和可比的 worker 拓扑。
- 低于该倍率或内存更高时，应量化并在变更文档中解释。固定启动开销、I/O 或压缩、
  原生库内核、不同并发模型和测量噪声可以是合理原因；Go 自身可避免的复制、分配、
  解析或锁竞争仍是优化目标，不能仅据此归为例外。
- 性能敏感变更必须使用独立新进程、保留一致的结果计数，并覆盖有代表性的大型
  文本、layout、图像和对象 workload。接口支持并发时同时覆盖单 worker 和有界
  并发；交错 warm run，避免文件系统缓存顺序偏向某一实现。
- 改变行为或缓存策略前，先用 CPU/heap profile 解释回归。不能用 eager 收集、
  benchmark 专用 GC 或不完整投影掩盖 retained work。benchmark 回归警告用于触发
  调查，不能成为削弱正确性或兼容性检查的理由。

## 依赖方向

根包与依赖文档的领域 facade（`content`、`font`、`page`、`image`、`outline`、
`structure`）导入 `document`。文档引擎导入 `parser` 以及相应的值、几何和配置包，
不导入这些 facade。无依赖包不能导入 `document` 或其 facade；需要上下文时传递窄
接口或稳定标识。这能避免 Go import cycle，同时保留公开包的直接导入能力。

## 数据和许可证

- 上游派生表、fixture 和生成数据必须在 `NOTICE` 中追溯到上游 commit 和许可证。
- 除非公开 PDF 领域行为或命令行功能需要，否则不要复制周边 Python 打包、
  multiprocessing 或 service 基础设施。

## 并发读取与生命周期

`Open` 成功后，同一 `Document` 上的只读操作可以由多个 goroutine 执行。页面
iterator、内容解释器及其临时输出状态属于页面本地；文档级惰性缓存共享并由内部
同步。借用序列返回的值不能跨迭代保留或修改，除非先调用 `Finalize`、`ValueCopy`
或相应的 `Copy` 方法。

`ForEachPageConcurrent(ctx, callback, options...)` 和
`ForEachPageLayoutConcurrent(ctx, layoutOptions, callback, options...)` 使用
相同的函数式选项与自适应调度器。不传选项时自动选择 worker 数上限，每个潜在
worker 的增长预算为十页，并受 `GOMAXPROCS` 和页数限制。`WithMaxPageWorkers`
配置上限，`WithPagesPerWorker` 配置增长预算。非正数值保留各自默认值。nil
选项会被忽略，后面的选项优先。页面惰性生成，回调完成顺序不固定；context
取消或首个回调错误、被恢复的 panic 会停止剩余工作。调用会等待正在运行的
回调结束后再返回。借用值和布局结果需要跨越回调保留时，先 finalize 或复制。

`Close` 和 `ReleaseTransientCaches` 是互斥的生命周期操作。它们会释放或重置
文档自有状态，不能与文档读取并发。调用任一操作前必须完成所有页面 worker。
