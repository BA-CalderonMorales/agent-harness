// Package feedback records friction a user reports with /feedback. Records
// stay on this machine: one JSON line per report, appended to a local
// file. No conversation content, no user identity, no network.
package feedback

// Category classifies a report. Only these four exist; a report whose
// first word is none of them is filed under CategoryOther.
type Category string

const (
	CategoryBug      Category = "bug"
	CategoryAnnoying Category = "annoying"
	CategoryMissing  Category = "missing"
	CategoryOther    Category = "other"
)

// Record is one saved report. Its JSON fields are the complete agreed set;
// adding a field is a change to the on-disk format and needs a new test.
type Record struct {
	Time           string   `json:"time"`
	SessionID      string   `json:"session_id"`
	Provider       string   `json:"provider"`
	Model          string   `json:"model"`
	HarnessVersion string   `json:"harness_version"`
	Category       Category `json:"category"`
	Text           string   `json:"text"`
}
