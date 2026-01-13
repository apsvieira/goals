#!/bin/bash
# Wrapper script to start backend for Playwright tests
# If port 8080 is already in use, exit successfully (server is already running)

# Enable CORS for Vite dev server origin to allow sync requests from localhost:5173
export CORS_ORIGINS="http://localhost:5173"

if ss -ltn | grep -q ':8080 '; then
    echo "Backend already running on port 8080"
    # Keep the script running so Playwright doesn't think the server died
    sleep infinity
    exit 0
fi

# Otherwise, start the backend
cd ../backend && go run ./cmd/server
