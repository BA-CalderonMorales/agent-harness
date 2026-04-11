package defaults

// Git bucket defaults
const (
	// Command timeouts
	GitCommandTimeoutSecs = 30
	GitLogMaxEntries      = 50
	GitDiffMaxLines       = 500
	GitStatusMaxFiles     = 100

	// Safety
	GitRequireApprovalForPush  = true
	GitRequireApprovalForForce = true
)

// GitSafeCommands are read-only git operations
var GitSafeCommands = []string{
	"status", "log", "diff", "show", "branch", "remote", "tag",
	"stash list", "config --list", "rev-parse", "ls-files",
}

// GitDestructiveCommands require approval
var GitDestructiveCommands = []string{
	"push", "push --force", "push -f",
	"reset", "reset --hard", "reset --soft",
	"clean", "clean -f", "clean -fd",
	"rebase", "merge", "cherry-pick",
	"stash drop", "stash pop", "stash clear",
}

// GitStatusIndicators maps file prefixes to status
var GitStatusIndicators = map[string]string{
	"M":  "modified",
	"A":  "added",
	"D":  "deleted",
	"R":  "renamed",
	"C":  "copied",
	"U":  "updated",
	"??": "untracked",
}
