package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"rclonedrive/internal/rclone"
)

var remoteNamePattern = regexp.MustCompile(`^\S+$`)

// Remote 描述一个远程存储及其本地挂载偏好。
// Options 只作为 Create/Update 的请求负载在进程内传递，不落盘：
// 凭证明文存 remotes.json 会被本机其他进程收割，配置只保存在 rclone.conf（密码已混淆）。
type Remote struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	MountPoint  string `json:"mountPoint"`
	AutoConnect bool   `json:"autoConnect"`
	ReadOnly    bool   `json:"readOnly"`
	// Insecure 为 true 时该驱动器的挂载跳过 TLS 证书校验（自签名证书用）
	Insecure bool `json:"insecure"`
	// DiskTotal 手动配置的显示容量（如 "2TiB"）；空表示自动探测后端容量
	DiskTotal string `json:"diskTotal,omitempty"`
	// RemotePath 远程存储内的子路径（SMB 场景为共享名）；空表示挂载远程根。
	// 不写入 rclone 配置——smb 后端无 root 配置项，共享只能经 name:path 路径语法传入
	RemotePath string `json:"remotePath,omitempty"`
	// AutoOpen 连接成功后自动打开挂载文件夹（仅手动连接触发，开机自动连接不触发）
	AutoOpen bool              `json:"autoOpen"`
	Options  map[string]string `json:"options,omitempty"`
}

// RemoteService 管理远程存储的增删改查与连接测试。
// 移植自 Wails 版：去掉了 application.App 依赖与 "remotes:changed" 事件，
// 列表变化通过 OnChange 回调多播，UI 主动调用 List/Get 等查询方法取数。
type RemoteService struct {
	mu        sync.RWMutex
	remotes   []Remote
	metaPath  string
	mounts    *rclone.MountManager
	observers []func()
}

// NewRemoteService 创建服务，元数据存于 <ConfigDir>/remotes.json。
func NewRemoteService(mounts *rclone.MountManager) *RemoteService {
	return &RemoteService{
		metaPath: filepath.Join(rclone.ConfigDir(), "remotes.json"),
		mounts:   mounts,
	}
}

// Start 启动时加载本地元数据（对应旧版的 ServiceStartup）。
func (s *RemoteService) Start() error {
	return s.load()
}

// OnChange 注册远程列表变化回调（用于刷新托盘菜单与界面）；可注册多个，互不覆盖。
func (s *RemoteService) OnChange(fn func()) {
	if fn == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.observers = append(s.observers, fn)
}

func (s *RemoteService) notifyChange() {
	s.mu.RLock()
	observers := make([]func(), len(s.observers))
	copy(observers, s.observers)
	s.mu.RUnlock()
	// 在锁外回调，避免观察者里再次访问 s.mu 造成死锁
	for _, fn := range observers {
		if fn == nil {
			continue
		}
		fn()
	}
}

func (s *RemoteService) load() error {
	data, err := os.ReadFile(s.metaPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("读取远程存储列表失败: %w", err)
	}
	var remotes []Remote
	if err := json.Unmarshal(data, &remotes); err != nil {
		return fmt.Errorf("解析远程存储列表失败: %w", err)
	}
	// 迁移旧版本：Options 曾以明文落盘，这里清掉内存副本并立即重写文件。
	// 配置本体在 rclone.conf（密码已混淆），编辑时经 Options() 从那里回读。
	stripped := false
	for i := range remotes {
		if len(remotes[i].Options) > 0 {
			remotes[i].Options = nil
			stripped = true
		}
	}
	// 自愈孤儿条目：元数据存在而 rclone 配置节不存在（配置被外部删改、
	// 历史缺陷遗留），启动即清理，避免编辑窗反复弹"读取驱动器配置失败"。
	// 前提：所有条目在创建时都同步写入 rclone 配置节（无懒创建模式）。
	kept := make([]Remote, 0, len(remotes))
	pruned := []string{}
	for _, r := range remotes {
		exists, err := rclone.Exists(r.Name)
		if err != nil {
			return fmt.Errorf("检查远程存储配置失败: %w", err)
		}
		if exists {
			kept = append(kept, r)
		} else {
			pruned = append(pruned, r.Name)
		}
	}
	remotes = kept
	s.mu.Lock()
	s.remotes = remotes
	s.mu.Unlock()
	if len(pruned) > 0 {
		log.Printf("已清理 %d 个孤儿驱动器条目（rclone 配置中不存在）: %v", len(pruned), pruned)
	}
	if stripped || len(pruned) > 0 {
		return s.save()
	}
	return nil
}

