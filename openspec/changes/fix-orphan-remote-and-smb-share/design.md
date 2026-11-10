# Design

## Context

诊断结论（tools/diag-about 实测）：SMB 根路径 `back:` 的 `About` 返回空 Usage（`backend/smb/smb.go` 对 `share == ""` 直接返回 `&fs.Usage{}`），Total=nil → rclone mountlib 用 1 PiB 占位；`back:backup` 返回真实 16 TB。rclone v1.75.2 smb 后端选项表**没有 root 配置项**，共享只能通过 `fs.NewFs` 的 `name:share` 路径语法传入；而 `MountManager.Mount` 写死 `remote+":"`。孤儿条目问题源于元数据与配置文件分别持久化、无一致性校验；编辑窗 `fillFrom` 读取失败仅弹通知不关窗。

## Goals / Non-Goals

**Goals**

- 启动自愈孤儿条目；编辑窗读取失败即关窗，杜绝重复弹窗困扰。
- 驱动器支持远程子路径（SMB 共享名），使容量探测作用于真实共享。
- UI 提示明确：填写共享名后挂载内容与容量的行为变化。

**Non-Goals**

- 不做"自动猜测共享名"（挂根时取第一个共享查容量属于隐式行为，违反最小惊讶）。
- 不改 rclone smb 后端本身、不升级 rclone。
- 不支持远程路径浏览/补全（纯文本输入，YAGNI）。
- 不处理 crypt/union 等引用型后端对子路径的组合语义（本项目无此场景）。

## Decisions

### D1 孤儿自愈位置：`RemoteService.load()` 启动加载时（选择）vs 每次打开编辑窗时

- **选择**：在启动加载 remotes.json 后一次性校验：对每个条目查 `rclone.Exists(name)`，不存在的收集后从切片移除并 `save()` + 记日志。校验用 `config.LoadedData().HasSection`（已在内存，零 IO 成本）。
- 否决编辑窗时点：只能治标（关窗已单独处理），且弹窗式反馈正是本次要消除的。
- 写回时机：移除动作在启动早期完成，`save()` 失败仅记日志不阻断启动（下次启动重试，`ponytail:` 幂等自愈，无需重试队列）。

### D2 远程子路径存 `Remote.RemotePath`（选择）vs 塞进 Options map

- **选择**：独立字段 `RemotePath string \`json:"remotePath,omitempty"\``。它是挂载行为参数而非 rclone 后端选项——写进 rclone.conf 会被 smb 后端以"未知选项"拒绝（该后端无 root 配置项）。
- 校验与规范化：`strings.Trim(path, "/ ")`；拒绝空段（`//`）、`..`、`:`、空白字符。校验放 `validate()`（与名称规则同处），规范化在保存与挂载拼接前各执行一次（幂等）。

### D3 挂载拼接：`MountManager` 内部组装 `name:path`（选择）vs `MountService` 拼好传入

- **选择**：`MountOptions` 新增 `RemotePath` 字段，`MountManager.Mount` 内部经 `mountFsPath(name, subpath)` 组装实际挂载路径；状态索引（`m.mounts`/`failedLogs`/统计分组）仍按驱动器名 `remote`。首参数同时承担"状态键"与"fs 路径"两个职责，若由调用方拼好传入会把两者耦死（`IsMounted("back")` 将查不到 `back:backup`），故拼接收进执行层、键与路径解耦。
- 容量探测（`diskTotalOverride`）作用于 `fs.NewFs(name:path)` 返回的 Fs，自动命中实际路径，无需额外处理。

### D4 UI 提示明确化与布局一致

- 布局与表单其他行一致（label 在上、控件在下）：水平 label 列 + 固定 90px 宽会在短标签后留大空隙、控件左缘与全宽输入框不对齐（2026-10 验收反馈）。显示容量下拉固定 160px；共享路径输入框 FillWidth。
- 字段标签：SMB 类型显示"共享路径"（支持共享内子目录，如 `backup/照片`），其余后端显示"远程路径"。
- tooltip 给出示例并说明行为差异（SMB）："填写 NAS 上的共享或共享内路径（如 backup 或 backup/照片）后，驱动器直接是该路径的内容，容量显示为真实值；留空则显示全部共享文件夹，容量为虚拟占位值。"
- 显示条件：仅当后端**没有** root 配置项时显示（smb 是当前唯一命中项；有 root 项的后端用既有高级字段即可）。判断方式：`s.findField("root") == nil` 时显示。

### D5 fillFrom 失败关窗

- 复用 `ensureLoaded` 既有模式（`Get` 失败 → 提示 + `allWindows.hide(driveWindowName)`）；`fillFrom` 的 `Options` 失败分支改为提示 + 隐藏窗口 + 立即返回。

## Risks / Trade-offs

- 启动自愈依赖 `HasSection`：若未来引入"元数据存在、配置懒创建"的模式会误删——当前不存在该模式，写注释说明前提。
- `RemotePath` 拼接后大小写：SMB 共享名大小写不敏感但路径串参与 Fs 缓存键，规范化只做首尾清理，不做大小写折叠（与 rclone 行为一致）。
- 用户误把远程路径填成本地盘符（`F:`）→ 被 `:` 校验拒绝，报错信息给出示例。

## Migration Plan

无需迁移：字段 `omitempty`，旧数据视为留空（挂根，行为同 0.2.5）。

## Open Questions

（无——用户已确认方案与提示要求。）
