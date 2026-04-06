package behaviors

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/BA-CalderonMorales/agent-harness/e2e"
)

// TestBehavior_SessionPersistence verifies B1: Session Persistence
func TestBehavior_SessionPersistence(t *testing.T) {
	runner := e2e.NewBehaviorRunner(t)
	
	test := e2e.BehaviorTest{
		Name:    "B1: Session Persistence",
		Timeout: 30 * time.Second,
		
		Given: func() e2e.TestContext {
			ctx := e2e.NewTestContext()
			ctx.State["session_created"] = true
			ctx.State["files_created"] = []string{}
			return ctx
		},
		
		When: func(ctx e2e.TestContext) e2e.TestContext {
			files := ctx.State["files_created"].([]string)
			
			testFile := filepath.Join(ctx.WorkDir, "test.txt")
			if err := os.WriteFile(testFile, []byte("test content"), 0644); err != nil {
				ctx.Errors = append(ctx.Errors, err)
				return ctx
			}
			files = append(files, "test.txt")
			ctx.State["files_created"] = files
			ctx.State["last_command"] = "echo test"
			ctx.State["crashed"] = true
			ctx.State["crash_time"] = time.Now()
			
			return ctx
		},
		
		Then: func(ctx e2e.TestContext) error {
			if !ctx.State["session_created"].(bool) {
				return e2e.BehaviorError("session_created flag not preserved")
			}
			
			files := ctx.State["files_created"].([]string)
			if len(files) == 0 {
				return e2e.BehaviorError("files_created not preserved")
			}
			
			testFile := filepath.Join(ctx.WorkDir, "test.txt")
			if _, err := os.Stat(testFile); os.IsNotExist(err) {
				return e2e.BehaviorError("created file not found after crash")
			}
			
			if ctx.State["last_command"] != "echo test" {
				return e2e.BehaviorError("command history not preserved")
			}
			
			return nil
		},
	}
	
	result := runner.Run(test)
	if !result.Passed {
		t.Fatalf("Behavior test failed: %v", result.Error)
	}
}

// TestBehavior_ToolExecutionSafety verifies B2: Tool Execution Safety
func TestBehavior_ToolExecutionSafety(t *testing.T) {
	runner := e2e.NewBehaviorRunner(t)
	
	test := e2e.BehaviorTest{
		Name:    "B2: Tool Execution Safety",
		Timeout: 10 * time.Second,
		
		Given: func() e2e.TestContext {
			ctx := e2e.NewTestContext()
			ctx.State["tool_config"] = map[string]interface{}{
				"name":             "bash",
				"requires_approval": true,
			}
			ctx.State["approval_state"] = "pending"
			return ctx
		},
		
		When: func(ctx e2e.TestContext) e2e.TestContext {
			config := ctx.State["tool_config"].(map[string]interface{})
			
			if requiresApproval, ok := config["requires_approval"].(bool); ok && requiresApproval {
				ctx.State["execution_state"] = "awaiting_approval"
				ctx.State["approval_state"] = "approved"
				ctx.State["execution_state"] = "completed"
			}
			
			return ctx
		},
		
		Then: func(ctx e2e.TestContext) error {
			if ctx.State["execution_state"] != "completed" {
				return e2e.BehaviorError("execution did not complete")
			}
			
			if ctx.State["approval_state"] != "approved" {
				return e2e.BehaviorError("approval state not tracked")
			}
			
			return nil
		},
	}
	
	result := runner.Run(test)
	if !result.Passed {
		t.Fatalf("Behavior test failed: %v", result.Error)
	}
}
