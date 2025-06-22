// internal/cli/status.go - Enhanced version with resource monitoring
package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/localcloud/localcloud/internal/config"
	"github.com/localcloud/localcloud/internal/docker"
	"github.com/localcloud/localcloud/internal/monitoring"
	"github.com/spf13/cobra"
)

var (
	showDetails bool
	continuous  bool
	interval    int
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show status of LocalCloud services",
	Long: `Display the current status, health, and resource usage of all LocalCloud services.
	
Shows real-time metrics including CPU usage, memory consumption, network I/O, and
service-specific performance indicators.`,
	RunE: runStatus,
	Example: `  localcloud status              # Show current status
  localcloud status --details    # Show detailed metrics
  localcloud status --watch      # Continuous monitoring
  localcloud status --interval 2 # Update every 2 seconds`,
}

func init() {
	statusCmd.Flags().BoolVarP(&showDetails, "details", "d", false, "Show detailed metrics")
	statusCmd.Flags().BoolVarP(&continuous, "watch", "w", false, "Continuously monitor status")
	statusCmd.Flags().IntVarP(&interval, "interval", "i", 5, "Update interval in seconds (with --watch)")
}

func runStatus(cmd *cobra.Command, args []string) error {
	// Check if project is initialized
	if !IsProjectInitialized() {
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

	// Create metrics collector
	collector, err := monitoring.NewCollector(cfg)
	if err != nil {
		return fmt.Errorf("failed to create metrics collector: %w", err)
	}

	// Start metrics collection
	if err := collector.Start(ctx); err != nil {
		return fmt.Errorf("failed to start metrics collection: %w", err)
	}
	defer collector.Stop()

	// Wait a moment for initial metrics collection
	time.Sleep(1 * time.Second)

	// Handle continuous monitoring
	if continuous {
		return runContinuousStatus(ctx, cfg, manager, collector)
	}

	// Single status display
	return displayStatus(cfg, manager, collector)
}

func displayStatus(cfg *config.Config, manager *docker.Manager, collector *monitoring.Collector) error {
	// Clear screen for clean display
	if continuous {
		fmt.Print("\033[H\033[2J")
	}

	// Get service status
	statuses, err := manager.GetServicesStatus()
	if err != nil {
		return fmt.Errorf("failed to get service status: %w", err)
	}

	// Get metrics
	metrics := collector.GetMetrics()

	// Display header
	fmt.Printf("LocalCloud Status - Project: %s\n", successColor(cfg.Project.Name))
	fmt.Printf("Time: %s\n", time.Now().Format("2006-01-02 15:04:05"))
	fmt.Println(strings.Repeat("═", 100))

	if len(statuses) == 0 {
		fmt.Println("No services running")
		return nil
	}

	// Create tabwriter for aligned output
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)

	// Print headers
	fmt.Fprintln(w, "SERVICE\tSTATUS\tHEALTH\tCPU\tMEMORY\tNETWORK I/O\tDISK I/O\t")
	fmt.Fprintln(w, "───────\t──────\t──────\t───\t──────\t───────────\t────────\t")

	// Display service information
	var totalCPU float64
	var totalMemory uint64
	var totalRxBytes, totalTxBytes uint64

	for _, status := range statuses {
		// Get metrics for this service
		serviceMetrics, hasMetrics := metrics[status.Name]

		// Format status with color
		statusText := formatStatus(status.Status)

		// Format health
		healthText := formatHealth(status.Health)

		// Format resource usage
		cpuText := "-"
		memoryText := "-"
		networkText := "-"
		diskText := "-"

		if hasMetrics {
			cpuText = fmt.Sprintf("%.1f%%", serviceMetrics.CPU.UsagePercent)
			totalCPU += serviceMetrics.CPU.UsagePercent

			memoryText = fmt.Sprintf("%s/%s (%.1f%%)",
				FormatBytes(int64(serviceMetrics.Memory.WorkingSet)),
				FormatBytes(int64(serviceMetrics.Memory.Limit)),
				serviceMetrics.Memory.Percent)
			totalMemory += serviceMetrics.Memory.WorkingSet

			networkText = fmt.Sprintf("↓%s ↑%s",
				FormatBytes(int64(serviceMetrics.Network.RxBytes)),
				FormatBytes(int64(serviceMetrics.Network.TxBytes)))
			totalRxBytes += serviceMetrics.Network.RxBytes
			totalTxBytes += serviceMetrics.Network.TxBytes

			diskText = fmt.Sprintf("R:%s W:%s",
				FormatBytes(int64(serviceMetrics.Disk.ReadBytes)),
				FormatBytes(int64(serviceMetrics.Disk.WriteBytes)))
		}

		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\t\n",
			status.Name,
			statusText,
			healthText,
			cpuText,
			memoryText,
			networkText,
			diskText,
		)
	}

	w.Flush()
	fmt.Println(strings.Repeat("─", 100))

	// Show totals
	runningCount := 0
	for _, s := range statuses {
		if s.Status == "running" {
			runningCount++
		}
	}

	fmt.Printf("Summary: %d services running | CPU: %.1f%% | Memory: %s | Network: ↓%s ↑%s\n",
		runningCount, totalCPU, FormatBytes(int64(totalMemory)),
		FormatBytes(int64(totalRxBytes)), FormatBytes(int64(totalTxBytes)))

	// Show detailed metrics if requested
	if showDetails {
		fmt.Println("\n" + strings.Repeat("═", 100))
		displayDetailedMetrics(metrics)
	}

	return nil
}

