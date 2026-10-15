# EAgent 
0 tokens、0 门槛 | 基于 DeepSeek 网页版制作的轻量级 Agent —— 让 DeepSeek "操控"你的电脑。

## 环境要求
- Edge 浏览器(电脑自带)
- Win10/11 `理论上支持所有 Win，但我没测试过喵`


## 起源——为了省钱
- 先说一下网络上的项目：
    - `DeepSeek Harness` 等正经的 Agent：它们的功能都比 EAgent 强大 100 倍！**但它们都要 API-Key、都要花钱买 token**
    - `网页反代` 等同类非正经的 Agent：都通过反代等技术手段实现，功能也很强大，但他们封号概率很大！...本项目核心是自动化模拟操控浏览器、比反代更安全、封号概率更小 (但绝不为 0 .因为均违反了使用协议) (目前没有遇到过 `因为使用 EAgent 导致被封禁/限制` 的话题.)

- 再看我的情况：
    - 用 `正经的 Agent` 嘛...我是个穷B、又不搞啥企业级 Agent 工作、也不专业搞计算机，没必要买 token ，而且这个蓝色大肥鱼有时候他妈的会吃白饭，思考陷入死循环了一直吃干我的 token ，白瞎了十多块钱。所以我一直用免费的 DeepSeek 网页版。
    - 用 `非正经的 Agent` 嘛...我又怕反代把我号封了。那我自己搓一个呗...

- 是个硅基生物都知道：DeepSeek 网页无法直接编辑电脑文件。最多ai生成代码、然后复制粘贴，非常麻烦，它不能"操控"我的电脑。
- 我想，直接让ai生成 PowerShell 代码、执行、结果扔回去，不就好了吗？卧槽对啊，于是就有了这个项目。
- 本项目能让 DeepSeek 网页版：编辑电脑文件、能看电脑屏幕、能控制你的键盘和鼠标、能编辑 Excel/Word ！...还能更多，正在持续开发中...


## 快速使用
- 下载文件：[点我下载](https://github.com/YaoGaoChaoLe/EAgent/releases)
- 运行 `EAgentWeb.exe`
- 进入程序，点击 `选择工作目录` 按钮，选择一个目录。
- 然后会自动打开 Edge 浏览器，请自行登录后，才能进行下一步操作。
- 点击 `开始新对话` 。等待 Ai 分析提示词...
- 到你回复它了 —— 使用 EAgent 发送信息，而**不是用网页版发送信息**。 `网页版的唯一作用就是给你看的，而不是发消息的。`


<img width="1061" height="761" alt="1" src="https://github.com/user-attachments/assets/5e6db9a2-9301-44a0-aa83-3d288646df36" />

---

<img width="1965" height="806" alt="df1d401c-527e-41bb-81a3-8fcd8b257d95" src="https://github.com/user-attachments/assets/a08c8307-d83b-4cea-8f8a-df4388310b9c" />

---

<img width="1962" height="929" alt="QQ20261012-222024" src="https://github.com/user-attachments/assets/3ab39b76-3e14-4581-9a67-3665773bc9f3" />

  

## 实现逻辑
- EAgent **不是反代**、没第三方中转、没逆向 DeepSeek 网页！核心原理是：自动化模拟用户复制粘贴等操作，比反代安全一点，但都违反 [DeepSeek 用户协议](https://cdn.deepseek.com/policies/zh-CN/deepseek-terms-of-use.html)。除非你一直用 EAgent 累计写个几百万行代码，个人简单折腾玩一下没问题。
- 以下是AI对本项目 (EAgent) 原理的总结：
```
用 PowerShell 连接 Chrome/Edge 的远程调试端口，
通过 CDP WebSocket 在真实 DeepSeek 页面里执行 JavaScript，
模拟输入、上传文件、点击发送，并轮询 DOM 判断助手回复是否完成，最后返回助手回复信息。
```

## 适用场景
- ~~个人工具、学习交流、拿来瞎折腾、协助开发...~~
- 这是我 `自己为自己定制` 的项目。仅供学习交流，请于 24 小时内删除，严禁用于商业用途，否则后果自负。


## 声明
- 你不可以：极端高频使用、规模滥用、用于生产环境、没改提示词情况下指望生成优质作品、倒卖盈利、仅改几个字/几乎无改动就称是自己的项目...
- 安全警告：本项目未做安全措施，DeepSeek 可能会生成恶意代码 > 再通过 EAgent 破坏你的系统；本项目在某些 `极端高频使用` 情况下会被 DeepSeek 限制/封禁，作者不承担任何责任。（其实这句话是我瞎几把猜的，我用这么久也没听过 `因为用 EAgent 导致账号封禁/限制` 的话题。）
- 本程序内置skill大多来源于网络，侵删请联系。(issues)
- 本项目由 `e语言` 编写，由于其一言难尽的狗屎编译器，会让部分杀毒软件会误报为恶意程序。但我保证：**本项目自身完全不会带有任何恶意代码，请放心使用。**[微步云-样本分析报告.pdf](https://github.com/user-attachments/files/33344596/_3ef75307f364aac0ab0212977530d167eadcc090084ce65829f8e4678737dcf7.pdf)
- 本项目创建于 2026 年 10 月 8 日，目前处于开发阶段。遵循 `GPL-3.0` 协议，开源仓库 https://github.com/YaoGaoChaoLe/EAgent/
