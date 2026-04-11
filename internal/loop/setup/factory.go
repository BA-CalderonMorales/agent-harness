// Package setup provides factory functions for creating orchestrators.
package setup

import (
	"time"

	"github.com/BA-CalderonMorales/agent-harness/internal/llm"
	"github.com/BA-CalderonMorales/agent-harness/internal/loop"
	"github.com/BA-CalderonMorales/agent-harness/internal/loop/buckets"
	"github.com/BA-CalderonMorales/agent-harness/internal/loop/buckets/defaults"
)

// Factory creates pre-configured orchestrators with standard buckets.
type Factory struct {
	basePath string
	config   loop.LoopConfig
	client   llm.Client
}

// NewFactory creates a new orchestrator factory.
func NewFactory(basePath string, client llm.Client) *Factory {
	return &Factory{
		basePath: basePath,
		config:   loop.DefaultConfig(),
		client:   client,
	}
}

// WithConfig sets a custom configuration.
func (f *Factory) WithConfig(cfg loop.LoopConfig) *Factory {
	f.config = cfg
	return f
}

// CreateStandard builds an orchestrator with all standard buckets.
func (f *Factory) CreateStandard() *loop.Orchestrator {
	return f.createWithBuckets(
		buckets.NewLoopFileSystem(f.basePath),
		buckets.NewLoopShell(f.basePath),
		buckets.NewLoopSearch(f.basePath),
		buckets.NewLoopGit(f.basePath),
		buckets.NewLoopUI("", ""),
		buckets.NewLoopPlan(),
		buckets.NewLoopTranscript(),
	)
}

// CreateSafe builds an orchestrator with read-only buckets only.
func (f *Factory) CreateSafe() *loop.Orchestrator {
	fs := buckets.NewLoopFileSystem(f.basePath).
		WithBlockedPaths("/etc", "/usr", "/bin", "/sbin", "/dev", "/sys")

	search := buckets.NewLoopSearch(f.basePath)
	git := buckets.NewLoopGit(f.basePath).WithoutApproval()
	transcript := buckets.NewLoopTranscript()

	return f.createWithBuckets(fs, search, git, transcript)
}

// CreateFast builds an orchestrator optimized for speed.
func (f *Factory) CreateFast() *loop.Orchestrator {
	cfg := loop.FastConfig()

	fs := buckets.NewLoopFileSystem(f.basePath)

	shell := buckets.NewLoopShell(f.basePath).
		WithTimeout(cfg.DefaultTimeout).
		WithoutApproval()

	search := buckets.NewLoopSearch(f.basePath).
		WithMaxResults(defaults.SearchMaxResultsFast)

	git := buckets.NewLoopGit(f.basePath).WithoutApproval()
	transcript := buckets.NewLoopTranscript().
		WithMaxHistory(defaults.TranscriptMaxHistoryFast)

	return loop.NewOrchestrator(cfg, f.client, fs, shell, search, git, transcript)
}

// CreateRobust builds an orchestrator for complex tasks.
func (f *Factory) CreateRobust() *loop.Orchestrator {
	cfg := loop.RobustConfig()

	fs := buckets.NewLoopFileSystem(f.basePath)

	shell := buckets.NewLoopShell(f.basePath).
		WithTimeout(5 * cfg.DefaultTimeout)

	search := buckets.NewLoopSearch(f.basePath).
		WithMaxResults(defaults.SearchMaxResultsRobust).
		WithContextLines(defaults.SearchContextLinesMore)

	git := buckets.NewLoopGit(f.basePath)
	ui := buckets.NewLoopUI("", "")
	agent := buckets.NewLoopAgent(f.basePath, f.client).WithMaxDepth(defaults.AgentMaxDepthDeep)
	plan := buckets.NewLoopPlan()
	transcript := buckets.NewLoopTranscript().WithMaxHistory(defaults.TranscriptMaxHistoryFull)
	web := buckets.NewLoopWeb().WithTimeout(defaults.WebFetchTimeout)
	code := buckets.NewLoopCode(f.basePath).WithMaxIssues(defaults.CodeLintMaxIssues)
	test := buckets.NewLoopTest(f.basePath).WithParallel(defaults.TestMaxParallel)

	return loop.NewOrchestrator(cfg, f.client, fs, shell, search, git, ui, agent, plan, transcript, web, code, test)
}

// CreateCustom builds an orchestrator with specific buckets.
func (f *Factory) CreateCustom(bucketsList ...loop.LoopBase) *loop.Orchestrator {
	return f.createWithBuckets(bucketsList...)
}

// CreateMinimal builds an orchestrator with only file operations.
func (f *Factory) CreateMinimal() *loop.Orchestrator {
	return f.createWithBuckets(
		buckets.NewLoopFileSystem(f.basePath),
	)
}

// CreateWithFileSystemOnly builds an orchestrator with only the filesystem bucket.
func (f *Factory) CreateWithFileSystemOnly() *loop.Orchestrator {
	return f.createWithBuckets(
		buckets.NewLoopFileSystem(f.basePath),
	)
}

