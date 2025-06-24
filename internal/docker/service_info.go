// internal/docker/service_info.go
package docker

import (
	"fmt"
	"github.com/fatih/color"
)

// ServiceInfoProvider provides detailed service information and examples
type ServiceInfoProvider interface {
	PrintInfo()
}

// RedisServiceInfo provides Redis-specific information and examples
type RedisServiceInfo struct {
	Port int
}

// PrintInfo displays Redis service information with queue examples
func (r *RedisServiceInfo) PrintInfo() {
	bold := color.New(color.Bold).SprintFunc()
	green := color.New(color.FgGreen).SprintFunc()
	cyan := color.New(color.FgCyan).SprintFunc()

	fmt.Println()
	fmt.Printf("%s %s\n", green("✓"), bold("Redis (Cache + Queue)"))
	fmt.Printf("  URL: %s\n", cyan(fmt.Sprintf("redis://localhost:%d", r.Port)))
	fmt.Println()
	fmt.Println("  " + bold("Try these commands:"))
	fmt.Println()

	// Connection test
	fmt.Println("  " + bold("Test connection:"))
	fmt.Printf("    %s\n", cyan("redis-cli ping"))
	fmt.Println()

	// Queue operations
	fmt.Println("  " + bold("Queue operations:"))
	fmt.Printf("    %s %s\n", cyan("redis-cli LPUSH jobs '{\"id\":\"123\",\"task\":\"process\"}'"), "# Add job to queue")
	fmt.Printf("    %s %s\n", cyan("redis-cli BRPOP jobs 0"), "# Get job (blocking)")
	fmt.Printf("    %s %s\n", cyan("redis-cli LLEN jobs"), "# Check queue length")
	fmt.Println()

	// Priority queue example
	fmt.Println("  " + bold("Priority queue:"))
	fmt.Printf("    %s\n", cyan("redis-cli ZADD priority-jobs 1 '{\"id\":\"456\",\"priority\":\"high\"}'"))
	fmt.Printf("    %s\n", cyan("redis-cli ZRANGE priority-jobs 0 -1 WITHSCORES"))
	fmt.Println()

	// Pub/Sub example
	fmt.Println("  " + bold("Pub/Sub messaging:"))
	fmt.Printf("    %s %s\n", cyan("redis-cli SUBSCRIBE events"), "# In terminal 1")
	fmt.Printf("    %s %s\n", cyan("redis-cli PUBLISH events 'Hello World'"), "# In terminal 2")
	fmt.Println()

	// Cache operations
	fmt.Println("  " + bold("Cache operations:"))
	fmt.Printf("    %s\n", cyan("redis-cli SET user:123 '{\"name\":\"John\",\"email\":\"john@example.com\"}'"))
	fmt.Printf("    %s\n", cyan("redis-cli GET user:123"))
	fmt.Printf("    %s\n", cyan("redis-cli EXPIRE user:123 3600"))
	fmt.Println()
}

// PostgresServiceInfo provides PostgreSQL-specific information
type PostgresServiceInfo struct {
	Port     int
	User     string
	Password string
	Database string
}

// PrintInfo displays PostgreSQL service information
func (p *PostgresServiceInfo) PrintInfo() {
	bold := color.New(color.Bold).SprintFunc()
	green := color.New(color.FgGreen).SprintFunc()
	cyan := color.New(color.FgCyan).SprintFunc()

	fmt.Println()
	fmt.Printf("%s %s\n", green("✓"), bold("PostgreSQL"))
	fmt.Printf("  URL: %s\n", cyan(fmt.Sprintf("postgresql://%s:%s@localhost:%d/%s", p.User, p.Password, p.Port, p.Database)))
	fmt.Println()
	fmt.Println("  " + bold("Connect:"))
	fmt.Printf("    %s\n", cyan(fmt.Sprintf("psql -U %s -d %s -h localhost -p %d", p.User, p.Database, p.Port)))
}

// MinIOServiceInfo provides MinIO-specific information
type MinIOServiceInfo struct {
	Port        int
	ConsolePort int
	AccessKey   string
	SecretKey   string
}

