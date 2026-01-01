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

// OpenRouter implements Provider using OpenRouter API
type OpenRouter struct {
	APIKey  string
	Model   string
	BaseURL string
	Timeout time.Duration
	client  *http.Client
}

// NewOpenRouter creates a new OpenRouter provider
func NewOpenRouter(model string) *OpenRouter {
	apiKey := config.GetOpenRouterKey()
	return &OpenRouter{
		APIKey:  apiKey,
		Model:   model,
		BaseURL: "https://openrouter.ai/api/v1",
		Timeout: 2 * time.Minute,
		client:  &http.Client{Timeout: 2 * time.Minute},
	}
}

// NewOpenRouterWithKey creates a new OpenRouter provider with explicit API key
func NewOpenRouterWithKey(apiKey, model string) *OpenRouter {
	return &OpenRouter{
		APIKey:  apiKey,
		Model:   model,
		BaseURL: "https://openrouter.ai/api/v1",
		Timeout: 2 * time.Minute,
		client:  &http.Client{Timeout: 2 * time.Minute},
	}
}

// convertMessages converts internal messages to OpenAI-compatible format
func (o *OpenRouter) convertMessages(messages []Message) []openAIMessage {
	result := make([]openAIMessage, 0, len(messages))
	for _, msg := range messages {
		result = append(result, openAIMessage(msg))
	}
	return result
}

// Generate calls OpenRouter API and returns the response
func (o *OpenRouter) Generate(ctx context.Context, messages []Message) (string, error) {
	if o.APIKey == "" {
		return "", fmt.Errorf("OpenRouter API key not configured. Use 'zcode config set openrouter <key>' or set OPENROUTER_API_KEY")
	}

	reqBody := openAIRequest{
		Model:    o.Model,
		Messages: o.convertMessages(messages),
		Stream:   false,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", o.BaseURL+"/chat/completions", bytes.NewReader(jsonBody))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+o.APIKey)
	req.Header.Set("HTTP-Referer", "https://github.com/simonyos/Z-CODE")
	req.Header.Set("X-Title", "Z-Code")

	resp, err := o.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	var openAIResp openAIResponse
	if err := json.Unmarshal(body, &openAIResp); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	if openAIResp.Error != nil {
		return "", fmt.Errorf("OpenRouter API error: %s", openAIResp.Error.Message)
	}

	if len(openAIResp.Choices) == 0 {
		return "", fmt.Errorf("no response choices returned")
	}

	return openAIResp.Choices[0].Message.Content, nil
}

// GenerateStream calls OpenRouter API and streams the response
func (o *OpenRouter) GenerateStream(ctx context.Context, messages []Message) (<-chan StreamChunk, error) {
	if o.APIKey == "" {
		return nil, fmt.Errorf("OpenRouter API key not configured. Use 'zcode config set openrouter <key>' or set OPENROUTER_API_KEY")
	}

	reqBody := openAIRequest{
		Model:    o.Model,
		Messages: o.convertMessages(messages),
		Stream:   true,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", o.BaseURL+"/chat/completions", bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+o.APIKey)
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("HTTP-Referer", "https://github.com/simonyos/Z-CODE")
	req.Header.Set("X-Title", "Z-Code")

	resp, err := o.client.Do(req)
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
func (o *OpenRouter) ModelName() string {
	return o.Model
}
