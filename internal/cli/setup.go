// internal/cli/setup.go
package cli

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/fatih/color"
	"github.com/localcloud-sh/localcloud/internal/components"
	"github.com/localcloud-sh/localcloud/internal/config"
	"github.com/localcloud-sh/localcloud/internal/models"
	"github.com/localcloud-sh/localcloud/internal/system"
	"github.com/localcloud-sh/localcloud/internal/templates"
	"github.com/spf13/cobra"
)

// templatesFS will be set from main.go
var templatesFS embed.FS

var setupCmd = &cobra.Command{
	Use:   "setup [project-name]",
	Short: "Initialize and configure LocalCloud project",
	Long: `Initialize a new LocalCloud project or configure an existing one.

This command combines project initialization and component configuration:
- New project: Creates project structure and configures components
- Existing project: Modifies current component configuration
- With flags: Add or remove specific components`,
	Example: `  lc setup                   # Setup in current directory
  lc setup my-project        # Create and setup new project
  lc setup --add llm         # Add component to existing project
  lc setup --remove cache    # Remove component from project`,
	Args: cobra.MaximumNArgs(1),
	RunE: runSetup,
}

var (
	setupAdd    []string
	setupRemove []string
)

func init() {
	setupCmd.Flags().StringSliceVar(&setupAdd, "add", []string{}, "Components to add")
	setupCmd.Flags().StringSliceVar(&setupRemove, "remove", []string{}, "Components to remove")
}

func runSetup(cmd *cobra.Command, args []string) error {
	var projectName string
	var projectDir string
	isNewProject := false

	// Determine project name and directory
	if len(args) > 0 {
		// Project name provided - create new project
		projectName = args[0]
		projectDir = filepath.Join(projectPath, projectName)
		isNewProject = true
	} else {
		// No project name - use current directory
		projectDir = projectPath
		projectName = filepath.Base(projectDir)
	}

	// Check if this is an existing project or we need to initialize
	if !IsProjectInitialized() {
		// For --add/--remove flags, project must exist
		if len(setupAdd) > 0 || len(setupRemove) > 0 {
			printError("No LocalCloud project found in current directory")
			fmt.Println("\nTo create a new project, run:")
			fmt.Printf("  %s\n", infoColor("lc setup"))
			return fmt.Errorf("project not initialized")
		}

		// Initialize new project
		if err := initializeProject(projectName, projectDir, isNewProject); err != nil {
			return err
		}

		// Change to project directory if we created one
		if isNewProject {
			originalDir, _ := os.Getwd()
			if err := os.Chdir(projectDir); err != nil {
				return fmt.Errorf("failed to change directory: %w", err)
			}
			defer os.Chdir(originalDir)
		}

		// Reload config after initialization
		if err := config.Init(""); err != nil {
			return fmt.Errorf("failed to initialize config: %w", err)
		}
	}

	// Handle --add and --remove flags
	if len(setupAdd) > 0 || len(setupRemove) > 0 {
		return handleComponentModification(setupAdd, setupRemove)
	}

	// Load current configuration
	cfg := config.Get()
	if cfg == nil {
		return fmt.Errorf("failed to load configuration")
	}

	// Update project name in config if needed
	if cfg.Project.Name == "" || cfg.Project.Name == "my-project" {
		cfg.Project.Name = projectName
	}

	// Determine setup mode based on current state
	existingComponents := getConfiguredComponents(cfg)

	if len(existingComponents) == 0 {
		// Empty config - run full setup
		fmt.Println(color.New(color.FgCyan, color.Bold).Sprint("🚀 LocalCloud Project Setup"))
		fmt.Println(strings.Repeat("━", 60))
		fmt.Println("\nLet's configure your LocalCloud project!")
		fmt.Println()

		return runFullSetup(cfg)
	} else {
		// Has components - run modification setup
		fmt.Println(color.New(color.FgCyan, color.Bold).Sprint("🔧 LocalCloud Project Configuration"))
		fmt.Println(strings.Repeat("━", 60))
		fmt.Printf("\nCurrent components: %s\n", strings.Join(existingComponents, ", "))
		fmt.Println()

		return runModificationSetup(cfg, existingComponents)
	}
}

