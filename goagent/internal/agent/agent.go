// Package agent 实现 Agent 主循环：发送 → 等待回复 → 解析工具 → 执行 → 回传。
package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"time"

	"eagent-cli/internal/cdp"
	"eagent-cli/internal/config"
	"eagent-cli/internal/script"
	"eagent-cli/internal/selectors"
)

type pageState struct {
	ACount    int    `json:"a_count"`
	UCount    int    `json:"u_count"`
	Last      string `json:"last"`
	HasInput  bool   `json:"has_input"`
	SendState string `json:"send_state"`
}

type Agent struct {
	cfg    *config.Config
	src    *selectors.Source
	prof   *selectors.Profile
	cdp    *cdp.Client
	TmpDir string
	Logf   func(format string, args ...any)
}

func New(cfg *config.Config, src *selectors.Source, tmpDir string, logf func(string, ...any)) (*Agent, error) {
	prof, err := src.Load()
	if err != nil {
		return nil, err
	}
	client, err := cdp.New(cfg.Port, prof.URLMatch)
	if err != nil {
		return nil, err
	}
	client.Logf = logf
	return &Agent{cfg: cfg, src: src, prof: prof, cdp: client, TmpDir: tmpDir, Logf: logf}, nil
}

func (a *Agent) logf(format string, args ...any) {
	if a.Logf != nil {
		a.Logf(format, args...)
	}
}

func (a *Agent) Client() *cdp.Client         { return a.cdp }
func (a *Agent) Profile() *selectors.Profile { return a.prof }

// ReloadSelectors 重新加载选择器配置（文件来源支持热重载）。
func (a *Agent) ReloadSelectors() (bool, error) {
	changed, prof, err := a.src.ReloadIfChanged()
	if err != nil {
		return false, err
	}
	if changed {
		a.prof = prof
		client, err := cdp.New(a.cfg.Port, prof.URLMatch)
		if err != nil {
			return false, err
		}
		client.Logf = a.Logf
		a.cdp = client
		a.logf("选择器配置已热重载: %s", prof.Name)
	}
	return changed, nil
}

// WaitForPage 轮询等待目标页面出现（给用户留出登录时间）。
func (a *Agent) WaitForPage(ctx context.Context, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		if a.cdp == nil {
			client, err := cdp.New(a.cfg.Port, a.prof.URLMatch)
			if err != nil {
				return err
			}
			client.Logf = a.Logf
			a.cdp = client
		}
		err := a.cdp.EnsureConn(ctx)
		if err == nil {
			return nil
		}
		if time.Now().After(deadline) {
			return err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}
}

func (a *Agent) readState(ctx context.Context) (*pageState, error) {
	raw, err := a.cdp.Evaluate(ctx, script.StateJS(a.prof))
	if err != nil {
		return nil, err
	}
	// Runtime.evaluate 按值返回的是 JS 字符串表达式结果，需要先解一层引号。
	if len(raw) > 1 && raw[0] == '"' {
		var inner string
		if err := json.Unmarshal(raw, &inner); err == nil {
			raw = json.RawMessage(inner)
		}
	}
	var st pageState
	if err := json.Unmarshal(raw, &st); err != nil {
		return nil, fmt.Errorf("状态 JSON 解析失败: %w (%s)", err, string(raw))
	}
	return &st, nil
}

