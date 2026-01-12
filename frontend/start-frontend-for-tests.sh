#!/bin/bash
# Wrapper script to start frontend for Playwright tests
# If port 5173 is already in use, exit successfully (server is already running)

if ss -ltn | grep -q ':5173 '; then
    echo "Frontend already running on port 5173"
    # Keep the script running so Playwright doesn't think the server died
    sleep infinity
    exit 0
fi

# Otherwise, start the frontend
npm run dev
