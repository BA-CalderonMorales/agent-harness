# Conversation Flow Improvements

## Overview

This document describes the improvements made to the agent-harness conversation flow, specifically addressing issues with greetings and simple conversational queries on Termux and other environments.

## Problem Statement

Previously, when users entered simple greetings like "Hello" or "Hi", the agent would:
1. Load all tools unnecessarily
2. Make an expensive LLM API call
3. Often fail to respond appropriately or timeout on mobile connections

This created a poor first-time user experience, especially on Termux where network conditions can be variable.

## Solution

### 1. Conversation Classification

Added `internal/agent/conversation.go` which classifies user input into categories:

- **ConvGreeting**: "Hello", "Hi", "Good morning", etc.
- **ConvQuestion**: "What can you do?", "Who are you?", etc.
- **ConvCasual**: "How are you?", "Thanks!", etc.
- **ConvTask**: Actual work requests that need tools

```go
convType := agent.ClassifyInput(input)
if convType == ConvGreeting {
    // Fast path: direct response without LLM call
}
```

### 2. Dual-Path Processing

The `processMessage` function in `main.go` now has two paths:

#### Conversational Path (Fast)
- Detects greetings and simple questions
- Returns pre-written responses immediately
- No LLM API call needed
- Works offline
- ~0ms response time

#### Task Path (Full Agent Loop)
- For actual work requests
- Uses the full agent loop with tools
- Enhanced system prompt guides tool usage
- Streaming responses with progress indicators

### 3. Enhanced System Prompt

Created `internal/agent/prompt.go` with clear behavioral guidance:

```
## Response Behavior

For GREETINGS and SIMPLE CONVERSATION:
- If the user says "Hello", "Hi", "Good morning", etc. → Just respond warmly, NO tools
- If the user asks "What can you do?" → Explain your capabilities briefly, NO tools  
- If the user says "Thanks" or "How are you?" → Respond naturally, NO tools

For CODING TASKS and WORK:
- Use tools freely to accomplish the user's goals
- Read files before editing them
- Show what you're doing with clear explanations
```

### 4. Termux Input Validation

Added `internal/ui/termux.go` with:

- **Input sanitization**: Removes problematic characters from mobile keyboards
- **Samsung keyboard handling**: Converts double-space period to single period
- **Line ending normalization**: Handles Windows/Unix/Mac line endings
- **Input validation**: Detects empty input, excessive repeats, null bytes

## Usage Examples

### Greetings (Fast Response)
```
◆ Hello

Hello! I'm Harness, your coding assistant. What would you like to work on?
```

### Questions About Capabilities (Fast Response)
```
◆ What can you do?

I'm Harness, a coding assistant that can help you with various development tasks.

Here's what I can do:
  • Read and analyze code files
  • Write and edit files in your workspace  
  • Run bash commands and scripts
  • Search through code with grep and glob
  ...
```

### Task Requests (Full Agent Loop)
```
◆ Create a file called hello.txt with "Hello World"

→ write: Creating hello.txt
   ┌( >_<)┘ running...
   ✓ write: Creating hello.txt

Done. Created hello.txt with the content "Hello World".
```

## Technical Details

### Conversation Detection Patterns

**Greetings (exact match or greeting + name):**
- "Hello", "Hi", "Hey", "Howdy", "Greetings"
- "Yo", "Hiya", "What's up", "Sup"
- "Good morning", "Good afternoon", "Good evening"
- Variations: "Hello there", "Hi Harness", "Hey buddy"

**Questions (standalone questions only):**
- "What can you do?"
- "Who are you?"
- "What are you?"
- "Help" (alone, not "Help me with...")

**Casual:**
- "How are you?", "How's it going?"
- "Thanks!", "Thank you", "Great job"
- "Tell me a joke"

**Task Indicators (triggers full agent loop):**
- Verbs: create, write, edit, fix, debug, build, run, test
- File references: @filename, ./path, file extensions
- Action words: search, find, look, analyze, explain

### Performance Impact

| Input Type | Before | After |
|------------|--------|-------|
| "Hello" | ~2-5s API call | ~0ms direct response |
| "What can you do?" | ~2-5s API call | ~0ms direct response |
| "Create a file" | ~2-5s API call | ~2-5s API call (unchanged) |

## Future Enhancements

1. **Context-Aware Greetings**: Remember user's name and preferences
2. **Localized Responses**: Support multiple languages for greetings
3. **Time-Based Greetings**: "Good morning" vs "Good evening" based on local time
4. **Session Context**: Reference previous work in greetings
5. **Custom Greetings**: Allow users to configure their own greeting responses

## Files Changed

- `internal/agent/conversation.go` - New: Conversation classification
- `internal/agent/conversation_test.go` - New: Tests for conversation detector
- `internal/agent/prompt.go` - New: Enhanced system prompt builder
- `internal/ui/termux.go` - New: Termux input validation
- `internal/ui/termux_test.go` - New: Tests for Termux validator
- `cmd/agent-harness/main.go` - Modified: Dual-path message processing
