// internal/cli/models.go
package cli

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/briandowns/spinner"
	"github.com/localcloud/localcloud/internal/config"
	"github.com/localcloud/localcloud/internal/models"
	"github.com/spf13/cobra"
)

var modelsCmd = &cobra.Command{
	Use:     "models",
	Short:   "Manage AI models",
	Aliases: []string{"model", "m"},
	Long: `Manage AI models for LocalCloud.
	
Download, list, and remove AI models. Models are stored locally and can be used
across all your LocalCloud projects.`,
}

var modelsListCmd = &cobra.Command{
	Use:     "list",
	Short:   "List available models",
	Aliases: []string{"ls"},
	Long:    `List all locally available AI models and recommended models to download.`,
	RunE:    runModelsList,
}

var modelsPullCmd = &cobra.Command{
	Use:     "pull [model-name]",
	Short:   "Download a model",
	Aliases: []string{"download", "get"},
	Long:    `Download an AI model from the Ollama library.`,
	Args:    cobra.ExactArgs(1),
	RunE:    runModelsPull,
}

var modelsRemoveCmd = &cobra.Command{
	Use:     "remove [model-name]",
	Short:   "Remove a model",
	Aliases: []string{"rm", "delete"},
	Long:    `Remove a locally stored AI model.`,
	Args:    cobra.ExactArgs(1),
	RunE:    runModelsRemove,
}

var modelsInfoCmd = &cobra.Command{
	Use:   "info",
	Short: "Show model information",
	Long:  `Display information about available models and AI provider status.`,
	RunE:  runModelsInfo,
}

func init() {
	modelsCmd.AddCommand(modelsListCmd)
	modelsCmd.AddCommand(modelsPullCmd)
	modelsCmd.AddCommand(modelsRemoveCmd)
	modelsCmd.AddCommand(modelsInfoCmd)
}

func runModelsList(cmd *cobra.Command, args []string) error {
	cfg := config.Get()
	manager := models.NewManager(fmt.Sprintf("http://localhost:%d", cfg.Services.AI.Port))

	// Check if Ollama is available
	if !manager.IsOllamaAvailable() {
		printWarning("Ollama service is not running. Start it with 'lc start'")
		fmt.Println()
		showRecommendedModels()
		return nil
	}

	// Get installed models
	modelList, err := manager.List()
	if err != nil {
		return fmt.Errorf("failed to list models: %w", err)
	}

	if len(modelList) == 0 {
		printInfo("No models installed yet")
		fmt.Println()
		showRecommendedModels()
		return nil
	}

	// Display installed models
	fmt.Println("\nInstalled Models:")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Printf("%-20s %-10s %s\n", "MODEL", "SIZE", "MODIFIED")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	for _, model := range modelList {
		size := FormatBytes(model.Size)
		modified := model.ModifiedAt.Format("2006-01-02 15:04")

		// Highlight active model
		name := model.Name
		if name == cfg.Services.AI.Default {
			name = successColor(name + " ✓")
		}

		fmt.Printf("%-20s %-10s %s\n", name, size, modified)
	}

	fmt.Println()

	// Show recommended models not yet installed
	showRecommendedModelsNotInstalled(modelList)

	return nil
}

func runModelsPull(cmd *cobra.Command, args []string) error {
	modelName := args[0]

	cfg := config.Get()
	manager := models.NewManager(fmt.Sprintf("http://localhost:%d", cfg.Services.AI.Port))

	// Check if Ollama is available
	if !manager.IsOllamaAvailable() {
		return fmt.Errorf("Ollama service is not running. Start it with 'lc start'")
	}

	// Check if model already exists
	existingModels, _ := manager.List()
	for _, m := range existingModels {
		if m.Name == modelName {
			printInfo(fmt.Sprintf("Model '%s' is already installed", modelName))
			return nil
		}
	}

	printInfo(fmt.Sprintf("Pulling model '%s'...", modelName))
	fmt.Println("This may take a few minutes depending on the model size and your internet connection.")

	// Create progress channel
	progress := make(chan models.PullProgress)
	done := make(chan error)

	// Start pull in goroutine
	go func() {
		done <- manager.Pull(modelName, progress)
	}()

	// Progress bar
	s := spinner.New(spinner.CharSets[14], 100)
	s.Suffix = " Connecting..."
	s.Start()

	var lastStatus string
	startTime := time.Now()

	for {
		select {
		case p, ok := <-progress:
			if !ok {
				s.Stop()
				goto finished
			}

			// Update spinner based on status
			if p.Status != lastStatus {
				s.Stop()

				switch p.Status {
				case "pulling manifest":
					s.Suffix = " Fetching manifest..."
				case "downloading":
					s.Suffix = " Starting download..."
				case "verifying":
					s.Suffix = " Verifying..."
				case "writing manifest":
					s.Suffix = " Finalizing..."
				case "success":
					printSuccess("Model pulled successfully!")
					continue
				default:
					s.Suffix = fmt.Sprintf(" %s...", p.Status)
				}

				s.Start()
				lastStatus = p.Status
			}

			// Show download progress
			if p.Total > 0 && p.Status == "downloading" {
				s.Stop()

				// Calculate speed
				elapsed := time.Since(startTime).Seconds()
				speed := float64(p.Completed) / elapsed / 1024 / 1024 // MB/s

				// Progress bar
				barWidth := 30
				filled := int(float64(barWidth) * float64(p.Completed) / float64(p.Total))
				bar := strings.Repeat("=", filled) + strings.Repeat("-", barWidth-filled)

				fmt.Printf("\r[%s] %d%% | %s / %s | %.1f MB/s",
					bar,
					p.Percentage,
					FormatBytes(p.Completed),
					FormatBytes(p.Total),
					speed,
				)
			}

		case err := <-done:
			s.Stop()
			if err != nil {
				return fmt.Errorf("failed to pull model: %w", err)
			}
			goto finished
		}
	}

finished:
	fmt.Println() // New line after progress
	duration := time.Since(startTime)
	printSuccess(fmt.Sprintf("Model '%s' pulled successfully in %s!", modelName, duration.Round(time.Second)))

	return nil
}

