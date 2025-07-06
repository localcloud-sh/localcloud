#!/bin/bash

# LocalCloud Release Script
# This script creates a new release by tagging and pushing to GitHub
# GitHub Actions will automatically build binaries and create the release

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Helper functions
error() { echo -e "${RED}Error: $1${NC}" >&2; exit 1; }
success() { echo -e "${GREEN}✓ $1${NC}"; }
info() { echo -e "${BLUE}→ $1${NC}"; }
warning() { echo -e "${YELLOW}⚠ $1${NC}"; }

# Check if version argument is provided
if [ $# -eq 0 ]; then
    echo "Usage: $0 <version>"
    echo "Example: $0 1.0.0"
    echo ""
    echo "This will create and push tag 'v1.0.0' which triggers:"
    echo "• GitHub Actions build"
    echo "• Multi-platform binary creation"
    echo "• GitHub Release with changelog"
    echo "• Homebrew tap update"
    exit 1
fi

VERSION=$1
TAG="v$VERSION"

# Validate version format (basic semver check)
if ! [[ "$VERSION" =~ ^[0-9]+\.[0-9]+\.[0-9]+(-[a-zA-Z0-9]+)?$ ]]; then
    error "Invalid version format. Use semantic versioning (e.g., 1.0.0, 1.0.0-beta)"
fi

info "Preparing release $TAG"

# Check if we're in a git repository
if ! git rev-parse --git-dir > /dev/null 2>&1; then
    error "Not in a git repository"
fi

# Check if we're on main branch
CURRENT_BRANCH=$(git branch --show-current)
if [ "$CURRENT_BRANCH" != "main" ]; then
    warning "You're on branch '$CURRENT_BRANCH', not 'main'"
    read -p "Continue anyway? (y/N): " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        error "Aborted"
    fi
fi

# Check if working directory is clean
if [ -n "$(git status --porcelain)" ]; then
    warning "Working directory has uncommitted changes:"
    git status --short
    read -p "Continue anyway? (y/N): " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        error "Aborted"
    fi
fi

# Check if tag already exists
if git rev-parse "$TAG" >/dev/null 2>&1; then
    error "Tag $TAG already exists"
fi

# Get latest tag for reference
LATEST_TAG=$(git describe --tags --abbrev=0 2>/dev/null || echo "none")
info "Latest tag: $LATEST_TAG"

# Show what will be included in this release
echo ""
info "Changes since $LATEST_TAG:"
if [ "$LATEST_TAG" != "none" ]; then
    git log $LATEST_TAG..HEAD --oneline --no-merges | head -10
    if [ $(git log $LATEST_TAG..HEAD --oneline --no-merges | wc -l) -gt 10 ]; then
        echo "  ... and $(( $(git log $LATEST_TAG..HEAD --oneline --no-merges | wc -l) - 10 )) more commits"
    fi
else
    git log --oneline --no-merges | head -10
fi

echo ""
warning "This will:"
echo "  • Create and push tag: $TAG"
echo "  • Trigger GitHub Actions release workflow"
echo "  • Build binaries for all platforms"
echo "  • Create GitHub release with changelog"
echo "  • Update Homebrew formula"

# Final confirmation
read -p "Create release $TAG? (y/N): " -n 1 -r
echo
if [[ ! $REPLY =~ ^[Yy]$ ]]; then
    error "Aborted"
fi

# Create and push the tag
info "Creating tag $TAG..."
git tag -a "$TAG" -m "Release $TAG

## What's Changed
$(git log $LATEST_TAG..HEAD --pretty=format:"- %s" --no-merges)"

info "Pushing tag to GitHub..."
git push origin "$TAG"

success "Release $TAG created and pushed!"
echo ""
info "GitHub Actions will now:"
echo "  • Run tests"
echo "  • Build multi-platform binaries"  
echo "  • Create GitHub release"
echo "  • Update Homebrew tap"
echo ""
info "Track progress at: https://github.com/localcloud-sh/localcloud/actions"
info "Release will be available at: https://github.com/localcloud-sh/localcloud/releases/tag/$TAG"