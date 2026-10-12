# DeepSeek EAgent Web
0配置、0门槛、0tokens。基于deepseek-网页版制作的简易Agent，不需要APIkey，实现tokens自由。

## 环境要求
- Edge浏览器(电脑自带)
- Win10/11 `更早的win版本我没测试过，我不知道喵。`

## 起源——为了省钱
- 市面上常见的Agent，它们的功能都比我这个项目强大100倍！**但它们都需要ApiKey、都需要花钱买tokens**。
- 但我是个穷B，只能用免费的deepseek网页版，我又不是搞什么企业级Agent工作任务，我难道还要花几毛钱的tokens跟ai说"你好"、花几块钱让ai帮我写个文章？哎呀用网页端得了。
- 但是，网页ai无法直接编辑我的电脑文件，开发中，只能让ai生成代码，然后复制粘贴。那我直接让ai生成PowerShell代码，直接执行，扔结果给它，不就好了吗？卧槽对啊，于是就有了这个项目：自动复制粘贴、自动执行PowerShell代码、实现token自由!

## 快速使用
- 下载文件：https://github.com/YaoGaoChaoLe/EAgentWeb/releases
- 解压、运行`EAgentWeb.exe`
- 进入程序，点击`选择工作目录`按钮，选择一个目录。

<img width="1061" height="761" alt="1" src="https://github.com/user-attachments/assets/5e6db9a2-9301-44a0-aa83-3d288646df36" />

- 会自动打开edge浏览器，请自行登录后，才能进行下一步操作。
- 点击`开始新对话`。

 <img width="1965" height="806" alt="df1d401c-527e-41bb-81a3-8fcd8b257d95" src="https://github.com/user-attachments/assets/a08c8307-d83b-4cea-8f8a-df4388310b9c" />

<img width="1962" height="929" alt="QQ20261012-222024" src="https://github.com/user-attachments/assets/3ab39b76-3e14-4581-9a67-3665773bc9f3" />

  

## 实现逻辑
- 本项目**不是反代**，没有逆向deepseek网页。
- 用人话来讲，底层逻辑就是：通过edge浏览器开发者模式，调用复制粘贴，实现`工具-模型`交互，完成Agent循环。
- 以下是AI对本项目原理的总结：
```DeepSeek_text
用 PowerShell 连接 Chrome/Edge 的远程调试端口，
通过 CDP WebSocket 在真实 DeepSeek 页面里执行 JavaScript，
模拟输入、上传文件、点击发送，并轮询 DOM 判断助手回复是否稳定，最后返回助手回复信息。
```

## 适用场景
- 个人工具、学习交流、拿来玩、协助开发、节省在网页端复制粘贴ai代码的流程...

## 声明
- 本项目不适用：商业、盈利、生产环境、指望生成优质作品、大规模滥用...
- 安全警告：本项目不论在工具设计上还是提示词上，没有做任何安全防御。提示词注入攻击等，作者不承担任何责任。
- 本项目由`E语言`编写，由于其一言难尽的狗屎编译器，会让部分杀毒软件会误报为恶意程序。但我保证：本项目自身完全不会带有任何恶意代码，请放心使用。
- 本项目从2026年10月8日从0开始创建，正在处于开发阶段。开源仓库 https://github.com/YaoGaoChaoLe/EAgentWeb/
