package swarm

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/nats-io/nats.go"
)

// NATSConfig contains NATS connection configuration.
type NATSConfig struct {
	URL            string        `json:"url" yaml:"url"`
	CredsFile      string        `json:"creds_file,omitempty" yaml:"creds_file,omitempty"`
	Token          string        `json:"token,omitempty" yaml:"token,omitempty"`
	ConnectTimeout time.Duration `json:"connect_timeout,omitempty" yaml:"connect_timeout,omitempty"`
	ReconnectWait  time.Duration `json:"reconnect_wait,omitempty" yaml:"reconnect_wait,omitempty"`
	MaxReconnects  int           `json:"max_reconnects,omitempty" yaml:"max_reconnects,omitempty"`
}

// DefaultNATSConfig returns the default NATS configuration.
func DefaultNATSConfig() NATSConfig {
	return NATSConfig{
		URL:            nats.DefaultURL, // "nats://localhost:4222"
		ConnectTimeout: 5 * time.Second,
		ReconnectWait:  2 * time.Second,
		MaxReconnects:  60,
	}
}

// NATSSwarm manages NATS connections for a swarm session.
type NATSSwarm struct {
	conn     *nats.Conn
	roomID   string
	role     Role
	config   NATSConfig
	presence *PresenceTracker

	// Subscriptions
	subs     map[string]*nats.Subscription
	handlers []MessageHandler
	mu       sync.RWMutex

	// Event channels
	messages    chan *Message
	presenceC   chan *PresenceEvent
	errors      chan error
	connections chan *ConnectionEvent

	// Connection state
	state ConnectionState

	// Context for graceful shutdown
	ctx    context.Context
	cancel context.CancelFunc
}

// NewNATSSwarm creates a new NATS swarm connection.
func NewNATSSwarm(config NATSConfig) *NATSSwarm {
	ctx, cancel := context.WithCancel(context.Background())
	return &NATSSwarm{
		config:      config,
		subs:        make(map[string]*nats.Subscription),
		handlers:    make([]MessageHandler, 0),
		messages:    make(chan *Message, 100),
		presenceC:   make(chan *PresenceEvent, 50),
		errors:      make(chan error, 10),
		connections: make(chan *ConnectionEvent, 10),
		state:       StateDisconnected,
		ctx:         ctx,
		cancel:      cancel,
	}
}

// Connect establishes a connection to the NATS server.
func (n *NATSSwarm) Connect() error {
	n.setState(StateConnecting)
	n.emitConnectionEvent(StateConnecting, nil, "Connecting to NATS server...")

	opts := []nats.Option{
		nats.Name(fmt.Sprintf("zcode-swarm-%s", n.role)),
		nats.Timeout(n.config.ConnectTimeout),
		nats.ReconnectWait(n.config.ReconnectWait),
		nats.MaxReconnects(n.config.MaxReconnects),
		nats.DisconnectErrHandler(func(nc *nats.Conn, err error) {
			n.setState(StateReconnecting)
			if err != nil {
				n.errors <- fmt.Errorf("disconnected: %w", err)
				n.emitConnectionEvent(StateReconnecting, err, "Connection lost, attempting to reconnect...")
			} else {
				n.emitConnectionEvent(StateReconnecting, nil, "Connection lost, attempting to reconnect...")
			}
		}),
		nats.ReconnectHandler(func(nc *nats.Conn) {
			n.setState(StateConnected)
			n.emitConnectionEvent(StateConnected, nil, "Reconnected to NATS server")
			// Re-announce presence on reconnect
			if n.roomID != "" && n.role != "" {
				n.announcePresence(PresenceOnline)
			}
		}),
		nats.ClosedHandler(func(nc *nats.Conn) {
			n.setState(StateClosed)
			if nc.LastError() != nil {
				n.emitConnectionEvent(StateClosed, nc.LastError(), "Connection closed")
			} else {
				n.emitConnectionEvent(StateClosed, nil, "Connection closed")
			}
		}),
		nats.ErrorHandler(func(nc *nats.Conn, sub *nats.Subscription, err error) {
			n.errors <- fmt.Errorf("nats error: %w", err)
		}),
	}

	if n.config.CredsFile != "" {
		opts = append(opts, nats.UserCredentials(n.config.CredsFile))
	}

	if n.config.Token != "" {
		opts = append(opts, nats.Token(n.config.Token))
	}

	conn, err := nats.Connect(n.config.URL, opts...)
	if err != nil {
		n.setState(StateDisconnected)
		n.emitConnectionEvent(StateDisconnected, err, "Failed to connect to NATS server")
		return fmt.Errorf("%w: %s", ErrConnectionFailed, err)
	}

	n.conn = conn
	n.setState(StateConnected)
	n.emitConnectionEvent(StateConnected, nil, "Connected to NATS server")
	return nil
}

// setState updates the connection state safely.
func (n *NATSSwarm) setState(state ConnectionState) {
	n.mu.Lock()
	n.state = state
	n.mu.Unlock()
}

