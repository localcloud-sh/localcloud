// internal/cli/helpers.go
package cli

import (
	"fmt"
	"github.com/fatih/color"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"unicode"
)

// Common error types
var (
	ErrDockerNotRunning = &CLIError{
		Type:    "docker_not_running",
		Message: "Docker is not running",
		Solution: `Start Docker:
  - macOS/Windows: Open Docker Desktop
  - Linux: sudo systemctl start docker

For installation: https://docs.docker.com/get-docker/`,
	}

	ErrPortInUse = &CLIError{
		Type:    "port_in_use",
		Message: "Port already in use",
	}

	ErrInsufficientMemory = &CLIError{
		Type:    "insufficient_memory",
		Message: "Insufficient memory",
		Solution: `LocalCloud requires at least 4GB of RAM.

To free up memory:
  - Close unnecessary applications
  - Use smaller AI models (gemma2:2b, phi3:mini)
  - Reduce memory limits in config`,
	}

	ErrDiskSpace = &CLIError{
		Type:    "disk_space",
		Message: "Insufficient disk space",
		Solution: `Free up disk space:
  - Remove unused Docker images: docker system prune -a
  - Clear logs: rm -rf .localcloud/logs/*
  - Remove unused models: lc models remove <model>`,
	}
)

// CLIError represents a structured error with solutions
type CLIError struct {
	Type     string
	Message  string
	Solution string
	Details  map[string]interface{}
}

func (e *CLIError) Error() string {
	return e.Message
}

// FormatError formats an error with helpful information
func FormatError(err error) string {
	if err == nil {
		return ""
	}

	// Check if it's a CLI error
	if cliErr, ok := err.(*CLIError); ok {
		return formatCLIError(cliErr)
	}

	// Check for common error patterns
	errStr := err.Error()

	// Docker not running
	if strings.Contains(errStr, "Cannot connect to the Docker daemon") ||
		strings.Contains(errStr, "Docker daemon not running") {
		return formatCLIError(ErrDockerNotRunning)
	}

	// Port in use
	if strings.Contains(errStr, "bind: address already in use") {
		// Try to extract port number
		port := extractPort(errStr)
		portErr := *ErrPortInUse
		portErr.Message = fmt.Sprintf("Port %s already in use", port)
		portErr.Solution = fmt.Sprintf(`This usually means another service is using port %s.

To fix this:
1. Find the process: lsof -i :%s
2. Stop the process or change the port in .localcloud/config.yaml
3. Run 'lc doctor' to check all ports`, port, port)
		return formatCLIError(&portErr)
	}

	// Memory issues
	if strings.Contains(errStr, "out of memory") ||
		strings.Contains(errStr, "cannot allocate memory") {
		return formatCLIError(ErrInsufficientMemory)
	}

	// Disk space
	if strings.Contains(errStr, "no space left on device") {
		return formatCLIError(ErrDiskSpace)
	}

	// Default formatting
	return fmt.Sprintf("%s %s", errorColor("Error:"), err.Error())
}

// formatCLIError formats a CLIError with colors and structure
func formatCLIError(err *CLIError) string {
	var output strings.Builder

	// Error message
	output.WriteString(fmt.Sprintf("\n%s %s\n", errorColor("Error:"), err.Message))

	// Solution if available
	if err.Solution != "" {
		output.WriteString(fmt.Sprintf("\n%s\n", warningColor("To fix this:")))
		lines := strings.Split(err.Solution, "\n")
		for _, line := range lines {
			if strings.TrimSpace(line) != "" {
				output.WriteString(fmt.Sprintf("%s\n", line))
			}
		}
	}

	// Help command
	output.WriteString(fmt.Sprintf("\n%s\n", infoColor("For more help: lc doctor")))

	return output.String()
}

// extractPort tries to extract port number from error message
func extractPort(errStr string) string {
	// Look for patterns like :3000 or port 3000
	parts := strings.Split(errStr, ":")
	for i := range parts {
		// Check if next part might be a port number
		if i+1 < len(parts) {
			portStr := strings.TrimSpace(parts[i+1])
			// Extract just the number part
			for j, ch := range portStr {
				if !unicode.IsDigit(ch) {
					portStr = portStr[:j]
					break
				}
			}
			if portStr != "" && len(portStr) <= 5 {
				return portStr
			}
		}
	}
	return "unknown"
}

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

