# CLAUDE.md — Jotnar

## What is Jotnar?

Jotnar is an automatic journaling system for people who are chronically online but have terrible memory. It captures periodic screenshots from a user's devices, sends them to a locally-hosted AI vision model for interpretation, and consolidates those interpretations into natural-language journal entries. All data stays on the user's own hardware — nothing is sent to the cloud.

## Core Philosophy

- **Privacy-first**: All processing happens on the user's self-hosted server. Screenshots are deleted immediately after interpretation. No cloud, no telemetry, no external calls.
- **Anti-surveillance by design**: The app must be useless as a tool for monitoring someone else. Biometric gates on sensitive actions, persistent capture notifications, weekly reminders, and journal access limited to authenticated devices.
- **Zero-effort journaling**: The user configures their rules once and forgets about it. The system captures, interprets, consolidates, and journals automatically.

## Architecture

Jotnar has three deployable components:

1. **Android app** (Kotlin) — captures screenshots via MediaProjection, uploads them to the server, displays the journal, and manages settings. A desktop client is planned but not yet implemented.
2. **Jotnar server** (Go) — a single binary running inside Docker. Handles the API, authentication, screenshot processing, consolidation, and database. Exposes a REST API on port 8910.
3. **SGLang** (separate Docker container) — serves the AI model (default: Qwen/Qwen3.5-4B) with an OpenAI-compatible API on port 8000. Chosen over Ollama/vLLM for its RadixAttention prefix caching, which benefits the repeated system prompt pattern.

## Monorepo Structure

```
jotnar/
├── apps/
│   ├── android/          # Kotlin Android app
│   └── desktop/          # Future desktop capture client
├── server/
│   ├── cmd/jotnar/       # Go entry point
│   ├── internal/
│   │   ├── api/          # HTTP routes, middleware
│   │   ├── auth/         # Device pairing, token management, recovery keys
│   │   ├── processing/   # Interpreter, consolidator, reconsolidator
│   │   ├── config/       # Server-side config manager
│   │   ├── store/        # SQLite connection, models, CRUD for all tables
│   │   └── inference/    # OpenAI-compatible client for SGLang
│   ├── Dockerfile
│   └── docker-compose.yml
├── shared/               # OpenAPI spec, shared constants
└── docs/
```

## Server Layers

The server has a strict layered architecture. Respect the boundaries:

- **API layer** (`internal/api/`) — HTTP routing, request validation, auth middleware. Does not contain business logic. Request/response structs are defined in the route handler files.
- **Auth** (`internal/auth/`) — Device pairing, token management, recovery keys.
- **Processing** (`internal/processing/`) — Interpreter (Stage 1: screenshot → metadata), consolidator (Stage 2: metadata batch → journal entry), reconsolidator (preview + commit edits).
- **Config** (`internal/config/`) — Reads/writes `/data/config.yml`, applies defaults.
- **Store** (`internal/store/`) — SQLite connection, schema, models, and all CRUD operations for journal entries, metadata rows, and devices. Single package for data access.
- **Inference layer** (`internal/inference/`) — Thin wrapper around the SGLang OpenAI-compatible API. Builds prompts, sends requests, parses responses into standardized structs. Does not know about journals or screenshots — only prompts and completions.

**Data flow**: API → Auth/Processing/Config → Inference/Store.

## Two-Stage Inference

Screenshots go through two inference passes:

1. **Interpretation** (per screenshot): Vision model receives the image + device/timestamp metadata. Returns a JSON object with `interpretation` (natural language), `category` (gaming, social, browsing, etc.), and `app_name`. The screenshot is deleted immediately after.
2. **Consolidation** (periodic): Core collects metadata rows from the configured time window (default 30 min) and sends them as text to the model. Returns a narrative journal entry. The consolidator tracks the last processed timestamp so changing the window size never skips or double-processes data.

## Reconsolidation

Users can edit journal entries by toggling individual metadata rows on/off:
- `POST /journal/{id}/preview` — returns what the entry would look like with the given metadata subset. Does NOT save.
- `POST /journal/{id}/reconsolidate` — commits the change: deletes excluded metadata, rewrites the entry, marks it as edited.

The Android app should debounce preview requests (~1.5s after last toggle) since each preview triggers a model inference call.

## Authentication

