// internal/docker/compose.go
package docker

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/localcloud-sh/localcloud/internal/config"
	"gopkg.in/yaml.v3"
)

// ComposeGenerator generates Docker Compose files from LocalCloud config
type ComposeGenerator struct {
	config *config.Config
}

// NewComposeGenerator creates a new compose generator
func NewComposeGenerator(cfg *config.Config) *ComposeGenerator {
	return &ComposeGenerator{config: cfg}
}

// ComposeFile represents a Docker Compose file structure
type ComposeFile struct {
	Version  string                    `yaml:"version"`
	Services map[string]ComposeService `yaml:"services"`
	Networks map[string]ComposeNetwork `yaml:"networks,omitempty"`
	Volumes  map[string]ComposeVolume  `yaml:"volumes,omitempty"`
}

// ComposeService represents a service in Docker Compose
type ComposeService struct {
	Image         string              `yaml:"image"`
	ContainerName string              `yaml:"container_name"`
	Environment   map[string]string   `yaml:"environment,omitempty"`
	Ports         []string            `yaml:"ports,omitempty"`
	Volumes       []string            `yaml:"volumes,omitempty"`
	Networks      []string            `yaml:"networks,omitempty"`
	Restart       string              `yaml:"restart,omitempty"`
	DependsOn     []string            `yaml:"depends_on,omitempty"`
	Command       []string            `yaml:"command,omitempty"`
	HealthCheck   *ComposeHealthCheck `yaml:"healthcheck,omitempty"`
	Deploy        *ComposeDeploy      `yaml:"deploy,omitempty"`
}

// ComposeHealthCheck represents health check configuration
type ComposeHealthCheck struct {
	Test        []string `yaml:"test"`
	Interval    string   `yaml:"interval"`
	Timeout     string   `yaml:"timeout"`
	Retries     int      `yaml:"retries"`
	StartPeriod string   `yaml:"start_period,omitempty"`
}

// ComposeDeploy represents deployment configuration
type ComposeDeploy struct {
	Resources ComposeResources `yaml:"resources"`
}

// ComposeResources represents resource limits
type ComposeResources struct {
	Limits ComposeResourceLimits `yaml:"limits"`
}

// ComposeResourceLimits represents specific resource limits
type ComposeResourceLimits struct {
	Memory string `yaml:"memory,omitempty"`
	CPUs   string `yaml:"cpus,omitempty"`
}

// ComposeNetwork represents a network in Docker Compose
type ComposeNetwork struct {
	Driver string `yaml:"driver,omitempty"`
}

// ComposeVolume represents a volume in Docker Compose
type ComposeVolume struct {
	Driver string `yaml:"driver,omitempty"`
}

// Generate generates a Docker Compose file
func (g *ComposeGenerator) Generate() (*ComposeFile, error) {
	compose := &ComposeFile{
		Version:  "3.8",
		Services: make(map[string]ComposeService),
		Networks: make(map[string]ComposeNetwork),
		Volumes:  make(map[string]ComposeVolume),
	}

	// Add default network
	compose.Networks["localcloud_default"] = ComposeNetwork{
		Driver: "bridge",
	}

	// Generate AI service
	if err := g.generateAIService(compose); err != nil {
		return nil, err
	}

	// Generate Database service
	if g.config.Services.Database.Type != "" {
		if err := g.generateDatabaseService(compose); err != nil {
			return nil, err
		}
	}

	// Generate Cache service
	if g.config.Services.Cache.Type != "" {
		if err := g.generateCacheService(compose); err != nil {
			return nil, err
		}
	}

	// Generate Storage service
	if g.config.Services.Storage.Type != "" {
		if err := g.generateStorageService(compose); err != nil {
			return nil, err
		}
	}

	return compose, nil
}

// generateAIService generates the AI service configuration
func (g *ComposeGenerator) generateAIService(compose *ComposeFile) error {
	service := ComposeService{
		Image:         "ollama/ollama:latest",
		ContainerName: "localcloud-ai",
		Ports: []string{
			fmt.Sprintf("%d:11434", g.config.Services.AI.Port),
		},
		Volumes: []string{
			"ollama_models:/root/.ollama",
		},
		Networks: []string{"localcloud_default"},
		Restart:  "unless-stopped",
		Environment: map[string]string{
			"OLLAMA_HOST": "0.0.0.0:11434",
		},
		HealthCheck: &ComposeHealthCheck{
			Test:        []string{"CMD", "curl", "-f", "http://localhost:11434/api/tags"},
			Interval:    "30s",
			Timeout:     "10s",
			Retries:     3,
			StartPeriod: "40s",
		},
	}

	// Add resource limits
	if g.config.Resources.MemoryLimit != "" {
		service.Deploy = &ComposeDeploy{
			Resources: ComposeResources{
				Limits: ComposeResourceLimits{
					Memory: g.config.Resources.MemoryLimit,
					CPUs:   g.config.Resources.CPULimit,
				},
			},
		}
	}

	compose.Services["ai"] = service
	compose.Volumes["ollama_models"] = ComposeVolume{}

	return nil
}

