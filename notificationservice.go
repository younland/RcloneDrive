package main

import "github.com/egoist/mygo"

// mainWindow 是主窗口，用于判断界面是否正在被看到（见 notifyUser）。
// 由 windowManager 在创建主窗口后赋值。
var mainWindow *mygo.Window

// windowByName 按名称查找窗口；主窗口兼容旧的无名引用。
// 窗口统一由 windowManager 管理（mygo 只有 WindowByID，没有 GetByName）。
func windowByName(name string) *mygo.Window {
	if name == "main" && mainWindow != nil {
		return mainWindow
	}
	if allWindows != nil {
		return allWindows.find(name)
	}
	return nil
}

// notifyUser 提示用户：窗口可见时发应用内 toast（uihelpers.go 的 toastBus，
// 对应旧版 app:toast 事件 + sonner），隐藏到托盘时改用系统通知
// （窗口藏着的时候 toast 看不到，那条提示就白发了）。
func notifyUser(level, title, body string) {
	notifyUserTo("main", level, title, body)
}

// notifyNativeFn 是系统通知的出口，测试中替换为静默实现：headless 测试
// 进程没有可见窗口，所有通知都会落到这条通道，不应真的弹系统气泡
// （2026-10：跑 go test 时曾向用户连弹十余条"读取驱动器配置失败"）。
var notifyNativeFn = notifyNative

// notifyUserTo 与 notifyUser 相同，但 toast 出现在 target 指定的窗口上
// （该窗口不可见时同样回落到系统通知）。target 为空按主窗口处理。
func notifyUserTo(target, level, title, body string) {
	if target == "" {
		target = "main"
	}
	if window := windowByName(target); window != nil && window.IsVisible() {
		pushToast(target, level, title, body)
		return
	}
	notifyNativeFn(title, body)
}

// notifyNativeOnly 只发系统通知，不在任何窗口弹 toast。
// 连接/断开这类动作本身有界面状态反馈（卡片状态、托盘菜单），不需要窗口提示。
func notifyNativeOnly(title, body string) {
	notifyNativeFn(title, body)
}
