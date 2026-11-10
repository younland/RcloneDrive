package main

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/rclone/rclone/fs/config"
	"github.com/rclone/rclone/fs/config/configfile"
)

// testRoot 是本包测试共用的隔离目录：rclone 配置与远程元数据都指到这里，
// 测试绝不读写真实用户配置（%AppData%/RcloneDrive）。
var testRoot string

// TestMain 统一隔离测试环境（2026-10：此前 headless 测试会读真实 rclone.conf，
// 并把"读取驱动器配置失败"弹成系统通知骚扰用户）。useTestEnv 的每测试隔离
// 在此之上继续生效，两者叠加不冲突。
func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "rclonedrive-test-*")
	if err != nil {
		fmt.Fprintln(os.Stderr, "TestMain 创建临时目录失败:", err)
		os.Exit(1)
	}
	testRoot = dir
	if err := config.SetConfigPath(filepath.Join(dir, "rclone.conf")); err != nil {
		fmt.Fprintln(os.Stderr, "TestMain SetConfigPath:", err)
		os.Exit(1)
	}
	configfile.Install()
	notifyNativeFn = func(string, string) {}
	code := m.Run()
	// 容忍清理失败（日志句柄可能仍被全局 Handler 持有，见 mount.go 沉淀经验）
	_ = os.RemoveAll(dir)
	os.Exit(code)
}
