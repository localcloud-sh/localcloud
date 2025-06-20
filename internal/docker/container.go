// internal/docker/container.go
package docker

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/api/types/strslice"
	"github.com/docker/go-connections/nat"
)

// ContainerManager manages Docker containers
type ContainerManager interface {
	Create(config ContainerConfig) (string, error)
	Start(containerID string) error
	Stop(containerID string, timeout int) error
	Remove(containerID string) error
	Logs(containerID string, follow bool) (io.ReadCloser, error)
	Inspect(containerID string) (ContainerInfo, error)
	List(filters map[string]string) ([]ContainerInfo, error)
	Exists(name string) (bool, string, error)
	WaitHealthy(containerID string, timeout time.Duration) error
	Stats(containerID string) (ContainerStats, error)
}

// containerManager implements ContainerManager
type containerManager struct {
	client *Client
}

// NewContainerManager creates a new container manager
func (c *Client) NewContainerManager() ContainerManager {
	return &containerManager{client: c}
}

// Create creates a new container
func (m *containerManager) Create(config ContainerConfig) (string, error) {
	// Ensure container name has project prefix
	config.Name = getContainerName(config.Name)

	// Check if container already exists
	exists, existingID, err := m.Exists(config.Name)
	if err != nil {
		return "", err
	}
	if exists {
		return existingID, nil
	}

	// Parse port bindings
	exposedPorts, portBindings, err := parsePortBindings(config.Ports)
	if err != nil {
		return "", err
	}

	// Create container configuration
	containerConfig := &container.Config{
		Image:        config.Image,
		Env:          formatEnvironment(config.Env),
		ExposedPorts: exposedPorts,
		Labels:       config.Labels,
	}

	// Add command if specified
	if len(config.Command) > 0 {
		containerConfig.Cmd = strslice.StrSlice(config.Command)
	}

	// Add health check if specified
	if config.HealthCheck != nil {
		containerConfig.Healthcheck = &container.HealthConfig{
			Test:        config.HealthCheck.Test,
			Interval:    time.Duration(config.HealthCheck.Interval) * time.Second,
			Timeout:     time.Duration(config.HealthCheck.Timeout) * time.Second,
			Retries:     config.HealthCheck.Retries,
			StartPeriod: time.Duration(config.HealthCheck.StartPeriod) * time.Second,
		}
	}

	// Create host configuration
	hostConfig := &container.HostConfig{
		PortBindings: portBindings,
		Mounts:       parseMounts(config.Volumes),
		RestartPolicy: container.RestartPolicy{
			Name: config.RestartPolicy,
		},
	}

	// Set resource limits
	if config.Memory > 0 {
		hostConfig.Memory = config.Memory
	}
	if config.CPUQuota > 0 {
		hostConfig.CPUQuota = config.CPUQuota
		hostConfig.CPUPeriod = 100000 // Default period
	}

	// Create container
	resp, err := m.client.docker.ContainerCreate(
		m.client.ctx,
		containerConfig,
		hostConfig,
		nil, // NetworkingConfig
		nil, // Platform
		config.Name,
	)
	if err != nil {
		return "", fmt.Errorf("failed to create container: %w", err)
	}

	// Connect to networks
	for _, network := range config.Networks {
		err = m.client.docker.NetworkConnect(
			m.client.ctx,
			network,
			resp.ID,
			nil,
		)
		if err != nil {
			// Try to clean up container if network connection fails
			_ = m.Remove(resp.ID)
			return "", fmt.Errorf("failed to connect to network %s: %w", network, err)
		}
	}

	return resp.ID, nil
}

// Start starts a container
func (m *containerManager) Start(containerID string) error {
	err := m.client.docker.ContainerStart(m.client.ctx, containerID, types.ContainerStartOptions{})
	if err != nil {
		return fmt.Errorf("failed to start container: %w", err)
	}
	return nil
}

// Stop stops a container
func (m *containerManager) Stop(containerID string, timeout int) error {
	timeoutDuration := time.Duration(timeout) * time.Second
	err := m.client.docker.ContainerStop(m.client.ctx, containerID, &timeoutDuration)
	if err != nil {
		return fmt.Errorf("failed to stop container: %w", err)
	}
	return nil
}

// Remove removes a container
func (m *containerManager) Remove(containerID string) error {
	err := m.client.docker.ContainerRemove(m.client.ctx, containerID, types.ContainerRemoveOptions{
		Force:         true,
		RemoveVolumes: false, // Keep volumes by default
	})
	if err != nil {
		return fmt.Errorf("failed to remove container: %w", err)
	}
	return nil
}

// Logs returns container logs
func (m *containerManager) Logs(containerID string, follow bool) (io.ReadCloser, error) {
	options := types.ContainerLogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Follow:     follow,
		Timestamps: true,
	}

	reader, err := m.client.docker.ContainerLogs(m.client.ctx, containerID, options)
	if err != nil {
		return nil, fmt.Errorf("failed to get container logs: %w", err)
	}

	return reader, nil
}

