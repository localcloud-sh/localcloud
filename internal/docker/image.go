// internal/docker/image.go
package docker

import (
	"encoding/json"
	"fmt"
	"github.com/docker/docker/api/types/filters"
	"io"
	"strings"

	"github.com/docker/docker/api/types"
)

// ImageManager manages Docker images
type ImageManager interface {
	Pull(image string, progress chan<- PullProgress) error
	Exists(image string) (bool, error)
	Remove(image string) error
	List() ([]ImageInfo, error)
	GetSize(image string) (int64, error)
}

// ImageInfo represents image information
type ImageInfo struct {
	ID      string
	Name    string
	Tag     string
	Size    int64
	Created int64
}

// PullProgress represents image pull progress
type PullProgress struct {
	Status         string
	Progress       string
	ProgressDetail ProgressDetail
	Error          string
}

// ProgressDetail represents detailed progress information
type ProgressDetail struct {
	Current int64
	Total   int64
}

// imageManager implements ImageManager
type imageManager struct {
	client *Client
}

// NewImageManager creates a new image manager
func (c *Client) NewImageManager() ImageManager {
	return &imageManager{client: c}
}

// Pull pulls a Docker image with progress reporting
func (m *imageManager) Pull(image string, progress chan<- PullProgress) error {
	defer close(progress)

	// Parse image name and tag
	imageName, tag := parseImageTag(image)
	if tag == "" {
		tag = "latest"
	}
	fullImage := fmt.Sprintf("%s:%s", imageName, tag)

	// Start pulling the image
	reader, err := m.client.docker.ImagePull(
		m.client.ctx,
		fullImage,
		types.ImagePullOptions{},
	)
	if err != nil {
		return fmt.Errorf("failed to pull image %s: %w", fullImage, err)
	}
	defer reader.Close()

	// Parse progress updates
	decoder := json.NewDecoder(reader)
	for {
		var event PullProgress
		if err := decoder.Decode(&event); err != nil {
			if err == io.EOF {
				break
			}
			return fmt.Errorf("error reading pull progress: %w", err)
		}

		// Send progress update
		if progress != nil {
			progress <- event
		}

		// Check for errors
		if event.Error != "" {
			return fmt.Errorf("pull error: %s", event.Error)
		}
	}

	return nil
}

// Exists checks if an image exists locally
func (m *imageManager) Exists(image string) (bool, error) {
	imageName, tag := parseImageTag(image)
	if tag == "" {
		tag = "latest"
	}
	fullImage := fmt.Sprintf("%s:%s", imageName, tag)

	_, _, err := m.client.docker.ImageInspectWithRaw(m.client.ctx, fullImage)
	if err != nil {
		if strings.Contains(err.Error(), "No such image") {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// Remove removes an image
func (m *imageManager) Remove(image string) error {
	_, err := m.client.docker.ImageRemove(
		m.client.ctx,
		image,
		types.ImageRemoveOptions{
			Force:         false,
			PruneChildren: true,
		},
	)
	if err != nil {
		return fmt.Errorf("failed to remove image: %w", err)
	}
	return nil
}

// List lists all images
func (m *imageManager) List() ([]ImageInfo, error) {
	images, err := m.client.docker.ImageList(m.client.ctx, types.ImageListOptions{
		All: false,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list images: %w", err)
	}

	var result []ImageInfo
	for _, img := range images {
		// Skip <none> images
		if len(img.RepoTags) == 0 || img.RepoTags[0] == "<none>:<none>" {
			continue
		}

		for _, repoTag := range img.RepoTags {
			name, tag := parseImageTag(repoTag)
			result = append(result, ImageInfo{
				ID:      img.ID,
				Name:    name,
				Tag:     tag,
				Size:    img.Size,
				Created: img.Created,
			})
		}
	}

	return result, nil
}

// GetSize gets the size of an image
func (m *imageManager) GetSize(image string) (int64, error) {
	inspect, _, err := m.client.docker.ImageInspectWithRaw(m.client.ctx, image)
	if err != nil {
		return 0, fmt.Errorf("failed to inspect image: %w", err)
	}
	return inspect.Size, nil
}

// CleanupOldImages removes unused images to free up space
func (m *imageManager) CleanupOldImages() error {
	// Get all LocalCloud related images
	filterArgs := filters.NewArgs()
	filterArgs.Add("dangling", "true")

	report, err := m.client.docker.ImagesPrune(
		m.client.ctx,
		filterArgs,
	)
	if err != nil {
		return fmt.Errorf("failed to prune images: %w", err)
	}

	if report.SpaceReclaimed > 0 {
		fmt.Printf("Cleaned up %d bytes of unused images\n", report.SpaceReclaimed)
	}

	return nil
}

// parseImageTag splits an image string into name and tag
func parseImageTag(image string) (string, string) {
	parts := strings.Split(image, ":")
	if len(parts) == 1 {
		return parts[0], ""
	}
	// Handle case where image contains registry with port
	lastColon := strings.LastIndex(image, ":")
	if lastColon == -1 {
		return image, ""
	}

	// Check if part after colon could be a tag (not a port number)
	possibleTag := image[lastColon+1:]
	if strings.Contains(possibleTag, "/") {
		// This is likely a registry URL with port, not a tag
		return image, ""
	}

	return image[:lastColon], possibleTag
}
