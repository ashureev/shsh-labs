// Package container provides Docker container management for playground sessions.
package container

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"time"

	"github.com/containerd/errdefs"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
)

const (
	// Container configuration.
	imageName       = "playground:latest"
	containerUser   = "1000"
	workingDir      = "/home/learner/work"
	mountPath       = "/home/learner/work"
	stopTimeoutSecs = 10

	// Resource limits.
	memoryLimitBytes = 512 * 1024 * 1024 // 512MB
	cpuQuota         = 50000             // 0.5 CPU
	pidsLimit        = 256

	// Exec defaults.
	defaultCols = 80
	defaultRows = 24

	// Restart grace period for stopped containers.
	restartGracePeriod = 60 * time.Minute

	// Playground network configuration.
	playgroundNetwork = "shsh-playground"
	playgroundSubnet  = "172.28.0.0/16"

	createRetryAttempts = 20
	createRetryDelay    = 250 * time.Millisecond
)

// Manager defines the interface for managing playground containers.
type Manager interface {
	// EnsureContainer ensures a container exists and is running for a user.
	EnsureContainer(ctx context.Context, userID string, currentContainerID string, lastSeenAt time.Time, env map[string]string) (string, error)

	// StopContainer stops and removes a container.
	StopContainer(ctx context.Context, containerID string) error

	// IsRunning checks if a container is currently running.
	IsRunning(ctx context.Context, containerID string) (bool, error)

	// CreateExecSession creates a new exec session in a running container.
	CreateExecSession(ctx context.Context, containerID string) (string, io.ReadWriteCloser, error)

	// ResizeExecSession resizes a running exec session.
	ResizeExecSession(ctx context.Context, execID string, cols, rows uint) error

	// Client returns the underlying Docker client.
	Client() *client.Client

	// EnsureNetwork creates the custom bridge network if it doesn't exist.
	EnsureNetwork(ctx context.Context) (string, error)
}

// DockerManager implements Manager using the Docker API.
type DockerManager struct {
	cli     *client.Client
	runtime string // Container runtime: "" = default (runc), "runsc" = gVisor
}

// NewDockerManager creates a new Docker-backed container manager.
// runtime can be "" for default Docker runtime or "runsc" for gVisor.
func NewDockerManager(runtime string) (Manager, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("create docker client: %w", err)
	}
	if runtime != "" {
		slog.Info("Docker client initialized", "runtime", runtime)
	} else {
		slog.Info("Docker client initialized", "runtime", "default")
	}
	return &DockerManager{cli: cli, runtime: runtime}, nil
}

