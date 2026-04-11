package loop

import (
	"strconv"

	"github.com/BA-CalderonMorales/agent-harness/internal/tools"
	"github.com/BA-CalderonMorales/agent-harness/pkg/types"
)

// LoopTool provides tool management and routing capabilities.
// It abstracts the tool registry and provides bucket-aware routing.
type LoopTool struct {
	registry   *tools.ToolRegistry
	bucketMap  map[string]LoopBase // tool name -> bucket
	validators map[string]Validator
}

// Validator is a function that validates tool input.
type Validator func(input map[string]any) ValidationResult

// ValidationResult is the output of input validation.
type ValidationResult struct {
	Valid   bool
	Message string
	Code    string
}

// NewLoopTool creates a tool manager.
func NewLoopTool() *LoopTool {
	return &LoopTool{
		registry:   tools.NewRegistry(),
		bucketMap:  make(map[string]LoopBase),
		validators: make(map[string]Validator),
	}
}

// Register adds a tool to the registry and optionally maps it to a bucket.
func (lt *LoopTool) Register(tool tools.Tool, bucket LoopBase) {
	lt.registry.RegisterBuiltIn(tool)
	if bucket != nil {
		lt.bucketMap[tool.Name] = bucket
		for _, alias := range tool.Aliases {
			lt.bucketMap[alias] = bucket
		}
	}
}

// RegisterValidator adds a custom validator for a tool.
func (lt *LoopTool) RegisterValidator(toolName string, validator Validator) {
	lt.validators[toolName] = validator
}

// FindTool looks up a tool by name or alias.
func (lt *LoopTool) FindTool(name string) (tools.Tool, bool) {
	return lt.registry.FindToolByName(name)
}

// FindBucketForTool returns the bucket that handles this tool.
func (lt *LoopTool) FindBucketForTool(toolName string) (LoopBase, bool) {
	bucket, ok := lt.bucketMap[toolName]
	return bucket, ok
}

// GetAllTools returns all registered tools.
func (lt *LoopTool) GetAllTools() []tools.Tool {
	return lt.registry.AllTools()
}

// GetEnabledTools returns only enabled tools.
func (lt *LoopTool) GetEnabledTools() []tools.Tool {
	return lt.registry.FilterEnabled()
}

// ValidateInput runs validation for a tool.
func (lt *LoopTool) ValidateInput(toolName string, input map[string]any) ValidationResult {
	// Check custom validator first
	if validator, ok := lt.validators[toolName]; ok {
		return validator(input)
	}

	// Use tool's built-in validation
	tool, ok := lt.FindTool(toolName)
	if !ok {
		return ValidationResult{Valid: false, Message: "unknown tool: " + toolName, Code: "unknown_tool"}
	}

	if tool.ValidateInput != nil {
		result := tool.ValidateInput(input, tools.Context{})
		return ValidationResult{
			Valid:   result.Valid,
			Message: result.Message,
			Code:    strconv.Itoa(result.ErrorCode),
		}
	}

	return ValidationResult{Valid: true}
}

// BuildToolSchemas returns JSON schemas for all enabled tools.
func (lt *LoopTool) BuildToolSchemas() []map[string]any {
	toolsList := lt.GetEnabledTools()
	schemas := make([]map[string]any, 0, len(toolsList))

	for _, t := range toolsList {
		schema := t.InputSchema()
		if schema == nil && t.InputJSONSchema != nil {
			schema = t.InputJSONSchema
		}
		schemas = append(schemas, map[string]any{
			"type": "function",
			"function": map[string]any{
				"name":        t.Name,
				"description": t.Description,
				"parameters":  schema,
			},
		})
	}

	return schemas
}

// ToolUse represents a parsed tool call from the LLM.
type ToolUse struct {
	ID    string
	Name  string
	Input map[string]any
}

// ParseToolUses extracts tool calls from LLM response content.
func (lt *LoopTool) ParseToolUses(content []types.ContentBlock) []ToolUse {
	var uses []ToolUse
	for _, block := range content {
		if tu, ok := block.(types.ToolUseBlock); ok {
			uses = append(uses, ToolUse{
				ID:    tu.ID,
				Name:  tu.Name,
				Input: tu.Input,
			})
		}
	}
	return uses
}

// ToolCategory groups related tools.
type ToolCategory struct {
	Name        string
	Description string
	ToolNames   []string
}

// CategorizeTools organizes tools by their buckets.
func (lt *LoopTool) CategorizeTools() []ToolCategory {
	categories := make(map[string]*ToolCategory)

	for _, tool := range lt.GetAllTools() {
		bucket, ok := lt.FindBucketForTool(tool.Name)
		var category string
		if ok {
			category = bucket.Capabilities().Category
		} else {
			category = "uncategorized"
		}

		if _, exists := categories[category]; !exists {
			categories[category] = &ToolCategory{
				Name:      category,
				ToolNames: make([]string, 0),
			}
		}
		categories[category].ToolNames = append(categories[category].ToolNames, tool.Name)
	}

	result := make([]ToolCategory, 0, len(categories))
	for _, cat := range categories {
		result = append(result, *cat)
	}
	return result
}
