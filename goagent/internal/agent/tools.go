package agent

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"syscall"
	"time"

	"eagent-cli/internal/config"
	"eagent-cli/internal/tools"
)

// ToolCall 一次工具调用。Body 为标记之间的原始内容。
type ToolCall struct {
	Name   string
	Body   string
	Legacy bool
}

var newMarkerRe = regexp.MustCompile(`EAGENT_TOOL\s+(\S+)\s+BEGIN`)

var newMarkerEndRe = regexp.MustCompile(`EAGENT_TOOL\s+END`)

// knownTools 是新协议支持的正式工具名；白名单外的名字按占位符/叙述忽略。
var knownTools = map[string]bool{
	"ps": true, "sh": true, "getfile": true,
	"screenshot": true, "mouse": true, "topmost": true,
}

// legacyPairs 兼容易语言原版的识别头/识别尾协议。
var legacyPairs = []struct{ head, tail, name string }{
	{"# EAgent_ps_识别头_完全保留本行注释", "# EAgent_ps_识别尾_完全保留本行注释", "ps"},
	{"# EAgent_getfile_识别头_完全保留本行注释", "# EAgent_getfile_识别尾_完全保留本行注释", "getfile"},
	{"# EAgent_getjpg_识别头_完全保留本行注释", "# EAgent_getjpg_识别尾_完全保留本行注释", "screenshot"},
	{"# EAgent_mouse_simulate_识别头_完全保留本行注释", "# EAgent_mouse_simulate_识别尾_完全保留本行注释", "mouse"},
	{"# EAgent_窗口_置顶_识别头_完全保留本行注释", "# EAgent_窗口_置顶_识别尾_完全保留本行注释", "topmost"},
}

// ParseTools 从回复文本中解析全部工具调用（新协议 + 旧协议）。
func ParseTools(text string) []ToolCall {
	var out []ToolCall
	for _, loc := range newMarkerRe.FindAllStringIndex(text, -1) {
		name := newMarkerRe.FindStringSubmatch(text[loc[0]:loc[1]])[1]
		if !knownTools[name] {
			continue // "..."/叙述文字等，不是真实调用
		}
		rest := text[loc[1]:]
		out = append(out, ToolCall{Name: name, Body: trimToMarker(rest)})
	}
	for _, pair := range legacyPairs {
		from := 0
		for {
			i := strings.Index(text[from:], pair.head)
			if i < 0 {
				break
			}
			start := from + i + len(pair.head)
			j := strings.Index(text[start:], pair.tail)
			if j < 0 {
				break
			}
			body := strings.TrimSpace(text[start : start+j])
			from = start + j + len(pair.tail)
			out = append(out, ToolCall{Name: pair.name, Body: legacyBody(pair.name, body), Legacy: true})
		}
	}
	return out
}

// trimToMarker 裁掉 END 标记(含被渲染破坏的变体)之后的内容。
func trimToMarker(rest string) string {
	loc := newMarkerEndRe.FindStringIndex(rest)
	if loc != nil {
		rest = rest[:loc[0]]
	}
	// 去掉标记前被渲染残留的 # 行。
	body := strings.TrimSpace(rest)
	for strings.HasSuffix(body, "#") {
		body = strings.TrimSpace(strings.TrimSuffix(body, "#"))
	}
	return body
}

// legacyBody 把旧协议的参数包络(xxx_1_star(...)等)转成新协议的正文。
func legacyBody(name, body string) string {
	mid := func(pre, post string) string {
		i := strings.Index(body, pre)
		if i < 0 {
			return ""
		}
		rest := body[i+len(pre):]
		j := strings.Index(rest, post)
		if j < 0 {
			return ""
		}
		return strings.TrimSpace(rest[:j])
	}
	switch name {
	case "getfile":
		return mid("agent_getfile_head(", ")agent_getfile_end")
	case "screenshot":
		return mid("agent_getjpg_1_star(", ")_1_getjpg_end")
	case "mouse":
		x := mid("agent_mouse_simulate_1_start(", ")_agent_mouse_simulate_1_end")
		y := mid("agent_mouse_simulate_2_start(", ")_agent_mouse_simulate_2_end")
		btn := mid("agent_mouse_simulate_3_start(", ")_agent_mouse_simulate_3_end")
		verb := "click"
		if strings.TrimSpace(btn) == "2" {
			verb = "rclick"
		}
		return strings.TrimSpace(verb + " " + x + " " + y)
	case "topmost":
		h := mid("窗口_置顶_1_star(", ")_窗口_置顶_1_end")
		on := mid("窗口_置顶_2_start(", ")_窗口_置顶_2_end")
		flag := "off"
		if strings.Contains(strings.ToLower(on), "true") {
			flag = "on"
		}
		return strings.TrimSpace("hwnd " + h + " " + flag)
	}
	return body
}

