// Package cli implements the command-line interface for Claude Code.
package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/anthropics/claude-code-go/internal/api"
	"github.com/anthropics/claude-code-go/internal/config"
	"github.com/anthropics/claude-code-go/internal/constants"
	"github.com/anthropics/claude-code-go/internal/cost"
	"github.com/anthropics/claude-code-go/internal/query"
	"github.com/anthropics/claude-code-go/internal/session"
	"github.com/anthropics/claude-code-go/internal/tools"
	"github.com/anthropics/claude-code-go/internal/ui"
)

// App is the main Claude Code application.
type App struct {
	client      *api.Client
	registry    *tools.Registry
	costTracker *cost.Tracker
	session     *session.Session
	engine      *query.Engine
	cwd         string
	model       string
	verbose     bool
}

// RunConfig contains configuration for running the CLI.
type RunConfig struct {
	Model       string
	APIKey      string
	CWD         string
	SessionID   string
	Verbose     bool
	Prompt      string // Non-interactive mode: single prompt
	SystemPrompt string
	MaxTurns    int
}

// Run starts the CLI application.
func Run(cfg RunConfig) error {
	// Determine working directory
	cwd := cfg.CWD
	if cwd == "" {
		var err error
		cwd, err = os.Getwd()
		if err != nil {
			return fmt.Errorf("getting working directory: %w", err)
		}
	}

	// Create API client
	clientOpts := []api.ClientOption{
		api.WithSessionID(cfg.SessionID),
	}
	if cfg.Model != "" {
		clientOpts = append(clientOpts, api.WithModel(cfg.Model))
	}
	if cfg.APIKey != "" {
		clientOpts = append(clientOpts, api.WithAPIKey(cfg.APIKey))
	}

	client, err := api.NewClient(clientOpts...)
	if err != nil {
		return fmt.Errorf("creating API client: %w", err)
	}

	// Create tool registry
	registry := tools.DefaultRegistry

	// Create cost tracker
	costTracker := cost.NewTracker()

	// Create or load session
	sessionID := cfg.SessionID
	if sessionID == "" {
		sessionID = generateSessionID()
	}
	sess := session.NewSession(sessionID, cwd, client.GetModel())

	// Build system prompt
	systemPrompt := buildSystemPrompt(cwd, cfg.SystemPrompt)

	// Create tool context
	toolCtx := &tools.ToolContext{
		CWD:            cwd,
		AbortCtx:       context.Background(),
		ReadFileState:  tools.NewFileStateCache(),
		Debug:          cfg.Verbose,
		SessionID:      sessionID,
	}

	app := &App{
		client:      client,
		registry:    registry,
		costTracker: costTracker,
		session:     sess,
		cwd:         cwd,
		model:       client.GetModel(),
		verbose:     cfg.Verbose,
	}

	// Create query engine
	engine := query.NewEngine(query.EngineConfig{
		Client:       client,
		Registry:     registry,
		CostTracker:  costTracker,
		ToolCtx:      toolCtx,
		SystemPrompt: systemPrompt,
		MaxTurns:     cfg.MaxTurns,
		OnText:       ui.PrintAssistantText,
		OnToolUse: func(name string, input json.RawMessage) {
			ui.PrintToolUse(name)
		},
		OnToolResult: func(name string, result *tools.ToolResult) {
			ui.PrintToolResult(name, result.Content, result.IsError)
		},
		OnThinking: ui.PrintThinking,
		OnError: func(err error) {
			ui.PrintError(err.Error())
		},
	})
	app.engine = engine

	// Handle non-interactive mode
	if cfg.Prompt != "" {
		return app.runNonInteractive(cfg.Prompt)
	}

	// Interactive mode
	return app.runInteractive()
}

func (app *App) runNonInteractive(prompt string) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Set up signal handling
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		cancel()
	}()

	err := app.engine.Submit(ctx, prompt)
	fmt.Println() // Ensure final newline
	return err
}

func (app *App) runInteractive() error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Set up signal handling
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		fmt.Println()
		ui.PrintCostSummary(app.costTracker.GetSummary())
		cancel()
		os.Exit(0)
	}()

	// Display header
	ui.PrintHeader(constants.Version, app.model)
	ui.PrintWelcome(app.cwd)

	// Main REPL loop
	for {
		input, err := ui.ReadInput("claude> ")
		if err != nil {
			// EOF (Ctrl+D)
			fmt.Println()
			break
		}

		input = strings.TrimSpace(input)
		if input == "" {
			continue
		}

		// Handle commands
		if strings.HasPrefix(input, "/") {
			shouldContinue := app.handleCommand(input)
			if !shouldContinue {
				break
			}
			continue
		}

		// Submit to Claude
		err = app.engine.Submit(ctx, input)
		if err != nil {
			if ctx.Err() != nil {
				break
			}
			ui.PrintError(err.Error())
		}
		fmt.Println() // Newline after response

		// Save session periodically
		app.session.Messages = app.engine.GetMessages()
		if err := app.session.Save(); err != nil && app.verbose {
			ui.PrintError(fmt.Sprintf("Failed to save session: %v", err))
		}
	}

	// Print final cost summary
	ui.PrintCostSummary(app.costTracker.GetSummary())

	return nil
}

