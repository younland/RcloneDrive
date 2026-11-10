package rclone

import "strings"

// 本文件集中维护界面文案的中文本地化表：
//   - optionLabels / optionHelps：按字段名（跨后端通用）映射标签与说明。
//   - backendDescriptions：连接类型的中文描述（类型树展示）。
//   - webdavVendorLabels：webdav 服务商示例选项的中文展示名。
//
// 未收录的字段按兜底规则降级（标签蛇形转标题、说明保留 rclone 原文），
// 覆盖率守卫见 localize_test.go。说明采用要点式概括（非逐句直译），
// 以适配 tooltip 的展示长度。

// backendDescriptions 是连接类型的中文描述；未收录的类型保留 rclone 原文。
var backendDescriptions = map[string]string{
	"webdav": "通过 WebDAV 协议连接网盘或自建服务（如坚果云、Nextcloud、群晖）。",
	"smb":    "连接 Windows/Samba 网络共享或 NAS 的 SMB/CIFS 共享。",
	"ftp":    "通过 FTP/FTPS 协议连接文件服务器。",
	"sftp":   "通过 SSH 文件传输协议（SFTP）连接服务器，支持密码或密钥登录。",
	"http":   "以只读方式浏览 HTTP 网站上的文件目录。",
	"local":  "挂载本机磁盘或已挂载的外部存储目录。",
	"memory": "内存中的临时存储，应用退出即消失（用于测试）。",
}

// webdavVendorLabels 是 webdav 服务商选项的简短中文名（value 是写入配置的标识符，不变）。
var webdavVendorLabels = map[string]string{
	"fastmail":        "Fastmail 文件",
	"nextcloud":       "Nextcloud 私有云",
	"owncloud":        "ownCloud 10（PHP 版）",
	"infinitescale":   "ownCloud Infinite Scale",
	"sharepoint":      "SharePoint 在线版（微软账号）",
	"sharepoint-ntlm": "SharePoint（NTLM 认证，自建/本地）",
	"rclone":          "rclone WebDAV 服务器",
	"other":           "其他服务",
}

// optionHelp 返回字段说明的中文翻译，未收录时保留 rclone 原文。
// 与 optionLabel 一样按字段名映射：这些字段在各后端里的含义基本一致。
func optionHelp(name, fallback string) string {
	if help, ok := optionHelps[name]; ok {
		return help
	}
	return fallback
}

