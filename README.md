# LocalCloud

> Transform your laptop into a powerful AI workstation in seconds

LocalCloud is a revolutionary platform that turns any machine into a fully-featured AI development environment. Deploy chat interfaces, RAG systems, and custom AI workflows with a single command - no cloud dependencies, no recurring costs, no data leaving your machine.

## Vision

**Make AI development as simple as running a local web server.**

LocalCloud bridges the gap between complex AI infrastructure and developer productivity. We believe every developer should be able to prototype, build, and deploy AI applications locally without wrestling with complex setups or sending sensitive data to third parties.

## Features

🚀 **One-Command Setup**: Install and run complete AI stacks instantly  
🔒 **Privacy First**: Your data never leaves your machine  
💰 **Zero Recurring Costs**: No cloud bills, ever  
🎯 **Pre-built Templates**: Chat, RAG, API endpoints ready to customize  
⚡ **Optimized for Laptops**: Efficient 4GB RAM models that actually work  
🔧 **Developer Friendly**: Full source access, extensible architecture  

## Quick Start

```bash
# Install LocalCloud
curl -fsSL https://install.localcloud.dev | sh

# Start a chat interface
localcloud create chat my-ai-assistant
localcloud start my-ai-assistant

# Visit http://localhost:3000
```

## Architecture

LocalCloud consists of:
- **CLI Tool**: Single binary for all operations
- **AI Models**: Curated small models (qwen2.5:3b, deepseek-coder:1.3b, etc.)
- **Templates**: Pre-built applications (Chat, RAG, API)
- **Orchestrator**: Manages containers and services
- **Mobile Support**: mDNS discovery + tunnel fallback

## Development Status

🚧 **Phase 1 - MVP Development** (Current)
- Core CLI functionality
- Docker-based AI model management
- Basic chat and RAG templates
- Local deployment system

## Contributing

We're building in the open! Check out [CONTRIBUTING.md](./CONTRIBUTING.md) for:
- Development setup
- Architecture decisions
- How to add new templates
- Community guidelines

## License

MIT License - see [LICENSE](./LICENSE) for details.

---

**LocalCloud** - AI development, simplified. 