<div align="center">
  <img src="resources/appicon.png" width="96" alt="RcloneDrive" />
  <h1>RcloneDrive</h1>
  <p><strong>基于 mygo native 模式的网盘挂载工具：内嵌 rclone 引擎，把远程存储挂载为本地磁盘</strong></p>
  <p>纯 Go 原生界面 · 免前端工具链 · WinFsp 内嵌安装 · 开机自启 · 自动更新</p>
  <p>
    <a href="https://github.com/younland/RcloneDrive/releases"><img src="https://img.shields.io/github/v/release/younland/RcloneDrive?style=for-the-badge&labelColor=1a1b26&color=7aa2f7&logo=github" alt="Release" /></a>
    <a href="LICENSE"><img src="https://img.shields.io/badge/license-MIT-9ece6a?style=for-the-badge&labelColor=1a1b26" alt="License" /></a>
    <a href="go.mod"><img src="https://img.shields.io/badge/Go-1.27-00add8?style=for-the-badge&labelColor=1a1b26&logo=go&logoColor=white" alt="Go" /></a>
    <img src="https://img.shields.io/badge/Windows-0078d7?style=for-the-badge&logo=windows&logoColor=white&labelColor=1a1b26" alt="Windows" />
    <img src="https://img.shields.io/badge/macOS-000000?style=for-the-badge&logo=apple&logoColor=white&labelColor=1a1b26" alt="macOS" />
    <img src="https://img.shields.io/badge/Linux-fcc624?style=for-the-badge&logo=linux&logoColor=black&labelColor=1a1b26" alt="Linux" />
  </p>
  <p align="center">
    <a href="https://awesomemygo.dev/apps/rclonedrive">
      <img src="https://awesomemygo.dev/badge.svg" alt="Listed on Awesome MyGo">
    </a>
  </p>
  <p>
    <a href="#-核心特性">核心特性</a> ·
    <a href="#-安装">安装</a> ·
    <a href="#-快速开始">快速开始</a> ·
    <a href="#-配置说明">配置说明</a> ·
    <a href="#️-构建与开发">构建与开发</a> ·
    <a href="#-winfsp-与挂载">WinFsp 与挂载</a> ·
    <a href="#-开源协议">开源协议</a>
  </p>
</div>

## ✨ 核心特性

### 💽 网盘挂载为本地磁盘

内嵌 rclone v1.75.2 引擎，支持 WebDAV、SFTP、FTP、SMB、S3 及各大网盘等
70+ 存储后端。挂载为盘符（Windows）或目录，带容量用量、实时上下行速率。

### 🖥️ 纯 Go 原生界面

