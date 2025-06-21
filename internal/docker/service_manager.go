// internal/docker/service_manager.go
package docker

import (
	"fmt"
	"strings"
	"time"
)

// ServiceManager handles service-specific operations
type ServiceManager struct {
	manager *Manager
}

// NewServiceManager creates a new service manager
func NewServiceManager(m *Manager) *ServiceManager {
	return &ServiceManager{manager: m}
}

// ServiceProgress represents service operation progress
type ServiceProgress struct {
	Service string
	Status  string
	Error   string
}

// ServiceStatus represents the status of a service
type ServiceStatus struct {
	Name        string
	Status      string
	Health      string
	Port        string
	CPUPercent  float64
	MemoryUsage uint64
	MemoryLimit uint64
}

// StartAll starts all configured services
func (sm *ServiceManager) StartAll(progress chan<- ServiceProgress) error {
	defer close(progress)

	fmt.Println("DEBUG: ServiceManager.StartAll called")

	// Initialize project resources
	fmt.Println("DEBUG: Initializing project resources...")
	if err := sm.manager.InitializeProject(); err != nil {
		fmt.Printf("DEBUG: InitializeProject failed: %v\n", err)
		return err
	}

	// Start services in order
	services := sm.getServiceOrder()
	fmt.Printf("DEBUG: Services to start: %v\n", services)

	var lastError error
	for _, service := range services {
		fmt.Printf("DEBUG: Starting service: %s\n", service)

		progress <- ServiceProgress{
			Service: service,
			Status:  "starting",
		}

		if err := sm.startService(service); err != nil {
			fmt.Printf("DEBUG: Service %s failed with error: %v\n", service, err)
			progress <- ServiceProgress{
				Service: service,
				Status:  "failed",
				Error:   err.Error(),
			}
			lastError = err
			// Continue with other services instead of returning immediately
			continue
		}

		fmt.Printf("DEBUG: Service %s started successfully\n", service)
		progress <- ServiceProgress{
			Service: service,
			Status:  "started",
		}
	}

	if lastError != nil {
		fmt.Printf("DEBUG: StartAll completed with errors. Last error: %v\n", lastError)
	} else {
		fmt.Println("DEBUG: StartAll completed successfully")
	}

	return lastError
}

// StopAll stops all services
func (sm *ServiceManager) StopAll(progress chan<- ServiceProgress) error {
	defer close(progress)

	// Get running containers
	containers, err := sm.manager.container.List(map[string]string{
		"label": "com.localcloud.project=" + sm.manager.config.Project.Name,
	})
	if err != nil {
		return err
	}

	// Stop in reverse order
	for i := len(containers) - 1; i >= 0; i-- {
		container := containers[i]
		serviceName := getServiceFromContainer(container.Name)

		progress <- ServiceProgress{
			Service: serviceName,
			Status:  "stopping",
		}

		if err := sm.manager.container.Stop(container.ID, 10); err != nil {
			progress <- ServiceProgress{
				Service: serviceName,
				Status:  "failed",
				Error:   err.Error(),
			}
			// Continue stopping other services
		} else {
			progress <- ServiceProgress{
				Service: serviceName,
				Status:  "stopped",
			}
		}
	}

	return nil
}

// GetStatus returns the status of all services
func (sm *ServiceManager) GetStatus() ([]ServiceStatus, error) {
	containers, err := sm.manager.container.List(map[string]string{
		"label": "com.localcloud.project=" + sm.manager.config.Project.Name,
	})
	if err != nil {
		return nil, err
	}

	var statuses []ServiceStatus
	for _, container := range containers {
		serviceName := getServiceFromContainer(container.Name)

		status := ServiceStatus{
			Name:   serviceName,
			Status: container.State,
			Health: container.Health,
		}

		// Get container stats if running
		if container.State == "running" {
			stats, err := sm.manager.container.Stats(container.ID)
			if err == nil {
				status.CPUPercent = stats.CPUPercent
				status.MemoryUsage = stats.MemoryUsage
				status.MemoryLimit = stats.MemoryLimit
			}
		}

		// Extract port mappings
		for port, binding := range container.Ports {
			if strings.Contains(port, "/tcp") {
				status.Port = strings.TrimSuffix(binding, ":"+strings.Split(port, "/")[0])
				break
			}
		}

		statuses = append(statuses, status)
	}

	return statuses, nil
}

// startService starts a specific service
func (sm *ServiceManager) startService(service string) error {
	fmt.Printf("DEBUG: startService called for: %s\n", service)

	switch service {
	case "ai":
		starter := NewAIServiceStarter(sm.manager)
		fmt.Println("DEBUG: Created AIServiceStarter")
		return starter.Start()
	case "database":
		starter := NewDatabaseServiceStarter(sm.manager)
		fmt.Println("DEBUG: Created DatabaseServiceStarter")
		return starter.Start()
	case "cache":
		starter := NewCacheServiceStarter(sm.manager)
		fmt.Println("DEBUG: Created CacheServiceStarter")
		return starter.Start()
	case "storage":
		starter := NewStorageServiceStarter(sm.manager)
		fmt.Println("DEBUG: Created StorageServiceStarter")
		return starter.Start()
	default:
		return fmt.Errorf("unknown service: %s", service)
	}
}

// getServiceOrder returns the order in which services should be started
func (sm *ServiceManager) getServiceOrder() []string {
	var services []string

	// Database first
	if sm.manager.config.Services.Database.Type != "" {
		services = append(services, "database")
	}

	// Cache second
	if sm.manager.config.Services.Cache.Type != "" {
		services = append(services, "cache")
	}

	// Storage third
	if sm.manager.config.Services.Storage.Type != "" {
		services = append(services, "storage")
	}

	// AI last (may depend on other services)
	services = append(services, "ai")

	return services
}

// getServiceFromContainer extracts service name from container name
func getServiceFromContainer(containerName string) string {
	// Container names are like: localcloud-ai, localcloud-postgres
	parts := strings.Split(containerName, "-")
	if len(parts) >= 2 {
		return parts[1]
	}
	return containerName
}

// WaitForService waits for a specific service to be ready
func (sm *ServiceManager) WaitForService(service string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		containers, err := sm.manager.container.List(map[string]string{
			"label": fmt.Sprintf("com.localcloud.service=%s", service),
		})
		if err != nil {
			return err
		}

		for _, container := range containers {
			if container.State == "running" && (container.Health == "" || container.Health == "healthy") {
				return nil
			}
		}

		time.Sleep(1 * time.Second)
	}

	return fmt.Errorf("timeout waiting for service %s", service)
}