func (s *RemoteService) save() error {
	s.mu.RLock()
	// 凭据不落盘：marshal 前剥离 Options（见 Remote 结构注释）
	stored := make([]Remote, len(s.remotes))
	copy(stored, s.remotes)
	s.mu.RUnlock()
	for i := range stored {
		stored[i].Options = nil
	}
	data, err := json.MarshalIndent(stored, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(s.metaPath), 0o700); err != nil {
		return err
	}
	// 0600：文件里不再有凭据，仍按可能含敏感偏好的配置处理
	if err := os.WriteFile(s.metaPath, data, 0o600); err != nil {
		return err
	}
	// 列表变化后通知观察者（刷新托盘菜单与界面）
	s.notifyChange()
	return nil
}

// List 返回所有远程存储。
func (s *RemoteService) List() []Remote {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Remote, len(s.remotes))
	copy(out, s.remotes)
	return out
}

// Get 按名称查找远程存储。
func (s *RemoteService) Get(name string) (Remote, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, remote := range s.remotes {
		if remote.Name == name {
			return remote, true
		}
	}
	return Remote{}, false
}

// Options 返回某个远程存储的完整配置项（从 rclone.conf 回读，密码字段已解密），
// 供编辑表单回填。凭据不落 remotes.json（见 Remote 结构注释）。
func (s *RemoteService) Options(name string) (map[string]string, error) {
	if _, ok := s.Get(name); !ok {
		return nil, fmt.Errorf("远程存储不存在: %s", name)
	}
	return rclone.ReadRemote(name)
}

// BackendCatalog 返回按用途分组的连接类型目录（rclone 全部后端），供 UI 渲染左侧树。
func (s *RemoteService) BackendCatalog() []rclone.BackendGroup {
	return rclone.Catalog()
}

// BackendFields 返回某个连接类型的配置字段（基本 + 高级），供 UI 渲染表单。
func (s *RemoteService) BackendFields(typ string) ([]rclone.Field, error) {
	return rclone.Fields(typ)
}

// TypeLabels 返回连接类型 → 界面名的映射，供主界面显示协议名。
func (s *RemoteService) TypeLabels() map[string]string {
	labels := make(map[string]string)
	for _, group := range rclone.Catalog() {
		for _, item := range group.Types {
			labels[item.Type] = item.Label
		}
	}
	return labels
}

// Create 校验并新建远程存储，敏感字段写入 INI 前会做 rclone 混淆。
func (s *RemoteService) Create(remote Remote) error {
	path, err := normalizeRemotePath(remote.RemotePath)
	if err != nil {
		return fmt.Errorf("远程路径不合法: %w", err)
	}
	remote.RemotePath = path
	if err := s.validate(remote, true); err != nil {
		return err
	}
	if err := s.writeConfigSection(remote); err != nil {
		return err
	}
	s.mu.Lock()
	s.remotes = append(s.remotes, remote)
	s.mu.Unlock()
	return s.save()
}

