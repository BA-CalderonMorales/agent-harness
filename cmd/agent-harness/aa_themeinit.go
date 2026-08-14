package main

// Import themeinit before any file that pulls in Bubble Tea so the
// terminal theme is pinned before tea's package init can query the
// terminal for its background color (see themeinit's doc comment).
// Go initializes a package's imports in lexical file order, so this
// file must sort before app.go — keep it first in the directory.
import (
	_ "github.com/BA-CalderonMorales/agent-harness/internal/interface/tui/themeinit"
)
