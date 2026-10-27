# EAgent-CLI 详细使用说明

## 一、安装与首次运行

### 1. 编译（或直接用已编译的 eagent-cli.exe）

```
cd goagent
go build -o eagent-cli.exe .
```

### 2. 首次运行

```
./eagent-cli.exe chat
```

启动流程：

1. 程序自动找 Edge/Chrome，以调试模式启动（端口 9222），打开 DeepSeek。
2. **切到浏览器窗口，完成 DeepSeek 登录**（手机号/微信/邮箱均可）。登录态保存在独立 profile（`%TEMP%\eagent-cli-profile`），以后不用再登录。
3. CLI 每秒检测一次页面，发现输入框出现即自动继续。
4. 自动上传系统提示词（告诉 AI 工具协议和行为规范）。
5. 看到 `eagent>` 提示符，开始对话。

> 如果浏览器已经在跑（比如上次没关），程序会直接复用，不会重复启动。

## 二、日常使用

### 交互模式（推荐）

```
./eagent-cli.exe chat --workdir E:/tmp --exec-policy auto
```

- `--workdir`：AI 的工作目录，读写文件默认在这里
- `--exec-policy auto`：AI 执行命令不再逐条询问（谨慎使用）

对话示例：

```
eagent> 帮我看看 E:/tmp 下有哪些超过 100MB 的文件
eagent> 把结果写个 markdown 保存
eagent> /quit
```

AI 需要执行命令时会自动调用 `ps`/`sh` 工具，终端会显示：

```
===== AI 请求执行工具 [ps] =====
Get-ChildItem E:\tmp | ...
=============================
允许执行? (y/N) y
```

### 单任务模式

不进 REPL，跑完一个任务就退出：

```
./eagent-cli.exe send -m "统计当前目录代码行数" --workdir E:/code
./eagent-cli.exe send -m "分析这份日志" -f app.log --exec-policy auto
```

### 常用参数

| 参数 | 说明 | 默认 |
|---|---|---|
| `--workdir <dir>` | AI 工作目录 | 当前目录 |
| `--port <n>` | 调试端口 | 9222 |
| `--browser <path>` | 浏览器路径 | 自动找 |
| `--exec-policy <p>` | auto / ask / deny | ask |
| `--selectors <src>` | 选择器来源 | builtin |
| `--prompt-file <f>` | 自定义系统提示词 | 内置 |
| `--config <file>` | 配置文件路径 | 自动查找 |
| `--shell <path>` | bash 路径 | PATH 里找 |

## 三、交互命令

在 `eagent>` 提示符下可用：

| 命令 | 说明 |
|---|---|
| `/file E:\a.pdf E:\b.png` | 给下一条消息挂附件（支持多个） |
| `/auto` | ask ↔ auto 切换；`/auto deny` 全拒绝 |
| `/reload` | 重新加载选择器配置（改了 JSON 不用重启） |
| `/msg` | 多行输入模式；单独一行输入 `.` 表示结束 |
| `/quit` 或 `/exit` | 退出 |

`/msg` 示例：

```
eagent> /msg
...第一行
...第二行
...  ← 这里输入 . 后回车
```

## 四、AI 能做什么

| 工具 | 用途 | 示例指令 |
|---|---|---|
| `ps` | PowerShell | "列出 F 盘最大的 10 个文件" |
| `sh` | Git Bash | "用 rg 在代码里搜 xxx" |
| `getfile` | 传文件给 AI | "读一下 config.json 的内容" |
| `screenshot` | 全屏截图 | "看看我屏幕上是什么" |
| `mouse` | 模拟点击 | "帮我点一下(500,300)" |
| `topmost` | 窗口置顶 | "把记事本窗口置顶" |

典型工作流：

```
你: 分析 E:/logs/app.log 里的错误模式，写个报告
AI: (调用 sh: rg -i "error" E:/logs/app.log | head -50)
AI: (调用 sh: 统计各类错误数量)
AI: (调用 getfile: 读取报告草稿)
AI: 已生成 E:/tmp/报告.md，要点是……
```

## 五、执行策略与安全

