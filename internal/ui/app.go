package ui

import (
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"runtime"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	lg "github.com/charmbracelet/lipgloss"

	"github.com/rockscy/spell/internal/history"
	"github.com/rockscy/spell/internal/llm"
)

// ---------- styles ----------

var (
	colAccent = lg.Color("#a78bfa") // violet — "spell"
	colDim    = lg.Color("#6b7280")
	colOk     = lg.Color("#10b981")
	colErr    = lg.Color("#ef4444")
	colCmd    = lg.Color("#fbbf24") // amber for command

	titleStyle = lg.NewStyle().Foreground(colAccent).Bold(true)
	tagStyle   = lg.NewStyle().Foreground(colAccent).Background(lg.Color("#1f1b2e")).
			Padding(0, 1).MarginRight(1)
	dimStyle = lg.NewStyle().Foreground(colDim)
	cmdBox   = lg.NewStyle().
			Border(lg.RoundedBorder()).
			BorderForeground(colCmd).
			Padding(0, 2).
			MarginTop(1)
	streamBox = lg.NewStyle().
			Border(lg.RoundedBorder()).
			BorderForeground(colDim).
			Foreground(colDim).
			Padding(0, 2).
			MarginTop(1)
	errBox = lg.NewStyle().
		Border(lg.RoundedBorder()).
		BorderForeground(colErr).
		Foreground(colErr).
		Padding(0, 2).
		MarginTop(1)
	cmdText  = lg.NewStyle().Foreground(colCmd).Bold(true)
	okText   = lg.NewStyle().Foreground(colOk)
	keyHint  = lg.NewStyle().Foreground(colAccent).Bold(true)
	footerSt = lg.NewStyle().Foreground(colDim).MarginTop(1)
)

// ---------- model ----------

type state int

const (
	stInput state = iota
	stStream
	stResult
	stErr
)

// Result is what the program produces when the user picks an action.
type Result struct {
	Command string
	Action  Action
}

type Action int

const (
	ActionNone Action = iota
	ActionRun
	ActionCopy
	ActionEdit
	ActionAbort
)

type chunkMsg llm.Chunk
type doneMsg struct{}
type errMsg struct{ err error }

type Model struct {
	provider     llm.Provider
	providerName string
	platform     string
	shell        string

	state         state
	input         textinput.Model
	spin          spinner.Model
	rawText       string // model "answer" content — used for command parsing
	reasoningText string // chain-of-thought, shown dimmed during streaming only
	command       string
	explain       string
	errStr        string
	width         int
	height        int
	chunkCh       <-chan llm.Chunk
	cancel        context.CancelFunc
	finished      Result
}

// New constructs an interactive Model.
func New(p llm.Provider, providerName, initialQuery string) Model {
	in := textinput.New()
	in.Prompt = "✦ "
	in.Placeholder = "describe what you want to do…"
	in.CharLimit = 500
	in.Focus()
	in.PromptStyle = lg.NewStyle().Foreground(colAccent).Bold(true)
	in.TextStyle = lg.NewStyle()
	in.Cursor.Style = lg.NewStyle().Foreground(colAccent)
	if initialQuery != "" {
		in.SetValue(initialQuery)
	}

	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lg.NewStyle().Foreground(colAccent)

	return Model{
		provider:     p,
		providerName: providerName,
		platform:     runtime.GOOS,
		shell:        detectShell(),
		state:        stInput,
		input:        in,
		spin:         sp,
	}
}

// Finished reports the user's choice once the program quits.
func (m Model) Finished() Result { return m.finished }

