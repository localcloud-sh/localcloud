// 1. Delete internal/diagnostics/system_info.go (it conflicts with system_info_generic.go)
// The system_info.go file should be removed as it has the same build tags as system_info_generic.go

// 2. Update internal/diagnostics/types.go (remove it if it exists, as types are already in debugger.go)

// 3. Update internal/diagnostics/doctor.go:
// internal/diagnostics/doctor.go
package diagnostics

import (
	"context"
	"fmt"
	"net"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/docker/docker/client"
	"github.com/localcloud-sh/localcloud/internal/config"
)

// Doctor performs system diagnostics
type Doctor struct {
	config  *config.Config
	results []DiagnosticResult
}

// DiagnosticResult represents a diagnostic check result
type DiagnosticResult struct {
	Check    string
	Status   CheckStatus
	Message  string
	Solution string
	Details  map[string]interface{}
}

// CheckStatus represents the status of a diagnostic check
type CheckStatus string

const (
	CheckStatusOK      CheckStatus = "ok"
	CheckStatusWarning CheckStatus = "warning"
	CheckStatusError   CheckStatus = "error"
	CheckStatusSkipped CheckStatus = "skipped"
)

// NewDoctor creates a new diagnostic doctor
func NewDoctor(cfg *config.Config) *Doctor {
	return &Doctor{
		config:  cfg,
		results: []DiagnosticResult{},
	}
}

// RunDiagnostics runs all diagnostic checks
func (d *Doctor) RunDiagnostics() error {
	checks := []struct {
		name string
		fn   func() DiagnosticResult
	}{
		{"Docker Daemon", d.checkDocker},
		{"Docker Version", d.checkDockerVersion},
		{"System Resources", d.checkSystemResources},
		{"Port Availability", d.checkPorts},
		{"Network Connectivity", d.checkNetwork},
		{"Disk Space", d.checkDiskSpace},
		{"Configuration", d.checkConfiguration},
		{"Service Dependencies", d.checkDependencies},
		{"Model Compatibility", d.checkModelCompatibility},
	}

	for _, check := range checks {
		result := check.fn()
		result.Check = check.name
		d.results = append(d.results, result)
	}

	return nil
}

// GetResults returns diagnostic results
func (d *Doctor) GetResults() []DiagnosticResult {
	return d.results
}

// HasIssues returns true if any issues were found
func (d *Doctor) HasIssues() bool {
	for _, result := range d.results {
		if result.Status == CheckStatusError || result.Status == CheckStatusWarning {
			return true
		}
	}
	return false
}

// checkDocker checks if Docker daemon is running
func (d *Doctor) checkDocker() DiagnosticResult {
	cli, err := client.NewClientWithOpts(
		client.FromEnv,
		client.WithAPIVersionNegotiation(),
	)
	if err != nil {
		return DiagnosticResult{
			Status:  CheckStatusError,
			Message: "Failed to create Docker client",
			Solution: "Ensure Docker is installed and the Docker daemon is running:\n" +
				"  - macOS/Windows: Start Docker Desktop\n" +
				"  - Linux: sudo systemctl start docker",
			Details: map[string]interface{}{
				"error": err.Error(),
			},
		}
	}
	defer cli.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err = cli.Ping(ctx)
	if err != nil {
		return DiagnosticResult{
			Status:  CheckStatusError,
			Message: "Docker daemon is not running",
			Solution: "Start Docker:\n" +
				"  - macOS/Windows: Open Docker Desktop\n" +
				"  - Linux: sudo systemctl start docker",
			Details: map[string]interface{}{
				"error": err.Error(),
			},
		}
	}

	return DiagnosticResult{
		Status:  CheckStatusOK,
		Message: "Docker daemon is running",
	}
}

