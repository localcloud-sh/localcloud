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
	Example: `  # Interactive setup (human users)
  lc setup                   # Setup in current directory
  lc setup my-project        # Create and setup new project
  
  # Non-interactive setup (perfect for code assistants)
  lc setup my-ai-app --preset=ai-dev --yes
  lc setup --components=llm,database --models=llama3.2:3b --yes
  lc setup my-app --components=database,cache,storage --yes
  
  # Modify existing projects
  lc setup --add llm         # Add component to existing project
  lc setup --remove cache    # Remove component from project`,
	Args: cobra.MaximumNArgs(1),
	RunE: runSetup,
}

var (
	setupAdd        []string
	setupRemove     []string
	setupComponents []string
	setupModels     []string
	setupPreset     string
	setupYes        bool
)

func init() {
	setupCmd.Flags().StringSliceVar(&setupAdd, "add", []string{}, "Components to add")
	setupCmd.Flags().StringSliceVar(&setupRemove, "remove", []string{}, "Components to remove")

	// Non-interactive flags for code assistants
	setupCmd.Flags().StringSliceVar(&setupComponents, "components", []string{}, "Components to configure (llm,database,cache,storage,etc)")
	setupCmd.Flags().StringSliceVar(&setupModels, "models", []string{}, "AI models to download (llama3.2:3b,nomic-embed-text)")
	setupCmd.Flags().StringVar(&setupPreset, "preset", "", "Preset configuration (ai-dev,full-stack,minimal)")
	setupCmd.Flags().BoolVarP(&setupYes, "yes", "y", false, "Accept all defaults (non-interactive mode)")
}

func runSetup(cmd *cobra.Command, args []string) error {
	// Check if non-interactive mode is requested
	isNonInteractive := setupYes || len(setupComponents) > 0 || len(setupModels) > 0 || setupPreset != ""

	if len(args) > 0 {
		// New project setup
		if isNonInteractive {
			return runNonInteractiveSetup(cmd, args[0])
		}
		return runNewProjectSetup(cmd, args[0])
	} else {
		// Existing project setup
		if isNonInteractive {
			return runNonInteractiveSetup(cmd, ".")
		}
		return runExistingProjectSetup(cmd)
	}
}

func runNewProjectSetup(cmd *cobra.Command, projectName string) error {
	projectDir, err := filepath.Abs(projectName)
	if err != nil {
		return fmt.Errorf("invalid project path: %w", err)
	}

	// Check if directory already exists
	if _, err := os.Stat(projectDir); !os.IsNotExist(err) {
		return fmt.Errorf("project directory '%s' already exists", projectName)
	}

	// Create project directory
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		return fmt.Errorf("failed to create project directory: %w", err)
	}

	// Change to project directory
	originalDir, _ := os.Getwd()
	if err := os.Chdir(projectDir); err != nil {
		return fmt.Errorf("failed to change directory: %w", err)
	}
	defer os.Chdir(originalDir)

	// Initialize a new, empty config for this project
	if err := config.Init(""); err != nil {
		return fmt.Errorf("failed to initialize config: %w", err)
	}

	// Initialize project structure (.localcloud, .gitignore)
	if err := initializeProject(projectName, ".", true); err != nil {
		return err
	}

	// Reload config after initialization
	if err := config.Init(""); err != nil {
		return fmt.Errorf("failed to reload config: %w", err)
	}

	cfg := config.Get()
	cfg.Project.Name = projectName

	// Run full setup wizard
	fmt.Println(color.New(color.FgCyan, color.Bold).Sprint("🚀 LocalCloud Project Setup"))
	fmt.Println(strings.Repeat("━", 60))
	fmt.Println("\nLet's configure your new LocalCloud project!")
	fmt.Println()

	return runFullSetup(cfg)
}