func (m Model) Init() tea.Cmd { return textinput.Blink }

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			if m.cancel != nil {
				m.cancel()
			}
			m.finished = Result{Action: ActionAbort}
			return m, tea.Quit
		}
		return m.handleKey(msg)

	case chunkMsg:
		if msg.Err != nil {
			m.state = stErr
			m.errStr = msg.Err.Error()
			return m, nil
		}
		if msg.Reasoning {
			m.reasoningText += msg.Delta
		} else {
			m.rawText += msg.Delta
		}
		return m, waitForChunk(m.chunkCh)

	case doneMsg:
		cmd, explain := parseResponse(m.rawText)
		if cmd == "" {
			m.state = stErr
			m.errStr = "model returned no executable command"
			return m, nil
		}
		m.command = cmd
		m.explain = explain
		m.state = stResult
		_ = history.Append(history.Entry{
			Provider: m.providerName,
			Query:    m.input.Value(),
			Command:  cmd,
			Explain:  explain,
		})
		return m, nil

	case errMsg:
		m.state = stErr
		m.errStr = msg.err.Error()
		return m, nil

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spin, cmd = m.spin.Update(msg)
		return m, cmd
	}

	if m.state == stInput {
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch m.state {
	case stInput:
		switch msg.String() {
		case "enter":
			q := strings.TrimSpace(m.input.Value())
			if q == "" {
				return m, nil
			}
			return m.startStream(q)
		case "esc":
			m.finished = Result{Action: ActionAbort}
			return m, tea.Quit
		}
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		return m, cmd

	case stStream:
		if msg.String() == "esc" {
			if m.cancel != nil {
				m.cancel()
			}
			m.state = stInput
			m.rawText = ""
			m.reasoningText = ""
			return m, textinput.Blink
		}

	case stResult:
		switch msg.String() {
		case "enter":
			m.finished = Result{Command: m.command, Action: ActionRun}
			return m, tea.Quit
		case "c", "y":
			_ = copyToClipboard(m.command)
			m.finished = Result{Command: m.command, Action: ActionCopy}
			return m, tea.Quit
		case "e":
			m.input.SetValue(m.command)
			m.input.CursorEnd()
			m.state = stInput
			m.rawText = ""
			m.reasoningText = ""
			return m, textinput.Blink
		case "esc", "q":
			m.finished = Result{Action: ActionAbort}
			return m, tea.Quit
		case "r":
			// regenerate
			return m.startStream(m.input.Value())
		}

	case stErr:
		switch msg.String() {
		case "esc", "q":
			m.finished = Result{Action: ActionAbort}
			return m, tea.Quit
		case "enter":
			m.state = stInput
			m.errStr = ""
			m.rawText = ""
			return m, textinput.Blink
		}
	}
	return m, nil
}

func (m Model) startStream(q string) (tea.Model, tea.Cmd) {
	m.state = stStream
	m.rawText = ""
	m.reasoningText = ""
	m.command = ""
	m.explain = ""

	ctx, cancel := context.WithCancel(context.Background())
	m.cancel = cancel

	sys := buildSystemPrompt(m.platform, m.shell)
	ch, err := m.provider.Stream(ctx, sys, q)
	if err != nil {
		cancel()
		m.state = stErr
		m.errStr = err.Error()
		return m, nil
	}
	m.chunkCh = ch
	return m, tea.Batch(m.spin.Tick, waitForChunk(ch))
}

func waitForChunk(ch <-chan llm.Chunk) tea.Cmd {
	return func() tea.Msg {
		c, ok := <-ch
		if !ok {
			return doneMsg{}
		}
		return chunkMsg(c)
	}
}

// ---------- view ----------

func (m Model) View() string {
	var b strings.Builder

	// header
	b.WriteString(titleStyle.Render("✦ spell"))
	b.WriteString("  ")
	b.WriteString(tagStyle.Render(m.providerName))
	b.WriteString(dimStyle.Render(fmt.Sprintf("%s · %s", m.platform, m.shell)))
	b.WriteString("\n\n")

	// input always visible
	b.WriteString(m.input.View())
	b.WriteString("\n")

	switch m.state {
	case stStream:
		head := fmt.Sprintf("%s thinking…", m.spin.View())
		// Prefer the answer text when it has started arriving;
		// otherwise show the model's reasoning so the user knows
		// something is happening.
		var body string
		switch {
		case m.rawText != "":
			body = truncate(m.rawText, m.width, 6)
		case m.reasoningText != "":
			body = dimStyle.Italic(true).Render(truncate(m.reasoningText, m.width, 6))
		default:
			body = dimStyle.Render("(waiting for first token)")
		}
		b.WriteString(streamBox.Render(head + "\n" + body))
		b.WriteString("\n")
		b.WriteString(footerSt.Render(keyHint.Render("esc") + dimStyle.Render(" cancel")))

	case stResult:
		body := cmdText.Render("$ "+m.command) + "\n"
		if m.explain != "" {
			body += "\n" + dimStyle.Render(m.explain)
		}
		b.WriteString(cmdBox.Render(body))
		b.WriteString("\n")
		b.WriteString(footerSt.Render(strings.Join([]string{
			keyHint.Render("enter") + dimStyle.Render(" run"),
			keyHint.Render("e") + dimStyle.Render(" edit"),
			keyHint.Render("c") + dimStyle.Render(" copy"),
			keyHint.Render("r") + dimStyle.Render(" retry"),
			keyHint.Render("esc") + dimStyle.Render(" cancel"),
		}, "  ")))

	case stErr:
		b.WriteString(errBox.Render("✗ " + m.errStr))
		b.WriteString("\n")
		b.WriteString(footerSt.Render(
			keyHint.Render("enter") + dimStyle.Render(" retry  ") +
				keyHint.Render("esc") + dimStyle.Render(" quit"),
		))

	default: // stInput
		b.WriteString(footerSt.Render(
			keyHint.Render("enter") + dimStyle.Render(" cast  ") +
				keyHint.Render("esc") + dimStyle.Render(" quit"),
		))
	}
	return b.String()
}

