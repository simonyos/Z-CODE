package swarm

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"sync"
)

// SwarmHandler handles incoming swarm messages and coordinates with the LLM.
type SwarmHandler struct {
	role       Role
	definition *RoleDefinition
	nats       *NATSSwarm
	room       *Room

	// Message history for context
	history   []*Message
	historyMu sync.RWMutex

	// Pending requests awaiting response
	pendingRequests map[string]chan *Message
	pendingMu       sync.RWMutex

	// Configuration
	maxHistory int
}

// NewSwarmHandler creates a new swarm handler.
func NewSwarmHandler(role Role, nats *NATSSwarm, room *Room) *SwarmHandler {
	roles := DefaultRoles()
	def := roles[role]

	return &SwarmHandler{
		role:            role,
		definition:      def,
		nats:            nats,
		room:            room,
		history:         make([]*Message, 0),
		pendingRequests: make(map[string]chan *Message),
		maxHistory:      100,
	}
}

// Start begins processing incoming messages.
// Note: When using the TUI, don't call Start() as the TUI handles message reading.
// This is intended for headless/programmatic use.
func (h *SwarmHandler) Start(ctx context.Context) {
	go h.processMessages(ctx)
	go h.processPresence(ctx)
}

// StartPresenceOnly starts only presence processing (for TUI mode where messages are handled separately).
func (h *SwarmHandler) StartPresenceOnly(ctx context.Context) {
	// Don't start message processing - TUI will handle that
	// Just let presence be tracked by NATSSwarm automatically
}

// processMessages handles incoming messages from NATS.
func (h *SwarmHandler) processMessages(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-h.nats.Messages():
			if !ok {
				return
			}
			h.handleMessage(msg)
		}
	}
}

// processPresence handles presence updates.
func (h *SwarmHandler) processPresence(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case _, ok := <-h.nats.Presence():
			if !ok {
				return
			}
			// Presence is automatically tracked by NATSSwarm
			// Additional handling can be added here
		}
	}
}

// handleMessage processes an incoming message.
func (h *SwarmHandler) handleMessage(msg *Message) {
	// Add to history
	h.addToHistory(msg)

	// Check if this is a response to a pending request
	if msg.ReplyTo != "" {
		h.pendingMu.RLock()
		ch, exists := h.pendingRequests[msg.ReplyTo]
		h.pendingMu.RUnlock()

		if exists {
			select {
			case ch <- msg:
			default:
			}
		}
	}
}

// addToHistory adds a message to the history.
func (h *SwarmHandler) addToHistory(msg *Message) {
	h.historyMu.Lock()
	defer h.historyMu.Unlock()

	h.history = append(h.history, msg)

	// Trim if too long
	if len(h.history) > h.maxHistory {
		h.history = h.history[len(h.history)-h.maxHistory:]
	}
}

// GetHistory returns recent message history.
func (h *SwarmHandler) GetHistory(limit int) []*Message {
	h.historyMu.RLock()
	defer h.historyMu.RUnlock()

	if limit <= 0 || limit > len(h.history) {
		limit = len(h.history)
	}

	start := len(h.history) - limit
	if start < 0 {
		start = 0
	}

	result := make([]*Message, limit)
	copy(result, h.history[start:])
	return result
}

// Send sends a message through the handler.
func (h *SwarmHandler) Send(msg *Message) error {
	// Add to our history
	h.addToHistory(msg)

	// Send via NATS
	return h.nats.Send(msg)
}

// Broadcast sends a broadcast message.
func (h *SwarmHandler) Broadcast(content string) error {
	msg := NewBroadcast(h.room.ID, h.role, content)
	return h.Send(msg)
}

// SendTo sends a direct message to a specific role.
func (h *SwarmHandler) SendTo(to Role, content string) error {
	msg := NewRequest(h.room.ID, h.role, to, content)
	return h.Send(msg)
}

// Request sends a request and waits for a response.
func (h *SwarmHandler) Request(ctx context.Context, to Role, content string) (*Message, error) {
	msg := NewRequest(h.room.ID, h.role, to, content)

	// Create response channel
	respCh := make(chan *Message, 1)
	h.pendingMu.Lock()
	h.pendingRequests[msg.ID] = respCh
	h.pendingMu.Unlock()

	defer func() {
		h.pendingMu.Lock()
		delete(h.pendingRequests, msg.ID)
		h.pendingMu.Unlock()
		close(respCh)
	}()

	// Send the request
	if err := h.Send(msg); err != nil {
		return nil, err
	}

	// Wait for response
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case resp := <-respCh:
		return resp, nil
	}
}

// Reply sends a response to a previous message.
func (h *SwarmHandler) Reply(to Role, content string, replyToID string) error {
	msg := NewResponse(h.room.ID, h.role, to, content, replyToID)
	return h.Send(msg)
}

