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

// StartSelectedServices starts only the specified services
func (sm *ServiceManager) StartSelectedServices(services []string, progress chan<- ServiceProgress) error {
	defer close(progress)

	fmt.Printf("DEBUG: ServiceManager.StartSelectedServices called with services: %v\n", services)

	// Initialize project resources
	fmt.Println("DEBUG: Initializing project resources...")
	if err := sm.manager.InitializeProject(); err != nil {
		fmt.Printf("DEBUG: InitializeProject failed: %v\n", err)
		return err
	}

	// Validate and normalize service names
	normalizedServices := []string{}
	for _, service := range services {
		normalized := sm.normalizeServiceName(service)
		if normalized != "" {
			normalizedServices = append(normalizedServices, normalized)
		}
	}

	fmt.Printf("DEBUG: Normalized services to start: %v\n", normalizedServices)

	var lastError error
	for _, service := range normalizedServices {
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
			continue
		}

		fmt.Printf("DEBUG: Service %s started successfully\n", service)
		progress <- ServiceProgress{
			Service: service,
			Status:  "started",
		}
	}

	if lastError != nil {
		fmt.Printf("DEBUG: StartSelectedServices completed with errors. Last error: %v\n", lastError)
	} else {
		fmt.Println("DEBUG: StartSelectedServices completed successfully")
	}

	return lastError
}

// normalizeServiceName converts various service name formats to internal names
func (sm *ServiceManager) normalizeServiceName(name string) string {
	// Handle common aliases
	switch strings.ToLower(name) {
	case "ai", "ollama", "llm":
		return "ai"
	case "db", "database", "postgres", "postgresql", "pg":
		return "postgres"
	case "cache", "redis-cache":
		return "cache"
	case "queue", "redis-queue":
		return "queue"
	case "storage", "minio", "s3":
		return "minio"
	default:
		return name
	}
}

func (sm *ServiceManager) getServiceStarter(service string) ServiceStarter {
	switch service {
	case "ai":
		return NewAIServiceStarter(sm.manager)
	case "postgres":
		return NewDatabaseServiceStarter(sm.manager)
	case "cache":
		return NewCacheServiceStarter(sm.manager)
	case "queue":
		return NewQueueServiceStarter(sm.manager)
	case "minio":
		return NewStorageServiceStarter(sm.manager)
	default:
		return nil
	}
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

// startService starts a specific service
func (sm *ServiceManager) startService(service string) error {
	fmt.Printf("DEBUG: startService called for: %s\n", service)

	switch service {
	case "ai":
		starter := NewAIServiceStarter(sm.manager)
		fmt.Println("DEBUG: Created AIServiceStarter")
		return starter.Start()
	case "postgres", "database":
		starter := NewDatabaseServiceStarter(sm.manager)
		fmt.Println("DEBUG: Created DatabaseServiceStarter")
		return starter.Start()
	case "cache":
		starter := NewCacheServiceStarter(sm.manager)
		fmt.Println("DEBUG: Created CacheServiceStarter")
		return starter.Start()
	case "queue":
		starter := NewQueueServiceStarter(sm.manager)
		fmt.Println("DEBUG: Created QueueServiceStarter")
		return starter.Start()
	case "minio", "storage":
		starter := NewStorageServiceStarter(sm.manager)
		fmt.Println("DEBUG: Created StorageServiceStarter")
		return starter.Start()
	default:
		return fmt.Errorf("unknown service: %s", service)
	}
}

// getServiceOrder returns the order in which services should be started
func (sm *ServiceManager) getServiceOrder() []string {
	services := []string{}

	// AI service first (if configured)
	if sm.manager.config.Services.AI.Port > 0 {
		services = append(services, "ai")
	}

	// Database (if configured)
	if sm.manager.config.Services.Database.Type != "" {
		services = append(services, "postgres")
	}

	// Cache service (if configured)
	if sm.manager.config.Services.Cache.Type != "" {
		services = append(services, "cache")
	}

	// Queue service (if configured)
	if sm.manager.config.Services.Queue.Type != "" {
		services = append(services, "queue")
	}

	// Storage (if configured)
	if sm.manager.config.Services.Storage.Type != "" {
		services = append(services, "minio")
	}

	return services
}

// Update getServiceFromContainer to handle new container names
func getServiceFromContainer(containerName string) string {
	// Container names are like: localcloud-ai, localcloud-postgres, localcloud-redis-cache
	containerName = strings.TrimPrefix(containerName, "/") // Remove leading slash if present
	parts := strings.Split(containerName, "-")

	if len(parts) < 2 {
		return ""
	}

	// Handle special cases
	if len(parts) >= 3 && parts[1] == "redis" {
		// localcloud-redis-cache or localcloud-redis-queue
		if parts[2] == "cache" {
			return "cache"
		} else if parts[2] == "queue" {
			return "queue"
		}
	}

	// Standard cases
	switch parts[1] {
	case "ai", "ollama":
		return "ai"
	case "postgres", "postgresql":
		return "postgres"
	case "minio":
		return "minio"
	case "redis":
		// Legacy redis (if any)
		return "cache"
	default:
		return parts[1]
	}
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
func (sm *ServiceManager) GetStatus() ([]ServiceStatus, error) {
	statuses := []ServiceStatus{}

	// Get all LocalCloud containers
	containers, err := sm.manager.container.List(map[string]string{
		"label": fmt.Sprintf("com.localcloud.project=%s", sm.manager.config.Project.Name),
	})
	if err != nil {
		return nil, err
	}

	// Map containers to services
	for _, container := range containers {
		// Extract service name from container name
		serviceName := getServiceFromContainer(container.Name)

		// Skip if not a recognized service
		if serviceName == "" {
			continue
		}

		// Get port mapping based on service name
		port := ""
		switch serviceName {
		case "ai", "ollama":
			port = fmt.Sprintf("%d", sm.manager.config.Services.AI.Port)
		case "postgres", "database", "postgresql":
			port = fmt.Sprintf("%d", sm.manager.config.Services.Database.Port)
		case "cache", "redis-cache":
			port = fmt.Sprintf("%d", sm.manager.config.Services.Cache.Port)
		case "queue", "redis-queue":
			port = fmt.Sprintf("%d", sm.manager.config.Services.Queue.Port)
		case "minio", "storage":
			port = fmt.Sprintf("%d", sm.manager.config.Services.Storage.Port)
		}

		status := ServiceStatus{
			Name:   serviceName,
			Status: strings.ToLower(container.State),
			Health: container.Health,
			Port:   port,
		}

		// Get resource usage if running
		if container.State == "running" {
			if stats, err := sm.manager.container.Stats(container.ID); err == nil {
				status.CPUPercent = stats.CPUPercent
				status.MemoryUsage = stats.MemoryUsage
				status.MemoryLimit = stats.MemoryLimit
			}
		}

		statuses = append(statuses, status)
	}

	return statuses, nil
}
