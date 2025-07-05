// internal/components/registry.go
// Package components provides component registry and management for LocalCloud
package components

import (
	"fmt"
)

const (
	GB = 1024 * 1024 * 1024
	MB = 1024 * 1024
)

// Model represents an AI model option for a component
// This is the type name used throughout the codebase
type Model struct {
	Name       string
	Size       string
	RAM        int64
	Default    bool
	Dimensions int    // For embedding models
	Family     string // Model family (e.g., "bert", "llama")
}

// Component represents a LocalCloud component
type Component struct {
	ID          string
	Name        string
	Description string
	Category    string                 // "ai", "database", "infrastructure"
	Services    []string               // Required docker services
	Models      []Model                // Available models (for AI components)
	MinRAM      int64                  // Minimum RAM requirement
	Config      map[string]interface{} // Additional configuration
}

// ProjectTemplate represents a project type with preset components
type ProjectTemplate struct {
	Name        string
	Description string
	Components  []string
}

// Registry holds all available components
var Registry = map[string]Component{
	"llm": {
		ID:          "llm",
		Name:        "LLM (Text generation)",
		Description: "Large language models for text generation, chat, and completion",
		Category:    "ai",
		Services:    []string{"ai"},
		Models: []Model{
			{Name: "qwen2.5:3b", Size: "2.3GB", RAM: 4 * GB, Default: true},
			{Name: "llama3.2:3b", Size: "2.0GB", RAM: 4 * GB},
			{Name: "deepseek-coder:1.3b", Size: "1.5GB", RAM: 3 * GB},
			{Name: "phi3:mini", Size: "2.3GB", RAM: 4 * GB},
			{Name: "gemma2:2b", Size: "1.6GB", RAM: 3 * GB},
		},
		MinRAM: 4 * GB,
	},
	"embedding": {
		ID:          "embedding",
		Name:        "Embeddings (Semantic search)",
		Description: "Text embeddings for semantic search and similarity",
		Category:    "ai",
		Services:    []string{"ai"},
		Models: []Model{
			{Name: "nomic-embed-text", Size: "274MB", RAM: 768 * MB, Dimensions: 768, Default: true},
			{Name: "mxbai-embed-large", Size: "670MB", RAM: 1 * GB, Dimensions: 1024},
			{Name: "all-minilm", Size: "46MB", RAM: 256 * MB, Dimensions: 384},
			{Name: "bge-small", Size: "134MB", RAM: 512 * MB, Dimensions: 384},
		},
		MinRAM: 2 * GB,
	},
	"database": {
		ID:          "database",
		Name:        "Database (PostgreSQL)",
		Description: "Standard relational database for data storage",
		Category:    "database",
		Services:    []string{"postgres"},
		MinRAM:      512 * MB,
	},
	"vector": {
		ID:          "vector",
		Name:        "Vector Search (pgvector)",
		Description: "Add vector similarity search to PostgreSQL",
		Category:    "database",
		Services:    []string{"postgres"},
		MinRAM:      512 * MB,
		Config: map[string]interface{}{
			"extension":  "pgvector",
			"depends_on": "database",
		},
	},
	"mongodb": {
		ID:          "mongodb",
		Name:        "NoSQL Database (MongoDB)",
		Description: "Document-oriented database for flexible data storage",
		Category:    "database",
		Services:    []string{"mongodb"},
		MinRAM:      1 * GB,
	},
	"cache": {
		ID:          "cache",
		Name:        "Cache (Redis)",
		Description: "In-memory cache for temporary data and sessions",
		Category:    "infrastructure",
		Services:    []string{"cache"},
		MinRAM:      512 * MB,
	},
	"queue": {
		ID:          "queue",
		Name:        "Queue (Redis)",
		Description: "Reliable job queue for background processing",
		Category:    "infrastructure",
		Services:    []string{"queue"},
		MinRAM:      512 * MB,
	},
	"storage": {
		ID:          "storage",
		Name:        "Object Storage (MinIO)",
		Description: "S3-compatible object storage for files and media",
		Category:    "infrastructure",
		Services:    []string{"minio"},
		MinRAM:      1 * GB,
	},
}

