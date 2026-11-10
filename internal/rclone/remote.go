package rclone

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/rclone/rclone/fs"
)

const testTimeout = 20 * time.Second

// ConfigCtx 返回带 rclone 配置的 context。insecure 为 true 时跳过 TLS 证书校验：
// rclone 只有全局开关 --no-check-certificate，这里用 AddConfig 复制一份配置按单个远程存储设置，
// 不影响其他驱动器（HTTP 后端在 NewFs 时经 fshttp.NewClient(ctx) 读取它，见 fs/fshttp/http.go）。
func ConfigCtx(ctx context.Context, insecure bool) context.Context {
	if !insecure {
		return ctx
	}
	ctx, cfg := fs.AddConfig(ctx)
	cfg.InsecureSkipVerify = true
	return ctx
}

// TestRemote 在进程内建立连接并列出根目录，验证远程存储是否可用。
func TestRemote(name string, insecure bool) error {
	ctx, cancel := context.WithTimeout(ConfigCtx(context.Background(), insecure), testTimeout)
	defer cancel()

	f, err := fs.NewFs(ctx, name+":")
	if err != nil {
		return fmt.Errorf("连接测试失败: %w", certHint(err))
	}
	if _, err := f.List(ctx, ""); err != nil && !errors.Is(err, fs.ErrorDirNotFound) {
		return fmt.Errorf("连接测试失败: %w", certHint(err))
	}
	return nil
}
