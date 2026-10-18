package mcp

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/BA-CalderonMorales/agent-harness/internal/testharness/mcppeer"
)

func TestMCPPeerProcess(t *testing.T) { mcppeer.RunIfRequested() }

func peerConfig(t *testing.T, mode string) McpServerConfig {
	t.Helper()
	command, args, err := mcppeer.Command()
	if err != nil {
		t.Fatal(err)
	}
	return McpServerConfig{Command: command, Args: args, Env: map[string]string{"AH_MCP_TEST_PEER": mode}}
}

func TestManagerFailureCleansUpProcessAndAllowsRetry(t *testing.T) {
	for _, mode := range []string{"reject-initialize", "hang-initialize", "duplicate-catalog", "cursor-loop"} {
		t.Run(mode, func(t *testing.T) {
			m := NewManager()
			t.Cleanup(func() { _ = m.Close() })
			cfg := peerConfig(t, mode)
			exit := filepath.Join(t.TempDir(), "exited")
			cfg.Env["AH_MCP_TEST_EXIT"] = exit
			ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
			defer cancel()
			if err := m.Connect(ctx, "fixture", cfg); err == nil {
				t.Fatal("broken startup/catalog was accepted")
			}
			if len(m.ListConnections()) != 0 || len(m.AllToolDefs()) != 0 {
				t.Fatal("failed server leaked a connection or partial tool catalog")
			}
			if _, err := os.Stat(exit); err != nil {
				t.Fatal("failed connection left its server process alive")
			}
			if err := m.Connect(context.Background(), "fixture", peerConfig(t, "normal")); err != nil {
				t.Fatalf("failed name could not be retried: %v", err)
			}
		})
	}
}

func TestManagerCloseCancelsPendingSetupWithoutHoldingRegistryLock(t *testing.T) {
	m := NewManager()
	t.Cleanup(func() { _ = m.Close() })
	done := make(chan error, 1)
	go func() { done <- m.Connect(context.Background(), "hanging", peerConfig(t, "hang-initialize")) }()
	deadline := time.Now().Add(time.Second)
	for {
		if _, ok := m.GetConnection("hanging"); ok {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("pending connection never became observable")
		}
		time.Sleep(time.Millisecond)
	}
	closed := make(chan error, 1)
	go func() { closed <- m.Close() }()
	select {
	case err := <-closed:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("shutdown deadlocked behind a pending initialization")
	}
	if err := <-done; err == nil {
		t.Fatal("cancelled initialization succeeded")
	}
}

func TestManagerAttemptsEveryServerAndKeepsHealthyTools(t *testing.T) {
	m := NewManager()
	t.Cleanup(func() { _ = m.Close() })
	err := m.LoadAndConnect(context.Background(), map[string]McpServerConfig{
		"broken":  {Command: filepath.Join(t.TempDir(), "missing-program")},
		"healthy": peerConfig(t, "normal"),
	})
	if err == nil || !strings.Contains(err.Error(), "broken") {
		t.Fatalf("failed extension was not identified: %v", err)
	}
	defs := m.AllToolDefs()
	if len(defs) != 5 || defs[0].ServerName != "healthy" {
		t.Fatalf("healthy catalog disappeared after another server failed: %+v", defs)
	}
	if _, err := m.CallTool(context.Background(), "healthy", "echo", nil); err != nil {
		t.Fatal(err)
	}
}

func TestManagerCloseReapsProcessesAndIsConcurrentSafe(t *testing.T) {
	m := NewManager()
	cfg := peerConfig(t, "normal")
	exit := filepath.Join(t.TempDir(), "exited")
	cfg.Env["AH_MCP_TEST_EXIT"] = exit
	if err := m.Connect(context.Background(), "fixture", cfg); err != nil {
		t.Fatal(err)
	}
	conn, _ := m.GetConnection("fixture")
	conn.Status = "modified snapshot"
	conn.Tools[0].Name = "modified tool snapshot"
	live, _ := m.GetConnection("fixture")
	if live.Status != "connected" || live.Tools[0].Name != "inspect" {
		t.Fatal("caller changed manager-owned connection state")
	}
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := m.Close(); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	if _, err := os.Stat(exit); err != nil {
		t.Fatal("Close returned before the server process exited")
	}
	if len(m.ListConnections()) != 0 {
		t.Fatal("closed manager retained connections")
	}
	if err := m.Connect(context.Background(), "fixture", cfg); err == nil {
		t.Fatal("closed manager started a new process")
	}
}

func TestManagerStartsAndInitializesSubprocess(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("MCP connection panicked before starting its server: %v", r)
		}
	}()
	manager := NewManager()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := manager.Connect(ctx, "fixture", peerConfig(t, "normal")); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = manager.Disconnect("fixture") })
	conn, ok := manager.GetConnection("fixture")
	if !ok || conn.Status != "connected" || len(conn.Tools) != 5 {
		t.Fatalf("incomplete initialization or tool pagination: %+v", conn)
	}
}
