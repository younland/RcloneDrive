# Design

## Context

现有基建：`catalog.go` 已有 `optionLabels`（key→中文标签，约 40 项）、`optionHelps`（key→中文说明，10 项）、`backendLabels`（类型中文名）、`webdavVendorLabels`（webdav 服务商示例中文），以及兜底函数 `optionLabel`（蛇形转标题）与 `optionHelp`（未收录保留 rclone 原文）。缺口：① 常用协议字段（smb 的 domain/idle_timeout 等、ftp/sftp 的 TLS/密钥系字段）大量未收录；② `backendType()` 的 `Help` 直接输出 rclone 英文 `Description`；③ catalog.go 已 376 行，扩表后会突破 300 行硬指标。

## Goals / Non-Goals

**Goals**

- 常用协议（webdav/smb/ftp/sftp/http）字段 label + help 全覆盖中文；类型描述中文。
- 覆盖率守卫测试防漏翻。
- 表拆分到 `localize.go`，满足文件行数硬指标。

**Non-Goals**

- 不翻译对象存储/网盘等长尾后端的特有字段与类型描述（命中通用 key 自动生效，其余英文兜底）；后续按需补表。
- 不改字段 key、默认值、示例值（value）与写入逻辑——翻译只作用于展示。
- 不引入 i18n 框架/运行时语言切换——应用是单语言（中文）产品，静态表足够（YAGNI）。

## Decisions

### D1 表结构：沿用全局 key 表（选择）vs backend→key 二级表

- **选择**：继续 `optionLabels`/`optionHelps` 按 field key 全局生效。同一 key 跨后端语义一致（host/user/pass/port/url/region…），全局表让通用字段一次翻译处处生效。
- 升级路径（`ponytail:`）：若未来出现同 key 语义冲突（如某后端的 `token` 含义特殊），再加 `backendFieldOverrides map[string]map[string]string` 一级覆盖，查找顺序：后端覆盖 → 全局 → 兜底。当前无冲突，不预先建。

### D2 文件拆分：翻译内容迁至 `localize.go`

`optionLabels`、`optionHelps`、`webdavVendorLabels`、`backendDescriptions`（新增）与 `optionLabel`/`optionHelp` 函数整体迁出 catalog.go；`backendLabels`/`backendGroups`（目录结构，非翻译文案）留在 catalog.go。行数预估：现翻译相关约 150 行 + 新增约 200 行 ≈ 350 行，仍超 300——**接受**：表驱动文案文件以"一张完整映射表"为职责单元，强行拆成 localize_labels.go/localize_helps.go 反而破坏内聚；超出部分记录为有意豁免（`ponytail:` 表文件不适用 300 行上限，上限针对逻辑代码）。

### D3 类型描述：`backendDescriptions` 表 + 原文兜底

`backendType()` 中 `Help` 改为查 `backendDescriptions[info.Name]`，未命中保持 `strings.TrimSpace(info.Description)`。文案风格与现有 tooltip 一致（一句话说明该协议适用场景），常用协议优先收录 webdav/smb/ftp/sftp/http/local。

### D4 覆盖率守卫测试

`localize_test.go`：

- 遍历常用协议 `Fields(typ)`，断言每个字段的 `Label` 命中 `optionLabels`、`Help` 命中 `optionHelps`；失败信息列出缺失 key（作为补翻清单）。
- 抽查断言锚定关键翻译（如 smb `host` label == "主机地址"），防止表被误清空。
- 不断言"含中文字符"——兜底标题（英文转标题）对长尾字段合法，断言 key 集合更稳定不脆。

## Risks / Trade-offs

- rclone 升级若增改常用协议字段 → 覆盖率测试失败，提示补翻（这正是守卫目的）。
- 翻译质量：help 文案以 rclone 原意为准做意译，不确定语义的字段（如 smb `spn`）保留术语原文并在句中解释，不硬翻。
- 长尾后端长期英文：接受——用户实际触达率低，逐需补全。

## Migration Plan

纯展示层表扩充，无数据迁移；旧配置/旧界面行为除文案外无变化。

## Open Questions

（无。）