func displayDetailedMetrics(metrics map[string]*monitoring.Metrics) {
	fmt.Println("Detailed Performance Metrics:")
	fmt.Println(strings.Repeat("─", 100))

	for service, m := range metrics {
		fmt.Printf("\n%s Service:\n", strings.Title(service))

		// CPU Details
		fmt.Printf("  CPU: %.2f%% (Online CPUs: %d)\n",
			m.CPU.UsagePercent, m.CPU.OnlineCPUs)

		// Memory Details
		fmt.Printf("  Memory:\n")
		fmt.Printf("    Working Set: %s\n", FormatBytes(int64(m.Memory.WorkingSet)))
		fmt.Printf("    Cache: %s\n", FormatBytes(int64(m.Memory.Cache)))
		fmt.Printf("    Total Usage: %s / %s (%.2f%%)\n",
			FormatBytes(int64(m.Memory.Usage)),
			FormatBytes(int64(m.Memory.Limit)),
			m.Memory.Percent)

		// Network Details
		fmt.Printf("  Network:\n")
		fmt.Printf("    Received: %s (%d packets, %d dropped)\n",
			FormatBytes(int64(m.Network.RxBytes)),
			m.Network.RxPackets,
			m.Network.RxDropped)
		fmt.Printf("    Sent: %s (%d packets, %d dropped)\n",
			FormatBytes(int64(m.Network.TxBytes)),
			m.Network.TxPackets,
			m.Network.TxDropped)

		// Disk Details
		fmt.Printf("  Disk I/O:\n")
		fmt.Printf("    Read: %s (%d ops)\n",
			FormatBytes(int64(m.Disk.ReadBytes)),
			m.Disk.ReadOps)
		fmt.Printf("    Write: %s (%d ops)\n",
			FormatBytes(int64(m.Disk.WriteBytes)),
			m.Disk.WriteOps)

		// Service-specific performance metrics
		if len(m.Performance) > 0 {
			fmt.Printf("  Performance:\n")
			for key, value := range m.Performance {
				fmt.Printf("    %s: %v\n", formatMetricName(key), value)
			}
		}
	}
}

func runContinuousStatus(ctx context.Context, cfg *config.Config, manager *docker.Manager, collector *monitoring.Collector) error {
	ticker := time.NewTicker(time.Duration(interval) * time.Second)
	defer ticker.Stop()

	// Initial display
	if err := displayStatus(cfg, manager, collector); err != nil {
		return err
	}

	// Handle interrupt
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt)

	fmt.Println("\nPress Ctrl+C to stop monitoring...")

	for {
		select {
		case <-ticker.C:
			if err := displayStatus(cfg, manager, collector); err != nil {
				return err
			}
		case <-sigChan:
			fmt.Println("\nStopping monitor...")
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// Helper functions

func formatStatus(status string) string {
	switch status {
	case "running":
		return successColor("● running")
	case "starting":
		return warningColor("◐ starting")
	case "stopping":
		return warningColor("◑ stopping")
	case "stopped", "exited":
		return errorColor("○ stopped")
	default:
		return status
	}
}

func formatHealth(health string) string {
	switch health {
	case "healthy", "":
		return successColor("✓ healthy")
	case "unhealthy":
		return errorColor("✗ unhealthy")
	case "starting":
		return warningColor("… starting")
	default:
		return health
	}
}

func formatMetricName(name string) string {
	// Convert snake_case to Title Case
	parts := strings.Split(name, "_")
	for i, part := range parts {
		parts[i] = strings.Title(part)
	}
	return strings.Join(parts, " ")
}
