package tools

import "testing"

func TestTryRecordResultEnforcesToolAndTurnLimits(t *testing.T) {
	budget := NewContentBudget(1_000)

	if limit, admitted := budget.TryRecordResult("bounded", 101, 100); admitted || limit != 100 {
		t.Fatalf("tool-limit admission = (%d, %v), want (100, false)", limit, admitted)
	}
	if limit, admitted := budget.TryRecordResult("bounded", 100, 100); !admitted || limit != 100 {
		t.Fatalf("bounded admission = (%d, %v), want (100, true)", limit, admitted)
	}
	if limit, admitted := budget.TryRecordResult("second", 901, 1_000); admitted || limit != 900 {
		t.Fatalf("turn-limit admission = (%d, %v), want (900, false)", limit, admitted)
	}

	used, max := budget.CurrentUsage()
	if used != 100 || max != 1_000 {
		t.Fatalf("budget usage = %d/%d, want 100/1000", used, max)
	}
}

func TestTryRecordResultPreservesExplicitExemptions(t *testing.T) {
	budget := NewContentBudget(10)
	budget.MarkToolExempt("read")

	if _, admitted := budget.TryRecordResult("read", 10_000, 1); !admitted {
		t.Fatal("explicitly exempt result was rejected")
	}
	if used, _ := budget.CurrentUsage(); used != 0 {
		t.Fatalf("exempt result consumed %d budget chars, want 0", used)
	}
}
