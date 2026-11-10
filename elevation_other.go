//go:build !windows

package main

// isElevated 报告当前进程是否提权运行；仅 Windows 存在 UAC 盘符可见性问题，
// 其他平台恒为 false。
func isElevated() bool { return false }
