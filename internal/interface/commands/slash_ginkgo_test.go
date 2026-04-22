package commands

import (
	"errors"
	"strings"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestSlashCommands(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Slash Commands Suite")
}

var _ = Describe("Slash Commands", func() {

	// ========================================================================
	// Registry and Parsing
	// ========================================================================
	Describe("SlashRegistry", func() {
		var registry *SlashRegistry

		BeforeEach(func() {
			registry = NewSlashRegistry()
		})

		Context("Given an empty registry", func() {
			It("should not handle non-slash input", func() {
				By("when receiving plain text 'hello'")
				result, handled, err := registry.Handle("hello")
				Expect(err).ToNot(HaveOccurred())
				Expect(handled).To(BeFalse())
				Expect(result).To(BeEmpty())
			})

			It("should return unknown command for unregistered slash", func() {
				By("when receiving /foo")
				result, handled, err := registry.Handle("/foo")
				Expect(err).ToNot(HaveOccurred())
				Expect(handled).To(BeTrue())
				Expect(result).To(ContainSubstring("Unknown command: /foo"))
			})

			It("should suggest similar commands by prefix", func() {
				By("given registered commands /help and /hello")
				registry.Register("help", "Show help", func(string) (string, error) { return "", nil })
				registry.Register("hello", "Say hello", func(string) (string, error) { return "", nil })

				By("when receiving /hel")
				result, handled, err := registry.Handle("/hel")
				Expect(err).ToNot(HaveOccurred())
				Expect(handled).To(BeTrue())
				Expect(result).To(ContainSubstring("Did you mean"))
			})

			It("should suggest similar commands by substring", func() {
				By("given registered command /quit")
				registry.Register("quit", "Quit", func(string) (string, error) { return "", nil })

				By("when receiving /qu")
				result, handled, err := registry.Handle("/qu")
				Expect(err).ToNot(HaveOccurred())
				Expect(handled).To(BeTrue())
				Expect(result).To(ContainSubstring("/quit"))
			})

			It("should handle bare slash input", func() {
				By("when receiving '/'")
				result, handled, err := registry.Handle("/")
				Expect(err).ToNot(HaveOccurred())
				Expect(handled).To(BeTrue())
				Expect(result).To(ContainSubstring("Unknown command"))
			})

			It("should suggest Type /help when no similar commands exist", func() {
				By("given a registry with no commands")
				By("when receiving /xyz")
				result, handled, err := registry.Handle("/xyz")
				Expect(err).ToNot(HaveOccurred())
				Expect(handled).To(BeTrue())
				Expect(result).To(ContainSubstring("Type /help"))
			})
		})

		Context("Given a populated registry", func() {
			BeforeEach(func() {
				registry.Register("help", "Show help", func(string) (string, error) { return "help text", nil })
				registry.Register("quit", "Quit", func(string) (string, error) { return "__QUIT__", nil })
				registry.Register("compact", "Compact", func(string) (string, error) { return "compacted", nil })
			})

			It("should dispatch exact matches", func() {
				By("when receiving /help")
				result, handled, err := registry.Handle("/help")
				Expect(err).ToNot(HaveOccurred())
				Expect(handled).To(BeTrue())
				Expect(result).To(Equal("help text"))
			})

			It("should return deterministic help text", func() {
				By("when generating help twice")
				help1 := registry.GetHelp()
				help2 := registry.GetHelp()
				Expect(help1).To(Equal(help2))
				Expect(help1).To(ContainSubstring("Available commands:"))
			})

			It("should include uncategorized commands in Other section", func() {
				By("registering an uncategorized command")
				registry.Register("newcmd", "A new command", func(string) (string, error) { return "", nil })

				By("generating help")
				help := registry.GetHelp()
				Expect(help).To(ContainSubstring("Other:"))
				Expect(help).To(ContainSubstring("/newcmd"))
			})

			It("should filter aliases from completions and sort", func() {
				By("given commands including an alias")
				registry.Register("exit", "Exit alias", func(string) (string, error) { return "", nil })
				registry.Register("abc", "ABC", func(string) (string, error) { return "", nil })

				By("when requesting completions")
				comps := registry.GetCompletions()
				Expect(comps).ToNot(ContainElement("/exit"))
				for i := 1; i < len(comps); i++ {
					Expect(comps[i] >= comps[i-1]).To(BeTrue(), "completions not sorted: %v", comps)
				}
			})
		})
	})

	Describe("ParseSlashCommand", func() {
		DescribeTable("should parse commands correctly",
			func(input, wantName, wantArgs string) {
				By("when parsing " + input)
				cmd := ParseSlashCommand(input)
				Expect(cmd.Name).To(Equal(wantName))
				Expect(cmd.Args).To(Equal(wantArgs))
			},
			Entry("model with arg", "/model gpt-4o", "model", "gpt-4o"),
			Entry("clear no arg", "/clear", "clear", ""),
			Entry("clear with confirm", "/clear   --confirm", "clear", "--confirm"),
			Entry("model with spaces", "/model  claude-3-5-sonnet  with spaces", "model", "claude-3-5-sonnet  with spaces"),
			Entry("reset confirm", "/reset --confirm", "reset", "--confirm"),
			Entry("session load", "/session load abc-123", "session", "load abc-123"),
			Entry("slash with trailing spaces", "/   ", "", ""),
			Entry("slash with leading space before command", "/ help", "help", ""),
			Entry("slash with leading space and args", "/ model gpt", "model", "gpt"),
		)
	})

	// ========================================================================
	// Session Commands
	// ========================================================================
	Describe("HelpHandler", func() {
		var registry *SlashRegistry
		var handler SlashHandler

		BeforeEach(func() {
			registry = NewSlashRegistry()
			registry.Register("help", "Show available commands", HelpHandler(registry))
			registry.Register("quit", "Exit the application", QuitHandler())
			handler = HelpHandler(registry)
		})

		Context("Given a registry with commands", func() {
			It("should return full help when no args provided", func() {
				By("when invoking /help")
				result, err := handler("")
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(ContainSubstring("Available commands:"))
			})

			It("should return specific command help when arg provided", func() {
				By("when invoking /help quit")
				result, err := handler("quit")
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(Equal("/quit - Exit the application"))
			})

			It("should return unknown for invalid command arg", func() {
				By("when invoking /help foo")
				result, err := handler("foo")
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(Equal("Unknown command: /foo"))
			})

			It("should not resolve hidden aliases", func() {
				By("when invoking /help exit")
				result, err := handler("exit")
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(Equal("Unknown command: /exit"))
			})
		})
	})

	Describe("StatusHandler", func() {
		It("should return the current status", func() {
			By("given a status function returning session info")
			handler := StatusHandler(func() string { return "session: active\nmodel: gpt-4o" })

			By("when invoking /status")
			result, err := handler("")
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(Equal("session: active\nmodel: gpt-4o"))
		})
	})

	Describe("ClearHandler", func() {
		It("should clear session and chat when both callbacks provided", func() {
			var cleared, chatCleared bool

			By("given a clear handler with session and chat callbacks")
			handler := ClearHandler(
				func() error { cleared = true; return nil },
				func(msg string) { chatCleared = true },
			)

			By("when invoking /clear")
			result, err := handler("")
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(BeEmpty())
			Expect(cleared).To(BeTrue())
			Expect(chatCleared).To(BeTrue())
		})

		It("should return confirmation when only session callback provided", func() {
			By("given a clear handler without chat callback")
			handler := ClearHandler(func() error { return nil }, nil)

			By("when invoking /clear")
			result, err := handler("")
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(Equal("Session cleared."))
		})

		It("should propagate errors from clear function", func() {
			By("given a failing clear function")
			handler := ClearHandler(func() error { return errors.New("disk full") }, nil)

			By("when invoking /clear")
			_, err := handler("")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("disk full"))
		})
	})

	Describe("CompactHandler", func() {
		It("should return compaction result", func() {
			By("given a compaction function")
			handler := CompactHandler(func() (string, error) { return "Compacted: removed 5 messages", nil })

			By("when invoking /compact")
			result, err := handler("")
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(Equal("Compacted: removed 5 messages"))
		})

		It("should propagate compaction errors", func() {
			By("given a failing compaction function")
			handler := CompactHandler(func() (string, error) { return "", errors.New("compaction failed") })

			By("when invoking /compact")
			_, err := handler("")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("compaction failed"))
		})
	})

	Describe("CostHandler", func() {
		It("should return cost information", func() {
			By("given a cost function")
			handler := CostHandler(func() string { return "Cost: $0.42" })

			By("when invoking /cost")
			result, err := handler("")
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(Equal("Cost: $0.42"))
		})
	})

	// ========================================================================
	// Model Commands
	// ========================================================================
	Describe("CurrentModelHandler", func() {
		It("should display the current model", func() {
			By("given a model getter returning gpt-4o")
			handler := CurrentModelHandler(func() string { return "gpt-4o" })

			By("when invoking /current-model")
			result, err := handler("")
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(Equal("Current model: gpt-4o"))
		})
	})

	Describe("ModelHandler", func() {
		var current string
		var handler SlashHandler

		BeforeEach(func() {
			current = "gpt-4o"
			handler = ModelHandler(
				func() string { return current },
				func(m string) error { current = m; return nil },
				func() []string { return []string{"gpt-4o", "claude-3-5-sonnet"} },
			)
		})

		Context("Given no arguments", func() {
			It("should list models with current marked", func() {
				By("when invoking /model")
				result, err := handler("")
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(ContainSubstring("gpt-4o"))
				Expect(result).To(ContainSubstring("● gpt-4o"))
				Expect(result).To(ContainSubstring("claude-3-5-sonnet"))
			})
		})

		Context("Given a model name argument", func() {
			It("should switch to the specified model", func() {
				By("when invoking /model claude-3-5-sonnet")
				result, err := handler("claude-3-5-sonnet")
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(ContainSubstring("Model updated"))
				Expect(current).To(Equal("claude-3-5-sonnet"))
			})
		})

		Context("Given a failing setModel function", func() {
			It("should propagate the error", func() {
				By("given a handler with invalid model rejection")
				failHandler := ModelHandler(
					func() string { return "gpt-4o" },
					func(m string) error { return errors.New("invalid model") },
					func() []string { return nil },
				)

				By("when invoking /model bad-model")
				_, err := failHandler("bad-model")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal("invalid model"))
			})
		})

		Context("Given empty model list", func() {
			It("should show current model only", func() {
				By("given a handler with empty model list")
				handler := ModelHandler(
					func() string { return "gpt-4o" },
					func(m string) error { return nil },
					func() []string { return []string{} },
				)

				By("when invoking /model")
				result, err := handler("")
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(ContainSubstring("Current"))
				Expect(result).To(ContainSubstring("gpt-4o"))
			})
		})

		Context("Given multiple models", func() {
			It("should mark non-current models with spaces not bullet", func() {
				By("given a handler with multiple models")
				handler := ModelHandler(
					func() string { return "gpt-4o" },
					func(m string) error { return nil },
					func() []string { return []string{"gpt-4o", "claude-3-5-sonnet"} },
				)

				By("when invoking /model")
				result, err := handler("")
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(ContainSubstring("● gpt-4o"))
				Expect(result).To(ContainSubstring("  claude-3-5-sonnet"))
			})
		})
	})

	// ========================================================================
	// Settings Commands
	// ========================================================================
	Describe("PermissionsHandler", func() {
		var current string
		var handler SlashHandler

		BeforeEach(func() {
			current = "read-only"
			handler = PermissionsHandler(
				func() string { return current },
				func(m string) error { current = m; return nil },
				func() string { return "Mode: read-only" },
			)
		})

		Context("Given no arguments", func() {
			It("should show current permission report", func() {
				By("when invoking /permissions")
				result, err := handler("")
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(Equal("Mode: read-only"))
			})
		})

		Context("Given a mode argument", func() {
			It("should switch permission mode", func() {
				By("when invoking /permissions workspace-write")
				result, err := handler("workspace-write")
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(ContainSubstring("Permissions updated"))
				Expect(current).To(Equal("workspace-write"))
			})
		})

		Context("Given a failing setMode function", func() {
			It("should propagate the error", func() {
				By("given a handler rejecting invalid modes")
				failHandler := PermissionsHandler(
					func() string { return "read-only" },
					func(m string) error { return errors.New("invalid mode") },
					func() string { return "" },
				)

				By("when invoking /permissions bad-mode")
				_, err := failHandler("bad-mode")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal("invalid mode"))
			})
		})
	})

	Describe("ConfigHandler", func() {
		It("should return configuration", func() {
			By("given a config function")
			handler := ConfigHandler(func() string { return "provider: openrouter\nmodel: gpt-4o" })

			By("when invoking /config")
			result, err := handler("")
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(Equal("provider: openrouter\nmodel: gpt-4o"))
		})
	})

	// ========================================================================
	// Output Commands
	// ========================================================================
	Describe("ExportHandler", func() {
		It("should export to specified path", func() {
			By("given an export handler")
			handler := ExportHandler(func(path string) (string, error) {
				if path == "" {
					path = "session-default.json"
				}
				return path, nil
			})

			By("when invoking /export my-export.json")
			result, err := handler("my-export.json")
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(ContainSubstring("my-export.json"))

			By("when invoking /export with no args")
			result, err = handler("")
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(ContainSubstring("session-default.json"))
		})

		It("should propagate export errors", func() {
			By("given a failing export function")
			handler := ExportHandler(func(path string) (string, error) { return "", errors.New("write failed") })

			By("when invoking /export /bad/path")
			_, err := handler("/bad/path")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("write failed"))
		})
	})

	Describe("DiffHandler", func() {
		It("should return diff when changes exist", func() {
			By("given a diff function with changes")
			handler := DiffHandler(func() string { return "+added line" })

			By("when invoking /diff")
			result, err := handler("")
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(Equal("+added line"))
		})

		It("should indicate no changes when diff is empty", func() {
			By("given a diff function with no changes")
			handler := DiffHandler(func() string { return "" })

			By("when invoking /diff")
			result, err := handler("")
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(Equal("No changes detected in workspace."))
		})
	})

	Describe("VersionHandler", func() {
		It("should include build info when provided", func() {
			By("given a version handler with build info")
			handler := VersionHandler("0.1.4", "Built: 2024-01-01")

			By("when invoking /version")
			result, err := handler("")
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(Equal("agent-harness 0.1.4\nBuilt: 2024-01-01"))
		})

		It("should show version only when no build info", func() {
			By("given a version handler without build info")
			handler := VersionHandler("0.1.4", "")

			By("when invoking /version")
			result, err := handler("")
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(Equal("agent-harness 0.1.4"))
		})
	})

	// ========================================================================
	// Git Commands
	// ========================================================================
	Describe("CommitHandler", func() {
		It("should commit with a message", func() {
			By("given a commit handler")
			handler := CommitHandler(func(message string) (string, error) {
				return "Committed to main: " + message, nil
			})

			By("when invoking /commit 'fix bug'")
			result, err := handler("fix bug")
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(Equal("Committed to main: fix bug"))
		})

		It("should show usage when no message provided", func() {
			By("given a commit handler")
			handler := CommitHandler(func(message string) (string, error) { return "", nil })

			By("when invoking /commit with no args")
			result, err := handler("")
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(ContainSubstring("Usage: /commit <message>"))
		})

		It("should propagate commit errors", func() {
			By("given a failing commit function")
			handler := CommitHandler(func(message string) (string, error) {
				return "", errors.New("nothing to commit")
			})

			By("when invoking /commit 'empty'")
			_, err := handler("empty")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("nothing to commit"))
		})
	})

	Describe("BranchHandler", func() {
		var (
			branches []string
			created  string
			switched string
			deleted  string
			handler  SlashHandler
		)

		BeforeEach(func() {
			branches = []string{"main", "develop", "feature/x"}
			created = ""
			switched = ""
			deleted = ""
			handler = BranchHandler(
				func() (string, error) { return strings.Join(branches, "\n"), nil },
				func(name string) (string, error) { created = name; return "created", nil },
				func(name string) (string, error) { switched = name; return "switched", nil },
				func(name string) (string, error) { deleted = name; return "deleted", nil },
			)
		})

		Context("Given no arguments", func() {
			It("should list branches", func() {
				By("when invoking /branch")
				result, err := handler("")
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(ContainSubstring("main"))
			})

			It("should list branches with list subcommand", func() {
				By("when invoking /branch list")
				result, err := handler("list")
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(ContainSubstring("main"))
			})
		})

		Context("Given create subcommand", func() {
			It("should create a branch", func() {
				By("when invoking /branch create feat-1")
				result, err := handler("create feat-1")
				Expect(err).ToNot(HaveOccurred())
				Expect(created).To(Equal("feat-1"))
				Expect(result).To(Equal("created"))
			})

			It("should show usage when name missing", func() {
				By("when invoking /branch create")
				result, err := handler("create")
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(ContainSubstring("Usage: /branch create <name>"))
			})
		})

		Context("Given switch subcommand", func() {
			It("should switch to a branch", func() {
				By("when invoking /branch switch develop")
				_, err := handler("switch develop")
				Expect(err).ToNot(HaveOccurred())
				Expect(switched).To(Equal("develop"))
			})

			It("should show usage when name missing", func() {
				By("when invoking /branch switch")
				result, err := handler("switch")
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(ContainSubstring("Usage: /branch switch <name>"))
			})
		})

		Context("Given delete subcommand", func() {
			It("should delete a branch", func() {
				By("when invoking /branch delete old-branch")
				_, err := handler("delete old-branch")
				Expect(err).ToNot(HaveOccurred())
				Expect(deleted).To(Equal("old-branch"))
			})

			It("should show usage when name missing", func() {
				By("when invoking /branch delete")
				result, err := handler("delete")
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(ContainSubstring("Usage: /branch delete <name>"))
			})
		})

		Context("Given unknown subcommand", func() {
			It("should return usage instructions", func() {
				By("when invoking /branch foo")
				result, err := handler("foo")
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(ContainSubstring("Unknown branch command: foo"))
			})
		})

		Context("Given failing callback functions", func() {
			It("should propagate listFn errors", func() {
				By("given a handler with failing list")
				failHandler := BranchHandler(
					func() (string, error) { return "", errors.New("git error") },
					func(string) (string, error) { return "", nil },
					func(string) (string, error) { return "", nil },
					func(string) (string, error) { return "", nil },
				)

				By("when invoking /branch")
				_, err := failHandler("")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal("git error"))
			})

			It("should propagate createFn errors", func() {
				By("given a handler with failing create")
				failHandler := BranchHandler(
					func() (string, error) { return "", nil },
					func(string) (string, error) { return "", errors.New("create failed") },
					func(string) (string, error) { return "", nil },
					func(string) (string, error) { return "", nil },
				)

				By("when invoking /branch create feat")
				_, err := failHandler("create feat")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal("create failed"))
			})

			It("should propagate switchFn errors", func() {
				By("given a handler with failing switch")
				failHandler := BranchHandler(
					func() (string, error) { return "", nil },
					func(string) (string, error) { return "", nil },
					func(string) (string, error) { return "", errors.New("switch failed") },
					func(string) (string, error) { return "", nil },
				)

				By("when invoking /branch switch main")
				_, err := failHandler("switch main")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal("switch failed"))
			})

			It("should propagate deleteFn errors", func() {
				By("given a handler with failing delete")
				failHandler := BranchHandler(
					func() (string, error) { return "", nil },
					func(string) (string, error) { return "", nil },
					func(string) (string, error) { return "", nil },
					func(string) (string, error) { return "", errors.New("delete failed") },
				)

				By("when invoking /branch delete old")
				_, err := failHandler("delete old")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal("delete failed"))
			})
		})
	})

	Describe("PRHandler", func() {
		var handler SlashHandler

		BeforeEach(func() {
			handler = PRHandler(
				func(title, body string) (string, error) {
					return "PR created: " + title, nil
				},
				func() (string, error) { return "PR list", nil },
			)
		})

		Context("Given no arguments", func() {
			It("should list PRs", func() {
				By("when invoking /pr")
				result, err := handler("")
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(Equal("PR list"))
			})

			It("should list PRs with list subcommand", func() {
				By("when invoking /pr list")
				result, err := handler("list")
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(Equal("PR list"))
			})
		})

		Context("Given create with quoted title", func() {
			It("should create PR with quoted title and body", func() {
				By("when invoking /pr create \"Fix bug\" body here")
				result, err := handler("create \"Fix bug\" body here")
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(Equal("PR created: Fix bug"))
			})
		})

		Context("Given create with unquoted title", func() {
			It("should create PR with first word as title", func() {
				By("when invoking /pr create Fix-bug extra body")
				result, err := handler("create Fix-bug extra body")
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(Equal("PR created: Fix-bug"))
			})
		})

		Context("Given create with malformed quotes", func() {
			It("should return usage", func() {
				By("when invoking /pr create \"unclosed")
				result, err := handler("create \"unclosed")
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(ContainSubstring("Usage: /pr create"))
			})
		})

		Context("Given unknown subcommand", func() {
			It("should return usage instructions", func() {
				By("when invoking /pr foo")
				result, err := handler("foo")
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(ContainSubstring("Usage: /pr"))
			})
		})

		Context("Given failing callback functions", func() {
			It("should propagate listFn errors", func() {
				By("given a handler with failing list")
				failHandler := PRHandler(
					func(string, string) (string, error) { return "", nil },
					func() (string, error) { return "", errors.New("list failed") },
				)

				By("when invoking /pr list")
				_, err := failHandler("list")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal("list failed"))
			})

			It("should propagate createFn errors", func() {
				By("given a handler with failing create")
				failHandler := PRHandler(
					func(string, string) (string, error) { return "", errors.New("create failed") },
					func() (string, error) { return "", nil },
				)

				By("when invoking /pr create title")
				_, err := failHandler("create title")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal("create failed"))
			})
		})

		Context("Given edge case inputs", func() {
			It("should return usage when create has no title", func() {
				var createCalled bool
				handler := PRHandler(
					func(title, body string) (string, error) {
						createCalled = true
						return "created", nil
					},
					func() (string, error) { return "list", nil },
				)

				By("when invoking /pr create ' '")
				result, err := handler("create ")
				Expect(err).ToNot(HaveOccurred())
				Expect(createCalled).To(BeFalse())
				Expect(result).To(ContainSubstring("Usage: /pr create"))
			})
		})
	})

	Describe("WorktreeHandler", func() {
		var (
			added   []string
			removed string
			handler SlashHandler
		)

		BeforeEach(func() {
			added = nil
			removed = ""
			handler = WorktreeHandler(
				func() (string, error) { return "wt1\nwt2", nil },
				func(path, branch string) (string, error) {
					added = append(added, path+":"+branch)
					return "added", nil
				},
				func(path string) (string, error) { removed = path; return "removed", nil },
			)
		})

		Context("Given no arguments", func() {
			It("should list worktrees", func() {
				By("when invoking /worktree")
				result, err := handler("")
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(Equal("wt1\nwt2"))
			})
		})

		Context("Given add subcommand", func() {
			It("should add worktree with path and branch", func() {
				By("when invoking /worktree add /tmp/wt feat-branch")
				result, err := handler("add /tmp/wt feat-branch")
				Expect(err).ToNot(HaveOccurred())
				Expect(added).To(ContainElement("/tmp/wt:feat-branch"))
				Expect(result).To(Equal("added"))
			})

			It("should add worktree with path only", func() {
				By("when invoking /worktree add /tmp/wt")
				result, err := handler("add /tmp/wt")
				Expect(err).ToNot(HaveOccurred())
				Expect(added).To(ContainElement("/tmp/wt:"))
				Expect(result).To(Equal("added"))
			})

			It("should show usage when path missing", func() {
				By("when invoking /worktree add")
				result, err := handler("add")
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(ContainSubstring("Usage: /worktree add"))
			})
		})

		Context("Given remove subcommand", func() {
			It("should remove worktree", func() {
				By("when invoking /worktree remove /tmp/wt")
				result, err := handler("remove /tmp/wt")
				Expect(err).ToNot(HaveOccurred())
				Expect(removed).To(Equal("/tmp/wt"))
				Expect(result).To(Equal("removed"))
			})

			It("should show usage when path missing", func() {
				By("when invoking /worktree remove")
				result, err := handler("remove")
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(ContainSubstring("Usage: /worktree remove"))
			})
		})

		Context("Given unknown subcommand", func() {
			It("should return usage instructions", func() {
				By("when invoking /worktree foo")
				result, err := handler("foo")
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(ContainSubstring("Usage: /worktree"))
			})
		})

		Context("Given failing callback functions", func() {
			It("should propagate listFn errors", func() {
				By("given a handler with failing list")
				failHandler := WorktreeHandler(
					func() (string, error) { return "", errors.New("list failed") },
					func(string, string) (string, error) { return "", nil },
					func(string) (string, error) { return "", nil },
				)

				By("when invoking /worktree")
				_, err := failHandler("")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal("list failed"))
			})

			It("should propagate addFn errors", func() {
				By("given a handler with failing add")
				failHandler := WorktreeHandler(
					func() (string, error) { return "", nil },
					func(string, string) (string, error) { return "", errors.New("add failed") },
					func(string) (string, error) { return "", nil },
				)

				By("when invoking /worktree add /tmp/wt")
				_, err := failHandler("add /tmp/wt")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal("add failed"))
			})

			It("should propagate removeFn errors", func() {
				By("given a handler with failing remove")
				failHandler := WorktreeHandler(
					func() (string, error) { return "", nil },
					func(string, string) (string, error) { return "", nil },
					func(string) (string, error) { return "", errors.New("remove failed") },
				)

				By("when invoking /worktree remove /tmp/wt")
				_, err := failHandler("remove /tmp/wt")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal("remove failed"))
			})
		})
	})

	// ========================================================================
	// Mode Commands
	// ========================================================================
	Describe("PlanHandler", func() {
		var mode bool
		var handler SlashHandler

		BeforeEach(func() {
			mode = false
			handler = PlanHandler(
				func() bool { return mode },
				func(on bool) string {
					mode = on
					if on {
						return "Plan mode ON"
					}
					return "Plan mode OFF"
				},
			)
		})

		Context("Given no arguments", func() {
			It("should toggle plan mode on when currently off", func() {
				By("given plan mode is off")
				mode = false

				By("when invoking /plan")
				result, err := handler("")
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(Equal("Plan mode ON"))
				Expect(mode).To(BeTrue())
			})

			It("should toggle plan mode off when currently on", func() {
				By("given plan mode is on")
				mode = true

				By("when invoking /plan")
				result, err := handler("")
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(Equal("Plan mode OFF"))
				Expect(mode).To(BeFalse())
			})
		})

		Context("Given explicit on argument", func() {
			It("should enable plan mode", func() {
				By("when invoking /plan on")
				result, err := handler("on")
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(Equal("Plan mode ON"))
				Expect(mode).To(BeTrue())
			})
		})

		Context("Given explicit off argument", func() {
			It("should disable plan mode", func() {
				By("when invoking /plan off")
				result, err := handler("off")
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(Equal("Plan mode OFF"))
				Expect(mode).To(BeFalse())
			})
		})

		Context("Given invalid argument", func() {
			It("should return usage instructions", func() {
				By("when invoking /plan maybe")
				result, err := handler("maybe")
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(ContainSubstring("Usage: /plan"))
			})
		})

		Context("Given setMode returns distinct strings", func() {
			It("should return the ON string when toggling off→on", func() {
				By("given plan mode is off")
				mode := false
				handler := PlanHandler(
					func() bool { return mode },
					func(on bool) string {
						mode = on
						if on {
							return "Plan mode ON. Outline before execute."
						}
						return "Plan mode OFF. Direct execute."
					},
				)

				By("when invoking /plan")
				result, err := handler("")
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(Equal("Plan mode ON. Outline before execute."))
				Expect(mode).To(BeTrue())
			})

			It("should return the OFF string when toggling on→off", func() {
				By("given plan mode is on")
				mode := true
				handler := PlanHandler(
					func() bool { return mode },
					func(on bool) string {
						mode = on
						if on {
							return "Plan mode ON. Outline before execute."
						}
						return "Plan mode OFF. Direct execute."
					},
				)

				By("when invoking /plan")
				result, err := handler("")
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(Equal("Plan mode OFF. Direct execute."))
				Expect(mode).To(BeFalse())
			})
		})
	})

	// ========================================================================
	// Tool Commands
	// ========================================================================
	Describe("AgentsHandler", func() {
		It("should list all agents when no args", func() {
			By("given an agents handler")
			handler := AgentsHandler(func(args string) string {
				if args == "" {
					return "All agents"
				}
				return "Agent: " + args
			})

			By("when invoking /agents")
			result, err := handler("")
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(Equal("All agents"))
		})

		It("should show specific agent details when arg provided", func() {
			By("given an agents handler")
			handler := AgentsHandler(func(args string) string {
				if args == "" {
					return "All agents"
				}
				return "Agent: " + args
			})

			By("when invoking /agents custom")
			result, err := handler("custom")
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(Equal("Agent: custom"))
		})
	})

	Describe("SkillsHandler", func() {
		It("should return skills information", func() {
			By("given a skills handler")
			handler := SkillsHandler(func(args string) string { return "Skills: coding, writing" })

			By("when invoking /skills")
			result, err := handler("")
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(Equal("Skills: coding, writing"))
		})
	})

	// ========================================================================
	// Session Management Commands
	// ========================================================================
	Describe("SessionHandler", func() {
		var loadedID string
		var handler SlashHandler

		BeforeEach(func() {
			loadedID = ""
			handler = SessionHandler(
				func() string { return "session-1\nsession-2" },
				func(id string) error { loadedID = id; return nil },
			)
		})

		Context("Given no arguments", func() {
			It("should list sessions", func() {
				By("when invoking /session")
				result, err := handler("")
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(Equal("session-1\nsession-2"))
			})
		})

		Context("Given list subcommand", func() {
			It("should list sessions", func() {
				By("when invoking /session list")
				result, err := handler("list")
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(Equal("session-1\nsession-2"))
			})
		})

		Context("Given load subcommand", func() {
			It("should load the specified session", func() {
				By("when invoking /session load abc-123")
				result, err := handler("load abc-123")
				Expect(err).ToNot(HaveOccurred())
				Expect(loadedID).To(Equal("abc-123"))
				Expect(result).To(Equal("Session loaded: abc-123"))
			})
		})

		Context("Given invalid subcommand", func() {
			It("should return usage instructions", func() {
				By("when invoking /session delete")
				result, err := handler("delete")
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(Equal("Usage: /session [list|load <id>]"))
			})
		})

		Context("Given a failing load function", func() {
			It("should propagate the error", func() {
				By("given a handler with failing load")
				failHandler := SessionHandler(
					func() string { return "" },
					func(id string) error { return errors.New("session not found") },
				)

				By("when invoking /session load missing")
				_, err := failHandler("load missing")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal("session not found"))
			})
		})

		Context("Given load without id", func() {
			It("should return usage instructions", func() {
				By("when invoking /session load")
				result, err := handler("load")
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(Equal("Usage: /session [list|load <id>]"))
			})
		})
	})

	Describe("InitHandler", func() {
		It("should init with specified project type", func() {
			By("given an init handler")
			handler := InitHandler(func(projectType string) (string, error) {
				return "Initialized " + projectType, nil
			})

			By("when invoking /init go")
			result, err := handler("go")
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(Equal("Initialized go"))
		})

		It("should default to generic when no type provided", func() {
			By("given an init handler")
			handler := InitHandler(func(projectType string) (string, error) {
				return "Initialized " + projectType, nil
			})

			By("when invoking /init")
			result, err := handler("")
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(Equal("Initialized generic"))
		})

		It("should propagate init errors", func() {
			By("given a failing init handler")
			handler := InitHandler(func(projectType string) (string, error) {
				return "", errors.New("directory not empty")
			})

			By("when invoking /init go")
			_, err := handler("go")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("directory not empty"))
		})
	})

	Describe("TestHandler", func() {
		It("should run tests and return results", func() {
			By("given a test handler")
			handler := TestHandler(func() (string, error) { return "PASS: 42 tests", nil })

			By("when invoking /test")
			result, err := handler("")
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(Equal("PASS: 42 tests"))
		})

		It("should propagate test failures", func() {
			By("given a failing test handler")
			handler := TestHandler(func() (string, error) { return "", errors.New("test failed") })

			By("when invoking /test")
			_, err := handler("")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("test failed"))
		})
	})

	Describe("MemoryHandler", func() {
		It("should return memory state", func() {
			By("given a memory handler")
			handler := MemoryHandler(func() string { return "Memory: 42 tokens" })

			By("when invoking /memory")
			result, err := handler("")
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(Equal("Memory: 42 tokens"))
		})
	})

	// ========================================================================
	// Workspace Commands
	// ========================================================================
	Describe("WorkspaceHandler", func() {
		It("should return workspace information", func() {
			By("given a workspace handler")
			handler := WorkspaceHandler(func() string { return "Workspace: /home/user/projects" })

			By("when invoking /workspace")
			result, err := handler("")
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(Equal("Workspace: /home/user/projects"))
		})
	})

	// ========================================================================
	// Auth Commands
	// ========================================================================
	Describe("LogoutHandler", func() {
		It("should clear credentials successfully", func() {
			var called bool
			By("given a logout handler")
			handler := LogoutHandler(func() error { called = true; return nil })

			By("when invoking /logout")
			result, err := handler("")
			Expect(err).ToNot(HaveOccurred())
			Expect(called).To(BeTrue())
			Expect(result).To(ContainSubstring("Logged out"))
		})

		It("should propagate logout errors", func() {
			By("given a failing logout function")
			handler := LogoutHandler(func() error { return errors.New("storage error") })

			By("when invoking /logout")
			_, err := handler("")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("storage error"))
		})
	})

	Describe("LoginHandler", func() {
		It("should start login wizard successfully", func() {
			var called bool
			By("given a login handler")
			handler := LoginHandler(func() error { called = true; return nil })

			By("when invoking /login")
			result, err := handler("")
			Expect(err).ToNot(HaveOccurred())
			Expect(called).To(BeTrue())
			Expect(result).To(BeEmpty())
		})

		It("should propagate login errors", func() {
			By("given a failing login function")
			handler := LoginHandler(func() error { return errors.New("wizard failed") })

			By("when invoking /login")
			_, err := handler("")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("wizard failed"))
		})
	})

	// ========================================================================
	// System Commands
	// ========================================================================
	Describe("ResetHandler", func() {
		var resetCalled bool
		var handler SlashHandler

		BeforeEach(func() {
			resetCalled = false
			handler = ResetHandler(func() error { resetCalled = true; return nil })
		})

		Context("Given no confirmation flag", func() {
			It("should warn and not reset", func() {
				By("when invoking /reset")
				result, err := handler("")
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(ContainSubstring("WARNING"))
				Expect(resetCalled).To(BeFalse())
			})
		})

		Context("Given --confirm flag", func() {
			It("should reset and return reset signal", func() {
				By("when invoking /reset --confirm")
				result, err := handler("--confirm")
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(Equal("__RESET__"))
				Expect(resetCalled).To(BeTrue())
			})
		})

		Context("Given -y flag", func() {
			It("should reset and return reset signal", func() {
				By("when invoking /reset -y")
				result, err := handler("-y")
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(Equal("__RESET__"))
				Expect(resetCalled).To(BeTrue())
			})
		})

		Context("Given a failing reset function", func() {
			It("should propagate the error", func() {
				By("given a handler with failing reset")
				failHandler := ResetHandler(func() error { return errors.New("reset failed") })

				By("when invoking /reset --confirm")
				_, err := failHandler("--confirm")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal("reset failed"))
			})
		})
	})

	Describe("QuitHandler", func() {
		It("should return quit signal", func() {
			By("given a quit handler")
			handler := QuitHandler()

			By("when invoking /quit")
			result, err := handler("")
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(Equal("__QUIT__"))
		})

		It("should be detectable by IsQuit", func() {
			By("checking __QUIT__")
			Expect(IsQuit("__QUIT__")).To(BeTrue())
			Expect(IsQuit("something else")).To(BeFalse())
		})
	})

	Describe("SteerHandler", func() {
		It("should queue a message and return empty", func() {
			By("given a steer handler")
			queued := ""
			handler := SteerHandler(func(msg string) { queued = msg })

			By("when invoking /steer check the tests")
			result, err := handler("check the tests")
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(BeEmpty())
			Expect(queued).To(Equal("check the tests"))
		})

		It("should return usage when no args provided", func() {
			By("given a steer handler")
			handler := SteerHandler(func(msg string) {})

			By("when invoking /steer with no args")
			result, err := handler("")
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(ContainSubstring("Usage: /steer"))
		})
	})

	Describe("PersonaHandler", func() {
		It("should list personas on empty or 'list' arg", func() {
			handler := PersonaHandler(nil, nil, func() string { return "Available: developer, designer" })
			
			res1, err1 := handler("")
			Expect(err1).ToNot(HaveOccurred())
			Expect(res1).To(Equal("Available: developer, designer"))
			
			res2, err2 := handler("list")
			Expect(err2).ToNot(HaveOccurred())
			Expect(res2).To(Equal("Available: developer, designer"))
		})

		It("should change persona and return success message", func() {
			current := "developer"
			handler := PersonaHandler(
				func() string { return current },
				func(p string) error { current = p; return nil },
				nil,
			)

			res, err := handler("designer")
			Expect(err).ToNot(HaveOccurred())
			Expect(res).To(ContainSubstring("Persona updated"))
			Expect(res).To(ContainSubstring("Previous         developer"))
			Expect(res).To(ContainSubstring("Current          designer"))
		})

		It("should handle nil dependencies", func() {
			handlerListOnly := PersonaHandler(nil, nil, nil)
			_, err := handlerListOnly("")
			Expect(err).To(MatchError("persona listing is not available"))

			handlerSwitchOnly := PersonaHandler(nil, nil, func() string { return "" })
			_, err = handlerSwitchOnly("designer")
			Expect(err).To(MatchError("persona switching is not available"))
		})
	})

	Describe("IsReset", func() {
		It("should detect reset signal", func() {
			By("checking __RESET__")
			Expect(IsReset("__RESET__")).To(BeTrue())
			Expect(IsReset("other")).To(BeFalse())
		})
	})
})
