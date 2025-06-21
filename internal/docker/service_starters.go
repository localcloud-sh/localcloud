// internal/docker/service_starters.go
package docker

import (
	"fmt"
	"time"
)

// ServiceStarter interface for starting individual services
type ServiceStarter interface {
	Start() error
}

// AIServiceStarter handles AI service startup
type AIServiceStarter struct {
	manager *Manager
}

// NewAIServiceStarter creates a new AI service starter
func NewAIServiceStarter(m *Manager) ServiceStarter {
	return &AIServiceStarter{manager: m}
}

// Start starts the AI service
func (s *AIServiceStarter) Start() error {
	// Check and pull image
	if err := s.ensureImage("ollama/ollama:latest"); err != nil {
		return err
	}

	// Create container config
	config := ContainerConfig{
		Name:  "localcloud-ai",
		Image: "ollama/ollama:latest",
		Env: map[string]string{
			"OLLAMA_HOST": "0.0.0.0:11434",
		},
		Ports: []PortBinding{
			{
				ContainerPort: "11434",
				HostPort:      fmt.Sprintf("%d", s.manager.config.Services.AI.Port),
				Protocol:      "tcp",
			},
		},
		Volumes: []VolumeMount{
			{
				Source: fmt.Sprintf("localcloud_%s_ollama_models", s.manager.config.Project.Name),
				Target: "/root/.ollama",
			},
		},
		Networks:      []string{fmt.Sprintf("localcloud_%s_default", s.manager.config.Project.Name)},
		RestartPolicy: "unless-stopped",
		HealthCheck: &HealthCheckConfig{
			Test:        []string{"CMD", "curl", "-f", "http://localhost:11434/api/tags"},
			Interval:    30,
			Timeout:     10,
			Retries:     3,
			StartPeriod: 40,
		},
		Labels: map[string]string{
			"com.localcloud.project": s.manager.config.Project.Name,
			"com.localcloud.service": "ai",
		},
	}

	// Apply resource limits
	if s.manager.config.Resources.MemoryLimit != "" {
		config.Memory = parseMemoryLimit(s.manager.config.Resources.MemoryLimit)
	}

	// Create and start container
	containerID, err := s.manager.container.Create(config)
	if err != nil {
		return err
	}

	if err := s.manager.container.Start(containerID); err != nil {
		return err
	}

	// Wait for health check
	return s.manager.container.WaitHealthy(containerID, 120*time.Second)
}

// ensureImage checks and pulls image if needed
func (s *AIServiceStarter) ensureImage(image string) error {
	exists, err := s.manager.image.Exists(image)
	if err != nil {
		return err
	}
	if !exists {
		progress := make(chan PullProgress)
		go func() {
			for range progress {
				// Could log progress here
			}
		}()

		if err := s.manager.image.Pull(image, progress); err != nil {
			return fmt.Errorf("failed to pull image %s: %w", image, err)
		}
	}
	return nil
}

// DatabaseServiceStarter handles database service startup
type DatabaseServiceStarter struct {
	manager *Manager
}

// NewDatabaseServiceStarter creates a new database service starter
func NewDatabaseServiceStarter(m *Manager) ServiceStarter {
	return &DatabaseServiceStarter{manager: m}
}

// Start starts the database service
func (s *DatabaseServiceStarter) Start() error {
	if s.manager.config.Services.Database.Type == "" {
		return nil // Database not configured
	}

	image := fmt.Sprintf("postgres:%s-alpine", s.manager.config.Services.Database.Version)

	// Check and pull image
	if err := s.ensureImage(image); err != nil {
		return err
	}

	// Create container config
	config := ContainerConfig{
		Name:  "localcloud-postgres",
		Image: image,
		Env: map[string]string{
			"POSTGRES_USER":     "localcloud",
			"POSTGRES_PASSWORD": "localcloud-dev",
			"POSTGRES_DB":       "localcloud",
		},
		Ports: []PortBinding{
			{
				ContainerPort: "5432",
				HostPort:      fmt.Sprintf("%d", s.manager.config.Services.Database.Port),
				Protocol:      "tcp",
			},
		},
		Volumes: []VolumeMount{
			{
				Source: fmt.Sprintf("localcloud_%s_postgres_data", s.manager.config.Project.Name),
				Target: "/var/lib/postgresql/data",
			},
		},
		Networks:      []string{fmt.Sprintf("localcloud_%s_default", s.manager.config.Project.Name)},
		RestartPolicy: "unless-stopped",
		HealthCheck: &HealthCheckConfig{
			Test:     []string{"CMD-SHELL", "pg_isready -U localcloud"},
			Interval: 10,
			Timeout:  5,
			Retries:  5,
		},
		Labels: map[string]string{
			"com.localcloud.project": s.manager.config.Project.Name,
			"com.localcloud.service": "database",
		},
	}

	// Create and start container
	containerID, err := s.manager.container.Create(config)
	if err != nil {
		return err
	}

	if err := s.manager.container.Start(containerID); err != nil {
		return err
	}

	// Wait for health check
	return s.manager.container.WaitHealthy(containerID, 30*time.Second)
}

