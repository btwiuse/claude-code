package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/btwiuse/claude-code/api"
	"github.com/btwiuse/claude-code/config"
	"github.com/btwiuse/claude-code/session"
	"github.com/btwiuse/claude-code/tools"
	"github.com/btwiuse/claude-code/transport"
	"github.com/spf13/cobra"
)

const version = "0.1.0"

func main() {
	rootCmd := buildRootCmd()
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func buildRootCmd() *cobra.Command {
	var (
		flagModel       string
		flagResume      string
		flagContinue    bool
		flagPrint       string
		flagSdkURL      string
		flagSystemPrompt string
		flagOutputFormat string
		flagMaxTurns    int
		flagDebug       bool
	)

	rootCmd := &cobra.Command{
		Use:     "claude-code",
		Short:   "Claude Code - AI-powered coding assistant",
		Version: version,
		Long:    "Claude Code is an AI-powered coding assistant that can read, write, and edit files, run commands, and search code.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runMain(cmd.Context(), runConfig{
				model:        flagModel,
				resume:       flagResume,
				cont:         flagContinue,
				print:        flagPrint,
				sdkURL:       flagSdkURL,
				systemPrompt: flagSystemPrompt,
				outputFormat: flagOutputFormat,
				maxTurns:     flagMaxTurns,
				debug:        flagDebug,
				prompt:       strings.Join(args, " "),
			})
		},
	}

	rootCmd.Flags().StringVarP(&flagModel, "model", "m", "", "Model to use (default: claude-sonnet-4-20250514)")
	rootCmd.Flags().StringVar(&flagResume, "resume", "", "Resume a session by ID")
	rootCmd.Flags().BoolVarP(&flagContinue, "continue", "c", false, "Continue the most recent session")
	rootCmd.Flags().StringVarP(&flagPrint, "print", "p", "", "Run in non-interactive (headless) mode with the given prompt")
	rootCmd.Flags().StringVar(&flagSdkURL, "sdk-url", "", "WebSocket endpoint for SDK I/O streaming")
	rootCmd.Flags().StringVar(&flagSystemPrompt, "system-prompt", "", "Custom system prompt")
	rootCmd.Flags().StringVar(&flagOutputFormat, "output-format", "text", "Output format: text, json, stream-json")
	rootCmd.Flags().IntVar(&flagMaxTurns, "max-turns", 0, "Maximum number of agentic turns (0 = unlimited)")
	rootCmd.Flags().BoolVarP(&flagDebug, "debug", "d", false, "Enable debug output")

	rootCmd.AddCommand(buildSessionsCmd())

	return rootCmd
}

func buildSessionsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "sessions",
		Short: "List saved sessions",
		RunE: func(cmd *cobra.Command, args []string) error {
			sessions, err := session.List()
			if err != nil {
				return fmt.Errorf("listing sessions: %w", err)
			}

			if len(sessions) == 0 {
				fmt.Println("No sessions found.")
				return nil
			}

			for _, s := range sessions {
				fmt.Printf("%-36s  %s  %s\n",
					s.ID,
					s.UpdatedAt.Format("2006-01-02 15:04"),
					s.Title,
				)
			}
			return nil
		},
	}
}

type runConfig struct {
	model        string
	resume       string
	cont         bool
	print        string
	sdkURL       string
	systemPrompt string
	outputFormat string
	maxTurns     int
	debug        bool
	prompt       string
}

func runMain(ctx context.Context, cfg runConfig) error {
	ctx, cancel := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer cancel()

	// Load settings.
	settings, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not load settings: %v\n", err)
		settings = &config.Settings{}
	}

	// Resolve API key.
	apiKey := settings.Resolve()
	if apiKey == "" {
		return fmt.Errorf("no API key found. Set ANTHROPIC_API_KEY environment variable or add apiKey to ~/.claude/settings.json")
	}

	// Resolve model.
	model := settings.EffectiveModel(cfg.model)

	// Create tool registry.
	registry := tools.NewRegistry()

	// Create API client.
	client := api.NewClient(apiKey, model)

	// Handle SDK URL mode.
	if cfg.sdkURL != "" {
		return runSDKMode(ctx, client, registry, cfg)
	}

	// Handle session management.
	var sess *session.Session
	var history []anthropic.MessageParam

	if cfg.resume != "" {
		sess, history, err = resumeSession(cfg.resume)
		if err != nil {
			return fmt.Errorf("resuming session: %w", err)
		}
		fmt.Fprintf(os.Stderr, "Resumed session: %s\n", cfg.resume)
	} else if cfg.cont {
		sess, history, err = continueLatestSession()
		if err != nil {
			return fmt.Errorf("continuing session: %w", err)
		}
		if sess != nil {
			fmt.Fprintf(os.Stderr, "Continuing session: %s\n", sess.ID)
		}
	}

	if sess == nil {
		sess, err = session.New()
		if err != nil {
			return fmt.Errorf("creating session: %w", err)
		}
	}
	defer sess.Close()

	fmt.Fprintf(os.Stderr, "Session: %s\n", sess.ID)
	if cfg.debug {
		fmt.Fprintf(os.Stderr, "Model: %s\n", model)
	}

	// Headless mode with --print.
	if cfg.print != "" {
		return runHeadless(ctx, client, registry, sess, history, cfg)
	}

	// Interactive REPL mode.
	return runInteractive(ctx, client, registry, sess, history, cfg)
}

