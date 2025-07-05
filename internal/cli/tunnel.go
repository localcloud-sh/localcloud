// internal/cli/tunnel.go
package cli

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/briandowns/spinner"
	"github.com/localcloud-sh/localcloud/internal/config"
	"github.com/localcloud-sh/localcloud/internal/network"
	"github.com/spf13/cobra"
)

var (
	tunnelProvider string
	tunnelPersist  bool
	tunnelDomain   string

	// Component flags
	tunnelAll     bool
	tunnelAPI     bool
	tunnelDB      bool
	tunnelMongo   bool
	tunnelStorage bool
	tunnelAI      bool
	tunnelCache   bool
	tunnelQueue   bool

	// Multi-service options
	tunnelComponents []string
	tunnelPrefix     string

	// Flexible service options
	tunnelServices []string // Format: "name:port" or "name:url"
	tunnelName     string   // Custom name for single service
	autoDetect     bool     // Auto-detect services on common ports
)

var tunnelCmd = &cobra.Command{
	Use:   "tunnel",
	Short: "Manage tunnel connections",
	Long:  `Manage tunnel connections for public access to your LocalCloud services.`,
}

var tunnelSetupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Setup a persistent tunnel",
	Long: `Setup a persistent tunnel for your project. This creates a stable URL that 
remains the same across restarts.`,
	Example: `  lc tunnel setup                    # Interactive setup
  lc tunnel setup --provider cloudflare
  lc tunnel setup --domain my-app.example.com`,
	RunE: runTunnelSetup,
}

var tunnelStartCmd = &cobra.Command{
	Use:   "start",
	Short: "Start tunnel connection",
	Long: `Start a tunnel connection using the configured provider.

This command supports tunneling individual components or multiple services simultaneously.
All tunneled services will be accessible via HTTPS with automatic SSL certificates.`,
	Example: `  # Tunnel LocalCloud components
  lc tunnel start --api
  lc tunnel start --db --storage
  lc tunnel start --all

  # Tunnel any port or service (flexible mode)
  lc tunnel start --port 8080
  lc tunnel start --port 3000 --name my-api
  lc tunnel start --url http://localhost:8080
  lc tunnel start --url http://192.168.1.100:5000

  # Tunnel multiple arbitrary services
  lc tunnel start --service api:3000 --service docs:8080
  lc tunnel start --service frontend:3000 --service backend:8000

  # Mix LocalCloud and custom services
  lc tunnel start --storage --service custom-api:8080
  lc tunnel start --api --service docs:4000 --service admin:9000

  # Auto-detect services on common ports
  lc tunnel start --auto-detect

  # Custom subdomain prefix
  lc tunnel start --api --prefix myapp`,
	RunE: runTunnelStart,
}

var tunnelStopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop tunnel connection",
	RunE:  runTunnelStop,
}

var tunnelStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show tunnel status",
	RunE:  runTunnelStatus,
}

