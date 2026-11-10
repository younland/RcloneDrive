package rclone

import (
	"context"
	"errors"
	"fmt"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/rclone/rclone/cmd/mountlib"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/accounting"
	"github.com/rclone/rclone/fs/rc"
	"github.com/rclone/rclone/vfs/vfscommon"
)

const (
	logMaxLines        = 200
	mountReadyTimeout  = 20 * time.Second
	mountReadyInterval = 200 * time.Millisecond
	// usageTimeout 是查询远端容量的超时（sftp 等后端要通过 SSH 跑 df）
	usageTimeout = 15 * time.Second
)

func stamp() string {
	return time.Now().Format(time.RFC3339)
}

// logBuffer 是固定行数的环形日志缓冲，记录一次挂载的过程信息。
type logBuffer struct {
	mu    sync.Mutex
	lines []string
	head  int
	count int
}

func newLogBuffer() *logBuffer {
	return &logBuffer{lines: make([]string, logMaxLines)}
}

func (b *logBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	text := strings.TrimRight(string(p), "\r\n")
	if text == "" {
		return len(p), nil
	}
	for _, line := range strings.Split(text, "\n") {
		b.lines[b.head] = line
		b.head = (b.head + 1) % logMaxLines
		if b.count < logMaxLines {
			b.count++
		}
	}
	return len(p), nil
}

func (b *logBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	// head 指向下一个待写入槽位，最早的一行在 (head-count) 处
	start := (b.head - b.count + logMaxLines) % logMaxLines
	out := make([]string, 0, b.count)
	for i := 0; i < b.count; i++ {
		out = append(out, b.lines[(start+i)%logMaxLines])
	}
	return strings.Join(out, "\n")
}

type mountEntry struct {
	point  *mountlib.MountPoint
	log    *logBuffer
	target string

	// statsCtx 带上该驱动器的 rclone 统计分组：写入（operations.Rcat）按 ctx 归属到它，
	// 读取则固定记在全局统计里（见 Speed）
	statsCtx context.Context
}

// MountManager 基于内嵌的 rclone mountlib 管理挂载的生命周期。
type MountManager struct {
	mu sync.Mutex
	mounts     map[string]*mountEntry
	// failedLogs 保留最近一次挂载失败的过程日志（成功路径在 mounts 里），
	// 否则失败瞬间日志随局部变量丢弃，界面无从排查
	failedLogs map[string]*logBuffer
	observers  []func(name string, mounted bool, errMsg string)
}

// NewMountManager 创建挂载管理器。
func NewMountManager() *MountManager {
	return &MountManager{
		mounts:     make(map[string]*mountEntry),
		failedLogs: make(map[string]*logBuffer),
	}
}

// clearFailedLog 清除指定远程的上一轮挂载失败日志（重新发起挂载时调用）。
func (m *MountManager) clearFailedLog(remote string) {
	m.mu.Lock()
	delete(m.failedLogs, remote)
	m.mu.Unlock()
}

// MountType 返回当前构建可用的挂载实现（如 cmount）；无可用实现时返回空字符串。
func MountType() string {
	typ, fn := mountlib.ResolveMountMethod("")
	if fn == nil {
		return ""
	}
	return typ
}

// MountAvailable 报告当前构建是否具备挂载能力。
func MountAvailable() bool {
	return MountType() != ""
}

// OnChange 注册挂载状态变更回调（挂载成功、卸载、异常结束）；可注册多个，互不覆盖。
func (m *MountManager) OnChange(fn func(name string, mounted bool, errMsg string)) {
	if fn == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.observers = append(m.observers, fn)
}

func (m *MountManager) emitChange(name string, mounted bool, errMsg string) {
	m.mu.Lock()
	observers := make([]func(string, bool, string), len(m.observers))
	copy(observers, m.observers)
	m.mu.Unlock()
	for _, fn := range observers {
		fn(name, mounted, errMsg)
	}
}

// friendlyMountError 把 rclone/WinFsp 的底层错误翻译成用户能理解的提示。
func friendlyMountError(target string, err error) error {
	if err == nil {
		return nil
	}
	msg := err.Error()
	if strings.Contains(msg, "mountpoint path already exists") {
		return fmt.Errorf("挂载点 %s 已被占用，请在驱动器设置中换一个盘符后重试", target)
	}
	return certHint(err)
}

// certHint 给证书类错误补上可操作的提示：自签名证书在自建服务器上很常见，
// 对应驱动器设置里的「忽略证书错误」。
func certHint(err error) error {
	if err == nil || !strings.Contains(err.Error(), "x509:") {
		return err
	}
	return fmt.Errorf("%w（若服务端使用自签名证书，请在编辑驱动器时勾选「忽略证书错误」）", err)
}

