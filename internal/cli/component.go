// internal/cli/component.go
package cli

import (
	"fmt"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/localcloud-sh/localcloud/internal/components"
	"github.com/localcloud-sh/localcloud/internal/config"
	"github.com/localcloud-sh/localcloud/internal/models"
	"github.com/spf13/cobra"
)

var componentCmd = &cobra.Command{
	Use:     "component",
	Aliases: []string{"comp"},
	Short:   "Manage project components",
	Long:    `Add, remove, or list components in your LocalCloud project.`,
}

var componentListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List available components",
	Long:    `Display all available components and their status.`,
	RunE:    runComponentList,
}

var componentAddCmd = &cobra.Command{
	Use:   "add [component-id]",
	Short: "Add a component to the project",
	Long:  `Add a new component to your LocalCloud project.`,
	Args:  cobra.ExactArgs(1),
	RunE:  runComponentAdd,
}

var componentRemoveCmd = &cobra.Command{
	Use:     "remove [component-id]",
	Aliases: []string{"rm"},
	Short:   "Remove a component from the project",
	Long:    `Remove a component from your LocalCloud project.`,
	Args:    cobra.ExactArgs(1),
	RunE:    runComponentRemove,
}

var componentInfoCmd = &cobra.Command{
	Use:   "info [component-id]",
	Short: "Show component details",
	Long:  `Display detailed information about a specific component.`,
	Args:  cobra.ExactArgs(1),
	RunE:  runComponentInfo,
}

var componentUpdateCmd = &cobra.Command{
	Use:   "update [component-id]",
	Short: "Update component configuration (e.g., change model)",
	Long:  `Update a component's configuration, such as changing the AI model for LLM or embedding components.`,
	Args:  cobra.ExactArgs(1),
	RunE:  runComponentUpdate,
}

func init() {
	componentCmd.AddCommand(componentListCmd)
	componentCmd.AddCommand(componentAddCmd)
	componentCmd.AddCommand(componentRemoveCmd)
	componentCmd.AddCommand(componentUpdateCmd)
	componentCmd.AddCommand(componentInfoCmd)
}

func runComponentList(cmd *cobra.Command, args []string) error {
	// Check if project is initialized
	if !IsProjectInitialized() {
		return fmt.Errorf("no LocalCloud project found. Run 'lc setup' first")
	}

	cfg := config.Get()
	if cfg == nil {
		return fmt.Errorf("failed to load configuration")
	}

	// Get enabled components from project.components
	enabledMap := make(map[string]bool)
	for _, comp := range cfg.Project.Components {
		enabledMap[comp] = true
	}

	fmt.Println("Available Components:")
	fmt.Println(strings.Repeat("━", 70))
	fmt.Printf("%-15s %-25s %-10s %s\n", "ID", "NAME", "STATUS", "DESCRIPTION")
	fmt.Println(strings.Repeat("─", 70))

	// Group by category
	categories := []string{"ai", "database", "infrastructure"}
	for _, category := range categories {
		comps := components.GetComponentsByCategory(category)
		if len(comps) == 0 {
			continue
		}

		// Category header
		fmt.Println()
		fmt.Printf("%s:\n", strings.Title(category))

		for _, comp := range comps {
			status := "Disabled"
			statusColor := errorColor
			if enabledMap[comp.ID] {
				status = "Enabled"
				statusColor = successColor
			}

			fmt.Printf("%-15s %-25s %-10s %s\n",
				comp.ID,
				comp.Name,
				statusColor(status),
				truncateString(comp.Description, 35))
		}
	}

	fmt.Println(strings.Repeat("━", 70))
	fmt.Println("\nTo add a component: lc component add <component-id>")

	return nil
}

