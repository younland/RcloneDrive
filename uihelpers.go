package main

import (
	"fmt"
	"sync"
	"time"

	"github.com/egoist/mygo/ui"
)

// 视图共享的 UI 基建：暗色主题、自绘 WindowBar、图标、数值格式化、
// 应用内 toast（替代旧前端的 app:toast 事件 + sonner）。

// applyAppearance 按用户的外观设置决定本帧使用的主题，并应用到 Context。
// 「跟随系统」保留 Context 的默认主题（mygo 随系统深浅色）；「深色」用
// 旧前端的 #06070f 背景色，「浅色」用 ui 的浅色主题。
// 每个窗口每帧都会走到这里，resolvedTheme 据此把主题底色同步给系统窗口
// （见 syncChrome）：Linux 上 Frameless 窗口的默认表面是透明的，不设窗口
// 底色会透出桌面/其他窗口的内容（曾出现别的应用文字透进窗口的异常）。
func applyAppearance(c *ui.Context) *ui.Theme {
	mode := "system"
	if settingsService != nil {
		mode = settingsService.ThemeMode()
	}
	switch mode {
	case "dark":
		t := ui.DarkTheme()
		t.Background = ui.Hex("#06070f") // 旧前端 WindowBar/背景色
		c.SetTheme(t)
	case "light":
		c.SetTheme(ui.LightTheme())
	}
	t := c.Theme()
	syncChrome(themeHex(t.Background))
	return t
}

// syncChrome 把当前主题底色同步到所有系统窗口的底色（变化时才下发）。
// 窗口底色是帧内容铺不满/首帧未绘制时的兜底：透明表面在 Linux 上会透出
// 桌面内容，外观上与「文字显示异常」混在一起难以排查。
func syncChrome(hex string) {
	resolvedTheme.Lock()
	changed := resolvedTheme.hex != hex
	resolvedTheme.hex = hex
	resolvedTheme.Unlock()
	if changed && allWindows != nil {
		allWindows.syncChrome(hex)
	}
}

// resolvedTheme 记录最近一次渲染帧解析出的主题底色（含「跟随系统」解析结果）。
var resolvedTheme struct {
	sync.Mutex
	hex string
}

// themeHex 把 ui.Color 转成 #rrggbb 形式（窗口底色参数用）。
func themeHex(c ui.Color) string {
	return fmt.Sprintf("#%02x%02x%02x", c.R, c.G, c.B)
}

// initialChrome 返回创建窗口时的底色初值：跟随系统时交给运行时（留空），
// 首帧渲染后经 syncChrome 校正为解析后的主题底色。
func initialChrome() string {
	if settingsService == nil {
		return ""
	}
	switch settingsService.ThemeMode() {
	case "dark":
		return "#06070f"
	case "light":
		return "#ffffff"
	}
	return ""
}

// windowBar 是三个窗口共用的自绘标题栏（对应旧 components/WindowBar.vue +
// App.vue 的 header）：左侧标题区整片可拖动窗口（Frameless 无原生标题栏，
// DragWindow 由 ui 运行时转交给系统拖动），右侧动作按钮 + 关闭按钮。
// 关闭 = 隐藏到托盘（OnClose 拦截已保证，这里直接 Hide 语义一致）。
func windowBar(c *ui.Context, t *ui.Theme, title, subtitle string, actions func(), onClose func()) {
	ui.Row(c).FillWidth().Height(40).Padding(10, 12).Gap(8).AlignItems(ui.Center).
		BorderWidth(0, 0, 1, 0).BorderColor(t.Border).
		DragWindow().
		Children(func() {
			ui.Row(c).Gap(8).AlignItems(ui.Center).Grow(1).Shrink(1).Children(func() {
				// 标题栏左侧用应用图标（resources/appicon.png）
				if b := appIconBitmap(); b != nil {
					ui.Image(c, b).Size(16, 16).Radius(4).Shrink(0)
				} else {
					ui.Icon(c, iconHardDrive).Size(16, 16).TextColor(t.Text)
				}
				ui.Column(c).Gap(0).Shrink(1).Children(func() {
					ui.Text(c, title).Bold().MaxLines(1)
					if subtitle != "" {
						ui.Text(c, subtitle).FontSize(t.FontSize - 2).TextColor(t.TextMuted).MaxLines(1)
					}
				})
			})
			if actions != nil {
				actions()
			}
			iconButton(c, t, "关闭", iconClose, onClose)
		})
}

// appIconBitmap 解码标题栏用的应用图标（resources/appicon.png，见 tray.go
// 的 embed），解码失败时标题栏回退到硬盘 SVG 图标。
var appIconBitmap = sync.OnceValue(func() *ui.Bitmap {
	b, err := ui.DecodeBitmap(appIconPNG)
	if err != nil {
		return nil
	}
	return b
})