func init() {
	// Add subcommands
	tunnelCmd.AddCommand(tunnelSetupCmd)
	tunnelCmd.AddCommand(tunnelStartCmd)
	tunnelCmd.AddCommand(tunnelStopCmd)
	tunnelCmd.AddCommand(tunnelStatusCmd)

	// Setup command flags
	tunnelSetupCmd.Flags().StringVar(&tunnelProvider, "provider", "", "Tunnel provider (cloudflare, ngrok)")
	tunnelSetupCmd.Flags().BoolVar(&tunnelPersist, "persist", true, "Create persistent tunnel")
	tunnelSetupCmd.Flags().StringVar(&tunnelDomain, "domain", "", "Custom domain for tunnel")

	// Start command flags - NOT using -p shorthand to avoid conflict
	tunnelStartCmd.Flags().Int("port", 3000, "Local port to tunnel")
	tunnelStartCmd.Flags().String("url", "", "Custom URL to tunnel (e.g., http://localhost:3000, http://192.168.1.100:8080)")

	// Component flags
	tunnelStartCmd.Flags().BoolVar(&tunnelAll, "all", false, "Tunnel all configured services")
	tunnelStartCmd.Flags().BoolVar(&tunnelAPI, "api", false, "Tunnel API service")
	tunnelStartCmd.Flags().BoolVar(&tunnelDB, "db", false, "Tunnel PostgreSQL database (admin interface)")
	tunnelStartCmd.Flags().BoolVar(&tunnelMongo, "mongo", false, "Tunnel MongoDB (admin interface)")
	tunnelStartCmd.Flags().BoolVar(&tunnelStorage, "storage", false, "Tunnel MinIO storage (console)")
	tunnelStartCmd.Flags().BoolVar(&tunnelAI, "ai", false, "Tunnel AI/Ollama service")
	tunnelStartCmd.Flags().BoolVar(&tunnelCache, "cache", false, "Tunnel Redis cache")
	tunnelStartCmd.Flags().BoolVar(&tunnelQueue, "queue", false, "Tunnel Redis queue")

	// Multi-service options
	tunnelStartCmd.Flags().StringSliceVar(&tunnelComponents, "components", []string{}, "Comma-separated list of components to tunnel")
	tunnelStartCmd.Flags().StringVar(&tunnelPrefix, "prefix", "", "Custom subdomain prefix for tunnel URLs")

	// Flexible service options
	tunnelStartCmd.Flags().StringSliceVar(&tunnelServices, "service", []string{}, "Tunnel arbitrary services (format: name:port or name:url)")
	tunnelStartCmd.Flags().StringVar(&tunnelName, "name", "", "Custom name for single service tunnel")
	tunnelStartCmd.Flags().BoolVar(&autoDetect, "auto-detect", false, "Auto-detect and tunnel services on common ports")
}

func runTunnelSetup(cmd *cobra.Command, args []string) error {
	// Check if project is initialized
	if !IsProjectInitialized() {
		return fmt.Errorf("no LocalCloud project found. Run 'lc setup' first")
	}

	cfg := config.Get()
	if cfg == nil {
		return fmt.Errorf("failed to load configuration")
	}

	// Interactive setup if no provider specified
	if tunnelProvider == "" {
		fmt.Println("Which tunnel provider would you like to use?")
		fmt.Println("1. Cloudflare (recommended - free, persistent URLs)")
		fmt.Println("2. Ngrok (requires account for persistent URLs)")
		fmt.Print("\nSelect provider [1-2]: ")

		var choice string
		fmt.Scanln(&choice)

		switch choice {
		case "1":
			tunnelProvider = "cloudflare"
		case "2":
			tunnelProvider = "ngrok"
		default:
			return fmt.Errorf("invalid choice")
		}
	}

	// Update config
	v := config.GetViper()
	v.Set("connectivity.tunnel.provider", tunnelProvider)
	v.Set("connectivity.tunnel.persist", tunnelPersist)

	if tunnelDomain != "" {
		v.Set("connectivity.tunnel.domain", tunnelDomain)
	}

	// Provider-specific setup
	switch tunnelProvider {
	case "cloudflare":
		if err := setupCloudflare(cfg); err != nil {
			return err
		}
	case "ngrok":
		if err := setupNgrok(cfg); err != nil {
			return err
		}
	}

	// Save config
	configPath := ".localcloud/config.yaml"
	if err := v.WriteConfigAs(configPath); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	printSuccess("Tunnel configuration saved!")
	fmt.Println("\nNext steps:")
	fmt.Println("  1. Run 'lc start' to start your services")
	fmt.Println("  2. Run 'lc tunnel start' to establish tunnel connection")
	fmt.Println("  3. Run 'lc info' to see your public URL")

	return nil
}

func setupCloudflare(cfg *config.Config) error {
	// Check if cloudflared is installed
	if _, err := exec.LookPath("cloudflared"); err != nil {
		printError("cloudflared not found!")
		fmt.Println("\nInstall cloudflared:")
		fmt.Println("  macOS:   brew install cloudflared")
		fmt.Println("  Linux:   https://developers.cloudflare.com/cloudflare-one/connections/connect-apps/install-and-setup/installation")
		fmt.Println("  Windows: https://developers.cloudflare.com/cloudflare-one/connections/connect-apps/install-and-setup/installation")
		return fmt.Errorf("cloudflared not installed")
	}

	if tunnelPersist && tunnelDomain != "" {
		fmt.Println("\nTo use a custom domain with Cloudflare:")
		fmt.Println("  1. Add your domain to Cloudflare")
		fmt.Println("  2. Run: cloudflared tunnel login")
		fmt.Println("  3. Run: cloudflared tunnel create", cfg.Project.Name)
		fmt.Println("  4. Run: cloudflared tunnel route dns", cfg.Project.Name, tunnelDomain)
		fmt.Println("\nThen run 'lc tunnel setup' again.")
		return fmt.Errorf("manual Cloudflare setup required")
	}

	printSuccess("Cloudflare tunnel ready to use!")
	return nil
}

