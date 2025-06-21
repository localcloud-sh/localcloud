// internal/cli/helpers.go
package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"unicode"
)

// IsProjectInitialized checks if the current directory is a LocalCloud project
func IsProjectInitialized() bool {
	configPath := filepath.Join(projectPath, ".localcloud", "config.yaml")
	_, err := os.Stat(configPath)
	return err == nil
}

// GetProjectRoot finds the project root directory by looking for .localcloud folder
func GetProjectRoot() (string, error) {
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

// FormatBytes converts bytes to human readable format
func FormatBytes(bytes int64) string {
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

// openBrowser opens the specified URL in the default browser
func openBrowser(url string) error {
	var cmd string
	var args []string

	switch runtime.GOOS {
	case "darwin":
		cmd = "open"
		args = []string{url}
	case "windows":
		cmd = "rundll32"
		args = []string{"url.dll,FileProtocolHandler", url}
	default: // linux and others
		cmd = "xdg-open"
		args = []string{url}
	}

	return exec.Command(cmd, args...).Start()
}

// parseMemoryLimit converts memory string (e.g., "4GB") to bytes
func parseMemoryLimit(limit string) int64 {
	// Simple implementation
	limit = strings.TrimSpace(limit)
	if limit == "" {
		return 0 // No limit
	}

	// Extract number and unit
	var value int64
	var unit string

	// Try to parse formats like "4GB", "512MB", etc
	for i, r := range limit {
		if !unicode.IsDigit(r) {
			fmt.Sscanf(limit[:i], "%d", &value)
			unit = strings.ToUpper(limit[i:])
			break
		}
	}

	// If no unit found, assume bytes
	if unit == "" {
		fmt.Sscanf(limit, "%d", &value)
		return value
	}

	// Convert to bytes based on unit
	switch unit {
	case "GB", "G":
		return value * 1024 * 1024 * 1024
	case "MB", "M":
		return value * 1024 * 1024
	case "KB", "K":
		return value * 1024
	default:
		return value
	}
}