// formatTime formats a time to a user-friendly string
func formatTime(t string) string {
	// Basic implementation - can be enhanced
	return t
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

// PrintRedisServiceInfo prints Redis service information with queue examples
func PrintRedisServiceInfo(port int) {
	green := color.New(color.FgGreen).SprintFunc()
	bold := color.New(color.Bold).SprintFunc()
	cyan := color.New(color.FgCyan).SprintFunc()

	fmt.Printf("\n%s %s\n", green("✓"), bold("Redis (Cache + Queue)"))
	fmt.Printf("  URL: %s\n", cyan(fmt.Sprintf("redis://localhost:%d", port)))
	fmt.Println("  Try:")
	fmt.Printf("    %s\n", cyan("# Test connection"))
	fmt.Printf("    %s\n", cyan(fmt.Sprintf("redis-cli -p %d ping", port)))
	fmt.Println()
	fmt.Printf("    %s\n", cyan("# Simple queue operations"))
	fmt.Printf("    %s\n", cyan(fmt.Sprintf("redis-cli -p %d LPUSH jobs '{\"id\":\"123\",\"task\":\"process\"}'", port)))
	fmt.Printf("    %s\n", cyan(fmt.Sprintf("redis-cli -p %d BRPOP jobs 0", port)))
	fmt.Println()
	fmt.Printf("    %s\n", cyan("# Check queue length"))
	fmt.Printf("    %s\n", cyan(fmt.Sprintf("redis-cli -p %d LLEN jobs", port)))
}

// internal/cli/helpers.go
// Add these functions to the existing helpers.go file

func PrintRedisCacheInfo(port int) {
	green := color.New(color.FgGreen).SprintFunc()
	bold := color.New(color.Bold).SprintFunc()
	cyan := color.New(color.FgCyan).SprintFunc()
	yellow := color.New(color.FgYellow).SprintFunc()

	fmt.Printf("\n%s %s\n", green("✓"), bold("Redis Cache"))
	fmt.Printf("  URL: %s\n", cyan(fmt.Sprintf("redis://localhost:%d", port)))
	fmt.Printf("  %s\n", yellow("Optimized for: Temporary data, sessions"))
	fmt.Printf("  %s\n", yellow("Persistence: Disabled (memory-only)"))
	fmt.Println()
	fmt.Println("  Try:")
	fmt.Printf("    %s\n", cyan("# Test connection"))
	fmt.Printf("    %s\n", cyan(fmt.Sprintf("redis-cli -p %d ping", port)))
	fmt.Println()
	fmt.Printf("    %s\n", cyan("# Cache operations"))
	fmt.Printf("    %s\n", cyan(fmt.Sprintf("redis-cli -p %d SET user:123 '{\"name\":\"John\"}' EX 3600", port)))
	fmt.Printf("    %s\n", cyan(fmt.Sprintf("redis-cli -p %d GET user:123", port)))
	fmt.Printf("    %s\n", cyan(fmt.Sprintf("redis-cli -p %d TTL user:123", port)))
}

// PrintRedisQueueInfo prints Redis queue-specific information
func PrintRedisQueueInfo(port int) {
	green := color.New(color.FgGreen).SprintFunc()
	bold := color.New(color.Bold).SprintFunc()
	cyan := color.New(color.FgCyan).SprintFunc()
	yellow := color.New(color.FgYellow).SprintFunc()

	fmt.Printf("\n%s %s\n", green("✓"), bold("Redis Queue"))
	fmt.Printf("  URL: %s\n", cyan(fmt.Sprintf("redis://localhost:%d", port)))
	fmt.Printf("  %s\n", yellow("Optimized for: Job queues, persistent data"))
	fmt.Printf("  %s\n", yellow("Persistence: Enabled (survives restarts)"))
	fmt.Println()
	fmt.Println("  Try:")
	fmt.Printf("    %s\n", cyan("# Queue operations"))
	fmt.Printf("    %s\n", cyan(fmt.Sprintf("redis-cli -p %d LPUSH jobs '{\"id\":\"123\",\"task\":\"process\"}'", port)))
	fmt.Printf("    %s\n", cyan(fmt.Sprintf("redis-cli -p %d BRPOP jobs 0", port)))
	fmt.Printf("    %s\n", cyan(fmt.Sprintf("redis-cli -p %d LLEN jobs", port)))
	fmt.Println()
	fmt.Printf("    %s\n", cyan("# Priority queue"))
	fmt.Printf("    %s\n", cyan(fmt.Sprintf("redis-cli -p %d ZADD priority 1 'urgent-task'", port)))
	fmt.Printf("    %s\n", cyan(fmt.Sprintf("redis-cli -p %d ZRANGE priority 0 -1", port)))
}

// internal/cli/helpers.go
// ADD these functions to your existing helpers.go file

// PrintPgVectorServiceInfo prints PostgreSQL with pgvector service information
func PrintPgVectorServiceInfo(port int) {
	green := color.New(color.FgGreen).SprintFunc()
	bold := color.New(color.Bold).SprintFunc()
	cyan := color.New(color.FgCyan).SprintFunc()
	yellow := color.New(color.FgYellow).SprintFunc()

	fmt.Printf("\n%s %s\n", green("✓"), bold("PostgreSQL with pgvector"))
	fmt.Printf("  URL: %s\n", cyan(fmt.Sprintf("postgresql://localcloud:localcloud@localhost:%d/localcloud", port)))
	fmt.Println("  Vector search ready!")
	fmt.Println()

	fmt.Printf("  %s\n", yellow("pgvector Examples:"))

	// Connection example
	fmt.Printf("    %s\n", cyan("# Connect to database"))
	fmt.Printf("    %s\n", cyan(fmt.Sprintf("psql postgresql://localcloud:localcloud@localhost:%d/localcloud", port)))
	fmt.Println()

	// Create table example
	fmt.Printf("    %s\n", cyan("# Create a table with vector column"))
	fmt.Printf("    %s\n", cyan("CREATE TABLE items ("))
	fmt.Printf("    %s\n", cyan("  id SERIAL PRIMARY KEY,"))
	fmt.Printf("    %s\n", cyan("  content TEXT,"))
	fmt.Printf("    %s\n", cyan("  embedding vector(1536)"))
	fmt.Printf("    %s\n", cyan(");"))
	fmt.Println()

	// Insert example
	fmt.Printf("    %s\n", cyan("# Insert vector data"))
	fmt.Printf("    %s\n", cyan("INSERT INTO items (content, embedding)"))
	fmt.Printf("    %s\n", cyan("VALUES ('Sample text', '[0.1, 0.2, 0.3, ...]');"))
	fmt.Println()

	// Search example
	fmt.Printf("    %s\n", cyan("# Find similar vectors (cosine similarity)"))
	fmt.Printf("    %s\n", cyan("SELECT content, embedding <=> '[0.1, 0.2, 0.3, ...]' AS distance"))
	fmt.Printf("    %s\n", cyan("FROM items"))
	fmt.Printf("    %s\n", cyan("ORDER BY distance"))
	fmt.Printf("    %s\n", cyan("LIMIT 5;"))
	fmt.Println()

	// Index example
	fmt.Printf("    %s\n", cyan("# Create index for faster searches"))
	fmt.Printf("    %s\n", cyan("CREATE INDEX ON items"))
	fmt.Printf("    %s\n", cyan("USING ivfflat (embedding vector_cosine_ops)"))
	fmt.Printf("    %s\n", cyan("WITH (lists = 100);"))
	fmt.Println()

	fmt.Printf("  %s\n", yellow("Integration with Ollama embeddings:"))
	fmt.Printf("    %s\n", cyan("# Get embedding from Ollama"))
	fmt.Printf("    %s\n", cyan("curl http://localhost:11434/api/embeddings \\"))
	fmt.Printf("    %s\n", cyan("  -d '{\"model\":\"nomic-embed-text\",\"prompt\":\"Your text here\"}'"))
	fmt.Println()

	fmt.Printf("  %s For more: %s\n", yellow("📚"), cyan("https://github.com/pgvector/pgvector"))
}

// PrintPostgreSQLServiceInfo prints standard PostgreSQL service information
func PrintPostgreSQLServiceInfo(port int, extensions []string) {
	green := color.New(color.FgGreen).SprintFunc()
	bold := color.New(color.Bold).SprintFunc()
	cyan := color.New(color.FgCyan).SprintFunc()

	// Check if pgvector is enabled
	hasPgVector := false
	for _, ext := range extensions {
		if ext == "pgvector" || ext == "vector" {
			hasPgVector = true
			break
		}
	}

	if hasPgVector {
		PrintPgVectorServiceInfo(port)
		return
	}

	// Standard PostgreSQL info
	fmt.Printf("\n%s %s\n", green("✓"), bold("PostgreSQL"))
	fmt.Printf("  URL: %s\n", cyan(fmt.Sprintf("postgresql://localcloud:localcloud@localhost:%d/localcloud", port)))
	fmt.Println("  Try:")
	fmt.Printf("    %s\n", cyan(fmt.Sprintf("psql postgresql://localcloud:localcloud@localhost:%d/localcloud", port)))
}
