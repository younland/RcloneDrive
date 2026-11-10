# 全局 Agent 工作流

本文件约束自动化代理在本仓库中的默认工作方式：最短路径、风险分级、验证门禁与可验证交付。
框架知识见 skill `.agents/skills/mygo-maintenance`（mygo 官方维护技能）；项目结构、构建与发布流程见 `README.md`。

## 指令优先级

1. 当前会话中用户的明确要求
2. 仓库规则与本文件的工作流规则
3. 通用习惯

只读/分析类任务不进入实现流程，但结论必须有依据、可追溯。

## 默认原则

- **最短路径**：采用满足质量要求的最轻流程；能直接完成并验证的，不升级为更重流程。
- **自包含**：Debug 先收集证据、定位根因，再修改；Review 独立检查；完成前必须运行与风险相称的验证。
- **流程升级**：影响面超出初始判断、涉及共享逻辑/多窗口/安装与更新链路、需求不清或验证手段不足时，升级流程。
- **流程降级**：改动局部、边界清晰、验证简单时，及时降级，避免形式成本。

## 任务分流

| 类型 | 处理方式 |
|---|---|
| 只读（分析/解释/阅读） | 直接处理，给出可追溯结论 |
| 实现（新功能/修复/重构） | 先澄清目标、边界、验收标准与验证方式；小任务轻量 planning（目标/边界/风险/验证四要素），大任务再升级 |
| Debug | 不猜测式修补：先收集日志/复现/渲染证据，定位根因后再修，修复补贴近故障表面的回归保护 |
| Review | 检查规格符合性、正确性、边界与错误路径、回归风险、可维护性；反馈先验证成立性再处理 |
| 原生界面（ui 视图） | 明确布局对齐/主题（亮暗）/缩放（100%–150%）行为；改后跑 `go test ./...`（含 ui.Tester 视图测试），必要时真实窗口截图核验（见已沉淀经验） |

## 验证与交付门禁（硬门禁）

- **不得虚构**已运行的命令、退出码或验证结果；关键验证无法执行时必须说明原因，并降低完成度表述。
- 本项目标准验证命令：
  - 基础：`go build ./...`、`go vet ./...`、`go test ./...`
  - 完整：`task check`（go vet + `mygo vet` + 测试）
  - 挂载构建：`CGO_ENABLED=1 CPATH=<绝对路径>/third_party/winfsp/inc/fuse go build -tags cmount ./...`（CPATH 必须绝对路径）
  - 交叉编译：`GOOS=linux|darwin go build ./...`
  - 安装器改动：`task package`（或单独 makensis 编译 installer.nsi 验证语法）
- **GUI 验证**：界面变更优先用 `ui.Tester`/`ui.Render` 离屏渲染 + 真实窗口 `PrintWindow` 截图；无人值守跑不了 GUI 时必须声明，不得声称"界面正常"。
- **交付门禁**：声明"完成/已修复/可提交"前必须完成上述相关验证并阅读实际输出。

## 测试策略（TDD 按风险判定，非默认全量）

- **A 直接修改 + 定向验证**：文案/样式微调、显然局部的小修复；不强制新增测试，但须跑相关既有测试。
- **B 修复后补回归测试**：中小 bug、局部行为变化；尽量补贴近问题表面的回归测试。
- **C 必须 TDD**（Red→Green→Refactor→Verify）：新功能、服务/视图契约变更、跨模块共享逻辑、并发/状态机相关改动（更新器状态机、挂载状态）。TDD 要求先观察到测试因预期原因失败。

质量门禁分层：L0 定向验证 < L1 回归测试 < L2 TDD < L3 Code Review < L4 完成前验证（所有声称完成的任务必须过 L4）。

## 工程实践

**快速上手顺序**：`.agents/skills/mygo-maintenance` → 本文件「已沉淀经验」→ README → 任务涉及的模块代码（服务在根目录 `*service.go`，视图在 `*view.go`，窗口管理 `windows.go`）。

**文档与经验沉淀**：开发中反复证明有价值的经验（常见根因、排查顺序、易回归点、验证入口）应及时补充到本文件「已沉淀经验」，模板为：

