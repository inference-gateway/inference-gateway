#!/bin/bash

# Pre-commit script to ensure code quality
# This script should be run before making any commits

set -e

echo "🔍 Running pre-commit checks..."

# Check if workspace has unstaged changes (staged changes are fine since we're committing them)
if ! git diff --quiet; then
    echo "⚠️  Working directory has unstaged changes. Please stage or stash them first."
    exit 1
fi

echo "⚙️  Running code generation..."
task generate

if ! git diff --quiet; then
    echo "❌ Code generation produced changes that are not committed."
    echo "   Please run 'task generate' and commit the changes before proceeding."
    echo ""
    echo "Files that changed:"
    git diff --name-only
    exit 1
fi

echo "📝 Running Go linting..."
task lint

echo "🔧 Running OpenAPI linting..."
task openapi-lint

echo "🏗️  Building application..."
task build

echo "🧪 Running tests..."
task test

echo "✅ All pre-commit checks passed!"
echo "💡 You can now safely commit your changes."
