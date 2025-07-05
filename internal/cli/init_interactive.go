// internal/cli/init_interactive.go
// Package cli implements the command-line interface for LocalCloud
package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/spf13/viper"

	"github.com/AlecAivazis/survey/v2"
	"github.com/briandowns/spinner"
	"github.com/fatih/color"
	"github.com/localcloud-sh/localcloud/internal/components"
	"github.com/localcloud-sh/localcloud/internal/config"
	"github.com/localcloud-sh/localcloud/internal/models"
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

func selectProjectType() (string, error) {
	var options []string
	var typeMap = make(map[string]string)

	// Build options from templates
	// Order templates for better UX - matching your requirements
	templateOrder := []string{"custom", "rag", "chatbot", "fullstack", "simple"}

	for _, key := range templateOrder {
		if tmpl, ok := components.ProjectTemplates[key]; ok {
			option := fmt.Sprintf("%s - %s", tmpl.Name, tmpl.Description)
			options = append(options, option)
			typeMap[option] = key
		}
	}

	// If no templates found, add at least custom
	if len(options) == 0 {
		options = append(options, "Custom - Select components manually")
		typeMap["Custom - Select components manually"] = "custom"
	}

	prompt := &survey.Select{
		Message:  "What would you like to build?",
		Options:  options,
		PageSize: 10,
	}

	var selected string
	if err := survey.AskOne(prompt, &selected); err != nil {
		return "", err
	}

	projectType, ok := typeMap[selected]
	if !ok {
		return "custom", nil
	}

	return projectType, nil
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

// internal/cli/init_interactive.go - Updated selectEmbeddingModel function

func selectEmbeddingModel(manager *models.Manager) (string, error) {
	fmt.Println()
	fmt.Println(infoColor("Selecting embedding model..."))

	// Get ALL installed models
	allInstalledModels, _ := manager.List()

	// Filter embedding models from installed
	installedEmbeddings := []models.Model{}
	customEmbeddings := []models.Model{}

	for _, model := range allInstalledModels {
		if models.IsEmbeddingModel(model.Name) {
			// Check if it's in our predefined list
			isPredefined := false
			modelBaseName := strings.TrimSuffix(model.Name, ":latest")

			for _, predef := range models.PredefinedEmbeddingModels {
				if predef.Name == modelBaseName || predef.Name == model.Name {
					isPredefined = true
					// Update model name to match predefined name (without :latest)
					model.Name = predef.Name
					break
				}
			}

			if isPredefined {
				installedEmbeddings = append(installedEmbeddings, model)
			} else {
				customEmbeddings = append(customEmbeddings, model)
			}
		}
	}

	// Build options
	var options []string
	var modelMap = make(map[string]string)

	// 1. Add predefined models (installed and not installed)
	// Check installed predefined models
	for _, model := range installedEmbeddings {
		info := models.GetEmbeddingModelInfo(model.Name)
		var dims string
		if info != nil && info.Dimensions > 0 {
			dims = fmt.Sprintf(", %d dims", info.Dimensions)
		}
		option := fmt.Sprintf("✓ %s%s [Installed]", model.Name, dims)
		if model.Name == "nomic-embed-text" {
			option += " (Recommended)"
		}
		options = append(options, option)
		modelMap[option] = model.Name
	}

	// Add not-installed predefined models
	popularModels := []string{"nomic-embed-text", "mxbai-embed-large", "all-minilm", "bge-base", "bge-large"}
	for _, modelName := range popularModels {
		// Skip if already installed
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

	// 2. Add custom embedding models if any
	if len(customEmbeddings) > 0 {
		options = append(options, "─────────────────────────────────")

		for _, model := range customEmbeddings {
			option := fmt.Sprintf("✓ %s [Installed] (Custom)", model.Name)
			options = append(options, option)
			modelMap[option] = model.Name
		}
	}

	// 3. Add manual entry option
	options = append(options, "─────────────────────────────────")
	customOption := "💡 Enter embedding model name manually..."
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
		Message:  "Select embedding model:",
		Options:  options,
		Default:  options[defaultIndex],
		Help:     "Models marked with ✓ are already installed",
		PageSize: 15,
	}

	var selected string
	err := survey.AskOne(prompt, &selected)
	if err != nil {
		return "", err
	}

	// Handle manual entry
	if selected == customOption {
		var customModel string
		inputPrompt := &survey.Input{
			Message: "Enter custom embedding model name:",
			Help:    "e.g., bge-m3, e5-mistral-7b-instruct, multilingual-e5-large",
		}
		err := survey.AskOne(inputPrompt, &customModel, survey.WithValidator(survey.Required))
		if err != nil {
			return "", err
		}

		// Check if it's actually an embedding model
		if !models.IsEmbeddingModel(customModel) {
			fmt.Printf("\n%s '%s' doesn't appear to be an embedding model.\n",
				warningColor("⚠"), customModel)

			var proceed bool
			proceedPrompt := &survey.Confirm{
				Message: "Use it anyway?",
				Default: false,
			}
			survey.AskOne(proceedPrompt, &proceed)
			if !proceed {
				return selectEmbeddingModel(manager) // Retry selection
			}
		}

		// Check if already installed
		isInstalled := false
		for _, m := range allInstalledModels {
			if m.Name == customModel {
				isInstalled = true
				break
			}
		}

		if !isInstalled {
			fmt.Printf("\n%s Model '%s' is not installed.\n",
				warningColor("!"), customModel)

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
		}

		return customModel, nil
	}

	// Skip separator lines
	if strings.Contains(selected, "─────") {
		return "", fmt.Errorf("invalid selection")
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
func generateInteractiveConfig(projectName, projectType string, componentIDs []string, selectedModels map[string]string) *config.Config {
	// Start with minimal config - no default services
	cfg := &config.Config{
		Version: "1",
		Project: config.ProjectConfig{
			Name:       projectName,
			Type:       projectType,
			Components: componentIDs, // This is the source of truth
		},
		Services: config.ServicesConfig{}, // Empty services - will only add what's selected
		Resources: config.ResourcesConfig{
			MemoryLimit: "",
			CPULimit:    "",
		},
		Connectivity: config.ConnectivityConfig{
			Enabled: false,
			Tunnel: config.TunnelConfig{
				Provider: "cloudflare",
			},
		},
		CLI: config.CLIConfig{
			ShowServiceInfo: false,
		},
	}

	// Only configure services that were actually selected
	for _, compID := range componentIDs {
		switch compID {
		case "llm", "embedding", "stt":
			// Initialize AI service only if AI components are selected
			if cfg.Services.AI.Port == 0 {
				cfg.Services.AI = config.AIConfig{
					Port:    11434,
					Models:  []string{},
					Default: "",
				}
			}

			// Add models for the specific component
			if model, ok := selectedModels[compID]; ok {
				cfg.Services.AI.Models = append(cfg.Services.AI.Models, model)

				// Set default for LLM models
				if compID == "llm" && cfg.Services.AI.Default == "" {
					cfg.Services.AI.Default = model
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
			// Vector requires database
			if cfg.Services.Database.Port == 0 {
				cfg.Services.Database = config.DatabaseConfig{
					Type:       "postgres",
					Version:    "16",
					Port:       5432,
					Extensions: []string{"pgvector"},
				}
			} else {
				// Add pgvector extension to existing database
				cfg.Services.Database.Extensions = append(cfg.Services.Database.Extensions, "pgvector")
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
	v.Set("project.components", cfg.Project.Components)

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

	if cfg.Services.MongoDB.Type != "" {
		v.Set("services.mongodb.type", cfg.Services.MongoDB.Type)
		v.Set("services.mongodb.version", cfg.Services.MongoDB.Version)
		v.Set("services.mongodb.port", cfg.Services.MongoDB.Port)
		v.Set("services.mongodb.replica_set", cfg.Services.MongoDB.ReplicaSet)
		v.Set("services.mongodb.auth_enabled", cfg.Services.MongoDB.AuthEnabled)
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
	progress := make(chan models.PullProgress, 100) // Buffered channel
	done := make(chan error, 1)

	// Start time tracking
	startTime := time.Now()
	lastProgressTime := time.Now()
	var lastCompleted int64

	// Start pull in goroutine
	go func() {
		done <- manager.Pull(modelName, progress)
	}()

	// Create spinner for initial state
	s := spinner.New(spinner.CharSets[14], 100*time.Millisecond)
	s.Suffix = fmt.Sprintf(" Connecting to download %s...", modelName)
	s.Start()

	// Track download phases
	var currentPhase string
	progressStarted := false

	// Handle progress updates
	for {
		select {
		case p, ok := <-progress:
			if !ok {
				// Channel closed, wait for final result
				continue
			}

			// Stop spinner once we have progress
			if !progressStarted {
				s.Stop()
				progressStarted = true
			}

			// Update phase tracking
			if p.Status != currentPhase {
				currentPhase = p.Status
				// Clear line for phase changes
				fmt.Printf("\r%s\r", strings.Repeat(" ", 80))
			}

			// Calculate download speed
			elapsed := time.Since(lastProgressTime).Seconds()
			if elapsed > 0 && p.Completed > lastCompleted {
				speed := float64(p.Completed-lastCompleted) / elapsed
				lastCompleted = p.Completed
				lastProgressTime = time.Now()

				if p.Total > 0 {
					percentage := int((p.Completed * 100) / p.Total)
					bar := progressBar(percentage, 30)

					// Calculate ETA
					if speed > 0 {
						remaining := p.Total - p.Completed
						eta := time.Duration(float64(remaining)/speed) * time.Second

						fmt.Printf("\r%s: %d%% [%s] %s/%s @ %s/s - ETA: %s",
							p.Status,
							percentage,
							bar,
							FormatBytes(p.Completed),
							FormatBytes(p.Total),
							FormatBytes(int64(speed)),
							formatDuration(eta))
					} else {
						fmt.Printf("\r%s: %d%% [%s] %s/%s",
							p.Status,
							percentage,
							bar,
							FormatBytes(p.Completed),
							FormatBytes(p.Total))
					}
				} else {
					// No total size known yet
					fmt.Printf("\r%s: %s downloaded @ %s/s",
						p.Status,
						FormatBytes(p.Completed),
						FormatBytes(int64(speed)))
				}
			} else if p.Status != "" {
				// Status update without progress
				fmt.Printf("\r%s: %s", p.Status, p.Digest)
			}

		case err := <-done:
			if progressStarted {
				fmt.Println() // New line after progress
			} else {
				s.Stop()
			}

			if err != nil {
				// Check if it's a timeout error
				if strings.Contains(err.Error(), "timeout") || strings.Contains(err.Error(), "deadline") {
					return fmt.Errorf("download timed out - this might be due to slow connection. Try again or download manually with 'lc models pull %s'", modelName)
				}
				return err
			}

			// Calculate total time and average speed
			totalTime := time.Since(startTime)
			if lastCompleted > 0 {
				avgSpeed := float64(lastCompleted) / totalTime.Seconds()
				fmt.Printf("%s Model %s downloaded successfully! (%s @ avg %s/s)\n",
					successColor("✓"),
					modelName,
					formatDuration(totalTime),
					FormatBytes(int64(avgSpeed)))
			} else {
				fmt.Printf("%s Model %s downloaded successfully!\n", successColor("✓"), modelName)
			}

			return nil

		case <-time.After(30 * time.Second):
			// Timeout check - if no progress for 30 seconds, show a message
			if !progressStarted {
				s.Stop()
				fmt.Printf("\rStill waiting for download to start... (this is normal for large models)\n")
				s.Start()
			}
		}
	}
}

// formatDuration formats a duration in human-readable format
func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm %ds", int(d.Minutes()), int(d.Seconds())%60)
	}
	hours := int(d.Hours())
	minutes := int(d.Minutes()) % 60
	return fmt.Sprintf("%dh %dm", hours, minutes)
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

// internal/cli/init_interactive.go - Fixes

// 1. Fix typo: savebInteractiveConfig -> saveInteractiveConfig
// Line 71: Change savebInteractiveConfig to saveInteractiveConfig

// 2. Fix components.GetComponent usage
// Replace this around line 125:
func selectComponents(projectType string) ([]string, error) {
	// If not custom, return template components
	if projectType != "custom" {
		if tmpl, ok := components.ProjectTemplates[projectType]; ok {
			return tmpl.Components, nil
		}
	}

	// Custom selection
	var options []string
	var componentMap = make(map[string]string)

	// Order components logically
	componentOrder := []string{"llm", "embedding", "database", "vector", "mongodb", "stt", "cache", "queue", "storage"}

	for _, compID := range componentOrder {
		comp, err := components.GetComponent(compID)
		if err != nil {
			continue
		}
		option := fmt.Sprintf("%s - %s", comp.Name, comp.Description)
		options = append(options, option)
		componentMap[option] = compID
	}

	prompt := &survey.MultiSelect{
		Message:  "Select components you need (Space to select, Enter to confirm):",
		Options:  options,
		PageSize: 10,
	}

	var selected []string
	if err := survey.AskOne(prompt, &selected, survey.WithValidator(survey.MinItems(1))); err != nil {
		return nil, err
	}

	// Convert selections to component IDs
	var selectedIDs []string
	for _, sel := range selected {
		if compID, ok := componentMap[sel]; ok {
			selectedIDs = append(selectedIDs, compID)
		}
	}

	// Validate dependencies
	hasVector := false
	hasDatabase := false
	for _, id := range selectedIDs {
		if id == "vector" {
			hasVector = true
		}
		if id == "database" {
			hasDatabase = true
		}
	}

	if hasVector && !hasDatabase {
		fmt.Printf("\n%s Vector Search requires Database (PostgreSQL) to be selected.\n", errorColor("Error:"))
		fmt.Println("Please select both or select Database instead of Vector Search.")
		return selectComponents(projectType) // Retry selection
	}

	return selectedIDs, nil
}

// Fix selectComponentModel to use components.ModelOption
func selectComponentModel(comp components.Component, manager *models.Manager) (string, error) {
	fmt.Printf("\n%s %s\n", infoColor("Selecting model for:"), comp.Name)

	// Get available models for this component
	availableModels := comp.Models

	// Check which models are already installed
	installedModels, _ := manager.List()
	installedMap := make(map[string]bool)
	for _, m := range installedModels {
		installedMap[m.Name] = true
		// Also check without :latest suffix
		installedMap[strings.TrimSuffix(m.Name, ":latest")] = true
	}

	// Build options
	var options []string
	var modelMap = make(map[string]string)

	for _, model := range availableModels {
		var option string
		if installedMap[model.Name] {
			option = fmt.Sprintf("✓ %s (%s) [Installed]", model.Name, model.Size)
		} else {
			option = fmt.Sprintf("  %s (%s) [Not installed]", model.Name, model.Size)
		}

		if model.Default {
			option += " - Recommended"
		}

		options = append(options, option)
		modelMap[option] = model.Name
	}

	// Add custom option
	customOption := "  Custom model..."
	options = append(options, customOption)

	prompt := &survey.Select{
		Message: "Select model:",
		Options: options,
	}

	var selected string
	if err := survey.AskOne(prompt, &selected); err != nil {
		return "", err
	}

	// Handle custom model
	if selected == customOption {
		var customModel string
		customPrompt := &survey.Input{
			Message: "Enter custom model name:",
			Help:    "e.g., llama3.2:3b, mistral:latest",
		}
		if err := survey.AskOne(customPrompt, &customModel); err != nil {
			return "", err
		}
		return customModel, nil
	}

	return modelMap[selected], nil
}

// 4. Fix handleComponentModification
func handleComponentModification(toAdd, toRemove []string) error {
	cfg := config.Get()
	if cfg == nil {
		return fmt.Errorf("failed to load configuration")
	}

	// Validate component IDs
	for _, compID := range append(toAdd, toRemove...) {
		comp, err := components.GetComponent(compID)
		if err != nil {
			return fmt.Errorf("unknown component: %s", compID)
		}
		_ = comp // silence unused variable warning
	}

	// Handle removals
	if len(toRemove) > 0 {
		fmt.Printf("%s Removing components: %s\n", warningColor("⚠"), strings.Join(toRemove, ", "))
		removeComponentsFromConfig(cfg, toRemove)
	}

	// Handle additions
	if len(toAdd) > 0 {
		fmt.Printf("%s Adding components: %s\n", successColor("✓"), strings.Join(toAdd, ", "))

		// Select models for AI components
		selectedModels := make(map[string]string)
		manager := models.NewManager("http://localhost:11434")

		for _, compID := range toAdd {
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

		updateConfig(cfg, toAdd, selectedModels)
	}

	// Save configuration
	if err := config.Save(); err != nil {
		return fmt.Errorf("failed to save configuration: %w", err)
	}

	printSuccess("Configuration updated!")
	fmt.Println("\nRun 'lc restart' to apply changes.")

	return nil
}

// 5. Add missing helper functions that were referenced
func updateConfig(cfg *config.Config, componentIDs []string, selectedModels map[string]string) {
	// Helper function to check for component presence
	contains := func(s []string, e string) bool {
		for _, a := range s {
			if a == e {
				return true
			}
		}
		return false
	}

	// IMPORTANT: Set the project components field - this is crucial for the system to recognize configured components
	cfg.Project.Components = componentIDs

	// Configure PostgreSQL only if 'database' or 'vector' is selected
	if contains(componentIDs, "database") || contains(componentIDs, "vector") {
		if cfg.Services.Database.Port == 0 {
			cfg.Services.Database = config.DatabaseConfig{
				Type:       "postgres",
				Version:    "16",
				Port:       5432,
				Extensions: []string{},
			}
		}
	}

	// Configure other components
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
				// Avoid adding duplicate models
				if !contains(cfg.Services.AI.Models, model) {
					cfg.Services.AI.Models = append(cfg.Services.AI.Models, model)
				}
			}
		case "vector":
			// Add pgvector extension if not already present
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

	// Update default AI model
	if len(cfg.Services.AI.Models) > 0 && cfg.Services.AI.Default == "" {
		for _, model := range cfg.Services.AI.Models {
			if !models.IsEmbeddingModel(model) {
				cfg.Services.AI.Default = model
				break
			}
		}
	}

	// IMPORTANT: Ensure complete config cleanup - call the same function used in setup.go
	// Note: This requires importing the updateCompleteConfig function or making it available
	// For now, we'll ensure the project components are set at minimum
	cfg.Project.Components = componentIDs
}

func removeComponentsFromConfig(cfg *config.Config, componentIDs []string) {
	for _, compID := range componentIDs {
		switch compID {
		case "llm":
			// Remove LLM models but keep embedding models
			var embeddingModels []string
			for _, model := range cfg.Services.AI.Models {
				if models.IsEmbeddingModel(model) {
					embeddingModels = append(embeddingModels, model)
				}
			}
			cfg.Services.AI.Models = embeddingModels

			// Clear default if it was an LLM
			if cfg.Services.AI.Default != "" && !models.IsEmbeddingModel(cfg.Services.AI.Default) {
				cfg.Services.AI.Default = ""
			}

		case "embedding":
			// Remove embedding models but keep LLM models
			var llmModels []string
			for _, model := range cfg.Services.AI.Models {
				if !models.IsEmbeddingModel(model) {
					llmModels = append(llmModels, model)
				}
			}
			cfg.Services.AI.Models = llmModels

		case "vector":
			// Remove pgvector extension but keep database if it exists
			if cfg.Services.Database.Port > 0 {
				var newExtensions []string
				for _, ext := range cfg.Services.Database.Extensions {
					if ext != "pgvector" {
						newExtensions = append(newExtensions, ext)
					}
				}
				cfg.Services.Database.Extensions = newExtensions
				fmt.Printf("\n%s Vector extension will be removed safely from PostgreSQL.\n", infoColor("ℹ"))
				fmt.Println("Your data will remain intact.")
			}

		case "database":
			// Clear entire database config (including vector if present)
			cfg.Services.Database = config.DatabaseConfig{}

		case "cache":
			cfg.Services.Cache = config.CacheConfig{}

		case "queue":
			cfg.Services.Queue = config.QueueConfig{}

		case "storage":
			cfg.Services.Storage = config.StorageConfig{}

		case "mongodb":
			cfg.Services.MongoDB = config.MongoDBConfig{}

		case "stt":
			cfg.Services.Whisper = config.WhisperConfig{}
		}
	}

	// If no models left, clear AI service
	if len(cfg.Services.AI.Models) == 0 {
		cfg.Services.AI = config.AIConfig{}
	}
}

// progressBar function is already defined in models.go, no need to duplicate it here
