// internal/cli/init.go
package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/briandowns/spinner"
	"github.com/localcloud/localcloud/internal/config"
	"github.com/spf13/cobra"
	"time"
)

var initCmd = &cobra.Command{
	Use:   "init [project-name]",
	Short: "Initialize a new LocalCloud project",
	Long:  `Initialize a new LocalCloud project in the current directory or with the specified name.`,
	Args:  cobra.MaximumNArgs(1),
	RunE:  runInit,
}

func runInit(cmd *cobra.Command, args []string) error {
	projectName := "my-project"
	if len(args) > 0 {
		projectName = args[0]
	}

	// Create project directory if specified
	projectDir := projectPath
	if len(args) > 0 {
		projectDir = filepath.Join(projectPath, projectName)
		if err := os.MkdirAll(projectDir, 0755); err != nil {
			return fmt.Errorf("failed to create project directory: %w", err)
		}
	}

	// Check if already initialized
	configPath := filepath.Join(projectDir, ".localcloud")
	if _, err := os.Stat(configPath); !os.IsNotExist(err) {
		return fmt.Errorf("project already initialized in %s", projectDir)
	}

	// Start initialization
	s := spinner.New(spinner.CharSets[14], 100*time.Millisecond)
	s.Suffix = " Initializing LocalCloud project..."
	s.Start()

	// Create .localcloud directory
	if err := os.MkdirAll(configPath, 0755); err != nil {
		s.Stop()
		return fmt.Errorf("failed to create .localcloud directory: %w", err)
	}

	// Create config file (will be properly implemented in Task 3)
	configFile := filepath.Join(configPath, "config.yaml")
	configContent := config.GenerateDefault(projectName, "general")
	if err := os.WriteFile(configFile, []byte(configContent), 0644); err != nil {
		s.Stop()
		return fmt.Errorf("failed to create config file: %w", err)
	}

	// Create .gitignore
	gitignore := filepath.Join(projectDir, ".gitignore")
	gitignoreContent := `.localcloud/data/
.localcloud/logs/
.env.local
*.log
`
	if err := os.WriteFile(gitignore, []byte(gitignoreContent), 0644); err != nil {
		s.Stop()
		return fmt.Errorf("failed to create .gitignore: %w", err)
	}

	s.Stop()

	// Print success message
	printSuccess(fmt.Sprintf("Initialized LocalCloud project: %s", projectName))
	fmt.Println()
	fmt.Println("Next steps:")
	fmt.Println("  1. cd", projectDir)
	fmt.Println("  2. localcloud start")
	fmt.Println()
	fmt.Println("For more information, run: localcloud --help")

	return nil
}
