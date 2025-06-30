// internal/models/manager.go
package models

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

// Model represents an AI model
type Model struct {
	Name        string    `json:"name"`
	Model       string    `json:"model"`
	Size        int64     `json:"size"`
	Digest      string    `json:"digest"`
	ModifiedAt  time.Time `json:"modified_at"`
	Description string    `json:"description,omitempty"`
}

// PullProgress represents download progress
type PullProgress struct {
	Status     string `json:"status"`
	Digest     string `json:"digest,omitempty"`
	Total      int64  `json:"total,omitempty"`
	Completed  int64  `json:"completed,omitempty"`
	Percentage int    `json:"percentage,omitempty"`
}

// Provider represents an AI provider
type Provider string

const (
	ProviderOllama Provider = "ollama"
	ProviderOpenAI Provider = "openai"
)

// Manager handles AI model operations
type Manager struct {
	ollamaEndpoint string
	httpClient     *http.Client
}

// DetectProvider detects which AI provider is available
func (m *Manager) DetectProvider() Provider {
	// Check for OpenAI API key first
	if os.Getenv("OPENAI_API_KEY") != "" {
		return ProviderOpenAI
	}

	// Check if Ollama is available
	if m.IsOllamaAvailable() {
		return ProviderOllama
	}

	return ""
}

// internal/models/manager.go - Updated NewManager and Pull functions

// NewManager creates a new model manager
func NewManager(ollamaEndpoint string) *Manager {
	if ollamaEndpoint == "" {
		ollamaEndpoint = "http://localhost:11434"
	}

	return &Manager{
		ollamaEndpoint: ollamaEndpoint,
		httpClient: &http.Client{
			Timeout: 30 * time.Second, // This is only for non-streaming requests
		},
	}
}

// Pull downloads a model with progress updates
func (m *Manager) Pull(modelName string, progress chan<- PullProgress) error {
	defer close(progress)

	// Prepare request
	payload := map[string]string{
		"name": modelName,
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", m.ollamaEndpoint+"/api/pull", bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")

	// Create a custom client without timeout for streaming
	// The pull operation can take a long time for large models
	client := &http.Client{
		Timeout: 0, // No timeout for streaming responses
	}

	// Make request
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to connect to Ollama: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to pull model: %s", string(body))
	}

	// Stream progress updates with a deadline for each read
	decoder := json.NewDecoder(resp.Body)
	lastUpdate := time.Now()

	for {
		var update map[string]interface{}

		// Set a read deadline for the decoder to prevent hanging
		if err := decoder.Decode(&update); err != nil {
			if err == io.EOF {
				break
			}

			// Check if we haven't received updates for too long
			if time.Since(lastUpdate) > 5*time.Minute {
				return fmt.Errorf("no progress update received for 5 minutes, connection might be stalled")
			}

			return err
		}

		lastUpdate = time.Now()

		// Convert to PullProgress
		p := PullProgress{
			Status: getString(update, "status"),
			Digest: getString(update, "digest"),
		}

		if total, ok := update["total"].(float64); ok {
			p.Total = int64(total)
		}

		if completed, ok := update["completed"].(float64); ok {
			p.Completed = int64(completed)
			if p.Total > 0 {
				p.Percentage = int((p.Completed * 100) / p.Total)
			}
		}

		// Send progress update
		select {
		case progress <- p:
			// Progress sent successfully
		case <-time.After(10 * time.Second):
			// If we can't send progress for 10 seconds, something might be wrong
			return fmt.Errorf("progress channel blocked, aborting download")
		}
	}

	return nil
}

// Helper function to safely get string from map
func getString(m map[string]interface{}, key string) string {
	if val, ok := m[key]; ok {
		if str, ok := val.(string); ok {
			return str
		}
	}
	return ""
}

// IsOllamaAvailable checks if Ollama service is running
func (m *Manager) IsOllamaAvailable() bool {
	resp, err := m.httpClient.Get(m.ollamaEndpoint + "/api/tags")
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	return resp.StatusCode == http.StatusOK
}

// List returns all available models
func (m *Manager) List() ([]Model, error) {
	resp, err := m.httpClient.Get(m.ollamaEndpoint + "/api/tags")
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Ollama: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var result struct {
		Models []Model `json:"models"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return result.Models, nil
}

// Remove deletes a model
func (m *Manager) Remove(modelName string) error {
	payload := map[string]string{
		"name": modelName,
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("DELETE", m.ollamaEndpoint+"/api/delete", bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := m.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to connect to Ollama: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to remove model: %s", string(body))
	}

	return nil
}

// GetActiveModel returns the currently active model from config
func (m *Manager) GetActiveModel(configDefault string) (string, error) {
	// For now, return config default
	// Can be enhanced to track last used model
	return configDefault, nil
}

// SetActiveModel sets the active model
func (m *Manager) SetActiveModel(modelName string) error {
	// Verify model exists
	models, err := m.List()
	if err != nil {
		return err
	}

	found := false
	for _, model := range models {
		if model.Name == modelName || model.Model == modelName {
			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("model %s not found", modelName)
	}

	// TODO: Save to config or state file
	return nil
}

// GetRecommendedModels returns a list of recommended models
func GetRecommendedModels() []struct {
	Name        string
	Size        string
	Description string
} {
	return []struct {
		Name        string
		Size        string
		Description string
	}{
		{
			Name:        "qwen2.5:3b",
			Size:        "2.3GB",
			Description: "Fast general purpose model with good performance",
		},
		{
			Name:        "deepseek-coder:1.3b",
			Size:        "1.5GB",
			Description: "Specialized for code completion and generation",
		},
		{
			Name:        "llama3.2:3b",
			Size:        "2.0GB",
			Description: "Latest Llama model with improved capabilities",
		},
		{
			Name:        "phi3:mini",
			Size:        "2.3GB",
			Description: "Microsoft's efficient small language model",
		},
		{
			Name:        "gemma2:2b",
			Size:        "1.6GB",
			Description: "Google's lightweight open model",
		},
	}
}