func runExistingProjectSetup(cmd *cobra.Command) error {
	// Initialize config from current directory
	if err := config.Init(""); err != nil {
		return fmt.Errorf("failed to initialize config: %w", err)
	}

	// Handle --add and --remove flags
	if len(setupAdd) > 0 || len(setupRemove) > 0 {
		return handleComponentModification(setupAdd, setupRemove)
	}

	cfg := config.Get()
	if cfg == nil {
		return fmt.Errorf("failed to load configuration")
	}

	// If no project name, use current directory name
	if cfg.Project.Name == "" {
		cwd, _ := os.Getwd()
		cfg.Project.Name = filepath.Base(cwd)
	}

	// Determine setup mode
	existingComponents := getConfiguredComponents(cfg)

	if !IsProjectInitialized() {
		// Not a project yet, run full setup
		fmt.Println(color.New(color.FgCyan, color.Bold).Sprint("🚀 LocalCloud Project Setup"))
		fmt.Println(strings.Repeat("━", 60))
		fmt.Println("\nLet's configure your LocalCloud project!")
		fmt.Println()

		// Create .localcloud dir and initial config
		if err := initializeProject(cfg.Project.Name, ".", false); err != nil {
			return err
		}
		// Reload config
		if err := config.Init(""); err != nil {
			return err
		}
		cfg = config.Get()

		return runFullSetup(cfg)
	} else {
		// Existing project, run modification setup
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

	if cfg == nil {
		return components
	}

	// PRIORITY 1: Use project.components field if available (this is the source of truth)
	if len(cfg.Project.Components) > 0 {
		return cfg.Project.Components
	}

	// FALLBACK: Check service configurations (for backward compatibility with old configs)
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

	// Check database services
	if cfg.Services.Database.Type != "" {
		components = append(components, "database")

		// Check if pgvector extension is enabled
		for _, ext := range cfg.Services.Database.Extensions {
			if ext == "pgvector" {
				components = append(components, "vector")
				break
			}
		}
	}

	if cfg.Services.Cache.Type != "" {
		components = append(components, "cache")
	}

	if cfg.Services.Queue.Type != "" {
		components = append(components, "queue")
	}

	if cfg.Services.MongoDB.Type != "" {
		components = append(components, "mongodb")
	}

	if cfg.Services.Storage.Type != "" {
		components = append(components, "storage")
	}

	if cfg.Services.Whisper.Type != "" {
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

	// 4.5. Ensure complete config cleanup and component tracking
	updateCompleteConfig(cfg, selectedComponents)

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
	allComponents := []string{"llm", "embedding", "database", "vector", "mongodb", "cache", "queue", "storage", "stt"}

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

	// Validate dependencies
	if err := validateComponentDependencies(newComponents); err != nil {
		return err
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

	// IMPORTANT: After handling additions and removals, update the entire configuration
	// to reflect the final component state and clear any stale configurations
	updateCompleteConfig(cfg, newComponents)

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
		fmt.Println("  lc restart    # Apply configuration changes by restarting services")
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
		if projectDir != "." {
			if err := os.MkdirAll(projectDir, 0755); err != nil {
				return fmt.Errorf("failed to create project directory: %w", err)
			}
		}
		fmt.Printf("%s Created project directory: %s\n", successColor("✓"), projectName)
	}

	// Create .localcloud directory
	configPath := filepath.Join(projectDir, ".localcloud")
	if err := os.MkdirAll(configPath, 0755); err != nil {
		return fmt.Errorf("failed to create .localcloud directory: %w", err)
	}

	// Create initial config file in .localcloud
	configFile := filepath.Join(configPath, "config.yaml")
	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		configContent, err := config.GenerateDefault(projectName, "custom")
		if err != nil {
			return fmt.Errorf("failed to generate config: %w", err)
		}

		if err := os.WriteFile(configFile, configContent, 0644); err != nil {
			return fmt.Errorf("failed to create config file: %w", err)
		}
	}

	// Create .gitignore
	gitignore := filepath.Join(projectDir, ".gitignore")
	if _, err := os.Stat(gitignore); os.IsNotExist(err) {
		gitignoreContent := `.localcloud/data/
.localcloud/logs/
.localcloud/tunnels/
.env.local
*.log
`
		if err := os.WriteFile(gitignore, []byte(gitignoreContent), 0644); err != nil {
			return fmt.Errorf("failed to create .gitignore: %w", err)
		}
	}

	// Create CLAUDE.md for code assistants
	claudeFile := filepath.Join(projectDir, "CLAUDE.md")
	if _, err := os.Stat(claudeFile); os.IsNotExist(err) {
		claudeContent := generateClaudeContent(projectName)
		if err := os.WriteFile(claudeFile, []byte(claudeContent), 0644); err != nil {
			return fmt.Errorf("failed to create CLAUDE.md: %w", err)
		}
	}

	fmt.Printf("%s Initialized LocalCloud project structure\n", successColor("✓"))
	return nil
}

// generateClaudeContent creates CLAUDE.md content for code assistants
func generateClaudeContent(projectName string) string {
	return fmt.Sprintf("# %s - LocalCloud Project\n\n## 🤖 AI Assistant Guide\n\nThis is a LocalCloud project for local AI development. LocalCloud eliminates cloud costs during development by running everything locally.\n\n## Available Commands\n\n### Core Commands\n- **`lc start`** - Start all configured services\n- **`lc status`** - Check service status and connection info\n- **`lc stop`** - Stop all services\n- **`lc restart`** - Restart services\n\n### AI Models\n- **`lc models list`** - List available models\n- **`lc models pull llama3.2:3b`** - Download AI models\n- **`lc models remove <model>`** - Remove models\n\n### Data Export (for production)\n- **`lc export all`** - Export all data for cloud migration\n- **`lc export db`** - Export PostgreSQL database\n- **`lc export storage`** - Export MinIO storage\n- **`lc export mongo`** - Export MongoDB (if configured)\n\n### Project Management\n- **`lc setup`** - Configure project components\n- **`lc info`** - Show project and system information\n- **`lc doctor`** - Check system requirements\n\n## Typical Development Workflow\n\n1. **Start services**: `lc start`\n2. **Download AI model**: `lc models pull llama3.2:3b`\n3. **Check status**: `lc status` (shows connection endpoints)\n4. **Develop your application** using the local services\n5. **Export for production**: `lc export all --output=./production-backup/`\n\n## Project Configuration\n\nRun `lc status` to see which services are configured and their connection details.\nCommon services include:\n- **PostgreSQL** (local database)\n- **AI Models** via Ollama\n- **MinIO** (S3-compatible storage)\n- **Redis** (caching)\n- **MongoDB** (document database)\n\n## Notes for AI Assistants\n\n- This project uses LocalCloud for local development\n- All services run locally (no cloud costs during development)\n- Use `lc export` commands when ready to migrate to production\n- Connection details are shown in `lc status` output\n- Always start services with `lc start` before development\n", projectName)
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

// validateComponentDependencies validates that component dependencies are satisfied
func validateComponentDependencies(componentIDs []string) error {
	// Check if vector is selected without database
	hasVector := contains(componentIDs, "vector")
	hasDatabase := contains(componentIDs, "database")

	if hasVector && !hasDatabase {
		fmt.Printf("\n%s Vector Search requires Database (PostgreSQL) to be selected.\n", errorColor("Error:"))
		fmt.Println("Vector Search is implemented as a PostgreSQL extension (pgvector).")
		return fmt.Errorf("dependency validation failed")
	}

	return nil
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

func appendUnique(slice []string, item string) []string {
	for _, existing := range slice {
		if existing == item {
			return slice
		}
	}
	return append(slice, item)
}

// internal/cli/setup.go
// Fix for duplicate case issue - locate the updateCompleteConfig function
// and remove the duplicate "stt" case. The function should have only one switch statement
// Here's the corrected section of updateCompleteConfig:

func updateCompleteConfig(cfg *config.Config, componentIDs []string) {
	// Set the project components field
	cfg.Project.Components = componentIDs

	// Create a map for quick lookup
	enabledComponents := make(map[string]bool)
	for _, comp := range componentIDs {
		enabledComponents[comp] = true
	}

	// Store AI models if they exist (to preserve them when updating AI services)
	var existingAIModels []string
	var existingAIDefault string
	if cfg.Services.AI.Port > 0 {
		existingAIModels = cfg.Services.AI.Models
		existingAIDefault = cfg.Services.AI.Default
	}

	// Clear ALL service configurations - start fresh
	cfg.Services = config.ServicesConfig{
		AI:       config.AIConfig{},
		Database: config.DatabaseConfig{},
		MongoDB:  config.MongoDBConfig{},
		Cache:    config.CacheConfig{},
		Queue:    config.QueueConfig{},
		Storage:  config.StorageConfig{},
		Whisper:  config.WhisperConfig{},
	}

	// Re-add only the services for enabled components
	for _, compID := range componentIDs {
		switch compID {
		case "llm", "embedding":
			// Initialize AI service if needed
			if cfg.Services.AI.Port == 0 {
				cfg.Services.AI = config.AIConfig{
					Port:    11434,
					Models:  []string{},
					Default: "",
				}
			}
			// Restore existing models if they match the component type
			for _, model := range existingAIModels {
				isEmbedding := models.IsEmbeddingModel(model)
				if (compID == "embedding" && isEmbedding) ||
					(compID == "llm" && !isEmbedding) {
					cfg.Services.AI.Models = append(cfg.Services.AI.Models, model)
				}
			}
			// Restore default if it's still valid
			if existingAIDefault != "" {
				for _, model := range cfg.Services.AI.Models {
					if model == existingAIDefault {
						cfg.Services.AI.Default = existingAIDefault
						break
					}
				}
			}

		case "database":
			cfg.Services.Database = config.DatabaseConfig{
				Type:       "postgres",
				Version:    "16",
				Port:       5432,
				Extensions: []string{},
			}

		case "vector":
			// Ensure database exists and add pgvector
			if cfg.Services.Database.Type == "" {
				cfg.Services.Database = config.DatabaseConfig{
					Type:       "postgres",
					Version:    "16",
					Port:       5432,
					Extensions: []string{"pgvector"},
				}
			} else {
				// Add pgvector if not present
				hasVector := false
				for _, ext := range cfg.Services.Database.Extensions {
					if ext == "pgvector" {
						hasVector = true
						break
					}
				}
				if !hasVector {
					cfg.Services.Database.Extensions = append(cfg.Services.Database.Extensions, "pgvector")
				}
			}

		case "mongodb":
			cfg.Services.MongoDB = config.MongoDBConfig{
				Type:        "mongodb",
				Version:     "7.0",
				Port:        27017,
				ReplicaSet:  false,
				AuthEnabled: true,
			}

		case "cache":
			cfg.Services.Cache = config.CacheConfig{
				Type:            "redis",
				Port:            6379,
				MaxMemory:       "256mb",
				MaxMemoryPolicy: "allkeys-lru",
				Persistence:     false,
			}

		case "queue":
			cfg.Services.Queue = config.QueueConfig{
				Type:            "redis",
				Port:            6380,
				MaxMemory:       "512mb",
				MaxMemoryPolicy: "noeviction",
				Persistence:     true,
				AppendFsync:     "everysec",
				AppendOnly:      true,
			}

		case "storage":
			cfg.Services.Storage = config.StorageConfig{
				Type:    "minio",
				Port:    9000,
				Console: 9001,
			}
		}
	}
}

// runNonInteractiveSetup handles setup without user interaction for code assistants
func runNonInteractiveSetup(cmd *cobra.Command, projectName string) error {
	var projectDir string
	var err error

	if projectName != "." {
		// New project setup
		projectDir, err = filepath.Abs(projectName)
		if err != nil {
			return fmt.Errorf("invalid project path: %w", err)
		}

		// Check if directory already exists
		if _, err := os.Stat(projectDir); !os.IsNotExist(err) {
			return fmt.Errorf("project directory '%s' already exists", projectName)
		}

		// Create project directory
		if err := os.MkdirAll(projectDir, 0755); err != nil {
			return fmt.Errorf("failed to create project directory: %w", err)
		}

		// Change to project directory
		originalDir, _ := os.Getwd()
		if err := os.Chdir(projectDir); err != nil {
			return fmt.Errorf("failed to change directory: %w", err)
		}
		defer os.Chdir(originalDir)

		fmt.Printf("%s Created project directory: %s\n", successColor("✓"), projectName)
	} else {
		projectDir = "."
		projectName = filepath.Base(projectDir)
	}

	// Initialize project structure first (creates empty config.yaml)
	if err := initializeProject(projectName, ".", projectName != "."); err != nil {
		return err
	}

	// Initialize config from the created file
	if err := config.Init(""); err != nil {
		return fmt.Errorf("failed to initialize config: %w", err)
	}

	// Get config and set project name
	cfg := config.Get()
	cfg.Project.Name = projectName

	// Handle preset configurations
	if setupPreset != "" {
		if err := applyPreset(cfg, setupPreset); err != nil {
			return fmt.Errorf("failed to apply preset: %w", err)
		}
		fmt.Printf("%s Applied preset: %s\n", successColor("✓"), setupPreset)
	}

	// Handle specific components
	if len(setupComponents) > 0 {
		if err := configureComponents(cfg, setupComponents); err != nil {
			return fmt.Errorf("failed to configure components: %w", err)
		}
		fmt.Printf("%s Configured components: %s\n", successColor("✓"), strings.Join(setupComponents, ", "))
	}

	// Handle specific models
	if len(setupModels) > 0 {
		cfg.Services.AI.Models = setupModels
		if cfg.Services.AI.Default == "" && len(setupModels) > 0 {
			cfg.Services.AI.Default = setupModels[0]
		}
		fmt.Printf("%s Configured models: %s\n", successColor("✓"), strings.Join(setupModels, ", "))
	}

	// Save configuration
	if err := config.Save(); err != nil {
		return fmt.Errorf("failed to save configuration: %w", err)
	}

	fmt.Printf("\n%s LocalCloud project setup completed!\n", successColor("✓"))
	fmt.Printf("Next steps:\n")
	fmt.Printf("  %s lc start              # Start all services\n", infoColor("→"))
	if len(setupModels) > 0 {
		fmt.Printf("  %s Models will be downloaded on first start\n", infoColor("→"))
	}
	fmt.Printf("  %s lc status             # Check service status\n", infoColor("→"))

	return nil
}

// applyPreset applies a preset configuration
func applyPreset(cfg *config.Config, preset string) error {
	switch preset {
	case "ai-dev":
		return configureComponents(cfg, []string{"llm", "embedding", "database", "vector"})
	case "full-stack":
		return configureComponents(cfg, []string{"llm", "embedding", "database", "vector", "cache", "queue", "storage"})
	case "minimal":
		return configureComponents(cfg, []string{"llm"})
	default:
		return fmt.Errorf("unknown preset: %s. Available presets: ai-dev, full-stack, minimal", preset)
	}
}

// configureComponents configures the specified components
func configureComponents(cfg *config.Config, componentList []string) error {
	for _, comp := range componentList {
		switch comp {
		case "llm", "embedding":
			if cfg.Services.AI.Port == 0 {
				cfg.Services.AI.Port = 11434
			}
		case "database":
			cfg.Services.Database = config.DatabaseConfig{
				Type:    "postgres",
				Version: "16",
				Port:    5432,
			}
		case "vector":
			// Ensure database is configured first
			if cfg.Services.Database.Type == "" {
				cfg.Services.Database = config.DatabaseConfig{
					Type:    "postgres",
					Version: "16",
					Port:    5432,
				}
			}
			cfg.Services.Database.Extensions = append(cfg.Services.Database.Extensions, "pgvector")
		case "cache":
			cfg.Services.Cache = config.CacheConfig{
				Type: "redis",
				Port: 6379,
			}
		case "queue":
			cfg.Services.Queue = config.QueueConfig{
				Type: "redis",
				Port: 6380,
			}
		case "storage":
			cfg.Services.Storage = config.StorageConfig{
				Type:    "minio",
				Port:    9000,
				Console: 9001,
			}
		case "mongodb":
			cfg.Services.MongoDB = config.MongoDBConfig{
				Type:        "mongodb",
				Version:     "7",
				Port:        27017,
				AuthEnabled: true,
			}
		default:
			return fmt.Errorf("unknown component: %s. Available: llm, embedding, database, vector, cache, queue, storage, mongodb", comp)
		}
	}
	return nil
}
