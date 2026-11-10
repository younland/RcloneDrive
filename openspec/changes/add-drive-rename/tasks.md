## 1. rclone 配置节改名（TDD）

- [x] 1.1 写失败测试 `TestRenameRemote`（临时配置目录）：保存含混淆密码的节 A → RenameRemote("A","B") → 断言节 B 存在且各键值（含混淆密码）逐字节一致、节 A 不存在
- [x] 1.2 实现 `internal/rclone.RenameRemote(old, new)`：`config.LoadedData()` 复制原始键值到新节 + `DeleteRemote(old)`；old 不存在或 new 已存在时报错
- [x] 1.3 补测试：old 不存在报错；new 已占用报错；跑 `go test ./internal/...` 全绿

## 2. RemoteService.Update 支持改名（TDD）

- [x] 2.1 写失败测试（临时配置目录 + 临时元数据）：改名成功（rclone 节迁移 + remotes.json 条目名称更新）；新名占用拒绝；oldName 不存在拒绝；名称未变走既有路径；已挂载拒绝
- [x] 2.2 改 `Update(oldName string, remote Remote)`：定位/校验/先 rename 配置节再更新条目；名称未变时不调 RenameRemote
- [x] 2.3 跑 `go test ./internal/...` 与根包服务测试全绿

## 3. UI 放开名称编辑

- [x] 3.1 `driveeditorview.go`：`nameField` 去掉 `Disabled(s.editing())`，tooltip 改为"修改名称将同步更新连接配置与本地记录"；保存调用点改传 `s.editName` 作为 oldName
- [x] 3.2 扩展 `TestDriveEditorView`：编辑态名称输入可交互（非禁用）；`go test .` 全绿

## 4. 全量验证与交付

- [x] 4.1 `go build ./...`、`go vet ./...`、`go test ./...`（含 cmount tag 组合）全绿
- [x] 4.2 `openspec validate add-drive-rename --strict` 通过
- [ ] 4.3 中文提交（`feat(remote): 支持编辑驱动器时修改名称`）
- [ ] 4.4 重新打包（`mygo build` + makensis），交用户装机验收：改名保存后配置列表/托盘显示新名、重连正常、原名配置已不存在
