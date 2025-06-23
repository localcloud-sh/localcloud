# {{.ProjectName}} - LocalCloud Chat Application

A ChatGPT-like interface built with LocalCloud, featuring streaming responses, conversation history, and model switching.

## Features

- 💬 Real-time streaming chat responses
- 📝 Persistent conversation history
- 🔄 Model switching on the fly
- 🌙 Dark/Light mode support
- 📱 Mobile responsive design
- 🎨 Markdown rendering with syntax highlighting
- 📋 Code copying functionality

## Tech Stack

- **Frontend**: React 18 + Vite + Tailwind CSS
- **Backend**: Express.js + PostgreSQL
- **AI**: Ollama with {{.ModelName}}
- **Database**: PostgreSQL 16

## Getting Started

### Prerequisites

- Docker installed and running
- LocalCloud CLI installed
- At least 4GB RAM available

### Running the Application

```bash
# Start all services
lc start

# Or start specific services
lc start ollama postgres
lc start api frontend
```

### Accessing the Application

- **Frontend**: http://localhost:{{.FrontendPort}}
- **API**: http://localhost:{{.APIPort}}
- **Database**: localhost:{{.DatabasePort}}

## Development

### Frontend Development

```bash
cd frontend
npm install
npm run dev
```

### Backend Development

```bash
cd backend
npm install
npm run dev
```

### Database Management

Connect to PostgreSQL:
```bash
psql -h localhost -p {{.DatabasePort}} -U {{.DatabaseUser}} -d {{.DatabaseName}}
```

## API Endpoints

- `GET /api/models` - List available AI models
- `GET /api/conversations` - List conversations
- `POST /api/conversations` - Create new conversation
- `GET /api/conversations/:id/messages` - Get messages for a conversation
- `POST /api/chat/completions` - Send message and get AI response
- `DELETE /api/conversations/:id` - Delete a conversation

## Configuration

All configuration is managed through environment variables in `.env`:

- `AI_MODEL` - Default AI model to use
- `API_PORT` - Backend API port
- `FRONTEND_PORT` - Frontend dev server port
- `DATABASE_*` - PostgreSQL connection settings

## Customization

### Adding New Models

Models are automatically detected from Ollama. To add a new model:

```bash
lc models pull <model-name>
```

### Styling

The application uses Tailwind CSS. Modify `frontend/src/index.css` for custom styles.

### Database Schema

See `backend/db/schema.sql` for the database structure. Migrations are in the `migrations/` folder.

## Troubleshooting

### Services not starting

```bash
# Check service status
lc status

# View logs
lc logs api
lc logs postgres
```

### Database connection issues

1. Ensure PostgreSQL is running: `lc status postgres`
2. Check credentials in `.env`
3. Verify port {{.DatabasePort}} is not in use

### Model not responding

1. Check Ollama is running: `lc status ollama`
2. Verify model is installed: `lc models list`
3. Check available RAM: `lc system info`

## License

This project was generated with LocalCloud. See LICENSE for details.