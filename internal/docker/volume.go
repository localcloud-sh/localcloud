// internal/docker/volume.go
package docker

import (
	"fmt"
	"time"

	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/volume"
)

// VolumeManager manages Docker volumes
type VolumeManager interface {
	Create(name string, labels map[string]string) error
	Remove(name string) error
	Exists(name string) (bool, error)
	List(filter map[string]string) ([]VolumeInfo, error)
	Backup(volumeName string, targetPath string) error
	Restore(volumeName string, sourcePath string) error
}

// VolumeInfo represents volume information
type VolumeInfo struct {
	Name       string
	Driver     string
	Mountpoint string
	Created    time.Time
	Labels     map[string]string
	Scope      string
	Size       int64
}

// volumeManager implements VolumeManager
type volumeManager struct {
	client *Client
}

// NewVolumeManager creates a new volume manager
func (c *Client) NewVolumeManager() VolumeManager {
	return &volumeManager{client: c}
}

// Create creates a new volume
func (m *volumeManager) Create(name string, labels map[string]string) error {
	// Check if volume already exists
	exists, err := m.Exists(name)
	if err != nil {
		return err
	}
	if exists {
		return nil // Volume already exists, not an error
	}

	// Prepare volume create options
	options := volume.CreateOptions{
		Name:   name,
		Driver: "local",
		Labels: labels,
	}

	// Create the volume
	_, err = m.client.docker.VolumeCreate(m.client.ctx, options)
	if err != nil {
		return fmt.Errorf("failed to create volume %s: %w", name, err)
	}

	return nil
}

// Remove removes a volume
func (m *volumeManager) Remove(name string) error {
	// Remove the volume (force = true to remove even if in use)
	err := m.client.docker.VolumeRemove(m.client.ctx, name, true)
	if err != nil {
		return fmt.Errorf("failed to remove volume %s: %w", name, err)
	}
	return nil
}

// Exists checks if a volume exists
func (m *volumeManager) Exists(name string) (bool, error) {
	// Create filter for specific volume
	filterArgs := filters.NewArgs()
	filterArgs.Add("name", name)

	// List volumes with filter
	listOptions := volume.ListOptions{
		Filters: filterArgs,
	}

	resp, err := m.client.docker.VolumeList(m.client.ctx, listOptions)
	if err != nil {
		return false, fmt.Errorf("failed to list volumes: %w", err)
	}

	// Check if the exact volume name exists
	for _, vol := range resp.Volumes {
		if vol.Name == name {
			return true, nil
		}
	}

	return false, nil
}

// List lists volumes matching the filter
func (m *volumeManager) List(filterMap map[string]string) ([]VolumeInfo, error) {
	// Create filters
	filterArgs := filters.NewArgs()
	for key, value := range filterMap {
		filterArgs.Add(key, value)
	}

	// List volumes
	listOptions := volume.ListOptions{
		Filters: filterArgs,
	}

	resp, err := m.client.docker.VolumeList(m.client.ctx, listOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to list volumes: %w", err)
	}

	// Convert to VolumeInfo
	var volumes []VolumeInfo
	for _, v := range resp.Volumes {
		info := VolumeInfo{
			Name:       v.Name,
			Driver:     v.Driver,
			Mountpoint: v.Mountpoint,
			Labels:     v.Labels,
			Scope:      v.Scope,
		}

		// Parse created time
		if v.CreatedAt != "" {
			if t, err := time.Parse(time.RFC3339, v.CreatedAt); err == nil {
				info.Created = t
			}
		}

		volumes = append(volumes, info)
	}

	return volumes, nil
}

// Backup backs up a volume to a tar file
func (m *volumeManager) Backup(volumeName string, targetPath string) error {
	// This would be implemented using a temporary container
	// that mounts the volume and creates a tar archive
	// For now, return not implemented
	return fmt.Errorf("backup not implemented yet")
}

// Restore restores a volume from a tar file
func (m *volumeManager) Restore(volumeName string, sourcePath string) error {
	// This would be implemented using a temporary container
	// that mounts the volume and extracts the tar archive
	// For now, return not implemented
	return fmt.Errorf("restore not implemented yet")
}
