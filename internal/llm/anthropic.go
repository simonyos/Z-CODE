// Package llm - Anthropic provider for Z-CODE
// Native Claude API support with tool calling
package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/simonyos/Z-CODE/internal/config"
)

// Default timeout for Anthropic API requests (Claude can take longer for complex tasks)
const defaultAnthropicTimeout = 5 * time.Minute

// Anthropic implements Provider using Claude API
type Anthropic struct {
	APIKey  string
	Model   string
	BaseURL string
	client  *http.Client
}

// Anthropic API types
type anthropicRequest struct {
	Model     string             `json:"model"`
	MaxTokens int                `json:"max_tokens"`
	System    string             `json:"system,omitempty"`
	Messages  []anthropicMessage `json:"messages"`
	Stream    bool               `json:"stream,omitempty"`
	Tools     []anthropicTool    `json:"tools,omitempty"`
}

type anthropicMessage struct {
	Role    string      `json:"role"`
	Content interface{} `json:"content"` // string or []anthropicContentBlock
}

type anthropicContentBlock struct {
	Type      string `json:"type"`                  // "text", "tool_use", "tool_result"
	Text      string `json:"text,omitempty"`        // for text blocks
	ID        string `json:"id,omitempty"`          // for tool_use blocks
	Name      string `json:"name,omitempty"`        // for tool_use blocks
	Input     any    `json:"input,omitempty"`       // for tool_use blocks
	ToolUseID string `json:"tool_use_id,omitempty"` // for tool_result blocks
	Content   string `json:"content,omitempty"`     // for tool_result blocks (result text)
}