- **标题**：一句话概括
- **触发信号**：什么现象说明又遇到同类问题
- **根因/约束**：为什么发生
- **正确做法**：以后优先怎么处理
- **验证方式**：如何确认处理正确
- **适用范围**：影响哪些模块/命令

#### 已沉淀经验

- **标题**：`ui.Select` 弹层平铺全部选项且不可滚动，选项多时（盘符 24 个）被窗口底边截断
  - **触发信号**：下拉列表画到窗口外，选不到后面的项，也无法滚动。
  - **根因/约束**：`coreSelect` 的弹层是无 Scroll 的普通 panel；选项一多必然溢出窗口。
  - **正确做法**：用 `ui.SelectBase` 自建弹层，panel 内套 `ui.Scroll().MaxHeight(240)`；样式对齐 mygo `styleButton`/`stylePanel`（`t.Space(1.5)`/`t.Space(3.5)`/`Radius+2`/Shadow）。本仓库已封装在 `uihelpers.go selectLabeled`，新下拉一律走它。**绑定真实值指针**（`SelectBase(c, value)`），label 只作展示文本——绑定"每帧重算的局部变量"会与两帧写回机制冲突导致选择丢失。
  - **验证方式**：`ui.Tester` 展开弹层断言首尾选项存在并点选写回（见 `TestDriveEditorView`）；视觉滚动效果需真实窗口核验。
  - **适用范围**：所有选项可能较多的下拉（盘符、枚举示例、端口等）。
- **标题**：headless 测试点击 `SelectBase` 的 item 选不中：三条铁律
  - **触发信号**：`tt.Click(选项)` 无报错但绑定值未变（真实鼠标点击却正常）。
  - **根因/约束**：① item 首次捕获指针会标记输入 consumed 并重建一帧，跳过 Move 的直接点击 Down/Up 分属两个构建 pass，`Clicked()` 判定失效——点击前先 `tt.Move` 到目标（官方 base_test 同款顺序）；② item 必须有 `.Label(...)`，仅 Children 文本时点击传导不可靠；③ `Find/HasText` 基于 labels 表，包含被 Scroll 裁剪区外的元素——点击视口外的项会穿透（真实滚动行为），测试只能点视口内可见项。
  - **正确做法**：Move → Click → Frame → 断言绑定值；点选目标选弹层视口内靠前的项。
  - **验证方式**：`TestDriveEditorView` 的盘符弹层用例。
  - **适用范围**：所有 `SelectBase`/`ComboboxBase` 自建弹层的 headless 测试。

- **标题**：`ui.Row` 默认把子元素垂直居中，内容不足满高时会被"居中"到窗口中部
  - **触发信号**：窗口上部大片空白、内容出现在视觉中心（设置窗口内容少时最明显）。
  - **根因/约束**：`ui.Row` 的跨轴对齐默认 Center（"lays its children out from left to right, centered vertically"），侧栏 FillHeight 不受影响，右侧内容区（Scroll/Column）只等于内容高度。
  - **正确做法**：主布局 Row 显式 `.AlignItems(ui.Start)`，且**内容区再显式 `.FillHeight()`**——只靠 AlignItems(Start) 时 Scroll 拿不到有界高度，内容超长会溢出窗口而不是滚动。
  - **验证方式**：内容少的窗口靠顶、内容多的窗口出现滚动条且底部按钮不被覆盖。
  - **适用范围**：所有「侧栏 + 内容区」的窗口布局（settingsview/driveeditorview）。
- **标题**：`ui.Select` 触发器有 140px 主题最小宽度（t.Space(35)），在固定宽列内会溢出并与相邻控件重叠
  - **触发信号**：两个下拉框画成一个长框、边框/箭头叠在一起。
  - **根因/约束**：Select 的 MinWidth 在 clampW 里最后生效，会顶掉显式 Width；子元素溢出固定宽父列后，后续兄弟仍按列宽排布 → 重叠。
  - **正确做法**：列内 Select 一律 `.MinWidth(0)`（`selectLabeled` 已内置），再 FillWidth 或显式 Width。
  - **验证方式**：真实窗口截图核验（离屏渲染可能看不出，见下条）。
  - **适用范围**：所有窄列内的下拉（盘符、协议、端口等）。
