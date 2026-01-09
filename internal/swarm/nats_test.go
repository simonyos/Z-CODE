package swarm

import (
	"testing"
	"time"
)

func TestNATSConnection(t *testing.T) {
	// Skip if NATS isn't running
	config := DefaultNATSConfig()
	ns := NewNATSSwarm(config)

	err := ns.Connect()
	if err != nil {
		t.Skipf("NATS not available: %v", err)
	}
	defer ns.Close()

	if !ns.IsConnected() {
		t.Fatal("Expected to be connected")
	}
}

func TestNATSSwarmMessaging(t *testing.T) {
	config := DefaultNATSConfig()

	// Create two swarm connections
	ns1 := NewNATSSwarm(config)
	ns2 := NewNATSSwarm(config)

	if err := ns1.Connect(); err != nil {
		t.Skipf("NATS not available: %v", err)
	}
	defer ns1.Close()

	if err := ns2.Connect(); err != nil {
		t.Fatalf("Failed to connect ns2: %v", err)
	}
	defer ns2.Close()

	roomID := "test-room-" + generateRoomID()[:4]

	// Agent 1 joins as ORCH
	if err := ns1.JoinRoom(roomID, RoleOrchestrator); err != nil {
		t.Fatalf("Failed to join room as ORCH: %v", err)
	}

	// Agent 2 joins as SA
	if err := ns2.JoinRoom(roomID, RoleSA); err != nil {
		t.Fatalf("Failed to join room as SA: %v", err)
	}

	// Give subscriptions time to set up
	time.Sleep(100 * time.Millisecond)

	// ORCH sends message to SA
	testContent := "Hello SA, please design the system"
	if err := ns1.SendTo(RoleSA, testContent, MsgRequest); err != nil {
		t.Fatalf("Failed to send message: %v", err)
	}

	// SA should receive the message
	select {
	case msg := <-ns2.Messages():
		if msg.Content != testContent {
			t.Errorf("Expected content '%s', got '%s'", testContent, msg.Content)
		}
		if msg.From != RoleOrchestrator {
			t.Errorf("Expected from ORCH, got %s", msg.From)
		}
		t.Logf("Message received successfully: %s -> %s: %s", msg.From, msg.To, msg.Content)
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for message")
	}
}

func TestNATSBroadcast(t *testing.T) {
	config := DefaultNATSConfig()

	// Create three connections
	ns1 := NewNATSSwarm(config)
	ns2 := NewNATSSwarm(config)
	ns3 := NewNATSSwarm(config)

	if err := ns1.Connect(); err != nil {
		t.Skipf("NATS not available: %v", err)
	}
	defer ns1.Close()

	if err := ns2.Connect(); err != nil {
		t.Fatalf("Failed to connect ns2: %v", err)
	}
	defer ns2.Close()

	if err := ns3.Connect(); err != nil {
		t.Fatalf("Failed to connect ns3: %v", err)
	}
	defer ns3.Close()

	roomID := "test-broadcast-" + generateRoomID()[:4]

	// Join room with different roles
	ns1.JoinRoom(roomID, RoleOrchestrator)
	ns2.JoinRoom(roomID, RoleSA)
	ns3.JoinRoom(roomID, RoleBEDev)

	time.Sleep(100 * time.Millisecond)

	// ORCH broadcasts
	broadcastContent := "Team meeting in 5 minutes!"
	if err := ns1.Broadcast(broadcastContent, MsgBroadcast); err != nil {
		t.Fatalf("Failed to broadcast: %v", err)
	}

	// Both SA and BE_DEV should receive
	received := 0
	timeout := time.After(2 * time.Second)

	for received < 2 {
		select {
		case msg := <-ns2.Messages():
			if msg.Content == broadcastContent {
				received++
				t.Logf("SA received broadcast")
			}
		case msg := <-ns3.Messages():
			if msg.Content == broadcastContent {
				received++
				t.Logf("BE_DEV received broadcast")
			}
		case <-timeout:
			t.Fatalf("Timeout: only received %d/2 broadcasts", received)
		}
	}

	t.Logf("Both agents received broadcast successfully")
}

func TestPresenceTracking(t *testing.T) {
	config := DefaultNATSConfig()

	ns1 := NewNATSSwarm(config)
	ns2 := NewNATSSwarm(config)

	if err := ns1.Connect(); err != nil {
		t.Skipf("NATS not available: %v", err)
	}
	defer ns1.Close()

	if err := ns2.Connect(); err != nil {
		t.Fatalf("Failed to connect ns2: %v", err)
	}
	defer ns2.Close()

	roomID := "test-presence-" + generateRoomID()[:4]

	// ORCH joins first
	ns1.JoinRoom(roomID, RoleOrchestrator)

	// Drain ORCH's own presence announcement
	select {
	case <-ns1.Presence():
	case <-time.After(500 * time.Millisecond):
	}

	// SA joins - ORCH should see SA's presence update
	ns2.JoinRoom(roomID, RoleSA)

	// Wait for SA's presence event
	timeout := time.After(2 * time.Second)
	for {
		select {
		case event := <-ns1.Presence():
			if event.Role == RoleSA {
				if event.Status != PresenceOnline {
					t.Errorf("Expected online status, got %s", event.Status)
				}
				t.Logf("Presence event received: %s is %s", event.Role, event.Status)
				return // Success
			}
			// Keep waiting if it's not SA
		case <-timeout:
			t.Fatal("Timeout waiting for SA presence event")
		}
	}
}
