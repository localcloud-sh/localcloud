// internal/docker/container_manager_ext.go
package docker

import (
	"context"
	"fmt"
	"time"

	"github.com/docker/docker/api/types"
)

// CreateAndStartContainer creates and starts a new container using existing managers
func (m *Manager) CreateAndStartContainer(config ContainerConfig) (string, error) {
	// Create container using existing container manager
	containerID, err := m.container.Create(config)
	if err != nil {
		return "", fmt.Errorf("failed to create container: %w", err)
	}

	// Start container using existing container manager
	if err := m.container.Start(containerID); err != nil {
		// Remove container on start failure
		m.container.Remove(containerID)
		return "", fmt.Errorf("failed to start container: %w", err)
	}

	return containerID, nil
}

// GetContainerStatus gets the status of a container
func (m *Manager) GetContainerStatus(containerID string) (*ContainerInfo, error) {
	// Get container info using existing container manager
	containers, err := m.container.List(map[string]string{})
	if err != nil {
		return nil, fmt.Errorf("failed to list containers: %w", err)
	}

	// Find the specific container
	for _, container := range containers {
		if container.ID == containerID || container.ID[:12] == containerID[:12] {
			info := &ContainerInfo{
				ID:     container.ID,
				Name:   container.Name,
				Image:  container.Image,
				Status: container.State,
				State:  container.State,
				Health: container.Health,
			}

			// Extract ports
			info.Ports = make(map[string]string)
			for port, binding := range container.Ports {
				info.Ports[port] = binding
			}

			return info, nil
		}
	}

	return nil, fmt.Errorf("container %s not found", containerID)
}

// WaitForContainerHealth waits for a container to become healthy
func (m *Manager) WaitForContainerHealth(containerID string, timeout time.Duration) error {
	// Use existing WaitHealthy method
	return m.container.WaitHealthy(containerID, timeout)
}

// Additional helper methods that extend existing functionality

// GetContainerLogs retrieves logs from a container
func (m *Manager) GetContainerLogs(containerID string, follow bool, tail string) ([]byte, error) {
	ctx := context.Background()

	// Use the docker client directly for logs
	options := types.ContainerLogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Follow:     follow,
		Timestamps: true,
	}

	if tail != "" {
		options.Tail = tail
	}

	reader, err := m.client.docker.ContainerLogs(ctx, containerID, options)
	if err != nil {
		return nil, fmt.Errorf("failed to get container logs: %w", err)
	}
	defer reader.Close()

	// Read all logs for now (could be streamed in the future)
	var logs []byte
	buf := make([]byte, 8192)
	for {
		n, err := reader.Read(buf)
		if n > 0 {
			logs = append(logs, buf[:n]...)
		}
		if err != nil {
			break
		}
	}

	return logs, nil
}

// ExecInContainer executes a command in a running container
func (m *Manager) ExecInContainer(containerID string, cmd []string) (string, error) {
	ctx := context.Background()

	// Create exec configuration
	execConfig := types.ExecConfig{
		Cmd:          cmd,
		AttachStdout: true,
		AttachStderr: true,
	}

	// Create exec instance
	execResp, err := m.client.docker.ContainerExecCreate(ctx, containerID, execConfig)
	if err != nil {
		return "", fmt.Errorf("failed to create exec: %w", err)
	}

	// Start exec
	resp, err := m.client.docker.ContainerExecAttach(ctx, execResp.ID, types.ExecStartCheck{})
	if err != nil {
		return "", fmt.Errorf("failed to start exec: %w", err)
	}
	defer resp.Close()

	// Read output
	var output []byte
	buf := make([]byte, 8192)
	for {
		n, err := resp.Reader.Read(buf)
		if n > 0 {
			output = append(output, buf[:n]...)
		}
		if err != nil {
			break
		}
	}

	return string(output), nil
}

// ListContainersByLabel lists containers with specific labels
func (m *Manager) ListContainersByLabel(labels map[string]string) ([]ContainerInfo, error) {
	// Use existing container manager to list
	containers, err := m.container.List(labels)
	if err != nil {
		return nil, err
	}

	// Convert to ContainerInfo slice
	var result []ContainerInfo
	for _, c := range containers {
		info := ContainerInfo{
			ID:     c.ID,
			Name:   c.Name,
			Image:  c.Image,
			Status: c.State,
			State:  c.State,
			Health: c.Health,
		}

		// Extract ports
		info.Ports = make(map[string]string)
		for port, binding := range c.Ports {
			info.Ports[port] = binding
		}

		result = append(result, info)
	}

	return result, nil
}

// StopContainer stops a running container
func (m *Manager) StopContainer(containerID string, timeout int) error {
	// Use existing container manager
	return m.container.Stop(containerID, timeout)
}
