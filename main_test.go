package main

import (
	"path/filepath"
	"strings"
	"testing"

	"rclonedrive/internal/rclone"

	"github.com/egoist/mygo/ui"
)

// 视图冒烟测试：ui.Tester 在无窗口环境下渲染视图、读文本、模拟点击。
// 服务与 rclone 配置由 TestMain 统一隔离到临时目录（testRoot），不读写
// 真实用户配置、不弹系统通知，覆盖各窗口的关键渲染路径。

func testServices() {
	mountManager = rclone.NewMountManager()
	remoteService = NewRemoteService(mountManager)
	// 元数据指向 TestMain 的隔离目录（NewRemoteService 默认用真实用户目录）
	remoteService.metaPath = filepath.Join(testRoot, "remotes.json")
	mountService = NewMountService(remoteService, mountManager)
	systemService = &SystemService{}
	settingsService = &SettingsService{}
	updateService = NewUpdateService(settingsService)
}

// TestMainView 主窗口：标题栏、空状态、添加驱动器按钮。
func TestMainView(t *testing.T) {
	testServices()
	state := &mainViewState{}
	tt := ui.NewTester(state.view, 580, 540)
	if !tt.HasText("RcloneDrive") {
		t.Fatalf("texts %q do not contain the app title", tt.Texts())
	}
	if !tt.HasText("还没有虚拟驱动器") {
		t.Fatalf("texts %q do not contain the empty state", tt.Texts())
	}
	if err := tt.Click("添加驱动器"); err != nil {
		// allWindows 为 nil 时点击是空操作，不应出错
		t.Fatalf("click 添加驱动器: %v", err)
	}
}

// TestDriveEditorView 驱动器编辑窗口：标题、类型树、表单与底部按钮。
func TestDriveEditorView(t *testing.T) {
	testServices()
	state := newDriveEditorState()
	tt := ui.NewTester(state.view, driveWindowW, driveWindowH)

	if !tt.HasText("新虚拟驱动器") {
		t.Fatalf("texts %q do not contain the window title", tt.Texts())
	}
	// 左侧类型树默认展开「常用协议」分组（第一个）
	if !tt.HasText("常用协议") {
		t.Fatalf("texts %q do not contain the first catalog group", tt.Texts())
	}
	// 右侧表单：名称 + 连接设置 + 底部按钮
	if !tt.HasText("名称") || !tt.HasText("连接设置") {
		t.Fatalf("texts %q do not contain the form sections", tt.Texts())
	}
	for _, label := range []string{"测试连接", "取消", "保存", "保存并连接"} {
		if !tt.HasText(label) {
			t.Fatalf("texts %q do not contain footer button %q", tt.Texts(), label)
		}
	}

	// 盘符下拉弹层：展开后全部盘符都构建在弹层里（限高滚动可滚到全部，
	// 2026-10 修复弹层被窗口底边截断且无法滚动的问题）。默认盘符必然在
	// 列表里，触发器按其标签可点。
	letters := state.driveLetters()
	if len(letters) >= 2 {		if err := tt.Click(strings.TrimSuffix(state.mountPoint, ":")); err != nil {
			t.Fatalf("click 盘符下拉触发器: %v", err)
		}
		first := strings.TrimSuffix(letters[0], ":")
		last := strings.TrimSuffix(letters[len(letters)-1], ":")
		if !tt.HasText(first) || !tt.HasText(last) {
			t.Fatalf("盘符弹层应包含首尾选项 %q/%q, texts %q", first, last, tt.Texts())
		}
		target := letters[1] // 视口内的第二项（首项是当前默认值；列表末尾在
		// MaxHeight 裁剪区外，点击穿透属正常滚动行为，headless 无法模拟滚动）
		if target != state.mountPoint {
			// 先 Move 再 Click：item 首次捕获指针会标记输入 consumed 并
			// 重建一帧，跳过 Move 的直接点击会因 Down/Up 分属两个构建
			// pass 而选不中（真实鼠标移动不受影响，官方测试同款顺序）
			if r, ok := tt.Find(strings.TrimSuffix(target, ":")); ok {
				tt.Move(r.X+r.W/2, r.Y+r.H/2)
			}
			tt.Click(strings.TrimSuffix(target, ":")) // 选择后写回真实值（含冒号）
			tt.Frame()
			if state.mountPoint != target {
				t.Fatalf("选择 %s 后 mountPoint = %q", target, state.mountPoint)
			}
		}
	}

	// 说明页
	state.tab = 2
	tt.Frame()
	if !tt.HasText("文档") {
		t.Fatalf("texts %q do not contain the about tab", tt.Texts())
	}
}

