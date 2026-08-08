package commands

import (
	"fmt"
	"strings"
)

// DiffHandler shows git diff
func DiffHandler(getDiff func() string) SlashHandler {
	return func(args string) (string, error) {
		diff := getDiff()
		if diff == "" {
			return "No changes detected in workspace.", nil
		}
		return diff, nil
	}
}

// CommitHandler stages changes and commits with the given message.
func CommitHandler(commitFn func(message string) (string, error)) SlashHandler {
	return func(args string) (string, error) {
		if args == "" {
			return "Usage: /commit <message>\nStages all changes and commits.", nil
		}
		return commitFn(args)
	}
}

// BranchHandler handles git branch operations.
func BranchHandler(listFn func() (string, error), createFn func(string) (string, error), switchFn func(string) (string, error), deleteFn func(string) (string, error)) SlashHandler {
	return func(args string) (string, error) {
		if args == "" || args == "list" {
			return listFn()
		}
		parts := strings.SplitN(args, " ", 2)
		subcmd := parts[0]
		name := ""
		if len(parts) > 1 {
			name = parts[1]
		}
		switch subcmd {
		case "create":
			if name == "" {
				return "Usage: /branch create <name>", nil
			}
			return createFn(name)
		case "switch":
			if name == "" {
				return "Usage: /branch switch <name>", nil
			}
			return switchFn(name)
		case "delete":
			if name == "" {
				return "Usage: /branch delete <name>", nil
			}
			return deleteFn(name)
		default:
			return fmt.Sprintf("Unknown branch command: %s\nUsage: /branch [list|create <name>|switch <name>|delete <name>]", subcmd), nil
		}
	}
}

// PlanHandler toggles plan mode.

// PRHandler handles pull request operations via gh CLI.
func PRHandler(createFn func(title, body string) (string, error), listFn func() (string, error)) SlashHandler {
	return func(args string) (string, error) {
		if args == "" || args == "list" {
			return listFn()
		}
		// /pr create "title" [body]
		if strings.HasPrefix(args, "create ") {
			rest := strings.TrimPrefix(args, "create ")
			if rest == "" {
				return "Usage: /pr create \"title\" [body]", nil
			}
			// Simple parsing: first quoted string is title, rest is body
			if strings.HasPrefix(rest, "\"") {
				end := strings.Index(rest[1:], "\"")
				if end == -1 {
					return "Usage: /pr create \"title\" [body]", nil
				}
				title := rest[1 : end+1]
				body := strings.TrimSpace(rest[end+2:])
				return createFn(title, body)
			}
			// Unquoted: first word is title
			parts := strings.SplitN(rest, " ", 2)
			title := parts[0]
			body := ""
			if len(parts) > 1 {
				body = parts[1]
			}
			return createFn(title, body)
		}
		return "Usage: /pr [list|create \"title\" [body]]", nil
	}
}

// InitHandler scaffolds a new project with standard files.

// WorktreeHandler handles git worktree commands
func WorktreeHandler(listFn func() (string, error), addFn func(path, branch string) (string, error), removeFn func(path string) (string, error)) SlashHandler {
	return func(args string) (string, error) {
		if args == "" || args == "list" {
			return listFn()
		}
		parts := strings.SplitN(args, " ", 2)
		switch parts[0] {
		case "add":
			if len(parts) < 2 {
				return "Usage: /worktree add <path> [branch]", nil
			}
			addParts := strings.SplitN(parts[1], " ", 2)
			path := addParts[0]
			branch := ""
			if len(addParts) > 1 {
				branch = addParts[1]
			}
			return addFn(path, branch)
		case "remove":
			if len(parts) < 2 {
				return "Usage: /worktree remove <path>", nil
			}
			return removeFn(parts[1])
		default:
			return "Usage: /worktree [list|add <path> [branch]|remove <path>]", nil
		}
	}
}

// AgentsHandler handles agent-related commands
