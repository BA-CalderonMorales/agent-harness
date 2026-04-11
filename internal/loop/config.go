package loop

import "time"

// LoopConfig provides unified configuration for the agent loop.
// All settings have sensible defaults and can be overridden.
type LoopConfig struct {
	// Core loop settings
	MaxTurns        int // Maximum iterations before giving up
	MaxOutputTokens int // Token limit for LLM responses

	// Behavior toggles
	AutoCompactEnabled     bool // Automatically compact context when full
	StreamingToolExecution bool // Stream tool results as they complete
	EnableRecovery         bool // Attempt to recover from recoverable errors

	// Recovery settings
	MaxOutputTokensRecovery int // Max retry attempts for token errors
	RecoveryBackoff         time.Duration

	// Token management
	BlockingTokenLimit int // Hard limit before blocking
	TargetTokenLimit   int // Soft limit for compaction

	// Tool execution
	DefaultTimeout    time.Duration
	MaxConcurrentTools int // Limit concurrent tool execution

	// Logging/Debugging
	Debug          bool
	VerboseLogging bool
}

// DefaultConfig returns production-ready defaults.
func DefaultConfig() LoopConfig {
	return LoopConfig{
		MaxTurns:                10,
		MaxOutputTokens:         8192,
		AutoCompactEnabled:      true,
		StreamingToolExecution:  true,
		EnableRecovery:          true,
		MaxOutputTokensRecovery: 3,
		RecoveryBackoff:         500 * time.Millisecond,
		BlockingTokenLimit:      180000,
		TargetTokenLimit:        120000,
		DefaultTimeout:          60 * time.Second,
		MaxConcurrentTools:      5,
		Debug:                   false,
		VerboseLogging:          false,
	}
}

// FastConfig returns settings optimized for quick responses.
func FastConfig() LoopConfig {
	cfg := DefaultConfig()
	cfg.MaxTurns = 5
	cfg.MaxOutputTokens = 4096
	cfg.AutoCompactEnabled = false
	cfg.MaxConcurrentTools = 3
	return cfg
}

// RobustConfig returns settings optimized for complex tasks.
func RobustConfig() LoopConfig {
	cfg := DefaultConfig()
	cfg.MaxTurns = 25
	cfg.MaxOutputTokens = 16384
	cfg.MaxOutputTokensRecovery = 5
	cfg.MaxConcurrentTools = 10
	return cfg
}

// WithMaxTurns returns a copy with modified max turns.
func (c LoopConfig) WithMaxTurns(n int) LoopConfig {
	c.MaxTurns = n
	return c
}

// WithTimeout returns a copy with modified timeout.
func (c LoopConfig) WithTimeout(d time.Duration) LoopConfig {
	c.DefaultTimeout = d
	return c
}

// WithDebug returns a copy with debug enabled.
func (c LoopConfig) WithDebug() LoopConfig {
	c.Debug = true
	c.VerboseLogging = true
	return c
}
