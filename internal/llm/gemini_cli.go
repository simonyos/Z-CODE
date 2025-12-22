package llm

import (
	"bufio"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// GeminiCLI implements Provider using Gemini CLI
type GeminiCLI struct {
	Timeout time.Duration
}

// NewGeminiCLI creates a new Gemini CLI provider
func NewGeminiCLI() *GeminiCLI {
	return &GeminiCLI{
		Timeout: 2 * time.Minute,
	}
}

// buildPrompt creates the prompt from messages
func (g *GeminiCLI) buildPrompt(messages []Message) string {
	var conversationParts []string

	for _, msg := range messages {
		switch msg.Role {
		case "system":
			conversationParts = append(conversationParts, "System: "+msg.Content)
		case "user":
			conversationParts = append(conversationParts, "User: "+msg.Content)
		case "assistant":
			conversationParts = append(conversationParts, "Assistant: "+msg.Content)
		}
	}

	return strings.Join(conversationParts, "\n\n")
}

// Generate calls Gemini CLI and returns the response
func (g *GeminiCLI) Generate(ctx context.Context, messages []Message) (string, error) {
	prompt := g.buildPrompt(messages)

	// Create command with timeout
	// Use -p flag for non-interactive prompt mode
	execCtx, cancel := context.WithTimeout(ctx, g.Timeout)
	defer cancel()

	cmd := exec.CommandContext(execCtx, "gemini", "-p", prompt)
	output, err := cmd.Output()

	if execCtx.Err() == context.DeadlineExceeded {
		return "", fmt.Errorf("request timed out")
	}

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return "", fmt.Errorf("gemini CLI failed: %s", string(exitErr.Stderr))
		}
		// Check if gemini is not installed
		if strings.Contains(err.Error(), "executable file not found") {
			return "", fmt.Errorf("gemini CLI not found. Install with: npm install -g @google/gemini-cli")
		}
		return "", fmt.Errorf("gemini CLI failed: %w", err)
	}

	return strings.TrimSpace(string(output)), nil
}

// GenerateStream calls Gemini CLI and streams the response line by line
func (g *GeminiCLI) GenerateStream(ctx context.Context, messages []Message) (<-chan StreamChunk, error) {
	prompt := g.buildPrompt(messages)

	// Create command with timeout
	// Use -p flag for non-interactive prompt mode
	execCtx, cancel := context.WithTimeout(ctx, g.Timeout)

	cmd := exec.CommandContext(execCtx, "gemini", "-p", prompt)

	// Get stdout pipe for streaming
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to get stdout pipe: %w", err)
	}

	// Start the command
	if err := cmd.Start(); err != nil {
		cancel()
		// Check if gemini is not installed
		if strings.Contains(err.Error(), "executable file not found") {
			return nil, fmt.Errorf("gemini CLI not found. Install with: npm install -g @google/gemini-cli")
		}
		return nil, fmt.Errorf("failed to start gemini CLI: %w", err)
	}

	// Create output channel
	chunks := make(chan StreamChunk)

	// Read output in goroutine
	go func() {
		defer close(chunks)
		defer cancel()

		scanner := bufio.NewScanner(stdout)
		var fullOutput strings.Builder

		for scanner.Scan() {
			line := scanner.Text()
			fullOutput.WriteString(line)
			fullOutput.WriteString("\n")

			select {
			case chunks <- StreamChunk{Text: line + "\n", Done: false}:
			case <-execCtx.Done():
				return
			}
		}

		if err := scanner.Err(); err != nil {
			chunks <- StreamChunk{Error: fmt.Errorf("error reading output: %w", err)}
			return
		}

		// Wait for command to finish
		if err := cmd.Wait(); err != nil {
			if execCtx.Err() == context.DeadlineExceeded {
				chunks <- StreamChunk{Error: fmt.Errorf("request timed out")}
				return
			}
			if exitErr, ok := err.(*exec.ExitError); ok {
				chunks <- StreamChunk{Error: fmt.Errorf("gemini CLI failed: %s", string(exitErr.Stderr))}
				return
			}
			chunks <- StreamChunk{Error: fmt.Errorf("gemini CLI failed: %w", err)}
			return
		}

		// Send final chunk with complete text
		chunks <- StreamChunk{Text: strings.TrimSpace(fullOutput.String()), Done: true}
	}()

	return chunks, nil
}
