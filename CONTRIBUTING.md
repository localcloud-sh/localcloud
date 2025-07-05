# Contributing to LocalCloud

We're excited to have you contribute to LocalCloud! This guide will help you get started with development and understand our contribution process.

> **Note**: For questions or discussions, check out [GitHub Discussions](https://github.com/localcloud-sh/localcloud/discussions) or email us at [dev@localcloud.sh](mailto:dev@localcloud.sh).

## Table of Contents
- [Development Setup](#development-setup)
- [Architecture Overview](#architecture-overview)
- [Making Changes](#making-changes)
- [Adding Services](#adding-new-services)
- [Testing](#testing)
- [Pull Request Process](#pull-request-process)
- [Community Guidelines](#community-guidelines)

## Development Setup

### Prerequisites
- Go 1.21 or later
- Docker Desktop
- Node.js 18+ (for development tools)
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
  /services/             # Service orchestration logic
  /models/               # AI model management
  /templates/            # Template generator
/pkg/                    # Public APIs and shared code
  /api/                  # REST API definitions  
  /types/                # Shared type definitions
/scripts/                # Build and installation scripts
  /build.sh              # Multi-platform build script
  /install.sh            # Installation script
/.github/                # GitHub specific files
  /workflows/            # GitHub Actions
```

### Key Design Principles

1. **Single Binary**: Everything runs from one executable
2. **Offline First**: No internet required after initial setup
3. **Resource Efficient**: Optimized for 4GB RAM laptops
4. **Developer Experience**: Simple commands, clear errors
5. **Extensible**: Easy to add new services and models

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

## Adding New Services

LocalCloud supports various AI, database, and infrastructure services. Here's how to add a new service:

### Service Implementation

```go
// internal/services/myservice/service.go
type MyService struct {
    config *config.ServiceConfig
}

func (s *MyService) Start() error {
    // Implementation
}

func (s *MyService) Stop() error {
    // Implementation
}

func (s *MyService) HealthCheck() error {
    // Implementation
}
```

### Service Configuration

Add to the setup wizard in `internal/cli/setup.go`:

```go
{
    Type:        "myservice",
    Name:        "My Service", 
    Description: "Description of what it does",
    Category:    "AI|Database|Infrastructure",
    Default:     false,
}
```

### Docker Image

If your service needs a custom image, add it to the docker management layer.

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
3. **Service Tests**: Verify services start/stop correctly
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
   ./dist/localcloud --version
   ```

4. **Create Pull Request**
   - Use descriptive title and description
   - Reference any related issues
   - Include examples of the feature in action
   - Ensure CI passes

5. **Code Review**
   - Address reviewer feedback
   - Keep discussions respectful and constructive
   - Update documentation if needed

## Release Process

### For Maintainers

1. **Prepare Release**
   ```bash
   # Update version in code
   # Update CHANGELOG.md
   git commit -m "chore: prepare v0.1.0 release"
   ```

2. **Tag Release**
   ```bash
   git tag -a v0.1.0 -m "Release v0.1.0"
   git push origin v0.1.0
   ```

3. **Verify**
   - GitHub Actions builds binaries
   - Release is created automatically
   - Homebrew tap is updated

## Community Guidelines

### Code of Conduct

- Be respectful and inclusive
- Focus on constructive feedback
- Help others learn and grow
- Celebrate different perspectives

### Getting Help

- 🐛 **[GitHub Issues](https://github.com/localcloud-sh/localcloud/issues)** - Bug reports and feature requests
- 💬 **[GitHub Discussions](https://github.com/localcloud-sh/localcloud/discussions)** - Questions, ideas, and community chat
- 📧 **[dev@localcloud.sh](mailto:dev@localcloud.sh)** - Direct contact for collaboration

### Recognition

Contributors are recognized in:
- 🏆 **Contributors section** in README
- 📝 **Release notes** for each version
- 🌟 **Community highlights** and social media
- 🎯 **Special thanks** in documentation

## Development Tools

### Useful Make Commands

```bash
make build          # Build the binary
make test           # Run all tests  
make lint           # Run linters
make clean          # Clean build artifacts
make install        # Install binary locally
make release        # Build all platform binaries
```

### Project Structure Details

```
localcloud/
├── cmd/localcloud/       # Main application entry
│   └── main.go          # Entry point
├── internal/            # Private packages
│   ├── cli/            # Command implementations
│   │   ├── root.go     # Root command
│   │   ├── setup.go    # Setup command
│   │   ├── start.go    # Start command
│   │   ├── status.go   # Status command
│   │   └── ...
│   ├── docker/         # Docker management
│   │   ├── client.go   # Docker client
│   │   ├── container.go # Container operations
│   │   └── manager.go  # Service orchestration
│   └── config/         # Configuration
│       ├── config.go   # Config structures
│       └── loader.go   # Config loading
├── scripts/            # Build & install scripts
├── .github/            # GitHub Actions
└── Makefile           # Build commands
```

### Debugging Tips

- Use `lc --verbose` for detailed output
- Check Docker logs: `lc logs [service]`
- Inspect containers: `docker ps -a`
- Check service health: `lc status`

---

<div align="center">

**Thank you for contributing to LocalCloud!** 🚀

Together, we're making AI development accessible to everyone.

[Website](https://localcloud.sh) • [Documentation](https://docs.localcloud.sh) • [GitHub](https://github.com/localcloud-sh/localcloud) • [Contact](mailto:dev@localcloud.sh)

</div>