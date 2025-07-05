#!/bin/bash

# Emergency cleanup script for stuck LocalCloud tests

echo "=== Emergency LocalCloud Cleanup ==="

# Stop any running LocalCloud processes
echo "1. Stopping LocalCloud services..."
lc stop &>/dev/null || true

# Stop and remove all LocalCloud containers
echo "2. Cleaning up Docker containers..."
docker ps -a --filter "name=localcloud-" --format "{{.Names}}" | xargs -r docker stop 2>/dev/null || true
docker ps -a --filter "name=localcloud-" --format "{{.Names}}" | xargs -r docker rm 2>/dev/null || true

# Remove LocalCloud networks
echo "3. Cleaning up Docker networks..."
docker network ls --filter "name=localcloud_" --format "{{.Name}}" | xargs -r docker network rm 2>/dev/null || true

# Clean up any orphaned volumes (optional - removes downloaded models)
echo "4. Cleaning up Docker volumes..."
docker volume ls --filter "name=localcloud_" --format "{{.Name}}" | xargs -r docker volume rm 2>/dev/null || true

# Remove test project if it exists
echo "5. Cleaning up test project..."
rm -rf .localcloud 2>/dev/null || true

# Show current Docker state
echo "6. Current Docker state:"
docker ps -a | grep localcloud || echo "No LocalCloud containers found"

echo "=== Cleanup Complete ==="
echo ""
echo "You can now run tests again. For embedding tests, use:"
echo "./test-runner.sh --components embedding --timeout 1200 --verbose"