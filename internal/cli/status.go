// internal/cli/status.go
package cli

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show status of LocalCloud services",
	Long:  `Display the current status and health of all LocalCloud services.`,
	RunE:  runStatus,
}

func runStatus(cmd *cobra.Command, args []string) error {
	// Check if project is initialized
	if !isProjectInitialized() {
		return fmt.Errorf("no LocalCloud project found")
	}

	// Create a tabwriter for aligned output
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)

	// Print header
	fmt.Println("LocalCloud Status")
	fmt.Println("─────────────────────────────────────────────")
	fmt.Fprintln(w, "SERVICE\tSTATUS\tHEALTH\tPORT\t")
	fmt.Fprintln(w, "───────\t──────\t──────\t────\t")

	// Service status (placeholder data for now)
	services := []struct {
		name   string
		status string
		health string
		port   string
	}{
		{"ai", "running", "✓", "11434"},
		{"postgres", "running", "✓", "5432"},
		{"redis", "running", "✓", "6379"},
		{"minio", "running", "✓", "9000"},
	}

	for _, s := range services {
		statusColor := successColor
		if s.status != "running" {
			statusColor = errorColor
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t\n",
			s.name,
			statusColor(s.status),
			s.health,
			s.port,
		)
	}

	w.Flush()
	fmt.Println("─────────────────────────────────────────────")
	fmt.Println()
	fmt.Println("Project: my-project")
	fmt.Println("Uptime: 2h 34m")
	fmt.Println("Memory: 3.2GB / 4.0GB (80%)")

	return nil
}