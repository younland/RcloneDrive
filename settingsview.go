package main

import (
	"rclonedrive/internal/rclone"

	"github.com/egoist/mygo/ui"
)

// settingsViewState 是设置窗口的视图状态（对应旧前端 SettingsWindow.vue）：
// 左侧分区（外观/启动/更新）+ 右侧内容。数据每帧从服务查询。
type settingsViewState struct {
	section string // appearance / startup / update

	// 开机自启
	autoStartLoaded bool
	autoStart       AutoStartStatus

	// 开关进行中标记（避免重复触发）
	autoStartSaving  bool
	autoUpdateSaving bool

	// 更新
	restartConfirm bool
}

func (s *settingsViewState) view(c *ui.Context) {
	t := applyAppearance(c)
	if s.section == "" {
		s.section = "appearance"
	}
	s.ensureAutoStartLoaded()

	ui.Column(c).Fill().Background(t.Background).Children(func() {
		windowBar(c, t, "设置", "调整界面外观与启动方式", nil, func() {
			if allWindows != nil {
				allWindows.hide(settingsWindowName)
			}
		})

		// 注意 ui.Row 默认垂直居中子元素，这里必须靠顶对齐，
		// 否则内容高度不足时会被居到窗口中部（设置窗口内容少，问题明显）。
		ui.Row(c).FillWidth().Grow(1).AlignItems(ui.Start).Children(func() {
			// 左侧分区导航（对应旧 aside w-35）
			ui.Column(c).Width(120).FillHeight().Gap(2).Padding(10).
				BorderWidth(0, 0, 0, 0).Children(func() {
				s.navItem(c, t, "appearance", "外观", iconPalette)
				s.navItem(c, t, "startup", "启动", iconPower)
				s.navItem(c, t, "update", "更新", iconUpdate)
			})

			// 右侧内容（FillHeight 限定视口高度，内容超长时滚动；
			// 见 driveeditorview.editorPanel 的注释）
			ui.Scroll(c).FillWidth().FillHeight().Grow(1).Children(func() {
				ui.Column(c).FillWidth().Padding(14).Gap(14).Children(func() {
					switch s.section {
					case "startup":
						s.startupSection(c, t)
					case "update":
						s.updateSection(c, t)
					default:
						s.appearanceSection(c, t)
					}
				})
			})
		})
	})

	drainToasts(c, settingsWindowName)
}

func (s *settingsViewState) navItem(c *ui.Context, t *ui.Theme, key, label string, icon *ui.SVG) {
	e := ui.ButtonBase(c).Label(label).Padding(7, 10).Radius(t.Radius).FillWidth().Justify(ui.Start)
	if s.section == key {
		e.Background(t.Surface)
	}
	e.Children(func() {
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Icon(c, icon).Size(14, 14)
			label := ui.Text(c, label)
			if s.section == key {
				label.Bold()
			}
		})
	})
	if e.Clicked() {
		s.section = key
	}
}

// appearanceSection 对应旧「外观」分区：主题模式（旧前端持久化在
// localStorage，这里存 settings.json 的 themeMode）。
// 切换器是三个图标卡片（浅色/深色/跟随系统），选中项用高亮边框标识。
func (s *settingsViewState) appearanceSection(c *ui.Context, t *ui.Theme) {
	ui.Column(c).FillWidth().Gap(10).Children(func() {
		ui.Text(c, "外观").Bold()
		ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
			ui.Text(c, "主题").TextColor(t.TextMuted).FontSize(t.FontSize - 1)
			ui.Text(c, "?").FontSize(t.FontSize - 2).TextColor(t.TextMuted).
				Tooltip("「跟随系统」会随操作系统的深浅色设置自动切换。")
		})
		mode := settingsService.ThemeMode()
		ui.Row(c).Gap(10).Children(func() {
			s.themeCard(c, t, "浅色", "light", iconSun, mode)
			s.themeCard(c, t, "深色", "dark", iconMoon, mode)
			s.themeCard(c, t, "跟随系统", "system", iconMonitor, mode)
		})
	})
}

