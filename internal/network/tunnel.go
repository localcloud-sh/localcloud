// internal/network/tunnel.go
package network

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/localcloud-sh/localcloud/internal/config"
	"golang.ngrok.com/ngrok"
	ngrokconfig "golang.ngrok.com/ngrok/config"
)

// TunnelProvider interface for different tunnel implementations
type TunnelProvider interface {
	Connect(ctx context.Context, port int) (string, error)
	Disconnect() error
	GetURL() string
	IsPersistent() bool
	GetProviderName() string
}

// TunnelManager manages tunnel connections
type TunnelManager struct {
	provider TunnelProvider
	config   *config.TunnelConfig
}

// NewTunnelManager creates a new tunnel manager
func NewTunnelManager(cfg *config.ConnectivityConfig) (*TunnelManager, error) {
	if cfg == nil || !cfg.Enabled {
		return nil, fmt.Errorf("connectivity not enabled")
	}

	var provider TunnelProvider
	var err error
	providerName := cfg.Tunnel.Provider

	// If provider is "auto" or not ready, run onboarding
	if providerName == "auto" || !IsProviderReady(providerName) {
		onboarding := NewTunnelOnboarding()
		configuredProvider, onboardErr := onboarding.CheckAndSetup()
		if onboardErr == nil {
			providerName = configuredProvider
			// Update config with the selected provider
			cfg.Tunnel.Provider = providerName
		} else {
			return nil, onboardErr
		}
	}

	switch providerName {
	case "cloudflare":
		provider, err = NewCloudflareTunnel(&cfg.Tunnel)
	case "ngrok":
		provider, err = NewNgrokTunnel(&cfg.Tunnel)
	default:
		return nil, fmt.Errorf("unknown tunnel provider: %s", providerName)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to create tunnel provider: %w", err)
	}

	return &TunnelManager{
		provider: provider,
		config:   &cfg.Tunnel,
	}, nil
}

// Connect establishes a tunnel connection
func (tm *TunnelManager) Connect(ctx context.Context, port int) (string, error) {
	return tm.provider.Connect(ctx, port)
}

// Disconnect closes the tunnel
func (tm *TunnelManager) Disconnect() error {
	return tm.provider.Disconnect()
}

// GetURL returns the current tunnel URL
func (tm *TunnelManager) GetURL() string {
	return tm.provider.GetURL()
}

// GetProviderName returns the active provider name
func (tm *TunnelManager) GetProviderName() string {
	return tm.provider.GetProviderName()
}

// CloudflareTunnel implementation
type CloudflareTunnel struct {
	config     *config.TunnelConfig
	url        string
	cmd        *exec.Cmd
	configPath string
}

// NewCloudflareTunnel creates a new Cloudflare tunnel provider
func NewCloudflareTunnel(cfg *config.TunnelConfig) (*CloudflareTunnel, error) {
	// Check if cloudflared is available
	if _, err := exec.LookPath("cloudflared"); err != nil {
		return nil, fmt.Errorf("cloudflared not found in PATH")
	}

	return &CloudflareTunnel{
		config: cfg,
	}, nil
}

// Connect establishes a Cloudflare tunnel
func (ct *CloudflareTunnel) Connect(ctx context.Context, port int) (string, error) {
	if ct.config.Persist && ct.config.Cloudflare.TunnelID != "" {
		// Use existing named tunnel
		return ct.connectNamedTunnel(ctx, port)
	}

	// Use quick tunnel for development
	return ct.connectQuickTunnel(ctx, port)
}

// connectNamedTunnel connects using a persistent named tunnel
func (ct *CloudflareTunnel) connectNamedTunnel(ctx context.Context, port int) (string, error) {
	// Create config file
	configDir := filepath.Join(os.Getenv("HOME"), ".localcloud", "tunnels")
	os.MkdirAll(configDir, 0755)

	ct.configPath = filepath.Join(configDir, fmt.Sprintf("%s.yml", ct.config.Cloudflare.TunnelID))

	tunnelConfig := fmt.Sprintf(`
tunnel: %s
credentials-file: %s

ingress:
  - hostname: %s
    service: http://localhost:%d
  - service: http_status:404
`, ct.config.Cloudflare.TunnelID, ct.config.Cloudflare.Credentials, ct.config.Domain, port)

	if err := os.WriteFile(ct.configPath, []byte(tunnelConfig), 0644); err != nil {
		return "", fmt.Errorf("failed to write tunnel config: %w", err)
	}

	// Start tunnel
	ct.cmd = exec.CommandContext(ctx, "cloudflared", "tunnel", "run",
		"--config", ct.configPath,
		ct.config.Cloudflare.TunnelID,
	)

	if err := ct.cmd.Start(); err != nil {
		return "", fmt.Errorf("failed to start cloudflared: %w", err)
	}

	// Wait for tunnel to be ready
	time.Sleep(3 * time.Second)

	ct.url = fmt.Sprintf("https://%s", ct.config.Domain)
	return ct.url, nil
}

