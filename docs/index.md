# Agent Harness Docs

Start here -- [Architecture](architecture.md): what this is for, and how the
agent loop moves a turn. The README's [demo GIF](../README.md) shows the whole
surface in one take.

## Getting started

| Document | What |
|---|---|
| [Install](install.md) | Platforms, prerequisites, manual paths |
| [Usage](usage.md) | Tabs, controls, compose mode, slash commands |
| [Local models](local-models.md) | llama.cpp, GGUF download, overrides |
| [Environment variables](environment-variables.md) | Every override, one table |
| [Supported models](supported_models.md) | Provider/model matrix |

## How it works

| Document | What |
|---|---|
| [Architecture](architecture.md) | Why the harness is shaped this way |
| [Conversation flow](conversation_flow.md) | How a turn moves through the app |
| [Loop architecture](loop-architecture.md) | The agent loop, buckets, naming pattern |
| [Command approval](command-approval.md) | How commands get approved, permissions stacking |
| [TUI design patterns](tui-design-patterns.md) | The screen patterns the UI reuses |
| [Services and features](services-features.md) | The runtime services surface |

## Operating it

| Document | What |
|---|---|
| [Branch protection](branch-protection.md) | Branch rules and the release flow |
| [Changelog](changelog.md) | What changed between releases |
| [Edge cases](edgecases.md) | Non-obvious behaviors |
| [Parity](parity.md) | Feature parity edges |
| [Termux edge cases](termux_edge_cases.md) | Android-specific behavior |
| [DX improvements](dx-improvements.md) | Recent developer-experience work |

## Building it

| Document | What |
|---|---|
| [Demo](demo.md) | The recording, the mock server, making new demos |
| [Release notes](releases/v0.1.1.md) | The release ledger |
