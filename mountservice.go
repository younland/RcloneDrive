package main

import (
	"fmt"
	"log"
	"maps"
	"sync"
	"time"

	"rclonedrive/internal/rclone"
)

// autoConnectDelay 自动连接的等待时间：等界面起来再挂载，避免与应用启动抢资源。
const autoConnectDelay = time.Second

// MountStatus 描述单个远程存储的挂载状态。
type MountStatus struct {
	Mounted bool   `json:"mounted"`
	Log     string `json:"log"`
}

// MountService 提供挂载/卸载操作并广播状态变更。
// 移植自 Wails 版："mount:changed" / "mount:speed" 事件改为回调订阅：
//   - 挂载状态变化通过 OnChange 多播（托盘菜单、界面据此刷新后调用查询方法取数）；
//   - 速率轮询 goroutine 保留（250ms 更新一次内部最新数据），UI 经 Speeds()/ListStatuses()
//     主动拉取，有变化时回调通知（窗口层收到通知后 Invalidate 重绘）。
type MountService struct {
	remotes *RemoteService
	mounts  *rclone.MountManager

	mu        sync.Mutex
	observers []func()
	speeds    map[string]rclone.Speed // 最近一次轮询到的速率
}

// NewMountService 创建挂载服务。
func NewMountService(remotes *RemoteService, mounts *rclone.MountManager) *MountService {
	return &MountService{remotes: remotes, mounts: mounts}
}

// Start 注册状态变更回调并启动后台任务（对应旧版的 ServiceStartup）。
// 应在应用就绪前调用；托盘创建后自行订阅 OnChange 做初始刷新。
func (s *MountService) Start() {
	s.mounts.OnChange(func(name string, mounted bool, errMsg string) {
		s.notifyChange()
		// 挂载在成功之后异常结束（连接中断等）时用系统通知提醒
		if !mounted && errMsg != "" {
			notifyUser("error", "已断开连接", "「"+name+"」："+errMsg)
		}
	})
	// 启动后台自动连接（标记了「自动连接」的驱动器）
	go s.autoConnect()
	// 有驱动器挂载时持续采样速率，供界面查询
	go s.watchSpeeds()
}

// OnChange 注册挂载状态/速率变化回调（用于刷新托盘菜单与界面重绘）；
// 可注册多个，互不覆盖。
func (s *MountService) OnChange(fn func()) {
	if fn == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.observers = append(s.observers, fn)
}

func (s *MountService) notifyChange() {
	s.mu.Lock()
	observers := make([]func(), len(s.observers))
	copy(observers, s.observers)
	s.mu.Unlock()
	for _, fn := range observers {
		fn()
	}
}

// autoConnect 依次挂载所有标记了「自动连接」且当前未挂载的驱动器。
// 单台失败只发通知并继续处理后面的，避免一台配错导致其余都不连。
func (s *MountService) autoConnect() {
	time.Sleep(autoConnectDelay)
	for _, remote := range s.remotes.List() {
		if !remote.AutoConnect || s.mounts.IsMounted(remote.Name) {
			continue
		}
		if err := s.MountQuiet(remote.Name); err != nil {
			notifyUser("error", "自动连接失败", fmt.Sprintf("「%s」：%v", remote.Name, err))
		}
	}
}

// Stop 退出时卸载所有挂载（对应旧版的 ServiceShutdown）。
func (s *MountService) Stop() {
	s.mounts.UnmountAll()
}

// Mount 挂载指定远程存储（用户手动触发：界面/托盘）；连接成功且驱动器
// 勾选「连接时自动打开」时自动打开挂载文件夹。
// 未设置挂载点时自动选择默认盘符（Windows）/ 独立子目录（Linux/macOS）。
func (s *MountService) Mount(name string) error {
	return s.mount(name, true)
}

// MountQuiet 同 Mount，但不触发「连接时自动打开」（开机自动连接等后台场景用）。
func (s *MountService) MountQuiet(name string) error {
	return s.mount(name, false)
}