// State returns the current connection state.
func (n *NATSSwarm) State() ConnectionState {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return n.state
}

// emitConnectionEvent sends a connection event to the channel.
func (n *NATSSwarm) emitConnectionEvent(state ConnectionState, err error, message string) {
	// Check if we're shutting down
	select {
	case <-n.ctx.Done():
		return // Don't emit events after shutdown
	default:
	}

	event := &ConnectionEvent{
		State:   state,
		Error:   err,
		Message: message,
	}
	select {
	case n.connections <- event:
	case <-n.ctx.Done():
		// Shutting down, don't block
	default:
		// Channel full, drop event
	}
}

// ConnectionEvents returns the channel for connection state changes.
func (n *NATSSwarm) ConnectionEvents() <-chan *ConnectionEvent {
	return n.connections
}

// JoinRoom joins a room with the given role.
func (n *NATSSwarm) JoinRoom(roomID string, role Role) error {
	if n.conn == nil {
		return ErrNotConnected
	}

	n.mu.Lock()
	n.roomID = roomID
	n.role = role
	n.presence = NewPresenceTracker(roomID)
	n.mu.Unlock()

	// Subscribe to relevant subjects
	if err := n.subscribe(); err != nil {
		return err
	}

	// Announce our presence
	n.announcePresence(PresenceOnline)

	return nil
}

// subscribe sets up all necessary subscriptions.
func (n *NATSSwarm) subscribe() error {
	// Subscribe to direct messages for our role
	directSubject := n.subjectForRole(n.role)
	directSub, err := n.conn.Subscribe(directSubject, func(msg *nats.Msg) {
		n.handleIncomingMessage(msg)
	})
	if err != nil {
		return fmt.Errorf("failed to subscribe to direct messages: %w", err)
	}
	n.subs["direct"] = directSub

	// Subscribe to broadcasts
	broadcastSubject := n.subjectBroadcast()
	broadcastSub, err := n.conn.Subscribe(broadcastSubject, func(msg *nats.Msg) {
		n.handleIncomingMessage(msg)
	})
	if err != nil {
		return fmt.Errorf("failed to subscribe to broadcasts: %w", err)
	}
	n.subs["broadcast"] = broadcastSub

	// Subscribe to presence updates
	presenceSubject := n.subjectPresence()
	presenceSub, err := n.conn.Subscribe(presenceSubject, func(msg *nats.Msg) {
		n.handlePresenceUpdate(msg)
	})
	if err != nil {
		return fmt.Errorf("failed to subscribe to presence: %w", err)
	}
	n.subs["presence"] = presenceSub

	return nil
}

// Subject formatting helpers
func (n *NATSSwarm) subjectForRole(role Role) string {
	return fmt.Sprintf("room.%s.%s", n.roomID, role)
}

func (n *NATSSwarm) subjectBroadcast() string {
	return fmt.Sprintf("room.%s.broadcast", n.roomID)
}

func (n *NATSSwarm) subjectPresence() string {
	return fmt.Sprintf("room.%s.presence", n.roomID)
}

func (n *NATSSwarm) subjectState() string {
	return fmt.Sprintf("room.%s.state", n.roomID)
}

// handleIncomingMessage processes an incoming message.
func (n *NATSSwarm) handleIncomingMessage(msg *nats.Msg) {
	swarmMsg, err := DecodeMessage(msg.Data)
	if err != nil {
		n.errors <- fmt.Errorf("failed to decode message: %w", err)
		return
	}

	// Send to message channel for processing
	select {
	case n.messages <- swarmMsg:
	default:
		n.errors <- fmt.Errorf("message channel full, dropping message")
	}

	// Call registered handlers
	n.mu.RLock()
	handlers := make([]MessageHandler, len(n.handlers))
	copy(handlers, n.handlers)
	n.mu.RUnlock()

	for _, handler := range handlers {
		go handler(swarmMsg)
	}
}

// handlePresenceUpdate processes a presence update.
func (n *NATSSwarm) handlePresenceUpdate(msg *nats.Msg) {
	event, err := DecodePresenceEvent(msg.Data)
	if err != nil {
		n.errors <- fmt.Errorf("failed to decode presence: %w", err)
		return
	}

	// Update our presence tracker
	if n.presence != nil {
		n.presence.Update(event)
	}

	// Send to presence channel
	select {
	case n.presenceC <- event:
	default:
		// Channel full, drop event
	}
}

// Send sends a message to a specific role or broadcast.
func (n *NATSSwarm) Send(msg *Message) error {
	if n.conn == nil {
		return ErrNotConnected
	}

	// Set from role if not set
	if msg.From == "" {
		msg.From = n.role
	}

	// Set room ID if not set
	if msg.RoomID == "" {
		msg.RoomID = n.roomID
	}

	data, err := msg.Encode()
	if err != nil {
		return fmt.Errorf("failed to encode message: %w", err)
	}

	// Determine subject based on target
	var subject string
	if msg.IsBroadcast() {
		subject = n.subjectBroadcast()
	} else {
		subject = n.subjectForRole(msg.To)
	}

	if err := n.conn.Publish(subject, data); err != nil {
		return fmt.Errorf("%w: %s", ErrMessageSendFailed, err)
	}

	return nil
}