// ProjectTemplates defines component sets for project types
var ProjectTemplates = map[string]ProjectTemplate{
	"custom": {
		Name:        "Custom",
		Description: "Select components manually",
		Components:  []string{},
	},
	"rag": {
		Name:        "RAG Application",
		Description: "Retrieval-augmented generation with vector search",
		Components:  []string{"llm", "embedding", "database", "vector", "cache"},
	},
	"chatbot": {
		Name:        "Chatbot Application",
		Description: "Create conversational AI interfaces",
		Components:  []string{"llm", "database", "cache"},
	},
	"fullstack": {
		Name:        "Full Stack App",
		Description: "Complete application with all necessary components",
		Components:  []string{"llm", "database", "cache", "queue", "storage"},
	},
	"simple": {
		Name:        "Simple LLM",
		Description: "Just language model, no additional services",
		Components:  []string{"llm"},
	},
}

// GetComponent returns a component by ID
func GetComponent(id string) (Component, error) {
	if comp, ok := Registry[id]; ok {
		return comp, nil
	}
	return Component{}, fmt.Errorf("component %s not found", id)
}

// GetTemplate returns a project template by name
func GetTemplate(name string) (ProjectTemplate, error) {
	if tmpl, ok := ProjectTemplates[name]; ok {
		return tmpl, nil
	}
	return ProjectTemplate{}, fmt.Errorf("template %s not found", name)
}

// GetComponentsByCategory returns all components in a category
func GetComponentsByCategory(category string) []Component {
	var components []Component
	for _, comp := range Registry {
		if comp.Category == category {
			components = append(components, comp)
		}
	}
	return components
}

// GetAllComponents returns all available components
func GetAllComponents() []Component {
	var components []Component
	// Use a specific order
	order := []string{"llm", "embedding", "database", "vector", "mongodb", "cache", "queue", "storage", "stt"}

	for _, id := range order {
		if comp, ok := Registry[id]; ok {
			components = append(components, comp)
		}
	}

	// Add any components not in the order list
	for id, comp := range Registry {
		found := false
		for _, orderedID := range order {
			if id == orderedID {
				found = true
				break
			}
		}
		if !found {
			components = append(components, comp)
		}
	}

	return components
}

// CalculateRAMRequirement calculates total RAM needed for components
func CalculateRAMRequirement(componentIDs []string) int64 {
	var totalRAM int64
	seenServices := make(map[string]bool)

	for _, id := range componentIDs {
		if comp, ok := Registry[id]; ok {
			// Add component's minimum RAM
			totalRAM += comp.MinRAM

			// Track services to avoid double counting
			for _, service := range comp.Services {
				seenServices[service] = true
			}
		}
	}

	// Add base system overhead
	totalRAM += 1 * GB

	return totalRAM
}

// ComponentsToServices converts component IDs to required services
func ComponentsToServices(componentIDs []string) []string {
	serviceMap := make(map[string]bool)

	for _, id := range componentIDs {
		if comp, ok := Registry[id]; ok {
			for _, service := range comp.Services {
				serviceMap[service] = true
			}
		}
	}

	// Convert map to slice
	var services []string
	for service := range serviceMap {
		services = append(services, service)
	}

	return services
}

// IsAIComponent returns true if the component requires AI models
func IsAIComponent(id string) bool {
	comp, err := GetComponent(id)
	if err != nil {
		return false
	}
	return comp.Category == "ai" && len(comp.Models) > 0
}

// ValidateComponentDependencies checks if component dependencies are satisfied
func ValidateComponentDependencies(componentIDs []string) error {
	components := make(map[string]bool)
	for _, id := range componentIDs {
		components[id] = true
	}

	// Check vector requires database
	if components["vector"] && !components["database"] {
		return fmt.Errorf("vector search requires database component")
	}

	return nil
}

// GetComponentDependencies returns required components for a given component
func GetComponentDependencies(componentID string) []string {
	switch componentID {
	case "vector":
		return []string{"database"}
	default:
		return []string{}
	}
}

// GetDependentComponents returns components that depend on the given component
func GetDependentComponents(componentID string, enabledComponents []string) []string {
	var dependents []string

	enabled := make(map[string]bool)
	for _, id := range enabledComponents {
		enabled[id] = true
	}

	switch componentID {
	case "database":
		// Vector depends on database
		if enabled["vector"] {
			dependents = append(dependents, "vector")
		}
	}

	return dependents
}
