# Design

## Context

应用为单进程内嵌 rclone（cmount + WinFsp），挂载成功判定是同一进程内 `os.Stat` 盘符（`internal/rclone/mount.go` 的 `waitMountReady`）——提权进程自己能看到它挂的盘符，因此提权场景下判定必然"成功"。生产构建 `-H=windowsgui`（Taskfile.yml）无控制台，`log`/rclone 日志走 stderr 即丢；挂载日志 `logBuffer` 只在成功存入 `m.mounts` 后可经 `Log()` 读取，两条失败路径直接丢弃。安装器为管理员级 NSIS（`build/windows/installer.nsi`），完成页 `MUI_FINISHPAGE_RUN` 直接 `Exec` 应用，继承提权 token。动机与影响面见 proposal.md。

## Goals / Non-Goals

**Goals**

- 安装器完成页启动的应用以非提权运行（根因修复）。
- 提权运行时用户可见警告（防御，覆盖绿色版提权运行等安装器之外的入口）。
- 挂载失败后日志面板可查失败过程。
- 应用 + rclone 日志统一落盘、有轮转，界面可直达。

**Non-Goals**

- 不改挂载就绪判定（stat）本身；不为其增加 readdir 探测/重试。
- 不处理"连接成功后异常中断"的日志保留（已有系统通知兜底）。
- 不做日志查看界面（只提供"打开日志文件夹"），不提供日志级别开关。
- 不改为网络模式挂载（network-mode）来绕过提权可见性问题。

## Decisions

### D1 完成页启动：explorer.exe 中转（选择）vs ShellExecAsUser/UAC 插件 vs 去掉勾选

- **选择**：`MUI_FINISHPAGE_RUN_FUNCTION` 自定义函数内 `Exec 'explorer.exe "$INSTDIR\RcloneDrive.exe"'`。explorer 以桌面会话（非提权）token 启动目标进程后退出。零插件、零新依赖，NSIS 原生指令即可表达。
- 备选否决：`ShellExecAsUser`/`UAC::Exec` 需引入第三方插件（安装器供应链变重）；直接去掉完成页"运行"勾选会改变既有 UX 且用户手动双击 exe 前没有防呆。
- 兼容性注记：explorer 未运行的极简环境（Server Core）拉不起应用——桌面场景可接受。

### D2 提权检测：OpenProcessToken + TokenElevation（选择）vs 其他

- **选择**：`golang.org/x/sys/windows` 的 `OpenProcessToken(GetCurrentProcess())` + `GetTokenInformation(TokenElevation)`，封装为 `isElevated() bool`（windows 实现 + `_other.go` 返回 false）。已在依赖内，约 20 行。
- 警告而非阻止：管理员运行对服务化、脚本化场景仍合法；阻止会破坏这些用户。警告经现有 `notifyUser` 托盘气泡，在 `WhenReady` 托盘创建之后触发一次。

### D3 失败日志保留：`failedLogs map`（选择）vs 挂载返回结构体 vs 全局单槽

- **选择**：`MountManager` 增加 `failedLogs map[string]*logBuffer`，与 `mounts` 对称。两条失败路径写入；`Log(remote)` 先查 `mounts` 再查 `failedLogs`；重新挂载时清除该驱动器旧失败日志。UI 查询路径（`MountService.Status`）零改动。
- 备选否决：把日志塞进 `Mount()` 的 error 返回值——错误信息会变成长文本，托盘通知与 UI 提示都要再截断处理；全局单槽——多驱动器互相覆盖。

### D4 日志落盘：rclone `fs/log.Opt.File` + `InitLogging()`（选择）vs 自写轮转

- **选择**：设置 `fs/log.Opt.File = <ConfigDir>/logs/rclonedrive.log`（设大小上限触发其内置 lumberjack 轮转），随后调用 `InitLogging()`。v1.75.2 已核实（源码级）：`InitLogging` 内 `slog.SetDefault` 会把标准库 `log` 输出也汇入 rclone Handler——应用日志与 rclone 运行期日志一条通道全覆盖，`-H=windowsgui` 的 stderr 黑洞随之解决。
- 实施第一步先对照模块缓存核实 `Opt`/`InitLogging` 的导出形态；若符号不可达，降级路径：`log.SetOutput(自己打开的日志文件)`（覆盖应用日志 + 多数 rclone 内部日志路径），轮转用启动时超阈值截断的朴素策略（`ponytail:` 注释标注上限与升级路径）。
- 隐私边界：默认日志级别（NOTICE）不含凭据；不开启 dump 类调试选项。

### D5 挂载日志面板内容不变

`logBuffer` 仍只记录挂载过程事件（开始/就绪/失败/卸载），不把 rclone 运行期日志混入每驱动器面板（日志无 mount 上下文可分流）；运行期错误由文件日志承载。

## Risks / Trade-offs

- [explorer 中转在无桌面环境失效] → 可接受（该环境本身无 GUI 可用）；任务中在 NSIS 注释标注。
- [D4 依赖 rclone 内部符号，升级 rclone 可能变动] → 使用面收敛到两个符号；任务中先验证再写码，降级路径已定义。
- [失败日志占用内存] → 每驱动器 200 行环形缓冲，与现有一致，忽略。
- [NSIS `Exec` 里 exe 路径含空格的引号处理] → `task package` 编译 + 本机安装实测启动路径。
- [提权警告在通知系统未就绪时调用会丢] → 触发点放在 `WhenReady` 托盘创建之后；无人值守测试跑不了真实通知时在验证报告中声明。

## Migration Plan

全部为新增/局部改动，无数据迁移。回滚 = git revert + 重打包。发布物仍为 `task package` 产出的安装包与便携 exe，文件名与更新器 AssetMatcher 约定保持不变。

## Open Questions

- 更新链路更新完成后重启应用的方式是否同样继承提权？（预期安全：更新器由非提权应用自身拉起；实施时核实一次，若发现提权路径则并入本变更任务清单。）
