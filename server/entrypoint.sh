#!/bin/bash

# Start Jotnar (single binary runs API server + background worker).
# Jotnar handles waiting for the inference backend internally.
exec /usr/local/bin/jotnar
