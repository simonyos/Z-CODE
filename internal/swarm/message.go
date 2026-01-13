package swarm

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// MessageType represents the type of swarm message.
type MessageType string

// Message types for inter-agent communication.
const (
	MsgBroadcast     MessageType = "BROADCAST"      // Message to all agents
	MsgRequest       MessageType = "REQUEST"        // Question or task for specific agent
	MsgResponse      MessageType = "RESPONSE"       // Reply to a request
	MsgHandoff       MessageType = "HANDOFF"        // Transfer work to another agent
	MsgStatus        MessageType = "STATUS"         // Progress update
	MsgReviewRequest MessageType = "REVIEW_REQUEST" // Ask for code/design review
	MsgApproval      MessageType = "APPROVAL"       // Approve a proposal or PR
	MsgRejection     MessageType = "REJECTION"      // Reject with feedback
	MsgHumanOverride MessageType = "HUMAN_OVERRIDE" // Human takes control
	MsgPause         MessageType = "PAUSE"          // Pause agent autopilot
	MsgResume        MessageType = "RESUME"         // Resume agent autopilot
)

// Priority levels for messages.
type Priority string

const (
	PriorityLow    Priority = "low"
	PriorityNormal Priority = "normal"
	PriorityHigh   Priority = "high"
	PriorityUrgent Priority = "urgent"
)

// CodeBlock represents a code snippet in a message.
type CodeBlock struct {
	Language string `json:"language"`
	Code     string `json:"code"`
	Filename string `json:"filename,omitempty"`
}

// MessageMetadata contains additional information about a message.
type MessageMetadata struct {
	Files       []string    `json:"files,omitempty"`        // File paths referenced
	CodeBlocks  []CodeBlock `json:"code_blocks,omitempty"`  // Code snippets
	References  []string    `json:"references,omitempty"`   // Message IDs referenced
	Priority    Priority    `json:"priority,omitempty"`     // Message priority
	RequiresACK bool        `json:"requires_ack,omitempty"` // Needs acknowledgment
	TaskID      string      `json:"task_id,omitempty"`      // Associated task ID
}

// Message represents a swarm message between agents.
type Message struct {
	ID        string          `json:"id"`                   // Unique message ID
	RoomID    string          `json:"room_id"`              // Room this message belongs to
	Timestamp time.Time       `json:"timestamp"`            // When the message was sent
	From      Role            `json:"from"`                 // Sender role
	To        Role            `json:"to"`                   // Target role (or "ALL" for broadcast)
	Type      MessageType     `json:"type"`                 // Message type
	Content   string          `json:"content"`              // Natural language content
	ReplyTo   string          `json:"reply_to,omitempty"`   // ID of message being replied to
	Metadata  MessageMetadata `json:"metadata,omitempty"`   // Additional metadata
}

// NewMessage creates a new message with a generated ID and timestamp.
func NewMessage(roomID string, from, to Role, msgType MessageType, content string) *Message {
	return &Message{
		ID:        uuid.New().String(),
		RoomID:    roomID,
		Timestamp: time.Now(),
		From:      from,
		To:        to,
		Type:      msgType,
		Content:   content,
	}
}

// NewBroadcast creates a new broadcast message.
func NewBroadcast(roomID string, from Role, content string) *Message {
	return NewMessage(roomID, from, RoleAll, MsgBroadcast, content)
}

// NewRequest creates a new request message.
func NewRequest(roomID string, from, to Role, content string) *Message {
	return NewMessage(roomID, from, to, MsgRequest, content)
}

// NewResponse creates a new response message.
func NewResponse(roomID string, from, to Role, content string, replyTo string) *Message {
	msg := NewMessage(roomID, from, to, MsgResponse, content)
	msg.ReplyTo = replyTo
	return msg
}

// NewStatus creates a new status update message.
func NewStatus(roomID string, from Role, content string) *Message {
	return NewMessage(roomID, from, RoleAll, MsgStatus, content)
}

// NewApproval creates a new approval message.
func NewApproval(roomID string, from, to Role, content string, replyTo string) *Message {
	msg := NewMessage(roomID, from, to, MsgApproval, content)
	msg.ReplyTo = replyTo
	return msg
}