func resumeSession(id string) (*session.Session, []anthropic.MessageParam, error) {
	sess, entries, err := session.Resume(id)
	if err != nil {
		return nil, nil, err
	}

	pairs := session.ExtractMessages(entries)
	history := make([]anthropic.MessageParam, 0, len(pairs))
	for _, p := range pairs {
		switch p.Role {
		case "user":
			history = append(history, api.BuildUserMessage(p.Content))
		case "assistant":
			history = append(history, api.BuildAssistantMessage(p.Content))
		}
	}

	return sess, history, nil
}

func continueLatestSession() (*session.Session, []anthropic.MessageParam, error) {
	sessions, err := session.List()
	if err != nil || len(sessions) == 0 {
		return nil, nil, nil
	}

	latest := sessions[0]
	return resumeSession(latest.ID)
}

func runHeadless(ctx context.Context, client *api.Client, registry *tools.Registry, sess *session.Session, history []anthropic.MessageParam, cfg runConfig) error {
	prompt := cfg.print
	history = append(history, api.BuildUserMessage(prompt))
	_ = sess.AppendUser(prompt)

	return runConversationLoop(ctx, client, registry, sess, history, cfg, nil)
}

func runInteractive(ctx context.Context, client *api.Client, registry *tools.Registry, sess *session.Session, history []anthropic.MessageParam, cfg runConfig) error {
	reader := bufio.NewReader(os.Stdin)

	// If prompt was given as positional arg, use it as first message.
	if cfg.prompt != "" {
		fmt.Printf("\x1b[1;34mYou:\x1b[0m %s\n\n", cfg.prompt)
		history = append(history, api.BuildUserMessage(cfg.prompt))
		_ = sess.AppendUser(cfg.prompt)

		if err := runConversationLoop(ctx, client, registry, sess, history, cfg, nil); err != nil {
			return err
		}
	}

	for {
		fmt.Print("\x1b[1;34mYou:\x1b[0m ")
		input, err := reader.ReadString('\n')
		if err != nil {
			return nil // EOF
		}
		input = strings.TrimSpace(input)
		if input == "" {
			continue
		}

		// Handle special commands.
		if input == "/quit" || input == "/exit" {
			return nil
		}
		if input == "/sessions" {
			sessions, err := session.List()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error listing sessions: %v\n", err)
				continue
			}
			for _, s := range sessions {
				fmt.Printf("%-36s  %s  %s\n", s.ID, s.UpdatedAt.Format("2006-01-02 15:04"), s.Title)
			}
			continue
		}

		history = append(history, api.BuildUserMessage(input))
		_ = sess.AppendUser(input)

		if err := runConversationLoop(ctx, client, registry, sess, history, cfg, nil); err != nil {
			fmt.Fprintf(os.Stderr, "\x1b[31mError: %v\x1b[0m\n", err)
		}
		fmt.Println()
	}
}