// optionHelps 是常见配置字段说明的中文提示（tooltip 内容，要点式概括）。
var optionHelps = map[string]string{
	// 通用 / 认证
	"url":                    "要连接的服务器地址，例如 https://example.com。",
	"vendor":                 "所用的 WebDAV 服务/软件类型，影响 rclone 的兼容性处理；不确定时选「其他服务」。",
	"user":                   "登录该服务的用户名。",
	"pass":                   "登录该服务的密码。",
	"host":                   "服务器主机名或 IP 地址。",
	"port":                   "服务器端口。",
	"bearer_token":           "用 Bearer 令牌代替用户名/密码登录（如 Macaroon）。",
	"bearer_token_command":   "执行指定命令来获取 Bearer 令牌。",	"token":                  "OAuth 授权令牌，可通过 rclone authorize 获取。",
	"ask_password":           "未提供密码时在需要时询问密码。",
	"socks_proxy":            "SOCKS5 代理地址，格式 user:pass@host:port。",
	"http_proxy":             "HTTP 代理（CONNECT）地址，格式 http://host:port。",
	"description":            "该远程存储的备注说明。",
	"headers":                "为所有请求附加 HTTP 头，格式如 \"Cookie\",\"name=value\"，逗号分隔多组。",
	"idle_timeout":           "空闲连接的最长保留时间，超时后关闭连接池；设为 0 表示一直保留。",
	"encoding":               "后端的文件名编码规则，一般保持默认。",
	// SMB
	"domain":               "NTLM 认证使用的域（Windows 域环境才需要，个人 NAS 通常留空）。",
	"spn":                  "服务主体名（SPN），集群环境才需要；例如 cifs/remotehost:1020，不确定留空。",
	"use_kerberos":         "启用 Kerberos 认证代替 NTLM（需要本机已配置好 Kerberos）。",
	"hide_special_share":   "隐藏用户无权访问的特殊共享（如 print$）。",
	"case_insensitive":     "服务器是否大小写不敏感；Windows 共享恒为是。",
	"kerberos_ccache":      "Kerberos 凭据缓存文件路径（krb5cc），多凭据挂载时用于覆盖默认位置。",
	// WebDAV
	"pacer_min_sleep":         "相邻 API 请求之间的最小间隔时间。",
	"nextcloud_chunk_size":    "Nextcloud 分块上传大小；设为 0 关闭分块上传。",
	"owncloud_exclude_shares": "排除 ownCloud 的共享条目。",
	"owncloud_exclude_mounts": "排除 ownCloud 的挂载存储条目。",
	"unix_socket":             "改用 Unix 域套接字连接（代替 TCP），用于本机反代场景。",
	"auth_redirect":           "重定向时保留认证信息（遇到 401 报错时可尝试开启，注意会降低安全性）。",
	// FTP
	"tls":                         "使用隐式 FTPS（FTP over TLS）：连接一开始就启用 TLS，通常端口 990。",
	"explicit_tls":                "使用显式 FTPS：先明文连接再升级为 TLS 加密，通常端口 21。",
	"concurrency":                 "FTP 并发连接数上限，0 为不限。设置过大容易死锁，需与传输线程数配合。",
	"no_check_certificate":        "不校验服务器 TLS 证书（自签名证书时开启，注意有中间人风险）。",
	"disable_epsv":                "即使服务器支持也不使用 EPSV 被动模式。",
	"disable_mlsd":                "即使服务器支持也不使用 MLSD 列目录命令。",
	"disable_utf8":                "即使服务器支持也不使用 UTF-8 文件名。",
	"writing_mdtm":                "用 MDTM 命令设置修改时间（VsFtpd 的兼容怪癖）。",
	"force_list_hidden":           "用 LIST -a 强制列出隐藏文件（会禁用 MLSD）。",
	"close_timeout":               "等待连接关闭响应的最长时间。",
	"tls_cache_size":              "TLS 会话缓存大小（用于会话复用）；0 为关闭。",
	"disable_tls13":               "禁用 TLS 1.3（对 TLS 实现有 bug 的 FTP 服务器的变通办法）。",
	"allow_insecure_tls_ciphers":  "允许不安全的 TLS 加密套件（仅在服务器过旧时使用）。",
	"shut_timeout":                "等待数据连接关闭状态的最长时间。",
	"no_check_upload":             "上传后不检查文件是否成功写入（用于只写目录，需配合 --inplace）。",
	// SFTP
	"key_pem":                    "PEM 格式的 SSH 私钥内容（单行，换行写成 \\n）；填写后覆盖私钥文件项。",
	"pubkey":                     "SSH 公钥证书内容（证书认证用）。",
	"pubkey_file":                "公钥证书文件路径。",
	"known_hosts_file":           "known_hosts 文件路径，用于校验服务器主机密钥；填 none 关闭提示。",
	"host_keys":                  "本远程固定的服务器主机密钥列表，用于校验服务器身份（最多 16 条）。",
	"key_use_agent":              "强制使用 ssh-agent 认证（本机密钥很多时可避免认证失败报错）。",
	"use_insecure_cipher":        "启用不安全的加密与密钥交换算法（连接老旧服务器时才用）。",
	"disable_hashcheck":          "禁用通过 SSH 命令检测哈希能力（部分受限服务器需要）。",
	"path_override":              "覆盖 SSH 命令使用的路径（SFTP 与 SSH 路径不一致时用，如群晖 NAS）。",
	"set_modtime":                "上传后设置远程文件的修改时间。",
	"shell_type":                 "远程服务器的 shell 类型，留空自动检测。",
	"hashes":                     "支持的校验和类型列表（逗号分隔），留空自动检测。",
	"md5sum_command":             "读取 MD5 的命令，留空自动检测。",
	"sha1sum_command":            "读取 SHA-1 的命令，留空自动检测。",
	"crc32sum_command":           "读取 CRC-32 的命令，留空自动检测。",
	"sha256sum_command":          "读取 SHA-256 的命令，留空自动检测。",
	"blake3sum_command":          "读取 BLAKE3 的命令，留空自动检测。",
	"xxh3sum_command":            "读取 XXH3 的命令，留空自动检测。",
	"xxh128sum_command":          "读取 XXH128 的命令，留空自动检测。",
	"skip_links":                 "列目录时跳过符号链接等非常规文件（不影响写入行为）。",
	"subsystem":                  "远程主机的 SSH2 子系统，一般保持默认 sftp。",
	"server_command":             "在远程主机上启动 SFTP 服务的命令（受限服务器用，如 sudo /usr/libexec/openssh/sftp-server）。",
	"use_fstat":                  "用 fstat 代替 stat（限制同时打开文件数的服务器需要）。",
	"disable_concurrent_reads":   "禁用并发读取（服务器限制同一文件下载次数时开启）。",
	"disable_concurrent_writes":  "禁用并发写入（会明显降低上传速度，仅在服务器不支持时开启）。",
	"connections":                "SFTP 并发连接数上限，0 为不限；设置过大容易死锁，需与传输线程数配合。",
	"set_env":                    "传给 sftp 客户端与远程命令的环境变量，空格分隔，如 VAR=value。",
	"ciphers":                    "会话加密算法列表（按优先级，空格分隔），需与服务器配置匹配。",
	"key_exchange":               "密钥交换算法列表（按优先级，空格分隔）。",
	"macs":                       "消息认证码（MAC）算法列表（按优先级，空格分隔）。",
	"host_key_algorithms":        "主机密钥算法列表（按优先级，空格分隔）。",
	"ssh":                        "使用外部 ssh 程序连接（配置其全部参数，需免密登录）。",
	"copy_is_hardlink":           "服务端复制改用硬链接实现（部分服务器支持，备份场景有用）。",
	// HTTP
	"no_slash":  "目标网站的目录不以 / 结尾时开启（rclone 将把 text/html 文件视为目录）。",
	"no_head":   "不使用 HEAD 请求探测文件（目录浏览更快但拿不到大小与时间）。",
	"no_escape": "不对路径中的 URL 元字符转义（个别编码异常的服务器需要）。",
	// 既有条目（保持原文案）
	"key_file":      "SSH 私钥文件路径，留空表示使用密码登录。",
	"key_file_pass": "私钥文件的口令（私钥未设口令则留空）。",
	// 通用传输参数（多后端共享同名 key，语义一致）
	"chunk_size": "单次传输的分块大小；加大可提高高延迟链路速度，异常报错时调回默认。",
}

