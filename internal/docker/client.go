// internal/docker/client.go
package docker

import (
	"context"
	"fmt"
	"github.com/docker/docker/api/types/container"
	"io"
	"os"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
)

// Client wraps the Docker client with LocalCloud-specific functionality
type Client struct {
	docker *client.Client
	ctx    context.Context
}

// NewClient creates a new Docker client
func NewClient(ctx context.Context) (*Client, error) {
	// Create Docker client with environment defaults
	cli, err := client.NewClientWithOpts(
		client.FromEnv,
		client.WithAPIVersionNegotiation(),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create Docker client: %w", err)
	}

	// Test connection
	_, err = cli.Ping(ctx)
	if err != nil {
		return nil, fmt.Errorf("Docker daemon not running: %w", err)
	}

	return &Client{
		docker: cli,
		ctx:    ctx,
	}, nil
}

// Close closes the Docker client connection
func (c *Client) Close() error {
	return c.docker.Close()
}

// IsDockerRunning checks if Docker daemon is accessible
func (c *Client) IsDockerRunning() bool {
	_, err := c.docker.Ping(c.ctx)
	return err == nil
}

// GetDockerInfo returns Docker system information
func (c *Client) GetDockerInfo() (types.Info, error) {
	return c.docker.Info(c.ctx)
}

// CheckDockerVersion ensures Docker version is compatible
func (c *Client) CheckDockerVersion() error {
	version, err := c.docker.ServerVersion(c.ctx)
	if err != nil {
		return fmt.Errorf("failed to get Docker version: %w", err)
	}

	// Check minimum version (Docker 20.10+)
	// This is a simplified check - in production, parse and compare versions properly
	if version.APIVersion < "1.41" {
		return fmt.Errorf("Docker version too old. Please upgrade to Docker 20.10 or newer")
	}

	return nil
}

// PortBinding represents a port binding configuration
type PortBinding struct {
	ContainerPort string
	HostPort      string
	Protocol      string // tcp or udp
}

// VolumeMount represents a volume mount configuration
type VolumeMount struct {
	Source   string
	Target   string
	Type     string // "bind" or "volume"
	ReadOnly bool
}

// ContainerConfig represents container configuration
type ContainerConfig struct {
	Name          string
	Image         string
	Env           map[string]string
	Ports         []PortBinding
	Volumes       []VolumeMount
	Networks      []string
	RestartPolicy string
	Memory        int64 // Memory limit in bytes
	CPUQuota      int64 // CPU quota (100000 = 1 CPU)
	HealthCheck   *HealthCheckConfig
	Labels        map[string]string
	Command       []string
}

// HealthCheckConfig represents health check configuration
type HealthCheckConfig struct {
	Test        []string
	Interval    int // seconds
	Timeout     int // seconds
	Retries     int
	StartPeriod int // seconds
}

// ContainerInfo represents container information
type ContainerInfo struct {
	ID         string
	Name       string
	Image      string
	Status     string
	State      string
	Health     string
	Ports      map[string]string
	Created    int64
	StartedAt  int64
	Memory     int64
	CPUPercent float64
}

// parsePortBindings converts our port config to Docker format
func parsePortBindings(ports []PortBinding) (nat.PortSet, nat.PortMap, error) {
	portSet := nat.PortSet{}
	portMap := nat.PortMap{}

	for _, p := range ports {
		containerPort, err := nat.NewPort(p.Protocol, p.ContainerPort)
		if err != nil {
			return nil, nil, fmt.Errorf("invalid container port %s: %w", p.ContainerPort, err)
		}

		portSet[containerPort] = struct{}{}

		if p.HostPort != "" {
			portMap[containerPort] = []nat.PortBinding{
				{
					HostIP:   "0.0.0.0",
					HostPort: p.HostPort,
				},
			}
		}
	}

	return portSet, portMap, nil
}

// formatEnvironment converts environment map to slice
func formatEnvironment(env map[string]string) []string {
	var result []string
	for k, v := range env {
		result = append(result, fmt.Sprintf("%s=%s", k, v))
	}
	return result
}

// getContainerName ensures container name has project prefix
func getContainerName(name string) string {
	if name == "" {
		return ""
	}
	// Add localcloud prefix if not present
	prefix := "localcloud-"
	if len(name) >= len(prefix) && name[:len(prefix)] == prefix {
		return name
	}
	return prefix + name
}

// StreamLogs streams container logs
func (c *Client) StreamLogs(containerID string, follow bool, since string, tail string, writer io.Writer) error {
	options := types.ContainerLogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Follow:     follow,
		Timestamps: true,
		Since:      since,
		Tail:       tail,
	}

	reader, err := c.docker.ContainerLogs(c.ctx, containerID, options)
	if err != nil {
		return fmt.Errorf("failed to get container logs: %w", err)
	}
	defer reader.Close()

	_, err = io.Copy(writer, reader)
	return err
}

// GetDockerInstallInstructions returns platform-specific Docker installation instructions
func GetDockerInstallInstructions() string {
	instructions := `Docker is not installed or not running.

To install Docker:`

	switch {
	case isWSL():
		instructions += `

For Windows with WSL2:
1. Install Docker Desktop: https://docs.docker.com/desktop/install/windows-install/
2. Enable WSL2 integration in Docker Desktop settings
3. Start Docker Desktop
4. Run 'localcloud start' again`

	case isMac():
		instructions += `

For macOS:
1. Install Docker Desktop: https://docs.docker.com/desktop/install/mac-install/
   Or use Homebrew: brew install --cask docker
2. Start Docker Desktop from Applications
3. Wait for Docker to fully start
4. Run 'localcloud start' again`

	default: // Linux
		instructions += `

For Linux:
1. Install Docker Engine: https://docs.docker.com/engine/install/
   Ubuntu/Debian: sudo apt-get install docker-ce docker-ce-cli containerd.io
   Fedora: sudo dnf install docker-ce docker-ce-cli containerd.io
2. Start Docker: sudo systemctl start docker
3. Add user to docker group: sudo usermod -aG docker $USER
4. Log out and back in
5. Run 'localcloud start' again`
	}

	instructions += `

For more information: https://docs.docker.com/get-docker/`

	return instructions
}

// Helper functions to detect platform
func isWSL() bool {
	_, err := os.Stat("/proc/sys/fs/binfmt_misc/WSLInterop")
	return err == nil
}

func isMac() bool {
	return os.Getenv("GOOS") == "darwin" ||
		(os.Getenv("GOOS") == "" && fileExists("/System/Library/CoreServices/SystemVersion.plist"))
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
func (c *Client) GetDockerClient() *client.Client {
	return c.docker
}

// ContainerWait waits for a container to reach a certain condition
func (c *Client) ContainerWait(ctx context.Context, containerID string, condition container.WaitCondition) (<-chan container.WaitResponse, <-chan error) {
	return c.docker.ContainerWait(ctx, containerID, condition)
}