func setupNgrok(cfg *config.Config) error {
	// Check for auth token
	authToken := os.Getenv("NGROK_AUTH_TOKEN")
	if authToken == "" {
		printError("NGROK_AUTH_TOKEN not set!")
		fmt.Println("\nTo use Ngrok:")
		fmt.Println("  1. Sign up at https://ngrok.com")
		fmt.Println("  2. Get your auth token from the dashboard")
		fmt.Println("  3. Set: export NGROK_AUTH_TOKEN=your_token")
		return fmt.Errorf("ngrok auth token required")
	}

	if tunnelPersist && tunnelDomain == "" {
		printWarning("Note: Custom domains require a paid Ngrok plan")
	}

	printSuccess("Ngrok tunnel ready to use!")
	return nil
}

func runTunnelStart(cmd *cobra.Command, args []string) error {
	var cfg *config.Config

	// Check if project is initialized
	if !IsProjectInitialized() {
		// For tunnel testing, we don't need a full project
		printInfo("No LocalCloud project found. Running in standalone mode...")

		// Create minimal config
		cfg = &config.Config{
			Project: config.ProjectConfig{
				Name: "tunnel-test",
			},
			Connectivity: config.ConnectivityConfig{
				Enabled: true,
				Tunnel: config.TunnelConfig{
					Provider: tunnelProvider,
				},
			},
		}
	} else {
		cfg = config.Get()
		if cfg == nil {
			return fmt.Errorf("failed to load configuration")
		}
	}

	// Determine which services to tunnel
	services := determineServicesToTunnel(cfg)

	if len(services) == 0 {
		return fmt.Errorf("no services specified to tunnel. Use flags like --api, --db, --storage, or --all")
	}

	// Check for legacy single port mode
	port, _ := cmd.Flags().GetInt("port")
	customURL, _ := cmd.Flags().GetString("url")

	if customURL != "" || (!tunnelAll && !tunnelAPI && !tunnelDB && !tunnelMongo && !tunnelStorage && !tunnelAI && !tunnelCache && !tunnelQueue && len(tunnelComponents) == 0 && len(tunnelServices) == 0 && !autoDetect) {
		// Legacy single-service mode - handle --name flag for custom service name
		serviceName := "tunnel"
		if tunnelName != "" {
			serviceName = tunnelName
		}
		return runLegacyTunnelWithName(cfg, port, customURL, serviceName)
	}

	// Multi-service mode with reverse proxy
	return runMultiServiceTunnel(cfg, services)
}

func runTunnelStop(cmd *cobra.Command, args []string) error {
	// TODO: Implement tunnel stop for persistent tunnels
	printInfo("Tunnel stop not yet implemented for persistent tunnels")
	return nil
}

func runTunnelStatus(cmd *cobra.Command, args []string) error {
	// Check if project is initialized
	if !IsProjectInitialized() {
		return fmt.Errorf("no LocalCloud project found. Run 'lc setup' first")
	}

	cfg := config.Get()
	if cfg == nil {
		return fmt.Errorf("failed to load configuration")
	}

	// Load saved tunnel info
	info, err := network.LoadTunnelInfo(cfg.Project.Name)
	if err != nil {
		printWarning("No tunnel configuration found")
		fmt.Println("\nRun 'lc tunnel setup' to configure a tunnel")
		return nil
	}

	// Display status
	fmt.Printf("Provider:     %s\n", info.Provider)
	fmt.Printf("URL:          %s\n", info.URL)
	fmt.Printf("Created:      %s\n", info.CreatedAt.Format("2006-01-02 15:04:05"))
	fmt.Printf("Last Started: %s\n", info.LastStarted.Format("2006-01-02 15:04:05"))

	if info.Domain != "" {
		fmt.Printf("Domain:       %s\n", info.Domain)
	}

	// Test connectivity
	fmt.Print("\nTesting connection... ")
	connMgr, _ := network.NewConnectivityManager(cfg)
	if connMgr != nil {
		test, err := connMgr.TestConnectivity(info.URL)
		if err == nil && test.Success {
			printSuccess(fmt.Sprintf("OK (%dms)", test.HTTPLatency.Milliseconds()))
		} else {
			printError("Failed")
		}
	}

	return nil
}

