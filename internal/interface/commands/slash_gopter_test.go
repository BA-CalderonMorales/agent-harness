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

	properties.Property("Feature-flagged and registered commands parity across all discovery surfaces", prop.ForAll(
		func(regName string, flagName string) bool {
			if regName == "" || flagName == "" || regName == flagName {
				return true
			}
			reg := NewSlashRegistry()
			reg.Register(regName, "registered desc", func(_ string) (string, error) { return "ok", nil })
			reg.FeatureFlag(flagName, "feature flagged desc")

			help := reg.GetHelp()
			comps := reg.GetCompletions()
			descs := reg.GetCompletionDescriptions()
			infos := reg.GetCommandInfos()

			// 1. Registered command MUST be present in all surfaces
			registeredPresent := strings.Contains(help, "/"+regName)
			foundComp := false
			for _, c := range comps {
				if c == "/"+regName {
					foundComp = true
					break
				}
			}
			descExists := descs["/"+regName] == "registered desc"
			infoFound := false
			for _, info := range infos {
				if info.Command == "/"+regName && info.Description == "registered desc" {
					infoFound = true
					break
				}
			}

			if !registeredPresent || !foundComp || !descExists || !infoFound {
				return false
			}

			// 2. Feature-flagged command MUST be absent from all user-facing discovery surfaces
			flaggedPresentInHelp := strings.Contains(help, "/"+flagName)
			flaggedPresentInComps := false
			for _, c := range comps {
				if c == "/"+flagName {
					flaggedPresentInComps = true
					break
				}
			}
			flaggedPresentInDescs := descs["/"+flagName] != ""
			flaggedPresentInInfos := false
			for _, info := range infos {
				if info.Command == "/"+flagName {
					flaggedPresentInInfos = true
					break
				}
			}

			return !flaggedPresentInHelp && !flaggedPresentInComps && !flaggedPresentInDescs && !flaggedPresentInInfos
		},
		gen.AlphaString(),
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
