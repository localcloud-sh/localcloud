// internal/cli/start.go
package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/briandowns/spinner"
	"github.com/localcloud/localcloud/internal/config"
	"github.com/localcloud/localcloud/internal/docker"
	"github.com/spf13/cobra"
)

var (
	startService string
	startOnly    []string
)

var startCmd = &cobra.Command{
	Use:       "start [service]",
	Short:     "Start LocalCloud services",
	Long:      `Start all or specific LocalCloud services for the current project.`,
	Args:      cobra.MaximumNArgs(1),
	ValidArgs: []string{"ai", "postgres", "redis", "minio", "all"},
	Example: `  lc start           # Start all services
  lc start ai        # Start only AI service
  lc start postgres  # Start only PostgreSQL
  lc start --only ai,redis  # Start only AI and Redis`,
	RunE: runStart,
}

func init() {
	startCmd.Flags().StringSliceVar(&startOnly, "only", []string{}, "Start only specified services (comma-separated)")
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
	startAll := true // ← BU SATIR EKSİK!

	if len(args) > 0 && args[0] != "all" {
		// Single service specified
		servicesToStart = []string{args[0]}
		startAll = false // ← BU DA!
	} else if len(startOnly) > 0 {
		// --only flag used
		servicesToStart = startOnly
		startAll = false // ← BU DA!
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
			printError("Docker is not running!")
			fmt.Println("\nPlease start Docker Desktop and try again.")
			fmt.Println("\nInstallation instructions:")
			fmt.Println("  macOS:   https://docs.docker.com/desktop/install/mac-install/")
			fmt.Println("  Windows: https://docs.docker.com/desktop/install/windows-install/")
			fmt.Println("  Linux:   https://docs.docker.com/engine/install/")
			return err
		}
		return fmt.Errorf("failed to create Docker manager: %w", err)
	}
	defer manager.Close()

	// Start services with progress
	progress := make(chan docker.ServiceProgress)
	done := make(chan error)

	// Run startup in goroutine
	go func() {
		if startAll { // ← servicesToStart[0] == "all" YERİNE startAll
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

	// Show only started services URLs
	if len(startedServices) > 0 {
		fmt.Println("Services:")

		if startedServices["ai"] {
			fmt.Printf("  • AI Models:    http://localhost:%d\n", cfg.Services.AI.Port)
		}

		if startedServices["postgres"] && cfg.Services.Database.Type != "" {
			fmt.Printf("  • PostgreSQL:   localhost:%d\n", cfg.Services.Database.Port)
		}

		if startedServices["redis"] && cfg.Services.Cache.Type != "" {
			fmt.Printf("  • Redis:        localhost:%d\n", cfg.Services.Cache.Port)
		}

		if startedServices["minio"] && cfg.Services.Storage.Type != "" {
			fmt.Printf("  • MinIO:        http://localhost:%d (console: %d)\n",
				cfg.Services.Storage.Port, cfg.Services.Storage.Console)
		}

		fmt.Println()
	}

	fmt.Println("Run 'localcloud status' to check service health")
	fmt.Println("Run 'localcloud logs' to view service logs")

	return nil
}