// isPortInUse checks if a port is already in use
func isPortInUse(port int) bool {
	conn, err := net.Dial("tcp", fmt.Sprintf("localhost:%d", port))
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

// startTestServer starts a simple HTTP server for testing
func startTestServer(port int) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `
<!DOCTYPE html>
<html>
<head>
    <title>LocalCloud Tunnel</title>
    <style>
        body {
            font-family: -apple-system, system-ui, sans-serif;
            max-width: 600px;
            margin: 100px auto;
            padding: 20px;
            text-align: center;
        }
        h1 { color: #2563eb; }
        .success { 
            background: #10b981; 
            color: white; 
            padding: 10px 20px; 
            border-radius: 5px; 
            display: inline-block;
            margin: 20px 0;
        }
    </style>
</head>
<body>
    <h1>🚀 LocalCloud Tunnel</h1>
    <div class="success">✓ Tunnel is working!</div>
    <p>Your local service is now accessible from anywhere.</p>
    <p>Request from: %s</p>
</body>
</html>`, r.RemoteAddr)
	})

	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: mux,
	}

	go server.ListenAndServe()
}

// determineServicesToTunnel determines which services to tunnel based on flags
func determineServicesToTunnel(cfg *config.Config) map[string]int {
	services := make(map[string]int)

	// Handle auto-detect mode
	if autoDetect {
		detected := network.DetectRunningServices(cfg)
		for name, port := range detected {
			services[name] = port
		}
		printInfo(fmt.Sprintf("Auto-detected %d running services", len(detected)))
	}

	// If --all is specified, detect all running services
	if tunnelAll {
		return network.DetectRunningServices(cfg)
	}

	// Add services based on component flags
	if tunnelAPI {
		services["api"] = network.ComponentRoutes["api"]
	}
	if tunnelDB {
		services["db"] = network.ComponentRoutes["db"]
	}
	if tunnelMongo {
		services["mongo"] = network.ComponentRoutes["mongo"]
	}
	if tunnelStorage {
		if cfg.Services.Storage.Console != 0 {
			services["storage"] = cfg.Services.Storage.Console
		} else {
			services["storage"] = network.ComponentRoutes["storage"]
		}
	}
	if tunnelAI {
		services["ai"] = network.ComponentRoutes["ai"]
	}
	if tunnelCache {
		services["cache"] = network.ComponentRoutes["cache"]
	}
	if tunnelQueue {
		services["queue"] = network.ComponentRoutes["queue"]
	}

	// Add services from --components flag
	for _, component := range tunnelComponents {
		if port, exists := network.ComponentRoutes[component]; exists {
			services[component] = port
		}
	}

	// Add services from --service flag (flexible services)
	for _, service := range tunnelServices {
		if err := parseAndAddService(services, service); err != nil {
			printWarning(fmt.Sprintf("Invalid service specification '%s': %v", service, err))
		}
	}

	return services
}

// parseAndAddService parses service specifications like "name:port" or "name:url"
func parseAndAddService(services map[string]int, spec string) error {
	parts := strings.SplitN(spec, ":", 2)
	if len(parts) != 2 {
		return fmt.Errorf("service must be in format 'name:port' or 'name:url'")
	}

	name := strings.TrimSpace(parts[0])
	target := strings.TrimSpace(parts[1])

	if name == "" {
		return fmt.Errorf("service name cannot be empty")
	}

	// Try to parse as port number first
	if port, err := strconv.Atoi(target); err == nil {
		if port < 1 || port > 65535 {
			return fmt.Errorf("port must be between 1 and 65535")
		}
		services[name] = port
		return nil
	}

	// Try to parse as URL
	if strings.HasPrefix(target, "http://") || strings.HasPrefix(target, "https://") {
		u, err := url.Parse(target)
		if err != nil {
			return fmt.Errorf("invalid URL: %w", err)
		}

		port := 80
		if u.Scheme == "https" {
			port = 443
		}

		if u.Port() != "" {
			if p, err := strconv.Atoi(u.Port()); err == nil {
				port = p
			}
		}

		services[name] = port
		return nil
	}

	return fmt.Errorf("target must be a port number or URL (http://... or https://...)")
}

