package swarm

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// PersistenceConfig holds configuration for persistence.
type PersistenceConfig struct {
	// Directory to store persistence files
	DataDir string `json:"data_dir" yaml:"data_dir"`
	// Whether to enable persistence
	Enabled bool `json:"enabled" yaml:"enabled"`
	// Maximum number of messages to persist per room
	MaxMessages int `json:"max_messages" yaml:"max_messages"`
}

// DefaultPersistenceConfig returns the default persistence configuration.
func DefaultPersistenceConfig() PersistenceConfig {
	homeDir, _ := os.UserHomeDir()
	return PersistenceConfig{
		DataDir:     filepath.Join(homeDir, ".zcode", "swarm"),
		Enabled:     true,
		MaxMessages: 1000,
	}
}

// PersistedRoom represents the persisted state of a room.
type PersistedRoom struct {
	RoomID    string          `json:"room_id"`
	Name      string          `json:"name"`
	Code      string          `json:"code"`
	CreatedAt time.Time       `json:"created_at"`
	UpdatedAt time.Time       `json:"updated_at"`
	Messages  []*Message      `json:"messages"`
	Agents    []AgentState    `json:"agents"`
	Metadata  RoomMetadata    `json:"metadata"`
}

// AgentState represents the persisted state of an agent in a room.
type AgentState struct {
	Role       Role            `json:"role"`
	SessionID  string          `json:"session_id"`
	JoinedAt   time.Time       `json:"joined_at"`
	LastSeenAt time.Time       `json:"last_seen_at"`
	Status     PresenceStatus  `json:"status"`
}

// RoomMetadata holds additional room metadata.
type RoomMetadata struct {
	Description string            `json:"description,omitempty"`
	Tags        []string          `json:"tags,omitempty"`
	Custom      map[string]string `json:"custom,omitempty"`
}

// Persistence manages persistent storage for swarm data.
type Persistence struct {
	config PersistenceConfig
	mu     sync.RWMutex
}

// NewPersistence creates a new persistence manager.
func NewPersistence(config PersistenceConfig) (*Persistence, error) {
	p := &Persistence{
		config: config,
	}

	if config.Enabled {
		// Create data directory if it doesn't exist
		if err := os.MkdirAll(config.DataDir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create data directory: %w", err)
		}
	}

	return p, nil
}

// SaveRoom saves the current room state to disk.
func (p *Persistence) SaveRoom(room *Room, messages []*Message) error {
	if !p.config.Enabled {
		return nil
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	// Build agent states
	var agents []AgentState
	room.mu.RLock()
	for role, agent := range room.Agents {
		agents = append(agents, AgentState{
			Role:       role,
			SessionID:  agent.SessionID,
			JoinedAt:   agent.JoinedAt,
			LastSeenAt: time.Now(),
			Status:     PresenceOnline,
		})
	}
	room.mu.RUnlock()

	// Truncate messages if necessary
	if len(messages) > p.config.MaxMessages {
		messages = messages[len(messages)-p.config.MaxMessages:]
	}

	state := PersistedRoom{
		RoomID:    room.ID,
		Name:      room.Name,
		Code:      room.Code,
		CreatedAt: room.CreatedAt,
		UpdatedAt: time.Now(),
		Messages:  messages,
		Agents:    agents,
	}

	// Write to file
	filename := p.roomFilePath(room.Code)
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal room state: %w", err)
	}

	if err := os.WriteFile(filename, data, 0644); err != nil {
		return fmt.Errorf("failed to write room state: %w", err)
	}

	return nil
}

// LoadRoom loads a room state from disk.
func (p *Persistence) LoadRoom(roomCode string) (*PersistedRoom, error) {
	if !p.config.Enabled {
		return nil, fmt.Errorf("persistence is disabled")
	}

	p.mu.RLock()
	defer p.mu.RUnlock()

	filename := p.roomFilePath(roomCode)
	data, err := os.ReadFile(filename)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrRoomNotFound
		}
		return nil, fmt.Errorf("failed to read room state: %w", err)
	}

	var state PersistedRoom
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("failed to unmarshal room state: %w", err)
	}

	return &state, nil
}

// DeleteRoom removes a room's persisted state.
func (p *Persistence) DeleteRoom(roomCode string) error {
	if !p.config.Enabled {
		return nil
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	filename := p.roomFilePath(roomCode)
	if err := os.Remove(filename); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete room state: %w", err)
	}

	return nil
}

// ListRooms returns a list of all persisted room codes.
func (p *Persistence) ListRooms() ([]string, error) {
	if !p.config.Enabled {
		return nil, nil
	}

	p.mu.RLock()
	defer p.mu.RUnlock()

	entries, err := os.ReadDir(p.config.DataDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read data directory: %w", err)
	}

	var rooms []string
	for _, entry := range entries {
		if !entry.IsDir() && filepath.Ext(entry.Name()) == ".json" {
			roomCode := entry.Name()[:len(entry.Name())-5] // Remove .json extension
			rooms = append(rooms, roomCode)
		}
	}

	return rooms, nil
}

// AppendMessage appends a message to a room's history without loading the full state.
func (p *Persistence) AppendMessage(roomCode string, msg *Message) error {
	if !p.config.Enabled {
		return nil
	}

	// Load existing state
	state, err := p.LoadRoom(roomCode)
	if err != nil {
		if err == ErrRoomNotFound {
			// Create new state with just this message
			state = &PersistedRoom{
				RoomID:    roomCode,
				Code:      roomCode,
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
				Messages:  []*Message{msg},
			}
		} else {
			return err
		}
	} else {
		// Append message
		state.Messages = append(state.Messages, msg)
		state.UpdatedAt = time.Now()

		// Truncate if necessary
		if len(state.Messages) > p.config.MaxMessages {
			state.Messages = state.Messages[len(state.Messages)-p.config.MaxMessages:]
		}
	}

	// Save back
	p.mu.Lock()
	defer p.mu.Unlock()

	filename := p.roomFilePath(roomCode)
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal room state: %w", err)
	}

	if err := os.WriteFile(filename, data, 0644); err != nil {
		return fmt.Errorf("failed to write room state: %w", err)
	}

	return nil
}

// GetRoomInfo returns basic info about a room without loading all messages.
func (p *Persistence) GetRoomInfo(roomCode string) (*PersistedRoom, error) {
	state, err := p.LoadRoom(roomCode)
	if err != nil {
		return nil, err
	}

	// Return state without messages for quick access
	info := &PersistedRoom{
		RoomID:    state.RoomID,
		Name:      state.Name,
		Code:      state.Code,
		CreatedAt: state.CreatedAt,
		UpdatedAt: state.UpdatedAt,
		Agents:    state.Agents,
		Metadata:  state.Metadata,
	}

	return info, nil
}

// GetRecentMessages returns the most recent N messages from a room.
func (p *Persistence) GetRecentMessages(roomCode string, limit int) ([]*Message, error) {
	state, err := p.LoadRoom(roomCode)
	if err != nil {
		return nil, err
	}

	if limit <= 0 || limit > len(state.Messages) {
		return state.Messages, nil
	}

	return state.Messages[len(state.Messages)-limit:], nil
}

// roomFilePath returns the file path for a room's state file.
func (p *Persistence) roomFilePath(roomCode string) string {
	return filepath.Join(p.config.DataDir, roomCode+".json")
}

// IsEnabled returns whether persistence is enabled.
func (p *Persistence) IsEnabled() bool {
	return p.config.Enabled
}

// DataDir returns the data directory path.
func (p *Persistence) DataDir() string {
	return p.config.DataDir
}
