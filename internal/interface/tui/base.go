// Package tui provides the modular TUI architecture.
//
// UI components are decomposed into focused interfaces.
//
// Core principle: Each bucket implements TUIBase but only handles
// what it knows. The TUIOrchestrator coordinates without digging
// into bucket internals.
package tui

import (
	"github.com/BA-CalderonMorales/agent-harness/pkg/types"
)

// TUIBase is the fundamental interface all TUI buckets must implement.
type TUIBase interface {
	// Name returns the bucket identifier (e.g., "input", "render", "stream")
	Name() string

	// CanHandle determines if this bucket handles the component.
	CanHandle(component string) bool

	// Render renders the component.
	Render(ctx TUIRenderContext) TUIResult

	// Capabilities describes what this bucket can do.
	Capabilities() TUIBucketCapabilities
}

// TUIRenderContext carries render context.
type TUIRenderContext struct {
	Component string
	Data      any
	Events    chan<- types.StreamEvent
}

// TUIResult is the render output.
type TUIResult struct {
	Success bool
	Error   error
}

// TUIBucketCapabilities describes capabilities.
type TUIBucketCapabilities struct {
	IsInteractive bool // Handles user input
	IsStreaming   bool // Handles real-time updates
	Category      string
}

// TUIOrchestrator coordinates multiple TUIBase implementations.
type TUIOrchestrator struct {
	buckets []TUIBase
}

// NewTUIOrchestrator creates a new orchestrator.
func NewTUIOrchestrator(buckets ...TUIBase) *TUIOrchestrator {
	return &TUIOrchestrator{buckets: buckets}
}

// RegisterBucket adds a bucket.
func (o *TUIOrchestrator) RegisterBucket(bucket TUIBase) {
	o.buckets = append(o.buckets, bucket)
}

// Render routes and renders.
func (o *TUIOrchestrator) Render(ctx TUIRenderContext) TUIResult {
	for _, bucket := range o.buckets {
		if bucket.CanHandle(ctx.Component) {
			return bucket.Render(ctx)
		}
	}
	return TUIResult{Success: false, Error: ErrNoBucket}
}

// ErrNoBucket is returned when no bucket can handle.
var ErrNoBucket = &TUIError{Code: "no_bucket", Message: "no bucket for component"}

// TUIError represents a TUI error.
type TUIError struct {
	Code    string
	Message string
}

func (e *TUIError) Error() string {
	return e.Code + ": " + e.Message
}
