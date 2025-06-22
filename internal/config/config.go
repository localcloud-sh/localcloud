// internal/config/config.go
package config

import (
	"fmt"
	"github.com/spf13/viper"
	"os"
	"path/filepath"
)

// Config represents configuration
type Config struct {
	Version      string              `mapstructure:"version"`
	Project      ProjectConfig       `mapstructure:"project"`
	Services     ServicesConfig      `mapstructure:"services"`
	Resources    ResourceConfig      `mapstructure:"resources"`
	Connectivity *ConnectivityConfig `mapstructure:"connectivity"`
}

type ProjectConfig struct {
	Name string `mapstructure:"name"`
	Type string `mapstructure:"type"`
}

type ServicesConfig struct {
	AI       AIConfig       `mapstructure:"ai"`
	Database DatabaseConfig `mapstructure:"database"`
	Cache    CacheConfig    `mapstructure:"cache"`
	Storage  StorageConfig  `mapstructure:"storage"`
}

type AIConfig struct {
	Models  []string `mapstructure:"models"`
	Default string   `mapstructure:"default"`
	Port    int      `mapstructure:"port"`
}

type DatabaseConfig struct {
	Type       string   `mapstructure:"type"`
	Version    string   `mapstructure:"version"`
	Port       int      `mapstructure:"port"`
	Extensions []string `mapstructure:"extensions"`
}

type CacheConfig struct {
	Type      string `mapstructure:"type"`
	Port      int    `mapstructure:"port"`
	MaxMemory string `mapstructure:"maxmemory"`
}

type StorageConfig struct {
	Type    string `mapstructure:"type"`
	Port    int    `mapstructure:"port"`
	Console int    `mapstructure:"console"`
}

type ResourceConfig struct {
	MemoryLimit string `mapstructure:"memory_limit"`
	CPULimit    string `mapstructure:"cpu_limit"`
}

// Connectivity Configuration
type ConnectivityConfig struct {
	Enabled bool         `mapstructure:"enabled"`
	MDNS    MDNSConfig   `mapstructure:"mdns"`
	Tunnel  TunnelConfig `mapstructure:"tunnel"`
}

type MDNSConfig struct {
	Enabled     bool   `mapstructure:"enabled"`
	ServiceName string `mapstructure:"service_name"`
	Domain      string `mapstructure:"domain"`
}

type TunnelConfig struct {
	Provider   string           `mapstructure:"provider"` // auto, cloudflare, ngrok
	Persist    bool             `mapstructure:"persist"`
	Domain     string           `mapstructure:"domain"`
	TargetURL  string           `mapstructure:"target_url"` // Custom URL to tunnel
	Cloudflare CloudflareConfig `mapstructure:"cloudflare"`
	Ngrok      NgrokConfig      `mapstructure:"ngrok"`
}

type CloudflareConfig struct {
	TunnelID    string `mapstructure:"tunnel_id"`
	Credentials string `mapstructure:"credentials"`
}

type NgrokConfig struct {
	AuthToken string `mapstructure:"auth_token"`
	Region    string `mapstructure:"region"`
}

var (
	cfg *Config
	v   *viper.Viper
)

// Init initializes config
func Init(configFile string) error {
	v = viper.New()

	// Set config type
	v.SetConfigType("yaml")

	// If specific config file provided
	if configFile != "" && configFile != "./.localcloud/config.yaml" {
		v.SetConfigFile(configFile)
	} else {
		// Set config name
		v.SetConfigName("config")

		// Add search paths
		v.AddConfigPath("./.localcloud")
		v.AddConfigPath(".")

		// Try to find project root
		if projectRoot, err := findProjectRoot(); err == nil {
			v.AddConfigPath(filepath.Join(projectRoot, ".localcloud"))
		}
	}

	// Set defaults
	setDefaults()

	// Bind environment variables
	bindEnvVariables()

	// Read config
	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			// No config file found, use defaults
			cfg = &Config{}
			v.Unmarshal(cfg)
			return nil
		}
		return fmt.Errorf("error reading config: %w", err)
	}

	// Unmarshal config
	cfg = &Config{}
	if err := v.Unmarshal(cfg); err != nil {
		return fmt.Errorf("error unmarshaling config: %w", err)
	}

	return nil
}

