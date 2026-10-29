package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/graydraft/gocommon/sealed"
	"gopkg.in/yaml.v3"
)

// ProviderConfig is an OpenAI-compatible API endpoint (DeepSeek / OpenAI / 其他).
type ProviderConfig struct {
	Enabled bool   `yaml:"enabled"`
	BaseURL string `yaml:"base_url"`
	APIKey  string `yaml:"api_key"`
}

// ModelBinding selects provider + model id for a capability.
type ModelBinding struct {
	Provider string `yaml:"provider"`
	Model    string `yaml:"model"`
}

// Config holds model gateway settings.
type Config struct {
	App struct {
		Name string `yaml:"name"`
		Env  string `yaml:"env"`
	} `yaml:"app"`
	HTTP struct {
		Port int `yaml:"port"`
	} `yaml:"http"`
	Models struct {
		Embedding ModelBinding `yaml:"embedding"`
		LLMFast   ModelBinding `yaml:"llm_fast"`
		LLM       ModelBinding `yaml:"llm"` // complex / high-accuracy
		Rerank    ModelBinding `yaml:"rerank"`
	} `yaml:"models"`
	Embedding struct {
		Dim           int  `yaml:"dim"`
		FallbackLocal bool `yaml:"fallback_local"`
	} `yaml:"embedding"`
	LLM struct {
		FallbackLocal bool    `yaml:"fallback_local"`
		MaxTokens     int     `yaml:"max_tokens"`
		Temperature   float64 `yaml:"temperature"`
	} `yaml:"llm"`
	Rerank struct {
		FallbackLocal bool `yaml:"fallback_local"`
	} `yaml:"rerank"`
	Providers struct {
		DeepSeek  ProviderConfig `yaml:"deepseek"`
		DashScope ProviderConfig `yaml:"dashscope"`
		OpenAI    ProviderConfig `yaml:"openai"`
		Intern    ProviderConfig `yaml:"intern"`
	} `yaml:"providers"`
}

// Load reads YAML and overlays environment.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	overlayEnv(&cfg)
	applyDefaults(&cfg)
	return &cfg, nil
}

func applyDefaults(cfg *Config) {
	if cfg.HTTP.Port == 0 {
		cfg.HTTP.Port = 18002
	}
	if cfg.Embedding.Dim == 0 {
		cfg.Embedding.Dim = 1024
	}
	if cfg.Models.Embedding.Provider == "" {
		cfg.Models.Embedding.Provider = "dashscope"
	}
	if cfg.Models.Embedding.Model == "" {
		cfg.Models.Embedding.Model = "text-embedding-v4"
	}
	if cfg.Models.LLMFast.Provider == "" {
		cfg.Models.LLMFast.Provider = "intern"
	}
	if cfg.Models.LLMFast.Model == "" {
		cfg.Models.LLMFast.Model = "intern-s1-mini"
	}
	if cfg.Models.LLM.Provider == "" {
		cfg.Models.LLM.Provider = cfg.Models.LLMFast.Provider
	}
	if cfg.Models.LLM.Model == "" {
		cfg.Models.LLM.Model = "intern-s2"
	}
	if cfg.Models.Rerank.Provider == "" {
		cfg.Models.Rerank.Provider = "dashscope"
	}
	if cfg.Models.Rerank.Model == "" {
		cfg.Models.Rerank.Model = "qwen3-rerank"
	}
	if cfg.LLM.MaxTokens == 0 {
		cfg.LLM.MaxTokens = 4096
	}
	if cfg.Providers.DeepSeek.BaseURL == "" {
		cfg.Providers.DeepSeek.BaseURL = "https://api.deepseek.com"
	}
	if cfg.Providers.OpenAI.BaseURL == "" {
		cfg.Providers.OpenAI.BaseURL = "https://api.openai.com"
	}
	if cfg.Providers.Intern.BaseURL == "" {
		cfg.Providers.Intern.BaseURL = "https://chat.intern-ai.org.cn/api/v1"
	}
	if cfg.Providers.DashScope.BaseURL == "" {
		cfg.Providers.DashScope.BaseURL = "https://dashscope.aliyuncs.com/compatible-mode"
	}
}

