package setup

// Preset is a built-in provider template. The wizard prefills the base URL,
// suggests an env-var name for the API key, and offers a curated model list.
type Preset struct {
	Key      string  // toml table name, e.g. "openai"
	Display  string  // human label
	Type     string  // "openai-compatible" | "anthropic"
	BaseURL  string  // empty for Custom (user types it)
	EnvVar   string  // suggested env var, e.g. "OPENAI_API_KEY"
	Models   []Model // empty for Custom (user types it)
	IsCustom bool    // true ⇒ wizard asks for name/base_url/type
}

type Model struct {
	ID          string
	Display     string
	Recommended bool
	Reasoning   bool // when true, wizard sets max_tokens=2048 automatically
}

// Catalog is the ordered list shown to the user. Reasoning models are
// flagged so the wizard can bump max_tokens for them; the order roughly
// reflects "what most people probably want first".
var Catalog = []Preset{
	{
		Key: "openai", Display: "OpenAI",
		Type: "openai-compatible", BaseURL: "https://api.openai.com/v1",
		EnvVar: "OPENAI_API_KEY",
		Models: []Model{
			{ID: "gpt-4o-mini", Display: "GPT-4o mini  (fast, cheap)", Recommended: true},
			{ID: "gpt-4o", Display: "GPT-4o"},
			{ID: "o4-mini", Display: "o4-mini  (reasoning)", Reasoning: true},
		},
	},
	{
		Key: "anthropic", Display: "Anthropic (Claude)",
		Type: "anthropic", BaseURL: "https://api.anthropic.com",
		EnvVar: "ANTHROPIC_API_KEY",
		Models: []Model{
			{ID: "claude-haiku-4-5-20251001", Display: "Claude Haiku 4.5  (fast)", Recommended: true},
			{ID: "claude-sonnet-4-6", Display: "Claude Sonnet 4.6"},
			{ID: "claude-opus-4-7", Display: "Claude Opus 4.7"},
		},
	},
	{
		Key: "mimo", Display: "Xiaomi MiMo (Token Plan, Singapore)",
		Type: "openai-compatible", BaseURL: "https://token-plan-sgp.xiaomimimo.com/v1",
		EnvVar: "MIMO_API_KEY",
		Models: []Model{
			{ID: "mimo-v2-pro", Display: "MiMo v2 Pro  (reasoning)", Recommended: true, Reasoning: true},
			{ID: "mimo-v2.5-pro", Display: "MiMo v2.5 Pro  (reasoning)", Reasoning: true},
			{ID: "mimo-v2-omni", Display: "MiMo v2 Omni  (multimodal)"},
			{ID: "mimo-v2.5", Display: "MiMo v2.5"},
		},
	},
	{
		Key: "deepseek", Display: "DeepSeek",
		Type: "openai-compatible", BaseURL: "https://api.deepseek.com/v1",
		EnvVar: "DEEPSEEK_API_KEY",
		Models: []Model{
			{ID: "deepseek-chat", Display: "DeepSeek-V3", Recommended: true},
			{ID: "deepseek-reasoner", Display: "DeepSeek-R1  (reasoning)", Reasoning: true},
		},
	},
	{
		Key: "kimi", Display: "Moonshot / Kimi",
		Type: "openai-compatible", BaseURL: "https://api.moonshot.cn/v1",
		EnvVar: "MOONSHOT_API_KEY",
		Models: []Model{
			{ID: "moonshot-v1-8k", Display: "Moonshot v1 (8k)", Recommended: true},
			{ID: "moonshot-v1-32k", Display: "Moonshot v1 (32k)"},
			{ID: "moonshot-v1-128k", Display: "Moonshot v1 (128k)"},
		},
	},
	{
		Key: "groq", Display: "Groq  (fast inference)",
		Type: "openai-compatible", BaseURL: "https://api.groq.com/openai/v1",
		EnvVar: "GROQ_API_KEY",
		Models: []Model{
			{ID: "llama-3.3-70b-versatile", Display: "Llama 3.3 70B", Recommended: true},
			{ID: "llama-3.1-8b-instant", Display: "Llama 3.1 8B  (instant)"},
		},
	},
	{
		Key: "openrouter", Display: "OpenRouter  (one key, every model)",
		Type: "openai-compatible", BaseURL: "https://openrouter.ai/api/v1",
		EnvVar: "OPENROUTER_API_KEY",
		Models: []Model{
			{ID: "anthropic/claude-haiku-4-5", Display: "Claude Haiku 4.5", Recommended: true},
			{ID: "openai/gpt-4o-mini", Display: "GPT-4o mini"},
			{ID: "xiaomi/mimo-v2-flash:free", Display: "Xiaomi MiMo v2 Flash  (free)"},
		},
	},
	{
		Key: "ollama", Display: "Ollama  (local, no key needed)",
		Type: "openai-compatible", BaseURL: "http://localhost:11434/v1",
		EnvVar: "", // ollama ignores the key but the field is required
		Models: []Model{
			{ID: "llama3.2", Display: "Llama 3.2", Recommended: true},
			{ID: "qwen2.5", Display: "Qwen 2.5"},
			{ID: "mistral", Display: "Mistral"},
		},
	},
	{
		Key: "custom", Display: "Custom  (any OpenAI-compatible / Anthropic-compatible endpoint)",
		IsCustom: true,
	},
}

// FindPreset returns the catalog entry matching key, or nil.
func FindPreset(key string) *Preset {
	for i := range Catalog {
		if Catalog[i].Key == key {
			return &Catalog[i]
		}
	}
	return nil
}
