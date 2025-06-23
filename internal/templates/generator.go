package templates

import (
	"bytes"
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"gopkg.in/yaml.v3"
)

// Generator handles template file generation
type Generator struct {
	templatesFS embed.FS
	rootPath    string // Root path in embed FS (e.g., "templates")
}

// NewGenerator creates a new template generator
func NewGenerator(templatesFS embed.FS, rootPath string) *Generator {
	return &Generator{
		templatesFS: templatesFS,
		rootPath:    rootPath,
	}
}

// TemplateVars contains variables for template substitution
type TemplateVars struct {
	ProjectName      string
	APIPort          int
	FrontendPort     int
	DatabasePort     int
	CachePort        int
	StoragePort      int
	StorageUIPort    int
	AIPort           int
	ModelName        string
	DatabasePassword string
	DatabaseUser     string
	DatabaseName     string
	JWTSecret        string

	// Computed values
	APIBaseURL  string
	FrontendURL string

	// Additional custom variables
	Custom map[string]interface{}
}

// Generate creates project structure from template
func (g *Generator) Generate(templateName, projectPath string, vars TemplateVars) error {
	// Ensure project directory exists
	if err := os.MkdirAll(projectPath, 0755); err != nil {
		return fmt.Errorf("failed to create project directory: %w", err)
	}

	// Template source path
	templatePath := filepath.Join(g.rootPath, templateName)

	// Walk through template files
	err := fs.WalkDir(g.templatesFS, templatePath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Skip template metadata file
		if d.Name() == "template.yaml" {
			return nil
		}

		// Calculate relative path from template root
		relPath, err := filepath.Rel(templatePath, path)
		if err != nil {
			return err
		}

		// Skip the root directory itself
		if relPath == "." {
			return nil
		}

		// Destination path
		destPath := filepath.Join(projectPath, relPath)

		if d.IsDir() {
			// Create directory
			return os.MkdirAll(destPath, 0755)
		}

		// Process and copy file
		return g.processFile(path, destPath, vars)
	})

	if err != nil {
		return fmt.Errorf("failed to generate template: %w", err)
	}

	// Generate additional files
	if err := g.generateDockerCompose(projectPath, templateName, vars); err != nil {
		return fmt.Errorf("failed to generate docker-compose.yml: %w", err)
	}

	if err := g.generateEnvFile(projectPath, vars); err != nil {
		return fmt.Errorf("failed to generate .env: %w", err)
	}

	return nil
}

// processFile processes a single template file
func (g *Generator) processFile(srcPath, destPath string, vars TemplateVars) error {
	// Read source file
	content, err := g.templatesFS.ReadFile(srcPath)
	if err != nil {
		return fmt.Errorf("failed to read template file: %w", err)
	}

	// Check if file should be processed as template
	if shouldProcessTemplate(srcPath) {
		// Process template
		processed, err := g.processTemplate(string(content), vars)
		if err != nil {
			return fmt.Errorf("failed to process template %s: %w", srcPath, err)
		}
		content = []byte(processed)
	}

	// Write to destination
	if err := os.WriteFile(destPath, content, 0644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	// Make scripts executable
	if strings.HasSuffix(destPath, ".sh") {
		if err := os.Chmod(destPath, 0755); err != nil {
			return fmt.Errorf("failed to make script executable: %w", err)
		}
	}

	return nil
}

// shouldProcessTemplate determines if a file should be processed as template
func shouldProcessTemplate(path string) bool {
	// Always process these files as templates
	alwaysProcess := []string{
		"App.jsx",      // Main app component needs ProjectName
		"index.html",   // HTML files
		".env",         // Environment files
		"package.json", // Package files
		"server.js",    // Server files
	}

	// Check if file should always be processed
	for _, file := range alwaysProcess {
		if strings.HasSuffix(path, file) {
			return true
		}
	}

	// Don't process these file types
	noProcess := []string{
		"MessageList.jsx",   // Has complex JSX syntax
		"ChatInterface.jsx", // Has complex JSX syntax
		"ModelSelector.jsx", // Has complex JSX syntax
	}

	// Check if file should not be processed
	for _, file := range noProcess {
		if strings.HasSuffix(path, file) {
			return false
		}
	}

	// Process these file types by default
	extensions := []string{
		".js", ".ts",
		".json", ".yaml", ".yml",
		".env", ".env.example",
		".md", ".txt",
		".html", ".css",
		".sh", ".bash",
		".sql",
		".config.js",
	}

	for _, ext := range extensions {
		if strings.HasSuffix(path, ext) {
			return true
		}
	}

	// Don't process binary files
	binaryExts := []string{
		".png", ".jpg", ".jpeg", ".gif", ".ico",
		".woff", ".woff2", ".ttf", ".eot",
		".zip", ".tar", ".gz",
	}

	for _, ext := range binaryExts {
		if strings.HasSuffix(path, ext) {
			return false
		}
	}

	// Default to not processing
	return false
}

// processTemplate processes template content with variables
func (g *Generator) processTemplate(content string, vars TemplateVars) (string, error) {
	// Create template
	tmpl, err := template.New("file").Parse(content)
	if err != nil {
		return "", err
	}

	// Execute template
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, vars); err != nil {
		return "", err
	}

	return buf.String(), nil
}

