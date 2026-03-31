// Package ui provides terminal UI components for Claude Code.
package ui

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"time"
)

// Colors for terminal output.
const (
	Reset     = "\033[0m"
	Bold      = "\033[1m"
	Dim       = "\033[2m"
	Italic    = "\033[3m"
	Underline = "\033[4m"

	Red     = "\033[31m"
	Green   = "\033[32m"
	Yellow  = "\033[33m"
	Blue    = "\033[34m"
	Magenta = "\033[35m"
	Cyan    = "\033[36m"
	White   = "\033[37m"
	Gray    = "\033[90m"

	BgBlue    = "\033[44m"
	BgMagenta = "\033[45m"
)

// Spinner provides an animated loading indicator.
type Spinner struct {
	message string
	frames  []string
	done    chan struct{}
	active  bool
}

// NewSpinner creates a new spinner with the given message.
func NewSpinner(message string) *Spinner {
	return &Spinner{
		message: message,
		frames:  []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"},
		done:    make(chan struct{}),
	}
}

// Start begins the spinner animation.
func (s *Spinner) Start() {
	if s.active {
		return
	}
	s.active = true
	go func() {
		i := 0
		for {
			select {
			case <-s.done:
				fmt.Print("\r\033[K") // Clear line
				return
			default:
				frame := s.frames[i%len(s.frames)]
				fmt.Printf("\r%s%s %s%s", Cyan, frame, s.message, Reset)
				i++
				time.Sleep(80 * time.Millisecond)
			}
		}
	}()
}

// Stop ends the spinner animation.
func (s *Spinner) Stop() {
	if !s.active {
		return
	}
	s.active = false
	close(s.done)
}

// UpdateMessage changes the spinner message.
func (s *Spinner) UpdateMessage(msg string) {
	s.message = msg
}

// PrintHeader displays the application header.
func PrintHeader(version, model string) {
	fmt.Printf("\n%s%s╭─────────────────────────────────────────╮%s\n", Bold, Blue, Reset)
	fmt.Printf("%s%s│%s %s%sClaude Code%s (Go) v%s %s%s│%s\n", Bold, Blue, Reset, Bold, White, Reset, version, Bold, Blue, Reset)
	fmt.Printf("%s%s│%s Model: %-33s%s%s│%s\n", Bold, Blue, Reset, model, Bold, Blue, Reset)
	fmt.Printf("%s%s╰─────────────────────────────────────────╯%s\n\n", Bold, Blue, Reset)
}

// PrintWelcome displays the welcome message.
func PrintWelcome(cwd string) {
	fmt.Printf("%sTip:%s Use %s/help%s to see available commands, %s/quit%s to exit.\n", Dim, Reset, Bold, Reset, Bold, Reset)
	fmt.Printf("%sCWD:%s %s\n\n", Dim, Reset, cwd)
}

// PrintAssistantText displays assistant text output.
func PrintAssistantText(text string) {
	fmt.Print(text)
}

// PrintToolUse displays a tool usage notification.
func PrintToolUse(name string) {
	fmt.Printf("\n%s%s  %s%s\n", Yellow, Bold, name, Reset)
}

// PrintToolResult displays a tool execution result.
func PrintToolResult(name string, content string, isError bool) {
	if isError {
		fmt.Printf("%s%sError:%s %s\n", Red, Bold, Reset, content)
	} else {
		// Show truncated result
		lines := strings.Split(content, "\n")
		maxLines := 20
		if len(lines) > maxLines {
			for _, line := range lines[:maxLines] {
				fmt.Printf("%s%s%s\n", Gray, line, Reset)
			}
			fmt.Printf("%s... (%d more lines)%s\n", Dim, len(lines)-maxLines, Reset)
		} else {
			for _, line := range lines {
				fmt.Printf("%s%s%s\n", Gray, line, Reset)
			}
		}
	}
}

// PrintThinking displays a thinking indicator.
func PrintThinking(text string) {
	if text != "" {
		fmt.Printf("%s%s%s", Dim, text, Reset)
	}
}

// PrintCostSummary displays cost information.
func PrintCostSummary(summary string) {
	fmt.Printf("\n%s%s%s\n", Dim, summary, Reset)
}

// PrintError displays an error message.
func PrintError(msg string) {
	fmt.Printf("%s%sError: %s%s\n", Red, Bold, msg, Reset)
}

// PrintDivider displays a horizontal divider.
func PrintDivider() {
	fmt.Printf("%s%s%s\n", Dim, strings.Repeat("─", 50), Reset)
}

// ReadInput reads a line of input from the user with a prompt.
func ReadInput(prompt string) (string, error) {
	fmt.Printf("%s%s%s", Bold, prompt, Reset)
	reader := bufio.NewReader(os.Stdin)
	input, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.TrimRight(input, "\n\r"), nil
}

// ReadMultilineInput reads multiple lines until a blank line is entered.
func ReadMultilineInput() (string, error) {
	var lines []string
	reader := bufio.NewReader(os.Stdin)

	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if len(lines) > 0 {
				return strings.Join(lines, "\n"), nil
			}
			return "", err
		}
		line = strings.TrimRight(line, "\n\r")
		if line == "" && len(lines) > 0 {
			break
		}
		lines = append(lines, line)
	}

	return strings.Join(lines, "\n"), nil
}

// Confirm asks the user for a yes/no confirmation.
func Confirm(prompt string) bool {
	fmt.Printf("%s%s%s [y/N] ", Bold, prompt, Reset)
	reader := bufio.NewReader(os.Stdin)
	answer, _ := reader.ReadString('\n')
	answer = strings.TrimSpace(strings.ToLower(answer))
	return answer == "y" || answer == "yes"
}
