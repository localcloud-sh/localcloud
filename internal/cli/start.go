// internal/cli/start.go
package cli

import (
	"context"
	"fmt"
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

// internal/cli/start.go

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
		// No components configured - launch interactive wizard
		fmt.Println(warningColor("No components configured in this project."))
		fmt.Println("\n" + infoColor("Launching interactive setup wizard..."))
		fmt.Println()

		// Get project name from config or use default
		projectName := cfg.Project.Name
		if projectName == "" {
			projectName = "my-project"
		}

		// Run interactive wizard
		if err := RunInteractiveInit(projectName); err != nil {
			return fmt.Errorf("setup wizard failed: %w", err)
		}

		// Reload config after wizard
		if err := config.Init(configFile); err != nil {
			return fmt.Errorf("failed to reload configuration: %w", err)
		}

		// Get the new config
		cfg = config.Get()
		if cfg == nil {
			return fmt.Errorf("failed to load configuration after setup")
		}

		// Check if components were added
		enabledComponents = getEnabledComponents(cfg)
		if len(enabledComponents) == 0 {
			// User cancelled or didn't select any components
			fmt.Println("\nNo components were configured. Exiting.")
			return nil
		}

		// Continue with normal start flow
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
				printError(fmt.Sprintf("%s failed: %s", p.Service, p.Error))
				if verbose {
					fmt.Printf("DEBUG: Full error for %s: %v\n", p.Service, p.Error)
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

// showStartedServicesInfo displays information about started services based on components
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
			fmt.Println(`      -d '{"model":"qwen2.5:3b","prompt":"Hello!"}'`)
			fmt.Println()

		case "embedding":
			fmt.Println("✓ Embeddings (Semantic search)")
			fmt.Printf("  URL: http://localhost:%d/api/embeddings\n", cfg.Services.AI.Port)
			fmt.Println("  Try:")
			fmt.Printf("    curl http://localhost:%d/api/embeddings \\\n", cfg.Services.AI.Port)
			fmt.Println(`      -d '{"model":"nomic-embed-text","prompt":"Hello world"}'`)
			fmt.Println()

		case "vector":
			PrintPgVectorServiceInfo(cfg.Services.Database.Port)

		case "cache":
			fmt.Println("✓ Cache (Redis)")
			fmt.Printf("  URL: redis://localhost:%d\n", cfg.Services.Cache.Port)
			fmt.Println("  Try:")
			fmt.Printf("    redis-cli -p %d ping\n", cfg.Services.Cache.Port)
			fmt.Printf("    redis-cli -p %d set key value\n", cfg.Services.Cache.Port)
			fmt.Println()

		case "queue":
			fmt.Println("✓ Queue (Redis)")
			fmt.Printf("  URL: redis://localhost:%d\n", cfg.Services.Queue.Port)
			fmt.Println("  Try:")
			fmt.Printf("    redis-cli -p %d LPUSH jobs '{\"task\":\"process\"}'\n", cfg.Services.Queue.Port)
			fmt.Printf("    redis-cli -p %d BRPOP jobs 0\n", cfg.Services.Queue.Port)
			fmt.Println()

		case "storage":
			fmt.Println("✓ Object Storage (MinIO)")
			fmt.Printf("  API: http://localhost:%d\n", cfg.Services.Storage.Port)
			fmt.Printf("  Console: http://localhost:%d\n", cfg.Services.Storage.Console)
			fmt.Println("  Credentials: see ~/.localcloud/minio-credentials")
			fmt.Println()
		case "stt":
			fmt.Println("✓ Speech-to-Text (Whisper)")
			fmt.Printf("  URL: http://localhost:%d\n", cfg.Services.Whisper.Port)
			fmt.Println("  Try:")
			fmt.Println("    # Transcribe audio file")
			fmt.Printf("    curl -X POST http://localhost:%d/asr \\\n", cfg.Services.Whisper.Port)
			fmt.Println(`      -F "audio_file=@sample.wav" \`)
			fmt.Println(`      -F "language=en"`)
			fmt.Println()
			if cfg.Services.Whisper.Model != "" {
				fmt.Printf("  Model: %s\n", cfg.Services.Whisper.Model)
			}
			fmt.Println()
		}

	}
	if cfg.Services.Database.Type == "postgres" && startedServices["postgres"] {
		// Check if pgvector extension is enabled
		if !componentFound("vector", enabledComponents) {
			// Show regular PostgreSQL info or pgvector info based on extensions
			PrintPostgreSQLServiceInfo(cfg.Services.Database.Port, cfg.Services.Database.Extensions)
		}
	}
}

// showConnectionInfo displays connection information
func showConnectionInfo(cfg *config.Config) {
	fmt.Println("Connection Information:")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Printf("Project: %s\n", cfg.Project.Name)

	// Show model configuration
	if len(cfg.Services.AI.Models) > 0 {
		fmt.Printf("Models: %s\n", strings.Join(cfg.Services.AI.Models, ", "))
		if cfg.Services.AI.Default != "" {
			fmt.Printf("Default: %s\n", cfg.Services.AI.Default)
		}
	}

	// Database URL if enabled
	if cfg.Services.Database.Type != "" {
		fmt.Printf("\nDatabase URL:\n")
		fmt.Printf("postgresql://localcloud:localcloud@localhost:%d/localcloud\n", cfg.Services.Database.Port)
	}

	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
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
