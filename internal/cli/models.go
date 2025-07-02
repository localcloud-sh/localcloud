// internal/cli/models.go
package cli

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/briandowns/spinner"
	"github.com/localcloud-sh/localcloud/internal/config"
	"github.com/localcloud-sh/localcloud/internal/models"
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
		printWarning("Ollama service is not running. Start it with: lc start ai")
		fmt.Println()
		showRecommendedModels()
		return nil
	}

	// List installed models
	modelList, err := manager.List()
	if err != nil {
		return fmt.Errorf("failed to list models: %w", err)
	}

	if len(modelList) == 0 {
		fmt.Println("No models installed yet.")
		fmt.Println()
		showRecommendedModels()
		return nil
	}

	// Separate models by type
	var llmModels []models.Model
	var embeddingModels []models.Model

	for _, model := range modelList {
		if models.IsEmbeddingModel(model.Name) {
			embeddingModels = append(embeddingModels, model)
		} else {
			llmModels = append(llmModels, model)
		}
	}

	// Display installed models by type
	fmt.Println("Installed Models:")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	if len(llmModels) > 0 {
		fmt.Println("\n🤖 Language Models:")
		fmt.Printf("%-30s %-12s %-20s\n", "NAME", "SIZE", "MODIFIED")
		fmt.Println(strings.Repeat("─", 62))
		for _, model := range llmModels {
			fmt.Printf("%-30s %-12s %-20s\n",
				model.Name,
				FormatBytes(model.Size),
				model.ModifiedAt.Format("2006-01-02 15:04"),
			)
		}
	}

	if len(embeddingModels) > 0 {
		fmt.Println("\n🔍 Embedding Models:")
		fmt.Printf("%-30s %-12s %-20s\n", "NAME", "SIZE", "MODIFIED")
		fmt.Println(strings.Repeat("─", 62))
		for _, model := range embeddingModels {
			info := models.GetEmbeddingModelInfo(model.Name)
			name := model.Name
			if info != nil && info.Dimensions > 0 {
				name = fmt.Sprintf("%s (%dd)", model.Name, info.Dimensions)
			}
			fmt.Printf("%-30s %-12s %-20s\n",
				name,
				FormatBytes(model.Size),
				model.ModifiedAt.Format("2006-01-02 15:04"),
			)
		}
	}

	// Get active model
	activeModel, _ := manager.GetActiveModel(cfg.Services.AI.Default)
	if activeModel != "" {
		fmt.Printf("\nActive model: %s\n", infoColor(activeModel))
	}

	// Show recommendations
	fmt.Println()
	showRecommendedModelsNotInstalled(modelList)

	return nil
}

func runModelsPull(cmd *cobra.Command, args []string) error {
	modelName := args[0]
	cfg := config.Get()
	manager := models.NewManager(fmt.Sprintf("http://localhost:%d", cfg.Services.AI.Port))

	// Check if Ollama is available
	if !manager.IsOllamaAvailable() {
		return fmt.Errorf("Ollama service is not running. Start it with: lc start ai")
	}

	// Check if model is an embedding model
	isEmbedding := models.IsEmbeddingModel(modelName)
	modelType := "language"
	if isEmbedding {
		modelType = "embedding"
	}

	fmt.Printf("Pulling %s model: %s\n", modelType, modelName)

	// Create progress channel
	progress := make(chan models.PullProgress)
	done := make(chan error)

	// Start pull in goroutine
	go func() {
		done <- manager.Pull(modelName, progress)
	}()

	// Display progress
	s := spinner.New(spinner.CharSets[14], 100*time.Millisecond)
	s.Start()

	for {
		select {
		case p := <-progress:
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

		case err := <-done:
			s.Stop()
			fmt.Println() // New line after progress

			if err != nil {
				return fmt.Errorf("failed to pull model: %w", err)
			}

			printSuccess(fmt.Sprintf("Model '%s' pulled successfully!", modelName))

			// Show usage examples based on model type
			fmt.Println("\nTry it out:")
			if isEmbedding {
				fmt.Println("  # Generate embedding")
				fmt.Printf("  curl http://localhost:%d/api/embeddings \\\n", cfg.Services.AI.Port)
				fmt.Printf("    -d '{\"model\":\"%s\",\"prompt\":\"Hello world\"}'\n", modelName)
				fmt.Println()
				fmt.Println("  # Python example")
				fmt.Println("  import requests")
				fmt.Printf("  resp = requests.post('http://localhost:%d/api/embeddings',\n", cfg.Services.AI.Port)
				fmt.Printf("      json={'model': '%s', 'prompt': 'Hello world'})\n", modelName)
				fmt.Println("  embedding = resp.json()['embedding']")
			} else {
				fmt.Println("  # Chat completion")
				fmt.Printf("  curl http://localhost:%d/api/chat \\\n", cfg.Services.AI.Port)
				fmt.Printf("    -d '{\"model\":\"%s\",\"messages\":[{\"role\":\"user\",\"content\":\"Hello!\"}]}'\n", modelName)
				fmt.Println()
				fmt.Println("  # Generate text")
				fmt.Printf("  curl http://localhost:%d/api/generate \\\n", cfg.Services.AI.Port)
				fmt.Printf("    -d '{\"model\":\"%s\",\"prompt\":\"Once upon a time\"}'\n", modelName)
			}

			// Update config if this is the first model
			if cfg.Services.AI.Default == "" {
				fmt.Printf("\nSetting %s as default model\n", modelName)
				// This would update the config
			}

			return nil
		}
	}
}

