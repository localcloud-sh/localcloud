// internal/cli/create.go
package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/briandowns/spinner"
	"github.com/localcloud/localcloud/internal/config"
	"github.com/spf13/cobra"
)

var (
	templateName string
)

var createCmd = &cobra.Command{
	Use:   "create [project-name]",
	Short: "Create a new project from a template",
	Long: `Create a new LocalCloud project from a template. 
Available templates: chat, rag, api`,
	Args: cobra.ExactArgs(1),
	RunE: runCreate,
}

func init() {
	createCmd.Flags().StringVarP(&templateName, "template", "t", "chat", "Template to use (chat, rag, api)")
}

func runCreate(cmd *cobra.Command, args []string) error {
	projectName := args[0]

	// Validate template
	validTemplates := []string{"chat", "rag", "api"}
	isValid := false
	for _, t := range validTemplates {
		if t == templateName {
			isValid = true
			break
		}
	}
	if !isValid {
		return fmt.Errorf("invalid template '%s'. Available templates: %s", templateName, strings.Join(validTemplates, ", "))
	}

	// Check if project already exists
	projectDir := filepath.Join(projectPath, projectName)
	if _, err := os.Stat(projectDir); !os.IsNotExist(err) {
		return fmt.Errorf("project '%s' already exists", projectName)
	}

	// Start creation process
	s := spinner.New(spinner.CharSets[14], 100*time.Millisecond)
	s.Suffix = fmt.Sprintf(" Creating project from template '%s'...", templateName)
	s.Start()

	// Create project directory
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		s.Stop()
		return fmt.Errorf("failed to create project directory: %w", err)
	}

	// Simulate template copying (will be implemented with actual templates in Task 11)
	time.Sleep(1 * time.Second)

	s.Suffix = " Installing dependencies..."
	time.Sleep(1 * time.Second)

	s.Suffix = " Configuring services..."
	time.Sleep(1 * time.Second)

	s.Stop()

	// Initialize the project
	if err := initProject(projectDir, projectName, templateName); err != nil {
		return fmt.Errorf("failed to initialize project: %w", err)
	}

	// Print success message
	printSuccess(fmt.Sprintf("Created project '%s' from template '%s'", projectName, templateName))
	fmt.Println()
	fmt.Println("Your project is ready! Next steps:")
	fmt.Println("  1. cd", projectDir)
	fmt.Println("  2. localcloud start")
	fmt.Println()

	if templateName == "chat" {
		fmt.Println("Your chat application will be available at:")
		fmt.Println("  • http://localhost:3000")
	}

	return nil
}

func initProject(projectDir, projectName, template string) error {
	// Create .localcloud directory
	configPath := filepath.Join(projectDir, ".localcloud")
	if err := os.MkdirAll(configPath, 0755); err != nil {
		return err
	}

	// Create template-specific config
	configFile := filepath.Join(configPath, "config.yaml")
	config := generateTemplateConfig(projectName, template)
	if err := os.WriteFile(configFile, []byte(config), 0644); err != nil {
		return err
	}

	// Create basic project structure based on template
	switch template {
	case "chat":
		// Create basic Next.js structure (placeholder for Task 11)
		dirs := []string{"src", "src/app", "src/components", "public"}
		for _, dir := range dirs {
			if err := os.MkdirAll(filepath.Join(projectDir, dir), 0755); err != nil {
				return err
			}
		}

		// Create placeholder files
		readme := `# ${projectName} - AI Chat Application

Built with LocalCloud - AI Development at Zero Cost

## Getting Started

` + "```bash\nlocalcloud start\n```" + `

Your application will be available at http://localhost:3000
`
		readme = strings.ReplaceAll(readme, "${projectName}", projectName)
		if err := os.WriteFile(filepath.Join(projectDir, "README.md"), []byte(readme), 0644); err != nil {
			return err
		}
	}

	return nil
}

func generateTemplateConfig(projectName, template string) string {
	return config.TemplateConfig(projectName, template)
}
