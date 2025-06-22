// internal/network/connectivity.go
package network

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/localcloud/localcloud/internal/config"
)

// ConnectivityManager manages all connectivity features
type ConnectivityManager struct {
	discovery *NetworkDiscovery
	tunnel    *TunnelManager
	config    *config.Config
	services  map[string]int // service name -> port mapping
}

// NewConnectivityManager creates a new connectivity manager
func NewConnectivityManager(cfg *config.Config) (*ConnectivityManager, error) {
	cm := &ConnectivityManager{
		discovery: NewNetworkDiscovery(),
		config:    cfg,
		services:  make(map[string]int),
	}

	// Initialize tunnel manager if connectivity is enabled
	if cfg.Connectivity != nil && cfg.Connectivity.Enabled {
		tunnel, err := NewTunnelManager(cfg.Connectivity)
		if err != nil {
			// Tunnel errors are not fatal, just log
			fmt.Printf("Warning: Tunnel initialization failed: %v\n", err)
		} else {
			cm.tunnel = tunnel
		}
	}

	return cm, nil
}

// RegisterService registers a service port
func (cm *ConnectivityManager) RegisterService(name string, port int) {
	cm.services[name] = port
}

// Start starts all connectivity services
func (cm *ConnectivityManager) Start(ctx context.Context) error {
	// Start mDNS if enabled
	if cm.config.Connectivity != nil && cm.config.Connectivity.MDNS.Enabled {
		if err := cm.discovery.StartMDNS(cm.config.Project.Name, cm.services); err != nil {
			fmt.Printf("Warning: mDNS start failed: %v\n", err)
		}
	}

	// Start tunnel if configured
	if cm.tunnel != nil {
		// Find primary port (web or api)
		port := 3000 // default
		if p, ok := cm.services["web"]; ok {
			port = p
		} else if p, ok := cm.services["api"]; ok {
			port = p
		} else if p, ok := cm.services["tunnel"]; ok {
			// Use explicitly set tunnel port
			port = p
		}

		url, err := cm.tunnel.Connect(ctx, port)
		if err != nil {
			return fmt.Errorf("failed to establish tunnel: %w", err)
		}

		// Save tunnel info for persistence
		info := &TunnelInfo{
			Provider:    cm.tunnel.GetProviderName(),
			URL:         url,
			CreatedAt:   time.Now(),
			LastStarted: time.Now(),
		}

		if err := SaveTunnelInfo(cm.config.Project.Name, info); err != nil {
			fmt.Printf("Warning: Failed to save tunnel info: %v\n", err)
		}
	}

	return nil
}

// Stop stops all connectivity services
func (cm *ConnectivityManager) Stop() error {
	// Stop mDNS
	cm.discovery.StopMDNS()

	// Stop tunnel
	if cm.tunnel != nil {
		return cm.tunnel.Disconnect()
	}

	return nil
}

// GetConnectionInfo returns all connection information
func (cm *ConnectivityManager) GetConnectionInfo() (*ConnectionInfo, error) {
	info := &ConnectionInfo{
		ProjectName: cm.config.Project.Name,
		Status:      "running",
		LocalURLs:   make(map[string]string),
		NetworkURLs: make(map[string][]string),
		MDNSURLs:    make(map[string]string),
		TunnelURLs:  make(map[string]string),
		GeneratedAt: time.Now(),
	}

	// Get local URLs
	for name, port := range cm.services {
		info.LocalURLs[name] = fmt.Sprintf("http://localhost:%d", port)
		info.MDNSURLs[name] = fmt.Sprintf("http://%s.local:%d", cm.config.Project.Name, port)
	}

	// Get network URLs
	networkURLs, err := cm.discovery.GetNetworkURLs(cm.config.Project.Name, cm.services)
	if err == nil {
		info.NetworkURLs = networkURLs
	}

	// Get tunnel URL if available
	if cm.tunnel != nil && cm.tunnel.GetURL() != "" {
		tunnelURL := cm.tunnel.GetURL()
		// Apply to all services (they'll be routed by the tunnel)
		for name := range cm.services {
			info.TunnelURLs[name] = tunnelURL
		}
	}

	return info, nil
}

// TestConnectivity tests connectivity to a URL
func (cm *ConnectivityManager) TestConnectivity(url string) (*ConnectivityTest, error) {
	test := &ConnectivityTest{
		URL:      url,
		TestedAt: time.Now(),
	}

	// HTTP test
	start := time.Now()
	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	resp, err := client.Get(url)
	test.HTTPLatency = time.Since(start)

	if err != nil {
		test.HTTPStatus = "failed"
		test.Error = err.Error()
		return test, nil
	}
	defer resp.Body.Close()

	test.HTTPStatus = fmt.Sprintf("%d", resp.StatusCode)
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		test.Success = true
	}

	// Test WebSocket if it's an HTTP URL
	if wsURL := httpToWS(url); wsURL != "" {
		// WebSocket test would go here
		test.WebSocketStatus = "not_tested"
	}

	return test, nil
}

