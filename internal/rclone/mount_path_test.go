package rclone

import "testing"

// 实际挂载路径组装：子路径为空挂根（既有行为），非空拼 name:path。
func TestMountFsPath(t *testing.T) {
	cases := []struct {
		name, subpath, want string
	}{
		{"back", "", "back:"},
		{"back", "backup", "back:backup"},
		{"back", "a/b", "back:a/b"},
	}
	for _, c := range cases {
		if got := mountFsPath(c.name, c.subpath); got != c.want {
			t.Errorf("mountFsPath(%q, %q) = %q, want %q", c.name, c.subpath, got, c.want)
		}
	}
}