// iconButton 是标题栏/卡片上的图标按钮（ghost 风格，hover 高亮）。
func iconButton(c *ui.Context, t *ui.Theme, tooltip string, icon *ui.SVG, onClick func()) ui.Element {
	b := ui.ButtonBase(c).Label(tooltip).Padding(6).Radius(t.Radius)
	b.Children(func() {
		ui.Icon(c, icon).Size(14, 14)
	})
	if onClick != nil && b.Clicked() {
		onClick()
	}
	return b
}

// toastBus 是应用内 toast 的中转站：后端服务经 notifyUserTo 推入
// （target = 窗口名，对应旧版 app:toast 事件的 Target 字段），视图每帧
// drain 自己窗口的待显示 toast 交给 ui 绘制。窗口隐藏时通知走系统通知
// （见 notificationservice.go），与旧版语义一致。
type pendingToast struct {
	level string // success / error
	title string
	body  string
}

var toastBus = struct {
	sync.Mutex
	pending map[string][]pendingToast
}{pending: make(map[string][]pendingToast)}

// pushToast 记入 target 窗口的待显示队列并请求所有窗口重绘。
func pushToast(target, level, title, body string) {
	if target == "" {
		target = "main"
	}
	toastBus.Lock()
	toastBus.pending[target] = append(toastBus.pending[target], pendingToast{level, title, body})
	toastBus.Unlock()
	if allWindows != nil {
		allWindows.invalidateAll()
	}
}

// drainToasts 取出 target 窗口待显示的 toast，视图在构建末尾调用一次。
func drainToasts(c *ui.Context, target string) {
	if target == "" {
		target = "main"
	}
	toastBus.Lock()
	list := toastBus.pending[target]
	delete(toastBus.pending, target)
	toastBus.Unlock()
	for _, p := range list {
		c.AddToast(ui.Toast{
			Title:       p.title,
			Description: p.body,
			Type:        p.level, // ui 按 Type 调整外观（success/error）
			Timeout:     2 * time.Second,
		})
	}
}

// formatSpeed 把字节/秒转成易读文本（如 "1.2 MB/s"），移植自旧前端 lib/format.ts。
func formatSpeed(bytesPerSecond float64) string {
	if bytesPerSecond <= 0 {
		return "0 B/s"
	}
	units := []string{"B/s", "KB/s", "MB/s", "GB/s", "TB/s"}
	scaled, index := bytesPerSecond, 0
	for scaled >= 1024 && index < len(units)-1 {
		scaled /= 1024
		index++
	}
	digits := 2
	switch {
	case scaled >= 100:
		digits = 0
	case scaled >= 10:
		digits = 1
	}
	return fmt.Sprintf("%.*f %s", digits, scaled, units[index])
}

// formatSize 把字节数转成易读文本（如 "18.7 GB"，1024 进制），移植自旧前端 lib/format.ts。
func formatSize(bytes float64) string {
	if bytes <= 0 {
		return "0 B"
	}
	units := []string{"B", "KB", "MB", "GB", "TB", "PB"}
	scaled, index := bytes, 0
	for scaled >= 1024 && index < len(units)-1 {
		scaled /= 1024
		index++
	}
	digits := 1
	if scaled >= 100 {
		digits = 0
	}
	return fmt.Sprintf("%.*f %s", digits, scaled, units[index])
}

// selectLabeled 是下拉的 label↔value 变体：选项按标签展示，选中值直接
// 写回 value（SelectBase 绑定真实值指针，Item 的展示文本用 label）。
// 标签必须唯一（连接类型枚举的中文标签满足）；当前值不在列表时触发器
// 原样显示该值。
// 弹层用 SelectBase 自建并限高滚动——ui.Select 的弹层平铺全部选项且无法
// 滚动，选项多时（盘符最多 24 个）会被窗口底边截断（2026-10 盘符下拉修复）。
// 触发器与弹层样式对齐 ui.Select 的主题外观（mygo styleButton/stylePanel）。
func selectLabeled(c *ui.Context, t *ui.Theme, value *string, labels, values []string, placeholder string) {
	if len(labels) == 0 {
		return
	}
	shown := *value
	for i := range values {
		if values[i] == *value {
			shown = labels[i]
			break
		}
	}
	sel := ui.SelectBase(c, value)
	tr := sel.Trigger
	tr.Padding(t.Space(1.5), t.Space(3.5)).Gap(t.Space(1.5)).Radius(t.Radius).
		Background(t.Surface).TextColor(t.Text).Border(1, t.Border).
		Justify(ui.SpaceBetween).FillWidth()
	if tr.Hovered() {
		tr.Background(t.SurfaceHover)
	}
	if tr.Pressed() {
		tr.Background(t.SurfacePressed)
	}
	tr.Children(func() {
		ui.Text(c, shown).SingleLine()
		ui.Icon(c, iconChevronDown).Size(13, 13)
	})
	sel.Popup(func(panel ui.Element) {
		panel.Margin(t.Space(1), 0, 0, 0).Padding(t.Space(1)).Radius(t.Radius + 2).
			Background(t.Background).Border(1, t.Border).
			Shadow(0, 6, 20, 0, ui.RGBA(0, 0, 0, 0.18))
		// MaxHeight 约起滚动；选项少时收缩为内容高度，视觉与平铺一致
		ui.Scroll(c).MaxHeight(240).FillWidth().Children(func() {
		for i := range values {
			// Label 让可点击 item 自身可被命中（mygo 官方测试同款模式：
			// 仅 Children 文本时点击传导不到 item 的 Clicked 判定）
			item := sel.Item(values[i]).Padding(t.Space(1.5), t.Space(2.5)).Radius(t.Radius).Label(labels[i])
				switch {
				case item.Highlighted():
					item.Background(t.Accent).TextColor(t.AccentText)
				case values[i] == *value:
					item.Background(t.Surface)
				}
				item.Children(func() {
					ui.Text(c, labels[i]).SingleLine()
				})
			}
		})
	})
}

