// internal/docker/utils.go
package docker

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
)

// parseMemoryLimit converts memory limit string to bytes
func parseMemoryLimit(limit string) int64 {
	limit = strings.ToUpper(strings.TrimSpace(limit))

	multipliers := map[string]int64{
		"B":  1,
		"KB": 1024,
		"MB": 1024 * 1024,
		"GB": 1024 * 1024 * 1024,
		"TB": 1024 * 1024 * 1024 * 1024,
	}

	for suffix, multiplier := range multipliers {
		if strings.HasSuffix(limit, suffix) {
			numStr := strings.TrimSuffix(limit, suffix)
			numStr = strings.TrimSpace(numStr)

			// Parse the number (simplified - in production use strconv)
			var num int64
			for _, ch := range numStr {
				if ch >= '0' && ch <= '9' {
					num = num*10 + int64(ch-'0')
				}
			}

			return num * multiplier
		}
	}

	// Default to 4GB if parsing fails
	return 4 * 1024 * 1024 * 1024
}

// generatePassword generates a secure random password
func generatePassword() string {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		// Fallback to default in case of error
		return "localcloud-dev-password"
	}
	return hex.EncodeToString(bytes)
}

// getProjectName extracts project name from config or defaults
func getProjectName() string {
	// This should be properly implemented to get from config
	// For now, return default
	return "default"
}

// formatBytes converts bytes to human-readable format
func formatBytes(bytes uint64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}

	div, exp := uint64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}

	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// Container name helpers
func getContainerName(service string) string {
	return fmt.Sprintf("localcloud-%s", service)
}

func getVolumeName(volume string) string {
	return fmt.Sprintf("localcloud_%s", volume)
}

func getNetworkName(network string) string {
	return fmt.Sprintf("localcloud_%s", network)
}