// setDefaults sets default values
func setDefaults() {
	v.SetDefault("version", "1")
	v.SetDefault("project.name", "my-project")
	v.SetDefault("project.type", "general")
	v.SetDefault("services.ai.port", 11434)
	v.SetDefault("services.database.port", 5432)
	v.SetDefault("services.cache.port", 6379)
	v.SetDefault("services.storage.port", 9000)
	v.SetDefault("services.storage.console", 9001)
	v.SetDefault("resources.memory_limit", "4GB")
	v.SetDefault("resources.cpu_limit", "2")

	// Connectivity defaults
	v.SetDefault("connectivity.enabled", true)
	v.SetDefault("connectivity.mdns.enabled", true)
	v.SetDefault("connectivity.mdns.service_name", "_localcloud._tcp")
	v.SetDefault("connectivity.mdns.domain", ".local")
	v.SetDefault("connectivity.tunnel.provider", "auto")
	v.SetDefault("connectivity.tunnel.persist", false)
}

// bindEnvVariables binds environment variables
func bindEnvVariables() {
	// Tunnel environment variables
	v.BindEnv("connectivity.tunnel.cloudflare.tunnel_id", "CLOUDFLARE_TUNNEL_ID")
	v.BindEnv("connectivity.tunnel.cloudflare.credentials", "CLOUDFLARE_TUNNEL_CREDS")
	v.BindEnv("connectivity.tunnel.ngrok.auth_token", "NGROK_AUTH_TOKEN")
	v.BindEnv("connectivity.tunnel.domain", "TUNNEL_DOMAIN")
}

// Get returns current config
func Get() *Config {
	if cfg == nil {
		// Try to initialize with defaults if not initialized
		Init("")
	}
	return cfg
}

// GetViper returns viper instance
func GetViper() *viper.Viper {
	if v == nil {
		v = viper.New()
	}
	return v
}

// findProjectRoot finds the project root by looking for .localcloud directory
func findProjectRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	for {
		configPath := filepath.Join(dir, ".localcloud")
		if _, err := os.Stat(configPath); err == nil {
			return dir, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	return "", fmt.Errorf(".localcloud directory not found")
}

// GenerateDefault generates default config
func GenerateDefault(projectName, projectType string) string {
	return fmt.Sprintf(`version: "1"
project:
  name: "%s"
  type: "%s"

services:
  ai:
    models:
      - qwen2.5:3b
    default: qwen2.5:3b
    port: 11434
  
  database:
    type: postgres
    version: "16"
    port: 5432
    extensions:
      - uuid-ossp
  
  cache:
    type: redis
    port: 6379
    maxmemory: "512mb"

  storage:
    type: minio
    port: 9000
    console: 9001

resources:
  memory_limit: "4GB"
  cpu_limit: "2"

connectivity:
  enabled: true
  
  mdns:
    enabled: true
    service_name: "_localcloud._tcp"
    domain: ".local"
  
  tunnel:
    provider: "auto"  # auto, cloudflare, ngrok
    persist: false    # Create persistent tunnel
    # domain: "my-app.example.com"  # Optional custom domain
    
    # Cloudflare settings (set via env vars)
    # cloudflare:
    #   tunnel_id: ""
    #   credentials: ""
    
    # Ngrok settings (set via env vars)
    # ngrok:
    #   auth_token: ""
    #   region: "us"
`, projectName, projectType)
}

// SaveConfig saves current configuration to file
func SaveConfig() error {
	if v == nil {
		return fmt.Errorf("viper not initialized")
	}

	configPath := ".localcloud/config.yaml"
	return v.WriteConfigAs(configPath)
}
