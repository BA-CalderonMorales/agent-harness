# Changelog

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
