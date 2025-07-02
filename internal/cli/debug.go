// internal/cli/debug.go
package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/localcloud-sh/localcloud/internal/config"
	"github.com/localcloud-sh/localcloud/internal/diagnostics"
	"github.com/spf13/cobra"
)

var (
	debugService    string
	debugSaveToFile string
	debugVerbose    bool
)

var debugCmd = &cobra.Command{
	Use:   "debug [service]",
	Short: "Collect debug information",
	Long: `Collect debug information for troubleshooting LocalCloud issues.
	
Without arguments, collects system-wide debug information.
With a service name, collects detailed information about that service.`,
	Example: `  lc debug                    # Collect all debug info
  lc debug ai                 # Debug AI service
  lc debug postgres           # Debug PostgreSQL
  lc debug --save debug.json  # Save to file`,
	Args: cobra.MaximumNArgs(1),
	RunE: runDebug,
}

func init() {
	debugCmd.Flags().StringVarP(&debugSaveToFile, "save", "s", "", "Save debug info to file")
	debugCmd.Flags().BoolVarP(&debugVerbose, "verbose", "v", false, "Include verbose debug information")
}

func runDebug(cmd *cobra.Command, args []string) error {
	// Check if project is initialized
	if !IsProjectInitialized() {
		return fmt.Errorf("no LocalCloud project found. Run 'localcloud init' first")
	}

	// Get config
	cfg := config.Get()
	if cfg == nil {
		return fmt.Errorf("failed to load configuration")
	}

	// Get service name if provided
	if len(args) > 0 {
		debugService = args[0]
	}

	printInfo("Collecting debug information...")

	// Create debugger
	debugger, err := diagnostics.NewDebugger(cfg)
	if err != nil {
		return fmt.Errorf("failed to create debugger: %w", err)
	}

	// Collect debug info
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := debugger.CollectDebugInfo(ctx, debugService); err != nil {
		printWarning(fmt.Sprintf("Some debug information could not be collected: %v", err))
	}

	// Get debug info
	debugInfo := debugger.GetDebugInfo()

	// Save to file if requested
	if debugSaveToFile != "" {
		if err := saveDebugInfo(debugInfo, debugSaveToFile); err != nil {
			return fmt.Errorf("failed to save debug info: %w", err)
		}
		printSuccess(fmt.Sprintf("Debug information saved to %s", debugSaveToFile))
		return nil
	}

	// Display debug info
	displayDebugInfo(debugInfo)

	return nil
}

func saveDebugInfo(info *diagnostics.DebugInfo, filename string) error {
	// Ensure directory exists
	dir := filepath.Dir(filename)
	if dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}

	// Marshal to JSON
	data, err := json.MarshalIndent(info, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(filename, data, 0644)
}