// NewRejection creates a new rejection message.
func NewRejection(roomID string, from, to Role, content string, replyTo string) *Message {
	msg := NewMessage(roomID, from, to, MsgRejection, content)
	msg.ReplyTo = replyTo
	return msg
}

// NewPause creates a pause command message (typically from HUMAN).
func NewPause(roomID string, from Role, reason string) *Message {
	msg := NewMessage(roomID, from, RoleAll, MsgPause, reason)
	msg.Metadata.Priority = PriorityUrgent
	return msg
}

// NewResume creates a resume command message (typically from HUMAN).
func NewResume(roomID string, from Role, message string) *Message {
	msg := NewMessage(roomID, from, RoleAll, MsgResume, message)
	msg.Metadata.Priority = PriorityUrgent
	return msg
}

// NewOverride creates a human override message with urgent priority.
func NewOverride(roomID string, from, to Role, instruction string) *Message {
	msg := NewMessage(roomID, from, to, MsgHumanOverride, instruction)
	msg.Metadata.Priority = PriorityUrgent
	return msg
}

// IsControlMessage returns true if this is a swarm control message (PAUSE/RESUME/OVERRIDE).
func (m *Message) IsControlMessage() bool {
	return m.Type == MsgPause || m.Type == MsgResume || m.Type == MsgHumanOverride
}

// IsFromHuman returns true if this message is from the HUMAN role.
func (m *Message) IsFromHuman() bool {
	return m.From == RoleHuman
}

// WithMetadata adds metadata to the message and returns it for chaining.
func (m *Message) WithMetadata(meta MessageMetadata) *Message {
	m.Metadata = meta
	return m
}

// WithFiles adds file references to the message metadata.
func (m *Message) WithFiles(files ...string) *Message {
	m.Metadata.Files = append(m.Metadata.Files, files...)
	return m
}

// WithCodeBlock adds a code block to the message metadata.
func (m *Message) WithCodeBlock(language, code string, filename string) *Message {
	m.Metadata.CodeBlocks = append(m.Metadata.CodeBlocks, CodeBlock{
		Language: language,
		Code:     code,
		Filename: filename,
	})
	return m
}

// WithPriority sets the message priority.
func (m *Message) WithPriority(p Priority) *Message {
	m.Metadata.Priority = p
	return m
}

// WithTaskID associates the message with a task.
func (m *Message) WithTaskID(taskID string) *Message {
	m.Metadata.TaskID = taskID
	return m
}

// IsBroadcast returns true if this is a broadcast message.
func (m *Message) IsBroadcast() bool {
	return m.To == RoleAll || m.Type == MsgBroadcast
}

// IsDirectMessage returns true if this is a direct message to a specific role.
func (m *Message) IsDirectMessage() bool {
	return !m.IsBroadcast() && m.To != ""
}

// IsToRole returns true if this message is addressed to the given role.
func (m *Message) IsToRole(role Role) bool {
	return m.To == role || m.IsBroadcast()
}

// IsFromRole returns true if this message is from the given role.
func (m *Message) IsFromRole(role Role) bool {
	return m.From == role
}

// Encode serializes the message to JSON.
func (m *Message) Encode() ([]byte, error) {
	return json.Marshal(m)
}

// DecodeMessage deserializes a message from JSON.
func DecodeMessage(data []byte) (*Message, error) {
	var msg Message
	if err := json.Unmarshal(data, &msg); err != nil {
		return nil, err
	}
	return &msg, nil
}

// MessageHandler is a function that processes incoming messages.
type MessageHandler func(msg *Message)

// MessageFilter is a function that determines if a message should be processed.
type MessageFilter func(msg *Message) bool

// FilterByType returns a filter that matches messages of the given type.
func FilterByType(msgType MessageType) MessageFilter {
	return func(msg *Message) bool {
		return msg.Type == msgType
	}
}

// FilterByFrom returns a filter that matches messages from the given role.
func FilterByFrom(role Role) MessageFilter {
	return func(msg *Message) bool {
		return msg.From == role
	}
}

// FilterByTo returns a filter that matches messages to the given role (including broadcasts).
func FilterByTo(role Role) MessageFilter {
	return func(msg *Message) bool {
		return msg.IsToRole(role)
	}
}

// FilterDirectOnly returns a filter that matches only direct messages.
func FilterDirectOnly() MessageFilter {
	return func(msg *Message) bool {
		return msg.IsDirectMessage()
	}
}
