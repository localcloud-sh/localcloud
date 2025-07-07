# mini - LocalCloud Project

## 🤖 AI Assistant Guide

This is a LocalCloud project for local AI development. LocalCloud eliminates cloud costs during development by running everything locally.

## Available Commands

### Core Commands
- **`lc start`** - Start all configured services
- **`lc status`** - Check service status and connection info
- **`lc stop`** - Stop all services
- **`lc restart`** - Restart services

### AI Models
- **`lc models list`** - List available models
- **`lc models pull llama3.2:3b`** - Download AI models
- **`lc models remove <model>`** - Remove models

### Data Export (for production)
- **`lc export all`** - Export all data for cloud migration
- **`lc export db`** - Export PostgreSQL database
- **`lc export storage`** - Export MinIO storage
- **`lc export mongo`** - Export MongoDB (if configured)

### Project Management
- **`lc setup`** - Configure project components
- **`lc info`** - Show project and system information
- **`lc doctor`** - Check system requirements

## Typical Development Workflow

1. **Start services**: `lc start`
2. **Download AI model**: `lc models pull llama3.2:3b`
3. **Check status**: `lc status` (shows connection endpoints)
4. **Develop your application** using the local services
5. **Export for production**: `lc export all --output=./production-backup/`

## Project Configuration

Run `lc status` to see which services are configured and their connection details.
Common services include:
- **PostgreSQL** (local database)
- **AI Models** via Ollama
- **MinIO** (S3-compatible storage)
- **Redis** (caching)
- **MongoDB** (document database)

## Notes for AI Assistants

- This project uses LocalCloud for local development
- All services run locally (no cloud costs during development)
- Use `lc export` commands when ready to migrate to production
- Connection details are shown in `lc status` output
- Always start services with `lc start` before development
