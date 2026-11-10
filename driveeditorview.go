package main

import (
	"fmt"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"

	"rclonedrive/internal/rclone"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

// driveEditorState 是驱动器编辑窗口的视图状态（对应旧前端 DriveWindow.vue）：
// 左侧连接类型树（分组/搜索）+ 右侧动态表单（基本/高级/说明）+ 底部操作。
// 窗口在新增/编辑间复用，setTarget 切换目标后由视图重新加载。
type driveEditorState struct {
	win *mygo.Window // 创建后赋值，用于在后台 goroutine 里把状态变更带回主线程

	mu         sync.Mutex // 保护 goroutine 完成时要写的瞬态标记
	submitting bool
	testing    bool

	editName  string // "" = 新增
	loadedFor string // 已完成加载的目标（避免每帧重复加载）

	catalog    []rclone.BackendGroup
	openGroups map[string]bool
	search     string

	selectedType string
	fields       []rclone.Field
	fieldsCache  map[string][]rclone.Field

	// 表单（旧 DriveWindow.vue 的 ref 组）
	name        string
	mountPoint  string
	autoConnect bool
	readOnly    bool
	insecure    bool
	diskTotal   string     // 手动配置的显示容量（如 "2TiB"），空 = 自动探测
	remotePath  string     // 远程子路径（SMB 场景为共享名），空 = 挂远程根
	autoOpen    bool       // 连接成功后自动打开挂载文件夹
	options     map[string]string
	invalid     map[string]bool
	tab         int

	defaultMountPoint string
	usedLetters       []string
	remotesList       []Remote

	// webdav 组合行：url 拆成 协议/地址/端口（组合值仍写入 options.url）
	webdavProtocol string
	webdavHost     string
	webdavPort     string

	deleteConfirm bool
}

func newDriveEditorState() *driveEditorState {
	return &driveEditorState{
		options:     make(map[string]string),
		invalid:     make(map[string]bool),
		openGroups:  make(map[string]bool),
		fieldsCache: make(map[string][]rclone.Field),
	}
}

// setTarget 切换窗口目标（新增传 "；编辑传驱动器名称），复用时由
// windowManager 调用；视图检测到目标变化后重新加载（对应旧 drive:open 事件）。
func (s *driveEditorState) setTarget(name string) {
	s.editName = name
}

func (s *driveEditorState) editing() bool { return s.editName != "" }

// onMain 把状态变更带回主线程（Window.Update 会随后重绘）。
// 测试里没有窗口，直接执行（Tester 单线程）。
func (s *driveEditorState) onMain(fn func()) {
	if s.win != nil {
		s.win.Update(fn)
		return
	}
	fn()
}

func (s *driveEditorState) view(c *ui.Context) {
	t := applyAppearance(c)
	s.ensureLoaded()

	title, subtitle := "新虚拟驱动器", "选择连接类型并填写连接信息"
	if s.editing() {
		title, subtitle = "编辑驱动器", "修改远程存储的连接信息"
	}

	ui.Column(c).Fill().Background(t.Background).Children(func() {
		windowBar(c, t, title, subtitle, nil, func() {
			if allWindows != nil {
				allWindows.hide(driveWindowName)
			}
		})

		// 注意 ui.Row 默认垂直居中子元素，必须靠顶对齐，
		// 否则右侧表单高度不足时会被居到窗口中部。
		ui.Row(c).FillWidth().Grow(1).AlignItems(ui.Start).Children(func() {
			s.typeSidebar(c, t)
			s.editorPanel(c, t)
		})

		s.footer(c, t)
	})

	// 删除确认（编辑模式）
	if s.editing() && s.deleteConfirm {
		if ui.AlertDialog(c, &s.deleteConfirm, "删除驱动器",
			fmt.Sprintf("确定要删除「%s」吗？相关的远程存储配置将被移除。", s.editName),
			"取消", "删除") == 1 {
			s.onDelete()
		}
	}

	drainToasts(c, driveWindowName)
}

// ensureLoaded 目标变化后同步加载（catalog/字段元数据是内存查询，
// 盘符列表是系统调用，都不慢；不放在 goroutine 里以便测试确定性）。
func (s *driveEditorState) ensureLoaded() {
	key := "<new>"
	if s.editing() {
		key = "edit:" + s.editName
	}
	if s.loadedFor == key {
		return
	}
	s.loadedFor = key
	s.resetTransient()

	if s.catalog == nil {
		s.catalog = remoteService.BackendCatalog()
		// 分组默认全部收起，只展开「常用协议」（分组顺序里的第一个）
		for i, group := range s.catalog {
			s.openGroups[group.Label] = i == 0
		}
	}
	s.defaultMountPoint = systemService.DefaultMountPoint()
	s.usedLetters = systemService.UsedDriveLetters()
	s.remotesList = remoteService.List()

	if s.editing() {
		remote, ok := remoteService.Get(s.editName)
		if !ok {
			notifyUserTo(driveWindowName, "error", "打开驱动器失败", "驱动器不存在："+s.editName)
			if allWindows != nil {
				allWindows.hide(driveWindowName)
			}
			return
		}
		s.selectedType = remote.Type
		s.loadFields(remote.Type)
		s.fillFrom(remote)
		return
	}

	if s.selectedType == "" {
		if all := s.allTypes(); len(all) > 0 {
			s.selectedType = all[0].Type
		}
	}
	if s.selectedType != "" {
		s.loadFields(s.selectedType)
	}
	s.resetForm()
}

// resetTransient 复位复用窗口遗留的进行中状态（对应旧 resetTransientState）。
func (s *driveEditorState) resetTransient() {
	s.mu.Lock()
	s.submitting, s.testing = false, false
	s.mu.Unlock()
	s.invalid = make(map[string]bool)
	s.tab = 0
	s.search = ""
	s.deleteConfirm = false
	if s.catalog != nil {
		for i, group := range s.catalog {
			s.openGroups[group.Label] = i == 0
		}
	}
}

func (s *driveEditorState) allTypes() []rclone.BackendType {
	out := make([]rclone.BackendType, 0)
	for _, group := range s.catalog {
		out = append(out, group.Types...)
	}
	return out
}

func (s *driveEditorState) currentType() *rclone.BackendType {
	for i := range s.allTypes() {
		if s.allTypes()[i].Type == s.selectedType {
			return &s.allTypes()[i]
		}
	}
	return nil
}

func (s *driveEditorState) loadFields(typ string) {
	if cached, ok := s.fieldsCache[typ]; ok {
		s.fields = cached
		return
	}
	fields, err := remoteService.BackendFields(typ)
	if err != nil {
		notifyUserTo(driveWindowName, "error", "加载连接类型字段失败", err.Error())
		s.fields = nil
		return
	}
	s.fieldsCache[typ] = fields
	s.fields = fields
}

// selectType 切换连接类型：新增模式重置表单，编辑模式不允许切换（旧语义）。
func (s *driveEditorState) selectType(typ string) {
	if s.editing() || typ == s.selectedType {
		return
	}
	s.selectedType = typ
	s.loadFields(typ)
	s.resetForm()
}

// seedOptions 用 rclone 字段元数据的默认值填充表单（保留已有值可选）。
func (s *driveEditorState) seedOptions(keepExisting bool) {
	opts := make(map[string]string)
	for _, field := range s.fields {
		if keepExisting {
			if v, ok := s.options[field.Key]; ok {
				opts[field.Key] = v
				continue
			}
		}
		opts[field.Key] = field.Default
	}
	s.options = opts
}

// resetForm 清空表单并落到可用默认值（对应旧 resetForm）。
func (s *driveEditorState) resetForm() {
	s.name = ""
	s.mountPoint = s.defaultMountPoint
	s.autoConnect, s.readOnly, s.insecure, s.autoOpen = false, false, false, false
	s.diskTotal = ""
	s.remotePath = ""
	s.invalid = make(map[string]bool)
	s.seedOptions(false)
	// 默认盘符可能被其他驱动器占用，落到第一个可用项
	letters := s.driveLetters()
	if !s.editing() && len(letters) > 0 && !containsString(letters, s.mountPoint) {
		s.mountPoint = letters[0]
	}
	if s.selectedType == "webdav" {
		s.splitWebdavUrl(s.options["url"])
	}
}

// fillFrom 用已有远程存储回填表单（编辑模式）。
func (s *driveEditorState) fillFrom(remote Remote) {
	s.name = remote.Name
	s.mountPoint = remote.MountPoint
	if s.mountPoint == "" {
		s.mountPoint = s.defaultMountPoint
	}
	s.autoConnect = remote.AutoConnect
	s.readOnly = remote.ReadOnly
	s.insecure = remote.Insecure
	s.diskTotal = remote.DiskTotal
	s.remotePath = remote.RemotePath
	s.autoOpen = remote.AutoOpen
	s.invalid = make(map[string]bool)
	s.seedOptions(false)
	// 凭据不落 remotes.json：配置项从 rclone.conf 回读（密码字段后端已解密）
	stored, err := remoteService.Options(remote.Name)
	if err != nil {
		notifyUserTo(driveWindowName, "error", "读取驱动器配置失败", err.Error())
		// 元数据存在但配置节不在（孤儿条目等）：提示后关窗回列表，
		// 不停留在一个无法回填的半初始化表单
		if allWindows != nil {
			allWindows.hide(driveWindowName)
		}
		return
	}
	for key, value := range stored {
		s.options[key] = value
	}
	if s.selectedType == "webdav" {
		s.splitWebdavUrl(s.options["url"])
	}
}

// --- 左侧：连接类型树 ---

func (s *driveEditorState) typeSidebar(c *ui.Context, t *ui.Theme) {
	ui.Column(c).Width(200).FillHeight().BorderWidth(0, 0, 1, 0).BorderColor(t.Border).Children(func() {
		ui.Row(c).FillWidth().Padding(8).Children(func() {
			ui.TextInput(c, &s.search).Label("搜索连接类型").Placeholder("搜索连接类型").FillWidth().
				Disabled(s.editing())
		})

		ui.Scroll(c).FillWidth().Grow(1).Children(func() {
			ui.Column(c).FillWidth().Gap(1).Padding(6).Children(func() {
				keyword := strings.TrimSpace(s.search)
				if keyword != "" {
					s.searchResults(c, t, strings.ToLower(keyword))
					return
				}
				for _, group := range s.catalog {
					group := group
					open := s.openGroups[group.Label]
					ui.Collapsible(c, fmt.Sprintf("%s  %d", group.Label, len(group.Types)), &open, func() {
						for _, item := range group.Types {
							s.typeItem(c, t, item)
						}
					})
					s.openGroups[group.Label] = open
				}
			})
		})
	})
}

func (s *driveEditorState) searchResults(c *ui.Context, t *ui.Theme, keyword string) {
	count := 0
	for _, item := range s.allTypes() {
		if !strings.Contains(strings.ToLower(item.Label), keyword) && !strings.Contains(strings.ToLower(item.Type), keyword) {
			continue
		}
		count++
		s.typeItem(c, t, item)
	}
	if count == 0 {
		ui.Text(c, "没有匹配的连接类型").FontSize(t.FontSize - 2).TextColor(t.TextMuted).Padding(4)
	}
}

func (s *driveEditorState) typeItem(c *ui.Context, t *ui.Theme, item rclone.BackendType) {
	// ButtonBase 默认把内容居中（coreButtonBase 的 Center()），树项要左对齐。
	e := ui.ButtonBase(c).Label(item.Label).Padding(5, 8).Radius(t.Radius).FillWidth().Justify(ui.Start)
	if item.Type == s.selectedType {
		e.Background(t.Surface)
	}
	e.Disabled(s.editing())
	e.Children(func() {
		ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
			ui.Icon(c, protocolIcon(item.Type)).Size(13, 13).TextColor(t.TextMuted)
			label := ui.Text(c, item.Label).FontSize(t.FontSize - 1).MaxLines(1).Shrink(1)
			if item.Type == s.selectedType {
				label.Bold()
			}
			if item.Authorized {
				ui.Badge(c, "需授权")
			}
		})
	})
	if e.Clicked() {
		s.selectType(item.Type)
	}
}

