#!/bin/bash
# cmd/localcloud/templates/chat/hooks/post_setup.sh
# Post-setup hook for chat template

# Load environment variables if .env exists
if [ -f ".env" ]; then
    export $(cat .env | grep -v '^#' | xargs)
elif [ -f "../.env" ]; then
    export $(cat ../.env | grep -v '^#' | xargs)
fi

echo "🚀 Running post-setup tasks..."

# Install dependencies
echo "📦 Installing frontend dependencies..."
cd frontend && npm install --silent
if [ $? -ne 0 ]; then
    echo "❌ Failed to install frontend dependencies"
    exit 1
fi

echo "📦 Installing backend dependencies..."
cd ../backend && npm install --silent
if [ $? -ne 0 ]; then
    echo "❌ Failed to install backend dependencies"
    exit 1
fi

cd ..

# Create .gitignore if it doesn't exist
if [ ! -f .gitignore ]; then
    echo "📝 Creating .gitignore..."
    cat > .gitignore << EOF
# Dependencies
node_modules/

# Production builds
dist/
build/

# Environment files
.env
.env.local
.env.*.local

# Logs
*.log
npm-debug.log*

# Editor directories
.vscode/
.idea/

# OS files
.DS_Store
Thumbs.db

# LocalCloud
.localcloud/cache/
.localcloud/logs/
EOF
fi

# Initialize git repo if not exists
if [ ! -d .git ]; then
    echo "📝 Initializing git repository..."
    git init
    git add .
    git commit -m "Initial commit - LocalCloud chat template" --quiet
fi

# Pull model if needed
MODEL="${AI_MODEL:-qwen2.5:3b}"
if ! lc models list 2>/dev/null | grep -q "$MODEL"; then
    echo "🤖 Pulling AI model: $MODEL"
    echo "   This may take a few minutes..."
    lc models pull "$MODEL"
    if [ $? -ne 0 ]; then
        echo "⚠️  Failed to pull model, but you can pull it later with:"
        echo "   lc models pull $MODEL"
    fi
fi

# Note about database schema
echo "📝 Note: Database schema will be created when services start"
echo "   The migration will run automatically on first connection"

# Success message
echo ""
echo "✅ Setup completed successfully!"
echo ""
echo "📋 Next steps:"
echo "   1. Start the services: lc start"
echo "   2. Open frontend: http://localhost:${FRONTEND_PORT}"
echo "   3. API endpoint: http://localhost:${API_PORT}"
echo ""
echo "📚 For more information, see README.md"

exit 0