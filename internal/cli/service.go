// internal/cli/service.go
package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/briandowns/spinner"
	"github.com/fatih/color"
	"github.com/localcloud/localcloud/internal/config"
	"github.com/localcloud/localcloud/internal/docker"
	"github.com/localcloud/localcloud/internal/services"
	"github.com/localcloud/localcloud/internal/templates"
	"github.com/spf13/cobra"
	"os"
	"strings"
	"time"
)

var (
	servicePort   int
	serviceEnv    []string
	serviceDetach bool
	serviceFollow bool
	// Removed serviceVerbose as it conflicts with global verbose
)

// Helper functions
func isLocalCloudProject() bool {
	_, err := os.Stat(".localcloud/config.yaml")
	return err == nil
}

func loadConfig() (*config.Config, error) {
	return config.Get(), nil
}

// serviceCmd represents the service command
var serviceCmd = &cobra.Command{
	Use:   "service",
	Short: "Manage individual services",
	Long:  `Manage LocalCloud services including starting, stopping, and querying service information.`,
}

// servicesCmd lists all services
var servicesCmd = &cobra.Command{
	Use:     "services",
	Aliases: []string{"svcs", "ls"},
	Short:   "List all running services",
	Long:    `List all running LocalCloud services with their status and connection information.`,
	RunE:    runServicesList,
}

// serviceStartCmd starts a specific service
var serviceStartCmd = &cobra.Command{
	Use:   "start [service]",
	Short: "Start a specific service",
	Long: `Start a specific LocalCloud service dynamically.
    
Supported services:
  Core Services:
    • ai, llm, ollama      - AI/LLM service
    • postgres, db, vector-db         - PostgreSQL database
    • cache, redis-cache   - Redis cache
    • queue, redis-queue   - Redis queue
    • minio, storage, s3   - MinIO object storage

  AI Components:
    • whisper, stt, speech-to-text    - Speech recognition
    • tts, text-to-speech, piper      - Text to speech
    • stable-diffusion, image-gen     - Image generation
    • qdrant, vector, vector-db       - Vector database

Examples:
  lc service start whisper
  lc service start tts --port 10201
  lc service start qdrant`,
	Args: cobra.ExactArgs(1),
	RunE: runServiceStart,
}

// serviceRestartCmd restarts a specific service
var serviceRestartCmd = &cobra.Command{
	Use:   "restart [service]",
	Short: "Restart a specific service",
	Long:  `Restart a running LocalCloud service.`,
	Args:  cobra.ExactArgs(1),
	RunE:  runServiceRestart,
}

// serviceStopCmd stops a specific service
var serviceStopCmd = &cobra.Command{
	Use:     "stop [service]",
	Short:   "Stop a specific service",
	Args:    cobra.ExactArgs(1),
	RunE:    runServiceStop,
	Aliases: []string{"down"},
}

// serviceStatusCmd shows status of a specific service
var serviceStatusCmd = &cobra.Command{
	Use:     "status [service]",
	Short:   "Show status of a specific service",
	Args:    cobra.ExactArgs(1),
	RunE:    runServiceStatus,
	Aliases: []string{"info"},
}

// serviceURLCmd shows the URL of a specific service
var serviceURLCmd = &cobra.Command{
	Use:   "url [service]",
	Short: "Get the URL of a service",
	Args:  cobra.ExactArgs(1),
	RunE:  runServiceURL,
}

var (
	jsonOutput bool
	urlOnly    bool
	detailed   bool
)

