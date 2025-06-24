#!/bin/bash

# LocalCloud Docker Cleanup Script
# This script removes all LocalCloud related containers and volumes

echo "🧹 LocalCloud Docker Cleanup"
echo "=========================="
echo ""

# List all containers that will be removed
echo "📦 Containers to remove:"
docker ps -a --filter "name=localcloud-" --filter "name=my-chat-app-" --format "table {{.Names}}\t{{.Status}}"
echo ""

# Confirm before proceeding
read -p "⚠️  This will remove all LocalCloud containers and volumes. Continue? [y/N] " -n 1 -r
echo ""
if [[ ! $REPLY =~ ^[Yy]$ ]]; then
    echo "❌ Cleanup cancelled"
    exit 1
fi

echo ""
echo "🛑 Stopping containers..."

# Stop all LocalCloud related containers
docker stop $(docker ps -q --filter "name=localcloud-") 2>/dev/null
docker stop $(docker ps -q --filter "name=my-chat-app-") 2>/dev/null

echo "🗑️  Removing containers..."

# Remove all LocalCloud related containers
docker rm $(docker ps -a -q --filter "name=localcloud-") 2>/dev/null
docker rm $(docker ps -a -q --filter "name=my-chat-app-") 2>/dev/null

echo "💾 Removing volumes..."

# Remove LocalCloud volumes (optional - commented out to preserve data)
# Uncomment the following lines if you want to remove volumes too:
# docker volume rm $(docker volume ls -q --filter "name=localcloud-") 2>/dev/null
# docker volume rm $(docker volume ls -q --filter "name=my-chat-app-") 2>/dev/null

echo ""
echo "✅ Cleanup complete!"
echo ""

# Show remaining containers
REMAINING=$(docker ps -a --filter "name=localcloud-" --filter "name=my-chat-app-" -q | wc -l)
if [ $REMAINING -eq 0 ]; then
    echo "✨ All LocalCloud containers have been removed"
else
    echo "⚠️  Some containers might still remain:"
    docker ps -a --filter "name=localcloud-" --filter "name=my-chat-app-" --format "table {{.Names}}\t{{.Status}}"
fi

echo ""
echo "💡 Tip: To also remove volumes and free up disk space, run:"
echo "   docker volume prune"
echo ""
echo "   To remove all unused Docker resources:"
echo "   docker system prune -a"