package swarm

import (
	"encoding/json"
	"sync"
	"time"
)

// PresenceStatus represents an agent's online status.
type PresenceStatus string

const (
	PresenceOnline  PresenceStatus = "online"
	PresenceOffline PresenceStatus = "offline"
	PresenceBusy    PresenceStatus = "busy"    // Agent is processing
	PresenceTyping  PresenceStatus = "typing"  // Agent is composing a response
	PresenceAway    PresenceStatus = "away"    // Connected but inactive
)

// PresenceEvent represents a presence update.
type PresenceEvent struct {
	RoomID    string         `json:"room_id"`
	Role      Role           `json:"role"`
	Status    PresenceStatus `json:"status"`
	Timestamp time.Time      `json:"timestamp"`
	SessionID string         `json:"session_id,omitempty"`
	Message   string         `json:"message,omitempty"` // Optional status message
}

// NewPresenceEvent creates a new presence event.
func NewPresenceEvent(roomID string, role Role, status PresenceStatus, sessionID string) *PresenceEvent {
	return &PresenceEvent{
		RoomID:    roomID,
		Role:      role,
		Status:    status,
		Timestamp: time.Now(),
		SessionID: sessionID,
	}
}

// Encode serializes the presence event to JSON.
func (e *PresenceEvent) Encode() ([]byte, error) {
	return json.Marshal(e)
}

// DecodePresenceEvent deserializes a presence event from JSON.
func DecodePresenceEvent(data []byte) (*PresenceEvent, error) {
	var event PresenceEvent
	if err := json.Unmarshal(data, &event); err != nil {
		return nil, err
	}
	return &event, nil
}

// PresenceInfo contains current presence information for an agent.
type PresenceInfo struct {
	Role      Role           `json:"role"`
	Status    PresenceStatus `json:"status"`
	LastSeen  time.Time      `json:"last_seen"`
	SessionID string         `json:"session_id"`
	Message   string         `json:"message,omitempty"`
}

// PresenceTracker tracks the presence status of agents in a room.
type PresenceTracker struct {
	roomID    string
	presence  map[Role]*PresenceInfo
	listeners []PresenceListener
	mu        sync.RWMutex

	// Heartbeat configuration
	heartbeatInterval time.Duration
	offlineThreshold  time.Duration
}

// PresenceListener is called when presence changes.
type PresenceListener func(event *PresenceEvent)

// NewPresenceTracker creates a new presence tracker for a room.
func NewPresenceTracker(roomID string) *PresenceTracker {
	return &PresenceTracker{
		roomID:            roomID,
		presence:          make(map[Role]*PresenceInfo),
		listeners:         make([]PresenceListener, 0),
		heartbeatInterval: 10 * time.Second,
		offlineThreshold:  30 * time.Second,
	}
}

// AddListener registers a presence change listener.
func (p *PresenceTracker) AddListener(listener PresenceListener) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.listeners = append(p.listeners, listener)
}

// notifyListeners calls all listeners with the event.
func (p *PresenceTracker) notifyListeners(event *PresenceEvent) {
	p.mu.RLock()
	listeners := make([]PresenceListener, len(p.listeners))
	copy(listeners, p.listeners)
	p.mu.RUnlock()

	for _, listener := range listeners {
		go listener(event)
	}
}

// Update processes a presence event and updates the tracker.
func (p *PresenceTracker) Update(event *PresenceEvent) {
	p.mu.Lock()
	defer p.mu.Unlock()

	info, exists := p.presence[event.Role]
	if !exists {
		info = &PresenceInfo{
			Role: event.Role,
		}
		p.presence[event.Role] = info
	}

	info.Status = event.Status
	info.LastSeen = event.Timestamp
	info.SessionID = event.SessionID
	info.Message = event.Message

	// Notify listeners outside of lock
	go p.notifyListeners(event)
}

// SetOnline marks a role as online.
func (p *PresenceTracker) SetOnline(role Role, sessionID string) {
	event := NewPresenceEvent(p.roomID, role, PresenceOnline, sessionID)
	p.Update(event)
}