func runComponentAdd(cmd *cobra.Command, args []string) error {
	componentID := args[0]

	// Check if project is initialized
	if !IsProjectInitialized() {
		return fmt.Errorf("no LocalCloud project found. Run 'lc setup' first")
	}

	// Validate component exists
	comp, err := components.GetComponent(componentID)
	if err != nil {
		return fmt.Errorf("unknown component: %s", componentID)
	}

	// Load config
	cfg := config.Get()
	if cfg == nil {
		return fmt.Errorf("failed to load configuration")
	}

	// Check if already enabled in project.components
	for _, enabled := range cfg.Project.Components {
		if enabled == componentID {
			printWarning(fmt.Sprintf("Component '%s' is already enabled", componentID))
			return nil
		}
	}

	// Check dependencies
	deps := components.GetComponentDependencies(componentID)
	missingDeps := []string{}
	for _, dep := range deps {
		found := false
		for _, enabled := range cfg.Project.Components {
			if enabled == dep {
				found = true
				break
			}
		}
		if !found {
			missingDeps = append(missingDeps, dep)
		}
	}

	if len(missingDeps) > 0 {
		fmt.Printf("\n%s Component '%s' requires: %s\n",
			warningColor("⚠"),
			componentID,
			strings.Join(missingDeps, ", "))

		var confirm bool
		prompt := &survey.Confirm{
			Message: "Add required components?",
			Default: true,
		}
		if err := survey.AskOne(prompt, &confirm); err != nil {
			return err
		}

		if !confirm {
			return fmt.Errorf("cannot add %s without required components", componentID)
		}

		// Add missing dependencies first
		for _, dep := range missingDeps {
			depComp, _ := components.GetComponent(dep)
			if err := addComponent(cfg, depComp); err != nil {
				return fmt.Errorf("failed to add dependency %s: %w", dep, err)
			}
			printSuccess(fmt.Sprintf("Added required component: %s", depComp.Name))
		}
	}

	// Add the component
	if err := addComponent(cfg, comp); err != nil {
		return err
	}

	// Save configuration
	if err := config.Save(); err != nil {
		return fmt.Errorf("failed to save configuration: %w", err)
	}

	printSuccess(fmt.Sprintf("Added component: %s", comp.Name))

	// Handle model selection for AI components
	if components.IsAIComponent(componentID) && len(comp.Models) > 0 {
		fmt.Println()
		var confirm bool
		prompt := &survey.Confirm{
			Message: "Would you like to select a model now?",
			Default: true,
		}
		if err := survey.AskOne(prompt, &confirm); err == nil && confirm {
			if err := selectAndConfigureModel(cfg, comp); err != nil {
				printWarning(fmt.Sprintf("Model selection failed: %v", err))
			}
		}
	}

	// Show next steps
	showComponentNextSteps(comp)

	return nil
}

func runComponentRemove(cmd *cobra.Command, args []string) error {
	componentID := args[0]

	// Check if project is initialized
	if !IsProjectInitialized() {
		return fmt.Errorf("no LocalCloud project found. Run 'lc setup' first")
	}

	// Validate component exists
	comp, err := components.GetComponent(componentID)
	if err != nil {
		return fmt.Errorf("unknown component: %s", componentID)
	}

	// Load config
	cfg := config.Get()
	if cfg == nil {
		return fmt.Errorf("failed to load configuration")
	}

	// Check if enabled
	found := false
	for _, enabled := range cfg.Project.Components {
		if enabled == componentID {
			found = true
			break
		}
	}

	if !found {
		printWarning(fmt.Sprintf("Component '%s' is not enabled", componentID))
		return nil
	}

	// Check for dependent components
	dependents := components.GetDependentComponents(componentID, cfg.Project.Components)
	if len(dependents) > 0 {
		fmt.Printf("\n%s The following components depend on %s: %s\n",
			warningColor("⚠"),
			componentID,
			strings.Join(dependents, ", "))

		var confirm bool
		prompt := &survey.Confirm{
			Message: "Remove dependent components too?",
			Default: false,
		}
		if err := survey.AskOne(prompt, &confirm); err != nil {
			return err
		}

		if !confirm {
			return fmt.Errorf("cannot remove %s while dependent components are enabled", componentID)
		}

		// Remove dependents first
		for _, dep := range dependents {
			depComp, _ := components.GetComponent(dep)
			if err := removeComponent(cfg, depComp); err != nil {
				return fmt.Errorf("failed to remove dependent %s: %w", dep, err)
			}
			printSuccess(fmt.Sprintf("Removed dependent component: %s", depComp.Name))
		}
	}

	// Confirm removal
	var confirm bool
	confirmPrompt := &survey.Confirm{
		Message: fmt.Sprintf("Remove component '%s'?", comp.Name),
		Default: true,
	}
	if err := survey.AskOne(confirmPrompt, &confirm); err != nil {
		return err
	}
	if !confirm {
		return nil
	}

	// Remove the component
	if err := removeComponent(cfg, comp); err != nil {
		return err
	}

	// Save configuration
	if err := config.Save(); err != nil {
		return fmt.Errorf("failed to save configuration: %w", err)
	}

	printSuccess(fmt.Sprintf("Removed component: %s", comp.Name))

	fmt.Println("\nNext steps:")
	fmt.Println("  • Restart services to apply changes: lc restart")

	return nil
}

