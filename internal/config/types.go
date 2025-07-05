// internal/config/types.go
package config

// Config represents the main configuration structure
type Config struct {
	Version      string             `yaml:"version" json:"version"`
	Project      ProjectConfig      `yaml:"project" json:"project"`
	Services     ServicesConfig     `yaml:"services" json:"services"`
	Resources    ResourcesConfig    `yaml:"resources" json:"resources"`
	Connectivity ConnectivityConfig `yaml:"connectivity" json:"connectivity"`
	CLI          CLIConfig          `yaml:"cli" json:"cli"`
}

// ProjectConfig represents project configuration
type ProjectConfig struct {
	Name       string   `yaml:"name" json:"name"`
	Type       string   `yaml:"type" json:"type"`
	Components []string `yaml:"components" json:"components"` // Seçilen componentler
}

// ServicesConfig represents all services configuration
type ServicesConfig struct {
	AI       AIConfig       `yaml:"ai" json:"ai"`
	Database DatabaseConfig `yaml:"database" json:"database"`
	MongoDB  MongoDBConfig  `yaml:"mongodb" json:"mongodb"`
	Cache    CacheConfig    `yaml:"cache" json:"cache"`
	Queue    QueueConfig    `yaml:"queue" json:"queue"`
	Storage  StorageConfig  `yaml:"storage" json:"storage"`
	Whisper  WhisperConfig  `yaml:"whisper" json:"whisper"` // Bu satırı ekle
}

// AIConfig represents AI service configuration
type AIConfig struct {
	Port    int      `yaml:"port" json:"port"`
	Models  []string `yaml:"models" json:"models"`
	Default string   `yaml:"default" json:"default"`
}

// DatabaseConfig represents database service configuration
type DatabaseConfig struct {
	Type       string   `yaml:"type" json:"type"`
	Version    string   `yaml:"version" json:"version"`
	Port       int      `yaml:"port" json:"port"`
	Extensions []string `yaml:"extensions,omitempty" json:"extensions,omitempty"`
}

// CacheConfig represents cache service configuration
type CacheConfig struct {
	Type            string `yaml:"type" json:"type"`
	Port            int    `yaml:"port" json:"port"`
	MaxMemory       string `yaml:"maxmemory" json:"maxmemory"`
	MaxMemoryPolicy string `yaml:"maxmemory_policy" json:"maxmemory_policy"`
	Persistence     bool   `yaml:"persistence" json:"persistence"`
}

// QueueConfig represents queue service configuration
type QueueConfig struct {
	Type            string `yaml:"type" json:"type"`
	Port            int    `yaml:"port" json:"port"`
	MaxMemory       string `yaml:"maxmemory" json:"maxmemory"`
	MaxMemoryPolicy string `yaml:"maxmemory_policy" json:"maxmemory_policy"`
	Persistence     bool   `yaml:"persistence" json:"persistence"`
	AppendOnly      bool   `yaml:"appendonly" json:"appendonly"`
	AppendFsync     string `yaml:"appendfsync" json:"appendfsync"`
}

// StorageConfig represents storage service configuration
type StorageConfig struct {
	Type    string `yaml:"type" json:"type"`
	Port    int    `yaml:"port" json:"port"`
	Console int    `yaml:"console" json:"console"`
}

// MongoDBConfig represents MongoDB service configuration
type MongoDBConfig struct {
	Type        string `yaml:"type" json:"type"`
	Version     string `yaml:"version" json:"version"`
	Port        int    `yaml:"port" json:"port"`
	ReplicaSet  bool   `yaml:"replica_set" json:"replica_set"`
	AuthEnabled bool   `yaml:"auth_enabled" json:"auth_enabled"`
}

// ResourcesConfig represents resource limits configuration
type ResourcesConfig struct {
	MemoryLimit string `yaml:"memory_limit" json:"memory_limit"`
	CPULimit    string `yaml:"cpu_limit" json:"cpu_limit"`
}

// ConnectivityConfig represents connectivity configuration
type ConnectivityConfig struct {
	Enabled bool         `yaml:"enabled" json:"enabled"`
	MDNS    MDNSConfig   `yaml:"mdns" json:"mdns"`
	Tunnel  TunnelConfig `yaml:"tunnel" json:"tunnel"`
}

// MDNSConfig represents mDNS configuration
type MDNSConfig struct {
	Enabled bool `yaml:"enabled" json:"enabled"`
}

// TunnelConfig represents tunnel configuration
type TunnelConfig struct {
	Provider   string           `yaml:"provider" json:"provider"`
	Persist    bool             `yaml:"persist,omitempty" json:"persist,omitempty"`
	Domain     string           `yaml:"domain,omitempty" json:"domain,omitempty"`
	TargetURL  string           `yaml:"target_url,omitempty" json:"target_url,omitempty"`
	Cloudflare CloudflareConfig `yaml:"cloudflare,omitempty" json:"cloudflare,omitempty"`
	Ngrok      NgrokConfig      `yaml:"ngrok,omitempty" json:"ngrok,omitempty"`
}

// CloudflareConfig represents Cloudflare tunnel configuration
type CloudflareConfig struct {
	TunnelID    string `yaml:"tunnel_id,omitempty" json:"tunnel_id,omitempty"`
	Secret      string `yaml:"secret,omitempty" json:"secret,omitempty"`
	Credentials string `yaml:"credentials,omitempty" json:"credentials,omitempty"`
}

// NgrokConfig represents Ngrok tunnel configuration
type NgrokConfig struct {
	AuthToken string `yaml:"auth_token,omitempty" json:"auth_token,omitempty"`
}

// CLIConfig represents CLI configuration
type CLIConfig struct {
	ShowServiceInfo bool `yaml:"show_service_info" json:"show_service_info"`
}
type WhisperConfig struct {
	Type  string `yaml:"type" json:"type"`
	Port  int    `yaml:"port" json:"port"`
	Model string `yaml:"model" json:"model"`
}
