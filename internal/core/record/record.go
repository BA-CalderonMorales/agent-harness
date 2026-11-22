// Package record folds the events a session already produced into a small,
// bounded, deterministic structured record: the goal, the constraints the
// user stated, the files touched, the commands run, the plan, what was
// verified, and what is still pending.
//
// The package is pure. It does no I/O, reads no clock, draws no randomness,
// and holds no globals, so the same messages always fold to the same record
// and the same record always renders to the same bytes.
package record

// Version is the record's own schema version. The session header's Version
// is a mutation counter, not a schema version, so the record carries this.
const Version = 1

// Hard caps. Every collection and string in a Record is bounded so the record
// can never grow with the length of the session.
const (
	MaxGoal          = 300
	MaxConstraints   = 5
	MaxConstraintLen = 100
	MaxFiles         = 12
	MaxPathLen       = 120
	MaxCmds          = 8
	MaxCmdLen        = 80
	MaxErrLen        = 80
	MaxTodos         = 8
	MaxTodoLen       = 80
	MaxVerified      = 4
	MaxPending       = 6
	MaxPendingLen    = 200
	MaxDenied        = 3
	MaxDeniedLen     = 80

	maxOpenCalls      = 8
	maxOpenCallArgLen = 200
)

// File operations a FileTouch can carry.
const (
	OpRead  = "read"
	OpWrite = "write"
	OpEdit  = "edit"
)

// FileTouch is one file the session read or changed.
type FileTouch struct {
	Path string `json:"path"`
	Op   string `json:"op"`
	N    int    `json:"n"`
}

// CmdRun is one finished shell command. Err is a truncated head of the error
// text, never a parsed exit code.
type CmdRun struct {
	Cmd    string `json:"cmd"`
	Failed bool   `json:"failed,omitempty"`
	Err    string `json:"err,omitempty"`
}

// Todo is one entry of the latest todo_write plan.
type Todo struct {
	Text   string `json:"text"`
	Status string `json:"status"`
}

// OpenCall is a tool call whose result has not arrived yet. It lets a result
// be matched to its call by id, and is dropped when the result lands.
type OpenCall struct {
	ID   string `json:"id"`
	Tool string `json:"tool"`
	Arg  string `json:"arg"`
}

// Record is the structured session record. All slices are oldest first.
type Record struct {
	V           int         `json:"v"`
	Goal        string      `json:"goal,omitempty"`
	Constraints []string    `json:"constraints,omitempty"`
	Files       []FileTouch `json:"files,omitempty"`
	Cmds        []CmdRun    `json:"cmds,omitempty"`
	Todos       []Todo      `json:"todos,omitempty"`
	Verified    []string    `json:"verified,omitempty"`
	Pending     []string    `json:"pending,omitempty"`
	Denied      []string    `json:"denied,omitempty"`

	// Bookkeeping that Apply needs between messages. Rendering ignores it.
	LastUser string     `json:"last_user,omitempty"`
	Replied  bool       `json:"replied,omitempty"`
	Open     []OpenCall `json:"open,omitempty"`
	// Undo holds the plan each unanswered todo_write proposal replaced.
	Undo []TodoUndo `json:"undo,omitempty"`
	// PlanSeq counts todo proposals and names the one whose plan is current.
	PlanSeq int `json:"plan_seq,omitempty"`
}

// IsZero reports whether nothing has been recorded.
func (r Record) IsZero() bool {
	return r.V == 0 && r.Goal == "" && len(r.Constraints) == 0 && len(r.Files) == 0 &&
		len(r.Cmds) == 0 && len(r.Todos) == 0 && len(r.Verified) == 0 &&
		len(r.Pending) == 0 && len(r.Denied) == 0 && r.LastUser == "" && !r.Replied && len(r.Open) == 0 && len(r.Undo) == 0 && r.PlanSeq == 0
}
