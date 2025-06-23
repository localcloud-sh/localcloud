package templates

import (
	"fmt"
	"math/rand"
	"net"
	"sync"
	"time"
)

// PortManager manages port allocation for services
type PortManager struct {
	mu        sync.Mutex
	allocated map[int]string // port -> service name
	checker   PortChecker
}

// PortChecker interface for checking port availability
type PortChecker interface {
	CheckPort(port int) bool
}

// DefaultPortChecker implements basic port checking
type DefaultPortChecker struct{}

// CheckPort checks if a port is available
func (d *DefaultPortChecker) CheckPort(port int) bool {
	timeout := time.Second
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("localhost:%d", port), timeout)
	if err != nil {
		// Port is available (connection failed)
		return true
	}
	conn.Close()
	// Port is in use (connection succeeded)
	return false
}

// NewPortManager creates a new port manager
func NewPortManager() *PortManager {
	return &PortManager{
		allocated: make(map[int]string),
		checker:   &DefaultPortChecker{},
	}
}

// NewPortManagerWithChecker creates a port manager with custom checker
func NewPortManagerWithChecker(checker PortChecker) *PortManager {
	return &PortManager{
		allocated: make(map[int]string),
		checker:   checker,
	}
}

// DefaultPorts contains default ports for services
var DefaultPorts = map[string]int{
	"ollama":        11434,
	"postgres":      5432,
	"redis":         6379,
	"minio":         9000,
	"minio-console": 9001,
	"api":           8080,
	"frontend":      3000,
	"localllama":    8081,
}

// AllocatePort finds an available port for a service
func (pm *PortManager) AllocatePort(service string, preferred int) (int, error) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	// Check if service already has a port allocated
	for port, svc := range pm.allocated {
		if svc == service {
			return port, nil
		}
	}

	// Try preferred port first
	if preferred > 0 && pm.isPortAvailable(preferred) {
		pm.allocated[preferred] = service
		return preferred, nil
	}

	// Try default port for the service
	if defaultPort, ok := DefaultPorts[service]; ok {
		if pm.isPortAvailable(defaultPort) {
			pm.allocated[defaultPort] = service
			return defaultPort, nil
		}

		// Try ports near the default
		for i := 1; i <= 10; i++ {
			port := defaultPort + i
			if pm.isPortAvailable(port) {
				pm.allocated[port] = service
				return port, nil
			}
		}
	}

	// Find random available port
	port, err := pm.findRandomPort()
	if err != nil {
		return 0, err
	}

	pm.allocated[port] = service
	return port, nil
}

// ReleasePort releases a previously allocated port
func (pm *PortManager) ReleasePort(port int) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	delete(pm.allocated, port)
}

// GetAllocatedPorts returns all allocated ports
func (pm *PortManager) GetAllocatedPorts() map[int]string {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	result := make(map[int]string)
	for port, service := range pm.allocated {
		result[port] = service
	}
	return result
}

// GetServicePort returns the allocated port for a service
func (pm *PortManager) GetServicePort(service string) (int, bool) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	for port, svc := range pm.allocated {
		if svc == service {
			return port, true
		}
	}
	return 0, false
}

// isPortAvailable checks if a port is available (not in use)
func (pm *PortManager) isPortAvailable(port int) bool {
	// Check if already allocated by us
	if _, allocated := pm.allocated[port]; allocated {
		return false
	}

	// Check if port is actually available on the system
	return pm.checker.CheckPort(port)
}

// findRandomPort finds a random available port
func (pm *PortManager) findRandomPort() (int, error) {
	// Try random ports in the dynamic/private range (49152-65535)
	rand.Seed(time.Now().UnixNano())

	for attempts := 0; attempts < 100; attempts++ {
		port := 49152 + rand.Intn(16383)
		if pm.isPortAvailable(port) {
			return port, nil
		}
	}

	// Fallback: try sequential ports
	for port := 49152; port < 65535; port++ {
		if pm.isPortAvailable(port) {
			return port, nil
		}
	}

	return 0, fmt.Errorf("no available ports found")
}

// PortAllocation represents port assignments for a template
type PortAllocation struct {
	API       int            `json:"api_port"`
	Frontend  int            `json:"frontend_port"`
	Database  int            `json:"database_port"`
	Cache     int            `json:"cache_port"`
	Storage   int            `json:"storage_port"`
	StorageUI int            `json:"storage_ui_port"`
	AI        int            `json:"ai_port"`
	Custom    map[string]int `json:"custom_ports"`
}

// AllocateTemplatePorts allocates all required ports for a template
func (pm *PortManager) AllocateTemplatePorts(services []string, options SetupOptions) (*PortAllocation, error) {
	allocation := &PortAllocation{
		Custom: make(map[string]int),
	}

	// Helper to allocate with preference
	allocate := func(service string, preferred int) (int, error) {
		return pm.AllocatePort(service, preferred)
	}

	var err error

	// Allocate standard ports based on required services
	for _, service := range services {
		switch service {
		case "api":
			allocation.API, err = allocate("api", options.APIPort)
		case "frontend":
			allocation.Frontend, err = allocate("frontend", options.FrontendPort)
		case "postgres", "database":
			allocation.Database, err = allocate("postgres", options.DatabasePort)
		case "redis", "cache":
			allocation.Cache, err = allocate("redis", 0)
		case "minio", "storage":
			allocation.Storage, err = allocate("minio", 0)
			if err == nil {
				allocation.StorageUI, err = allocate("minio-console", 0)
			}
		case "ollama", "ai":
			allocation.AI, err = allocate("ollama", 0)
		default:
			// Custom service
			allocation.Custom[service], err = allocate(service, 0)
		}

		if err != nil {
			// Rollback allocations on error
			pm.rollbackAllocations(allocation)
			return nil, fmt.Errorf("failed to allocate port for %s: %w", service, err)
		}
	}

	return allocation, nil
}

// rollbackAllocations releases all allocated ports in case of error
func (pm *PortManager) rollbackAllocations(allocation *PortAllocation) {
	if allocation.API > 0 {
		pm.ReleasePort(allocation.API)
	}
	if allocation.Frontend > 0 {
		pm.ReleasePort(allocation.Frontend)
	}
	if allocation.Database > 0 {
		pm.ReleasePort(allocation.Database)
	}
	if allocation.Cache > 0 {
		pm.ReleasePort(allocation.Cache)
	}
	if allocation.Storage > 0 {
		pm.ReleasePort(allocation.Storage)
	}
	if allocation.StorageUI > 0 {
		pm.ReleasePort(allocation.StorageUI)
	}
	if allocation.AI > 0 {
		pm.ReleasePort(allocation.AI)
	}
	for _, port := range allocation.Custom {
		pm.ReleasePort(port)
	}
}
