// Package models provides AI model management functionality
package models

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// EmbeddingModel represents an embedding model
type EmbeddingModel struct {
	Name       string
	Size       string
	Dimensions int
	Family     string
}

// PredefinedEmbeddingModels contains known embedding models
var PredefinedEmbeddingModels = []EmbeddingModel{
	{
		Name:       "nomic-embed-text",
		Size:       "274MB",
		Dimensions: 768,
		Family:     "bert",
	},
	{
		Name:       "mxbai-embed-large",
		Size:       "670MB",
		Dimensions: 1024,
		Family:     "bert",
	},
	{
		Name:       "all-minilm",
		Size:       "46MB",
		Dimensions: 384,
		Family:     "bert",
	},
	{
		Name:       "bge-base",
		Size:       "420MB",
		Dimensions: 768,
		Family:     "bert",
	},
	{
		Name:       "bge-large",
		Size:       "1.3GB",
		Dimensions: 1024,
		Family:     "bert",
	},
	{
		Name:       "e5-base",
		Size:       "438MB",
		Dimensions: 768,
		Family:     "bert",
	},
	{
		Name:       "e5-large",
		Size:       "1.3GB",
		Dimensions: 1024,
		Family:     "bert",
	},
}

// IsEmbeddingModel checks if a model is an embedding model
func IsEmbeddingModel(modelName string) bool {
	// 1. Check predefined list
	for _, em := range PredefinedEmbeddingModels {
		if em.Name == modelName {
			return true
		}
	}

	// 2. Check name patterns
	lowerName := strings.ToLower(modelName)
	if strings.Contains(lowerName, "embed") {
		return true
	}

	// 3. Check known embedding model prefixes
	embedPatterns := []string{
		"bge-", "gte-", "e5-", "instructor-",
		"sentence-transformers", "all-minilm",
		"text-embedding-", "stella-",
	}

	for _, pattern := range embedPatterns {
		if strings.HasPrefix(lowerName, pattern) {
			return true
		}
	}

	return false
}

// GetEmbeddingModelInfo returns information about an embedding model
func GetEmbeddingModelInfo(modelName string) *EmbeddingModel {
	// Check predefined models
	for _, em := range PredefinedEmbeddingModels {
		if em.Name == modelName {
			return &em
		}
	}

	// Return nil for unknown models
	return nil
}

// GetAvailableEmbeddingModels returns all installed embedding models
func (m *Manager) GetAvailableEmbeddingModels() ([]Model, error) {
	allModels, err := m.List()
	if err != nil {
		return nil, err
	}

	var embeddingModels []Model
	for _, model := range allModels {
		if IsEmbeddingModel(model.Name) {
			embeddingModels = append(embeddingModels, model)
		}
	}

	return embeddingModels, nil
}

// GetModelDetails fetches detailed information about a model from Ollama
func (m *Manager) GetModelDetails(modelName string) (map[string]interface{}, error) {
	payload := map[string]string{
		"name": modelName,
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	resp, err := m.httpClient.Post(
		m.ollamaEndpoint+"/api/show",
		"application/json",
		strings.NewReader(string(jsonData)),
	)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get model details: status %d", resp.StatusCode)
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return result, nil
}

// CheckModelType determines the type of a model (llm, embedding, etc)
func (m *Manager) CheckModelType(modelName string) string {
	// First check if it's a known embedding model
	if IsEmbeddingModel(modelName) {
		return "embedding"
	}

	// Try to get model details from Ollama
	details, err := m.GetModelDetails(modelName)
	if err == nil {
		// Check model family in details
		if detailsMap, ok := details["details"].(map[string]interface{}); ok {
			if families, ok := detailsMap["families"].([]interface{}); ok {
				for _, family := range families {
					if familyStr, ok := family.(string); ok {
						if familyStr == "bert" || familyStr == "sentence-transformers" {
							return "embedding"
						}
					}
				}
			}
		}

		// Check template for embedding indicators
		if template, ok := details["template"].(string); ok {
			if strings.Contains(strings.ToLower(template), "embedding") {
				return "embedding"
			}
		}
	}

	// Default to LLM
	return "llm"
}

// ListEmbeddingModels returns both installed and available embedding models
func (m *Manager) ListEmbeddingModels() (installed []Model, available []EmbeddingModel, err error) {
	// Get installed embedding models
	installed, err = m.GetAvailableEmbeddingModels()
	if err != nil {
		return nil, nil, err
	}

	// Build list of available models not yet installed
	installedMap := make(map[string]bool)
	for _, model := range installed {
		installedMap[model.Name] = true
	}

	for _, predefined := range PredefinedEmbeddingModels {
		if !installedMap[predefined.Name] {
			available = append(available, predefined)
		}
	}

	return installed, available, nil
}
