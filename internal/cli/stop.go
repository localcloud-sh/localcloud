// internal/cli/stop.go
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
	stopAll     bool
	stopService string
)

var stopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop LocalCloud services",
	Long:  `Stop all or specific LocalCloud services for the current project.`,
	RunE:  runStop,
}

func init() {
	stopCmd.Flags().BoolVarP(&stopAll, "all", "a", true, "Stop all services")
	stopCmd.Flags().StringVarP(&stopService, "service", "s", "", "Stop specific service (ai, postgres, redis, minio)")
}

func runStop(cmd *cobra.Command, args []string) error {
	// Check if project is initialized
	if !isProjectInitialized() {
		return fmt.Errorf("no LocalCloud project found")
	}

	// Get config
	cfg := config.Get()
	if cfg == nil {
		return fmt.Errorf("failed to load configuration")
	}

	// Create Docker manager
	ctx := context.Background()
	manager, err := docker.NewManager(ctx, cfg)
	if err != nil {
		if strings.Contains(err.Error(), "Docker daemon not running") {
			return fmt.Errorf("Docker is not running")
		}
		return fmt.Errorf("failed to create Docker manager: %w", err)
	}
	defer manager.Close()

	// Check what services are running
	statuses, err := manager.GetServicesStatus()
	if err != nil {
		return fmt.Errorf("failed to get service status: %w", err)
	}

	runningCount := 0
	for _, s := range statuses {
		if s.Status == "running" {
			runningCount++
		}
	}

	if runningCount == 0 {
		printInfo("No services are running")
		return nil
	}

	// Stop services with progress
	progress := make(chan docker.ServiceProgress)
	done := make(chan error)

	// Run stop in goroutine
	go func() {
		if stopService != "" {
			// Stop specific service - need to implement in docker manager
			// For now, stop all
			done <- manager.StopServices(progress)
		} else {
			done <- manager.StopServices(progress)
		}
	}()

	// Handle progress updates
	s := spinner.New(spinner.CharSets[14], 100)
	s.Start()

	stoppedCount := 0
	for {
		select {
		case p, ok := <-progress:
			if !ok {
				// Channel closed, stop complete
				s.Stop()
				goto finished
			}

			switch p.Status {
			case "stopping":
				s.Suffix = fmt.Sprintf(" Stopping %s...", p.Service)
			case "stopped":
				s.Stop()
				printSuccess(fmt.Sprintf("%s stopped", p.Service))
				stoppedCount++
				s.Start()
			case "failed":
				s.Stop()
				printWarning(fmt.Sprintf("Failed to stop %s: %s", p.Service, p.Error))
				s.Start()
			}
		}
	}

finished:
	s.Stop()

	// Wait for completion
	if err := <-done; err != nil {
		// Even if some services failed to stop, show what we did stop
		if stoppedCount > 0 {
			fmt.Println()
			printSuccess(fmt.Sprintf("Stopped %d services", stoppedCount))
		}
		return err
	}

	fmt.Println()
	printSuccess("All services stopped successfully!")

	return nil
}
