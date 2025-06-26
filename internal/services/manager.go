// internal/services/manager.go
package services

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/localcloud/localcloud/internal/docker"
	"github.com/localcloud/localcloud/internal/templates"
)

// ServiceConfig represents configuration for a service
type ServiceConfig struct {
	Name          string
	Image         string
	PreferredPort int
	Environment   map[string]string
	Volumes       []string
	Command       []string
	HealthCheck   *HealthCheck
}

// HealthCheck represents health check configuration
type HealthCheck struct {
	Type     string // http, tcp, cmd
	Endpoint string
	Interval time.Duration
	Timeout  time.Duration
	Retries  int
}

// ComponentType represents abstract component types
type ComponentType string

const (
	ComponentSpeechToText ComponentType = "speech-to-text"
	ComponentTextToSpeech ComponentType = "text-to-speech"
	ComponentVectorDB     ComponentType = "vector-db"
	ComponentImageGen     ComponentType = "image-generation"
	ComponentStorage      ComponentType = "storage"
)

// ComponentRegistry maps components to their default implementations
var ComponentRegistry = map[ComponentType]ServiceConfig{
	ComponentSpeechToText: {
		Name:          "whisper",
		Image:         "onerahmet/openai-whisper-asr-webservice:latest",
		PreferredPort: 9000,
		Environment: map[string]string{
			"ASR_MODEL": "base",
		},
	},
	ComponentVectorDB: {
		Name:          "pgvector",
		Image:         "pgvector/pgvector:pg16",
		PreferredPort: 5432,
		Environment: map[string]string{
			"POSTGRES_USER":     "localcloud",
			"POSTGRES_PASSWORD": "localcloud",
			"POSTGRES_DB":       "localcloud",
		},
		Volumes: []string{
			"pgvector_data:/var/lib/postgresql/data",
		},
		HealthCheck: &HealthCheck{
			Type:     "cmd",
			Endpoint: "pg_isready -U localcloud",
			Interval: 10 * time.Second,
			Timeout:  5 * time.Second,
			Retries:  5,
		},
	},
	ComponentStorage: {
		Name:          "minio",
		Image:         "minio/minio:latest",
		PreferredPort: 9000,
		Environment: map[string]string{
			"MINIO_ROOT_USER":     "localcloud",
			"MINIO_ROOT_PASSWORD": "localcloud123",
		},
		Command: []string{"server", "/data", "--console-address", ":9001"},
		Volumes: []string{
			"minio_data:/data",
		},
	},
	ComponentTextToSpeech: {
		Name:          "piper",
		Image:         "lscr.io/linuxserver/piper:latest",
		PreferredPort: 10200,
		Environment: map[string]string{
			"PIPER_VOICE": "en_US-amy-medium",
		},
	},
	ComponentImageGen: {
		Name:          "stable-diffusion-webui",
		Image:         "ghcr.io/automattic/stable-diffusion-webui:latest",
		PreferredPort: 7860,
		Volumes: []string{
			"sd_models:/stable-diffusion-webui/models",
		},
	},
}

// ServiceManager manages dynamic service lifecycle
type ServiceManager struct {
	registry    *ServiceRegistry
	portManager *templates.PortManager
	docker      *docker.Manager
	mu          sync.RWMutex
	projectName string
}

// NewServiceManager creates a new service manager
func NewServiceManager(projectName string, docker *docker.Manager, portManager *templates.PortManager) *ServiceManager {
	return &ServiceManager{
		registry:    NewServiceRegistry("."),
		portManager: portManager,
		docker:      docker,
		projectName: projectName,
	}
}

// ParseComponentName converts various inputs to component type or service name
func (sm *ServiceManager) ParseComponentName(input string) (string, *ServiceConfig) {
	normalized := strings.ToLower(strings.TrimSpace(input))

	// Check if it's a component type
	componentMappings := map[string]ComponentType{
		"speech-to-text":   ComponentSpeechToText,
		"stt":              ComponentSpeechToText,
		"whisper":          ComponentSpeechToText,
		"text-to-speech":   ComponentTextToSpeech,
		"tts":              ComponentTextToSpeech,
		"vector-db":        ComponentVectorDB,
		"vectordb":         ComponentVectorDB,
		"vector":           ComponentVectorDB,
		"pgvector":         ComponentVectorDB,
		"qdrant":           ComponentVectorDB,
		"image-generation": ComponentImageGen,
		"image-gen":        ComponentImageGen,
		"image":            ComponentImageGen,
		"storage":          ComponentStorage,
		"s3":               ComponentStorage,
		"minio":            ComponentStorage,
	}

	if componentType, exists := componentMappings[normalized]; exists {
		if config, ok := ComponentRegistry[componentType]; ok {
			// Use component type as service name for consistency
			serviceName := string(componentType)
			return serviceName, &config
		}
	}

	// Legacy direct service configs
	legacyConfigs := map[string]ServiceConfig{
		"whisper": ComponentRegistry[ComponentSpeechToText],
		"qdrant": {
			Name:          "qdrant",
			Image:         "qdrant/qdrant:latest",
			PreferredPort: 6333,
			Volumes: []string{
				"qdrant_data:/qdrant/storage",
			},
		},
		"minio": ComponentRegistry[ComponentStorage],
	}

	if config, exists := legacyConfigs[normalized]; exists {
		return normalized, &config
	}

	return normalized, nil
}

