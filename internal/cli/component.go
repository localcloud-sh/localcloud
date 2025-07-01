// Package cli implements the command-line interface for LocalCloud
package cli

import (
	"fmt"
	"strings"

	"github.com/localcloud/localcloud/internal/components"
	"github.com/localcloud/localcloud/internal/config"
	"github.com/localcloud/localcloud/internal/models"
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
	componentCmd.AddCommand(componentUpdateCmd) // Add update command
	componentCmd.AddCommand(componentInfoCmd)
}

func runComponentList(cmd *cobra.Command, args []string) error {
	// Check if project is initialized
	if !IsProjectInitialized() {
		return fmt.Errorf("no LocalCloud project found. Run 'lc init' first")
	}

	cfg := config.Get()
	if cfg == nil {
		return fmt.Errorf("failed to load configuration")
	}

	// Get enabled components from config
	enabledComponents := getEnabledComponents(cfg)
	enabledMap := make(map[string]bool)
	for _, comp := range enabledComponents {
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

// internal/cli/component.go - Updated runComponentAdd function

func runComponentAdd(cmd *cobra.Command, args []string) error {
	componentID := args[0]

	// Check if project is initialized
	if !IsProjectInitialized() {
		return fmt.Errorf("no LocalCloud project found. Run 'lc init' first")
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

	// Check if already enabled
	enabledComponents := getEnabledComponents(cfg)
	for _, enabled := range enabledComponents {
		if enabled == componentID {
			printWarning(fmt.Sprintf("Component '%s' is already enabled", componentID))
			return nil
		}
	}

	// Update configuration based on component
	if err := enableComponent(cfg, comp); err != nil {
		return err
	}

	// Save configuration first
	if err := config.Save(); err != nil {
		return fmt.Errorf("failed to save configuration: %w", err)
	}

	printSuccess(fmt.Sprintf("Added component: %s", comp.Name))

	// If component has models, run interactive model selection
	if len(comp.Models) > 0 && (comp.ID == "llm" || comp.ID == "embedding") {
		fmt.Println()

		// Ask if they want to select a model now
		fmt.Printf("Would you like to select a model now? (Y/n): ")
		var response string
		fmt.Scanln(&response)

		if response == "" || strings.ToLower(response) == "y" {
			// Create Ollama manager
			manager := models.NewManager("http://localhost:11434")

			// Select model based on component type
			var selectedModel string
			if comp.ID == "embedding" {
				selectedModel, err = selectEmbeddingModel(manager)
			} else {
				selectedModel, err = selectComponentModel(comp, manager)
			}

			if err != nil {
				printWarning(fmt.Sprintf("Model selection cancelled: %v", err))
			} else if selectedModel != "" {
				// Update config with selected model
				v := config.GetViper()
				currentModels := cfg.Services.AI.Models

				// Add model if not already in list
				modelExists := false
				for _, m := range currentModels {
					if m == selectedModel {
						modelExists = true
						break
					}
				}

				if !modelExists {
					currentModels = append(currentModels, selectedModel)
					v.Set("services.ai.models", currentModels)

					// Set as default if it's the first model
					if len(currentModels) == 1 {
						v.Set("services.ai.default", selectedModel)
					}

					// Save updated config
					if err := config.Save(); err != nil {
						printWarning("Failed to save model selection")
					} else {
						printSuccess(fmt.Sprintf("Model '%s' configured", selectedModel))
					}
				}
			}
		}
	}

	// Show next steps
	fmt.Println("\nNext steps:")

	// Show restart command for AI components
	if comp.ID == "llm" || comp.ID == "embedding" {
		fmt.Println("  • Restart services: lc restart")
	} else if len(comp.Services) > 0 {
		fmt.Printf("  • Start the service: lc start %s\n", comp.Services[0])
	}

	return nil
}

func runComponentRemove(cmd *cobra.Command, args []string) error {
	componentID := args[0]

	// Check if project is initialized
	if !IsProjectInitialized() {
		return fmt.Errorf("no LocalCloud project found. Run 'lc init' first")
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
	enabledComponents := getEnabledComponents(cfg)
	found := false
	for _, enabled := range enabledComponents {
		if enabled == componentID {
			found = true
			break
		}
	}

	if !found {
		printWarning(fmt.Sprintf("Component '%s' is not enabled", componentID))
		return nil
	}

	// Update configuration
	if err := disableComponent(cfg, comp); err != nil {
		return err
	}

	// Save configuration
	if err := config.Save(); err != nil {
		return fmt.Errorf("failed to save configuration: %w", err)
	}

	printSuccess(fmt.Sprintf("Removed component: %s", comp.Name))

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

	return nil
}

// Helper functions

// internal/cli/component.go - Update getEnabledComponents function

// internal/cli/component.go - Replace getEnabledComponents function

// getEnabledComponents returns list of enabled component IDs from config
func getEnabledComponents(cfg *config.Config) []string {
	var components []string

	// Check AI services (LLM and embedding use same service)
	// Only consider it enabled if models are actually configured
	if cfg.Services.AI.Port > 0 && len(cfg.Services.AI.Models) > 0 {
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

	// Check database - only if actually configured with a type
	if cfg.Services.Database.Type != "" && cfg.Services.Database.Port > 0 {
		// Check for pgvector
		for _, ext := range cfg.Services.Database.Extensions {
			if ext == "pgvector" {
				components = appendUnique(components, "vector")
				break
			}
		}
	}

	// Check cache - only if type is set
	if cfg.Services.Cache.Type != "" && cfg.Services.Cache.Port > 0 {
		components = appendUnique(components, "cache")
	}

	// Check queue - only if type is set
	if cfg.Services.Queue.Type != "" && cfg.Services.Queue.Port > 0 {
		components = appendUnique(components, "queue")
	}

	// Check storage - only if type is set
	if cfg.Services.Storage.Type != "" && cfg.Services.Storage.Port > 0 {
		components = appendUnique(components, "storage")
	}

	// Check STT/Whisper - only if type is set
	//if cfg.Services.Whisper.Type != "" && cfg.Services.Whisper.Port > 0 {
	//	components = appendUnique(components, "stt")
	//}

	return components
}

// enableComponent updates config to enable a component
func enableComponent(cfg *config.Config, comp components.Component) error {
	v := config.GetViper()

	switch comp.ID {
	case "llm", "embedding":
		// Enable AI service if not already
		if cfg.Services.AI.Port == 0 {
			v.Set("services.ai.port", 11434)
		}

	case "vector":
		// Enable PostgreSQL with pgvector
		v.Set("services.database.type", "postgres")
		v.Set("services.database.port", 5432)
		v.Set("services.database.extensions", []string{"pgvector"})

	case "cache":
		v.Set("services.cache.type", "redis")
		v.Set("services.cache.port", 6379)

	case "queue":
		v.Set("services.queue.type", "redis")
		v.Set("services.queue.port", 6380)

	case "storage":
		v.Set("services.storage.type", "minio")
		v.Set("services.storage.port", 9000)
		v.Set("services.storage.console", 9001)

		//case "stt":
		//	// Enable Whisper service
		//	v.Set("services.whisper.type", "localllama")
		//	v.Set("services.whisper.port", 9000)
		//	// Model will be set during init process
	}

	return nil
}

// Updated disableComponent function to handle AI components
func disableComponent(cfg *config.Config, comp components.Component) error {
	v := config.GetViper()

	switch comp.ID {
	case "llm", "embedding":
		// Remove only the models for this component type
		newModels := []string{}
		removedModel := ""

		for _, model := range cfg.Services.AI.Models {
			if comp.ID == "embedding" && models.IsEmbeddingModel(model) {
				removedModel = model
				continue // Remove embedding models
			} else if comp.ID == "llm" && !models.IsEmbeddingModel(model) {
				removedModel = model
				continue // Remove LLM models
			}
			newModels = append(newModels, model)
		}

		v.Set("services.ai.models", newModels)

		// Update default if needed
		if cfg.Services.AI.Default == removedModel && len(newModels) > 0 {
			v.Set("services.ai.default", newModels[0])
		}

		// If no models left, disable the AI service
		if len(newModels) == 0 {
			v.Set("services.ai.port", 0)
			v.Set("services.ai.default", "")
		}

	case "vector":
		// Remove pgvector extension
		extensions := cfg.Services.Database.Extensions
		newExtensions := []string{}
		for _, ext := range extensions {
			if ext != "pgvector" {
				newExtensions = append(newExtensions, ext)
			}
		}
		v.Set("services.database.extensions", newExtensions)

		// If no extensions left and no other use, disable database
		if len(newExtensions) == 0 {
			// Check if any other component needs database
			// For now, just remove extensions
		}

	case "cache":
		v.Set("services.cache.type", "")
		v.Set("services.cache.port", 0)

	case "queue":
		v.Set("services.queue.type", "")
		v.Set("services.queue.port", 0)

	case "storage":
		v.Set("services.storage.type", "")
		v.Set("services.storage.port", 0)
		v.Set("services.storage.console", 0)

	case "stt":
		v.Set("services.whisper.type", "")
		v.Set("services.whisper.port", 0)
		v.Set("services.whisper.model", "")
	}

	return nil
}

// appendUnique appends a string to slice if not already present
func appendUnique(slice []string, item string) []string {
	for _, existing := range slice {
		if existing == item {
			return slice
		}
	}
	return append(slice, item)
}
func runComponentUpdate(cmd *cobra.Command, args []string) error {
	componentID := args[0]

	// Check if project is initialized
	if !IsProjectInitialized() {
		return fmt.Errorf("no LocalCloud project found. Run 'lc init' first")
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
	enabledComponents := getEnabledComponents(cfg)
	found := false
	for _, enabled := range enabledComponents {
		if enabled == componentID {
			found = true
			break
		}
	}

	if !found {
		printWarning(fmt.Sprintf("Component '%s' is not enabled", componentID))
		return nil
	}

	// Only AI components can be updated (model change)
	if componentID != "llm" && componentID != "embedding" {
		printWarning("Only LLM and embedding components can be updated")
		return nil
	}

	// Find current model
	var currentModel string
	for _, model := range cfg.Services.AI.Models {
		if componentID == "embedding" && models.IsEmbeddingModel(model) {
			currentModel = model
			break
		} else if componentID == "llm" && !models.IsEmbeddingModel(model) {
			currentModel = model
			break
		}
	}

	fmt.Printf("Current %s model: %s\n", componentID, currentModel)
	fmt.Println("\nSelect new model:")

	// Create Ollama manager
	manager := models.NewManager("http://localhost:11434")

	// Select new model
	var selectedModel string
	if componentID == "embedding" {
		selectedModel, err = selectEmbeddingModel(manager)
	} else {
		selectedModel, err = selectComponentModel(comp, manager)
	}

	if err != nil || selectedModel == "" {
		printWarning("Model selection cancelled")
		return nil
	}

	// Update config
	v := config.GetViper()
	newModels := []string{}

	// Keep other models, replace the one for this component
	for _, model := range cfg.Services.AI.Models {
		if componentID == "embedding" && models.IsEmbeddingModel(model) {
			continue // Skip old embedding model
		} else if componentID == "llm" && !models.IsEmbeddingModel(model) {
			continue // Skip old LLM model
		}
		newModels = append(newModels, model)
	}

	// Add new model
	newModels = append(newModels, selectedModel)
	v.Set("services.ai.models", newModels)

	// Update default if it was the old model
	if cfg.Services.AI.Default == currentModel {
		v.Set("services.ai.default", selectedModel)
	}

	// Save configuration
	if err := config.Save(); err != nil {
		return fmt.Errorf("failed to save configuration: %w", err)
	}

	printSuccess(fmt.Sprintf("Updated %s model: %s → %s", componentID, currentModel, selectedModel))

	// Offer to remove old model
	fmt.Printf("\nWould you like to remove the old model '%s'? (y/N): ", currentModel)
	var response string
	fmt.Scanln(&response)

	if strings.ToLower(response) == "y" {
		fmt.Printf("Run: lc models remove %s\n", currentModel)
	}

	fmt.Println("\nNext steps:")
	fmt.Println("  • Restart services: lc restart")

	return nil
}