- **标题**：`ButtonBase` 默认 `.Center()` 水平居中内容，列表项要左对齐需显式改
  - **触发信号**：列表里短标签项的图标比长标签项更靠右，逐项错位。
  - **根因/约束**：`coreButtonBase = coreRow(c).Center()`。
  - **正确做法**：`.Justify(ui.Start)`（垂直居中保留）。
  - **验证方式**：树/导航项图标左缘对齐。
  - **适用范围**：类型树项、设置导航项等 FillWidth 的 ButtonBase。
- **标题**：离屏渲染（ui.Render/Tester）与真实窗口可能有差异，关键界面要真实窗口截图核验
  - **触发信号**：Tester 渲染正常，用户反馈真实窗口布局错乱。
  - **根因/约束**：真实窗口的字体回退、DPI、动画帧状态与 Tester 不完全一致。
  - **正确做法**：用 `PrintWindow`（flags=2）对运行中的真实窗口截图检查；临时调试可用环境变量钩子直接弹出目标窗口。
  - **验证方式**：截图中布局/控件与预期一致。
  - **适用范围**：所有涉及具体布局坐标、控件重叠的界面改动。
- **标题**：WinFsp 2.x 驱动服务名带构建时间戳后缀（WinFsp+2026...），查固定名 `WinFsp` 永远判为未安装
  - **触发信号**：已安装 WinFsp 仍弹"需要安装"；安装器每次运行都重复装。
  - **根因/约束**：WinFsp 2.x 服务名为 `WinFsp+YYYYmmddTHHMMSSZ`（另有 WinFsp.Launcher / WinFsp.Np）。
  - **正确做法**：枚举 `HKLM\SYSTEM\CurrentControlSet\Services` 按 `WinFsp` 前缀匹配；NSIS 用 EnumRegKey 循环 + `StrCpy $3 $1 6` 前缀比较；再以 `winfsp-x64.dll` 默认路径兜底。
  - **验证方式**：`go test -run TestWinfsp` 中 `winfspInstalled()` 在已装机返回 true。
  - **适用范围**：winfsp_windows.go、build/windows/installer.nsi。
- **标题**：mygo 自动生成的安装器是按用户级安装（%LOCALAPPDATA%、不提权）且无定制注入点
  - **触发信号**：想在 setup 阶段装系统组件/驱动（如 WinFsp）或加安装步骤。
  - **根因/约束**：mygo 的 NSIS 脚本是硬编码模板（cmd/mygo/nsis.go），config 无 nsis 钩子；RequestExecutionLevel user。
  - **正确做法**：自建 NSIS 脚本（见 build/windows/installer.nsi，结构照抄 mygo 模板改管理员级），task package 在 mygo build 后用 makensis 产出同名安装包覆盖。
  - **验证方式**：`task package` 全流程通过，安装包文件名保持 `RcloneDrive Setup <版本> amd64.exe`（更新器 AssetMatcher 按此匹配）。
  - **适用范围**：Windows 打包链路。
- **标题**：NSIS 脚本必须是 UTF-8 带 BOM，且 SetRegView 不能出现在区段/函数外
  - **触发信号**：makensis 报 `Bad text encoding`（脚本按系统 ACP/GBK 解析）；或 `SetRegView not valid outside Section or Function`。
  - **根因/约束**：NSIS 按 ACP 读无 BOM 文件；SetRegView 是运行期指令。
  - **正确做法**：脚本保存为 UTF-8 with BOM；`SetRegView 64` 放 `Function .onInit`。注意：编辑器重写文件会丢 BOM，提交前检查。
  - **验证方式**：makensis 编译通过且界面中文无乱码。
  - **适用范围**：build/windows/installer.nsi。
- **标题**：mygo 的 `App.Quit()` 会被窗口 `OnClose` 的 PreventDefault 拦截而永不退出
  - **触发信号**：托盘"退出"点了没反应。
  - **根因/约束**：`prepareQuit` 逐个 `w.close()`，任何 PreventDefault 都会中止退出（关闭=隐藏到托盘的拦截逻辑同样作用于退出路径）。
  - **正确做法**：真正退出前先置 `quitting` 标志（atomic.Bool），各窗口 OnClose 见到标志直接放行。
  - **验证方式**：托盘退出后进程消失；关窗按钮仍只隐藏。
  - **适用范围**：windows.go、main.go、tray.go。
