// Package selectors 实现可插拔的 DOM 选择器配置：
// builtin（内嵌 deepseek.json）/ 本地文件（支持热重载）/ 远程 URL（带本地缓存）。
// 网页改版时只需提供新的 JSON 配置，无需改动程序。
package selectors

import (
	"crypto/sha1"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"time"
)

//go:embed deepseek.json
var builtinJSON []byte

// Behavior 控制发送方式与回复完成判定的节奏。
type Behavior struct {
	SendMode string `json:"send_mode"` // auto | enter | click
	// auto: 非空消息走输入框回车，空消息(仅文件)走按钮点击。
	PollIntervalMS      int     `json:"poll_interval_ms"`
	StableWindowMS      int     `json:"stable_window_ms"`
	SendButtonXMaxRatio float64 `json:"send_button_x_max_ratio"`
	SendButtonYMinRatio float64 `json:"send_button_y_min_ratio"`
}

// Profile 是一份完整的页面适配配置。
type Profile struct {
	Name string `json:"name"`
	// URLMatch 用于从调试端口的目标列表里挑出正确的标签页。
	URLMatch string `json:"url_match"`
	// Selectors 至少需要 assistant_message 与 input；
	// user_message、send_button 可选，缺省时对应功能降级。
	Selectors map[string][]string `json:"selectors"`
	// StopIconPathPrefix 发送按钮 SVG 路径前缀，用于区分"发送/停止"状态。
	StopIconPrefix string   `json:"stop_icon_path_prefix"`
	Behavior       Behavior `json:"behavior"`
	// JsOverrides 可整体替换生成的 JS，键：get_state / send。
	// 两者都必须是完整的 IIFE 表达式，返回约定格式的 JSON 字符串。
	JsOverrides map[string]string `json:"js_overrides"`
}

func (p *Profile) Validate() error {
	if p == nil {
		return errors.New("profile 为空")
	}
	if p.Name == "" {
		return errors.New("缺少 name")
	}
	if _, err := regexp.Compile(p.URLMatch); err != nil {
		return fmt.Errorf("url_match 不是合法正则: %w", err)
	}
	if len(p.Selectors["assistant_message"]) == 0 {
		return errors.New("缺少 selectors.assistant_message")
	}
	if len(p.Selectors["input"]) == 0 {
		return errors.New("缺少 selectors.input")
	}
	if p.Behavior.PollIntervalMS <= 0 {
		p.Behavior.PollIntervalMS = 120
	}
	if p.Behavior.StableWindowMS <= 0 {
		p.Behavior.StableWindowMS = 2500
	}
	switch p.Behavior.SendMode {
	case "", "auto", "enter", "click":
	default:
		return fmt.Errorf("behavior.send_mode 非法: %q (应为 auto|enter|click)", p.Behavior.SendMode)
	}
	for _, key := range []string{"get_state", "send"} {
		if v := p.JsOverrides[key]; v != "" && len(v) < 10 {
			return fmt.Errorf("js_overrides.%s 内容过短", key)
		}
	}
	return nil
}

func Builtin() (*Profile, error) {
	return Parse(builtinJSON)
}

func Parse(data []byte) (*Profile, error) {
	var p Profile
	if err := json.Unmarshal(data, &p); err != nil {
		return nil, fmt.Errorf("解析选择器配置: %w", err)
	}
	if err := p.Validate(); err != nil {
		return nil, err
	}
	return &p, nil
}

// Source 表示一个选择器配置来源，可反复 Load/Reload。
// Kind: builtin | file | url
type Source struct {
	Kind    string
	Locator string // 文件路径或 URL

	cachePath string
	lastRaw   []byte
}

// Open 按字符串构造来源："builtin"、本地路径或 http(s) URL。
func Open(source string) (*Source, error) {
	switch {
	case source == "" || source == "builtin":
		return &Source{Kind: "builtin"}, nil
	case len(source) > 8 && (source[:7] == "http://" || source[:8] == "https://"):
		s := &Source{Kind: "url", Locator: source}
		sum := sha1.Sum([]byte(source))
		if base, err := os.UserCacheDir(); err == nil {
			s.cachePath = filepath.Join(base, "eagent-cli", "selectors-"+hex.EncodeToString(sum[:8])+".json")
		}
		return s, nil
	default:
		return &Source{Kind: "file", Locator: source}, nil
	}
}

func (s *Source) Load() (*Profile, error) {
	switch s.Kind {
	case "builtin":
		return Parse(builtinJSON)
	case "file":
		data, err := os.ReadFile(s.Locator)
		if err != nil {
			return nil, err
		}
		p, err := Parse(data)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", s.Locator, err)
		}
		s.lastRaw = data
		return p, nil
	case "url":
		data, err := fetch(s.Locator)
		if err != nil {
			if s.cachePath != "" {
				if cached, cerr := os.ReadFile(s.cachePath); cerr == nil {
					if p, perr := Parse(cached); perr == nil {
						s.lastRaw = cached
						return p, nil
					}
				}
			}
			return nil, fmt.Errorf("拉取选择器配置失败: %w", err)
		}
		p, err := Parse(data)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", s.Locator, err)
		}
		if s.cachePath != "" {
			_ = os.MkdirAll(filepath.Dir(s.cachePath), 0o755)
			_ = os.WriteFile(s.cachePath, data, 0o644)
		}
		s.lastRaw = data
		return p, nil
	default:
		return nil, fmt.Errorf("未知来源类型: %s", s.Kind)
	}
}

// ReloadIfChanged 文件来源时检查内容是否变化，变化则重新加载；
// 其余来源原样加载。返回配置是否更新。
func (s *Source) ReloadIfChanged() (bool, *Profile, error) {
	if s.Kind != "file" {
		p, err := s.Load()
		return false, p, err
	}
	data, err := os.ReadFile(s.Locator)
	if err != nil {
		return false, nil, err
	}
	if s.lastRaw != nil && string(data) == string(s.lastRaw) {
		return false, nil, nil
	}
	p, err := Parse(data)
	if err != nil {
		return false, nil, fmt.Errorf("%s: %w", s.Locator, err)
	}
	changed := s.lastRaw != nil
	s.lastRaw = data
	return changed, p, nil
}

func fetch(url string) ([]byte, error) {
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return io.ReadAll(io.LimitReader(resp.Body, 1<<20))
}