// createWithBuckets is the internal constructor.
func (f *Factory) createWithBuckets(bucketsList ...loop.LoopBase) *loop.Orchestrator {
	return loop.NewOrchestrator(f.config, f.client, bucketsList...)
}

// Preset is a named orchestrator configuration.
type Preset string

const (
	PresetStandard Preset = "standard"
	PresetSafe     Preset = "safe"
	PresetFast     Preset = "fast"
	PresetRobust   Preset = "robust"
	PresetMinimal  Preset = "minimal"
)

// CreateFromPreset creates an orchestrator from a named preset.
func CreateFromPreset(preset Preset, basePath string, client llm.Client) *loop.Orchestrator {
	factory := NewFactory(basePath, client)

	switch preset {
	case PresetSafe:
		return factory.CreateSafe()
	case PresetFast:
		return factory.CreateFast()
	case PresetRobust:
		return factory.CreateRobust()
	case PresetMinimal:
		return factory.CreateMinimal()
	default:
		return factory.CreateStandard()
	}
}

// Builder provides a fluent API for constructing orchestrators.
type Builder struct {
	factory *Factory
	buckets []loop.LoopBase
}

// NewBuilder starts building an orchestrator.
func (f *Factory) NewBuilder() *Builder {
	return &Builder{
		factory: f,
		buckets: make([]loop.LoopBase, 0),
	}
}

// WithFileSystem adds the filesystem bucket.
func (b *Builder) WithFileSystem(opts ...func(*buckets.LoopFileSystem)) *Builder {
	fs := buckets.NewLoopFileSystem(b.factory.basePath)
	for _, opt := range opts {
		opt(fs)
	}
	b.buckets = append(b.buckets, fs)
	return b
}

// WithShell adds the shell bucket.
func (b *Builder) WithShell(opts ...func(*buckets.LoopShell)) *Builder {
	sh := buckets.NewLoopShell(b.factory.basePath)
	for _, opt := range opts {
		opt(sh)
	}
	b.buckets = append(b.buckets, sh)
	return b
}

// WithSearch adds the search bucket.
func (b *Builder) WithSearch(opts ...func(*buckets.LoopSearch)) *Builder {
	search := buckets.NewLoopSearch(b.factory.basePath)
	for _, opt := range opts {
		opt(search)
	}
	b.buckets = append(b.buckets, search)
	return b
}

// WithBucket adds a custom bucket.
func (b *Builder) WithBucket(bucket loop.LoopBase) *Builder {
	b.buckets = append(b.buckets, bucket)
	return b
}

// Build constructs the orchestrator.
func (b *Builder) Build() *loop.Orchestrator {
	if len(b.buckets) == 0 {
		// Default to standard set
		return b.factory.CreateStandard()
	}
	return b.factory.CreateCustom(b.buckets...)
}

// FileSystemOption provides configuration helpers for filesystem bucket.
var FileSystemOption = struct {
	AllowPaths  func(paths ...string) func(*buckets.LoopFileSystem)
	BlockPaths  func(paths ...string) func(*buckets.LoopFileSystem)
}{
	AllowPaths: func(paths ...string) func(*buckets.LoopFileSystem) {
		return func(fs *buckets.LoopFileSystem) {
			fs.WithAllowedPaths(paths...)
		}
	},
	BlockPaths: func(paths ...string) func(*buckets.LoopFileSystem) {
		return func(fs *buckets.LoopFileSystem) {
			fs.WithBlockedPaths(paths...)
		}
	},
}

// ShellOption provides configuration helpers for shell bucket.
var ShellOption = struct {
	Timeout      func(d int) func(*buckets.LoopShell)
	NoApproval   func() func(*buckets.LoopShell)
	AllowCommands func(cmds ...string) func(*buckets.LoopShell)
}{
	Timeout: func(d int) func(*buckets.LoopShell) {
		return func(sh *buckets.LoopShell) {
			sh.WithTimeout(time.Duration(d) * time.Second)
		}
	},
	NoApproval: func() func(*buckets.LoopShell) {
		return func(sh *buckets.LoopShell) {
			sh.WithoutApproval()
		}
	},
	AllowCommands: func(cmds ...string) func(*buckets.LoopShell) {
		return func(sh *buckets.LoopShell) {
			sh.WithAllowedCommands(cmds...)
		}
	},
}

// SearchOption provides configuration helpers for search bucket.
var SearchOption = struct {
	MaxResults   func(n int) func(*buckets.LoopSearch)
	ContextLines func(n int) func(*buckets.LoopSearch)
}{
	MaxResults: func(n int) func(*buckets.LoopSearch) {
		return func(s *buckets.LoopSearch) {
			s.WithMaxResults(n)
		}
	},
	ContextLines: func(n int) func(*buckets.LoopSearch) {
		return func(s *buckets.LoopSearch) {
			s.WithContextLines(n)
		}
	},
}