// MountOptions 是挂载的可选项。
type MountOptions struct {
	ReadOnly bool
	// Insecure 跳过 TLS 证书校验（自签名证书的自建服务器用）
	Insecure bool
	// DiskTotal 手动配置的显示容量（如 "2TiB"）；空表示自动探测：
	// 后端报告了真实容量就用真实值，否则按 virtualDiskTotalBytes 显示
	DiskTotal string
	// RemotePath 远程存储内的子路径（如 SMB 共享名）；空表示挂载远程根。
	// 容量探测作用于该实际路径；挂载状态索引仍按驱动器名
	RemotePath string
}

// mountFsPath 组装实际挂载的远程路径：name:path（子路径为空挂根）。
func mountFsPath(name, subpath string) string {
	if subpath == "" {
		return name + ":"
	}
	return name + ":" + subpath
}

// Mount 在当前进程中把远程存储挂载到 target（Windows 为盘符，如 "Z:"）。
func (m *MountManager) Mount(remote, target string, opt MountOptions) error {
	mountType, mountFn := mountlib.ResolveMountMethod("")
	if mountFn == nil {
		return errors.New("当前构建未启用挂载功能：请使用 -tags cmount 构建，并在 Windows 上安装 WinFsp")
	}
	if err := Ensure(); err != nil {
		return err
	}
	if m.IsMounted(remote) {
		return fmt.Errorf("%s 已挂载", remote)
	}
	m.clearFailedLog(remote)
	if runtime.GOOS != "windows" {
		if err := os.MkdirAll(target, 0o755); err != nil {
			return fmt.Errorf("创建挂载目录失败: %w", err)
		}
	}

	// 实际挂载 name:path（如 back:backup），驱动器状态仍按 name 索引
	fsPath := mountFsPath(remote, opt.RemotePath)
	f, err := fs.NewFs(ConfigCtx(context.Background(), opt.Insecure), fsPath)
	if err != nil {
		return fmt.Errorf("打开远程存储失败: %w", err)
	}

	vfsOpt := vfscommon.Opt
	vfsOpt.CacheMode = vfscommon.CacheModeWrites
	vfsOpt.ReadOnly = opt.ReadOnly
	// 服务器不报告容量（如 WebDAV 无 quota）时 rclone 会用 1 PiB 占位
	// （Windows 显示"1.00 PB 可用"），改为上报合理的虚拟总量或手动配置值
	vfsOpt.DiskSpaceTotalSize = diskTotalOverride(f.Features().About, parseDiskTotal(opt.DiskTotal))
	mountOpt := mountlib.Opt
	mountOpt.VolumeName = remote

	log := newLogBuffer()
	fmt.Fprintf(log, "[%s] 使用 %s 挂载 %s -> %s (只读=%v, 忽略证书错误=%v)\n", stamp(), mountType, fsPath, target, opt.ReadOnly, opt.Insecure)

	// 该驱动器的统计分组：写入路径（operations.Rcat）按 ctx 记账，用于读取写入速率
	statsCtx := accounting.WithStatsGroup(ConfigCtx(context.Background(), opt.Insecure), remote)

	point := mountlib.NewMountPoint(mountFn, target, f, &mountOpt, &vfsOpt)
	point.Ctx = statsCtx
	if _, err := point.Mount(); err != nil {
		friendly := friendlyMountError(target, err)
		fmt.Fprintf(log, "[%s] 挂载失败: %v\n", stamp(), err)
		m.mu.Lock()
		m.failedLogs[remote] = log
		m.mu.Unlock()
		return fmt.Errorf("挂载失败: %w", friendly)
	}

	if err := waitMountReady(point, log); err != nil {
		m.mu.Lock()
		m.failedLogs[remote] = log
		m.mu.Unlock()
		_ = point.Unmount()
		return err
	}

	entry := &mountEntry{point: point, log: log, target: point.MountPoint, statsCtx: statsCtx}
	m.mu.Lock()
	m.mounts[remote] = entry
	m.mu.Unlock()

	go func() {
		err := point.Wait()
		m.mu.Lock()
		delete(m.mounts, remote)
		m.mu.Unlock()
		if err != nil {
			friendly := friendlyMountError(point.MountPoint, err)
			fmt.Fprintf(log, "[%s] 挂载异常结束: %v\n", stamp(), err)
			m.emitChange(remote, false, friendly.Error())
			return
		}
		fmt.Fprintf(log, "[%s] 已卸载\n", stamp())
		m.emitChange(remote, false, "")
	}()

	m.emitChange(remote, true, "")
	return nil
}

