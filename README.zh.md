<div align="center">

# ✦ spell

**终端里的 AI 命令面板。**
说人话，咒一下，命令到手就跑。

[![License: MIT](https://img.shields.io/badge/license-MIT-a78bfa.svg)](LICENSE)
[![Go Report](https://goreportcard.com/badge/github.com/rockscy/spell)](https://goreportcard.com/report/github.com/rockscy/spell)
[![Release](https://img.shields.io/github/v/release/rockscy/spell?color=a78bfa)](https://github.com/rockscy/spell/releases)

[English](README.md) · **简体中文**

<img src="docs/demo.gif" alt="spell demo" width="720"/>

</div>

---

## 这是什么

`spell` 是一个单二进制的终端工具，把你说的人话翻译成 shell 命令。它跑在你已经在用的终端里，调你已经在付费的大模型，不抢戏、不切屏，全程留在你的 scrollback 里。

```
✦ 找出最近一周修改过、超过 100MB 的文件
```

```bash
$ find . -type f -size +100M -mtime -7
```

生成的命令直接落在同一个输入框里，可以马上跑也可以马上改。`enter` 执行、`ctrl+r` 重新生成、`esc` 重来。一来一回，没多余确认页。

## 为什么要再写一个

| | spell | `thefuck` | `gh copilot cli` | shell-gpt | Warp |
|---|:-:|:-:|:-:|:-:|:-:|
| 开源 | ✅ | ✅ | ❌ | ✅ | 部分 |
| 本地大模型（Ollama 等） | ✅ | ❌ | ❌ | ✅ | ❌ |
| 自带 API key | ✅ 任意 OpenAI 兼容 | ❌ | ❌ | ⚠️ | ❌ |
| 单二进制，零依赖 | ✅ Go | ❌ Python | ⚠️ 要 `gh` | ❌ Python | ❌ GUI |
| 命令执行前 TUI 预览 | ✅ | ⚠️ | ❌ | ❌ | ✅ |
| 跑在你现有 shell 上 | ✅ | ✅ | ✅ | ✅ | ❌ 整个替换掉 |

## 安装

### Homebrew（macOS / Linux）

```sh
brew install rockscy/tap/spell
```

### `go install`

```sh
go install github.com/rockscy/spell/cmd/spell@latest
```

### 直接下二进制

去 [Releases](https://github.com/rockscy/spell/releases) 拿对应平台的压缩包，把 `spell` 放到 `$PATH` 里即可。

### 从源码编译

```sh
git clone https://github.com/rockscy/spell && cd spell
make build && ./bin/spell --help
```

## 配置

```sh
spell init
```

完事。`spell init` 走一遍 4 步交互向导 —— **选 provider → 填 API key → 选 model → 设默认** —— 然后写好 config。任何时候重新跑都行，可以加新 provider、改默认、改某个已有的配置。

```
✦ spell init
  Let's get you a provider configured.

? Pick a provider
  ▸ OpenAI
    Anthropic (Claude)
    Xiaomi MiMo (Token Plan, Singapore)
    DeepSeek
    Moonshot / Kimi
    Groq  (fast inference)
    OpenRouter  (one key, every model)
    Ollama  (local, no key needed)
    Custom  (any OpenAI- or Anthropic-compatible endpoint)

? API key for openai
  ●●●●●●●●●●●●  ← 输入框，遮挡的 password mode
  > 粘贴 key 即可；留空则使用 $ENV_VAR 环境变量。

? Model
  ▸ ★ GPT-4o mini  (fast, cheap)
    GPT-4o
    o4-mini  (reasoning)
    Custom (type your own)

? Set "openai" as the default provider?  [Y/n]

✓ Saved to ~/.config/spell/config.toml
```

Reasoning 模型（o-series、MiMo Pro、DeepSeek-R1）会自动给 `max_tokens = 2048`，给思维链 + 最终命令足够预算。

想手动改也行，配置就是普通 TOML：`$(spell --where)`。

### 已经能直接用的厂商

只要厂商提供 **OpenAI-compatible chat completions** 或 **Anthropic Messages** 协议都能用：

OpenAI · Anthropic · **Ollama**（本地）· **DeepSeek** · **Moonshot/Kimi** · **智谱（Zhipu）** · **通义（Qwen/DashScope）** · **豆包（Doubao）** · Groq · Together · Fireworks · OpenRouter · vLLM · LM Studio · LocalAI · **小米 MiMo** · 百川 · 零一万物 · MiniMax · ……

只要你的 provider 有 OpenAI 兼容的 endpoint，填 `base_url` 就能跑。

## 使用

```sh
spell                                   # 打开输入框
spell 列出所有 docker 容器               # 一行命令直接出结果，自动提交不用再敲 enter
spell -p ollama "压缩这个文件夹"         # 单次切换 provider
```

全部 **inline 渲染** —— `spell` 不会霸占你整个终端。命令跑完，输出紧接着写在下面，整段历史都留在 scrollback 里能向上翻。

### 推荐的 shell 函数

`spell` 默认用 `syscall.Exec` 把命令塞进新 shell 跑，意味着 `cd`、`export` 这种修改环境的命令不会保留。如果想让命令**在你当前 shell 里执行**，把这段加到 `~/.zshrc` / `~/.bashrc`：

```sh
sp() {
  local cmd
  cmd="$(SPELL_PRINT=1 spell "$@")" || return $?
  [ -n "$cmd" ] && print -s -- "$cmd" && eval "$cmd"
}
```

然后 `sp "建一个叫 spike 的 python venv"`，`cd` 就会留在当前 shell 里。

## 反向模式：`spell explain`

有时候你不是要生成命令 —— 你已经有一条命令，想知道它在干嘛。直接喂给 `spell explain`：

```sh
spell explain "find . -type f -mtime -7 -size +50M"
```

直接把白话解释流式打到 stdout，没有 TUI、没有二次确认，可以放心 pipe 进别的东西。

### 命令打错时自动解释

`spell install-hook` 会打印一段 zsh / bash 钩子函数。把它追加到 `~/.zshrc`（或 `~/.bashrc`），以后你**敲错命令**时 `spell` 会安静地提示你想敲的可能是什么：

```sh
spell install-hook >> ~/.zshrc
exec zsh
gti staus
# → Did you mean: `git status`
#   Show the working tree status of the current Git repository.
```

钩子**只在命令不存在时触发**，正常命令一律不拦截。不想用了把 rc 文件里那段删掉就行。

## 快捷键

| 按键 | 作用 |
|---|---|
| `enter`（intent 模式） | 把你说的话发给模型 |
| `enter`（command 模式） | 执行生成的命令 |
| `ctrl+r`（command 模式） | 用同样的意图重新生成 |
| `esc`（command 模式） | 推倒重来，回到 intent |
| `esc`（intent 模式） | 退出 |
| `ctrl+c` | 任何阶段都立刻退出 |

命令和 intent 共用同一个输入框 —— 直接打字就能改命令，`enter` 就跑。

## 历史记录

每次施法都追加到 `~/.local/share/spell/history.jsonl`，方便你 grep 之前用过的命令。一行一次：

```json
{"ts":"2026-04-30T18:42:01Z","provider":"mimo","query":"…","command":"…","explain":"…"}
```

## 隐私

`spell` 只跟你配置里的 provider URL 通信。**没有 telemetry、没有 analytics、不偷偷 phone home**。整个 binary 就是纯 Go 加标准库 `net/http` —— `strings(spell) | grep -i posthog` 啥也搜不到。

## 从源码编译

```sh
make build       # 输出到 ./bin/spell
make install     # 装到 $GOPATH/bin
make run         # 编译 + 运行
make release-snap # 本地 goreleaser 模拟一次 release
```

## Roadmap

- [x] Shell hook：命令打错时自动用 AI 解释（`command_not_found_handler`）—— `spell install-hook`（v0.4.0）
- [x] `spell explain <cmd>` —— 反向模式，解释这条命令在干嘛（v0.4.0）
- [ ] 配 zsh `widget::accept-line` 做内联补全
- [ ] Tool calling，做更安全的多步规划
- [ ] 更多内置 provider 预设
- [ ] 主题：支持 `LIPGLOSS_NO_COLOR` 和 `[theme]` 配置块

## 贡献

欢迎 PR —— 请保持二进制小、依赖少、TUI 流畅。详见 [CONTRIBUTING.md](CONTRIBUTING.md)。

## License

[MIT](LICENSE) © 2026 —— 用 [Bubble Tea](https://github.com/charmbracelet/bubbletea) + [Lip Gloss](https://github.com/charmbracelet/lipgloss) 写的，顺便鄙视一下那些又卡又胖的 Electron 应用。