func displayDebugInfo(info *diagnostics.DebugInfo) {
	fmt.Println("\n" + infoColor("=== LocalCloud Debug Information ==="))
	fmt.Printf("Generated: %s\n\n", info.Timestamp.Format("2006-01-02 15:04:05"))

	// System Information
	fmt.Println(infoColor("System Information:"))
	fmt.Printf("  OS: %s/%s\n", info.System.OS, info.System.Arch)
	fmt.Printf("  CPUs: %d\n", info.System.CPUs)
	fmt.Printf("  Go Version: %s\n", info.System.GoVersion)
	fmt.Printf("  Memory: %.1fGB total, %.1fGB free (%.1f%% used)\n",
		float64(info.System.Memory.Total)/(1024*1024*1024),
		float64(info.System.Memory.Free)/(1024*1024*1024),
		info.System.Memory.PercentUsed)
	fmt.Printf("  Disk: %.1fGB free (%.1f%% used)\n",
		float64(info.System.Disk.Free)/(1024*1024*1024),
		info.System.Disk.PercentUsed)
	fmt.Println()

	// Docker Information
	fmt.Println(infoColor("Docker Information:"))
	fmt.Printf("  Version: %s (API: %s)\n", info.Docker.Version, info.Docker.APIVersion)
	fmt.Printf("  Containers: %d total, %d running\n", info.Docker.Containers, info.Docker.RunningCount)
	fmt.Printf("  Images: %d\n", info.Docker.Images)
	fmt.Printf("  Storage Driver: %s\n", info.Docker.SystemInfo.Driver)
	fmt.Println()

	// LocalCloud Configuration
	fmt.Println(infoColor("LocalCloud Configuration:"))
	fmt.Printf("  Version: %s\n", info.LocalCloud.Version)
	fmt.Printf("  Project: %s (type: %s)\n", info.LocalCloud.ProjectName, info.LocalCloud.ProjectType)
	fmt.Printf("  Config Path: %s\n", info.LocalCloud.ConfigPath)
	fmt.Println()

	// Service Information
	if len(info.Services) > 0 {
		fmt.Println(infoColor("Services:"))
		for name, svc := range info.Services {
			displayServiceDebugInfo(name, svc)
		}
	} else {
		fmt.Println(warningColor("No services found"))
	}

	// Errors
	if len(info.Errors) > 0 {
		fmt.Println(errorColor("\nErrors Detected:"))
		for _, err := range info.Errors {
			fmt.Printf("  • [%s] %s: %s\n", err.Service, err.Context, err.Error)
		}
	}

	// Verbose information
	if debugVerbose {
		fmt.Println("\n" + infoColor("Environment Variables:"))
		for k, v := range info.System.Environment {
			// Only show LocalCloud related or Docker related
			if containsAny(k, []string{"LOCALCLOUD", "DOCKER", "PATH", "HOME", "USER"}) {
				fmt.Printf("  %s=%s\n", k, v)
			}
		}

		// Show recent logs if available
		if len(info.RecentLogs) > 0 {
			fmt.Println("\n" + infoColor("Recent Logs:"))
			for service, logs := range info.RecentLogs {
				fmt.Printf("\n  %s:\n", service)
				for _, line := range logs {
					fmt.Printf("    %s\n", line)
				}
			}
		}
	}

	fmt.Println("\n" + infoColor("Debug Summary:"))
	if len(info.Errors) == 0 {
		printSuccess("No critical issues detected")
	} else {
		printError(fmt.Sprintf("%d issues detected", len(info.Errors)))
		fmt.Println("\nRun 'lc doctor' for diagnosis and solutions")
	}
}

func displayServiceDebugInfo(name string, svc diagnostics.ServiceDebugInfo) {
	statusColor := successColor
	if svc.State != "running" {
		statusColor = errorColor
	} else if svc.Health == "unhealthy" {
		statusColor = warningColor
	}

	fmt.Printf("\n  %s:\n", name)
	fmt.Printf("    Status: %s\n", statusColor(svc.Status))
	fmt.Printf("    State: %s\n", svc.State)
	if svc.Health != "" {
		fmt.Printf("    Health: %s\n", svc.Health)
	}
	fmt.Printf("    Container ID: %s\n", svc.ContainerID[:12])
	fmt.Printf("    Image: %s\n", svc.Image)
	fmt.Printf("    Created: %s\n", svc.CreatedAt.Format("2006-01-02 15:04:05"))
	if !svc.StartedAt.IsZero() {
		fmt.Printf("    Started: %s (up %s)\n",
			svc.StartedAt.Format("2006-01-02 15:04:05"),
			time.Since(svc.StartedAt).Round(time.Second))
	}
	if svc.RestartCount > 0 {
		fmt.Printf("    Restart Count: %d\n", svc.RestartCount)
	}

	// Ports
	if len(svc.Ports) > 0 {
		fmt.Printf("    Ports:\n")
		for internal, external := range svc.Ports {
			fmt.Printf("      %s → %s\n", internal, external)
		}
	}

	// Resource usage
	if svc.ResourceUsage.CPUPercent > 0 || svc.ResourceUsage.MemoryUsage > 0 {
		fmt.Printf("    Resources:\n")
		fmt.Printf("      CPU: %.1f%%\n", svc.ResourceUsage.CPUPercent)
		if svc.ResourceUsage.MemoryLimit > 0 {
			fmt.Printf("      Memory: %s / %s (%.1f%%)\n",
				FormatBytes(int64(svc.ResourceUsage.MemoryUsage)),
				FormatBytes(int64(svc.ResourceUsage.MemoryLimit)),
				svc.ResourceUsage.MemoryPercent)
		} else {
			fmt.Printf("      Memory: %s\n", FormatBytes(int64(svc.ResourceUsage.MemoryUsage)))
		}
	}

	// Last logs in verbose mode
	if debugVerbose && len(svc.LastLogs) > 0 {
		fmt.Printf("    Recent Logs:\n")
		for _, log := range svc.LastLogs {
			fmt.Printf("      %s\n", log)
		}
	}
}

func containsAny(str string, substrs []string) bool {
	for _, substr := range substrs {
		if strings.Contains(str, substr) {
			return true
		}
	}
	return false
}
