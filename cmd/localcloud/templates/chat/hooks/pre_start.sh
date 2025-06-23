#!/bin/bash
# cmd/localcloud/templates/chat/hooks/pre_start.sh
# Pre-start hook for chat template

# Load environment variables if .env exists
if [ -f ".env" ]; then
    export $(cat .env | grep -v '^#' | xargs)
elif [ -f "../.env" ]; then
    export $(cat ../.env | grep -v '^#' | xargs)
fi

echo "🔍 Checking services before start..."

# Function to wait for service
wait_for_service() {
    local service=$1
    local max_attempts=30
    local attempt=1

    echo "⏳ Waiting for $service to be ready..."

    while [ $attempt -le $max_attempts ]; do
        if lc status $service 2>/dev/null | grep -q "running"; then
            echo "✅ $service is ready"
            return 0
        fi

        echo -n "."
        sleep 1
        attempt=$((attempt + 1))
    done

    echo ""
    echo "❌ Timeout waiting for $service"
    return 1
}

# Ensure required services are running
SERVICES="ollama postgres"

for service in $SERVICES; do
    if ! lc status $service 2>/dev/null | grep -q "running"; then
        echo "🚀 Starting $service..."
        lc start $service

        if ! wait_for_service $service; then
            echo "❌ Failed to start $service"
            exit 1
        fi
    else
        echo "✅ $service is already running"
    fi
done

# Wait for database to be ready
echo "⏳ Waiting for database connection..."

# Get container name from lc ps or use default
POSTGRES_CONTAINER=$(lc ps 2>/dev/null | grep postgres | awk '{print $1}' | head -1)
if [ -z "$POSTGRES_CONTAINER" ]; then
    POSTGRES_CONTAINER="localcloud-postgres"
fi

for i in {1..10}; do
    if docker exec $POSTGRES_CONTAINER pg_isready -U "${DATABASE_USER:-localcloud}" >/dev/null 2>&1; then
        echo "✅ Database is ready"
        break
    fi

    if [ $i -eq 10 ]; then
        echo "❌ Database is not responding"
        exit 1
    fi

    sleep 1
done

# Ensure database schema exists
echo "🗄️  Checking database schema..."

# Set default values if not provided
DB_USER="${DATABASE_USER:-localcloud}"
DB_NAME="${DATABASE_NAME:-localcloud}"

TABLES=$(docker exec $POSTGRES_CONTAINER psql -U "$DB_USER" -d "$DB_NAME" -t -c "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema='public' AND table_name IN ('conversations', 'messages')" 2>/dev/null | tr -d ' ')

if [ -z "$TABLES" ] || [ "$TABLES" -lt "2" ]; then
    echo "📝 Creating database schema..."
    # Copy migration file to container and execute
    docker cp migrations/001_initial_schema.sql $POSTGRES_CONTAINER:/tmp/
    docker exec $POSTGRES_CONTAINER psql -U "$DB_USER" -d "$DB_NAME" -f /tmp/001_initial_schema.sql

    if [ $? -eq 0 ]; then
        echo "✅ Database schema created"
    else
        echo "⚠️  Failed to create database schema, but continuing..."
    fi
else
    echo "✅ Database schema exists"
fi

# Ensure model is available
MODEL="${AI_MODEL:-qwen2.5:3b}"
if ! lc models list 2>/dev/null | grep -q "$MODEL"; then
    echo "⚠️  Model '$MODEL' not found"
    echo "   Pulling model now..."
    lc models pull "$MODEL"

    if [ $? -ne 0 ]; then
        echo "❌ Failed to pull model"
        echo "   The application will not work properly without the model"
        exit 1
    fi
else
    echo "✅ Model '$MODEL' is available"
fi

echo ""
echo "✅ All services ready!"
exit 0