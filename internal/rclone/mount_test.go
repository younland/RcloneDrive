package rclone

import (
	"fmt"
	"strings"
	"testing"
)

// Task 1.1：挂载失败后 Log() 必须返回失败过程日志（当前实现丢弃，测试先红）。
func TestLogReturnsFailedLog(t *testing.T) {
	m := NewMountManager()
	log := newLogBuffer()
	fmt.Fprintf(log, "[%s] 挂载失败: mountpoint path already exists\n", stamp())
	m.failedLogs["web"] = log

	got := m.Log("web")
	if !strings.Contains(got, "挂载失败: mountpoint path already exists") {
		t.Fatalf("Log() 应返回失败日志，实际 %q", got)
	}
}

// Task 1.3：已挂载条目优先于失败日志；未挂载且无失败日志时返回空。
func TestLogPrefersMountedEntry(t *testing.T) {
	m := NewMountManager()
	failed := newLogBuffer()
	fmt.Fprintf(failed, "[%s] 挂载失败: old\n", stamp())
	m.failedLogs["web"] = failed
	mounted := newLogBuffer()
	fmt.Fprintf(mounted, "[%s] 挂载就绪\n", stamp())
	m.mounts["web"] = &mountEntry{log: mounted}

	if got := m.Log("web"); !strings.Contains(got, "挂载就绪") || strings.Contains(got, "old") {
		t.Fatalf("Log() 应优先返回已挂载日志，实际 %q", got)
	}
	if got := m.Log("other"); got != "" {
		t.Fatalf("未知驱动器 Log() 应为空，实际 %q", got)
	}
}

// Task 1.3：重新发起挂载时清除上一轮失败日志。
func TestClearFailedLog(t *testing.T) {
	m := NewMountManager()
	log := newLogBuffer()
	fmt.Fprintf(log, "[%s] 挂载失败: old\n", stamp())
	m.failedLogs["web"] = log

	m.clearFailedLog("web")
	if m.Log("web") != "" {
		t.Fatalf("clearFailedLog 后 Log() 应为空，实际 %q", m.Log("web"))
	}
	// 不存在的驱动器：不应 panic
	m.clearFailedLog("other")
}
