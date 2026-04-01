// claude-code is a Go port of Claude Code, an AI-powered coding assistant.
//
// Usage:
//
//	claude-code [flags] [prompt]
//
// Flags:
//
//	-p, --print          Print response without interactive mode
//	-m, --model          Model to use (default: claude-sonnet-4-20250514)
//	-r, --resume         Resume a previous session by ID
//	--sdk-url            WebSocket endpoint for SDK I/O
//	--version            Print version and exit
//	--list-sessions      List all sessions
//	-h, --help           Show help
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/btwiuse/claude-code/api"
	"github.com/btwiuse/claude-code/config"
	"github.com/btwiuse/claude-code/session"
	"github.com/btwiuse/claude-code/skills"
	"github.com/btwiuse/claude-code/tools"
	"github.com/btwiuse/claude-code/transport"
)

const version = "0.1.0"

func main() {
	// Define flags
	printMode := flag.Bool("p", false, "Print response and exit (non-interactive mode)")
	flag.BoolVar(printMode, "print", false, "Print response and exit (non-interactive mode)")

	model := flag.String("m", "", "Model to use (e.g., sonnet, opus, haiku, or full model ID)")
	flag.StringVar(model, "model", "", "Model to use (e.g., sonnet, opus, haiku, or full model ID)")

	resumeID := flag.String("r", "", "Resume a previous session by ID")
	flag.StringVar(resumeID, "resume", "", "Resume a previous session by ID")

	sdkURL := flag.String("sdk-url", "", "WebSocket endpoint for SDK I/O streaming")

	showVersion := flag.Bool("version", false, "Print version and exit")
	listSessions := flag.Bool("list-sessions", false, "List all available sessions")
	maxTokens := flag.Int("max-tokens", 8192, "Maximum tokens in response")
	systemPrompt := flag.String("system-prompt", "", "Override system prompt")

	flag.Parse()

	if *showVersion {
		fmt.Printf("claude-code %s\n", version)
		os.Exit(0)
	}

	if *listSessions {
		handleListSessions()
		return
	}

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to load config: %v\n", err)
		cfg = &config.Settings{}
	}

	// Resolve model
	resolvedModel := api.ResolveModel(*model)
	if resolvedModel == api.DefaultModel && cfg.Model != "" {
		resolvedModel = api.ResolveModel(cfg.Model)
	}

	// Get API key
	apiKey := api.APIKeyFromEnv()
	if apiKey == "" && cfg.APIKey != "" {
		apiKey = cfg.APIKey
	}

	// Create API client
	client := api.NewClient(apiKey, resolvedModel)

	// Create tool registry
	registry := tools.NewRegistry()

	// Load skills
	loadedSkills, err := skills.LoadAll()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to load skills: %v\n", err)
	}

	// Build system prompt
	sysPrompt := buildSystemPrompt(cfg, *systemPrompt, loadedSkills)

	// Set up context with signal handling
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		cancel()
	}()

	// Configure agent tool with recursive execution
	configureAgentTool(registry, client, sysPrompt, *maxTokens)

	// Handle --sdk-url mode
	if *sdkURL != "" {
		handleSDKMode(ctx, *sdkURL, client, registry, sysPrompt, *maxTokens)
		return
	}

	// Determine initial prompt
	prompt := strings.Join(flag.Args(), " ")

	// Handle session resumption or creation
	var sess *session.Session
	cwd, _ := os.Getwd()

	if *resumeID != "" {
		sess, err = session.Resume(*resumeID, cwd)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: failed to resume session: %v\n", err)
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "Resumed session %s (%d messages)\n", sess.ID, len(sess.Messages))
	} else {
		sess, err = session.New(cwd)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: failed to create session: %v\n", err)
			os.Exit(1)
		}
	}
	defer sess.Close()

	// Print mode: single prompt, print response, exit
	if *printMode {
		if prompt == "" {
			// Read from stdin
			data, err := io.ReadAll(os.Stdin)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error reading stdin: %v\n", err)
				os.Exit(1)
			}
			prompt = strings.TrimSpace(string(data))
		}
		if prompt == "" {
			fmt.Fprintf(os.Stderr, "Error: no prompt provided\n")
			os.Exit(1)
		}

		result, err := runConversationTurn(ctx, client, sess, registry, prompt, sysPrompt, *maxTokens, true)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		fmt.Println(result)
		return
	}

	// Interactive mode
	fmt.Fprintf(os.Stderr, "Claude Code v%s (model: %s)\n", version, resolvedModel)
	fmt.Fprintf(os.Stderr, "Session: %s\n", sess.ID)
	fmt.Fprintf(os.Stderr, "Type your message (Ctrl+D to exit)\n\n")

	// If initial prompt provided, run it first
	if prompt != "" {
		result, err := runConversationTurn(ctx, client, sess, registry, prompt, sysPrompt, *maxTokens, false)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		} else {
			fmt.Println(result)
		}
		fmt.Println()
	}

	// Interactive loop
	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Fprint(os.Stderr, "> ")
		if !scanner.Scan() {
			break
		}

		input := strings.TrimSpace(scanner.Text())
		if input == "" {
			continue
		}

		// Handle special commands
		if input == "/quit" || input == "/exit" {
			break
		}

		result, err := runConversationTurn(ctx, client, sess, registry, input, sysPrompt, *maxTokens, false)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			continue
		}

		fmt.Println(result)
		fmt.Println()
	}
}

