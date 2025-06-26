// internal/cli/init_interactive.go
// Package cli implements the command-line interface for LocalCloud
package cli

import (
	"fmt"
	"github.com/spf13/viper"
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
	// Order templates for better UX
	templateOrder := []string{"rag", "chatbot", "voice", "api", "custom"}

	for _, key := range templateOrder {
		if tmpl, ok := components.ProjectTemplates[key]; ok {
			option := fmt.Sprintf("%s - %s", tmpl.Name, tmpl.Description)
			options = append(options, option)
			typeMap[option] = key
		}
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

	// Group by category with specific order
	categoryOrder := []struct {
		name  string
		color func(a ...interface{}) string
	}{
		{"AI", color.New(color.FgGreen).SprintFunc()},
		{"Database", color.New(color.FgBlue).SprintFunc()},
		{"Infrastructure", color.New(color.FgYellow).SprintFunc()},
	}

	// Component display order within categories
	componentOrder := map[string][]string{
		"ai":             {"llm", "embedding", "stt"},
		"database":       {"vector"},
		"infrastructure": {"cache", "queue", "storage"},
	}

	for _, cat := range categoryOrder {
		categoryLower := strings.ToLower(cat.name)

		// Get components in specified order
		if compIDs, ok := componentOrder[categoryLower]; ok {
			for _, compID := range compIDs {
				if comp, err := components.GetComponent(compID); err == nil {
					option := fmt.Sprintf("[%s] %s - %s",
						cat.color(cat.name), comp.Name, comp.Description)
					options = append(options, option)
					componentMap[option] = comp.ID
				}
			}
		}
	}

	prompt := &survey.MultiSelect{
		Message:  "Select components you need: (Press <space> to select, <enter> to confirm)",
		Options:  options,
		Help:     "Use arrow keys to navigate, space to select/deselect, Enter to confirm",
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
		option := fmt.Sprintf("✓ %s%s [Installed]", model.Name, dims)
		options = append(options, option)
		modelMap[option] = model.Name
	}

	// Add popular embedding models if not installed
	popularModels := []string{"nomic-embed-text", "mxbai-embed-large", "all-minilm"}
	for _, modelName := range popularModels {
		installed := false
		for _, m := range installedEmbeddings {
			if m.Name == modelName {
				installed = true
				break
			}
		}
		if !installed {
			info := models.GetEmbeddingModelInfo(modelName)
			var details string
			if info != nil {
				details = fmt.Sprintf(" (%s, %d dims)", info.Size, info.Dimensions)
			}
			option := fmt.Sprintf("  %s%s [Not installed]", modelName, details)
			if modelName == "nomic-embed-text" {
				option += " (Recommended)"
			}
			options = append(options, option)
			modelMap[option] = modelName
		}
	}

	// Add custom model option
	customOption := "  💡 Use custom model..."
	options = append(options, customOption)

	// Find default option
	defaultIndex := 0
	for i, opt := range options {
		if strings.Contains(opt, "(Recommended)") {
			defaultIndex = i
			break
		}
	}

	prompt := &survey.Select{
		Message: "Select embedding model:",
		Options: options,
		Default: options[defaultIndex],
		Help:    "Models marked with ✓ are already installed",
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

		// Offer to download custom model
		fmt.Printf("\n%s Custom model '%s' selected.\n",
			infoColor("ℹ"), customModel)

		var install bool
		installPrompt := &survey.Confirm{
			Message: "Would you like to download it now?",
			Default: true,
		}
		survey.AskOne(installPrompt, &install)

		if install {
			if err := downloadModel(manager, customModel); err != nil {
				return "", fmt.Errorf("failed to download model: %w", err)
			}
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

// checkResources performs resource checks for selected components
func checkResources(componentIDs []string, selectedModels map[string]string) error {
	// Calculate total RAM requirement
	totalRAM := components.CalculateRAMRequirement(componentIDs)

	// Add model-specific RAM requirements
	for compID, modelName := range selectedModels {
		comp, _ := components.GetComponent(compID)
		for _, model := range comp.Models {
			if model.Name == modelName && model.RAM > 0 {
				totalRAM += model.RAM - comp.MinRAM // Adjust for model-specific RAM
				break
			}
		}
	}

	// Get available system resources
	availableRAM := getAvailableRAM()

	// Check if we have enough RAM
	if totalRAM > availableRAM {
		fmt.Printf("\n%s Selected components need %s RAM, you have %s available\n",
			warningColor("⚠️  Warning:"),
			FormatBytes(totalRAM),
			FormatBytes(availableRAM))

		var proceed bool
		prompt := &survey.Confirm{
			Message: "Continue anyway?",
			Default: false,
		}
		err := survey.AskOne(prompt, &proceed)
		if err != nil || !proceed {
			return fmt.Errorf("insufficient resources")
		}
	}

	return nil
}

// internal/cli/init_interactive.go - Replace generateInteractiveConfig

// generateInteractiveConfig generates configuration from selections
func generateInteractiveConfig(projectName, projectType string, componentIDs []string, selectedModels map[string]string) *config.Config {
	// Start with empty config, not defaults
	cfg := &config.Config{
		Version: "1",
		Project: config.ProjectConfig{
			Name: projectName,
			Type: projectType,
		},
		Services: config.ServicesConfig{}, // Empty services
		Resources: config.ResourcesConfig{
			MemoryLimit: "4GB",
			CPULimit:    "2",
		},
		Connectivity: config.ConnectivityConfig{
			Enabled: false,
			Tunnel: config.TunnelConfig{
				Provider: "cloudflare",
			},
		},
		CLI: config.CLIConfig{
			ShowServiceInfo: true,
		},
	}

	// Only configure selected components
	for _, compID := range componentIDs {
		switch compID {
		case "llm", "embedding":
			if cfg.Services.AI.Port == 0 {
				cfg.Services.AI = config.AIConfig{
					Port:   11434,
					Models: []string{},
				}
			}
			if model, ok := selectedModels[compID]; ok {
				cfg.Services.AI.Models = append(cfg.Services.AI.Models, model)
			}

		case "vector":
			cfg.Services.Database = config.DatabaseConfig{
				Type:       "postgres",
				Version:    "16",
				Port:       5432,
				Extensions: []string{"pgvector"},
			}

		case "cache":
			cfg.Services.Cache = config.CacheConfig{
				Type:            "redis",
				Port:            6379,
				MaxMemory:       "512mb",
				MaxMemoryPolicy: "allkeys-lru",
				Persistence:     false,
			}

		case "queue":
			cfg.Services.Queue = config.QueueConfig{
				Type:            "redis",
				Port:            6380,
				MaxMemory:       "1gb",
				MaxMemoryPolicy: "noeviction",
				Persistence:     true,
				AppendOnly:      true,
				AppendFsync:     "everysec",
			}

		case "storage":
			cfg.Services.Storage = config.StorageConfig{
				Type:    "minio",
				Port:    9000,
				Console: 9001,
			}

		case "stt":
			cfg.Services.Whisper = config.WhisperConfig{
				Type: "localllama",
				Port: 9000,
			}
			if model, ok := selectedModels[compID]; ok {
				cfg.Services.Whisper.Model = model
			}
		}
	}

	// Set default model for AI service
	if len(cfg.Services.AI.Models) > 0 {
		cfg.Services.AI.Default = cfg.Services.AI.Models[0]
	}

	return cfg
}

// createProjectStructure creates project directories
func createProjectStructure(projectName string) error {
	// Create .localcloud directory
	localcloudDir := filepath.Join(".", ".localcloud")
	if err := os.MkdirAll(localcloudDir, 0755); err != nil {
		return fmt.Errorf("failed to create .localcloud directory: %w", err)
	}

	// Create .gitignore if it doesn't exist
	gitignorePath := ".gitignore"
	if _, err := os.Stat(gitignorePath); os.IsNotExist(err) {
		gitignoreContent := `# LocalCloud
.localcloud/data/
.localcloud/logs/
*.log
`
		if err := os.WriteFile(gitignorePath, []byte(gitignoreContent), 0644); err != nil {
			return fmt.Errorf("failed to create .gitignore: %w", err)
		}
	}

	return nil
}

// saveInteractiveConfig saves configuration to file
func saveInteractiveConfig(cfg *config.Config, projectName string) error {
	// Create new viper instance for clean config
	v := viper.New()
	v.SetConfigType("yaml")

	// Set all configuration values
	v.Set("version", cfg.Version)
	v.Set("project.name", cfg.Project.Name)
	v.Set("project.type", cfg.Project.Type)

	// Set service configurations only if they're configured
	if cfg.Services.AI.Port > 0 {
		v.Set("services.ai.port", cfg.Services.AI.Port)
		v.Set("services.ai.models", cfg.Services.AI.Models)
		v.Set("services.ai.default", cfg.Services.AI.Default)
	}

	if cfg.Services.Database.Type != "" {
		v.Set("services.database.type", cfg.Services.Database.Type)
		v.Set("services.database.version", cfg.Services.Database.Version)
		v.Set("services.database.port", cfg.Services.Database.Port)
		v.Set("services.database.extensions", cfg.Services.Database.Extensions)
	}

	if cfg.Services.Cache.Type != "" {
		v.Set("services.cache.type", cfg.Services.Cache.Type)
		v.Set("services.cache.port", cfg.Services.Cache.Port)
		v.Set("services.cache.maxmemory", cfg.Services.Cache.MaxMemory)
		v.Set("services.cache.maxmemory_policy", cfg.Services.Cache.MaxMemoryPolicy)
		v.Set("services.cache.persistence", cfg.Services.Cache.Persistence)
	}

	if cfg.Services.Queue.Type != "" {
		v.Set("services.queue.type", cfg.Services.Queue.Type)
		v.Set("services.queue.port", cfg.Services.Queue.Port)
		v.Set("services.queue.maxmemory", cfg.Services.Queue.MaxMemory)
		v.Set("services.queue.maxmemory_policy", cfg.Services.Queue.MaxMemoryPolicy)
		v.Set("services.queue.persistence", cfg.Services.Queue.Persistence)
		v.Set("services.queue.appendfsync", cfg.Services.Queue.AppendFsync)
	}

	if cfg.Services.Storage.Type != "" {
		v.Set("services.storage.type", cfg.Services.Storage.Type)
		v.Set("services.storage.port", cfg.Services.Storage.Port)
		v.Set("services.storage.console", cfg.Services.Storage.Console)
	}

	if cfg.Services.Whisper.Type != "" {
		v.Set("services.whisper.type", cfg.Services.Whisper.Type)
		v.Set("services.whisper.port", cfg.Services.Whisper.Port)
		v.Set("services.whisper.model", cfg.Services.Whisper.Model)
	}

	// Set resource configurations
	v.Set("resources.memory_limit", cfg.Resources.MemoryLimit)
	v.Set("resources.cpu_limit", cfg.Resources.CPULimit)

	// Set connectivity configurations
	v.Set("connectivity.enabled", cfg.Connectivity.Enabled)
	v.Set("connectivity.tunnel.provider", cfg.Connectivity.Tunnel.Provider)

	// Set CLI configurations
	v.Set("cli.show_service_info", cfg.CLI.ShowServiceInfo)

	// Save configuration
	configPath := filepath.Join(".localcloud", "config.yaml")
	return v.WriteConfigAs(configPath)
}

// showProjectSummary displays project configuration summary
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

// downloadModel downloads a model using the manager
func downloadModel(manager *models.Manager, modelName string) error {
	// Create progress channel
	progress := make(chan models.PullProgress)
	done := make(chan error)

	// Start pull in goroutine
	go func() {
		done <- manager.Pull(modelName, progress)
	}()

	// Create spinner
	s := spinner.New(spinner.CharSets[14], 100*time.Millisecond)
	s.Suffix = fmt.Sprintf(" Downloading %s...", modelName)
	s.Start()

	// Handle progress updates
	for {
		select {
		case p, ok := <-progress:
			if !ok {
				// Channel closed, wait for final result
				continue
			}
			s.Stop()
			if p.Total > 0 {
				percentage := int((p.Completed * 100) / p.Total)
				bar := progressBar(percentage, 30)
				fmt.Printf("\r%s: %d%% [%s] %s/%s",
					p.Status,
					percentage,
					bar,
					FormatBytes(p.Completed),
					FormatBytes(p.Total))
			} else {
				fmt.Printf("\r%s: %s", p.Status, p.Digest)
			}
			s.Start()

		case err := <-done:
			s.Stop()
			fmt.Println() // New line after progress
			if err != nil {
				return err
			}
			fmt.Printf("%s Model %s downloaded successfully!\n", successColor("✓"), modelName)
			return nil
		}
	}
}

// Helper functions for resource checking
func getAvailableRAM() int64 {
	// This is a simplified version - in production you'd use runtime.MemStats
	// or system-specific calls to get actual available memory
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	// For now, return a reasonable default based on system
	// In a real implementation, this would query actual system memory
	return 8 * 1024 * 1024 * 1024 // 8GB default
}
