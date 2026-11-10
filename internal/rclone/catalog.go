package rclone

import (
	"fmt"
	"sort"
	"strings"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/lib/env"
)

// FieldExample 是枚举字段的一个可选值；Value 是写入配置的值，Label 是展示文字。
type FieldExample struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

// Field 是一个配置字段，元数据全部来自 rclone 的 fs.Option。
type Field struct {
	Key      string         `json:"key"`
	Label    string         `json:"label"`
	Help     string         `json:"help"`
	Type     string         `json:"type"`
	Default  string         `json:"default"`
	Required bool           `json:"required"`
	Password bool           `json:"password"`
	Advanced bool           `json:"advanced"`
	Provider string         `json:"provider,omitempty"`
	Examples []FieldExample `json:"examples,omitempty"`
}

// BackendType 描述一种连接类型。
type BackendType struct {
	Type  string `json:"type"`
	Label string `json:"label"`
	Help  string `json:"help"`
	// Authorized 为 true 表示该类型需要先完成 OAuth 授权（可粘贴 rclone token 使用）
	Authorized bool `json:"authorized"`
}

// BackendGroup 是连接类型树的一个分组。
type BackendGroup struct {
	Label string        `json:"label"`
	Types []BackendType `json:"types"`
}

// localUser 是本机账户名。rclone 把它当成 sftp / ftp / smb 的默认登录名（backend/*: `Default: currentUser`，
// currentUser = env.CurrentUser()），但那是「这台机器」的账户、不是远端主机的账户，预填还会被写进配置，
// 所以这里不预填；留空时 rclone 运行时照样会回退到它（如 backend/sftp/sftp.go: `if opt.User == "" { opt.User = currentUser }`）。
var localUser = env.CurrentUser()

// Fields 返回某个连接类型的配置字段（基本 + 高级），顺序与 rclone 定义一致。
func Fields(typ string) ([]Field, error) {
	info, err := fs.Find(typ)
	if err != nil || info == nil {
		return nil, fmt.Errorf("不支持的连接类型: %s", typ)
	}

	fields := make([]Field, 0, len(info.Options))
	for _, option := range info.Options {
		// 仅命令行使用的选项不在界面上展示
		if option.Hide&fs.OptionHideConfigurator != 0 {
			continue
		}
		fallback := option.String()
		if fallback != "" && fallback == localUser {
			fallback = ""
		}
		field := Field{
			Key:      option.Name,
			Label:    optionLabel(option.Name),
			Help:     optionHelp(option.Name, strings.TrimSpace(option.Help)),
			Type:     option.Type(),
			Default:  fallback,
			Required: option.Required,
			// 只把 IsPassword 当作密码输入框：rclone 的 Sensitive 只表示「日志/rc 输出里要打码」，
			// 很多非机密字段（host、user、port、endpoint、region 等）也标了它，用密码框会没法看
			Password: option.IsPassword,
			Advanced: option.Advanced,
			Provider: strings.TrimSpace(option.Provider),
		}
		for _, example := range option.Examples {
			label := strings.TrimSpace(example.Help)
			if label == "" {
				label = example.Value
			}
			// webdav 服务商选项的英文说明换成简短中文（value 不变，写入配置的仍是标识符）
			if typ == "webdav" && option.Name == "vendor" {
				if zh, ok := webdavVendorLabels[example.Value]; ok {
					label = zh
				}
			}
			field.Examples = append(field.Examples, FieldExample{Value: example.Value, Label: label})
		}
		fields = append(fields, field)
	}
	return fields, nil
}

// Catalog 返回按用途分组的全部连接类型（顺序固定，未知类型归入「其他」）。
func Catalog() []BackendGroup {
	byName := make(map[string]*fs.RegInfo, len(fs.Registry))
	for _, info := range fs.Registry {
		if info == nil || info.Hide {
			continue
		}
		if _, ok := byName[info.Name]; ok {
			continue
		}
		byName[info.Name] = info
	}

	groups := make([]BackendGroup, 0, len(backendGroups)+1)
	used := make(map[string]bool, len(byName))
	for _, group := range backendGroups {
		types := make([]BackendType, 0, len(group.types))
		for _, name := range group.types {
			info, ok := byName[name]
			if !ok || used[name] {
				continue
			}
			used[name] = true
			types = append(types, backendType(info))
		}
		if len(types) > 0 {
			groups = append(groups, BackendGroup{Label: group.label, Types: types})
		}
	}

	rest := make([]BackendType, 0, len(byName)-len(used))
	for name, info := range byName {
		if used[name] {
			continue
		}
		rest = append(rest, backendType(info))
	}
	if len(rest) > 0 {
		sort.Slice(rest, func(i, j int) bool { return rest[i].Type < rest[j].Type })
		groups = append(groups, BackendGroup{Label: "其他", Types: rest})
	}
	return groups
}

func backendType(info *fs.RegInfo) BackendType {
	label := backendLabels[info.Name]
	if label == "" {
		label = shortLabel(info.Description)
	}
	help, ok := backendDescriptions[info.Name]
	if !ok {
		help = strings.TrimSpace(info.Description)
	}
	return BackendType{
		Type:       info.Name,
		Label:      label,
		Help:       help,
		Authorized: needsAuth(info),
	}
}

