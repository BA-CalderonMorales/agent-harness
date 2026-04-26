package agent

import (
	"sync"
	"time"

	"github.com/BA-CalderonMorales/agent-harness/internal/runtime/llm"
	"github.com/BA-CalderonMorales/agent-harness/internal/runtime/tools"
	"github.com/BA-CalderonMorales/agent-harness/pkg/types"
)

// QueryParams configures a single agentic turn.
type QueryParams struct {
	Messages        []types.Message
	SystemPrompt    string
	UserContext     map[string]string
	SystemContext   map[string]string
	CanUseTool      tools.CanUseToolFn
	ToolUseContext  tools.Context
	FallbackModel   string
	QuerySource     types.QuerySource
	MaxOutputTokens int
	MaxTurns        int
	SkipCacheWrite  bool
}

// TerminalReason explains why the query loop ended.
type TerminalReason string

const (
	TerminalReasonComplete      TerminalReason = "complete"
	TerminalReasonMaxTurns      TerminalReason = "max_turns"
	TerminalReasonBlockingLimit TerminalReason = "blocking_limit"
	TerminalReasonUserInterrupt TerminalReason = "user_interrupt"
	TerminalReasonError         TerminalReason = "error"
)

// Terminal is the final state of a query loop.
type Terminal struct {
	Reason  TerminalReason
	Message *types.Message
	Error   error
}

// QueryResult is the outcome of a complete query session.
type QueryResult struct {
	Messages []types.Message
	Terminal Terminal
}

// Loop implements the core agentic query loop.
type Loop struct {
	Client llm.Client
	Config LoopConfig
	mu     sync.Mutex
}

// LoopConfig tunes loop behavior.
type LoopConfig struct {
	AutoCompactEnabled      bool
	StreamingToolExecution  bool
	MaxOutputTokensRecovery int
	DefaultMaxTurns         int
	MaxToolCalls            int
	BlockingTokenLimit      int
}

// DefaultLoopConfig returns sensible defaults.
func DefaultLoopConfig() LoopConfig {
	return LoopConfig{
		AutoCompactEnabled:      true,
		StreamingToolExecution:  true,
		MaxOutputTokensRecovery: 3,
		DefaultMaxTurns:         10,
		MaxToolCalls:            15,
		BlockingTokenLimit:      180000,
	}
}

// QueryChainTracking prevents infinite recursion.
type QueryChainTracking struct {
	ChainID string
	Depth   int
}

// State is mutable state carried between loop iterations.
type loopState struct {
	messages                     []types.Message
	toolUseContext               tools.Context
	maxOutputTokensRecoveryCount int
	hasAttemptedReactiveCompact  bool
	maxOutputTokensOverride      int
	turnCount                    int
	toolCallCount                int
}

// Prominent token thresholds.
const (
	MaxOutputTokensRecoveryLimit = 3
	AssistantBlockingBudget      = 15 * time.Second
)
