// Package swarm provides multi-agent collaboration through NATS messaging.
package swarm

import "errors"

// Domain-specific errors for swarm operations.
var (
	// Room errors
	ErrRoomNotFound      = errors.New("room not found")
	ErrRoomAlreadyExists = errors.New("room already exists")
	ErrRoomClosed        = errors.New("room is closed")
	ErrRoomFull          = errors.New("room is at capacity")

	// Role errors
	ErrRoleNotFound    = errors.New("role not found")
	ErrRoleTaken       = errors.New("role is already taken in this room")
	ErrInvalidRole     = errors.New("invalid role")
	ErrRoleNotAllowed  = errors.New("role not allowed in this room")
	ErrNotOrchestrator = errors.New("only orchestrator can perform this action")

	// Connection errors
	ErrNotConnected      = errors.New("not connected to room")
	ErrAlreadyConnected  = errors.New("already connected to a room")
	ErrConnectionFailed  = errors.New("failed to connect to NATS")
	ErrConnectionTimeout = errors.New("connection timeout")
	ErrReconnecting      = errors.New("connection lost, reconnecting")
	ErrMaxReconnects     = errors.New("max reconnection attempts reached")

	// Message errors
	ErrMessageTooLarge   = errors.New("message exceeds size limit")
	ErrInvalidMessage    = errors.New("invalid message format")
	ErrMessageSendFailed = errors.New("failed to send message")
	ErrNoRecipient       = errors.New("no recipient specified")
	ErrChannelFull       = errors.New("message channel full")

	// Presence errors
	ErrAgentNotFound = errors.New("agent not found in room")
	ErrAgentOffline  = errors.New("agent is offline")
)

// ConnectionState represents the current connection state.
type ConnectionState int

const (
	StateDisconnected ConnectionState = iota
	StateConnecting
	StateConnected
	StateReconnecting
	StateClosed
)

func (s ConnectionState) String() string {
	switch s {
	case StateDisconnected:
		return "disconnected"
	case StateConnecting:
		return "connecting"
	case StateConnected:
		return "connected"
	case StateReconnecting:
		return "reconnecting"
	case StateClosed:
		return "closed"
	default:
		return "unknown"
	}
}

// ConnectionEvent represents a connection state change.
type ConnectionEvent struct {
	State   ConnectionState
	Error   error
	Message string
}
