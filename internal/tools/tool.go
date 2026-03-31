// Package tools defines the tool interface and registry for Claude Code.
package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/anthropics/claude-code-go/internal/types"
)

// ToolResult represents the result of a tool execution.
type ToolResult struct {
	// Content is the result content to return to the model.
	Content string `json:"content"`

	// IsError indicates the tool call failed.
	IsError bool `json:"is_error,omitempty"`

	// Metadata contains optional structured metadata.
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// ToolContext provides contextual information to tools during execution.
type ToolContext struct {
	// CWD is the current working directory.
	CWD string

	// AbortCtx can be used to cancel long-running operations.
	AbortCtx context.Context

	// ReadFileState tracks files that have been read in this session.
	ReadFileState *FileStateCache

	// PermissionMode controls how permissions are checked.
	PermissionMode types.PermissionMode

	// Debug enables verbose logging.
	Debug bool

	// SessionID is the current session identifier.
	SessionID string
}

// FileStateEntry tracks the state of a previously read file.
type FileStateEntry struct {
	Content   string
	Timestamp int64
	Size      int64
}

// FileStateCache tracks files read during a session.
type FileStateCache struct {
	mu      sync.RWMutex
	entries map[string]*FileStateEntry
}

// NewFileStateCache creates a new file state cache.
func NewFileStateCache() *FileStateCache {
	return &FileStateCache{
		entries: make(map[string]*FileStateEntry),
	}
}

// Get retrieves a cached file state entry.
func (c *FileStateCache) Get(path string) (*FileStateEntry, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	entry, ok := c.entries[path]
	return entry, ok
}

// Set stores a file state entry.
func (c *FileStateCache) Set(path string, entry *FileStateEntry) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries[path] = entry
}

// Tool is the interface that all tools must implement.
type Tool interface {
	// Name returns the tool's unique name.
	Name() string

	// Description returns a description of what the tool does.
	Description() string

	// InputSchema returns the JSON Schema for the tool's input.
	InputSchema() types.ToolInputSchema

	// IsReadOnly returns true if the tool only reads data.
	IsReadOnly() bool

	// IsEnabled returns true if the tool is currently available.
	IsEnabled() bool

	// Execute runs the tool with the given input.
	Execute(ctx context.Context, input json.RawMessage, toolCtx *ToolContext) (*ToolResult, error)

	// UserFacingName returns a human-readable name for display.
	UserFacingName(input json.RawMessage) string
}

// Registry manages available tools.
type Registry struct {
	mu    sync.RWMutex
	tools map[string]Tool
	order []string
}

// NewRegistry creates a new tool registry.
func NewRegistry() *Registry {
	return &Registry{
		tools: make(map[string]Tool),
	}
}

// Register adds a tool to the registry.
func (r *Registry) Register(tool Tool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	name := tool.Name()
	if _, exists := r.tools[name]; !exists {
		r.order = append(r.order, name)
	}
	r.tools[name] = tool
}

// Get retrieves a tool by name.
func (r *Registry) Get(name string) (Tool, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	tool, ok := r.tools[name]
	return tool, ok
}

// All returns all registered tools in registration order.
func (r *Registry) All() []Tool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]Tool, 0, len(r.order))
	for _, name := range r.order {
		if tool, ok := r.tools[name]; ok && tool.IsEnabled() {
			result = append(result, tool)
		}
	}
	return result
}

// ToDefinitions converts all enabled tools to API tool definitions.
func (r *Registry) ToDefinitions() []types.ToolDefinition {
	tools := r.All()
	defs := make([]types.ToolDefinition, 0, len(tools))
	for _, tool := range tools {
		defs = append(defs, types.ToolDefinition{
			Name:        tool.Name(),
			Description: tool.Description(),
			InputSchema: tool.InputSchema(),
		})
	}
	return defs
}

// Execute runs a tool by name with the given input.
func (r *Registry) Execute(ctx context.Context, name string, input json.RawMessage, toolCtx *ToolContext) (*ToolResult, error) {
	tool, ok := r.Get(name)
	if !ok {
		return &ToolResult{
			Content: fmt.Sprintf("Unknown tool: %s", name),
			IsError: true,
		}, nil
	}

	if !tool.IsEnabled() {
		return &ToolResult{
			Content: fmt.Sprintf("Tool %s is not currently enabled", name),
			IsError: true,
		}, nil
	}

	return tool.Execute(ctx, input, toolCtx)
}

// DefaultRegistry is the global tool registry with all built-in tools registered.
var DefaultRegistry *Registry

func init() {
	DefaultRegistry = NewRegistry()
	RegisterBuiltinTools(DefaultRegistry)
}

// RegisterBuiltinTools registers all built-in tools with the given registry.
func RegisterBuiltinTools(r *Registry) {
	r.Register(NewBashTool())
	r.Register(NewFileReadTool())
	r.Register(NewFileWriteTool())
	r.Register(NewFileEditTool())
	r.Register(NewGlobTool())
	r.Register(NewGrepTool())
	r.Register(NewListDirTool())
	r.Register(NewWebFetchTool())
	r.Register(NewAgentTool())
}