// waitMountReady 等待挂载点可访问；Windows 盘符或目录出现即视为就绪。
func waitMountReady(point *mountlib.MountPoint, log *logBuffer) error {
	deadline := time.Now().Add(mountReadyTimeout)
	for {
		select {
		case err := <-point.ErrChan:
			if err == nil {
				err = errors.New("挂载意外结束")
			}
			fmt.Fprintf(log, "[%s] 挂载失败: %v\n", stamp(), err)
			return fmt.Errorf("挂载失败: %w", err)
		case <-time.After(mountReadyInterval):
		}
		if _, err := os.Stat(mountPath(point.MountPoint)); err == nil {
			fmt.Fprintf(log, "[%s] 挂载就绪: %s\n", stamp(), point.MountPoint)
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("等待挂载就绪超时: %s", point.MountPoint)
		}
	}
}

// mountPath 把 Windows 盘符补全为可 stat 的路径。
func mountPath(target string) string {
	if runtime.GOOS == "windows" && len(target) == 2 && target[1] == ':' {
		return target + `\`
	}
	return target
}

// virtualDiskTotalBytes 是后端拿不到容量时的虚拟总量（2 TiB）。
// rclone 对缺失的容量值填 1 PiB 占位（Windows 显示"1.00 PB 可用"），
// 上报一个合理值更可信；free 由 rclone 按 total-used 推得。
const virtualDiskTotalBytes = fs.SizeSuffix(2 << 40)

// parseDiskTotal 解析手动配置的显示容量（如 "2TiB"）；空或非法返回 0（自动）。
func parseDiskTotal(s string) fs.SizeSuffix {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	var size fs.SizeSuffix
	if err := size.Set(s); err != nil {
		fs.Logf(nil, "显示容量 %q 解析失败，改为自动探测: %v", s, err)
		return 0
	}
	return size
}

// diskTotalOverride 返回 vfs DiskSpaceTotalSize 的取值：手动配置优先；
// 自动模式下探测后端容量，拿不到真实 Total（后端不支持 About、请求失败
// 或不返回 quota）时返回虚拟总量，拿得到则返回 -1（不干预 rclone 真实值）。
func diskTotalOverride(about func(context.Context) (*fs.Usage, error), manual fs.SizeSuffix) fs.SizeSuffix {
	if manual > 0 {
		return manual
	}
	if about == nil {
		return virtualDiskTotalBytes
	}
	ctx, cancel := context.WithTimeout(context.Background(), usageTimeout)
	defer cancel()
	usage, err := about(ctx)
	if err != nil || usage == nil || usage.Total == nil {
		return virtualDiskTotalBytes
	}
	return -1
}

// Unmount 卸载指定远程，等待其挂载循环退出。
func (m *MountManager) Unmount(remote string) error {
	m.mu.Lock()
	entry, ok := m.mounts[remote]
	m.mu.Unlock()
	if !ok {
		return fmt.Errorf("%s 未挂载", remote)
	}
	if err := entry.point.Unmount(); err != nil {
		return fmt.Errorf("卸载失败: %w", err)
	}
	return nil
}

// IsMounted 报告指定远程当前是否已挂载。
func (m *MountManager) IsMounted(remote string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	_, ok := m.mounts[remote]
	return ok
}

// ListMounted 返回所有已挂载的远程名称。
func (m *MountManager) ListMounted() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	names := make([]string, 0, len(m.mounts))
	for name := range m.mounts {
		names = append(names, name)
	}
	return names
}

// Log 返回指定远程的挂载日志：优先当前挂载，未挂载时返回最近一次失败的过程日志。
func (m *MountManager) Log(remote string) string {
	m.mu.Lock()
	entry, ok := m.mounts[remote]
	if !ok {
		failed, failedOk := m.failedLogs[remote]
		m.mu.Unlock()
		if !failedOk {
			return ""
		}
		return failed.String()
	}
	m.mu.Unlock()
	return entry.log.String()
}

// Usage 描述挂载盘的空间用量（字节）。
type Usage struct {
	Total int64 `json:"total"`
	Used  int64 `json:"used"`
	Free  int64 `json:"free"`
}

// Usage 返回指定远程的容量信息；未挂载或后端不支持时返回 nil。
func (m *MountManager) Usage(remote string) *Usage {
	m.mu.Lock()
	entry, ok := m.mounts[remote]
	m.mu.Unlock()
	if !ok || entry.point == nil || entry.point.Fs == nil {
		return nil
	}
	about := entry.point.Fs.Features().About
	if about == nil {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), usageTimeout)
	defer cancel()
	usage, err := about(ctx)
	if err != nil || usage == nil {
		return nil
	}

	total, used, free := int64(0), int64(0), int64(0)
	if usage.Total != nil {
		total = *usage.Total
	}
	if usage.Used != nil {
		used = *usage.Used
	}
	if usage.Free != nil {
		free = *usage.Free
	}
	// 后端给的信息不一定全，能推的补上
	if free == 0 && total > 0 && used > 0 {
		free = total - used
	}
	if used == 0 && total > 0 && free > 0 {
		used = total - free
	}
	if total == 0 && free > 0 && used > 0 {
		total = free + used
	}
	if total <= 0 {
		return nil
	}
	return &Usage{Total: total, Used: used, Free: free}
}

// Usages 返回所有已挂载远程的容量信息（不支持的会被省略）。
func (m *MountManager) Usages() map[string]Usage {
	usages := make(map[string]Usage)
	for _, name := range m.ListMounted() {
		if usage := m.Usage(name); usage != nil {
			usages[name] = *usage
		}
	}
	return usages
}

// Target 返回指定远程的挂载目标（盘符或目录）。
func (m *MountManager) Target(remote string) string {
	m.mu.Lock()
	defer m.mu.Unlock()
	if entry, ok := m.mounts[remote]; ok {
		return entry.target
	}
	return ""
}

// Speed 是单个驱动器的实时速率（字节/秒）。
type Speed struct {
	Down float64 `json:"down"` // 下行：从挂载盘读取
	Up   float64 `json:"up"`   // 上行：写入挂载盘
}

// Speed 返回指定远程的实时速率；未挂载或无进行中的传输时返回零值。
//
// rclone 的挂载读写记在两个地方，都要取：
//   - 读取：vfs 的读路径固定写入全局统计（vfs/read.go 用的是 GlobalStats）
//   - 写入：operations.Rcat 按 ctx 记账，落在该驱动器的统计分组里
//
// 两边的「进行中传输」都带名称、实时速率与 srcFs/dstFs：
// dstFs 为空的是读取（下载），否则是写入（上传）。
func (m *MountManager) Speed(remote string) Speed {
	m.mu.Lock()
	entry, ok := m.mounts[remote]
	m.mu.Unlock()
	if !ok {
		return Speed{}
	}

	want := remote + ":"
	speed := transfersSpeed(accounting.GlobalStats(), want)
	if entry.statsCtx != nil {
		speed.Up += transfersSpeed(accounting.Stats(entry.statsCtx), want).Up
	}
	return speed
}

// Speeds 返回所有已挂载远程的实时速率。
func (m *MountManager) Speeds() map[string]Speed {
	speeds := make(map[string]Speed)
	for _, name := range m.ListMounted() {
		speeds[name] = m.Speed(name)
	}
	return speeds
}

// transfersSpeed 累加某份统计里属于该远程的进行中传输速率，并按方向分类。
// 归属依据：传输自带的 srcFs/dstFs（fs.ConfigString 形式，如 "远程名:"），
// 以及以 "远程名:" 开头的名称（有些传输不带 Fs 信息）。
func transfersSpeed(stats *accounting.StatsInfo, want string) Speed {
	var speed Speed
	params, err := stats.RemoteStats(false)
	if err != nil {
		return speed
	}
	transfers, ok := params["transferring"].([]rc.Params)
	if !ok {
		return speed
	}
	for _, transfer := range transfers {
		if !transferBelongsTo(transfer, want) {
			continue
		}
		value := transferSpeed(transfer)
		if value <= 0 {
			continue
		}
		if transferIsUpload(transfer) {
			speed.Up += value
		} else {
			speed.Down += value
		}
	}
	return speed
}

// transferSpeed 取一条传输的速率：
// 优先用 rclone 的近期移动平均（speedAvg，代表当前速率），它每个采样周期才更新一次，
// 传输刚开始时还是 0，此时退回累计平均（speed），否则短传输会完全没有数字。
func transferSpeed(transfer rc.Params) float64 {
	if value, ok := transfer["speedAvg"].(float64); ok && value > 0 {
		return value
	}
	if value, ok := transfer["speed"].(float64); ok && value > 0 {
		return value
	}
	return 0
}

// transferBelongsTo 判断一条传输是否属于指定远程。
func transferBelongsTo(transfer rc.Params, want string) bool {
	for _, key := range []string{"srcFs", "dstFs"} {
		if value, ok := transfer[key].(string); ok && strings.TrimSuffix(value, ":") == strings.TrimSuffix(want, ":") {
			return true
		}
	}
	name, _ := transfer["name"].(string)
	return strings.HasPrefix(name, want)
}

// transferIsUpload 判断传输方向：上传（写入挂载盘）的 dstFs 是挂载的 Fs，
// 读取则是 srcFs 指向它、dstFs 为空（见 vfs/read.go 与 operations.Rcat）。
func transferIsUpload(transfer rc.Params) bool {
	value, ok := transfer["dstFs"].(string)
	return ok && value != ""
}

// UnmountAll 卸载所有挂载，用于应用退出时清理。
func (m *MountManager) UnmountAll() {
	for _, name := range m.ListMounted() {
		_ = m.Unmount(name)
	}
}
