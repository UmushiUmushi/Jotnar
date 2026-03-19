# Setup Guide

## Requirements

- An NVIDIA GPU with 6GB+ VRAM (RTX 3060, 4060, etc.)
- Docker and Docker Compose
- An Android device for capture (desktop client is planned)

If you don't have an NVIDIA GPU, see [Alternative: Running without a bundled GPU](#alternative-running-without-a-bundled-gpu) below.

## Quick Start (GPU Setup)

1. Create a `docker-compose.yml` file with the following contents:

```yaml
services:
  jotnar:
    image: ghcr.io/UmushiUmushi/jotnar-server:latest
    ports:
      - "8910:8910"
    volumes:
      - jotnar-data:/data
    environment:
      - INFERENCE_HOST=${INFERENCE_HOST:-http://sglang:8000}
      - INFERENCE_TIMEOUT_SEC=${INFERENCE_TIMEOUT_SEC:-120}
      - INFERENCE_MAX_RETRIES=${INFERENCE_MAX_RETRIES:-3}
      - INFERENCE_WORKERS=${INFERENCE_WORKERS:-4}
      - JOTNAR_DB_PATH=${JOTNAR_DB_PATH:-/data/journal.db}
      - JOTNAR_CONFIG_PATH=${JOTNAR_CONFIG_PATH:-/data/config.yml}
    depends_on:
      - sglang
    restart: unless-stopped

  sglang:
    image: lmsysorg/sglang:latest
    ports:
      - "8000:8000"
    volumes:
      - model-cache:/root/.cache/huggingface
    command: >
      python -m sglang.launch_server
      --model-path Qwen/Qwen3.5-4B
      --port 8000
      --context-length 32768
      --reasoning-parser qwen3
    deploy:
      resources:
        reservations:
          devices:
            - capabilities: [gpu]
    restart: unless-stopped

volumes:
  jotnar-data:
  model-cache:
```

2. Start the containers:

```bash
docker compose up -d
```

This starts two containers:
- **jotnar** — the Jotnar server on port `8910`
- **sglang** — the AI vision model (Qwen/Qwen3.5-4B) on port `8000`

On first launch, SGLang will download the model (~2.5GB). This only happens once — the model is cached in a Docker volume.

3. Watch the logs for first-time setup output:

```bash
docker compose logs -f jotnar
```

You'll see something like:

```
============================================
  FIRST-TIME SETUP
  Pairing code: ABC123
  (expires in 10 minutes)
  Recovery key: xxxx-xxxx-xxxx-xxxx
  SAVE THIS KEY — you cannot retrieve it later!
============================================
```

Write down the recovery key somewhere safe. You'll need it if you lose access to all your paired devices.

4. Install the Jotnar Android app on your phone.

5. Open the app, enter your server address (e.g. `http://192.168.1.100:8910` or `http://100.x.x.x:8910` if using Tailscale) and the pairing code from step 3.

6. The app will authenticate via biometric/PIN, pair with the server, and start capturing. You're done.

## Alternative: Running Without a Bundled GPU

If you're running your own OpenAI-compatible inference backend (Ollama, vLLM, etc.) separately, use this compose file instead:

```yaml
services:
  jotnar:
    image: ghcr.io/UmushiUmushi/jotnar-server:latest
    ports:
      - "8910:8910"
    volumes:
      - jotnar-data:/data
    environment:
      - INFERENCE_HOST=${INFERENCE_HOST:-http://host.docker.internal:11434}
      - INFERENCE_TIMEOUT_SEC=${INFERENCE_TIMEOUT_SEC:-300}
      - INFERENCE_MAX_RETRIES=${INFERENCE_MAX_RETRIES:-3}
      - INFERENCE_WORKERS=${INFERENCE_WORKERS:-1}
      - JOTNAR_DB_PATH=${JOTNAR_DB_PATH:-/data/journal.db}
      - JOTNAR_CONFIG_PATH=${JOTNAR_CONFIG_PATH:-/data/config.yml}
    restart: unless-stopped

volumes:
  jotnar-data:
```

Set `INFERENCE_HOST` to wherever your backend is running. The default points to `http://host.docker.internal:11434` (Ollama on the host machine).

```bash
docker compose up -d
```

### Notes on Ollama

- Ollama in Docker on macOS **cannot access Metal** (Apple GPU) and will run on CPU. Vision inference on CPU takes minutes per image and will likely time out. Run Ollama natively on macOS to get Metal acceleration.
- With Ollama, set `INFERENCE_WORKERS=1` (the default for server-only). Ollama processes one request at a time, so more workers just creates contention.
- Expect ~30–60s per vision request with Ollama on Metal vs ~2–5s on an NVIDIA GPU with SGLang.

## Environment Variables

The only file you need is `docker-compose.yml` — it includes sensible defaults for all settings and works out of the box. A `.env` file is entirely optional; use one only if you want to override the defaults without editing the compose file directly.

The compose files use `${VAR:-default}` syntax, so the defaults shown below apply if unset:

