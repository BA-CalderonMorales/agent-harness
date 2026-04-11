package defaults

import "time"

// Plan bucket defaults
const (
	// Plan limits
	PlanMaxStepsDefault = 20
	PlanMaxStepsSmall   = 5
	PlanMaxStepsLarge   = 50

	// Step descriptions
	PlanMaxStepLength = 500
	PlanMinStepLength = 10

	// Timeouts
	PlanStepTimeout  = 10 * time.Minute
	PlanTotalTimeout = 60 * time.Minute

	// Approval
	PlanRequireApprovalDefault = true
	PlanAutoApproveSmall       = false
)

// PlanTemplates contains plan format templates
var PlanTemplates = map[string]string{
	"simple": `Plan: {{.Title}}
{{range .Steps}}{{.Number}}. {{.Description}}
{{end}}`,

	"detailed": `Plan: {{.Title}}
Started: {{.StartTime}}

{{range .Steps}}[{{.Status}}] {{.Number}}. {{.Description}}
{{if .Result}}   Result: {{.Result}}{{end}}
{{end}}`,

	"markdown": `# {{.Title}}

{{range .Steps}}## Step {{.Number}}: {{.Description}}
- Status: {{.Status}}
{{if .Result}}
- Result: {{.Result}}
{{end}}
{{end}}`,
}

// PlanStatusColors maps statuses to display colors (for TUI)
var PlanStatusColors = map[string]string{
	"pending": "gray",
	"active":  "blue",
	"done":    "green",
	"error":   "red",
	"skipped": "yellow",
}

// PlanStatusIcons maps statuses to display icons
var PlanStatusIcons = map[string]string{
	"pending": "○",
	"active":  "◐",
	"done":    "●",
	"error":   "✗",
	"skipped": "⊘",
}
