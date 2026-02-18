package agent

import (
	"context"
	"errors"
	"fmt"
	"io"
	"iter"
	"log/slog"
	"math"
	"os"
	"time"

	"github.com/ashureev/shsh-labs/internal/proto/agent"
	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
)

var (
	errConnectionShutdown       = errors.New("connection shutdown")
	errConnectionStateUnchanged = errors.New("connection state did not change")
	errChatResponse             = errors.New("chat response returned error")
)

// GrpcClient provides a gRPC client to the Python Agent Service.
type GrpcClient struct {
	conn   *grpc.ClientConn
	client agent.AgentServiceClient
	addr   string
	logger *slog.Logger
}

// GrpcClientConfig holds configuration for the gRPC client.
type GrpcClientConfig struct {
	Address          string
	ConnectTimeout   time.Duration
	RequestTimeout   time.Duration
	KeepaliveTime    time.Duration
	KeepaliveTimeout time.Duration
}

// DefaultGrpcClientConfig returns default configuration.
func DefaultGrpcClientConfig() GrpcClientConfig {
	return GrpcClientConfig{
		Address:          getEnv("PYTHON_AGENT_ADDR", "localhost:50051"),
		ConnectTimeout:   5 * time.Second,
		RequestTimeout:   30 * time.Second,
		KeepaliveTime:    2 * time.Minute,
		KeepaliveTimeout: 10 * time.Second,
	}
}

// NewGrpcClient creates a new gRPC client to the Python Agent Service.
func NewGrpcClient(addr string, logger *slog.Logger) (*GrpcClient, error) {
	if logger == nil {
		logger = slog.Default()
	}

	cfg := DefaultGrpcClientConfig()
	if addr != "" {
		cfg.Address = addr
	}

	// Set up keepalive parameters
	kacp := keepalive.ClientParameters{
		Time:                cfg.KeepaliveTime,
		Timeout:             cfg.KeepaliveTimeout,
		PermitWithoutStream: false,
	}

	// Build client connection (no network I/O yet).
	conn, err := grpc.NewClient(cfg.Address,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithKeepaliveParams(kacp),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Python agent at %s: %w", cfg.Address, err)
	}

	// Force a connection attempt during startup so we fail fast on bad agent endpoints.
	connectCtx, cancel := context.WithTimeout(context.Background(), cfg.ConnectTimeout)
	defer cancel()
	if err := waitForReady(connectCtx, conn); err != nil {
		if closeErr := conn.Close(); closeErr != nil {
			logger.Warn("failed to close gRPC connection after readiness failure", "error", closeErr)
		}
		return nil, fmt.Errorf("python agent at %s not ready: %w", cfg.Address, err)
	}

	client := agent.NewAgentServiceClient(conn)

	logger.Info("Connected to Python Agent Service", "address", cfg.Address)

	return &GrpcClient{
		conn:   conn,
		client: client,
		addr:   cfg.Address,
		logger: logger,
	}, nil
}

func waitForReady(ctx context.Context, conn *grpc.ClientConn) error {
	for {
		state := conn.GetState()
		switch state {
		case connectivity.Ready:
			return nil
		case connectivity.Idle:
			conn.Connect()
		case connectivity.Shutdown:
			return errConnectionShutdown
		}

		if !conn.WaitForStateChange(ctx, state) {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return fmt.Errorf("%w from %s", errConnectionStateUnchanged, state)
		}
	}
}

// Close closes the gRPC connection.
func (c *GrpcClient) Close() {
	if c.conn != nil {
		if err := c.conn.Close(); err != nil {
			c.logger.Warn("failed to close gRPC connection", "error", err)
		}
	}
}

// Health checks if the Python Agent Service is healthy.
func (c *GrpcClient) Health(ctx context.Context) (*agent.HealthResponse, error) {
	resp, err := c.client.Health(ctx, &agent.HealthRequest{})
	if err != nil {
		return nil, fmt.Errorf("health check failed: %w", err)
	}
	return resp, nil
}

