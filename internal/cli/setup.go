// internal/cli/setup.go
package cli

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"

	"github.com/localcloud/localcloud/internal/config"
	"github.com/localcloud/localcloud/internal/system"
	"github.com/localcloud/localcloud/internal/templates"
	"github.com/spf13/cobra"
)

// templatesFS will be set from main.go
var templatesFS embed.FS

var setupCmd = &cobra.Command{
	Use:   "setup [template]",
	Short: "Configure project or create from template",
	Long: `Configure your LocalCloud project interactively or create a new project from a template.

When run without arguments, it launches the interactive setup wizard for the current project.
When run with a template name, it creates a new project from that template.

Available templates:
  chat           - ChatGPT-like interface with conversation history
  code-assistant - AI-powered code editor and assistant
  transcribe     - Audio/video transcription service
  image-gen      - AI image generation interface
  api-only       - REST API without frontend`,
	Example: `  lc setup                   # Configure current project interactively
  lc setup chat              # Create new project from chat template
  lc setup api-only --port 8080`,
	Args: cobra.MaximumNArgs(1),
	RunE: runSetup,
}

func init() {
	// Template-specific flags
	setupCmd.Flags().String("name", "", "Project name (for template creation)")
	setupCmd.Flags().Int("port", 0, "API port (for template creation)")
	setupCmd.Flags().Int("frontend-port", 0, "Frontend port (for template creation)")
	setupCmd.Flags().String("model", "", "AI model to use (for template creation)")
	setupCmd.Flags().Bool("skip-docker", false, "Generate files only, don't start services")
	setupCmd.Flags().Bool("force", false, "Overwrite existing directory")
}

func runSetup(cmd *cobra.Command, args []string) error {
	// If no template specified, run interactive setup for current project
	if len(args) == 0 {
		return runInteractiveSetup(cmd)
	}

	// Otherwise, create from template
	return runTemplateSetup(cmd, args[0])
}

// runInteractiveSetup runs the component/model configuration wizard
func runInteractiveSetup(cmd *cobra.Command) error {
	// Check if project is initialized
	if !IsProjectInitialized() {
		return fmt.Errorf("no LocalCloud project found. Run 'lc init' first")
	}

	// Get config to extract project name
	cfg := config.Get()
	if cfg == nil {
		return fmt.Errorf("failed to load configuration")
	}

	projectName := cfg.Project.Name
	if projectName == "" {
		projectName = "my-project"
	}

	// Run the existing interactive init function
	return RunInteractiveInit(projectName)
}

// runTemplateSetup creates a new project from template
func runTemplateSetup(cmd *cobra.Command, templateName string) error {
	var options templates.SetupOptions

	// Get flags
	options.ProjectName, _ = cmd.Flags().GetString("name")
	options.APIPort, _ = cmd.Flags().GetInt("port")
	options.FrontendPort, _ = cmd.Flags().GetInt("frontend-port")
	options.ModelName, _ = cmd.Flags().GetString("model")
	options.SkipDocker, _ = cmd.Flags().GetBool("skip-docker")
	options.Force, _ = cmd.Flags().GetBool("force")

	// Initialize templates
	if err := templates.InitializeTemplates(templatesFS); err != nil {
		return fmt.Errorf("failed to initialize templates: %w", err)
	}

	// Get template
	template, err := templates.GetTemplate(templateName)
	if err != nil {
		return err
	}

	// Set default project name if not provided
	if options.ProjectName == "" {
		options.ProjectName = templateName

		// If current directory is empty, use it
		entries, _ := os.ReadDir(".")
		if len(entries) == 0 {
			options.ProjectName = "."
		}
	}

	// Get absolute path
	projectPath, err := filepath.Abs(options.ProjectName)
	if err != nil {
		return fmt.Errorf("invalid project path: %w", err)
	}

	// Update options with absolute path
	options.ProjectName = projectPath

	// Create system checker
	systemChecker := system.NewChecker(cmd.Context())

	// Create port manager
	portManager := templates.NewPortManager()

	// Create generator
	generator := templates.NewGenerator(templatesFS, "templates")

	// TODO: Create model manager (integrate with existing Ollama manager)
	var modelManager templates.ModelManager = nil

	// Create and run setup wizard
	wizard := templates.NewSetupWizard(
		template,
		systemChecker,
		portManager,
		modelManager,
		generator,
	)

	// Run setup
	if err := wizard.Run(cmd.Context(), templateName, options); err != nil {
		return fmt.Errorf("setup failed: %w", err)
	}

	return nil
}

// SetupCmd creates the setup command (for external initialization)
func SetupCmd(fs embed.FS) *cobra.Command {
	// Store the filesystem
	templatesFS = fs
	return setupCmd
}

// TemplatesCmd creates the templates command
func TemplatesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "templates",
		Short: "Manage LocalCloud templates",
		Long:  "List and get information about available LocalCloud templates.",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			// Initialize templates before any subcommand runs
			return templates.InitializeTemplates(templatesFS)
		},
	}

	// Add subcommands
	cmd.AddCommand(templatesListCmd())
	cmd.AddCommand(templatesInfoCmd())

	return cmd
}

func templatesListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List available templates",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Initialize templates first
			if err := templates.InitializeTemplates(templatesFS); err != nil {
				return fmt.Errorf("failed to initialize templates: %w", err)
			}

			templateList := templates.ListTemplates()

			if len(templateList) == 0 {
				fmt.Println("No templates available")
				return nil
			}

			fmt.Println("Available Templates:")
			fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

			for _, tmpl := range templateList {
				fmt.Printf("%-15s %s\n", tmpl.Name, tmpl.Description)
				fmt.Printf("               Min RAM: %s, Services: %d\n\n",
					tmpl.MinRAM, tmpl.Services)
			}

			return nil
		},
	}
}

func templatesInfoCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "info [template]",
		Short: "Get detailed information about a template",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			templateName := args[0]

			template, err := templates.GetTemplate(templateName)
			if err != nil {
				return err
			}

			metadata := template.GetMetadata()

			fmt.Printf("%s Template\n", metadata.Name)
			fmt.Println("═══════════════════════════════════")
			fmt.Printf("Description: %s\n", metadata.Description)
			fmt.Printf("Version: %s\n", metadata.Version)
			fmt.Println()
			fmt.Println("Requirements:")
			fmt.Printf("- RAM: %s minimum\n", system.FormatBytes(metadata.MinRAM))
			fmt.Printf("- Disk: %s free space\n", system.FormatBytes(metadata.MinDisk))
			fmt.Println("- Docker: Required")
			fmt.Println()
			fmt.Println("Services:")
			for _, service := range metadata.Services {
				fmt.Printf("- %s\n", service)
			}
			fmt.Println()
			fmt.Println("Models:")
			for _, model := range metadata.Models {
				status := ""
				if model.Default {
					status = " (default)"
				} else if model.Recommended {
					status = " (recommended)"
				}
				fmt.Printf("- %s: %s, needs %s RAM%s\n",
					model.Name,
					system.FormatBytes(model.Size),
					system.FormatBytes(model.MinRAM),
					status)
			}

			return nil
		},
	}
}