// --- 右侧：表单面板 ---

func (s *driveEditorState) editorPanel(c *ui.Context, t *ui.Theme) {
	// FillHeight 是必须的：主布局 Row 靠顶对齐后子元素不再被纵向拉伸，
	// 不显式填满行高时 Scroll 拿不到有界高度，内容超长会溢出窗口而非滚动。
	ui.Column(c).FillWidth().FillHeight().Grow(1).Children(func() {
		// 当前类型标题行
		ui.Row(c).FillWidth().Gap(8).Padding(10, 12).AlignItems(ui.Center).
			BorderWidth(0, 0, 1, 0).BorderColor(t.Border).Children(func() {
			ui.Icon(c, protocolIcon(s.selectedType)).Size(14, 14).TextColor(t.TextMuted)
			current := s.currentType()
			label := s.selectedType
			if current != nil {
				label = current.Label
			}
			ui.Text(c, label).Bold().MaxLines(1).Shrink(1)
			if current != nil && current.Authorized {
				ui.Badge(c, "需授权")
			}
		})

		// 标签页：基本 / 高级·N / 说明
		basic, advanced := s.visibleFields()
		labels := []string{"基本", fmt.Sprintf("高级·%d", len(advanced)), "说明"}
		ui.Row(c).Padding(10, 12, 0, 12).Children(func() {
			ui.Tabs(c, &s.tab, labels...)
		})

		ui.Scroll(c).FillWidth().Grow(1).Children(func() {
			ui.Column(c).FillWidth().Gap(12).Padding(12).Children(func() {
				switch s.tab {
				case 1:
					s.advancedTab(c, t, advanced)
				case 2:
					s.aboutTab(c, t, s.currentType())
				default:
					s.basicTab(c, t, basic)
				}
			})
		})
	})
}

