#!/bin/bash

# Wait for SGLang to be ready (OpenAI-compatible endpoint)
MAX_RETRIES=150
RETRY_INTERVAL=2
echo "Waiting for SGLang (timeout: $((MAX_RETRIES * RETRY_INTERVAL))s)..."
retries=0
until curl -s http://sglang:8000/v1/models > /dev/null 2>&1; do
  retries=$((retries + 1))
  if [ "$retries" -ge "$MAX_RETRIES" ]; then
    echo "ERROR: SGLang did not become ready after $((MAX_RETRIES * RETRY_INTERVAL))s. Exiting."
    exit 1
  fi
  sleep $RETRY_INTERVAL
done
echo "SGLang is ready."

# Start Jotnar (single binary runs API server + background worker).
# First-time pairing setup is handled in main.go by checking the database.
exec /usr/local/bin/jotnar