// Engine 负责按策略执行工具并生成回传消息。
type Engine struct {
	A   *Agent
	Ask func(tc ToolCall) bool // policy=ask 时的确认回调
}

type execResult struct {
	feedback string
	attach   string
}

// HandleReply 解析并执行回复中的工具，返回要回传的消息与附件路径。
// 没有工具调用时 ran=false。
func (e *Engine) HandleReply(ctx context.Context, reply string) (feedbackMsg, attach string, ran bool, err error) {
	calls := ParseTools(reply)
	if len(calls) == 0 {
		return "", "", false, nil
	}
	var parts []string
	for _, tc := range calls {
		e.A.Logf("工具调用: %s (legacy=%v)", tc.Name, tc.Legacy)
		switch e.A.cfg.ExecPolicy {
		case "deny":
			parts = append(parts, "用户拒绝执行工具")
			continue
		case "ask":
			if e.Ask != nil && !e.Ask(tc) {
				parts = append(parts, "[工具] 用户拒绝执行命令")
				continue
			}
		}
		res, err := e.exec(ctx, tc)
		if err != nil {
			parts = append(parts, fmt.Sprintf("!![工具][%s] 执行出错!!\n%v", tc.Name, err))
			continue
		}
		parts = append(parts, res.feedback)
		if res.attach != "" {
			attach = res.attach
		}
	}
	return strings.Join(parts, "\n\n"), attach, true, nil
}

func (e *Engine) exec(ctx context.Context, tc ToolCall) (execResult, error) {
	cfg := e.A.cfg
	switch tc.Name {
	case "ps":
		if err := checkWhitelist(cfg.ExecAllow, tc.Body); err != nil {
			return execResult{feedback: "被执行白名单拦截: " + err.Error()}, nil
		}
		out, err := runPowerShell(ctx, cfg, e.A.TmpDir, tc.Body)
		if err != nil {
			if out != "" {
				return execResult{}, fmt.Errorf("%v\n输出: %s", err, truncate(out, 2000))
			}
			return execResult{}, err
		}
		if len([]rune(out)) > cfg.MaxInlineChars {
			file := e.A.TmpDir + fmt.Sprintf("\\ps_%d_out.txt", time.Now().UnixNano())
			if werr := os.WriteFile(file, []byte(out), 0o644); werr == nil {
				return execResult{feedback: fmt.Sprintf("工具输出较长(%d 字符)，已作为附件上传。", len([]rune(out))), attach: file}, nil
			}
		}
		if strings.TrimSpace(out) == "" {
			out = "(无输出)"
		}
		return execResult{feedback: out}, nil
	case "sh":
		if err := checkWhitelist(cfg.ExecAllow, tc.Body); err != nil {
			return execResult{feedback: "被执行白名单拦截: " + err.Error()}, nil
		}
		out, err := runShell(ctx, cfg, tc.Body)
		if err != nil {
			if out != "" {
				return execResult{}, fmt.Errorf("%v\n输出: %s", err, truncate(out, 2000))
			}
			return execResult{}, err
		}
		if len([]rune(out)) > cfg.MaxInlineChars {
			file := e.A.TmpDir + fmt.Sprintf("\\sh_%d_out.txt", time.Now().UnixNano())
			if werr := os.WriteFile(file, []byte(out), 0o644); werr == nil {
				return execResult{feedback: fmt.Sprintf("工具输出较长(%d 字符)，已作为附件上传。", len([]rune(out))), attach: file}, nil
			}
		}
		if strings.TrimSpace(out) == "" {
			out = "(无输出)"
		}
		return execResult{feedback: out}, nil
	case "getfile":
		var found []string
		for _, line := range strings.FieldsFunc(tc.Body, func(r rune) bool { return r == '\n' || r == '\r' || r == ',' }) {
			line = strings.TrimSpace(strings.Trim(line, `"'`))
			if line == "" {
				continue
			}
			if _, err := os.Stat(line); err != nil {
				return execResult{}, fmt.Errorf("文件不存在: %s", line)
			}
			found = append(found, line)
		}
		if len(found) == 0 {
			return execResult{}, fmt.Errorf("未提供任何文件路径")
		}
		return execResult{feedback: "已选择上传文件: " + strings.Join(found, "; "), attach: strings.Join(found, "\n")}, nil
	case "screenshot":
		file := e.A.TmpDir + fmt.Sprintf("\\screenshot_%d.png", time.Now().UnixNano())
		if err := tools.ScreenshotPNG(file); err != nil {
			return execResult{}, err
		}
		w, h := tools.ScreenSize()
		return execResult{feedback: fmt.Sprintf("截图已保存(%dx%d 物理像素): %s", w, h, file), attach: file}, nil
	case "mouse":
		return execMouse(tc.Body)
	case "topmost":
		return execTopmost(tc.Body)
	default:
		return execResult{}, fmt.Errorf("未知工具: %s", tc.Name)
	}
}

