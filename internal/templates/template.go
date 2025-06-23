package templates

import (
	"context"
)

// Template defines the interface that all templates must implement
type Template interface {
	// GetMetadata returns template information
	GetMetadata() TemplateMetadata

	// Validate checks if system resources meet template requirements
	Validate(resources SystemResources) error

	// Generate creates the project structure and files
	Generate(projectPath string, options SetupOptions) error

	// PostSetup runs any post-installation scripts or tasks
	PostSetup() error
}

// TemplateMetadata contains information about a template
type TemplateMetadata struct {
	Name        string             `yaml:"name"`
	Description string             `yaml:"description"`
	Version     string             `yaml:"version"`
	MinRAM      int64              `yaml:"minRAM"`   // Minimum RAM in bytes
	MinDisk     int64              `yaml:"minDisk"`  // Minimum disk space in bytes
	Services    []string           `yaml:"services"` // Required services (ollama, postgres, etc.)
	Models      []ModelRequirement `yaml:"models"`
}

// ModelRequirement specifies AI model requirements
type ModelRequirement struct {
	Name        string `yaml:"name"`
	Size        int64  `yaml:"size"`        // Size in bytes
	MinRAM      int64  `yaml:"minRAM"`      // Minimum RAM for this model
	Recommended bool   `yaml:"recommended"` // Is this the recommended model
	Default     bool   `yaml:"default"`     // Is this the default model
}

// SetupOptions contains user-provided setup configuration
type SetupOptions struct {
	ProjectName      string
	APIPort          int
	FrontendPort     int
	DatabasePort     int
	ModelName        string
	DatabasePassword string
	SkipDocker       bool // Generate files only, don't start services
	Force            bool // Overwrite existing directory
}

// SystemResources contains information about the host system
type SystemResources struct {
	TotalRAM            int64
	AvailableRAM        int64
	TotalDisk           int64
	AvailableDisk       int64
	CPUCount            int
	DockerInstalled     bool
	DockerVersion       string
	OllamaInstalled     bool
	LocalLlamaInstalled bool
	Platform            string // darwin, linux, windows
	Architecture        string // amd64, arm64
}

// TemplateRegistry manages available templates
type TemplateRegistry struct {
	templates map[string]Template
}

// NewTemplateRegistry creates a new template registry
func NewTemplateRegistry() *TemplateRegistry {
	return &TemplateRegistry{
		templates: make(map[string]Template),
	}
}

// Register adds a template to the registry
func (r *TemplateRegistry) Register(name string, template Template) {
	r.templates[name] = template
}

// Get retrieves a template by name
func (r *TemplateRegistry) Get(name string) (Template, bool) {
	t, ok := r.templates[name]
	return t, ok
}

// List returns all registered templates
func (r *TemplateRegistry) List() map[string]Template {
	return r.templates
}

// TemplateError represents a template-related error
type TemplateError struct {
	Template string
	Stage    string // validate, generate, postsetup
	Err      error
}

func (e *TemplateError) Error() string {
	return "template " + e.Template + " failed at " + e.Stage + ": " + e.Err.Error()
}

// SetupContext contains context for template setup
type SetupContext struct {
	Context         context.Context
	ProjectPath     string
	Options         SetupOptions
	Resources       SystemResources
	Logger          Logger
	ProgressHandler ProgressHandler
}

// Logger interface for template logging
type Logger interface {
	Debug(msg string, args ...interface{})
	Info(msg string, args ...interface{})
	Warn(msg string, args ...interface{})
	Error(msg string, args ...interface{})
}

// ProgressHandler interface for setup progress updates
type ProgressHandler interface {
	Start(total int)
	Update(current int, message string)
	Complete()
	Error(err error)
}
