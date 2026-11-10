## Purpose

定义连接类型目录与配置字段的展示语言契约：常用协议与通用字段的文案必须为中文，未收录条目按既定兜底降级且不产生错误文案。

## ADDED Requirements

### Requirement: 常用协议字段文案中文化

常用协议（webdav、smb、ftp、sftp、http）的每个界面可见配置字段，其标签与说明（tooltip）MUST 为中文。示例选项（Examples）的展示文案有中文翻译时 MUST 使用中文，无翻译时保持原值。

#### Scenario: SMB 字段展示

- **WHEN** 用户在驱动器编辑窗选择 SMB 类型，查看任意字段（如 host、domain、idle_timeout）
- **THEN** 该字段的标签与 tooltip 说明均为中文

#### Scenario: 覆盖率守卫

- **WHEN** 常用协议后端新增或修改配置字段而中文表未收录
- **THEN** 覆盖率守卫测试失败，明确列出缺失翻译的字段 key

### Requirement: 连接类型描述中文化

连接类型树中常用协议类型的描述（类型说明）MUST 为中文；未收录的类型保持 rclone 英文原文，MUST NOT 显示空白或占位符。

#### Scenario: 常用协议类型描述

- **WHEN** 用户浏览连接类型树的"常用协议"分组
- **THEN** webdav、smb、ftp、sftp、http 等类型的描述为中文

#### Scenario: 未收录类型降级

- **WHEN** 某长尾后端类型无中文描述
- **THEN** 描述显示 rclone 英文原文，内容完整非空

### Requirement: 未收录字段兜底

中文表未收录的字段，标签 MUST 用字段名转可读标题兜底（蛇形转标题），说明 MUST 保留 rclone 原文；MUST NOT 因翻译缺失导致标签为空、说明为空或界面报错。

#### Scenario: 长尾后端字段兜底

- **WHEN** 用户选择未翻译的后端类型，查看未被中文表收录的字段
- **THEN** 标签为可读标题（如 `hide_special_share` → `Hide Special Share` 样式），说明为 rclone 原文，界面正常渲染
