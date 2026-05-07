// Package explain implements the reverse direction of spell:
// take a shell command and stream a plain-language explanation,
// or — when invoked from a shell `command_not_found` hook — guess
// what the user actually meant and suggest the corrected command.
//
// Unlike the main interactive flow, explain has no TUI, no
// confirmation step, and no execution: it streams to stdout and
// exits. That keeps it cheap to call from shell hooks and pipelines.
package explain

import (
	"context"
	"fmt"
	"io"
	"os"
	"runtime"
	"strings"

	"github.com/rockscy/spell/internal/llm"
)

// Mode picks the system prompt + output framing.
type Mode int

const (
	// ModeExplain: user invoked `spell explain <cmd>` to read about
	// a command. Plain prose, no decorations.
	ModeExplain Mode = iota
	// ModeHook: shell's command_not_found_handle{,r} fired and
	// handed us the offending input. We show "Did you mean: ..."
	// then a short note. Goes to stderr and exits 127 so the
	// caller's shell sees a normal "not found" failure.
	ModeHook
)

// Run streams the model's response to stdout (ModeExplain) or stderr
// (ModeHook). It returns the exit code the caller should propagate.
func Run(ctx context.Context, p llm.Provider, mode Mode, command string) int {
	command = strings.TrimSpace(command)
	if command == "" {
		fmt.Fprintln(os.Stderr, "spell explain: no command given")
		return 2
	}

	system := buildSystemPrompt(mode)
	out := io.Writer(os.Stdout)
	if mode == ModeHook {
		out = os.Stderr
		fmt.Fprintln(out, "spell ✦", command)
	}

	// errCode is what we return on internal failures. In hook mode
	// the user's typed command was already a "not found" — preserve
	// that semantics regardless of whether spell itself failed.
	errCode := 1
	if mode == ModeHook {
		errCode = 127
	}

	ch, err := p.Stream(ctx, system, command)
	if err != nil {
		fmt.Fprintln(os.Stderr, "spell explain:", err)
		return errCode
	}

	wrote := false
	for c := range ch {
		if c.Err != nil {
			fmt.Fprintln(os.Stderr, "\nspell explain:", c.Err)
			return errCode
		}
		// Reasoning_content is internal monologue from o-series /
		// MiMo Pro / DeepSeek-R1 — never useful to the end user
		// in this mode, so swallow it.
		if c.Reasoning {
			continue
		}
		fmt.Fprint(out, c.Delta)
		wrote = true
	}
	if wrote {
		fmt.Fprintln(out)
	}

	if mode == ModeHook {
		// Preserve standard "command not found" exit semantics.
		return 127
	}
	return 0
}

func buildSystemPrompt(mode Mode) string {
	switch mode {
	case ModeHook:
		return fmt.Sprintf(`The user typed something at their terminal that isn't a recognized command.
Your job: in <=3 short lines, suggest the correct command they probably meant.

Output format (strict, no markdown headers, no preamble):

  Did you mean: `+"`<corrected command>`"+`
  <one short sentence describing what it does>

If you genuinely cannot guess (the input is gibberish), say:

  spell: I'm not sure what you meant by that.
  <one short suggestion of what to try>

Platform: %s`, runtime.GOOS)
	default:
		return fmt.Sprintf(`Explain a shell command in plain prose for someone who can read shell but isn't sure what this particular command does.

Rules:
- Open with one sentence summarising the whole command.
- Then walk through each significant flag/argument in order, one short sentence each.
- Mention any caveat or risk briefly if relevant (destructive flags, root requirements, side effects).
- Total length: <=180 words. No markdown headers. No bullet points unless the command genuinely has unrelated parts.

Platform: %s`, runtime.GOOS)
	}
}
