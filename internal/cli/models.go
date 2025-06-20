// internal/cli/models.go
package cli

import (
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/briandowns/spinner"
	"github.com/spf13/cobra"
)

var modelsCmd = &cobra.Command{
	Use:   "models",
	Short: "Manage AI models",
	Long:  `Download, list, and manage AI models for use with LocalCloud.`,
}

var modelsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List available AI models",
	Long:  `Display all available AI models that can be used with LocalCloud.`,
	RunE:  runModelsList,
}

var modelsPullCmd = &cobra.Command{
	Use:   "pull [model]",
	Short: "Download an AI model",
	Long:  `Download an AI model to use with LocalCloud. Models are cached locally.`,
	Args:  cobra.ExactArgs(1),
	RunE:  runModelsPull,
	Example: `  localcloud models pull qwen2.5:3b
  localcloud models pull deepseek-coder:1.3b`,
}

var modelsRemoveCmd = &cobra.Command{
	Use:   "remove [model]",
	Short: "Remove a downloaded model",
	Long:  `Remove a downloaded AI model to free up disk space.`,
	Args:  cobra.ExactArgs(1),
	RunE:  runModelsRemove,
}

func init() {
	// Add subcommands to models command
	modelsCmd.AddCommand(modelsListCmd)
	modelsCmd.AddCommand(modelsPullCmd)
	modelsCmd.AddCommand(modelsRemoveCmd)
}

func runModelsList(cmd *cobra.Command, args []string) error {
	// Available models data
	models := []struct {
		name        string
		size        string
		description string
		downloaded  bool
	}{
		{
			name:        "qwen2.5:3b",
			size:        "2.3GB",
			description: "Fast general purpose model",
			downloaded:  true,
		},
		{
			name:        "deepseek-coder:1.3b",
			size:        "1.5GB",
			description: "Code completion model",
			downloaded:  false,
		},
		{
			name:        "llama3.2:3b",
			size:        "2.0GB",
			description: "Latest Llama model",
			downloaded:  false,
		},
		{
			name:        "phi3:mini",
			size:        "2.3GB",
			description: "Microsoft's efficient model",
			downloaded:  false,
		},
		{
			name:        "mistral:7b",
			size:        "4.1GB",
			description: "Powerful open model",
			downloaded:  false,
		},
		{
			name:        "codellama:7b",
			size:        "3.8GB",
			description: "Meta's code model",
			downloaded:  false,
		},
	}

	// Create a tabwriter for aligned output
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)

	fmt.Println("Available AI Models:")
	fmt.Println(strings.Repeat("─", 70))
	fmt.Fprintln(w, "MODEL\tSIZE\tDESCRIPTION\tSTATUS\t")
	fmt.Fprintln(w, "─────\t────\t───────────\t──────\t")

	for _, m := range models {
		status := "Not downloaded"
		statusColor := warningColor
		if m.downloaded {
			status = "Ready"
			statusColor = successColor
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t\n",
			m.name,
			m.size,
			m.description,
			statusColor(status),
		)
	}

	w.Flush()
	fmt.Println(strings.Repeat("─", 70))
	fmt.Println()
	fmt.Println("To download a model: localcloud models pull <model-name>")
	fmt.Println("Recommended for 4GB RAM: qwen2.5:3b, deepseek-coder:1.3b")

	return nil
}

func runModelsPull(cmd *cobra.Command, args []string) error {
	modelName := args[0]

	// Validate model name
	validModels := map[string]string{
		"qwen2.5:3b":        "2.3GB",
		"deepseek-coder:1.3b": "1.5GB",
		"llama3.2:3b":       "2.0GB",
		"phi3:mini":         "2.3GB",
		"mistral:7b":        "4.1GB",
		"codellama:7b":      "3.8GB",
	}

	size, exists := validModels[modelName]
	if !exists {
		return fmt.Errorf("unknown model '%s'. Run 'localcloud models list' to see available models", modelName)
	}

	fmt.Printf("Pulling %s (%s)...\n", modelName, size)

	// Progress bar simulation
	progressBar := func(percent int) string {
		filled := percent / 5
		bar := strings.Repeat("=", filled) + ">" + strings.Repeat(" ", 20-filled)
		return fmt.Sprintf("[%s] %d%%", bar, percent)
	}

	// Simulate download with progress
	for i := 0; i <= 100; i += 5 {
		fmt.Printf("\r%s", progressBar(i))
		time.Sleep(100 * time.Millisecond)
	}
	fmt.Println()

	printSuccess(fmt.Sprintf("Successfully pulled %s", modelName))
	fmt.Println()
	fmt.Println("Model is ready to use. Start LocalCloud to begin:")
	fmt.Println("  localcloud start")

	return nil
}

func runModelsRemove(cmd *cobra.Command, args []string) error {
	modelName := args[0]

	// Check if model exists
	validModels := []string{
		"qwen2.5:3b",
		"deepseek-coder:1.3b",
		"llama3.2:3b",
		"phi3:mini",
		"mistral:7b",
		"codellama:7b",
	}

	isValid := false
	for _, m := range validModels {
		if m == modelName {
			isValid = true
			break
		}
	}

	if !isValid {
		return fmt.Errorf("model '%s' not found", modelName)
	}

	// Confirmation prompt
	fmt.Printf("Are you sure you want to remove %s? (y/N): ", modelName)
	var response string
	fmt.Scanln(&response)

	if strings.ToLower(response) != "y" {
		fmt.Println("Cancelled")
		return nil
	}

	// Simulate removal
	s := spinner.New(spinner.CharSets[14], 100*time.Millisecond)
	s.Suffix = fmt.Sprintf(" Removing %s...", modelName)
	s.Start()
	time.Sleep(1 * time.Second)
	s.Stop()

	printSuccess(fmt.Sprintf("Removed model %s", modelName))

	return nil
}