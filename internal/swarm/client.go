package swarm

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// Client is the main entry point for swarm functionality.
// It manages the connection, room, and message handling.
type Client struct {
	config      *SwarmConfig
	nats        *NATSSwarm
	room        *Room
	handler     *SwarmHandler
	persistence *Persistence

	role      Role
	sessionID string

	// Event callbacks
	onMessage  func(*Message)
	onPresence func(*PresenceEvent)
	onError    func(error)

	// State
	connected bool
	mu        sync.RWMutex

	// Context for lifecycle management
	ctx    context.Context
	cancel context.CancelFunc
}

// NewClient creates a new swarm client.
func NewClient(config *SwarmConfig) *Client {
	if config == nil {
		config = DefaultSwarmConfig()
	}

	ctx, cancel := context.WithCancel(context.Background())

	// Initialize persistence (errors are non-fatal)
	persistence, _ := NewPersistence(DefaultPersistenceConfig())

	return &Client{
		config:      config,
		persistence: persistence,
		sessionID:   generateRoomID()[:8], // Short session ID
		ctx:         ctx,
		cancel:      cancel,
	}
}

// NewClientWithPersistence creates a new swarm client with custom persistence config.
func NewClientWithPersistence(config *SwarmConfig, persistConfig PersistenceConfig) (*Client, error) {
	if config == nil {
		config = DefaultSwarmConfig()
	}

	ctx, cancel := context.WithCancel(context.Background())

	persistence, err := NewPersistence(persistConfig)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to initialize persistence: %w", err)
	}

	return &Client{
		config:      config,
		persistence: persistence,
		sessionID:   generateRoomID()[:8],
		ctx:         ctx,
		cancel:      cancel,
	}, nil
}

// Connect establishes connection to the NATS server.
func (c *Client) Connect() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.connected {
		return ErrAlreadyConnected
	}

	c.nats = NewNATSSwarm(c.config.NATS)
	if err := c.nats.Connect(); err != nil {
		return err
	}

	c.connected = true
	return nil
}

// CreateRoom creates a new room and joins as the orchestrator.
func (c *Client) CreateRoom(name string, config RoomConfig) (*Room, error) {
	if !c.IsConnected() {
		if err := c.Connect(); err != nil {
			return nil, err
		}
	}

	// Create room locally
	room := NewRoom(name, RoleOrchestrator, config)

	// Join as orchestrator
	if err := c.joinRoomInternal(room, RoleOrchestrator, false); err != nil {
		return nil, err
	}

	return room, nil
}

// CreateRoomRaw creates a new room without starting background message/presence consumers.
// Use this when the caller (e.g., TUI) will handle channel reading directly.
func (c *Client) CreateRoomRaw(name string, config RoomConfig) (*Room, error) {
	if !c.IsConnected() {
		if err := c.Connect(); err != nil {
			return nil, err
		}
	}

	// Create room locally
	room := NewRoom(name, RoleOrchestrator, config)

	// Join as orchestrator in raw mode
	if err := c.joinRoomInternal(room, RoleOrchestrator, true); err != nil {
		return nil, err
	}

	return room, nil
}

// JoinRoom joins an existing room by code.
func (c *Client) JoinRoom(roomCode string, role Role) error {
	if !c.IsConnected() {
		if err := c.Connect(); err != nil {
			return err
		}
	}

	// For now, we create a placeholder room with the code as ID
	// In production, this would fetch room info from a discovery service
	room := &Room{
		ID:     roomCode,
		Code:   roomCode,
		State:  RoomStateActive,
		Agents: make(map[Role]*Agent),
		Config: DefaultRoomConfig(),
	}

	return c.joinRoomInternal(room, role, false)
}

// JoinRoomRaw joins an existing room without starting background message/presence consumers.
// Use this when the caller (e.g., TUI) will handle channel reading directly.
func (c *Client) JoinRoomRaw(roomCode string, role Role) error {
	if !c.IsConnected() {
		if err := c.Connect(); err != nil {
			return err
		}
	}

	room := &Room{
		ID:     roomCode,
		Code:   roomCode,
		State:  RoomStateActive,
		Agents: make(map[Role]*Agent),
		Config: DefaultRoomConfig(),
	}

	return c.joinRoomInternal(room, role, true)
}