- Token-based device pairing. No sessions, no expiry, no refresh.
- First device pairs via a one-time code printed to container logs.
- Subsequent devices pair via codes generated from an already-authenticated device.
- Recovery via a key generated at first setup, or via SSH access to the server.
- The server has no concept of biometrics. The Android app gates sensitive actions (pairing, revoking, config changes, metadata deletion) behind BiometricPrompt. The server just sees a normal authenticated request.

## Configuration

Two categories, stored in different places:

**Server-side** (`/data/config.yml`, managed via `GET/PUT /config`):
- `consolidation_window_min` (default: 30) — minutes of metadata per journal entry
- `interpretation_detail` (default: "standard") — minimal / standard / detailed
- `journal_tone` (default: "casual") — casual / concise / narrative
- `metadata_retention_days` (default: null) — auto-delete old metadata, null = forever

**Device-side** (stored locally on each Android device, never synced):
- Capture interval, battery saver pause, Wi-Fi only upload
- App blocklist with finance/health/auth categories blocked by default
- Persistent capture notification, weekly capture reminder
- Server address, upload batch size, retry interval

## Database Schema

```sql
devices (id, name, paired_at, token_hash, last_seen)
metadata (id, device_id, captured_at, interpretation, category, entry_id, created_at)
journal_entries (id, narrative, time_start, time_end, edited, created_at, updated_at)
recovery (id CHECK(id=1), key_hash)
```

The `metadata.entry_id` is nullable — rows exist with `entry_id = NULL` until the consolidator processes them and links them to a journal entry.

## API Endpoints

| Method | Path | Description |
|--------|------|-------------|
| POST | /capture | Receive screenshot from device |
| GET | /journal | List journal entries |
| GET | /journal/{id} | Get single entry |
| PUT | /journal/{id} | Edit entry narrative |
| DELETE | /journal/{id} | Delete entry + linked metadata |
| GET | /journal/{id}/metadata | Get metadata rows for an entry |
| POST | /journal/{id}/preview | Preview reconsolidation (no save) |
| POST | /journal/{id}/reconsolidate | Commit reconsolidation |
| GET | /config | Get server config |
| PUT | /config | Update server config |
| POST | /auth/pair | Pair device with one-time code |
| POST | /auth/pair/new | Generate new pairing code (requires auth) |
| POST | /auth/recover | Recover access with recovery key |
| GET | /status | Server health, model status, version |

All endpoints except `/auth/pair`, `/auth/recover`, and `/status` require a valid device token in the `Authorization: Bearer` header.

## Tech Stack

| Component | Technology |
|-----------|------------|
| Server | Go (single binary) |
| Android app | Kotlin |
| Database | SQLite |
| Inference engine | SGLang (OpenAI-compatible API) |
| Default model | Qwen/Qwen3.5-4B (multimodal, Apache 2.0) |
| Containerization | Docker Compose (jotnar + sglang) |
| API contract | OpenAPI YAML in `shared/api-spec.yml` |

## Development Guidelines

- The server compiles to a single binary. No supervisord, no process managers. The API server and background worker run as goroutines.
- The inference layer communicates via the OpenAI `/v1/chat/completions` endpoint. If someone swaps SGLang for vLLM or any other compatible server, it should work without code changes. The model is configured in SGLang's `--model-path` flag in the compose file, not in Jotnar itself — Jotnar is model-agnostic.
- Screenshots are ephemeral. They exist only in transit and during processing. Never write them to the database or persist them to disk beyond the processing queue.
- Config changes via the API must be immediately reflected in processing behavior. The config manager should be safe for concurrent reads.
- All auth token storage on Android must use the encrypted keystore, never SharedPreferences or plaintext.
- The consolidator must be idempotent and resumable. It tracks progress by timestamp, not by batch count. Changing `consolidation_window_min` mid-operation must not skip or duplicate entries.
- Tests live next to the code they test (e.g. `internal/processing/consolidator_test.go`), following standard Go conventions.

## Release

- **Android**: Git tag → GitHub Actions → signed APK → Google Play via Fastlane
- **Server**: Git tag → GitHub Actions → Docker image → `ghcr.io/yourname/jotnar-server:latest` + version tag. Users update with `docker compose pull && docker compose up -d`.
- The app checks the server version via `GET /status` and nudges the user to update if behind.