// optionLabels 是常见配置字段的中文标签；未收录的用字段名转标题兜底。
var optionLabels = map[string]string{
	// 通用 / 认证 / 对象存储
	"access_key_id":           "Access Key ID",
	"bearer_token":            "Bearer 令牌",
	"account":                 "账号",
	"acl":                     "访问控制（ACL）",
	"auth_url":                "授权地址",
	"bucket":                  "存储桶",
	"chunk_size":              "分块大小",
	"client_id":               "客户端 ID（Client ID）",
	"client_secret":           "客户端密钥（Client Secret）",
	"config_is_local":         "使用本地配置文件",
	"description":             "说明",
	"encoding":                "文件名编码",
	"endpoint":                "接入点（Endpoint）",
	"env_auth":                "从环境变量读取凭据",
	"host":                    "主机地址",
	"key_file":                "私钥文件",
	"key_file_pass":           "私钥口令",
	"location":                "区域",
	"no_check_bucket":         "不自动创建存储桶",
	"pass":                    "密码",
	"path":                    "路径",
	"port":                    "端口",
	"provider":                "服务商",
	"region":                  "区域（Region）",
	"remote":                  "远程路径",
	"root_folder_id":          "根目录 ID",
	"secret_access_key":       "Secret Access Key",
	"server_side_encryption":  "服务端加密",
	"session_token":           "会话令牌（Session Token）",
	"shared_credentials_file": "共享凭据文件",
	"storage_class":           "存储级别",
	"team_drive":              "共享盘",
	"token":                   "授权令牌（token）",
	"token_url":               "令牌地址",
	"upload_cutoff":           "分块上传阈值",
	"url":                     "服务器地址",
	"use_multipart_upload":    "使用分段上传",
	"user":                    "用户名",
	"vendor":                  "服务商",
	// 通用（连接行为）
	"idle_timeout":    "空闲超时",
	"ask_password":    "按需询问密码",
	"socks_proxy":     "SOCKS5 代理",
	"http_proxy":      "HTTP 代理",
	"headers":         "HTTP 头",
	"concurrency":     "并发数",
	// SMB
	"domain":              "域",
	"spn":                 "服务主体名（SPN）",
	"use_kerberos":        "使用 Kerberos 认证",
	"hide_special_share":  "隐藏特殊共享",
	"case_insensitive":    "大小写不敏感",
	"kerberos_ccache":     "Kerberos 凭据缓存",
	// WebDAV
	"bearer_token_command":   "Bearer 令牌命令",
	"pacer_min_sleep":        "请求最小间隔",
	"nextcloud_chunk_size":   "Nextcloud 分块大小",
	"owncloud_exclude_shares": "排除 ownCloud 共享",
	"owncloud_exclude_mounts": "排除 ownCloud 挂载",
	"unix_socket":             "Unix 套接字",
	"auth_redirect":           "重定向保留认证",
	// FTP
	"tls":                        "隐式 FTPS（TLS）",
	"explicit_tls":               "显式 FTPS（TLS）",
	"no_check_certificate":       "不校验证书",
	"disable_epsv":               "禁用 EPSV",
	"disable_mlsd":               "禁用 MLSD",
	"disable_utf8":               "禁用 UTF-8",
	"writing_mdtm":               "用 MDTM 写修改时间",
	"force_list_hidden":          "强制列出隐藏文件",
	"close_timeout":              "关闭超时",
	"tls_cache_size":             "TLS 会话缓存",
	"disable_tls13":              "禁用 TLS 1.3",
	"allow_insecure_tls_ciphers": "允许不安全加密套件",
	"shut_timeout":               "数据连接关闭超时",
	"no_check_upload":            "不检查上传结果",
	// SFTP
	"key_pem":                   "私钥内容（PEM）",
	"pubkey":                    "公钥证书",
	"pubkey_file":               "公钥证书文件",
	"known_hosts_file":          "known_hosts 文件",
	"host_keys":                 "固定主机密钥",
	"key_use_agent":             "使用 ssh-agent",
	"use_insecure_cipher":       "使用不安全加密算法",
	"disable_hashcheck":         "禁用哈希检测",
	"path_override":             "SSH 路径覆盖",
	"set_modtime":               "设置修改时间",
	"shell_type":                "Shell 类型",
	"hashes":                    "校验和类型",
	"md5sum_command":            "MD5 命令",
	"sha1sum_command":           "SHA-1 命令",
	"crc32sum_command":          "CRC-32 命令",
	"sha256sum_command":         "SHA-256 命令",
	"blake3sum_command":         "BLAKE3 命令",
	"xxh3sum_command":           "XXH3 命令",
	"xxh128sum_command":         "XXH128 命令",
	"skip_links":                "跳过符号链接",
	"subsystem":                 "SSH 子系统",
	"server_command":            "远程 SFTP 服务命令",
	"use_fstat":                 "使用 fstat",
	"disable_concurrent_reads":  "禁用并发读取",
	"disable_concurrent_writes": "禁用并发写入",
	"connections":               "并发连接数",
	"set_env":                   "环境变量",
	"ciphers":                   "加密算法",
	"key_exchange":              "密钥交换算法",
	"macs":                      "MAC 算法",
	"host_key_algorithms":       "主机密钥算法",
	"ssh":                       "外部 ssh 程序",
	"copy_is_hardlink":          "复制用硬链接",
	// HTTP
	"no_slash":  "目录不带斜杠",
	"no_head":   "不用 HEAD 请求",
	"no_escape": "不转义 URL 字符",
}

// optionLabel 返回字段的中文标签，未收录时把 rclone 的蛇形名转成可读标题。
func optionLabel(name string) string {
	if label, ok := optionLabels[name]; ok && label != "" {
		return label
	}
	words := strings.FieldsFunc(name, func(r rune) bool { return r == '_' })
	for i, word := range words {
		words[i] = strings.ToUpper(word[:1]) + word[1:]
	}
	return strings.Join(words, " ")
}
