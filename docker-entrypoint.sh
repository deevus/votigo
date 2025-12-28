#!/bin/bash
set -e

# Check required environment variable
if [ -z "$ADMIN_PASSWORD" ]; then
    echo "Error: ADMIN_PASSWORD environment variable is required"
    exit 1
fi

# Handle graceful shutdown
cleanup() {
    echo "Shutting down servers..."
    kill -TERM "$LEGACY_PID" 2>/dev/null || true
    wait "$LEGACY_PID" 2>/dev/null || true
    exit 0
}
trap cleanup SIGTERM SIGINT

# Start legacy server on port 8000 in background
echo "Starting legacy server on port 8000..."
./votigo serve --db /data/votigo.db --port 8000 --ui legacy --admin-password "$ADMIN_PASSWORD" &
LEGACY_PID=$!

# Start modern server on port 8001 in foreground
echo "Starting modern server on port 8001..."
./votigo serve --db /data/votigo.db --port 8001 --ui modern --admin-password "$ADMIN_PASSWORD" &
MODERN_PID=$!

# Wait for both processes
wait $LEGACY_PID $MODERN_PID
