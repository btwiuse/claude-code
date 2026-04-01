// Package tools defines the tool interface and provides implementations
// of all built-in tools for Claude Code.
package tools

import (
	"context"
	"encoding/json"
)

// Tool defines the interface that all tools must implement.
type Tool interface {
	// Name returns the unique tool name.
	Name() string

	// Description returns a human-readable description for the model.
	Description() string

	// InputSchema returns the JSON Schema for the tool's input parameters.
	InputSchema() json.RawMessage

	// Run executes the tool with the given input and returns the result.
	Run(ctx context.Context, input json.RawMessage) (*Result, error)

	// IsReadOnly returns true if the tool doesn't modify state.
	IsReadOnly() bool
}

// Result holds the output of a tool execution.
type Result struct {
	Output   string `json:"output,omitempty"`
	Error    string `json:"error,omitempty"`
	ExitCode int    `json:"exitCode,omitempty"`
	IsError  bool   `json:"isError,omitempty"`
}

// Registry holds all available tools.
type Registry struct {
	tools map[string]Tool
	order []string
}

// NewRegistry creates a new tool registry with all built-in tools.
func NewRegistry() *Registry {
	r := &Registry{
		tools: make(map[string]Tool),
	}

	// Register all built-in tools
	builtins := []Tool{
		&BashTool{},
		&FileReadTool{},
		&FileWriteTool{},
		&FileEditTool{},
		&GlobTool{},
		&GrepTool{},
		&WebFetchTool{},
		&AgentTool{},
	}

	for _, t := range builtins {
		r.Register(t)
	}

	return r
}

// Register adds a tool to the registry.
func (r *Registry) Register(t Tool) {
	name := t.Name()
	r.tools[name] = t
	r.order = append(r.order, name)
}

// Get returns a tool by name.
func (r *Registry) Get(name string) (Tool, bool) {
	t, ok := r.tools[name]
	return t, ok
}

// All returns all registered tools in registration order.
func (r *Registry) All() []Tool {
	result := make([]Tool, 0, len(r.order))
	for _, name := range r.order {
		result = append(result, r.tools[name])
	}
	return result
}

// Names returns the names of all registered tools.
func (r *Registry) Names() []string {
	return append([]string{}, r.order...)
}