// generateDatabaseService generates the database service configuration
func (g *ComposeGenerator) generateDatabaseService(compose *ComposeFile) error {
	if g.config.Services.Database.Type != "postgres" {
		return fmt.Errorf("unsupported database type: %s", g.config.Services.Database.Type)
	}

	service := ComposeService{
		Image:         fmt.Sprintf("postgres:%s-alpine", g.config.Services.Database.Version),
		ContainerName: "localcloud-postgres",
		Environment: map[string]string{
			"POSTGRES_USER":     "localcloud",
			"POSTGRES_PASSWORD": generatePassword(),
			"POSTGRES_DB":       "localcloud",
		},
		Ports: []string{
			fmt.Sprintf("%d:5432", g.config.Services.Database.Port),
		},
		Volumes: []string{
			"postgres_data:/var/lib/postgresql/data",
		},
		Networks: []string{"localcloud_default"},
		Restart:  "unless-stopped",
		HealthCheck: &ComposeHealthCheck{
			Test:     []string{"CMD-SHELL", "pg_isready -U localcloud"},
			Interval: "10s",
			Timeout:  "5s",
			Retries:  5,
		},
	}

	// Add init script for extensions
	if len(g.config.Services.Database.Extensions) > 0 {
		initSQL := g.generateDatabaseInitScript()
		service.Volumes = append(service.Volumes, "./init.sql:/docker-entrypoint-initdb.d/init.sql:ro")

		// We'll need to create this file
		compose.Services["_init_sql"] = ComposeService{
			Image:   "busybox",
			Command: []string{"sh", "-c", fmt.Sprintf("echo '%s' > /init.sql", initSQL)},
			Volumes: []string{"./init.sql:/init.sql"},
		}
	}

	compose.Services["postgres"] = service
	compose.Volumes["postgres_data"] = ComposeVolume{}

	return nil
}

// generateCacheService generates the cache service configuration
func (g *ComposeGenerator) generateCacheService(compose *ComposeFile) error {
	if g.config.Services.Cache.Type != "redis" {
		return fmt.Errorf("unsupported cache type: %s", g.config.Services.Cache.Type)
	}

	maxMemory := g.config.Services.Cache.MaxMemory
	if maxMemory == "" {
		maxMemory = "512mb"
	}

	service := ComposeService{
		Image:         "redis:7-alpine",
		ContainerName: "localcloud-redis",
		Command: []string{
			"redis-server",
			"--maxmemory", maxMemory,
			"--maxmemory-policy", "allkeys-lru",
		},
		Ports: []string{
			fmt.Sprintf("%d:6379", g.config.Services.Cache.Port),
		},
		Volumes: []string{
			"redis_data:/data",
		},
		Networks: []string{"localcloud_default"},
		Restart:  "unless-stopped",
		HealthCheck: &ComposeHealthCheck{
			Test:     []string{"CMD", "redis-cli", "ping"},
			Interval: "10s",
			Timeout:  "5s",
			Retries:  5,
		},
	}

	compose.Services["redis"] = service
	compose.Volumes["redis_data"] = ComposeVolume{}

	return nil
}

// generateStorageService generates the storage service configuration
func (g *ComposeGenerator) generateStorageService(compose *ComposeFile) error {
	if g.config.Services.Storage.Type != "minio" {
		return fmt.Errorf("unsupported storage type: %s", g.config.Services.Storage.Type)
	}

	rootUser := "localcloud"
	rootPassword := generatePassword()

	service := ComposeService{
		Image:         "minio/minio:latest",
		ContainerName: "localcloud-minio",
		Command: []string{
			"server",
			"/data",
			"--console-address", ":9001",
		},
		Environment: map[string]string{
			"MINIO_ROOT_USER":     rootUser,
			"MINIO_ROOT_PASSWORD": rootPassword,
		},
		Ports: []string{
			fmt.Sprintf("%d:9000", g.config.Services.Storage.Port),
			fmt.Sprintf("%d:9001", g.config.Services.Storage.Console),
		},
		Volumes: []string{
			"minio_data:/data",
		},
		Networks: []string{"localcloud_default"},
		Restart:  "unless-stopped",
		HealthCheck: &ComposeHealthCheck{
			Test:     []string{"CMD", "curl", "-f", "http://localhost:9000/minio/health/live"},
			Interval: "30s",
			Timeout:  "10s",
			Retries:  3,
		},
	}

	compose.Services["minio"] = service
	compose.Volumes["minio_data"] = ComposeVolume{}

	return nil
}

// generateDatabaseInitScript generates SQL script for extensions
func (g *ComposeGenerator) generateDatabaseInitScript() string {
	var lines []string
	for _, ext := range g.config.Services.Database.Extensions {
		lines = append(lines, fmt.Sprintf("CREATE EXTENSION IF NOT EXISTS %s;", ext))
	}
	return strings.Join(lines, "\n")
}

// WriteToFile writes the compose file to disk
func (g *ComposeGenerator) WriteToFile(compose *ComposeFile, path string) error {
	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Marshal to YAML
	data, err := yaml.Marshal(compose)
	if err != nil {
		return fmt.Errorf("failed to marshal compose file: %w", err)
	}

	// Write file
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write compose file: %w", err)
	}

	return nil
}

// GenerateOverride generates a docker-compose.override.yml for development
func (g *ComposeGenerator) GenerateOverride() (*ComposeFile, error) {
	override := &ComposeFile{
		Version:  "3.8",
		Services: make(map[string]ComposeService),
	}

	// Add development-specific overrides
	// For example, mount source code directories, enable debug modes, etc.

	return override, nil
}

// generatePassword generates a random password
func generatePassword() string {
	// In production, use crypto/rand for secure password generation
	// For now, return a placeholder
	return "localcloud-dev-password"
}
