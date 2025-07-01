// internal/cli/start.go
package cli

import (
	"context"
	"fmt"
	"github.com/localcloud/localcloud/internal/models"
	"strings"

	"github.com/briandowns/spinner"
	"github.com/localcloud/localcloud/internal/components"
	"github.com/localcloud/localcloud/internal/config"
	"github.com/localcloud/localcloud/internal/docker"
	"github.com/localcloud/localcloud/internal/network"
	"github.com/spf13/cobra"
)

var (
	startService string
	startOnly    []string
	noTunnel     bool
	showInfo     bool
)

var startCmd = &cobra.Command{
	Use:       "start [service]",
	Short:     "Start LocalCloud services",
	Long:      `Start all or specific LocalCloud services for the current project.`,
	Args:      cobra.MaximumNArgs(1),
	ValidArgs: []string{"ai", "postgres", "cache", "queue", "minio", "all"},
	Example: `  lc start           # Start all services
  lc start ai        # Start only AI service
  lc start cache     # Start only Cache
  lc start queue     # Start only Queue
  lc start --only ai,cache  # Start only AI and Cache`,
	RunE: runStart,
}

func init() {
	startCmd.Flags().StringSliceVar(&startOnly, "only", []string{}, "Start only specified services (comma-separated)")
	startCmd.Flags().BoolVar(&noTunnel, "no-tunnel", true, "Start tunnel connection")
	startCmd.Flags().BoolVar(&showInfo, "info", true, "Show connection info after start")
}

// internal/cli/start.go - Updated runStart function

func runStart(cmd *cobra.Command, args []string) error {
	if verbose {
		fmt.Println("DEBUG: runStart called")
		fmt.Printf("DEBUG: Args: %v\n", args)
		fmt.Printf("DEBUG: Only flag: %v\n", startOnly)
	}

	// Check if project is initialized
	if !IsProjectInitialized() {
		return fmt.Errorf("no LocalCloud project found. Run 'localcloud init' first")
	}

	// Get config
	cfg := config.Get()
	if cfg == nil {
		return fmt.Errorf("failed to load configuration")
	}

	// Get enabled components from config
	enabledComponents := getEnabledComponents(cfg)
	if len(enabledComponents) == 0 {
		// No components configured - show helpful message
		fmt.Println(warningColor("No components configured in this project."))
		fmt.Println("\nThis project was created without any components.")
		fmt.Println("To configure components, run:")
		fmt.Println("  • lc setup")
		fmt.Println("  • Or: lc component add <component-id>")
		fmt.Println("\nAvailable components:")
		fmt.Println("  • llm        - Large language models")
		fmt.Println("  • embedding  - Text embeddings")
		fmt.Println("  • vector     - Vector database (pgvector)")
		fmt.Println("  • cache      - Redis cache")
		fmt.Println("  • queue      - Redis queue")
		fmt.Println("  • storage    - Object storage (MinIO)")
		fmt.Println("  • stt        - Speech-to-text (Whisper)")
		return nil
	}

	// Show what components are configured
	fmt.Printf("Starting services for configured components: %s\n", strings.Join(enabledComponents, ", "))

	// Determine which services to start
	var servicesToStart []string

	if len(args) > 0 && args[0] != "all" {
		// Single service specified
		servicesToStart = []string{args[0]}
	} else if len(startOnly) > 0 {
		// --only flag used
		servicesToStart = startOnly
	} else {
		// Start services for enabled components only
		servicesToStart = components.ComponentsToServices(enabledComponents)

		if verbose {
			fmt.Printf("DEBUG: Enabled components: %v\n", enabledComponents)
			fmt.Printf("DEBUG: Services to start: %v\n", servicesToStart)
		}
	}

	// Create Docker manager
	ctx := context.Background()
	manager, err := docker.NewManager(ctx, cfg)
	if err != nil {
		if strings.Contains(err.Error(), "Docker daemon not running") {
			fmt.Println(FormatError(ErrDockerNotRunning))
			return err
		}
		fmt.Println(FormatError(err))
		return err
	}
	defer manager.Close()

	// Start only the services for enabled components
	progress := make(chan docker.ServiceProgress)
	done := make(chan error)

	// Run startup in goroutine
	go func() {
		if len(args) > 0 || len(startOnly) > 0 {
			// Specific services requested
			done <- manager.StartSelectedServices(servicesToStart, progress)
		} else {
			// Convert components to services and start them
			componentsAsServices := components.ComponentsToServices(enabledComponents)
			done <- manager.StartSelectedServices(componentsAsServices, progress)
		}
	}()

	// Handle progress updates
	var spin *spinner.Spinner
	if !verbose {
		spin = spinner.New(spinner.CharSets[14], 100)
		spin.Start()
	}

	hasErrors := false
	startedServices := make(map[string]bool)

	for {
		select {
		case p, ok := <-progress:
			if !ok {
				// Channel closed, services started
				if spin != nil {
					spin.Stop()
				}
				goto finished
			}

			switch p.Status {
			case "starting":
				if verbose {
					fmt.Printf("Starting %s...\n", p.Service)
				} else if spin != nil {
					spin.Suffix = fmt.Sprintf(" Starting %s...", p.Service)
				}
			case "started":
				if spin != nil {
					spin.Stop()
				}
				printSuccess(fmt.Sprintf("%s started", p.Service))
				startedServices[p.Service] = true
				if spin != nil && !verbose {
					spin.Start()
				}
			case "failed":
				hasErrors = true
				if spin != nil {
					spin.Stop()
				}
				// Fix: Check if p.Error is empty before using it
				errorMsg := p.Error
				if errorMsg == "" {
					errorMsg = "unknown error"
				}
				printError(fmt.Sprintf("%s failed: %s", p.Service, errorMsg))
				if verbose {
					fmt.Printf("DEBUG: Full error for %s: %s\n", p.Service, errorMsg)
				}
				if spin != nil && !verbose {
					spin.Start()
				}
			}
		}
	}

finished:
	// Wait for completion
	if err := <-done; err != nil {
		if verbose {
			fmt.Printf("DEBUG: StartServices returned error: %v\n", err)
		}
		printWarning("Some services failed to start")
		return err
	}

	// Print success message
	fmt.Println()
	if hasErrors {
		printWarning("Service startup completed with errors!")
	} else {
		printSuccess("Service startup complete!")

		// Show component-specific information
		if showInfo {
			showStartedServicesInfo(cfg, startedServices)
		}
	}

	// Show connection info
	if showInfo && !hasErrors {
		fmt.Println()
		showConnectionInfo(cfg)

		// Start tunnel if requested
		if !noTunnel && cfg.Connectivity.Enabled {
			fmt.Println()
			fmt.Println("Starting tunnel connection...")
			if err := startTunnel(cfg); err != nil {
				printWarning(fmt.Sprintf("Failed to start tunnel: %v", err))
			}
		}
	}

	return nil
}

