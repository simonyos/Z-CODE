package llm

import (
	"context"
	"testing"
)

func TestMessage(t *testing.T) {
	msg := Message{
		Role:    "user",
		Content: "Hello, world!",
	}

	if msg.Role != "user" {
		t.Errorf("Message.Role = %q, want %q", msg.Role, "user")
	}
	if msg.Content != "Hello, world!" {
		t.Errorf("Message.Content = %q, want %q", msg.Content, "Hello, world!")
	}
}

func TestStreamChunk(t *testing.T) {
	tests := []struct {
		name  string
		chunk StreamChunk
	}{
		{
			name: "text chunk",
			chunk: StreamChunk{
				Text: "Hello",
				Done: false,
			},
		},
		{
			name: "final chunk",
			chunk: StreamChunk{
				Text: "Complete response",
				Done: true,
			},
		},
		{
			name: "error chunk",
			chunk: StreamChunk{
				Error: context.Canceled,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Just verify the struct is usable
			_ = tt.chunk.Text
			_ = tt.chunk.Done
			_ = tt.chunk.Error
		})
	}
}

// MockProvider is a test implementation of the Provider interface
type MockProvider struct {
	GenerateFunc       func(ctx context.Context, messages []Message) (string, error)
	GenerateStreamFunc func(ctx context.Context, messages []Message) (<-chan StreamChunk, error)
}

func (m *MockProvider) Generate(ctx context.Context, messages []Message) (string, error) {
	if m.GenerateFunc != nil {
		return m.GenerateFunc(ctx, messages)
	}
	return "mock response", nil
}

func (m *MockProvider) GenerateStream(ctx context.Context, messages []Message) (<-chan StreamChunk, error) {
	if m.GenerateStreamFunc != nil {
		return m.GenerateStreamFunc(ctx, messages)
	}
	ch := make(chan StreamChunk, 1)
	ch <- StreamChunk{Text: "mock stream response", Done: true}
	close(ch)
	return ch, nil
}

func TestMockProvider(t *testing.T) {
	ctx := context.Background()

	t.Run("Generate", func(t *testing.T) {
		provider := &MockProvider{
			GenerateFunc: func(ctx context.Context, messages []Message) (string, error) {
				return "custom response", nil
			},
		}

		response, err := provider.Generate(ctx, []Message{{Role: "user", Content: "test"}})
		if err != nil {
			t.Fatalf("Generate() error = %v", err)
		}
		if response != "custom response" {
			t.Errorf("Generate() = %q, want %q", response, "custom response")
		}
	})

	t.Run("GenerateStream", func(t *testing.T) {
		provider := &MockProvider{}

		ch, err := provider.GenerateStream(ctx, []Message{{Role: "user", Content: "test"}})
		if err != nil {
			t.Fatalf("GenerateStream() error = %v", err)
		}

		var chunks []StreamChunk
		for chunk := range ch {
			chunks = append(chunks, chunk)
		}

		if len(chunks) != 1 {
			t.Errorf("GenerateStream() returned %d chunks, want 1", len(chunks))
		}
		if !chunks[0].Done {
			t.Error("GenerateStream() last chunk should have Done=true")
		}
	})
}

func TestClaudeCLI_buildPrompt(t *testing.T) {
	cli := NewClaudeCLI()

	messages := []Message{
		{Role: "system", Content: "You are helpful."},
		{Role: "user", Content: "Hello"},
		{Role: "assistant", Content: "Hi there!"},
		{Role: "user", Content: "How are you?"},
	}

	prompt, systemPrompt := cli.buildPrompt(messages)

	// Check that prompt is not empty
	if prompt == "" {
		t.Error("buildPrompt() returned empty prompt")
	}

	// System message should be extracted separately
	if systemPrompt != "You are helpful." {
		t.Errorf("buildPrompt() systemPrompt = %q, want %q", systemPrompt, "You are helpful.")
	}

	// The prompt should contain user and assistant message contents
	for _, msg := range messages {
		if msg.Role == "system" {
			continue // System is extracted separately
		}
		if msg.Content != "" && !contains(prompt, msg.Content) {
			t.Errorf("buildPrompt() should contain %q", msg.Content)
		}
	}
}

func TestGeminiCLI_buildPrompt(t *testing.T) {
	cli := NewGeminiCLI()

	messages := []Message{
		{Role: "system", Content: "You are helpful."},
		{Role: "user", Content: "Hello"},
		{Role: "assistant", Content: "Hi there!"},
	}

	prompt := cli.buildPrompt(messages)

	if prompt == "" {
		t.Error("buildPrompt() returned empty string")
	}

	// Check structure
	for _, msg := range messages {
		if msg.Content != "" && !contains(prompt, msg.Content) {
			t.Errorf("buildPrompt() should contain %q", msg.Content)
		}
	}
}

func TestOpenAI_convertMessages(t *testing.T) {
	openai := NewOpenAI("gpt-4o")
	openai.APIKey = "test-key" // Set a test key

	messages := []Message{
		{Role: "system", Content: "You are helpful."},
		{Role: "user", Content: "Hello"},
		{Role: "assistant", Content: "Hi!"},
	}

	converted := openai.convertMessages(messages)

	if len(converted) != len(messages) {
		t.Errorf("convertMessages() returned %d messages, want %d", len(converted), len(messages))
	}

	for i, msg := range converted {
		if msg.Role != messages[i].Role {
			t.Errorf("convertMessages()[%d].Role = %q, want %q", i, msg.Role, messages[i].Role)
		}
		if msg.Content != messages[i].Content {
			t.Errorf("convertMessages()[%d].Content = %q, want %q", i, msg.Content, messages[i].Content)
		}
	}
}

func TestOpenAI_ModelName(t *testing.T) {
	tests := []string{"gpt-4o", "gpt-4-turbo", "gpt-3.5-turbo"}

	for _, model := range tests {
		t.Run(model, func(t *testing.T) {
			openai := NewOpenAI(model)
			if openai.ModelName() != model {
				t.Errorf("ModelName() = %q, want %q", openai.ModelName(), model)
			}
		})
	}
}

func TestNewClaudeCLI(t *testing.T) {
	cli := NewClaudeCLI()
	if cli == nil {
		t.Fatal("NewClaudeCLI() returned nil")
	}
	if cli.Timeout <= 0 {
		t.Error("NewClaudeCLI().Timeout should be positive")
	}
}

func TestNewGeminiCLI(t *testing.T) {
	cli := NewGeminiCLI()
	if cli == nil {
		t.Fatal("NewGeminiCLI() returned nil")
	}
	if cli.Timeout <= 0 {
		t.Error("NewGeminiCLI().Timeout should be positive")
	}
}

func TestNewOpenAI(t *testing.T) {
	openai := NewOpenAI("gpt-4o")
	if openai == nil {
		t.Fatal("NewOpenAI() returned nil")
	}
	if openai.Model != "gpt-4o" {
		t.Errorf("NewOpenAI().Model = %q, want %q", openai.Model, "gpt-4o")
	}
	if openai.BaseURL != "https://api.openai.com/v1" {
		t.Errorf("NewOpenAI().BaseURL = %q, want %q", openai.BaseURL, "https://api.openai.com/v1")
	}
}

// Helper function
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