// checkDockerVersion checks Docker version compatibility
func (d *Doctor) checkDockerVersion() DiagnosticResult {
	cli, err := client.NewClientWithOpts(
		client.FromEnv,
		client.WithAPIVersionNegotiation(),
	)
	if err != nil {
		return DiagnosticResult{
			Status:  CheckStatusSkipped,
			Message: "Could not check Docker version",
		}
	}
	defer cli.Close()

	version, err := cli.ServerVersion(context.Background())
	if err != nil {
		return DiagnosticResult{
			Status:  CheckStatusError,
			Message: "Failed to get Docker version",
			Details: map[string]interface{}{
				"error": err.Error(),
			},
		}
	}

	// Check minimum version (20.10)
	versionParts := strings.Split(version.Version, ".")
	if len(versionParts) < 2 {
		return DiagnosticResult{
			Status:  CheckStatusWarning,
			Message: fmt.Sprintf("Could not parse Docker version: %s", version.Version),
		}
	}

	major, _ := strconv.Atoi(versionParts[0])
	minor, _ := strconv.Atoi(versionParts[1])

	if major < 20 || (major == 20 && minor < 10) {
		return DiagnosticResult{
			Status:  CheckStatusWarning,
			Message: fmt.Sprintf("Docker version %s is older than recommended", version.Version),
			Solution: "Consider upgrading Docker to version 20.10 or later:\n" +
				"  - https://docs.docker.com/engine/install/",
			Details: map[string]interface{}{
				"current_version": version.Version,
				"minimum_version": "20.10",
			},
		}
	}

	return DiagnosticResult{
		Status:  CheckStatusOK,
		Message: fmt.Sprintf("Docker version %s (compatible)", version.Version),
		Details: map[string]interface{}{
			"version":     version.Version,
			"api_version": version.APIVersion,
			"os":          version.Os,
			"arch":        version.Arch,
		},
	}
}

// checkSystemResources checks system resources
func (d *Doctor) checkSystemResources() DiagnosticResult {
	// Get memory info using platform-specific implementation
	memInfo := getSystemMemoryInfo()

	totalMemGB := float64(memInfo.Total) / (1024 * 1024 * 1024)
	freeMemGB := float64(memInfo.Free) / (1024 * 1024 * 1024)

	// Check minimum memory (4GB recommended)
	if totalMemGB < 4 {
		return DiagnosticResult{
			Status:  CheckStatusError,
			Message: fmt.Sprintf("Insufficient memory: %.1fGB (4GB required)", totalMemGB),
			Solution: "LocalCloud requires at least 4GB of RAM to run AI models effectively.\n" +
				"Consider:\n" +
				"  - Closing other applications\n" +
				"  - Using smaller models (gemma2:2b, phi3:mini)\n" +
				"  - Upgrading system memory",
			Details: map[string]interface{}{
				"total_memory_gb": totalMemGB,
				"free_memory_gb":  freeMemGB,
				"required_gb":     4,
			},
		}
	}

	if freeMemGB < 2 {
		return DiagnosticResult{
			Status:   CheckStatusWarning,
			Message:  fmt.Sprintf("Low available memory: %.1fGB free", freeMemGB),
			Solution: "Close unnecessary applications to free up memory",
			Details: map[string]interface{}{
				"total_memory_gb": totalMemGB,
				"free_memory_gb":  freeMemGB,
			},
		}
	}

	// Check CPU cores
	cpuCount := runtime.NumCPU()
	if cpuCount < 2 {
		return DiagnosticResult{
			Status:   CheckStatusWarning,
			Message:  fmt.Sprintf("Limited CPU cores: %d", cpuCount),
			Solution: "LocalCloud works best with 2 or more CPU cores",
			Details: map[string]interface{}{
				"cpu_cores": cpuCount,
			},
		}
	}

	return DiagnosticResult{
		Status:  CheckStatusOK,
		Message: fmt.Sprintf("System resources: %.1fGB RAM (%.1fGB free), %d CPU cores", totalMemGB, freeMemGB, cpuCount),
		Details: map[string]interface{}{
			"total_memory_gb": totalMemGB,
			"free_memory_gb":  freeMemGB,
			"cpu_cores":       cpuCount,
			"os":              runtime.GOOS,
			"arch":            runtime.GOARCH,
		},
	}
}

// checkPorts checks if required ports are available
func (d *Doctor) checkPorts() DiagnosticResult {
	// Check if config is nil
	if d.config == nil {
		return DiagnosticResult{
			Status:  CheckStatusSkipped,
			Message: "No configuration available to check ports",
		}
	}

	ports := map[string]int{
		"AI (Ollama)":   d.config.Services.AI.Port,
		"PostgreSQL":    d.config.Services.Database.Port,
		"Redis":         d.config.Services.Cache.Port,
		"MinIO":         d.config.Services.Storage.Port,
		"MinIO Console": d.config.Services.Storage.Console,
		"Web UI":        3000,
		"API":           8080,
	}

	conflicts := []string{}
	details := map[string]interface{}{}

	for service, port := range ports {
		if port == 0 {
			continue
		}

		if process := checkPort(port); process != "" {
			conflicts = append(conflicts, fmt.Sprintf("Port %d (%s) is in use by: %s", port, service, process))
			details[fmt.Sprintf("port_%d", port)] = map[string]interface{}{
				"service": service,
				"status":  "in_use",
				"process": process,
			}
		} else {
			details[fmt.Sprintf("port_%d", port)] = map[string]interface{}{
				"service": service,
				"status":  "available",
			}
		}
	}

	if len(conflicts) > 0 {
		return DiagnosticResult{
			Status:  CheckStatusError,
			Message: "Port conflicts detected",
			Solution: "Free up the ports or change them in .localcloud/config.yaml:\n" +
				strings.Join(conflicts, "\n"),
			Details: details,
		}
	}

	return DiagnosticResult{
		Status:  CheckStatusOK,
		Message: "All required ports are available",
		Details: details,
	}
}