// Update 校验并更新远程存储，按 oldName 定位旧条目（改名场景 oldName 与
// remote.Name 不同）。改名时以新名视角校验（不得与其他驱动器重名）；同名
// 编辑沿用既有校验。已连接的驱动器需要先断开连接，避免连接中途改写配置。
func (s *RemoteService) Update(oldName string, remote Remote) error {
	path, err := normalizeRemotePath(remote.RemotePath)
	if err != nil {
		return fmt.Errorf("远程路径不合法: %w", err)
	}
	remote.RemotePath = path
	renaming := remote.Name != oldName
	if err := s.validate(remote, renaming); err != nil {
		return err
	}
	if s.mounts.IsMounted(oldName) {
		return fmt.Errorf("「%s」已连接，请先断开连接再编辑", oldName)
	}
	s.mu.RLock()
	idx := -1
	for i := range s.remotes {
		if s.remotes[i].Name == oldName {
			idx = i
			break
		}
	}
	s.mu.RUnlock()
	if idx < 0 {
		return fmt.Errorf("远程存储不存在: %s", oldName)
	}
	// 名称变化：先把 rclone 配置节迁移到新名（凭据混淆值原样），再写最新选项
	// 与元数据；配置先行，元数据失败时用户可删除重建恢复（见 design D3）
	if remote.Name != oldName {
		if err := rclone.RenameRemote(oldName, remote.Name); err != nil {
			return err
		}
	}
	if err := s.writeConfigSection(remote); err != nil {
		return err
	}
	s.mu.Lock()
	s.remotes[idx] = remote
	s.mu.Unlock()
	return s.save()
}

// Delete 删除远程存储：卸载挂载、移除配置节与本地元数据。
func (s *RemoteService) Delete(name string) error {
	if _, ok := s.Get(name); !ok {
		return fmt.Errorf("远程存储不存在: %s", name)
	}
	if s.mounts.IsMounted(name) {
		if err := s.mounts.Unmount(name); err != nil {
			return err
		}
	}
	if err := rclone.DeleteRemote(name); err != nil {
		return err
	}
	s.mu.Lock()
	for i := range s.remotes {
		if s.remotes[i].Name == name {
			s.remotes = append(s.remotes[:i], s.remotes[i+1:]...)
			break
		}
	}
	s.mu.Unlock()
	return s.save()
}

// Test 在进程内建立连接并列举根目录，验证远程存储可用。
func (s *RemoteService) Test(name string) error {
	remote, ok := s.Get(name)
	if !ok {
		return fmt.Errorf("远程存储不存在: %s", name)
	}
	return rclone.TestRemote(name, remote.Insecure)
}

func (s *RemoteService) validate(remote Remote, creating bool) error {
	name := strings.TrimSpace(remote.Name)
	if name == "" {
		return errors.New("名称不能为空")
	}
	if !remoteNamePattern.MatchString(name) {
		return fmt.Errorf("名称不能包含空白字符: %q", remote.Name)
	}
	fields, err := rclone.Fields(remote.Type)
	if err != nil {
		return fmt.Errorf("不支持的协议类型: %s", remote.Type)
	}
	_, exists := s.Get(name)
	if creating && exists {
		return fmt.Errorf("远程存储已存在: %s", name)
	}
	if !creating && !exists {
		return fmt.Errorf("远程存储不存在: %s", name)
	}
	for _, field := range fields {
		value := strings.TrimSpace(remote.Options[field.Key])
		if field.Required && value == "" {
			return fmt.Errorf("缺少必填字段: %s", field.Label)
		}
	}
	return nil
}

// normalizeRemotePath 规范化远程子路径（如 SMB 共享名）：去首尾 "/" 与空白。
// 空值合法（挂远程根）；含冒号、空白、空段或 ".." 视为非法——子路径会拼进
// 挂载目标 name:path，这些字符要么破坏路径语法要么可能逃出预期目录。
func normalizeRemotePath(p string) (string, error) {
	p = strings.Trim(strings.TrimSpace(p), "/")
	if p == "" {
		return "", nil
	}
	if strings.ContainsAny(p, ": \t") {
		return "", fmt.Errorf("不能包含冒号或空白字符: %q", p)
	}
	for _, seg := range strings.Split(p, "/") {
		if seg == "" {
			return "", fmt.Errorf("存在空路径段: %q", p)
		}
		if seg == ".." {
			return "", fmt.Errorf("不能包含 \"..\": %q", p)
		}
	}
	return p, nil
}

// writeConfigSection 将远程存储的选项写入 rclone.conf，密码类字段由 rclone 自动混淆。
func (s *RemoteService) writeConfigSection(remote Remote) error {
	if _, err := rclone.Fields(remote.Type); err != nil {
		return fmt.Errorf("不支持的协议类型: %s", remote.Type)
	}
	return rclone.SaveRemote(remote.Name, remote.Type, remote.Options)
}
