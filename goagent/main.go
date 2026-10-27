// eagent-cli 是 EAgent 的 Go 命令行重构版：
// CDP 直连浏览器 + 可插拔 DOM 选择器 + 工具白名单/超时/截断。
package main

import (
	"bufio"
	"context"
	_ "embed"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"eagent-cli/internal/agent"
	"eagent-cli/internal/cdp"
	"eagent-cli/internal/config"
	"eagent-cli/internal/selectors"
)

//go:embed internal/mock/mock.html
var mockHTML []byte

func main() {
	logf := func(format string, args ...any) {
		fmt.Fprintf(os.Stderr, "[%s] %s\n", time.Now().Format("15:04:05"), fmt.Sprintf(format, args...))
	}
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	var err error
	switch os.Args[1] {
	case "chat":
		err = cmdChat(os.Args[2:], logf)
	case "send":
		err = cmdSend(os.Args[2:], logf)
	case "doctor":
		err = cmdDoctor(os.Args[2:], logf)
	case "selectors":
		err = cmdSelectors(os.Args[2:])
	case "mock":
		err = cmdMock(os.Args[2:])
	default:
		usage()
		os.Exit(2)
	}
	if err != nil {
		logf("错误: %v", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Print(`EAgent-CLI —— 让 DeepSeek 网页版操控电脑 (Go 重构版)

用法:
  eagent-cli chat    [flags]         交互模式：启动浏览器、上传系统提示词、进入对话循环
  eagent-cli send    [flags]         单任务模式：发送一条消息并自动驱动工具循环直到完成
  eagent-cli doctor  [flags]         体检：浏览器路径/调试端口/目标页面/选择器逐项检查
  eagent-cli selectors <子命令>       选择器配置管理，见下方
  eagent-cli mock    [flags]         启动本地模拟聊天页，用于端到端测试

通用 flags:
  --config <file>     配置文件路径
  --workdir <dir>     工作目录(默认当前目录)
  --port <n>          浏览器调试端口(默认 9222)
  --browser <path>    浏览器可执行文件路径
  --selectors <src>   builtin | 文件路径 | http(s) URL
  --exec-policy <p>   auto | ask | deny (默认 ask)
  --prompt-file <f>   自定义系统提示词文件(chat 模式)
  --shell <path>      sh 工具使用的 bash 路径(默认 PATH 中的 bash)
  --url <u>           浏览器打开的页面 URL(默认 DeepSeek)

selectors 子命令:
  show                打印内置的 DeepSeek 选择器配置
  check <file>        校验一个选择器配置文件是否合法
  pull <url> [--out <file>]   从远程拉取配置(自动缓存，断网可用缓存)

chat 交互命令:
  /file <路径>...     下一轮消息挂载附件
  /auto [on|off|deny] 切换工具执行策略(无参数则在 ask/auto 间切换)
  /reload             热重载选择器配置
  /msg                进入多行输入模式，单独一行 . 结束
  /quit               退出

示例:
  eagent-cli chat
  eagent-cli chat --workdir E:/tmp --exec-policy auto
  eagent-cli send -m "查看系统内存占用" --workdir E:/tmp
  eagent-cli send -m "分析 data.csv" -f data.csv
  eagent-cli selectors pull https://example.com/deepseek.json --out deepseek.json
`)
}

type commonFlags struct {
	fs         *flag.FlagSet
	config     string
	workdir    string
	port       int
	browser    string
	selectors  string
	policy     string
	promptFile string
	shell      string
	url        string
}

func newCommon(name string) *commonFlags {
	c := &commonFlags{fs: flag.NewFlagSet(name, flag.ExitOnError)}
	c.fs.StringVar(&c.config, "config", "", "配置文件路径")
	c.fs.StringVar(&c.workdir, "workdir", "", "工作目录")
	c.fs.IntVar(&c.port, "port", 0, "调试端口")
	c.fs.StringVar(&c.browser, "browser", "", "浏览器路径")
	c.fs.StringVar(&c.selectors, "selectors", "", "选择器来源: builtin|文件|URL")
	c.fs.StringVar(&c.policy, "exec-policy", "", "auto|ask|deny")
	c.fs.StringVar(&c.promptFile, "prompt-file", "", "自定义系统提示词文件")
	c.fs.StringVar(&c.shell, "shell", "", "sh 工具的 bash 路径")
	c.fs.StringVar(&c.url, "url", "", "浏览器打开的页面 URL")
	return c
}

func (c *commonFlags) build(logf func(string, ...any)) (*config.Config, *agent.Agent, error) {
	cfg, err := config.Load(c.config)
	if err != nil {
		return nil, nil, err
	}
	if c.workdir != "" {
		cfg.WorkDir = c.workdir
	}
	if cfg.WorkDir == "" {
		if cfg.WorkDir, err = os.Getwd(); err != nil {
			return nil, nil, err
		}
	}
	if c.port > 0 {
		cfg.Port = c.port
	}
	if c.browser != "" {
		cfg.BrowserPath = c.browser
	}
	if c.selectors != "" {
		cfg.Selectors = c.selectors
	}
	if c.policy != "" {
		cfg.ExecPolicy = c.policy
	}
	if c.promptFile != "" {
		cfg.PromptFile = c.promptFile
	}
	if c.shell != "" {
		cfg.ShellPath = c.shell
	}
	if c.url != "" {
		cfg.URL = c.url
	}

	src, err := selectors.Open(cfg.Selectors)
	if err != nil {
		return nil, nil, err
	}
	tmpDir, err := os.MkdirTemp("", "eagent-cli-*")
	if err != nil {
		return nil, nil, err
	}
	a, err := agent.New(cfg, src, tmpDir, logf)
	if err != nil {
		return nil, nil, err
	}
	return cfg, a, nil
}

// prepareBrowser 启动(或复用)调试浏览器并等待目标页面就绪。
func prepareBrowser(ctx context.Context, cfg *config.Config, a *agent.Agent, logf func(string, ...any)) error {
	if !cdp.PortAlive(cfg.Port) {
		browserPath := cfg.BrowserPath
		if browserPath == "" {
			bp, err := cdp.FindBrowserPath("")
			if err != nil {
				return err
			}
			browserPath = bp
		}
		logf("启动浏览器 %s (端口 %d)…", browserPath, cfg.Port)
		if err := cdp.Launch(ctx, cfg.BrowserPath, cfg.URL, cfg.Port, cfg.UserDataDir); err != nil {
			return err
		}
	} else {
		logf("复用已运行的调试浏览器(端口 %d)", cfg.Port)
	}
	logf("等待页面就绪(首次使用请在浏览器中完成登录)…")
	return a.WaitForPage(ctx, 5*time.Minute)
}

func makeEngine(a *agent.Agent, in *bufio.Reader) *agent.Engine {
	return &agent.Engine{
		A: a,
		Ask: func(tc agent.ToolCall) bool {
			fmt.Fprintf(os.Stderr, "\n===== AI 请求执行工具 [%s] =====\n%s\n=============================\n允许执行? (y/N) ", tc.Name, tc.Body)
			line, _ := in.ReadString('\n')
			line = strings.TrimSpace(strings.ToLower(line))
			return line == "y" || line == "yes"
		},
	}
}

func cmdChat(args []string, logf func(string, ...any)) error {
	c := newCommon("chat")
	noPrompt := c.fs.Bool("no-prompt", false, "跳过初始系统提示词上传")
	if err := c.fs.Parse(args); err != nil {
		return err
	}
	cfg, a, err := c.build(logf)
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := prepareBrowser(ctx, cfg, a, logf); err != nil {
		return err
	}

	in := bufio.NewReader(os.Stdin)
	eng := makeEngine(a, in)

	if !*noPrompt {
		prompt := agent.RenderPrompt(cfg.WorkDir, a.TmpDir)
		if cfg.PromptFile != "" {
			data, err := os.ReadFile(cfg.PromptFile)
			if err != nil {
				return err
			}
			prompt = string(data)
		}
		sysFile := filepath.Join(a.TmpDir, "system.txt")
		if err := os.WriteFile(sysFile, []byte(prompt), 0o644); err != nil {
			return err
		}
		logf("上传系统提示词 (%s)…", sysFile)
		if _, err := a.RunTask(ctx, eng, "根据 system.txt 执行", []string{sysFile}); err != nil {
			return err
		}
		logf("系统提示词已就绪。输入消息开始对话；/file <路径> 挂附件，/auto 设置自动执行，/quit 退出。")
	}

	var pendingFiles []string
	for {
		fmt.Print("\neagent> ")
		line, err := in.ReadString('\n')
		if err != nil {
			return nil
		}
		line = strings.TrimSpace(line)
		switch {
		case line == "":
			continue
		case line == "/quit" || line == "/exit":
			return nil
		case strings.HasPrefix(line, "/file "):
			paths := strings.Fields(strings.TrimPrefix(line, "/file "))
			for _, p := range paths {
				if abs, err := filepath.Abs(p); err == nil {
					pendingFiles = append(pendingFiles, abs)
				}
			}
			fmt.Fprintf(os.Stderr, "已挂载 %d 个文件: %v\n", len(pendingFiles), pendingFiles)
			continue
		case line == "/reload":
			changed, err := a.ReloadSelectors()
			fmt.Fprintf(os.Stderr, "选择器配置: 变更=%v 错误=%v\n", changed, err)
			continue
		case line == "/auto" || strings.HasPrefix(line, "/auto "):
			arg := strings.TrimSpace(strings.TrimPrefix(line, "/auto"))
			switch arg {
			case "on", "auto":
				cfg.ExecPolicy = "auto"
			case "off", "ask":
				cfg.ExecPolicy = "ask"
			case "deny":
				cfg.ExecPolicy = "deny"
			default: // 无参数：ask <-> auto 切换
				if cfg.ExecPolicy == "auto" {
					cfg.ExecPolicy = "ask"
				} else {
					cfg.ExecPolicy = "auto"
				}
			}
			fmt.Fprintf(os.Stderr, "执行策略: %s\n", cfg.ExecPolicy)
			continue
		case strings.HasPrefix(line, "/msg"):
			// 多行输入，单独一行 "." 结束
			var lines []string
			for {
				ml, _ := in.ReadString('\n')
				ml = strings.TrimRight(ml, "\r\n")
				if ml == "." {
					break
				}
				lines = append(lines, ml)
			}
			line = strings.Join(lines, "\n")
			if line == "" {
				continue
			}
		}
		files := pendingFiles
		pendingFiles = nil
		reply, err := a.RunTask(ctx, eng, line, files)
		if err != nil {
			logf("任务中止: %v", err)
			continue
		}
		fmt.Println("\n----- AI 回复 -----")
		fmt.Println(reply)
	}
}

func cmdSend(args []string, logf func(string, ...any)) error {
	c := newCommon("send")
	msg := c.fs.String("m", "", "要发送的消息(必填)")
	fileList := c.fs.String("f", "", "逗号分隔的附件路径")
	launch := c.fs.Bool("launch", false, "浏览器未运行时自动启动")
	if err := c.fs.Parse(args); err != nil {
		return err
	}
	if *msg == "" {
		return fmt.Errorf("send 需要 -m 消息内容")
	}
	cfg, a, err := c.build(logf)
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if *launch || cdp.PortAlive(cfg.Port) {
		if err := prepareBrowser(ctx, cfg, a, logf); err != nil {
			return err
		}
	} else {
		logf("使用已运行的调试浏览器(端口 %d)", cfg.Port)
		if err := a.WaitForPage(ctx, 30*time.Second); err != nil {
			return err
		}
	}
	var files []string
	for _, p := range strings.Split(*fileList, ",") {
		if p = strings.TrimSpace(p); p != "" {
			abs, err := filepath.Abs(p)
			if err != nil {
				return err
			}
			files = append(files, abs)
		}
	}
	in := bufio.NewReader(os.Stdin)
	reply, err := a.RunTask(ctx, makeEngine(a, in), *msg, files)
	if err != nil {
		return err
	}
	fmt.Println(reply)
	return nil
}

func cmdDoctor(args []string, logf func(string, ...any)) error {
	c := newCommon("doctor")
	if err := c.fs.Parse(args); err != nil {
		return err
	}
	cfg, a, err := c.build(logf)
	if err != nil {
		return err
	}
	fmt.Printf("选择器配置: %s (来源 %s)\n", a.Profile().Name, cfg.Selectors)
	if browser, err := cdp.FindBrowserPath(cfg.BrowserPath); err == nil {
		fmt.Printf("浏览器: %s\n", browser)
	} else {
		fmt.Printf("浏览器: 未找到 (%v)\n", err)
	}
	if cdp.PortAlive(cfg.Port) {
		fmt.Printf("调试端口 %d: 在线\n", cfg.Port)
		targets, err := a.Client().ListTargets()
		if err != nil {
			return err
		}
		found := false
		for _, t := range targets {
			if t.Type == "page" {
				matched := a.Client().MatchTarget(t.URL)
				fmt.Printf("  页面: %s%s\n", t.URL, map[bool]string{true: "  [匹配]", false: ""}[matched])
				found = found || matched
			}
		}
		if !found {
			return fmt.Errorf("没有匹配的页面，请用 --url 打开正确页面")
		}
	} else {
		fmt.Printf("调试端口 %d: 未运行 (chat/send 会自动启动)\n", cfg.Port)
	}
	fmt.Println("体检完成。")
	return nil
}

func cmdSelectors(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("用法: selectors show | check <file> | pull <url> [--out <file>]")
	}
	switch args[0] {
	case "show":
		p, err := selectors.Builtin()
		if err != nil {
			return err
		}
		return json.NewEncoder(os.Stdout).Encode(p)
	case "check":
		if len(args) < 2 {
			return fmt.Errorf("check 需要文件路径")
		}
		src, err := selectors.Open(args[1])
		if err != nil {
			return err
		}
		p, err := src.Load()
		if err != nil {
			return err
		}
		fmt.Printf("配置有效: %s | 选择器组: ", p.Name)
		for k, v := range p.Selectors {
			fmt.Printf("%s=%d条 ", k, len(v))
		}
		fmt.Println()
		return nil
	case "pull":
		if len(args) < 2 {
			return fmt.Errorf("pull 需要 URL")
		}
		src, err := selectors.Open(args[1])
		if err != nil {
			return err
		}
		p, err := src.Load()
		if err != nil {
			return err
		}
		data, _ := json.MarshalIndent(p, "", "  ")
		out := ""
		if len(args) >= 4 && args[2] == "--out" {
			out = args[3]
		}
		if out != "" {
			return os.WriteFile(out, data, 0o644)
		}
		fmt.Println(string(data))
		return nil
	}
	return fmt.Errorf("未知子命令: %s", args[0])
}

func cmdMock(args []string) error {
	addr := ":8787"
	fs := flag.NewFlagSet("mock", flag.ExitOnError)
	fs.StringVar(&addr, "addr", addr, "监听地址")
	if err := fs.Parse(args); err != nil {
		return err
	}
	http.HandleFunc("/mock.html", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(mockHTML)
	})
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/mock.html", http.StatusFound)
	})
	host := addr
	if strings.HasPrefix(addr, ":") {
		host = "127.0.0.1" + addr
	}
	fmt.Printf("mock 页面: http://%s/mock.html\n", host)
	return http.ListenAndServe(addr, nil)
}