// getConfiguredComponents returns list of configured component IDs
func getConfiguredComponents(cfg *config.Config) []string {
	var components []string

	// Check AI service
	if cfg.Services.AI.Port > 0 {
		// Check for LLM models
		for _, model := range cfg.Services.AI.Models {
			if !models.IsEmbeddingModel(model) {
				components = appendUnique(components, "llm")
				break
			}
		}

		// Check for embedding models
		for _, model := range cfg.Services.AI.Models {
			if models.IsEmbeddingModel(model) {
				components = appendUnique(components, "embedding")
				break
			}
		}
	}

	// Check other services
	if cfg.Services.Database.Port > 0 {
		// Check if pgvector extension is enabled
		for _, ext := range cfg.Services.Database.Extensions {
			if ext == "pgvector" {
				components = append(components, "vector")
				break
			}
		}
	}

	if cfg.Services.Cache.Port > 0 {
		components = append(components, "cache")
	}

	if cfg.Services.Queue.Port > 0 {
		components = append(components, "queue")
	}

	if cfg.Services.Storage.Port > 0 {
		components = append(components, "storage")
	}

	if cfg.Services.Whisper.Port > 0 {
		components = append(components, "stt")
	}

	return components
}

// runFullSetup runs the complete setup wizard for empty projects
func runFullSetup(cfg *config.Config) error {
	// 1. Project type selection (function from init_interactive.go)
	projectType, err := selectProjectType()
	if err != nil {
		return err
	}

	// 2. Component selection based on type (function from init_interactive.go)
	selectedComponents, err := selectComponents(projectType)
	if err != nil {
		return err
	}

	// 3. Model selection for AI components (function from init_interactive.go)
	selectedModels, err := selectModels(selectedComponents)
	if err != nil {
		return err
	}

	// 4. Update configuration (function from init_interactive.go)
	updateConfig(cfg, selectedComponents, selectedModels)

	// 5. Save configuration - config.Save() doesn't take parameters
	if err := config.Save(); err != nil {
		return fmt.Errorf("failed to save configuration: %w", err)
	}

	// 6. Show summary
	showSetupSummary(selectedComponents, selectedModels)

	return nil
}

