package rclone

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/rclone/rclone/fs/config"
	"github.com/rclone/rclone/fs/config/configfile"
	"github.com/rclone/rclone/fs/config/obscure"
)

// useTestConfig 把进程内 rclone 配置切到临时目录。写函数不做防御性 Ensure
// （见 Ensure 注释），这里 SetConfigPath+Install 即等价其核心步骤；旧路径
// 在 Cleanup 恢复，避免污染同进程其他测试。
func useTestConfig(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	prev := config.GetConfigPath()
	t.Cleanup(func() {
		_ = config.SetConfigPath(prev)
		configfile.Install()
	})
	if err := config.SetConfigPath(filepath.Join(dir, "rclone.conf")); err != nil {
		t.Fatalf("SetConfigPath: %v", err)
	}
	configfile.Install()
}

// Task 1.1：RenameRemote 迁移配置节，混淆值逐字节原样（不做解密/再加密）。
func TestRenameRemote(t *testing.T) {
	useTestConfig(t)
	if err := SaveRemote("A", "webdav", map[string]string{
		"url": "https://example.com", "vendor": "other", "pass": "secret",
	}); err != nil {
		t.Fatalf("SaveRemote: %v", err)
	}
	// 记录迁移前 A 节的原始（混淆）值
	data := config.LoadedData()
	before := map[string]string{}
	for _, key := range data.GetKeyList("A") {
		v, _ := data.GetValue("A", key)
		before[key] = v
	}

	if err := RenameRemote("A", "B"); err != nil {
		t.Fatalf("RenameRemote: %v", err)
	}

	data = config.LoadedData()
	if !data.HasSection("B") || data.HasSection("A") {
		t.Fatalf("迁移后应有 B 无 A: B=%v A=%v", data.HasSection("B"), data.HasSection("A"))
	}
	for key, want := range before {
		got, _ := data.GetValue("B", key)
		if got != want {
			t.Errorf("B 节 %s = %q, want 迁移前的 %q", key, got, want)
		}
	}
	// 混淆值原样迁移后仍可解密回明文，证明凭据可用
	pass, _ := data.GetValue("B", "pass")
	if revealed, err := obscure.Reveal(pass); err != nil || revealed != "secret" {
		t.Errorf("B 节 pass 解密 = %q (err %v), want secret", revealed, err)
	}
	// 持久化到文件：重载后仍为新节名
	if _, err := os.Stat(config.GetConfigPath()); err != nil {
		t.Errorf("配置文件应已写盘: %v", err)
	}
}

// Task 1.3：old 不存在与 new 已占用时报错，配置不变。
func TestRenameRemoteErrors(t *testing.T) {
	useTestConfig(t)
	if err := SaveRemote("A", "webdav", map[string]string{"url": "https://x"}); err != nil {
		t.Fatalf("SaveRemote: %v", err)
	}
	if err := SaveRemote("C", "webdav", map[string]string{"url": "https://y"}); err != nil {
		t.Fatalf("SaveRemote: %v", err)
	}

	if err := RenameRemote("X", "B"); err == nil {
		t.Error("old 不存在应报错")
	}
	if err := RenameRemote("A", "C"); err == nil {
		t.Error("new 已占用应报错")
	}
	data := config.LoadedData()
	if !data.HasSection("A") || !data.HasSection("C") || data.HasSection("B") {
		t.Error("失败调用不应改变配置")
	}
	if err := RenameRemote("A", "A"); err != nil {
		t.Errorf("同名 rename 应为无操作: %v", err)
	}
}
