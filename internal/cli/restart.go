// internal/cli/restart.go
package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/briandowns/spinner"
	"github.com/localcloud-sh/localcloud/internal/components"
	"github.com/localcloud-sh/localcloud/internal/config"
	"github.com/localcloud-sh/localcloud/internal/docker"
	"github.com/spf13/cobra"
)

var restartCmd = &cobra.Command{
	Use:   "restart",
	Short: "Restart LocalCloud services",
	Long: `Stop all running services and start them again based on current configuration.
	
This is useful when you've changed your project configuration and need to apply 
the changes. It will:
  1. Stop all currently running services
  2. Start services for currently enabled components
  3. Show connection information

This ensures your running services match your current configuration.`,
	Example: `  lc restart              # Restart all services
  lc restart --info       # Show connection info after restart`,
	RunE: runRestart,
}

var (
	restartShowInfo bool
)

func init() {
	restartCmd.Flags().BoolVar(&restartShowInfo, "info", true, "Show connection info after restart")
}

func runRestart(cmd *cobra.Command, args []string) error {
	if verbose {
		fmt.Println("DEBUG: runRestart called")
	}

	// Check if project is initialized
	if !IsProjectInitialized() {
		printError("No LocalCloud project found")
		fmt.Println("\nTo create a new project:")
		fmt.Printf("  %s\n", infoColor("lc setup my-project   # Create and configure project"))
		fmt.Printf("  %s\n", infoColor("lc setup              # Setup in current directory"))
		return fmt.Errorf("project not initialized")
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
		printWarning("No components configured")
		fmt.Println("\nThis project has no components selected.")
		fmt.Println("\nTo configure components:")
		fmt.Printf("  %s\n", infoColor("lc setup              # Interactive configuration"))
		return nil
	}

	fmt.Println("🔄 Restarting LocalCloud services...")
	fmt.Printf("Target components: %s\n", strings.Join(enabledComponents, ", "))
	fmt.Println()

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

	// Phase 1: Stop all services
	fmt.Println("📱 Phase 1: Stopping all services...")

	// Check what services are running
	statuses, err := manager.GetServicesStatus()
	if err != nil {
		printWarning("Failed to get service status, continuing with restart...")
	} else {
		runningCount := 0
		for _, s := range statuses {
			if s.Status == "running" {
				runningCount++
			}
		}

		if runningCount > 0 {
			fmt.Printf("Found %d running services\n", runningCount)

			// Stop services
			stopProgress := make(chan docker.ServiceProgress)
			stopDone := make(chan error)

			go func() {
				stopDone <- manager.StopServices(stopProgress)
			}()

			// Handle stop progress
			var stopSpin *spinner.Spinner
			if !verbose {
				stopSpin = spinner.New(spinner.CharSets[14], 100)
				stopSpin.Start()
			}

			stoppedCount := 0
			for {
				select {
				case p, ok := <-stopProgress:
					if !ok {
						if stopSpin != nil {
							stopSpin.Stop()
						}
						goto stopFinished
					}

					switch p.Status {
					case "stopping":
						if verbose {
							fmt.Printf("  Stopping %s...\n", p.Service)
						} else if stopSpin != nil {
							stopSpin.Suffix = fmt.Sprintf(" Stopping %s...", p.Service)
						}
					case "stopped":
						if stopSpin != nil {
							stopSpin.Stop()
						}
						fmt.Printf("  %s %s stopped\n", successColor("✓"), p.Service)
						stoppedCount++
						if stopSpin != nil && !verbose {
							stopSpin.Start()
						}
					case "failed":
						if stopSpin != nil {
							stopSpin.Stop()
						}
						printWarning(fmt.Sprintf("Failed to stop %s: %s", p.Service, p.Error))
						if stopSpin != nil && !verbose {
							stopSpin.Start()
						}
					}
				}
			}

		stopFinished:
			// Wait for stop completion
			if err := <-stopDone; err != nil {
				printWarning("Some services failed to stop, continuing with start...")
			} else {
				fmt.Printf("  %s Stopped %d services\n", successColor("✓"), stoppedCount)
			}
		} else {
			fmt.Println("  No services currently running")
		}
	}

	fmt.Println()

	// Phase 2: Start services for enabled components
	fmt.Println("🚀 Phase 2: Starting configured services...")

	// Convert components to services and start them
	servicesToStart := components.ComponentsToServices(enabledComponents)

	if verbose {
		fmt.Printf("DEBUG: Services to start: %v\n", servicesToStart)
	}

	startProgress := make(chan docker.ServiceProgress)
	startDone := make(chan error)

	// Run startup in goroutine
	go func() {
		startDone <- manager.StartSelectedServices(servicesToStart, startProgress)
	}()

	// Handle start progress updates
	var startSpin *spinner.Spinner
	if !verbose {
		startSpin = spinner.New(spinner.CharSets[14], 100)
		startSpin.Start()
	}

	hasErrors := false
	startedServices := make(map[string]bool)

	for {
		select {
		case p, ok := <-startProgress:
			if !ok {
				// Channel closed, services started
				if startSpin != nil {
					startSpin.Stop()
				}
				goto startFinished
			}

			switch p.Status {
			case "starting":
				if verbose {
					fmt.Printf("  Starting %s...\n", p.Service)
				} else if startSpin != nil {
					startSpin.Suffix = fmt.Sprintf(" Starting %s...", p.Service)
				}
			case "started":
				if startSpin != nil {
					startSpin.Stop()
				}
				fmt.Printf("  %s %s started\n", successColor("✓"), p.Service)
				startedServices[p.Service] = true
				if startSpin != nil && !verbose {
					startSpin.Start()
				}
			case "failed":
				hasErrors = true
				if startSpin != nil {
					startSpin.Stop()
				}
				errorMsg := p.Error
				if errorMsg == "" {
					errorMsg = "unknown error"
				}
				printError(fmt.Sprintf("Failed to start %s: %s", p.Service, errorMsg))
				if startSpin != nil && !verbose {
					startSpin.Start()
				}
			}
		}
	}

startFinished:
	// Wait for start completion
	if err := <-startDone; err != nil {
		if verbose {
			fmt.Printf("DEBUG: StartServices returned error: %v\n", err)
		}
		printWarning("Some services failed to start")
		return err
	}

	// Print completion message
	fmt.Println()
	if hasErrors {
		printWarning("Restart completed with some errors!")
	} else {
		printSuccess("🎉 Restart completed successfully!")
		fmt.Printf("All services are now running with your current configuration\n")
	}

	// Show connection info
	if restartShowInfo && !hasErrors {
		fmt.Println()
		showConnectionInfo(cfg)
	}

	return nil
}