// ensureImage checks and pulls image if needed
func (s *DatabaseServiceStarter) ensureImage(image string) error {
	starter := &AIServiceStarter{manager: s.manager}
	return starter.ensureImage(image)
}

// CacheServiceStarter handles cache service startup
type CacheServiceStarter struct {
	manager *Manager
}

// NewCacheServiceStarter creates a new cache service starter
func NewCacheServiceStarter(m *Manager) ServiceStarter {
	return &CacheServiceStarter{manager: m}
}

// Start starts the cache service
func (s *CacheServiceStarter) Start() error {
	if s.manager.config.Services.Cache.Type == "" {
		return nil // Cache not configured
	}

	// Check and pull image
	if err := s.ensureImage("redis:7-alpine"); err != nil {
		return err
	}

	// Create container config
	config := ContainerConfig{
		Name:  "localcloud-redis",
		Image: "redis:7-alpine",
		Command: []string{
			"redis-server",
			"--maxmemory", s.manager.config.Services.Cache.MaxMemory,
			"--maxmemory-policy", "allkeys-lru",
		},
		Ports: []PortBinding{
			{
				ContainerPort: "6379",
				HostPort:      fmt.Sprintf("%d", s.manager.config.Services.Cache.Port),
				Protocol:      "tcp",
			},
		},
		Volumes: []VolumeMount{
			{
				Source: fmt.Sprintf("localcloud_%s_redis_data", s.manager.config.Project.Name),
				Target: "/data",
			},
		},
		Networks:      []string{fmt.Sprintf("localcloud_%s_default", s.manager.config.Project.Name)},
		RestartPolicy: "unless-stopped",
		HealthCheck: &HealthCheckConfig{
			Test:     []string{"CMD", "redis-cli", "ping"},
			Interval: 10,
			Timeout:  5,
			Retries:  5,
		},
		Labels: map[string]string{
			"com.localcloud.project": s.manager.config.Project.Name,
			"com.localcloud.service": "cache",
		},
	}

	// Create and start container
	containerID, err := s.manager.container.Create(config)
	if err != nil {
		return err
	}

	return s.manager.container.Start(containerID)
}

// ensureImage checks and pulls image if needed
func (s *CacheServiceStarter) ensureImage(image string) error {
	starter := &AIServiceStarter{manager: s.manager}
	return starter.ensureImage(image)
}

// StorageServiceStarter handles storage service startup
type StorageServiceStarter struct {
	manager *Manager
}

// NewStorageServiceStarter creates a new storage service starter
func NewStorageServiceStarter(m *Manager) ServiceStarter {
	return &StorageServiceStarter{manager: m}
}

// Start starts the storage service
func (s *StorageServiceStarter) Start() error {
	if s.manager.config.Services.Storage.Type == "" {
		return nil // Storage not configured
	}

	// Check and pull image
	if err := s.ensureImage("minio/minio:latest"); err != nil {
		return err
	}

	// Create container config
	config := ContainerConfig{
		Name:  "localcloud-minio",
		Image: "minio/minio:latest",
		Command: []string{
			"server",
			"/data",
			"--console-address", ":9001",
		},
		Env: map[string]string{
			"MINIO_ROOT_USER":     "localcloud",
			"MINIO_ROOT_PASSWORD": "localcloud-dev",
		},
		Ports: []PortBinding{
			{
				ContainerPort: "9000",
				HostPort:      fmt.Sprintf("%d", s.manager.config.Services.Storage.Port),
				Protocol:      "tcp",
			},
			{
				ContainerPort: "9001",
				HostPort:      fmt.Sprintf("%d", s.manager.config.Services.Storage.Console),
				Protocol:      "tcp",
			},
		},
		Volumes: []VolumeMount{
			{
				Source: fmt.Sprintf("localcloud_%s_minio_data", s.manager.config.Project.Name),
				Target: "/data",
			},
		},
		Networks:      []string{fmt.Sprintf("localcloud_%s_default", s.manager.config.Project.Name)},
		RestartPolicy: "unless-stopped",
		HealthCheck: &HealthCheckConfig{
			Test:     []string{"CMD", "curl", "-f", "http://localhost:9000/minio/health/live"},
			Interval: 30,
			Timeout:  10,
			Retries:  3,
		},
		Labels: map[string]string{
			"com.localcloud.project": s.manager.config.Project.Name,
			"com.localcloud.service": "storage",
		},
	}

	// Create and start container
	containerID, err := s.manager.container.Create(config)
	if err != nil {
		return err
	}

	return s.manager.container.Start(containerID)
}

// ensureImage checks and pulls image if needed
func (s *StorageServiceStarter) ensureImage(image string) error {
	starter := &AIServiceStarter{manager: s.manager}
	return starter.ensureImage(image)
}
