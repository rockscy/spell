package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"syscall"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/rockscy/spell/internal/config"
	"github.com/rockscy/spell/internal/explain"
	"github.com/rockscy/spell/internal/setup"
	"github.com/rockscy/spell/internal/ui"
)

var version = "dev"

const usage = `spell — AI command palette for your terminal.

USAGE
  spell init                    interactive provider setup wizard
  spell [flags] [query…]        cast a spell

  spell explain <cmd…>          read a plain-language explanation of a command
  spell install-hook            print a zsh/bash command_not_found hook to install

FLAGS
  -p, --provider NAME           override the default provider
  -c, --config PATH             use a specific config file
      --where                   print resolved config path and exit
      --version                 show version and exit
  -h, --help                    this help

KEYS (interactive cast)
  enter   submit intent / run command
  ctrl+r  regenerate from the same intent
  esc     start over / quit
`

func main() {
	// Subcommand handling — must happen before flag.Parse so the
	// command name is not mistaken for a flag argument.
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "init", "setup":
			os.Args = append(os.Args[:1], os.Args[2:]...)
			runInit()
			return
		case "explain":
			os.Exit(runExplain(os.Args[2:]))
		case "install-hook":
			runInstallHook(os.Args[2:])
			return
		}
	}

	var (
		providerOverride string
		configOverride   string
		doWhere          bool
		doInit           bool
		showVersion      bool
		showHelp         bool
	)
	flag.StringVar(&providerOverride, "p", "", "provider name")
	flag.StringVar(&providerOverride, "provider", "", "provider name")
	flag.StringVar(&configOverride, "c", "", "config path")
	flag.StringVar(&configOverride, "config", "", "config path")
	flag.BoolVar(&doWhere, "where", false, "print config path")
	flag.BoolVar(&doInit, "init", false, "alias for `spell init`")
	flag.BoolVar(&showVersion, "version", false, "show version")
	flag.BoolVar(&showHelp, "h", false, "help")
	flag.BoolVar(&showHelp, "help", false, "help")
	flag.Usage = func() { fmt.Fprint(os.Stderr, usage) }
	flag.Parse()

	if showHelp {
		fmt.Print(usage)
		return
	}
	if showVersion {
		fmt.Println("spell", version)
		return
	}
	if configOverride != "" {
		_ = os.Setenv("SPELL_CONFIG", configOverride)
	}
	if doWhere {
		fmt.Println(config.Path())
		return
	}
	if doInit {
		runInit()
		return
	}

	cfg, err := config.Load()
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Fprintln(os.Stderr, "no config yet — running `spell init`...")
			fmt.Fprintln(os.Stderr)
			runInit()
			return
		}
		check(err)
	}
	if len(cfg.Providers) == 0 {
		fmt.Fprintln(os.Stderr, "no providers configured — running `spell init`...")
		fmt.Fprintln(os.Stderr)
		runInit()
		return
	}

	name := providerOverride
	if name == "" {
		name = cfg.Default
	}
	if name == "" {
		for k := range cfg.Providers {
			name = k
			break
		}
	}
	pcfg, ok := cfg.Providers[name]
	if !ok {
		fail("provider %q not found in config — try `spell init`", name)
	}
	if pcfg.APIKey == "" {
		fail("provider %q has an empty api_key (env var unset?) — try `spell init` or check your shell env", name)
	}
	provider, err := config.Build(name, pcfg)
	check(err)

	initialQuery := strings.TrimSpace(strings.Join(flag.Args(), " "))
	model := ui.New(provider, name, initialQuery)

	// Inline rendering — keep everything in the user's scrollback, do
	// not switch to the alt-screen. After exec, the command's output
	// flows naturally below the spell render.
	prog := tea.NewProgram(model)
	final, err := prog.Run()
	check(err)

	res := final.(ui.Model).Finished()
	printMode := os.Getenv("SPELL_PRINT") == "1"
	switch res.Action {
	case ui.ActionRun:
		if printMode {
			fmt.Println(res.Command)
		} else {
			execCommand(res.Command)
		}
	case ui.ActionAbort, ui.ActionNone:
		if printMode {
			os.Exit(130)
		}
	}
}

