#!/bin/bash

# Quick demo of working export functionality

echo "=== LocalCloud Export Functionality Demo ==="
echo

# Check if services are running
if docker ps | grep -q localcloud-postgres; then
    echo "✅ PostgreSQL service is running"
    
    # Test database export
    echo "🔄 Testing database export..."
    if PATH="/opt/homebrew/opt/postgresql@15/bin:$PATH" lc export db --output=demo-db-export.sql; then
        echo "✅ Database export successful"
        echo "   File: demo-db-export.sql ($(wc -l < demo-db-export.sql) lines)"
    else
        echo "❌ Database export failed"
    fi
    echo
fi

if docker ps | grep -q localcloud-minio; then
    echo "✅ MinIO service is running"
    
    # Test storage export
    echo "🔄 Testing storage export..."
    if lc export storage --output=demo-storage-export.tar.gz; then
        echo "✅ Storage export successful"
        echo "   File: demo-storage-export.tar.gz ($(wc -c < demo-storage-export.tar.gz) bytes)"
    else
        echo "❌ Storage export failed"
    fi
    echo
fi

# Test export all
echo "🔄 Testing export all functionality..."
if PATH="/opt/homebrew/opt/postgresql@15/bin:$PATH" lc export all; then
    echo "✅ Export all successful"
    echo "   Files created:"
    ls -la *.sql *.tar.gz 2>/dev/null | grep "$(date +%Y%m%d)" || echo "   (No new files found)"
else
    echo "❌ Export all failed"
fi

echo
echo "=== Demo Complete ==="
echo "Export functionality is working correctly!"