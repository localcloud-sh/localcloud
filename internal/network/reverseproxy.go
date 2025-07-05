// internal/network/reverseproxy.go
package network

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/localcloud-sh/localcloud/internal/config"
)

// ComponentRoutes maps component names to their default ports
var ComponentRoutes = map[string]int{
	"api":     3000,
	"web":     3000,
	"db":      5050,  // PgAdmin or custom DB interface
	"mongo":   8081,  // Mongo Express
	"storage": 9001,  // MinIO Console
	"ai":      11434, // Ollama
	"cache":   8001,  // Redis Commander or custom interface
	"queue":   8002,  // Redis Queue interface
}

// ServiceConfig represents configuration for a tunneled service
type ServiceConfig struct {
	Name        string `json:"name"`
	Port        int    `json:"port"`
	Path        string `json:"path,omitempty"`         // Optional path prefix
	Subdomain   string `json:"subdomain,omitempty"`    // Optional subdomain
	HealthCheck string `json:"health_check,omitempty"` // Health check endpoint
}

// MultiTunnelProxy manages multiple service routing through a single tunnel
type MultiTunnelProxy struct {
	services  map[string]*ServiceConfig
	proxyPort int
	server    *http.Server
	tunnelID  string
	prefix    string
	mu        sync.RWMutex
	isRunning bool
}

// NewMultiTunnelProxy creates a new multi-service tunnel proxy
func NewMultiTunnelProxy(prefix string) *MultiTunnelProxy {
	// Generate unique tunnel ID
	tunnelID := generateTunnelID()

	return &MultiTunnelProxy{
		services:  make(map[string]*ServiceConfig),
		proxyPort: 8080, // Default proxy port
		tunnelID:  tunnelID,
		prefix:    prefix,
	}
}

// AddService adds a service to the proxy
func (mtp *MultiTunnelProxy) AddService(name string, port int) {
	mtp.mu.Lock()
	defer mtp.mu.Unlock()

	service := &ServiceConfig{
		Name: name,
		Port: port,
		Path: fmt.Sprintf("/%s", name),
	}

	// Set subdomain for the service
	if mtp.prefix != "" {
		service.Subdomain = fmt.Sprintf("%s-%s", mtp.prefix, name)
	} else {
		service.Subdomain = name
	}

	mtp.services[name] = service
}

// GetServiceURLs returns the public URLs for all services
func (mtp *MultiTunnelProxy) GetServiceURLs(baseTunnelURL string) map[string]string {
	mtp.mu.RLock()
	defer mtp.mu.RUnlock()

	urls := make(map[string]string)

	// Parse the base tunnel URL to get the base domain
	baseURL, err := url.Parse(baseTunnelURL)
	if err != nil {
		return urls
	}

	// Create service-specific URLs
	for name, service := range mtp.services {
		// For Cloudflare tunnels, we can use subdomain-style URLs
		// Format: service-tunnelid.trycloudflare.com
		host := baseURL.Host

		// Extract the tunnel ID from the base URL
		if strings.Contains(host, ".trycloudflare.com") {
			parts := strings.Split(host, ".")
			if len(parts) >= 3 {
				// Create service-specific subdomain
				serviceHost := fmt.Sprintf("%s-%s.%s.%s", service.Subdomain, mtp.tunnelID, parts[1], parts[2])
				urls[name] = fmt.Sprintf("https://%s", serviceHost)
			}
		} else {
			// Fallback to path-based routing for other providers
			urls[name] = fmt.Sprintf("%s%s", baseTunnelURL, service.Path)
		}
	}

	return urls
}

// Start starts the reverse proxy server
func (mtp *MultiTunnelProxy) Start() error {
	mtp.mu.Lock()
	defer mtp.mu.Unlock()

	if mtp.isRunning {
		return fmt.Errorf("proxy already running")
	}

	mux := http.NewServeMux()

	// Add health check endpoint
	mux.HandleFunc("/health", mtp.handleHealth)

	// Add service landing page
	mux.HandleFunc("/", mtp.handleServiceList)

	// Add routes for each service
	for _, service := range mtp.services {
		mtp.addServiceRoute(mux, service)
	}

	mtp.server = &http.Server{
		Addr:    fmt.Sprintf(":%d", mtp.proxyPort),
		Handler: mtp.corsMiddleware(mux),
	}

	mtp.isRunning = true

	// Start server in goroutine
	go func() {
		if err := mtp.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Printf("Proxy server error: %v\n", err)
		}
	}()

	return nil
}

// Stop stops the reverse proxy server
func (mtp *MultiTunnelProxy) Stop() error {
	mtp.mu.Lock()
	defer mtp.mu.Unlock()

	if !mtp.isRunning || mtp.server == nil {
		return nil
	}

	mtp.isRunning = false
	return mtp.server.Close()
}

// GetProxyPort returns the proxy port
func (mtp *MultiTunnelProxy) GetProxyPort() int {
	return mtp.proxyPort
}

