# Sub-agents

A sub-agent is a delegated worker with its own instructions, its own tool
set, and optionally its own model. The agent tool's `agent_type` picks
one by name.

Until now `agent_type` was accepted and then ignored: it changed the
label on the transcript row and nothing else, so every delegation was the
same generic worker. A definition file makes the name mean something.

## Where definitions live

| Root | Purpose |
|---|---|
| `<workspace>/.agent-harness/agents/` | The project's sub-agents |
| `<config home>/agents/` | Your own sub-agents, shared across projects |

Later roots win, so a project overrides a user-level definition of the
same name.

## The file format

A definition is a Markdown file: YAML frontmatter, then the instructions.

```markdown
---
name: reviewer
description: Reviews a finished change for correctness.
model: inherit
tools: [read, grep, glob]
---
You are a code reviewer. Work through the diff once, cite the file and
line for every finding, and say plainly when you found nothing.

Do not edit files. Report only.
```

| Field | Meaning |
|---|---|
| `name` | The name the model passes as `agent_type`. Defaults to the file name. |
| `description` | One line, shown to the model and in `/agents`. |
| `model` | The model the sub-agent should use. `inherit` (or omitted) keeps the parent's model. |
| `tools` | Allowlist of tool names. Omit it to inherit the parent's tools. |

The body is the sub-agent's instructions: it replaces the generic worker
prompt, and the harness context (workspace, permissions, provider) still
applies.

## Using them

The model sees the installed definitions in its system prompt and
delegates by name:

```
agent(agent_type: "reviewer", prompt: "Review the auth changes on this branch")
```

`/agents` lists what is installed, next to the personas:

```
Sub-agent definitions:
  reviewer       Reviews a finished change for correctness.
                 tools: read, grep, glob
```

## Rules

- A delegated worker never gets the `agent` tool, so sub-agents cannot
  spawn more sub-agents.
- The parent's permission callback still applies: a sub-agent's tool
  calls are decided by the same rules as the parent's.
- An unknown `agent_type` is an error that lists the available names, so
  the model can correct itself instead of silently getting the generic
  worker.
- A definition that lists tools narrows the child's toolset. Leave the
  list out unless the role genuinely should not write.

## Examples worth copying

| Name | Tools | What it is for |
|---|---|---|
| `reviewer` | `read, grep, glob` | Reads a change and reports findings with evidence |
| `tester` | `read, grep, bash` | Runs the tests and reports what failed and why |
| `explorer` | `read, grep, glob, ls_recursive` | Maps an unfamiliar codebase and hands back a summary |

The repo's own `.agent-harness/skills/release-workflow/SKILL.md` is the
same idea for procedures: a file the agent reads and follows. Skills
describe how; sub-agents describe who.
