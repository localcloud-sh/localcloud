package config

import (
	"fmt"
	"github.com/spf13/viper"
)

// Config represents configuration
type Config struct {
	Version   string
	Project   ProjectConfig
	Services  ServicesConfig
	Resources ResourceConfig
}

type ProjectConfig struct {
	Name string
	Type string
}

type ServicesConfig struct {
	AI       AIConfig
	Database DatabaseConfig
	Cache    CacheConfig
	Storage  StorageConfig
}

type AIConfig struct {
	Models  []string
	Default string
	Port    int
}

type DatabaseConfig struct {
	Type       string
	Version    string
	Port       int
	Extensions []string
}

type CacheConfig struct {
	Type      string
	Port      int
	MaxMemory string
}

type StorageConfig struct {
	Type    string
	Port    int
	Console int
}

type ResourceConfig struct {
	MemoryLimit string
	CPULimit    string
}

var (
	cfg *Config
	v   *viper.Viper
)

// Init initializes config
func Init(configFile string) error {
	cfg = &Config{
		Version: "1",
		Project: ProjectConfig{
			Name: "my-project",
			Type: "general",
		},
	}
	v = viper.New()
	return nil
}

// Get returns current config
func Get() *Config {
	if cfg == nil {
		cfg = &Config{}
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

resources:
  memory_limit: "4GB"
  cpu_limit: "2"
`, projectName, projectType)
}
