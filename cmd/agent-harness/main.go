// Package main provides the entry point for agent-harness.
//
// The application is organized into layers:
//   - main: entry point only
//   - app: App struct and lifecycle
//   - init: initialization (config, session, tools, commands)
//   - handlers: user input processing
//   - delegates: TUI integration
//   - helpers: utility functions
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/BA-CalderonMorales/agent-harness/internal/ui"
)

var (
	Version   = "0.1.12"
	BuildTime = "unknown"
	GitSHA    = "unknown"
	GitTag    = "unknown"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "\n%s %v\n", ui.RenderError(err.Error()), "")
		os.Exit(1)
	}
}

func run() error {
	var showVersion, showHelp bool
	flag.BoolVar(&showVersion, "version", false, "Show version")
	flag.BoolVar(&showVersion, "v", false, "Show version (shorthand)")
	flag.BoolVar(&showHelp, "help", false, "Show help")
	flag.BoolVar(&showHelp, "h", false, "Show help (shorthand)")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [options]\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Agent Harness - AI-powered coding assistant\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		fmt.Fprintf(os.Stderr, "  -v, --version    Show version\n")
		fmt.Fprintf(os.Stderr, "  -h, --help       Show help\n")
		fmt.Fprintf(os.Stderr, "\nFor more: https://github.com/BA-CalderonMorales/agent-harness\n")
	}

	flag.Parse()

	if showVersion {
		printVersion()
		return nil
	}

	if showHelp {
		flag.Usage()
		return nil
	}

	app, err := newApp()
	if err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigCh
		fmt.Println("\nSaving session before exit...")
		if app.session != nil {
			_ = app.session.SaveToFile(app.getSessionsDir() + "/" + app.session.ID + ".json")
		}
		cancel()
		os.Exit(0)
	}()

	_ = ctx
	return app.run()
}

func printVersion() {
	buildType := "release"
	if stringsContains(Version, "dev") || stringsContains(Version, "local") {
		buildType = "dev"
	}
	fmt.Printf("agent-harness %s\n", Version)
	fmt.Printf("  Build type: %s\n", buildType)
	if GitTag != "unknown" && GitTag != "" {
		fmt.Printf("  Tag: %s\n", GitTag)
	}
	if BuildTime != "unknown" && BuildTime != "" {
		fmt.Printf("  Built: %s\n", BuildTime)
	}
	if GitSHA != "unknown" && GitSHA != "" {
		fmt.Printf("  Git: %s\n", GitSHA)
	}
}

func stringsContains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 || containsSubstring(s, substr))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