// GetMobileConfig returns configuration for mobile apps
func (cm *ConnectivityManager) GetMobileConfig() (*MobileConfig, error) {
	info, err := cm.GetConnectionInfo()
	if err != nil {
		return nil, err
	}

	config := &MobileConfig{
		ProjectName: info.ProjectName,
		UpdatedAt:   time.Now(),
		Endpoints:   make(map[string]MobileEndpoint),
	}

	// Prefer tunnel URL if available, otherwise use local network
	for service, localURL := range info.LocalURLs {
		endpoint := MobileEndpoint{
			Local: localURL,
		}

		if tunnelURL, ok := info.TunnelURLs[service]; ok && tunnelURL != "" {
			endpoint.Public = tunnelURL
			endpoint.Primary = tunnelURL // Tunnel is primary if available
		} else if networkURLs, ok := info.NetworkURLs[service]; ok && len(networkURLs) > 0 {
			// Use first non-localhost network URL
			for _, url := range networkURLs {
				if url != localURL {
					endpoint.Network = url
					endpoint.Primary = url
					break
				}
			}
		} else {
			endpoint.Primary = localURL
		}

		// Add WebSocket URL if applicable
		if service == "api" || service == "web" {
			endpoint.WebSocket = httpToWS(endpoint.Primary)
		}

		config.Endpoints[service] = endpoint
	}

	// Add environment variables format
	config.EnvFormat = generateEnvFormat(config)

	return config, nil
}

// ConnectionInfo represents all connection information
type ConnectionInfo struct {
	ProjectName string              `json:"project_name"`
	Status      string              `json:"status"`
	LocalURLs   map[string]string   `json:"local_urls"`
	NetworkURLs map[string][]string `json:"network_urls"`
	MDNSURLs    map[string]string   `json:"mdns_urls"`
	TunnelURLs  map[string]string   `json:"tunnel_urls"`
	GeneratedAt time.Time           `json:"generated_at"`
}

// ConnectivityTest represents a connectivity test result
type ConnectivityTest struct {
	URL             string        `json:"url"`
	Success         bool          `json:"success"`
	HTTPStatus      string        `json:"http_status"`
	HTTPLatency     time.Duration `json:"http_latency"`
	WebSocketStatus string        `json:"websocket_status"`
	Error           string        `json:"error,omitempty"`
	TestedAt        time.Time     `json:"tested_at"`
}

// MobileConfig represents configuration for mobile apps
type MobileConfig struct {
	ProjectName string                    `json:"project_name"`
	UpdatedAt   time.Time                 `json:"updated_at"`
	Endpoints   map[string]MobileEndpoint `json:"endpoints"`
	EnvFormat   string                    `json:"env_format"`
}

// MobileEndpoint represents an endpoint configuration
type MobileEndpoint struct {
	Primary   string `json:"primary"`   // The URL to use
	Local     string `json:"local"`     // localhost URL
	Network   string `json:"network"`   // LAN IP URL
	Public    string `json:"public"`    // Tunnel URL
	WebSocket string `json:"websocket"` // WebSocket URL
}

// ToJSON returns JSON representation
func (mc *MobileConfig) ToJSON() (string, error) {
	data, err := json.MarshalIndent(mc, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// httpToWS converts HTTP URL to WebSocket URL
func httpToWS(url string) string {
	if url == "" {
		return ""
	}

	wsURL := url
	if len(url) > 7 && url[:7] == "http://" {
		wsURL = "ws://" + url[7:]
	} else if len(url) > 8 && url[:8] == "https://" {
		wsURL = "wss://" + url[8:]
	}

	return wsURL + "/ws"
}

// generateEnvFormat generates environment variable format
func generateEnvFormat(config *MobileConfig) string {
	env := "# Add to your .env file:\n"

	if api, ok := config.Endpoints["api"]; ok {
		env += fmt.Sprintf("API_URL=%s\n", api.Primary)
		if api.WebSocket != "" {
			env += fmt.Sprintf("WS_URL=%s\n", api.WebSocket)
		}
	}

	if web, ok := config.Endpoints["web"]; ok && web.Primary != config.Endpoints["api"].Primary {
		env += fmt.Sprintf("WEB_URL=%s\n", web.Primary)
	}

	return env
}
