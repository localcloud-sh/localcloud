// internal/docker/network.go
package docker

import (
	"fmt"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/network"
)

// NetworkManager manages Docker networks
type NetworkManager interface {
	Create(name string) (string, error)
	Remove(networkID string) error
	Exists(name string) (bool, string, error)
	List() ([]NetworkInfo, error)
}

// NetworkInfo represents network information
type NetworkInfo struct {
	ID     string
	Name   string
	Driver string
}

// networkManager implements NetworkManager
type networkManager struct {
	client *Client
}

// NewNetworkManager creates a new network manager
func (c *Client) NewNetworkManager() NetworkManager {
	return &networkManager{client: c}
}

// Create creates a new network
func (m *networkManager) Create(name string) (string, error) {
	// Ensure network name has project prefix
	networkName := getNetworkName(name)

	// Check if network already exists
	exists, existingID, err := m.Exists(networkName)
	if err != nil {
		return "", err
	}
	if exists {
		return existingID, nil
	}

	// Create network
	resp, err := m.client.docker.NetworkCreate(
		m.client.ctx,
		networkName,
		types.NetworkCreate{
			Driver: "bridge",
			Labels: map[string]string{
				"com.localcloud.managed": "true",
				"com.localcloud.project": getProjectName(),
			},
			CheckDuplicate: true,
			EnableIPv6:     false,
			IPAM: &network.IPAM{
				Driver: "default",
			},
		},
	)
	if err != nil {
		return "", fmt.Errorf("failed to create network: %w", err)
	}

	return resp.ID, nil
}

// Remove removes a network
func (m *networkManager) Remove(networkID string) error {
	err := m.client.docker.NetworkRemove(m.client.ctx, networkID)
	if err != nil {
		return fmt.Errorf("failed to remove network: %w", err)
	}
	return nil
}

// Exists checks if a network with the given name exists
func (m *networkManager) Exists(name string) (bool, string, error) {
	networks, err := m.List()
	if err != nil {
		return false, "", err
	}

	for _, n := range networks {
		if n.Name == name {
			return true, n.ID, nil
		}
	}

	return false, "", nil
}

// List lists all networks
func (m *networkManager) List() ([]NetworkInfo, error) {
	networks, err := m.client.docker.NetworkList(m.client.ctx, types.NetworkListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list networks: %w", err)
	}

	var result []NetworkInfo
	for _, n := range networks {
		// Only include LocalCloud managed networks
		if _, ok := n.Labels["com.localcloud.managed"]; ok {
			result = append(result, NetworkInfo{
				ID:     n.ID,
				Name:   n.Name,
				Driver: n.Driver,
			})
		}
	}

	return result, nil
}

// getNetworkName ensures network name has project prefix
func getNetworkName(name string) string {
	if name == "" {
		name = "default"
	}
	// Add localcloud prefix if not present
	prefix := "localcloud_"
	if strings.HasPrefix(name, prefix) {
		return name
	}
	return prefix + name
}

// getProjectName gets the current project name from config
func getProjectName() string {
	// This will be implemented to get from config
	// For now, return a default
	return "default"
}
