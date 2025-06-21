// internal/cli/logs.go
package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
	"github.com/localcloud/localcloud/internal/config"
	"github.com/spf13/cobra"
)

var (
	followLogs bool
	tailLines  int
)

var logsCmd = &cobra.Command{
	Use:   "logs [service]",
	Short: "Show logs from LocalCloud services",
	Long: `Display logs from LocalCloud services. If no service is specified,
shows logs from all services.`,
	Args: cobra.MaximumNArgs(1),
	RunE: runLogs,
	Example: `  localcloud logs          # Show all logs
  localcloud logs ai       # Show AI service logs
  localcloud logs -f       # Follow log output
  localcloud logs -n 50    # Show last 50 lines`,
}

func init() {
	logsCmd.Flags().BoolVarP(&followLogs, "follow", "f", false, "Follow log output")
	logsCmd.Flags().IntVarP(&tailLines, "tail", "n", 100, "Number of lines to show from the end")
}

func runLogs(cmd *cobra.Command, args []string) error {
	// Check if project is initialized
	if !isProjectInitialized() {
		return fmt.Errorf("no LocalCloud project found")
	}

	// Get config
	cfg := config.Get()
	if cfg == nil {
		return fmt.Errorf("failed to load configuration")
	}

	// Create Docker client
	ctx := context.Background()
	cli, err := client.NewClientWithOpts(
		client.FromEnv,
		client.WithAPIVersionNegotiation(),
	)
	if err != nil {
		return fmt.Errorf("failed to create Docker client: %w", err)
	}
	defer cli.Close()

	service := ""
	if len(args) > 0 {
		service = args[0]
		// Validate service name
		validServices := []string{"ai", "postgres", "redis", "minio", "database", "cache", "storage"}
		isValid := false
		for _, s := range validServices {
			if s == service {
				isValid = true
				break
			}
		}
		if !isValid {
			return fmt.Errorf("unknown service '%s'. Available services: %s",
				service, strings.Join(validServices, ", "))
		}
	}

	// Map service names to container names
	serviceMap := map[string]string{
		"ai":       "localcloud-ai",
		"postgres": "localcloud-postgres",
		"database": "localcloud-postgres",
		"redis":    "localcloud-redis",
		"cache":    "localcloud-redis",
		"minio":    "localcloud-minio",
		"storage":  "localcloud-minio",
	}

	if service != "" {
		// Show logs for specific service
		containerName := serviceMap[service]
		if containerName == "" {
			containerName = "localcloud-" + service
		}

		fmt.Printf("Showing logs for %s service (last %d lines):\n", service, tailLines)
		fmt.Println(strings.Repeat("─", 60))

		return showContainerLogs(ctx, cli, containerName)
	} else {
		// Show logs for all services
		fmt.Printf("Showing logs for all services (last %d lines):\n", tailLines)
		fmt.Println(strings.Repeat("─", 60))

		// List all LocalCloud containers
		filterArgs := filters.NewArgs()
		filterArgs.Add("label", fmt.Sprintf("com.localcloud.project=%s", cfg.Project.Name))

		containers, err := cli.ContainerList(ctx, types.ContainerListOptions{
			All:     true,
			Filters: filterArgs,
		})
		if err != nil {
			return fmt.Errorf("failed to list containers: %w", err)
		}

		if len(containers) == 0 {
			fmt.Println("No LocalCloud containers found")
			return nil
		}

		// Show logs for each container
		for _, container := range containers {
			name := strings.TrimPrefix(container.Names[0], "/")
			fmt.Printf("\n[%s]\n", name)
			if err := showContainerLogs(ctx, cli, container.ID); err != nil {
				fmt.Printf("Error getting logs: %v\n", err)
			}
		}
	}

	return nil
}

func showContainerLogs(ctx context.Context, cli *client.Client, containerID string) error {
	options := types.ContainerLogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Follow:     followLogs,
		Tail:       fmt.Sprintf("%d", tailLines),
		Timestamps: true,
	}

	reader, err := cli.ContainerLogs(ctx, containerID, options)
	if err != nil {
		// Try by name if ID didn't work
		if strings.HasPrefix(containerID, "localcloud-") {
			// Find container by name
			containers, err := cli.ContainerList(ctx, types.ContainerListOptions{
				All: true,
			})
			if err == nil {
				for _, c := range containers {
					for _, name := range c.Names {
						if strings.TrimPrefix(name, "/") == containerID {
							return showContainerLogs(ctx, cli, c.ID)
						}
					}
				}
			}
		}
		return fmt.Errorf("container not found or not accessible: %w", err)
	}
	defer reader.Close()

	_, err = io.Copy(os.Stdout, reader)
	return err
}
