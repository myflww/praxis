# 功能对照与验证 / Capability comparison and validation

本页区分已经观察的原工具行为、新实现的验证结果，以及仍未验证的部分。对照是有限的行为合同，不是对第三方所有内部实现的复制声明。

## 研究依据

- 研究目标为一个本机安装的 Grill Me macOS arm64 选择器，SHA-256：`22c92aaaefe8c2aa1f613a255936fb8a258f8fbd7e34448b5a1a4c2ed1f2f117`。
- REA 确认该文件为 arm64 Mach-O，并捕获帮助与 seed / 重复排除的受控进程运行。原始证据按研究用途留存，没有把私有环境路径和进程信息放入公开包。
- 静态引擎未配置，未取得静态反编译结果。本轮研究由运行观察和自建小型 fixture 的黑盒输入输出构成，没有用反编译伪代码生成新实现。
- 黑盒合同包含 24 类行为要求。新 Go 实现者只接收行为规格，未读取原 Skill、上游 Go 实现或原二进制；新版指令和例子另外独立撰写。

## 已对齐的用户能力

| 能力 | 新版本的行为 |
| --- | --- |
| 默认或指定目录 | 省略目录使用当前目录，也可限制到一个子目录。 |
| 本地选题起点 | 成功只输出一条仓库相对路径，不输出正文或候选清单。 |
| 不重复路径 | 调用者通过重复的 `--exclude-path` 记录本会话已选的文件或目录。工具不自行写历史文件。 |
| 可复现选择 | 支持 `--seed INT` 和 `=` 写法；同实现、同候选集合可复现，重复 seed 使用最后一个值。 |
| Git 语义 | 包含 tracked 与未忽略的 untracked，保留已经 tracked 的 ignored 文件，跳过已经不存在的 tracked 文件。 |
| 无 Git 回退 | 在查询不可用时静默遍历目录；此时不保证项目 `.gitignore` 生效。 |
| 文件过滤 | 对照实测的大小、后缀、生成命名、依赖目录与文本探测行为。可用大小为 80–1,000,000 字节。 |
| 文本探测 | 检查最多 16,384 字节前缀，不声称检查全文；严格 UTF-8 窗与控制字节规则有边界测试。 |
| 链接与特殊文件 | 有效且目标仍在范围内的文件链接可选；不选外逃、目录链接、坏链接和 FIFO。根目录本身可为合法目录链接。 |
| 退出状态 | 成功与帮助为 0；坏参数、无候选等失败为 1。帮助和诊断文字是新写的。 |
| 理解问答 | 根据当前实现准备题目，默认提问后等待回答，再给判断、解释与证据；用户可指定语言、题量和批量形式。 |
| 四象限协作 | 日常任务静默复用上下文、查证事实、澄清关键选择，并在授权范围内做最小验证。 |

## 已完成的验证

| 验证 | 结果及范围 |
| --- | --- |
| Go 单元与文件系统测试 | 15 个顶层测试、18 个子测试通过，涵盖参数、Git、排除、文本、链接及只读行为。 |
| 文件筛选对照 | 新 macOS arm64 产物对 560 个黑盒 fixture 场景通过 560 个。多候选时核对合法单路径输出，不要求复刻原 PRNG 的具体序列。 |
| 稳定 CLI 对照 | 33 个可复用参考场景全部通过。五个已发生阶段变化的 Git 参考改由新 fixture 重建生命周期测试。 |
| 六平台构建 | macOS、Linux、Windows 的 amd64 / arm64 全部构建成功，产物摘要见 `bin/checksums.json`。 |
| 重复构建 | 使用同一已验证工具链再次构建，六个文件和整个摘要清单与第一次一致。 |
| 原生运行 | 本机 macOS arm64 新产物已执行并取得 REA 进程证据。其他交叉目标的本机编译不等于实机运行；在线 CI 按实际运行结果另行呈现。 |
| Skill 行为样例 | 独立小型代码样例检查了直接解释、三题批量出题与作答后评分；解释没有被改为测验，题面未附答案，评分识别了取消时机的错误理解。此为有限行为观察，不是跨模型学习效果统计。 |

构建身份为 Go 1.27.3，`CGO_ENABLED=0`、`-trimpath`、无 VCS 注入、无随机 build ID。重建请运行 `python3 tools/build-pickers.py --go /path/to/go`。这只需要开发者具备工具链，使用已编译的 Skill 不需要安装 Python 或 Go。

## 明确差异与未覆盖项

- 新版的指令表达、案例、CLI 文案、源码组织和抽样实现独立设计。随机选择可以使用不同内部算法，因此同一个 seed 不保证与原工具选中相同路径。
- 常见凭据文件名并不一定被候选过滤拒绝，选择器不是秘密扫描器。Skill 在读取前必须判断材料性质、跳过秘密，并按需追加排除。
- 问答质量和遵守指令仍取决于宿主代理、模型、上下文与实际证据。文件筛选对照不能证明任何模型始终公平评分或全面理解代码。
- 本轮没有覆盖全部可能文件名、全部随机分布、大型仓库性能与并发文件替换时的全部竞态。运行捕获不是完整系统调用或网络审计。
- 完整使用边界和来源见 [PROVENANCE](PROVENANCE.md)；此项目不把对照成功表述为第三方背书。

## English summary

The independently written native picker passed **560/560 file-filter cases**, **33/33 stable CLI cases**, and **15 top-level plus 18 subtests** on the measured host. Six OS/architecture artifacts were cross-built and reproduced with identical hashes. The measured new native process also has a REA runtime observation.

This is a finite compatibility contract. It does not claim identical internal code, matching PRNG sequences, static decompilation, universal model behavior, or local execution on every cross-built target. The skill instructions and examples are independently written, with everyday four-quadrant collaboration separated from user-requested understanding checks.