// connectQuickTunnel creates a temporary tunnel
func (ct *CloudflareTunnel) connectQuickTunnel(ctx context.Context, port int) (string, error) {
	// Determine target URL
	targetURL := fmt.Sprintf("http://localhost:%d", port)
	if ct.config.TargetURL != "" {
		targetURL = ct.config.TargetURL
	}

	ct.cmd = exec.CommandContext(ctx, "cloudflared", "tunnel", "--url", targetURL)

	// Capture output to get the URL
	output := &strings.Builder{}
	ct.cmd.Stderr = output

	if err := ct.cmd.Start(); err != nil {
		return "", fmt.Errorf("failed to start cloudflared: %w", err)
	}

	// Wait for URL to appear in output
	for i := 0; i < 30; i++ {
		if strings.Contains(output.String(), "https://") {
			lines := strings.Split(output.String(), "\n")
			for _, line := range lines {
				if strings.Contains(line, "https://") && strings.Contains(line, ".trycloudflare.com") {
					// Extract URL from line
					parts := strings.Fields(line)
					for _, part := range parts {
						if strings.HasPrefix(part, "https://") {
							ct.url = strings.TrimSpace(part)
							return ct.url, nil
						}
					}
				}
			}
		}
		time.Sleep(500 * time.Millisecond)
	}

	return "", fmt.Errorf("failed to get tunnel URL")
}

// Disconnect stops the Cloudflare tunnel
func (ct *CloudflareTunnel) Disconnect() error {
	if ct.cmd != nil && ct.cmd.Process != nil {
		return ct.cmd.Process.Kill()
	}
	return nil
}

// GetURL returns the tunnel URL
func (ct *CloudflareTunnel) GetURL() string {
	return ct.url
}

// IsPersistent returns true if using named tunnel
func (ct *CloudflareTunnel) IsPersistent() bool {
	return ct.config.Persist && ct.config.Cloudflare.TunnelID != ""
}

// GetProviderName returns "cloudflare"
func (ct *CloudflareTunnel) GetProviderName() string {
	return "cloudflare"
}

// NgrokTunnel implementation
type NgrokTunnel struct {
	config *config.TunnelConfig
	url    string
	tunnel ngrok.Tunnel
}

// NewNgrokTunnel creates a new Ngrok tunnel provider
func NewNgrokTunnel(cfg *config.TunnelConfig) (*NgrokTunnel, error) {
	if cfg.Ngrok.AuthToken == "" {
		// Check environment variable
		if token := os.Getenv("NGROK_AUTH_TOKEN"); token != "" {
			cfg.Ngrok.AuthToken = token
		} else {
			return nil, fmt.Errorf("ngrok auth token not configured")
		}
	}

	return &NgrokTunnel{
		config: cfg,
	}, nil
}

// Connect establishes an Ngrok tunnel
func (nt *NgrokTunnel) Connect(ctx context.Context, port int) (string, error) {
	// Determine target URL
	targetURL := fmt.Sprintf("localhost:%d", port)
	if nt.config.TargetURL != "" {
		// Parse custom URL to get host:port
		if u, err := url.Parse(nt.config.TargetURL); err == nil {
			targetURL = u.Host
		}
	}

	opts := []ngrokconfig.HTTPEndpointOption{
		ngrokconfig.WithForwardsTo(targetURL),
	}

	// Add domain if configured
	if nt.config.Domain != "" && nt.config.Persist {
		opts = append(opts, ngrokconfig.WithDomain(nt.config.Domain))
	}

	tunnel, err := ngrok.Listen(ctx,
		ngrokconfig.HTTPEndpoint(opts...),
		ngrok.WithAuthtoken(nt.config.Ngrok.AuthToken),
	)

	if err != nil {
		return "", fmt.Errorf("failed to create ngrok tunnel: %w", err)
	}

	nt.tunnel = tunnel
	nt.url = tunnel.URL()

	return nt.url, nil
}

// Disconnect stops the Ngrok tunnel
func (nt *NgrokTunnel) Disconnect() error {
	if nt.tunnel != nil {
		return nt.tunnel.Close()
	}
	return nil
}

// GetURL returns the tunnel URL
func (nt *NgrokTunnel) GetURL() string {
	return nt.url
}

// IsPersistent returns true if using custom domain
func (nt *NgrokTunnel) IsPersistent() bool {
	return nt.config.Domain != ""
}

// GetProviderName returns "ngrok"
func (nt *NgrokTunnel) GetProviderName() string {
	return "ngrok"
}

// TunnelInfo represents tunnel information for persistence
type TunnelInfo struct {
	Provider    string    `json:"provider"`
	URL         string    `json:"url"`
	TunnelID    string    `json:"tunnel_id,omitempty"`
	Domain      string    `json:"domain,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	LastStarted time.Time `json:"last_started"`
}

// SaveTunnelInfo saves tunnel information to disk
func SaveTunnelInfo(projectName string, info *TunnelInfo) error {
	configDir := filepath.Join(".localcloud", "tunnels")
	os.MkdirAll(configDir, 0755)

	infoPath := filepath.Join(configDir, fmt.Sprintf("%s.json", projectName))
	data, err := json.MarshalIndent(info, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(infoPath, data, 0644)
}

// LoadTunnelInfo loads tunnel information from disk
func LoadTunnelInfo(projectName string) (*TunnelInfo, error) {
	infoPath := filepath.Join(".localcloud", "tunnels", fmt.Sprintf("%s.json", projectName))
	data, err := os.ReadFile(infoPath)
	if err != nil {
		return nil, err
	}

	var info TunnelInfo
	if err := json.Unmarshal(data, &info); err != nil {
		return nil, err
	}

	return &info, nil
}