// GetRoleDefinition returns the role definition.
func (h *SwarmHandler) GetRoleDefinition() *RoleDefinition {
	return h.definition
}

// GetSystemPrompt returns the system prompt for this role.
func (h *SwarmHandler) GetSystemPrompt() string {
	if h.definition != nil {
		return h.definition.SystemPrompt
	}
	return ""
}

// BuildContext builds context string from recent messages for LLM.
func (h *SwarmHandler) BuildContext() string {
	messages := h.GetHistory(20) // Last 20 messages for context

	var sb strings.Builder
	sb.WriteString("## Recent Swarm Activity\n\n")

	for _, msg := range messages {
		sb.WriteString(fmt.Sprintf("[%s] %s → %s: %s\n",
			msg.Timestamp.Format("15:04"),
			msg.From,
			msg.To,
			truncate(msg.Content, 200),
		))
	}

	return sb.String()
}

// truncate shortens a string to the given length.
func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}

// ParseMention extracts @ROLE mentions from content.
func ParseMention(content string) (Role, string) {
	// Pattern: @ROLE_NAME at the start of the message
	re := regexp.MustCompile(`^@(\w+)\s+(.*)$`)
	matches := re.FindStringSubmatch(strings.TrimSpace(content))

	if len(matches) == 3 {
		roleStr := strings.ToUpper(matches[1])
		role, err := ParseRole(roleStr)
		if err == nil {
			return role, matches[2]
		}
	}

	return "", content
}

// ExtractMentions extracts all @ROLE mentions from content.
func ExtractMentions(content string) []Role {
	re := regexp.MustCompile(`@(\w+)`)
	matches := re.FindAllStringSubmatch(content, -1)

	roles := make([]Role, 0)
	seen := make(map[Role]bool)

	for _, match := range matches {
		if len(match) == 2 {
			roleStr := strings.ToUpper(match[1])
			role, err := ParseRole(roleStr)
			if err == nil && !seen[role] {
				roles = append(roles, role)
				seen[role] = true
			}
		}
	}

	return roles
}

// MessageClassifier helps determine what action to take on a message.
type MessageClassifier struct{}

// ClassifyContent analyzes content to determine intent.
func (mc *MessageClassifier) ClassifyContent(content string) (MessageType, Role) {
	content = strings.TrimSpace(content)
	upper := strings.ToUpper(content)

	// Check for approval/rejection patterns
	if strings.HasPrefix(upper, "APPROVE:") || strings.HasPrefix(upper, "APPROVED:") {
		return MsgApproval, ""
	}
	if strings.HasPrefix(upper, "REJECT:") || strings.HasPrefix(upper, "REJECTED:") {
		return MsgRejection, ""
	}

	// Check for status updates
	if strings.HasPrefix(upper, "STATUS:") || strings.HasPrefix(upper, "UPDATE:") {
		return MsgStatus, ""
	}

	// Check for review requests
	if strings.Contains(upper, "PLEASE REVIEW") || strings.Contains(upper, "READY FOR REVIEW") {
		return MsgReviewRequest, ""
	}

	// Check for mentions
	target, _ := ParseMention(content)
	if target != "" {
		return MsgRequest, target
	}

	// Default to broadcast
	return MsgBroadcast, RoleAll
}

// SwarmTools provides tool implementations for swarm communication.
type SwarmTools struct {
	handler *SwarmHandler
}

// NewSwarmTools creates tools for swarm interaction.
func NewSwarmTools(handler *SwarmHandler) *SwarmTools {
	return &SwarmTools{handler: handler}
}

// AskAgent asks another agent a question.
func (st *SwarmTools) AskAgent(target Role, question string) error {
	return st.handler.SendTo(target, question)
}

// BroadcastMessage sends a message to all agents.
func (st *SwarmTools) BroadcastMessage(content string) error {
	return st.handler.Broadcast(content)
}

// ReportStatus reports status to the orchestrator.
func (st *SwarmTools) ReportStatus(status string) error {
	msg := NewStatus(st.handler.room.ID, st.handler.role, status)
	return st.handler.Send(msg)
}

// RequestReview asks for a code review.
func (st *SwarmTools) RequestReview(reviewer Role, details string) error {
	msg := NewMessage(st.handler.room.ID, st.handler.role, reviewer, MsgReviewRequest, details)
	return st.handler.Send(msg)
}

// Approve approves a proposal.
func (st *SwarmTools) Approve(target Role, message string, replyTo string) error {
	msg := NewApproval(st.handler.room.ID, st.handler.role, target, message, replyTo)
	return st.handler.Send(msg)
}

// Reject rejects a proposal with feedback.
func (st *SwarmTools) Reject(target Role, message string, replyTo string) error {
	msg := NewRejection(st.handler.room.ID, st.handler.role, target, message, replyTo)
	return st.handler.Send(msg)
}