type anthropicTool struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"input_schema"`
}

type anthropicResponse struct {
	ID           string                  `json:"id"`
	Type         string                  `json:"type"`
	Role         string                  `json:"role"`
	Content      []anthropicContentBlock `json:"content"`
	Model        string                  `json:"model"`
	StopReason   string                  `json:"stop_reason"`
	StopSequence *string                 `json:"stop_sequence"`
	Usage        struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
	Error *anthropicError `json:"error,omitempty"`
}

type anthropicError struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

// Streaming event types
type anthropicStreamEvent struct {
	Type         string `json:"type"`
	Index        int    `json:"index,omitempty"`
	ContentBlock *struct {
		Type  string `json:"type"`
		ID    string `json:"id,omitempty"`
		Name  string `json:"name,omitempty"`
		Text  string `json:"text,omitempty"`
		Input any    `json:"input,omitempty"`
	} `json:"content_block,omitempty"`
	Delta *struct {
		Type        string `json:"type"`
		Text        string `json:"text,omitempty"`
		PartialJSON string `json:"partial_json,omitempty"`
	} `json:"delta,omitempty"`
	Message *anthropicResponse `json:"message,omitempty"`
	Usage   *struct {
		OutputTokens int `json:"output_tokens"`
	} `json:"usage,omitempty"`
}

// NewAnthropic creates a new Anthropic provider
func NewAnthropic(model string) *Anthropic {
	apiKey := config.GetAnthropicKey()
	if model == "" {
		model = "claude-sonnet-4-20250514" // Default to Claude Sonnet 4
	}
	return &Anthropic{
		APIKey:  apiKey,
		Model:   model,
		BaseURL: "https://api.anthropic.com/v1",
		client:  &http.Client{Timeout: defaultAnthropicTimeout},
	}
}

// NewAnthropicWithKey creates a new Anthropic provider with explicit API key
func NewAnthropicWithKey(apiKey, model string) *Anthropic {
	if model == "" {
		model = "claude-sonnet-4-20250514"
	}
	return &Anthropic{
		APIKey:  apiKey,
		Model:   model,
		BaseURL: "https://api.anthropic.com/v1",
		client:  &http.Client{Timeout: defaultAnthropicTimeout},
	}
}

// convertToAnthropicMessages converts internal messages to Anthropic format
func (a *Anthropic) convertToAnthropicMessages(messages []Message) (string, []anthropicMessage) {
	var systemPrompt string
	var anthropicMsgs []anthropicMessage

	for _, msg := range messages {
		if msg.Role == "system" {
			systemPrompt = msg.Content
			continue
		}

		// Handle tool result messages
		if msg.Role == "tool" {
			// Tool results are added as user messages with tool_result content
			anthropicMsgs = append(anthropicMsgs, anthropicMessage{
				Role: "user",
				Content: []anthropicContentBlock{{
					Type:      "tool_result",
					ToolUseID: msg.ToolCallID,
					Content:   msg.Content,
				}},
			})
			continue
		}

		// Handle assistant messages with tool calls
		if msg.Role == "assistant" && len(msg.ToolCalls) > 0 {
			var blocks []anthropicContentBlock
			if msg.Content != "" {
				blocks = append(blocks, anthropicContentBlock{
					Type: "text",
					Text: msg.Content,
				})
			}
			for _, tc := range msg.ToolCalls {
				var input any
				if err := json.Unmarshal([]byte(tc.Function.Arguments), &input); err != nil {
					input = map[string]any{} // fallback to empty object
				}
				blocks = append(blocks, anthropicContentBlock{
					Type:  "tool_use",
					ID:    tc.ID,
					Name:  tc.Function.Name,
					Input: input,
				})
			}
			anthropicMsgs = append(anthropicMsgs, anthropicMessage{
				Role:    "assistant",
				Content: blocks,
			})
			continue
		}

		// Regular text messages
		anthropicMsgs = append(anthropicMsgs, anthropicMessage{
			Role:    msg.Role,
			Content: msg.Content,
		})
	}

	return systemPrompt, anthropicMsgs
}

// convertToolsToAnthropic converts OpenAI tool format to Anthropic format
func convertToolsToAnthropic(tools []OpenAITool) []anthropicTool {
	result := make([]anthropicTool, 0, len(tools))
	for _, t := range tools {
		result = append(result, anthropicTool{
			Name:        t.Function.Name,
			Description: t.Function.Description,
			InputSchema: t.Function.Parameters,
		})
	}
	return result
}

// Generate calls Anthropic API and returns the response
func (a *Anthropic) Generate(ctx context.Context, messages []Message) (string, error) {
	if a.APIKey == "" {
		return "", fmt.Errorf("Anthropic API key not configured. Use 'zcode config set anthropic <key>' or set ANTHROPIC_API_KEY")
	}

	systemPrompt, anthropicMsgs := a.convertToAnthropicMessages(messages)

	reqBody := anthropicRequest{
		Model:     a.Model,
		MaxTokens: 8192,
		System:    systemPrompt,
		Messages:  anthropicMsgs,
		Stream:    false,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", a.BaseURL+"/messages", bytes.NewReader(jsonBody))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", a.APIKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := a.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var anthropicResp anthropicResponse
	if err := json.Unmarshal(body, &anthropicResp); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	if anthropicResp.Error != nil {
		return "", fmt.Errorf("Anthropic API error: %s", anthropicResp.Error.Message)
	}

	// Extract text content
	var result strings.Builder
	for _, block := range anthropicResp.Content {
		if block.Type == "text" {
			result.WriteString(block.Text)
		}
	}

	return result.String(), nil
}

// GenerateStream calls Anthropic API and streams the response
func (a *Anthropic) GenerateStream(ctx context.Context, messages []Message) (<-chan StreamChunk, error) {
	if a.APIKey == "" {
		return nil, fmt.Errorf("Anthropic API key not configured. Use 'zcode config set anthropic <key>' or set ANTHROPIC_API_KEY")
	}

	systemPrompt, anthropicMsgs := a.convertToAnthropicMessages(messages)

	reqBody := anthropicRequest{
		Model:     a.Model,
		MaxTokens: 8192,
		System:    systemPrompt,
		Messages:  anthropicMsgs,
		Stream:    true,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", a.BaseURL+"/messages", bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", a.APIKey)
	req.Header.Set("anthropic-version", "2023-06-01")
	req.Header.Set("Accept", "text/event-stream")

	resp, err := a.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	chunks := make(chan StreamChunk)

	go func() {
		defer close(chunks)
		defer resp.Body.Close()

		reader := bufio.NewReader(resp.Body)
		var fullContent strings.Builder

		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				if err == io.EOF {
					break
				}
				chunks <- StreamChunk{Error: fmt.Errorf("error reading stream: %w", err)}
				return
			}

			line = strings.TrimSpace(line)
			if line == "" || !strings.HasPrefix(line, "data: ") {
				continue
			}

			data := strings.TrimPrefix(line, "data: ")

			var event anthropicStreamEvent
			if err := json.Unmarshal([]byte(data), &event); err != nil {
				continue
			}

			switch event.Type {
			case "content_block_delta":
				if event.Delta != nil && event.Delta.Type == "text_delta" {
					fullContent.WriteString(event.Delta.Text)
					select {
					case chunks <- StreamChunk{Text: event.Delta.Text, Done: false}:
					case <-ctx.Done():
						return
					}
				}
			case "message_stop":
				chunks <- StreamChunk{Text: fullContent.String(), Done: true}
				return
			}
		}
	}()

	return chunks, nil
}

// GenerateWithTools calls Anthropic API with tool definitions
func (a *Anthropic) GenerateWithTools(ctx context.Context, messages []Message, tools []OpenAITool) (*ToolCallResponse, error) {
	if a.APIKey == "" {
		return nil, fmt.Errorf("Anthropic API key not configured. Use 'zcode config set anthropic <key>' or set ANTHROPIC_API_KEY")
	}

	systemPrompt, anthropicMsgs := a.convertToAnthropicMessages(messages)

	reqBody := anthropicRequest{
		Model:     a.Model,
		MaxTokens: 8192,
		System:    systemPrompt,
		Messages:  anthropicMsgs,
		Stream:    false,
		Tools:     convertToolsToAnthropic(tools),
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", a.BaseURL+"/messages", bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", a.APIKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := a.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var anthropicResp anthropicResponse
	if err := json.Unmarshal(body, &anthropicResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if anthropicResp.Error != nil {
		return nil, fmt.Errorf("Anthropic API error: %s", anthropicResp.Error.Message)
	}

	// Convert response to ToolCallResponse
	var textContent strings.Builder
	var toolCalls []OpenAIToolCall

	for _, block := range anthropicResp.Content {
		switch block.Type {
		case "text":
			textContent.WriteString(block.Text)
		case "tool_use":
			// Convert input to JSON string
			inputJSON, _ := json.Marshal(block.Input)
			toolCalls = append(toolCalls, OpenAIToolCall{
				ID:   block.ID,
				Type: "function",
				Function: struct {
					Name      string `json:"name"`
					Arguments string `json:"arguments"`
				}{
					Name:      block.Name,
					Arguments: string(inputJSON),
				},
			})
		}
	}

	return &ToolCallResponse{
		Content:   textContent.String(),
		ToolCalls: toolCalls,
		Done:      len(toolCalls) == 0,
	}, nil
}

// GenerateStreamWithTools calls Anthropic API and streams with tool call support
func (a *Anthropic) GenerateStreamWithTools(ctx context.Context, messages []Message, tools []OpenAITool) (<-chan ToolStreamChunk, error) {
	if a.APIKey == "" {
		return nil, fmt.Errorf("Anthropic API key not configured. Use 'zcode config set anthropic <key>' or set ANTHROPIC_API_KEY")
	}

	systemPrompt, anthropicMsgs := a.convertToAnthropicMessages(messages)

	reqBody := anthropicRequest{
		Model:     a.Model,
		MaxTokens: 8192,
		System:    systemPrompt,
		Messages:  anthropicMsgs,
		Stream:    true,
		Tools:     convertToolsToAnthropic(tools),
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", a.BaseURL+"/messages", bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", a.APIKey)
	req.Header.Set("anthropic-version", "2023-06-01")
	req.Header.Set("Accept", "text/event-stream")

	resp, err := a.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	chunks := make(chan ToolStreamChunk)

	go func() {
		defer close(chunks)
		defer resp.Body.Close()

		reader := bufio.NewReader(resp.Body)
		var fullContent strings.Builder
		var currentToolCall *OpenAIToolCall
		var toolCalls []OpenAIToolCall
		var currentToolInput strings.Builder

		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				if err == io.EOF {
					break
				}
				chunks <- ToolStreamChunk{Error: fmt.Errorf("error reading stream: %w", err)}
				return
			}

			line = strings.TrimSpace(line)
			if line == "" || !strings.HasPrefix(line, "data: ") {
				continue
			}

			data := strings.TrimPrefix(line, "data: ")

			var event anthropicStreamEvent
			if err := json.Unmarshal([]byte(data), &event); err != nil {
				continue
			}

			switch event.Type {
			case "content_block_start":
				if event.ContentBlock != nil {
					if event.ContentBlock.Type == "tool_use" {
						currentToolCall = &OpenAIToolCall{
							ID:   event.ContentBlock.ID,
							Type: "function",
							Function: struct {
								Name      string `json:"name"`
								Arguments string `json:"arguments"`
							}{
								Name: event.ContentBlock.Name,
							},
						}
						currentToolInput.Reset()
					}
				}
			case "content_block_delta":
				if event.Delta != nil {
					switch event.Delta.Type {
					case "text_delta":
						fullContent.WriteString(event.Delta.Text)
						select {
						case chunks <- ToolStreamChunk{Text: event.Delta.Text, Done: false}:
						case <-ctx.Done():
							return
						}
					case "input_json_delta":
						if currentToolCall != nil {
							currentToolInput.WriteString(event.Delta.PartialJSON)
						}
					}
				}
			case "content_block_stop":
				if currentToolCall != nil {
					currentToolCall.Function.Arguments = currentToolInput.String()
					toolCalls = append(toolCalls, *currentToolCall)
					currentToolCall = nil
				}
			case "message_stop":
				chunks <- ToolStreamChunk{
					Text:      fullContent.String(),
					ToolCalls: toolCalls,
					Done:      true,
				}
				return
			}
		}
	}()

	return chunks, nil
}

// ModelName returns the model being used
func (a *Anthropic) ModelName() string {
	return a.Model
}

// Ensure Anthropic implements ToolProvider
var _ ToolProvider = (*Anthropic)(nil)
