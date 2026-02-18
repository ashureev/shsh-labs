package terminal

import (
	"context"
	"sync"
	"testing"
	"time"
)

// TestOutputBufferNoRace verifies that concurrent ProcessOutput and
// processAnalysisJob calls do not race on OutputBuffer.
//
// Run with: go test -race ./internal/terminal/...
//
// Before the P0 fix, ProcessOutput wrote to OutputBuffer under tm.mu while
// processAnalysisJob read it under session.mu — two different locks protecting
// the same field. This test exercises both paths concurrently to confirm the
// race is gone.
func TestOutputBufferNoRace(t *testing.T) {
	t.Parallel()

	tm := NewMonitor(nil, nil, nil)
	userID := "race-user"
	sessionID := "race-session"

	tm.RegisterSession(userID, sessionID, "container1", "/home/user")

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	const iterations = 200

	var wg sync.WaitGroup

	// Writer goroutine: simulates terminal output arriving.
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < iterations; i++ {
			tm.ProcessOutput(ctx, userID, sessionID, []byte("$ some output line\n"))
			time.Sleep(time.Microsecond)
		}
	}()

	// Reader goroutine: simulates the analysis worker reading the buffer.
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < iterations; i++ {
			tm.mu.RLock()
			session, exists := tm.sessions[monitorSessionKey(userID, sessionID)]
			tm.mu.RUnlock()

			if exists {
				session.mu.RLock()
				_ = session.OutputBuffer.Len()
				session.mu.RUnlock()
			}
			time.Sleep(time.Microsecond)
		}
	}()

	wg.Wait()
}

// TestUpdateTypingStatusNoDeadlock verifies that UpdateTypingStatus does not
// deadlock when the agent service is nil (no gRPC call) and that the lock is
// released before any external call would be made.
func TestUpdateTypingStatusNoDeadlock(t *testing.T) {
	t.Parallel()

	tm := NewMonitor(nil, nil, nil)
	userID := "typing-user"
	sessionID := "typing-session"

	tm.RegisterSession(userID, sessionID, "container1", "/home/user")

	done := make(chan struct{})
	go func() {
		defer close(done)
		// Toggle typing status rapidly; should never deadlock.
		for i := 0; i < 50; i++ {
			tm.UpdateTypingStatus(userID, sessionID, i%2 == 0)
		}
	}()

	select {
	case <-done:
		// OK
	case <-time.After(3 * time.Second):
		t.Fatal("UpdateTypingStatus deadlocked")
	}
}
