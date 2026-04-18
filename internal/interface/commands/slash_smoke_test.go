package commands

import (
	"errors"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// =============================================================================
// Registry Smoke Tests
// =============================================================================
// These tests verify that ALL slash commands can be registered and dispatched
// through the registry with real handler signatures. They act as contract tests
// ensuring handler signatures align with registry expectations.
// =============================================================================

var _ = Describe("Slash Registry Smoke Tests", func() {
	var registry *SlashRegistry

	BeforeEach(func() {
		registry = NewSlashRegistry()
	})

	Describe("Command Registration and Dispatch", func() {
		It("should register and dispatch all session commands", func() {
			By("registering session-related commands")
			registry.Register("help", "Show help", HelpHandler(registry))
			registry.Register("status", "Show status", StatusHandler(func() string { return "ok" }))
			registry.Register("clear", "Clear session", ClearHandler(func() error { return nil }, nil))
			registry.Register("compact", "Compact session", CompactHandler(func() (string, error) { return "compacted", nil }))
			registry.Register("session", "Manage sessions", SessionHandler(func() string { return "s1" }, func(string) error { return nil }))
			registry.Register("reset", "Reset", ResetHandler(func() error { return nil }))
			registry.Register("quit", "Quit", QuitHandler())
			registry.Register("workspace", "Workspace", WorkspaceHandler(func() string { return "/home" }))

			By("dispatching each command")
			commands := []string{"/help", "/status", "/clear", "/compact", "/session", "/reset", "/quit", "/workspace"}
			for _, cmd := range commands {
				result, handled, err := registry.Handle(cmd)
				Expect(err).ToNot(HaveOccurred(), "command %s errored", cmd)
				Expect(handled).To(BeTrue(), "command %s not handled", cmd)
				Expect(result).ToNot(BeNil())
			}
		})

		It("should register and dispatch all model commands", func() {
			By("registering model-related commands")
			registry.Register("model", "Switch model", ModelHandler(
				func() string { return "gpt-4o" },
				func(string) error { return nil },
				func() []string { return []string{"gpt-4o"} },
			))
			registry.Register("current-model", "Show model", CurrentModelHandler(func() string { return "gpt-4o" }))

			By("dispatching model commands")
			_, handled, err := registry.Handle("/model")
			Expect(err).ToNot(HaveOccurred())
			Expect(handled).To(BeTrue())

			_, handled, err = registry.Handle("/current-model")
			Expect(err).ToNot(HaveOccurred())
			Expect(handled).To(BeTrue())
		})

		It("should register and dispatch all settings commands", func() {
			By("registering settings commands")
			registry.Register("permissions", "Permissions", PermissionsHandler(
				func() string { return "read-only" },
				func(string) error { return nil },
				func() string { return "Mode: read-only" },
			))
			registry.Register("config", "Show config", ConfigHandler(func() string { return "cfg" }))

			By("dispatching settings commands")
			_, handled, err := registry.Handle("/permissions")
			Expect(err).ToNot(HaveOccurred())
			Expect(handled).To(BeTrue())

			_, handled, err = registry.Handle("/config")
			Expect(err).ToNot(HaveOccurred())
			Expect(handled).To(BeTrue())
		})

		It("should register and dispatch all output commands", func() {
			By("registering output commands")
			registry.Register("cost", "Show cost", CostHandler(func() string { return "$0" }))
			registry.Register("diff", "Show diff", DiffHandler(func() string { return "" }))
			registry.Register("export", "Export", ExportHandler(func(string) (string, error) { return "file", nil }))
			registry.Register("version", "Show version", VersionHandler("0.1.0", ""))

			By("dispatching output commands")
			for _, cmd := range []string{"/cost", "/diff", "/export", "/version"} {
				_, handled, err := registry.Handle(cmd)
				Expect(err).ToNot(HaveOccurred(), "command %s errored", cmd)
				Expect(handled).To(BeTrue(), "command %s not handled", cmd)
			}
		})

		It("should register and dispatch all git commands", func() {
			By("registering git commands")
			registry.Register("commit", "Commit", CommitHandler(func(string) (string, error) { return "committed", nil }))
			registry.Register("branch", "Branch", BranchHandler(
				func() (string, error) { return "main", nil },
				func(string) (string, error) { return "created", nil },
				func(string) (string, error) { return "switched", nil },
				func(string) (string, error) { return "deleted", nil },
			))
			registry.Register("pr", "PR", PRHandler(
				func(string, string) (string, error) { return "pr", nil },
				func() (string, error) { return "list", nil },
			))
			registry.Register("worktree", "Worktree", WorktreeHandler(
				func() (string, error) { return "wt", nil },
				func(string, string) (string, error) { return "added", nil },
				func(string) (string, error) { return "removed", nil },
			))

			By("dispatching git commands")
			for _, cmd := range []string{"/commit msg", "/branch", "/pr", "/worktree"} {
				_, handled, err := registry.Handle(cmd)
				Expect(err).ToNot(HaveOccurred(), "command %s errored", cmd)
				Expect(handled).To(BeTrue(), "command %s not handled", cmd)
			}
		})

		It("should register and dispatch all tool commands", func() {
			By("registering tool commands")
			registry.Register("agents", "Show agents", AgentsHandler(func(string) string { return "agents" }))
			registry.Register("skills", "Show skills", SkillsHandler(func(string) string { return "skills" }))
			registry.Register("test", "Run tests", TestHandler(func() (string, error) { return "pass", nil }))
			registry.Register("memory", "Show memory", MemoryHandler(func() string { return "mem" }))
			registry.Register("plan", "Toggle plan", PlanHandler(func() bool { return false }, func(bool) string { return "ok" }))
			registry.Register("init", "Init project", InitHandler(func(string) (string, error) { return "inited", nil }))

			By("dispatching tool commands")
			for _, cmd := range []string{"/agents", "/skills", "/test", "/memory", "/plan", "/init"} {
				_, handled, err := registry.Handle(cmd)
				Expect(err).ToNot(HaveOccurred(), "command %s errored", cmd)
				Expect(handled).To(BeTrue(), "command %s not handled", cmd)
			}
		})

		It("should register and dispatch auth commands", func() {
			By("registering auth commands")
			registry.Register("logout", "Logout", LogoutHandler(func() error { return nil }))
			registry.Register("login", "Login", LoginHandler(func() error { return nil }))

			By("dispatching auth commands")
			_, handled, err := registry.Handle("/logout")
			Expect(err).ToNot(HaveOccurred())
			Expect(handled).To(BeTrue())

			_, handled, err = registry.Handle("/login")
			Expect(err).ToNot(HaveOccurred())
			Expect(handled).To(BeTrue())
		})
	})

	Describe("Error Propagation Through Registry", func() {
		It("should propagate handler errors to the caller", func() {
			By("registering a command that always errors")
			registry.Register("fail", "Always fails", func(string) (string, error) {
				return "", errors.New("boom")
			})

			By("dispatching the failing command")
			_, handled, err := registry.Handle("/fail")
			Expect(handled).To(BeTrue())
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("boom"))
		})
	})

	Describe("Alias Handling", func() {
		It("should treat exit as an alias for quit", func() {
			By("registering quit and exit as aliases")
			registry.Register("quit", "Quit", QuitHandler())
			registry.Register("exit", "Exit", QuitHandler())

			By("dispatching /exit")
			result, handled, err := registry.Handle("/exit")
			Expect(err).ToNot(HaveOccurred())
			Expect(handled).To(BeTrue())
			Expect(result).To(Equal("__QUIT__"))
		})
	})

	Describe("Help Coverage", func() {
		It("should include all registered commands in help text", func() {
			By("registering a representative set of commands")
			registry.Register("help", "Show help", HelpHandler(registry))
			registry.Register("quit", "Quit", QuitHandler())
			registry.Register("model", "Model", ModelHandler(
				func() string { return "gpt-4o" },
				func(string) error { return nil },
				func() []string { return nil },
			))
			registry.Register("clear", "Clear", ClearHandler(func() error { return nil }, nil))

			By("generating help text")
			help := registry.GetHelp()

			By("verifying all commands appear")
			Expect(help).To(ContainSubstring("/help"))
			Expect(help).To(ContainSubstring("/quit"))
			Expect(help).To(ContainSubstring("/model"))
			Expect(help).To(ContainSubstring("/clear"))
		})

		It("should exclude hidden aliases from help", func() {
			By("registering a visible command and a hidden alias")
			registry.Register("quit", "Quit", QuitHandler())
			registry.Register("exit", "Exit alias", QuitHandler())

			By("generating help text")
			help := registry.GetHelp()

			By("verifying exit is hidden")
			Expect(help).To(ContainSubstring("/quit"))
			Expect(help).ToNot(ContainSubstring("/exit"))
		})
	})

	Describe("Completions Coverage", func() {
		It("should include all non-alias commands in completions", func() {
			By("registering commands including an alias")
			registry.Register("help", "Show help", HelpHandler(registry))
			registry.Register("quit", "Quit", QuitHandler())
			registry.Register("exit", "Exit alias", QuitHandler())
			registry.Register("model", "Model", ModelHandler(
				func() string { return "gpt-4o" },
				func(string) error { return nil },
				func() []string { return nil },
			))

			By("getting completions")
			comps := registry.GetCompletions()

			By("verifying coverage")
			Expect(comps).To(ContainElement("/help"))
			Expect(comps).To(ContainElement("/quit"))
			Expect(comps).To(ContainElement("/model"))
			Expect(comps).ToNot(ContainElement("/exit"))
		})
	})

	Describe("Argument Parsing Through Registry", func() {
		It("should correctly pass arguments to handlers", func() {
			var receivedArgs string
			registry.Register("echo", "Echo args", func(args string) (string, error) {
				receivedArgs = args
				return args, nil
			})

			By("dispatching with simple args")
			registry.Handle("/echo hello")
			Expect(receivedArgs).To(Equal("hello"))

			By("dispatching with complex args")
			registry.Handle("/echo hello world")
			Expect(receivedArgs).To(Equal("hello world"))

			By("dispatching with no args")
			registry.Handle("/echo")
			Expect(receivedArgs).To(Equal(""))
		})

		It("should preserve leading and trailing spaces in args via TrimSpace", func() {
			var receivedArgs string
			registry.Register("capture", "Capture", func(args string) (string, error) {
				receivedArgs = args
				return args, nil
			})

			By("dispatching with extra spaces")
			registry.Handle("/capture   spaced   args  ")
			Expect(receivedArgs).To(Equal("spaced   args"))
		})
	})
})

