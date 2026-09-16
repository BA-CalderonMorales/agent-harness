// Where to get an API key, per hosted provider.
//
// The login wizard used to ask for a key with no hint of where one comes
// from: a first-run user was told "Enter API key" with no path forward.
// Every provider that needs a key now carries its own short journey, so
// the wizard can answer "how do I get one?" at the moment it asks.
//
// The local runtimes answer without a key, so they are absent by design.

package tui

import "fmt"

// providerKeyStep is one hosted provider's key-acquisition journey: the
// page it starts from and the ordered steps to a usable key.
type providerKeyStep struct {
	URL   string
	Steps []string
}

// providerKeySteps maps a provider id to its journey. Keys match
// loginProviders; an id missing here needs no key.
var providerKeySteps = map[string]providerKeyStep{
	"openai": {
		URL: "https://platform.openai.com/api-keys",
		Steps: []string{
			"Sign in at platform.openai.com",
			"Open API keys, then Create new secret key",
			"Copy it now — it is shown once",
		},
	},
	"anthropic": {
		URL: "https://console.anthropic.com/settings/keys",
		Steps: []string{
			"Sign in at console.anthropic.com",
			"Open Settings, then API keys",
			"Create a key and copy it",
		},
	},
	"openrouter": {
		URL: "https://openrouter.ai/keys",
		Steps: []string{
			"Sign in at openrouter.ai",
			"Open Keys, then Create key",
			"Copy it — keys start with sk-or-",
		},
	},
	"fireworks": {
		URL: "https://fireworks.ai/account/api-keys",
		Steps: []string{
			"Sign in at fireworks.ai",
			"Open Account, then API keys",
			"Create a key and copy it",
		},
	},
	"nvidia": {
		URL: "https://build.nvidia.com/settings/api-keys",
		Steps: []string{
			"Sign in at build.nvidia.com",
			"Open Settings, then API keys",
			"Copy the key — it starts with nvapi-",
		},
	},
	"omniroute": {
		URL: "https://api.cheaperinference.com",
		Steps: []string{
			"Sign in to the Omniroute dashboard",
			"Issue an API key for your account",
			"Copy it into the field below",
		},
	},
}

// localProviderIDs are the runtimes that answer on this machine and
// authenticate with nothing. Mirrors config.IsLocalProvider; kept here
// so the wizard's key step and its "no key needed" blurbs cannot
// disagree — the wizard used to ask ollama and flm for a key the
// moment after promising they needed none.
var localProviderIDs = map[string]bool{"local": true, "ollama": true, "flm": true}

// providerNeedsKey reports whether a provider authenticates with an API
// key.
func providerNeedsKey(provider string) bool {
	return !localProviderIDs[provider]
}

// keyStepsFor returns a provider's key journey. ok is false for the
// local runtimes, which need no key at all.
func keyStepsFor(provider string) (providerKeyStep, bool) {
	if !providerNeedsKey(provider) {
		return providerKeyStep{}, false
	}
	step, ok := providerKeySteps[provider]
	return step, ok
}

// keyRemedyLine is the one-line fix for a provider that authenticates
// with a key: where to get one, and which key opens the wizard. Empty for
// a local runtime, which has nothing to fix.
func keyRemedyLine(provider string) string {
	steps, ok := keyStepsFor(provider)
	if !ok {
		return ""
	}
	return fmt.Sprintf("Get a key: %s  ·  press l to enter it", steps.URL)
}

// readinessNotice composes the durable transcript notice for a provider
// verdict. One home for the wording: the same sentence lands in the
// transcript and in the Logs tab, and a credential failure carries its
// fix. The boot probe is the precheck — it runs before the first prompt,
// so a bad key is a direction rather than a dead end discovered by a
// failed turn.
func readinessNotice(provider string, readiness int, message string) string {
	switch readiness {
	case 1: // ProviderReady
		return "Provider ready: " + message
	case 2: // ProviderWarning
		return "Provider warning: " + message
	case 3: // ProviderUnavailable
		return "Provider unavailable: " + message
	case 4: // ProviderMisconfigured
		notice := "Provider misconfigured: " + message
		if remedy := keyRemedyLine(provider); remedy != "" {
			notice += "\n" + remedy
		}
		return notice
	}
	return ""
}