// Round 发送一条消息并等待本轮回复完成，返回助手回复文本。
// 完成判定 = 回复文本稳定 stable_window 毫秒，且发送按钮不在"停止"状态；
// 页面不支持停止状态检测时，稳定窗口加倍作为补偿。
func (a *Agent) Round(ctx context.Context, msg string, files []string) (string, error) {
	if err := a.cdp.EnsureConn(ctx); err != nil {
		return "", err
	}
	if _, _, err := a.src.ReloadIfChanged(); err != nil {
		a.logf("选择器配置重载失败(沿用旧配置): %v", err)
	}
	a.applyOverrides()

	if err := a.cdp.BringToFront(ctx); err != nil {
		a.logf("页面置前失败(继续): %v", err)
	}

	st0, err := a.readState(ctx)
	if err != nil {
		return "", fmt.Errorf("读取页面状态失败: %w", err)
	}

	if err := a.waitReady(ctx, st0); err != nil {
		return "", err
	}

	if len(files) > 0 {
		abs := files
		if err := a.cdp.SetInputFiles(ctx, abs); err != nil {
			return "", err
		}
		a.logf("已挂载 %d 个文件，等待页面处理 %dms", len(files), a.cfg.UploadSettleMS)
		time.Sleep(time.Duration(a.cfg.UploadSettleMS) * time.Millisecond)
	}

	if a.cfg.ExtraDelayMaxS > 0 {
		lo, hi := a.cfg.ExtraDelayMinS, a.cfg.ExtraDelayMaxS
		if hi < lo {
			lo, hi = hi, lo
		}
		d := lo + rand.Intn(hi-lo+1)
		a.logf("防风控延迟 %ds", d)
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(time.Duration(d) * time.Second):
		}
	}

	raw, err := a.cdp.Evaluate(ctx, script.SendJS(a.prof, msg))
	if err != nil {
		return "", fmt.Errorf("注入消息失败: %w", err)
	}
	var sendRes struct {
		Ok  bool   `json:"ok"`
		Err string `json:"err"`
		Via string `json:"via"`
	}
	if len(raw) > 1 && raw[0] == '"' {
		var inner string
		if err := json.Unmarshal(raw, &inner); err == nil {
			raw = json.RawMessage(inner)
		}
	}
	_ = json.Unmarshal(raw, &sendRes)
	if !sendRes.Ok {
		return "", fmt.Errorf("页面拒绝发送: %s", sendRes.Err)
	}
	a.logf("消息已发送(via %s, %d 字)", sendRes.Via, len([]rune(msg)))

	poll := time.Duration(a.effPoll()) * time.Millisecond
	stableNeed := time.Duration(a.effStable()) * time.Millisecond
	deadline := time.Now().Add(time.Duration(a.cfg.SendTimeoutS) * time.Second)

	sawNew := false
	prev := st0.Last
	stable := time.Duration(0)
	nulls := 0
	lastWasStop := false
	lastProgress := time.Now()

	for {
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		if time.Now().After(deadline) {
			if sawNew {
				a.logf("等待超时，返回已收到的部分回复")
				return prev, fmt.Errorf("等待回复超时(已返回部分内容)")
			}
			return "", fmt.Errorf("等待回复超时(%ds)", a.cfg.SendTimeoutS)
		}
		time.Sleep(poll)

		st, err := a.readState(ctx)
		if err != nil {
			nulls++
			if nulls == 2 {
				a.logf("CDP 连续取空，尝试重连…")
				a.cdp.Invalidate()
			}
			if nulls >= 6 {
				return "", fmt.Errorf("CDP 通道连续无响应: %w", err)
			}
			continue
		}
		nulls = 0

		if !sawNew {
			// 内容比对为主：DeepSeek 长对话会虚拟化回收旧消息，
			// 助手消息数量可能不增反减，不能只依赖计数。
			if st.Last != "" && (st.Last != st0.Last || st.ACount != st0.ACount) {
				a.logf("发现新回复(len=%d)", len([]rune(st.Last)))
				sawNew = true
				prev = st.Last
				stable = 0
			}
			continue
		}

		if st.SendState == "stop" {
			// 仍在流式输出，按钮处于停止态：不判定完成。
			if st.Last != prev {
				prev = st.Last
			}
			stable = 0
			lastWasStop = true
			continue
		}
		lastWasStop = false

		if st.Last == prev {
			stable += poll
		} else {
			prev = st.Last
			stable = 0
		}
		if time.Since(lastProgress) > 10*time.Second {
			lastProgress = time.Now()
			a.logf("等待回复中(len=%d, 稳定 %dms, send_state=%s)", len([]rune(prev)), stable.Milliseconds(), st.SendState)
		}
		need := stableNeed
		if st.SendState == "unknown" {
			need *= 2 // 无按钮状态信号时加倍稳定窗口
		}
		if stable >= need && !lastWasStop {
			a.logf("回复完成(len=%d, 稳定 %dms, send_state=%s)", len([]rune(prev)), stable.Milliseconds(), st.SendState)
			return prev, nil
		}
	}
}

// waitReady 在发送前确认页面可用：输入框存在；若未登录则提示并等待用户完成登录。
// 未登录的判定：无输入框时直接视为未就绪（DeepSeek 登录前页面没有输入框）。
func (a *Agent) waitReady(ctx context.Context, st *pageState) error {
	if st.HasInput {
		return nil
	}
	a.logf("页面还没有输入框（可能正在加载或未登录），请在浏览器中完成登录…")
	deadline := time.Now().Add(5 * time.Minute)
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("等待页面就绪超时(5分钟)，请确认已在浏览器中登录")
		}
		time.Sleep(2 * time.Second)
		s, err := a.readState(ctx)
		if err != nil {
			continue
		}
		if s.HasInput {
			a.logf("页面已就绪")
			return nil
		}
	}
}

func (a *Agent) applyOverrides() {
	if a.cfg.PollIntervalMS > 0 {
		a.prof.Behavior.PollIntervalMS = a.cfg.PollIntervalMS
	}
	if a.cfg.StableWindowMS > 0 {
		a.prof.Behavior.StableWindowMS = a.cfg.StableWindowMS
	}
}

func (a *Agent) effPoll() int {
	if a.prof.Behavior.PollIntervalMS > 0 {
		return a.prof.Behavior.PollIntervalMS
	}
	return 120
}

func (a *Agent) effStable() int {
	if a.prof.Behavior.StableWindowMS > 0 {
		return a.prof.Behavior.StableWindowMS
	}
	return 2500
}