// themeCard 是主题切换卡片：图标 + 文字；选中项高亮边框。
func (s *settingsViewState) themeCard(c *ui.Context, t *ui.Theme, label, mode string, icon *ui.SVG, current string) {
	selected := current == mode
	b := ui.ButtonBase(c).Label(label).Padding(8, 14).Radius(t.Radius).Width(120)
	if selected {
		b.BorderWidth(1.5).BorderColor(t.Text)
	} else {
		b.Background(t.Surface).BorderWidth(1).BorderColor(t.Border)
	}
	b.Children(func() {
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Icon(c, icon).Size(14, 14)
			label := ui.Text(c, label).MaxLines(1)
			if selected {
				label.Bold()
			}
		})
	})
	if b.Clicked() && !selected {
		if err := settingsService.SetThemeMode(mode); err == nil && allWindows != nil {
			allWindows.invalidateAll() // 所有窗口换主题
		}
	}
}

// ensureAutoStartLoaded 首帧同步读取开机自启状态（注册表/plist 查询很快）。
func (s *settingsViewState) ensureAutoStartLoaded() {
	if s.autoStartLoaded || settingsService == nil {
		return
	}
	s.autoStartLoaded = true
	status, err := settingsService.AutoStart()
	if err != nil {
		notifyUserTo(settingsWindowName, "error", "读取开机自启状态失败", err.Error())
		return
	}
	s.autoStart = status
}

func (s *settingsViewState) autoStartHint() string {
	if !s.autoStart.Supported {
		return "当前平台不支持开机自启。"
	}
	if s.autoStart.Target != "" {
		return "登录后自动启动：" + s.autoStart.Target
	}
	return "登录系统后自动启动 RcloneDrive（静默进入托盘）。"
}

// startupSection 对应旧「启动」分区：开机自启 Switch。
func (s *settingsViewState) startupSection(c *ui.Context, t *ui.Theme) {
	ui.Column(c).FillWidth().Gap(10).Children(func() {
		ui.Text(c, "启动").Bold()
		ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
			on := s.autoStart.Enabled
			switchChanged := false
			sw := ui.Switch(c, &on).Disabled(s.autoStartSaving || !s.autoStart.Supported)
			switchChanged = sw.Changed()
			ui.Text(c, "开机自启").Tooltip(s.autoStartHint())
			if s.autoStartSaving {
				ui.Spinner(c).Size(14, 14)
			}
			if switchChanged && on != s.autoStart.Enabled {
				s.setAutoStart(on)
			}
		})
		ui.Text(c, s.autoStartHint()).FontSize(t.FontSize - 2).TextColor(t.TextMuted)

		// 诊断入口：应用与 rclone 的运行日志都写在这里（internal/rclone.SetupLogging）
		ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
			if ui.Button(c, "打开日志文件夹").Clicked() {
				s.openLogsDir()
			}
			ui.Text(c, "应用与云盘连接的运行日志，排查连接问题时发给开发者").
				FontSize(t.FontSize - 2).TextColor(t.TextMuted)
		})
	})
}

// openLogsDir 打开日志目录（不存在时由 LogsDir 创建）。
func (s *settingsViewState) openLogsDir() {
	dir, err := rclone.LogsDir()
	if err != nil {
		notifyUserTo(settingsWindowName, "error", "打开日志文件夹失败", err.Error())
		return
	}
	if err := openDirectory(dir); err != nil {
		notifyUserTo(settingsWindowName, "error", "打开日志文件夹失败", err.Error())
	}
}

func (s *settingsViewState) setAutoStart(enabled bool) {
	s.autoStartSaving = true
	go func() {
		err := settingsService.SetAutoStart(enabled)
		s.autoStartSaving = false
		status, loadErr := settingsService.AutoStart()
		if loadErr == nil {
			s.autoStart = status
		}
		if err != nil {
			notifyUserTo(settingsWindowName, "error", "设置开机自启失败", err.Error())
		} else if enabled {
			notifyUserTo(settingsWindowName, "success", "已开启开机自启", "登录系统后自动启动 RcloneDrive（静默进入托盘）")
		} else {
			notifyUserTo(settingsWindowName, "success", "已关闭开机自启", "登录系统后不再自动启动 RcloneDrive")
		}
		if allWindows != nil {
			allWindows.invalidateAll()
		}
	}()
}

