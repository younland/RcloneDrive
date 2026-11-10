package main

import (
	"log"
	"os"
	"slices"

	"rclonedrive/internal/rclone"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

// 服务实例是包级变量：视图（mainview/driveeditorview/settingsview）在同一
// 进程内直接调用，与旧前端经 bindings 调用的服务一一对应。
var (
	mountManager    *rclone.MountManager
	remoteService   *RemoteService
	mountService    *MountService
	systemService   = &SystemService{}
	settingsService *SettingsService
	updateService   *UpdateService
)

func main() {
	// 最先接日志：之后的 rclone/应用日志统一落盘（windowsgui 无控制台）
	if err := rclone.SetupLogging(); err != nil {
		log.Printf("日志落盘初始化失败（仅本次进程内输出）: %v", err)
	}

	if err := rclone.Ensure(); err != nil {
		log.Printf("rclone 配置初始化失败: %v", err)
	}

	// --tray：开机自启时静默启动到托盘，不弹出窗口
	startHidden := hasTrayFlag(os.Args[1:])

	// 单实例：重复启动时新进程只把命令行交给已有实例（见 OnSecondInstance）。
	// 开机自启（--tray）重复启动不打扰用户，直接静默退出。
	if !mygo.App.RequestSingleInstanceLock() {
		return
	}

	mountManager = rclone.NewMountManager()
	remoteService = NewRemoteService(mountManager)
	mountService = NewMountService(remoteService, mountManager)
	settingsService = &SettingsService{}
	updateService = NewUpdateService(settingsService)

	// 服务在应用就绪前初始化：加载本地元数据、注册挂载状态回调、
	// 启动自动连接与速率采样的后台 goroutine。
	if err := remoteService.Start(); err != nil {
		log.Printf("远程存储列表加载失败: %v", err)
	}
	mountService.Start()
	updateService.Start()

	mygo.App.WhenReady(func() {
		// 任务栏图标：裸 exe（go run/task build）没有内嵌图标，运行时设置。
		if err := mygo.App.Dock.SetIcon(appIconPNG); err != nil {
			log.Printf("设置应用图标失败: %v", err)
		}
		window := mygo.NewWindow(mygo.WindowOptions{
			Title:  "RcloneDrive",
			Width:  580,
			Height: 540,
			// 固定尺寸、无边框：标题栏由视图的 WindowBar 自绘（对应旧前端的
			// Frameless + --wails-draggable），关闭=隐藏到托盘
			DisableResize:   true,
			Frameless:       true,
			Hidden:          startHidden,
			BackgroundColor: initialChrome(),
			StateKey:        "main",
			Content:         ui.View(mainWindowView.view),
		})

		windows := newWindowManager(window)
		// 关闭窗口 = 隐藏到托盘，挂载保持运行；quitting 时不拦截（见 quit）。
		window.OnClose(func(e *mygo.CloseEvent) {
			if windows.quitting.Load() {
				return
			}
			e.PreventDefault()
			window.Hide()
		})

		// 服务状态变化后让所有窗口重绘；UI 视图重绘时经服务查询方法取最新数据
		refreshUI := func() {
			if allWindows != nil {
				allWindows.invalidateAll()
			}
		}
		mountService.OnChange(refreshUI)
		remoteService.OnChange(refreshUI)
		updateService.OnChange(refreshUI)

		if _, err := newTrayManager(windows, remoteService, mountService, updateService); err != nil {
			log.Printf("托盘创建失败: %v", err)
		}

		// 管理员运行的挂载盘符对桌面（非提权）不可见，启动即警告（见 specs/mount）
		if isElevated() {
			notifyUser("error", "正在以管理员身份运行",
				"管理员运行的挂载盘符对桌面不可见，请退出后以普通权限重新启动 RcloneDrive。")
		}
	})

	// 第二个实例的显示请求：弹出主界面（带 --tray 的静默启动直接忽略）
	mygo.App.OnSecondInstance(func(args []string, _ string) {
		if hasTrayFlag(args) {
			return
		}
		if allWindows != nil {
			allWindows.showMainWindow()
		}
	})

	// 退出时停止后台任务、卸载所有挂载
	mygo.App.OnQuit(func() {
		updateService.Stop()
		mountService.Stop()
	})

	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}

// hasTrayFlag 报告参数里是否有静默启动到托盘的标记。
func hasTrayFlag(args []string) bool {
	return slices.Contains(args, trayFlag)
}