// PrintInfo displays MinIO service information
func (m *MinIOServiceInfo) PrintInfo() {
	bold := color.New(color.Bold).SprintFunc()
	green := color.New(color.FgGreen).SprintFunc()
	cyan := color.New(color.FgCyan).SprintFunc()

	fmt.Println()
	fmt.Printf("%s %s\n", green("✓"), bold("MinIO (S3-compatible storage)"))
	fmt.Printf("  API URL: %s\n", cyan(fmt.Sprintf("http://localhost:%d", m.Port)))
	fmt.Printf("  Console: %s\n", cyan(fmt.Sprintf("http://localhost:%d", m.ConsolePort)))
	fmt.Println()
	fmt.Println("  " + bold("Credentials:"))
	fmt.Printf("    Access Key: %s\n", cyan(m.AccessKey))
	fmt.Printf("    Secret Key: %s\n", cyan(m.SecretKey))
}

// OllamaServiceInfo provides Ollama-specific information
type OllamaServiceInfo struct {
	Port   int
	Models []string
}

// PrintInfo displays Ollama service information
func (o *OllamaServiceInfo) PrintInfo() {
	bold := color.New(color.Bold).SprintFunc()
	green := color.New(color.FgGreen).SprintFunc()
	cyan := color.New(color.FgCyan).SprintFunc()

	fmt.Println()
	fmt.Printf("%s %s\n", green("✓"), bold("Ollama (AI Models)"))
	fmt.Printf("  URL: %s\n", cyan(fmt.Sprintf("http://localhost:%d", o.Port)))
	fmt.Println()

	if len(o.Models) > 0 {
		fmt.Println("  " + bold("Available models:"))
		for _, model := range o.Models {
			fmt.Printf("    • %s\n", model)
		}
		fmt.Println()
	}

	fmt.Println("  " + bold("Try:"))
	fmt.Printf("    %s\n", cyan(fmt.Sprintf("curl http://localhost:%d/api/generate -d '{\"model\":\"qwen2.5:3b\",\"prompt\":\"Hello!\"}'", o.Port)))
}

// GetServiceInfoProvider returns the appropriate service info provider
func GetServiceInfoProvider(serviceName string, config interface{}) ServiceInfoProvider {
	switch serviceName {
	case "redis", "cache":
		if cfg, ok := config.(map[string]interface{}); ok {
			port := 6379 // default
			if p, exists := cfg["port"].(int); exists {
				port = p
			}
			return &RedisServiceInfo{Port: port}
		}
	case "postgres", "postgresql", "database":
		if cfg, ok := config.(map[string]interface{}); ok {
			return &PostgresServiceInfo{
				Port:     cfg["port"].(int),
				User:     "localcloud",
				Password: "localcloud-dev",
				Database: "localcloud",
			}
		}
	case "minio", "storage":
		if cfg, ok := config.(map[string]interface{}); ok {
			return &MinIOServiceInfo{
				Port:        cfg["port"].(int),
				ConsolePort: cfg["console_port"].(int),
				AccessKey:   "localcloud",
				SecretKey:   cfg["secret_key"].(string),
			}
		}
	case "ollama", "ai":
		if cfg, ok := config.(map[string]interface{}); ok {
			return &OllamaServiceInfo{
				Port:   cfg["port"].(int),
				Models: cfg["models"].([]string),
			}
		}
	}
	return nil
}

// PrintServiceStartupInfo displays detailed service information after startup
func PrintServiceStartupInfo(serviceName string, port int) {
	config := map[string]interface{}{
		"port": port,
	}

	// Add extra config for specific services
	switch serviceName {
	case "minio", "storage":
		config["console_port"] = port + 1       // MinIO console is typically on port+1
		config["secret_key"] = "localcloud-dev" // This should come from actual config
	case "ollama", "ai":
		config["models"] = []string{"qwen2.5:3b"} // This should come from actual config
	}

	if provider := GetServiceInfoProvider(serviceName, config); provider != nil {
		provider.PrintInfo()
	}
}