// updateSection 对应旧「更新」分区：自动检查更新 Switch、当前版本、
// 状态文本、下载进度、检查更新/立即重启按钮。
func (s *settingsViewState) updateSection(c *ui.Context, t *ui.Theme) {
	status := updateService.Status()
	progress := updateService.Progress()

	ui.Column(c).FillWidth().Gap(10).Children(func() {
		ui.Text(c, "更新").Bold()

		// 自动检查更新
		ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
			on := settingsService.AutoUpdate()
			changed := false
			sw := ui.Switch(c, &on).Disabled(s.autoUpdateSaving)
			changed = sw.Changed()
			ui.Text(c, "自动检查更新").Tooltip("默认关闭。开启后应用启动时及每 6 小时自动检查并下载新版本；下载的文件会经 SHA-256 校验通过后才允许重启生效")
			if s.autoUpdateSaving {
				ui.Spinner(c).Size(14, 14)
			}
			if changed && on != settingsService.AutoUpdate() {
				s.setAutoUpdate(on)
			}
		})

		ui.Row(c).Gap(6).Children(func() {
			ui.Text(c, "当前版本").TextColor(t.TextMuted)
			ui.Text(c, "v"+status.Current)
		})

		ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
			ui.Text(c, "项目主页").TextColor(t.TextMuted)
			ui.Link(c, "github.com/"+updateRepository, "https://github.com/"+updateRepository).
				Tooltip("点击在浏览器中打开项目主页，可查看源码、提交问题与下载历史版本")
		})

		ui.Column(c).Gap(4).Children(func() {
			ui.Text(c, "状态").TextColor(t.TextMuted).FontSize(t.FontSize - 1)
			ui.Text(c, s.statusText(status)).TextColor(t.TextMuted)
		})

		downloading := status.State == stateDownloading || status.State == stateVerifying || status.State == stateInstalling
		if downloading {
			percent := 0
			if progress.Total > 0 {
				percent = int(progress.Written * 100 / progress.Total)
			}
			ui.Column(c).Gap(4).Children(func() {
				ui.Progress(c, float64(percent)/100).FillWidth()
				ui.Textf(c, "%d%% · %s / %s", percent, formatSize(float64(progress.Written)), formatSize(float64(progress.Total))).
					FontSize(t.FontSize - 2).TextColor(t.TextMuted)
			})
		}

		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			busy := status.State == stateChecking || downloading
			if ui.Button(c, "检查更新").Disabled(busy).Clicked() {
				go updateService.CheckManual()
			}
			// 仅当已下载且 SHA-256 二次校验通过时才允许「立即重启」
			if status.Downloaded && status.Verified {
				if ui.PrimaryButton(c, "立即重启").Clicked() {
					s.restartConfirm = true
				}
			}
		})
	})

	if s.restartConfirm {
		if ui.AlertDialog(c, &s.restartConfirm, "重启以完成更新？",
			"重启期间所有云盘挂载会临时断开，重启完成后请重新连接。", "取消", "重启") == 1 {
			go func() {
				if err := updateService.Restart(); err != nil {
					notifyUserTo(settingsWindowName, "error", "重启失败", err.Error())
				}
			}()
		}
	}
}

func (s *settingsViewState) setAutoUpdate(enabled bool) {
	s.autoUpdateSaving = true
	go func() {
		err := settingsService.SetAutoUpdate(enabled)
		s.autoUpdateSaving = false
		if err != nil {
			notifyUserTo(settingsWindowName, "error", "设置自动更新失败", err.Error())
		} else if enabled {
			notifyUserTo(settingsWindowName, "success", "已开启自动检查更新", "应用启动后将自动检查并下载新版本（每 6 小时复查）")
			go updateService.CheckManual() // 开启后立即检查一次
		} else {
			notifyUserTo(settingsWindowName, "success", "已关闭自动检查更新", "应用不再自动检查更新，可在下方手动检查")
		}
		if allWindows != nil {
			allWindows.invalidateAll()
		}
	}()
}

// statusText 是更新状态的人类可读描述（对应旧 SettingsWindow.vue 的 statusText）。
func (s *settingsViewState) statusText(status UpdateStatus) string {
	switch status.State {
	case stateChecking:
		return "正在检查更新…"
	case stateAvailable:
		if status.Latest != "" {
			return "发现新版本 v" + status.Latest + "，准备下载…"
		}
		return "发现新版本，准备下载…"
	case stateVerifying:
		return "正在校验更新…"
	case stateInstalling:
		return "正在安装更新…"
	case stateError:
		return "检查更新失败"
	}
	if status.Downloaded && status.Verified {
		return "更新已就绪（校验通过），重启后生效"
	}
	if status.State == stateUpToDate {
		return "已是最新版本"
	}
	return "点击「检查更新」查看是否有新版本"
}