func init() {
	rootCmd.AddCommand(serviceCmd)
	rootCmd.AddCommand(servicesCmd)

	serviceCmd.AddCommand(serviceStartCmd)
	serviceCmd.AddCommand(serviceStopCmd)
	serviceCmd.AddCommand(serviceStatusCmd)
	serviceCmd.AddCommand(serviceURLCmd)
	serviceCmd.AddCommand(serviceRestartCmd)

	// Add flags
	servicesCmd.Flags().BoolVar(&jsonOutput, "json", false, "Output in JSON format")
	servicesCmd.Flags().BoolVar(&detailed, "detailed", false, "Show detailed information including service types")
	// Removed duplicate -d flag definition
	serviceURLCmd.Flags().BoolVar(&urlOnly, "url", true, "Output URL only")

	// Service start flags
	serviceStartCmd.Flags().IntVar(&servicePort, "port", 0, "Override default port")
	serviceStartCmd.Flags().StringSliceVarP(&serviceEnv, "env", "e", []string{}, "Set environment variables")
	serviceStartCmd.Flags().BoolVar(&serviceDetach, "detach", true, "Run in background") // Removed -d shorthand
	serviceStartCmd.Flags().BoolVarP(&serviceFollow, "follow", "f", false, "Follow service logs")
	// Removed verbose flag as it conflicts with global verbose

	// Add shorthand commands at root level for convenience
	rootCmd.AddCommand(&cobra.Command{
		Use:   "start [service]",
		Short: "Start a service or all services",
		Long: `Start LocalCloud services.

Without arguments, starts all configured services.
With a service name, starts only that specific service.

Examples:
  lc start           # Start all services
  lc start whisper   # Start only Whisper service`,
		RunE: runStartService,
	})

	rootCmd.AddCommand(&cobra.Command{
		Use:   "stop [service]",
		Short: "Stop a service or all services",
		Long: `Stop LocalCloud services.

Without arguments, stops all services.
With a service name, stops only that specific service.

Examples:
  lc stop           # Stop all services
  lc stop whisper   # Stop only Whisper service`,
		RunE: runStopService,
	})
}

// formatServiceDisplayName converts service names to user-friendly display
func formatServiceDisplayName(name string) string {
	// Convert 'ai' to 'llm' for clarity
	if name == "ai" {
		return "llm"
	}
	return name
}

// getServiceTypeAndModel returns service type and model information
func getServiceTypeAndModel(serviceName string, cfg *config.Config) (serviceType, model string) {
	serviceType = "-"
	model = "-"

	switch serviceName {
	case "ai", "llm":
		serviceType = "ollama"
		// Get active model from config
		if cfg.Services.AI.Default != "" {
			model = cfg.Services.AI.Default
		} else {
			model = "qwen2.5:3b" // default
		}
	case "embeddings":
		serviceType = "ollama"
		model = "nomic-embed" // or get from config
	case "postgres", "postgresql":
		serviceType = "database"
		if hasExtension(cfg, "vector") {
			model = "pgvector"
		}
	case "redis":
		serviceType = "cache"
	case "speech-to-text", "whisper":
		serviceType = "whisper"
		model = "base" // or get from config
	case "text-to-speech", "tts", "piper":
		serviceType = "piper"
		model = "en_US-amy" // or get from config
	case "vector-db", "pgvector":
		serviceType = "database"
		model = "pgvector"
	case "storage", "minio":
		serviceType = "storage"
		model = "s3"
	case "image-generation", "stable-diffusion":
		serviceType = "sd-webui"
		model = "sdxl" // or get from config
	}

	return serviceType, model
}

// hasExtension checks if a PostgreSQL extension is enabled
func hasExtension(cfg *config.Config, extension string) bool {
	// Check if extension is in the config
	// This is a simplified check - implement based on actual config structure
	return extension == "vector" // For now, assume pgvector is always enabled
}

