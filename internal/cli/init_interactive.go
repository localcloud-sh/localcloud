// Package cli implements the command-line interface for LocalCloud
package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/AlecAivazis/survey/v2"
	"github.com/briandowns/spinner"
	"github.com/fatih/color"
	"github.com/localcloud/localcloud/internal/components"
	"github.com/localcloud/localcloud/internal/config"
	"github.com/localcloud/localcloud/internal/models"
)

// InteractiveConfig represents the configuration built during interactive init
type InteractiveConfig struct {
	ProjectName string
	ProjectType string
	Components  []string
	Models      map[string]string // component -> model mapping
	Services    map[string]bool   // enabled services
}

// RunInteractiveInit runs the interactive initialization wizard
func RunInteractiveInit(projectName string) error {
	// Show welcome banner
	fmt.Println()
	fmt.Println(color.New(color.FgCyan, color.Bold).Sprint("🚀 LocalCloud Project Setup"))
	fmt.Println(strings.Repeat("━", 60))
	fmt.Println()

	// 1. Project type selection
	projectType, err := selectProjectType()
	if err != nil {
		return err
	}

	// 2. Component selection
	selectedComponents, err := selectComponents(projectType)
	if err != nil {
		return err
	}

	// 3. Model selection for AI components
	selectedModels, err := selectModels(selectedComponents)
	if err != nil {
		return err
	}

	// 4. Resource check
	if err := checkResources(selectedComponents, selectedModels); err != nil {
		return err
	}

	// 5. Generate configuration
	cfg := generateInteractiveConfig(projectName, projectType, selectedComponents, selectedModels)

	// 6. Create project structure
	if err := createProjectStructure(projectName); err != nil {
		return err
	}

	// 7. Save configuration
	if err := saveInteractiveConfig(cfg, projectName); err != nil {
		return err
	}

	// 8. Show summary
	showProjectSummary(projectName, selectedComponents, selectedModels)

	return nil
}

// selectProjectType prompts user to select project type
func selectProjectType() (string, error) {
	var options []string
	var typeMap = make(map[string]string)

	// Build options from templates
	for key, tmpl := range components.ProjectTemplates {
		option := fmt.Sprintf("%s - %s", tmpl.Name, tmpl.Description)
		options = append(options, option)
		typeMap[option] = key
	}

	prompt := &survey.Select{
		Message: "What would you like to build?",
		Options: options,
		Help:    "Select a project template or choose Custom to select components manually",
	}

	var selected string
	err := survey.AskOne(prompt, &selected)
	if err != nil {
		return "", err
	}

	return typeMap[selected], nil
}

// selectComponents prompts user to select components
func selectComponents(projectType string) ([]string, error) {
	// If not custom, use template
	if projectType != "custom" {
		template, _ := components.GetTemplate(projectType)
		return template.Components, nil
	}

	// Custom selection
	var options []string
	var componentMap = make(map[string]string)

	// Group by category
	categories := []string{"AI", "Database", "Infrastructure"}
	categoryColors := map[string]func(a ...interface{}) string{
		"AI":             color.New(color.FgGreen).SprintFunc(),
		"Database":       color.New(color.FgBlue).SprintFunc(),
		"Infrastructure": color.New(color.FgYellow).SprintFunc(),
	}

	for _, category := range categories {
		categoryLower := strings.ToLower(category)
		comps := components.GetComponentsByCategory(categoryLower)

		for _, comp := range comps {
			colorFunc := categoryColors[category]
			option := fmt.Sprintf("[%s] %s - %s",
				colorFunc(category), comp.Name, comp.Description)
			options = append(options, option)
			componentMap[option] = comp.ID
		}
	}

	prompt := &survey.MultiSelect{
		Message:  "Select components you need:",
		Options:  options,
		Help:     "Use space to select/deselect, Enter to confirm",
		PageSize: 10,
	}

	var selectedOptions []string
	err := survey.AskOne(prompt, &selectedOptions, survey.WithValidator(survey.MinItems(1)))
	if err != nil {
		return nil, err
	}

	// Map back to component IDs
	var selected []string
	for _, opt := range selectedOptions {
		selected = append(selected, componentMap[opt])
	}

	return selected, nil
}

