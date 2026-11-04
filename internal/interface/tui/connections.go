package tui

import "github.com/BA-CalderonMorales/agent-harness/internal/core/config"

// IDs remain stable in saved settings; labels explain the connection to people.
func connectionLabel(provider string) string {
	switch provider {
	case "lemonade":
		return "Lemonade"
	case "flm":
		return "FastFlowLM (direct)"
	case "local":
		return "Custom local server"
	case "ollama":
		return "Ollama"
	case "openai":
		return "OpenAI"
	case "anthropic":
		return "Anthropic"
	case "openrouter":
		return "OpenRouter"
	case "fireworks":
		return "Fireworks"
	case "nvidia":
		return "NVIDIA"
	case "omniroute":
		return "Omniroute"
	default:
		return provider
	}
}

// Headings belong to the first option in a group, keeping cursor indices and
// keyboard navigation aligned with actual connections, including short panes.
func connectionItems(cursor int, descriptions bool) []modalItem {
	items := make([]modalItem, 0, len(loginProviders))
	for i, provider := range loginProviders {
		var lines []string
		local := config.IsLocalProvider(provider)
		if i == 0 || local != config.IsLocalProvider(loginProviders[i-1]) {
			heading := "Hosted inference"
			if local {
				heading = "On this computer"
			}
			lines = append(lines, HelpTitleStyle.Render(heading))
		}
		marker, style := IndicatorUnselected, HelpDimStyle
		if i == cursor {
			marker, style = IndicatorSelected, InfoStyle
		}
		lines = append(lines, style.Render(marker+connectionLabel(provider)))
		if descriptions {
			lines = append(lines, HelpDimStyle.Render("    "+providerBlurbs[provider]))
		}
		items = append(items, modalItem{lines: lines})
	}
	return items
}
