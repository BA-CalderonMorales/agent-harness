# Changelog

## [0.1.4] - 2026-04-13

### Fixed
- Slash command help deduplication: removed duplicate /exit and /memory entries
- Race condition in command palette execution (removed goroutine mutation of TUI state)
- Deterministic /help output by using ordered category slices
- Missing user message logging for slash commands in chat history
- Missing /workspace command in help and command palette

## [0.1.3] - 2026-04-13

### Fixed
- Respect AGENT_HARNESS_PROVIDER and AGENT_HARNESS_MODEL environment variables
- Prevent secure credentials from overriding explicit env-based provider/model selection

## [0.0.54] - 2026-04-06

### Changed
- Version bump to v0.0.54

## [0.0.48] - 2026-04-04

### Fixed
- Credential decryption error handling with user-friendly recovery options
- Numeric model input (1, 2, 3) now maps to actual models instead of literal "1"
- ESC key now properly cancels running agent execution
- Model display validation to catch invalid numeric-only model names

### Changed
- Tool calling UX: now shows grey command preview like Kimi does
- Animated tool display in yolo mode: spinner + tool name + command preview
- Single-line tool animation instead of endless [bash] bash repetitions
- Better password input handling with whitespace trimming

### Security
- Added validation for corrupted credential files (salt/nonce/ciphertext)
- Clear master key on decryption failure to force fresh password prompt

## [0.0.47] - 2026-04-04

### Added
- Command approval system with two execution modes:
  - Interactive mode: prompts for approval before executing shell/write/edit commands
  - Yolo mode: auto-approves commands but shows what is happening in the UI
- Approval dialog with four options: Approve, Approve All, Reject, Reject + Suggest
- Pattern memory: remembers "Approve All" and "Reject All" choices per session
- ESC key integration to cancel agent execution at any time
- Command visibility: always see what commands are about to run

### Changed
- Tool display name changed from "bash" to "Shell" for clarity
- Removed emojis from error messages (replaced with text indicators)
- Slimmed down README with clearer documentation structure
- Added awesome-tuis credit to acknowledgments

### Documentation
- New docs/command-approval.md explaining the approval system
- Updated debugging-patterns.md with approval system patterns

## [0.0.46] - 2026-04-04

### Fixed
- TUI status bar now updates correctly when selecting model via picker
- Model selection via /model command or Settings view reflects immediately in status bar

### Added
- Release publish script for streamlined release workflow

All notable changes to agent-harness will be documented in this file.

## [0.0.45] - 2026-04-04

### Changed
- Status bar now shows actionable hints instead of "default" when no model is set
- Status bar shows [⚠ no model] warning when model is not configured
- Improved model display: shows shortened model name or "(use /model)" hint

### Added
- Visual feedback with actionable hints when models fail to respond
- Error messages now include specific guidance for common failure patterns:
  - Timeout errors: suggests switching models with /model command
  - Connection errors: suggests checking /config and Settings
  - Rate limit errors: suggests trying different models
  - Authentication errors: suggests updating API key in Settings
  - Model not found: suggests using /model to list available models
- Follows visual-ux skill patterns: uses ⚠, ?, → indicators consistently

## [0.0.43] - 2026-04-04

### Added
- Release workflow skill to prevent version mismatches
- check-version.sh script for version validation
- bump-version.sh script for semver calculations
- release.sh script for one-command releases

### Fixed
- Version alignment: bumped to 0.0.43 to match release process

## [0.0.41] - 2026-04-04

### Added
- TUI design patterns documentation based on awesome-tuis research
- Analyzed top TUI projects (lazygit, k9s, lazydocker, yazi, gh-dash)
- Documented universal patterns: viewport components, status bars, vim navigation

### Changed
- Enhanced TUI architecture with patterns from top-starred TUI projects
- Improved visual design consistency with semantic color system

## [0.0.40] - 2026-04-03

### Added
- Tab-based navigation (Chat, Sessions, Settings)
- Command palette for quick command access
- Model picker for interactive model selection
- Vim-style navigation modes (insert/normal)
- Real-time streaming response display
- Markdown rendering for assistant responses
- Response time tracking and display
- Status bar with contextual keybindings
- Activity indicators for tabs with unseen updates

### Changed
- Migrated to bubbletea-based TUI architecture
- Improved terminal handling for Termux environment

## [0.0.39] - 2026-04-01

### Added
- Initial TUI mode with basic viewport
- Session management UI
- Settings configuration view

### Fixed
- Terminal input handling on mobile devices
## [0.0.49] - 2026-04-05

### Fixed
- Tool UI now uses per-tool UserFacingName and GetActivityDescription methods
- Rich activity descriptions show what tools are doing (e.g., 'Reading file.go (lines 10-20)')
- Anti-pattern fixed: removed hardcoded switch statements for tool display names
