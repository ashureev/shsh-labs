package terminal

import (
	"strconv"
	"testing"
	"time"

	"github.com/coder/websocket"
)

func TestSessionManager_Register(t *testing.T) {
	sm := NewSessionManager()
	conn := &websocket.Conn{}
	userID := "user123"
	sessionID := "tab-1"

	sm.Register(userID, sessionID, conn)

	active := sm.GetActive(userID, sessionID)
	if active != conn {
		t.Errorf("Expected connection %v, got %v", conn, active)
	}
}

func TestSessionManager_Unregister(t *testing.T) {
	sm := NewSessionManager()
	conn := &websocket.Conn{}
	userID := "user123"
	sessionID := "tab-1"

	sm.Register(userID, sessionID, conn)
	sm.Unregister(userID, sessionID, conn)

	active := sm.GetActive(userID, sessionID)
	if active != nil {
		t.Errorf("Expected nil connection, got %v", active)
	}
}

func TestSessionManager_UnregisterStale(t *testing.T) {
	sm := NewSessionManager()
	conn1 := &websocket.Conn{}
	conn2 := &websocket.Conn{}
	userID := "user123"
	session1 := "tab-1"
	session2 := "tab-2"

	sm.Register(userID, session1, conn1)

	// Another tab should remain active when stale unregister happens.
	sm.Register(userID, session2, conn2)

	sm.Unregister(userID, session1, conn1)

	active := sm.GetActive(userID, session2)
	if active != conn2 {
		t.Errorf("Expected connection %v, got %v", conn2, active)
	}
}

func TestSessionManager_ConcurrentAccess(t *testing.T) {
	sm := NewSessionManager()
	userID := "concurrentUser"

	go func() {
		for i := 0; i < 1000; i++ {
			sm.Register(userID, "tab-"+strconv.Itoa(i), &websocket.Conn{})
		}
	}()

	go func() {
		for i := 0; i < 1000; i++ {
			sm.GetActive(userID, "tab-"+strconv.Itoa(i))
		}
	}()

	time.Sleep(100 * time.Millisecond)
}
