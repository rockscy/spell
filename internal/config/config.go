package config

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/BurntSushi/toml"

	"github.com/rockscy/spell/internal/llm"
)

type ProviderCfg struct {
	Type      string `toml:"type"`       // "openai-compatible" | "anthropic"
	BaseURL   string `toml:"base_url"`
	APIKey    string `toml:"api_key"`
	Model     string `toml:"model"`
	MaxTokens int    `toml:"max_tokens"`
}

type Config struct {
	Default   string                 `toml:"default"`
	Providers map[string]ProviderCfg `toml:"providers"`
}

// Path returns the resolved config file path, honouring $XDG_CONFIG_HOME.
func Path() string {
	if p := os.Getenv("SPELL_CONFIG"); p != "" {
		return p
	}
	if x := os.Getenv("XDG_CONFIG_HOME"); x != "" {
		return filepath.Join(x, "spell", "config.toml")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "spell", "config.toml")
}

// Load reads the config file, expanding ${VAR} / $VAR refs in api_key fields.
// If the file does not exist, returns os.ErrNotExist so callers can offer
// a first-run flow.
func Load() (*Config, error) {
	p := Path()
	b, err := os.ReadFile(p)
	if err != nil {
		return nil, err
	}
	var c Config
	if err := toml.Unmarshal(b, &c); err != nil {
		return nil, fmt.Errorf("parse %s: %w", p, err)
	}
	for name, pr := range c.Providers {
		pr.APIKey = os.ExpandEnv(pr.APIKey)
		pr.BaseURL = os.ExpandEnv(pr.BaseURL)
		c.Providers[name] = pr
	}
	return &c, nil
}

// WriteExample creates the config directory and a starter config.toml
// at Path() if it does not already exist. Returns true if a new file
// was written.
func WriteExample() (bool, error) {
	p := Path()
	if _, err := os.Stat(p); err == nil {
		return false, nil
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return false, err
	}
	return true, os.WriteFile(p, []byte(exampleTOML), 0o600)
}

const exampleTOML = `# spell — AI command palette for your terminal
# https://github.com/rockscy/spell
#
# pick the active provider with "default", or override per-run with -p <name>.
default = "openai"

# ---------- OpenAI ----------
[providers.openai]
type     = "openai-compatible"
base_url = "https://api.openai.com/v1"
api_key  = "$OPENAI_API_KEY"
model    = "gpt-4o-mini"

# ---------- Anthropic (native protocol) ----------
[providers.anthropic]
type     = "anthropic"
api_key  = "$ANTHROPIC_API_KEY"
model    = "claude-haiku-4-5-20251001"

# ---------- Local: Ollama ----------
[providers.ollama]
type     = "openai-compatible"
base_url = "http://localhost:11434/v1"
api_key  = "ollama"  # ollama ignores the key but the field is required
model    = "llama3.2"

# ---------- Xiaomi MiMo (Token Plan, Singapore) ----------
# Uses reasoning models — bump max_tokens so the model has budget
# for both its chain-of-thought and the final command.
# [providers.mimo]
# type       = "openai-compatible"
# base_url   = "https://token-plan-sgp.xiaomimimo.com/v1"
# api_key    = "$MIMO_API_KEY"
# model      = "mimo-v2-pro"  # or mimo-v2.5-pro / mimo-v2-omni
# max_tokens = 2048

# ---------- DeepSeek ----------
# [providers.deepseek]
# type     = "openai-compatible"
# base_url = "https://api.deepseek.com/v1"
# api_key  = "$DEEPSEEK_API_KEY"
# model    = "deepseek-chat"

# ---------- Moonshot / Kimi ----------
# [providers.kimi]
# type     = "openai-compatible"
# base_url = "https://api.moonshot.cn/v1"
# api_key  = "$MOONSHOT_API_KEY"
# model    = "moonshot-v1-8k"

# ---------- Groq ----------
# [providers.groq]
# type     = "openai-compatible"
# base_url = "https://api.groq.com/openai/v1"
# api_key  = "$GROQ_API_KEY"
# model    = "llama-3.3-70b-versatile"

# ---------- OpenRouter ----------
# [providers.openrouter]
# type     = "openai-compatible"
# base_url = "https://openrouter.ai/api/v1"
# api_key  = "$OPENROUTER_API_KEY"
# model    = "anthropic/claude-haiku-4-5"
`

// Build wires a ProviderCfg into a concrete llm.Provider.
func Build(name string, p ProviderCfg) (llm.Provider, error) {
	httpc := &http.Client{Timeout: 60 * time.Second}
	switch p.Type {
	case "openai-compatible", "openai", "":
		return &llm.OpenAI{
			BaseURL:    p.BaseURL,
			APIKey:     p.APIKey,
			Model:      p.Model,
			MaxTokens:  p.MaxTokens,
			HTTPClient: httpc,
			Label:      name,
		}, nil
	case "anthropic":
		base := p.BaseURL
		if base == "" {
			base = "https://api.anthropic.com"
		}
		return &llm.Anthropic{
			BaseURL:    base,
			APIKey:     p.APIKey,
			Model:      p.Model,
			MaxTokens:  p.MaxTokens,
			HTTPClient: httpc,
		}, nil
	default:
		return nil, fmt.Errorf("unknown provider type %q for %q", p.Type, name)
	}
}
