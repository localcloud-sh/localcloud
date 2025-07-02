// internal/diagnostics/debugger.go
package diagnostics

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
	"github.com/localcloud-sh/localcloud/internal/config"
)

// Debugger collects debug information
type Debugger struct {
	config *config.Config
	client *client.Client
	info   *DebugInfo
}

// DebugInfo contains all debug information
type DebugInfo struct {
	Timestamp  time.Time                   `json:"timestamp"`
	System     SystemInfo                  `json:"system"`
	Docker     DockerInfo                  `json:"docker"`
	LocalCloud LocalCloudInfo              `json:"localcloud"`
	Services   map[string]ServiceDebugInfo `json:"services"`
	RecentLogs map[string][]string         `json:"recent_logs"`
	Errors     []ErrorInfo                 `json:"errors"`
}

// SystemInfo contains system information
type SystemInfo struct {
	OS          string            `json:"os"`
	Arch        string            `json:"arch"`
	CPUs        int               `json:"cpus"`
	GoVersion   string            `json:"go_version"`
	Memory      MemoryInfo        `json:"memory"`
	Disk        DiskInfo          `json:"disk"`
	Environment map[string]string `json:"environment"`
}

// MemoryInfo contains memory information
type MemoryInfo struct {
	Total       uint64  `json:"total"`
	Free        uint64  `json:"free"`
	Used        uint64  `json:"used"`
	PercentUsed float64 `json:"percent_used"`
}

// DiskInfo contains disk information
type DiskInfo struct {
	Total       uint64  `json:"total"`
	Free        uint64  `json:"free"`
	Used        uint64  `json:"used"`
	PercentUsed float64 `json:"percent_used"`
}

// DockerInfo contains Docker information
type DockerInfo struct {
	Version       string        `json:"version"`
	APIVersion    string        `json:"api_version"`
	ServerVersion types.Version `json:"server_version"`
	Images        int           `json:"images_count"`
	Containers    int           `json:"containers_count"`
	RunningCount  int           `json:"running_count"`
	SystemInfo    types.Info    `json:"system_info"`
}

// LocalCloudInfo contains LocalCloud specific information
type LocalCloudInfo struct {
	Version       string         `json:"version"`
	ProjectName   string         `json:"project_name"`
	ProjectType   string         `json:"project_type"`
	ConfigPath    string         `json:"config_path"`
	Configuration *config.Config `json:"configuration"`
}

// ServiceDebugInfo contains debug info for a service
type ServiceDebugInfo struct {
	Name          string            `json:"name"`
	ContainerID   string            `json:"container_id"`
	Image         string            `json:"image"`
	Status        string            `json:"status"`
	State         string            `json:"state"`
	Health        string            `json:"health"`
	CreatedAt     time.Time         `json:"created_at"`
	StartedAt     time.Time         `json:"started_at"`
	RestartCount  int               `json:"restart_count"`
	Ports         map[string]string `json:"ports"`
	Environment   []string          `json:"environment"`
	Mounts        []string          `json:"mounts"`
	Networks      []string          `json:"networks"`
	ResourceUsage ResourceInfo      `json:"resource_usage"`
	LastLogs      []string          `json:"last_logs"`
}

// ResourceInfo contains resource usage information
type ResourceInfo struct {
	CPUPercent    float64 `json:"cpu_percent"`
	MemoryUsage   uint64  `json:"memory_usage"`
	MemoryLimit   uint64  `json:"memory_limit"`
	MemoryPercent float64 `json:"memory_percent"`
	NetworkRx     uint64  `json:"network_rx_bytes"`
	NetworkTx     uint64  `json:"network_tx_bytes"`
}

// ErrorInfo contains error information
type ErrorInfo struct {
	Service   string    `json:"service"`
	Error     string    `json:"error"`
	Context   string    `json:"context"`
	Timestamp time.Time `json:"timestamp"`
}

// NewDebugger creates a new debugger
func NewDebugger(cfg *config.Config) (*Debugger, error) {
	cli, err := client.NewClientWithOpts(
		client.FromEnv,
		client.WithAPIVersionNegotiation(),
	)
	if err != nil {
		return nil, err
	}

	return &Debugger{
		config: cfg,
		client: cli,
		info: &DebugInfo{
			Timestamp:  time.Now(),
			Services:   make(map[string]ServiceDebugInfo),
			RecentLogs: make(map[string][]string),
			Errors:     []ErrorInfo{},
		},
	}, nil
}

