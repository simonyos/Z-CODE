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

// LiteLLM implements Provider using LiteLLM proxy API
// LiteLLM provides a unified interface to 100+ LLM providers using OpenAI-compatible format
type LiteLLM struct {
	APIKey  string
	Model   string
	BaseURL string
	Timeout time.Duration
	client  *http.Client
}

// NewLiteLLM creates a new LiteLLM provider
func NewLiteLLM(model string) *LiteLLM {
	apiKey := config.GetLiteLLMKey()
	baseURL := config.GetLiteLLMBaseURL()
	return &LiteLLM{
		APIKey:  apiKey,
		Model:   model,
		BaseURL: baseURL,
		Timeout: 2 * time.Minute,
		client:  &http.Client{Timeout: 2 * time.Minute},
	}
}

// NewLiteLLMWithConfig creates a new LiteLLM provider with explicit configuration
func NewLiteLLMWithConfig(apiKey, model, baseURL string) *LiteLLM {
	if baseURL == "" {
		baseURL = "http://localhost:4000"
	}
	return &LiteLLM{
		APIKey:  apiKey,
		Model:   model,
		BaseURL: baseURL,
		Timeout: 2 * time.Minute,
		client:  &http.Client{Timeout: 2 * time.Minute},
	}
}

// convertMessages converts internal messages to OpenAI-compatible format
func (l *LiteLLM) convertMessages(messages []Message) []openAIMessage {
	result := make([]openAIMessage, 0, len(messages))
	for _, msg := range messages {
		result = append(result, openAIMessage{
			Role:    msg.Role,
			Content: msg.Content,
		})
	}
	return result
}

// Generate calls LiteLLM API and returns the response
func (l *LiteLLM) Generate(ctx context.Context, messages []Message) (string, error) {
	reqBody := openAIRequest{
		Model:    l.Model,
		Messages: l.convertMessages(messages),
		Stream:   false,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", l.BaseURL+"/chat/completions", bytes.NewReader(jsonBody))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if l.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+l.APIKey)
	}

	resp, err := l.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	var openAIResp openAIResponse
	if err := json.Unmarshal(body, &openAIResp); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	if openAIResp.Error != nil {
		return "", fmt.Errorf("LiteLLM API error: %s", openAIResp.Error.Message)
	}

	if len(openAIResp.Choices) == 0 {
		return "", fmt.Errorf("no response choices returned")
	}

	return openAIResp.Choices[0].Message.Content, nil
}

// GenerateStream calls LiteLLM API and streams the response
func (l *LiteLLM) GenerateStream(ctx context.Context, messages []Message) (<-chan StreamChunk, error) {
	reqBody := openAIRequest{
		Model:    l.Model,
		Messages: l.convertMessages(messages),
		Stream:   true,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", l.BaseURL+"/chat/completions", bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	if l.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+l.APIKey)
	}

	resp, err := l.client.Do(req)
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
			if line == "" {
				continue
			}

			// SSE format: data: {...}
			if !strings.HasPrefix(line, "data: ") {
				continue
			}

			data := strings.TrimPrefix(line, "data: ")
			if data == "[DONE]" {
				break
			}

			var streamResp openAIStreamResponse
			if err := json.Unmarshal([]byte(data), &streamResp); err != nil {
				continue // Skip malformed chunks
			}

			if len(streamResp.Choices) > 0 {
				content := streamResp.Choices[0].Delta.Content
				if content != "" {
					fullContent.WriteString(content)
					select {
					case chunks <- StreamChunk{Text: content, Done: false}:
					case <-ctx.Done():
						return
					}
				}

				if streamResp.Choices[0].FinishReason != nil {
					break
				}
			}
		}

		// Send final chunk with complete text
		chunks <- StreamChunk{Text: fullContent.String(), Done: true}
	}()

	return chunks, nil
}

// ModelName returns the model being used
func (l *LiteLLM) ModelName() string {
	return l.Model
}

// Tool calling support types (OpenAI-compatible)

