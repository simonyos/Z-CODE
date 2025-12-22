package llm

import (
	"bufio"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// ClaudeCLI implements Provider using Claude Code CLI
type ClaudeCLI struct {
	Timeout time.Duration
}

// NewClaudeCLI creates a new Claude CLI provider
func NewClaudeCLI() *ClaudeCLI {
	return &ClaudeCLI{
		Timeout: 2 * time.Minute,
	}
}

// buildPrompt creates the prompt from messages
func (c *ClaudeCLI) buildPrompt(messages []Message) (string, string) {
	var systemPrompt string
	var conversationParts []string

	for _, msg := range messages {
		switch msg.Role {
		case "system":
			systemPrompt = msg.Content
		case "user":
			conversationParts = append(conversationParts, "User: "+msg.Content)
		case "assistant":
			conversationParts = append(conversationParts, "Assistant: "+msg.Content)
		}
	}

	// Build the full prompt with conversation history
	fullPrompt := strings.Join(conversationParts, "\n\n")
	if len(conversationParts) > 1 {
		fullPrompt += "\n\nAssistant:"
	}

	return fullPrompt, systemPrompt
}

// Generate calls Claude CLI and returns the response
func (c *ClaudeCLI) Generate(ctx context.Context, messages []Message) (string, error) {
	fullPrompt, systemPrompt := c.buildPrompt(messages)

	// Build command arguments
	args := []string{"--print", fullPrompt, "--tools", ""}
	if systemPrompt != "" {
		args = append(args, "--system-prompt", systemPrompt)
	}

	// Create command with timeout
	execCtx, cancel := context.WithTimeout(ctx, c.Timeout)
	defer cancel()

	cmd := exec.CommandContext(execCtx, "claude", args...)
	output, err := cmd.Output()

	if execCtx.Err() == context.DeadlineExceeded {
		return "", fmt.Errorf("request timed out")
	}

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return "", fmt.Errorf("claude CLI failed: %s", string(exitErr.Stderr))
		}
		return "", fmt.Errorf("claude CLI failed: %w", err)
	}

	return strings.TrimSpace(string(output)), nil
}

// GenerateStream calls Claude CLI and streams the response line by line
func (c *ClaudeCLI) GenerateStream(ctx context.Context, messages []Message) (<-chan StreamChunk, error) {
	fullPrompt, systemPrompt := c.buildPrompt(messages)

	// Build command arguments
	args := []string{"--print", fullPrompt, "--tools", ""}
	if systemPrompt != "" {
		args = append(args, "--system-prompt", systemPrompt)
	}

	// Create command with timeout
	execCtx, cancel := context.WithTimeout(ctx, c.Timeout)

	cmd := exec.CommandContext(execCtx, "claude", args...)

	// Get stdout pipe for streaming
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to get stdout pipe: %w", err)
	}

	// Start the command
	if err := cmd.Start(); err != nil {
		cancel()
		return nil, fmt.Errorf("failed to start claude CLI: %w", err)
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
				chunks <- StreamChunk{Error: fmt.Errorf("claude CLI failed: %s", string(exitErr.Stderr))}
				return
			}
			chunks <- StreamChunk{Error: fmt.Errorf("claude CLI failed: %w", err)}
			return
		}

		// Send final chunk with complete text
		chunks <- StreamChunk{Text: strings.TrimSpace(fullOutput.String()), Done: true}
	}()

	return chunks, nil
}