// StartService starts a service dynamically
func (sm *ServiceManager) StartService(name string, config ServiceConfig) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	// Parse component name to get actual service name and config
	serviceName, componentConfig := sm.ParseComponentName(name)

	// If component config found, use it (unless custom config provided)
	if componentConfig != nil && config.Image == "" {
		config = *componentConfig
	}

	// Check if service is already running
	if _, err := sm.registry.Get(serviceName); err == nil {
		return fmt.Errorf("service %s is already running", serviceName)
	}

	// Allocate port
	port, err := sm.portManager.AllocatePort(serviceName, config.PreferredPort)
	if err != nil {
		return fmt.Errorf("failed to allocate port for %s: %w", serviceName, err)
	}

	// Create container configuration
	containerConfig := sm.buildContainerConfig(serviceName, config, port)

	// Start the container
	containerID, err := sm.docker.CreateAndStartContainer(containerConfig)
	if err != nil {
		sm.portManager.ReleasePort(port)
		return fmt.Errorf("failed to start %s: %w", serviceName, err)
	}

	// Register the service
	service := Service{
		Name:        serviceName,
		Port:        port,
		ContainerID: containerID,
		URL:         fmt.Sprintf("http://localhost:%d", port),
		Status:      "starting",
		StartedAt:   time.Now(),
	}

	// Add model and type information for AI services
	if componentConfig != nil {
		service.Type = getServiceType(serviceName)
		service.Model = getServiceModel(serviceName, config)

		// Store original service name in metadata if using component name
		if serviceName != name {
			service.Metadata = map[string]interface{}{
				"original_name": name,
				"component":     serviceName,
			}
		}
	}

	if err := sm.registry.Register(service); err != nil {
		// Cleanup on registration failure
		sm.docker.StopContainer(containerID, 10)
		sm.portManager.ReleasePort(port)
		return fmt.Errorf("failed to register service: %w", err)
	}

	// Start health monitoring
	go sm.monitorServiceHealth(serviceName)

	return nil
}

// StopService stops a running service
func (sm *ServiceManager) StopService(name string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	// Get service from registry
	service, err := sm.registry.Get(name)
	if err != nil {
		return fmt.Errorf("service %s not found", name)
	}

	// Update status
	sm.registry.UpdateStatus(name, "stopping")

	// Stop the container
	if err := sm.docker.StopContainer(service.ContainerID, 10); err != nil {
		return fmt.Errorf("failed to stop container: %w", err)
	}

	// Release the port
	sm.portManager.ReleasePort(service.Port)

	// Unregister the service
	if err := sm.registry.Unregister(name); err != nil {
		return fmt.Errorf("failed to unregister service: %w", err)
	}

	return nil
}

// GetServiceURL returns the URL for a service
func (sm *ServiceManager) GetServiceURL(name string) (string, error) {
	return sm.registry.GetURL(name)
}

// ListServices returns all running services
func (sm *ServiceManager) ListServices() []Service {
	return sm.registry.List()
}

// GetServiceStatus returns the status of a specific service
func (sm *ServiceManager) GetServiceStatus(name string) (*Service, error) {
	return sm.registry.Get(name)
}

// RestartService restarts a service
func (sm *ServiceManager) RestartService(name string) error {
	// Get current service config
	_, err := sm.registry.Get(name)
	if err != nil {
		return fmt.Errorf("service %s not found", name)
	}

	// Stop the service
	if err := sm.StopService(name); err != nil {
		return fmt.Errorf("failed to stop service: %w", err)
	}

	// Wait a moment
	time.Sleep(2 * time.Second)

	// Get the service configuration
	config := sm.getServiceConfig(name)
	if config == nil {
		return fmt.Errorf("unknown service type: %s", name)
	}

	// Start the service again
	return sm.StartService(name, *config)
}

