package rclone

import (
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Task 3.3：日志初始化后，log 输出写入 <dir>/logs/rclonedrive.log。
// 用 MkdirTemp 而非 t.TempDir：lumberjack 的句柄由全局 Handler 持有，
// 进程内删不掉日志文件，TempDir 的严格清理会误报失败。
func TestSetupLoggingWritesFile(t *testing.T) {
	dir, err := os.MkdirTemp("", "rclonedrive-logtest")
	if err != nil {
		t.Fatalf("MkdirTemp: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })

	if err := setupLoggingAt(dir); err != nil {
		t.Fatalf("setupLoggingAt: %v", err)
	}

	log.Printf("setup-logging-probe")
	// lumberjack 按需创建文件；写一条后应已落盘
	data, err := os.ReadFile(filepath.Join(dir, "logs", "rclonedrive.log"))
	if err != nil {
		t.Fatalf("日志文件应产生: %v", err)
	}
	if !strings.Contains(string(data), "setup-logging-probe") {
		t.Fatalf("日志文件应包含写入内容，实际: %q", string(data))
	}
}

// LogsDir 在配置目录不存在时也应创建成功。
func TestLogsDirCreates(t *testing.T) {
	dir := t.TempDir()
	configRoot := filepath.Join(dir, "RcloneDrive")
	logsDir, err := logsDirAt(configRoot)
	if err != nil {
		t.Fatalf("logsDirAt: %v", err)
	}
	if info, err := os.Stat(logsDir); err != nil || !info.IsDir() {
		t.Fatalf("日志目录应被创建: %v", err)
	}
}