- **标题**：`.gitignore` 排除目录后无法反选其中子目录（"It is not possible to re-include a file if a parent directory is excluded"）
  - **触发信号**：`git add -A` 后 build/windows/ 下的源码文件（installer.nsi 等）始终未入库。
  - **正确做法**：写 `/build/*` + `!/build/windows/`（排除目录内容而非目录本身）。
  - **验证方式**：`git status --short` 能看到新文件。
  - **适用范围**：本仓库任何想纳入版本控制的构建目录子路径。
- **标题**：Taskfile 的 Go 模板字符串里反斜杠是非法转义
  - **触发信号**：`task` 报 `template: :1: invalid syntax`。
  - **正确做法**：vars 里的 Windows 路径一律用正斜杠（`C:/Program Files (x86)/NSIS/makensis.exe`）。
  - **适用范围**：Taskfile.yml 所有模板表达式。
- **标题**：headless 测试（go test）会真弹系统通知，且默认读写真实用户配置
  - **触发信号**：跑 `go test` 时用户机器连弹"读取驱动器配置失败: 测试盘"等系统通知；测试行为随测试机真实配置变化。
  - **根因/约束**：`notifyUserTo` 在无可见窗口时走 `notifyNative`（系统通知）——go test 进程没有窗口，全部通知都落到系统气泡；`testServices`/`NewRemoteService` 默认 metaPath 在真实 `%AppData%/RcloneDrive`，编辑窗回填还会经 `Ensure()` 把 rclone 配置钉到真实 rclone.conf。
  - **正确做法**：主包 `TestMain` 统一隔离——`config.SetConfigPath` 指向临时目录、`remoteService.metaPath` 覆盖到临时目录、`notifyNativeFn` 替换为静默（notificationservice.go 的通知出口必须是可注入的 `notifyNativeFn`，不许直接调 `notifyNative`）；需要完整回填路径的测试用 `rclone.SaveRemote` 在隔离配置里建节。
  - **验证方式**：跑全量 `go test ./...` 期间无系统通知弹出；测试后真实 remotes.json/rclone.conf 无测试条目（grep 测试名 0 命中）。
  - **适用范围**：main 包全部测试、notificationservice.go、任何新增的"用户感知"出口（系统通知/托盘气泡等）。

## 编码与重构规范

- SOLID/DRY/YAGNI；仅在关键或不直观逻辑加简短注释（解释"为什么"，不复述代码）；修改行为时顺手移除死代码；不隐藏失败。
- **硬指标（新代码）**：函数 ≤ 50 行、文件 ≤ 300 行（视图/服务文件超限时按职责拆分）、嵌套 ≤ 3 层、位置参数 ≤ 3、单函数圈复杂度 ≤ 10、禁止魔法数字。
- 断言约定 expected 在前、actual 在后；测试优先覆盖关键路径、边界与错误路径，避免脆弱测试（视图测试用 `ui.Tester`，不依赖真实窗口）。
- 视图代码遵循 ui 包惯用写法：builder 链式调用 + Children 闭包；元素值只在当帧有效，不进应用状态/后台 goroutine。
- 服务（`*service.go`）不依赖 UI：状态变化走 `OnChange` 多播回调 + 视图 `Invalidate` 拉取，不反向持有窗口。
- 重构默认行为不变，在测试保护下分步推进。

## 安全规则

- 不运行破坏性命令（如 `git reset --hard`、强制推送），除非用户明确要求；不用非 Git 工具操作 `.git`。
- 密钥/凭据不硬编码；rclone 远程凭据只存在用户配置目录（rclone.conf），不进仓库。
- WinFsp MSI 只用官方发布资产，入库前必须 Authenticode 验签（NAVIMATICS LLC）；不得替换成来源不明的二进制。
- 提权操作（UAC/runas）必须先给用户明确提示，禁止无提示静默提权。
- 不终止非当前任务启动的进程；不修改旧项目 `E:\github\RcloneDrive2` 的任何文件（只读参考）。

## 沟通与子代理

- 执行类任务先说进度与下一步；分析类任务先给结论与依据。
- 子代理仅在可明确拆分、并行收益真实或需要独立审查时派发；派发时说清目标/范围/输入/预期输出/禁止区域（尤其禁止改动旧项目）；主代理必须复核子代理结论并对最终交付负责。