| 策略 | 行为 |
|---|---|
| `ask`（默认） | 每次执行命令前终端确认 |
| `auto` | 直接执行，不询问 |
| `deny` | 拒绝所有工具调用 |

切换方式：启动参数 `--exec-policy`、配置文件 `exec_policy`、会话内 `/auto`。

白名单（只允许特定命令）——在配置文件里：

```json
{
  "exec_policy": "auto",
  "exec_allow": ["^Get-", "^rg ", "^ls ", "^cat "]
}
```

代码必须匹配至少一条正则才会执行，否则回传"被白名单拦截"。

其他保护：

- 每条命令有超时（`exec_timeout_s`，默认 120 秒）
- 输出超过 `max_inline_chars`（默认 1800 字符）自动转附件，避免刷屏
- 连续工具调用超过 `max_tool_rounds`（默认 20 轮）强制中止，防死循环

## 六、配置文件

程序按顺序找：`--config` 指定 → `./eagent.config.json` → `~/.eagent-cli/config.json`。找到第一个就用，没有全用默认值。

所有字段和默认值见 README「配置文件」一节。改完立即生效（下次启动读取），无需编译。

## 七、DeepSeek 网页改版了怎么办

选择器是开放配置，三种修法（按推荐顺序）：

1. **本地文件热重载**：把内置配置导出，改选择器，用 `--selectors` 指定。改完不用重启，下一轮自动生效：
   ```
   ./eagent-cli.exe selectors show > my.json   # 导出
   # 编辑 my.json 里的 selectors
   ./eagent-cli.exe chat --selectors my.json
   ```
2. **远程配置**：把新 JSON 放到 gist/服务器，`--selectors https://...` 拉取（自动缓存）。
3. **JS 完全自定义**：如果连交互流程都变了，用 `js_overrides.get_state` / `js_overrides.send` 整体替换注入脚本。

校验配置：`./eagent-cli.exe selectors check my.json`

## 八、常见问题

**Q: 提示"页面还没有输入框，请完成登录"**
→ 切到浏览器登录 DeepSeek，登录后自动继续。等 5 分钟超时则重跑。

**Q: 浏览器没弹出来？**
→ 检查是否已有别的调试浏览器占了 9222 端口：`./eagent-cli.exe doctor`。

**Q: AI 只说不做？**
→ 大概率是旧会话。刷新 DeepSeek 页面开新对话（新系统提示词才会生效），或重跑 `chat`。

**Q: 提示词放在哪？**
→ 内嵌在 exe 里，源码 `internal/agent/system_prompt.md`。用 `--prompt-file my.txt` 换成自己的。

**Q: 能用 Git Bash / rg / fd 吗？**
→ 能。AI 有 `sh` 工具走 bash，提示词已让它优先用 rg。确认 `bash` 在 PATH 里，或 `--shell "C:\Program Files\Git\bin\bash.exe"`。

**Q: 想换别的浏览器/端口？**
→ `--browser "C:\...\chrome.exe" --port 9223`。

**Q: 长时间没反应？**
→ 终端每 10 秒会打一行"等待回复中"进度。网页端如果显示已输出完但 CLI 还在等，是稳定窗口太保守，可调 `stable_window_ms`（默认 2500）。

**Q: 封号风险？**
→ 本项目通过 CDP 自动化操作 DeepSeek 网页，违反其用户协议。比反代安全（模拟真人输入、独立 profile、可选随机延迟），但极端高频使用仍可能被限制。自担风险。

## 九、开发者

```
go build -o eagent-cli.exe .   # 编译
go test ./...                  # 单测
go vet ./...                   # 静态检查
```

端到端测试（不依赖 DeepSeek）：

```
./eagent-cli.exe mock -addr 127.0.0.1:8787 &
msedge --remote-debugging-port=9223 --user-data-dir=%TEMP%\t1 --app=http://127.0.0.1:8787/mock.html &
./eagent-cli.exe send -m "run: 42" --selectors testdata/mock-selectors.json --port 9223
```

mock 页面会模拟 AI 返回工具调用，验证完整循环。
