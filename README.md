<div align="center">

# ✦ spell

**An AI command palette for your terminal.**
Type what you want. Cast a spell. Run the command.

[![License: MIT](https://img.shields.io/badge/license-MIT-a78bfa.svg)](LICENSE)
[![Go Report](https://goreportcard.com/badge/github.com/rockscy/spell)](https://goreportcard.com/report/github.com/rockscy/spell)
[![Release](https://img.shields.io/github/v/release/rockscy/spell?color=a78bfa)](https://github.com/rockscy/spell/releases)

<img src="docs/demo.gif" alt="spell demo" width="720"/>

</div>

---

## What it is

`spell` is a single-binary TUI that turns plain English (or Chinese, or anything your model speaks) into shell commands. It runs in the terminal you already use, talks to the LLM you already pay for, and stays out of the way.

```
✦ how do I find files larger than 100MB modified in the last week
```

```bash
$ find . -type f -size +100M -mtime -7
```

`enter` to run, `e` to edit first, `c` to copy, `r` to retry. That's the whole thing.

## Why another one

| | spell | `thefuck` | `gh copilot cli` | shell-gpt | Warp |
|---|:-:|:-:|:-:|:-:|:-:|
| Open source | ✅ | ✅ | ❌ | ✅ | partial |
| Local LLMs (Ollama, etc.) | ✅ | ❌ | ❌ | ✅ | ❌ |
| Bring-your-own provider | ✅ any OpenAI-compatible | ❌ | ❌ | ⚠️ | ❌ |
| Single binary, no runtime | ✅ Go | ❌ Python | ⚠️ needs `gh` | ❌ Python | ❌ GUI |
| TUI preview before running | ✅ | ⚠️ | ❌ | ❌ | ✅ |
| Works on top of your shell | ✅ | ✅ | ✅ | ✅ | ❌ replaces it |

## Install

### Homebrew (macOS / Linux)

```sh
brew install rockscy/tap/spell
```

### `go install`

```sh
go install github.com/rockscy/spell/cmd/spell@latest
```

### From a release binary

Grab the right archive from [Releases](https://github.com/rockscy/spell/releases) and put `spell` on your `$PATH`.

### From source

```sh
git clone https://github.com/rockscy/spell && cd spell
make build && ./bin/spell --help
```

## Configure

On first run, `spell` writes a starter config:

```sh
spell --init
$EDITOR "$(spell --where)"
```

A `~/.config/spell/config.toml` looks like this — pick the provider you want as `default`, fill in **your own** API key (env var refs are expanded):

```toml
default = "openai"

[providers.openai]
type     = "openai-compatible"
base_url = "https://api.openai.com/v1"
api_key  = "$OPENAI_API_KEY"
model    = "gpt-4o-mini"

[providers.anthropic]
type     = "anthropic"
api_key  = "$ANTHROPIC_API_KEY"
model    = "claude-haiku-4-5-20251001"

[providers.ollama]
type     = "openai-compatible"
base_url = "http://localhost:11434/v1"
api_key  = "ollama"
model    = "llama3.2"
```

### Already works with

Anything that speaks **OpenAI-compatible** chat completions OR **Anthropic Messages**. That includes — but is not limited to:

OpenAI · Anthropic · **Ollama** · DeepSeek · Moonshot/Kimi · Zhipu · Qwen/DashScope · Doubao · Groq · Together · Fireworks · OpenRouter · vLLM · LM Studio · LocalAI · Xiaomi · Baichuan · 01.AI · MiniMax · …

If your provider has an OpenAI-compatible endpoint, you set `base_url` and you're done.

## Usage

```sh
spell                                  # interactive TUI
spell list all docker containers        # pre-fill the prompt
spell -p ollama "compress this folder"  # pick a provider for one shot
```

### Recommended shell function

`spell` execs the chosen command in a fresh shell, which means env changes (like `cd`) won't survive. To run the suggestion **inside your current shell**, drop this in your `~/.zshrc` / `~/.bashrc`:

```sh
sp() {
  local cmd
  cmd="$(SPELL_PRINT=1 spell "$@")" || return $?
  [ -n "$cmd" ] && print -s -- "$cmd" && eval "$cmd"
}
```

Then `sp "make me a python venv called spike"` and the `cd` sticks.

## Keys

| key | does |
|---|---|
| `enter` (input) | submit query |
| `enter` (result) | run the command |
| `e` | edit the suggestion |
| `c` / `y` | copy to clipboard |
| `r` | regenerate |
| `esc` | cancel / quit |

## History

Every cast is appended to `~/.local/share/spell/history.jsonl` so you can grep your past commands. One line, one cast:

```json
{"ts":"2026-04-30T18:42:01Z","provider":"ollama","query":"…","command":"…","explain":"…"}
```

## Privacy

`spell` only talks to the provider URL you configure. No telemetry, no analytics, no calls home. The binary is pure Go with `net/http` — `strings(spell) | grep -i posthog` will turn up nothing.

## Build from source

```sh
make build       # ./bin/spell
make install     # to $GOPATH/bin
make run         # build + run
make release-snap # local goreleaser dry run
```

## Roadmap

- [ ] Shell hook: auto-explain failed commands (`command_not_found_handler`)
- [ ] `spell explain <cmd>` — reverse mode, what does this command do
- [ ] Inline completion via `widget::accept-line`
- [ ] Tools / function calling for safer multi-step plans
- [ ] More built-in provider presets
- [ ] Theming via `LIPGLOSS_NO_COLOR` and a `[theme]` block

## Contributing

PRs welcome — please keep the binary small, dependencies few, and the TUI snappy. See [CONTRIBUTING.md](CONTRIBUTING.md).

## License

[MIT](LICENSE) © 2026 — built with [Bubble Tea](https://github.com/charmbracelet/bubbletea), [Lip Gloss](https://github.com/charmbracelet/lipgloss), and a healthy disrespect for sluggish Electron apps.
