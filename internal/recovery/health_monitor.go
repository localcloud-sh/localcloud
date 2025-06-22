// internal/recovery/health_monitor.go
package recovery

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"syscall"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/localcloud/localcloud/internal/config"
	"github.com/localcloud/localcloud/internal/docker"
)

// HealthMonitor monitors service health and triggers recovery
type HealthMonitor struct {
	client         *client.Client
	config         *config.Config
	restartManager *RestartManager
	mu             sync.RWMutex
	monitors       map[string]*ServiceMonitor
	alerts         chan HealthAlert
}

// ServiceMonitor monitors a specific service
type ServiceMonitor struct {
	Service          string
	ContainerID      string
	HealthCheckFunc  HealthCheckFunc
	Interval         time.Duration
	Timeout          time.Duration
	FailureThreshold int
	consecutiveFails int
	lastCheck        time.Time
	lastStatus       HealthStatus
	cancel           context.CancelFunc
}

// HealthCheckFunc is a function that performs health check
type HealthCheckFunc func(containerID string) error

// HealthStatus represents the health status of a service
type HealthStatus struct {
	Healthy       bool
	Message       string
	LastCheck     time.Time
	ResponseTime  time.Duration
	ResourceUsage ResourceUsage
}

// ResourceUsage represents resource usage metrics
type ResourceUsage struct {
	CPUPercent    float64
	MemoryPercent float64
	MemoryUsage   uint64
	MemoryLimit   uint64
	DiskUsage     uint64
	NetworkErrors uint64
}

// HealthAlert represents a health alert
type HealthAlert struct {
	Service   string
	Type      AlertType
	Message   string
	Severity  AlertSeverity
	Timestamp time.Time
	Action    string
}

// AlertType represents the type of alert
type AlertType string

const (
	AlertTypeHealthCheck     AlertType = "health_check"
	AlertTypeResourceLimit   AlertType = "resource_limit"
	AlertTypeRestartRequired AlertType = "restart_required"
	AlertTypeRecoveryFailed  AlertType = "recovery_failed"
)

// AlertSeverity represents alert severity
type AlertSeverity string

const (
	AlertSeverityInfo     AlertSeverity = "info"
	AlertSeverityWarning  AlertSeverity = "warning"
	AlertSeverityCritical AlertSeverity = "critical"
)

// NewHealthMonitor creates a new health monitor
func NewHealthMonitor(cfg *config.Config, restartMgr *RestartManager) (*HealthMonitor, error) {
	cli, err := client.NewClientWithOpts(
		client.FromEnv,
		client.WithAPIVersionNegotiation(),
	)
	if err != nil {
		return nil, err
	}

	return &HealthMonitor{
		client:         cli,
		config:         cfg,
		restartManager: restartMgr,
		monitors:       make(map[string]*ServiceMonitor),
		alerts:         make(chan HealthAlert, 100),
	}, nil
}

// RegisterService registers a service for health monitoring
func (hm *HealthMonitor) RegisterService(service, containerID string, checkFunc HealthCheckFunc) {
	hm.mu.Lock()
	defer hm.mu.Unlock()

	monitor := &ServiceMonitor{
		Service:          service,
		ContainerID:      containerID,
		HealthCheckFunc:  checkFunc,
		Interval:         30 * time.Second,
		Timeout:          10 * time.Second,
		FailureThreshold: 3,
		lastStatus: HealthStatus{
			Healthy:   true,
			LastCheck: time.Now(),
		},
	}

	// Use default health check if none provided
	if checkFunc == nil {
		monitor.HealthCheckFunc = hm.defaultHealthCheck
	}

	hm.monitors[service] = monitor
}

// Start starts health monitoring for all registered services
func (hm *HealthMonitor) Start(ctx context.Context) {
	hm.mu.Lock()
	defer hm.mu.Unlock()

	for service, monitor := range hm.monitors {
		monitorCtx, cancel := context.WithCancel(ctx)
		monitor.cancel = cancel
		go hm.monitorService(monitorCtx, service)
	}

	// Start resource monitoring
	go hm.monitorSystemResources(ctx)
}

// Stop stops all health monitors
func (hm *HealthMonitor) Stop() {
	hm.mu.Lock()
	defer hm.mu.Unlock()

	for _, monitor := range hm.monitors {
		if monitor.cancel != nil {
			monitor.cancel()
		}
	}
}

// monitorService monitors a single service
func (hm *HealthMonitor) monitorService(ctx context.Context, service string) {
	hm.mu.RLock()
	monitor := hm.monitors[service]
	hm.mu.RUnlock()

	ticker := time.NewTicker(monitor.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			hm.performHealthCheck(monitor)
		}
	}
}

