# TUI Design Patterns Analysis

> Based on analysis of top-starred TUI projects from awesome-tuis

## Top Reference Projects

| Project | Stars | Language | Key Patterns |
|---------|-------|----------|--------------|
| fzf | 68k+ | Go | Fuzzy search, preview pane, multi-select |
| lazygit | 58k+ | Go | Side-by-side diff, contextual keybindings, vim nav |
| lazydocker | 42k+ | Go | Resource monitoring, tabbed interface, real-time updates |
| delta | 23k+ | Rust | Syntax highlighting, pager integration |
| btop | 22k+ | C++ | Beautiful graphs, color themes, smooth animations |
| yazi | 19k+ | Rust | Preview pane, async I/O, plugin system |
| k9s | 25k+ | Go | Resource browser, real-time updates, command palette |
| gh-dash | 6k+ | Go | Dashboard layout, table views, filtering |

## Universal TUI Patterns

### 1. Layout Architecture

```
┌─────────────────────────────────────────────────────────────┐
│ Header: Context info, breadcrumbs, status indicators        │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  Main Content Area                                          │
│  - Scrollable viewport                                      │
│  - Split panes for list+detail                              │
│  - Real-time updates with minimal redraw                    │
│                                                             │
├─────────────────────────────────────────────────────────────┤
│ Contextual Help: Keys change based on focused panel         │
└─────────────────────────────────────────────────────────────┘
```

### 2. Key Design Principles

**From lazygit/lazydocker:**
- Persistent contextual keybindings at bottom
- Vim-style navigation (j/k, gg/G, Ctrl+d/u)
- Single-key shortcuts for common actions
- Visual feedback for all operations

**From k9s:**
- Resource browser with drill-down
- Command palette (`:`) for quick actions
- Real-time status updates
- Color-coded status indicators

**From yazi:**
- Preview pane for content inspection
- Async operations (non-blocking UI)
- Extensible via configuration

**From gh-dash:**
- Dashboard with multiple sections
- Table views with sortable columns
- Filter/search integration

### 3. Color & Styling Patterns

```go
// Semantic colors (consistent meaning)
const (
    ColorSuccess = "#4ade80"  // Green: success, online, complete
    ColorError   = "#f87171"  // Red: error, offline, failed
    ColorWarning = "#fbbf24"  // Yellow: warning, pending
    ColorInfo    = "#60a5fa"  // Blue: info, active, selected
    ColorMuted   = "#6b7280"  // Gray: dim, disabled, help text
    ColorPrimary = "#a78bfa"  // Purple: brand, accent
)

// Text-only indicators (no emojis)
const (
    IndicatorSelected = "●"   // Selected item
    IndicatorActive   = "◆"   // Active/ongoing
    IndicatorComplete = "✓"   // Complete/success
    IndicatorError    = "✗"   // Error/failed
    IndicatorWarning  = "!"   // Warning
    IndicatorRunning  = "◐"   // Running/progress
)
```

### 4. Animation Patterns

```go
// Braille spinner (from btop/bubbles)
var BrailleFrames = []string{"⣾", "⣽", "⣻", "⢿", "⡿", "⣟", "⣯", "⣷"}

// Simple ASCII fallback
var SimpleFrames = []string{"|", "/", "-", "\\"}

// Kaomoji spinners (for personality)
var KaomojiFrames = []string{"┌( >_<)┘", "└( >_<)┐"}
```

### 5. Navigation Patterns

| Key | Action | Context |
|-----|--------|---------|
| `j/k` or `↓/↑` | Navigate items | Universal |
| `h/l` or `←/→` | Switch panes | Multi-pane views |
| `g/G` | Top/Bottom | Lists |
| `Ctrl+d/u` | Half-page scroll | Long content |
| `Tab/Shift+Tab` | Cycle focus | Forms, panes |
| `Esc` | Cancel/Back | Modals, forms |
| `?` | Show help | Non-input views |
| `q` | Quit | Normal mode |
| `/` | Search | Lists, content |

## Specific Improvements for agent-harness

### Current State
- Basic bubbletea model with simple input/output
- No persistent header/footer
- Limited keyboard navigation
- Simple message rendering

### Target Improvements

1. **Viewport Component**
   - Scrollable message history
   - Auto-scroll on new messages
   - Mouse support for scrolling

2. **Persistent Footer**
   - Contextual keybindings (change based on mode)
   - Status indicators (model, connection, tokens)
   - Mode indicator (insert/normal)

3. **Enhanced Message Rendering**
   - Markdown/glamour integration
   - Syntax highlighting for code blocks
   - Collapsible sections for long responses

4. **Command Palette**
   - Quick command access (`/` or `:`)
   - Fuzzy search through commands
   - History-aware suggestions

5. **Real-time Indicators**
   - Token usage counter
   - Cost tracking display
   - Connection status

## Specific Improvements for lumina-bot

### Current State
- Multi-tab interface with chat, agents, schedules, traffic, settings
- Design system with consistent styling
- Command palette and model picker
- Vim-style navigation modes

### Target Improvements

1. **Dashboard View**
   - Overview of all systems at a glance
   - Quick stats cards
   - Recent activity feed

2. **Enhanced Chat View**
   - Better message threading
   - File attachment preview
   - Tool call visualization

3. **Status Bar Improvements**
   - More compact layout
   - Additional metrics (response time, queue depth)
   - Connection health indicator

4. **Navigation Enhancements**
   - Breadcrumb navigation
   - Quick jump (Ctrl+P style)
   - Recent locations history

## Implementation Priority

### Phase 1: Foundation (agent-harness)
- [ ] Add viewport component for scrollable history
- [ ] Implement persistent footer with keybindings
- [ ] Add mode switching (insert/normal)

### Phase 2: Polish (agent-harness)
- [ ] Command palette
- [ ] Enhanced message rendering
- [ ] Real-time status bar

### Phase 3: lumina-bot Enhancements
- [ ] Dashboard overview
- [ ] Improved chat threading
- [ ] Enhanced status bar

### Phase 4: Shared Components
- [ ] Extract common patterns to shared package
- [ ] Unified theme system
- [ ] Animation library
