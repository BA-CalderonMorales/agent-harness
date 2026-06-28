package config

const (
	DefaultProvider           = "local"
	DefaultRuntime            = "llama.cpp"
	DefaultModel              = "deepreinforce-ai/Ornith-1.0-9B-GGUF"
	DefaultEndpointURL        = "http://127.0.0.1:8080/v1"
	DefaultContextLength      = 8192
	DefaultMaxTokens          = 4096
	DefaultTemperature        = 0.2
	DefaultLocalServerCommand = "llama-server -m ./models/ornith-1.0-9b-Q4_K_M.gguf -c 8192 --host 127.0.0.1 --port 8080"
)

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
	default:
		return "nvidia/nemotron-3-super-120b-a12b:free"
	}
}

func DefaultEndpointForProvider(provider string) string {
	switch provider {
	case "local":
		return DefaultEndpointURL
	default:
		return ""
	}
}
