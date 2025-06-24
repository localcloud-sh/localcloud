// internal/config/config.go
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"
)

// Config represents the LocalCloud configuration
type Config struct {
	Version      string             `mapstructure:"version"`
	Project      ProjectConfig      `mapstructure:"project"`
	Services     ServicesConfig     `mapstructure:"services"`
	Resources    ResourcesConfig    `mapstructure:"resources"`
	Connectivity ConnectivityConfig `mapstructure:"connectivity"`
	CLI          CLIConfig          `mapstructure:"cli"`
}

// ProjectConfig represents project configuration
type ProjectConfig struct {
	Name string `mapstructure:"name"`
	Type string `mapstructure:"type"`
}

// ServicesConfig represents all service configurations
type ServicesConfig struct {
	AI       AIConfig       `mapstructure:"ai"`
	Database DatabaseConfig `mapstructure:"database"`
	Cache    CacheConfig    `mapstructure:"cache"`
	Queue    QueueConfig    `mapstructure:"queue"`
	Storage  StorageConfig  `mapstructure:"storage"`
}

// AIConfig represents AI service configuration
type AIConfig struct {
	Port    int      `mapstructure:"port"`
	Models  []string `mapstructure:"models"`
	Default string   `mapstructure:"default"`
}

// DatabaseConfig represents database service configuration
type DatabaseConfig struct {
	Type       string   `mapstructure:"type"`
	Version    string   `mapstructure:"version"`
	Port       int      `mapstructure:"port"`
	Extensions []string `mapstructure:"extensions"`
}

// CacheConfig represents cache service configuration
type CacheConfig struct {
	Type            string `mapstructure:"type"`
	Port            int    `mapstructure:"port"`
	MaxMemory       string `mapstructure:"maxmemory"`
	MaxMemoryPolicy string `mapstructure:"maxmemory_policy"`
	Persistence     bool   `mapstructure:"persistence"`
}

// QueueConfig represents queue service configuration
type QueueConfig struct {
	Type            string `mapstructure:"type"`
	Port            int    `mapstructure:"port"`
	MaxMemory       string `mapstructure:"maxmemory"`
	MaxMemoryPolicy string `mapstructure:"maxmemory_policy"`
	Persistence     bool   `mapstructure:"persistence"`
	AppendOnly      bool   `mapstructure:"appendonly"`
	AppendFsync     string `mapstructure:"appendfsync"`
}

// StorageConfig represents storage service configuration
type StorageConfig struct {
	Type    string `mapstructure:"type"`
	Port    int    `mapstructure:"port"`
	Console int    `mapstructure:"console"`
}

// ResourcesConfig represents resource limits
type ResourcesConfig struct {
	MemoryLimit string `mapstructure:"memory_limit"`
	CPULimit    string `mapstructure:"cpu_limit"`
}

// ConnectivityConfig represents connectivity configuration
type ConnectivityConfig struct {
	Enabled bool         `mapstructure:"enabled"`
	MDNS    MDNSConfig   `mapstructure:"mdns"`
	Tunnel  TunnelConfig `mapstructure:"tunnel"`
}

// MDNSConfig represents mDNS configuration
type MDNSConfig struct {
	Enabled bool `mapstructure:"enabled"`
}

// TunnelConfig represents tunnel configuration
type TunnelConfig struct {
	Provider   string           `mapstructure:"provider"`
	Persist    bool             `mapstructure:"persist"`
	Domain     string           `mapstructure:"domain"`
	TargetURL  string           `mapstructure:"target_url"`
	Cloudflare CloudflareConfig `mapstructure:"cloudflare"`
	Ngrok      NgrokConfig      `mapstructure:"ngrok"`
}

// CloudflareConfig represents Cloudflare tunnel configuration
type CloudflareConfig struct {
	TunnelID    string `mapstructure:"tunnel_id"`
	Credentials string `mapstructure:"credentials"`
}

// NgrokConfig represents Ngrok tunnel configuration
type NgrokConfig struct {
	AuthToken string `mapstructure:"auth_token"`
	Region    string `mapstructure:"region"`
}

// CLIConfig represents CLI-specific configuration
type CLIConfig struct {
	ShowServiceInfo bool `mapstructure:"show_service_info"`
}

var (
	// Global config instance
	instance *Config
	// Config file path
	configPath string
)

