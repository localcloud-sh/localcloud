// internal/services/registry.go
package services

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Service represents a running service
type Service struct {
	Name        string                 `json:"name"`
	Port        int                    `json:"port"`
	ContainerID string                 `json:"container_id"`
	URL         string                 `json:"url"`
	Status      string                 `json:"status"`
	StartedAt   time.Time              `json:"started_at"`
	Health      string                 `json:"health,omitempty"`
	Model       string                 `json:"model,omitempty"`    // For AI services
	Type        string                 `json:"type,omitempty"`     // Service type
	Metadata    map[string]interface{} `json:"metadata,omitempty"` // Additional metadata
}

// ServiceRegistry manages service registration and discovery
type ServiceRegistry struct {
	mu       sync.RWMutex
	services map[string]*Service
	dataFile string
}

// NewServiceRegistry creates a new service registry
func NewServiceRegistry(projectPath string) *ServiceRegistry {
	dataFile := filepath.Join(projectPath, ".localcloud", "services.json")
	registry := &ServiceRegistry{
		services: make(map[string]*Service),
		dataFile: dataFile,
	}

	// Load existing services from file
	registry.load()

	return registry
}

// Register adds a new service to the registry
func (sr *ServiceRegistry) Register(service Service) error {
	sr.mu.Lock()
	defer sr.mu.Unlock()

	if _, exists := sr.services[service.Name]; exists {
		return fmt.Errorf("service %s already registered", service.Name)
	}

	sr.services[service.Name] = &service
	return sr.save()
}

// Unregister removes a service from the registry
func (sr *ServiceRegistry) Unregister(serviceName string) error {
	sr.mu.Lock()
	defer sr.mu.Unlock()

	if _, exists := sr.services[serviceName]; !exists {
		return fmt.Errorf("service %s not found", serviceName)
	}

	delete(sr.services, serviceName)
	return sr.save()
}

// Get returns a service by name
func (sr *ServiceRegistry) Get(serviceName string) (*Service, error) {
	sr.mu.RLock()
	defer sr.mu.RUnlock()

	service, exists := sr.services[serviceName]
	if !exists {
		return nil, fmt.Errorf("service %s not found", serviceName)
	}

	// Return a copy to prevent external modification
	serviceCopy := *service
	return &serviceCopy, nil
}

// List returns all registered services
func (sr *ServiceRegistry) List() []Service {
	sr.mu.RLock()
	defer sr.mu.RUnlock()

	services := make([]Service, 0, len(sr.services))
	for _, service := range sr.services {
		services = append(services, *service)
	}

	return services
}

// GetURL returns the URL for a service
func (sr *ServiceRegistry) GetURL(serviceName string) (string, error) {
	service, err := sr.Get(serviceName)
	if err != nil {
		return "", err
	}

	return service.URL, nil
}

// UpdateStatus updates the status of a service
func (sr *ServiceRegistry) UpdateStatus(serviceName, status string) error {
	sr.mu.Lock()
	defer sr.mu.Unlock()

	service, exists := sr.services[serviceName]
	if !exists {
		return fmt.Errorf("service %s not found", serviceName)
	}

	service.Status = status
	return sr.save()
}

// UpdateHealth updates the health status of a service
func (sr *ServiceRegistry) UpdateHealth(serviceName, health string) error {
	sr.mu.Lock()
	defer sr.mu.Unlock()

	service, exists := sr.services[serviceName]
	if !exists {
		return fmt.Errorf("service %s not found", serviceName)
	}

	service.Health = health
	return sr.save()
}

// GetByContainerID returns a service by container ID
func (sr *ServiceRegistry) GetByContainerID(containerID string) (*Service, error) {
	sr.mu.RLock()
	defer sr.mu.RUnlock()

	for _, service := range sr.services {
		if service.ContainerID == containerID {
			serviceCopy := *service
			return &serviceCopy, nil
		}
	}

	return nil, fmt.Errorf("service with container ID %s not found", containerID)
}

// ListByStatus returns services with a specific status
func (sr *ServiceRegistry) ListByStatus(status string) []Service {
	sr.mu.RLock()
	defer sr.mu.RUnlock()

	var services []Service
	for _, service := range sr.services {
		if service.Status == status {
			services = append(services, *service)
		}
	}

	return services
}

// Clear removes all services from the registry
func (sr *ServiceRegistry) Clear() error {
	sr.mu.Lock()
	defer sr.mu.Unlock()

	sr.services = make(map[string]*Service)
	return sr.save()
}

// save persists the registry to disk
func (sr *ServiceRegistry) save() error {
	// Ensure directory exists
	dir := filepath.Dir(sr.dataFile)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Marshal services to JSON
	data, err := json.MarshalIndent(sr.services, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal services: %w", err)
	}

	// Write to temporary file first
	tmpFile := sr.dataFile + ".tmp"
	if err := os.WriteFile(tmpFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write registry file: %w", err)
	}

	// Rename to actual file (atomic operation)
	if err := os.Rename(tmpFile, sr.dataFile); err != nil {
		return fmt.Errorf("failed to save registry file: %w", err)
	}

	return nil
}

// load reads the registry from disk
func (sr *ServiceRegistry) load() error {
	data, err := os.ReadFile(sr.dataFile)
	if err != nil {
		if os.IsNotExist(err) {
			// File doesn't exist yet, that's OK
			return nil
		}
		return fmt.Errorf("failed to read registry file: %w", err)
	}

	var services map[string]*Service
	if err := json.Unmarshal(data, &services); err != nil {
		return fmt.Errorf("failed to unmarshal services: %w", err)
	}

	sr.services = services
	return nil
}