func runComponentInfo(cmd *cobra.Command, args []string) error {
	componentID := args[0]

	// Get component
	comp, err := components.GetComponent(componentID)
	if err != nil {
		return fmt.Errorf("unknown component: %s", componentID)
	}

	// Display component information
	fmt.Printf("\n%s\n", comp.Name)
	fmt.Println(strings.Repeat("─", len(comp.Name)))
	fmt.Printf("ID: %s\n", comp.ID)
	fmt.Printf("Category: %s\n", strings.Title(comp.Category))
	fmt.Printf("Description: %s\n", comp.Description)

	// Resource requirements
	fmt.Printf("\nResource Requirements:\n")
	fmt.Printf("  Minimum RAM: %s\n", FormatBytes(comp.MinRAM))

	// Required services
	if len(comp.Services) > 0 {
		fmt.Printf("\nRequired Services:\n")
		for _, service := range comp.Services {
			fmt.Printf("  • %s\n", service)
		}
	}

	// Dependencies
	deps := components.GetComponentDependencies(comp.ID)
	if len(deps) > 0 {
		fmt.Printf("\nDepends On:\n")
		for _, dep := range deps {
			if depComp, err := components.GetComponent(dep); err == nil {
				fmt.Printf("  • %s (%s)\n", depComp.Name, dep)
			}
		}
	}

	// Available models
	if len(comp.Models) > 0 {
		fmt.Printf("\nAvailable Models:\n")
		for _, model := range comp.Models {
			fmt.Printf("  • %s (%s)", model.Name, model.Size)
			if model.Default {
				fmt.Print(" [Recommended]")
			}
			if model.Dimensions > 0 {
				fmt.Printf(" - %d dimensions", model.Dimensions)
			}
			fmt.Println()
		}
	}

	// Additional configuration
	if len(comp.Config) > 0 {
		fmt.Printf("\nAdditional Configuration:\n")
		for key, value := range comp.Config {
			fmt.Printf("  %s: %v\n", key, value)
		}
	}

	// Check if enabled
	if cfg := config.Get(); cfg != nil {
		enabled := false
		for _, id := range cfg.Project.Components {
			if id == comp.ID {
				enabled = true
				break
			}
		}

		fmt.Printf("\nStatus: ")
		if enabled {
			fmt.Println(successColor("Enabled"))

			// Show current model for AI components
			if components.IsAIComponent(comp.ID) {
				for _, model := range cfg.Services.AI.Models {
					isEmbedding := models.IsEmbeddingModel(model)
					if (comp.ID == "embedding" && isEmbedding) ||
						(comp.ID == "llm" && !isEmbedding) {
						fmt.Printf("Current Model: %s\n", model)
						break
					}
				}
			}
		} else {
			fmt.Println(errorColor("Disabled"))
		}
	}

	return nil
}

func runComponentUpdate(cmd *cobra.Command, args []string) error {
	componentID := args[0]

	// Check if project is initialized
	if !IsProjectInitialized() {
		return fmt.Errorf("no LocalCloud project found. Run 'lc setup' first")
	}

	// Validate component exists
	comp, err := components.GetComponent(componentID)
	if err != nil {
		return fmt.Errorf("unknown component: %s", componentID)
	}

	// Only AI components can be updated
	if !components.IsAIComponent(componentID) {
		return fmt.Errorf("only AI components (llm, embedding, stt) can be updated")
	}

	// Load config
	cfg := config.Get()
	if cfg == nil {
		return fmt.Errorf("failed to load configuration")
	}

	// Check if enabled
	enabled := false
	for _, id := range cfg.Project.Components {
		if id == componentID {
			enabled = true
			break
		}
	}

	if !enabled {
		return fmt.Errorf("component '%s' is not enabled", componentID)
	}

	// Find current model
	var currentModel string
	for _, model := range cfg.Services.AI.Models {
		isEmbedding := models.IsEmbeddingModel(model)
		if (componentID == "embedding" && isEmbedding) ||
			(componentID == "llm" && !isEmbedding) {
			currentModel = model
			break
		}
	}

	if currentModel == "" {
		fmt.Printf("No model currently configured for %s\n", componentID)
	} else {
		fmt.Printf("Current %s model: %s\n", componentID, currentModel)
	}

	// Select new model
	if err := selectAndConfigureModel(cfg, comp); err != nil {
		return err
	}

	// Show next steps
	fmt.Println("\nNext steps:")
	fmt.Println("  • Restart services to apply changes: lc restart")

	if currentModel != "" {
		fmt.Printf("  • Remove old model if no longer needed: lc models remove %s\n", currentModel)
	}

	return nil
}

// Helper functions