// =============================================================================
// End-to-End Handler Chain Tests
// =============================================================================
// These tests verify complete handler chains: registry → parse → handler → result
// =============================================================================

var _ = Describe("End-to-End Command Chains", func() {
	It("should handle a full model switch workflow", func() {
		By("setting up a registry with model command")
		registry := NewSlashRegistry()
		current := "gpt-4o"
		registry.Register("model", "Switch model", ModelHandler(
			func() string { return current },
			func(m string) error { current = m; return nil },
			func() []string { return []string{"gpt-4o", "claude-3-5-sonnet"} },
		))

		By("listing models")
		result, handled, err := registry.Handle("/model")
		Expect(err).ToNot(HaveOccurred())
		Expect(handled).To(BeTrue())
		Expect(result).To(ContainSubstring("gpt-4o"))

		By("switching model")
		result, handled, err = registry.Handle("/model claude-3-5-sonnet")
		Expect(err).ToNot(HaveOccurred())
		Expect(handled).To(BeTrue())
		Expect(result).To(ContainSubstring("Model updated"))
		Expect(current).To(Equal("claude-3-5-sonnet"))

		By("listing again to verify new current")
		result, handled, err = registry.Handle("/model")
		Expect(err).ToNot(HaveOccurred())
		Expect(handled).To(BeTrue())
		Expect(result).To(ContainSubstring("● claude-3-5-sonnet"))
	})

	It("should handle a full branch workflow", func() {
		By("setting up a registry with branch command")
		registry := NewSlashRegistry()
		branches := []string{"main"}
		registry.Register("branch", "Manage branches", BranchHandler(
			func() (string, error) { return strings.Join(branches, "\n"), nil },
			func(name string) (string, error) { branches = append(branches, name); return "created", nil },
			func(name string) (string, error) { return "switched", nil },
			func(name string) (string, error) { return "deleted", nil },
		))

		By("listing branches")
		result, handled, err := registry.Handle("/branch")
		Expect(err).ToNot(HaveOccurred())
		Expect(handled).To(BeTrue())
		Expect(result).To(ContainSubstring("main"))

		By("creating a branch")
		result, handled, err = registry.Handle("/branch create feat")
		Expect(err).ToNot(HaveOccurred())
		Expect(handled).To(BeTrue())
		Expect(result).To(Equal("created"))
		Expect(branches).To(ContainElement("feat"))
	})

	It("should handle a full session workflow", func() {
		By("setting up a registry with session command")
		registry := NewSlashRegistry()
		sessions := []string{"session-1", "session-2"}
		loaded := ""
		registry.Register("session", "Manage sessions", SessionHandler(
			func() string { return strings.Join(sessions, "\n") },
			func(id string) error { loaded = id; return nil },
		))

		By("listing sessions")
		result, handled, err := registry.Handle("/session")
		Expect(err).ToNot(HaveOccurred())
		Expect(handled).To(BeTrue())
		Expect(result).To(ContainSubstring("session-1"))

		By("loading a session")
		result, handled, err = registry.Handle("/session load session-2")
		Expect(err).ToNot(HaveOccurred())
		Expect(handled).To(BeTrue())
		Expect(loaded).To(Equal("session-2"))
		Expect(result).To(ContainSubstring("Session loaded: session-2"))
	})

	It("should handle a full reset workflow", func() {
		By("setting up a registry with reset command")
		registry := NewSlashRegistry()
		resetCalled := false
		registry.Register("reset", "Reset", ResetHandler(func() error { resetCalled = true; return nil }))

		By("requesting reset without confirmation")
		result, handled, err := registry.Handle("/reset")
		Expect(err).ToNot(HaveOccurred())
		Expect(handled).To(BeTrue())
		Expect(result).To(ContainSubstring("WARNING"))
		Expect(resetCalled).To(BeFalse())

		By("confirming reset")
		result, handled, err = registry.Handle("/reset --confirm")
		Expect(err).ToNot(HaveOccurred())
		Expect(handled).To(BeTrue())
		Expect(IsReset(result)).To(BeTrue())
		Expect(resetCalled).To(BeTrue())
	})
})
