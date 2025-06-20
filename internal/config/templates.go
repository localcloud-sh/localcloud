package config

import "fmt"

// TemplateConfig generates template-specific configurations
func TemplateConfig(projectName, template string) string {
	switch template {
	case "chat":
		return chatTemplate(projectName)
	case "api":
		return apiTemplate(projectName)
	case "rag":
		return ragTemplate(projectName)
	default:
		return generateDefaultTemplate(projectName, "general")
	}
}
func generateDefaultTemplate(projectName, projectType string) string {
	return fmt.Sprintf(`version: "1"
project:
  name: "%s"
  type: "%s"
`, projectName, projectType)
}
func chatTemplate(projectName string) string {
	return fmt.Sprintf(`version: "1"
project:
  name: "%s"
  type: "chat"

services:
  ai:
    models:
      - qwen2.5:3b
      - deepseek-coder:1.3b
    default: qwen2.5:3b
    port: 11434
  
  database:
    type: postgres
    version: "16"
    port: 5432
    extensions:
      - pgvector
      - uuid-ossp
  
  cache:
    type: redis
    port: 6379
    maxmemory: "512mb"

resources:
  memory_limit: "4GB"
  cpu_limit: "2"
`, projectName)
}

func apiTemplate(projectName string) string {
	return fmt.Sprintf(`version: "1"
project:
  name: "%s"
  type: "api"

services:
  ai:
    models:
      - qwen2.5:3b
    default: qwen2.5:3b
    port: 11434
  
  database:
    type: postgres
    version: "16"
    port: 5432
    extensions:
      - uuid-ossp

resources:
  memory_limit: "2GB"
  cpu_limit: "2"
`, projectName)
}

func ragTemplate(projectName string) string {
	return fmt.Sprintf(`version: "1"
project:
  name: "%s"
  type: "rag"

services:
  ai:
    models:
      - qwen2.5:3b
      - llama3.2:3b
    default: qwen2.5:3b
    port: 11434
  
  database:
    type: postgres
    version: "16"
    port: 5432
    extensions:
      - pgvector
      - pg_trgm
  
  storage:
    type: minio
    port: 9000
    console: 9001

resources:
  memory_limit: "6GB"
  cpu_limit: "2"
`, projectName)
}
