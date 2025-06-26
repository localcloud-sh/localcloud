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

// ModelOption represents an AI model option for a component
type ModelOption struct {
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
	Models      []ModelOption          // Available models (for AI components)
	MinRAM      int64                  // Minimum RAM requirement
	Config      map[string]interface{} // Additional configuration
}

// Registry holds all available components
var Registry = map[string]Component{
	"llm": {
		ID:          "llm",
		Name:        "LLM (Text generation)",
		Description: "Large language models for text generation, chat, and completion",
		Category:    "ai",
		Services:    []string{"ai"},
		Models: []ModelOption{
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
		Models: []ModelOption{
			{Name: "nomic-embed-text", Size: "274MB", Dimensions: 768, Default: true},
			{Name: "mxbai-embed-large", Size: "670MB", Dimensions: 1024},
			{Name: "all-minilm", Size: "46MB", Dimensions: 384},
			{Name: "bge-small", Size: "134MB", Dimensions: 384},
		},
		MinRAM: 2 * GB,
	},
	"stt": {
		ID:          "stt",
		Name:        "Speech-to-Text (Whisper)",
		Description: "Convert speech to text using Whisper models",
		Category:    "ai",
		Services:    []string{"whisper"},
		Models: []ModelOption{
			{Name: "whisper-tiny", Size: "39MB", RAM: 1 * GB},
			{Name: "whisper-base", Size: "74MB", RAM: 1 * GB, Default: true},
			{Name: "whisper-small", Size: "244MB", RAM: 2 * GB},
		},
		MinRAM: 1 * GB,
	},
	"vector": {
		ID:          "vector",
		Name:        "Vector Database (pgvector)",
		Description: "PostgreSQL with pgvector for similarity search",
		Category:    "database",
		Services:    []string{"postgres"},
		MinRAM:      1 * GB,
		Config: map[string]interface{}{
			"extensions": []string{"pgvector"},
		},
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
	"rag": {
		Name:        "RAG/Knowledge Base",
		Description: "Build AI-powered search and Q&A systems",
		Components:  []string{"llm", "embedding", "vector", "cache"},
	},
	"chatbot": {
		Name:        "Chatbot Application",
		Description: "Create conversational AI interfaces",
		Components:  []string{"llm", "cache"},
	},
	"voice": {
		Name:        "Voice Assistant",
		Description: "Build voice-enabled AI applications",
		Components:  []string{"llm", "stt", "cache"},
	},
	"api": {
		Name:        "API Service",
		Description: "AI-powered REST API backend",
		Components:  []string{"llm", "cache", "queue"},
	},
	"custom": {
		Name:        "Custom",
		Description: "Select components manually",
		Components:  []string{},
	},
}

// ProjectTemplate represents a project type with preset components
type ProjectTemplate struct {
	Name        string
	Description string
	Components  []string
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
