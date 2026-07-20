#!/bin/bash
set -e

echo "=== FuuDelivery Admin Build Script ===" 
echo "Node version: $(node --version)"
echo "NPM version: $(npm --version)"
echo "Current dir: $(pwd)"
echo ""

echo "=== Step 1: npm install ==="
npm install --legacy-peer-deps 2>&1 || echo "npm install had warnings/errors"
echo "npm install exit code: $?"
echo ""

echo "=== Step 2: npm run build ==="
npm run build 2>&1 || echo "npm run build failed"
echo "npm run build exit code: $?"
echo ""

echo "=== Step 3: Check build output ==="
if [ -d "build" ]; then
  echo "build/ directory exists!"
  ls -la build/
elif [ -d "dist" ]; then
  echo "dist/ directory exists!"
  ls -la dist/
else
  echo "NO build output found!"
fi

echo "=== Build script complete ==="
