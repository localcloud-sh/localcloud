// internal/cli/status.go
package cli

import (
	"context"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/localcloud/localcloud/internal/config"
	"github.com/localcloud/localcloud/internal/docker"
	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show status of LocalCloud services",
	Long:  `Display the current status and health of all LocalCloud services.`,
	RunE:  runStatus,
}

func runStatus(cmd *cobra.Command, args []string) error {
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
			return fmt.Errorf("Docker is not running. Please start Docker Desktop")
		}
		return fmt.Errorf("failed to create Docker manager: %w", err)
	}
	defer manager.Close()

	// Get service status
	statuses, err := manager.GetServicesStatus()
	if err != nil {
		return fmt.Errorf("failed to get service status: %w", err)
	}

	// Create a tabwriter for aligned output
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)

	// Print header
	fmt.Printf("LocalCloud Status - Project: %s\n", cfg.Project.Name)
	fmt.Println("─────────────────────────────────────────────────────────────")
	fmt.Fprintln(w, "SERVICE\tSTATUS\tHEALTH\tPORT\tCPU\tMEMORY\t")
	fmt.Fprintln(w, "───────\t──────\t──────\t────\t───\t──────\t")

	// Print service status
	if len(statuses) == 0 {
		fmt.Println("No services running")
	} else {
		for _, s := range statuses {
			statusColor := successColor
			if s.Status != "running" {
				statusColor = errorColor
			}

			healthSymbol := "✓"
			if s.Health == "unhealthy" {
				healthSymbol = "✗"
			} else if s.Health == "starting" {
				healthSymbol = "…"
			}

			// Format memory
			memoryStr := "-"
			if s.MemoryUsage > 0 {
				memoryStr = formatBytes(int64(s.MemoryUsage))
			}

			// Format CPU
			cpuStr := "-"
			if s.CPUPercent > 0 {
				cpuStr = fmt.Sprintf("%.1f%%", s.CPUPercent)
			}

			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t\n",
				s.Name,
				statusColor(s.Status),
				healthSymbol,
				s.Port,
				cpuStr,
				memoryStr,
			)
		}
	}

	w.Flush()
	fmt.Println("─────────────────────────────────────────────────────────────")

	// Show overall resource usage
	var totalMemory uint64
	var totalCPU float64
	runningCount := 0

	for _, s := range statuses {
		if s.Status == "running" {
			runningCount++
			totalMemory += s.MemoryUsage
			totalCPU += s.CPUPercent
		}
	}

	if runningCount > 0 {
		fmt.Printf("\nTotal: %d services running | CPU: %.1f%% | Memory: %s\n",
			runningCount, totalCPU, formatBytes(int64(totalMemory)))
	}

	return nil
}

// formatBytes converts bytes to human readable format
