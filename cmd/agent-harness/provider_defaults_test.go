package main

import (
	"testing"

	"github.com/BA-CalderonMorales/agent-harness/internal/core/config"
)

// Every provider's default model must be a model that provider's catalog
// lists. A default outside the catalog resets or 404s: the NVIDIA default
// named an unserved id and the Fireworks default a retired one, and both
// stranded a fresh login on a model the endpoint rejected.
//
// The catalog is getModelsForProvider; openrouter is its default branch,
// so there is a catalog for every provider the login dialog offers.
func TestEveryProviderDefaultModelIsInItsCatalog(t *testing.T) {
	providers := []string{
		"openai", "anthropic", "ollama", "lemonade", "flm", "local",
		"openrouter", "fireworks", "nvidia", "omniroute",
	}
	for _, provider := range providers {
		t.Run(provider, func(t *testing.T) {
			def := config.DefaultModelForProvider(provider)
			if def == "" {
				t.Fatal("no default model")
			}
			for _, item := range getModelsForProvider(provider, def) {
				if item.ID == def {
					if !item.IsDefault {
						t.Errorf("default %q is in the catalog but not marked default", def)
					}
					return
				}
			}
			t.Fatalf("default model %q is not in the %s catalog", def, provider)
		})
	}
}