func (s *MountService) mount(name string, allowAutoOpen bool) error {
	remote, ok := s.remotes.Get(name)
	if !ok {
		return fmt.Errorf("远程存储不存在: %s", name)
	}
	if rclone.MountType() != "" {
		if err := ensureWinFsp(); err != nil {
			return err
		}
		// Linux 挂载经 FUSE3（fusermount3），缺失时提示安装命令（需 root，不代装）
		if err := ensureFuse3(); err != nil {
			return err
		}
		// macOS 挂载经 macFUSE：缺失时引导安装，安装后需一次性手动批准扩展
		if err := ensureMacFUSE(); err != nil {
			return err
		}
	}
	target := remote.MountPoint
	if target == "" {
		// 留空时按平台分配默认挂载点（Windows 空闲盘符；Linux/macOS 为
		// mounts/<驱动器名> 子目录，各驱动器互不冲突）
		target = defaultMountTarget(remote.Name)
		if target == "" {
			return fmt.Errorf("没有可用的挂载点")
		}
	}
	if err := s.mounts.Mount(name, target, rclone.MountOptions{
		ReadOnly:   remote.ReadOnly,
		Insecure:   remote.Insecure,
		DiskTotal:  remote.DiskTotal,
		RemotePath: remote.RemotePath,
	}); err != nil {
		return err
	}
	if allowAutoOpen && remote.AutoOpen {
		if err := openDirectory(target); err != nil {
			notifyUser("error", "自动打开文件夹失败", fmt.Sprintf("「%s」：%v", name, err))
		}
	}
	return nil
}

// Unmount 卸载指定远程存储。
func (s *MountService) Unmount(name string) error {
	return s.mounts.Unmount(name)
}

// OpenFolder 在系统文件管理器中打开已连接驱动器对应的挂载目录。
func (s *MountService) OpenFolder(name string) error {
	if !s.mounts.IsMounted(name) {
		return fmt.Errorf("「%s」未连接，请先连接后再打开文件夹", name)
	}
	target := s.mounts.Target(name)
	if target == "" {
		return fmt.Errorf("驱动器「%s」没有可用的挂载点", name)
	}
	return openDirectory(target)
}

// Status 返回指定远程存储的挂载状态与最近日志。
func (s *MountService) Status(name string) MountStatus {
	return MountStatus{
		Mounted: s.mounts.IsMounted(name),
		Log:     s.mounts.Log(name),
	}
}

// ListStatuses 返回所有远程存储的挂载状态。
func (s *MountService) ListStatuses() []MountStatus {
	statuses := make([]MountStatus, 0)
	for _, remote := range s.remotes.List() {
		status := s.Status(remote.Name)
		statuses = append(statuses, status)
	}
	return statuses
}

// Speeds 返回各已挂载驱动器的实时速率（上下行，字节/秒）。
func (s *MountService) Speeds() map[string]rclone.Speed {
	return s.mounts.Speeds()
}

// Usages 返回各已挂载驱动器的容量信息（后端不支持的不会出现在结果里）。
func (s *MountService) Usages() map[string]rclone.Usage {
	return s.mounts.Usages()
}

// watchSpeeds 每 250ms 采样一次速率并缓存为内部最新数据，变化时通知观察者。
//
// rclone 内部只在 1 秒的 ticker 上重算速率（fs/accounting/accounting.go:218），
// 所以这里取 250ms：足够及时地更新「当前速率」，又不会做无意义的高频调用。
// 通知到达后由窗口层 Invalidate 重绘，UI 视图从 Speeds() 取数。
func (s *MountService) watchSpeeds() {
	const interval = 250 * time.Millisecond
	var last map[string]rclone.Speed
	lastActive := -1
	for {
		time.Sleep(interval)
		speeds := s.mounts.Speeds()
		if len(speeds) == 0 && len(last) == 0 {
			continue
		}
		active := 0
		for _, speed := range speeds {
			if speed.Down > 0 || speed.Up > 0 {
				active++
			}
		}
		if active != lastActive {
			log.Printf("[speed] 有流量的驱动器 %d 个", active)
			lastActive = active
		}
		if maps.Equal(speeds, last) {
			continue
		}
		last = speeds
		s.mu.Lock()
		s.speeds = speeds
		s.mu.Unlock()
		// 全部断开时通知一次（speeds 为空表），让界面清掉残留数字
		s.notifyChange()
	}
}
