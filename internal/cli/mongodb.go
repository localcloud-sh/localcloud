// internal/cli/mongodb.go
package cli

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"

	"github.com/localcloud-sh/localcloud/internal/config"
	"github.com/spf13/cobra"
)

var mongoCmd = &cobra.Command{
	Use:   "mongo",
	Short: "MongoDB management commands",
	Long:  `Manage MongoDB database including connections and operations.`,
}

var mongoConnectCmd = &cobra.Command{
	Use:   "connect",
	Short: "Connect to MongoDB",
	Long:  `Open an interactive MongoDB session using mongosh.`,
	RunE:  runMongoConnect,
}

func init() {
	// Add subcommands
	mongoCmd.AddCommand(mongoConnectCmd)

	// Add to root command
	rootCmd.AddCommand(mongoCmd)
}

func runMongoConnect(cmd *cobra.Command, args []string) error {
	// Check if project is initialized
	if !IsProjectInitialized() {
		return fmt.Errorf("no LocalCloud project found")
	}

	// Get config
	cfg := config.Get()

	if cfg.Services.MongoDB.Type == "" {
		return fmt.Errorf("MongoDB service not configured")
	}

	// Create connection string
	mongoURI := fmt.Sprintf("mongodb://localcloud:localcloud@localhost:%d/localcloud?authSource=admin",
		cfg.Services.MongoDB.Port)

	printInfo("Connecting to MongoDB...")
	printInfo(fmt.Sprintf("Connection URI: %s", mongoURI))

	// Try to use mongosh first, fallback to mongo
	err := executeMongoCommand("mongosh", mongoURI)
	if err != nil {
		printInfo("mongosh not found, trying legacy mongo client...")
		return executeMongoCommand("mongo", mongoURI)
	}
	return err
}

func executeMongoCommand(command, uri string) error {
	cmd := exec.Command(command, uri)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err := cmd.Run()
	if err != nil {
		// Check if command not found
		if exitError, ok := err.(*exec.ExitError); ok {
			if status, ok := exitError.Sys().(syscall.WaitStatus); ok && status.ExitStatus() == 127 {
				return fmt.Errorf("%s command not found. Please install MongoDB client tools", command)
			}
		}
		return fmt.Errorf("failed to connect to MongoDB: %w", err)
	}
	return nil
}
