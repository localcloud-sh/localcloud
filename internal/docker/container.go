// internal/docker/container.go
package docker

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/api/types/strslice"
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
	// Debug log
	fmt.Printf("DEBUG: Creating container with name: %s\n", config.Name)

	// Ensure container name has project prefix
	config.Name = getContainerName(config.Name)
	fmt.Printf("DEBUG: Container name after prefix: %s\n", config.Name)

	// Check if container already exists
	exists, existingID, err := m.Exists(config.Name)
	if err != nil {
		fmt.Printf("DEBUG: Error checking if container exists: %v\n", err)
		return "", err
	}
	if exists {
		fmt.Printf("DEBUG: Container already exists with ID: %s\n", existingID)
		return existingID, nil
	}

	// Parse port bindings
	fmt.Printf("DEBUG: Parsing port bindings: %+v\n", config.Ports)
	exposedPorts, portBindings, err := parsePortBindings(config.Ports)
	if err != nil {
		fmt.Printf("DEBUG: Error parsing ports: %v\n", err)
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

	fmt.Printf("DEBUG: Creating container with image: %s\n", config.Image)
	fmt.Printf("DEBUG: Networks: %v\n", config.Networks)

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
		fmt.Printf("DEBUG: Container creation failed: %v\n", err)
		return "", fmt.Errorf("failed to create container: %w", err)
	}

	fmt.Printf("DEBUG: Container created with ID: %s\n", resp.ID)

	// Connect to networks
	for _, network := range config.Networks {
		fmt.Printf("DEBUG: Connecting to network: %s\n", network)
		err = m.client.docker.NetworkConnect(
			m.client.ctx,
			network,
			resp.ID,
			nil,
		)
		if err != nil {
			fmt.Printf("DEBUG: Network connection failed: %v\n", err)
			// Try to clean up container if network connection fails
			_ = m.Remove(resp.ID)
			return "", fmt.Errorf("failed to connect to network %s: %w", network, err)
		}
	}

	fmt.Printf("DEBUG: Container %s created successfully\n", config.Name)
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
	// Create stop options with timeout
	stopOptions := container.StopOptions{
		Timeout: &timeout,
	}

	err := m.client.docker.ContainerStop(m.client.ctx, containerID, stopOptions)
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
		Created: time.Now().Unix(), // Parse from inspect.Created if needed
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
func (m *containerManager) List(filterMap map[string]string) ([]ContainerInfo, error) {
	opts := types.ContainerListOptions{
		All: true,
	}

	// Add filters
	if len(filterMap) > 0 {
		filterArgs := filters.NewArgs()
		for key, value := range filterMap {
			filterArgs.Add(key, value)
		}
		opts.Filters = filterArgs
	}

	containers, err := m.client.docker.ContainerList(m.client.ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to list containers: %w", err)
	}

	// Convert to ContainerInfo
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

		// Add health status if available
		if c.Status != "" && strings.Contains(c.Status, "(healthy)") {
			info.Health = "healthy"
		} else if strings.Contains(c.Status, "(unhealthy)") {
			info.Health = "unhealthy"
		}

		// Parse ports
		info.Ports = make(map[string]string)
		for _, p := range c.Ports {
			key := fmt.Sprintf("%d/%s", p.PrivatePort, p.Type)
			value := ""
			if p.PublicPort > 0 {
				value = fmt.Sprintf("%s:%d", p.IP, p.PublicPort)
			}
			info.Ports[key] = value
		}

		result = append(result, info)
	}

	return result, nil
}

// Exists checks if a container with the given name exists
func (m *containerManager) Exists(name string) (bool, string, error) {
	// Ensure name has prefix
	name = getContainerName(name)

	filterArgs := filters.NewArgs()
	filterArgs.Add("name", name)

	containers, err := m.client.docker.ContainerList(m.client.ctx, types.ContainerListOptions{
		All:     true,
		Filters: filterArgs,
	})
	if err != nil {
		return false, "", err
	}

	for _, c := range containers {
		// Check exact name match (Docker returns partial matches)
		for _, containerName := range c.Names {
			if strings.TrimPrefix(containerName, "/") == name {
				return true, c.ID, nil
			}
		}
	}

	return false, "", nil
}

// WaitHealthy waits for a container to become healthy
func (m *containerManager) WaitHealthy(containerID string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	startTime := time.Now()
	lastLogTime := time.Now()
	checkInterval := 2 * time.Second

	for time.Now().Before(deadline) {
		inspect, err := m.client.docker.ContainerInspect(m.client.ctx, containerID)
		if err != nil {
			return fmt.Errorf("failed to inspect container: %w", err)
		}

		// Log status every 5 seconds for debugging
		if time.Since(lastLogTime) > 5*time.Second {
			fmt.Printf("DEBUG: Container %s status - Running: %v, Health: %v\n",
				inspect.Name,
				inspect.State.Running,
				func() string {
					if inspect.State.Health == nil {
						return "no health check"
					}
					return inspect.State.Health.Status
				}())
			lastLogTime = time.Now()
		}

		// Check if container has exited
		if !inspect.State.Running {
			return fmt.Errorf("container exited with status: %s", inspect.State.Status)
		}

		// If no health check is configured, just check if running
		if inspect.Config.Healthcheck == nil {
			// For containers without health check, wait a bit to ensure it's stable
			if time.Since(startTime) > 5*time.Second && inspect.State.Running {
				return nil
			}
		} else if inspect.State.Health != nil {
			switch inspect.State.Health.Status {
			case "healthy":
				return nil
			case "unhealthy":
				// For MinIO, sometimes it reports unhealthy initially but recovers
				// Check if it's been unhealthy for too long
				if time.Since(startTime) > 20*time.Second {
					// Get the last few health check logs for debugging
					logs := inspect.State.Health.Log
					if len(logs) > 0 {
						fmt.Printf("DEBUG: Last health check output:\n")
						for i := len(logs) - 1; i >= 0 && i >= len(logs)-3; i-- {
							fmt.Printf("  [%d] Exit: %d, Output: %s\n",
								i, logs[i].ExitCode, strings.TrimSpace(logs[i].Output))
						}
					}
					return fmt.Errorf("container is unhealthy after %v", time.Since(startTime))
				}
			case "starting":
				// Still starting, continue waiting
			default:
				// Unknown status, log it
				fmt.Printf("DEBUG: Unknown health status: %s\n", inspect.State.Health.Status)
			}
		}

		time.Sleep(checkInterval)
	}

	// Timeout reached, provide detailed error
	inspect, _ := m.client.docker.ContainerInspect(m.client.ctx, containerID)
	status := "unknown"
	if inspect.State.Health != nil {
		status = inspect.State.Health.Status
	} else if inspect.State.Running {
		status = "running (no health check)"
	} else {
		status = inspect.State.Status
	}

	return fmt.Errorf("timeout waiting for container to be healthy after %v (current status: %s)",
		timeout, status)
}

