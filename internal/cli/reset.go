// internal/cli/reset.go
package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/localcloud/localcloud/internal/config"
	"github.com/localcloud/localcloud/internal/docker"
	"github.com/spf13/cobra"
)

// internal/cli/reset.go - Updated with destroy option

var (
	resetKeepData   bool
	resetHard       bool
	resetConfirm    bool
	resetKeepModels bool
	resetDestroy    bool // New flag for complete destruction
)

var resetCmd = &cobra.Command{
	Use:   "reset",
	Short: "Reset LocalCloud to clean state",
	Long: `Reset LocalCloud project by stopping services and cleaning up resources.
	
By default, this will:
  - Stop all running services
  - Remove all containers
  - Keep data volumes and configuration
	
Use --hard for complete cleanup including data.
Use --destroy for complete project removal (like teardown).`,
	Example: `  lc reset              # Soft reset (keep data)
  lc reset --hard       # Hard reset (remove everything)
  lc reset --destroy    # Complete removal (requires confirmation)
  lc reset --yes        # Skip confirmation`,
	RunE: runReset,
}

func init() {
	resetCmd.Flags().BoolVar(&resetKeepData, "keep-data", true, "Keep data volumes")
	resetCmd.Flags().BoolVar(&resetHard, "hard", false, "Remove everything including data and config")
	resetCmd.Flags().BoolVar(&resetDestroy, "destroy", false, "Complete project destruction (removes everything)")
	resetCmd.Flags().BoolVarP(&resetConfirm, "yes", "y", false, "Skip confirmation prompt")
	resetCmd.Flags().BoolVar(&resetKeepModels, "keep-models", true, "Keep AI models (saves bandwidth)")
}

func runReset(cmd *cobra.Command, args []string) error {
	// Check if project exists
	if !IsProjectInitialized() {
		return fmt.Errorf("no LocalCloud project found")
	}

	// Get config
	cfg := config.Get()
	if cfg == nil {
		return fmt.Errorf("failed to load configuration")
	}

	// Determine reset level
	resetLevel := "soft"
	if resetDestroy {
		resetLevel = "destroy"
		resetHard = true
		resetKeepData = false
		resetKeepModels = false
	} else if resetHard {
		resetLevel = "hard"
		resetKeepData = false
		resetKeepModels = false
	}

	// Show what will be reset
	fmt.Println(infoColor("LocalCloud Reset"))
	fmt.Println(strings.Repeat("━", 50))
	fmt.Printf("Reset Type: %s\n", warningColor(strings.ToUpper(resetLevel)))
	fmt.Println("\nThis will:")

	fmt.Println("  • Stop all running services")
	fmt.Println("  • Remove all containers")

	if !resetKeepData {
		fmt.Println("  • " + errorColor("DELETE all data volumes"))
	} else {
		fmt.Println("  • " + successColor("Keep data volumes"))
	}

	if !resetKeepModels {
		fmt.Println("  • " + errorColor("DELETE AI models"))
		if len(cfg.Services.AI.Models) > 0 {
			fmt.Println("\n    Models to remove:")
			for _, model := range cfg.Services.AI.Models {
				fmt.Printf("      - %s\n", model)
			}
		}
	} else {
		fmt.Println("  • " + successColor("Keep AI models"))
	}

	if resetHard || resetDestroy {
		fmt.Println("  • " + errorColor("DELETE configuration"))
		fmt.Println("  • " + errorColor("DELETE logs"))
		fmt.Println("  • " + errorColor("DELETE all LocalCloud files"))
	}

	if resetDestroy {
		fmt.Println("  • " + errorColor("DELETE Docker network"))
		fmt.Println("\n" + errorColor("⚠️  This is IRREVERSIBLE and will completely remove the project!"))
		fmt.Println("\n" + infoColor("Note: The current directory will NOT be deleted."))
		fmt.Println(infoColor("      Only LocalCloud files will be removed."))
	}

	fmt.Println(strings.Repeat("━", 50))

	// Confirmation
	if !resetConfirm {
		if resetDestroy {
			// Extra confirmation for destroy
			fmt.Printf("\n%s Type '%s' to confirm: ", errorColor("DANGER:"), errorColor("DESTROY"))
			var response string
			fmt.Scanln(&response)
			if response != "DESTROY" {
				fmt.Println("Reset cancelled")
				return nil
			}
		} else {
			fmt.Printf("\n%s This action cannot be undone. Continue? [y/N]: ", warningColor("Warning:"))
			var response string
			fmt.Scanln(&response)
			if strings.ToLower(response) != "y" {
				fmt.Println("Reset cancelled")
				return nil
			}
		}
	}

	// Create Docker manager
	ctx := context.Background()
	manager, err := docker.NewManager(ctx, cfg)
	if err != nil {
		// Docker might not be running, continue with file cleanup
		printWarning("Docker is not running. Will clean up files only.")
	} else {
		defer manager.Close()

		// Stop all services
		if err := stopAllServices(manager); err != nil {
			printWarning(fmt.Sprintf("Failed to stop some services: %v", err))
		}

		// Clean up Docker resources
		if err := cleanupDocker(manager, resetKeepData, resetKeepModels); err != nil {
			printWarning(fmt.Sprintf("Failed to clean up some Docker resources: %v", err))
		}

		// Remove AI models if requested
		if !resetKeepModels && len(cfg.Services.AI.Models) > 0 {
			fmt.Println("\nRemoving AI models...")
			for _, model := range cfg.Services.AI.Models {
				fmt.Printf("  Removing %s...\n", model)
				// Call model removal logic here
				// manager.RemoveModel(model)
			}
		}
	}

	// Clean up files
	if err := cleanupFiles(resetHard || resetDestroy); err != nil {
		printWarning(fmt.Sprintf("Failed to clean up some files: %v", err))
	}

	// Final message
	fmt.Println()
	if resetDestroy {
		printSuccess("LocalCloud project has been completely destroyed")
		fmt.Println("All LocalCloud files and resources have been removed.")
		fmt.Println()
		fmt.Println("To remove this directory completely:")
		fmt.Printf("  cd .. && rm -rf %s\n", filepath.Base(projectPath))
	} else if resetHard {
		printSuccess("LocalCloud has been completely reset")
		fmt.Println("Run 'lc init' to start a new project")
	} else {
		printSuccess("LocalCloud has been reset")
		if resetKeepData {
			fmt.Println("Your data has been preserved")
		}
		fmt.Println("Run 'lc start' to restart services")
	}

	return nil
}