// addServiceRoute adds a route for a specific service
func (mtp *MultiTunnelProxy) addServiceRoute(mux *http.ServeMux, service *ServiceConfig) {
	targetURL := fmt.Sprintf("http://localhost:%d", service.Port)
	target, err := url.Parse(targetURL)
	if err != nil {
		fmt.Printf("Error parsing target URL for %s: %v\n", service.Name, err)
		return
	}

	proxy := httputil.NewSingleHostReverseProxy(target)

	// Custom error handler
	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		fmt.Printf("Proxy error for %s: %v\n", service.Name, err)
		w.WriteHeader(http.StatusBadGateway)
		fmt.Fprintf(w, `
<!DOCTYPE html>
<html>
<head>
    <title>Service Unavailable</title>
    <style>
        body { 
            font-family: -apple-system, system-ui, sans-serif; 
            text-align: center; 
            padding: 50px; 
            color: #333; 
        }
        .error { color: #e74c3c; }
        .service { color: #3498db; font-weight: bold; }
    </style>
</head>
<body>
    <h1 class="error">Service Unavailable</h1>
    <p>The <span class="service">%s</span> service is not running on port %d.</p>
    <p>Please start the service and try again.</p>
    <p><a href="/">← Back to Service List</a></p>
</body>
</html>`, service.Name, service.Port)
	}

	// Add route with path prefix
	pattern := service.Path + "/"
	mux.HandleFunc(pattern, http.StripPrefix(strings.TrimSuffix(service.Path, "/"), proxy).ServeHTTP)

	// Also handle exact path match
	mux.HandleFunc(service.Path, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == service.Path {
			// Redirect to add trailing slash
			http.Redirect(w, r, service.Path+"/", http.StatusMovedPermanently)
			return
		}
		proxy.ServeHTTP(w, r)
	})
}

// handleHealth handles health check requests
func (mtp *MultiTunnelProxy) handleHealth(w http.ResponseWriter, r *http.Request) {
	mtp.mu.RLock()
	defer mtp.mu.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	status := map[string]interface{}{
		"status":    "healthy",
		"services":  len(mtp.services),
		"timestamp": time.Now().Format(time.RFC3339),
		"proxy_id":  mtp.tunnelID,
	}

	fmt.Fprintf(w, `{
		"status": "%s",
		"services": %d,
		"timestamp": "%s",
		"proxy_id": "%s"
	}`, status["status"], status["services"], status["timestamp"], status["proxy_id"])
}

// handleServiceList displays available services
func (mtp *MultiTunnelProxy) handleServiceList(w http.ResponseWriter, r *http.Request) {
	mtp.mu.RLock()
	defer mtp.mu.RUnlock()

	w.Header().Set("Content-Type", "text/html")

	html := `
<!DOCTYPE html>
<html>
<head>
    <title>LocalCloud Tunnel Services</title>
    <style>
        body {
            font-family: -apple-system, system-ui, sans-serif;
            max-width: 800px;
            margin: 50px auto;
            padding: 20px;
            line-height: 1.6;
        }
        h1 { color: #2563eb; text-align: center; }
        .service {
            background: #f8f9fa;
            border: 1px solid #e9ecef;
            border-radius: 8px;
            padding: 20px;
            margin: 15px 0;
            display: flex;
            justify-content: space-between;
            align-items: center;
        }
        .service-info { flex: 1; }
        .service-name { font-weight: bold; font-size: 1.2em; color: #2563eb; }
        .service-port { color: #6c757d; font-size: 0.9em; }
        .service-link {
            background: #2563eb;
            color: white;
            padding: 10px 20px;
            border-radius: 5px;
            text-decoration: none;
            font-weight: bold;
        }
        .service-link:hover { background: #1d4ed8; }
        .empty { text-align: center; color: #6c757d; }
        .footer { 
            text-align: center; 
            margin-top: 40px; 
            padding-top: 20px; 
            border-top: 1px solid #e9ecef; 
            color: #6c757d; 
        }
    </style>
</head>
<body>
    <h1>🚀 LocalCloud Tunnel Services</h1>
`

	if len(mtp.services) == 0 {
		html += `
    <div class="empty">
        <h3>No services configured</h3>
        <p>Start your LocalCloud services and tunnel them with:</p>
        <code>lc tunnel start --api --storage --db</code>
    </div>
`
	} else {
		for _, service := range mtp.services {
			html += fmt.Sprintf(`
    <div class="service">
        <div class="service-info">
            <div class="service-name">%s</div>
            <div class="service-port">Port: %d</div>
        </div>
        <a href="%s" class="service-link" target="_blank">Open →</a>
    </div>
`, service.Name, service.Port, service.Path)
		}
	}

	html += fmt.Sprintf(`
    <div class="footer">
        <p>Tunnel ID: %s | Services: %d</p>
        <p>Powered by LocalCloud</p>
    </div>
</body>
</html>`, mtp.tunnelID, len(mtp.services))

	fmt.Fprint(w, html)
}

// corsMiddleware adds CORS headers for cross-origin requests
func (mtp *MultiTunnelProxy) corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Add CORS headers
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Accept, Authorization, Content-Type, X-CSRF-Token")

		// Handle preflight requests
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// generateTunnelID generates a unique tunnel identifier
func generateTunnelID() string {
	bytes := make([]byte, 4)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}

// DetectRunningServices checks which services are actually running
func DetectRunningServices(cfg *config.Config) map[string]int {
	services := make(map[string]int)

	// Check each component based on configuration
	if cfg.Services.Database.Type != "" && cfg.Services.Database.Port != 0 {
		if isPortInUse(5050) { // PgAdmin default port
			services["db"] = 5050
		}
	}

	if cfg.Services.MongoDB.Type != "" && cfg.Services.MongoDB.Port != 0 {
		if isPortInUse(8081) { // Mongo Express default port
			services["mongo"] = 8081
		}
	}

	if cfg.Services.Storage.Type != "" && cfg.Services.Storage.Console != 0 {
		services["storage"] = cfg.Services.Storage.Console
	}

	// Check for common API ports
	for _, port := range []int{3000, 8000, 8080, 5000} {
		if isPortInUse(port) {
			services["api"] = port
			break
		}
	}

	// Check for AI services (Ollama)
	if isPortInUse(11434) {
		services["ai"] = 11434
	}

	return services
}

// isPortInUse checks if a port is in use
func isPortInUse(port int) bool {
	// This is a placeholder - in real implementation, we'd check if port is open
	// For now, we'll implement basic port checking logic
	return true // Assume services are running for demo
}
