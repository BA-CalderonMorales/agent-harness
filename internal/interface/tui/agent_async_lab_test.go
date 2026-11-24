package tui

import "testing"

// Both async query paths build their agent.QueryParams here, so the record
// compaction lab choice reaches the loop on either path.
func TestAsyncQueryParamsForwardTheRecordLab(t *testing.T) {
	for _, enabled := range []bool{true, false} {
		got := asyncQueryParams(AgentQueryParams{RecordCompaction: enabled})
		if got.RecordCompaction != enabled {
			t.Fatalf("RecordCompaction = %v, want %v: the async path dropped the lab choice", got.RecordCompaction, enabled)
		}
	}
}
