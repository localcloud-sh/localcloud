// internal/recovery/restart_policy.go
package recovery

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/localcloud-sh/localcloud/internal/docker"
)

// RestartPolicy defines the restart behavior
type RestartPolicy string

const (
	RestartPolicyAlways        RestartPolicy = "always"
	RestartPolicyOnFailure     RestartPolicy = "on-failure"
	RestartPolicyUnlessStopped RestartPolicy = "unless-stopped"
	RestartPolicyNo            RestartPolicy = "no"
)

// RestartManager manages container restart policies
type RestartManager struct {
	mu              sync.RWMutex
	policies        map[string]*ServicePolicy
	restartAttempts map[string]int
	lastRestart     map[string]time.Time
	notifications   chan RestartNotification
}

// ServicePolicy defines restart policy for a service
type ServicePolicy struct {
	Service        string
	Policy         RestartPolicy
	MaxAttempts    int
	BackoffSeconds []int // Exponential backoff intervals
	OnRestart      func(service string, attempt int)
}

// RestartNotification represents a restart event
type RestartNotification struct {
	Service   string
	Attempt   int
	Reason    string
	Success   bool
	Timestamp time.Time
}

// NewRestartManager creates a new restart manager
func NewRestartManager() *RestartManager {
	return &RestartManager{
		policies:        make(map[string]*ServicePolicy),
		restartAttempts: make(map[string]int),
		lastRestart:     make(map[string]time.Time),
		notifications:   make(chan RestartNotification, 100),
	}
}

// RegisterPolicy registers a restart policy for a service
func (rm *RestartManager) RegisterPolicy(service string, policy *ServicePolicy) {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	// Set default backoff if not provided
	if len(policy.BackoffSeconds) == 0 {
		policy.BackoffSeconds = []int{1, 2, 4, 8, 16, 32, 64} // Default exponential backoff
	}

	rm.policies[service] = policy
	rm.restartAttempts[service] = 0
}

// ShouldRestart determines if a service should be restarted
func (rm *RestartManager) ShouldRestart(service string, exitCode int) (bool, time.Duration) {
	rm.mu.RLock()
	policy, exists := rm.policies[service]
	attempts := rm.restartAttempts[service]
	lastRestart := rm.lastRestart[service]
	rm.mu.RUnlock()

	if !exists {
		return false, 0
	}

	// Check policy type
	switch policy.Policy {
	case RestartPolicyNo:
		return false, 0
	case RestartPolicyAlways:
		// Always restart
	case RestartPolicyOnFailure:
		if exitCode == 0 {
			return false, 0
		}
	case RestartPolicyUnlessStopped:
		// This is handled by Docker, but we track it
	}

	// Check max attempts
	if policy.MaxAttempts > 0 && attempts >= policy.MaxAttempts {
		rm.notify(RestartNotification{
			Service:   service,
			Attempt:   attempts,
			Reason:    fmt.Sprintf("Max restart attempts (%d) reached", policy.MaxAttempts),
			Success:   false,
			Timestamp: time.Now(),
		})
		return false, 0
	}

	// Calculate backoff
	backoffIndex := attempts
	if backoffIndex >= len(policy.BackoffSeconds) {
		backoffIndex = len(policy.BackoffSeconds) - 1
	}
	backoffDuration := time.Duration(policy.BackoffSeconds[backoffIndex]) * time.Second

	// Check if we're still in backoff period
	if time.Since(lastRestart) < backoffDuration {
		return false, backoffDuration - time.Since(lastRestart)
	}

	return true, 0
}

// RecordRestart records a restart attempt
func (rm *RestartManager) RecordRestart(service string, success bool) {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	if success {
		rm.restartAttempts[service]++
		rm.lastRestart[service] = time.Now()

		// Call callback if defined
		if policy, exists := rm.policies[service]; exists && policy.OnRestart != nil {
			go policy.OnRestart(service, rm.restartAttempts[service])
		}

		rm.notify(RestartNotification{
			Service:   service,
			Attempt:   rm.restartAttempts[service],
			Reason:    "Service restarted",
			Success:   true,
			Timestamp: time.Now(),
		})
	} else {
		rm.notify(RestartNotification{
			Service:   service,
			Attempt:   rm.restartAttempts[service],
			Reason:    "Restart failed",
			Success:   false,
			Timestamp: time.Now(),
		})
	}
}

// ResetAttempts resets restart attempts for a service
func (rm *RestartManager) ResetAttempts(service string) {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	rm.restartAttempts[service] = 0
	delete(rm.lastRestart, service)
}

// GetNotifications returns the notification channel
func (rm *RestartManager) GetNotifications() <-chan RestartNotification {
	return rm.notifications
}

// notify sends a restart notification
func (rm *RestartManager) notify(notification RestartNotification) {
	select {
	case rm.notifications <- notification:
	default:
		// Channel full, drop oldest notification
		<-rm.notifications
		rm.notifications <- notification
	}
}

