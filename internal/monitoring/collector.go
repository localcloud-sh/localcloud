// internal/monitoring/collector.go
package monitoring

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
	"github.com/localcloud-sh/localcloud/internal/config"
)

// Metrics represents collected metrics for a service
type Metrics struct {
	Service     string                 `json:"service"`
	ContainerID string                 `json:"container_id"`
	Timestamp   time.Time              `json:"timestamp"`
	CPU         CPUMetrics             `json:"cpu"`
	Memory      MemoryMetrics          `json:"memory"`
	Network     NetworkMetrics         `json:"network"`
	Disk        DiskMetrics            `json:"disk"`
	Performance map[string]interface{} `json:"performance,omitempty"`
}

// CPUMetrics represents CPU usage metrics
type CPUMetrics struct {
	UsagePercent float64 `json:"usage_percent"`
	SystemCPU    uint64  `json:"system_cpu"`
	OnlineCPUs   int     `json:"online_cpus"`
}

// MemoryMetrics represents memory usage metrics
type MemoryMetrics struct {
	Usage      uint64  `json:"usage"`
	Limit      uint64  `json:"limit"`
	Percent    float64 `json:"percent"`
	Cache      uint64  `json:"cache"`
	WorkingSet uint64  `json:"working_set"`
}

// NetworkMetrics represents network I/O metrics
type NetworkMetrics struct {
	RxBytes   uint64 `json:"rx_bytes"`
	TxBytes   uint64 `json:"tx_bytes"`
	RxPackets uint64 `json:"rx_packets"`
	TxPackets uint64 `json:"tx_packets"`
	RxDropped uint64 `json:"rx_dropped"`
	TxDropped uint64 `json:"tx_dropped"`
}

// DiskMetrics represents disk usage metrics
type DiskMetrics struct {
	ReadBytes  uint64 `json:"read_bytes"`
	WriteBytes uint64 `json:"write_bytes"`
	ReadOps    uint64 `json:"read_ops"`
	WriteOps   uint64 `json:"write_ops"`
}

// Collector collects metrics from containers
type Collector struct {
	client       *client.Client
	config       *config.Config
	mu           sync.RWMutex
	metrics      map[string]*Metrics
	collectors   map[string]context.CancelFunc
	interval     time.Duration
	perfHandlers map[string]PerformanceHandler
}

// PerformanceHandler is a function that collects performance metrics for a specific service
type PerformanceHandler func(containerID string) (map[string]interface{}, error)

// NewCollector creates a new metrics collector
func NewCollector(cfg *config.Config) (*Collector, error) {
	cli, err := client.NewClientWithOpts(
		client.FromEnv,
		client.WithAPIVersionNegotiation(),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create Docker client: %w", err)
	}

	c := &Collector{
		client:       cli,
		config:       cfg,
		metrics:      make(map[string]*Metrics),
		collectors:   make(map[string]context.CancelFunc),
		interval:     5 * time.Second,
		perfHandlers: make(map[string]PerformanceHandler),
	}

	// Register performance handlers
	c.registerPerformanceHandlers()

	return c, nil
}

// Start begins metrics collection
func (c *Collector) Start(ctx context.Context) error {
	// List all LocalCloud containers
	filterArgs := filters.NewArgs()
	filterArgs.Add("label", fmt.Sprintf("com.localcloud.project=%s", c.config.Project.Name))

	containers, err := c.client.ContainerList(ctx, types.ContainerListOptions{
		All:     false, // Only running containers
		Filters: filterArgs,
	})
	if err != nil {
		return fmt.Errorf("failed to list containers: %w", err)
	}

	// Start collecting metrics from each container
	for _, container := range containers {
		serviceName := c.getServiceName(container)
		c.startCollector(ctx, container.ID, serviceName)
	}

	return nil
}

// Stop stops all metric collectors
func (c *Collector) Stop() {
	c.mu.Lock()
	defer c.mu.Unlock()

	for _, cancel := range c.collectors {
		cancel()
	}
}

// GetMetrics returns current metrics for all services
func (c *Collector) GetMetrics() map[string]*Metrics {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// Return a copy to avoid race conditions
	result := make(map[string]*Metrics)
	for k, v := range c.metrics {
		metricsCopy := *v
		result[k] = &metricsCopy
	}

	return result
}

// GetServiceMetrics returns metrics for a specific service
func (c *Collector) GetServiceMetrics(service string) (*Metrics, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	metrics, exists := c.metrics[service]
	if !exists {
		return nil, false
	}

	metricsCopy := *metrics
	return &metricsCopy, true
}

// startCollector starts metrics collection for a container
func (c *Collector) startCollector(ctx context.Context, containerID, serviceName string) {
	ctx, cancel := context.WithCancel(ctx)

	c.mu.Lock()
	c.collectors[containerID] = cancel
	c.mu.Unlock()

	go func() {
		ticker := time.NewTicker(c.interval)
		defer ticker.Stop()
		defer func() {
			c.mu.Lock()
			delete(c.collectors, containerID)
			delete(c.metrics, serviceName)
			c.mu.Unlock()
		}()

		// Initial collection
		c.collectMetrics(containerID, serviceName)

		for {
			select {
			case <-ticker.C:
				c.collectMetrics(containerID, serviceName)
			case <-ctx.Done():
				return
			}
		}
	}()
}

