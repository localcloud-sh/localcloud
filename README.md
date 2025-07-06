<div align="center">

  <img src=".github/assets/banner.svg" alt="LocalCloud - Ship AI Products Before Your Coffee Gets Cold" width="100%">

[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](LICENSE)
[![Language Agnostic](https://img.shields.io/badge/Language-Agnostic-brightgreen?logo=polywork)](README.md)
[![Docker Required](https://img.shields.io/badge/Docker-Required-2496ED?logo=docker)](https://docker.com)
[![GitHub release](https://img.shields.io/github/v/release/localcloud-sh/localcloud?color=success)](https://github.com/localcloud-sh/localcloud/releases)
[![GitHub issues](https://img.shields.io/github/issues/localcloud-sh/localcloud)](https://github.com/localcloud-sh/localcloud/issues)
[![GitHub stars](https://img.shields.io/github/stars/localcloud-sh/localcloud?style=social)](https://github.com/localcloud-sh/localcloud/stargazers)
[![Platform Support](https://img.shields.io/badge/Platform-macOS%20%7C%20Linux%20%7C%20Windows-lightgrey)](README.md#installation)
[![AI Assistant Ready](https://img.shields.io/badge/AI%20Assistant-Ready-9cf?logo=openai)](README.md#🤖-ai-assistant-integration-guide)
[![Test Suite](https://img.shields.io/badge/Tests-8%20Components-green)](test-components/README.md)

</div>

**From idea to running code in minutes, not weeks.** LocalCloud delivers developer-friendly PostgreSQL, MongoDB, vector databases, AI models, Redis cache, job queues, and S3-like storage instantly. No DevOps, no cloud bills, no infrastructure drama.

**🌐 Programming Language Agnostic** - Works seamlessly with Python, Node.js, Go, Java, Rust, PHP, .NET, or any language. LocalCloud provides standard service APIs (PostgreSQL, MongoDB, Redis, S3, etc.) that integrate with your existing code regardless of technology stack.

## 🚀 **Why Developers Choose LocalCloud**

- 💸 **Bootstrapped Startups** - Build MVPs with zero infrastructure costs during early development  
- 🔒 **Privacy-First Enterprises** - Run open-source AI models locally, keeping data in-house
- ⏰ **Corporate Developers** - Skip IT approval queues - get PostgreSQL and Redis running now
- 📱 **Demo Heroes** - Tunnel your app to any device - present from iPhone to client's iPad instantly
- 🤝 **Remote Teams** - Share running environments with frontend developers without deployment hassles
- 🎓 **Students & Learners** - Master databases and AI without complex setup or cloud accounts
- 🧪 **Testing Pipelines** - Integrate AI and databases in CI without external dependencies
- 🔧 **Prototype Speed** - Spin up full-stack environments faster than booting a VM
- 🤖 **AI Assistant Users** - Works seamlessly with Claude Code, Cursor, Gemini CLI for AI-powered development

## 🚦 Quick Start

### Installation

**Quick Install (macOS & Linux):**
```bash
curl -fsSL https://localcloud.sh/install | bash
```

**Or via Homebrew:**
```bash
brew install localcloud-sh/tap/localcloud
```

<details>
<summary>Manual Installation & Windows</summary>

**macOS Manual:**
```bash
# Apple Silicon
curl -L https://localcloud.sh/releases/darwin-arm64 | tar xz
sudo install -m 755 localcloud-darwin-arm64 /usr/local/bin/localcloud

# Intel
curl -L https://localcloud.sh/releases/darwin-amd64 | tar xz
sudo install -m 755 localcloud-darwin-amd64 /usr/local/bin/localcloud
```

**Linux Manual:**
```bash
# AMD64
curl -L https://localcloud.sh/releases/linux-amd64 | tar xz
sudo install -m 755 localcloud-linux-amd64 /usr/local/bin/localcloud

# ARM64
curl -L https://localcloud.sh/releases/linux-arm64 | tar xz
sudo install -m 755 localcloud-linux-arm64 /usr/local/bin/localcloud
```

**Windows (Experimental):**
1. Download from [GitHub Releases](https://github.com/localcloud-sh/localcloud/releases)
2. Extract and add to PATH
3. Use WSL2 for best compatibility

</details>

### Getting Started

#### 👨‍💻 Interactive Setup

```bash
# Setup your project with an interactive wizard
lc setup
```

You'll see an interactive wizard:
```
? What would you like to build? (Use arrow keys)
❯ Chat Assistant - Conversational AI with memory
  RAG System - Document Q&A with vector search  
  Custom - Select components manually

? Select components you need: (Press <space> to select, <enter> to confirm)
❯ ◯ [AI] LLM (Text generation) - Large language models for text generation, chat, and completion
  ◯ [AI] Embeddings (Semantic search) - Text embeddings for semantic search and similarity
  ◯ [Database] Database (PostgreSQL) - Standard relational database for data storage
  ◯ [Database] Vector Search (pgvector) - Add vector similarity search to PostgreSQL
  ◯ [Database] NoSQL Database (MongoDB) - Document-oriented database for flexible data storage
  ◯ [Infrastructure] Cache (Redis) - In-memory cache for temporary data and sessions
  ◯ [Infrastructure] Queue (Redis) - Reliable job queue for background processing
  ◯ [Infrastructure] Object Storage (MinIO) - S3-compatible object storage for files and media
```

Then start your services:
```bash
lc start

# Your infrastructure is now running!
# Check status: lc status
```

#### 🤖 Non-Interactive Setup (AI Assistants)

AI assistants can set up projects with simple commands:

```bash
# Quick presets for common stacks
lc setup my-ai-app --preset=ai-dev --yes        # AI + Database + Vector search
lc setup my-app --preset=full-stack --yes       # Everything included
lc setup blog --preset=minimal --yes            # Just AI models

# Or specify exact components
lc setup my-app --components=llm,database,storage --models=llama3.2:3b --yes
```

> **Note**: `lc` is the short alias for `localcloud` - use whichever you prefer!

## ✨ Key Features

- **🚀 One-Command Setup**: Create and configure projects with just `lc setup`
- **💰 Zero Cloud Costs**: Everything runs locally - no API fees or usage limits
- **🔒 Complete Privacy**: Your data never leaves your machine
- **📦 Pre-built Templates**: Production-ready backends for common AI use cases
- **🧠 Optimized Models**: Carefully selected models that run on 4GB RAM
- **🔧 Developer Friendly**: Simple CLI, clear errors, extensible architecture
- **🐳 Docker-Based**: Consistent environment across all platforms
- **🌐 Mobile Ready**: Built-in tunnel support for demos anywhere
- **📤 Export Tools**: One-command migration to any cloud provider
- **🤖 AI Assistant Ready**: Non-interactive setup perfect for Claude Code, Cursor, Gemini CLI

## 🎯 Vision

**Make production infrastructure as simple as running a local web server.**

LocalCloud eliminates the complexity and cost of infrastructure setup by providing a complete, local-first development environment. No cloud bills, no data privacy concerns, no complex configurations - just pure development productivity.

## 🤖 AI Assistant Integration

**For AI coding assistants:** Share this repository link to give your AI assistant complete context:

> *"I'm using LocalCloud for local AI development. Please review this repository to understand its capabilities: https://github.com/localcloud-sh/localcloud"*

Your AI assistant will automatically understand all commands and help you build applications using LocalCloud's non-interactive setup options.

## 💡 Perfect For These Scenarios

### 🏢 **Enterprise POCs Without The Red Tape**
Waiting 3 weeks for cloud access approval? Your POC could be done by then. LocalCloud lets you build and demonstrate AI solutions immediately, no IT tickets required.

### 📱 **Mobile Demos That Actually Work**
Present from your phone to any client's screen. Built-in tunneling means you can demo your AI app from anywhere - coffee shop WiFi, client office, or conference room.

### 💸 **The $2,000 Cloud Bill You Forgot About**
We've all been there - spun up a demo, showed the client, forgot to tear it down. With LocalCloud, closing your laptop *is* shutting down the infrastructure.

### 🔐 **When "Cloud-First" Meets "Compliance-First"**
Healthcare, finance, government? Some data can't leave the building. LocalCloud keeps everything local while giving you cloud-level capabilities.

### 🚀 **Hackathon Secret Weapon**
No API rate limits. No usage caps. No waiting for credits. Just pure development speed when every minute counts.

### 💼 **Build Commercial Products Without Burning Cash**
**Your Own Cursor/Copilot**: Build an AI code editor without $10k/month in API costs during development.  
**AI Mobile Apps**: Develop and test your AI-powered iOS/Android app locally until you're ready to monetize.  
**SaaS MVP**: Validate your AI startup idea without cloud bills - switch to cloud only after getting paying customers.

### 🎯 **Technical Interview Assignments That Shine**
**For Employers**: Give candidates a pre-configured LocalCloud environment. No setup headaches, just coding skills evaluation.  
**For Candidates**: Submit a fully-working AI application. While others struggle with API keys, you ship a complete solution.

### 🛠️ **Internal Tools That Would Never Get Budget**
**AI Customer Support Trainer**: Process your support tickets locally to train a custom assistant.  
**Code Review Bot**: Build a team-specific code reviewer without sending code to external APIs.  
**Meeting Transcription System**: Record, transcribe, and summarize meetings - all on company hardware.

### 🤖 **AI Assistant Development in One Coffee Sip**
**"Hey Claude, build me a chatbot backend"** → Your AI assistant runs `lc setup my-chatbot --preset=ai-dev --yes` and in 60 seconds you have PostgreSQL, vector search, AI models, and Redis running locally. Complete with database schema, API endpoints, and a working chat interface. By the time you finish your coffee, you're making API calls to your fully functional backend.

No cloud signup. No credit card. No infrastructure drama. Just pure AI-assisted development velocity.

## 📚 Available Templates

During `lc setup`, you can choose from pre-configured templates or customize your own service selection:

### 1. Chat Assistant with Memory
```bash
lc setup my-assistant  # Select "Chat Assistant" template
```
- Conversational AI with persistent memory
- PostgreSQL for conversation storage
- Streaming responses
- Model switching support

### 2. RAG System (Retrieval-Augmented Generation)
```bash
lc setup my-knowledge-base  # Select "RAG System" template
```
- Document ingestion and embedding
- Vector search with pgvector
- Context-aware responses
- Scalable to millions of documents

### 3. Speech-to-Text with Whisper
```bash
lc setup my-transcriber  # Select "Speech/Whisper" template
```
- Audio transcription API
- Multiple language support
- Real-time processing
- Optimized Whisper models

### 4. Custom Selection
```bash
lc setup my-project  # Choose "Custom" and select individual services
```
- Pick only the services you need
- Configure each service individually
- Optimal resource usage

> **Note**: MVP includes backend infrastructure only. Frontend applications coming in v2.

## 🏗️ Architecture

```
LocalCloud Project Structure:
├── .localcloud/          # Project configuration
│   └── config.yaml       # Service configurations and runtime settings
├── .gitignore           # Git ignore file (excludes .localcloud from version control)
└── your-app/            # Your application code goes here
```

### Setup Flow

1. **Setup**: `lc setup [project-name]` creates project and opens interactive wizard
   - Creates project structure (if new)
   - Choose template or custom services
   - Select AI models
   - Configure ports and resources
2. **Start**: `lc start` launches all services
3. **Develop**: Services are ready for your application

### Core Services

| Service | Description | Default Port |
|---------|-------------|--------------|
| **AI/LLM** | Ollama with selected models | 11434 |
| **Database** | PostgreSQL (optional pgvector extension) | 5432 |
| **MongoDB** | Document-oriented NoSQL database | 27017 |
| **Cache** | Redis for performance | 6379 |
| **Queue** | Redis for job processing | 6380 |
| **Storage** | MinIO (S3-compatible) | 9000/9001 |


## 🛠️ System Requirements

- **OS**: macOS, Linux, Windows (WSL2)
- **RAM**: 4GB minimum (8GB recommended)
- **Disk**: 10GB free space
- **Docker**: Docker Desktop or Docker Engine
- **CPU**: x64 or ARM64 processor

> **Note**: LocalCloud is written in Go for performance, but you don't need Go installed. The CLI is a single binary that works everywhere.

## 🎮 Command Reference

### Project Commands
```bash
# Create and configure new project
lc setup [project-name]

# Configure existing project (in current directory)
lc setup

# Add/remove components
lc setup --add llm
lc setup --add vector      # Add vector search to existing database
lc setup --remove cache
lc setup --remove vector   # Remove vector search, keep database

# Start all services
lc start

# Stop all services
lc stop

# View service status
lc status

# View logs
lc logs [service]
```

### Service Management
```bash
# Start specific service
lc service start postgres
lc service start whisper

# Stop specific service
lc service stop postgres

# Restart service
lc service restart ai

# Get service URL
lc service url postgres
```

### Database Commands
```bash
# Connect to database
lc db connect

# Backup database
lc db backup

# Restore from backup
lc db restore backup-file.sql

# Run migrations
lc db migrate
```

### Model Management
```bash
# List available models
lc models list

# Pull a new model
lc models pull llama3.2:3b

# Remove a model
lc models remove llama3.2:3b

# Show model information
lc models info qwen2.5:3b
```

## 🔧 Configuration

LocalCloud uses a simple YAML configuration:

```yaml
# .localcloud/config.yaml
project:
   name: my-assistant
   version: 1.0.0

models:
   default: qwen2.5:3b
   embeddings: nomic-embed-text

services:
   ai:
      memory_limit: 2GB
      gpu: false

   database:
      port: 5432
      extensions:
         - pgvector
```

## 🐛 Troubleshooting

### Docker Not Running
```bash
# Check Docker status
docker info

# macOS/Windows: Start Docker Desktop
# Linux: sudo systemctl start docker
```

### Port Already in Use
```bash
# Find process using port
lsof -i :3000

# Use different port
lc start --port 3001
```

### Model Download Issues
```bash
# Check available space
df -h

# Clear unused models
lc models prune
```

### Database Connection Failed
```bash
# Check if PostgreSQL is running
lc status postgres

# View PostgreSQL logs
lc logs postgres

# Restart PostgreSQL
lc service restart postgres
```

## 🧪 Testing Components

LocalCloud includes a comprehensive test suite for validating all components work correctly:

```bash
cd test-components

# Test all components
./test-runner.sh

# Test specific components
./test-runner.sh --components database,vector,llm

# Test with verbose output and progress monitoring
./test-runner.sh --components llm --verbose

# GitHub Actions compatible output
./test-runner.sh --format junit --output ./reports
```

**Key Features:**
- ✅ **Cross-platform**: Works on macOS, Linux with automatic timeout handling
- ✅ **Robust interruption**: Proper Ctrl+C handling and process cleanup
- ✅ **Smart monitoring**: Event-driven readiness detection for all services
- ✅ **CI/CD ready**: JUnit XML output for GitHub Actions integration
- ✅ **Dependency aware**: Understands component relationships (database ↔ vector)

For detailed testing documentation, see [test-components/README.md](test-components/README.md).

## 🤝 Contributing

We welcome contributions! See [CONTRIBUTING.md](CONTRIBUTING.md) for:
- Development setup
- Code style guidelines
- Testing requirements
- Pull request process

## 📖 Documentation

**[docs.localcloud.sh](https://docs.localcloud.sh)** - Complete documentation, CLI reference, and examples

## 🚀 Future Work

We're excited about the future of local-first AI development! Here are some ideas we're exploring:

### 🎯 **High-Impact Features**
- **Multi-Language SDKs** - Python, JavaScript, Go, and Rust client libraries
- **Web Admin Panel** - Visual service management and monitoring dashboard
- **Model Fine-tuning** - Train custom models on your local data
- **Team Collaboration** - Share projects and sync configurations across teams
- **Performance Optimization** - GPU acceleration and model quantization
- **Enterprise Features** - SSO, audit logs, and compliance tools
- **Project Isolation** - Currently, multiple projects share the same Docker containers (e.g., localcloud-mongodb, localcloud-postgres). Future releases will implement project-based container naming for complete isolation between projects

### 🤔 **Community Ideas**
- **Plugin System** - Extend LocalCloud with custom services
- **Alternative AI Providers** - Support for Hugging Face Transformers, OpenAI-compatible APIs
- **Cloud Sync** - Seamlessly transition from local to cloud deployment
- **Mobile Development** - Native iOS/Android development tools
- **Kubernetes Integration** - Deploy LocalCloud setups to K8s clusters
- **IDE Extensions** - VS Code and JetBrains plugins for better DX

### 💭 **Want to Shape the Future?**

We'd love to hear your ideas! Share your thoughts:
- 💬 **[GitHub Discussions](https://github.com/localcloud-sh/localcloud/discussions)** - Feature requests and community chat
- 🐛 **[GitHub Issues](https://github.com/localcloud-sh/localcloud/issues)** - Bug reports and specific feature requests
- 📧 **[dev@localcloud.sh](mailto:dev@localcloud.sh)** - Direct feedback and collaboration

Your input helps us prioritize what matters most to developers building AI applications.

## 📄 License

Licensed under Apache 2.0 - see [LICENSE](LICENSE) for details.

## 🙏 Acknowledgments

LocalCloud exists because of amazing open-source projects and communities:

### 🤖 **AI & ML Infrastructure**
- **[Ollama](https://ollama.ai)** - Our AI model serving foundation, making local LLMs accessible to everyone
- **[Meta AI](https://ai.meta.com)** - Llama models available through Ollama
- **[Mistral AI](https://mistral.ai)** - Mistral models available through Ollama
- **Model creators** - All the researchers and companies who open-source their models for Ollama

### 🗄️ **Database & Storage**
- **[PostgreSQL](https://postgresql.org)** - The world's most advanced open source database
- **[pgvector](https://github.com/pgvector/pgvector)** - Vector similarity search for PostgreSQL
- **[MongoDB](https://mongodb.com)** - Document database for modern applications
- **[Redis](https://redis.io)** - In-memory data structure store
- **[MinIO](https://min.io)** - High-performance object storage

### 🛠️ **Development Tools**
- **[Docker](https://docker.com)** - Containerization that makes deployment simple
- **[Go](https://golang.org)** - Fast, reliable, and efficient programming language
- **[Cobra](https://github.com/spf13/cobra)** - Powerful CLI framework for Go

### 🌟 **Special Thanks**
- The **Ollama team** for creating such an elegant local AI solution
- **Docker community** for making containerization accessible
- All the **model creators** who chose to open-source their work
- **Contributors and testers** who help make LocalCloud better

Without these projects and their maintainers, LocalCloud wouldn't exist. We're proud to be part of the open-source ecosystem.

---

<div align="center">
  <b>LocalCloud</b> - AI development at zero cost, infinite possibilities
  <br>
  <a href="https://localcloud.sh">Website</a> • 
  <a href="https://docs.localcloud.sh">Documentation</a> • 
  <a href="https://github.com/localcloud-sh/localcloud">GitHub</a> • 
  <a href="mailto:dev@localcloud.sh">Contact</a>
</div>