// compositeKeys 由组合行单独渲染的字段（对应旧 compositeKeys）。
func (s *driveEditorState) compositeKeys() []string {
	switch s.selectedType {
	case "webdav":
		return []string{"url", "vendor", "user", "pass"}
	case "sftp", "ftp":
		return []string{"host", "port", "user", "pass"}
	default:
		return nil
	}
}

// visibleFields 过滤掉组合行字段并按 provider 过滤，拆成基本/高级两组。
func (s *driveEditorState) visibleFields() (basic, advanced []rclone.Field) {
	composite := s.compositeKeys()
	provider := strings.ToLower(strings.TrimSpace(s.options["provider"]))
	for _, field := range s.fields {
		if containsString(composite, field.Key) {
			continue
		}
		if field.Provider != "" && !providerMatches(field.Provider, provider) {
			continue
		}
		if field.Advanced {
			advanced = append(advanced, field)
		} else {
			basic = append(basic, field)
		}
	}
	return basic, advanced
}

// providerMatches 选项带 Provider 过滤串时（如 s3 的各厂商专属选项），
// 只展示与当前 provider 匹配的项（对应旧 providerMatches）。
func providerMatches(filter, provider string) bool {
	rules := strings.Split(filter, ",")
	var positive []string
	for _, rule := range rules {
		rule = strings.ToLower(strings.TrimSpace(rule))
		if rule == "" {
			continue
		}
		if strings.HasPrefix(rule, "!") {
			if rule[1:] == provider {
				return false
			}
			continue
		}
		positive = append(positive, rule)
	}
	return len(positive) == 0 || containsString(positive, provider)
}

