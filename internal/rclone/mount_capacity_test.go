package rclone

import (
	"context"
	"errors"
	"testing"

	"github.com/rclone/rclone/fs"
)

// 后端容量探测：手动配置优先；自动模式下拿不到真实 Total 时用虚拟总量，拿得到则不干预。
func TestDiskTotalOverride(t *testing.T) {
	t.Run("about 不支持", func(t *testing.T) {
		if got := diskTotalOverride(nil, 0); got != virtualDiskTotalBytes {
			t.Errorf("got %d, want 虚拟总量", got)
		}
	})
	t.Run("about 报错", func(t *testing.T) {
		got := diskTotalOverride(func(context.Context) (*fs.Usage, error) {
			return nil, errors.New("boom")
		}, 0)
		if got != virtualDiskTotalBytes {
			t.Errorf("got %d, want 虚拟总量", got)
		}
	})
	t.Run("about 无容量", func(t *testing.T) {
		got := diskTotalOverride(func(context.Context) (*fs.Usage, error) {
			return &fs.Usage{}, nil
		}, 0)
		if got != virtualDiskTotalBytes {
			t.Errorf("got %d, want 虚拟总量", got)
		}
	})
	t.Run("有真实容量", func(t *testing.T) {
		total := int64(123)
		got := diskTotalOverride(func(context.Context) (*fs.Usage, error) {
			return &fs.Usage{Total: &total}, nil
		}, 0)
		if got != -1 {
			t.Errorf("got %d, want -1（不干预真实值）", got)
		}
	})
	t.Run("手动配置优先", func(t *testing.T) {
		// 即使 about 为 nil（后端不支持）也直接用配置值
		got := diskTotalOverride(nil, fs.SizeSuffix(2<<40))
		if got != fs.SizeSuffix(2<<40) {
			t.Errorf("got %d, want 手动配置的 2TiB", got)
		}
		total := int64(123)
		got = diskTotalOverride(func(context.Context) (*fs.Usage, error) {
			return &fs.Usage{Total: &total}, nil
		}, fs.SizeSuffix(4<<30))
		if got != fs.SizeSuffix(4<<30) {
			t.Errorf("got %d, want 手动配置覆盖真实值", got)
		}
	})
}

func TestParseDiskTotal(t *testing.T) {
	cases := []struct {
		in   string
		want fs.SizeSuffix
	}{
		{"", 0},
		{"  ", 0},
		{"2TiB", fs.SizeSuffix(2 << 40)},
		{"512GiB", fs.SizeSuffix(512 << 30)},
		{"非法", 0},
		{"1.5TiB", fs.SizeSuffix(3 << 39)}, // SizeSuffix 支持小数
	}
	for _, c := range cases {
		if got := parseDiskTotal(c.in); got != c.want {
			t.Errorf("parseDiskTotal(%q) = %d, want %d", c.in, got, c.want)
		}
	}
}
