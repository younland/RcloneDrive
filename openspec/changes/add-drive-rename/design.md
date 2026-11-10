# Design

## Context

名称贯穿 rclone.conf 配置节名、remotes.json 定位键、挂载状态 key、统计分组与默认挂载目录（动机见 proposal）。现状：`RemoteService.Update(remote)` 按 `remote.Name` 定位旧条目，UI 在编辑态禁用名称输入。rclone v1.75 的 `config.RenameRemote` 是 TUI 交互式函数（内部 `fmt.Printf` + 询问输入），不可嵌入调用；但其逻辑仅为"复制旧节全部键值到新节 + 删旧节 + SaveConfig"，可在本项目 data 层自实现。

## Goals / Non-Goals

**Goals**

- 编辑驱动器时支持改名：配置节迁移 + 元数据更新，凭据混淆值原样迁移。
- 名称未变的编辑路径行为零变化。

**Non-Goals**

- 不支持"已连接时改名并自动断开重连"——沿用现有"已连接拒绝编辑"约束。
- 不更新其他驱动器配置里对本驱动器的文本引用（如 crypt 远程指向 `A:` 之类）——本项目单层远程无此场景（`ponytail:` 若未来引入引用型后端，rename 时需扫描其他节的 remote 类字段）。
- 不改挂载点/盘符与名称的联动逻辑（盘符本就独立于名称）。

## Decisions

### D1 配置节迁移：data 层自实现（选择）vs ReadRemote→SaveRemote→DeleteRemote 往返

- **选择**：照 rclone `copyRemote` 的做法直接操作 `config.LoadedData()`：`GetKeyList(old)` + `GetValue` → `SetValue(new, key, value)`（**混淆值原样**）→ `DeleteRemote(old)`（内部 SaveConfig）。封装为 `internal/rclone.RenameRemote(old, new)`，约 15 行。
- 否决解密往返：ReadRemote 解密密码 → SaveRemote 重新混淆，功能等价但多两次编解码，且 obscure 往返对个别字符集存在理论风险；原样复制零损耗。

### D2 Update 签名：显式 `oldName` 参数（选择）vs Remote 结构携带旧名

- **选择**：`Update(oldName string, remote Remote)`。调用点仅驱动器编辑窗口一处，显式参数让"改的是哪个驱动器"一目了然，避免调用方忘记填旧名字段。
- 校验顺序：oldName 存在 → 新名合法（复用 `validate` 的名称规则与必填项检查）→ 新名未被其他驱动器占用 → 未挂载 → 写配置。

### D3 配置与元数据的更新顺序：先 rename 配置节、后更新元数据

rclone 配置节改名成功后再写 remotes.json。若元数据写入失败（磁盘错误），配置节已在新名——下次编辑会因"旧名不存在"报错，用户删除重建即可恢复，属可接受的一致性窗口；反过来（先改元数据）会留下"元数据指向不存在的配置节"，编辑表单回填直接失败，更糟。

### D4 UI：放开输入 + 保存时校验

`nameField` 去掉 `Disabled(s.editing())`；tooltip 改为"修改名称将同步更新连接配置与本地记录"。实时重名校验不加（YAGNI），保存时统一报错——与现有其他字段的校验路径一致。

## Risks / Trade-offs

- [改名与连接并发：改名的瞬间驱动器刚好被自动连接] → 自动连接发生在应用启动 1 秒后；编辑窗口打开时用户操作与 autoConnect 竞态窗口极小，且 Update 的 IsMounted 检查会拒绝，风险可忽略。
- [Update 签名变化漏改调用方] → 编译期暴露（唯一调用点在 driveeditorview.go）。

## Migration Plan

无数据迁移（文件格式不变）。回滚 = git revert。

## Open Questions

（无）
