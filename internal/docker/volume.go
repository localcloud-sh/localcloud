// internal/docker/volume.go
package docker

import (
	"fmt"
	"strings"

	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/volume"
)

// VolumeManager manages Docker volumes
type VolumeManager interface {
	Create(name string) (string, error)
	Remove(name string) error
	Exists(name string) (bool, error)
	List() ([]VolumeInfo, error)
	Backup(volumeName string, backupPath string) error
	Restore(volumeName string, backupPath string) error
}

// VolumeInfo represents volume information
type VolumeInfo struct {
	Name       string
	Driver     string
	Mountpoint string
	CreatedAt  string
	Size       int64
	Labels     map[string]string
}

// volumeManager implements VolumeManager
type volumeManager struct {
	client *Client
}

// NewVolumeManager creates a new volume manager
func (c *Client) NewVolumeManager() VolumeManager {
	return &volumeManager{client: c}
}

// Remove removes a volume
func (m *volumeManager) Remove(name string) error {
	volumeName := getVolumeName(name)

	err := m.client.docker.VolumeRemove(m.client.ctx, volumeName, false)
	if err != nil {
		return fmt.Errorf("failed to remove volume: %w", err)
	}
	return nil
}

// Create creates a new volume
func (m *volumeManager) Create(name string) (string, error) {
	// Ensure volume name has project prefix
	volumeName := getVolumeName(name)
	fmt.Printf("DEBUG: Creating volume with name: %s\n", volumeName)

	// Check if volume already exists
	exists, err := m.Exists(volumeName)
	if err != nil {
		fmt.Printf("DEBUG: Error checking if volume exists: %v\n", err)
		// Don't return error, try to create anyway
		// return "", err
	}
	if exists {
		fmt.Printf("DEBUG: Volume already exists: %s\n", volumeName)
		return volumeName, nil
	}

	fmt.Printf("DEBUG: Volume does not exist, creating new volume\n")

	// Create volume
	vol, err := m.client.docker.VolumeCreate(
		m.client.ctx,
		volume.VolumeCreateBody{
			Name:   volumeName,
			Driver: "local",
			Labels: map[string]string{
				"com.localcloud.managed": "true",
				"com.localcloud.project": getProjectName(),
				"com.localcloud.service": name,
			},
		},
	)
	if err != nil {
		fmt.Printf("DEBUG: VolumeCreate failed: %v\n", err)
		return "", fmt.Errorf("failed to create volume: %w", err)
	}

	fmt.Printf("DEBUG: Volume created successfully: %s\n", vol.Name)
	return vol.Name, nil
}

// Exists checks if a volume exists
func (m *volumeManager) Exists(name string) (bool, error) {
	fmt.Printf("DEBUG: Checking if volume exists: %s\n", name)

	_, err := m.client.docker.VolumeInspect(m.client.ctx, name)
	if err != nil {
		// Check for various error messages
		errStr := strings.ToLower(err.Error())
		if strings.Contains(errStr, "no such volume") ||
			strings.Contains(errStr, "not found") ||
			strings.Contains(errStr, "404") {
			fmt.Printf("DEBUG: Volume does not exist (error: %v)\n", err)
			return false, nil
		}
		fmt.Printf("DEBUG: VolumeInspect returned unexpected error: %v\n", err)
		return false, fmt.Errorf("failed to inspect volume: %w", err)
	}
	fmt.Printf("DEBUG: Volume exists\n")
	return true, nil
}

// List lists all LocalCloud managed volumes
func (m *volumeManager) List() ([]VolumeInfo, error) {
	// Create filter for LocalCloud managed volumes
	filterArgs := filters.NewArgs()
	filterArgs.Add("label", "com.localcloud.managed=true")

	// List volumes with filter
	volumes, err := m.client.docker.VolumeList(m.client.ctx, filterArgs)
	if err != nil {
		return nil, fmt.Errorf("failed to list volumes: %w", err)
	}

	var result []VolumeInfo
	for _, v := range volumes.Volumes {

		info := VolumeInfo{
			Name:       v.Name,
			Driver:     v.Driver,
			Mountpoint: v.Mountpoint,
			CreatedAt:  v.CreatedAt,
			Labels:     v.Labels,
		}

		// Get volume size if available
		if v.UsageData != nil {
			info.Size = v.UsageData.Size
		}

		result = append(result, info)
	}

	return result, nil
}

// Backup backs up a volume to a tar file
func (m *volumeManager) Backup(volumeName string, backupPath string) error {
	// Ensure volume name has project prefix
	volumeName = getVolumeName(volumeName)

	// Create a temporary container to backup the volume
	containerConfig := ContainerConfig{
		Name:  "localcloud-backup-temp",
		Image: "alpine:latest",
		Volumes: []VolumeMount{
			{
				Source:   volumeName,
				Target:   "/backup-source",
				ReadOnly: true,
			},
		},
		Command: []string{"tar", "czf", "/backup.tar.gz", "-C", "/backup-source", "."},
	}

	cm := m.client.NewContainerManager()

	// Create and start container
	containerID, err := cm.Create(containerConfig)
	if err != nil {
		return fmt.Errorf("failed to create backup container: %w", err)
	}
	defer cm.Remove(containerID)

	if err := cm.Start(containerID); err != nil {
		return fmt.Errorf("failed to start backup container: %w", err)
	}

	// Wait for container to finish
	statusCh, errCh := m.client.docker.ContainerWait(m.client.ctx, containerID, "not-running")
	select {
	case err := <-errCh:
		if err != nil {
			return fmt.Errorf("error waiting for backup: %w", err)
		}
	case <-statusCh:
		// Container finished
	}

	// Copy backup file from container
	reader, _, err := m.client.docker.CopyFromContainer(m.client.ctx, containerID, "/backup.tar.gz")
	if err != nil {
		return fmt.Errorf("failed to copy backup: %w", err)
	}
	defer reader.Close()

	// Extract tar from the reader and save to file
	// This is simplified - in production, properly extract the tar stream
	return fmt.Errorf("backup implementation incomplete")
}

// Restore restores a volume from a tar file
func (m *volumeManager) Restore(volumeName string, backupPath string) error {
	// Ensure volume name has project prefix
	volumeName = getVolumeName(volumeName)

	// Implementation would:
	// 1. Create a temporary container with the volume mounted
	// 2. Copy the backup file to the container
	// 3. Extract the backup to the volume
	// 4. Clean up the container

	return fmt.Errorf("restore implementation incomplete")
}

// getVolumeName ensures volume name has project prefix
func getVolumeName(name string) string {
	if name == "" {
		return ""
	}
	// Add localcloud prefix if not present
	prefix := "localcloud_"
	if strings.HasPrefix(name, prefix) {
		return name
	}
	return prefix + name
}
