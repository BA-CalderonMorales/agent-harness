// Package state provides the modular state management architecture.
//
// State backends are decomposed into focused interfaces that can be implemented
// independently by "buckets" - storage-specific implementations.
//
// Core principle: Each bucket implements StateBase but only handles
// what it knows. The StateOrchestrator coordinates without digging
// into bucket internals.
package state

import (
	"sync"
)

// StateBase is the fundamental interface all state buckets must implement.
type StateBase interface {
	// Name returns the bucket identifier (e.g., "memory", "persistent")
	Name() string

	// CanHandle determines if this bucket should handle the given key/operation.
	CanHandle(key string, operation string) bool

	// Get retrieves a value by key.
	Get(key string) (any, bool)

	// Set stores a value by key.
	Set(key string, value any)

	// Delete removes a key.
	Delete(key string)

	// Capabilities describes what this bucket can do.
	Capabilities() StateBucketCapabilities
}

// StateBucketCapabilities describes static capabilities.
type StateBucketCapabilities struct {
	IsPersistent bool // Survives process restart
	IsThreadSafe bool // Safe for concurrent access
	SupportsTTL  bool // Supports time-to-live
	Category     string
}

// StateOrchestrator coordinates multiple StateBase implementations.
type StateOrchestrator struct {
	buckets []StateBase
	mu      sync.RWMutex
}

// NewStateOrchestrator creates a new orchestrator.
func NewStateOrchestrator(buckets ...StateBase) *StateOrchestrator {
	return &StateOrchestrator{buckets: buckets}
}

// RegisterBucket adds a bucket.
func (o *StateOrchestrator) RegisterBucket(bucket StateBase) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.buckets = append(o.buckets, bucket)
}

// FindBucket locates the appropriate bucket for a key.
func (o *StateOrchestrator) FindBucket(key string) (StateBase, bool) {
	o.mu.RLock()
	defer o.mu.RUnlock()
	for _, bucket := range o.buckets {
		if bucket.CanHandle(key, "get") {
			return bucket, true
		}
	}
	return nil, false
}

// Get retrieves from appropriate bucket.
func (o *StateOrchestrator) Get(key string) (any, bool) {
	bucket, found := o.FindBucket(key)
	if !found {
		return nil, false
	}
	return bucket.Get(key)
}

// Set stores in appropriate bucket.
func (o *StateOrchestrator) Set(key string, value any) {
	bucket, found := o.FindBucket(key)
	if !found {
		return
	}
	bucket.Set(key, value)
}

// Delete removes from appropriate bucket.
func (o *StateOrchestrator) Delete(key string) {
	bucket, found := o.FindBucket(key)
	if !found {
		return
	}
	bucket.Delete(key)
}