// Helper function to check if a string is in a slice
func containsString(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// startTunnel starts the tunnel connection
func startTunnel(cfg *config.Config) error {
	tunnelMgr, err := network.NewTunnelManager(&cfg.Connectivity)
	if err != nil {
		return fmt.Errorf("failed to create tunnel manager: %w", err)
	}

	url, err := tunnelMgr.Connect(context.Background(), 3000) // Default to port 3000
	if err != nil {
		return err
	}

	printSuccess(fmt.Sprintf("Tunnel connected: %s", url))
	return nil
}

// Helper function to check if component exists (add this at the end of the file)
func componentFound(id string, components []string) bool {
	for _, comp := range components {
		if comp == id {
			return true
		}
	}
	return false
}

// internal/cli/start.go
// Updated showStartedServicesInfo function to show only relevant component info

func showStartedServicesInfo(cfg *config.Config, startedServices map[string]bool) {
	fmt.Println()

	// Check which components are running
	enabledComponents := getEnabledComponents(cfg)

	for _, compID := range enabledComponents {
		comp, _ := components.GetComponent(compID)

		// Check if component's services are running
		allRunning := true
		for _, service := range comp.Services {
			if !startedServices[service] {
				allRunning = false
				break
			}
		}

		if !allRunning {
			continue
		}

		switch compID {
		case "llm":
			fmt.Println("✓ LLM (Text generation)")
			fmt.Printf("  Chat: http://localhost:%d/api/chat\n", cfg.Services.AI.Port)
			fmt.Printf("  Generate: http://localhost:%d/api/generate\n", cfg.Services.AI.Port)
			fmt.Println("  Try:")
			fmt.Printf("    curl http://localhost:%d/api/generate \\\n", cfg.Services.AI.Port)
			// Find a configured LLM model
			var llmModel string
			for _, model := range cfg.Services.AI.Models {
				if !models.IsEmbeddingModel(model) {
					llmModel = model
					break
				}
			}
			if llmModel == "" {
				llmModel = "qwen2.5:3b" // fallback
			}
			fmt.Printf(`      -d '{"model":"%s","prompt":"Hello!"}'`, llmModel)
			fmt.Println()

		case "embedding":
			fmt.Println("✓ Embeddings (Semantic search)")
			fmt.Printf("  URL: http://localhost:%d/api/embeddings\n", cfg.Services.AI.Port)
			fmt.Println("  Try:")
			fmt.Printf("    curl http://localhost:%d/api/embeddings \\\n", cfg.Services.AI.Port)
			// Find configured embedding model
			var embModel string
			for _, model := range cfg.Services.AI.Models {
				if models.IsEmbeddingModel(model) {
					embModel = model
					break
				}
			}
			if embModel == "" {
				embModel = "nomic-embed-text" // fallback
			}
			fmt.Printf(`      -d '{"model":"%s","prompt":"Hello world"}'`, embModel)
			fmt.Println()

		case "vector":
			PrintPgVectorServiceInfo(cfg.Services.Database.Port)

		case "cache":
			PrintRedisCacheInfo(cfg.Services.Cache.Port)

		case "queue":
			PrintRedisQueueInfo(cfg.Services.Queue.Port)

		case "storage":
			fmt.Println("✓ Object Storage (MinIO)")
			fmt.Printf("  API: http://localhost:%d\n", cfg.Services.Storage.Port)
			fmt.Printf("  Console: http://localhost:%d\n", cfg.Services.Storage.Console)
			fmt.Println("  Credentials: see ~/.localcloud/minio-credentials")
			fmt.Println()

			//case "stt":
			//	// Whisper info
			//	fmt.Printf("✓ Speech-to-Text (Whisper)\n")
			//	fmt.Printf("  URL: http://localhost:%d\n", cfg.Services.Whisper.Port)
		}
	}
}

// Updated showConnectionInfo to show only configured services
func showConnectionInfo(cfg *config.Config) {
	fmt.Println("Connection Information:")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Printf("Project: %s\n", cfg.Project.Name)

	// Show model configuration
	if len(cfg.Services.AI.Models) > 0 {
		fmt.Printf("Models: %s\n", strings.Join(cfg.Services.AI.Models, ", "))
	}
	if cfg.Services.AI.Default != "" {
		fmt.Printf("Default: %s\n", cfg.Services.AI.Default)
	}

	// Show all configured services with their URLs
	fmt.Println("\nActive Services:")

	// Get enabled components to determine what to show
	enabledComponents := getEnabledComponents(cfg)

	// AI Service - check if port is configured (meaning it's enabled)
	if cfg.Services.AI.Port > 0 {
		fmt.Printf("✓ AI Models (Ollama): http://localhost:%d\n", cfg.Services.AI.Port)

		// Only show APIs for enabled components
		hasLLM := false
		hasEmbedding := false

		for _, comp := range enabledComponents {
			if comp == "llm" {
				hasLLM = true
			} else if comp == "embedding" {
				hasEmbedding = true
			}
		}

		if hasLLM {
			fmt.Printf("  - Chat API: http://localhost:%d/api/chat\n", cfg.Services.AI.Port)
			fmt.Printf("  - Generate API: http://localhost:%d/api/generate\n", cfg.Services.AI.Port)
		}

		if hasEmbedding {
			fmt.Printf("  - Embeddings API: http://localhost:%d/api/embeddings\n", cfg.Services.AI.Port)
		}
	}

	// Database Service - check if type is configured
	if cfg.Services.Database.Type != "" {
		fmt.Printf("✓ PostgreSQL: postgresql://localhost:%d\n", cfg.Services.Database.Port)
		if containsString(cfg.Services.Database.Extensions, "pgvector") {
			fmt.Printf("  - pgvector extension enabled\n")
		}
	}

	// Cache Service - check if type is configured
	if cfg.Services.Cache.Type != "" {
		fmt.Printf("✓ Redis Cache: redis://localhost:%d\n", cfg.Services.Cache.Port)
	}

	// Queue Service
	if cfg.Services.Queue.Type != "" {
		fmt.Printf("✓ Redis Queue: redis://localhost:%d\n", cfg.Services.Queue.Port)
		fmt.Printf("  - Persistent with AOF enabled\n")
	}

	// Storage Service
	if cfg.Services.Storage.Type != "" {
		fmt.Printf("✓ Object Storage (MinIO): http://localhost:%d\n", cfg.Services.Storage.Port)
		fmt.Printf("  - Console: http://localhost:%d\n", cfg.Services.Storage.Console)
		fmt.Printf("  - Access Key: minioadmin\n")
		fmt.Printf("  - Secret Key: minioadmin\n")
	}

	// Whisper Service
	if cfg.Services.Whisper.Type != "" {
		fmt.Printf("✓ Speech-to-Text: http://localhost:%d\n", cfg.Services.Whisper.Port)
	}

	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
}