// Chat processes a chat message with bidirectional streaming.
func (c *GrpcClient) Chat(ctx context.Context, req ChatRequest) iter.Seq2[*ChatResponse, error] {
	return func(yield func(*ChatResponse, error) bool) {
		sessionID := req.SessionID
		if sessionID == "" {
			sessionID = req.UserID
		}

		stream, err := c.client.Chat(ctx, &agent.ChatRequest{
			Message:     req.Message,
			UserId:      req.UserID,
			ContainerId: req.ContainerID,
			VolumePath:  req.VolumePath,
			SessionId:   sessionID,
		})
		if err != nil {
			yield(nil, fmt.Errorf("chat request failed: %w", err))
			return
		}

		for {
			resp, err := stream.Recv()
			if errors.Is(err, io.EOF) {
				return
			}
			if err != nil {
				yield(nil, fmt.Errorf("chat stream error: %w", err))
				return
			}

			if resp.GetResponseType() == "error" {
				errMsg := resp.GetErrorMessage()
				if errMsg == "" {
					yield(nil, errChatResponse)
					return
				}
				yield(nil, fmt.Errorf("%w: %s", errChatResponse, errMsg))
				return
			}

			chatResp := &ChatResponse{
				Response:  resp.GetContent(),
				ToolsUsed: resp.GetToolsUsed(),
			}
			if !yield(chatResp, nil) {
				return
			}
		}
	}
}

// GetStats returns agent statistics from the Python service.
func (c *GrpcClient) GetStats() AgentStats {
	// For now, return empty stats - could be extended to query Python service
	return AgentStats{
		PatternCount:    0,
		SafetyRuleCount: 0,
	}
}

// ProcessTerminalInput processes terminal input through the Python Agent Service.
func (c *GrpcClient) ProcessTerminalInput(ctx context.Context, input TerminalInput) iter.Seq2[*Response, error] {
	return func(yield func(*Response, error) bool) {
		c.logger.Debug("Processing terminal input via gRPC",
			"command", input.Command,
			"user_id", input.UserID,
		)

		// Convert TerminalInput to protobuf
		req := &agent.TerminalInput{
			Command:   input.Command,
			Pwd:       input.PWD,
			ExitCode:  safeIntToInt32(input.ExitCode),
			Output:    input.Output,
			Timestamp: input.Timestamp,
			UserId:    input.UserID,
			SessionId: input.SessionID,
		}

		// Use a longer timeout for streaming
		ctx, cancel := context.WithTimeout(ctx, 120*time.Second)
		defer cancel()

		stream, err := c.client.ProcessTerminal(ctx, req)
		if err != nil {
			c.logger.Error("ProcessTerminal failed to start", "error", err, "user_id", input.UserID)
			// Yield error response to maintain interface behavior
			yield(&Response{
				Type:    "error",
				Content: "AI service unavailable",
				UserID:  input.UserID,
			}, nil)
			return
		}

		for {
			resp, err := stream.Recv()
			if errors.Is(err, io.EOF) {
				return
			}
			if err != nil {
				c.logger.Error("ProcessTerminal stream error", "error", err, "user_id", input.UserID)
				yield(nil, err)
				return
			}

			// Convert protobuf response to Response
			response := &Response{
				Type:           resp.Type,
				Content:        resp.Content,
				Sidebar:        resp.Sidebar,
				Block:          resp.Block,
				Alert:          resp.Alert,
				RequireConfirm: resp.RequireConfirm,
				Pattern:        resp.Pattern,
				ToolsUsed:      resp.ToolsUsed,
				Silent:         resp.Silent,
				UserID:         resp.UserId,
				SessionID:      input.SessionID,
			}

			if !yield(response, nil) {
				return
			}
		}
	}
}

// UpdateSessionSignals syncs transient learner state to Python session store.
func (c *GrpcClient) UpdateSessionSignals(ctx context.Context, req SessionSignalRequest) error {
	resp, err := c.client.UpdateSessionSignals(ctx, &agent.SessionSignalRequest{
		UserId:            req.UserID,
		SessionId:         req.SessionID,
		InEditorMode:      req.InEditorMode,
		IsTyping:          req.IsTyping,
		JustSelfCorrected: req.JustSelfCorrected,
		EditorName:        req.EditorName,
		Timestamp:         req.Timestamp,
	})
	if err != nil {
		c.logger.Warn("UpdateSessionSignals failed", "error", err, "user_id", req.UserID)
		return err
	}
	// Check logical ok flag — Python returns ok=false on validation/storage failures.
	if !resp.GetOk() {
		c.logger.Warn("UpdateSessionSignals returned ok=false",
			"status", resp.GetStatus(),
			"user_id", req.UserID,
		)
		return fmt.Errorf("UpdateSessionSignals: %s", resp.GetStatus())
	}
	return nil
}

// Helper function.
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func safeIntToInt32(v int) int32 {
	if v > math.MaxInt32 {
		return math.MaxInt32
	}
	if v < math.MinInt32 {
		return math.MinInt32
	}
	return int32(v)
}
