<div align="center">

  <img src=".github/assets/banner.svg" alt="LocalCloud - Ship AI Products Before Your Coffee Gets Cold" width="100%">

[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](LICENSE)
[![Go Version](https://img.shields.io/badge/Go-1.21+-00ADD8?logo=go)](go.mod)
[![Docker Required](https://img.shields.io/badge/Docker-Required-2496ED?logo=docker)](https://docker.com)

</div>

**LocalCloud transforms your laptop into a complete AI development environment.** Run GPT-level models, vector databases, and production infrastructure locally with zero cloud costs. No API keys, no usage limits, no data leaving your machine.

## 🚦 Quick Start

```bash
# Install LocalCloud
curl -fsSL https://get.localcloud.ai | sh

# Initialize a new project
lc init my-assistant
cd my-assistant

# Configure your services (interactive setup)
lc setup
```

You'll see an interactive wizard:
```
? What would you like to build? (Use arrow keys)
❯ Chat Assistant - Conversational AI with memory
  RAG System - Document Q&A with vector search  
  Speech Processing - Whisper + TTS
  Custom - Select components manually

? Select components you need: (Press <space> to select, <enter> to confirm)
❯ ◯ [AI] LLM (Text generation) - Large language models for text generation
  ◯ [AI] Embeddings (Semantic search) - Text embeddings for similarity
  ◯ [AI] Speech-to-Text (Whisper) - Convert speech to text
  ◯ [Database] Vector Database (pgvector) - PostgreSQL with pgvector
  ◯ [Infrastructure] Cache (Redis) - In-memory cache for sessions
  ◯ [Infrastructure] Queue (Redis) - Job queue for background processing
  ◯ [Infrastructure] Object Storage (MinIO) - S3-compatible storage
```

```bash
# Start all selected services
lc start

# Your AI services are now running!
# Check status: lc status
```

> **Note**: `lc` is the short alias for `localcloud` - use whichever you prefer!

## ✨ Key Features

- **🚀 One-Command Setup**: Get started in seconds with `lc setup`
- **💰 Zero Cloud Costs**: Everything runs locally - no API fees or usage limits
- **🔒 Complete Privacy**: Your data never leaves your machine
- **📦 Pre-built Templates**: Production-ready backends for common AI use cases
- **🧠 Optimized Models**: Carefully selected models that run on 4GB RAM
- **🔧 Developer Friendly**: Simple CLI, clear errors, extensible architecture
- **🐳 Docker-Based**: Consistent environment across all platforms
- **🌐 Mobile Ready**: Built-in tunnel support for demos anywhere

## 🎯 Vision

**Make AI development as simple as running a local web server.**

LocalCloud eliminates the complexity and cost of AI development by providing a complete, local-first development environment. No cloud bills, no data privacy concerns, no complex configurations - just pure development productivity.

## 💡 Perfect For These Scenarios

### 🏢 **Enterprise POCs Without The Red Tape**
Waiting 3 weeks for cloud access approval? Your POC could be done by then. LocalCloud lets you build and demonstrate AI solutions immediately, no IT tickets required.

### 📱 **Mobile Demos That Actually Work**
Present from your phone to any client's screen. Built-in tunneling means you can demo your AI app from anywhere - coffee shop WiFi, client office, or conference room.

### 💸 **The $2,000 Cloud Bill You Forgot About**
We've all been there - spun up a demo, showed the client, forgot to tear it down. With LocalCloud, closing your laptop *is* shutting down the infrastructure.

### 🎓 **Turn Lecture Recordings into Study Notes**
Got 50 hours of lecture recordings? LocalCloud + Whisper can transcribe them all for free. Add RAG and you've got an AI study buddy that knows your entire course.

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

## 📚 Available Templates

During `lc setup`, you can choose from pre-configured templates or customize your own service selection:

### 1. Chat Assistant with Memory
```bash
lc init my-assistant
lc setup  # Select "Chat Assistant" template
```
- Conversational AI with persistent memory
- PostgreSQL for conversation storage
- Streaming responses
- Model switching support

### 2. RAG System (Retrieval-Augmented Generation)
```bash
lc init my-knowledge-base
lc setup  # Select "RAG System" template
```
- Document ingestion and embedding
- Vector search with pgvector
- Context-aware responses
- Scalable to millions of documents

### 3. Speech-to-Text with Whisper
```bash
lc init my-transcriber
lc setup  # Select "Speech/Whisper" template
```
- Audio transcription API
- Multiple language support
- Real-time processing
- Optimized Whisper models

### 4. Custom Selection
```bash
lc init my-project
lc setup  # Choose "Custom" and select individual services
```
- Pick only the services you need
- Configure each service individually
- Optimal resource usage

> **Note**: MVP includes backend infrastructure only. Frontend applications coming in v2.

## 🏗️ Architecture

```
LocalCloud Project Structure:
├── .localcloud/          # Project configuration
│   └── config.yaml       # Service configurations
├── docker-compose.yml    # Generated service definitions
└── .env                  # Environment variables
```

### Setup Flow

1. **Initialize**: `lc init` creates project structure
2. **Configure**: `lc setup` opens interactive wizard
   - Choose template or custom services
   - Select AI models
   - Configure ports and resources
3. **Start**: `lc start` launches all services
4. **Develop**: Services are ready for your application

### Core Services

| Service | Description | Default Port |
|---------|-------------|--------------|
| **AI/LLM** | Ollama with selected models | 11434 |
| **Database** | PostgreSQL with pgvector | 5432 |
| **Cache** | Redis for performance | 6379 |
| **Queue** | Redis for job processing | 6380 |
| **Storage** | MinIO (S3-compatible) | 9000/9001 |

### AI Components

| Component | Service | Use Case |
|-----------|---------|----------|
| **Whisper** | Speech-to-Text | Audio transcription |
| **Piper** | Text-to-Speech | Voice synthesis |
| **Stable Diffusion** | Image Generation | AI images |
| **Qdrant** | Vector Database | Similarity search |

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
# Initialize new project
lc init [project-name]

# Interactive setup wizard
lc setup

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

## 🤝 Contributing

We welcome contributions! See [CONTRIBUTING.md](CONTRIBUTING.md) for:
- Development setup
- Code style guidelines
- Testing requirements
- Pull request process

## 📖 Documentation

- **Quick Start Guide**: [docs/getting-started.md](docs/getting-started.md)
- **API Reference**: [docs/api-reference.md](docs/api-reference.md)
- **Template Guide**: [docs/templates.md](docs/templates.md)
- **Troubleshooting**: [docs/troubleshooting.md](docs/troubleshooting.md)

## 🗺️ Roadmap

### ✅ Phase 1 - MVP (Current)
- [x] Core CLI with `lc` alias
- [x] Interactive setup wizard
- [x] Docker service orchestration
- [x] PostgreSQL with pgvector
- [x] Model management
- [x] Service templates
- [x] Service health monitoring

### 🚧 Phase 2 - Application Layer
- [ ] Backend code templates
- [ ] LocalCloud SDK
- [ ] API scaffolding
- [ ] Migration system

### 🔮 Phase 3 - Frontend & Advanced
- [ ] Next.js frontend templates
- [ ] Web-based admin panel
- [ ] Mobile app support
- [ ] Model fine-tuning
- [ ] Kubernetes support

## 📄 License

Apache License 2.0 - see [LICENSE](LICENSE) for details.

### Why Apache 2.0?
- ✅ **Enterprise-friendly** - Your legal team will actually approve it
- ✅ **Patent protection** - Explicit patent grants protect everyone
- ✅ **Commercial use** - Build products, sell services, no restrictions
- ✅ **Modification** - Fork it, change it, make it yours
- ✅ **Attribution** - Just keep the license notice

Perfect for both startups building products and enterprises needing compliance.

## 🙏 Acknowledgments

LocalCloud is built on the shoulders of giants:
- [Ollama](https://ollama.ai) for local model serving
- [PostgreSQL](https://postgresql.org) for reliable data storage
- [Docker](https://docker.com) for containerization
- All the open-source AI models making this possible

---

<div align="center">
  <b>LocalCloud</b> - AI development at zero cost, infinite possibilities
  <br>
  <a href="https://localcloud.ai">Website</a> • 
  <a href="https://github.com/localcloud/localcloud">GitHub</a> • 
  <a href="https://discord.gg/localcloud">Discord</a> • 
  <a href="https://twitter.com/localcloudai">Twitter</a>
</div>