// buildContainerConfig creates container configuration
func (sm *ServiceManager) buildContainerConfig(name string, config ServiceConfig, port int) docker.ContainerConfig {
	// Base labels
	labels := map[string]string{
		"com.localcloud.project": sm.projectName,
		"com.localcloud.service": name,
	}

	// Port bindings
	var ports []docker.PortBinding
	if port > 0 {
		ports = append(ports, docker.PortBinding{
			ContainerPort: fmt.Sprintf("%d", port),
			HostPort:      fmt.Sprintf("%d", port),
			Protocol:      "tcp",
		})
	}

	// Volume mounts
	var volumes []docker.VolumeMount
	for _, vol := range config.Volumes {
		parts := strings.Split(vol, ":")
		if len(parts) >= 2 {
			mount := docker.VolumeMount{
				Source: parts[0],
				Target: parts[1],
			}
			if len(parts) >= 3 && parts[2] == "ro" {
				mount.ReadOnly = true
			}
			volumes = append(volumes, mount)
		}
	}

	// Container name
	containerName := fmt.Sprintf("localcloud-%s-%s", sm.projectName, name)

	return docker.ContainerConfig{
		Name:          containerName,
		Image:         config.Image,
		Ports:         ports,
		Env:           config.Environment,
		Volumes:       volumes,
		Command:       config.Command,
		Labels:        labels,
		Networks:      []string{fmt.Sprintf("localcloud_%s", sm.projectName)},
		RestartPolicy: "unless-stopped",
	}
}

// monitorServiceHealth monitors service health
func (sm *ServiceManager) monitorServiceHealth(name string) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			service, err := sm.registry.Get(name)
			if err != nil {
				// Service was removed
				return
			}

			// Check container status
			status, err := sm.docker.GetContainerStatus(service.ContainerID)
			if err != nil {
				sm.registry.UpdateStatus(name, "error")
				continue
			}

			// Update status
			if status.State == "running" {
				sm.registry.UpdateStatus(name, "running")
				if status.Health != "" {
					sm.registry.UpdateHealth(name, status.Health)
				}
			} else {
				sm.registry.UpdateStatus(name, "stopped")
			}
		}
	}
}

// getServiceConfig returns configuration for known service types
func (sm *ServiceManager) getServiceConfig(name string) *ServiceConfig {
	_, config := sm.ParseComponentName(name)
	return config
}

// GetPortAllocations returns all port allocations
func (sm *ServiceManager) GetPortAllocations() map[int]string {
	return sm.portManager.GetAllocatedPorts()
}

// getServiceType returns the type of service
func getServiceType(serviceName string) string {
	switch serviceName {
	case "speech-to-text", "whisper":
		return "whisper"
	case "text-to-speech", "piper":
		return "piper"
	case "vector-db", "pgvector":
		return "database"
	case "storage", "minio":
		return "storage"
	case "image-generation":
		return "sd-webui"
	default:
		return serviceName
	}
}

// getServiceModel returns the model for AI services
func getServiceModel(serviceName string, config ServiceConfig) string {
	// Check environment variables for model info
	if model, exists := config.Environment["ASR_MODEL"]; exists {
		return model
	}
	if model, exists := config.Environment["PIPER_VOICE"]; exists {
		return model
	}
	if model, exists := config.Environment["MODEL"]; exists {
		return model
	}

	// Default models
	switch serviceName {
	case "speech-to-text":
		return "base"
	case "text-to-speech":
		return "en_US-amy-medium"
	case "vector-db":
		return "pgvector"
	default:
		return ""
	}
}

// WithCustomConfig allows using custom service configuration
func (sm *ServiceManager) WithCustomConfig(name string, config ServiceConfig) error {
	// Ensure the config has required fields
	if config.Image == "" {
		return fmt.Errorf("image is required for service %s", name)
	}

	config.Name = name
	return sm.StartService(name, config)
}

// IsVectorEnabled checks if service is vector-enabled
func (sm *ServiceManager) IsVectorEnabled(serviceName string) bool {
	// Check if it's pgvector service
	if serviceName == "pgvector" || serviceName == "vector-db" {
		return true
	}

	// Check if PostgreSQL has pgvector extension
	if serviceName == "postgres" || serviceName == "database" {
		// This would need to check the actual config
		// For now, return false as base postgres doesn't have pgvector
		return false
	}

	return false
}
