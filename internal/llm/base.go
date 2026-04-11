// Package llm provides the modular LLM provider architecture.
//
// LLM providers are decomposed into focused interfaces that can be implemented
// independently by "buckets" - provider-specific implementations.
//
// Core principle: Each bucket implements LLMBase but only handles
// what it knows. The LLMOrchestrator coordinates without digging
// into bucket internals.
package llm

import (
	"context"

	"github.com/BA-CalderonMorales/agent-harness/pkg/types"
)

// LLMBase is the fundamental interface all LLM provider buckets must implement.
type LLMBase interface {
	// Name returns the bucket identifier (e.g., "openrouter", "anthropic", "ollama")
	Name() string

	// CanHandle determines if this bucket should handle the given model/request.
	CanHandle(model string, params map[string]any) bool

	// Stream sends a request and returns a stream of events.
	Stream(ctx context.Context, req Request) (<-chan types.LLMEvent, error)

	// Capabilities describes what this bucket can do.
	Capabilities() LLMBucketCapabilities
}

// LLMBucketCapabilities describes static capabilities of an LLM bucket.
type LLMBucketCapabilities struct {
	Provider      string   // "openrouter", "anthropic", "ollama"
	Models        []string // Supported model names
	SupportsTools bool
	SupportsStreaming bool
}

// LLMOrchestrator coordinates multiple LLMBase implementations.
type LLMOrchestrator struct {
	buckets []LLMBase
}

// NewLLMOrchestrator creates a new orchestrator.
func NewLLMOrchestrator(buckets ...LLMBase) *LLMOrchestrator {
	return &LLMOrchestrator{buckets: buckets}
}

// RegisterBucket adds a bucket.
func (o *LLMOrchestrator) RegisterBucket(bucket LLMBase) {
	o.buckets = append(o.buckets, bucket)
}

// FindBucket locates the appropriate bucket for a model.
func (o *LLMOrchestrator) FindBucket(model string) (LLMBase, bool) {
	for _, bucket := range o.buckets {
		if bucket.CanHandle(model, nil) {
			return bucket, true
		}
	}
	return nil, false
}

// Stream routes to appropriate bucket and streams.
func (o *LLMOrchestrator) Stream(ctx context.Context, req Request) (<-chan types.LLMEvent, error) {
	bucket, found := o.FindBucket(req.Model)
	if !found {
		return nil, &LLMError{Code: "no_provider", Message: "no provider for model: " + req.Model}
	}
	return bucket.Stream(ctx, req)
}

// LLMError represents an LLM error.
type LLMError struct {
	Code    string
	Message string
	Cause   error
}

func (e *LLMError) Error() string {
	if e.Cause != nil {
		return e.Code + ": " + e.Message + " (" + e.Cause.Error() + ")"
	}
	return e.Code + ": " + e.Message
}