type toolRequest struct {
	Model      string               `json:"model"`
	Messages   []ToolRequestMessage `json:"messages"`
	Tools      []OpenAITool         `json:"tools,omitempty"`
	ToolChoice interface{}          `json:"tool_choice,omitempty"` // "auto", "none", or specific
	Stream     bool                 `json:"stream,omitempty"`
}

type toolResponse struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	Model   string `json:"model"`
	Choices []struct {
		Index   int `json:"index"`
		Message struct {
			Role      string           `json:"role"`
			Content   string           `json:"content"`
			ToolCalls []OpenAIToolCall `json:"tool_calls,omitempty"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Error *openAIError `json:"error,omitempty"`
}

type toolStreamResponse struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	Model   string `json:"model"`
	Choices []struct {
		Index int `json:"index"`
		Delta struct {
			Role      string          `json:"role,omitempty"`
			Content   string          `json:"content,omitempty"`
			ToolCalls []ToolCallDelta `json:"tool_calls,omitempty"`
		} `json:"delta"`
		FinishReason *string `json:"finish_reason"`
	} `json:"choices"`
}

// GenerateWithTools calls LiteLLM API with tool definitions
func (l *LiteLLM) GenerateWithTools(ctx context.Context, messages []Message, tools []OpenAITool) (*ToolCallResponse, error) {
	reqBody := toolRequest{
		Model:      l.Model,
		Messages:   ConvertMessagesToToolFormat(messages),
		Tools:      tools,
		ToolChoice: "auto",
		Stream:     false,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", l.BaseURL+"/chat/completions", bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if l.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+l.APIKey)
	}

	resp, err := l.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var toolResp toolResponse
	if err := json.Unmarshal(body, &toolResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if toolResp.Error != nil {
		return nil, fmt.Errorf("LiteLLM API error: %s", toolResp.Error.Message)
	}

	if len(toolResp.Choices) == 0 {
		return nil, fmt.Errorf("no response choices returned")
	}

	choice := toolResp.Choices[0]
	return &ToolCallResponse{
		Content:   choice.Message.Content,
		ToolCalls: choice.Message.ToolCalls,
		Done:      len(choice.Message.ToolCalls) == 0,
	}, nil
}

// GenerateStreamWithTools calls LiteLLM API and streams the response with tool call support
func (l *LiteLLM) GenerateStreamWithTools(ctx context.Context, messages []Message, tools []OpenAITool) (<-chan ToolStreamChunk, error) {
	reqBody := toolRequest{
		Model:      l.Model,
		Messages:   ConvertMessagesToToolFormat(messages),
		Tools:      tools,
		ToolChoice: "auto",
		Stream:     true,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", l.BaseURL+"/chat/completions", bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	if l.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+l.APIKey)
	}

	resp, err := l.client.Do(req)
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
		accumulator := NewToolCallAccumulator()

		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				if err == io.EOF {
					break
				}
				chunks <- ToolStreamChunk{Error: fmt.Errorf("error reading stream: %w", err)}
				return
			}

			data := ParseSSELine(line)
			if data == "" {
				continue
			}

			var streamResp toolStreamResponse
			if err := json.Unmarshal([]byte(data), &streamResp); err != nil {
				continue
			}

			if len(streamResp.Choices) > 0 {
				delta := streamResp.Choices[0].Delta

				// Handle text content
				if delta.Content != "" {
					fullContent.WriteString(delta.Content)
					select {
					case chunks <- ToolStreamChunk{Text: delta.Content, Done: false}:
					case <-ctx.Done():
						return
					}
				}

				// Handle tool call deltas
				for _, tcDelta := range delta.ToolCalls {
					accumulator.AddDelta(tcDelta)
				}

				if streamResp.Choices[0].FinishReason != nil {
					break
				}
			}
		}

		// Send final chunk with complete content and tool calls
		chunks <- ToolStreamChunk{
			Text:      fullContent.String(),
			ToolCalls: accumulator.GetToolCalls(),
			Done:      true,
		}
	}()

	return chunks, nil
}

// Ensure LiteLLM implements ToolProvider
var _ ToolProvider = (*LiteLLM)(nil)
