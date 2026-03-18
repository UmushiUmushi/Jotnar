# Jotnar API Reference

Base URL: `http://localhost:8910`

## Authentication

All endpoints except `/status`, `/auth/pair`, and `/auth/recover` require a device token:

```
Authorization: Bearer <device_token>
```

Tokens are obtained during device pairing and do not expire. Store them securely (Android Keystore).

## Error Format

All errors return JSON:

```json
{ "error": "description of what went wrong" }
```

---

## Health

### GET /status

Server health check. No authentication required.

**Response** `200`

```json
{
  "version": "0.1.0",
  "model_available": true,
  "device_count": 2
}
```

---

## Authentication

### POST /auth/pair

Pair a new device using a one-time code.

**Request**

```json
{
  "code": "A3K7W2",
  "device_name": "Sharon's Pixel 8"
}
```

**Response** `200`

```json
{
  "device_id": "550e8400-e29b-41d4-a716-446655440000",
  "token": "a1b2c3d4e5f6..."
}
```

**Errors:** `400` invalid body, `401` invalid or expired code.

### POST /auth/pair/new

Generate a new pairing code for another device. Requires authentication.

**Response** `200`

```json
{
  "code": "B4M8X9"
}
```

### POST /auth/recover

Recover access using the recovery key printed at first setup.

**Request**

```json
{
  "recovery_key": "a1b2c3d4..."
}
```

**Response** `200`

```json
{
  "pairing_code": "C5N9Y1"
}
```

**Errors:** `401` invalid recovery key.

---

## Capture

### POST /capture

Upload a single screenshot for interpretation. The screenshot is interpreted by the AI model and deleted immediately.

**Request:** `multipart/form-data`

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `screenshot` | file | yes | PNG or JPEG, max 10 MB |
| `captured_at` | string | no | RFC 3339 timestamp. Defaults to server time. |

**Response** `200`

```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "interpretation": "Chatting with Alex on Discord about weekend plans",
  "category": "communication",
  "app_name": "Discord"
}
```

**Errors:** `400` missing file or invalid format, `413` exceeds 10 MB, `500` interpretation failed.

### POST /capture/batch

Upload up to 50 screenshots in a single request. Processed concurrently (4 at a time).

**Request:** `multipart/form-data`

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `screenshots` | file[] | yes | PNG or JPEG files, max 10 MB each, max 50 |
| `captured_at` | string[] | no | RFC 3339 timestamps, index-matched to files |

**Response** `200` (all succeeded), `207` (partial), `500` (all failed)

```json
{
  "results": [
    {
      "index": 0,
      "id": "...",
      "interpretation": "Playing Genshin Impact",
      "category": "gaming",
      "app_name": "Genshin Impact"
    },
    {
      "index": 1,
      "error": "invalid image format, expected PNG or JPEG"
    }
  ],
  "succeeded": 1,
  "failed": 1
}
```

---

## Journal

### GET /journal

List journal entries, newest first.

**Query Parameters**

| Param | Type | Default | Description |
|-------|------|---------|-------------|
| `limit` | int | 20 | Max entries to return (max 100) |
| `offset` | int | 0 | Entries to skip |

**Response** `200`

```json
{
  "entries": [
    {
      "id": "...",
      "narrative": "Spent the evening chatting on Discord...",
      "time_start": "2024-06-15T20:00:00Z",
      "time_end": "2024-06-15T20:30:00Z",
      "edited": false,
      "created_at": "2024-06-15T20:31:00Z",
      "updated_at": null
    }
  ],
  "total": 42,
  "limit": 20,
  "offset": 0
}
```

### GET /journal/{id}

Get a single journal entry.

**Response** `200` — same shape as entries in the list.

**Errors:** `404` entry not found.

### PUT /journal/{id}

Edit a journal entry's narrative. Marks the entry as `edited: true`.

**Request**

```json
{
  "narrative": "Updated journal text"
}
```

**Response** `200` — updated entry.

**Errors:** `400` missing narrative, `404` entry not found.

### DELETE /journal/{id}

Delete a journal entry. Linked metadata rows are unlinked (not deleted).

**Response** `200`

```json
{
  "deleted": true
}
```

---

## Metadata & Reconsolidation

### GET /journal/{id}/metadata

Get all metadata rows linked to a journal entry.

**Response** `200`

```json
{
  "metadata": [
    {
      "id": "...",
      "device_id": "...",
      "captured_at": "2024-06-15T20:05:00Z",
      "interpretation": "Browsing r/golang",
      "category": "browsing",
      "app_name": "Reddit",
      "created_at": "2024-06-15T20:05:01Z"
    }
  ]
}
```

### POST /journal/{id}/preview

Preview what a journal entry would look like with a subset of metadata. Does NOT save anything. Use this with debouncing (~1.5s after last toggle).

**Request**

```json
{
  "include_metadata_ids": ["uuid1", "uuid3", "uuid4"]
}
```

**Response** `200`

```json
{
  "narrative": "Preview of the reconsolidated entry..."
}
```

### POST /journal/{id}/reconsolidate

Commit a reconsolidation: deletes excluded metadata permanently, rewrites the entry narrative, marks the entry as edited, and updates the time range.

**Request**

```json
{
  "include_metadata_ids": ["uuid1", "uuid3", "uuid4"],
  "narrative": "Optional pre-generated narrative"
}
```

If `narrative` is empty or omitted, the model generates a new one from the included metadata.

**Response** `200` — updated journal entry.

---

## Configuration

### GET /config

Get the current server configuration.

**Response** `200`

```json
{
  "consolidation_window_min": 30,
  "interpretation_detail": "standard",
  "journal_tone": "casual",
  "metadata_retention_days": null
}
```

### PUT /config

Update server configuration. All fields are optional — only provided fields are changed.

**Request**

```json
{
  "consolidation_window_min": 60,
  "journal_tone": "concise"
}
```

**Response** `200` — full updated config.

**Config values:**

| Field | Type | Values | Default |
|-------|------|--------|---------|
| `consolidation_window_min` | int | >= 1 | 30 |
| `interpretation_detail` | string | minimal, standard, detailed | standard |
| `journal_tone` | string | casual, concise, narrative | casual |
| `metadata_retention_days` | int\|null | >= 1 or null | null |

---

## Status Codes

| Code | Meaning |
|------|---------|
| 200 | Success |
| 207 | Partial success (batch capture) |
| 400 | Invalid request |
| 401 | Unauthorized (missing or invalid token) |
| 404 | Resource not found |
| 413 | Payload too large |
| 500 | Server error |