func overlayEnv(cfg *Config) {
	if v := os.Getenv("HTTP_PORT"); v != "" {
		if p, err := strconv.Atoi(v); err == nil {
			cfg.HTTP.Port = p
		}
	}
	if v := os.Getenv("DEEPSEEK_API_KEY"); v != "" {
		cfg.Providers.DeepSeek.APIKey = v
		cfg.Providers.DeepSeek.Enabled = true
	}
	if v := os.Getenv("DEEPSEEK_BASE_URL"); v != "" {
		cfg.Providers.DeepSeek.BaseURL = v
	}
	if v := os.Getenv("OPENAI_API_KEY"); v != "" {
		cfg.Providers.OpenAI.APIKey = v
		cfg.Providers.OpenAI.Enabled = true
	}
	if v := os.Getenv("OPENAI_BASE_URL"); v != "" {
		cfg.Providers.OpenAI.BaseURL = v
	}
	if v := os.Getenv("INTERN_API_KEY"); v != "" {
		cfg.Providers.Intern.APIKey = v
		cfg.Providers.Intern.Enabled = true
	}
	if v := os.Getenv("INTERN_BASE_URL"); v != "" {
		cfg.Providers.Intern.BaseURL = v
	}
	if v := os.Getenv("DASHSCOPE_API_KEY"); v != "" {
		cfg.Providers.DashScope.APIKey = v
		cfg.Providers.DashScope.Enabled = true
	}
	if v := os.Getenv("DASHSCOPE_BASE_URL"); v != "" {
		cfg.Providers.DashScope.BaseURL = v
	}
	if v := os.Getenv("EMBEDDING_DIM"); v != "" {
		if d, err := strconv.Atoi(v); err == nil {
			cfg.Embedding.Dim = d
		}
	}
	if v := os.Getenv("EMBEDDING_MODEL"); v != "" {
		cfg.Models.Embedding.Model = v
	}
	if v := os.Getenv("LLM_MODEL_FAST"); v != "" {
		cfg.Models.LLMFast.Model = v
	}
	if v := os.Getenv("LLM_MODEL"); v != "" {
		cfg.Models.LLM.Model = v
	}
	if v := os.Getenv("LLM_MODEL_COMPLEX"); v != "" {
		cfg.Models.LLM.Model = v
	}
	if v := os.Getenv("RERANK_MODEL"); v != "" {
		cfg.Models.Rerank.Model = v
	}
	if v := os.Getenv("EMBEDDING_PROVIDER"); v != "" {
		cfg.Models.Embedding.Provider = v
	}
	if v := os.Getenv("LLM_PROVIDER"); v != "" {
		cfg.Models.LLM.Provider = v
	}
	if v := os.Getenv("RERANK_PROVIDER"); v != "" {
		cfg.Models.Rerank.Provider = v
	}
	if strings.TrimSpace(cfg.Providers.DeepSeek.APIKey) == "" {
		cfg.Providers.DeepSeek.APIKey = sealed.DeepSeek()
	}
	if strings.TrimSpace(cfg.Providers.Intern.APIKey) == "" {
		cfg.Providers.Intern.APIKey = sealed.Intern()
	}
	if strings.TrimSpace(cfg.Providers.DashScope.APIKey) == "" {
		cfg.Providers.DashScope.APIKey = sealed.DashScope()
	}
}

// ResolveProvider returns API settings for a named provider.
func (c *Config) ResolveProvider(name string) (ProviderConfig, error) {
	name = strings.ToLower(strings.TrimSpace(name))
	switch name {
	case "deepseek":
		p := c.Providers.DeepSeek
		if !p.Enabled && p.APIKey == "" {
			return ProviderConfig{}, fmt.Errorf("provider deepseek not configured (set DEEPSEEK_API_KEY)")
		}
		if !p.Enabled {
			p.Enabled = p.APIKey != ""
		}
		return p, nil
	case "openai":
		p := c.Providers.OpenAI
		if !p.Enabled && p.APIKey == "" {
			return ProviderConfig{}, fmt.Errorf("provider openai not configured")
		}
		if !p.Enabled {
			p.Enabled = p.APIKey != ""
		}
		return p, nil
	case "intern":
		p := c.Providers.Intern
		if !p.Enabled && p.APIKey == "" {
			return ProviderConfig{}, fmt.Errorf("provider intern not configured (set INTERN_API_KEY)")
		}
		if !p.Enabled {
			p.Enabled = p.APIKey != ""
		}
		return p, nil
	case "dashscope", "qwen", "bailian":
		p := c.Providers.DashScope
		if !p.Enabled && p.APIKey == "" {
			return ProviderConfig{}, fmt.Errorf("provider dashscope not configured (set DASHSCOPE_API_KEY)")
		}
		if !p.Enabled {
			p.Enabled = p.APIKey != ""
		}
		return p, nil
	default:
		return ProviderConfig{}, fmt.Errorf("unknown provider: %s", name)
	}
}

// APIReady reports whether remote API can be used (any provider has key).
func (c *Config) APIReady() bool {
	return (c.Providers.DeepSeek.APIKey != "") ||
		(c.Providers.OpenAI.APIKey != "") ||
		(c.Providers.DashScope.APIKey != "") ||
		(c.Providers.Intern.APIKey != "")
}