// TestSettingsView 设置窗口：标题栏与三个分区。
func TestSettingsView(t *testing.T) {
	testServices()
	state := &settingsViewState{}
	tt := ui.NewTester(state.view, settingsWindowW, settingsWindowH)
	for _, label := range []string{"设置", "外观", "启动", "更新"} {
		if !tt.HasText(label) {
			t.Fatalf("texts %q do not contain %q", tt.Texts(), label)
		}
	}
	// 切到「更新」分区：当前版本与检查更新按钮
	if err := tt.Click("更新"); err != nil {
		t.Fatalf("click 更新: %v", err)
	}
	if !tt.HasText("当前版本") || !tt.HasText("检查更新") {
		t.Fatalf("texts %q do not contain the update section", tt.Texts())
	}

	// 「启动」分区的日志入口：只验证渲染，不点击（点击会真实打开资源管理器）
	if err := tt.Click("启动"); err != nil {
		t.Fatalf("click 启动: %v", err)
	}
	if !tt.HasText("打开日志文件夹") {
		t.Fatalf("texts %q do not contain the logs entry", tt.Texts())
	}
}

// TestDriveEditorRenameEditable 编辑态：名称回填且可编辑（改名功能，
// 2026 add-drive-rename）。注入内存条目，不落盘、不污染真实 remotes.json。
func TestDriveEditorRenameEditable(t *testing.T) {
	testServices()
	remoteService.remotes = append(remoteService.remotes, Remote{
		Name: "测试盘", Type: "webdav", MountPoint: "F:",
		Options: map[string]string{"url": "https://x", "vendor": "other"},
	})
	state := newDriveEditorState()
	state.setTarget("测试盘")
	tt := ui.NewTester(state.view, driveWindowW, driveWindowH)
	tt.Frame() // ensureLoaded 同步回填
	// 名称输入框的值不进 labels 表，直接断言懒加载回填的状态
	if state.name != "测试盘" {
		t.Fatalf("编辑态应回填名称, got %q", state.name)
	}
}

// TestDriveEditorDiskTotal 显示容量：界面渲染 + 编辑态回填 + 保存携带（
// 2026-10 容量显示修复：后端不报告容量时手动指定驱动器显示总量）。
func TestDriveEditorDiskTotal(t *testing.T) {
	testServices()
	remoteService.remotes = append(remoteService.remotes, Remote{
		Name: "容量盘", Type: "webdav", MountPoint: "F:", DiskTotal: "2TiB",
		Options: map[string]string{"url": "https://x", "vendor": "other"},
	})
	state := newDriveEditorState()
	tt := ui.NewTester(state.view, driveWindowW, driveWindowH)
	if !tt.HasText("显示容量") {
		t.Fatalf("texts %q do not contain 显示容量", tt.Texts())
	}
	if state.diskTotal != "" {
		t.Fatalf("新增态显示容量应为空(自动), got %q", state.diskTotal)
	}
	state.setTarget("容量盘")
	tt.Frame() // ensureLoaded 同步回填
	if state.diskTotal != "2TiB" {
		t.Fatalf("编辑态应回填显示容量, got %q", state.diskTotal)
	}
	if got := state.buildRemote().DiskTotal; got != "2TiB" {
		t.Fatalf("保存应携带显示容量, got %q", got)
	}
}

