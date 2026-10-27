# EAgent-CLI (goagent)

让 DeepSeek 网页版"操控"你的电脑：不花钱、不要 API Key——通过 Chrome DevTools 协议(CDP)驱动本机 Edge/Chrome，把 DeepSeek 网页版变成一个能执行命令、读文件、截屏、控制鼠标的 Agent。

> 本项目是 [EAgent](https://github.com/JayVae/EAgent)（易语言原版 [YaoGaoChaoLe/EAgent](https://github.com/YaoGaoChaoLe/EAgent)）的 Go 命令行重构版，本仓库只包含 Go 版本源码。

不花钱、不要 API Key——通过 Chrome DevTools 协议(CDP)驱动本机 Edge/Chrome，把 DeepSeek 网页版变成一个能执行命令、读文件、截屏、控制鼠标的 Agent。

相对易语言原版的重构改进：

- CDP 直连（chromedp 同级协议实现），不再经过 PowerShell 脚本中转
- 多信号回复完成判定（文本稳定 + 发送/停止按钮图标状态），避免截断流式回复
- 工具调用有白名单、超时、确认机制（原版直接执行）
- DOM 选择器开放为 JSON 配置：内置/本地文件(热重载)/远程 URL 三种来源，网页改版只改配置
- 不再全局 taskkill 杀 PowerShell，进程管理按需启动

> ⚠️ 免责声明：本项目通过自动化操作 DeepSeek 网页，违反 DeepSeek 用户协议，有封号风险（理论上比反代安全，但概率不为 0）。AI 生成的命令默认逐条询问确认（`--exec-policy ask`），全自动模式请自行评估风险。

## 环境要求

- Windows 10/11
- Edge（系统自带）或 Chrome
- PowerShell 5.1+（ps 工具）；Git Bash（sh 工具，可选）

## 快速开始

```
go build -o eagent-cli.exe .
./eagent-cli.exe chat
```

首次运行会自动以调试模式启动 Edge 打开 DeepSeek，**在浏览器里完成登录**（只需一次，登录态保存在独立 profile），CLI 检测到输入框出现后自动上传系统提示词，看到 `eagent>` 提示符即可对话。

## 命令一览

| 命令 | 说明 |
|---|---|
| `chat` | 交互模式：启动浏览器 + 系统提示词 + REPL |
| `send` | 单任务模式：发一条消息，自动驱动工具循环到任务完成 |
| `doctor` | 体检：浏览器 / 调试端口 / 目标页面 / 选择器 |
| `selectors show` | 打印内置 DeepSeek 选择器配置 |
| `selectors check <file>` | 校验选择器配置文件 |
| `selectors pull <url> [--out <f>]` | 拉取远程配置（带缓存） |
| `mock` | 启动本地模拟聊天页，端到端测试用 |

## 使用示例

```
# 交互模式，工作目录 E:/tmp，命令自动执行
./eagent-cli.exe chat --workdir E:/tmp --exec-policy auto

# 单任务
./eagent-cli.exe send -m "帮我查看系统内存占用并总结" --workdir E:/tmp

# 带附件
./eagent-cli.exe send -m "分析这个文件" -f data.csv

# 指定调试端口和自定义选择器
./eagent-cli.exe chat --port 9223 --selectors my-deepseek.json
```

## chat 交互命令

| 命令 | 说明 |
|---|---|
| `/file <路径>...` | 下一轮消息挂载附件（可多个） |
| `/auto` | 切换 ask/auto 执行策略；`/auto on|off|deny` 指定 |
| `/reload` | 热重载选择器配置 |
| `/msg` | 多行输入，单独一行 `.` 结束 |
| `/quit` | 退出 |

## 工具协议

AI 在回复中用标记调用工具，程序解析执行后把结果回传，循环直到 AI 给出最终结论：

```
### EAGENT_TOOL ps BEGIN
Get-Process | Select-Object -First 5
### EAGENT_TOOL END
```

可用工具：

| 工具 | 说明 |
|---|---|
| `ps` | 执行 PowerShell 脚本（多行） |
| `sh` | 执行 bash 命令（Git Bash；支持 rg/fd/git 等） |
| `getfile` | 上传本地文件给 AI（每行一个绝对路径） |
| `screenshot` | 全屏截图并上传 |
| `mouse` | `click|rclick|move <x> <y>`，物理像素坐标 |
| `topmost` | `hwnd <句柄> on|off` 窗口置顶 |

同时兼容易语言原版的 `# EAgent_ps_识别头...` 旧协议。

安全机制：

- `--exec-policy ask`（默认）：每次执行前终端确认
- `--exec-policy auto`：全自动（配合 `/auto` 随时切换）
- `--exec-policy deny`：所有工具调用都拒绝
- 配置文件 `exec_allow`：正则白名单，代码必须匹配才放行
- 执行超时 `exec_timeout_s`（默认 120s）；输出超长自动转附件上传

## 配置文件

查找顺序：`--config` 指定路径 → 工作目录 `eagent.config.json` → `~/.eagent-cli/config.json`。都不存在用默认值。

完整示例：

```json
{
  "port": 9222,
  "browser_path": "",
  "url": "https://chat.deepseek.com",
  "selectors": "builtin",
  "send_timeout_s": 300,
  "poll_interval_ms": 120,
  "stable_window_ms": 2500,
  "exec_policy": "ask",
  "exec_timeout_s": 120,
  "exec_allow": ["Get-Process.*", "Get-CimInstance.*"],
  "max_inline_chars": 1800,
  "max_tool_rounds": 20,
  "extra_delay_min_s": 0,
  "extra_delay_max_s": 0,
  "shell_path": "bash",
  "powershell_path": "powershell",
  "prompt_file": "",
  "work_dir": "E:/tmp"
}
```

`extra_delay_*`：每次发送前的随机延迟（秒），防风控用，CLI 默认 0。

## DOM 选择器开放接口

DeepSeek 网页改版时，无需改代码，提供新 JSON 即可。三种来源：

1. `builtin`：程序内置（`internal/selectors/deepseek.json`）
2. 文件：`--selectors path/to/config.json`，**每轮发送前自动检测变更热重载**
3. URL：`--selectors https://.../deepseek.json`，自动缓存到本地（断网用缓存）

配置结构：

```json
{
  "name": "deepseek-web",
  "url_match": "chat\\.deepseek\\.com",
  "selectors": {
    "assistant_message": [".ds-assistant-message-main-content"],
    "user_message": ["[class*='fbb737a4']"],
    "input": ["textarea"],
    "send_button": ["div[role='button'].ds-button--primary..."],
    "attach_button": []
  },
  "stop_icon_path_prefix": "M2 4.88C2 3.68009",
  "behavior": {
    "send_mode": "auto",
    "poll_interval_ms": 120,
    "stable_window_ms": 2500,
    "send_button_x_max_ratio": 0.5,
    "send_button_y_min_ratio": 0.6
  },
  "js_overrides": {
    "get_state": "(function(){ ... return JSON.stringify({...}) })()",
    "send": "(function(){ ... })()"
  }
}
```

- `js_overrides.get_state` / `js_overrides.send`：完全替换生成的注入 JS，返回约定 JSON——彻底的自定义通道，适配任何聊天网页。
- `send_mode`：`auto`（文本走回车，纯文件走按钮）/ `enter` / `click`。

校验：`./eagent-cli.exe selectors check my.json`

## 开发

```
go build -o eagent-cli.exe .
go test ./...
go vet ./...

# 端到端测试
./eagent-cli.exe mock -addr 127.0.0.1:8787 &
msedge --remote-debugging-port=9223 --user-data-dir=%TEMP%\t1 --app=http://127.0.0.1:8787/mock.html &
./eagent-cli.exe send -m "run: 42" --selectors testdata/mock-selectors.json --port 9223
```

源码结构：

```
main.go               CLI 入口与命令分发
internal/agent/       状态机、工具解析执行、系统提示词
internal/cdp/         CDP WebSocket 客户端 + 浏览器启动
internal/selectors/   开放选择器接口 + 内置配置
internal/script/      页面注入 JS 生成
internal/tools/       截屏/鼠标/置顶 (Windows syscall)
internal/config/      配置加载
internal/mock/        测试用模拟聊天页
```

## License

GPL-3.0（继承原项目）