// performHealthCheck performs a health check on a service
func (hm *HealthMonitor) performHealthCheck(monitor *ServiceMonitor) {
	start := time.Now()

	// Create timeout context
	ctx, cancel := context.WithTimeout(context.Background(), monitor.Timeout)
	defer cancel()

	// Run health check in goroutine
	checkDone := make(chan error, 1)
	go func() {
		checkDone <- monitor.HealthCheckFunc(monitor.ContainerID)
	}()

	var err error
	select {
	case <-ctx.Done():
		err = fmt.Errorf("health check timeout after %v", monitor.Timeout)
	case err = <-checkDone:
	}

	responseTime := time.Since(start)
	monitor.lastCheck = time.Now()

	// Get resource usage
	resourceUsage := hm.getResourceUsage(monitor.ContainerID)

	// Update status
	if err != nil {
		monitor.consecutiveFails++
		monitor.lastStatus = HealthStatus{
			Healthy:       false,
			Message:       err.Error(),
			LastCheck:     monitor.lastCheck,
			ResponseTime:  responseTime,
			ResourceUsage: resourceUsage,
		}

		// Check if we've hit the failure threshold
		if monitor.consecutiveFails >= monitor.FailureThreshold {
			hm.handleUnhealthyService(monitor)
		} else {
			hm.sendAlert(HealthAlert{
				Service:   monitor.Service,
				Type:      AlertTypeHealthCheck,
				Message:   fmt.Sprintf("Health check failed (%d/%d): %v", monitor.consecutiveFails, monitor.FailureThreshold, err),
				Severity:  AlertSeverityWarning,
				Timestamp: time.Now(),
			})
		}
	} else {
		// Service is healthy
		if monitor.consecutiveFails > 0 {
			// Service recovered
			hm.sendAlert(HealthAlert{
				Service:   monitor.Service,
				Type:      AlertTypeHealthCheck,
				Message:   "Service recovered",
				Severity:  AlertSeverityInfo,
				Timestamp: time.Now(),
			})
		}

		monitor.consecutiveFails = 0
		monitor.lastStatus = HealthStatus{
			Healthy:       true,
			Message:       "OK",
			LastCheck:     monitor.lastCheck,
			ResponseTime:  responseTime,
			ResourceUsage: resourceUsage,
		}

		// Check resource usage
		hm.checkResourceLimits(monitor.Service, resourceUsage)
	}

	hm.mu.Lock()
	hm.monitors[monitor.Service] = monitor
	hm.mu.Unlock()
}

// handleUnhealthyService handles an unhealthy service
func (hm *HealthMonitor) handleUnhealthyService(monitor *ServiceMonitor) {
	hm.sendAlert(HealthAlert{
		Service:   monitor.Service,
		Type:      AlertTypeRestartRequired,
		Message:   fmt.Sprintf("Service unhealthy after %d consecutive failures", monitor.consecutiveFails),
		Severity:  AlertSeverityCritical,
		Timestamp: time.Now(),
		Action:    "restart",
	})

	// Trigger restart through restart manager
	// This would integrate with the restart manager
}

// defaultHealthCheck provides a default health check
func (hm *HealthMonitor) defaultHealthCheck(containerID string) error {
	// Check if container is running
	inspect, err := hm.client.ContainerInspect(context.Background(), containerID)
	if err != nil {
		return fmt.Errorf("failed to inspect container: %w", err)
	}

	if !inspect.State.Running {
		return fmt.Errorf("container is not running")
	}

	// Check if container has health check and it's passing
	if inspect.State.Health != nil {
		if inspect.State.Health.Status != "healthy" {
			return fmt.Errorf("container health check status: %s", inspect.State.Health.Status)
		}
	}

	return nil
}

// getResourceUsage gets resource usage for a container
func (hm *HealthMonitor) getResourceUsage(containerID string) ResourceUsage {
	stats, err := hm.client.ContainerStats(context.Background(), containerID, false)
	if err != nil {
		return ResourceUsage{}
	}
	defer stats.Body.Close()

	var containerStats types.Stats
	if err := json.NewDecoder(stats.Body).Decode(&containerStats); err != nil {
		return ResourceUsage{}
	}

	// Calculate CPU percentage
	cpuDelta := float64(containerStats.CPUStats.CPUUsage.TotalUsage - containerStats.PreCPUStats.CPUUsage.TotalUsage)
	systemDelta := float64(containerStats.CPUStats.SystemUsage - containerStats.PreCPUStats.SystemUsage)
	cpuPercent := 0.0
	if systemDelta > 0.0 && cpuDelta > 0.0 {
		cpuPercent = (cpuDelta / systemDelta) * float64(len(containerStats.CPUStats.CPUUsage.PercpuUsage)) * 100.0
	}

	// Calculate memory percentage
	memoryPercent := 0.0
	if containerStats.MemoryStats.Limit > 0 {
		memoryPercent = (float64(containerStats.MemoryStats.Usage) / float64(containerStats.MemoryStats.Limit)) * 100.0
	}

	return ResourceUsage{
		CPUPercent:    cpuPercent,
		MemoryPercent: memoryPercent,
		MemoryUsage:   containerStats.MemoryStats.Usage,
		MemoryLimit:   containerStats.MemoryStats.Limit,
	}
}