| Variable | Default (full) | Default (server-only) | Description |
|----------|---------------|----------------------|-------------|
| `INFERENCE_HOST` | `http://sglang:8000` | `http://host.docker.internal:11434` | Inference backend URL |
| `INFERENCE_TIMEOUT_SEC` | `120` | `300` | HTTP timeout (seconds) for inference requests |
| `INFERENCE_MAX_RETRIES` | `3` | `3` | Retry attempts for transient inference failures |
| `INFERENCE_WORKERS` | `4` | `1` | Concurrent interpretation workers. With SGLang, higher values enable GPU batching. With Ollama, keep at 1. |
| `JOTNAR_DB_PATH` | `/data/journal.db` | `/data/journal.db` | SQLite database path |
| `JOTNAR_CONFIG_PATH` | `/data/config.yml` | `/data/config.yml` | Server config file path |

If you do want to customize, copy the `.env.example` next to your compose file and edit as needed:

```bash
cp .env.example .env
```

Docker Compose automatically reads `.env` from the same directory. You can also set variables directly in your shell or in the compose file itself — whatever you prefer.

## Pairing Additional Devices

Once your first device is paired:

1. On the already-paired device, go to Settings → "Pair new device" (requires biometric/PIN).
2. The app will show a one-time pairing code.
3. On the new device, enter the server address and the code.

## Recovering Access

If you lose access to all paired devices:

**Option A — Recovery key:**
Install the app on a new device, tap "Recover access", and enter the recovery key from first-time setup.

**Option B — Server access:**
SSH into the server and generate a new pairing code:

```bash
docker exec jotnar pairingcode
```

Then pair the new device with that code.

## Changing the AI Model

Jotnar is model-agnostic — it talks to the inference backend via the OpenAI-compatible API and doesn't care which model is running. To change the model, edit the `--model-path` in `docker-compose.yml`:

```yaml
command: >
  python -m sglang.launch_server
  --model-path Your/Preferred-Model
  --port 8000
  --context-length 32768
```

Then restart SGLang and tell Jotnar to reconnect:

```bash
docker compose up -d sglang
docker exec jotnar updateinference
```

Jotnar will pause its workers while SGLang downloads and loads the new model, then automatically resume once the model is serving.

## Server Configuration

Once paired, you can view and change server settings from the app's settings screen, or via the API:

```bash
# View current config
curl -H "Authorization: Bearer <token>" http://localhost:8910/config

# Update config
curl -X PUT -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{"consolidation_window_min": 60, "journal_tone": "narrative"}' \
  http://localhost:8910/config
```

See the [CLAUDE.md](../CLAUDE.md) configuration reference for all available settings.

## Updating Jotnar

```bash
docker compose pull jotnar
docker compose up -d jotnar
```

Any screenshots queued for processing are persisted to the database during shutdown and restored automatically when the new container starts. Nothing is lost during the update.

The Android app will check the server version via `GET /status` and nudge you to update if behind.

## Changing Inference Settings at Runtime

You can change inference settings without restarting the Jotnar container or losing your processing queue. This is useful for:

- Switching inference backends (e.g. SGLang → Ollama, or pointing to a different host)
- Changing the number of concurrent workers
- Adjusting the inference timeout or retry count
- Waiting for a model to reload after changing SGLang's `--model-path`

There are two ways to use `updateinference`: via CLI flags or by reloading from the environment.

### Option A: CLI flags (change specific settings)

Pass only the settings you want to change. Everything else stays as-is on the running server.

```bash
docker exec jotnar updateinference --host=http://newhost:8000
```

Available flags:

| Flag | Description |
|------|-------------|
| `--host` | Inference server URL |
| `--workers` | Concurrent interpretation workers |
| `--timeout` | Inference timeout in seconds |
| `--retries` | Max retry attempts for transient failures |

You can combine flags — only the specified settings change:

```bash
# Change host and workers, keep timeout and retries as-is
docker exec jotnar updateinference --host=http://newhost:8000 --workers=2
```

### Option B: Reload from environment

Run `updateinference` with no flags to reload all settings from the `.env` file (or Docker Compose environment):

```bash
# 1. Edit .env
# 2. Apply all changes at once
docker exec jotnar updateinference
```

### What happens during updateinference

Regardless of which option you use, `updateinference` will:

1. Pause all workers (in-flight interpretations finish first, buffered jobs are kept)
2. Apply the new settings and create a new inference client
3. Wait for the inference server to become healthy (auto-detects Ollama vs OpenAI-compatible)
4. Resume workers with the new configuration

If the inference server isn't available yet (e.g. SGLang is still loading a model), Jotnar will poll every 3 seconds until it responds, then resume automatically. Times out after 10 minutes.

### Examples

**Switch from SGLang to Ollama on the host:**

```bash
docker exec jotnar updateinference --host=http://host.docker.internal:11434 --workers=1
```

**SGLang model swap (same host, new model):**

```bash
# Edit docker-compose.yml to change --model-path, then:
docker compose up -d sglang
docker exec jotnar updateinference
# Jotnar waits for SGLang to finish loading, then resumes.
```

**Just change the worker count:**

```bash
docker exec jotnar updateinference --workers=8
```
