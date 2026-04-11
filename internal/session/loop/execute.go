package loop

import (
	"context"
	"sync"
	"time"
)

// LoopExecute provides execution strategies for tool calls.
// It handles sequential vs parallel execution, timeouts, and retries.
type LoopExecute struct {
	defaultTimeout time.Duration
}

// NewLoopExecute creates a new execution strategy manager.
func NewLoopExecute() LoopExecute {
	return LoopExecute{
		defaultTimeout: 60 * time.Second,
	}
}

// WithDefaultTimeout sets the default timeout for executions.
func (e LoopExecute) WithDefaultTimeout(d time.Duration) LoopExecute {
	e.defaultTimeout = d
	return e
}

// ExecutionStrategy determines how multiple operations are run.
type ExecutionStrategy int

const (
	// Sequential runs one at a time, in order.
	Sequential ExecutionStrategy = iota
	// Parallel runs all at once, waiting for all to complete.
	Parallel
	// ParallelOrdered runs in parallel but returns results in order.
	ParallelOrdered
	// Batched runs in parallel groups by concurrency safety.
	Batched
)

// Execute runs a single operation with timeout.
func (e LoopExecute) Execute(ctx context.Context, op Operation) LoopResult {
	timeout := op.Timeout
	if timeout == 0 {
		timeout = e.defaultTimeout
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	done := make(chan LoopResult, 1)
	go func() {
		done <- op.Fn(ctx)
	}()

	select {
	case result := <-done:
		return result
	case <-ctx.Done():
		return LoopResult{
			Success: false,
			Error:   NewLoopError("timeout", "operation timed out after "+timeout.String()),
		}
	}
}

// ExecuteMany runs multiple operations with the given strategy.
func (e LoopExecute) ExecuteMany(ctx context.Context, strategy ExecutionStrategy, ops []Operation) []LoopResult {
	switch strategy {
	case Parallel:
		return e.executeParallel(ctx, ops)
	case ParallelOrdered:
		return e.executeParallelOrdered(ctx, ops)
	case Batched:
		return e.executeBatched(ctx, ops)
	default: // Sequential
		return e.executeSequential(ctx, ops)
	}
}

// Operation is a unit of work for execution.
type Operation struct {
	Name     string
	Fn       func(ctx context.Context) LoopResult
	Timeout  time.Duration
	Priority int // Higher = execute first
}

func (e LoopExecute) executeSequential(ctx context.Context, ops []Operation) []LoopResult {
	results := make([]LoopResult, len(ops))
	for i, op := range ops {
		results[i] = e.Execute(ctx, op)
	}
	return results
}

func (e LoopExecute) executeParallel(ctx context.Context, ops []Operation) []LoopResult {
	results := make([]LoopResult, len(ops))
	var wg sync.WaitGroup
	wg.Add(len(ops))

	for i, op := range ops {
		go func(idx int, operation Operation) {
			defer wg.Done()
			results[idx] = e.Execute(ctx, operation)
		}(i, op)
	}

	wg.Wait()
	return results
}

func (e LoopExecute) executeParallelOrdered(ctx context.Context, ops []Operation) []LoopResult {
	// Similar to parallel but ensures results are in original order
	results := make([]LoopResult, len(ops))
	type indexedResult struct {
		idx    int
		result LoopResult
	}

	resultChan := make(chan indexedResult, len(ops))
	var wg sync.WaitGroup
	wg.Add(len(ops))

	for i, op := range ops {
		go func(idx int, operation Operation) {
			defer wg.Done()
			resultChan <- indexedResult{idx: idx, result: e.Execute(ctx, operation)}
		}(i, op)
	}

	go func() {
		wg.Wait()
		close(resultChan)
	}()

	for ir := range resultChan {
		results[ir.idx] = ir.result
	}
	return results
}

func (e LoopExecute) executeBatched(ctx context.Context, ops []Operation) []LoopResult {
	// Group operations by whether they're "safe" for concurrency
	// For now, treat all as sequential (safe default)
	return e.executeSequential(ctx, ops)
}

// RetryConfig controls retry behavior.
type RetryConfig struct {
	MaxAttempts int
	Backoff     time.Duration
	MaxBackoff  time.Duration
	ShouldRetry func(LoopResult) bool
}

// DefaultRetryConfig returns sensible retry settings.
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxAttempts: 3,
		Backoff:     500 * time.Millisecond,
		MaxBackoff:  5 * time.Second,
		ShouldRetry: func(r LoopResult) bool {
			return !r.Success && r.Retryable
		},
	}
}

// ExecuteWithRetry runs an operation with retries.
func (e LoopExecute) ExecuteWithRetry(ctx context.Context, op Operation, cfg RetryConfig) LoopResult {
	var lastResult LoopResult

	for attempt := 0; attempt < cfg.MaxAttempts; attempt++ {
		lastResult = e.Execute(ctx, op)
		if lastResult.Success || !cfg.ShouldRetry(lastResult) {
			return lastResult
		}

		if attempt < cfg.MaxAttempts-1 {
			backoff := cfg.Backoff * time.Duration(attempt+1)
			if backoff > cfg.MaxBackoff {
				backoff = cfg.MaxBackoff
			}
			select {
			case <-time.After(backoff):
				continue
			case <-ctx.Done():
				return LoopResult{
					Success: false,
					Error:   NewLoopError("retry_timeout", "retry aborted due to context cancellation"),
				}
			}
		}
	}

	return lastResult
}