// generateDockerCompose generates docker-compose.yml
func (g *Generator) generateDockerCompose(projectPath, templateName string, vars TemplateVars) error {
	// Load template metadata to get services
	metadata, err := g.loadTemplateMetadata(templateName)
	if err != nil {
		return err
	}

	// Docker compose content
	compose := DockerCompose{
		Version:  "3.8",
		Services: make(map[string]DockerService),
		Networks: map[string]DockerNetwork{
			"localcloud": {
				Driver: "bridge",
			},
		},
		Volumes: make(map[string]DockerVolume),
	}

	// Add services based on template requirements
	for _, service := range metadata.Services {
		switch service {
		case "ollama", "ai":
			compose.Services["ollama"] = DockerService{
				Image:         "ollama/ollama:latest",
				ContainerName: fmt.Sprintf("%s-ollama", filepath.Base(vars.ProjectName)),
				Ports:         []string{fmt.Sprintf("%d:11434", vars.AIPort)},
				Volumes: []string{
					"ollama_data:/root/.ollama",
				},
				Networks: []string{"localcloud"},
				Restart:  "unless-stopped",
			}
			compose.Volumes["ollama_data"] = DockerVolume{}

		case "postgres", "database":
			compose.Services["postgres"] = DockerService{
				Image:         "postgres:16-alpine",
				ContainerName: fmt.Sprintf("%s-postgres", filepath.Base(vars.ProjectName)),
				Ports:         []string{fmt.Sprintf("%d:5432", vars.DatabasePort)},
				Environment: map[string]string{
					"POSTGRES_USER":     vars.DatabaseUser,
					"POSTGRES_PASSWORD": vars.DatabasePassword,
					"POSTGRES_DB":       vars.DatabaseName,
				},
				Volumes: []string{
					"postgres_data:/var/lib/postgresql/data",
					"./migrations:/docker-entrypoint-initdb.d",
				},
				Networks: []string{"localcloud"},
				Restart:  "unless-stopped",
				HealthCheck: &HealthCheck{
					Test:     []string{"CMD-SHELL", "pg_isready -U " + vars.DatabaseUser},
					Interval: "5s",
					Timeout:  "5s",
					Retries:  5,
				},
			}
			compose.Volumes["postgres_data"] = DockerVolume{}

		case "redis", "cache":
			compose.Services["redis"] = DockerService{
				Image:         "redis:7-alpine",
				ContainerName: fmt.Sprintf("%s-redis", filepath.Base(vars.ProjectName)),
				Ports:         []string{fmt.Sprintf("%d:6379", vars.CachePort)},
				Volumes: []string{
					"redis_data:/data",
				},
				Networks: []string{"localcloud"},
				Restart:  "unless-stopped",
				Command:  []string{"redis-server", "--appendonly", "yes"},
			}
			compose.Volumes["redis_data"] = DockerVolume{}

		case "minio", "storage":
			compose.Services["minio"] = DockerService{
				Image:         "minio/minio:latest",
				ContainerName: fmt.Sprintf("%s-minio", filepath.Base(vars.ProjectName)),
				Ports: []string{
					fmt.Sprintf("%d:9000", vars.StoragePort),
					fmt.Sprintf("%d:9001", vars.StorageUIPort),
				},
				Environment: map[string]string{
					"MINIO_ROOT_USER":     "minioadmin",
					"MINIO_ROOT_PASSWORD": "minioadmin",
				},
				Volumes: []string{
					"minio_data:/data",
				},
				Networks: []string{"localcloud"},
				Restart:  "unless-stopped",
				Command:  []string{"server", "/data", "--console-address", ":9001"},
			}
			compose.Volumes["minio_data"] = DockerVolume{}
		}
	}

	// Generate docker-compose.yml
	composeYAML, err := yaml.Marshal(compose)
	if err != nil {
		return err
	}

	composePath := filepath.Join(projectPath, "docker-compose.yml")
	return os.WriteFile(composePath, composeYAML, 0644)
}

