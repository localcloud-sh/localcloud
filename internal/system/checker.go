package system

import (
	"context"
	"fmt"
	"net"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/mem"
)

// Checker implements system resource checking
type Checker struct {
	ctx context.Context
}

// NewChecker creates a new system checker
func NewChecker(ctx context.Context) *Checker {
	return &Checker{ctx: ctx}
}

// GetRAM returns total and available RAM in bytes
func (c *Checker) GetRAM() (total, available int64, err error) {
	v, err := mem.VirtualMemoryWithContext(c.ctx)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to get memory info: %w", err)
	}
	return int64(v.Total), int64(v.Available), nil
}

// GetDisk returns total and available disk space in bytes for the given path
func (c *Checker) GetDisk(path string) (total, available int64, err error) {
	if path == "" {
		path = "/"
		if runtime.GOOS == "windows" {
			path = "C:\\"
		}
	}

	usage, err := disk.UsageWithContext(c.ctx, path)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to get disk usage: %w", err)
	}
	return int64(usage.Total), int64(usage.Free), nil
}

// GetCPU returns the number of CPU cores
func (c *Checker) GetCPU() (int, error) {
	count, err := cpu.CountsWithContext(c.ctx, true)
	if err != nil {
		return 0, fmt.Errorf("failed to get CPU count: %w", err)
	}
	return count, nil
}

// CheckDocker checks if Docker is installed and returns version
func (c *Checker) CheckDocker() (installed bool, version string, err error) {
	cmd := exec.CommandContext(c.ctx, "docker", "--version")
	output, err := cmd.Output()
	if err != nil {
		return false, "", nil // Docker not installed
	}

	versionStr := strings.TrimSpace(string(output))
	// Parse version from "Docker version X.Y.Z, build abc123"
	parts := strings.Split(versionStr, " ")
	if len(parts) >= 3 {
		version = parts[2]
		version = strings.TrimSuffix(version, ",")
	}

	// Also check if Docker daemon is running
	cmd = exec.CommandContext(c.ctx, "docker", "info")
	if err := cmd.Run(); err != nil {
		return true, version, fmt.Errorf("Docker installed but daemon not running")
	}

	return true, version, nil
}

// CheckPort checks if a port is available
func (c *Checker) CheckPort(port int) bool {
	timeout := time.Second
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("localhost:%d", port), timeout)
	if err != nil {
		// Port is available (connection failed)
		return true
	}
	conn.Close()
	// Port is in use (connection succeeded)
	return false
}

// CheckOllama checks if Ollama is installed
func (c *Checker) CheckOllama() bool {
	cmd := exec.CommandContext(c.ctx, "ollama", "--version")
	return cmd.Run() == nil
}

// CheckLocalLlama checks if LocalLlama is installed
func (c *Checker) CheckLocalLlama() bool {
	// Check if localllama command exists
	cmd := exec.CommandContext(c.ctx, "localllama", "--version")
	if cmd.Run() == nil {
		return true
	}

	// Alternative: Check if LocalLlama container is running
	cmd = exec.CommandContext(c.ctx, "docker", "ps", "--filter", "name=localllama", "--format", "{{.Names}}")
	output, err := cmd.Output()
	if err == nil && strings.Contains(string(output), "localllama") {
		return true
	}

	return false
}

// GetSystemInfo returns comprehensive system information
func (c *Checker) GetSystemInfo() (SystemInfo, error) {
	info := SystemInfo{
		Platform:     runtime.GOOS,
		Architecture: runtime.GOARCH,
	}

	// RAM info
	total, available, err := c.GetRAM()
	if err != nil {
		return info, err
	}
	info.TotalRAM = total
	info.AvailableRAM = available

	// Disk info
	totalDisk, availableDisk, err := c.GetDisk("")
	if err != nil {
		return info, err
	}
	info.TotalDisk = totalDisk
	info.AvailableDisk = availableDisk

	// CPU info
	cpuCount, err := c.GetCPU()
	if err != nil {
		return info, err
	}
	info.CPUCount = cpuCount

	// Docker info
	dockerInstalled, dockerVersion, dockerErr := c.CheckDocker()
	info.DockerInstalled = dockerInstalled
	info.DockerVersion = dockerVersion
	info.DockerError = dockerErr

	// Ollama and LocalLlama
	info.OllamaInstalled = c.CheckOllama()
	info.LocalLlamaInstalled = c.CheckLocalLlama()

	return info, nil
}

