# DeepSeek EAgent Web
让 DeepSeek网页版 "操控" 你的电脑 | 基于`DeepSeek网页版`制作的简易Agent，不需要APIkey，实现tokens自由 | 0配置、0门槛、0 tokens

## 环境要求
- Edge浏览器(电脑自带)
- Win10/11 `更早的 Win版本 我没测试过，我不知道喵。`

## 起源——为了省钱
- 市面上常见的Agent，它们的功能都比我这个项目强大100倍！**但它们都需要ApiKey、都需要花钱买tokens**。
- 但我是个穷B、我又不搞啥企业级Agent工作,没必要花钱买tokens。所以只能用免费的DeepSeek网页版。
- 但是，DeepSeek网页 无法直接编辑我的电脑文件。开发时，只能让ai生成代码，然后复制粘贴，非常麻烦，它不能"操控"我的电脑。
- 我想，直接让ai生成PowerShell代码、执行、结果扔回去，不就好了吗？卧槽对啊，于是就有了这个项目。
- deepseek网页版 能编辑电脑文件、能看到你的电脑屏幕、控制你的键盘和鼠标！...还能更多，正在持续开发中...

## 快速使用
- 下载文件：[点我下载](https://github.com/YaoGaoChaoLe/EAgentWeb/releases)
- 运行`EAgentWeb.exe`
- 进入程序，点击`选择工作目录`按钮，选择一个目录。
- 然后会自动打开edge浏览器，请自行登录后，才能进行下一步操作。
- 点击`开始新对话`。等待ai分析提示词...
- 到你回复ai了！使用工具发送信息，而**不是用网页版发送信息**。`网页版的唯一作用就是给你看的，而不是发消息的。`


<img width="1061" height="761" alt="1" src="https://github.com/user-attachments/assets/5e6db9a2-9301-44a0-aa83-3d288646df36" />

---

<img width="1965" height="806" alt="df1d401c-527e-41bb-81a3-8fcd8b257d95" src="https://github.com/user-attachments/assets/a08c8307-d83b-4cea-8f8a-df4388310b9c" />

---

<img width="1962" height="929" alt="QQ20261012-222024" src="https://github.com/user-attachments/assets/3ab39b76-3e14-4581-9a67-3665773bc9f3" />

  

## 实现逻辑
- 本项目**不是反代**，没有逆向deepseek网页。
- 以下是AI对本项目原理的总结：
```
用 PowerShell 连接 Chrome/Edge 的远程调试端口，
通过 CDP WebSocket 在真实 DeepSeek 页面里执行 JavaScript，
模拟输入、上传文件、点击发送，并轮询 DOM 判断助手回复是否稳定，最后返回助手回复信息。
```
- 用人话来讲，底层逻辑就是：通过edge浏览器开发者模式，调用复制粘贴，实现`工具-模型`交互，完成Agent循环。

## 适用场景
- ~~个人工具、学习交流、拿来玩、协助开发、节省在网页端复制粘贴ai代码的流程...~~
- 本项目仅供学习交流，请于 24 小时内删除，严禁用于商业用途，否则后果自负。

## 声明
- 你不能将本项目：仅改几个字/几乎无改动就称是自己的项目、商业、盈利、生产环境、指望生成优质作品、大规模滥用...
- 安全警告：本项目未对Ai代码做安全措施，Ai生成的代码可能会通过本项目破坏你的系统，作者不承担任何责任。
- 本程序内置skill大多来源于网络，侵删请联系。(issues)
- 本项目由`E语言`编写，由于其一言难尽的狗屎编译器，会让部分杀毒软件会误报为恶意程序。但我保证：**本项目自身完全不会带有任何恶意代码，请放心使用。**[微步云-样本分析报告.pdf](https://github.com/user-attachments/files/33344596/_3ef75307f364aac0ab0212977530d167eadcc090084ce65829f8e4678737dcf7.pdf)

- 本项目创建于 2026 年 10 月 8 日，目前处于开发阶段。开源仓库 https://github.com/YaoGaoChaoLe/EAgentWeb/
