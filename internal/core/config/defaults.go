package config

const (
	DefaultProvider           = "local"
	DefaultRuntime            = "llama.cpp"
	DefaultModel              = "deepreinforce-ai/Ornith-1.0-9B-GGUF"
	DefaultEndpointURL        = "http://127.0.0.1:8080/v1"
	DefaultContextLength      = 8192
	DefaultMaxTokens          = 4096
	DefaultTemperature        = 0.2
	DefaultEffort             = "medium"
	DefaultLocalServerCommand = "llama-server -m ./models/ornith-1.0-9b-Q4_K_M.gguf -c 8192 --host 127.0.0.1 --port 8080"
)

// EffortLevels lists supported reasoning effort values in cycle order.
var EffortLevels = []string{"low", "medium", "high"}

func IsLocalProvider(provider string) bool {
	return provider == "local" || provider == "ollama"
}

func DefaultModelForProvider(provider string) string {
	switch provider {
	case "openai":
		return "gpt-4o"
	case "anthropic":
		return "claude-3-5-sonnet-20241022"
	case "ollama":
		return "gemma4:2b"
	case "local":
		return DefaultModel
	case "fireworks":
		return "accounts/fireworks/models/llama-v3p3-70b-instruct"
	case "nvidia":
		return "nvidia/nemotron-3.5-lightning"
	default:
		return "nvidia/nemotron-3-super-120b-a12b:free"
	}
}

func DefaultEndpointForProvider(provider string) string {
	switch provider {
	case "openai":
		return "https://api.openai.com/v1"
	case "anthropic":
		return "https://api.anthropic.com/v1"
	case "openrouter":
		return "https://openrouter.ai/api/v1"
	case "fireworks":
		return "https://api.fireworks.ai/inference/v1"
	case "nvidia":
		return "https://integrate.api.nvidia.com/v1"
	case "ollama":
		return "http://127.0.0.1:11434/v1"
	case "local":
		return DefaultEndpointURL
	default:
		return DefaultEndpointURL
	}
}
