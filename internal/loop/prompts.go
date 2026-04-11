package loop

import (
	"fmt"
	"strings"
	"sync"
)

// LoopSystemPrompts manages system prompts and context injection.
// It handles prompt composition, caching, and dynamic updates.
type LoopSystemPrompts struct {
	mu            sync.RWMutex
	basePrompt    string
	contextBlocks map[string]string // named context blocks
	appendBlocks  []string          // blocks to append
	cache         string            // cached composed prompt
	cacheValid    bool
}

// NewLoopSystemPrompts creates a prompt manager.
func NewLoopSystemPrompts() LoopSystemPrompts {
	return LoopSystemPrompts{
		contextBlocks: make(map[string]string),
		appendBlocks:  make([]string, 0),
		cacheValid:    false,
	}
}

// SetBase sets the foundational system prompt.
func (p *LoopSystemPrompts) SetBase(prompt string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.basePrompt = prompt
	p.cacheValid = false
}

// GetBase returns the base prompt.
func (p *LoopSystemPrompts) GetBase() string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.basePrompt
}

// AddContextBlock adds a named context block.
// These are injected as XML-like tags in the composed prompt.
func (p *LoopSystemPrompts) AddContextBlock(name, content string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.contextBlocks[name] = content
	p.cacheValid = false
}

// RemoveContextBlock removes a named block.
func (p *LoopSystemPrompts) RemoveContextBlock(name string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.contextBlocks, name)
	p.cacheValid = false
}

// Append adds content to the end of the prompt.
func (p *LoopSystemPrompts) Append(content string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.appendBlocks = append(p.appendBlocks, content)
	p.cacheValid = false
}

// Compose builds the final system prompt.
func (p *LoopSystemPrompts) Compose() string {
	p.mu.RLock()
	if p.cacheValid {
		cached := p.cache
		p.mu.RUnlock()
		return cached
	}
	p.mu.RUnlock()

	p.mu.Lock()
	defer p.mu.Unlock()

	var parts []string
	if p.basePrompt != "" {
		parts = append(parts, p.basePrompt)
	}

	// Add context blocks in deterministic order
	for name, content := range p.contextBlocks {
		if content != "" {
			parts = append(parts, fmt.Sprintf("<%s>\n%s\n</%s>", name, content, name))
		}
	}

	// Add appended content
	for _, block := range p.appendBlocks {
		if block != "" {
			parts = append(parts, block)
		}
	}

	p.cache = strings.Join(parts, "\n\n")
	p.cacheValid = true
	return p.cache
}

// InvalidateCache forces recomposition on next call.
func (p *LoopSystemPrompts) InvalidateCache() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.cacheValid = false
}

// Clear removes all content.
func (p *LoopSystemPrompts) Clear() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.basePrompt = ""
	p.contextBlocks = make(map[string]string)
	p.appendBlocks = make([]string, 0)
	p.cache = ""
	p.cacheValid = false
}

// PromptTemplate is a reusable prompt with placeholders.
type PromptTemplate struct {
	Name     string
	Template string
	Defaults map[string]string
}

// Execute fills the template with values.
func (t PromptTemplate) Execute(values map[string]string) string {
	result := t.Template
	for key, val := range t.Defaults {
		placeholder := fmt.Sprintf("{{%s}}", key)
		result = strings.ReplaceAll(result, placeholder, val)
	}
	for key, val := range values {
		placeholder := fmt.Sprintf("{{%s}}", key)
		result = strings.ReplaceAll(result, placeholder, val)
	}
	return result
}

// Common prompt templates
var (
	// ToolErrorPrompt is shown when tools fail
	ToolErrorPrompt = PromptTemplate{
		Name: "tool_error",
		Template: `The tool {{tool_name}} failed with error: {{error}}

Please either:
1. Try a different approach
2. Ask the user for clarification
3. Report the error and stop`,
		Defaults: map[string]string{},
	}

	// ContextCompactedPrompt is shown after context compaction
	ContextCompactedPrompt = PromptTemplate{
		Name:     "context_compacted",
		Template: `[Context was compacted to stay within limits. Previous conversation summary: {{summary}}]`,
		Defaults: map[string]string{
			"summary": "Earlier conversation details were summarized to save space.",
		},
	}

	// MaxTurnsReachedPrompt is shown when loop hits max turns
	MaxTurnsReachedPrompt = PromptTemplate{
		Name: "max_turns",
		Template: `Maximum number of turns ({{max_turns}}) reached without completion.

Current state: {{state}}

Please provide a final response summarizing what was accomplished.`,
		Defaults: map[string]string{},
	}
)
