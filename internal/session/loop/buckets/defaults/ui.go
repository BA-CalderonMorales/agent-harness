package defaults

import "time"

// UI bucket defaults
const (
	// Export settings
	UIExportDirDefault = ".agent-harness/exports"
	UIExportMaxSize    = 100 * 1024 * 1024 // 100MB
	UIExportFormats    = "json,md,txt,html"

	// Notebook settings
	UINotebookDirDefault = ".agent-harness/notebooks"
	UINotebookMaxSize    = 10 * 1024 * 1024 // 10MB
	UINotebookMaxEntries = 1000

	// Todo settings
	UITodoMaxItems      = 100
	UITodoMaxItemLength = 500

	// Ask settings
	UIAskMaxOptions        = 10
	UIAskMaxQuestionLength = 2000
	UIAskTimeoutDefault    = 300 * time.Second // 5 minutes

	// Settings
	UISettingsMaxKeyLength   = 100
	UISettingsMaxValueLength = 10000
)

// UIExportTemplates contains templates for export formats
var UIExportTemplates = map[string]string{
	"json": "{{.JSON}}",
	"md": `# Session Export

**Date:** {{.Date}}
**Model:** {{.Model}}
**Messages:** {{.MessageCount}}

---

{{.Content}}
`,
	"txt": `Session Export
==============
Date: {{.Date}}
Model: {{.Model}}
Messages: {{.MessageCount}}

{{.Content}}
`,
	"html": `<!DOCTYPE html>
<html>
<head><title>Session Export</title></head>
<body>
<h1>Session Export</h1>
<p><strong>Date:</strong> {{.Date}}</p>
<p><strong>Model:</strong> {{.Model}}</p>
<hr>
{{.Content}}
</body>
</html>`,
}

// UISettingsDefaults contains default settings values
var UISettingsDefaults = map[string]string{
	"theme":               "dark",
	"auto_save":           "true",
	"confirm_destructive": "true",
	"stream_output":       "true",
	"show_token_count":    "true",
	"max_history":         "100",
}

// UITodoStatuses contains valid todo statuses
var UITodoStatuses = []string{
	"pending", "in_progress", "done", "cancelled", "blocked",
}