func runModelsRemove(cmd *cobra.Command, args []string) error {
	modelName := args[0]

	cfg := config.Get()
	manager := models.NewManager(fmt.Sprintf("http://localhost:%d", cfg.Services.AI.Port))

	// Check if Ollama is available
	if !manager.IsOllamaAvailable() {
		return fmt.Errorf("Ollama service is not running. Start it with 'lc start'")
	}

	// Confirm removal
	fmt.Printf("Are you sure you want to remove model '%s'? [y/N]: ", modelName)
	var response string
	fmt.Scanln(&response)

	if strings.ToLower(response) != "y" {
		printInfo("Removal cancelled")
		return nil
	}

	// Remove model
	s := spinner.New(spinner.CharSets[14], 100)
	s.Suffix = fmt.Sprintf(" Removing %s...", modelName)
	s.Start()

	if err := manager.Remove(modelName); err != nil {
		s.Stop()
		return fmt.Errorf("failed to remove model: %w", err)
	}

	s.Stop()
	printSuccess(fmt.Sprintf("Model '%s' removed successfully", modelName))

	return nil
}

func runModelsInfo(cmd *cobra.Command, args []string) error {
	cfg := config.Get()
	manager := models.NewManager(fmt.Sprintf("http://localhost:%d", cfg.Services.AI.Port))

	fmt.Println("\nAI Provider Information:")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	// Check providers
	provider := manager.DetectProvider()

	// Ollama status
	if manager.IsOllamaAvailable() {
		fmt.Printf("Ollama Status: %s\n", successColor("Running ✓"))
		fmt.Printf("Ollama Endpoint: %s\n", infoColor(fmt.Sprintf("http://localhost:%d", cfg.Services.AI.Port)))

		// Count models
		modelList, _ := manager.List()
		fmt.Printf("Installed Models: %s\n", infoColor(fmt.Sprintf("%d", len(modelList))))
	} else {
		fmt.Printf("Ollama Status: %s\n", errorColor("Not Running ✗"))
	}

	// OpenAI status
	if os.Getenv("OPENAI_API_KEY") != "" {
		fmt.Printf("\nOpenAI Status: %s\n", successColor("API Key Found ✓"))
		fmt.Printf("API Key: %s\n", infoColor(maskAPIKey(os.Getenv("OPENAI_API_KEY"))))
	} else {
		fmt.Printf("\nOpenAI Status: %s\n", warningColor("No API Key"))
	}

	// Active provider
	fmt.Printf("\nActive Provider: %s\n", infoColor(string(provider)))

	// Configuration
	fmt.Println("\nConfiguration:")
	fmt.Printf("Default Model: %s\n", infoColor(cfg.Services.AI.Default))
	fmt.Printf("Configured Models: %s\n", infoColor(strings.Join(cfg.Services.AI.Models, ", ")))

	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	return nil
}

// Helper functions

func showRecommendedModels() {
	fmt.Println("Recommended Models:")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Printf("%-20s %-10s %s\n", "MODEL", "SIZE", "DESCRIPTION")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	for _, model := range models.GetRecommendedModels() {
		fmt.Printf("%-20s %-10s %s\n",
			infoColor(model.Name),
			model.Size,
			model.Description,
		)
	}

	fmt.Println("\nTo download a model, run:")
	fmt.Println("  lc models pull <model-name>")
}

func showRecommendedModelsNotInstalled(installed []models.Model) {
	recommended := models.GetRecommendedModels()
	notInstalled := []struct {
		Name        string
		Size        string
		Description string
	}{}

	// Find recommended models not installed
	for _, rec := range recommended {
		found := false
		for _, inst := range installed {
			if inst.Name == rec.Name || inst.Model == rec.Name {
				found = true
				break
			}
		}
		if !found {
			notInstalled = append(notInstalled, rec)
		}
	}

	if len(notInstalled) > 0 {
		fmt.Println("Recommended models to try:")
		for _, model := range notInstalled {
			fmt.Printf("  • %s (%s) - %s\n",
				infoColor(model.Name),
				model.Size,
				model.Description,
			)
		}
		fmt.Println("\nTo download: lc models pull <model-name>")
	}
}

func maskAPIKey(key string) string {
	if len(key) <= 8 {
		return "***"
	}
	return key[:4] + "..." + key[len(key)-4:]
}
