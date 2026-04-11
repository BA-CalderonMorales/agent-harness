package loop

import (
	"sync"
	"time"

	"github.com/BA-CalderonMorales/agent-harness/pkg/types"
)

// LoopResults manages result collection and aggregation across buckets.
// It provides thread-safe access to results and supports streaming.
type LoopResults struct {
	mu       sync.RWMutex
	results  []LoopResult
	messages []types.Message
	metadata map[string]any
}

// NewLoopResults creates a fresh results container.
func NewLoopResults() LoopResults {
	return LoopResults{
		results:  make([]LoopResult, 0),
		messages: make([]types.Message, 0),
		metadata: make(map[string]any),
	}
}

// Add appends a result to the collection.
func (r *LoopResults) Add(result LoopResult) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.results = append(r.results, result)
	r.messages = append(r.messages, result.Messages...)
}

// GetAll returns all collected results.
func (r *LoopResults) GetAll() []LoopResult {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]LoopResult, len(r.results))
	copy(out, r.results)
	return out
}

// GetMessages returns all messages from all results.
func (r *LoopResults) GetMessages() []types.Message {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]types.Message, len(r.messages))
	copy(out, r.messages)
	return out
}

// HasFailures checks if any result failed.
func (r *LoopResults) HasFailures() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, r := range r.results {
		if !r.Success {
			return true
		}
	}
	return false
}

// GetFailures returns only failed results.
func (r *LoopResults) GetFailures() []LoopResult {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var failures []LoopResult
	for _, res := range r.results {
		if !res.Success {
			failures = append(failures, res)
		}
	}
	return failures
}

// SetMetadata attaches arbitrary metadata.
func (r *LoopResults) SetMetadata(key string, value any) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.metadata[key] = value
}

// GetMetadata retrieves metadata.
func (r *LoopResults) GetMetadata(key string) (any, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	v, ok := r.metadata[key]
	return v, ok
}

// ResultAggregator combines multiple results into a single coherent result.
// This is used when multiple tools run in parallel.
type ResultAggregator struct {
	results []LoopResult
}

// NewAggregator creates an aggregator.
func NewAggregator() *ResultAggregator {
	return &ResultAggregator{results: make([]LoopResult, 0)}
}

// Add includes a result in the aggregation.
func (a *ResultAggregator) Add(r LoopResult) {
	a.results = append(a.results, r)
}

// Aggregate combines all results.
// - If any result has ShouldHalt=true, the aggregate halts
// - If any result fails, the aggregate is a failure
// - Messages are concatenated
func (a *ResultAggregator) Aggregate() LoopResult {
	if len(a.results) == 0 {
		return LoopResult{Success: true}
	}

	if len(a.results) == 1 {
		return a.results[0]
	}

	agg := LoopResult{
		Success:    true,
		Messages:   make([]types.Message, 0),
		ShouldHalt: false,
	}

	var errors []LoopError
	for _, r := range a.results {
		if !r.Success {
			agg.Success = false
			errors = append(errors, r.Error)
		}
		agg.Messages = append(agg.Messages, r.Messages...)
		if r.ShouldHalt {
			agg.ShouldHalt = true
		}
	}

	if len(errors) > 0 {
		agg.Error = NewLoopError("aggregate", "multiple errors occurred")
	}

	return agg
}

// TimedResult wraps a result with timing information.
type TimedResult struct {
	Result    LoopResult
	StartTime time.Time
	EndTime   time.Time
	Duration  time.Duration
}

// Elapsed returns how long the operation took.
func (t TimedResult) Elapsed() time.Duration {
	return t.Duration
}

// WithTiming wraps a function call with timing.
func WithTiming(fn func() LoopResult) TimedResult {
	start := time.Now()
	result := fn()
	end := time.Now()
	return TimedResult{
		Result:    result,
		StartTime: start,
		EndTime:   end,
		Duration:  end.Sub(start),
	}
}