// handleCommand processes slash commands. Returns false if the app should exit.
func (app *App) handleCommand(input string) bool {
	parts := strings.Fields(input)
	cmd := strings.ToLower(parts[0])

	switch cmd {
	case "/quit", "/exit", "/q":
		return false

	case "/help", "/h":
		printHelp()

	case "/clear":
		app.engine.SetMessages(nil)
		fmt.Println("Conversation cleared.")

	case "/cost":
		ui.PrintCostSummary(app.costTracker.GetSummary())

	case "/model":
		fmt.Printf("Current model: %s\n", app.model)

	case "/version":
		fmt.Printf("Claude Code (Go) v%s\n", constants.Version)

	case "/session":
		fmt.Printf("Session ID: %s\n", app.session.ID)
		fmt.Printf("Messages: %d\n", len(app.engine.GetMessages()))

	case "/config":
		fmt.Printf("Config directory: %s\n", config.ConfigDir())
		fmt.Printf("API key configured: %v\n", config.GetAPIKey() != "")

	case "/compact":
		msgs := app.engine.GetMessages()
		if len(msgs) > 4 {
			// Keep system context + last 4 messages
			app.engine.SetMessages(msgs[len(msgs)-4:])
			fmt.Printf("Compacted conversation: kept last %d messages.\n", 4)
		} else {
			fmt.Println("Conversation is already compact.")
		}

	case "/doctor":
		runDoctor()

	default:
		fmt.Printf("Unknown command: %s. Type /help for available commands.\n", cmd)
	}

	return true
}

func printHelp() {
	fmt.Println()
	fmt.Printf("%s%sAvailable Commands:%s\n", ui.Bold, ui.Cyan, ui.Reset)
	fmt.Println()
	commands := []struct{ cmd, desc string }{
		{"/help, /h", "Show this help message"},
		{"/quit, /exit, /q", "Exit Claude Code"},
		{"/clear", "Clear conversation history"},
		{"/compact", "Compact conversation to save context"},
		{"/cost", "Show session cost summary"},
		{"/model", "Show current model"},
		{"/version", "Show version"},
		{"/session", "Show session info"},
		{"/config", "Show configuration info"},
		{"/doctor", "Run diagnostics"},
	}
	for _, c := range commands {
		fmt.Printf("  %s%-20s%s %s\n", ui.Bold, c.cmd, ui.Reset, c.desc)
	}
	fmt.Println()
}

func runDoctor() {
	fmt.Printf("\n%s%sClaude Code Doctor%s\n\n", ui.Bold, ui.Cyan, ui.Reset)

	// Check API key
	apiKey := config.GetAPIKey()
	if apiKey != "" {
		fmt.Printf("  %s✓%s API key configured\n", ui.Green, ui.Reset)
	} else {
		fmt.Printf("  %s✗%s API key not configured\n", ui.Red, ui.Reset)
	}

	// Check config directory
	configDir := config.ConfigDir()
	if _, err := os.Stat(configDir); err == nil {
		fmt.Printf("  %s✓%s Config directory exists: %s\n", ui.Green, ui.Reset, configDir)
	} else {
		fmt.Printf("  %s✗%s Config directory missing: %s\n", ui.Red, ui.Reset, configDir)
	}

	// Check git
	if _, err := os.Stat(".git"); err == nil {
		fmt.Printf("  %s✓%s Git repository detected\n", ui.Green, ui.Reset)
	} else {
		fmt.Printf("  %s·%s Not in a git repository\n", ui.Yellow, ui.Reset)
	}

	// Check ripgrep
	if _, err := exec.LookPath("rg"); err == nil {
		fmt.Printf("  %s✓%s ripgrep (rg) available\n", ui.Green, ui.Reset)
	} else {
		fmt.Printf("  %s·%s ripgrep (rg) not found (will fall back to grep)\n", ui.Yellow, ui.Reset)
	}

	fmt.Println()
}

func buildSystemPrompt(cwd string, custom string) string {
	if custom != "" {
		return custom
	}

	return fmt.Sprintf(`You are Claude Code, an AI assistant by Anthropic, running as a CLI tool.

You are an expert software engineer with deep knowledge of programming languages, frameworks, design patterns, and best practices.

Your current working directory is: %s

You have access to tools for reading/writing files, executing bash commands, searching code, and more.

Guidelines:
- Be concise and direct in your responses
- When editing files, make minimal targeted changes
- Always verify your changes work correctly
- Use the available tools to explore the codebase before making changes
- Explain your reasoning briefly before making changes
- If you're unsure about something, ask for clarification`, cwd)
}

func generateSessionID() string {
	return fmt.Sprintf("session_%d", time.Now().UnixNano())
}
