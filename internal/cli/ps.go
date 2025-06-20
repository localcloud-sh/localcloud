// internal/cli/ps.go
package cli

import (
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

var psCmd = &cobra.Command{
	Use:   "ps",
	Short: "List running LocalCloud services",
	Long:  `Display a list of all running LocalCloud services with their status.`,
	RunE:  runPs,
}

func runPs(cmd *cobra.Command, args []string) error {
	// Check if project is initialized
	if !isProjectInitialized() {
		return fmt.Errorf("no LocalCloud project found")
	}

	// Create a tabwriter for aligned output
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)

	// Print header
	fmt.Fprintln(w, "NAME\tIMAGE\tSTATUS\tPORTS\tCREATED\t")
	fmt.Fprintln(w, "────\t─────\t──────\t─────\t───────\t")

	// Service list (placeholder data for now)
	services := []struct {
		name    string
		image   string
		status  string
		ports   string
		created string
	}{
		{
			name:    "localcloud-ai",
			image:   "ollama/ollama:latest",
			status:  "Up 2 hours",
			ports:   "0.0.0.0:11434->11434/tcp",
			created: "2 hours ago",
		},
		{
			name:    "localcloud-postgres",
			image:   "postgres:16-alpine",
			status:  "Up 2 hours",
			ports:   "0.0.0.0:5432->5432/tcp",
			created: "2 hours ago",
		},
		{
			name:    "localcloud-redis",
			image:   "redis:7-alpine",
			status:  "Up 2 hours",
			ports:   "0.0.0.0:6379->6379/tcp",
			created: "2 hours ago",
		},
		{
			name:    "localcloud-minio",
			image:   "minio/minio:latest",
			status:  "Up 2 hours",
			ports:   "0.0.0.0:9000->9000/tcp",
			created: "2 hours ago",
		},
	}

	for _, s := range services {
		statusColor := successColor
		if !strings.HasPrefix(s.status, "Up") {
			statusColor = errorColor
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t\n",
			s.name,
			s.image,
			statusColor(s.status),
			s.ports,
			s.created,
		)
	}

	w.Flush()

	return nil
}