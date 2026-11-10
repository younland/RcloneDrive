package rclone

import "testing"

// commonProtocols 是覆盖率守卫覆盖的常用协议；这些类型的全部界面字段
// 必须有中文标签与中文说明（漏翻时本测试失败并列出缺失清单）。
var commonProtocols = []string{"smb", "webdav", "ftp", "sftp", "http"}

// 常用协议字段的标签与说明必须全部命中中文表（2026-10 中文化守卫）。
func TestCommonProtocolFieldsLocalized(t *testing.T) {
	for _, typ := range commonProtocols {
		fields, err := Fields(typ)
		if err != nil {
			t.Fatalf("Fields(%s): %v", typ, err)
		}
		var missingLabel, missingHelp []string
		for _, f := range fields {
			if _, ok := optionLabels[f.Key]; !ok {
				missingLabel = append(missingLabel, f.Key)
			}
			if _, ok := optionHelps[f.Key]; !ok {
				missingHelp = append(missingHelp, f.Key)
			}
		}
		if len(missingLabel) > 0 {
			t.Errorf("%s 缺少中文标签的字段: %v", typ, missingLabel)
		}
		if len(missingHelp) > 0 {
			t.Errorf("%s 缺少中文说明的字段: %v", typ, missingHelp)
		}
	}
}

// 常用协议的类型描述必须是中文（未收录的类型保留英文原文为合法降级）。
func TestCommonProtocolDescriptionsLocalized(t *testing.T) {
	for _, typ := range commonProtocols {
		if desc, ok := backendDescriptions[typ]; !ok || desc == "" {
			t.Errorf("类型 %s 缺少中文描述", typ)
		}
	}
}

// 抽查锚定：防止映射表被误清空或 key 打错仍通过集合检查。
func TestLocalizedSpotChecks(t *testing.T) {
	if got := optionLabel("host"); got != "主机地址" {
		t.Errorf("optionLabel(host) = %q, want 主机地址", got)
	}
	if got := optionHelp("idle_timeout", ""); got == "" || got == "fallback" {
		t.Errorf("optionHelp(idle_timeout) = %q, want 中文说明", got)
	}
	// 兜底：未收录的 key 蛇形转标题，不返回空
	if got := optionLabel("some_new_option"); got != "Some New Option" {
		t.Errorf("兜底标题 = %q, want Some New Option", got)
	}
}
