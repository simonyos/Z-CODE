package swarm

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"sync"
	"time"
)

// RoomState represents the current state of a room.
type RoomState string

const (
	RoomStateActive    RoomState = "active"
	RoomStatePaused    RoomState = "paused"
	RoomStateCompleted RoomState = "completed"
	RoomStateClosed    RoomState = "closed"
)

// RoomConfig contains configuration for a room.
type RoomConfig struct {
	ProjectRepo     string   `json:"project_repo,omitempty" yaml:"project_repo,omitempty"`
	WorkingDir      string   `json:"working_dir,omitempty" yaml:"working_dir,omitempty"`
	AllowedRoles    []Role   `json:"allowed_roles,omitempty" yaml:"allowed_roles,omitempty"`
	RequireApproval bool     `json:"require_approval" yaml:"require_approval"`
	AutoPilot       bool     `json:"auto_pilot" yaml:"auto_pilot"`
	MaxAgents       int      `json:"max_agents,omitempty" yaml:"max_agents,omitempty"`
	HistoryLimit    int      `json:"history_limit,omitempty" yaml:"history_limit,omitempty"`
}

// DefaultRoomConfig returns the default room configuration.
func DefaultRoomConfig() RoomConfig {
	return RoomConfig{
		AllowedRoles:    AllRoles(),
		RequireApproval: false,
		AutoPilot:       true,
		MaxAgents:       10,
		HistoryLimit:    100,
	}
}

// Agent represents an agent connected to a room.
type Agent struct {
	Role      Role      `json:"role"`
	JoinedAt  time.Time `json:"joined_at"`
	LastSeen  time.Time `json:"last_seen"`
	SessionID string    `json:"session_id"`
	Provider  string    `json:"provider,omitempty"` // LLM provider being used
	Model     string    `json:"model,omitempty"`    // LLM model being used
}

// Room represents a collaboration space where agents connect.
type Room struct {
	ID        string            `json:"id"`
	Name      string            `json:"name"`
	Code      string            `json:"code"` // Human-friendly join code
	CreatedAt time.Time         `json:"created_at"`
	CreatedBy Role              `json:"created_by"`
	State     RoomState         `json:"state"`
	Agents    map[Role]*Agent   `json:"agents"`
	Config    RoomConfig        `json:"config"`
	mu        sync.RWMutex
}

// NewRoom creates a new room with the given name and configuration.
func NewRoom(name string, createdBy Role, config RoomConfig) *Room {
	code := generateRoomCode()

	if name == "" {
		name = code
	}

	// Use the code as the ID so that joining agents can find the room
	// by the same ID they use to join
	return &Room{
		ID:        code,
		Name:      name,
		Code:      code,
		CreatedAt: time.Now(),
		CreatedBy: createdBy,
		State:     RoomStateActive,
		Agents:    make(map[Role]*Agent),
		Config:    config,
	}
}

// generateRoomID creates a unique room identifier.
func generateRoomID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// generateRoomCode creates a human-friendly room code like "merry-panda-9k2j".
var adjectives = []string{
	"brave", "calm", "eager", "fancy", "gentle", "happy", "jolly", "keen",
	"lively", "merry", "noble", "proud", "quick", "rapid", "smart", "swift",
	"witty", "zesty", "bright", "clever",
}

var animals = []string{
	"falcon", "tiger", "panda", "eagle", "dolphin", "fox", "hawk", "lion",
	"otter", "raven", "shark", "wolf", "bear", "deer", "lynx", "owl",
	"seal", "swan", "whale", "zebra",
}

func generateRoomCode() string {
	adj := adjectives[randomInt(len(adjectives))]
	animal := animals[randomInt(len(animals))]
	suffix := randomHex(4)
	return fmt.Sprintf("%s-%s-%s", adj, animal, suffix)
}

func randomInt(max int) int {
	b := make([]byte, 1)
	rand.Read(b)
	return int(b[0]) % max
}

func randomHex(n int) string {
	b := make([]byte, n)
	rand.Read(b)
	return hex.EncodeToString(b)[:n]
}

// AddAgent adds an agent to the room.
func (r *Room) AddAgent(role Role, sessionID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.State == RoomStateClosed {
		return ErrRoomClosed
	}

	if _, exists := r.Agents[role]; exists {
		return ErrRoleTaken
	}

	if len(r.Config.AllowedRoles) > 0 {
		allowed := false
		for _, ar := range r.Config.AllowedRoles {
			if ar == role {
				allowed = true
				break
			}
		}
		if !allowed {
			return ErrRoleNotAllowed
		}
	}

	if r.Config.MaxAgents > 0 && len(r.Agents) >= r.Config.MaxAgents {
		return ErrRoomFull
	}

	r.Agents[role] = &Agent{
		Role:      role,
		JoinedAt:  time.Now(),
		LastSeen:  time.Now(),
		SessionID: sessionID,
	}

	return nil
}

// RemoveAgent removes an agent from the room.
func (r *Room) RemoveAgent(role Role) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.Agents[role]; !exists {
		return ErrAgentNotFound
	}

	delete(r.Agents, role)
	return nil
}

