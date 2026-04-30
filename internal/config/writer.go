package config

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Save writes the config back to Path() in a stable, comment-aware layout.
// It does NOT preserve free-form comments the user may have added — once
// `spell init` manages a config, the wizard owns the file shape.
func Save(c *Config) error {
	p := Path()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}

	var b strings.Builder
	b.WriteString("# spell config — managed by `spell init`.\n")
	b.WriteString("# https://github.com/rockscy/spell\n")
	b.WriteString("#\n")
	b.WriteString("# api_key may be a raw secret OR a $ENV_VAR reference\n")
	b.WriteString("# (recommended: keep keys in env, not on disk).\n\n")

	if c.Default != "" {
		fmt.Fprintf(&b, "default = %q\n\n", c.Default)
	}

	names := make([]string, 0, len(c.Providers))
	for k := range c.Providers {
		names = append(names, k)
	}
	sort.Strings(names)

	for _, name := range names {
		pr := c.Providers[name]
		fmt.Fprintf(&b, "[providers.%s]\n", name)
		fmt.Fprintf(&b, "type     = %q\n", pr.Type)
		if pr.BaseURL != "" {
			fmt.Fprintf(&b, "base_url = %q\n", pr.BaseURL)
		}
		fmt.Fprintf(&b, "api_key  = %q\n", pr.APIKey)
		fmt.Fprintf(&b, "model    = %q\n", pr.Model)
		if pr.MaxTokens > 0 {
			fmt.Fprintf(&b, "max_tokens = %d\n", pr.MaxTokens)
		}
		b.WriteString("\n")
	}

	return os.WriteFile(p, []byte(b.String()), 0o600)
}

// LoadOrEmpty returns the existing config, or a zero-value Config if the
// file does not exist. Any other read/parse error is propagated.
func LoadOrEmpty() (*Config, error) {
	c, err := Load()
	if err != nil {
		if os.IsNotExist(err) {
			return &Config{Providers: map[string]ProviderCfg{}}, nil
		}
		return nil, err
	}
	if c.Providers == nil {
		c.Providers = map[string]ProviderCfg{}
	}
	return c, nil
}
