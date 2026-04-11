// Package contextmgr provides the modular context management architecture.
//
// Context operations are decomposed into focused interfaces that can be implemented
// independently by "buckets".
//
// Core principle: Each bucket implements ContextBase but only handles
// what it knows. The ContextOrchestrator coordinates without digging
// into bucket internals.
package contextmgr

// ContextBase is the fundamental interface all context buckets must implement.
type ContextBase interface {
	// Name returns the bucket identifier (e.g., "compact", "token")
	Name() string

	// CanHandle determines if this bucket should handle the operation.
	CanHandle(operation string) bool

	// Execute runs the context operation.
	Execute(ctx ContextExecutionContext) ContextResult

	// Capabilities describes what this bucket can do.
	Capabilities() ContextBucketCapabilities
}

// ContextExecutionContext carries execution context.
type ContextExecutionContext struct {
	Operation string
	Messages  []any // Simplified - actual type depends on implementation
	Params    map[string]any
}

// ContextResult is the output.
type ContextResult struct {
	Success  bool
	Data     any
	Messages []any
	Error    error
}

// ContextBucketCapabilities describes capabilities.
type ContextBucketCapabilities struct {
	IsDestructive bool // Can modify/delete messages
	Category      string
}

// ContextOrchestrator coordinates multiple ContextBase implementations.
type ContextOrchestrator struct {
	buckets []ContextBase
}

// NewContextOrchestrator creates a new orchestrator.
func NewContextOrchestrator(buckets ...ContextBase) *ContextOrchestrator {
	return &ContextOrchestrator{buckets: buckets}
}

// RegisterBucket adds a bucket.
func (o *ContextOrchestrator) RegisterBucket(bucket ContextBase) {
	o.buckets = append(o.buckets, bucket)
}

// Execute routes and executes.
func (o *ContextOrchestrator) Execute(ctx ContextExecutionContext) ContextResult {
	for _, bucket := range o.buckets {
		if bucket.CanHandle(ctx.Operation) {
			return bucket.Execute(ctx)
		}
	}
	return ContextResult{Success: false, Error: ErrNoBucket}
}

// ErrNoBucket is returned when no bucket can handle.
var ErrNoBucket = &ContextError{Code: "no_bucket", Message: "no bucket for operation"}

// ContextError represents a context error.
type ContextError struct {
	Code    string
	Message string
}

func (e *ContextError) Error() string {
	return e.Code + ": " + e.Message
}