// Init initializes the configuration
func Init(cfgFile string) error {
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
		configPath = cfgFile
	} else {
		// Look for config in project directory
		viper.AddConfigPath("./.localcloud")
		viper.AddConfigPath(".")
		viper.SetConfigName("config")
		viper.SetConfigType("yaml")
	}

	// Set environment variable prefix
	viper.SetEnvPrefix("LOCALCLOUD")
	viper.AutomaticEnv()
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	// Set defaults
	setDefaults()

	// Read config file
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			// Config file not found; use defaults
			instance = GetDefaults()
			return nil
		}
		return fmt.Errorf("failed to read config: %w", err)
	}

	// Unmarshal config
	instance = &Config{}
	if err := viper.Unmarshal(instance); err != nil {
		return fmt.Errorf("failed to unmarshal config: %w", err)
	}

	return nil
}

// Get returns the current configuration
func Get() *Config {
	if instance == nil {
		instance = GetDefaults()
	}
	return instance
}

// Save saves the current configuration to file
func Save() error {
	if configPath == "" {
		configPath = ".localcloud/config.yaml"
	}

	// Ensure directory exists
	dir := filepath.Dir(configPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Write config
	if err := viper.WriteConfigAs(configPath); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}

	return nil
}

// GetDefaults returns default configuration
func GetDefaults() *Config {
	return &Config{
		Version: "1",
		Project: ProjectConfig{
			Name: "localcloud-project",
			Type: "custom",
		},
		Services: ServicesConfig{
			AI: AIConfig{
				Port:    11434,
				Models:  []string{"qwen2.5:3b"},
				Default: "qwen2.5:3b",
			},
			Database: DatabaseConfig{
				Type:       "postgres",
				Version:    "16",
				Port:       5432,
				Extensions: []string{},
			},
			Cache: CacheConfig{
				Type:            "redis",
				Port:            6379,
				MaxMemory:       "512mb",
				MaxMemoryPolicy: "allkeys-lru",
				Persistence:     false,
			},
			Queue: QueueConfig{
				Type:            "redis",
				Port:            6380,
				MaxMemory:       "1gb",
				MaxMemoryPolicy: "noeviction",
				Persistence:     true,
				AppendOnly:      true,
				AppendFsync:     "everysec",
			},
			Storage: StorageConfig{
				Type:    "minio",
				Port:    9000,
				Console: 9001,
			},
		},
		Resources: ResourcesConfig{
			MemoryLimit: "4GB",
			CPULimit:    "2",
		},
		Connectivity: ConnectivityConfig{
			Enabled: false,
			Tunnel: TunnelConfig{
				Provider: "cloudflare",
			},
		},
		CLI: CLIConfig{
			ShowServiceInfo: true,
		},
	}
}

// setDefaults sets default values in Viper
func setDefaults() {
	defaults := GetDefaults()

	viper.SetDefault("version", defaults.Version)
	viper.SetDefault("project.name", defaults.Project.Name)
	viper.SetDefault("project.type", defaults.Project.Type)

	// AI service defaults
	viper.SetDefault("services.ai.port", defaults.Services.AI.Port)
	viper.SetDefault("services.ai.models", defaults.Services.AI.Models)
	viper.SetDefault("services.ai.default", defaults.Services.AI.Default)

	// Database defaults
	viper.SetDefault("services.database.type", defaults.Services.Database.Type)
	viper.SetDefault("services.database.version", defaults.Services.Database.Version)
	viper.SetDefault("services.database.port", defaults.Services.Database.Port)

	// Cache defaults
	viper.SetDefault("services.cache.type", defaults.Services.Cache.Type)
	viper.SetDefault("services.cache.port", defaults.Services.Cache.Port)
	viper.SetDefault("services.cache.maxmemory", defaults.Services.Cache.MaxMemory)
	viper.SetDefault("services.cache.maxmemory_policy", defaults.Services.Cache.MaxMemoryPolicy)
	viper.SetDefault("services.cache.persistence", defaults.Services.Cache.Persistence)

	// Queue defaults
	viper.SetDefault("services.queue.type", defaults.Services.Queue.Type)
	viper.SetDefault("services.queue.port", defaults.Services.Queue.Port)
	viper.SetDefault("services.queue.maxmemory", defaults.Services.Queue.MaxMemory)
	viper.SetDefault("services.queue.maxmemory_policy", defaults.Services.Queue.MaxMemoryPolicy)
	viper.SetDefault("services.queue.persistence", defaults.Services.Queue.Persistence)
	viper.SetDefault("services.queue.appendonly", defaults.Services.Queue.AppendOnly)
	viper.SetDefault("services.queue.appendfsync", defaults.Services.Queue.AppendFsync)

	// Storage defaults
	viper.SetDefault("services.storage.type", defaults.Services.Storage.Type)
	viper.SetDefault("services.storage.port", defaults.Services.Storage.Port)
	viper.SetDefault("services.storage.console", defaults.Services.Storage.Console)

	// Resource defaults
	viper.SetDefault("resources.memory_limit", defaults.Resources.MemoryLimit)
	viper.SetDefault("resources.cpu_limit", defaults.Resources.CPULimit)

	// Connectivity defaults
	viper.SetDefault("connectivity.enabled", defaults.Connectivity.Enabled)
	viper.SetDefault("connectivity.tunnel.provider", defaults.Connectivity.Tunnel.Provider)

	// CLI defaults
	viper.SetDefault("cli.show_service_info", defaults.CLI.ShowServiceInfo)
}

