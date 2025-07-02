// internal/docker/service_starters.go
package docker

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/localcloud-sh/localcloud/internal/models"
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

// Start starts the AI service with proper volume mounting
func (s *AIServiceStarter) Start() error {
	// Check and pull image
	if err := s.ensureImage("ollama/ollama:latest"); err != nil {
		return err
	}

	// Determine the Ollama models directory based on OS
	var ollamaModelsPath string
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}

	switch runtime.GOOS {
	case "darwin": // macOS
		ollamaModelsPath = filepath.Join(homeDir, ".ollama", "models")
	case "linux":
		ollamaModelsPath = filepath.Join(homeDir, ".ollama", "models")
	case "windows":
		ollamaModelsPath = filepath.Join(homeDir, ".ollama", "models")
	default:
		ollamaModelsPath = filepath.Join(homeDir, ".ollama", "models")
	}

	// Check if we should use host's Ollama models or project-specific volume
	useHostModels := false
	if _, err := os.Stat(ollamaModelsPath); err == nil {
		// Host has Ollama models directory
		useHostModels = true
		fmt.Printf("DEBUG: Found host Ollama models at: %s\n", ollamaModelsPath)
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
		Networks:      []string{fmt.Sprintf("localcloud_%s_default", s.manager.config.Project.Name)},
		RestartPolicy: "unless-stopped",
		HealthCheck: &HealthCheckConfig{
			Test:        []string{"CMD-SHELL", "wget --no-verbose --tries=1 --spider http://localhost:11434/api/tags || exit 1"},
			Interval:    60,
			Timeout:     30,
			Retries:     5,
			StartPeriod: 120,
		},
		Labels: map[string]string{
			"com.localcloud.project": s.manager.config.Project.Name,
			"com.localcloud.service": "ai",
		},
	}

	// Configure volume mounting
	if useHostModels {
		// Bind mount the host's Ollama directory to share models
		config.Volumes = []VolumeMount{
			{
				Type:     "bind",
				Source:   filepath.Dir(ollamaModelsPath), // Mount the parent .ollama directory
				Target:   "/root/.ollama",
				ReadOnly: false,
			},
		}
		fmt.Println("ℹ Using host Ollama models directory")
	} else {
		// Use project-specific volume
		config.Volumes = []VolumeMount{
			{
				Source: fmt.Sprintf("localcloud_%s_ollama_models", s.manager.config.Project.Name),
				Target: "/root/.ollama",
			},
		}
		fmt.Println("ℹ Using project-specific models volume")
	}

	// Create and start container
	containerID, err := s.manager.container.Create(config)
	if err != nil {
		return err
	}

	fmt.Printf("DEBUG: Container %s created successfully\n", config.Name)

	if err := s.manager.container.Start(containerID); err != nil {
		return err
	}

	// Wait for Ollama to be ready
	if err := s.waitForOllama(); err != nil {
		return fmt.Errorf("ollama failed to start: %w", err)
	}

	fmt.Println("DEBUG: Service ai started successfully")

	// Post-start tasks
	return s.postStart()
}

