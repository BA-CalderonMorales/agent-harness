package defaults

import "time"

// Web bucket defaults
const (
	// Fetch settings
	WebFetchTimeout         = 30 * time.Second
	WebFetchMaxSize         = 5 * 1024 * 1024 // 5MB
	WebFetchMaxRedirects    = 10
	WebFetchUserAgent       = "Agent-Harness/1.0"

	// Content limits
	WebMaxContentLength     = 1024 * 1024 // 1MB
	WebMaxLineLength        = 10000

	// Retry settings
	WebMaxRetries           = 3
	WebRetryBackoff         = 2 * time.Second

	// Search settings  
	WebSearchMaxResults     = 10
	WebSearchTimeout        = 30 * time.Second
)

// WebAllowedSchemes contains allowed URL schemes
var WebAllowedSchemes = []string{
	"http", "https",
}

// WebBlockedHosts contains domains to block
var WebBlockedHosts = []string{
	"localhost",
	"127.0.0.1",
	"::1",
	"0.0.0.0",
	"[::]",
}

// WebBlockedExtensions are file types to skip
var WebBlockedExtensions = []string{
	".exe", ".dll", ".so", ".dylib", // Executables
	".zip", ".tar", ".gz", ".bz2", ".7z", // Archives
	".mp3", ".mp4", ".avi", ".mov", // Media
	".pdf", ".doc", ".docx", // Documents
}

// WebContentTypePriority for fetching
var WebContentTypePriority = []string{
	"text/html",
	"text/plain",
	"text/markdown",
	"application/json",
	"application/xml",
	"text/xml",
}
