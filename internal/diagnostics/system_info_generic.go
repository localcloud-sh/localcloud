//go:build !linux
// +build !linux

// internal/diagnostics/system_info_generic.go
package diagnostics

import (
	"runtime"
)

// getSystemMemoryInfo returns memory information for non-Linux platforms
func getSystemMemoryInfo() MemoryInfo {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	// Use runtime memory stats as a fallback
	// This gives us Go process memory, not system memory
	return MemoryInfo{
		Total:       m.Sys,
		Free:        m.Sys - m.Alloc,
		Used:        m.Alloc,
		PercentUsed: float64(m.Alloc) / float64(m.Sys) * 100,
	}
}

// getSystemDiskInfo returns disk information for non-Linux platforms
func getSystemDiskInfo() DiskInfo {
	// Return placeholder values for cross-platform compatibility
	return DiskInfo{
		Total:       100 * 1024 * 1024 * 1024, // 100GB placeholder
		Free:        50 * 1024 * 1024 * 1024,  // 50GB placeholder
		Used:        50 * 1024 * 1024 * 1024,  // 50GB placeholder
		PercentUsed: 50.0,
	}
}
