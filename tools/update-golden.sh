#!/bin/bash
set -e

echo "Updating golden test files..."

# List of packages that have golden file tests
GOLDEN_PACKAGES=(
    "./pkg/generator"
)

# Update golden files for packages that support it
for package in "${GOLDEN_PACKAGES[@]}"; do
    echo "Updating golden files for $package..."
    go test "$package" -update-golden -v
done

echo "Golden files updated successfully!"