// internal/cli/logs.go
package cli

import (
	"fmt"
	"strings"

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
	Args:    cobra.MaximumNArgs(1),
	RunE:    runLogs,
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

	service := ""
	if len(args) > 0 {
		service = args[0]
		// Validate service name
		validServices := []string{"ai", "postgres", "redis", "minio"}
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

	if service != "" {
		fmt.Printf("Showing logs for %s service (last %d lines):\n", service, tailLines)
	} else {
		fmt.Printf("Showing logs for all services (last %d lines):\n", tailLines)
	}
	fmt.Println(strings.Repeat("─", 60))

	// Simulate log output (will be implemented with Docker in Task 4)
	if followLogs {
		fmt.Println("Following logs... (press Ctrl+C to stop)")
		fmt.Println()
		// In real implementation, this would stream logs
		fmt.Println("[postgres] 2024-12-10 10:15:23 INFO: Database ready to accept connections")
		fmt.Println("[redis] 2024-12-10 10:15:24 INFO: Server started, Redis version 7.0.0")
		fmt.Println("[ai] 2024-12-10 10:15:25 INFO: Ollama server listening on :11434")
	} else {
		// Show static logs
		fmt.Println("[postgres] 2024-12-10 10:15:23 INFO: Database ready to accept connections")
		fmt.Println("[redis] 2024-12-10 10:15:24 INFO: Server started, Redis version 7.0.0")
		fmt.Println("[ai] 2024-12-10 10:15:25 INFO: Ollama server listening on :11434")
		fmt.Println("[minio] 2024-12-10 10:15:26 INFO: MinIO Object Storage Server")
	}

	return nil
}