// CollectDebugInfo collects all debug information
func (d *Debugger) CollectDebugInfo(ctx context.Context, service string) error {
	// Collect system info
	d.collectSystemInfo()

	// Collect Docker info
	if err := d.collectDockerInfo(); err != nil {
		d.addError("docker", err.Error(), "collecting docker info")
	}

	// Collect LocalCloud info
	d.collectLocalCloudInfo()

	// Collect service info
	if service != "" {
		// Collect specific service
		if err := d.collectServiceInfo(service); err != nil {
			d.addError(service, err.Error(), "collecting service info")
		}
	} else {
		// Collect all services
		if err := d.collectAllServicesInfo(); err != nil {
			d.addError("services", err.Error(), "collecting services info")
		}
	}

	return nil
}

// GetDebugInfo returns the collected debug information
func (d *Debugger) GetDebugInfo() *DebugInfo {
	return d.info
}

// SaveToFile saves debug info to a file
func (d *Debugger) SaveToFile(filename string) error {
	data, err := json.MarshalIndent(d.info, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal debug info: %w", err)
	}

	return os.WriteFile(filename, data, 0644)
}

// collectSystemInfo collects system information
func (d *Debugger) collectSystemInfo() {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	// Get environment variables (filter sensitive ones)
	env := make(map[string]string)
	for _, e := range os.Environ() {
		parts := strings.SplitN(e, "=", 2)
		if len(parts) == 2 {
			key := parts[0]
			// Filter out sensitive environment variables
			if !strings.Contains(strings.ToLower(key), "key") &&
				!strings.Contains(strings.ToLower(key), "token") &&
				!strings.Contains(strings.ToLower(key), "password") &&
				!strings.Contains(strings.ToLower(key), "secret") {
				env[key] = parts[1]
			}
		}
	}

	// Get memory and disk info in a cross-platform way
	memInfo := getSystemMemoryInfo()
	diskInfo := getSystemDiskInfo()

	d.info.System = SystemInfo{
		OS:          runtime.GOOS,
		Arch:        runtime.GOARCH,
		CPUs:        runtime.NumCPU(),
		GoVersion:   runtime.Version(),
		Memory:      memInfo,
		Disk:        diskInfo,
		Environment: env,
	}
}

// collectDockerInfo collects Docker information
func (d *Debugger) collectDockerInfo() error {
	ctx := context.Background()

	// Get Docker version
	version, err := d.client.ServerVersion(ctx)
	if err != nil {
		return err
	}

	// Get Docker info
	info, err := d.client.Info(ctx)
	if err != nil {
		return err
	}

	// Count images
	images, err := d.client.ImageList(ctx, types.ImageListOptions{})
	if err != nil {
		return err
	}

	// Count containers
	containers, err := d.client.ContainerList(ctx, types.ContainerListOptions{All: true})
	if err != nil {
		return err
	}

	runningCount := 0
	for _, c := range containers {
		if c.State == "running" {
			runningCount++
		}
	}

	d.info.Docker = DockerInfo{
		Version:       info.ServerVersion,
		APIVersion:    version.APIVersion,
		ServerVersion: version,
		Images:        len(images),
		Containers:    len(containers),
		RunningCount:  runningCount,
		SystemInfo:    info,
	}

	return nil
}

// collectLocalCloudInfo collects LocalCloud information
func (d *Debugger) collectLocalCloudInfo() {
	d.info.LocalCloud = LocalCloudInfo{
		Version:       "0.1.0", // TODO: Get from version constant
		ProjectName:   d.config.Project.Name,
		ProjectType:   d.config.Project.Type,
		ConfigPath:    ".localcloud/config.yaml",
		Configuration: d.config,
	}
}

// collectServiceInfo collects information for a specific service
func (d *Debugger) collectServiceInfo(service string) error {
	ctx := context.Background()

	// Find container for service
	filterArgs := filters.NewArgs()
	filterArgs.Add("label", fmt.Sprintf("com.localcloud.project=%s", d.config.Project.Name))
	filterArgs.Add("label", fmt.Sprintf("com.localcloud.service=%s", service))

	containers, err := d.client.ContainerList(ctx, types.ContainerListOptions{
		All:     true,
		Filters: filterArgs,
	})
	if err != nil {
		return err
	}

	if len(containers) == 0 {
		return fmt.Errorf("no container found for service %s", service)
	}

	container := containers[0]
	debugInfo, err := d.getServiceDebugInfo(container.ID, service)
	if err != nil {
		return err
	}

	d.info.Services[service] = debugInfo
	return nil
}

