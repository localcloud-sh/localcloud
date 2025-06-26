// internal/docker/manager.go
package docker

import (
	"context"
	"fmt"

	"github.com/localcloud/localcloud/internal/config"
)

// Manager provides a high-level interface for Docker operations
type Manager struct {
	client    *Client
	config    *config.Config
	container ContainerManager
	network   NetworkManager
	volume    VolumeManager
	image     ImageManager
	services  *ServiceManager
}

// NewManager creates a new Docker manager
func NewManager(ctx context.Context, cfg *config.Config) (*Manager, error) {
	client, err := NewClient(ctx)
	if err != nil {
		return nil, err
	}

	// Check Docker version
	if err := client.CheckDockerVersion(); err != nil {
		return nil, err
	}

	manager := &Manager{
		client:    client,
		config:    cfg,
		container: client.NewContainerManager(),
		network:   client.NewNetworkManager(),
		volume:    client.NewVolumeManager(),
		image:     client.NewImageManager(),
	}

	// Initialize service manager
	manager.services = NewServiceManager(manager)

	return manager, nil
}

// Close closes the Docker client connection
func (m *Manager) Close() error {
	return m.client.Close()
}

// internal/docker/manager.go - Updated functions

// InitializeProject initializes Docker resources for a project
func (m *Manager) InitializeProject() error {
	fmt.Println("DEBUG: Manager.InitializeProject called")

	// Create default network
	networkName := fmt.Sprintf("localcloud_%s_default", m.config.Project.Name)
	fmt.Printf("DEBUG: Creating network: %s\n", networkName)

	_, err := m.network.Create(networkName)
	if err != nil {
		fmt.Printf("DEBUG: Network creation failed: %v\n", err)
		return fmt.Errorf("failed to create network: %w", err)
	}
	fmt.Println("DEBUG: Network created successfully")

	// Create volumes for each service
	volumes := m.getRequiredVolumes()
	fmt.Printf("DEBUG: Required volumes: %v\n", volumes)

	for _, vol := range volumes {
		volumeName := fmt.Sprintf("localcloud_%s_%s", m.config.Project.Name, vol)
		fmt.Printf("DEBUG: Creating volume: %s\n", volumeName)

		// FIX: Add labels parameter
		labels := map[string]string{
			"com.localcloud.project": m.config.Project.Name,
		}
		err := m.volume.Create(volumeName, labels)
		if err != nil {
			fmt.Printf("DEBUG: Volume creation failed: %v\n", err)
			return fmt.Errorf("failed to create volume %s: %w", vol, err)
		}
		fmt.Printf("DEBUG: Volume %s created successfully\n", volumeName)
	}

	fmt.Println("DEBUG: InitializeProject completed successfully")
	return nil
}

// CleanupProject removes all project resources
func (m *Manager) CleanupProject() error {
	// Stop all services first
	progress := make(chan ServiceProgress)
	go func() {
		for range progress {
			// Drain channel
		}
	}()

	if err := m.StopServices(progress); err != nil {
		return fmt.Errorf("failed to stop services: %w", err)
	}

	// Remove volumes
	// FIX: Add nil parameter for no filters
	volumes, err := m.volume.List(nil)
	if err != nil {
		return err
	}

	for _, vol := range volumes {
		if vol.Labels["com.localcloud.project"] == m.config.Project.Name {
			if err := m.volume.Remove(vol.Name); err != nil {
				// Log error but continue
				fmt.Printf("Warning: failed to remove volume %s: %v\n", vol.Name, err)
			}
		}
	}

	// Remove network
	networks, err := m.network.List()
	if err != nil {
		return err
	}

	for _, net := range networks {
		if net.Name == fmt.Sprintf("localcloud_%s_default", m.config.Project.Name) {
			if err := m.network.Remove(net.ID); err != nil {
				fmt.Printf("Warning: failed to remove network %s: %v\n", net.Name, err)
			}
		}
	}

	return nil
}
func (m *Manager) StartServicesByComponents(components []string, progress chan<- ServiceProgress) error {
	fmt.Printf("DEBUG: Manager.StartServicesByComponents called with components: %v\n", components)
	return m.services.StartServicesByComponents(components, progress)
}

// StartServices delegates to service manager
func (m *Manager) StartServices(progress chan<- ServiceProgress) error {
	fmt.Println("DEBUG: Manager.StartServices (ALL) called")
	return m.services.StartAll(progress)
}

// StartSelectedServices starts only the specified services
func (m *Manager) StartSelectedServices(services []string, progress chan<- ServiceProgress) error {
	fmt.Printf("DEBUG: Manager.StartSelectedServices called with: %v\n", services)
	return m.services.StartSelectedServices(services, progress)
}

// StopServices delegates to service manager
func (m *Manager) StopServices(progress chan<- ServiceProgress) error {
	return m.services.StopAll(progress)
}

// GetServicesStatus delegates to service manager
func (m *Manager) GetServicesStatus() ([]ServiceStatus, error) {
	return m.services.GetStatus()
}

// getRequiredVolumes returns list of volumes needed based on config
func (m *Manager) getRequiredVolumes() []string {
	volumes := []string{"ollama_models"} // Always need AI models volume

	if m.config.Services.Database.Type != "" {
		volumes = append(volumes, "postgres_data")
	}

	// Cache doesn't need volume (memory-only)
	// Remove this old code:
	// if m.config.Services.Cache.Type != "" {
	//     volumes = append(volumes, "redis_data")
	// }

	// Queue needs volume for persistence
	if m.config.Services.Queue.Type != "" {
		volumes = append(volumes, "redis_queue_data")
	}

	if m.config.Services.Storage.Type != "" {
		volumes = append(volumes, "minio_data")
	}

	return volumes
}

// GetClient returns the Docker client for advanced usage
func (m *Manager) GetClient() *Client {
	return m.client
}

// GetConfig returns the current configuration
func (m *Manager) GetConfig() *config.Config {
	return m.config
}