// --- 图标（24x24，描边风格，ui.Icon 以文字颜色着色；Parse 一次全局复用） ---

var (
	iconHardDrive   = mustIcon(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="black" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M3 16v-4a2 2 0 0 1 2-2h14a2 2 0 0 1 2 2v4"/><rect x="3" y="16" width="18" height="4" rx="1"/><circle cx="8" cy="18" r="0.5"/><circle cx="16" cy="18" r="0.5"/></svg>`)
	iconClose       = mustIcon(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="black" stroke-width="2" stroke-linecap="round"><path d="M6 6l12 12M18 6L6 18"/></svg>`)
	iconPlus        = mustIcon(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="black" stroke-width="2" stroke-linecap="round"><path d="M12 5v14M5 12h14"/></svg>`)
	iconSettings    = mustIcon(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="black" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M4 7h9M17 7h3"/><circle cx="15" cy="7" r="2"/><path d="M4 17h3M11 17h9"/><circle cx="9" cy="17" r="2"/></svg>`)
	iconFolderOpen  = mustIcon(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="black" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M3 8a2 2 0 0 1 2-2h4l2 2h8a2 2 0 0 1 2 2v8a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2z"/><path d="M3 13h18"/></svg>`)
	iconPlug        = mustIcon(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="black" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M9 3v5M15 3v5"/><path d="M7 8h10v4a5 5 0 0 1-10 0z"/><path d="M12 17v4"/></svg>`)
	iconUnplug      = mustIcon(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="black" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M9 3v5M15 3v5"/><path d="M7 8h10v2a5 5 0 0 1-10 0z"/><path d="M12 15v6"/></svg>`)
	iconPencil      = mustIcon(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="black" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M4 20l1-4L16.5 4.5a2.1 2.1 0 0 1 3 3L8 19z"/></svg>`)
	iconTrash       = mustIcon(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="black" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M4 7h16M9 7V5a1 1 0 0 1 1-1h4a1 1 0 0 1 1 1v2"/><path d="M6 7l1 13a1 1 0 0 0 1 1h8a1 1 0 0 0 1-1l1-13"/></svg>`)
	iconChevronDown = mustIcon(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="black" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M6 9l6 6 6-6"/></svg>`)
	iconSearch      = mustIcon(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="black" stroke-width="2" stroke-linecap="round"><circle cx="10.5" cy="10.5" r="6.5"/><path d="M15.5 15.5L21 21"/></svg>`)
	iconWarning     = mustIcon(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="black" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M12 4L2.5 20h19z"/><path d="M12 10v4M12 17.5v.5"/></svg>`)
	iconGlobe       = mustIcon(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="black" stroke-width="2" stroke-linecap="round"><circle cx="12" cy="12" r="9"/><path d="M3 12h18M12 3c3 3.5 3 14.5 0 18-3-3.5-3-14.5 0-18z"/></svg>`)
	iconCloud       = mustIcon(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="black" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M7 18a4.5 4.5 0 1 1 .8-8.9A5.5 5.5 0 0 1 18 11a3.5 3.5 0 0 1-1 6.9z"/></svg>`)
	iconDatabase    = mustIcon(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="black" stroke-width="2" stroke-linecap="round"><ellipse cx="12" cy="5.5" rx="8" ry="2.5"/><path d="M4 5.5V12c0 1.4 3.6 2.5 8 2.5s8-1.1 8-2.5V5.5"/><path d="M4 12v6.5c0 1.4 3.6 2.5 8 2.5s8-1.1 8-2.5V12"/></svg>`)
	iconFolder      = mustIcon(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="black" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M3 7a2 2 0 0 1 2-2h4l2 2h8a2 2 0 0 1 2 2v9a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2z"/></svg>`)
	iconNetwork     = mustIcon(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="black" stroke-width="2" stroke-linecap="round"><rect x="3" y="10" width="18" height="7" rx="1"/><path d="M7 17v3M17 17v3M6.5 13.5v.5M10 13.5v.5M14 13.5v.5M17.5 13.5v.5"/></svg>`)
	iconShield      = mustIcon(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="black" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M12 3l7 3v6c0 4.5-3 7.5-7 9-4-1.5-7-4.5-7-9V6z"/></svg>`)
	iconKey         = mustIcon(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="black" stroke-width="2" stroke-linecap="round"><circle cx="8" cy="15" r="4"/><path d="M11 12l8-8M15 8l3 3M18 5l2 2"/></svg>`)
	iconLayers      = mustIcon(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="black" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M12 3l9 5-9 5-9-5z"/><path d="M3 13l9 5 9-5"/></svg>`)
	iconArchive     = mustIcon(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="black" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect x="3" y="4" width="18" height="5" rx="1"/><path d="M5 9v10a1 1 0 0 0 1 1h12a1 1 0 0 0 1-1V9"/><path d="M10 13h4"/></svg>`)
	iconTerminal    = mustIcon(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="black" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect x="3" y="4" width="18" height="16" rx="2"/><path d="M7 9l3 3-3 3M12 15h5"/></svg>`)
	iconServer      = mustIcon(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="black" stroke-width="2" stroke-linecap="round"><rect x="3" y="4" width="18" height="7" rx="1"/><rect x="3" y="13" width="18" height="7" rx="1"/><circle cx="7" cy="7.5" r=".5"/><circle cx="7" cy="16.5" r=".5"/></svg>`)
	iconLink        = mustIcon(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="black" stroke-width="2" stroke-linecap="round"><path d="M10 14a5 5 0 0 0 7 0l3-3a5 5 0 0 0-7-7l-1.5 1.5"/><path d="M14 10a5 5 0 0 0-7 0l-3 3a5 5 0 0 0 7 7l1.5-1.5"/></svg>`)
	iconPower       = mustIcon(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="black" stroke-width="2" stroke-linecap="round"><path d="M12 3v8"/><path d="M6.3 6.5a8 8 0 1 0 11.4 0"/></svg>`)
	iconUpdate      = mustIcon(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="black" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="9"/><path d="M12 16v-6M9 11l3-3 3 3"/></svg>`)
	iconPalette     = mustIcon(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="black" stroke-width="2" stroke-linecap="round"><path d="M12 3a9 9 0 1 0 0 18c1.5 0 2-1 2-2 0-1.5 1-2 2-2h1a3 3 0 0 0 3-3c0-5-3.5-11-8-11z"/><circle cx="7.5" cy="11.5" r="1"/><circle cx="12" cy="7.5" r="1"/><circle cx="16.5" cy="11.5" r="1"/></svg>`)
	iconSun         = mustIcon(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="black" stroke-width="2" stroke-linecap="round"><circle cx="12" cy="12" r="4"/><path d="M12 2v2M12 20v2M2 12h2M20 12h2M4.9 4.9l1.4 1.4M17.7 17.7l1.4 1.4M4.9 19.1l1.4-1.4M17.7 6.3l1.4-1.4"/></svg>`)
	iconMoon        = mustIcon(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="black" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M20 13.5A8 8 0 1 1 10.5 4 6.5 6.5 0 0 0 20 13.5z"/></svg>`)
	iconMonitor     = mustIcon(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="black" stroke-width="2" stroke-linecap="round"><rect x="3" y="4" width="18" height="13" rx="2"/><path d="M9 21h6M12 17v4"/></svg>`)
)

func mustIcon(svg string) *ui.SVG {
	return ui.MustParseSVG([]byte(svg))
}

// protocolIcon 返回连接类型的图标（对应旧 components/ProtocolIcon.vue 的映射）。
func protocolIcon(typ string) *ui.SVG {
	switch typ {
	case "webdav", "http":
		return iconGlobe
	case "sftp":
		return iconTerminal
	case "ftp":
		return iconFolder
	case "smb":
		return iconNetwork
	case "local":
		return iconHardDrive
	case "memory", "s3", "azureblob", "azurefiles", "google cloud storage", "b2", "swift":
		return iconDatabase
	case "storj", "drive", "onedrive", "dropbox", "box", "mega", "pcloud", "yandex", "mailru", "google photos":
		return iconCloud
	case "protondrive", "hasher":
		return iconShield
	case "crypt":
		return iconKey
	case "alias":
		return iconLink
	case "union", "combine", "chunker":
		return iconLayers
	case "compress", "archive":
		return iconArchive
	case "cache":
		return iconFolderOpen
	default:
		return iconServer
	}
}
