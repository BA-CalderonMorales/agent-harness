package record

import "strings"

// checkRule names a real check command: Bin invoked with one of Subs as its
// subcommand (any invocation when Subs is empty). SkipRun lets `npm run test`
// match like `npm test`; Flag requires an argument, as in `gofmt -l`.
type checkRule struct {
	Bin     string
	Subs    []string
	SkipRun bool
	Flag    string
}

var checkRules = []checkRule{
	{Bin: "go", Subs: []string{"test", "vet", "build"}},
	{Bin: "make", Subs: []string{"test", "verify", "build", "lint", "check"}},
	{Bin: "npm", Subs: []string{"test", "build", "lint"}, SkipRun: true},
	{Bin: "pnpm", Subs: []string{"test", "build", "lint"}, SkipRun: true},
	{Bin: "yarn", Subs: []string{"test", "build", "lint"}, SkipRun: true},
	{Bin: "pytest"},
	{Bin: "cargo", Subs: []string{"test", "build", "check", "clippy"}},
	{Bin: "golangci-lint"},
	{Bin: "staticcheck"},
	{Bin: "gofmt", Flag: "-l"},
	{Bin: "tsc"},
	{Bin: "mvn", Subs: []string{"test"}},
	{Bin: "gradle", Subs: []string{"test", "build"}},
}

// isCheckCommand reports whether any command in a shell line (split at
// && || ; | and newlines) is a real check. A word like "test" or "build"
// appearing as a path or argument elsewhere does not count.
func isCheckCommand(line string) bool {
	line = strings.NewReplacer("&&", "\n", "||", "\n", ";", "\n", "|", "\n").Replace(line)
	for _, segment := range strings.Split(line, "\n") {
		if isCheckSegment(strings.Fields(segment)) {
			return true
		}
	}
	return false
}

func isCheckSegment(tokens []string) bool {
	for len(tokens) > 0 && strings.Contains(tokens[0], "=") && !strings.HasPrefix(tokens[0], "-") {
		tokens = tokens[1:] // leading VAR=value assignments
	}
	if len(tokens) == 0 {
		return false
	}
	for _, rule := range checkRules {
		if tokens[0] != rule.Bin {
			continue
		}
		args := tokens[1:]
		if rule.SkipRun && len(args) > 0 && args[0] == "run" {
			args = args[1:]
		}
		if rule.Flag != "" {
			for _, a := range args {
				if a == rule.Flag {
					return true
				}
			}
			return false
		}
		if len(rule.Subs) == 0 {
			return true
		}
		if len(args) == 0 {
			return false
		}
		for _, sub := range rule.Subs {
			if args[0] == sub {
				return true
			}
		}
		return false
	}
	return false
}
