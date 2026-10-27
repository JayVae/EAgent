// Package config 提供 CLI 配置加载，所有字段都有可用默认值。
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type Config struct {
	Port         int    `json:"port"`
	BrowserPath  string `json:"browser_path"`
	UserDataDir  string `json:"user_data_dir"`
	URL          string `json:"url"`
	Selectors    string `json:"selectors"` // builtin | 文件路径 | URL
	SendTimeoutS int    `json:"send_timeout_s"`
	// 覆盖选择器配置里的节奏参数；0 表示使用选择器配置的值。
	PollIntervalMS int `json:"poll_interval_ms"`
	StableWindowMS int `json:"stable_window_ms"`
	// 上传文件后等待页面处理的时间。
	UploadSettleMS int `json:"upload_settle_ms"`
	// 每次发送前的随机延迟(防风控)，CLI 默认关闭。
	ExtraDelayMinS int    `json:"extra_delay_min_s"`
	ExtraDelayMaxS int    `json:"extra_delay_max_s"`
	ExecPolicy     string `json:"exec_policy"` // auto | ask | deny
	ExecTimeoutS   int    `json:"exec_timeout_s"`
	// ExecAllow 非空时，PowerShell 代码必须匹配其中至少一条正则才允许执行。
	ExecAllow      []string `json:"exec_allow"`
	MaxInlineChars int      `json:"max_inline_chars"`
	PowerShellPath string   `json:"powershell_path"`
	// ShellPath 是 sh 工具使用的 bash 路径，默认 "bash"(需在 PATH 中)。
	ShellPath      string   `json:"shell_path"`
	PromptFile     string   `json:"prompt_file"`
	// 每个任务最多连续执行的轮数(防止工具循环失控)。
	MaxToolRounds int    `json:"max_tool_rounds"`
	WorkDir       string `json:"work_dir"`
}

func Defaults() *Config {
	return &Config{
		Port:           9222,
		URL:            "https://chat.deepseek.com",
		Selectors:      "builtin",
		SendTimeoutS:   300,
		UploadSettleMS: 3000,
		ExecPolicy:     "ask",
		ExecTimeoutS:   120,
		MaxInlineChars: 1800,
		MaxToolRounds:  20,
	}
}

// Load 依次尝试 显式路径 → 工作目录/eagent.config.json → 用户目录/eagent-cli/config.json。
// 都不存在时返回默认值。文件里缺省的字段保持默认值。
func Load(explicit string) (*Config, error) {
	cfg := Defaults()
	paths := []string{}
	if explicit != "" {
		paths = append(paths, explicit)
	} else {
		wd, _ := os.Getwd()
		paths = append(paths, filepath.Join(wd, "eagent.config.json"))
		if home, err := os.UserHomeDir(); err == nil {
			paths = append(paths, filepath.Join(home, ".eagent-cli", "config.json"))
		}
	}
	for _, p := range paths {
		data, err := os.ReadFile(p)
		if err != nil {
			if explicit == p {
				return nil, err
			}
			continue
		}
		if err := json.Unmarshal(data, cfg); err != nil {
			return nil, fmt.Errorf("%s: %w", p, err)
		}
		break
	}
	return cfg, nil
}
