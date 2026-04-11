package defaults

import "time"

// Test bucket defaults
const (
	// Timeout settings
	TestDefaultTimeout = 60 * time.Second
	TestMaxTimeout     = 300 * time.Second // 5 minutes
	TestSetupTimeout   = 120 * time.Second

	// Output limits
	TestMaxOutputLines = 1000
	TestMaxOutputBytes = 100 * 1024 // 100KB

	// Parallelism
	TestMaxParallel     = 4
	TestDefaultParallel = 2
)

// TestFrameworks maps languages to test frameworks
var TestFrameworks = map[string]TestFrameworkConfig{
	"go": {
		Command:     "go test",
		Args:        "-v -race -count=1",
		PatternArg:  "-run",
		CoverageArg: "-cover",
		TimeoutArg:  "-timeout",
		ParallelArg: "-parallel",
	},
	"javascript": {
		Command:     "npm test",
		Args:        "",
		PatternArg:  "--testNamePattern",
		CoverageArg: "--coverage",
		TimeoutArg:  "",
		ParallelArg: "",
	},
	"typescript": {
		Command:     "npm test",
		Args:        "",
		PatternArg:  "--testNamePattern",
		CoverageArg: "--coverage",
		TimeoutArg:  "",
		ParallelArg: "",
	},
	"python": {
		Command:     "pytest",
		Args:        "-v",
		PatternArg:  "-k",
		CoverageArg: "--cov",
		TimeoutArg:  "--timeout",
		ParallelArg: "-n",
	},
	"rust": {
		Command:     "cargo test",
		Args:        "--",
		PatternArg:  "",
		CoverageArg: "",
		TimeoutArg:  "",
		ParallelArg: "",
	},
	"ruby": {
		Command:     "rspec",
		Args:        "--format documentation",
		PatternArg:  "-e",
		CoverageArg: "",
		TimeoutArg:  "",
		ParallelArg: "",
	},
}

// TestFrameworkConfig holds test framework configuration
type TestFrameworkConfig struct {
	Command     string
	Args        string
	PatternArg  string
	CoverageArg string
	TimeoutArg  string
	ParallelArg string
}

// TestResultStatuses
const (
	TestStatusPass  = "pass"
	TestStatusFail  = "fail"
	TestStatusSkip  = "skip"
	TestStatusError = "error"
)

// TestPriorityOrder for sorting results
var TestPriorityOrder = []string{
	TestStatusFail,
	TestStatusError,
	TestStatusSkip,
	TestStatusPass,
}