func runServiceStart(cmd *cobra.Command, args []string) error {
	serviceName := args[0]

	if !isLocalCloudProject() {
		return fmt.Errorf("not in a LocalCloud project directory")
	}

	cfg, err := loadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	projectName := cfg.Project.Name
	if projectName == "" {
		projectName = "default"
	}

	ctx := context.Background()
	dockerManager, err := docker.NewManager(ctx, cfg)
	if err != nil {
		if strings.Contains(err.Error(), "Docker daemon not running") {
			return fmt.Errorf("Docker is not running. Please start Docker Desktop")
		}
		return fmt.Errorf("failed to create Docker manager: %w", err)
	}

	portManager := templates.NewPortManager()
	serviceManager := services.NewServiceManager(projectName, dockerManager, portManager)

	// Parse component name to get service config
	normalizedName, componentConfig := serviceManager.ParseComponentName(serviceName)

	var serviceConfig services.ServiceConfig
	if componentConfig != nil {
		serviceConfig = *componentConfig
	} else {
		// Fallback to legacy getServiceConfig
		legacyConfig := getServiceConfig(serviceName)
		if legacyConfig != nil {
			serviceConfig = *legacyConfig
		} else {
			return fmt.Errorf("unknown service or component: %s", serviceName)
		}
	}

	// Override port if specified
	if servicePort > 0 {
		serviceConfig.PreferredPort = servicePort
	}

	// Add custom environment variables
	if len(serviceEnv) > 0 {
		if serviceConfig.Environment == nil {
			serviceConfig.Environment = make(map[string]string)
		}
		for _, env := range serviceEnv {
			parts := strings.SplitN(env, "=", 2)
			if len(parts) == 2 {
				serviceConfig.Environment[parts[0]] = parts[1]
			}
		}
	}

	// Show starting message with spinner
	var spin *spinner.Spinner
	if !verbose && !serviceFollow { // Changed from serviceVerbose to verbose
		spin = spinner.New(spinner.CharSets[14], 100*time.Millisecond)
		spin.Suffix = fmt.Sprintf(" Starting %s...", serviceName)
		spin.Start()
	} else {
		fmt.Printf("Starting %s service...\n", serviceName)
	}

	// Start the service
	err = serviceManager.StartService(serviceName, serviceConfig)

	if spin != nil {
		spin.Stop()
	}

	if err != nil {
		printError(fmt.Sprintf("Failed to start %s: %v", serviceName, err))
		return err
	}

	// Get service info
	service, err := serviceManager.GetServiceStatus(normalizedName)
	if err != nil {
		service, _ = serviceManager.GetServiceStatus(serviceName)
	}

	fmt.Printf("\n✓ %s started successfully!\n", strings.Title(serviceName))
	fmt.Printf("  URL: %s\n", service.URL)

	// Show service-specific examples
	showServiceExamples(serviceName, service.Port)

	return nil
}

// Enhanced start/stop commands that support individual services
func runStartService(cmd *cobra.Command, args []string) error {
	if len(args) > 0 {
		// Start specific service
		return runServiceStart(cmd, args)
	}

	// Start all services (existing behavior)
	return runStart(cmd, args)
}

func runStopService(cmd *cobra.Command, args []string) error {
	if len(args) > 0 {
		// Stop specific service
		return runServiceStop(cmd, args)
	}

	// Stop all services (existing behavior)
	return runStop(cmd, args)
}