// selectModels prompts user to select models for AI components
func selectModels(componentIDs []string) (map[string]string, error) {
	selectedModels := make(map[string]string)

	// Create Ollama manager to check installed models
	manager := models.NewManager("http://localhost:11434")

	for _, compID := range componentIDs {
		comp, _ := components.GetComponent(compID)

		// Skip non-AI components
		if len(comp.Models) == 0 {
			continue
		}

		// Special handling for embedding component
		if compID == "embedding" {
			model, err := selectEmbeddingModel(manager)
			if err != nil {
				return nil, err
			}
			selectedModels[compID] = model
			continue
		}

		// Regular model selection
		model, err := selectComponentModel(comp, manager)
		if err != nil {
			return nil, err
		}
		selectedModels[compID] = model
	}

	return selectedModels, nil
}

// selectEmbeddingModel handles embedding model selection
func selectEmbeddingModel(manager *models.Manager) (string, error) {
	fmt.Println()
	fmt.Println(infoColor("Selecting embedding model..."))

	// Get installed embedding models
	installedEmbeddings, _ := manager.GetAvailableEmbeddingModels()

	// Build options
	var options []string
	var modelMap = make(map[string]string)

	// Add installed models
	for _, model := range installedEmbeddings {
		info := models.GetEmbeddingModelInfo(model.Name)
		var dims string
		if info != nil {
			dims = fmt.Sprintf(", %d dims", info.Dimensions)
		}

		option := fmt.Sprintf("✓ %s (%s%s) [Installed]",
			model.Name, FormatBytes(model.Size), dims)
		options = append(options, option)
		modelMap[option] = model.Name
	}

	// Add predefined but not installed models
	for _, predefined := range models.PredefinedEmbeddingModels {
		installed := false
		for _, inst := range installedEmbeddings {
			if inst.Name == predefined.Name {
				installed = true
				break
			}
		}

		if !installed {
			option := fmt.Sprintf("  %s (%s, %d dims) [Not installed]",
				predefined.Name, predefined.Size, predefined.Dimensions)
			options = append(options, option)
			modelMap[option] = predefined.Name
		}
	}

	// Add custom option
	customOption := "  📝 Enter custom model name..."
	options = append(options, customOption)

	// Show prompt
	prompt := &survey.Select{
		Message:  "Select embedding model:",
		Options:  options,
		Help:     "Models marked with ✓ are already installed",
		PageSize: 10,
	}

	var selected string
	err := survey.AskOne(prompt, &selected)
	if err != nil {
		return "", err
	}

	// Handle custom model
	if selected == customOption {
		var customModel string
		prompt := &survey.Input{
			Message: "Enter custom embedding model name:",
			Help:    "e.g., mxbai-embed-large, bge-base-en-v1.5",
		}
		err := survey.AskOne(prompt, &customModel, survey.WithValidator(survey.Required))
		if err != nil {
			return "", err
		}
		return customModel, nil
	}

	modelName := modelMap[selected]

	// If not installed, offer to download
	if !strings.Contains(selected, "[Installed]") {
		fmt.Printf("\n%s Model '%s' is not installed.\n",
			warningColor("!"), modelName)

		var install bool
		installPrompt := &survey.Confirm{
			Message: "Would you like to download it now?",
			Default: true,
		}
		survey.AskOne(installPrompt, &install)

		if install {
			if err := downloadModel(manager, modelName); err != nil {
				return "", fmt.Errorf("failed to download model: %w", err)
			}
		}
	}

	return modelName, nil
}