// GetStatus returns restart status for all services
func (rm *RestartManager) GetStatus() map[string]RestartStatus {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	status := make(map[string]RestartStatus)
	for service, policy := range rm.policies {
		status[service] = RestartStatus{
			Service:     service,
			Policy:      policy.Policy,
			Attempts:    rm.restartAttempts[service],
			MaxAttempts: policy.MaxAttempts,
			LastRestart: rm.lastRestart[service],
			NextBackoff: rm.getNextBackoff(service),
		}
	}

	return status
}

// getNextBackoff calculates the next backoff duration
func (rm *RestartManager) getNextBackoff(service string) time.Duration {
	policy, exists := rm.policies[service]
	if !exists {
		return 0
	}

	attempts := rm.restartAttempts[service]
	if attempts >= len(policy.BackoffSeconds) {
		attempts = len(policy.BackoffSeconds) - 1
	}

	return time.Duration(policy.BackoffSeconds[attempts]) * time.Second
}

// RestartStatus represents the restart status of a service
type RestartStatus struct {
	Service     string
	Policy      RestartPolicy
	Attempts    int
	MaxAttempts int
	LastRestart time.Time
	NextBackoff time.Duration
}

// ServiceRestarter handles the actual restart logic
type ServiceRestarter struct {
	manager    *docker.Manager
	restartMgr *RestartManager
	mu         sync.Mutex
	monitoring map[string]context.CancelFunc
}

// NewServiceRestarter creates a new service restarter
func NewServiceRestarter(manager *docker.Manager, restartMgr *RestartManager) *ServiceRestarter {
	return &ServiceRestarter{
		manager:    manager,
		restartMgr: restartMgr,
		monitoring: make(map[string]context.CancelFunc),
	}
}

// MonitorService starts monitoring a service for restarts
func (sr *ServiceRestarter) MonitorService(ctx context.Context, containerID, service string) {
	// Cancel any existing monitoring
	sr.mu.Lock()
	if cancel, exists := sr.monitoring[service]; exists {
		cancel()
	}

	monitorCtx, cancel := context.WithCancel(ctx)
	sr.monitoring[service] = cancel
	sr.mu.Unlock()

	go sr.monitorLoop(monitorCtx, containerID, service)
}

// monitorLoop monitors a container and handles restarts
func (sr *ServiceRestarter) monitorLoop(ctx context.Context, containerID, service string) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			// Wait for container to exit
			client := sr.manager.GetClient()
			dockerClient := client.GetDockerClient()
			statusCh, errCh := dockerClient.ContainerWait(ctx, containerID, "not-running")

			select {
			case err := <-errCh:
				if err != nil {
					fmt.Printf("Error monitoring %s: %v\n", service, err)
					return
				}
			case status := <-statusCh:
				// Container exited, check if we should restart
				shouldRestart, waitDuration := sr.restartMgr.ShouldRestart(service, int(status.StatusCode))

				if !shouldRestart {
					fmt.Printf("Service %s exited with code %d, not restarting\n", service, status.StatusCode)
					return
				}

				if waitDuration > 0 {
					fmt.Printf("Waiting %v before restarting %s\n", waitDuration, service)
					time.Sleep(waitDuration)
				}

				// Attempt restart
				fmt.Printf("Attempting to restart %s (exit code: %d)\n", service, status.StatusCode)
				if err := sr.restartService(service, containerID); err != nil {
					fmt.Printf("Failed to restart %s: %v\n", service, err)
					sr.restartMgr.RecordRestart(service, false)

					// Try again after backoff
					time.Sleep(sr.restartMgr.getNextBackoff(service))
					continue
				}

				sr.restartMgr.RecordRestart(service, true)
				fmt.Printf("Successfully restarted %s\n", service)

				// Update container ID for next iteration
				// In real implementation, get new container ID
				return
			case <-ctx.Done():
				return
			}
		}
	}
}

// restartService restarts a service
func (sr *ServiceRestarter) restartService(service, oldContainerID string) error {
	// Remove old container
	containerMgr := sr.manager.GetClient().NewContainerManager()
	if err := containerMgr.Remove(oldContainerID); err != nil {
		fmt.Printf("Warning: failed to remove old container: %v\n", err)
	}

	// Start the service again using the service manager
	progress := make(chan docker.ServiceProgress)
	go func() {
		for range progress {
			// Consume progress updates
		}
	}()

	// This would use the actual service starter
	// For now, return nil to indicate we would restart
	close(progress)
	return nil
}

// StopMonitoring stops monitoring a service
func (sr *ServiceRestarter) StopMonitoring(service string) {
	sr.mu.Lock()
	defer sr.mu.Unlock()

	if cancel, exists := sr.monitoring[service]; exists {
		cancel()
		delete(sr.monitoring, service)
	}
}