// Add these functions to internal/config/config.go

// GetViper returns the viper instance
func GetViper() *viper.Viper {
	return viper.GetViper()
}

// GenerateDefault generates a default configuration file
func GenerateDefault(projectName, projectType string) ([]byte, error) {
	cfg := GetDefaults()

	// Update with provided values
	if projectName != "" {
		cfg.Project.Name = projectName
	}
	if projectType != "" {
		cfg.Project.Type = projectType
	}

	// Convert to YAML
	v := viper.New()
	v.Set("version", cfg.Version)
	v.Set("project", cfg.Project)
	v.Set("services", cfg.Services)
	v.Set("resources", cfg.Resources)
	v.Set("connectivity", cfg.Connectivity)
	v.Set("cli", cfg.CLI)

	// Write to buffer
	var buf strings.Builder
	if err := v.WriteConfigAs("config.yaml"); err != nil {
		// Fallback to manual YAML generation
		buf.WriteString(fmt.Sprintf(`version: "%s"
project:
  name: "%s"
  type: "%s"

services:
  ai:
    port: %d
    models:
      - %s
    default: %s
  
  database:
    type: %s
    version: "%s"
    port: %d
    extensions: []
  
  cache:
    type: %s
    port: %d
    maxmemory: %s
    maxmemory_policy: %s
    persistence: %v
  
  queue:
    type: %s
    port: %d
    maxmemory: %s
    maxmemory_policy: %s
    persistence: %v
    appendonly: %v
    appendfsync: %s
  
  storage:
    type: %s
    port: %d
    console: %d

resources:
  memory_limit: "%s"
  cpu_limit: "%s"

connectivity:
  enabled: %v
  mdns:
    enabled: false
  tunnel:
    provider: "%s"

cli:
  show_service_info: %v
`,
			cfg.Version,
			cfg.Project.Name,
			cfg.Project.Type,
			cfg.Services.AI.Port,
			cfg.Services.AI.Models[0],
			cfg.Services.AI.Default,
			cfg.Services.Database.Type,
			cfg.Services.Database.Version,
			cfg.Services.Database.Port,
			cfg.Services.Cache.Type,
			cfg.Services.Cache.Port,
			cfg.Services.Cache.MaxMemory,
			cfg.Services.Cache.MaxMemoryPolicy,
			cfg.Services.Cache.Persistence,
			cfg.Services.Queue.Type,
			cfg.Services.Queue.Port,
			cfg.Services.Queue.MaxMemory,
			cfg.Services.Queue.MaxMemoryPolicy,
			cfg.Services.Queue.Persistence,
			cfg.Services.Queue.AppendOnly,
			cfg.Services.Queue.AppendFsync,
			cfg.Services.Storage.Type,
			cfg.Services.Storage.Port,
			cfg.Services.Storage.Console,
			cfg.Resources.MemoryLimit,
			cfg.Resources.CPULimit,
			cfg.Connectivity.Enabled,
			cfg.Connectivity.Tunnel.Provider,
			cfg.CLI.ShowServiceInfo,
		))
		return []byte(buf.String()), nil
	}

	return []byte{}, nil
}