func (s *driveEditorState) findField(key string) *rclone.Field {
	for i := range s.fields {
		if s.fields[i].Key == key {
			return &s.fields[i]
		}
	}
	return nil
}

// basicTab 基本页：名称、盘符/服务商、开关组、webdav/sftp/ftp 组合行、其余字段。
func (s *driveEditorState) basicTab(c *ui.Context, t *ui.Theme, basic []rclone.Field) {
	ui.Column(c).FillWidth().Gap(12).Children(func() {
		// 名称
		s.nameField(c, t)

		// 盘符（Windows）/ 挂载目录（其他平台）
		ui.Row(c).Gap(8).AlignItems(ui.Start).Children(func() {
			mountCol := ui.Column(c).Gap(4).Width(90)
			if runtime.GOOS != "windows" {
				// 非 Windows 填的是目录路径，给整行宽度
				mountCol = ui.Column(c).Gap(4).Grow(1)
			}
			mountCol.Children(func() {
				label := "盘符"
				if runtime.GOOS != "windows" {
					label = "挂载目录"
				}
				ui.Text(c, label).FontSize(t.FontSize - 1).TextColor(t.TextMuted)
				letters := s.driveLetters()
				if len(letters) > 0 {
					// Select 自带 140px 最小宽度（t.Space(35)），会溢出
					// 本列并与右侧下拉重叠，这里清零后填满 90px 列宽。
					// 标签展示去掉冒号（Z: → Z），写回的值保留冒号（rclone 需要）。
					labels := make([]string, len(letters))
					for i, l := range letters {
						labels[i] = strings.TrimSuffix(l, ":")
					}
					selectLabeled(c, t, &s.mountPoint, labels, letters, "")
				} else {
					// 非 Windows：挂载目录由应用按驱动器名分配（mounts/<名称>），
					// 只读展示生效路径，不允许编辑（盘符是 Windows 的概念）
					ui.Row(c).Gap(8).AlignItems(ui.Center).FillWidth().
						Padding(6, 8).Radius(t.Radius).Background(t.Surface).Border(1, t.Border).
						Children(func() {
							ui.Text(c, s.effectiveMountDir()).FontSize(t.FontSize - 1).
								TextColor(t.TextMuted).MaxLines(1)
						})
				}
			})
			if s.selectedType == "webdav" {
				if field := s.findField("vendor"); field != nil && len(field.Examples) > 0 {
					ui.Column(c).Gap(4).Grow(1).Children(func() {
						ui.Text(c, field.Label).FontSize(t.FontSize - 1).TextColor(t.TextMuted).
							Tooltip(field.Help)
						value := s.options["vendor"]
						labels, values := exampleOptions(field)
						selectLabeled(c, t, &value, labels, values, "选择服务商")
						s.options["vendor"] = value
					})
				}
			}
		})

		// 显示容量：后端报不出容量时（如 WebDAV 无 quota），资源管理器会显示
		// rclone 的 1 PiB 占位值（"1.00 PB 可用"），这里手动指定一个合理总量。
		// 布局与表单其他行一致：label 在上、控件在下（水平 label 列会留大空隙）
		ui.Column(c).Gap(4).FillWidth().Children(func() {
			ui.Text(c, "显示容量").FontSize(t.FontSize - 1).TextColor(t.TextMuted).
				Tooltip("驱动器属性中显示的总容量。选\"自动\"时探测后端真实容量；后端不报告容量（如部分 WebDAV 服务器）会显示 1 PB 占位值，此时可手动指定")
			ui.Column(c).Width(160).Children(func() {
				selectLabeled(c, t, &s.diskTotal,
					[]string{"自动", "512 GB", "1 TB", "2 TB", "4 TB", "8 TB", "16 TB"},
					[]string{"", "512GiB", "1TiB", "2TiB", "4TiB", "8TiB", "16TiB"}, "")
			})
		})

		// 共享路径 / 远程路径：后端没有 root 配置项时（如 smb）才显示——有
		// root 项的后端用既有高级字段即可。填写后挂载点直接是该路径内容
		// 且容量为真实值；路径支持共享内子目录（name:share/sub 语法）
		if s.findField("root") == nil {
			label := "远程路径"
			placeholder := "可选，如 folder/sub"
			tooltip := "留空挂载远程根目录；填写远程子路径后，驱动器直接是该子目录的内容，容量显示为真实值。"
			if s.selectedType == "smb" {
				label = "共享路径"
				placeholder = "可选，如 backup 或 backup/照片"
				tooltip = "填写 NAS 上的共享或共享内路径（如 backup 或 backup/照片）后，驱动器直接是该路径的内容，容量显示为真实值；留空则显示全部共享文件夹，容量为虚拟占位值。"
			}
			ui.Column(c).Gap(4).FillWidth().Children(func() {
				ui.Text(c, label).FontSize(t.FontSize - 1).TextColor(t.TextMuted).Tooltip(tooltip)
				ui.TextInput(c, &s.remotePath).Label(label).Placeholder(placeholder).FillWidth().Tooltip(tooltip)
			})
		}

		// 开关组：自动连接 / 只读模式 / 忽略证书错误 / 连接时自动打开
		ui.Row(c).Gap(16).AlignItems(ui.Center).Wrap().Children(func() {
			ui.Checkbox(c, &s.autoConnect, "自动连接")
			ui.Checkbox(c, &s.readOnly, "只读模式")
			ui.Checkbox(c, &s.insecure, "忽略证书错误").Tooltip("自签名证书的 HTTPS 服务器无法校验证书时勾选。注意：开启后该驱动器的流量不再受 TLS 保护，存在被窃听、篡改的风险")
			ui.Checkbox(c, &s.autoOpen, "连接时自动打开").Tooltip("手动连接成功后自动打开挂载的文件夹；开机自动连接不触发")
		})

		if s.insecure {
			ui.Row(c).FillWidth().Gap(8).Padding(8, 10).Radius(t.Radius).
				Border(1, t.Warning).Children(func() {
				ui.Icon(c, iconWarning).Size(13, 13).TextColor(t.Warning)
				ui.Text(c, "安全警告：启用后该驱动器的 HTTPS 流量将不再校验证书，同一网络下的攻击者可能窃听或篡改传输内容（包括账号密码）。仅建议对使用自签名证书的自建服务器开启。").
					FontSize(t.FontSize - 2).TextColor(t.Warning)
			})
		}

		ui.Text(c, "连接设置").FontSize(t.FontSize - 1).TextColor(t.TextMuted)

		s.compositeRows(c, t)

		// 其余基本字段（连续的 bool 并排一行，对应旧 segmentFields）
		for _, seg := range segmentFields(basic) {
			if len(seg.bools) > 0 {
				ui.Row(c).Gap(16).AlignItems(ui.Center).Wrap().Children(func() {
					for _, f := range seg.bools {
						s.driveField(c, t, f)
					}
				})
			} else {
				s.driveField(c, t, seg.field)
			}
		}
	})
}

