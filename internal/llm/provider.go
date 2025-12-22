package llm

import "context"

// Message represents a chat message
type Message struct {
	Role    string `json:"role"` // "user", "assistant", "system"
	Content string `json:"content"`
}

// StreamChunk represents a piece of streaming output
type StreamChunk struct {
	Text  string // Text content
	Done  bool   // True if this is the final chunk
	Error error  // Error if any
}

// Provider is the interface for LLM backends
type Provider interface {
	// Generate produces a response given messages
	Generate(ctx context.Context, messages []Message) (string, error)

	// GenerateStream produces a streaming response
	GenerateStream(ctx context.Context, messages []Message) (<-chan StreamChunk, error)
}
