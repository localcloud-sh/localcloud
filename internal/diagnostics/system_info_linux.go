//go:build linux
// +build linux

// internal/diagnostics/system_info_linux.go
package diagnostics

import (
	"syscall"
)

// getSystemMemoryInfo returns memory information on Linux
func getSystemMemoryInfo() MemoryInfo {
	var info syscall.Sysinfo_t
	err := syscall.Sysinfo(&info)
	if err != nil {
		// Fallback to generic implementation
		return MemoryInfo{
			Total:       0,
			Free:        0,
			Used:        0,
			PercentUsed: 0,
		}
	}

	return MemoryInfo{
		Total:       info.Totalram,
		Free:        info.Freeram,
		Used:        info.Totalram - info.Freeram,
		PercentUsed: float64(info.Totalram-info.Freeram) / float64(info.Totalram) * 100,
	}
}

// getSystemDiskInfo returns disk information on Linux
func getSystemDiskInfo() DiskInfo {
	var stat syscall.Statfs_t
	err := syscall.Statfs(".", &stat)
	if err != nil {
		// Fallback to generic implementation
		return DiskInfo{
			Total:       0,
			Free:        0,
			Used:        0,
			PercentUsed: 0,
		}
	}

	total := stat.Blocks * uint64(stat.Bsize)
	free := stat.Bavail * uint64(stat.Bsize)
	used := (stat.Blocks - stat.Bavail) * uint64(stat.Bsize)
	percentUsed := float64(stat.Blocks-stat.Bavail) / float64(stat.Blocks) * 100

	return DiskInfo{
		Total:       total,
		Free:        free,
		Used:        used,
		PercentUsed: percentUsed,
	}
}