// nameField 名称输入：编辑模式同样可改（保存时按旧名定位，改名会同步
// 迁移 rclone 配置节与本地记录，见 RemoteService.Update）。
func (s *driveEditorState) nameField(c *ui.Context, t *ui.Theme) {
	ui.Column(c).Gap(4).Children(func() {
		label := ui.Text(c, "名称").FontSize(t.FontSize - 1).TextColor(t.TextMuted)
		if s.editing() {
			label.Tooltip("修改名称将同步更新连接配置与本地记录。")
		}
		if s.invalid["__name"] {
			label.TextColor(t.Danger)
		}
		ui.TextInput(c, &s.name).Label("名称").Placeholder("我的网盘").FillWidth()
		if s.invalid["__name"] {
			ui.Text(c, "请输入驱动器名称。").FontSize(t.FontSize - 2).TextColor(t.Danger)
		}
	})
}

// compositeRows 渲染 webdav / sftp / ftp 的组合行（对应旧模板中的三段组合）。
func (s *driveEditorState) compositeRows(c *ui.Context, t *ui.Theme) {
	switch s.selectedType {
	case "webdav":
		// 地址 = 协议 / 主机 / 端口
		ui.Column(c).Gap(4).Children(func() {
			label := ui.Text(c, "地址").FontSize(t.FontSize - 1).TextColor(t.TextMuted)
			if help := s.findField("url"); help != nil && help.Help != "" {
				label.Tooltip(help.Help)
			}
			if s.invalid["url"] {
				label.TextColor(t.Danger)
			}
			ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
				protocol := s.webdavProtocol
				ui.Select(c, &protocol, []string{"http", "https"}).MinWidth(0).Width(90)
				if protocol != s.webdavProtocol {
					s.onWebdavProtocol(protocol)
				}
				host := s.webdavHost
				hostInput := ui.TextInput(c, &host).Placeholder("IP / 域名").Label("地址").Grow(1)
				if hostInput.Changed() {
					s.webdavHost = host
				}
				port := s.webdavPort
				portInput := ui.TextInput(c, &port).Placeholder("端口").Label("端口").Width(70)
				if portInput.Changed() {
					s.webdavPort = port
				}
				s.options["url"] = s.joinWebdavUrl()
			})
			if s.invalid["url"] {
				ui.Text(c, "请填写完整的地址信息。").FontSize(t.FontSize - 2).TextColor(t.Danger)
			}
		})
		s.userPassRow(c, t)
	case "sftp", "ftp":
		host, port := s.findField("host"), s.findField("port")
		if host != nil && port != nil {
			ui.Row(c).Gap(8).AlignItems(ui.Start).Children(func() {
				ui.Column(c).Gap(4).Grow(1).Children(func() { s.driveField(c, t, *host) })
				ui.Column(c).Gap(4).Width(90).Children(func() { s.driveField(c, t, *port) })
			})
		}
		s.userPassRow(c, t)
	}
}