// SetOffline marks a role as offline.
func (p *PresenceTracker) SetOffline(role Role) {
	p.mu.RLock()
	info, exists := p.presence[role]
	sessionID := ""
	if exists {
		sessionID = info.SessionID
	}
	p.mu.RUnlock()

	event := NewPresenceEvent(p.roomID, role, PresenceOffline, sessionID)
	p.Update(event)
}

// SetBusy marks a role as busy (processing).
func (p *PresenceTracker) SetBusy(role Role) {
	p.mu.RLock()
	info, exists := p.presence[role]
	sessionID := ""
	if exists {
		sessionID = info.SessionID
	}
	p.mu.RUnlock()

	event := NewPresenceEvent(p.roomID, role, PresenceBusy, sessionID)
	p.Update(event)
}

// SetTyping marks a role as typing.
func (p *PresenceTracker) SetTyping(role Role) {
	p.mu.RLock()
	info, exists := p.presence[role]
	sessionID := ""
	if exists {
		sessionID = info.SessionID
	}
	p.mu.RUnlock()

	event := NewPresenceEvent(p.roomID, role, PresenceTyping, sessionID)
	p.Update(event)
}

// Get returns the presence info for a role.
func (p *PresenceTracker) Get(role Role) (*PresenceInfo, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	info, exists := p.presence[role]
	if !exists {
		return nil, false
	}

	// Return a copy to avoid race conditions
	infoCopy := *info
	return &infoCopy, true
}

// GetStatus returns just the status for a role.
func (p *PresenceTracker) GetStatus(role Role) PresenceStatus {
	info, exists := p.Get(role)
	if !exists {
		return PresenceOffline
	}
	return info.Status
}

// GetAll returns presence info for all tracked roles.
func (p *PresenceTracker) GetAll() map[Role]*PresenceInfo {
	p.mu.RLock()
	defer p.mu.RUnlock()

	result := make(map[Role]*PresenceInfo, len(p.presence))
	for role, info := range p.presence {
		infoCopy := *info
		result[role] = &infoCopy
	}
	return result
}

// Online returns all roles that are currently online.
func (p *PresenceTracker) Online() []Role {
	p.mu.RLock()
	defer p.mu.RUnlock()

	roles := make([]Role, 0)
	for role, info := range p.presence {
		if info.Status == PresenceOnline || info.Status == PresenceBusy || info.Status == PresenceTyping {
			roles = append(roles, role)
		}
	}
	return roles
}

// IsOnline returns true if the role is currently online.
func (p *PresenceTracker) IsOnline(role Role) bool {
	status := p.GetStatus(role)
	return status == PresenceOnline || status == PresenceBusy || status == PresenceTyping
}

// Heartbeat updates the last seen time for a role without changing status.
func (p *PresenceTracker) Heartbeat(role Role) {
	p.mu.Lock()
	defer p.mu.Unlock()

	info, exists := p.presence[role]
	if exists {
		info.LastSeen = time.Now()
	}
}

// CheckStale marks roles as offline if they haven't been seen recently.
func (p *PresenceTracker) CheckStale() []Role {
	p.mu.Lock()
	defer p.mu.Unlock()

	stale := make([]Role, 0)
	threshold := time.Now().Add(-p.offlineThreshold)

	for role, info := range p.presence {
		if info.Status != PresenceOffline && info.LastSeen.Before(threshold) {
			info.Status = PresenceOffline
			stale = append(stale, role)
		}
	}

	return stale
}

// Remove removes a role from tracking.
func (p *PresenceTracker) Remove(role Role) {
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.presence, role)
}

// Clear removes all presence information.
func (p *PresenceTracker) Clear() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.presence = make(map[Role]*PresenceInfo)
}

// RoomPresence combines room info with presence tracking.
type RoomPresence struct {
	Room    *Room
	Tracker *PresenceTracker
}

// NewRoomPresence creates a room presence tracker.
func NewRoomPresence(room *Room) *RoomPresence {
	return &RoomPresence{
		Room:    room,
		Tracker: NewPresenceTracker(room.ID),
	}
}