// getServiceConfig returns configuration for known service types
func getServiceConfig(name string) *services.ServiceConfig {
	configs := map[string]services.ServiceConfig{
		"whisper": {
			Name:          "whisper",
			Image:         "onerahmet/openai-whisper-asr-webservice:latest",
			PreferredPort: 9000,
			Environment: map[string]string{
				"ASR_MODEL": "base",
			},
		},
		"speech-to-text": {
			Name:          "whisper",
			Image:         "onerahmet/openai-whisper-asr-webservice:latest",
			PreferredPort: 9000,
			Environment: map[string]string{
				"ASR_MODEL": "base",
			},
		},
		"stt": {
			Name:          "whisper",
			Image:         "onerahmet/openai-whisper-asr-webservice:latest",
			PreferredPort: 9000,
			Environment: map[string]string{
				"ASR_MODEL": "base",
			},
		},
		"vector-db": {
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
		},
		"pgvector": {
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
		},
		"qdrant": {
			Name:          "qdrant",
			Image:         "qdrant/qdrant:latest",
			PreferredPort: 6333,
			Volumes: []string{
				"qdrant_data:/qdrant/storage",
			},
		},
		"minio": {
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
		"storage": {
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
		"text-to-speech": {
			Name:          "piper",
			Image:         "lscr.io/linuxserver/piper:latest",
			PreferredPort: 10200,
			Environment: map[string]string{
				"PIPER_VOICE": "en_US-amy-medium",
			},
		},
		"tts": {
			Name:          "piper",
			Image:         "lscr.io/linuxserver/piper:latest",
			PreferredPort: 10200,
			Environment: map[string]string{
				"PIPER_VOICE": "en_US-amy-medium",
			},
		},
		"image-generation": {
			Name:          "stable-diffusion-webui",
			Image:         "ghcr.io/automattic/stable-diffusion-webui:latest",
			PreferredPort: 7860,
			Volumes: []string{
				"sd_models:/stable-diffusion-webui/models",
			},
		},
		"image": {
			Name:          "stable-diffusion-webui",
			Image:         "ghcr.io/automattic/stable-diffusion-webui:latest",
			PreferredPort: 7860,
			Volumes: []string{
				"sd_models:/stable-diffusion-webui/models",
			},
		},

		"localllama": {
			Name:          "localllama",
			Image:         "localcloud/localllama:latest",
			PreferredPort: 8081,
			Environment: map[string]string{
				"MODELS_PATH": "/models",
			},
			Volumes: []string{
				"localllama_models:/models",
			},
		},
	}

	normalized := strings.ToLower(strings.TrimSpace(name))
	if config, exists := configs[normalized]; exists {
		return &config
	}

	return nil
}

// showServiceExamples displays usage examples for a service
func showServiceExamples(service string, port int) {
	fmt.Println("\n  Try:")

	// Normalize service name to component type for examples
	componentType := ""
	switch service {
	case "whisper", "speech-to-text", "stt":
		componentType = "speech-to-text"
	case "vector-db", "vectordb", "pgvector", "qdrant":
		componentType = "vector-db"
	case "storage", "minio", "s3":
		componentType = "storage"
	case "text-to-speech", "tts", "piper":
		componentType = "text-to-speech"
	case "image-generation", "image-gen", "image", "stable-diffusion":
		componentType = "image-generation"
	default:
		componentType = service
	}

	switch componentType {
	case "speech-to-text":
		fmt.Println("    # Transcribe audio file")
		fmt.Printf("    curl -X POST http://localhost:%d/asr \\\n", port)
		fmt.Println("      -F \"audio_file=@sample.wav\" \\")
		fmt.Println("      -F \"language=en\"")

	case "vector-db":
		if service == "pgvector" || service == "vector-db" {
			fmt.Println("    # Create table with embeddings")
			fmt.Println("    psql $DATABASE_URL -c \"CREATE TABLE docs (id serial, embedding vector(1536))\"")
			fmt.Println()
			fmt.Println("    # Insert embedding")
			fmt.Println("    psql $DATABASE_URL -c \"INSERT INTO docs (embedding) VALUES ('[1,2,3]')\"")
		} else {
			fmt.Println("    # Create collection")
			fmt.Printf("    curl -X PUT http://localhost:%d/collections/test_collection \\\n", port)
			fmt.Println("      -H \"Content-Type: application/json\" \\")
			fmt.Println("      -d '{\"vectors\": {\"size\": 384, \"distance\": \"Cosine\"}}'")
		}

	case "storage":
		fmt.Printf("    # MinIO Console: http://localhost:%d\n", port+1)
		fmt.Println("    # Login: localcloud / localcloud123")
		fmt.Println()
		fmt.Println("    # Create bucket (using mc client)")
		fmt.Printf("    mc alias set local http://localhost:%d localcloud localcloud123\n", port)
		fmt.Println("    mc mb local/my-bucket")

	case "text-to-speech":
		fmt.Println("    # Generate speech")
		fmt.Printf("    curl -X POST http://localhost:%d/api/tts \\\n", port)
		fmt.Println("      -H \"Content-Type: application/json\" \\")
		fmt.Println("      -d '{\"text\": \"Hello, world!\", \"speaker\": \"p270\"}'")

	case "image-generation":
		fmt.Printf("    # Web UI: http://localhost:%d\n", port)
		fmt.Println()
		fmt.Println("    # Generate image via API")
		fmt.Printf("    curl -X POST http://localhost:%d/sdapi/v1/txt2img \\\n", port)
		fmt.Println("      -H \"Content-Type: application/json\" \\")
		fmt.Println("      -d '{\"prompt\": \"a beautiful landscape\", \"steps\": 20}'")
	}
}

func runServiceStop(cmd *cobra.Command, args []string) error {
	serviceName := args[0]

	if !isLocalCloudProject() {
		return fmt.Errorf("not in a LocalCloud project directory")
	}

	cfg, err := loadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	projectName := cfg.Project.Name
	if projectName == "" {
		projectName = "default"
	}

	ctx := context.Background()
	dockerManager, err := docker.NewManager(ctx, cfg)
	if err != nil {
		return fmt.Errorf("failed to create Docker manager: %w", err)
	}

	portManager := templates.NewPortManager()
	serviceManager := services.NewServiceManager(projectName, dockerManager, portManager)

	// Use resolveServiceName to find the actual service name
	actualServiceName, err := resolveServiceName(serviceManager, serviceName)
	if err != nil {
		return err
	}

	fmt.Printf("Stopping %s service...\n", serviceName)

	// Stop the service
	if err := serviceManager.StopService(actualServiceName); err != nil {
		return fmt.Errorf("failed to stop %s: %w", serviceName, err)
	}

	fmt.Printf("✓ %s stopped\n", serviceName)
	return nil
}

func runServiceURL(cmd *cobra.Command, args []string) error {
	serviceName := args[0]

	if !isLocalCloudProject() {
		return fmt.Errorf("not in a LocalCloud project directory")
	}

	cfg, err := loadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	projectName := cfg.Project.Name
	if projectName == "" {
		projectName = "default"
	}

	ctx := context.Background()
	dockerManager, err := docker.NewManager(ctx, cfg)
	if err != nil {
		return fmt.Errorf("failed to create Docker manager: %w", err)
	}

	portManager := templates.NewPortManager()
	serviceManager := services.NewServiceManager(projectName, dockerManager, portManager)

	// Use resolveServiceName to find the actual service name
	actualServiceName, err := resolveServiceName(serviceManager, serviceName)
	if err != nil {
		return err
	}

	// Get service URL
	url, err := serviceManager.GetServiceURL(actualServiceName)
	if err != nil {
		return err
	}

	if urlOnly {
		fmt.Println(url)
	} else {
		fmt.Printf("%s: %s\n", serviceName, url)
	}

	return nil
}

func runServiceRestart(cmd *cobra.Command, args []string) error {
	serviceName := args[0]

	if !isLocalCloudProject() {
		return fmt.Errorf("not in a LocalCloud project directory")
	}

	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	projectName := cfg.Project.Name
	if projectName == "" {
		projectName = "default"
	}

	ctx := context.Background()
	dockerManager, err := docker.NewManager(ctx, cfg)
	if err != nil {
		return fmt.Errorf("failed to connect to Docker: %w", err)
	}
	defer dockerManager.Close()

	portManager := templates.NewPortManager()
	serviceManager := services.NewServiceManager(projectName, dockerManager, portManager)

	// Use resolveServiceName to find the actual service name
	actualServiceName, err := resolveServiceName(serviceManager, serviceName)
	if err != nil {
		return err
	}

	fmt.Printf("Restarting %s...\n", serviceName)
	if err := serviceManager.RestartService(actualServiceName); err != nil {
		return fmt.Errorf("failed to restart %s: %w", serviceName, err)
	}

	printSuccess(fmt.Sprintf("%s restarted", serviceName))
	return nil
}

// internal/cli/service.go - Updated resolveServiceName function and service status

// resolveServiceName tries to find the actual service name from an alias
func resolveServiceName(serviceManager *services.ServiceManager, inputName string) (string, error) {
	// Common service name mappings
	serviceAliases := map[string]string{
		// Cache aliases
		"cache":       "redis",
		"redis-cache": "redis",

		// Queue aliases
		"queue":       "redis-queue",
		"redis-queue": "redis-queue",

		// Database aliases
		"db":         "postgres",
		"database":   "postgres",
		"postgresql": "postgres",
		"pg":         "postgres",

		// AI aliases
		"ai":     "ai",
		"ollama": "ai",
		"llm":    "ai",

		// Storage aliases
		"storage": "minio",
		"s3":      "minio",
		"minio":   "minio",

		// Speech-to-text aliases
		"whisper":        "whisper",
		"stt":            "whisper",
		"speech-to-text": "whisper",

		// Text-to-speech aliases
		"tts":            "piper",
		"text-to-speech": "piper",
		"piper":          "piper",

		// Vector DB aliases
		"vector":    "pgvector",
		"vector-db": "pgvector",
		"pgvector":  "pgvector",
		"qdrant":    "qdrant",

		// Image generation aliases
		"image":            "stable-diffusion",
		"image-gen":        "stable-diffusion",
		"image-generation": "stable-diffusion",
		"stable-diffusion": "stable-diffusion",
		"sd":               "stable-diffusion",
	}

	// First check if it's an alias
	if actualName, exists := serviceAliases[strings.ToLower(inputName)]; exists {
		// Try to get service status with the mapped name
		if _, err := serviceManager.GetServiceStatus(actualName); err == nil {
			return actualName, nil
		}
		// If that fails, try some variations
		// For Redis services, container names might be different
		if actualName == "redis" {
			// Try localcloud-redis
			if _, err := serviceManager.GetServiceStatus("localcloud-redis"); err == nil {
				return "localcloud-redis", nil
			}
		}
	}

	// Try the input name directly
	if _, err := serviceManager.GetServiceStatus(inputName); err == nil {
		return inputName, nil
	}

	// Try with localcloud- prefix
	prefixedName := "localcloud-" + inputName
	if _, err := serviceManager.GetServiceStatus(prefixedName); err == nil {
		return prefixedName, nil
	}

	// List all services and try to find a match
	allServices := serviceManager.ListServices()

	// Debug: show all available services
	if len(allServices) > 0 {
		availableNames := []string{}
		for _, svc := range allServices {
			availableNames = append(availableNames, svc.Name)

			// Check if the service name contains our input
			if strings.Contains(strings.ToLower(svc.Name), strings.ToLower(inputName)) {
				return svc.Name, nil
			}
		}

		return "", fmt.Errorf("service '%s' not found. Available services: %s", inputName, strings.Join(availableNames, ", "))
	}

	return "", fmt.Errorf("service '%s' not found (no services are running)", inputName)
}

// Updated runServiceStatus to show better error messages
func runServiceStatus(cmd *cobra.Command, args []string) error {
	serviceName := args[0]

	if !isLocalCloudProject() {
		return fmt.Errorf("not in a LocalCloud project directory")
	}

	cfg, err := loadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	projectName := cfg.Project.Name
	if projectName == "" {
		projectName = "default"
	}

	ctx := context.Background()
	dockerManager, err := docker.NewManager(ctx, cfg)
	if err != nil {
		if strings.Contains(err.Error(), "Docker daemon not running") {
			return fmt.Errorf("Docker is not running. Please start Docker Desktop")
		}
		return fmt.Errorf("failed to create Docker manager: %w", err)
	}
	defer dockerManager.Close()

	portManager := templates.NewPortManager()
	serviceManager := services.NewServiceManager(projectName, dockerManager, portManager)

	// First, list all services to help with debugging
	if verbose {
		allServices := serviceManager.ListServices()
		fmt.Printf("DEBUG: Available services: %d\n", len(allServices))
		for _, svc := range allServices {
			fmt.Printf("  - %s (status: %s)\n", svc.Name, svc.Status)
		}
		fmt.Println()
	}

	// Use resolveServiceName to find the actual service name
	actualServiceName, err := resolveServiceName(serviceManager, serviceName)
	if err != nil {
		// If service not found, suggest running services list
		fmt.Printf("Error: %v\n\n", err)
		fmt.Println("Tip: Run 'lc services' to see all available services")
		return nil
	}

	// Get service status
	service, err := serviceManager.GetServiceStatus(actualServiceName)
	if err != nil {
		return fmt.Errorf("failed to get status for %s: %w", actualServiceName, err)
	}

	// Display status
	fmt.Printf("Service: %s", service.Name)
	if service.Name != serviceName {
		fmt.Printf(" (alias: %s)", serviceName)
	}
	fmt.Println()

	// Color the status
	statusColor := color.New(color.FgRed).SprintFunc()
	if service.Status == "running" {
		statusColor = color.New(color.FgGreen).SprintFunc()
	} else if service.Status == "starting" {
		statusColor = color.New(color.FgYellow).SprintFunc()
	}

	fmt.Printf("Status: %s\n", statusColor(service.Status))
	fmt.Printf("Port: %d\n", service.Port)
	fmt.Printf("URL: %s\n", service.URL)

	if service.Health != "" {
		healthColor := color.New(color.FgRed).SprintFunc()
		if service.Health == "healthy" {
			healthColor = color.New(color.FgGreen).SprintFunc()
		} else if service.Health == "starting" {
			healthColor = color.New(color.FgYellow).SprintFunc()
		}
		fmt.Printf("Health: %s\n", healthColor(service.Health))
	}

	if !service.StartedAt.IsZero() {
		fmt.Printf("Started: %s (up %s)\n",
			service.StartedAt.Format("2006-01-02 15:04:05"),
			time.Since(service.StartedAt).Round(time.Second))
	}

	// Show container info if verbose
	if verbose && service.ContainerID != "" {
		fmt.Printf("Container: %s\n", service.ContainerID)
	}

	return nil
}

// internal/cli/service.go - Replace the existing runServicesList function with this:

func runServicesList(cmd *cobra.Command, args []string) error {
	// Check if we're in a LocalCloud project
	if !isLocalCloudProject() {
		return fmt.Errorf("not in a LocalCloud project directory")
	}

	// Load configuration
	cfg, err := loadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Create Docker manager
	ctx := context.Background()
	dockerManager, err := docker.NewManager(ctx, cfg)
	if err != nil {
		if strings.Contains(err.Error(), "Docker daemon not running") {
			return fmt.Errorf("Docker is not running. Please start Docker Desktop")
		}
		return fmt.Errorf("failed to create Docker manager: %w", err)
	}
	defer dockerManager.Close()

	// Get service status from Docker directly (like lc status does)
	statuses, err := dockerManager.GetServicesStatus()
	if err != nil {
		return fmt.Errorf("failed to get service status: %w", err)
	}

	if len(statuses) == 0 {
		fmt.Println("No services are currently running.")
		fmt.Println("\nStart services with: lc start [service-name]")
		return nil
	}

	// Convert docker.ServiceStatus to services.Service for display
	var svcList []services.Service
	for _, status := range statuses {
		// Parse port from string
		var port int
		fmt.Sscanf(status.Port, "%d", &port)

		svc := services.Service{
			Name:   status.Name,
			Status: status.Status,
			Health: status.Health,
			Port:   port,
			URL:    fmt.Sprintf("http://localhost:%d", port),
		}
		svcList = append(svcList, svc)
	}

	if jsonOutput {
		// JSON output
		data, err := json.MarshalIndent(svcList, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(data))
		return nil
	}

	// Table output
	if detailed {
		// Detailed view with manual formatting
		fmt.Printf("%-10s %-12s %-15s %-6s %-30s %-10s\n",
			"SERVICE", "TYPE", "MODEL", "PORT", "URL", "STATUS")
		fmt.Printf("%-10s %-12s %-15s %-6s %-30s %-10s\n",
			"-------", "----", "-----", "----", "---", "------")

		for _, svc := range svcList {
			serviceType, model := getServiceTypeAndModel(svc.Name, cfg)
			fmt.Printf("%-10s %-12s %-15s %-6d %-30s %-10s\n",
				formatServiceDisplayName(svc.Name),
				serviceType,
				model,
				svc.Port,
				svc.URL,
				svc.Status,
			)
		}
	} else {
		// Simple view with manual formatting for better control
		fmt.Printf("%-10s %-12s %-12s %-6s\n", "SERVICE", "STATUS", "HEALTH", "PORT")
		fmt.Printf("%-10s %-12s %-12s %-6s\n", "-------", "------", "------", "----")

		for _, svc := range svcList {
			// Color status based on state
			statusDisplay := svc.Status
			if svc.Status == "running" {
				statusDisplay = successColor(svc.Status)
			} else {
				statusDisplay = errorColor(svc.Status)
			}

			// Color health based on state
			healthDisplay := svc.Health
			if svc.Health == "healthy" {
				healthDisplay = successColor(svc.Health)
			} else if svc.Health == "unhealthy" {
				healthDisplay = errorColor(svc.Health)
			} else if svc.Health == "starting" {
				healthDisplay = warningColor(svc.Health)
			} else {
				healthDisplay = "-"
			}

			// Format with fixed widths
			serviceName := formatServiceDisplayName(svc.Name)
			fmt.Printf("%-10s %-20s %-20s %-6d\n",
				serviceName,
				statusDisplay,
				healthDisplay,
				svc.Port,
			)
		}
	}

	return nil
}