func (s *AIServiceStarter) postStart() error {
	cfg := s.manager.config
	if len(cfg.Services.AI.Models) == 0 {
		return nil
	}

	// Create model manager
	modelManager := models.NewManager(fmt.Sprintf("http://localhost:%d", cfg.Services.AI.Port))

	// Wait a bit more for Ollama to stabilize
	time.Sleep(2 * time.Second)

	// Check installed models
	installedModels, err := modelManager.List()
	if err != nil {
		// Not critical, Ollama might still be starting
		fmt.Printf("DEBUG: Could not list models: %v\n", err)
		return nil
	}

	fmt.Printf("DEBUG: Found %d installed models\n", len(installedModels))

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

	// Check if configured models are installed
	for _, modelName := range cfg.Services.AI.Models {
		installed := false
		for _, inst := range installedModels {
			if inst.Name == modelName || inst.Model == modelName {
				installed = true
				break
			}
		}

		if !installed {
			fmt.Printf("\n%s Default model '%s' is not installed.\n", warningColor("!"), modelName)
			fmt.Printf("Run '%s' to download it.\n", infoColor(fmt.Sprintf("lc models pull %s", modelName)))
		}
	}

	// Show configured models by type
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

func (s *AIServiceStarter) ensureImage(image string) error {
	imageManager := s.manager.GetClient().NewImageManager()

	exists, err := imageManager.Exists(image)
	if err != nil {
		return fmt.Errorf("failed to check image: %w", err)
	}

	if !exists {
		fmt.Printf("Pulling %s image...\n", image)
		progress := make(chan PullProgress)
		done := make(chan error)

		go func() {
			done <- imageManager.Pull(image, progress)
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

	// Set default values if not configured
	maxMemory := s.manager.config.Services.Cache.MaxMemory
	if maxMemory == "" {
		maxMemory = "512mb" // Default value
	}

	maxMemoryPolicy := s.manager.config.Services.Cache.MaxMemoryPolicy
	if maxMemoryPolicy == "" {
		maxMemoryPolicy = "allkeys-lru" // Default policy
	}

	// Create container config
	config := ContainerConfig{
		Name:  "localcloud-redis",
		Image: "redis:7-alpine",
		Command: []string{
			"redis-server",
			"--maxmemory", maxMemory,
			"--maxmemory-policy", maxMemoryPolicy,
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

	// Set default values if not configured
	maxMemory := s.manager.config.Services.Queue.MaxMemory
	if maxMemory == "" {
		maxMemory = "1gb" // Default for queue
	}

	maxMemoryPolicy := s.manager.config.Services.Queue.MaxMemoryPolicy
	if maxMemoryPolicy == "" {
		maxMemoryPolicy = "noeviction" // Default for queue
	}

	appendFsync := s.manager.config.Services.Queue.AppendFsync
	if appendFsync == "" {
		appendFsync = "everysec" // Default fsync policy
	}

	// Build Redis command for queue
	redisCmd := []string{
		"redis-server",
		"--maxmemory", maxMemory,
		"--maxmemory-policy", maxMemoryPolicy,
	}

	// Add persistence options if enabled
	if s.manager.config.Services.Queue.Persistence {
		redisCmd = append(redisCmd,
			"--appendonly", "yes",
			"--appendfsync", appendFsync,
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

	// Create container config WITHOUT health check
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
		// REMOVED HealthCheck configuration
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

	// Wait for MinIO to be ready using proper method
	fmt.Println("⏳ Waiting for MinIO to start...")
	if err := s.waitForMinIOReady(containerID); err != nil {
		// Get container logs for debugging
		logs, _ := s.manager.GetContainerLogs(containerID, false, "50")
		fmt.Printf("DEBUG: MinIO container logs:\n%s\n", string(logs))
		return err
	}

	// Post-start: create default bucket
	return s.createDefaultBucket()
}

// waitForMinIOReady waits for MinIO to be ready using TCP checks
func (s *StorageServiceStarter) waitForMinIOReady(containerID string) error {
	timeout := 60 * time.Second
	deadline := time.Now().Add(timeout)
	startTime := time.Now()

	// First, ensure container is running using our Inspect method
	for i := 0; i < 10; i++ {
		info, err := s.manager.container.Inspect(containerID)
		if err != nil {
			return fmt.Errorf("failed to inspect container: %w", err)
		}

		if info.State == "running" {
			break
		}

		if i == 9 {
			return fmt.Errorf("container failed to start (state: %s)", info.State)
		}
		time.Sleep(1 * time.Second)
	}

	// Now wait for MinIO to be ready
	for time.Now().Before(deadline) {
		// Try TCP connection
		address := fmt.Sprintf("localhost:%d", s.manager.config.Services.Storage.Port)
		conn, err := net.DialTimeout("tcp", address, 5*time.Second)
		if err == nil {
			conn.Close()

			// Also try HTTP health check
			client := &http.Client{Timeout: 5 * time.Second}
			healthURL := fmt.Sprintf("http://localhost:%d/minio/health/live",
				s.manager.config.Services.Storage.Port)

			resp, httpErr := client.Get(healthURL)
			if httpErr == nil {
				resp.Body.Close()
				if resp.StatusCode == http.StatusOK {
					fmt.Println("✓ MinIO is ready")
					return nil
				}
			}

			// TCP is working but HTTP might not be ready yet
			// If we've been waiting for more than 10 seconds with TCP working, assume it's ready
			if time.Since(startTime) > 10*time.Second {
				fmt.Println("✓ MinIO TCP port is ready")
				return nil
			}
		}

		// Show progress
		elapsed := time.Since(startTime)
		fmt.Printf("\r⏳ Waiting for MinIO... %ds", int(elapsed.Seconds()))

		time.Sleep(2 * time.Second)
	}

	return fmt.Errorf("timeout waiting for MinIO to be ready after %v", timeout)
}

// ensureImage checks and pulls image if needed
func (s *StorageServiceStarter) ensureImage(image string) error {
	starter := &AIServiceStarter{manager: s.manager}
	return starter.ensureImage(image)
}

// createDefaultBucket creates the default bucket after MinIO starts
func (s *StorageServiceStarter) createDefaultBucket() error {
	// Wait a bit more to ensure MinIO is fully ready
	time.Sleep(3 * time.Second)

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

	// Create default bucket with retries
	bucketName := "localcloud"
	ctx := context.Background()

	var lastErr error
	for i := 0; i < 5; i++ {
		err = minioClient.MakeBucket(ctx, bucketName, minio.MakeBucketOptions{})
		if err != nil {
			// Check if bucket already exists
			exists, errBucketExists := minioClient.BucketExists(ctx, bucketName)
			if errBucketExists == nil && exists {
				// Bucket already exists, not an error
				err = nil
				break
			}

			lastErr = err
			if i < 4 {
				// Retry after a short delay
				time.Sleep(3 * time.Second)
				continue
			}
		} else {
			// Success
			break
		}
	}

	if err != nil {
		return fmt.Errorf("failed to create bucket after retries: %w (last error: %v)", err, lastErr)
	}

	// Save credentials to file for user reference
	credsPath := filepath.Join(os.Getenv("HOME"), ".localcloud", "minio-credentials")
	os.MkdirAll(filepath.Dir(credsPath), 0700)

	credsContent := fmt.Sprintf(`MinIO Credentials
=================
Endpoint: http://localhost:%d
Access Key: localcloud
Secret Key: %s
Console: http://localhost:%d

To access MinIO console, open: http://localhost:%d
Login with the credentials above.
`,
		s.manager.config.Services.Storage.Port,
		s.rootPassword,
		s.manager.config.Services.Storage.Console,
		s.manager.config.Services.Storage.Console,
	)

	if err := os.WriteFile(credsPath, []byte(credsContent), 0600); err != nil {
		// Non-fatal error, just log it
		fmt.Printf("Warning: Could not save credentials to file: %v\n", err)
	}

	fmt.Printf("\n✓ MinIO storage service started successfully\n")
	fmt.Printf("  Console: http://localhost:%d\n", s.manager.config.Services.Storage.Console)
	fmt.Printf("  Credentials saved to: %s\n", credsPath)

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
