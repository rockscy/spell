package ui

import (
	"context"
	"fmt"
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
	colAccent = lg.Color("#a78bfa") // violet — intent / "spell"
	colDim    = lg.Color("#6b7280")
	colErr    = lg.Color("#ef4444")
	colCmd    = lg.Color("#fbbf24") // amber — generated command

	titleStyle = lg.NewStyle().Foreground(colAccent).Bold(true)
	tagStyle   = lg.NewStyle().Foreground(colAccent).Background(lg.Color("#1f1b2e")).
			Padding(0, 1).MarginRight(1)
	dimStyle  = lg.NewStyle().Foreground(colDim)
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
	keyHint  = lg.NewStyle().Foreground(colAccent).Bold(true)
	footerSt = lg.NewStyle().Foreground(colDim).MarginTop(1)

	// Two prompt presets — switching between them is how the input
	// signals "type natural language" vs "you have a command, hit Enter".
	intentPromptStyle  = lg.NewStyle().Foreground(colAccent).Bold(true)
	commandPromptStyle = lg.NewStyle().Foreground(colCmd).Bold(true)
	commandTextStyle   = lg.NewStyle().Foreground(colCmd)
)

// ---------- model ----------

type state int

const (
	stIntent  state = iota // user is typing a natural-language request
	stStream               // waiting for / receiving the model's response
	stCommand              // model returned a command, now editable in the input
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
	ActionAbort
)

type chunkMsg llm.Chunk
type doneMsg struct{}
type errMsg struct{ err error }
type autoSubmitMsg struct{}

type Model struct {
	provider     llm.Provider
	providerName string
	platform     string
	shell        string

	state         state
	input         textinput.Model
	spin          spinner.Model
	intent        string // last-submitted natural-language query (for retry)
	rawText       string // model "answer" stream
	reasoningText string // chain-of-thought stream
	explain       string // parsed one-line explanation
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
	in.CharLimit = 1000
	in.Focus()
	in.PromptStyle = intentPromptStyle
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
		state:        stIntent,
		input:        in,
		spin:         sp,
	}
}

// Finished reports the user's choice once the program quits.
func (m Model) Finished() Result { return m.finished }

func (m Model) Init() tea.Cmd {
	cmds := []tea.Cmd{textinput.Blink}
	// If the program was launched with `spell <query>`, the input is
	// pre-filled — kick off the model call right away so the user
	// doesn't have to press Enter on text they already typed.
	if strings.TrimSpace(m.input.Value()) != "" {
		cmds = append(cmds, func() tea.Msg { return autoSubmitMsg{} })
	}
	return tea.Batch(cmds...)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
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
		m.explain = explain
		m.enterCommandMode(cmd)
		_ = history.Append(history.Entry{
			Provider: m.providerName,
			Query:    m.intent,
			Command:  cmd,
			Explain:  explain,
		})
		return m, textinput.Blink

	case errMsg:
		m.state = stErr
		m.errStr = msg.err.Error()
		return m, nil

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spin, cmd = m.spin.Update(msg)
		return m, cmd

	case autoSubmitMsg:
		q := strings.TrimSpace(m.input.Value())
		if q == "" || m.state != stIntent {
			return m, nil
		}
		m.intent = q
		return m.startStream(q)
	}

	if m.state == stIntent || m.state == stCommand {
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch m.state {
	case stIntent:
		switch msg.String() {
		case "enter":
			q := strings.TrimSpace(m.input.Value())
			if q == "" {
				return m, nil
			}
			m.intent = q
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
			m.enterIntentMode(m.intent)
			return m, textinput.Blink
		}

	case stCommand:
		switch msg.String() {
		case "enter":
			cmd := strings.TrimSpace(m.input.Value())
			if cmd == "" {
				return m, nil
			}
			m.finished = Result{Command: cmd, Action: ActionRun}
			return m, tea.Quit
		case "ctrl+r":
			if m.intent != "" {
				return m.startStream(m.intent)
			}
			return m, nil
		case "esc":
			m.enterIntentMode("")
			return m, textinput.Blink
		}
		// any other key edits the command in place
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		return m, cmd

	case stErr:
		switch msg.String() {
		case "esc", "q":
			m.finished = Result{Action: ActionAbort}
			return m, tea.Quit
		case "enter":
			m.enterIntentMode(m.intent)
			return m, textinput.Blink
		}
	}
	return m, nil
}

func (m Model) startStream(q string) (tea.Model, tea.Cmd) {
	m.state = stStream
	m.rawText = ""
	m.reasoningText = ""
	m.explain = ""
	m.errStr = ""

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

// enterCommandMode swaps the input from intent style to command style
// and pre-fills it with the generated command, ready to edit or run.
func (m *Model) enterCommandMode(cmd string) {
	m.state = stCommand
	m.input.Prompt = "$ "
	m.input.PromptStyle = commandPromptStyle
	m.input.TextStyle = commandTextStyle
	m.input.Cursor.Style = lg.NewStyle().Foreground(colCmd)
	m.input.Placeholder = ""
	m.input.SetValue(cmd)
	m.input.CursorEnd()
}

// enterIntentMode resets the input back to the natural-language prompt.
// preset is what to seed the input with (typically "" or the previous intent).
func (m *Model) enterIntentMode(preset string) {
	m.state = stIntent
	m.input.Prompt = "✦ "
	m.input.PromptStyle = intentPromptStyle
	m.input.TextStyle = lg.NewStyle()
	m.input.Cursor.Style = lg.NewStyle().Foreground(colAccent)
	m.input.Placeholder = "describe what you want to do…"
	m.input.SetValue(preset)
	m.input.CursorEnd()
	m.rawText = ""
	m.reasoningText = ""
	m.explain = ""
	m.errStr = ""
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

	// input is always visible
	b.WriteString(m.input.View())
	b.WriteString("\n")

	switch m.state {
	case stStream:
		head := fmt.Sprintf("%s thinking…", m.spin.View())
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

	case stCommand:
		if m.explain != "" {
			b.WriteString("  ")
			b.WriteString(dimStyle.Render(m.explain))
			b.WriteString("\n")
		}
		b.WriteString(footerSt.Render(strings.Join([]string{
			keyHint.Render("enter") + dimStyle.Render(" run"),
			keyHint.Render("ctrl+r") + dimStyle.Render(" regen"),
			keyHint.Render("esc") + dimStyle.Render(" start over"),
		}, "  ")))

	case stErr:
		b.WriteString(errBox.Render("✗ " + m.errStr))
		b.WriteString("\n")
		b.WriteString(footerSt.Render(
			keyHint.Render("enter") + dimStyle.Render(" retry  ") +
				keyHint.Render("esc") + dimStyle.Render(" quit"),
		))

	default: // stIntent
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

func detectShell() string {
	sh := strings.ToLower(strings.TrimSpace(osGetenv("SHELL")))
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
