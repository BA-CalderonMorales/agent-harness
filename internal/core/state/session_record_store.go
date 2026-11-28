package state

import (
	"bufio"
	"bytes"
	"encoding/json"
	"os"

	"github.com/BA-CalderonMorales/agent-harness/internal/core/record"
)

// recordLinePrefix opens every record line: storeEvent marshals Kind first,
// and no message line starts this way.
const recordLinePrefix = `{"type":"record"`

var recordLinePrefixBytes = []byte(recordLinePrefix)

// lastRecordEvent returns the newest record line in a session file, or the
// zero Record when there is none or it cannot be read. It streams line-by-line
// so reading large transcripts avoids allocating the whole file at once.
func lastRecordEvent(path string) record.Record {
	f, err := os.Open(path)
	if err != nil {
		return record.Record{}
	}
	defer func() { _ = f.Close() }()

	reader := bufio.NewReaderSize(f, 32*1024)
	var last record.Record
	for {
		line, err := reader.ReadBytes('\n')
		if len(line) > 0 {
			trimmed := bytes.TrimSpace(line)
			if bytes.HasPrefix(trimmed, recordLinePrefixBytes) {
				var event storeEvent
				if json.Unmarshal(trimmed, &event) == nil && event.Record != nil {
					last = *event.Record
				}
			}
		}
		if err != nil {
			return last
		}
	}
}