func checkWhitelist(patterns []string, code string) error {
	if len(patterns) == 0 {
		return nil
	}
	for _, p := range patterns {
		re, err := regexp.Compile(p)
		if err != nil {
			return fmt.Errorf("白名单正则非法 %q: %w", p, err)
		}
		if re.MatchString(code) {
			return nil
		}
	}
	return fmt.Errorf("代码不匹配任何白名单规则(%d 条)", len(patterns))
}

// runPowerShell 写出带 BOM 的 UTF-8 脚本(兼容 Windows PowerShell 5 的中文)，
// 隐藏窗口运行并捕获合并输出。
func runPowerShell(ctx context.Context, cfg *config.Config, tmpDir, code string) (string, error) {
	ps1 := tmpDir + fmt.Sprintf("\\ps_%d.ps1", time.Now().UnixNano())
	if err := os.WriteFile(ps1, append([]byte{0xEF, 0xBB, 0xBF}, []byte(code)...), 0o644); err != nil {
		return "", err
	}
	exe := cfg.PowerShellPath
	if exe == "" {
		exe = "powershell"
	}
	runCtx, cancel := context.WithTimeout(ctx, time.Duration(cfg.ExecTimeoutS)*time.Second)
	defer cancel()
	cmd := exec.CommandContext(runCtx, exe, "-NoProfile", "-ExecutionPolicy", "Bypass", "-File", ps1)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
	out, err := cmd.CombinedOutput()
	if runCtx.Err() != nil && ctx.Err() == nil {
		return string(out), fmt.Errorf("执行超时(%ds)", cfg.ExecTimeoutS)
	}
	return string(out), err
}

// runShell 通过 bash -c 执行命令（Git Bash/WSL），输出同样捕获合并流。
func runShell(ctx context.Context, cfg *config.Config, code string) (string, error) {
	exe := cfg.ShellPath
	if exe == "" {
		exe = "bash"
	}
	runCtx, cancel := context.WithTimeout(ctx, time.Duration(cfg.ExecTimeoutS)*time.Second)
	defer cancel()
	cmd := exec.CommandContext(runCtx, exe, "-c", code)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
	out, err := cmd.CombinedOutput()
	if runCtx.Err() != nil && ctx.Err() == nil {
		return string(out), fmt.Errorf("执行超时(%ds)", cfg.ExecTimeoutS)
	}
	return string(out), err
}

func execMouse(body string) (execResult, error) {
	f := strings.Fields(strings.Join(strings.Fields(body), " "))
	if len(f) < 3 {
		return execResult{}, fmt.Errorf("mouse 参数格式: click|rclick|move <x> <y>, 收到 %q", body)
	}
	var x, y int
	if _, err := fmt.Sscanf(f[1]+" "+f[2], "%d %d", &x, &y); err != nil {
		return execResult{}, fmt.Errorf("坐标解析失败: %w", err)
	}
	switch strings.ToLower(f[0]) {
	case "click":
		if err := tools.MouseClick(x, y, false); err != nil {
			return execResult{}, err
		}
	case "rclick":
		if err := tools.MouseClick(x, y, true); err != nil {
			return execResult{}, err
		}
	case "move":
		if err := tools.MouseMove(x, y); err != nil {
			return execResult{}, err
		}
	default:
		return execResult{}, fmt.Errorf("mouse 动作: click|rclick|move, 收到 %q", f[0])
	}
	return execResult{feedback: fmt.Sprintf("已执行 %s(%d,%d)。", strings.ToLower(f[0]), x, y)}, nil
}

func execTopmost(body string) (execResult, error) {
	f := strings.Fields(body)
	if len(f) < 3 {
		return execResult{}, fmt.Errorf("topmost 参数格式: hwnd <句柄> on|off, 收到 %q", body)
	}
	if strings.ToLower(f[0]) != "hwnd" {
		return execResult{}, fmt.Errorf("topmost 仅支持 hwnd 定位, 收到 %q", f[0])
	}
	var hwnd uintptr
	if _, err := fmt.Sscanf(f[1], "%d", &hwnd); err != nil {
		return execResult{}, fmt.Errorf("句柄解析失败: %w", err)
	}
	on := strings.EqualFold(f[2], "on") || strings.EqualFold(f[2], "true")
	if err := tools.Topmost(hwnd, on); err != nil {
		return execResult{}, err
	}
	return execResult{feedback: fmt.Sprintf("窗口 %d 置顶=%v 完成。", hwnd, on)}, nil
}

func truncate(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max]) + "…(已截断)"
}
