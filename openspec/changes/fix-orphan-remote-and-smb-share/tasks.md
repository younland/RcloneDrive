## 1. 孤儿条目自愈（TDD）

- [x] 1.1 写失败测试（临时配置目录）：元数据含孤儿条目 → `NewRemoteService` 加载后列表无孤儿、remotes.json 已写回、日志记录；配置完整时列表不变
- [x] 1.2 实现：`RemoteService.load()`（或 `NewRemoteService`）加载后按 `config.LoadedData().HasSection` 过滤孤儿并 `save()`
- [x] 1.3 测试全绿

## 2. Remote.RemotePath 字段与校验（TDD）

- [x] 2.1 写失败测试：Create/Update 保存 `remotePath`（规范化 `Trim "/ "`）；非法值（`..`、`:`、空白）被 `validate` 拒绝
- [x] 2.2 `remoteservice.go`：`Remote` 加 `RemotePath` 字段 + `validate` 校验/规范化规则
- [x] 2.3 测试全绿

## 3. 挂载路径拼接

- [x] 3.1 `mountservice.Mount`：拼接 `name:remotePath`（含单测断言拼接结果，挂载执行用既有 fake/跳过真实 IO）
- [x] 3.2 `internal/rclone/mount.go`：挂载日志输出完整远程路径（`back:backup`）
- [x] 3.3 回归：未填路径时挂载目标仍为 `name:`（既有测试不破坏）

## 4. UI：远程路径输入与失败关窗

- [x] 4.1 `driveeditorview.go`：后端无 `root` 字段时显示"远程路径"输入（smb 标签"共享名"），tooltip 按 D4 文案说明行为与容量变化；state/buildRemote/fillFrom 读写 `RemotePath`
- [x] 4.2 `fillFrom` 读取配置失败分支：提示 + 关闭编辑窗回列表
- [x] 4.3 扩展 headless 测试：smb 类型显示共享名输入且回填/保存往返；`go test .` 全绿

## 5. 全量验证与交付

- [x] 5.1 `go build ./...`、`go vet ./...`、`go test ./...`（含 cmount tag 组合）全绿
- [x] 5.2 `openspec validate fix-orphan-remote-and-smb-share --strict` 通过
- [ ] 5.3 中文提交；重新打包（mygo build + makensis），交用户装机验收：SMB 驱动器填共享名重连后容量为真实值；孤儿条目不再弹通知
- [ ] 5.4 验收通过后同步「已沉淀经验」：SMB 根路径无容量语义（About 空 Usage → 1 PiB 占位）与 `name:share` 路径语法
