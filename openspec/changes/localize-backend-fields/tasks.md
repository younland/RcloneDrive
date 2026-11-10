## 1. 表拆分迁移（行为不变重构）

- [x] 1.1 新建 `internal/rclone/localize.go`：迁移 `optionLabels`、`optionHelps`、`webdavVendorLabels`、`optionLabel`、`optionHelp`；catalog.go 引用点更新
- [x] 1.2 `go build ./... && go test ./...` 全绿（纯迁移，无行为变化）

## 2. 常用协议字段翻译（按后端逐个补表）

- [x] 2.1 smb：全部可见字段 label + help 中文（host/user/port/pass/domain/idle_timeout/hide_special_share/case_insensitive 等）
- [x] 2.2 webdav：剩余字段（url 已有则补齐 help 缺口、headers、配额与并发类字段）
- [x] 2.3 ftp：TLS/并发/编码类字段
- [x] 2.4 sftp：密钥系、shell/链路类字段
- [x] 2.5 http：基础字段
- [x] 2.6 通用字段查漏：上述后端用到但表中缺失的共享 key 一并补齐

## 3. 类型描述中文

- [x] 3.1 `backendDescriptions` 表：webdav/smb/ftp/sftp/http/local 一句话中文描述
- [x] 3.2 `backendType()` 的 `Help` 接表，未命中保持英文原文

## 4. 覆盖率守卫测试

- [x] 4.1 `localize_test.go`：常用协议逐字段断言 label/help 已收录（失败列缺失清单）+ 抽查锚定断言
- [x] 4.2 `go test ./internal/rclone/ -run TestLocalize` 全绿

## 5. 全量验证与交付

- [x] 5.1 `go build ./...`、`go vet ./...`、`go test ./...` 全绿
- [x] 5.2 `openspec validate localize-backend-fields --strict` 通过
- [ ] 5.3 中文提交；与 fix-orphan-remote-and-smb-share 合并进同版本打包，交用户装机验收：SMB/WebDAV 编辑窗字段标签与 tooltip 全中文、类型树描述中文
