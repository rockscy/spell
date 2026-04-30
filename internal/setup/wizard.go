package setup

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/huh"

	"github.com/rockscy/spell/internal/config"
)

// Run launches the interactive provider wizard. It loads the existing
// config (or starts empty), walks through one provider's worth of
// choices, optionally loops to add more, and saves to disk.
func Run() error {
	cfg, err := config.LoadOrEmpty()
	if err != nil {
		return err
	}

	printIntro(cfg)

	for {
		if err := addOne(cfg); err != nil {
			return err
		}
		var more bool
		if err := huh.NewConfirm().
			Title("Add another provider?").
			Affirmative("Yes, add another").
			Negative("No, I'm done").
			Value(&more).
			Run(); err != nil {
			return err
		}
		if !more {
			break
		}
	}

	if cfg.Default == "" {
		for k := range cfg.Providers {
			cfg.Default = k
			break
		}
	}

	if err := config.Save(cfg); err != nil {
		return err
	}

	fmt.Printf("\n✓ Saved %d provider(s) to %s\n", len(cfg.Providers), config.Path())
	fmt.Printf("  default: %s\n", cfg.Default)
	fmt.Println("  Run `spell` to cast your first command.")
	return nil
}

func printIntro(cfg *config.Config) {
	fmt.Println("✦ spell init")
	if len(cfg.Providers) == 0 {
		fmt.Println("  Let's get you a provider configured.")
		fmt.Println()
		return
	}
	fmt.Printf("  Found %d provider(s) in %s\n", len(cfg.Providers), config.Path())
	for k, p := range cfg.Providers {
		marker := "  "
		if k == cfg.Default {
			marker = "★ "
		}
		fmt.Printf("  %s%-12s %s\n", marker, k, p.Model)
	}
	fmt.Println()
}

func addOne(cfg *config.Config) error {
	preset, err := pickPreset()
	if err != nil {
		return err
	}

	name := preset.Key
	pcfg := config.ProviderCfg{
		Type:    preset.Type,
		BaseURL: preset.BaseURL,
	}

	if preset.IsCustom {
		if err := promptCustom(&name, &pcfg); err != nil {
			return err
		}
	}

	if err := promptKey(name, preset, &pcfg); err != nil {
		return err
	}

	if err := promptModel(preset, &pcfg); err != nil {
		return err
	}

	if err := promptDefault(name, cfg); err != nil {
		return err
	}

	cfg.Providers[name] = pcfg
	return nil
}

func pickPreset() (*Preset, error) {
	opts := make([]huh.Option[string], 0, len(Catalog))
	for _, p := range Catalog {
		opts = append(opts, huh.NewOption(p.Display, p.Key))
	}
	var key string
	err := huh.NewSelect[string]().
		Title("Pick a provider").
		Description("Each entry comes with a curated model list. Custom lets you point at any compatible endpoint.").
		Options(opts...).
		Height(12).
		Value(&key).
		Run()
	if err != nil {
		return nil, err
	}
	p := FindPreset(key)
	if p == nil {
		return nil, fmt.Errorf("unknown preset %q", key)
	}
	return p, nil
}

func promptCustom(name *string, pcfg *config.ProviderCfg) error {
	*name = ""
	pcfg.BaseURL = ""
	pcfg.Type = "openai-compatible"
	return huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Provider name").
				Description("Lowercase identifier used as the toml key. e.g. mycompany").
				Value(name).
				Validate(validateName),
			huh.NewInput().
				Title("Base URL").
				Description("e.g. https://api.example.com/v1").
				Value(&pcfg.BaseURL).
				Validate(validateURL),
			huh.NewSelect[string]().
				Title("Protocol").
				Description("Most providers expose an OpenAI-compatible chat-completions endpoint.").
				Options(
					huh.NewOption("OpenAI-compatible", "openai-compatible"),
					huh.NewOption("Anthropic-native", "anthropic"),
				).
				Value(&pcfg.Type),
		),
	).Run()
}

