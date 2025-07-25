// internal/cli/root.go
package cli

import (
	"embed"
	"fmt"
	"os"

	"github.com/fatih/color"
	"github.com/localcloud-sh/localcloud/internal/config"
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
	ServiceCmd   = serviceCmd
)

// rootCmd represents the base command
// Version information - set by main.go
var (
	version   = "dev"
	commit    = "unknown"
	buildDate = "unknown"
)

// SetVersionInfo sets the version information from build-time variables
func SetVersionInfo(v, c, d string) {
	version = v
	commit = c
	buildDate = d
	rootCmd.Version = v
}

var rootCmd = &cobra.Command{
	Use:     "localcloud",
	Aliases: []string{"lc"},
	Short:   "AI Development at Zero Cost",
	Long: `LocalCloud is an open-source, local-first AI development platform that 
eliminates cloud costs during development. Run AI models, databases, storage, 
and compute services entirely on your machine.

🤖 PERFECT FOR CODE ASSISTANTS (Claude Code, Cursor, Gemini CLI):
- Zero-cost local development environment
- One-command setup: 'lc setup my-project'  
- Instant AI model deployment with Ollama integration
- Local PostgreSQL, MongoDB, Redis, and MinIO storage
- Built-in export tools for seamless cloud migration

You can use either 'localcloud' or 'lc' to run commands.`,
	Version: version,
	Example: `  # Complete AI development setup
  lc setup my-ai-app
  cd my-ai-app
  lc start
  lc models pull llama3.2:3b
  
  # Check everything is running
  lc status
  
  # When ready for production
  lc export all --output=./production-backup/`,
}

// Execute adds all child commands to the root command and sets flags appropriately.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "%s %v\n", errorColor("Error:"), err)
		os.Exit(1)
	}
}

// internal/cli/root.go - init() function update

func init() {
	cobra.OnInitialize(initConfig)

	// Global flags
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Enable verbose output")
	rootCmd.PersistentFlags().StringVarP(&configFile, "config", "c", "", "Config file path (default: ./.localcloud/config.yaml)")
	rootCmd.PersistentFlags().StringVarP(&projectPath, "project", "p", ".", "Project directory path")

	// Add all subcommands
	rootCmd.AddCommand(setupCmd) // Setup replaces init - combines initialization and configuration
	rootCmd.AddCommand(startCmd)
	rootCmd.AddCommand(stopCmd)
	rootCmd.AddCommand(restartCmd) // Restart services with current configuration
	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(logsCmd)
	rootCmd.AddCommand(psCmd)
	rootCmd.AddCommand(modelsCmd)
	rootCmd.AddCommand(configCmd)
	rootCmd.AddCommand(storageCmd)
	rootCmd.AddCommand(infoCmd)
	rootCmd.AddCommand(tunnelCmd)
	rootCmd.AddCommand(debugCmd)
	rootCmd.AddCommand(doctorCmd)
	rootCmd.AddCommand(resetCmd)
	rootCmd.AddCommand(componentCmd)
	rootCmd.AddCommand(TemplatesCmd())
	// Database command is added in database.go init()
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

// InitializeTemplateFS sets the template filesystem for commands that need it
func InitializeTemplateFS(fs embed.FS) {
	// Set the filesystem for setup command
	SetTemplateFS(fs)
}
