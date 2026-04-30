package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"syscall"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/rockscy/spell/internal/config"
	"github.com/rockscy/spell/internal/ui"
)

var version = "dev"

const usage = `spell — AI command palette for your terminal.

USAGE
  spell [flags] [query…]

FLAGS
  -p, --provider NAME   override the default provider
  -c, --config PATH     use a specific config file
      --init            write a starter config and exit
      --where           print resolved config path and exit
      --version         show version and exit
  -h, --help            this help

KEYS
  enter   submit / run command
  e       edit the suggested command
  c       copy command to clipboard and exit
  r       regenerate
  esc     cancel
`

func main() {
	var (
		providerOverride string
		configOverride   string
		doInit, doWhere  bool
		showVersion      bool
		showHelp         bool
	)
	flag.StringVar(&providerOverride, "p", "", "provider name")
	flag.StringVar(&providerOverride, "provider", "", "provider name")
	flag.StringVar(&configOverride, "c", "", "config path")
	flag.StringVar(&configOverride, "config", "", "config path")
	flag.BoolVar(&doInit, "init", false, "write starter config")
	flag.BoolVar(&doWhere, "where", false, "print config path")
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
		created, err := config.WriteExample()
		check(err)
		p := config.Path()
		if created {
			fmt.Fprintf(os.Stderr, "wrote starter config to %s\n", p)
			fmt.Fprintln(os.Stderr, "edit it to add your api keys, then run `spell`.")
		} else {
			fmt.Fprintf(os.Stderr, "config already exists at %s\n", p)
		}
		return
	}

	cfg, err := config.Load()
	if err != nil {
		if os.IsNotExist(err) {
			created, werr := config.WriteExample()
			check(werr)
			if created {
				fmt.Fprintf(os.Stderr, "no config found — wrote starter to %s\n", config.Path())
				fmt.Fprintln(os.Stderr, "edit it to add your api keys, then run `spell` again.")
				os.Exit(2)
			}
		}
		check(err)
	}
	if len(cfg.Providers) == 0 {
		fail("no providers configured in %s", config.Path())
	}

	name := providerOverride
	if name == "" {
		name = cfg.Default
	}
	if name == "" {
		// pick the first declared
		for k := range cfg.Providers {
			name = k
			break
		}
	}
	pcfg, ok := cfg.Providers[name]
	if !ok {
		fail("provider %q not found in config", name)
	}
	if pcfg.APIKey == "" || strings.HasPrefix(pcfg.APIKey, "$") {
		fail("provider %q has no api_key (got %q) — set the env var or fill the config", name, pcfg.APIKey)
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
	case ui.ActionCopy:
		fmt.Fprintln(os.Stderr, "copied to clipboard.")
	case ui.ActionAbort, ui.ActionNone:
		if printMode {
			os.Exit(130)
		}
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
		// fall back: print and let the user paste
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