// EnsureContainer ensures a container exists and is running for a user.
func (m *DockerManager) EnsureContainer(ctx context.Context, userID string, currentContainerID string, lastSeenAt time.Time, env map[string]string) (string, error) {
	containerName := fmt.Sprintf("playground-%s", userID)
	volumeName := fmt.Sprintf("playground-%s-data", userID)

	// Check if container already exists.
	inspect, err := m.cli.ContainerInspect(ctx, containerName)
	if err == nil {
		// If DB no longer points to an active container, any lingering named container
		// is stale and must be recycled instead of reused.
		if currentContainerID == "" {
			slog.Info("Found unbound container, recreating",
				"container_id", inspect.ID,
				"user_id", userID,
			)
			if err := m.StopContainer(ctx, inspect.ID); err != nil {
				slog.Warn("Failed to stop unbound container before recreation", "error", err, "container_id", inspect.ID)
			}
		} else {
			if inspect.State.Running {
				slog.Info("Container already running", "container_id", inspect.ID, "user_id", userID)
				return inspect.ID, nil
			}

			// Check if within grace period for restart.
			if time.Since(lastSeenAt) < restartGracePeriod {
				slog.Info("Restarting stopped container", "container_id", inspect.ID, "user_id", userID)
				if err := m.cli.ContainerStart(ctx, inspect.ID, container.StartOptions{}); err != nil {
					return "", fmt.Errorf("restart container %s: %w", inspect.ID, err)
				}
				return inspect.ID, nil
			}

			// Outside grace period: recreate.
			slog.Info("Container expired, recreating", "container_id", inspect.ID, "user_id", userID)
			if err := m.StopContainer(ctx, inspect.ID); err != nil {
				slog.Warn("Failed to stop container before recreation", "error", err, "container_id", inspect.ID)
			}
		}
	}

	slog.Info("Creating new container", "user_id", userID, "volume", volumeName)

	envVars := make([]string, 0, len(env))
	for k, v := range env {
		envVars = append(envVars, fmt.Sprintf("%s=%s", k, v))
	}

	config := &container.Config{
		Image:      imageName,
		User:       containerUser,
		WorkingDir: workingDir,
		Tty:        true,
		Env:        envVars,
	}

	hostConfig := &container.HostConfig{
		Runtime:     m.runtime,
		NetworkMode: container.NetworkMode(playgroundNetwork),
		Mounts: []mount.Mount{{
			Type:   mount.TypeVolume,
			Source: volumeName,
			Target: mountPath,
		}},
		Resources: container.Resources{
			Memory:    memoryLimitBytes,
			CPUQuota:  cpuQuota,
			PidsLimit: ptr(int64(pidsLimit)),
		},
		DNS: []string{"8.8.8.8", "8.8.4.4"},
	}

	var resp container.CreateResponse
	var createErr error
	for i := 0; i < createRetryAttempts; i++ {
		resp, createErr = m.cli.ContainerCreate(ctx, config, hostConfig, nil, nil, containerName)
		if createErr == nil {
			break
		}

		errStr := strings.ToLower(createErr.Error())
		if !strings.Contains(errStr, "is already in use") && !strings.Contains(errStr, "conflict") {
			return "", fmt.Errorf("create container: %w", createErr)
		}

		// A concurrent/delayed cleanup can leave the old named container briefly.
		// Force-stop/remove by name and retry shortly.
		slog.Warn("Container name conflict during create, retrying",
			"user_id", userID,
			"container_name", containerName,
			"attempt", i+1,
			"error", createErr,
		)

		if inspect, inspectErr := m.cli.ContainerInspect(ctx, containerName); inspectErr == nil {
			if stopErr := m.StopContainer(ctx, inspect.ID); stopErr != nil {
				slog.Warn("Failed to stop conflicting container before retry", "container_id", inspect.ID, "error", stopErr)
			}
		}

		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(createRetryDelay):
		}
	}
	if createErr != nil {
		return "", fmt.Errorf("create container after retries: %w", createErr)
	}

	if err := m.cli.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		if removeErr := m.cli.ContainerRemove(ctx, resp.ID, container.RemoveOptions{Force: true}); removeErr != nil && !errors.Is(removeErr, context.Canceled) {
			slog.Warn("Failed to remove container after start failure", "container_id", resp.ID, "error", removeErr)
		}
		return "", fmt.Errorf("start container %s: %w", resp.ID, err)
	}

	// Fix DNS if using gVisor: Overwrite /etc/resolv.conf to bypass Docker's
	// embedded DNS (127.0.0.11) which often fails with gVisor netstack.
	if m.runtime == "runsc" {
		if err := m.fixDNS(ctx, resp.ID); err != nil {
			slog.Warn("Failed to apply DNS fix", "error", err)
			// Proceed anyway, might work partially
		}
	}

	slog.Info("Container created and started", "container_id", resp.ID, "user_id", userID)
	return resp.ID, nil
}

// fixDNS forces public DNS servers into /etc/resolv.conf (gVisor workaround).
func (m *DockerManager) fixDNS(ctx context.Context, containerID string) error {
	cmd := []string{"sh", "-c", "echo 'nameserver 8.8.8.8' > /etc/resolv.conf && echo 'nameserver 8.8.4.4' >> /etc/resolv.conf"}

	execConfig := container.ExecOptions{
		Cmd:  cmd,
		User: "root",
	}

	resp, err := m.cli.ContainerExecCreate(ctx, containerID, execConfig)
	if err != nil {
		return fmt.Errorf("create exec for dns fix: %w", err)
	}

	// We use Attach to wait for completion
	attachResp, err := m.cli.ContainerExecAttach(ctx, resp.ID, container.ExecStartOptions{})
	if err != nil {
		return fmt.Errorf("attach exec for dns fix: %w", err)
	}
	defer attachResp.Close()

	// Wait for it to finish reading (blocking)
	if _, err := io.ReadAll(attachResp.Reader); err != nil {
		return fmt.Errorf("read dns fix output: %w", err)
	}

	inspect, err := m.cli.ContainerExecInspect(ctx, resp.ID)
	if err != nil {
		return fmt.Errorf("inspect dns fix exec: %w", err)
	}
	if inspect.ExitCode != 0 {
		return fmt.Errorf("dns fix command failed with exit code %d", inspect.ExitCode)
	}

	return nil
}

