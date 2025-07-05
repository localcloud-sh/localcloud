#!/bin/bash

# LLM Progress Monitor
# Helper script to monitor LLM/Ollama container progress during model downloads

echo "🤖 LLM Progress Monitor"
echo "========================"
echo "This script monitors Ollama container logs to show download progress."
echo "Press Ctrl+C to stop monitoring."
echo ""

# Check if Ollama container is running
if ! docker ps --filter "name=localcloud-ollama" --filter "status=running" | grep -q ollama; then
    echo "❌ Ollama container not found or not running"
    echo "Make sure the LLM test is running first."
    exit 1
fi

echo "📊 Monitoring Ollama container logs (press Ctrl+C to stop)..."
echo "============================================================="

# Follow container logs with filtering for relevant progress information
docker logs -f localcloud-ollama 2>&1 | while read -r line; do
    # Filter and format relevant log lines
    if [[ "$line" =~ (pulling|downloading|verifying|writing|success) ]]; then
        timestamp=$(date "+%H:%M:%S")
        echo "[$timestamp] $line"
    elif [[ "$line" =~ (error|Error|ERROR|failed|Failed) ]]; then
        timestamp=$(date "+%H:%M:%S")
        echo "[$timestamp] ❌ $line"
    elif [[ "$line" =~ (loaded|ready|started|serving) ]]; then
        timestamp=$(date "+%H:%M:%S")
        echo "[$timestamp] ✅ $line"
    fi
done