// ---------- helpers ----------

var fenceRe = regexp.MustCompile("(?s)```[a-zA-Z]*\\n?(.+?)```")

func parseResponse(raw string) (cmd, explain string) {
	m := fenceRe.FindStringSubmatch(raw)
	if len(m) > 1 {
		cmd = strings.TrimSpace(m[1])
	}
	if cmd == "" {
		// fallback: first non-empty, non-comment line
		for _, line := range strings.Split(raw, "\n") {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "//") {
				continue
			}
			line = strings.Trim(line, "`")
			cmd = line
			break
		}
	}
	if idx := strings.LastIndex(raw, "```"); idx >= 0 && idx+3 <= len(raw) {
		rest := strings.TrimSpace(raw[idx+3:])
		if i := strings.Index(rest, "\n\n"); i > 0 {
			rest = rest[:i]
		}
		explain = rest
	} else if cmd != "" {
		// no fence: drop the command line and use the rest
		explain = strings.TrimSpace(strings.Replace(raw, cmd, "", 1))
	}
	if len(explain) > 240 {
		explain = explain[:240] + "…"
	}
	return cmd, explain
}

func truncate(s string, width, lines int) string {
	if width <= 0 {
		width = 80
	}
	all := strings.Split(s, "\n")
	if len(all) > lines {
		all = all[len(all)-lines:]
	}
	for i, ln := range all {
		if lg.Width(ln) > width-6 && width > 10 {
			all[i] = ln[:max(0, len(ln)-(lg.Width(ln)-(width-6)))] + "…"
		}
	}
	return strings.Join(all, "\n")
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func copyToClipboard(s string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("pbcopy")
	case "linux":
		if _, err := exec.LookPath("wl-copy"); err == nil {
			cmd = exec.Command("wl-copy")
		} else {
			cmd = exec.Command("xclip", "-selection", "clipboard")
		}
	case "windows":
		cmd = exec.Command("clip")
	default:
		return fmt.Errorf("unsupported platform")
	}
	cmd.Stdin = strings.NewReader(s)
	return cmd.Run()
}

func detectShell() string {
	sh := strings.ToLower(strings.TrimSpace(getenv("SHELL")))
	if sh == "" {
		if runtime.GOOS == "windows" {
			return "powershell"
		}
		return "sh"
	}
	if i := strings.LastIndex(sh, "/"); i >= 0 {
		sh = sh[i+1:]
	}
	return sh
}

// indirection so the package compiles without importing os everywhere
func getenv(k string) string {
	return osGetenv(k)
}

func buildSystemPrompt(platform, shell string) string {
	return fmt.Sprintf(`You translate a user's natural-language request into ONE shell command for their terminal.

OUTPUT FORMAT — strict:
1. A single fenced code block (`+"```bash"+` or `+"```sh"+`) containing only the command. One logical line. No leading "$" prompt.
2. Immediately after the closing fence, ONE short sentence (max 25 words) explaining what the command does.

RULES:
- Prefer the simplest correct command for the user's platform.
- Use POSIX/coreutils when portable; reach for platform-specific tools only when needed.
- If the request is destructive (rm -rf, dd, mkfs, etc.) still output the command but warn briefly in the explanation.
- No preamble, no caveats, no markdown headings, no bullet points.

ENVIRONMENT:
- Platform: %s
- Shell: %s`, platform, shell)
}