// userPassRow 用户名 + 密码并排（webdav/sftp/ftp）。
func (s *driveEditorState) userPassRow(c *ui.Context, t *ui.Theme) {
	user, pass := s.findField("user"), s.findField("pass")
	if user == nil || pass == nil {
		return
	}
	ui.Row(c).Gap(8).AlignItems(ui.Start).Children(func() {
		ui.Column(c).Gap(4).Grow(1).Children(func() { s.driveField(c, t, *user) })
		ui.Column(c).Gap(4).Grow(1).Children(func() { s.driveField(c, t, *pass) })
	})
}

// driveField 按 rclone 字段元数据渲染一个配置项（对应旧 DriveField.vue）：
// bool → 复选框，枚举 → 下拉（中文标签），密码 → 密码框，其余 → 文本框。
func (s *driveEditorState) driveField(c *ui.Context, t *ui.Theme, field rclone.Field) {
	invalid := s.invalid[field.Key]
	value := s.options[field.Key]

	if field.Type == "bool" {
		b := value == "true"
		cb := ui.Checkbox(c, &b, field.Label)
		if field.Help != "" && !invalid {
			cb.Tooltip(field.Help)
		}
		if cb.Changed() {
			s.options[field.Key] = strconv.FormatBool(b)
		}
		return
	}

	ui.Column(c).Gap(4).FillWidth().Children(func() {
		ui.Row(c).Gap(2).AlignItems(ui.Center).Children(func() {
			label := ui.Text(c, field.Label).FontSize(t.FontSize - 1).TextColor(t.TextMuted)
			if field.Help != "" && !invalid {
				label.Tooltip(field.Help)
			}
			if invalid {
				label.TextColor(t.Danger)
			}
			if field.Required {
				ui.Text(c, "*").FontSize(t.FontSize - 1).TextColor(t.Danger)
			}
		})

		if len(field.Examples) > 0 {
			labels, values := exampleOptions(&field)
			selectLabeled(c, t, &value, labels, values, "请选择"+field.Label)
			s.options[field.Key] = value
		} else {
			input := ui.TextInput(c, &value).FillWidth().Label(field.Label)
			if field.Password {
				input.Password()
			}
			if input.Changed() {
				s.options[field.Key] = value
			}
		}
		if invalid {
			ui.Text(c, "此字段为必填项。").FontSize(t.FontSize - 2).TextColor(t.Danger)
		}
	})
}

// advancedTab 高级页（对应旧 advanced TabsContent）。
func (s *driveEditorState) advancedTab(c *ui.Context, t *ui.Theme, advanced []rclone.Field) {
	if len(advanced) == 0 {
		ui.Text(c, "该连接类型没有高级选项。").FontSize(t.FontSize - 2).TextColor(t.TextMuted)
		return
	}
	ui.Column(c).FillWidth().Gap(12).Children(func() {
		for _, seg := range segmentFields(advanced) {
			if len(seg.bools) > 0 {
				ui.Row(c).Gap(16).AlignItems(ui.Center).Wrap().Children(func() {
					for _, f := range seg.bools {
						s.driveField(c, t, f)
					}
				})
			} else {
				s.driveField(c, t, seg.field)
			}
		}
	})
}

// aboutTab 说明页：帮助文本 + rclone.org 文档链接 + 授权说明。
func (s *driveEditorState) aboutTab(c *ui.Context, t *ui.Theme, current *rclone.BackendType) {
	ui.Column(c).FillWidth().Gap(10).Children(func() {
		if current != nil {
			ui.Text(c, strings.TrimSpace(current.Help)).FontSize(t.FontSize - 1).TextColor(t.TextMuted)
		}
		ui.Text(c, "文档").FontSize(t.FontSize - 1).TextColor(t.TextMuted)
		slug := strings.ReplaceAll(s.selectedType, " ", "")
		ui.Link(c, "rclone.org/"+slug+"/", "https://rclone.org/"+slug+"/")
		if current != nil && current.Authorized {
			ui.Text(c, "需要授权").FontSize(t.FontSize - 1).TextColor(t.TextMuted)
			ui.Text(c, "该连接类型需要先完成 OAuth 授权，本界面不提供浏览器授权流程。可以先在其它机器上运行 rclone authorize \""+s.selectedType+"\" 得到 token，再把它粘贴到「高级」里的「授权令牌（token）」；已有的 rclone 配置也可以直接复用。").
				FontSize(t.FontSize - 2).TextColor(t.TextMuted)
		}
	})
}

// --- 底部操作 ---

