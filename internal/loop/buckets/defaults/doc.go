// Package defaults provides hardcoded configuration values for all buckets.
//
// This package is the single source of truth for:
//   - Default limits (file sizes, timeouts, result counts)
//   - Allowed/blocked lists (paths, commands, hosts)
//   - String constants and templates
//   - Safety thresholds
//
// Why this exists:
//   - Keeps bucket implementations clean (no hardcoded strings)
//   - Makes configuration changes in one place
//   - Enables easy customization for different environments
//   - Makes testing easier (can override defaults)
//
// Usage in buckets:
//
//	import "github.com/BA-CalderonMorales/agent-harness/internal/loop/buckets/defaults"
//	
//	func (b *MyBucket) Execute(ctx loop.ExecutionContext) loop.LoopResult {
//	    if size > defaults.FSMaxFileSize {
//	        return loop.LoopResult{Error: loop.NewLoopError("too_large", ...)}
//	    }
//	}
//
// Adding new defaults:
//   1. Create a new file in this package (e.g., myfeature.go)
//   2. Use const for simple values, var for complex types
//   3. Document what the default controls
//   4. Group related defaults in the same file
package defaults
