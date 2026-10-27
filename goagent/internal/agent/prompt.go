package agent

import (
	_ "embed"
	"strings"
)

//go:embed system_prompt.md
var SystemPrompt string

// RenderPrompt 渲染系统提示词模板中的占位符。
func RenderPrompt(workDir, tmpDir string) string {
	r := strings.NewReplacer("{{WORKDIR}}", workDir, "{{TEMPDIR}}", tmpDir)
	return r.Replace(SystemPrompt)
}