// addComponent adds a component to the configuration
func addComponent(cfg *config.Config, comp components.Component) error {
	// Add to project.components
	cfg.Project.Components = append(cfg.Project.Components, comp.ID)

	// Configure the service
	switch comp.ID {
	case "llm", "embedding", "stt":
		if cfg.Services.AI.Port == 0 {
			cfg.Services.AI = config.AIConfig{
				Port:    11434,
				Models:  []string{},
				Default: "",
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
		// Ensure database is configured
		if cfg.Services.Database.Type == "" {
			cfg.Services.Database = config.DatabaseConfig{
				Type:       "postgres",
				Version:    "16",
				Port:       5432,
				Extensions: []string{"pgvector"},
			}
		} else {
			// Add pgvector extension
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
			AppendOnly:      true,
			AppendFsync:     "everysec",
		}

	case "storage":
		cfg.Services.Storage = config.StorageConfig{
			Type:    "minio",
			Port:    9000,
			Console: 9001,
		}
	}

	return nil
}

// removeComponent removes a component from the configuration
func removeComponent(cfg *config.Config, comp components.Component) error {
	// Remove from project.components
	newComponents := []string{}
	for _, c := range cfg.Project.Components {
		if c != comp.ID {
			newComponents = append(newComponents, c)
		}
	}
	cfg.Project.Components = newComponents

	// Clear service configuration
	switch comp.ID {
	case "llm", "embedding", "stt":
		// Remove models for this component type
		newModels := []string{}
		for _, model := range cfg.Services.AI.Models {
			isEmbedding := models.IsEmbeddingModel(model)
			if (comp.ID == "embedding" && !isEmbedding) ||
				(comp.ID == "llm" && isEmbedding) ||
				(comp.ID == "stt") {
				newModels = append(newModels, model)
			}
		}
		cfg.Services.AI.Models = newModels

		// Clear AI service if no models left
		if len(newModels) == 0 {
			cfg.Services.AI = config.AIConfig{}
		}

	case "vector":
		// Remove pgvector extension
		newExtensions := []string{}
		for _, ext := range cfg.Services.Database.Extensions {
			if ext != "pgvector" {
				newExtensions = append(newExtensions, ext)
			}
		}
		cfg.Services.Database.Extensions = newExtensions

	case "database":
		// Only clear if vector is not enabled
		hasVector := false
		for _, c := range newComponents {
			if c == "vector" {
				hasVector = true
				break
			}
		}
		if !hasVector {
			cfg.Services.Database = config.DatabaseConfig{}
		}

	case "mongodb":
		cfg.Services.MongoDB = config.MongoDBConfig{}

	case "cache":
		cfg.Services.Cache = config.CacheConfig{}

	case "queue":
		cfg.Services.Queue = config.QueueConfig{}

	case "storage":
		cfg.Services.Storage = config.StorageConfig{}
	}

	return nil
}

// selectAndConfigureModel handles model selection for AI components
func selectAndConfigureModel(cfg *config.Config, comp components.Component) error {
	manager := models.NewManager("http://localhost:11434")

	var selectedModel string
	var err error

	if comp.ID == "embedding" {
		selectedModel, err = selectEmbeddingModel(manager)
	} else {
		selectedModel, err = selectComponentModel(comp, manager)
	}

	if err != nil || selectedModel == "" {
		return fmt.Errorf("model selection cancelled")
	}

	// Update models list
	newModels := []string{}

	// Keep models for other component types
	for _, model := range cfg.Services.AI.Models {
		isEmbedding := models.IsEmbeddingModel(model)
		if (comp.ID == "embedding" && !isEmbedding) ||
			(comp.ID == "llm" && isEmbedding) {
			newModels = append(newModels, model)
		}
	}

	// Add the new model
	newModels = append(newModels, selectedModel)
	cfg.Services.AI.Models = newModels

	// Update default if needed
	if comp.ID == "llm" && cfg.Services.AI.Default == "" {
		cfg.Services.AI.Default = selectedModel
	}

	// Save configuration
	if err := config.Save(); err != nil {
		return fmt.Errorf("failed to save model configuration: %w", err)
	}

	printSuccess(fmt.Sprintf("Configured %s model: %s", comp.ID, selectedModel))
	return nil
}

// showComponentNextSteps shows next steps after adding a component
func showComponentNextSteps(comp components.Component) {
	fmt.Println("\nNext steps:")

	if len(comp.Services) > 0 {
		fmt.Println("  • Restart services to apply changes: lc restart")
	}

	if comp.ID == "vector" {
		fmt.Println("  • Create vector table: lc db exec \"CREATE EXTENSION IF NOT EXISTS vector;\"")
	}

	if comp.ID == "storage" {
		fmt.Println("  • Access MinIO console: lc service url minio-console")
	}
}

// getEnabledComponents returns list of enabled component IDs from config
// This uses project.components as the source of truth
func getEnabledComponents(cfg *config.Config) []string {
	if cfg == nil || cfg.Project.Components == nil {
		return []string{}
	}

	// Return the project.components array directly - this is the source of truth
	return cfg.Project.Components
}