// GetAgent returns the agent with the given role.
func (r *Room) GetAgent(role Role) (*Agent, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	agent, exists := r.Agents[role]
	if !exists {
		return nil, ErrAgentNotFound
	}
	return agent, nil
}

// UpdateAgentLastSeen updates the last seen time for an agent.
func (r *Room) UpdateAgentLastSeen(role Role) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	agent, exists := r.Agents[role]
	if !exists {
		return ErrAgentNotFound
	}
	agent.LastSeen = time.Now()
	return nil
}

// ListAgents returns all agents in the room.
func (r *Room) ListAgents() []*Agent {
	r.mu.RLock()
	defer r.mu.RUnlock()

	agents := make([]*Agent, 0, len(r.Agents))
	for _, agent := range r.Agents {
		agents = append(agents, agent)
	}
	return agents
}

// OnlineRoles returns the roles of all currently connected agents.
func (r *Room) OnlineRoles() []Role {
	r.mu.RLock()
	defer r.mu.RUnlock()

	roles := make([]Role, 0, len(r.Agents))
	for role := range r.Agents {
		roles = append(roles, role)
	}
	return roles
}

// HasRole returns true if the given role is occupied in the room.
func (r *Room) HasRole(role Role) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, exists := r.Agents[role]
	return exists
}

// SetState updates the room state.
func (r *Room) SetState(state RoomState) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.State = state
}

// GetState returns the current room state.
func (r *Room) GetState() RoomState {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.State
}

// IsActive returns true if the room is in active state.
func (r *Room) IsActive() bool {
	return r.GetState() == RoomStateActive
}

// Close marks the room as closed.
func (r *Room) Close() {
	r.SetState(RoomStateClosed)
}

// Pause marks the room as paused.
func (r *Room) Pause() {
	r.SetState(RoomStatePaused)
}

// Resume marks the room as active (from paused state).
func (r *Room) Resume() {
	r.SetState(RoomStateActive)
}

// RoomInfo represents serializable room information.
type RoomInfo struct {
	ID          string      `json:"id"`
	Name        string      `json:"name"`
	Code        string      `json:"code"`
	CreatedAt   time.Time   `json:"created_at"`
	CreatedBy   Role        `json:"created_by"`
	State       RoomState   `json:"state"`
	AgentCount  int         `json:"agent_count"`
	OnlineRoles []Role      `json:"online_roles"`
	Config      RoomConfig  `json:"config"`
}

// Info returns serializable room information.
func (r *Room) Info() RoomInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()

	roles := make([]Role, 0, len(r.Agents))
	for role := range r.Agents {
		roles = append(roles, role)
	}

	return RoomInfo{
		ID:          r.ID,
		Name:        r.Name,
		Code:        r.Code,
		CreatedAt:   r.CreatedAt,
		CreatedBy:   r.CreatedBy,
		State:       r.State,
		AgentCount:  len(r.Agents),
		OnlineRoles: roles,
		Config:      r.Config,
	}
}

// RoomManager manages multiple rooms.
type RoomManager struct {
	rooms map[string]*Room // keyed by room ID
	codes map[string]string // code -> room ID lookup
	mu    sync.RWMutex
}

// NewRoomManager creates a new room manager.
func NewRoomManager() *RoomManager {
	return &RoomManager{
		rooms: make(map[string]*Room),
		codes: make(map[string]string),
	}
}

// Create creates a new room.
func (m *RoomManager) Create(name string, createdBy Role, config RoomConfig) (*Room, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	room := NewRoom(name, createdBy, config)
	m.rooms[room.ID] = room
	m.codes[room.Code] = room.ID

	return room, nil
}

// Get retrieves a room by ID.
func (m *RoomManager) Get(roomID string) (*Room, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	room, exists := m.rooms[roomID]
	if !exists {
		return nil, ErrRoomNotFound
	}
	return room, nil
}

// GetByCode retrieves a room by its join code.
func (m *RoomManager) GetByCode(code string) (*Room, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	code = strings.ToLower(code)
	roomID, exists := m.codes[code]
	if !exists {
		return nil, ErrRoomNotFound
	}

	room, exists := m.rooms[roomID]
	if !exists {
		return nil, ErrRoomNotFound
	}
	return room, nil
}

// Delete removes a room.
func (m *RoomManager) Delete(roomID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	room, exists := m.rooms[roomID]
	if !exists {
		return ErrRoomNotFound
	}

	delete(m.codes, room.Code)
	delete(m.rooms, roomID)
	return nil
}

// List returns all rooms.
func (m *RoomManager) List() []*Room {
	m.mu.RLock()
	defer m.mu.RUnlock()

	rooms := make([]*Room, 0, len(m.rooms))
	for _, room := range m.rooms {
		rooms = append(rooms, room)
	}
	return rooms
}

// ListActive returns all active rooms.
func (m *RoomManager) ListActive() []*Room {
	m.mu.RLock()
	defer m.mu.RUnlock()

	rooms := make([]*Room, 0)
	for _, room := range m.rooms {
		if room.IsActive() {
			rooms = append(rooms, room)
		}
	}
	return rooms
}

// Count returns the number of rooms.
func (m *RoomManager) Count() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.rooms)
}
