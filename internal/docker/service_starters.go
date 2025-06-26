// internal/docker/service_starters.go
package docker

import (
	"context"
	"crypto/rand"
	"encoding/base64"
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
			Timeout:     30,
			Retries:     5,
			StartPeriod: 60,
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
	installedModels, err := modelManager.List()
	if err != nil {
		// Not critical, Ollama might still be starting
		return nil
	}

	// Separate configured models by type
	var llmModels []string
	var embeddingModels []string

	for _, modelName := range cfg.Services.AI.Models {
		if models.IsEmbeddingModel(modelName) {
			embeddingModels = append(embeddingModels, modelName)
		} else {
			llmModels = append(llmModels, modelName)
		}
	}

	// Check if default model is installed
	defaultModel := cfg.Services.AI.Default
	if defaultModel == "" && len(llmModels) > 0 {
		defaultModel = llmModels[0]
	}

	defaultInstalled := false
	for _, model := range installedModels {
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

	// Show configured embedding models
	if len(embeddingModels) > 0 {
		fmt.Printf("\nConfigured embedding models:\n")
		for _, modelName := range embeddingModels {
			installed := false
			for _, inst := range installedModels {
				if inst.Name == modelName {
					installed = true
					break
				}
			}
			if installed {
				fmt.Printf("  ✓ %s\n", modelName)
			} else {
				fmt.Printf("  ✗ %s (not installed - run: lc models pull %s)\n", modelName, modelName)
			}
		}
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
		time.Sleep(2 * time.Second)
	}
	return fmt.Errorf("ollama failed to start after %d attempts", maxAttempts)
}

// PrintServiceInfo prints service information including embedding support
func (s *AIServiceStarter) PrintServiceInfo() {
	port := s.manager.config.Services.AI.Port

	// LLM Information
	fmt.Println("\n✓ LLM (Text generation)")
	fmt.Printf("  URL: http://localhost:%d/api/generate\n", port)
	fmt.Printf("  Chat: http://localhost:%d/api/chat\n", port)
	fmt.Println("  Try:")
	fmt.Println(`    curl http://localhost:11434/api/generate \`)
	fmt.Println(`      -d '{"model":"qwen2.5:3b","prompt":"Hello, world!"}'`)

	// Embedding Information
	fmt.Println("\n✓ Embeddings (Semantic search)")
	fmt.Printf("  URL: http://localhost:%d/api/embeddings\n", port)
	fmt.Println("  Models:")
	fmt.Println("    • nomic-embed-text (274MB, 768 dimensions)")
	fmt.Println("    • mxbai-embed-large (670MB, 1024 dimensions)")
	fmt.Println("    • all-minilm (46MB, 384 dimensions)")
	fmt.Println("  Try:")
	fmt.Println(`    # Generate embedding`)
	fmt.Println(`    curl http://localhost:11434/api/embeddings \`)
	fmt.Println(`      -d '{"model":"nomic-embed-text","prompt":"Hello world"}'`)
	fmt.Println()
	fmt.Println(`    # Python example`)
	fmt.Println(`    import requests`)
	fmt.Println(`    resp = requests.post('http://localhost:11434/api/embeddings',`)
	fmt.Println(`        json={'model': 'nomic-embed-text', 'prompt': 'Hello world'})`)
	fmt.Println(`    embedding = resp.json()['embedding']  # 768-dim vector`)
}

// ensureImage checks if image exists and pulls if needed
func (s *AIServiceStarter) ensureImage(image string) error {
	hasImage, err := s.manager.image.Exists(image)
	if err != nil {
		return fmt.Errorf("failed to check image: %w", err)
	}

	if !hasImage {
		fmt.Printf("Pulling %s image...\n", image)
		progress := make(chan PullProgress)
		done := make(chan error)

		go func() {
			done <- s.manager.image.Pull(image, progress)
		}()

		// Display progress
		for {
			select {
			case p := <-progress:
				if p.ProgressDetail.Total > 0 {
					percentage := int((p.ProgressDetail.Current * 100) / p.ProgressDetail.Total)
					fmt.Printf("\rPulling: %s [%d%%]", p.Status, percentage)
				} else {
					fmt.Printf("\rPulling: %s", p.Status)
				}
			case err := <-done:
				fmt.Println() // New line after progress
				return err
			}
		}
	}

	return nil
}

// Helper color functions (should be imported from CLI package in real implementation)
func warningColor(s string) string {
	return fmt.Sprintf("\033[33m%s\033[0m", s)
}

func infoColor(s string) string {
	return fmt.Sprintf("\033[36m%s\033[0m", s)
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
// Start starts the database service
func (s *DatabaseServiceStarter) Start() error {
	if s.manager.config.Services.Database.Type == "" {
		return nil // Database not configured
	}

	// Check if pgvector extension is requested
	hasPgVector := false
	for _, ext := range s.manager.config.Services.Database.Extensions {
		if ext == "pgvector" || ext == "vector" {
			hasPgVector = true
			break
		}
	}

	// Select appropriate image
	var image string
	if hasPgVector {
		// Use pgvector-enabled PostgreSQL image
		image = fmt.Sprintf("pgvector/pgvector:pg%s", s.manager.config.Services.Database.Version)
	} else {
		// Use standard PostgreSQL image
		image = fmt.Sprintf("postgres:%s-alpine", s.manager.config.Services.Database.Version)
	}

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
			"POSTGRES_PASSWORD": "localcloud",
			"POSTGRES_DB":       "localcloud",
			"PGDATA":            "/var/lib/postgresql/data/pgdata",
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

	// Add pgvector-specific label if enabled
	if hasPgVector {
		config.Labels["com.localcloud.pgvector"] = "enabled"
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

// QueueServiceStarter handles queue service startup
type QueueServiceStarter struct {
	manager *Manager
}

// NewQueueServiceStarter creates a new queue service starter
func NewQueueServiceStarter(m *Manager) ServiceStarter {
	return &QueueServiceStarter{manager: m}
}

// Start starts the queue service
func (s *QueueServiceStarter) Start() error {
	if s.manager.config.Services.Queue.Type == "" {
		return nil // Queue not configured
	}

	// Check and pull image
	if err := s.ensureImage("redis:7-alpine"); err != nil {
		return err
	}

	// Build Redis command for queue
	redisCmd := []string{
		"redis-server",
		"--maxmemory", s.manager.config.Services.Queue.MaxMemory,
		"--maxmemory-policy", s.manager.config.Services.Queue.MaxMemoryPolicy,
	}

	// Add persistence options if enabled
	if s.manager.config.Services.Queue.Persistence {
		redisCmd = append(redisCmd,
			"--appendonly", "yes",
			"--appendfsync", s.manager.config.Services.Queue.AppendFsync,
			// RDB snapshots for additional safety
			"--save", "900", "1", // After 900 sec (15 min) if at least 1 key changed
			"--save", "300", "10", // After 300 sec (5 min) if at least 10 keys changed
			"--save", "60", "10000", // After 60 sec if at least 10000 keys changed
		)
	}

	// Create container config
	config := ContainerConfig{
		Name:    "localcloud-redis-queue",
		Image:   "redis:7-alpine",
		Command: redisCmd,
		Ports: []PortBinding{
			{
				ContainerPort: "6379",
				HostPort:      fmt.Sprintf("%d", s.manager.config.Services.Queue.Port),
				Protocol:      "tcp",
			},
		},
		Volumes: []VolumeMount{
			{
				Source: fmt.Sprintf("localcloud_%s_redis_queue_data", s.manager.config.Project.Name),
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
			"com.localcloud.service": "queue",
			"com.localcloud.type":    "redis-queue",
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
func (s *QueueServiceStarter) ensureImage(image string) error {
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

	// Wait for health and create default bucket
	if err := s.manager.container.WaitHealthy(containerID, 30*time.Second); err != nil {
		return err
	}

	// Post-start: create default bucket
	return s.createDefaultBucket()
}

// ensureImage checks and pulls image if needed
func (s *StorageServiceStarter) ensureImage(image string) error {
	starter := &AIServiceStarter{manager: s.manager}
	return starter.ensureImage(image)
}

// createDefaultBucket creates the default bucket after MinIO starts
func (s *StorageServiceStarter) createDefaultBucket() error {
	// Wait a bit for MinIO to be fully ready
	time.Sleep(5 * time.Second)

	// Create MinIO client
	endpoint := fmt.Sprintf("localhost:%d", s.manager.config.Services.Storage.Port)
	accessKeyID := "localcloud"
	secretAccessKey := s.rootPassword
	useSSL := false

	minioClient, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKeyID, secretAccessKey, ""),
		Secure: useSSL,
	})
	if err != nil {
		return fmt.Errorf("failed to create MinIO client: %w", err)
	}

	// Create default bucket
	bucketName := "localcloud"
	ctx := context.Background()

	err = minioClient.MakeBucket(ctx, bucketName, minio.MakeBucketOptions{})
	if err != nil {
		// Check if bucket already exists
		exists, errBucketExists := minioClient.BucketExists(ctx, bucketName)
		if errBucketExists == nil && exists {
			// Bucket already exists, not an error
			return nil
		}
		return fmt.Errorf("failed to create bucket: %w", err)
	}

	// Save credentials to file for user reference
	credsPath := filepath.Join(os.Getenv("HOME"), ".localcloud", "minio-credentials")
	os.MkdirAll(filepath.Dir(credsPath), 0700)

	credsContent := fmt.Sprintf("MinIO Credentials\n=================\nEndpoint: http://localhost:%d\nAccess Key: localcloud\nSecret Key: %s\nConsole: http://localhost:%d\n",
		s.manager.config.Services.Storage.Port,
		s.rootPassword,
		s.manager.config.Services.Storage.Console,
	)

	os.WriteFile(credsPath, []byte(credsContent), 0600)

	return nil
}

// generateSecurePassword generates a secure random password
func generateSecurePassword() string {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		// Fallback to a default password if random generation fails
		return "localcloud-dev-2024"
	}
	return base64.URLEncoding.EncodeToString(bytes)[:32]
}
