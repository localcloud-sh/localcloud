// internal/cli/root.go
package cli

import (
	"fmt"
	"os"

	"github.com/fatih/color"
	"github.com/localcloud/localcloud/internal/config"
	"github.com/spf13/cobra"
)

var (
	// Global flags
	verbose     bool
	configFile  string
	projectPath string

	// Color helpers
	successColor = color.New(color.FgGreen).SprintFunc()
	errorColor   = color.New(color.FgRed).SprintFunc()
	warningColor = color.New(color.FgYellow).SprintFunc()
	infoColor    = color.New(color.FgCyan).SprintFunc()
)

// rootCmd represents the base command
var rootCmd = &cobra.Command{
	Use:     "localcloud",
	Aliases: []string{"lc"},
	Short:   "AI Development at Zero Cost",
	Long: `LocalCloud is an open-source, local-first AI development platform that 
eliminates cloud costs during development. Run AI models, databases, storage, 
and compute services entirely on your machine.

You can use either 'localcloud' or 'lc' to run commands.`,
	Version: "0.1.0",
	Example: `  localcloud init my-project
  lc start
  lc models list
  lc status`,
}

// Execute adds all child commands to the root command and sets flags appropriately.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "%s %v\n", errorColor("Error:"), err)
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	// Global flags
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Enable verbose output")
	rootCmd.PersistentFlags().StringVarP(&configFile, "config", "c", "", "Config file path (default: ./.localcloud/config.yaml)")
	rootCmd.PersistentFlags().StringVarP(&projectPath, "project", "p", ".", "Project directory path")

	// Add all subcommands
	rootCmd.AddCommand(initCmd)
	rootCmd.AddCommand(createCmd)
	rootCmd.AddCommand(startCmd)
	rootCmd.AddCommand(stopCmd)
	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(logsCmd)
	rootCmd.AddCommand(psCmd)
	rootCmd.AddCommand(modelsCmd)
	rootCmd.AddCommand(configCmd)
	rootCmd.AddCommand(storageCmd)
	rootCmd.AddCommand(infoCmd)   // NEW: Add info command
	rootCmd.AddCommand(tunnelCmd) // NEW: Add tunnel command
}

func initConfig() {
	// Config initialization will be implemented in Task 3
	if verbose {
		fmt.Println(infoColor("Debug mode enabled"))
	}

	// Initialize configuration
	if err := config.Init(configFile); err != nil {
		// Config errors are only fatal for commands that need config
		// init and create commands should work without existing config
		if verbose {
			fmt.Printf("Config initialization warning: %v\n", err)
		}
	}
}

// Helper functions for consistent output
func printSuccess(message string) {
	fmt.Println(successColor("✓"), message)
}

func printError(message string) {
	fmt.Println(errorColor("✗"), message)
}

func printWarning(message string) {
	fmt.Println(warningColor("!"), message)
}

func printInfo(message string) {
	fmt.Println(infoColor("ℹ"), message)
}