// collectAllServicesInfo collects information for all services
func (d *Debugger) collectAllServicesInfo() error {
	ctx := context.Background()

	// List all LocalCloud containers
	filterArgs := filters.NewArgs()
	filterArgs.Add("label", fmt.Sprintf("com.localcloud.project=%s", d.config.Project.Name))

	containers, err := d.client.ContainerList(ctx, types.ContainerListOptions{
		All:     true,
		Filters: filterArgs,
	})
	if err != nil {
		return err
	}

	for _, container := range containers {
		// Extract service name from labels
		serviceName := container.Labels["com.localcloud.service"]
		if serviceName == "" {
			// Try to extract from container name
			name := strings.TrimPrefix(container.Names[0], "/")
			parts := strings.Split(name, "-")
			if len(parts) >= 2 {
				serviceName = parts[1]
			} else {
				serviceName = name
			}
		}

		debugInfo, err := d.getServiceDebugInfo(container.ID, serviceName)
		if err != nil {
			d.addError(serviceName, err.Error(), "collecting service debug info")
			continue
		}

		d.info.Services[serviceName] = debugInfo
	}

	return nil
}

// getServiceDebugInfo gets debug information for a service
func (d *Debugger) getServiceDebugInfo(containerID, serviceName string) (ServiceDebugInfo, error) {
	ctx := context.Background()

	// Get container inspection data
	inspect, err := d.client.ContainerInspect(ctx, containerID)
	if err != nil {
		return ServiceDebugInfo{}, err
	}

	// Basic info
	info := ServiceDebugInfo{
		Name:         serviceName,
		ContainerID:  containerID,
		Image:        inspect.Config.Image,
		Status:       inspect.State.Status,
		State:        inspect.State.Status,
		RestartCount: inspect.RestartCount,
		Environment:  inspect.Config.Env,
		Networks:     []string{},
	}

	// Parse timestamps
	if createdAt, err := time.Parse(time.RFC3339Nano, inspect.Created); err == nil {
		info.CreatedAt = createdAt
	}
	if startedAt, err := time.Parse(time.RFC3339Nano, inspect.State.StartedAt); err == nil {
		info.StartedAt = startedAt
	}

	// Health status
	if inspect.State.Health != nil {
		info.Health = inspect.State.Health.Status
	}

	// Port mappings
	info.Ports = make(map[string]string)
	for port, bindings := range inspect.NetworkSettings.Ports {
		if len(bindings) > 0 {
			info.Ports[string(port)] = fmt.Sprintf("%s:%s", bindings[0].HostIP, bindings[0].HostPort)
		}
	}

	// Mounts
	for _, mount := range inspect.Mounts {
		info.Mounts = append(info.Mounts, fmt.Sprintf("%s:%s", mount.Source, mount.Destination))
	}

	// Networks
	for network := range inspect.NetworkSettings.Networks {
		info.Networks = append(info.Networks, network)
	}

	// Get resource usage
	stats, err := d.client.ContainerStats(ctx, containerID, false)
	if err == nil {
		defer stats.Body.Close()
		var containerStats types.Stats
		if err := json.NewDecoder(stats.Body).Decode(&containerStats); err == nil {
			info.ResourceUsage = d.calculateResourceUsage(containerStats)
		}
	}

	// Get recent logs
	logsOpts := types.ContainerLogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Tail:       "20",
		Timestamps: true,
	}

	logsReader, err := d.client.ContainerLogs(ctx, containerID, logsOpts)
	if err == nil {
		defer logsReader.Close()
		logs := []string{}
		scanner := bufio.NewScanner(logsReader)
		for scanner.Scan() {
			logs = append(logs, scanner.Text())
		}
		info.LastLogs = logs
		d.info.RecentLogs[serviceName] = logs
	}

	return info, nil
}

// calculateResourceUsage calculates resource usage from container stats
func (d *Debugger) calculateResourceUsage(stats types.Stats) ResourceInfo {
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

	// Network stats - Docker API doesn't have Networks field directly in Stats
	// Network stats are usually in a different API call or structure
	var networkRx, networkTx uint64
	// For now, we'll leave network stats at 0 as they require a different approach

	return ResourceInfo{
		CPUPercent:    cpuPercent,
		MemoryUsage:   stats.MemoryStats.Usage,
		MemoryLimit:   stats.MemoryStats.Limit,
		MemoryPercent: memoryPercent,
		NetworkRx:     networkRx,
		NetworkTx:     networkTx,
	}
}

// addError adds an error to the debug info
func (d *Debugger) addError(service, errMsg, context string) {
	d.info.Errors = append(d.info.Errors, ErrorInfo{
		Service:   service,
		Error:     errMsg,
		Context:   context,
		Timestamp: time.Now(),
	})
}
