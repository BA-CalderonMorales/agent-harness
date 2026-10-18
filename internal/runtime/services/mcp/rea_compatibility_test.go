package mcp

import (
	"context"
	"crypto/sha256"
	"debug/buildinfo"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// Opt in explicitly: the ordinary suite is offline and needs neither Node nor
// REA. This pins the published release, independently of the reference checkout.
func TestREACompatibilitySmoke(t *testing.T) {
	binary := os.Getenv("AGENT_HARNESS_REA_SMOKE_BINARY")
	if binary == "" {
		t.Skip("set AGENT_HARNESS_REA_SMOKE_BINARY to a built Go executable")
	}
	binary, err := filepath.Abs(binary)
	if err != nil {
		t.Fatal(err)
	}
	bytes, err := os.ReadFile(binary)
	if err != nil {
		t.Fatal(err)
	}
	build, err := buildinfo.ReadFile(binary)
	if err != nil {
		t.Fatal(err)
	}
	manager := NewManager()
	t.Cleanup(func() {
		if err := manager.Close(); err != nil {
			t.Error(err)
		}
	})
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	if err := manager.Connect(ctx, "rea", McpServerConfig{Command: "npx", Args: []string{
		"--yes", "--package=rea-agents@6.5.0", "rea", "--mcp",
	}}); err != nil {
		t.Fatal(err)
	}
	defs := manager.AllToolDefs()
	found := false
	for _, def := range defs {
		if def.Name == "inspect_go_binary" {
			found = true
		}
	}
	if !found {
		t.Fatal("REA did not advertise inspect_go_binary")
	}
	// A target error must stay a tool error, and must not poison the session.
	failed, err := manager.CallToolWithProgress(ctx, "rea", "inspect_go_binary", map[string]any{"path": filepath.Join(t.TempDir(), "missing-binary")}, nil)
	if err != nil || !failed.IsError {
		t.Fatalf("missing target lost tool-error boundary: %v, isError=%v", err, failed.IsError)
	}
	result, err := manager.CallToolWithProgress(ctx, "rea", "inspect_go_binary", map[string]any{"path": binary}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.IsError {
		t.Fatalf("valid inspection failed: %s", result.Content)
	}
	var envelope struct {
		Structured struct {
			EvidenceID string `json:"evidence_id"`
			Normalized struct {
				Artifact struct {
					SHA256 string `json:"sha256"`
					Bytes  int    `json:"bytes"`
				} `json:"artifact"`
				BuildInfo struct {
					GoVersion       string `json:"go_version"`
					VersionBytes    string `json:"go_version_bytes_base64"`
					ModuleBytes     string `json:"module_bytes_base64"`
					VersionLocation struct {
						Offset int `json:"offset"`
						Bytes  int `json:"bytes"`
					} `json:"version_location"`
					ModuleLocation struct {
						Offset int `json:"offset"`
						Bytes  int `json:"bytes"`
					} `json:"module_location"`
				} `json:"build_info"`
			} `json:"normalized_result"`
		} `json:"structuredContent"`
		Meta map[string]any `json:"_meta"`
	}
	if err := json.Unmarshal([]byte(result.Content), &envelope); err != nil {
		t.Fatal(err)
	}
	evidence := envelope.Structured
	if evidence.EvidenceID == "" || envelope.Meta["provenance"] == nil {
		t.Fatal("REA evidence identity/provenance was dropped")
	}
	sum := sha256.Sum256(bytes)
	if evidence.Normalized.Artifact.SHA256 != fmt.Sprintf("%x", sum) || evidence.Normalized.Artifact.Bytes != len(bytes) {
		t.Fatalf("REA artifact identity disagrees with local SHA-256/size: %+v", evidence.Normalized.Artifact)
	}
	info := evidence.Normalized.BuildInfo
	if info.GoVersion != build.GoVersion {
		t.Fatalf("REA compiler %q disagrees with Go debug/buildinfo %q", info.GoVersion, build.GoVersion)
	}
	for name, region := range map[string]struct {
		encoded      string
		offset, size int
	}{
		"compiler": {info.VersionBytes, info.VersionLocation.Offset, info.VersionLocation.Bytes},
		"module":   {info.ModuleBytes, info.ModuleLocation.Offset, info.ModuleLocation.Bytes},
	} {
		decoded, err := base64.StdEncoding.DecodeString(region.encoded)
		if err != nil {
			t.Fatal(err)
		}
		if region.size == 0 || region.offset < 0 || region.offset > len(bytes)-region.size {
			t.Fatalf("invalid %s evidence region", name)
		}
		if string(decoded) != string(bytes[region.offset:region.offset+region.size]) {
			t.Fatalf("%s evidence does not match raw artifact bytes", name)
		}
	}
	t.Logf("REA 6.5.0: %d tools; %s; compiler=%s; sha256=%x; source byte ranges verified; error then retry passed", len(defs), evidence.EvidenceID, build.GoVersion, sum)
}