// collectMetrics collects metrics for a single container
func (c *Collector) collectMetrics(containerID, serviceName string) {
	metrics := &Metrics{
		Service:     serviceName,
		ContainerID: containerID[:12],
		Timestamp:   time.Now(),
	}

	// Get container stats
	statsResp, err := c.client.ContainerStats(context.Background(), containerID, false)
	if err != nil {
		return
	}
	defer statsResp.Body.Close()

	var stats types.Stats
	if err := json.NewDecoder(statsResp.Body).Decode(&stats); err != nil {
		return
	}

	// Calculate CPU metrics
	metrics.CPU = c.calculateCPUMetrics(&stats)

	// Calculate memory metrics
	metrics.Memory = c.calculateMemoryMetrics(&stats)

	// Calculate network metrics
	metrics.Network = c.calculateNetworkMetrics(&stats)

	// Calculate disk metrics
	metrics.Disk = c.calculateDiskMetrics(&stats)

	// Collect service-specific performance metrics
	if handler, exists := c.perfHandlers[serviceName]; exists {
		if perf, err := handler(containerID); err == nil {
			metrics.Performance = perf
		}
	}

	// Store metrics
	c.mu.Lock()
	c.metrics[serviceName] = metrics
	c.mu.Unlock()
}

// calculateCPUMetrics calculates CPU usage percentage
func (c *Collector) calculateCPUMetrics(stats *types.Stats) CPUMetrics {
	cpuDelta := float64(stats.CPUStats.CPUUsage.TotalUsage - stats.PreCPUStats.CPUUsage.TotalUsage)
	systemDelta := float64(stats.CPUStats.SystemUsage - stats.PreCPUStats.SystemUsage)
	onlineCPUs := len(stats.CPUStats.CPUUsage.PercpuUsage)

	cpuPercent := 0.0
	if systemDelta > 0.0 && cpuDelta > 0.0 {
		cpuPercent = (cpuDelta / systemDelta) * float64(onlineCPUs) * 100.0
	}

	return CPUMetrics{
		UsagePercent: cpuPercent,
		SystemCPU:    stats.CPUStats.SystemUsage,
		OnlineCPUs:   onlineCPUs,
	}
}

// calculateMemoryMetrics calculates memory usage
func (c *Collector) calculateMemoryMetrics(stats *types.Stats) MemoryMetrics {
	usage := stats.MemoryStats.Usage
	limit := stats.MemoryStats.Limit
	cache := stats.MemoryStats.Stats["cache"]

	// Working set is usage minus cache
	workingSet := usage
	if cache > 0 && usage > cache {
		workingSet = usage - cache
	}

	percent := 0.0
	if limit > 0 {
		percent = (float64(workingSet) / float64(limit)) * 100.0
	}

	return MemoryMetrics{
		Usage:      usage,
		Limit:      limit,
		Percent:    percent,
		Cache:      cache,
		WorkingSet: workingSet,
	}
}

// calculateNetworkMetrics calculates network I/O
func (c *Collector) calculateNetworkMetrics(stats *types.Stats) NetworkMetrics {
	var metrics NetworkMetrics

	// In newer Docker API versions, network stats might be in different locations
	// For now, return empty metrics - this would need to be updated based on
	// the actual Docker API version being used

	// Note: The exact location of network stats varies by Docker API version
	// You may need to check stats.Networks (if it's a map[string]types.NetworkStats)
	// or other fields depending on your Docker version

	return metrics
}

// calculateDiskMetrics calculates disk I/O
func (c *Collector) calculateDiskMetrics(stats *types.Stats) DiskMetrics {
	var metrics DiskMetrics

	// Aggregate all block devices
	for _, blkio := range stats.BlkioStats.IoServiceBytesRecursive {
		switch blkio.Op {
		case "Read":
			metrics.ReadBytes += blkio.Value
		case "Write":
			metrics.WriteBytes += blkio.Value
		}
	}

	for _, blkio := range stats.BlkioStats.IoServicedRecursive {
		switch blkio.Op {
		case "Read":
			metrics.ReadOps += blkio.Value
		case "Write":
			metrics.WriteOps += blkio.Value
		}
	}

	return metrics
}

// registerPerformanceHandlers registers service-specific performance collectors
func (c *Collector) registerPerformanceHandlers() {
	// AI service performance
	c.perfHandlers["ai"] = func(containerID string) (map[string]interface{}, error) {
		// In a real implementation, this would query the Ollama API
		// For now, return mock data
		return map[string]interface{}{
			"models_loaded":   1,
			"inference_speed": "450 tokens/sec",
			"active_sessions": 2,
			"queue_size":      0,
		}, nil
	}

	// PostgreSQL performance
	c.perfHandlers["postgres"] = func(containerID string) (map[string]interface{}, error) {
		// In a real implementation, this would run pg_stat queries
		return map[string]interface{}{
			"connections":      5,
			"active_queries":   1,
			"cache_hit_ratio":  0.98,
			"transactions_sec": 120,
		}, nil
	}

	// Redis performance
	c.perfHandlers["redis"] = func(containerID string) (map[string]interface{}, error) {
		// In a real implementation, this would query Redis INFO
		return map[string]interface{}{
			"keys":              1523,
			"cache_hit_rate":    0.95,
			"ops_per_sec":       3500,
			"connected_clients": 3,
		}, nil
	}

	// MinIO performance
	c.perfHandlers["minio"] = func(containerID string) (map[string]interface{}, error) {
		return map[string]interface{}{
			"buckets":       2,
			"objects":       156,
			"total_size_mb": 234,
			"api_calls":     1250,
		}, nil
	}
}

// getServiceName extracts service name from container
func (c *Collector) getServiceName(container types.Container) string {
	if service, ok := container.Labels["com.localcloud.service"]; ok {
		return service
	}

	name := strings.TrimPrefix(container.Names[0], "/")
	parts := strings.Split(name, "-")
	if len(parts) >= 2 {
		return parts[1]
	}

	return name
}