// checkResourceLimits checks if resource usage is within limits
func (hm *HealthMonitor) checkResourceLimits(service string, usage ResourceUsage) {
	// Check memory usage
	if usage.MemoryPercent > 90 {
		hm.sendAlert(HealthAlert{
			Service:   service,
			Type:      AlertTypeResourceLimit,
			Message:   fmt.Sprintf("High memory usage: %.1f%%", usage.MemoryPercent),
			Severity:  AlertSeverityCritical,
			Timestamp: time.Now(),
		})
	} else if usage.MemoryPercent > 80 {
		hm.sendAlert(HealthAlert{
			Service:   service,
			Type:      AlertTypeResourceLimit,
			Message:   fmt.Sprintf("Memory usage warning: %.1f%%", usage.MemoryPercent),
			Severity:  AlertSeverityWarning,
			Timestamp: time.Now(),
		})
	}

	// Check CPU usage
	if usage.CPUPercent > 90 {
		hm.sendAlert(HealthAlert{
			Service:   service,
			Type:      AlertTypeResourceLimit,
			Message:   fmt.Sprintf("High CPU usage: %.1f%%", usage.CPUPercent),
			Severity:  AlertSeverityWarning,
			Timestamp: time.Now(),
		})
	}
}

// monitorSystemResources monitors overall system resources
func (hm *HealthMonitor) monitorSystemResources(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			hm.checkSystemResources()
		}
	}
}

// checkSystemResources checks system-wide resources
func (hm *HealthMonitor) checkSystemResources() {
	// Check disk space
	if usage, err := getDiskUsage("/"); err == nil {
		if usage.PercentUsed > 90 {
			hm.sendAlert(HealthAlert{
				Service:   "system",
				Type:      AlertTypeResourceLimit,
				Message:   fmt.Sprintf("Critical: Disk space low (%.1f%% used)", usage.PercentUsed),
				Severity:  AlertSeverityCritical,
				Timestamp: time.Now(),
			})
		} else if usage.PercentUsed > 80 {
			hm.sendAlert(HealthAlert{
				Service:   "system",
				Type:      AlertTypeResourceLimit,
				Message:   fmt.Sprintf("Warning: Disk space low (%.1f%% used)", usage.PercentUsed),
				Severity:  AlertSeverityWarning,
				Timestamp: time.Now(),
			})
		}
	}
}

// sendAlert sends a health alert
func (hm *HealthMonitor) sendAlert(alert HealthAlert) {
	select {
	case hm.alerts <- alert:
	default:
		// Channel full, drop oldest
		<-hm.alerts
		hm.alerts <- alert
	}
}

// GetAlerts returns the alerts channel
func (hm *HealthMonitor) GetAlerts() <-chan HealthAlert {
	return hm.alerts
}

// GetStatus returns the current health status of all services
func (hm *HealthMonitor) GetStatus() map[string]HealthStatus {
	hm.mu.RLock()
	defer hm.mu.RUnlock()

	status := make(map[string]HealthStatus)
	for service, monitor := range hm.monitors {
		status[service] = monitor.lastStatus
	}

	return status
}

// DiskUsage represents disk usage information
type DiskUsage struct {
	Total       uint64
	Used        uint64
	Free        uint64
	PercentUsed float64
}

// getDiskUsage gets disk usage for a path
func getDiskUsage(path string) (*DiskUsage, error) {
	fs := syscall.Statfs_t{}
	err := syscall.Statfs(path, &fs)
	if err != nil {
		return nil, err
	}

	total := fs.Blocks * uint64(fs.Bsize)
	free := fs.Bfree * uint64(fs.Bsize)
	used := total - free
	percentUsed := float64(used) / float64(total) * 100

	return &DiskUsage{
		Total:       total,
		Used:        used,
		Free:        free,
		PercentUsed: percentUsed,
	}, nil
}