// needsAuth 判断后端是否需要 OAuth 授权：rclone 的授权后端都会带一个 token 选项。
func needsAuth(info *fs.RegInfo) bool {
	for _, option := range info.Options {
		if option.Name == "token" {
			return true
		}
	}
	return false
}

// shortLabel 取后端描述的首句并截断，作为未收录中文名时的兜底标签。
func shortLabel(description string) string {
	text := strings.TrimSpace(strings.SplitN(description, "\n", 2)[0])
	if i := strings.Index(text, " including "); i > 0 {
		text = text[:i]
	}
	if i := strings.IndexAny(text, ",，"); i > 0 {
		text = text[:i]
	}
	runes := []rune(text)
	if len(runes) > 26 {
		return string(runes[:26]) + "…"
	}
	return text
}

// backendGroups 是连接类型的分组顺序；未列出的后端自动归入「其他」。
var backendGroups = []struct {
	label string
	types []string
}{
	{
		label: "常用协议",
		types: []string{"webdav", "sftp", "ftp", "smb", "http", "local", "memory"},
	},
	{
		label: "对象存储",
		types: []string{
			"s3", "azureblob", "azurefiles", "google cloud storage", "oracleobjectstorage",
			"qingstor", "swift", "b2", "storj", "sia", "hdfs", "netstorage",
			"cloudinary", "imagekit", "pixeldrain", "gofile", "fichier", "filelu",
			"ulozto", "quatrix", "filescom", "filefabric", "seafile", "archive",
		},
	},
	{
		label: "网盘与协作",
		types: []string{
			"drive", "onedrive", "dropbox", "box", "mega", "pcloud", "yandex",
			"mailru", "protondrive", "pikpak", "koofr", "jottacloud", "sugarsync",
			"sharefile", "hidrive", "huaweidrive", "iclouddrive", "internxt",
			"linkbox", "opendrive", "premiumizeme", "putio", "zoho", "drime",
			"filen", "google photos", "shade", "doi", "internetarchive",
		},
	},
	{
		label: "加密与联合",
		types: []string{"crypt", "alias", "union", "combine", "chunker", "hasher", "compress", "cache"},
	},
}

// backendLabels 是常见后端的界面名；未收录的用 rclone 描述兜底。
var backendLabels = map[string]string{
	"alias":                "别名（引用已有远程）",
	"archive":              "压缩包（只读）",
	"azureblob":            "Azure Blob 存储",
	"azurefiles":           "Azure 文件",
	"b2":                   "Backblaze B2",
	"box":                  "Box",
	"cache":                "缓存远程",
	"chunker":              "分块远程",
	"cloudinary":           "Cloudinary",
	"combine":              "组合多个远程",
	"compress":             "压缩远程",
	"crypt":                "加密远程",
	"doi":                  "DOI 数据集",
	"drime":                "Drime",
	"drive":                "Google Drive",
	"dropbox":              "Dropbox",
	"fichier":              "1Fichier",
	"filefabric":           "Enterprise File Fabric",
	"filelu":               "FileLu",
	"filen":                "Filen",
	"filescom":             "Files.com",
	"ftp":                  "FTP",
	"gofile":               "Gofile",
	"google cloud storage": "Google Cloud Storage",
	"google photos":        "Google 相册",
	"hasher":               "校验和远程",
	"hdfs":                 "HDFS",
	"hidrive":              "HiDrive",
	"http":                 "HTTP（只读）",
	"huaweidrive":          "华为云盘",
	"iclouddrive":          "iCloud Drive",
	"imagekit":             "ImageKit.io",
	"internetarchive":      "Internet Archive",
	"internxt":             "Internxt Drive",
	"jottacloud":           "Jottacloud",
	"koofr":                "Koofr / Digi Storage",
	"linkbox":              "Linkbox",
	"local":                "本地磁盘",
	"mailru":               "Mail.ru 云盘",
	"mega":                 "MEGA",
	"memory":               "内存存储（临时）",
	"netstorage":           "Akamai NetStorage",
	"onedrive":             "OneDrive",
	"opendrive":            "OpenDrive",
	"oracleobjectstorage":  "Oracle 对象存储",
	"pcloud":               "pCloud",
	"pikpak":               "PikPak",
	"pixeldrain":           "Pixeldrain",
	"premiumizeme":         "premiumize.me",
	"protondrive":          "Proton Drive",
	"putio":                "put.io",
	"qingstor":             "青云对象存储",
	"quatrix":              "Quatrix",
	"s3":                   "S3 兼容存储（含阿里云 OSS、腾讯 COS 等）",
	"seafile":              "Seafile",
	"sftp":                 "SFTP",
	"shade":                "Shade FS",
	"sharefile":            "Citrix ShareFile",
	"sia":                  "Sia",
	"smb":                  "SMB / CIFS（Windows 共享）",
	"storj":                "Storj",
	"sugarsync":            "SugarSync",
	"swift":                "OpenStack Swift",
	"ulozto":               "Uloz.to",
	"union":                "联合（合并多个远程）",
	"webdav":               "WebDAV",
	"yandex":               "Yandex Disk",
	"zoho":                 "Zoho",
}

// webdavVendorLabels、optionLabels、optionHelps 与 optionLabel/optionHelp
// 在 localize.go：界面文案中文表集中维护，覆盖率由 localize_test.go 守卫。