// checkPort checks if a port is in use
func checkPort(port int) string {
	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		// Port is in use, try to find the process
		if runtime.GOOS == "darwin" || runtime.GOOS == "linux" {
			cmd := exec.Command("lsof", "-i", fmt.Sprintf(":%d", port))
			output, _ := cmd.Output()
			lines := strings.Split(string(output), "\n")
			if len(lines) > 1 {
				fields := strings.Fields(lines[1])
				if len(fields) > 0 {
					return fields[0]
				}
			}
		} else if runtime.GOOS == "windows" {
			cmd := exec.Command("netstat", "-ano", "-p", "tcp")
			output, _ := cmd.Output()
			lines := strings.Split(string(output), "\n")
			for _, line := range lines {
				if strings.Contains(line, fmt.Sprintf(":%d", port)) {
					return "unknown process"
				}
			}
		}
		return "unknown"
	}
	ln.Close()
	return ""
}

// checkNetwork checks network connectivity
func (d *Doctor) checkNetwork() DiagnosticResult {
	// Check DNS resolution
	addrs, err := net.LookupHost("github.com")
	if err != nil || len(addrs) == 0 {
		return DiagnosticResult{
			Status:   CheckStatusWarning,
			Message:  "DNS resolution issues detected",
			Solution: "Check your internet connection and DNS settings",
			Details: map[string]interface{}{
				"error": err,
			},
		}
	}

	// Check internet connectivity
	conn, err := net.DialTimeout("tcp", "github.com:443", 5*time.Second)
	if err != nil {
		return DiagnosticResult{
			Status:   CheckStatusWarning,
			Message:  "Limited internet connectivity",
			Solution: "Check your internet connection. Some features may not work properly.",
			Details: map[string]interface{}{
				"error": err.Error(),
			},
		}
	}
	conn.Close()

	return DiagnosticResult{
		Status:  CheckStatusOK,
		Message: "Network connectivity OK",
	}
}

// checkDiskSpace checks available disk space
func (d *Doctor) checkDiskSpace() DiagnosticResult {
	// Use platform-specific implementation
	diskInfo := getSystemDiskInfo()

	// Calculate available space in GB
	availableGB := float64(diskInfo.Free) / (1024 * 1024 * 1024)
	totalGB := float64(diskInfo.Total) / (1024 * 1024 * 1024)
	usedPercent := diskInfo.PercentUsed

	// Check minimum disk space (10GB recommended)
	if availableGB < 5 {
		return DiagnosticResult{
			Status:  CheckStatusError,
			Message: fmt.Sprintf("Critical: Only %.1fGB disk space available", availableGB),
			Solution: "Free up disk space. LocalCloud needs at least 10GB for models and data:\n" +
				"  - Remove unused Docker images: docker system prune -a\n" +
				"  - Clear LocalCloud logs: rm -rf .localcloud/logs/*\n" +
				"  - Remove unused models: lc models remove <model>",
			Details: map[string]interface{}{
				"available_gb": availableGB,
				"total_gb":     totalGB,
				"used_percent": usedPercent,
			},
		}
	}

	if availableGB < 10 {
		return DiagnosticResult{
			Status:   CheckStatusWarning,
			Message:  fmt.Sprintf("Low disk space: %.1fGB available", availableGB),
			Solution: "Consider freeing up disk space for optimal performance",
			Details: map[string]interface{}{
				"available_gb": availableGB,
				"total_gb":     totalGB,
				"used_percent": usedPercent,
			},
		}
	}

	return DiagnosticResult{
		Status:  CheckStatusOK,
		Message: fmt.Sprintf("Disk space: %.1fGB available (%.1f%% used)", availableGB, usedPercent),
		Details: map[string]interface{}{
			"available_gb": availableGB,
			"total_gb":     totalGB,
			"used_percent": usedPercent,
		},
	}
}

