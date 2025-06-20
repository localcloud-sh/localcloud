// package docker
//
// import (
// 	"context"
// 	"fmt"
// 	"io"
// 	"strings"
//
// 	"github.com/docker/docker/api/types"
// 	"github.com/docker/docker/api/types/container"
// 	"github.com/docker/docker/api/types/network"
// 	"github.com/docker/docker/client"
// 	"github.com/docker/go-connections/nat"
// )
//
// // Client wraps the Docker client for LocalCloud operations
// type Client struct {
// 	docker *client.Client
// }
//
// // NewClient creates a new Docker client
// func NewClient() (*Client, error) {
// 	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
// 	if err != nil {
// 		return nil, fmt.Errorf("failed to create Docker client: %w", err)
// 	}
//
// 	return &Client{docker: cli}, nil
// }
//
// // Close closes the Docker client
// func (c *Client) Close() error {
// 	return c.docker.Close()
// }
//
// // ContainerConfig represents container configuration
// type ContainerConfig struct {
// 	Name         string
// 	Image        string
// 	Ports        map[string]string // host:container
// 	Environment  map[string]string
// 	Volumes      map[string]string // host:container
// 	Networks     []string
// 	Labels       map[string]string
// 	Memory       string
// 	CPU          string
// 	RestartPolicy string
// }
//
// // CreateContainer creates a Docker container
// func (c *Client) CreateContainer(ctx context.Context, config ContainerConfig) (string, error) {
// 	// Convert port mappings
// 	portBindings := nat.PortMap{}
// 	exposedPorts := nat.PortSet{}
//
// 	for hostPort, containerPort := range config.Ports {
// 		port, err := nat.NewPort("tcp", containerPort)
// 		if err != nil {
// 			return "", fmt.Errorf("invalid port %s: %w", containerPort, err)
// 		}
//
// 		exposedPorts[port] = struct{}{}
// 		portBindings[port] = []nat.PortBinding{
// 			{
// 				HostIP:   "0.0.0.0",
// 				HostPort: hostPort,
// 			},
// 		}
// 	}
//
// 	// Convert environment variables
// 	env := make([]string, 0, len(config.Environment))
// 	for key, value := range config.Environment {
// 		env = append(env, fmt.Sprintf("%s=%s", key, value))
// 	}
//
// 	// Convert volume bindings
// 	binds := make([]string, 0, len(config.Volumes))
// 	for hostPath, containerPath := range config.Volumes {
// 		binds = append(binds, fmt.Sprintf("%s:%s", hostPath, containerPath))
// 	}
//
// 	// Container configuration
// 	containerConfig := &container.Config{
// 		Image:        config.Image,
// 		Env:          env,
// 		ExposedPorts: exposedPorts,
// 		Labels:       config.Labels,
// 	}
//
// 	// Host configuration
// 	hostConfig := &container.HostConfig{
// 		PortBindings: portBindings,
// 		Binds:        binds,
// 		NetworkMode:  container.NetworkMode("localcloud"),
// 	}
//
// 	// Set resource limits
// 	if config.Memory != "" {
// 		// TODO: Parse memory string (e.g., "2g" -> bytes)
// 		// hostConfig.Memory = parseMemory(config.Memory)
// 	}
//
// 	if config.CPU != "" {
// 		// TODO: Parse CPU string (e.g., "1.5" -> nano CPUs)
// 		// hostConfig.CPUPeriod = 100000
// 		// hostConfig.CPUQuota = parseCPU(config.CPU)
// 	}
//
// 	// Network configuration
// 	networkConfig := &network.NetworkingConfig{
// 		EndpointsConfig: make(map[string]*network.EndpointSettings),
// 	}
//
// 	for _, networkName := range config.Networks {
// 		networkConfig.EndpointsConfig[networkName] = &network.EndpointSettings{}
// 	}
//
// 	// Create container
// 	resp, err := c.docker.ContainerCreate(
// 		ctx,
// 		containerConfig,
// 		hostConfig,
// 		networkConfig,
// 		nil,
// 		config.Name,
// 	)
//
// 	if err != nil {
// 		return "", fmt.Errorf("failed to create container: %w", err)
// 	}
//
// 	return resp.ID, nil
// }
//
// // StartContainer starts a Docker container
// func (c *Client) StartContainer(ctx context.Context, containerID string) error {
// 	err := c.docker.ContainerStart(ctx, containerID, types.ContainerStartOptions{})
// 	if err != nil {
// 		return fmt.Errorf("failed to start container: %w", err)
// 	}
// 	return nil
// }
//
// // StopContainer stops a Docker container
// func (c *Client) StopContainer(ctx context.Context, containerID string) error {
// 	timeout := 30 // seconds
// 	err := c.docker.ContainerStop(ctx, containerID, container.StopOptions{
// 		Timeout: &timeout,
// 	})
// 	if err != nil {
// 		return fmt.Errorf("failed to stop container: %w", err)
// 	}
// 	return nil
// }
//
// // RemoveContainer removes a Docker container
// func (c *Client) RemoveContainer(ctx context.Context, containerID string) error {
// 	err := c.docker.ContainerRemove(ctx, containerID, types.ContainerRemoveOptions{
// 		Force: true,
// 	})
// 	if err != nil {
// 		return fmt.Errorf("failed to remove container: %w", err)
// 	}
// 	return nil
// }
//
// // ListContainers lists Docker containers with LocalCloud labels
// func (c *Client) ListContainers(ctx context.Context) ([]types.Container, error) {
// 	containers, err := c.docker.ContainerList(ctx, types.ContainerListOptions{
// 		All: true,
// 		Filters: map[string][]string{
// 			"label": {"com.localcloud.managed=true"},
// 		},
// 	})
// 	if err != nil {
// 		return nil, fmt.Errorf("failed to list containers: %w", err)
// 	}
// 	return containers, nil
// }
//
// // GetContainerStatus returns the status of a container
// func (c *Client) GetContainerStatus(ctx context.Context, containerID string) (string, error) {
// 	inspect, err := c.docker.ContainerInspect(ctx, containerID)
// 	if err != nil {
// 		return "", fmt.Errorf("failed to inspect container: %w", err)
// 	}
// 	return inspect.State.Status, nil
// }
//
// // PullImage pulls a Docker image
// func (c *Client) PullImage(ctx context.Context, image string) error {
// 	reader, err := c.docker.ImagePull(ctx, image, types.ImagePullOptions{})
// 	if err != nil {
// 		return fmt.Errorf("failed to pull image: %w", err)
// 	}
// 	defer reader.Close()
//
// 	// Read the pull output (usually for progress display)
// 	_, err = io.Copy(io.Discard, reader)
// 	if err != nil {
// 		return fmt.Errorf("failed to read pull output: %w", err)
// 	}
//
// 	return nil
// }
//
// // ImageExists checks if a Docker image exists locally
// func (c *Client) ImageExists(ctx context.Context, image string) (bool, error) {
// 	_, _, err := c.docker.ImageInspectWithRaw(ctx, image)
// 	if err != nil {
// 		if client.IsErrNotFound(err) {
// 			return false, nil
// 		}
// 		return false, fmt.Errorf("failed to inspect image: %w", err)
// 	}
// 	return true, nil
// }
//
// // CreateNetwork creates a Docker network
// func (c *Client) CreateNetwork(ctx context.Context, name string) error {
// 	_, err := c.docker.NetworkCreate(ctx, name, types.NetworkCreate{
// 		Driver: "bridge",
// 		Labels: map[string]string{
// 			"com.localcloud.managed": "true",
// 		},
// 	})
// 	if err != nil {
// 		return fmt.Errorf("failed to create network: %w", err)
// 	}
// 	return nil
// }
//
// // NetworkExists checks if a Docker network exists
// func (c *Client) NetworkExists(ctx context.Context, name string) (bool, error) {
// 	networks, err := c.docker.NetworkList(ctx, types.NetworkListOptions{})
// 	if err != nil {
// 		return false, fmt.Errorf("failed to list networks: %w", err)
// 	}
//
// 	for _, network := range networks {
// 		if network.Name == name {
// 			return true, nil
// 		}
// 	}
// 	return false, nil
// }
//
// // EnsureNetwork ensures a Docker network exists
// func (c *Client) EnsureNetwork(ctx context.Context, name string) error {
// 	exists, err := c.NetworkExists(ctx, name)
// 	if err != nil {
// 		return err
// 	}
//
// 	if !exists {
// 		return c.CreateNetwork(ctx, name)
// 	}
// 	return nil
// }
//
// // GetContainerLogs gets logs from a container
// func (c *Client) GetContainerLogs(ctx context.Context, containerID string, lines int) (string, error) {
// 	options := types.ContainerLogsOptions{
// 		ShowStdout: true,
// 		ShowStderr: true,
// 		Tail:       fmt.Sprintf("%d", lines),
// 	}
//
// 	reader, err := c.docker.ContainerLogs(ctx, containerID, options)
// 	if err != nil {
// 		return "", fmt.Errorf("failed to get container logs: %w", err)
// 	}
// 	defer reader.Close()
//
// 	logs, err := io.ReadAll(reader)
// 	if err != nil {
// 		return "", fmt.Errorf("failed to read logs: %w", err)
// 	}
//
// 	return string(logs), nil
// }
//
// // FindContainerByName finds a container by name
// func (c *Client) FindContainerByName(ctx context.Context, name string) (*types.Container, error) {
// 	containers, err := c.ListContainers(ctx)
// 	if err != nil {
// 		return nil, err
// 	}
//
// 	for _, container := range containers {
// 		for _, containerName := range container.Names {
// 			// Container names are prefixed with "/"
// 			if strings.TrimPrefix(containerName, "/") == name {
// 				return &container, nil
// 			}
// 		}
// 	}
//
// 	return nil, fmt.Errorf("container not found: %s", name)
// }