// Inspect inspects a container and returns its info
func (m *containerManager) Inspect(containerID string) (ContainerInfo, error) {
	inspect, err := m.client.docker.ContainerInspect(m.client.ctx, containerID)
	if err != nil {
		return ContainerInfo{}, fmt.Errorf("failed to inspect container: %w", err)
	}

	info := ContainerInfo{
		ID:      inspect.ID,
		Name:    strings.TrimPrefix(inspect.Name, "/"),
		Image:   inspect.Config.Image,
		Status:  inspect.State.Status,
		State:   inspect.State.Status,
		Created: inspect.Created,
	}

	// Parse health status
	if inspect.State.Health != nil {
		info.Health = inspect.State.Health.Status
	}

	// Parse port mappings
	info.Ports = make(map[string]string)
	for port, bindings := range inspect.NetworkSettings.Ports {
		if len(bindings) > 0 {
			info.Ports[string(port)] = fmt.Sprintf("%s:%s", bindings[0].HostIP, bindings[0].HostPort)
		}
	}

	// Parse started time
	if startedAt, err := time.Parse(time.RFC3339Nano, inspect.State.StartedAt); err == nil {
		info.StartedAt = startedAt.Unix()
	}

	return info, nil
}

// List lists containers matching the filters
func (m *containerManager) List(filters map[string]string) ([]ContainerInfo, error) {
	// Build filter args
	filterArgs := types.NewFilterArgs()
	for k, v := range filters {
		filterArgs.Add(k, v)
	}

	containers, err := m.client.docker.ContainerList(m.client.ctx, types.ContainerListOptions{
		All:     true,
		Filters: filterArgs,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list containers: %w", err)
	}

	var result []ContainerInfo
	for _, c := range containers {
		info := ContainerInfo{
			ID:      c.ID,
			Name:    strings.TrimPrefix(c.Names[0], "/"),
			Image:   c.Image,
			Status:  c.Status,
			State:   c.State,
			Created: c.Created,
		}

		// Parse ports
		info.Ports = make(map[string]string)
		for _, p := range c.Ports {
			key := fmt.Sprintf("%d/%s", p.PrivatePort, p.Type)
			if p.PublicPort > 0 {
				info.Ports[key] = fmt.Sprintf("%s:%d", p.IP, p.PublicPort)
			}
		}

		result = append(result, info)
	}

	return result, nil
}

// Exists checks if a container with the given name exists
func (m *containerManager) Exists(name string) (bool, string, error) {
	containers, err := m.List(map[string]string{
		"name": name,
	})
	if err != nil {
		return false, "", err
	}

	for _, c := range containers {
		if c.Name == name {
			return true, c.ID, nil
		}
	}

	return false, "", nil
}

// WaitHealthy waits for a container to become healthy
func (m *containerManager) WaitHealthy(containerID string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		info, err := m.Inspect(containerID)
		if err != nil {
			return err
		}

		if info.State != "running" {
			return fmt.Errorf("container is not running: %s", info.State)
		}

		if info.Health == "" || info.Health == "healthy" {
			// No health check or already healthy
			return nil
		}

		if info.Health == "unhealthy" {
			return fmt.Errorf("container is unhealthy")
		}

		// Still starting, wait a bit
		time.Sleep(1 * time.Second)
	}

	return fmt.Errorf("timeout waiting for container to become healthy")
}

// ContainerStats represents container resource usage
type ContainerStats struct {
	CPUPercent    float64
	MemoryUsage   uint64
	MemoryLimit   uint64
	MemoryPercent float64
	NetworkRx     uint64
	NetworkTx     uint64
}

// Stats returns container resource statistics
func (m *containerManager) Stats(containerID string) (ContainerStats, error) {
	statsResp, err := m.client.docker.ContainerStats(m.client.ctx, containerID, false)
	if err != nil {
		return ContainerStats{}, fmt.Errorf("failed to get container stats: %w", err)
	}
	defer statsResp.Body.Close()

	var stats types.Stats
	if err := json.NewDecoder(statsResp.Body).Decode(&stats); err != nil {
		return ContainerStats{}, fmt.Errorf("failed to decode stats: %w", err)
	}

	// Calculate CPU percentage
	cpuDelta := float64(stats.CPUStats.CPUUsage.TotalUsage - stats.PreCPUStats.CPUUsage.TotalUsage)
	systemDelta := float64(stats.CPUStats.SystemUsage - stats.PreCPUStats.SystemUsage)
	cpuPercent := 0.0
	if systemDelta > 0.0 && cpuDelta > 0.0 {
		cpuPercent = (cpuDelta / systemDelta) * float64(len(stats.CPUStats.CPUUsage.PercpuUsage)) * 100.0
	}

	// Calculate memory percentage
	memoryPercent := 0.0
	if stats.MemoryStats.Limit > 0 {
		memoryPercent = (float64(stats.MemoryStats.Usage) / float64(stats.MemoryStats.Limit)) * 100.0
	}

	// Calculate network usage
	var networkRx, networkTx uint64
	for _, network := range stats.Networks {
		networkRx += network.RxBytes
		networkTx += network.TxBytes
	}

	return ContainerStats{
		CPUPercent:    cpuPercent,
		MemoryUsage:   stats.MemoryStats.Usage,
		MemoryLimit:   stats.MemoryStats.Limit,
		MemoryPercent: memoryPercent,
		NetworkRx:     networkRx,
		NetworkTx:     networkTx,
	}, nil
}

// parseMounts converts volume mounts to Docker format
func parseMounts(volumes []VolumeMount) []mount.Mount {
	var mounts []mount.Mount
	for _, v := range volumes {
		m := mount.Mount{
			Type:     mount.TypeBind,
			Source:   v.Source,
			Target:   v.Target,
			ReadOnly: v.ReadOnly,
		}

		// Check if this is a named volume
		if !strings.HasPrefix(v.Source, "/") && !strings.HasPrefix(v.Source, "./") {
			m.Type = mount.TypeVolume
		}

		mounts = append(mounts, m)
	}
	return mounts
}
