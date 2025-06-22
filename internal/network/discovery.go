// internal/network/discovery.go
package network

import (
	"context"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/grandcat/zeroconf"
)

// NetworkDiscovery handles local network discovery and mDNS
type NetworkDiscovery struct {
	mdnsServer *zeroconf.Server
	mu         sync.Mutex
	localIPs   []string
}

// NewNetworkDiscovery creates a new network discovery instance
func NewNetworkDiscovery() *NetworkDiscovery {
	return &NetworkDiscovery{}
}

// GetLocalIPs returns all valid local IP addresses
func (n *NetworkDiscovery) GetLocalIPs() ([]string, error) {
	n.mu.Lock()
	defer n.mu.Unlock()

	if len(n.localIPs) > 0 {
		return n.localIPs, nil
	}

	ips := []string{}
	interfaces, err := net.Interfaces()
	if err != nil {
		return nil, fmt.Errorf("failed to get network interfaces: %w", err)
	}

	for _, iface := range interfaces {
		// Skip down interfaces
		if iface.Flags&net.FlagUp == 0 {
			continue
		}

		// Skip loopback
		if iface.Flags&net.FlagLoopback != 0 {
			continue
		}

		// Skip Docker and virtual interfaces
		if strings.HasPrefix(iface.Name, "docker") ||
			strings.HasPrefix(iface.Name, "br-") ||
			strings.HasPrefix(iface.Name, "veth") ||
			strings.Contains(iface.Name, "bridge") {
			continue
		}

		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}

		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}

			// Only IPv4 for now
			if ip == nil || ip.IsLoopback() || ip.To4() == nil {
				continue
			}

			ips = append(ips, ip.String())
		}
	}

	n.localIPs = ips
	return ips, nil
}

// StartMDNS starts mDNS service advertisement
func (n *NetworkDiscovery) StartMDNS(projectName string, services map[string]int) error {
	n.mu.Lock()
	defer n.mu.Unlock()

	// Stop existing server if any
	if n.mdnsServer != nil {
		n.mdnsServer.Shutdown()
	}

	// Get primary service port (usually web UI)
	port := 3000
	if webPort, ok := services["web"]; ok {
		port = webPort
	} else if apiPort, ok := services["api"]; ok {
		port = apiPort
	}

	// Build TXT records with service info
	txt := []string{
		fmt.Sprintf("project=%s", projectName),
		fmt.Sprintf("version=0.1.0"),
		"service=localcloud",
	}

	// Add service ports to TXT
	for name, port := range services {
		txt = append(txt, fmt.Sprintf("%s_port=%d", name, port))
	}

	// Register mDNS service
	server, err := zeroconf.Register(
		projectName,        // Instance name
		"_localcloud._tcp", // Service type
		"local.",           // Domain
		port,               // Port
		txt,                // TXT records
		nil,                // Interfaces (nil = all)
	)

	if err != nil {
		return fmt.Errorf("failed to register mDNS service: %w", err)
	}

	n.mdnsServer = server
	return nil
}

// StopMDNS stops the mDNS server
func (n *NetworkDiscovery) StopMDNS() {
	n.mu.Lock()
	defer n.mu.Unlock()

	if n.mdnsServer != nil {
		n.mdnsServer.Shutdown()
		n.mdnsServer = nil
	}
}

// DiscoverServices discovers other LocalCloud services on the network
func (n *NetworkDiscovery) DiscoverServices(timeout time.Duration) ([]ServiceInfo, error) {
	resolver, err := zeroconf.NewResolver(nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create resolver: %w", err)
	}

	entries := make(chan *zeroconf.ServiceEntry)
	services := []ServiceInfo{}
	var mu sync.Mutex

	go func() {
		for entry := range entries {
			mu.Lock()
			services = append(services, ServiceInfo{
				Name:    entry.Instance,
				Host:    entry.HostName,
				Port:    entry.Port,
				IPs:     entry.AddrIPv4,
				TxtData: entry.Text,
			})
			mu.Unlock()
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	err = resolver.Browse(ctx, "_localcloud._tcp", "local.", entries)
	if err != nil {
		return nil, fmt.Errorf("failed to browse services: %w", err)
	}

	<-ctx.Done()
	return services, nil
}

// GetNetworkURLs returns all possible network URLs for services
func (n *NetworkDiscovery) GetNetworkURLs(projectName string, services map[string]int) (map[string][]string, error) {
	urls := make(map[string][]string)

	ips, err := n.GetLocalIPs()
	if err != nil {
		return nil, err
	}

	for serviceName, port := range services {
		serviceURLs := []string{}

		// Local URLs
		serviceURLs = append(serviceURLs, fmt.Sprintf("http://localhost:%d", port))

		// Network IPs
		for _, ip := range ips {
			serviceURLs = append(serviceURLs, fmt.Sprintf("http://%s:%d", ip, port))
		}

		// mDNS URL
		serviceURLs = append(serviceURLs, fmt.Sprintf("http://%s.local:%d", projectName, port))

		urls[serviceName] = serviceURLs
	}

	return urls, nil
}

// ServiceInfo represents discovered service information
type ServiceInfo struct {
	Name    string
	Host    string
	Port    int
	IPs     []net.IP
	TxtData []string
}

// ParseTxtData parses TXT record data into a map
func (s ServiceInfo) ParseTxtData() map[string]string {
	data := make(map[string]string)
	for _, txt := range s.TxtData {
		parts := strings.SplitN(txt, "=", 2)
		if len(parts) == 2 {
			data[parts[0]] = parts[1]
		}
	}
	return data
}