func runModelsRemove(cmd *cobra.Command, args []string) error {
	modelName := args[0]
	cfg := config.Get()
	manager := models.NewManager(fmt.Sprintf("http://localhost:%d", cfg.Services.AI.Port))

	// Check if Ollama is available
	if !manager.IsOllamaAvailable() {
		return fmt.Errorf("Ollama service is not running. Start it with: lc start ai")
	}

	// Confirm removal
	fmt.Printf("Are you sure you want to remove model '%s'? (y/N): ", modelName)
	var confirm string
	fmt.Scanln(&confirm)

	if strings.ToLower(confirm) != "y" {
		fmt.Println("Cancelled")
		return nil
	}

	// Remove model
	s := spinner.New(spinner.CharSets[14], 100*time.Millisecond)
	s.Suffix = fmt.Sprintf(" Removing %s...", modelName)
	s.Start()

	err := manager.Remove(modelName)
	s.Stop()

	if err != nil {
		return fmt.Errorf("failed to remove model: %w", err)
	}

	printSuccess(fmt.Sprintf("Model '%s' removed successfully", modelName))
	return nil
}

func runModelsInfo(cmd *cobra.Command, args []string) error {
	cfg := config.Get()
	manager := models.NewManager(fmt.Sprintf("http://localhost:%d", cfg.Services.AI.Port))

	fmt.Println("AI Model Information")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	// Provider detection
	provider := manager.DetectProvider()

	// Ollama status
	if manager.IsOllamaAvailable() {
		fmt.Printf("Ollama Status: %s\n", successColor("Running ✓"))
		fmt.Printf("Endpoint: %s\n", infoColor(fmt.Sprintf("http://localhost:%d", cfg.Services.AI.Port)))

		// Count installed models
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

	fmt.Println("\n🤖 Language Models:")
	fmt.Printf("%-20s %-10s %s\n", "MODEL", "SIZE", "DESCRIPTION")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	for _, model := range models.GetRecommendedModels() {
		fmt.Printf("%-20s %-10s %s\n",
			infoColor(model.Name),
			model.Size,
			model.Description,
		)
	}

	fmt.Println("\n🔍 Embedding Models:")
	fmt.Printf("%-20s %-10s %s\n", "MODEL", "SIZE", "DIMENSIONS")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	for _, model := range models.PredefinedEmbeddingModels {
		fmt.Printf("%-20s %-10s %d dimensions\n",
			infoColor(model.Name),
			model.Size,
			model.Dimensions,
		)
	}

	fmt.Println("\nTo download a model, run:")
	fmt.Println("  lc models pull <model-name>")
}

func showRecommendedModelsNotInstalled(installed []models.Model) {
	// Get recommended LLM models
	recommendedLLMs := models.GetRecommendedModels()
	notInstalledLLMs := []struct {
		Name        string
		Size        string
		Description string
	}{}

	// Find recommended LLMs not installed
	for _, rec := range recommendedLLMs {
		found := false
		for _, inst := range installed {
			if inst.Name == rec.Name || inst.Model == rec.Name {
				found = true
				break
			}
		}
		if !found {
			notInstalledLLMs = append(notInstalledLLMs, rec)
		}
	}

	// Find recommended embedding models not installed
	notInstalledEmbeddings := []models.EmbeddingModel{}
	for _, rec := range models.PredefinedEmbeddingModels {
		found := false
		for _, inst := range installed {
			if inst.Name == rec.Name {
				found = true
				break
			}
		}
		if !found {
			notInstalledEmbeddings = append(notInstalledEmbeddings, rec)
		}
	}

	// Show recommendations if any
	if len(notInstalledLLMs) > 0 || len(notInstalledEmbeddings) > 0 {
		fmt.Println("Recommended models to try:")

		if len(notInstalledLLMs) > 0 {
			fmt.Println("\n🤖 Language Models:")
			for _, model := range notInstalledLLMs[:min(3, len(notInstalledLLMs))] {
				fmt.Printf("  • %s (%s) - %s\n",
					infoColor(model.Name),
					model.Size,
					model.Description,
				)
			}
		}

		if len(notInstalledEmbeddings) > 0 {
			fmt.Println("\n🔍 Embedding Models:")
			for _, model := range notInstalledEmbeddings[:min(3, len(notInstalledEmbeddings))] {
				fmt.Printf("  • %s (%s) - %d dimensions\n",
					infoColor(model.Name),
					model.Size,
					model.Dimensions,
				)
			}
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

func progressBar(percentage int, width int) string {
	filled := (percentage * width) / 100
	bar := strings.Repeat("=", filled)
	if filled < width {
		bar += ">"
		bar += strings.Repeat(" ", width-filled-1)
	}
	return bar
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
