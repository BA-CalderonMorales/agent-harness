// Package config provides the modular configuration architecture.
//
// Config sources are decomposed into focused interfaces.
//
// Core principle: Each bucket implements ConfigBase but only handles
// what it knows. The ConfigOrchestrator coordinates without digging
// into bucket internals.
package config

// ConfigBase is the fundamental interface all config buckets must implement.
type ConfigBase interface {
	// Name returns the bucket identifier (e.g., "env", "file", "flags")
	Name() string

	// CanHandle determines if this bucket provides the given config key.
	CanHandle(key string) bool

	// Get retrieves a config value by key.
	Get(key string) (any, bool)

	// Capabilities describes what this bucket can do.
	Capabilities() ConfigBucketCapabilities
}

// ConfigBucketCapabilities describes capabilities.
type ConfigBucketCapabilities struct {
	Priority   int    // Lower = higher priority
	IsMutable  bool   // Can be changed at runtime
	Category   string // "env", "file", "flags"
}

// ConfigOrchestrator coordinates multiple ConfigBase implementations.
type ConfigOrchestrator struct {
	buckets []ConfigBase
}

// NewConfigOrchestrator creates a new orchestrator.
func NewConfigOrchestrator(buckets ...ConfigBase) *ConfigOrchestrator {
	return &ConfigOrchestrator{buckets: buckets}
}

// RegisterBucket adds a bucket.
func (o *ConfigOrchestrator) RegisterBucket(bucket ConfigBase) {
	o.buckets = append(o.buckets, bucket)
}

// Get retrieves config from the first matching bucket.
func (o *ConfigOrchestrator) Get(key string) (any, bool) {
	for _, bucket := range o.buckets {
		if bucket.CanHandle(key) {
			return bucket.Get(key)
		}
	}
	return nil, false
}

// GetString retrieves a string config value.
func (o *ConfigOrchestrator) GetString(key string) string {
	val, ok := o.Get(key)
	if !ok {
		return ""
	}
	s, _ := val.(string)
	return s
}

// GetBool retrieves a bool config value.
func (o *ConfigOrchestrator) GetBool(key string) bool {
	val, ok := o.Get(key)
	if !ok {
		return false
	}
	b, _ := val.(bool)
	return b
}
