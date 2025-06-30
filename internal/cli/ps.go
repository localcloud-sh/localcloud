// internal/cli/ps.go
package cli

import (
	"context"
	"fmt"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
	"github.com/localcloud/localcloud/internal/config"
	"github.com/spf13/cobra"
	"os"
	"strings"
	"text/tabwriter"
)

var (
	showAll bool
)

var psCmd = &cobra.Command{
	Use:   "ps",
	Short: "List LocalCloud containers",
	Long:  `Display a list of all LocalCloud containers with their status.`,
	RunE:  runPs,
}

func init() {
	psCmd.Flags().BoolVarP(&showAll, "all", "a", false, "Show all containers (including stopped)")
}

func runPs(cmd *cobra.Command, args []string) error {
	// Check if project is initialized
	if !IsProjectInitialized() {
		return fmt.Errorf("no LocalCloud project found")
	}

	// Get config
	cfg := config.Get()
	if cfg == nil {
		return fmt.Errorf("failed to load configuration")
	}

	// Create Docker client directly for ps command
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

	// Create filters
	filterArgs := filters.NewArgs()
	filterArgs.Add("label", fmt.Sprintf("com.localcloud.project=%s", cfg.Project.Name))

	// List containers with LocalCloud label
	options := types.ContainerListOptions{
		All:     showAll,
		Filters: filterArgs,
	}

	containers, err := cli.ContainerList(ctx, options)
	if err != nil {
		return fmt.Errorf("failed to list containers: %w", err)
	}

	if len(containers) == 0 {
		if showAll {
			fmt.Println("No LocalCloud containers found")
		} else {
			fmt.Println("No running LocalCloud containers")
			fmt.Println("Use 'localcloud ps -a' to see all containers")
		}
		return nil
	}

	// Create a tabwriter for aligned output
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)

	// Print header
	fmt.Fprintln(w, "CONTAINER ID\tNAME\tIMAGE\tSTATUS\tPORTS\t")
	fmt.Fprintln(w, "────────────\t────\t─────\t──────\t─────\t")

	// Print containers
	for _, container := range containers {
		// Get container name (remove leading /)
		name := strings.TrimPrefix(container.Names[0], "/")

		// Get container ID (first 12 chars)
		id := container.ID[:12]

		// Format image with model info for AI service
		image := formatImageWithModel(container, cfg)

		// Format ports
		ports := formatPorts(container.Ports)

		// Color status based on state
		status := container.Status
		if container.State == "running" {
			status = successColor(status)
		} else if container.State == "exited" {
			status = errorColor(status)
		} else {
			status = warningColor(status)
		}

		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t\n",
			id,
			name,
			image,
			status,
			ports,
		)
	}

	w.Flush()

	// Show summary
	runningCount := 0
	for _, c := range containers {
		if c.State == "running" {
			runningCount++
		}
	}

	fmt.Printf("\n%d containers (%d running)\n", len(containers), runningCount)

	return nil
}

// formatImageWithModel formats the image name, showing model for AI service
func formatImageWithModel(container types.Container, cfg *config.Config) string {
	// Get service name from labels
	serviceName := container.Labels["localcloud.service"]

	// For AI/Ollama service, show the model if configured
	if serviceName == "ai" || serviceName == "ollama" {
		// Check if we have model configuration
		if cfg.Services.AI.Default != "" {
			return fmt.Sprintf("ollama/ollama (%s)", cfg.Services.AI.Default)
		} else if len(cfg.Services.AI.Models) > 0 {
			return fmt.Sprintf("ollama/ollama (%s)", cfg.Services.AI.Models[0])
		}
		// If no model configured, just return the original image
		// This way we see what's actually running
	}

	// For all other cases, return as-is
	return container.Image
}

// formatPorts formats container ports for display
func formatPorts(ports []types.Port) string {
	if len(ports) == 0 {
		return "-"
	}

	var portStrs []string
	for _, p := range ports {
		if p.PublicPort > 0 {
			portStr := fmt.Sprintf("%s:%d->%d/%s", p.IP, p.PublicPort, p.PrivatePort, p.Type)
			portStrs = append(portStrs, portStr)
		} else {
			portStr := fmt.Sprintf("%d/%s", p.PrivatePort, p.Type)
			portStrs = append(portStrs, portStr)
		}
	}

	// Return first port if multiple, or indicate multiple
	if len(portStrs) > 2 {
		return fmt.Sprintf("%s (+%d more)", portStrs[0], len(portStrs)-1)
	}

	return strings.Join(portStrs, ", ")
}
