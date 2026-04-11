// Package services provides the modular service architecture.
//
// Services are decomposed into focused interfaces.
//
// Core principle: Each bucket implements ServiceBase but only handles
// what it knows. The ServiceOrchestrator coordinates without digging
// into bucket internals.
package services

import (
	"context"
)

// ServiceBase is the fundamental interface all service buckets must implement.
type ServiceBase interface {
	// Name returns the bucket identifier (e.g., "mcp", "compact")
	Name() string

	// CanHandle determines if this bucket handles the service operation.
	CanHandle(service string, operation string) bool

	// Execute runs the service operation.
	Execute(ctx context.Context, req ServiceRequest) ServiceResult

	// Capabilities describes what this bucket can do.
	Capabilities() ServiceBucketCapabilities
}

// ServiceRequest carries request context.
type ServiceRequest struct {
	Service   string
	Operation string
	Params    map[string]any
}

// ServiceResult is the output.
type ServiceResult struct {
	Success bool
	Data    any
	Error   error
}

// ServiceBucketCapabilities describes capabilities.
type ServiceBucketCapabilities struct {
	IsAsync     bool // Can run asynchronously
	IsStateful  bool // Maintains state between calls
	Category    string
}

// ServiceOrchestrator coordinates multiple ServiceBase implementations.
type ServiceOrchestrator struct {
	buckets []ServiceBase
}

// NewServiceOrchestrator creates a new orchestrator.
func NewServiceOrchestrator(buckets ...ServiceBase) *ServiceOrchestrator {
	return &ServiceOrchestrator{buckets: buckets}
}

// RegisterBucket adds a bucket.
func (o *ServiceOrchestrator) RegisterBucket(bucket ServiceBase) {
	o.buckets = append(o.buckets, bucket)
}

// Execute routes and executes.
func (o *ServiceOrchestrator) Execute(ctx context.Context, req ServiceRequest) ServiceResult {
	for _, bucket := range o.buckets {
		if bucket.CanHandle(req.Service, req.Operation) {
			return bucket.Execute(ctx, req)
		}
	}
	return ServiceResult{Success: false, Error: ErrNoBucket}
}

// ErrNoBucket is returned when no bucket can handle.
var ErrNoBucket = &ServiceError{Code: "no_bucket", Message: "no bucket for service"}

// ServiceError represents a service error.
type ServiceError struct {
	Code    string
	Message string
}

func (e *ServiceError) Error() string {
	return e.Code + ": " + e.Message
}