// runLegacyTunnel runs the original single-service tunnel
func runLegacyTunnel(cfg *config.Config, port int, customURL string) error {
	var displayURL string

	if customURL != "" {
		// Use custom URL
		displayURL = customURL

		// Extract port from URL if not specified
		if u, err := url.Parse(customURL); err == nil {
			if u.Port() != "" {
				if p, err := strconv.Atoi(u.Port()); err == nil {
					port = p
				}
			}
		}

		// Set custom target URL in config
		cfg.Connectivity.Tunnel.TargetURL = customURL
	} else {
		// Use localhost with specified port
		displayURL = fmt.Sprintf("http://localhost:%d", port)

		// Start a simple test server if nothing is running on the port
		if !isPortInUse(port) {
			printInfo(fmt.Sprintf("No service detected on port %d. Starting a test server...", port))
			startTestServer(port)
			// Give server time to start
			time.Sleep(1 * time.Second)
		}
	}

	// Create connectivity manager
	connMgr, err := network.NewConnectivityManager(cfg)
	if err != nil {
		return fmt.Errorf("failed to create connectivity manager: %w", err)
	}

	// Register the service with the specified port
	connMgr.RegisterService("tunnel", port)

	// Start tunnel with spinner
	spin := spinner.New(spinner.CharSets[14], 100)
	spin.Suffix = " Establishing tunnel connection..."
	spin.Start()

	ctx := context.Background()
	if err := connMgr.Start(ctx); err != nil {
		spin.Stop()
		return fmt.Errorf("failed to start tunnel: %w", err)
	}

	spin.Stop()

	// Get connection info
	info, err := connMgr.GetConnectionInfo()
	if err != nil {
		return err
	}

	// Display tunnel URL
	fmt.Println()
	if len(info.TunnelURLs) > 0 {
		for _, tunnelURL := range info.TunnelURLs {
			printSuccess(fmt.Sprintf("Tunnel established: %s", tunnelURL))
			fmt.Println()
			fmt.Println("Your local service is now accessible from anywhere!")
			fmt.Println("Share this URL to allow others to access your service.")
			break
		}
	}

	// Show what's being tunneled
	fmt.Println()
	fmt.Printf("Tunneling: %s → Public URL\n", displayURL)

	// Handle interrupt
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	fmt.Println("\nPress Ctrl+C to stop the tunnel...")
	<-sigChan

	fmt.Println("\nStopping tunnel...")
	connMgr.Stop()

	return nil
}

