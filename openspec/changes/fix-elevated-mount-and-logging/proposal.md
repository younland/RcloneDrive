## Why

安装包是管理员级 NSIS 安装器，完成页直接启动应用导致 RcloneDrive 以管理员 token 运行；Windows/WinFsp 下管理员进程挂载的盘符对非提权桌面会话不可见（rclone 官方文档明确记载），于是界面显示"已连接"（挂载进程自己能 stat 到盘符）而用户在资源管理器里看不到——"显示成功，实际失败"。叠加三个日志缺陷（生产构建 `-H=windowsgui` 丢弃 stderr、挂载失败路径的日志随局部变量丢弃、rclone 运行期错误未接入任何缓冲/文件），故障发生时用户完全无日志可查。

## What Changes

- 安装器完成页启动改为经 explorer 降权启动（`Exec 'explorer.exe "$INSTDIR\RcloneDrive.exe"'`），应用不再继承安装器的管理员 token；完成页"运行"勾选体验保持不变。
- 应用启动时检测提权状态（`TokenElevation`），管理员运行时弹托盘气泡警告"挂载的盘符对桌面不可见，请以普通权限重启"；只警告不阻止。
- 挂载失败路径（`point.Mount()` 失败、`waitMountReady` 失败）的过程日志保留到 `MountManager`，UI 日志面板在失败后仍可查看；重新发起挂载或成功后清除。
- 接入 rclone 日志系统（`fs/log.Opt.File` + `InitLogging()`），rclone 运行期错误（WebDAV 认证/证书/超时等）与应用自身 `log.Printf` 统一落盘到 `<ConfigDir>/logs/rclonedrive.log`（rclone 内置轮转）；设置页新增"打开日志文件夹"入口。

## Capabilities

### New Capabilities

- `installer`：Windows 安装器的启动行为——完成页必须以非提权方式启动应用，保证挂载盘符对桌面会话可见。
- `mount`：挂载生命周期——挂载失败后过程日志必须可查；提权运行时必须警告盘符不可见的风险。
- `logging`：诊断日志——应用与内嵌 rclone 的日志必须落盘，且可从界面直达日志目录。

### Modified Capabilities

（无——仓库尚无任何已发布 spec，本次全部为新增。）

## Impact

- **代码**：`build/windows/installer.nsi`（完成页启动函数）；新增 `elevation_windows.go` / `elevation_other.go`；`internal/rclone/mount.go`（失败日志保留）；`internal/rclone/config.go` 或紧邻文件（日志初始化）；`settingsview.go`（日志目录入口）；`main.go`（提权警告调用点）。
- **依赖**：无新增外部依赖（提权检测用已有 `golang.org/x/sys/windows`；日志轮转复用 rclone 内置 lumberjack）。
- **系统/链路**：Windows 打包链路（`task package` 必须重验）；静默安装 `/S` 路径不走完成页，不受影响；更新链路启动应用的方式需在实施时确认是否同样继承提权（更新器由应用自身非提权启动，预期安全）。
- **验证边界**：提权端到端验收需 UAC 交互，无人值守无法完成，须由用户装机验收（方案 tasks 中单列）。
