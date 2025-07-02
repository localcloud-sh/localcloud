// internal/cli/config.go
package cli

import (
	"fmt"
	"github.com/localcloud-sh/localcloud/internal/config"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
	"os"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage LocalCloud configuration",
	Long:  `View and manage LocalCloud configuration settings.`,
}

var configShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Show current configuration",
	Long:  `Display the current LocalCloud configuration.`,
	RunE:  runConfigShow,
}

var configValidateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate configuration file",
	Long:  `Validate the LocalCloud configuration file for errors.`,
	RunE:  runConfigValidate,
}

var configGenerateCmd = &cobra.Command{
	Use:   "generate [project-name] [project-type]",
	Short: "Generate default configuration",
	Long:  `Generate a default LocalCloud configuration file.`,
	Example: `  lc config generate                    # Uses defaults
  lc config generate my-app              # Custom project name
  lc config generate my-app chat         # Custom name and type`,
	Args: cobra.MaximumNArgs(2),
	RunE: runConfigGenerate,
}

func init() {
	// Add subcommands
	configCmd.AddCommand(configShowCmd)
	configCmd.AddCommand(configValidateCmd)
	configCmd.AddCommand(configGenerateCmd)
}

func runConfigShow(cmd *cobra.Command, args []string) error {
	cfg := config.Get()

	// Marshal config to YAML for display
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("failed to format config: %w", err)
	}

	fmt.Println("Current Configuration:")
	fmt.Println(string(data))

	// Show config file location
	if configFile := config.GetViper().ConfigFileUsed(); configFile != "" {
		fmt.Printf("\nConfig file: %s\n", configFile)
	}

	return nil
}

func runConfigValidate(cmd *cobra.Command, args []string) error {
	// Config is already validated during Init
	// If we got here, it's valid
	printSuccess("Configuration is valid!")

	cfg := config.Get()
	fmt.Printf("\nProject: %s (type: %s)\n", cfg.Project.Name, cfg.Project.Type)
	fmt.Printf("Services: AI, ")

	services := []string{}
	if cfg.Services.Database.Type != "" {
		services = append(services, "Database")
	}
	if cfg.Services.Cache.Type != "" {
		services = append(services, "Cache")
	}
	if cfg.Services.Storage.Type != "" {
		services = append(services, "Storage")
	}

	if len(services) > 0 {
		fmt.Printf("Database, Cache")
		if cfg.Services.Storage.Type != "" {
			fmt.Printf(", Storage")
		}
	}
	fmt.Println()

	return nil
}

// internal/cli/config.go
// Fix the runConfigGenerate function

func runConfigGenerate(cmd *cobra.Command, args []string) error {
	// Check if config already exists
	if _, err := os.Stat(".localcloud/config.yaml"); err == nil {
		return fmt.Errorf("config file already exists at .localcloud/config.yaml")
	}

	// Create .localcloud directory
	if err := os.MkdirAll(".localcloud", 0755); err != nil {
		return fmt.Errorf("failed to create .localcloud directory: %w", err)
	}

	// Generate default config
	projectName := "my-project"
	projectType := "custom" // ADD THIS LINE - default project type

	if len(args) > 0 {
		projectName = args[0]
	}
	if len(args) > 1 {
		projectType = args[1] // Optional: second argument as project type
	}

	configContent, err := config.GenerateDefault(projectName, projectType)
	if err != nil {
		return fmt.Errorf("failed to generate config: %w", err)
	}

	// Write config file
	if err := os.WriteFile(".localcloud/config.yaml", configContent, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	printSuccess("Generated default configuration at .localcloud/config.yaml")
	return nil
}