func runInit() {
	if err := setup.Run(); err != nil {
		// huh returns its own error on user abort (Ctrl+C); keep the
		// exit code distinguishable but quiet.
		if err.Error() == "user aborted" {
			os.Exit(130)
		}
		check(err)
	}
}

// runExplain handles `spell explain [--hook] <cmd…>`. Returns the exit
// code the process should use — main passes it to os.Exit so the
// shell command_not_found hook can preserve "127 = not found".
func runExplain(args []string) int {
	hook := false
	rest := args
	if len(rest) > 0 && (rest[0] == "--hook" || rest[0] == "-hook") {
		hook = true
		rest = rest[1:]
	}
	if len(rest) == 0 {
		fmt.Fprintln(os.Stderr, "usage: spell explain [--hook] <command…>")
		return 2
	}

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, "spell explain:", err)
		if hook {
			// Don't make the user's missing-config problem look like
			// a "not found" mystery — the hook should fall through.
			return 127
		}
		return 1
	}
	name := cfg.Default
	if name == "" {
		for k := range cfg.Providers {
			name = k
			break
		}
	}
	pcfg, ok := cfg.Providers[name]
	if !ok || pcfg.APIKey == "" {
		fmt.Fprintln(os.Stderr, "spell explain: no usable provider — run `spell init`")
		if hook {
			return 127
		}
		return 1
	}
	provider, err := config.Build(name, pcfg)
	if err != nil {
		fmt.Fprintln(os.Stderr, "spell explain:", err)
		if hook {
			return 127
		}
		return 1
	}

	mode := explain.ModeExplain
	if hook {
		mode = explain.ModeHook
	}
	return explain.Run(context.Background(), provider, mode, strings.Join(rest, " "))
}

// runInstallHook prints a shell snippet the user can append to their
// rc file. We do not write the file ourselves — that's the user's
// shell config, not ours, and people have strong opinions about who
// touches their dotfiles.
func runInstallHook(args []string) {
	shellName := ""
	if len(args) > 0 {
		shellName = args[0]
	} else {
		// best-effort detect from $SHELL
		if s := os.Getenv("SHELL"); s != "" {
			parts := strings.Split(s, "/")
			shellName = parts[len(parts)-1]
		}
	}
	switch shellName {
	case "zsh":
		fmt.Print(zshHook)
	case "bash":
		fmt.Print(bashHook)
	default:
		// Print both so the user can pick.
		fmt.Println("# --- zsh — append to ~/.zshrc ---")
		fmt.Print(zshHook)
		fmt.Println()
		fmt.Println("# --- bash — append to ~/.bashrc ---")
		fmt.Print(bashHook)
		fmt.Fprintln(os.Stderr, "\n# tip: pass `zsh` or `bash` as an argument to print just one")
	}
}

const zshHook = `# spell command-not-found hook (zsh)
# Pipe a typo / unknown command through "spell explain --hook" so the
# AI suggests what you probably meant. Remove this block to disable.
command_not_found_handler() {
  if command -v spell >/dev/null 2>&1; then
    spell explain --hook "$@"
    return $?
  fi
  echo "zsh: command not found: $1" >&2
  return 127
}
`

const bashHook = `# spell command-not-found hook (bash)
# Pipe a typo / unknown command through "spell explain --hook" so the
# AI suggests what you probably meant. Remove this block to disable.
command_not_found_handle() {
  if command -v spell >/dev/null 2>&1; then
    spell explain --hook "$@"
    return $?
  fi
  echo "bash: $1: command not found" >&2
  return 127
}
`

func execCommand(cmd string) {
	cmd = strings.TrimSpace(cmd)
	if cmd == "" {
		return
	}
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/sh"
	}
	// Replace this process so signals/exit codes flow naturally.
	args := []string{shell, "-c", cmd}
	env := os.Environ()
	if err := syscall.Exec(shell, args, env); err != nil {
		fmt.Println(cmd)
	}
}

func check(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func fail(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "spell: "+format+"\n", args...)
	os.Exit(1)
}