// runMultiServiceTunnel runs the new multi-service tunnel with reverse proxy
func runMultiServiceTunnel(cfg *config.Config, services map[string]int) error {
	printInfo(fmt.Sprintf("Setting up multi-service tunnel for %d services...", len(services)))

	// Create reverse proxy
	proxy := network.NewMultiTunnelProxy(tunnelPrefix)

	// Add services to proxy
	for name, port := range services {
		proxy.AddService(name, port)
		printInfo(fmt.Sprintf("- %s service on port %d", name, port))
	}

	// Start reverse proxy
	if err := proxy.Start(); err != nil {
		return fmt.Errorf("failed to start reverse proxy: %w", err)
	}
	defer proxy.Stop()

	// Start tunnel pointing to reverse proxy
	proxyPort := proxy.GetProxyPort()
	cfg.Connectivity.Tunnel.TargetURL = fmt.Sprintf("http://localhost:%d", proxyPort)

	// Create connectivity manager
	connMgr, err := network.NewConnectivityManager(cfg)
	if err != nil {
		return fmt.Errorf("failed to create connectivity manager: %w", err)
	}

	// Register the proxy as the main service
	connMgr.RegisterService("proxy", proxyPort)

	// Start tunnel with spinner
	spin := spinner.New(spinner.CharSets[14], 100)
	spin.Suffix = " Establishing multi-service tunnel connection..."
	spin.Start()

	ctx := context.Background()
	if err := connMgr.Start(ctx); err != nil {
		spin.Stop()
		return fmt.Errorf("failed to start tunnel: %w", err)
	}

	spin.Stop()

	// Get connection info
	info, err := connMgr.GetConnectionInfo()
	if err != nil {
		return err
	}

	// Display tunnel URLs
	fmt.Println()
	if len(info.TunnelURLs) > 0 {
		var baseTunnelURL string
		for _, tunnelURL := range info.TunnelURLs {
			baseTunnelURL = tunnelURL
			break
		}

		printSuccess("🚀 Multi-service tunnel established!")
		fmt.Println()

		// Get service-specific URLs
		serviceURLs := proxy.GetServiceURLs(baseTunnelURL)

		fmt.Println("Service URLs (HTTPS enabled):")
		for name, serviceURL := range serviceURLs {
			fmt.Printf("  %s: %s\n", name, serviceURL)
		}

		fmt.Println()
		fmt.Printf("Service Dashboard: %s\n", baseTunnelURL)
		fmt.Println()
		fmt.Println("All services are accessible from anywhere with HTTPS!")
		fmt.Println("Perfect for iOS development and external integrations.")
	}

	// Handle interrupt
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	fmt.Println("\nPress Ctrl+C to stop the tunnel...")
	<-sigChan

	fmt.Println("\nStopping multi-service tunnel...")
	connMgr.Stop()
	proxy.Stop()

	return nil
}

// runLegacyTunnelWithName runs the original single-service tunnel with custom service name
func runLegacyTunnelWithName(cfg *config.Config, port int, customURL, serviceName string) error {
	var displayURL string

	if customURL != "" {
		// Use custom URL
		displayURL = customURL

		// Extract port from URL if not specified
		if u, err := url.Parse(customURL); err == nil {
			if u.Port() != "" {
				if p, err := strconv.Atoi(u.Port()); err == nil {
					port = p
				}
			}
		}

		// Set custom target URL in config
		cfg.Connectivity.Tunnel.TargetURL = customURL
	} else {
		// Use localhost with specified port
		displayURL = fmt.Sprintf("http://localhost:%d", port)

		// Start a simple test server if nothing is running on the port
		if !isPortInUse(port) {
			printInfo(fmt.Sprintf("No service detected on port %d. Starting a test server...", port))
			startTestServer(port)
			// Give server time to start
			time.Sleep(1 * time.Second)
		}
	}

	// Create connectivity manager
	connMgr, err := network.NewConnectivityManager(cfg)
	if err != nil {
		return fmt.Errorf("failed to create connectivity manager: %w", err)
	}

	// Register the service with the custom name
	connMgr.RegisterService(serviceName, port)

	// Start tunnel with spinner
	spin := spinner.New(spinner.CharSets[14], 100)
	spin.Suffix = fmt.Sprintf(" Establishing tunnel connection for %s...", serviceName)
	spin.Start()

	ctx := context.Background()
	if err := connMgr.Start(ctx); err != nil {
		spin.Stop()
		return fmt.Errorf("failed to start tunnel: %w", err)
	}

	spin.Stop()

	// Get connection info
	info, err := connMgr.GetConnectionInfo()
	if err != nil {
		return err
	}

	// Display tunnel URL
	fmt.Println()
	if len(info.TunnelURLs) > 0 {
		for _, tunnelURL := range info.TunnelURLs {
			printSuccess(fmt.Sprintf("%s tunnel established: %s", serviceName, tunnelURL))
			fmt.Println()
			fmt.Println("Your local service is now accessible from anywhere!")
			fmt.Println("Share this URL to allow others to access your service.")
			break
		}
	}

	// Show what's being tunneled
	fmt.Println()
	fmt.Printf("Tunneling: %s (%s) → Public URL\n", displayURL, serviceName)

	// Handle interrupt
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	fmt.Println("\nPress Ctrl+C to stop the tunnel...")
	<-sigChan

	fmt.Println("\nStopping tunnel...")
	connMgr.Stop()

	return nil
}