// generateEnvFile generates .env file with defaults
func (g *Generator) generateEnvFile(projectPath string, vars TemplateVars) error {
	envContent := fmt.Sprintf(`# LocalCloud Environment Configuration
PROJECT_NAME=%s

# API Configuration
API_PORT=%d
API_BASE_URL=%s

# Frontend Configuration
FRONTEND_PORT=%d
FRONTEND_URL=%s

# Database Configuration
DATABASE_HOST=localhost
DATABASE_PORT=%d
DATABASE_USER=%s
DATABASE_PASSWORD=%s
DATABASE_NAME=%s

# AI Model Configuration
AI_MODEL=%s
AI_PORT=%d
OLLAMA_HOST=http://localhost:%d

# Security
JWT_SECRET=%s

# Environment
NODE_ENV=development
`,
		vars.ProjectName,
		vars.APIPort,
		vars.APIBaseURL,
		vars.FrontendPort,
		vars.FrontendURL,
		vars.DatabasePort,
		vars.DatabaseUser,
		vars.DatabasePassword,
		vars.DatabaseName,
		vars.ModelName,
		vars.AIPort,
		vars.AIPort,
		vars.JWTSecret,
	)

	// Add service-specific env vars
	if vars.CachePort > 0 {
		envContent += fmt.Sprintf("\n# Cache Configuration\nREDIS_HOST=localhost\nREDIS_PORT=%d\n", vars.CachePort)
	}

	if vars.StoragePort > 0 {
		envContent += fmt.Sprintf("\n# Storage Configuration\nMINIO_ENDPOINT=localhost:%d\nMINIO_ACCESS_KEY=minioadmin\nMINIO_SECRET_KEY=minioadmin\n", vars.StoragePort)
	}

	envPath := filepath.Join(projectPath, ".env")
	return os.WriteFile(envPath, []byte(envContent), 0644)
}

// loadTemplateMetadata loads template.yaml for a template
func (g *Generator) loadTemplateMetadata(templateName string) (*TemplateMetadata, error) {
	metadataPath := filepath.Join(g.rootPath, templateName, "template.yaml")
	content, err := g.templatesFS.ReadFile(metadataPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read template metadata: %w", err)
	}

	var metadata TemplateMetadata
	if err := yaml.Unmarshal(content, &metadata); err != nil {
		return nil, fmt.Errorf("failed to parse template metadata: %w", err)
	}

	return &metadata, nil
}

// DockerCompose represents docker-compose.yml structure
type DockerCompose struct {
	Version  string                   `yaml:"version"`
	Services map[string]DockerService `yaml:"services"`
	Networks map[string]DockerNetwork `yaml:"networks,omitempty"`
	Volumes  map[string]DockerVolume  `yaml:"volumes,omitempty"`
}

// DockerService represents a service in docker-compose
type DockerService struct {
	Image         string            `yaml:"image"`
	ContainerName string            `yaml:"container_name,omitempty"`
	Ports         []string          `yaml:"ports,omitempty"`
	Environment   map[string]string `yaml:"environment,omitempty"`
	Volumes       []string          `yaml:"volumes,omitempty"`
	Networks      []string          `yaml:"networks,omitempty"`
	DependsOn     []string          `yaml:"depends_on,omitempty"`
	Restart       string            `yaml:"restart,omitempty"`
	Command       []string          `yaml:"command,omitempty"`
	HealthCheck   *HealthCheck      `yaml:"healthcheck,omitempty"`
}

// HealthCheck represents docker health check
type HealthCheck struct {
	Test     []string `yaml:"test"`
	Interval string   `yaml:"interval"`
	Timeout  string   `yaml:"timeout"`
	Retries  int      `yaml:"retries"`
}

// DockerNetwork represents a network in docker-compose
type DockerNetwork struct {
	Driver string `yaml:"driver,omitempty"`
}

// DockerVolume represents a volume in docker-compose
type DockerVolume struct {
	Driver     string            `yaml:"driver,omitempty"`
	DriverOpts map[string]string `yaml:"driver_opts,omitempty"`
}
