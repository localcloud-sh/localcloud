#!/bin/bash
# cmd/localcloud/templates/chat/hooks/pre_setup.sh
# Pre-setup hook for chat template

# Load environment variables if .env exists
if [ -f ".env" ]; then
    export $(cat .env | grep -v '^#' | xargs)
elif [ -f "../.env" ]; then
    export $(cat ../.env | grep -v '^#' | xargs)
fi

echo "🔍 Checking prerequisites for chat template..."

# Check if required services are available
check_service() {
    local service=$1
    # Check if service is in the ps output
    if ! lc ps 2>/dev/null | grep -q "$service"; then
        echo "❌ Required service '$service' is not running"
        echo "   Run: lc start $service"
        return 1
    fi
    echo "✅ Service '$service' is available"
    return 0
}

# Check required services
REQUIRED_SERVICES="ollama postgres"
FAILED=0

for service in $REQUIRED_SERVICES; do
    if ! check_service "$service"; then
        FAILED=1
    fi
done

# Check if model is available
MODEL="${AI_MODEL:-qwen2.5:3b}"
echo "🔍 Checking if model '$MODEL' is available..."

if ! lc models list 2>/dev/null | grep -q "$MODEL"; then
    echo "⚠️  Model '$MODEL' not found locally"
    echo "   It will be pulled during setup"
else
    echo "✅ Model '$MODEL' is available"
fi

# Check system resources - simplified version
echo "🔍 Checking system resources..."
MIN_RAM_GB=4

# Try to parse RAM from lc info output
# Assuming format like "Available RAM: 8.0 GB" or similar
AVAILABLE_RAM=$(lc info 2>/dev/null | grep -i "ram\|memory" | grep -i "available" | grep -oE '[0-9]+(\.[0-9]+)?' | head -1)

if [ -z "$AVAILABLE_RAM" ]; then
    echo "⚠️  Could not determine available RAM"
    echo "   Please ensure you have at least ${MIN_RAM_GB}GB RAM available"
else
    # Use awk for floating point comparison since bc might not be available
    if awk -v ram="$AVAILABLE_RAM" -v min="$MIN_RAM_GB" 'BEGIN {exit !(ram < min)}'; then
        echo "⚠️  Low RAM: ${AVAILABLE_RAM}GB available, ${MIN_RAM_GB}GB recommended"
        echo "   The application may run slowly"
    else
        echo "✅ RAM: ${AVAILABLE_RAM}GB available"
    fi
fi

if [ $FAILED -eq 1 ]; then
    echo ""
    echo "❌ Pre-setup checks failed. Please fix the issues above and try again."
    exit 1
fi

echo ""
echo "✅ All prerequisites met!"
exit 0