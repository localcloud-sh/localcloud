// internal/config/types.go
// Package docker provides Docker container management for LocalCloud
package config

import "time"

// ContainerConfig represents container configuration
type ContainerConfig struct {
	Name          string
	Image         string
	Command       []string
	Env           map[string]string
	Ports         []PortBinding
	Volumes       []VolumeMount
	Networks      []string
	Labels        map[string]string
	RestartPolicy string
	HealthCheck   *HealthCheckConfig
	Memory        int64
	CPUQuota      int64
}

// PortBinding represents a port binding configuration
type PortBinding struct {
	ContainerPort string
	HostPort      string
	Protocol      string
}

// VolumeMount represents a volume mount configuration
type VolumeMount struct {
	Source   string
	Target   string
	Type     string // "bind" or "volume"
	ReadOnly bool
}

// HealthCheckConfig represents health check configuration
type HealthCheckConfig struct {
	Test        []string
	Interval    int
	Timeout     int
	Retries     int
	StartPeriod int
}

// ContainerInfo represents container information
type ContainerInfo struct {
	ID         string
	Name       string
	Image      string
	Status     string
	State      string
	Health     string
	Ports      map[string]string
	Created    int64
	StartedAt  int64
	Memory     int64
	CPUPercent float64
}

// ContainerStats represents container resource usage statistics
type ContainerStats struct {
	CPUPercent    float64
	MemoryUsage   uint64
	MemoryLimit   uint64
	MemoryPercent float64
	NetworkRx     uint64
	NetworkTx     uint64
	BlockRead     uint64
	BlockWrite    uint64
}

// PullProgress represents image pull progress
type PullProgress struct {
	Status         string
	Progress       string
	ProgressDetail ProgressDetail
	Error          string
}

// ProgressDetail represents detailed progress information
type ProgressDetail struct {
	Current int64
	Total   int64
}

// ServiceProgress represents service operation progress
type ServiceProgress struct {
	Service string
	Status  string // "starting", "started", "failed", "stopping", "stopped"
	Error   string
	Details map[string]interface{}
}

// ServiceStatus represents the status of a service
type ServiceStatus struct {
	Name        string
	Status      string
	Health      string
	Ports       map[string]string
	StartedAt   time.Time
	Error       string
	ContainerID string
}

// ImageInfo represents Docker image information
type ImageInfo struct {
	ID      string
	Name    string
	Tag     string
	Size    int64
	Created int64
}

// VolumeInfo represents Docker volume information
type VolumeInfo struct {
	Name       string
	Driver     string
	Mountpoint string
	Created    time.Time
	Labels     map[string]string
	Scope      string
	Size       int64
}

// NetworkInfo represents Docker network information
type NetworkInfo struct {
	ID         string
	Name       string
	Driver     string
	Scope      string
	Internal   bool
	Attachable bool
	Created    time.Time
}
