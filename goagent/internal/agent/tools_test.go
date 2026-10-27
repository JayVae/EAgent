package agent

import "testing"

func TestParseToolsCanonical(t *testing.T) {
	text := "说明\n```\n### EAGENT_TOOL ps BEGIN\nWrite-Output 1\n### EAGENT_TOOL END\n```\n"
	calls := ParseTools(text)
	if len(calls) != 1 || calls[0].Name != "ps" || calls[0].Body != "Write-Output 1" {
		t.Fatalf("canonical parse failed: %+v", calls)
	}
}

// 网页 Markdown 渲染可能吃掉行首 ### 与换行，解析必须容忍。
func TestParseToolsMangled(t *testing.T) {
	text := "我先用工具\nEAGENT_TOOL ps BEGINos := 1\nEAGENT_TOOL END"
	calls := ParseTools(text)
	if len(calls) != 1 || calls[0].Name != "ps" {
		t.Fatalf("mangled parse failed: %+v", calls)
	}
}

func TestParseLegacy(t *testing.T) {
	text := "# EAgent_ps_识别头_完全保留本行注释\nWrite-Output 2\n# EAgent_ps_识别尾_完全保留本行注释"
	calls := ParseTools(text)
	if len(calls) != 1 || calls[0].Name != "ps" || calls[0].Body != "Write-Output 2" {
		t.Fatalf("legacy parse failed: %+v", calls)
	}
}

// AI 在正文里解释协议时不应被误判为工具调用。
func TestParseIgnoresPlaceholder(t *testing.T) {
	text := "用法示例: EAGENT_TOOL ... BEGIN 或 EAGENT_TOOL 工具名 BEGIN，需包在代码块里"
	if calls := ParseTools(text); len(calls) != 0 {
		t.Fatalf("placeholder must be ignored: %+v", calls)
	}
}

func TestParseSh(t *testing.T) {
	text := "```\n### EAGENT_TOOL sh BEGIN\necho hi\n### EAGENT_TOOL END\n```"
	calls := ParseTools(text)
	if len(calls) != 1 || calls[0].Name != "sh" || calls[0].Body != "echo hi" {
		t.Fatalf("sh parse failed: %+v", calls)
	}
}
