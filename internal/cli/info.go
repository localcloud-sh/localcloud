// internal/cli/info.go
package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/fatih/color"
	"github.com/localcloud-sh/localcloud/internal/config"
	"github.com/localcloud-sh/localcloud/internal/docker"
	"github.com/localcloud-sh/localcloud/internal/network"
	"github.com/spf13/cobra"
)

var (
	infoJSON            bool
	infoMobileConfig    bool
	infoCopyToClipboard bool
)

var infoCmd = &cobra.Command{
	Use:   "info",
	Short: "Display project connection information",
	Long: `Display comprehensive connection information for your LocalCloud project,
including local URLs, network URLs, and tunnel access for mobile/web development.`,
	Example: `  lc info                    # Show all connection info
  lc info --json             # Output as JSON
  lc info --mobile-config    # Show mobile app configuration
  lc info --copy             # Copy primary URL to clipboard`,
	RunE: runInfo,
}

func init() {
	infoCmd.Flags().BoolVar(&infoJSON, "json", false, "Output in JSON format")
	infoCmd.Flags().BoolVar(&infoMobileConfig, "mobile-config", false, "Show mobile app configuration")
	infoCmd.Flags().BoolVar(&infoCopyToClipboard, "copy", false, "Copy primary URL to clipboard")
}

func runInfo(cmd *cobra.Command, args []string) error {
	// Check if project is initialized
	if !IsProjectInitialized() {
		return fmt.Errorf("no LocalCloud project found. Run 'localcloud init' first")
	}

	// Get config
	cfg := config.Get()
	if cfg == nil {
		return fmt.Errorf("failed to load configuration")
	}

	// Get Docker manager to check service status
	ctx := cmd.Context()
	manager, err := docker.NewManager(ctx, cfg)
	if err != nil {
		printWarning("Docker is not running. Connection info may be incomplete.")
	} else {
		defer manager.Close()
	}

	// Create connectivity manager
	connMgr, err := network.NewConnectivityManager(cfg)
	if err != nil {
		return fmt.Errorf("failed to create connectivity manager: %w", err)
	}

	// Register services based on config
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

	// Also check for common web ports
	connMgr.RegisterService("web", 3000)
	connMgr.RegisterService("api", 8080)

	// Handle mobile config request
	if infoMobileConfig {
		return showMobileConfig(connMgr)
	}

	// Get connection info
	info, err := connMgr.GetConnectionInfo()
	if err != nil {
		return fmt.Errorf("failed to get connection info: %w", err)
	}

	// Handle JSON output
	if infoJSON {
		data, err := json.MarshalIndent(info, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal JSON: %w", err)
		}
		fmt.Println(string(data))
		return nil
	}

	// Load saved tunnel info if available
	var savedTunnel *network.TunnelInfo
	if tunnelInfo, err := network.LoadTunnelInfo(cfg.Project.Name); err == nil {
		savedTunnel = tunnelInfo
	}

	// Display formatted output
	displayConnectionInfo(cfg, info, savedTunnel, manager)

	// Handle copy to clipboard
	if infoCopyToClipboard {
		// TODO: Implement clipboard copy
		printInfo("Clipboard copy not yet implemented")
	}

	return nil
}