// StopContainer stops and removes a container.
// It is idempotent and handles concurrent calls gracefully.
func (m *DockerManager) StopContainer(ctx context.Context, containerID string) error {
	slog.Info("Stopping container", "container_id", containerID)

	// Check if container exists before trying to stop
	_, err := m.cli.ContainerInspect(ctx, containerID)
	if err != nil {
		if errdefs.IsNotFound(err) {
			slog.Debug("Container already removed", "container_id", containerID)
			return nil
		}
		return fmt.Errorf("inspect container %s: %w", containerID, err)
	}

	// Stop the container with timeout
	timeout := stopTimeoutSecs
	if err := m.cli.ContainerStop(ctx, containerID, container.StopOptions{Timeout: &timeout}); err != nil {
		// Container may already be stopped or being removed by another process
		if errdefs.IsNotFound(err) {
			slog.Debug("Container already stopped/removed", "container_id", containerID)
		} else if ctx.Err() != nil {
			// Context was canceled - log but continue to try force removal
			slog.Debug("Context canceled during stop, continuing with force removal", "container_id", containerID)
		} else {
			slog.Debug("Container stop returned error, continuing to remove", "container_id", containerID, "error", err)
		}
	}

	// Remove the container (force to ensure it's removed even if stop failed)
	if err := m.cli.ContainerRemove(ctx, containerID, container.RemoveOptions{Force: true}); err != nil {
		// Already being removed or already gone is OK
		if errdefs.IsNotFound(err) {
			slog.Debug("Container already removed", "container_id", containerID)
			return nil
		}
		// Check for "removal is already in progress" error
		if strings.Contains(err.Error(), "is already in progress") {
			slog.Debug("Container removal already in progress", "container_id", containerID)
			return nil
		}
		// If context was canceled, log but don't treat as fatal error
		// since the container may still be getting removed
		if ctx.Err() != nil {
			slog.Debug("Context canceled during remove, container may still be removed", "container_id", containerID, "error", err)
			return nil
		}
		return fmt.Errorf("remove container %s: %w", containerID, err)
	}

	slog.Info("Container stopped and removed", "container_id", containerID)
	return nil
}

// IsRunning checks if a container is currently running.
func (m *DockerManager) IsRunning(ctx context.Context, containerID string) (bool, error) {
	inspect, err := m.cli.ContainerInspect(ctx, containerID)
	if err != nil {
		if errdefs.IsNotFound(err) {
			return false, nil
		}
		return false, fmt.Errorf("inspect container %s: %w", containerID, err)
	}
	return inspect.State.Running, nil
}

// CreateExecSession creates a new exec session in a running container.
func (m *DockerManager) CreateExecSession(ctx context.Context, containerID string) (string, io.ReadWriteCloser, error) {
	execConfig := container.ExecOptions{
		AttachStdin:  true,
		AttachStdout: true,
		AttachStderr: true,
		Tty:          true,
		Cmd:          []string{"/bin/bash"},
		User:         containerUser,
		ConsoleSize:  &[2]uint{defaultCols, defaultRows},
	}

	resp, err := m.cli.ContainerExecCreate(ctx, containerID, execConfig)
	if err != nil {
		return "", nil, fmt.Errorf("create exec session in container %s: %w", containerID, err)
	}

	attachResp, err := m.cli.ContainerExecAttach(ctx, resp.ID, container.ExecStartOptions{Tty: true})
	if err != nil {
		return "", nil, fmt.Errorf("attach to exec session %s: %w", resp.ID, err)
	}

	slog.Info("Exec session created", "exec_id", resp.ID, "container_id", containerID)
	return resp.ID, attachResp.Conn, nil
}

// ResizeExecSession resizes a running exec session.
func (m *DockerManager) ResizeExecSession(ctx context.Context, execID string, cols, rows uint) error {
	if err := m.cli.ContainerExecResize(ctx, execID, container.ResizeOptions{
		Height: rows,
		Width:  cols,
	}); err != nil {
		return fmt.Errorf("resize exec session %s to %dx%d: %w", execID, cols, rows, err)
	}
	return nil
}

// Client returns the underlying Docker client.
func (m *DockerManager) Client() *client.Client {
	return m.cli
}

// EnsureNetwork creates the custom bridge network if it doesn't exist.
func (m *DockerManager) EnsureNetwork(ctx context.Context) (string, error) {
	// Check if network already exists.
	networks, err := m.cli.NetworkList(ctx, network.ListOptions{})
	if err != nil {
		return "", fmt.Errorf("list networks: %w", err)
	}

	for _, nw := range networks {
		if nw.Name == playgroundNetwork {
			slog.Info("Playground network already exists", "network_id", nw.ID)
			return nw.ID, nil
		}
	}

	// Create the network.
	createResp, err := m.cli.NetworkCreate(ctx, playgroundNetwork, network.CreateOptions{
		Driver: "bridge",
		IPAM: &network.IPAM{
			Config: []network.IPAMConfig{
				{
					Subnet: playgroundSubnet,
				},
			},
		},
	})
	if err != nil {
		return "", fmt.Errorf("create network %s: %w", playgroundNetwork, err)
	}

	slog.Info("Playground network created", "network_id", createResp.ID, "subnet", playgroundSubnet)
	return createResp.ID, nil
}

func ptr[T any](v T) *T {
	return &v
}
