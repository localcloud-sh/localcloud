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
	Long:  `Start a tunnel connection using the configured provider.`,
	RunE:  runTunnelStart,
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
}

func runTunnelSetup(cmd *cobra.Command, args []string) error {
	// Check if project is initialized
	if !IsProjectInitialized() {
		return fmt.Errorf("no LocalCloud project found. Run 'localcloud init' first")
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
			Connectivity: config.ConnectivityConfig{ // Remove the & here
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

	// Get port and URL from flags
	port, _ := cmd.Flags().GetInt("port")
	customURL, _ := cmd.Flags().GetString("url")

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
		for _, url := range info.TunnelURLs {
			printSuccess(fmt.Sprintf("Tunnel established: %s", url))
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

func runTunnelStop(cmd *cobra.Command, args []string) error {
	// TODO: Implement tunnel stop for persistent tunnels
	printInfo("Tunnel stop not yet implemented for persistent tunnels")
	return nil
}

func runTunnelStatus(cmd *cobra.Command, args []string) error {
	// Check if project is initialized
	if !IsProjectInitialized() {
		return fmt.Errorf("no LocalCloud project found. Run 'localcloud init' first")
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
