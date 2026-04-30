package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"syscall"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/rockscy/spell/internal/config"
	"github.com/rockscy/spell/internal/setup"
	"github.com/rockscy/spell/internal/ui"
)

var version = "dev"

const usage = `spell — AI command palette for your terminal.

USAGE
  spell init                    interactive provider setup wizard
  spell [flags] [query…]        cast a spell

FLAGS
  -p, --provider NAME           override the default provider
  -c, --config PATH             use a specific config file
      --where                   print resolved config path and exit
      --version                 show version and exit
  -h, --help                    this help

KEYS
  enter   submit / run command
  e       edit the suggested command
  c       copy command to clipboard and exit
  r       regenerate
  esc     cancel
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

	prog := tea.NewProgram(model, tea.WithAltScreen())
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