// joinRoomInternal handles the actual joining logic.
// If rawMode is true, it skips starting background message/presence consumers,
// allowing the caller to read directly from the channels.
func (c *Client) joinRoomInternal(room *Room, role Role, rawMode bool) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Add ourselves to the room
	if err := room.AddAgent(role, c.sessionID); err != nil {
		return err
	}

	// Join via NATS
	if err := c.nats.JoinRoom(room.ID, role); err != nil {
		room.RemoveAgent(role)
		return err
	}

	c.room = room
	c.role = role
	c.handler = NewSwarmHandler(role, c.nats, room)

	// Start heartbeat
	c.nats.StartHeartbeat(10 * time.Second)

	if rawMode {
		// In raw mode, don't start background consumers - caller handles channels directly
		return nil
	}

	// Start processing messages (consumes from channels)
	c.handler.Start(c.ctx)

	// Set up internal message routing
	c.nats.OnMessage(func(msg *Message) {
		if c.onMessage != nil {
			c.onMessage(msg)
		}
	})

	// Monitor presence
	go c.monitorPresence()

	// Monitor errors
	go c.monitorErrors()

	return nil
}

// monitorPresence handles presence updates.
func (c *Client) monitorPresence() {
	for {
		select {
		case <-c.ctx.Done():
			return
		case event, ok := <-c.nats.Presence():
			if !ok {
				return
			}
			if c.onPresence != nil {
				c.onPresence(event)
			}
		}
	}
}

// monitorErrors handles errors from NATS.
func (c *Client) monitorErrors() {
	for {
		select {
		case <-c.ctx.Done():
			return
		case err, ok := <-c.nats.Errors():
			if !ok {
				return
			}
			if c.onError != nil {
				c.onError(err)
			}
		}
	}
}

// Send sends a message.
func (c *Client) Send(msg *Message) error {
	if c.handler == nil {
		return ErrNotConnected
	}
	return c.handler.Send(msg)
}

// SendTo sends a direct message to a role.
func (c *Client) SendTo(to Role, content string) error {
	if c.handler == nil {
		return ErrNotConnected
	}
	return c.handler.SendTo(to, content)
}

// Broadcast sends a message to all agents.
func (c *Client) Broadcast(content string) error {
	if c.handler == nil {
		return ErrNotConnected
	}
	return c.handler.Broadcast(content)
}

// Reply responds to a message.
func (c *Client) Reply(to Role, content string, replyToID string) error {
	if c.handler == nil {
		return ErrNotConnected
	}
	return c.handler.Reply(to, content, replyToID)
}

// Request sends a request and waits for response.
func (c *Client) Request(ctx context.Context, to Role, content string) (*Message, error) {
	if c.handler == nil {
		return nil, ErrNotConnected
	}
	return c.handler.Request(ctx, to, content)
}

// SetStatus updates our presence status.
func (c *Client) SetStatus(status PresenceStatus) error {
	if c.nats == nil {
		return ErrNotConnected
	}
	return c.nats.SetStatus(status)
}

// OnMessage sets the callback for incoming messages.
func (c *Client) OnMessage(handler func(*Message)) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.onMessage = handler
}

// OnPresence sets the callback for presence updates.
func (c *Client) OnPresence(handler func(*PresenceEvent)) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.onPresence = handler
}

// OnError sets the callback for errors.
func (c *Client) OnError(handler func(error)) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.onError = handler
}

// GetHistory returns recent message history.
func (c *Client) GetHistory(limit int) []*Message {
	if c.handler == nil {
		return nil
	}
	return c.handler.GetHistory(limit)
}

// GetPresence returns the presence tracker.
func (c *Client) GetPresence() *PresenceTracker {
	if c.nats == nil {
		return nil
	}
	return c.nats.GetPresence()
}

// GetOnlineRoles returns roles that are currently online.
func (c *Client) GetOnlineRoles() []Role {
	presence := c.GetPresence()
	if presence == nil {
		return nil
	}
	return presence.Online()
}

// Messages returns the channel for incoming swarm messages.
func (c *Client) Messages() <-chan *Message {
	if c.nats == nil {
		return nil
	}
	return c.nats.Messages()
}

// PresenceUpdates returns the channel for presence updates.
func (c *Client) PresenceUpdates() <-chan *PresenceEvent {
	if c.nats == nil {
		return nil
	}
	return c.nats.Presence()
}

// ConnectionEvents returns the channel for connection state changes.
func (c *Client) ConnectionEvents() <-chan *ConnectionEvent {
	if c.nats == nil {
		return nil
	}
	return c.nats.ConnectionEvents()
}

// ConnectionState returns the current connection state.
func (c *Client) ConnectionState() ConnectionState {
	if c.nats == nil {
		return StateDisconnected
	}
	return c.nats.State()
}

// Errors returns the channel for errors.
func (c *Client) Errors() <-chan error {
	if c.nats == nil {
		return nil
	}
	return c.nats.Errors()
}

