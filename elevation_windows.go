//go:build windows

package main

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

// isElevated 报告当前进程是否以提升（管理员）token 运行。
// 管理员运行的挂载盘符对非提权桌面会话不可见（UAC 的 per-LUID DosDevices 隔离），
// 提权时应用应警告用户改用普通权限。
func isElevated() bool {
	var token windows.Token
	if err := windows.OpenProcessToken(windows.CurrentProcess(), windows.TOKEN_QUERY, &token); err != nil {
		return false
	}
	defer token.Close()

	var elevation uint32
	var returned uint32
	err := windows.GetTokenInformation(token, windows.TokenElevation,
		(*byte)(unsafe.Pointer(&elevation)), uint32(unsafe.Sizeof(elevation)), &returned)
	if err != nil {
		return false
	}
	return elevation != 0
}
