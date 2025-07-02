package templates

import (
	"context"
	"embed"
	"fmt"
	"path/filepath"

	"github.com/localcloud-sh/localcloud/internal/system"
	"gopkg.in/yaml.v3"
)

// BaseTemplate provides common functionality for all templates
type BaseTemplate struct {
	name        string
	metadata    TemplateMetadata
	generator   *Generator
	portManager *PortManager
}

// NewBaseTemplate creates a new base template
func NewBaseTemplate(name string, templatesFS embed.FS) (*BaseTemplate, error) {
	// Load metadata
	metadataPath := filepath.Join("templates", name, "template.yaml")
	content, err := templatesFS.ReadFile(metadataPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read template metadata: %w", err)
	}

	var metadata TemplateMetadata
	if err := yaml.Unmarshal(content, &metadata); err != nil {
		return nil, fmt.Errorf("failed to parse template metadata: %w", err)
	}

	return &BaseTemplate{
		name:        name,
		metadata:    metadata,
		generator:   NewGenerator(templatesFS, "templates"),
		portManager: NewPortManager(),
	}, nil
}

// GetMetadata returns template metadata
func (t *BaseTemplate) GetMetadata() TemplateMetadata {
	return t.metadata
}

// Validate checks if system resources meet requirements
func (t *BaseTemplate) Validate(resources SystemResources) error {
	// Check RAM
	if resources.AvailableRAM < t.metadata.MinRAM {
		return &ValidationError{
			Type:      "RAM",
			Required:  system.FormatBytes(t.metadata.MinRAM),
			Available: system.FormatBytes(resources.AvailableRAM),
		}
	}

	// Check disk space
	if resources.AvailableDisk < t.metadata.MinDisk {
		return &ValidationError{
			Type:      "Disk",
			Required:  system.FormatBytes(t.metadata.MinDisk),
			Available: system.FormatBytes(resources.AvailableDisk),
		}
	}

	// Check Docker
	if !resources.DockerInstalled {
		return &ValidationError{
			Type:      "Docker",
			Required:  "Docker must be installed",
			Available: "Not installed",
		}
	}

	// Check service-specific requirements
	for _, service := range t.metadata.Services {
		switch service {
		case "ollama", "ai":
			if !resources.OllamaInstalled {
				// This is OK, we'll install it
				fmt.Println("Note: Ollama will be installed during setup")
			}
		case "localllama":
			if !resources.LocalLlamaInstalled {
				// This is OK for some templates
				fmt.Println("Note: LocalLlama will be configured during setup")
			}
		}
	}

	return nil
}

// Generate creates the project structure
func (t *BaseTemplate) Generate(projectPath string, options SetupOptions) error {
	// BaseTemplate doesn't do anything here
	// The actual generation is handled by the wizard's generator
	return nil
}

// PostSetup runs post-installation tasks
func (t *BaseTemplate) PostSetup() error {
	// Default implementation does nothing
	// Templates can override this for custom post-setup
	return nil
}

// ValidationError represents a validation failure
type ValidationError struct {
	Type      string
	Required  string
	Available string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("insufficient %s: required %s, available %s", e.Type, e.Required, e.Available)
}

// Service orchestration helpers

// ServiceStarter interface for starting services
type ServiceStarter interface {
	StartService(ctx context.Context, service string, config ServiceConfig) error
	StopService(ctx context.Context, service string) error
	GetServiceStatus(ctx context.Context, service string) (ServiceStatus, error)
}

// ServiceConfig contains service configuration
type ServiceConfig struct {
	Name          string
	Image         string
	Ports         map[string]string
	Environment   map[string]string
	Volumes       []string
	Networks      []string
	HealthCheck   *HealthCheck
	RestartPolicy string
}

// ServiceStatus represents service status
type ServiceStatus struct {
	Running     bool
	Healthy     bool
	Error       string
	ContainerID string
}

// DefaultServiceStarter provides default service management
type DefaultServiceStarter struct {
	// This would integrate with the existing Docker management
}

// StartService starts a Docker service
func (s *DefaultServiceStarter) StartService(ctx context.Context, service string, config ServiceConfig) error {
	// TODO: Integrate with existing Docker manager
	return nil
}

// StopService stops a Docker service
func (s *DefaultServiceStarter) StopService(ctx context.Context, service string) error {
	// TODO: Integrate with existing Docker manager
	return nil
}

// GetServiceStatus gets service status
func (s *DefaultServiceStarter) GetServiceStatus(ctx context.Context, service string) (ServiceStatus, error) {
	// TODO: Integrate with existing Docker manager
	return ServiceStatus{}, nil
}

// Template registration

var (
	// Global template registry
	registry = NewTemplateRegistry()

	// TemplatesFS should be set by main package
	TemplatesFS embed.FS
)

// RegisterTemplate registers a template
func RegisterTemplate(name string, template Template) {
	registry.Register(name, template)
}

// GetTemplate retrieves a template by name
func GetTemplate(name string) (Template, error) {
	template, ok := registry.Get(name)
	if !ok {
		return nil, fmt.Errorf("template %s not found", name)
	}
	return template, nil
}

// ListTemplates returns all available templates
func ListTemplates() []TemplateInfo {
	var templates []TemplateInfo

	for name, template := range registry.List() {
		metadata := template.GetMetadata()
		templates = append(templates, TemplateInfo{
			Name:        name,
			Description: metadata.Description,
			Version:     metadata.Version,
			MinRAM:      system.FormatBytes(metadata.MinRAM),
			MinDisk:     system.FormatBytes(metadata.MinDisk),
			Services:    len(metadata.Services),
		})
	}

	return templates
}

// TemplateInfo contains template information for listing
type TemplateInfo struct {
	Name        string
	Description string
	Version     string
	MinRAM      string
	MinDisk     string
	Services    int
}

// Initialize templates
func InitializeTemplates(fs embed.FS) error {
	TemplatesFS = fs

	// Register built-in templates
	templates := []string{"chat", "code-assistant", "transcribe", "image-gen", "api-only"}

	for _, name := range templates {
		template, err := NewBaseTemplate(name, fs)
		if err != nil {
			// Template might not exist yet, that's OK
			continue
		}
		RegisterTemplate(name, template)
	}

	return nil
}