func promptKey(name string, preset *Preset, pcfg *config.ProviderCfg) error {
	// fallback is what we save if the user submits an empty input
	fallback := ""
	if preset.EnvVar != "" {
		fallback = "$" + preset.EnvVar
	}
	if preset.Key == "ollama" {
		// ollama ignores the key but the field is required by spec
		fallback = "ollama"
	}

	desc := "Paste the API key."
	if fallback != "" {
		desc = fmt.Sprintf("Paste the key, or leave blank to use %s.", fallback)
	}

	var keyInput string
	input := huh.NewInput().
		Title(fmt.Sprintf("API key for %s", name)).
		Description(desc).
		EchoMode(huh.EchoModePassword).
		Placeholder(fallback).
		Value(&keyInput)
	if fallback == "" {
		input = input.Validate(huh.ValidateNotEmpty())
	}
	if err := input.Run(); err != nil {
		return err
	}

	keyInput = strings.TrimSpace(keyInput)
	if keyInput == "" {
		keyInput = fallback
	}
	pcfg.APIKey = keyInput
	return nil
}

func promptModel(preset *Preset, pcfg *config.ProviderCfg) error {
	if preset.IsCustom || len(preset.Models) == 0 {
		var id string
		if err := huh.NewInput().
			Title("Model").
			Description("Exact model id as the provider expects it. e.g. gpt-4o-mini").
			Value(&id).
			Validate(huh.ValidateNotEmpty()).
			Run(); err != nil {
			return err
		}
		pcfg.Model = strings.TrimSpace(id)
		return nil
	}

	opts := make([]huh.Option[string], 0, len(preset.Models)+1)
	var defaultID string
	for _, m := range preset.Models {
		label := m.Display
		if m.Recommended {
			label = "★ " + label
		}
		opts = append(opts, huh.NewOption(label, m.ID))
		if m.Recommended && defaultID == "" {
			defaultID = m.ID
		}
	}
	opts = append(opts, huh.NewOption("Custom (type your own)", "__custom__"))

	id := defaultID
	if err := huh.NewSelect[string]().
		Title("Model").
		Options(opts...).
		Height(8).
		Value(&id).
		Run(); err != nil {
		return err
	}

	if id == "__custom__" {
		var typed string
		if err := huh.NewInput().
			Title("Custom model id").
			Value(&typed).
			Validate(huh.ValidateNotEmpty()).
			Run(); err != nil {
			return err
		}
		id = strings.TrimSpace(typed)
	}
	pcfg.Model = id

	for _, m := range preset.Models {
		if m.ID == id && m.Reasoning {
			pcfg.MaxTokens = 2048
			break
		}
	}
	return nil
}

func promptDefault(name string, cfg *config.Config) error {
	if cfg.Default == "" {
		cfg.Default = name
		return nil
	}
	if cfg.Default == name {
		return nil
	}
	var setDefault bool
	if err := huh.NewConfirm().
		Title(fmt.Sprintf("Set %q as the default provider?", name)).
		Description(fmt.Sprintf("Current default: %s", cfg.Default)).
		Value(&setDefault).
		Run(); err != nil {
		return err
	}
	if setDefault {
		cfg.Default = name
	}
	return nil
}

func validateName(s string) error {
	s = strings.TrimSpace(s)
	if s == "" {
		return fmt.Errorf("name required")
	}
	for _, r := range s {
		ok := (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_' || r == '-'
		if !ok {
			return fmt.Errorf("only lowercase letters, digits, _ or - allowed")
		}
	}
	return nil
}

func validateURL(s string) error {
	s = strings.TrimSpace(s)
	if s == "" {
		return fmt.Errorf("URL required")
	}
	if !strings.HasPrefix(s, "http://") && !strings.HasPrefix(s, "https://") {
		return fmt.Errorf("must start with http:// or https://")
	}
	return nil
}
