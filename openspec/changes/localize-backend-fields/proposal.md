## Why

驱动器编辑窗的配置字段说明（tooltip）与连接类型树的类型描述大多仍是 rclone 英文原文：现有 `optionHelps` 中文表仅覆盖 10 个通用字段，`BackendType.Help` 直接使用 rclone 的英文 Description。用户（中文环境）配置 SMB/WebDAV/FTP/SFTP 等服务时，高级字段与类型说明理解成本高。项目已有字段本地化基建（`optionLabels` 约 40 项、`optionHelps`、兜底函数），本变更在其上补全覆盖面，不引入新机制。

## What Changes

- 扩充字段中文标签表 `optionLabels` 与说明表 `optionHelps`：常用协议（webdav、smb、ftp、sftp、http）的全部界面字段 label + help 全覆盖；全局通用字段（host/port/user/pass/url/region 等）继续按 key 全局生效。
- 新增类型描述中文表 `backendDescriptions`（type → 中文描述）：`backendType()` 的 `Help` 优先取中文，未收录保持 rclone 英文原文（不产生空白/乱码）。
- 表与翻译函数从 `catalog.go` 拆分到独立文件 `localize.go`（catalog.go 376 行，扩表后将超文件行数硬指标）。
- 覆盖率守卫测试：断言常用协议每个可见字段的 label/help 均已收录中文（表缺失即测试失败，驱动后续补全），防止新增字段悄悄漏翻。

## Capabilities

### New Capabilities

- `backend-localization`：连接类型目录（类型名、类型描述）与配置字段（标签、说明、示例文案）的展示语言契约——常用协议与通用字段必须中文，未收录条目按既定兜底降级。

### Modified Capabilities

（无——字段展示契约由本变更新建立。）

## Impact

- **代码**：`internal/rclone/catalog.go`（`backendType()` 接类型描述翻译）、新文件 `internal/rclone/localize.go`（迁移并扩充 `optionLabels`/`optionHelps`/`backendDescriptions`/`webdavVendorLabels` 与 `optionLabel`/`optionHelp`）、新增 `localize_test.go`（覆盖率守卫 + 抽查断言）。
- **不受影响**：字段 key、默认值、校验、示例值（value）与配置写入逻辑——翻译只作用于展示文案（Label/Help/类型描述），`Value` 与后端标识符不变。
- **范围界定**：对象存储/网盘等长尾后端不在本次翻译范围（命中通用 key 的自动生效，其余按兜底显示英文）；后续按需往表里补。
- **兼容性**：纯展示层变化，无数据迁移。