// buildSystemPrompt constructs the system prompt from config, flags, and skills.
func buildSystemPrompt(cfg *config.Settings, override string, loadedSkills []skills.Skill) string {
	if override != "" {
		return override
	}

	var sb strings.Builder

	sb.WriteString("You are Claude Code, an AI assistant that helps with coding tasks. ")
	sb.WriteString("You have access to tools that let you read, write, and edit files, ")
	sb.WriteString("run shell commands, search code, and fetch web content. ")
	sb.WriteString("Use these tools to help the user with their coding tasks.\n\n")
	sb.WriteString("When making changes to code, prefer making targeted edits using the file_edit tool ")
	sb.WriteString("rather than rewriting entire files. Always read files before editing them.\n")

	if cfg.SystemPrompt != "" {
		sb.WriteString("\n")
		sb.WriteString(cfg.SystemPrompt)
		sb.WriteString("\n")
	}

	if cfg.CustomInstructions != "" {
		sb.WriteString("\n## Custom Instructions\n\n")
		sb.WriteString(cfg.CustomInstructions)
		sb.WriteString("\n")
	}

	if cfg.AppendSystemPrompt != "" {
		sb.WriteString("\n")
		sb.WriteString(cfg.AppendSystemPrompt)
		sb.WriteString("\n")
	}

	// Append skills
	sb.WriteString(skills.SkillsPrompt(loadedSkills))

	return sb.String()
}

// runConversationTurn executes a single conversation turn with tool use loop.
func runConversationTurn(ctx context.Context, client *api.Client, sess *session.Session, registry *tools.Registry, prompt string, systemPrompt string, maxTokens int, streaming bool) (string, error) {
	cwd, _ := os.Getwd()

	// Append user message to session
	if err := sess.AppendUserMessage(prompt, cwd); err != nil {
		return "", fmt.Errorf("failed to save user message: %w", err)
	}

	// Build messages from session history
	messages := buildAPIMessages(sess.GetMessages())

	// Tool use loop
	for {
		params := api.MessageParams{
			Messages:     messages,
			SystemPrompt: systemPrompt,
			Tools:        registry.All(),
			MaxTokens:    maxTokens,
		}

		var resp *api.Response
		var err error

		if streaming {
			// Stream the response for real-time output
			resp, err = client.StreamMessage(ctx, params, func(evt api.StreamEvent) {
				if evt.Type == "text_delta" {
					fmt.Fprint(os.Stdout, evt.Text)
				}
			})
		} else {
			resp, err = client.SendMessage(ctx, params)
		}

		if err != nil {
			return "", err
		}

		// Save assistant response to session
		contentJSON, _ := json.Marshal(resp.Content)
		if err := sess.AppendAssistantMessage(contentJSON, cwd); err != nil {
			return "", fmt.Errorf("failed to save assistant message: %w", err)
		}

		// Build assistant content blocks for the next API call
		var assistantBlocks []anthropic.ContentBlockParamUnion
		for _, b := range resp.Content {
			switch b.Type {
			case "text":
				assistantBlocks = append(assistantBlocks, api.TextBlock(b.Text))
			case "tool_use":
				var input interface{}
				if b.Input != nil {
					_ = json.Unmarshal(b.Input, &input)
				}
				assistantBlocks = append(assistantBlocks, api.ToolUseBlock(b.ID, b.Name, input))
			}
		}

		messages = append(messages, anthropic.NewAssistantMessage(assistantBlocks...))

		// Check if we need to process tool uses
		if !resp.HasToolUse() {
			return resp.TextContent(), nil
		}

		// Process tool uses
		toolResults := processToolUses(ctx, registry, resp.ToolUseBlocks())

		// Add tool results as user message
		messages = append(messages, anthropic.NewUserMessage(toolResults...))

		// Continue loop for next turn
	}
}

