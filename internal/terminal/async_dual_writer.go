package terminal

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

// AsyncDualWriter writes to both WebSocket and monitor asynchronously.
// Prevents blocking WebSocket I/O when monitor processing is slow.
type AsyncDualWriter struct {
	wsWriter     *wsWriter
	monitor      *Monitor
	outputChan   chan []byte
	userID       string
	sessionID    string
	sessionKey   string
	ctx          context.Context
	cancel       context.CancelFunc
	wg           sync.WaitGroup
	logger       *slog.Logger
	maxQueueSize int
}

// NewAsyncDualWriter creates a new async dual writer for Monitor.
func NewAsyncDualWriter(ws *wsWriter, monitor *Monitor, userID, sessionID, sessionKey string, logger *slog.Logger) *AsyncDualWriter {
	if logger == nil {
		logger = slog.Default()
	}

	ctx, cancel := context.WithCancel(context.Background())
	dw := &AsyncDualWriter{
		wsWriter:     ws,
		monitor:      monitor,
		outputChan:   make(chan []byte, 100), // Buffered channel for backpressure
		userID:       userID,
		sessionID:    sessionID,
		sessionKey:   sessionKey,
		ctx:          ctx,
		cancel:       cancel,
		logger:       logger,
		maxQueueSize: 100,
	}

	// Start background processor
	dw.wg.Add(1)
	go dw.processOutput()

	return dw
}

// Write implements io.Writer.
// Writes to WebSocket synchronously, queues for monitor asynchronously.
func (w *AsyncDualWriter) Write(p []byte) (int, error) {
	// Write to WebSocket first (must not block)
	n, err := w.wsWriter.Write(p)
	if err != nil {
		return n, err
	}

	// Queue for monitor processing (non-blocking with backpressure)
	data := make([]byte, len(p))
	copy(data, p)

	select {
	case w.outputChan <- data:
		// Successfully queued
		w.logger.Debug("[ASYNC-WRITER] Output queued",
			"user_id", w.userID,
			"data_len", len(data),
			"queue_len", len(w.outputChan),
		)

	case <-w.ctx.Done():
		// Context cancelled, ignore
		w.logger.Debug("[ASYNC-WRITER] Context cancelled, dropping output",
			"user_id", w.userID,
		)

	default:
		// Channel full - apply backpressure by dropping oldest
		w.logger.Warn("[ASYNC-WRITER] Queue full, applying backpressure",
			"user_id", w.userID,
			"queue_len", len(w.outputChan),
		)

		// Remove oldest message to make room
		select {
		case <-w.outputChan:
			// Removed oldest
			w.logger.Debug("[ASYNC-WRITER] Dropped oldest message",
				"user_id", w.userID,
			)
		default:
		}

		// Try to queue again
		select {
		case w.outputChan <- data:
			w.logger.Debug("[ASYNC-WRITER] Output queued after backpressure",
				"user_id", w.userID,
			)
		case <-w.ctx.Done():
		default:
			w.logger.Warn("[ASYNC-WRITER] Failed to queue after backpressure",
				"user_id", w.userID,
			)
		}
	}

	return n, nil
}

// processOutput processes queued output in background.
func (w *AsyncDualWriter) processOutput() {
	defer w.wg.Done()

	w.logger.Info("[ASYNC-WRITER] Output processor started",
		"user_id", w.userID,
	)

	for {
		select {
		case <-w.ctx.Done():
			w.logger.Info("[ASYNC-WRITER] Output processor stopping",
				"user_id", w.userID,
			)
			return

		case data := <-w.outputChan:
			start := time.Now()

			if w.monitor != nil {
				w.monitor.ProcessOutput(w.ctx, w.userID, w.sessionID, data)
			}

			duration := time.Since(start)
			if duration > 100*time.Millisecond {
				w.logger.Warn("[ASYNC-WRITER] Slow monitor processing",
					"user_id", w.userID,
					"duration_ms", duration.Milliseconds(),
				)
			}
		}
	}
}

// Close shuts down the async writer gracefully.
func (w *AsyncDualWriter) Close() error {
	w.logger.Info("[ASYNC-WRITER] Closing",
		"user_id", w.userID,
		"queue_remaining", len(w.outputChan),
	)

	// Signal shutdown
	w.cancel()

	// Drain the channel to unblock the worker goroutine if it's waiting on outputChan
	drained := 0
	for {
		select {
		case <-w.outputChan:
			drained++
		default:
			// Channel empty, worker is either processing or checking context
			goto doneDraining
		}
	}
doneDraining:

	if drained > 0 {
		w.logger.Warn("[ASYNC-WRITER] Drained messages to unblock worker",
			"user_id", w.userID,
			"count", drained,
		)
	}

	// Wait for processor to finish with timeout
	done := make(chan struct{})
	go func() {
		w.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		w.logger.Info("[ASYNC-WRITER] Processor stopped gracefully",
			"user_id", w.userID,
		)
	case <-time.After(5 * time.Second):
		w.logger.Warn("[ASYNC-WRITER] Processor shutdown timeout",
			"user_id", w.userID,
		)
	}

	// Close the channel after worker has exited (or timed out)
	close(w.outputChan)

	return nil
}

// Stats returns writer statistics.
func (w *AsyncDualWriter) Stats() map[string]interface{} {
	return map[string]interface{}{
		"user_id":        w.userID,
		"queue_len":      len(w.outputChan),
		"queue_capacity": cap(w.outputChan),
		"max_queue_size": w.maxQueueSize,
	}
}