func (s *driveEditorState) footer(c *ui.Context, t *ui.Theme) {
	s.mu.Lock()
	submitting, testing := s.submitting, s.testing
	s.mu.Unlock()

	ui.Row(c).FillWidth().Gap(8).Padding(10, 12).Justify(ui.End).AlignItems(ui.Center).
		BorderWidth(1, 0, 0, 0).BorderColor(t.Border).Children(func() {
		if ui.Button(c, "测试连接").Disabled(submitting || testing).Clicked() {
			s.onTest()
		}
		if ui.Button(c, "取消").Disabled(submitting || testing).Clicked() {
			if allWindows != nil {
				allWindows.hide(driveWindowName)
			}
		}
		if s.editing() && ui.Button(c, "删除").Disabled(submitting || testing).Clicked() {
			s.deleteConfirm = true
		}
		if ui.PrimaryButton(c, "保存").Disabled(submitting || testing).Clicked() {
			s.onSave(false)
		}
		if !s.editing() && ui.Button(c, "保存并连接").Disabled(submitting || testing).Clicked() {
			s.onSave(true)
		}
	})
}

// --- 表单动作 ---

func (s *driveEditorState) isBusy() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.submitting || s.testing
}

func (s *driveEditorState) setBusy(testing, submitting bool) {
	s.onMain(func() {
		s.mu.Lock()
		s.testing, s.submitting = testing, submitting
		s.mu.Unlock()
	})
}

// validate 校验名称与必填字段；失败时切到基本页（对应旧 validate）。
func (s *driveEditorState) validate() bool {
	errs := make(map[string]bool)
	if strings.TrimSpace(s.name) == "" {
		errs["__name"] = true
	}
	for _, field := range s.fields {
		if field.Required && strings.TrimSpace(s.options[field.Key]) == "" {
			errs[field.Key] = true
		}
	}
	s.invalid = errs
	if len(errs) > 0 {
		s.tab = 0
		return false
	}
	return true
}

// buildRemote 组装保存请求（对应旧 buildRemote；空值选项不落配置）。
func (s *driveEditorState) buildRemote() Remote {
	opts := make(map[string]string)
	for key, value := range s.options {
		if value != "" {
			opts[key] = value
		}
	}
	mountPoint := s.mountPoint
	if runtime.GOOS != "windows" && mountPoint == s.defaultMountPoint {
		// 用户没改默认挂载目录：留空存储，连接时按驱动器名分配
		// mounts/<名称> 子目录，避免多个驱动器挤在 mounts 根目录冲突
		mountPoint = ""
	}
	return Remote{
		Name:        strings.TrimSpace(s.name),
		Type:        s.selectedType,
		MountPoint:  mountPoint,
		AutoConnect: s.autoConnect,
		ReadOnly:    s.readOnly,
		Insecure:    s.insecure,
		DiskTotal:   s.diskTotal,
		RemotePath:  strings.Trim(strings.TrimSpace(s.remotePath), "/"),
		AutoOpen:    s.autoOpen,
		Options:     opts,
	}
}

func (s *driveEditorState) persist() error {
	if s.editing() {
		return remoteService.Update(s.editName, s.buildRemote())
	}
	return remoteService.Create(s.buildRemote())
}

// onTest 先写入配置再测试连接（对应旧 onTest）。
func (s *driveEditorState) onTest() {
	if s.isBusy() {
		return
	}
	if !s.validate() {
		notifyUserTo(driveWindowName, "error", "无法测试连接", "请填写所有必填项")
		return
	}
	s.setBusy(true, false)
	go func() {
		defer s.setBusy(false, false)
		if err := s.persist(); err != nil {
			notifyUserTo(driveWindowName, "error", "连接测试失败", err.Error())
			return
		}
		name := strings.TrimSpace(s.name)
		if err := remoteService.Test(name); err != nil {
			notifyUserTo(driveWindowName, "error", "连接测试失败", err.Error())
			return
		}
		notifyUserTo(driveWindowName, "success", "连接成功", "「"+name+"」连接测试通过")
	}()
}

// onSave 保存配置；connect 为 true 时保存后立即挂载（新增模式）。
// 成功后隐藏窗口（子窗口只隐藏不销毁）；配置已写入但挂载失败时也照常
// 关闭窗口，从主界面重试连接（对应旧注释）。
func (s *driveEditorState) onSave(connect bool) {
	if s.isBusy() {
		return
	}
	if !s.validate() {
		notifyUserTo(driveWindowName, "error", "无法保存", "请填写所有必填项")
		return
	}
	s.setBusy(false, true)
	go func() {
		err := s.persist()
		s.setBusy(false, false)
		if err != nil {
			title := "创建失败"
			if s.editing() {
				title = "保存失败"
			}
			notifyUserTo(driveWindowName, "error", title, err.Error())
			return
		}
		name := strings.TrimSpace(s.name)
		if connect {
			if err := mountService.Mount(name); err != nil {
				notifyUserTo(driveWindowName, "error", "已保存，但连接失败", err.Error())
			} else {
				notifyUserTo(driveWindowName, "success", "已连接", "「"+name+"」已挂载")
			}
		}
		s.onMain(func() {
			if allWindows != nil {
				allWindows.hide(driveWindowName)
			}
		})
	}()
}