// Alternative: Add a MinIO-specific health check function
func (m *containerManager) WaitMinIOHealthy(containerID string, port int, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	client := &http.Client{Timeout: 5 * time.Second}

	for time.Now().Before(deadline) {
		// First check if container is running
		inspect, err := m.client.docker.ContainerInspect(m.client.ctx, containerID)
		if err != nil {
			return fmt.Errorf("failed to inspect container: %w", err)
		}

		if !inspect.State.Running {
			time.Sleep(1 * time.Second)
			continue
		}

		// Try MinIO health endpoint
		healthURL := fmt.Sprintf("http://localhost:%d/minio/health/live", port)
		resp, err := client.Get(healthURL)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				fmt.Printf("DEBUG: MinIO health check passed\n")
				return nil
			}
			fmt.Printf("DEBUG: MinIO health check returned status %d\n", resp.StatusCode)
		} else {
			fmt.Printf("DEBUG: MinIO health check error: %v\n", err)
		}

		time.Sleep(2 * time.Second)
	}

	return fmt.Errorf("timeout waiting for MinIO to be healthy")
}

// internal/docker/container.go
// Stats returns container statistics - FIXED VERSION

// Stats returns container statistics
func (m *containerManager) Stats(containerID string) (ContainerStats, error) {
	stats, err := m.client.docker.ContainerStats(m.client.ctx, containerID, false)
	if err != nil {
		return ContainerStats{}, fmt.Errorf("failed to get container stats: %w", err)
	}
	defer stats.Body.Close()

	var v types.StatsJSON
	if err := json.NewDecoder(stats.Body).Decode(&v); err != nil {
		return ContainerStats{}, fmt.Errorf("failed to decode stats: %w", err)
	}

	// Calculate CPU percentage
	cpuDelta := float64(v.CPUStats.CPUUsage.TotalUsage - v.PreCPUStats.CPUUsage.TotalUsage)
	systemDelta := float64(v.CPUStats.SystemUsage - v.PreCPUStats.SystemUsage)
	cpuPercent := 0.0
	if systemDelta > 0.0 && cpuDelta > 0.0 {
		cpuPercent = (cpuDelta / systemDelta) * float64(len(v.CPUStats.CPUUsage.PercpuUsage)) * 100.0
	}

	// Memory usage
	memUsage := v.MemoryStats.Usage
	memLimit := v.MemoryStats.Limit
	memPercent := 0.0
	if memLimit > 0 {
		memPercent = (float64(memUsage) / float64(memLimit)) * 100.0
	}

	// Network stats - safely check if network exists
	var networkRx, networkTx uint64
	if v.Networks != nil {
		// Try different network interfaces
		for _, netName := range []string{"eth0", "bridge", "host"} {
			if netStats, ok := v.Networks[netName]; ok {
				networkRx = netStats.RxBytes
				networkTx = netStats.TxBytes
				break
			}
		}
		// If no specific interface found, use the first available
		if networkRx == 0 && networkTx == 0 {
			for _, netStats := range v.Networks {
				networkRx = netStats.RxBytes
				networkTx = netStats.TxBytes
				break
			}
		}
	}

	// Block I/O stats - safely check if available
	var blockRead, blockWrite uint64
	if v.BlkioStats.IoServiceBytesRecursive != nil {
		for _, ioStat := range v.BlkioStats.IoServiceBytesRecursive {
			switch strings.ToLower(ioStat.Op) {
			case "read":
				blockRead += ioStat.Value
			case "write":
				blockWrite += ioStat.Value
			}
		}
	}

	return ContainerStats{
		CPUPercent:    cpuPercent,
		MemoryUsage:   memUsage,
		MemoryLimit:   memLimit,
		MemoryPercent: memPercent,
		NetworkRx:     networkRx,
		NetworkTx:     networkTx,
		BlockRead:     blockRead,
		BlockWrite:    blockWrite,
	}, nil
}

// parseMounts converts volume mount configs to Docker mount configs
func parseMounts(volumes []VolumeMount) []mount.Mount {
	var mounts []mount.Mount

	for _, v := range volumes {
		m := mount.Mount{
			Target: v.Target,
		}

		if v.Type == "bind" {
			m.Type = mount.TypeBind
			m.Source = v.Source
			m.ReadOnly = v.ReadOnly
		} else {
			m.Type = mount.TypeVolume
			m.Source = v.Source
		}

		mounts = append(mounts, m)
	}

	return mounts
}
