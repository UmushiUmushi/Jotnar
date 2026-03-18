# Privacy

Jotnar is designed around a single principle: **your data never leaves your hardware**.

## Self-Hosted, No Cloud

Every component runs on your own machine:

- The Jotnar server and AI model run in Docker containers on your hardware.
- Screenshots are sent from your device directly to your server over your local network.
- No telemetry, no analytics, no external API calls. The server makes zero outbound network requests.

There is no Jotnar cloud service. There is no account to create. The only network traffic is between your devices and your server.

## Screenshots Are Ephemeral

Screenshots are the most sensitive data in the system. Jotnar treats them accordingly:

- Screenshots exist only in transit (device → server) and briefly in memory during processing.
- Once the AI model interprets a screenshot, it is deleted immediately. It is never written to disk on the server, never stored in the database.
- What remains is a text interpretation — e.g. "Chatting with Alex on Discord about weekend plans" — not the image itself.

## What Is Stored

The server stores three types of data, all in a local SQLite file:

| Data | Retention | Contains |
|------|-----------|----------|
| **Journal entries** | Permanent | Narrative text summarizing a time period |
| **Metadata** | Configurable (`metadata_retention_days`) | Text interpretations, category, app name, timestamps |
| **Device records** | Until revoked | Device name, hashed token, last seen timestamp |

No images, no raw screen content, no passwords, no credentials.

## Anti-Surveillance by Design

Jotnar is built to be useless as a tool for monitoring someone else:

- **Biometric gates**: Pairing a device, revoking access, changing settings, and viewing metadata all require biometric authentication (fingerprint/face) or device PIN on the Android app.
- **Persistent capture notification**: The Android foreground service notification is always visible while capture is active. You cannot run Jotnar silently.
- **Weekly capture reminder**: A periodic notification reminds the user that capture is active and how many screenshots were taken — even if they haven't opened the app.
- **No remote access to screenshots**: Screenshots never reach the database. Even with server access, there are no images to view.
- **Device-scoped tokens**: Each device has its own token. Revoking a device immediately cuts its access.

## App Blocklist

Certain app categories are blocked from capture by default:

| Category | Examples | Reason |
|----------|----------|--------|
| Finance | Banking, payment, crypto apps | Sensitive financial data |
| Health | Health trackers, medical, therapy apps | Protected health information |
| Authentication | Password managers, 2FA apps | Security credentials |

Users can add any app to the blocklist manually. The blocklist is enforced on the device — blocked apps are never captured, so no data about them reaches the server.

## Interpretation Detail Levels

Users control how much the AI model extracts from each screenshot:

| Level | What it captures |
|-------|-----------------|
| `minimal` | App name and general activity only — "Using Discord" |
| `standard` | App name and summarized activity — "Chatting with Alex on Discord about weekend plans" |
| `detailed` | App name, key topics, names, and context |

Lower detail = less personal information stored.

## Token Security

- Device tokens are stored in the Android encrypted keystore, not in SharedPreferences or plaintext.
- The server stores only bcrypt hashes of tokens, never the raw values.
- Tokens don't expire but can be revoked at any time from any paired device.

## Data Portability and Deletion

- All data lives in a single SQLite file (`/data/journal.db`). You can back it up, move it, or delete it at will.
- Individual journal entries and their linked metadata can be deleted through the app or API.
- The `metadata_retention_days` config automatically cleans up old metadata while keeping journal entries intact.
- To delete everything: stop the containers and remove the Docker volume.
