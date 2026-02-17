package terminal

import (
	"sync"
)

// CircularBuffer provides fixed-size circular buffer for terminal output.
// Prevents memory exhaustion from commands like `yes` or large cat outputs.
type CircularBuffer struct {
	buf  []byte
	size int
	head int // write position
	tail int // read position
	full bool
	mu   sync.RWMutex
}

// NewCircularBuffer creates a new circular buffer with specified max size.
// Default size is 64KB which captures most command outputs.
func NewCircularBuffer(size int) *CircularBuffer {
	if size <= 0 {
		size = 64 * 1024 // 64KB default
	}
	return &CircularBuffer{
		buf:  make([]byte, size),
		size: size,
	}
}

// Write implements io.Writer interface.
// When buffer is full, overwrites oldest data.
func (cb *CircularBuffer) Write(p []byte) (n int, err error) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	for _, b := range p {
		if cb.full {
			// Overwrite: advance tail to skip oldest byte
			cb.tail = (cb.tail + 1) % cb.size
		}
		cb.buf[cb.head] = b
		cb.head = (cb.head + 1) % cb.size
		if cb.head == cb.tail {
			cb.full = true
		}
	}
	return len(p), nil
}

// String returns the buffer contents as a string.
// Returns data in correct order even if buffer has wrapped.
func (cb *CircularBuffer) String() string {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	if !cb.full && cb.head == cb.tail {
		return ""
	}

	if cb.full && cb.head == cb.tail {
		// Buffer completely full
		return string(cb.buf)
	}

	if cb.head > cb.tail {
		// No wrap-around
		return string(cb.buf[cb.tail:cb.head])
	}

	// Wrap-around: tail -> end + start -> head
	return string(cb.buf[cb.tail:]) + string(cb.buf[:cb.head])
}

// Bytes returns the buffer contents as a byte slice.
func (cb *CircularBuffer) Bytes() []byte {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	if !cb.full && cb.head == cb.tail {
		return []byte{}
	}

	if cb.full && cb.head == cb.tail {
		result := make([]byte, cb.size)
		copy(result, cb.buf)
		return result
	}

	if cb.head > cb.tail {
		result := make([]byte, cb.head-cb.tail)
		copy(result, cb.buf[cb.tail:cb.head])
		return result
	}

	// Wrap-around
	size := (cb.size - cb.tail) + cb.head
	result := make([]byte, size)
	copy(result, cb.buf[cb.tail:])
	copy(result[cb.size-cb.tail:], cb.buf[:cb.head])
	return result
}

// Len returns the number of bytes in the buffer.
func (cb *CircularBuffer) Len() int {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	if !cb.full && cb.head == cb.tail {
		return 0
	}

	if cb.full && cb.head == cb.tail {
		return cb.size
	}

	if cb.head > cb.tail {
		return cb.head - cb.tail
	}

	return (cb.size - cb.tail) + cb.head
}

// Reset clears the buffer.
func (cb *CircularBuffer) Reset() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.head = 0
	cb.tail = 0
	cb.full = false
}

// Capacity returns the maximum capacity of the buffer.
func (cb *CircularBuffer) Capacity() int {
	return cb.size
}
