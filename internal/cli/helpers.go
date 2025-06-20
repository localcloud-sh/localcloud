// internal/cli/helpers.go
package cli

import (
	"fmt"
	"os"
	"path/filepath"
)

// isProjectInitialized checks if the current directory is a LocalCloud project
func isProjectInitialized() bool {
	configPath := filepath.Join(projectPath, ".localcloud", "config.yaml")
	_, err := os.Stat(configPath)
	return err == nil
}

// getProjectRoot finds the project root directory by looking for .localcloud folder
func getProjectRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	// Walk up the directory tree looking for .localcloud
	for {
		configPath := filepath.Join(dir, ".localcloud")
		if _, err := os.Stat(configPath); err == nil {
			return dir, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached root without finding .localcloud
			break
		}
		dir = parent
	}

	return "", os.ErrNotExist
}

// formatBytes converts bytes to human readable format
func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// truncateString truncates a string to a maximum length
func truncateString(str string, maxLen int) string {
	if len(str) <= maxLen {
		return str
	}
	return str[:maxLen-3] + "..."
}