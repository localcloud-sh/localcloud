# Contributing to LocalCloud

Welcome! We're excited to have you contribute to LocalCloud. This guide will help you get started with development and understand our contribution process.

## Table of Contents
- [Development Setup](#development-setup)
- [Architecture Overview](#architecture-overview)
- [Making Changes](#making-changes)
- [Adding Templates](#adding-templates)
- [Testing](#testing)
- [Pull Request Process](#pull-request-process)
- [Community Guidelines](#community-guidelines)

## Development Setup

### Prerequisites
- Go 1.21 or later
- Docker Desktop
- Node.js 18+ (for frontend templates)
- Make

### Getting Started

1. **Fork and Clone**
   ```bash
   git clone https://github.com/your-username/localcloud.git
   cd localcloud
   ```

2. **Install Dependencies**
   ```bash
   go mod download
   ```

3. **Build LocalCloud**
   ```bash
   make build
   ```

4. **Run Tests**
   ```bash
   make test
   ```

5. **Install Local Binary**
   ```bash
   make install
   ```

## Architecture Overview

LocalCloud follows a modular architecture:

```
/cmd/localcloud          # CLI entry point and root command
/internal/               # Private application code
  /cli/                  # CLI command implementations
  /docker/               # Docker container management
  /config/               # Configuration management
  /orchestrator/         # Service orchestration logic
  /models/               # AI model management
/pkg/                    # Public APIs and shared code
  /api/                  # REST API definitions
  /types/                # Shared type definitions
/templates/              # Application templates
  /chat/                 # Chat interface template
  /rag/                  # RAG system template
  /api/                  # API endpoint template
```

### Key Design Principles

1. **Single Binary**: Everything runs from one executable
2. **Offline First**: No internet required after initial setup
3. **Resource Efficient**: Optimized for 4GB RAM laptops
4. **Developer Experience**: Simple commands, clear errors
5. **Extensible**: Easy to add new templates and models

## Making Changes

### Code Style

- Follow standard Go conventions (`go fmt`, `go vet`)
- Use meaningful variable and function names
- Add comments for public APIs
- Keep functions small and focused

### Commit Messages

Use conventional commits format:
```
type(scope): description

Examples:
feat(cli): add model download command
fix(docker): resolve container cleanup issue
docs(readme): update installation instructions
```

### Branch Naming

- `feature/description` - New features
- `fix/description` - Bug fixes
- `docs/description` - Documentation updates
- `refactor/description` - Code refactoring

## Adding Templates

Templates are the core of LocalCloud's value proposition. Here's how to add a new template:

### Template Structure

```
/templates/my-template/
  /frontend/             # Next.js application
  /backend/              # Optional backend service
  /docker/               # Docker configurations
  template.yaml          # Template configuration
  README.md              # Template documentation
```

### Template Configuration

```yaml
# template.yaml
name: "my-template"
description: "Description of what this template does"
version: "1.0.0"
author: "Your Name"
category: "chat|rag|api|utility"

requirements:
  models:
    - name: "qwen2.5:3b"
      required: true
  ports:
    - 3000
    - 8080
  memory: "2GB"

services:
  frontend:
    type: "nextjs"
    path: "./frontend"
    port: 3000
  backend:
    type: "docker"
    path: "./docker/backend.dockerfile"
    port: 8080
```

### Template Development Guide

1. **Start with existing template**: Copy the closest existing template
2. **Update template.yaml**: Configure your template's requirements
3. **Develop frontend**: Use Next.js 14 with App Router
4. **Test thoroughly**: Ensure it works on different systems
5. **Document**: Write clear README with usage examples

## Testing

### Running Tests

```bash
# Run all tests
make test

# Run specific package tests
go test ./internal/cli/...

# Run tests with coverage
make test-coverage
```

### Test Categories

1. **Unit Tests**: Test individual functions and methods
2. **Integration Tests**: Test component interactions
3. **Template Tests**: Verify templates work end-to-end
4. **CLI Tests**: Test command-line interface

### Writing Tests

- Test files should end with `_test.go`
- Use table-driven tests for multiple scenarios
- Mock external dependencies (Docker, filesystem)
- Test both success and error cases

## Pull Request Process

1. **Create Feature Branch**
   ```bash
   git checkout -b feature/my-feature
   ```

2. **Make Changes**: Follow the guidelines above

3. **Test Locally**
   ```bash
   make test
   make build
   # Test the built binary
   ```

4. **Create Pull Request**
   - Use descriptive title and description
   - Reference any related issues
   - Include screenshots for UI changes
   - Ensure CI passes

5. **Code Review**
   - Address reviewer feedback
   - Keep discussions respectful and constructive
   - Update documentation if needed

## Community Guidelines

### Code of Conduct

- Be respectful and inclusive
- Focus on constructive feedback
- Help others learn and grow
- Celebrate different perspectives

### Getting Help

- **Discord**: Join our development discussions
- **GitHub Issues**: Report bugs or request features
- **GitHub Discussions**: Ask questions or share ideas

### Recognition

Contributors are recognized in:
- README contributors section
- Release notes
- Community highlights

## Development Tools

### Useful Make Commands

```bash
make build          # Build the binary
make test           # Run all tests
make lint           # Run linters
make clean          # Clean build artifacts
make install        # Install binary locally
make dev            # Start development mode
make docker-build   # Build Docker images for templates
```

### Debugging

- Use Go's built-in debugging tools
- Add verbose logging with `--verbose` flag
- Test with different AI models
- Use Docker logs for container debugging

---

Thank you for contributing to LocalCloud! Together, we're making AI development accessible to everyone. 