// onDelete 删除驱动器（编辑模式的删除按钮，确认对话框已过）。
func (s *driveEditorState) onDelete() {
	s.setBusy(false, true)
	go func() {
		err := remoteService.Delete(s.editName)
		s.setBusy(false, false)
		if err != nil {
			notifyUserTo(driveWindowName, "error", "删除失败", err.Error())
			return
		}
		s.onMain(func() {
			if allWindows != nil {
				allWindows.hide(driveWindowName)
			}
		})
	}()
}

// --- 盘符与 webdav 组合行的辅助（对应旧 computed/工具函数） ---

// effectiveMountDir 返回非 Windows 平台展示给用户看的生效挂载目录：
// 自定义过（不等于默认根目录）就显示自定义值；否则按当前名称预演默认
// 子目录，名称为空时显示占位提示。
func (s *driveEditorState) effectiveMountDir() string {
	if s.mountPoint != "" && s.mountPoint != s.defaultMountPoint {
		return s.mountPoint
	}
	if name := mountNameDir(strings.TrimSpace(s.name)); name != "drive" {
		return defaultMountTarget(name)
	}
	return filepath.Join(nextFreeMountPoint(), "<驱动器名>")
}

// driveLetters 从 Z 往下排除系统已占用、以及分配给其他驱动器的盘符。
// 非 Windows 平台没有盘符概念，返回空——挂载目录是文本输入，不走下拉。
func (s *driveEditorState) driveLetters() []string {
	if runtime.GOOS != "windows" {
		return nil
	}
	taken := make(map[string]bool)
	for _, letter := range s.usedLetters {
		taken[strings.ToUpper(letter)] = true
	}
	for _, remote := range s.remotesList {
		if remote.Name == s.editName {
			continue
		}
		if remote.MountPoint != "" {
			taken[strings.ToUpper(remote.MountPoint)] = true
		}
	}
	letters := make([]string, 0, 26)
	for i := 'Z'; i >= 'C'; i-- {
		letter := string(i) + ":"
		if !taken[letter] {
			letters = append(letters, letter)
		}
	}
	return letters
}

func defaultWebdavPort(protocol string) string {
	if protocol == "https" {
		return "443"
	}
	return "80"
}

// splitWebdavUrl 把已保存的 url 拆回协议/主机/端口（编辑回填用）。
func (s *driveEditorState) splitWebdavUrl(raw string) {
	if raw == "" {
		s.webdavProtocol, s.webdavHost, s.webdavPort = "http", "", "80"
		return
	}
	rest := raw
	protocol := "http"
	if strings.HasPrefix(rest, "https://") {
		protocol, rest = "https", rest[len("https://"):]
	} else if strings.HasPrefix(rest, "http://") {
		rest = rest[len("http://"):]
	}
	host, port := rest, defaultWebdavPort(protocol)
	if i := strings.IndexByte(rest, ':'); i >= 0 {
		host = rest[:i]
		if p := strings.TrimSpace(rest[i+1:]); p != "" {
			port = p
		}
	}
	s.webdavProtocol, s.webdavHost, s.webdavPort = protocol, host, port
}

// joinWebdavUrl 把协议/主机/端口拼回 url（组合值仍写入 options.url）。
func (s *driveEditorState) joinWebdavUrl() string {
	host := strings.TrimSpace(s.webdavHost)
	if host == "" {
		return ""
	}
	port := strings.TrimSpace(s.webdavPort)
	if port == "" {
		port = defaultWebdavPort(s.webdavProtocol)
	}
	return s.webdavProtocol + "://" + host + ":" + port
}

// onWebdavProtocol 切换协议时跟随默认端口（对应旧 onWebdavProtocol）。
func (s *driveEditorState) onWebdavProtocol(next string) {
	prev := s.webdavProtocol
	s.webdavProtocol = next
	if s.webdavPort == "" || s.webdavPort == defaultWebdavPort(prev) {
		s.webdavPort = defaultWebdavPort(next)
	}
}

// fieldSegment 是 segmentFields 的一个段：连续的 bool 字段一组，其余单独。
type fieldSegment struct {
	bools []rclone.Field
	field rclone.Field // bools 为空时有效
}

// segmentFields 把连续的 bool 字段并排成一行（对应旧 segmentFields）。
func segmentFields(list []rclone.Field) []fieldSegment {
	var segs []fieldSegment
	var bools []rclone.Field
	flush := func() {
		if len(bools) > 0 {
			segs = append(segs, fieldSegment{bools: bools})
			bools = nil
		}
	}
	for _, field := range list {
		if field.Type == "bool" {
			bools = append(bools, field)
		} else {
			flush()
			segs = append(segs, fieldSegment{field: field})
		}
	}
	flush()
	return segs
}

// exampleOptions 拆出枚举字段的标签/值两个平行切片。
func exampleOptions(field *rclone.Field) (labels, values []string) {
	for _, example := range field.Examples {
		label := example.Label
		if label == "" {
			label = example.Value
		}
		labels = append(labels, label)
		values = append(values, example.Value)
	}
	return labels, values
}

func containsString(list []string, want string) bool {
	for _, item := range list {
		if item == want {
			return true
		}
	}
	return false
}