// selectComponentModel handles model selection for a component
func selectComponentModel(comp components.Component, manager *models.Manager) (string, error) {
	fmt.Println()
	fmt.Printf("%s Selecting model for %s...\n", infoColor("ℹ"), comp.Name)

	// Get installed models
	installedModels, _ := manager.List()
	installedMap := make(map[string]bool)
	for _, m := range installedModels {
		installedMap[m.Name] = true
	}

	// Build options
	var options []string
	var modelMap = make(map[string]string)

	for _, model := range comp.Models {
		var option string
		if installedMap[model.Name] {
			option = fmt.Sprintf("✓ %s (%s) [Installed]", model.Name, model.Size)
			if model.Default {
				option += " (Recommended)"
			}
		} else {
			option = fmt.Sprintf("  %s (%s) [Not installed]", model.Name, model.Size)
			if model.Default {
				option += " (Recommended)"
			}
		}
		options = append(options, option)
		modelMap[option] = model.Name
	}

	// Find default option
	defaultIndex := 0
	for i, opt := range options {
		if strings.Contains(opt, "(Recommended)") {
			defaultIndex = i
			break
		}
	}

	prompt := &survey.Select{
		Message: fmt.Sprintf("Select %s model:", strings.ToLower(comp.Name)),
		Options: options,
		Default: options[defaultIndex],
		Help:    "Models marked with ✓ are already installed",
	}

	var selected string
	err := survey.AskOne(prompt, &selected)
	if err != nil {
		return "", err
	}

	modelName := modelMap[selected]

	// If not installed, offer to download
	if !strings.Contains(selected, "[Installed]") {
		fmt.Printf("\n%s Model '%s' is not installed.\n",
			warningColor("!"), modelName)

		var install bool
		installPrompt := &survey.Confirm{
			Message: "Would you like to download it now?",
			Default: true,
		}
		survey.AskOne(installPrompt, &install)

		if install {
			if err := downloadModel(manager, modelName); err != nil {
				return "", fmt.Errorf("failed to download model: %w", err)
			}
		}
	}

	return modelName, nil
}

// downloadModel downloads a model with progress
func downloadModel(manager *models.Manager, modelName string) error {
	fmt.Printf("\nDownloading %s...\n", modelName)

	progress := make(chan models.PullProgress)
	done := make(chan error)

	// Start pull in goroutine
	go func() {
		done <- manager.Pull(modelName, progress)
	}()

	// Show progress
	spin := spinner.New(spinner.CharSets[14], 100*time.Millisecond)
	spin.Start()

	for {
		select {
		case p := <-progress:
			spin.Stop()
			if p.Total > 0 {
				fmt.Printf("\r%s: %d%% [%s] %s/%s",
					p.Status,
					p.Percentage,
					progressBar(p.Percentage, 30),
					FormatBytes(p.Completed),
					FormatBytes(p.Total))
			} else {
				fmt.Printf("\r%s: %s", p.Status, p.Digest)
			}

		case err := <-done:
			spin.Stop()
			fmt.Println()
			if err != nil {
				return err
			}
			printSuccess(fmt.Sprintf("Model %s downloaded successfully!", modelName))
			return nil
		}
	}
}

// checkResources checks if system has enough resources
func checkResources(componentIDs []string, models map[string]string) error {
	totalRAM := components.CalculateRAMRequirement(componentIDs)

	// Get available RAM
	var availableRAM int64
	if runtime.GOOS == "darwin" {
		// macOS specific
		// This is simplified - in production use proper system calls
		availableRAM = 8 * components.GB
	} else {
		// Linux/Windows
		availableRAM = 8 * components.GB
	}

	if totalRAM > availableRAM {
		fmt.Printf("\n%s Selected components need %s RAM, you have %s available\n",
			warningColor("⚠️ Warning:"),
			FormatBytes(totalRAM),
			FormatBytes(availableRAM))

		var proceed bool
		prompt := &survey.Confirm{
			Message: "Continue anyway?",
			Default: false,
		}
		survey.AskOne(prompt, &proceed)

		if !proceed {
			return fmt.Errorf("cancelled by user")
		}
	}

	return nil
}

