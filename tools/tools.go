package tools

import (
	"context"
	"encoding/json"
	"fmt"
)

// Tool defines the interface for all tools.
type Tool interface {
	Name() string
	Description() string
	InputSchema() json.RawMessage
	Execute(ctx context.Context, input json.RawMessage) (Result, error)
}

// Result represents the output of a tool execution.
type Result struct {
	Content string `json:"content"`
	IsError bool   `json:"is_error,omitempty"`
}

// Registry holds all available tools.
type Registry struct {
	tools map[string]Tool
}

// NewRegistry creates a registry with all built-in tools.
func NewRegistry() *Registry {
	r := &Registry{tools: make(map[string]Tool)}
	r.Register(&BashTool{})
	r.Register(&FileReadTool{})
	r.Register(&FileEditTool{})
	r.Register(&FileWriteTool{})
	r.Register(&GlobTool{})
	r.Register(&GrepTool{})
	r.Register(&WebFetchTool{})
	return r
}

// Register adds a tool to the registry.
func (r *Registry) Register(t Tool) {
	r.tools[t.Name()] = t
}

// Get returns a tool by name.
func (r *Registry) Get(name string) (Tool, bool) {
	t, ok := r.tools[name]
	return t, ok
}

// All returns all registered tools.
func (r *Registry) All() []Tool {
	result := make([]Tool, 0, len(r.tools))
	for _, t := range r.tools {
		result = append(result, t)
	}
	return result
}

// Execute runs a tool by name with the given input.
func (r *Registry) Execute(ctx context.Context, name string, input json.RawMessage) (Result, error) {
	t, ok := r.tools[name]
	if !ok {
		return Result{
			Content: fmt.Sprintf("Unknown tool: %s", name),
			IsError: true,
		}, nil
	}
	return t.Execute(ctx, input)
}
