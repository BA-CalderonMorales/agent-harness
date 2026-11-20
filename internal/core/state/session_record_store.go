package state

import (
	"encoding/json"
	"os"
	"strings"

	"github.com/BA-CalderonMorales/agent-harness/internal/core/record"
)

// recordLinePrefix opens every record line: storeEvent marshals Kind first,
// and no message line starts this way.
const recordLinePrefix = `{"type":"record"`

// lastRecordEvent returns the newest record line in a session file, or the
// zero Record when there is none or it cannot be read. It is how a save
// learns what is already on disk, so an unchanged record is not re-written.
func lastRecordEvent(path string) record.Record {
	data, err := os.ReadFile(path)
	if err != nil {
		return record.Record{}
	}
	var last record.Record
	for _, line := range strings.Split(string(data), "\n") {
		if !strings.HasPrefix(line, recordLinePrefix) {
			continue
		}
		var event storeEvent
		if json.Unmarshal([]byte(line), &event) == nil && event.Record != nil {
			last = *event.Record
		}
	}
	return last
}
