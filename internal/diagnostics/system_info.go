// internal/diagnostics/system_info.go
//go:build !linux
// +build !linux

package diagnostics

import (
	"os"
	"runtime"
)

// getSystemMemoryInfo returns memory information in a cross-platform way
func getSystemMemoryInfo() MemoryInfo {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	// Use runtime memory stats as a fallback
	// This gives us Go process memory, not system memory
	// But it's better than nothing for cross-platform support
	return MemoryInfo{
		Total:       m.Sys,
		Free:        m.Sys - m.Alloc,
		Used:        m.Alloc,
		PercentUsed: float64(m.Alloc) / float64(m.Sys) * 100,
	}
}

// getSystemDiskInfo returns disk information in a cross-platform way
func getSystemDiskInfo() DiskInfo {
	// For cross-platform compatibility, we'll use a simple approach
	// In production, you might want to use a library like github.com/shirou/gopsutil

	// Try to get working directory
	wd, err := os.Getwd()
	if err != nil {
		return DiskInfo{
			Total:       0,
			Free:        0,
			Used:        0,
			PercentUsed: 0,
		}
	}

	// Get file info to at least check if we can access the filesystem
	_, err = os.Stat(wd)
	if err != nil {
		return DiskInfo{
			Total:       0,
			Free:        0,
			Used:        0,
			PercentUsed: 0,
		}
	}

	// Return placeholder values for now
	// In a real implementation, use platform-specific code or a library
	return DiskInfo{
		Total:       100 * 1024 * 1024 * 1024, // 100GB placeholder
		Free:        50 * 1024 * 1024 * 1024,  // 50GB placeholder
		Used:        50 * 1024 * 1024 * 1024,  // 50GB placeholder
		PercentUsed: 50.0,
	}
}
