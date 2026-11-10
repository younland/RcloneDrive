## Why

两个用户实测问题：① remotes.json 与 rclone.conf 一旦出现不一致（元数据有"测试盘"、配置节没有），每次打开其编辑窗都弹"读取驱动器配置失败"通知，且编辑窗停留在半初始化状态，无自愈机制；② SMB 驱动器挂载的是服务器根（共享列表层），SMB 协议对根没有容量语义，rclone 返回空导致 Windows 显示 1 PB 占位（0.2.5 起为 2 TiB 虚拟值），而 rclone smb 后端没有"共享名"配置项、挂载又写死 `name:` 挂根，用户当前无法通过任何操作挂到单个共享拿到真实容量（实测 `back:backup` 可得 16 TB/10.94 TB，与 Windows 系统映射一致）。

## What Changes

- `RemoteService` 启动加载 remotes.json 时校验每个条目在 rclone.conf 中存在，孤儿条目自动清理并记日志（数据一致性自愈）。
- 驱动器编辑窗回填（`fillFrom`）读取配置失败时，提示后关闭编辑窗回列表，不再停留半初始化状态。
- `Remote` 新增 `RemotePath` 字段（远程子路径，选填；SMB 场景为共享名）：挂载目标由 `name:` 变为 `name:RemotePath`；字段做清理与校验（去首尾 `/`、拒绝 `..` 与 `:`）。
- 驱动器编辑窗基本页新增"远程路径"输入（SMB 类型显示提示"共享名"语义），提示明确：填写后挂载点直接是该共享内容（不再是共享文件夹列表），容量显示为真实值；留空维持挂根行为（容量显示虚拟值）。
- 挂载日志注明实际挂载的远程路径（`name:path`），便于排查。

## Capabilities

### New Capabilities

（无）

### Modified Capabilities

- `remote-management`：新增"启动时配置一致性自愈"要求（孤儿条目自动清理）；`Remote` 数据结构新增远程子路径字段及其校验规则。
- `mount`：新增"挂载远程子路径"要求——挂载目标支持 `name:子路径`，容量探测作用于实际挂载路径。

## Impact

- **代码**：`remoteservice.go`（加载自愈 + `Remote.RemotePath` 校验）；`mountservice.go`（挂载路径拼接）；`internal/rclone/mount.go`（日志含完整远程路径，MountOptions 不变——路径由调用方拼好）；`driveeditorview.go`（"远程路径"输入 + 提示 + 失败关窗）；对应测试。
- **调用方**：`MountManager.Mount` 签名不变（仍收 remote 名字符串），由 `MountService` 决定拼不拼子路径。
- **行为变化**：填写"远程路径"的驱动器，挂载点内容从"共享文件夹列表"变为"该共享/目录的内容"——这是用户明确要修复的目标行为，UI 提示必须说明。
- **兼容性**：`RemotePath` 为 `omitempty`，旧 remotes.json 无需迁移；不填时挂载行为与 0.2.5 完全一致。
- **不受影响**：改名/删除/连接测试/盘符分配逻辑（均按驱动器名定位，与子路径无关）；显示容量手动配置（0.2.5 已有）与其正交，填了真实路径后"自动"即可得真实容量。