// checkConfiguration checks LocalCloud configuration
func (d *Doctor) checkConfiguration() DiagnosticResult {
	issues := []string{}

	// Check if config exists
	if d.config == nil {
		return DiagnosticResult{
			Status:   CheckStatusError,
			Message:  "No configuration found",
			Solution: "Run 'lc setup' to create a new project",
		}
	}

	// Validate memory limits
	if d.config.Resources.MemoryLimit != "" {
		// Parse memory limit
		var memLimit int64
		if strings.HasSuffix(d.config.Resources.MemoryLimit, "GB") {
			val, _ := strconv.ParseInt(strings.TrimSuffix(d.config.Resources.MemoryLimit, "GB"), 10, 64)
			memLimit = val * 1024 * 1024 * 1024
		}

		// Get system memory using platform-specific implementation
		memInfo := getSystemMemoryInfo()
		if memLimit > int64(memInfo.Total) {
			issues = append(issues, fmt.Sprintf("Memory limit %s exceeds system memory", d.config.Resources.MemoryLimit))
		}
	}

	// Check service configurations
	if d.config.Services.AI.Default != "" {
		found := false
		for _, model := range d.config.Services.AI.Models {
			if model == d.config.Services.AI.Default {
				found = true
				break
			}
		}
		if !found {
			issues = append(issues, fmt.Sprintf("Default model '%s' not in configured models list", d.config.Services.AI.Default))
		}
	}

	if len(issues) > 0 {
		return DiagnosticResult{
			Status:   CheckStatusWarning,
			Message:  "Configuration issues detected",
			Solution: "Review and fix the configuration in .localcloud/config.yaml:\n" + strings.Join(issues, "\n"),
			Details: map[string]interface{}{
				"issues": issues,
			},
		}
	}

	return DiagnosticResult{
		Status:  CheckStatusOK,
		Message: "Configuration is valid",
	}
}

// checkDependencies checks service dependencies
func (d *Doctor) checkDependencies() DiagnosticResult {
	// Check for optional tools
	tools := map[string]string{
		"psql":      "PostgreSQL client for database access",
		"redis-cli": "Redis client for cache access",
	}

	missing := []string{}
	available := []string{}

	for tool, description := range tools {
		if _, err := exec.LookPath(tool); err != nil {
			missing = append(missing, fmt.Sprintf("%s - %s", tool, description))
		} else {
			available = append(available, tool)
		}
	}

	details := map[string]interface{}{
		"available": available,
		"missing":   missing,
	}

	if len(missing) > 0 {
		return DiagnosticResult{
			Status:  CheckStatusWarning,
			Message: "Optional tools not found",
			Solution: "These tools enhance LocalCloud functionality but are not required:\n" +
				strings.Join(missing, "\n") + "\n\n" +
				"Install them for better experience:\n" +
				"  - macOS: brew install postgresql redis\n" +
				"  - Linux: apt-get install postgresql-client redis-tools",
			Details: details,
		}
	}

	return DiagnosticResult{
		Status:  CheckStatusOK,
		Message: "All optional dependencies available",
		Details: details,
	}
}

// checkModelCompatibility checks if configured models are compatible
func (d *Doctor) checkModelCompatibility() DiagnosticResult {
	// Check if config is nil
	if d.config == nil {
		return DiagnosticResult{
			Status:  CheckStatusSkipped,
			Message: "No configuration available to check models",
		}
	}

	// Model size requirements
	modelRequirements := map[string]int64{
		"qwen2.5:3b":          3 * 1024 * 1024 * 1024, // 3GB
		"deepseek-coder:1.3b": 2 * 1024 * 1024 * 1024, // 2GB
		"llama3.2:3b":         3 * 1024 * 1024 * 1024, // 3GB
		"phi3:mini":           2 * 1024 * 1024 * 1024, // 2GB
		"gemma2:2b":           2 * 1024 * 1024 * 1024, // 2GB
	}

	// Get system memory info
	memInfo := getSystemMemoryInfo()
	availableMemory := memInfo.Free

	warnings := []string{}

	for _, model := range d.config.Services.AI.Models {
		if req, ok := modelRequirements[model]; ok {
			if req > int64(availableMemory) {
				warnings = append(warnings, fmt.Sprintf("Model '%s' requires %.1fGB but only %.1fGB available",
					model,
					float64(req)/(1024*1024*1024),
					float64(availableMemory)/(1024*1024*1024)))
			}
		}
	}

	if len(warnings) > 0 {
		return DiagnosticResult{
			Status:  CheckStatusWarning,
			Message: "Some models may not run with current memory",
			Solution: "Consider using smaller models or freeing up memory:\n" +
				strings.Join(warnings, "\n"),
			Details: map[string]interface{}{
				"configured_models": d.config.Services.AI.Models,
				"warnings":          warnings,
			},
		}
	}

	return DiagnosticResult{
		Status:  CheckStatusOK,
		Message: "Model configuration is compatible with system resources",
		Details: map[string]interface{}{
			"configured_models": d.config.Services.AI.Models,
		},
	}
}