// Room returns the current room.
func (c *Client) Room() *Room {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.room
}

// Role returns our role.
func (c *Client) Role() Role {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.role
}

// RoleDefinition returns our role definition.
func (c *Client) RoleDefinition() *RoleDefinition {
	if c.handler == nil {
		return nil
	}
	return c.handler.GetRoleDefinition()
}

// SystemPrompt returns the system prompt for our role.
func (c *Client) SystemPrompt() string {
	if c.handler == nil {
		return ""
	}
	return c.handler.GetSystemPrompt()
}

// SessionID returns our session ID.
func (c *Client) SessionID() string {
	return c.sessionID
}

// IsConnected returns true if connected to NATS.
func (c *Client) IsConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.connected && c.nats != nil && c.nats.IsConnected()
}

// IsInRoom returns true if we're in a room.
func (c *Client) IsInRoom() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.room != nil
}

// LeaveRoom leaves the current room.
func (c *Client) LeaveRoom() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.room == nil {
		return ErrNotConnected
	}

	// Remove ourselves from room
	c.room.RemoveAgent(c.role)

	// Leave via NATS
	if c.nats != nil {
		c.nats.LeaveRoom()
	}

	c.room = nil
	c.role = ""
	c.handler = nil

	return nil
}

// Close closes the client and all connections.
func (c *Client) Close() error {
	c.cancel() // Cancel context first

	c.mu.Lock()
	defer c.mu.Unlock()

	// Save room state before closing
	if c.room != nil && c.persistence != nil {
		history := c.GetHistory(0) // Get all history
		c.persistence.SaveRoom(c.room, history)
		c.room.RemoveAgent(c.role)
		c.room = nil
	}

	if c.nats != nil {
		c.nats.Close()
		c.nats = nil
	}

	c.connected = false
	c.handler = nil

	return nil
}

// SaveMessage persists a single message to storage.
func (c *Client) SaveMessage(msg *Message) error {
	if c.persistence == nil || c.room == nil {
		return nil
	}
	return c.persistence.AppendMessage(c.room.Code, msg)
}

// LoadRoomHistory loads the persisted history for a room.
func (c *Client) LoadRoomHistory(roomCode string) ([]*Message, error) {
	if c.persistence == nil {
		return nil, nil
	}
	state, err := c.persistence.LoadRoom(roomCode)
	if err != nil {
		return nil, err
	}
	return state.Messages, nil
}

// GetRecentHistory returns the most recent N messages from persisted storage.
func (c *Client) GetRecentHistory(roomCode string, limit int) ([]*Message, error) {
	if c.persistence == nil {
		return nil, nil
	}
	return c.persistence.GetRecentMessages(roomCode, limit)
}

// ListPersistedRooms returns a list of all persisted room codes.
func (c *Client) ListPersistedRooms() ([]string, error) {
	if c.persistence == nil {
		return nil, nil
	}
	return c.persistence.ListRooms()
}

// GetPersistedRoomInfo returns info about a persisted room.
func (c *Client) GetPersistedRoomInfo(roomCode string) (*PersistedRoom, error) {
	if c.persistence == nil {
		return nil, nil
	}
	return c.persistence.GetRoomInfo(roomCode)
}

// IsPersistenceEnabled returns whether persistence is enabled.
func (c *Client) IsPersistenceEnabled() bool {
	return c.persistence != nil && c.persistence.IsEnabled()
}

// SwarmSession represents an active swarm session with convenience methods.
type SwarmSession struct {
	Client *Client
	Tools  *SwarmTools
}

// NewSession creates a new swarm session.
func NewSession(config *SwarmConfig) *SwarmSession {
	client := NewClient(config)
	return &SwarmSession{
		Client: client,
	}
}

// Create creates and joins a new room as orchestrator.
func (s *SwarmSession) Create(name string) error {
	room, err := s.Client.CreateRoom(name, DefaultRoomConfig())
	if err != nil {
		return err
	}
	s.Tools = NewSwarmTools(s.Client.handler)
	fmt.Printf("Room created: %s\n", room.Name)
	fmt.Printf("Room code: %s\n", room.Code)
	return nil
}

// Join joins an existing room.
func (s *SwarmSession) Join(roomCode string, role Role) error {
	if err := s.Client.JoinRoom(roomCode, role); err != nil {
		return err
	}
	s.Tools = NewSwarmTools(s.Client.handler)
	return nil
}

// Close closes the session.
func (s *SwarmSession) Close() error {
	return s.Client.Close()
}
