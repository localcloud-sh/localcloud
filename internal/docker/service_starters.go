// internal/docker/service_starters.go
package docker

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/localcloud/localcloud/internal/models"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
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
	return &AIServiceStarter{
		manager: m,
	}
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
			Timeout:     30, // Increased from 10
			Retries:     5,  // Increased from 3
			StartPeriod: 60, // Increased from 40
		},
		Labels: map[string]string{
			"com.localcloud.project": s.manager.config.Project.Name,
			"com.localcloud.service": "ai",
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

	// Just wait for Ollama to start
	time.Sleep(10 * time.Second)

	// Post-start tasks
	return s.postStart()
}

// postStart performs post-startup tasks for AI service
func (s *AIServiceStarter) postStart() error {
	// Wait for Ollama to be fully ready
	time.Sleep(3 * time.Second)

	// Check if any models are configured to be auto-pulled
	cfg := s.manager.config
	if len(cfg.Services.AI.Models) == 0 {
		return nil
	}

	// Create model manager
	modelManager := models.NewManager(fmt.Sprintf("http://localhost:%d", cfg.Services.AI.Port))

	// Check installed models
	installed, err := modelManager.List()
	if err != nil {
		// Not critical, Ollama might still be starting
		return nil
	}

	// Check if default model is installed
	defaultModel := cfg.Services.AI.Default
	if defaultModel == "" && len(cfg.Services.AI.Models) > 0 {
		defaultModel = cfg.Services.AI.Models[0]
	}

	defaultInstalled := false
	for _, model := range installed {
		if model.Name == defaultModel || model.Model == defaultModel {
			defaultInstalled = true
			break
		}
	}

	// If default model not installed, show a message
	if !defaultInstalled && defaultModel != "" {
		fmt.Printf("\n%s Default model '%s' is not installed.\n", warningColor("!"), defaultModel)
		fmt.Printf("Run '%s' to download it.\n\n", infoColor(fmt.Sprintf("lc models pull %s", defaultModel)))
	}

	return nil
}

// waitForOllama waits for Ollama API to be ready
func (s *AIServiceStarter) waitForOllama() error {
	fmt.Println("DEBUG: Waiting for Ollama to be ready...")
	endpoint := fmt.Sprintf("http://localhost:%d/api/tags", s.manager.config.Services.AI.Port)
	client := &http.Client{Timeout: 5 * time.Second}

	maxAttempts := 30
	for i := 0; i < maxAttempts; i++ {
		resp, err := client.Get(endpoint)
		if err == nil && resp.StatusCode == 200 {
			resp.Body.Close()
			fmt.Println("DEBUG: Ollama is ready!")
			return nil
		}

		if err != nil {
			fmt.Printf("DEBUG: Ollama not ready yet (attempt %d/%d): %v\n", i+1, maxAttempts, err)
		} else if resp != nil {
			fmt.Printf("DEBUG: Ollama returned status %d (attempt %d/%d)\n", resp.StatusCode, i+1, maxAttempts)
			resp.Body.Close()
		}

		time.Sleep(3 * time.Second)
	}

	return fmt.Errorf("ollama failed to start after %d attempts", maxAttempts)
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
	manager      *Manager
	rootPassword string
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

	// Generate secure password
	s.rootPassword = generateSecurePassword()

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
			"MINIO_ROOT_PASSWORD": s.rootPassword,
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

	if err := s.manager.container.Start(containerID); err != nil {
		return err
	}

	// Wait for health check
	if err := s.manager.container.WaitHealthy(containerID, 60*time.Second); err != nil {
		return err
	}

	// Post-start setup (create default bucket, save credentials)
	return s.postStart()
}

// ensureImage checks and pulls image if needed
func (s *StorageServiceStarter) ensureImage(image string) error {
	starter := &AIServiceStarter{manager: s.manager}
	return starter.ensureImage(image)
}

// postStart performs post-startup tasks
func (s *StorageServiceStarter) postStart() error {
	// Wait a bit for MinIO to fully initialize
	time.Sleep(2 * time.Second)

	// Create MinIO client
	client, err := s.createMinIOClient()
	if err != nil {
		return fmt.Errorf("failed to create MinIO client: %w", err)
	}

	// Create default bucket
	bucketName := fmt.Sprintf("%s-storage", s.manager.config.Project.Name)
	if err := s.createDefaultBucket(client, bucketName); err != nil {
		return fmt.Errorf("failed to create default bucket: %w", err)
	}

	// Save connection info
	return s.saveConnectionInfo()
}

// createMinIOClient creates a MinIO client
func (s *StorageServiceStarter) createMinIOClient() (*minio.Client, error) {
	endpoint := fmt.Sprintf("localhost:%d", s.manager.config.Services.Storage.Port)

	return minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4("localcloud", s.rootPassword, ""),
		Secure: false,
	})
}

// createDefaultBucket creates the default bucket with public policy
func (s *StorageServiceStarter) createDefaultBucket(client *minio.Client, bucketName string) error {
	ctx := context.Background()

	exists, err := client.BucketExists(ctx, bucketName)
	if err != nil {
		return err
	}

	if !exists {
		if err := client.MakeBucket(ctx, bucketName, minio.MakeBucketOptions{}); err != nil {
			return err
		}

		// Set public read policy for /public/* path
		policy := fmt.Sprintf(`{
			"Version": "2012-10-17",
			"Statement": [{
				"Effect": "Allow",
				"Principal": {"AWS": ["*"]},
				"Action": ["s3:GetObject"],
				"Resource": ["arn:aws:s3:::%s/public/*"]
			}]
		}`, bucketName)

		return client.SetBucketPolicy(ctx, bucketName, policy)
	}

	return nil
}

// StorageCredentials holds MinIO connection information
type StorageCredentials struct {
	Endpoint   string `json:"endpoint"`
	AccessKey  string `json:"access_key"`
	SecretKey  string `json:"secret_key"`
	UseSSL     bool   `json:"use_ssl"`
	ConsoleURL string `json:"console_url"`
	BucketName string `json:"default_bucket"`
}

// saveConnectionInfo saves MinIO connection information to file
func (s *StorageServiceStarter) saveConnectionInfo() error {
	creds := StorageCredentials{
		Endpoint:   fmt.Sprintf("http://localhost:%d", s.manager.config.Services.Storage.Port),
		AccessKey:  "localcloud",
		SecretKey:  s.rootPassword,
		UseSSL:     false,
		ConsoleURL: fmt.Sprintf("http://localhost:%d", s.manager.config.Services.Storage.Console),
		BucketName: fmt.Sprintf("%s-storage", s.manager.config.Project.Name),
	}

	// Create .localcloud directory if it doesn't exist
	if err := os.MkdirAll(".localcloud", 0755); err != nil {
		return err
	}

	credsPath := filepath.Join(".localcloud", "storage-credentials.json")
	data, err := json.MarshalIndent(creds, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(credsPath, data, 0600)
}

// generateSecurePassword generates a secure random password
func generateSecurePassword() string {
	bytes := make([]byte, 16)
	rand.Read(bytes)
	return base64.URLEncoding.EncodeToString(bytes)
}

// Helper color functions (will be imported from CLI package)
var (
	warningColor = func(s string) string { return s }
	infoColor    = func(s string) string { return s }
)