// processToolUses executes tool calls and returns result blocks.
func processToolUses(ctx context.Context, registry *tools.Registry, toolUses []api.ContentBlock) []anthropic.ContentBlockParamUnion {
	var results []anthropic.ContentBlockParamUnion

	for _, tu := range toolUses {
		tool, ok := registry.Get(tu.Name)
		if !ok {
			results = append(results, api.ToolResultBlock(tu.ID,
				fmt.Sprintf("Error: unknown tool '%s'", tu.Name)))
			continue
		}

		fmt.Fprintf(os.Stderr, "  → %s\n", tu.Name)

		result, err := tool.Run(ctx, tu.Input)
		if err != nil {
			results = append(results, api.ToolResultBlock(tu.ID,
				fmt.Sprintf("Error: %v", err)))
			continue
		}

		output := result.Output
		if result.IsError && result.Error != "" {
			output = "Error: " + result.Error
		}

		results = append(results, api.ToolResultBlock(tu.ID, output))
	}

	return results
}

// buildAPIMessages converts session messages to Anthropic API message params.
func buildAPIMessages(msgs []session.Message) []anthropic.MessageParam {
	var params []anthropic.MessageParam
	for _, m := range msgs {
		// Try to parse content as string first
		var textContent string
		if err := json.Unmarshal(m.Content, &textContent); err == nil {
			if m.Role == "user" {
				params = append(params, anthropic.NewUserMessage(api.TextBlock(textContent)))
			} else {
				params = append(params, anthropic.NewAssistantMessage(api.TextBlock(textContent)))
			}
			continue
		}

		// Try as content blocks array
		var blocks []api.ContentBlock
		if err := json.Unmarshal(m.Content, &blocks); err == nil {
			var cbBlocks []anthropic.ContentBlockParamUnion
			for _, b := range blocks {
				switch b.Type {
				case "text":
					cbBlocks = append(cbBlocks, api.TextBlock(b.Text))
				case "tool_use":
					var input interface{}
					if b.Input != nil {
						_ = json.Unmarshal(b.Input, &input)
					}
					cbBlocks = append(cbBlocks, api.ToolUseBlock(b.ID, b.Name, input))
				case "tool_result":
					cbBlocks = append(cbBlocks, api.ToolResultBlock(b.ID, b.Text))
				}
			}
			if len(cbBlocks) > 0 {
				if m.Role == "user" {
					params = append(params, anthropic.NewUserMessage(cbBlocks...))
				} else {
					params = append(params, anthropic.NewAssistantMessage(cbBlocks...))
				}
			}
			continue
		}

		// Fallback: use raw content as string
		if m.Role == "user" {
			params = append(params, anthropic.NewUserMessage(api.TextBlock(string(m.Content))))
		} else {
			params = append(params, anthropic.NewAssistantMessage(api.TextBlock(string(m.Content))))
		}
	}
	return params
}