基于 [mygo](https://github.com/egoist/mygo) native UI（非 WebView），单一
二进制、无前端工具链。主窗口 / 设置 / 驱动器编辑三窗口，自绘标题栏，
亮/暗/跟随系统主题，系统托盘动态菜单，关闭即驻留托盘。

### ⚡ WinFsp 内嵌安装

Windows 挂载依赖的 WinFsp 官方 MSI 内嵌在应用里：setup 安装时静默装好
（一次 UAC）；便携版首次挂载时自动提取安装，无需用户单独下载。

### 🔄 GitHub Releases 自动更新

内置更新器检查 GitHub 最新 Release，下载经 SHA-256 二次校验后生效：
NSIS 安装版走安装包静默接管，便携版就地替换二进制并重启。

### 🚀 开机自启与静默托盘

支持开机自动启动（`--tray` 静默进入托盘）、自动连接指定驱动器、
单实例（重复启动聚焦已有窗口）。

## 📦 安装

从 [Releases](https://github.com/younland/RcloneDrive/releases) 下载对应平台的产物：

| 平台 | 产物 | 说明 |
|---|---|---|
| Windows | `RcloneDrive Setup <版本> amd64.exe` | 管理员级安装器，安装时自动装 WinFsp；安装到 `Program Files`，支持自动更新 |
| Windows | `RcloneDrive.exe`（便携版） | 单文件免安装，就地替换方式自动更新 |
| macOS | `RcloneDrive.app` | 需从源码在 macOS 上构建（dmg/签名需要 macOS 环境）；支持挂载（经 macFUSE，首次挂载引导安装） |
| Linux | `rclone-drive_<版本>_amd64.deb` / `.tar.gz` | 支持挂载；deb 经 apt 自动安装 fuse3，tar.gz 需自行安装 fuse3 |

> [!NOTE]
> Windows、Linux、macOS 发布包均启用了挂载（Windows 为 cmount + WinFsp，
> Linux 为 cmount + FUSE3，macOS 为 cmount + macFUSE）。源码自行构建时不带
> `-tags cmount` 则无挂载能力，界面会显示相应提示。

## 🚀 快速开始

前置依赖：

| 依赖 | 版本 | 用途 |
|---|---|---|
| Go | 1.27.x（`.mise.toml` 已钉住，mise 自动切换） | 编译 |
| task | 3.x | 任务入口 |
| gcc（WinLibs） | 任意近期版本 | Windows 挂载的 cgo 编译 |
| NSIS | 3.x | Windows 安装包打包 |

```bash
git clone https://github.com/younland/RcloneDrive.git
cd RcloneDrive
task dev      # 热重载开发（挂载可用）；task run 为单次运行
```

## ⚙️ 配置说明

### 数据目录

用户配置保存在系统配置目录下的 `RcloneDrive/`：

| 文件 | 内容 |
|---|---|
| `rclone.conf` | rclone 远程存储配置（凭据所在，勿泄露） |
| `remotes.json` | 驱动器元数据（挂载点、自动连接、只读等） |
| `settings.json` | 应用设置（主题、开机自启、自动更新） |

### 启动参数

| 参数 | 说明 |
|---|---|
| `--tray` | 静默启动到托盘（开机自启项使用）；重复启动时不打扰用户 |

### 构建变量（Taskfile）

| 变量 | 默认值 | 说明 |
|---|---|---|
| `APP_VERSION` | `0.2.0` | 版本号唯一来源，改版本用 `task sync:version APP_VERSION=x.y.z` |
| `WINFSP_INC` | `third_party/winfsp/inc/fuse` | WinFsp 头文件目录 |
| `CGO_ENABLED` | `1` | 传 `0` 可构建无挂载能力的二进制 |
| `MAKENSIS` | `C:/Program Files (x86)/NSIS/makensis.exe` | NSIS 编译器路径 |

## 🛠️ 构建与开发

### 常用命令

| 命令 | 作用 |
|---|---|
| `task dev` / `task run` | 开发运行（含 cmount 挂载） |
| `task check` | go vet + `mygo vet` + 全部测试 |
| `task build` | 裸 exe → `bin/`（便携版产物） |
| `task package` | Windows 安装包 + 便携 exe → `build/windows-amd64/` |
| `task package:darwin` / `task package:linux` | 交叉打包（无 cgo，不支持挂载） |
| `task package:all` | 全部平台 |
| `task sync:version APP_VERSION=x.y.z` | 改版本号（同步 mygo.json） |
| `task clean` / `task doctor` | 清理产物 / 环境检查 |

### 目录结构

```text
├── main.go / *service.go        # 应用入口与六个后台服务（远程/挂载/系统/设置/更新/窗口）
├── *view.go / uihelpers.go      # mygo ui 原生视图（主窗口/设置/驱动器编辑）与共享基建
├── windows.go / tray.go         # 窗口管理（关闭=隐藏托盘）与系统托盘动态菜单
├── winfsp_windows.go            # WinFsp 检测与内嵌静默安装
├── updateservice.go             # GitHub Releases 更新器（sha256 校验/就地替换/NSIS 接管）
├── internal/rclone/             # 内嵌 rclone 引擎封装（mountlib/config/catalog）
├── build/windows/               # 自定义管理员级 NSIS 安装器（安装时装 WinFsp）
├── third_party/winfsp/          # WinFsp 头文件、官方 MSI（已验签）与 GPLv3 许可
├── resources/                   # 应用图标（appicon/tray，任务栏/托盘/标题栏共用）
├── tools/syncversion/           # 版本号同步小工具
└── .agents/skills/              # 项目技能：commit（提交规范）/ readme（本文档规范）/ mygo-maintenance
```

### 发布流程

CI 流水线（`.github/workflows/release.yml`）在打 tag 时自动构建三平台
产物并创建/更新 GitHub Release（含 `.sha256` 校验和，供更新器校验）：

```bash
task sync:version APP_VERSION=0.2.0   # 1. 定版本（唯一来源是 Taskfile 变量）
git tag v0.2.0 && git push --tags     # 2. 打 tag，CI 自动打包发版
```

手动触发（Actions 页 workflow_dispatch）可不带 tag 构建，产物只上传
workflow artifact 不创建 Release。日常推送/PR 跑 `.github/workflows/check.yml`
（`task check` + Windows cmount 构建验证）。

> [!IMPORTANT]
> 安装包文件名必须保持 `RcloneDrive Setup <版本> amd64.exe` 的命名——
> 更新器靠它区分安装版（走 NSIS 静默接管）与便携版（就地替换）。

## 🧩 挂载的系统依赖

| 平台 | 依赖 | 安装方式 |
|---|---|---|
| Windows | [WinFsp](https://winfsp.dev/) 2.1 | 内嵌在应用里：setup 安装时静默装好（一次 UAC）；便携版首次挂载自动提取安装；卸载应用时保留 |
| Linux | fuse3（fusermount3） | deb 包的 Depends 已声明，apt 随包自动安装；tar.gz 手动 `sudo apt install fuse3`（首次挂载时应用会检测并提示对应发行版的命令，不代装——需要 root） |
| macOS | [macFUSE](https://macfuse.github.io/) 5.x | 首次挂载时应用下载官方 dmg（SHA-256 校验）并经 osascript 请求管理员权限安装；装好后需在 系统设置 → 隐私与安全性 允许系统扩展（苹果安全设计，一次性手动步骤），Apple 芯片可能需先在恢复模式允许降低安全性 |

检测方式：Windows 枚举 `HKLM\SYSTEM\CurrentControlSet\Services` 按 `WinFsp`
前缀匹配（兼容 2.x 的 `WinFsp+时间戳` 服务名）；Linux 检测 `fusermount3`
是否在 PATH；macOS 检查 `/Library/Filesystems/macfuse.fs` 及系统扩展启用状态。

## 📜 开源协议

本项目基于 [MIT License](LICENSE) 开源。

| 组件 | 许可 | 用途 |
|---|---|---|
| [rclone](https://rclone.org/) v1.75.2 | MIT | 云存储引擎 |
| [WinFsp](https://winfsp.dev/) 2.1.25156 | [GPLv3（含链接例外）](third_party/winfsp/winfsp.LICENSE.txt) | Windows 文件系统代理（仅 Windows 产物包含） |
| [mygo](https://github.com/egoist/mygo) v0.5.5 | MIT | 桌面应用框架与原生 UI |