// Broadcast sends a message to all agents in the room.
func (n *NATSSwarm) Broadcast(content string, msgType MessageType) error {
	msg := &Message{
		RoomID:    n.roomID,
		From:      n.role,
		To:        RoleAll,
		Type:      msgType,
		Content:   content,
		Timestamp: time.Now(),
	}
	return n.Send(msg)
}

// SendTo sends a direct message to a specific role.
func (n *NATSSwarm) SendTo(to Role, content string, msgType MessageType) error {
	msg := NewMessage(n.roomID, n.role, to, msgType, content)
	return n.Send(msg)
}

// Request sends a request to a role and returns the message ID.
func (n *NATSSwarm) Request(to Role, content string) (string, error) {
	msg := NewRequest(n.roomID, n.role, to, content)
	if err := n.Send(msg); err != nil {
		return "", err
	}
	return msg.ID, nil
}

// Reply sends a response to a previous message.
func (n *NATSSwarm) Reply(to Role, content string, replyToID string) error {
	msg := NewResponse(n.roomID, n.role, to, content, replyToID)
	return n.Send(msg)
}

// announcePresence publishes a presence update.
func (n *NATSSwarm) announcePresence(status PresenceStatus) error {
	if n.conn == nil {
		return ErrNotConnected
	}

	event := NewPresenceEvent(n.roomID, n.role, status, "")
	data, err := event.Encode()
	if err != nil {
		return err
	}

	return n.conn.Publish(n.subjectPresence(), data)
}

// SetStatus updates our presence status.
func (n *NATSSwarm) SetStatus(status PresenceStatus) error {
	return n.announcePresence(status)
}

// OnMessage registers a handler for incoming messages.
func (n *NATSSwarm) OnMessage(handler MessageHandler) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.handlers = append(n.handlers, handler)
}

// Messages returns the channel of incoming messages.
func (n *NATSSwarm) Messages() <-chan *Message {
	return n.messages
}

// Presence returns the channel of presence updates.
func (n *NATSSwarm) Presence() <-chan *PresenceEvent {
	return n.presenceC
}

// Errors returns the channel of errors.
func (n *NATSSwarm) Errors() <-chan error {
	return n.errors
}

// GetPresence returns the presence tracker.
func (n *NATSSwarm) GetPresence() *PresenceTracker {
	return n.presence
}

// LeaveRoom leaves the current room.
func (n *NATSSwarm) LeaveRoom() error {
	if n.roomID == "" {
		return ErrNotConnected
	}

	// Announce we're leaving
	n.announcePresence(PresenceOffline)

	// Unsubscribe from all
	n.mu.Lock()
	for _, sub := range n.subs {
		sub.Unsubscribe()
	}
	n.subs = make(map[string]*nats.Subscription)
	n.roomID = ""
	n.role = ""
	n.presence = nil
	n.mu.Unlock()

	return nil
}

// Close closes the NATS connection.
func (n *NATSSwarm) Close() error {
	n.cancel() // Cancel context

	if n.roomID != "" {
		n.LeaveRoom()
	}

	if n.conn != nil {
		n.conn.Close()
		n.conn = nil
	}

	n.setState(StateClosed)
	close(n.messages)
	close(n.presenceC)
	close(n.errors)
	close(n.connections)

	return nil
}

// IsConnected returns true if connected to NATS.
func (n *NATSSwarm) IsConnected() bool {
	return n.conn != nil && n.conn.IsConnected()
}

// RoomID returns the current room ID.
func (n *NATSSwarm) RoomID() string {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return n.roomID
}

// Role returns the current role.
func (n *NATSSwarm) Role() Role {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return n.role
}

// RequestRoomState requests the current room state from the orchestrator.
func (n *NATSSwarm) RequestRoomState() error {
	if n.conn == nil {
		return ErrNotConnected
	}

	request := map[string]string{
		"type":    "state_request",
		"from":    string(n.role),
		"room_id": n.roomID,
	}
	data, _ := json.Marshal(request)

	return n.conn.Publish(n.subjectState(), data)
}

// PublishRoomState publishes room state (for orchestrator use).
func (n *NATSSwarm) PublishRoomState(room *Room) error {
	if n.conn == nil {
		return ErrNotConnected
	}

	info := room.Info()
	data, err := json.Marshal(info)
	if err != nil {
		return err
	}

	return n.conn.Publish(n.subjectState(), data)
}

// StartHeartbeat starts sending periodic heartbeats.
func (n *NATSSwarm) StartHeartbeat(interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-n.ctx.Done():
				return
			case <-ticker.C:
				n.announcePresence(PresenceOnline)
			}
		}
	}()
}