// configureAgentTool sets up the agent tool with recursive execution capability.
func configureAgentTool(registry *tools.Registry, client *api.Client, systemPrompt string, maxTokens int) {
	agentTool, ok := registry.Get("agent")
	if !ok {
		return
	}

	at, ok := agentTool.(*tools.AgentTool)
	if !ok {
		return
	}

	at.RunAgent = func(ctx context.Context, prompt string, model string) (string, error) {
		resolvedModel := api.ResolveModel(model)
		subClient := api.NewClient(api.APIKeyFromEnv(), resolvedModel)

		// Build initial messages
		messages := []anthropic.MessageParam{
			anthropic.NewUserMessage(api.TextBlock(prompt)),
		}

		// Sub-agent tool loop
		for i := 0; i < 50; i++ { // Max 50 iterations to prevent infinite loops
			params := api.MessageParams{
				Messages:     messages,
				SystemPrompt: systemPrompt + "\n\nYou are a sub-agent. Complete the assigned task and provide a clear result.",
				Tools:        registry.All(),
				MaxTokens:    maxTokens,
			}

			resp, err := subClient.SendMessage(ctx, params)
			if err != nil {
				return "", err
			}

			// Build assistant blocks
			var assistantBlocks []anthropic.ContentBlockParamUnion
			for _, b := range resp.Content {
				switch b.Type {
				case "text":
					assistantBlocks = append(assistantBlocks, api.TextBlock(b.Text))
				case "tool_use":
					var input interface{}
					if b.Input != nil {
						_ = json.Unmarshal(b.Input, &input)
					}
					assistantBlocks = append(assistantBlocks, api.ToolUseBlock(b.ID, b.Name, input))
				}
			}
			messages = append(messages, anthropic.NewAssistantMessage(assistantBlocks...))

			if !resp.HasToolUse() {
				return resp.TextContent(), nil
			}

			// Process tool uses
			toolResults := processToolUses(ctx, registry, resp.ToolUseBlocks())
			messages = append(messages, anthropic.NewUserMessage(toolResults...))
		}

		return "Sub-agent reached maximum iteration limit", nil
	}
}

// handleListSessions prints all available sessions.
func handleListSessions() {
	infos, err := session.List("", 50)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if len(infos) == 0 {
		fmt.Println("No sessions found")
		return
	}

	fmt.Printf("%-36s  %-20s  %s\n", "SESSION ID", "LAST MODIFIED", "SUMMARY")
	fmt.Println(strings.Repeat("-", 80))

	for _, info := range infos {
		summary := info.Summary
		if summary == "" {
			summary = info.FirstPrompt
		}
		if summary == "" {
			summary = info.CustomTitle
		}
		if len(summary) > 50 {
			summary = summary[:47] + "..."
		}

		t := time.UnixMilli(info.LastModified)
		fmt.Printf("%-36s  %-20s  %s\n", info.SessionID, t.Format("2006-01-02 15:04:05"), summary)
	}
}

// handleSDKMode runs in SDK mode with a WebSocket transport.
func handleSDKMode(ctx context.Context, sdkURL string, client *api.Client, registry *tools.Registry, systemPrompt string, maxTokens int) {
	var ws *transport.Transport

	ws = transport.New(sdkURL,
		transport.WithOnMessage(func(msg transport.WSMessage) {
			handleSDKMessage(ctx, ws, client, registry, systemPrompt, maxTokens, msg)
		}),
		transport.WithOnError(func(err error) {
			fmt.Fprintf(os.Stderr, "WebSocket error: %v\n", err)
		}),
		transport.WithOnClose(func() {
			fmt.Fprintf(os.Stderr, "WebSocket connection closed\n")
		}),
	)

	if err := ws.Connect(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to connect to %s: %v\n", sdkURL, err)
		os.Exit(1)
	}
	defer ws.Close()

	fmt.Fprintf(os.Stderr, "Connected to SDK endpoint: %s\n", sdkURL)

	// Wait for context cancellation or connection close
	ws.Wait()
}

// handleSDKMessage processes an incoming SDK message and sends responses.
func handleSDKMessage(ctx context.Context, ws *transport.Transport, client *api.Client, registry *tools.Registry, systemPrompt string, maxTokens int, msg transport.WSMessage) {
	// Parse the incoming message as a prompt
	var prompt string
	if err := json.Unmarshal(msg.Data, &prompt); err != nil {
		// Try as object with content field
		var obj struct {
			Content string `json:"content"`
			Prompt  string `json:"prompt"`
		}
		if err := json.Unmarshal(msg.Data, &obj); err == nil {
			prompt = obj.Content
			if prompt == "" {
				prompt = obj.Prompt
			}
		}
	}

	if prompt == "" {
		return
	}

	// Create a temporary session for this interaction
	cwd, _ := os.Getwd()
	sess, err := session.New(cwd)
	if err != nil {
		return
	}
	defer sess.Close()

	// Run the conversation
	result, err := runConversationTurn(ctx, client, sess, registry, prompt, systemPrompt, maxTokens, false)
	if err != nil {
		errData, _ := json.Marshal(map[string]string{"error": err.Error()})
		_ = ws.Send(ctx, transport.WSMessage{
			Type: "error",
			Data: errData,
		})
		return
	}

	responseData, _ := json.Marshal(map[string]string{"content": result})
	_ = ws.Send(ctx, transport.WSMessage{
		Type: "response",
		Data: responseData,
	})
}