func displayConnectionInfo(cfg *config.Config, info *network.ConnectionInfo, savedTunnel *network.TunnelInfo, manager *docker.Manager) {
	bold := color.New(color.Bold).SprintFunc()

	// Header
	fmt.Printf("%s: %s\n", bold("LocalCloud Project"), cfg.Project.Name)
	fmt.Printf("%s: %s\n", bold("Type"), cfg.Project.Type)

	// Service status if Docker is running
	if manager != nil {
		statuses, err := manager.GetServicesStatus()
		if err == nil && len(statuses) > 0 {
			fmt.Printf("%s: Running (%d services)\n\n", bold("Status"), len(statuses))
		} else {
			fmt.Printf("%s: Ready\n\n", bold("Status"))
		}
	} else {
		fmt.Printf("%s: Docker not running\n\n", bold("Status"))
	}

	// Local Access
	fmt.Println(bold("Local Access:"))
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	for service, url := range info.LocalURLs {
		fmt.Fprintf(w, "  • %s:\t%s\n", formatServiceName(service), url)
	}
	w.Flush()
	fmt.Println()

	// Network Access
	if len(info.NetworkURLs) > 0 {
		fmt.Println(bold("Network Access:"))
		hasNetworkURLs := false
		for service, urls := range info.NetworkURLs {
			// Show first non-localhost URL
			for _, url := range urls {
				if url != info.LocalURLs[service] {
					fmt.Printf("  • %s: %s\n", formatServiceName(service), url)
					hasNetworkURLs = true
					break
				}
			}
		}
		if hasNetworkURLs {
			fmt.Println()
		}
	}

	// mDNS Access
	if cfg.Connectivity.Enabled {
		fmt.Println(bold("mDNS Access (Same Network):"))
		for service, url := range info.MDNSURLs {
			fmt.Printf("  • %s: %s\n", formatServiceName(service), url)
		}
		fmt.Println()
	}

	// Tunnel Access
	if len(info.TunnelURLs) > 0 || savedTunnel != nil {
		fmt.Println(bold("Public Access (Tunnel):"))

		if len(info.TunnelURLs) > 0 {
			// Active tunnel
			for service, url := range info.TunnelURLs {
				if service == "web" || service == "api" {
					fmt.Printf("  • %s: %s %s\n", formatServiceName(service), url, successColor("(active)"))
					break // Show only primary URL
				}
			}
		} else if savedTunnel != nil {
			// Saved but not active
			fmt.Printf("  • URL: %s %s\n", savedTunnel.URL, warningColor("(not active)"))
			fmt.Printf("  • Provider: %s\n", savedTunnel.Provider)
			fmt.Printf("  • Created: %s\n", savedTunnel.CreatedAt.Format("2006-01-02 15:04"))
		}
		fmt.Println()
	}

	// Connection strings for databases
	if cfg.Services.Database.Type != "" {
		fmt.Println(bold("Database Connection:"))
		fmt.Printf("  • PostgreSQL: postgresql://localcloud:password@localhost:%d/localcloud\n",
			cfg.Services.Database.Port)
		fmt.Println()
	}

	// Instructions
	fmt.Println(bold("Quick Tips:"))
	fmt.Println("  • Use 'lc info --mobile-config' to get mobile app configuration")
	fmt.Println("  • Use 'lc info --json' for JSON output")
	fmt.Println("  • Run 'lc tunnel setup' to create a persistent public URL")
}

func showMobileConfig(connMgr *network.ConnectivityManager) error {
	config, err := connMgr.GetMobileConfig()
	if err != nil {
		return fmt.Errorf("failed to get mobile config: %w", err)
	}

	// If JSON requested, output full config
	if infoJSON {
		data, err := config.ToJSON()
		if err != nil {
			return err
		}
		fmt.Println(data)
		return nil
	}

	// Otherwise show environment format
	bold := color.New(color.Bold).SprintFunc()
	fmt.Println(bold("Mobile App Configuration"))
	fmt.Println(strings.Repeat("-", 40))
	fmt.Println()

	// Show primary endpoints
	fmt.Println(bold("Primary Endpoints:"))
	for service, endpoint := range config.Endpoints {
		if service == "api" || service == "web" {
			fmt.Printf("  • %s: %s\n", strings.ToUpper(service), endpoint.Primary)
			if endpoint.WebSocket != "" {
				fmt.Printf("  • %s_WS: %s\n", strings.ToUpper(service), endpoint.WebSocket)
			}
		}
	}
	fmt.Println()

	// Show env format
	fmt.Println(config.EnvFormat)

	// Additional instructions
	if config.Endpoints["api"].Public != "" {
		fmt.Println(bold("Note:") + " Using public tunnel URL - accessible from anywhere")
	} else {
		fmt.Println(bold("Note:") + " Using local network URL - device must be on same network")
	}

	return nil
}

func formatServiceName(service string) string {
	switch service {
	case "ai":
		return "AI Models"
	case "postgres":
		return "PostgreSQL"
	case "redis":
		return "Redis"
	case "minio":
		return "MinIO"
	case "minio-console":
		return "MinIO Console"
	case "web":
		return "Web UI"
	case "api":
		return "API"
	default:
		return service
	}
}
