// Command diag-about 对本机 RcloneDrive 配置里的远程执行 About 容量查询，
// 用于诊断“挂载后显示 1 PB 占位”类问题。只读操作，不写任何配置。
package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	_ "github.com/rclone/rclone/backend/all" // 注册全部后端
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config"
	"github.com/rclone/rclone/fs/config/configfile"
)


func main() {
	dir, err := os.UserConfigDir()
	if err != nil {
		fmt.Fprintln(os.Stderr, "UserConfigDir:", err)
		os.Exit(1)
	}
	if err := config.SetConfigPath(filepath.Join(dir, "RcloneDrive", "rclone.conf")); err != nil {
		fmt.Fprintln(os.Stderr, "SetConfigPath:", err)
		os.Exit(1)
	}
	configfile.Install()

	ctx := context.Background()
	for _, name := range []string{"back:", "back:backup"} {
		f, err := fs.NewFs(ctx, name)
		if err != nil {
			fmt.Printf("%-14s NewFs 失败: %v\n", name, err)
			continue
		}
		about := f.Features().About
		if about == nil {
			fmt.Printf("%-14s 后端不支持 About\n", name)
			continue
		}
		u, err := about(ctx)
		if err != nil {
			fmt.Printf("%-14s About 失败: %v\n", name, err)
			continue
		}
		if u.Total == nil {
			fmt.Printf("%-14s About 成功但无容量（Total=nil）→ rclone 会用 1 PiB 占位\n", name)
			continue
		}
		fmt.Printf("%-14s 总量=%.2f TB 可用=%.2f TB 已用=%.2f TB\n",
			name, float64(*u.Total)/(1<<40), float64(*u.Free)/(1<<40), float64(*u.Used)/(1<<40))
	}
}