// generateInteractiveConfig generates configuration from selections
func generateInteractiveConfig(projectName, projectType string, componentIDs []string, models map[string]string) *config.Config {
	cfg := config.GetDefaults()

	// Set project info
	cfg.Project.Name = projectName
	cfg.Project.Type = projectType

	// Enable services based on components
	services := components.ComponentsToServices(componentIDs)

	// Configure services
	for _, service := range services {
		switch service {
		case "ai":
			// Already enabled by default
			// Add all selected models
			cfg.Services.AI.Models = []string{}
			for compID, modelName := range models {
				if compID == "llm" || compID == "embedding" {
					cfg.Services.AI.Models = append(cfg.Services.AI.Models, modelName)
				}
			}
			// Set default model
			if llmModel, ok := models["llm"]; ok {
				cfg.Services.AI.Default = llmModel
			}

		case "postgres":
			// Check if pgvector needed
			for _, compID := range componentIDs {
				if compID == "vector" {
					cfg.Services.Database.Extensions = []string{"pgvector"}
					break
				}
			}

		case "cache":
			// Already configured in defaults

		case "queue":
			// Already configured in defaults

		case "minio":
			// Already configured in defaults
		}
	}

	return cfg
}

// createProjectStructure creates the project directory structure
func createProjectStructure(projectName string) error {
	// Create .localcloud directory
	configPath := filepath.Join(projectPath, ".localcloud")
	if err := os.MkdirAll(configPath, 0755); err != nil {
		return fmt.Errorf("failed to create .localcloud directory: %w", err)
	}

	// Create .gitignore
	gitignore := filepath.Join(projectPath, ".gitignore")
	gitignoreContent := `.localcloud/data/
.localcloud/logs/
.localcloud/tunnels/
.env.local
*.log
`
	if err := os.WriteFile(gitignore, []byte(gitignoreContent), 0644); err != nil {
		return fmt.Errorf("failed to create .gitignore: %w", err)
	}

	return nil
}

// saveInteractiveConfig saves the configuration with component info
func saveInteractiveConfig(cfg *config.Config, projectName string) error {
	// Convert to viper for saving
	v := config.GetViper()

	// Set all values
	v.Set("version", cfg.Version)
	v.Set("project", cfg.Project)
	v.Set("services", cfg.Services)
	v.Set("resources", cfg.Resources)
	v.Set("connectivity", cfg.Connectivity)
	v.Set("cli", cfg.CLI)

	// Save to file
	configFile := filepath.Join(projectPath, ".localcloud", "config.yaml")
	if err := v.WriteConfigAs(configFile); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	return nil
}

// showProjectSummary displays the project configuration summary
func showProjectSummary(projectName string, componentIDs []string, models map[string]string) {
	fmt.Println()
	fmt.Println(successColor("✓ Project configuration created!"))
	fmt.Println()

	fmt.Println("Your project will include:")

	// Group components by category
	aiComps := []string{}
	dbComps := []string{}
	infraComps := []string{}

	for _, compID := range componentIDs {
		comp, _ := components.GetComponent(compID)
		switch comp.Category {
		case "ai":
			modelInfo := ""
			if model, ok := models[compID]; ok {
				modelInfo = fmt.Sprintf(" with %s", model)
			}
			aiComps = append(aiComps, fmt.Sprintf("• %s%s", comp.Name, modelInfo))
		case "database":
			dbComps = append(dbComps, fmt.Sprintf("• %s", comp.Name))
		case "infrastructure":
			infraComps = append(infraComps, fmt.Sprintf("• %s", comp.Name))
		}
	}

	if len(aiComps) > 0 {
		fmt.Println("\n🤖 AI Services:")
		for _, comp := range aiComps {
			fmt.Println("  " + comp)
		}
	}

	if len(dbComps) > 0 {
		fmt.Println("\n💾 Database:")
		for _, comp := range dbComps {
			fmt.Println("  " + comp)
		}
	}

	if len(infraComps) > 0 {
		fmt.Println("\n🔧 Infrastructure:")
		for _, comp := range infraComps {
			fmt.Println("  " + comp)
		}
	}

	fmt.Println()
	fmt.Println("Ready to start? Run: " + color.New(color.FgGreen, color.Bold).Sprint("lc start"))
	fmt.Println()
}
