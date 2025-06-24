// internal/cli/start.go
package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/briandowns/spinner"
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
	ValidArgs: []string{"ai", "postgres", "cache", "queue", "minio", "all"}, // Updated: removed redis, added cache/queue
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

	// Determine which services to start
	var servicesToStart []string
	startAll := true

	if len(args) > 0 && args[0] != "all" {
		// Single service specified
		servicesToStart = []string{args[0]}
		startAll = false
	} else if len(startOnly) > 0 {
		// --only flag used
		servicesToStart = startOnly
		startAll = false
	}

	if verbose {
		fmt.Printf("DEBUG: Start all: %v\n", startAll)
		fmt.Printf("DEBUG: Services to start: %v\n", servicesToStart)
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

	// Start services with progress
	progress := make(chan docker.ServiceProgress)
	done := make(chan error)

	// Run startup in goroutine
	go func() {
		if startAll {
			done <- manager.StartServices(progress)
		} else {
			done <- manager.StartSelectedServices(servicesToStart, progress)
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
	}
	fmt.Println()

	// Start connectivity services if enabled
	if cfg.Connectivity.Enabled && !noTunnel {
		if err := startConnectivity(ctx, cfg); err != nil {
			printWarning(fmt.Sprintf("Connectivity setup failed: %v", err))
		}
	}

	// Show connection info if requested
	if showInfo {
		showStartedServicesInfo(cfg, startedServices)
	}

	return nil
}

func startConnectivity(ctx context.Context, cfg *config.Config) error {
	// Create connectivity manager
	connMgr, err := network.NewConnectivityManager(cfg)
	if err != nil {
		return err
	}

	// Register services
	if cfg.Services.AI.Port > 0 {
		connMgr.RegisterService("ai", cfg.Services.AI.Port)
	}
	if cfg.Services.Database.Type != "" && cfg.Services.Database.Port > 0 {
		connMgr.RegisterService("postgres", cfg.Services.Database.Port)
	}
	if cfg.Services.Cache.Type != "" && cfg.Services.Cache.Port > 0 {
		connMgr.RegisterService("redis", cfg.Services.Cache.Port)
	}
	if cfg.Services.Storage.Type != "" {
		connMgr.RegisterService("minio", cfg.Services.Storage.Port)
		connMgr.RegisterService("minio-console", cfg.Services.Storage.Console)
	}

	// Register default web/api ports
	connMgr.RegisterService("web", 3000)
	connMgr.RegisterService("api", 8080)

	// Start connectivity services
	if err := connMgr.Start(ctx); err != nil {
		return err
	}

	// Show tunnel info if established
	info, err := connMgr.GetConnectionInfo()
	if err == nil && len(info.TunnelURLs) > 0 {
		fmt.Println()
		for _, url := range info.TunnelURLs {
			printSuccess(fmt.Sprintf("Public URL: %s", url))
			break
		}
	}

	return nil
}

func showStartedServicesInfo(cfg *config.Config, startedServices map[string]bool) {
	if len(startedServices) > 0 {
		fmt.Println("\nServices:")

		if startedServices["ai"] {
			fmt.Printf("  • AI Models:    http://localhost:%d\n", cfg.Services.AI.Port)
		}

		if startedServices["postgres"] && cfg.Services.Database.Type != "" {
			fmt.Printf("  • PostgreSQL:   localhost:%d\n", cfg.Services.Database.Port)
		}

		if startedServices["cache"] && cfg.Services.Cache.Type != "" {
			// Show cache-specific info
			PrintRedisCacheInfo(cfg.Services.Cache.Port)
		}

		if startedServices["queue"] && cfg.Services.Queue.Type != "" {
			// Show queue-specific info
			PrintRedisQueueInfo(cfg.Services.Queue.Port)
		}

		if startedServices["minio"] && cfg.Services.Storage.Type != "" {
			fmt.Printf("  • MinIO:        http://localhost:%d (console: %d)\n",
				cfg.Services.Storage.Port, cfg.Services.Storage.Console)
		}

		fmt.Println()
	}

	fmt.Println("Run 'localcloud info' for detailed connection information")
	fmt.Println("Run 'localcloud status' to check service health")
	fmt.Println("Run 'localcloud logs' to view service logs")
}