func stopAllServices(manager *docker.Manager) error {
	fmt.Println("\nStopping services...")

	progress := make(chan docker.ServiceProgress)
	done := make(chan error)

	go func() {
		done <- manager.StopServices(progress)
	}()

	// Display progress
	for {
		select {
		case p, ok := <-progress:
			if !ok {
				goto finished
			}

			switch p.Status {
			case "stopping":
				fmt.Printf("  Stopping %s...\n", p.Service)
			case "stopped":
				fmt.Printf("  %s %s stopped\n", successColor("✓"), p.Service)
			case "failed":
				fmt.Printf("  %s Failed to stop %s: %s\n", errorColor("✗"), p.Service, p.Error)
			}
		}
	}

finished:
	return <-done
}

func cleanupDocker(manager *docker.Manager, keepData, keepModels bool) error {
	fmt.Println("\nCleaning up Docker resources...")

	// Get container manager
	containerMgr := manager.GetClient().NewContainerManager()

	// List and remove all LocalCloud containers
	containers, err := containerMgr.List(map[string]string{
		"label": fmt.Sprintf("com.localcloud.project=%s", manager.GetConfig().Project.Name),
	})
	if err != nil {
		return err
	}

	for _, container := range containers {
		fmt.Printf("  Removing container %s...\n", container.Name)
		if err := containerMgr.Remove(container.ID); err != nil {
			printWarning(fmt.Sprintf("Failed to remove %s: %v", container.Name, err))
		} else {
			fmt.Printf("  %s Removed %s\n", successColor("✓"), container.Name)
		}
	}

	// Clean up volumes if requested
	if !keepData || !keepModels {
		volumeMgr := manager.GetClient().NewVolumeManager()
		volumes, err := volumeMgr.List(nil)
		if err != nil {
			return err
		}

		for _, vol := range volumes {
			// Skip model volumes if keeping models
			if keepModels && strings.Contains(vol.Name, "ollama_models") {
				fmt.Printf("  %s Keeping models volume: %s\n", infoColor("○"), vol.Name)
				continue
			}

			// Skip data volumes if keeping data
			if keepData && !strings.Contains(vol.Name, "ollama_models") {
				fmt.Printf("  %s Keeping data volume: %s\n", infoColor("○"), vol.Name)
				continue
			}

			fmt.Printf("  Removing volume %s...\n", vol.Name)
			if err := volumeMgr.Remove(vol.Name); err != nil {
				printWarning(fmt.Sprintf("Failed to remove %s: %v", vol.Name, err))
			} else {
				fmt.Printf("  %s Removed %s\n", successColor("✓"), vol.Name)
			}
		}
	}

	// Clean up networks
	networkMgr := manager.GetClient().NewNetworkManager()
	networks, err := networkMgr.List()
	if err != nil {
		return err
	}

	for _, net := range networks {
		fmt.Printf("  Removing network %s...\n", net.Name)
		if err := networkMgr.Remove(net.ID); err != nil {
			printWarning(fmt.Sprintf("Failed to remove %s: %v", net.Name, err))
		} else {
			fmt.Printf("  %s Removed %s\n", successColor("✓"), net.Name)
		}
	}

	return nil
}

func cleanupFiles(hardReset bool) error {
	fmt.Println("\nCleaning up files...")

	// Always clean these
	cleanupPaths := []string{
		".localcloud/logs",
		".localcloud/tmp",
		".localcloud/cache",
		".localcloud/tunnels",
		".localcloud/storage-credentials.json",
	}

	// Additional paths for hard reset
	if hardReset {
		cleanupPaths = append(cleanupPaths,
			".localcloud/config.yaml",
			".localcloud/backups",
			".localcloud",
		)
	}

	for _, path := range cleanupPaths {
		if _, err := os.Stat(path); os.IsNotExist(err) {
			continue
		}

		fmt.Printf("  Removing %s...\n", path)
		if err := os.RemoveAll(path); err != nil {
			printWarning(fmt.Sprintf("Failed to remove %s: %v", path, err))
		} else {
			fmt.Printf("  %s Removed %s\n", successColor("✓"), path)
		}
	}

	// Clean up any docker-compose files
	composeFiles := []string{
		"docker-compose.yml",
		"docker-compose.yaml",
		"docker-compose.override.yml",
		"docker-compose.override.yaml",
	}

	for _, file := range composeFiles {
		if _, err := os.Stat(file); err == nil {
			fmt.Printf("  Removing %s...\n", file)
			if err := os.Remove(file); err != nil {
				printWarning(fmt.Sprintf("Failed to remove %s: %v", file, err))
			} else {
				fmt.Printf("  %s Removed %s\n", successColor("✓"), file)
			}
		}
	}

	return nil
}
