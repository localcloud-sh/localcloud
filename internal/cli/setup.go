// internal/cli/setup.go
package cli

import (
	"context"
	"embed"
	"fmt"
	"os"
	"path/filepath"

	"github.com/localcloud/localcloud/internal/system"
	"github.com/localcloud/localcloud/internal/templates"
	"github.com/spf13/cobra"
)

// templatesFS will be set from main.go
var templatesFS embed.FS

// SetupCmd creates the setup command
func SetupCmd(fs embed.FS) *cobra.Command {
	// Store the filesystem
	templatesFS = fs

	var options templates.SetupOptions

	cmd := &cobra.Command{
		Use:   "setup [template]",
		Short: "Set up a new project from a template",
		Long: `Set up a new LocalCloud project from a template.

Available templates:
  chat           - ChatGPT-like interface with conversation history
  code-assistant - AI-powered code editor and assistant
  transcribe     - Audio/video transcription service
  image-gen      - AI image generation interface
  api-only       - REST API without frontend

Examples:
  lc setup chat
  lc setup chat --name my-ai-chat
  lc setup api-only --port 8080 --skip-docker`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			templateName := args[0]
			return runSetup(cmd.Context(), templatesFS, templateName, options)
		},
	}

	// Add flags
	cmd.Flags().StringVar(&options.ProjectName, "name", "", "Project name (default: template name)")
	cmd.Flags().IntVar(&options.APIPort, "port", 0, "API port (default: auto)")
	cmd.Flags().IntVar(&options.FrontendPort, "frontend-port", 0, "Frontend port (default: auto)")
	cmd.Flags().StringVar(&options.ModelName, "model", "", "AI model to use (default: recommended)")
	cmd.Flags().BoolVar(&options.SkipDocker, "skip-docker", false, "Generate files only, don't start services")
	cmd.Flags().BoolVar(&options.Force, "force", false, "Overwrite existing directory")

	return cmd
}

func runSetup(ctx context.Context, templatesFS embed.FS, templateName string, options templates.SetupOptions) error {
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
	systemChecker := system.NewChecker(ctx)

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
	if err := wizard.Run(ctx, templateName, options); err != nil {
		return fmt.Errorf("setup failed: %w", err)
	}

	return nil
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
