package cli

import (
	"fmt"
	"time"

	"github.com/briandowns/spinner"
	"github.com/spf13/cobra"
)

var stopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop all LocalCloud services",
	Long:  `Stop all running LocalCloud services for the current project.`,
	RunE:  runStop,
}

func runStop(cmd *cobra.Command, args []string) error {
	// Check if project is initialized
	if !isProjectInitialized() {
		return fmt.Errorf("no LocalCloud project found")
	}

	s := spinner.New(spinner.CharSets[14], 100*time.Millisecond)
	s.Suffix = " Stopping LocalCloud services..."
	s.Start()

	// Simulate service shutdown (will be implemented with Docker in Task 4-5)
	services := []string{"Ollama", "MinIO", "Redis", "PostgreSQL"}
	for _, service := range services {
		s.Suffix = fmt.Sprintf(" Stopping %s...", service)
		time.Sleep(500 * time.Millisecond)
	}

	s.Stop()

	printSuccess("All services stopped successfully!")
	return nil
}