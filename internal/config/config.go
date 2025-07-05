// internal/config/config.go
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"
	"gopkg.in/yaml.v2"
)

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

	// Sync current instance to viper before saving
	if instance != nil {
		syncToViper()
	}

	// Write config
	if err := viper.WriteConfigAs(configPath); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}

	return nil
}

func syncToViper() {
	if instance == nil {
		return
	}

	viper.Set("version", instance.Version)
	viper.Set("project.name", instance.Project.Name)
	viper.Set("project.type", instance.Project.Type)
	viper.Set("project.components", instance.Project.Components)

	// Clear all service keys first to ensure removed services are not persisted
	viper.Set("services", nil)

	// Only set service configurations if they are actually configured
	if instance.Services.AI.Port > 0 {
		viper.Set("services.ai.port", instance.Services.AI.Port)
		viper.Set("services.ai.models", instance.Services.AI.Models)
		viper.Set("services.ai.default", instance.Services.AI.Default)
	}

	if instance.Services.Database.Type != "" {
		viper.Set("services.database.type", instance.Services.Database.Type)
		viper.Set("services.database.version", instance.Services.Database.Version)
		viper.Set("services.database.port", instance.Services.Database.Port)
		viper.Set("services.database.extensions", instance.Services.Database.Extensions)
	}

	if instance.Services.MongoDB.Type != "" {
		viper.Set("services.mongodb.type", instance.Services.MongoDB.Type)
		viper.Set("services.mongodb.version", instance.Services.MongoDB.Version)
		viper.Set("services.mongodb.port", instance.Services.MongoDB.Port)
		viper.Set("services.mongodb.replica_set", instance.Services.MongoDB.ReplicaSet)
		viper.Set("services.mongodb.auth_enabled", instance.Services.MongoDB.AuthEnabled)
	}

	if instance.Services.Cache.Type != "" {
		viper.Set("services.cache.type", instance.Services.Cache.Type)
		viper.Set("services.cache.port", instance.Services.Cache.Port)
		viper.Set("services.cache.maxmemory", instance.Services.Cache.MaxMemory)
		viper.Set("services.cache.maxmemory_policy", instance.Services.Cache.MaxMemoryPolicy)
		viper.Set("services.cache.persistence", instance.Services.Cache.Persistence)
	}

	if instance.Services.Queue.Type != "" {
		viper.Set("services.queue.type", instance.Services.Queue.Type)
		viper.Set("services.queue.port", instance.Services.Queue.Port)
		viper.Set("services.queue.maxmemory", instance.Services.Queue.MaxMemory)
		viper.Set("services.queue.maxmemory_policy", instance.Services.Queue.MaxMemoryPolicy)
		viper.Set("services.queue.persistence", instance.Services.Queue.Persistence)
		viper.Set("services.queue.appendonly", instance.Services.Queue.AppendOnly)
		viper.Set("services.queue.appendfsync", instance.Services.Queue.AppendFsync)
	}

	if instance.Services.Storage.Type != "" {
		viper.Set("services.storage.type", instance.Services.Storage.Type)
		viper.Set("services.storage.port", instance.Services.Storage.Port)
		viper.Set("services.storage.console", instance.Services.Storage.Console)
	}

	if instance.Services.Whisper.Type != "" {
		viper.Set("services.whisper.type", instance.Services.Whisper.Type)
		viper.Set("services.whisper.port", instance.Services.Whisper.Port)
		viper.Set("services.whisper.model", instance.Services.Whisper.Model)
	}

	// Set resource configurations
	viper.Set("resources.memory_limit", instance.Resources.MemoryLimit)
	viper.Set("resources.cpu_limit", instance.Resources.CPULimit)

	// Set connectivity configurations
	viper.Set("connectivity.enabled", instance.Connectivity.Enabled)
	viper.Set("connectivity.tunnel.provider", instance.Connectivity.Tunnel.Provider)

	// Set CLI configurations
	viper.Set("cli.show_service_info", instance.CLI.ShowServiceInfo)
}

// GetDefaults returns minimal default configuration
func GetDefaults() *Config {
	return &Config{
		Version: "1",
		Project: ProjectConfig{
			Name: "localcloud-project",
			Type: "custom",
		},
		Services: ServicesConfig{
			// Empty services by default - let wizard populate them
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
	viper.SetDefault("project.components", defaults.Project.Components)

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

	// MongoDB defaults
	viper.SetDefault("services.mongodb.type", defaults.Services.MongoDB.Type)
	viper.SetDefault("services.mongodb.version", defaults.Services.MongoDB.Version)
	viper.SetDefault("services.mongodb.port", defaults.Services.MongoDB.Port)
	viper.SetDefault("services.mongodb.replica_set", defaults.Services.MongoDB.ReplicaSet)
	viper.SetDefault("services.mongodb.auth_enabled", defaults.Services.MongoDB.AuthEnabled)

	// Resource defaults
	viper.SetDefault("resources.memory_limit", defaults.Resources.MemoryLimit)
	viper.SetDefault("resources.cpu_limit", defaults.Resources.CPULimit)

	// Connectivity defaults
	viper.SetDefault("connectivity.enabled", defaults.Connectivity.Enabled)
	viper.SetDefault("connectivity.tunnel.provider", defaults.Connectivity.Tunnel.Provider)

	// CLI defaults
	viper.SetDefault("cli.show_service_info", defaults.CLI.ShowServiceInfo)
}

// GetViper returns the viper instance
func GetViper() *viper.Viper {
	return viper.GetViper()
}

// GenerateDefault generates a default configuration file content as a byte slice
func GenerateDefault(projectName, projectType string) ([]byte, error) {
	cfg := GetDefaults()

	// Update with provided values
	if projectName != "" {
		cfg.Project.Name = projectName
	}
	if projectType != "" {
		cfg.Project.Type = projectType
	}

	// Marshal the config to YAML
	yamlBytes, err := yaml.Marshal(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal config to YAML: %w", err)
	}

	return yamlBytes, nil
}
