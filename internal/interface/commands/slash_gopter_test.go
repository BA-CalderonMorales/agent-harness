package commands

import (
	"strings"
	"testing"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

func TestSlashCommandsPropertyBased(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MaxSize = 100
	properties := gopter.NewProperties(parameters)

	properties.Property("ParseSlashCommand: input starting with / always produces a valid Name", prop.ForAll(
		func(input string) bool {
			cmd := ParseSlashCommand("/" + input)
			return cmd.Name != "" || input == ""
		},
		gen.AlphaString(),
	))

	properties.Property("ParseSlashCommand: Raw matches trimmed input", prop.ForAll(
		func(input string) bool {
			cmd := ParseSlashCommand("/" + input)
			trimmed := strings.TrimLeft(input, " ")
			return cmd.Raw == trimmed
		},
		gen.AlphaString(),
	))

	properties.Property("FeatureFlag followed by IsFeatureFlagged returns true", prop.ForAll(
		func(name string) bool {
			if name == "" {
				return true
			}
			reg := NewSlashRegistry()
			reg.FeatureFlag(name, "Coming soon")
			return reg.IsFeatureFlagged(name)
		},
		gen.AlphaString(),
	))

	properties.Property("Handle on a feature-flagged command returns coming soon", prop.ForAll(
		func(name string) bool {
			if name == "" {
				return true
			}
			reg := NewSlashRegistry()
			reg.FeatureFlag(name, "Coming soon")
			result, handled, err := reg.Handle("/" + name)
			return err == nil && handled && strings.Contains(result, "Coming soon")
		},
		gen.AlphaString(),
	))

	properties.Property("GetHelp includes registered commands", prop.ForAll(
		func(name string) bool {
			if name == "" {
				return true
			}
			reg := NewSlashRegistry()
			reg.Register(name, "desc", func(_ string) (string, error) { return "ok", nil })
			help := reg.GetHelp()
			return strings.Contains(help, "/"+name)
		},
		gen.AlphaString(),
	))

	properties.Property("Feature-flagged commands are excluded from GetHelp, GetCompletions, and GetCompletionDescriptions", prop.ForAll(
		func(name string) bool {
			if name == "" {
				return true
			}
			reg := NewSlashRegistry()
			reg.FeatureFlag(name, "Hidden coming soon command")
			help := reg.GetHelp()
			comps := reg.GetCompletions()
			descs := reg.GetCompletionDescriptions()
			similar := reg.findSimilar(name)

			// Feature-flagged command must NOT show up to end users
			if strings.Contains(help, "/"+name) {
				return false
			}
			for _, c := range comps {
				if strings.TrimPrefix(c, "/") == name {
					return false
				}
			}
			if _, exists := descs["/"+name]; exists {
				return false
			}
			for _, s := range similar {
				if strings.Contains(s, name) {
					return false
				}
			}
			return true
		},
		gen.AlphaString(),
	))

	properties.Property("Registered command dispatch succeeds", prop.ForAll(
		func(name string) bool {
			if name == "" {
				return true
			}
			reg := NewSlashRegistry()
			reg.Register(name, "desc", func(_ string) (string, error) { return "response:" + name, nil })
			result, handled, err := reg.Handle("/" + name)
			return err == nil && handled && result == "response:"+name
		},
		gen.AlphaString(),
	))

	properties.TestingRun(t)
}
