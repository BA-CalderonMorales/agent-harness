// Package e2e provides behavior-based testing harness for agent repositories
package e2e

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// BehaviorTest defines a behavior-based test
type BehaviorTest struct {
	Name        string
	Given       func() TestContext
	When        func(TestContext) TestContext
	Then        func(TestContext) error
	Timeout     time.Duration
	Cleanup     func(TestContext)
}

// TestContext holds state during test execution
type TestContext struct {
	SessionID   string
	WorkDir     string
	State       map[string]interface{}
	Errors      []error
	Artifacts   map[string]string
}

// NewTestContext creates a fresh test context
func NewTestContext() TestContext {
	return TestContext{
		SessionID: generateSessionID(),
		WorkDir:   createTempWorkDir(),
		State:     make(map[string]interface{}),
		Errors:    make([]error, 0),
		Artifacts: make(map[string]string),
	}
}

// BehaviorRunner executes behavior tests
type BehaviorRunner struct {
	t          *testing.T
	baseline   map[string]TestResult
	hooks      RunnerHooks
}

// RunnerHooks for test lifecycle
type RunnerHooks struct {
	BeforeEach func(BehaviorTest) error
	AfterEach  func(BehaviorTest, TestResult) error
	OnFailure  func(BehaviorTest, error)
}

// TestResult captures test execution result
type TestResult struct {
	Name      string
	Passed    bool
	Duration  time.Duration
	Error     error
	Artifacts map[string]string
}

// NewBehaviorRunner creates a test runner
func NewBehaviorRunner(t *testing.T) *BehaviorRunner {
	return &BehaviorRunner{
		t:        t,
		baseline: make(map[string]TestResult),
	}
}

// Run executes a behavior test
func (r *BehaviorRunner) Run(test BehaviorTest) TestResult {
	start := time.Now()
	result := TestResult{Name: test.Name}
	
	ctx, cancel := context.WithTimeout(context.Background(), test.Timeout)
	defer cancel()
	
	// Execute hooks
	if r.hooks.BeforeEach != nil {
		if err := r.hooks.BeforeEach(test); err != nil {
			result.Error = fmt.Errorf("before hook failed: %w", err)
			result.Duration = time.Since(start)
			return result
		}
	}
	
	// Execute test phases
	testCtx := test.Given()
	defer func() {
		if test.Cleanup != nil {
			test.Cleanup(testCtx)
		}
		cleanupWorkDir(testCtx.WorkDir)
	}()
	
	// Run When phase
	testCtx = test.When(testCtx)
	
	// Run Then phase
	if err := test.Then(testCtx); err != nil {
		result.Error = err
		result.Passed = false
		if r.hooks.OnFailure != nil {
			r.hooks.OnFailure(test, err)
		}
	} else {
		result.Passed = true
	}
	
	result.Duration = time.Since(start)
	result.Artifacts = testCtx.Artifacts
	
	// Execute after hook
	if r.hooks.AfterEach != nil {
		r.hooks.AfterEach(test, result)
	}
	
	return result
}

// RunAll executes multiple behavior tests
func (r *BehaviorRunner) RunAll(tests []BehaviorTest) []TestResult {
	results := make([]TestResult, len(tests))
	for i, test := range tests {
		results[i] = r.Run(test)
	}
	return results
}

// CompareWithBaseline checks if results match baseline
func (r *BehaviorRunner) CompareWithBaseline(results []TestResult) []Regression {
	var regressions []Regression
	for _, result := range results {
		if baseline, ok := r.baseline[result.Name]; ok {
			if baseline.Passed && !result.Passed {
				regressions = append(regressions, Regression{
					TestName: result.Name,
					Baseline: baseline,
					Current:  result,
				})
			}
		}
	}
	return regressions
}

// Regression represents a test failure compared to baseline
type Regression struct {
	TestName string
	Baseline TestResult
	Current  TestResult
}

// Helpers

func generateSessionID() string {
	return fmt.Sprintf("test-%d", time.Now().UnixNano())
}

func createTempWorkDir() string {
	dir, err := os.MkdirTemp("", "harness-e2e-*")
	if err != nil {
		panic(fmt.Sprintf("failed to create work dir: %v", err))
	}
	return dir
}

func cleanupWorkDir(dir string) {
	os.RemoveAll(dir)
}

// Assertions for behavior tests

type BehaviorAssertions struct {
	t   *testing.T
	ctx TestContext
}

func NewAssertions(t *testing.T, ctx TestContext) *BehaviorAssertions {
	return &BehaviorAssertions{t: t, ctx: ctx}
}

func (a *BehaviorAssertions) StateExists(key string) {
	if _, ok := a.ctx.State[key]; !ok {
		a.t.Errorf("expected state key %q to exist", key)
	}
}

func (a *BehaviorAssertions) StateEquals(key string, expected interface{}) {
	actual, ok := a.ctx.State[key]
	if !ok {
		a.t.Errorf("expected state key %q to exist", key)
		return
	}
	if actual != expected {
		a.t.Errorf("state[%q] = %v, want %v", key, actual, expected)
	}
}

func (a *BehaviorAssertions) FileExists(path string) {
	fullPath := filepath.Join(a.ctx.WorkDir, path)
	if _, err := os.Stat(fullPath); os.IsNotExist(err) {
		a.t.Errorf("expected file %q to exist", path)
	}
}

func (a *BehaviorAssertions) NoErrors() {
	if len(a.ctx.Errors) > 0 {
		a.t.Errorf("expected no errors, got: %v", a.ctx.Errors)
	}
}

func (a *BehaviorAssertions) ErrorCount(expected int) {
	if len(a.ctx.Errors) != expected {
		a.t.Errorf("expected %d errors, got %d", expected, len(a.ctx.Errors))
	}
}

// SessionManager simulates session lifecycle
type SessionManager struct {
	sessions map[string]*Session
}

type Session struct {
	ID       string
	State    map[string]interface{}
	History  []Action
	WorkDir  string
}

type Action struct {
	Type      string
	Timestamp time.Time
	Data      map[string]interface{}
}

func NewSessionManager() *SessionManager {
	return &SessionManager{
		sessions: make(map[string]*Session),
	}
}

func (sm *SessionManager) CreateSession() *Session {
	session := &Session{
		ID:      generateSessionID(),
		State:   make(map[string]interface{}),
		History: make([]Action, 0),
		WorkDir: createTempWorkDir(),
	}
	sm.sessions[session.ID] = session
	return session
}

func (sm *SessionManager) GetSession(id string) (*Session, bool) {
	session, ok := sm.sessions[id]
	return session, ok
}

func (sm *SessionManager) SimulateCrash(sessionID string) {
	// Simulate crash by removing from active sessions but keeping state
	if session, ok := sm.sessions[sessionID]; ok {
		// In real implementation, this would persist to disk
		session.State["crashed"] = true
		session.State["crash_time"] = time.Now()
	}
}

func (sm *SessionManager) ResumeSession(id string) (*Session, bool) {
	session, ok := sm.sessions[id]
	if !ok {
		return nil, false
	}
	// Verify state was preserved
	if crashed, _ := session.State["crashed"].(bool); crashed {
		session.State["resumed"] = true
		session.State["crash_time"] = nil
	}
	return session, true
}

func (s *Session) RecordAction(actionType string, data map[string]interface{}) {
	s.History = append(s.History, Action{
		Type:      actionType,
		Timestamp: time.Now(),
		Data:      data,
	})
}
