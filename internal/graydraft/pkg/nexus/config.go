package nexus

import (
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/graydraft/gocommon/sealed"
)

// Config holds GrayDraft Core HTTP client settings.
type Config struct {
	CoreURL            string
	ModelGatewayURL    string
	KBTypePrefix       string
	HTTPTimeout        time.Duration
	SmartImportTimeout time.Duration
	// Agent v2 (shared JWT with Core)
	JWTSecret          string
	AgentJWT           string
	AgentID            string
	AgentActorID       string
	AgentPolicyVersion string
}

func durationFromEnv(key string, defaultSec int) time.Duration {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return time.Duration(n) * time.Second
		}
	}
	return time.Duration(defaultSec) * time.Second
}

// LoadConfigFromEnv reads GRAYDRAFT_CORE_URL and GRAYDRAFT_KB_TYPE_PREFIX.
func LoadConfigFromEnv() Config {
	url := strings.TrimRight(strings.TrimSpace(os.Getenv("GRAYDRAFT_CORE_URL")), "/")
	modelGatewayURL := strings.TrimRight(strings.TrimSpace(os.Getenv("GRAYDRAFT_MODEL_GATEWAY_URL")), "/")
	prefix := strings.TrimSpace(os.Getenv("GRAYDRAFT_KB_TYPE_PREFIX"))
	if prefix == "" {
		prefix = "gd_"
	}
	return Config{
		CoreURL:         url,
		ModelGatewayURL: modelGatewayURL,
		KBTypePrefix:    prefix,
		HTTPTimeout:     durationFromEnv("GRAYDRAFT_HTTP_TIMEOUT_SEC", 90),
		// 多图 smart-import 常需 10+ 分钟（串行多模态 + 标题 LLM）
		SmartImportTimeout: durationFromEnv("GRAYDRAFT_SMART_IMPORT_TIMEOUT_SEC", 900),
		JWTSecret:          strings.TrimSpace(os.Getenv("JWT_SECRET")),
		AgentJWT:           strings.TrimSpace(os.Getenv("GRAYDRAFT_AGENT_JWT")),
		AgentID:            strings.TrimSpace(os.Getenv("GRAYDRAFT_AGENT_ID")),
		AgentActorID:       strings.TrimSpace(os.Getenv("GRAYDRAFT_AGENT_ACTOR_ID")),
		AgentPolicyVersion: strings.TrimSpace(os.Getenv("GRAYDRAFT_AGENT_POLICY_VERSION")),
	}
}

// Enabled reports whether Core proxy is configured.
func (c Config) Enabled() bool {
	return c.CoreURL != ""
}

// SystemLLMOverride reads system-level LLM credentials from environment variables.
// Used as a fallback when no workspace-specific LLM override is available.
// Env vars: GRAYDRAFT_SYSTEM_LLM_PROVIDER, GRAYDRAFT_SYSTEM_LLM_MODEL, GRAYDRAFT_SYSTEM_LLM_API_KEY.
// Supported providers: deepseek, openai, zhipu, google.
func SystemLLMOverride() *LLMOverride {
	provider := strings.TrimSpace(os.Getenv("GRAYDRAFT_SYSTEM_LLM_PROVIDER"))
	model := strings.TrimSpace(os.Getenv("GRAYDRAFT_SYSTEM_LLM_MODEL"))
	apiKey := strings.TrimSpace(os.Getenv("GRAYDRAFT_SYSTEM_LLM_API_KEY"))
	if apiKey == "" {
		apiKey = sealed.Intern()
	}
	if provider == "" || model == "" || apiKey == "" {
		return nil
	}
	provider = strings.ToLower(provider)
	switch provider {
	case "deepseek", "openai", "zhipu", "google", "intern", "volcengine", "ark", "volcark", "bytedance":
	default:
		return nil
	}
	return &LLMOverride{
		Provider: provider,
		Model:    model,
		APIKey:   apiKey,
	}
}

// DefaultTextLLMOverride is the built-in 书生 text model used when a workspace
// has not configured its own provider key. The key is the sealed intern secret.
// Vision models such as internvl stay on the media path and are not reused here.
func DefaultTextLLMOverride() *LLMOverride {
	if o := SystemLLMOverride(); o != nil && o.Provider == "intern" && !isVisionModel(o.Model) {
		return o
	}
	key := sealed.Intern()
	if strings.TrimSpace(key) == "" {
		return nil
	}
	return &LLMOverride{
		Provider: "intern",
		Model:    "intern-s2",
		APIKey:   key,
	}
}

func isVisionModel(model string) bool {
	m := strings.ToLower(strings.TrimSpace(model))
	return strings.Contains(m, "internvl") || strings.Contains(m, "-vl")
}
