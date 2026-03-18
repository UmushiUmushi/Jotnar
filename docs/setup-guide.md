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
      - INFERENCE_TIMEOUT_SEC=120
      - INFERENCE_MAX_RETRIES=3
      - INFERENCE_WORKERS=${INFERENCE_WORKERS:-4}
      - JOTNAR_DB_PATH=/data/journal.db
      - JOTNAR_CONFIG_PATH=/data/config.yml
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
      - INFERENCE_TIMEOUT_SEC=300
      - INFERENCE_MAX_RETRIES=3
      - INFERENCE_WORKERS=${INFERENCE_WORKERS:-1}
      - JOTNAR_DB_PATH=/data/journal.db
      - JOTNAR_CONFIG_PATH=/data/config.yml
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

| Variable | Default | Description |
|----------|---------|-------------|
| `INFERENCE_HOST` | `http://sglang:8000` (full) / `http://host.docker.internal:11434` (server-only) | Inference backend URL |
| `INFERENCE_TIMEOUT_SEC` | `120` (full) / `300` (server-only) | HTTP timeout for inference requests |
| `INFERENCE_MAX_RETRIES` | `3` | Retry attempts for transient inference failures |
| `INFERENCE_WORKERS` | `4` (full) / `1` (server-only) | Concurrent interpretation workers. With SGLang, higher values enable GPU batching. With Ollama, keep at 1. |
| `JOTNAR_DB_PATH` | `/data/journal.db` | SQLite database path |
| `JOTNAR_CONFIG_PATH` | `/data/config.yml` | Server config file path |

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
docker compose exec jotnar jotnar pairingcode
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

Then restart: `docker compose up -d`.

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

## Updating

```bash
cd server
docker compose pull
docker compose up -d
```

The Android app will check the server version via `GET /status` and nudge you to update if behind.
