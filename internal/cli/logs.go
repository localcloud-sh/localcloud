// internal/cli/logs.go
package cli

import (
	"bufio"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
	"github.com/localcloud/localcloud/internal/config"
	"github.com/spf13/cobra"
)

var (
	followLogs   bool
	tailLines    int
	sinceTime    string
	logLevel     string
	outputFormat string
	searchTerm   string
)

var logsCmd = &cobra.Command{
	Use:   "logs [service]",
	Short: "Show logs from LocalCloud services",
	Long:  `Display logs from LocalCloud services with filtering options.`,
	Args:  cobra.MaximumNArgs(1),
	RunE:  runLogs,
	Example: `  lc logs                    # Show all logs
  lc logs ai                  # Show AI service logs
  lc logs -f                  # Follow log output
  lc logs -n 50               # Show last 50 lines
  lc logs --since 1h          # Show logs from last hour`,
}

func init() {
	logsCmd.Flags().BoolVarP(&followLogs, "follow", "f", false, "Follow log output")
	logsCmd.Flags().IntVarP(&tailLines, "tail", "n", 100, "Number of lines to show from the end")
	logsCmd.Flags().StringVar(&sinceTime, "since", "", "Show logs since timestamp (e.g., 1h, 30m)")
	logsCmd.Flags().StringVar(&outputFormat, "output", "text", "Output format (text, json)")
}

func runLogs(cmd *cobra.Command, args []string) error {
	// Check if project is initialized
	if !IsProjectInitialized() {
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

	// Test Docker connection
	_, err = cli.Ping(ctx)
	if err != nil {
		return fmt.Errorf("Docker is not running. Please start Docker Desktop")
	}

	// Get service name if specified
	var serviceName string
	if len(args) > 0 {
		serviceName = args[0]
		// Map aliases
		switch serviceName {
		case "database":
			serviceName = "postgres"
		case "cache":
			serviceName = "redis"
		case "storage":
			serviceName = "minio"
		}
	}

	// List containers with LocalCloud label
	filterArgs := filters.NewArgs()
	filterArgs.Add("label", fmt.Sprintf("com.localcloud.project=%s", cfg.Project.Name))

	containers, err := cli.ContainerList(ctx, types.ContainerListOptions{
		All:     false, // Only running containers
		Filters: filterArgs,
	})
	if err != nil {
		return fmt.Errorf("failed to list containers: %w", err)
	}

	if len(containers) == 0 {
		fmt.Println("No running LocalCloud containers found")
		return nil
	}

	// Filter by service if specified
	var targetContainers []types.Container
	for _, container := range containers {
		// Get service name from labels
		service := container.Labels["com.localcloud.service"]
		if service == "" {
			// Try to extract from container name
			service = extractServiceFromContainerName(container.Names[0])
		}

		if serviceName == "" || service == serviceName {
			targetContainers = append(targetContainers, container)
		}
	}

	if len(targetContainers) == 0 {
		if serviceName != "" {
			return fmt.Errorf("no containers found for service: %s", serviceName)
		}
		return fmt.Errorf("no containers found")
	}

	// Show header
	if !followLogs && outputFormat == "text" {
		if serviceName != "" {
			fmt.Printf("Showing logs for %s service", serviceName)
		} else {
			fmt.Printf("Showing logs for all services")
		}
		if tailLines > 0 && !followLogs {
			fmt.Printf(" (last %d lines)", tailLines)
		}
		fmt.Println(":")
		fmt.Println(strings.Repeat("─", 80))
		fmt.Println()
	}

	// Setup signal handling for graceful exit
	if followLogs {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, os.Interrupt)
		go func() {
			<-sigChan
			fmt.Println("\nStopping log stream...")
			os.Exit(0)
		}()
	}

	// Get logs from each container
	for _, container := range targetContainers {
		service := container.Labels["com.localcloud.service"]
		if service == "" {
			service = extractServiceFromContainerName(container.Names[0])
		}

		// Show service header if multiple containers
		if len(targetContainers) > 1 && !followLogs {
			fmt.Printf("\n=== %s ===\n", service)
		}

		// Get container logs
		options := types.ContainerLogsOptions{
			ShowStdout: true,
			ShowStderr: true,
			Follow:     followLogs,
			Timestamps: true,
			Tail:       fmt.Sprintf("%d", tailLines),
		}

		// Parse since time if provided
		if sinceTime != "" {
			if since, err := parseSinceTime(sinceTime); err == nil {
				options.Since = since.Format(time.RFC3339)
			}
		}

		reader, err := cli.ContainerLogs(ctx, container.ID, options)
		if err != nil {
			fmt.Printf("Error getting logs for %s: %v\n", service, err)
			continue
		}

		// Stream logs
		if followLogs {
			// For following, prefix each line with service name
			go func(svc string, r io.ReadCloser) {
				defer r.Close()
				scanner := bufio.NewScanner(r)
				for scanner.Scan() {
					line := scanner.Text()
					fmt.Printf("[%s] %s\n", svc, line)
				}
			}(service, reader)
		} else {
			// For non-following, process the stream properly
			buf := make([]byte, 8)
			for {
				// Read header
				_, err := reader.Read(buf)
				if err != nil {
					if err != io.EOF {
						fmt.Printf("Error reading logs: %v\n", err)
					}
					break
				}

				// Parse header to get size
				size := binary.BigEndian.Uint32(buf[4:8])
				if size == 0 {
					continue
				}

				// Read the actual log line
				line := make([]byte, size)
				_, err = reader.Read(line)
				if err != nil {
					break
				}

				// Print the log line
				fmt.Print(string(line))
			}
			reader.Close()
		}
	}

	// Keep main thread alive if following
	if followLogs {
		select {}
	}

	return nil
}

// Helper function to extract service name from container name
func extractServiceFromContainerName(containerName string) string {
	// Container names are like: /localcloud-ai, /localcloud-postgres, /localcloud-redis
	name := strings.TrimPrefix(containerName, "/")
	parts := strings.Split(name, "-")

	if len(parts) >= 2 {
		service := parts[1]
		// Map container name parts to service names
		switch service {
		case "ai", "ollama":
			return "ai"
		case "postgres", "postgresql":
			return "postgres"
		case "redis":
			// Could be cache or queue, check full name
			if len(parts) >= 3 && parts[2] == "queue" {
				return "queue"
			}
			return "cache"
		case "minio":
			return "storage"
		}
		return service
	}

	return "unknown"
}

// parseSinceTime parses a relative time string (1h, 30m, etc.) or absolute timestamp
func parseSinceTime(since string) (time.Time, error) {
	// Try parsing as duration first (1h, 30m, etc.)
	if duration, err := time.ParseDuration(since); err == nil {
		return time.Now().Add(-duration), nil
	}

	// Try parsing as absolute time
	if t, err := time.Parse(time.RFC3339, since); err == nil {
		return t, nil
	}

	if t, err := time.Parse("2006-01-02T15:04:05", since); err == nil {
		return t, nil
	}

	if t, err := time.Parse("2006-01-02", since); err == nil {
		return t, nil
	}

	return time.Time{}, fmt.Errorf("invalid time format: %s", since)
}
