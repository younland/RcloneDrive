//go:build windows

package main

import "testing"

// isElevated 的返回值取决于运行环境是否提权，这里只验证调用路径（syscall、
// token 查询）不会 panic；真实提权场景由装机验收覆盖。
func TestIsElevatedDoesNotPanic(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("isElevated 不应 panic: %v", r)
		}
	}()
	t.Logf("当前进程提权状态: %v", isElevated())
}
