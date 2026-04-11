package defaults

import (
	"regexp"
	"time"
)

// Shell bucket defaults
const (
	// Timeouts
	ShellDefaultTimeout     = 60 * time.Second
	ShellMaxTimeout         = 300 * time.Second // 5 minutes max
	ShellDefaultTimeoutSecs = 60

	// Output limits
	ShellMaxOutputSize     = 1024 * 1024 // 1MB
	ShellMaxOutputSizeText = "1MB"

	// Command validation
	ShellMaxCommandLength = 10000
)

// ShellBlockedCommands contains dangerous command patterns
var ShellBlockedCommands = []string{
	"rm -rf /",
	"mkfs",
	"fdisk",
	"dd if=/dev/zero",
	":(){ :|:& };:", // fork bomb
	"> /dev/sda",
	"> /dev/hda",
	"chmod -R 777 /",
	"chmod -R 777 ~",
	"del /f /s /q",
	"format c:",
}

// ShellBlockedPatterns contains regex patterns for dangerous commands
var ShellBlockedPatterns = []*regexp.Regexp{
	regexp.MustCompile(`rm\s+-rf\s+/`),
	regexp.MustCompile(`>\s*/dev/[sh]da`),
	regexp.MustCompile(`:\s*\(\s*\)\s*{\s*:\s*\|\s*:\s*&\s*}`), // fork bomb
	regexp.MustCompile(`curl.*\|.*bash`),                       // pipe curl to bash
	regexp.MustCompile(`wget.*-O-.*\|.*bash`),                  // pipe wget to bash
	regexp.MustCompile(`eval\s*\(`),
	regexp.MustCompile(`base64\s+-d.*\|`),
}

// ShellDestructivePatterns detects potentially destructive commands
var ShellDestructivePatterns = []string{
	`\brm\b`,
	`\bmv\b`,
	`\bcp\b`,
	`\bdd\b`,
	`\bmkfs\b`,
	`\bfdisk\b`,
	`\bparted\b`,
}

// ShellSafeReadOnlyCommands are safe commands that don't modify state
var ShellSafeReadOnlyCommands = []string{
	"ls", "cat", "head", "tail", "less", "more",
	"find", "grep", "which", "whereis", "file",
	"pwd", "echo", "printf", "date", "whoami",
	"uname", "id", "groups", "env", "printenv",
	"stat", "wc", "sort", "uniq", "cut",
	"ps", "top", "htop", "df", "du",
	"git status", "git log", "git branch", "git diff",
	"npm list", "npm view", "go version", "go env",
}