// TestDriveEditorRemotePath SMB 共享路径输入：界面渲染 + 编辑态回填 + 保存携带
// （2026-10 SMB 共享挂载修复：后端无 root 配置项时显示"共享路径/远程路径"）。
func TestDriveEditorRemotePath(t *testing.T) {
	testServices()
	// 写入真实的 rclone 配置节，让编辑窗的配置回读（Options）完整走通
	if err := rclone.SaveRemote("SMB盘", "smb", map[string]string{
		"host": "192.0.2.10", "user": "demo", "pass": "secret",
	}); err != nil {
		t.Fatalf("SaveRemote: %v", err)
	}
	remoteService.remotes = append(remoteService.remotes, Remote{
		Name: "SMB盘", Type: "smb", MountPoint: "K:", RemotePath: "backup/照片",
		Options: map[string]string{"host": "192.0.2.10", "user": "demo"},
	})
	state := newDriveEditorState()
	state.setTarget("SMB盘")
	tt := ui.NewTester(state.view, driveWindowW, driveWindowH)
	tt.Frame() // ensureLoaded 同步回填
	if !tt.HasText("共享路径") {
		t.Fatalf("smb 类型应显示共享路径输入, texts %q", tt.Texts())
	}
	if state.remotePath != "backup/照片" {
		t.Fatalf("编辑态应回填共享路径, got %q", state.remotePath)
	}
	if got := state.buildRemote().RemotePath; got != "backup/照片" {
		t.Fatalf("保存应携带共享路径, got %q", got)
	}
	// 配置回读应把 host 填回表单选项（凭据从 rclone.conf 解密回读）
	if host := state.options["host"]; host != "192.0.2.10" {
		t.Fatalf("配置回读应回填 host, got %q", host)
	}
}

// TestFormatters 数值格式化与旧前端 lib/format.ts 对齐。
func TestFormatters(t *testing.T) {
	sizeCases := []struct {
		bytes float64
		want  string
	}{
		{0, "0 B"},
		{512, "512 B"},
		{2048, "2.0 KB"},
		{18.7 * 1024 * 1024 * 1024, "18.7 GB"},
	}
	for _, tc := range sizeCases {
		if got := formatSize(tc.bytes); got != tc.want {
			t.Errorf("formatSize(%v) = %q, want %q", tc.bytes, got, tc.want)
		}
	}
	speedCases := []struct {
		bps  float64
		want string
	}{
		{0, "0 B/s"},
		{100, "100 B/s"},
		{1536, "1.50 KB/s"},
		{12.345 * 1024 * 1024, "12.3 MB/s"},
	}
	for _, tc := range speedCases {
		if got := formatSpeed(tc.bps); got != tc.want {
			t.Errorf("formatSpeed(%v) = %q, want %q", tc.bps, got, tc.want)
		}
	}
}

// TestProviderMatches 枚举字段的 provider 过滤（DriveWindow.vue 的 providerMatches）。
func TestProviderMatches(t *testing.T) {
	cases := []struct {
		filter, provider string
		want             bool
	}{
		{"", "anything", true},
		{"aws", "aws", true},
		{"aws", "gcp", false},
		{"aws,gcp", "gcp", true},
		{"!aws", "aws", false},
		{"!aws", "gcp", true},
		{"aws,!secret", "aws", true},
		{"aws,!aws", "aws", false},
		{"", "", true},
	}
	for _, tc := range cases {
		if got := providerMatches(tc.filter, tc.provider); got != tc.want {
			t.Errorf("providerMatches(%q, %q) = %v, want %v", tc.filter, tc.provider, got, tc.want)
		}
	}
}

// TestSegmentFields 连续 bool 字段分段（DriveWindow.vue 的 segmentFields）。
func TestSegmentFields(t *testing.T) {
	fields := []rclone.Field{
		{Key: "a", Type: "string"},
		{Key: "b", Type: "bool"},
		{Key: "c", Type: "bool"},
		{Key: "d", Type: "string"},
	}
	segs := segmentFields(fields)
	if len(segs) != 3 {
		t.Fatalf("segmentFields returned %d segments, want 3", len(segs))
	}
	if len(segs[1].bools) != 2 {
		t.Fatalf("middle segment has %d bools, want 2", len(segs[1].bools))
	}
	if segs[0].field.Key != "a" || segs[2].field.Key != "d" {
		t.Fatalf("unexpected field segments: %+v", segs)
	}
}