// runModificationSetup allows adding/removing components from existing setup
func runModificationSetup(cfg *config.Config, existingComponents []string) error {
	// Create component options with existing ones pre-selected
	allComponents := []string{"llm", "embedding", "vector", "cache", "queue", "storage", "stt"}

	var options []string
	var defaults []string
	componentMap := make(map[string]string)

	for _, compID := range allComponents {
		comp, err := components.GetComponent(compID)
		if err != nil {
			continue
		}

		isExisting := contains(existingComponents, compID)
		var option string

		if isExisting {
			// Show with X for existing components
			option = fmt.Sprintf("[X] %s - %s", comp.Name, comp.Description)
			defaults = append(defaults, option)
		} else {
			option = fmt.Sprintf("[ ] %s - %s", comp.Name, comp.Description)
		}

		options = append(options, option)
		componentMap[option] = compID
	}

	// Multi-select prompt with existing components pre-selected
	prompt := &survey.MultiSelect{
		Message:  "Select components (Space to toggle, Enter to confirm):",
		Options:  options,
		Default:  defaults,
		PageSize: 10,
	}

	var selected []string
	if err := survey.AskOne(prompt, &selected); err != nil {
		return err
	}

	// Convert selections back to component IDs
	newComponents := []string{}
	for _, sel := range selected {
		if compID, ok := componentMap[sel]; ok {
			newComponents = append(newComponents, compID)
		}
	}

	// Determine what changed
	added := difference(newComponents, existingComponents)
	removed := difference(existingComponents, newComponents)

	// Handle removals
	if len(removed) > 0 {
		fmt.Printf("\n%s Removing components: %s\n", warningColor("⚠"), strings.Join(removed, ", "))

		// Confirm removal
		var confirm bool
		confirmPrompt := &survey.Confirm{
			Message: "Are you sure you want to remove these components?",
			Default: true,
		}
		if err := survey.AskOne(confirmPrompt, &confirm); err != nil {
			return err
		}

		if !confirm {
			return fmt.Errorf("cancelled")
		}

		// Remove components from config (function from init_interactive.go)
		removeComponentsFromConfig(cfg, removed)
	}

	// Handle additions
	if len(added) > 0 {
		fmt.Printf("\n%s Adding components: %s\n", successColor("✓"), strings.Join(added, ", "))

		// Select models for new AI components
		selectedModels := make(map[string]string)
		manager := models.NewManager("http://localhost:11434")

		for _, compID := range added {
			if compID == "llm" || compID == "embedding" || compID == "stt" {
				comp, err := components.GetComponent(compID)
				if err != nil {
					continue
				}

				var model string
				if compID == "embedding" {
					model, err = selectEmbeddingModel(manager)
				} else {
					model, err = selectComponentModel(comp, manager)
				}

				if err != nil {
					return err
				}
				selectedModels[compID] = model
			}
		}

		// Update config with new components (function from init_interactive.go)
		updateConfig(cfg, added, selectedModels)
	}

	// Save configuration
	if err := config.Save(); err != nil {
		return fmt.Errorf("failed to save configuration: %w", err)
	}

	// Show summary
	if len(added) > 0 || len(removed) > 0 {
		fmt.Println()
		printSuccess("Configuration updated!")

		if len(added) > 0 {
			fmt.Printf("  Added:   %s\n", strings.Join(added, ", "))
		}
		if len(removed) > 0 {
			fmt.Printf("  Removed: %s\n", strings.Join(removed, ", "))
		}

		fmt.Println("\nNext step:")
		fmt.Println("  lc restart    # Restart services with new configuration")
	} else {
		fmt.Println("\nNo changes made.")
	}

	return nil
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

// initializeProject creates a new LocalCloud project structure
func initializeProject(projectName, projectDir string, createDir bool) error {
	// Create project directory if needed
	if createDir {
		if err := os.MkdirAll(projectDir, 0755); err != nil {
			return fmt.Errorf("failed to create project directory: %w", err)
		}
		fmt.Printf("%s Created project directory: %s\n", successColor("✓"), projectName)
	}

	// Create .localcloud directory
	configPath := filepath.Join(projectDir, ".localcloud")
	if err := os.MkdirAll(configPath, 0755); err != nil {
		return fmt.Errorf("failed to create .localcloud directory: %w", err)
	}

	// Create initial config file
	configFile := filepath.Join(configPath, "config.yaml")
	configContent, err := config.GenerateDefault(projectName, "custom")
	if err != nil {
		return fmt.Errorf("failed to generate config: %w", err)
	}

	if err := os.WriteFile(configFile, configContent, 0644); err != nil {
		return fmt.Errorf("failed to create config file: %w", err)
	}

	// Create .gitignore
	gitignore := filepath.Join(projectDir, ".gitignore")
	gitignoreContent := `.localcloud/data/
.localcloud/logs/
.localcloud/tunnels/
.env.local
*.log
`
	if err := os.WriteFile(gitignore, []byte(gitignoreContent), 0644); err != nil {
		return fmt.Errorf("failed to create .gitignore: %w", err)
	}

	fmt.Printf("%s Initialized LocalCloud project structure\n", successColor("✓"))
	return nil
}

// Helper functions - only add functions not already defined elsewhere
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

func difference(a, b []string) []string {
	mb := make(map[string]bool)
	for _, x := range b {
		mb[x] = true
	}

	var diff []string
	for _, x := range a {
		if !mb[x] {
			diff = append(diff, x)
		}
	}
	return diff
}

// showSetupSummary displays what was configured
func showSetupSummary(componentIDs []string, models map[string]string) {
	fmt.Println()
	printSuccess("Configuration complete!")
	fmt.Println("\nYour project includes:")

	for _, compID := range componentIDs {
		comp, err := components.GetComponent(compID)
		if err != nil {
			continue
		}

		fmt.Printf("  • %s", comp.Name)
		if model, ok := models[compID]; ok {
			fmt.Printf(" (%s)", model)
		}
		fmt.Println()
	}

	fmt.Println("\nNext step:")
	fmt.Println("  lc start    # Start all services")
}

// SetTemplateFS sets the template filesystem
func SetTemplateFS(fs embed.FS) {
	templatesFS = fs
}

// TemplatesCmd creates the templates command
func TemplatesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "templates",
		Short: "Manage LocalCloud templates",
		Long:  "List and get information about available LocalCloud templates.",
	}

	cmd.AddCommand(templatesListCmd())
	cmd.AddCommand(templatesInfoCmd())

	return cmd
}

func templatesListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List available templates",
		RunE: func(cmd *cobra.Command, args []string) error {
			templates := templates.ListTemplates()

			fmt.Println("Available Templates:")
			fmt.Println(strings.Repeat("─", 70))

			for _, t := range templates {
				fmt.Printf("%-20s %s\n", t.Name, t.Description)
			}

			return nil
		},
	}
}

func templatesInfoCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "info [template]",
		Short: "Show template details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			template, err := templates.GetTemplate(args[0])
			if err != nil {
				return err
			}

			metadata := template.GetMetadata()
			fmt.Printf("Template: %s\n", metadata.Name)
			fmt.Printf("Description: %s\n", metadata.Description)
			fmt.Printf("Version: %s\n", metadata.Version)
			fmt.Printf("Min RAM: %d GB\n", metadata.MinRAM/(1024*1024*1024))
			fmt.Printf("Services: %v\n", metadata.Services)

			return nil
		},
	}
}
