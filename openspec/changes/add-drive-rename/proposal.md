## Why

驱动器名称是全链路主键（rclone.conf 配置节名、remotes.json 定位键、挂载状态 key、统计分组、Linux/macOS 默认挂载目录），编辑驱动器时名称输入被禁用（旧 Wails 前端沿袭的约束）。名称写错或需要规范的场景（如 "XXM" 改为语义化名称）目前只能删除重建，要重走连接配置，体验差。改名是配置管理的基本能力，且实现风险可控。

## What Changes

- `internal/rclone` 新增 `RenameRemote(old, new string) error`：在 data 层把旧配置节的原始（已混淆）键值复制到新节并删除旧节，不做明文解密/再加密往返。
- `RemoteService.Update` 增加 `oldName` 参数：按 oldName 定位旧条目；名称未变化走现有路径；变化时先 rename rclone 配置节、再更新 remotes.json 条目；新名非法或与其他驱动器重名、驱动器不存在、已连接状态下修改均报错（已连接拒绝沿用现有约束）。
- 编辑驱动器窗口放开名称输入（去掉 `Disabled(s.editing())`），tooltip 改为说明改名行为。
- headless/服务层回归测试覆盖：改名成功（配置节迁移正确、凭据混淆值不变）、新名重名拒绝、旧名不存在拒绝、名称未变兼容、已连接拒绝。

## Capabilities

### New Capabilities

- `remote-management`：驱动器（远程存储）的生命周期管理契约——创建、编辑（含改名）、删除、连接测试中名称的定位与唯一性规则。

### Modified Capabilities

（无——仓库尚无已归档的正式 spec，`remote-management` 为首次建立。）

## Impact

- **代码**：`internal/rclone/config.go`（+`RenameRemote` 及测试）；`remoteservice.go`（`Update` 签名加 `oldName`、改名分支）；`driveeditorview.go`（`nameField` 放开编辑、`Update` 调用点传旧名）；`remoteservice` 相关测试。
- **调用方**：`Update` 签名变化，调用点仅 `driveeditorview.go` 一处（保存按钮）。
- **不受影响**：挂载状态 map、统计分组、托盘菜单——均在断开重连后自然使用新名，OnChange 自动刷新；名称未变的编辑路径行为完全不变。
- **兼容性**：remotes.json/rclone.conf 文件格式不变；旧数据无需迁移。
