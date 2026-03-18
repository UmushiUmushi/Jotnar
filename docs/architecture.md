# Architecture

## Overview

Jotnar has three components:

1. **Android app** (Kotlin) — captures screenshots via MediaProjection, uploads them to the server, displays the journal, and manages device-side settings.
2. **Jotnar server** (Go) — a single binary running in Docker. Handles the REST API, authentication, screenshot processing, consolidation, and the SQLite database. Listens on port 8910.
3. **SGLang** (separate Docker container) — serves the AI vision model with an OpenAI-compatible API on port 8000. Chosen for its RadixAttention prefix caching, which benefits the repeated system prompt pattern.

```
┌────────────────────────────┐
│     Android / Desktop      │
│   (capture + journal UI)   │
└─────────────┬──────────────┘
              │ HTTPS + Bearer token
┌─────────────▼──────────────┐
│        Jotnar Server       │
│                            │
│  API layer (routing, auth) │
│            │               │
│  Processing layer          │
│  (interpret, consolidate)  │
│            │               │
│  Store (SQLite)            │
└─────────────┬──────────────┘
              │ OpenAI-compatible API
┌─────────────▼──────────────┐
│    SGLang (Qwen3.5-4B)     │
└────────────────────────────┘
```

## Server Layers

The server has a strict layered architecture:

- **API** (`internal/api/`) — HTTP routing, request validation, auth middleware. No business logic.
- **Auth** (`internal/auth/`) — Device pairing, token management, recovery keys.
- **Processing** (`internal/processing/`) — Interpreter (screenshot → metadata), consolidator (metadata → journal entry), reconsolidator (edit previews and commits).
- **Config** (`internal/config/`) — Reads/writes `/data/config.yml`, applies defaults. Safe for concurrent reads.
- **Store** (`internal/store/`) — SQLite connection, schema, models, and all CRUD. Single package for data access.
- **Inference** (`internal/inference/`) — Thin wrapper around the OpenAI-compatible API. Builds prompts, sends requests, parses responses. Model-agnostic.

**Data flow**: API → Auth/Processing/Config → Inference/Store

## Two-Stage Inference

### Stage 1 — Interpretation (per screenshot)

The `/capture` and `/capture/batch` endpoints return 202 immediately after queuing. A background worker pool (`INFERENCE_WORKERS` goroutines) processes screenshots concurrently:

1. Vision model receives the image + device/timestamp metadata.
2. Returns JSON with `interpretation` (natural language), `category` (gaming, social, browsing, etc.), and `app_name`.
3. Result is stored as a metadata row. The screenshot is deleted immediately.

With SGLang, concurrent workers enable GPU-level batching — SGLang packs multiple requests into a single GPU kernel call transparently.

### Stage 2 — Consolidation (periodic)

A background goroutine runs on a timer (default every 30 minutes):

1. Collects unprocessed metadata rows from the configured time window.
2. Sends them as text to the model for consolidation.
3. Stores the resulting narrative as a journal entry and links the metadata rows.

The consolidator tracks progress by timestamp, not batch count. Changing `consolidation_window_min` mid-operation never skips or double-processes data.

## Reconsolidation

Users can edit journal entries by toggling individual metadata rows on/off:

1. **Preview** (`POST /journal/{id}/preview`) — returns what the entry would look like with a given metadata subset. Does not save.
2. **Commit** (`POST /journal/{id}/reconsolidate`) — deletes excluded metadata, rewrites the entry narrative, marks it as edited, and updates the time range.

The Android app debounces preview requests (~1.5s after last toggle) since each preview triggers a model inference call.

## Authentication

- Token-based device pairing. No sessions, no expiry, no refresh.
- First device pairs via a one-time code printed to container logs at first startup.
- Subsequent devices pair via codes generated from an already-authenticated device.
- Recovery via a key generated at first setup, or via SSH access to the server.
- Biometric gates are client-side only (Android BiometricPrompt). The server just sees a normal authenticated request.

## Database

SQLite, stored at `/data/journal.db`. Five tables:

| Table | Purpose |
|-------|---------|
| `devices` | Paired devices with hashed tokens |
| `metadata` | Individual screenshot interpretations, linked to entries via `entry_id` |
| `journal_entries` | Consolidated narrative entries with time ranges |
| `recovery` | Single-row table for the hashed recovery key |
| `pairing_codes` | Temporary one-time codes with expiration |

`metadata.entry_id` is nullable — rows exist with `NULL` until the consolidator processes them.

## Deployment

Two Docker Compose configurations:

- **`docker-compose.yml`** — Full setup: Jotnar server + SGLang. Requires NVIDIA GPU with CUDA.
- **`docker-compose.server-only.yml`** — Jotnar server only. For users running their own OpenAI-compatible backend (Ollama, vLLM, etc.) separately.

The server compiles to a single binary. No process managers — the API server, capture queue, and consolidation worker all run as goroutines in the same process.