// SystemInfo contains comprehensive system information
type SystemInfo struct {
	Platform            string
	Architecture        string
	TotalRAM            int64
	AvailableRAM        int64
	TotalDisk           int64
	AvailableDisk       int64
	CPUCount            int
	DockerInstalled     bool
	DockerVersion       string
	DockerError         error
	OllamaInstalled     bool
	LocalLlamaInstalled bool
}

// CheckRequirements validates system meets minimum requirements
func (c *Checker) CheckRequirements(minRAM, minDisk int64) error {
	info, err := c.GetSystemInfo()
	if err != nil {
		return fmt.Errorf("failed to get system info: %w", err)
	}

	if info.AvailableRAM < minRAM {
		return fmt.Errorf("insufficient RAM: need %s, have %s available",
			FormatBytes(minRAM), FormatBytes(info.AvailableRAM))
	}

	if info.AvailableDisk < minDisk {
		return fmt.Errorf("insufficient disk space: need %s, have %s available",
			FormatBytes(minDisk), FormatBytes(info.AvailableDisk))
	}

	if !info.DockerInstalled {
		return fmt.Errorf("Docker is not installed")
	}

	if info.DockerError != nil {
		return fmt.Errorf("Docker daemon is not running: %w", info.DockerError)
	}

	return nil
}

// RecommendModel suggests the best model based on available RAM
func (c *Checker) RecommendModel(models []ModelSpec, availableRAM int64) *ModelSpec {
	var best *ModelSpec

	for i := range models {
		model := &models[i]
		// Skip models that require more RAM than available
		if model.MinRAM > availableRAM {
			continue
		}

		// Prefer recommended models
		if model.Recommended {
			return model
		}

		// Otherwise pick the largest model that fits
		if best == nil || model.MinRAM > best.MinRAM {
			best = model
		}
	}

	// If no model fits, return the smallest one
	if best == nil && len(models) > 0 {
		smallest := &models[0]
		for i := range models {
			if models[i].MinRAM < smallest.MinRAM {
				smallest = &models[i]
			}
		}
		return smallest
	}

	return best
}

// ModelSpec represents an AI model specification
type ModelSpec struct {
	Name        string
	Size        int64 // Download size in bytes
	MinRAM      int64 // Minimum RAM required
	Recommended bool  // Is this the recommended model
}

// FormatBytes converts bytes to human-readable format
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

// ParseBytes converts human-readable format to bytes
func ParseBytes(s string) (int64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("empty string")
	}

	// Extract numeric part and unit
	var numStr string
	var unit string
	for i, r := range s {
		if (r < '0' || r > '9') && r != '.' {
			numStr = s[:i]
			unit = strings.ToUpper(strings.TrimSpace(s[i:]))
			break
		}
	}
	if numStr == "" {
		numStr = s
	}

	num, err := strconv.ParseFloat(numStr, 64)
	if err != nil {
		return 0, err
	}

	// Convert to bytes based on unit
	multiplier := int64(1)
	switch {
	case strings.HasPrefix(unit, "KB"):
		multiplier = 1024
	case strings.HasPrefix(unit, "MB"):
		multiplier = 1024 * 1024
	case strings.HasPrefix(unit, "GB"):
		multiplier = 1024 * 1024 * 1024
	case strings.HasPrefix(unit, "TB"):
		multiplier = 1024 * 1024 * 1024 * 1024
	}

	return int64(num * float64(multiplier)), nil
}
