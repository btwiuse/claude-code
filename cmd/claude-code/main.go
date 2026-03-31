// claude-code is an AI-powered coding assistant CLI built with Go.
//
// Usage:
//
//	claude-code [flags] [prompt]
//
// Flags:
//
//	-m, --model     Model to use (default: claude-sonnet-4-20250514)
//	-k, --api-key   Anthropic API key (or set ANTHROPIC_API_KEY)
//	-p, --print     Print response and exit (non-interactive mode)
//	-s, --session   Resume a specific session by ID
//	-v, --verbose   Enable verbose output
//	--version       Print version and exit
//	--system-prompt Custom system prompt
//	--max-turns     Maximum conversation turns per query (default: 25)
//	--cwd           Set working directory
package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/anthropics/claude-code-go/internal/cli"
	"github.com/anthropics/claude-code-go/internal/constants"
)

func main() {
	cfg := cli.RunConfig{}
	args := os.Args[1:]

	// Fast-path: version check
	for _, arg := range args {
		if arg == "--version" || arg == "-V" {
			fmt.Printf("Claude Code (Go) v%s\n", constants.Version)
			os.Exit(0)
		}
	}

	// Parse flags
	var positionalArgs []string
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-m", "--model":
			if i+1 < len(args) {
				i++
				cfg.Model = args[i]
			}
		case "-k", "--api-key":
			if i+1 < len(args) {
				i++
				cfg.APIKey = args[i]
			}
		case "-p", "--print":
			// Next args are the prompt
			if i+1 < len(args) {
				cfg.Prompt = strings.Join(args[i+1:], " ")
				i = len(args)
			}
		case "-s", "--session":
			if i+1 < len(args) {
				i++
				cfg.SessionID = args[i]
			}
		case "-v", "--verbose":
			cfg.Verbose = true
		case "--system-prompt":
			if i+1 < len(args) {
				i++
				cfg.SystemPrompt = args[i]
			}
		case "--max-turns":
			if i+1 < len(args) {
				i++
				fmt.Sscanf(args[i], "%d", &cfg.MaxTurns)
			}
		case "--cwd":
			if i+1 < len(args) {
				i++
				cfg.CWD = args[i]
			}
		case "--help", "-h":
			printUsage()
			os.Exit(0)
		default:
			if strings.HasPrefix(args[i], "-") {
				fmt.Fprintf(os.Stderr, "Unknown flag: %s\n", args[i])
				printUsage()
				os.Exit(1)
			}
			positionalArgs = append(positionalArgs, args[i])
		}
	}

	// If positional args provided without -p, treat as non-interactive prompt
	if cfg.Prompt == "" && len(positionalArgs) > 0 {
		cfg.Prompt = strings.Join(positionalArgs, " ")
	}

	if err := cli.Run(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Printf(`Claude Code (Go) v%s - AI-powered coding assistant

Usage:
  claude-code [flags] [prompt]

Flags:
  -m, --model <model>       Model to use (default: %s)
  -k, --api-key <key>       Anthropic API key (or set ANTHROPIC_API_KEY env var)
  -p, --print <prompt>      Print response and exit (non-interactive mode)
  -s, --session <id>        Resume a specific session by ID
  -v, --verbose             Enable verbose output
  --system-prompt <prompt>  Custom system prompt
  --max-turns <n>           Max conversation turns per query (default: 25)
  --cwd <dir>               Set working directory
  -V, --version             Print version and exit
  -h, --help                Show this help message

Examples:
  claude-code                             Start interactive mode
  claude-code "explain this codebase"     Single prompt mode
  claude-code -p "fix the tests"          Print mode (non-interactive)
  claude-code -m claude-opus-4-20250514      Use a specific model

Environment Variables:
  ANTHROPIC_API_KEY         API key for Anthropic
  ANTHROPIC_BASE_URL        Custom API base URL
  ANTHROPIC_MODEL           Default model override
  ANTHROPIC_CUSTOM_HEADERS  Additional HTTP headers (newline-separated)
  CLAUDE_CONFIG_DIR         Custom config directory path
`, constants.Version, constants.DefaultModel)
}
