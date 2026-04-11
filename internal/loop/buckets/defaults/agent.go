package defaults

// Agent bucket defaults
const (
	// Recursion limits
	AgentMaxDepthDefault = 5
	AgentMaxDepthShallow = 2
	AgentMaxDepthDeep    = 10

	// Sub-agent settings
	AgentSubAgentMaxTurns    = 5
	AgentSubAgentModel       = "gpt-4o-mini"
	AgentSubAgentModelFast   = "gpt-3.5-turbo"

	// Timeout for sub-agents
	AgentSubAgentTimeoutSecs = 120
)

// AgentTypes contains predefined agent specializations
var AgentTypes = map[string]string{
	"default":   "You are a helpful assistant. Complete the task efficiently.",
	"reviewer":  "You are a code reviewer. Focus on bugs, security issues, and best practices.",
	"tester":    "You are a testing specialist. Write and run tests to verify functionality.",
	"docs":      "You are a documentation specialist. Write clear, concise documentation.",
	"refactor":  "You are a refactoring specialist. Improve code quality without changing behavior.",
	"explainer": "You are an explainer. Break down complex code into simple concepts.",
	"debugger":  "You are a debugging specialist. Find and fix bugs systematically.",
	"optimizer": "You are a performance specialist. Optimize for speed and resource usage.",
	"security":  "You are a security specialist. Audit for vulnerabilities and security issues.",
}

// AgentAllowedTools maps agent types to recommended tool sets
var AgentAllowedTools = map[string][]string{
	"default":   {"read", "write", "edit", "bash", "grep"},
	"reviewer":  {"read", "grep", "glob"},
	"tester":    {"read", "bash", "write"},
	"docs":      {"read", "write", "edit"},
	"refactor":  {"read", "write", "edit", "grep"},
	"explainer": {"read", "grep"},
	"debugger":  {"read", "bash", "grep", "edit"},
	"optimizer": {"read", "bash", "grep"},
	"security":  {"read", "grep", "glob"},
}