func runConversationLoop(ctx context.Context, client *api.Client, registry *tools.Registry, sess *session.Session, history []anthropic.MessageParam, cfg runConfig, onChunk func(string)) error {
	toolDefs := api.ToolDefs(registry)
	systemPrompt := buildSystemPrompt(cfg.systemPrompt)

	turns := 0
	for {
		if cfg.maxTurns > 0 && turns >= cfg.maxTurns {
			fmt.Fprintf(os.Stderr, "\nMax turns (%d) reached.\n", cfg.maxTurns)
			return nil
		}

		if cfg.print == "" {
			fmt.Print("\x1b[1;32mClaude:\x1b[0m ")
		}

		textCallback := func(text string) {
			if cfg.outputFormat == "json" || cfg.outputFormat == "stream-json" {
				return
			}
			fmt.Print(text)
		}
		if onChunk != nil {
			textCallback = onChunk
		}

		resp, err := client.SendMessage(ctx, history, toolDefs, systemPrompt, textCallback)
		if err != nil {
			return err
		}

		if cfg.print == "" && cfg.outputFormat == "text" {
			fmt.Println()
		}

		// Persist assistant response.
		if resp.Content != "" {
			assistantMsg, _ := json.Marshal(map[string]interface{}{
				"role":    "assistant",
				"content": resp.Content,
			})
			_ = sess.AppendAssistant(assistantMsg)
		}

		// Handle JSON output format.
		if cfg.outputFormat == "json" || cfg.outputFormat == "stream-json" {
			output := map[string]interface{}{
				"type":    "assistant",
				"content": resp.Content,
				"usage": map[string]interface{}{
					"input_tokens":  resp.Usage.InputTokens,
					"output_tokens": resp.Usage.OutputTokens,
				},
			}
			if len(resp.ToolCalls) > 0 {
				output["tool_calls"] = resp.ToolCalls
			}
			data, _ := json.Marshal(output)
			fmt.Println(string(data))
		}

		// If no tool calls, we're done with this turn.
		if len(resp.ToolCalls) == 0 {
			return nil
		}

		// Process tool calls.
		// Add assistant message with tool use to history.
		history = append(history, api.BuildAssistantToolUseMessage(resp.ToolCalls))

		for _, tc := range resp.ToolCalls {
			if cfg.debug {
				fmt.Fprintf(os.Stderr, "\n\x1b[33m[Tool: %s]\x1b[0m\n", tc.Name)
			}

			result, err := registry.Execute(ctx, tc.Name, tc.Input)
			if err != nil {
				result = tools.Result{Content: fmt.Sprintf("tool error: %v", err), IsError: true}
			}

			if cfg.debug {
				preview := result.Content
				if len(preview) > 200 {
					preview = preview[:200] + "..."
				}
				fmt.Fprintf(os.Stderr, "\x1b[90m%s\x1b[0m\n", preview)
			}

			history = append(history, api.BuildToolResultMessage(tc.ID, result.Content, result.IsError))

			// Persist tool result.
			toolResultMsg, _ := json.Marshal(map[string]interface{}{
				"type":      "tool_result",
				"tool_id":   tc.ID,
				"tool_name": tc.Name,
				"content":   result.Content,
				"is_error":  result.IsError,
			})
			_ = sess.AppendAssistant(toolResultMsg)
		}

		turns++
	}
}

func buildSystemPrompt(custom string) string {
	cwd, _ := os.Getwd()
	base := fmt.Sprintf(`You are Claude Code, an AI coding assistant. You help users with software development tasks.

Current working directory: %s

You have access to tools that let you:
- Run shell commands (Bash)
- Read files (Read)
- Edit files by string replacement (Edit)
- Write files (Write)
- Find files by glob pattern (Glob)
- Search code by regex (Grep)
- Fetch web content (WebFetch)

Always use absolute paths when working with files. Prefer reading files before editing them.
When making changes, explain what you're doing. Use tools to verify your changes work.`, cwd)

	if custom != "" {
		base += "\n\nAdditional instructions:\n" + custom
	}
	return base
}

func runSDKMode(ctx context.Context, client *api.Client, registry *tools.Registry, cfg runConfig) error {
	ws := transport.New(cfg.sdkURL)
	if err := ws.Connect(ctx); err != nil {
		return fmt.Errorf("connecting to SDK URL: %w", err)
	}
	defer ws.Close()

	var history []anthropic.MessageParam

	sess, _ := session.New()
	if sess != nil {
		defer sess.Close()
	}

	for {
		msg, err := ws.Receive(ctx)
		if err != nil {
			return nil // Connection closed
		}

		switch msg.Type {
		case "user_message":
			var payload struct {
				Content string `json:"content"`
			}
			if err := json.Unmarshal(msg.Payload, &payload); err != nil {
				continue
			}

			history = append(history, api.BuildUserMessage(payload.Content))

			// Run conversation loop, sending responses back over WebSocket.
			onChunk := func(text string) {
				chunkMsg := transport.Message{
					Type: "text_delta",
				}
				chunkPayload, _ := json.Marshal(map[string]string{"text": text})
				chunkMsg.Payload = chunkPayload
				_ = ws.Send(ctx, chunkMsg)
			}

			if err := runConversationLoop(ctx, client, registry, sess, history, cfg, onChunk); err != nil {
				errMsg := transport.Message{Type: "error"}
				errPayload, _ := json.Marshal(map[string]string{"error": err.Error()})
				errMsg.Payload = errPayload
				_ = ws.Send(ctx, errMsg)
			}

			doneMsg := transport.Message{Type: "message_done"}
			_ = ws.Send(ctx, doneMsg)
		}
	}
}
