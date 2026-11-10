## 1. 挂载失败日志保留（TDD，先红后绿）

- [x] 1.1 写失败测试：构造 `MountManager` 注入 `failedLogs` 后断言 `Log(remote)` 返回内容（当前实现返回空，观察测试因预期原因失败）
- [x] 1.2 实现 `failedLogs map[string]*logBuffer`：两条失败路径（`point.Mount()` 失败、`waitMountReady` 失败）写入；`Log()` 先查 `mounts` 再查 `failedLogs`；重新挂载时清除旧失败日志
- [x] 1.3 补充行为测试：重新挂载清除旧失败日志；成功挂载后 `Log()` 返回成功日志（走 `mounts` 路径）
- [x] 1.4 跑 `go test ./internal/...` 与 `CGO_ENABLED=0 go test ./...` 确认全绿

## 2. 提权检测与警告

- [x] 2.1 新增 `elevation_windows.go`（`isElevated()`：OpenProcessToken + TokenElevation）与 `elevation_other.go`（返回 false），不引入新依赖
- [x] 2.2 `main.go` 在 `WhenReady` 托盘创建后：`isElevated()` 为真时经 `notifyUser` 发一次警告气泡（文案含"以普通权限重新启动"）
- [x] 2.3 写最小断言测试：`isElevated()` 可调用且返回 bool（提权与否取决于运行环境，只验证不崩溃、类型正确）；`go vet ./...` 通过

## 3. 日志落盘与界面入口

- [x] 3.1 对照模块缓存核实 rclone v1.75.2 `fs/log` 的 `Opt` 与 `InitLogging()` 导出形态（design D4 主方案/降级路径以此为分叉点）
- [x] 3.2 实现：启动早期设置 `fs/log.Opt.File = <ConfigDir>/logs/rclonedrive.log`（设轮转上限）并调用 `InitLogging()`；符号不可达时走降级路径（`log.SetOutput` + 启动时超阈值截断，`ponytail:` 注释标注上限与升级路径）
- [x] 3.3 写测试：临时目录下初始化日志后写一条日志，断言文件产生且包含该内容
- [x] 3.4 设置页新增"打开日志文件夹"入口（复用 `openDirectory`；目录不存在时先创建），`ui.Tester` 视图测试覆盖入口渲染与点击回调注册
- [ ] 3.5 用生产构建（`-H=windowsgui`）启动冒烟：确认 `logs/rclonedrive.log` 生成且含应用启动日志（无人值守只能验证文件产生，真实通知/窗口行为交由第 6 组用户验收）

## 4. 安装器完成页降权启动

- [x] 4.1 修改 `build/windows/installer.nsi`：`MUI_FINISHPAGE_RUN` 改为 `MUI_FINISHPAGE_RUN_FUNCTION LaunchUnelevated`，函数内 `Exec 'explorer.exe "$INSTDIR\RcloneDrive.exe"'`（注释标注 explorer 未运行环境的限制）
- [x] 4.2 确认脚本仍为 UTF-8 with BOM（编辑器重写会丢 BOM，NSIS 会报 Bad text encoding）
- [x] 4.3 核实更新链路（updateservice/updatemode）重启应用的方式不继承提权；若发现提权路径，补同类降权处理（对应 design Open Question）

## 5. 全量验证（L4 完成前验证）

- [x] 5.1 `go build ./...`、`go vet ./...`、`go test ./...` 全绿
- [x] 5.2 cmount 构建：`CGO_ENABLED=1 CPATH=<绝对路径>/third_party/winfsp/inc/fuse go build -tags cmount ./...`
- [x] 5.3 交叉编译 `GOOS=linux go build ./...`（确认 `*_other.go` 边界正确）
- [x] 5.4 `task package` 全流程通过，安装包文件名保持 `RcloneDrive Setup <版本> amd64.exe`
- [x] 5.5 `openspec validate --strict` 通过
- [x] 5.6 中文提交（`fix(mount): ...` / `fix(installer): ...` 按范围拆分或合为一提交，视改动内聚度定）

## 6. 用户装机验收（需 UAC 交互，无人值守无法替代）

- [ ] 6.1 管理员运行安装包，完成页勾选运行 → 任务管理器确认进程不带提升标记
- [ ] 6.2 挂载 webdav 驱动器 → 资源管理器可见、可访问盘符
- [ ] 6.3 故意配错一个驱动器挂载 → 日志面板出现失败过程记录
- [ ] 6.4 确认 `%APPDATA%\RcloneDrive\logs\rclonedrive.log` 存在且含 rclone/应用日志；设置页"打开日志文件夹"能打开目录
- [ ] 6.5 验收通过后归档本变更（`openspec archive`），沉淀经验到根 AGENTS.md「已沉淀经验」（提权挂载不可见 + 完成页降权启动）
