package record

import (
	"strings"
	"testing"
)

func TestHugeCallIDMatchesResultAndStaysBounded(t *testing.T) {
	id := strings.Repeat("x", 10000)
	r := fold(fileCall(id, "read", "a.go"))
	if len(r.Files) != 1 {
		t.Fatalf("result did not match its call: %+v", r)
	}
	r = Apply(Record{}, use(id, "bash", map[string]any{"command": "go test"}))
	if len(r.Open) != 1 || len(r.Open[0].ID) > maxCallIDLen {
		t.Fatalf("open id not bounded: %d", len(r.Open[0].ID))
	}
	if out := Render(r, 300); (len(out)+3)/4 > 300 {
		t.Fatal("render over budget")
	}
	r = Apply(r, result(id, "ok", false))
	if len(r.Open) != 0 || len(r.Cmds) != 1 {
		t.Fatalf("huge id never resolved: %+v", r)
	}
}

func TestBoundIDKeepsLongIDsDistinct(t *testing.T) {
	prefix := strings.Repeat("p", 30)
	a, b := boundID(prefix+"-one"+strings.Repeat("z", 100)), boundID(prefix+"-two"+strings.Repeat("z", 100))
	if a == b {
		t.Fatal("ids sharing a prefix collided")
	}
	if boundID("short-id") != "short-id" {
		t.Fatal("short ids must be unchanged")
	}
}
