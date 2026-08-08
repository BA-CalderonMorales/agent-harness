package main

import (
	"github.com/BA-CalderonMorales/agent-harness/internal/interface/commands"
	"github.com/BA-CalderonMorales/agent-harness/pkg/git"
	"strings"
)

func (app *App) initCommandsGit() {
	app.cmdRegistry.Register("pr", "Manage pull requests",
		commands.PRHandler(
			func(title, body string) (string, error) {
				if err := app.requireGitRepo(); err != nil {
					return "", err
				}
				repo := git.NewRepo(app.cwd)
				return repo.CreatePR(title, body)
			},
			func() (string, error) {
				if err := app.requireGitRepo(); err != nil {
					return "", err
				}
				repo := git.NewRepo(app.cwd)
				return repo.ListPRs()
			},
		))

	app.cmdRegistry.Register("branch", "Manage git branches",
		commands.BranchHandler(
			func() (string, error) {
				if err := app.requireGitRepo(); err != nil {
					return "", err
				}
				repo := git.NewRepo(app.cwd)
				branches, err := repo.ListBranches()
				if err != nil {
					return "", err
				}
				return strings.Join(branches, "\n"), nil
			},
			func(name string) (string, error) {
				if err := app.requireGitRepo(); err != nil {
					return "", err
				}
				repo := git.NewRepo(app.cwd)
				if err := repo.CreateBranch(name); err != nil {
					return "", err
				}
				return sprintf("Created and switched to branch %s", name), nil
			},
			func(name string) (string, error) {
				if err := app.requireGitRepo(); err != nil {
					return "", err
				}
				repo := git.NewRepo(app.cwd)
				if err := repo.SwitchBranch(name); err != nil {
					return "", err
				}
				return sprintf("Switched to branch %s", name), nil
			},
			func(name string) (string, error) {
				if err := app.requireGitRepo(); err != nil {
					return "", err
				}
				repo := git.NewRepo(app.cwd)
				if err := repo.DeleteBranch(name); err != nil {
					return "", err
				}
				return sprintf("Deleted branch %s", name), nil
			},
		))

	app.cmdRegistry.Register("diff", "Show git working tree changes",
		commands.DiffHandler(func() string {
			if err := app.requireGitRepo(); err != nil {
				return "Not in a git repository."
			}
			repo := git.NewRepo(app.cwd)
			diff, err := repo.Diff()
			if err != nil {
				return sprintf("Failed to get diff: %v", err)
			}
			return diff
		}))

	app.cmdRegistry.Register("commit", "Stage and commit changes",
		commands.CommitHandler(func(msg string) (string, error) {
			if err := app.requireGitRepo(); err != nil {
				return "", err
			}
			repo := git.NewRepo(app.cwd)
			if err := repo.Add("."); err != nil {
				return "", err
			}
			if err := repo.Commit(msg); err != nil {
				return "", err
			}
			return sprintf("Committed changes with message: %s", msg), nil
		}))

}
